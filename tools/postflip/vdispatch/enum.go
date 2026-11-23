package main

// The -enums phase: dispatch in enums whose constants have bodies.
//
// A Java enum Foo whose constants have bodies is a struct Foo embedding the runtime's Enum, and a
// struct per constant with a body embedding Foo. Foo's dispatchers assert Enum's self_, which is
// declared `any` in the runtime (rt_object.go) and set by every enum constant of the package and
// of the runtime:
//
//	func (this *Foo) m() bool { return this.self_.(interface{ m_impl() bool }).m_impl() }
//
// -retype cannot type Enum's self_ with one interface: its methods would be those of every enum
// at once. Instead Foo gets its own field `selfI_ Foo_I`, with the methods Foo's dispatchers call,
// set beside self_ by the constructors of Foo's constants (this.selfI_ = this), and the dispatch
// statement becomes this.selfI_.m_impl(): an itab call without the assertion's cache probe. An
// enum constant is a singleton, so the field costs a few words per enum, not per object.
//
// Foo_I holds a method only when every constant type of Foo (every type whose constructor sets
// self_ and that embeds Foo) has it with the asserted signature. The enum is left alone when one
// of them sets self_ in the runtime, or Foo is declared there. self_ is unchanged (self(), name()
// and the EnumSet code read it). Enums are never cloned (Enum.clone throws), so Object_clone,
// which redirects only self_ in a copy, never meets selfI_. An enum that already has selfI_ is
// left alone, so the phase can be rerun.

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

type enumClass struct {
	tn      *types.TypeName
	decl    *ast.GenDecl
	st      *ast.StructType
	sites   []site
	assigns []*ast.AssignStmt
	types   map[string]types.Type
	bad     string
}

