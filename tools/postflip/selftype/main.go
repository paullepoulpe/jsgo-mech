// Command selftype is a post-flip step: where Java's type of a method's result is the receiver's
// own class (a self-typed builder's `T setX(..)` with `class Builder<T extends Builder<T>>`, a
// covariant `FunctionType build()` overriding `PrototypeObjectType build()`), a caller holding
// the subclass gets that class without the cast j2go writes through self().
//
//	cast_[*FunctionType_Builder](b.setName(n).self())      ->  b.setNameT_(n)
//	to_FunctionType(b.withName(n).build().self())           ->  b.withName(n).buildT_()
//
// j2go erases T to the base (PrototypeObjectType.Builder) and writes a method's result with the
// type of the method it overrides, so a caller that holds a FunctionType.Builder casts the
// result back: K(r.m(a).self()), javac's checkcast, which cannot fail. The step applies to a
// receiver r of a class S that nothing embeds (a final class in Java: S's self_ is S itself, so
// a dispatcher of S runs S's own method, the one Java's dispatch runs), and to the method that
// runs for an S receiver (following dispatchers, this.m_impl / this.self_.m_impl, to their
// target):
//
//   - when the method returns its receiver (every return is this, this.up_B(), or a call on this
//     or on an embedded part of it, or on this.self_, of a method that does: castThis(), the
//     base's setName_impl), the result is a pointer into the S object r, whose self() is r: the
//     cast to *S is r after the call. S gets mT_, the call of m returning this (javac's view of
//     `T m()` with T = S), and K(r.m(a).self()) is r.mT_(a): r and the arguments are evaluated
//     once, as before, and a nil r fails in m as before;
//   - when the method's target is declared on S and every return is x.up_B() with x of one type
//     *T (the covariant result: FunctionType.Builder.build_impl returns new_FunctionType(this)
//     .up_PrototypeObjectType()), it is split as javac splits a covariant return (covret): mT_
//     returns x, the target is the bridge returning this.mT_(..).up_B(), and K(r.m(a).self())
//     with K the cast to *T is r.mT_(a) (up_B and self() keep nil, the cast gives nil: x).
//
// A cross-cast icast_I(x.self()) of an object of a class X that something embeds to an interface
// I that X implements (AbstractCompiler as its CompilerInputProvider: an upcast in Java) gives
// the slot the most derived object, which is what an erased slot holds and identity compares;
// x itself (iup_I(x), nil for nil as before) would hold X's part of it. The two answer every
// method of I the same where each of X's methods of I dispatches to self_ or is not overridden
// by a class embedding X, and the value is only called: the receiver of a method call, or an
// argument of a parameter of a function of the package that its body only calls (or passes on
// to another such parameter) and never assigns. Then the cast is iup_I(x) (castfold -nilup's
// form; the iup_ added to g_0iup.go as castfold writes it). A value stored or compared keeps the
// cast, counted.
//
// Left, counted: receivers of a class that something embeds (an override may run), methods that
// neither return their receiver nor one covariant type, casts to another class. A rerun changes
// nothing (the calls no longer cast).
//
// usage: selftype [-n] [-v] <dir of package closure>
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

var qual = func(p *types.Package) string {
	if p == rw.Pkg {
		return ""
	}
	return p.Name()
}

var (
	verbose   bool
	counts    = map[string]int{}
	lefts     = map[string][]string{}
	declOf    = map[*types.Func]*ast.FuncDecl{}
	embedders = map[*types.Named]bool{}
	embList   = map[*types.Named][]*types.Named{} // the classes that embed a class, transitively
	specOf    = map[*types.Named]*ast.GenDecl{}
	iupNeed   = map[string]bool{}
)

func left(why string, n ast.Node) {
	lefts[why] = append(lefts[why], rw.Fset.Position(n.Pos()).String())
}

// a written-out method
type recvM struct {
	s     *types.Named
	name  string // m
	fd    *ast.FuncDecl
	calls []ast.Expr
}

