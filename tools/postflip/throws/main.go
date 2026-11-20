// Command throws is a post-flip step: a Java throw of a new exception panics with the exception,
// a Go error value, instead of with the exception's Java object reference.
//
// j2go writes `throw new X(...)` as panic(new_X(...).self()) (or panic(obj_(new_X(...))) when
// the class is not one of the runtime's), the Object reference of the new object. A class of
// java.lang.Throwable is a Go error (rt_object.go: (*Throwable).Error, promoted to every
// exception class), and panicking with an error is how Go reports an unrecoverable programmer
// error, which is what Java's unchecked exceptions are (IllegalStateException,
// IllegalArgumentException, NullPointerException, UnsupportedOperationException,
// AssertionError: 3,000 of the 3,500 throws). So the step writes
//
//	panic(new_X(args).self())   ->  panic(new_X(args))
//	panic(obj_(new_X(args)))    ->  panic(new_X(args))
//	panic(f(args).self())       ->  panic(f(args))      f returns only new exceptions
//
// The panic value is the same: new_X sets the object's self_ to itself (`this := &X{};
// this.self_ = this; ...; return this`, checked for each constructor), self() returns self_ of
// a non-nil object (checked: its body is `if this == nil { return nil }; return this.self_`),
// and obj_ of a non-nil pointer is the pointer. A recovered value is the same *X, so catch
// clauses, the top-level report (class, message, cause chain) and every other observer see
// the same exception. A function f qualifies when every return statement of its body returns
// such a new object directly.
//
// A Java switch expression over an enum's constants that j2go writes as a switch whose cases
// all return, followed by panic("unreachable"), reaches the panic only for a value no case
// names. For an integer enum (tools/postflip/enums) whose cases name every constant that value
// is 0, Java's null, and Java's switch on a null enum throws NullPointerException; the panic
// is that exception, panic(new_NullPointerException_0()), not a RuntimeException made of a
// string. (A NullPointerException has no message in this port, as those of Go nil
// dereferences, catch_.)
//
// Every other panic is left and counted by kind: rethrows of a caught value, throws of an
// exception held in a variable (its self() is the most derived object, which a view such as
// *Throwable is not), Enum.valueOf failures (enumValueOfFail_, typed by the runtime), and
// panics with a string (abstract-method fallbacks and stubs j2go writes for code Java never
// reaches, and panic("unreachable") after an emulated try).
//
// usage: throws [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

var count = map[string]int{}

var funcDecls = map[*types.Func]*ast.FuncDecl{}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				funcDecls[rw.Info.Defs[fd.Name].(*types.Func)] = fd
			}
		}
	}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue // the handwritten runtime is read, not rewritten
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if es, ok := n.(*ast.ExprStmt); ok && rw.Text(es) == `panic("unreachable")` {
				if enumSwitchEnd(es) {
					count[`exhaustive switch on an enum: panic("unreachable") -> panic(new_NullPointerException_0()), Java's switch on null`]++
					rw.Rewrite[es] = func() string { return "panic(new_NullPointerException_0())" }
					return false
				}
			}
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) != 1 {
				return true
			}
			id, ok := c.Fun.(*ast.Ident)
			if !ok || id.Name != "panic" {
				return true
			}
			if _, ok := rw.Info.Uses[id].(*types.Builtin); !ok {
				return true
			}
			visit(c.Args[0])
			return true
		})
	}
	var keys []string
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%5d %s\n", count[k], k)
	}
	if !report {
		fmt.Printf("throws: %d files changed\n", rw.SaveChecked())
	}
}

