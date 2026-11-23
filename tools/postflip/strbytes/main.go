// Command strbytes is a post-flip step: the positions that a function computes in one of its
// strings are byte offsets instead of UTF-16 unit indexes, where nothing can tell the
// difference.
//
// A Java String is a sequence of UTF-16 units and every int position in it counts units; the
// transpiled Go keeps the string as WTF-8 bytes, and String_length / String_charAt /
// String_substring / String_indexOf ... translate unit indexes into byte offsets by decoding
// the string from its start (or a checkpoint) on each call. A function that only scans a
// string for ASCII characters and cuts it where it found them does not need units: at a
// character boundary the unit index and the byte offset name the same place, a byte of a
// longer sequence is never ASCII, and an ASCII char is one unit and one byte.
//
// For a function F and a string variable s of F (a parameter or a local variable not assigned
// once a position in it has been computed), the step takes the int32 local variables that hold
// positions in s (those given a position, and those used as one) and shows, by an abstract
// interpretation of F, that each position F computes is, at every point, one of:
//
//	in:   a character boundary before the end (unit index u, byte offset of unit u),
//	end:  the end (length(), len(s)),
//	past: past the end (length()+k, len(s)+k),
//	neg:  a negative number (the same in both),
//	prev: t-1 of a boundary t (the last unit / byte of the character before t; only read and
//	      compared with ASCII chars),
//	mid:  at the head of a scan loop (below), anywhere in the stretch of non-ASCII characters
//	      before the next ASCII character (in both, the same next ASCII character).
//
// Positions start as 0 (in or end), length() (end), the result of indexOf / lastIndexOf of an
// ASCII char or String constant (neg -1, or in, with the needle's chars ASCII), and move by a
// constant k: from in by k when the k chars there are known to be ASCII (a test charAt(s,
// t + j) == C of an ASCII char C, a switch on it with ASCII cases, an ASCII-only predicate,
// the needle of the indexOf that found t), from end into past, from neg -1 to 0. Moving by one
// over a char not known to be ASCII is only allowed in a scan loop: a loop whose condition
// tests i < length() and that adds one to i at the end of each iteration, whose body does
// nothing (assigns nothing, calls nothing, leaves the loop not) when the char at i is not ASCII.
// Then the Java loop and the byte loop do the same thing at each ASCII char, in the same order,
// at the same position, and nothing at the other units and bytes; the byte loop only does more
// iterations that do nothing.
//
// Every char read from s at such a position must be used only where all non-ASCII units give
// the same result as all bytes 0x80 and above: compared (==, !=) with an ASCII char, ordered
// against a constant up to 0x80, switched on with ASCII cases, passed to a predicate that is
// such a test (Ascii_isUpperCase ...), or looked up in an ASCII String constant with indexOf
// compared with 0 / -1. Every position must stay inside F (no call, field, result or
// arithmetic other than +/- a constant receives it) and be compared only with positions in s
// or constants 0 and below. Then:
//
//	String_length(s)              -> int32(len(s))
//	String_charAt(s, i)           -> s[i] (i in), byteAt_(s, i)
//	String_substring(s, a, b)     -> s[a:b] (0 <= a <= b <= len shown), cutBytes_(s, a, b)
//	String_substring_int(s, a)    -> s[a:] (a in or end), cutBytesFrom_(s, a)
//	String_indexOf(s, C)          -> strings.IndexByte(s, C) (and Index, LastIndexByte,
//	                                 LastIndex), indexByteFrom_ ... with a start
//	String_startsWith_String_int  -> hasPrefixAt_(s, p, off)
//
// The helpers (rt_strbytes.go) throw, for an offset out of range, what the String_ helper
// throws at the unit index that the offset stands for (a negative one is itself, a past one
// is length() + k), so even the failures are Java's.
//
// Calls of other functions that F's positions do not reach (a charAt(s, 0), a substring(s, 2)
// after tests of the first two chars) are converted the same way on their own.
//
// The rest are left and counted, by reason. Rerunning the step changes nothing (the rewritten
// calls are gone).
//
// usage: strbytes [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strconv"
	"strings"

	"postflip/rw"
)

var count = map[string]int{}

var verbose = os.Getenv("STRBYTES_VERBOSE") != ""

func note(what string, n ast.Node) {
	count[what]++
	if verbose {
		fmt.Fprintf(os.Stderr, "%s: %s\n", rw.Fset.Position(n.Pos()), what)
	}
}

func constInt(e ast.Expr) (int64, bool) {
	tv, ok := rw.Info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(tv.Value)
}

func constString(e ast.Expr) (string, bool) {
	tv, ok := rw.Info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

func asciiOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func isType(t types.Type, k types.BasicKind) bool {
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == k
}

func identVar(e ast.Expr) *types.Var {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	o := rw.Info.Uses[id]
	if o == nil {
		o = rw.Info.Defs[id]
	}
	v, _ := o.(*types.Var)
	return v
}

// callName is the name of a call of a package-level function (String_charAt ...).
func callName(e ast.Expr) (string, *ast.CallExpr) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return "", nil
	}
	id, ok := c.Fun.(*ast.Ident)
	if !ok {
		return "", c
	}
	if _, ok := rw.Info.Uses[id].(*types.Func); !ok {
		return "", c
	}
	return id.Name, c
}

// conversion returns x when e is a conversion T(x) to an integer type.
func conversion(e ast.Expr) ast.Expr {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 1 {
		return nil
	}
	if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() {
		return c.Args[0]
	}
	return nil
}

