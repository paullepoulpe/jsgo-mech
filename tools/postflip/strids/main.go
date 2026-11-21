// Command strids is a post-flip step for the node store: where only the identity of a node's
// string matters, the code compares or looks up the string's id instead of its bytes.
//
//	n.getString() == "prototype"       ->  n.getStringIs_(sid_prototype)
//	n.getString() != SOME_CONSTANT     ->  !n.getStringIs_(sid_...)
//	a.getString() == b.getString()     ->  a.getStringId_() == b.getStringId_()
//	scope.getVar(n.getString())        ->  scope.getVarById_(n.getStringId_())
//
// A Node record holds its string as an id into the process-wide table of interned strings
// (rt_ids.go): getString() reads the table (a load that misses the cache on a large program)
// and the caller then compares the bytes, or hashes them for a scope's table at every lookup.
// Java's String.equals on interned literals is a pointer comparison and its hashCode is
// cached; the id is the same thing here. Equal strings have one id (the table is keyed by the
// string's contents, the null string has its own id), so:
//   - a comparison of getString() with a constant string is the comparison of the node's id
//     with the constant's id, which a package variable `sid_<name> = strid_("...")` holds
//     (g_0strids.go, written by this tool, one variable per distinct constant);
//   - a comparison of the strings of two nodes is the comparison of their ids;
//   - AbstractScope.getVar of a node's string is getVarById_ (g_AbstractScope.go), the same
//     walk of the scope chain with the tables read by id (rt_scopevars.go); a TypedScope,
//     which overrides getVar, gets the string from the id and does what it did.
//
// Java's behaviour is kept: getStringId_ and getStringIs_ (rt_strid.go) throw what
// getString() throws for a node that holds no string, at the same point of the evaluation
// (operands and the receiver of getVar are evaluated in the same order as before; an id or a
// constant has no effect), and a null scope fails at the call as before.
//
// Left, counted: getString() compared with a string that is neither a constant nor another
// node's getString(). A rerun changes nothing.
//
// usage: strids [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"postflip/rw"
)

const idsFile = "g_0strids.go"

