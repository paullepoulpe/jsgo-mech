// Command castfold is a post-flip step: a cast whose result the operand's static type already
// decides is not a cast.
//
// j2go writes every Java reference cast as a call of a helper on a value boxed into any:
// cast_[*X](v) (a class nothing extends), to_X(v) (a class with subclasses: an assertion to
// interface{ as_X() *X }) and icast_I(v) (a Java interface I). Erasure makes most of them
// necessary, but after the typing steps many take a value whose static type is already the
// target, or one that converts to it without a check:
//
//	cast_[*X](obj_(y))        y of type *X          ->  y
//	to_X(y.self())            y of type *X          ->  y
//	icast_I(obj_(y))          y of interface type J ->  I(y)   (J's methods include I's)
//	icast_I(any(y))           y of type *C          ->  I(y)   (*C implements I)
//	icast_I(obj_(this))                             ->  I(this)
//	to_X(obj_(this))          this a *Y, Y embeds X ->  this.as_X()
//
// The operand is unwrapped from obj_(y), any(y), y.self() or the implicit conversion of y to
// any. Each rewrite gives the same value and the same failure as the cast:
//   - identity (the static type of y is the target type T): cast_[T] and to_X return the boxed
//     value as a T, which is y (obj_ boxes a nil y as nil, which casts back to the zero T, y
//     again; any(y) of a nil pointer is a non-nil any holding it, which asserts back to it).
//     y.self() is the most derived object, the one holding y (every constructor sets self_ to
//     it before anything else runs; the step checks that every composite literal of a class
//     with self_ is followed by that assignment): to_X of it is its as_X(), which is y. A cast_[*X]
//     of y.self() is folded only for a class X nothing extends, whose objects are their own self_.
//   - a Java interface I that the operand implements statically: the assertion v.(I) of a
//     boxed y gives the interface value I(y) (the same dynamic type and pointer). They differ
//     only for nil: obj_ boxes a nil pointer as nil, and icast_I gives a nil I, while I(y) of a
//     nil pointer is a non-nil I holding it. So a pointer operand under obj_ (or under self(),
//     for a class nothing extends, or for the object a constructor has just made, which is
//     its own self_) is folded only when it cannot be nil: the receiver this, a composite
//     literal's address, the result of a function every return of which is such a value
//     (constructors), or a local or package variable defined once from one and never assigned.
//     An interface operand is nil exactly when I(y) is; any(y) and the implicit conversion
//     keep a nil pointer non-nil, as I(y) does.
//   - an upcast to_X of a *Y whose class extends X: y.as_X(), which is what to_X calls on it.
//     For a nil y to_X(obj_(y)) is nil while y.as_X() panics, so y must not be nil; any(y) and
//     the implicit conversion make to_X call as_X on the nil pointer too, so they always fold.
//
// The erased Preconditions.checkNotNull between the boxing and the cast,
// cast_[*X](Preconditions_checkNotNull(obj_(y))), is its typed runtime form,
// Preconditions_checkNotNullT(y) (the same NullPointerException and message when y is nil, else
// y), when the boxing keeps nil as nil (obj_, self(), an interface operand); it also proves the
// pointer non-nil for the conversions above. The step uses it only once the runtime has it.
//
// Where the context takes exactly the type I (an argument for a parameter of type I, the
// right side of an assignment to a variable of type I, a returned result of type I), I(y) is
// written y. Casts nested in the operand are folded too. What is left takes a value whose
// static type does not decide the result (from an erased slot, a downcast, a cast between Java
// interfaces) or a pointer that may be nil; -n prints them by reason.
//
// A folded cast no longer matches, so running the step again changes nothing.
//
// With -instof, an instanceof that guards an if statement is a comma-ok type assertion and the
// casts it guards use the asserted value; see instof.go.
//
// With -assert, a cast that cannot see null, or whose value is dereferenced at once, is a Go type
// assertion; see assert.go.
//
// usage: castfold [-n] [-v] [-instof | -assert] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

