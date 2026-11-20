// Command nnassert is a post-flip step: castfold -assert's r.m(a).(*T).f, the assertion of a
// bridge's result dereferenced at once, is the typed method's result with Java's null check
// before the dereference, where m is a bridge of a typed method mT_ returning *T (covret,
// fbound, selftype: `return iup_I(this.mT_(..))`, obj_(..), this.mT_(..).up_B()).
//
//	v.getScope().(*Scope).getClosestHoistScope()  ->  requireNonNull0_(v.getScopeT_()).getClosestHoistScope()
//	name.getDeclaration().(*GlobalNamespace_Ref).node_  ->  name.getDeclarationT_().node_
//
// Java's ((Scope) v.getScope()).getClosestHoistScope() throws NullPointerException when the
// scope is null, at the call, after the receiver (and in Java the arguments) are evaluated. The
// assertion of a nil interface panics where the receiver is evaluated, and the runtime turns
// that panic into the NullPointerException (catchNilAssert_); the cast to *T itself cannot fail,
// since the bridge boxes the typed method's *T. So the typed result needs only the null check:
//
//   - where the dereference is a field, or a method whose body reads a field of its receiver
//     before it calls anything (fbound's rule; for a bridge, its typed method's body), the nil
//     dereference is where the assertion panicked and gives the NullPointerException: the typed
//     result as it is;
//   - otherwise (the method hands its receiver on first: getClosestHoistScope returns nil for a
//     nil scope), requireNonNull0_(r.mT_(a)) throws the NullPointerException at the same point,
//     before anything after the receiver is evaluated, as the assertion did.
//
// The same holds for x.self().(*T) of an x of type *T whose class nothing embeds (x.self() is
// x, nil for nil: the assertion is only Java's null check): x, or requireNonNull0_(x).
//
// Left, counted: assertions of results of methods that are not such a bridge (real downcasts of
// erased results: collections' elements, Iterables.getFirst, ...), assertions to another type
// than the typed method's. A rerun changes nothing.
//
// usage: nnassert [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

var (
	verbose bool
	counts  = map[string]int{}
	lefts   = map[string][]string{}
	declOf  = map[*types.Func]*ast.FuncDecl{}
	// the struct types another struct embeds
	embedded = map[types.Type]bool{}
)

func left(why string, n ast.Node) {
	lefts[why] = append(lefts[why], rw.Fset.Position(n.Pos()).String())
}

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
				if fo, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					declOf[fo] = fd
				}
			}
		}
	}
	for _, n := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		if st, ok := tn.Type().Underlying().(*types.Struct); ok {
			for i := 0; i < st.NumFields(); i++ {
				if f := st.Field(i); f.Embedded() {
					embedded[types.Unalias(f.Type())] = true
					if nt, ok := types.Unalias(f.Type()).(*types.Named); ok {
						embedded[nt.Origin()] = true
					}
				}
			}
		}
	}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			ta, ok := n.(*ast.TypeAssertExpr)
			if !ok || ta.Type == nil {
				return true
			}
			deref, ok := rw.Up(ta).(*ast.SelectorExpr)
			if !ok || rw.Unparen(deref.X) != ta {
				return true
			}
			c, ok := rw.Unparen(ta.X).(*ast.CallExpr)
			if !ok {
				return true
			}
			// x.self().(*T) with x a *T of a class nothing embeds: x.self() is x (nil for nil)
			if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "self" && len(c.Args) == 0 {
				at := rw.Info.TypeOf(ta.Type)
				if !types.Identical(rw.Info.TypeOf(s.X), at) {
					left("an assertion of self() to another class (a downcast)", ta)
					return true
				}
				if pt, ok := at.(*types.Pointer); !ok || embedded[pt.Elem()] {
					left("an assertion of self() of an object of a class something embeds", ta)
					return true
				}
				check := !derefsAtOnce(deref)
				if check {
					counts["assertions x.self().(*T) of a *T are x under requireNonNull0_ (the dereference hands its receiver on first)"]++
				} else {
					counts["assertions x.self().(*T) of a *T are x (the dereference is a field read)"]++
				}
				x := s.X
				rw.Rewrite[ta] = func() string {
					if check {
						return "requireNonNull0_(" + rw.Render(x) + ")"
					}
					return rw.Render(x)
				}
				return true
			}
			ms, ok := c.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			sel := rw.Info.Selections[ms]
			if sel == nil || sel.Kind() != types.MethodVal {
				return true
			}
			fo := sel.Obj().(*types.Func)
			tname := strings.TrimSuffix(fo.Name(), "_impl") + "T_"
			o, _, _ := types.LookupFieldOrMethod(rw.Info.TypeOf(ms.X), true, rw.Pkg, tname)
			tm, ok := o.(*types.Func)
			if !ok || !isBridge(declOf[fo], tname) {
				left("an assertion of a result of a method that is not a typed method's bridge", ta)
				return true
			}
			tsig := tm.Type().(*types.Signature)
			at := rw.Info.TypeOf(ta.Type)
			if tsig.Results().Len() != 1 || !types.Identical(tsig.Results().At(0).Type(), at) {
				left("an assertion to another type than the typed method's result", ta)
				return true
			}
			check := !derefsAtOnce(deref)
			if check {
				counts["assertions of a bridge's result are the typed result under requireNonNull0_ (the dereference hands its receiver on first)"]++
			} else {
				counts["assertions of a bridge's result are the typed result (the dereference is a field read: the nil dereference panics there)"]++
			}
			rw.Rewrite[ta] = func() string {
				var args []string
				for _, a := range c.Args {
					args = append(args, rw.Render(a))
				}
				s := rw.Render(ms.X) + "." + tname + "(" + strings.Join(args, ", ") + ")"
				if check {
					return "requireNonNull0_(" + s + ")"
				}
				return s
			}
			return true
		})
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", counts[k], k)
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
	if report || len(counts) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "nnassert: files changed: %d\n", rw.SaveChecked())
}

