// Command tfnparams is a post-flip step: a parameter of a Java functional interface type
// (Supplier, Function, Predicate, Consumer, ...) that every call gives a typed lambda adapter
// of one type, and that the function only calls, takes the adapter's typed function.
//
//	func (this *WritingLogFile) log_impl(value SupplierOf[string]) *LogFile { return this.logInternal(str_(value.get())) }
//	log.log(&Supplier_tfn_string{func() string { return "..." }})
//	  ->
//	func (this *WritingLogFile) log_impl(value func() string) *LogFile { return this.logInternal(value()) }
//	log.log(func() string { return "..." })
//
// Java declares the parameter with the library's interface (LogFile.log(Supplier<String>)),
// which the runtime keeps erased: the typed translator writes each lambda passed there as an
// adapter object &X_tfn_T{f} whose method unboxes its arguments, calls f and boxes the result,
// and the callee unboxes that result again. Where the callers are typed already (every call
// passes an adapter of one type X), the parameter's type is X's function type and the callee
// calls it: a call p.m(a...) is the body of X.m with this.f -> p and its parameters -> a...
// (the adapter's method is what the call ran), and a cast K of it that undoes the method's
// boxing B (str_ of strobj_, cast_[T] of obj_, Node_of_ of box_()) is the call p(a...) itself.
// The adapter was a new object, never null, and could only be called: nothing else could see
// it, so Java's behaviour is the same, and the adapter object and its boxes are not made.
//
// The functions converted together: a function; or a method and everything a call of it may
// reach, as for the dispatch of a typed self_ (LogFile.log calls this.self_.log_impl(p0)): the
// methods named m and m_impl of the classes of its hierarchy and the method of the dispatch
// interface, as one parameter. No other interface may list them (an interface call from
// elsewhere could reach them), none may be generic or used other than by calling it, every
// call of any of them passes an adapter literal of the one type X, or the group's parameter
// itself (a dispatch, a forwarding), and every use of the parameter in their bodies is a call
// of a method X has (with arguments that compute nothing when there are several) or such a
// forwarding. Everything else is left and counted. A rerun changes nothing.
//
// Two more arguments are accepted besides the adapter literal. An adapter made by an immediately
// called literal (func() *X { ...; return &X{f} }()) is the literal returning f (result type X's
// function type). A named object of a class C that has the typed method mT_ of the adapter's
// method m (implsplit: javac's typed method, m its bridge boxing mT_'s result as the adapter's m
// boxes f's) is the method value o.mT_, where o is never null: a constructor call new_C(..), or a
// call of a function whose body only returns such a call (RemoveUnusedCode's RemovalLogRecord
// factories). The method value evaluates o where the argument was evaluated, and a call of it
// is the call of the bridge with the boxing undone, as for the adapter. -n -v lists every other
// parameter of a functional interface type by the reason it stays (a runtime value such as
// Predicates.alwaysTrue(); stored in a field; listed by a Java interface).
//
// A runtime value with a typed variant (Predicates.alwaysTrue() given where the adapter's
// function is a func(T) bool: Predicates_alwaysTrueT_[T], the runtime's) is that variant: it is
// only called there, so the singleton's identity is not needed. A package variable given to the
// group (NodeUtil.MATCH_NOT_FUNCTION) whose every use is an argument of the group's calls or an
// assignment of an adapter literal (its static initializer) holds the function: its type is the
// function type and the adapter literals are their functions; groups given the same variable
// are converted as one. A null variable (read before its initializer) fails at the call as the
// nil interface did. In the inlined adapter method, the unboxing K(p) of a parameter given the
// box B(e) that K undoes (Node_of_(n.box_())) is e.
//
// usage: tfnparams [-n] [-v] <dir of package closure>
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

var qual = func(p *types.Package) string {
	if p == rw.Pkg {
		return ""
	}
	return p.Name()
}

type group struct {
	key     string
	idx     int
	idxOf   map[*types.Func]int   // the parameter's index in each function of the group
	callIdx map[*ast.CallExpr]int // the parameter's index in each call
	ifaceIx []int                 // the parameter's index in each interface method field
	funcs   []*types.Func         // the functions/methods (and the interface method) converted together
	decls   []*ast.FuncDecl       // their declarations (not the interface method)
	iface   []*ast.Field          // the interface method fields
	params  []*types.Var          // the parameter in each declaration
	calls   []*ast.CallExpr       // calls of any of them
	adapter *types.Named          // X
	ftype   *types.Signature      // X's function type
	why     string
	uses    []*ast.Ident
	named   []ast.Expr   // arguments that are named objects with the typed method <method>T_
	method  string       // the adapter's method
	rtvals  []ast.Expr   // arguments that are runtime values with a typed variant
	vars    []*types.Var // package variables given as arguments, typed with the group
}

