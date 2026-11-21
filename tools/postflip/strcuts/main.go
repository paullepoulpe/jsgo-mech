// Command strcuts is a post-flip step: a String cut at a position that a test of the same
// string has shown to end an ASCII prefix or start an ASCII suffix (or to end a prefix equal
// to another string) is a Go slice of the string, and a char of it that a test has shown to
// exist after only ASCII chars, compared with an ASCII char, is its byte.
//
// The runtime keeps a String as WTF-8 and String_substring / String_charAt count UTF-16
// units, locating unit indexes by decoding the string. Where a test has shown what the string
// starts or ends with, the unit index and the byte index of the cut are the same position:
//
//   - after strings.HasPrefix(s, p) (Java's s.startsWith(p)), the first bytes of s are the
//     bytes of p and end where a character of s ends (UTF-8 is self-synchronizing), so the
//     first p.length() units of s are the units of p: s.substring(p.length()) is
//     s[len(p):] and s.substring(0, p.length()) is s[:len(p)]. When p is a constant whose
//     first k bytes are ASCII, the first k units of s are those bytes, so s.substring(j) and
//     s.substring(0, j) for a constant 0 <= j <= k are s[j:] and s[:j]. The test
//     s.charAt(0) == C of an ASCII char C is strings.HasPrefix(s, "C") for this purpose, and
//     a disjunction of such tests gives the shortest ASCII prefix they all give.
//   - after strings.HasSuffix(s, p) the same holds at the end: s.substring(0, s.length() -
//     p.length()) is s[:len(s)-len(p)], and s.substring(0, s.length() - j) for a constant
//     0 <= j <= (the ASCII bytes at the end of a constant p) is s[:len(s)-j].
//   - s.charAt(k) for a constant k, compared (== or !=) with an ASCII char constant or as the
//     tag of a switch whose cases are all ASCII char constants, is s[k] when tests have shown
//     that unit k exists (s != "", len(s) > 0, s.length() > j for j >= k, a prefix test with a
//     longer constant) and that the k units before it are ASCII (k = 0, or an ASCII prefix of
//     at least k): unit k then starts at byte k, and is ASCII exactly when byte k is, when
//     they are equal; otherwise both are 0x80 or above and equal no ASCII char. Likewise
//     s.charAt(s.length() - 1) of a non-empty s is s[len(s)-1]: the last byte of a longer
//     sequence (and of a lone surrogate's) is 0x80 or above.
//
// None of these cuts and reads can be out of range, so neither Java's exception nor Go's can
// happen.
//
// A test applies to a use when the use is in the body of an if whose condition implies it,
// in the right operand of an && whose left operand implies it (of an || whose left operand
// implies its negation), or after an if statement of the same or an enclosing block whose
// condition implies its negation and whose body ends in return, break, continue or panic.
// s, and p when it is not a constant, are local variables or parameters whose address
// is not taken, that no function literal assigns, and that nothing assigns between the test
// and the use (nor in a loop around the use that does not also hold the test).
//
// Rerunning the step changes nothing (the rewritten calls are gone).
//
// usage: strcuts [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"postflip/rw"
)

var count = map[string]int{}

var verbose = os.Getenv("STRCUTS_VERBOSE") != ""

func note(what string, n ast.Node) {
	count[what]++
	if verbose {
		fmt.Fprintf(os.Stderr, "%s: %s\n", rw.Fset.Position(n.Pos()), what)
	}
}

