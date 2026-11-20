// Command hashmemo is a post-flip step: a class whose hashCode is a function of data that never
// changes after construction computes it once and keeps it, as java.lang.String keeps its own.
//
// Java caches a String's hash code in the String object; Go's strings have no such place, so the
// translated `name.hashCode()` (utf16Hash_, String_hashCode) walks the bytes on every call. A
// class that hashes a string field (ScopedName: the name of a Var; InputId) therefore pays for
// the string at every lookup of a hash map keyed by it, where Java pays once per String.
//
// Rule. For a class C of the generated code (not generic) that declares hashCode, when
//
//	(a) no subclass of C declares hashCode (nor a hashCode_impl exists in C's hierarchy: the
//	    method is not dispatched), so that C's hashCode is the hashCode of every C;
//	(b) its body is straight-line (declarations and assignments of local variables, one return)
//	    and reads, of `this`, only final fields: fields whose every assignment in the package
//	    (the handwritten runtime included) is in a constructor function (new_X, init_X,
//	    finit_X, zero_X) through `this`, whose address is never taken, and for which no
//	    constructor lets `this` escape before the field has its value (below); and
//	(c) it combines only values whose hash is the same for the object's lifetime: constants,
//	    integers and booleans, strings (immutable), node handles and enum constants and objects
//	    of classes without hashCode (the identity hash: of the handle, of the enum's record, of
//	    the address, which Go's collector never moves: identityHash_, rt_object.go), an
//	    Optional of a string, objects of classes this rule proves (recursively), and the results
//	    of stable getters: a method without parameters called on `this` or on the object of a
//	    final field, whose body is `return <such an expression>`, or the dispatcher of such
//	    methods (`return this.self_.m_impl()`, with the generated shortcuts), every
//	    implementation of which in the object's class and its subclasses is one. The calls
//	    allowed are those getters, the runtime's hash functions listed in pure, conversions
//	    between basic types and arithmetic,
//
// C gets a field `hashMemo_ int32` and its hashCode becomes
//
//	func (this *C) hashCode() int32 {
//		if h := this.hashMemo_; h != 0 {
//			return h
//		}
//		h := this.hashCodeOnce_()   // the body as it was
//		this.hashMemo_ = h
//		return h
//	}
//
// 0 means "not computed", as in java.lang.String: an object whose hash code is 0 computes it
// on every call, as before. The value returned is the value the body returns, and by (a)-(c) the
// body returns the same value at every call on one object, so no caller can tell. The first
// call runs the body unchanged, so a null field throws there what it threw; a body that throws
// stores nothing and throws again at the next call. A nil receiver fails at its first field
// read as it did (the classes written all read a field or call a dispatcher first).
//
// Written only where it can gain: classes of the generated code whose hashed data includes a
// string or an object of a proved class (the hash of an integer, a handle or an address is
// as cheap as the test of the memo). The other proved classes are reported, not changed.
//
// Proof obligations and how the tool discharges them:
//   - final fields: every use of the field object (go/types) that is an assignment target, an
//     inc/dec, a range variable or under & is examined; composite literals make new objects.
//     An assignment of a whole struct of the class's hierarchy (`*p = v`), which would
//     overwrite the fields without naming them, and an == of two such structs by value (the
//     new field would take part) refuse every class of that hierarchy.
//   - no early escape: hashCode could be called on a half-made object only if `this` left the
//     constructor before the field was assigned. Along every new_X of the field's class and its
//     subclasses, every statement executed up to the last assignment of the field (into the
//     constructors called, whole bodies of those that run before it) may use `this` only to
//     read or write its fields and to call constructor functions on it or on its embedded
//     superclass; `this.self_ = this` is allowed. Constructor functions must be called only
//     from constructor functions, on `this`.
//   - the methods are the ones Go calls: a non-dispatcher method call is static in Go, and a
//     dispatcher's implementations are looked up in every class of the receiver's subtree.
//   - not covered by the tool, checked by hand for this runtime: writes through reflection or
//     unsafe (Object_clone copies an object with its memo, and the copy's fields are equal;
//     embeddedPtr_ only walks embedded structs), and concurrency (the package and cmd/mech have
//     no go statement: one goroutine).
//
// The report lists every class with a hashCode: written, proved but not written (and why), or
// the obligation it fails; and for the written ones the size of the struct and of every struct
// that embeds it, before and after (gc/amd64, with the allocator's size class).
//
// A rerun changes nothing (a class with hashMemo_ is reported as memoized).
//
// usage: hashmemo [-n] <dir of package closure>
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

const (
	memoField  = "hashMemo_"
	onceMethod = "hashCodeOnce_"
)

var (
	structDecl = map[*types.Named]*ast.StructType{}
	supers     = map[*types.Named][]*types.Named{}
	subs       = map[*types.Named][]*types.Named{}
	fieldOwner = map[*types.Var]*types.Named{}
	funcDecls  = map[*types.Func]*ast.FuncDecl{}
	methodsOf  = map[*types.Named][]*ast.FuncDecl{}
	newFns     = map[*types.Named][]*ast.FuncDecl{}
	allStructs []*types.Named
	// structs of which a whole value is assigned or compared somewhere, with the place
	structWritten = map[*types.Named]string{}
)

func at(n ast.Node) string {
	p := rw.Fset.Position(n.Pos())
	return fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
}

func generated(pos token.Pos) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(pos).Filename), "g_")
}

func classOf(t types.Type) *types.Named {
	if t == nil {
		return nil
	}
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	n, ok := t.(*types.Named)
	if !ok {
		return nil
	}
	if _, ok := n.Underlying().(*types.Struct); !ok {
		return nil
	}
	return n.Origin()
}

