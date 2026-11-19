package main

// -index: the for-each loops that -loops leaves as cursors because their bodies return (or
// branch to an enclosing loop's label), and the range-over-func loops over immutable
// collections, become plain Go loops with no iterator object and no closure:
//
//   - a typed list (X.each()), a slice (eachOf_(&s)) and their reverse cursors: an index loop
//     that reads the list's length and item anew at every step, exactly as the cursor did
//     (for i := 0; i < len(s); i++ { x := s[i] ... });
//   - a typed set or map (X.each()): an index loop over its table from its first live entry
//     that skips removed entries, as the cursor did;
//   - an immutable collection (foreach_(obj_(X)), elems_/elemsAs_/elemsBy_(obj_(X), ...) with X
//     of static type *ImmutableCollection): a range over its elements, immElems_(X); an
//     immutable collection never changes, so the loop sees what its cursor saw.
//
// In a Go index or range loop return, break, continue and labelled branches are ordinary
// jumps, so the early exits that make a range-over-func body slow cost nothing here.

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"postflip/rw"
)

// isImm reports whether t is *ImmutableCollection (any of its aliases).
func isImm(t types.Type) bool {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	return ok && n.Obj().Name() == "ImmutableCollection" && n.Obj().Parent() == rw.Pkg.Scope()
}

// immArg returns X when c is obj_(X), any(X) or X with X of static type *ImmutableCollection.
func immArg(c ast.Expr) ast.Expr {
	c = rw.Unparen(stripAny(c))
	if call, ok := c.(*ast.CallExpr); ok && len(call.Args) == 1 {
		if name, _ := rw.CallName(call); name == "obj_" {
			c = rw.Unparen(call.Args[0])
		}
	}
	if isImm(typeOf(c)) {
		return c
	}
	return nil
}

// assigned reports whether o is assigned, incremented or has its address taken anywhere in
// the function declaring it.
func assigned(o types.Object) bool {
	for _, u := range rw.UsesOf[o] {
		switch p := rw.Parent[u].(type) {
		case *ast.AssignStmt:
			for _, l := range p.Lhs {
				if l == u {
					return true
				}
			}
		case *ast.IncDecStmt:
			return true
		case *ast.UnaryExpr:
			if p.Op == token.AND {
				return true
			}
		}
	}
	return false
}

// stableVar reports whether e is a local variable or parameter that is never assigned after
// its declaration, so that reading it anywhere in the loop gives the value the cursor was made
// from.
func stableVar(e ast.Expr) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	v, ok := rw.Info.Uses[id].(*types.Var)
	if !ok || v.Parent() == rw.Pkg.Scope() || v.IsField() {
		return false
	}
	return !assigned(v)
}

// stablePlace reports whether p is &E for E a chain of field selections from a stable
// variable (&s, &this.f, &a.b.c), so that E read anywhere in the loop is the slice the cursor
// reads through p.
func stablePlace(p ast.Expr) (ast.Expr, bool) {
	u, ok := rw.Unparen(p).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return nil, false
	}
	e := rw.Unparen(u.X)
	for {
		switch x := e.(type) {
		case *ast.SelectorExpr:
			if s := rw.Info.Selections[x]; s == nil || s.Kind() != types.FieldVal {
				return nil, false
			}
			e = rw.Unparen(x.X)
			continue
		case *ast.Ident:
			if stableVar(x) {
				return u.X, true
			}
		}
		return nil, false
	}
}

