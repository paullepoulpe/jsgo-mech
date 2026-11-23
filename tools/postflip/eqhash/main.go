// Command eqhash is a post-flip step: equals and hashCode calls whose static types decide what
// they do are written as what they do, without the runtime's dynamic protocol.
//
// j2go translates Objects.equals(a, b), a.equals(b) on an Object and Objects.hashCode(a) as
// calls of runtime functions taking `any`: Objects_equals(obj_(a), obj_(b)) boxes both values,
// and Object_equals then asks the dynamic type of a for an equals method (an interface
// assertion), else compares the two values as Go values (identity for a pointer). Where the
// static type of a boxed value bounds its dynamic type, this step decides the call at compile
// time:
//
//   - the value is boxed by obj_(x) or any(x) (the dynamic type is the static type of x: a
//     pointer to a class, a node handle, an enum constant, an Optional, a string or a
//     primitive, or one of the types that implement x's interface type), by strobj_(s) (a
//     string), by x.box_() (a handle), by Boolean_valueOf(b), or by x.self() (the most derived
//     object of x's class: x's class or one of its subclasses);
//   - identity: when both values have one such type T and T has no equals method (a class that
//     does not override equals, an enum, Node, ...), or T is a string or an integer or boolean
//     type, Java compares them by identity or value, which is Go's == on the unboxed values:
//     Objects_equals(obj_(a), obj_(b)) -> a == b (null equals only null: a nil pointer, a zero
//     handle or enum, nullstr_). Object_equals(obj_(a), obj_(b)) (a.equals(b)) throws a
//     NullPointerException when a is null: equalsId_(a, b) (strequals_ for strings), or a == b
//     when a is `this` or a constant. Against a value of type any: obj_(a) == B, which is what
//     the runtime compares, or equalsIdAny_(a, B);
//   - a type with its own equals: Object_equals(obj_(a), B) calls T's equals directly:
//     a.equals(B) when a is `this`, else equalsBy_(a, B, (*C).equals), which throws on null
//     as before; Objects_equals(obj_(a), B) -> objequalsBy_(a, B, (*C).equals), which keeps
//     Objects.equals' null cases; two Optionals: optequals_(a, b), without boxing either;
//   - hashCode: Objects_hashCode(A) is 0 for null, else the type's hashCode method, called
//     directly (objhashBy_(a, (*C).hashCode), a.hashCode() for `this`), or the identity hash
//     (objhashId_(a)) when the type has none (objhashHandle_ for a handle, unboxed); String.hashCode of a string (objhashStr_(s)); an
//     int is its own hash, a boolean 1231 or 1237. Object_hashCode(A) (a.hashCode()) throws on
//     null: hashBy_, hashId_;
//   - x.self() of a class with subclasses boxes the most derived object: when neither the
//     class nor a subclass has an equals (hashCode) method, the call is identity (the identity
//     hash of that object), and two pointers to the class's part of objects are equal when the
//     objects are. When the class has one and no subclass declares its own (a subclass that
//     overrides it declares equals_impl, which the class's equals dispatches to), the most
//     derived object's equals is the class's: objequalsBy_(x, B, (*C).equals);
//   - a value of an interface type: the dynamic types are the package's types that implement
//     it (all its methods are unexported, so no other package's type does). When the interface
//     has equals (hashCode), the call is through it (equalsBy_(a, B, (I).equals)); when every
//     implementing type has one, the runtime's assertion is the only outcome
//     (equalsVia_, objequalsVia_, objhashVia_); when none has one and all are comparable, it
//     is identity as above;
//   - Objects.hash(a, b, ...) of values whose hashes are all decided is hashOf_(h(a), h(b),
//     ...): 31-based, as Arrays.hashCode, without the []any;
//   - Objects.requireNonNull(x, msg), as a statement or as the value of an assignment, is an
//     inline nil check (`if x == nil { panic(NPE(msg)) }`) before the assignment of x; in other
//     expressions requireNonNull_(x, msg), which returns x unboxed.
//
// The runtime helpers (rt_eqhash.go) are generic and small, so the compiler inlines them and the
// method expression becomes a direct call. Values of type any, pairs of different types and
// interfaces whose implementing types mix identity and value equality are left to the runtime.
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

