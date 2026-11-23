// Command native is a post-flip step after steps 3-5: typed collections that no other code
// needs as collection objects become Go slices and maps. Run it as
//
//	native -kind list; native -kind map; native -kind set; native -kind imm
//
// in that order (each kind works on what the earlier ones left).
//
// -kind list: *TList[T] to []T. Step 3 gave lists a type, a heap object with the identity of
// the Java list. A slice is a value: two variables holding one slice see each other's set,
// but not each other's add or remove. So a typed list can become a []T only where no list
// object is ever held by two places at a time at which one of them changes its length. This
// tool proves that with the class analysis of colls (slots: variables, fields, parameters and
// results that exchange values; a class is converted or not as a whole) and, inside a class,
// the direction of each flow:
//
//   - A flow from a local variable or parameter is a move when it is the variable's last use
//     (nothing uses the variable after it, also not in a later turn of a loop, from a function
//     literal, or while a for-each loop walks it), or a return. A flow out of a call result is
//     a move. A new list, or a copy of one, enters by a move. Every other flow (out of a
//     field, or out of a variable used again) is a share, and so is an ImmutableList, whose
//     typed copy is shared.
//   - A slot is unique when everything entering it is a move from a unique slot.
//   - A slot whose list changes through it (add, insert, removeAt, clear, addAll, sort, and
//     set, which an ImmutableList rejects) must be unique, and every flow out of it a move.
//
// Then no list that changes is ever held twice, and value semantics are the list's. A class
// stays a typed list when any of it goes to code that wants the list object: a view kept or
// passed on (asList), the runtime, an Object, a comparison of two lists, an element of
// another collection. A view used for one call that reads the list (ImmutableList.copyOf,
// Iterables.getFirst, String.join, a sort in place...) is made for that call (sliceView_).
//
// The operations become slice operations: get is sliceAt_ (ArrayList's exception), size is
// len, add is append, set is an index assignment; insert, removeAt, clear and the view's
// addAll/remove are helpers of rt_native.go. A for-each loop over a slice whose length the
// loop cannot change is a range loop; over one it can (a unique slot changed somewhere, and
// in the loop or through a field) it walks the slice with eachOf_, which reads it anew at
// every step, as the typed list's cursor did. Step 1's eachOf_ loops over local slices that
// the loop body does not assign become range loops too, and an eachOf_ loop over a field
// does when nothing the loop body can run changes the field's length (fieldLoops: a
// call-graph over-approximation; interface calls reach every implementation, calls of
// function values every function stored where they are called from).
//
// An explicit iterator over a list, it := l.asList().iterator() used only for hasNext and
// next, becomes a range loop (the canonical `for it.hasNext() { var x T = it.next(); ...}`)
// or an int cursor (sliceNext_) when the list does not change while it is in use; one that
// removes through the iterator keeps the typed list. A list boxed for one reading call
// (rw.ReadOnlySink: equals, toString, a Preconditions message...) gets a view for it. The
// stack and queue methods of a deque's typed list (colls -kind deque) are slice helpers.
//
// -kind map, -kind set: *TMap[K, V] to map[K]V and *TSet[K] to map[K]struct{}, when keys
// compare in Go as in Java (rw.KeyMode; a typed map whose keys compare by hashCode/equals
// stays one). A Go map is a reference, like the typed
// map, so no flow analysis is needed; but its order is not the insertion order, so a class
// converts only when nothing iterates it, nothing sees it as an erased Map or Set (except
// Map.computeIfAbsent through the view, and Set.addAll, which do not depend on order), and
// an immutable map is never changed (it would throw).
//
// -kind imm: an ImmutableList<T> (an *ImmutableCollection declared ImmutableListOf[T]) to []T.
// It never changes, so sharing is harmless; a class converts when every slot of it is declared
// ImmutableListOf[T] with one T and no value of it reaches code that wants the object (an
// Object, an erased collection kept, equals), except one-call readers as above. Elements are
// unboxed where the code unboxed them; ImmutableList.of(a, b) is sliceOf_ (which rejects
// nulls as Guava does), any other immutable list from the runtime is copied once (sliceImm_,
// cached on the immutable list).
//
// usage: native -kind list|map|set|imm [-n] [-v] <dir of package closure>
//
//	(-n: report only; -v: list the classes)
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"postflip/rw"
)

// ---- slots, classes, flows

type node struct {
	key     string
	v       *types.Var // a variable or field; nil for a group's parameter or result
	local   bool       // a local variable or parameter
	fn      ast.Node   // the function declaring a local
	mutated string     // the first operation through it that changes the list, if any
	copies  []*ast.Ident
	inRet   map[*ast.Ident]bool // copies that are returned
	out     []*edge
	in      []*edge
	unique  bool
	isRes   bool       // a group's result (a call's value)
	loops   []ast.Node // bodies of the for-each loops over a local, which read it throughout
}

type edge struct {
	from *node // nil for a new list or an ImmutableList
	to   *node
	imm  bool // an ImmutableList: shared
	use  *ast.Ident
}

type class struct {
	// the element type of an ImmutableList class (kind imm), known once it is decided
	elem     types.Type
	elemText string
	cv       rw.Conv
	tc       string
	poison   []string
	typeExpr []ast.Expr
	edits    []rw.Edit
	nodes    []*node
	decls    int
}

var (
	nodes  = map[string]*node{}
	parent = map[*node]*node{}
	data   = map[*node]*class{}
)

func get(key string) *node {
	if n, ok := nodes[key]; ok {
		return n
	}
	n := &node{key: key, inRet: map[*ast.Ident]bool{}}
	nodes[key] = n
	parent[n] = n
	data[n] = &class{nodes: []*node{n}}
	return n
}

func find(n *node) *node {
	for parent[n] != n {
		parent[n] = parent[parent[n]]
		n = parent[n]
	}
	return n
}

func union(a, b *node) {
	a, b = find(a), find(b)
	if a == b {
		return
	}
	parent[b] = a
	da, db := data[a], data[b]
	da.poison = append(da.poison, db.poison...)
	da.typeExpr = append(da.typeExpr, db.typeExpr...)
	da.edits = append(da.edits, db.edits...)
	da.nodes = append(da.nodes, db.nodes...)
	delete(data, b)
}

func cls(n *node) *class { return data[find(n)] }

var poisonAt = map[string]string{}
var curPos token.Pos

func poison(n *node, why string) {
	c := cls(n)
	c.poison = append(c.poison, why)
	if _, ok := poisonAt[why]; !ok && curPos.IsValid() {
		p := rw.Fset.Position(curPos)
		poisonAt[why] = fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
	}
}

func edit(n *node, at ast.Node, gen func() string) {
	c := cls(n)
	c.edits = append(c.edits, rw.Edit{Node: at, Gen: gen})
}

func flowEdge(from, to *node, imm bool, use *ast.Ident) {
	e := &edge{from: from, to: to, imm: imm, use: use}
	to.in = append(to.in, e)
	if from != nil {
		from.out = append(from.out, e)
		union(from, to)
	}
}

// ---- types

// kind is what is converted: "list" (*TList[T] to []T), "map" (*TMap[K, V] to map[K]V) or
// "set" (*TSet[K] to map[K]struct{}).
var kind string

var runtimeType = map[string]string{"list": "TList", "map": "TMap", "set": "TSet", "imm": "ImmutableCollection"}

// typeArgs returns the type arguments of t if t is the typed collection being converted.
func typeArgs(t types.Type) []types.Type {
	if t == nil {
		return nil
	}
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.Obj().Name() != runtimeType[kind] || n.Obj().Pkg() != rw.Pkg {
		return nil
	}
	var as []types.Type
	for i := 0; i < n.TypeArgs().Len(); i++ {
		as = append(as, n.TypeArgs().At(i))
	}
	return as
}

// elemOf returns T of *TList[T] (K of a map or set).
func elemOf(t types.Type) types.Type {
	if as := typeArgs(t); as != nil {
		return as[0]
	}
	return nil
}

func isList(t types.Type) bool {
	if kind == "imm" {
		return t != nil && types.Identical(types.Unalias(t), immType)
	}
	return typeArgs(t) != nil
}

var immType types.Type // *ImmutableCollection

func tstr(t types.Type) string { return types.TypeString(t, func(*types.Package) string { return "" }) }

// goType is the Go type that replaces the typed collection type t.
func goType(t types.Type) string {
	as := typeArgs(t)
	switch kind {
	case "map":
		return "map[" + tstr(as[0]) + "]" + tstr(as[1])
	case "set":
		return "map[" + tstr(as[0]) + "]struct{}"
	}
	return "[]" + tstr(as[0])
}

func typeOf(e ast.Expr) types.Type {
	if tv, ok := rw.Info.Types[e]; ok {
		return tv.Type
	}
	return nil
}

func elemText(t types.Type) string {
	return types.TypeString(elemOf(t), func(*types.Package) string { return "" })
}

func generated(p token.Pos) bool {
	b := filepath.Base(rw.Fset.Position(p).Filename)
	return strings.HasPrefix(b, "g_") && b != "g_zstubs.go" && b != "g_0aliases.go"
}

// ---- slot keys

func varNode(v *types.Var) *node {
	n := get(fmt.Sprintf("v|%d", v.Pos()))
	if v.Origin() != v || rw.ReflipGeneric_(v.Type(), false) { // reflip: generic instance
		poison(n, "variable of a type parameter's type")
	}
	if n.v == nil {
		n.v = v
		if !v.IsField() && v.Parent() != rw.Pkg.Scope() && v.Parent() != nil {
			n.local = true
			n.fn = funcAt(v.Pos())
		}
		if !generated(v.Pos()) {
			poison(n, "variable of the runtime")
		}
	}
	return n
}

var funcKeys = map[*types.Func]string{}

func groupOf(f *types.Func) string {
	if g, ok := funcKeys[f]; ok {
		return g
	}
	sig := f.Type().(*types.Signature)
	var g string
	if sig.Recv() == nil {
		g = "f|" + f.Name()
	} else {
		var ps, rs []string
		for i := 0; i < sig.Params().Len(); i++ {
			ps = append(ps, types.TypeString(types.Unalias(sig.Params().At(i).Type()), nil))
		}
		for i := 0; i < sig.Results().Len(); i++ {
			rs = append(rs, types.TypeString(types.Unalias(sig.Results().At(i).Type()), nil))
		}
		g = "m|" + strings.TrimSuffix(f.Name(), "_impl") + "(" + strings.Join(ps, ",") + ")" + strings.Join(rs, ",")
	}
	funcKeys[f] = g
	return g
}

func paramNode(g string, i int) *node { return get(fmt.Sprintf("%s:p%d", g, i)) }
func resultNode(g string, i int) *node {
	n := get(fmt.Sprintf("%s:r%d", g, i))
	n.isRes = true
	return n
}
func litGroup(fl *ast.FuncLit) string { return fmt.Sprintf("lit|%d", fl.Pos()) }

var declared = map[ast.Expr]bool{}

func declareType(n *node, te ast.Expr) {
	if declared[te] {
		return
	}
	declared[te] = true
	c := cls(n)
	c.typeExpr = append(c.typeExpr, te)
}

// declare registers the parameters and results of function type ft as slots of group g.
func declare(g string, ft *ast.FuncType, fixed bool, what string) {
	walk := func(fl *ast.FieldList, mk func(string, int) *node, param bool) {
		if fl == nil {
			return
		}
		i := 0
		for _, f := range fl.List {
			n := max(len(f.Names), 1)
			for j := 0; j < n; j++ {
				if isList(typeOf(f.Type)) {
					k := mk(g, i)
					if fixed {
						poison(k, what)
					} else {
						declareType(k, f.Type)
					}
					if j < len(f.Names) {
						if v, ok := rw.Info.Defs[f.Names[j]].(*types.Var); ok {
							vn := varNode(v)
							if param {
								flowEdge(k, vn, false, nil) // the argument is delivered to this parameter
							} else {
								union(k, vn) // a named result
								poison(k, "named result")
							}
						}
					}
				}
				i++
			}
		}
	}
	walk(ft.Params, paramNode, true)
	walk(ft.Results, resultNode, false)
}

// ---- functions and positions

func funcAt(p token.Pos) ast.Node {
	var best ast.Node
	for _, f := range rw.Files {
		if f.Pos() > p || f.End() < p {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil || n.Pos() > p || n.End() < p {
				return n == nil
			}
			switch n.(type) {
			case *ast.FuncDecl, *ast.FuncLit:
				best = n
			}
			return true
		})
	}
	return best
}

func enclosingFn(n ast.Node) ast.Node {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch p.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return p
		}
	}
	return nil
}

