// Command opteq is a post-flip step: equals and hashCode of Optionals whose values Java compares
// by value are computed on the Optionals, without boxing them for the dynamic protocol.
//
//	Object_equals(Optional_of[string](name).box_(), other.box_())   ->  optEquals_(Optional_of[string](name), other)
//	Objects_equals(this.alias_.box_(), o.alias_.box_())             ->  optEqualsNull_(this.alias_, o.alias_)
//	Objects_hashCode(this.alias_.box_())                            ->  optHash_(this.alias_)
//
// (objequals_ is Objects_equals, objhash_ is Objects_hashCode.) j2go passes an Optional<T> to
// Object/Objects equals and hashCode as its box (rt_base.go: Optional.box_, nil for null), and
// the runtime then asks the box for its equals method, which asserts the other operand to an
// Optional and compares the two contents boxed again (Optional.equals: objequals_ of
// content_()). Step eqhash decides such calls from the static types where the type has no
// equals of its own; Optional has one, so these stayed.
//
// The rule applies to two operands of one type Optional[T] (each `X.box_()`), with T a type
// whose values Java's equals compares as Go's == does: a string (String.equals; an Optional
// never holds null), a boolean or integer type (Boolean, Integer, ... equals), an enum (identity
// of the constant), a node handle (Node has no equals: identity). For such T the content
// comparison objequals_(boxT_(a), boxT_(b)) is a.value == b.value when both are present, and
// two empty Optionals are equal. The helpers (rt_base.go) are:
//   - optEquals_(a, b): Object_equals(a.box_(), b.box_()), that is a.equals(b): throws
//     NullPointerException when a is null (Object_equals of nil throws the same), false when b
//     is null or present differently, else the values compared;
//   - optEqualsNull_(a, b): Objects.equals: null equals only null, else as a.equals(b);
//   - optHash_(a): Objects.hashCode: 0 for null, else a.hashCode() (unchanged).
//
// The operands are evaluated once each, in the same order; box_ has no effect beyond its result.
// Left, counted: an operand that is not an Optional's box_(), Optionals of two types, Optionals
// of a type compared otherwise (a class with equals, a float, an interface). A rerun changes
// nothing.
//
// usage: opteq [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"sort"

	"postflip/rw"
)

func main() {
	report, verbose := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	dir := os.Args[len(os.Args)-1]
	rw.Load(dir)
	done := map[string]int{}
	left := map[string]int{}
	leftAt := map[string][]string{}
	skip := func(why string, n ast.Node) {
		left[why]++
		leftAt[why] = append(leftAt[why], rw.Fset.Position(n.Pos()).String())
	}
	// optArg is X and Optional's T if e is X.box_() with X of type Optional[T].
	optArg := func(e ast.Expr) (ast.Expr, types.Type) {
		call, ok := rw.Unparen(e).(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return nil, nil
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "box_" {
			return nil, nil
		}
		nt, ok := types.Unalias(rw.Info.Types[sel.X].Type).(*types.Named)
		if !ok || nt.Obj().Name() != "Optional" || nt.Obj().Pkg() != rw.Pkg || nt.TypeArgs().Len() != 1 {
			return nil, nil
		}
		return sel.X, nt.TypeArgs().At(0)
	}
	// byValue reports whether Java's equals on values of T is Go's ==.
	byValue := func(t types.Type) bool {
		switch u := types.Unalias(t).(type) {
		case *types.Basic:
			return u.Info()&(types.IsString|types.IsInteger|types.IsBoolean) != 0
		case *types.Named:
			if u.Obj().Pkg() != rw.Pkg {
				return false
			}
			b, ok := u.Underlying().(*types.Basic)
			if !ok || b.Info()&types.IsInteger == 0 {
				return false
			}
			if u.Obj().Name() == "Node" {
				return !rw.HasMethod(u, "equals")
			}
			return rw.HasMethod(u, "enum_") && !rw.HasMethod(u, "equals")
		}
		return false
	}
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			if fn, ok := rw.Info.Uses[id].(*types.Func); !ok || fn.Pkg() != rw.Pkg {
				return true
			}
			var helper string
			switch id.Name {
			case "Object_equals":
				helper = "optEquals_"
			case "Objects_equals", "objequals_":
				helper = "optEqualsNull_"
			case "Objects_hashCode", "objhash_":
				helper = "optHash_"
			default:
				return true
			}
			args := make([]ast.Expr, len(call.Args))
			var elem types.Type
			anyOpt := false
			for i, a := range call.Args {
				x, t := optArg(a)
				args[i] = x
				if x != nil {
					anyOpt = true
					if elem == nil {
						elem = t
					} else if !types.Identical(elem, t) {
						skip("Optionals of two types", call)
						return true
					}
				}
			}
			if !anyOpt {
				return true
			}
			for _, x := range args {
				if x == nil {
					skip("an operand is not an Optional's box_()", call)
					return true
				}
			}
			if !byValue(elem) {
				skip("the Optional's values are not compared by == ("+types.TypeString(elem, types.RelativeTo(rw.Pkg))+")", call)
				return true
			}
			done[id.Name+" of Optional boxes -> "+helper]++
			rw.Rewrite[call] = func() string {
				s := helper + "("
				for i, x := range args {
					if i > 0 {
						s += ", "
					}
					s += rw.Render(x)
				}
				return s + ")"
			}
			return true
		})
	}
	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
		if verbose {
			for _, p := range leftAt[k] {
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
		fmt.Fprintf(os.Stderr, "  %4d %s\n", done[k], k)
	}
	if report || total == 0 {
		fmt.Fprintf(os.Stderr, "opteq: %d calls\n", total)
		return
	}
	fmt.Fprintf(os.Stderr, "opteq: %d calls; files changed: %d\n", total, rw.SaveChecked())
}