var (
	declOf  = map[*types.Func]*ast.FuncDecl{}
	ifaceOf = map[*types.Func]*ast.Field{} // interface method -> its field
	verbose bool
	counts  = map[string]int{}
	lefts   = map[string][]string{}
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
			switch x := d.(type) {
			case *ast.FuncDecl:
				if fo, ok := rw.Info.Defs[x.Name].(*types.Func); ok {
					declOf[fo] = x
				}
			case *ast.GenDecl:
				for _, sp := range x.Specs {
					ts, ok := sp.(*ast.TypeSpec)
					if !ok {
						continue
					}
					it, ok := ts.Type.(*ast.InterfaceType)
					if !ok {
						continue
					}
					for _, m := range it.Methods.List {
						if len(m.Names) == 1 {
							if fo, ok := rw.Info.Defs[m.Names[0]].(*types.Func); ok {
								ifaceOf[fo] = m
							}
						}
					}
				}
			}
		}
	}
	groups := findGroups()
	var done []*group
	for _, g := range groups {
		analyse(g)
		if g.why != "" {
			left(g.why, g.key)
			continue
		}
		done = append(done, g)
	}
	sort.Slice(done, func(i, j int) bool { return done[i].key < done[j].key })
	for _, g := range done {
		rewrite(g)
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", counts[k], k)
	}
	if verbose {
		for _, g := range done {
			fmt.Fprintf(os.Stderr, "        %s#%d: %s, %d calls\n", g.key, g.idx, types.TypeString(g.ftype, qual), len(g.calls))
		}
	}
	keys = keys[:0]
	for k := range lefts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", len(lefts[k]), k)
		if verbose {
			for _, p := range lefts[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	fmt.Fprintf(os.Stderr, "tfnparams: %d parameters (groups) take the typed function\n", len(done))
	if report || len(done) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "tfnparams: files changed: %d\n", rw.SaveChecked())
}

func left(why, where string) { lefts[why] = append(lefts[why], where) }

func generated(n ast.Node) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "g_")
}

// fnIface: an interface type of the package whose methods are a Java functional interface's
// (j2go's stubs: the lambda adapters X_tfn_.. implement it).
func fnIface(t types.Type) *types.Named {
	nt, ok := types.Unalias(t).(*types.Named)
	if !ok || nt.Obj().Parent() != rw.Pkg.Scope() {
		return nil
	}
	it, ok := nt.Underlying().(*types.Interface)
	if !ok || it.NumMethods() == 0 || it.NumMethods() > 2 {
		return nil
	}
	return nt
}

// adapterOf: &X{funclit} with X a typed lambda adapter (one func field f).
func adapterOf(e ast.Expr) (*types.Named, *ast.FuncLit) {
	u, ok := rw.Unparen(e).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return nil, nil
	}
	cl, ok := u.X.(*ast.CompositeLit)
	if !ok || len(cl.Elts) != 1 {
		return nil, nil
	}
	id, ok := cl.Type.(*ast.Ident)
	if !ok || !strings.Contains(id.Name, "_tfn") {
		return nil, nil
	}
	fl, ok := cl.Elts[0].(*ast.FuncLit)
	if !ok {
		return nil, nil
	}
	tn, ok := rw.Info.Uses[id].(*types.TypeName)
	if !ok {
		return nil, nil
	}
	nt := tn.Type().(*types.Named)
	st, ok := nt.Underlying().(*types.Struct)
	if !ok || st.NumFields() != 1 {
		return nil, nil
	}
	return nt, fl
}

// findGroups: the parameters of functional interface types, grouped by the dispatch.
func findGroups() []*group {
	byKey := map[string]*group{}
	var out []*group
	for fo, fd := range declOf {
		if !generated(fd) || fd.Type.TypeParams != nil || fd.Body == nil {
			continue
		}
		sig := fo.Type().(*types.Signature)
		if sig.RecvTypeParams().Len() > 0 {
			continue
		}
		for i := 0; i < sig.Params().Len(); i++ {
			if fnIface(sig.Params().At(i).Type()) == nil || (sig.Variadic() && i == sig.Params().Len()-1) {
				continue
			}
			base := strings.TrimSuffix(fo.Name(), "_impl")
			key := fo.FullName()
			if sig.Recv() != nil {
				key = "method " + base
			}
			k := fmt.Sprintf("%s#%d", key, i)
			g := byKey[k]
			if g == nil {
				g = &group{key: key, idx: i, idxOf: map[*types.Func]int{}, callIdx: map[*ast.CallExpr]int{}}
				byKey[k] = g
				out = append(out, g)
			}
			g.idxOf[fo] = i
			g.funcs = append(g.funcs, fo)
			g.decls = append(g.decls, fd)
			g.params = append(g.params, sig.Params().At(i))
		}
	}
	// interface methods of the names of method groups
	for fo, fld := range ifaceOf {
		base := strings.TrimSuffix(fo.Name(), "_impl")
		sig := fo.Type().(*types.Signature)
		for i := 0; i < sig.Params().Len(); i++ {
			if g := byKey[fmt.Sprintf("method %s#%d", base, i)]; g != nil {
				g.idxOf[fo] = i
				g.funcs = append(g.funcs, fo)
				g.iface = append(g.iface, fld)
				g.ifaceIx = append(g.ifaceIx, i)
			}
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].key+fmt.Sprint(out[i].idx) < out[j].key+fmt.Sprint(out[j].idx) })
	for _, g := range out {
		checkNames(g)
	}
	return merge(out)
}

