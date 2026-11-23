// Command dispatch is post-flip step 6: virtual calls with a single implementation become
// direct calls.
//
// j2go writes an overridable method m as a dispatcher that finds the most derived object and
// calls its m_impl:
//
//	func (this *C) m(p0 T) R { return this.self_.(interface{ m_impl(T) R }).m_impl(p0) }
//
// When the whole package declares m_impl with that signature exactly once, on a class D that no
// other class extends, every object that reaches the dispatcher is a D: the assertion to an
// anonymous interface (an itab lookup) becomes an assertion to *D (one comparison), and the
// call is a direct call the compiler can inline. (j2go writes a dispatcher only for methods
// that some class overrides or implements, so D is never C itself.)
//
// usage: dispatch [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"

	"postflip/rw"
)

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	sigKey := func(f *types.Func) string {
		sig := f.Type().(*types.Signature)
		k := f.Name() + "("
		for i := 0; i < sig.Params().Len(); i++ {
			k += types.TypeString(types.Unalias(sig.Params().At(i).Type()), nil) + ","
		}
		k += ")"
		for i := 0; i < sig.Results().Len(); i++ {
			k += types.TypeString(types.Unalias(sig.Results().At(i).Type()), nil) + ","
		}
		return k
	}

	decls := map[string][]*types.Func{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv != nil {
				if o, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					decls[sigKey(o)] = append(decls[sigKey(o)], o)
				}
			}
		}
	}
	// Classes that another class extends (embeds).
	extended := map[types.Type]bool{}
	for _, n := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			if f := st.Field(i); f.Embedded() {
				t := f.Type()
				if p, ok := t.(*types.Pointer); ok {
					t = p.Elem()
				}
				extended[t] = true
			}
		}
	}
	total, direct := 0, 0
	hist := map[int]int{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Body == nil || len(fd.Body.List) != 1 {
				continue
			}
			var call *ast.CallExpr
			switch st := fd.Body.List[0].(type) {
			case *ast.ReturnStmt:
				if len(st.Results) == 1 {
					call, _ = st.Results[0].(*ast.CallExpr)
				}
			case *ast.ExprStmt:
				call, _ = st.X.(*ast.CallExpr)
			}
			if call == nil {
				continue
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				continue
			}
			ta, ok := sel.X.(*ast.TypeAssertExpr)
			if !ok {
				continue
			}
			inner, ok := ta.X.(*ast.SelectorExpr)
			if !ok || inner.Sel.Name != "self_" {
				continue
			}
			if id, ok := inner.X.(*ast.Ident); !ok || id.Name != "this" {
				continue
			}
			total++
			s, ok := rw.Info.Selections[sel]
			if !ok {
				continue
			}
			impl := s.Obj().(*types.Func)
			ds := decls[sigKey(impl)]
			hist[len(ds)]++
			if len(ds) != 1 {
				continue
			}
			recv := ds[0].Type().(*types.Signature).Recv().Type()
			p, ok := recv.(*types.Pointer)
			if !ok || extended[p.Elem()] {
				continue
			}
			named, ok := p.Elem().(*types.Named)
			if !ok {
				continue
			}
			direct++
			tae, name := ta, named.Obj().Name()
			rw.Rewrite[tae.Type] = func() string { return "*" + name }
			if report {
				p := rw.Fset.Position(fd.Pos())
				fmt.Printf("direct\t%s:%d\t%s\n", p.Filename, p.Line, fd.Name.Name)
			}
		}
	}
	fmt.Fprintln(os.Stderr, "implementations per dispatcher:", hist)
	fmt.Fprintf(os.Stderr, "dispatchers: %d; with a single implementation, in a class nothing extends: %d\n", total, direct)
	if !report {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
	}
}