func constString(e ast.Expr) (string, bool) {
	tv, ok := rw.Info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

func constInt(e ast.Expr) (int64, bool) {
	tv, ok := rw.Info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(tv.Value)
}

const nullstr = "\x00\xff<null>\xff\x00"

// ---- the function being rewritten

// fn holds, for the function declaration being rewritten, the variables its function
// literals assign or whose address it takes, and every assignment of each variable.
type fn struct {
	unstable map[types.Object]bool
	assigns  map[types.Object][]ast.Node // the statements that assign the variable
	loops    []ast.Node                  // for and range statements
}

var cur *fn

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

func analyse(fd *ast.FuncDecl) *fn {
	f := &fn{unstable: map[types.Object]bool{}, assigns: map[types.Object][]ast.Node{}}
	var inLit func(n ast.Node, lit bool)
	inLit = func(n ast.Node, lit bool) {
		ast.Inspect(n, func(x ast.Node) bool {
			mark := func(e ast.Expr, stmt ast.Node) {
				if o := objOf(e); o != nil {
					if lit {
						f.unstable[o] = true
					}
					f.assigns[o] = append(f.assigns[o], stmt)
				}
			}
			switch s := x.(type) {
			case *ast.FuncLit:
				if !lit {
					inLit(s.Body, true)
					return false
				}
			case *ast.AssignStmt:
				if s.Tok != token.DEFINE {
					for _, l := range s.Lhs {
						mark(l, s)
					}
				}
			case *ast.IncDecStmt:
				mark(s.X, s)
			case *ast.RangeStmt:
				if s.Tok != token.DEFINE {
					for _, l := range []ast.Expr{s.Key, s.Value} {
						if l != nil {
							mark(l, s)
						}
					}
				}
				f.loops = append(f.loops, s)
			case *ast.ForStmt:
				f.loops = append(f.loops, s)
			case *ast.UnaryExpr:
				if s.Op == token.AND {
					if o := objOf(s.X); o != nil {
						f.unstable[o] = true
					}
				}
			}
			return true
		})
	}
	inLit(fd.Body, false)
	return f
}

func contains(outer, inner ast.Node) bool {
	return outer.Pos() <= inner.Pos() && inner.End() <= outer.End()
}

// stable reports whether variable o (a local variable or parameter) holds the same value at
// use u as at the end of test g.
func stable(o types.Object, g, u ast.Node) bool {
	v, ok := o.(*types.Var)
	if !ok || v.IsField() || v.Parent() == rw.Pkg.Scope() || v.Parent() == nil || cur.unstable[o] {
		return false
	}
	for _, a := range cur.assigns[o] {
		if a.Pos() >= g.End() && a.End() <= u.Pos() {
			return false // between the test and the use
		}
		for _, l := range cur.loops {
			if contains(l, u) && contains(l, a) && !contains(l, g) {
				return false // in a loop that repeats the use without the test
			}
		}
	}
	return true
}

// ---- facts

const (
	prefix = iota
	suffix
	minLen
	has
)

// fact is what a test shows about string variable s: that it starts (ends) with p, where the
// first (last) n bytes of p are ASCII (p nil: only that many ASCII bytes are known), or that
// it has at least n UTF-16 units, or that it contains p.
type fact struct {
	kind int
	s    types.Object
	p    ast.Expr
	n    int
	test ast.Node
}

func leadingASCII(s string) int {
	n := 0
	for n < len(s) && s[n] < 0x80 {
		n++
	}
	return n
}

func trailingASCII(s string) int {
	n := 0
	for n < len(s) && s[len(s)-1-n] < 0x80 {
		n++
	}
	return n
}

// patternOK reports whether p is a constant String (not null) or a stable variable.
func patternOK(p ast.Expr) bool {
	if cs, ok := constString(p); ok {
		return cs != nullstr
	}
	return objOf(p) != nil
}

// facts returns what e (a condition) shows when it is true (pos) or false.
func facts(e ast.Expr, pos bool) []fact {
	e = rw.Unparen(e)
	switch x := e.(type) {
	case *ast.UnaryExpr:
		if x.Op == token.NOT {
			return facts(x.X, !pos)
		}
	case *ast.BinaryExpr:
		switch {
		case x.Op == token.LAND && pos, x.Op == token.LOR && !pos:
			return append(facts(x.X, pos), facts(x.Y, pos)...)
		case x.Op == token.LOR && pos, x.Op == token.LAND && !pos:
			return either(facts(x.X, pos), facts(x.Y, pos), e)
		}
		return compare(x, pos)
	case *ast.CallExpr:
		if !pos {
			return nil
		}
		s, ok := x.Fun.(*ast.SelectorExpr)
		if !ok || len(x.Args) != 2 {
			return nil
		}
		if p, ok := s.X.(*ast.Ident); !ok || p.Name != "strings" {
			return nil
		} else if _, isPkg := rw.Info.Uses[p].(*types.PkgName); !isPkg {
			return nil
		}
		so := objOf(x.Args[0])
		if so == nil || !patternOK(x.Args[1]) {
			return nil
		}
		cs, isConst := constString(x.Args[1])
		switch s.Sel.Name {
		case "HasPrefix":
			f := []fact{{kind: prefix, s: so, p: x.Args[1], n: leadingASCII(cs), test: e}}
			if isConst && cs != "" {
				f = append(f, fact{kind: minLen, s: so, n: 1, test: e})
			}
			return f
		case "Contains":
			return []fact{{kind: has, s: so, p: x.Args[1], test: e}}
		case "HasSuffix":
			f := []fact{{kind: suffix, s: so, p: x.Args[1], n: trailingASCII(cs), test: e}}
			if isConst && cs != "" {
				f = append(f, fact{kind: minLen, s: so, n: 1, test: e})
			}
			return f
		}
	}
	return nil
}

// either is what a disjunction of conditions with facts a and b shows: an ASCII prefix or
// suffix of s both show (the shorter), and non-emptiness both show.
func either(a, b []fact, test ast.Node) []fact {
	var out []fact
	for _, x := range a {
		for _, y := range b {
			if x.s != y.s || x.kind != y.kind {
				continue
			}
			switch x.kind {
			case minLen:
				out = append(out, fact{kind: minLen, s: x.s, n: min(x.n, y.n), test: test})
			case prefix, suffix:
				if n := min(x.n, y.n); n > 0 {
					out = append(out, fact{kind: x.kind, s: x.s, n: n, test: test})
				}
			}
		}
	}
	return out
}

// compare returns the facts of s.charAt(0) == C (an ASCII char), s != "" and len(s) > 0.
func compare(b *ast.BinaryExpr, pos bool) []fact {
	op := b.Op
	if !pos {
		op = map[token.Token]token.Token{token.EQL: token.NEQ, token.NEQ: token.EQL, token.LSS: token.GEQ, token.GEQ: token.LSS, token.GTR: token.LEQ, token.LEQ: token.GTR}[op]
	}
	x, y := rw.Unparen(b.X), rw.Unparen(b.Y)
	if _, ok := constInt(x); ok {
		x, y = y, x
		op = map[token.Token]token.Token{token.EQL: token.EQL, token.NEQ: token.NEQ, token.LSS: token.GTR, token.GTR: token.LSS, token.LEQ: token.GEQ, token.GEQ: token.LEQ}[op]
	} else if _, ok := constString(x); ok {
		x, y = y, x
	}
	// s.charAt(0) == C
	if name, c := rw.CallName(x); name == "String_charAt" && op == token.EQL {
		k, ok1 := constInt(c.Args[1])
		ch, ok2 := constInt(y)
		if so := objOf(c.Args[0]); so != nil && ok1 && ok2 && k == 0 && ch >= 0 && ch < 0x80 {
			return []fact{{kind: prefix, s: so, n: 1, test: b}, {kind: minLen, s: so, n: 1, test: b}}
		}
	}
	// s != ""
	if cs, ok := constString(y); ok && cs == "" && op == token.NEQ {
		if so := objOf(x); so != nil {
			return []fact{{kind: minLen, s: so, n: 1, test: b}}
		}
	}
	// len(s) > 0, len(s) != 0, len(s) >= 1; String_length(s) likewise
	if k, ok := constInt(y); ok {
		var arg ast.Expr
		units := false // String_length counts units; len bytes, which only tells emptiness
		if c, ok := x.(*ast.CallExpr); ok && len(c.Args) == 1 {
			if id, ok := c.Fun.(*ast.Ident); ok {
				if _, builtin := rw.Info.Uses[id].(*types.Builtin); builtin && id.Name == "len" {
					arg = c.Args[0]
				} else if id.Name == "String_length" {
					arg, units = c.Args[0], true
				}
			}
		}
		if arg != nil {
			if t := rw.Info.Types[arg].Type; t == nil || !isString(t) {
				return nil
			}
			n := int64(0)
			switch {
			case op == token.GTR && k >= 0:
				n = k + 1
			case op == token.GEQ, op == token.EQL:
				n = k
			case op == token.NEQ && k == 0:
				n = 1
			}
			if !units && n > 1 {
				n = 1
			}
			if so := objOf(arg); so != nil && n >= 1 {
				return []fact{{kind: minLen, s: so, n: int(n), test: b}}
			}
		}
	}
	return nil
}

func isString(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

// known returns the facts that hold at use u, from the tests around it.
func known(u ast.Node) []fact {
	var out []fact
	var child ast.Node = u
	for p := rw.Parent[u]; p != nil; child, p = p, rw.Parent[p] {
		switch x := p.(type) {
		case *ast.FuncLit, *ast.FuncDecl:
			return out
		case *ast.BinaryExpr:
			if child == ast.Node(x.Y) {
				switch x.Op {
				case token.LAND:
					out = append(out, facts(x.X, true)...)
				case token.LOR:
					out = append(out, facts(x.X, false)...)
				}
			}
		case *ast.ParenExpr:
		case *ast.IfStmt:
			if child == ast.Node(x.Body) {
				out = append(out, facts(x.Cond, true)...)
			} else if x.Else != nil && child == x.Else {
				out = append(out, facts(x.Cond, false)...)
			}
		case *ast.BlockStmt:
			for _, s := range x.List {
				if s == child {
					break
				}
				if is, ok := s.(*ast.IfStmt); ok && is.Init == nil && is.Else == nil && terminates(is.Body.List) {
					out = append(out, facts(is.Cond, false)...)
				}
			}
		case *ast.CaseClause, *ast.CommClause:
			for _, s := range caseBody(x) {
				if s == child {
					break
				}
				if is, ok := s.(*ast.IfStmt); ok && is.Init == nil && is.Else == nil && terminates(is.Body.List) {
					out = append(out, facts(is.Cond, false)...)
				}
			}
		}
	}
	return out
}

func caseBody(n ast.Node) []ast.Stmt {
	if c, ok := n.(*ast.CaseClause); ok {
		return c.Body
	}
	return n.(*ast.CommClause).Body
}

func terminates(l []ast.Stmt) bool {
	if len(l) == 0 {
		return false
	}
	switch x := l[len(l)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		// not goto: it may jump to a label between the test and the use
		return x.Tok == token.CONTINUE || x.Tok == token.BREAK
	case *ast.ExprStmt:
		if c, ok := x.X.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "panic" {
				return true
			}
		}
	}
	return false
}

// holds returns a fact of kind k about the string variable of expression s at use u, among
// facts fs, for which ok accepts it, with the variables involved unchanged since the test.
func holds(fs []fact, k int, s ast.Expr, u ast.Node, ok func(fact) bool) (fact, bool) {
	so := objOf(s)
	if so == nil {
		return fact{}, false
	}
	for _, f := range fs {
		if f.kind != k || f.s != so || !stable(so, f.test, u) || !ok(f) {
			continue
		}
		if f.p != nil {
			if po := objOf(f.p); po != nil {
				if _, isConst := po.(*types.Const); !isConst && !stable(po, f.test, u) {
					continue
				}
			}
		}
		return f, true
	}
	return fact{}, false
}

// samePattern reports whether q names what f.p names: the same variable, or a constant with
// the same value.
func samePattern(f fact, q ast.Expr) bool {
	if f.p == nil {
		return false
	}
	if a, ok := constString(f.p); ok {
		b, ok := constString(q)
		return ok && a == b
	}
	po := objOf(f.p)
	return po != nil && objOf(q) == po
}

// lengthOf returns q when e is String_length(q).
func lengthOf(e ast.Expr) ast.Expr {
	if name, c := rw.CallName(e); name == "String_length" && len(c.Args) == 1 {
		return c.Args[0]
	}
	return nil
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			cur = nil
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				name, _ := rw.CallName(c)
				if name != "String_substring" && name != "String_substring_int" && name != "String_charAt" {
					return true
				}
				if cur == nil {
					cur = analyse(fd)
					curFile, curDecl = f, fd
				}
				rewrite(name, c)
				return true
			})
		}
	}
	addImports()
	var keys []string
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	done := 0
	for _, k := range keys {
		fmt.Printf("%5d %s\n", count[k], k)
		done += count[k]
	}
	fmt.Printf("strcuts: %d String_ calls become slices or byte reads\n", done)
	if !report {
		fmt.Printf("strcuts: %d files changed\n", rw.SaveChecked())
	}
}