// merge: groups whose parameters are passed on to one another (Compiler.logToFile passes its
// supplier to LogFile.log) are converted as one.
func merge(gs []*group) []*group {
	of := map[*types.Func]*group{}
	for _, g := range gs {
		for fo := range g.idxOf {
			of[fo] = g
		}
	}
	parent := map[*group]*group{}
	var find func(g *group) *group
	find = func(g *group) *group {
		for parent[g] != nil {
			g = parent[g]
		}
		return g
	}
	for _, g := range gs {
		for i, fd := range g.decls {
			p := g.params[i]
			for _, id := range rw.UsesOf[p] {
				if !(id.Pos() >= fd.Pos() && id.Pos() < fd.End()) {
					continue
				}
				c, idx := argOf(id)
				if c == nil {
					continue
				}
				fo, _ := callee(c).(*types.Func)
				h := of[fo]
				if h == nil || h.idxOf[fo] != idx {
					continue
				}
				if a, b := find(g), find(h); a != b {
					parent[b] = a
				}
			}
		}
	}
	// groups given the same package variable (NodeUtil.MATCH_NOT_FUNCTION to has, findPreorder
	// and visitPreOrder) are converted as one: the variable takes the function type for all
	byVar := map[*types.Var]*group{}
	for _, g := range gs {
		for fo, i := range g.idxOf {
			for _, id := range rw.UsesOf[fo] {
				var fun ast.Node = id
				if s, ok := rw.Parent[id].(*ast.SelectorExpr); ok && s.Sel == id {
					fun = s
				}
				c, ok := rw.Up(fun).(*ast.CallExpr)
				if !ok || rw.Unparen(c.Fun) != fun || i >= len(c.Args) {
					continue
				}
				v := pkgVar(c.Args[i])
				if v == nil {
					continue
				}
				if h := byVar[v]; h != nil {
					if a, b := find(h), find(g); a != b {
						parent[b] = a
					}
				} else {
					byVar[v] = g
				}
			}
		}
	}
	var out []*group
	merged := map[*group]*group{}
	for _, g := range gs {
		r := find(g)
		if r == g {
			out = append(out, g)
			continue
		}
		merged[g] = r
	}
	for _, g := range gs {
		r, ok := merged[g]
		if !ok {
			continue
		}
		r.key += " + " + g.key
		r.funcs = append(r.funcs, g.funcs...)
		r.decls = append(r.decls, g.decls...)
		r.params = append(r.params, g.params...)
		r.iface = append(r.iface, g.iface...)
		r.ifaceIx = append(r.ifaceIx, g.ifaceIx...)
		for fo, i := range g.idxOf {
			r.idxOf[fo] = i
		}
		if g.why != "" && r.why == "" {
			r.why = g.why
		}
	}
	return out
}

func inGroup(g *group, o types.Object) bool {
	for _, f := range g.funcs {
		if f == o {
			return true
		}
	}
	return false
}

func isGroupParam(g *group, o types.Object) bool {
	for _, p := range g.params {
		if p == o {
			return true
		}
	}
	return false
}

// checkNames: a method group's methods m / m_impl are listed by no interface but the dispatch
// interface of their classes.
func checkNames(g *group) {
	// a method group: the methods m / m_impl of the package, all of them generated, and the
	// interfaces listing them only the dispatch interfaces of their classes
	if strings.HasPrefix(g.key, "method ") {
		name := strings.TrimPrefix(g.key, "method ")
		for fo, fld := range ifaceOf {
			n := fo.Name()
			if (n == name || n == name+"_impl") && !inGroup(g, fo) {
				_ = fld
				g.why = "an interface lists the method with another signature"
				return
			}
		}
		for _, fo := range g.funcs {
			if _, ok := ifaceOf[fo]; ok && !strings.HasSuffix(fo.Name(), "_impl") {
				g.why = "a Java interface lists the method (an interface call may reach it)"
				return
			}
		}
	}
}

