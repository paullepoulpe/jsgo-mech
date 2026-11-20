// Command tfnargs is a post-flip step for the typed translator's text: a function parameter
// that is always given the method of a typed lambda adapter takes the typed function itself.
//
//	func NodeUtil_getEnclosingNode(n Node, pred func(any) bool) Node { ... pred(curNode.box_()) ... }
//	NodeUtil_getEnclosingNode(n, (&Predicate_base_tfn_Node{func(p0 Node) bool { return p0.isClass() }}).apply)
//	  ->
//	func NodeUtil_getEnclosingNode(n Node, pred func(Node) bool) Node { ... pred(curNode) ... }
//	NodeUtil_getEnclosingNode(n, func(p0 Node) bool { return p0.isClass() })
//
// The typed translator writes a Java lambda with typed parameters as an adapter object
// &X_tfn_T{f} whose method unboxes the arguments, calls f and boxes the result. The step
// funcparams made the parameters that are only called Go func values of the erased signature
// and passes the adapter's method value: per call an adapter, a method value and (the
// function escapes with them) the closure, and a box and a type assertion per call of the
// function. Here the parameter gets the type of the adapter's function.
//
// A parameter p of a generated function F that is not generic is converted when
//   - F is only ever called, and for a method no interface of the package has a method of its
//     name (an interface call cannot reach it);
//   - every call of F passes (&X_tfn...{f}).m, with one adapter type X and its method m for
//     all calls, or, inside F, p itself (a recursive call);
//   - m is `return this.f(c0(p0), ...)` or `return box(this.f(...))`, where each c is absent
//     (the parameter is passed as it is), X_of_ (a handle) or str_ (a String), and box is
//     absent or strobj_;
//   - every use of p in F is such a recursive argument or a call p(a0, ...) whose arguments at
//     the converted positions are the matching boxings, e.box_() and strobj_(e), with e of
//     the function's parameter type, and whose result, when the adapter boxes it, is the
//     operand of str_(...) or objstr_(...).
//
// The call p(e.box_()) through the adapter was f(X_of_(e.box_())), which is f(e) for every e
// (0 boxes to nil and comes back as 0); f(str_(strobj_(e))) is f(e) for every string, the
// null string included. str_(strobj_(r)) is r, and objstr_(strobj_(r)) is sstr_(r): "null"
// for the null string, r otherwise. The adapter was never null, f is evaluated where the
// adapter was made, and nothing else could see the adapter, so Java's behaviour is the same.
//
// A call that passes any other function of the erased type (a method value of a predicate
// object, a variable) to such a parameter wraps it where the runtime has the wrapper, for
// the adapters of one handle: unboxedPred_[T](f) and unboxedCons_[T](f) (rt_tfn.go) are func(x T) { f(x.box_()) }, the call the function body made; f is
// evaluated where it was, and a null f fails when it is called, as before.
//
// Everything else is left and counted by reason. A rerun changes nothing.
//
// usage: tfnargs [-n] <dir of package closure>
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

// adapter describes a method of a typed adapter type: the function field's type as text and
// as a type, the conversion of each argument ("" none, "of" a handle's X_of_, "str" str_) and
// of the result ("" none, "strobj").
type adapter struct {
	fieldText string
	sig       *types.Signature
	args      []string
	res       string
}

var (
	declOf   = map[*types.Func]*ast.FuncDecl{}
	adapters = map[*types.Func]*adapter{}
)

