// Command fbound is a post-flip step: an F-bounded generic base class, AbstractScope<S, V> /
// AbstractVar<S, V>, has its code written out once per subclass with the type parameters
// substituted, as Scope / Var (S = Scope, V = Var) and TypedScope / TypedVar.
//
// Java declares
//
//	abstract class AbstractScope<S extends AbstractScope<S, V>, V extends AbstractVar<S, V>>
//	class AbstractVar<S extends AbstractScope<S, V>, V extends AbstractVar<S, V>>
//	final class Scope extends AbstractScope<Scope, Var>
//	class TypedScope extends AbstractScope<TypedScope, TypedVar>
//
// and the base's code works on S and V: `S getParent()`, `V getVar(String)`, `Map<String, V>
// vars`, `S scope` of a variable. j2go erases S and V to their bounds (*AbstractScope,
// *AbstractVar), so a caller that holds a Scope and asks for its parent or a variable gets the
// erased base and casts it back: cast_[*Var](scope.getVar(name).self()),
// scope.getParent().self().(*Scope), cast_[*Scope](v.getScope()) — javac's checkcasts of S
// and V, which cannot fail. Go has no F-bounded generics a base's methods could be written
// against without a dictionary call per use of S (a method of a type parameter is called
// through the instantiation's dictionary), so the step does what the hand port did (jsgo:
// Scope.GetVar returns *Var, TypedScope's its own; scopeCommon holds what does not mention S
// or V): each subclass gets the base's methods written out with S and V substituted.
//
// The family is read from the Java sources: the classes whose type parameters are bounded
// by the family's classes (the bound of S is AbstractScope, of V AbstractVar), and each
// subclass's `extends Base<A, B>` gives its substitution σ (AbstractScope -> Scope,
// AbstractVar -> Var). Then, for each base B:
//
//   - fields whose type mentions S or V (the variables table, the implicit variables, a
//     variable's scope) move into each subclass with σ applied to their type;
//   - the methods that mention S or V (in their signature, in a typed local, an expression of
//     type S or V, a moved field, or a call of such a method on this) are written out on each
//     subclass with σ applied to every type in them; the subclass's override of the method
//     (TypedScope.getVar), if any, is the body, with σ applied, and Java's super call in it
//     is a call of the base's body written out as <name>Super_. A dispatched method's typed
//     copy is direct: the hierarchy is closed (nothing extends a subclass), so the copy is
//     what Java's dispatch runs on a subclass object;
//   - a method that implements an interface method with a wider result (StaticScope's
//     `StaticSlot getOwnSlot(String)`, Java `V getOwnSlot(String)`) is split as javac splits
//     it (covret): <name>T_ returns σ(V) and <name> is the bridge, boxing it with iup_ (nil
//     stays nil);
//   - the base keeps the methods that code holding the erased base (AbstractScope<?, ?>:
//     NodeTraversal, the scope creators) calls; a kept dispatcher reaches the subclass's
//     typed method through a bridge <name>_impl returning the erased type (javac's bridge),
//     a kept method that does not read a moved field keeps its body, the rest are deleted.
//
// Callers on a subclass get the typed results: the casts of a result to its σ type are the
// call (cast_[*Var](s.getVar(n).self()) -> s.getVar(n); cast_[*Scope](v.getScope()) ->
// v.getScopeT_(); s.getParent().self().(*Scope).f -> s.getParent().f, the assertion's nil
// panicking as the dereference does), an upcast x.up_B() given to a slot now typed σ(B) is x,
// and a typed result that meets a slot of the erased type (a variable of AbstractScope<?, ?>,
// an Object) is upcast with up_B, the value the erased code had. Every cast removed was
// javac's checkcast of S or V: the values a Scope's table holds are the Vars its declare
// methods put, a Var's scope is the Scope its constructor got.
//
// A cast of a typed result to an interface it implements is iup_I of the typed result (nil
// stays nil, castfold -nilup's form); j2go's cast to the erased base of a value given to a slot
// of AbstractScope<?, ?> is the typed value's up_B. castfold -assert's x.m().(T).f (Java's
// NullPointerException at the dereference of a null cast) loses the assertion only when f is a
// field or a method whose body reads a field of its receiver before it calls anything else:
// then the nil dereference is where Java throws. A method that first hands its receiver on
// (getClosestHoistScope starts with thisScope(), which returns it, and returns nil for nil) keeps
// the assertion, counted. Casts whose operand reaches the typed value only through other code
// are left to castfold. A rerun finds every method written out and changes nothing.
//
// usage: fbound [-n] [-v] <dir of package closure>
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

