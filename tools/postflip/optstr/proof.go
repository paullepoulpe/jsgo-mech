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
	// mapKept: the erased map variables whose Optional values stay Optional objects.
	mapKept = map[*types.Var]bool{}
	// unboxOK: the optOf_[string] calls proved to read a non-null Optional.
	unboxOK = map[*ast.CallExpr]bool{}
	// runtimeOpt: runtime functions whose Optional<String> result is never null.
	runtimeOpt = map[string]bool{"Io_utf8StringStrict": true}
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
							if n, ok := types.Unalias(rw.Info.TypeOf(fl.Type)).(*types.Named); ok {
								supers[tn] = append(supers[tn], n.Origin().Obj())
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

// proveSlots checks every place the zero value of Optional[string] could be read.
func proveSlots() bool {
	ok := true
	fail := func(n ast.Node, why string) {
		leave(n, why)
		ok = false
	}
	optFields := map[*types.TypeName]bool{}
	for id, o := range rw.Info.Defs {
		v, isVar := o.(*types.Var)
		if !isVar || !generated(id) || !contains(v.Type()) {
			continue
		}
		direct := isOptStr(v.Type())
		_, isSig := types.Unalias(v.Type()).Underlying().(*types.Signature)
		if !direct && isSig {
			proved["func-typed slots (their zero is a nil func, not an Optional)"]++
			continue
		}
		switch {
		case v.IsField():
			st := fieldOwner(v)
			if st == nil {
				fail(id, "field of an unknown struct")
				continue
			}
			if !direct {
				if isMapKept(v) {
					mapKept[v] = true
					proved["erased map field holding Optional objects (kept: MapOf[string, Optional[string]])"]++
					continue
				}
				fail(id, "field holding Optionals in a container ("+types.TypeString(v.Type(), types.RelativeTo(rw.Pkg))+")")
				continue
			}
			optFields[st] = true
			if why := fieldProof(st, v); why != "" {
				fail(id, "field: "+why)
			} else if neverAllocated[v] {
				proved["fields of classes never allocated"]++
			} else if escapes[v] {
				proved["fields assigned by every constructor before any read (the object escapes first; no code it reaches reads the field)"]++
			} else if contained[v] {
				proved["fields assigned by every constructor before any read (the object is first stored only in a field of its own: tainted)"]++
			} else if false {
				proved["fields assigned by every constructor before any read (the object escapes first; no code it reaches reads the field)"]++
			} else {
				proved["fields assigned by every constructor before any read"]++
			}
		case v.Parent() == rw.Pkg.Scope():
			if direct {
				if why := clinitProof(v); why != "" {
					fail(id, "package variable: "+why)
				} else {
					proved["package variables assigned first in their clinit"]++
				}
			} else if a, isArr := types.Unalias(v.Type()).(*types.Array); isArr && isOptStr(a.Elem()) {
				if why := enumArrayProof(v, a.Len()); why != "" {
					fail(id, "enum field array: "+why)
				} else {
					proved["enum field arrays assigned for every constant in the enum's clinit"]++
				}
			} else {
				fail(id, "package variable holding Optionals in a container")
			}
		default:
			switch p := rw.Parent[id].(type) {
			case *ast.Field:
				fl := rw.Parent[p].(*ast.FieldList)
				if ft, isFT := rw.Parent[fl].(*ast.FuncType); isFT && ft.Results == fl {
					fail(id, "named result")
				} else if !direct {
					fail(id, "parameter holding Optionals in a container")
				} else {
					proved["parameters (given a value by every call)"]++
				}
			case *ast.ValueSpec:
				if !direct {
					fail(id, "local holding Optionals in a container")
				} else if len(p.Values) > 0 {
					proved["locals declared with a value"]++
				} else if why := daProof(v, p); why != "" {
					fail(id, "local without a value: "+why)
				} else if strings.HasPrefix(v.Name(), "ret_") {
					proved["locals without a value: j2go's try result temp, set before every return 1"]++
				} else {
					proved["locals without a value: definitely assigned before every read"]++
				}
			case *ast.AssignStmt:
				if !direct {
					fail(id, "local holding Optionals in a container")
				} else {
					proved["locals declared with a value"]++
				}
			default:
				fail(id, "variable declared otherwise")
			}
		}
	}
	// A struct with such a field must only be used through pointers (its zero value is never
	// made otherwise than by the allocations fieldProof checked).
	for id, o := range rw.Info.Defs {
		v, isVar := o.(*types.Var)
		if !isVar || v.Embedded() {
			continue
		}
		if tn := byValue(v.Type(), optFields); tn != nil {
			fail(id, "struct "+tn.Name()+" with an Optional field used by value")
		}
	}
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				t := rw.Info.TypeOf(x)
				if isOptStr(t) {
					fail(x, "composite literal of the type")
				} else if _, isStruct := types.Unalias(t).Underlying().(*types.Struct); !isStruct && contains(t) {
					if _, isSig := types.Unalias(t).Underlying().(*types.Signature); !isSig {
						fail(x, "composite literal holding Optionals")
					}
				}
			case *ast.CallExpr:
				if id, isId := x.Fun.(*ast.Ident); isId && (id.Name == "make" || id.Name == "new") && rw.Info.Uses[id] != nil {
					if _, isB := rw.Info.Uses[id].(*types.Builtin); isB && (contains(rw.Info.TypeOf(x)) || byValue(rw.Info.TypeOf(x), optFields) != nil) {
						fail(x, "make / new of the type")
					}
				}
				if name, _ := rw.CallName(x); name == "optOf_[string]" {
					if why := unboxProof(x); why != "" {
						fail(x, "optOf_[string]: "+why)
					} else {
						unboxOK[x] = true
						proved["casts of Objects from a map of non-null Optionals with a non-null default"]++
					}
				}
			case *ast.TypeAssertExpr:
				if x.Type != nil && contains(rw.Info.TypeOf(x.Type)) {
					fail(x, "type assertion to the type")
				}
			}
			return true
		})
	}
	// A function that recovers a panic in a deferred call returns its results' zero values
	// (unnamed results cannot be set by the deferred call).
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			var ft *ast.FuncType
			var body *ast.BlockStmt
			switch x := n.(type) {
			case *ast.FuncDecl:
				ft, body = x.Type, x.Body
			case *ast.FuncLit:
				ft, body = x.Type, x.Body
			default:
				return true
			}
			if ft.Results == nil || body == nil {
				return true
			}
			has := false
			for _, r := range ft.Results.List {
				has = has || contains(rw.Info.TypeOf(r.Type))
			}
			if !has {
				return true
			}
			ast.Inspect(body, func(m ast.Node) bool {
				switch y := m.(type) {
				case *ast.FuncLit:
					return false
				case *ast.DeferStmt:
					ast.Inspect(y, func(k ast.Node) bool {
						if c, ok := k.(*ast.CallExpr); ok {
							if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "recover" {
								fail(y, "a function returning the type recovers a panic (it would return the zero value)")
							}
						}
						return true
					})
					return false
				}
				return true
			})
			return true
		})
	}
	for id, inst := range rw.Info.Instances {
		if !generated(id) {
			continue
		}
		for i := 0; i < inst.TypeArgs.Len(); i++ {
			ta := inst.TypeArgs.At(i)
			if !isOptStr(ta) && !(contains(ta) && !isSigType(ta)) {
				continue
			}
			switch id.Name {
			case "MapOf":
				// the kept map: checked with its field
			default:
				fail(id, "generic code instantiated with Optional[string] ("+id.Name+")")
			}
		}
	}
	return ok
}

