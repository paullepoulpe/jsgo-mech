package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"

	"postflip/rw"
)

// A class-hierarchy call graph of the package (generated code and runtime), for the fields whose
// constructor lets the object escape before it assigns the field: the code that can run before
// the assignment must not read the field on any object. Edges: a call of a function or of a
// concrete method goes to its declaration; a call of an interface method (or a type parameter's)
// to every method of that name; a call of a func value, a range over a func, or a func value
// handed to code outside the package, to every function used as a value whose signature can be
// the call's; making a function literal (or naming a function as a value) is no call of it.
//
// Function values, closed world: every func value of the program is a function literal or a
// declared function or method named outside the callee position of a call, both in the package
// (valueFuncs lists them all, wherever they are made: function bodies and package variable
// initializers alike). Go runs such a value only where a call's callee is an expression of func
// type, where a range statement ranges over one, or in code outside the package that was
// given it; each of those sites has edges to every listed value whose signature can be the
// site's. A func value's dynamic type is the func type it was made with, and the site's static
// type is that type up to instantiation: inside generic code either side may be written with
// type parameters (the hash func(K) int32 a typed map calls is func(*Var) int32 where the
// conversion was made, and func(T) int32 in a generic conversion). So two signatures can be
// the same at run time when they are identical with every type parameter, of either side,
// standing for any type (sameSig); for signatures without type parameters this is identity.
// Hence a function that only stores a literal (a conversion's hash and eq, a sorted map's
// comparison) has no edge to it: the literal is reached from whatever can reach a call of a
// func value of its type, which is the only code that runs it.
//
// A call of a func-typed struct field whose every store in the package is a known function goes
// to those functions (fieldfuncs.go).
//
// A typed Object call (jtostr_(x), tostrR_(x) of an x whose static type is a pointer to a class)
// goes to that class's toString alone (objcalls.go).

var (
	chaNodes   = map[ast.Node][]ast.Node{} // callees, computed once per node
	chaDone    = map[ast.Node]bool{}
	methodsBy  = map[string][]ast.Node{}
	valueFuncs []*valueFunc
	readsField = map[ast.Node]map[*types.Var]bool{}
	chaBuilt   bool
)

type valueFunc struct {
	sig  *types.Signature
	node ast.Node
	site ast.Node // where the value is made: its generic context gives the signature's instances
	all  []*types.Signature
	done bool
}

// sigs: the signatures the value can have at run time: its own, or, made in generic code, its
// signature at every instantiation of that code the program makes (instances.go); with the
// type parameters left in (standing for any type) where those are not known.
func (v *valueFunc) sigs() []*types.Signature {
	if v.done {
		return v.all
	}
	v.done = true
	v.all = []*types.Signature{v.sig}
	if !mentionsTP(v.sig) {
		return v.all
	}
	tps, tuples, ok := contextOf(v.site)
	if !ok || len(tuples) == 0 {
		return v.all
	}
	v.all = nil
	seen := map[string]bool{}
	for _, tu := range tuples {
		m := map[*types.TypeParam]types.Type{}
		for i, tp := range tps {
			m[tp] = tu[i]
		}
		s2 := subst(v.sig, m).(*types.Signature)
		k := types.TypeString(s2, nil)
		if mentionsTP(s2) || !seen[k] {
			seen[k] = true
			v.all = append(v.all, s2)
		}
	}
	return v.all
}