type base struct {
	tn          *types.TypeName
	named       *types.Named
	jc          *javaClass
	file        *ast.File
	spec        *ast.TypeSpec
	subs        []*sub
	decls       map[string]*ast.FuncDecl
	order       []*ast.FuncDecl
	moved       []*types.Var
	movedField  map[*types.Var]*ast.Field
	iface       *ast.TypeSpec // the dispatch interface of self_ (nil if self_ is a superclass's)
	ifaceName   string
	ifaceFields map[string]*ast.Field
	newFn       *ast.FuncDecl
}

type sub struct {
	tn    *types.TypeName
	named *types.Named
	b     *base
	sigma map[*types.TypeName]*types.TypeName
	file  *ast.File
	spec  *ast.TypeSpec
	embed *ast.Field
	decls map[string]*ast.FuncDecl
}

type group struct {
	b         *base
	name      string        // the method's Go name (the dispatcher's or the plain method's)
	d         *ast.FuncDecl // the dispatcher B.name (calls self_.implName), or nil
	impl      *ast.FuncDecl // the base's body: the dispatcher's target, or the plain method
	implName  string
	tname     string     // the typed method's name on each subclass
	ires      types.Type // T_: the interface result of the bridge
	jparam    string     // T_: the Java type parameter it returns
	kept      bool
	toDisp    bool // kept, reads a moved field: becomes a dispatcher
	superUsed map[*sub]bool
}

var (
	bases     = map[*types.TypeName]*base{}
	subs      = map[*types.TypeName]*sub{}
	groups    []*group
	groupOf   = map[types.Object]*group{} // method objects of D, impl, and the subclasses' O and W
	copyDecl  = map[*ast.FuncDecl]*group{}
	ownerSub  = map[*ast.FuncDecl]*sub{} // a subclass's override / wrapper decl
	declOfN   = map[ast.Node]*ast.FuncDecl{}
	cur       *sub   // the subclass a copied decl is being rendered for
	curGroup  *group // the group of the copied decl being rendered
	curTyped  bool   // rendering the typed method (returns take its result)
	gdecls    = map[*ast.FuncDecl]bool{}
	counts    = map[string]map[string]bool{}
	leftCount = map[string]map[string]bool{}
	verbose   bool
)

func count(what string, n ast.Node) {
	if counts[what] == nil {
		counts[what] = map[string]bool{}
	}
	k := rw.Fset.Position(n.Pos()).String()
	if cur != nil {
		k += "@" + cur.tn.Name()
	}
	counts[what][k] = true
}

func left(why string, n ast.Node) {
	if leftCount[why] == nil {
		leftCount[why] = map[string]bool{}
	}
	leftCount[why][rw.Fset.Position(n.Pos()).String()] = true
}

func isG(f *ast.File) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_")
}

func declOf(n ast.Node) *ast.FuncDecl {
	if d, ok := declOfN[n]; ok {
		return d
	}
	var d *ast.FuncDecl
	for p := n; p != nil; p = rw.Parent[p] {
		if fd, ok := p.(*ast.FuncDecl); ok {
			d = fd
			break
		}
	}
	declOfN[n] = d
	return d
}

// inCopy reports whether n is rendered as part of a written-out copy (cur set).
func inCopy(n ast.Node) bool {
	return cur != nil && copyDecl[declOf(n)] != nil
}

// ---- types

func ptrNamed(t types.Type) *types.TypeName {
	if t == nil {
		return nil
	}
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok {
		return nil
	}
	return n.Obj()
}

func mentionsErased(t types.Type) bool {
	found := false
	walkType(t, func(n *types.Named) {
		if bases[n.Obj()] != nil {
			found = true
		}
	})
	return found
}

