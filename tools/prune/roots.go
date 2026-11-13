package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

// roots writes dir/rt_zroots.go: a variable that references every function and method of the
// generated files, so that a reachability analysis treats all translated code as live.
func roots(dir string) {
	files, _ := filepath.Glob(filepath.Join(dir, "g_*.go"))
	fset := token.NewFileSet()
	var b strings.Builder
	b.WriteString("package closure\n\n// Temporary: roots for tools/prune.\n\nvar Roots_ = []any{\n")
	for _, f := range files {
		if strings.HasSuffix(f, "g_zstubs.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			panic(err)
		}
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Type.TypeParams != nil || fd.Name.Name == "init" || fd.Name.Name == "_" {
				continue
			}
			if fd.Recv == nil {
				fmt.Fprintf(&b, "\t%s,\n", fd.Name.Name)
				continue
			}
			t := fd.Recv.List[0].Type
			if s, ok := t.(*ast.StarExpr); ok {
				if id, ok := s.X.(*ast.Ident); ok {
					fmt.Fprintf(&b, "\t(*%s).%s,\n", id.Name, fd.Name.Name)
				}
			} else if id, ok := t.(*ast.Ident); ok {
				fmt.Fprintf(&b, "\t%s.%s,\n", id.Name, fd.Name.Name)
			}
		}
	}
	b.WriteString("}\n")
	os.WriteFile(filepath.Join(dir, "rt_zroots.go"), []byte(b.String()), 0o644)
}
