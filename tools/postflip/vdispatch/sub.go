package main

// The -sub phase, run after -retype: the dispatch sites -retype left as assertions because some
// class of the hierarchy lacks the method.
//
// They are the dispatchers of a class C below the root R (ObjectType below JSType, DiGraph below
// Graph, ...) calling a method only C's subclasses have:
//
//	func (this *C) m(p0 T) U { return this.self_.(interface{ m_impl(T) U }).m_impl(p0) }
//
// this.self_ is the most derived object, which embeds C, so the assertion never fails; Java's
// compiler knows that (m is declared on C), Go's does not. A named interface C_I asserted
// instead costs the same: the compiler emits the same per-site cache probe for any interface
// target, named or not. What does not need the probe is a method of R_I. So the step adds m_impl
// to R_I, and gives R a body for it, which every class of the hierarchy that lacks the method
// (the classes outside C's subtree, and abstract classes in it) inherits; the dispatch statement
// becomes this.self_.m_impl(p0), an itab call. R's body is never run: it is reached only where the
// assertion would have panicked (the receiver's class lacks m_impl), and it panics.
//
// A method is added only when that cannot change which method a selector or a dynamic check
// finds:
//   - every struct type of the package that embeds R either has no member m_impl, or has a
//     method m_impl with the asserted signature at a smaller depth than R's (so the classes
//     that implement m keep their method, and none gets an ambiguous selector);
//   - no other type assertion or type switch in the package names an interface with m_impl
//     (with R's body more types implement such an interface).
//
// Roots whose self_ is not typed R_I by -retype (the runtime's Enum) are left alone, as are
// roots declared in the runtime. Rerunning finds no sites left.

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

type subRoot struct {
	tn     *types.TypeName
	iface  *types.TypeName // R_I
	idecl  *ast.GenDecl    // its declaration
	itype  *ast.InterfaceType
	sites  []site
	types  map[string]types.Type // types assigned to self_, by name
	bad    string
	owners map[string]map[string]bool // method -> classes whose dispatchers call it
}

