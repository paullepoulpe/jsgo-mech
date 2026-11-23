// The field proof of optstr (tools/postflip/optstr/proof.go, its "fields" part; cha.go is its
// call graph, unchanged; both as of 8b9c7e77, before optstr 48db3634 made the call graph follow
// function values to their calls: this one goes from a function to the literals it makes, which
// reaches more, so it proves less and nothing wrongly): every allocation of a class (and of each class below it) is
// `this := &C{}` followed by silent statements and the constructor call, and every constructor
// assigns the field before anything can read it. See optstr's header for the rule. Copied
// without optstr's own bookkeeping, with two additions, each marked "valplain's addition":
//
//   - inert: a contained escape `this.g = F(this)` (F a trivial constructor that only stores the
//     object in fields h of the objects it makes) does not taint this.g when every use of each h
//     in the package is an assignment to it or a read `x.h.k` of a field k of the object other
//     than the field being proved, self_ and embedded ones: no code can get the object out of h,
//     so no code can read the field through it and the object has not escaped (Parser's
//     CommentRecorder.outer_, read only as outer_.sourceMapURL).
//   - fullySilent (a constructor called before the field's class is initialized) takes a
//     contained escape as quiet() does.
package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"postflip/rw"
)

var (
	funcDecls = map[*types.Func]*ast.FuncDecl{}
	// supers: the classes a struct embeds (j2go's superclass embedding), by struct type name.
	supers = map[*types.TypeName][]*types.TypeName{}
	lits   []*ast.CompositeLit
	// ownerOf: the struct type that declares a field.
	ownerOf = map[*types.Var]*types.TypeName{}
)

func index() {
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				if fn, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					funcDecls[fn] = fd
				}
			}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeSpec:
				tn, _ := rw.Info.Defs[x.Name].(*types.TypeName)
				if st, ok := x.Type.(*ast.StructType); ok && tn != nil {
					for _, fl := range st.Fields.List {
						if len(fl.Names) == 0 {
							if n, ok := types.Unalias(derefT(rw.Info.TypeOf(fl.Type))).(*types.Named); ok {
								supers[tn] = append(supers[tn], n.Origin().Obj())
							}
						}
						for _, nm := range fl.Names {
							if v, ok := rw.Info.Defs[nm].(*types.Var); ok {
								ownerOf[v] = tn
							}
						}
					}
				}
			case *ast.CompositeLit:
				lits = append(lits, x)
			}
			return true
		})
	}
}

// below reports whether class t is s or embeds s (transitively).
func below(t, s *types.TypeName) bool {
	if t == s {
		return true
	}
	for _, p := range supers[t] {
		if below(p, s) {
			return true
		}
	}
	return false
}

func recvClass(fn *types.Func) *types.TypeName {
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return nil
	}
	t := sig.Recv().Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := types.Unalias(t).(*types.Named); ok {
		return n.Origin().Obj()
	}
	return nil
}

func ctorName(name string) bool {
	return strings.HasPrefix(name, "init_") || strings.HasPrefix(name, "finit_") || strings.HasPrefix(name, "zero_")
}

func short(n ast.Node) string {
	s := strings.Join(strings.Fields(rw.Text(n)), " ")
	if len(s) > 70 {
		s = s[:70] + "..."
	}
	return s
}

// ---- fields

// fieldProof: every allocation of class s (or of a class below it) runs a constructor that
// assigns f before anything can read it. "" when proved, else the reason.
func fieldProof(s *types.TypeName, f *types.Var) string {
	n := 0
	for _, lit := range lits {
		nt, ok := types.Unalias(rw.Info.TypeOf(lit)).(*types.Named)
		if !ok || !below(nt.Origin().Obj(), s) {
			continue
		}
		n++
		if why := allocProof(lit, nt.Origin().Obj(), s, f); why != "" {
			return why
		}
	}
	if n == 0 {
		neverAllocated[f] = true
	}
	return ""
}