func isSigType(t types.Type) bool {
	_, ok := types.Unalias(t).Underlying().(*types.Signature)
	return ok
}

// byValue is the struct with an Optional field that t holds by value (not through a pointer).
func byValue(t types.Type, structs map[*types.TypeName]bool) *types.TypeName {
	switch u := types.Unalias(t).(type) {
	case *types.Named:
		if structs[u.Origin().Obj()] {
			return u.Origin().Obj()
		}
	case *types.Slice:
		return byValue(u.Elem(), structs)
	case *types.Array:
		return byValue(u.Elem(), structs)
	case *types.Map:
		return byValue(u.Elem(), structs)
	}
	return nil
}

func fieldOwner(v *types.Var) *types.TypeName {
	for tn := range supers {
		_ = tn
	}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			g, ok := d.(*ast.GenDecl)
			if !ok || g.Tok != token.TYPE {
				continue
			}
			for _, s := range g.Specs {
				ts := s.(*ast.TypeSpec)
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				for _, fl := range st.Fields.List {
					for _, n := range fl.Names {
						if rw.Info.Defs[n] == v {
							return rw.Info.Defs[ts.Name].(*types.TypeName)
						}
					}
				}
			}
		}
	}
	return nil
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
	for i, arg := range call.Args {
		if silentExpr(arg, thisObj, f) {
			continue
		}
		if !isObj(arg, thisObj) && !isMethodValueOf(arg, thisObj) {
			return false
		}
		if !trivialStore(funcDecls[fn], i, map[*ast.FuncDecl]bool{}) {
			return false
		}
		any = true
	}
	if !any {
		return false
	}
	tainted[g] = true
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

