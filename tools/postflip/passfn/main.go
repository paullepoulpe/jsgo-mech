// Command passfn is a post-flip step: the three functions a PassFactory holds (its condition,
// its precondition check and its internal factory) are typed Go functions with an identity,
// instead of erased Function objects.
//
// PassFactory is an AutoValue class: its equals, hashCode and toString compare, hash and print
// the three Function fields, which in Java compares lambda objects by identity (a lambda that
// captures nothing is one object per site, the others one object per evaluation). A Go func
// value has no identity, so each field holds a *funcObj_[F] (rt_core.go): a pointer to a func
// value of the typed signature F, made where Java made the lambda object (the package-level
// lambdas of the lambdas step once, the others at each evaluation), so == and the identity hash
// of the pointer are those of the Java object. What changes is the function: F takes and
// returns the declared types (func(*AbstractCompiler) CompilerPass for the factory, func(
// *CompilerOptions) bool for the condition, func(*CompilerOptions)
// *PassFactory_PreconditionResult for the check), so the lambda no longer casts its argument
// and boxes its result, and PassFactory.create, validatePreconditions and
// PassListBuilder.maybeAdd call f.call(x) instead of casting f.apply(box).
//
// The slots: the PassFactory fields of type FunctionOf[A, B], the PassFactory_Builder fields of
// the same types, the parameters of that type of their methods and of new_PassFactory, and the
// results of the getters that return those fields (declared Function). Every value stored in a
// slot must be another slot, a lambda literal &Function_fn{func(x_ any) any {...}} or a
// package-level lambda whose every use is such a value; every use of a slot must store it in a
// slot, return it from its getter, call it (f.apply(box) under the cast to B), compare it (with
// nil or another slot) or hash or print it by identity (objhashId_, objstr_). Otherwise nothing
// is changed and the use is reported.
//
// A lambda becomes newFuncObj_(func(x A) B {...}): its first statements `var x A = cast(x_)` and
// `_ = x` go (the parameter is x, of type A: the cast was of the value the caller boxed, of type
// A), and so do casts of x_ to A elsewhere in the body (a method reference: to_X(p0)); a result `return box(e)` returns e when e's type is B, or is an interface B accepts, or is
// a new object (never nil); bool(e) of a Boolean condition is e; a result of another type is
// converted as the caller converted it (icast_B / cast_[B] / unbox_[bool]), so a null stays
// null and a wrong class fails with the same ClassCastException, at the same call.
//
// usage: passfn [-n] <dir of package closure>
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

// slot kinds, by the field of PassFactory they hold
type kind struct {
	alias string     // PassFactory_Condition ...
	a, b  types.Type // FunctionOf[A, B]
	aText string
	bText string // B as Go writes the typed result (bool for BoxedOf[bool])
	boxed bool   // B is BoxedOf[bool]
}

var (
	kinds     = map[string]*kind{} // by the text of FunctionOf[A, B]
	slotVars  = map[types.Object]*kind{}
	getters   = map[types.Object]*kind{} // methods returning a slot field
	failures  []string
	lambdas   = map[types.Object]*kind{} // package-level lambdas stored in slots
	litKinds  = map[*ast.CompositeLit]*kind{}
	applies   = 0
	typeEdits = 0
)

