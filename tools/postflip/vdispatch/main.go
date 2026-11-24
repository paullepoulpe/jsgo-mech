// Command vdispatch is a post-flip step: a virtual call is a call through a named interface held
// by the object, not an assertion to an anonymous interface.
//
// j2go writes an overridable method m as a dispatcher that finds the most derived object and
// calls its m_impl:
//
//	func (this *C) m(p0 T) R { return this.self_.(interface{ m_impl(T) R }).m_impl(p0) }
//
// (the overrides step repeats that statement as the fallback of the methods it adds). self_ is
// declared `any` on the root class R of the hierarchy, and the constructor of every class sets
// it to the new object (this.self_ = this). The assertion looks up an itab in the runtime's
// table on every call. This step gives each root class R whose dispatchers (in generated code)
// make such calls one named interface R_I with every m_impl they call, and a field
// `selfI_ R_I` beside self_, set wherever self_ is set, to the same value. The dispatch
// statement becomes this.selfI_.m_impl(p0): an itab call with no assertion, of the method the
// assertion found (selfI_ holds the same dynamic value as self_, and the method set of that
// value's type is the same whichever interface it is called through).
//
// A method is in R_I only when every type assigned to self_ of R in the package has it, with
// the signature the dispatcher asserts (go/types: declared or promoted from an embedded class,
// so a class that does not override m_impl calls the base one, as the assertion did). A
// method some assigned type lacks keeps its assertion (Java would never call it on such an
// object; the assertion would panic as before). self_ stays `any`: self(), the overrides
// step's this.self_ == any(this) checks and step 6's concrete assertions read it.
//
// With -retype there is no new field: self_ itself is declared R_I (every value assigned to
// it implements R_I, and every reader of self_ in the package type-checks with it: self()
// converts it to any, the comparisons and concrete assertions work on any interface), and
// the dispatch statement becomes this.self_.m_impl(p0). Objects keep their size.
//
// Roots declared in the handwritten runtime (rt_*.go), and roots whose self_ is written
// anywhere other than an assignment statement of a concrete pointer in generated code, are
// left alone. A root that already has selfI_ is left alone, so the step can be rerun.
//
// With -sub (after -retype), the methods a sub-hierarchy's dispatchers call go into R_I too,
// with a body on R that is never run; see sub.go. With -enums, an enum whose constants have
// bodies gets its own interface and field selfI_; see enum.go. With -rest (after those), the
// roots that have no dispatch site of the form above (generic classes, which dispatch through a
// vt_R_ field, and classes without an overridden method) get self_ typed too, and the casts
// to_X that assert to an anonymous interface assert to a named X_as; see rest.go.
//
// usage: vdispatch [-n] [-retype | -sub | -enums | -rest] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

func generated(n ast.Node) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "g_")
}

type site struct {
	ta     *ast.TypeAssertExpr
	recv   ast.Expr // the expression whose self_ is asserted
	method string
	sig    *types.Signature
	text   string // the interface's method, as written
}

type root struct {
	tn      *types.TypeName
	field   *types.Var
	fieldAt *ast.Field
	decl    *ast.GenDecl
	sites   []site
	assigns []*ast.AssignStmt
	types   map[string]types.Type // assigned types, by name
	bad     string                // why the root is left alone
}