func buildCHA() {
	if chaBuilt {
		return
	}
	chaBuilt = true
	for fn, fd := range funcDecls {
		if fd.Recv != nil && fd.Body != nil {
			methodsBy[fn.Name()] = append(methodsBy[fn.Name()], fd)
		}
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncLit:
				if c, ok := rw.Parent[x].(*ast.CallExpr); ok && c.Fun == ast.Expr(x) {
					return true // called in place: an edge of its caller
				}
				if sig, ok := rw.Info.TypeOf(x).(*types.Signature); ok {
					valueFuncs = append(valueFuncs, &valueFunc{sig: sig, node: x, site: x})
				}
			case *ast.Ident:
				fn, ok := rw.Info.Uses[x].(*types.Func)
				if !ok {
					return true
				}
				var cur ast.Node = x
				if s, ok := rw.Parent[x].(*ast.SelectorExpr); ok && s.Sel == x {
					cur = s
				}
				if ix, ok := rw.Parent[cur].(*ast.IndexExpr); ok && ix.X == cur {
					cur = ix
				}
				if c, ok := rw.Parent[cur].(*ast.CallExpr); ok && c.Fun == cur {
					return true
				}
				fd := funcDecls[fn.Origin()]
				if fd == nil || fd.Body == nil {
					return true
				}
				if fnForwarder_(fn) { // reflip31: the method value x.m of a function type is the value x
					return true
				}
				sig := fn.Type().(*types.Signature)
				if inst, ok := rw.Info.Instances[x]; ok {
					// a generic function named with type arguments (written or inferred)
					if is, ok := inst.Type.(*types.Signature); ok {
						sig = is
					}
				}
				valueFuncs = append(valueFuncs, &valueFunc{sig: types.NewSignatureType(nil, nil, nil, sig.Params(), sig.Results(), sig.Variadic()), node: fd, site: x})
			}
			return true
		})
	}
}

func bodyOf(n ast.Node) *ast.BlockStmt {
	switch x := n.(type) {
	case *ast.FuncDecl:
		return x.Body
	case *ast.FuncLit:
		return x.Body
	case *boundFn:
		return x.Body
	case *instLit_: // reflip31
		return x.Body
	}
	return nil
}

// sameSig reports whether func values of signature a can be called where one of signature b
// is: the signatures (without receivers) are identical with every type parameter standing for
// any type.
func sameSig(a, b *types.Signature) bool {
	if a.Variadic() != b.Variadic() || a.Params().Len() != b.Params().Len() || a.Results().Len() != b.Results().Len() {
		return false
	}
	for i := 0; i < a.Params().Len(); i++ {
		if !typeMatch(a.Params().At(i).Type(), b.Params().At(i).Type()) {
			return false
		}
	}
	for i := 0; i < a.Results().Len(); i++ {
		if !typeMatch(a.Results().At(i).Type(), b.Results().At(i).Type()) {
			return false
		}
	}
	return true
}

// typeMatch: x and y can be one type under some instantiation of the type parameters in them.
// A type parameter matches anything (its constraint is not used: more matches, more edges).
func typeMatch(x, y types.Type) bool {
	x, y = types.Unalias(x), types.Unalias(y)
	if _, ok := x.(*types.TypeParam); ok {
		return true
	}
	if _, ok := y.(*types.TypeParam); ok {
		return true
	}
	if types.Identical(x, y) {
		return true
	}
	switch a := x.(type) {
	case *types.Pointer:
		b, ok := y.(*types.Pointer)
		return ok && typeMatch(a.Elem(), b.Elem())
	case *types.Slice:
		b, ok := y.(*types.Slice)
		return ok && typeMatch(a.Elem(), b.Elem())
	case *types.Array:
		b, ok := y.(*types.Array)
		return ok && a.Len() == b.Len() && typeMatch(a.Elem(), b.Elem())
	case *types.Map:
		b, ok := y.(*types.Map)
		return ok && typeMatch(a.Key(), b.Key()) && typeMatch(a.Elem(), b.Elem())
	case *types.Chan:
		b, ok := y.(*types.Chan)
		return ok && a.Dir() == b.Dir() && typeMatch(a.Elem(), b.Elem())
	case *types.Signature:
		b, ok := y.(*types.Signature)
		return ok && sameSig(a, b)
	case *types.Named:
		b, ok := y.(*types.Named)
		if !ok || a.Origin() != b.Origin() || a.TypeArgs().Len() != b.TypeArgs().Len() {
			return false
		}
		for i := 0; i < a.TypeArgs().Len(); i++ {
			if !typeMatch(a.TypeArgs().At(i), b.TypeArgs().At(i)) {
				return false
			}
		}
		return true
	case *types.Struct, *types.Interface, *types.Tuple:
		// a literal struct or interface type that mentions a type parameter: not compared
		// structurally, taken to match when either side mentions one
		return mentionsTP(x) || mentionsTP(y)
	}
	return false
}

