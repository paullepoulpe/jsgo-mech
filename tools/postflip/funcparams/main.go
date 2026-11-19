// Command funcparams is a post-flip step: a parameter, local variable or field of a
// function-interface type that is only called becomes a Go func value, so a lambda passed to
// it is not an object.
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
// A function interface is a non-empty interface whose methods all have one signature: one
// method (Function.apply), or Guava's Predicate, whose apply and test (Java's default method
// calling apply) both call the lambda. A use that calls any of them is a call of the value.
//
// A parameter is converted when
//   - it has a function-interface type and every use in the body calls a method of it,
//     compares it with nil, or passes it to a converted parameter;
//   - the function is declared in a generated file, is not generic, and is only ever called
//     (never used as a value); a method also must not be one that any interface asks for,
//     except as below;
//   - every call passes one of: a lambda literal &F_fn{func...} whose methods call its
//     function; a package-level lambda (the lambdas step); a converted parameter, local or
//     field; nil; or an object that is certainly not null (this, a new object, or the result
//     of a function that only returns such objects) and whose class has the method, passed as
//     the method value o.m: Java calls o.m when the callee calls the value, the method value
//     calls the same method, and o is evaluated where it was. Any other value (a field or
//     local of interface type, a call result) keeps the parameter an interface: there a null
//     is reported only when it is called, a method value would report it early.
//
// A method that an interface asks for (an interface call x.m(...) may reach it) is converted
// with that interface method: the parameter of the interface method and of every method of
// the package's types that implement it, as one parameter. That is the case of the dispatch
// methods m_impl of a typed self_ (LogFile.log(Supplier) calls this.self_.log_impl(p0)) and of
// Java interfaces (PMap.reconcile). All the implementations must be generated and satisfy the
// conditions above, every call of any of them, through the interface or not, passes a value
// as above, and no other interface (an anonymous one in an assertion) asks for them.
//
// A local variable of a function-interface type is converted the same way when every value
// assigned to it is one of those and every use calls it or passes it to a converted parameter
// (TypeValidator.expectCanAssignToPropertyOf picks one of two lambdas this way). So is a field
// of a generated class: every value stored in it (assignments, keyed composite literals) is
// one of those, its zero value is nil in both forms, and every use calls it, compares it with
// nil or passes it to a converted parameter.
//
// The handwritten runtime offers variants taking a func value for its functions of this kind:
// X_f beside X (Map_computeIfAbsent_f, TMap.computeIfAbsent_f; for a helper named X_,
// mapComputeIfAbsent_f beside mapComputeIfAbsent_), with the function-interface parameters as
// Go function types. A call of X whose function-interface arguments
// are all lambda literals or package-level lambdas becomes a call of X_f.
//
// A package-level lambda (the lambdas step) whose every use is now its function becomes a
// function declaration: `var lambda_X_ = &F_fn{func(a T) R {...}}` -> `func lambda_X_(a T) R
// {...}`, and its uses name the function.
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

// ifaceDecl is the declaration of an interface method: its function type in the interface.
var ifaceDecl = map[*types.Func]*ast.FuncType{}

// groupOf maps an interface method with a function-interface parameter, and each method of
// the package's types implementing it, to the interface method (the group's head).
var groupOf = map[*types.Func]*types.Func{}

// members are the implementations of a group's head.
var members = map[*types.Func][]*types.Func{}

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
	var namedIfaces []*types.Named
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
			namedIfaces = append(namedIfaces, nt)
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
				for _, fld := range it.Methods.List {
					if ft, ok := fld.Type.(*ast.FuncType); ok && len(fld.Names) == 1 {
						if m, ok := rw.Info.Defs[fld.Names[0]].(*types.Func); ok {
							ifaceDecl[m] = ft
						}
					}
				}
			}
			return true
		})
	}
	findGroups(namedIfaces)
	convert(os.Args[1] == "-n")
}

