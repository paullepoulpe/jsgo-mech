package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"postflip/rw"
)

// A checked value that is computed (a call, a chain of fields) is hoisted into a variable when
// the statement around the check computes it before anything else that could be observed:
//
//	var t *JSType = Preconditions_checkNotNullT_Object(left.getJSType(), left.box_())
//	receiver.setJSType(Preconditions_checkNotNullT(var_.getType()))
//
// become
//
//	var t *JSType = left.getJSType()
//	if t == nil {
//		nullFail_(objstr_(left.box_()))
//	}
//	nn_1 := var_.getType()
//	if nn_1 == nil {
//		panic(new_NullPointerException_0().self())
//	}
//	receiver.setJSType(nn_1)
//
// Java computes the value, then the message arguments, then tests the value. Everything the
// statement evaluates before the value must be a read that computing the value cannot change: a
// constant, `this`, a local variable that no function literal assigns and whose address is not
// taken, a package variable that no function assigns. Each message argument must be a read, a
// field of one or the box of one (computing it on the failure path only is the same as
// computing it before the test). A check that is the whole statement becomes
// `if <value> == <zero> { ... }`; a declaration of a variable of the value's type is followed by
// the test of that variable; a variable or a field of one (inside an expression) is tested
// before the statement and read again by it; otherwise the value goes into a new variable
// nn_<k> (the first k whose name the function does not use).

var hoisted int

// done are the checks already moved before their statement (their place reads a value now), and
// pre the tests put before each statement, in source order.
var (
	done = map[ast.Expr]bool{}
	pre  = map[ast.Stmt][]string{}
)

// before puts text before statement st (after the tests already put there) and replaces c with
// the read r.
func before(st ast.Stmt, c ast.Expr, text, r string) {
	done[c] = true
	rw.Rewrite[c] = func() string { return r }
	if len(pre[st]) == 0 {
		rw.Rewrite[st] = func() string { return strings.Join(pre[st], "") + rw.RenderChildren(st) }
	}
	pre[st] = append(pre[st], text)
}

func simple(e ast.Expr) bool { // a variable, or a field of this
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		_, isVar := rw.Info.Uses[x].(*types.Var)
		return isVar
	case *ast.SelectorExpr:
		id, ok := x.X.(*ast.Ident)
		return ok && id.Name == "this"
	}
	return false
}

func zv0(t types.Type) string { z, _ := zero(t); return z }

func zero(t types.Type) (string, bool) {
	switch u := t.Underlying().(type) {
	case *types.Pointer, *types.Interface, *types.Map, *types.Slice, *types.Chan, *types.Signature:
		return "nil", true
	case *types.Basic:
		if u.Info()&types.IsString != 0 {
			return "", false
		}
		if u.Info()&types.IsNumeric != 0 {
			return "0", true
		}
	}
	return "", false
}

// throwOf is the panic of the helper's failure path, with the call's message arguments.
func throwOf(variant string, msg []ast.Expr) (string, bool) {
	switch variant {
	case "":
		return "panic(new_NullPointerException_0().self())", true
	case "_Object":
		return "nullFail_(objstr_(" + rw.Render(msg[0]) + "))", true
	case "_ofNullable": // Optional.ofNullable(v).orElseThrow() / .get()
		return "panic(new_NoSuchElementException_String(\"No value present\").self())", true
	case "_String_Object", "_String_Object_Object":
		ps := []string{}
		for _, a := range msg[1:] {
			ps = append(ps, rw.Render(a))
		}
		return "nullFail_(Strings_lenientFormat(" + rw.Render(msg[0]) + ", []any{" + strings.Join(ps, ", ") + "}))", true
	}
	return "", false
}

// modified are the variables a call can change: local variables a function literal assigns
// (other than its own) or whose address is taken, and package variables assigned in a function.
var modified = map[*types.Var]bool{}

