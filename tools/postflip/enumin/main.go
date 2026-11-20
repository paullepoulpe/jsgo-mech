// Command enumin is a post-flip step: the membership test of an enum value in a constant
// immutable set of enum constants is a switch on the value.
//
//	NodeUtil_IS_STATEMENT_PARENT.contains(obj_(parent.getToken()))
//	    ->  NodeUtil_IS_STATEMENT_PARENT_has_(parent.getToken())
//
//	// NodeUtil_IS_STATEMENT_PARENT_has_ is NodeUtil_IS_STATEMENT_PARENT.contains(obj_(x)) ...
//	func NodeUtil_IS_STATEMENT_PARENT_has_(x Token_rhino) bool {
//		switch x {
//		case Token_rhino_SCRIPT, Token_rhino_MODULE_BODY, Token_rhino_BLOCK, Token_rhino_LABEL, ...:
//			return true
//		}
//		return false
//	}
//
// Java declares these sets `static final ImmutableSet<Token> X = Sets.immutableEnumSet(A, B,
// ...)` and asks `X.contains(token)`; the hand port (jsgo, NodeUtil_IsStatementParent) writes
// a switch over the same constants. The translation boxes the value, asserts it to an
// interface with ordinal(), and compares the boxed element at that ordinal with it
// (rt_guava.go: ImmutableCollection.contains): an interface assertion and an interface
// comparison per test, on the traversal's hot paths (NodeUtil.isStatementParent,
// GuardedCallback, NodeUtil.isValidCfgRoot).
//
// A set qualifies when it is a package variable assigned once, in its class's static
// initializer (X_clinit, which the package's init runs before any other code: g_0init.go), from
// Sets_immutableEnumSet of constants of one enum type E, and the test is contains(obj_(x)) of a
// value x of type E. Then the set's elements are exactly those constants for the whole run
// (an immutable set, never reassigned), contains(x) is true exactly when x is one of them, and
// null (obj_ of E's 0, which is not a constant) is in no set: the switch gives the same answer
// for every value. x is evaluated once, as before; the set variable is no longer read there
// (it holds the same set at every read, and reading it has no effect). The variable stays for
// its other uses.
//
// The functions are written to g_0enumin.go, one per set, the constants in the initializer's
// order without repeats. Left, counted: contains(obj_(x)) of an enum value on a set that is not
// such a constant (a field, a set built otherwise, assigned more than once). A rerun changes
// nothing.
//
// usage: enumin [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

const outFile = "g_0enumin.go"

