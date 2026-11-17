// Command eqtyped is a post-flip step: a class's equals(Object) gets a typed form, and the calls
// whose argument has the class's type call it with the value itself instead of boxing it.
//
// j2go writes Java's `boolean equals(Object o)` as `equals(o any) bool`, whose body tests the
// class of o (`o instanceof C c`, a cast after `instanceof`, or AutoValue's
// `if (o instanceof C) { C that = (C) o; ... }`) and then compares fields of the bound value.
// A call a.equals(b) with b of static type C is written a.equals(b.self()) (or obj_(b)): the
// value goes through Object and the test only finds again what the caller knew.
//
// For a class C whose equals is generated with one of these shapes, and whose parameter is used
// only to test it against `this` (o == any(this), any(this) == o, this.self() == o) or to bind
// it to a variable x of type *C by a test of C (`func() bool { v_ := o; p, ok := v_.(T); if !ok
// { return false }; x = p[.as_C()]; return true }()`, or `if x, ok := o.(*C); ok {`, with T *C
// or C_as), the step writes
//
//	func (this *C) equals(o any) bool { [the identity test;] p, ok := o.(T); return ok && this.equals_C(p[.as_C()]) }
//	func (this *C) equals_C(x *C) bool { <the body: the test is x != nil, x == this the identity test> }
//
// x != nil is the test of the dynamic class: the value an Object parameter holds is a C exactly
// when the assertion holds, and a null is no C (equals_C receives the asserted value, nil only
// for null); `o == this.self()` holds exactly when o is this object, that is when x == this.
// A call a.equals(b.self()) / a.equals(obj_(b)) / a.equals(any(b)) that resolves to C.equals,
// with b of type *C or of a subclass of C (b.up_C(), nil for nil), becomes a.equals_C(b) and
// computes the same: the argument is C's view of the same object.
// An equals that only casts its parameter (cast_[*C](o): Java's `((C) o).f == f`) gets
// equals(o) = equals_C(cast_[*C](o)), the cast done once on entry, as Java does it before
// reading the field. Other calls keep equals(any), which gives the same answers. Only a class
// that such a call uses gets the typed form; classes and calls left are counted with the reason.
//
// usage: eqtyped [-n] <dir of package closure>
package main

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

type class struct {
	name   string // C
	x      string // the bound variable, the typed parameter's name
	assert string // T
	asC    bool   // T is C_as: p.as_C()
	apply  func() // registers the rewrite of the method
	calls  int
}

var converted = map[*types.Func]*class{}
var left = map[string]int{}

func generated(n ast.Node) bool {
	fn := rw.Fset.Position(n.Pos()).Filename
	return strings.HasPrefix(fn[strings.LastIndex(fn, "/")+1:], "g_")
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Name.Name == "equals" && fd.Recv != nil && fd.Body != nil {
				if why := method(fd); why != "" {
					left["equals left: "+why]++
				}
			}
		}
	}
	calls := 0
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) != 1 {
				return true
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "equals" {
				return true
			}
			s, ok := rw.Info.Selections[sel]
			if !ok || s.Kind() != types.MethodVal {
				return true
			}
			cl := converted[s.Obj().(*types.Func)]
			if cl == nil {
				left["call left: equals of a class without a typed form"]++
				return true
			}
			b, ok := boxed(c.Args[0])
			if !ok {
				left["call left: the argument is not a boxed value of a static type"]++
				return true
			}
			arg, ok := asC(b, cl.name)
			if !ok {
				left["call left: the argument's type is not C or a subclass"]++
				return true
			}
			calls++
			cl.calls++
			rw.Rewrite[c] = func() string { return rw.Render(sel.X) + ".equals_" + cl.name + "(" + arg() + ")" }
			return true
		})
	}
	// Only the classes that a call uses get the typed form.
	classes := 0
	for _, cl := range converted {
		if cl.calls > 0 {
			classes++
			cl.apply()
		}
	}
	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", left[k], k)
	}
	msg := fmt.Sprintf("eqtyped: %d classes with a typed equals (of %d that could have one); %d calls typed", classes, len(converted), calls)
	if report {
		fmt.Fprintln(os.Stderr, msg)
		return
	}
	fmt.Fprintf(os.Stderr, "%s; files changed: %d\n", msg, rw.SaveChecked())
}

