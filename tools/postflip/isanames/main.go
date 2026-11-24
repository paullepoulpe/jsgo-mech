// Command isanames is a post-flip step (fifth flip): the marker interface of an instanceof test
// of a generic class gets a name.
//
//	_, ok := v.(interface{ isa_X_() })      ->  _, ok := v.(X_isa)
//	type X_isa interface{ isa_X_() }            (declared after the marker method of X)
//
// The translator writes `v instanceof X` of a generic class X (any type arguments: Java tests
// the erased class) as an assertion to the anonymous interface of X's marker method
// func (this *X[P...]) isa_X_() {}, which X and the classes that embed it have. The named
// interface has the same method set, so every assertion, type switch case and helper is_X
// gives the same result. X_isa is declared once, where the marker method is, when the name is
// free in the package.
//
// A generated helper func is_X(v any) bool { _, ok := v.(X_isa); return ok } that no code
// calls is deleted (tswitch and instof write its callers as assertions and cases).
//
// A rerun changes nothing.
//
// usage: isanames [-n] <dir of package closure>
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

// marker returns the method name when t is interface{ isa_X_() }.
func marker(t ast.Expr) string {
	it, ok := t.(*ast.InterfaceType)
	if !ok || it.Methods == nil || len(it.Methods.List) != 1 {
		return ""
	}
	f := it.Methods.List[0]
	ft, ok := f.Type.(*ast.FuncType)
	if !ok || len(f.Names) != 1 || ft.Params.NumFields() != 0 || ft.Results.NumFields() != 0 || ft.TypeParams != nil {
		return ""
	}
	if n := f.Names[0].Name; strings.HasPrefix(n, "isa_") && strings.HasSuffix(n, "_") && len(n) > 5 {
		return n
	}
	return ""
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
	}
	rw.Load(os.Args[len(os.Args)-1])
	gen := func(n ast.Node) bool {
		return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "g_")
	}
	// the marker methods, and the names already declared for them
	methods := map[string]*ast.FuncDecl{}
	declared := map[string]bool{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.FuncDecl:
				if gen(x) && x.Recv != nil && x.Body != nil && len(x.Body.List) == 0 && x.Type.Params.NumFields() == 0 && x.Type.Results.NumFields() == 0 &&
					strings.HasPrefix(x.Name.Name, "isa_") && strings.HasSuffix(x.Name.Name, "_") {
					methods[x.Name.Name] = x
				}
			case *ast.GenDecl:
				if x.Tok != token.TYPE {
					continue
				}
				for _, s := range x.Specs {
					ts := s.(*ast.TypeSpec)
					if m := marker(ts.Type); m != "" && ts.TypeParams == nil && ts.Name.Name == strings.TrimSuffix(strings.TrimPrefix(m, "isa_"), "_")+"_isa" {
						declared[m] = true
					}
				}
			}
		}
	}
	named, taken := map[string]int{}, map[string]int{}
	for _, f := range rw.Files {
		if !gen(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			it, ok := n.(*ast.InterfaceType)
			if !ok {
				return true
			}
			m := marker(it)
			if m == "" || methods[m] == nil {
				return true
			}
			if ts, ok := rw.Parent[it].(*ast.TypeSpec); ok && ts.Type == ast.Expr(it) {
				return true // a declaration
			}
			name := strings.TrimSuffix(strings.TrimPrefix(m, "isa_"), "_") + "_isa"
			if !declared[m] && rw.Pkg.Scope().Lookup(name) != nil {
				taken[name]++
				return true
			}
			named[m]++
			rw.Rewrite[it] = func() string { return name }
			return true
		})
	}
	var ms []string
	for m := range named {
		ms = append(ms, m)
	}
	sort.Strings(ms)
	ndecl := 0
	for _, m := range ms {
		if declared[m] {
			continue
		}
		ndecl++
		fd := methods[m]
		x := strings.TrimSuffix(strings.TrimPrefix(m, "isa_"), "_")
		rw.Rewrite[fd] = func() string {
			return rw.RenderChildren(fd) + "\n\n// " + x + "_isa is implemented by " + x + ", at any type arguments, and by the classes that extend it:\n// `instanceof " + x + "` asserts to it (post-flip).\ntype " + x + "_isa interface{ " + m + "() }"
		}
	}
	// helpers is_X of a marker interface that nothing calls
	ndead := 0
	for _, f := range rw.Files {
		if !gen(f) {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Body == nil || !strings.HasPrefix(fd.Name.Name, "is_") || len(fd.Body.List) != 2 || fd.Type.Params.NumFields() != 1 {
				continue
			}
			as, ok := fd.Body.List[0].(*ast.AssignStmt)
			if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 2 || len(as.Rhs) != 1 {
				continue
			}
			ta, ok := as.Rhs[0].(*ast.TypeAssertExpr)
			if !ok || ta.Type == nil {
				continue
			}
			rs, ok := fd.Body.List[1].(*ast.ReturnStmt)
			if !ok || len(rs.Results) != 1 || rw.Text(rs.Results[0]) != rw.Text(as.Lhs[1]) {
				continue
			}
			m := marker(ta.Type)
			if m == "" {
				if tv, ok := rw.Info.Types[ta.Type]; ok {
					if nt, ok := types.Unalias(tv.Type).(*types.Named); ok && strings.HasSuffix(nt.Obj().Name(), "_isa") {
						if it, ok := nt.Underlying().(*types.Interface); ok && it.NumMethods() == 1 && it.Method(0).Name() == "isa_"+strings.TrimSuffix(nt.Obj().Name(), "_isa")+"_" {
							m = it.Method(0).Name()
						}
					}
				}
			}
			if m == "" || methods[m] == nil || len(rw.UsesOf[rw.Info.Defs[fd.Name]]) != 0 {
				continue
			}
			ndead++
			rw.Rewrite[fd] = func() string { return rw.DelDoc }
		}
	}
	total := 0
	for _, n := range named {
		total += n
	}
	var tk []string
	for n, c := range taken {
		tk = append(tk, fmt.Sprintf("%s (%d)", n, c))
	}
	sort.Strings(tk)
	fmt.Fprintf(os.Stderr, "isanames: %d anonymous marker interfaces named (%d classes, %d X_isa declared); %d uncalled is_X helpers of a marker deleted; left: %d names taken %v\n", total, len(named), ndecl, ndead, len(taken), tk)
	if report || total+ndead == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "isanames: files changed: %d\n", rw.SaveChecked())
}
