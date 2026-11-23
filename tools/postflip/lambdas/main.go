// Command lambdas is a post-flip step: a lambda that captures nothing is made once.
//
// j2go writes a Java lambda or method reference as a function-interface object holding a Go
// function literal, &F_fn{func(...) {...}}, built where the lambda is evaluated. Go allocates
// that object each time. HotSpot does not: a non-capturing lambda evaluates to one instance per
// call site (its invokedynamic call site is bound to a constant). This step does the same: when
// the function literal uses no variable of the function around it (no parameter, local or
// receiver; package-level names and its own locals only), the object becomes a package-level
// variable declared after the function, and the expression names it.
//
// Lambdas in generic functions are left alone (a package-level variable cannot use a type
// parameter), and so are the ones listed in the -keep file: those whose variable would be in
// an initialization cycle (the literal calls a function that evaluates the lambda), which the
// type checker reports when the step is applied to them.
//
// usage: lambdas [-keep file] <dir of package closure>
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"strings"

	"postflip/rw"
)

func main() {
	keep := map[string]bool{}
	if len(os.Args) > 3 && os.Args[1] == "-keep" {
		f, err := os.Open(os.Args[2])
		if err != nil {
			panic(err)
		}
		sc := bufio.NewScanner(f)
		for sc.Scan() {
			if s := strings.TrimSpace(sc.Text()); s != "" {
				keep[s] = true
			}
		}
		f.Close()
	}
	rw.Load(os.Args[len(os.Args)-1])

	// fnType reports whether the composite literal's type is a j2go function-interface
	// wrapper: a named struct F_fn with one field, of function type.
	fnType := func(cl *ast.CompositeLit) (string, bool) {
		id, ok := cl.Type.(*ast.Ident)
		if !ok || !strings.HasSuffix(id.Name, "_fn") || len(cl.Elts) != 1 {
			return "", false
		}
		tn, ok := rw.Info.Uses[id].(*types.TypeName)
		if !ok || tn.Parent() != rw.Pkg.Scope() {
			return "", false
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok || st.NumFields() != 1 {
			return "", false
		}
		if _, ok := st.Field(0).Type().Underlying().(*types.Signature); !ok {
			return "", false
		}
		return id.Name, true
	}
	// captures reports whether the function literal uses a variable declared outside it that
	// is not package-level, or a type parameter.
	captures := func(fl *ast.FuncLit) bool {
		bad := false
		ast.Inspect(fl, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || bad {
				return !bad
			}
			switch o := rw.Info.Uses[id].(type) {
			case *types.Var:
				if o.IsField() || o.Parent() == rw.Pkg.Scope() {
					return true
				}
				if o.Pos() < fl.Pos() || o.Pos() >= fl.End() {
					bad = true
				}
			case *types.TypeName:
				if _, ok := o.Type().(*types.TypeParam); ok {
					bad = true
				}
			}
			return true
		})
		return bad
	}

	hoisted, kept, generic := 0, 0, 0
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			isGeneric := fd.Type.TypeParams != nil
			if fd.Recv != nil && len(fd.Recv.List) == 1 {
				t := fd.Recv.List[0].Type
				if s, ok := t.(*ast.StarExpr); ok {
					t = s.X
				}
				switch t.(type) {
				case *ast.IndexExpr, *ast.IndexListExpr:
					isGeneric = true
				}
			}
			base := fd.Name.Name
			if fd.Recv != nil && len(fd.Recv.List) == 1 {
				t := fd.Recv.List[0].Type
				if s, ok := t.(*ast.StarExpr); ok {
					t = s.X
				}
				if id, ok := t.(*ast.Ident); ok {
					base = id.Name + "_" + base
				}
			}
			var mine []pend
			k := 0
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				u, ok := n.(*ast.UnaryExpr)
				if !ok || u.Op != token.AND {
					return true
				}
				cl, ok := u.X.(*ast.CompositeLit)
				if !ok {
					return true
				}
				if _, ok := fnType(cl); !ok {
					return true
				}
				fl, ok := cl.Elts[0].(*ast.FuncLit)
				if !ok || captures(fl) {
					return true
				}
				if isGeneric {
					generic++
					return true
				}
				k++
				name := fmt.Sprintf("lambda_%s_%d_", base, k)
				if keep[name] {
					kept++
					return true
				}
				hoisted++
				rw.Rewrite[u] = func() string { return name }
				mine = append(mine, pend{name, u})
				return true
			})
			if len(mine) > 0 {
				fdd, b := fd, base
				// The declarations follow the function; they are rendered after every rewrite
				// is registered, so that a hoisted lambda inside a hoisted lambda is a name.
				rw.Rewrite[fd] = func() string {
					var out strings.Builder
					out.WriteString(rw.RenderChildren(fdd))
					for _, p := range mine {
						out.WriteString("\n\n// " + p.name + " is a lambda of " + b + " that captures nothing (post-flip step lambdas).\nvar " + p.name + " = " + rw.RenderChildren(p.u))
					}
					return out.String()
				}
			}
		}
	}
	fmt.Fprintf(os.Stderr, "lambdas hoisted: %d; kept (initialization cycle): %d; in generic functions: %d; files changed: %d\n", hoisted, kept, generic, rw.Save())
}

type pend struct {
	name string
	u    *ast.UnaryExpr
}