// named are the package's non-alias, non-interface named types (the possible dynamic types of
// an interface value).
var named []*types.Named

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

// declaredBelow reports whether a strict subclass of nt declares method name itself (not
// promoted from the class it embeds).
func declaredBelow(nt *types.Named, name string) bool {
	for _, s := range subs[nt] {
		if o, idx, _ := types.LookupFieldOrMethod(types.NewPointer(s), true, rw.Pkg, name); o != nil && len(idx) == 1 {
			return true
		}
		if declaredBelow(s, name) {
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
		if nt, ok := tn.Type().(*types.Named); ok {
			if _, isIface := nt.Underlying().(*types.Interface); !isIface {
				named = append(named, nt)
			}
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
			if nt, ok := types.Unalias(t).(*types.Named); ok {
				extended[nt] = true
				subs[nt] = append(subs[nt], tn.Type().(*types.Named))
			}
		}
	}
	run(os.Args[1] == "-n")
}

// boxed is a value boxed into an `any` whose dynamic type the static type of x bounds.
type boxed struct {
	x    ast.Expr
	t    types.Type // the type of x, without aliases
	self bool       // x.self() of a class with subclasses: the most derived object
	// keepNull: the box is never nil (any(s) of a string boxes nullstr_; an implicit
	// conversion of a pointer boxes a nil pointer): the runtime sees a non-null value.
	keepNull bool
}

// exact returns the value that boxed expression e boxes, when its dynamic type is its static
// type, and that type.
func exact(e ast.Expr) (ast.Expr, types.Type) {
	b, ok := unbox(e)
	if !ok || b.self || b.keepNull || isIface(b.t) {
		return nil, nil
	}
	return b.x, b.t
}

func isIface(t types.Type) bool {
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

// enumType reports whether t is an enum whose constants are integers (0 is null).
func enumType(t types.Type) bool {
	nt, ok := t.(*types.Named)
	if !ok {
		return false
	}
	b, ok := nt.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0 && rw.HasMethod(nt, "enum_")
}

// optionalType reports whether t is an instantiation of the runtime's Optional.
func optionalType(t types.Type) bool {
	nt, ok := t.(*types.Named)
	return ok && nt.Origin().Obj().Name() == "Optional" && nt.Origin().Obj().Pkg() == rw.Pkg
}

// unbox analyses a boxed expression e.
func unbox(e ast.Expr) (boxed, bool) {
	e = rw.Unparen(e)
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return boxed{}, false
	}
	var x ast.Expr
	via := ""
	switch f := c.Fun.(type) {
	case *ast.Ident:
		if len(c.Args) == 1 {
			switch f.Name {
			case "obj_", "any":
				x, via = c.Args[0], f.Name
			case "strobj_":
				return boxed{x: c.Args[0], t: types.Typ[types.String]}, true
			case "Boolean_valueOf":
				return boxed{x: c.Args[0], t: types.Typ[types.Bool]}, true
			}
		}
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok && id.Name == "obj_" && len(c.Args) == 1 {
			x, via = c.Args[0], "obj_"
		}
	case *ast.SelectorExpr:
		if len(c.Args) == 0 && f.Sel.Name == "box_" {
			x, via = f.X, "box_"
		} else if len(c.Args) == 0 && f.Sel.Name == "self" {
			tv := rw.Info.Types[f.X]
			if tv.Type == nil {
				return boxed{}, false
			}
			t := types.Unalias(tv.Type)
			if p, ok := t.(*types.Pointer); ok {
				if nt, ok := types.Unalias(p.Elem()).(*types.Named); ok {
					return boxed{x: f.X, t: t, self: extended[nt]}, true
				}
			}
			if enumType(t) {
				// An enum constant's self() is the constant, nil for 0.
				return boxed{x: f.X, t: t}, true
			}
		}
	}
	if x == nil {
		return boxed{}, false
	}
	tv, ok := rw.Info.Types[x]
	if !ok || tv.Type == nil {
		return boxed{}, false
	}
	return classify(x, types.Unalias(tv.Type), via)
}

// classify is unbox for a value x of type t boxed by obj_, any or box_.
func classify(x ast.Expr, t types.Type, via string) (boxed, bool) {
	switch u := t.Underlying().(type) {
	case *types.Pointer:
		if _, ok := types.Unalias(u.Elem()).(*types.Named); ok {
			if _, ok := t.(*types.Named); !ok {
				return boxed{x: x, t: t, keepNull: via == ""}, true
			}
		}
	case *types.Interface:
		if via == "obj_" || via == "any" || via == "" {
			for i := 0; i < u.NumMethods(); i++ {
				if u.Method(i).Exported() {
					return boxed{}, false
				}
			}
			if u.NumMethods() > 0 {
				return boxed{x: x, t: t}, true
			}
		}
	case *types.Struct:
		if via == "obj_" && optionalType(t) {
			return boxed{x: x, t: t}, true
		}
	case *types.Basic:
		switch u.Kind() {
		case types.String:
			// Only strobj_ maps nullstr_ to nil; any(s) boxes the marker, which the runtime
			// then compares and hashes as a string.
			if (via == "any" || via == "") && t == u {
				return boxed{x: x, t: t, keepNull: true}, true
			}
			return boxed{}, false
		case types.Int32, types.Int64, types.Int16, types.Int8, types.Uint16, types.Bool:
			if t == u && via != "box_" {
				return boxed{x: x, t: t}, true
			}
			if handle(t) {
				return boxed{x: x, t: t}, true
			}
		case types.Uint32, types.Uint8, types.Uint64, types.Int:
			if handle(t) {
				return boxed{x: x, t: t}, true
			}
			if via == "obj_" && enumType(t) {
				return boxed{x: x, t: t}, true
			}
		}
	}
	return boxed{}, false
}

// unboxArg is unbox, and also takes a value passed to an `any` parameter without a boxing call
// (an implicit conversion of a pointer: a nil pointer is a non-nil box).
func unboxArg(e ast.Expr) (boxed, bool) {
	if b, ok := unbox(e); ok {
		return b, true
	}
	tv := rw.Info.Types[rw.Unparen(e)]
	if tv.Type == nil {
		return boxed{}, false
	}
	if it, ok := tv.Type.Underlying().(*types.Interface); ok && it.Empty() {
		return boxed{}, false
	}
	t := types.Unalias(tv.Type)
	if handle(t) {
		// A handle converted implicitly boxes the handle value; 0 is a non-nil box.
		return boxed{x: e, t: t, keepNull: true}, true
	}
	return classify(e, t, "")
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

const eqSig, hashSig = "func(any) bool", "func() int32"

func hasEquals(t types.Type) bool   { return method(t, "equals", eqSig) != nil }
func hasHashCode(t types.Type) bool { return method(t, "hashCode", hashSig) != nil }

// impls are the dynamic types a value of interface type t can have: the package's named types
// and pointers to them that implement it. A generic type is counted when it has methods of all
// the interface's names (its instantiations may implement it).
var implsOf = map[types.Type][]types.Type{}

func impls(t types.Type) []types.Type {
	if r, ok := implsOf[t]; ok {
		return r
	}
	it := t.Underlying().(*types.Interface)
	var r []types.Type
	for _, nt := range named {
		if nt.TypeParams() != nil {
			ms := types.NewMethodSet(types.NewPointer(nt))
			all := true
			for i := 0; i < it.NumMethods() && all; i++ {
				all = ms.Lookup(rw.Pkg, it.Method(i).Name()) != nil
			}
			if all {
				r = append(r, types.NewPointer(nt))
			}
			continue
		}
		if types.Implements(nt, it) {
			r = append(r, nt)
		} else if p := types.NewPointer(nt); types.Implements(p, it) {
			r = append(r, p)
		}
	}
	implsOf[t] = r
	return r
}

// has reports whether a dynamic type t has the method; for a generic type, by name.
func has(t types.Type, name, sig string) bool {
	if p, ok := t.(*types.Pointer); ok {
		if nt, ok := p.Elem().(*types.Named); ok && nt.TypeParams() != nil {
			return types.NewMethodSet(t).Lookup(rw.Pkg, name) != nil
		}
	}
	return method(t, name, sig) != nil
}

// Modes of an equals or hashCode call on a value of a type.
const (
	mNone     = iota // undecided
	mIdentity        // no equals (hashCode): identity, the identity hash
	mMethod          // the static type has the method: call it
	mVia             // every dynamic type has it, the static type does not: the runtime's assertion
	mMixed           // some dynamic types have it, the others are comparable: ask the value
)

// mode is how equals (hashCode) of a value of type t (not x.self()) is decided.
func mode(t types.Type, name, sig string) int {
	if b, ok := t.(*types.Basic); ok {
		if b.Kind() == types.Float64 || b.Kind() == types.Float32 {
			return mNone
		}
		return mIdentity
	}
	if method(t, name, sig) != nil {
		return mMethod
	}
	if !isIface(t) {
		return mIdentity
	}
	all, none, comparable := true, true, true
	for _, d := range impls(t) {
		if has(d, name, sig) {
			none = false
		} else {
			all = false
			if b, ok := d.Underlying().(*types.Basic); !types.Comparable(d) || ok && b.Info()&types.IsFloat != 0 {
				comparable = false
			}
		}
	}
	switch {
	case !comparable:
		return mNone
	case none:
		return mIdentity
	case all:
		return mVia
	}
	return mMixed
}

// selfMode is mode for x.self() of a class nt with subclasses.
func selfMode(nt *types.Named, name, sig string) int {
	if method(types.NewPointer(nt), name, sig) != nil {
		if declaredBelow(nt, name) {
			return mNone
		}
		return mMethod
	}
	if overridden(nt, name, sig) {
		return mNone
	}
	return mIdentity
}

// methodExpr is the method expression of method m of x's type t, spelled as x's type is
// declared (an alias such as ImmutableSetOf[T] kept).
func methodExpr(x ast.Expr, t types.Type, m string) string {
	if d := rw.Info.Types[x].Type; d != nil && types.Identical(types.Unalias(d), t) {
		t = d
	}
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
	identityS(c, func() string { return rw.Render(a) }, func() string { return rw.Render(b) })
}

func identityS(c *ast.CallExpr, a, b func() string) {
	if u, ok := rw.Up(c).(*ast.UnaryExpr); ok && u.Op == token.NOT {
		rw.Rewrite[u] = func() string { return paren(u, a()+" != "+b()) }
		return
	}
	rw.Rewrite[c] = func() string { return paren(c, a()+" == "+b()) }
}

func run(report bool) {
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
			if g, ok := n.(*ast.GenDecl); ok && g.Tok == token.VAR && topVar[g] {
				inTopVar = true
				ast.Inspect(g, visit)
				inTopVar = false
				return false
			}
			return visit(n)
		})
	}
	finish(report)
}

