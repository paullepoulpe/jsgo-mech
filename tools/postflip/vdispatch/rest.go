package main

// The -rest phase, run after -retype, -sub and -enums: the roots whose self_ those phases left
// `any`, and the casts that assert to an interface without a name.
//
// -retype types self_ of a root R only when a dispatcher of R's hierarchy asserts self_ to an
// anonymous interface (this.self_.(interface{ m_impl(T) U }).m_impl(p0)): the methods of those
// assertions are R_I. Two kinds of roots have no such assertion and kept `self_ any`:
//
//   - a generic class, whose dispatchers j2go writes through a second field of the object,
//     vt_R_ of the generic interface R_vt_[P...] (this.vt_R_.m_impl(p0)), set beside self_ by
//     the constructors;
//   - a class without a dispatcher of that form: no overridable method that is overridden
//     (ParseTree, whose subclasses are reached by this.self().(*C)), or one implementation
//     only, which step 6 calls by a concrete assertion (this.self_.(*C).m_impl(p0)).
//
// This phase types self_ of every such root R declared in generated code with the hierarchy's
// interface H:
//
//	a root with a field vt_R_   H = the type of vt_R_ (R_vt_[P...]); the field vt_R_ goes:
//	                            its reads read self_, its assignments are deleted
//	any other root              H = R_as (R_as[P...] for a generic R), the interface
//	                            { as_R() *R[P...] } that R and the classes that extend it
//	                            implement; declared here when the package has none
//
// What must hold, and is checked, for self_ typed H to hold the values `self_ any` held and for
// every reader to see what it saw:
//
//  1. Writes. Every write of self_ of R is an assignment statement x.self_ = v in a block of
//     generated code, v of a pointer-to-class type (no composite literal names the field, its
//     address is not taken). For a root without vt_R_, v's type has the method as_R declared on
//     R, with the result type as_R has on x: v implements R_as[args] for the instantiation of R
//     that x embeds. For a root with vt_R_, x and v are one identifier and the statement is
//     followed in its block, with only other assignments x.vt_C_ = x between, by x.vt_R_ = x;
//     every write of vt_R_ is the second statement of such a pair. So v is assignable to H (go/types
//     checked that for vt_R_), and at every point where a reader runs the two fields of one
//     object hold the same value (both unset, or both the object): a read of vt_R_ may read
//     self_ instead.
//  2. Reads. Every read of self_ of R is one of: the operand of a return in a function whose
//     result there is `any` (self(): the conversion to any gives the value the field held as
//     any, nil for nil); the operand of a type assertion or a type switch; an operand of == or
//     !=. Each of these gives the same result for an interface value of type H as for the same
//     dynamic value held as any. Any other read (a copy into a variable, an argument) leaves the
//     root alone. A read of vt_R_ becomes x.self_ only where x.self_ selects R's field.
//  3. The rewritten package type-checks (rw.SaveChecked): an assertion of self_ to a class that
//     does not implement H, or anything else that no longer compiles, writes nothing.
//
// Casts. to_X(v any) of a generic class X is written
//
//	return v.(interface{ as_X_() any }).as_X_().(*X[P...])
//
// (an interface assertion, a call and a second assertion). It becomes v.(X_as[P...]).as_X()
// with the named generic interface X_as[P...] { as_X() *X[P...] }. The two fail for the same
// values (both with a *runtime.TypeAssertionError; its text differs) and give the same pointer
// otherwise, because, checked: as_X and as_X_ are each declared once in the package, both on
// *X, both with the body `return this`, and no interface of the package lists either name and
// no struct field has it; so a type has as_X_ exactly when it has as_X (it embeds one X[A], at
// one depth for both, and embedded types are the only other source of a method), and
// as_X_()'s value is a *X[P...] exactly when as_X's result type is *X[P...]. as_X_ is deleted
// when nothing else names it. A to_X that asserts to another anonymous one-method interface
// (an enum's { as_X() X }) gets the name X_as for that interface, nothing else.
//
// Roots declared in the runtime are left alone. A rerun finds nothing to do.

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

