package main

// The abstract interpretation of one function for one string (see the package comment).

import (
	"go/ast"
	"go/token"
	"go/types"

	"postflip/rw"
)

func (a *an) interpret() (why string) {
	defer func() {
		if r := recover(); r != nil {
			f, ok := r.(fail)
			if !ok {
				panic(r)
			}
			why = f.why
			if verbose && f.at != nil {
				println(rw.Fset.Position(f.at.Pos()).String(), f.why)
			}
		}
	}()
	a.zero = types.NewVar(token.NoPos, nil, "zero", types.Typ[types.Int32])
	a.end = types.NewVar(token.NoPos, nil, "end", types.Typ[types.Int32])
	a.scan = map[ast.Node]*types.Var{}
	a.scanW = map[ast.Node]string{}
	a.ctls = nil
	a.frams = []*frame{{}}
	st := &state{v: map[*types.Var]val{a.zero: {k: kIn | kEnd}, a.end: {k: kEnd}}, chars: map[*types.Var]cpos{}}
	a.block(a.fd.Body.List, st)
	return ""
}

// mentionsIV reports whether e reads an index variable.
func (a *an) mentionsIV(e ast.Node) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if x, ok := n.(ast.Expr); ok {
			if v := identVar(x); v != nil && a.iv[v] {
				found = true
			}
		}
		return !found
	})
	return found
}

// try runs f; a failure in an expression that involves no index variable only leaves the
// calls in it.
func (a *an) try(e ast.Node, f func()) {
	if a.mentionsIV(e) {
		f()
		return
	}
	defer func() {
		if r := recover(); r != nil {
			fl, ok := r.(fail)
			if !ok {
				panic(r)
			}
			ast.Inspect(e, func(n ast.Node) bool {
				if c, ok := n.(*ast.CallExpr); ok {
					if si := a.sites[c]; si != nil && si.bad == "" {
						si.bad = fl.why
					}
				}
				return true
			})
		}
	}()
	f()
}

// ---- positions

func (a *an) konst(k int64, st *state, at ast.Node) val {
	if k < 0 {
		return val{k: kNeg, neg: k, negEx: true}
	}
	return add(st.v[a.zero], k, st.v[a.zero], at)
}

func (a *an) eval(e ast.Expr, st *state) val {
	e = rw.Unparen(e)
	if k, ok := constInt(e); ok {
		return a.konst(k, st, e)
	}
	if v := identVar(e); v != nil {
		if a.iv[v] {
			x, ok := st.v[v]
			if !ok || x.k == 0 {
				bail("a position variable is read where the analysis has no value for it", e)
			}
			if x.k&kGone != 0 {
				bail("a position in an earlier value of the string variable is used", e)
			}
			return x
		}
		bail(notPos, e)
	}
	if name, c := a.opOn(e); c != nil {
		a.visitOp(c, st)
		if name == "String_length" {
			return st.v[a.end]
		}
		if !posOps[name] {
			bail("a position is computed by "+name, e)
		}
		nd, ok := needle(name, c)
		if !ok {
			bail("indexOf of a char or String that is not an ASCII constant", c)
		}
		var asc uint32
		for j := range nd {
			asc |= bit(j)
		}
		return val{k: kNeg | kIn, neg: -1, negEx: true, asc: asc}
	}
	switch x := e.(type) {
	case *ast.BinaryExpr:
		if x.Op == token.ADD || x.Op == token.SUB {
			if k, ok := constInt(x.Y); ok {
				v := a.eval(x.X, st)
				if x.Op == token.SUB {
					k = -k
				}
				return add(v, k, st.v[a.zero], x)
			}
			if k, ok := constInt(x.X); ok && x.Op == token.ADD {
				return add(a.eval(x.Y, st), k, st.v[a.zero], x)
			}
		}
	case *ast.CallExpr:
		if y := conversion(x); y != nil {
			return a.eval(y, st)
		}
		if fl := iife(x); fl != nil {
			return a.callLit(fl, st, true)
		}
	}
	bail("a position is computed by an expression the step does not follow", e)
	return val{}
}

// posOf returns the variable and offset a position expression is relative to.
func (a *an) posOf(e ast.Expr) (*types.Var, int, bool) {
	e = rw.Unparen(e)
	if k, ok := constInt(e); ok {
		return a.zero, int(k), true
	}
	if v := identVar(e); v != nil && a.iv[v] {
		return v, 0, true
	}
	if name, c := a.opOn(e); c != nil && name == "String_length" {
		return a.end, 0, true
	}
	switch x := e.(type) {
	case *ast.BinaryExpr:
		if x.Op == token.ADD || x.Op == token.SUB {
			if k, ok := constInt(x.Y); ok {
				if b, o, ok := a.posOf(x.X); ok {
					if x.Op == token.SUB {
						k = -k
					}
					return b, o + int(k), true
				}
			}
			if k, ok := constInt(x.X); ok && x.Op == token.ADD {
				if b, o, ok := a.posOf(x.Y); ok {
					return b, o + int(k), true
				}
			}
		}
	case *ast.CallExpr:
		if y := conversion(x); y != nil {
			return a.posOf(y)
		}
	}
	return nil, 0, false
}