// inTopVar: the call is in the initializer of a package-level variable, where a method
// expression would put the variable in an initialization cycle with every method that reaches
// it (the hash and eq of a key conversion, tch_*): the methods are called through the
// value's interface instead (equalsVia_, objhashVia_), with no static reference.
var inTopVar bool

var counts, left = map[string]int{}, map[string]int{}

// visit rewrites n if it is a call of the protocol.
func visit(n ast.Node) bool {
	c, ok := n.(*ast.CallExpr)
	if !ok {
		return true
	}
	k := ""
	name := fnName(c)
	switch name {
	case "Objects_equals", "objequals_", "Object_equals":
		if len(c.Args) != 2 {
			return true
		}
		k = equalsCall(c, name == "Object_equals")
	case "Objects_hashCode", "objhash_", "Object_hashCode":
		if len(c.Args) != 1 {
			return true
		}
		k = hashCall(c, name == "Object_hashCode")
	case "Objects_hash":
		k = hashAll(c)
	case "Objects_requireNonNull", "Objects_requireNonNull_Object":
		if len(c.Args) == 0 {
			return true
		}
		k = requireNonNull(c, name == "Objects_requireNonNull")
		if k == "" {
			left[name+": "+why(c.Args[0])]++
			return true
		}
	default:
		return true
	}
	if k != "" {
		counts[name+": "+k]++
	} else {
		left[name+": "+why(c.Args...)]++
	}
	return true
}

