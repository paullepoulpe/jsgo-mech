// Command tfn is a post-flip step: a typed lambda adapter handed to a runtime function that has
// a variant taking a Go function is that function.
//
// The typed translator writes a lambda for an erased functional interface as an adapter object,
// &ToIntFunction_tfn_X{func(x *X) int32 {...}}, whose one method takes the erased arguments,
// converts them and calls the typed function (g_zstubs.go). The runtime's functions that take
// such interfaces have variants taking a Go function of the method's signature (R_f:
// Comparator_comparingInt_f, Map_computeIfAbsent_f, Suppliers_memoize_f...; the post-flip step
// funcparams used them for the erased lambda wrappers of the first flip). So
//
//	R(a, &I_tfn_X{F})  ->  R_f(a, func(p0 any) int32 { return (F)(cast_[*X](p0)) })
//
// where the function literal is the adapter's method with this.f replaced by F: the same
// conversions in the same order, no adapter object and no interface call per use, and R_f may
// know more about a Go function than about an interface value (the int-key comparator of the
// priority queue). Only where R_f exists in the handwritten runtime and its parameter has the
// adapter method's signature; the package must type-check afterwards. Counted by function; what
// is left is counted by reason.
//
// usage: tfn [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

func generated(p token.Pos) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(p).Filename), "g_")
}

func main() {
	dry := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	// The one method of each adapter type, by the type's object.
	methods := map[types.Object][]*ast.FuncDecl{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 || fd.Body == nil {
				continue
			}
			t := fd.Recv.List[0].Type
			if s, ok := t.(*ast.StarExpr); ok {
				t = s.X
			}
			switch x := t.(type) {
			case *ast.IndexExpr:
				t = x.X
			case *ast.IndexListExpr:
				t = x.X
			}
			if id, ok := t.(*ast.Ident); ok && strings.Contains(id.Name, "_tfn") {
				if o := rw.Info.Uses[id]; o != nil {
					methods[o] = append(methods[o], fd)
				}
			}
		}
	}
	done, left := map[string]int{}, map[string]int{}
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := c.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			fn, ok := rw.Info.Uses[id].(*types.Func)
			if !ok || generated(fn.Pos()) {
				return true
			}
			var adapters []int
			for i, a := range c.Args {
				if adapterOf(a) != nil {
					adapters = append(adapters, i)
				}
			}
			if len(adapters) == 0 {
				return true
			}
			variant := id.Name + "_f"
			if strings.HasSuffix(id.Name, "_") { // mapComputeIfAbsent_ -> mapComputeIfAbsent_f
				variant = id.Name + "f"
			}
			ff, ok := rw.Pkg.Scope().Lookup(variant).(*types.Func)
			if !ok || generated(ff.Pos()) {
				left["the runtime has no _f variant of "+id.Name]++
				return true
			}
			fsig := ff.Type().(*types.Signature)
			if fsig.Params().Len() != len(c.Args) || fsig.Variadic() {
				left[id.Name+": the _f variant has other parameters"]++
				return true
			}
			var edits []func()
			for _, i := range adapters {
				lit := adapterOf(c.Args[i])
				tn := adapterType(lit)
				ms := methods[tn]
				want, isFunc := fsig.Params().At(i).Type().Underlying().(*types.Signature)
				if len(ms) != 1 || !isFunc || tn == nil {
					left[id.Name+": the adapter has no single method, or the _f parameter is no function"]++
					return true
				}
				m := ms[0]
				got, _ := rw.Info.Defs[m.Name].(*types.Func)
				if got == nil || got.Type().(*types.Signature).TypeParams() != nil || !sameSig(got.Type().(*types.Signature), want) ||
					typeArgsOf(lit) {
					left[id.Name+": the adapter method's signature is not the _f parameter's (or the adapter is generic)"]++
					return true
				}
				body := rw.Text(m.Body)
				if strings.Count(body, "this.f(") != 1 || strings.Count(body, "this") != 1 {
					left[id.Name+": the adapter method does more than convert and call"]++
					return true
				}
				sigText := "func" + rw.Text(m.Type.Params)
				if m.Type.Results != nil {
					sigText += " " + rw.Text(m.Type.Results)
				}
				arg, fnExpr := c.Args[i], lit.Elts[0]
				if kv, ok := fnExpr.(*ast.KeyValueExpr); ok {
					fnExpr = kv.Value
				}
				edits = append(edits, func() {
					rw.Rewrite[arg] = func() string {
						return sigText + " " + strings.Replace(body, "this.f(", "("+rw.Render(fnExpr)+")(", 1)
					}
				})
			}
			for _, e := range edits {
				e()
			}
			done[id.Name] += len(edits)
			rw.Rewrite[id] = func() string { return variant }
			return true
		})
	}
	// A method value of an adapter literal, (&I_tfn_X{F}).m, where a Go function is wanted (the
	// typed translator passes it to a parameter or stores it in a slot that is a func): it is the
	// adapter's method m with this.f replaced by F, as a function literal. The adapter object is
	// made only to take its method; a func value has no identity, so nothing can tell them apart.
	mvals := 0
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			lit := adapterOf(sel.X)
			if lit == nil {
				return true
			}
			if c, ok := rw.Up(sel).(*ast.CallExpr); ok && rw.Unparen(c.Fun) == ast.Expr(sel) {
				left["method value: the adapter's method is called at once"]++
				return true
			}
			if _, ok := rw.Info.Types[sel].Type.(*types.Signature); !ok {
				return true
			}
			var m *ast.FuncDecl
			for _, d := range methods[adapterType(lit)] {
				if d.Name.Name == sel.Sel.Name {
					m = d
				}
			}
			if m == nil || typeArgsOf(lit) {
				left["method value: no such method, or the adapter is generic"]++
				return true
			}
			body := rw.Text(m.Body)
			if strings.Count(body, "this.f(") != 1 || strings.Count(body, "this") != 1 {
				left["method value: the adapter method does more than convert and call"]++
				return true
			}
			sigText := "func" + rw.Text(m.Type.Params)
			if m.Type.Results != nil {
				sigText += " " + rw.Text(m.Type.Results)
			}
			fnExpr := lit.Elts[0]
			if kv, ok := fnExpr.(*ast.KeyValueExpr); ok {
				fnExpr = kv.Value
			}
			rw.Rewrite[sel] = func() string {
				return sigText + " " + strings.Replace(body, "this.f(", "("+rw.Render(fnExpr)+")(", 1)
			}
			mvals++
			return true
		})
	}
	fmt.Fprintf(os.Stderr, "tfn: %d method values of an adapter literal are function literals\n", mvals)
	total := 0
	for _, k := range sorted(done) {
		total += done[k]
		fmt.Fprintf(os.Stderr, "  %d %s -> its _f variant\n", done[k], k)
	}
	fmt.Fprintf(os.Stderr, "tfn: %d typed lambda adapters handed to the runtime are Go functions\n", total)
	for _, k := range sorted(left) {
		fmt.Fprintf(os.Stderr, "left: %d %s\n", left[k], k)
	}
	if !dry {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
}

