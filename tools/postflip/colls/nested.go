package main

// Collections held in typed containers.
//
// A collection of the kind may be an element of something that is typed already: the value
// of a typed map (*TMap[K, ListOf[X]]) or of a Go map, an element of a typed list or a slice,
// the type argument of a generic class (Outcome_base[List]). Such a container is an "outer"
// value: its type has component types (the type arguments of a generic type, the key and
// element of a map, the element of a slice), one of which holds the kind. The slots of outer
// values (variables, fields, parameters, results) are joined as the collections' own are,
// and each component of an outer class is a slot of the kind: what is stored into the
// container enters it, what is read from the container comes from it.
//
// Calls of generic functions and methods (m.get(k), m.put(k, v), Outcome_base_ok_Object[List](l),
// o.get()) bind the type parameters of the callee to slots per call: every parameter, result
// and receiver in whose declared type a type parameter occurs is joined with that slot (the
// values themselves where the type is the parameter, the components of outer values where it
// is nested). That is right only for a callee that handles its type parameter's values
// without looking at them:
//
//   - a runtime generic whose other parameters are plain values (numbers, strings) or
//     conversions: no erased collection, Object or function through which a value of another
//     class could enter. computeIfAbsent with a function literal or a named function is
//     handled (the function's results enter the slot, boxed); the views (asMap, asList, asSet)
//     only where the view goes to a call that reads it or to Map.computeIfAbsent, putIfAbsent
//     and getOrDefault, whose values are followed too.
//   - a generated generic class or function that never converts a value of its type parameter
//     to an interface (boxes): followed through the generic calls it makes.
//
// Where the container converts its elements (the *tconv_[V] of a typed map), the conversion
// of a typed collection is its view (asList) one way, and the typed collection behind the
// view the other (tlistOf_): tcv_... variables, declared with the other conversions.

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"postflip/rw"
)

// comps are the component types of a composite type: the type arguments of a generic named
// type (through a pointer), the key and element of a map, the element of a slice.
func comps(t types.Type) []types.Type {
	if t == nil {
		return nil
	}
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	switch x := t.(type) {
	case *types.Named:
		if x.Obj().Name() == "tconv_" {
			return nil // a conversion holds no values
		}
		if ta := x.TypeArgs(); ta != nil {
			out := make([]types.Type, ta.Len())
			for i := range out {
				out[i] = ta.At(i)
			}
			return out
		}
	case *types.Map:
		return []types.Type{x.Key(), x.Elem()}
	case *types.Slice:
		return []types.Type{x.Elem()}
	}
	return nil
}

// isOuter reports whether a component of t (at any depth) holds a collection of the kind.
func isOuter(t types.Type) bool { return outerDepth(t, 0) }

func outerDepth(t types.Type, d int) bool {
	if d > 4 || K.imm != "" { // the imm kinds (imm.go) convert at containers and type parameters instead
		return false
	}
	for _, c := range comps(t) {
		if isList(c) || outerDepth(c, d+1) {
			return true
		}
	}
	return false
}

func mentions(t types.Type) bool { return isList(t) || isOuter(t) }

// hasTP reports whether a type parameter occurs in t.
func hasTP(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return true
	}
	for _, c := range comps(t) {
		if hasTP(c) {
			return true
		}
	}
	return false
}

// compExprs are the type expressions of the components of type expression te, in the order
// of comps; nil when te does not spell them (a named type).
func compExprs(te ast.Expr) []ast.Expr {
	switch x := rw.Unparen(te).(type) {
	case *ast.StarExpr:
		return compExprs(x.X)
	case *ast.IndexExpr:
		return []ast.Expr{x.Index}
	case *ast.IndexListExpr:
		return x.Indices
	case *ast.MapType:
		return []ast.Expr{x.Key, x.Value}
	case *ast.ArrayType:
		if x.Len == nil {
			return []ast.Expr{x.Elt}
		}
	}
	return nil
}

var ntarg int

// targKey is the slot of component i of the outer class of k.
func targKey(k, i int) int {
	c := cls(k)
	if c.targ == nil {
		c.targ = map[int]int{}
	}
	if t, ok := c.targ[i]; ok {
		return t
	}
	ntarg++
	t := key(fmt.Sprintf("t|%d", ntarg))
	c.targ[i] = t
	if c.deep != "" {
		poisonDeep(t, c.deep)
	}
	return t
}

// poisonDeep poisons slot k and, if it is an outer slot, every component of it: the container
// goes somewhere this tool does not follow.
func poisonDeep(k int, why string) {
	c := cls(k)
	if c.deep != "" {
		return
	}
	c.deep = why
	poison(k, why)
	for _, t := range c.targ {
		poisonDeep(t, why)
	}
}

// declareOuter registers the type expressions of the kind inside the declared type te of
// outer slot k.
func declareOuter(k int, te ast.Expr, t types.Type, fixed bool, what string) {
	cs := comps(t)
	var as []ast.Expr
	if te != nil {
		as = compExprs(te)
		if len(as) != len(cs) {
			as = nil
		}
	}
	for i, ct := range cs {
		switch {
		case isList(ct):
			kk := targKey(k, i)
			switch {
			case fixed:
				poison(kk, what)
			case as == nil:
				poison(kk, "held in a container whose declared type does not spell its components")
			default:
				declareType(kk, as[i])
			}
		case isOuter(ct):
			var a ast.Expr
			if as != nil {
				a = as[i]
			}
			if a == nil && !fixed {
				poisonDeep(targKey(k, i), "held in a container whose declared type does not spell its components")
				continue
			}
			declareOuter(targKey(k, i), a, ct, fixed, what)
		}
	}
	if fixed {
		poisonDeep(k, what)
	}
}

func at(n ast.Node) string {
	p := rw.Fset.Position(n.Pos())
	return fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
}

// ---- generic calls