// resultType is the declared type of result i of the function that return statement r is in.
func resultType(r ast.Node, i int) types.Type {
	var t types.Type
	switch f := enclosingFn(r).(type) {
	case *ast.FuncLit:
		t = typeOf(f)
	case *ast.FuncDecl:
		if o, ok := rw.Info.Defs[f.Name].(*types.Func); ok {
			t = o.Type()
		}
	}
	sig, ok := t.(*types.Signature)
	if !ok || i >= sig.Results().Len() {
		return nil
	}
	return sig.Results().At(i).Type()
}

func enclosingGroup(n ast.Node) string {
	switch f := enclosingFn(n).(type) {
	case *ast.FuncLit:
		return litGroup(f)
	case *ast.FuncDecl:
		if o, ok := rw.Info.Defs[f.Name].(*types.Func); ok {
			return groupOf(o)
		}
	}
	return ""
}

// loopsBetween returns the loops that enclose n inside fn.
func loopsBetween(n, fn ast.Node) []ast.Node {
	var ls []ast.Node
	for p := rw.Parent[n]; p != nil && p != fn; p = rw.Parent[p] {
		switch p.(type) {
		case *ast.ForStmt, *ast.RangeStmt:
			ls = append(ls, p)
		}
	}
	return ls
}

func inside(n, outer ast.Node) bool { return n.Pos() >= outer.Pos() && n.End() <= outer.End() }

// terminal reports whether every flow out of local n is a move.
func (n *node) terminal() bool {
	if !n.local || n.fn == nil {
		return false
	}
	uses := rw.UsesOf[n.v]
	for _, u := range uses {
		if enclosingFn(u) != n.fn {
			return false // used from a function literal: it may run at any time
		}
	}
	for _, c := range n.copies {
		if n.inRet[c] {
			continue
		}
		for _, b := range n.loops {
			if inside(c, b) {
				return false // passed on while a loop walks it
			}
		}
		loops := loopsBetween(c, n.fn)
		for _, u := range uses {
			if u == c {
				continue
			}
			if u.Pos() > c.Pos() {
				return false
			}
			for _, l := range loops {
				if inside(u, l) && !(l.Pos() <= n.v.Pos() && n.v.Pos() < l.End()) {
					return false
				}
			}
		}
	}
	return true
}

// ---- values

const (
	vSlot = iota
	vNew
	vImm
	vNil
	vForeign
)

type value struct {
	kind int
	n    *node
	why  string
	use  *ast.Ident
}

func calleeFunc(c *ast.CallExpr) *types.Func {
	switch f := rw.Unparen(c.Fun).(type) {
	case *ast.Ident:
		o, _ := rw.Info.Uses[f].(*types.Func)
		return o
	case *ast.SelectorExpr:
		if s, ok := rw.Info.Selections[f]; ok {
			o, _ := s.Obj().(*types.Func)
			return o
		}
		o, _ := rw.Info.Uses[f.Sel].(*types.Func)
		return o
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			o, _ := rw.Info.Uses[id].(*types.Func)
			return o
		}
	}
	return nil
}

func varOf(e ast.Expr) (*types.Var, *ast.Ident) {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		if v, ok := rw.Info.Uses[x].(*types.Var); ok {
			return v, x
		}
		v, _ := rw.Info.Defs[x].(*types.Var)
		return v, x
	case *ast.SelectorExpr:
		if s, ok := rw.Info.Selections[x]; ok && s.Kind() == types.FieldVal {
			v, _ := s.Obj().(*types.Var)
			return v, x.Sel
		}
		v, _ := rw.Info.Uses[x.Sel].(*types.Var)
		return v, x.Sel
	}
	return nil, nil
}

var newFns = map[string]bool{"newTList_": true, "newTListN_": true, "tlistFrom_": true, "newTMap_": true, "newTSet_": true}

// isImmFrom reports whether call is tmapFrom_/tsetFrom_ of an immutable collection (imm, its
// last argument, true), and whether it is one of them at all.
func isImmFrom(call *ast.CallExpr, n string) (imm, from bool) {
	if (kind == "map" && n == "tmapFrom_") || (kind == "set" && n == "tsetFrom_") {
		id, _ := call.Args[len(call.Args)-1].(*ast.Ident)
		return id != nil && id.Name == "true", true
	}
	return false, false
}

// uncast strips cast_[*ImmutableList](obj_(x)) of an x that already is one (kind imm).
func uncast(e ast.Expr) ast.Expr {
	e = rw.Unparen(e)
	if kind != "imm" {
		return e
	}
	for {
		c, ok := e.(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			return e
		}
		n, _ := rw.CallName(c)
		if n != "cast_[*ImmutableList]" && n != "cast_[*ImmutableCollection]" {
			return e
		}
		in, ok := rw.Unparen(c.Args[0]).(*ast.CallExpr)
		if !ok || len(in.Args) != 1 {
			return e
		}
		if n2, _ := rw.CallName(in); n2 != "obj_" && n2 != "any" || !isList(typeOf(in.Args[0])) {
			return e
		}
		e = rw.Unparen(in.Args[0])
	}
}