func rewrite(name string, c *ast.CallExpr) {
	a := c.Args
	s := a[0]
	if objOf(s) == nil {
		return
	}
	if name == "String_substring_int" {
		// s.substring(s.lastIndexOf(C) + 1) of an ASCII char C: after the match, or all of s
		// (-1 + 1 = 0) when there is none; the byte after an ASCII byte is the unit after it.
		if b, ok := rw.Unparen(a[1]).(*ast.BinaryExpr); ok && b.Op == token.ADD {
			if one, ok := constInt(b.Y); ok && one == 1 {
				if fn, _, ok := search(rw.Unparen(b.X), s); ok && isCharSearch(rw.Unparen(b.X)) {
					note("substring(indexOf(C) + 1) of an ASCII char C: s[strings.IndexByte(s, C)+1:]", c)
					needStrings[curFile] = true
					rw.Rewrite[c] = func() string { return rw.Render(s) + "[" + fn() + "+1:]" }
					return
				}
			}
		}
	}
	fs := known(c)
	if len(fs) == 0 {
		return
	}
	r := func(e ast.Expr) string { return rw.Render(e) }
	switch name {
	case "String_substring_int": // s.substring(x)
		x := a[1]
		if fn, q, ok := search(x, s); ok {
			if _, ok := holds(fs, has, s, c, func(f fact) bool { return samePattern(f, q) }); ok {
				note("substring(indexOf(p)) after contains(p): s[strings.Index(s, p):]", c)
				needStrings[curFile] = true
				rw.Rewrite[c] = func() string { return r(s) + "[" + fn() + ":]" }
				return
			}
		}
		if q := lengthOf(x); q != nil {
			if _, ok := holds(fs, prefix, s, c, func(f fact) bool { return samePattern(f, q) }); ok {
				note("substring(p.length()) after startsWith(p): s[len(p):]", c)
				rw.Rewrite[c] = func() string { return r(s) + "[len(" + r(q) + "):]" }
				return
			}
		}
		if k, ok := constInt(x); ok && k >= 0 {
			if _, ok := holds(fs, prefix, s, c, func(f fact) bool { return int64(f.n) >= k }); ok {
				note("substring(k) inside an ASCII prefix: s[k:]", c)
				rw.Rewrite[c] = func() string { return r(s) + "[" + index(x, k) + ":]" }
				return
			}
		}
	case "String_substring": // s.substring(0, y)
		if k, ok := constInt(a[1]); !ok || k != 0 {
			return
		}
		y := rw.Unparen(a[2])
		if fn, q, ok := search(y, s); ok {
			if _, ok := holds(fs, has, s, c, func(f fact) bool { return samePattern(f, q) }); ok {
				note("substring(0, indexOf(p)) after contains(p): s[:strings.Index(s, p)]", c)
				needStrings[curFile] = true
				rw.Rewrite[c] = func() string { return r(s) + "[:" + fn() + "]" }
				return
			}
		}
		if q := lengthOf(y); q != nil {
			if _, ok := holds(fs, prefix, s, c, func(f fact) bool { return samePattern(f, q) }); ok {
				note("substring(0, p.length()) after startsWith(p): s[:len(p)]", c)
				rw.Rewrite[c] = func() string { return r(s) + "[:len(" + r(q) + ")]" }
				return
			}
		}
		if k, ok := constInt(y); ok && k >= 0 {
			if _, ok := holds(fs, prefix, s, c, func(f fact) bool { return int64(f.n) >= k }); ok {
				note("substring(0, k) inside an ASCII prefix: s[:k]", c)
				rw.Rewrite[c] = func() string { return r(s) + "[:" + index(y, k) + "]" }
				return
			}
		}
		// s.substring(0, s.length() - z)
		b, ok := y.(*ast.BinaryExpr)
		if !ok || b.Op != token.SUB {
			return
		}
		if l := lengthOf(rw.Unparen(b.X)); l == nil || objOf(l) != objOf(s) {
			return
		}
		z := rw.Unparen(b.Y)
		if q := lengthOf(z); q != nil {
			if _, ok := holds(fs, suffix, s, c, func(f fact) bool { return samePattern(f, q) }); ok {
				note("substring(0, length() - p.length()) after endsWith(p): s[:len(s)-len(p)]", c)
				rw.Rewrite[c] = func() string { return r(s) + "[:len(" + r(s) + ")-len(" + r(q) + ")]" }
				return
			}
		}
		if k, ok := constInt(z); ok && k >= 0 {
			if _, ok := holds(fs, suffix, s, c, func(f fact) bool { return int64(f.n) >= k }); ok {
				note("substring(0, length() - k) inside an ASCII suffix: s[:len(s)-k]", c)
				rw.Rewrite[c] = func() string { return r(s) + "[:len(" + r(s) + ")-" + index(z, k) + "]" }
				return
			}
		}
	case "String_charAt": // s.charAt(k) or s.charAt(s.length() - 1) against ASCII chars
		if !asciiUse(c) {
			return
		}
		any := func(fact) bool { return true }
		if b, ok := rw.Unparen(a[1]).(*ast.BinaryExpr); ok && b.Op == token.SUB {
			// the last unit is ASCII exactly when the last byte is (a longer sequence ends
			// in a byte 0x80 or above)
			if l := lengthOf(rw.Unparen(b.X)); l != nil && objOf(l) == objOf(s) {
				if one, ok := constInt(b.Y); ok && one == 1 {
					if _, ok := holds(fs, minLen, s, c, any); ok {
						note("charAt(length() - 1) of a non-empty string, against ASCII chars: s[len(s)-1]", c)
						rw.Rewrite[c] = func() string { return r(s) + "[len(" + r(s) + ")-1]" }
					}
				}
			}
			return
		}
		k, ok := constInt(a[1])
		if !ok || k < 0 {
			return
		}
		// unit k exists, and the units before it are ASCII bytes, so it starts at byte k
		_, long := holds(fs, minLen, s, c, func(f fact) bool { return int64(f.n) > k })
		_, pre := holds(fs, prefix, s, c, func(f fact) bool { return int64(f.n) >= k })
		_, inside := holds(fs, prefix, s, c, func(f fact) bool { return int64(f.n) > k })
		if inside || (long && (k == 0 || pre)) {
			ks := strconv.FormatInt(k, 10)
			note("charAt(k) of a string with k ASCII units before unit k, against ASCII chars: s[k]", c)
			rw.Rewrite[c] = func() string { return r(s) + "[" + ks + "]" }
		}
	}
}