// charPos returns the position of a char of s: a charAt call's index, or a char variable's.
func (a *an) charPos(e ast.Expr, st *state) (cpos, bool) {
	e = charOperand(e, a.isCharOfS)
	if e == nil {
		return cpos{}, false
	}
	if _, c := a.opOn(e); c != nil {
		if b, o, ok := a.posOf(c.Args[1]); ok {
			return cpos{b, o}, true
		}
		return cpos{}, false
	}
	if v := identVar(e); v != nil {
		p, ok := st.chars[v]
		return p, ok
	}
	return cpos{}, false
}

// bytePos returns the position of a byte read s[t] of s at a position t.
func (a *an) bytePos(e ast.Expr) (cpos, bool) {
	for {
		e = rw.Unparen(e)
		x := conversion(e)
		if x == nil {
			break
		}
		e = x
	}
	ix, ok := e.(*ast.IndexExpr)
	if !ok || identVar(ix.X) != a.s {
		return cpos{}, false
	}
	if b, o, ok := a.posOf(ix.Index); ok {
		return cpos{b, o}, true
	}
	return cpos{}, false
}

// setASCII records that the char at p is ASCII.
func setASCII(st *state, p cpos) *state {
	v, ok := st.v[p.base]
	if !ok || bit(p.off) == 0 {
		return st
	}
	st = st.copy()
	v.asc |= bit(p.off)
	if p.off >= 0 {
		v.k &^= kEnd | kPast
		if p.off == 0 {
			if v.k&(kMid|kPrev) != 0 {
				v.k = v.k&^(kMid|kPrev) | kIn
			}
			v.k &^= kNeg
		}
	} else {
		v.k &^= kNeg | kPast
	}
	if v.k&kNeg == 0 {
		v.neg, v.negEx = 0, false
	}
	if v.k == 0 {
		st.dead = true
	}
	st.v[p.base] = v
	return st
}

// ---- conditions

func (a *an) refine(st *state, e ast.Expr, truth bool) *state {
	if st.dead {
		return st
	}
	e = rw.Unparen(e)
	switch x := e.(type) {
	case *ast.UnaryExpr:
		if x.Op == token.NOT {
			return a.refine(st, x.X, !truth)
		}
	case *ast.BinaryExpr:
		switch x.Op {
		case token.LAND:
			if truth {
				return a.refine(a.refine(st, x.X, true), x.Y, true)
			}
			return joinStates(a.refine(st, x.X, false), a.refine(a.refine(st, x.X, true), x.Y, false))
		case token.LOR:
			if !truth {
				return a.refine(a.refine(st, x.X, false), x.Y, false)
			}
			return joinStates(a.refine(st, x.X, true), a.refine(a.refine(st, x.X, false), x.Y, true))
		}
		return a.refineCmp(st, x, truth)
	case *ast.CallExpr:
		if id, ok := x.Fun.(*ast.Ident); ok && len(x.Args) == 1 {
			if fn, ok := rw.Info.Uses[id].(*types.Func); ok {
				if r, ok := blind[fn]; ok && r != truth {
					if p, ok := a.charPos(x.Args[0], st); ok {
						return setASCII(st, p)
					}
				}
			}
		}
		// s.startsWith(P, t) of an ASCII constant P
		if name, c := a.opOn(x); c != nil && name == "String_startsWith_String_int" && truth {
			if cs, ok := constString(c.Args[1]); ok && asciiOnly(cs) {
				if b, o, ok := a.posOf(c.Args[2]); ok {
					for j := 0; j < len(cs); j++ {
						st = setASCII(st, cpos{b, o + j})
					}
				}
			}
		}
		// strings.HasPrefix / HasSuffix of an ASCII constant
		if sel, ok := x.Fun.(*ast.SelectorExpr); ok && truth && len(x.Args) == 2 && identVar(x.Args[0]) == a.s {
			if pk, ok := sel.X.(*ast.Ident); ok && pk.Name == "strings" {
				if _, isPkg := rw.Info.Uses[pk].(*types.PkgName); isPkg {
					if cs, ok := constString(x.Args[1]); ok && asciiOnly(cs) && len(cs) < 32-ascBias {
						for j := 0; j < len(cs); j++ {
							switch sel.Sel.Name {
							case "HasPrefix":
								st = setASCII(st, cpos{a.zero, j})
							case "HasSuffix":
								st = setASCII(st, cpos{a.end, j - len(cs)})
							}
						}
					}
				}
			}
		}
	}
	return st
}

