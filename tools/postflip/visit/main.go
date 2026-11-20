// Command visit is a post-flip step: JSType.visit is a generic function whose body is a type
// switch on the type's class, and the visitors return their own type instead of Object.
//
//	func (this *JSType) visit(visitor Visitor) any {        func JSType_visit[T any](this *JSType, visitor Visitor[T]) T {
//		return this.visitUntyped(visitor)                       switch this := this.self_.(type) {
//	}                                                           case *AllType:
//	func (this *AllType) visitUntyped_impl(v Visitor) any { ->      return visitor.caseAllType()
//		return visitor.caseAllType()                            case *EnumType:
//	}                                                               return visitor.caseObjectType(this.up_ObjectType())
//	...                                                         ...
//	to_JSType(t.visit(replacer))                            JSType_visit(t, replacer)
//
// Java declares `public final <T> T visit(Visitor<T> visitor) { return (T) visitUntyped(visitor); }`
// and every class of the type hierarchy overrides the virtual, non-generic `Object
// visitUntyped(Visitor<?>)` with one line that calls the visitor's case for the class. The
// translation erases T: Visitor's cases return `any`, every visitor boxes what it returns
// (obj_, self(), any(bool(...))) and every caller unboxes it again (to_JSType, unbox_[bool],
// cast_[R]). Go has no generic methods, so the generic method is a generic function, and the
// virtual dispatch on the type's class is a type switch on the most derived object (self_), as
// the hand port writes it (jsgo: TemplateTypeReplacer.visitJSType switches on the class). Each
// case is the body of the override that Java's dispatch picks for that class (the class's own,
// else the nearest inherited one), with the receiver given as the declaring class (up_X) where
// the class inherits it; every class whose pointer implements JSType_I (the classes self_ can
// hold) has its case, so the switch picks exactly the method the dispatch picked. self_ is nil
// in no constructed object; the switch's fall-through throws Java's NullPointerException, as
// the dispatch through a nil self_ did.
//
// The visitor interfaces get the type parameter (Visitor[T], RelationshipVisitor[T]) and the
// visitors return their T:
//   - a visitor class's T is found from what it returns and what its callers unbox, over the
//     class and the classes it embeds or that embed it (one Java class hierarchy, one
//     `implements Visitor<X>`), and over every Visitor-typed variable, field, parameter and
//     result that a visitor reaches (union-find); pointers to classes give their nearest common
//     class. A generic base (Visitor.WithDefaultCase<T>) keeps its parameter, and a class that
//     instantiates it with `any` (Java's Boolean) gets the T found.
//   - the case methods (caseX of the interface, their caseX_impl and the self_ / vt_ interfaces
//     that list them) return T. A returned box of a value of T (obj_(x), x.self(), any(x),
//     boxT_(x)) is the value; of a subclass, its up_X(); nil stays nil where T is a pointer; a
//     conditional's func() any {...}() returns T. Boxing and unboxing are inverse (null is nil
//     both ways), so the value the caller reads is the one Java's visitor returned.
//   - a call of visit, of a case method or of a forwarder (a method whose every return is
//     visit of its Visitor parameter: ProxyObjectType.visitReferenceType) whose value the
//     caller unboxed to T is the value; read where an Object is expected, it is boxed the way
//     the visitor boxed it.
//   - icast_Visitor(obj_(x)) of a visitor x that is never nil (`this`, a new_X(...), a package
//     variable assigned once, in its class's static initializer, from new_X(...)) is x.
//
// Left, counted: icast_X(obj_(x)) of a visitor that may be nil (a nil pointer in a Visitor[T]
// is not a nil interface: the case method would run where Java threw a NullPointerException);
// those keep a generic icast_X[T]. A returned value of type any that is not a box (a helper's
// Object result) is unboxed with the cast for T and counted. Nothing is changed (exit 1) when
// a visitor's T cannot be found or two disagree. A rerun finds no visitUntyped and changes
// nothing.
//
// usage: visit [-n] [-v] <dir of package closure>
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

var (
	done    = map[string]int{}
	left    = map[string]int{}
	leftAt  = map[string][]string{}
	fails   []string
	verbose bool
)

func skip(why string, n ast.Node) {
	left[why]++
	leftAt[why] = append(leftAt[why], rw.Fset.Position(n.Pos()).String())
}

func fail(n ast.Node, f string, a ...any) {
	p := ""
	if n != nil {
		p = rw.Fset.Position(n.Pos()).String() + ": "
	}
	fails = append(fails, p+fmt.Sprintf(f, a...))
}

// front is one generic visit method: visit -> visitUntyped -> self_.visitUntyped_impl.
type front struct {
	fn       *types.Func // (*JSType).visit
	decl     *ast.FuncDecl
	disp     string // visitUntyped
	impl     string // visitUntyped_impl
	iface    *types.Named
	params   []string // the front's parameter names
	name     string   // JSType_visit
	implDecl *ast.FuncDecl
}

var (
	pkgScope *types.Scope
	declOf   = map[*types.Func]*ast.FuncDecl{}
	root     *types.Named // JSType
	selfI    *types.Named // JSType_I
	fronts   []*front
	frontOf  = map[*types.Func]*front{}
	ifaces   = map[*types.Named]*front{}
)

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	pkgScope = rw.Pkg.Scope()
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				if fn, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					declOf[fn] = fd
				}
			}
		}
	}
	root, _ = namedOf(pkgScope.Lookup("JSType"))
	selfI, _ = namedOf(pkgScope.Lookup("JSType_I"))
	if root == nil || selfI == nil {
		fmt.Fprintln(os.Stderr, "visit: no JSType / JSType_I: nothing to do")
		return
	}
	findFronts()
	if len(fronts) == 0 {
		fmt.Fprintln(os.Stderr, "visit: 0 visit methods with a visitUntyped dispatch (already converted)")
		return
	}
	analyze()
	if len(fails) > 0 {
		for _, f := range fails {
			fmt.Fprintln(os.Stderr, "visit: cannot convert:", f)
		}
		os.Exit(1)
	}
	rewrite()
	if len(fails) > 0 {
		for _, f := range fails {
			fmt.Fprintln(os.Stderr, "visit: cannot convert:", f)
		}
		os.Exit(1)
	}
	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
		if verbose {
			for _, p := range leftAt[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	keys = keys[:0]
	for k := range done {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", done[k], k)
	}
	if report {
		fmt.Fprintln(os.Stderr, "visit: dry run, nothing written")
		return
	}
	fmt.Fprintf(os.Stderr, "visit: files changed: %d\n", rw.SaveChecked())
}

// ---- types

func namedOf(o types.Object) (*types.Named, bool) {
	if o == nil {
		return nil, false
	}
	n, ok := types.Unalias(o.Type()).(*types.Named)
	return n, ok
}

// classOf is the package's named struct type N of a type *N (instantiations give their origin).
func classOf(t types.Type) *types.Named {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.Obj().Pkg() != rw.Pkg {
		return nil
	}
	if _, ok := n.Underlying().(*types.Struct); !ok {
		return nil
	}
	return n.Origin()
}

func isAny(t types.Type) bool {
	if t == nil {
		return false
	}
	i, ok := types.Unalias(t).(*types.Interface)
	return ok && i.Empty()
}

// super is the class N embeds first (Java's superclass), or nil.
func super(n *types.Named) *types.Named {
	st, ok := n.Underlying().(*types.Struct)
	if !ok {
		return nil
	}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Embedded() {
			continue
		}
		if e, ok := types.Unalias(f.Type()).(*types.Named); ok && e.Obj().Pkg() == rw.Pkg {
			if _, ok := e.Underlying().(*types.Struct); ok {
				return e
			}
		}
	}
	return nil
}

// chain is n and its superclasses, nearest first (origins).
func chain(n *types.Named) []*types.Named {
	var out []*types.Named
	for ; n != nil; n = super(n) {
		out = append(out, n.Origin())
	}
	return out
}