func fail(n ast.Node, why string) {
	failures = append(failures, fmt.Sprintf("%s: %s", rw.Fset.Position(n.Pos()), why))
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	pf := rw.Pkg.Scope().Lookup("PassFactory")
	pb := rw.Pkg.Scope().Lookup("PassFactory_Builder")
	if pf == nil || pb == nil {
		fmt.Fprintln(os.Stderr, "passfn: no PassFactory")
		return
	}
	// 1. The slot types: the FunctionOf[A, B] fields of PassFactory.
	var file *ast.File
	for _, f := range rw.Files {
		if strings.HasSuffix(rw.Fset.Position(f.Pos()).Filename, "/g_PassFactory.go") {
			file = f
		}
	}
	if file == nil {
		fmt.Fprintln(os.Stderr, "passfn: no g_PassFactory.go")
		return
	}
	st := pf.Type().Underlying().(*types.Struct)
	for i := 0; i < st.NumFields(); i++ {
		fv := st.Field(i)
		if !strings.HasPrefix(fv.Name(), "get") {
			continue
		}
		k := kindOf(fv)
		if k == nil {
			continue
		}
		k.alias = "PassFactory_" + strings.TrimSuffix(strings.TrimPrefix(fv.Name(), "get"), "_")
		slotVars[fv] = k
	}
	if len(kinds) == 0 {
		fmt.Fprintln(os.Stderr, "passfn: no function fields (already converted?)")
		return
	}
	bt := pb.Type().Underlying().(*types.Struct)
	for i := 0; i < bt.NumFields(); i++ {
		if k := kindOf(bt.Field(i)); k != nil {
			slotVars[bt.Field(i)] = k
		}
	}
	// Parameters of that type of the two classes' functions, and the getters.
	ast.Inspect(file, func(n ast.Node) bool {
		fd, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		for _, fl := range fd.Type.Params.List {
			for _, nm := range fl.Names {
				if k := kindOf(rw.Info.Defs[nm]); k != nil {
					slotVars[rw.Info.Defs[nm]] = k
				}
			}
		}
		if fd.Recv != nil && len(fd.Body.List) == 1 && fd.Type.Params.NumFields() == 0 {
			if r, ok := fd.Body.List[0].(*ast.ReturnStmt); ok && len(r.Results) == 1 {
				if s, ok := r.Results[0].(*ast.SelectorExpr); ok && rw.Text(s.X) == "this" {
					if k := slotVars[rw.Info.Uses[s.Sel]]; k != nil {
						getters[rw.Info.Defs[fd.Name]] = k
					}
				}
			}
		}
		return true
	})
	// 2. Every value stored in a slot parameter, every use of a slot.
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if fn := calleeOf(c); fn != nil {
				for i, a := range c.Args {
					if k := paramKind(fn, i); k != nil {
						store(a, k)
					}
				}
			}
			return true
		})
	}
	for o, k := range slotVars {
		for _, id := range rw.UsesOf[o] {
			use(rw.VarExpr(id), k)
		}
	}
	for g, k := range getters {
		for _, id := range rw.UsesOf[g] {
			sel, ok := rw.Parent[id].(*ast.SelectorExpr)
			if !ok {
				fail(id, "getter used as a value")
				continue
			}
			c, ok := rw.Parent[sel].(*ast.CallExpr)
			if !ok {
				fail(id, "getter used as a value")
				continue
			}
			use(c, k)
		}
	}
	// Package-level lambdas: every use must be a value stored in a slot (checked by store).
	for o, k := range lambdas {
		for _, id := range rw.UsesOf[o] {
			if lambdaUse[id] != k {
				fail(id, "a package-level lambda used otherwise")
			}
		}
	}
	if len(failures) > 0 {
		sort.Strings(failures)
		for _, f := range failures {
			fmt.Fprintln(os.Stderr, "  left: "+f)
		}
		fmt.Fprintf(os.Stderr, "passfn: %d uses do not fit; nothing changed\n", len(failures))
		os.Exit(1)
	}
	// 3. The rewrites.
	for _, f := range rw.Files {
		if f != file {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.IndexListExpr:
				if k := kinds[rw.Text(x)]; k != nil {
					typeEdits++
					rw.Rewrite[x] = func() string { return k.alias }
				}
			case *ast.FuncDecl:
				if k := getters[rw.Info.Defs[x.Name]]; k != nil && len(x.Type.Results.List) == 1 {
					typeEdits++
					res := x.Type.Results.List[0].Type
					rw.Rewrite[res] = func() string { return k.alias }
				}
			}
			return true
		})
	}
	for o, k := range lambdas {
		v := o.(*types.Var)
		spec := declOf(v)
		lit := rw.Unparen(spec.Values[0]).(*ast.UnaryExpr).X.(*ast.CompositeLit)
		litKinds[lit] = k
	}
	nlits := 0
	for lit, k := range litKinds {
		text, why := typed(lit, k)
		if why != "" {
			fail(lit, why)
			continue
		}
		nlits++
		u := rw.Parent[lit].(*ast.UnaryExpr)
		rw.Rewrite[u] = text
	}
	if len(failures) > 0 {
		for _, f := range failures {
			fmt.Fprintln(os.Stderr, "  left: "+f)
		}
		fmt.Fprintf(os.Stderr, "passfn: %d lambdas do not fit; nothing changed\n", len(failures))
		os.Exit(1)
	}
	// The aliases.
	last := file.Decls[len(file.Decls)-1]
	names := []string{}
	for _, k := range kinds {
		names = append(names, k.alias)
	}
	sort.Strings(names)
	var decls strings.Builder
	decls.WriteString("\n\n// The functions a PassFactory holds, each with the identity of the Java lambda object it was\n// made from: PassFactory's equals, hashCode and toString compare, hash and print them (post-flip\n// step passfn).\n")
	for _, nm := range names {
		for _, k := range kinds {
			if k.alias == nm {
				decls.WriteString("type " + k.alias + " = *funcObj_[func(" + k.aText + ") " + k.bText + "]\n")
			}
		}
	}
	prev := rw.Rewrite[last]
	rw.Rewrite[last] = func() string {
		s := rw.RenderChildren(last)
		if prev != nil {
			s = prev()
		}
		return s + strings.TrimSuffix(decls.String(), "\n")
	}
	msg := fmt.Sprintf("passfn: %d slot types, %d calls, %d lambdas (%d package-level)", typeEdits, applies, nlits, len(lambdas))
	if report {
		fmt.Fprintln(os.Stderr, msg)
		return
	}
	fmt.Fprintf(os.Stderr, "%s; files changed: %d\n", msg, rw.SaveChecked())
}