// calleeObj is the function or method a call names, with its signature as instantiated at
// the call and as declared.
func calleeObj(c *ast.CallExpr) (o *types.Func, isig, osig *types.Signature) {
	fun := rw.Unparen(c.Fun)
	switch x := fun.(type) {
	case *ast.IndexExpr:
		fun = x.X
	case *ast.IndexListExpr:
		fun = x.X
	}
	switch f := fun.(type) {
	case *ast.Ident:
		o, _ = rw.Info.Uses[f].(*types.Func)
		if o != nil {
			isig, _ = typeOf(c.Fun).(*types.Signature)
		}
	case *ast.SelectorExpr:
		if s, ok := rw.Info.Selections[f]; ok {
			o, _ = s.Obj().(*types.Func)
			if o != nil {
				isig, _ = o.Type().(*types.Signature)
			}
		} else if o, _ = rw.Info.Uses[f.Sel].(*types.Func); o != nil {
			isig, _ = typeOf(c.Fun).(*types.Signature)
		}
	}
	if o == nil || isig == nil {
		return nil, nil, nil
	}
	o = o.Origin()
	osig = o.Type().(*types.Signature)
	return o, isig, osig
}

type callBind struct {
	call   *ast.CallExpr
	tp     map[string]int
	params []int // per argument: the slot a value of the kind passed there enters (0: none)
	result int   // the slot of the call's value (of the kind, or outer); 0: none
}

var binds = map[*ast.CallExpr]*callBind{}

func (b *callBind) tpKey(p *types.TypeParam) int {
	n := p.Obj().Name()
	if k, ok := b.tp[n]; ok {
		return k
	}
	k := key(fmt.Sprintf("tp|%d|%s", b.call.Pos(), n))
	b.tp[n] = k
	return k
}

// plainParam reports whether no value of another class can enter a generic's type parameter
// through a parameter of declared type t: a number, string or bool, a type parameter, or a
// composite of those (a typed collection, a conversion).
func plainParam(t types.Type) bool {
	t = types.Unalias(t)
	switch t.(type) {
	case *types.Basic, *types.TypeParam:
		return true
	}
	if _, ok := isConv(t); ok {
		return true
	}
	cs := comps(t)
	if len(cs) == 0 {
		return false
	}
	for _, c := range cs {
		if !plainParam(c) {
			return false
		}
	}
	return true
}

func isConv(t types.Type) (*types.TypeParam, bool) {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil, false
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.Obj().Name() != "tconv_" || n.TypeArgs() == nil {
		return nil, false
	}
	tp, ok := types.Unalias(n.TypeArgs().At(0)).(*types.TypeParam)
	return tp, ok
}

// viewMethods make an erased view of a typed collection.
var viewMethods = map[string]bool{"asList": true, "asMap": true, "asSet": true}

// link joins two slots holding the same value (of the kind, or outer); 0 is no slot.
func link(a, b int) {
	if a != 0 && b != 0 {
		union(a, b)
	}
}

