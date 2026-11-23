// Command casts is a post-flip step: a class cast checks the concrete types it can meet before it
// asserts to an anonymous interface, and instanceof of a class that nothing extends is a concrete
// type assertion.
//
// j2go writes a cast to class X, and instanceof X, as assertions to an anonymous interface
// (j2go itself already writes cast_[*X] and is_[*X], concrete assertions, for most classes
// without subclasses; these helpers are for the rest):
//
//	func to_X(v any) *X {
//		if v == nil {
//			return nil
//		}
//		return v.(interface{ as_X() *X }).as_X()
//	}
//
//	func is_X(v any) bool {
//		_, ok := v.(interface{ as_X() *X })
//		return ok
//	}
//
// Each is an interface type assertion: a probe of the call site's type-assertion cache (on a
// miss, runtime.typeAssert and the itab hash table), then an indirect call of as_X through the
// itab. A value boxed from a Java object is its most derived object (self()), so its dynamic type
// is one of the types in the package that have the method as_X (X and the classes that embed it)
// and that the package makes values of. When those are at most three, to_X tests them first:
//
//	func to_X(v any) *X {
//		switch p := v.(type) {
//		case *X:
//			return p
//		case *Y:
//			return p.as_X()
//		case nil:
//			return nil
//		}
//		return v.(interface{ as_X() *X }).as_X()
//	}
//
// A case compares the type word (after a load of the type's hash, which the cache probe loads
// too) and calls as_X statically; the original assertion stays as the last line, for any other
// value, so a cast that fails panics with the same runtime error, which catch_ turns into the same
// ClassCastException. With one case the helper still inlines (cost 75-78 of 80); with two or
// three it is a direct call doing less than the inlined probe and its indirect call.
//
// When *X is the only type with as_X, is_X is exact as a concrete assertion:
//
//	func is_X(v any) bool {
//		_, ok := v.(*X)
//		return ok
//	}
//
// is_X of an extended class is kept: a false answer would pay the cases and then the probe.
// Larger hierarchies (JSType: 24 types made) keep their helpers too: a type switch over them is a
// jump table on the type hash, about the cost of the probe, and too big to inline. An assertion
// to a named interface instead of the anonymous one compiles to the same cache probe and itab
// (both are one interface type descriptor), so it would gain nothing. icast_X asserts to a Java
// interface and its result is an interface value, which needs the itab.
//
// Only g_*.go files are rewritten; the helpers of the handwritten runtime are listed. Rewritten
// helpers no longer match, so running the step again changes nothing.
//
// usage: casts [-n] <dir of package closure>
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