func finish(report bool) {
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
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
}

// why describes the arguments of a call that is left.
func why(args ...ast.Expr) string {
	var parts []string
	for _, a := range args {
		b, ok := unboxArg(a)
		switch {
		case !ok:
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
			} else if cl, ok := rw.Unparen(a).(*ast.CompositeLit); ok {
				s = "[" + why(cl.Elts...) + "]"
			} else if tv.Type != nil {
				s = "value of " + typeString(tv.Type)
			}
			parts = append(parts, s)
		case b.self:
			parts = append(parts, typeString(b.t)+".self()")
		case isIface(b.t):
			parts = append(parts, "interface "+typeString(b.t))
		default:
			parts = append(parts, typeString(b.t))
		}
	}
	return strings.Join(parts, ", ")
}

// classOf is the class that values of pointer type t point to.
func classOf(t types.Type) *types.Named {
	if p, ok := t.(*types.Pointer); ok {
		nt, _ := types.Unalias(p.Elem()).(*types.Named)
		return nt
	}
	return nil
}

func render(e ast.Expr) func() string { return func() string { return rw.Render(e) } }

// equalsCall rewrites c, Object_equals(A, B) (a.equals(b), npe) or Objects_equals(A, B).
func equalsCall(c *ast.CallExpr, npe bool) string {
	ba, ok := unbox(c.Args[0])
	if !ok {
		return ""
	}
	a, ta := ba.x, ba.t
	bb, bok := unbox(c.Args[1])
	B := c.Args[1]
	sameType := bok && bb.self == ba.self && bb.keepNull == ba.keepNull && types.Identical(ta, bb.t)
	var m int
	if ba.self {
		m = selfMode(classOf(ta), "equals", eqSig)
	} else {
		m = mode(ta, "equals", eqSig)
	}
	switch m {
	case mIdentity:
		basic, isBasic := ta.(*types.Basic)
		isStr := isBasic && basic.Kind() == types.String
		if !sameType {
			// Against a value of another static type (of type any): the runtime compares the
			// boxes, which is what obj_(a) == B does.
			if ba.keepNull || ba.self || isStr || isBasic || bok || handle(ta) {
				// (A handle boxed by obj_ would allocate; its box_ does not.)
				return ""
			}
			if !npe {
				identityS(c, func() string { return "obj_(" + rw.Render(a) + ")" }, render(B))
				return "identity (==), against a value of type any"
			}
			rw.Rewrite[c] = func() string { return "equalsIdAny_(" + rw.Render(a) + ", " + rw.Render(B) + ")" }
			return "identity, receiver may be null, against a value of type any"
		}
		b := bb.x
		kind := "identity"
		if ba.self {
			kind += ", subclasses without equals"
		} else if isIface(ta) {
			kind += ", interface whose types have no equals"
		}
		switch {
		case !npe || (isBasic && !isStr) || sure(a) || ba.keepNull:
			// ba.keepNull: any(s) boxes nullstr_, which the runtime compares as a string.
			identity(c, a, b)
			return kind + " (==)"
		case isStr:
			rw.Rewrite[c] = func() string { return "strequals_(" + rw.Render(a) + ", " + rw.Render(b) + ")" }
			return "string, receiver may be null"
		default:
			rw.Rewrite[c] = func() string { return "equalsId_(" + rw.Render(a) + ", " + rw.Render(b) + ")" }
			return kind + ", receiver may be null"
		}
	case mMethod:
		if ba.keepNull {
			return ""
		}
		if inTopVar {
			return equalsDyn(c, npe, a, B, "Via", "equals method, called through the value (package-level initializer)")
		}
		me := methodExpr(a, ta, "equals")
		kind := "equals method"
		switch {
		case ba.self:
			kind += " of the class, no subclass declares one"
		case optionalType(ta):
			kind += " of Optional"
		case isIface(ta):
			kind += " of the interface"
		}
		switch {
		case npe && sure(a):
			rw.Rewrite[c] = func() string { return rw.Render(a) + ".equals(" + rw.Render(B) + ")" }
			return kind + ", on this"
		case optionalType(ta) && sameType:
			x, y := unerase(a, bb.x)
			h := "optequals_("
			if npe {
				h = "optequalsOn_("
			}
			rw.Rewrite[c] = func() string { return h + rw.Render(x) + ", " + rw.Render(y) + ")" }
			return kind + ", two Optionals"
		case npe:
			rw.Rewrite[c] = func() string { return "equalsBy_(" + rw.Render(a) + ", " + rw.Render(B) + ", " + me + ")" }
			return kind
		default:
			rw.Rewrite[c] = func() string { return "objequalsBy_(" + rw.Render(a) + ", " + rw.Render(B) + ", " + me + ")" }
			return kind + ", null-safe"
		}
	case mVia:
		return equalsDyn(c, npe, a, B, "Via", "interface whose types all have equals")
	case mMixed:
		return equalsDyn(c, npe, a, B, "Dyn", "interface whose types mix equals and identity")
	}
	return ""
}