// ---- package variables

func writesOf(v *types.Var) []*ast.Ident {
	var out []*ast.Ident
	for _, id := range rw.UsesOf[v] {
		if a, ok := rw.Parent[id].(*ast.AssignStmt); ok {
			for _, l := range a.Lhs {
				if l == ast.Expr(id) {
					out = append(out, id)
				}
			}
		}
	}
	return out
}

func enclosingFunc(n ast.Node) *ast.FuncDecl {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		if fd, ok := p.(*ast.FuncDecl); ok {
			return fd
		}
	}
	return nil
}

// constish: e calls nothing but Optional's constructors (no code of the program runs).
func constish(e ast.Expr) bool {
	ok := true
	ast.Inspect(e, func(n ast.Node) bool {
		if c, isC := n.(*ast.CallExpr); isC {
			name, _ := rw.CallName(c)
			if !strings.HasPrefix(name, "Optional_") {
				if tv, has := rw.Info.Types[c.Fun]; !has || !tv.IsType() {
					ok = false
				}
			}
		}
		if _, isF := n.(*ast.FuncLit); isF {
			ok = false
		}
		return ok
	})
	return ok
}

func clinitProof(v *types.Var) string {
	for _, id := range writesOf(v) {
		fd := enclosingFunc(id)
		if fd == nil || !strings.HasSuffix(fd.Name.Name, "_clinit") || fd.Recv != nil {
			continue
		}
		for _, st := range fd.Body.List {
			if a, ok := st.(*ast.AssignStmt); ok && len(a.Lhs) == 1 && a.Lhs[0] == ast.Expr(id) {
				if !constish(a.Rhs[0]) {
					return "the clinit assigns it a computed value"
				}
				return ""
			}
			switch x := st.(type) {
			case *ast.IfStmt: // if X_clinit_done { return }
				if id2, ok := x.Cond.(*ast.Ident); ok && strings.HasSuffix(id2.Name, "_clinit_done") {
					continue
				}
			case *ast.AssignStmt:
				ok := true
				for _, r := range x.Rhs {
					ok = ok && constish(r)
				}
				if ok {
					continue
				}
			}
			return "code runs in its clinit before the assignment (" + short(st) + ")"
		}
	}
	return "not assigned at the top of a clinit"
}