// assertedIface returns the interface type of e if e is v.(interface{ as_X() *X }) for the
// parameter v, with X the named type x.
func assertedIface(e ast.Expr, v types.Object, x *types.Named) *types.Interface {
	ta, ok := rw.Unparen(e).(*ast.TypeAssertExpr)
	if !ok || ta.Type == nil {
		return nil
	}
	if id, ok := ta.X.(*ast.Ident); !ok || rw.Info.Uses[id] != v {
		return nil
	}
	it, ok := rw.Info.Types[ta.Type].Type.(*types.Interface)
	if !ok || it.NumMethods() != 1 || it.Method(0).Name() != "as_"+x.Obj().Name() {
		return nil
	}
	sig := it.Method(0).Type().(*types.Signature)
	if sig.Params().Len() != 0 || sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), types.NewPointer(x)) {
		return nil
	}
	return it
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	const maxFast = 3
	rw.Load(os.Args[len(os.Args)-1])

	// Pointers to the package's non-generic, non-interface named types: the dynamic types a
	// value of a Java class can have.
	var ptrs []*types.Pointer
	for _, n := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		nt, ok := tn.Type().(*types.Named)
		if !ok || nt.TypeParams().Len() > 0 {
			continue
		}
		if _, ok := nt.Underlying().(*types.Interface); ok {
			continue
		}
		ptrs = append(ptrs, types.NewPointer(nt))
	}
	implementers := map[*types.Interface][]string{}
	impl := func(it *types.Interface) []string {
		if s, ok := implementers[it]; ok {
			return s
		}
		var s []string
		for _, p := range ptrs {
			if types.Implements(p, it) {
				s = append(s, p.Elem().(*types.Named).Obj().Name())
			}
		}
		implementers[it] = s
		return s
	}

	// Types the package makes values of (a composite literal or new(T) outside a function
	// that nothing calls, such as the constructor of an abstract class); the others are only
	// ever part of a value of a subclass.
	instantiable := map[string]bool{}
	for _, d := range decls() {
		if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && len(rw.UsesOf[rw.Info.Defs[fd.Name]]) == 0 {
			continue
		}
		ast.Inspect(d, func(n ast.Node) bool {
			var t types.Type
			switch e := n.(type) {
			case *ast.CompositeLit:
				t = rw.Info.Types[e].Type
			case *ast.CallExpr:
				if id, ok := e.Fun.(*ast.Ident); ok && id.Name == "new" && len(e.Args) == 1 {
					t = rw.Info.Types[e.Args[0]].Type
				}
			}
			if nt, ok := t.(*types.Named); ok && nt.Obj().Pkg() == rw.Pkg {
				instantiable[nt.Obj().Name()] = true
			}
			return true
		})
	}

	calls := map[string]int{}
	for o, us := range rw.UsesOf {
		if f, ok := o.(*types.Func); ok && f.Pkg() == rw.Pkg && f.Type().(*types.Signature).Recv() == nil {
			calls[f.Name()] += len(us)
		}
	}

	type helper struct {
		name, file string
		sites      int
		what       string
	}
	var changed, kept, runtime []helper
	for _, f := range rw.Files {
		file := filepath.Base(rw.Fset.File(f.Pos()).Name())
		gen := strings.HasPrefix(file, "g_")
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Body == nil || fd.Type.TypeParams != nil {
				continue
			}
			name := fd.Name.Name
			var cls string
			switch {
			case strings.HasPrefix(name, "to_"):
				cls = name[3:]
			case strings.HasPrefix(name, "is_"):
				cls = name[3:]
			default:
				continue
			}
			fn := rw.Info.Defs[fd.Name].(*types.Func)
			sig := fn.Type().(*types.Signature)
			if sig.Params().Len() != 1 || !types.Identical(sig.Params().At(0).Type(), types.NewInterfaceType(nil, nil)) {
				continue
			}
			tn, ok := rw.Pkg.Scope().Lookup(cls).(*types.TypeName)
			if !ok {
				continue
			}
			x, ok := tn.Type().(*types.Named)
			if !ok {
				continue
			}
			v := sig.Params().At(0)
			pname := fd.Type.Params.List[0].Names[0].Name
			body := fd.Body.List
			var it *types.Interface
			var isGen func() string
			var slow string
			isTo := name[:3] == "to_"
			if isTo {
				// if v == nil { return nil }; return v.(I).as_X()
				if len(body) != 2 {
					continue
				}
				ret, ok := body[1].(*ast.ReturnStmt)
				if !ok || len(ret.Results) != 1 {
					continue
				}
				call, ok := ret.Results[0].(*ast.CallExpr)
				if !ok || len(call.Args) != 0 {
					continue
				}
				sel, ok := call.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "as_"+cls {
					continue
				}
				if it = assertedIface(sel.X, v, x); it == nil {
					continue
				}
				ifs, ok := body[0].(*ast.IfStmt)
				if !ok || ifs.Init != nil || ifs.Else != nil || rw.Text(ifs.Cond) != pname+" == nil" || rw.Text(ifs.Body) != "{\n\t\treturn nil\n\t}" {
					continue
				}
				slow = rw.Text(ret)
			} else {
				// _, ok := v.(I); return ok
				if len(body) != 2 {
					continue
				}
				as, ok := body[0].(*ast.AssignStmt)
				if !ok || len(as.Lhs) != 2 || len(as.Rhs) != 1 || rw.Text(as.Lhs[0]) != "_" {
					continue
				}
				if it = assertedIface(as.Rhs[0], v, x); it == nil {
					continue
				}
				ok_ := rw.Text(as.Lhs[1])
				if ret, ok := body[1].(*ast.ReturnStmt); !ok || len(ret.Results) != 1 || rw.Text(ret.Results[0]) != ok_ {
					continue
				}
				isGen = func() string {
					return fmt.Sprintf("{\n\t_, %s := %s.(*%s)\n\treturn %s\n}", ok_, pname, cls, ok_)
				}
			}
			h := helper{name, file, calls[name], ""}
			impls := impl(it)
			var inst []string
			for _, t := range impls {
				if instantiable[t] {
					inst = append(inst, t)
				}
			}
			// The types a cast checks first: X itself when nothing extends X, else the types
			// that are made when there are at most three (X first if it is one of them).
			var fast []string
			switch {
			case len(impls) == 1 && impls[0] == cls:
				fast = []string{cls}
			case len(inst) <= maxFast:
				fast = inst
				sort.SliceStable(fast, func(i, j int) bool { return fast[i] == cls && fast[j] != cls })
			}
			switch {
			case !gen:
				h.what = "runtime, not rewritten"
				runtime = append(runtime, h)
				continue
			case len(impls) == 1 && impls[0] == cls:
				h.what = "concrete"
			case isTo && len(fast) > 0:
				h.what = fmt.Sprintf("checks %s first (%d types have as_%s)", strings.Join(fast, " "), len(impls), cls)
			default:
				h.what = fmt.Sprintf("kept (%d types have as_%s, %d made)", len(impls), cls, len(inst))
				kept = append(kept, h)
				continue
			}
			changed = append(changed, h)
			if isTo {
				var sb strings.Builder
				fmt.Fprintf(&sb, "{\n\tswitch p := %s.(type) {\n", pname)
				for _, t := range fast {
					ret := "p"
					if t != cls {
						ret = "p.as_" + cls + "()"
					}
					fmt.Fprintf(&sb, "\tcase *%s:\n\t\treturn %s\n", t, ret)
				}
				fmt.Fprintf(&sb, "\tcase nil:\n\t\treturn nil\n\t}\n\t%s\n}", slow)
				rw.Rewrite[fd.Body] = func() string { return sb.String() }
			} else {
				rw.Rewrite[fd.Body] = isGen
			}
		}
	}

	if report {
		all := append(append(append([]helper{}, changed...), kept...), runtime...)
		sort.Slice(all, func(i, j int) bool { return all[i].name < all[j].name })
		for _, h := range all {
			fmt.Printf("%s\t%s\t%d sites\t%s\n", h.name, h.file, h.sites, h.what)
		}
	}
	count := func(hs []helper, prefix, what string) (n, sites int) {
		for _, h := range hs {
			if strings.HasPrefix(h.name, prefix) && strings.HasPrefix(h.what, what) {
				n++
				sites += h.sites
			}
		}
		return
	}
	for _, p := range []string{"to_", "is_"} {
		cn, cs := count(changed, p, "concrete")
		fn, fs := count(changed, p, "checks")
		kn, ks := count(kept, p, "")
		rn, rs := count(runtime, p, "")
		fmt.Fprintf(os.Stderr, "%sX helpers: %d of classes nothing extends made concrete (%d call sites), %d of extended classes check the types made first (%d call sites), %d kept (%d call sites); %d in the runtime, not rewritten (%d call sites)\n", p, cn, cs, fn, fs, kn, ks, rn, rs)
	}
	if !report {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
	}
}

func decls() []ast.Decl {
	var ds []ast.Decl
	for _, f := range rw.Files {
		ds = append(ds, f.Decls...)
	}
	return ds
}