func mirror(op token.Token) token.Token {
	return map[token.Token]token.Token{token.EQL: token.EQL, token.NEQ: token.NEQ, token.LSS: token.GTR, token.GTR: token.LSS, token.LEQ: token.GEQ, token.GEQ: token.LEQ}[op]
}

func negate(op token.Token) token.Token {
	return map[token.Token]token.Token{token.EQL: token.NEQ, token.NEQ: token.EQL, token.LSS: token.GEQ, token.GEQ: token.LSS, token.GTR: token.LEQ, token.LEQ: token.GTR}[op]
}

func (a *an) refineCmp(st *state, x *ast.BinaryExpr, truth bool) *state {
	op := x.Op
	if !truth {
		op = negate(op)
	}
	if op == 0 {
		return st
	}
	l, r := x.X, x.Y
	if _, ok := constInt(l); ok {
		l, r, op = r, l, mirror(op)
	}
	// a char test
	if k, ok := constInt(r); ok {
		p, ok := a.charPos(l, st)
		if !ok {
			p, ok = a.bytePos(l)
		}
		if ok {
			ascii := false
			switch op {
			case token.EQL, token.LEQ:
				ascii = k >= 0 && k < 0x80
			case token.LSS:
				ascii = k >= 0 && k <= 0x80
			}
			if ascii {
				return setASCII(st, p)
			}
			return st
		}
		// String_indexOf(ASCII, c) compared with 0 / -1: found
		if name, c := callName(l); name == "String_indexOf" && len(c.Args) == 2 {
			if p, ok := a.charPos(c.Args[1], st); ok {
				if cs, ok := constString(c.Args[0]); ok && asciiOnly(cs) {
					if (op == token.GEQ && k == 0) || (op == token.NEQ && k == -1) || (op == token.GTR && k == -1) {
						return setASCII(st, p)
					}
				}
			}
			return st
		}
	}
	// a position variable against the end or a constant
	v := identVar(l)
	if v == nil || !a.iv[v] {
		l, r, op = r, l, mirror(op)
		v = identVar(l)
		if v == nil || !a.iv[v] {
			return st
		}
	}
	cur, ok := st.v[v]
	if !ok {
		return st
	}
	n := cur
	if a.isEnd(r, st) {
		switch op {
		case token.LSS: // v < end
			n.k &^= kEnd | kPast
		case token.LEQ:
			n.k &^= kPast
		case token.GEQ, token.GTR, token.EQL:
			k := kind(0)
			if cur.k&(kEnd|kMid) != 0 && op != token.GTR {
				k |= kEnd
			}
			if cur.k&kPast != 0 && op != token.EQL {
				k |= kPast
			}
			n = val{k: k}
			if k == kEnd {
				n.asc = st.v[a.end].asc
			}
		case token.NEQ:
			n.k &^= kEnd
		}
	} else if c, ok := constInt(r); ok {
		switch {
		case op == token.EQL && c < 0:
			if cur.k&kNeg != 0 && (!cur.negEx || cur.neg == c) {
				n = val{k: kNeg, neg: c, negEx: true}
			} else {
				n = val{}
			}
		case (op == token.LSS && c <= 0) || (op == token.LEQ && c < 0):
			n = val{k: cur.k & kNeg, neg: cur.neg, negEx: cur.negEx}
		case (op == token.GEQ && c >= 0) || (op == token.GTR && c >= -1) || (op == token.NEQ && c == -1 && cur.negEx && cur.neg == -1):
			n.k &^= kNeg
			n.neg, n.negEx = 0, false
		}
	}
	if n == cur {
		return st
	}
	st = st.copy()
	st.v[v] = n
	if n.k == 0 {
		st.dead = true
	}
	return st
}

// isEnd reports whether e is length() of s, or a variable that holds it.
func (a *an) isEnd(e ast.Expr, st *state) bool {
	e = rw.Unparen(e)
	if name, c := a.opOn(e); c != nil && name == "String_length" {
		return true
	}
	if v := identVar(e); v != nil && a.iv[v] {
		return st.v[v].k == kEnd
	}
	return false
}

// ---- expressions

var compare = map[token.Token]bool{token.EQL: true, token.NEQ: true, token.LSS: true, token.LEQ: true, token.GTR: true, token.GEQ: true}

func (a *an) walk(e ast.Expr, st *state) {
	if e == nil || st.dead {
		return
	}
	switch x := e.(type) {
	case *ast.BinaryExpr:
		switch {
		case x.Op == token.LAND || x.Op == token.LOR:
			a.walk(x.X, st)
			a.walk(x.Y, a.refine(st, x.X, x.Op == token.LAND))
			return
		case compare[x.Op] && a.isPosExpr(x.X) && a.isPosExpr(x.Y) && (a.mentionsPos(x.X) || a.mentionsPos(x.Y)):
			a.try(x, func() { a.cmp(x, st) })
			return
		}
	case *ast.CallExpr:
		if _, c := a.opOn(x); c != nil {
			a.try(c, func() { a.visitOp(c, st) })
			return
		}
		if fl := iife(x); fl != nil {
			if a.isPosExpr(x) && a.mentionsPos(x) {
				a.try(x, func() { a.callLit(fl, st, true) })
			} else {
				a.callLit(fl, st, false)
			}
			return
		}
	case *ast.FuncLit:
		return
	}
	ast.Inspect(e, func(n ast.Node) bool {
		if n == ast.Node(e) {
			return true
		}
		if ce, ok := n.(ast.Expr); ok {
			a.walk(ce, st)
		}
		return false
	})
}

