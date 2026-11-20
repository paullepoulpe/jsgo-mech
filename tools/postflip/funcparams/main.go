// Command funcparams is a post-flip step: a parameter of a function-interface type that the
// function only calls becomes a Go func value, so a lambda passed to it is not an object.
//
// j2go writes a Java lambda as a function-interface object holding a Go function literal,
// &F_fn{func(...) {...}}. Passed to a parameter of interface type, the object escapes (the
// compiler cannot see through the interface call), so the object and the closure it holds
// are both allocated on the heap each time the lambda is evaluated. When the parameter is only
// called inside the function (p.apply(x)), or passed on to another such parameter, this step
// gives it the Go function type of the method (func(any) any) and writes the calls p(x); at
// the call sites the lambda is the function literal itself, which the compiler keeps on the
// stack when the function does not let it escape. Java's behaviour is the same: the callee
// could only call the lambda, never compare it, store it or hand it on, so it cannot tell a
// function value from the object.
//
// A parameter is converted when
//   - it has a function-interface type (a non-empty interface with one method) and every use
//     in the body calls that method or passes it to a converted parameter;
//   - the function is declared in a generated file, is not generic, and is only ever called
//     (never used as a value); a method also must not be one that any interface asks for;
//   - every call passes a lambda literal &F_fn{func...} (F_fn's method calls its function),
//     a package-level lambda (the lambdas step), or a converted parameter. A call that passes
//     any other value (a field, a local, a class implementing the interface) keeps the
//     parameter an interface: there the value could be null, which Java reports only when
//     it is called.
//
// A local variable of a function-interface type is converted the same way when every value
// assigned to it is one of those and every use calls it or passes it to a converted parameter
// (TypeValidator.expectCanAssignToPropertyOf picks one of two lambdas this way).
//
// The handwritten runtime offers variants taking a func value for its functions of this kind:
// X_f beside X (Map_computeIfAbsent_f, TMap.computeIfAbsent_f, ...), with the function-
// interface parameters as Go function types. A call of X whose function-interface arguments
// are all lambda literals or package-level lambdas becomes a call of X_f.
//
// usage: funcparams [-n] <dir of package closure>   (-n: report, change nothing)
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

var declOf = map[*types.Func]*ast.FuncDecl{}

// ifaces are the interface types of the package: named ones and those written in place.
var ifaces []*types.Interface

// named are the package's named non-interface types.
var named []*types.Named

func main() {
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				if o, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					declOf[o] = fd
				}
			}
		}
	}
	seen := map[*types.Interface]bool{}
	addIface := func(t types.Type) {
		if it, ok := t.Underlying().(*types.Interface); ok && !seen[it] && it.NumMethods() > 0 {
			seen[it] = true
			ifaces = append(ifaces, it)
		}
	}
	sc := rw.Pkg.Scope()
	for _, n := range sc.Names() {
		tn, ok := sc.Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		nt, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		if _, ok := nt.Underlying().(*types.Interface); ok {
			addIface(nt)
		} else {
			named = append(named, nt)
		}
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if it, ok := n.(*ast.InterfaceType); ok {
				if tv, ok := rw.Info.Types[it]; ok {
					addIface(tv.Type)
				}
			}
			return true
		})
	}
	convert(os.Args[1] == "-n")
}

// key is a parameter of a function, or a local variable (fn nil).
type key struct {
	fn  *types.Func
	idx int
	v   *types.Var
}

func (k key) String() string {
	if k.fn == nil {
		pos := rw.Fset.Position(k.v.Pos())
		return fmt.Sprintf("local %s (%s:%d)", k.v.Name(), filepath.Base(pos.Filename), pos.Line)
	}
	return fmt.Sprintf("%s#%d", strings.TrimPrefix(k.fn.FullName(), "closure."), k.idx)
}

