// Command optstr is a post-flip step: java.util.Optional<String> is a string, with the null
// String nullstr_ for the empty Optional, as j2go writes every other nullable String.
//
//	Optional[string]                    ->  string
//	Optional_empty[string]()            ->  nullstr_
//	Optional_ofNullable[string](x)      ->  x
//	Optional_of[string](x)              ->  optstrOf_(x)        (x itself for a string literal)
//	o.isPresent() / o.isEmpty()         ->  o != nullstr_ / o == nullstr_
//	o.get() / o.orElseThrow()           ->  optstrGet_(o)
//	o.orElse(y)                         ->  optstrOr_(o, y)     (o for orElse(nullstr_))
//	o.filterFn_(p)                      ->  optstrFilter_(o, p)
//	optEquals_ / optEqualsNull_(a, b)   ->  a == b
//	optHash_(a)                         ->  optstrHash_(a)
//	o.box_()                            ->  optstrBox_(o)       (the Optional object erased code saw)
//	optOf_[string](v)                   ->  optstrUnbox_(v)
//	Io_utf8StringStrict(b)              ->  optstr_(Io_utf8StringStrict(b))  (the runtime's Optional)
//
// Prep step 10 wrote Java's nullable Strings as Optional<String>, and j2go writes Optional as a
// value with three states: null (the zero value, a field never assigned; every method throws
// NullPointerException), empty, and present. A string has two of them here: nullstr_ (Java's
// null String, which Optional.ofNullable makes empty and Optional.of rejects) and any other
// string (present). So the rewrite is exact when no Optional<String> is ever null, and that is
// what the tool proves before it rewrites anything. A null Optional can only come from a Go zero
// value (j2go writes no other: prep 10 turned every `null` given to an Optional into
// Optional.empty(), and no generic code is instantiated with Optional[string]) or from a cast of
// a null Object (optOf_). For every place a zero value can be read the tool checks:
//
//   - fields: every allocation of the class (and of each subclass) is `this := &C{}` followed
//     by silent statements and the constructor call (or the assignment of a captured variable),
//     and every constructor assigns the field (in the field initializers, finit_, or in its body,
//     on every path) before anything can read it. The code run before the assignment (the
//     superclass constructors, earlier initializers) is silent: it mentions `this` only to read
//     or write its fields, to call the constructor chain and to call methods of it whose bodies
//     are silent, so no other code holds the object. Where the object escapes before, either the
//     escape is contained (`this.g = F(this.m)`, F a trivial constructor that only stores its
//     argument: this.g is then tainted, reading it is not silent), or no code the remaining
//     statements can reach reads the field at all (a class-hierarchy call graph of the package,
//     cha.go). A function returning the type must not recover a panic (it would return the zero
//     value), and no struct with such a field is used by value.
//   - package variables: assigned by their class initializer before anything else in it runs
//     (Java initializes the class on its first use, so Java never reads them null; j2go's
//     clinit order is the invariant every String static relies on as well); an enum's field
//     array is assigned for every constant by its constructor in the enum's clinit.
//   - locals declared without a value: definitely assigned before every read (Java's rule;
//     checked on the Go code), or j2go's try result temp ret_N, assigned right before every
//     `return 1` / `code_ = 1` of its closure and read only under `code_N == 1`.
//   - named results, slices, arrays, maps, new, composite literals, type assertions of the
//     type: none may exist (counted if one does).
//   - optOf_[string](v): v is the getOrDefault, with a boxed Optional as default, of an erased
//     map whose every value is put as an Optional's box_; the map keeps its Optional objects
//     (its toString prints them), so its type keeps the spelling MapOf[string, Optional[string]].
//
// and every expression of the type must be one of the forms above or a read of a variable,
// field, array slot, call result or parameter of the type (flowing between slots that all
// become strings together); any other use (an implicit conversion to an interface, a method
// value) is counted and stops the rewrite. With every value non-null: of(x) throws for nullstr_
// as Java's Optional.of(null) does; ofNullable(nullstr_) is empty; get/orElseThrow throw
// NoSuchElementException("No value present") on empty; orElse, filter, isPresent and isEmpty
// read the two states; Optional.equals of two non-null Optional<String> is == of their strings
// (both empty, or both present and String.equals); hashCode is the String's (0 when empty);
// box_ gives erased code the same Optional object (Optional.ofNullable(s).box_()). No exception
// thrown on any path changes: the NullPointerException of a call on a null Optional cannot
// happen, by the proof.
//
// A rerun changes nothing (no Optional[string] is left but the counted map spelling).
//
// usage: optstr [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

var (
	verbose bool
	done    = map[string]int{}
	left    = map[string][]string{}
	proved  = map[string]int{}
)

func leave(n ast.Node, why string) {
	left[why] = append(left[why], rw.Fset.Position(n.Pos()).String())
}

func isOptStr(t types.Type) bool {
	if t == nil {
		return false
	}
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Name() == "Optional" && n.Obj().Pkg() == rw.Pkg && n.TypeArgs().Len() == 1 && types.Identical(n.TypeArgs().At(0), types.Typ[types.String])
}

// contains reports whether t mentions Optional[string] (structs by their fields, named types by
// their type arguments).
func contains(t types.Type) bool {
	if isOptStr(t) {
		return true
	}
	if a, ok := t.(*types.Alias); ok {
		for i := 0; i < a.TypeArgs().Len(); i++ {
			if contains(a.TypeArgs().At(i)) {
				return true
			}
		}
	}
	switch u := types.Unalias(t).(type) {
	case *types.Pointer:
		return contains(u.Elem())
	case *types.Slice:
		return contains(u.Elem())
	case *types.Array:
		return contains(u.Elem())
	case *types.Map:
		return contains(u.Key()) || contains(u.Elem())
	case *types.Chan:
		return contains(u.Elem())
	case *types.Signature:
		for i := 0; i < u.Params().Len(); i++ {
			if contains(u.Params().At(i).Type()) {
				return true
			}
		}
		for i := 0; i < u.Results().Len(); i++ {
			if contains(u.Results().At(i).Type()) {
				return true
			}
		}
	case *types.Named:
		for i := 0; i < u.TypeArgs().Len(); i++ {
			if contains(u.TypeArgs().At(i)) {
				return true
			}
		}
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			if contains(u.Field(i).Type()) {
				return true
			}
		}
	}
	return false
}

func generated(n ast.Node) bool {
	p := rw.Fset.Position(n.Pos()).Filename
	return strings.HasPrefix(p[strings.LastIndex(p, "/")+1:], "g_")
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	index()
	ok := proveSlots()
	ok = checkExprs() && ok
	if ok {
		rewrite()
	}
	keys := []string{}
	for k := range proved {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  proved: %4d %s\n", proved[k], k)
	}
	keys = keys[:0]
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", len(left[k]), k)
		if verbose {
			for _, p := range left[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	total := 0
	keys = keys[:0]
	for k, n := range done {
		keys = append(keys, k)
		total += n
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %5d %s\n", done[k], k)
	}
	if !ok {
		fmt.Fprintf(os.Stderr, "optstr: not proved, nothing rewritten\n")
		os.Exit(1)
	}
	if report || total == 0 {
		fmt.Fprintf(os.Stderr, "optstr: %d rewrites\n", total)
		return
	}
	fmt.Fprintf(os.Stderr, "optstr: %d rewrites; files changed: %d\n", total, rw.SaveChecked())
}