// findGroups records the groups: each method of a named interface declared in a generated
// file that has a function-interface parameter, with the methods implementing it.
func findGroups(nis []*types.Named) {
	bad := map[*types.Func]bool{}
	for _, ni := range nis {
		it := ni.Underlying().(*types.Interface)
		for i := 0; i < it.NumExplicitMethods(); i++ {
			im := it.ExplicitMethod(i)
			if ifaceDecl[im] == nil || !generated(ifaceDecl[im]) {
				continue
			}
			sig := im.Type().(*types.Signature)
			has := false
			for j := 0; j < sig.Params().Len(); j++ {
				if fnIface(sig.Params().At(j).Type()) != nil {
					has = true
				}
			}
			if !has {
				continue
			}
			var impls []*types.Func
			ok := true
			for _, nt := range named {
				for _, t := range []types.Type{nt, types.NewPointer(nt)} {
					if !types.Implements(t, it) {
						continue
					}
					o, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, im.Name())
					fn, _ := o.(*types.Func)
					if fn == nil || declOf[fn] == nil || !generated(declOf[fn]) {
						ok = false
						continue
					}
					dup := false
					for _, g := range impls {
						dup = dup || g == fn
					}
					if !dup {
						impls = append(impls, fn)
					}
				}
			}
			if !ok || len(impls) == 0 {
				continue
			}
			for _, fn := range impls {
				if h, ok := groupOf[fn]; ok && h != im {
					bad[h], bad[im] = true, true
				}
				groupOf[fn] = im
			}
			groupOf[im] = im
			members[im] = impls
		}
	}
	for h := range bad {
		for _, fn := range members[h] {
			delete(groupOf, fn)
		}
		delete(groupOf, h)
		delete(members, h)
	}
}

// key is a parameter of a function (or of a group, fn its head), or a local variable or field
// (fn nil).
type key struct {
	fn  *types.Func
	idx int
	v   *types.Var
}

