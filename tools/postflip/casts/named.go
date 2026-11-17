package main

// casts -named: the class casts that remain assert to a named interface, and cast helpers that
// nothing calls are removed.
//
// After the steps that fold casts at their call sites (castfold, instof), most of j2go's helpers
// to_X, is_X (one per class) and icast_I (one per Java interface) have no caller left; they are
// removed (a function nothing calls has no behaviour). Helpers the runtime's tests call are kept.
//
// The helpers that remain, for a class X with subclasses, assert to interface{ as_X() *X }:
// the types that have the method as_X are X and the classes that embed it, so the assertion is
// the Java instanceof X / cast to X. The step declares that interface under a name, X_as, next
// to the method as_X, and every assertion of that form in generated code asserts to it: the same
// method set, so the same values pass and the same as_X is called. (The vdispatch step's X_I
// cannot take as_X: X itself and abstract classes of the hierarchy lack some of its methods, and
// any(this) in a base method boxes such a pointer.) An assertion to a named interface compiles to
// the same type-assertion cache probe and itab as one to an anonymous interface. A failing one
// panics with the same runtime error, which catch_ turns into the same ClassCastException; only
// the interface's name in its Go message changes ("closure.X_as" for
// "interface { as_X() *closure.X }"), and no Java code reads that message (nothing in the
// package catches ClassCastException).
//
// Running the step again changes nothing: no anonymous assertion is left, and the helpers left
// are called.

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"postflip/rw"
)

func named(dry bool, dir string) {
	gen := func(n ast.Node) bool {
		return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "g_")
	}
	anyT := types.NewInterfaceType(nil, nil)
	// rw.Load leaves out the runtime's tests; the helpers they call are kept.
	inTests := map[string]bool{}
	tests, _ := filepath.Glob(filepath.Join(dir, "*_test.go"))
	for _, t := range tests {
		b, err := os.ReadFile(t)
		if err != nil {
			panic(err)
		}
		for _, m := range helperCall.FindAllStringSubmatch(string(b), -1) {
			inTests[m[1]] = true
		}
	}

	// 1. Helpers that nothing calls.
	removed := map[string]int{}
	for _, f := range rw.Files {
		if !gen(f) {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Type.TypeParams != nil {
				continue
			}
			var kind string
			for _, p := range []string{"to_", "is_", "icast_"} {
				if strings.HasPrefix(fd.Name.Name, p) {
					kind = p
				}
			}
			if kind == "" {
				continue
			}
			fn := rw.Info.Defs[fd.Name].(*types.Func)
			sig := fn.Type().(*types.Signature)
			if sig.Params().Len() != 1 || !types.Identical(sig.Params().At(0).Type(), anyT) || sig.Results().Len() != 1 {
				continue
			}
			if _, ok := rw.Pkg.Scope().Lookup(fd.Name.Name[len(kind):]).(*types.TypeName); !ok {
				continue
			}
			if len(rw.UsesOf[fn]) > 0 || inTests[fd.Name.Name] {
				continue
			}
			removed[kind]++
			rw.Rewrite[fd] = func() string { return rw.Del }
		}
	}

	// 2. Assertions to interface{ as_X() *X }.
	type class struct {
		x     *types.Named
		sites int
	}
	classes := map[*types.Named]*class{}
	for _, f := range rw.Files {
		if !gen(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if fd, ok := n.(*ast.FuncDecl); ok && rw.Rewrite[fd] != nil {
				return false // removed
			}
			ta, ok := n.(*ast.TypeAssertExpr)
			if !ok || ta.Type == nil {
				return true
			}
			if _, ok := ta.Type.(*ast.InterfaceType); !ok {
				return true
			}
			it, ok := rw.Info.TypeOf(ta.Type).(*types.Interface)
			if !ok || it.NumMethods() != 1 || !strings.HasPrefix(it.Method(0).Name(), "as_") {
				return true
			}
			sig := it.Method(0).Type().(*types.Signature)
			if sig.Params().Len() != 0 || sig.Results().Len() != 1 {
				return true
			}
			pt, ok := sig.Results().At(0).Type().(*types.Pointer)
			if !ok {
				return true
			}
			x, ok := pt.Elem().(*types.Named)
			if !ok || x.Obj().Pkg() != rw.Pkg || it.Method(0).Name() != "as_"+x.Obj().Name() {
				return true
			}
			c := classes[x]
			if c == nil {
				c = &class{x: x}
				classes[x] = c
			}
			c.sites++
			name := x.Obj().Name() + "_as"
			rw.Rewrite[ta.Type] = func() string { return name }
			return true
		})
	}
	var names []string
	byName := map[string]*class{}
	for x, c := range classes {
		names = append(names, x.Obj().Name())
		byName[x.Obj().Name()] = c
	}
	sort.Strings(names)
	declared, sites := 0, 0
	for _, n := range names {
		c := byName[n]
		sites += c.sites
		iname := n + "_as"
		asX := "as_" + n
		if o := rw.Pkg.Scope().Lookup(iname); o != nil {
			// declared by an earlier run: it must be the interface of as_X alone
			if it, ok := o.Type().Underlying().(*types.Interface); !ok || it.NumMethods() != 1 || it.Method(0).Name() != asX {
				fmt.Fprintf(os.Stderr, "casts -named: %s is already declared\n", iname)
				os.Exit(1)
			}
			continue
		}
		// X_as, before the method as_X of X (or the first helper, when as_X is not in
		// generated code)
		m, _, _ := types.LookupFieldOrMethod(types.NewPointer(c.x), false, rw.Pkg, asX)
		var at ast.Node
		if f, ok := m.(*types.Func); ok {
			at = funcDeclOf(f)
		}
		if at == nil || !gen(at) {
			for _, h := range []string{"to_" + n, "is_" + n} {
				if f, ok := rw.Pkg.Scope().Lookup(h).(*types.Func); ok && rw.Rewrite[funcDeclOf(f)] == nil && gen(funcDeclOf(f)) {
					at = funcDeclOf(f)
					break
				}
			}
		}
		if at == nil {
			fmt.Fprintf(os.Stderr, "casts -named: no place to declare %s\n", iname)
			os.Exit(1)
		}
		declared++
		fd := at.(*ast.FuncDecl)
		if fd.Doc != nil {
			fmt.Fprintf(os.Stderr, "casts -named: %s has a doc comment; no place to declare %s\n", fd.Name.Name, iname)
			os.Exit(1)
		}
		prev := rw.Rewrite[fd]
		rw.Rewrite[fd] = func() string {
			body := rw.RenderChildren(fd)
			if prev != nil {
				body = prev()
			}
			return fmt.Sprintf("// %s is implemented by %s and the classes that extend it: a cast to %s asserts to it\n// (post-flip).\ntype %s interface {\n\t%s() *%s\n}\n\n%s", iname, n, n, iname, asX, n, body)
		}
	}
	fmt.Fprintf(os.Stderr, "casts -named: helpers nothing calls removed: %d to_X, %d is_X, %d icast_I\n", removed["to_"], removed["is_"], removed["icast_"])
	fmt.Fprintf(os.Stderr, "casts -named: %d assertions to interface{ as_X() *X } of %d classes assert to X_as (%d declared)\n", sites, len(names), declared)
	if dry {
		return
	}
	fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
}

var helperCall = regexp.MustCompile(`\b((?:to|is|icast)_[A-Z]\w*)\(`)

func funcDeclOf(f *types.Func) *ast.FuncDecl {
	for _, file := range rw.Files {
		for _, d := range file.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && rw.Info.Defs[fd.Name] == f {
				return fd
			}
		}
	}
	return nil
}
