// Command ranges is a post-flip step: a translated for-each loop, which walks its collection
// with a cursor object (foreach_, each, eachRev, eachOf_, eachRevOf_), and a loop over an
// explicit Iterator used only for hasNext/next, become range loops over Go 1.23 iterators
// (rt_iter.go):
//
//	it_1 := xs.each()                     for x := range xs.All() {
//	for it_1.next() {                ->       ...
//		var x *Foo = it_1.v               }
//		...
//	}
//
// The iterator walks the collection exactly as the cursor did (rt_iter.go): it reads the
// collection anew at every step and starts where the cursor started, and the range expression
// is evaluated where the cursor was made (the statement just before the loop). Sources:
//   - X.each() of a *TList: X.All(); X.eachRev(): X.AllRev(); eachOf_(p): sliceAll_(p);
//     eachRevOf_(p): sliceAllRev_(p);
//   - X.each() of a *TSet: X.All(); of a *TMap: X.All() for keys and values, X.Keys() for keys
//     only, X.Values() for values only;
//   - foreach_(c): elems_(c); when the loop's first statement casts the element,
//     `var x T = cast_[T](it.v)`, elemsAs_[T](c); when it converts it with a function of the
//     package from any, `var x T = F(it.v)` (str_, Node_of_, to_X, icast_X), elemsBy_(c, F);
//   - an iterator `it := C.iterator()` (or `var it IteratorOf[T] = C.iterator()`) whose only
//     uses are the loop's condition it.hasNext() and one it.next() in the loop's first
//     statement: C.asList() of a *TList, C.asSet() of a *TSet and C.asMap().keySet()/values()
//     of a *TMap range over the typed collection when the first statement unboxes the element
//     to the element type (their views' iterators read the typed collection in place as each
//     does); any other C of a type that implements Iterable ranges over elems_/elemsAs_/
//     elemsBy_(C), which call C.iterator() as the loop did, or read one of the runtime's
//     collections in place exactly as its iterator does (no runtime iterator checks for
//     concurrent modification).
//
// The variable of the first statement `var x T = ...` becomes the range variable when the
// element (converted as above) has type T and the cursor is read nowhere else; otherwise the
// range variable is a fresh name that replaces every read of the cursor's element. Go gives each
// iteration its own range variable, as Java's for-each variable is: assigning it in the body or
// capturing it in a function literal means what it did. A loop whose cursor is read inside a
// function literal (which could run after the cursor moved on), assigned, or used after the
// loop is left as it is. break and continue keep their meaning in a range-over-func body, and so
// would return, defer and a branch to an enclosing loop's label; but Go carries a return or such
// a branch out of the body through state checks after every call of it, and on a short body
// that returns the range loop measured 8-35% slower than the cursor loop (bodies that only break
// or continue the loop itself measured as fast), so a loop whose body returns, or branches to
// another label, keeps its cursor. No generated code uses goto.
//
// Rerunning the step changes nothing (the rewritten loops no longer match).
//
// The loop rewrites are applied only with -loops. Measured on go1.27.3 (per-pass tracer, ABBA,
// under the gate lock, against postflip5), the rewritten tree was slower on typed-full: +1.0%
// (8 runs each, medians) with every loop rewritten, +2.9% (6 runs each, a noisy host) with the
// loops that return kept, the slower passes the same in every run (inlineVariables,
// markPureFunctions, inferTypes, coalesceVariableNames), although bundle was 0.4% and 1.1%
// faster and microbenchmarks of short bodies without a return measured the range loops as fast
// as the cursors. A longer body is a closure the inlined iterator calls, and its captured
// variables live in memory across the enclosing function; the cursor (a stack struct, no
// allocation) has neither cost. Without -loops the step rewrites only C.iterator().next().
//
// -index (index.go) turns the cursor loops -loops leaves because their bodies return into
// index loops (typed lists, slices, typed sets and maps) and every loop over an immutable
// collection into a Go range over its elements; -entries (entries.go) turns loops over the
// entry set of a typed map's erased view into ranges over the typed map. Run after -loops.
//
// usage: ranges [-n] [-loops] [-index] [-entries] <dir of package closure>
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

