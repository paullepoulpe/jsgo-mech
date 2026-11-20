// Command nilmaps is a post-flip step after native: a Go map that may be nil throws
// NullPointerException where it is read, as Java's null Map or Set did.
//
// tools/postflip/native turns typed maps and sets into Go maps. Java's null is then a nil
// map: storing into one panics (Go's "assignment to entry in nil map", which the runtime
// reports as a NullPointerException), but reading one does not: m[k] is the zero value,
// len(m) is 0, a range walks nothing, delete and clear do nothing. Java throws
// NullPointerException at each of them. The runtime's map helpers (rt_native.go: has_,
// mapStr_, setAdd_...) check for nil themselves; this tool handles the operations written
// inline.
//
// It proves a map variable never nil when everything that enters it is never nil:
//
//   - a map literal, make, the copies mapFrom_ and setFrom_, mapImm_ / setImm_ of an immutable
//     collection that was just made (ImmutableMap.of, copyOf, a builder's build), and mapNN_;
//   - another variable or a result of a function of the package that is never nil;
//   - a parameter gets the arguments of every call of the function (of every method of that
//     name, for a method: calls through interfaces reach any of them); a function used as a
//     value, a variadic or a runtime caller makes its parameters possibly nil;
//   - a field must also be assigned, as a statement of its own, by the field initializers of
//     its class (finit_T) or by each of its constructors (init_T..., or a constructor that
//     starts by calling another one): before that Java's field is null too. A local variable
//     needs no such check, Java's definite assignment guarantees a store before each read.
//
// Anything else (nil, a value from the runtime or out of another collection, a type
// assertion, an address taken) may be nil. An inline read through an expression that may be
// nil is wrapped: mapNN_(m)[k], len(mapNN_(m)), range mapNN_(m), delete(mapNN_(m), k),
// clear(mapNN_(m)); mapNN_ throws NullPointerException for nil and returns m. Java evaluates
// the arguments of m.get(k) before it finds m null, mapNN_(m)[k] finds it first: a read whose
// key expression calls something goes through mapAt_(m, k) (mapAt2_ for v, ok := m[k],
// mapDelete_ for delete), which checks after both are evaluated.
//
// Not the Go maps of manual steps whose nil is their empty state by design (goNative below):
// they have no Java map behind them.
//
// usage: nilmaps [-n] [-v] <dir of package closure>
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

type slot struct {
	name    string
	nilWhy  string     // why it may be nil ("" while it is thought never nil)
	sources []ast.Expr // what is stored into it
	field   bool
	owner   string // the struct of a field
	native  bool   // a manual step's Go map, nil when empty by design
}

var (
	slots    = map[any]*slot{} // *types.Var, *types.Func (its result), *ast.FuncLit (its result)
	nonNilRT = map[string]bool{"mapFrom_": true, "setFrom_": true, "mapNN_": true}
	immRT    = map[string]bool{"mapImm_": true, "setImm_": true}
)

// goNative are fields a manual step made Go maps that are nil until first used (commits
// 6073a778, 010746fe, 35e845b7): Java has no map there, and the code tests for nil itself.
var goNative = map[string]bool{
	"DataFlowAnalysis_UniqueQueue.seenSet":                                           true,
	"MakeDeclaredNamesUnique_ContextualRenameInverter_RenamingInfo.shadowDeclared":   true,
	"MakeDeclaredNamesUnique_ContextualRenameInverter_RenamingInfo.shadowUndeclared": true,
}

func isMap(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := t.Underlying().(*types.Map)
	return ok
}

func generated(p token.Pos) bool {
	b := filepath.Base(rw.Fset.Position(p).Filename)
	return strings.HasPrefix(b, "g_") && b != "g_zstubs.go"
}

func slotOf(k any, name string) *slot {
	s := slots[k]
	if s == nil {
		s = &slot{name: name}
		slots[k] = s
	}
	return s
}

