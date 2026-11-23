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
				// for I := int32(0); I < String_length(S); I++
				init, ok := loop.Init.(*ast.AssignStmt)
				if !ok || init.Tok != token.DEFINE || len(init.Lhs) != 1 || len(init.Rhs) != 1 {
					return true
				}
				iId, ok := init.Lhs[0].(*ast.Ident)
				if !ok || rw.Text(init.Rhs[0]) != "int32(0)" {
					return true
				}
				iObj := rw.Info.Defs[iId]
				cond, ok := rw.Unparen(loop.Cond).(*ast.BinaryExpr)
				if !ok || cond.Op != token.LSS || identObj(cond.X) != iObj {
					return true
				}
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
				// The calls String_charAt(S, I) of the body, and the other uses of I.
				var reads []*ast.CallExpr
				otherUses := 0
				ast.Inspect(loop.Body, func(y ast.Node) bool {
					if c, ok := y.(*ast.CallExpr); ok {
						if a := charAt(c); a != nil && identObj(a[0]) == sObj && identObj(a[1]) == iObj {
							reads = append(reads, c)
							return false
						}
					}
					if id, ok := y.(*ast.Ident); ok && rw.Info.Uses[id] == iObj {
						otherUses++
					}
					return true
				})
				if len(reads) == 0 {
					left["a body without charAt of the index"]++
					return true
				}
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
				idx := iId.Name
				if otherUses == 0 {
					idx = "_"
				}
				s, body := lc.Args[0], loop.Body
				// The header from `for` to the body: a label before it stays where it is.
				rw.Rewrite[loop] = func() string {
					return "for " + idx + ", " + unit + " := range stringUnits_(" + rw.Render(s) + ") " + dropRx.ReplaceAllString(rw.Render(body), "")
				}
				loops++
				calls += len(reads)
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
	fmt.Printf("strunits: %d loops over a string's units (%d charAt), %d charAt of an ASCII constant\n", loops, calls, consts)
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