func subMain(report bool) {
	// Roots: struct types R with a field self_ of the named interface R_I declared in generated code.
	roots := map[*types.Var]*subRoot{}
	idecls := map[*types.TypeName]*ast.GenDecl{}
	itypes := map[*types.TypeName]*ast.InterfaceType{}
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
				if it, ok := ts.Type.(*ast.InterfaceType); ok && len(gd.Specs) == 1 {
					tn := rw.Info.Defs[ts.Name].(*types.TypeName)
					idecls[tn], itypes[tn] = gd, it
				}
			}
		}
	}
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
						tn := rw.Info.Defs[ts.Name].(*types.TypeName)
						r := &subRoot{tn: tn, types: map[string]types.Type{}, owners: map[string]map[string]bool{}}
						roots[v] = r
						named, ok := types.Unalias(v.Type()).(*types.Named)
						switch {
						case !generated(gd):
							r.bad = "declared in the runtime"
						case !ok || named.Obj().Name() != tn.Name()+"_I" || idecls[named.Obj()] == nil || !generated(idecls[named.Obj()]):
							r.bad = "self_ is not typed " + tn.Name() + "_I (run -retype first)"
						default:
							r.iface, r.idecl, r.itype = named.Obj(), idecls[named.Obj()], itypes[named.Obj()]
						}
					}
				}
			}
		}
	}

	selfVar := func(e ast.Expr) *subRoot {
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
	// Method names that some type assertion or type switch other than a dispatch site checks.
	checked := map[string]string{}
	noteChecked := func(t types.Type, at ast.Node) {
		it, ok := t.Underlying().(*types.Interface)
		if !ok {
			return
		}
		for i := 0; i < it.NumMethods(); i++ {
			if _, ok := checked[it.Method(i).Name()]; !ok {
				checked[it.Method(i).Name()] = rw.Fset.Position(at.Pos()).String()
			}
		}
	}
	isSite := map[*ast.TypeAssertExpr]bool{}
	nsites := 0
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for i, l := range x.Lhs {
					r := selfVar(l)
					if r == nil || i >= len(x.Rhs) {
						continue
					}
					t := rw.Info.Types[x.Rhs[i]].Type
					if p, ok := t.(*types.Pointer); ok && isNamed(p.Elem()) {
						r.types[types.TypeString(t, nil)] = t
					} else if r.bad == "" {
						r.bad = fmt.Sprintf("self_ set to a %s at %s", t, rw.Fset.Position(x.Pos()))
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
				isSite[x] = true
				r.sites = append(r.sites, site{x, recv, name, iface.Method(0).Type().(*types.Signature), rw.Text(it.Methods.List[0])})
				owner := "?"
				for p := rw.Parent[x]; p != nil; p = rw.Parent[p] {
					if fd, ok := p.(*ast.FuncDecl); ok && fd.Recv != nil {
						owner = strings.TrimPrefix(rw.Text(fd.Recv.List[0].Type), "*")
						break
					}
				}
				if r.owners[name] == nil {
					r.owners[name] = map[string]bool{}
				}
				r.owners[name][owner] = true
			}
			return true
		})
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeAssertExpr:
				if x.Type != nil && !isSite[x] {
					noteChecked(rw.Info.Types[x.Type].Type, x)
				}
			case *ast.TypeSwitchStmt:
				for _, c := range x.Body.List {
					for _, e := range c.(*ast.CaseClause).List {
						if tv, ok := rw.Info.Types[e]; ok && tv.IsType() {
							noteChecked(tv.Type, e)
						}
					}
				}
			}
			return true
		})
	}

	// Struct types of the package, for the depth checks.
	var structs []*types.TypeName
	for _, nm := range rw.Pkg.Scope().Names() {
		if tn, ok := rw.Pkg.Scope().Lookup(nm).(*types.TypeName); ok && !tn.IsAlias() {
			if _, ok := tn.Type().Underlying().(*types.Struct); ok {
				if n, ok := tn.Type().(*types.Named); !ok || n.TypeParams().Len() == 0 {
					structs = append(structs, tn)
				}
			}
		}
	}

	var order []*subRoot
	for _, r := range roots {
		if len(r.sites) > 0 {
			order = append(order, r)
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].tn.Name() < order[j].tn.Name() })

	nconv, nmeth, nroots := 0, 0, 0
	for _, r := range order {
		if r.bad != "" {
			fmt.Fprintf(os.Stderr, "left alone: %s (%d sites): %s\n", r.tn.Name(), len(r.sites), r.bad)
			continue
		}
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
			names = append(names, m)
		}
		sort.Strings(names)
		ri := r.iface.Type().Underlying().(*types.Interface)
		inIface := map[string]bool{}
		for i := 0; i < ri.NumMethods(); i++ {
			inIface[ri.Method(i).Name()] = true
		}
		var add []string // methods added to R_I, with R's body
		conv := map[string]bool{}
		for _, m := range names {
			why := ""
			switch {
			case conflict[m]:
				why = "asserted with two signatures"
			case inIface[m]:
				// Already in R_I (by an earlier run): the site only needs converting.
				if sel := types.NewMethodSet(r.iface.Type()).Lookup(rw.Pkg, m); sel == nil || !types.Identical(sel.Obj().Type(), sigs[m].sig) {
					why = "in " + r.iface.Name() + " with another signature"
				}
			case checked[m] != "":
				why = "checked dynamically at " + checked[m]
			default:
				why = depthConflict(r.tn, m, sigs[m].sig, structs)
			}
			if why != "" {
				fmt.Fprintf(os.Stderr, "%s: %s kept: %s\n", r.tn.Name(), m, why)
				continue
			}
			conv[m] = true
			if !inIface[m] {
				add = append(add, m)
			}
		}
		if len(conv) == 0 {
			continue
		}
		nroots++
		nmeth += len(add)
		if len(add) > 0 {
			it := r.itype
			var lines, stubs strings.Builder
			for _, m := range add {
				lines.WriteString("\t" + sigs[m].text + "\n")
			}
			// Group the added methods by the topmost classes whose dispatchers call them.
			byOwner := map[string][]string{}
			for _, m := range add {
				k := strings.Join(topmost(r.owners[m]), " and ")
				byOwner[k] = append(byOwner[k], m)
			}
			var keys []string
			for k := range byOwner {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			var who []string
			for _, k := range keys {
				who = append(who, fmt.Sprintf("%s (%d)", k, len(byOwner[k])))
			}
			fmt.Fprintf(&stubs, "\n\n// Bodies of the methods that only the subclasses of %s implement. They put the\n"+
				"// methods in %s, so that the dispatchers of those classes call them through self_ without\n"+
				"// an assertion. They are never run (a dispatcher's object is of a subclass of its class,\n"+
				"// which has the method) and panic where the assertion would have (post-flip).\n",
				strings.Join(who, " and "), r.iface.Name())
			for _, m := range add {
				fmt.Fprintf(&stubs, "func (this *%s) %s {\n\tpanic(%q + goTypeName_(this.self_))\n}\n", r.tn.Name(), sigs[m].text, m+" called on a ")
			}
			closing := it.Methods.Closing
			rw.Rewrite[it] = func() string {
				s := rw.RenderChildren(it)
				// Insert before the closing brace (keep the body's own methods as they are).
				off := int(closing - it.Pos())
				return s[:off] + lines.String() + s[off:]
			}
			decl := r.idecl
			rw.Rewrite[decl] = func() string { return rw.RenderChildren(decl) + strings.TrimRight(stubs.String(), "\n") }
		}
		n := 0
		for _, s := range r.sites {
			if !conv[s.method] {
				continue
			}
			s := s
			rw.Rewrite[s.ta] = func() string { return rw.Render(s.recv) + ".self_" }
			n++
		}
		nconv += n
		if report {
			fmt.Printf("%s\t%d classes\t%d methods added\t%d sites\n", r.tn.Name(), len(r.types), len(add), n)
			for _, m := range add {
				fmt.Printf("\t%s\t%s\n", m, strings.Join(topmost(r.owners[m]), ","))
			}
		}
	}
	fmt.Fprintf(os.Stderr, "sub-hierarchy dispatch sites: %d in %d hierarchies; through self_: %d sites, %d methods added to %d interfaces\n",
		nsites, len(order), nconv, nmeth, nroots)
	if !report {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
	}
}