func visit(arg ast.Expr) {
	a := rw.Unparen(arg)
	// panic(x.self()) / panic(obj_(x)) with x a new exception.
	var inner ast.Expr
	how := ""
	if c, ok := a.(*ast.CallExpr); ok {
		switch fun := c.Fun.(type) {
		case *ast.SelectorExpr:
			if fun.Sel.Name == "self" && len(c.Args) == 0 && selfMethod(fun) {
				inner, how = rw.Unparen(fun.X), "self()"
			}
		case *ast.Ident:
			if fun.Name == "obj_" && len(c.Args) == 1 {
				inner, how = rw.Unparen(c.Args[0]), "obj_"
			}
		case *ast.IndexExpr:
			if rw.Text(fun.X) == "obj_" && len(c.Args) == 1 {
				inner, how = rw.Unparen(c.Args[0]), "obj_"
			}
		}
	}
	if inner != nil {
		if kind := fresh(inner, 0); kind != "" {
			count["throw of "+kind+": panic(... "+how+") -> panic(the exception)"]++
			x := inner
			rw.Rewrite[arg] = func() string { return rw.Render(x) }
			return
		}
	}
	text := rw.Text(a)
	switch {
	case fresh(a, 0) != "":
		count["done: panic of "+fresh(a, 0)+" (a Go error value)"]++
	case text == "r_":
		count["left: rethrow of the recovered value in a catch defer (panic(r_), the Go idiom for a recover that does not handle the value)"]++
	case text == `"unreachable"`:
		count[`left: panic("unreachable") `+unreachableKind(arg)]++
	case strings.HasPrefix(text, "enumValueOfFail_("):
		count["left: Enum.valueOf of an unknown name, panic(enumValueOfFail_(...)) of a new IllegalArgumentException (an error value)"]++
	case isString(a):
		count["left: panic with a string (abstract-method fallback or j2go stub that Java never reaches)"]++
	case inner != nil:
		count["left: throw of an exception held in a variable or a field (self() is the most derived object; the static type may be a view)"]++
	default:
		count["left: throw of another value ("+shape(a)+")"]++
	}
}

func shape(a ast.Expr) string {
	t := rw.Text(a)
	if i := strings.IndexAny(t, "("); i > 0 {
		t = t[:i]
	}
	return t
}

func isString(e ast.Expr) bool {
	t := rw.Info.Types[e].Type
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsString != 0
}

// selfMethod reports whether s (x.self) is the generated self(): `if this == nil { return nil };
// return this.self_`.
func selfMethod(s *ast.SelectorExpr) bool {
	sel := rw.Info.Selections[s]
	if sel == nil {
		return false
	}
	fd := funcDecls[sel.Obj().(*types.Func)]
	if fd == nil || fd.Recv == nil || len(fd.Body.List) != 2 {
		return false
	}
	return strings.Join(strings.Fields(rw.Text(fd.Body.List[0])), " ") == "if this == nil { return nil }" && rw.Text(fd.Body.List[1]) == "return this.self_"
}

// fresh reports what x is when it is a new exception object whose self_ is itself: a call of
// a constructor new_X of an exception class, or of a function all of whose returns are such
// calls. It returns "" otherwise.
func fresh(x ast.Expr, depth int) string {
	c, ok := x.(*ast.CallExpr)
	if !ok {
		return ""
	}
	var f *types.Func
	switch fun := c.Fun.(type) {
	case *ast.Ident:
		f, _ = rw.Info.Uses[fun].(*types.Func)
	case *ast.SelectorExpr:
		if sel := rw.Info.Selections[fun]; sel != nil {
			f, _ = sel.Obj().(*types.Func)
		} else {
			f, _ = rw.Info.Uses[fun.Sel].(*types.Func)
		}
	}
	if f == nil {
		return ""
	}
	fd := funcDecls[f]
	if fd == nil {
		return ""
	}
	if strings.HasPrefix(f.Name(), "new_") && fd.Recv == nil {
		if constructor(fd) {
			return "a new exception"
		}
		return ""
	}
	if depth > 0 {
		return ""
	}
	// A helper whose every return is a new exception.
	n := 0
	good := true
	ast.Inspect(fd.Body, func(m ast.Node) bool {
		switch r := m.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(r.Results) != 1 || fresh(rw.Unparen(r.Results[0]), depth+1) == "" {
				good = false
			}
			n++
		}
		return good
	})
	if good && n > 0 {
		return "a new exception made by a helper"
	}
	return ""
}

