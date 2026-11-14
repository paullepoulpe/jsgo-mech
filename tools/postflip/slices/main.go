// Command slices is post-flip step 1: lists that never leave their owner become Go slices.
//
// A struct field or local variable declared ListOf[T] or ArrayListOf[T] is rewritten to []T when
// every assignment to it is a new empty ArrayList (or nil) and every other use is one of
//
//	x.add(e) as a statement, x.get(i), x.set(i, e) as a statement, x.size(), x.isEmpty(),
//	x.clear() as a statement, or the source of a for-each loop (it := foreach_(any(x))).
//
// Elements are stored unboxed. Where the surrounding code boxes or unboxes an element with the
// conversion that belongs to T, the conversion is dropped; anywhere else the canonical conversion
// for T is written out, so the rewrite does not depend on what the surroundings are.
//
// usage: slices [-n] <dir of package closure>     (-n: report only)
package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

type cand struct {
	obj      *types.Var
	typeExpr ast.Expr // the ListOf[T] expression in the declaration
	elemExpr ast.Expr
	elem     types.Type
	reason   string // why not eligible; "" if eligible
	edits    []edit
	isLocal  bool
}

type edit struct {
	node ast.Node
	gen  func() string
}

var (
	fset    = token.NewFileSet()
	info    = &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	parent  = map[ast.Node]ast.Node{}
	src     = map[*token.File][]byte{}
	rewrite = map[ast.Node]func() string{}
	pkg     *types.Package
)

func text(n ast.Node) string {
	f := fset.File(n.Pos())
	return string(src[f][f.Offset(n.Pos()):f.Offset(n.End())])
}

// render is the source of n with the rewrites of its descendants applied.
func render(n ast.Node) string {
	if g, ok := rewrite[n]; ok {
		return g()
	}
	return renderChildren(n)
}

func renderChildren(n ast.Node) string {
	f := fset.File(n.Pos())
	base := f.Offset(n.Pos())
	s := src[f][base:f.Offset(n.End())]
	var out strings.Builder
	last := 0
	ast.Inspect(n, func(c ast.Node) bool {
		if c == nil || c == n {
			return true
		}
		if g, ok := rewrite[c]; ok {
			a, b := f.Offset(c.Pos())-base, f.Offset(c.End())-base
			out.Write(s[last:a])
			out.WriteString(g())
			last = b
			return false
		}
		return true
	})
	out.Write(s[last:])
	return out.String()
}

func up(n ast.Node) ast.Node {
	p := parent[n]
	for {
		if pe, ok := p.(*ast.ParenExpr); ok {
			p = parent[pe]
			continue
		}
		return p
	}
}

func unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// pure reports whether evaluating e twice is the same as evaluating it once.
func pure(e ast.Expr) bool {
	switch x := unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return pure(x.X)
	case *ast.CallExpr:
		if s, ok := x.Fun.(*ast.SelectorExpr); ok && len(x.Args) == 0 && (s.Sel.Name == "ps_" || s.Sel.Name == "ns_") {
			return pure(s.X)
		}
	}
	return false
}

func callName(e ast.Expr) (string, *ast.CallExpr) {
	c, ok := unparen(e).(*ast.CallExpr)
	if !ok {
		return "", nil
	}
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return f.Name, c
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			return id.Name + "[" + text(f.Index) + "]", c
		}
	}
	return "", c
}

func hasFunc(name string) bool {
	_, ok := pkg.Scope().Lookup(name).(*types.Func)
	return ok
}

func hasMethod(t types.Type, name string) bool {
	o, _, _ := types.LookupFieldOrMethod(t, true, pkg, name)
	_, ok := o.(*types.Func)
	return ok
}

// conv describes how a value of the element type is boxed into `any` and taken out again.
type conv struct {
	box   func(e string) string
	unbox func(e string) string
	// isUnbox reports whether call c (with one argument) is an unboxing to exactly the element type.
	isUnbox func(name string) bool
	ok      bool
}

func convFor(t types.Type) conv {
	id := func(e string) string { return e }
	switch u := types.Unalias(t).(type) {
	case *types.Basic:
		if u.Kind() == types.String {
			return conv{func(e string) string { return "strobj_(" + e + ")" }, func(e string) string { return "str_(" + e + ")" }, func(n string) bool { return n == "str_" }, true}
		}
	case *types.Interface:
		if u.Empty() {
			return conv{id, id, func(string) bool { return false }, true}
		}
	case *types.Named:
		name := u.Obj().Name()
		if _, isIface := u.Underlying().(*types.Interface); isIface {
			if hasFunc("icast_" + name) {
				return conv{func(e string) string { return "any(" + e + ")" }, func(e string) string { return "icast_" + name + "(" + e + ")" }, func(n string) bool { return n == "icast_"+name }, true}
			}
		} else if hasMethod(u, "box_") && hasFunc(name+"_of_") {
			return conv{func(e string) string { return e + ".box_()" }, func(e string) string { return name + "_of_(" + e + ")" }, func(n string) bool { return n == name+"_of_" }, true}
		}
	case *types.Pointer:
		if n, ok := u.Elem().(*types.Named); ok {
			name := n.Obj().Name()
			box := func(e string) string { return "obj_(" + e + ")" }
			if hasMethod(u, "self") {
				box = func(e string) string { return e + ".self()" }
			}
			un := "cast_[*" + name + "]"
			if hasFunc("to_" + name) {
				un = "to_" + name
			}
			return conv{box, func(e string) string { return un + "(" + e + ")" }, func(n string) bool { return n == "cast_[*"+name+"]" || n == "to_"+name }, true}
		}
	}
	return conv{}
}