// bind is the binding of the type parameters of the generic callee of call to slots, or nil
// if the callee is not generic or nothing of the kind passes through it.
func bind(call *ast.CallExpr) *callBind {
	if K.imm != "" {
		return nil // imm.go: a type parameter's value is converted where it is used (genericResult, genericParam)
	}
	if b, ok := binds[call]; ok {
		return b
	}
	binds[call] = nil
	if tv, ok := rw.Info.Types[call.Fun]; ok && tv.IsType() {
		return nil
	}
	o, isig, osig := calleeObj(call)
	if o == nil || (osig.TypeParams().Len() == 0 && osig.RecvTypeParams().Len() == 0) {
		return nil
	}
	// Something of the kind must pass where the callee declares a type parameter.
	relevant := false
	if isig.Recv() != nil && mentions(isig.Recv().Type()) {
		relevant = true
	}
	for i := 0; i < isig.Params().Len(); i++ {
		relevant = relevant || (mentions(isig.Params().At(i).Type()) && hasTP(osig.Params().At(i).Type()))
		if _, ok := isConv(osig.Params().At(i).Type()); ok {
			relevant = relevant || mentions(types.Unalias(isig.Params().At(i).Type()).(*types.Pointer).Elem().(*types.Named).TypeArgs().At(0))
		}
	}
	for i := 0; i < isig.Results().Len(); i++ {
		relevant = relevant || (mentions(isig.Results().At(i).Type()) && hasTP(osig.Results().At(i).Type()))
	}
	if !relevant || !generated(call.Pos()) {
		return nil
	}
	save := curPos
	curPos = call.Pos()
	defer func() { curPos = save }()
	b := &callBind{call: call, tp: map[string]int{}, params: make([]int, len(call.Args))}
	binds[call] = b
	why := ""
	var match func(ot, it types.Type, k func() int)
	match = func(ot, it types.Type, k func() int) {
		if !mentions(it) {
			return
		}
		if p, ok := types.Unalias(ot).(*types.TypeParam); ok {
			link(b.tpKey(p), k())
			return
		}
		ocs, ics := comps(ot), comps(it)
		if len(ocs) != len(ics) {
			if kk := k(); kk != 0 {
				poisonDeep(kk, "passed to "+o.Name()+" as another type")
			}
			return
		}
		for i := range ics {
			if !mentions(ics[i]) {
				continue
			}
			i := i
			if !hasTP(ocs[i]) {
				if kk := k(); kk != 0 {
					poisonDeep(targKey(kk, i), "a component "+o.Name()+" declares itself")
				}
				continue
			}
			match(ocs[i], ics[i], func() int {
				if kk := k(); kk != 0 {
					return targKey(kk, i)
				}
				return 0
			})
		}
	}
	// The receiver.
	var recvKey int
	if osig.Recv() != nil && mentions(isig.Recv().Type()) {
		sel := rw.Unparen(call.Fun).(*ast.SelectorExpr)
		recvKey = outerSlot(sel.X)
		rt, it := typeOf(sel.X), isig.Recv().Type()
		if p, ok := types.Unalias(rt).(*types.Pointer); ok {
			rt = p.Elem()
		}
		if p, ok := types.Unalias(it).(*types.Pointer); ok {
			it = p.Elem()
		}
		if !types.Identical(rt, it) {
			why = "a method of an embedded type (" + o.Name() + ")"
		}
		match(osig.Recv().Type(), isig.Recv().Type(), func() int { return recvKey })
	}
	// Views of the container.
	if osig.Recv() != nil && viewMethods[o.Name()] && recvKey != 0 {
		viewOf(call, recvKey, isig.Recv().Type())
		return b
	}
	lambdaArg := -1
	switch o.Name() {
	case "computeIfAbsent_f":
		lambdaArg = 1
	case "mapComputeIfAbsent_f":
		lambdaArg = 4
	}
	np := osig.Params().Len()
	for i, arg := range call.Args {
		if i >= np || (osig.Variadic() && i >= np-1) {
			if mentions(typeOf(arg)) {
				why = "a variadic parameter of " + o.Name()
			}
			continue
		}
		ot, it := osig.Params().At(i).Type(), isig.Params().At(i).Type()
		if i == lambdaArg {
			continue
		}
		if !plainParam(ot) && !generated(o.Pos()) {
			why = o.Name() + " of the runtime takes a " + types.TypeString(ot, types.RelativeTo(rw.Pkg)) + ", through which other values may enter"
		}
		if p, ok := isConv(ot); ok {
			ct := types.Unalias(it).(*types.Pointer).Elem().(*types.Named).TypeArgs().At(0)
			if !mentions(ct) {
				continue
			}
			if id, ok := rw.Unparen(arg).(*ast.Ident); ok && isList(ct) {
				if v, ok := rw.Info.Uses[id].(*types.Var); ok && v.Parent() == rw.Pkg.Scope() {
					convArg(b.tpKey(p), id)
					continue
				}
			}
			poisonDeep(b.tpKey(p), "a conversion that is not a conversion variable of the collection")
			continue
		}
		if !mentions(it) {
			continue
		}
		if p, ok := types.Unalias(ot).(*types.TypeParam); ok {
			if isList(it) {
				b.params[i] = b.tpKey(p)
			} else {
				link(b.tpKey(p), outerSlot(arg))
			}
			continue
		}
		arg := arg
		match(ot, it, func() int { return outerSlot(arg) })
	}
	if lambdaArg >= 0 && lambdaArg < len(call.Args) {
		// The function's results become values of the map: V of the receiver or of the map argument.
		var vk int
		var mt types.Type
		if osig.Recv() != nil {
			vk, mt = targKey(recvKey, 1), isig.Recv().Type()
		} else {
			vk, mt = targKey(outerSlot(call.Args[2]), 1), typeOf(call.Args[2])
		}
		if cs := comps(mt); len(cs) == 2 && isList(cs[1]) {
			lambdaInto(vk, call.Args[lambdaArg])
		} else if len(cs) == 2 && isOuter(cs[1]) {
			poisonDeep(vk, "computed by a function, boxed")
		}
	}
	switch {
	case isig.Results().Len() == 1:
		ot, it := osig.Results().At(0).Type(), isig.Results().At(0).Type()
		if mentions(it) && hasTP(ot) {
			if p, ok := types.Unalias(ot).(*types.TypeParam); ok {
				b.result = b.tpKey(p)
			} else {
				b.result = key(fmt.Sprintf("res|%d", call.Pos()))
				r := b.result
				match(ot, it, func() int { return r })
			}
		}
	case isig.Results().Len() > 1:
		for i := 0; i < isig.Results().Len(); i++ {
			if mentions(isig.Results().At(i).Type()) && hasTP(osig.Results().At(i).Type()) {
				why = o.Name() + " has several results"
			}
		}
	}
	// A result that is an erased object (an iterator's next(), a view of a slice): the
	// elements leave as Objects.
	if isig.Results().Len() == 1 && types.IsInterface(isig.Results().At(0).Type()) && !hasTP(osig.Results().At(0).Type()) {
		var top ast.Expr = call
		for {
			if p, ok := rw.Parent[top].(*ast.ParenExpr); ok {
				top = p
				continue
			}
			break
		}
		if recvKey != 0 {
			markBoxed(recvKey, isig.Recv().Type(), call)
		}
		for _, arg := range call.Args {
			if at := typeOf(arg); isOuter(at) {
				if k := outerSlot(arg); k != 0 {
					if osig.Recv() == nil && !readsOnly(top) {
						poisonDeep(k, "seen as an erased collection, which may store other values ("+o.Name()+")")
					} else {
						markBoxed(k, at, call)
					}
				}
			}
		}
	}
	// Type arguments written at the call.
	var targs []ast.Expr
	switch x := rw.Unparen(call.Fun).(type) {
	case *ast.IndexExpr:
		targs = []ast.Expr{x.Index}
	case *ast.IndexListExpr:
		targs = x.Indices
	}
	if osig.Recv() == nil {
		for j, a := range targs {
			if j >= osig.TypeParams().Len() {
				break
			}
			tv, ok := rw.Info.Types[a]
			if !ok || !tv.IsType() {
				break
			}
			p := osig.TypeParams().At(j)
			switch {
			case isList(tv.Type):
				declareType(b.tpKey(p), a)
			case isOuter(tv.Type):
				declareOuter(b.tpKey(p), a, tv.Type, false, "")
			}
		}
	}
	if why == "" {
		why = genBoxes(o)
	}
	if why != "" {
		for _, k := range b.tp {
			poisonDeep(k, why)
		}
		if recvKey != 0 {
			poisonDeep(recvKey, why)
		}
	}
	return b
}

// ---- conversions of typed collections held in typed containers