// enumArrayProof: arr[this-1] is assigned by every constructor of enum E (silently: array slots
// only), and E's clinit constructs all n constants.
func enumArrayProof(v *types.Var, n int64) string {
	var enum *types.TypeName
	for _, id := range rw.UsesOf[v] {
		ix, ok := rw.Parent[id].(*ast.IndexExpr)
		if !ok {
			return "used otherwise than indexed"
		}
		fd := enclosingFunc(id)
		if fd == nil || fd.Recv == nil {
			return "indexed outside the enum's methods"
		}
		rc := recvClass(rw.Info.Defs[fd.Name].(*types.Func))
		if enum == nil {
			enum = rc
		} else if enum != rc {
			return "indexed in methods of two types"
		}
		if !isThisMinus1(ix.Index, recvObj(fd)) {
			return "indexed otherwise than [this-1]"
		}
		if a, ok := rw.Parent[ix].(*ast.AssignStmt); ok && a.Lhs[0] == ast.Expr(ix) && !strings.HasPrefix(fd.Name.Name, "init_") {
			return "assigned outside the constructors"
		}
	}
	if enum == nil {
		return "never used"
	}
	// every constructor assigns the slot
	for fn, fd := range funcDecls {
		if recvClass(fn) == enum && strings.HasPrefix(fn.Name(), "init_") {
			if !enumCtorAssigns(fd, v, map[*ast.FuncDecl]bool{}) {
				return "constructor " + fn.Name() + " does not assign it first"
			}
		}
	}
	// the clinit constructs every constant
	clinit := rw.Pkg.Scope().Lookup(enum.Name() + "_clinit")
	fd := funcDecls[asFunc(clinit)]
	if fd == nil {
		return "no clinit"
	}
	consts := map[types.Object]bool{}
	for _, st := range fd.Body.List {
		es, ok := st.(*ast.ExprStmt)
		if !ok {
			continue
		}
		c, ok := es.X.(*ast.CallExpr)
		if !ok {
			continue
		}
		sel, ok := c.Fun.(*ast.SelectorExpr)
		if !ok || !strings.HasPrefix(sel.Sel.Name, "init_") {
			continue
		}
		if id, ok := sel.X.(*ast.Ident); ok {
			if k, ok := rw.Info.Uses[id].(*types.Const); ok {
				consts[k] = true
			}
		}
	}
	if int64(len(consts)) != n {
		return "the clinit constructs fewer constants than the array holds"
	}
	return ""
}

func asFunc(o types.Object) *types.Func {
	f, _ := o.(*types.Func)
	return f
}

func isThisMinus1(e ast.Expr, thisObj types.Object) bool {
	b, ok := rw.Unparen(e).(*ast.BinaryExpr)
	if !ok || b.Op != token.SUB || !isObj(b.X, thisObj) {
		return false
	}
	l, ok := b.Y.(*ast.BasicLit)
	return ok && l.Value == "1"
}