// join is the type that holds values of a and b: identical types, or the nearest common class of
// two class pointers.
func join(a, b types.Type) types.Type {
	if a == nil {
		return b
	}
	if b == nil || types.Identical(a, b) {
		return a
	}
	ca, cb := classOf(a), classOf(b)
	if ca == nil || cb == nil {
		return nil
	}
	for _, x := range chain(ca) {
		for _, y := range chain(cb) {
			if x == y {
				return types.NewPointer(x)
			}
		}
	}
	return nil
}

func spell(t types.Type) string { return types.TypeString(t, types.RelativeTo(rw.Pkg)) }

// ---- the fronts

func findFronts() {
	ms := types.NewMethodSet(types.NewPointer(root))
	for i := 0; i < ms.Len(); i++ {
		fn, ok := ms.At(i).Obj().(*types.Func)
		if !ok || len(ms.At(i).Index()) != 1 {
			continue
		}
		fd := declOf[fn]
		if fd == nil || len(fd.Body.List) != 1 {
			continue
		}
		ret, ok := fd.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(ret.Results) != 1 {
			continue
		}
		call, ok := ret.Results[0].(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || !isRecv(sel.X, fd) || !strings.HasPrefix(sel.Sel.Name, "visitUntyped") {
			continue
		}
		sig := fn.Type().(*types.Signature)
		if sig.Params().Len() == 0 || sig.Results().Len() != 1 || !isAny(sig.Results().At(0).Type()) {
			continue
		}
		vi, ok := types.Unalias(sig.Params().At(0).Type()).(*types.Named)
		if !ok {
			continue
		}
		it, ok := vi.Underlying().(*types.Interface)
		if !ok {
			continue
		}
		allAny := it.NumMethods() > 0
		for j := 0; j < it.NumMethods(); j++ {
			r := it.Method(j).Type().(*types.Signature).Results()
			allAny = allAny && r.Len() == 1 && isAny(r.At(0).Type())
		}
		if !allAny {
			continue
		}
		fr := &front{fn: fn, decl: fd, disp: sel.Sel.Name, impl: sel.Sel.Name + "_impl", iface: vi}
		for j := 0; j < sig.Params().Len(); j++ {
			fr.params = append(fr.params, sig.Params().At(j).Name())
		}
		fr.name = root.Obj().Name() + "_" + fn.Name()
		fronts = append(fronts, fr)
		frontOf[fn] = fr
		ifaces[vi] = fr
	}
	sort.Slice(fronts, func(i, j int) bool { return fronts[i].fn.Name() < fronts[j].fn.Name() })
}

func isRecv(e ast.Expr, fd *ast.FuncDecl) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 || len(fd.Recv.List[0].Names) != 1 {
		return false
	}
	return rw.Info.Uses[id] != nil && rw.Info.Uses[id] == rw.Info.Defs[fd.Recv.List[0].Names[0]]
}

// ---- analysis: visitor classes, their T, the Visitor-typed slots

// uf is union-find over visitor classes (*types.Named origins), slots (*types.Var, *ast.FuncType
// results, *ast.TypeAssertExpr) and expressions.
var ufParent = map[any]any{}

func find(x any) any {
	p, ok := ufParent[x]
	if !ok || p == x {
		ufParent[x] = x
		return x
	}
	r := find(p)
	ufParent[x] = r
	return r
}

func union(a, b any) {
	if ra, rb := find(a), find(b); ra != rb {
		ufParent[ra] = rb
	}
}

// evidence per union class.
type evid struct {
	t      types.Type
	nilOK  bool // a nil is returned
	fixed  types.Type
	where  ast.Node
	reason string
}

var (
	visitorClasses = map[*types.Named]*types.Named{} // visitor class -> the interface it implements
	genericBase    = map[*types.Named]bool{}         // Visitor_WithDefaultCase[T]
	instArg        = map[*types.Named]types.Type{}   // class embedding a generic base -> its type argument
	instField      = map[*types.Named]*types.Named{} // class -> the generic base it embeds
	caseNames      = map[string]bool{}               // the visitor interfaces' method names (and _impl)
	retyped        = map[*types.Func]*types.Named{}  // case methods returning T, by their class
	retypedIface   = map[*types.Func]*types.Named{}  // interface methods (self_ / vt_) returning T, by class
	forwarders     = map[*types.Func]*types.Var{}    // methods whose returns are visit(param)
	helpers        = map[*types.Func]*types.Named{}  // other methods of a visitor class returning T
	ev             = map[any]*evid{}
	slotTypeExprs  []slotExpr // the type expressions `Visitor` to give T
)

type slotExpr struct {
	id   *ast.Ident
	node any
}

func evOf(x any) *evid {
	r := find(x)
	e := ev[r]
	if e == nil {
		e = &evid{}
		ev[r] = e
	}
	return e
}

// tOf is the type found for the class of x (nil: none).
func tOf(x any) types.Type {
	e := ev[find(x)]
	if e == nil {
		return nil
	}
	if e.fixed != nil {
		return e.fixed
	}
	return e.t
}