// unerase returns x and y without optErase_ when both are optErase_ of Optionals of one type
// (equal as Optional<?> exactly when equal as Optional<T>).
func unerase(x, y ast.Expr) (ast.Expr, ast.Expr) {
	nx, cx := rw.CallName(x)
	ny, cy := rw.CallName(y)
	if nx == "optErase_" && ny == "optErase_" && len(cx.Args) == 1 && len(cy.Args) == 1 {
		tx, ty := rw.Info.Types[cx.Args[0]].Type, rw.Info.Types[cy.Args[0]].Type
		if tx != nil && ty != nil && types.Identical(tx, ty) {
			return cx.Args[0], cy.Args[0]
		}
	}
	return x, y
}

// equalsDyn rewrites c as equals<h>_(a, B) (npe) or objequals<h>_(a, B).
func equalsDyn(c *ast.CallExpr, npe bool, a, B ast.Expr, h, kind string) string {
	if npe {
		rw.Rewrite[c] = func() string { return "equals" + h + "_(" + rw.Render(a) + ", " + rw.Render(B) + ")" }
		return kind
	}
	rw.Rewrite[c] = func() string { return "objequals" + h + "_(" + rw.Render(a) + ", " + rw.Render(B) + ")" }
	return kind + ", null-safe"
}