func isCtorName(n string) bool {
	return strings.HasPrefix(n, "new_") || strings.HasPrefix(n, "init_") || strings.HasPrefix(n, "finit_") || strings.HasPrefix(n, "zero_")
}

func subtree(n *types.Named) []*types.Named {
	out := []*types.Named{n}
	for _, s := range subs[n] {
		out = append(out, subtree(s)...)
	}
	return out
}

func ancestors(n *types.Named) []*types.Named {
	var out []*types.Named
	for _, s := range supers[n] {
		out = append(out, s)
		out = append(out, ancestors(s)...)
	}
	return out
}

func hierarchy(n *types.Named) []*types.Named { return append(ancestors(n), subtree(n)...) }

func lookupMethod(n *types.Named, name string) *types.Func {
	o, _, _ := types.LookupFieldOrMethod(types.NewPointer(n), true, rw.Pkg, name)
	f, _ := o.(*types.Func)
	if f != nil {
		f = f.Origin()
	}
	return f
}

func enclosingFunc(n ast.Node) *ast.FuncDecl {
	for n != nil {
		if fd, ok := n.(*ast.FuncDecl); ok {
			return fd
		}
		n = rw.Parent[n]
	}
	return nil
}

func inFuncLit(n ast.Node) bool {
	for n != nil {
		if _, ok := n.(*ast.FuncLit); ok {
			return true
		}
		if _, ok := n.(*ast.FuncDecl); ok {
			return false
		}
		n = rw.Parent[n]
	}
	return false
}

// thisOf is the object `this` of a constructor function: the receiver, or the local variable
// a new_X declares with `this := &X{}`.
func thisOf(fd *ast.FuncDecl) *types.Var {
	if fd.Recv != nil {
		if len(fd.Recv.List) == 1 && len(fd.Recv.List[0].Names) == 1 {
			v, _ := rw.Info.Defs[fd.Recv.List[0].Names[0]].(*types.Var)
			return v
		}
		return nil
	}
	if fd.Body == nil || len(fd.Body.List) == 0 {
		return nil
	}
	as, ok := fd.Body.List[0].(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return nil
	}
	id, ok := as.Lhs[0].(*ast.Ident)
	if !ok || id.Name != "this" {
		return nil
	}
	u, ok := as.Rhs[0].(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return nil
	}
	if _, ok := u.X.(*ast.CompositeLit); !ok {
		return nil
	}
	v, _ := rw.Info.Defs[id].(*types.Var)
	return v
}

// partOfThis: e is `this` or `this.Super.Super...` (embedded structs by value): the object.
func partOfThis(e ast.Expr, this *types.Var) bool {
	e = rw.Unparen(e)
	switch x := e.(type) {
	case *ast.Ident:
		return this != nil && rw.Info.Uses[x] == types.Object(this)
	case *ast.SelectorExpr:
		s, ok := rw.Info.Selections[x]
		if !ok || s.Kind() != types.FieldVal {
			return false
		}
		fv, ok := s.Obj().(*types.Var)
		if !ok || !fv.Embedded() {
			return false
		}
		if _, isPtr := types.Unalias(fv.Type()).(*types.Pointer); isPtr {
			return false
		}
		return partOfThis(x.X, this)
	}
	return false
}

// ---- constructors: `this` does not escape

// stmtSafe reports why statement s of a constructor function lets `this` escape ("" if it does
// not): `this` may be used to read or write a field, and as the receiver (itself or an
// embedded superclass part) of a constructor function.
func stmtSafe(s ast.Stmt, this *types.Var, isNew bool) string {
	why := ""
	ast.Inspect(s, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || why != "" || rw.Info.Uses[id] != types.Object(this) {
			return why == ""
		}
		if inFuncLit(id) {
			why = "this in a function literal (" + at(id) + ")"
			return false
		}
		var cur ast.Expr = id
		for {
			p := rw.Parent[cur]
			if pe, ok := p.(*ast.ParenExpr); ok {
				cur = pe
				continue
			}
			sel, ok := p.(*ast.SelectorExpr)
			if !ok || sel.X != cur {
				break
			}
			sn, ok := rw.Info.Selections[sel]
			if !ok {
				why = "unknown selection on this (" + at(id) + ")"
				return false
			}
			if sn.Kind() == types.MethodVal {
				if c, ok := rw.Parent[sel].(*ast.CallExpr); ok && c.Fun == ast.Expr(sel) && isCtorName(sel.Sel.Name) {
					return true
				}
				why = "this." + sel.Sel.Name + " used in a constructor (" + at(id) + ")"
				return false
			}
			fv := sn.Obj().(*types.Var)
			_, isPtr := types.Unalias(fv.Type()).(*types.Pointer)
			if !fv.Embedded() || isPtr {
				// A field of the object, read or written; its address must not be taken.
				if u, ok := rw.Parent[sel].(*ast.UnaryExpr); ok && u.Op == token.AND {
					why = "address of a field of this taken (" + at(id) + ")"
					return false
				}
				return true
			}
			cur = sel
		}
		// `this` (or an embedded part) as a value.
		switch p := rw.Parent[cur].(type) {
		case *ast.AssignStmt:
			if cur == ast.Expr(id) && len(p.Lhs) == 1 && len(p.Rhs) == 1 && p.Rhs[0] == cur {
				if l, ok := p.Lhs[0].(*ast.SelectorExpr); ok && l.Sel.Name == "self_" && partOfThis(l.X, this) {
					return true
				}
			}
		}
		why = "this used as a value in a constructor (" + at(id) + ")"
		return false
	})
	return why
}