// index renders constant index e (of value k) for a slice: len(C) for int32(len(C)) (the
// length strnative writes for an ASCII constant C), else k.
func index(e ast.Expr, k int64) string {
	if c, ok := rw.Unparen(e).(*ast.CallExpr); ok && len(c.Args) == 1 {
		if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "int32" {
			if l, ok := rw.Unparen(c.Args[0]).(*ast.CallExpr); ok && len(l.Args) == 1 {
				if f, ok := l.Fun.(*ast.Ident); ok && f.Name == "len" {
					return rw.Render(l)
				}
			}
		}
	}
	return strconv.FormatInt(k, 10)
}

// asciiUse reports whether the value of call c (a char) is only compared (== or !=) with an
// untyped ASCII char constant, or is the tag of a switch whose cases are all such constants.
func asciiUse(c *ast.CallExpr) bool {
	asciiConst := func(e ast.Expr) bool {
		tv := rw.Info.Types[e]
		if tv.Value == nil || tv.Value.Kind() != constant.Int {
			return false
		}
		if b, ok := tv.Type.(*types.Basic); !ok || b.Info()&types.IsUntyped == 0 {
			// a typed constant would not compare with a byte; j2go writes chars untyped
			if lit, ok := rw.Unparen(e).(*ast.BasicLit); !ok || lit.Kind != token.CHAR {
				return false
			}
		}
		k, _ := constant.Int64Val(tv.Value)
		return k >= 0 && k < 0x80
	}
	switch p := rw.Up(c).(type) {
	case *ast.BinaryExpr:
		if p.Op != token.EQL && p.Op != token.NEQ {
			return false
		}
		other := p.Y
		if rw.Unparen(p.Y) == ast.Expr(c) {
			other = p.X
		}
		return asciiConst(rw.Unparen(other))
	case *ast.SwitchStmt:
		if p.Tag == nil || rw.Unparen(p.Tag) != ast.Expr(c) {
			return false
		}
		for _, st := range p.Body.List {
			for _, e := range st.(*ast.CaseClause).List {
				if !asciiConst(rw.Unparen(e)) {
					return false
				}
			}
		}
		return true
	}
	return false
}