func kindOf(o types.Object) *kind {
	v, ok := o.(*types.Var)
	if !ok {
		return nil
	}
	a, ok := v.Type().(*types.Alias)
	if !ok || a.Obj().Name() != "FunctionOf" || a.TypeArgs().Len() != 2 {
		return nil
	}
	A, B := a.TypeArgs().At(0), a.TypeArgs().At(1)
	q := types.RelativeTo(rw.Pkg)
	key := "FunctionOf[" + types.TypeString(A, q) + ", " + types.TypeString(B, q) + "]"
	if k := kinds[key]; k != nil {
		return k
	}
	k := &kind{a: A, b: B, aText: types.TypeString(A, q), bText: types.TypeString(B, q)}
	if ba, ok := B.(*types.Alias); ok && ba.Obj().Name() == "BoxedOf" && ba.TypeArgs().Len() == 1 && types.Identical(ba.TypeArgs().At(0), types.Typ[types.Bool]) {
		k.b, k.bText, k.boxed = types.Typ[types.Bool], "bool", true
	}
	kinds[key] = k
	return k
}

func declOf(v *types.Var) *ast.ValueSpec {
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.VAR {
				continue
			}
			for _, s := range g.Specs {
				vs := s.(*ast.ValueSpec)
				for _, n := range vs.Names {
					if rw.Info.Defs[n] == v {
						return vs
					}
				}
			}
		}
	}
	return nil
}

// isSlot: an expression that reads a slot (a slot variable or field, a getter call).
func isSlot(e ast.Expr) *kind {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		return slotVars[rw.Info.Uses[x]]
	case *ast.SelectorExpr:
		return slotVars[rw.Info.Uses[x.Sel]]
	case *ast.CallExpr:
		if s, ok := x.Fun.(*ast.SelectorExpr); ok && len(x.Args) == 0 {
			return getters[rw.Info.Uses[s.Sel]]
		}
	}
	return nil
}

var lambdaUse = map[*ast.Ident]*kind{}