// hashGen is the code of Objects.hashCode(A) (npe false) or A.hashCode() (npe true) for a
// boxed argument A, and what it is.
func hashGen(A ast.Expr, npe bool) (func() string, string) {
	ba, ok := unboxArg(A)
	if !ok {
		return nil, ""
	}
	a, ta := ba.x, ba.t
	if b, ok := ta.(*types.Basic); ok && !ba.self {
		switch b.Kind() {
		case types.Int32:
			return render(a), "int"
		case types.Bool:
			return func() string { return "Boolean_hashCode(" + rw.Render(a) + ")" }, "boolean"
		case types.String:
			if ba.keepNull {
				// any(s): the runtime hashes the string, nullstr_ too.
				return func() string { return "String_hashCode(" + rw.Render(a) + ")" }, "string boxed by any"
			}
			if npe {
				return nil, ""
			}
			return func() string { return "objhashStr_(" + rw.Render(a) + ")" }, "string"
		}
		return nil, ""
	}
	var m int
	if ba.self {
		m = selfMode(classOf(ta), "hashCode", hashSig)
	} else {
		m = mode(ta, "hashCode", hashSig)
	}
	kind := ""
	switch {
	case ba.self:
		kind = ", x.self()"
	case optionalType(ta):
		kind = ", Optional"
	case enumType(ta):
		kind = ", enum"
	case isIface(ta):
		kind = ", interface"
	}
	if m == mMethod && inTopVar {
		m = mVia
		kind += ", called through the value (package-level initializer)"
	}
	if n, oc := rw.CallName(a); m == mMethod && n == "optErase_" && len(oc.Args) == 1 {
		// The hash of an Optional<?> is the hash of the Optional<T> it is.
		if t := rw.Info.Types[oc.Args[0]].Type; t != nil && optionalType(types.Unalias(t)) {
			a, ta = oc.Args[0], types.Unalias(t)
		}
	}
	switch m {
	case mMethod:
		me := methodExpr(a, ta, "hashCode")
		switch {
		case sure(a) || (npe && ba.keepNull):
			// A pointer converted implicitly: the runtime calls the method on it, nil or not.
			return func() string { return rw.Render(a) + ".hashCode()" }, "hashCode method" + kind + ", on this"
		case ba.keepNull:
			return nil, ""
		case npe:
			return func() string { return "hashBy_(" + rw.Render(a) + ", " + me + ")" }, "hashCode method" + kind + ", receiver may be null"
		}
		return func() string { return "objhashBy_(" + rw.Render(a) + ", " + me + ")" }, "hashCode method" + kind
	case mVia, mMixed:
		h, what := "Via", "all have hashCode"
		if m == mMixed {
			h, what = "Dyn", "mix hashCode and the identity hash"
		}
		if !isIface(ta) {
			what = "hashCode method"
		} else {
			what = "interface whose types " + what
		}
		switch {
		case ba.keepNull:
			return nil, ""
		case npe && !sure(a):
			return func() string { return "hash" + h + "_(" + rw.Render(a) + ")" }, what + kind + ", receiver may be null"
		}
		return func() string { return "objhash" + h + "_(" + rw.Render(a) + ")" }, what + kind
	case mIdentity:
		if ba.self {
			// The identity hash of the most derived object.
			A := A
			if npe && !sure(a) {
				return nil, ""
			}
			return func() string { return "objhashId_(" + rw.Render(A) + ")" }, "identity hash" + kind
		}
		switch {
		case ba.keepNull:
			if handle(ta) && rw.HasMethod(ta, "handle_") {
				// A handle converted implicitly: the runtime hashes the handle, 0 too.
				return func() string { return "handleHash_(" + rw.Render(a) + ".handle_())" }, "identity hash, handle"
			}
			return nil, ""
		case handle(ta) && rw.HasMethod(ta, "handle_"):
			// The identity hash of a handle is the hash of its number (identityHash_), taken
			// here without boxing the handle.
			if npe && !sure(a) {
				return func() string { return "hashHandle_(" + rw.Render(a) + ")" }, "identity hash, handle, receiver may be null"
			}
			return func() string { return "objhashHandle_(" + rw.Render(a) + ")" }, "identity hash, handle"
		case npe && !sure(a):
			return func() string { return "hashId_(" + rw.Render(a) + ")" }, "identity hash" + kind + ", receiver may be null"
		}
		return func() string { return "objhashId_(" + rw.Render(a) + ")" }, "identity hash" + kind
	}
	return nil, ""
}