// assigns reports whether statement s assigns field f directly.
func assignsField(s ast.Node, f *types.Var) bool {
	found := false
	ast.Inspect(s, func(n ast.Node) bool {
		if as, ok := n.(*ast.AssignStmt); ok {
			for _, l := range as.Lhs {
				if sel, ok := rw.Unparen(l).(*ast.SelectorExpr); ok && rw.Info.Uses[sel.Sel] == types.Object(f) {
					found = true
				}
			}
		}
		return !found
	})
	return found
}

// ctorCalls are the constructor functions statement s calls on this.
func ctorCalls(s ast.Node) []*ast.FuncDecl {
	var out []*ast.FuncDecl
	ast.Inspect(s, func(n ast.Node) bool {
		c, ok := n.(*ast.CallExpr)
		if !ok {
			return true
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || !isCtorName(sel.Sel.Name) {
			return true
		}
		if sn, ok := rw.Info.Selections[sel]; ok && sn.Kind() == types.MethodVal {
			if fd := funcDecls[sn.Obj().(*types.Func).Origin()]; fd != nil {
				out = append(out, fd)
			}
		}
		return true
	})
	return out
}

type mayKey struct {
	fd *ast.FuncDecl
	f  *types.Var
}

var mayMemo = map[mayKey]int{} // 1 in progress/no, 2 yes

// mayAssign: constructor function fd assigns f, itself or in a constructor it calls.
func mayAssign(fd *ast.FuncDecl, f *types.Var) bool {
	k := mayKey{fd, f}
	if m, ok := mayMemo[k]; ok {
		return m == 2
	}
	mayMemo[k] = 1
	if fd.Body != nil {
		if assignsField(fd.Body, f) {
			mayMemo[k] = 2
			return true
		}
		for _, c := range ctorCalls(fd.Body) {
			if mayAssign(c, f) {
				mayMemo[k] = 2
				return true
			}
		}
	}
	return false
}

type safeKey struct {
	fd   *ast.FuncDecl
	f    *types.Var
	full bool
}

var safeMemo = map[safeKey]*string{}

// requireSafe: the statements of constructor function fd that run before f has its final
// value (all of them when full, else those up to the last one that may assign f) do not let
// `this` escape. It returns the reason they may ("" if none).
func requireSafe(fd *ast.FuncDecl, f *types.Var, full bool) string {
	k := safeKey{fd, f, full}
	if r, ok := safeMemo[k]; ok {
		if r == nil {
			return "" // in progress (recursive constructors): judged by the outer call
		}
		return *r
	}
	safeMemo[k] = nil
	why := ""
	defer func() { safeMemo[k] = &why }()
	if fd.Body == nil {
		return ""
	}
	this := thisOf(fd)
	if this == nil {
		why = fd.Name.Name + " has no `this`"
		return why
	}
	last := len(fd.Body.List) - 1
	if !full {
		last = -1
		for i, s := range fd.Body.List {
			may := assignsField(s, f)
			for _, c := range ctorCalls(s) {
				may = may || mayAssign(c, f)
			}
			if may {
				last = i
			}
		}
	}
	for i := 0; i <= last; i++ {
		s := fd.Body.List[i]
		if i == 0 && fd.Recv == nil {
			continue // this := &X{}
		}
		if r, ok := s.(*ast.ReturnStmt); ok && fd.Recv == nil && i == len(fd.Body.List)-1 && len(r.Results) == 1 {
			if id, ok := r.Results[0].(*ast.Ident); ok && rw.Info.Uses[id] == types.Object(this) {
				continue // return this
			}
		}
		if w := stmtSafe(s, this, fd.Recv == nil); w != "" {
			why = w
			return why
		}
		for _, c := range ctorCalls(s) {
			// A constructor called before the last statement that may assign f runs wholly
			// before f is final; the one in that statement, up to its own last assignment.
			cfull := full || i < last || !mayAssign(c, f)
			if w := requireSafe(c, f, cfull); w != "" {
				why = w
				return why
			}
		}
	}
	return ""
}

// ---- final fields

var finalMemo = map[*types.Var]*string{}

// finalField reports why field f is not final ("" if it is).
func finalField(f *types.Var) string {
	if r, ok := finalMemo[f]; ok {
		if r == nil {
			return ""
		}
		return *r
	}
	finalMemo[f] = nil
	why := finalField1(f)
	finalMemo[f] = &why
	return why
}

func finalField1(f *types.Var) string {
	owner := fieldOwner[f]
	if owner == nil {
		return "field " + f.Name() + " of no known class"
	}
	if owner.TypeParams().Len() > 0 {
		return "field " + f.Name() + " of a generic class"
	}
	for _, h := range hierarchy(owner) {
		if w, ok := structWritten[h]; ok {
			return "a whole " + h.Obj().Name() + " is assigned or compared by value (" + w + ")"
		}
	}
	for _, id := range rw.UsesOf[f] {
		if _, ok := rw.Parent[id].(*ast.KeyValueExpr); ok {
			continue // a composite literal: a new object
		}
		var e ast.Expr = rw.VarExpr(id)
		for {
			if p, ok := rw.Parent[e].(*ast.ParenExpr); ok {
				e = p
				continue
			}
			break
		}
		write := false
		switch p := rw.Parent[e].(type) {
		case *ast.AssignStmt:
			for _, l := range p.Lhs {
				write = write || l == e
			}
		case *ast.IncDecStmt:
			write = true
		case *ast.RangeStmt:
			write = p.Key == e || p.Value == e
		case *ast.UnaryExpr:
			if p.Op == token.AND {
				return "address of " + f.Name() + " taken (" + at(id) + ")"
			}
		}
		if !write {
			continue
		}
		fd := enclosingFunc(id)
		if fd == nil || !isCtorName(fd.Name.Name) || inFuncLit(id) {
			n := "a function literal"
			if fd != nil {
				n = fd.Name.Name
			}
			return f.Name() + " assigned outside a constructor (" + n + ", " + at(id) + ")"
		}
		sel, ok := rw.Unparen(e).(*ast.SelectorExpr)
		if !ok || !partOfThis(sel.X, thisOf(fd)) {
			return f.Name() + " of another object assigned in a constructor (" + at(id) + ")"
		}
	}
	// Constructor functions of the hierarchy are called only by constructor functions, on this.
	for _, h := range hierarchy(owner) {
		for _, m := range methodsOf[h] {
			if !isCtorName(m.Name.Name) {
				continue
			}
			mo, _ := rw.Info.Defs[m.Name].(*types.Func)
			for _, id := range rw.UsesOf[mo] {
				fd := enclosingFunc(id)
				sel, _ := rw.Parent[id].(*ast.SelectorExpr)
				if fd == nil || !isCtorName(fd.Name.Name) || inFuncLit(id) || sel == nil || !partOfThis(sel.X, thisOf(fd)) {
					return "constructor " + m.Name.Name + " called outside a constructor's own object (" + at(id) + ")"
				}
			}
		}
	}
	// `this` does not escape before f is assigned, along every new_X of the classes holding f.
	for _, h := range subtree(owner) {
		for _, nf := range newFns[h] {
			if !mayAssign(nf, f) {
				continue // never assigned on this path: the zero value for good
			}
			if w := requireSafe(nf, f, false); w != "" {
				return "before " + f.Name() + " is assigned: " + w
			}
		}
	}
	return ""
}

// ---- stable expressions

type result struct {
	why    string
	str    bool                  // a string is hashed
	nested map[*types.Named]bool // classes whose hashCode is called: to be proved
	fields map[string]bool       // final fields read (for the report)
}

func (r *result) fail(format string, a ...any) bool {
	if r.why == "" {
		r.why = fmt.Sprintf(format, a...)
	}
	return false
}

type ctx struct {
	this   *types.Var
	cls    *types.Named
	locals map[*types.Var]bool
	r      *result
}

// pure are the runtime's hash functions a hashCode may call, by what they take.
var pure = map[string]string{
	"utf16Hash_": "string", "String_hashCode": "string", "strhash_": "string", "objhashStr_": "string",
	"boolhash_": "prim", "longhash_": "prim", "dblhash_": "prim", "Boolean_hashCode": "prim", "hashOf_": "prim",
	"hashH_": "handle", "hashRH_": "handle", "objhashHandle_": "handle",
	"hash_": "object", "hashR_": "object", "objhashId_": "object", "hashId_": "object", "objhashDyn_": "object", "objhashVia_": "object",
	"objhashBy_": "by", "hashBy_": "by",
	"optHash_": "optional",
}

func hasHashCode(t types.Type) bool {
	o, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, "hashCode")
	_, ok := o.(*types.Func)
	return ok
}