func markModified() {
	for _, f := range rw.Files {
		var lits []*ast.FuncLit
		var walk func(n ast.Node) bool
		mark := func(e ast.Expr) {
			id, ok := rw.Unparen(e).(*ast.Ident)
			if !ok {
				return
			}
			v, ok := rw.Info.Uses[id].(*types.Var)
			if !ok {
				return
			}
			if v.Parent() == rw.Pkg.Scope() {
				modified[v] = true // a package variable assigned in a function
				return
			}
			for _, l := range lits {
				if v.Pos() < l.Pos() || v.Pos() >= l.End() {
					modified[v] = true
					return
				}
			}
		}
		walk = func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncLit:
				lits = append(lits, x)
				ast.Inspect(x.Body, walk)
				lits = lits[:len(lits)-1]
				return false
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					mark(l)
				}
			case *ast.IncDecStmt:
				mark(x.X)
			case *ast.RangeStmt:
				if x.Key != nil {
					mark(x.Key)
				}
				if x.Value != nil {
					mark(x.Value)
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					if id, ok := rw.Unparen(x.X).(*ast.Ident); ok {
						if v, ok := rw.Info.Uses[id].(*types.Var); ok {
							modified[v] = true
						}
					}
				}
			}
			return true
		}
		ast.Inspect(f, walk)
	}
}

// read: a constant, nil, a function or type name, or a variable no call can change.
func read(e ast.Expr) bool {
	e = rw.Unparen(e)
	if done[e] {
		return true
	}
	if tv, ok := rw.Info.Types[e]; ok && (tv.Value != nil || tv.IsType() || tv.IsNil()) {
		return true
	}
	switch x := e.(type) {
	case *ast.BasicLit:
		return true
	case *ast.Ident:
		switch o := rw.Info.Uses[x].(type) {
		case *types.Const, *types.Nil, *types.Func, *types.TypeName, *types.Builtin:
			return true
		case *types.Var:
			return o.Parent() != nil && !o.IsField() && !modified[o]
		}
	case *ast.IndexExpr: // an instantiated generic function
		if tv, ok := rw.Info.Types[x.X]; ok && !tv.IsValue() {
			return true
		}
		if id, ok := x.X.(*ast.Ident); ok {
			_, isFunc := rw.Info.Uses[id].(*types.Func)
			return isFunc
		}
	case *ast.SelectorExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			if _, isPkg := rw.Info.Uses[id].(*types.PkgName); isPkg {
				return true
			}
		}
	}
	return false
}

// msgRead: a message argument whose computation can only be observed through its value: a
// read, a field of a read, or obj_/strobj_/self()/box_() of one.
func msgRead(e ast.Expr) bool {
	e = rw.Unparen(e)
	if read(e) {
		return true
	}
	switch x := e.(type) {
	case *ast.Ident:
		_, isVar := rw.Info.Uses[x].(*types.Var)
		return isVar
	case *ast.SelectorExpr:
		if sel, ok := rw.Info.Selections[x]; ok && sel.Kind() == types.FieldVal {
			return msgRead(x.X)
		}
	case *ast.CallExpr:
		if f, ok := x.Fun.(*ast.Ident); ok && (f.Name == "strobj_" || f.Name == "obj_" || f.Name == "any") && len(x.Args) == 1 {
			return msgRead(x.Args[0])
		}
		if s, ok := x.Fun.(*ast.SelectorExpr); ok && len(x.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
			return msgRead(s.X)
		}
	}
	return false
}

// readPath: a read, or a field of one.
func readPath(e ast.Expr) bool {
	e = rw.Unparen(e)
	if s, ok := e.(*ast.SelectorExpr); ok {
		if sel, ok := rw.Info.Selections[s]; ok && sel.Kind() == types.FieldVal {
			return readPath(s.X)
		}
	}
	if id, ok := e.(*ast.Ident); ok {
		if v, ok := rw.Info.Uses[id].(*types.Var); ok && !v.IsField() && v.Parent() != rw.Pkg.Scope() {
			return true // a local variable (read twice with nothing in between)
		}
	}
	return read(e)
}

func contains(n ast.Node, c ast.Node) bool { return c.Pos() >= n.Pos() && c.End() <= n.End() }

// first reports whether evaluating e computes c before anything other than reads.
func first(e ast.Expr, c ast.Expr) bool {
	if e == c {
		return true
	}
	parts := func(ps ...ast.Expr) bool {
		for _, p := range ps {
			if p == nil {
				continue
			}
			if contains(p, c) {
				return first(p, c)
			}
			if !read(p) {
				return false
			}
		}
		return false
	}
	switch x := e.(type) {
	case *ast.ParenExpr:
		return first(x.X, c)
	case *ast.StarExpr:
		return first(x.X, c)
	case *ast.UnaryExpr:
		return x.Op != token.ARROW && first(x.X, c)
	case *ast.SelectorExpr:
		return first(x.X, c)
	case *ast.TypeAssertExpr:
		return first(x.X, c)
	case *ast.IndexExpr:
		return parts(x.X, x.Index)
	case *ast.BinaryExpr:
		if x.Op == token.LAND || x.Op == token.LOR {
			return parts(x.X)
		}
		return parts(x.X, x.Y)
	case *ast.CallExpr:
		ps := []ast.Expr{}
		switch f := x.Fun.(type) {
		case *ast.SelectorExpr:
			if read(f) {
				break
			}
			if sel, ok := rw.Info.Selections[f]; ok && sel.Kind() == types.MethodVal {
				ps = append(ps, f.X)
			} else {
				ps = append(ps, f)
			}
		default:
			ps = append(ps, f)
		}
		return parts(append(ps, x.Args...)...)
	}
	return false
}

