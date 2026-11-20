// Command eqhash is a post-flip step: equals and hashCode calls whose static types decide what
// they do are written as what they do, without the runtime's dynamic protocol.
//
// j2go translates Objects.equals(a, b), a.equals(b) on an Object and Objects.hashCode(a) as
// calls of runtime functions taking `any`: Objects_equals(obj_(a), obj_(b)) boxes both values,
// and Object_equals then asks the dynamic type of a for an equals method (an interface
// assertion), else compares the two values as Go values (identity for a pointer). Where the
// static type of a boxed value fixes its dynamic type, this step decides the call at compile
// time:
//
//   - the value is boxed by obj_(x) or any(x) (the dynamic type is the static type of x: a
//     pointer to a class, a node handle, a string or a primitive), by strobj_(s) (a string),
//     by x.box_() (a handle), or by x.self() of a class that no class extends;
//   - identity: when both values have one such type T and T has no equals method (a class that
//     does not override equals, an enum, Node, ...), or T is a string or an integer or boolean
//     type, Java compares them by identity or value, which is Go's == on the unboxed values:
//     Objects_equals(obj_(a), obj_(b)) -> a == b (null equals only null: a nil pointer, a zero
//     handle, nullstr_). Object_equals(obj_(a), obj_(b)) (a.equals(b)) throws a
//     NullPointerException when a is null: equalsId_(a, b) (strequals_ for strings), or a == b
//     when a is `this` or a constant;
//   - a class with its own equals: Object_equals(obj_(a), B) calls (*C).equals directly:
//     a.equals(B) when a is `this`, else equalsBy_(a, B, (*C).equals), which throws on null
//     as before; Objects_equals(obj_(a), B) -> objequalsBy_(a, B, (*C).equals), which keeps
//     Objects.equals' null cases;
//   - hashCode: Objects_hashCode(A) is 0 for null, else the class's hashCode method, called
//     directly (objhashBy_(a, (*C).hashCode), a.hashCode() for `this`), or the identity hash
//     (objhashId_(a)) when the class has none; String.hashCode of a string (objhashStr_(s)); an
//     int is its own hash, a boolean 1231 or 1237;
//   - x.self() of a class with subclasses boxes the most derived object: when neither the
//     class nor a subclass has an equals (hashCode) method, the call is identity (the identity
//     hash of that object), and two pointers to the class's part of objects are equal when the
//     objects are.
//
// The runtime helpers (rt_eqhash.go) are generic and small, so the compiler inlines them and the
// method expression becomes a direct call. Values of interface type, of float types (NaN equals
// NaN in Double.equals), and pairs of different types are left to the runtime.
//
// usage: eqhash [-n] <dir of package closure>
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

// extended are the named struct types that another struct embeds (a class with subclasses).
var extended = map[*types.Named]bool{}

// subs are the structs that embed a struct (its direct subclasses).
var subs = map[*types.Named][]*types.Named{}

// overridden reports whether a class or one of its subclasses has the method.
func overridden(nt *types.Named, name, sig string) bool {
	if method(types.NewPointer(nt), name, sig) != nil {
		return true
	}
	for _, s := range subs[nt] {
		if overridden(s, name, sig) {
			return true
		}
	}
	return false
}

func main() {
	rw.Load(os.Args[len(os.Args)-1])
	sc := rw.Pkg.Scope()
	for _, n := range sc.Names() {
		tn, ok := sc.Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			if !f.Embedded() {
				continue
			}
			t := f.Type()
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			if nt, ok := t.(*types.Named); ok {
				extended[nt] = true
				subs[nt] = append(subs[nt], tn.Type().(*types.Named))
			}
		}
	}
	run(os.Args[1] == "-n")
}

// exact returns the value that boxed expression e boxes, when its dynamic type is its static
// type, and that type.
func exact(e ast.Expr) (ast.Expr, types.Type) {
	x, t, self := exactOrSelf(e)
	if self {
		return nil, nil
	}
	return x, t
}

