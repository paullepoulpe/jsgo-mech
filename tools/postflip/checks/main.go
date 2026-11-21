// Command checks is a post-flip step: Guava preconditions become inline Go checks.
//
// j2go translates Preconditions.checkState(c, ...) as a call of a runtime function,
// Preconditions_checkState_boolean_String_Object(c, "fmt %s", x), which tests c and throws
// IllegalStateException with the formatted message. Go code checks inline:
//
//	if !c {
//		panic(new_IllegalStateException_String(Strings_lenientFormat("fmt %s", []any{x})).self())
//	}
//
// which is what j2go writes for `if (!c) throw new IllegalStateException(...)`. The exception
// is the one Guava throws: checkState IllegalStateException, checkArgument
// IllegalArgumentException, checkNotNull NullPointerException; without a message it is made
// with no message (new_X_0), with an Object message with String.valueOf of it (objstr_, or the
// string itself for a String), with a template and arguments with Strings.lenientFormat, as in
// Guava and as the runtime functions do.
//
// Order of evaluation. Java evaluates the condition and every message argument before the
// call. A check used as a statement keeps that order:
//   - when every message argument can be evaluated later than Java evaluates it without a
//     difference (a constant, a variable, a field of `this`, a numeric conversion or a boxing
//     of one of these: nothing runs between the condition and them, and they cannot throw),
//     they are evaluated in the failing branch;
//   - otherwise the condition and the non-constant arguments are evaluated in Java's order into
//     variables of the if statement (`if ok_, a1_ := c, x.f(); !ok_ {`), as tools/postflip/precond
//     already does; its output (a check called with false or nil in such a branch) is
//     recognised and its call becomes the panic.
//
// checkNotNull(v) tests v as Java null: nil for a class pointer (x.self() is nil exactly when x
// is) or an interface, 0 for a node handle (box_ is nil exactly then), nullstr_ for a String,
// the zero value for a value boxed by obj_ (obj_ makes exactly that nil).
//
// checkNotNull used as an expression (its result is the checked value) is rewritten when the
// statement holding it evaluates it first (it is the value of an assignment, declaration or
// return, or the argument of a call or an unboxing whose earlier operands do nothing):
//   - when the checked value is a variable or a field path (rw.Pure), the check is put before
//     the statement, and the statement uses the value itself (with the boxing and unboxing
//     around it dropped when they cancel);
//   - otherwise, when the statement declares or assigns a local variable that no closure
//     captures, the statement keeps the value (unboxed) and the check of the variable follows
//     it: if the check fails, the variable is never read again.
//
// The rest are left and counted, by reason.
//
// usage: checks [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

var count = map[string]int{}

// leave counts a check left as it is; with -v, it prints where.
func leave(reason string, n ast.Node) {
	count[reason]++
	if verbose {
		fmt.Fprintf(os.Stderr, "%s: %s\n", rw.Fset.Position(n.Pos()), reason)
	}
}

var verbose = os.Getenv("CHECKS_VERBOSE") != ""

// boxers are the functions that box their one argument into an `any`.
var boxers = map[string]bool{"obj_": true, "strobj_": true, "any": true}

// unbox splits an argument into its boxing (a format with %s for the inner expression) and the
// expression that is boxed. An argument that is not a boxing is its own inner.
func unbox(e ast.Expr) (string, ast.Expr) {
	e = rw.Unparen(e)
	if b, ok := reflipBox_[e]; ok { // reflip21: an argument a typed variant boxes itself
		return b, e
	}
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
// evaluation of it and the branch can change. (As in tools/postflip/precond.)
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
		if box, inner := unbox(x); box != "%s" && inner != ast.Expr(x) {
			return late(inner)
		}
	}
	return false
}

// primary renders e so that it can be the operand of a binary operator.
func primary(e ast.Expr) string {
	switch rw.Unparen(e).(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr, *ast.BasicLit:
		return rw.Render(rw.Unparen(e))
	}
	return "(" + rw.Render(e) + ")"
}