// cmp checks a comparison of positions.
func (a *an) cmp(x *ast.BinaryExpr, st *state) {
	op := x.Op
	// v < 1 is v <= 0: 0 is a boundary in both
	for i, side := range []ast.Expr{x.Y, x.X} {
		if k, ok := constInt(side); ok && k == 1 {
			o := op
			if i == 1 {
				o = mirror(o)
			}
			if o == token.LSS || o == token.GEQ {
				if v := a.eval([]ast.Expr{x.X, x.Y}[i], st); v.k&^consistent == 0 {
					return
				}
			}
		}
	}
	l, r := a.eval(x.X, st), a.eval(x.Y, st)
	for i, v := range []val{l, r} {
		if v.k&(kMid|kPrev) == 0 {
			continue
		}
		side := []ast.Expr{x.X, x.Y}[i]
		other := []ast.Expr{x.Y, x.X}[i]
		if k, ok := constInt(other); ok && v.k&kMid == 0 {
			o := op
			if i == 1 {
				o = mirror(o)
			}
			// a sign test: t-1 < 0 exactly when the boundary t is 0
			if (k == 0 && (o == token.GEQ || o == token.LSS)) || (k == -1 && (o == token.GTR || o == token.LEQ || o == token.EQL || o == token.NEQ)) {
				continue
			}
		}
		if a.scanCV != nil && identVar(side) == a.scanCV && v.k&kPrev == 0 && a.isEnd([]ast.Expr{x.Y, x.X}[i], st) {
			continue // i < length() at the head of a scan loop
		}
		why := "a position steps over a char not known to be ASCII and is compared"
		if w := a.scanWhy(side); w != "" {
			why = "a loop steps over chars not known to be ASCII: " + w
		}
		bail(why, x)
	}
}

func (a *an) scanWhy(e ast.Expr) string {
	for p := rw.Parent[e]; p != nil; p = rw.Parent[p] {
		if w, ok := a.scanW[p]; ok {
			return w
		}
	}
	return ""
}

// visitOp records the positions a String_ call on s receives.
func (a *an) visitOp(c *ast.CallExpr, st *state) {
	si := a.sites[c]
	if st.dead || si == nil {
		return
	}
	if si.bad != "" {
		for _, arg := range c.Args[1:] {
			a.walk(arg, st)
		}
		return
	}
	if si.args == nil {
		si.args = make([]val, len(c.Args))
	}
	si.visited = true
	for i, arg := range c.Args[1:] {
		i++
		if (si.name != "String_charAt" && si.name != "String_substring" && si.name != "String_substring_int") && i == 1 {
			continue // the needle
		}
		v := a.eval(arg, st)
		if si.name == "String_charAt" && a.scanCV != nil && identVar(arg) == a.scanCV {
			si.scanOK = true
		}
		si.args[i] = join(si.args[i], v)
	}
}

// callLit interprets a conditional literal; with pos, it returns the join of its results.
func (a *an) callLit(fl *ast.FuncLit, st *state, pos bool) val {
	fr := &frame{pos: pos}
	a.frams = append(a.frams, fr)
	a.block(fl.Body.List, st)
	a.frams = a.frams[:len(a.frams)-1]
	return fr.val
}

// ---- statements

func (a *an) block(list []ast.Stmt, st *state) *state {
	for _, s := range list {
		st = a.stmt(s, st, "")
	}
	return st
}

var dead = &state{dead: true}

const notPos = "a number that is not a position in the string is used as one"

func isPanic(e ast.Expr) bool {
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := c.Fun.(*ast.Ident)
	return ok && id.Name == "panic"
}