// analyse decides a group: its calls and the adapter of all of them, the uses of its parameter.
func analyse(g *group) {
	if g.why != "" {
		return
	}
	// the calls
	for _, f := range g.funcs {
		for _, id := range rw.UsesOf[f] {
			var fun ast.Node = id
			if s, ok := rw.Parent[id].(*ast.SelectorExpr); ok && s.Sel == id {
				fun = s
			}
			c, ok := rw.Up(fun).(*ast.CallExpr)
			if !ok || rw.Unparen(c.Fun) != fun {
				g.why = "used other than by calling it"
				return
			}
			if !generated(c) {
				g.why = "called by the runtime"
				return
			}
			if g.idxOf[f] >= len(c.Args) {
				g.why = "a call without the argument"
				return
			}
			g.calls = append(g.calls, c)
			g.callIdx[c] = g.idxOf[f]
		}
	}
	// the arguments: adapters of one type, or the group's own parameter
	for _, c := range g.calls {
		a := c.Args[g.callIdx[c]]
		if id, ok := rw.Unparen(a).(*ast.Ident); ok && isGroupParam(g, rw.Info.Uses[id]) {
			continue
		}
		x, _ := adapterOf(a)
		if x == nil {
			if x, _ = iifeOf(a); x == nil {
				if namedObj(a) != nil {
					g.named = append(g.named, a)
					continue
				}
				if rtTyped(a) != "" {
					g.rtvals = append(g.rtvals, a)
					continue
				}
				if v := pkgVar(a); v != nil {
					if !hasVar(g, v) {
						g.vars = append(g.vars, v)
					}
					continue
				}
				g.why = "given another value than a typed lambda adapter"
				return
			}
		}
		if g.adapter == nil {
			g.adapter = x
		} else if g.adapter != x {
			g.why = "given adapters of two types"
			return
		}
	}
	// the package variables: every use is an argument of the group's calls or an assignment of
	// an adapter literal of the group's adapter type
	for _, v := range g.vars {
		for _, id := range rw.UsesOf[v] {
			if c, idx := argOf(id); c != nil {
				if i, ok := g.callIdx[c]; ok && i == idx {
					continue
				}
			}
			as, ok := rw.Parent[id].(*ast.AssignStmt)
			if !ok || len(as.Lhs) != 1 || as.Lhs[0] != id || as.Tok != token.ASSIGN {
				g.why = "given a package variable that is used otherwise"
				if verbose {
					fmt.Fprintf(os.Stderr, "        (%s: %s)\n", v.Name(), rw.Fset.Position(id.Pos()))
				}
				return
			}
			x, _ := adapterOf(as.Rhs[0])
			if x == nil {
				g.why = "given a package variable assigned another value than an adapter"
				return
			}
			if g.adapter == nil {
				g.adapter = x
			} else if g.adapter != x {
				g.why = "given adapters of two types"
				return
			}
		}
		if len(rw.UsesOf[v]) == 0 {
			g.why = "given a package variable that is never assigned"
			return
		}
	}
	if g.adapter == nil && len(g.rtvals) > 0 {
		g.adapter = adapterFor(g.params[0].Type())
	}
	if g.adapter == nil {
		g.why = "never given an adapter"
		return
	}
	g.ftype = g.adapter.Underlying().(*types.Struct).Field(0).Type().(*types.Signature)
	// the uses of the parameter
	for i, fd := range g.decls {
		p := g.params[i]
		for _, id := range rw.UsesOf[p] {
			if !(id.Pos() >= fd.Pos() && id.Pos() < fd.End()) {
				continue
			}
			g.uses = append(g.uses, id)
			if c, idx := argOf(id); c != nil {
				if fo, ok := callee(c).(*types.Func); ok && inGroup(g, fo) && idx == g.idxOf[fo] {
					continue
				}
			}
			s, ok := rw.Parent[id].(*ast.SelectorExpr)
			if !ok || s.X != id {
				g.why = "the parameter is used other than by calling it"
				return
			}
			c, ok := rw.Parent[s].(*ast.CallExpr)
			if !ok || c.Fun != s {
				g.why = "the parameter's method is used as a value"
				return
			}
			md := methodDecl(g.adapter, s.Sel.Name)
			if md == nil {
				g.why = "a method the adapter does not have: " + s.Sel.Name
				return
			}
			if inlineable(md) == nil {
				g.why = "the adapter's method is not a call of its function"
				return
			}
			if len(c.Args) > 1 {
				for _, a := range c.Args {
					if !pure(a) {
						g.why = "a call with arguments that compute"
						return
					}
				}
			}
		}
	}
	for _, a := range g.rtvals {
		if why := rtTypedFor(g, a); why != "" {
			g.why = why
			return
		}
	}
	// the named objects: a class with the typed method of the adapter's method and its bridge
	if len(g.named) > 0 {
		// the adapter's method the group calls (one name), else its only method
		names := map[string]bool{}
		for _, id := range g.uses {
			if s, ok := rw.Parent[id].(*ast.SelectorExpr); ok && s.X == id {
				names[s.Sel.Name] = true
			}
		}
		if len(names) == 0 {
			ms := types.NewMethodSet(types.NewPointer(g.adapter))
			if ms.Len() != 1 {
				g.why = "given a named object, the parameter never called and the adapter has several methods"
				return
			}
			names[ms.At(0).Obj().Name()] = true
		}
		if len(names) != 1 {
			g.why = "given a named object and two of the adapter's methods called"
			return
		}
		for n := range names {
			g.method = n
		}
		for _, a := range g.named {
			if why := typedMethodOf(g, namedObj(a)); why != "" {
				g.why = why
				return
			}
		}
	}
}