// iife returns the function literal of an immediately called one (j2go's conditional
// expressions).
func iife(e ast.Expr) *ast.FuncLit {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return nil
	}
	fl, _ := rw.Unparen(c.Fun).(*ast.FuncLit)
	return fl
}

// ---- ASCII-only predicates

// blind holds the functions f(c uint16) bool whose body is return E, E a test of c that gives
// the same result for every unit 0x80 and above (and for every byte 0x80 and above): the
// result for those.
var blind = map[*types.Func]bool{}

func findBlind() {
	for changed := true; changed; {
		changed = false
		for _, f := range rw.Files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || fd.Recv != nil || fd.Body == nil || len(fd.Body.List) != 1 {
					continue
				}
				fn, _ := rw.Info.Defs[fd.Name].(*types.Func)
				if fn == nil {
					continue
				}
				if _, done := blind[fn]; done {
					continue
				}
				sig := fn.Type().(*types.Signature)
				if sig.Params().Len() != 1 || sig.Results().Len() != 1 || !isType(sig.Params().At(0).Type(), types.Uint16) || !isType(sig.Results().At(0).Type(), types.Bool) {
					continue
				}
				r, ok := fd.Body.List[0].(*ast.ReturnStmt)
				if !ok || len(r.Results) != 1 {
					continue
				}
				p := sig.Params().At(0)
				// every use of the parameter must be a test that s1 decides
				ok = true
				ast.Inspect(r.Results[0], func(n ast.Node) bool {
					if id, isId := n.(*ast.Ident); isId && rw.Info.Uses[id] == p {
						if _, decided := charTestUse(id, func(e ast.Expr) bool { return identVar(e) == p }); !decided {
							ok = false
						}
					}
					return ok
				})
				if !ok {
					continue
				}
				v := s1eval(r.Results[0], func(e ast.Expr) bool { return identVar(e) == p })
				if v == s1True || v == s1False {
					blind[fn] = v == s1True
					changed = true
				}
			}
		}
	}
}

// ---- evaluation for a char that is not ASCII

const (
	s1Unknown = iota // depends on something else
	s1True
	s1False
	s1Bad // not a pure test
)

// charTestUse returns, for a use x of a char (isChar(x)), the parent test and whether all
// units 0x80 and above give the same result there.
func charTestUse(x ast.Expr, isChar func(ast.Expr) bool) (ast.Expr, bool) {
	var e ast.Expr = x
	for {
		p, ok := rw.Up(e).(ast.Expr)
		if !ok {
			break
		}
		if conversion(p) != nil && rw.Unparen(conversion(p)) == rw.Unparen(e) {
			if t := rw.Info.Types[p].Type; isType(t, types.Int32) || isType(t, types.Uint16) || isType(t, types.Int64) || isType(t, types.Int) {
				e = p
				continue
			}
		}
		break
	}
	switch p := rw.Up(e).(type) {
	case *ast.BinaryExpr:
		o := p.Y
		if rw.Unparen(p.Y) == rw.Unparen(e) {
			o = p.X
		}
		k, ok := constInt(o)
		if !ok {
			return nil, false
		}
		switch p.Op {
		case token.EQL, token.NEQ:
			return p, k >= 0 && k < 0x80
		case token.LSS, token.GEQ, token.GTR, token.LEQ:
			return p, k >= 0 && k <= 0x80
		}
	case *ast.CallExpr:
		if name, _ := callName(p); name == "String_indexOf" && len(p.Args) == 2 && rw.Unparen(p.Args[1]) == rw.Unparen(e) {
			if cs, ok := constString(p.Args[0]); ok && asciiOnly(cs) {
				if b, ok := rw.Up(p).(*ast.BinaryExpr); ok {
					if k, ok := constInt(b.Y); ok && (k == 0 || k == -1) && rw.Unparen(b.X) == ast.Expr(p) {
						return b, true
					}
				}
			}
			return nil, false
		}
		if id, ok := p.Fun.(*ast.Ident); ok && len(p.Args) == 1 {
			if fn, ok := rw.Info.Uses[id].(*types.Func); ok {
				if _, ok := blind[fn]; ok {
					return p, true
				}
			}
		}
	}
	return nil, false
}