type covM struct {
	s     *types.Named
	fd    *ast.FuncDecl // the target, declared on S
	tname string
	elem  types.Type
	upB   string
	rets  []*ast.CallExpr
	calls []ast.Expr
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.FuncDecl:
				if fo, ok := rw.Info.Defs[x.Name].(*types.Func); ok {
					declOf[fo] = x
				}
			case *ast.GenDecl:
				for _, sp := range x.Specs {
					if ts, ok := sp.(*ast.TypeSpec); ok {
						if tn, ok := rw.Info.Defs[ts.Name].(*types.TypeName); ok {
							if nt, ok := tn.Type().(*types.Named); ok {
								specOf[nt] = x
							}
						}
					}
				}
			}
		}
	}
	for _, n := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		nt, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		var walk func(t *types.Named)
		walk = func(t *types.Named) {
			st, ok := t.Underlying().(*types.Struct)
			if !ok {
				return
			}
			for i := 0; i < st.NumFields(); i++ {
				if f := st.Field(i); f.Embedded() {
					if e, ok := types.Unalias(f.Type()).(*types.Named); ok {
						if _, ok := e.Underlying().(*types.Struct); ok {
							embedders[e.Origin()] = true
							embList[e.Origin()] = append(embList[e.Origin()], nt)
							walk(e)
						}
					}
				}
			}
		}
		walk(nt)
	}
	recvs := map[string]*recvM{}
	covs := map[*ast.FuncDecl]*covM{}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			var c ast.Expr
			var t types.Type
			var operand ast.Expr
			assert := false
			if x, ok := n.(*ast.CallExpr); ok {
				if why, done := crossCast(x); done {
					if why != "" {
						left(why, x)
					}
					return true
				}
			}
			switch x := n.(type) {
			case *ast.CallExpr:
				if t = castTo(x); t != nil {
					c, operand = x, x.Args[0]
				}
			case *ast.TypeAssertExpr:
				// castfold -assert's x.self().(*S) of a value dereferenced at once
				if x.Type != nil {
					if tt := rw.Info.TypeOf(x.Type); ptrNamed(tt) != nil {
						c, t, operand, assert = x, tt, x.X, true
					}
				}
			}
			if c == nil {
				return true
			}
			sc, ok := rw.Unparen(operand).(*ast.CallExpr)
			if !ok || len(sc.Args) != 0 {
				return true
			}
			ss, ok := sc.Fun.(*ast.SelectorExpr)
			if !ok || ss.Sel.Name != "self" {
				return true
			}
			mc, ok := rw.Unparen(ss.X).(*ast.CallExpr)
			if !ok {
				return true
			}
			ms, ok := mc.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			sel := rw.Info.Selections[ms]
			if sel == nil || sel.Kind() != types.MethodVal {
				return true
			}
			s := ptrNamed(rw.Info.TypeOf(ms.X))
			if s == nil || s.TypeParams().Len() > 0 || s.Obj().Pkg() != rw.Pkg {
				return true
			}
			if _, ok := s.Underlying().(*types.Struct); !ok {
				return true
			}
			fd := declOf[sel.Obj().(*types.Func)]
			if fd == nil || fd.Body == nil {
				return true
			}
			if embedders[s] {
				left("the receiver's class is embedded by another (an override may run)", c)
				return true
			}
			// the receiver is returned
			if types.Identical(t, types.NewPointer(s)) && returnsRecv(fd, s, 8) {
				if assert && !nonNil(ms.X) {
					left("an assertion of a receiver-returning method's result on a receiver that may be nil", c)
					return true
				}
				k := s.Obj().Name() + "." + ms.Sel.Name
				r := recvs[k]
				if r == nil {
					r = &recvM{s: s, name: ms.Sel.Name, fd: fd}
					recvs[k] = r
				}
				r.calls = append(r.calls, c)
				return true
			}
			// a covariant result
			if assert {
				left("an assertion of another method's result", c)
				return true
			}
			target := follow(fd, s, 8)
			if cm := covariant(target, s); cm != nil {
				if !types.Identical(t, cm.elem) {
					left("a cast of a covariant result to another class", c)
					return true
				}
				if old := covs[target]; old != nil {
					cm = old
				} else {
					covs[target] = cm
				}
				cm.calls = append(cm.calls, c)
				return true
			}
			left("the method neither returns its receiver nor one covariant class", c)
			return true
		})
	}
	// rewrite: receiver-returning
	var rs []*recvM
	for _, r := range recvs {
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool {
		return rs[i].s.Obj().Name()+rs[i].name < rs[j].s.Obj().Name()+rs[j].name
	})
	added := map[*types.Named][]string{}
	for _, r := range rs {
		r := r
		tname := r.name + "T_"
		if rw.HasMethod(types.NewPointer(r.s), tname) {
			left("the typed name is taken", r.calls[0])
			continue
		}
		counts["methods returning their receiver get mT_ on the class (the call, then this)"]++
		sig := rw.Info.Defs[r.fd.Name].Type().(*types.Signature)
		var ps, as []string
		for i := 0; i < sig.Params().Len(); i++ {
			p := sig.Params().At(i)
			ps = append(ps, fmt.Sprintf("p%d %s", i, types.TypeString(p.Type(), qual)))
			a := fmt.Sprintf("p%d", i)
			if sig.Variadic() && i == sig.Params().Len()-1 {
				ps[i] = fmt.Sprintf("p%d ...%s", i, types.TypeString(p.Type().(*types.Slice).Elem(), qual))
				a += "..."
			}
			as = append(as, a)
		}
		sn := r.s.Obj().Name()
		added[r.s] = append(added[r.s], fmt.Sprintf("// %s is %s for a %s receiver: %s returns its receiver (Java's self type T = %s), so the result is this (selftype).\nfunc (this *%s) %s(%s) *%s {\n\tthis.%s(%s)\n\treturn this\n}",
			tname, r.name, sn, r.name, sn, sn, tname, strings.Join(ps, ", "), sn, r.name, strings.Join(as, ", ")))
		for _, c := range r.calls {
			c := c
			mc := methodCall(c)
			ms := mc.Fun.(*ast.SelectorExpr)
			if _, ok := c.(*ast.TypeAssertExpr); ok {
				counts["assertions .self().(*S) of a receiver-returning method's result on a never-nil receiver are calls of mT_"]++
			} else {
				counts["casts of a receiver-returning method's result through self() are calls of mT_"]++
			}
			rw.Rewrite[c] = func() string {
				var args []string
				for _, a := range mc.Args {
					args = append(args, rw.Render(a))
				}
				return rw.Render(ms.X) + "." + tname + "(" + strings.Join(args, ", ") + ")"
			}
		}
	}
	for s, ms := range added {
		gd := specOf[s]
		ms := ms
		rw.Rewrite[gd] = func() string { return rw.RenderChildren(gd) + "\n\n" + strings.Join(ms, "\n\n") }
	}
	// rewrite: covariant
	var cs []*covM
	for _, cm := range covs {
		cs = append(cs, cm)
	}
	sort.Slice(cs, func(i, j int) bool { return cs[i].fd.Pos() < cs[j].fd.Pos() })
	for _, cm := range cs {
		cm := cm
		if rw.HasMethod(types.NewPointer(cm.s), cm.tname) {
			left("the typed name is taken", cm.calls[0])
			continue
		}
		counts["covariant results split into mT_ and the bridge"]++
		for _, c := range cm.calls {
			c := c
			mc := methodCall(c)
			ms := mc.Fun.(*ast.SelectorExpr)
			counts["casts of a covariant result through self() are calls of mT_"]++
			rw.Rewrite[c] = func() string {
				var args []string
				for _, a := range mc.Args {
					args = append(args, rw.Render(a))
				}
				return rw.Render(ms.X) + "." + cm.tname + "(" + strings.Join(args, ", ") + ")"
			}
		}
		rw.Rewrite[cm.fd] = func() string {
			for _, r := range cm.rets {
				r := r
				rw.Rewrite[r] = func() string { return rw.Render(r.Fun.(*ast.SelectorExpr).X) }
			}
			recv := rw.Text(cm.fd.Recv)
			params := rw.Text(cm.fd.Type.Params)
			var names []string
			for _, f := range cm.fd.Type.Params.List {
				for _, n := range f.Names {
					names = append(names, n.Name)
				}
			}
			body := rw.Render(cm.fd.Body)
			return "// " + cm.tname + " is " + cm.fd.Name.Name + " with the class Java's override declares (a covariant result; selftype).\nfunc " + recv + " " + cm.tname + params + " " + types.TypeString(cm.elem, qual) + " " + body + "\n\n" +
				"// " + cm.fd.Name.Name + " is the bridge with the overridden method's result (javac's).\nfunc " + recv + " " + cm.fd.Name.Name + params + " " + rw.Text(cm.fd.Type.Results) + " {\n\treturn this." + cm.tname + "(" + strings.Join(names, ", ") + ")." + cm.upB + "()\n}"
		}
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", counts[k], k)
	}
	if verbose {
		for _, r := range rs {
			fmt.Fprintf(os.Stderr, "        %s.%sT_: %d\n", r.s.Obj().Name(), r.name, len(r.calls))
		}
		for _, cm := range cs {
			fmt.Fprintf(os.Stderr, "        %s.%s -> %s: %d\n", cm.s.Obj().Name(), cm.tname, types.TypeString(cm.elem, qual), len(cm.calls))
		}
	}
	keys = keys[:0]
	for k := range lefts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", len(lefts[k]), k)
		if verbose {
			for _, p := range lefts[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	if report || len(counts) == 0 {
		return
	}
	if n := writeIup(); n > 0 {
		fmt.Fprintf(os.Stderr, "selftype: %d iup_ functions added to g_0iup.go\n", n)
	}
	fmt.Fprintf(os.Stderr, "selftype: files changed: %d\n", rw.SaveChecked())
}

func ptrNamed(t types.Type) *types.Named {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		if n, ok := types.Unalias(p.Elem()).(*types.Named); ok {
			return n
		}
	}
	return nil
}