// castOuter is the cast of which e is the inside, if any.
func castOuter(e ast.Expr) bool {
	if kind != "imm" {
		return false
	}
	in, ok := rw.Up(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	c, ok := rw.Up(in).(*ast.CallExpr)
	return ok && uncast(c) != c
}

func valueOf(e ast.Expr) value {
	e = uncast(e)
	if id, ok := e.(*ast.Ident); ok && id.Name == "nil" {
		return value{kind: vNil}
	}
	if x, ok := e.(*ast.CallExpr); ok {
		n, _ := rw.CallName(x)
		if kind == "imm" {
			switch {
			case n == "ImmutableList_of" && len(x.Args) == 0, strings.HasPrefix(n, "ImmutableList_of_Object") && !strings.HasSuffix(n, "Arr"):
				return value{kind: vNew}
			}
			// Any other immutable collection from the runtime: the slots it enters are all
			// declared ImmutableList<T> (decideImm), so in Java it is an ImmutableList.
			if f := calleeFunc(x); f != nil && !generated(f.Pos()) {
				return value{kind: vImm}
			}
		}
		if imm, ok := isImmFrom(x, n); ok {
			if imm {
				return value{kind: vImm}
			}
			return value{kind: vNew}
		}
		switch {
		case newFns[n]:
			return value{kind: vNew}
		case n == "tlistImm_", n == "tlistOfT_" && rw.HasFunc("sliceOfT_"): // reflip44a
			return value{kind: vImm}
		}
		if fl, ok := rw.Unparen(x.Fun).(*ast.FuncLit); ok {
			return value{kind: vSlot, n: resultNode(litGroup(fl), 0)}
		}
		if f := calleeFunc(x); f != nil && f.Type().(*types.Signature).Results().Len() == 1 {
			if !generated(f.Pos()) {
				return value{kind: vForeign, why: "result of the runtime's " + f.Name()}
			}
			if rw.ReflipGeneric_(f.Type().(*types.Signature).Results().At(0).Type(), false) { // reflip10
				poison(resultNode(groupOf(f), 0), "result of a type parameter's type")
				return value{kind: vForeign, why: "result of a type parameter's type"}
			}
			if f.Origin() != f { // reflip: generic instance
				poison(resultNode(groupOf(f.Origin()), 0), "result of a method of a generic type")
				return value{kind: vForeign, why: "result of a method of a generic type"}
			}
			return value{kind: vSlot, n: resultNode(groupOf(f), 0)}
		}
		return value{kind: vForeign, why: "result of a call"}
	}
	if v, id := varOf(e); v != nil {
		return value{kind: vSlot, n: varNode(v), use: id}
	}
	return value{kind: vForeign, why: fmt.Sprintf("value of a %T", e)}
}

// flow records that the value of src goes to slot dest.
func flow(dest *node, src ast.Expr, inReturn bool) {
	v := valueOf(src)
	switch v.kind {
	case vNil:
	case vForeign:
		poison(dest, v.why)
	case vNew:
		flowEdge(nil, dest, false, nil)
		newValue(dest, uncast(src).(*ast.CallExpr))
	case vImm:
		flowEdge(nil, dest, true, nil)
		newValue(dest, uncast(src).(*ast.CallExpr))
	case vSlot:
		if v.use != nil && v.n.local {
			v.n.copies = append(v.n.copies, v.use)
			if inReturn {
				v.n.inRet[v.use] = true
			}
		}
		flowEdge(v.n, dest, false, v.use)
	}
	if in := uncast(src); in != rw.Unparen(src) && v.kind != vNil && v.kind != vForeign {
		edit(dest, src, func() string { return rw.Render(in) })
	}
}

// newValue rewrites a new typed list, a copy of a collection, or an ImmutableList.
func newValue(n *node, call *ast.CallExpr) {
	if _, done := newEdited[call]; done {
		return
	}
	newEdited[call] = true
	name, _ := rw.CallName(call)
	if kind == "imm" {
		edit(n, call, func() string {
			c := cls(n)
			if name == "ImmutableList_of" {
				return "[]" + c.elemText + "{}"
			}
			if !strings.HasPrefix(name, "ImmutableList_of_Object") {
				return "sliceImm_(" + c.tc + ", " + rw.RenderChildren(call) + ")"
			}
			var ps []string
			for _, a := range call.Args {
				ps = append(ps, rw.Unboxed(c.cv, c.elem, a))
			}
			return "sliceOf_(" + c.tc + ", " + strings.Join(ps, ", ") + ")"
		})
		return
	}
	et := elemText(typeOf(call))
	A := func(i int) string { return rw.Render(call.Args[i]) }
	gt := goType(typeOf(call))
	switch name {
	case "newTMap_", "newTSet_":
		edit(n, call, func() string { return gt + "{}" })
	case "tmapFrom_":
		edit(n, call, func() string {
			if imm, _ := isImmFrom(call, name); imm {
				return "mapImm_(" + A(0) + ", " + A(1) + ", " + A(2) + ")"
			}
			return "mapFrom_(" + A(0) + ", " + A(1) + ", " + A(2) + ")"
		})
	case "tsetFrom_":
		edit(n, call, func() string {
			if imm, _ := isImmFrom(call, name); imm {
				return "setImm_(" + A(0) + ", " + A(1) + ")"
			}
			return "setFrom_(" + A(0) + ", " + A(1) + ")"
		})
	case "newTList_":
		edit(n, call, func() string { return "[]" + et + "{}" })
	case "newTListN_":
		edit(n, call, func() string { return "make([]" + et + ", 0, " + rw.Render(call.Args[1]) + ")" })
	case "tlistFrom_":
		edit(n, call, func() string { return "sliceFrom_(" + rw.Render(call.Args[0]) + ", " + rw.Render(call.Args[1]) + ")" })
	case "tlistOfT_": // reflip44a
		edit(n, call, func() string {
			if cls(n).ptr {
				return "slicePtrOfT_(" + rw.Render(call.Args[0]) + ", " + rw.Render(call.Args[1]) + ")"
			}
			return "sliceOfT_(" + rw.Render(call.Args[0]) + ", " + rw.Render(call.Args[1]) + ")"
		})
	case "tlistImm_":
		edit(n, call, func() string { return "sliceImm_(" + rw.Render(call.Args[0]) + ", " + rw.Render(call.Args[1]) + ")" })
	}
}

var newEdited = map[*ast.CallExpr]bool{}

// paramFor is the slot of argument i of call c, if the callee is generated code.
func paramFor(c *ast.CallExpr, i int) (*node, bool) {
	if fl, ok := rw.Unparen(c.Fun).(*ast.FuncLit); ok {
		return paramNode(litGroup(fl), i), true
	}
	f := calleeFunc(c)
	if f == nil || !generated(f.Pos()) {
		return nil, false
	}
	sig := f.Type().(*types.Signature)
	if sig.Variadic() && i >= sig.Params().Len()-1 {
		return nil, false
	}
	if i >= sig.Params().Len() || !isList(sig.Params().At(i).Type()) {
		return nil, false
	}
	if rw.ReflipGeneric_(sig.Params().At(i).Type(), false) { // reflip10
		poison(paramNode(groupOf(f), i), "parameter of a type parameter's type")
	}
	if f.Origin() != f { // reflip: generic instance
		poison(paramNode(groupOf(f), i), "argument of a method of a generic type")
		poison(paramNode(groupOf(f.Origin()), i), "argument of a method of a generic type")
	}
	return paramNode(groupOf(f), i), true
}

// handle classifies one occurrence of list-valued expression e.
func handle(e ast.Expr) {
	if castOuter(e) {
		return // the cast is handled as the value
	}
	v := valueOf(e)
	var src *node
	if v.kind == vSlot {
		src = v.n
	}
	top := e
	for {
		if p, ok := rw.Parent[top].(*ast.ParenExpr); ok {
			top = p
			continue
		}
		break
	}
	bad := func(why string) {
		if src != nil {
			poison(src, why)
		}
	}
	switch x := rw.Parent[top].(type) {
	case *ast.AssignStmt:
		for i, r := range x.Rhs {
			if r != top {
				continue
			}
			if len(x.Lhs) != len(x.Rhs) {
				bad("multi-value assignment")
				return
			}
			if id, ok := x.Lhs[i].(*ast.Ident); ok && id.Name == "_" {
				bad("assigned to _")
				return
			}
			if !isList(typeOf(x.Lhs[i])) {
				bad("stored as another type")
				return
			}
			dv, _ := varOf(x.Lhs[i])
			if dv == nil {
				bad("stored into an element of a map, slice or array")
				return
			}
			flow(varNode(dv), e, false)
			return
		}
		return // an assignment target
	case *ast.ValueSpec:
		for i, r := range x.Values {
			if r == top {
				dv := rw.Info.Defs[x.Names[i]].(*types.Var)
				if x.Names[i].Name == "_" || !isList(dv.Type()) {
					bad("stored as another type")
					return
				}
				flow(varNode(dv), e, false)
			}
		}
		return
	case *ast.ReturnStmt:
		g := enclosingGroup(x)
		for i, r := range x.Results {
			if r == top {
				if !isList(typeOf(top)) || g == "" || !isList(resultType(x, i)) {
					bad("returned as another type")
					return
				}
				if rw.ReflipGeneric_(resultType(x, i), false) { // reflip10
					poison(resultNode(g, i), "result of a type parameter's type")
				}
				flow(resultNode(g, i), e, true)
			}
		}
		return
	case *ast.CallExpr:
		for i, a := range x.Args {
			if a != top {
				continue
			}
			if s, ok := rw.Unparen(x.Fun).(*ast.SelectorExpr); ok && isList(typeOf(s.X)) && s.Sel.Name == "addAllT" && kind != "map" {
				// l.addAllT(e): e is read; both are slices or neither.
				if rv := valueOf(s.X); rv.kind == vSlot && src != nil {
					union(rv.n, src)
				} else {
					bad("added to a list that is not a slot")
				}
				return
			}
			if k, ok := paramFor(x, i); ok {
				flow(k, e, false)
				return
			}
			if kind == "imm" && src != nil {
				nm, _ := rw.CallName(x)
				if nm == "obj_" || nm == "any" {
					if fe, ok := rw.Up(x).(*ast.CallExpr); ok && len(fe.Args) == 1 {
						if n2, _ := rw.CallName(fe); n2 == "foreach_" {
							if as, ok := rw.Parent[fe].(*ast.AssignStmt); ok && as.Tok == token.DEFINE && len(as.Lhs) == 1 && rangeLoopConv(as, e, nil, nil, false) {
								immLoops = append(immLoops, immLoop{src, as, e})
								return
							}
							bad("for-each loop of another shape")
							return
						}
					}
				}
				if immBoundary(src, e, x) {
					return
				}
				if nm == "sliceImm_" && i == 1 {
					// a slice of the immutable list (step native -kind list): the slice itself
					edit(src, x, func() string { return rw.Render(e) })
					return
				}
			}
			n, _ := rw.CallName(x)
			if n == "" {
				n = "a method"
				if s, ok := rw.Unparen(x.Fun).(*ast.SelectorExpr); ok {
					n = "." + s.Sel.Name
				}
			}
			if n == "obj_" || n == "any" {
				if o, ok := rw.Up(x).(*ast.CallExpr); ok {
					on, _ := rw.CallName(o)
					if on == "" {
						if s, ok := rw.Unparen(o.Fun).(*ast.SelectorExpr); ok {
							on = "." + s.Sel.Name
						}
					}
					n += " in " + on
				} else {
					n += fmt.Sprintf(" in a %T", rw.Up(x))
				}
			}
			bad("passed to the runtime (" + n + ")")
			return
		}
		bad("called")
		return
	case *ast.SelectorExpr:
		if x.X == top {
			if c, ok := rw.Parent[x].(*ast.CallExpr); ok && c.Fun == x {
				if v.kind == vSlot {
					if kind == "imm" {
						immUse(v.n, e, x.Sel.Name, c)
					} else if kind == "list" {
						use(v.n, e, x.Sel.Name, c)
					} else {
						mapUse(v.n, e, x.Sel.Name, c)
					}
				}
				return
			}
		}
		if x.Sel.Name == "items" && v.kind == vSlot {
			// l.items, the typed list's slice (a manual step reads it): the slice itself.
			edit(v.n, x, func() string { return rw.Render(e) })
			return
		}
		bad("method value or field of the list taken")
		return
	case *ast.BinaryExpr:
		if x.Op == token.EQL || x.Op == token.NEQ {
			other := x.X
			if other == top {
				other = x.Y
			}
			if id, ok := rw.Unparen(other).(*ast.Ident); ok && id.Name == "nil" {
				return
			}
		}
		bad("compared with another list (identity)")
		return
	case *ast.ExprStmt:
		return // a call whose list is dropped
	case *ast.KeyValueExpr:
		if x.Value == top {
			if id, ok := x.Key.(*ast.Ident); ok {
				if fv, ok := rw.Info.Uses[id].(*types.Var); ok && isList(fv.Type()) {
					flow(varNode(fv), e, false)
					return
				}
			}
		}
		bad("element of a composite literal")
		return
	}
	bad(fmt.Sprintf("used in a %T", rw.Parent[top]))
}

// ---- operations

func isStmt(call ast.Expr) bool { _, ok := rw.Up(call).(*ast.ExprStmt); return ok }

// use handles the call of method m on list expression recv, the value of slot n.
func use(n *node, recv ast.Expr, m string, call *ast.CallExpr) {
	R := func() string { return rw.Render(recv) }
	_, isVar := varOf(recv)
	addr := isVar != nil && rw.Pure(recv)
	args := call.Args
	A := func(i int) string { return rw.Render(args[i]) }
	stmt := isStmt(call)
	mut := func() bool {
		if n.mutated == "" {
			n.mutated = m
		}
		if !addr {
			poison(n, "changed through an expression that is not a variable")
			return false
		}
		return true
	}
	switch {
	case m == "size" && len(args) == 0:
		edit(n, call, func() string { return "int32(len(" + R() + "))" })
	case m == "isEmpty" && len(args) == 0:
		lenTest(n, call, R)
	case m == "get" && len(args) == 1:
		edit(n, call, func() string { return "sliceAt_(" + R() + ", " + A(0) + ")" })
	case m == "set" && len(args) == 2 && stmt:
		if mut() {
			edit(n, call, func() string { return R() + "[" + A(0) + "] = " + A(1) })
		}
	case m == "add" && len(args) == 1 && stmt:
		if mut() {
			edit(n, call, func() string { return R() + " = append(" + R() + ", " + A(0) + ")" })
		}
	case m == "addAllT" && len(args) == 1 && stmt:
		if mut() {
			edit(n, call, func() string { return R() + " = append(" + R() + ", " + A(0) + "...)" })
		}
	case m == "insert" && len(args) == 2 && stmt:
		if mut() {
			edit(n, call, func() string { return "sliceInsert_(&" + R() + ", " + A(0) + ", " + A(1) + ")" })
		}
	case m == "removeAt" && len(args) == 1:
		if mut() {
			edit(n, call, func() string { return "sliceRemoveAt_(&" + R() + ", " + A(0) + ")" })
		}
	case m == "clear" && len(args) == 0 && stmt:
		if mut() {
			edit(n, call, func() string { return "sliceClear_(&" + R() + ")" })
		}
	case (m == "trimToSize" && len(args) == 0 || m == "ensureCapacity" && len(args) == 1) && stmt && rw.Pure(recv):
		edit(n, rw.Up(call), func() string { return rw.Del })
	case m == "push_" && len(args) == 1 && stmt:
		// A deque used as a stack (colls -kind deque): its head is the end of the slice.
		if mut() {
			edit(n, call, func() string { return R() + " = append(" + R() + ", " + A(0) + ")" })
		}
	case (m == "pop_" || m == "pollTop_" || m == "popFirst_" || m == "pollFirst_") && len(args) == 0:
		tc := tcName(elemOf(typeOf(recv)))
		if tc == "" {
			poison(n, "no element conversion for "+m)
		} else if mut() {
			h := map[string]string{"pop_": "slicePop_", "pollTop_": "slicePollTop_", "popFirst_": "slicePopFirst_", "pollFirst_": "slicePollFirst_"}[m]
			edit(n, call, func() string { return h + "(" + tc + ", &" + R() + ")" })
		}
	case (m == "peekTop_" || m == "getTop_" || m == "peekFirst_" || m == "getFirst_") && len(args) == 0:
		tc := tcName(elemOf(typeOf(recv)))
		if tc == "" {
			poison(n, "no element conversion for "+m)
		} else {
			h := map[string]string{"peekTop_": "slicePeekTop_", "getTop_": "sliceTop_", "peekFirst_": "slicePeekFirst_", "getFirst_": "sliceFirst_"}[m]
			edit(n, call, func() string { return h + "(" + tc + ", " + R() + ")" })
		}
	case m == "eachRev" && len(args) == 0 && addr:
		// From the end of the slice, reading it anew at every step (as the typed list did).
		before := len(loops)
		loop(n, recv, call)
		loops = loops[:before] // not a forward loop: it stays a cursor
		if as, ok := rw.Parent[call].(*ast.AssignStmt); ok && n.local {
			if b := loopBody(as); b != nil {
				n.loops = append(n.loops, b)
			} else {
				poison(n, "for-each cursor not followed by its loop")
			}
		}
		edit(n, call, func() string { return "eachRevOf_(&" + R() + ")" })
	case m == "each" && len(args) == 0:
		loop(n, recv, call)
		if as, ok := rw.Parent[call].(*ast.AssignStmt); ok && n.local {
			if b := loopBody(as); b != nil {
				n.loops = append(n.loops, b)
			} else {
				poison(n, "for-each cursor not followed by its loop")
			}
		}
	case (m == "asList" || m == "asListT" || m == "asCollT" || m == "asIterable") && len(args) == 0: // reflip41a
		view(n, recv, call)
	default:
		what := "method " + m
		if !stmt && (m == "add" || m == "set" || m == "clear" || m == "insert" || m == "addAllT") {
			what += " used as a value"
		}
		poison(n, what)
	}
}

// Views. A list that code wanting a List reads during one call, and does not keep, gets a
// view of its own made for that call: a typed list over the same items (sliceView_). What a
// view changes in place (sort, set) reaches the slice; the consumers below never change the
// length. Operations that change the length through a view have slice helpers.

// readers take a collection, read it during the call and do not keep it.
var readers = map[string]bool{
	"ImmutableList_copyOf": true, "ImmutableList_copyOf_Iterable": true, "ImmutableSet_copyOf": true, "ImmutableSet_copyOf_Iterable": true,
	"Iterables_getOnlyElement": true, "Iterables_getFirst": true, "Iterables_getLast": true, "Iterables_size": true,
	"Collection_contains_s": true, "String_join": true, "new_ArrayList_Collection": true, "new_LinkedHashSet_Collection": true,
	"new_ArrayDeque_Collection": true, "tlistFrom_": true, "sliceFrom_": true, "tsetFrom_": true,
	"ImmutableList_copyOf_Collection": true, "ImmutableSet_copyOf_Collection": true,
}

// sorters change the list in place, keeping its length.
var sorters = map[string]bool{"List_sort": true, "Collections_sort": true, "Collections_sort_List_Comparator": true, "Collections_reverse": true}

// readMethods of the view itself, and readerMethods of an erased runtime collection that
// takes the view as an argument.
var readMethods = map[string]bool{"toArray": true, "toArray_0": true, "contains": true, "containsAll": true, "indexOf": true,
	"lastIndexOf": true, "subList": true, "get": true, "size": true, "isEmpty": true, "toString": true, "hashCode": true, "equals": true}
var readerMethods = map[string]bool{"addAll": true, "containsAll": true, "removeAll": true, "retainAll": true}

func tcName(t types.Type) string {
	text := types.TypeString(t, func(*types.Package) string { return "" })
	n := "tc_" + nonIdent.ReplaceAllString(strings.ReplaceAll(text, "*", "p_"), "_")
	if rw.Pkg.Scope().Lookup(n) == nil {
		return ""
	}
	return n
}

var nonIdent = regexp.MustCompile(`[^A-Za-z0-9_]`)

// erasedRuntime reports whether t is a collection interface or type of the runtime.
func erasedRuntime(t types.Type) bool {
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	nt, ok := t.(*types.Named)
	return ok && !generated(nt.Obj().Pos()) && nt.Obj().Pkg() == rw.Pkg
}

// view handles recv.asList(), a list of slot n seen as an erased List.
func view(n *node, recv ast.Expr, call *ast.CallExpr) {
	R := func() string { return rw.Render(recv) }
	tc := tcName(elemOf(typeOf(recv)))
	if tc == "" {
		poison(n, "no element conversion for a view")
		return
	}
	top := ast.Node(call)
	for {
		if c, ok := rw.Up(top).(*ast.CallExpr); ok && len(c.Args) == 1 && rw.Unparen(c.Args[0]) == top {
			if nm, _ := rw.CallName(c); nm == "Collection" || nm == "Iterable" || nm == "List" {
				top = c
				continue
			}
			// icast_Collection(any(l.asList())): the view seen as another collection interface.
			if nm, _ := rw.CallName(c); nm == "any" || nm == "obj_" {
				if c2, ok := rw.Up(c).(*ast.CallExpr); ok && len(c2.Args) == 1 && rw.Unparen(c2.Args[0]) == c {
					if nm2, _ := rw.CallName(c2); nm2 == "icast_Collection" || nm2 == "icast_Iterable" || nm2 == "icast_List" {
						top = c2
						continue
					}
				}
			}
		}
		break
	}
	viewCode := func() string { return "sliceView_(" + tc + ", " + R() + ")" }
	switch p := rw.Up(top).(type) {
	case *ast.SelectorExpr: // recv.asList().m(...)
		c, ok := rw.Up(p).(*ast.CallExpr)
		if !ok || c.Fun != p || top != call {
			break
		}
		_, isVar := varOf(recv)
		addr := isVar != nil && rw.Pure(recv)
		A := func() string { return rw.Render(c.Args[0]) }
		switch m := p.Sel.Name; {
		case m == "iterator" && len(c.Args) == 0 && iterSite(n, recv, c):
			return
		case readMethods[m]:
			edit(n, call, viewCode)
			return
		case (m == "addAll" || m == "remove_Object" || m == "removeAll") && len(c.Args) == 1 && addr:
			if n.mutated == "" {
				n.mutated = m
			}
			helper := map[string]string{"addAll": "sliceAddAll_", "remove_Object": "sliceRemoveObject_", "removeAll": "sliceRemoveAll_"}[m]
			edit(n, c, func() string { return helper + "(" + tc + ", &" + R() + ", " + A() + ")" })
			return
		}
		poison(n, "a view for ."+p.Sel.Name)
		return
	case *ast.CallExpr:
		nm, _ := rw.CallName(p)
		if (nm == "any" || nm == "obj_") && len(p.Args) == 1 && rw.ReadOnlySink(p, false) {
			// Boxed for one call that only reads it (equals, toString, a Preconditions message).
			edit(n, call, viewCode)
			return
		}
		if readers[nm] || sorters[nm] {
			if sorters[nm] && n.mutated == "" {
				n.mutated = nm
			}
			edit(n, call, viewCode)
			return
		}
		if s, ok := rw.Unparen(p.Fun).(*ast.SelectorExpr); ok && readerMethods[s.Sel.Name] && erasedRuntime(typeOf(s.X)) {
			edit(n, call, viewCode)
			return
		}
		if nm == "" {
			if s, ok := p.Fun.(*ast.SelectorExpr); ok {
				nm = "." + s.Sel.Name
			}
		}
		poison(n, "a view passed to "+nm)
		return
	case *ast.AssignStmt, *ast.ValueSpec:
		poison(n, "a view stored")
		return
	case *ast.ReturnStmt:
		poison(n, "a view returned")
		return
	}
	poison(n, fmt.Sprintf("a view in a %T", rw.Up(top)))
}

// iterSites are explicit iterators over lists (it := l.asList().iterator(), used only for
// hasNext and next): over a slice they become an index (an int cursor). The slice must not
// change while the iterator is in use (decided with the classes), so that Java's iterator
// could not have thrown ConcurrentModificationException either.
type iterSiteT struct {
	n    *node
	decl ast.Node // the statement declaring the iterator
	last ast.Node // the last use of the iterator
	rng  bool     // the loop became a range loop
}

var iterSites []iterSiteT

// iterSite handles call, recv.asList().iterator(), if its iterator is a local used only for
// hasNext() and next() in the function declaring it, and reports whether it did.
func iterSite(n *node, recv ast.Expr, call *ast.CallExpr) bool {
	var itID *ast.Ident
	var decl ast.Stmt
	switch d := rw.Up(call).(type) {
	case *ast.ValueSpec:
		if len(d.Names) != 1 || len(d.Values) != 1 {
			return false
		}
		gd, _ := rw.Parent[d].(*ast.GenDecl)
		if gd == nil || len(gd.Specs) != 1 {
			return false
		}
		ds, _ := rw.Parent[gd].(*ast.DeclStmt)
		if ds == nil {
			return false
		}
		itID, decl = d.Names[0], ds
	case *ast.AssignStmt:
		if d.Tok != token.DEFINE || len(d.Lhs) != 1 || len(d.Rhs) != 1 {
			return false
		}
		itID, _ = d.Lhs[0].(*ast.Ident)
		decl = d
	}
	if itID == nil {
		return false
	}
	it, _ := rw.Info.Defs[itID].(*types.Var)
	if it == nil {
		return false
	}
	fn := enclosingFn(decl)
	var hasNext, next []*ast.CallExpr
	for _, u := range rw.UsesOf[it] {
		s, ok := rw.Parent[u].(*ast.SelectorExpr)
		if !ok || enclosingFn(u) != fn {
			return false
		}
		c, ok := rw.Parent[s].(*ast.CallExpr)
		if !ok || c.Fun != s || len(c.Args) != 0 {
			return false
		}
		switch s.Sel.Name {
		case "hasNext":
			hasNext = append(hasNext, c)
		case "next":
			next = append(next, c)
		default:
			return false
		}
	}
	// recv is evaluated again at every hasNext and next: the same variable there.
	if !sameAt(recv, decl, rw.UsesOf[it]) {
		return false
	}
	elem := elemOf(typeOf(recv))
	cv := rw.ConvFor(elem)
	if !cv.OK {
		return false
	}
	var last ast.Node = decl
	if us := rw.UsesOf[it]; len(us) > 0 {
		last = us[len(us)-1]
	}
	iterSites = append(iterSites, iterSiteT{n: n, decl: decl, last: last})
	st := &iterSites[len(iterSites)-1]
	R := func() string { return rw.Render(recv) }
	name := itID.Name
	// The canonical loop: the declaration, then `for it.hasNext() { var x T = it.next(); ... }`
	// and no other use: a range loop.
	if fs, first := iterLoop(decl, hasNext, next, elem); fs != nil {
		vs := first.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec)
		x := vs.Names[0].Name
		edit(n, decl, func() string { return rw.Del })
		edit(n, first, func() string { return rw.Del })
		edit(n, fs.Cond, func() string { return "_, " + x + " := range " + R() })
		st.rng = true
		return true
	}
	if ds, ok := decl.(*ast.DeclStmt); ok {
		edit(n, ds, func() string { return "var " + name + " int" })
	} else {
		edit(n, call, func() string { return "0" })
	}
	for _, c := range hasNext {
		edit(n, c, func() string { return "(" + name + " < len(" + R() + "))" })
	}
	for _, c := range next {
		e := rw.Read(cv, c, func() string { return "sliceNext_(" + R() + ", &" + name + ")" })
		edit(n, e.Node, e.Gen)
	}
	return true
}