func main() {
	dry, verbose, instofMode, assertMode := false, false, false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		switch a {
		case "-n":
			dry = true
		case "-v":
			verbose = true
		case "-instof":
			instofMode = true
		case "-assert":
			assertMode = true
		}
	}
	rw.Load(os.Args[len(os.Args)-1])
	if instofMode {
		instof(dry)
		return
	}
	for id, o := range rw.Info.Defs {
		if o != nil {
			defOf[o] = id
		}
	}
	checkSelfInvariant()
	if assertMode {
		asserts(dry, verbose)
		return
	}

	folded := map[string]int{}
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
			if kind == "" {
				return true
			}
			gen, why := fold(c, kind, target, fn)
			if gen == nil {
				left[kind+": "+why]++
				if verbose && len(examples[kind+": "+why]) < 5 {
					p := rw.Fset.Position(c.Pos())
					t := rw.Text(c)
					if len(t) > 160 {
						t = t[:160]
					}
					examples[kind+": "+why] = append(examples[kind+": "+why], fmt.Sprintf("%s:%d %s", filepath.Base(p.Filename), p.Line, t))
				}
				return true
			}
			folded[kind+": "+why]++
			rw.Rewrite[c] = gen
			return true
		})
	}
	print := func(title string, m map[string]int) {
		keys := sortedKeys(m)
		total := 0
		for _, n := range m {
			total += n
		}
		fmt.Fprintf(os.Stderr, "castfold: %d %s\n", total, title)
		for _, k := range keys {
			fmt.Fprintf(os.Stderr, "  %5d %s\n", m[k], k)
			for _, e := range examples[k] {
				fmt.Fprintf(os.Stderr, "          %s\n", e)
			}
		}
	}
	print("casts folded", folded)
	print("casts left", left)
	if dry {
		return
	}
	fmt.Fprintf(os.Stderr, "castfold: %d files written\n", rw.SaveChecked())
}

func sortedKeys(m map[string]int) []string {
	var keys []string
	for k := range m {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool { return m[keys[i]] > m[keys[j]] || m[keys[i]] == m[keys[j]] && keys[i] < keys[j] })
	return keys
}

func generated(n ast.Node) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "g_")
}

// castOf returns the kind of cast c is ("to", "cast", "icast", "is", "is_") and its target type.
func castOf(c *ast.CallExpr) (string, types.Type) {
	switch f := c.Fun.(type) {
	case *ast.Ident:
		o, ok := rw.Info.Uses[f].(*types.Func)
		if !ok || o.Pkg() != rw.Pkg {
			return "", nil
		}
		sig := o.Type().(*types.Signature)
		if sig.Params().Len() != 1 || !isAny(sig.Params().At(0).Type()) {
			return "", nil
		}
		res := sig.Results()
		switch {
		case strings.HasPrefix(f.Name, "to_") && res.Len() == 1:
			pt, ok := res.At(0).Type().(*types.Pointer)
			if !ok {
				return "", nil
			}
			if nt, ok := pt.Elem().(*types.Named); !ok || "to_"+nt.Obj().Name() != f.Name {
				return "", nil
			}
			return "to", pt
		case strings.HasPrefix(f.Name, "icast_") && res.Len() == 1:
			nt, ok := res.At(0).Type().(*types.Named)
			if !ok || !types.IsInterface(nt) || "icast_"+nt.Obj().Name() != f.Name {
				return "", nil
			}
			return "icast", nt
		case strings.HasPrefix(f.Name, "is_"):
			tn, _ := rw.Pkg.Scope().Lookup(f.Name[3:]).(*types.TypeName)
			if tn == nil {
				return "", nil
			}
			return "is", types.NewPointer(tn.Type())
		}
	case *ast.IndexExpr:
		id, ok := f.X.(*ast.Ident)
		if !ok {
			return "", nil
		}
		o, ok := rw.Info.Uses[id].(*types.Func)
		if !ok || o.Pkg() != rw.Pkg {
			return "", nil
		}
		switch id.Name {
		case "cast_":
			return "cast", rw.Info.TypeOf(f.Index)
		case "is_":
			return "is_", rw.Info.TypeOf(f.Index)
		}
	}
	return "", nil
}