func argOf(id *ast.Ident) (*ast.CallExpr, int) {
	c, ok := rw.Up(id).(*ast.CallExpr)
	if !ok {
		return nil, -1
	}
	for i, a := range c.Args {
		if rw.Unparen(a) == id {
			return c, i
		}
	}
	return nil, -1
}

func callee(c *ast.CallExpr) types.Object {
	switch f := rw.Unparen(c.Fun).(type) {
	case *ast.Ident:
		return rw.Info.Uses[f]
	case *ast.SelectorExpr:
		if s := rw.Info.Selections[f]; s != nil {
			return s.Obj()
		}
		return rw.Info.Uses[f.Sel]
	}
	return nil
}

func methodDecl(x *types.Named, name string) *ast.FuncDecl {
	o, _, _ := types.LookupFieldOrMethod(types.NewPointer(x), true, rw.Pkg, name)
	fo, ok := o.(*types.Func)
	if !ok {
		return nil
	}
	fd := declOf[fo]
	if fd == nil || fd.Body == nil || len(fd.Body.List) != 1 {
		return nil
	}
	return fd
}

// inlineable: the adapter method's body expression, `return E` or `E`, in which this.f is
// called once and each parameter appears once.
func inlineable(fd *ast.FuncDecl) ast.Expr {
	var e ast.Expr
	switch s := fd.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(s.Results) == 1 {
			e = s.Results[0]
		}
	case *ast.ExprStmt:
		e = s.X
	}
	if e == nil {
		return nil
	}
	recv := rw.Info.Defs[fd.Recv.List[0].Names[0]]
	params := map[types.Object]int{}
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			params[rw.Info.Defs[n]] = 0
		}
	}
	fcalls := 0
	ok := true
	ast.Inspect(e, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, isId := x.X.(*ast.Ident); isId && rw.Info.Uses[id] == recv {
				if x.Sel.Name != "f" {
					ok = false
				}
				if c, isCall := rw.Parent[x].(*ast.CallExpr); isCall && c.Fun == x {
					fcalls++
				} else {
					ok = false
				}
			}
		case *ast.Ident:
			if o := rw.Info.Uses[x]; o != nil {
				if _, isParam := params[o]; isParam {
					params[o]++
				}
				if o == recv {
					if s, isSel := rw.Parent[x].(*ast.SelectorExpr); !isSel || s.X != x {
						ok = false
					}
				}
			}
		}
		return true
	})
	for _, n := range params {
		if n != 1 {
			ok = false
		}
	}
	if !ok || fcalls != 1 {
		return nil
	}
	return e
}

func pure(e ast.Expr) bool {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident, *ast.BasicLit:
		return true
	case *ast.SelectorExpr:
		if s := rw.Info.Selections[x]; s != nil && s.Kind() != types.FieldVal {
			return false
		}
		return pure(x.X)
	case *ast.CallExpr:
		// a boxing of a pure value
		if n, _ := rw.CallName(x); (n == "strobj_" || strings.HasPrefix(n, "obj_")) && len(x.Args) == 1 {
			return pure(x.Args[0])
		}
		if s, ok := x.Fun.(*ast.SelectorExpr); ok && len(x.Args) == 0 && (s.Sel.Name == "box_" || s.Sel.Name == "self") {
			return pure(s.X)
		}
	}
	return false
}

// unboxes: the cast K undoes the boxing B of a value of type t.
func unboxes(k *ast.CallExpr, b *ast.CallExpr, t types.Type) bool {
	kn, _ := rw.CallName(k)
	bn, _ := rw.CallName(b)
	if s, ok := b.Fun.(*ast.SelectorExpr); ok && len(b.Args) == 0 {
		bn = "." + s.Sel.Name
	}
	switch {
	case kn == "str_" && bn == "strobj_":
		return types.Identical(t, types.Typ[types.String])
	case kn == "Node_of_" && bn == ".box_":
		return true
	case strings.HasPrefix(kn, "cast_[") && strings.HasPrefix(bn, "obj_"):
		if sig, ok := rw.Info.TypeOf(k.Fun).(*types.Signature); ok {
			_, isPtr := t.(*types.Pointer)
			return isPtr && types.Identical(sig.Results().At(0).Type(), t)
		}
	}
	return false
}

