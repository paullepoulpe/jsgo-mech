package main

// castfold -assert: a Java reference cast that cannot see null, or whose value is dereferenced at
// once, is a Go type assertion.
//
// A checked Java downcast `(X) v` passes null and throws ClassCastException for an object of
// another class. Go's type assertion v.(T) panics with a *runtime.TypeAssertionError for a value
// of another type, which catch_ turns into Java's ClassCastException, so it keeps Java's failure;
// but it also panics for a nil interface, where Java passes null. That is why j2go writes every
// cast as a helper that lets nil through first: cast_[T](v) (if v == nil: the zero T; else
// v.(T)), icast_I(v) (the same, asserting to the Go interface of Java interface I) and to_X(v)
// (for a class with subclasses: v.(X_as).as_X(), the view of the most derived object as an X).
//
// The idiomatic Go downcast is therefore the bare assertion wherever null cannot reach it or
// cannot get past it, and the helper (a nil-passing assertion, which Go does not have) elsewhere:
//
//	cast_[*X](this.self())          ->  this.self().(*X)           the operand is not nil
//	icast_I(v).m(a, b)              ->  v.(I).m(a, b)              dereferenced at once
//	to_X(p.self()).f                ->  p.self().(X_as).as_X().f   (the same, through X's view)
//
// The rewrite keeps Java's behaviour in both cases:
//   - an operand that cannot be nil: the receiver this (never assigned) and values castfold
//     already proves non-nil (composite literals, constructors, locals defined once from them),
//     and their self() (every constructor sets self_ before anything else runs: castfold's
//     self invariant), obj_ of them (non-nil), any(p) of a pointer (never nil: the interface
//     holds the pointer, as the helper's assertion does), and a local variable or parameter that
//     nothing assigns after its declaration, tested against nil before the cast: on the right of
//     `v != nil &&` or `v == nil ||`, in the body of `if v != nil` (and conditions joined to it
//     by &&), in the else branch of `if v == nil`, or after `if v == nil { return/panic/... }` in
//     the same block. On a non-nil operand the helper is exactly its assertion.
//   - a value dereferenced at once: the cast is the operand of a field selection (read, or
//     assigned a value that computes nothing) or the receiver of a Java method call whose
//     arguments compute nothing (constants, variables, fields, operators other than / and %,
//     conversions, function literals). Java casts null and then throws NullPointerException at
//     the dereference; the bare assertion of nil panics with a TypeAssertionError whose concrete
//     type is nil, which catch_ turns into the same NullPointerException (runtime: Java never
//     throws ClassCastException for null, so a failed assertion of nil can only be this). As the
//     arguments compute nothing, nothing Java evaluates between the cast and the dereference is
//     lost. Synthetic methods that Java does not call (self, box_, as_X, up_X) are not
//     dereferences.
//
// Left (counted by reason): casts of values that may be null and are stored or passed on (the
// helper stays: a Go assertion would panic where Java passes null), casts to value types whose
// null is their zero value (enums, Node handles, slices), dereferences whose arguments compute
// (Java evaluates them before the null check). to_X of a small hierarchy (a type switch over the
// concrete classes, from casts -named) stays the helper: it is the faster form of the same check.
// The rewritten casts no longer match, so a rerun changes nothing.
//
// usage: castfold -assert [-n] [-v] <dir of package closure>

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"postflip/rw"
)

