// Command slices is post-flip step 1: lists that never leave their owner become Go slices.
//
// A struct field or local variable declared ListOf[T] or ArrayListOf[T] is rewritten to []T when
// every assignment to it is a new empty ArrayList (or nil) and every other use is one of
//
//	x.add(e) as a statement, x.get(i), x.set(i, e) as a statement, x.size(), x.isEmpty(),
//	x.clear() as a statement, or the source of a for-each loop (it := foreach_(any(x))).
//
// Elements are stored unboxed. Where the surrounding code boxes or unboxes an element with the
// rw.Conversion that belongs to T, the rw.Conversion is dropped; anywhere else the canonical rw.Conversion
// for T is written out, so the rw.Rewrite does not depend on what the surroundings are.
//
// usage: slices [-n] <dir of package closure>     (-n: report only)
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"

	"postflip/rw"
)

type cand struct {
	obj      *types.Var
	typeExpr ast.Expr // the ListOf[T] expression in the declaration
	elemExpr ast.Expr
	elem     types.Type
	reason   string // why not eligible; "" if eligible
	edits    []rw.Edit
	isLocal  bool
}

func (c *cand) unboxed(cv rw.Conv, a ast.Expr) string { return rw.Unboxed(cv, c.elem, a) }

func (c *cand) read(cv rw.Conv, n ast.Expr, code func() string) {
	c.edits = append(c.edits, rw.Read(cv, n, code))
}

func isNewList(e ast.Expr) (capacity ast.Expr, ok bool) {
	e = rw.Unparen(e)
	if id, isId := e.(*ast.Ident); isId && id.Name == "nil" {
		return nil, true
	}
	name, call := rw.CallName(e)
	if name == "icast_List" && len(call.Args) == 1 {
		if c2, isCall := rw.Unparen(call.Args[0]).(*ast.CallExpr); isCall {
			if s, isSel := c2.Fun.(*ast.SelectorExpr); isSel && s.Sel.Name == "self" && len(c2.Args) == 0 {
				name, call = rw.CallName(s.X)
			}
		}
	}
	switch name {
	case "new_ArrayList_0":
		return nil, len(call.Args) == 0
	case "new_ArrayList_int":
		return call.Args[0], len(call.Args) == 1
	}
	return nil, false
}

func (c *cand) newCode(capacity ast.Expr) string {
	if capacity == nil {
		return "[]" + rw.Text(c.elemExpr) + "{}"
	}
	return "make([]" + rw.Text(c.elemExpr) + ", 0, " + rw.Render(capacity) + ")"
}

func (c *cand) fail(n ast.Node, why string) {
	if c.reason == "" {
		c.reason = why
	}
}