func isAny(t types.Type) bool {
	it, ok := t.Underlying().(*types.Interface)
	return ok && it.Empty()
}

// unwrap returns how the operand of a cast was boxed ("obj_", "any", "self" or "-" for the
// implicit conversion) and the expression boxed.
func unwrap(e ast.Expr) (string, ast.Expr) {
	e = rw.Unparen(e)
	if call, ok := e.(*ast.CallExpr); ok && len(call.Args) <= 1 {
		fun := call.Fun
		if ix, ok := fun.(*ast.IndexExpr); ok {
			fun = ix.X
		}
		if id, ok := fun.(*ast.Ident); ok && len(call.Args) == 1 && (id.Name == "obj_" || id.Name == "any") {
			if o := rw.Info.Uses[id]; o == types.Universe.Lookup("any") || o != nil && o.Pkg() == rw.Pkg && o.Parent() == rw.Pkg.Scope() {
				return id.Name, rw.Unparen(call.Args[0])
			}
		}
		if s, ok := fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && s.Sel.Name == "self" {
			if sel := rw.Info.Selections[s]; sel != nil && sel.Kind() == types.MethodVal && selfMethod(sel.Obj().(*types.Func)) {
				return "self", rw.Unparen(s.X)
			}
		}
	}
	return "-", e
}

