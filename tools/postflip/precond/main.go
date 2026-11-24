// Command precond is a post-flip step: the message arguments of a Guava precondition are boxed
// only when the check fails.
//
// j2go translates checkState(cond, "fmt %s", x) as a call whose message arguments are of type
// any: Preconditions_checkState_boolean_String_Object(cond, "fmt %s", strobj_(x)). Go evaluates
// and boxes them before the call, on every call, and boxing a string or a number allocates. Java
// evaluates the argument expressions before the call too, but autoboxing an int is a cache
// lookup and passing a String boxes nothing.
//
// A check used as a statement becomes an if statement that calls the same function, with a
// failing condition, only when the check fails:
//
//	if !cond {
//		Preconditions_checkState_boolean_String_Object(false, "fmt %s", strobj_(x))
//	}
//
// (checkNotNull(v, ...) as a statement: if v == nil { Preconditions_checkNotNull_...(nil, ...) }.)
// The failing call builds the same message and throws the same exception as before.
//
// This is done when every message argument can be evaluated later than Java evaluates it without
// a difference: a constant, a variable, a field of `this` (which is never null), a numeric
// conversion of one of these, or one of these boxed (obj_, strobj_, any, .self(), .box_()).
// Nothing runs between the condition and the message, so the values read are the ones Java
// passes; and none of them can throw. When some argument is anything else (a call, a field of
// an object that may be null), all non-constant arguments are still evaluated in Java's order,
// into variables of the if statement, and only their boxing moves into the failing branch:
//
//	if ok_, a1_ := cond, x.toString(); !ok_ {
//		Preconditions_checkState_boolean_String_Object(false, "fmt %s", strobj_(a1_))
//	}
//
// Checks whose message arguments are all constants are left alone (boxing a constant does not
// allocate), as are checks used as expressions (checkNotNull's result). A call that already
// passes false (or nil) is the failing branch of an earlier run, so the step can be rerun.
//
// usage: precond [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

// boxers are the functions that box their one argument into an `any`.
var boxers = map[string]bool{"obj_": true, "strobj_": true, "any": true}

// unbox splits a message argument into its boxing (a format with %s for the inner expression)
// and the expression that is boxed. An argument that is not a boxing is its own inner.
func unbox(e ast.Expr) (string, ast.Expr) {
	e = rw.Unparen(e)
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return "%s", e
	}
	switch f := c.Fun.(type) {
	case *ast.Ident:
		if boxers[f.Name] && len(c.Args) == 1 {
			return f.Name + "(%s)", c.Args[0]
		}
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok && id.Name == "obj_" && len(c.Args) == 1 {
			return "obj_[" + rw.Text(f.Index) + "](%s)", c.Args[0]
		}
	case *ast.SelectorExpr:
		if len(c.Args) == 0 && (f.Sel.Name == "self" || f.Sel.Name == "box_") {
			return "%s." + f.Sel.Name + "()", f.X
		}
	}
	return "%s", e
}

func isConst(e ast.Expr) bool {
	tv, ok := rw.Info.Types[e]
	return ok && (tv.Value != nil || tv.IsNil())
}

// late reports whether e can be evaluated later, in the failing branch, without a difference:
// it has no effect, cannot panic, and reads only variables that nothing between Java's
// evaluation of it and the branch can change.
func late(e ast.Expr) bool {
	e = rw.Unparen(e)
	if isConst(e) {
		return true
	}
	switch x := e.(type) {
	case *ast.Ident:
		_, ok := rw.Info.Uses[x].(*types.Var)
		return ok
	case *ast.SelectorExpr:
		sel := rw.Info.Selections[x]
		if sel == nil || sel.Kind() != types.FieldVal {
			return false
		}
		if id, ok := x.X.(*ast.Ident); ok && id.Name == "this" {
			return true
		}
		_, ptr := rw.Info.Types[x.X].Type.Underlying().(*types.Pointer)
		return !ptr && late(x.X)
	case *ast.CallExpr:
		if tv := rw.Info.Types[x.Fun]; tv.IsType() && len(x.Args) == 1 {
			if b, ok := tv.Type.Underlying().(*types.Basic); ok && b.Info()&types.IsNumeric != 0 {
				return late(x.Args[0])
			}
		}
		if box, inner := unbox(x); box != "%s" {
			return late(inner)
		}
	}
	return false
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])

	count := map[string]int{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := c.Fun.(*ast.Ident)
			if !ok || !strings.HasPrefix(id.Name, "Preconditions_check") || len(c.Args) < 2 {
				return true
			}
			if strings.HasSuffix(id.Name, "_T") || strings.HasSuffix(id.Name, "_Str") { // reflip7
				count["generic checkNotNull of the typed translator, left"]++
				return true
			}
			notNull := strings.HasPrefix(id.Name, "Preconditions_checkNotNull")
			if strings.HasSuffix(id.Name, "Arr") {
				count["array of arguments, left"]++
				return true
			}
			if isConst(c.Args[0]) {
				return true // the failing branch of an earlier run
			}
			msg := c.Args[1:]
			allConst := true
			for _, a := range msg {
				if _, inner := unbox(a); !isConst(inner) {
					allConst = false
				}
			}
			stmt, isStmt := rw.Up(c).(*ast.ExprStmt)
			switch {
			case allConst:
				count["constant message, left"]++
				return true
			case !isStmt:
				count["used as an expression, left"]++
				return true
			}
			allLate := true
			for _, a := range msg {
				if !late(a) {
					allLate = false
				}
			}
			fail := "false"
			if notNull {
				fail = "nil"
			}
			if allLate {
				count["message evaluated in the failing branch"]++
				rw.Rewrite[stmt] = func() string {
					args := []string{fail}
					for _, a := range msg {
						args = append(args, rw.Render(a))
					}
					return "if " + failing(c.Args[0], notNull) + " {\n" + id.Name + "(" + strings.Join(args, ", ") + ")\n}"
				}
				return true
			}
			count["message evaluated before the check, boxed in the failing branch"]++
			rw.Rewrite[stmt] = func() string {
				v0 := "ok_"
				if notNull {
					v0 = "v_"
				}
				vars, vals, args := []string{v0}, []string{rw.Render(c.Args[0])}, []string{fail}
				for _, a := range msg {
					box, inner := unbox(a)
					if isConst(inner) {
						args = append(args, rw.Render(a))
						continue
					}
					v := fmt.Sprintf("a%d_", len(vars))
					vars, vals = append(vars, v), append(vals, rw.Render(inner))
					args = append(args, strings.Replace(box, "%s", v, 1))
				}
				test := "!ok_"
				if notNull {
					test = "v_ == nil"
				}
				return "if " + strings.Join(vars, ", ") + " := " + strings.Join(vals, ", ") + "; " + test + " {\n" + id.Name + "(" + strings.Join(args, ", ") + ")\n}"
			}
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
		fmt.Printf("precond: %d files changed\n", rw.Save())
	}
}

// failing is the condition under which a check fails: !cond for checkState and checkArgument,
// v == nil for checkNotNull.
func failing(e ast.Expr, notNull bool) string {
	if notNull {
		return rw.Render(e) + " == nil"
	}
	switch x := rw.Unparen(e).(type) {
	case *ast.UnaryExpr:
		if x.Op == token.NOT {
			return rw.Render(x.X)
		}
	case *ast.Ident, *ast.CallExpr, *ast.SelectorExpr:
		return "!" + rw.Render(e)
	}
	if _, ok := e.(*ast.ParenExpr); ok {
		return "!" + rw.Render(e)
	}
	return "!(" + rw.Render(e) + ")"
}