// enumCtorAssigns: fd's statements are array-slot assignments of constant-like values and
// delegations, and one of them assigns v's slot.
func enumCtorAssigns(fd *ast.FuncDecl, v *types.Var, seen map[*ast.FuncDecl]bool) bool {
	if seen[fd] {
		return false
	}
	seen[fd] = true
	thisObj := recvObj(fd)
	for _, st := range fd.Body.List {
		switch x := st.(type) {
		case *ast.AssignStmt:
			ix, ok := x.Lhs[0].(*ast.IndexExpr)
			if !ok || len(x.Lhs) != 1 || !isThisMinus1(ix.Index, thisObj) || !constish(x.Rhs[0]) {
				return false
			}
			if id, ok := ix.X.(*ast.Ident); ok && rw.Info.Uses[id] == v {
				return true
			}
		case *ast.ExprStmt:
			c, ok := x.X.(*ast.CallExpr)
			if !ok {
				return false
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok || !strings.HasPrefix(sel.Sel.Name, "init_") || !isObj(sel.X, thisObj) {
				return false
			}
			for _, a := range c.Args {
				if !constish(a) {
					return false
				}
			}
			return enumCtorAssigns(funcDecls[asFunc(rw.Info.Uses[sel.Sel])], v, seen)
		default:
			return false
		}
	}
	return false
}

// ---- the erased map of Optionals

// isMapKept: v is an erased map field (MapOf[K, Optional[string]]) assigned only new maps, whose
// values are only put as box_() of Optionals and read by getOrDefault with a box_() default, or
// printed / tested for keys.
func isMapKept(v *types.Var) bool {
	n, ok := types.Unalias(v.Type()).(*types.Named)
	if a, isA := v.Type().(*types.Alias); isA {
		if a.Obj().Name() != "MapOf" || a.TypeArgs().Len() != 2 || !isOptStr(a.TypeArgs().At(1)) {
			return false
		}
	} else if !ok || n.Obj().Name() != "MapOf" {
		return false
	}
	for _, id := range rw.UsesOf[v] {
		sel, ok := rw.Parent[id].(*ast.SelectorExpr)
		if !ok {
			return false
		}
		switch p := rw.Up(sel).(type) {
		case *ast.AssignStmt:
			if len(p.Lhs) != 1 || p.Lhs[0] != ast.Expr(sel) {
				return false
			}
			rhs := p.Rhs[0]
			if c, ok := rw.Unparen(rhs).(*ast.CallExpr); ok && len(c.Args) == 1 { // reflip15: Map(new_LinkedHashMap())
				if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() && types.IsInterface(tv.Type) {
					rhs = c.Args[0]
				}
			}
			if name, _ := rw.CallName(rhs); name != "new_LinkedHashMap" && name != "new_HashMap" {
				return false
			}
		case *ast.CallExpr:
			name, _ := rw.CallName(p)
			switch name {
			case "Map_put_s":
				if len(p.Args) != 3 || p.Args[0] != ast.Expr(sel) || !isOptBox(p.Args[2]) {
					return false
				}
			case "Map_containsKey_s":
			case "Map_getOrDefault_s":
				if len(p.Args) != 3 || !isOptBox(p.Args[2]) {
					return false
				}
				if c, ok := rw.Up(p).(*ast.CallExpr); !ok {
					return false
				} else if n, _ := rw.CallName(c); n != "optOf_[string]" && n != "optstrUnbox_" {
					return false
				}
			default:
				// MoreObjects.toStringHelper(...).add("declarations", map): prints the Optionals
				if s, ok := p.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "add" {
					continue
				}
				return false
			}
		default:
			return false
		}
	}
	return true
}

func isOptBox(e ast.Expr) bool {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if name, _ := rw.CallName(e); ok && name == "optstrBox_" {
		return true // a rerun: the box the step wrote
	}
	if !ok || len(c.Args) != 0 {
		return false
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	return ok && s.Sel.Name == "box_" && isOptStr(rw.Info.TypeOf(s.X))
}

// unboxProof: optOf_[string](Map_getOrDefault_s(m, k, o.box_())) of a kept map.
func unboxProof(c *ast.CallExpr) string {
	g, ok := rw.Unparen(c.Args[0]).(*ast.CallExpr)
	if !ok {
		return "not a read of a map of Optionals"
	}
	if name, _ := rw.CallName(g); name != "Map_getOrDefault_s" {
		return "not a getOrDefault"
	}
	sel, ok := g.Args[0].(*ast.SelectorExpr)
	if !ok {
		return "map not a field"
	}
	v, _ := rw.Info.Uses[sel.Sel].(*types.Var)
	if v == nil || !isMapKept(v) {
		return "map not proved to hold only boxed Optionals"
	}
	return ""
}

// ---- locals declared without a value

// daProof: v is assigned on every path before each read (Java's definite assignment, checked
// on the Go statements), or is j2go's try result temp.
func daProof(v *types.Var, spec *ast.ValueSpec) string {
	decl := rw.Parent[rw.Parent[spec]]
	blk, ok := rw.Parent[decl].(*ast.BlockStmt)
	if !ok {
		return "declared outside a block"
	}
	if strings.HasPrefix(v.Name(), "ret_") {
		return retTempProof(v)
	}
	i := 0
	for i < len(blk.List) && blk.List[i] != decl {
		i++
	}
	d := &daState{v: v}
	d.stmts(blk.List[i+1:], false)
	return d.fail
}

type daState struct {
	v      *types.Var
	fail   string
	breaks []*brk
	rets   [][]bool // per enclosing immediately-called func literal
}

type brk struct {
	label  string
	target ast.Stmt
	states []bool
}

func (d *daState) failAt(n ast.Node, why string) {
	if d.fail == "" {
		d.fail = why + " (" + rw.Fset.Position(n.Pos()).String() + ")"
	}
}

// stmts returns the state after list and whether it cannot complete normally.
func (d *daState) stmts(list []ast.Stmt, in bool) (bool, bool) {
	for _, s := range list {
		o, t := d.stmt(s, in, "")
		if t {
			return true, true
		}
		in = o
	}
	return in, false
}

func and(states []bool) bool {
	for _, s := range states {
		if !s {
			return false
		}
	}
	return true
}

func (d *daState) stmt(s ast.Stmt, in bool, label string) (bool, bool) {
	switch x := s.(type) {
	case *ast.DeclStmt:
		for _, sp := range x.Decl.(*ast.GenDecl).Specs {
			if vs, ok := sp.(*ast.ValueSpec); ok {
				for _, e := range vs.Values {
					in = d.reads(e, in)
				}
			}
		}
		return in, false
	case *ast.AssignStmt:
		for _, e := range x.Rhs {
			in = d.reads(e, in)
		}
		for _, l := range x.Lhs {
			if id, ok := l.(*ast.Ident); ok && (rw.Info.Uses[id] == d.v || rw.Info.Defs[id] == d.v) {
				if x.Tok != token.ASSIGN && x.Tok != token.DEFINE && !in {
					d.failAt(id, "compound assignment before assigned")
				}
				in = true
				continue
			}
			in = d.reads(l, in)
		}
		return in, false
	case *ast.ExprStmt:
		in = d.reads(x.X, in)
		if c, ok := x.X.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "panic" {
				return true, true
			}
		}
		return in, false
	case *ast.IncDecStmt:
		return d.reads(x.X, in), false
	case *ast.ReturnStmt:
		for _, e := range x.Results {
			in = d.reads(e, in)
		}
		if len(d.rets) > 0 {
			d.rets[len(d.rets)-1] = append(d.rets[len(d.rets)-1], in)
		}
		return true, true
	case *ast.BranchStmt:
		switch x.Tok {
		case token.BREAK:
			for i := len(d.breaks) - 1; i >= 0; i-- {
				b := d.breaks[i]
				if x.Label == nil && b.target != nil || x.Label != nil && b.label == x.Label.Name {
					b.states = append(b.states, in)
					return true, true
				}
			}
			d.failAt(x, "break to an unknown target")
		case token.CONTINUE:
			return true, true
		default:
			d.failAt(x, "goto / fallthrough")
		}
		return true, true
	case *ast.BlockStmt:
		return d.stmts(x.List, in)
	case *ast.LabeledStmt:
		return d.stmt(x.Stmt, in, x.Label.Name)
	case *ast.IfStmt:
		if x.Init != nil {
			in, _ = d.stmt(x.Init, in, "")
		}
		in = d.reads(x.Cond, in)
		o1, t1 := d.stmt(x.Body, in, "")
		o2, t2 := in, false
		if x.Else != nil {
			o2, t2 = d.stmt(x.Else, in, "")
		}
		switch {
		case t1 && t2:
			return true, true
		case t1:
			return o2, false
		case t2:
			return o1, false
		}
		return o1 && o2, false
	case *ast.SwitchStmt, *ast.TypeSwitchStmt:
		var init ast.Stmt
		var body *ast.BlockStmt
		if sw, ok := x.(*ast.SwitchStmt); ok {
			init, body = sw.Init, sw.Body
			if init != nil {
				in, _ = d.stmt(init, in, "")
			}
			if sw.Tag != nil {
				in = d.reads(sw.Tag, in)
			}
		} else {
			ts := x.(*ast.TypeSwitchStmt)
			if ts.Init != nil {
				in, _ = d.stmt(ts.Init, in, "")
			}
			in, _ = d.stmt(ts.Assign, in, "")
			body = ts.Body
		}
		b := &brk{label: label, target: s}
		d.breaks = append(d.breaks, b)
		var outs []bool
		hasDefault := false
		for _, c := range body.List {
			cc := c.(*ast.CaseClause)
			if cc.List == nil {
				hasDefault = true
			}
			cin := in
			for _, e := range cc.List {
				cin = d.reads(e, cin)
			}
			if len(cc.Body) > 0 {
				if br, ok := cc.Body[len(cc.Body)-1].(*ast.BranchStmt); ok && br.Tok == token.FALLTHROUGH {
					d.failAt(br, "fallthrough")
				}
			}
			o, t := d.stmts(cc.Body, cin)
			if !t {
				outs = append(outs, o)
			}
		}
		d.breaks = d.breaks[:len(d.breaks)-1]
		if !hasDefault {
			outs = append(outs, in)
		}
		outs = append(outs, b.states...)
		if len(outs) == 0 {
			return true, true
		}
		return and(outs), false
	case *ast.ForStmt:
		if x.Init != nil {
			in, _ = d.stmt(x.Init, in, "")
		}
		if x.Cond != nil {
			in = d.reads(x.Cond, in)
		}
		b := &brk{label: label, target: s}
		d.breaks = append(d.breaks, b)
		o, t := d.stmt(x.Body, in, "")
		if x.Post != nil && !t {
			d.stmt(x.Post, o, "")
		}
		d.breaks = d.breaks[:len(d.breaks)-1]
		var outs []bool
		if x.Cond != nil {
			outs = append(outs, in)
		}
		outs = append(outs, b.states...)
		if len(outs) == 0 {
			return true, true
		}
		return and(outs), false
	case *ast.RangeStmt:
		in = d.reads(x.X, in)
		b := &brk{label: label, target: s}
		d.breaks = append(d.breaks, b)
		d.stmt(x.Body, in, "")
		d.breaks = d.breaks[:len(d.breaks)-1]
		return and(append([]bool{in}, b.states...)), false
	case *ast.DeferStmt:
		return d.reads(x.Call, in), false
	case *ast.EmptyStmt:
		return in, false
	}
	d.failAt(s, "statement the check does not follow")
	return in, false
}

// reads checks the reads of v in e against state in; an immediately called func literal runs
// in place (its returns join), any other func literal is checked with the state at its creation.
func (d *daState) reads(e ast.Expr, in bool) bool {
	out := in
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.CallExpr:
			if fl, ok := rw.Unparen(x.Fun).(*ast.FuncLit); ok {
				for _, a := range x.Args {
					out = d.reads(a, out)
				}
				saved := d.breaks
				d.breaks = nil
				d.rets = append(d.rets, nil)
				o, t := d.stmts(fl.Body.List, out)
				rs := d.rets[len(d.rets)-1]
				d.rets = d.rets[:len(d.rets)-1]
				d.breaks = saved
				if !t {
					rs = append(rs, o)
				}
				out = and(rs)
				return false
			}
		case *ast.FuncLit:
			sub := &daState{v: d.v}
			sub.rets = [][]bool{nil}
			sub.stmts(x.Body.List, out)
			if sub.fail != "" {
				d.failAt(x, "read in a function literal before assigned")
			}
			return false
		case *ast.Ident:
			if rw.Info.Uses[x] == d.v && !out {
				d.failAt(x, "read before assigned")
			}
		case *ast.UnaryExpr:
			if x.Op == token.AND {
				if id, ok := rw.Unparen(x.X).(*ast.Ident); ok && rw.Info.Uses[id] == d.v {
					d.failAt(x, "address taken")
				}
			}
		}
		return true
	})
	return out
}