// nested are the kinds whose typed collection has a view to be boxed as: the function making
// the conversion of the typed collection, and the one giving the typed collection of a view.
var nested = map[string][2]string{
	"list": {"tlistConv_", "tlistOf_"},
	"map":  {"tmapConv_", "tmapOf_"},
	"set":  {"tsetConv_", "tsetOf_"},
}

func (c *class) typeArgs() string { return "[" + strings.Join(c.elemText, ", ") + "]" }

// nestedConv is the conversion variable of the class's typed collection as an element.
func (c *class) nestedConv() string {
	// The element conversions must exist too: the typed collections are made with them.
	n := "tcv_" + nonIdent.ReplaceAllString(strings.ReplaceAll(K.goType(c), "*", "p_"), "_")
	if rw.Pkg.Scope().Lookup(n) != nil {
		return n
	}
	if _, ok := convs[n]; !ok {
		convs[n] = fmt.Sprintf("var %s = %s%s()", n, nested[K.name][0], c.typeArgs())
	}
	return n
}

// convArg handles conversion variable id passed for a type parameter bound to slot k.
func convArg(k int, id *ast.Ident) {
	if _, ok := nested[K.name]; !ok {
		poison(k, "held in a typed container that converts its elements (no view of a typed "+K.name+")")
		return
	}
	edit(k, func(c *class) rw.Edit { return rw.Edit{Node: id, Gen: c.nestedConv} })
}

// ---- views of the container, and values that pass boxed

var (
	boxedSrc = map[*ast.CallExpr]int{}         // calls whose Object result is a value of the slot
	boxedArg = map[*ast.CallExpr]map[int]int{} // arguments that enter the slot as Objects
	lret     = map[ast.Node]int{}              // functions whose Object results enter the slot
	lretUse  = map[*ast.Ident]bool{}
	declOf   = map[*types.Func]*ast.FuncDecl{}
)

// viewOf handles x.asMap() (asList, asSet) of an outer value: erased code may store values of
// other classes through the view, unless the view goes to a call that only reads it or to one
// of the map helpers whose values are followed.
func viewOf(view *ast.CallExpr, recvKey int, recvType types.Type) {
	var top ast.Expr = view
	for {
		if p, ok := rw.Parent[top].(*ast.ParenExpr); ok {
			top = p
			continue
		}
		break
	}
	if readsOnly(top) {
		markBoxed(recvKey, recvType, view) // the reader may keep the elements, as Objects
		return
	}
	if p, ok := rw.Parent[top].(*ast.CallExpr); ok && len(p.Args) >= 1 && p.Args[0] == top && K.view != "" {
		n, _ := rw.CallName(p)
		cs := comps(recvType)
		if len(cs) == 2 && isList(cs[1]) {
			vk := targKey(recvKey, 1)
			switch {
			case (n == "Map_computeIfAbsent_f") && len(p.Args) == 3:
				lambdaInto(vk, p.Args[2])
				boxedSrc[p] = vk
				return
			case (n == "Map_putIfAbsent" || n == "Map_getOrDefault") && len(p.Args) == 3:
				if a := rw.Unparen(p.Args[2]); !isList(typeOf(unwrap(a))) {
					if id, ok := a.(*ast.Ident); !ok || id.Name != "nil" {
						poisonDeep(vk, "an Object that is not a "+K.name+" is stored ("+at(a)+")")
					}
				}
				boxedArg[p] = map[int]int{2: vk}
				boxedSrc[p] = vk
				return
			}
		}
	}
	poisonDeep(recvKey, "seen as an erased collection, which may store other values ("+at(view)+")")
}

// markBoxed records that erased code gets the elements of outer slot k (of type t) as Objects
// at n: the views of typed collections of the kind, which no cast to the concrete runtime
// collection accepts.
func markBoxed(k int, t types.Type, n ast.Node) {
	for i, ct := range comps(t) {
		switch {
		case isList(ct):
			if c := cls(targKey(k, i)); c.anyAt == "" {
				c.anyAt = at(n)
			}
		case isOuter(ct):
			markBoxed(targKey(k, i), ct, n)
		}
	}
}

// lambdaInto records that the Object results of function f (a literal or a named function)
// are values of slot k.
func lambdaInto(k int, f ast.Expr) {
	var node ast.Node
	switch x := rw.Unparen(f).(type) {
	case *ast.FuncLit:
		node = x
	case *ast.Ident:
		if o, ok := rw.Info.Uses[x].(*types.Func); ok && declOf[o] != nil && generated(o.Pos()) {
			node = declOf[o]
			lretUse[x] = true
		}
	}
	if node == nil || K.view == "" {
		poisonDeep(k, "computed by a function that is not a literal or a named function")
		return
	}
	if old, ok := lret[node]; ok {
		union(old, k)
		return
	}
	lret[node] = k
}

// checkLambdas poisons the slots fed by functions that are also used elsewhere or return
// something that is not a collection of the kind.
func checkLambdas() {
	for node, k := range lret {
		curPos = node.Pos()
		var body *ast.BlockStmt
		switch x := node.(type) {
		case *ast.FuncDecl:
			body = x.Body
			if o, ok := rw.Info.Defs[x.Name].(*types.Func); ok {
				for _, u := range rw.UsesOf[o] {
					if !lretUse[u] {
						poisonDeep(k, "computed by a function that is used elsewhere too")
					}
				}
			}
		case *ast.FuncLit:
			body = x.Body
		}
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.ReturnStmt:
				if len(x.Results) != 1 {
					poisonDeep(k, "computed by a function with several results")
					return true
				}
				r := rw.Unparen(x.Results[0])
				if id, ok := r.(*ast.Ident); ok && id.Name == "nil" {
					return true
				}
				if !isList(typeOf(unwrap(r))) {
					poisonDeep(k, "computed by a function that returns something that is not a "+K.name)
				}
			}
			return true
		})
	}
}