// fold returns the rewrite of cast c, or nil and the reason it is left.
func fold(c *ast.CallExpr, kind string, target types.Type, fn *ast.FuncDecl) (func() string, string) {
	if kind == "is" || kind == "is_" {
		return nil, "instanceof (not folded by this step)"
	}
	if target == nil {
		return nil, "unknown target"
	}
	arg := c.Args[0]
	nn, nnName := checkNotNullCall(arg)
	if nn != nil {
		arg = nn.Args[0]
	}
	wrap, y := unwrap(arg)
	ty := types.Unalias(rw.Info.TypeOf(y))
	if ty == nil {
		return nil, "untyped operand"
	}
	target = types.Unalias(target)
	if isAny(ty) {
		return nil, "operand of type any: " + anySource(y)
	}
	if nn != nil && (wrap == "any" || wrap == "-") && !types.IsInterface(ty) {
		return nil, "checkNotNull of a pointer boxed non-nil (any(p) of a nil p passes the check)"
	}
	render := func() string {
		if nn == nil {
			return rw.Render(y)
		}
		var sb strings.Builder
		sb.WriteString(nnName + "(" + rw.Render(y))
		for _, a := range nn.Args[1:] {
			sb.WriteString(", " + rw.Render(a))
		}
		sb.WriteString(")")
		return sb.String()
	}
	paren := func() string {
		s := render()
		if nn != nil {
			return s
		}
		switch rw.Unparen(y).(type) {
		case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr, *ast.CompositeLit:
			return s
		}
		return "(" + s + ")"
	}
	_, ptr := ty.(*types.Pointer)
	// identity
	if types.Identical(ty, target) {
		if wrap == "self" {
			x := classOf(ty)
			if x == nil || !selfOK[x] {
				return nil, "self() of a class whose self_ is not set by its constructor"
			}
			if kind == "cast" && extended[x] {
				return nil, "cast_ of self() of an extended class"
			}
		}
		if nn != nil {
			return render, "checkNotNull of an operand already of the target type (" + wrap + ")"
		}
		return render, "operand already of the target type (" + wrap + ")"
	}
	switch kind {
	case "icast":
		if !types.AssignableTo(ty, target) {
			if types.IsInterface(ty) {
				return nil, "cast between Java interfaces (checked at run time)"
			}
			return nil, "downcast or cross cast of a class to an interface it does not implement statically"
		}
		conv := func() string { return convert(c, target, paren) }
		switch {
		case types.IsInterface(ty):
			if wrap == "self" {
				return nil, "self() of an interface value"
			}
			return conv, "interface operand that implements the target (" + wrap + ")"
		case !ptr:
			return nil, "operand neither pointer nor interface"
		case wrap == "any" || wrap == "-":
			return conv, "pointer that implements the target, boxed non-nil (" + wrap + ")"
		case wrap == "self":
			x := classOf(ty)
			if x == nil || !selfOK[x] || extended[x] && !exactNew(y, fn, 0) {
				return nil, "self() of an extended class (the most derived object, not the operand)"
			}
			fallthrough
		default:
			if nn != nil {
				return conv, "checkNotNull of a pointer that implements the target (" + wrap + ")"
			}
			if !nonNil(y, fn, 0) {
				return nil, "pointer that may be nil (obj_ boxes a nil pointer as nil; an interface holding a nil pointer is not nil)"
			}
			return conv, "non-nil pointer that implements the target (" + wrap + ")"
		}
	case "to", "cast":
		pt, ok := target.(*types.Pointer)
		if !ok {
			return nil, "target not a pointer"
		}
		x, ok := types.Unalias(pt.Elem()).(*types.Named)
		if !ok {
			return nil, "target not a named pointer"
		}
		if kind == "cast" || !ptr || !rw.HasMethod(ty, "as_"+x.Obj().Name()) {
			if types.IsInterface(ty) {
				return nil, "downcast of an interface value"
			}
			return nil, "downcast (Java narrowing cast, checked at run time)"
		}
		up := func() string { return paren() + ".as_" + x.Obj().Name() + "()" }
		switch wrap {
		case "any", "-":
			return up, "upcast (" + wrap + ")"
		case "self":
			if cx := classOf(ty); cx == nil || !selfOK[cx] {
				return nil, "self() of a class whose self_ is not set by its constructor"
			}
		}
		if nn != nil {
			return up, "upcast of checkNotNull of a pointer (" + wrap + ")"
		}
		if !nonNil(y, fn, 0) {
			return nil, "upcast of a pointer that may be nil"
		}
		return up, "upcast of a non-nil pointer (" + wrap + ")"
	}
	return nil, "?"
}

// anySource names where an operand of type any comes from.
func anySource(y ast.Expr) string {
	switch e := rw.Unparen(y).(type) {
	case *ast.SelectorExpr:
		if e.Sel.Name == "v" && strings.HasPrefix(rw.Text(e.X), "it_") {
			return "element of an erased collection in a for-each"
		}
		return "field of type any"
	case *ast.Ident:
		v, ok := rw.Info.Uses[e].(*types.Var)
		if !ok {
			break
		}
		if v.Parent() == rw.Pkg.Scope() {
			return "package variable of type any"
		}
		def := defOf[v]
		for p := rw.Parent[def]; p != nil; p = rw.Parent[p] {
			switch q := p.(type) {
			case *ast.FieldList:
				if _, ok := rw.Parent[q].(*ast.FuncType); !ok {
					continue
				}
				if _, ok := rw.Parent[rw.Parent[q]].(*ast.FuncLit); ok {
					return "parameter of a func literal (a lambda of an erased functional interface)"
				}
				return "parameter of type any (Java Object or an erased type parameter)"
			case *ast.AssignStmt, *ast.ValueSpec, *ast.RangeStmt:
				if as, ok := q.(*ast.AssignStmt); ok && len(as.Rhs) == 1 {
					if n, _ := rw.CallName(as.Rhs[0]); n == "catch_" {
						return "caught exception (catch_)"
					}
				}
				return "local of type any (an erased value)"
			}
		}
	case *ast.CallExpr:
		if n, _ := rw.CallName(e); n != "" {
			if strings.HasPrefix(n, "Preconditions_checkNotNull") {
				return "checkNotNull of an erased value"
			}
			return "result of an erased generic function or one returning Object"
		}
		return "result of an erased generic method or one returning Object"
	case *ast.IndexExpr:
		return "element of a []any or map of any"
	case *ast.TypeAssertExpr:
		return "type assertion"
	}
	return "other"
}