// boxed: b of b.self(), obj_(b), any(b).
func boxed(e ast.Expr) (ast.Expr, bool) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	switch f := c.Fun.(type) {
	case *ast.SelectorExpr:
		if f.Sel.Name == "self" && len(c.Args) == 0 {
			if s, ok := rw.Info.Selections[f]; ok && s.Kind() == types.MethodVal {
				return f.X, true
			}
		}
	case *ast.Ident:
		if (f.Name == "obj_" || f.Name == "any") && len(c.Args) == 1 {
			return c.Args[0], true
		}
	}
	return nil, false
}

// asC is b as a *C: b itself, or b.up_C() of a subclass.
func asC(b ast.Expr, c string) (func() string, bool) {
	p, ok := types.Unalias(rw.Info.Types[b].Type).(*types.Pointer)
	if !ok {
		return nil, false
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok {
		return nil, false
	}
	if n.Obj().Name() == c {
		return func() string { return rw.Render(b) }, true
	}
	if m, _, _ := types.LookupFieldOrMethod(p, true, rw.Pkg, "up_"+c); m != nil {
		if fn, ok := m.(*types.Func); ok {
			sig := fn.Type().(*types.Signature)
			if sig.Params().Len() == 0 && sig.Results().Len() == 1 && types.TypeString(sig.Results().At(0).Type(), types.RelativeTo(rw.Pkg)) == "*"+c {
				return func() string { return rw.Render(b) + ".up_" + c + "()" }, true
			}
		}
	}
	return nil, false
}

// method recognises C.equals and registers its rewrite, or says why not.
func method(fd *ast.FuncDecl) string {
	if len(fd.Recv.List) != 1 || fd.Type.Params.NumFields() != 1 || fd.Type.Results.NumFields() != 1 {
		return "not equals(Object)"
	}
	star, ok := fd.Recv.List[0].Type.(*ast.StarExpr)
	if !ok {
		return "receiver is not a pointer"
	}
	cname := rw.Text(star.X)
	pf := fd.Type.Params.List[0]
	if len(pf.Names) != 1 || rw.Text(pf.Type) != "any" || len(fd.Recv.List[0].Names) != 1 || fd.Recv.List[0].Names[0].Name != "this" {
		return "not equals(Object)"
	}
	po := rw.Info.Defs[pf.Names[0]]
	fobj := rw.Info.Defs[fd.Name].(*types.Func)
	var x string
	var assertT string
	var viaAs bool
	bind := func(name, t string, as bool) bool {
		if x != "" && (x != name || assertT != t) {
			return false
		}
		x, assertT, viaAs = name, t, as
		return true
	}
	why := ""
	fail := func(w string) {
		if why == "" {
			why = w
		}
	}
	edits := map[ast.Node]string{}
	ident := "" // the identity test, kept first in equals(any)
	var xDecls []ast.Stmt
	// Every use of the parameter.
	for _, id := range rw.UsesOf[po] {
		p := rw.Up(id)
		switch u := p.(type) {
		case *ast.BinaryExpr: // identity: o == any(this), any(this) == o, this.self() == o
			other := u.Y
			if rw.Unparen(u.Y) == ast.Expr(id) {
				other = u.X
			}
			if u.Op != token.EQL || !isThis(other) {
				fail("the parameter is compared with something else")
				continue
			}
			edits[u] = "IDENT"
			if ident == "" {
				ident = rw.Text(u)
			}
		case *ast.AssignStmt: // v_ := o of the binding function literal
			lit, x0, t, as, ok := bindingLit(u)
			if !ok || !bind(x0, t, as) {
				fail("the parameter is used otherwise")
				continue
			}
			edits[lit] = "BIND"
		case *ast.TypeAssertExpr: // if that, ok := o.(*C); ok {
			as, ok := rw.Up(u).(*ast.AssignStmt)
			if !ok {
				fail("the parameter is asserted otherwise")
				continue
			}
			is, ok := rw.Parent[as].(*ast.IfStmt)
			if !ok || is.Init != ast.Stmt(as) || len(as.Lhs) != 2 || as.Tok != token.DEFINE || rw.Text(is.Cond) != rw.Text(as.Lhs[1]) || rw.Text(u.Type) != "*"+cname {
				fail("the parameter is asserted otherwise")
				continue
			}
			if !bind(rw.Text(as.Lhs[0]), "*"+cname, false) {
				fail("two bound variables")
				continue
			}
			edits[is] = "IFBIND"
		case *ast.CallExpr: // cast_[*C](o): a cast without a test (ClassCastException otherwise)
			if rw.Text(u.Fun) != "cast_[*"+cname+"]" || len(u.Args) != 1 {
				fail("the parameter is used otherwise")
				continue
			}
			if !bind(pf.Names[0].Name, "cast", false) {
				fail("the parameter is both cast and tested")
				continue
			}
			edits[u] = "CAST"
		default:
			fail("the parameter is used otherwise")
		}
	}
	if why != "" {
		return why
	}
	if x == "" {
		return "no test of the parameter's class"
	}
	if x == "this" || x == "_" {
		return "bound variable named this"
	}
	if assertT == "cast" {
		return castForm(fd, cname, x, edits)
	}
	// The bound variable's declaration `var x *C` and `_ = x`, and no other assignment.
	for _, st := range fd.Body.List {
		switch s := st.(type) {
		case *ast.DeclStmt:
			if g, ok := s.Decl.(*ast.GenDecl); ok && len(g.Specs) == 1 {
				if vs, ok := g.Specs[0].(*ast.ValueSpec); ok && len(vs.Names) == 1 && vs.Names[0].Name == x && len(vs.Values) == 0 {
					if rw.Text(vs.Type) != "*"+cname {
						return "the bound variable is not a *C"
					}
					xDecls = append(xDecls, s)
				}
			}
		case *ast.AssignStmt:
			if rw.Text(s) == "_ = "+x {
				xDecls = append(xDecls, s)
			}
		}
	}
	var xobj types.Object
	for id, o := range rw.Info.Defs {
		if id.Name == x && id.Pos() >= fd.Pos() && id.End() <= fd.End() {
			if xobj != nil && xobj != o {
				return "the bound variable's name is declared twice"
			}
			xobj = o
		}
	}
	if xobj == nil {
		return "the bound variable is not declared"
	}
	for _, id := range rw.UsesOf[xobj] {
		if as, ok := rw.Up(id).(*ast.AssignStmt); ok {
			for _, l := range as.Lhs {
				if rw.Unparen(l) == ast.Expr(id) {
					if lit, ok := enclosingLit(as); !ok || edits[lit] != "BIND" {
						if rw.Text(as) != "_ = "+x {
							return "the bound variable is assigned elsewhere"
						}
					}
				}
			}
		}
	}
	// The rewrites, registered when a call uses the typed form.
	cl := &class{name: cname, x: x, assert: assertT, asC: viaAs}
	converted[fobj] = cl
	cl.apply = func() {
		for n, k := range edits {
			n, k := n, k
			switch k {
			case "IDENT":
				rw.Rewrite[n] = func() string { return "(" + x + " == this)" }
			case "BIND":
				rw.Rewrite[n] = func() string { return "(" + x + " != nil)" }
			case "IFBIND":
				is := n.(*ast.IfStmt)
				rw.Rewrite[is] = func() string {
					out := "if " + x + " != nil " + rw.Render(is.Body)
					if is.Else != nil {
						out += " else " + rw.Render(is.Else)
					}
					return out
				}
			}
		}
		for _, s := range xDecls {
			rw.Rewrite[s] = func() string { return rw.Del }
		}
		pname := pf.Names[0].Name
		conv := "p"
		if viaAs {
			conv = "p.as_" + cname + "()"
		}
		rw.Rewrite[fd] = func() string {
			first := ""
			if ident != "" { // the cheap identity test first, as before (collections call equals(any))
				first = "if " + ident + " {\nreturn true\n}\n"
			}
			return "func (this *" + cname + ") equals(" + pname + " any) bool {\n" + first +
				"p, ok := " + pname + ".(" + assertT + ")\n" +
				"return ok && this.equals_" + cname + "(" + conv + ")\n}\n\n" +
				"// equals_" + cname + " is equals of a " + cname + " (post-flip step eqtyped).\n" +
				"func (this *" + cname + ") equals_" + cname + "(" + x + " *" + cname + ") bool " + rw.Render(fd.Body)
		}
	}
	return ""
}

func isThis(e ast.Expr) bool {
	t := rw.Text(rw.Unparen(e))
	return t == "any(this)" || t == "this.self()"
}

// bindingLit recognises the instanceof-binding function literal around `v_ := o`:
// func() bool { v_ := o; p, ok := v_.(T); if !ok { return false }; x = p[.as_C()]; return true }().
func bindingLit(first *ast.AssignStmt) (ast.Expr, string, string, bool, bool) {
	blk, ok := rw.Parent[first].(*ast.BlockStmt)
	if !ok || len(blk.List) != 5 || blk.List[0] != ast.Stmt(first) || rw.Text(first.Lhs[0]) != "v_" {
		return nil, "", "", false, false
	}
	lit, ok := rw.Parent[blk].(*ast.FuncLit)
	if !ok {
		return nil, "", "", false, false
	}
	call, ok := rw.Parent[lit].(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil, "", "", false, false
	}
	as, ok := blk.List[1].(*ast.AssignStmt)
	if !ok || len(as.Lhs) != 2 || rw.Text(as.Lhs[0]) != "p" || rw.Text(as.Lhs[1]) != "ok" {
		return nil, "", "", false, false
	}
	ta, ok := as.Rhs[0].(*ast.TypeAssertExpr)
	if !ok || rw.Text(ta.X) != "v_" {
		return nil, "", "", false, false
	}
	if rw.Text(blk.List[2]) != "if !ok {\n\t\t\treturn false\n\t\t}" && !strings.HasPrefix(strings.Join(strings.Fields(rw.Text(blk.List[2])), " "), "if !ok { return false }") {
		return nil, "", "", false, false
	}
	set, ok := blk.List[3].(*ast.AssignStmt)
	if !ok || len(set.Lhs) != 1 || set.Tok != token.ASSIGN || rw.Text(blk.List[4]) != "return true" {
		return nil, "", "", false, false
	}
	t := rw.Text(ta.Type)
	rhs := rw.Text(set.Rhs[0])
	switch {
	case strings.HasPrefix(t, "*") && rhs == "p":
		return call, rw.Text(set.Lhs[0]), t, false, true
	case strings.HasSuffix(t, "_as") && rhs == "p.as_"+strings.TrimSuffix(t, "_as")+"()":
		return call, rw.Text(set.Lhs[0]), t, true, true
	}
	return nil, "", "", false, false
}

func enclosingLit(n ast.Node) (ast.Expr, bool) {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		if lit, ok := p.(*ast.FuncLit); ok {
			c, ok := rw.Parent[lit].(*ast.CallExpr)
			return c, ok
		}
	}
	return nil, false
}

// castForm: equals whose parameter o is only cast, cast_[*C](o) (Java's `((C) o).f == f`,
// ClassCastException for another class, null for null): equals(o) is equals_C(cast_[*C](o)) and
// equals_C(o *C) has the body with the casts replaced by o.
func castForm(fd *ast.FuncDecl, cname, x string, edits map[ast.Node]string) string {
	fobj := rw.Info.Defs[fd.Name].(*types.Func)
	cl := &class{name: cname, x: x, assert: "cast"}
	converted[fobj] = cl
	cl.apply = func() {
		for n := range edits {
			rw.Rewrite[n] = func() string { return x }
		}
		rw.Rewrite[fd] = func() string {
			return "func (this *" + cname + ") equals(" + x + " any) bool {\n" +
				"return this.equals_" + cname + "(cast_[*" + cname + "](" + x + "))\n}\n\n" +
				"// equals_" + cname + " is equals of a " + cname + " (post-flip step eqtyped).\n" +
				"func (this *" + cname + ") equals_" + cname + "(" + x + " *" + cname + ") bool " + rw.Render(fd.Body)
		}
	}
	return ""
}