// unboxed returns the code storing boxed expression a (of type any) as an element.
func (c *cand) unboxed(cv conv, a ast.Expr) string {
	a = unparen(a)
	// obj_(x), strobj_(x), any(x), x.self(), x.box_() of an x whose type is the element type: x itself.
	if call, ok := a.(*ast.CallExpr); ok {
		var inner ast.Expr
		if id, ok := call.Fun.(*ast.Ident); ok && len(call.Args) == 1 && (id.Name == "obj_" || id.Name == "strobj_" || id.Name == "any") {
			inner = call.Args[0]
		} else if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
			inner = s.X
		}
		if inner != nil {
			if tv, ok := info.Types[inner]; ok && types.Identical(tv.Type, c.elem) {
				return render(inner)
			}
		}
	}
	return cv.unbox(render(a))
}

// read registers the rewrite of a read `n` (of type any) whose unboxed value is code().
func (c *cand) read(cv conv, n ast.Expr, code func() string) {
	if p, ok := up(n).(*ast.CallExpr); ok && len(p.Args) == 1 && unparen(p.Args[0]) == n {
		if name, _ := callName(p); name != "" && cv.isUnbox(name) {
			c.edits = append(c.edits, edit{p, code})
			return
		}
	}
	c.edits = append(c.edits, edit{n, func() string { return cv.box(code()) }})
}