// nullTest is the Go test that a value of type t, rendered v, is Java null.
func nullTest(t types.Type, v string) (string, bool) {
	switch u := t.Underlying().(type) {
	case *types.Pointer, *types.Interface, *types.Map, *types.Chan, *types.Signature:
		return v + " == nil", true
	case *types.Basic:
		switch {
		case u.Kind() == types.String:
			return v + " == nullstr_", true
		case rw.HasMethod(t, "box_") && u.Info()&types.IsInteger != 0:
			return v + " == 0", true
		case reflipTypedNull_ && rw.HasMethod(t, "enum_") && u.Info()&types.IsInteger != 0: // an enum constant: null is 0
			return v + " == 0", true
		}
	}
	return "", false
}

// boxedNullTest is the test that boxed value e is nil, on the value inside the boxing; ok is
// false when the boxing cannot make nil from a value of that type (any of a pointer, a
// number), or the representation of null is unknown.
func boxedNullTest(e ast.Expr) (test string, inner ast.Expr, ok bool) {
	box, in := unbox(e)
	t := rw.Info.Types[in].Type
	if t == nil {
		return "", nil, false
	}
	u := t.Underlying()
	switch {
	case box == "%s" && reflipChecked_[in]: // reflip21: the value of a typed checkNotNull, tested at its type
		test, ok := nullTest(t, primary(in))
		return test, in, ok
	case box == "%s" || box == "any(%s)":
		if _, isIface := u.(*types.Interface); !isIface {
			return "", nil, false
		}
		return primary(in) + " == nil", in, true
	case box == "strobj_(%s)":
		return primary(in) + " == nullstr_", in, true
	case box == "%s.self()":
		if _, isPtr := u.(*types.Pointer); !isPtr {
			return "", nil, false
		}
		return primary(in) + " == nil", in, true
	case box == "%s.box_()":
		test, ok := nullTest(t, primary(in))
		return test, in, ok
	case strings.HasPrefix(box, "obj_"):
		switch b := u.(type) {
		case *types.Pointer, *types.Interface, *types.Map, *types.Chan, *types.Signature:
			return primary(in) + " == nil", in, true
		case *types.Basic:
			if b.Info()&types.IsNumeric != 0 {
				return primary(in) + " == 0", in, true
			}
			if b.Kind() == types.String {
				return primary(in) + ` == ""`, in, true
			}
		}
	}
	return "", nil, false
}

// check is one call of a Guava precondition.
type check struct {
	call *ast.CallExpr
	kind string   // State, Argument, NotNull
	sig  []string // parameter types after the first: Object, String, int, ObjectArr
}

func parse(c *ast.CallExpr) (check, bool) {
	id, ok := c.Fun.(*ast.Ident)
	if !ok || !strings.HasPrefix(id.Name, "Preconditions_check") {
		return check{}, false
	}
	if _, ok := rw.Info.Uses[id].(*types.Func); !ok {
		return check{}, false
	}
	parts := strings.Split(strings.TrimPrefix(id.Name, "Preconditions_check"), "_")
	variant := ""
	if n := len(parts); n > 1 && (parts[n-1] == "H" || parts[n-1] == "T" || parts[n-1] == "Str") {
		variant, parts = parts[n-1], parts[:n-1]
	}
	k := check{call: c, kind: parts[0]}
	switch k.kind {
	case "State", "Argument", "NotNull":
	default:
		return check{}, false
	}
	if len(parts) > 1 {
		k.sig = parts[2:]
	}
	if len(k.sig) != len(c.Args)-1 {
		return check{}, false
	}
	if variant != "" && !reflipVariant_(k, variant) {
		return check{}, false
	}
	return k, true
}

var exceptions = map[string]string{"State": "IllegalStateException", "Argument": "IllegalArgumentException", "NotNull": "NullPointerException"}