func hashCall(c *ast.CallExpr, npe bool) string {
	g, k := hashGen(c.Args[0], npe)
	if g == nil {
		return ""
	}
	rw.Rewrite[c] = g
	return k
}

// hashAll rewrites Objects_hash([]any{a, b, ...}) as hashOf_(h(a), h(b), ...) when every
// element's hash is decided.
func hashAll(c *ast.CallExpr) string {
	if len(c.Args) != 1 {
		return ""
	}
	cl, ok := rw.Unparen(c.Args[0]).(*ast.CompositeLit)
	if !ok || len(cl.Elts) == 0 {
		return ""
	}
	var gs []func() string
	for _, e := range cl.Elts {
		g, _ := hashGen(e, false)
		if g == nil {
			return ""
		}
		gs = append(gs, g)
	}
	rw.Rewrite[c] = func() string {
		var parts []string
		for _, g := range gs {
			parts = append(parts, g())
		}
		return "hashOf_(" + strings.Join(parts, ", ") + ")"
	}
	return fmt.Sprintf("%d values", len(gs))
}

// nullTest is the Go test that value x of type t is Java null.
func nullTest(x ast.Expr, t types.Type, keepNull bool) string {
	switch u := t.Underlying().(type) {
	case *types.Pointer, *types.Interface:
		return rw.Render(x) + " == nil"
	case *types.Basic:
		switch {
		case u.Kind() == types.String:
			return rw.Render(x) + " == nullstr_"
		case handle(t) || enumType(t):
			return rw.Render(x) + " == 0"
		}
	}
	return ""
}