// wrapperOf reports whether the composite literal is a j2go function-interface wrapper
// &F_fn{func...}: a named struct whose one field has a function type.
func wrapperOf(cl *ast.CompositeLit) (*types.Named, bool) {
	id, ok := cl.Type.(*ast.Ident)
	if !ok || !strings.HasSuffix(id.Name, "_fn") || len(cl.Elts) != 1 {
		return nil, false
	}
	if _, ok := cl.Elts[0].(*ast.FuncLit); !ok {
		return nil, false
	}
	tn, ok := rw.Info.Uses[id].(*types.TypeName)
	if !ok || tn.Parent() != rw.Pkg.Scope() {
		return nil, false
	}
	if wrapperField(tn.Type()) == nil {
		return nil, false
	}
	return tn.Type().(*types.Named), true
}

// wrapperField is the function field of a wrapper type F_fn.
func wrapperField(t types.Type) *types.Var {
	nt, ok := t.(*types.Named)
	if !ok || !strings.HasSuffix(nt.Obj().Name(), "_fn") {
		return nil
	}
	st, ok := nt.Underlying().(*types.Struct)
	if !ok || st.NumFields() != 1 {
		return nil
	}
	if _, ok := st.Field(0).Type().Underlying().(*types.Signature); !ok {
		return nil
	}
	return st.Field(0)
}

// direct reports whether method m of wrapper type w just calls its function field:
// `return this.f(p0, ...)` or `this.f(p0, ...)`.
func direct(w *types.Named, m string) bool {
	fld := wrapperField(w)
	o, _, _ := types.LookupFieldOrMethod(types.NewPointer(w), true, rw.Pkg, m)
	fn, ok := o.(*types.Func)
	if !ok || fld == nil {
		return false
	}
	fd := declOf[fn]
	if fd == nil || fd.Body == nil || len(fd.Body.List) != 1 {
		return false
	}
	var call ast.Expr
	switch s := fd.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(s.Results) == 1 {
			call = s.Results[0]
		}
	case *ast.ExprStmt:
		call = s.X
	}
	c, ok := call.(*ast.CallExpr)
	if !ok {
		return false
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != fld.Name() {
		return false
	}
	if id, ok := sel.X.(*ast.Ident); !ok || len(fd.Recv.List[0].Names) != 1 || id.Name != fd.Recv.List[0].Names[0].Name {
		return false
	}
	var names []string
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			names = append(names, n.Name)
		}
	}
	if len(c.Args) != len(names) || c.Ellipsis.IsValid() {
		return false
	}
	for i, a := range c.Args {
		if id, ok := a.(*ast.Ident); !ok || id.Name != names[i] {
			return false
		}
	}
	return true
}

// argOf returns the call that e is a direct argument of, and its index.
func argOf(e ast.Expr) (*ast.CallExpr, int) {
	var n ast.Node = e
	for {
		p := rw.Parent[n]
		if pe, ok := p.(*ast.ParenExpr); ok {
			n = pe
			continue
		}
		c, ok := p.(*ast.CallExpr)
		if !ok {
			return nil, -1
		}
		for i, a := range c.Args {
			if a == n {
				return c, i
			}
		}
		return nil, -1
	}
}

// calleeOf is the declared function or concrete method a call calls statically.
func calleeOf(c *ast.CallExpr) *types.Func {
	fun := rw.Unparen(c.Fun)
	if ix, ok := fun.(*ast.IndexExpr); ok {
		fun = ix.X
	} else if ix, ok := fun.(*ast.IndexListExpr); ok {
		fun = ix.X
	}
	switch f := fun.(type) {
	case *ast.Ident:
		fn, _ := rw.Info.Uses[f].(*types.Func)
		if fn == nil {
			return nil
		}
		return fn.Origin()
	case *ast.SelectorExpr:
		s, ok := rw.Info.Selections[f]
		if !ok || s.Kind() != types.MethodVal {
			return nil
		}
		fn, _ := s.Obj().(*types.Func)
		if fn == nil {
			return nil
		}
		if _, isIface := s.Recv().Underlying().(*types.Interface); isIface {
			return nil
		}
		return fn.Origin()
	}
	return nil
}