// iterLoop returns the loop `for it.hasNext() { var x T = unbox(it.next()); ... }` that
// directly follows the iterator's declaration, with its first statement, when those are the
// iterator's only uses and T is the element type.
func iterLoop(decl ast.Stmt, hasNext, next []*ast.CallExpr, elem types.Type) (*ast.ForStmt, *ast.DeclStmt) {
	if len(hasNext) != 1 || len(next) != 1 {
		return nil, nil
	}
	list := stmtList(rw.Parent[decl])
	var fs *ast.ForStmt
	for i, s := range list {
		if s == decl && i+1 < len(list) {
			fs, _ = list[i+1].(*ast.ForStmt)
		}
	}
	if fs == nil || fs.Init != nil || fs.Post != nil || rw.Unparen(fs.Cond) != hasNext[0] || len(fs.Body.List) == 0 {
		return nil, nil
	}
	ds, ok := fs.Body.List[0].(*ast.DeclStmt)
	if !ok {
		return nil, nil
	}
	gd, ok := ds.Decl.(*ast.GenDecl)
	if !ok || len(gd.Specs) != 1 {
		return nil, nil
	}
	vs, ok := gd.Specs[0].(*ast.ValueSpec)
	if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
		return nil, nil
	}
	x := rw.Info.Defs[vs.Names[0]].Type()
	if !types.Identical(x, elem) {
		return nil, nil
	}
	val := rw.Unparen(vs.Values[0])
	if c, ok := val.(*ast.CallExpr); ok && len(c.Args) == 1 && rw.Unparen(c.Args[0]) == next[0] {
		if nm, _ := rw.CallName(c); rw.ConvFor(x).IsUnbox(nm) {
			val = next[0] // unbox(it.next()): the element itself once unboxed
		}
	}
	if val != next[0] {
		return nil, nil
	}
	return fs, ds
}

// sameAt reports whether expression e (identifiers and fields from one) names the same
// variable at each of the uses as at decl.
func sameAt(e ast.Expr, decl ast.Node, uses []*ast.Ident) bool {
	root := e
	for {
		switch x := rw.Unparen(root).(type) {
		case *ast.SelectorExpr:
			root = x.X
			continue
		case *ast.Ident:
			o := rw.Info.Uses[x]
			if o == nil {
				return false
			}
			for _, u := range uses {
				if sc := rw.Pkg.Scope().Innermost(u.Pos()); sc == nil {
					return false
				} else if _, at := sc.LookupParent(x.Name, u.Pos()); at != o {
					return false
				}
			}
			return true
		}
		return false
	}
}

// loops are the for-each loops over lists, decided once the classes are.
type loopSite struct {
	n    *node
	recv ast.Expr
	call *ast.CallExpr
}

var loops []loopSite