// checkNotNull are Guava's Preconditions.checkNotNull, erased, and their typed forms in the
// runtime: Preconditions_checkNotNullT(v T) T panics with the same NullPointerException (and
// message) when v is nil and returns v.
var checkNotNull = map[string]string{
	"Preconditions_checkNotNull":                             "Preconditions_checkNotNullT",
	"Preconditions_checkNotNull_Object_Object":               "Preconditions_checkNotNullT_Object",
	"Preconditions_checkNotNull_Object_String_Object":        "Preconditions_checkNotNullT_String_Object",
	"Preconditions_checkNotNull_Object_String_Object_Object": "Preconditions_checkNotNullT_String_Object_Object",
}

// checkNotNullCall returns e if it is a call of an erased checkNotNull, and the name of its typed
// form.
func checkNotNullCall(e ast.Expr) (*ast.CallExpr, string) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) == 0 {
		return nil, ""
	}
	id, ok := c.Fun.(*ast.Ident)
	if !ok {
		return nil, ""
	}
	typed, ok := checkNotNull[id.Name]
	if !ok || !rw.HasFunc(typed) {
		return nil, ""
	}
	if o := rw.Info.Uses[id]; o == nil || o.Pkg() != rw.Pkg {
		return nil, ""
	}
	return c, typed
}

// convert writes the conversion of the operand to interface type t, or the operand alone where
// the context takes exactly t.
func convert(c *ast.CallExpr, t types.Type, operand func() string) string {
	if expects(c, t) {
		return operand()
	}
	return types.TypeString(t, func(p *types.Package) string {
		if p == rw.Pkg {
			return ""
		}
		return p.Name()
	}) + "(" + operand() + ")"
}

// expects reports whether expression e is in a place that converts it to exactly type t.
func expects(e ast.Expr, t types.Type) bool {
	var want types.Type
	switch p := rw.Up(e).(type) {
	case *ast.CallExpr:
		ft := rw.Info.TypeOf(p.Fun)
		if ft == nil {
			return false
		}
		sig, ok := ft.Underlying().(*types.Signature)
		if !ok || rw.Info.Types[p.Fun].IsType() || generic(p.Fun) {
			return false
		}
		for i, a := range p.Args {
			if rw.Unparen(a) == e {
				if sig.Variadic() && i >= sig.Params().Len()-1 {
					return false
				}
				want = sig.Params().At(i).Type()
			}
		}
	case *ast.AssignStmt:
		if len(p.Lhs) != len(p.Rhs) || p.Tok == token.DEFINE {
			return false
		}
		for i, r := range p.Rhs {
			if rw.Unparen(r) == e {
				want = rw.Info.TypeOf(p.Lhs[i])
			}
		}
	case *ast.ValueSpec:
		if p.Type == nil {
			return false
		}
		want = rw.Info.TypeOf(p.Type)
	case *ast.ReturnStmt:
		var sig *types.Signature
		for q := rw.Parent[p]; q != nil; q = rw.Parent[q] {
			if fl, ok := q.(*ast.FuncLit); ok {
				sig, _ = rw.Info.TypeOf(fl).(*types.Signature)
				break
			}
			if fd, ok := q.(*ast.FuncDecl); ok {
				sig = rw.Info.Defs[fd.Name].Type().(*types.Signature)
				break
			}
		}
		if sig == nil || sig.Results().Len() != len(p.Results) {
			return false
		}
		for i, r := range p.Results {
			if rw.Unparen(r) == e {
				want = sig.Results().At(i).Type()
			}
		}
	}
	return want != nil && types.Identical(want, t)
}