// retTempProof: j2go's lowering of a return inside try: `code_N := func() (code_ int) {...}()`,
// then `if code_N == 1 { return ret_N }`. ret_N is read only there, and inside the closure every
// `return 1` and `code_ = 1` is right after an assignment of ret_N in the same block.
func retTempProof(v *types.Var) string {
	n := strings.TrimPrefix(v.Name(), "ret_")
	for _, id := range rw.UsesOf[v] {
		if a, ok := rw.Parent[id].(*ast.AssignStmt); ok && a.Lhs[0] == ast.Expr(id) && len(a.Lhs) == 1 {
			continue
		}
		r, ok := rw.Parent[id].(*ast.ReturnStmt)
		if !ok {
			return "ret_ read otherwise than returned"
		}
		blk, _ := rw.Parent[r].(*ast.BlockStmt)
		ifs, _ := rw.Parent[blk].(*ast.IfStmt)
		if ifs == nil {
			return "ret_ returned outside `if code_N == 1`"
		}
		b, ok := ifs.Cond.(*ast.BinaryExpr)
		if !ok || b.Op != token.EQL {
			return "ret_ returned outside `if code_N == 1`"
		}
		cid, ok := b.X.(*ast.Ident)
		lit, ok2 := b.Y.(*ast.BasicLit)
		if !ok || !ok2 || cid.Name != "code_"+n || lit.Value != "1" {
			return "ret_ returned outside `if code_N == 1`"
		}
		// the closure assigned to code_N
		co, _ := rw.Info.Uses[cid].(*types.Var)
		var fl *ast.FuncLit
		for did, o := range rw.Info.Defs {
			if o == co {
				if as, ok := rw.Parent[did].(*ast.AssignStmt); ok {
					if c, ok := as.Rhs[0].(*ast.CallExpr); ok {
						fl, _ = c.Fun.(*ast.FuncLit)
					}
				}
			}
		}
		if fl == nil {
			return "code_N not a called closure"
		}
		if why := retSetBefore1(fl, v); why != "" {
			return why
		}
	}
	return ""
}