var searchFuncs = map[string]string{"String_indexOf": "IndexByte", "String_lastIndexOf": "LastIndexByte", "String_indexOf_String": "Index", "String_lastIndexOf_String": "LastIndex"}

// search matches e = s.indexOf(p) / lastIndexOf(p) of the string variable s, with p an ASCII
// char constant or a String constant or variable, and returns the Go search (strings.IndexByte
// / LastIndexByte / Index / LastIndex), and p as an expression to match a test against. A
// match's unit index and byte index name the same position of s (the runtime finds Strings by
// their bytes too).
func search(e ast.Expr, s ast.Expr) (func() string, ast.Expr, bool) {
	name, c := rw.CallName(e)
	fn, ok := searchFuncs[name]
	if !ok || len(c.Args) != 2 || objOf(c.Args[0]) == nil || objOf(c.Args[0]) != objOf(s) {
		return nil, nil, false
	}
	if declares(curDecl, "strings") {
		return nil, nil, false
	}
	if name == "String_indexOf" || name == "String_lastIndexOf" {
		k, ok := constInt(c.Args[1])
		if !ok || k < 0 || k >= 0x80 {
			return nil, nil, false
		}
		lit := &ast.BasicLit{Kind: token.STRING, Value: strconv.Quote(string(rune(k)))}
		rw.Info.Types[lit] = types.TypeAndValue{Type: types.Typ[types.UntypedString], Value: constant.MakeString(string(rune(k)))}
		return func() string {
			return "strings." + fn + "(" + rw.Render(c.Args[0]) + ", " + strconv.QuoteRune(rune(k)) + ")"
		}, lit, true
	}
	if !patternOK(c.Args[1]) {
		return nil, nil, false
	}
	return func() string {
		return "strings." + fn + "(" + rw.Render(c.Args[0]) + ", " + rw.Render(c.Args[1]) + ")"
	}, c.Args[1], true
}

