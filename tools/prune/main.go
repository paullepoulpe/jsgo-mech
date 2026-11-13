// prune deletes unreachable functions from the handwritten runtime (go/closure/rt_*.go).
//
// Input: the output of `deadcode ./cmd/mech` (golang.org/x/tools/cmd/deadcode) on stdin.
// A plain function listed there is deleted. A method that is never called can still be what
// makes a type satisfy an interface in a type assertion, so a listed method is deleted only when
// no interface in the package declares a method of that name, or when every method of its
// receiver type is listed.
//
//	cd go && deadcode ./cmd/mech | go run ../tools/prune closure
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"
)

func recvName(fd *ast.FuncDecl) string {
	if fd.Recv == nil || len(fd.Recv.List) == 0 {
		return ""
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	if ix, ok := t.(*ast.IndexExpr); ok {
		t = ix.X
	}
	if id, ok := t.(*ast.Ident); ok {
		return id.Name
	}
	return "?"
}

func main() {
	if os.Args[1] == "roots" {
		roots(os.Args[2])
		return
	}
	if os.Args[1] == "restore" {
		restore(os.Args[2], os.Args[3], os.Args[4:])
		return
	}
	dir := os.Args[1]
	dead := map[string]bool{} // "file:line"
	re := regexp.MustCompile(`^(\S+/rt_[^:]+\.go):(\d+):\d+: unreachable func`)
	sc := bufio.NewScanner(os.Stdin)
	for sc.Scan() {
		if m := re.FindStringSubmatch(sc.Text()); m != nil {
			dead[filepath.Base(m[1])+":"+m[2]] = true
		}
	}
	files, _ := filepath.Glob(filepath.Join(dir, "rt_*.go"))
	fset := token.NewFileSet()
	type decl struct {
		file       string
		fd         *ast.FuncDecl
		start, end int
		dead       bool
	}
	var decls []*decl
	methods := map[string][]*decl{}
	src := map[string][]byte{}
	for _, f := range files {
		if strings.HasSuffix(f, "_test.go") || strings.HasSuffix(f, "rt_proto_gen.go") {
			continue
		}
		b, _ := os.ReadFile(f)
		src[f] = b
		af, err := parser.ParseFile(fset, f, b, parser.ParseComments)
		if err != nil {
			panic(err)
		}
		for _, d := range af.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			start := fd.Pos()
			if fd.Doc != nil {
				start = fd.Doc.Pos()
			}
			line := fset.Position(fd.Pos()).Line
			x := &decl{f, fd, fset.Position(start).Offset, fset.Position(fd.End()).Offset,
				dead[filepath.Base(f)+":"+strconv.Itoa(line)]}
			decls = append(decls, x)
			if r := recvName(fd); r != "" {
				methods[r] = append(methods[r], x)
			}
		}
	}
	// Method names declared by any interface type in the package (named or written inline in a
	// type assertion): only such a method can decide whether a value satisfies an interface.
	ifaceMethods := map[string]bool{}
	all, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	for _, f := range all {
		if strings.HasSuffix(f, "_test.go") {
			continue
		}
		af, err := parser.ParseFile(fset, f, nil, 0)
		if err != nil {
			panic(err)
		}
		ast.Inspect(af, func(n ast.Node) bool {
			if it, ok := n.(*ast.InterfaceType); ok && it.Methods != nil {
				for _, m := range it.Methods.List {
					for _, id := range m.Names {
						ifaceMethods[id.Name] = true
					}
				}
			}
			return true
		})
	}
	del := map[string][]*decl{}
	n := 0
	for _, x := range decls {
		if !x.dead {
			continue
		}
		if r := recvName(x.fd); r != "" {
			all := true
			for _, m := range methods[r] {
				all = all && m.dead
			}
			if !all && ifaceMethods[x.fd.Name.Name] {
				continue
			}
		}
		del[x.file] = append(del[x.file], x)
		n++
	}
	for f, ds := range del {
		sort.Slice(ds, func(i, j int) bool { return ds[i].start > ds[j].start })
		b := src[f]
		for _, x := range ds {
			end := x.end
			for end < len(b) && b[end] == '\n' {
				end++
			}
			b = append(b[:x.start:x.start], b[end:]...)
		}
		os.WriteFile(f, b, 0o644)
	}
	fmt.Println("deleted", n, "functions")
}