// isBridge: fd's body is `return B(this.<tname>(params...))` with B one of iup_I(..), obj_(..),
// (..).up_X(): the typed method's result boxed, nil kept nil.
func isBridge(fd *ast.FuncDecl, tname string) bool {
	if fd == nil || fd.Body == nil || len(fd.Body.List) != 1 {
		return false
	}
	r, ok := fd.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(r.Results) != 1 {
		return false
	}
	b, ok := rw.Unparen(r.Results[0]).(*ast.CallExpr)
	if !ok {
		return false
	}
	var inner ast.Expr
	name, _ := rw.CallName(b)
	switch {
	case (strings.HasPrefix(name, "iup_") || name == "obj_" || strings.HasPrefix(name, "obj_[")) && len(b.Args) == 1:
		inner = b.Args[0]
	case len(b.Args) == 0:
		if s, ok := b.Fun.(*ast.SelectorExpr); ok && strings.HasPrefix(s.Sel.Name, "up_") {
			inner = s.X
		}
	}
	if inner == nil {
		return false
	}
	ic, ok := rw.Unparen(inner).(*ast.CallExpr)
	if !ok {
		return false
	}
	s, ok := ic.Fun.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != tname {
		return false
	}
	recv := rw.Info.Defs[fd.Recv.List[0].Names[0]]
	if id, ok := s.X.(*ast.Ident); !ok || rw.Info.Uses[id] != recv {
		return false
	}
	var params []types.Object
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			params = append(params, rw.Info.Defs[n])
		}
	}
	if len(ic.Args) != len(params) {
		return false
	}
	for i, a := range ic.Args {
		if id, ok := rw.Unparen(a).(*ast.Ident); !ok || rw.Info.Uses[id] != params[i] {
			return false
		}
	}
	return true
}

// derefsAtOnce: the selector reads a field, or calls a method whose body's first statement
// reads a field of its receiver (this.f) before any call: a nil receiver panics there.
func derefsAtOnce(sel *ast.SelectorExpr) bool {
	s := rw.Info.Selections[sel]
	if s == nil {
		return false
	}
	if s.Kind() == types.FieldVal {
		return true
	}
	if s.Kind() != types.MethodVal {
		return false
	}
	if _, ok := s.Recv().Underlying().(*types.Interface); ok {
		return false
	}
	if len(s.Index()) != 1 {
		return false // promoted through an embedded field: the receiver is reached first
	}
	fd := declOf[s.Obj().(*types.Func)]
	if fd == nil || fd.Body == nil || len(fd.Body.List) == 0 || fd.Recv == nil || len(fd.Recv.List[0].Names) == 0 {
		return false
	}
	// a bridge's first action is the call of its typed method on this: look at that one
	tname := strings.TrimSuffix(fd.Name.Name, "_impl") + "T_"
	if isBridge(fd, tname) {
		if o, idx, _ := types.LookupFieldOrMethod(s.Recv(), true, rw.Pkg, tname); len(idx) == 1 {
			if tf, ok := o.(*types.Func); ok && declOf[tf] != nil {
				fd = declOf[tf]
				if fd.Body == nil || len(fd.Body.List) == 0 || len(fd.Recv.List[0].Names) == 0 {
					return false
				}
			}
		}
	}
	recv := rw.Info.Defs[fd.Recv.List[0].Names[0]]
	// the first field read of this in evaluation order comes before any call
	first := ""
	ast.Inspect(fd.Body.List[0], func(n ast.Node) bool {
		if first != "" {
			return false
		}
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.CallExpr:
			// the callee's operands are evaluated first; look at them, then the call
			ast.Inspect(x.Fun, func(m ast.Node) bool {
				if first != "" {
					return false
				}
				if fs, ok := m.(*ast.SelectorExpr); ok && isFieldOfRecv(fs, recv) {
					first = "field"
					return false
				}
				return true
			})
			for _, a := range x.Args {
				ast.Inspect(a, func(m ast.Node) bool {
					if first != "" {
						return false
					}
					if fs, ok := m.(*ast.SelectorExpr); ok && isFieldOfRecv(fs, recv) {
						first = "field"
						return false
					}
					if _, ok := m.(*ast.CallExpr); ok {
						first = "call"
						return false
					}
					return true
				})
			}
			if first == "" {
				first = "call"
			}
			return false
		case *ast.SelectorExpr:
			if isFieldOfRecv(x, recv) {
				first = "field"
				return false
			}
		}
		return true
	})
	return first == "field"
}

func isFieldOfRecv(s *ast.SelectorExpr, recv types.Object) bool {
	id, ok := s.X.(*ast.Ident)
	if !ok || rw.Info.Uses[id] != recv {
		return false
	}
	ss := rw.Info.Selections[s]
	return ss != nil && ss.Kind() == types.FieldVal
}