// s1eval evaluates condition e for a char (isChar) that is not ASCII.
func s1eval(e ast.Expr, isChar func(ast.Expr) bool) int {
	e = rw.Unparen(e)
	switch x := e.(type) {
	case *ast.UnaryExpr:
		if x.Op == token.NOT {
			switch v := s1eval(x.X, isChar); v {
			case s1True:
				return s1False
			case s1False:
				return s1True
			default:
				return v
			}
		}
		return s1Bad
	case *ast.BinaryExpr:
		switch x.Op {
		case token.LAND, token.LOR:
			a, b := s1eval(x.X, isChar), s1eval(x.Y, isChar)
			if a == s1Bad || b == s1Bad {
				return s1Bad
			}
			stop, go_ := s1False, s1True
			if x.Op == token.LOR {
				stop, go_ = s1True, s1False
			}
			if a == stop {
				return stop
			}
			if a == go_ {
				return b
			}
			if b == stop {
				return s1Unknown // depends on a
			}
			return s1Unknown
		}
		// a test of the char
		for _, side := range []ast.Expr{x.X, x.Y} {
			if ch := charOperand(side, isChar); ch != nil {
				o := x.Y
				if side == x.Y {
					o = x.X
				}
				k, ok := constInt(o)
				if !ok {
					return s1Bad
				}
				op := x.Op
				if side == x.Y { // k op c: mirror
					op = map[token.Token]token.Token{token.EQL: token.EQL, token.NEQ: token.NEQ, token.LSS: token.GTR, token.GTR: token.LSS, token.LEQ: token.GEQ, token.GEQ: token.LEQ}[op]
				}
				switch op {
				case token.EQL:
					if k < 0x80 {
						return s1False
					}
				case token.NEQ:
					if k < 0x80 {
						return s1True
					}
				case token.LSS:
					if k <= 0x80 {
						return s1False
					}
				case token.LEQ:
					if k < 0x80 {
						return s1False
					}
				case token.GTR:
					if k < 0x80 {
						return s1True
					}
				case token.GEQ:
					if k <= 0x80 {
						return s1True
					}
				}
				return s1Bad
			}
		}
		// String_indexOf(ASCII, c) compared with 0 / -1
		if name, c := callName(x.X); name == "String_indexOf" && len(c.Args) == 2 && charOperand(c.Args[1], isChar) != nil {
			if cs, ok := constString(c.Args[0]); ok && asciiOnly(cs) {
				if k, ok := constInt(x.Y); ok {
					found := false // a non-ASCII char is not in an ASCII string: -1
					_ = found
					switch {
					case x.Op == token.LSS && k == 0, x.Op == token.EQL && k == -1, x.Op == token.LEQ && k == -1:
						return s1True
					case x.Op == token.GEQ && k == 0, x.Op == token.NEQ && k == -1, x.Op == token.GTR && k == -1:
						return s1False
					}
				}
			}
			return s1Bad
		}
		if pure(x) && !mentions(x, isChar) {
			return s1Unknown
		}
		return s1Bad
	case *ast.CallExpr:
		if id, ok := x.Fun.(*ast.Ident); ok && len(x.Args) == 1 && charOperand(x.Args[0], isChar) != nil {
			if fn, ok := rw.Info.Uses[id].(*types.Func); ok {
				if r, ok := blind[fn]; ok {
					if r {
						return s1True
					}
					return s1False
				}
			}
			return s1Bad
		}
		if pure(x) && !mentions(x, isChar) {
			return s1Unknown
		}
		return s1Bad
	case *ast.Ident:
		if pure(x) {
			return s1Unknown
		}
	}
	return s1Bad
}

// charOperand returns e (without conversions) when it is the char.
func charOperand(e ast.Expr, isChar func(ast.Expr) bool) ast.Expr {
	for {
		e = rw.Unparen(e)
		if isChar(e) {
			return e
		}
		x := conversion(e)
		if x == nil {
			return nil
		}
		e = x
	}
}

func mentions(e ast.Expr, isChar func(ast.Expr) bool) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if x, ok := n.(ast.Expr); ok && isChar(x) {
			found = true
		}
		return !found
	})
	return found
}

// pure reports whether evaluating e has no effect and cannot throw: local variables,
// constants, comparisons and logic, len and String_length.
func pure(e ast.Expr) bool {
	ok := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case nil:
		case *ast.Ident, *ast.BasicLit, *ast.ParenExpr:
		case *ast.BinaryExpr:
			switch x.Op {
			case token.QUO, token.REM:
				ok = false
			}
		case *ast.UnaryExpr:
			if x.Op != token.NOT && x.Op != token.SUB {
				ok = false
			}
		case *ast.CallExpr:
			if conversion(x) != nil {
				return true
			}
			if id, isId := x.Fun.(*ast.Ident); isId {
				if _, b := rw.Info.Uses[id].(*types.Builtin); b && id.Name == "len" {
					return true
				}
				if id.Name == "String_length" {
					return true
				}
				if fn, isFn := rw.Info.Uses[id].(*types.Func); isFn {
					if _, bl := blind[fn]; bl {
						return true
					}
				}
			}
			ok = false
		default:
			ok = false
		}
		return ok
	})
	return ok
}

// ---- abstract positions

type kind uint8

const (
	kNeg kind = 1 << iota
	kIn
	kEnd
	kPast
	kMid
	kPrev
	kGone // a position in a value the string no longer has
)

const consistent = kNeg | kIn | kEnd | kPast

const ascBias = 16

// val is what is known of a position: its kinds, its value when it is a known negative
// number, and (bit ascBias+j) the offsets j at which the char is known to be ASCII.
type val struct {
	k     kind
	neg   int64
	negEx bool
	asc   uint32
}

func (v val) only(ks kind) bool { return v.k != 0 && v.k&^ks == 0 }

func bit(j int) uint32 {
	if j < -ascBias || j >= 32-ascBias {
		return 0
	}
	return 1 << uint(j+ascBias)
}

func (v val) ascAt(j int) bool { b := bit(j); return b != 0 && v.asc&b != 0 }

func shift(asc uint32, k int) uint32 { // offsets j become j-k
	if k > 0 {
		return asc >> uint(k)
	}
	return asc << uint(-k)
}

func join(a, b val) val {
	if a.k == 0 {
		return b
	}
	if b.k == 0 {
		return a
	}
	// the facts about chars hold for the boundaries (and say nothing of the other kinds)
	aa, ba := a.asc, b.asc
	if a.k&(kIn|kEnd|kMid|kPrev) == 0 {
		aa = ^uint32(0)
	}
	if b.k&(kIn|kEnd|kMid|kPrev) == 0 {
		ba = ^uint32(0)
	}
	r := val{k: a.k | b.k, asc: aa & ba}
	if r.k&(kIn|kEnd|kMid|kPrev) == 0 {
		r.asc = 0
	}
	switch {
	case a.k&kNeg != 0 && b.k&kNeg != 0:
		r.neg, r.negEx = a.neg, a.negEx && b.negEx && a.neg == b.neg
	case a.k&kNeg != 0:
		r.neg, r.negEx = a.neg, a.negEx
	case b.k&kNeg != 0:
		r.neg, r.negEx = b.neg, b.negEx
	}
	return r
}