// requireNonNull rewrites Objects_requireNonNull(A, msg) (withMsg) or
// Objects_requireNonNull_Object(A).
func requireNonNull(c *ast.CallExpr, withMsg bool) string {
	if (withMsg && len(c.Args) != 2) || (!withMsg && len(c.Args) != 1) {
		return ""
	}
	ba, ok := unbox(c.Args[0])
	if !ok {
		return ""
	}
	if b, isBasic := ba.t.(*types.Basic); isBasic && b.Kind() != types.String {
		return ""
	}
	test := nullTest(ba.x, ba.t, ba.keepNull)
	if test == "" {
		return ""
	}
	npe := func() string {
		if withMsg {
			return "panic(new_NullPointerException_String(" + rw.Render(c.Args[1]) + ").self())"
		}
		return "panic(new_NullPointerException_0().self())"
	}
	check := func() string { return "if " + nullTest(ba.x, ba.t, ba.keepNull) + " {\n" + npe() + "\n}" }
	// A statement: the check.
	if es, ok := rw.Up(c).(*ast.ExprStmt); ok {
		rw.Rewrite[es] = check
		return "statement: inline nil check"
	}
	// The value, unboxed to the type of x.
	top := ast.Expr(c)
	if p, ok := rw.Up(c).(*ast.CallExpr); ok && len(p.Args) == 1 && rw.Unparen(p.Args[0]) == c {
		if tv := rw.Info.Types[p]; tv.Type != nil && types.Identical(types.Unalias(tv.Type), ba.t) {
			top = p
		}
	}
	if top == ast.Expr(c) {
		return ""
	}
	if as, ok := rw.Up(top).(*ast.AssignStmt); ok && len(as.Lhs) == 1 && len(as.Rhs) == 1 && rw.Unparen(as.Rhs[0]) == top && as.Tok == token.ASSIGN && rw.Pure(ba.x) {
		if _, isId := rw.Unparen(ba.x).(*ast.Ident); isId {
			if _, inBlock := rw.Parent[as].(*ast.BlockStmt); inBlock {
				lhs := as.Lhs[0]
				x := ba.x
				rw.Rewrite[as] = func() string { return check() + "\n" + rw.Render(lhs) + " = " + rw.Render(x) }
				return "assigned: inline nil check"
			}
		}
	}
	if b, ok := ba.t.Underlying().(*types.Basic); ok && b.Kind() == types.String {
		return ""
	}
	x := ba.x
	rw.Rewrite[top] = func() string {
		if withMsg {
			return "requireNonNull_(" + rw.Render(x) + ", " + rw.Render(c.Args[1]) + ")"
		}
		return "requireNonNull0_(" + rw.Render(x) + ")"
	}
	return "in an expression: requireNonNull_"
}