func mentionsTP(t types.Type) bool {
	found := false
	seen := map[types.Type]bool{}
	var walk func(t types.Type)
	walk = func(t types.Type) {
		if t == nil || found || seen[t] {
			return
		}
		seen[t] = true
		switch u := types.Unalias(t).(type) {
		case *types.TypeParam:
			found = true
		case *types.Pointer:
			walk(u.Elem())
		case *types.Slice:
			walk(u.Elem())
		case *types.Array:
			walk(u.Elem())
		case *types.Map:
			walk(u.Key())
			walk(u.Elem())
		case *types.Chan:
			walk(u.Elem())
		case *types.Named:
			for i := 0; i < u.TypeArgs().Len(); i++ {
				walk(u.TypeArgs().At(i))
			}
		case *types.Signature:
			for i := 0; i < u.Params().Len(); i++ {
				walk(u.Params().At(i).Type())
			}
			for i := 0; i < u.Results().Len(); i++ {
				walk(u.Results().At(i).Type())
			}
		case *types.Struct:
			for i := 0; i < u.NumFields(); i++ {
				walk(u.Field(i).Type())
			}
		case *types.Interface:
			for i := 0; i < u.NumMethods(); i++ {
				walk(u.Method(i).Type())
			}
		}
	}
	walk(t)
	return found
}

var calleeCache = map[string][]ast.Node{}

// valueCallees: the function values that a site (a call of a func value, a range over one, a
// func handed to outside code) of signature sig at node site can run. In generic code the
// signature is taken at every instantiation of the enclosing generic function or type that the
// program makes (instances.go); where those are not known, with its type parameters standing
// for any type.
func valueCallees(sig *types.Signature, site ast.Node) []ast.Node {
	if !mentionsTP(sig) {
		return matching(sig)
	}
	tps, tuples, ok := contextOf(site)
	if !ok || len(tuples) == 0 {
		return matching(sig)
	}
	if curInst_ != nil { // reflip31: inside a literal reached at one instantiation of its generic code
		all := true
		for _, tp := range tps {
			if _, ok := curInst_[tp]; !ok {
				all = false
			}
		}
		if all {
			return matching(subst(sig, curInst_).(*types.Signature))
		}
	}
	seen := map[ast.Node]bool{}
	var out []ast.Node
	for _, tu := range tuples {
		m := map[*types.TypeParam]types.Type{}
		for i, tp := range tps {
			m[tp] = tu[i]
		}
		for _, n := range matching(subst(sig, m).(*types.Signature)) {
			if !seen[n] {
				seen[n] = true
				out = append(out, n)
			}
		}
	}
	return out
}

func matching(sig *types.Signature) []ast.Node {
	key := types.TypeString(types.NewSignatureType(nil, nil, nil, sig.Params(), sig.Results(), sig.Variadic()), nil)
	if mentionsTP(sig) {
		key = "" // type parameters print alike: not cached
	} else if r, ok := calleeCache[key]; ok {
		return r
	}
	var out []ast.Node
	for _, v := range valueFuncs {
		if lit, ok := v.node.(*ast.FuncLit); ok && mentionsTP(v.sig) { // reflip31: a literal of generic code, per instantiation
			if tps, tuples, ok := contextOf(v.site); ok && len(tuples) > 0 {
				for _, tu := range tuples {
					m := map[*types.TypeParam]types.Type{}
					for i, tp := range tps {
						m[tp] = tu[i]
					}
					if sameSig(subst(v.sig, m).(*types.Signature), sig) {
						out = append(out, instLitOf_(lit, tps, m))
					}
				}
				continue
			}
		}
		for _, vs := range v.sigs() {
			if sameSig(vs, sig) {
				out = append(out, v.node)
				break
			}
		}
	}
	if key != "" {
		calleeCache[key] = out
	}
	return out
}