// use classifies one occurrence v of the variable (an identifier, or the selector naming the field).
func (c *cand) use(cv rw.Conv, v ast.Expr, uses map[types.Object][]*ast.Ident) {
	V := func() string { return rw.Render(v) }
	switch p := rw.Up(v).(type) {
	case *ast.AssignStmt:
		if len(p.Lhs) == 1 && rw.Unparen(p.Lhs[0]) == v && p.Tok == token.ASSIGN {
			capacity, ok := isNewList(p.Rhs[0])
			if !ok {
				c.fail(p, "assigned something other than a new ArrayList")
				return
			}
			if id, isId := rw.Unparen(p.Rhs[0]).(*ast.Ident); isId && id.Name == "nil" {
				return
			}
			rhs := p.Rhs[0]
			c.edits = append(c.edits, rw.Edit{rhs, func() string { return c.newCode(capacity) }})
			return
		}
		c.fail(p, "used as a value in an assignment")
	case *ast.SelectorExpr:
		call, ok := rw.Up(p).(*ast.CallExpr)
		if !ok || rw.Unparen(call.Fun) != p || rw.Unparen(p.X) != v {
			c.fail(p, "field or method value taken")
			return
		}
		if !rw.Pure(v) {
			c.fail(p, "receiver expression has a call in it")
			return
		}
		_, isStmt := rw.Up(call).(*ast.ExprStmt)
		switch m := p.Sel.Name; {
		case m == "get" && len(call.Args) == 1:
			c.read(cv, call, func() string { return V() + "[" + rw.Render(call.Args[0]) + "]" })
		case m == "size" && len(call.Args) == 0:
			c.edits = append(c.edits, rw.Edit{call, func() string { return "int32(len(" + V() + "))" }})
		case m == "isEmpty" && len(call.Args) == 0:
			c.edits = append(c.edits, rw.Edit{call, func() string { return "(len(" + V() + ") == 0)" }})
		case m == "add" && len(call.Args) == 1 && isStmt:
			c.edits = append(c.edits, rw.Edit{call, func() string { return V() + " = append(" + V() + ", " + c.unboxed(cv, call.Args[0]) + ")" }})
		case m == "set" && len(call.Args) == 2 && isStmt:
			c.edits = append(c.edits, rw.Edit{call, func() string { return V() + "[" + rw.Render(call.Args[0]) + "] = " + c.unboxed(cv, call.Args[1]) }})
		case m == "clear" && len(call.Args) == 0 && isStmt:
			c.edits = append(c.edits, rw.Edit{call, func() string { return "clear(" + V() + ")\n" + V() + " = " + V() + "[:0]" }})
		default:
			what := "method " + m
			if (m == "add" || m == "set" || m == "clear") && !isStmt {
				what += " used as a value"
			}
			c.fail(p, what)
		}
	case *ast.CallExpr:
		// it := foreach_(any(v))
		if name, _ := rw.CallName(p); name == "any" {
			if fe, ok := rw.Up(p).(*ast.CallExpr); ok {
				if n2, _ := rw.CallName(fe); n2 == "foreach_" {
					if as, ok := rw.Up(fe).(*ast.AssignStmt); ok && as.Tok == token.DEFINE && len(as.Lhs) == 1 && rw.Pure(v) {
						it := rw.Info.Defs[as.Lhs[0].(*ast.Ident)]
						for _, u := range uses[it] {
							s, ok := rw.Up(u).(*ast.SelectorExpr)
							if !ok || (s.Sel.Name != "v" && s.Sel.Name != "next") {
								c.fail(p, "for-each iterator used for more than next and v")
								return
							}
							if s.Sel.Name == "v" {
								s := s
								c.read(cv, s, func() string { return rw.Text(s) })
							}
						}
						c.edits = append(c.edits, rw.Edit{fe, func() string { return "eachOf_(&" + V() + ")" }})
						return
					}
				}
			}
			c.fail(p, "passed on as a List (rw.Converted to any)")
			return
		}
		name, _ := rw.CallName(p)
		if name == "" {
			if s, ok := p.Fun.(*ast.SelectorExpr); ok {
				name = s.Sel.Name
			}
		}
		c.fail(p, "argument of a call")
		_ = name
	case *ast.ReturnStmt:
		c.fail(p, "returned")
	case *ast.BinaryExpr:
		c.fail(p, "compared")
	case *ast.ValueSpec:
		c.fail(p, "assigned to another variable")
	case *ast.KeyValueExpr:
		c.fail(p, "set in a composite literal")
	default:
		c.fail(p, fmt.Sprintf("other use (%T)", p))
	}
}