// castTo: the type of a class cast cast_[*T](x) / to_X(x) of one operand, or nil.
func castTo(c *ast.CallExpr) types.Type {
	if len(c.Args) != 1 {
		return nil
	}
	name, _ := rw.CallName(c)
	if !(strings.HasPrefix(name, "cast_[") || strings.HasPrefix(name, "to_")) {
		return nil
	}
	sig, ok := rw.Info.TypeOf(c.Fun).(*types.Signature)
	if !ok || sig.Results().Len() != 1 || sig.Params().Len() != 1 {
		return nil
	}
	if ptrNamed(sig.Results().At(0).Type()) == nil {
		return nil
	}
	return sig.Results().At(0).Type()
}

// returnsOf: the return statements of fd, not of function literals in it.
func returnsOf(fd *ast.FuncDecl) []*ast.ReturnStmt {
	var out []*ast.ReturnStmt
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			out = append(out, x)
		}
		return true
	})
	return out
}

func recvObj(fd *ast.FuncDecl) types.Object {
	if fd.Recv == nil || len(fd.Recv.List) == 0 || len(fd.Recv.List[0].Names) == 0 {
		return nil
	}
	return rw.Info.Defs[fd.Recv.List[0].Names[0]]
}

// intoRecv: e is this, or a chain of embedded-field selections from this (a part of the object).
func intoRecv(e ast.Expr, recv types.Object) bool {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		return recv != nil && rw.Info.Uses[x] == recv
	case *ast.SelectorExpr:
		s := rw.Info.Selections[x]
		if s == nil || s.Kind() != types.FieldVal {
			return false
		}
		v, ok := s.Obj().(*types.Var)
		return ok && v.Embedded() && intoRecv(x.X, recv)
	}
	return false
}