type restRoot struct {
	name    string
	ts      *ast.TypeSpec
	field   *types.Var
	fieldAt *ast.Field
	vt      *types.Var // the field vt_R_, or nil
	vtAt    *ast.Field
	bad     string
	writes  []*ast.AssignStmt // x.self_ = v
	vtw     []*ast.AssignStmt // x.vt_R_ = v
	vtr     []*ast.SelectorExpr
	nread   int
}

type restState struct {
	specs   map[string]*ast.TypeSpec
	funcs   map[string][]*ast.FuncDecl // by name, methods and functions
	idents  map[string]int             // occurrences of an identifier in the package
	imeths  map[string][]*ast.Field    // interface methods written in the package, by name
	fields  map[string]bool            // names of the struct fields of the package
	asDecl  map[string]bool            // X_as declared by this run
	asNote  []string
	nAsDecl int
}

func pos(n ast.Node) string {
	p := rw.Fset.Position(n.Pos())
	return fmt.Sprintf("%s:%d", strings.TrimPrefix(p.Filename, "../../go/closure/"), p.Line)
}

func restMain(report bool) {
	st := &restState{specs: map[string]*ast.TypeSpec{}, funcs: map[string][]*ast.FuncDecl{}, idents: map[string]int{}, imeths: map[string][]*ast.Field{}, fields: map[string]bool{}, asDecl: map[string]bool{}}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.GenDecl:
				for _, sp := range x.Specs {
					if ts, ok := sp.(*ast.TypeSpec); ok {
						st.specs[ts.Name.Name] = ts
					}
				}
			case *ast.FuncDecl:
				st.funcs[x.Name.Name] = append(st.funcs[x.Name.Name], x)
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				st.idents[x.Name]++
			case *ast.StructType:
				for _, fl := range x.Fields.List {
					for _, nm := range fl.Names {
						st.fields[nm.Name] = true
					}
				}
			case *ast.InterfaceType:
				for _, m := range x.Methods.List {
					for _, nm := range m.Names {
						st.imeths[nm.Name] = append(st.imeths[nm.Name], m)
					}
				}
			}
			return true
		})
	}

	ncast, nname, ndead, nleftCast := st.casts()

	// Roots: struct types of generated code declaring a field self_ of type any.
	roots := map[*types.Var]*restRoot{}
	vts := map[*types.Var]*restRoot{}
	var order []*restRoot
	nrt := 0
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
				stt, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				var r *restRoot
				for _, fl := range stt.Fields.List {
					for _, nm := range fl.Names {
						if nm.Name != "self_" {
							continue
						}
						v := rw.Info.Defs[nm].(*types.Var)
						if t, ok := types.Unalias(v.Type()).(*types.Interface); !ok || !t.Empty() {
							continue
						}
						if !generated(gd) {
							nrt++
							continue
						}
						r = &restRoot{name: ts.Name.Name, ts: ts, field: v, fieldAt: fl}
						if len(fl.Names) != 1 {
							r.bad = "self_ declared with another field"
						}
					}
				}
				if r == nil {
					continue
				}
				for _, fl := range stt.Fields.List {
					for _, nm := range fl.Names {
						if nm.Name != "vt_"+r.name+"_" {
							continue
						}
						v := rw.Info.Defs[nm].(*types.Var)
						if _, ok := v.Type().Underlying().(*types.Interface); !ok || len(fl.Names) != 1 {
							r.bad = "vt_" + r.name + "_ is no interface field of its own"
						}
						r.vt, r.vtAt = v, fl
						vts[v] = r
					}
				}
				roots[r.field] = r
				order = append(order, r)
			}
		}
	}
	sort.Slice(order, func(i, j int) bool { return order[i].name < order[j].name })

	// Every use of the two fields.
	fail := func(r *restRoot, format string, a ...any) {
		if r.bad == "" {
			r.bad = fmt.Sprintf(format, a...)
		}
	}
	for _, r := range order {
		for _, id := range rw.UsesOf[r.field] {
			sel, ok := rw.Parent[id].(*ast.SelectorExpr)
			if !ok || sel.Sel != id {
				fail(r, "self_ named outside a selector at %s", pos(id))
				continue
			}
			switch p := rw.Up(sel).(type) {
			case *ast.AssignStmt:
				if onLeft(p, sel) {
					if why := plainWrite(p); why != "" {
						fail(r, "self_ %s at %s", why, pos(p))
						continue
					}
					r.writes = append(r.writes, p)
					continue
				}
				fail(r, "self_ copied at %s", pos(p))
			case *ast.ReturnStmt:
				if !returnsAny(p, sel) {
					fail(r, "self_ returned as another type than any at %s", pos(p))
				}
				r.nread++
			case *ast.TypeAssertExpr:
				r.nread++
			case *ast.BinaryExpr:
				if p.Op != token.EQL && p.Op != token.NEQ {
					fail(r, "self_ read at %s", pos(p))
				}
				r.nread++
			default:
				fail(r, "self_ read at %s", pos(sel))
			}
		}
		if r.vt == nil {
			continue
		}
		for _, id := range rw.UsesOf[r.vt] {
			sel, ok := rw.Parent[id].(*ast.SelectorExpr)
			if !ok || sel.Sel != id {
				fail(r, "%s named outside a selector at %s", id.Name, pos(id))
				continue
			}
			up := rw.Up(sel)
			if p, ok := up.(*ast.AssignStmt); ok && onLeft(p, sel) {
				if why := plainWrite(p); why != "" {
					fail(r, "%s %s at %s", id.Name, why, pos(p))
					continue
				}
				r.vtw = append(r.vtw, p)
				continue
			}
			if u, ok := up.(*ast.UnaryExpr); ok && u.Op == token.AND {
				fail(r, "%s address taken at %s", id.Name, pos(u))
				continue
			}
			// the read becomes x.self_: that must select this root's field
			o, _, _ := types.LookupFieldOrMethod(rw.Info.Types[sel.X].Type, true, rw.Pkg, "self_")
			if v, ok := o.(*types.Var); !ok || v.Origin() != r.field {
				fail(r, "self_ of %s is not the root's at %s", rw.Text(sel.X), pos(sel))
				continue
			}
			r.vtr = append(r.vtr, sel)
		}
	}

	// The writes: obligation 1.
	for _, r := range order {
		if r.bad != "" {
			continue
		}
		if r.vt != nil {
			paired := map[*ast.AssignStmt]bool{}
			for _, w := range r.writes {
				p := pairOf(w, r.vt)
				if p == nil {
					fail(r, "self_ set at %s without vt_%s_ set to the same object after it", pos(w), r.name)
					break
				}
				paired[p] = true
			}
			for _, w := range r.vtw {
				if !paired[w] {
					fail(r, "vt_%s_ set at %s without self_ set to the same object before it", r.name, pos(w))
				}
			}
			continue
		}
		for _, w := range r.writes {
			x := rw.Unparen(w.Lhs[0]).(*ast.SelectorExpr).X
			as := "as_" + r.name
			ox, _, _ := types.LookupFieldOrMethod(rw.Info.Types[x].Type, true, rw.Pkg, as)
			ov, _, _ := types.LookupFieldOrMethod(rw.Info.Types[w.Rhs[0]].Type, true, rw.Pkg, as)
			fx, ok1 := ox.(*types.Func)
			fv, ok2 := ov.(*types.Func)
			if !ok1 || !ok2 || !st.declaredOn(fx, r.name) || fx.Origin() != fv.Origin() || !types.Identical(fx.Type(), fv.Type()) {
				fail(r, "self_ set at %s to a value that is no %s_as of the object", pos(w), r.name)
			}
		}
	}

	// Rewrites.
	nvt, nas, nvtr, nvtw := 0, 0, 0, 0
	var rows []string
	for _, r := range order {
		if r.bad != "" {
			continue
		}
		var h string
		if r.vt != nil {
			h = rw.Text(r.vtAt.Type)
		} else {
			var why string
			if h, why = st.asIface(r.name); why != "" {
				r.bad = why
				continue
			}
		}
		fieldAt := r.fieldAt
		rw.Rewrite[fieldAt.Type] = func() string { return h }
		kind := "as"
		if r.vt != nil {
			kind = "vt"
			nvt++
			rw.Rewrite[r.vtAt] = func() string { return rw.Del }
			for _, w := range r.vtw {
				rw.Rewrite[w] = func() string { return rw.Del }
				nvtw++
			}
			for _, s := range r.vtr {
				rw.Rewrite[s.Sel] = func() string { return "self_" }
				nvtr++
			}
		} else {
			nas++
		}
		rows = append(rows, fmt.Sprintf("%s\t%s\tself_ %s\t%d writes, %d reads of self_; %d reads, %d writes of vt", r.name, kind, oneLine(h), len(r.writes), r.nread, len(r.vtr), len(r.vtw)))
	}
	left := 0
	for _, r := range order {
		if r.bad != "" {
			left++
			fmt.Fprintf(os.Stderr, "left alone: %s: %s\n", r.name, r.bad)
		}
	}
	for _, n := range st.asNote {
		fmt.Fprintln(os.Stderr, n)
	}
	if report {
		for _, row := range rows {
			fmt.Println(row)
		}
	}
	fmt.Fprintf(os.Stderr, "roots with self_ any: %d in generated code (%d in the runtime, not looked at); typed: %d with the type of their vt field (%d reads of it read self_, %d assignments and %d fields deleted), %d with X_as; left: %d\n",
		len(order), nrt, nvt, nvtr, nvtw, nvt, nas, left)
	fmt.Fprintf(os.Stderr, "casts to_X through an anonymous interface: %d; generic, now one assertion to X_as[P...]: %d (%d as_X_ methods deleted); named: %d; left: %d; X_as interfaces declared: %d\n",
		ncast+nname+nleftCast, ncast, ndead, nname, nleftCast, st.nAsDecl)
	if !report {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
}