// allocProof: lit is `this := &T{}` in a block whose next statements are quiet up to the
// call of T's constructor, which reaches f's assignment (or they assign f: an anonymous class's
// captured variables).
func allocProof(lit *ast.CompositeLit, t, s *types.TypeName, f *types.Var) string {
	if len(lit.Elts) > 0 {
		return "allocated with fields given (" + short(lit) + ")"
	}
	u, ok := rw.Up(lit).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return "allocated otherwise than by &T{} (" + short(rw.Up(lit)) + ")"
	}
	as, ok := rw.Up(u).(*ast.AssignStmt)
	if !ok || len(as.Lhs) != 1 || as.Tok != token.DEFINE {
		return "allocation not stored in a new variable (" + short(rw.Up(u)) + ")"
	}
	thisObj := rw.Info.Defs[as.Lhs[0].(*ast.Ident)]
	blk, ok := rw.Parent[as].(*ast.BlockStmt)
	if !ok {
		return "allocation not in a block"
	}
	i := 0
	for i < len(blk.List) && blk.List[i] != as {
		i++
	}
	q := &quietness{thisObj: thisObj, f: f}
	tainted = map[*types.Var]bool{}
	for _, st := range blk.List[i+1:] {
		// this.self_ = this: the dispatch field set to the object (reading it is not silent)
		if a, ok := st.(*ast.AssignStmt); ok && len(a.Lhs) == 1 && len(a.Rhs) == 1 {
			if sel, ok := a.Lhs[0].(*ast.SelectorExpr); ok && sel.Sel.Name == "self_" && isObj(sel.X, thisObj) && isObj(a.Rhs[0], thisObj) {
				continue
			}
		}
		if assignsAllPaths(st, thisObj, f) {
			if !q.quiet(st) {
				return "the assignment's value can read the field (" + short(st) + ")"
			}
			return ""
		}
		if call, m := ctorCall(st, thisObj); call != nil {
			fd := funcDecls[m]
			if fd == nil {
				return "constructor without a body"
			}
			rc := recvClass(m)
			if strings.HasPrefix(m.Name(), "zero_") {
				if !q.escaped && silentArgs(call, thisObj, f) && fullySilent(fd, f, map[*ast.FuncDecl]bool{}) || q.quiet(st) {
					continue
				}
				return "zero_ can read the field"
			}
			if rc != t {
				return "allocation calls another class's constructor"
			}
			if !q.quietArgs(call) {
				return "constructor arguments can read the field (" + short(st) + ")"
			}
			return reaches(fd, s, f, map[*ast.FuncDecl]bool{}, q.escaped)
		}
		if !q.quiet(st) {
			return "statement before the constructor call (" + short(st) + ")"
		}
	}
	return "allocated without a constructor call (" + short(as) + ")"
}

// quietness: the code run before a field's assignment. While the object has not escaped, a
// statement is quiet when it is silent (silentStmt); once a statement let it escape (passed the
// object or called a method of it), every statement must be one that can reach no read of the
// field anywhere in the package (mayRead), and so is the escaping statement itself.
type quietness struct {
	thisObj types.Object
	f       *types.Var
	escaped bool
}

func (q *quietness) quiet(st ast.Node) bool {
	if !q.escaped {
		var ok bool
		switch x := st.(type) {
		case ast.Stmt:
			ok = silentStmt(x, q.thisObj, q.f, false)
		default:
			ok = silentExpr(x, q.thisObj, q.f)
		}
		if ok {
			return true
		}
	}
	if s, isStmt := st.(ast.Stmt); isStmt && !q.escaped && containedEscape(s, q.thisObj, q.f) {
		contained[q.f] = true
		return true
	}
	if s, isStmt := st.(ast.Stmt); isStmt && hasReturn(s) {
		return false
	}
	if mayRead(st, q.f) {
		return false
	}
	q.escaped = true
	escapes[q.f] = true
	return true
}

func (q *quietness) quietArgs(call *ast.CallExpr) bool {
	for _, a := range call.Args {
		if !q.quiet(a) {
			return false
		}
	}
	return true
}

// contained: fields proved with an object that escaped only into fields of its own.
var contained = map[*types.Var]bool{}

// neverAllocated: fields of classes no code allocates (no object, no read).
var neverAllocated = map[*types.Var]bool{}

// escapes: the fields proved through the call graph (their object escaped before the assignment).
var escapes = map[*types.Var]bool{}

func hasReturn(s ast.Stmt) bool {
	found := false
	ast.Inspect(s, func(n ast.Node) bool {
		switch n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			found = true
		}
		return !found
	})
	return found
}