// generic reports whether fun names a generic function, whose type arguments may be inferred
// from the argument types.
func generic(fun ast.Expr) bool {
	switch f := rw.Unparen(fun).(type) {
	case *ast.IndexExpr, *ast.IndexListExpr:
		return true
	case *ast.Ident:
		o, ok := rw.Info.Uses[f].(*types.Func)
		return ok && o.Type().(*types.Signature).TypeParams().Len() > 0
	case *ast.SelectorExpr:
		o, ok := rw.Info.Uses[f.Sel].(*types.Func)
		return ok && o.Type().(*types.Signature).TypeParams().Len() > 0
	}
	return false
}

// classOf is the named type a pointer type points to.
func classOf(t types.Type) *types.Named {
	if pt, ok := types.Unalias(t).(*types.Pointer); ok {
		if nt, ok := types.Unalias(pt.Elem()).(*types.Named); ok {
			return nt
		}
	}
	return nil
}

// selfMethod reports whether f is a generated `self() any` that returns this.self_.
func selfMethod(f *types.Func) bool {
	d := funcDecl(f)
	if d == nil || d.Body == nil || len(d.Body.List) != 2 {
		return false
	}
	return rw.Text(d.Body.List[1]) == "return this.self_"
}

var declOf map[*types.Func]*ast.FuncDecl

func funcDecl(f *types.Func) *ast.FuncDecl {
	if declOf == nil {
		declOf = map[*types.Func]*ast.FuncDecl{}
		for _, file := range rw.Files {
			for _, d := range file.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok {
					if o, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
						declOf[o] = fd
					}
				}
			}
		}
	}
	return declOf[f]
}

var (
	// selfOK: every composite literal of the class, or of a class embedding it, is followed
	// by this.self_ = this.
	selfOK = map[*types.Named]bool{}
	// extended: some class embeds the class.
	extended = map[*types.Named]bool{}
)

func checkSelfInvariant() {
	var named []*types.Named
	for _, n := range rw.Pkg.Scope().Names() {
		if tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName); ok && !tn.IsAlias() {
			if nt, ok := tn.Type().(*types.Named); ok && nt.TypeParams().Len() == 0 {
				if _, ok := nt.Underlying().(*types.Struct); ok {
					named = append(named, nt)
				}
			}
		}
	}
	// embedded: for each class, the classes it embeds, transitively.
	embeds := func(nt *types.Named) []*types.Named {
		var out []*types.Named
		var walk func(t *types.Named)
		walk = func(t *types.Named) {
			st := t.Underlying().(*types.Struct)
			for i := 0; i < st.NumFields(); i++ {
				f := st.Field(i)
				if !f.Embedded() {
					continue
				}
				if e, ok := f.Type().(*types.Named); ok {
					if _, ok := e.Underlying().(*types.Struct); ok {
						out = append(out, e)
						walk(e)
					}
				}
			}
		}
		walk(nt)
		return out
	}
	bad := map[*types.Named]bool{}
	for _, nt := range named {
		selfOK[nt] = true
		for _, e := range embeds(nt) {
			extended[e] = true
		}
	}
	hasSelf := func(t types.Type) bool {
		o, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, "self_")
		v, ok := o.(*types.Var)
		return ok && v.IsField()
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			nt, ok := rw.Info.TypeOf(lit).(*types.Named)
			if !ok || !hasSelf(types.NewPointer(nt)) {
				return true
			}
			// this := &C{} followed by this.self_ = this
			ok = false
			if u, isU := rw.Parent[lit].(*ast.UnaryExpr); isU {
				if as, isA := rw.Parent[u].(*ast.AssignStmt); isA && len(as.Lhs) == 1 {
					if blk, isB := rw.Parent[as].(*ast.BlockStmt); isB {
						for i, s := range blk.List {
							if s == as && i+1 < len(blk.List) {
								v := rw.Text(as.Lhs[0])
								ok = rw.Text(blk.List[i+1]) == v+".self_ = "+v
							}
						}
					}
				}
			}
			if !ok {
				bad[nt] = true
			}
			return true
		})
	}
	for nt := range bad {
		selfOK[nt] = false
		for _, e := range embeds(nt) {
			selfOK[e] = false
		}
		fmt.Fprintf(os.Stderr, "castfold: %s is made without setting self_: self() of it and its superclasses is not folded\n", nt.Obj().Name())
	}
}

