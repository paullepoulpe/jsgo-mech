// Command ctx (JAVAISMS6, read-only): for each line "file TAB offset ..." of a TSV on stdin, prints the
// line followed by TAB, the enclosing function, TAB, the names of up to 5 enclosing calls (innermost
// first, "callee#argindex", Go conversions as "conv:T"), TAB, the static type of the innermost
// expression at the offset.
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"postflip/rw"
)

func main() {
	rw.Load(os.Args[1])
	files := map[string]*ast.File{}
	for _, f := range rw.Files {
		files[filepath.Base(rw.Fset.Position(f.Pos()).Filename)] = f
	}
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	q := func(t types.Type) string {
		return types.TypeString(t, func(*types.Package) string { return "" })
	}
	for sc.Scan() {
		l := sc.Text()
		p := strings.Split(l, "\t")
		fname, ok1 := files[p[0]]
		off, err := strconv.Atoi(p[1])
		if !ok1 || err != nil {
			fmt.Fprintf(w, "%s\t?\t?\t?\n", l)
			continue
		}
		tf := rw.Fset.File(fname.Pos())
		pos := tf.Pos(off)
		var path []ast.Node
		ast.Inspect(fname, func(n ast.Node) bool {
			if n == nil {
				return false
			}
			if n.Pos() <= pos && pos < n.End() {
				path = append(path, n)
				return true
			}
			return false
		})
		fn := "-"
		var calls []string
		typ := "-"
		for i := len(path) - 1; i >= 0; i-- {
			n := path[i]
			if e, ok := n.(ast.Expr); ok && typ == "-" {
				if t := rw.Info.TypeOf(e); t != nil {
					typ = q(t)
				}
			}
			switch n := n.(type) {
			case *ast.FuncDecl:
				if fn == "-" {
					fn = n.Name.Name
					if n.Recv != nil && len(n.Recv.List) > 0 {
						fn = q(rw.Info.TypeOf(n.Recv.List[0].Type)) + "." + fn
					}
				}
			case *ast.CallExpr:
				if len(calls) >= 5 {
					continue
				}
				idx := -1
				if i+1 < len(path) {
					for k, a := range n.Args {
						if a == path[i+1] {
							idx = k
						}
					}
				}
				if idx < 0 {
					continue // the offset is in the callee expression, not an argument
				}
				name := ""
				if tv, ok := rw.Info.Types[n.Fun]; ok && tv.IsType() {
					name = "conv:" + q(tv.Type)
				} else {
					name, _ = rw.CallName(n)
					if name == "" {
						name = "?"
					}
					if s, ok := ast.Unparen(n.Fun).(*ast.SelectorExpr); ok {
						if t := rw.Info.TypeOf(s.X); t != nil {
							name = q(t) + "." + s.Sel.Name
						}
					}
				}
				calls = append(calls, fmt.Sprintf("%s#%d", name, idx))
			}
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", l, fn, strings.Join(calls, " < "), typ)
	}
}