type fail struct {
	why string
	at  ast.Node
}

func bail(why string, at ast.Node) { panic(fail{why, at}) }

// add moves position v by k.
func add(v val, k int64, zero val, at ast.Node) val {
	if k == 0 {
		return v
	}
	var r val
	if v.k&kNeg != 0 {
		if !v.negEx {
			if k > 0 {
				bail("a position that may be negative is moved forward", at)
			}
			r = join(r, val{k: kNeg})
		} else if n := v.neg + k; n < 0 {
			r = join(r, val{k: kNeg, neg: n, negEx: true})
		} else if n == 0 {
			r = join(r, zero)
		} else {
			bail("a negative position is moved past 0", at)
		}
	}
	if v.k&kPrev != 0 {
		if k != 1 {
			bail("a position before a char not known to be ASCII is moved", at)
		}
		r = join(r, val{k: kIn | kEnd}) // the boundary it is before
	}
	if v.k&kMid != 0 {
		bail("a scan position is moved", at)
	}
	if v.k&kPast != 0 {
		if k < 0 {
			bail("a position past the end is moved back", at)
		}
		r = join(r, val{k: kPast})
	}
	if ie := v.k & (kIn | kEnd); ie != 0 {
		// the known ASCII chars it moves over
		run := 0
		if k > 0 {
			for run < int(k) && v.ascAt(run) {
				run++
			}
		} else {
			for run < int(-k) && v.ascAt(-1-run) {
				run++
			}
		}
		switch {
		case k > 0 && run == int(k):
			r = join(r, val{k: kIn | kEnd, asc: shift(v.asc, int(k))})
		case k < 0 && run == int(-k):
			r = join(r, val{k: kIn, asc: shift(v.asc, int(k))})
		case k > 0 && run == int(k)-1:
			// over ASCII chars and then one more: inside it or past it
			if run > 0 || ie&kIn != 0 {
				r = join(r, val{k: kMid})
			}
			if run == 0 && ie&kEnd != 0 {
				r = join(r, val{k: kPast})
			}
		case k < 0 && run == int(-k)-1:
			r = join(r, val{k: kPrev})
		default:
			bail("a position moves over chars not known to be ASCII", at)
		}
	}
	return r
}

// ---- one string of one function

type cpos struct {
	base *types.Var
	off  int
}

type state struct {
	dead  bool
	v     map[*types.Var]val
	chars map[*types.Var]cpos
}

func (st *state) copy() *state {
	n := &state{dead: st.dead, v: map[*types.Var]val{}, chars: map[*types.Var]cpos{}}
	for k, x := range st.v {
		n.v[k] = x
	}
	for k, x := range st.chars {
		n.chars[k] = x
	}
	return n
}

func joinStates(a, b *state) *state {
	if a == nil || a.dead {
		if b == nil {
			return &state{dead: true}
		}
		return b
	}
	if b == nil || b.dead {
		return a
	}
	n := &state{v: map[*types.Var]val{}, chars: map[*types.Var]cpos{}}
	for k, x := range a.v {
		n.v[k] = join(x, b.v[k])
	}
	for k, x := range b.v {
		if _, ok := a.v[k]; !ok {
			n.v[k] = x
		}
	}
	for k, x := range a.chars {
		if y, ok := b.chars[k]; ok && y == x {
			n.chars[k] = x
		}
	}
	return n
}

func sameState(a, b *state) bool {
	if a.dead != b.dead || len(a.v) != len(b.v) || len(a.chars) != len(b.chars) {
		return false
	}
	for k, x := range a.v {
		if y, ok := b.v[k]; !ok || x != y {
			return false
		}
	}
	for k, x := range a.chars {
		if y, ok := b.chars[k]; !ok || x != y {
			return false
		}
	}
	return true
}

// site is a String_ call on s with the positions it received (joined over the analysis).
type site struct {
	name    string
	call    *ast.CallExpr
	args    []val
	visited bool
	scanOK  bool // a charAt that may be at a mid position, in a scan loop condition
	bad     string
}

type ctl struct {
	node  ast.Node
	label string
	brk   *state
	cont  *state
	loop  bool
}

type frame struct {
	rets *state
	val  val
	pos  bool // the literal returns a position
}

type an struct {
	fd    *ast.FuncDecl
	s     *types.Var
	iv    map[*types.Var]bool
	cv    map[*types.Var]bool
	zero  *types.Var
	end   *types.Var
	sites map[*ast.CallExpr]*site
	ctls  []*ctl
	frams []*frame
	scan  map[ast.Node]*types.Var // scan loops and their index variable
	scanW map[ast.Node]string     // why a loop is not a scan loop
	useIV bool
	ivBad   string
	ivBadAt ast.Node
	scanCV  *types.Var // while walking the condition of a scan loop: its index variable
	files   map[*ast.File]bool
}

var stringOps = map[string]bool{
	"String_length": true, "String_charAt": true, "String_substring": true, "String_substring_int": true,
	"String_indexOf": true, "String_indexOf_int_int": true, "String_indexOf_String": true, "String_indexOf_String_int": true,
	"String_lastIndexOf": true, "String_lastIndexOf_int_int": true, "String_lastIndexOf_String": true, "String_lastIndexOf_String_int": true,
	"String_startsWith_String_int": true,
}