// isSelf: e is this.self_ (the most derived object: S itself for a class nothing embeds).
func isSelf(e ast.Expr, recv types.Object) bool {
	if ta, ok := rw.Unparen(e).(*ast.TypeAssertExpr); ok && ta.Type != nil {
		// this.self_.(*C): dispatch's single implementation (the dispatch step)
		return isSelf(ta.X, recv)
	}
	x, ok := rw.Unparen(e).(*ast.SelectorExpr)
	return ok && x.Sel.Name == "self_" && intoRecv(x.X, recv)
}

// calleeOn: the declaration a call this.f(..) / part.f(..) / this.self_.f(..) runs for an S
// object (self_ is S itself), or nil.
func calleeOn(c *ast.CallExpr, recv types.Object, s *types.Named) *ast.FuncDecl {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	if isSelf(sel.X, recv) {
		o, _, _ := types.LookupFieldOrMethod(types.NewPointer(s), true, rw.Pkg, sel.Sel.Name)
		if fo, ok := o.(*types.Func); ok {
			return declOf[fo]
		}
		return nil
	}
	if !intoRecv(sel.X, recv) {
		return nil
	}
	if ss := rw.Info.Selections[sel]; ss != nil && ss.Kind() == types.MethodVal {
		return declOf[ss.Obj().(*types.Func)]
	}
	return nil
}