// store checks a value stored in a slot of kind k.
func store(v ast.Expr, k *kind) {
	v = rw.Unparen(v)
	if sk := isSlot(v); sk != nil {
		if sk != k {
			fail(v, "a slot stored in a slot of another type")
		}
		return
	}
	if id, ok := v.(*ast.Ident); ok {
		if pv, ok := rw.Info.Uses[id].(*types.Var); ok && pv.Parent() == rw.Pkg.Scope() && strings.HasPrefix(pv.Name(), "lambda_") {
			spec := declOf(pv)
			if spec == nil || len(spec.Values) != 1 || !isFnLit(spec.Values[0]) {
				fail(v, "a package variable that is not a lambda")
				return
			}
			if old := lambdas[pv]; old != nil && old != k {
				fail(v, "a package-level lambda stored in slots of two types")
			}
			lambdas[pv] = k
			lambdaUse[id] = k
			return
		}
	}
	if isFnLit(v) {
		lit := v.(*ast.UnaryExpr).X.(*ast.CompositeLit)
		litKinds[lit] = k
		return
	}
	fail(v, "a value stored in a slot that is not a lambda or a slot: "+rw.Text(v))
}

func isFnLit(e ast.Expr) bool {
	u, ok := rw.Unparen(e).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return false
	}
	cl, ok := u.X.(*ast.CompositeLit)
	if !ok || rw.Text(cl.Type) != "Function_fn" || len(cl.Elts) != 1 {
		return false
	}
	_, ok = cl.Elts[0].(*ast.FuncLit)
	return ok
}

// use checks a use e of a slot of kind k (e reads the slot).
func use(e ast.Expr, k *kind) {
	p := rw.Up(e)
	switch x := p.(type) {
	case *ast.AssignStmt: // stored into a slot, or a slot assigned
		for i, l := range x.Lhs {
			if rw.Unparen(l) == e {
				store(x.Rhs[i], k)
				return
			}
		}
		for i, r := range x.Rhs {
			if rw.Unparen(r) == e {
				if isSlot(x.Lhs[i]) != k {
					fail(e, "assigned to something else")
				}
				return
			}
		}
	case *ast.ReturnStmt: // the getter
		fd := enclosingFunc(x)
		if fd == nil || getters[rw.Info.Defs[fd.Name]] != k {
			fail(e, "returned from a function that is not its getter")
		}
		return
	case *ast.BinaryExpr:
		other := x.X
		if rw.Unparen(x.X) == e {
			other = x.Y
		}
		if (x.Op == token.EQL || x.Op == token.NEQ) && (rw.Text(other) == "nil" || isSlot(other) == k) {
			return
		}
		fail(e, "compared with something else")
		return
	case *ast.CallExpr:
		name, _ := rw.CallName(x)
		if name == "objhashId_" || name == "objstr_" {
			return
		}
		// An argument passed to a slot parameter.
		if fn := calleeOf(x); fn != nil {
			for i, a := range x.Args {
				if rw.Unparen(a) == e {
					if pk := paramKind(fn, i); pk == k {
						return
					}
				}
			}
		}
		fail(e, "passed to "+rw.Text(x.Fun))
		return
	case *ast.SelectorExpr: // e.apply(box)
		if x.Sel.Name != "apply" {
			fail(e, "method "+x.Sel.Name)
			return
		}
		c, ok := rw.Parent[x].(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			fail(e, "apply used as a value")
			return
		}
		applyCall(c, e, k)
		return
	case *ast.KeyValueExpr, *ast.CompositeLit:
		fail(e, "in a composite literal")
		return
	}
	fail(e, fmt.Sprintf("used as %T", p))
}

func enclosingFunc(n ast.Node) *ast.FuncDecl {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		if fd, ok := p.(*ast.FuncDecl); ok {
			return fd
		}
	}
	return nil
}

func calleeOf(c *ast.CallExpr) *types.Func {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		fn, _ := rw.Info.Uses[f].(*types.Func)
		return fn
	case *ast.SelectorExpr:
		fn, _ := rw.Info.Uses[f.Sel].(*types.Func)
		return fn
	}
	return nil
}

func paramKind(fn *types.Func, i int) *kind {
	sig := fn.Type().(*types.Signature)
	if i >= sig.Params().Len() {
		return nil
	}
	return slotVars[sig.Params().At(i)]
}