// enumSet is a qualifying set: its variable, enum type and constants.
type enumSet struct {
	v      *types.Var
	elem   *types.Named
	consts []string
}

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
	// isEnum: a named type of the package with enum_() and an integer underlying type (the
	// translator's enums: a constant is ordinal + 1, 0 is null).
	isEnum := func(t types.Type) *types.Named {
		nt, ok := types.Unalias(t).(*types.Named)
		if !ok || nt.Obj().Pkg() != rw.Pkg {
			return nil
		}
		if b, ok := nt.Underlying().(*types.Basic); !ok || b.Info()&types.IsInteger == 0 {
			return nil
		}
		if !rw.HasMethod(nt, "enum_") {
			return nil
		}
		return nt
	}
	// The assignments to package variables, by variable, and the qualifying ones.
	assigns := map[*types.Var][]*ast.AssignStmt{}
	inClinit := map[*ast.AssignStmt]bool{}
	for _, file := range rw.Files {
		for _, d := range file.Decls {
			fd, _ := d.(*ast.FuncDecl)
			ast.Inspect(d, func(n ast.Node) bool {
				as, ok := n.(*ast.AssignStmt)
				if !ok {
					return true
				}
				for _, l := range as.Lhs {
					id, ok := l.(*ast.Ident)
					if !ok {
						continue
					}
					v, ok := rw.Info.Uses[id].(*types.Var)
					if !ok || v.Parent() != rw.Pkg.Scope() {
						continue
					}
					assigns[v] = append(assigns[v], as)
					if fd != nil && fd.Recv == nil && strings.HasSuffix(fd.Name.Name, "_clinit") && rw.Parent[as] == fd.Body {
						inClinit[as] = true
					}
				}
				return true
			})
		}
	}
	// constOf is the name of the enum constant of E that e.enum_() is, or "".
	constOf := func(e ast.Expr, E **types.Named) string {
		call, ok := e.(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return ""
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "enum_" {
			return ""
		}
		id, ok := sel.X.(*ast.Ident)
		if !ok {
			return ""
		}
		c, ok := rw.Info.Uses[id].(*types.Const)
		if !ok || c.Parent() != rw.Pkg.Scope() {
			return ""
		}
		nt := isEnum(c.Type())
		if nt == nil || (*E != nil && *E != nt) {
			return ""
		}
		*E = nt
		return id.Name
	}
	sets := map[*types.Var]*enumSet{}
	why := map[*types.Var]string{}
	setOf := func(v *types.Var) (*enumSet, string) {
		if s, ok := sets[v]; ok {
			return s, ""
		}
		if w, ok := why[v]; ok {
			return nil, w
		}
		s, w := func() (*enumSet, string) {
			as := assigns[v]
			if len(as) != 1 {
				return nil, "the set variable is assigned more than once"
			}
			if !inClinit[as[0]] || len(as[0].Lhs) != 1 || len(as[0].Rhs) != 1 {
				return nil, "the set variable is assigned outside its class's static initializer"
			}
			name, call := rw.CallName(as[0].Rhs[0])
			if name != "Sets_immutableEnumSet" || len(call.Args) != 2 {
				return nil, "the set is not built by Sets_immutableEnumSet"
			}
			var E *types.Named
			first := constOf(call.Args[0], &E)
			lit, ok := call.Args[1].(*ast.CompositeLit)
			if first == "" || !ok {
				return nil, "the set's elements are not enum constants written out"
			}
			s := &enumSet{v: v, consts: []string{first}}
			seen := map[string]bool{first: true}
			for _, el := range lit.Elts {
				c := constOf(el, &E)
				if c == "" {
					return nil, "the set's elements are not enum constants written out"
				}
				if !seen[c] {
					seen[c] = true
					s.consts = append(s.consts, c)
				}
			}
			s.elem = E
			return s, ""
		}()
		if s != nil {
			sets[v] = s
		} else {
			why[v] = w
		}
		return s, w
	}
	used := map[*types.Var]bool{}
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "contains" {
				return true
			}
			name, arg := rw.CallName(call.Args[0])
			if arg == nil || (name != "obj_" && !strings.HasPrefix(name, "obj_[")) || len(arg.Args) != 1 {
				return true
			}
			x := arg.Args[0]
			E := isEnum(rw.Info.Types[x].Type)
			if E == nil {
				return true
			}
			id, ok := sel.X.(*ast.Ident)
			if !ok {
				skip("the set is not a package variable", call)
				return true
			}
			v, ok := rw.Info.Uses[id].(*types.Var)
			if !ok || v.Parent() != rw.Pkg.Scope() {
				skip("the set is not a package variable", call)
				return true
			}
			s, w := setOf(v)
			if s == nil {
				skip(w, call)
				return true
			}
			if s.elem != E {
				skip("the value's enum type is not the set's", call)
				return true
			}
			if t, ok := v.Type().(*types.Pointer); !ok || !isNamed(t.Elem(), "ImmutableCollection") {
				skip("the set variable is not an *ErasedImmutableSet (ImmutableCollection)", call)
				return true
			}
			used[v] = true
			done["contains(obj_(x)) on a constant enum set -> X_has_(x)"]++
			fn := v.Name() + "_has_"
			rw.Rewrite[call] = func() string { return fn + "(" + rw.Render(x) + ")" }
			return true
		})
	}
	// The functions the package has, and those the rewrites need.
	var outF *ast.File
	have := map[string]bool{}
	for _, f := range rw.Files {
		if filepath.Base(rw.Fset.File(f.Pos()).Name()) != outFile {
			continue
		}
		outF = f
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				have[fd.Name.Name] = true
			}
		}
	}
	var vs []*types.Var
	for v := range used {
		if !have[v.Name()+"_has_"] {
			vs = append(vs, v)
		}
	}
	sort.Slice(vs, func(i, j int) bool { return vs[i].Name() < vs[j].Name() })
	var b strings.Builder
	for _, v := range vs {
		s := sets[v]
		fn := v.Name() + "_has_"
		fmt.Fprintf(&b, "\n// %s is\n// %s.contains(obj_(x)): the set is Sets.immutableEnumSet\n// of these constants, built once by %s_clinit.\n",
			fn, v.Name(), classOf(assigns[v][0]))
		fmt.Fprintf(&b, "func %s(x %s) bool {\n\tswitch x {\n\tcase %s:\n\t\treturn true\n\t}\n\treturn false\n}\n", fn, s.elem.Obj().Name(), strings.Join(s.consts, ", "))
	}
	decls := b.String()

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
		fmt.Fprintf(os.Stderr, "enumin: %d tests, %d new functions\n", total, len(vs))
		return
	}
	if outF != nil {
		last := outF.Decls[len(outF.Decls)-1]
		rw.Rewrite[last] = func() string { return rw.RenderChildren(last) + "\n" + decls }
	} else {
		path := filepath.Join(dir, outFile)
		src := []byte("// Code generated by tools/postflip/enumin. DO NOT EDIT.\n\npackage closure\n\n" +
			"// Membership tests of enum values in the constant immutable enum sets.\n" + decls)
		if err := os.WriteFile(path, src, 0o644); err != nil {
			panic(err)
		}
		f, err := parser.ParseFile(rw.Fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			panic(err)
		}
		rw.Src[rw.Fset.File(f.Pos())] = src
		rw.Files = append(rw.Files, f)
	}
	fmt.Fprintf(os.Stderr, "enumin: %d tests, %d new functions; files changed: %d\n", total, len(vs), rw.SaveChecked())
}

// classOf is the class of the static initializer that holds as (X of X_clinit).
func classOf(as *ast.AssignStmt) string {
	for n := rw.Parent[ast.Node(as)]; n != nil; n = rw.Parent[n] {
		if fd, ok := n.(*ast.FuncDecl); ok {
			return strings.TrimSuffix(fd.Name.Name, "_clinit")
		}
	}
	return ""
}

// isNamed reports whether t (through aliases) is the package's named type name.
func isNamed(t types.Type, name string) bool {
	nt, ok := types.Unalias(t).(*types.Named)
	return ok && nt.Obj().Name() == name && nt.Obj().Pkg() == rw.Pkg
}