var (
	done = map[string]int{}
	left = map[string]int{}
)

// named returns the origin name of t's named type (through a pointer), or "".
func named(t types.Type) string {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := types.Unalias(t).(*types.Named); ok {
		return n.Origin().Obj().Name()
	}
	return ""
}

func typeOf(e ast.Expr) types.Type {
	if tv, ok := rw.Info.Types[e]; ok {
		return tv.Type
	}
	return nil
}

// typeArgs returns the type arguments of the named type of t (through a pointer).
func typeArgs(t types.Type) []types.Type {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return nil
	}
	var out []types.Type
	for i := 0; i < n.TypeArgs().Len(); i++ {
		out = append(out, n.TypeArgs().At(i))
	}
	return out
}

func isAny(t types.Type) bool {
	i, ok := types.Unalias(t).(*types.Interface)
	return ok && i.Empty()
}

// method returns (receiver, name) of a call X.m() without arguments.
func method(e ast.Expr) (ast.Expr, string) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return nil, ""
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, ""
	}
	return s.X, s.Sel.Name
}

func objOf(e ast.Expr) types.Object {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	if o := rw.Info.Uses[id]; o != nil {
		return o
	}
	return rw.Info.Defs[id]
}

// source is the collection a loop ranges over.
type source struct {
	kind  string        // tlist, tlistRev, slice, sliceRev, tset, tmap, erased
	x     ast.Expr      // the collection (receiver, slice pointer or erased value)
	elem  types.Type    // element type (value type for tmap); nil for erased
	key   types.Type    // tmap's key type
	unbox func() string // explicit iterators over typed views: set when the element is boxed
}

// cursor matches `it := SRC` of a for-each cursor.
func cursor(st ast.Stmt) (*ast.Ident, *source) {
	as, ok := st.(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return nil, nil
	}
	id, ok := as.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, nil
	}
	if name, c := rw.CallName(as.Rhs[0]); c != nil && len(c.Args) == 1 {
		switch name {
		case "foreach_":
			return id, &source{kind: "erased", x: c.Args[0]}
		case "eachOf_", "eachRevOf_":
			el := typeArgs(typeOf(c.Fun))
			k := "slice"
			if name == "eachRevOf_" {
				k = "sliceRev"
			}
			if sl, ok := types.Unalias(typeOf(c.Args[0])).(*types.Pointer); ok {
				if s, ok := sl.Elem().Underlying().(*types.Slice); ok {
					return id, &source{kind: k, x: c.Args[0], elem: s.Elem()}
				}
			}
			_ = el
			return nil, nil
		}
	}
	recv, m := method(as.Rhs[0])
	if recv == nil || (m != "each" && m != "eachRev") {
		return nil, nil
	}
	t := typeOf(recv)
	args := typeArgs(t)
	switch named(t) {
	case "TList":
		if m == "eachRev" {
			return id, &source{kind: "tlistRev", x: recv, elem: args[0]}
		}
		return id, &source{kind: "tlist", x: recv, elem: args[0]}
	case "TSet":
		if m == "each" {
			return id, &source{kind: "tset", x: recv, elem: args[0]}
		}
	case "TMap":
		if m == "each" {
			return id, &source{kind: "tmap", x: recv, key: args[0], elem: args[1]}
		}
	}
	return nil, nil
}