func oneLine(s string) string { return strings.Join(strings.Fields(s), " ") }

func onLeft(a *ast.AssignStmt, e ast.Expr) bool {
	for _, l := range a.Lhs {
		if rw.Unparen(l) == e {
			return true
		}
	}
	return false
}

// plainWrite says why a is not a statement `x.f = v` of generated code in a block, v a pointer
// to a named type ("" when it is one).
func plainWrite(a *ast.AssignStmt) string {
	if len(a.Lhs) != 1 || len(a.Rhs) != 1 || a.Tok != token.ASSIGN || !generated(a) {
		return "written by something else than a plain assignment of generated code"
	}
	if _, ok := rw.Parent[a].(*ast.BlockStmt); !ok {
		return "written outside a block"
	}
	t := rw.Info.Types[a.Rhs[0]].Type
	if p, ok := t.(*types.Pointer); !ok || !isNamed(p.Elem()) {
		return fmt.Sprintf("set to a %s", t)
	}
	return ""
}

// returnsAny: sel is a result of r, and the enclosing function's result there is `any`.
func returnsAny(r *ast.ReturnStmt, sel ast.Expr) bool {
	i := -1
	for k, e := range r.Results {
		if rw.Unparen(e) == sel {
			i = k
		}
	}
	var sig *types.Signature
	for n := rw.Parent[r]; n != nil && sig == nil; n = rw.Parent[n] {
		switch f := n.(type) {
		case *ast.FuncDecl:
			sig, _ = rw.Info.Defs[f.Name].Type().(*types.Signature)
		case *ast.FuncLit:
			sig, _ = rw.Info.Types[f].Type.(*types.Signature)
		}
	}
	if i < 0 || sig == nil || sig.Results().Len() != len(r.Results) {
		return false
	}
	t, ok := types.Unalias(sig.Results().At(i).Type()).(*types.Interface)
	return ok && t.Empty()
}