// edges: the callees of the code in n (a statement, expression or function body), and the
// fields it reads.
func edges(n ast.Node, root ast.Node) ([]ast.Node, map[*types.Var]bool) {
	var out []ast.Node
	reads := map[*types.Var]bool{}
	ast.Inspect(n, func(c ast.Node) bool {
		switch x := c.(type) {
		case *ast.FuncLit:
			if x != root {
				if p, ok := rw.Parent[x].(*ast.CallExpr); !ok || p.Fun != ast.Expr(x) {
					return false // made, not called: reached from the calls of func values
				}
			}
		case *ast.SelectorExpr:
			if s := rw.Info.Selections[x]; s != nil && s.Kind() == types.FieldVal {
				if a, ok := rw.Parent[x].(*ast.AssignStmt); !ok || !isLhs(a, x) {
					reads[s.Obj().(*types.Var)] = true
				}
			}
		case *ast.RangeStmt:
			if sig, ok := types.Unalias(rw.Info.TypeOf(x.X)).Underlying().(*types.Signature); ok {
				out = append(out, valueCallees(sig, x)...)
			}
		case *ast.CallExpr:
			if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
				return true
			}
			fun := rw.Unparen(x.Fun)
			if _, ok := fun.(*ast.FuncLit); ok {
				return true
			}
			var fn *types.Func
			var recvIface bool
			switch f := fun.(type) {
			case *ast.SelectorExpr:
				if s := rw.Info.Selections[f]; s != nil {
					if s.Kind() == types.FieldVal {
						break
					}
					fn, _ = s.Obj().(*types.Func)
					if fn != nil {
						// the method's own receiver: an interface's (also when promoted through
						// an embedded interface) or a type parameter's constraint
						if r := fn.Type().(*types.Signature).Recv(); r != nil {
							_, recvIface = r.Type().Underlying().(*types.Interface)
						}
						if _, ok := s.Recv().(*types.TypeParam); ok {
							recvIface = true
						}
					}
				} else {
					fn, _ = rw.Info.Uses[f.Sel].(*types.Func)
				}
			default:
				fn, _ = fnOf(fun)
			}
			if fn != nil {
				if fs, ok := fun.(*ast.SelectorExpr); ok && fnForwarder_(fn) { // reflip31: x.m(..) of a function type is x(..)
					if xs, ok := rw.Unparen(fs.X).(*ast.SelectorExpr); ok && fieldCalled(xs, root, &out) {
						return true // a func-typed field whose stores are all known (fieldfuncs.go)
					}
					if sig, ok := types.Unalias(rw.Info.TypeOf(fs.X)).Underlying().(*types.Signature); ok {
						out = append(out, valueCallees(sig, x)...)
						return true
					}
				}
				if tgt, ok := objCallEdges(x, fn); ok {
					out = append(out, tgt...)
					return true
				}
				if recvIface {
					out = append(out, implementers(fn)...)
				} else if fd := funcDecls[fn.Origin()]; fd != nil {
					out = append(out, fd)
				}
				if fn.Pkg() != rw.Pkg {
					// code outside the package calls the funcs it is given, and the methods of
					// the values it is given (fmt's String / Error / Format, sort's Less / Swap /
					// Len, io's Read / Write / Close)
					for _, a := range x.Args {
						t := rw.Info.TypeOf(a)
						if sig, ok := types.Unalias(t).Underlying().(*types.Signature); ok {
							out = append(out, valueCallees(sig, x)...)
							continue
						}
						if p, ok := types.Unalias(t).(*types.Pointer); ok {
							t = p.Elem()
						}
						if n, ok := types.Unalias(t).(*types.Named); ok && n.Obj().Pkg() == rw.Pkg {
							if _, isI := n.Underlying().(*types.Interface); !isI {
								for i := 0; i < n.NumMethods(); i++ {
									if fd := funcDecls[n.Method(i)]; fd != nil {
										out = append(out, fd)
									}
								}
								continue
							}
						}
						if _, isI := types.Unalias(t).Underlying().(*types.Interface); isI {
							for _, m := range []string{"String", "Error", "Format", "GoString", "Less", "Swap", "Len", "Read", "Write", "Close", "Unwrap", "Is", "As"} {
								out = append(out, methodsBy[m]...)
							}
						}
					}
				}
			} else if fs, isSel := fun.(*ast.SelectorExpr); isSel && fieldCalled(fs, root, &out) {
				// a func-typed field whose stores are all known (fieldfuncs.go)
			} else if _, isB := rw.Info.Uses[identOf(fun)].(*types.Builtin); !isB {
				if sig, ok := types.Unalias(rw.Info.TypeOf(fun)).Underlying().(*types.Signature); ok {
					out = append(out, valueCallees(sig, x)...)
				}
			}
		}
		return true
	})
	return out, reads
}