func walkType(t types.Type, f func(*types.Named)) {
	switch x := types.Unalias(t).(type) {
	case *types.Pointer:
		walkType(x.Elem(), f)
	case *types.Slice:
		walkType(x.Elem(), f)
	case *types.Array:
		walkType(x.Elem(), f)
	case *types.Map:
		walkType(x.Key(), f)
		walkType(x.Elem(), f)
	case *types.Named:
		f(x)
		for i := 0; i < x.TypeArgs().Len(); i++ {
			walkType(x.TypeArgs().At(i), f)
		}
	case *types.Signature:
		for i := 0; i < x.Params().Len(); i++ {
			walkType(x.Params().At(i).Type(), f)
		}
		for i := 0; i < x.Results().Len(); i++ {
			walkType(x.Results().At(i).Type(), f)
		}
	}
}

// subst applies σ of s to t.
func subst(t types.Type, s *sub) types.Type {
	if s == nil || t == nil {
		return t
	}
	switch x := types.Unalias(t).(type) {
	case *types.Pointer:
		e := subst(x.Elem(), s)
		if e == x.Elem() {
			return t
		}
		return types.NewPointer(e)
	case *types.Slice:
		e := subst(x.Elem(), s)
		if e == x.Elem() {
			return t
		}
		return types.NewSlice(e)
	case *types.Array:
		e := subst(x.Elem(), s)
		if e == x.Elem() {
			return t
		}
		return types.NewArray(e, x.Len())
	case *types.Map:
		k, e := subst(x.Key(), s), subst(x.Elem(), s)
		if k == x.Key() && e == x.Elem() {
			return t
		}
		return types.NewMap(k, e)
	case *types.Named:
		if to, ok := s.sigma[x.Obj()]; ok {
			return to.Type()
		}
		if x.TypeArgs().Len() == 0 {
			return t
		}
		var args []types.Type
		changed := false
		for i := 0; i < x.TypeArgs().Len(); i++ {
			a := subst(x.TypeArgs().At(i), s)
			changed = changed || a != x.TypeArgs().At(i)
			args = append(args, a)
		}
		if !changed {
			return t
		}
		it, err := types.Instantiate(nil, x.Origin(), args, false)
		if err != nil {
			panic(err)
		}
		return it
	}
	return t
}

func tstr(t types.Type) string { return types.TypeString(t, qual) }

// ---- new static types

var ntMemo = map[[2]any]types.Type{}

func old(e ast.Expr) types.Type {
	if tv, ok := rw.Info.Types[e]; ok {
		return tv.Type
	}
	if id, ok := e.(*ast.Ident); ok {
		if o := rw.Info.ObjectOf(id); o != nil {
			return o.Type()
		}
	}
	return nil
}

// nt is the static type of e once the step has run (in the copy being rendered, if any).
func nt(e ast.Expr) types.Type {
	k := [2]any{e, cur}
	if t, ok := ntMemo[k]; ok {
		return t
	}
	ntMemo[k] = old(e) // cycles
	t := nt1(e)
	ntMemo[k] = t
	return t
}