// pairOf: w is `x.self_ = x`; the result is the statement `x.vt_R_ = x` (vt the field) that
// follows it in its block with only assignments `x.vt_C_ = x` between, or nil.
func pairOf(w *ast.AssignStmt, vt *types.Var) *ast.AssignStmt {
	x, ok := rw.Unparen(w.Lhs[0]).(*ast.SelectorExpr).X.(*ast.Ident)
	v, ok2 := w.Rhs[0].(*ast.Ident)
	if !ok || !ok2 || rw.Info.Uses[x] == nil || rw.Info.Uses[x] != rw.Info.Uses[v] {
		return nil
	}
	obj := rw.Info.Uses[x]
	list := rw.Parent[w].(*ast.BlockStmt).List
	i := 0
	for list[i] != ast.Stmt(w) {
		i++
	}
	for _, s := range list[i+1:] {
		a, ok := s.(*ast.AssignStmt)
		if !ok || len(a.Lhs) != 1 || len(a.Rhs) != 1 || a.Tok != token.ASSIGN {
			return nil
		}
		sel, ok := a.Lhs[0].(*ast.SelectorExpr)
		if !ok || !strings.HasPrefix(sel.Sel.Name, "vt_") {
			return nil
		}
		ax, ok := sel.X.(*ast.Ident)
		av, ok2 := a.Rhs[0].(*ast.Ident)
		if !ok || !ok2 || rw.Info.Uses[ax] != obj || rw.Info.Uses[av] != obj {
			return nil
		}
		if rw.Info.Uses[sel.Sel] == types.Object(vt) {
			return a
		}
	}
	return nil
}