func retSetBefore1(fl *ast.FuncLit, v *types.Var) string {
	why := ""
	ast.Inspect(fl.Body, func(n ast.Node) bool {
		var st ast.Stmt
		switch x := n.(type) {
		case *ast.ReturnStmt:
			if len(x.Results) == 1 {
				if l, ok := x.Results[0].(*ast.BasicLit); ok && l.Value == "1" {
					st = x
				}
			}
		case *ast.AssignStmt:
			if id, ok := x.Lhs[0].(*ast.Ident); ok && id.Name == "code_" {
				if l, ok := x.Rhs[0].(*ast.BasicLit); ok && l.Value == "1" {
					st = x
				}
			}
		}
		if st == nil {
			return true
		}
		blk, ok := rw.Parent[st].(*ast.BlockStmt)
		if !ok {
			why = "a return 1 outside a block"
			return false
		}
		for i, s := range blk.List {
			if s == st {
				if i == 0 {
					why = "a return 1 not right after ret_N = ..."
					return false
				}
				a, ok := blk.List[i-1].(*ast.AssignStmt)
				if !ok || len(a.Lhs) != 1 {
					why = "a return 1 not right after ret_N = ..."
					return false
				}
				if id, ok := a.Lhs[0].(*ast.Ident); !ok || rw.Info.Uses[id] != v {
					why = "a return 1 not right after ret_N = ..."
					return false
				}
			}
		}
		return true
	})
	return why
}