func isString(t types.Type) bool {
	b, ok := types.Unalias(t).Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

// objectHash judges the hash of a value of static type t taken through the Object protocol.
func (c *ctx) objectHash(t types.Type, what string) bool {
	t = types.Unalias(t)
	if isString(t) {
		c.r.str = true
		return true
	}
	if _, ok := t.Underlying().(*types.Interface); ok {
		return c.r.fail("%s: a value of interface type %s (its class is not known)", what, types.TypeString(t, types.RelativeTo(rw.Pkg)))
	}
	if n := classOf(t); n != nil {
		if _, ok := t.(*types.Pointer); !ok {
			return c.r.fail("%s: a struct value", what)
		}
		own := lookupMethod(n, "hashCode")
		if own == nil {
			for _, s := range subtree(n) {
				if lookupMethod(s, "hashCode") != nil {
					return c.r.fail("%s: %s has none and its subclass %s has a hashCode", what, n.Obj().Name(), s.Obj().Name())
				}
			}
			return true // identity: the address
		}
		// the class that declares it; no class below it may (obligation (a) of that class)
		c.r.nested[classOf(own.Type().(*types.Signature).Recv().Type())] = true
		return true
	}
	if nt, ok := t.(*types.Named); ok {
		if _, ok := nt.Underlying().(*types.Basic); ok {
			if hasHashCode(nt) || hasHashCode(types.NewPointer(nt)) {
				return c.r.fail("%s: %s has a hashCode of its own", what, nt.Obj().Name())
			}
			return true // an enum constant or a handle: the identity hash
		}
	}
	return c.r.fail("%s: a value of type %s", what, types.TypeString(t, types.RelativeTo(rw.Pkg)))
}

func (c *ctx) stable(e ast.Expr) bool {
	e = rw.Unparen(e)
	switch x := e.(type) {
	case *ast.BasicLit:
		return true
	case *ast.UnaryExpr:
		if x.Op == token.AND || x.Op == token.ARROW {
			return c.r.fail("operator %s (%s)", x.Op, at(x))
		}
		return c.stable(x.X)
	case *ast.BinaryExpr:
		return c.stable(x.X) && c.stable(x.Y)
	case *ast.Ident:
		switch o := rw.Info.Uses[x].(type) {
		case *types.Const, *types.Nil:
			return true
		case *types.Var:
			if c.locals[o] {
				return true
			}
			if o == c.this {
				return c.r.fail("this used as a value (%s)", at(x))
			}
			return c.r.fail("variable %s (%s)", x.Name, at(x))
		}
		return c.r.fail("identifier %s (%s)", x.Name, at(x))
	case *ast.SelectorExpr:
		sn, ok := rw.Info.Selections[x]
		if !ok || sn.Kind() != types.FieldVal {
			return c.r.fail("selector %s (%s)", rw.Text(x), at(x))
		}
		fv := sn.Obj().(*types.Var).Origin()
		if !partOfThis(x.X, c.this) {
			// a field of the object of a stable expression
			if !c.stable(x.X) {
				return false
			}
		}
		if w := finalField(fv); w != "" {
			return c.r.fail("%s", w)
		}
		c.r.fields[fieldOwner[fv].Obj().Name()+"."+fv.Name()] = true
		return true
	case *ast.CallExpr:
		return c.call(x)
	}
	return c.r.fail("expression %T (%s)", e, at(e))
}

func (c *ctx) call(x *ast.CallExpr) bool {
	if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
		if _, ok := types.Unalias(tv.Type).Underlying().(*types.Basic); ok && len(x.Args) == 1 {
			return c.stable(x.Args[0])
		}
		return c.r.fail("conversion to %s (%s)", rw.Text(x.Fun), at(x))
	}
	name, _ := rw.CallName(x)
	if name == "identityHash_" && len(x.Args) == 1 {
		// the identity hash of this object: of its address
		a := rw.Unparen(x.Args[0])
		if ac, ok := a.(*ast.CallExpr); ok && len(ac.Args) == 0 {
			if s, ok := ac.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "self" {
				a = s.X
			}
		}
		if partOfThis(a, c.this) {
			return true
		}
		return c.r.fail("identityHash_ of something else than this (%s)", at(x))
	}
	if kind, ok := pure[name]; ok {
		if fo, _ := rw.Info.Uses[calleeIdent(x)].(*types.Func); fo == nil || generated(fo.Pos()) {
			return c.r.fail("call of %s, not the runtime's (%s)", name, at(x))
		}
		for i, a := range x.Args {
			if kind == "by" && i == 1 {
				continue
			}
			if !c.stable(a) {
				return false
			}
		}
		if len(x.Args) == 0 {
			return true
		}
		t := rw.Info.TypeOf(x.Args[0])
		switch kind {
		case "string":
			if !isString(t) {
				return c.r.fail("%s of a non-string (%s)", name, at(x))
			}
			c.r.str = true
		case "prim":
			for _, a := range x.Args {
				if _, ok := types.Unalias(rw.Info.TypeOf(a)).Underlying().(*types.Basic); !ok {
					return c.r.fail("%s of a %s (%s)", name, rw.Info.TypeOf(a), at(x))
				}
			}
		case "handle":
			if hasHashCode(t) || hasHashCode(types.NewPointer(t)) {
				return c.r.fail("%s: the handle class has a hashCode of its own (%s)", name, at(x))
			}
		case "object":
			return c.objectHash(t, name+" ("+at(x)+")")
		case "by":
			if len(x.Args) != 2 {
				return c.r.fail("%s (%s)", name, at(x))
			}
			m, ok := rw.Unparen(x.Args[1]).(*ast.SelectorExpr)
			if !ok || m.Sel.Name != "hashCode" {
				return c.r.fail("%s with a function that is no hashCode (%s)", name, at(x))
			}
			n := classOf(rw.Info.TypeOf(x.Args[0]))
			mo, _ := rw.Info.Uses[m.Sel].(*types.Func)
			if n == nil || mo == nil || lookupMethod(n, "hashCode") != mo.Origin() {
				return c.r.fail("%s: not the hashCode of the value's class (%s)", name, at(x))
			}
			c.r.nested[classOf(mo.Type().(*types.Signature).Recv().Type())] = true
		case "optional":
			nt, ok := types.Unalias(t).(*types.Named)
			if !ok || nt.Obj().Name() != "Optional" || nt.TypeArgs().Len() != 1 || !isString(nt.TypeArgs().At(0)) {
				return c.r.fail("optHash_ of an Optional that holds no string (%s)", at(x))
			}
			c.r.str = true
		}
		return true
	}
	sel, ok := rw.Unparen(x.Fun).(*ast.SelectorExpr)
	if !ok {
		return c.r.fail("call of %s (%s)", rw.Text(x.Fun), at(x))
	}
	sn, ok := rw.Info.Selections[sel]
	if !ok || sn.Kind() != types.MethodVal || len(x.Args) != 0 {
		return c.r.fail("call of %s (%s)", rw.Text(x.Fun), at(x))
	}
	f := sn.Obj().(*types.Func).Origin()
	var root *types.Named
	if partOfThis(sel.X, c.this) {
		root = c.cls
	} else {
		if !c.stable(sel.X) {
			return false
		}
		if _, ok := types.Unalias(rw.Info.TypeOf(sel.X)).(*types.Pointer); !ok {
			return c.r.fail("method of a value that is no object (%s)", at(x))
		}
		root = classOf(rw.Info.TypeOf(sel.X))
	}
	if root == nil {
		return c.r.fail("method %s of a value whose class is not known (%s)", f.Name(), at(x))
	}
	if f.Name() == "hashCode" {
		if root == c.cls && partOfThis(sel.X, c.this) {
			return c.r.fail("hashCode of this (%s)", at(x))
		}
		c.r.nested[classOf(f.Type().(*types.Signature).Recv().Type())] = true
		return true
	}
	return c.getter(f, root, at(x))
}