// assignsAllPaths: st assigns this.f on every path that completes it.
func assignsAllPaths(st ast.Stmt, thisObj types.Object, f *types.Var) bool {
	switch x := st.(type) {
	case *ast.AssignStmt:
		if len(x.Lhs) == 1 && len(x.Rhs) == 1 && x.Tok == token.ASSIGN {
			if sel, ok := x.Lhs[0].(*ast.SelectorExpr); ok && thisPath(sel.X, thisObj) {
				if s := rw.Info.Selections[sel]; s != nil && s.Obj() == f {
					return true
				}
			}
		}
	case *ast.BlockStmt:
		for _, s := range x.List {
			if assignsAllPaths(s, thisObj, f) {
				return true
			}
		}
	case *ast.IfStmt:
		return x.Else != nil && assignsAllPaths(x.Body, thisObj, f) && assignsAllPaths(x.Else, thisObj, f)
	}
	return false
}

func isObj(e ast.Expr, o types.Object) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	return ok && o != nil && rw.Info.Uses[id] == o
}

// ctorCall: st is a call of an init_/finit_/zero_ method on this (or on an embedded class of it).
func ctorCall(st ast.Stmt, thisObj types.Object) (*ast.CallExpr, *types.Func) {
	es, ok := st.(*ast.ExprStmt)
	if !ok {
		return nil, nil
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok {
		return nil, nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !ctorName(sel.Sel.Name) || !thisPath(sel.X, thisObj) {
		return nil, nil
	}
	m, ok := rw.Info.Uses[sel.Sel].(*types.Func)
	if !ok {
		return nil, nil
	}
	return call, m
}

// thisPath: e is this, or this.E1.E2... through embedded (superclass) fields.
func thisPath(e ast.Expr, thisObj types.Object) bool {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		return rw.Info.Uses[x] == thisObj
	case *ast.SelectorExpr:
		if sel := rw.Info.Selections[x]; sel != nil && sel.Kind() == types.FieldVal {
			if v, ok := sel.Obj().(*types.Var); ok && v.Embedded() {
				return thisPath(x.X, thisObj)
			}
		}
	}
	return false
}

func recvObj(fd *ast.FuncDecl) types.Object {
	if fd.Recv == nil || len(fd.Recv.List) == 0 || len(fd.Recv.List[0].Names) == 0 {
		return nil
	}
	return rw.Info.Defs[fd.Recv.List[0].Names[0]]
}

// reaches: constructor fd (of s or a class below it) assigns f before anything can read it.
func reaches(fd *ast.FuncDecl, s *types.TypeName, f *types.Var, seen map[*ast.FuncDecl]bool, escaped bool) string {
	if seen[fd] {
		return "recursive constructor"
	}
	seen[fd] = true
	thisObj := recvObj(fd)
	q := &quietness{thisObj: thisObj, f: f, escaped: escaped}
	r := recvClass(rw.Info.Defs[fd.Name].(*types.Func))
	for _, st := range fd.Body.List {
		if assignsAllPaths(st, thisObj, f) {
			if !q.quiet(st) {
				return "the assignment's value can read the field (" + short(st) + ")"
			}
			return ""
		}
		if call, m := ctorCall(st, thisObj); call != nil {
			cfd := funcDecls[m]
			rc := recvClass(m)
			if cfd == nil {
				return "constructor without a body"
			}
			if !q.quietArgs(call) {
				return "constructor arguments can read the field (" + short(st) + ")"
			}
			switch {
			case below(rc, s) && below(r, rc) && !strings.HasPrefix(m.Name(), "zero_"):
				// a constructor of the class or between it and f's class (a delegation, a super call)
				why := reaches(cfd, s, f, seen, q.escaped)
				if why == "" {
					return ""
				}
				if !(!q.escaped && fullySilent(cfd, f, map[*ast.FuncDecl]bool{})) && !q.quiet(st) {
					return why
				}
			default:
				// a superclass constructor (runs before f's class is initialized), or zero_
				if !(below(s, rc) || rc == r) {
					return "constructor call of an unrelated class (" + short(st) + ")"
				}
				if q.escaped || !fullySilent(cfd, f, map[*ast.FuncDecl]bool{}) {
					if mayRead(st, f) {
						return "superclass constructor " + m.Name() + " can reach a read of the field"
					}
					q.escaped = true
					escapes[f] = true
				}
			}
			continue
		}
		if !q.quiet(st) {
			return "statement before the assignment can read the field (" + short(st) + ")"
		}
	}
	return "a constructor (" + fd.Name.Name + ") does not assign it"
}

// fullySilent: fd (a constructor-chain method) mentions this only through field reads/writes
// and silent constructor-chain calls, and does not read f.
func fullySilent(fd *ast.FuncDecl, f *types.Var, seen map[*ast.FuncDecl]bool) bool {
	if seen[fd] {
		return false
	}
	seen[fd] = true
	thisObj := recvObj(fd)
	for _, st := range fd.Body.List {
		if call, m := ctorCall(st, thisObj); call != nil {
			cfd := funcDecls[m]
			if cfd == nil || !silentArgs(call, thisObj, f) || !fullySilent(cfd, f, seen) {
				return false
			}
			continue
		}
		if !silentStmt(st, thisObj, f, true) {
			// valplain's addition: a contained escape is quiet here as it is in quiet()
			if containedEscape(st, thisObj, f) {
				contained[f] = true
				continue
			}
			return false
		}
	}
	return true
}

func silentArgs(call *ast.CallExpr, thisObj types.Object, f *types.Var) bool {
	for _, a := range call.Args {
		if !silentExpr(a, thisObj, f) {
			return false
		}
	}
	return true
}

// silentStmt: st mentions this only to read or write fields other than self_ (reading f is not
// silent); returns are allowed only where the caller completes anyway (retOK).
func silentStmt(st ast.Stmt, thisObj types.Object, f *types.Var, retOK bool) bool {
	ok := true
	ast.Inspect(st, func(n ast.Node) bool {
		if !ok {
			return false
		}
		switch x := n.(type) {
		case *ast.FuncLit:
			ok = silentExpr(x, thisObj, f)
			return false
		case *ast.ReturnStmt:
			if !retOK {
				ok = false
			}
		case *ast.BranchStmt:
			if x.Tok == token.GOTO {
				ok = false
			}
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				if sel, isSel := l.(*ast.SelectorExpr); isSel && thisPath(sel.X, thisObj) {
					if s := rw.Info.Selections[sel]; s != nil && s.Kind() == types.FieldVal {
						if !silentExpr(sel.X, thisObj, f) && !thisPath(sel.X, thisObj) {
							ok = false
						}
						continue
					}
				}
				if !silentExpr(l, thisObj, f) {
					ok = false
				}
			}
			for _, r := range x.Rhs {
				if !silentExpr(r, thisObj, f) {
					ok = false
				}
			}
			return false
		case ast.Expr:
			ok = silentExpr(x, thisObj, f)
			return false
		}
		return true
	})
	return ok
}