// pureTarget: an assignment target whose evaluation reads only: a variable, or a field of a read.
func pureTarget(e ast.Expr) bool {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return read(x.X)
	}
	return false
}

var used = map[*ast.FuncDecl]map[string]bool{}

func tempName(n ast.Node) string {
	var fd *ast.FuncDecl
	for p := n; p != nil; p = rw.Parent[p] {
		if d, ok := p.(*ast.FuncDecl); ok {
			fd = d
		}
	}
	if used[fd] == nil {
		used[fd] = map[string]bool{}
		ast.Inspect(fd, func(m ast.Node) bool {
			if id, ok := m.(*ast.Ident); ok {
				used[fd][id.Name] = true
			}
			return true
		})
	}
	for k := 1; ; k++ {
		name := fmt.Sprintf("nn_%d", k)
		if !used[fd][name] {
			used[fd][name] = true
			return name
		}
	}
}

// hoist rewrites the check c of value v whose value is computed, or says why not.
func hoist(c ast.Expr, v ast.Expr, msg []ast.Expr, variant string) string {
	v = rw.Unparen(v)
	t := rw.Info.Types[c].Type
	isNull := func(name string) string { return name + " == " + zv0(t) }
	if variant == "_ofNullable" { // Optional.ofNullable's test: isnull_
		switch u := t.Underlying().(type) {
		case *types.Interface:
			isNull = func(name string) string { return "isnull_(" + name + ")" }
		case *types.Pointer:
		case *types.Basic:
			if u.Info()&types.IsString == 0 {
				return "Optional.ofNullable of a value that is not an object"
			}
			isNull = func(name string) string { return name + " == nullstr_" }
		default:
			return "Optional.ofNullable of a value that is not an object"
		}
	} else if _, ok := zero(t); !ok {
		return "checked value is a string or a struct (not the helper's nil test)"
	}
	for _, a := range msg {
		if !msgRead(a) {
			return "a message argument computes something (Java computes it before the test)"
		}
	}
	throw, ok := throwOf(variant, msg)
	if !ok {
		return "variant " + variant
	}
	same := types.Identical(rw.Info.Types[v].Type, t)
	// The statement around the check, and its expression that holds the check.
	var stmt ast.Stmt
	var root ast.Expr
	var p ast.Node = c
	for {
		p = rw.Parent[p]
		if _, ok := p.(ast.Expr); !ok {
			break
		}
	}
	vs := rw.Render(v)
	decl := func(name string) string {
		if same {
			return name + " := " + vs + "\n"
		}
		return "var " + name + " " + types.TypeString(t, types.RelativeTo(rw.Pkg)) + " = " + vs + "\n"
	}
	test := func(name string) string { return "if " + isNull(name) + " {\n" + throw + "\n}\n" }
	switch s := p.(type) {
	case *ast.ExprStmt:
		stmt, root = s, s.X
		if rw.Unparen(s.X) == ast.Expr(c) && same {
			if !inList(s) {
				return "statement is not in a statement list"
			}
			rw.Rewrite[s] = func() string { return strings.TrimSuffix(test(rw.Render(v)), "\n") }
			return ""
		}
	case *ast.AssignStmt:
		if len(s.Lhs) != 1 || len(s.Rhs) != 1 {
			return "not the value of a statement, assignment, declaration or return"
		}
		stmt, root = s, s.Rhs[0]
		if s.Tok == token.DEFINE && rw.Unparen(s.Rhs[0]) == ast.Expr(c) && same {
			if !inList(s) {
				return "statement is not in a statement list"
			}
			name := rw.Text(s.Lhs[0])
			rw.Rewrite[c] = func() string { return vs }
			rw.Rewrite[s] = func() string { return rw.RenderChildren(s) + "\n" + strings.TrimSuffix(test(name), "\n") }
			return ""
		}
		if s.Tok != token.DEFINE && !pureTarget(s.Lhs[0]) {
			return "the assignment target computes something"
		}
	case *ast.ValueSpec:
		g, ok := rw.Parent[s].(*ast.GenDecl)
		if !ok || len(s.Names) != 1 || len(s.Values) != 1 || len(g.Specs) != 1 || g.Tok != token.VAR {
			return "not the value of a statement, assignment, declaration or return"
		}
		d, ok := rw.Parent[g].(*ast.DeclStmt)
		if !ok {
			return "not the value of a statement, assignment, declaration or return"
		}
		stmt, root = d, s.Values[0]
		if rw.Unparen(s.Values[0]) == ast.Expr(c) && same && types.Identical(rw.Info.Defs[s.Names[0]].Type(), rw.Info.Types[v].Type) {
			if !inList(d) {
				return "statement is not in a statement list"
			}
			name := s.Names[0].Name
			rw.Rewrite[c] = func() string { return vs }
			rw.Rewrite[d] = func() string { return rw.RenderChildren(d) + "\n" + strings.TrimSuffix(test(name), "\n") }
			return ""
		}
	case *ast.ReturnStmt:
		if len(s.Results) == 0 {
			return "not the value of a statement, assignment, declaration or return"
		}
		stmt, root = s, s.Results[0]
	case *ast.IfStmt:
		if s.Init != nil || !contains(s.Cond, c) {
			return "in the init statement of an if"
		}
		if e, ok := rw.Parent[s].(*ast.IfStmt); ok && e.Else == ast.Stmt(s) {
			return "in the condition of an else-if (computed only when the conditions before fail)"
		}
		stmt, root = s, s.Cond
	case *ast.SwitchStmt:
		if s.Init != nil || s.Tag == nil || !contains(s.Tag, c) {
			return "in a switch case or init"
		}
		stmt, root = s, s.Tag
	case *ast.RangeStmt:
		if !contains(s.X, c) {
			return "in a range key or value"
		}
		stmt, root = s, s.X
	case *ast.ForStmt:
		return "in a for condition or post statement (computed on each iteration)"
	default:
		return "not the value of a statement, assignment, declaration or return"
	}
	if !first(root, c) {
		if inLogical(root, c) {
			return "the right operand of && or || (computed only when the left one allows)"
		}
		return "the statement computes something before the checked value"
	}
	if !inList(stmt) {
		return "statement is not in a statement list"
	}
	if readPath(v) && same { // read again by the statement: nothing in between
		before(stmt, c, test(vs), vs)
		return ""
	}
	name := tempName(c)
	before(stmt, c, decl(name)+test(name), name)
	return ""
}