func (a *an) stmt(s ast.Stmt, st *state, label string) *state {
	if st.dead {
		return st
	}
	switch x := s.(type) {
	case *ast.ExprStmt:
		a.walk(x.X, st)
		if isPanic(x.X) {
			return dead
		}
		return st
	case *ast.AssignStmt:
		return a.assign(x, st)
	case *ast.IncDecStmt:
		if v := identVar(x.X); v != nil && a.iv[v] {
			a.noLitAssign(v, x)
			d := int64(1)
			if x.Tok == token.DEC {
				d = -1
			}
			n := st.copy()
			n.v[v] = add(st.v[v], d, st.v[a.zero], x)
			a.invalidate(n, v)
			return n
		}
		a.walk(x.X, st)
		return st
	case *ast.DeclStmt:
		gd, ok := x.Decl.(*ast.GenDecl)
		if !ok || gd.Tok != token.VAR {
			return st
		}
		for _, sp := range gd.Specs {
			vs := sp.(*ast.ValueSpec)
			for i, nm := range vs.Names {
				v, _ := rw.Info.Defs[nm].(*types.Var)
				var rhs ast.Expr
				if i < len(vs.Values) {
					rhs = vs.Values[i]
				}
				st = a.set(v, rhs, st, x, token.DEFINE)
			}
		}
		return st
	case *ast.IfStmt:
		if x.Init != nil {
			st = a.stmt(x.Init, st, "")
		}
		a.walk(x.Cond, st)
		t := a.block(x.Body.List, a.refine(st, x.Cond, true))
		f := a.refine(st, x.Cond, false)
		if x.Else != nil {
			f = a.stmt(x.Else, f, "")
		}
		return joinStates(t, f)
	case *ast.BlockStmt:
		return a.block(x.List, st)
	case *ast.ForStmt:
		return a.forStmt(x, st, label)
	case *ast.RangeStmt:
		return a.rangeStmt(x, st, label)
	case *ast.SwitchStmt:
		return a.switchStmt(x, st, label)
	case *ast.TypeSwitchStmt:
		return a.typeSwitch(x, st, label)
	case *ast.LabeledStmt:
		return a.stmt(x.Stmt, st, x.Label.Name)
	case *ast.ReturnStmt:
		fr := a.frams[len(a.frams)-1]
		for _, r := range x.Results {
			if fr.pos {
				fr.val = join(fr.val, a.eval(r, st))
			} else {
				a.walk(r, st)
			}
		}
		fr.rets = joinStates(fr.rets, st)
		return dead
	case *ast.BranchStmt:
		switch x.Tok {
		case token.FALLTHROUGH:
			return st
		case token.GOTO:
			bail("goto", x)
		}
		c := a.target(x)
		if c == nil {
			bail("a branch to a statement the step does not follow", x)
		}
		if x.Tok == token.BREAK {
			c.brk = joinStates(c.brk, st)
		} else {
			c.cont = joinStates(c.cont, st)
		}
		return dead
	case *ast.EmptyStmt:
		return st
	case *ast.DeferStmt:
		a.walk(x.Call, st)
		return st
	case *ast.GoStmt:
		a.walk(x.Call, st)
		return st
	}
	bail("a statement the step does not follow", s)
	return st
}

func (a *an) target(b *ast.BranchStmt) *ctl {
	for i := len(a.ctls) - 1; i >= 0; i-- {
		c := a.ctls[i]
		if b.Label != nil {
			if c.label == b.Label.Name {
				return c
			}
			continue
		}
		if b.Tok == token.CONTINUE && !c.loop {
			continue
		}
		return c
	}
	return nil
}

// noLitAssign rejects an assignment of a position or char variable inside a conditional
// literal: the step does not carry state out of it.
func (a *an) noLitAssign(v *types.Var, at ast.Node) {
	if len(a.frams) > 1 && (a.iv[v] || a.cv[v]) {
		bail("a position or char variable is assigned inside a conditional expression", at)
	}
}

// invalidate forgets the chars read at positions relative to v.
func (a *an) invalidate(st *state, v *types.Var) {
	for c, p := range st.chars {
		if p.base == v {
			delete(st.chars, c)
		}
	}
}

// set gives variable v the value of rhs (nil: the zero value).
func (a *an) set(v *types.Var, rhs ast.Expr, st *state, at ast.Node, tok token.Token) *state {
	switch {
	case v != nil && a.iv[v]:
		a.noLitAssign(v, at)
		var x val
		switch {
		case rhs == nil:
			x = st.v[a.zero]
		case tok == token.ADD_ASSIGN || tok == token.SUB_ASSIGN:
			k, _ := constInt(rhs)
			if tok == token.SUB_ASSIGN {
				k = -k
			}
			x = add(st.v[v], k, st.v[a.zero], at)
		default:
			x = a.eval(rhs, st)
		}
		n := st.copy()
		n.v[v] = x
		a.invalidate(n, v)
		if x.k == 0 {
			n.dead = true
		}
		return n
	case v != nil && a.cv[v]:
		a.noLitAssign(v, at)
		n := st.copy()
		delete(n.chars, v)
		if rhs != nil {
			a.walk(rhs, st)
			if _, c := a.opOn(rw.Unparen(rhs)); c != nil {
				if b, o, ok := a.posOf(c.Args[1]); ok {
					n.chars[v] = cpos{b, o}
				}
			}
		}
		return n
	}
	if v != nil && v == a.s {
		// a new string: the positions in the old one are gone
		if rhs != nil {
			a.walk(rhs, st)
		}
		if len(a.frams) > 1 {
			bail("the string variable is assigned inside a conditional expression", at)
		}
		n := st.copy()
		for x := range n.v {
			n.v[x] = val{k: kGone}
		}
		n.v[a.zero] = val{k: kIn | kEnd}
		n.v[a.end] = val{k: kEnd}
		n.chars = map[*types.Var]cpos{}
		return n
	}
	if rhs != nil {
		a.walk(rhs, st)
	}
	return st
}