// silentExpr: every mention of this in e reads a field (through embedded classes) other than
// self_ and f.
func silentExpr(e ast.Node, thisObj types.Object, f *types.Var) bool {
	ok := true
	ast.Inspect(e, func(n ast.Node) bool {
		id, isId := n.(*ast.Ident)
		if !ok || !isId || rw.Info.Uses[id] != thisObj || thisObj == nil {
			return ok
		}
		var cur ast.Node = id
		for {
			sel, isSel := rw.Up(cur).(*ast.SelectorExpr)
			if !isSel || sel.X != cur && rw.Unparen(sel.X) != cur {
				ok = false // this itself used as a value
				return false
			}
			s := rw.Info.Selections[sel]
			if s != nil && s.Kind() == types.MethodVal {
				// a method of the object called directly (no dispatch): silent when its body is
				if c, isC := rw.Parent[sel].(*ast.CallExpr); isC && c.Fun == ast.Expr(sel) {
					if fd := funcDecls[s.Obj().(*types.Func).Origin()]; fd != nil && silentMethod(fd, f) {
						return false
					}
				}
				ok = false
				return false
			}
			if s == nil || s.Kind() != types.FieldVal {
				ok = false
				return false
			}
			v := s.Obj().(*types.Var)
			if v.Embedded() {
				cur = sel
				continue
			}
			if v == f || v.Name() == "self_" || tainted[v] {
				ok = false
				return false
			}
			// a field: its value is another object (this has not escaped)
			if p, isU := rw.Up(sel).(*ast.UnaryExpr); isU && p.Op == token.AND {
				ok = false
			}
			return false
		}
	})
	return ok
}

// tainted: fields of the object under construction that hold an object holding it (a contained
// escape: reading them is not silent). Reset for each allocation checked.
var tainted = map[*types.Var]bool{}

var (
	silentMemo = map[*ast.FuncDecl]map[*types.Var]bool{}
	silentBusy = map[*ast.FuncDecl]bool{}
)