func calleeIdent(x *ast.CallExpr) *ast.Ident {
	switch f := rw.Unparen(x.Fun).(type) {
	case *ast.Ident:
		return f
	case *ast.IndexExpr:
		id, _ := f.X.(*ast.Ident)
		return id
	}
	return nil
}

// dispatcher: fd is `return this.self_.m_impl()` with the generated shortcuts for an object
// that is exactly the class (if this.self_ == any(this) { return this.m_impl() }).
func dispatcher(fd *ast.FuncDecl) bool {
	if fd.Body == nil || fd.Recv == nil || len(fd.Body.List) == 0 {
		return false
	}
	impl := fd.Name.Name + "_impl"
	isRet := func(s ast.Stmt) bool {
		r, ok := s.(*ast.ReturnStmt)
		if !ok || len(r.Results) != 1 {
			return false
		}
		c, ok := r.Results[0].(*ast.CallExpr)
		if !ok || len(c.Args) != 0 {
			return false
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		return ok && sel.Sel.Name == impl
	}
	noCalls := func(n ast.Node) bool {
		ok := true
		ast.Inspect(n, func(m ast.Node) bool {
			if c, isCall := m.(*ast.CallExpr); isCall {
				if id, isId := c.Fun.(*ast.Ident); !isId || id.Name != "any" {
					ok = false
				}
			}
			return ok
		})
		return ok
	}
	for i, s := range fd.Body.List {
		if i == len(fd.Body.List)-1 {
			return isRet(s)
		}
		is, ok := s.(*ast.IfStmt)
		if !ok || is.Else != nil || len(is.Body.List) != 1 || !isRet(is.Body.List[0]) || !noCalls(is.Cond) {
			return false
		}
		if is.Init != nil && !noCalls(is.Init) {
			return false
		}
	}
	return false
}

var getterBusy = map[*types.Func]bool{}

// getter: method f, called on an object whose class is root or one of its subclasses, returns
// a stable expression of that object.
func (c *ctx) getter(f *types.Func, root *types.Named, where string) bool {
	fd := funcDecls[f]
	if fd == nil || fd.Body == nil {
		return c.r.fail("call of %s, which has no body here (%s)", f.Name(), where)
	}
	var impls []*types.Func
	if dispatcher(fd) {
		// self_ is the most derived object for good.
		selfOwner := classOf(f.Type().(*types.Signature).Recv().Type())
		so, _, _ := types.LookupFieldOrMethod(types.NewPointer(selfOwner), true, rw.Pkg, "self_")
		sv, ok := so.(*types.Var)
		if !ok {
			return c.r.fail("dispatcher %s without self_ (%s)", f.Name(), where)
		}
		if w := finalField(sv.Origin()); w != "" {
			return c.r.fail("%s", w)
		}
		seen := map[*types.Func]bool{}
		for _, s := range subtree(root) {
			m := lookupMethod(s, f.Name()+"_impl")
			if m == nil {
				// A class without the implementation (abstract) is no value of self_, whose
				// interface type has the method.
				if it, ok := sv.Type().Underlying().(*types.Interface); ok {
					if o, _, _ := types.LookupFieldOrMethod(it, false, rw.Pkg, f.Name()+"_impl"); o != nil {
						continue
					}
				}
				return c.r.fail("%s has no %s_impl (%s)", s.Obj().Name(), f.Name(), where)
			}
			if !seen[m] {
				seen[m] = true
				impls = append(impls, m)
			}
		}
	} else {
		impls = []*types.Func{f}
	}
	for _, m := range impls {
		md := funcDecls[m]
		if md == nil || md.Body == nil || len(md.Body.List) != 1 {
			return c.r.fail("%s.%s is no plain getter (%s)", recvName(m), m.Name(), where)
		}
		if es, ok := md.Body.List[0].(*ast.ExprStmt); ok {
			if n, _ := rw.CallName(es.X); n == "panic" {
				continue // an abstract method's stub: throws at every call
			}
		}
		r, ok := md.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(r.Results) != 1 {
			return c.r.fail("%s.%s is no plain getter (%s)", recvName(m), m.Name(), where)
		}
		if getterBusy[m] {
			return c.r.fail("%s.%s is recursive (%s)", recvName(m), m.Name(), where)
		}
		getterBusy[m] = true
		sub := &ctx{this: thisOf(md), cls: classOf(m.Type().(*types.Signature).Recv().Type()), locals: map[*types.Var]bool{}, r: c.r}
		// the object is of a class of root's subtree that has this implementation: its
		// subclasses are those of the implementation's class within the subtree
		ok = sub.stable(r.Results[0])
		getterBusy[m] = false
		if !ok {
			return false
		}
	}
	return true
}

func recvName(f *types.Func) string {
	if n := classOf(f.Type().(*types.Signature).Recv().Type()); n != nil {
		return n.Obj().Name()
	}
	return "?"
}

// body judges the body of a hashCode.
func (c *ctx) body(fd *ast.FuncDecl) bool {
	n := len(fd.Body.List)
	for i, s := range fd.Body.List {
		switch x := s.(type) {
		case *ast.DeclStmt:
			gd, ok := x.Decl.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				return c.r.fail("declaration (%s)", at(s))
			}
			for _, sp := range gd.Specs {
				vs := sp.(*ast.ValueSpec)
				for _, v := range vs.Values {
					if !c.stable(v) {
						return false
					}
				}
				for _, id := range vs.Names {
					v, _ := rw.Info.Defs[id].(*types.Var)
					if v == nil {
						return c.r.fail("declaration (%s)", at(s))
					}
					if _, ok := types.Unalias(v.Type()).Underlying().(*types.Basic); !ok {
						if _, isHandle := types.Unalias(v.Type()).(*types.Named); !isHandle {
							return c.r.fail("local variable %s of type %s (%s)", id.Name, v.Type(), at(s))
						}
					}
					c.locals[v] = true
				}
			}
		case *ast.AssignStmt:
			for _, r := range x.Rhs {
				if !c.stable(r) {
					return false
				}
			}
			for _, l := range x.Lhs {
				id, ok := l.(*ast.Ident)
				if !ok {
					return c.r.fail("assignment to %s (%s)", rw.Text(l), at(s))
				}
				v, _ := rw.Info.Defs[id].(*types.Var)
				if v == nil {
					v, _ = rw.Info.Uses[id].(*types.Var)
					if v == nil || !c.locals[v] {
						return c.r.fail("assignment to %s (%s)", id.Name, at(s))
					}
				}
				c.locals[v] = true
			}
		case *ast.ReturnStmt:
			if i != n-1 || len(x.Results) != 1 {
				return c.r.fail("return before the end (%s)", at(s))
			}
			return c.stable(x.Results[0])
		default:
			return c.r.fail("not straight-line: %T (%s)", s, at(s))
		}
	}
	return c.r.fail("no return at the end")
}

