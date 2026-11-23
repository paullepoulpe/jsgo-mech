// Command sbuild is a post-flip step: a local StringBuilder that is only appended to and read
// whole is a strBuilder_, a strings.Builder of the appended bytes, held in the variable.
//
// j2go translates `StringBuilder sb = new StringBuilder()` as a pointer to the runtime's
// StringBuilder, which keeps Java's UTF-16 units (bytes while everything is ASCII) and copies
// them into a new string at toString. A builder that the function only appends to and then
// reads whole (toString, length, isEmpty) needs none of that: strBuilder_ (rt_strbuf.go)
// appends the strings' bytes to a strings.Builder and returns them without a copy, and is the
// local variable itself, not an object. It builds the same string as StringBuilder for every
// input: the same text for each append_* method (the String.valueOf rules, which both take
// from the runtime), and for a string that is not valid UTF-8 (a lone surrogate, which UTF-16
// pairs with a neighbour, or a stray byte) it continues as a StringBuilder.
//
// A local variable is converted when its declaration is `var sb *StringBuilder =
// new_StringBuilder()` (or new_StringBuilder_int(n): sb.grow(n), or new_StringBuilder_String(s):
// sb.append(s)), and every other use is a call of one of the methods strBuilder_ has, whose
// result is unused, read (toString, length, isEmpty) or the receiver of another such call
// (sb.append(a).append(b)), or an assignment of a new builder (sb = new_StringBuilder(), which
// becomes sb = strBuilder_{}). Nothing else sees the builder, so no one can tell.
//
// usage: sbuild [-n] <dir of package closure>
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

// methods are the StringBuilder methods that strBuilder_ has, and whether they return the
// builder.
var methods = map[string]bool{
	"append": true, "append_char": true, "append_charArr": true, "append_int": true,
	"append_long": true, "append_boolean": true, "append_double": true, "append_Object": true,
	"append_CharSequence": true, "append_CharSequence_int_int": true,
	"toString": false, "length": false, "isEmpty": false,
}