func (a *an) assign(x *ast.AssignStmt, st *state) *state {
	if len(x.Lhs) != len(x.Rhs) {
		for _, r := range x.Rhs {
			a.walk(r, st)
		}
		for _, l := range x.Lhs {
			if v := identVar(l); v != nil && (a.iv[v] || a.cv[v]) {
				bail("a position or char variable is assigned from several results", x)
			}
		}
		return st
	}
	if len(x.Lhs) > 1 {
		for _, l := range x.Lhs {
			if v := identVar(l); v != nil && (a.iv[v] || a.cv[v]) {
				bail("a position or char variable is assigned in a parallel assignment", x)
			}
		}
	}
	out := st
	for i, l := range x.Lhs {
		v := identVar(l)
		if v == nil {
			a.walk(l, st)
		}
		if v != nil && (a.iv[v] || a.cv[v] || v == a.s) {
			out = a.set(v, x.Rhs[i], out, x, x.Tok)
		} else {
			a.walk(x.Rhs[i], st)
		}
	}
	return out
}

// ---- loops

// scanVar returns the index variable of a scan loop, or nil (with why).
func (a *an) scanVar(x *ast.ForStmt) (*types.Var, string) {
	if x.Cond == nil {
		return nil, ""
	}
	var v *types.Var
	for _, c := range conjuncts(x.Cond) {
		b, ok := rw.Unparen(c).(*ast.BinaryExpr)
		if !ok {
			continue
		}
		l, r, op := b.X, b.Y, b.Op
		if op == token.GTR {
			l, r, op = r, l, token.LSS
		}
		if op != token.LSS {
			continue
		}
		if iv := identVar(l); iv != nil && a.iv[iv] && a.endLike(r) {
			v = iv
		}
	}
	if v == nil {
		return nil, ""
	}
	// the advance
	hasPost := false
	if x.Post != nil {
		if !isIncr(x.Post, v) {
			return nil, ""
		}
		hasPost = true
	} else if n := len(x.Body.List); n == 0 || !isIncr(x.Body.List[n-1], v) {
		return nil, ""
	}
	isChar := func(e ast.Expr) bool {
		if name, c := a.opOn(rw.Unparen(e)); c != nil && name == "String_charAt" && identVar(c.Args[1]) == v {
			return true
		}
		return false
	}
	switch s1eval(x.Cond, isChar) {
	case s1False:
		return nil, "" // the body only runs at ASCII chars
	case s1Bad:
		return nil, "the loop condition is not a test the step can evaluate for a non-ASCII char"
	}
	chars := map[*types.Var]bool{}
	isChar2 := func(e ast.Expr) bool {
		if isChar(e) {
			return true
		}
		cv := identVar(e)
		return cv != nil && chars[cv]
	}
	if ok, why := a.s1NoOp(x.Body.List, v, hasPost, true, chars, isChar2); !ok {
		return nil, why
	}
	return v, ""
}

func (a *an) endLike(e ast.Expr) bool {
	e = rw.Unparen(e)
	if name, c := a.opOn(e); c != nil && name == "String_length" {
		return true
	}
	if v := identVar(e); v != nil && a.iv[v] {
		return true // checked at the analysis: it must hold length()
	}
	return false
}

func conjuncts(e ast.Expr) []ast.Expr {
	if b, ok := rw.Unparen(e).(*ast.BinaryExpr); ok && b.Op == token.LAND {
		return append(conjuncts(b.X), conjuncts(b.Y)...)
	}
	return []ast.Expr{e}
}

func isIncr(s ast.Stmt, v *types.Var) bool {
	switch x := s.(type) {
	case *ast.IncDecStmt:
		return x.Tok == token.INC && identVar(x.X) == v
	case *ast.AssignStmt:
		if x.Tok == token.ADD_ASSIGN && len(x.Lhs) == 1 && identVar(x.Lhs[0]) == v {
			k, ok := constInt(x.Rhs[0])
			return ok && k == 1
		}
	}
	return false
}

