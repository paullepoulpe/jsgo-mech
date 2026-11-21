// Command valcls is a post-flip step: a class that is a value (immutable, equal exactly when all
// its fields are, never compared by identity) is a comparable Go struct used by value, and its
// equals and hashCode go.
//
//	type BranchIndexPair struct{ line_, branch int32 }       type BranchIndexPair struct{ line_, branch int32 }
//	func new_BranchIndexPair(l, b int32) *BranchIndexPair  -> func new_BranchIndexPair(l, b int32) BranchIndexPair
//	func (this *BranchIndexPair) getLine() int32              func (this BranchIndexPair) getLine() int32
//	func (this *BranchIndexPair) equals(object any) bool      (deleted, and hashCode)
//	*TMap[*BranchIndexPair, Node] with tch_p_X (hash, eq)     *TMap[BranchIndexPair, Node] with tc_X (by ==)
//
// Java compares such keys with equals and hashCode (a record, or a hand-written pair); the
// translation keeps a pointer per value and hashes it through the class's methods. The hand port
// writes these classes as comparable structs used by value (jsgo: FileInstrumentationData's
// BranchIndexPair, Es6RewriteModulesToCommonJsModules' es6CommonJsModuleRequest, CachingTranspiler
// .Key). The tool converts a class C when it proves:
//
//   - all-field: C's equals asserts its argument to *C (false when it is not one, or null) and
//     returns the conjunction of `this.f == that.f` over every field of C exactly once (a field
//     read directly or through a getter that returns it), each field of a type whose Go == is
//     Java's equals: a non-float number, boolean, string, an integer-based named type (enums,
//     node handles), or a pointer to a class with no equals of its own (identity both ways).
//     Then two values are equal exactly when their structs are ==, and Go's map hashing agrees.
//   - immutable: C embeds no class, no class embeds it, it has no self_; every write of a field
//     is in C's zero_/finit_/init_ methods, which only C's new_C calls, and no field's address is
//     taken.
//   - identity-free and never null: every expression of type *C is a call of a function
//     returning *C (new_C, a factory, a method of C), a local variable defined with one, a
//     parameter, a range key of a typed set or map of *C, or C's receiver; and it is used only
//     as the receiver of a method of C (not equals or hashCode), a key given to a TMap/TSet
//     method, a value stored in a local, passed, or returned. No nil of type *C, no comparison,
//     no conversion to an interface (no box: the value is never seen as an Object), no field of
//     another class, no erased view (asMap/asSet). equals and hashCode are called by nothing but
//     the typed containers' conversion (tch_p_C).
//
// The values a program sees are then the same: a value is never null, equal values are equal
// structs, a container finds and orders keys as before (LinkedHashMap order is insertion order;
// the hash only finds them), and nothing can tell a copy from the original. The conversion
// tch_p_C becomes tc_C (keys compared by ==). Left, counted per class: the first property that
// fails. A rerun finds no candidate (the converted classes have no equals).
//
// usage: valcls [-n] [-v] <dir of package closure>
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

var (
	declOf  = map[*types.Func]*ast.FuncDecl{}
	verbose bool
)

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				if fn, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					declOf[fn] = fd
				}
			}
		}
	}
	embedded := map[*types.Named]bool{}
	scope := rw.Pkg.Scope()
	var cands []*types.Named
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		n, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		st, ok := n.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			if st.Field(i).Embedded() {
				if e := namedOf(st.Field(i).Type()); e != nil {
					embedded[e] = true
				}
			}
		}
		if n.TypeParams().Len() == 0 && ownMethod(n, "equals") != nil && ownMethod(n, "hashCode") != nil {
			cands = append(cands, n)
		}
	}
	left := map[string][]string{}
	var conv []*types.Named
	for _, c := range cands {
		if why := check(c, embedded); why != "" {
			left[why] = append(left[why], c.Obj().Name())
			continue
		}
		conv = append(conv, c)
	}
	for _, c := range conv {
		convert(c)
	}
	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", len(left[k]), k)
		if verbose {
			fmt.Fprintf(os.Stderr, "        %s\n", strings.Join(left[k], " "))
		}
	}
	names := []string{}
	for _, c := range conv {
		names = append(names, c.Obj().Name())
	}
	fmt.Fprintf(os.Stderr, "  %4d classes with equals and hashCode; %d are values: %s\n", len(cands), len(conv), strings.Join(names, " "))
	if report || len(conv) == 0 {
		fmt.Fprintln(os.Stderr, "valcls: nothing written")
		return
	}
	fmt.Fprintf(os.Stderr, "valcls: files changed: %d\n", rw.SaveChecked())
}