// funcIdent is the identifier naming the function a call calls.
func funcIdent(c *ast.CallExpr) *ast.Ident {
	fun := rw.Unparen(c.Fun)
	if ix, ok := fun.(*ast.IndexExpr); ok {
		fun = ix.X
	} else if ix, ok := fun.(*ast.IndexListExpr); ok {
		fun = ix.X
	}
	switch f := fun.(type) {
	case *ast.Ident:
		return f
	case *ast.SelectorExpr:
		return f.Sel
	}
	return nil
}

// method1 is the one method of a function-interface type.
func method1(t types.Type) *types.Func {
	it, ok := t.Underlying().(*types.Interface)
	if !ok || it.NumMethods() != 1 {
		return nil
	}
	return it.Method(0)
}

func generated(n ast.Node) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "g_")
}

// asCallee reports whether every use of fn calls it.
func asCallee(fn *types.Func) bool {
	for _, id := range rw.UsesOf[fn] {
		var e ast.Expr = id
		if s, ok := rw.Parent[id].(*ast.SelectorExpr); ok && s.Sel == id {
			e = s
		}
		for {
			p := rw.Parent[e]
			if ix, ok := p.(*ast.IndexExpr); ok && ix.X == e {
				e = ix
				continue
			}
			if ix, ok := p.(*ast.IndexListExpr); ok && ix.X == e {
				e = ix
				continue
			}
			if pe, ok := p.(*ast.ParenExpr); ok {
				e = pe
				continue
			}
			break
		}
		if c, ok := rw.Parent[e].(*ast.CallExpr); !ok || c.Fun != e {
			return false
		}
	}
	return true
}

// required reports whether method fn is one that an interface asks of some type.
func required(fn *types.Func) bool {
	for _, it := range ifaces {
		has := false
		for i := 0; i < it.NumMethods(); i++ {
			if it.Method(i).Name() == fn.Name() {
				has = true
			}
		}
		if !has {
			continue
		}
		for _, nt := range named {
			for _, t := range []types.Type{nt, types.NewPointer(nt)} {
				if !types.Implements(t, it) {
					continue
				}
				if o, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, fn.Name()); o == fn {
					return true
				}
			}
		}
	}
	return false
}

// use is one use of a parameter in its function's body.
type use struct {
	id   *ast.Ident
	call *ast.SelectorExpr // p.m in a call p.m(...)
	pass key               // or passed as this argument
}

// param is the analysis of one parameter or local variable.
type param struct {
	k    key
	obj  *types.Var
	typ  ast.Expr    // its declared type (nil for v := ...)
	m    *types.Func // the interface's method
	uses []use
	vals []ast.Expr // the values it is given: arguments of the calls, or assigned values
	why  string
}

// usesOf classifies the uses of p.obj other than the assignments in skip; it sets p.why at
// the first use that is not a call of the method or a pass to a parameter.
func (p *param) usesOf(skip map[*ast.Ident]bool) {
	for _, id := range rw.UsesOf[p.obj] {
		if skip[id] {
			continue
		}
		if s, ok := rw.Up(id).(*ast.SelectorExpr); ok && s.X == id && s.Sel.Name == p.m.Name() {
			if c, ok := rw.Up(s).(*ast.CallExpr); ok && c.Fun == s {
				p.uses = append(p.uses, use{id: id, call: s})
				continue
			}
		}
		if c, i := argOf(id); c != nil {
			if g := calleeOf(c); g != nil {
				p.uses = append(p.uses, use{id: id, pass: key{fn: g, idx: i}})
				continue
			}
		}
		p.why = fmt.Sprintf("used as %T", rw.Up(id))
		return
	}
}