// nonNil reports whether pointer expression e cannot be nil.
func nonNil(e ast.Expr, fn *ast.FuncDecl, depth int) bool {
	switch x := rw.Unparen(e).(type) {
	case *ast.UnaryExpr:
		_, ok := rw.Unparen(x.X).(*ast.CompositeLit)
		return x.Op == token.AND && ok
	case *ast.Ident:
		v, ok := rw.Info.Uses[x].(*types.Var)
		if !ok {
			return false
		}
		if fn != nil && fn.Recv != nil && len(fn.Recv.List[0].Names) == 1 && rw.Info.Defs[fn.Recv.List[0].Names[0]] == v {
			return !assigned(v)
		}
		if v.Parent() == rw.Pkg.Scope() {
			return globalNonNil(v, depth)
		}
		return localNonNil(v, fn, depth)
	case *ast.CallExpr:
		id, ok := x.Fun.(*ast.Ident)
		if !ok {
			return false
		}
		f, ok := rw.Info.Uses[id].(*types.Func)
		if !ok {
			return false
		}
		return returnsNonNil(f, depth)
	}
	return false
}

// exactNew reports whether e is a new object of exactly its static class, which is its own self_:
// a call of a constructor `this := &C{...}; this.self_ = this; ...; return this` (with no other
// return and no other assignment of self_ or this), or a local defined once from one.
func exactNew(e ast.Expr, fn *ast.FuncDecl, depth int) bool {
	if depth > 8 {
		return false
	}
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		v, ok := rw.Info.Uses[x].(*types.Var)
		if !ok || v.Parent() == rw.Pkg.Scope() || v.IsField() || assigned(v) || defOf[v] == nil {
			return false
		}
		switch p := rw.Parent[defOf[v]].(type) {
		case *ast.AssignStmt:
			return p.Tok == token.DEFINE && len(p.Lhs) == 1 && len(p.Rhs) == 1 && exactNew(p.Rhs[0], fn, depth+1)
		case *ast.ValueSpec:
			return len(p.Names) == 1 && len(p.Values) == 1 && types.Identical(rw.Info.TypeOf(p.Values[0]), v.Type()) && exactNew(p.Values[0], fn, depth+1)
		}
	case *ast.CallExpr:
		id, ok := x.Fun.(*ast.Ident)
		if !ok {
			return false
		}
		f, ok := rw.Info.Uses[id].(*types.Func)
		if !ok {
			return false
		}
		d := funcDecl(f)
		if d == nil || d.Body == nil || len(d.Body.List) < 3 {
			return false
		}
		l := d.Body.List
		as, ok := l[0].(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || rw.Text(as.Lhs[0]) != "this" {
			return false
		}
		u, ok := as.Rhs[0].(*ast.UnaryExpr)
		if !ok || u.Op != token.AND {
			return false
		}
		if _, ok := u.X.(*ast.CompositeLit); !ok || !types.Identical(rw.Info.TypeOf(u), rw.Info.TypeOf(x)) {
			return false
		}
		if rw.Text(l[1]) != "this.self_ = this" || rw.Text(l[len(l)-1]) != "return this" {
			return false
		}
		if assigned(rw.Info.Defs[as.Lhs[0].(*ast.Ident)].(*types.Var)) {
			return false
		}
		ok = true
		ast.Inspect(d.Body, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.ReturnStmt:
				if s != l[len(l)-1] {
					ok = false
				}
			case *ast.AssignStmt:
				if s != l[1] {
					for _, lh := range s.Lhs {
						if se, isSel := lh.(*ast.SelectorExpr); isSel && se.Sel.Name == "self_" {
							ok = false
						}
					}
				}
			case *ast.FuncLit:
				return false
			}
			return ok
		})
		return ok
	}
	return false
}