func namedOf(t types.Type) *types.Named {
	n, _ := types.Unalias(t).(*types.Named)
	if n == nil || n.Obj().Pkg() != rw.Pkg {
		return nil
	}
	return n.Origin()
}

// ptrTo reports whether t is *c.
func ptrTo(t types.Type, c *types.Named) bool {
	p, ok := types.Unalias(t).(*types.Pointer)
	return ok && namedOf(p.Elem()) == c
}

// ownMethod is the method name declared on *c itself.
func ownMethod(c *types.Named, name string) *types.Func {
	for i := 0; i < c.NumMethods(); i++ {
		if m := c.Method(i); m.Name() == name {
			return m
		}
	}
	return nil
}

// check returns why c is not a value class, or "".
func check(c *types.Named, embedded map[*types.Named]bool) string {
	st := c.Underlying().(*types.Struct)
	if embedded[c] {
		return "a class embeds it (a subclass)"
	}
	fields := map[*types.Var]bool{}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if f.Embedded() {
			return "it embeds a class (a superclass)"
		}
		if f.Name() == "self_" || strings.HasPrefix(f.Name(), "vt_") {
			return "it dispatches through self_ / vt_"
		}
		if !eqIsEquals(f.Type()) {
			return "a field whose Go == is not Java's equals (" + types.TypeString(f.Type(), types.RelativeTo(rw.Pkg)) + ")"
		}
		fields[f] = true
	}
	if len(fields) == 0 {
		return "no fields"
	}
	if why := allFieldEquals(c, fields); why != "" {
		return why
	}
	if why := immutable(c, fields); why != "" {
		return why
	}
	return valueUses(c)
}

// eqIsEquals reports whether Go's == on values of t is Java's equals of the field.
func eqIsEquals(t types.Type) bool {
	switch u := types.Unalias(t).(type) {
	case *types.Basic:
		return u.Info()&(types.IsInteger|types.IsBoolean|types.IsString) != 0
	case *types.Named:
		if b, ok := u.Underlying().(*types.Basic); ok {
			return b.Info()&(types.IsInteger|types.IsBoolean) != 0
		}
	case *types.Pointer:
		n := namedOf(u.Elem())
		if n == nil {
			return false
		}
		if _, ok := n.Underlying().(*types.Struct); !ok {
			return false
		}
		o, _, _ := types.LookupFieldOrMethod(u, true, rw.Pkg, "equals")
		return o == nil
	}
	return false
}

// getters: the methods of c whose body is `return this.f`.
func getters(c *types.Named) map[*types.Func]*types.Var {
	out := map[*types.Func]*types.Var{}
	for i := 0; i < c.NumMethods(); i++ {
		m := c.Method(i)
		fd := declOf[m]
		if fd == nil || len(fd.Body.List) != 1 || fd.Type.Params.NumFields() != 0 {
			continue
		}
		r, ok := fd.Body.List[0].(*ast.ReturnStmt)
		if !ok || len(r.Results) != 1 {
			continue
		}
		if v := fieldOf(r.Results[0], recvOf(fd)); v != nil {
			out[m] = v
		}
	}
	return out
}