// throw renders the panic of a failing check with message arguments args. inner(i, x) renders
// the value inside the boxing of argument i (x itself, or the variable holding it).
func (k check) throw(args []ast.Expr, inner func(int, ast.Expr) string) string {
	exc := exceptions[k.kind]
	if len(k.sig) == 0 {
		return "panic(new_" + exc + "_0().self())"
	}
	boxed := func(i int) string {
		box, in := unbox(args[i])
		if box == "%s" {
			return inner(i, args[i])
		}
		return strings.Replace(box, "%s", inner(i, in), 1)
	}
	var msg string
	switch {
	case len(k.sig) == 1 && k.sig[0] == "Object":
		// String.valueOf(message); of a String, the string ("null" for null).
		box, in := unbox(args[0])
		switch {
		case box == "strobj_(%s)" && isConst(in) && constant.StringVal(rw.Info.Types[in].Value) != nullstr:
			msg = rw.Render(in)
		case box == "strobj_(%s)":
			msg = "sstr_(" + inner(0, in) + ")"
		default:
			msg = "objstr_(" + boxed(0) + ")"
		}
	case k.sig[0] == "String" && len(k.sig) == 2 && k.sig[1] == "ObjectArr":
		msg = "Strings_lenientFormat(" + boxed(0) + ", " + boxed(1) + ")"
	case k.sig[0] == "String":
		var ps []string
		for i := 1; i < len(args); i++ {
			ps = append(ps, boxed(i))
		}
		msg = "Strings_lenientFormat(" + boxed(0) + ", []any{" + strings.Join(ps, ", ") + "})"
	default:
		panic("unknown precondition signature " + strings.Join(k.sig, "_"))
	}
	return "panic(new_" + exc + "_String(" + msg + ").self())"
}

// nullstr is the runtime's String null (rt_core.go nullstr_).
const nullstr = "\x00\xff<null>\xff\x00"

func plain(_ int, a ast.Expr) string { return rw.Render(a) }

// failing is the Go condition under which check state/argument with condition e fails: !e,
// with a negated comparison flipped and De Morgan's law applied to && and || of comparisons
// and simple operands (the operands are evaluated in the same order, and as far).
func failing(e ast.Expr) string {
	if n, ok := negate(e, true); ok {
		return n
	}
	return "!(" + rw.Render(rw.Unparen(e)) + ")"
}

var flipped = map[token.Token]token.Token{token.EQL: token.NEQ, token.NEQ: token.EQL, token.LSS: token.GEQ, token.GEQ: token.LSS, token.GTR: token.LEQ, token.LEQ: token.GTR}

// negate renders !e without a leading !( when e is a negation, a comparison, a simple operand
// or (with deep) && or || of these.
func negate(e ast.Expr, deep bool) (string, bool) {
	switch x := rw.Unparen(e).(type) {
	case *ast.UnaryExpr:
		if x.Op == token.NOT {
			return rw.Render(x.X), true
		}
	case *ast.BinaryExpr:
		if op, ok := flipped[x.Op]; ok {
			if b, ok := rw.Info.Types[x.X].Type.Underlying().(*types.Basic); ok && b.Info()&types.IsFloat != 0 && x.Op != token.EQL && x.Op != token.NEQ {
				return "", false // !(a < b) is not a >= b for NaN
			}
			return rw.Render(x.X) + " " + op.String() + " " + rw.Render(x.Y), true
		}
		if (x.Op == token.LAND || x.Op == token.LOR) && deep {
			l, ok1 := negate(x.X, false)
			r, ok2 := negate(x.Y, false)
			if !ok1 || !ok2 || !simpleNeg(x.X) || !simpleNeg(x.Y) {
				return "", false
			}
			op := " || "
			if x.Op == token.LOR {
				op = " && "
			}
			return l + op + r, true
		}
	case *ast.Ident, *ast.CallExpr, *ast.SelectorExpr, *ast.IndexExpr:
		return "!" + rw.Render(x), true
	}
	return "", false
}