// indexLoop rewrites a cursor loop whose body exits early into an index or range loop.
func indexLoop(f *fn, decl ast.Stmt, id *ast.Ident, src *source, loop *ast.ForStmt) bool {
	it := rw.Info.Defs[id]
	var reads []*ast.SelectorExpr
	var kreads []*ast.SelectorExpr
	for _, u := range rw.UsesOf[it] {
		if within(u, loop.Cond) {
			continue
		}
		s, ok := rw.Parent[u].(*ast.SelectorExpr)
		if !ok || s.X != u || !within(u, loop.Body) || (s.Sel.Name != "v" && s.Sel.Name != "k") {
			left["index: the cursor is used other than as the loop's element"]++
			return false
		}
		if inFuncLit(s, loop.Body) {
			left["index: the element is read inside a function literal"]++
			return false
		}
		if as, ok := rw.Up(s).(*ast.AssignStmt); ok {
			for _, l := range as.Lhs {
				if rw.Unparen(l) == s {
					left["index: the element is assigned"]++
					return false
				}
			}
		}
		if u, ok := rw.Up(s).(*ast.UnaryExpr); ok && u.Op == token.AND {
			left["index: the element's address is taken"]++
			return false
		}
		if s.Sel.Name == "k" {
			kreads = append(kreads, s)
		} else {
			reads = append(reads, s)
		}
	}
	n := strings.TrimPrefix(id.Name, "it")
	var pre, header string // the statement before the loop, the loop's header
	var elem, key string   // the element (value) and key at the current position
	var guard string       // a first statement of the body
	ix := f.fresh("i" + n)
	switch src.kind {
	case "tlist", "tlistRev", "tset", "tmap":
		b := rw.Render(src.x)
		if !stableVar(src.x) {
			b = f.fresh("l" + n)
			pre = b + " := " + rw.Render(src.x)
		}
		switch src.kind {
		case "tlist":
			header = ix + " := 0; " + ix + " < len(" + b + ".items); " + ix + "++"
			elem = b + ".items[" + ix + "]"
		case "tlistRev":
			header = ix + " := 0; " + ix + " < len(" + b + ".items); " + ix + "++"
			elem = b + ".items[len(" + b + ".items)-1-" + ix + "]"
		default:
			header = ix + " := " + b + ".first(); " + ix + " < len(" + b + ".keys); " + ix + "++"
			guard = "if " + b + ".isDead(" + ix + ") {\ncontinue\n}"
			if src.kind == "tset" {
				elem = b + ".keys[" + ix + "]"
			} else {
				key, elem = b+".keys["+ix+"]", b+".vals["+ix+"]"
			}
		}
		done["index loop over a typed "+strings.TrimSuffix(src.kind, "Rev")]++
	case "slice", "sliceRev":
		s, ok := stablePlace(src.x)
		var sl string
		if ok {
			sl = rw.Render(s)
		} else {
			p := f.fresh("p" + n)
			pre = p + " := " + rw.Render(src.x)
			sl = "(*" + p + ")"
		}
		header = ix + " := 0; " + ix + " < len(" + sl + "); " + ix + "++"
		if src.kind == "slice" {
			elem = sl + "[" + ix + "]"
		} else {
			elem = sl + "[len(" + sl + ")-1-" + ix + "]"
		}
		done["index loop over a slice"]++
	case "erased":
		x := immArg(src.x)
		if x == nil {
			left["index: an erased collection (the cursor reads the runtime's collections in place; its range-over-func form measured slower)"]++
			return false
		}
		el := ""
		// var x any = it.v as the loop's first statement: x is the range variable.
		if len(reads) == 1 && len(loop.Body.List) > 0 {
			if v, val := firstVar(loop.Body.List[0]); v != nil && rw.Unparen(val) == reads[0] && isAny(rw.Info.Defs[v].Type()) {
				el = v.Name
				rw.Rewrite[loop.Body.List[0]] = func() string { return rw.Del }
				reads = nil
			}
		}
		if el == "" && len(reads) > 0 {
			el = f.fresh("el" + n)
			bind(reads, nil, el)
		}
		header = "range immElems_(" + rw.Render(x) + ")"
		if el != "" {
			header = "_, " + el + " := " + header
		}
		done["range loop over an immutable collection"]++
		rw.Rewrite[decl] = func() string { return rw.Del }
		body := loop.Body
		rw.Rewrite[loop] = func() string { return "for " + header + " " + rw.Render(body) }
		return true
	default:
		return false
	}
	// The element: substituted for its one read when that is in the first statement (no code
	// runs between the step and the read); otherwise read once into a variable at the top.
	var tops []string
	if guard != "" {
		tops = append(tops, guard)
	}
	bindOne := func(rs []*ast.SelectorExpr, val, base string) {
		if len(rs) == 0 {
			return
		}
		if len(rs) == 1 && len(loop.Body.List) > 0 && within(rs[0], loop.Body.List[0]) && firstStmtReads(loop.Body.List[0], rs[0]) {
			rw.Rewrite[rs[0]] = func() string { return val }
			return
		}
		v := f.fresh(base + n)
		tops = append(tops, v+" := "+val)
		for _, r := range rs {
			rw.Rewrite[r] = func() string { return v }
		}
	}
	if src.kind == "tmap" {
		// both reads in the first statements count as first only for the first one
		bindOne(kreads, key, "k")
		if len(kreads) > 0 && len(reads) > 0 {
			v := f.fresh("v" + n)
			tops = append(tops, v+" := "+elem)
			for _, r := range reads {
				rw.Rewrite[r] = func() string { return v }
			}
		} else {
			bindOne(reads, elem, "v")
		}
	} else {
		if len(kreads) > 0 {
			panic("k read of a non-map cursor")
		}
		bindOne(reads, elem, "el")
	}
	if pre != "" {
		rw.Rewrite[decl] = func() string { return pre }
	} else {
		rw.Rewrite[decl] = func() string { return rw.Del }
	}
	body := loop.Body
	rw.Rewrite[loop] = func() string {
		b := rw.Render(body)
		if len(tops) > 0 {
			b = "{\n" + strings.Join(tops, "\n") + strings.TrimPrefix(b, "{")
		}
		return "for " + header + " " + b
	}
	return true
}