func recvOf(fd *ast.FuncDecl) types.Object {
	if fd.Recv == nil || len(fd.Recv.List) != 1 || len(fd.Recv.List[0].Names) != 1 {
		return nil
	}
	return rw.Info.Defs[fd.Recv.List[0].Names[0]]
}

// fieldOf: e is x.f with x the variable v: the field f.
func fieldOf(e ast.Expr, v types.Object) *types.Var {
	s, ok := rw.Unparen(e).(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	id, ok := rw.Unparen(s.X).(*ast.Ident)
	if !ok || v == nil || rw.Info.Uses[id] != v {
		return nil
	}
	sel := rw.Info.Selections[s]
	if sel == nil || sel.Kind() != types.FieldVal || len(sel.Index()) != 1 {
		return nil
	}
	f, _ := sel.Obj().(*types.Var)
	return f
}

// allFieldEquals checks equals's shape: an assertion of the argument to *c, false when it fails
// (or is null), and the conjunction of this.f == that.f over every field once.
func allFieldEquals(c *types.Named, fields map[*types.Var]bool) string {
	fd := declOf[ownMethod(c, "equals")]
	if fd == nil || fd.Type.Params.NumFields() != 1 {
		return "equals is not equals(any)"
	}
	this := recvOf(fd)
	gets := getters(c)
	var that types.Object
	var conj ast.Expr
	body := fd.Body.List
	// form 1: o, ok := p.(*C); if !ok || o == nil { return false }; return <conj>
	// form 2: if that, ok := p.(*C); ok { return <conj> }; return false
	assertion := func(s ast.Stmt) (types.Object, types.Object) {
		as, ok := s.(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 2 || len(as.Rhs) != 1 {
			return nil, nil
		}
		ta, ok := as.Rhs[0].(*ast.TypeAssertExpr)
		if !ok || ta.Type == nil || !ptrTo(rw.Info.Types[ta.Type].Type, c) {
			return nil, nil
		}
		a, _ := as.Lhs[0].(*ast.Ident)
		b, _ := as.Lhs[1].(*ast.Ident)
		if a == nil || b == nil {
			return nil, nil
		}
		return rw.Info.Defs[a], rw.Info.Defs[b]
	}
	isFalse := func(s ast.Stmt) bool {
		r, ok := s.(*ast.ReturnStmt)
		if !ok || len(r.Results) != 1 {
			return false
		}
		id, ok := r.Results[0].(*ast.Ident)
		return ok && id.Name == "false"
	}
	switch {
	case len(body) == 3:
		o, okv := assertion(body[0])
		ifs, isIf := body[1].(*ast.IfStmt)
		r, isR := body[2].(*ast.ReturnStmt)
		if o == nil || !isIf || !isR || len(r.Results) != 1 || ifs.Else != nil || len(ifs.Body.List) != 1 || !isFalse(ifs.Body.List[0]) {
			return "equals is not all fields by =="
		}
		// the condition: !ok || o == nil (or !ok)
		cond := rw.Text(ifs.Cond)
		if cond != "!"+okv.Name() && cond != "!"+okv.Name()+" || "+o.Name()+" == nil" {
			return "equals is not all fields by =="
		}
		that, conj = o, r.Results[0]
	case len(body) == 2:
		ifs, isIf := body[0].(*ast.IfStmt)
		if !isIf || ifs.Init == nil || ifs.Else != nil || len(ifs.Body.List) != 1 || !isFalse(body[1]) {
			return "equals is not all fields by =="
		}
		o, okv := assertion(ifs.Init)
		id, isId := ifs.Cond.(*ast.Ident)
		r, isR := ifs.Body.List[0].(*ast.ReturnStmt)
		if o == nil || !isId || rw.Info.Uses[id] != okv || !isR || len(r.Results) != 1 {
			return "equals is not all fields by =="
		}
		that, conj = o, r.Results[0]
	default:
		return "equals is not all fields by =="
	}
	seen := map[*types.Var]bool{}
	var walk func(e ast.Expr) bool
	read := func(e ast.Expr, v types.Object) *types.Var {
		if f := fieldOf(e, v); f != nil {
			return f
		}
		call, ok := rw.Unparen(e).(*ast.CallExpr)
		if !ok || len(call.Args) != 0 {
			return nil
		}
		s, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		id, ok := rw.Unparen(s.X).(*ast.Ident)
		if !ok || rw.Info.Uses[id] != v {
			return nil
		}
		m, _ := rw.Info.Uses[s.Sel].(*types.Func)
		return gets[m]
	}
	walk = func(e ast.Expr) bool {
		e = rw.Unparen(e)
		if id, ok := e.(*ast.Ident); ok && id.Name == "true" {
			return true
		}
		b, ok := e.(*ast.BinaryExpr)
		if !ok {
			return false
		}
		switch b.Op {
		case token.LAND:
			return walk(b.X) && walk(b.Y)
		case token.EQL:
			f1, f2 := read(b.X, this), read(b.Y, that)
			if f1 == nil {
				f1, f2 = read(b.Y, this), read(b.X, that)
			}
			if f1 == nil || f1 != f2 || seen[f1] {
				return false
			}
			seen[f1] = true
			return true
		}
		return false
	}
	if !walk(conj) || len(seen) != len(fields) {
		return "equals is not all fields by =="
	}
	return ""
}

// lifecycle methods: zero_C, finit_C, init_C.
func isLifecycle(c *types.Named, name string) bool {
	n := c.Obj().Name()
	return name == "zero_"+n || name == "finit_"+n || name == "init_"+n
}

func immutable(c *types.Named, fields map[*types.Var]bool) string {
	newC := rw.Pkg.Scope().Lookup("new_" + c.Obj().Name())
	if newC == nil {
		return "no new_ constructor"
	}
	why := ""
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			if why != "" {
				return false
			}
			switch n := x.(type) {
			case *ast.AssignStmt:
				for _, l := range n.Lhs {
					if s, ok := rw.Unparen(l).(*ast.SelectorExpr); ok {
						if v, ok := rw.Info.Uses[s.Sel].(*types.Var); ok && fields[v] {
							fd := enclosingDecl(n)
							if fd == nil || fd.Recv == nil || !isLifecycle(c, fd.Name.Name) {
								why = "a field is written outside its constructor"
							}
						}
					}
				}
			case *ast.IncDecStmt:
				if s, ok := rw.Unparen(n.X).(*ast.SelectorExpr); ok {
					if v, ok := rw.Info.Uses[s.Sel].(*types.Var); ok && fields[v] {
						why = "a field is written outside its constructor"
					}
				}
			case *ast.UnaryExpr:
				if n.Op == token.AND {
					if s, ok := rw.Unparen(n.X).(*ast.SelectorExpr); ok {
						if v, ok := rw.Info.Uses[s.Sel].(*types.Var); ok && fields[v] {
							why = "a field's address is taken"
						}
					}
				}
			}
			return true
		})
	}
	if why != "" {
		return why
	}
	// The lifecycle methods are called only by new_C.
	for i := 0; i < c.NumMethods(); i++ {
		m := c.Method(i)
		if !isLifecycle(c, m.Name()) {
			continue
		}
		for _, u := range rw.UsesOf[m] {
			fd := enclosingDecl(u)
			if fd == nil || fd.Recv != nil || fd.Name.Name != "new_"+c.Obj().Name() {
				return "its constructor is run by more than new_ (a subclass, a reset)"
			}
		}
	}
	return ""
}