// simpleNeg reports whether the negation of e is a comparison or a (negated) primary, which
// needs no parentheses as an operand of && or ||.
func simpleNeg(e ast.Expr) bool {
	switch x := rw.Unparen(e).(type) {
	case *ast.UnaryExpr:
		if x.Op == token.NOT {
			switch rw.Unparen(x.X).(type) {
			case *ast.Ident, *ast.CallExpr, *ast.SelectorExpr, *ast.IndexExpr:
				return true
			}
		}
	case *ast.BinaryExpr:
		_, ok := flipped[x.Op]
		return ok
	case *ast.Ident, *ast.CallExpr, *ast.SelectorExpr, *ast.IndexExpr:
		return true
	}
	return false
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])

	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			k, ok := parse(c)
			if !ok {
				return true
			}
			for p := rw.Parent[c]; p != nil; p = rw.Parent[p] {
				if pc, ok := p.(*ast.CallExpr); ok {
					if _, ok := parse(pc); ok {
						leave("left: inside another check", c)
						return true
					}
				}
			}
			if _, isStmt := rw.Up(c).(*ast.ExprStmt); isStmt {
				statement(k)
			} else {
				expression(k)
			}
			return true
		})
	}
	var keys []string
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	total := 0
	for _, k := range keys {
		fmt.Printf("%5d %s\n", count[k], k)
		if !strings.HasPrefix(k, "left") {
			total += count[k]
		}
	}
	fmt.Printf("%5d checks inline\n", total)
	if !report {
		fmt.Printf("checks: %d files changed\n", rw.SaveChecked())
	}
}

// statement rewrites a check used as a statement.
func statement(k check) {
	c := k.call
	stmt := rw.Up(c).(*ast.ExprStmt)
	msg := c.Args[1:]
	if isConst(c.Args[0]) {
		tv := rw.Info.Types[c.Args[0]]
		if !(tv.IsNil() || tv.Value.ExactString() == "false") {
			leave("left: condition is constant true", c)
			return
		}
		// The failing branch of tools/postflip/precond, or Java's own checkState(false, ...).
		if ifs, ok := rw.Parent[stmt].(*ast.BlockStmt); ok && len(ifs.List) == 1 {
			if is, ok := rw.Parent[ifs].(*ast.IfStmt); ok && is.Body == ifs && is.Else == nil {
				failingBranch(k, is)
				return
			}
		}
		count["unconditional throw"]++
		rw.Rewrite[stmt] = func() string { return k.throw(msg, plain) }
		return
	}
	// test renders the failing condition (when the rewrite is rendered, after all are known).
	test := func() string { return failing(c.Args[0]) }
	var testType types.Type
	if k.kind == "NotNull" {
		_, inner, ok := boxedNullTest(c.Args[0])
		if !ok {
			leave("left: checkNotNull of a value that is never nil in Go (any of a slice)", c)
			return
		}
		testType = rw.Info.Types[inner].Type
		test = func() string { t, _, _ := boxedNullTest(c.Args[0]); return t }
	}
	allLate := true
	for _, a := range msg {
		if !late(a) {
			allLate = false
		}
	}
	if allLate {
		count["statement, message evaluated in the failing branch"]++
		rw.Rewrite[stmt] = func() string {
			return "if " + test() + " {\n" + k.throw(msg, plain) + "\n}"
		}
		return
	}
	// Some message argument must be evaluated before the check, as in Java.
	count["statement, message evaluated before the check"]++
	rw.Rewrite[stmt] = func() string {
		v0, val := "ok_", rw.Render(c.Args[0])
		t := "!ok_"
		if k.kind == "NotNull" {
			_, inner := unbox(c.Args[0])
			v0, val = "v_", rw.Render(inner)
			t, _ = nullTest(testType, "v_")
		}
		vars, vals := []string{v0}, []string{val}
		names := map[int]string{}
		for i, a := range msg {
			_, inner := unbox(a)
			if isConst(inner) {
				continue
			}
			v := fmt.Sprintf("a%d_", len(vars))
			names[i] = v
			vars, vals = append(vars, v), append(vals, rw.Render(inner))
		}
		render := func(i int, a ast.Expr) string {
			if v, ok := names[i]; ok {
				return v
			}
			return rw.Render(a)
		}
		return "if " + strings.Join(vars, ", ") + " := " + strings.Join(vals, ", ") + "; " + t + " {\n" + k.throw(msg, render) + "\n}"
	}
}

