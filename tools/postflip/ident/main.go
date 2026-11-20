// Command ident is a post-flip step: JSCompObjects.identical of two values of one type that
// the code boxes only to compare them is Go's == on the values.
//
//	JSCompObjects_identical(root.box_(), last.getRoot().box_())   ->  (root == last.getRoot())
//	!JSCompObjects_identical(n.box_(), parent.getLastChild().box_())
//	                                                              ->  (n != parent.getLastChild())
//	JSCompObjects_identical(obj_(existing), obj_(ColorPool_PENDING_COLOR))
//	                                                              ->  (existing == ColorPool_PENDING_COLOR)
//
// Java's JSCompObjects.identical(a, b) is `a == b` on two references; the translator passes
// both operands as `any` (g_JSCompObjects.go: `return a == b`). Two interface values are equal
// when their dynamic types are equal and their values are equal, so for operands of one static
// type T, boxed in the same way:
//   - X.box_() of a Node (a handle: 0 is null) is nil for 0 and otherwise an interface of type
//     Node whose value is X's handle (rt_handles.go: handleBox_): the boxes are equal exactly
//     when the handles are, null included;
//   - obj_(X) (rt_core.go) is nil for T's zero value (null) and otherwise any(X): equal exactly
//     when the values are, for every comparable T.
//
// So the comparison of the boxes is the comparison of the values, and Java's behaviour is kept
// for every value, null included. Both operands are still evaluated, left to right, once each;
// box_ and obj_ have no effect beyond their result and cannot throw. A Node's box compared as
// an interface costs a call of runtime.efaceeq and of the type's equality function per
// comparison (ReferenceCollector.popLastBlock, once per block of every reference collection).
//
// Left, counted: operands of different static types, operands boxed otherwise (self() of a
// class pointer: two pointers to one class's record and their self() differ for an object
// whose self_ is not set yet, during its constructor, which the tool cannot rule out; a value
// that is already an interface), interface types. A rerun changes nothing.
//
// usage: ident [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
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
	isNode := func(t types.Type) bool {
		nt, ok := types.Unalias(t).(*types.Named)
		return ok && nt.Obj().Name() == "Node" && nt.Obj().Pkg() == rw.Pkg
	}
	// unboxed is the value an operand boxes and how, or nil.
	unboxed := func(e ast.Expr) (ast.Expr, string) {
		call, ok := rw.Unparen(e).(*ast.CallExpr)
		if !ok {
			return nil, ""
		}
		switch f := call.Fun.(type) {
		case *ast.SelectorExpr:
			if f.Sel.Name == "box_" && len(call.Args) == 0 && isNode(rw.Info.Types[f.X].Type) {
				return f.X, "box_"
			}
			if f.Sel.Name == "self" && len(call.Args) == 0 {
				return nil, "self"
			}
		case *ast.Ident:
			if f.Name == "obj_" && len(call.Args) == 1 {
				if fn, ok := rw.Info.Uses[f].(*types.Func); ok && fn.Pkg() == rw.Pkg {
					return call.Args[0], "obj_"
				}
			}
		case *ast.IndexExpr:
			if id, ok := f.X.(*ast.Ident); ok && id.Name == "obj_" && len(call.Args) == 1 {
				return call.Args[0], "obj_"
			}
		}
		return nil, ""
	}
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				return true
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok || id.Name != "JSCompObjects_identical" {
				return true
			}
			if fn, ok := rw.Info.Uses[id].(*types.Func); !ok || fn.Pkg() != rw.Pkg {
				return true
			}
			a, ka := unboxed(call.Args[0])
			b, kb := unboxed(call.Args[1])
			if a == nil || b == nil {
				if ka == "self" || kb == "self" {
					skip("an operand is self() of a class pointer (equal only once the object's self_ is set)", call)
				} else {
					skip("an operand is not a box_() of a Node or an obj_() of a value", call)
				}
				return true
			}
			ta, tb := rw.Info.Types[a].Type, rw.Info.Types[b].Type
			if !types.Identical(ta, tb) {
				skip("the two operands have different static types", call)
				return true
			}
			if types.IsInterface(ta) || !types.Comparable(ta) {
				skip("the operands are of an interface or not comparable type", call)
				return true
			}
			kind := "box_() of two Nodes -> =="
			if ka == "obj_" || kb == "obj_" {
				kind = "obj_() of two values of one type -> =="
			}
			done[kind]++
			// `!identical(...)` is `!=`; otherwise `(a == b)`.
			var target ast.Node = call
			op := "=="
			if u, ok := rw.Up(call).(*ast.UnaryExpr); ok && u.Op == token.NOT && rw.Unparen(u.X) == call {
				target, op = u, "!="
			}
			rw.Rewrite[target] = func() string { return "(" + rw.Render(a) + " " + op + " " + rw.Render(b) + ")" }
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
		fmt.Fprintf(os.Stderr, "ident: %d comparisons\n", total)
		return
	}
	fmt.Fprintf(os.Stderr, "ident: %d comparisons; files changed: %d\n", total, rw.SaveChecked())
}