func varSlot(v *types.Var) *slot {
	s := slotOf(v, v.Name())
	if !generated(v.Pos()) && s.nilWhy == "" {
		s.nilWhy = "a variable of the runtime"
	}
	return s
}

func mark(s *slot, why string) {
	if s.nilWhy == "" {
		s.nilWhy = why
	}
}

func varOf(e ast.Expr) *types.Var {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		if v, ok := rw.Info.Uses[x].(*types.Var); ok {
			return v
		}
		v, _ := rw.Info.Defs[x].(*types.Var)
		return v
	case *ast.SelectorExpr:
		if s, ok := rw.Info.Selections[x]; ok && s.Kind() == types.FieldVal {
			v, _ := s.Obj().(*types.Var)
			return v
		}
		v, _ := rw.Info.Uses[x.Sel].(*types.Var)
		return v
	}
	return nil
}

func calleeFunc(c *ast.CallExpr) *types.Func {
	fun := rw.Unparen(c.Fun)
	if ix, ok := fun.(*ast.IndexExpr); ok {
		fun = ix.X
	}
	if ix, ok := fun.(*ast.IndexListExpr); ok {
		fun = ix.X
	}
	switch f := fun.(type) {
	case *ast.Ident:
		o, _ := rw.Info.Uses[f].(*types.Func)
		return o
	case *ast.SelectorExpr:
		if s, ok := rw.Info.Selections[f]; ok {
			o, _ := s.Obj().(*types.Func)
			return o
		}
		o, _ := rw.Info.Uses[f.Sel].(*types.Func)
		return o
	}
	return nil
}

// methodParams are the parameter variables of the generated methods, by method name.
var methodParams = map[string][]*types.Signature{}

// why says why expression e may be nil, or "" if it is never nil (given the slots' state).
func why(e ast.Expr) string {
	e = rw.Unparen(e)
	switch x := e.(type) {
	case *ast.CompositeLit:
		return ""
	case *ast.Ident:
		if x.Name == "nil" {
			return "nil is stored"
		}
	case *ast.CallExpr:
		if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "make" {
			return ""
		}
		if fl, ok := rw.Unparen(x.Fun).(*ast.FuncLit); ok {
			if s := slots[fl]; s != nil {
				return s.nilWhy
			}
			return "the result of a function literal"
		}
		// A conversion to a map type keeps the value.
		if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() && len(x.Args) == 1 {
			return why(x.Args[0])
		}
		f := calleeFunc(x)
		if f == nil {
			return "the result of a function value"
		}
		f = f.Origin()
		if nonNilRT[f.Name()] {
			return ""
		}
		if immRT[f.Name()] {
			a := rw.Unparen(x.Args[len(x.Args)-1])
			for {
				c, ok := a.(*ast.CallExpr)
				if !ok {
					break
				}
				n, _ := rw.CallName(c)
				if i := strings.Index(n, "["); i >= 0 {
					n = n[:i]
				}
				if strings.HasPrefix(n, "Immutable") && (strings.Contains(n, "_of") || strings.Contains(n, "_copyOf")) {
					return ""
				}
				if s, ok := c.Fun.(*ast.SelectorExpr); ok && len(c.Args) == 0 && (s.Sel.Name == "build" || s.Sel.Name == "buildOrThrow") {
					return ""
				}
				if (n == "Collection" || n == "Map" || n == "Set" || n == "obj_" || n == "any" || strings.HasPrefix(n, "icast_")) && len(c.Args) == 1 {
					a = rw.Unparen(c.Args[0])
					continue
				}
				break
			}
			return "an immutable collection that may be null (" + f.Name() + ")"
		}
		if !generated(f.Pos()) {
			return "the result of " + f.Name() + " (runtime)"
		}
		if s := slots[f]; s != nil {
			return s.nilWhy
		}
		return "the result of " + f.Name()
	}
	if v := varOf(e); v != nil {
		return varSlot(v).nilWhy
	}
	return fmt.Sprintf("the value of a %T", e)[len("the value of a *ast."):]
}

