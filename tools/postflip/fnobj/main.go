// Command fnobj is a post-flip step: a Java functional interface that only lambdas implement is
// a typed function value with an identity, *funcObj_[F] (rt_core.go), instead of an interface
// and j2go's adapter class.
//
// j2go writes a lambda of a functional interface X (one method m) as &X_fn{func...}: X_fn is a
// struct holding the Go function, whose method m calls it. When X_fn is the only type of the
// package that implements X (no class implements it, the runtime neither implements nor uses
// it), every non-nil X is an *X_fn, so X can be the pointer type itself, with the function
// typed: `type X = *funcObj_[F]`, F the signature of m. Then
//
//	&X_fn{f}     ->  newFuncObj_(f)        the object made where Java made the lambda object
//	x.m(args)    ->  x.call(args)          the same function, without an interface call
//	x.m          ->  x.call                (a method value)
//
// and X_fn with its method goes. Identity is that of the object, as before: x == y compares the
// pointers where it compared interfaces holding the pointers, a nil X is a nil pointer, a
// package-level lambda (the lambdas step) is still one object, and hashing or printing the
// object by identity sees one object per evaluation of the lambda, as in Java.
//
// What changes meaning with X a pointer type is the conversion of a nil X to another interface
// (any, a boxing helper's argument): the interface would hold a typed nil. A value of static
// type X converted to Object (any), explicitly or implicitly (assignment, argument, return,
// composite element, comparison), is boxed with obj_, which is nil for a nil pointer; X is
// kept when a value is converted to another, non-empty interface type (a lambda object, never
// nil, may be). Type assertions and switches on an X, and uses of X_fn other
// than &X_fn{f}, keep X as it is. Each interface that is kept is reported with its reason.
//
// usage: fnobj [-n] <dir of package closure>
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

type cand struct {
	name   string
	iface  *types.TypeName
	fn     *types.TypeName // X_fn
	method *types.Func     // m
	sig    string          // F
	decl   *ast.TypeSpec
	why    string
	lits   int
}