func nt1(e ast.Expr) types.Type {
	copied := inCopy(e)
	switch x := e.(type) {
	case *ast.ParenExpr:
		return nt(x.X)
	case *ast.Ident:
		o := rw.Info.ObjectOf(x)
		v, ok := o.(*types.Var)
		if !ok || v.IsField() {
			return old(e)
		}
		d := declOf(x)
		if copied && d != nil {
			if d.Recv != nil && len(d.Recv.List[0].Names) > 0 && rw.Info.Defs[d.Recv.List[0].Names[0]] == v {
				return types.NewPointer(cur.named)
			}
			if def := defExpr(v); def != nil {
				return def()
			}
			return subst(v.Type(), cur)
		}
		if def := defExpr(v); def != nil && d != nil && v.Parent() != nil && v.Pos() >= d.Pos() && v.Pos() < d.End() {
			return def()
		}
		return old(e)
	case *ast.SelectorExpr:
		s := rw.Info.Selections[x]
		if s == nil || s.Kind() != types.FieldVal {
			return old(e)
		}
		f := s.Obj().(*types.Var)
		if b := movedOwner(f); b != nil {
			if c := subs[ptrNamed(nt(x.X))]; c != nil && c.b == b {
				return subst(f.Type(), c)
			}
		}
		return old(e)
	case *ast.CallExpr:
		if g, c, _, _ := groupCall(x); g != nil && c != nil {
			return typedResult(g, c, x)
		}
		if t := foldedType(x); t != nil {
			return t
		}
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
			if s := rw.Info.Selections[sel]; s != nil && s.Kind() == types.MethodVal {
				// a method of a generic container instantiated with the erased type
				rt, nrt := old(sel.X), nt(sel.X)
				if !types.Identical(rt, nrt) {
					if c := substOf(rt, nrt); c != nil {
						return subst(old(e), c)
					}
				}
			}
		}
		if copied {
			if fl, ok := x.Fun.(*ast.FuncLit); ok {
				return subst(old(fl.Type.Results.List[0].Type), cur)
			}
			if ix, ok := x.Fun.(*ast.IndexExpr); ok && rw.Info.Types[ix.Index].IsType() {
				return subst(old(e), cur)
			}
			if id, ok := x.Fun.(*ast.Ident); ok && (id.Name == "new" || id.Name == "make") {
				return subst(old(e), cur)
			}
			if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
				return subst(old(e), cur)
			}
		}
		return old(e)
	case *ast.TypeAssertExpr:
		if t := foldedType(x); t != nil {
			return t
		}
		if x.Type != nil {
			t := rw.Info.Types[x.Type].Type
			if copied {
				return subst(t, cur)
			}
			return t
		}
		return old(e)
	case *ast.IndexExpr:
		if tv, ok := rw.Info.Types[x.Index]; ok && tv.IsType() {
			return old(e)
		}
		xt := nt(x.X)
		if !types.Identical(xt, old(x.X)) {
			switch u := types.Unalias(xt).Underlying().(type) {
			case *types.Pointer:
				if a, ok := u.Elem().Underlying().(*types.Array); ok {
					return a.Elem()
				}
			case *types.Array:
				return u.Elem()
			case *types.Slice:
				return u.Elem()
			case *types.Map:
				return u.Elem()
			}
		}
		return old(e)
	case *ast.StarExpr:
		xt := nt(x.X)
		if p, ok := types.Unalias(xt).(*types.Pointer); ok && !types.Identical(xt, old(x.X)) {
			return p.Elem()
		}
		return old(e)
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			xt := nt(x.X)
			if !types.Identical(xt, old(x.X)) {
				return types.NewPointer(xt)
			}
		}
		return old(e)
	case *ast.CompositeLit, *ast.FuncLit:
		if copied {
			return subst(old(e), cur)
		}
	}
	return old(e)
}

// substOf is the subclass whose σ makes old type a into type b, or nil.
func substOf(a, b types.Type) *sub {
	for _, c := range sortedSubs() {
		if types.Identical(subst(a, c), b) {
			return c
		}
	}
	return nil
}

func sortedSubs() []*sub {
	var out []*sub
	for _, b := range sortedBases() {
		out = append(out, b.subs...)
	}
	return out
}

func sortedBases() []*base {
	var out []*base
	for _, b := range bases {
		out = append(out, b)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].tn.Name() < out[j].tn.Name() })
	return out
}

// defExpr returns, for a local defined by := (or a comma-ok assertion), its defining
// expression's new type.
var defs = map[*types.Var]func() types.Type{}

func defExpr(v *types.Var) func() types.Type {
	return defs[v]
}

func indexDefs() {
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if rs, ok := n.(*ast.RangeStmt); ok && rs.Tok == token.DEFINE {
				for _, l := range []ast.Expr{rs.Key, rs.Value} {
					id, ok := l.(*ast.Ident)
					if !ok {
						continue
					}
					v, ok := rw.Info.Defs[id].(*types.Var)
					if !ok {
						continue
					}
					x := rs.X
					defs[v] = func() types.Type {
						if types.Identical(nt(x), old(x)) {
							return v.Type()
						}
						return subst(v.Type(), substOf(old(x), nt(x)))
					}
				}
				return true
			}
			as, ok := n.(*ast.AssignStmt)
			if !ok || as.Tok != token.DEFINE {
				return true
			}
			for i, l := range as.Lhs {
				id, ok := l.(*ast.Ident)
				if !ok {
					continue
				}
				v, ok := rw.Info.Defs[id].(*types.Var)
				if !ok {
					continue
				}
				if len(as.Rhs) == len(as.Lhs) {
					r := as.Rhs[i]
					defs[v] = func() types.Type { return nt(r) }
				} else if len(as.Rhs) == 1 && i == 0 {
					if ta, ok := rw.Unparen(as.Rhs[0]).(*ast.TypeAssertExpr); ok {
						defs[v] = func() types.Type { return nt(ta) }
					}
				}
			}
			return true
		})
	}
}