// position results
var posOps = map[string]bool{
	"String_length": true, "String_indexOf": true, "String_indexOf_int_int": true, "String_indexOf_String": true, "String_indexOf_String_int": true,
	"String_lastIndexOf": true, "String_lastIndexOf_int_int": true, "String_lastIndexOf_String": true, "String_lastIndexOf_String_int": true,
}

// opOn returns the String_ op of e when it is a call on s.
func (a *an) opOn(e ast.Expr) (string, *ast.CallExpr) {
	name, c := callName(e)
	if !stringOps[name] || len(c.Args) == 0 || identVar(c.Args[0]) != a.s {
		return "", nil
	}
	return name, c
}

// needle returns the ASCII needle of an indexOf-like op (a char as a one-char string).
func needle(name string, c *ast.CallExpr) (string, bool) {
	if len(c.Args) < 2 {
		return "", false
	}
	if strings.Contains(name, "_String") {
		cs, ok := constString(c.Args[1])
		return cs, ok && cs != "" && asciiOnly(cs)
	}
	k, ok := constInt(c.Args[1])
	return string(rune(k)), ok && k >= 0 && k < 0x80
}

// isPosExpr reports whether e is a position expression: an index variable, a constant, a
// position op on s, p +/- constant, or a conditional literal returning positions.
func (a *an) isPosExpr(e ast.Expr) bool {
	e = rw.Unparen(e)
	if _, ok := constInt(e); ok {
		return true
	}
	if v := identVar(e); v != nil {
		return a.iv[v]
	}
	if name, c := a.opOn(e); c != nil {
		return posOps[name]
	}
	switch x := e.(type) {
	case *ast.BinaryExpr:
		if x.Op == token.ADD || x.Op == token.SUB {
			if _, ok := constInt(x.Y); ok {
				return a.isPosExpr(x.X)
			}
			if _, ok := constInt(x.X); ok && x.Op == token.ADD {
				return a.isPosExpr(x.Y)
			}
		}
	case *ast.CallExpr:
		if y := conversion(x); y != nil && isType(rw.Info.Types[x].Type, types.Int32) {
			if _, ok := constInt(y); ok {
				return true
			}
			return isType(rw.Info.Types[y].Type, types.Int32) && a.isPosExpr(y)
		}
		if fl := iife(x); fl != nil {
			ok := true
			n := 0
			ast.Inspect(fl.Body, func(m ast.Node) bool {
				if _, lit := m.(*ast.FuncLit); lit {
					return false
				}
				if r, isRet := m.(*ast.ReturnStmt); isRet {
					n++
					ok = ok && len(r.Results) == 1 && a.isPosExpr(r.Results[0])
				}
				return ok
			})
			return ok && n > 0
		}
	}
	return false
}

// mentionsPos reports whether e involves an index variable or a position op on s.
func (a *an) mentionsPos(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if x, ok := n.(ast.Expr); ok {
			if v := identVar(x); v != nil && a.iv[v] {
				found = true
			}
			if name, c := a.opOn(x); c != nil && posOps[name] {
				found = true
			}
		}
		return !found
	})
	return found
}

func isLocal(v *types.Var, fd *ast.FuncDecl) bool {
	return v != nil && !v.IsField() && v.Parent() != nil && v.Parent() != rw.Pkg.Scope() && fd.Pos() <= v.Pos() && v.Pos() < fd.End()
}

// assignments of v in fd (not counting its definition), and whether its address is taken or a
// non-immediate function literal assigns it.
func assigned(v *types.Var, fd *ast.FuncDecl) (as []ast.Node, bad bool) {
	for _, u := range rw.UsesOf[v] {
		if u.Pos() < fd.Pos() || u.Pos() >= fd.End() {
			continue
		}
		p := rw.Parent[u]
		switch x := p.(type) {
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				if l == ast.Expr(u) {
					as = append(as, x)
					if inClosure(x, fd) {
						bad = true
					}
				}
			}
		case *ast.IncDecStmt:
			as = append(as, x)
			if inClosure(x, fd) {
				bad = true
			}
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				bad = true
			}
		case *ast.RangeStmt:
			if x.Key == ast.Expr(u) || x.Value == ast.Expr(u) {
				as = append(as, x)
				bad = true
			}
		}
	}
	return
}

// inClosure reports whether n is inside a function literal of fd that is not immediately
// called.
func inClosure(n ast.Node, fd *ast.FuncDecl) bool {
	for p := rw.Parent[n]; p != nil && p != ast.Node(fd); p = rw.Parent[p] {
		if fl, ok := p.(*ast.FuncLit); ok {
			if c, ok := rw.Parent[fl].(*ast.CallExpr); !ok || c.Fun != ast.Expr(fl) || len(c.Args) != 0 {
				return true
			}
		}
	}
	return false
}

// ---- the candidates

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	findBlind()
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath(f), "g_") {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			function(fd, f)
		}
	}
	charArrays()
	leftovers()
	addImports()
	var keys []string
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	done := 0
	for _, k := range keys {
		fmt.Printf("%5d %s\n", count[k], k)
		if !strings.HasPrefix(k, "left") && !strings.HasPrefix(k, "  ") {
			done += count[k]
		}
	}
	fmt.Printf("strbytes: %d String_ calls at byte offsets\n", done)
	if !report {
		fmt.Printf("strbytes: %d files changed\n", rw.SaveChecked())
	}
}

func filepath(f *ast.File) string {
	n := rw.Fset.File(f.Pos()).Name()
	return n[strings.LastIndex(n, "/")+1:]
}