func analyze() {
	// The visitor classes: named struct types of the package whose pointer has every method of
	// a visitor interface (by name: a generic class is checked on its origin).
	for _, fr := range fronts {
		it := fr.iface.Underlying().(*types.Interface)
		for j := 0; j < it.NumMethods(); j++ {
			caseNames[it.Method(j).Name()] = true
			caseNames[it.Method(j).Name()+"_impl"] = true
		}
	}
	for _, name := range pkgScope.Names() {
		n, ok := namedOf(pkgScope.Lookup(name))
		if !ok || n.Obj().Name() != name {
			continue
		}
		if _, ok := n.Underlying().(*types.Struct); !ok {
			continue
		}
		for _, fr := range fronts {
			it := fr.iface.Underlying().(*types.Interface)
			all := true
			for j := 0; j < it.NumMethods() && all; j++ {
				o, _, _ := types.LookupFieldOrMethod(types.NewPointer(n), true, rw.Pkg, it.Method(j).Name())
				f, ok := o.(*types.Func)
				all = ok && f.Type().(*types.Signature).Params().Len() == it.Method(j).Type().(*types.Signature).Params().Len()
			}
			if all {
				visitorClasses[n] = fr.iface
				if n.TypeParams().Len() > 0 {
					genericBase[n] = true
				}
			}
		}
	}
	// Classes embedding a visitor class: one hierarchy (one T), except a generic base, whose
	// instantiation gives the subclass's T.
	for n := range visitorClasses {
		if genericBase[n] {
			continue
		}
		if s := super(n); s != nil {
			if _, ok := visitorClasses[s.Origin()]; ok {
				if genericBase[s.Origin()] {
					instField[n] = s.Origin()
					instArg[n] = s.TypeArgs().At(0)
					if !isAny(instArg[n]) {
						evOf(n).fixed = instArg[n]
					}
				} else {
					union(n, s.Origin())
				}
			}
		}
	}
	// The case methods of each visitor class and of the interfaces its self_ / vt_ fields hold.
	for n := range visitorClasses {
		ptr := types.NewPointer(n)
		ms := types.NewMethodSet(ptr)
		for i := 0; i < ms.Len(); i++ {
			fn, ok := ms.At(i).Obj().(*types.Func)
			if !ok || len(ms.At(i).Index()) != 1 {
				continue // promoted: retyped on its own class
			}
			if !caseNames[fn.Name()] && !(instField[n] != nil && isBaseParamMethod(instField[n], fn.Name())) {
				continue
			}
			r := fn.Type().(*types.Signature).Results()
			if r.Len() != 1 {
				continue
			}
			if genericBase[n] {
				if isAny(r.At(0).Type()) {
					retyped[fn] = n
				}
				continue
			}
			if isAny(r.At(0).Type()) || (instField[n] != nil && isAny(instArg[n]) && isBaseParamMethod(instField[n], fn.Name())) {
				retyped[fn] = n
			}
		}
		st := n.Underlying().(*types.Struct)
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			if f.Name() != "self_" && !strings.HasPrefix(f.Name(), "vt_") {
				continue
			}
			in, ok := types.Unalias(f.Type()).(*types.Named)
			if !ok || in.Obj().Pkg() != rw.Pkg {
				continue
			}
			it, ok := in.Underlying().(*types.Interface)
			if !ok {
				continue
			}
			for j := 0; j < it.NumExplicitMethods(); j++ {
				m := it.ExplicitMethod(j)
				r := m.Type().(*types.Signature).Results()
				if caseNames[m.Name()] && r.Len() == 1 && isAny(r.At(0).Type()) {
					retypedIface[m.Origin()] = n
				}
			}
		}
	}
	// Forwarders: methods whose every return is visit(v) of their Visitor parameter v.
	for fn, fd := range declOf {
		sig := fn.Type().(*types.Signature)
		if frontOf[fn] != nil || isDeletedDecl(fd) || sig.Results().Len() != 1 || !isAny(sig.Results().At(0).Type()) || fd.Body == nil {
			continue
		}
		var param *types.Var
		ok := true
		n := 0
		inspectOwn(fd.Body, func(r *ast.ReturnStmt) {
			n++
			if len(r.Results) != 1 {
				ok = false
				return
			}
			fr, call := visitCall(r.Results[0])
			if fr == nil {
				ok = false
				return
			}
			id, isId := rw.Unparen(call.Args[0]).(*ast.Ident)
			if !isId {
				ok = false
				return
			}
			v, isVar := rw.Info.Uses[id].(*types.Var)
			if !isVar || (param != nil && param != v) {
				ok = false
				return
			}
			param = v
		})
		if ok && n > 0 && param != nil && isParamOf(param, sig) {
			forwarders[fn] = param
		}
	}
	// Evidence from the case methods' returns.
	for fn, n := range retyped {
		if genericBase[n] {
			continue
		}
		fd := declOf[fn]
		if fd == nil {
			continue
		}
		inspectOwn(fd.Body, func(r *ast.ReturnStmt) {
			for _, e := range r.Results {
				addEvidence(n, e)
			}
		})
	}
	// Slots: every type expression naming a visitor interface, and the flow between them.
	collectSlots()
	// Consumers: visit calls unboxed by their caller.
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			call, ok := x.(*ast.CallExpr)
			if !ok {
				return true
			}
			fr, _ := visitCall(call)
			if fr == nil {
				return true
			}
			if inDeleted(call) {
				return true
			}
			node := valueNode(call.Args[0])
			if node == nil {
				fail(call, "the visitor argument %s is not a visitor class, slot or assertion", rw.Text(call.Args[0]))
				return true
			}
			if t := consumerType(call); t != nil {
				e := evOf(node)
				if nt := join(e.t, t); nt != nil {
					e.t = nt
				} else {
					fail(call, "the caller unboxes to %s, the visitor returns %s", spell(t), spell(e.t))
				}
			}
			return true
		})
	}
	// Every class must have a T; check them.
	for n := range visitorClasses {
		if genericBase[n] {
			continue
		}
		e := evOf(n)
		t := tOf(n)
		if e.reason != "" {
			fail(nil, "%s: %s", n.Obj().Name(), e.reason)
			continue
		}
		if t == nil {
			fail(nil, "%s: no type found for its visitor's T", n.Obj().Name())
			continue
		}
		if e.fixed != nil && e.t != nil && !types.Identical(join(e.fixed, e.t), e.fixed) {
			fail(nil, "%s: T is %s but %s is returned", n.Obj().Name(), spell(e.fixed), spell(e.t))
		}
		if e.nilOK {
			switch types.Unalias(t).Underlying().(type) {
			case *types.Pointer, *types.Interface:
			default:
				fail(nil, "%s: nil is returned and T is %s", n.Obj().Name(), spell(t))
			}
		}
	}
	for _, s := range slotTypeExprs {
		if tOf(s.node) == nil {
			fail(s.id, "a %s slot that no visitor class reaches", s.id.Name)
		}
	}
	if len(fails) == 0 {
		findHelpers()
	}
}

// findHelpers: a method of a visitor class (not a case method, listed by no interface) whose
// result is any, whose every return converts to the class's T without a cast, and whose every
// call is returned by a case method or helper or unboxed to T by its caller, returns T
// (CanCastToVisitor.castCastToHelper: Java's `private Boolean`, erased with the visitor).
func findHelpers() {
	listed := map[string]bool{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			if it, ok := x.(*ast.InterfaceType); ok {
				for _, m := range it.Methods.List {
					for _, nm := range m.Names {
						listed[nm.Name] = true
					}
				}
			}
			return true
		})
	}
	cand := map[*types.Func]*types.Named{}
	for fn, fd := range declOf {
		sig := fn.Type().(*types.Signature)
		if sig.Recv() == nil || fd.Body == nil || sig.Results().Len() != 1 || !isAny(sig.Results().At(0).Type()) {
			continue
		}
		n := classOf(sig.Recv().Type())
		if n == nil || genericBase[n] || caseNames[fn.Name()] || listed[fn.Name()] || isDeletedDecl(fd) {
			continue
		}
		if _, ok := visitorClasses[n]; !ok {
			continue
		}
		if _, ok := forwarders[fn]; ok {
			continue
		}
		t := tOf(n)
		if t == nil {
			continue
		}
		ok := true
		inspectOwn(fd.Body, func(r *ast.ReturnStmt) {
			for _, e := range r.Results {
				ok = ok && convertible(e, t)
			}
		})
		if ok {
			cand[fn] = n
		}
	}
	// Every call: returned by a case method or a candidate, or unboxed to T. Drop candidates
	// until none fails.
	for changed := true; changed; {
		changed = false
		for fn, n := range cand {
			helpers[fn] = n
		}
		for fn, n := range cand {
			t := tOf(n)
			good := true
			for _, u := range rw.UsesOf[fn] {
				sel, isSel := rw.Parent[u].(*ast.SelectorExpr)
				var call *ast.CallExpr
				if isSel {
					call, _ = rw.Parent[sel].(*ast.CallExpr)
				}
				if call == nil || call.Fun != ast.Expr(sel) {
					good = false
					break
				}
				if inReturnOfRetyped(call) {
					continue
				}
				if pc, ok := rw.Up(call).(*ast.CallExpr); ok && len(pc.Args) == 1 && types.Identical(typeOf(pc), t) {
					if name, _ := rw.CallName(pc); strings.HasPrefix(name, "unbox_[") || strings.HasPrefix(name, "cast_[") || strings.HasPrefix(name, "to_") {
						continue
					}
				}
				good = false
				break
			}
			if !good {
				delete(cand, fn)
				changed = true
			}
		}
		for fn := range helpers {
			delete(helpers, fn)
		}
	}
	for fn, n := range cand {
		helpers[fn] = n
	}
}

// convertible reports whether a returned e converts to t without a cast.
func convertible(e ast.Expr, t types.Type) bool {
	e = rw.Unparen(e)
	if lit := iife(e); lit != nil {
		ok := true
		inspectOwn(lit.Body, func(r *ast.ReturnStmt) {
			for _, re := range r.Results {
				ok = ok && convertible(re, t)
			}
		})
		return ok
	}
	if id, ok := e.(*ast.Ident); ok && id.Name == "nil" {
		switch types.Unalias(t).Underlying().(type) {
		case *types.Pointer, *types.Interface:
			return true
		}
		return false
	}
	if isCaseOrVisitCall(e) {
		return true
	}
	if b := boxed(e); b != nil {
		if isCaseOrVisitCall(b) {
			return true
		}
		b = rw.Unparen(b)
		bt := typeOf(b)
		if c, ok := b.(*ast.CallExpr); ok && len(c.Args) == 1 {
			if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() && types.Identical(tv.Type, t) {
				return true
			}
		}
		return bt != nil && (types.Identical(types.Default(bt), t) || upcast(b, bt, t) != nil)
	}
	return false
}

