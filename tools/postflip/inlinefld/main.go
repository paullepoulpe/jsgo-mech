// Command inlinefld is a post-flip step: a final field of a class that holds a typed collection
// (*TList[T], *TMap[K, V], *TSet[K] of the runtime, rt_tlist.go and rt_tmap.go) made by its
// field initializer is kept in the object by value, so constructing the object no longer
// allocates the collection's header (a list's slice is still allocated as before).
//
//	type C struct {                              type C struct {
//		f *TList[T]                                  f TList[T]
//	}                                            }
//	func (this *C) finit_C() {                   func (this *C) finit_C() {
//		this.f = newTListN_(tc, 1)        ->         initTListN_(&this.f, tc, 1)
//	}                                            }
//	... g(x.f) ... x.f.add(v) ...                ... g(&x.f) ... x.f.add(v) ...
//
// Rule: a field of a struct declared in generated code whose type is *TList[T], *TMap[K, V] or
// *TSet[K], assigned exactly once in the whole package: in C's finit_C (the Java field
// initializer), as a statement of its body (not under a condition or a loop), `this.f = newX(args)`
// with newX one of newTList_, newTListN_, newTMap_, newTMapN_, newTSet_, newTSetN_. Its type
// loses the `*`; that statement becomes initX(&this.f, args) (the runtime's in-place twin of
// newX: the same fields, the same capacity; TestInPlaceCollections); every other use x.f that
// is a value becomes &x.f, and a use that selects from it (x.f.add(v), x.f.items) is left as
// it is (Go takes the address of the addressable field). The collection's identity is the
// field's address, which is the object's for as long as the object lives; the object is
// reachable while the collection is (an interior pointer keeps it).
//
// Proof obligation (that every read sees the same collection as before), checked here; a
// field or a class that fails is refused with its reason and left as it was:
//
//   - The field is assigned once (above) and its address is never taken: every other use is a
//     selection x.f through a pointer (Selection.Indirect: x.f is addressable and x.f, &x.f
//     name the object's field), in generated code, not a key of a composite literal, not
//     compared (== or !=, nil included: after finit_ the field was never nil, and a test would
//     now be always false even before it; such a field is refused, not its test rewritten),
//     not a switch tag, and not read in a zero_ method (which runs before the constructor).
//   - The field is read only after finit_C has run: finit_C is called only by C's constructors
//     init_C* (methods of *C), once each, as a statement of the body; no such constructor
//     calls another (a this(...) delegation runs no initializer in Java); the constructors are
//     called only by new_* functions and init_* methods (a subclass's super(...)); what such a
//     constructor does before finit_C does not let the object escape: each statement there
//     either does not mention the receiver, or assigns a field of it from an expression that
//     does not, or calls a method of it (through embedded fields) declared in generated code
//     whose whole body passes this same test (the superclass's init_ chain: no virtual call
//     can reach a method that reads the field before its initializer).
//   - The object is made only by its constructor: no composite literal or new(T) of a struct
//     type that holds C by value (C, a class embedding it, an array of them) other than an
//     empty &T{} in a new_* function; before, a literal elsewhere would have held nil.
//   - The object stays where it was made: no value of such a type is copied, compared,
//     passed, returned, assigned, kept in a variable or used as a type argument (as htabself's
//     check: only selection, & and the literal above); Object_clone (a reflect copy) is called
//     only on objects of a known class that holds no such field.
//
// Not checked here (hand argument): that the new_* functions and the subclass constructors
// follow j2go's pattern (new_S makes &S{}, sets self_ and the vt fields, calls zero_S and then
// one constructor init_S, which calls its superclass's constructor first): the translator writes
// every constructor so. The runtime holds no generated object by value and copies none through
// unsafe or reflect except Object_clone (checked above).
//
// usage: inlinefld [-n] [-v] <dir of package closure>
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

// the constructors of the typed collections and their in-place twins (the runtime patch)
var inPlace = map[string]string{
	"newTList_": "initTList_", "newTListN_": "initTListN_",
	"newTMap_": "initTMap_", "newTMapN_": "initTMapN_",
	"newTSet_": "initTSet_", "newTSetN_": "initTSetN_",
}