func main() {
	report := len(os.Args) > 1 && os.Args[1] == "-n"
	dir := os.Args[len(os.Args)-1]
	rw.Load(dir)
	files := rw.Files
	uses := rw.UsesOf
	// Candidates: fields and declared variables of type ListOf[T] / ArrayListOf[T].
	cands := map[types.Object]*cand{}
	var order []*cand
	listType := func(e ast.Expr) ast.Expr {
		ix, ok := e.(*ast.IndexExpr)
		if !ok {
			return nil
		}
		if id, ok := ix.X.(*ast.Ident); ok && (id.Name == "ListOf" || id.Name == "ArrayListOf") {
			return ix.Index
		}
		return nil
	}
	add := func(names []*ast.Ident, te ast.Expr, local bool) {
		el := listType(te)
		if el == nil {
			return
		}
		for _, id := range names {
			if v, ok := rw.Info.Defs[id].(*types.Var); ok {
				c := &cand{obj: v, typeExpr: te, elemExpr: el, elem: rw.Info.Types[el].Type, isLocal: local}
				cands[v] = c
				order = append(order, c)
			}
		}
	}
	for _, f := range files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.StructType:
				for _, fl := range x.Fields.List {
					add(fl.Names, fl.Type, false)
				}
			case *ast.ValueSpec:
				if x.Type != nil {
					if _, isDecl := rw.Parent[rw.Parent[x]].(*ast.DeclStmt); isDecl && len(x.Names) == 1 {
						add(x.Names, x.Type, true)
					}
				}
			}
			return true
		})
	}
	for _, c := range order {
		cv := rw.ConvFor(c.elem)
		if !cv.OK {
			c.reason = "element type " + c.elem.String() + " has no known boxing"
			continue
		}
		// The declaration.
		for id, o := range rw.Info.Defs {
			if o != c.obj {
				continue
			}
			if spec, ok := rw.Parent[id].(*ast.ValueSpec); ok && len(spec.Values) == 1 {
				capacity, ok := isNewList(spec.Values[0])
				if !ok {
					c.fail(spec, "initialized with something other than a new ArrayList")
				} else if vid, isId := rw.Unparen(spec.Values[0]).(*ast.Ident); !(isId && vid.Name == "nil") {
					val := spec.Values[0]
					c.edits = append(c.edits, rw.Edit{val, func() string { return c.newCode(capacity) }})
				}
			}
		}
		te := c.typeExpr
		c.edits = append(c.edits, rw.Edit{te, func() string { return "[]" + rw.Text(c.elemExpr) }})
		for _, id := range uses[c.obj] {
			var v ast.Expr = id
			if s, ok := rw.Parent[id].(*ast.SelectorExpr); ok && s.Sel == id {
				v = s
			}
			c.use(cv, v, uses)
		}
	}
	// A struct type declared with several names sharing one type expression: all or none.
	reasons := map[string]int{}
	n := 0
	byType := map[ast.Expr][]*cand{}
	for _, c := range order {
		byType[c.typeExpr] = append(byType[c.typeExpr], c)
	}
	for _, cs := range byType {
		bad := ""
		for _, c := range cs {
			if c.reason != "" {
				bad = c.reason
			}
		}
		for _, c := range cs {
			if bad != "" && c.reason == "" {
				c.reason = "shares its declaration with a list that stays: " + bad
			}
		}
	}
	for _, c := range order {
		if c.reason != "" {
			reasons[c.reason]++
			continue
		}
		n++
		for _, e := range c.edits {
			if _, dup := rw.Rewrite[e.Node]; dup {
				continue
			}
			rw.Rewrite[e.Node] = e.Gen
		}
		if report {
			p := rw.Fset.Position(c.obj.Pos())
			fmt.Printf("slice\t%s:%d\t%s\t[]%s\n", filepath.Base(p.Filename), p.Line, c.obj.Name(), rw.Text(c.elemExpr))
		}
	}
	fmt.Fprintf(os.Stderr, "lists declared with element types: %d; become slices: %d\n", len(order), n)
	var rs []string
	for r := range reasons {
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool { return reasons[rs[i]] > reasons[rs[j]] })
	for _, r := range rs {
		fmt.Fprintf(os.Stderr, "  stays (%d): %s\n", reasons[r], r)
	}
	if report {
		return
	}
	changed := rw.Save()
	fmt.Fprintf(os.Stderr, "files changed: %d\n", changed)
}