// isBaseParamMethod reports whether the generic base's vt_ interface lists name with the type
// parameter as its result (caseDefault_impl).
func isBaseParamMethod(base *types.Named, name string) bool {
	st := base.Underlying().(*types.Struct)
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !strings.HasPrefix(f.Name(), "vt_") {
			continue
		}
		in, ok := types.Unalias(f.Type()).(*types.Named)
		if !ok {
			continue
		}
		it := in.Underlying().(*types.Interface)
		for j := 0; j < it.NumMethods(); j++ {
			m := it.Method(j)
			r := m.Type().(*types.Signature).Results()
			if m.Name() == name && r.Len() == 1 {
				if _, ok := r.At(0).Type().(*types.TypeParam); ok {
					return true
				}
			}
		}
	}
	return false
}

func isParamOf(v *types.Var, sig *types.Signature) bool {
	for i := 0; i < sig.Params().Len(); i++ {
		if sig.Params().At(i) == v {
			_, ok := ifaces[namedNoErr(v.Type())]
			return ok
		}
	}
	return false
}

func namedNoErr(t types.Type) *types.Named {
	n, _ := types.Unalias(t).(*types.Named)
	if n == nil {
		return nil
	}
	return n.Origin()
}

// inspectOwn calls f on the return statements of body that belong to it (not to nested func
// literals, except a conditional's func() {...}() called in place, whose returns are the body's).
func inspectOwn(body *ast.BlockStmt, f func(*ast.ReturnStmt)) {
	ast.Inspect(body, func(x ast.Node) bool {
		switch n := x.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			f(n)
		}
		return true
	})
}

// iife is the func literal of a conditional's `func() T {...}()`, or nil.
func iife(e ast.Expr) *ast.FuncLit {
	call, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil
	}
	lit, ok := call.Fun.(*ast.FuncLit)
	if !ok || lit.Type.Params.NumFields() != 0 || lit.Type.Results == nil || lit.Type.Results.NumFields() != 1 {
		return nil
	}
	return lit
}

// boxed is x of a box call obj_(x), strobj_(x), any(x), boxT_(x), x.self(), x.box_(), or nil.
func boxed(e ast.Expr) ast.Expr {
	call, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil
	}
	if name, _ := rw.CallName(call); len(call.Args) == 1 && (name == "obj_" || name == "strobj_" || name == "any" || name == "boxT_" || strings.HasPrefix(name, "boxT_[") || strings.HasPrefix(name, "obj_[")) {
		return call.Args[0]
	}
	if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
		return s.X
	}
	return nil
}

// typeOfBoxed is the type of the value a box holds: a class pointer for self() of a class.
func typeOf(e ast.Expr) types.Type { return rw.Info.Types[e].Type }

func addEvidence(n *types.Named, e ast.Expr) {
	x := evOf(n)
	add := func(t types.Type) {
		if nt := join(x.t, t); nt != nil {
			x.t = nt
		} else if x.reason == "" {
			x.reason = fmt.Sprintf("returns %s and %s (%s)", spell(x.t), spell(t), rw.Fset.Position(e.Pos()))
		}
	}
	e = rw.Unparen(e)
	if id, ok := e.(*ast.Ident); ok && id.Name == "nil" {
		x.nilOK = true
		return
	}
	if lit := iife(e); lit != nil {
		inspectOwn(lit.Body, func(r *ast.ReturnStmt) {
			for _, re := range r.Results {
				addEvidence(n, re)
			}
		})
		return
	}
	if b := boxed(e); b != nil {
		b = rw.Unparen(b)
		if c, ok := b.(*ast.CallExpr); ok && len(c.Args) == 1 {
			if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() {
				if bt, ok := types.Unalias(tv.Type).(*types.Basic); ok {
					add(bt)
					return
				}
			}
		}
		t := typeOf(b)
		if t != nil && !isAny(t) {
			if _, isTP := t.(*types.TypeParam); !isTP {
				add(types.Default(t))
			}
		}
		return
	}
	if isCaseOrVisitCall(e) {
		return
	}
	if t := typeOf(e); t != nil && !isAny(t) {
		add(t)
	}
}

// isCaseOrVisitCall reports whether e calls visit, a forwarder, or a retyped case method (whose
// result will be T).
func isCaseOrVisitCall(e ast.Expr) bool {
	call, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	if fr, _ := visitCall(call); fr != nil {
		return true
	}
	fn := calledFunc(call)
	if fn == nil {
		return false
	}
	_, a := retyped[fn]
	_, b := retypedIface[fn]
	_, c := forwarders[fn]
	_, d := helpers[fn]
	return a || b || c || d || isVisitorIfaceMethod(fn)
}

func isVisitorIfaceMethod(fn *types.Func) bool {
	sig := fn.Type().(*types.Signature)
	if sig.Recv() == nil {
		return false
	}
	_, ok := ifaces[namedNoErr(sig.Recv().Type())]
	return ok
}

func calledFunc(call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch f := rw.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	default:
		return nil
	}
	fn, _ := rw.Info.Uses[id].(*types.Func)
	if fn == nil {
		return nil
	}
	return fn.Origin()
}

// visitCall: e is X.visit(v, ...) of a front: the front and the call.
func visitCall(e ast.Expr) (*front, *ast.CallExpr) {
	call, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil, nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	fn, _ := rw.Info.Uses[sel.Sel].(*types.Func)
	if fn == nil {
		return nil, nil
	}
	if fr := frontOf[fn]; fr != nil {
		return fr, call
	}
	return nil, nil
}

// consumerType is the type the caller of a visit call unboxes it to, or nil.
func consumerType(call *ast.CallExpr) types.Type {
	if p, ok := rw.Up(call).(*ast.CallExpr); ok && len(p.Args) == 1 && rw.Unparen(p.Args[0]) == call {
		name, _ := rw.CallName(p)
		if strings.HasPrefix(name, "to_") || strings.HasPrefix(name, "cast_[") || strings.HasPrefix(name, "unbox_[") {
			return typeOf(p)
		}
	}
	if ta, ok := rw.Up(call).(*ast.TypeAssertExpr); ok {
		if s, ok := rw.Up(ta).(*ast.SelectorExpr); ok && strings.HasPrefix(s.Sel.Name, "as_") {
			if c, ok := rw.Up(s).(*ast.CallExpr); ok && len(c.Args) == 0 {
				return typeOf(c)
			}
		}
	}
	return nil
}

// inDeleted reports whether n is inside a declaration the step deletes or replaces.
func inDeleted(n ast.Node) bool {
	for p := ast.Node(n); p != nil; p = rw.Parent[p] {
		if fd, ok := p.(*ast.FuncDecl); ok {
			return isDeletedDecl(fd)
		}
	}
	return false
}

func isDeletedDecl(fd *ast.FuncDecl) bool {
	if fd.Recv == nil {
		return false
	}
	for _, fr := range fronts {
		if fd.Name.Name == fr.disp || fd.Name.Name == fr.impl {
			return true
		}
		if fn, ok := rw.Info.Defs[fd.Name].(*types.Func); ok && fn == fr.fn {
			return true
		}
	}
	return false
}