func rewrite(g *group) {
	ft := types.TypeString(g.ftype, qual)
	counts["parameters of a functional interface take the typed function"] += len(g.decls) + len(g.iface)
	// the parameter declarations
	for i, fd := range g.decls {
		p := g.params[i]
		for _, fl := range fd.Type.Params.List {
			for _, n := range fl.Names {
				if rw.Info.Defs[n] == p {
					fl := fl
					if len(fl.Names) != 1 {
						panic("tfnparams: a parameter declared with others: " + rw.Fset.Position(fl.Pos()).String())
					}
					rw.Rewrite[fl.Type] = func() string { return ft }
				}
			}
		}
	}
	for k, fld := range g.iface {
		want := g.ifaceIx[k]
		ftp := fld.Type.(*ast.FuncType)
		idx := 0
		for _, pf := range ftp.Params.List {
			n := len(pf.Names)
			if n == 0 {
				n = 1
			}
			if want >= idx && want < idx+n {
				if n != 1 {
					panic("tfnparams: an interface parameter declared with others")
				}
				pf := pf
				rw.Rewrite[pf.Type] = func() string { return ft }
			}
			idx += n
		}
	}
	// the arguments
	for _, c := range g.calls {
		a := c.Args[g.callIdx[c]]
		if _, fl := adapterOf(a); fl != nil {
			counts["adapter objects given to them are their functions"]++
			rw.Rewrite[a] = func() string { return rw.Render(fl) }
			continue
		}
		if _, lit := iifeOf(a); lit != nil {
			counts["adapters made by an immediately called literal are their functions"]++
			rw.Rewrite[lit.Type.Results.List[0].Type] = func() string { return ft }
			for _, r := range returnsOf(lit) {
				_, fl := adapterOf(r.Results[0])
				rw.Rewrite[r.Results[0]] = func() string { return rw.Render(fl) }
			}
		}
	}
	for _, a := range g.named {
		a := a
		counts["named objects given to them are their typed method values"]++
		rw.Rewrite[a] = func() string { return rw.RenderChildren(a) + "." + g.method + "T_" }
	}
	for _, a := range g.rtvals {
		v := rtTypedFor_(g, a)
		counts["runtime values given to them are their typed variants"]++
		rw.Rewrite[a] = func() string { return v }
	}
	for _, v := range g.vars {
		counts["package variables given to them hold the function"]++
		for _, f := range rw.Files {
			for _, d := range f.Decls {
				gd, ok := d.(*ast.GenDecl)
				if !ok || gd.Tok != token.VAR {
					continue
				}
				for _, sp := range gd.Specs {
					vs := sp.(*ast.ValueSpec)
					for _, n := range vs.Names {
						if rw.Info.Defs[n] == v {
							if len(vs.Names) != 1 || vs.Type == nil || len(vs.Values) != 0 {
								panic("tfnparams: a package variable declared with others or a value: " + rw.Fset.Position(vs.Pos()).String())
							}
							rw.Rewrite[vs.Type] = func() string { return ft }
						}
					}
				}
			}
		}
		for _, id := range rw.UsesOf[v] {
			if as, ok := rw.Parent[id].(*ast.AssignStmt); ok && as.Lhs[0] == id {
				_, fl := adapterOf(as.Rhs[0])
				counts["adapter objects assigned to them are their functions"]++
				rw.Rewrite[as.Rhs[0]] = func() string { return rw.Render(fl) }
			}
		}
	}
	// the calls of the parameter
	for _, id := range g.uses {
		s, ok := rw.Parent[id].(*ast.SelectorExpr)
		if !ok || s.X != id {
			continue
		}
		call := rw.Parent[s].(*ast.CallExpr)
		md := methodDecl(g.adapter, s.Sel.Name)
		e := inlineable(md)
		var names []types.Object
		for _, f := range md.Type.Params.List {
			for _, n := range f.Names {
				names = append(names, rw.Info.Defs[n])
			}
		}
		recv := rw.Info.Defs[md.Recv.List[0].Names[0]]
		pname := id.Name
		body := func(skipBox bool) string {
			subst := map[ast.Node]string{}
			ast.Inspect(e, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.SelectorExpr:
					if xid, ok := x.X.(*ast.Ident); ok && rw.Info.Uses[xid] == recv {
						subst[x] = pname
						return false
					}
				case *ast.CallExpr:
					// K(p) with the argument B(e) the boxing K undoes is e
					if len(x.Args) == 1 {
						if pid, ok := x.Args[0].(*ast.Ident); ok {
							for i, o := range names {
								if rw.Info.Uses[pid] == o {
									if e := unboxedArg(x, call.Args[i]); e != nil {
										counts["arguments boxed for the adapter's unboxing are passed as they are"]++
										subst[x] = rw.Render(e)
										return false
									}
								}
							}
						}
					}
				case *ast.Ident:
					for i, o := range names {
						if rw.Info.Uses[x] == o {
							subst[x] = rw.Render(call.Args[i])
						}
					}
				}
				return true
			})
			root := e
			if skipBox {
				if b := e.(*ast.CallExpr); len(b.Args) == 1 {
					root = b.Args[0]
				} else {
					root = b.Fun.(*ast.SelectorExpr).X
				}
			}
			return renderWith(root, subst)
		}
		// a cast that undoes the method's boxing
		if k, ok := rw.Up(call).(*ast.CallExpr); ok && len(k.Args) == 1 && rw.Unparen(k.Args[0]) == call {
			if b, ok := e.(*ast.CallExpr); ok && (len(b.Args) == 1 || len(b.Args) == 0) {
				inner := e
				if len(b.Args) == 1 {
					inner = b.Args[0]
				} else if s, ok := b.Fun.(*ast.SelectorExpr); ok {
					inner = s.X
				}
				if fc, ok := rw.Unparen(inner).(*ast.CallExpr); ok && g.ftype.Results().Len() == 1 && isFCall(fc, recv) && unboxes(k, b, g.ftype.Results().At(0).Type()) {
					counts["calls of the parameter under the cast that undoes the boxing are calls of the function"]++
					rw.Rewrite[k] = func() string { return body(true) }
					continue
				}
			}
		}
		counts["calls of the parameter are the adapter's method on the function"]++
		rw.Rewrite[call] = func() string { return body(false) }
	}
}