func movedOwner(f *types.Var) *base {
	for _, b := range bases {
		for _, m := range b.moved {
			if m == f {
				return b
			}
		}
	}
	return nil
}

// ---- group calls

// groupCall classifies a call of a group's method: the group, the subclass the receiver
// now has (nil: an erased receiver), whether it is Java's super call (this.B.m), and the
// called object.
func groupCall(c *ast.CallExpr) (*group, *sub, bool, types.Object) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, nil, false, nil
	}
	s := rw.Info.Selections[sel]
	if s == nil || s.Kind() != types.MethodVal {
		return nil, nil, false, nil
	}
	g := groupOf[s.Obj()]
	if g == nil {
		return nil, nil, false, nil
	}
	// super call: this.B.m(...)
	if inner, ok := sel.X.(*ast.SelectorExpr); ok {
		if is := rw.Info.Selections[inner]; is != nil && is.Kind() == types.FieldVal && is.Obj().(*types.Var).Embedded() {
			if bases[ptrNamedOrNamed(is.Obj().Type())] == g.b {
				if c := subs[ptrNamed(nt(inner.X))]; c != nil && c.b == g.b {
					return g, c, true, s.Obj()
				}
			}
		}
	}
	c2 := subs[ptrNamed(nt(sel.X))]
	if c2 != nil && c2.b != g.b {
		c2 = nil
	}
	return g, c2, false, s.Obj()
}

func ptrNamedOrNamed(t types.Type) *types.TypeName {
	if n, ok := types.Unalias(t).(*types.Named); ok {
		return n.Obj()
	}
	return ptrNamed(t)
}

// typedResult is the result type of a group call on a receiver of subclass c, as rendered
// (a T_ group's call is typed only where the step calls the typed method).
func typedResult(g *group, c *sub, call *ast.CallExpr) types.Type {
	sig := old(call.Fun).(*types.Signature)
	if sig.Results().Len() != 1 {
		return nil
	}
	r := sig.Results().At(0).Type()
	if g.ires != nil {
		if callsTyped(g, call) {
			return paramType(g, c)
		}
		return r
	}
	return subst(r, c)
}

// paramType is σ of the Java type parameter a T_ group returns.
func paramType(g *group, c *sub) types.Type {
	er := bases[erasedOf(g.b, g.jparam)]
	if er == nil {
		return nil
	}
	return types.NewPointer(c.sigma[er.tn].Type())
}

func erasedOf(b *base, param string) *types.TypeName {
	cls := b.jc.bounds[param]
	for _, bb := range bases {
		if bb.tn.Name() == cls {
			return bb.tn
		}
	}
	return nil
}

// callsTyped reports whether a call of a T_ group's method is rendered as the typed method:
// a call of the implementation name (inside written-out code) or a call whose cast is folded.
func callsTyped(g *group, call *ast.CallExpr) bool {
	sel := call.Fun.(*ast.SelectorExpr)
	if sel.Sel.Name != g.name {
		return true
	}
	return foldParent(call) != nil
}

// ---- casts around group calls

// castOf returns the target type of a cast call K(x) and its operand: cast_[T], to_X, icast_I.
// In written-out code a target spelled with the erased base is σ of it.
func castOf(c *ast.CallExpr) (types.Type, ast.Expr) {
	if len(c.Args) != 1 {
		return nil, nil
	}
	name, _ := rw.CallName(c)
	if !(strings.HasPrefix(name, "cast_[") || strings.HasPrefix(name, "to_") || strings.HasPrefix(name, "icast_")) {
		return nil, nil
	}
	sig, ok := old(c.Fun).(*types.Signature)
	if !ok || sig.Results().Len() != 1 {
		return nil, nil
	}
	t := sig.Results().At(0).Type()
	if inCopy(c) {
		t = subst(t, cur)
	}
	return t, c.Args[0]
}

// selfOf: x.self() -> x
func selfOf(e ast.Expr) ast.Expr {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return nil
	}
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "self" {
		return nil
	}
	return sel.X
}