// applyCall rewrites unwrap(f.apply(box(a))) as f.call(a).
func applyCall(c *ast.CallExpr, f ast.Expr, k *kind) {
	arg, ok := unbox(c.Args[0], k.a)
	if !ok {
		fail(c, "apply of a value that is not a boxed A: "+rw.Text(c.Args[0]))
		return
	}
	outer, ok := rw.Up(c).(*ast.CallExpr)
	if !ok || len(outer.Args) != 1 {
		fail(c, "apply's result not converted to B")
		return
	}
	on := rw.Text(outer.Fun)
	want := map[string]bool{"cast_[" + k.bText + "]": true, "icast_" + strings.TrimPrefix(k.bText, "*"): true}
	if k.boxed {
		want = map[string]bool{"unbox_[bool]": true}
	}
	if !want[on] {
		fail(c, "apply's result converted by "+on)
		return
	}
	applies++
	rw.Rewrite[outer] = func() string { return rw.Render(f) + ".call(" + arg() + ")" }
}

// unbox: a of obj_(a) / a.self() / any(a) with a of type t.
func unbox(e ast.Expr, t types.Type) (func() string, bool) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	var a ast.Expr
	switch f := c.Fun.(type) {
	case *ast.Ident:
		if (f.Name == "obj_" || f.Name == "any") && len(c.Args) == 1 {
			a = c.Args[0]
		}
	case *ast.SelectorExpr:
		if f.Sel.Name == "self" && len(c.Args) == 0 {
			a = f.X
		}
	}
	if a == nil || !types.Identical(rw.Info.Types[a].Type, t) {
		return nil, false
	}
	return func() string { return rw.Render(a) }, true
}

// typed is the text of newFuncObj_(func(x A) B {...}) for lambda lit, or why not.
func typed(lit *ast.CompositeLit, k *kind) (func() string, string) {
	fl := lit.Elts[0].(*ast.FuncLit)
	if fl.Type.Params.NumFields() != 1 || len(fl.Type.Params.List[0].Names) != 1 {
		return nil, "a lambda of another shape"
	}
	pname := fl.Type.Params.List[0].Names[0]
	pobj := rw.Info.Defs[pname]
	// var x A = cast(x_); _ = x
	name := "_"
	var dels []ast.Stmt
	if len(fl.Body.List) >= 2 {
		if d, ok := fl.Body.List[0].(*ast.DeclStmt); ok {
			if g, ok := d.Decl.(*ast.GenDecl); ok && len(g.Specs) == 1 {
				vs := g.Specs[0].(*ast.ValueSpec)
				if len(vs.Names) == 1 && len(vs.Values) == 1 && vs.Type != nil && types.Identical(rw.Info.Types[vs.Type].Type, k.a) {
					if c, ok := vs.Values[0].(*ast.CallExpr); ok && len(c.Args) == 1 && rw.Text(c.Args[0]) == pname.Name {
						if rw.Text(fl.Body.List[1]) == "_ = "+vs.Names[0].Name {
							name = vs.Names[0].Name
							dels = []ast.Stmt{fl.Body.List[0], fl.Body.List[1]}
						}
					}
				}
			}
		}
	}
	// The boxed parameter is used only by casts to A (a method reference: to_X(p0)).
	if len(dels) == 0 {
		name = pname.Name
	}
	for _, id := range rw.UsesOf[pobj] {
		if len(dels) > 0 && id.Pos() >= dels[0].Pos() && id.End() <= dels[0].End() {
			continue
		}
		c, ok := rw.Up(id).(*ast.CallExpr)
		if !ok || len(c.Args) != 1 || !types.Identical(rw.Info.Types[c].Type, k.a) {
			return nil, "the boxed parameter is used: " + rw.Text(rw.Up(id))
		}
		fn := rw.Text(c.Fun)
		if !strings.HasPrefix(fn, "to_") && !strings.HasPrefix(fn, "cast_[") && !strings.HasPrefix(fn, "icast_") {
			return nil, "the boxed parameter is used: " + rw.Text(c)
		}
		n := name
		rw.Rewrite[c] = func() string { return n }
	}
	// The results.
	var why string
	var rets []*ast.ReturnStmt
	ast.Inspect(fl.Body, func(n ast.Node) bool {
		switch r := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			rets = append(rets, r)
		}
		return true
	})
	for _, r := range rets {
		if len(r.Results) != 1 {
			why = "a return without a value"
			continue
		}
		res := rw.Unparen(r.Results[0])
		txt, ok := result(res, k)
		if !ok {
			why = "a result not of B: " + rw.Text(res)
			continue
		}
		rw.Rewrite[res] = txt
	}
	if why != "" {
		return nil, why
	}
	for _, d := range dels {
		rw.Rewrite[d] = func() string { return rw.Del }
	}
	return func() string {
		return "newFuncObj_(func(" + name + " " + k.aText + ") " + k.bText + " " + rw.Render(fl.Body) + ")"
	}, ""
}

