// Command covret is a post-flip step: a method whose result Java declares narrower than the
// method it overrides or implements (a covariant return) gets that type back; the method with
// the wide result stays as the bridge javac makes.
//
//	func (this *GlobalNamespace_Name) getDeclaration() StaticRef {
//		return iup_StaticRef(this.declaration_)
//	}
//	cast_[*GlobalNamespace_Ref](name.getDeclaration())
//	  ->
//	func (this *GlobalNamespace_Name) getDeclarationT_() *GlobalNamespace_Ref {
//		return this.declaration_
//	}
//	func (this *GlobalNamespace_Name) getDeclaration() StaticRef {
//		return iup_StaticRef(this.getDeclarationT_())
//	}
//	name.getDeclarationT_()
//
// Java's `Ref getDeclaration()` in GlobalNamespace.Name implements StaticSlot's `StaticRef
// getDeclaration()`; javac compiles it to the method returning Ref and a bridge returning
// StaticRef that calls it. j2go writes one Go method with the interface's signature, so a
// caller that has a Name and wants a Ref casts the result back. Here the method is split as
// javac splits it: the typed method (name + T_) holds the body with each returned value
// unboxed, the method with the interface's signature boxes its result, and the casts of the
// result by callers on the class are calls of the typed method.
//
// A method m of a class C (a pointer receiver, C not generic) whose single result type R is an
// interface (or any) is split when every return statement returns nil or B(x) with one boxing
// B for all: iup_R (castfold -nilup: nil for a nil x, else R holding x) or obj_ (R any), and
// every x of one pointer type T; and no class that embeds C declares m (no override: every
// call of m on a C runs this body). nil is T's nil, which B boxes to the nil the body
// returned. A call K(r.m(...)) with K a cast to exactly T (cast_[T], to_X with *X = T) and r of
// type *C, or of a class embedding C that does not declare m, is r.mT_(...): B(x) cast to T is
// x (B keeps nil as nil, the cast gives nil; a non-nil x boxes to an interface holding x, which
// the cast to T returns, and cannot fail since x is a T). The arguments and the receiver are
// evaluated as before, once.
//
// Only methods with at least one such cast are split. Left, counted: methods whose returns
// box more than one type or in another way, overridden methods, casts to another type than
// the returned one. A rerun changes nothing (the calls no longer cast).
//
// usage: covret [-n] [-v] <dir of package closure>
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

var qual = func(p *types.Package) string {
	if p == rw.Pkg {
		return ""
	}
	return p.Name()
}

type method struct {
	fd      *ast.FuncDecl
	obj     *types.Func
	class   *types.Named
	res     types.Type
	box     string // "iup_R" or "obj_"
	elem    types.Type
	returns []*ast.ReturnStmt
	calls   []*ast.CallExpr // the casts K(r.m(...)) to rewrite
}

// castTo returns the type a cast call converts to, and its operand, or nil.
func castTo(c *ast.CallExpr) (types.Type, ast.Expr) {
	if len(c.Args) != 1 {
		return nil, nil
	}
	switch f := c.Fun.(type) {
	case *ast.Ident:
		if !strings.HasPrefix(f.Name, "to_") {
			return nil, nil
		}
	case *ast.IndexExpr:
		id, ok := f.X.(*ast.Ident)
		if !ok || id.Name != "cast_" {
			return nil, nil
		}
	default:
		return nil, nil
	}
	o := rw.Info.Uses[funIdent(c.Fun)]
	if fo, ok := o.(*types.Func); !ok || fo.Pkg() != rw.Pkg || fo.Parent() != rw.Pkg.Scope() {
		return nil, nil
	}
	sig, ok := rw.Info.TypeOf(c.Fun).(*types.Signature)
	if !ok || sig.Results().Len() != 1 || sig.Params().Len() != 1 {
		return nil, nil
	}
	if it, ok := sig.Params().At(0).Type().Underlying().(*types.Interface); !ok || !it.Empty() {
		return nil, nil
	}
	return sig.Results().At(0).Type(), c.Args[0]
}

