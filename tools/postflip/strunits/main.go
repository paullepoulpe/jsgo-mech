// Command strunits is a post-flip step: a loop over the characters of a string reads the
// string once, and charAt of an ASCII constant reads its byte.
//
// A Java String is UTF-16 and the runtime keeps it as UTF-8 (rt_string.go), so String_length
// counts the units of the string and String_charAt(s, i) locates unit i: for a string under
// 256 bytes both scan it from the start on every call, which makes Java's
//
//	for (int i = 0; i < s.length(); i++) { char c = s.charAt(i); ... }
//
// quadratic in the length of s. When s is a local variable, parameter or constant of type
// string that nothing assigns during the loop (the body, and any function literal of the
// enclosing function, which the body might call), and the body does not assign i, the loop
// visits the units of one unchanging string in order, which is what stringUnits_(s) yields:
//
//	for i, c := range stringUnits_(s) { ... }
//
// with each String_charAt(s, i) of the body replaced by c (by the variable c of a first
// statement `var c uint16 = String_charAt(s, i)` that the body does not assign, which the
// range clause then declares). i is `_` when nothing else uses it. break, continue (labelled
// or not), return and defer keep their meaning in a range-over-func body; a body with goto is
// left as it is.
//
// Also:
//   - the index may start at a constant k > 0, or at a local variable whose values (its
//     declaration and every assignment) are all constants >= 0: the loop starts at an
//     existing unit or does not run, which is stringUnitsFrom_(s, k);
//   - the condition may be `i < s.length() && EXTRA`: EXTRA is tested at the start of each
//     pass, `if !EXTRA { break }`, after i < s.length() as in Java (reading the unit first
//     has no effect);
//   - String_substring(s, i, i + 1) in the body is chrstr_ of the unit: the String of one
//     unit, which for half a surrogate pair is its lone surrogate's 3-byte WTF-8 sequence,
//     as String_substring cuts it.
//
// String_charAt(C, e) of a string constant C that is all ASCII is asciiCharAt_(C, e): byte e,
// with the same StringIndexOutOfBoundsException outside the string.
//
// Rerunning the step changes nothing (the rewritten loops no longer match).
//
// usage: strunits [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"postflip/rw"
)

const drop = "\x00strunits-drop\x00"

var dropRx = regexp.MustCompile(drop + `\n[\t ]*`)

// assigns reports whether n assigns (=, op=, ++, --) the variable obj or takes its address.
func assigns(n ast.Node, obj types.Object) bool {
	found := false
	is := func(e ast.Expr) bool {
		id, ok := rw.Unparen(e).(*ast.Ident)
		return ok && (rw.Info.Uses[id] == obj || rw.Info.Defs[id] == obj)
	}
	ast.Inspect(n, func(x ast.Node) bool {
		switch s := x.(type) {
		case *ast.AssignStmt:
			for _, l := range s.Lhs {
				if is(l) {
					found = true
				}
			}
		case *ast.IncDecStmt:
			if is(s.X) {
				found = true
			}
		case *ast.UnaryExpr:
			if s.Op == token.AND && is(s.X) {
				found = true
			}
		case *ast.RangeStmt:
			if (s.Key != nil && is(s.Key)) || (s.Value != nil && is(s.Value)) {
				found = true
			}
		}
		return !found
	})
	return found
}

// charAt returns the arguments of a call String_charAt(a, b), or nil.
func charAt(e ast.Expr) []ast.Expr {
	if name, c := rw.CallName(e); name == "String_charAt" && len(c.Args) == 2 {
		return c.Args
	}
	return nil
}

func identObj(e ast.Expr) types.Object {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	return rw.Info.Uses[id]
}

func isString(o types.Object) bool {
	b, ok := o.Type().(*types.Basic)
	return ok && b.Kind() == types.String
}