func enumsMain(report bool) {
	enumTN, _ := rw.Pkg.Scope().Lookup("Enum").(*types.TypeName)
	if enumTN == nil {
		fmt.Fprintln(os.Stderr, "no Enum")
		return
	}
	es := enumTN.Type().Underlying().(*types.Struct)
	var selfField *types.Var
	for i := 0; i < es.NumFields(); i++ {
		if es.Field(i).Name() == "self_" {
			selfField = es.Field(i)
		}
	}
	// The class of a type t that directly embeds Enum (nil if t does not embed it).
	enumClassOf := func(t types.Type) *types.TypeName {
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		o, idx, _ := types.LookupFieldOrMethod(t, false, rw.Pkg, "Enum")
		if v, ok := o.(*types.Var); !ok || !v.Embedded() || types.Unalias(v.Type()) != enumTN.Type() {
			return nil
		}
		cur := t
		for _, i := range idx[:len(idx)-1] {
			f := cur.Underlying().(*types.Struct).Field(i)
			cur = f.Type()
			if p, ok := cur.(*types.Pointer); ok {
				cur = p.Elem()
			}
		}
		n, ok := types.Unalias(cur).(*types.Named)
		if !ok {
			return nil
		}
		return n.Obj()
	}

	classes := map[*types.TypeName]*enumClass{}
	class := func(tn *types.TypeName) *enumClass {
		c := classes[tn]
		if c == nil {
			c = &enumClass{tn: tn, types: map[string]types.Type{}}
			classes[tn] = c
		}
		return c
	}
	// Declarations of the classes.
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, sp := range gd.Specs {
				ts, ok := sp.(*ast.TypeSpec)
				if !ok {
					continue
				}
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				tn := rw.Info.Defs[ts.Name].(*types.TypeName)
				if enumClassOf(tn.Type()) != tn {
					continue
				}
				c := class(tn)
				c.decl, c.st = gd, st
				if !generated(gd) {
					c.bad = "declared in the runtime"
				}
				for _, fl := range st.Fields.List {
					for _, nm := range fl.Names {
						if nm.Name == "selfI_" {
							c.bad = "already converted"
						}
					}
				}
			}
		}
	}
	isSelf := func(e ast.Expr) bool {
		s, ok := rw.Unparen(e).(*ast.SelectorExpr)
		if !ok || s.Sel.Name != "self_" {
			return false
		}
		sel, ok := rw.Info.Selections[s]
		return ok && sel.Obj() == selfField
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for i, l := range x.Lhs {
					if !isSelf(l) {
						continue
					}
					recv := rw.Unparen(l).(*ast.SelectorExpr).X
					tn := enumClassOf(rw.Info.Types[recv].Type)
					if tn == nil {
						continue
					}
					c := class(tn)
					if len(x.Lhs) != 1 || len(x.Rhs) != 1 || x.Tok.String() != "=" || !generated(x) {
						c.bad = fmt.Sprintf("self_ written at %s", rw.Fset.Position(x.Pos()))
						continue
					}
					if _, ok := rw.Parent[x].(*ast.BlockStmt); !ok || !rw.Pure(recv) {
						c.bad = fmt.Sprintf("self_ written at %s", rw.Fset.Position(x.Pos()))
						continue
					}
					t := rw.Info.Types[x.Rhs[i]].Type
					if p, ok := t.(*types.Pointer); !ok || !isNamed(p.Elem()) {
						c.bad = fmt.Sprintf("self_ set to a %s at %s", t, rw.Fset.Position(x.Pos()))
						continue
					}
					c.assigns = append(c.assigns, x)
					c.types[types.TypeString(t, nil)] = t
				}
			case *ast.UnaryExpr:
				if isSelf(x.X) {
					if tn := enumClassOf(rw.Info.Types[rw.Unparen(x.X).(*ast.SelectorExpr).X].Type); tn != nil {
						class(tn).bad = fmt.Sprintf("self_ address taken at %s", rw.Fset.Position(x.Pos()))
					}
				}
			case *ast.TypeAssertExpr:
				it, ok := x.Type.(*ast.InterfaceType)
				if !isSelf(x.X) || !ok || len(it.Methods.List) != 1 || len(it.Methods.List[0].Names) != 1 {
					return true
				}
				name := it.Methods.List[0].Names[0].Name
				ps, ok := rw.Parent[x].(*ast.SelectorExpr)
				if !ok || ps.X != x || ps.Sel.Name != name {
					return true
				}
				if c, ok := rw.Parent[ps].(*ast.CallExpr); !ok || c.Fun != ps {
					return true
				}
				recv := rw.Unparen(x.X).(*ast.SelectorExpr).X
				if !generated(x) || !rw.Pure(recv) {
					return true
				}
				tn := enumClassOf(rw.Info.Types[recv].Type)
				if tn == nil {
					return true
				}
				iface := rw.Info.Types[x.Type].Type.Underlying().(*types.Interface)
				c := class(tn)
				c.sites = append(c.sites, site{x, recv, name, iface.Method(0).Type().(*types.Signature), rw.Text(it.Methods.List[0])})
			}
			return true
		})
	}

	var order []*enumClass
	nsites := 0
	for _, c := range classes {
		if len(c.sites) > 0 {
			order = append(order, c)
			nsites += len(c.sites)
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].tn.Name() < order[j].tn.Name() })
	nconv, nmeth, nenums := 0, 0, 0
	for _, c := range order {
		if c.bad == "" && c.decl == nil {
			c.bad = "no declaration"
		}
		if c.bad == "" {
			if _, ok := rw.Pkg.Scope().Lookup(c.tn.Name() + "_I").(*types.TypeName); ok {
				c.bad = c.tn.Name() + "_I exists"
			}
		}
		if c.bad != "" {
			fmt.Fprintf(os.Stderr, "left alone: %s (%d sites): %s\n", c.tn.Name(), len(c.sites), c.bad)
			continue
		}
		sigs := map[string]site{}
		conflict := map[string]bool{}
		for _, s := range c.sites {
			if o, ok := sigs[s.method]; ok && !types.Identical(o.sig, s.sig) {
				conflict[s.method] = true
			} else if !ok {
				sigs[s.method] = s
			}
		}
		var names, tnames []string
		for m := range sigs {
			if !conflict[m] {
				names = append(names, m)
			}
		}
		sort.Strings(names)
		for tn := range c.types {
			tnames = append(tnames, tn)
		}
		sort.Strings(tnames)
		var keep []string
		for _, m := range names {
			var lacking []string
			for _, tn := range tnames {
				o, _, _ := types.LookupFieldOrMethod(c.types[tn], true, rw.Pkg, m)
				fn, ok := o.(*types.Func)
				if !ok || !types.Identical(fn.Type(), sigs[m].sig) {
					lacking = append(lacking, strings.TrimPrefix(tn, "*closure."))
				}
			}
			if len(lacking) > 0 || len(tnames) == 0 {
				fmt.Fprintf(os.Stderr, "%s: %s not on %s; kept\n", c.tn.Name(), m, strings.Join(lacking, ", "))
				continue
			}
			keep = append(keep, m)
		}
		if len(keep) == 0 {
			continue
		}
		nenums++
		nmeth += len(keep)
		in := map[string]bool{}
		iname := c.tn.Name() + "_I"
		var body strings.Builder
		for _, m := range keep {
			in[m] = true
			body.WriteString("\t" + sigs[m].text + "\n")
		}
		decl, st := c.decl, c.st
		comment := fmt.Sprintf("\n\n// %s holds the methods the dispatchers of enum %s call on its\n// constant (selfI_, set with self_); every constant of the enum implements them (post-flip).\n", iname, c.tn.Name())
		rw.Rewrite[decl] = func() string {
			return rw.RenderChildren(decl) + comment + "type " + iname + " interface {\n" + body.String() + "}"
		}
		closing := st.Fields.Closing
		rw.Rewrite[st] = func() string {
			s := rw.RenderChildren(st)
			off := int(closing - st.Pos())
			return s[:off] + "\tselfI_ " + iname + "\n" + s[off:]
		}
		for _, a := range c.assigns {
			a := a
			rw.Rewrite[a] = func() string {
				return rw.RenderChildren(a) + "\n\t" + rw.Render(rw.Unparen(a.Lhs[0]).(*ast.SelectorExpr).X) + ".selfI_ = " + rw.Render(a.Rhs[0])
			}
		}
		n := 0
		for _, s := range c.sites {
			if !in[s.method] {
				continue
			}
			s := s
			rw.Rewrite[s.ta] = func() string { return rw.Render(s.recv) + ".selfI_" }
			n++
		}
		nconv += n
		if report {
			fmt.Printf("%s\t%d constants\t%d methods\t%d sites\n", c.tn.Name(), len(c.types), len(keep), n)
		}
	}
	fmt.Fprintf(os.Stderr, "enum dispatch sites: %d in %d enums; through selfI_: %d sites, %d methods, %d enums\n",
		nsites, len(order), nconv, nmeth, nenums)
	if !report {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
	}
}