func enclosingDecl(n ast.Node) *ast.FuncDecl {
	for p := ast.Node(n); p != nil; p = rw.Parent[p] {
		if fd, ok := p.(*ast.FuncDecl); ok {
			return fd
		}
	}
	return nil
}

// convVar is the package variable of the typed containers' conversion of *c (tch_p_C), or nil.
func convVars(c *types.Named) []*types.Var {
	var out []*types.Var
	scope := rw.Pkg.Scope()
	for _, name := range scope.Names() {
		v, ok := scope.Lookup(name).(*types.Var)
		if !ok {
			continue
		}
		p, ok := types.Unalias(v.Type()).(*types.Pointer)
		if !ok {
			continue
		}
		n, ok := types.Unalias(p.Elem()).(*types.Named)
		if !ok || n.Obj().Name() != "tconv_" || n.TypeArgs().Len() != 1 || !ptrTo(n.TypeArgs().At(0), c) {
			continue
		}
		out = append(out, v)
	}
	return out
}

func inConvDecl(n ast.Node, convs []*types.Var) bool {
	for p := ast.Node(n); p != nil; p = rw.Parent[p] {
		if vs, ok := p.(*ast.ValueSpec); ok {
			for _, nm := range vs.Names {
				for _, v := range convs {
					if rw.Info.Defs[nm] == v {
						return true
					}
				}
			}
		}
	}
	return false
}