// s1NoOp checks that statements do nothing when the char at v is not ASCII: they read it,
// test it and go on to the next iteration.
func (a *an) s1NoOp(list []ast.Stmt, v *types.Var, hasPost, top bool, chars map[*types.Var]bool, isChar func(ast.Expr) bool) (bool, string) {
	for i, s := range list {
		last := top && i == len(list)-1
		switch x := s.(type) {
		case *ast.EmptyStmt:
		case *ast.DeclStmt:
			gd, _ := x.Decl.(*ast.GenDecl)
			if gd == nil || len(gd.Specs) != 1 {
				return false, "it declares variables at a non-ASCII char"
			}
			vs := gd.Specs[0].(*ast.ValueSpec)
			if len(vs.Names) != 1 || len(vs.Values) != 1 || !isChar(vs.Values[0]) {
				return false, "it declares variables at a non-ASCII char"
			}
			cv, _ := rw.Info.Defs[vs.Names[0]].(*types.Var)
			chars[cv] = true
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE && len(x.Lhs) == 1 && len(x.Rhs) == 1 && isChar(x.Rhs[0]) {
				cv, _ := rw.Info.Defs[x.Lhs[0].(*ast.Ident)].(*types.Var)
				chars[cv] = true
				continue
			}
			return false, "it assigns at a non-ASCII char"
		case *ast.IfStmt:
			if x.Init != nil {
				return false, "an if with an init statement"
			}
			o := s1eval(x.Cond, isChar)
			if o == s1Bad {
				return false, "a test that is not pure or not on the char"
			}
			if o != s1False {
				ok, why := a.s1NoOp(x.Body.List, v, hasPost, false, chars, isChar)
				if !ok {
					return false, why
				}
				if !endsContinue(x.Body.List) && o == s1True {
					// falls through to the rest
				}
			}
			if o != s1True && x.Else != nil {
				ok, why := a.s1NoOp([]ast.Stmt{x.Else}, v, hasPost, false, chars, isChar)
				if !ok {
					return false, why
				}
			}
			if o == s1True && endsContinue(x.Body.List) {
				return true, ""
			}
			if o == s1False && x.Else != nil {
				if b, ok := x.Else.(*ast.BlockStmt); ok && endsContinue(b.List) {
					return true, ""
				}
			}
		case *ast.BlockStmt:
			ok, why := a.s1NoOp(x.List, v, hasPost, false, chars, isChar)
			if !ok {
				return false, why
			}
		case *ast.SwitchStmt:
			if x.Init != nil || x.Tag == nil || charOperand(x.Tag, isChar) == nil {
				return false, "a switch that is not on the char"
			}
			for _, cc := range x.Body.List {
				cl := cc.(*ast.CaseClause)
				if cl.List == nil {
					ok, why := a.s1NoOp(cl.Body, v, hasPost, false, chars, isChar)
					if !ok {
						return false, why
					}
				}
				for _, e := range cl.List {
					if k, ok := constInt(e); !ok || k < 0 || k >= 0x80 {
						return false, "a case that is not an ASCII char"
					}
				}
			}
		case *ast.BranchStmt:
			if x.Tok == token.CONTINUE && x.Label == nil {
				return true, ""
			}
			return false, "it leaves the loop at a non-ASCII char"
		case *ast.IncDecStmt:
			if last && !hasPost && isIncr(x, v) {
				return true, ""
			}
			return false, "it changes a variable at a non-ASCII char"
		default:
			return false, "it does something at a non-ASCII char"
		}
	}
	return true, ""
}

func endsContinue(l []ast.Stmt) bool {
	if len(l) == 0 {
		return false
	}
	b, ok := l[len(l)-1].(*ast.BranchStmt)
	return ok && b.Tok == token.CONTINUE && b.Label == nil
}

// s2 is position v where the char is ASCII.
func s2(v val) val {
	k := v.k
	if k&(kMid|kIn) == 0 {
		return val{k: 0}
	}
	return val{k: kIn, asc: v.asc | bit(0)}
}

func (a *an) forStmt(x *ast.ForStmt, st *state, label string) *state {
	if x.Init != nil {
		st = a.stmt(x.Init, st, "")
	}
	sv, why := a.scanVar(x)
	if why != "" {
		a.scanW[x] = why
	}
	c := &ctl{node: x, label: label, loop: true}
	a.ctls = append(a.ctls, c)
	defer func() { a.ctls = a.ctls[:len(a.ctls)-1] }()
	head := st
	for iter := 0; ; iter++ {
		if iter > 50 {
			bail("the analysis of a loop does not settle", x)
		}
		c.brk, c.cont = nil, nil
		if sv != nil {
			hv := head.v[sv]
			if hv.k&(kNeg|kPrev) != 0 {
				bail("a scan loop may start at a negative position", x)
			}
			// its bound must be length()
			for _, cj := range conjuncts(x.Cond) {
				if b, ok := rw.Unparen(cj).(*ast.BinaryExpr); ok {
					for _, side := range []ast.Expr{b.X, b.Y} {
						if bv := identVar(side); bv != nil && bv != sv && a.iv[bv] && head.v[bv].k != kEnd {
							bail("a scan loop is bounded by a position other than length()", x)
						}
					}
				}
			}
		}
		if x.Cond != nil {
			a.scanCV = sv
			a.walk(x.Cond, head)
			a.scanCV = nil
		}
		entry := head
		if x.Cond != nil {
			entry = a.refine(head, x.Cond, true)
		}
		if sv != nil && !entry.dead {
			entry = entry.copy()
			entry.v[sv] = s2(entry.v[sv])
			if entry.v[sv].k == 0 {
				entry.dead = true
			}
		}
		out := a.block(x.Body.List, entry)
		out = joinStates(out, c.cont)
		if x.Post != nil && !out.dead {
			out = a.stmt(x.Post, out, "")
		}
		next := joinStates(st, out)
		if sv != nil && !next.dead {
			next = next.copy()
			hv := next.v[sv]
			hv.k |= kMid | kEnd
			hv.asc = 0
			next.v[sv] = hv
		}
		if sameState(next, head) {
			break
		}
		head = next
	}
	exit := head
	if x.Cond != nil {
		exit = a.refine(head, x.Cond, false)
	}
	return joinStates(exit, c.brk)
}