func analyse(k key) *param {
	p := &param{k: k}
	fd := declOf[k.fn]
	sig := k.fn.Type().(*types.Signature)
	switch {
	case fd == nil || fd.Body == nil:
		p.why = "no body"
	case !generated(fd):
		p.why = "runtime"
	case sig.TypeParams() != nil || sig.RecvTypeParams() != nil:
		p.why = "generic"
	case sig.Variadic() && k.idx >= sig.Params().Len()-1:
		p.why = "variadic"
	}
	if p.why != "" {
		return p
	}
	p.obj = sig.Params().At(k.idx)
	if p.m = method1(p.obj.Type()); p.m == nil {
		p.why = "not a function interface: " + p.obj.Type().String()
		return p
	}
	i := 0
	for _, f := range fd.Type.Params.List {
		for range f.Names {
			if i == k.idx {
				if len(f.Names) != 1 {
					p.why = "grouped parameter"
					return p
				}
				p.typ = f.Type
			}
			i++
		}
	}
	if p.usesOf(nil); p.why != "" {
		return p
	}
	if !asCallee(k.fn) {
		p.why = "used as a value"
		return p
	}
	if sig.Recv() != nil && required(k.fn) {
		p.why = "an interface asks for the method"
		return p
	}
	return p
}

// callsOf are the calls of fn.
func callsOf(fn *types.Func) []*ast.CallExpr {
	var out []*ast.CallExpr
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok && calleeOf(c) == fn {
				out = append(out, c)
			}
			return true
		})
	}
	return out
}

// lambdaVar is the package-level lambda (lambdas step) that e names, if any.
func lambdaVar(e ast.Expr) (*types.Var, *types.Var) {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok {
		return nil, nil
	}
	v, ok := rw.Info.Uses[id].(*types.Var)
	if !ok || v.Parent() != rw.Pkg.Scope() || !strings.HasPrefix(v.Name(), "lambda_") {
		return nil, nil
	}
	pt, ok := v.Type().(*types.Pointer)
	if !ok {
		return nil, nil
	}
	return v, wrapperField(pt.Elem())
}

// argKind classifies argument a passed to a parameter whose interface method is m: "lit"
// (&F_fn{func...}), "var" (package-level lambda), "param" (a converted parameter), or ""
// (anything else).
func argKind(a ast.Expr, m *types.Func, conv map[*types.Var]*param) string {
	a = rw.Unparen(a)
	if u, ok := a.(*ast.UnaryExpr); ok && u.Op == token.AND {
		if cl, ok := u.X.(*ast.CompositeLit); ok {
			if w, ok := wrapperOf(cl); ok && direct(w, m.Name()) && types.Identical(rw.Info.Types[cl.Elts[0]].Type, funcSig(m)) {
				return "lit"
			}
		}
		return ""
	}
	if v, fld := lambdaVar(a); v != nil && fld != nil && direct(v.Type().(*types.Pointer).Elem().(*types.Named), m.Name()) && types.Identical(fld.Type(), funcSig(m)) {
		return "var"
	}
	if id, ok := a.(*ast.Ident); ok {
		if v, ok := rw.Info.Uses[id].(*types.Var); ok {
			if p := conv[v]; p != nil && p.m.Name() == m.Name() && types.Identical(funcSig(p.m), funcSig(m)) {
				return "param"
			}
		}
	}
	return ""
}

// funcSig is the Go function type of interface method m.
func funcSig(m *types.Func) *types.Signature {
	s := m.Type().(*types.Signature)
	return types.NewSignatureType(nil, nil, nil, s.Params(), s.Results(), s.Variadic())
}

func typeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == rw.Pkg {
			return ""
		}
		return p.Name()
	})
}

// variant is a runtime function's variant X_f taking func values, and the parameters it
// takes as func values, with their interface methods.
type variant struct {
	fn    *types.Func
	meths map[int]*types.Func
}

var variants = map[*types.Func]*variant{}

func variantOf(fn *types.Func) *variant {
	if v, ok := variants[fn]; ok {
		return v
	}
	variants[fn] = nil
	if fd := declOf[fn]; fd == nil || generated(fd) {
		return nil
	}
	sig := fn.Type().(*types.Signature)
	name := fn.Name() + "_f"
	var vf *types.Func
	if r := sig.Recv(); r != nil {
		o, _, _ := types.LookupFieldOrMethod(r.Type(), true, rw.Pkg, name)
		vf, _ = o.(*types.Func)
	} else {
		vf, _ = rw.Pkg.Scope().Lookup(name).(*types.Func)
	}
	if vf == nil {
		return nil
	}
	vs := vf.Type().(*types.Signature)
	if vs.Params().Len() != sig.Params().Len() || typeString(vs.Results()) != typeString(sig.Results()) {
		return nil
	}
	v := &variant{fn: vf, meths: map[int]*types.Func{}}
	for i := 0; i < sig.Params().Len(); i++ {
		a, b := sig.Params().At(i).Type(), vs.Params().At(i).Type()
		if typeString(a) == typeString(b) {
			continue
		}
		m := method1(a)
		if m == nil || typeString(funcSig(m)) != typeString(b) {
			return nil
		}
		v.meths[i] = m
	}
	if len(v.meths) == 0 {
		return nil
	}
	variants[fn] = v
	return v
}