// valueNode is the union-find node a visitor value belongs to: its class, its slot, or nil.
func valueNode(e ast.Expr) any {
	e = rw.Unparen(e)
	if n := classOf(typeOf(e)); n != nil {
		if _, ok := visitorClasses[n]; ok {
			return n
		}
		// A class embedding a visitor class (an anonymous subclass) is one hierarchy with it.
		for _, c := range chain(n) {
			if _, ok := visitorClasses[c]; ok {
				return c
			}
		}
		return nil
	}
	if name, call := rw.CallName(e); call != nil && strings.HasPrefix(name, "icast_") && len(call.Args) == 1 {
		if b := boxed(call.Args[0]); b != nil {
			return valueNode(b)
		}
	}
	switch x := e.(type) {
	case *ast.Ident:
		if v, ok := rw.Info.Uses[x].(*types.Var); ok {
			return v
		}
	case *ast.SelectorExpr:
		if v, ok := rw.Info.Uses[x.Sel].(*types.Var); ok {
			return v.Origin()
		}
	case *ast.TypeAssertExpr:
		return x
	case *ast.CallExpr:
		if lit := iife(x); lit != nil {
			return lit.Type
		}
		if fn := calledFunc(x); fn != nil {
			if fd := declOf[fn]; fd != nil {
				return fd.Type
			}
		}
	}
	return nil
}

func isVisitorType(t types.Type) bool {
	n := namedNoErr(t)
	_, ok := ifaces[n]
	return ok && n != nil
}

// collectSlots finds every type expression naming a visitor interface outside the interface's
// own declaration and the declarations the step deletes, the slot it types, and the flow of
// visitor values between slots and classes.
func collectSlots() {
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			switch n := x.(type) {
			case *ast.FuncDecl:
				if isDeletedDecl(n) || isIcastDecl(n) {
					return false
				}
			case *ast.GenDecl:
				for _, s := range n.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok {
						if tn, ok := rw.Info.Defs[ts.Name].(*types.TypeName); ok {
							if _, ok := ifaces[namedNoErr(tn.Type())]; ok {
								return false
							}
							if tn == selfI.Obj() {
								return false
							}
						}
					}
				}
			case *ast.Ident:
				tn, ok := rw.Info.Uses[n].(*types.TypeName)
				if !ok || !isVisitorType(tn.Type()) {
					return true
				}
				node := slotOfTypeExpr(n)
				if node == nil {
					fail(n, "a %s type expression whose slot is not found", n.Name)
					return true
				}
				slotTypeExprs = append(slotTypeExprs, slotExpr{n, node})
			case *ast.AssignStmt:
				if len(n.Lhs) == len(n.Rhs) {
					for i := range n.Lhs {
						flow(n.Lhs[i], n.Rhs[i])
					}
				}
			case *ast.ValueSpec:
				if len(n.Names) == len(n.Values) {
					for i := range n.Names {
						flow(n.Names[i], n.Values[i])
					}
				}
			case *ast.CallExpr:
				fn := calledFunc(n)
				if fn == nil {
					return true
				}
				sig, ok := fn.Type().(*types.Signature)
				if !ok || sig.Variadic() || sig.Params().Len() != len(n.Args) {
					return true
				}
				for i, a := range n.Args {
					p := sig.Params().At(i)
					if isVisitorType(p.Type()) {
						if fr := frontOf[fn]; fr != nil {
							continue // the front's parameter: per call (valueNode at the call)
						}
						if vn := valueNode(a); vn != nil {
							union(p.Origin(), vn)
						} else {
							fail(a, "a visitor argument %s that is not a visitor class or slot", rw.Text(a))
						}
					}
				}
			case *ast.ReturnStmt:
				ft := enclosingFuncType(n)
				if ft == nil || ft.Results == nil || len(n.Results) != 1 || ft.Results.NumFields() != 1 {
					return true
				}
				if t := typeOf(ft.Results.List[0].Type); t != nil && isVisitorType(t) {
					if vn := valueNode(n.Results[0]); vn != nil {
						union(ft, vn)
					} else {
						fail(n, "a returned visitor %s that is not a visitor class or slot", rw.Text(n.Results[0]))
					}
				}
			case *ast.TypeAssertExpr:
				if n.Type != nil && isVisitorType(typeOf(n.Type)) {
					// this.self().(Visitor) in a method of a visitor class: that class.
					if b := boxed(n.X); b != nil {
						if c := classOf(typeOf(b)); c != nil {
							if vn := valueNode(b); vn != nil {
								union(n, vn)
								return true
							}
						}
					}
					fail(n, "an assertion to a visitor of %s", rw.Text(n.X))
				}
			}
			return true
		})
	}
	// Forwarders: their parameter's class is their result's.
	for fn, p := range forwarders {
		union(declOf[fn].Type.Results, p) // the result node (the FuncType's results list)
		_ = fn
	}
}

func flow(l, r ast.Expr) {
	lt := typeOf(l)
	if lt == nil {
		if id, ok := l.(*ast.Ident); ok {
			if o := rw.Info.Defs[id]; o != nil {
				lt = o.Type()
			}
		}
	}
	if lt == nil || !isVisitorType(lt) {
		return
	}
	ln := slotNode(l)
	rn := valueNode(r)
	if ln == nil || rn == nil {
		fail(r, "a visitor stored in %s from %s: not a visitor class or slot", rw.Text(l), rw.Text(r))
		return
	}
	union(ln, rn)
}

func slotNode(e ast.Expr) any {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		if v, ok := rw.Info.Defs[x].(*types.Var); ok {
			return v
		}
		if v, ok := rw.Info.Uses[x].(*types.Var); ok {
			return v
		}
	case *ast.SelectorExpr:
		if v, ok := rw.Info.Uses[x.Sel].(*types.Var); ok {
			return v.Origin()
		}
	}
	return nil
}

func enclosingFuncType(n ast.Node) *ast.FuncType {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch f := p.(type) {
		case *ast.FuncLit:
			return f.Type
		case *ast.FuncDecl:
			return f.Type
		}
	}
	return nil
}

// slotOfTypeExpr: the slot a type expression `Visitor` declares.
func slotOfTypeExpr(id *ast.Ident) any {
	p := rw.Parent[id]
	switch x := p.(type) {
	case *ast.Field:
		// a struct field, a parameter, a result
		if len(x.Names) > 0 {
			if v, ok := rw.Info.Defs[x.Names[0]].(*types.Var); ok {
				return v
			}
		}
		fl, _ := rw.Parent[x].(*ast.FieldList)
		if ft, ok := rw.Parent[fl].(*ast.FuncType); ok && ft.Results == fl {
			return ft
		}
	case *ast.ValueSpec:
		if len(x.Names) == 1 {
			return rw.Info.Defs[x.Names[0]].(*types.Var)
		}
	case *ast.TypeAssertExpr:
		return x
	}
	return nil
}

// ---- rewriting

var curCase = map[*ast.FuncDecl]func(*ast.Ident) string{} // receiver substitution while a case body renders