var fileOf = map[*ast.CallExpr]*ast.File{}

// function converts the strings of fd.
func function(fd *ast.FuncDecl, f *ast.File) {
	// the strings with ops in fd, in order of first use
	var strs []*types.Var
	seen := map[*types.Var]bool{}
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		name, c := callName(asExpr(n))
		if stringOps[name] && len(c.Args) > 0 {
			fileOf[c] = f
			if v := identVar(c.Args[0]); v != nil && !seen[v] {
				seen[v] = true
				strs = append(strs, v)
			}
		}
		return true
	})
	for _, s := range strs {
		if !isType(s.Type(), types.String) || !(isLocal(s, fd) || isParam(s, fd)) {
			continue
		}
		a := &an{fd: fd, s: s}
		if why := a.stable(); why != "" {
			a.leaveAll("the string variable " + why)
			continue
		}
		a.run()
	}
}

func asExpr(n ast.Node) ast.Expr {
	e, _ := n.(ast.Expr)
	return e
}

func isParam(v *types.Var, fd *ast.FuncDecl) bool {
	for _, fl := range fd.Type.Params.List {
		for _, n := range fl.Names {
			if rw.Info.Defs[n] == v {
				return true
			}
		}
	}
	return false
}

// stable checks that s does not change once a position in it is computed.
func (a *an) stable() string {
	if _, bad := assigned(a.s, a.fd); bad {
		return "has its address taken or is assigned in a function literal"
	}
	return ""
}

// ops returns the String_ calls on s in fd outside closures.
func (a *an) ops() []*ast.CallExpr {
	var out []*ast.CallExpr
	ast.Inspect(a.fd.Body, func(n ast.Node) bool {
		if _, c := a.opOn(asExpr(n)); c != nil {
			out = append(out, c)
		}
		return true
	})
	return out
}

func (a *an) leaveAll(why string) {
	for _, c := range a.ops() {
		leftWhy[c] = why
	}
}

// leftWhy is the reason each call that is not rewritten stays (the last attempt's).
var leftWhy = map[*ast.CallExpr]string{}

func (a *an) run() {
	a.useIV = true
	why := a.collectIV()
	if why == "" {
		why = a.collectChars()
	}
	if why == "" {
		why = a.interpret()
	}
	if why == "" {
		if !a.convert() {
			why = "a call with positions cannot be converted"
		}
	}
	if why != "" {
		// convert what does not involve the index variables
		for _, c := range a.ops() {
			leftWhy[c] = why
		}
		a.useIV = false
		a.iv = map[*types.Var]bool{}
		if a.collectChars() == "" && a.interpret() == "" {
			a.convert()
		}
	}
}

// collectIV finds the index variables and checks that every use keeps them inside fd.
func (a *an) collectIV() string {
	a.iv = map[*types.Var]bool{}
	cand := func(v *types.Var) bool {
		return v != nil && isLocal(v, a.fd) && isType(v.Type(), types.Int32) && !isParam(v, a.fd)
	}
	type asg struct {
		lhs *types.Var
		rhs ast.Expr
		n   ast.Node
	}
	var asgs []asg
	ast.Inspect(a.fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i, l := range x.Lhs {
					if v := identVar(l); v != nil {
						asgs = append(asgs, asg{v, x.Rhs[i], x})
					}
				}
			}
		case *ast.ValueSpec:
			for i, nm := range x.Names {
				if i < len(x.Values) {
					if v, ok := rw.Info.Defs[nm].(*types.Var); ok {
						asgs = append(asgs, asg{v, x.Values[i], x})
					}
				}
			}
		}
		return true
	})
	// seeds: variables in index arguments
	for _, c := range a.ops() {
		for _, arg := range c.Args[1:] {
			ast.Inspect(arg, func(n ast.Node) bool {
				if v := identVar(asExpr(n)); v != nil && isType(v.Type(), types.Int32) {
					if !cand(v) {
						if isParam(v, a.fd) {
							a.ivBad = "a position is a parameter"
						} else {
							a.ivBad = "a position is not a local variable"
						}
						a.ivBadAt = n
					}
					a.iv[v] = true
				}
				if _, lit := n.(*ast.FuncLit); lit && iife(asExpr(rw.Parent[n])) == nil {
					return false
				}
				return true
			})
		}
	}
	for changed := true; changed; {
		changed = false
		for _, x := range asgs {
			if !isType(x.lhs.Type(), types.Int32) {
				continue
			}
			if !a.iv[x.lhs] && a.mentionsPos(x.rhs) {
				if !cand(x.lhs) {
					return "a position is stored in " + kindOf(x.lhs)
				}
				a.iv[x.lhs] = true
				changed = true
			}
			if a.iv[x.lhs] {
				ast.Inspect(x.rhs, func(n ast.Node) bool {
					if v := identVar(asExpr(n)); v != nil && isType(v.Type(), types.Int32) && !a.iv[v] && rw.Info.Uses[asIdent(n)] != nil {
						if _, isConst := rw.Info.Uses[asIdent(n)].(*types.Const); !isConst {
							if !cand(v) {
								a.ivBad, a.ivBadAt = "a position comes from a parameter, field or package variable", n
							}
							a.iv[v] = true
							changed = true
						}
					}
					return true
				})
			}
		}
	}
	if a.ivBad != "" {
		return a.ivBad
	}
	if len(a.iv) == 0 {
		return ""
	}
	// every use of an index variable
	for v := range a.iv {
		as, bad := assigned(v, a.fd)
		_ = as
		if bad {
			return "a position variable has its address taken or is assigned in a function literal"
		}
		for _, u := range rw.UsesOf[v] {
			if u.Pos() < a.fd.Pos() || u.Pos() >= a.fd.End() {
				continue
			}
			if inClosure(u, a.fd) {
				return "a position is used in a function literal"
			}
			if why := a.useOK(u); why != "" {
				return why
			}
		}
	}
	// every position op result on s is used as a position (or is length() used otherwise)
	for _, c := range a.ops() {
		name := ""
		name, _ = callName(c)
		if !posOps[name] || name == "String_length" {
			continue
		}
		if why := a.useOK(c); why != "" {
			if a.mentionsPos2(c) {
				return why
			}
		}
	}
	return ""
}