func asserts(dry, verbose bool) {
	if !rw.HasFunc("catchNilAssert_") {
		fmt.Fprintln(os.Stderr, "castfold -assert: the runtime does not map an assertion of nil to NullPointerException (catchNilAssert_); dereferenced casts are left")
	}
	done := map[string]int{}
	left := map[string]int{}
	examples := map[string][]string{}
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		var fn *ast.FuncDecl
		ast.Inspect(f, func(n ast.Node) bool {
			if fd, ok := n.(*ast.FuncDecl); ok {
				fn = fd
			}
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) != 1 {
				return true
			}
			kind, target := castOf(c)
			if kind != "cast" && kind != "icast" && kind != "to" {
				return true
			}
			gen, why := assertOf(c, kind, target, fn)
			key := kind + ": " + why
			if gen == nil {
				left[key]++
				if verbose && len(examples[key]) < 5 {
					p := rw.Fset.Position(c.Pos())
					t := rw.Text(c)
					if len(t) > 160 {
						t = t[:160]
					}
					examples[key] = append(examples[key], fmt.Sprintf("%s:%d %s", filepath.Base(p.Filename), p.Line, t))
				}
				return true
			}
			done[key]++
			rw.Rewrite[c] = gen
			return true
		})
	}
	print := func(title string, m map[string]int) {
		total := 0
		for _, n := range m {
			total += n
		}
		fmt.Fprintf(os.Stderr, "castfold -assert: %d %s\n", total, title)
		for _, k := range sortedKeys(m) {
			fmt.Fprintf(os.Stderr, "  %5d %s\n", m[k], k)
			for _, e := range examples[k] {
				fmt.Fprintf(os.Stderr, "          %s\n", e)
			}
		}
	}
	print("casts written as type assertions", done)
	print("casts left", left)
	if dry {
		return
	}
	fmt.Fprintf(os.Stderr, "castfold -assert: %d files written\n", rw.SaveChecked())
}

// assertOf returns the rewrite of cast c as a type assertion, or nil and the reason it is left.
func assertOf(c *ast.CallExpr, kind string, target types.Type, fn *ast.FuncDecl) (func() string, string) {
	// the asserted type and the text after the assertion
	var tText, suffix string
	switch kind {
	case "cast":
		t := types.Unalias(target)
		switch u := t.Underlying().(type) {
		case *types.Pointer:
			if _, ok := types.Unalias(u.Elem()).(*types.Named); !ok {
				return nil, "target a pointer to an unnamed type"
			}
		case *types.Interface:
			if u.Empty() {
				return nil, "target any"
			}
		default:
			return nil, "target a value type (its null is the zero value: an enum, a Node handle, a slice)"
		}
		tText = rw.Text(c.Fun.(*ast.IndexExpr).Index)
	case "icast":
		tText = types.Unalias(target).(*types.Named).Obj().Name()
	case "to":
		x := classOf(target)
		d := funcDecl(rw.Info.Uses[c.Fun.(*ast.Ident)].(*types.Func))
		if d == nil || !generated(d) {
			return nil, "to_X of a runtime class (its own conversion)"
		}
		if d.Body == nil || len(d.Body.List) != 2 {
			return nil, "to_X of an unexpected form"
		}
		if _, ok := d.Body.List[0].(*ast.TypeSwitchStmt); ok {
			return nil, "to_X of a small hierarchy (a type switch over its classes, faster than the assertion)"
		}
		if r, ok := d.Body.List[1].(*ast.ReturnStmt); !ok || len(r.Results) != 1 || rw.Text(r.Results[0]) != "v.("+x.Obj().Name()+"_as).as_"+x.Obj().Name()+"()" {
			return nil, "to_X of an unexpected form"
		}
		tText = x.Obj().Name() + "_as"
		suffix = ".as_" + x.Obj().Name() + "()"
	}
	arg := rw.Unparen(c.Args[0])
	paren := func(e ast.Expr) string {
		s := rw.Render(e)
		switch e.(type) {
		case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr, *ast.TypeAssertExpr:
			return s
		}
		return "(" + s + ")"
	}
	wrap, y := unwrap(arg)
	if why, ok := nonNull(arg, fn); ok {
		if wrap == "obj_" {
			// obj_(y) of a non-nil y is any(y)
			return func() string { return "any(" + rw.Render(y) + ").(" + tText + ")" + suffix }, "operand not null: " + why
		}
		return func() string { return paren(arg) + ".(" + tText + ")" + suffix }, "operand not null: " + why
	}
	gen := func() string { return paren(arg) + ".(" + tText + ")" + suffix }
	d, dwhy := deref(c)
	if d {
		if !rw.HasFunc("catchNilAssert_") {
			return nil, "dereferenced at once, but the runtime does not map an assertion of nil to NullPointerException"
		}
		return gen, "dereferenced at once (" + dwhy + ")"
	}
	if dwhy != "" {
		return nil, dwhy
	}
	return nil, "operand may be null and the value is stored or passed on: " + nullSource(arg)
}