func rewrite() {
	tSpell := func(n *types.Named) string {
		if genericBase[n] {
			return n.TypeParams().At(0).Obj().Name()
		}
		return spell(tOf(n))
	}
	// 1. The visitor interfaces: Visitor[T any], cases return T.
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, s := range gd.Specs {
				ts, ok := s.(*ast.TypeSpec)
				if !ok {
					continue
				}
				tn, _ := rw.Info.Defs[ts.Name].(*types.TypeName)
				if tn == nil {
					continue
				}
				if _, ok := ifaces[namedNoErr(tn.Type())]; ok {
					name := ts.Name.Name
					rw.Rewrite[ts.Name] = func() string { return name + "[T any]" }
					for _, m := range ts.Type.(*ast.InterfaceType).Methods.List {
						res := m.Type.(*ast.FuncType).Results.List[0].Type
						rw.Rewrite[res] = func() string { return "T" }
					}
					done["visitor interfaces given their type parameter"]++
				}
				// self_ / vt_ interfaces listing case methods; JSType_I loses visitUntyped*_impl.
				if it, ok := ts.Type.(*ast.InterfaceType); ok {
					for _, m := range it.Methods.List {
						if len(m.Names) != 1 {
							continue
						}
						fn, _ := rw.Info.Defs[m.Names[0]].(*types.Func)
						if fn == nil {
							continue
						}
						if n, ok := retypedIface[fn]; ok {
							res := m.Type.(*ast.FuncType).Results.List[0].Type
							ts := tSpell(n)
							rw.Rewrite[res] = func() string { return ts }
							done["case methods of self_ / vt_ interfaces returning T"]++
						}
						if tn.Type() == selfI || namedNoErr(tn.Type()) == selfI {
							for _, fr := range fronts {
								if m.Names[0].Name == fr.impl {
									m := m
									rw.Rewrite[m] = func() string { return rw.Del }
									done["dispatch entries deleted from "+selfI.Obj().Name()]++
								}
							}
						}
					}
				}
			}
		}
	}
	// 2. Embedded generic bases instantiated with any.
	for n, base := range instField {
		if !isAny(instArg[n]) {
			continue
		}
		ts := spell(tOf(n))
		for _, f := range rw.Files {
			ast.Inspect(f, func(x ast.Node) bool {
				ix, ok := x.(*ast.IndexExpr)
				if !ok {
					return true
				}
				id, ok := ix.X.(*ast.Ident)
				if !ok || rw.Info.Uses[id] != base.Obj() || !isAny(typeOf(ix.Index)) {
					return true
				}
				if !belongsTo(ix, n) {
					return true
				}
				rw.Rewrite[ix.Index] = func() string { return ts }
				done["generic visitor bases instantiated with any given T"]++
				return true
			})
		}
	}
	// 3. The case methods return T.
	for fn, n := range retyped {
		fd := declOf[fn]
		if fd == nil {
			continue
		}
		ts := tSpell(n)
		var t types.Type
		if !genericBase[n] {
			t = tOf(n)
		}
		res := fd.Type.Results.List[0].Type
		rw.Rewrite[res] = func() string { return ts }
		retReturns(fd.Body, t, ts)
		done["case methods returning T"]++
	}
	// Forwarders.
	for fn, p := range forwarders {
		fd := declOf[fn]
		ts := spell(tOf(p))
		res := fd.Type.Results.List[0].Type
		rw.Rewrite[res] = func() string { return ts }
		done["forwarders returning T ("+fn.Name()+")"]++
	}
	for fn, n := range helpers {
		fd := declOf[fn]
		ts := tSpell(n)
		rw.Rewrite[fd.Type.Results.List[0].Type] = func() string { return ts }
		retReturns(fd.Body, tOf(n), ts)
		done["helpers of visitor classes returning T"]++
	}
	// 4. Slot type expressions.
	for _, s := range slotTypeExprs {
		s := s
		ts := spell(tOf(s.node))
		rw.Rewrite[s.id] = func() string { return s.id.Name + "[" + ts + "]" }
		done["Visitor-typed slots given T"]++
	}
	// 5. Calls: visit -> the generic function; case / forwarder calls read as T.
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			if fd, ok := x.(*ast.FuncDecl); ok && fd.Recv != nil && isDeletedDecl(fd) && !isImplDecl(fd) {
				return false
			}
			call, ok := x.(*ast.CallExpr)
			if !ok {
				return true
			}
			if fr, _ := visitCall(call); fr != nil {
				rewriteVisit(fr, call)
				return true
			}
			if name, c := rw.CallName(call); c != nil && strings.HasPrefix(name, "icast_") && len(call.Args) == 1 {
				if isVisitorType(typeOf(call)) {
					rewriteIcast(call)
				}
			}
			fn := calledFunc(call)
			if fn == nil {
				return true
			}
			var t types.Type
			if n, ok := retyped[fn]; ok && !genericBase[n] {
				t = tOf(n)
			} else if n, ok := retypedIface[fn]; ok && !genericBase[n] {
				t = tOf(n)
			} else if p, ok := forwarders[fn]; ok {
				t = tOf(p)
			} else if n, ok := helpers[fn]; ok {
				t = tOf(n)
			} else if isVisitorIfaceMethod(fn) {
				if s, ok := call.Fun.(*ast.SelectorExpr); ok {
					t = tOf(valueNode(s.X))
				}
			}
			if t == nil || inReturnOfRetyped(call) || inImplBody(call) {
				return true
			}
			readAs(call, t, func() string { return rw.RenderChildren(call) })
			return true
		})
	}
	// 6. Delete the dispatch, the overrides; the fronts become generic functions.
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || !isDeletedDecl(fd) {
				continue
			}
			fn, _ := rw.Info.Defs[fd.Name].(*types.Func)
			if fr := frontOf[fn]; fr != nil {
				fd := fd
				countCases = true
				genFront(fr) // once now: its failures are reported before anything is written
				countCases = false
				rw.Rewrite[fd] = func() string { return genFront(fr) }
				done["visit methods made generic functions with a type switch"]++
				continue
			}
			rw.Rewrite[fd] = func() string { return rw.DelDoc }
			if strings.HasSuffix(fd.Name.Name, "_impl") {
				done["visitUntyped overrides moved into the type switch"]++
			} else {
				done["visitUntyped dispatchers deleted"]++
			}
		}
	}
	// 7. icast_ helpers of the visitor interfaces: generic, or deleted when unused.
	for _, fr := range fronts {
		name := "icast_" + fr.iface.Obj().Name()
		o, ok := pkgScope.Lookup(name).(*types.Func)
		if !ok {
			continue
		}
		fd := declOf[o]
		used := false
		for _, u := range rw.UsesOf[o] {
			if c, ok := rw.Parent[u].(*ast.CallExpr); ok {
				if _, kept := keptIcast[c]; kept {
					used = true
				}
			}
		}
		if !used {
			rw.Rewrite[fd] = func() string { return rw.DelDoc }
			done["icast_ helpers of visitor interfaces deleted"]++
			continue
		}
		iname := fr.iface.Obj().Name()
		rw.Rewrite[fd.Name] = func() string { return name + "[T any]" }
		rw.Rewrite[fd.Type.Results.List[0].Type] = func() string { return iname + "[T]" }
		ast.Inspect(fd.Body, func(x ast.Node) bool {
			if ta, ok := x.(*ast.TypeAssertExpr); ok && ta.Type != nil {
				rw.Rewrite[ta.Type] = func() string { return iname + "[T]" }
			}
			return true
		})
		done["icast_ helpers of visitor interfaces made generic"]++
	}
}

var countCases bool

var keptIcast = map[*ast.CallExpr]bool{}

// isIcastDecl: the icast_ helper of a visitor interface (rewritten on its own).
func isIcastDecl(fd *ast.FuncDecl) bool {
	if fd.Recv != nil {
		return false
	}
	for _, fr := range fronts {
		if fd.Name.Name == "icast_"+fr.iface.Obj().Name() {
			return true
		}
	}
	return false
}

func isImplDecl(fd *ast.FuncDecl) bool {
	for _, fr := range fronts {
		if fd.Name.Name == fr.impl {
			return true
		}
	}
	return false
}

func inImplBody(n ast.Node) bool {
	for p := ast.Node(n); p != nil; p = rw.Parent[p] {
		if fd, ok := p.(*ast.FuncDecl); ok {
			return fd.Recv != nil && isImplDecl(fd)
		}
	}
	return false
}

// inReturnOfRetyped: n is a returned value of a retyped method (or of an IIFE in one), or of
// a forwarder: already T there.
func inReturnOfRetyped(n ast.Expr) bool {
	p := rw.Up(n)
	r, ok := p.(*ast.ReturnStmt)
	if !ok {
		return false
	}
	for q := rw.Parent[r]; q != nil; q = rw.Parent[q] {
		switch f := q.(type) {
		case *ast.FuncLit:
			call, ok := rw.Up(f).(*ast.CallExpr)
			if !ok || iife(call) != f {
				return false
			}
			// the IIFE must itself be returned by a retyped method
			return inReturnOfRetyped(call)
		case *ast.FuncDecl:
			fn, _ := rw.Info.Defs[f.Name].(*types.Func)
			_, a := retyped[fn]
			_, b := forwarders[fn]
			_, c := helpers[fn]
			return a || b || c
		}
	}
	return false
}