// iterator matches `it := C.iterator()` or `var it IteratorOf[T] = C.iterator()`.
func iterator(st ast.Stmt) (*ast.Ident, ast.Expr) {
	var id *ast.Ident
	var val ast.Expr
	switch s := st.(type) {
	case *ast.AssignStmt:
		if s.Tok == token.DEFINE && len(s.Lhs) == 1 && len(s.Rhs) == 1 {
			id, _ = s.Lhs[0].(*ast.Ident)
			val = s.Rhs[0]
		}
	case *ast.DeclStmt:
		if g, ok := s.Decl.(*ast.GenDecl); ok && g.Tok == token.VAR && len(g.Specs) == 1 {
			if vs, ok := g.Specs[0].(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 1 {
				id, val = vs.Names[0], vs.Values[0]
			}
		}
	}
	if id == nil {
		return nil, nil
	}
	if named(typeOf(id)) != "Iterator" && !isIteratorAlias(rw.Info.Defs[id]) {
		return nil, nil
	}
	recv, m := method(val)
	if recv == nil || m != "iterator" {
		return nil, nil
	}
	return id, recv
}

func isIteratorAlias(o types.Object) bool {
	if o == nil {
		return false
	}
	it := rw.Pkg.Scope().Lookup("Iterator")
	return it != nil && types.Identical(o.Type(), it.Type())
}

// firstVar matches `var x T = v` and returns x and v.
func firstVar(st ast.Stmt) (*ast.Ident, ast.Expr) {
	d, ok := st.(*ast.DeclStmt)
	if !ok {
		return nil, nil
	}
	g, ok := d.Decl.(*ast.GenDecl)
	if !ok || g.Tok != token.VAR || len(g.Specs) != 1 {
		return nil, nil
	}
	vs, ok := g.Specs[0].(*ast.ValueSpec)
	if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 || vs.Type == nil {
		return nil, nil
	}
	return vs.Names[0], vs.Values[0]
}

// conv matches F(e) with F a conversion from any: cast_[T], or a non-generic package
// function of signature func(any) R. It returns F's text, R, and whether F is cast_.
func conv(call *ast.CallExpr) (string, types.Type, bool) {
	if len(call.Args) != 1 {
		return "", nil, false
	}
	if ix, ok := call.Fun.(*ast.IndexExpr); ok {
		if id, ok := ix.X.(*ast.Ident); ok && id.Name == "cast_" {
			return rw.Text(ix.Index), typeOf(call), true
		}
		return "", nil, false
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok {
		return "", nil, false
	}
	f, ok := rw.Info.Uses[id].(*types.Func)
	if !ok || f.Parent() != rw.Pkg.Scope() {
		return "", nil, false
	}
	sig := f.Type().(*types.Signature)
	if sig.TypeParams() != nil || sig.Params().Len() != 1 || sig.Results().Len() != 1 || sig.Variadic() || !isAny(sig.Params().At(0).Type()) {
		return "", nil, false
	}
	return id.Name, sig.Results().At(0).Type(), false
}

// stripAny drops a conversion any(x), which passing x to a parameter of type any makes anyway.
func stripAny(e ast.Expr) ast.Expr {
	if c, ok := rw.Unparen(e).(*ast.CallExpr); ok && len(c.Args) == 1 {
		if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "any" {
			if _, isType := rw.Info.Uses[id].(*types.TypeName); isType {
				return c.Args[0]
			}
		}
	}
	return e
}

// inFuncLit reports whether n is inside a function literal within body.
func inFuncLit(n ast.Node, body ast.Node) bool {
	for p := rw.Parent[n]; p != nil && p != body; p = rw.Parent[p] {
		if _, ok := p.(*ast.FuncLit); ok {
			return true
		}
	}
	return false
}

func within(n, outer ast.Node) bool { return n.Pos() >= outer.Pos() && n.End() <= outer.End() }

type fn struct {
	names map[string]bool
}

func (f *fn) fresh(base string) string {
	n := base
	for k := 2; f.names[n]; k++ {
		n = fmt.Sprintf("%s%d", base, k)
	}
	f.names[n] = true
	return n
}

// loopOf returns the for statement of st (under a label) with no init, post or range.
func loopOf(st ast.Stmt) *ast.ForStmt {
	if l, ok := st.(*ast.LabeledStmt); ok {
		st = l.Stmt
	}
	f, ok := st.(*ast.ForStmt)
	if !ok || f.Init != nil || f.Post != nil || f.Cond == nil {
		return nil
	}
	return f
}

// exitsEarly reports whether body (outside its function literals) has a return statement or a
// labelled break or continue that names a label other than the loop's own (st's). Go carries
// such an exit out of a range-over-func body through state checks after every call of the
// body: on a short loop body that returns, the range loop measured 8-35% slower than the
// cursor loop, while bodies that only break or continue the loop itself measured as fast.
func exitsEarly(st ast.Stmt, body *ast.BlockStmt) bool {
	own := ""
	if l, ok := st.(*ast.LabeledStmt); ok {
		own = l.Label.Name
	}
	exit := false
	ast.Inspect(body, func(n ast.Node) bool {
		switch s := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			exit = true
		case *ast.BranchStmt:
			if s.Label != nil && s.Label.Name != own || s.Tok == token.GOTO {
				exit = true
			}
		}
		return !exit
	})
	return exit
}

func condOn(loop *ast.ForStmt, it types.Object, m string) bool {
	recv, name := method(loop.Cond)
	return recv != nil && name == m && objOf(recv) == it
}

func process(f *fn, list []ast.Stmt) {
	for i := 0; i+1 < len(list); i++ {
		if id, src := cursor(list[i]); id != nil {
			if loop := loopOf(list[i+1]); loop != nil && condOn(loop, rw.Info.Defs[id], "next") {
				early := exitsEarly(list[i+1], loop.Body)
				if index && (early || src.kind == "erased" && immArg(src.x) != nil) {
					indexLoop(f, list[i], id, src, loop)
					continue
				}
				if !loops {
					continue
				}
				if early {
					left["the body returns, or branches to an enclosing loop's label (slower as a range-over-func loop)"]++
					continue
				}
				cursorLoop(f, list[i], id, src, loop)
			}
			continue
		}
		if !loops {
			continue
		}
		if id, c := iterator(list[i]); id != nil {
			if loop := loopOf(list[i+1]); loop != nil && condOn(loop, rw.Info.Defs[id], "hasNext") {
				iteratorLoop(f, list[i], id, c, loop)
			}
		}
	}
}

// cursorLoop rewrites `it := SRC; for it.next() { ... it.v / it.k ... }`.
func cursorLoop(f *fn, decl ast.Stmt, id *ast.Ident, src *source, loop *ast.ForStmt) {
	it := rw.Info.Defs[id]
	// Every use of it: the condition, and reads it.v / it.k in the body outside function literals.
	reads := map[string][]*ast.SelectorExpr{}
	for _, u := range rw.UsesOf[it] {
		if within(u, loop.Cond) {
			continue
		}
		s, ok := rw.Parent[u].(*ast.SelectorExpr)
		if !ok || s.X != u || !within(u, loop.Body) || (s.Sel.Name != "v" && s.Sel.Name != "k") {
			left["the cursor is used other than as the loop's element"]++
			return
		}
		if inFuncLit(s, loop.Body) {
			left["the element is read inside a function literal"]++
			return
		}
		if as, ok := rw.Up(s).(*ast.AssignStmt); ok {
			for _, l := range as.Lhs {
				if rw.Unparen(l) == s {
					left["the element is assigned"]++
					return
				}
			}
		}
		if u, ok := rw.Up(s).(*ast.UnaryExpr); ok && u.Op == token.AND {
			left["the element's address is taken"]++
			return
		}
		reads[s.Sel.Name] = append(reads[s.Sel.Name], s)
	}
	body := loop.Body
	var drops []ast.Stmt
	// The variables of the first statements var x T = it.v / it.k / F(it.v).
	takeFirst := func(field string, at int, want types.Type) (string, bool) {
		if at >= len(body.List) || len(reads[field]) != 1 {
			return "", false
		}
		x, val := firstVar(body.List[at])
		if x == nil || !types.Identical(rw.Info.Defs[x].Type(), want) {
			return "", false
		}
		if s, ok := rw.Unparen(val).(*ast.SelectorExpr); !ok || s != reads[field][0] {
			return "", false
		}
		drops = append(drops, body.List[at])
		return x.Name, true
	}
	n := strings.TrimPrefix(id.Name, "it")
	var header string
	switch src.kind {
	case "tmap":
		var kName, vName string
		at := 0
		if len(reads["k"]) > 0 {
			if k, ok := takeFirst("k", at, src.key); ok {
				kName, at = k, at+1
			}
		}
		if len(reads["v"]) > 0 {
			if v, ok := takeFirst("v", at, src.elem); ok {
				vName = v
			}
		}
		if len(reads["k"]) > 0 && kName == "" {
			kName = f.fresh("k" + n)
		}
		if len(reads["v"]) > 0 && vName == "" {
			vName = f.fresh("v" + n)
		}
		x := rw.Render(src.x)
		switch {
		case kName != "" && vName != "":
			header = kName + ", " + vName + " := range " + x + ".All()"
		case kName != "":
			header = kName + " := range " + x + ".Keys()"
		case vName != "":
			header = vName + " := range " + x + ".Values()"
		default:
			header = "range " + x + ".Keys()"
		}
		bind(reads["k"], drops, kName)
		bind(reads["v"], drops, vName)
		done["tmap"]++
	default:
		if len(reads["k"]) > 0 {
			left["the cursor is used other than as the loop's element"]++
			return
		}
		seq := ""
		name := ""
		switch src.kind {
		case "tlist", "tset":
			seq = rw.Render(src.x) + ".All()"
		case "tlistRev":
			seq = rw.Render(src.x) + ".AllRev()"
		case "slice":
			seq = "sliceAll_(" + rw.Render(src.x) + ")"
		case "sliceRev":
			seq = "sliceAllRev_(" + rw.Render(src.x) + ")"
		case "erased":
			c := rw.Render(stripAny(src.x))
			seq = "elems_(" + c + ")"
			if len(body.List) > 0 && len(reads["v"]) == 1 {
				if x, val := firstVar(body.List[0]); x != nil {
					if call, ok := rw.Unparen(val).(*ast.CallExpr); ok && len(call.Args) == 1 && rw.Unparen(stripAny(call.Args[0])) == reads["v"][0] {
						if fText, r, isCast := conv(call); fText != "" && types.Identical(r, rw.Info.Defs[x].Type()) {
							if isCast {
								seq = "elemsAs_[" + fText + "](" + c + ")"
							} else {
								seq = "elemsBy_(" + c + ", " + fText + ")"
							}
							drops = append(drops, body.List[0])
							name = x.Name
						}
					}
				}
			}
		}
		if name == "" && len(reads["v"]) > 0 {
			want := src.elem
			if want == nil {
				want = types.Universe.Lookup("any").Type()
			}
			if x, ok := takeFirst("v", 0, want); ok {
				name = x
			}
		}
		if name == "" && len(reads["v"]) > 0 {
			name = f.fresh("el" + n)
		}
		if name == "" {
			header = "range " + seq
		} else {
			header = name + " := range " + seq
		}
		bind(reads["v"], drops, name)
		done[src.kind]++
	}
	rw.Rewrite[decl] = func() string { return rw.Del }
	for _, d := range drops {
		rw.Rewrite[d] = func() string { return rw.Del }
	}
	rw.Rewrite[loop] = func() string { return "for " + header + " " + rw.Render(body) }
}

// bind replaces the reads not dropped with the range variable.
func bind(reads []*ast.SelectorExpr, drops []ast.Stmt, name string) {
	for _, r := range reads {
		dropped := false
		for _, d := range drops {
			if within(r, d) {
				dropped = true
			}
		}
		if !dropped {
			rw.Rewrite[r] = func() string { return name }
		}
	}
}

// iteratorLoop rewrites `it := C.iterator(); for it.hasNext() { var x T = F(it.next()) ... }`.
func iteratorLoop(f *fn, decl ast.Stmt, id *ast.Ident, c ast.Expr, loop *ast.ForStmt) {
	it := rw.Info.Defs[id]
	us := rw.UsesOf[it]
	body := loop.Body
	if exitsEarly(rw.Parent[loop].(ast.Stmt), body) {
		left["iterator: the body returns, or branches to an enclosing loop's label"]++
		return
	}
	if len(us) != 2 || !within(us[0], loop.Cond) || len(body.List) == 0 || !within(us[1], body.List[0]) {
		left["iterator: used other than for the loop's hasNext and one next in its first statement"]++
		return
	}
	x, val := firstVar(body.List[0])
	if x == nil {
		left["iterator: the first statement does not declare the element"]++
		return
	}
	xt := rw.Info.Defs[x].Type()
	call, ok := rw.Unparen(val).(*ast.CallExpr)
	var nextCall ast.Expr
	var fText string
	var r types.Type
	isCast := false
	if ok {
		if recv, m := method(call); recv != nil && m == "next" && objOf(recv) == it {
			nextCall, r = call, typeOf(call)
		} else if len(call.Args) == 1 {
			if recv, m := method(call.Args[0]); recv != nil && m == "next" && objOf(recv) == it {
				nextCall = call.Args[0]
				fText, r, isCast = conv(call)
			}
		}
	}
	if nextCall == nil || r == nil || !types.Identical(r, xt) {
		left["iterator: the first statement is not the element converted to its type"]++
		return
	}
	// A typed collection behind a view: its own iterator.
	seq := ""
	if fText != "" || isAny(xt) {
		var elem types.Type
		var typed string
		if recv, m := method(c); recv != nil {
			t := typeOf(recv)
			args := typeArgs(t)
			switch {
			case m == "asList" && named(t) == "TList":
				typed, elem = rw.Render(recv)+".All()", args[0]
			case m == "asSet" && named(t) == "TSet":
				typed, elem = rw.Render(recv)+".All()", args[0]
			case m == "keySet" || m == "values":
				if mr, mm := method(recv); mr != nil && mm == "asMap" && named(typeOf(mr)) == "TMap" {
					margs := typeArgs(typeOf(mr))
					if m == "keySet" {
						typed, elem = rw.Render(mr)+".Keys()", margs[0]
					} else {
						typed, elem = rw.Render(mr)+".Values()", margs[1]
					}
				}
			}
		}
		if typed != "" {
			cv := rw.ConvFor(elem)
			name, _ := rw.CallName(call)
			if types.Identical(elem, xt) && (cv.OK && cv.IsUnbox(name) || isAny(elem) && fText == "") {
				seq = typed
				done["iterator over a typed view"]++
			} else {
				left["iterator: a typed view whose element is converted to another type"]++
				return
			}
		}
	}
	if seq == "" {
		if !implementsIterable(typeOf(c)) {
			left["iterator: the collection's type does not implement Iterable"]++
			return
		}
		cs := rw.Render(c)
		switch {
		case fText == "":
			seq = "elems_(" + cs + ")"
		case isCast:
			seq = "elemsAs_[" + fText + "](" + cs + ")"
		default:
			seq = "elemsBy_(" + cs + ", " + fText + ")"
		}
		done["iterator over an Iterable"]++
	}
	rw.Rewrite[decl] = func() string { return rw.Del }
	rw.Rewrite[body.List[0]] = func() string { return rw.Del }
	header := x.Name + " := range " + seq
	rw.Rewrite[loop] = func() string { return "for " + header + " " + rw.Render(body) }
}

// firstElem rewrites C.iterator().next(), the first element: of a typed set or map behind its
// view, unboxed to the element type, S.First() / M.FirstKey() / M.FirstValue(); of any other
// Iterable, first_(C), which reads one of the runtime's collections in place and calls
// C.iterator().next() for anything else.
func firstElem(next *ast.CallExpr) {
	itr, m := method(next)
	if itr == nil || m != "next" {
		return
	}
	c, m := method(itr)
	if c == nil || m != "iterator" {
		return
	}
	// The unboxing around the read, if any.
	var outer *ast.CallExpr
	if p, ok := rw.Up(next).(*ast.CallExpr); ok && len(p.Args) == 1 && rw.Unparen(p.Args[0]) == next {
		outer = p
	}
	if recv, vm := method(c); recv != nil && outer != nil {
		var typed string
		var elem types.Type
		t := typeOf(recv)
		switch {
		case vm == "asSet" && named(t) == "TSet":
			typed, elem = rw.Render(recv)+".First()", typeArgs(t)[0]
		case vm == "keySet" || vm == "values":
			if mr, mm := method(recv); mr != nil && mm == "asMap" && named(typeOf(mr)) == "TMap" {
				margs := typeArgs(typeOf(mr))
				if vm == "keySet" {
					typed, elem = rw.Render(mr)+".FirstKey()", margs[0]
				} else {
					typed, elem = rw.Render(mr)+".FirstValue()", margs[1]
				}
			}
		}
		if typed != "" {
			cv := rw.ConvFor(elem)
			name, _ := rw.CallName(outer)
			if cv.OK && cv.IsUnbox(name) && types.Identical(typeOf(outer), elem) {
				rw.Rewrite[outer] = func() string { return typed }
				done["first element of a typed view"]++
				return
			}
		}
	}
	if !implementsIterable(typeOf(c)) {
		left["first element: the collection's type does not implement Iterable"]++
		return
	}
	rw.Rewrite[next] = func() string { return "first_(" + rw.Render(c) + ")" }
	done["first element of an Iterable"]++
}

func implementsIterable(t types.Type) bool {
	if t == nil {
		return false
	}
	o := rw.Pkg.Scope().Lookup("Iterable")
	if o == nil {
		return false
	}
	iface, ok := o.Type().Underlying().(*types.Interface)
	return ok && types.Implements(t, iface)
}

var loops, index, entries bool

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		switch a {
		case "-n":
			report = true
		case "-loops":
			loops = true
		case "-index":
			index = true
		case "-entries":
			entries = true
		default:
			fmt.Fprintln(os.Stderr, "usage: ranges [-n] [-loops] [-index] [-entries] <dir of package closure>")
			os.Exit(2)
		}
	}
	rw.Load(os.Args[len(os.Args)-1])
	for _, file := range rw.Files {
		if !loops && !index && !entries || !strings.HasPrefix(filepath.Base(rw.Fset.File(file.Pos()).Name()), "g_") {
			continue
		}
		for _, d := range file.Decls {
			// A function, or the function literals of a package-level variable (step lambdas).
			f := &fn{names: map[string]bool{}}
			ast.Inspect(d, func(x ast.Node) bool {
				if id, ok := x.(*ast.Ident); ok {
					f.names[id.Name] = true
				}
				return true
			})
			ast.Inspect(d, func(x ast.Node) bool {
				switch s := x.(type) {
				case *ast.BlockStmt:
					process(f, s.List)
				case *ast.CaseClause:
					process(f, s.Body)
				case *ast.CommClause:
					process(f, s.Body)
				case *ast.RangeStmt:
					if index {
						immRange(f, s)
					}
					if entries {
						entryRange(f, s)
					}
				}
				return true
			})
		}
	}
	for _, file := range rw.Files {
		if strings.HasPrefix(filepath.Base(rw.Fset.File(file.Pos()).Name()), "g_") {
			ast.Inspect(file, func(x ast.Node) bool {
				if c, ok := x.(*ast.CallExpr); ok {
					firstElem(c)
				}
				return true
			})
		}
	}
	total := 0
	var ks []string
	for k, v := range done {
		ks = append(ks, k)
		total += v
	}
	sort.Strings(ks)
	fmt.Printf("ranges: %d sites\n", total)
	for _, k := range ks {
		fmt.Printf("  %5d %s\n", done[k], k)
	}
	ks = ks[:0]
	for k := range left {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Printf("  left: %5d %s\n", left[k], k)
	}
	if report {
		return
	}
	fmt.Printf("files changed: %d\n", rw.SaveChecked())
}