// declaredOn: f is (an instance of) the one method of that name in the package, declared on *class.
func (st *restState) declaredOn(f *types.Func, class string) bool {
	ds := st.funcs[f.Name()]
	return len(ds) == 1 && recvClass(ds[0]) == class && rw.Info.Defs[ds[0].Name] == types.Object(f.Origin())
}

// recvClass is C for a method declared on *C or *C[P...] ("" otherwise).
func recvClass(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) != 1 {
		return ""
	}
	s, ok := fd.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return ""
	}
	switch t := s.X.(type) {
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	case *ast.IndexListExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			return id.Name
		}
	}
	return ""
}

// returnsThis: the method's body is `return <receiver>`.
func returnsThis(fd *ast.FuncDecl) bool {
	if fd.Body == nil || len(fd.Body.List) != 1 || len(fd.Recv.List[0].Names) != 1 {
		return false
	}
	r, ok := fd.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(r.Results) != 1 {
		return false
	}
	id, ok := r.Results[0].(*ast.Ident)
	return ok && id.Name == fd.Recv.List[0].Names[0].Name
}

// params is the type parameter list of class X as written ("" for a class that is not generic)
// and the list of its names in brackets.
func (st *restState) params(x string) (decl, args string) {
	ts := st.specs[x]
	if ts == nil || ts.TypeParams == nil {
		return "", ""
	}
	var names []string
	for _, f := range ts.TypeParams.List {
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
	}
	return rw.Text(ts.TypeParams), "[" + strings.Join(names, ", ") + "]"
}