// silentMethod: a method called on the object is silent: its body mentions its receiver only as
// silent code does (and does not read f).
func silentMethod(fd *ast.FuncDecl, f *types.Var) bool {
	if fd.Body == nil || silentBusy[fd] {
		return false
	}
	if len(tainted) == 0 {
		if r, ok := silentMemo[fd][f]; ok {
			return r
		}
	}
	silentBusy[fd] = true
	r := true
	for _, st := range fd.Body.List {
		if !silentStmt(st, recvObj(fd), f, true) {
			r = false
			break
		}
	}
	delete(silentBusy, fd)
	if len(tainted) == 0 {
		if silentMemo[fd] == nil {
			silentMemo[fd] = map[*types.Var]bool{}
		}
		silentMemo[fd][f] = r
	}
	return r
}

// containedEscape: st is `this.g = F(args)` where the arguments that mention the object are the
// object or a method value of it, and F only stores them in the objects it makes (trivial
// constructors): the object is then reachable only through this.g, which becomes tainted.
func containedEscape(st ast.Stmt, thisObj types.Object, f *types.Var) bool {
	a, ok := st.(*ast.AssignStmt)
	if !ok || len(a.Lhs) != 1 || len(a.Rhs) != 1 || a.Tok != token.ASSIGN {
		return false
	}
	sel, ok := a.Lhs[0].(*ast.SelectorExpr)
	if !ok || !thisPath(sel.X, thisObj) {
		return false
	}
	g, ok := rw.Info.Selections[sel].Obj().(*types.Var)
	if !ok || g == f {
		return false
	}
	call, ok := rw.Unparen(a.Rhs[0]).(*ast.CallExpr)
	if !ok {
		return false
	}
	fn, _ := fnOf(call.Fun)
	if fn == nil || funcDecls[fn] == nil {
		return false
	}
	any := false
	storedIn = map[*types.Var]bool{}
	objOnly := true
	for i, arg := range call.Args {
		if silentExpr(arg, thisObj, f) {
			continue
		}
		if !isObj(arg, thisObj) && !isMethodValueOf(arg, thisObj) {
			return false
		}
		objOnly = objOnly && isObj(arg, thisObj)
		if !trivialStore(funcDecls[fn], i, map[*ast.FuncDecl]bool{}) {
			return false
		}
		any = true
	}
	if !any {
		return false
	}
	if objOnly && inert(f) {
		// valplain's addition: the fields that now hold the object never hand it on, so the
		// object stays out of every other code's reach and this.g is an ordinary field
		return true
	}
	tainted[g] = true
	return true
}

// storedIn: the fields a trivial constructor stored the object in (containedEscape).
var storedIn = map[*types.Var]bool{}

// inert: every field in storedIn is, everywhere in the package, either assigned or read only as
// `x.h.k` with k a field other than f, self_ and the embedded ones (a read or write of k, not
// its address): no code gets the object out of h, so none can read its f through h.
func inert(f *types.Var) bool {
	if len(storedIn) == 0 {
		return false
	}
	for h := range storedIn {
		if h == nil {
			return false
		}
		for _, id := range rw.UsesOf[h] {
			sel, ok := rw.Parent[id].(*ast.SelectorExpr)
			if !ok || sel.Sel != id {
				return false
			}
			if a, ok := rw.Parent[sel].(*ast.AssignStmt); ok && isLhs(a, sel) && a.Tok == token.ASSIGN {
				continue
			}
			up, ok := rw.Parent[sel].(*ast.SelectorExpr)
			if !ok || up.X != ast.Expr(sel) {
				return false
			}
			s := rw.Info.Selections[up]
			if s == nil || s.Kind() != types.FieldVal || len(s.Index()) != 1 {
				return false
			}
			k := s.Obj().(*types.Var)
			if k.Origin() == f || k.Embedded() || k.Name() == "self_" {
				return false
			}
			if u, ok := rw.Up(up).(*ast.UnaryExpr); ok && u.Op == token.AND {
				return false
			}
		}
	}
	return true
}

func isMethodValueOf(e ast.Expr, thisObj types.Object) bool {
	sel, ok := rw.Unparen(e).(*ast.SelectorExpr)
	if !ok || !thisPath(sel.X, thisObj) {
		return false
	}
	s := rw.Info.Selections[sel]
	return s != nil && s.Kind() == types.MethodVal
}

