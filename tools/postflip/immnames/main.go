// Command immnames is a post-flip step: the runtime's erased Guava immutable types give their
// names to the typed ones.
//
// The flip translates Guava's ImmutableList, ImmutableSet, ImmutableMap and their builders to
// the runtime's erased types of those names (ImmutableList and ImmutableSet are aliases of the
// one ImmutableCollection; rt_guava.go, rt_guava_map.go). The typed runtime (rt_timm.go) is
// the generic ImmutableList[T], ImmutableSet[K], ImmutableMap[K, V] and builders a typed
// translation would emit, and Go has one name for one type: the erased types are renamed
// ErasedImmutableList, ErasedImmutableSet, ErasedImmutableMap, ErasedImmutableList_Builder,
// ErasedImmutableSet_Builder and ErasedImmutableMap_Builder. Only the Go names change: every
// identifier that denotes one of them in the generated code is respelled, and the Java class
// names that Object.getClass reports for the erased map types (javaNames_, by Go type name)
// are kept under their new names. Nothing else changes; a rerun changes nothing.
//
// usage: immnames [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"postflip/rw"
)

var renames = map[string]string{
	"ImmutableList":         "ErasedImmutableList",
	"ImmutableSet":          "ErasedImmutableSet",
	"ImmutableMap":          "ErasedImmutableMap",
	"ImmutableList_Builder": "ErasedImmutableList_Builder",
	"ImmutableSet_Builder":  "ErasedImmutableSet_Builder",
	"ImmutableMap_Builder":  "ErasedImmutableMap_Builder",
}

func generated(p token.Pos) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(p).Filename), "g_")
}

func main() {
	dry := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	// The erased types, by their new names (the runtime declares them).
	objs := map[types.Object]string{}
	for old, nw := range renames {
		o, _ := rw.Pkg.Scope().Lookup(nw).(*types.TypeName)
		if o == nil {
			fmt.Fprintf(os.Stderr, "the runtime does not declare %s\n", nw)
			os.Exit(1)
		}
		objs[o] = old
		// The old name, while the runtime still declares it as an alias of the new one.
		if oo, ok := rw.Pkg.Scope().Lookup(old).(*types.TypeName); ok && oo.IsAlias() && types.Identical(oo.Type(), o.Type()) {
			objs[oo] = old
		}
	}
	n, names := 0, 0
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			switch x := x.(type) {
			case *ast.Ident:
				if old, ok := objs[rw.Info.Uses[x]]; ok && x.Name == old {
					n++
					rw.Rewrite[x] = func() string { return renames[old] }
				}
			case *ast.KeyValueExpr:
				// javaNames_: "ImmutableMap": "com.google.common.collect.ImmutableMap"
				if lit, ok := x.Key.(*ast.BasicLit); ok && lit.Kind == token.STRING {
					if s, err := strconv.Unquote(lit.Value); err == nil {
						if nw, ok := renames[s]; ok && (s == "ImmutableMap" || s == "ImmutableMap_Builder") {
							names++
							rw.Rewrite[lit] = func() string { return strconv.Quote(nw) }
						}
					}
				}
			}
			return true
		})
	}
	fmt.Fprintf(os.Stderr, "identifiers renamed: %d; Java class names kept: %d\n", n, names)
	if !dry {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
	}
}