// flowBoxed records that the value of src (whose outermost conversion to Object is top) goes
// into slot dest boxed: the typed collection's view is what the container's conversion takes.
func flowBoxed(dest int, src ast.Expr, top ast.Expr) {
	box := func(s string) string { return "any(" + s + "." + K.view + "())" }
	v := valueOf(src)
	if v.kind == vNew || v.kind == vCopy {
		ctorDest[unwrap(src)] = dest
	}
	switch v.kind {
	case vNil:
	case vForeign:
		poison(dest, v.why)
	case vNew:
		capExpr := v.cap
		if capExpr != nil && (K.name == "treeset" || K.name == "treemap") {
			cls(dest).cmpNew = true
		}
		edit(dest, func(c *class) rw.Edit {
			return rw.Edit{Node: top, Gen: func() string { return box(K.newCode(c, capExpr)) }}
		})
	case vImm, vCopy:
		inferBad(dest, -1, "an erased collection is copied in")
		from, imm := v.from, v.kind == vImm
		edit(dest, func(c *class) rw.Edit {
			return rw.Edit{Node: top, Gen: func() string { return box(K.fromCode(c, rw.Render(from), imm)) }}
		})
	case vSlot:
		union(dest, v.k)
		inner := unwrap(src)
		edit(dest, func(c *class) rw.Edit {
			return rw.Edit{Node: top, Gen: func() string { return box(rw.Render(inner)) }}
		})
	case vTyped:
		poison(dest, "the view of a typed collection, boxed")
	}
}

// ---- outer values

var (
	outerMemo = map[ast.Expr]int{}
	nfresh    int
)

func freshPoisoned(why string) int {
	nfresh++
	k := key(fmt.Sprintf("x|%d", nfresh))
	poisonDeep(k, why)
	return k
}

// fieldComp: x.f where x is an instance of a generic struct and f is declared as its type
// parameter i.
func fieldComp(x *ast.SelectorExpr) (int, bool) {
	sel, ok := rw.Info.Selections[x]
	if !ok || sel.Kind() != types.FieldVal || len(sel.Index()) != 1 {
		return 0, false
	}
	fv, ok := sel.Obj().(*types.Var)
	if !ok {
		return 0, false
	}
	p, ok := types.Unalias(fv.Origin().Type()).(*types.TypeParam)
	if !ok {
		return 0, false
	}
	return p.Index(), len(comps(typeOf(x.X))) > p.Index()
}

// outerSlot is the slot of outer-valued expression e (0 for nil).
func outerSlot(e ast.Expr) int {
	e = rw.Unparen(e)
	if k, ok := outerMemo[e]; ok {
		return k
	}
	save := curPos
	curPos = e.Pos()
	k := outerSlot1(e)
	curPos = save
	outerMemo[e] = k
	return k
}

func outerSlot1(e ast.Expr) int {
	switch x := e.(type) {
	case *ast.Ident:
		if x.Name == "nil" {
			return 0
		}
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			return outerSlot(x.X)
		}
	case *ast.StarExpr:
		return outerSlot(x.X)
	case *ast.SliceExpr:
		return outerSlot(x.X)
	case *ast.CompositeLit:
		k := key(fmt.Sprintf("lit|%d", x.Pos()))
		if x.Type != nil {
			declareOuter(k, x.Type, typeOf(x), !generated(x.Pos()), "literal of the runtime")
		} else {
			poisonDeep(k, "a literal without its type")
		}
		return k
	case *ast.IndexExpr:
		switch types.Unalias(typeOf(x.X)).Underlying().(type) {
		case *types.Map:
			return targKey(outerSlot(x.X), 1)
		case *types.Slice:
			return targKey(outerSlot(x.X), 0)
		}
		return freshPoisoned("an indexed value")
	case *ast.SelectorExpr:
		if s, ok := rw.Info.Selections[x]; ok && s.Kind() == types.FieldVal {
			if fv, ok := s.Obj().(*types.Var); ok && hasTP(fv.Origin().Type()) {
				if i, ok := fieldComp(x); ok {
					return targKey(outerSlot(x.X), i)
				}
				return freshPoisoned("a field of a generic class")
			}
		}
	case *ast.CallExpr:
		if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
			if len(x.Args) == 1 && types.Identical(typeOf(x.Args[0]), tv.Type) {
				return outerSlot(x.Args[0])
			}
			return freshPoisoned("a converted value")
		}
		if id, ok := rw.Unparen(x.Fun).(*ast.Ident); ok {
			if _, builtin := rw.Info.Uses[id].(*types.Builtin); builtin {
				switch id.Name {
				case "append":
					return outerSlot(x.Args[0])
				case "make", "new":
					k := key(fmt.Sprintf("lit|%d", x.Pos()))
					declareOuter(k, x.Args[0], typeOf(x.Args[0]), !generated(x.Pos()), "made by the runtime")
					return k
				}
				return freshPoisoned("the result of " + id.Name)
			}
		}
		if fl, ok := rw.Unparen(x.Fun).(*ast.FuncLit); ok {
			return resultKey(litGroup(fl), 0)
		}
		if b := bind(x); b != nil {
			if b.result == 0 {
				return freshPoisoned("the result of a generic call")
			}
			return b.result
		}
		o, _, osig := calleeObj(x)
		if o == nil {
			return freshPoisoned("the result of a function value")
		}
		if osig.TypeParams().Len() > 0 || osig.RecvTypeParams().Len() > 0 {
			return freshPoisoned("the result of generic " + o.Name())
		}
		if !generated(o.Pos()) {
			return freshPoisoned("the result of " + o.Name())
		}
		if osig.Results().Len() != 1 {
			return freshPoisoned("a result of " + o.Name() + ", which has several")
		}
		return resultKey(groupOf(o), 0)
	case *ast.TypeAssertExpr:
		return freshPoisoned("a value cast from an Object")
	}
	if v := varOf(e); v != nil {
		k := varKey(v)
		if !generated(v.Pos()) {
			poisonDeep(k, "variable of the runtime")
		}
		return k
	}
	return freshPoisoned(fmt.Sprintf("the value of a %T", e))
}