func convert(report bool) {
	// Every function-interface parameter of a generated function, and the calls of each.
	all := map[key]*param{}
	var order []key
	calls := map[*types.Func][]*ast.CallExpr{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				if fn := calleeOf(c); fn != nil {
					calls[fn] = append(calls[fn], c)
				}
			}
			return true
		})
	}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || !generated(fd) {
				continue
			}
			fn, _ := rw.Info.Defs[fd.Name].(*types.Func)
			if fn == nil {
				continue
			}
			sig := fn.Type().(*types.Signature)
			for i := 0; i < sig.Params().Len(); i++ {
				if method1(sig.Params().At(i).Type()) == nil {
					continue
				}
				k := key{fn: fn, idx: i}
				p := analyse(k)
				for _, c := range calls[fn] {
					p.vals = append(p.vals, c.Args[i])
				}
				if p.why == "" && len(p.vals) == 0 {
					p.why = "never called"
				}
				all[k] = p
				order = append(order, k)
			}
			for _, p := range locals(fd) {
				all[p.k] = p
				order = append(order, p.k)
			}
		}
	}
	byVar := map[*types.Var]*param{}
	for _, p := range all {
		if p.why == "" {
			byVar[p.obj] = p
		}
	}
	// variantCall reports whether runtime call c becomes a call of its X_f variant.
	variantCall := func(c *ast.CallExpr) bool {
		v := variantOf(calleeOf(c))
		if v == nil {
			return false
		}
		for i, m := range v.meths {
			if argKind(c.Args[i], m, byVar) == "" {
				return false
			}
		}
		return true
	}
	for changed := true; changed; {
		changed = false
		for _, k := range order {
			p := all[k]
			if p.why != "" {
				continue
			}
			for _, u := range p.uses {
				if u.call != nil {
					continue
				}
				if q := all[u.pass]; q != nil {
					if q.why != "" {
						p.why = "passed to " + u.pass.String() + " (" + q.why + ")"
					}
				} else if c, _ := argOf(u.id); !variantCall(c) {
					p.why = "passed to " + u.pass.String()
				}
				if p.why != "" {
					break
				}
			}
			if p.why == "" {
				for _, a := range p.vals {
					if argKind(a, p.m, byVar) == "" {
						pos := rw.Fset.Position(a.Pos())
						p.why = fmt.Sprintf("given another value (%s:%d)", filepath.Base(pos.Filename), pos.Line)
						break
					}
				}
			}
			if p.why != "" {
				delete(byVar, p.obj)
				changed = true
			}
		}
	}

	// The rewrites. An argument is unwrapped where its parameter is converted.
	unwrap := func(a ast.Expr, m *types.Func) {
		a = rw.Unparen(a)
		switch argKind(a, m, byVar) {
		case "lit":
			fl := a.(*ast.UnaryExpr).X.(*ast.CompositeLit).Elts[0]
			rw.Rewrite[a] = func() string { return rw.Render(fl) }
		case "var":
			_, fld := lambdaVar(a)
			name := a.(*ast.Ident).Name
			rw.Rewrite[a] = func() string { return name + "." + fld.Name() }
		}
	}
	nparams, nlocals, nsites, nlit, nvar, nvariant := 0, 0, 0, 0, 0, 0
	count := func(a ast.Expr, m *types.Func) {
		switch argKind(a, m, byVar) {
		case "lit":
			nlit++
		case "var":
			nvar++
		}
	}
	for _, k := range order {
		p := all[k]
		if p.why != "" {
			continue
		}
		if k.fn == nil {
			nlocals++
		} else {
			nparams++
			nsites += len(p.vals)
		}
		ts := typeString(funcSig(p.m))
		if p.typ != nil {
			rw.Rewrite[p.typ] = func() string { return ts }
		}
		for _, u := range p.uses {
			if u.call != nil {
				name := u.id.Name
				rw.Rewrite[u.call] = func() string { return name }
			}
		}
		for _, a := range p.vals {
			count(a, p.m)
			unwrap(a, p.m)
		}
	}
	var rtcalls []*ast.CallExpr
	for fn, cs := range calls {
		if variantOf(fn) == nil {
			continue
		}
		for _, c := range cs {
			if variantCall(c) {
				rtcalls = append(rtcalls, c)
			}
		}
	}
	for _, c := range rtcalls {
		v := variantOf(calleeOf(c))
		nvariant++
		id := funcIdent(c)
		name := v.fn.Name()
		rw.Rewrite[id] = func() string { return name }
		for i, m := range v.meths {
			count(c.Args[i], m)
			unwrap(c.Args[i], m)
		}
	}
	if report {
		type line struct {
			lam int
			s   string
		}
		var ls []line
		for _, k := range order {
			p := all[k]
			lam := 0
			for _, v := range p.vals {
				if a, ok := rw.Unparen(v).(*ast.UnaryExpr); ok {
					if cl, ok := a.X.(*ast.CompositeLit); ok {
						if _, ok := wrapperOf(cl); ok {
							lam++
						}
					}
				}
			}
			if lam == 0 && p.why != "" {
				continue
			}
			st := "converted"
			if p.why != "" {
				st = p.why
			}
			ls = append(ls, line{lam, fmt.Sprintf("%4d %-70s %s", lam, k, st)})
		}
		sort.SliceStable(ls, func(i, j int) bool { return ls[i].lam > ls[j].lam })
		for _, l := range ls {
			fmt.Println(l.s)
		}
	} else {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
	}
	fmt.Fprintf(os.Stderr, "parameters converted: %d (%d calls); local variables: %d; runtime calls to a _f variant: %d; lambda literals unwrapped: %d; package-level lambdas passed as functions: %d\n", nparams, nsites, nlocals, nvariant, nlit, nvar)
}