// ---- sizes

var gcSizes = types.SizesFor("gc", "amd64")

func layout(t types.Type, extra map[*types.Named]bool) (size, align int64) {
	t = types.Unalias(t)
	if n, ok := t.(*types.Named); ok {
		if st, ok := n.Underlying().(*types.Struct); ok {
			var off, ma int64 = 0, 1
			add := func(s, a int64) {
				off = (off + a - 1) / a * a
				off += s
				ma = max(ma, a)
			}
			for i := 0; i < st.NumFields(); i++ {
				add(layout(st.Field(i).Type(), extra))
			}
			if extra[n.Origin()] {
				add(4, 4)
			}
			return (off + ma - 1) / ma * ma, ma
		}
	}
	if a, ok := t.Underlying().(*types.Array); ok {
		s, al := layout(a.Elem(), extra)
		return s * a.Len(), al
	}
	return gcSizes.Sizeof(t), gcSizes.Alignof(t)
}

var sizeClasses = []int64{8, 16, 24, 32, 48, 64, 80, 96, 112, 128, 144, 160, 176, 192, 208, 224, 240, 256, 288, 320, 352, 384, 416, 448, 480, 512, 576, 640, 704, 768, 896, 1024}

func sizeClass(n int64) int64 {
	for _, c := range sizeClasses {
		if n <= c {
			return c
		}
	}
	return n
}