func (k key) String() string {
	if k.fn == nil {
		pos := rw.Fset.Position(k.v.Pos())
		kind := "local"
		if k.v.IsField() {
			kind = "field"
		} else if k.v.Parent() == rw.Pkg.Scope() {
			kind = "var"
		}
		return fmt.Sprintf("%s %s (%s:%d)", kind, k.v.Name(), filepath.Base(pos.Filename), pos.Line)
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

// directAll reports whether every method of the function interface calls w's function.
func directAll(w *types.Named, ms []*types.Func) bool {
	for _, m := range ms {
		if !direct(w, forwardTo(types.NewPointer(w), m.Name())) {
			return false
		}
	}
	return true
}

// forwardTo is the method that method m of type t calls: Guava's Predicate.test, Java's
// default method, is written `return Predicate_base_test(this, p0)`, which calls apply.
func forwardTo(t types.Type, m string) string {
	o, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, m)
	fn, ok := o.(*types.Func)
	if !ok || declOf[fn] == nil || declOf[fn].Body == nil {
		return m
	}
	fd := declOf[fn]
	if len(fd.Body.List) != 1 || len(fd.Type.Params.List) != 1 || len(fd.Type.Params.List[0].Names) != 1 {
		return m
	}
	r, ok := fd.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(r.Results) != 1 {
		return m
	}
	if name, c := rw.CallName(r.Results[0]); name == "Predicate_base_test" && len(c.Args) == 2 {
		if id, ok := c.Args[1].(*ast.Ident); ok && id.Name == fd.Type.Params.List[0].Names[0].Name {
			return "apply"
		}
	}
	return m
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

// calleeOf is the declared function or concrete method a call calls statically, or the head
// of the group of the method it calls (through an interface or not).
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
		if h, ok := groupOf[fn.Origin()]; ok {
			return h
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

// fnIface returns the methods of a function-interface type: a non-empty interface whose
// methods all have one signature.
func fnIface(t types.Type) []*types.Func {
	it, ok := t.Underlying().(*types.Interface)
	if !ok || it.NumMethods() == 0 {
		return nil
	}
	var ms []*types.Func
	for i := 0; i < it.NumMethods(); i++ {
		m := it.Method(i)
		if i > 0 && !types.Identical(funcSig(m), funcSig(ms[0])) {
			return nil
		}
		ms = append(ms, m)
	}
	return ms
}

func hasName(ms []*types.Func, name string) bool {
	for _, m := range ms {
		if m.Name() == name {
			return true
		}
	}
	return false
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

// required reports whether method fn is one that an interface asks of some type; an interface
// whose method of that name is head (fn's group) does not count.
func required(fn, head *types.Func) bool {
	for _, it := range ifaces {
		var im *types.Func
		for i := 0; i < it.NumMethods(); i++ {
			if it.Method(i).Name() == fn.Name() {
				im = it.Method(i)
			}
		}
		if im == nil || (head != nil && im == head) {
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

// use is one use of a converted variable.
type use struct {
	x    ast.Expr          // the variable: an identifier, or a selector x.f of a field
	call *ast.SelectorExpr // x.m in a call x.m(...)
	pass key               // or passed as this argument
	nil_ bool              // or compared with nil
}

// param is the analysis of one parameter (of a function or a group), local variable or field.
type param struct {
	k    key
	objs []*types.Var
	typs []ast.Expr    // its declared types (nil for v := ...)
	ms   []*types.Func // the interface's methods
	uses []use
	vals []ast.Expr // the values it is given: arguments of the calls, or assigned values
	why  string
}

func isNil(e ast.Expr) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	_, ok = rw.Info.Uses[id].(*types.Nil)
	return ok
}

// classify records the use x of the variable; it sets p.why if x is not a call of a method,
// a comparison with nil or a pass to a parameter.
func (p *param) classify(x ast.Expr) {
	up := rw.Up(x)
	if s, ok := up.(*ast.SelectorExpr); ok && rw.Unparen(s.X) == x && hasName(p.ms, s.Sel.Name) {
		if c, ok := rw.Up(s).(*ast.CallExpr); ok && c.Fun == s {
			p.uses = append(p.uses, use{x: x, call: s})
			return
		}
	}
	if b, ok := up.(*ast.BinaryExpr); ok && (b.Op == token.EQL || b.Op == token.NEQ) && (isNil(b.X) || isNil(b.Y)) {
		p.uses = append(p.uses, use{x: x, nil_: true})
		return
	}
	if as, ok := up.(*ast.AssignStmt); ok && as.Tok == token.ASSIGN && len(as.Lhs) == 1 && len(as.Rhs) == 1 && rw.Unparen(as.Rhs[0]) == x {
		if v := varOf(as.Lhs[0]); v != nil && fnIface(v.Type()) != nil {
			p.uses = append(p.uses, use{x: x, pass: key{v: v}})
			return
		}
	}
	if kv, ok := up.(*ast.KeyValueExpr); ok && rw.Unparen(kv.Value) == x {
		if id, ok := kv.Key.(*ast.Ident); ok {
			if v, ok := rw.Info.Uses[id].(*types.Var); ok && v.IsField() && fnIface(v.Type()) != nil {
				p.uses = append(p.uses, use{x: x, pass: key{v: v}})
				return
			}
		}
	}
	if c, i := argOf(x); c != nil {
		if g := calleeOf(c); g != nil {
			p.uses = append(p.uses, use{x: x, pass: key{fn: g, idx: i}})
			return
		}
	}
	p.why = fmt.Sprintf("used as %T", up)
}

// usesOf classifies the uses of the variables of p other than the assignments in skip.
func (p *param) usesOf(skip map[*ast.Ident]bool) {
	for _, obj := range p.objs {
		for _, id := range rw.UsesOf[obj] {
			if skip[id] {
				continue
			}
			if p.classify(rw.VarExpr(id)); p.why != "" {
				return
			}
		}
	}
}

// analyseFn checks parameter idx of function fn, and adds its variable and declared type to p.
func (p *param) analyseFn(fn *types.Func, idx int) {
	fd := declOf[fn]
	sig := fn.Type().(*types.Signature)
	switch {
	case fd == nil || fd.Body == nil:
		p.why = "no body"
	case !generated(fd):
		p.why = "runtime"
	// A function with its own type parameters stays. A method of a generic type (second flip:
	// the typed translator keeps LinkedIdentityHashMap[K, V], HamtPMap[K, V] generic) is
	// converted like any other: its uses are found through the origin method, and the package is
	// type-checked after the rewrite.
	case sig.TypeParams() != nil:
		p.why = "generic"
	case sig.Variadic() && idx >= sig.Params().Len()-1:
		p.why = "variadic"
	}
	if p.why != "" {
		return
	}
	obj := sig.Params().At(idx)
	i := 0
	for _, f := range fd.Type.Params.List {
		for range f.Names {
			if i == idx {
				if len(f.Names) != 1 {
					p.why = "grouped parameter"
					return
				}
				p.typs = append(p.typs, f.Type)
			}
			i++
		}
	}
	p.objs = append(p.objs, obj)
	if !asCallee(fn) {
		p.why = "used as a value"
	}
}

func analyse(k key) *param {
	p := &param{k: k}
	sig := k.fn.Type().(*types.Signature)
	if p.ms = fnIface(sig.Params().At(k.idx).Type()); p.ms == nil {
		p.why = "not a function interface: " + sig.Params().At(k.idx).Type().String()
		return p
	}
	if impls, ok := members[k.fn]; ok {
		// A group: the interface method's declaration, then each implementation.
		ft := ifaceDecl[k.fn]
		i := 0
		for _, f := range ft.Params.List {
			n := len(f.Names)
			if n == 0 {
				n = 1
			}
			if k.idx >= i && k.idx < i+n {
				if n != 1 {
					p.why = "grouped parameter"
					return p
				}
				p.typs = append(p.typs, f.Type)
			}
			i += n
		}
		if !asCallee(k.fn) {
			p.why = "interface method used as a value"
			return p
		}
		for _, fn := range impls {
			if p.analyseFn(fn, k.idx); p.why != "" {
				p.why = fn.FullName() + ": " + p.why
				return p
			}
			if required(fn, k.fn) {
				p.why = fn.FullName() + ": another interface asks for the method"
				return p
			}
		}
		p.usesOf(nil)
		return p
	}
	if p.analyseFn(k.fn, k.idx); p.why != "" {
		return p
	}
	if p.usesOf(nil); p.why != "" {
		return p
	}
	if sig.Recv() != nil && required(k.fn, nil) {
		p.why = "an interface asks for the method"
	}
	return p
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

// varOf is the variable (local, parameter or field) that e names.
func varOf(e ast.Expr) *types.Var {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		v, _ := rw.Info.Uses[x].(*types.Var)
		return v
	case *ast.SelectorExpr:
		if s, ok := rw.Info.Selections[x]; ok && s.Kind() == types.FieldVal {
			v, _ := s.Obj().(*types.Var)
			return v
		}
	}
	return nil
}

// boxed is the object e boxes, if e is icast_I(obj_(o)) or o of a pointer type.
func boxed(e ast.Expr) ast.Expr {
	e = rw.Unparen(e)
	if name, c := rw.CallName(e); c != nil && strings.HasPrefix(name, "icast_") && len(c.Args) == 1 {
		if n2, c2 := rw.CallName(c.Args[0]); n2 == "obj_" && len(c2.Args) == 1 {
			e = rw.Unparen(c2.Args[0])
		} else {
			return nil
		}
	}
	if tv, ok := rw.Info.Types[e]; ok {
		if pt, ok := tv.Type.(*types.Pointer); ok {
			if _, ok := pt.Elem().(*types.Named); ok {
				return e
			}
		}
	}
	return nil
}

var nonNullMemo = map[*types.Func]int{} // 1 yes, 2 no, 3 in progress

var defOf map[types.Object]*ast.Ident

// nonNullLocal reports whether id names a local variable defined with a value that is not
// null (x := v, var x T = v), and never assigned again or taken by address.
func nonNullLocal(id *ast.Ident) bool {
	v, ok := rw.Info.Uses[id].(*types.Var)
	if !ok || v.Parent() == rw.Pkg.Scope() || v.IsField() {
		return false
	}
	if defOf == nil {
		defOf = map[types.Object]*ast.Ident{}
		for d, o := range rw.Info.Defs {
			if o != nil {
				defOf[o] = d
			}
		}
	}
	def := defOf[v]
	if def == nil {
		return false
	}
	var val ast.Expr
	switch p := rw.Parent[def].(type) {
	case *ast.AssignStmt:
		if p.Tok == token.DEFINE && len(p.Lhs) == 1 && len(p.Rhs) == 1 {
			val = p.Rhs[0]
		}
	case *ast.ValueSpec:
		if len(p.Names) == 1 && len(p.Values) == 1 {
			val = p.Values[0]
		}
	}
	if val == nil {
		return false
	}
	for _, u := range rw.UsesOf[v] {
		switch p := rw.Up(u).(type) {
		case *ast.AssignStmt:
			for _, l := range p.Lhs {
				if rw.Unparen(l) == u {
					return false
				}
			}
		case *ast.UnaryExpr:
			if p.Op == token.AND {
				return false
			}
		case *ast.IncDecStmt:
			return false
		}
	}
	return nonNull(val)
}

// nonNull reports whether e certainly is not null: this, a new object &T{...}, or the result of
// a generated function whose every return statement returns such a value.
func nonNull(e ast.Expr) bool {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		if x.Name == "this" {
			return true
		}
		return nonNullLocal(x)
	case *ast.UnaryExpr:
		_, ok := x.X.(*ast.CompositeLit)
		return x.Op == token.AND && ok
	case *ast.CallExpr:
		// A function, not a method: a method of a null receiver can return its receiver.
		id, ok := rw.Unparen(x.Fun).(*ast.Ident)
		if !ok {
			return false
		}
		fn, _ := rw.Info.Uses[id].(*types.Func)
		if fn == nil {
			return false
		}
		switch nonNullMemo[fn] {
		case 1:
			return true
		case 2, 3:
			return false
		}
		fd := declOf[fn]
		if fd == nil || fd.Body == nil || !generated(fd) {
			nonNullMemo[fn] = 2
			return false
		}
		nonNullMemo[fn] = 3
		ok, n := true, 0
		ast.Inspect(fd.Body, func(c ast.Node) bool {
			if _, isLit := c.(*ast.FuncLit); isLit {
				return false
			}
			if r, isRet := c.(*ast.ReturnStmt); isRet {
				n++
				ok = ok && len(r.Results) == 1 && nonNull(r.Results[0])
			}
			return true
		})
		if ok && n > 0 {
			nonNullMemo[fn] = 1
			return true
		}
		nonNullMemo[fn] = 2
	}
	return false
}

// methodValue is the method m such that o.m (o a pointer to a class) is a method value of the
// function type of the interface methods ms: the one they all call (forwardTo), or "".
func methodValue(o ast.Expr, ms []*types.Func) string {
	t := rw.Info.Types[o].Type
	m := ""
	for _, im := range ms {
		n := forwardTo(t, im.Name())
		if m != "" && n != m {
			return ""
		}
		m = n
	}
	obj, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, m)
	fn, ok := obj.(*types.Func)
	if !ok || !types.Identical(funcSig(fn), funcSig(ms[0])) {
		return ""
	}
	return m
}

// iife reports whether e is a function literal called at once that returns a lambda literal
// at its end and nowhere else (j2go's method reference o::m with o evaluated first:
// func() *F_fn { recv_1 := o; return &F_fn{func...} }()), and returns that literal.
func iife(e ast.Expr, ms []*types.Func) *ast.CompositeLit {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return nil
	}
	fl, ok := c.Fun.(*ast.FuncLit)
	if !ok || fl.Type.Results == nil || len(fl.Type.Results.List) != 1 || len(fl.Body.List) == 0 {
		return nil
	}
	n := 0
	ast.Inspect(fl.Body, func(x ast.Node) bool {
		if _, ok := x.(*ast.FuncLit); ok {
			return false
		}
		if _, ok := x.(*ast.ReturnStmt); ok {
			n++
		}
		return true
	})
	r, ok := fl.Body.List[len(fl.Body.List)-1].(*ast.ReturnStmt)
	if !ok || n != 1 || len(r.Results) != 1 {
		return nil
	}
	u, ok := r.Results[0].(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return nil
	}
	cl, ok := u.X.(*ast.CompositeLit)
	if !ok {
		return nil
	}
	if w, ok := wrapperOf(cl); ok && directAll(w, ms) && types.Identical(rw.Info.Types[cl.Elts[0]].Type, funcSig(ms[0])) {
		return cl
	}
	return nil
}

// argKind classifies argument a passed to a parameter whose interface methods are ms: "lit"
// (&F_fn{func...}), "var" (package-level lambda), "conv" (a converted variable), "nil", "obj"
// (an object certainly not null with the method), or "" (anything else).
func argKind(a ast.Expr, ms []*types.Func, conv map[*types.Var]*param) string {
	a = rw.Unparen(a)
	sig := funcSig(ms[0])
	if u, ok := a.(*ast.UnaryExpr); ok && u.Op == token.AND {
		if cl, ok := u.X.(*ast.CompositeLit); ok {
			if w, ok := wrapperOf(cl); ok && directAll(w, ms) && types.Identical(rw.Info.Types[cl.Elts[0]].Type, sig) {
				return "lit"
			}
		}
	}
	if v, fld := lambdaVar(a); v != nil && fld != nil && directAll(v.Type().(*types.Pointer).Elem().(*types.Named), ms) && types.Identical(fld.Type(), sig) {
		return "var"
	}
	if v := varOf(a); v != nil {
		if p := conv[v]; p != nil && types.Identical(funcSig(p.ms[0]), sig) {
			return "conv"
		}
	}
	if isNil(a) {
		return "nil"
	}
	if o := boxed(a); o != nil && nonNull(o) && methodValue(o, ms) != "" {
		return "obj"
	}
	if iife(a, ms) != nil {
		return "iife"
	}
	return ""
}

// funcSig is the Go function type of method m.
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
	meths map[int][]*types.Func
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
	// X_f, or for a runtime helper named X_ (mapComputeIfAbsent_), X_f too
	name := strings.TrimSuffix(fn.Name(), "_") + "_f"
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
	v := &variant{fn: vf, meths: map[int][]*types.Func{}}
	for i := 0; i < sig.Params().Len(); i++ {
		a, b := sig.Params().At(i).Type(), vs.Params().At(i).Type()
		if typeString(a) == typeString(b) {
			continue
		}
		ms := fnIface(a)
		if ms == nil || typeString(funcSig(ms[0])) != typeString(b) {
			return nil
		}
		v.meths[i] = ms
	}
	if len(v.meths) == 0 {
		return nil
	}
	variants[fn] = v
	return v
}

func convert(report bool) {
	// Every function-interface parameter of a generated function or group, and the calls of
	// each.
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
	addParams := func(fn *types.Func) {
		sig := fn.Type().(*types.Signature)
		for i := 0; i < sig.Params().Len(); i++ {
			if fnIface(sig.Params().At(i).Type()) == nil {
				continue
			}
			k := key{fn: fn, idx: i}
			p := analyse(k)
			for _, c := range calls[fn] {
				p.vals = append(p.vals, c.Args[i])
			}
			all[k] = p
			order = append(order, k)
		}
	}
	var heads []*types.Func
	for h := range members {
		heads = append(heads, h)
	}
	sort.Slice(heads, func(i, j int) bool { return heads[i].Pos() < heads[j].Pos() })
	for _, h := range heads {
		addParams(h)
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
			if _, inGroup := groupOf[fn]; !inGroup {
				addParams(fn)
			}
			for _, p := range locals(fd) {
				all[p.k] = p
				order = append(order, p.k)
			}
		}
	}
	for _, p := range fields() {
		all[p.k] = p
		order = append(order, p.k)
	}
	byVar := map[*types.Var]*param{}
	for _, p := range all {
		if p.why == "" {
			for _, o := range p.objs {
				byVar[o] = p
			}
		}
	}
	// variantCall reports whether runtime call c becomes a call of its X_f variant.
	variantCall := func(c *ast.CallExpr) bool {
		v := variantOf(calleeOf(c))
		if v == nil {
			return false
		}
		for i, ms := range v.meths {
			if k := argKind(c.Args[i], ms, byVar); k == "" || k == "obj" || k == "nil" {
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
				if u.call != nil || u.nil_ {
					continue
				}
				if q := all[u.pass]; q != nil {
					if q.why != "" {
						p.why = "passed to " + u.pass.String() + " (" + q.why + ")"
					}
				} else if u.pass.fn == nil {
					p.why = "stored in " + u.pass.String()
				} else if c, _ := argOf(u.x); !variantCall(c) {
					p.why = "passed to " + u.pass.String()
				}
				if p.why != "" {
					break
				}
			}
			if p.why == "" {
				for _, a := range p.vals {
					if argKind(a, p.ms, byVar) == "" {
						pos := rw.Fset.Position(a.Pos())
						p.why = fmt.Sprintf("given another value (%s:%d)", filepath.Base(pos.Filename), pos.Line)
						break
					}
				}
			}
			if p.why != "" {
				for _, o := range p.objs {
					delete(byVar, o)
				}
				changed = true
			}
		}
	}

	// The rewrites. An argument is unwrapped where its parameter is converted.
	asFunc := map[*types.Var]int{} // package-level lambdas: uses that become the function
	unwrap := func(a ast.Expr, ms []*types.Func) {
		a = rw.Unparen(a)
		switch argKind(a, ms, byVar) {
		case "lit":
			fl := a.(*ast.UnaryExpr).X.(*ast.CompositeLit).Elts[0]
			rw.Rewrite[a] = func() string { return rw.Render(fl) }
		case "var":
			v, fld := lambdaVar(a)
			asFunc[v]++
			name := a.(*ast.Ident).Name
			rw.Rewrite[a] = func() string {
				if lambdaFuncs[v] {
					return name
				}
				return name + "." + fld.Name()
			}
		case "obj":
			o := boxed(a)
			m := methodValue(o, ms)
			rw.Rewrite[a] = func() string {
				if o == a { // already unboxed by an earlier step: render it, not its own rewrite
					return reflipRecv_(o, rw.RenderChildren(o)) + "." + m
				}
				return reflipRecv_(o, rw.Render(o)) + "." + m
			}
		case "iife":
			cl := iife(a, ms)
			fl := rw.Unparen(a).(*ast.CallExpr).Fun.(*ast.FuncLit)
			ts := typeString(funcSig(ms[0]))
			rw.Rewrite[fl.Type.Results.List[0].Type] = func() string { return ts }
			rw.Rewrite[rw.Parent[cl]] = func() string { return rw.Render(cl.Elts[0]) }
		}
	}
	nparams, ngroups, nlocals, nfields, nsites, nlit, nvar, nobj, nvariant := 0, 0, 0, 0, 0, 0, 0, 0, 0
	count := func(a ast.Expr, ms []*types.Func) {
		switch argKind(a, ms, byVar) {
		case "lit":
			nlit++
		case "var":
			nvar++
		case "obj":
			nobj++
		case "iife":
			nlit++
		}
	}
	for _, k := range order {
		p := all[k]
		if p.why != "" {
			continue
		}
		switch {
		case k.fn != nil && members[k.fn] != nil:
			ngroups++
			nsites += len(p.vals)
		case k.fn != nil:
			nparams++
			nsites += len(p.vals)
		case k.v.IsField() || k.v.Parent() == rw.Pkg.Scope():
			nfields++
		default:
			nlocals++
		}
		ts := typeString(funcSig(p.ms[0]))
		for _, t := range p.typs {
			if t != nil {
				rw.Rewrite[t] = func() string { return ts }
			}
		}
		for _, u := range p.uses {
			if u.call != nil {
				x := u.x
				rw.Rewrite[u.call] = func() string { return rw.Render(x) }
			}
		}
		for _, a := range p.vals {
			count(a, p.ms)
			unwrap(a, p.ms)
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
		for i, ms := range v.meths {
			count(c.Args[i], ms)
			unwrap(c.Args[i], ms)
		}
	}
	nfuncs := lambdasAsFuncs(asFunc)
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
			if lam == 0 && p.why != "" && os.Getenv("FUNCPARAMS_ALL") == "" {
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
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
	fmt.Fprintf(os.Stderr, "parameters converted: %d, interface-method groups: %d (%d calls); local variables: %d; fields and package-level variables: %d; runtime calls to a _f variant: %d; lambda literals unwrapped: %d; package-level lambdas passed as functions: %d; objects passed as method values: %d; package-level lambdas made functions: %d\n", nparams, ngroups, nsites, nlocals, nfields, nvariant, nlit, nvar, nobj, nfuncs)
}

// lambdaFuncs are the package-level lambdas that become function declarations.
var lambdaFuncs = map[*types.Var]bool{}

// lambdasAsFuncs makes each package-level lambda whose every use is its function (unwrapped
// now, asFunc, or written lambda_X_.f already) a function declaration, and returns their number.
func lambdasAsFuncs(asFunc map[*types.Var]int) int {
	n := 0
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR || len(gd.Specs) != 1 || gd.Lparen.IsValid() {
				continue
			}
			vs := gd.Specs[0].(*ast.ValueSpec)
			if len(vs.Names) != 1 || len(vs.Values) != 1 || vs.Type != nil || !strings.HasPrefix(vs.Names[0].Name, "lambda_") {
				continue
			}
			v, ok := rw.Info.Defs[vs.Names[0]].(*types.Var)
			if !ok {
				continue
			}
			u, ok := vs.Values[0].(*ast.UnaryExpr)
			if !ok || u.Op != token.AND {
				continue
			}
			cl, ok := u.X.(*ast.CompositeLit)
			if !ok {
				continue
			}
			if _, ok := wrapperOf(cl); !ok {
				continue
			}
			fld := wrapperField(rw.Info.Types[cl].Type)
			var sels []*ast.SelectorExpr
			uses := rw.UsesOf[v]
			for _, id := range uses {
				if s, ok := rw.Parent[id].(*ast.SelectorExpr); ok && s.X == id && s.Sel.Name == fld.Name() {
					sels = append(sels, s)
				}
			}
			if len(uses) == 0 || len(sels)+asFunc[v] != len(uses) {
				continue
			}
			lambdaFuncs[v] = true
			n++
			name := v.Name()
			fl := cl.Elts[0].(*ast.FuncLit)
			rw.Rewrite[gd] = func() string {
				return "func " + name + strings.TrimPrefix(rw.Render(fl.Type), "func") + " " + rw.Render(fl.Body)
			}
			for _, s := range sels {
				rw.Rewrite[s] = func() string { return name }
			}
		}
	}
	return n
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
		p := &param{k: key{v: v}, objs: []*types.Var{v}, typs: []ast.Expr{typ}}
		if p.ms = fnIface(v.Type()); p.ms == nil {
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

// fields are the fields of function-interface type of the generated, non-generic classes, with
// the values stored in them.
func fields() []*param {
	var out []*param
	positional := map[*types.Struct]bool{} // structs built by a positional composite literal
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if cl, ok := n.(*ast.CompositeLit); ok && len(cl.Elts) > 0 {
				if _, kv := cl.Elts[0].(*ast.KeyValueExpr); !kv {
					if tv, ok := rw.Info.Types[cl]; ok {
						if st, ok := tv.Type.Underlying().(*types.Struct); ok {
							positional[st] = true
						}
					}
				}
			}
			return true
		})
	}
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if ok && gd.Tok == token.VAR {
				for _, s := range gd.Specs {
					vs := s.(*ast.ValueSpec)
					if len(vs.Names) != 1 || vs.Type == nil || strings.HasPrefix(vs.Names[0].Name, "lambda_") {
						continue
					}
					if v, ok := rw.Info.Defs[vs.Names[0]].(*types.Var); ok {
						p := &param{k: key{v: v}, objs: []*types.Var{v}, typs: []ast.Expr{vs.Type}}
						if p.ms = fnIface(v.Type()); p.ms != nil {
							p.vals = append(p.vals, vs.Values...)
							out = append(out, stored(p))
						}
					}
				}
			}
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, s := range gd.Specs {
				ts := s.(*ast.TypeSpec)
				st, ok := ts.Type.(*ast.StructType)
				if !ok || ts.TypeParams != nil || ts.Assign.IsValid() || rw.Info.Defs[ts.Name] == nil {
					continue
				}
				if wrapperField(rw.Info.Defs[ts.Name].Type()) != nil {
					continue // a lambda wrapper F_fn: its field is a func already
				}
				for _, fl := range st.Fields.List {
					if len(fl.Names) != 1 {
						continue
					}
					v, ok := rw.Info.Defs[fl.Names[0]].(*types.Var)
					if !ok {
						continue
					}
					p := &param{k: key{v: v}, objs: []*types.Var{v}, typs: []ast.Expr{fl.Type}}
					if p.ms = fnIface(v.Type()); p.ms == nil {
						continue
					}
					if positional[rw.Info.Defs[ts.Name].Type().Underlying().(*types.Struct)] {
						p.why = "built by a positional literal"
					}
					out = append(out, stored(p))
				}
			}
		}
	}
	return out
}

// stored classifies the uses of a field or package-level variable: the values stored in it,
// and the other uses.
func stored(p *param) *param {
	for _, id := range rw.UsesOf[p.objs[0]] {
		if p.why != "" {
			break
		}
		if !generated(id) {
			p.why = "used by the runtime"
			break
		}
		if kv, ok := rw.Parent[id].(*ast.KeyValueExpr); ok && kv.Key == id {
			p.vals = append(p.vals, kv.Value)
			continue
		}
		x := rw.VarExpr(id)
		if as, ok := rw.Up(x).(*ast.AssignStmt); ok && as.Tok == token.ASSIGN && len(as.Lhs) == 1 && rw.Unparen(as.Lhs[0]) == x {
			p.vals = append(p.vals, as.Rhs[0])
			continue
		}
		p.classify(x)
	}
	if p.why == "" && len(p.vals) == 0 {
		p.why = "never given a value"
	}
	return p
}

// reflipRecv_ parenthesizes the text s of receiver o where a selector would not bind to it.
func reflipRecv_(o ast.Expr, s string) string {
	if _, ok := o.(*ast.UnaryExpr); ok {
		return "(" + s + ")"
	}
	return s
}