// isTyped container: *TMap[*c, V] / *TSet[*c] (or their values).
func isContainerOf(t types.Type, c *types.Named) bool {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || n.Obj().Pkg() != rw.Pkg || (n.Obj().Name() != "TMap" && n.Obj().Name() != "TSet") {
		return false
	}
	return n.TypeArgs().Len() > 0 && ptrTo(n.TypeArgs().At(0), c)
}

var erasedViews = map[string]bool{"asMap": true, "asSet": true, "keySet": true, "entrySet": true, "values": true}

// valueUses checks every expression of type *c.
func valueUses(c *types.Named) string {
	convs := convVars(c)
	eqm, hm := ownMethod(c, "equals"), ownMethod(c, "hashCode")
	for _, m := range []*types.Func{eqm, hm} {
		for _, u := range rw.UsesOf[m] {
			if !inConvDecl(u, convs) {
				return "equals or hashCode is called directly"
			}
		}
	}
	newName := "new_" + c.Obj().Name()
	why := ""
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			if why != "" {
				return false
			}
			if fd, ok := x.(*ast.FuncDecl); ok {
				fn, _ := rw.Info.Defs[fd.Name].(*types.Func)
				if fn == eqm || fn == hm {
					return false
				}
			}
			if vs, ok := x.(*ast.ValueSpec); ok {
				if inConvDecl(vs, convs) {
					return false
				}
				if len(vs.Values) == 0 && vs.Type != nil && ptrTo(rw.Info.Types[vs.Type].Type, c) {
					why = "a variable of the class declared without a value (null)"
					return false
				}
			}
			// containers of *c: no erased views
			if s, ok := x.(*ast.SelectorExpr); ok && erasedViews[s.Sel.Name] {
				if tv, ok := rw.Info.Types[s.X]; ok && isContainerOf(tv.Type, c) {
					why = "a container of the class is seen through an erased view"
					return false
				}
			}
			e, ok := x.(ast.Expr)
			if !ok {
				return true
			}
			tv, ok := rw.Info.Types[e]
			if !ok || tv.IsType() || !ptrTo(tv.Type, c) {
				return true
			}
			if _, isParen := e.(*ast.ParenExpr); isParen {
				return true
			}
			if w := producer(e, c, newName); w != "" {
				why = w
				return false
			}
			if w := consumer(e, c); w != "" {
				why = w
				return false
			}
			return true
		})
	}
	return why
}