// firstStmtReads reports whether r is read by st before anything else st does could change
// the collection: st is `var x T = R` or `x := R` with R the read, possibly converted.
func firstStmtReads(st ast.Stmt, r *ast.SelectorExpr) bool {
	var val ast.Expr
	if _, v := firstVar(st); v != nil {
		val = v
	} else if as, ok := st.(*ast.AssignStmt); ok && len(as.Rhs) == 1 && len(as.Lhs) == 1 {
		val = as.Rhs[0]
	}
	if val == nil {
		return false
	}
	val = rw.Unparen(val)
	if val == r {
		return true
	}
	if c, ok := val.(*ast.CallExpr); ok && len(c.Args) == 1 && rw.Unparen(c.Args[0]) == r {
		if name, _, _ := conv(c); name != "" {
			return true
		}
	}
	return false
}

// immRange rewrites `for x := range elems_/elemsAs_[T]/elemsBy_(obj_(X), F)` over an immutable
// collection X into a range over its elements, `for _, el := range immElems_(X) { x := F(el)`.
func immRange(f *fn, r *ast.RangeStmt) {
	call, ok := rw.Unparen(r.X).(*ast.CallExpr)
	if !ok || len(call.Args) == 0 {
		return
	}
	var name, typeArg string
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		name = fun.Name
	case *ast.IndexExpr:
		if id, ok := fun.X.(*ast.Ident); ok {
			name, typeArg = id.Name, rw.Render(fun.Index)
		}
	}
	var conv string
	switch {
	case name == "elems_" && len(call.Args) == 1:
	case name == "elemsAs_" && len(call.Args) == 1 && typeArg != "":
		conv = "cast_[" + typeArg + "]"
	case name == "elemsBy_" && len(call.Args) == 2:
		conv = rw.Render(call.Args[1])
		if _, ok := rw.Unparen(call.Args[1]).(*ast.Ident); !ok {
			return
		}
	default:
		return
	}
	if o, ok := rw.Info.Uses[identOf(call.Fun)].(*types.Func); !ok || o.Parent() != rw.Pkg.Scope() {
		return
	}
	x := immArg(call.Args[0])
	if x == nil {
		return
	}
	if r.Value != nil || r.Tok == token.ASSIGN {
		return
	}
	src := "immElems_(" + rw.Render(x) + ")"
	body := r.Body
	var header, top string
	switch {
	case r.Key == nil:
		header = "range " + src
	case conv == "":
		header = "_, " + rw.Render(r.Key) + " := range " + src
	default:
		k, _ := r.Key.(*ast.Ident)
		if k == nil {
			return
		}
		if k.Name == "_" {
			header = "range " + src
			break
		}
		el := f.fresh("el")
		header = "_, " + el + " := range " + src
		top = k.Name + " := " + conv + "(" + el + ")"
	}
	done["range-over-func loop over an immutable collection"]++
	rw.Rewrite[r] = func() string {
		b := rw.Render(body)
		if top != "" {
			b = "{\n" + top + strings.TrimPrefix(b, "{")
		}
		return "for " + header + " " + b
	}
}

func identOf(e ast.Expr) *ast.Ident {
	switch x := e.(type) {
	case *ast.Ident:
		return x
	case *ast.IndexExpr:
		id, _ := x.X.(*ast.Ident)
		return id
	}
	return nil
}