// ---- main

type cand struct {
	n      *types.Named
	fd     *ast.FuncDecl
	r      *result
	status string
}

func main() {
	dry := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	sc := rw.Pkg.Scope()
	for _, name := range sc.Names() {
		tn, ok := sc.Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		nt, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		st, ok := nt.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		allStructs = append(allStructs, nt)
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			fieldOwner[f] = nt
			if f.Embedded() {
				if b, ok := types.Unalias(f.Type()).(*types.Named); ok {
					if _, ok := b.Underlying().(*types.Struct); ok {
						supers[nt] = append(supers[nt], b.Origin())
						subs[b.Origin()] = append(subs[b.Origin()], nt)
					}
				}
			}
		}
	}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.FuncDecl:
				o, _ := rw.Info.Defs[x.Name].(*types.Func)
				if o == nil {
					continue
				}
				funcDecls[o] = x
				sig := o.Type().(*types.Signature)
				if sig.Recv() != nil {
					if n := classOf(sig.Recv().Type()); n != nil {
						methodsOf[n] = append(methodsOf[n], x)
					}
				} else if strings.HasPrefix(x.Name.Name, "new_") && sig.Results().Len() == 1 {
					if n := classOf(sig.Results().At(0).Type()); n != nil {
						newFns[n] = append(newFns[n], x)
					}
				}
			case *ast.GenDecl:
				for _, sp := range x.Specs {
					if ts, ok := sp.(*ast.TypeSpec); ok {
						if st, ok := ts.Type.(*ast.StructType); ok {
							if nt, ok := rw.Info.Defs[ts.Name].Type().(*types.Named); ok {
								structDecl[nt] = st
							}
						}
					}
				}
			}
		}
		// whole structs assigned or compared by value
		ast.Inspect(f, func(n ast.Node) bool {
			note := func(e ast.Expr, what string) {
				t := rw.Info.TypeOf(e)
				if t == nil {
					return
				}
				if _, isPtr := types.Unalias(t).(*types.Pointer); isPtr {
					return
				}
				if c := classOf(t); c != nil {
					if _, ok := structWritten[c]; !ok {
						structWritten[c] = what + " " + at(e)
					}
				}
			}
			switch x := n.(type) {
			case *ast.AssignStmt:
				if x.Tok != token.DEFINE {
					for _, l := range x.Lhs {
						note(l, "assigned")
					}
				}
			case *ast.BinaryExpr:
				if x.Op == token.EQL || x.Op == token.NEQ {
					note(x.X, "compared")
				}
			}
			return true
		})
	}

	// Every class that declares hashCode.
	var cands []*cand
	byClass := map[*types.Named]*cand{}
	for _, nt := range allStructs {
		for _, m := range methodsOf[nt] {
			if m.Name.Name == "hashCode" && m.Type.Params.NumFields() == 0 {
				c := &cand{n: nt, fd: m, r: &result{nested: map[*types.Named]bool{}, fields: map[string]bool{}}}
				cands = append(cands, c)
				byClass[nt] = c
			}
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].n.Obj().Name() < cands[j].n.Obj().Name() })
	for _, c := range cands {
		nt := c.n
		switch {
		case lookupMethod(nt, memoField) != nil || func() bool {
			o, _, _ := types.LookupFieldOrMethod(types.NewPointer(nt), true, rw.Pkg, memoField)
			return o != nil
		}():
			c.status = "memoized already"
			continue
		case nt.TypeParams().Len() > 0:
			c.r.fail("(a) a generic class")
		case c.fd.Body == nil:
			c.r.fail("no body")
		}
		if c.r.why == "" {
			for _, s := range hierarchy(nt) {
				if s != nt {
					for _, m := range methodsOf[s] {
						if m.Name.Name == "hashCode" && contains(subtree(nt), s) {
							c.r.fail("(a) overridden in subclass %s", s.Obj().Name())
						}
					}
				}
				for _, m := range methodsOf[s] {
					if m.Name.Name == "hashCode_impl" {
						c.r.fail("(a) hashCode is dispatched (%s.hashCode_impl)", s.Obj().Name())
					}
				}
			}
		}
		if c.r.why == "" {
			x := &ctx{this: thisOf(c.fd), cls: nt, locals: map[*types.Var]bool{}, r: c.r}
			if x.this == nil {
				c.r.fail("no receiver name")
			} else {
				x.body(c.fd)
			}
		}
	}
	// Nested classes: proved when every class whose hashCode they call is (no cycles).
	proved := map[*types.Named]bool{}
	for changed := true; changed; {
		changed = false
		for _, c := range cands {
			if c.status != "" || c.r.why != "" || proved[c.n] {
				continue
			}
			ok := true
			for d := range c.r.nested {
				ok = ok && proved[d]
			}
			if ok {
				proved[c.n] = true
				changed = true
			}
		}
	}
	var written []*cand
	count := map[string]int{}
	for _, c := range cands {
		switch {
		case c.status != "":
		case c.r.why != "":
			c.status = "fails: " + c.r.why
		case !proved[c.n]:
			var ds []string
			for d := range c.r.nested {
				if !proved[d] {
					w := "has no hashCode of its own class"
					if dc := byClass[d]; dc != nil {
						w = dc.r.why
						if w == "" {
							w = "depends on unproved classes"
						}
					}
					ds = append(ds, d.Obj().Name()+" ("+w+")")
				}
			}
			sort.Strings(ds)
			c.status = "fails: (c) hashes an object of a class not proved: " + strings.Join(ds, "; ")
		case !generated(c.fd.Pos()):
			c.status = "proved, not written: a class of the runtime"
		case !c.r.str && len(c.r.nested) == 0:
			c.status = "proved, not written: hashes no string and no object of a proved class"
		default:
			for _, s := range hierarchy(c.n) {
				if lookupMethod(s, onceMethod) != nil {
					c.status = "fails: a method " + onceMethod + " exists"
				}
			}
			if c.status == "" {
				c.status = "written"
				written = append(written, c)
			}
		}
		key := c.status
		if i := strings.Index(key, ":"); i >= 0 {
			key = key[:i]
		}
		count[key]++
		var fs []string
		for f := range c.r.fields {
			fs = append(fs, f)
		}
		sort.Strings(fs)
		extra := ""
		if strings.HasPrefix(c.status, "written") || strings.HasPrefix(c.status, "proved") {
			extra = "  [final fields read: " + strings.Join(fs, ", ") + "]"
		}
		fmt.Fprintf(os.Stderr, "%s (%s): %s%s\n", c.n.Obj().Name(), at(c.fd), c.status, extra)
	}
	// Sizes.
	extra := map[*types.Named]bool{}
	for _, c := range written {
		extra[c.n] = true
	}
	none := map[*types.Named]bool{}
	sort.Slice(allStructs, func(i, j int) bool { return allStructs[i].Obj().Name() < allStructs[j].Obj().Name() })
	for _, nt := range allStructs {
		if nt.TypeParams().Len() > 0 {
			continue
		}
		b, _ := layout(nt, none)
		if std := gcSizes.Sizeof(nt); std != b {
			fmt.Fprintf(os.Stderr, "size: %s: own layout %d differs from go/types' %d\n", nt.Obj().Name(), b, std)
		}
		a, _ := layout(nt, extra)
		if a != b {
			what := "holds a memoized class"
			if extra[nt] {
				what = "memoized"
			}
			pad := "grows"
			if sizeClass(a) == sizeClass(b) {
				pad = "same size class"
			}
			fmt.Fprintf(os.Stderr, "size: %s (%s): %d -> %d bytes; allocated %d -> %d (%s)\n", nt.Obj().Name(), what, b, a, sizeClass(b), sizeClass(a), pad)
		} else if extra[nt] {
			fmt.Fprintf(os.Stderr, "size: %s (memoized): %d bytes, unchanged (the field fits in padding)\n", nt.Obj().Name(), b)
		}
	}
	// Edits.
	for _, c := range written {
		c := c
		st := structDecl[c.n]
		rw.Rewrite[st] = func() string {
			s := strings.TrimRight(rw.RenderChildren(st), " \t\n")
			s = strings.TrimSuffix(s, "}")
			if !strings.HasSuffix(s, "\n") {
				s += "\n"
			}
			return s + "\t" + memoField + " int32 // hashCode, once computed, when it is not 0 (post-flip step hashmemo)\n}"
		}
		recv := c.fd.Recv.List[0].Names[0].Name
		name := c.fd.Name
		rw.Rewrite[name] = func() string { return onceMethod }
		fd := c.fd
		rw.Rewrite[fd] = func() string {
			rt := rw.Text(fd.Recv.List[0].Type)
			return fmt.Sprintf("func (%s %s) hashCode() int32 {\n\tif h := %s.%s; h != 0 {\n\t\treturn h\n\t}\n\th := %s.%s()\n\t%s.%s = h\n\treturn h\n}\n\n// %s computes hashCode: a function of data that is final in the object (post-flip step hashmemo).\n%s",
				recv, rt, recv, memoField, recv, onceMethod, recv, memoField, onceMethod, rw.RenderChildren(fd))
		}
	}
	var keys []string
	for k := range count {
		keys = append(keys, fmt.Sprintf("%s %d", k, count[k]))
	}
	sort.Strings(keys)
	fmt.Fprintf(os.Stderr, "hashmemo: %d classes declare hashCode: %s\n", len(cands), strings.Join(keys, ", "))
	if dry {
		return
	}
	fmt.Fprintf(os.Stderr, "hashmemo: %d files written\n", rw.SaveChecked())
}

func contains(l []*types.Named, n *types.Named) bool {
	for _, x := range l {
		if x == n {
			return true
		}
	}
	return false
}