// exactOrSelf is exact, and also returns x for x.self() of a class with subclasses (self true):
// the boxed value is the object of the most derived class.
func exactOrSelf(e ast.Expr) (ast.Expr, types.Type, bool) {
	self := false
	e = rw.Unparen(e)
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return nil, nil, false
	}
	var x ast.Expr
	switch f := c.Fun.(type) {
	case *ast.Ident:
		if len(c.Args) == 1 && (f.Name == "obj_" || f.Name == "any" || f.Name == "strobj_") {
			x = c.Args[0]
			if f.Name == "strobj_" {
				return x, types.Typ[types.String], false
			}
		}
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok && id.Name == "obj_" && len(c.Args) == 1 {
			x = c.Args[0]
		}
	case *ast.SelectorExpr:
		if len(c.Args) == 0 && f.Sel.Name == "box_" {
			x = f.X
		} else if len(c.Args) == 0 && f.Sel.Name == "self" {
			tv := rw.Info.Types[f.X]
			if p, ok := tv.Type.(*types.Pointer); ok {
				if nt, ok := p.Elem().(*types.Named); ok {
					x, self = f.X, extended[nt]
				}
			}
		}
	}
	if x == nil {
		return nil, nil, false
	}
	tv, ok := rw.Info.Types[x]
	if !ok || tv.Type == nil {
		return nil, nil, false
	}
	t := tv.Type
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		if _, ok := u.Elem().(*types.Named); ok {
			if _, ok := t.(*types.Named); !ok {
				return x, t, self
			}
		}
	case *types.Basic:
		switch u.Kind() {
		case types.String:
			// Only strobj_ maps nullstr_ to nil; any(s) would box the marker.
			return nil, nil, false
		case types.Int32, types.Int64, types.Int16, types.Int8, types.Uint16, types.Bool:
			if t == u {
				return x, t, self
			}
			if handle(t) {
				return x, t, self
			}
		case types.Uint32, types.Uint8, types.Uint64, types.Int:
			if handle(t) {
				return x, t, self
			}
		}
	}
	return nil, nil, false
}

func fnName(c *ast.CallExpr) string {
	if id, ok := c.Fun.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// handle reports whether t is a handle type (Node): a named integer type boxed by box_ and
// unboxed by T_of_, whose zero value is null.
func handle(t types.Type) bool {
	nt, ok := t.(*types.Named)
	if !ok {
		return false
	}
	return rw.HasMethod(nt, "box_") && rw.HasFunc(nt.Obj().Name()+"_of_")
}

// method is t's method name with signature sig, if it has one.
func method(t types.Type, name, sig string) *types.Func {
	o, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, name)
	fn, ok := o.(*types.Func)
	if !ok {
		return nil
	}
	// The method's types, without the parameter names: what an interface{ name(...) } asks.
	s := fn.Type().(*types.Signature)
	unnamed := func(t *types.Tuple) *types.Tuple {
		var vs []*types.Var
		for i := 0; i < t.Len(); i++ {
			vs = append(vs, types.NewParam(token.NoPos, nil, "", t.At(i).Type()))
		}
		return types.NewTuple(vs...)
	}
	if types.TypeString(types.NewSignatureType(nil, nil, nil, unnamed(s.Params()), unnamed(s.Results()), s.Variadic()), nil) != sig {
		return nil
	}
	return fn
}

func hasEquals(t types.Type) bool   { return method(t, "equals", "func(any) bool") != nil }
func hasHashCode(t types.Type) bool { return method(t, "hashCode", "func() int32") != nil }

// identityType reports whether two values of type t compare as Java compares them with ==.
func identityType(t types.Type) bool {
	if b, ok := t.(*types.Basic); ok {
		return b.Kind() != types.Float64 && b.Kind() != types.Float32
	}
	return !hasEquals(t)
}

// receiverExpr is the Go expression naming method m of t's class, for a method expression.
func methodExpr(t types.Type, m string) string {
	return "(" + typeString(t) + ")." + m
}

func typeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == rw.Pkg {
			return ""
		}
		return p.Name()
	})
}

// sure reports whether x is certainly not null: this, or a constant.
func sure(x ast.Expr) bool {
	x = rw.Unparen(x)
	if id, ok := x.(*ast.Ident); ok && id.Name == "this" {
		return true
	}
	if tv, ok := rw.Info.Types[x]; ok && tv.Value != nil {
		return true
	}
	if bl, ok := x.(*ast.BasicLit); ok && bl.Kind == token.STRING {
		return true
	}
	return false
}