// nullSource names where a possibly-null operand comes from.
func nullSource(arg ast.Expr) string {
	wrap, y := unwrap(arg)
	switch wrap {
	case "self":
		return "self() of a pointer that may be nil"
	case "obj_":
		return "obj_ of a pointer that may be nil"
	}
	if t := rw.Info.TypeOf(y); t != nil && !isAny(t) && types.IsInterface(t) {
		return "an interface value"
	}
	return anySource(y)
}

// nonNull reports whether the operand of a cast (of type any) cannot be nil, and why.
func nonNull(arg ast.Expr, fn *ast.FuncDecl) (string, bool) {
	wrap, y := unwrap(arg)
	ty := rw.Info.TypeOf(y)
	if ty == nil {
		return "", false
	}
	_, ptr := types.Unalias(ty).(*types.Pointer)
	switch wrap {
	case "any", "-":
		if ptr {
			return "a pointer converted to any", true
		}
		if isAny(ty) || types.IsInterface(ty) {
			if guarded(y, arg) {
				return "tested against nil before", true
			}
		}
		return "", false
	case "obj_", "self":
		if !ptr {
			return "", false
		}
		if wrap == "self" {
			if x := classOf(ty); x == nil || !selfOK[x] {
				return "", false
			}
		}
		if nonNil(y, fn, 0) {
			return wrap + " of a pointer that is not nil", true
		}
		if guarded(y, arg) {
			return wrap + " of a pointer tested against nil before", true
		}
	}
	return "", false
}

// guarded reports whether e is a local variable or parameter that nothing assigns after its
// declaration and that is tested not nil on every path to use (inside expression or statement
// tree containing use).
func guarded(e ast.Expr, use ast.Node) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	v, ok := rw.Info.Uses[id].(*types.Var)
	if !ok || v.IsField() || v.Parent() == rw.Pkg.Scope() || assigned(v) {
		return false
	}
	isNil := func(x ast.Expr, op token.Token) bool {
		b, ok := rw.Unparen(x).(*ast.BinaryExpr)
		if !ok || b.Op != op {
			return false
		}
		l, r := rw.Unparen(b.X), rw.Unparen(b.Y)
		if n, ok := r.(*ast.Ident); !ok || n.Name != "nil" || rw.Info.Uses[n] != types.Universe.Lookup("nil") {
			return false
		}
		lid, ok := l.(*ast.Ident)
		return ok && rw.Info.Uses[lid] == v
	}
	// terms of a chain of op (&& or ||)
	var terms func(x ast.Expr, op token.Token) []ast.Expr
	terms = func(x ast.Expr, op token.Token) []ast.Expr {
		if b, ok := rw.Unparen(x).(*ast.BinaryExpr); ok && b.Op == op {
			return append(terms(b.X, op), terms(b.Y, op)...)
		}
		return []ast.Expr{x}
	}
	tests := func(x ast.Expr, op, cmp token.Token) bool {
		for _, t := range terms(x, op) {
			if isNil(t, cmp) {
				return true
			}
		}
		return false
	}
	child := use
	for p := rw.Parent[use]; p != nil; child, p = p, rw.Parent[p] {
		switch q := p.(type) {
		case *ast.BinaryExpr:
			if child == q.Y && (q.Op == token.LAND && tests(q.X, token.LAND, token.NEQ) || q.Op == token.LOR && tests(q.X, token.LOR, token.EQL)) {
				return true
			}
		case *ast.IfStmt:
			if child == q.Body && tests(q.Cond, token.LAND, token.NEQ) {
				return true
			}
			if child == q.Else && tests(q.Cond, token.LOR, token.EQL) {
				return true
			}
		case *ast.BlockStmt:
			for _, s := range q.List {
				if s == child {
					break
				}
				if is, ok := s.(*ast.IfStmt); ok && is.Init == nil && is.Else == nil && terminates(is.Body) && tests(is.Cond, token.LOR, token.EQL) {
					return true
				}
			}
		case *ast.CaseClause:
			for _, s := range q.Body {
				if s == child {
					break
				}
				if is, ok := s.(*ast.IfStmt); ok && is.Init == nil && is.Else == nil && terminates(is.Body) && tests(is.Cond, token.LOR, token.EQL) {
					return true
				}
			}
		case *ast.FuncDecl:
			return false
		}
	}
	return false
}