// producer: how a value of *c is made at e.
func producer(e ast.Expr, c *types.Named, newName string) string {
	switch x := e.(type) {
	case *ast.Ident:
		if x.Name == "nil" {
			return "nil of the class (null)"
		}
		o := rw.Info.Uses[x]
		if o == nil {
			o = rw.Info.Defs[x]
		}
		v, ok := o.(*types.Var)
		if !ok {
			return "a value of the class from a non-variable identifier"
		}
		if v.IsField() {
			return "a field of another class holds it"
		}
		if v.Parent() == rw.Pkg.Scope() {
			return "a package variable holds it"
		}
		return ""
	case *ast.CallExpr:
		fn := calledFunc(x)
		if fn == nil {
			return "a value of the class from a call of a func value"
		}
		sig := fn.Type().(*types.Signature)
		if sig.Recv() != nil && namedOf(derefT(sig.Recv().Type())) != c {
			return "a value of the class returned by " + fn.Name() + " (not the class's own)"
		}
		return ""
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			if cl, ok := x.X.(*ast.CompositeLit); ok && len(cl.Elts) == 0 {
				fd := enclosingDecl(x)
				if fd != nil && fd.Recv == nil && fd.Name.Name == newName {
					return ""
				}
			}
		}
	case *ast.SelectorExpr:
		return "a field of another class holds it"
	}
	return fmt.Sprintf("a value of the class from a %T (not a call, local or parameter)", e)
}

func derefT(t types.Type) types.Type {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

func calledFunc(call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch f := rw.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	case *ast.IndexExpr:
		if i, ok := f.X.(*ast.Ident); ok {
			id = i
		}
	default:
		return nil
	}
	fn, _ := rw.Info.Uses[id].(*types.Func)
	if fn == nil {
		return nil
	}
	return fn.Origin()
}

// consumer: e's use is allowed.
func consumer(e ast.Expr, c *types.Named) string {
	p := rw.Up(e)
	switch u := p.(type) {
	case *ast.SelectorExpr:
		if u.X != e && rw.Unparen(u.X) != e {
			return ""
		}
		sel := rw.Info.Selections[u]
		if sel == nil {
			return "a selector on the class: " + rw.Text(u)
		}
		if sel.Kind() == types.FieldVal {
			return ""
		}
		m, _ := sel.Obj().(*types.Func)
		if m == nil || namedOf(derefT(m.Type().(*types.Signature).Recv().Type())) != c {
			return "a method not of the class called on it"
		}
		if m.Name() == "equals" || m.Name() == "hashCode" {
			return "equals or hashCode is called directly"
		}
		// a method value (not called)
		if call, ok := rw.Up(u).(*ast.CallExpr); !ok || call.Fun != ast.Expr(u) {
			return "a method value of the class"
		}
		return ""
	case *ast.CallExpr:
		if u.Fun == e {
			return ""
		}
		fn := calledFunc(u)
		if fn == nil {
			return "passed to " + rw.Text(u.Fun)
		}
		sig := fn.Type().(*types.Signature)
		recvT := types.Type(nil)
		if s, ok := rw.Unparen(u.Fun).(*ast.SelectorExpr); ok {
			if sel := rw.Info.Selections[s]; sel != nil {
				recvT = sel.Recv()
			}
		}
		if recvT != nil && isContainerOf(recvT, c) {
			if erasedViews[fn.Name()] {
				return "a container of the class is seen through an erased view"
			}
			return ""
		}
		// a parameter of type *c
		for i, a := range u.Args {
			if rw.Unparen(a) == e && i < sig.Params().Len() && !sig.Variadic() && ptrTo(sig.Params().At(i).Type(), c) {
				if strings.HasPrefix(fn.Name(), "obj_") || strings.HasPrefix(fn.Name(), "cast_") {
					return "boxed"
				}
				return ""
			}
		}
		return "passed where an Object is taken (" + fn.Name() + ")"
	case *ast.AssignStmt:
		for i, r := range u.Rhs {
			if rw.Unparen(r) == e && i < len(u.Lhs) {
				l := u.Lhs[i]
				if lt, ok := rw.Info.Types[l]; ok && !ptrTo(lt.Type, c) {
					return "stored where an Object is held"
				}
				if id, ok := l.(*ast.Ident); ok {
					o := rw.Info.Defs[id]
					if o == nil {
						o = rw.Info.Uses[id]
					}
					if v, ok := o.(*types.Var); ok && !v.IsField() && v.Parent() != rw.Pkg.Scope() {
						return ""
					}
				}
				return "stored in a field or package variable"
			}
		}
		// a left-hand side
		return ""
	case *ast.ValueSpec:
		if u.Type != nil && ptrTo(rw.Info.Types[u.Type].Type, c) {
			return ""
		}
		return "stored where an Object is held"
	case *ast.ReturnStmt:
		fd := enclosingFuncType(u)
		if fd != nil && fd.Results != nil && len(fd.Results.List) == 1 && ptrTo(rw.Info.Types[fd.Results.List[0].Type].Type, c) {
			return ""
		}
		return "returned as an Object"
	case *ast.RangeStmt:
		return ""
	case *ast.BinaryExpr:
		return "compared (identity)"
	}
	return fmt.Sprintf("used in %T: %s", p, rw.Text(p))
}