func asciiConst(o types.Object) bool {
	c, ok := o.(*types.Const)
	if !ok || !isString(c) || c.Val().Kind() != constant.String {
		return false
	}
	s := constant.StringVal(c.Val())
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	loops, calls, consts := 0, 0, 0
	count := map[string]int{}
	left := map[string]int{}
	inLoop := map[*ast.CallExpr]bool{}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			var lits []*ast.FuncLit
			names := map[string]bool{}
			ast.Inspect(fd, func(x ast.Node) bool {
				switch y := x.(type) {
				case *ast.FuncLit:
					lits = append(lits, y)
				case *ast.Ident:
					names[y.Name] = true
				}
				return true
			})
			ast.Inspect(fd.Body, func(x ast.Node) bool {
				loop, ok := x.(*ast.ForStmt)
				if !ok || loop.Init == nil || loop.Cond == nil || loop.Post == nil {
					return true
				}
				// for I := int32(START); I < String_length(S) [&& EXTRA]; I++
				init, ok := loop.Init.(*ast.AssignStmt)
				if !ok || init.Tok != token.DEFINE || len(init.Lhs) != 1 || len(init.Rhs) != 1 {
					return true
				}
				iId, ok := init.Lhs[0].(*ast.Ident)
				if !ok {
					return true
				}
				start, ok := startOf(init.Rhs[0], fd)
				if !ok {
					return true
				}
				iObj := rw.Info.Defs[iId]
				// The conjuncts of the condition: the length test first, then the rest.
				conj := conjuncts(loop.Cond)
				cond, ok := rw.Unparen(conj[0]).(*ast.BinaryExpr)
				if !ok || cond.Op != token.LSS || identObj(cond.X) != iObj {
					return true
				}
				extra := conj[1:]
				name, lc := rw.CallName(cond.Y)
				if name != "String_length" || len(lc.Args) != 1 {
					return true
				}
				post, ok := loop.Post.(*ast.IncDecStmt)
				if !ok || post.Tok != token.INC || identObj(post.X) != iObj {
					return true
				}
				sObj := identObj(lc.Args[0])
				if sObj == nil || !isString(sObj) {
					left["a length of something other than a string variable or constant"]++
					return true
				}
				if v, ok := sObj.(*types.Var); ok {
					if v.Parent() == rw.Pkg.Scope() {
						left["a package-level string variable"]++
						return true
					}
					if assigns(loop.Body, v) {
						left["the string is assigned in the loop"]++
						return true
					}
					for _, l := range lits {
						if assigns(l, v) {
							left["the string is assigned in a function literal"]++
							return true
						}
					}
				}
				if assigns(loop.Body, iObj) {
					left["the index is assigned in the loop"]++
					return true
				}
				hasGoto := false
				ast.Inspect(loop.Body, func(y ast.Node) bool {
					if b, ok := y.(*ast.BranchStmt); ok && b.Tok == token.GOTO {
						hasGoto = true
					}
					return true
				})
				if hasGoto {
					left["a body with goto"]++
					return true
				}
				// The calls String_charAt(S, I) and String_substring(S, I, I+1) of the body, and
				// the other uses of I (in the body and the extra conditions).
				var reads, cuts []*ast.CallExpr
				otherUses := 0
				countUses := func(y ast.Node) bool {
					if id, ok := y.(*ast.Ident); ok && rw.Info.Uses[id] == iObj {
						otherUses++
					}
					return true
				}
				ast.Inspect(loop.Body, func(y ast.Node) bool {
					if c, ok := y.(*ast.CallExpr); ok {
						if a := charAt(c); a != nil && identObj(a[0]) == sObj && identObj(a[1]) == iObj {
							reads = append(reads, c)
							return false
						}
						if unitCut(c, sObj, iObj) {
							cuts = append(cuts, c)
							return false
						}
					}
					return countUses(y)
				})
				for _, e := range extra {
					ast.Inspect(e, countUses)
				}
				if len(reads)+len(cuts) == 0 {
					left["a body without charAt of the index"]++
					return true
				}
				// A start other than a constant: a variable whose values are all non-negative
				// constants (startOf), so the loop starts at an existing unit or does not run.
				// The unit's variable: the first statement's, or a fresh name.
				unit := ""
				if len(loop.Body.List) > 0 {
					var cId *ast.Ident
					var val ast.Expr
					switch s := loop.Body.List[0].(type) {
					case *ast.DeclStmt:
						if g, ok := s.Decl.(*ast.GenDecl); ok && g.Tok == token.VAR && len(g.Specs) == 1 {
							if vs, ok := g.Specs[0].(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 1 {
								cId, val = vs.Names[0], vs.Values[0]
							}
						}
					case *ast.AssignStmt:
						if s.Tok == token.DEFINE && len(s.Lhs) == 1 && len(s.Rhs) == 1 {
							cId, _ = s.Lhs[0].(*ast.Ident)
							val = s.Rhs[0]
						}
					}
					if cId != nil {
						cObj := rw.Info.Defs[cId]
						b, ok := cObj.Type().(*types.Basic)
						if c, isCall := rw.Unparen(val).(*ast.CallExpr); isCall && len(reads) > 0 && reads[0] == c &&
							ok && b.Kind() == types.Uint16 && !assigns(&ast.BlockStmt{List: loop.Body.List[1:]}, cObj) {
							unit = cId.Name
							first := loop.Body.List[0]
							rw.Rewrite[first] = func() string { return drop }
						}
					}
				}
				if unit == "" {
					unit = "u_"
					for k := 2; names[unit]; k++ {
						unit = fmt.Sprintf("u_%d", k)
					}
					names[unit] = true
				}
				for _, c := range reads {
					inLoop[c] = true
					u := unit
					rw.Rewrite[c] = func() string { return u }
				}
				for _, c := range cuts {
					u := unit
					rw.Rewrite[c] = func() string { return "chrstr_(" + u + ")" }
				}
				idx := iId.Name
				if otherUses == 0 {
					idx = "_"
				}
				s, body := lc.Args[0], loop.Body
				// The header from `for` to the body: a label before it stays where it is.
				rw.Rewrite[loop] = func() string {
					units := "stringUnits_(" + rw.Render(s) + ")"
					if start != nil {
						units = "stringUnitsFrom_(" + rw.Render(s) + ", " + rw.Render(start) + ")"
					}
					b := dropRx.ReplaceAllString(rw.Render(body), "")
					if len(extra) > 0 {
						// the rest of the condition, tested before each pass as Java does
						var es []string
						for _, e := range extra {
							es = append(es, rw.Render(e))
						}
						test := "!(" + strings.Join(es, " && ") + ")"
						if len(es) == 1 {
							test = "!" + paren(extra[0])
						}
						b = "{\nif " + test + " {\nbreak\n}" + strings.TrimPrefix(b, "{")
					}
					return "for " + idx + ", " + unit + " := range " + units + " " + b
				}
				loops++
				calls += len(reads) + len(cuts)
				if start != nil {
					count["from a start other than 0"]++
				}
				if len(extra) > 0 {
					count["with a further condition"]++
				}
				if len(cuts) > 0 {
					count["substring(i, i + 1): chrstr_ of the unit"] += len(cuts)
				}
				return true
			})
		}
	}
	// charAt of an ASCII constant.
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			c, ok := x.(*ast.CallExpr)
			if !ok || inLoop[c] {
				return true
			}
			a := charAt(c)
			if a == nil {
				return true
			}
			if o := identObj(a[0]); o != nil && asciiConst(o) {
				fn := c.Fun
				rw.Rewrite[fn] = func() string { return "asciiCharAt_" }
				consts++
			}
			return true
		})
	}
	fmt.Printf("strunits: %d loops over a string's units (%d charAt / substring), %d charAt of an ASCII constant\n", loops, calls, consts)
	var ks []string
	for k := range count {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Printf("  of which %5d %s\n", count[k], k)
	}
	var reasons []string
	for k := range left {
		reasons = append(reasons, k)
	}
	sort.Strings(reasons)
	for _, k := range reasons {
		fmt.Printf("  left: %5d %s\n", left[k], k)
	}
	if report {
		return
	}
	fmt.Printf("files changed: %d\n", rw.Save())
}