// returnsRecv: every return of fd gives a pointer into its receiver (for an S object).
func returnsRecv(fd *ast.FuncDecl, s *types.Named, depth int) bool {
	if depth == 0 || fd == nil || fd.Body == nil {
		return false
	}
	recv := recvObj(fd)
	rets := returnsOf(fd)
	if len(rets) == 0 {
		return false
	}
	for _, r := range rets {
		if len(r.Results) != 1 {
			return false
		}
		e := rw.Unparen(r.Results[0])
		if intoRecv(e, recv) {
			continue
		}
		c, ok := e.(*ast.CallExpr)
		if !ok {
			return false
		}
		if sel, ok := c.Fun.(*ast.SelectorExpr); ok && strings.HasPrefix(sel.Sel.Name, "up_") && len(c.Args) == 0 && intoRecv(sel.X, recv) {
			continue
		}
		if !returnsRecv(calleeOn(c, recv, s), s, depth-1) {
			return false
		}
	}
	return true
}

// follow: the declaration a dispatcher runs for an S object: fd whose every return calls one
// declaration (this.m_impl(p..), this.self_.m_impl(p..)) with its parameters, followed.
func follow(fd *ast.FuncDecl, s *types.Named, depth int) *ast.FuncDecl {
	if depth == 0 {
		return fd
	}
	recv := recvObj(fd)
	var params []types.Object
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			params = append(params, rw.Info.Defs[n])
		}
	}
	var target *ast.FuncDecl
	for _, r := range returnsOf(fd) {
		if len(r.Results) != 1 {
			return fd
		}
		c, ok := rw.Unparen(r.Results[0]).(*ast.CallExpr)
		if !ok || len(c.Args) != len(params) {
			return fd
		}
		for i, a := range c.Args {
			id, ok := rw.Unparen(a).(*ast.Ident)
			if !ok || rw.Info.Uses[id] != params[i] {
				return fd
			}
		}
		d := calleeOn(c, recv, s)
		if d == nil || (target != nil && d != target) {
			return fd
		}
		target = d
	}
	if target == nil {
		return fd
	}
	// the dispatcher has nothing but the exact-class test and the returns
	for _, st := range fd.Body.List {
		switch x := st.(type) {
		case *ast.ReturnStmt:
		case *ast.IfStmt:
			if len(x.Body.List) != 1 || x.Else != nil {
				return fd
			}
			if _, ok := x.Body.List[0].(*ast.ReturnStmt); !ok {
				return fd
			}
		default:
			return fd
		}
	}
	return follow(target, s, depth-1)
}