func loop(n *node, recv ast.Expr, call *ast.CallExpr) {
	as, ok := rw.Parent[call].(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 {
		poison(n, "for-each cursor not declared with :=")
		return
	}
	it := rw.Info.Defs[as.Lhs[0].(*ast.Ident)]
	for _, u := range rw.UsesOf[it] {
		s, ok := rw.Parent[u].(*ast.SelectorExpr)
		if !ok || (s.Sel.Name != "v" && s.Sel.Name != "next") {
			poison(n, "for-each cursor used for more than next and v")
			return
		}
	}
	loops = append(loops, loopSite{n, recv, call})
}

// rangeLoop rewrites `it := X.each(); for it.next() { ... it.v ... }` (or the same over
// eachOf_(&X)) into `for _, it := range X { ... it ... }`, folding a first statement
// `var x T = it.v` into the loop variable. It reports false when the shape is not that.
func rangeLoop(as *ast.AssignStmt, x ast.Expr) bool { return rangeLoopConv(as, x, nil, nil, true) }

// rangeLoopConv is rangeLoop over a loop whose cursor yields boxed elements (cv non-nil, the
// slice's elements are elem): reads are unboxed where the code unboxes them. With apply
// false it only checks the shape.
func rangeLoopConv(as *ast.AssignStmt, x ast.Expr, cv *rw.Conv, elem types.Type, apply bool) bool {
	list := stmtList(rw.Parent[as])
	var next ast.Stmt
	for i, s := range list {
		if s == as && i+1 < len(list) {
			next = list[i+1]
		}
	}
	if l, ok := next.(*ast.LabeledStmt); ok {
		next = l.Stmt
	}
	fs, ok := next.(*ast.ForStmt)
	if !ok || fs.Init != nil || fs.Post != nil || fs.Cond == nil {
		return false
	}
	itID := as.Lhs[0].(*ast.Ident)
	it := rw.Info.Defs[itID]
	cond, ok := fs.Cond.(*ast.CallExpr)
	if !ok {
		return false
	}
	if s, ok := cond.Fun.(*ast.SelectorExpr); !ok || s.Sel.Name != "next" || rw.Info.Uses[identOf(s.X)] != it {
		return false
	}
	var reads []*ast.SelectorExpr
	for _, u := range rw.UsesOf[it] {
		s, ok := rw.Parent[u].(*ast.SelectorExpr)
		if !ok || s.Sel.Name != "next" && s.Sel.Name != "v" {
			return false
		}
		if s.Sel.Name == "next" {
			if rw.Parent[s] != cond {
				return false
			}
			continue
		}
		if !inside(s, fs.Body) {
			return false
		}
		reads = append(reads, s)
	}
	if !apply {
		return true
	}
	name := itID.Name
	// var x T = it.v as the first statement, T the element type: x is the loop variable.
	if len(reads) == 1 && len(fs.Body.List) > 0 {
		if ds, ok := fs.Body.List[0].(*ast.DeclStmt); ok {
			if gd, ok := ds.Decl.(*ast.GenDecl); ok && len(gd.Specs) == 1 {
				if vs, ok := gd.Specs[0].(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 1 {
					val, et := rw.Unparen(vs.Values[0]), typeOf(reads[0])
					if cv != nil {
						// var x T = unbox(it.v)
						if c, ok := val.(*ast.CallExpr); ok && len(c.Args) == 1 && rw.Unparen(c.Args[0]) == reads[0] {
							if nm, _ := rw.CallName(c); cv.IsUnbox(nm) {
								val, et = reads[0], elem
							}
						}
					}
					if val == reads[0] && types.Identical(rw.Info.Defs[vs.Names[0]].Type(), et) {
						name = vs.Names[0].Name
						rw.Rewrite[ds] = func() string { return rw.Del }
						reads = nil
					}
				}
			}
		}
	}
	rw.Rewrite[as] = func() string { return rw.Del }
	header := "_, " + name + " := range " + rw.Render(x)
	if name == itID.Name && len(reads) == 0 {
		header = "range " + rw.Render(x)
	}
	rw.Rewrite[cond] = func() string { return header }
	for _, s := range reads {
		if cv != nil {
			e := rw.Read(*cv, s, func() string { return name })
			rw.Rewrite[e.Node] = e.Gen
			continue
		}
		rw.Rewrite[s] = func() string { return name }
	}
	return true
}

func identOf(e ast.Expr) *ast.Ident { id, _ := rw.Unparen(e).(*ast.Ident); return id }

var changing = map[string]bool{"add": true, "set": true, "addAllT": true, "insert": true, "removeAt": true, "clear": true}

// changedIn reports whether list variable v is changed, assigned, or has its address taken
// inside n.
func changedIn(v *types.Var, n ast.Node) bool {
	for _, u := range rw.UsesOf[v] {
		if !inside(u, n) {
			continue
		}
		switch p := rw.Parent[u].(type) {
		case *ast.SelectorExpr:
			if changing[p.Sel.Name] {
				return true
			}
		case *ast.AssignStmt:
			for _, l := range p.Lhs {
				if l == u {
					return true
				}
			}
		case *ast.UnaryExpr:
			if p.Op == token.AND {
				return true
			}
		}
	}
	return false
}

// stmtList is the statement list of a block or case clause.
func stmtList(n ast.Node) []ast.Stmt {
	switch b := n.(type) {
	case *ast.BlockStmt:
		return b.List
	case *ast.CaseClause:
		return b.Body
	case *ast.CommClause:
		return b.Body
	}
	return nil
}

// loopBody is the body of the loop that cursor assignment as drives, or nil.
func loopBody(as *ast.AssignStmt) *ast.BlockStmt {
	list := stmtList(rw.Parent[as])
	for i, s := range list {
		if s == as && i+1 < len(list) {
			next := list[i+1]
			if l, ok := next.(*ast.LabeledStmt); ok {
				next = l.Stmt
			}
			if fs, ok := next.(*ast.ForStmt); ok {
				return fs.Body
			}
		}
	}
	return nil
}

// ---- main

func main() {
	report, verbose := false, false
	for i := 1; i < len(os.Args)-1; i++ {
		switch os.Args[i] {
		case "-n":
			report = true
		case "-v":
			verbose = true
		case "-kind":
			i++
			kind = os.Args[i]
		}
	}
	if runtimeType[kind] == "" {
		fmt.Fprintln(os.Stderr, "usage: native -kind list|map|set [-n] [-v] <dir>")
		os.Exit(2)
	}
	rw.Load(os.Args[len(os.Args)-1])
	immType = types.NewPointer(rw.Pkg.Scope().Lookup("ImmutableCollection").Type())

	// Slots: function signatures, interface methods, fields and variables.
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				curPos = x.Pos()
				if o, ok := rw.Info.Defs[x.Name].(*types.Func); ok {
					declare(groupOf(o), x.Type, !generated(x.Pos()), "signature of the runtime")
				}
			case *ast.InterfaceType:
				for _, m := range x.Methods.List {
					ft, ok := m.Type.(*ast.FuncType)
					if !ok || len(m.Names) != 1 {
						continue
					}
					curPos = m.Pos()
					if o, ok := rw.Info.Defs[m.Names[0]].(*types.Func); ok {
						declare(groupOf(o), ft, !generated(x.Pos()), "method of a runtime interface")
					}
				}
			case *ast.FuncLit:
				declare(litGroup(x), x.Type, !generated(x.Pos()), "function literal of the runtime")
			case *ast.Field:
				if _, inStruct := rw.Parent[rw.Parent[x]].(*ast.StructType); inStruct && isList(typeOf(x.Type)) {
					for _, id := range x.Names {
						if v, ok := rw.Info.Defs[id].(*types.Var); ok {
							k := varNode(v)
							if generated(x.Pos()) {
								declareType(k, x.Type)
							}
						}
					}
					if len(x.Names) == 0 {
						poison(get(fmt.Sprintf("embedded|%d", x.Pos())), "embedded")
					}
				}
			case *ast.ValueSpec:
				if x.Type != nil && isList(typeOf(x.Type)) {
					for _, id := range x.Names {
						if v, ok := rw.Info.Defs[id].(*types.Var); ok {
							k := varNode(v)
							if generated(x.Pos()) {
								declareType(k, x.Type)
							}
						}
					}
				}
			}
			return true
		})
	}
	// Function literals and methods used as values may be called by anything.
	poisonGroup := func(g, why string) {
		for _, k := range []string{":p", ":r"} {
			for i := 0; i < 12; i++ {
				if n, ok := nodes[fmt.Sprintf("%s%s%d", g, k, i)]; ok {
					poison(n, why)
				}
			}
		}
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if fl, ok := n.(*ast.FuncLit); ok {
				if c, ok := rw.Parent[fl].(*ast.CallExpr); !ok || c.Fun != fl {
					curPos = fl.Pos()
					poisonGroup(litGroup(fl), "function literal used as a value")
				}
			}
			if s, ok := n.(*ast.SelectorExpr); ok {
				if o, ok := rw.Info.Uses[s.Sel].(*types.Func); ok {
					if c, ok := rw.Parent[s].(*ast.CallExpr); !ok || c.Fun != s {
						curPos = s.Pos()
						poisonGroup(groupOf(o), "method used as a function value")
					}
				}
			}
			if id, ok := n.(*ast.Ident); ok {
				if o, ok := rw.Info.Uses[id].(*types.Func); ok && o.Type().(*types.Signature).Recv() == nil {
					if c, ok := rw.Parent[id].(*ast.CallExpr); !ok || c.Fun != id {
						if _, isSel := rw.Parent[id].(*ast.SelectorExpr); !isSel {
							if _, isIdx := rw.Parent[id].(*ast.IndexExpr); !isIdx {
								curPos = id.Pos()
								poisonGroup(groupOf(o), "function used as a value")
							}
						}
					}
				}
			}
			return true
		})
	}
	// What the runtime calls cannot change.
	for _, f := range rw.Files {
		if generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn := calleeFunc(c)
			if fn == nil || !generated(fn.Pos()) {
				return true
			}
			curPos = c.Pos()
			poisonGroup(groupOf(fn), "called by the runtime")
			return true
		})
	}
	// Every list-valued expression in generated code.
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			switch e.(type) {
			case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr, *ast.TypeAssertExpr, *ast.StarExpr:
			default:
				return true
			}
			if !isList(typeOf(e)) {
				return true
			}
			if tv := rw.Info.Types[e]; tv.IsType() {
				return true
			}
			if s, ok := rw.Parent[e].(*ast.SelectorExpr); ok && s.Sel == e {
				return true
			}
			curPos = e.Pos()
			if id, ok := e.(*ast.Ident); ok {
				if _, isDef := rw.Info.Defs[id]; isDef {
					return true // x := v: handled as the value v
				}
			}
			handle(e)
			return true
		})
	}

	if kind == "imm" {
		decideImm()
	}
	// Decide: uniqueness (greatest fixpoint), then the conditions on changed slots.
	var all []*node
	for _, n := range nodes {
		all = append(all, n)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].key < all[j].key })
	term := map[*node]bool{}
	for _, n := range all {
		term[n] = n.terminal()
		n.unique = true
	}
	move := func(e *edge) bool {
		if e.from == nil {
			return !e.imm
		}
		if e.from.isRes || e.from.v == nil {
			return e.from.unique // a call's value, or an argument delivered to its parameter
		}
		return term[e.from] && e.from.unique
	}
	for changed := true; changed; {
		changed = false
		for _, n := range all {
			if !n.unique {
				continue
			}
			for _, e := range n.in {
				if !move(e) {
					n.unique = false
					changed = true
					break
				}
			}
		}
	}
	// A Go map is a reference, as the typed map was: only an immutable one must stay one.
	for _, n := range all {
		if kind == "list" || n.mutated == "" {
			continue
		}
		for _, m := range cls(n).nodes {
			for _, e := range m.in {
				if e.imm {
					poison(n, "changed ("+n.mutated+") and holds an immutable "+kind)
				}
			}
		}
	}
	for _, n := range all {
		if kind != "list" || n.mutated == "" {
			continue
		}
		if n.v != nil {
			curPos = n.v.Pos()
		}
		if !n.unique {
			poison(n, "changed ("+n.mutated+") while another slot may hold the same list")
			if os.Getenv("NATIVE_WHY") != "" {
				fmt.Fprintf(os.Stderr, "why: %s %s not unique\n", n.key, where(n))
				for _, e := range n.in {
					if !move(e) && e.from != nil {
						fmt.Fprintf(os.Stderr, "why:    from %s %s\n", e.from.key, where(e.from))
					}
				}
			}
			continue
		}
		if n.v != nil && !n.isRes && len(n.out) > 0 && !term[n] {
			poison(n, "changed ("+n.mutated+"), and passed on while still in use")
			if os.Getenv("NATIVE_WHY") != "" {
				fmt.Fprintf(os.Stderr, "why: %s %s passed on\n", n.key, where(n))
			}
		}
	}

	// Explicit iterators: the list does not change while one is in use.
	for _, st := range iterSites {
		n := st.n
		if n.mutated == "" {
			continue
		}
		if n.local && n.v != nil && !capturedAnywhere(n) && !changedIn(n.v, rw.Parent[st.decl]) {
			continue
		}
		curPos = st.decl.Pos()
		poison(n, "iterated explicitly, and changed ("+n.mutated+")")
	}
	reasons := map[string]int{}
	var good []*class
	nGood := 0
	conv := map[*class]bool{}
	for r, c := range data {
		if find(r) != r || len(c.typeExpr) == 0 {
			continue
		}
		if kind == "map" || kind == "set" {
			// Keys of a class with its own equals and hashCode stay in the typed map, which
			// compares them as Java does; a Go map would compare them by ==.
			for _, te := range c.typeExpr {
				if as := typeArgs(typeOf(te)); as != nil && rw.KeyMode(as[0]) != rw.KeyEq {
					c.poison = append([]string{"keys compared by equals"}, c.poison...)
					break
				}
			}
		}
		if len(c.poison) > 0 {
			reasons[c.poison[0]] += len(c.typeExpr)
			continue
		}
		good = append(good, c)
		conv[c] = true
		nGood += len(c.typeExpr)
	}
	total := nGood
	for _, n := range reasons {
		total += n
	}
	fmt.Fprintf(os.Stderr, "%s declarations: %d; become Go %ss: %d (%d classes)\n", map[string]string{"list": "typed list", "map": "typed map", "set": "typed set", "imm": "ImmutableCollection"}[kind], total, map[string]string{"list": "slice", "map": "map", "set": "map", "imm": "slice"}[kind], nGood, len(good))
	var rs []string
	for r := range reasons {
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool {
		return reasons[rs[i]] > reasons[rs[j]] || reasons[rs[i]] == reasons[rs[j]] && rs[i] < rs[j]
	})
	for i, r := range rs {
		if i >= 30 {
			break
		}
		fmt.Fprintf(os.Stderr, "  stays (%d declarations): %s  [%s]\n", reasons[r], r, poisonAt[r])
	}
	sort.Slice(good, func(i, j int) bool { return good[i].typeExpr[0].Pos() < good[j].typeExpr[0].Pos() })
	if verbose {
		for _, c := range good {
			p := rw.Fset.Position(c.typeExpr[0].Pos())
			fmt.Printf("class\t%d decls\t%s:%d\t%s\n", len(c.typeExpr), filepath.Base(p.Filename), p.Line, rw.Text(c.typeExpr[0]))
		}
	}
	if report {
		return
	}
	for _, c := range good {
		for _, te := range c.typeExpr {
			te := te
			c := c
			rw.Rewrite[te] = func() string {
				if kind == "imm" {
					return "[]" + c.elemText
				}
				return goType(typeOf(te))
			}
		}
		for _, e := range c.edits {
			if _, dup := rw.Rewrite[e.Node]; !dup {
				rw.Rewrite[e.Node] = e.Gen
			}
		}
	}
	// Loops over converted lists.
	nRange, nEach := 0, 0
	for _, l := range loops {
		c := cls(l.n)
		if !conv[c] {
			continue
		}
		as := rw.Parent[l.call].(*ast.AssignStmt)
		exact := true // the loop cannot change the slice's length
		if l.n.mutated != "" {
			exact = false
			if l.n.local {
				if body := loopBody(as); body != nil && !changedIn(l.n.v, body) && !capturedAnywhere(l.n) {
					exact = true
				}
			}
		}
		if exact && rangeLoop(as, l.recv) {
			nRange++
			continue
		}
		if v, _ := varOf(l.recv); v == nil {
			// Only a variable or field is changed through, so a call's value is never.
			fmt.Fprintf(os.Stderr, "loop over a changed list that is not a variable: %s\n", rw.Fset.Position(l.call.Pos()))
			os.Exit(1)
		}
		recv := l.recv
		rw.Rewrite[l.call] = func() string { return "eachOf_(&" + rw.Render(recv) + ")" }
		pendingField = append(pendingField, fieldSite{as, recv})
		nEach++
	}
	// ImmutableList reads and loops, unboxed where the code unboxes the element.
	for _, r := range immReads {
		if c := cls(r.n); conv[c] {
			e := rw.Read(c.cv, r.call, r.gen)
			rw.Rewrite[e.Node] = e.Gen
		}
	}
	for _, l := range immLoops {
		if c := cls(l.n); conv[c] {
			rangeLoopConv(l.as, l.x, &c.cv, c.elem, true)
			nRange++
		}
	}
	// Step 1's eachOf_ loops over local slices that the loop does not assign.
	n1 := 0
	if kind == "imm" {
		fmt.Fprintf(os.Stderr, "for-each loops: %d range\n", nRange)
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
		writeConvs(os.Args[len(os.Args)-1])
		return
	}
	if kind != "list" {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
		return
	}
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				return true
			}
			if _, done := rw.Rewrite[as]; done {
				return true
			}
			name, call := rw.CallName(as.Rhs[0])
			if name != "eachOf_" || len(call.Args) != 1 {
				return true
			}
			u, ok := call.Args[0].(*ast.UnaryExpr)
			if !ok || u.Op != token.AND {
				return true
			}
			id, ok := u.X.(*ast.Ident)
			if !ok {
				return true
			}
			v, ok := rw.Info.Uses[id].(*types.Var)
			if !ok || v.IsField() || v.Parent() == rw.Pkg.Scope() {
				return true
			}
			body := loopBody(as)
			if body == nil || funcAt(v.Pos()) != enclosingFn(as) {
				return true
			}
			for _, x := range rw.UsesOf[v] {
				if enclosingFn(x) != enclosingFn(as) {
					return true // captured by a function literal
				}
			}
			// The cursor's own &v is the one address taken; assignments in the body change it.
			for _, x := range rw.UsesOf[v] {
				if x == id || !inside(x, body) {
					continue
				}
				if p, ok := rw.Parent[x].(*ast.AssignStmt); ok {
					for _, l := range p.Lhs {
						if l == x {
							return true
						}
					}
				}
				if p, ok := rw.Parent[x].(*ast.UnaryExpr); ok && p.Op == token.AND {
					return true
				}
			}
			if rangeLoop(as, id) {
				n1++
			}
			return true
		})
	}
	nf := fieldLoops()
	fmt.Fprintf(os.Stderr, "for-each loops: %d range, %d eachOf_; step 1 loops to range: %d; eachOf_ loops over fields to range: %d\n", nRange, nEach, n1, nf)
	nIterRange, nIterIndex := 0, 0
	for _, st := range iterSites {
		if conv[cls(st.n)] {
			if st.rng {
				nIterRange++
			} else {
				nIterIndex++
			}
		}
	}
	fmt.Fprintf(os.Stderr, "explicit iterators over slices: %d range loops, %d index cursors\n", nIterRange, nIterIndex)
	fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
}