// asIface is the interface X_as (X_as[P...]) as written in X's declaration scope: the one the
// package has, when it is { as_X() *X[P...] }, or one this run declares above the method as_X.
func (st *restState) asIface(x string) (text, why string) {
	name := x + "_as"
	decl, args := st.params(x)
	if st.asDecl[name] {
		return name + args, ""
	}
	ts := st.specs[x]
	if ts == nil || !generated(ts) {
		return "", "no class declaration of generated code"
	}
	ds := st.funcs["as_"+x]
	if len(ds) != 1 || recvClass(ds[0]) != x || !generated(ds[0]) {
		return "", "as_" + x + " is not declared once, on the class"
	}
	fd := ds[0]
	sig := rw.Info.Defs[fd.Name].Type().(*types.Signature)
	if sig.Params().Len() != 0 || sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), sig.Recv().Type()) {
		return "", "as_" + x + " does not return its receiver's type"
	}
	if o := rw.Pkg.Scope().Lookup(name); o != nil {
		// declared before this run: it must be the interface this run would declare
		tn, ok := o.(*types.TypeName)
		its := st.specs[name]
		if !ok || its == nil {
			return "", name + " is declared and is no type"
		}
		it, ok := its.Type.(*ast.InterfaceType)
		if !ok || len(it.Methods.List) != 1 || len(it.Methods.List[0].Names) != 1 || it.Methods.List[0].Names[0].Name != "as_"+x {
			return "", name + " is declared and is not { as_" + x + "() }"
		}
		idecl, _ := st.params(name)
		if oneLine(idecl) != oneLine(decl) {
			return "", name + " is declared with other type parameters"
		}
		if _, ok := tn.Type().Underlying().(*types.Interface); !ok {
			return "", name + " is declared and is no interface"
		}
		if got, want := oneLine(rw.Text(it.Methods.List[0].Type)), "() *"+x+args; got != want {
			return "", fmt.Sprintf("%s is declared with as_%s%s", name, x, got)
		}
		return name + args, ""
	}
	st.asDecl[name] = true
	st.nAsDecl++
	idecl := fmt.Sprintf("// %s is implemented by %s and the classes that extend it: a cast to %s asserts to it\n// (post-flip).\ntype %s%s interface {\n\tas_%s() *%s%s\n}\n\n", name, x, x, name, decl, x, x, args)
	rw.Rewrite[fd] = func() string { return idecl + rw.RenderChildren(fd) }
	return name + args, ""
}