func isFCall(c *ast.CallExpr, recv types.Object) bool {
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	id, ok := s.X.(*ast.Ident)
	return ok && rw.Info.Uses[id] == recv
}

// renderWith renders n (from another declaration) with the given nodes replaced.
func renderWith(n ast.Node, subst map[ast.Node]string) string {
	saved := map[ast.Node]func() string{}
	for k, v := range subst {
		v := v
		if old, ok := rw.Rewrite[k]; ok {
			saved[k] = old
		}
		rw.Rewrite[k] = func() string { return v }
	}
	s := rw.Render(n)
	for k := range subst {
		delete(rw.Rewrite, k)
		if old, ok := saved[k]; ok {
			rw.Rewrite[k] = old
		}
	}
	return s
}

// returnsOf: the return statements of a function literal (not of literals inside it).
func returnsOf(fl *ast.FuncLit) []*ast.ReturnStmt {
	var out []*ast.ReturnStmt
	ast.Inspect(fl.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			out = append(out, x)
		}
		return true
	})
	return out
}

// iifeOf: func() *X { ...; return &X{f} }() with every return an adapter literal of X.
func iifeOf(e ast.Expr) (*types.Named, *ast.FuncLit) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return nil, nil
	}
	lit, ok := rw.Unparen(c.Fun).(*ast.FuncLit)
	if !ok || lit.Type.Params.NumFields() != 0 || lit.Type.Results == nil || len(lit.Type.Results.List) != 1 || len(lit.Type.Results.List[0].Names) != 0 {
		return nil, nil
	}
	var x *types.Named
	rets := returnsOf(lit)
	if len(rets) == 0 {
		return nil, nil
	}
	for _, r := range rets {
		if len(r.Results) != 1 {
			return nil, nil
		}
		rx, _ := adapterOf(r.Results[0])
		if rx == nil || (x != nil && rx != x) {
			return nil, nil
		}
		x = rx
	}
	if !types.Identical(rw.Info.TypeOf(lit.Type.Results.List[0].Type), types.NewPointer(x)) {
		return nil, nil
	}
	return x, lit
}

// namedObj: the class C of an argument that is a never-null object of a named class: a
// constructor call new_C(..) (j2go's constructors return the object they make), or a call of a
// package function whose body is only `return E` with E such a call.
func namedObj(e ast.Expr) *types.Named {
	return namedObjN(e, 3)
}

func namedObjN(e ast.Expr, depth int) *types.Named {
	if depth == 0 {
		return nil
	}
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil
	}
	id, ok := rw.Unparen(c.Fun).(*ast.Ident)
	if !ok {
		return nil
	}
	fo, ok := rw.Info.Uses[id].(*types.Func)
	if !ok || fo.Pkg() != rw.Pkg {
		return nil
	}
	pt, ok := types.Unalias(rw.Info.TypeOf(e)).(*types.Pointer)
	if !ok {
		return nil
	}
	nt, ok := types.Unalias(pt.Elem()).(*types.Named)
	if !ok || nt.Obj().Pkg() != rw.Pkg || nt.TypeParams().Len() > 0 {
		return nil
	}
	if fo.Name() == "new_"+nt.Obj().Name() || strings.HasPrefix(fo.Name(), "new_"+nt.Obj().Name()+"_") {
		return nt
	}
	fd := declOf[fo]
	if fd == nil || fd.Body == nil || len(fd.Body.List) != 1 {
		return nil
	}
	r, ok := fd.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(r.Results) != 1 {
		return nil
	}
	if namedObjN(r.Results[0], depth-1) != nt {
		return nil
	}
	return nt
}

// typedMethodOf: C has mT_ of the group's function type, and C's m is its bridge: m's body is
// the adapter's m with this.f in place of this.mT_.
func typedMethodOf(g *group, c *types.Named) string {
	o, _, _ := types.LookupFieldOrMethod(types.NewPointer(c), true, rw.Pkg, g.method+"T_")
	tm, ok := o.(*types.Func)
	if !ok {
		return "given a named object without the typed method " + g.method + "T_"
	}
	sig := tm.Type().(*types.Signature)
	if !types.Identical(types.NewSignatureType(nil, nil, nil, sig.Params(), sig.Results(), sig.Variadic()), g.ftype) {
		return "given a named object whose typed method has another signature"
	}
	bd := methodDecl(c, g.method)
	ad := methodDecl(g.adapter, g.method)
	if bd == nil || ad == nil || shape(ad, "f") == "" || shape(ad, "f") != shape(bd, g.method+"T_") {
		return "given a named object whose method is not the bridge of its typed method"
	}
	return ""
}