// assigned reports whether variable v is assigned or has its address taken anywhere but its
// declaration.
func assigned(v *types.Var) bool {
	for _, u := range rw.UsesOf[v] {
		switch p := rw.Up(u).(type) {
		case *ast.AssignStmt:
			for _, l := range p.Lhs {
				if rw.Unparen(l) == u {
					return true
				}
			}
		case *ast.UnaryExpr:
			if p.Op == token.AND {
				return true
			}
		case *ast.IncDecStmt:
			return true
		}
	}
	return false
}

var globalMemo = map[*types.Var]int{} // 1 non-nil, 2 not, 3 in progress

func globalNonNil(v *types.Var, depth int) bool {
	if m := globalMemo[v]; m != 0 {
		return m == 1
	}
	globalMemo[v] = 3
	ok := false
	if !assigned(v) {
		for _, f := range rw.Files {
			for _, d := range f.Decls {
				gd, isG := d.(*ast.GenDecl)
				if !isG || gd.Tok != token.VAR {
					continue
				}
				for _, s := range gd.Specs {
					vs := s.(*ast.ValueSpec)
					for i, n := range vs.Names {
						if rw.Info.Defs[n] == v && len(vs.Values) == len(vs.Names) {
							ok = depth < 8 && nonNil(vs.Values[i], nil, depth+1)
						}
					}
				}
			}
		}
	}
	if ok {
		globalMemo[v] = 1
	} else {
		globalMemo[v] = 2
	}
	return ok
}

var funcMemo = map[*types.Func]int{}

// returnsNonNil reports whether every return of function f gives a non-nil pointer: a
// composite literal's address, a local assigned only from one before, or a call of such a
// function.
func returnsNonNil(f *types.Func, depth int) bool {
	if m := funcMemo[f]; m != 0 {
		return m == 1
	}
	funcMemo[f] = 3
	ok := false
	if d := funcDecl(f); d != nil && d.Body != nil && depth < 8 && f.Type().(*types.Signature).Results().Len() == 1 {
		ok = true
		ast.Inspect(d.Body, func(n ast.Node) bool {
			switch s := n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.ReturnStmt:
				if len(s.Results) != 1 || !nonNil(s.Results[0], d, depth+1) {
					ok = false
				}
			}
			return ok
		})
		if len(d.Body.List) == 0 {
			ok = false
		}
		if _, isRet := d.Body.List[len(d.Body.List)-1].(*ast.ReturnStmt); !isRet {
			ok = false
		}
	}
	if ok {
		funcMemo[f] = 1
	} else {
		funcMemo[f] = 2
	}
	return ok
}

// localNonNil reports whether local variable v is defined once from a non-nil value (v := e or
// var v T = e) and never assigned again.
func localNonNil(v *types.Var, d *ast.FuncDecl, depth int) bool {
	if depth > 8 || v.Parent() == rw.Pkg.Scope() || v.IsField() || assigned(v) {
		return false
	}
	def := defOf[v]
	if def == nil {
		return false
	}
	switch p := rw.Parent[def].(type) {
	case *ast.AssignStmt:
		if p.Tok == token.DEFINE && len(p.Lhs) == 1 && len(p.Rhs) == 1 {
			return nonNil(p.Rhs[0], d, depth+1)
		}
	case *ast.ValueSpec:
		if len(p.Names) == 1 && len(p.Values) == 1 {
			return nonNil(p.Values[0], d, depth+1)
		}
	}
	return false
}

var defOf = map[types.Object]*ast.Ident{}