// adapterOf: e is &X_tfn...{F}; returns the composite literal.
func adapterOf(e ast.Expr) *ast.CompositeLit {
	u, ok := rw.Unparen(e).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return nil
	}
	lit, ok := u.X.(*ast.CompositeLit)
	if !ok || len(lit.Elts) != 1 || adapterType(lit) == nil {
		return nil
	}
	return lit
}

func adapterType(lit *ast.CompositeLit) types.Object {
	t := lit.Type
	switch x := t.(type) {
	case *ast.IndexExpr:
		t = x.X
	case *ast.IndexListExpr:
		t = x.X
	}
	id, ok := t.(*ast.Ident)
	if !ok || !strings.Contains(id.Name, "_tfn") {
		return nil
	}
	return rw.Info.Uses[id]
}

func typeArgsOf(lit *ast.CompositeLit) bool {
	switch lit.Type.(type) {
	case *ast.IndexExpr, *ast.IndexListExpr:
		return true
	}
	return false
}

func sameSig(a, b *types.Signature) bool {
	if a.Params().Len() != b.Params().Len() || a.Results().Len() != b.Results().Len() || a.Variadic() != b.Variadic() {
		return false
	}
	for i := 0; i < a.Params().Len(); i++ {
		if !types.Identical(a.Params().At(i).Type(), b.Params().At(i).Type()) {
			return false
		}
	}
	for i := 0; i < a.Results().Len(); i++ {
		if !types.Identical(a.Results().At(i).Type(), b.Results().At(i).Type()) {
			return false
		}
	}
	return true
}

func sorted(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}