// foldParent returns the cast around a group call that the typed call replaces, or nil.
func foldParent(call *ast.CallExpr) ast.Node {
	g, c, _, _ := groupCall(call)
	if g == nil || c == nil {
		return nil
	}
	var rt types.Type // the typed result
	sig := old(call.Fun).(*types.Signature)
	if sig.Results().Len() != 1 {
		return nil
	}
	if g.ires != nil {
		rt = paramType(g, c)
	} else {
		rt = subst(sig.Results().At(0).Type(), c)
		if types.Identical(rt, sig.Results().At(0).Type()) {
			return nil
		}
	}
	p := rw.Up(call)
	// x.self() under the cast
	viaSelf := false
	if sel, ok := p.(*ast.SelectorExpr); ok && sel.Sel.Name == "self" && g.ires == nil {
		if sc, ok := rw.Up(sel).(*ast.CallExpr); ok && sc.Fun == sel && len(sc.Args) == 0 {
			viaSelf = true
			p = rw.Up(sc)
		}
	}
	if q, ok := p.(*ast.TypeAssertExpr); ok && (viaSelf || g.ires != nil) {
		// x.self().(T).f, x.m().(T).f: the assertion's nil panic is the dereference's
		if q.Type != nil && types.Identical(assertType(q), rt) {
			if ok, why := derefAtOnce(q, rt); ok {
				return q
			} else {
				left("an assertion of a typed result: "+why, q)
			}
		}
		return nil
	}
	kc, ok := p.(*ast.CallExpr)
	if !ok {
		return nil
	}
	t, _ := castOf(kc)
	if t == nil {
		return nil
	}
	if g.ires == nil && !viaSelf {
		return nil // a cast of the erased pointer itself (does not occur)
	}
	if types.Identical(t, rt) {
		return kc
	}
	if bases[ptrNamed(t)] != nil {
		// j2go's cast to the erased type: of a slot the step typed (the typed value), or of a
		// slot that keeps the erased type, AbstractScope<?, ?> (the typed value's up_B)
		if s := subs[ptrNamed(rt)]; s != nil && s.b.tn == ptrNamed(t) {
			return kc
		}
		return nil
	}
	if types.IsInterface(t) && types.Implements(rt, t.Underlying().(*types.Interface)) && rw.HasFunc("iup_"+ptrOrName(t)) {
		if inCopy(kc) {
			if et := expected(kc); et != nil && types.Identical(et, rt) {
				return nil // a typed return: the value itself (renderConv)
			}
		}
		return kc // an upcast of the typed result: iup_I
	}
	return nil
}

// foldedType is the type of a cast or assertion the step folds into a group call's typed
// result, or nil.
func foldedType(k ast.Expr) types.Type {
	var x ast.Expr
	switch kk := k.(type) {
	case *ast.CallExpr:
		if len(kk.Args) != 1 {
			return nil
		}
		x = kk.Args[0]
	case *ast.TypeAssertExpr:
		x = kk.X
	default:
		return nil
	}
	if y := selfOf(x); y != nil {
		x = y
	}
	c, ok := rw.Unparen(x).(*ast.CallExpr)
	if !ok {
		return nil
	}
	g, s, _, _ := groupCall(c)
	if g == nil || s == nil || foldParent(c) != k {
		return nil
	}
	rt := paramType(g, s)
	if g.ires == nil {
		rt = subst(old(c.Fun).(*types.Signature).Results().At(0).Type(), s)
	}
	if kc, ok := k.(*ast.CallExpr); ok {
		if t, _ := castOf(kc); t != nil {
			if types.IsInterface(t) {
				return t
			}
			if bases[ptrNamed(t)] != nil {
				if et := expected(kc); et != nil && types.Identical(et, rt) {
					return rt
				}
				return t // up_B of the typed value
			}
		}
	}
	return rt
}

// derefAtOnce: the assertion q (castfold -assert's form: its value is dereferenced at once,
// the arguments computing nothing) can be dropped when what follows dereferences the typed
// pointer before anything else: a field, or a method whose body does (Java's
// NullPointerException at the call is then the nil dereference's).
func derefAtOnce(q *ast.TypeAssertExpr, rt types.Type) (bool, string) {
	s, ok := rw.Up(q).(*ast.SelectorExpr)
	if !ok || rw.Unparen(s.X) != q {
		return false, "not dereferenced at once"
	}
	sel := rw.Info.Selections[s]
	if sel == nil {
		return false, "not dereferenced at once"
	}
	if sel.Kind() == types.FieldVal {
		return true, ""
	}
	if derefsFirst(subs[ptrNamed(rt)], s.Sel.Name, map[string]bool{}) {
		return true, ""
	}
	return false, "the method called on it does not dereference it first (" + s.Sel.Name + ")"
}