// paren wraps s in parentheses when the call it replaces is an operand of an operator that
// binds as tightly as ==.
func paren(c ast.Expr, s string) string {
	switch p := rw.Up(c).(type) {
	case *ast.UnaryExpr, *ast.SelectorExpr, *ast.StarExpr:
		return "(" + s + ")"
	case *ast.BinaryExpr:
		if p.Op.Precedence() >= token.EQL.Precedence() {
			return "(" + s + ")"
		}
	}
	return s
}

// identity rewrites the call c as a == b, or its negation !c as a != b.
func identity(c *ast.CallExpr, a, b ast.Expr) {
	if u, ok := rw.Up(c).(*ast.UnaryExpr); ok && u.Op == token.NOT {
		rw.Rewrite[u] = func() string { return paren(u, rw.Render(a)+" != "+rw.Render(b)) }
		return
	}
	rw.Rewrite[c] = func() string { return paren(c, rw.Render(a)+" == "+rw.Render(b)) }
}

func run(report bool) {
	counts := map[string]int{}
	left := map[string]int{}
	for _, f := range rw.Files {
		if !strings.HasPrefix(rw.Fset.Position(f.Pos()).Filename[strings.LastIndex(rw.Fset.Position(f.Pos()).Filename, "/")+1:], "g_") {
			continue
		}
		topVar := map[*ast.GenDecl]bool{}
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok {
				topVar[g] = true
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			// Package-level variable initializers are left alone: a direct method reference
			// there (the hash and eq of a key conversion, tch_*) would put the variable in an
			// initialization cycle with every method that reaches it.
			if g, ok := n.(*ast.GenDecl); ok && g.Tok == token.VAR && topVar[g] {
				return false
			}
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name := fnName(c)
			switch name {
			case "Objects_equals", "objequals_", "Object_equals":
				if len(c.Args) != 2 {
					return true
				}
				if k := equalsCall(c, name == "Object_equals"); k != "" {
					counts[name+": "+k]++
				} else {
					left[name+": "+why(c.Args...)]++
				}
			case "Objects_hashCode", "objhash_", "Object_hashCode":
				if len(c.Args) != 1 {
					return true
				}
				if k := hashCall(c, name == "Object_hashCode"); k != "" {
					counts[name+": "+k]++
				} else {
					left[name+": "+why(c.Args...)]++
				}
			}
			return true
		})
	}
	print := func(title string, m map[string]int) {
		var ks []string
		total := 0
		for k, v := range m {
			ks = append(ks, k)
			total += v
		}
		sort.Strings(ks)
		fmt.Fprintf(os.Stderr, "%s: %d\n", title, total)
		for _, k := range ks {
			fmt.Fprintf(os.Stderr, "  %5d %s\n", m[k], k)
		}
	}
	print("rewritten", counts)
	print("left", left)
	if !report {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
	}
}

// why describes the arguments of a call that is left.
func why(args ...ast.Expr) string {
	var parts []string
	for _, a := range args {
		x, t := exact(a)
		switch {
		case x == nil:
			tv := rw.Info.Types[rw.Unparen(a)]
			s := "not exact"
			if c, ok := rw.Unparen(a).(*ast.CallExpr); ok {
				if id, ok := c.Fun.(*ast.Ident); ok {
					s = id.Name + "(...)"
					if len(c.Args) == 1 && rw.Info.Types[c.Args[0]].Type != nil {
						s = id.Name + "(" + typeString(rw.Info.Types[c.Args[0]].Type) + ")"
					}
				} else if sel, ok := c.Fun.(*ast.SelectorExpr); ok {
					s = "." + sel.Sel.Name + "()"
				}
			} else if tv.Type != nil {
				s = "value of " + typeString(tv.Type)
			}
			parts = append(parts, s)
		default:
			parts = append(parts, typeString(t))
		}
	}
	return strings.Join(parts, ", ")
}

// classOf is the class that values of pointer type t point to.
func classOf(t types.Type) *types.Named {
	if p, ok := t.(*types.Pointer); ok {
		nt, _ := p.Elem().(*types.Named)
		return nt
	}
	return nil
}