// handleOuter classifies the occurrence of outer-valued expression e.
func handleOuter(e ast.Expr) {
	var top ast.Expr = e
	for {
		if p, ok := rw.Parent[top].(*ast.ParenExpr); ok {
			top = p
			continue
		}
		break
	}
	lose := func(why string) {
		if k := outerSlot(e); k != 0 {
			poisonDeep(k, why+" ("+at(e)+")")
		}
	}
	switch x := rw.Parent[top].(type) {
	case *ast.AssignStmt:
		for i, r := range x.Rhs {
			if r != top {
				continue
			}
			if len(x.Lhs) != len(x.Rhs) {
				lose("a multi-value assignment")
				return
			}
			l := rw.Unparen(x.Lhs[i])
			if id, ok := l.(*ast.Ident); ok && id.Name == "_" {
				return
			}
			if !isOuter(typeOf(l)) {
				lose("stored as another type")
				return
			}
			link(outerSlot(l), outerSlot(e))
			return
		}
	case *ast.ValueSpec:
		for i, r := range x.Values {
			if r != top {
				continue
			}
			if len(x.Values) != len(x.Names) {
				lose("a multi-value declaration")
				return
			}
			if x.Names[i].Name == "_" {
				return
			}
			dv := rw.Info.Defs[x.Names[i]].(*types.Var)
			if !isOuter(dv.Type()) {
				lose("stored as another type")
				return
			}
			link(varKey(dv), outerSlot(e))
		}
	case *ast.ReturnStmt:
		g := enclosingFunc(x)
		for i, r := range x.Results {
			if r == top {
				if rt := enclosingResult(x, i); rt == nil || !isOuter(rt) {
					lose("returned as another type")
					return
				}
				link(resultKey(g, i), outerSlot(e))
			}
		}
	case *ast.CallExpr:
		if x.Fun == top {
			return
		}
		if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
			if !isOuter(tv.Type) {
				lose("converted to " + types.TypeString(tv.Type, types.RelativeTo(rw.Pkg)))
			}
			return
		}
		if id, ok := rw.Unparen(x.Fun).(*ast.Ident); ok {
			if _, builtin := rw.Info.Uses[id].(*types.Builtin); builtin {
				switch id.Name {
				case "len", "cap", "delete", "clear", "make", "new":
				case "append":
					for i, a := range x.Args {
						if a == top && i >= 1 {
							if x.Ellipsis.IsValid() {
								link(outerSlot(x.Args[0]), outerSlot(e))
							} else {
								link(targKey(outerSlot(x.Args[0]), 0), outerSlot(e))
							}
						}
					}
				default:
					lose("passed to " + id.Name)
				}
				return
			}
		}
		if bind(x) != nil {
			return
		}
		for i, a := range x.Args {
			if a != top {
				continue
			}
			if fl, ok := rw.Unparen(x.Fun).(*ast.FuncLit); ok {
				link(paramKey(litGroup(fl), i), outerSlot(e))
				return
			}
			o, _, osig := calleeObj(x)
			switch {
			case o == nil:
				lose("passed to a function value")
			case osig.TypeParams().Len() > 0 || osig.RecvTypeParams().Len() > 0:
				lose("passed to generic " + o.Name())
			case !generated(o.Pos()):
				if !rw.ReadOnlySink(top, false) {
					lose("passed to " + o.Name() + " of the runtime")
				}
			case osig.Variadic() && i >= osig.Params().Len()-1:
				lose("passed as a variadic argument")
			case i >= osig.Params().Len() || !isOuter(osig.Params().At(i).Type()):
				lose("passed as another type to " + o.Name())
			default:
				link(paramKey(groupOf(o), i), outerSlot(e))
			}
			return
		}
	case *ast.SelectorExpr:
		if x.X != top {
			return
		}
		if c, ok := rw.Parent[x].(*ast.CallExpr); ok && c.Fun == x {
			if bind(c) == nil {
				if o, _, _ := calleeObj(c); o == nil {
					lose("a call this tool does not resolve")
				}
			}
			return
		}
		if s, ok := rw.Info.Selections[x]; ok && s.Kind() == types.FieldVal {
			return // a field: handled where the field's value is used
		}
		lose("a method value")
	case *ast.IndexExpr, *ast.SliceExpr, *ast.StarExpr, *ast.ExprStmt, *ast.IncDecStmt:
	case *ast.UnaryExpr:
		if x.Op != token.AND {
			lose("used in a unary expression")
		}
	case *ast.RangeStmt:
	case *ast.BinaryExpr:
		other := x.X
		if other == top {
			other = x.Y
		}
		if id, ok := rw.Unparen(other).(*ast.Ident); ok && id.Name == "nil" {
			return
		}
		if isOuter(typeOf(other)) {
			link(outerSlot(other), outerSlot(e))
			return
		}
		lose("compared with another type")
	case *ast.KeyValueExpr:
		if x.Value != top {
			return
		}
		if id, ok := x.Key.(*ast.Ident); ok {
			if fv, ok := rw.Info.Uses[id].(*types.Var); ok && fv.IsField() {
				if isOuter(fv.Type()) && !hasTP(fv.Origin().Type()) {
					link(varKey(fv), outerSlot(e))
				} else {
					lose("stored in a field of another type")
				}
				return
			}
		}
		if lit, ok := rw.Parent[x].(*ast.CompositeLit); ok {
			if _, isMap := types.Unalias(typeOf(lit)).Underlying().(*types.Map); isMap {
				link(targKey(outerSlot(lit), 1), outerSlot(e))
				return
			}
		}
		lose("an element of a literal")
	case *ast.CompositeLit:
		if x.Type == top {
			return
		}
		if _, isSlice := types.Unalias(typeOf(x)).Underlying().(*types.Slice); isSlice {
			link(targKey(outerSlot(x), 0), outerSlot(e))
			return
		}
		lose("an element of a literal")
	default:
		lose(fmt.Sprintf("used in a %T", x))
	}
}