func main() {
	report, retype, sub, enums, rest := false, false, false, false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		switch a {
		case "-n":
			report = true
		case "-retype":
			retype = true
		case "-sub":
			sub = true
		case "-enums":
			enums = true
		case "-rest":
			rest = true
		default:
			panic("usage: vdispatch [-n] [-retype | -sub | -enums | -rest] <dir>")
		}
	}
	rw.Load(os.Args[len(os.Args)-1])
	if sub {
		subMain(report)
		return
	}
	if enums {
		enumsMain(report)
		return
	}
	if rest {
		restMain(report)
		return
	}

	// Roots: struct types declaring a field self_ of type any.
	roots := map[*types.Var]*root{}
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
				for _, fl := range st.Fields.List {
					for _, nm := range fl.Names {
						if nm.Name != "self_" {
							continue
						}
						v := rw.Info.Defs[nm].(*types.Var)
						if t, ok := types.Unalias(v.Type()).(*types.Interface); !ok || !t.Empty() {
							continue
						}
						r := &root{tn: rw.Info.Defs[ts.Name].(*types.TypeName), field: v, fieldAt: fl, decl: gd, types: map[string]types.Type{}}
						if !generated(gd) {
							r.bad = "declared in the runtime"
						}
						roots[v] = r
					}
				}
			}
		}
	}
	for _, r := range roots {
		if _, ok := rw.Pkg.Scope().Lookup(r.tn.Name() + "_I").(*types.TypeName); ok {
			if r.bad == "" {
				r.bad = "already converted"
			}
		}
	}

	// Writes of self_, and dispatch sites.
	selfVar := func(e ast.Expr) *root {
		s, ok := rw.Unparen(e).(*ast.SelectorExpr)
		if !ok || s.Sel.Name != "self_" {
			return nil
		}
		sel, ok := rw.Info.Selections[s]
		if !ok {
			return nil
		}
		v, _ := sel.Obj().(*types.Var)
		return roots[v]
	}
	nsites := 0
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for i, l := range x.Lhs {
					r := selfVar(l)
					if r == nil {
						continue
					}
					if len(x.Lhs) != 1 || len(x.Rhs) != 1 || x.Tok.String() != "=" || !generated(x) {
						r.bad = fmt.Sprintf("self_ written at %s", rw.Fset.Position(x.Pos()))
						continue
					}
					if _, ok := rw.Parent[x].(*ast.BlockStmt); !ok {
						r.bad = fmt.Sprintf("self_ written outside a block at %s", rw.Fset.Position(x.Pos()))
						continue
					}
					t := rw.Info.Types[x.Rhs[i]].Type
					if p, ok := t.(*types.Pointer); !ok || !isNamed(p.Elem()) {
						r.bad = fmt.Sprintf("self_ set to a %s at %s", t, rw.Fset.Position(x.Pos()))
						continue
					}
					r.assigns = append(r.assigns, x)
					r.types[types.TypeString(t, nil)] = t
				}
			case *ast.UnaryExpr:
				if r := selfVar(x.X); r != nil {
					r.bad = fmt.Sprintf("self_ address taken at %s", rw.Fset.Position(x.Pos()))
				}
			case *ast.KeyValueExpr:
				if id, ok := x.Key.(*ast.Ident); ok && id.Name == "self_" {
					if v, ok := rw.Info.Uses[id].(*types.Var); ok && roots[v] != nil {
						roots[v].bad = fmt.Sprintf("self_ in a composite literal at %s", rw.Fset.Position(x.Pos()))
					}
				}
			case *ast.TypeAssertExpr:
				r := selfVar(x.X)
				it, ok := x.Type.(*ast.InterfaceType)
				if r == nil || !ok || len(it.Methods.List) != 1 || len(it.Methods.List[0].Names) != 1 {
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
				iface := rw.Info.Types[x.Type].Type.Underlying().(*types.Interface)
				nsites++
				r.sites = append(r.sites, site{x, recv, name, iface.Method(0).Type().(*types.Signature), rw.Text(it.Methods.List[0])})
			}
			return true
		})
	}

	var order []*root
	for _, r := range roots {
		if len(r.sites) > 0 {
			order = append(order, r)
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].tn.Name() < order[j].tn.Name() })

	nroots, nconv, nmeth, npruned := 0, 0, 0, 0
	for _, r := range order {
		if r.bad != "" {
			fmt.Fprintf(os.Stderr, "left alone: %s (%d sites): %s\n", r.tn.Name(), len(r.sites), r.bad)
			continue
		}
		// The methods the dispatchers call, by name; one signature each.
		sigs := map[string]site{}
		conflict := map[string]bool{}
		for _, s := range r.sites {
			if o, ok := sigs[s.method]; ok && !types.Identical(o.sig, s.sig) {
				conflict[s.method] = true
			} else if !ok {
				sigs[s.method] = s
			}
		}
		var names []string
		for m := range sigs {
			if conflict[m] {
				fmt.Fprintf(os.Stderr, "%s: %s asserted with two signatures; kept\n", r.tn.Name(), m)
				continue
			}
			names = append(names, m)
		}
		sort.Strings(names)
		var tnames []string
		for tn := range r.types {
			tnames = append(tnames, tn)
		}
		sort.Strings(tnames)
		// Keep the methods every assigned type has with the asserted signature.
		var keep []string
		for _, m := range names {
			var lacking []string
			for _, tn := range tnames {
				o, _, _ := types.LookupFieldOrMethod(r.types[tn], true, rw.Pkg, m)
				fn, ok := o.(*types.Func)
				if !ok || !types.Identical(fn.Type(), sigs[m].sig) {
					lacking = append(lacking, strings.TrimPrefix(tn, "*closure."))
				}
			}
			if len(lacking) > 0 {
				npruned++
				fmt.Fprintf(os.Stderr, "%s: %s not on %s; kept\n", r.tn.Name(), m, strings.Join(lacking, ", "))
				continue
			}
			keep = append(keep, m)
		}
		if len(keep) == 0 {
			continue
		}
		nroots++
		nmeth += len(keep)
		in := map[string]bool{}
		for _, m := range keep {
			in[m] = true
		}
		iname := r.tn.Name() + "_I"
		var body strings.Builder
		for _, m := range keep {
			body.WriteString("\t" + sigs[m].text + "\n")
		}
		decl, fieldAt := r.decl, r.fieldAt
		field := "selfI_"
		comment := fmt.Sprintf("\n\n// %s holds the methods the dispatchers of %s's hierarchy call on the most derived\n// object (selfI_, set with self_); every class of the hierarchy implements them (post-flip).\n", iname, r.tn.Name())
		if retype {
			field = "self_"
			comment = fmt.Sprintf("\n\n// %s holds the methods the dispatchers of %s's hierarchy call on the most derived\n// object (self_); every class of the hierarchy implements them (post-flip).\n", iname, r.tn.Name())
		}
		decls := "type " + iname + " interface {\n" + body.String() + "}"
		rw.Rewrite[decl] = func() string { return rw.RenderChildren(decl) + comment + decls }
		if retype {
			rw.Rewrite[fieldAt.Type] = func() string { return iname }
		} else {
			rw.Rewrite[fieldAt] = func() string { return rw.RenderChildren(fieldAt) + "\n\tselfI_ " + iname }
			for _, a := range r.assigns {
				a := a
				rw.Rewrite[a] = func() string {
					return rw.RenderChildren(a) + "\n\t" + rw.Render(rw.Unparen(a.Lhs[0]).(*ast.SelectorExpr).X) + ".selfI_ = " + rw.Render(a.Rhs[0])
				}
			}
		}
		n := 0
		for _, s := range r.sites {
			if !in[s.method] {
				continue
			}
			s := s
			rw.Rewrite[s.ta] = func() string { return rw.Render(s.recv) + "." + field }
			n++
		}
		nconv += n
		if report {
			fmt.Printf("%s\t%d classes\t%d methods\t%d sites\n", r.tn.Name(), len(r.types), len(keep), n)
		}
	}
	fmt.Fprintf(os.Stderr, "dispatch sites: %d in %d hierarchies; through a named interface: %d sites, %d methods, %d hierarchies (%d methods kept as assertions)\n",
		nsites, len(order), nconv, nmeth, nroots, npruned)
	if !report {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
	}
}

func isNamed(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.Named)
	return ok
}