// derefsFirst reports whether the method name of subclass s (as the step writes it) reads a
// field of its receiver, or calls a method that does, before it calls anything else.
func derefsFirst(s *sub, name string, seen map[string]bool) bool {
	if s == nil || seen[name] {
		return false
	}
	seen[name] = true
	fd := methodSrc(s, name)
	if fd == nil || fd.Body == nil || len(fd.Body.List) == 0 {
		return false
	}
	recv := rw.Info.Defs[fd.Recv.List[0].Names[0]]
	res, decided := false, false
	ast.Inspect(fd.Body.List[0], func(n ast.Node) bool {
		if decided {
			return false
		}
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, ok := x.X.(*ast.Ident); ok && rw.Info.Uses[id] == recv {
				if sl := rw.Info.Selections[x]; sl != nil && sl.Kind() == types.FieldVal {
					res, decided = true, true
					return false
				}
				res, decided = derefsFirst(s, x.Sel.Name, seen), true
				return false
			}
		case *ast.CallExpr:
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if id, ok := sel.X.(*ast.Ident); ok && rw.Info.Uses[id] == recv {
					return true
				}
			}
			if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
				return true
			}
			// a call of something else first: arguments are evaluated before it
			return true
		case *ast.FuncLit:
			decided = true
			return false
		}
		return true
	})
	return res
}

// methodSrc is the declaration whose body the subclass runs for method name once the step
// has run: the group's source (the override or the base's body), the subclass's own, or the
// base's (promoted).
func methodSrc(s *sub, name string) *ast.FuncDecl {
	for _, g := range groups {
		if g.b != s.b || (g.tname != name && g.name != name) {
			continue
		}
		if o := s.decls[g.implName]; o != nil && copyDecl[o] == g {
			return o
		}
		if g.impl != nil {
			return g.impl
		}
	}
	if d := s.decls[name]; d != nil {
		if k := isWrapper(d); k != "" {
			return methodSrc(s, k)
		}
		return d
	}
	if d := s.b.decls[name]; d != nil {
		if k := dispatchTarget(s.b, d); k != "" {
			return methodSrc(s, k)
		}
		return d
	}
	return nil
}

func assertType(q *ast.TypeAssertExpr) types.Type {
	t := old(q.Type)
	if inCopy(q) {
		t = subst(t, cur)
	}
	return t
}

// ---- expected types

// expected is the type the context of e takes once the step has run; nil when anything goes
// (a receiver, a := definition, a statement).
func expected(e ast.Expr) types.Type {
	p := rw.Up(e)
	var inner ast.Node = e
	for {
		pe, ok := rw.Parent[inner].(*ast.ParenExpr)
		if !ok {
			break
		}
		inner = pe
	}
	switch x := p.(type) {
	case *ast.CallExpr:
		if x.Fun == inner {
			return nil
		}
		i := -1
		for k, a := range x.Args {
			if a == inner {
				i = k
			}
		}
		if i < 0 {
			return nil
		}
		sig, ok := old(x.Fun).(*types.Signature)
		if !ok {
			return old(e) // a conversion
		}
		pt := paramAt(sig, i)
		if g, c, _, _ := groupCall(x); g != nil && c != nil {
			return subst(pt, c)
		}
		if inCopy(x) {
			if sel, ok := x.Fun.(*ast.SelectorExpr); ok {
				if !types.Identical(nt(sel.X), old(sel.X)) {
					if c := substOf(old(sel.X), nt(sel.X)); c != nil {
						return subst(pt, c)
					}
				}
			}
		}
		return pt
	case *ast.AssignStmt:
		if x.Tok == token.DEFINE {
			return nil
		}
		for k, r := range x.Rhs {
			if r == inner && k < len(x.Lhs) {
				return nt(x.Lhs[k])
			}
		}
		return nil
	case *ast.ValueSpec:
		if x.Type == nil {
			return nil
		}
		t := old(x.Type)
		if inCopy(x) {
			t = subst(t, cur)
		}
		return t
	case *ast.ReturnStmt:
		return resultOf(x, inner)
	case *ast.BinaryExpr:
		if x.Op != token.EQL && x.Op != token.NEQ {
			return nil
		}
		o := x.X
		if o == inner {
			o = x.Y
		}
		if id, ok := rw.Unparen(o).(*ast.Ident); ok && id.Name == "nil" {
			return nil
		}
		return nt(o)
	case *ast.SelectorExpr, *ast.ExprStmt, *ast.TypeAssertExpr, *ast.StarExpr, *ast.UnaryExpr, *ast.IfStmt, *ast.ForStmt:
		return nil
	}
	return old(e)
}