// enclosingResult is the declared type of result i of the function around n.
func enclosingResult(n ast.Node, i int) types.Type {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		var sig *types.Signature
		switch f := p.(type) {
		case *ast.FuncLit:
			sig, _ = typeOf(f).(*types.Signature)
		case *ast.FuncDecl:
			if o, ok := rw.Info.Defs[f.Name].(*types.Func); ok {
				sig = o.Type().(*types.Signature)
			}
		default:
			continue
		}
		if sig == nil || i >= sig.Results().Len() {
			return nil
		}
		return sig.Results().At(i).Type()
	}
	return nil
}

// rangeVars joins the variables of a range loop over an outer value with its components;
// it reports whether variable e (the key or the value of the loop) was handled.
func rangeVar(x *ast.RangeStmt, e ast.Expr, isKey bool) bool {
	xt := typeOf(x.X)
	if !isOuter(xt) {
		return false
	}
	i := -1
	switch u := types.Unalias(xt).Underlying().(type) {
	case *types.Map:
		i = 1
		if isKey {
			i = 0
		}
	case *types.Slice:
		if !isKey {
			i = 0
		}
	case *types.Signature: // iter.Seq[T], iter.Seq2[K, V]
		_ = u
		if n := len(comps(xt)); n == 1 && isKey {
			i = 0
		} else if n == 2 {
			i = 1
			if isKey {
				i = 0
			}
		}
	}
	v := varOf(e)
	if i < 0 || v == nil || !types.Identical(types.Unalias(comps(xt)[i]), types.Unalias(v.Type())) {
		return false
	}
	link(slotOfVar(v), targKey(outerSlot(x.X), i))
	return true
}

// ---- generated generics that box their type parameter

var (
	boxScanned bool
	tpBoxes    = map[*types.TypeParam]string{} // where a value of the type parameter is boxed
	tpEdges    = map[*types.TypeParam][]*types.TypeParam{}
)

// canonTP is the type parameter of the generic class for a method's receiver type parameter.
var canon = map[*types.TypeParam]*types.TypeParam{}

func canonTP(p *types.TypeParam) *types.TypeParam {
	if c, ok := canon[p]; ok {
		return c
	}
	return p
}

func tpsOf(t types.Type, out *[]*types.TypeParam) {
	if t == nil {
		return
	}
	if p, ok := types.Unalias(t).(*types.TypeParam); ok {
		*out = append(*out, canonTP(p))
		return
	}
	for _, c := range comps(t) {
		tpsOf(c, out)
	}
}

