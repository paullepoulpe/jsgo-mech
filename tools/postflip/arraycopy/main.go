// Command arraycopy is a post-flip step: System.arraycopy between two arrays of one type
// copies them without boxing.
//
// Java's System.arraycopy takes its arrays as Objects, so j2go writes
// System_arraycopy(any(src), sp, any(dst), dp, n): each any(...) allocates a slice header on
// the heap, and arrays of other than eight element types are copied through reflect. When both
// arrays have the same static slice type the call becomes arraycopy_(src, sp, dst, dp, n)
// (rt_num.go), the same copy of the same elements with the same bounds failures.
//
// Arrays.copyOf of a typed array (T[] copyOf(T[], int)) is written through the erased
// Object[] form: arrmap_(Arrays_copyOf_ObjectArr_int(arrmap_(a, box), n), unbox), three new
// arrays for one. When a, the result and the elements all have one type it becomes
// arrcopyOf_(a, n) (rt_core.go): one new array of length n holding a's first elements, as
// Arrays_copyOf_ObjectArr_int made it.
//
// usage: arraycopy <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"postflip/rw"
)

func main() {
	rw.Load(os.Args[len(os.Args)-1])
	n, left, nOf, leftOf := 0, 0, 0, 0
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			call, ok := x.(*ast.CallExpr)
			if !ok {
				return true
			}
			if name, _ := rw.CallName(call); name == "arrmap_" {
				if a, n := typedCopyOf(call); a != nil {
					rw.Rewrite[call] = func() string { return "arrcopyOf_(" + rw.Render(a) + ", " + rw.Render(n) + ")" }
					nOf++
					return true
				} else if inner, _ := rw.CallName(call.Args[0]); inner == "Arrays_copyOf_ObjectArr_int" {
					leftOf++
				}
				return true
			}
			if len(call.Args) != 5 {
				return true
			}
			if id, ok := call.Fun.(*ast.Ident); !ok || id.Name != "System_arraycopy" {
				return true
			}
			inner := func(e ast.Expr) ast.Expr {
				if name, c := rw.CallName(e); name == "any" && len(c.Args) == 1 {
					return c.Args[0]
				}
				return nil
			}
			src, dst := inner(call.Args[0]), inner(call.Args[2])
			if src == nil || dst == nil {
				left++
				return true
			}
			st, dt := rw.Info.Types[src].Type, rw.Info.Types[dst].Type
			if _, ok := st.Underlying().(*types.Slice); !ok || !types.Identical(st, dt) {
				left++
				return true
			}
			rw.Rewrite[call] = func() string {
				return "arraycopy_(" + rw.Render(src) + ", " + rw.Render(call.Args[1]) + ", " + rw.Render(dst) + ", " + rw.Render(call.Args[3]) + ", " + rw.Render(call.Args[4]) + ")"
			}
			n++
			return true
		})
	}
	rw.Save()
	fmt.Printf("arraycopy: %d calls copy typed arrays (%d left)\n", n, left)
	fmt.Printf("arraycopy: %d Arrays.copyOf of typed arrays copy them directly (%d left)\n", nOf, leftOf)
}

// typedCopyOf matches arrmap_(Arrays_copyOf_ObjectArr_int(arrmap_(a, box), n), unbox), where
// box is func(v_ T) any { return obj_(v_) }, unbox is func(v_ any) T { return cast_[T](v_) }
// and a is a []T, and returns a and n.
func typedCopyOf(outer *ast.CallExpr) (ast.Expr, ast.Expr) {
	if len(outer.Args) != 2 {
		return nil, nil
	}
	name, mid := rw.CallName(outer.Args[0])
	if name != "Arrays_copyOf_ObjectArr_int" || len(mid.Args) != 2 {
		return nil, nil
	}
	name, in := rw.CallName(mid.Args[0])
	if name != "arrmap_" || len(in.Args) != 2 {
		return nil, nil
	}
	a := in.Args[0]
	at, rt := rw.Info.Types[a].Type, rw.Info.Types[outer].Type
	sl, ok := at.Underlying().(*types.Slice)
	if !ok || !types.Identical(at, rt) {
		return nil, nil
	}
	elem := types.TypeString(sl.Elem(), types.RelativeTo(rw.Pkg))
	if !bodyIs(in.Args[1], "return obj_(v_)") || !bodyIs(outer.Args[1], "return cast_["+elem+"](v_)") {
		return nil, nil
	}
	return a, mid.Args[1]
}

// bodyIs reports whether e is a func literal of one parameter v_ whose body is the one
// statement want.
func bodyIs(e ast.Expr, want string) bool {
	fl, ok := e.(*ast.FuncLit)
	if !ok || len(fl.Type.Params.List) != 1 || len(fl.Type.Params.List[0].Names) != 1 || fl.Type.Params.List[0].Names[0].Name != "v_" {
		return false
	}
	return len(fl.Body.List) == 1 && rw.Text(fl.Body.List[0]) == want
}