// belongsTo reports whether n is inside a declaration of class c (its type or a method).
func belongsTo(n ast.Node, c *types.Named) bool {
	for p := ast.Node(n); p != nil; p = rw.Parent[p] {
		switch d := p.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil {
				return false
			}
			return classOf(typeOf(d.Recv.List[0].Type)) == c
		case *ast.TypeSpec:
			tn, _ := rw.Info.Defs[d.Name].(*types.TypeName)
			return tn != nil && tn == c.Obj()
		}
	}
	return false
}

// retReturns converts the returns of body (and of IIFEs returned) to T.
func retReturns(body *ast.BlockStmt, t types.Type, ts string) {
	inspectOwn(body, func(r *ast.ReturnStmt) {
		for _, e := range r.Results {
			convRet(e, t, ts)
		}
	})
}

func convRet(e ast.Expr, t types.Type, ts string) {
	e = rw.Unparen(e)
	if lit := iife(e); lit != nil {
		res := lit.Type.Results.List[0].Type
		if isAny(typeOf(res)) {
			rw.Rewrite[res] = func() string { return ts }
		}
		retReturns(lit.Body, t, ts)
		return
	}
	if id, ok := e.(*ast.Ident); ok && id.Name == "nil" {
		return
	}
	if isCaseOrVisitCall(e) {
		return
	}
	et := typeOf(e)
	if t == nil { // the generic base: boxT_(x) of x of T is x
		if b := boxed(e); b != nil {
			if _, ok := typeOf(b).(*types.TypeParam); ok {
				rw.Rewrite[e] = func() string { return rw.Render(b) }
				done["boxes returned by case methods: the value"]++
			}
		}
		return
	}
	if et != nil && !isAny(et) {
		if types.Identical(et, t) {
			return
		}
		if up := upcast(e, et, t); up != nil {
			rw.Rewrite[e] = up
			return
		}
		fail(e, "a returned %s where T is %s", spell(et), ts)
		return
	}
	if b := boxed(e); b != nil {
		if isCaseOrVisitCall(b) {
			rw.Rewrite[e] = func() string { return rw.Render(b) }
			done["boxes returned by case methods: the value"]++
			return
		}
		b = rw.Unparen(b)
		// any(bool(x)) of a bool x: x
		if c, ok := b.(*ast.CallExpr); ok && len(c.Args) == 1 {
			if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() && types.Identical(tv.Type, t) {
				inner := c.Args[0]
				if it := typeOf(inner); it != nil && (types.Identical(types.Default(it), t)) {
					rw.Rewrite[e] = func() string { return rw.Render(inner) }
					done["boxes returned by case methods: the value"]++
					return
				}
			}
		}
		bt := typeOf(b)
		if bt != nil && types.Identical(types.Default(bt), t) {
			rw.Rewrite[e] = func() string { return rw.Render(b) }
			done["boxes returned by case methods: the value"]++
			return
		}
		if up := upcast(b, bt, t); up != nil {
			rw.Rewrite[e] = up
			done["boxes returned by case methods: the value"]++
			return
		}
		fail(e, "a returned box of %s where T is %s", spell(bt), ts)
		return
	}
	// Any other Object value: the cast to T.
	un := unboxFor(t)
	if un == "" {
		fail(e, "a returned Object value with no unboxing to %s", ts)
		return
	}
	rw.Rewrite[e] = func() string { return un + "(" + rw.RenderChildren(e) + ")" }
	skip("a returned Object value that is not a box (unboxed with the cast for T)", e)
}

// upcast is x.up_C() for x of a subclass pointer type et of t's class C.
func upcast(x ast.Expr, et, t types.Type) func() string {
	ce, ct := classOf(et), classOf(t)
	if ce == nil || ct == nil {
		return nil
	}
	for _, c := range chain(ce)[1:] {
		if c == ct {
			m := "up_" + ct.Obj().Name()
			if !rw.HasMethod(types.NewPointer(ce), m) {
				return nil
			}
			return func() string { return rw.Render(x) + "." + m + "()" }
		}
	}
	return nil
}

// unboxFor is the cast from any to t.
func unboxFor(t types.Type) string {
	if cv := rw.ConvFor(t); cv.OK && cv.Unbox != nil {
		return strings.TrimSuffix(cv.Unbox("\x00"), "(\x00)")
	}
	switch types.Unalias(t).(type) {
	case *types.Basic:
		return "unbox_[" + spell(t) + "]"
	case *types.Named:
		return "cast_[" + spell(t) + "]"
	}
	return ""
}

// boxFor boxes code of type t as the visitors did.
func boxFor(t types.Type, code string) string {
	if cv := rw.ConvFor(t); cv.OK && cv.Box != nil {
		return cv.Box(code)
	}
	switch types.Unalias(t).(type) {
	case *types.Basic:
		return "any(" + code + ")"
	}
	return "obj_(" + code + ")"
}

// readAs rewrites a call whose value is now t: an unboxing caller is dropped, an Object reader
// gets the box.
func readAs(call *ast.CallExpr, t types.Type, code func() string) {
	p := rw.Up(call)
	if pc, ok := p.(*ast.CallExpr); ok && len(pc.Args) == 1 && rw.Unparen(pc.Args[0]) == call {
		if pt := typeOf(pc); pt != nil && types.Identical(pt, t) {
			name, _ := rw.CallName(pc)
			if strings.HasPrefix(name, "to_") || strings.HasPrefix(name, "cast_[") || strings.HasPrefix(name, "unbox_[") || strings.HasPrefix(name, "icast_") {
				rw.Rewrite[pc] = code
				done["unboxings of visitor results dropped"]++
				return
			}
		}
	}
	if ta, ok := p.(*ast.TypeAssertExpr); ok {
		if s, ok := rw.Up(ta).(*ast.SelectorExpr); ok && strings.HasPrefix(s.Sel.Name, "as_") {
			if c, ok := rw.Up(s).(*ast.CallExpr); ok && len(c.Args) == 0 && types.Identical(typeOf(c), t) {
				// The assertion threw Java's NullPointerException on a null result; the method
				// called on the value must throw it too, else the check is kept.
				if derefsAtOnce(c) {
					rw.Rewrite[c] = code
					done["unboxings of visitor results dropped"]++
				} else {
					rw.Rewrite[c] = func() string { return "Preconditions_checkNotNullT(" + code() + ")" }
					done["asserted visitor results whose next method is nil-safe: checkNotNull (Java's NullPointerException)"]++
				}
				return
			}
		}
	}
	if _, ok := p.(*ast.ExprStmt); ok {
		rw.Rewrite[call] = code
		return
	}
	rw.Rewrite[call] = func() string { return boxFor(t, code()) }
	done["visitor results read as Object: boxed as the visitor boxed them"]++
}

// derefsAtOnce reports whether c is the receiver of a method call whose method's first statement
// reads a field of its receiver (a nil receiver panics there: Java's NullPointerException).
func derefsAtOnce(c *ast.CallExpr) bool {
	s, ok := rw.Up(c).(*ast.SelectorExpr)
	if !ok || rw.Unparen(s.X) != ast.Expr(c) {
		return false
	}
	sel := rw.Info.Selections[s]
	if sel == nil || sel.Kind() != types.MethodVal {
		return false
	}
	fn, _ := sel.Obj().(*types.Func)
	fd := declOf[fn]
	if fd == nil || fd.Recv == nil || len(fd.Body.List) == 0 {
		return false
	}
	var e ast.Expr
	switch st := fd.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(st.Results) == 1 {
			e = st.Results[0]
		}
	case *ast.ExprStmt:
		e = st.X
	}
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	m, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	f, ok := m.X.(*ast.SelectorExpr)
	if !ok || !isRecv(f.X, fd) {
		return false
	}
	fs := rw.Info.Selections[f]
	return fs != nil && fs.Kind() == types.FieldVal
}

