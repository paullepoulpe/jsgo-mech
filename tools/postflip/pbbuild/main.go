// Command pbbuild is a post-flip step: the cast of a protobuf builder's build() to its message
// class is the builder's typed buildT_():
//
//	cast_[*ConformanceConfig](builder.build())   ->   builder.buildT_()
//
// Java's generated builder declares build() with its message class (a covariant override of
// MessageLite.Builder.build()); the translation calls the runtime's erased build(), which
// returns the message as a MessageLite, and casts it back. buildT_ (rt_proto_build.go) builds
// the same message with the same pbBuild_ and returns it as the class: the cast could not fail
// (the builder of class G builds a G) and the value is the same. The receiver must be of
// static type *G_Builder and the cast to *G (cast_[*G], to_G or the assertion .(*G)). A rerun
// changes nothing.
//
// usage: pbbuild [-n] [-v] <dir of package closure>
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
	report, verbose := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	n, left := 0, 0
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(nd ast.Node) bool {
			c, ok := nd.(*ast.CallExpr)
			if !ok || len(c.Args) != 0 {
				return true
			}
			s, ok := c.Fun.(*ast.SelectorExpr)
			if !ok || s.Sel.Name != "build" {
				return true
			}
			rt := rw.Info.TypeOf(s.X)
			// a protobuf builder (the runtime's pbb_), not one of the program's
			if pb, _, _ := types.LookupFieldOrMethod(rt, true, rw.Pkg, "pbb_"); pb == nil {
				return true
			}
			o, _, _ := types.LookupFieldOrMethod(rt, true, rw.Pkg, "buildT_")
			bt, ok := o.(*types.Func)
			if !ok {
				return true
			}
			want := bt.Type().(*types.Signature).Results().At(0).Type()
			// the enclosing cast
			var cast ast.Node
			switch p := rw.Up(c).(type) {
			case *ast.CallExpr:
				if len(p.Args) == 1 && rw.Unparen(p.Args[0]) == ast.Expr(c) {
					name, _ := rw.CallName(p)
					cv := rw.ConvFor(want)
					if cv.OK && cv.IsUnbox(name) && types.Identical(rw.Info.TypeOf(p), want) {
						cast = p
					}
				}
			case *ast.TypeAssertExpr:
				if p.Type != nil && types.Identical(rw.Info.TypeOf(p), want) {
					cast = p
				}
			}
			if cast == nil {
				left++
				if verbose {
					fmt.Fprintf(os.Stderr, "        left: %s\n", rw.Fset.Position(c.Pos()))
				}
				return true
			}
			x := s.X
			rw.Rewrite[cast] = func() string { return rw.Render(x) + ".buildT_()" }
			n++
			return true
		})
	}
	fmt.Fprintf(os.Stderr, "pbbuild: %d casts of a builder's build() to its message class are buildT_(); left %d build() calls whose result is not cast to the class (given to erased code)\n", n, left)
	if report || n == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "pbbuild: files changed: %d\n", rw.SaveChecked())
}
