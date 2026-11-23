// Command deadtfn is a post-flip step: a lambda adapter type of j2go's stubs (X_tfn_.. / X_fn,
// in g_zstubs.go and g_0lamparams.go) that no code names any more is deleted with its methods.
//
// The typed translator writes each Java lambda given to an erased functional interface as an
// adapter object &X_tfn_T{f} whose methods (apply(p0 any) any, test(p0 any) bool, ...) are
// javac's bridge from the erased interface to the typed function; the steps since the flip
// (tfnargs, lamparams, etaconv, tfnparams, tcmp, ...) pass the function itself where the
// callee is typed, and an adapter type that no composite literal, conversion, assertion or
// declaration names can have no value: its methods never run. Nothing else changes. The types
// that only dead types name go with them (to a fixed point). An adapter a runtime test names
// stays (Comparator_fn of rt_cmp_test.go). A rerun changes nothing.
//
// usage: deadtfn [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"postflip/rw"
)

func main() {
	report, verbose := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	isStub := func(f *ast.File) bool {
		b := filepath.Base(rw.Fset.File(f.Pos()).Name())
		return b == "g_zstubs.go" || b == "g_0lamparams.go"
	}
	// the adapter types and the declarations that belong to them (the type and its methods)
	type adapter struct {
		tn    *types.TypeName
		decls []ast.Decl
		spec  *ast.GenDecl
	}
	adapters := map[*types.TypeName]*adapter{}
	for _, f := range rw.Files {
		if !isStub(f) {
			continue
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE || len(gd.Specs) != 1 {
				continue
			}
			ts := gd.Specs[0].(*ast.TypeSpec)
			if _, ok := ts.Type.(*ast.StructType); !ok {
				continue
			}
			n := ts.Name.Name
			if !strings.Contains(n, "_tfn") && !strings.HasSuffix(n, "_fn") {
				continue
			}
			tn := rw.Info.Defs[ts.Name].(*types.TypeName)
			adapters[tn] = &adapter{tn: tn, decls: []ast.Decl{gd}, spec: gd}
		}
	}
	for _, f := range rw.Files {
		if !isStub(f) {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil {
				continue
			}
			rt := fd.Recv.List[0].Type
			if s, ok := rt.(*ast.StarExpr); ok {
				rt = s.X
			}
			if ix, ok := rt.(*ast.IndexExpr); ok {
				rt = ix.X
			}
			if ix, ok := rt.(*ast.IndexListExpr); ok {
				rt = ix.X
			}
			id, ok := rt.(*ast.Ident)
			if !ok {
				continue
			}
			if a := adapters[rw.Info.Uses[id].(*types.TypeName)]; a != nil {
				a.decls = append(a.decls, fd)
			}
		}
	}
	// the enclosing top-level declaration of every use of an adapter's name
	declAt := func(id *ast.Ident) ast.Decl {
		var n ast.Node = id
		for n != nil {
			if d, ok := n.(ast.Decl); ok {
				if _, isFile := rw.Parent[d].(*ast.File); isFile {
					return d
				}
			}
			n = rw.Parent[n]
		}
		return nil
	}
	owner := map[ast.Decl]*adapter{}
	for _, a := range adapters {
		for _, d := range a.decls {
			owner[d] = a
		}
	}
	// names the runtime's tests use (not loaded by rw: a test file is not part of the package
	// the steps rewrite, but a test that builds an adapter keeps it)
	testUse := map[string]bool{}
	tests, _ := filepath.Glob(filepath.Join(os.Args[len(os.Args)-1], "*_test.go"))
	for _, t := range tests {
		b, err := os.ReadFile(t)
		if err != nil {
			panic(err)
		}
		for a := range adapters {
			if regexp.MustCompile(`\b` + a.Name() + `\b`).Match(b) {
				testUse[a.Name()] = true
			}
		}
	}
	dead := map[*adapter]bool{}
	for {
		changed := false
		for _, a := range adapters {
			if dead[a] || testUse[a.tn.Name()] {
				continue
			}
			live := false
			for _, id := range rw.UsesOf[a.tn] {
				d := declAt(id)
				if o := owner[d]; o == a || (o != nil && dead[o]) {
					continue
				}
				live = true
				break
			}
			if !live {
				dead[a] = true
				changed = true
			}
		}
		if !changed {
			break
		}
	}
	var names []string
	nm := 0
	for a := range dead {
		names = append(names, a.tn.Name())
		nm += len(a.decls) - 1
		for _, d := range a.decls {
			rw.Rewrite[d] = func() string { return rw.DelDoc }
		}
	}
	sort.Strings(names)
	fmt.Fprintf(os.Stderr, "deadtfn: %d lambda adapter types no code names are deleted, with their %d methods; %d adapter types stay\n", len(dead), nm, len(adapters)-len(dead))
	if verbose {
		for _, n := range names {
			fmt.Fprintf(os.Stderr, "        %s\n", n)
		}
	}
	if report || len(dead) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "deadtfn: files changed: %d\n", rw.SaveChecked())
}