func isCharSearch(e ast.Expr) bool {
	name, _ := rw.CallName(e)
	return name == "String_indexOf" || name == "String_lastIndexOf"
}

var (
	curFile     *ast.File
	curDecl     *ast.FuncDecl
	needStrings = map[*ast.File]bool{}
)

// declares reports whether fd declares an identifier named name (which would hide a package).
func declares(fd *ast.FuncDecl, name string) bool {
	found := false
	ast.Inspect(fd, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name && rw.Info.Defs[id] != nil {
			found = true
		}
		return !found
	})
	return found
}

// addImports adds import "strings" to the files whose rewrites call it.
func addImports() {
	for f := range needStrings {
		have := false
		for _, im := range f.Imports {
			if im.Path.Value == `"strings"` {
				have = true
			}
		}
		if have {
			continue
		}
		var gd *ast.GenDecl
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
				gd = g
				break
			}
		}
		if gd == nil {
			name := f.Name
			rw.Rewrite[name] = func() string { return name.Name + "\n\nimport \"strings\"" }
			continue
		}
		g := gd
		rw.Rewrite[g] = func() string {
			specs := []string{`"strings"`}
			for _, s := range g.Specs {
				specs = append(specs, rw.Text(s))
			}
			sort.Strings(specs)
			return "import (\n" + strings.Join(specs, "\n") + "\n)"
		}
	}
}