// capturedAnywhere reports whether local n is used from a function literal.
func capturedAnywhere(n *node) bool {
	for _, u := range rw.UsesOf[n.v] {
		if enclosingFn(u) != n.fn {
			return true
		}
	}
	return false
}

func where(n *node) string {
	if n.v == nil {
		return ""
	}
	p := rw.Fset.Position(n.v.Pos())
	return fmt.Sprintf("%s:%d %s", filepath.Base(p.Filename), p.Line, n.v.Name())
}

// mapUse handles the call of method m on map or set expression recv, the value of slot n.
// Nothing may iterate the map (its order is the insertion order, a Go map's is not) or see it
// as an erased Map or Set.
func mapUse(n *node, recv ast.Expr, m string, call *ast.CallExpr) {
	R := func() string { return rw.Render(recv) }
	args := call.Args
	A := func(i int) string { return rw.Render(args[i]) }
	stmt := isStmt(call)
	as := typeArgs(typeOf(recv))
	set := kind == "set"
	var vc string
	if !set {
		if vc = tcName(as[1]); vc == "" {
			poison(n, "no conversion for the value type")
			return
		}
	}
	kc := tcName(as[0])
	changes := func() {
		if n.mutated == "" {
			n.mutated = m
		}
	}
	strVal := !set && isString(as[1])
	switch {
	case m == "size" && len(args) == 0:
		edit(n, call, func() string { return "int32(len(" + R() + "))" })
	case m == "isEmpty" && len(args) == 0:
		lenTest(n, call, R)
	case m == "clear" && len(args) == 0 && stmt:
		changes()
		edit(n, call, func() string { return "clear(" + R() + ")" })
	case !set && m == "get" && len(args) == 1:
		edit(n, call, func() string {
			if strVal {
				return "mapStr_(" + R() + ", " + A(0) + ")"
			}
			return R() + "[" + A(0) + "]"
		})
	case !set && m == "containsKey" && len(args) == 1, set && m == "contains" && len(args) == 1:
		edit(n, call, func() string { return "has_(" + R() + ", " + A(0) + ")" })
	case !set && m == "put" && len(args) == 2:
		changes()
		if stmt {
			edit(n, call, func() string { return R() + "[" + A(0) + "] = " + A(1) })
		} else {
			edit(n, call, func() string { return "mapPut_(" + vc + ", " + R() + ", " + A(0) + ", " + A(1) + ")" })
		}
	case !set && m == "remove" && len(args) == 1:
		changes()
		if stmt {
			edit(n, call, func() string { return "delete(" + R() + ", " + A(0) + ")" })
		} else {
			edit(n, call, func() string { return "mapRemove_(" + vc + ", " + R() + ", " + A(0) + ")" })
		}
	case !set && m == "getOrDefault" && len(args) == 2:
		edit(n, call, func() string { return "mapGetOr_(" + R() + ", " + A(0) + ", " + A(1) + ")" })
	case !set && m == "putIfAbsent" && len(args) == 2:
		changes()
		edit(n, call, func() string { return "mapPutIfAbsent_(" + vc + ", " + R() + ", " + A(0) + ", " + A(1) + ")" })
	case !set && m == "computeIfAbsent" && len(args) == 2 && kc != "":
		changes()
		edit(n, call, func() string {
			return "mapComputeIfAbsent_(" + kc + ", " + vc + ", " + R() + ", " + A(0) + ", " + A(1) + ")"
		})
	case set && m == "add" && len(args) == 1:
		changes()
		if stmt {
			edit(n, call, func() string { return R() + "[" + A(0) + "] = struct{}{}" })
		} else {
			edit(n, call, func() string { return "setAdd_(" + R() + ", " + A(0) + ")" })
		}
	case set && m == "remove" && len(args) == 1:
		changes()
		if stmt {
			edit(n, call, func() string { return "delete(" + R() + ", " + A(0) + ")" })
		} else {
			edit(n, call, func() string { return "setRemove_(" + R() + ", " + A(0) + ")" })
		}
	case set && m == "addAllT" && len(args) == 1:
		// Adding a set to a set does not depend on the order of either.
		changes()
		edit(n, call, func() string { return "setAddAll_(" + R() + ", " + A(0) + ")" })
	case m == "each":
		poison(n, "iterated (insertion order)")
	case m == "asMap" && len(args) == 0 && kc != "":
		// Map_computeIfAbsent(m.asMap(), k, f), with a key that is boxed.
		if p, ok := rw.Up(call).(*ast.CallExpr); ok && len(p.Args) == 3 && rw.Unparen(p.Args[0]) == call {
			if nm, _ := rw.CallName(p); nm == "Map_computeIfAbsent" {
				changes()
				edit(n, p, func() string {
					return "mapComputeIfAbsentObj_(" + kc + ", " + vc + ", " + R() + ", " + rw.Render(p.Args[1]) + ", " + rw.Render(p.Args[2]) + ")"
				})
				return
			}
		}
		poison(n, "seen as an erased Map")
	case (m == "asSet" || m == "asSetT" || m == "asCollT") && len(args) == 0 && kc != "": // reflip41c
		// s.asSet().addAll(c): adding to a set does not depend on the order of c.
		if p, ok := rw.Up(call).(*ast.SelectorExpr); ok && p.Sel.Name == "addAll" {
			if c, ok := rw.Up(p).(*ast.CallExpr); ok && c.Fun == p && len(c.Args) == 1 {
				changes()
				edit(n, c, func() string { return "setAddAllFrom_(" + kc + ", " + R() + ", " + rw.Render(c.Args[0]) + ")" })
				return
			}
		}
		poison(n, "seen as an erased Set")
	case m == "asMap" || m == "asSet":
		poison(n, "seen as an erased "+map[bool]string{true: "Set", false: "Map"}[set])
	default:
		poison(n, "method "+m)
	}
}

func isString(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Kind() == types.String
}

// lenTest rewrites an isEmpty() call to a length test.
func lenTest(n *node, call *ast.CallExpr, R func() string) {
	at := ast.Node(call)
	gen := func() string { return "len(" + R() + ") == 0" }
	if u, ok := rw.Up(call).(*ast.UnaryExpr); ok && u.Op == token.NOT {
		at = u
		gen = func() string { return "len(" + R() + ") != 0" }
	}
	g := gen
	switch p := rw.Up(at).(type) {
	case *ast.BinaryExpr:
		if p.Op != token.LAND && p.Op != token.LOR {
			g = func() string { return "(" + gen() + ")" }
		}
	case *ast.UnaryExpr, *ast.SelectorExpr:
		g = func() string { return "(" + gen() + ")" }
	}
	edit(n, at, g)
}

// ---- ImmutableList (kind imm)

type immLoop struct {
	n  *node
	as *ast.AssignStmt
	x  ast.Expr
}

var immLoops []immLoop

// immUse handles the call of method m on ImmutableList expression recv, the value of slot n.
func immUse(n *node, recv ast.Expr, m string, call *ast.CallExpr) {
	R := func() string { return rw.Render(recv) }
	args := call.Args
	switch {
	case m == "size" && len(args) == 0:
		edit(n, call, func() string { return "int32(len(" + R() + "))" })
	case m == "isEmpty" && len(args) == 0:
		lenTest(n, call, R)
	case m == "contains" && len(args) == 1:
		edit(n, call, func() string { return "sliceContains_(" + cls(n).tc + ", " + R() + ", " + rw.Render(args[0]) + ")" })
	case m == "get" && len(args) == 1:
		immReads = append(immReads, immRead{n, call, func() string { return "sliceAt_(" + R() + ", " + rw.Render(args[0]) + ")" }})
	default:
		poison(n, "method "+m)
	}
}

type immRead struct {
	n    *node
	call *ast.CallExpr
	gen  func() string
}

var immReads []immRead

// decideImm sets the element type of each ImmutableList class from its declarations.
func decideImm() {
	for r, c := range data {
		if find(r) != r || len(c.typeExpr) == 0 || len(c.poison) > 0 {
			continue
		}
		for _, te := range c.typeExpr {
			a := rw.AliasArgs(te, "ImmutableListOf")
			if a == nil {
				poison(r, "declared as another immutable collection ("+rw.Text(te)+")")
				break
			}
			if rw.AliasArgs(a[0], "BoxedOf") != nil {
				poison(r, "element is a boxed primitive")
				break
			}
			t := typeOf(a[0])
			if c.elem == nil {
				c.elem, c.elemText = t, rw.Text(a[0])
			} else if !types.Identical(t, c.elem) {
				poison(r, "declarations disagree on the element type")
				break
			}
		}
		if len(c.poison) > 0 {
			continue
		}
		c.cv = rw.ConvFor(c.elem)
		if !c.cv.OK {
			poison(r, "element type has no known boxing")
			continue
		}
		c.tc = immConv(c)
	}
}

var immConvs = map[string]string{}