// failingBranch rewrites `if <test> { Preconditions_X(false, args...) }`, the output of
// tools/postflip/precond: the call becomes the panic, and a checkNotNull's boxed value in the
// test (or in the if statement's first variable) is tested unboxed.
func failingBranch(k check, is *ast.IfStmt) {
	c := k.call
	stmt := is.Body.List[0]
	msg := c.Args[1:]
	count["failing branch of tools/postflip/precond"]++
	rw.Rewrite[stmt] = func() string { return k.throw(msg, plain) }
	if k.kind != "NotNull" {
		// precond wrote !(c) for checkState(c): flip it as failing does.
		if u, ok := is.Cond.(*ast.UnaryExpr); ok && u.Op == token.NOT && is.Init == nil {
			if _, paren := u.X.(*ast.ParenExpr); paren {
				if _, ok := negate(u.X, true); ok {
					rw.Rewrite[is.Cond] = func() string { n, _ := negate(u.X, true); return n }
				}
			}
		}
		return
	}
	cond, ok := rw.Unparen(is.Cond).(*ast.BinaryExpr)
	if !ok || cond.Op != token.EQL || !isConst(cond.Y) || !rw.Info.Types[cond.Y].IsNil() {
		return
	}
	if as, ok := is.Init.(*ast.AssignStmt); ok {
		// if v_, a1_ := boxed, ...; v_ == nil
		id, ok := rw.Unparen(cond.X).(*ast.Ident)
		if !ok || len(as.Lhs) == 0 || len(as.Lhs) != len(as.Rhs) {
			return
		}
		if l, ok := as.Lhs[0].(*ast.Ident); !ok || l.Name != id.Name {
			return
		}
		_, inner, ok := boxedNullTest(as.Rhs[0])
		if !ok || inner == rw.Unparen(as.Rhs[0]) {
			return
		}
		test, ok := nullTest(rw.Info.Types[inner].Type, id.Name)
		if !ok {
			return
		}
		rw.Rewrite[as.Rhs[0]] = func() string { return rw.Render(inner) }
		rw.Rewrite[cond] = func() string { return test }
		return
	}
	if is.Init != nil {
		return
	}
	if _, inner, ok := boxedNullTest(cond.X); ok && inner != rw.Unparen(cond.X) {
		rw.Rewrite[cond] = func() string { t, _, _ := boxedNullTest(cond.X); return t }
	}
}

// unboxers are the calls that take a value out of an `any`.
func isUnboxing(p *ast.CallExpr) bool {
	if len(p.Args) != 1 {
		return false
	}
	name, _ := rw.CallName(p)
	switch {
	case name == "str_" || name == "Node_of_" || strings.HasPrefix(name, "cast_[") || strings.HasPrefix(name, "unbox_["):
		return true
	case strings.HasPrefix(name, "to_") || strings.HasPrefix(name, "icast_"):
		return true
	}
	return false
}