func inList(s ast.Stmt) bool {
	switch rw.Parent[s].(type) {
	case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
		return true
	}
	return false
}

func inLogical(root ast.Expr, c ast.Expr) bool {
	found := false
	ast.Inspect(root, func(n ast.Node) bool {
		if b, ok := n.(*ast.BinaryExpr); ok && (b.Op == token.LAND || b.Op == token.LOR) && contains(b.Y, c) {
			found = true
		}
		return !found
	})
	return found
}

// erased rewrites the erased checkNotNull of a value castfold left boxed:
//
//	Node_of_(Preconditions_checkNotNull(n.box_()))   n a node handle: box_ is nil iff n is 0
//	to_X(Preconditions_checkNotNull(x.self()))      x of type X, not an interface: self() is nil
//	                                                iff x is, and to_X gives x back
//	Preconditions_checkNotNull_Object_Object(any(s), msg)   s a slice, as a statement
//
// as the typed check of n or x (hoist). In the third, j2go's any(s) of a nil slice is not nil, so
// the helper never failed where Java's test of the array reference does; the test is s == nil
// (a Java array that exists is never a nil slice: j2go makes arrays with make).
func erased(c *ast.CallExpr, fname string) string {
	if fname == "Preconditions_checkNotNull_Object_Object" {
		st, ok := rw.Up(c).(*ast.ExprStmt)
		a, isCall := rw.Unparen(c.Args[0]).(*ast.CallExpr)
		if !ok || !isCall || len(a.Args) != 1 || rw.Text(a.Fun) != "any" || !inList(st) {
			return "erased check of a box (casts and boxing steps)"
		}
		if _, isSlice := rw.Info.Types[a.Args[0]].Type.Underlying().(*types.Slice); !isSlice || !readPath(a.Args[0]) || !msgRead(c.Args[1]) {
			return "erased check of a box (casts and boxing steps)"
		}
		throw, _ := throwOf("_Object", c.Args[1:])
		rw.Rewrite[st] = func() string { return "if " + rw.Render(a.Args[0]) + " == nil {\n" + throw + "\n}" }
		return ""
	}
	if len(c.Args) != 1 {
		return "erased check of a box (casts and boxing steps)"
	}
	outer, ok := rw.Up(c).(*ast.CallExpr)
	box, ok2 := rw.Unparen(c.Args[0]).(*ast.CallExpr)
	if !ok || !ok2 || len(outer.Args) != 1 || len(box.Args) != 0 {
		return "erased check of a box (casts and boxing steps)"
	}
	sel, ok := box.Fun.(*ast.SelectorExpr)
	if !ok {
		return "erased check of a box (casts and boxing steps)"
	}
	x := sel.X
	xt := rw.Info.Types[x].Type
	ot := rw.Info.Types[outer].Type
	if xt == nil || ot == nil || !types.Identical(xt, ot) {
		return "erased check of a box (casts and boxing steps)"
	}
	name := rw.Text(outer.Fun)
	switch {
	case name == "Node_of_" && sel.Sel.Name == "box_":
	case strings.HasPrefix(name, "to_") && sel.Sel.Name == "self" && !types.IsInterface(xt):
	default:
		return "erased check of a box (casts and boxing steps)"
	}
	return hoist(outer, x, nil, "")
}