// immConv is the conversion variable of the class's element type, declared by an earlier
// step or written to g_zconv_imm.go.
func immConv(c *class) string {
	text := tstr(c.elem)
	n := "tc_" + nonIdent.ReplaceAllString(strings.ReplaceAll(text, "*", "p_"), "_")
	if rw.Pkg.Scope().Lookup(n) != nil {
		return n
	}
	if _, ok := immConvs[n]; !ok {
		null := ""
		if isString(c.elem) {
			null = ", null: nullstr_"
		}
		if _, isFn := c.elem.Underlying().(*types.Signature); isFn { // reflip28: a function type is compared with nil only
			null = fmt.Sprintf(", isnull: func(v %s) bool { return v == nil }", c.elemText)
		}
		immConvs[n] = fmt.Sprintf("var %s = &tconv_[%s]{box: func(v %s) any { return %s }, unbox: func(v any) %s { return %s }%s}",
			n, c.elemText, c.elemText, c.cv.Box("v"), c.elemText, c.cv.Unbox("v"), null)
	}
	return n
}

// writeConvs writes the conversion variables of g_zconv_imm.go: those it holds and the new ones.
func writeConvs(dir string) {
	path := filepath.Join(dir, "g_zconv_imm.go")
	if b, err := os.ReadFile(path); err == nil {
		// One declaration per var line, with the lines gofmt wrapped it onto.
		cur := ""
		for _, l := range strings.Split(string(b), "\n") {
			switch {
			case strings.HasPrefix(l, "var tc_"):
				cur = strings.Fields(l)[1]
				immConvs[cur] = l
			case cur != "" && l != "" && (l[0] == '\t' || l[0] == '}'):
				immConvs[cur] += "\n" + l
			default:
				cur = ""
			}
		}
	}
	if len(immConvs) == 0 {
		return
	}
	var names []string
	for n := range immConvs {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	sb.WriteString("// Code generated by tools/postflip/native. DO NOT EDIT.\n\npackage closure\n\n// Element conversions of the ImmutableLists that are slices.\n\n")
	for _, n := range names {
		sb.WriteString(immConvs[n] + "\n")
	}
	if err := os.WriteFile(path, []byte(sb.String()), 0o644); err != nil {
		panic(err)
	}
}

// immReaders read an immutable list during the call and do not keep it; they get a view of
// the slice (sliceView_), a new object: an ImmutableList's identity is not observable there.
var immReaders = map[string]bool{
	"tlistImm_": true, "tlistFrom_": true, "tsetFrom_": true, "setImm_": true, "setFrom_": true, "sliceFrom_": true, "toSlice_": true,
	"ImmutableList_copyOf": true, "ImmutableList_copyOf_Iterable": true, "ImmutableSet_copyOf": true, "ImmutableSet_copyOf_Iterable": true,
	"Iterables_getOnlyElement": true, "Iterables_getFirst": true, "Iterables_getLast": true, "Iterables_size": true,
	"String_join": true, "new_ArrayList_Collection": true, "new_LinkedHashSet_Collection": true, "new_ArrayDeque_Collection": true,
	"Collection_contains_s": true,
}

// immBoundary handles slot n's list e as an argument of runtime call x (or of a conversion
// in x): a reader gets a view, Collection_contains_s on strings a slice search.
func immBoundary(n *node, e ast.Expr, x *ast.CallExpr) bool {
	// Climb the conversions to an Object, Collection or Iterable.
	arg := ast.Expr(e)
	call := x
	for {
		nm, _ := rw.CallName(call)
		if !immWrappers[nm] || len(call.Args) != 1 {
			break
		}
		outer, ok := rw.Up(call).(*ast.CallExpr)
		if !ok {
			return false
		}
		arg, call = call, outer
	}
	nm, _ := rw.CallName(call)
	inner := func() string {
		if arg == e {
			return rw.RenderChildren(e) // the view replaces e itself
		}
		return rw.Render(e)
	}
	view := func() string { return "sliceView_(" + cls(n).tc + ", " + inner() + ")" }
	switch {
	case nm == "Collection_contains_s" && len(call.Args) == 2 && rw.Unparen(call.Args[0]) == arg:
		edit(n, call, func() string {
			if isString(cls(n).elem) {
				return "sliceHasStr_(" + rw.Render(e) + ", " + rw.Render(call.Args[1]) + ")"
			}
			return "Collection_contains_s(" + view() + ", " + rw.Render(call.Args[1]) + ")"
		})
	case immReaders[nm], nm == "objstr_":
		edit(n, arg, view)
	case nullChecks[nm] && isStmt(call) && rw.Unparen(call.Args[0]) == arg:
		// a null check whose result is dropped: the slice's null, without a view
		edit(n, arg, func() string { return "sliceObj_(" + inner() + ")" })
	default:
		s, ok := rw.Unparen(call.Fun).(*ast.SelectorExpr)
		if !ok || !readerMethods[s.Sel.Name] || !erasedRuntime(typeOf(s.X)) {
			return false
		}
		edit(n, arg, view)
	}
	return true
}

var immWrappers = map[string]bool{"obj_": true, "any": true, "Collection": true, "Iterable": true, "List": true,
	"icast_Iterable": true, "icast_Collection": true, "icast_List": true}

var nullChecks = map[string]bool{"Objects_requireNonNull": true, "Objects_requireNonNull_Object": true, "Preconditions_checkNotNull": true}

// ---- eachOf_ loops over fields

// fieldLoops rewrites `it := eachOf_(&x.f); for it.next() { ... }` into a range loop when
// nothing the loop body does can change the length of any slice in field f: the body does
// not assign f or take its address, and no function it may call does (calls through an
// interface reach every method of that name; a call of a function value reaches anything,
// so such a body keeps eachOf_). Then the slice the loop walks is the one it started with,
// as a range loop reads it. It returns the number of loops rewritten.
type fieldSite struct {
	as *ast.AssignStmt
	x  ast.Expr
}

// pendingField are the loops this run makes eachOf_ loops (not yet in the source).
var pendingField []fieldSite

func fieldLoops() int {
	type site struct {
		as *ast.AssignStmt
		x  ast.Expr
		f  *types.Var
	}
	var sites []site
	fields := map[*types.Var]int{}
	for _, p := range pendingField {
		sel, ok := rw.Unparen(p.x).(*ast.SelectorExpr)
		if !ok || !rw.Pure(sel) || loopBody(p.as) == nil {
			continue
		}
		if v, _ := varOf(sel); v != nil && v.IsField() {
			if _, ok := fields[v]; !ok {
				fields[v] = len(fields)
			}
			sites = append(sites, site{p.as, sel, v})
		}
	}
	for _, file := range rw.Files {
		if !generated(file.Pos()) {
			continue
		}
		ast.Inspect(file, func(n ast.Node) bool {
			as, ok := n.(*ast.AssignStmt)
			if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				return true
			}
			if _, done := rw.Rewrite[as]; done {
				return true
			}
			name, call := rw.CallName(as.Rhs[0])
			if name != "eachOf_" || len(call.Args) != 1 {
				return true
			}
			u, ok := call.Args[0].(*ast.UnaryExpr)
			if !ok || u.Op != token.AND {
				return true
			}
			sel, ok := u.X.(*ast.SelectorExpr)
			if !ok || !rw.Pure(sel) || loopBody(as) == nil {
				return true
			}
			v, _ := varOf(sel)
			if v == nil || !v.IsField() {
				return true
			}
			if _, ok := fields[v]; !ok {
				fields[v] = len(fields)
			}
			sites = append(sites, site{as, sel, v})
			return true
		})
	}
	if len(sites) == 0 {
		return 0
	}
	// Per function (a declaration or a function literal): the fields of interest it changes,
	// the functions it calls (a key of a pool for a call of a function value). A function value is a
	// function literal not called where it is written, or a function or method not called
	// where it is named: calling one may run any of them.
	type fnInfo struct {
		mut   map[int]bool
		calls map[any]bool // *types.Func or *ast.FuncLit
	}
	// A call of a function value may call any function value of its signature (the key
	// "fv:" + the signature).
	sigKey := func(t types.Type) string {
		sig, ok := types.Unalias(t).Underlying().(*types.Signature)
		if !ok {
			return "fv:?"
		}
		if hasTypeParam(sig) {
			// In generic code: any function value of as many parameters and results.
			return fmt.Sprintf("fvg:%d/%d", sig.Params().Len(), sig.Results().Len())
		}
		return "fv:" + types.TypeString(types.NewSignatureType(nil, nil, nil, sig.Params(), sig.Results(), sig.Variadic()), nil)
	}
	arity := func(t types.Type) string {
		sig, _ := types.Unalias(t).Underlying().(*types.Signature)
		if sig == nil {
			return "fvg:?"
		}
		return fmt.Sprintf("fvg:%d/%d", sig.Params().Len(), sig.Results().Len())
	}
	methodsByName := map[string][]*types.Func{}
	infos := map[any]*fnInfo{}
	values := map[any]string{} // function value -> its signature key
	// Function-typed fields and parameters of declared functions ("holders"): the functions
	// stored in each (holderVals: function literals, named functions, other holders), unless
	// one is stored from anything else, or a parameter's function is called as a function
	// value (holderOpen): then a call through the holder may call any function value of its
	// signature.
	holderVals := map[*types.Var][]any{}
	holderOpen := map[*types.Var]bool{}
	holderKey := func(v *types.Var) string { return fmt.Sprintf("hold:%d", v.Origin().Pos()) }
	isFunc := func(t types.Type) bool { _, ok := types.Unalias(t).Underlying().(*types.Signature); return ok }
	declParams := map[*types.Var]*types.Func{}
	for _, file := range rw.Files {
		for _, d := range file.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				if o, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					ps := o.Type().(*types.Signature).Params()
					for i := 0; i < ps.Len(); i++ {
						if isFunc(ps.At(i).Type()) {
							declParams[ps.At(i).Origin()] = o.Origin()
						}
					}
				}
			}
		}
	}
	holder := func(e ast.Expr) *types.Var {
		v, _ := varOf(e)
		if v == nil {
			return nil
		}
		v = v.Origin()
		if _, ok := declParams[v]; ok || v.IsField() {
			return v
		}
		return nil
	}
	stored := func(fv *types.Var, e ast.Expr) {
		fv = fv.Origin()
		e = rw.Unparen(e)
		switch x := e.(type) { // an instantiation f[T] of a generic function
		case *ast.IndexExpr:
			e = x.X
		case *ast.IndexListExpr:
			e = x.X
		}
		switch x := e.(type) {
		case *ast.FuncLit:
			holderVals[fv] = append(holderVals[fv], x)
			return
		case *ast.Ident:
			if x.Name == "nil" {
				return
			}
			if f, ok := rw.Info.Uses[x].(*types.Func); ok {
				holderVals[fv] = append(holderVals[fv], f.Origin())
				return
			}
		case *ast.SelectorExpr:
			if f, ok := rw.Info.Uses[x.Sel].(*types.Func); ok {
				if sl := rw.Info.Selections[x]; sl == nil || !types.IsInterface(sl.Recv()) {
					holderVals[fv] = append(holderVals[fv], f.Origin())
					return
				}
			}
		}
		if h := holder(e); h != nil {
			holderVals[fv] = append(holderVals[fv], holderKey(h))
			return
		}
		holderOpen[fv] = true
	}
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				st, ok := types.Unalias(typeOf(x)).Underlying().(*types.Struct)
				if !ok {
					return true
				}
				for i, el := range x.Elts {
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						if id, ok := kv.Key.(*ast.Ident); ok {
							if fv, ok := rw.Info.Uses[id].(*types.Var); ok && isFunc(fv.Type()) {
								stored(fv, kv.Value)
							}
						}
					} else if i < st.NumFields() && isFunc(st.Field(i).Type()) {
						stored(st.Field(i), el)
					}
				}
			case *ast.AssignStmt:
				for i, l := range x.Lhs {
					if h := holder(l); h != nil {
						if len(x.Lhs) != len(x.Rhs) {
							holderOpen[h] = true
						} else {
							stored(h, x.Rhs[i])
						}
					}
				}
			case *ast.UnaryExpr:
				if h := holder(x.X); x.Op == token.AND && h != nil {
					holderOpen[h] = true
				}
			}
			return true
		})
	}
	// implsOf are the methods an interface call of m on a value of interface type it may
	// run: method m of every type of the package that implements it.
	var named []types.Type
	for _, nm := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(nm).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		if _, isIface := tn.Type().Underlying().(*types.Interface); isIface {
			continue
		}
		named = append(named, tn.Type(), types.NewPointer(tn.Type()))
	}
	implCache := map[string][]any{}
	implsOf := func(it types.Type, m string) []any {
		key := types.TypeString(it, nil) + "." + m
		if r, ok := implCache[key]; ok {
			return r
		}
		iface := it.Underlying().(*types.Interface)
		seen := map[any]bool{}
		var r []any
		for _, t := range named {
			nt, _ := types.Unalias(t).(*types.Named)
			if p, ok := t.(*types.Pointer); ok {
				nt, _ = p.Elem().(*types.Named)
			}
			generic := nt != nil && nt.TypeParams() != nil
			if !generic && !types.Implements(t, iface) {
				continue
			}
			o, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, m)
			if f, ok := o.(*types.Func); ok && !seen[f.Origin()] {
				seen[f.Origin()] = true
				r = append(r, f.Origin())
			}
		}
		if len(r) == 0 {
			// No implementation in the package (a library interface): anything of the name.
			for _, f := range methodsByName[m] {
				r = append(r, f.Origin())
			}
		}
		implCache[key] = r
		return r
	}
	// callTo records a static call of f; a library function reaches this package only
	// through what it is given: function values, and the methods of interface values.
	callTo := func(fi *fnInfo, f *types.Func, x *ast.CallExpr) {
		if f.Pkg() == rw.Pkg {
			fi.calls[f.Origin()] = true
			return
		}
		for _, a := range x.Args {
			switch u := types.Unalias(typeOf(a)).Underlying().(type) {
			case *types.Signature:
				fi.calls[sigKey(u)] = true
			case *types.Interface:
				if !u.Empty() {
					fi.calls["an interface given to the library"] = true
				}
				for _, m := range []string{"String", "Error"} {
					for _, mf := range methodsByName[m] {
						fi.calls[mf.Origin()] = true
					}
				}
			}
		}
	}
	// inStruct are the fields of interest within a struct value of type t (through embedded
	// and nested struct values): what an assignment of a whole struct value changes.
	inStruct := func(t types.Type) []int {
		var r []int
		var walk func(st *types.Struct, depth int)
		walk = func(st *types.Struct, depth int) {
			for i := 0; i < st.NumFields(); i++ {
				f := st.Field(i)
				if j, ok := fields[f]; ok {
					r = append(r, j)
				}
				if sub, ok := types.Unalias(f.Type()).Underlying().(*types.Struct); ok && depth < 8 {
					walk(sub, depth+1)
				}
			}
		}
		if st, ok := types.Unalias(t).Underlying().(*types.Struct); ok {
			walk(st, 0)
		}
		return r
	}
	scan := func(body ast.Node, fi *fnInfo) {
		ast.Inspect(body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					if t := typeOf(l); t != nil {
						for _, j := range inStruct(t) {
							fi.mut[j] = true
						}
					}
				}
			case *ast.FuncLit:
				if x == body {
					return true
				}
				if c, ok := rw.Up(x).(*ast.CallExpr); ok && rw.Unparen(c.Fun) == x {
					fi.calls[x] = true
				} else {
					values[x] = sigKey(typeOf(x))
				}
				return false // a function of its own
			case *ast.Ident:
				if f, ok := rw.Info.Uses[x].(*types.Func); ok {
					e := rw.VarExpr(x)
					if c, ok := rw.Up(e).(*ast.CallExpr); !ok || rw.Unparen(c.Fun) != e {
						var sl *types.Selection
						if se, ok := e.(*ast.SelectorExpr); ok {
							sl = rw.Info.Selections[se]
						}
						if sl != nil && types.IsInterface(sl.Recv()) {
							for _, m := range methodsByName[f.Name()] {
								values[m.Origin()] = sigKey(m.Type())
							}
						} else {
							values[f.Origin()] = sigKey(f.Type())
						}
					}
					return true
				}
				v, ok := rw.Info.Uses[x].(*types.Var)
				if !ok {
					return true
				}
				i, ok := fields[v]
				if !ok {
					return true
				}
				e := rw.VarExpr(x)
				switch p := rw.Up(e).(type) {
				case *ast.AssignStmt:
					for _, l := range p.Lhs {
						if l == e {
							fi.mut[i] = true
						}
					}
				case *ast.UnaryExpr:
					if p.Op == token.AND {
						if c, ok := rw.Up(p).(*ast.CallExpr); ok {
							if nm, _ := rw.CallName(c); nm == "eachOf_" {
								return true // reads
							}
						}
						fi.mut[i] = true
					}
				case *ast.IncDecStmt:
					fi.mut[i] = true
				}
			case *ast.CallExpr:
				if tv, ok := rw.Info.Types[x.Fun]; ok && (tv.IsType() || tv.IsBuiltin()) {
					return true
				}
				if _, ok := rw.Unparen(x.Fun).(*ast.FuncLit); ok {
					return true
				}
				if s, ok := rw.Unparen(x.Fun).(*ast.SelectorExpr); ok {
					if sl, ok := rw.Info.Selections[s]; ok && sl.Kind() == types.MethodVal {
						f := sl.Obj().(*types.Func)
						if types.IsInterface(sl.Recv()) && f.Pkg() == rw.Pkg {
							for _, m := range implsOf(sl.Recv(), f.Name()) {
								fi.calls[m] = true
							}
						} else {
							callTo(fi, f, x)
						}
						return true
					}
				}
				if f := calleeFunc(x); f != nil {
					callTo(fi, f, x)
					return true
				}
				if h := holder(x.Fun); h != nil {
					fi.calls[holderKey(h)] = true // a function stored in a field or parameter
					return true
				}
				fi.calls[sigKey(typeOf(x.Fun))] = true
			}
			return true
		})
	}
	// Arguments for function-typed parameters, at every call whose callees are known.
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			x, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var targets []*types.Func
			if s, ok := rw.Unparen(x.Fun).(*ast.SelectorExpr); ok {
				if sl, ok := rw.Info.Selections[s]; ok && sl.Kind() == types.MethodVal {
					f := sl.Obj().(*types.Func)
					if types.IsInterface(sl.Recv()) && f.Pkg() == rw.Pkg {
						for _, m := range implsOf(sl.Recv(), f.Name()) {
							targets = append(targets, m.(*types.Func))
						}
					} else {
						targets = append(targets, f.Origin())
					}
				}
			}
			if targets == nil {
				if f := calleeFunc(x); f != nil {
					targets = append(targets, f.Origin())
				}
			}
			for _, f := range targets {
				ps := f.Type().(*types.Signature).Params()
				for i, a := range x.Args {
					if i < ps.Len() {
						if p := ps.At(i).Origin(); declParams[p] != nil && !(f.Type().(*types.Signature).Variadic() && i >= ps.Len()-1) {
							stored(p, a)
						}
					}
				}
			}
			return true
		})
	}
	var decls []*ast.FuncDecl
	for _, file := range rw.Files {
		for _, d := range file.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				if o, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					decls = append(decls, fd)
					if fd.Recv != nil {
						methodsByName[o.Name()] = append(methodsByName[o.Name()], o)
					}
				}
			}
		}
	}
	for _, fd := range decls {
		fi := &fnInfo{mut: map[int]bool{}, calls: map[any]bool{}}
		scan(fd.Body, fi)
		infos[rw.Info.Defs[fd.Name].(*types.Func)] = fi
	}
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if fl, ok := n.(*ast.FuncLit); ok {
				fi := &fnInfo{mut: map[int]bool{}, calls: map[any]bool{}}
				scan(fl, fi)
				infos[fl] = fi
			}
			return true
		})
	}
	// A function used as a value is called with arguments from anywhere.
	for v := range values {
		if f, ok := v.(*types.Func); ok {
			for p, g := range declParams {
				if g == f {
					holderOpen[p] = true
				}
			}
		}
	}
	for p := range declParams {
		if _, ok := holderVals[p]; !ok {
			holderVals[p] = nil // every parameter has a pool
		}
	}
	for fv, vs := range holderVals {
		pool := &fnInfo{mut: map[int]bool{}, calls: map[any]bool{}}
		if holderOpen[fv] {
			pool.calls[sigKey(fv.Type())] = true
		}
		for _, v := range vs {
			pool.calls[v] = true
		}
		infos[holderKey(fv)] = pool
	}
	for fv := range holderOpen {
		if _, ok := infos[holderKey(fv)]; !ok {
			infos[holderKey(fv)] = &fnInfo{mut: map[int]bool{}, calls: map[any]bool{sigKey(fv.Type()): true}}
		}
	}
	// Calling a function value may call any function used as a value with its signature.
	for v, k := range values {
		var t types.Type
		switch f := v.(type) {
		case *types.Func:
			t = f.Type()
		case *ast.FuncLit:
			t = typeOf(f)
		}
		for _, k := range []string{k, arity(t)} {
			pool, ok := infos[k]
			if !ok {
				pool = &fnInfo{mut: map[int]bool{}, calls: map[any]bool{}}
				infos[k] = pool
			}
			pool.calls[v] = true
		}
	}
	// What each function may change: a fixpoint over the call graph (all: no body here).
	type eff struct {
		all bool
		mut map[int]bool
	}
	effs := map[any]*eff{}
	for f, fi := range infos {
		e := &eff{mut: map[int]bool{}}
		for i := range fi.mut {
			e.mut[i] = true
		}
		effs[f] = e
	}
	callee := func(f any) *eff {
		if e, ok := effs[f]; ok {
			return e
		}
		if k, ok := f.(string); ok && (strings.HasPrefix(k, "fv") || strings.HasPrefix(k, "hold:")) {
			return &eff{mut: map[int]bool{}} // no function value of the signature exists
		}
		return &eff{all: true} // no body here (a library interface method without one)
	}
	for changed := true; changed; {
		changed = false
		for f, fi := range infos {
			e := effs[f]
			if e.all {
				continue
			}
			for c := range fi.calls {
				ce := callee(c)
				if ce.all {
					e.all, changed = true, true
					break
				}
				for i := range ce.mut {
					if !e.mut[i] {
						e.mut[i], changed = true, true
					}
				}
			}
		}
	}
	if tr := os.Getenv("NATIVE_TRACE"); tr != "" {
		name := func(c any) string {
			switch f := c.(type) {
			case *types.Func:
				return f.FullName()
			case *ast.FuncLit:
				return rw.Fset.Position(f.Pos()).String()
			}
			return fmt.Sprint(c)
		}
		var start any
		for f := range infos {
			if name(f) == tr {
				start = f
			}
		}
		prev := map[any]any{start: nil}
		q := []any{start}
		for len(q) > 0 {
			c := q[0]
			q = q[1:]
			fi := infos[c]
			if k, ok := c.(string); fi == nil && ok && (strings.HasPrefix(k, "fv") || strings.HasPrefix(k, "hold:")) {
				continue
			}
			if fi == nil || len(fi.mut) > 0 {
				for x := c; x != nil; x = prev[x] {
					fmt.Fprintf(os.Stderr, "trace: %s (body %v, mut %v)\n", name(x), infos[x] != nil, infos[x] != nil && len(infos[x].mut) > 0)
				}
				break
			}
			for d := range fi.calls {
				if _, seen := prev[d]; !seen {
					prev[d] = c
					q = append(q, d)
				}
			}
		}
	}
	n := 0
	for _, st := range sites {
		body := loopBody(st.as)
		fi := &fnInfo{mut: map[int]bool{}, calls: map[any]bool{}}
		scan(body, fi)
		i := fields[st.f]
		ok := !fi.mut[i]
		for c := range fi.calls {
			if ce := callee(c); ce.all || ce.mut[i] {
				ok = false
				break
			}
		}
		if os.Getenv("NATIVE_WHY") != "" && !ok {
			var by []string
			for c := range fi.calls {
				if ce := callee(c); ce.all || ce.mut[i] {
					switch f := c.(type) {
					case *types.Func:
						by = append(by, f.FullName())
					case *ast.FuncLit:
						by = append(by, rw.Fset.Position(f.Pos()).String())
					default:
						by = append(by, fmt.Sprint(f))
					}
				}
			}
			sort.Strings(by)
			fmt.Fprintf(os.Stderr, "why: eachOf_ at %s stays: %v %v\n", rw.Fset.Position(st.as.Pos()), fi.mut[i], by)
		}
		if ok && rangeLoop(st.as, st.x) {
			n++
		}
	}
	return n
}

// hasTypeParam reports whether signature sig mentions a type parameter.
func hasTypeParam(sig *types.Signature) bool {
	var has func(t types.Type) bool
	seen := map[types.Type]bool{}
	has = func(t types.Type) bool {
		if seen[t] {
			return false
		}
		seen[t] = true
		switch u := t.(type) {
		case *types.TypeParam:
			return true
		case *types.Pointer:
			return has(u.Elem())
		case *types.Slice:
			return has(u.Elem())
		case *types.Array:
			return has(u.Elem())
		case *types.Map:
			return has(u.Key()) || has(u.Elem())
		case *types.Chan:
			return has(u.Elem())
		case *types.Signature:
			for _, tu := range []*types.Tuple{u.Params(), u.Results()} {
				for i := 0; i < tu.Len(); i++ {
					if has(tu.At(i).Type()) {
						return true
					}
				}
			}
		case *types.Named:
			for i := 0; i < u.TypeArgs().Len(); i++ {
				if has(u.TypeArgs().At(i)) {
					return true
				}
			}
		case *types.Alias:
			return has(types.Unalias(u))
		}
		return false
	}
	return has(sig)
}