type cand struct {
	fv     *types.Var
	field  *ast.Field
	owner  *types.Named
	assign *ast.AssignStmt
	reads  []*ast.SelectorExpr // values: become &x.f
	refuse string
}

func main() {
	report, verbose := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	dir := os.Args[len(os.Args)-1]
	rw.Load(dir)
	pos := func(n ast.Node) string { return rw.Fset.Position(n.Pos()).String() }
	generated := func(n ast.Node) bool {
		return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "g_")
	}
	fail := func(format string, a ...any) {
		fmt.Fprintf(os.Stderr, "inlinefld: "+format+"\n", a...)
		os.Exit(1)
	}
	scope := rw.Pkg.Scope()
	collType := map[types.Object]bool{}
	for _, n := range []string{"TList", "TMap", "TSet"} {
		if o, ok := scope.Lookup(n).(*types.TypeName); ok {
			collType[o] = true
		}
	}
	ctor := map[types.Object]string{}
	for n := range inPlace {
		if o, ok := scope.Lookup(n).(*types.Func); ok {
			ctor[o] = n
		}
	}
	for _, n := range inPlace {
		if _, ok := scope.Lookup(n).(*types.Func); !ok && !report {
			fail("the runtime has no %s (patch reflip6-inlinefld-rt.patch): nothing written", n)
		}
	}
	isColl := func(t types.Type) bool {
		p, ok := types.Unalias(t).(*types.Pointer)
		if !ok {
			return false
		}
		n, ok := types.Unalias(p.Elem()).(*types.Named)
		return ok && collType[n.Origin().Obj()]
	}

	// The struct types of generated code, and the functions by object.
	decl := map[*types.Named]*ast.TypeSpec{}
	funcs := map[*types.Func]*ast.FuncDecl{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				if d.Tok != token.TYPE {
					continue
				}
				for _, sp := range d.Specs {
					ts := sp.(*ast.TypeSpec)
					if n, ok := rw.Info.Defs[ts.Name].Type().(*types.Named); ok {
						decl[n] = ts
					}
				}
			case *ast.FuncDecl:
				if fn, ok := rw.Info.Defs[d.Name].(*types.Func); ok {
					funcs[fn] = d
				}
			}
		}
	}
	enclosing := func(n ast.Node) *ast.FuncDecl {
		for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
			if fd, ok := p.(*ast.FuncDecl); ok {
				return fd
			}
		}
		return nil
	}
	recvNamed := func(fd *ast.FuncDecl) *types.Named {
		if fd == nil || fd.Recv == nil {
			return nil
		}
		fn := rw.Info.Defs[fd.Name].(*types.Func)
		if p, ok := types.Unalias(fn.Type().(*types.Signature).Recv().Type()).(*types.Pointer); ok {
			if n, ok := types.Unalias(p.Elem()).(*types.Named); ok {
				return n.Origin()
			}
		}
		return nil
	}
	recvObj := func(fd *ast.FuncDecl) types.Object {
		if fd.Recv == nil || len(fd.Recv.List) != 1 || len(fd.Recv.List[0].Names) != 1 {
			return nil
		}
		return rw.Info.Defs[fd.Recv.List[0].Names[0]]
	}

	// The candidates: fields of a collection pointer type in a generated struct.
	var cands []*cand
	already := 0
	byField := map[*ast.Field][]*cand{}
	for n, ts := range decl {
		st, ok := ts.Type.(*ast.StructType)
		if !ok || !generated(ts) {
			continue
		}
		for _, fl := range st.Fields.List {
			for _, nm := range fl.Names {
				fv := rw.Info.Defs[nm].(*types.Var)
				if n, ok := types.Unalias(fv.Type()).(*types.Named); ok && collType[n.Origin().Obj()] {
					already++ // kept by value (a rerun)
				}
				if !isColl(fv.Type()) {
					continue
				}
				c := &cand{fv: fv, field: fl, owner: n}
				cands = append(cands, c)
				byField[fl] = append(byField[fl], c)
			}
		}
	}
	sort.Slice(cands, func(i, j int) bool { return cands[i].fv.Pos() < cands[j].fv.Pos() })

	// Per field: its uses.
	for _, c := range cands {
		finitName := "finit_" + c.owner.Obj().Name()
		for _, id := range rw.UsesOf[c.fv] {
			if c.refuse != "" {
				break
			}
			if !generated(id) {
				c.refuse = "used outside generated code"
				break
			}
			sel, ok := rw.Parent[id].(*ast.SelectorExpr)
			if !ok || sel.Sel != id {
				c.refuse = "set in a composite literal"
				break
			}
			if s := rw.Info.Selections[sel]; s == nil || !s.Indirect() {
				c.refuse = "read through a value, not a pointer"
				break
			}
			if fd := enclosing(sel); fd != nil && strings.HasPrefix(fd.Name.Name, "zero_") {
				c.refuse = "read in a zero_ method"
				break
			}
			switch p := rw.Up(sel).(type) {
			case *ast.AssignStmt:
				lhs := false
				for _, l := range p.Lhs {
					lhs = lhs || rw.Unparen(l) == sel
				}
				if !lhs {
					c.reads = append(c.reads, sel)
					continue
				}
				if c.assign != nil {
					c.refuse = "assigned more than once"
					continue
				}
				fd := enclosing(p)
				name, call := "", (*ast.CallExpr)(nil)
				if len(p.Rhs) == 1 {
					call, _ = rw.Unparen(p.Rhs[0]).(*ast.CallExpr)
				}
				if call != nil {
					fun := call.Fun
					switch x := fun.(type) {
					case *ast.IndexExpr:
						fun = x.X
					case *ast.IndexListExpr:
						fun = x.X
					}
					if fid, ok := fun.(*ast.Ident); ok {
						name = ctor[rw.Info.Uses[fid]]
					}
				}
				x, _ := rw.Unparen(sel.X).(*ast.Ident)
				switch {
				case p.Tok != token.ASSIGN || len(p.Lhs) != 1:
					c.refuse = "assigned other than by a single ="
				case fd == nil || fd.Name.Name != finitName || recvNamed(fd) != c.owner:
					c.refuse = "assigned outside its class's finit_"
				case rw.Parent[p] != fd.Body:
					c.refuse = "assigned under a condition or in a loop of finit_"
				case x == nil || rw.Info.Uses[x] != recvObj(fd):
					c.refuse = "assigned in finit_ to another object's field"
				case name == "":
					c.refuse = "initialized other than by a typed collection's constructor"
				default:
					c.assign = p
				}
			case *ast.UnaryExpr:
				if p.Op == token.AND {
					c.refuse = "its address is taken"
				} else {
					c.reads = append(c.reads, sel)
				}
			case *ast.BinaryExpr:
				if p.Op == token.EQL || p.Op == token.NEQ {
					other := p.X
					if rw.Unparen(other) == sel {
						other = p.Y
					}
					if id, ok := rw.Unparen(other).(*ast.Ident); ok && id.Name == "nil" {
						c.refuse = "tested for nil"
					} else {
						c.refuse = "compared with another value"
					}
				} else {
					c.reads = append(c.reads, sel)
				}
			case *ast.SwitchStmt:
				c.refuse = "a switch tag"
			case *ast.SelectorExpr:
				if rw.Unparen(p.X) != sel {
					c.reads = append(c.reads, sel)
				} // else x.f.m or x.f.g: Go takes the field's address itself
			default:
				c.reads = append(c.reads, sel)
			}
		}
		if c.refuse == "" && c.assign == nil {
			c.refuse = "never assigned"
		}
	}

	// Per class: the obligations on its objects.
	owners := map[*types.Named]bool{}
	for _, c := range cands {
		if c.refuse == "" {
			owners[c.owner] = true
		}
	}
	classRefuse := map[*types.Named]string{}
	refuseClass := func(n *types.Named, why string) {
		if classRefuse[n] == "" {
			classRefuse[n] = why
		}
	}

	// Does a type hold an owner by value, and which?
	memo := map[types.Type][]*types.Named{}
	var held func(t types.Type, seen map[types.Type]bool) []*types.Named
	held = func(t types.Type, seen map[types.Type]bool) []*types.Named {
		t = types.Unalias(t)
		if r, ok := memo[t]; ok {
			return r
		}
		if seen[t] {
			return nil
		}
		seen[t] = true
		var r []*types.Named
		switch u := t.(type) {
		case *types.Named:
			if owners[u.Origin()] {
				r = append(r, u.Origin())
			}
			r = append(r, held(u.Underlying(), seen)...)
		case *types.Struct:
			for i := 0; i < u.NumFields(); i++ {
				r = append(r, held(u.Field(i).Type(), seen)...)
			}
		case *types.Array:
			r = held(u.Elem(), seen)
		}
		memo[t] = r
		return r
	}
	holds := func(t types.Type) []*types.Named {
		if t == nil {
			return nil
		}
		return held(t, map[types.Type]bool{})
	}
	for e, tv := range rw.Info.Types {
		if !tv.IsValue() {
			continue
		}
		hs := holds(tv.Type)
		if len(hs) == 0 {
			continue
		}
		why := ""
		if cl, isLit := rw.Unparen(e).(*ast.CompositeLit); isLit {
			u, _ := rw.Up(e).(*ast.UnaryExpr)
			fd := enclosing(e)
			if u == nil || u.Op != token.AND || len(cl.Elts) != 0 || fd == nil || fd.Recv != nil || !strings.HasPrefix(fd.Name.Name, "new_") {
				why = "an object made other than by &T{} in a new_ function (" + pos(e) + ")"
			}
		} else {
			switch p := rw.Up(e).(type) {
			case *ast.SelectorExpr:
				if rw.Unparen(p.X) != rw.Unparen(e) {
					why = "a value copied (" + pos(e) + ")"
				}
			case *ast.UnaryExpr:
				if p.Op != token.AND {
					why = "a value copied (" + pos(e) + ")"
				}
			default:
				why = "a value copied, compared or passed (" + pos(e) + ")"
			}
		}
		if why != "" {
			for _, h := range hs {
				refuseClass(h, why)
			}
		}
	}
	for id, obj := range rw.Info.Defs {
		if v, ok := obj.(*types.Var); ok && !v.IsField() {
			for _, h := range holds(v.Type()) {
				refuseClass(h, "a variable holds an object by value ("+pos(id)+")")
			}
		}
	}
	for id, inst := range rw.Info.Instances {
		for i := 0; i < inst.TypeArgs.Len(); i++ {
			for _, h := range holds(inst.TypeArgs.At(i)) {
				refuseClass(h, "a type argument ("+pos(id)+")")
			}
		}
	}
	// new(T)
	if newFn := types.Universe.Lookup("new"); newFn != nil {
		for _, f := range rw.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok || len(c.Args) != 1 {
					return true
				}
				if id, ok := c.Fun.(*ast.Ident); ok && rw.Info.Uses[id] == newFn {
					for _, h := range holds(rw.Info.TypeOf(c.Args[0])) {
						refuseClass(h, "made by new(T) ("+pos(c)+")")
					}
				}
				return true
			})
		}
	}
	// Object_clone: only of a known class holding none.
	if cl, ok := scope.Lookup("Object_clone").(*types.Func); ok {
		for _, id := range rw.UsesOf[cl] {
			c, ok := rw.Up(id).(*ast.CallExpr)
			var t types.Type
			if ok && len(c.Args) == 1 {
				a := rw.Unparen(c.Args[0])
				if oc, ok := a.(*ast.CallExpr); ok && len(oc.Args) == 1 {
					if name, _ := rw.CallName(oc); name == "obj_" || strings.HasPrefix(name, "obj_[") {
						a = oc.Args[0]
					}
				}
				t = rw.Info.TypeOf(a)
			}
			p, isPtr := types.Unalias(t).(*types.Pointer)
			if t == nil || !isPtr {
				for n := range owners {
					refuseClass(n, "Object_clone of an object of unknown class ("+pos(id)+")")
				}
				continue
			}
			for _, h := range holds(p.Elem()) {
				refuseClass(h, "copied by Object_clone ("+pos(id)+")")
			}
		}
	}

	// The constructors: finit_C runs before anything can read the field.
	mentions := func(n ast.Node, o types.Object) bool {
		found := false
		ast.Inspect(n, func(x ast.Node) bool {
			if id, ok := x.(*ast.Ident); ok && rw.Info.Uses[id] == o {
				found = true
			}
			return !found
		})
		return found
	}
	// rooted reports whether e is the receiver o or a chain of field selections from it.
	var rooted func(e ast.Expr, o types.Object) bool
	rooted = func(e ast.Expr, o types.Object) bool {
		switch x := rw.Unparen(e).(type) {
		case *ast.Ident:
			return rw.Info.Uses[x] == o
		case *ast.SelectorExpr:
			s := rw.Info.Selections[x]
			return s != nil && s.Kind() == types.FieldVal && rooted(x.X, o)
		}
		return false
	}
	safeMemo := map[*ast.FuncDecl]int{} // 1 in progress, 2 safe, 3 not
	var safeStmts func(stmts []ast.Stmt, o types.Object) bool
	safeBody := func(fd *ast.FuncDecl) bool {
		switch safeMemo[fd] {
		case 1, 3:
			return false
		case 2:
			return true
		}
		safeMemo[fd] = 1
		o := recvObj(fd)
		ok := o != nil && fd.Body != nil && generated(fd) && safeStmts(fd.Body.List, o)
		safeMemo[fd] = 3
		if ok {
			safeMemo[fd] = 2
		}
		return ok
	}
	safeStmts = func(stmts []ast.Stmt, o types.Object) bool {
		for _, s := range stmts {
			if !mentions(s, o) {
				continue
			}
			switch s := s.(type) {
			case *ast.AssignStmt:
				if s.Tok != token.ASSIGN || len(s.Lhs) != 1 || len(s.Rhs) != 1 || mentions(s.Rhs[0], o) {
					return false
				}
				l, ok := rw.Unparen(s.Lhs[0]).(*ast.SelectorExpr)
				if !ok || !rooted(l, o) {
					return false
				}
			case *ast.ExprStmt:
				c, ok := rw.Unparen(s.X).(*ast.CallExpr)
				if !ok {
					return false
				}
				fs, ok := c.Fun.(*ast.SelectorExpr)
				if !ok || !rooted(fs.X, o) {
					return false
				}
				sl := rw.Info.Selections[fs]
				if sl == nil || sl.Kind() != types.MethodVal {
					return false
				}
				fn, _ := sl.Obj().(*types.Func)
				if fn == nil {
					return false
				}
				for _, a := range c.Args {
					if mentions(a, o) {
						return false
					}
				}
				fd := funcs[fn.Origin()]
				if fd == nil || !safeBody(fd) {
					return false
				}
			default:
				return false
			}
		}
		return true
	}
	for n := range owners {
		name := n.Obj().Name()
		var finit *types.Func
		for fn, fd := range funcs {
			if fd.Name.Name == "finit_"+name && recvNamed(fd) == n {
				finit = fn
			}
		}
		if finit == nil {
			refuseClass(n, "no finit_ method")
			continue
		}
		inits := map[*ast.FuncDecl]bool{}
		for _, id := range rw.UsesOf[finit] {
			fd := enclosing(id)
			sel, _ := rw.Parent[id].(*ast.SelectorExpr)
			var call *ast.CallExpr
			if sel != nil {
				call, _ = rw.Up(sel).(*ast.CallExpr)
			}
			var st *ast.ExprStmt
			if call != nil {
				st, _ = rw.Up(call).(*ast.ExprStmt)
			}
			if fd == nil || !strings.HasPrefix(fd.Name.Name, "init_"+name) || recvNamed(fd) != n || st == nil || rw.Parent[st] != fd.Body {
				refuseClass(n, "finit_ called other than as a statement of a constructor of its class ("+pos(id)+")")
				break
			}
			if x, ok := rw.Unparen(sel.X).(*ast.Ident); !ok || rw.Info.Uses[x] != recvObj(fd) {
				refuseClass(n, "finit_ called on another object ("+pos(id)+")")
				break
			}
			if inits[fd] {
				refuseClass(n, "finit_ called twice in a constructor ("+pos(id)+")")
				break
			}
			inits[fd] = true
			var before []ast.Stmt
			for _, s := range fd.Body.List {
				if s == ast.Stmt(st) {
					break
				}
				before = append(before, s)
			}
			if !safeStmts(before, recvObj(fd)) {
				refuseClass(n, "a constructor lets the object escape before finit_ ("+pos(fd)+")")
				break
			}
		}
		if classRefuse[n] != "" {
			continue
		}
		if len(inits) == 0 {
			refuseClass(n, "finit_ is never called")
			continue
		}
		for fd := range inits {
			fn := rw.Info.Defs[fd.Name].(*types.Func)
			for _, id := range rw.UsesOf[fn] {
				caller := enclosing(id)
				if caller == nil {
					refuseClass(n, "a constructor used outside a function ("+pos(id)+")")
					break
				}
				if inits[caller] {
					refuseClass(n, "a constructor delegates to another that runs finit_ ("+pos(id)+")")
					break
				}
				if !(caller.Recv == nil && strings.HasPrefix(caller.Name.Name, "new_")) && !(caller.Recv != nil && strings.HasPrefix(caller.Name.Name, "init_")) {
					refuseClass(n, "a constructor called outside construction ("+pos(id)+")")
					break
				}
			}
		}
	}

	// A field declared beside a refused one keeps the declaration's type.
	for _, c := range cands {
		if c.refuse == "" && classRefuse[c.owner] != "" {
			c.refuse = "its class: " + classRefuse[c.owner]
		}
	}
	for _, cs := range byField {
		for _, c := range cs {
			if c.refuse != "" {
				for _, d := range cs {
					if d.refuse == "" {
						d.refuse = "declared beside a refused field"
					}
				}
			}
		}
	}

	// Report.
	refused := map[string][]string{}
	var ok []*cand
	for _, c := range cands {
		label := c.owner.Obj().Name() + "." + c.fv.Name()
		if c.refuse != "" {
			key := c.refuse
			if i := strings.Index(key, " ("); i >= 0 {
				key = key[:i]
			}
			refused[key] = append(refused[key], label+": "+c.refuse)
			continue
		}
		ok = append(ok, c)
	}
	var keys []string
	for k := range refused {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  refused: %4d %s\n", len(refused[k]), k)
		if verbose {
			sort.Strings(refused[k])
			for _, p := range refused[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	nreads := 0
	for _, c := range ok {
		nreads += len(c.reads)
		fmt.Fprintf(os.Stderr, "  %s.%s %s: %d reads as values\n", c.owner.Obj().Name(), c.fv.Name(), types.TypeString(c.fv.Type(), types.RelativeTo(rw.Pkg)), len(c.reads))
	}
	classes := map[*types.Named]bool{}
	for _, c := range ok {
		classes[c.owner] = true
	}
	fmt.Fprintf(os.Stderr, "inlinefld: %d collection fields, %d kept by value (%d classes, %d reads as values), %d refused, %d already by value\n", len(cands), len(ok), len(classes), nreads, len(cands)-len(ok), already)
	if report || len(ok) == 0 {
		return
	}

	// Rewrite.
	for _, c := range ok {
		star := c.field.Type.(*ast.StarExpr)
		rw.Rewrite[star] = func() string { return rw.Render(star.X) }
		a := c.assign
		call := rw.Unparen(a.Rhs[0]).(*ast.CallExpr)
		lhs := a.Lhs[0]
		rw.Rewrite[a] = func() string {
			fun := rw.RenderChildren(call.Fun)
			base := fun
			if i := strings.IndexByte(fun, '['); i >= 0 {
				base = fun[:i]
			}
			fun = inPlace[base] + fun[len(base):]
			args := []string{"&" + rw.Render(lhs)}
			for _, x := range call.Args {
				args = append(args, rw.Render(x))
			}
			return fun + "(" + strings.Join(args, ", ") + ")"
		}
		for _, r := range c.reads {
			r := r
			rw.Rewrite[r] = func() string { return "&" + rw.RenderChildren(r) }
		}
	}
	fmt.Fprintf(os.Stderr, "inlinefld: files changed: %d\n", rw.SaveChecked())
}