// nested: c has a checkNotNull call in its arguments (its text is rendered once, before the inner
// one would be rewritten).
func nested(c *ast.CallExpr) bool {
	found := false
	for _, a := range c.Args {
		ast.Inspect(a, func(n ast.Node) bool {
			if e, ok := n.(ast.Expr); ok {
				if name, _ := rw.CallName(e); strings.HasPrefix(name, "Preconditions_checkNotNull") {
					found = true
				}
			}
			return !found
		})
	}
	return found
}

// optional rewrites the java.util.Optional made only to test its content for null:
//
//	Optional_ofNullable(v).isPresent()        !isnull_(v) (v of interface type), v != nil,
//	                                          s != nullstr_ (v = strobj_(s))
//	Optional_ofNullable(v).orElseThrow()      (.get() too) the check of v with Optional.get's
//	                                          NoSuchElementException, moved before the
//	                                          statement as the checks are (hoist)
//
// Optional_ofNullable's own test is isnull_(v); for a pointer it is v == nil, for a String
// v == nullstr_; strobj_(s) is null iff s is nullstr_. The Optional is not seen elsewhere.
func optional(c *ast.CallExpr) (string, bool) {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || len(c.Args) != 0 {
		return "", false
	}
	inner, ok := rw.Unparen(sel.X).(*ast.CallExpr)
	if !ok || len(inner.Args) != 1 {
		return "", false
	}
	if name, _ := rw.CallName(inner); name != "Optional_ofNullable" {
		return "", false
	}
	v := rw.Unparen(inner.Args[0])
	switch sel.Sel.Name {
	case "isPresent":
		t := rw.Info.Types[v].Type
		var text string
		if b, ok := v.(*ast.CallExpr); ok && rw.Text(b.Fun) == "strobj_" && len(b.Args) == 1 {
			text = "(" + rw.Render(b.Args[0]) + " != nullstr_)"
		} else {
			switch u := t.Underlying().(type) {
			case *types.Interface:
				text = "!isnull_(" + rw.Render(v) + ")"
			case *types.Pointer:
				text = "(" + rw.Render(v) + " != nil)"
			case *types.Basic:
				if u.Info()&types.IsString == 0 {
					return "Optional.ofNullable of a value that is not an object", true
				}
				text = "(" + rw.Render(v) + " != nullstr_)"
			default:
				return "Optional.ofNullable of a value that is not an object", true
			}
		}
		rw.Rewrite[c] = func() string { return text }
		return "", true
	case "orElseThrow", "get":
		// str_(Optional_ofNullable(strobj_(s)).orElseThrow()) is the check of s.
		if u, ok := rw.Up(c).(*ast.CallExpr); ok && rw.Text(u.Fun) == "str_" && len(u.Args) == 1 {
			if b, ok := v.(*ast.CallExpr); ok && rw.Text(b.Fun) == "strobj_" && len(b.Args) == 1 {
				return hoist(u, b.Args[0], nil, "_ofNullable"), true
			}
		}
		return hoist(c, v, nil, "_ofNullable"), true
	}
	return "", false
}
