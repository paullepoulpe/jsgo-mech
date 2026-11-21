// Command castcensus reports where the values of the remaining casts come from: for every
// cast_[T](x), to_X(x) and icast_I(x) in the generated code, what x is. It rewrites nothing; its
// table says which typing step would remove the cast (the collection the element is read from,
// the function whose result is an Object, the Object slot).
//
// usage: castcensus [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/anyflow"
	"postflip/rw"
)

func main() {
	verbose := len(os.Args) > 2 && os.Args[1] == "-v"
	anyflow.Load(os.Args[len(os.Args)-1])
	kinds := map[string]int{}
	detail := map[string]map[string]int{}
	add := func(k, d string) {
		kinds[k]++
		if detail[k] == nil {
			detail[k] = map[string]int{}
		}
		detail[k][d]++
	}
	total := 0
	for _, f := range rw.Files {
		if !anyflow.IsG[rw.Fset.File(f.Pos())] {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) != 1 {
				return true
			}
			fn := rw.Text(c.Fun)
			if !(strings.HasPrefix(fn, "cast_[") || strings.HasPrefix(fn, "to_") || strings.HasPrefix(fn, "icast_")) {
				return true
			}
			if fd := anyflow.EnclosingDecl(c); fd != nil && fd.Recv == nil && (strings.HasPrefix(fd.Name.Name, "icast_") || strings.HasPrefix(fd.Name.Name, "to_")) {
				return true // the helpers themselves
			}
			total++
			x := rw.Unparen(c.Args[0])
			for {
				// through a conversion to an interface type
				if cc, ok := x.(*ast.CallExpr); ok && len(cc.Args) == 1 {
					if tv, ok := rw.Info.Types[cc.Fun]; ok && tv.IsType() {
						x = rw.Unparen(cc.Args[0])
						continue
					}
				}
				break
			}
			xt := rw.Info.Types[x].Type
			switch e := x.(type) {
			case *ast.CallExpr:
				obj, name, recv := anyflow.Callee(e)
				fo, _ := obj.(*types.Func)
				switch {
				case fo == nil:
					add("the result of a function value (a lambda's Object result)", name)
				case name == "obj_" || name == "self" || name == "box_" || name == "strobj_" || name == "boxT_":
					at := ""
					if len(e.Args) == 1 {
						at = types.TypeString(rw.Info.Types[e.Args[0]].Type, func(*types.Package) string { return "" })
					} else if recv != nil {
						at = types.TypeString(rw.Info.Types[recv].Type, func(*types.Package) string { return "" })
					}
					_ = at
					add("a typed value boxed on the spot (an upcast or cross-cast: the static type does not decide it)", name)
				case !anyflow.InG(fo.Pos()):
					k := name
					if recv != nil {
						if tn := anyflow.Named(rw.Info.Types[recv].Type); tn != nil {
							k = tn.Name() + "." + name
						}
					}
					add("the Object result of a runtime function or method (an erased collection's element, a property bag, an Optional, a cache)", k)
				default:
					k := name
					if recv != nil {
						if tn := anyflow.Named(rw.Info.Types[recv].Type); tn != nil {
							k = tn.Name() + "." + name
						}
					}
					if sig := fo.Type().(*types.Signature); sig.Recv() != nil && types.IsInterface(sig.Recv().Type()) {
						add("the Object result of a method called through an interface of the program", k)
					} else {
						add("the Object result of a function or method of the program", k)
					}
				}
			case *ast.Ident, *ast.SelectorExpr:
				var o types.Object
				if id, ok := e.(*ast.Ident); ok {
					o = rw.Info.Uses[id]
				} else {
					o = rw.Info.Uses[e.(*ast.SelectorExpr).Sel]
				}
				v, _ := o.(*types.Var)
				ts := ""
				if xt != nil {
					ts = types.TypeString(xt, func(*types.Package) string { return "" })
				}
				switch {
				case v == nil:
					add("other", rw.Text(x))
				case v.IsField():
					add("a field declared "+ifaceKind(xt), v.Name()+" "+ts)
				default:
					add("a variable or parameter declared "+ifaceKind(xt), ts)
				}
			case *ast.IndexExpr:
				add("an element of a slice or map of Objects", rw.Text(e.X))
			case *ast.TypeAssertExpr:
				add("a type assertion's value", rw.Text(e.Type))
			default:
				add("other", fmt.Sprintf("%T", x))
			}
			return true
		})
	}
	keys := []string{}
	for k := range kinds {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if kinds[keys[i]] != kinds[keys[j]] {
			return kinds[keys[i]] > kinds[keys[j]]
		}
		return keys[i] < keys[j]
	})
	fmt.Printf("%5d casts in the generated code\n", total)
	for _, k := range keys {
		fmt.Printf("%5d %s\n", kinds[k], k)
		ds := []string{}
		for d := range detail[k] {
			ds = append(ds, d)
		}
		sort.Slice(ds, func(i, j int) bool {
			if detail[k][ds[i]] != detail[k][ds[j]] {
				return detail[k][ds[i]] > detail[k][ds[j]]
			}
			return ds[i] < ds[j]
		})
		max := 6
		if verbose {
			max = 40
		}
		for i, d := range ds {
			if i >= max {
				fmt.Printf("        ... %d more\n", len(ds)-max)
				break
			}
			fmt.Printf("      %4d %s\n", detail[k][d], d)
		}
	}
}

func ifaceKind(t types.Type) string {
	if t == nil {
		return "?"
	}
	if s := types.TypeString(t, nil); s == "any" {
		return "Object (any)"
	}
	return "as a Java interface"
}