func main() {
	report, verbose := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])

	// ---- slots and what enters them
	funcOfDecl := map[*ast.FuncDecl]*types.Func{}
	usedAsValue := map[*types.Func]bool{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				o, _ := rw.Info.Defs[x.Name].(*types.Func)
				if o == nil {
					return true
				}
				funcOfDecl[x] = o
				sig := o.Type().(*types.Signature)
				if sig.Recv() != nil && generated(x.Pos()) {
					methodParams[o.Name()] = append(methodParams[o.Name()], sig)
				}
				if sig.Results().Len() >= 1 && isMap(sig.Results().At(0).Type()) && generated(x.Pos()) {
					s := slotOf(o, o.Name()+"()")
					if sig.Results().Len() > 1 || x.Body == nil {
						mark(s, "a function of several results")
					}
				}
			case *ast.FuncLit:
				if t, ok := rw.Info.Types[x].Type.(*types.Signature); ok && t.Results().Len() == 1 && isMap(t.Results().At(0).Type()) {
					slotOf(x, "func literal")
				}
			case *ast.Ident:
				// A function named other than as the callee of a call.
				if o, ok := rw.Info.Uses[x].(*types.Func); ok {
					var e ast.Node = x
					if s, ok := rw.Parent[x].(*ast.SelectorExpr); ok && s.Sel == x {
						e = s
					}
					for {
						switch p := rw.Parent[e].(type) {
						case *ast.ParenExpr, *ast.IndexExpr, *ast.IndexListExpr:
							e = p
							continue
						}
						break
					}
					if c, ok := rw.Parent[e].(*ast.CallExpr); !ok || c.Fun != e {
						usedAsValue[o.Origin()] = true
					}
				}
			}
			return true
		})
	}
	store := func(dst ast.Expr, src ast.Expr) {
		if !isMap(rw.Info.TypeOf(dst)) {
			return
		}
		v := varOf(dst)
		if v == nil {
			return // an element of a collection: reading it back may be nil anyway
		}
		s := varSlot(v)
		if src == nil {
			mark(s, "assigned from a call of several results")
			return
		}
		s.sources = append(s.sources, src)
	}
	enclosing := func(n ast.Node) any {
		for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
			switch f := p.(type) {
			case *ast.FuncLit:
				return f
			case *ast.FuncDecl:
				return funcOfDecl[f]
			}
		}
		return nil
	}
	for _, f := range rw.Files {
		gen := generated(f.Pos())
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				if len(x.Lhs) != len(x.Rhs) {
					for _, l := range x.Lhs {
						store(l, nil)
					}
					return true
				}
				for i, l := range x.Lhs {
					store(l, x.Rhs[i])
				}
			case *ast.ValueSpec:
				for i, id := range x.Names {
					if len(x.Values) == len(x.Names) {
						store(id, x.Values[i])
					} else if len(x.Values) != 0 {
						store(id, nil)
					} else if v, ok := rw.Info.Defs[id].(*types.Var); ok && isMap(v.Type()) && v.Parent() == rw.Pkg.Scope() {
						mark(varSlot(v), "a package variable without a value")
					}
				}
			case *ast.RangeStmt:
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if e != nil && isMap(rw.Info.TypeOf(e)) {
						if v := varOf(e); v != nil {
							mark(varSlot(v), "an element of a collection")
						}
					}
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND && isMap(rw.Info.TypeOf(x.X)) {
					if v := varOf(x.X); v != nil {
						mark(varSlot(v), "its address is taken")
					}
				}
			case *ast.KeyValueExpr:
				if id, ok := x.Key.(*ast.Ident); ok {
					if fv, ok := rw.Info.Uses[id].(*types.Var); ok && fv.IsField() && isMap(fv.Type()) {
						varSlot(fv).sources = append(varSlot(fv).sources, x.Value)
					}
				}
			case *ast.ReturnStmt:
				if k := enclosing(x); k != nil {
					if s := slots[k]; s != nil {
						if len(x.Results) != 1 {
							mark(s, "a return without a value")
						} else {
							s.sources = append(s.sources, x.Results[0])
						}
					}
				}
			case *ast.CallExpr:
				if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
					return true
				}
				sig, _ := rw.Info.TypeOf(x.Fun).(*types.Signature)
				if sig == nil {
					return true
				}
				anyMap := false
				for i := 0; i < sig.Params().Len(); i++ {
					anyMap = anyMap || isMap(sig.Params().At(i).Type())
				}
				if !anyMap {
					return true
				}
				// The parameter variables this call may reach.
				var targets []*types.Signature
				if fl, ok := rw.Unparen(x.Fun).(*ast.FuncLit); ok {
					targets = []*types.Signature{rw.Info.Types[fl].Type.(*types.Signature)}
				} else if f := calleeFunc(x); f != nil {
					f = f.Origin()
					fs := f.Type().(*types.Signature)
					if fs.Recv() != nil {
						targets = methodParams[f.Name()]
					} else {
						targets = []*types.Signature{fs}
					}
				} else {
					return true // a function value: its parameters are possibly nil already
				}
				for _, t := range targets {
					for i := 0; i < t.Params().Len() && i < len(x.Args); i++ {
						p := t.Params().At(i)
						if !isMap(p.Type()) {
							continue
						}
						s := varSlot(p)
						if t.Variadic() && i >= t.Params().Len()-1 {
							mark(s, "a variadic parameter")
						} else if !gen {
							mark(s, "a parameter the runtime passes")
						} else {
							s.sources = append(s.sources, x.Args[i])
						}
					}
				}
			}
			return true
		})
	}
	// Parameters of functions used as values and of function literals not called in place.
	markParams := func(sig *types.Signature, why string) {
		for i := 0; i < sig.Params().Len(); i++ {
			if p := sig.Params().At(i); isMap(p.Type()) {
				mark(varSlot(p), why)
			}
		}
	}
	for f := range usedAsValue {
		markParams(f.Type().(*types.Signature), "a parameter of a function used as a value")
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if fl, ok := n.(*ast.FuncLit); ok {
				if c, ok := rw.Parent[fl].(*ast.CallExpr); !ok || c.Fun != fl {
					markParams(rw.Info.Types[fl].Type.(*types.Signature), "a parameter of a function used as a value")
				}
			}
			return true
		})
	}

	// ---- fields: assigned by the initializers or by every constructor
	type ctor struct {
		assigns  map[*types.Var]bool
		delegate string
	}
	ctors := map[string]map[string]*ctor{} // struct -> init method -> what it assigns
	finit := map[string]map[*types.Var]bool{}
	news := map[string][]map[*types.Var]bool{} // struct -> what each new_T function assigns
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if ok && fd.Recv == nil && fd.Body != nil && strings.HasPrefix(fd.Name.Name, "new_") && len(fd.Body.List) > 0 {
				// new_T(...): this := &T{}; this.f = a captured variable...; this.init_T()
				as, ok := fd.Body.List[0].(*ast.AssignStmt)
				if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 || rw.Text(as.Lhs[0]) != "this" {
					continue
				}
				pt, ok := rw.Info.TypeOf(as.Rhs[0]).(*types.Pointer)
				if !ok {
					continue
				}
				nt, ok := types.Unalias(pt.Elem()).(*types.Named)
				if !ok {
					continue
				}
				T := nt.Obj().Name()
				set := map[*types.Var]bool{}
				for _, st := range fd.Body.List[1:] {
					if x, ok := st.(*ast.AssignStmt); ok {
						for _, l := range x.Lhs {
							if s, ok := l.(*ast.SelectorExpr); ok && rw.Text(s.X) == "this" {
								if v := varOf(s); v != nil {
									set[v] = true
								}
							}
						}
					}
				}
				news[T] = append(news[T], set)
				continue
			}
			if !ok || fd.Recv == nil || fd.Body == nil || len(fd.Recv.List) != 1 || len(fd.Recv.List[0].Names) != 1 {
				continue
			}
			rt := rw.Info.TypeOf(fd.Recv.List[0].Type)
			if p, ok := rt.(*types.Pointer); ok {
				rt = p.Elem()
			}
			nt, ok := types.Unalias(rt).(*types.Named)
			if !ok {
				continue
			}
			T := nt.Obj().Name()
			recv := fd.Recv.List[0].Names[0].Name
			isFinit := fd.Name.Name == "finit_"+T
			isInit := fd.Name.Name == "init_"+T || strings.HasPrefix(fd.Name.Name, "init_"+T+"_")
			if !isFinit && !isInit {
				continue
			}
			c := &ctor{assigns: map[*types.Var]bool{}}
			for i, st := range fd.Body.List {
				switch x := st.(type) {
				case *ast.AssignStmt:
					for _, l := range x.Lhs {
						if s, ok := l.(*ast.SelectorExpr); ok {
							if id, ok := s.X.(*ast.Ident); ok && id.Name == recv {
								if v := varOf(s); v != nil {
									c.assigns[v] = true
								}
							}
						}
					}
				case *ast.ExprStmt:
					if call, ok := x.X.(*ast.CallExpr); ok && i == 0 {
						if s, ok := call.Fun.(*ast.SelectorExpr); ok {
							if id, ok := s.X.(*ast.Ident); ok && id.Name == recv && (s.Sel.Name == "init_"+T || strings.HasPrefix(s.Sel.Name, "init_"+T+"_")) {
								c.delegate = s.Sel.Name
							}
						}
					}
				}
			}
			if isFinit {
				finit[T] = c.assigns
			} else {
				if ctors[T] == nil {
					ctors[T] = map[string]*ctor{}
				}
				ctors[T][fd.Name.Name] = c
			}
		}
	}
	var ctorAssigns func(T, m string, v *types.Var, depth int) bool
	ctorAssigns = func(T, m string, v *types.Var, depth int) bool {
		c := ctors[T][m]
		if c == nil || depth > 8 {
			return false
		}
		return c.assigns[v] || (c.delegate != "" && ctorAssigns(T, c.delegate, v, depth+1))
	}
	for _, name := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			fv := st.Field(i)
			if !isMap(fv.Type()) || !generated(fv.Pos()) {
				continue
			}
			s := varSlot(fv)
			s.field, s.owner = true, name
			ok := finit[name][fv]
			if !ok && len(ctors[name]) > 0 {
				ok = true
				for m := range ctors[name] {
					ok = ok && ctorAssigns(name, m, fv, 0)
				}
			}
			if !ok && len(news[name]) > 0 {
				ok = true
				for _, set := range news[name] {
					ok = ok && set[fv]
				}
			}
			if goNative[name+"."+fv.Name()] {
				s.native = true
			}
			if !ok {
				mark(s, "a field its class's initializers and constructors do not all assign")
			}
		}
	}

	// ---- fixpoint
	for changed := true; changed; {
		changed = false
		for _, s := range slots {
			if s.nilWhy != "" {
				continue
			}
			for _, e := range s.sources {
				if w := why(e); w != "" {
					if !strings.HasPrefix(w, "from ") {
						w = "from " + rw.Text(e)[:min(30, len(rw.Text(e)))] + ": " + w
					}
					s.nilWhy = w
					changed = true
					break
				}
			}
		}
	}

	// ---- reads
	left := map[string]int{}
	proved, wrapped, impure := 0, 0, 0
	reasons := map[string]int{}
	wrap := func(m ast.Expr, pureArgs bool, what string, whole ast.Expr) {
		if n, _ := rw.CallName(m); n == "mapNN_" {
			return
		}
		if v := varOf(m); v != nil && slots[v] != nil && slots[v].native {
			left["a manual step's Go map that is nil when empty by design"]++
			return
		}
		w := why(m)
		if w == "" {
			proved++
			return
		}
		wrapped++
		if i := strings.Index(w, ": "); i >= 0 && strings.HasPrefix(w, "from ") {
			w = w[i+2:]
		}
		reasons[w]++
		if verbose {
			p := rw.Fset.Position(m.Pos())
			fmt.Fprintf(os.Stderr, "%s:%d %s %s: %s\n", filepath.Base(p.Filename), p.Line, what, rw.Text(m), why(m))
		}
		if pureArgs {
			rw.Rewrite[m] = func() string { return "mapNN_(" + rw.RenderChildren(m) + ")" }
			return
		}
		// The key calls something: check the map after the key is evaluated, as Java does.
		impure++
		switch x := whole.(type) {
		case *ast.IndexExpr:
			fn := "mapAt_"
			if as, ok := rw.Parent[x].(*ast.AssignStmt); ok && len(as.Lhs) == 2 && len(as.Rhs) == 1 {
				fn = "mapAt2_"
			}
			rw.Rewrite[x] = func() string { return fn + "(" + rw.Render(x.X) + ", " + rw.Render(x.Index) + ")" }
		case *ast.CallExpr: // delete(m, k)
			rw.Rewrite[x] = func() string { return "mapDelete_(" + rw.Render(x.Args[0]) + ", " + rw.Render(x.Args[1]) + ")" }
		}
	}
	noCalls := func(e ast.Expr) bool {
		ok := true
		ast.Inspect(e, func(n ast.Node) bool {
			if c, isCall := n.(*ast.CallExpr); isCall {
				if tv, has := rw.Info.Types[c.Fun]; !has || !tv.IsType() {
					ok = false
				}
			}
			return ok
		})
		return ok
	}
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.IndexExpr:
				if !isMap(rw.Info.TypeOf(x.X)) {
					return true
				}
				// A store (m[k] = v, m[k] += v, m[k]++) panics on a nil map by itself.
				switch p := rw.Parent[x].(type) {
				case *ast.AssignStmt:
					for _, l := range p.Lhs {
						if l == x {
							return true
						}
					}
				case *ast.IncDecStmt:
					return true
				}
				wrap(x.X, noCalls(x.Index), "get", x)
			case *ast.RangeStmt:
				if isMap(rw.Info.TypeOf(x.X)) {
					wrap(x.X, true, "range", nil)
				}
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok && len(x.Args) >= 1 && isMap(rw.Info.TypeOf(x.Args[0])) {
					if _, builtin := rw.Info.Uses[id].(*types.Builtin); builtin {
						switch id.Name {
						case "len", "clear":
							wrap(x.Args[0], true, id.Name, nil)
						case "delete":
							wrap(x.Args[0], noCalls(x.Args[1]), "delete", x)
						}
					}
				}
			}
			return true
		})
	}
	total, never := 0, 0
	for k, s := range slots {
		if v, ok := k.(*types.Var); ok && generated(v.Pos()) {
			total++
			if s.nilWhy == "" {
				never++
			}
		}
	}
	keys := []string{}
	for k := range reasons {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		if reasons[keys[i]] != reasons[keys[j]] {
			return reasons[keys[i]] > reasons[keys[j]]
		}
		return keys[i] < keys[j]
	})
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  checked: %4d may be nil: %s\n", reasons[k], k)
	}
	keys = keys[:0]
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
	}
	msg := fmt.Sprintf("nilmaps: %d map variables, %d never nil; inline reads: %d of a map never nil, %d get a nil check (%d of them after the key is evaluated: mapAt_)", total, never, proved, wrapped, impure)
	if report {
		fmt.Fprintln(os.Stderr, msg)
		return
	}
	fmt.Fprintf(os.Stderr, "%s; files changed: %d\n", msg, rw.SaveChecked())
}