func identOf(e ast.Expr) *ast.Ident {
	id, _ := e.(*ast.Ident)
	return id
}

func isLhs(a *ast.AssignStmt, e ast.Expr) bool {
	for _, l := range a.Lhs {
		if l == e {
			return a.Tok == 0 || a.Tok.String() == "=" || a.Tok.String() == ":="
		}
	}
	return false
}

func nodeEdges(n ast.Node) []ast.Node {
	if !chaDone[n] {
		chaDone[n] = true
		if il, ok := n.(*instLit_); ok { // reflip31
			curInst_ = il.m
		}
		out, reads := edges(bodyOf(n), n)
		curInst_ = nil
		chaNodes[n] = out
		readsField[n] = reads
	}
	return chaNodes[n]
}

// mayRead reports whether running code c (a statement or expression) can read field f.
func mayRead(c ast.Node, f *types.Var) bool {
	buildCHA()
	start, reads := edges(c, nil)
	if reads[f] {
		return true
	}
	seen := map[ast.Node]bool{}
	from := map[ast.Node]ast.Node{}
	work := append([]ast.Node(nil), start...)
	for len(work) > 0 {
		n := work[0]
		work = work[1:]
		if seen[n] || bodyOf(n) == nil {
			continue
		}
		seen[n] = true
		next := nodeEdges(n)
		if readsField[n][f] {
			if os.Getenv("OPTSTR_PATH") != "" {
				fmt.Fprintf(os.Stderr, "path to a read of %s:\n", f.Name())
				for p := n; p != nil; p = from[p] {
					fmt.Fprintf(os.Stderr, "    %s %s\n", rw.Fset.Position(p.Pos()), nodeName(p))
				}
			}
			return true
		}
		for _, x := range next {
			if !seen[x] {
				if _, ok := from[x]; !ok {
					from[x] = n
				}
			}
		}
		work = append(work, next...)
	}
	return false
}

func nodeName(n ast.Node) string {
	if fd, ok := n.(*ast.FuncDecl); ok {
		return fd.Name.Name
	}
	if b, ok := n.(*boundFn); ok {
		return b.Name.Name + " (bound to its literal)"
	}
	return "func literal"
}

var implCache = map[string][]ast.Node{}