func enclosingFuncType(n ast.Node) *ast.FuncType {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch f := p.(type) {
		case *ast.FuncLit:
			return f.Type
		case *ast.FuncDecl:
			return f.Type
		}
	}
	return nil
}

// convert rewrites c into a value class.
func convert(c *types.Named) {
	name := c.Obj().Name()
	eqm, hm := ownMethod(c, "equals"), ownMethod(c, "hashCode")
	for _, m := range []*types.Func{eqm, hm} {
		fd := declOf[m]
		rw.Rewrite[fd] = func() string { return rw.DelDoc }
	}
	convs := convVars(c)
	tcName := "tc_" + name
	for i, v := range convs {
		var spec *ast.ValueSpec
		for _, f := range rw.Files {
			ast.Inspect(f, func(x ast.Node) bool {
				if vs, ok := x.(*ast.ValueSpec); ok && len(vs.Names) == 1 && rw.Info.Defs[vs.Names[0]] == v {
					spec = vs
				}
				return spec == nil
			})
		}
		if i == 0 {
			rw.Rewrite[spec] = func() string {
				return tcName + " = &tconv_[" + name + "]{\n\tbox:   func(v " + name + ") any { return v },\n\tunbox: func(v any) " + name + " { return v.(" + name + ") },\n}"
			}
		} else {
			gd := rw.Parent[spec]
			rw.Rewrite[gd] = func() string { return rw.Del }
		}
		for _, u := range rw.UsesOf[v] {
			if inConvDecl(u, convs) {
				continue
			}
			rw.Rewrite[u] = func() string { return tcName }
		}
	}
	// *C -> C in every type expression except the lifecycle methods' receivers; &C{} -> C{}.
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			switch n := x.(type) {
			case *ast.FuncDecl:
				fn, _ := rw.Info.Defs[n.Name].(*types.Func)
				if fn == eqm || fn == hm {
					return false
				}
				if n.Recv != nil && isLifecycle(c, n.Name.Name) {
					if ptrTo(rw.Info.Types[n.Recv.List[0].Type].Type, c) {
						for _, fld := range []*ast.FieldList{n.Type.Params, n.Type.Results} {
							if fld != nil {
								ast.Inspect(fld, starRewrite(c))
							}
						}
						ast.Inspect(n.Body, starRewrite(c))
						return false
					}
				}
			case *ast.ValueSpec:
				if inConvDecl(n, convs) {
					return false
				}
			case *ast.UnaryExpr:
				if n.Op == token.AND {
					if cl, ok := n.X.(*ast.CompositeLit); ok && namedOf(rw.Info.Types[cl].Type) == c {
						rw.Rewrite[n] = func() string { return rw.Render(cl) }
						return false
					}
				}
			}
			return starRewrite(c)(x)
		})
	}
}

func starRewrite(c *types.Named) func(ast.Node) bool {
	return func(x ast.Node) bool {
		s, ok := x.(*ast.StarExpr)
		if !ok {
			return true
		}
		tv, ok := rw.Info.Types[s]
		if !ok || !tv.IsType() || !ptrTo(tv.Type, c) {
			return true
		}
		name := c.Obj().Name()
		rw.Rewrite[s] = func() string { return name }
		return false
	}
}