// mentionsPos2 reports whether an op's arguments involve index variables.
func (a *an) mentionsPos2(c *ast.CallExpr) bool {
	for _, arg := range c.Args[1:] {
		if a.mentionsPos(arg) {
			return true
		}
	}
	return false
}

func asIdent(n ast.Node) *ast.Ident {
	id, _ := n.(*ast.Ident)
	return id
}

func kindOf(v *types.Var) string {
	switch {
	case v.IsField():
		return "a field"
	case v.Parent() == rw.Pkg.Scope():
		return "a package variable"
	case !isType(v.Type(), types.Int32):
		return "a variable of another type"
	}
	return "a parameter or a variable of an enclosing function"
}

// useOK checks the use of a position expression part e.
func (a *an) useOK(e ast.Expr) string {
	var top ast.Expr = e
	for {
		p := rw.Parent[top]
		switch x := p.(type) {
		case *ast.ParenExpr:
			top = x
			continue
		case *ast.BinaryExpr:
			if (x.Op == token.ADD || x.Op == token.SUB) && a.isPosExpr(x) {
				top = x
				continue
			}
		case *ast.CallExpr:
			if conversion(x) != nil && isType(rw.Info.Types[x].Type, types.Int32) {
				top = x
				continue
			}
		case *ast.ReturnStmt:
			// the result of a conditional literal
			if fl := enclosingLit(x); fl != nil {
				if c, ok := rw.Parent[fl].(*ast.CallExpr); ok && iife(c) == fl && a.isPosExpr(c) {
					top = c
					continue
				}
			}
		}
		break
	}
	switch p := rw.Parent[top].(type) {
	case *ast.CallExpr:
		if name, c := a.opOn(p); c != nil {
			for i, arg := range c.Args {
				if arg == top && i > 0 {
					if (strings.Contains(name, "dexOf") || name == "String_startsWith_String_int") && i == 1 {
						return "a position is the char or String searched for"
					}
					return ""
				}
			}
		}
		if name, _ := callName(p); name != "" {
			return "a position is passed to " + name
		}
		return "a position is passed to a method or function value"
	case *ast.BinaryExpr:
		switch p.Op {
		case token.EQL, token.NEQ, token.LSS, token.LEQ, token.GTR, token.GEQ:
			o := p.X
			if p.X == top {
				o = p.Y
			}
			if a.isPosExpr(o) {
				return ""
			}
			return "a position is compared with a number that is not a position in the string"
		}
		return "a position is used in arithmetic (" + p.Op.String() + ")"
	case *ast.AssignStmt:
		for _, l := range p.Lhs {
			if l == top {
				if p.Tok == token.ADD_ASSIGN || p.Tok == token.SUB_ASSIGN {
					if len(p.Rhs) == 1 {
						if _, ok := constInt(p.Rhs[0]); ok {
							return ""
						}
					}
					return "a position is moved by a number that is not a constant"
				}
				if p.Tok == token.ASSIGN || p.Tok == token.DEFINE {
					return ""
				}
				return "a position is changed by " + p.Tok.String()
			}
		}
		for i, r := range p.Rhs {
			if r == top && len(p.Lhs) == len(p.Rhs) && (p.Tok == token.ASSIGN || p.Tok == token.DEFINE) {
				if v := identVar(p.Lhs[i]); v != nil && a.iv[v] {
					return ""
				}
				return "a position is stored in something other than a position variable"
			}
		}
		return "a position is assigned in an unusual way"
	case *ast.ValueSpec:
		for i, val := range p.Values {
			if val == top {
				if v, ok := rw.Info.Defs[p.Names[i]].(*types.Var); ok && a.iv[v] {
					return ""
				}
			}
		}
		return "a position is stored in something other than a position variable"
	case *ast.IncDecStmt:
		return ""
	case *ast.ReturnStmt:
		return "a position is returned"
	case *ast.IndexExpr:
		return "a position indexes an array"
	case *ast.SwitchStmt:
		return "a position is switched on"
	case *ast.CompositeLit, *ast.KeyValueExpr:
		return "a position is stored in a composite value"
	}
	return fmt.Sprintf("a position is used in a %T", rw.Parent[top])
}

func enclosingLit(n ast.Node) *ast.FuncLit {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch x := p.(type) {
		case *ast.FuncLit:
			return x
		case *ast.FuncDecl:
			return nil
		}
	}
	return nil
}

// collectChars finds the char variables and checks that every char read from s at a
// position is used only in tests that do not tell non-ASCII units apart.
func (a *an) collectChars() string {
	a.cv = map[*types.Var]bool{}
	a.sites = map[*ast.CallExpr]*site{}
	for _, c := range a.ops() {
		name, _ := callName(c)
		st := &site{name: name, call: c}
		a.sites[c] = st
		if !a.useIV {
			for _, arg := range c.Args[1:] {
				if a.mentionsVar(arg) {
					st.bad = leftWhy[c]
					if st.bad == "" {
						st.bad = "the call's positions are not followed"
					}
				}
			}
		}
		if name != "String_charAt" {
			continue
		}
		if why := a.charUse(c); why != "" {
			st.bad = why
			if a.useIV && a.mentionsPos2(c) {
				return why
			}
		}
	}
	return ""
}