func paramAt(sig *types.Signature, i int) types.Type {
	n := sig.Params().Len()
	if sig.Variadic() && i >= n-1 {
		return sig.Params().At(n - 1).Type().(*types.Slice).Elem()
	}
	return sig.Params().At(i).Type()
}

// resultOf is the result type a return statement gives its value to.
func resultOf(r *ast.ReturnStmt, inner ast.Node) types.Type {
	k := 0
	for i, x := range r.Results {
		if x == inner {
			k = i
		}
	}
	var fn ast.Node
	for p := rw.Parent[r]; p != nil; p = rw.Parent[p] {
		if _, ok := p.(*ast.FuncLit); ok {
			fn = p
			break
		}
		if _, ok := p.(*ast.FuncDecl); ok {
			fn = p
			break
		}
	}
	switch f := fn.(type) {
	case *ast.FuncLit:
		// an immediately called literal in a typed return takes the typed result
		if call, ok := rw.Parent[f].(*ast.CallExpr); ok && call.Fun == f {
			if et := expected(call); et != nil && inCopy(f) {
				return et
			}
		}
		t := old(f.Type).(*types.Signature).Results().At(k).Type()
		if inCopy(f) {
			t = subst(t, cur)
		}
		return t
	case *ast.FuncDecl:
		if inCopy(f) && curTyped && curGroup != nil && curGroup.ires != nil {
			return paramType(curGroup, cur)
		}
		t := rw.Info.Defs[f.Name].Type().(*types.Signature).Results().At(k).Type()
		if inCopy(f) {
			t = subst(t, cur)
		}
		return t
	}
	return nil
}

// ---- main

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	dir := os.Args[len(os.Args)-1]
	rw.Load(dir)
	javaRoot := filepath.Join(dir, "..", "..", "java", "src")
	findFamily(javaRoot)
	if len(bases) == 0 {
		fmt.Fprintln(os.Stderr, "fbound: no F-bounded family found (done already)")
		return
	}
	indexDefs()
	buildGroups()
	computeKept()
	register()
	// render once to count (decisions are made while rendering)
	for _, f := range rw.Files {
		if isG(f) {
			rw.RenderChildren(f)
		}
	}
	printCounts()
	if report {
		return
	}
	fmt.Fprintf(os.Stderr, "fbound: files changed: %d\n", rw.SaveChecked())
}

func printCounts() {
	for _, b := range sortedBases() {
		var names []string
		for _, s := range b.subs {
			names = append(names, s.tn.Name())
		}
		var mv []string
		for _, m := range b.moved {
			mv = append(mv, m.Name())
		}
		fmt.Fprintf(os.Stderr, "fbound: %s -> %s; fields moved: %s\n", b.tn.Name(), strings.Join(names, ", "), strings.Join(mv, ", "))
	}
	nw, nt, nk, nd := 0, 0, 0, 0
	var keptNames, tNames []string
	for _, g := range groups {
		nw++
		if g.ires != nil {
			nt++
			tNames = append(tNames, g.b.tn.Name()+"."+g.name)
		}
		if g.kept {
			nk++
			s := g.b.tn.Name() + "." + g.name
			if g.toDisp {
				s += " (now dispatched)"
			}
			keptNames = append(keptNames, s)
		}
		if g.toDisp {
			nd++
		}
	}
	fmt.Fprintf(os.Stderr, "fbound: %d methods written out per subclass (%d split into <name>T_ and the bridge: %s); the base keeps %d for erased receivers: %s\n",
		nw, nt, strings.Join(tNames, ", "), nk, strings.Join(keptNames, ", "))
	var ks []string
	for k := range counts {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", len(counts[k]), k)
	}
	ks = nil
	for k := range leftCount {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", len(leftCount[k]), k)
		if verbose {
			var ps []string
			for p := range leftCount[k] {
				ps = append(ps, p)
			}
			sort.Strings(ps)
			for _, p := range ps {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
}