func (a *an) rangeStmt(x *ast.RangeStmt, st *state, label string) *state {
	for _, e := range []ast.Expr{x.Key, x.Value} {
		if v := identVar(e); v != nil && (a.iv[v] || a.cv[v]) {
			bail("a position or char variable is a range variable", x)
		}
	}
	a.walk(x.X, st)
	c := &ctl{node: x, label: label, loop: true}
	a.ctls = append(a.ctls, c)
	defer func() { a.ctls = a.ctls[:len(a.ctls)-1] }()
	head := st
	for iter := 0; ; iter++ {
		if iter > 50 {
			bail("the analysis of a loop does not settle", x)
		}
		c.brk, c.cont = nil, nil
		out := a.block(x.Body.List, head)
		out = joinStates(out, c.cont)
		next := joinStates(st, out)
		if sameState(next, head) {
			break
		}
		head = next
	}
	return joinStates(head, c.brk)
}

func (a *an) switchStmt(x *ast.SwitchStmt, st *state, label string) *state {
	if x.Init != nil {
		st = a.stmt(x.Init, st, "")
	}
	if x.Tag != nil {
		a.walk(x.Tag, st)
	}
	c := &ctl{node: x, label: label}
	a.ctls = append(a.ctls, c)
	defer func() { a.ctls = a.ctls[:len(a.ctls)-1] }()
	out := dead
	var fall *state
	hasDefault := false
	rest := st // tagless: the cases before were false
	for _, cc := range x.Body.List {
		cl := cc.(*ast.CaseClause)
		var entry *state
		if cl.List == nil {
			hasDefault = true
			entry = st
		} else if x.Tag != nil {
			for _, e := range cl.List {
				a.walk(e, st)
			}
			entry = st
			allASCII := true
			for _, e := range cl.List {
				if k, ok := constInt(e); !ok || k < 0 || k >= 0x80 {
					allASCII = false
				}
			}
			if p, ok := a.charPos(x.Tag, st); ok && allASCII {
				entry = setASCII(st, p)
			}
		} else {
			entry = dead
			for _, e := range cl.List {
				a.walk(e, rest)
				entry = joinStates(entry, a.refine(rest, e, true))
				rest = a.refine(rest, e, false)
			}
		}
		if fall != nil {
			entry = joinStates(entry, fall)
		}
		res := a.block(cl.Body, entry)
		fall = nil
		if n := len(cl.Body); n > 0 {
			if b, ok := cl.Body[n-1].(*ast.BranchStmt); ok && b.Tok == token.FALLTHROUGH {
				fall = res
				continue
			}
		}
		out = joinStates(out, res)
	}
	if !hasDefault {
		if x.Tag == nil {
			out = joinStates(out, rest)
		} else {
			out = joinStates(out, st)
		}
	}
	return joinStates(out, c.brk)
}

func (a *an) typeSwitch(x *ast.TypeSwitchStmt, st *state, label string) *state {
	if x.Init != nil {
		st = a.stmt(x.Init, st, "")
	}
	if as, ok := x.Assign.(*ast.AssignStmt); ok {
		for _, r := range as.Rhs {
			a.walk(r, st)
		}
	} else if es, ok := x.Assign.(*ast.ExprStmt); ok {
		a.walk(es.X, st)
	}
	c := &ctl{node: x, label: label}
	a.ctls = append(a.ctls, c)
	defer func() { a.ctls = a.ctls[:len(a.ctls)-1] }()
	out := dead
	hasDefault := false
	for _, cc := range x.Body.List {
		cl := cc.(*ast.CaseClause)
		if cl.List == nil {
			hasDefault = true
		}
		out = joinStates(out, a.block(cl.Body, st))
	}
	if !hasDefault {
		out = joinStates(out, st)
	}
	return joinStates(out, c.brk)
}