// scanGenerics finds, in the generic functions and methods of the package, where a
// value whose type involves a type parameter is converted to an interface, and which type
// parameters of other generics each is passed for.
func scanGenerics() {
	boxScanned = true
	var decls []*ast.FuncDecl
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			o, ok := rw.Info.Defs[fd.Name].(*types.Func)
			if !ok {
				continue
			}
			declOf[o] = fd
			sig := o.Type().(*types.Signature)
			if sig.TypeParams().Len() == 0 && sig.RecvTypeParams().Len() == 0 {
				continue
			}
			if sig.Recv() != nil {
				rt := sig.Recv().Type()
				if p, ok := rt.(*types.Pointer); ok {
					rt = p.Elem()
				}
				if n, ok := types.Unalias(rt).(*types.Named); ok {
					if n.Obj().Name() == "tconv_" {
						continue // the conversion itself: its null test compares identities
					}
					ctp := n.Origin().TypeParams()
					for i := 0; i < sig.RecvTypeParams().Len() && i < ctp.Len(); i++ {
						canon[sig.RecvTypeParams().At(i)] = ctp.At(i)
					}
				}
			}
			if fd.Body != nil {
				decls = append(decls, fd)
			}
		}
	}
	same := func(a, b types.Type) bool { return a != nil && b != nil && types.Identical(a, b) }
	tOf := func(e ast.Expr) types.Type { return rw.Info.TypeOf(e) }
	for _, fd := range decls {
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				// Type parameters of this generic passed for those of another.
				if o, isig, osig := calleeObj(c); o != nil && (osig.TypeParams().Len() > 0 || osig.RecvTypeParams().Len() > 0) {
					// A runtime generic: fine if nothing but plain values and the type parameters'
					// own pass (as for a call from code that is not generic).
					rtBad := ""
					if !generated(o.Pos()) && generated(fd.Pos()) {
						if viewMethods[o.Name()] {
							rtBad = o.Name()
						}
						for i := 0; i < osig.Params().Len(); i++ {
							if !plainParam(osig.Params().At(i).Type()) {
								rtBad = o.Name()
							}
						}
					}
					var walk func(ot, it types.Type)
					walk = func(ot, it types.Type) {
						if p, ok := types.Unalias(ot).(*types.TypeParam); ok {
							var qs []*types.TypeParam
							tpsOf(it, &qs)
							for _, q := range qs {
								tpEdges[q] = append(tpEdges[q], canonTP(p))
								if rtBad != "" {
									if _, ok := tpBoxes[q]; !ok {
										tpBoxes[q] = at(c) + ", passed to " + rtBad
									}
								}
							}
							return
						}
						ocs, ics := comps(ot), comps(it)
						for i := 0; i < len(ocs) && i < len(ics); i++ {
							walk(ocs[i], ics[i])
						}
					}
					if osig.Recv() != nil && isig.Recv() != nil {
						walk(osig.Recv().Type(), isig.Recv().Type())
					}
					for i := 0; i < osig.Params().Len() && i < isig.Params().Len(); i++ {
						walk(osig.Params().At(i).Type(), isig.Params().At(i).Type())
					}
					for i := 0; i < osig.Results().Len() && i < isig.Results().Len(); i++ {
						walk(osig.Results().At(i).Type(), isig.Results().At(i).Type())
					}
				}
			}
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			tv, ok := rw.Info.Types[e]
			if !ok || tv.IsType() || !hasTP(tv.Type) {
				return true
			}
			if _, isFunc := types.Unalias(tv.Type).Underlying().(*types.Signature); isFunc {
				return true
			}
			// The runtime boxes its own composites on purpose (a typed list's view, an iterator):
			// they convert the elements they give out. Only the values themselves count there.
			if _, exact := types.Unalias(tv.Type).(*types.TypeParam); !exact && !generated(fd.Pos()) {
				return true
			}
			var top ast.Expr = e
			for {
				if p, ok := rw.Parent[top].(*ast.ParenExpr); ok {
					top = p
					continue
				}
				break
			}
			boxed := false
			switch x := rw.Parent[top].(type) {
			case *ast.AssignStmt:
				for i, r := range x.Rhs {
					if r == top && len(x.Lhs) == len(x.Rhs) {
						if id, ok := x.Lhs[i].(*ast.Ident); ok && id.Name == "_" {
							continue
						}
						boxed = !same(tOf(x.Lhs[i]), tv.Type)
					}
				}
			case *ast.ValueSpec:
				for i, r := range x.Values {
					if r == top && len(x.Values) == len(x.Names) {
						if dv, ok := rw.Info.Defs[x.Names[i]].(*types.Var); ok {
							boxed = !same(dv.Type(), tv.Type)
						}
					}
				}
			case *ast.ReturnStmt:
				for i, r := range x.Results {
					if r == top {
						boxed = !same(enclosingResult(x, i), tv.Type)
					}
				}
			case *ast.CallExpr:
				if x.Fun == top {
					break
				}
				if ftv, ok := rw.Info.Types[x.Fun]; ok && ftv.IsType() {
					boxed = !same(ftv.Type, tv.Type)
					break
				}
				if id, ok := rw.Unparen(x.Fun).(*ast.Ident); ok {
					if _, builtin := rw.Info.Uses[id].(*types.Builtin); builtin {
						break
					}
				}
				sig, _ := typeOf(x.Fun).(*types.Signature)
				for i, a := range x.Args {
					if a != top {
						continue
					}
					if sig == nil || i >= sig.Params().Len() || (sig.Variadic() && i >= sig.Params().Len()-1) {
						boxed = true
					} else {
						boxed = !same(sig.Params().At(i).Type(), tv.Type)
					}
				}
			case *ast.BinaryExpr:
				other := x.X
				if other == top {
					other = x.Y
				}
				if id, ok := rw.Unparen(other).(*ast.Ident); !ok || id.Name != "nil" {
					boxed = !same(typeOf(other), tv.Type)
				}
			case *ast.KeyValueExpr:
				if x.Value == top {
					if id, ok := x.Key.(*ast.Ident); ok {
						if fv, ok := rw.Info.Uses[id].(*types.Var); ok && fv.IsField() {
							boxed = !same(fv.Type(), tv.Type)
							break
						}
					}
					boxed = true
				}
			case *ast.CompositeLit:
				if x.Type == top {
					break
				}
				boxed = true
				switch u := types.Unalias(typeOf(x)).Underlying().(type) {
				case *types.Struct:
					for i, el := range x.Elts {
						if el == top && i < u.NumFields() {
							boxed = !same(u.Field(i).Type(), tv.Type)
						}
					}
				case *types.Slice:
					boxed = !same(u.Elem(), tv.Type)
				case *types.Map:
					boxed = !same(u.Elem(), tv.Type) && !same(u.Key(), tv.Type)
				}
			case *ast.SelectorExpr:
				// A method of the type parameter's constraint (x.toString()) looks at the value.
				_, exact := types.Unalias(tv.Type).(*types.TypeParam)
				boxed = exact && x.X == top
			case *ast.IndexExpr, *ast.SliceExpr, *ast.StarExpr, *ast.UnaryExpr, *ast.RangeStmt, *ast.ExprStmt,
				*ast.IncDecStmt, *ast.IfStmt, *ast.ForStmt:
			case *ast.TypeAssertExpr, *ast.TypeSwitchStmt:
				boxed = true
			default:
				boxed = true
			}
			if boxed {
				var ps []*types.TypeParam
				tpsOf(tv.Type, &ps)
				for _, p := range ps {
					if _, ok := tpBoxes[p]; !ok {
						tpBoxes[p] = at(e)
						if os.Getenv("COLLS_BOXES") != "" {
							fmt.Fprintf(os.Stderr, "BOXES %s of %s at %s: %s\n", p.Obj().Name(), fd.Name.Name, at(e), rw.Text(rw.Parent[top])[:min(80, len(rw.Text(rw.Parent[top])))])
						}
					}
				}
			}
			return true
		})
	}
}

// genBoxes says why generated generic o cannot hold typed collections: a value of one of its
// type parameters (or of those of the generics it passes them to) is boxed somewhere.
func genBoxes(o *types.Func) string {
	if !boxScanned {
		scanGenerics()
	}
	sig := o.Type().(*types.Signature)
	var start []*types.TypeParam
	for i := 0; i < sig.TypeParams().Len(); i++ {
		start = append(start, sig.TypeParams().At(i))
	}
	for i := 0; i < sig.RecvTypeParams().Len(); i++ {
		start = append(start, canonTP(sig.RecvTypeParams().At(i)))
	}
	seen := map[*types.TypeParam]bool{}
	for len(start) > 0 {
		p := start[0]
		start = start[1:]
		if seen[p] {
			continue
		}
		seen[p] = true
		if w, ok := tpBoxes[p]; ok {
			return "a generic class or function that boxes its type argument (" + w + ")"
		}
		start = append(start, tpEdges[p]...)
	}
	return ""
}