// result is the typed result of `return e` in a lambda of kind k.
func result(e ast.Expr, k *kind) (func() string, bool) {
	e = rw.Unparen(e)
	render := func(x ast.Expr) string { // e itself is the node rewritten
		if x == e {
			return rw.RenderChildren(x)
		}
		return rw.Render(x)
	}
	inner := e
	if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 {
		if f, ok := c.Fun.(*ast.Ident); ok && (f.Name == "obj_" || f.Name == "any") {
			inner = rw.Unparen(c.Args[0])
		}
	}
	t := rw.Info.Types[inner].Type
	if k.boxed {
		if c, ok := inner.(*ast.CallExpr); ok && rw.Text(c.Fun) == "bool" && len(c.Args) == 1 {
			return func() string { return render(c.Args[0]) }, true
		}
		if b, ok := t.(*types.Basic); ok && b.Kind() == types.Bool || types.Identical(t, types.Typ[types.UntypedBool]) {
			return func() string { return render(inner) }, true
		}
		return func() string { return "unbox_[bool](" + render(e) + ")" }, true
	}
	if types.Identical(t, k.b) {
		return func() string { return render(inner) }, true
	}
	if types.IsInterface(k.b) && types.AssignableTo(t, k.b) {
		if types.IsInterface(t) || isNew(inner) {
			return func() string { return render(inner) }, true
		}
	}
	// Converted as the caller converted the Object.
	if types.IsInterface(k.b) {
		return func() string { return "icast_" + k.bText + "(" + render(e) + ")" }, isBoxed(e)
	}
	return func() string { return "cast_[" + k.bText + "](" + render(e) + ")" }, isBoxed(e)
}

func isBoxed(e ast.Expr) bool {
	t := rw.Info.Types[e].Type
	return t != nil && types.IsInterface(t) && t.Underlying().(*types.Interface).Empty()
}

// isNew: a value that is never nil (a constructor call, an address of a composite literal, a
// package-level lambda of the lambdas step: made by its declaration, never assigned).
func isNew(e ast.Expr) bool {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		v, ok := rw.Info.Uses[x].(*types.Var)
		if !ok || v.Parent() != rw.Pkg.Scope() || !strings.HasPrefix(v.Name(), "lambda_") {
			return false
		}
		for _, id := range rw.UsesOf[v] {
			if as, ok := rw.Up(id).(*ast.AssignStmt); ok {
				for _, l := range as.Lhs {
					if rw.Unparen(l) == ast.Expr(id) {
						return false
					}
				}
			}
		}
		spec := declOf(v)
		if spec == nil || len(spec.Values) != 1 {
			return false
		}
		u, ok := rw.Unparen(spec.Values[0]).(*ast.UnaryExpr)
		if !ok || u.Op != token.AND {
			return false
		}
		_, ok = u.X.(*ast.CompositeLit)
		return ok
	case *ast.CallExpr:
		if id, ok := x.Fun.(*ast.Ident); ok && strings.HasPrefix(id.Name, "new_") {
			return true
		}
	case *ast.UnaryExpr:
		_, ok := x.X.(*ast.CompositeLit)
		return x.Op == token.AND && ok
	}
	return false
}