// depthConflict says why giving root a method m with signature sig would change a selector of
// a struct type embedding root ("" if it would not).
func depthConflict(root *types.TypeName, m string, sig *types.Signature, structs []*types.TypeName) string {
	if o, _, _ := types.LookupFieldOrMethod(root.Type(), true, rw.Pkg, m); o != nil {
		return root.Name() + " has a member " + m
	}
	for _, tn := range structs {
		if tn == root {
			continue
		}
		fo, fidx, _ := types.LookupFieldOrMethod(tn.Type(), false, rw.Pkg, root.Name())
		f, ok := fo.(*types.Var)
		if !ok || !f.Embedded() {
			continue
		}
		ft := types.Unalias(f.Type())
		if p, isPtr := ft.(*types.Pointer); isPtr {
			ft = p.Elem()
		}
		if ft != root.Type() {
			continue // does not embed root (or not under that name)
		}
		stubDepth := len(fidx) // root's method is one level below the embedded field
		o, idx, _ := types.LookupFieldOrMethod(types.NewPointer(tn.Type()), true, rw.Pkg, m)
		if o == nil {
			if idx != nil {
				return tn.Name() + " has an ambiguous " + m
			}
			continue
		}
		fn, ok := o.(*types.Func)
		if !ok {
			return tn.Name() + " has a field " + m
		}
		if len(idx)-1 >= stubDepth {
			return fmt.Sprintf("%s finds %s at depth %d, %s's at %d", tn.Name(), m, len(idx)-1, root.Name(), stubDepth)
		}
		if !types.Identical(fn.Type(), sig) {
			return tn.Name() + "'s " + m + " has another signature"
		}
	}
	return ""
}

// topmost are the classes of set that embed no other class of set, sorted.
func topmost(set map[string]bool) []string {
	var out []string
	for c := range set {
		tn, _ := rw.Pkg.Scope().Lookup(c).(*types.TypeName)
		top := true
		for d := range set {
			if d != c && tn != nil {
				if o, _, _ := types.LookupFieldOrMethod(tn.Type(), false, rw.Pkg, d); o != nil {
					if v, ok := o.(*types.Var); ok && v.Embedded() {
						top = false
					}
				}
			}
		}
		if top {
			out = append(out, c)
		}
	}
	sort.Strings(out)
	return out
}