// firstIn reports whether e is evaluated before anything with an effect in its statement, and
// returns that statement: e is the value of an assignment to a variable or a field of this, of
// a declaration, of a return, the condition of an if (without an initialization), or, within
// these, an operand of a call (after operands that do nothing), the receiver of a selector, or
// the operand of a unary operator or the left operand of a binary one.
func firstIn(e ast.Expr) (ast.Stmt, bool) {
	for {
		switch p := rw.Up(e).(type) {
		case *ast.CallExpr:
			if rw.Unparen(p.Fun) == rw.Unparen(e) {
				return nil, false
			}
			idx := -1
			for i, a := range p.Args {
				if rw.Unparen(a) == rw.Unparen(e) {
					idx = i
				}
			}
			if idx < 0 {
				return nil, false
			}
			for _, a := range p.Args[:idx] {
				if !late(a) && !hoisted[rw.Unparen(a)] {
					return nil, false
				}
			}
			switch f := p.Fun.(type) {
			case *ast.Ident, *ast.IndexExpr:
			case *ast.SelectorExpr:
				if _, isMethod := rw.Info.Selections[f]; isMethod && !late(f.X) {
					return nil, false
				}
			default:
				return nil, false
			}
			e = p
		case *ast.SelectorExpr:
			// The receiver of a method call or a field: evaluated first.
			if call, ok := rw.Up(p).(*ast.CallExpr); ok && rw.Unparen(call.Fun) == p {
				e = call
				continue
			}
			e = p
		case *ast.UnaryExpr:
			e = p
		case *ast.BinaryExpr:
			if rw.Unparen(p.X) != rw.Unparen(e) {
				return nil, false
			}
			e = p
		case *ast.ExprStmt:
			return p, true
		case *ast.IfStmt:
			if p.Init != nil || rw.Unparen(p.Cond) != rw.Unparen(e) {
				return nil, false
			}
			return p, true
		case *ast.AssignStmt:
			if len(p.Lhs) != 1 || len(p.Rhs) != 1 || rw.Unparen(p.Rhs[0]) != rw.Unparen(e) || (p.Tok != token.ASSIGN && p.Tok != token.DEFINE) {
				return nil, false
			}
			if !pureTarget(p.Lhs[0]) {
				return nil, false
			}
			return p, true
		case *ast.ValueSpec:
			if len(p.Names) != 1 || len(p.Values) != 1 {
				return nil, false
			}
			ds, ok := rw.Parent[rw.Parent[p]].(*ast.DeclStmt)
			if !ok {
				return nil, false
			}
			return ds, true
		case *ast.ReturnStmt:
			if len(p.Results) != 1 {
				return nil, false
			}
			return p, true
		default:
			return nil, false
		}
	}
}

// pureTarget reports whether assigning to l evaluates nothing that can have an effect or throw:
// a variable or a field of this.
func pureTarget(l ast.Expr) bool {
	switch x := rw.Unparen(l).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		id, ok := x.X.(*ast.Ident)
		return ok && id.Name == "this" && rw.Info.Selections[x] != nil && rw.Info.Selections[x].Kind() == types.FieldVal
	}
	return false
}

// inList reports whether s is in a statement list, where statements can be added around it.
func inList(s ast.Stmt) bool {
	switch rw.Parent[s].(type) {
	case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
		return true
	}
	return false
}

// mentions reports whether any of es uses an object named name.
func mentions(es []ast.Expr, name string) bool {
	found := false
	for _, e := range es {
		ast.Inspect(e, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == name {
				found = true
			}
			return !found
		})
	}
	return found
}

// localUncaptured reports whether id names a local variable of the innermost function around
// id that no function literal inside it uses.
func localUncaptured(id *ast.Ident) bool {
	v, ok := rw.Info.Uses[id].(*types.Var)
	if !ok {
		v, ok = rw.Info.Defs[id].(*types.Var)
	}
	if !ok || v.IsField() || v.Parent() == rw.Pkg.Scope() || v.Parent() == nil {
		return false
	}
	fn := enclosingFunc(id)
	if fn == nil || !(fn.Pos() <= v.Pos() && v.Pos() < fn.End()) {
		return false
	}
	for _, u := range rw.UsesOf[v] {
		if enclosingFunc(u) != fn {
			return false
		}
	}
	return true
}

func enclosingFunc(n ast.Node) ast.Node {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch p.(type) {
		case *ast.FuncLit, *ast.FuncDecl:
			return p
		}
	}
	return nil
}