// constructor reports whether fd is `func new_X(...) *X { this := &X{}; this.self_ = this; ...;
// return this }` for an exception class X, with no other return.
func constructor(fd *ast.FuncDecl) bool {
	l := fd.Body.List
	if len(l) < 3 || rw.Text(l[1]) != "this.self_ = this" || rw.Text(l[len(l)-1]) != "return this" {
		return false
	}
	as, ok := l[0].(*ast.AssignStmt)
	if !ok || len(as.Lhs) != 1 || rw.Text(as.Lhs[0]) != "this" || !strings.HasPrefix(rw.Text(as.Rhs[0]), "&") {
		return false
	}
	returns := 0
	ast.Inspect(fd.Body, func(m ast.Node) bool {
		switch m.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			returns++
		}
		return true
	})
	if returns != 1 {
		return false
	}
	sig := rw.Info.Defs[fd.Name].(*types.Func).Type().(*types.Signature)
	if sig.Results().Len() != 1 {
		return false
	}
	p, ok := sig.Results().At(0).Type().(*types.Pointer)
	if !ok {
		return false
	}
	m, _, _ := types.LookupFieldOrMethod(p, true, rw.Pkg, "as_Throwable")
	_, ok = m.(*types.Func)
	return ok
}

// enumSwitchEnd reports whether s, panic("unreachable"), follows a switch on a value of an integer
// enum (tools/postflip/enums: 0 is null, constant k is ordinal k-1, T_enum_ holds the
// constants) whose cases name every constant and all end in a return or a panic. j2go writes
// that for a Java switch expression over the enum's constants: no case matches only a null,
// and Java's switch on a null enum throws NullPointerException (the enum's ordinal() is read).
func enumSwitchEnd(s *ast.ExprStmt) bool {
	sw, ok := before(s).(*ast.SwitchStmt)
	if !ok || sw.Init != nil || sw.Tag == nil {
		return false
	}
	nt, ok := rw.Info.Types[sw.Tag].Type.(*types.Named)
	if !ok {
		return false
	}
	if b, ok := nt.Underlying().(*types.Basic); !ok || b.Info()&types.IsInteger == 0 {
		return false
	}
	v, ok := rw.Pkg.Scope().Lookup(nt.Obj().Name() + "_enum_").(*types.Var)
	if !ok {
		return false
	}
	arr, ok := v.Type().(*types.Array)
	if !ok {
		return false
	}
	seen := map[int64]bool{}
	for _, c := range sw.Body.List {
		cc := c.(*ast.CaseClause)
		if cc.List == nil || len(cc.Body) == 0 {
			return false // default, or a case that falls out of the switch
		}
		switch last := cc.Body[len(cc.Body)-1].(type) {
		case *ast.ReturnStmt:
		case *ast.ExprStmt:
			if !strings.HasPrefix(rw.Text(last), "panic(") {
				return false
			}
		default:
			return false
		}
		for _, e := range cc.List {
			tv := rw.Info.Types[e]
			if tv.Value == nil || !types.Identical(tv.Type, nt) {
				return false
			}
			k, ok := constantInt(tv)
			if !ok {
				return false
			}
			seen[k] = true
		}
	}
	for k := int64(1); k <= arr.Len(); k++ {
		if !seen[k] {
			return false
		}
	}
	return len(seen) == int(arr.Len())
}

func constantInt(tv types.TypeAndValue) (int64, bool) {
	return constantInt64(tv.Value.ExactString())
}

func constantInt64(s string) (int64, bool) {
	var k int64
	_, err := fmt.Sscan(s, &k)
	return k, err == nil
}

// before is the statement right before s in its list, or nil.
func before(s ast.Stmt) ast.Stmt {
	var list []ast.Stmt
	switch p := rw.Parent[s].(type) {
	case *ast.BlockStmt:
		list = p.List
	case *ast.CaseClause:
		list = p.Body
	}
	for i, x := range list {
		if x == s && i > 0 {
			return list[i-1]
		}
	}
	return nil
}

// unreachableKind says what a panic("unreachable") that is left ends.
func unreachableKind(arg ast.Expr) string {
	s, _ := rw.Parent[rw.Parent[arg]].(*ast.ExprStmt)
	if s == nil {
		return "(other)"
	}
	switch p := before(s).(type) {
	case *ast.IfStmt:
		if strings.HasPrefix(rw.Text(p.Cond), "code_") {
			return "after the code tests of an emulated try (the try block returns from inside a loop or branch)"
		}
	case *ast.SwitchStmt:
		return "after a switch that Go cannot prove exhaustive (not every case returns, or not an enum: a string or int switch, a default-less switch)"
	case *ast.ForStmt, *ast.RangeStmt:
		return "after a loop that only a return leaves"
	}
	return "ending a function whose end Go cannot prove unreachable"
}