// adapterOf analyses method m of an adapter type, or returns nil.
func adapterOf(m *types.Func) *adapter {
	if a, ok := adapters[m]; ok {
		return a
	}
	adapters[m] = nil
	fd := declOf[m]
	if fd == nil || fd.Recv == nil || fd.Body == nil || len(fd.Body.List) != 1 {
		return nil
	}
	recv := m.Type().(*types.Signature).Recv().Type()
	ptr, ok := recv.(*types.Pointer)
	if !ok {
		return nil
	}
	nt, ok := ptr.Elem().(*types.Named)
	if !ok || !strings.Contains(nt.Obj().Name(), "_tfn") {
		return nil
	}
	st, ok := nt.Underlying().(*types.Struct)
	if !ok || st.NumFields() != 1 {
		return nil
	}
	fsig, ok := st.Field(0).Type().(*types.Signature)
	if !ok {
		return nil
	}
	var fieldText string
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, sp := range gd.Specs {
				ts := sp.(*ast.TypeSpec)
				if rw.Info.Defs[ts.Name] == nt.Obj() {
					fieldText = rw.Text(ts.Type.(*ast.StructType).Fields.List[0].Type)
				}
			}
		}
	}
	if fieldText == "" {
		return nil
	}
	a := &adapter{fieldText: fieldText, sig: fsig}
	var e ast.Expr
	switch s := fd.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(s.Results) != 1 {
			return nil
		}
		e = s.Results[0]
	case *ast.ExprStmt:
		e = s.X
	default:
		return nil
	}
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return nil
	}
	if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "strobj_" && len(call.Args) == 1 {
		a.res = "strobj"
		if call, ok = call.Args[0].(*ast.CallExpr); !ok {
			return nil
		}
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || rw.Text(sel) != "this.f" {
		return nil
	}
	msig := m.Type().(*types.Signature)
	if len(call.Args) != msig.Params().Len() || len(call.Args) != fsig.Params().Len() || call.Ellipsis.IsValid() || fsig.Variadic() {
		return nil
	}
	params := map[string]int{}
	i := 0
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			params[n.Name] = i
			i++
		}
	}
	for j, arg := range call.Args {
		conv := ""
		if c, ok := arg.(*ast.CallExpr); ok && len(c.Args) == 1 {
			id, ok := c.Fun.(*ast.Ident)
			if !ok {
				return nil
			}
			switch {
			case id.Name == "str_":
				conv = "str"
			case strings.HasSuffix(id.Name, "_of_") && id.Name == typeString(fsig.Params().At(j).Type())+"_of_":
				conv = "of"
			default:
				return nil
			}
			arg = c.Args[0]
		}
		id, ok := arg.(*ast.Ident)
		if !ok || params[id.Name] != j {
			return nil
		}
		if _, has := params[id.Name]; !has {
			return nil
		}
		if conv == "" && !types.Identical(fsig.Params().At(j).Type(), msig.Params().At(j).Type()) {
			return nil
		}
		a.args = append(a.args, conv)
	}
	if a.res == "" {
		if fsig.Results().Len() != msig.Results().Len() || (fsig.Results().Len() == 1 && !types.Identical(fsig.Results().At(0).Type(), msig.Results().At(0).Type())) {
			return nil
		}
	}
	adapters[m] = a
	return a
}

// wrapper is the runtime function that makes a function of the erased type one of the
// adapter's typed type, for the adapters of one handle: unboxedPred_[T] (a predicate) and
// unboxedCons_[T] (a consumer), which call the function with x.box_() as the callee did.
func wrapper(ad *adapter) string {
	if len(ad.args) != 1 || ad.args[0] != "of" || ad.res != "" {
		return ""
	}
	t := typeString(ad.sig.Params().At(0).Type())
	switch r := ad.sig.Results(); {
	case r.Len() == 0:
		return "unboxedCons_[" + t + "]"
	case r.Len() == 1 && types.Identical(r.At(0).Type(), types.Typ[types.Bool]):
		return "unboxedPred_[" + t + "]"
	}
	return ""
}

func typeString(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == rw.Pkg {
			return ""
		}
		return p.Name()
	})
}

// methodValue is the adapter method and the function expression of (&X_tfn{f}).m, or nil.
func methodValue(e ast.Expr) (*types.Func, ast.Expr) {
	sel, ok := rw.Unparen(e).(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	u, ok := rw.Unparen(sel.X).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return nil, nil
	}
	lit, ok := u.X.(*ast.CompositeLit)
	if !ok || len(lit.Elts) != 1 {
		return nil, nil
	}
	if _, keyed := lit.Elts[0].(*ast.KeyValueExpr); keyed {
		return nil, nil
	}
	m, ok := rw.Info.Uses[sel.Sel].(*types.Func)
	if !ok {
		return nil, nil
	}
	return m, lit.Elts[0]
}