// casts rewrites the to_X functions (see the file comment) and returns the counts: generic casts
// rewritten, interfaces only named, as_X_ methods deleted, casts left.
func (st *restState) casts() (ncast, nname, ndead, nleft int) {
	var fds []*ast.FuncDecl
	for name, ds := range st.funcs {
		if strings.HasPrefix(name, "to_") && len(ds) == 1 && ds[0].Recv == nil && ds[0].Body != nil && generated(ds[0]) {
			fds = append(fds, ds[0])
		}
	}
	sort.Slice(fds, func(i, j int) bool { return fds[i].Name.Name < fds[j].Name.Name })
	for _, fd := range fds {
		x := strings.TrimPrefix(fd.Name.Name, "to_")
		// the anonymous interface assertions of the body: one, of the parameter, in the last return
		var tas []*ast.TypeAssertExpr
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if ta, ok := n.(*ast.TypeAssertExpr); ok {
				if _, ok := ta.Type.(*ast.InterfaceType); ok {
					tas = append(tas, ta)
				}
			}
			return true
		})
		if len(tas) == 0 {
			continue
		}
		left := func(why string) {
			nleft++
			st.asNote = append(st.asNote, fmt.Sprintf("cast left: %s: %s", fd.Name.Name, why))
		}
		ret, ok := fd.Body.List[len(fd.Body.List)-1].(*ast.ReturnStmt)
		if len(tas) != 1 || !ok || len(ret.Results) != 1 || fd.Type.Params.NumFields() != 1 || len(fd.Type.Params.List[0].Names) != 1 {
			left("not one assertion in a final return")
			continue
		}
		ta := tas[0]
		it := ta.Type.(*ast.InterfaceType)
		v, ok := ta.X.(*ast.Ident)
		if !ok || v.Name != fd.Type.Params.List[0].Names[0].Name || len(it.Methods.List) != 1 || len(it.Methods.List[0].Names) != 1 {
			left("the assertion is not of the parameter to a one-method interface")
			continue
		}
		m := it.Methods.List[0].Names[0].Name
		call, ok := rw.Parent[rw.Parent[ta]].(*ast.CallExpr)
		if s, ok2 := rw.Parent[ta].(*ast.SelectorExpr); !ok || !ok2 || s.X != ast.Expr(ta) || s.Sel.Name != m || call.Fun != ast.Expr(s) || len(call.Args) != 0 {
			left("the asserted value is not called at once")
			continue
		}
		if fd.Type.TypeParams == nil {
			// naming only
			if ret.Results[0] != ast.Expr(call) {
				left("the call is not the result")
				continue
			}
			name := x + "_as"
			if rw.Pkg.Scope().Lookup(name) != nil || st.asDecl[name] {
				left(name + " is declared")
				continue
			}
			st.asDecl[name] = true
			st.nAsDecl++
			text := fmt.Sprintf("// %s is what a cast to %s asserts to (post-flip).\ntype %s interface {\n\t%s\n}\n\n", name, x, name, rw.Text(it.Methods.List[0]))
			rw.Rewrite[fd] = func() string { return text + rw.RenderChildren(fd) }
			rw.Rewrite[it] = func() string { return name }
			nname++
			continue
		}
		// generic: return v.(interface{ as_X_() any }).as_X_().(*X[P...])
		decl, args := st.params(x)
		outer, ok := ret.Results[0].(*ast.TypeAssertExpr)
		if !ok || outer.X != ast.Expr(call) || outer.Type == nil || m != "as_"+x+"_" || decl == "" {
			left("not the cast of a generic class")
			continue
		}
		var own []string
		for _, f := range fd.Type.TypeParams.List {
			for _, n := range f.Names {
				own = append(own, n.Name)
			}
		}
		if oneLine(rw.Text(outer.Type)) != "*"+x+"["+strings.Join(own, ", ")+"]" || "["+strings.Join(own, ", ")+"]" != args {
			left("the result is not the class of the function's own type parameters")
			continue
		}
		why := ""
		for _, mn := range []string{"as_" + x, m} {
			ds := st.funcs[mn]
			if len(ds) != 1 || recvClass(ds[0]) != x || !returnsThis(ds[0]) {
				why = mn + " is not declared once, on the class, returning its receiver"
			}
		}
		// no interface of the package lists either method (this function's own aside)
		if why == "" && (len(st.imeths["as_"+x]) != 0 || len(st.imeths[m]) != 1) {
			why = "an interface of the package lists as_" + x + " or " + m
		}
		if why == "" && (st.fields["as_"+x] || st.fields[m]) {
			why = "a struct field is named as_" + x + " or " + m
		}
		if why == "" {
			if r, _ := rw.Info.Defs[st.funcs[m][0].Name].Type().(*types.Signature); r == nil || r.Results().Len() != 1 || !isAny(r.Results().At(0).Type()) || r.Params().Len() != 0 {
				why = m + " does not return any"
			}
		}
		var iface string
		if why == "" {
			iface, why = st.asIface(x)
		}
		if why != "" {
			left(why)
			continue
		}
		rw.Rewrite[outer] = func() string { return v.Name + ".(" + iface + ").as_" + x + "()" }
		ncast++
		// as_X_: its declaration, the interface's method and the call were its three names
		if st.idents[m] == 3 {
			dead := st.funcs[m][0]
			rw.Rewrite[dead] = func() string { return rw.DelDoc }
			ndead++
		}
	}
	return
}

func isAny(t types.Type) bool {
	i, ok := types.Unalias(t).(*types.Interface)
	return ok && i.Empty()
}