func funIdent(e ast.Expr) *ast.Ident {
	switch f := e.(type) {
	case *ast.Ident:
		return f
	case *ast.IndexExpr:
		id, _ := f.X.(*ast.Ident)
		return id
	}
	return nil
}

func main() {
	report, verbose := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	left := map[string]int{}
	leftAt := map[string][]string{}
	skip := func(why string, n ast.Node) {
		left[why]++
		if verbose {
			leftAt[why] = append(leftAt[why], rw.Fset.Position(n.Pos()).String())
		}
	}

	// embedders: for each class, the classes that embed it, transitively.
	embedders := map[*types.Named][]*types.Named{}
	for _, n := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		nt, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		var walk func(t *types.Named)
		walk = func(t *types.Named) {
			st, ok := t.Underlying().(*types.Struct)
			if !ok {
				return
			}
			for i := 0; i < st.NumFields(); i++ {
				f := st.Field(i)
				if !f.Embedded() {
					continue
				}
				if e, ok := types.Unalias(f.Type()).(*types.Named); ok {
					if _, ok := e.Underlying().(*types.Struct); ok {
						embedders[e.Origin()] = append(embedders[e.Origin()], nt)
						walk(e)
					}
				}
			}
		}
		walk(nt)
	}
	declares := func(nt *types.Named, name string) bool {
		for i := 0; i < nt.NumMethods(); i++ {
			if nt.Method(i).Name() == name {
				return true
			}
		}
		return false
	}

	methods := map[*types.Func]*method{}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Body == nil || fd.Type.TypeParams != nil {
				continue
			}
			obj := rw.Info.Defs[fd.Name].(*types.Func)
			sig := obj.Type().(*types.Signature)
			pt, ok := sig.Recv().Type().(*types.Pointer)
			if !ok {
				continue
			}
			class, ok := pt.Elem().(*types.Named)
			if !ok || class.TypeParams().Len() > 0 || class.TypeArgs().Len() > 0 {
				continue
			}
			if sig.Results().Len() != 1 || !types.IsInterface(sig.Results().At(0).Type()) {
				continue
			}
			methods[obj] = &method{fd: fd, obj: obj, class: class, res: sig.Results().At(0).Type()}
		}
	}
	// the casts of results
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			t, arg := castTo(c)
			if t == nil {
				return true
			}
			inner, ok := rw.Unparen(arg).(*ast.CallExpr)
			if !ok {
				return true
			}
			sel, ok := inner.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			s := rw.Info.Selections[sel]
			if s == nil || s.Kind() != types.MethodVal {
				return true
			}
			m := methods[s.Obj().(*types.Func)]
			if m == nil {
				return true
			}
			m.calls = append(m.calls, c)
			_ = t
			return true
		})
	}
	var done []*method
	ncalls := 0
	for _, m := range methods {
		if len(m.calls) == 0 {
			continue
		}
		why := ""
		// returns
		ast.Inspect(m.fd.Body, func(n ast.Node) bool {
			if _, ok := n.(*ast.FuncLit); ok {
				return false
			}
			r, ok := n.(*ast.ReturnStmt)
			if !ok {
				return true
			}
			m.returns = append(m.returns, r)
			if why != "" {
				return false
			}
			if len(r.Results) != 1 {
				why = "a return without one value"
				return false
			}
			e := rw.Unparen(r.Results[0])
			if id, ok := e.(*ast.Ident); ok && id.Name == "nil" && rw.Info.Uses[id] == types.Universe.Lookup("nil") {
				return false
			}
			c, ok := e.(*ast.CallExpr)
			if !ok || len(c.Args) != 1 {
				why = "a returned value not boxed by iup_ or obj_"
				return false
			}
			name, _ := rw.CallName(c)
			box := ""
			resName := ""
			if nt, ok := types.Unalias(m.res).(*types.Named); ok {
				resName = nt.Obj().Name()
			}
			switch {
			case resName != "" && name == "iup_"+resName:
				box = name
			case name == "obj_" && isAnyType(m.res):
				box = name
			default:
				why = "a returned value not boxed by iup_ or obj_"
				return false
			}
			xt := rw.Info.TypeOf(c.Args[0])
			if _, ok := xt.(*types.Pointer); !ok {
				why = "a returned value not a pointer"
				return false
			}
			if m.box != "" && (m.box != box || !types.Identical(m.elem, xt)) {
				why = "returns of two types or boxings"
				return false
			}
			m.box, m.elem = box, xt
			return false
		})
		if why == "" && m.box == "" {
			why = "returns only nil"
		}
		if why == "" {
			for _, e := range embedders[m.class] {
				if declares(e, m.fd.Name.Name) {
					why = "overridden by a class that embeds it"
					break
				}
			}
		}
		if why == "" {
			if o, _, _ := types.LookupFieldOrMethod(types.NewPointer(m.class), true, rw.Pkg, m.fd.Name.Name+"T_"); o != nil {
				why = "the typed name is taken"
			}
		}
		if why != "" {
			for _, c := range m.calls {
				skip("the method: "+why, c)
			}
			continue
		}
		var keep []*ast.CallExpr
		for _, c := range m.calls {
			t, _ := castTo(c)
			if !types.Identical(t, m.elem) {
				skip("a cast to another type than the returned one", c)
				continue
			}
			keep = append(keep, c)
		}
		m.calls = keep
		if len(keep) == 0 {
			continue
		}
		done = append(done, m)
		ncalls += len(keep)
	}
	sort.Slice(done, func(i, j int) bool { return done[i].fd.Pos() < done[j].fd.Pos() })
	for _, m := range done {
		m := m
		tname := m.fd.Name.Name + "T_"
		for _, c := range m.calls {
			c := c
			inner := rw.Unparen(c.Args[0]).(*ast.CallExpr)
			sel := inner.Fun.(*ast.SelectorExpr)
			rw.Rewrite[c] = func() string {
				var args []string
				for _, a := range inner.Args {
					args = append(args, rw.Render(a))
				}
				return rw.Render(sel.X) + "." + tname + "(" + strings.Join(args, ", ") + ")"
			}
		}
		rw.Rewrite[m.fd] = func() string {
			// the typed method: the body with each B(x) returned as x
			for _, r := range m.returns {
				r := r
				if len(r.Results) == 1 {
					if c, ok := rw.Unparen(r.Results[0]).(*ast.CallExpr); ok {
						rw.Rewrite[r.Results[0]] = func() string { return rw.Render(c.Args[0]) }
					}
				}
			}
			recv := rw.Text(m.fd.Recv)
			params := rw.Text(m.fd.Type.Params)
			var names []string
			for _, f := range m.fd.Type.Params.List {
				for _, n := range f.Names {
					names = append(names, n.Name)
				}
			}
			body := rw.Render(m.fd.Body)
			et := types.TypeString(m.elem, qual)
			return "func " + recv + " " + tname + params + " " + et + " " + body + "\n\n" +
				"// " + m.fd.Name.Name + " is " + tname + " with the result of the interface's type (javac's bridge; covret).\n" +
				"func " + recv + " " + m.fd.Name.Name + params + " " + types.TypeString(m.res, qual) + " {\n\treturn " + m.box + "(this." + tname + "(" + strings.Join(names, ", ") + "))\n}"
		}
	}

	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	nleft := 0
	for _, k := range keys {
		nleft += left[k]
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
		if verbose {
			for _, p := range leftAt[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	if verbose {
		for _, m := range done {
			fmt.Fprintf(os.Stderr, "  %3d %s.%s -> %s\n", len(m.calls), m.class.Obj().Name(), m.fd.Name.Name, types.TypeString(m.elem, qual))
		}
	}
	fmt.Fprintf(os.Stderr, "covret: %d methods split, %d casts of their results are calls of the typed method; %d casts left\n", len(done), ncalls, nleft)
	if report || len(done) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "covret: files changed: %d\n", rw.SaveChecked())
}

func isAnyType(t types.Type) bool {
	it, ok := t.Underlying().(*types.Interface)
	return ok && it.Empty()
}
