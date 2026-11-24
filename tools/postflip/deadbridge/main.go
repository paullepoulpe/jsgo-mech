// Command deadbridge is a post-flip step: javac's bridge of a class that no code can call any
// more is deleted (the deadtfn pattern for implsplit's bridges).
//
// implsplit splits a method of a named class implementing an erased library interface
// (Comparable.compareTo, Comparator.compare, Predicate.apply, Supplier.get, ...) into the typed
// method mT_ and the bridge m(Object) whose body is `return this.mT_(casts of its parameters)`,
// because the runtime calls m through the erased interface: compareNatural_ asserts
// interface{ compareTo(any) int32 }, a Comparator's compare, a Predicate's apply. The steps
// since then (tcmp's typed sorts, tfnparams' typed predicates) call mT_ itself. The bridge is
// dead, and is deleted, when
//   - no code names it (no call, no method value);
//   - no type of the package embeds the class (a subclass's dispatch could reach it);
//   - no value of the class, or of a type built from it (a slice, a map, a function's result),
//     ever becomes an interface value: no implicit or explicit conversion to an interface type
//     or `any` (argument, assignment, return, composite literal element), no self() / box_() of
//     it, no generic function or type given it (obj_[T], newTList_ and the rest box through the
//     element conversion), and the class has no element conversion tc_p_X that code uses.
// An escape inside an unused method of the class itself (javac's default-method forwarder
// test(p0) { return Predicate_base_test(this, p0) }) does not count: it runs only through an
// escape elsewhere, and such a method is deleted with the bridge.
// Then no interface value of the program holds the class's objects, so no dynamic call can
// reach the bridge: deleting it changes nothing. A rerun changes nothing.
//
// usage: deadbridge [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

var verbose bool

// noBox are the runtime's generic functions that never box their elements (rt_native.go,
// rt_iter.go): the typed sorts of a slice or an array and the typed cursor.
var noBox = map[string]bool{"sortArrF_": true, "sortSliceF_": true, "sortSlicePF_": true, "immCursorTOf_": true}