// deref reports whether the value of cast c is dereferenced at once (and how), or why not when
// it is dereferenced in a way the rewrite cannot keep.
func deref(c *ast.CallExpr) (bool, string) {
	s, ok := rw.Up(c).(*ast.SelectorExpr)
	if !ok || rw.Unparen(s.X) != c {
		return false, ""
	}
	sel := rw.Info.Selections[s]
	if sel == nil {
		return false, ""
	}
	switch sel.Kind() {
	case types.FieldVal:
		switch q := rw.Up(s).(type) {
		case *ast.AssignStmt:
			for _, l := range q.Lhs {
				if rw.Unparen(l) == s {
					for _, r := range q.Rhs {
						if !computesNothing(r) {
							return false, "a field assigned a value that computes (Java computes it before the null check)"
						}
					}
					return true, "field assigned"
				}
			}
		case *ast.UnaryExpr:
			if q.Op == token.AND {
				return false, "address of a field"
			}
		}
		return true, "field read"
	case types.MethodVal:
		name := sel.Obj().Name()
		if name == "self" || name == "box_" || strings.HasPrefix(name, "as_") || strings.HasPrefix(name, "up_") {
			return false, ""
		}
		call, ok := rw.Up(s).(*ast.CallExpr)
		if !ok || call.Fun != s {
			return false, "a method value (not called)"
		}
		for _, a := range call.Args {
			if !computesNothing(a) {
				return false, "a method called with arguments that compute (Java computes them before the null check)"
			}
		}
		return true, "method called"
	}
	return false, ""
}

// computesNothing reports whether evaluating e calls nothing and cannot fail other than by a nil
// dereference: constants, variables, fields, operators other than / and %, conversions,
// function literals and composite literals of such.
func computesNothing(e ast.Expr) bool {
	switch x := e.(type) {
	case *ast.Ident, *ast.BasicLit, *ast.FuncLit:
		return true
	case *ast.ParenExpr:
		return computesNothing(x.X)
	case *ast.SelectorExpr:
		if sel := rw.Info.Selections[x]; sel != nil && sel.Kind() != types.FieldVal {
			return false
		}
		if _, ok := x.X.(*ast.Ident); ok {
			return true // a field of a variable, or a package-qualified name
		}
		return computesNothing(x.X)
	case *ast.StarExpr:
		return computesNothing(x.X)
	case *ast.UnaryExpr:
		return x.Op != token.ARROW && computesNothing(x.X)
	case *ast.BinaryExpr:
		return x.Op != token.QUO && x.Op != token.REM && computesNothing(x.X) && computesNothing(x.Y)
	case *ast.CompositeLit:
		for _, el := range x.Elts {
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				el = kv.Value
			}
			if !computesNothing(el) {
				return false
			}
		}
		return true
	case *ast.CallExpr:
		if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() && len(x.Args) == 1 {
			// a conversion; to an interface it cannot fail, between numbers neither
			return computesNothing(x.Args[0])
		}
	}
	return false
}
