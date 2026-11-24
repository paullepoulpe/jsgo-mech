// Command arraycopy is a post-flip step: System.arraycopy between two arrays of one type
// copies them without boxing.
//
// Java's System.arraycopy takes its arrays as Objects, so j2go writes
// System_arraycopy(any(src), sp, any(dst), dp, n): each any(...) allocates a slice header on
// the heap, and arrays of other than eight element types are copied through reflect. When both
// arrays have the same static slice type the call becomes arraycopy_(src, sp, dst, dp, n)
// (rt_num.go), the same copy of the same elements with the same bounds failures.
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
	n, left := 0, 0
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			call, ok := x.(*ast.CallExpr)
			if !ok || len(call.Args) != 5 {
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
}