// bridge returns the name of the typed method a bridge's single statement calls on this.
func bridge(fd *ast.FuncDecl) string {
	if fd.Recv == nil || fd.Body == nil || len(fd.Body.List) != 1 {
		return ""
	}
	r, ok := fd.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(r.Results) != 1 {
		return ""
	}
	c, ok := rw.Unparen(r.Results[0]).(*ast.CallExpr)
	if !ok {
		return ""
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != fd.Name.Name+"T_" && !(fd.Name.Name == "compareTo" && s.Sel.Name == "compareTo_t") { // reflip51: piece cmp's bridge
		return ""
	}
	if id, ok := s.X.(*ast.Ident); !ok || id.Name != "this" {
		return ""
	}
	return s.Sel.Name
}

// mentions reports whether t is or is built from the named type n.
func mentions(t types.Type, n *types.Named, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	switch x := types.Unalias(t).(type) {
	case *types.Named:
		if x.Origin() == n.Origin() {
			return true
		}
		for i := 0; i < x.TypeArgs().Len(); i++ {
			if mentions(x.TypeArgs().At(i), n, seen) {
				return true
			}
		}
	case *types.Pointer:
		return mentions(x.Elem(), n, seen)
	case *types.Slice:
		return mentions(x.Elem(), n, seen)
	case *types.Array:
		return mentions(x.Elem(), n, seen)
	case *types.Map:
		return mentions(x.Key(), n, seen) || mentions(x.Elem(), n, seen)
	case *types.Chan:
		return mentions(x.Elem(), n, seen)
	case *types.Signature:
		for i := 0; i < x.Results().Len(); i++ {
			if mentions(x.Results().At(i).Type(), n, seen) {
				return true
			}
		}
		for i := 0; i < x.Params().Len(); i++ {
			if mentions(x.Params().At(i).Type(), n, seen) {
				return true
			}
		}
	case *types.Struct:
		for i := 0; i < x.NumFields(); i++ {
			if mentions(x.Field(i).Type(), n, seen) {
				return true
			}
		}
	}
	return false
}

func isIface(t types.Type) bool {
	_, ok := types.Unalias(t).Underlying().(*types.Interface)
	return ok
}

// escapes returns why a value of expression e (whose type mentions n) may become an interface
// value, or "".
func escapes(e ast.Expr) string {
	p := rw.Up(e)
	switch x := p.(type) {
	case *ast.CallExpr:
		if rw.Unparen(x.Fun) == e {
			return ""
		}
		ft := rw.Info.TypeOf(x.Fun)
		if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
			if isIface(tv.Type) {
				return "converted to an interface"
			}
			return ""
		}
		if id := calleeIdent(x.Fun); id != nil {
			if b, ok := rw.Info.Uses[id].(*types.Builtin); ok {
				switch b.Name() {
				case "append", "len", "cap", "copy", "delete", "clear", "min", "max":
					return ""
				}
				return "an argument of a builtin"
			}
			// the runtime's typed sorts and cursors keep their elements typed
			if noBox[id.Name] {
				return ""
			}
		}
		sig, ok := types.Unalias(ft).Underlying().(*types.Signature)
		if !ok {
			return "an argument of a builtin"
		}
		// a generic function or method of a generic type: its body may box its type parameters
		if id := calleeIdent(x.Fun); id != nil {
			if inst, ok := rw.Info.Instances[id]; ok && inst.TypeArgs.Len() > 0 {
				return "given to a generic function"
			}
			if f, ok := rw.Info.Uses[id].(*types.Func); ok {
				if r := f.Type().(*types.Signature).Recv(); r != nil {
					rt := r.Type()
					if pt, ok := rt.(*types.Pointer); ok {
						rt = pt.Elem()
					}
					if nt, ok := types.Unalias(rt).(*types.Named); ok && nt.TypeParams().Len() > 0 {
						return "given to a method of a generic type"
					}
				}
			}
		}
		for i, a := range x.Args {
			if rw.Unparen(a) != e {
				continue
			}
			var pt types.Type
			if sig.Variadic() && i >= sig.Params().Len()-1 {
				pt = sig.Params().At(sig.Params().Len() - 1).Type().(*types.Slice).Elem()
				if x.Ellipsis.IsValid() {
					pt = sig.Params().At(sig.Params().Len() - 1).Type()
				}
			} else if i < sig.Params().Len() {
				pt = sig.Params().At(i).Type()
			}
			if pt == nil || isIface(pt) {
				return "an argument of interface type"
			}
		}
	case *ast.SelectorExpr:
		if x.X == e {
			if sel, ok := rw.Info.Selections[x]; ok {
				if f, ok := sel.Obj().(*types.Func); ok {
					res := f.Type().(*types.Signature).Results()
					for i := 0; i < res.Len(); i++ {
						if isIface(res.At(i).Type()) {
							return "a method of it returns an interface (self, box_)"
						}
					}
				}
			}
		}
	case *ast.AssignStmt:
		for i, r := range x.Rhs {
			if rw.Unparen(r) == e && i < len(x.Lhs) && isIface(rw.Info.TypeOf(x.Lhs[i])) {
				return "assigned to an interface"
			}
		}
	case *ast.ValueSpec:
		for i, v := range x.Values {
			if rw.Unparen(v) == e && i < len(x.Names) && isIface(rw.Info.Defs[x.Names[i]].Type()) {
				return "assigned to an interface"
			}
		}
	case *ast.ReturnStmt:
		var fn ast.Node = x
		for fn != nil {
			if _, ok := fn.(*ast.FuncDecl); ok {
				break
			}
			if _, ok := fn.(*ast.FuncLit); ok {
				break
			}
			fn = rw.Parent[fn]
		}
		var sig *types.Signature
		switch f := fn.(type) {
		case *ast.FuncDecl:
			sig = rw.Info.Defs[f.Name].Type().(*types.Signature)
		case *ast.FuncLit:
			sig = rw.Info.TypeOf(f).(*types.Signature)
		}
		if sig == nil {
			return "returned"
		}
		for i, r := range x.Results {
			if rw.Unparen(r) == e && i < sig.Results().Len() && isIface(sig.Results().At(i).Type()) {
				return "returned as an interface"
			}
		}
	case *ast.CompositeLit, *ast.KeyValueExpr:
		if et := rw.Info.TypeOf(elemTarget(e)); et == nil || isIface(et) {
			return "an element of interface type"
		}
	case *ast.SendStmt:
		return "sent on a channel"
	case *ast.TypeAssertExpr:
		return ""
	}
	return ""
}