func main() {
	rw.Load(os.Args[len(os.Args)-1])
	report := os.Args[1] == "-n"
	sbType := rw.Pkg.Scope().Lookup("StringBuilder").Type()
	ptr := types.NewPointer(sbType)
	if rw.Pkg.Scope().Lookup("strBuilder_") == nil {
		fmt.Fprintln(os.Stderr, "the runtime has no strBuilder_ (rt_strbuf.go)")
		os.Exit(1)
	}
	why := map[string]int{}
	n, sites := 0, 0
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.Position(f.Pos()).Filename), "g_") {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			var id *ast.Ident
			var val ast.Expr
			var stmt ast.Stmt
			switch s := x.(type) {
			case *ast.DeclStmt:
				gd := s.Decl.(*ast.GenDecl)
				if gd.Tok != token.VAR || len(gd.Specs) != 1 {
					return true
				}
				vs := gd.Specs[0].(*ast.ValueSpec)
				if len(vs.Names) != 1 || len(vs.Values) != 1 {
					return true
				}
				id, val, stmt = vs.Names[0], vs.Values[0], s
			case *ast.AssignStmt:
				if s.Tok != token.DEFINE || len(s.Lhs) != 1 || len(s.Rhs) != 1 {
					return true
				}
				id, _ = s.Lhs[0].(*ast.Ident)
				val, stmt = s.Rhs[0], s
			default:
				return true
			}
			if id == nil {
				return true
			}
			v, ok := rw.Info.Defs[id].(*types.Var)
			if !ok || !types.Identical(v.Type(), ptr) {
				return true
			}
			if _, inBlock := rw.Parent[stmt].(*ast.BlockStmt); !inBlock {
				if _, inCase := rw.Parent[stmt].(*ast.CaseClause); !inCase {
					why["declared outside a block"]++
					return true
				}
			}
			ctor, ctorCall := rw.CallName(val)
			if ctor != "new_StringBuilder" && ctor != "new_StringBuilder_int" && ctor != "new_StringBuilder_String" {
				why["not a new builder: "+ctor]++
				return true
			}
			var calls []*ast.CallExpr
			var resets []*ast.AssignStmt
			var strs []*ast.CallExpr
			reason := ""
			for _, u := range rw.UsesOf[v] {
				if as, ok := rw.Parent[u].(*ast.AssignStmt); ok && as.Tok == token.ASSIGN && len(as.Lhs) == 1 && as.Lhs[0] == u {
					if name, _ := rw.CallName(as.Rhs[0]); name == "new_StringBuilder" {
						resets = append(resets, as)
						continue
					}
					reason = "assigned"
					break
				}
				if c := asString(u); c != nil {
					strs = append(strs, c)
					continue
				}
				var e ast.Expr = u
				ok := false
				for {
					s, isSel := rw.Up(e).(*ast.SelectorExpr)
					if !isSel || rw.Unparen(s.X) != e {
						break
					}
					c, isCall := rw.Up(s).(*ast.CallExpr)
					ret, known := methods[s.Sel.Name]
					if !isCall || c.Fun != s || !known {
						reason = "calls " + s.Sel.Name
						break
					}
					calls = append(calls, c)
					if !ret {
						ok = true
						break
					}
					if _, isStmt := rw.Up(c).(*ast.ExprStmt); isStmt {
						ok = true
						break
					}
					e = c
				}
				if !ok {
					if reason == "" {
						reason = fmt.Sprintf("used as %T", rw.Up(e))
					}
					break
				}
			}
			if reason != "" {
				why[reason]++
				return true
			}
			n++
			sites += len(calls)
			name := id.Name
			var tail string
			switch ctor {
			case "new_StringBuilder_int":
				arg := ctorCall.Args[0]
				tail = "\n" + name + ".grow(%s)"
				rw.Rewrite[stmt] = func() string { return "var " + name + " strBuilder_" + fmt.Sprintf(tail, rw.Render(arg)) }
			case "new_StringBuilder_String":
				arg := ctorCall.Args[0]
				tail = "\n" + name + ".append(%s)"
				rw.Rewrite[stmt] = func() string { return "var " + name + " strBuilder_" + fmt.Sprintf(tail, rw.Render(arg)) }
			default:
				rw.Rewrite[stmt] = func() string { return "var " + name + " strBuilder_" }
			}
			for _, c := range strs {
				rw.Rewrite[c] = func() string { return name + ".toString()" }
			}
			sites += len(strs)
			for _, r := range resets {
				rhs := r.Rhs[0]
				rw.Rewrite[rhs] = func() string { return "strBuilder_{}" }
			}
			return true
		})
	}
	var ks []string
	left := 0
	for k, c := range why {
		ks = append(ks, fmt.Sprintf("%5d %s", c, k))
		left += c
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ks)))
	fmt.Fprintf(os.Stderr, "builders converted: %d (%d method calls); left: %d\n", n, sites, left)
	for _, k := range ks {
		fmt.Fprintln(os.Stderr, "  "+k)
	}
	if !report {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
	}
}

// asString is the call that reads builder u as a string: Object_toString(obj_(u)),
// String_valueOf_Object(obj_(u)) or objstr_(obj_(u)) (u is never null).
func asString(u *ast.Ident) *ast.CallExpr {
	var e ast.Expr = u
	if pe, ok := rw.Up(u).(ast.Expr); ok {
		if name, c := rw.CallName(pe); c != nil && len(c.Args) == 1 && rw.Unparen(c.Args[0]) == u && name == "obj_" {
			e = c
		}
	}
	p, ok := rw.Up(e).(*ast.CallExpr)
	if !ok || len(p.Args) != 1 || rw.Unparen(p.Args[0]) != e {
		return nil
	}
	switch name, _ := rw.CallName(p); name {
	case "Object_toString", "String_valueOf_Object", "objstr_", "tostrR_": // reflip16
		return p
	}
	return nil
}