// expression rewrites a checkNotNull whose result is used.
func expression(k check) {
	c := k.call
	if k.kind != "NotNull" {
		leave("left: checkState/checkArgument used as an expression", c)
		return
	}
	msg := c.Args[1:]
	allLate := true
	for _, a := range msg {
		if !late(a) {
			allLate = false
		}
	}
	box, inner := unbox(c.Args[0])
	if _, _, ok := boxedNullTest(c.Args[0]); !ok {
		leave("left: checkNotNull of a value that is never nil in Go (any of a slice)", c)
		return
	}
	// top is the checkNotNull or the unboxing of its result.
	var top ast.Expr = c
	if p, ok := rw.Up(c).(*ast.CallExpr); ok && isUnboxing(p) {
		top = p
	}
	stmt, ok := firstIn(top)
	if !ok || !inList(stmt) {
		leave("left: expression, not the first thing its statement evaluates", c)
		return
	}
	// value is what top becomes without the check, when the checked value is v: v when the
	// unboxing gives back the boxed value unchanged, else the unboxing of the boxed value.
	value := func(v string) string {
		boxed := strings.Replace(box, "%s", v, 1)
		if top == ast.Expr(c) {
			return boxed
		}
		if tt, it := rw.Info.Types[top].Type, rw.Info.Types[inner].Type; tt != nil && it != nil && types.Identical(tt, it) && box != "%s" {
			return v
		}
		return rw.Render(top.(*ast.CallExpr).Fun) + "(" + boxed + ")"
	}
	if rw.Pure(inner) && allLate {
		count["expression, value checked before its statement"]++
		hoisted[top] = true
		rw.Rewrite[top] = func() string { return value(rw.Render(inner)) }
		before(stmt, func() string {
			test, _, _ := boxedNullTest(c.Args[0])
			return "if " + test + " {\n" + k.throw(msg, plain) + "\n}"
		})
		return
	}
	if post[stmt] {
		leave("left: expression, second check in a statement checked after it", c)
		return
	}
	// The statement stores the value in a local variable: check the variable after it.
	var target *ast.Ident
	switch s := stmt.(type) {
	case *ast.AssignStmt:
		target, _ = rw.Unparen(s.Lhs[0]).(*ast.Ident)
	case *ast.DeclStmt:
		target = s.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Names[0]
	}
	if target != nil && len(pre[stmt]) == 0 && rw.Unparen(stmtValue(stmt)) == top && allLate && target.Name != "_" && localUncaptured(target) && !mentions(msg, target.Name) {
		o := rw.Info.Defs[target]
		if o == nil {
			o = rw.Info.Uses[target]
		}
		if test, ok := nullTest(o.Type(), target.Name); ok {
			count["expression, variable checked after its statement"]++
			post[stmt] = true
			rw.Rewrite[top] = func() string { return value(rw.Render(inner)) }
			rw.Rewrite[stmt] = func() string {
				return rw.RenderChildren(stmt) + "\nif " + test + " {\n" + k.throw(msg, plain) + "\n}"
			}
			return
		}
	}
	// Otherwise the checked value (and the message arguments that Java evaluates before the
	// check) go into variables declared before the statement, in Java's order.
	t := rw.Info.Types[inner].Type
	v := temp(stmt, msg)
	test, ok := nullTest(t, v)
	if !ok {
		leave("left: expression, value of a type without a null test", c)
		return
	}
	count["expression, value checked in a variable before its statement"]++
	rw.Rewrite[top] = func() string { return value(v) }
	hoisted[top] = true
	before(stmt, func() string {
		vars, vals := []string{v}, []string{rw.Render(inner)}
		names := map[int]string{}
		if !allLate {
			for i, a := range msg {
				_, in := unbox(a)
				if isConst(in) {
					continue
				}
				n := fmt.Sprintf("%s%d", strings.TrimSuffix(v, "_"), len(vars)) + "_"
				names[i] = n
				vars, vals = append(vars, n), append(vals, rw.Render(in))
			}
		}
		render := func(i int, a ast.Expr) string {
			if n, ok := names[i]; ok {
				return n
			}
			return rw.Render(a)
		}
		return strings.Join(vars, ", ") + " := " + strings.Join(vals, ", ") + "\nif " + test + " {\n" + k.throw(msg, render) + "\n}"
	})
}

// hoisted are the checks (or their unboxings) whose test is put before their statement.
var hoisted = map[ast.Expr]bool{}

// post are the statements followed by the check of the variable they set.
var post = map[ast.Stmt]bool{}

// pre are the statements put before a statement.
var pre = map[ast.Stmt][]func() string{}

// before puts the statement gen() before stmt.
func before(stmt ast.Stmt, gen func() string) {
	if len(pre[stmt]) == 0 {
		rw.Rewrite[stmt] = func() string {
			var out []string
			for _, g := range pre[stmt] {
				out = append(out, g())
			}
			return strings.Join(out, "\n") + "\n" + rw.RenderChildren(stmt)
		}
	}
	pre[stmt] = append(pre[stmt], gen)
}