// covariant: fd is declared on S, its result is *B, and every return is x.up_B() with x of one
// pointer type *T other than *B.
func covariant(fd *ast.FuncDecl, s *types.Named) *covM {
	if fd == nil || fd.Body == nil || fd.Type.Results == nil || len(fd.Type.Results.List) != 1 {
		return nil
	}
	fo := rw.Info.Defs[fd.Name].(*types.Func)
	sig := fo.Type().(*types.Signature)
	if ptrNamed(sig.Recv().Type()) != s || sig.Results().Len() != 1 {
		return nil
	}
	b := ptrNamed(sig.Results().At(0).Type())
	if b == nil {
		return nil
	}
	cm := &covM{s: s, fd: fd, upB: "up_" + b.Obj().Name(), tname: strings.TrimSuffix(fd.Name.Name, "_impl") + "T_"}
	for _, r := range returnsOf(fd) {
		if len(r.Results) != 1 {
			return nil
		}
		c, ok := rw.Unparen(r.Results[0]).(*ast.CallExpr)
		if !ok || len(c.Args) != 0 {
			return nil
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != cm.upB {
			return nil
		}
		xt := rw.Info.TypeOf(sel.X)
		if ptrNamed(xt) == nil || ptrNamed(xt) == b {
			return nil
		}
		if cm.elem != nil && !types.Identical(cm.elem, xt) {
			return nil
		}
		cm.elem = xt
		cm.rets = append(cm.rets, c)
	}
	if cm.elem == nil {
		return nil
	}
	return cm
}

var _ = token.ASSIGN

// methodCall: the r.m(a) of a cast K(r.m(a).self()) or an assertion r.m(a).self().(*S).
func methodCall(c ast.Expr) *ast.CallExpr {
	var op ast.Expr
	switch x := c.(type) {
	case *ast.CallExpr:
		op = x.Args[0]
	case *ast.TypeAssertExpr:
		op = x.X
	}
	return rw.Unparen(rw.Unparen(op).(*ast.CallExpr).Fun.(*ast.SelectorExpr).X).(*ast.CallExpr)
}

// nonNil: a receiver that is never nil: a constructor call new_S(..), or a receiver-returning
// call (its receiver never nil), seen through the casts and assertions this step removes.
func nonNil(e ast.Expr) bool {
	e = rw.Unparen(e)
	switch x := e.(type) {
	case *ast.TypeAssertExpr:
		return nonNil(x.X)
	case *ast.CallExpr:
		if id, ok := rw.Unparen(x.Fun).(*ast.Ident); ok {
			if s := ptrNamed(rw.Info.TypeOf(x)); s != nil && strings.HasPrefix(id.Name, "new_"+s.Obj().Name()) {
				return true
			}
		}
		if castTo(x) != nil {
			return nonNil(x.Args[0])
		}
		sel, ok := x.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		if sel.Sel.Name == "self" && len(x.Args) == 0 {
			return nonNil(sel.X)
		}
		ss := rw.Info.Selections[sel]
		s := ptrNamed(rw.Info.TypeOf(sel.X))
		if ss == nil || ss.Kind() != types.MethodVal || s == nil || embedders[s] {
			return false
		}
		return returnsRecv(declOf[ss.Obj().(*types.Func)], s, 8) && nonNil(sel.X)
	}
	return false
}

// crossCast: icast_I(x.self()); done reports that c is one (rewritten, or left with why).
func crossCast(c *ast.CallExpr) (why string, done bool) {
	name, _ := rw.CallName(c)
	if !strings.HasPrefix(name, "icast_") || len(c.Args) != 1 {
		return "", false
	}
	sc, ok := rw.Unparen(c.Args[0]).(*ast.CallExpr)
	if !ok || len(sc.Args) != 0 {
		return "", false
	}
	ss, ok := sc.Fun.(*ast.SelectorExpr)
	if !ok || ss.Sel.Name != "self" {
		return "", false
	}
	xt := rw.Info.TypeOf(ss.X)
	x := ptrNamed(xt)
	if x == nil {
		return "", false
	}
	it, ok := types.Unalias(rw.Info.TypeOf(c)).(*types.Named)
	if !ok {
		return "", false
	}
	iface, ok := it.Underlying().(*types.Interface)
	if !ok || !types.Implements(xt, iface) {
		return "a cross-cast to an interface the class does not implement", true
	}
	if !embedders[x.Origin()] {
		return "", false // castfold's (a class nothing embeds: x.self() is x)
	}
	for i := 0; i < iface.NumMethods(); i++ {
		m := iface.Method(i).Name()
		o, _, _ := types.LookupFieldOrMethod(xt, true, rw.Pkg, m)
		fx, _ := o.(*types.Func)
		if fx == nil {
			return "a cross-cast to an interface whose method the class does not have", true
		}
		if dispatchesToSelf(declOf[fx]) {
			continue
		}
		for _, e := range embList[x.Origin()] {
			oe, _, _ := types.LookupFieldOrMethod(types.NewPointer(e), true, rw.Pkg, m)
			if fe, _ := oe.(*types.Func); fe != nil && fe.Origin() != fx.Origin() {
				return "a cross-cast to an interface whose method a class embedding the class overrides", true
			}
		}
	}
	if !onlyCalled(c, 4) {
		return "a cross-cast whose value is stored, compared or given to code that is not only calling it (the slot holds the most derived object)", true
	}
	iupNeed[it.Obj().Name()] = true
	counts["cross-casts icast_I(x.self()) to an interface the class implements, of a value only called, are iup_I(x)"]++
	rw.Rewrite[c] = func() string { return "iup_" + it.Obj().Name() + "(" + rw.Render(ss.X) + ")" }
	return "", true
}

// dispatchesToSelf: every statement of fd is a call of a method on this.self_ (or, under the
// exact-class test, on this), returned or not: the method runs what the most derived object's
// runs.
func dispatchesToSelf(fd *ast.FuncDecl) bool {
	if fd == nil || fd.Body == nil || len(fd.Body.List) == 0 {
		return false
	}
	recv := recvObj(fd)
	sawSelf := false
	var stmt func(st ast.Stmt, exact bool) bool
	stmt = func(st ast.Stmt, exact bool) bool {
		var e ast.Expr
		switch x := st.(type) {
		case *ast.ReturnStmt:
			if len(x.Results) == 0 {
				return exact
			}
			if len(x.Results) != 1 {
				return false
			}
			e = x.Results[0]
		case *ast.ExprStmt:
			e = x.X
		case *ast.IfStmt:
			if x.Else != nil || exact {
				return false
			}
			for _, b := range x.Body.List {
				if !stmt(b, true) {
					return false
				}
			}
			return true
		default:
			return false
		}
		c, ok := rw.Unparen(e).(*ast.CallExpr)
		if !ok {
			return false
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok {
			return false
		}
		if isSelf(sel.X, recv) {
			sawSelf = true
			return true
		}
		return exact && intoRecv(sel.X, recv)
	}
	for _, st := range fd.Body.List {
		if !stmt(st, false) {
			return false
		}
	}
	return sawSelf
}

// onlyCalled: the value of e is only the receiver of method calls: e is one, or an argument of
// a parameter that its function only calls (or passes on to such a parameter) and never assigns.
func onlyCalled(e ast.Expr, depth int) bool {
	if depth == 0 {
		return false
	}
	p := rw.Up(e)
	if sel, ok := p.(*ast.SelectorExpr); ok && rw.Unparen(sel.X) == rw.Unparen(e) {
		c, ok := rw.Parent[sel].(*ast.CallExpr)
		return ok && c.Fun == sel
	}
	c, ok := p.(*ast.CallExpr)
	if !ok {
		return false
	}
	idx := -1
	for i, a := range c.Args {
		if rw.Unparen(a) == rw.Unparen(e) {
			idx = i
		}
	}
	if idx < 0 || c.Ellipsis.IsValid() {
		return false
	}
	var fo *types.Func
	switch f := rw.Unparen(c.Fun).(type) {
	case *ast.Ident:
		fo, _ = rw.Info.Uses[f].(*types.Func)
	case *ast.SelectorExpr:
		if s := rw.Info.Selections[f]; s != nil && s.Kind() == types.MethodVal {
			if _, isIface := s.Recv().Underlying().(*types.Interface); isIface {
				return false
			}
			fo, _ = s.Obj().(*types.Func)
		}
	}
	if fo == nil {
		return false
	}
	fd := declOf[fo]
	if fd == nil || fd.Body == nil {
		return false
	}
	sig := fo.Type().(*types.Signature)
	if sig.Variadic() && idx >= sig.Params().Len()-1 {
		return false
	}
	pv := sig.Params().At(idx)
	var pobj types.Object
	for _, fl := range fd.Type.Params.List {
		for _, n := range fl.Names {
			if o := rw.Info.Defs[n]; o != nil && o.Pos() == pv.Pos() {
				pobj = o
			}
		}
	}
	if pobj == nil {
		return false
	}
	for _, id := range rw.UsesOf[pobj] {
		if !onlyCalled(id, depth-1) {
			return false
		}
	}
	return true
}

// writeIup adds the iup_ functions the rewrites call and the package does not declare yet to
// g_0iup.go, as castfold -nilup writes them.
func writeIup() int {
	var names []string
	for n := range iupNeed {
		if rw.Pkg.Scope().Lookup("iup_"+n) == nil {
			names = append(names, n)
		}
	}
	if len(names) == 0 {
		return 0
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		fmt.Fprintf(&b, "\n// iup_%s converts a p whose type implements %s to it, nil to nil.\nfunc iup_%s[P interface {\n\tcomparable\n\t%s\n}](p P) %s {\n\tvar z P\n\tif p == z {\n\t\treturn nil\n\t}\n\treturn p\n}\n", n, n, n, n, n)
	}
	decls := b.String()
	for _, f := range rw.Files {
		if filepath.Base(rw.Fset.File(f.Pos()).Name()) == "g_0iup.go" {
			last := f.Decls[len(f.Decls)-1]
			rw.Rewrite[last] = func() string { return rw.RenderChildren(last) + "\n" + decls }
			return len(names)
		}
	}
	panic("selftype: g_0iup.go not found")
}