// sidName is the variable holding the id of constant string s.
func sidName(s string) string {
	ident := s != ""
	for _, c := range s {
		if !(c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z') {
			ident = false
		}
	}
	if ident {
		return "sid_" + s
	}
	return fmt.Sprintf("sidx_%x", s)
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
	}
	dir := os.Args[len(os.Args)-1]
	rw.Load(dir)
	done := map[string]int{}
	left := map[string]int{}
	isNode := func(e ast.Expr) bool {
		nt, ok := types.Unalias(rw.Info.Types[e].Type).(*types.Named)
		return ok && nt.Obj().Name() == "Node" && nt.Obj().Pkg() == rw.Pkg
	}
	// strOf is the selector of e if e is n.getString() on a Node.
	strOf := func(e ast.Expr) *ast.SelectorExpr {
		call, ok := rw.Unparen(e).(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return nil
		}
		sel, ok := call.Fun.(*ast.SelectorExpr)
		if !ok || sel.Sel.Name != "getString" || !isNode(sel.X) {
			return nil
		}
		return sel
	}
	constOf := func(e ast.Expr) (string, bool) {
		tv := rw.Info.Types[e]
		if tv.Value == nil || tv.Value.Kind() != constant.String {
			return "", false
		}
		return constant.StringVal(tv.Value), true
	}
	need := map[string]string{} // variable -> constant
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.BinaryExpr:
				if x.Op != token.EQL && x.Op != token.NEQ {
					return true
				}
				l, r := strOf(x.X), strOf(x.Y)
				if l == nil && r == nil {
					return true
				}
				not := ""
				if x.Op == token.NEQ {
					not = "!"
				}
				if l != nil && r != nil {
					done["getString() of two nodes compared -> getStringId_"]++
					rw.Rewrite[l.Sel] = func() string { return "getStringId_" }
					rw.Rewrite[r.Sel] = func() string { return "getStringId_" }
					return true
				}
				sel, other := l, x.Y
				if l == nil {
					sel, other = r, x.X
				}
				s, ok := constOf(other)
				if !ok {
					left["getString() compared with a string that is neither a constant nor a node's string"]++
					return true
				}
				name := sidName(s)
				need[name] = s
				done["getString() compared with a constant -> getStringIs_"]++
				rw.Rewrite[x] = func() string { return not + rw.Render(sel.X) + ".getStringIs_(" + name + ")" }
			case *ast.CallExpr:
				sel, ok := x.Fun.(*ast.SelectorExpr)
				if !ok || (sel.Sel.Name != "getVar" && !strings.HasPrefix(sel.Sel.Name, "getVar_String_r")) || len(x.Args) != 1 { // reflip12
					return true
				}
				fn, ok := rw.Info.Uses[sel.Sel].(*types.Func)
				if !ok {
					return true
				}
				recv := fn.Type().(*types.Signature).Recv().Type()
				if p, ok := recv.(*types.Pointer); ok {
					recv = p.Elem()
				}
				nt, ok := recv.(*types.Named)
				if !ok || (nt.Obj().Name() != "AbstractScope" && nt.Obj().Name() != "TypedScope" && nt.Obj().Name() != "Scope") || !rw.HasMethod(types.NewPointer(nt), "getVarById_") { // reflip12
					return true
				}
				arg := strOf(x.Args[0])
				if arg == nil {
					return true
				}
				done["getVar(n.getString()) -> getVarById_(n.getStringId_())"]++
				rw.Rewrite[sel.Sel] = func() string { return "getVarById_" }
				rw.Rewrite[arg.Sel] = func() string { return "getStringId_" }
			}
			return true
		})
	}
	// The id variables: those the package has and those the rewrites need, sorted, in one
	// declaration of g_0strids.go.
	var idsDecl *ast.GenDecl
	for _, f := range rw.Files {
		if filepath.Base(rw.Fset.File(f.Pos()).Name()) != idsFile {
			continue
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			idsDecl = gd
			for _, sp := range gd.Specs {
				vs := sp.(*ast.ValueSpec)
				if call, ok := vs.Values[0].(*ast.CallExpr); ok && len(call.Args) == 1 {
					if s, ok := constOf(call.Args[0]); ok {
						if _, has := need[vs.Names[0].Name]; !has {
							need[vs.Names[0].Name] = s
						}
					}
				}
			}
		}
	}
	names := make([]string, 0, len(need))
	for n := range need {
		names = append(names, n)
	}
	sort.Strings(names)
	var b strings.Builder
	b.WriteString("var (\n")
	width := 0
	for _, n := range names {
		width = max(width, len(n))
	}
	for _, n := range names {
		fmt.Fprintf(&b, "\t%-*s = strid_(%s)\n", width, n, strconv.Quote(need[n]))
	}
	b.WriteString(")")
	decl := b.String()

	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
	}
	total := 0
	keys = keys[:0]
	for k, n := range done {
		keys = append(keys, k)
		total += n
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", done[k], k)
	}
	if report || total == 0 {
		fmt.Fprintf(os.Stderr, "strids: %d sites, %d id variables\n", total, len(names))
		return
	}
	if idsDecl != nil {
		rw.Rewrite[idsDecl] = func() string { return decl }
	} else {
		// A new file: written here, and given to the engine so that the rewritten package is
		// checked with it.
		path := filepath.Join(dir, idsFile)
		src := []byte("// Code generated by tools/postflip/strids. DO NOT EDIT.\n\npackage closure\n\n" +
			"// The ids of the constant strings that the strings of nodes are compared with\n" +
			"// (rt_strid.go: Node.getStringIs_).\n" + decl + "\n")
		if err := os.WriteFile(path, src, 0o644); err != nil {
			panic(err)
		}
		f, err := parser.ParseFile(rw.Fset, path, src, parser.SkipObjectResolution)
		if err != nil {
			panic(err)
		}
		rw.Src[rw.Fset.File(f.Pos())] = src
		rw.Files = append(rw.Files, f)
	}
	fmt.Fprintf(os.Stderr, "strids: %d sites, %d id variables; files changed: %d\n", total, len(names), rw.SaveChecked())
}