// stmtValue is the value an assignment or declaration of one variable stores.
func stmtValue(s ast.Stmt) ast.Expr {
	switch s := s.(type) {
	case *ast.AssignStmt:
		return s.Rhs[0]
	case *ast.DeclStmt:
		return s.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec).Values[0]
	}
	return nil
}

// temps counts the variables declared in each statement list.
var temps = map[ast.Node]int{}

// temp names a new variable declared before stmt: nn_, nn1_, ... unique in its statement list
// and among the names the message arguments use.
func temp(stmt ast.Stmt, msg []ast.Expr) string {
	list := rw.Parent[stmt]
	for {
		n := temps[list]
		temps[list]++
		v := "nn_"
		if n > 0 {
			v = fmt.Sprintf("nn%d_", n)
		}
		if !mentions(msg, v) && !declaredIn(list, v) {
			return v
		}
	}
}

// declaredIn reports whether a statement of the list declares name.
func declaredIn(list ast.Node, name string) bool {
	found := false
	ast.Inspect(list, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name && rw.Info.Defs[id] != nil {
			found = true
		}
		return !found
	})
	return found
}

// reflipBox_ gives the boxing (a format, as unbox returns) of a message argument that a typed
// variant of a check boxes itself; reflipChecked_ are the values typed checkNotNulls check.
var reflipBox_ = map[ast.Expr]string{}
var reflipChecked_ = map[ast.Expr]bool{}
var reflipSeen_ = map[*ast.CallExpr]bool{}

// reflipTypedNull_: the null test of an enum value (0) is known; set once a typed variant is seen
// (the erased forms never ask it: an enum constant is boxed by self()).
var reflipTypedNull_ = true

// reflipVariant_ reads the typed variant (_H, _T, _Str) of check k as the erased check of the
// same name: it records how the runtime function boxes the arguments it takes typed. It is
// false (the call is left, counted) for a form the tool has no exact rewrite of.
func reflipVariant_(k check, variant string) bool {
	c := k.call
	if ok, seen := reflipSeen_[c]; seen {
		return ok
	}
	ok := reflipVariant1_(k, variant)
	reflipSeen_[c] = ok
	if !ok {
		leave("left: typed variant _"+variant+" in a form without a rewrite", c)
	}
	return ok
}

func reflipPrimary_(e ast.Expr) bool {
	switch e.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr:
		return true
	}
	return false
}

func reflipVariant1_(k check, variant string) bool {
	c := k.call
	switch variant {
	case "H": // the last argument is a handle, boxed by box_() where the check fails
		if k.kind == "NotNull" || len(c.Args) < 2 {
			return false
		}
		a := rw.Unparen(c.Args[len(c.Args)-1])
		t := rw.Info.Types[a].Type
		if t == nil || !rw.HasMethod(t, "box_") || !reflipPrimary_(a) {
			return false
		}
		reflipBox_[a] = "%s.box_()"
		return true
	case "T", "Str": // the checked value at its own type; message arguments as they are
		if k.kind != "NotNull" {
			return false
		}
		v := rw.Unparen(c.Args[0])
		t := rw.Info.Types[v].Type
		if t == nil {
			return false
		}
		b, _ := t.Underlying().(*types.Basic)
		if (b != nil && b.Kind() == types.String) != (variant == "Str") {
			return false
		}
		if _, ok := nullTest(t, "v"); !ok {
			return false
		}
		reflipChecked_[v] = true
		if len(k.sig) == 1 { // one Object message: String.valueOf(canon_(msg))
			a := rw.Unparen(c.Args[1])
			at := rw.Info.Types[a].Type
			if box, _ := unbox(a); box == "%s" && at != nil && !rw.Info.Types[a].IsNil() {
				ab, _ := at.Underlying().(*types.Basic)
				switch {
				case ab != nil && ab.Kind() == types.String:
					reflipBox_[a] = "strobj_(%s)"
				default:
					reflipBox_[a] = "canon_(%s)"
				}
			}
		}
		return true
	}
	return false
}