// implementers: the methods named like fn of the package's types that implement the interface
// the call is made on (all methods of the name for a type parameter's constraint).
func implementers(fn *types.Func) []ast.Node {
	var recv types.Type
	if r := fn.Type().(*types.Signature).Recv(); r != nil {
		recv = r.Type()
	}
	if recv == nil {
		return methodsBy[fn.Name()]
	}
	iface, ok := recv.Underlying().(*types.Interface)
	if !ok {
		return methodsBy[fn.Name()]
	}
	key := types.TypeString(recv, nil) + "." + fn.Name()
	if r, ok := implCache[key]; ok {
		return r
	}
	var out []ast.Node
	for _, m := range methodsBy[fn.Name()] {
		fd := m.(*ast.FuncDecl)
		mf := rw.Info.Defs[fd.Name].(*types.Func)
		rt := mf.Type().(*types.Signature).Recv().Type()
		if p, ok := rt.(*types.Pointer); ok {
			rt = p.Elem()
		}
		named, ok := types.Unalias(rt).(*types.Named)
		if !ok {
			continue
		}
		if named.TypeParams().Len() > 0 {
			out = append(out, m) // a generic type: its instances are not enumerated
			continue
		}
		if types.Implements(named, iface) || types.Implements(types.NewPointer(named), iface) || embedsImpl(named, iface) {
			out = append(out, m)
		}
	}
	implCache[key] = out
	return out
}

// embedsImpl: a type that embeds named implements the interface (the method is promoted).
func embedsImpl(named *types.Named, iface *types.Interface) bool {
	for tn := range supers {
		if tn.Type() != named && below(tn, named.Obj()) {
			t := tn.Type()
			if types.Implements(t, iface) || types.Implements(types.NewPointer(t), iface) {
				return true
			}
		}
	}
	return false
}

func fieldCalled(sel *ast.SelectorExpr, root ast.Node, out *[]ast.Node) bool {
	srcs, ok := fieldCallees(sel, root)
	if ok {
		*out = append(*out, srcs...)
	}
	return ok
}

// fnForwarder_ (reflip31) reports whether fn is a method of a named function type whose body only calls the receiver with
// the method's parameters: the methods the translator's rule "function types" gives its function types (f.apply(x) is f(x)).
func fnForwarder_(fn *types.Func) bool {
	fn = fn.Origin()
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	if _, ok := types.Unalias(sig.Recv().Type()).Underlying().(*types.Signature); !ok {
		return false
	}
	fd := funcDecls[fn]
	if fd == nil || fd.Body == nil || len(fd.Body.List) != 1 || fd.Recv == nil || len(fd.Recv.List) != 1 || len(fd.Recv.List[0].Names) != 1 {
		return false
	}
	var e ast.Expr
	switch s := fd.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(s.Results) != 1 {
			return false
		}
		e = s.Results[0]
	case *ast.ExprStmt:
		e = s.X
	default:
		return false
	}
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := c.Fun.(*ast.Ident)
	if !ok || id.Name != fd.Recv.List[0].Names[0].Name {
		return false
	}
	n := 0
	for _, p := range fd.Type.Params.List {
		for _, pn := range p.Names {
			if n >= len(c.Args) {
				return false
			}
			if a, ok := c.Args[n].(*ast.Ident); !ok || a.Name != pn.Name {
				return false
			}
			n++
		}
	}
	return n == len(c.Args)
}

// instLit_ (reflip31) is a function literal written in generic code, as reached at one instantiation of that code (the type
// arguments m): a call of a func value inside it whose type is written with the code's type parameters has that instantiation's
// signature, not every instantiation's. Suppliers.memoize's closure reached as a func() string calls its s as a func() string.
type instLit_ struct {
	*ast.FuncLit
	m map[*types.TypeParam]types.Type
}

var (
	instLits_ = map[string]*instLit_{}
	curInst_  map[*types.TypeParam]types.Type
)

func instLitOf_(lit *ast.FuncLit, tps []*types.TypeParam, m map[*types.TypeParam]types.Type) *instLit_ {
	key := fmt.Sprint(lit.Pos())
	for _, tp := range tps {
		key += "|" + types.TypeString(m[tp], nil)
	}
	if il, ok := instLits_[key]; ok {
		return il
	}
	il := &instLit_{FuncLit: lit, m: m}
	instLits_[key] = il
	return il
}
