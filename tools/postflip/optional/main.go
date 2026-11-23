// Command optional is post-flip step 7, first half: makes the code independent of whether
// java.util.Optional is a pointer (*Optional) or a value (Optional). A nil of Optional type
// becomes optNil_, and x == nil / x != nil on an Optional become optNull_(x) / !optNull_(x).
// The second half (rt_base.go, g_0aliases.go, sed) changes the type itself.
//
// usage: optional <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"

	"postflip/rw"
)

func main() {
	rw.Load(os.Args[len(os.Args)-1])
	opt := types.NewPointer(rw.Pkg.Scope().Lookup("Optional").Type())
	isOpt := func(e ast.Expr) bool {
		tv, ok := rw.Info.Types[e]
		return ok && tv.Type != nil && types.Identical(types.Unalias(tv.Type), opt)
	}
	isNil := func(e ast.Expr) bool {
		id, ok := rw.Unparen(e).(*ast.Ident)
		return ok && id.Name == "nil"
	}
	nNil, nCmp := 0, 0
	skip := map[ast.Node]bool{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if b, ok := n.(*ast.BinaryExpr); ok && (b.Op == token.EQL || b.Op == token.NEQ) {
				x, y := b.X, b.Y
				if isNil(x) {
					x, y = y, x
				}
				if isNil(y) && isOpt(x) {
					not := ""
					if b.Op == token.NEQ {
						not = "!"
					}
					xx := x
					rw.Rewrite[b] = func() string { return "(" + not + "optNull_(" + rw.Render(xx) + "))" }
					skip[rw.Unparen(y)] = true
					nCmp++
				}
			}
			if id, ok := n.(*ast.Ident); ok && id.Name == "nil" && !skip[id] && isOpt(id) {
				rw.Rewrite[id] = func() string { return "optNil_" }
				nNil++
			}
			return true
		})
	}
	fmt.Fprintf(os.Stderr, "nil Optionals: %d; comparisons with nil: %d; files changed: %d\n", nNil, nCmp, rw.Save())
}
