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

// restore appends to dir/<file> the declarations of the named functions ("F" or "Type.method")
// found in olddir/<file>, unless dir already declares them.
func restore(olddir, dir string, names []string) {
	want := map[string]bool{}
	for _, n := range names {
		want[n] = true
	}
	have := map[string]bool{}
	key := func(fd *ast.FuncDecl) string {
		if r := recvName(fd); r != "" {
			return r + "." + fd.Name.Name
		}
		return fd.Name.Name
	}
	cur, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	fset := token.NewFileSet()
	for _, f := range cur {
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			panic(err)
		}
		for _, d := range af.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && !strings.Contains(f, "g_zstubs") {
				have[key(fd)] = true
			}
		}
	}
	old, _ := filepath.Glob(filepath.Join(olddir, "rt_*.go"))
	n := 0
	for _, f := range old {
		b, _ := os.ReadFile(f)
		af, err := parser.ParseFile(fset, f, b, parser.ParseComments)
		if err != nil {
			panic(err)
		}
		var add []string
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || !want[key(fd)] || have[key(fd)] {
				continue
			}
			start := fd.Pos()
			if fd.Doc != nil {
				start = fd.Doc.Pos()
			}
			add = append(add, string(b[fset.Position(start).Offset:fset.Position(fd.End()).Offset]))
			n++
		}
		if len(add) == 0 {
			continue
		}
		target := filepath.Join(dir, filepath.Base(f))
		out, err := os.OpenFile(target, os.O_APPEND|os.O_WRONLY, 0o644)
		if err != nil {
			fmt.Println("no file", target, "for", len(add), "functions")
			n -= len(add)
			continue
		}
		for _, a := range add {
			out.WriteString("\n" + a + "\n")
		}
		out.Close()
	}
	fmt.Println("restored", n, "functions")
}