// conjuncts returns the operands of a chain of && (one element for any other expression).
func conjuncts(e ast.Expr) []ast.Expr {
	if b, ok := rw.Unparen(e).(*ast.BinaryExpr); ok && b.Op == token.LAND {
		return append(conjuncts(b.X), conjuncts(b.Y)...)
	}
	return []ast.Expr{e}
}

func paren(e ast.Expr) string {
	switch rw.Unparen(e).(type) {
	case *ast.Ident, *ast.CallExpr, *ast.SelectorExpr, *ast.ParenExpr:
		return rw.Render(e)
	}
	if _, ok := e.(*ast.ParenExpr); ok {
		return rw.Render(e)
	}
	return "(" + rw.Render(e) + ")"
}

// startOf returns the start of a loop index initialized with e: nil for the constant 0; e
// itself for another constant >= 0 or int32(v) / v of a local variable v of the function whose
// values (its declaration and every assignment, none in a function literal, address never
// taken) are all constants >= 0. Then the loop starts at a unit of the string, or does not run.
func startOf(e ast.Expr, fd *ast.FuncDecl) (ast.Expr, bool) {
	if k, ok := constInt(e); ok {
		if k == 0 {
			return nil, true
		}
		return e, k > 0 && k < 1<<31
	}
	in := rw.Unparen(e)
	if c, ok := in.(*ast.CallExpr); ok && len(c.Args) == 1 {
		if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "int32" {
			in = rw.Unparen(c.Args[0])
		}
	}
	v, ok := identObj(in).(*types.Var)
	if !ok || v.IsField() || v.Parent() == rw.Pkg.Scope() || !contains(fd, v.Pos()) {
		return nil, false
	}
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			if rw.Info.Defs[n] == v {
				return nil, false
			}
		}
	}
	nonNeg := func(x ast.Expr) bool {
		k, ok := constInt(x)
		if ok {
			return k >= 0 && k < 1<<31
		}
		// an immediately called function literal whose returns are all such constants
		c, ok := rw.Unparen(x).(*ast.CallExpr)
		if !ok || len(c.Args) != 0 {
			return false
		}
		lit, ok := c.Fun.(*ast.FuncLit)
		if !ok {
			return false
		}
		all := true
		ast.Inspect(lit.Body, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			if r, ok := n.(*ast.ReturnStmt); ok {
				if len(r.Results) != 1 {
					all = false
				} else if k, ok := constInt(r.Results[0]); !ok || k < 0 || k >= 1<<31 {
					all = false
				}
			}
			return true
		})
		return all
	}
	ok = true
	seen := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.ValueSpec:
			for i, id := range x.Names {
				if rw.Info.Defs[id] == v {
					seen = true
					if len(x.Values) != len(x.Names) || !nonNeg(x.Values[i]) {
						ok = false
					}
				}
			}
		case *ast.AssignStmt:
			for i, l := range x.Lhs {
				if id, isId := rw.Unparen(l).(*ast.Ident); isId && (rw.Info.Defs[id] == v || rw.Info.Uses[id] == v) {
					seen = seen || x.Tok == token.DEFINE
					if len(x.Lhs) != len(x.Rhs) || (x.Tok != token.ASSIGN && x.Tok != token.DEFINE) || !nonNeg(x.Rhs[i]) {
						ok = false
					}
				}
			}
		case *ast.IncDecStmt:
			if identObj(x.X) == v {
				ok = false
			}
		case *ast.UnaryExpr:
			if x.Op == token.AND && identObj(x.X) == v {
				ok = false
			}
		case *ast.RangeStmt:
			if (x.Key != nil && identObj(x.Key) == v) || (x.Value != nil && identObj(x.Value) == v) {
				ok = false
			}
		}
		return true
	})
	return e, ok && seen
}

func contains(n ast.Node, p token.Pos) bool { return n.Pos() <= p && p < n.End() }

func constInt(e ast.Expr) (int64, bool) {
	tv, ok := rw.Info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(tv.Value)
}

// unitCut reports whether c is String_substring(S, I, I + 1): the unit at I as a String, which
// is chrstr_ of the unit (a lone surrogate half is its 3-byte WTF-8 sequence either way).
func unitCut(c *ast.CallExpr, sObj, iObj types.Object) bool {
	if name, _ := rw.CallName(c); name != "String_substring" || len(c.Args) != 3 {
		return false
	}
	if identObj(c.Args[0]) != sObj || identObj(c.Args[1]) != iObj {
		return false
	}
	b, ok := rw.Unparen(c.Args[2]).(*ast.BinaryExpr)
	if !ok || b.Op != token.ADD || identObj(b.X) != iObj {
		return false
	}
	k, ok := constInt(b.Y)
	return ok && k == 1
}