func generated(pos token.Pos) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(pos).Filename), "g_")
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	sc := rw.Pkg.Scope()
	var named []*types.Named
	for _, n := range sc.Names() {
		if tn, ok := sc.Lookup(n).(*types.TypeName); ok && !tn.IsAlias() {
			if nt, ok := tn.Type().(*types.Named); ok {
				named = append(named, nt)
			}
		}
	}
	specs := map[*types.TypeName]*ast.TypeSpec{}
	funcDecls := map[*types.Func]*ast.FuncDecl{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.GenDecl:
				for _, s := range x.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok {
						if tn, ok := rw.Info.Defs[ts.Name].(*types.TypeName); ok {
							specs[tn] = ts
						}
					}
				}
			case *ast.FuncDecl:
				funcDecls[rw.Info.Defs[x.Name].(*types.Func)] = x
			}
		}
	}
	cands := map[*types.TypeName]*cand{}
	byFn := map[*types.TypeName]*cand{}
	for _, nt := range named {
		fnName := nt.Obj().Name()
		if !strings.HasSuffix(fnName, "_fn") {
			continue
		}
		io, ok := sc.Lookup(strings.TrimSuffix(fnName, "_fn")).(*types.TypeName)
		if !ok || io.IsAlias() || !generated(io.Pos()) || !generated(nt.Obj().Pos()) {
			continue
		}
		it, ok := io.Type().Underlying().(*types.Interface)
		if !ok || it.NumMethods() != 1 || it.NumEmbeddeds() != 0 {
			continue
		}
		c := &cand{name: io.Name(), iface: io, fn: nt.Obj(), method: it.Method(0), decl: specs[io]}
		cands[io] = c
		byFn[nt.Obj()] = c
		st, ok := nt.Underlying().(*types.Struct)
		if !ok || st.NumFields() != 1 || st.Field(0).Name() != "f" || !types.Identical(st.Field(0).Type(), c.method.Type().(*types.Signature)) {
			c.why = "its adapter is not a struct holding the function"
			continue
		}
		c.sig = types.TypeString(st.Field(0).Type(), func(p *types.Package) string { return "" })
		for _, o := range named {
			if o == nt {
				continue
			}
			if _, isI := o.Underlying().(*types.Interface); isI {
				continue
			}
			if types.Implements(o, it) || types.Implements(types.NewPointer(o), it) {
				c.why = "a class implements it too (" + o.Obj().Name() + ")"
				break
			}
		}
	}
	isX := func(t types.Type) *cand {
		if t == nil {
			return nil
		}
		if n, ok := types.Unalias(t).(*types.Named); ok {
			return cands[n.Obj()]
		}
		return nil
	}
	fail := func(c *cand, why string, n ast.Node) {
		if c.why == "" {
			p := rw.Fset.Position(n.Pos())
			c.why = fmt.Sprintf("%s (%s:%d)", why, filepath.Base(p.Filename), p.Line)
		}
	}
	// Uses of the type names.
	litOf := map[*ast.CompositeLit]*cand{}
	for id, o := range rw.Info.Uses {
		tn, ok := o.(*types.TypeName)
		if !ok {
			continue
		}
		if c := cands[tn]; c != nil && !generated(id.Pos()) {
			fail(c, "the runtime uses it", id)
		}
		c := byFn[tn]
		if c == nil {
			continue
		}
		if !generated(id.Pos()) {
			fail(c, "the runtime uses its adapter", id)
			continue
		}
		cl, ok := rw.Parent[id].(*ast.CompositeLit)
		if ok && cl.Type == ast.Expr(id) && len(cl.Elts) == 1 {
			if u, ok := rw.Parent[cl].(*ast.UnaryExpr); ok && u.Op == token.AND {
				if _, kv := cl.Elts[0].(*ast.KeyValueExpr); !kv {
					litOf[cl] = c
					continue
				}
			}
		}
		// the adapter's own method declaration
		if fd, ok := enclosingDecl(id).(*ast.FuncDecl); ok && fd.Recv != nil && rw.Info.Defs[fd.Name] != nil {
			if m := rw.Info.Defs[fd.Name].(*types.Func); m.Name() == c.method.Name() && recvIs(m, tn) {
				continue
			}
		}
		fail(c, "its adapter is used otherwise", id)
	}
	// Values of static type X: conversions to other interfaces, assertions, method calls.
	calls := map[*ast.SelectorExpr]*cand{}
	boxes := map[ast.Expr]*cand{}
	for e, tv := range rw.Info.Types {
		c := isX(tv.Type)
		if c == nil || tv.IsType() {
			continue
		}
		if sel, ok := rw.Parent[e].(*ast.SelectorExpr); ok && sel.X == e {
			if sel.Sel.Name == c.method.Name() {
				calls[sel] = c
				continue
			}
		}
		if ta, ok := rw.Parent[e].(*ast.TypeAssertExpr); ok && ta.X == e {
			fail(c, "a value is type-asserted", e)
			continue
		}
		if ts, ok := rw.Parent[e].(*ast.TypeSwitchStmt); ok {
			_ = ts
			fail(c, "a value is type-switched", e)
			continue
		}
		if target := targetType(e); target != nil && !isNeverNil(e) {
			if it, iface := target.Underlying().(*types.Interface); iface && isX(target) != c {
				if it.Empty() {
					boxes[e] = c // obj_(e): nil stays nil
				} else {
					fail(c, "a value is converted to "+types.TypeString(target, func(*types.Package) string { return "" }), e)
				}
			}
		}
	}
	for cl, c := range litOf {
		if !types.Identical(rw.Info.Types[cl.Elts[0]].Type, c.method.Type().(*types.Signature)) {
			// a named function type: written with the type argument
			_ = cl
		}
		c.lits++
	}
	// Two interfaces of one signature would become one type: an Object holding a lambda of one
	// would cast to the other, where Java throws ClassCastException. The first by name is kept.
	bySig := map[string][]*cand{}
	for _, c := range cands {
		if c.why == "" {
			bySig[c.sig] = append(bySig[c.sig], c)
		}
	}
	for _, cs := range bySig {
		sort.Slice(cs, func(i, j int) bool { return cs[i].name < cs[j].name })
		for _, c := range cs[1:] {
			c.why = "another interface (" + cs[0].name + ") has its signature"
		}
	}
	// Report.
	var keys []string
	for _, c := range cands {
		keys = append(keys, c.name)
	}
	sort.Strings(keys)
	nconv, nlits, ncalls := 0, 0, 0
	for _, k := range keys {
		c := cands[sc.Lookup(k).(*types.TypeName)]
		if c.why != "" {
			fmt.Printf("  left: %-55s %3d lambdas: %s\n", c.name, c.lits, c.why)
			continue
		}
		fmt.Printf("  %-61s %3d lambdas: converted\n", c.name, c.lits)
		nconv++
		nlits += c.lits
	}
	if report {
		fmt.Printf("fnobj: %d interfaces, %d lambdas\n", nconv, nlits)
		return
	}
	// Rewrites.
	for cl, c := range litOf {
		if c.why != "" {
			continue
		}
		cl, c := cl, c
		u := rw.Parent[cl].(*ast.UnaryExpr)
		x := cl.Elts[0]
		if types.Identical(rw.Info.Types[x].Type, c.method.Type().(*types.Signature)) {
			rw.Rewrite[u] = func() string { return "newFuncObj_(" + rw.Render(x) + ")" }
		} else {
			rw.Rewrite[u] = func() string { return "newFuncObj_[" + c.sig + "](" + rw.Render(x) + ")" }
		}
	}
	nbox := 0
	for e, c := range boxes {
		if c.why != "" {
			continue
		}
		e := e
		nbox++
		// any(e) -> obj_(e); e given to an Object slot -> obj_(e)
		if conv, ok := rw.Parent[e].(*ast.CallExpr); ok && len(conv.Args) == 1 {
			if tv, ok := rw.Info.Types[conv.Fun]; ok && tv.IsType() {
				rw.Rewrite[conv] = func() string { return "obj_(" + rw.Render(e) + ")" }
				continue
			}
		}
		rw.Rewrite[e] = func() string { return "obj_(" + rw.RenderChildren(e) + ")" }
	}
	for sel, c := range calls {
		if c.why != "" {
			continue
		}
		ncalls++
		s := sel.Sel
		rw.Rewrite[s] = func() string { return "call" }
	}
	for _, c := range cands {
		if c.why != "" {
			continue
		}
		c := c
		rw.Rewrite[c.decl.Type] = func() string { return "= *funcObj_[" + c.sig + "]" }
		// X and its adapter.
		if fs := specs[c.fn]; fs != nil {
			g := enclosingGen(fs)
			rw.Rewrite[g] = func() string { return rw.DelDoc }
		}
		for m, fd := range funcDecls {
			if fd.Recv != nil && m.Name() == c.method.Name() && recvIs(m, c.fn) {
				rw.Rewrite[fd] = func() string { return rw.DelDoc }
			}
		}
	}
	fmt.Printf("fnobj: %d interfaces, %d lambdas, %d calls, %d values boxed with obj_ (nil stays nil); files changed: %d\n", nconv, nlits, ncalls, nbox, rw.SaveChecked())
}