// trivialStore: fd is a trivial constructor chain (new_X, init_X, a factory returning new_X(..)):
// it only allocates, sets self_, stores parameters and literals in fields of the new object and
// calls such functions, and parameter idx is only stored or handed on to such a function.
func trivialStore(fd *ast.FuncDecl, idx int, seen map[*ast.FuncDecl]bool) bool {
	if fd == nil || fd.Body == nil || seen[fd] {
		return false
	}
	seen[fd] = true
	var params []types.Object
	for _, fl := range fd.Type.Params.List {
		for _, n := range fl.Names {
			params = append(params, rw.Info.Defs[n])
		}
	}
	var p types.Object
	if idx >= 0 && idx < len(params) {
		p = params[idx]
	}
	recv := recvObj(fd)
	fresh := map[types.Object]bool{}
	if recv != nil {
		fresh[recv] = true
	}
	plain := func(e ast.Expr) bool {
		switch x := rw.Unparen(e).(type) {
		case *ast.BasicLit:
			return true
		case *ast.Ident:
			o := rw.Info.Uses[x]
			for _, q := range params {
				if q == o {
					return true
				}
			}
			return x.Name == "nil" || x.Name == "true" || x.Name == "false" || x.Name == "nullstr_"
		}
		return false
	}
	// handed: a call whose arguments are plain, each p at a position the callee only stores.
	handed := func(c *ast.CallExpr, callee *ast.FuncDecl) bool {
		for j, arg := range c.Args {
			if !plain(arg) {
				return false
			}
			if id, ok := rw.Unparen(arg).(*ast.Ident); ok && p != nil && rw.Info.Uses[id] == p {
				if !trivialStore(callee, j, seen) {
					return false
				}
			}
		}
		return trivialStore(callee, -1, seen) || p != nil
	}
	for _, st := range fd.Body.List {
		switch x := st.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) != 1 || len(x.Rhs) != 1 {
				return false
			}
			if x.Tok == token.DEFINE {
				u, ok := x.Rhs[0].(*ast.UnaryExpr)
				if !ok || u.Op != token.AND {
					return false
				}
				if cl, ok := u.X.(*ast.CompositeLit); !ok || len(cl.Elts) > 0 {
					return false
				}
				fresh[rw.Info.Defs[x.Lhs[0].(*ast.Ident)]] = true
				continue
			}
			sel, ok := x.Lhs[0].(*ast.SelectorExpr)
			if !ok {
				return false
			}
			base := sel.X
			for {
				if s2, ok := base.(*ast.SelectorExpr); ok {
					base = s2.X
					continue
				}
				break
			}
			bid, ok := base.(*ast.Ident)
			if !ok || !fresh[rw.Info.Uses[bid]] {
				return false
			}
			if sel.Sel.Name == "self_" && isObj(x.Rhs[0], rw.Info.Uses[bid]) {
				continue
			}
			if !plain(x.Rhs[0]) {
				return false
			}
			if id, ok := rw.Unparen(x.Rhs[0]).(*ast.Ident); ok && p != nil && rw.Info.Uses[id] == p {
				if s := rw.Info.Selections[sel]; s != nil && s.Kind() == types.FieldVal {
					storedIn[s.Obj().(*types.Var).Origin()] = true
				} else {
					storedIn[nil] = true
				}
			}
		case *ast.ExprStmt:
			c, ok := x.X.(*ast.CallExpr)
			if !ok {
				return false
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok || !ctorName(sel.Sel.Name) {
				return false
			}
			base := sel.X
			for {
				if s2, ok := base.(*ast.SelectorExpr); ok {
					base = s2.X
					continue
				}
				break
			}
			bid, ok := base.(*ast.Ident)
			if !ok || !fresh[rw.Info.Uses[bid]] {
				return false
			}
			m, _ := rw.Info.Uses[sel.Sel].(*types.Func)
			if m == nil || !handed(c, funcDecls[m.Origin()]) {
				return false
			}
		case *ast.ReturnStmt:
			if len(x.Results) != 1 {
				return false
			}
			if id, ok := rw.Unparen(x.Results[0]).(*ast.Ident); ok && fresh[rw.Info.Uses[id]] {
				continue
			}
			c, ok := rw.Unparen(x.Results[0]).(*ast.CallExpr)
			if !ok {
				return false
			}
			fn, _ := fnOf(c.Fun)
			if fn == nil || !handed(c, funcDecls[fn]) {
				return false
			}
		default:
			return false
		}
	}
	return true
}