// elemTarget is the expression whose type the composite literal element e takes (its field's
// key or the literal itself); for an element of a slice or map literal the element type is
// checked through the literal's type.
func elemTarget(e ast.Expr) ast.Expr {
	p := rw.Up(e)
	if kv, ok := p.(*ast.KeyValueExpr); ok && kv.Value == e {
		if cl, ok := rw.Up(kv).(*ast.CompositeLit); ok {
			if st, ok := types.Unalias(rw.Info.TypeOf(cl)).Underlying().(*types.Struct); ok {
				if id, ok := kv.Key.(*ast.Ident); ok {
					for i := 0; i < st.NumFields(); i++ {
						if st.Field(i).Name() == id.Name && isIface(st.Field(i).Type()) {
							return nil
						}
					}
				}
				return e
			}
			return elemOf(cl, e)
		}
	}
	if cl, ok := p.(*ast.CompositeLit); ok {
		return elemOf(cl, e)
	}
	return e
}

func elemOf(cl *ast.CompositeLit, e ast.Expr) ast.Expr {
	switch t := types.Unalias(rw.Info.TypeOf(cl)).Underlying().(type) {
	case *types.Slice:
		if isIface(t.Elem()) {
			return nil
		}
	case *types.Map:
		if isIface(t.Elem()) || isIface(t.Key()) {
			return nil
		}
	case *types.Array:
		if isIface(t.Elem()) {
			return nil
		}
	}
	return e
}