func main() {
	report, verbose := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	done := map[string]int{}
	left := map[string]int{}
	ifaceMethods := map[string]bool{}
	for _, name := range rw.Pkg.Scope().Names() {
		if tn, ok := rw.Pkg.Scope().Lookup(name).(*types.TypeName); ok {
			if it, ok := tn.Type().Underlying().(*types.Interface); ok {
				for i := 0; i < it.NumMethods(); i++ {
					ifaceMethods[it.Method(i).Name()] = true
				}
			}
		}
	}
	generated := map[*ast.FuncDecl]bool{}
	for _, f := range rw.Files {
		gen := strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_")
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				if fn, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					declOf[fn] = fd
					generated[fd] = gen
				}
			}
		}
	}
	// The calls of each function, and whether it is used other than as the callee of a call.
	callsOf := map[*types.Func][]*ast.CallExpr{}
	valueUse := map[*types.Func]bool{}
	for id, o := range rw.Info.Uses {
		fn, ok := o.(*types.Func)
		if !ok || declOf[fn] == nil {
			continue
		}
		var e ast.Node = id
		if sel, ok := rw.Parent[id].(*ast.SelectorExpr); ok && sel.Sel == id {
			e = sel
		}
		for {
			p, ok := rw.Parent[e].(*ast.ParenExpr)
			if !ok {
				break
			}
			e = p
		}
		if call, ok := rw.Parent[e].(*ast.CallExpr); ok && call.Fun == e {
			callsOf[fn] = append(callsOf[fn], call)
		} else {
			valueUse[fn] = true
		}
	}
	var fns []*types.Func
	for fn := range callsOf {
		fns = append(fns, fn)
	}
	sort.Slice(fns, func(i, j int) bool { return fns[i].Pos() < fns[j].Pos() })
	for _, fn := range fns {
		fd := declOf[fn]
		sig := fn.Type().(*types.Signature)
		calls := callsOf[fn]
		sort.Slice(calls, func(i, j int) bool { return calls[i].Pos() < calls[j].Pos() })
		// the parameter identifiers, by position
		var pids []*ast.Ident
		var ptypes []ast.Expr
		for _, f := range fd.Type.Params.List {
			for _, n := range f.Names {
				pids = append(pids, n)
				ptypes = append(ptypes, f.Type)
			}
		}
		if len(pids) != sig.Params().Len() {
			continue
		}
		for i := 0; i < sig.Params().Len(); i++ {
			pv := sig.Params().At(i)
			if _, isFunc := pv.Type().(*types.Signature); !isFunc {
				continue
			}
			// Is the parameter given an adapter's method anywhere?
			var m *types.Func
			sites := 0
			for _, c := range calls {
				if i < len(c.Args) && !c.Ellipsis.IsValid() {
					if am, _ := methodValue(c.Args[i]); am != nil && adapterOf(am) != nil {
						sites++
						if m == nil {
							m = am
						}
					}
				}
			}
			if m == nil {
				for _, c := range calls {
					if i < len(c.Args) && !c.Ellipsis.IsValid() {
						if am, _ := methodValue(c.Args[i]); am != nil && strings.Contains(typeString(am.Type().(*types.Signature).Recv().Type()), "_tfn") {
							left["the adapter's method converts its arguments or result in a way the rule does not know"]++
						}
					}
				}
				continue
			}
			leave := func(why string) {
				left[why] += sites
				if verbose {
					fmt.Fprintf(os.Stderr, "    %s (%d): %s\n", fn.FullName(), sites, why)
				}
			}
			if !generated[fd] || fd.Body == nil {
				leave("the function is handwritten")
				continue
			}
			if sig.TypeParams().Len() > 0 || sig.RecvTypeParams().Len() > 0 {
				leave("the function is generic")
				continue
			}
			if valueUse[fn] {
				leave("the function is used as a value")
				continue
			}
			if sig.Recv() != nil && ifaceMethods[fn.Name()] {
				leave("an interface has a method of the function's name")
				continue
			}
			if sig.Variadic() && i == sig.Params().Len()-1 {
				leave("the parameter is variadic")
				continue
			}
			ad := adapterOf(m)
			why := ""
			wrapped := 0
			var edits []func()
			for _, c := range calls {
				if len(c.Args) != sig.Params().Len() || c.Ellipsis.IsValid() {
					why = "a call does not pass the parameters one by one"
					break
				}
				arg := c.Args[i]
				if id, ok := rw.Unparen(arg).(*ast.Ident); ok && rw.Info.Uses[id] == pv {
					continue // p itself, in F: a recursive call
				}
				am, f := methodValue(arg)
				if am == nil || adapterOf(am) == nil {
					// any other function of the erased type: wrapped, where the runtime has the wrapper
					if w := wrapper(ad); w != "" {
						wrapped++
						edits = append(edits, func() { rw.Rewrite[arg] = func() string { return w + "(" + rw.RenderChildren(arg) + ")" } })
						continue
					}
					why = "a call passes something that is not the method of an adapter literal"
					break
				}
				if am != m {
					why = "the calls pass methods of different adapters"
					break
				}
				edits = append(edits, func() { rw.Rewrite[arg] = func() string { return rw.Render(f) } })
			}
			if why != "" {
				leave(why)
				continue
			}
			// the uses of p in F
			for _, use := range rw.UsesOf[pv] {
				var e ast.Node = use
				for {
					p, ok := rw.Parent[e].(*ast.ParenExpr)
					if !ok {
						break
					}
					e = p
				}
				call, ok := rw.Parent[e].(*ast.CallExpr)
				if !ok {
					why = "the parameter is used other than called or passed on in a recursive call"
					break
				}
				if call.Fun != e {
					// an argument: of a recursive call, at its own position
					callee := call.Fun
					if s, ok := callee.(*ast.SelectorExpr); ok {
						callee = s.Sel
					}
					cid, _ := callee.(*ast.Ident)
					if cid == nil || rw.Info.Uses[cid] != fn || len(call.Args) <= i || call.Args[i] != e {
						why = "the parameter is used other than called or passed on in a recursive call"
					}
					if why != "" {
						break
					}
					continue
				}
				if len(call.Args) != len(ad.args) || call.Ellipsis.IsValid() {
					why = "a call of the parameter does not pass the arguments one by one"
					break
				}
				for j, conv := range ad.args {
					if conv == "" {
						continue
					}
					a := call.Args[j]
					var inner ast.Expr
					if bc, ok := rw.Unparen(a).(*ast.CallExpr); ok {
						if conv == "of" && len(bc.Args) == 0 {
							if s, ok := bc.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "box_" {
								inner = s.X
							}
						}
						if conv == "str" && len(bc.Args) == 1 {
							if id, ok := bc.Fun.(*ast.Ident); ok && id.Name == "strobj_" {
								inner = bc.Args[0]
							}
						}
					}
					if inner == nil || !types.Identical(rw.Info.Types[inner].Type, ad.sig.Params().At(j).Type()) {
						why = "a call of the parameter passes an argument that is not the boxing of a value of the function's type"
						break
					}
					edits = append(edits, func() { rw.Rewrite[a] = func() string { return rw.Render(inner) } })
				}
				if why != "" {
					break
				}
				if ad.res == "strobj" {
					var up ast.Node = call
					for {
						p, ok := rw.Parent[up].(*ast.ParenExpr)
						if !ok {
							break
						}
						up = p
					}
					outer, ok := rw.Parent[up].(*ast.CallExpr)
					var oid *ast.Ident
					if ok && len(outer.Args) == 1 && outer.Args[0] == up {
						oid, _ = outer.Fun.(*ast.Ident)
					}
					switch {
					case oid != nil && oid.Name == "str_":
						edits = append(edits, func() { rw.Rewrite[outer] = func() string { return rw.Render(call) } })
					case oid != nil && oid.Name == "objstr_":
						edits = append(edits, func() { rw.Rewrite[oid] = func() string { return "sstr_" } })
					default:
						why = "the boxed result of a call of the parameter is not unboxed at once"
					}
					if why != "" {
						break
					}
				}
			}
			if why != "" {
				leave(why)
				continue
			}
			// a parameter list `a, b T` shares its type
			shared := false
			for _, f := range fd.Type.Params.List {
				if f.Type == ptypes[i] && len(f.Names) > 1 {
					shared = true
				}
			}
			if shared {
				leave("the parameter shares its type with another in the declaration")
				continue
			}
			for _, e := range edits {
				e()
			}
			pt := ptypes[i]
			rw.Rewrite[pt] = func() string { return ad.fieldText }
			if wrapped > 0 {
				done["another function given to such a parameter is wrapped ("+wrapper(ad)+")"] += wrapped
			}
			done[fmt.Sprintf("a parameter given %s takes the %s", m.Type().(*types.Signature).Recv().Type().(*types.Pointer).Elem().(*types.Named).Obj().Name(), ad.fieldText)] += sites
		}
	}
	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
	}
	total := 0
	keys = keys[:0]
	for k, n := range done {
		keys = append(keys, k)
		total += n
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", done[k], k)
	}
	if report {
		fmt.Fprintf(os.Stderr, "tfnargs: %d arguments take the typed function\n", total)
		return
	}
	fmt.Fprintf(os.Stderr, "tfnargs: %d arguments take the typed function; files changed: %d\n", total, rw.SaveChecked())
}