func equalsCall(c *ast.CallExpr, npe bool) string {
	a, ta, sa := exactOrSelf(c.Args[0])
	b, tb, sb := exactOrSelf(c.Args[1])
	if sa || sb {
		// x.self() of a class with subclasses: the most derived object. When neither the class
		// nor a subclass overrides equals, the call compares identities, and two pointers to
		// the class's part of an object are equal when the objects are.
		if a == nil || b == nil || !types.Identical(ta, tb) || overridden(classOf(ta), "equals", "func(any) bool") {
			return ""
		}
		switch {
		case !npe || sure(a):
			identity(c, a, b)
			return "identity (==), subclasses without equals"
		default:
			rw.Rewrite[c] = func() string { return "equalsId_(" + rw.Render(a) + ", " + rw.Render(b) + ")" }
			return "identity, receiver may be null, subclasses without equals"
		}
	}
	if a == nil {
		return ""
	}
	if b != nil && types.Identical(ta, tb) && identityType(ta) {
		_, basic := ta.(*types.Basic)
		isStr := basic && ta.(*types.Basic).Kind() == types.String
		switch {
		case !npe || (basic && !isStr) || sure(a):
			identity(c, a, b)
			return "identity (==)"
		case isStr:
			rw.Rewrite[c] = func() string { return "strequals_(" + rw.Render(a) + ", " + rw.Render(b) + ")" }
			return "string, receiver may be null"
		default:
			rw.Rewrite[c] = func() string { return "equalsId_(" + rw.Render(a) + ", " + rw.Render(b) + ")" }
			return "identity, receiver may be null"
		}
	}
	if _, ok := ta.Underlying().(*types.Pointer); ok && hasEquals(ta) {
		me := methodExpr(ta, "equals")
		B := c.Args[1]
		switch {
		case npe && sure(a):
			rw.Rewrite[c] = func() string { return rw.Render(a) + ".equals(" + rw.Render(B) + ")" }
			return "equals method, on this"
		case npe:
			rw.Rewrite[c] = func() string { return "equalsBy_(" + rw.Render(a) + ", " + rw.Render(B) + ", " + me + ")" }
			return "equals method"
		default:
			rw.Rewrite[c] = func() string { return "objequalsBy_(" + rw.Render(a) + ", " + rw.Render(B) + ", " + me + ")" }
			return "equals method, null-safe"
		}
	}
	return ""
}

func hashCall(c *ast.CallExpr, npe bool) string {
	a, ta, self := exactOrSelf(c.Args[0])
	if a == nil {
		return ""
	}
	if self {
		// The identity hash of the most derived object, when no subclass has a hashCode.
		if npe || overridden(classOf(ta), "hashCode", "func() int32") {
			return ""
		}
		A := c.Args[0]
		rw.Rewrite[c] = func() string { return "objhashId_(" + rw.Render(A) + ")" }
		return "identity hash, subclasses without hashCode"
	}
	if b, ok := ta.(*types.Basic); ok {
		switch b.Kind() {
		case types.Int32:
			rw.Rewrite[c] = func() string { return rw.Render(a) }
			return "int"
		case types.Bool:
			rw.Rewrite[c] = func() string { return "Boolean_hashCode(" + rw.Render(a) + ")" }
			return "boolean"
		case types.String:
			if npe {
				return ""
			}
			rw.Rewrite[c] = func() string { return "objhashStr_(" + rw.Render(a) + ")" }
			return "string"
		}
		return ""
	}
	if hasHashCode(ta) {
		switch {
		case sure(a):
			rw.Rewrite[c] = func() string { return rw.Render(a) + ".hashCode()" }
			return "hashCode method, on this"
		case npe:
			return ""
		default:
			me := methodExpr(ta, "hashCode")
			rw.Rewrite[c] = func() string { return "objhashBy_(" + rw.Render(a) + ", " + me + ")" }
			return "hashCode method"
		}
	}
	if npe {
		return ""
	}
	rw.Rewrite[c] = func() string { return "objhashId_(" + rw.Render(a) + ")" }
	return "identity hash"
}