func recvIs(m *types.Func, tn *types.TypeName) bool {
	r := m.Type().(*types.Signature).Recv()
	if r == nil {
		return false
	}
	t := r.Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := t.(*types.Named)
	return ok && n.Obj() == tn
}

func enclosingDecl(n ast.Node) ast.Node {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch p.(type) {
		case *ast.FuncDecl, *ast.GenDecl:
			return p
		}
	}
	return nil
}

func enclosingGen(ts *ast.TypeSpec) *ast.GenDecl {
	return rw.Parent[ts].(*ast.GenDecl)
}

// isNeverNil: a lambda object, or a package-level variable holding one.
func isNeverNil(e ast.Expr) bool {
	e = rw.Unparen(e)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		return true
	}
	return false
}

// targetType is the type the value of e is converted to where it is used, or nil.
func targetType(e ast.Expr) types.Type {
	p := rw.Parent[e]
	switch x := p.(type) {
	case *ast.ParenExpr:
		return targetType(x)
	case *ast.CallExpr:
		if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
			return tv.Type // a conversion
		}
		sig, ok := types.Unalias(rw.Info.Types[x.Fun].Type).Underlying().(*types.Signature)
		if !ok {
			return nil // a builtin
		}
		for i, a := range x.Args {
			if a != e {
				continue
			}
			if sig.Variadic() && i >= sig.Params().Len()-1 {
				if x.Ellipsis.IsValid() {
					return sig.Params().At(sig.Params().Len() - 1).Type()
				}
				return sig.Params().At(sig.Params().Len() - 1).Type().(*types.Slice).Elem()
			}
			return sig.Params().At(i).Type()
		}
	case *ast.AssignStmt:
		for i, r := range x.Rhs {
			if r == e && len(x.Lhs) == len(x.Rhs) {
				return rw.Info.Types[x.Lhs[i]].Type
			}
		}
	case *ast.ValueSpec:
		for i, v := range x.Values {
			if v == e {
				if o := rw.Info.Defs[x.Names[i]]; o != nil {
					return o.Type()
				}
			}
		}
	case *ast.ReturnStmt:
		var ft *ast.FuncType
		for q := rw.Parent[x]; q != nil; q = rw.Parent[q] {
			if fd, ok := q.(*ast.FuncDecl); ok {
				ft = fd.Type
				break
			}
			if fl, ok := q.(*ast.FuncLit); ok {
				ft = fl.Type
				break
			}
		}
		if ft == nil {
			return nil
		}
		var rs []types.Type
		for _, f := range ft.Results.List {
			t := rw.Info.Types[f.Type].Type
			n := len(f.Names)
			if n == 0 {
				n = 1
			}
			for k := 0; k < n; k++ {
				rs = append(rs, t)
			}
		}
		for i, r := range x.Results {
			if r == e && i < len(rs) {
				return rs[i]
			}
		}
	case *ast.KeyValueExpr:
		if x.Value == e {
			if cl, ok := rw.Parent[x].(*ast.CompositeLit); ok {
				return eltType(cl, x)
			}
		}
	case *ast.CompositeLit:
		return eltType(x, e)
	case *ast.BinaryExpr:
		if x.Op == token.EQL || x.Op == token.NEQ {
			o := x.X
			if o == e {
				o = x.Y
			}
			if rw.Text(o) == "nil" {
				return nil
			}
			return rw.Info.Types[o].Type
		}
	case *ast.SendStmt:
		return nil
	}
	return nil
}

func eltType(cl *ast.CompositeLit, x ast.Node) types.Type {
	t := types.Unalias(rw.Info.Types[cl].Type)
	switch u := t.Underlying().(type) {
	case *types.Slice:
		return u.Elem()
	case *types.Array:
		return u.Elem()
	case *types.Map:
		if kv, ok := x.(*ast.KeyValueExpr); ok {
			_ = kv
			return u.Elem()
		}
	case *types.Struct:
		if kv, ok := x.(*ast.KeyValueExpr); ok {
			if id, ok := kv.Key.(*ast.Ident); ok {
				for i := 0; i < u.NumFields(); i++ {
					if u.Field(i).Name() == id.Name {
						return u.Field(i).Type()
					}
				}
			}
		} else {
			for i, el := range cl.Elts {
				if el == x && i < u.NumFields() {
					return u.Field(i).Type()
				}
			}
		}
	}
	return nil
}