func rewriteVisit(fr *front, call *ast.CallExpr) {
	sel := call.Fun.(*ast.SelectorExpr)
	recv := sel.X
	node := valueNode(call.Args[0])
	t := tOf(node)
	recvCode := func() string {
		rt := typeOf(recv)
		if c := classOf(rt); c != nil && c != root.Origin() {
			return rw.Render(recv) + ".up_" + root.Obj().Name() + "()"
		}
		return rw.Render(recv)
	}
	code := func() string {
		args := []string{recvCode()}
		for _, a := range call.Args {
			args = append(args, rw.Render(a))
		}
		return fr.name + "(" + strings.Join(args, ", ") + ")"
	}
	if inImplBody(call) || inReturnOfRetyped(call) {
		rw.Rewrite[call] = code
	} else {
		readAs(call, t, code)
	}
	done["visit calls -> "+fr.name]++
}

// rewriteIcast: icast_Visitor(obj_(x)) of a visitor that is never nil is x; else the generic
// icast_Visitor[T].
func rewriteIcast(call *ast.CallExpr) {
	b := boxed(call.Args[0])
	node := valueNode(call)
	t := tOf(node)
	name, _ := rw.CallName(call)
	if b != nil && neverNil(b) {
		rw.Rewrite[call] = func() string { return rw.Render(b) }
		done["icast_ of a visitor that is never nil: the visitor"]++
		return
	}
	keptIcast[call] = true
	ts := spell(t)
	rw.Rewrite[call.Fun] = func() string { return name + "[" + ts + "]" }
	skip("icast_ of a visitor that may be nil (a nil pointer in an interface would not throw): generic icast_", call)
}

// neverNil: this, new_X(...), or a package variable assigned once, in its class's static
// initializer, from new_X(...).
func neverNil(e ast.Expr) bool {
	e = rw.Unparen(e)
	if name, c := rw.CallName(e); c != nil && strings.HasPrefix(name, "new_") {
		return true
	}
	id, ok := e.(*ast.Ident)
	if !ok {
		return false
	}
	if id.Name == "this" {
		return true
	}
	v, ok := rw.Info.Uses[id].(*types.Var)
	if !ok || v.Parent() != pkgScope {
		return false
	}
	n := 0
	good := false
	for _, u := range rw.UsesOf[v] {
		as, ok := rw.Parent[u].(*ast.AssignStmt)
		if !ok {
			continue
		}
		isL := false
		for _, l := range as.Lhs {
			if l == ast.Expr(u) {
				isL = true
			}
		}
		if !isL {
			continue
		}
		n++
		fd, _ := rw.Parent[rw.Parent[as]].(*ast.FuncDecl)
		if fd != nil && fd.Recv == nil && strings.HasSuffix(fd.Name.Name, "_clinit") && len(as.Rhs) == 1 && len(as.Lhs) == 1 {
			if name, c := rw.CallName(as.Rhs[0]); c != nil && strings.HasPrefix(name, "new_") {
				good = true
			}
		}
	}
	return n == 1 && good
}

// genFront is the generic function of a front: a type switch over the classes self_ can hold,
// each case the override of the dispatch method that class inherits or declares.
func genFront(fr *front) string {
	var b strings.Builder
	sig := fr.fn.Type().(*types.Signature)
	params := []string{"this *" + root.Obj().Name()}
	for i := 0; i < sig.Params().Len(); i++ {
		p := sig.Params().At(i)
		pt := spell(p.Type())
		if i == 0 {
			pt += "[T]"
		}
		params = append(params, p.Name()+" "+pt)
	}
	fmt.Fprintf(&b, "// %s is %s.%s: the class's %s override, chosen by a type switch on the most\n// derived object (post-flip: visit).\n",
		fr.name, root.Obj().Name(), fr.fn.Name(), fr.disp)
	fmt.Fprintf(&b, "func %s[T any](%s) T {\n\tswitch this := this.self_.(type) {\n", fr.name, strings.Join(params, ", "))
	var classes []*types.Named
	for _, name := range pkgScope.Names() {
		n, ok := namedOf(pkgScope.Lookup(name))
		if !ok || n.Obj().Name() != name || n.TypeParams().Len() > 0 {
			continue
		}
		if _, ok := n.Underlying().(*types.Struct); !ok {
			continue
		}
		if types.Implements(types.NewPointer(n), selfI.Underlying().(*types.Interface)) {
			classes = append(classes, n)
		}
	}
	for _, n := range classes {
		o, _, _ := types.LookupFieldOrMethod(types.NewPointer(n), true, rw.Pkg, fr.impl)
		m, ok := o.(*types.Func)
		if !ok {
			fail(nil, "%s has no %s", n.Obj().Name(), fr.impl)
			continue
		}
		fd := declOf[m]
		if fd == nil {
			fail(nil, "%s: the declaration of %s is not found", n.Obj().Name(), fr.impl)
			continue
		}
		decl := classOf(m.Type().(*types.Signature).Recv().Type())
		body, ok := caseBody(fr, n, decl, fd)
		if !ok {
			continue
		}
		fmt.Fprintf(&b, "\tcase *%s:\n%s", n.Obj().Name(), body)
	}
	fmt.Fprintf(&b, "\t}\n\tpanic(new_NullPointerException_0()) // self_ is nil\n}")
	if countCases {
		done["cases of the type switches"] += len(classes)
	}
	return b.String()
}

// caseBody renders the override fd (of class decl) as the case of class n: the receiver is
// `this` where n reaches the same field or method, else this.up_Decl(); parameters take the
// front's names.
func caseBody(fr *front, n, decl *types.Named, fd *ast.FuncDecl) (string, bool) {
	recv := rw.Info.Defs[fd.Recv.List[0].Names[0]]
	params := map[types.Object]string{}
	i := 0
	for _, f := range fd.Type.Params.List {
		for _, nm := range f.Names {
			params[rw.Info.Defs[nm]] = fr.params[i]
			i++
		}
	}
	ok := true
	var ids []*ast.Ident
	ast.Inspect(fd.Body, func(x ast.Node) bool {
		id, isId := x.(*ast.Ident)
		if !isId {
			return true
		}
		o := rw.Info.Uses[id]
		if o == recv || params[o] != "" {
			ids = append(ids, id)
		}
		return true
	})
	for _, id := range ids {
		id := id
		o := rw.Info.Uses[id]
		if o != recv {
			nm := params[o]
			rw.Rewrite[id] = func() string { return nm }
			continue
		}
		if n == decl {
			continue
		}
		// a selector through promotion reaching the same object keeps `this`
		if sel, ok := rw.Parent[id].(*ast.SelectorExpr); ok && sel.X == ast.Expr(id) {
			if s := rw.Info.Selections[sel]; s != nil {
				o2, _, _ := types.LookupFieldOrMethod(types.NewPointer(n), true, rw.Pkg, sel.Sel.Name)
				if o2 != nil && originOf(o2) == originOf(s.Obj()) {
					rw.Rewrite[id] = func() string { return "this" }
					continue
				}
			}
		}
		m := "up_" + decl.Obj().Name()
		if !rw.HasMethod(types.NewPointer(n), m) {
			fail(id, "%s has no %s for the inherited %s", n.Obj().Name(), m, fd.Name.Name)
			ok = false
			continue
		}
		rw.Rewrite[id] = func() string { return "this." + m + "()" }
	}
	if !ok {
		return "", false
	}
	txt := rw.Render(fd.Body)
	for _, id := range ids {
		delete(rw.Rewrite, id)
	}
	txt = strings.TrimSpace(txt)
	txt = strings.TrimPrefix(txt, "{")
	txt = strings.TrimSuffix(txt, "}")
	txt = strings.Trim(txt, "\n")
	return txt + "\n", true
}

func originOf(o types.Object) types.Object {
	switch x := o.(type) {
	case *types.Var:
		return x.Origin()
	case *types.Func:
		return x.Origin()
	}
	return o
}

var _ = token.NoPos