// shape: the single statement of a method, with this.<fn> and the parameters renamed.
func shape(fd *ast.FuncDecl, fn string) string {
	recv := rw.Info.Defs[fd.Recv.List[0].Names[0]]
	subst := map[ast.Node]string{}
	i := 0
	params := map[types.Object]string{}
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			params[rw.Info.Defs[n]] = fmt.Sprintf("P%d", i)
			i++
		}
	}
	ok := true
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.SelectorExpr:
			if id, isId := x.X.(*ast.Ident); isId && rw.Info.Uses[id] == recv {
				if x.Sel.Name != fn {
					ok = false
				}
				subst[x] = "F"
				return false
			}
		case *ast.Ident:
			if p, isP := params[rw.Info.Uses[x]]; isP {
				subst[x] = p
			}
		}
		return true
	})
	if !ok {
		return ""
	}
	return renderWith(fd.Body.List[0], subst)
}

func hasVar(g *group, v *types.Var) bool {
	for _, w := range g.vars {
		if w == v {
			return true
		}
	}
	return false
}

// pkgVar: the package-level variable of a functional interface type an argument names.
func pkgVar(e ast.Expr) *types.Var {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	v, ok := rw.Info.Uses[id].(*types.Var)
	if !ok || v.Parent() != rw.Pkg.Scope() || fnIface(v.Type()) == nil {
		return nil
	}
	return v
}

// rtTyped: the runtime value with a typed variant an argument is ("Predicates_alwaysTrue").
func rtTyped(e ast.Expr) string {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return ""
	}
	if n, _ := rw.CallName(c); n == "Predicates_alwaysTrue" {
		return n
	}
	return ""
}

// rtTypedFor: the typed variant fits the group's function type: func(T) bool.
func rtTypedFor(g *group, a ast.Expr) string {
	if rtTypedFor_(g, a) == "" {
		return "given a runtime value whose typed variant does not fit the function type"
	}
	return ""
}

func rtTypedFor_(g *group, a ast.Expr) string {
	sig := g.ftype
	if rtTyped(a) == "Predicates_alwaysTrue" && sig.Params().Len() == 1 && sig.Results().Len() == 1 && types.Identical(sig.Results().At(0).Type(), types.Typ[types.Bool]) && !sig.Variadic() {
		return "Predicates_alwaysTrueT_[" + types.TypeString(sig.Params().At(0).Type(), qual) + "]"
	}
	return ""
}

// unboxedArg: the e of an argument B(e) that the unboxing K undoes, of K's result type.
func unboxedArg(k *ast.CallExpr, a ast.Expr) ast.Expr {
	b, ok := rw.Unparen(a).(*ast.CallExpr)
	if !ok {
		return nil
	}
	var e ast.Expr
	if len(b.Args) == 1 {
		e = b.Args[0]
	} else if s, ok := b.Fun.(*ast.SelectorExpr); ok && len(b.Args) == 0 {
		e = s.X
	} else {
		return nil
	}
	sig, ok := rw.Info.TypeOf(k.Fun).(*types.Signature)
	if !ok || sig.Results().Len() != 1 {
		return nil
	}
	t := sig.Results().At(0).Type()
	if !types.Identical(rw.Info.TypeOf(e), t) || !unboxes(k, b, t) {
		return nil
	}
	return e
}

// adapterFor: for a group given only runtime values with typed variants, the adapter type of the
// parameter's type: the parameter is I_of[T] (j2go's alias of the erased interface I with its
// type argument), and the adapter the one type I_tfn_.. of the package that implements I and
// holds a func(T) bool (the predicates: the one function type a typed variant has).
func adapterFor(t types.Type) *types.Named {
	al, ok := t.(*types.Alias)
	if !ok || al.TypeArgs().Len() != 1 {
		return nil
	}
	it, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return nil
	}
	want := types.NewSignatureType(nil, nil, nil, types.NewTuple(types.NewParam(0, nil, "", al.TypeArgs().At(0))), types.NewTuple(types.NewParam(0, nil, "", types.Typ[types.Bool])), false)
	var found *types.Named
	for _, n := range rw.Pkg.Scope().Names() {
		if !strings.HasPrefix(n, it.Obj().Name()+"_tfn") {
			continue
		}
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok {
			continue
		}
		nt, ok := tn.Type().(*types.Named)
		if !ok || nt.TypeParams().Len() > 0 {
			continue
		}
		st, ok := nt.Underlying().(*types.Struct)
		if !ok || st.NumFields() != 1 || !types.Identical(st.Field(0).Type(), want) {
			continue
		}
		if !types.Implements(types.NewPointer(nt), it.Underlying().(*types.Interface)) {
			continue
		}
		if found != nil {
			return nil
		}
		found = nt
	}
	return found
}