// mentionsVar reports whether e reads an int variable (not a constant).
func (a *an) mentionsVar(e ast.Expr) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok {
			if v, ok := rw.Info.Uses[id].(*types.Var); ok && !isType(v.Type(), types.String) {
				found = true
			}
		}
		if _, ok := n.(*ast.FuncLit); ok {
			found = true
		}
		return !found
	})
	return found
}

func (a *an) isCharOfS(e ast.Expr) bool {
	e = rw.Unparen(e)
	if name, c := a.opOn(e); c != nil && name == "String_charAt" {
		return true
	}
	if v := identVar(e); v != nil && a.cv[v] {
		return true
	}
	return false
}

// charUse checks the use of a char read from s (a charAt call).
func (a *an) charUse(c *ast.CallExpr) string {
	if _, ok := charTestUse(c, a.isCharOfS); ok {
		return ""
	}
	// stored in a char variable
	var v *types.Var
	switch p := rw.Up(c).(type) {
	case *ast.ValueSpec:
		for i, val := range p.Values {
			if rw.Unparen(val) == ast.Expr(c) {
				v, _ = rw.Info.Defs[p.Names[i]].(*types.Var)
			}
		}
	case *ast.AssignStmt:
		if len(p.Lhs) == 1 && len(p.Rhs) == 1 && (p.Tok == token.DEFINE || p.Tok == token.ASSIGN) {
			v = identVar(p.Lhs[0])
		}
	case *ast.SwitchStmt:
		for _, cc := range p.Body.List {
			for _, x := range cc.(*ast.CaseClause).List {
				if k, ok := constInt(x); !ok || k < 0 || k >= 0x80 {
					return "a char is switched on with a case that is not an ASCII char"
				}
			}
		}
		return ""
	}
	if v == nil || !isLocal(v, a.fd) || !isType(v.Type(), types.Uint16) {
		return a.charUseWhy(c)
	}
	if a.cv[v] {
		return ""
	}
	a.cv[v] = true
	as, bad := assigned(v, a.fd)
	if bad {
		return "a char variable is assigned in a function literal"
	}
	for _, x := range as {
		as, ok := x.(*ast.AssignStmt)
		if !ok || len(as.Rhs) != 1 {
			return "a char variable is changed"
		}
		if !a.isCharOfS(as.Rhs[0]) {
			if k, ok := constInt(as.Rhs[0]); !ok || k < 0 || k >= 0x80 {
				return "a char variable is given another value"
			}
		}
	}
	for _, u := range rw.UsesOf[v] {
		if u.Pos() < a.fd.Pos() || u.Pos() >= a.fd.End() {
			continue
		}
		if p, ok := rw.Parent[u].(*ast.AssignStmt); ok && len(p.Lhs) == 1 && p.Lhs[0] == ast.Expr(u) {
			continue
		}
		if inClosure(u, a.fd) {
			return "a char is used in a function literal"
		}
		if _, ok := charTestUse(u, a.isCharOfS); ok {
			continue
		}
		if p, ok := rw.Parent[u].(*ast.SwitchStmt); ok && p.Tag == ast.Expr(u) {
			for _, cc := range p.Body.List {
				for _, x := range cc.(*ast.CaseClause).List {
					if k, ok := constInt(x); !ok || k < 0 || k >= 0x80 {
						return "a char is switched on with a case that is not an ASCII char"
					}
				}
			}
			continue
		}
		return a.charUseWhy(u)
	}
	return ""
}

func (a *an) charUseWhy(e ast.Expr) string {
	var x ast.Expr = e
	for {
		if p, ok := rw.Up(x).(ast.Expr); ok && conversion(p) != nil {
			x = p
			continue
		}
		break
	}
	switch p := rw.Up(x).(type) {
	case *ast.CallExpr:
		if name, _ := callName(p); name != "" {
			return "a char is passed to " + name
		}
		return "a char is passed to a method"
	case *ast.BinaryExpr:
		return "a char is compared with a value that is not an ASCII constant, or used in arithmetic"
	case *ast.ReturnStmt:
		return "a char is returned"
	}
	return "a char is used as a UTF-16 unit value"
}

// charArrays rewrites toCharArray of a String constant into the literal of its UTF-16 units:
// Java makes a new array on each call, as the composite literal does.
func charArrays() {
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath(f), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			name, c := callName(asExpr(n))
			if name != "String_toCharArray" || len(c.Args) != 1 {
				return true
			}
			cs, ok := constString(c.Args[0])
			if !ok || cs == "\x00\xff<null>\xff\x00" {
				if ok {
					return true
				}
				leftWhy[c] = "toCharArray of a String that is not a constant"
				return true
			}
			var units []string
			for _, r := range cs {
				if r >= 0x10000 || r == 0xFFFD || r >= 0x80 {
					leftWhy[c] = "toCharArray of a constant with non-ASCII chars"
					return true
				}
				units = append(units, strconvQuote(r))
			}
			note("toCharArray of an ASCII constant: the literal of its units", c)
			rw.Rewrite[c] = func() string { return "[]uint16{" + strings.Join(units, ", ") + "}" }
			return true
		})
	}
}

func strconvQuote(r rune) string { return strconv.QuoteRune(r) }