func isNewList(e ast.Expr) (capacity ast.Expr, ok bool) {
	e = unparen(e)
	if id, isId := e.(*ast.Ident); isId && id.Name == "nil" {
		return nil, true
	}
	name, call := callName(e)
	if name == "icast_List" && len(call.Args) == 1 {
		if c2, isCall := unparen(call.Args[0]).(*ast.CallExpr); isCall {
			if s, isSel := c2.Fun.(*ast.SelectorExpr); isSel && s.Sel.Name == "self" && len(c2.Args) == 0 {
				name, call = callName(s.X)
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
		return "[]" + text(c.elemExpr) + "{}"
	}
	return "make([]" + text(c.elemExpr) + ", 0, " + render(capacity) + ")"
}

func (c *cand) fail(n ast.Node, why string) {
	if c.reason == "" {
		c.reason = why
	}
}

// use classifies one occurrence v of the variable (an identifier, or the selector naming the field).
func (c *cand) use(cv conv, v ast.Expr, uses map[types.Object][]*ast.Ident) {
	V := func() string { return render(v) }
	switch p := up(v).(type) {
	case *ast.AssignStmt:
		if len(p.Lhs) == 1 && unparen(p.Lhs[0]) == v && p.Tok == token.ASSIGN {
			capacity, ok := isNewList(p.Rhs[0])
			if !ok {
				c.fail(p, "assigned something other than a new ArrayList")
				return
			}
			if id, isId := unparen(p.Rhs[0]).(*ast.Ident); isId && id.Name == "nil" {
				return
			}
			rhs := p.Rhs[0]
			c.edits = append(c.edits, edit{rhs, func() string { return c.newCode(capacity) }})
			return
		}
		c.fail(p, "used as a value in an assignment")
	case *ast.SelectorExpr:
		call, ok := up(p).(*ast.CallExpr)
		if !ok || unparen(call.Fun) != p || unparen(p.X) != v {
			c.fail(p, "field or method value taken")
			return
		}
		if !pure(v) {
			c.fail(p, "receiver expression has a call in it")
			return
		}
		_, isStmt := up(call).(*ast.ExprStmt)
		switch m := p.Sel.Name; {
		case m == "get" && len(call.Args) == 1:
			c.read(cv, call, func() string { return V() + "[" + render(call.Args[0]) + "]" })
		case m == "size" && len(call.Args) == 0:
			c.edits = append(c.edits, edit{call, func() string { return "int32(len(" + V() + "))" }})
		case m == "isEmpty" && len(call.Args) == 0:
			c.edits = append(c.edits, edit{call, func() string { return "(len(" + V() + ") == 0)" }})
		case m == "add" && len(call.Args) == 1 && isStmt:
			c.edits = append(c.edits, edit{call, func() string { return V() + " = append(" + V() + ", " + c.unboxed(cv, call.Args[0]) + ")" }})
		case m == "set" && len(call.Args) == 2 && isStmt:
			c.edits = append(c.edits, edit{call, func() string { return V() + "[" + render(call.Args[0]) + "] = " + c.unboxed(cv, call.Args[1]) }})
		case m == "clear" && len(call.Args) == 0 && isStmt:
			c.edits = append(c.edits, edit{call, func() string { return "clear(" + V() + ")\n" + V() + " = " + V() + "[:0]" }})
		default:
			what := "method " + m
			if (m == "add" || m == "set" || m == "clear") && !isStmt {
				what += " used as a value"
			}
			c.fail(p, what)
		}
	case *ast.CallExpr:
		// it := foreach_(any(v))
		if name, _ := callName(p); name == "any" {
			if fe, ok := up(p).(*ast.CallExpr); ok {
				if n2, _ := callName(fe); n2 == "foreach_" {
					if as, ok := up(fe).(*ast.AssignStmt); ok && as.Tok == token.DEFINE && len(as.Lhs) == 1 && pure(v) {
						it := info.Defs[as.Lhs[0].(*ast.Ident)]
						for _, u := range uses[it] {
							s, ok := up(u).(*ast.SelectorExpr)
							if !ok || (s.Sel.Name != "v" && s.Sel.Name != "next") {
								c.fail(p, "for-each iterator used for more than next and v")
								return
							}
							if s.Sel.Name == "v" {
								s := s
								c.read(cv, s, func() string { return text(s) })
							}
						}
						c.edits = append(c.edits, edit{fe, func() string { return "eachOf_(&" + V() + ")" }})
						return
					}
				}
			}
			c.fail(p, "passed on as a List (converted to any)")
			return
		}
		name, _ := callName(p)
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
	names, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	var files []*ast.File
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			panic(err)
		}
		f, err := parser.ParseFile(fset, n, b, parser.SkipObjectResolution)
		if err != nil {
			panic(err)
		}
		src[fset.File(f.Pos())] = b
		files = append(files, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(fset, "source", nil), Error: func(err error) { fmt.Fprintln(os.Stderr, err) }}
	var err error
	pkg, err = conf.Check("closure", fset, files, info)
	if err != nil {
		os.Exit(1)
	}
	for _, f := range files {
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if len(stack) > 0 {
				parent[n] = stack[len(stack)-1]
			}
			stack = append(stack, n)
			return true
		})
	}
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
			if v, ok := info.Defs[id].(*types.Var); ok {
				c := &cand{obj: v, typeExpr: te, elemExpr: el, elem: info.Types[el].Type, isLocal: local}
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
					if _, isDecl := parent[parent[x]].(*ast.DeclStmt); isDecl && len(x.Names) == 1 {
						add(x.Names, x.Type, true)
					}
				}
			}
			return true
		})
	}
	uses := map[types.Object][]*ast.Ident{}
	for id, o := range info.Uses {
		uses[o] = append(uses[o], id)
	}
	for _, us := range uses {
		sort.Slice(us, func(i, j int) bool { return us[i].Pos() < us[j].Pos() })
	}
	for _, c := range order {
		cv := convFor(c.elem)
		if !cv.ok {
			c.reason = "element type " + c.elem.String() + " has no known boxing"
			continue
		}
		// The declaration.
		for id, o := range info.Defs {
			if o != c.obj {
				continue
			}
			if spec, ok := parent[id].(*ast.ValueSpec); ok && len(spec.Values) == 1 {
				capacity, ok := isNewList(spec.Values[0])
				if !ok {
					c.fail(spec, "initialized with something other than a new ArrayList")
				} else if vid, isId := unparen(spec.Values[0]).(*ast.Ident); !(isId && vid.Name == "nil") {
					val := spec.Values[0]
					c.edits = append(c.edits, edit{val, func() string { return c.newCode(capacity) }})
				}
			}
		}
		te := c.typeExpr
		c.edits = append(c.edits, edit{te, func() string { return "[]" + text(c.elemExpr) }})
		for _, id := range uses[c.obj] {
			var v ast.Expr = id
			if s, ok := parent[id].(*ast.SelectorExpr); ok && s.Sel == id {
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
			if _, dup := rewrite[e.node]; dup {
				continue
			}
			rewrite[e.node] = e.gen
		}
		if report {
			p := fset.Position(c.obj.Pos())
			fmt.Printf("slice\t%s:%d\t%s\t[]%s\n", filepath.Base(p.Filename), p.Line, c.obj.Name(), text(c.elemExpr))
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
	changed := 0
	for _, f := range files {
		tf := fset.File(f.Pos())
		// The file node starts at `package`; keep what precedes it.
		out := string(src[tf][:tf.Offset(f.Pos())]) + renderChildren(f) + string(src[tf][tf.Offset(f.End()):])
		if out != string(src[tf]) {
			if err := os.WriteFile(tf.Name(), []byte(out), 0o644); err != nil {
				panic(err)
			}
			changed++
		}
	}
	fmt.Fprintf(os.Stderr, "files changed: %d\n", changed)
}