// locals are the local variables of fd that have a function-interface type, with their
// assigned values: `var v T` (assigned later) or `v := &F_fn{...}`, never taken by address.
func locals(fd *ast.FuncDecl) []*param {
	var out []*param
	add := func(id *ast.Ident, typ ast.Expr, val ast.Expr) {
		v, ok := rw.Info.Defs[id].(*types.Var)
		if !ok || v.Parent() == rw.Pkg.Scope() {
			return
		}
		p := &param{k: key{v: v}, obj: v, typ: typ}
		if p.m = method1(v.Type()); p.m == nil {
			return
		}
		if val != nil {
			p.vals = append(p.vals, val)
		}
		skip := map[*ast.Ident]bool{}
		for _, u := range rw.UsesOf[v] {
			if as, ok := rw.Parent[u].(*ast.AssignStmt); ok && as.Tok == token.ASSIGN && len(as.Lhs) == 1 && as.Lhs[0] == u {
				skip[u] = true
				p.vals = append(p.vals, as.Rhs[0])
			}
		}
		if p.usesOf(skip); p.why == "" && len(p.vals) == 0 {
			p.why = "never given a value"
		}
		out = append(out, p)
	}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ValueSpec:
			if len(x.Names) == 1 && x.Type != nil && len(x.Values) <= 1 {
				var val ast.Expr
				if len(x.Values) == 1 {
					val = x.Values[0]
				}
				add(x.Names[0], x.Type, val)
			}
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE && len(x.Lhs) == 1 && len(x.Rhs) == 1 {
				if id, ok := x.Lhs[0].(*ast.Ident); ok {
					add(id, nil, x.Rhs[0])
				}
			}
		}
		return true
	})
	return out
}