func calleeIdent(f ast.Expr) *ast.Ident {
	switch x := rw.Unparen(f).(type) {
	case *ast.Ident:
		return x
	case *ast.SelectorExpr:
		return x.Sel
	case *ast.IndexExpr:
		return calleeIdent(x.X)
	case *ast.IndexListExpr:
		return calleeIdent(x.X)
	}
	return nil
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	type br struct {
		fd   *ast.FuncDecl
		cls  *types.Named
		name string
	}
	var bridges []*br
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || bridge(fd) == "" {
				continue
			}
			rt := rw.Info.Defs[fd.Name].(*types.Func).Type().(*types.Signature).Recv().Type()
			if p, ok := rt.(*types.Pointer); ok {
				rt = p.Elem()
			}
			n, ok := types.Unalias(rt).(*types.Named)
			if !ok {
				continue
			}
			bridges = append(bridges, &br{fd, n, n.Obj().Name() + "." + fd.Name.Name})
		}
	}
	// why each class's values may become interface values (computed once per class)
	why := map[*types.Named]string{}
	embeddedBy := map[*types.Named]bool{}
	for _, name := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		if st, ok := tn.Type().Underlying().(*types.Struct); ok {
			for i := 0; i < st.NumFields(); i++ {
				if f := st.Field(i); f.Embedded() {
					ft := f.Type()
					if p, ok := ft.(*types.Pointer); ok {
						ft = p.Elem()
					}
					if n, ok := types.Unalias(ft).(*types.Named); ok {
						embeddedBy[n.Origin()] = true
					}
				}
			}
		}
	}
	classes := map[*types.Named]bool{}
	for _, b := range bridges {
		classes[b.cls] = true
	}
	for n := range classes {
		if embeddedBy[n] {
			why[n] = "a type of the package embeds the class (a subclass)"
			continue
		}
		if _, ok := types.Unalias(n).Underlying().(*types.Struct); !ok {
			why[n] = "not a struct"
			continue
		}
		// the class's element conversion, if code uses it
		if v := rw.Pkg.Scope().Lookup("tc_p_" + n.Obj().Name()); v != nil && len(rw.UsesOf[v]) > 0 {
			why[n] = "its element conversion tc_p_ boxes its values for erased code"
		}
	}
	// an escape inside an unused method of the class itself (javac's default-method forwarders,
	// test(p0) -> Predicate_base_test(this, p0)) is reachable only through an escape elsewhere;
	// such methods are dead with the bridge
	unusedOwn := func(e ast.Expr, n *types.Named) *ast.FuncDecl {
		var x ast.Node = e
		for x != nil {
			if fd, ok := x.(*ast.FuncDecl); ok {
				if fd.Recv == nil {
					return nil
				}
				f := rw.Info.Defs[fd.Name].(*types.Func)
				rt := f.Type().(*types.Signature).Recv().Type()
				if p, ok := rt.(*types.Pointer); ok {
					rt = p.Elem()
				}
				if rn, ok := types.Unalias(rt).(*types.Named); ok && rn.Origin() == n.Origin() && len(rw.UsesOf[f]) == 0 && bridge(fd) == "" && !strings.HasPrefix(fd.Name.Name, "zero_") && !strings.HasPrefix(fd.Name.Name, "init_") {
					return fd
				}
				return nil
			}
			x = rw.Parent[x]
		}
		return nil
	}
	alsoDead := map[*types.Named]map[*ast.FuncDecl]bool{}
	// in source order, so that each class's reason is the first escape (deterministic)
	exprs := make([]ast.Expr, 0, len(rw.Info.Types))
	for e := range rw.Info.Types {
		exprs = append(exprs, e)
	}
	sort.Slice(exprs, func(i, j int) bool {
		if exprs[i].Pos() != exprs[j].Pos() {
			return exprs[i].Pos() < exprs[j].Pos()
		}
		return exprs[i].End() > exprs[j].End()
	})
	for _, e := range exprs {
		tv := rw.Info.Types[e]
		if tv.IsType() || tv.Type == nil {
			continue
		}
		for n := range classes {
			if why[n] != "" || !mentions(tv.Type, n, map[types.Type]bool{}) {
				continue
			}
			if w := escapes(e); w != "" {
				if fd := unusedOwn(e, n); fd != nil {
					if alsoDead[n] == nil {
						alsoDead[n] = map[*ast.FuncDecl]bool{}
					}
					alsoDead[n][fd] = true
					continue
				}
				why[n] = "a value " + w
				if verbose {
					why[n] += " (" + rw.Fset.Position(e.Pos()).String() + ")"
				}
			}
		}
	}
	dead, forwarders, left := 0, 0, map[string][]string{}
	for _, b := range bridges {
		w := why[b.cls]
		if w == "" {
			if len(rw.UsesOf[rw.Info.Defs[b.fd.Name]]) > 0 {
				w = "the bridge is called or used as a value"
			}
		}
		if w != "" {
			k := w
			if i := strings.Index(k, " ("); i >= 0 {
				k = k[:i]
			}
			left[k] = append(left[k], b.name+" "+w)
			continue
		}
		dead++
		rw.Rewrite[b.fd] = func() string { return rw.DelDoc }
		for fd := range alsoDead[b.cls] {
			if _, ok := rw.Rewrite[fd]; !ok {
				forwarders++
				rw.Rewrite[fd] = func() string { return rw.DelDoc }
				if verbose {
					fmt.Fprintf(os.Stderr, "        dead with it: %s.%s\n", b.cls.Obj().Name(), fd.Name.Name)
				}
			}
		}
		if verbose {
			fmt.Fprintf(os.Stderr, "        dead: %s\n", b.name)
		}
	}
	fmt.Fprintf(os.Stderr, "deadbridge: %d bridges no code can call are deleted, with %d unused methods of their classes that alone made their objects interface values\n", dead, forwarders)
	var ks []string
	for k := range left {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(os.Stderr, "deadbridge: left: %d %s\n", len(left[k]), k)
		if verbose {
			for _, s := range left[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", s)
			}
		}
	}
	if report || dead == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "deadbridge: files changed: %d\n", rw.SaveChecked())
}
