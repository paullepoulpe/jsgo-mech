// Command overrides is a post-flip step: a class that implements an overridable method gets the
// method itself, so a call through an interface reaches it directly.
//
// j2go writes an overridable method m of a class B as a dispatcher on B that finds the most derived
// object and calls its m_impl (this.self_.(interface{ m_impl(...) }).m_impl(...)); a class C
// that extends B and overrides m declares only m_impl, and C's m is B's dispatcher, promoted.
// This step gives C its own m: when the object is exactly a C (this.self_ == this), it calls C's
// m_impl directly; otherwise (a subclass reached through a *C) it dispatches as before. Go picks
// the shallowest m, so a subclass that overrides m_impl again gets its own m too. A call through
// an interface (callbacks, listeners) then makes one dynamic call and one comparison instead of
// two dynamic calls and a type assertion.
//
// usage: overrides [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

func isDispatcher(fd *ast.FuncDecl) bool {
	if fd.Body == nil || len(fd.Body.List) != 1 {
		return false
	}
	var call *ast.CallExpr
	switch st := fd.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(st.Results) == 1 {
			call, _ = st.Results[0].(*ast.CallExpr)
		}
	case *ast.ExprStmt:
		call, _ = st.X.(*ast.CallExpr)
	}
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != fd.Name.Name+"_impl" {
		return false
	}
	ta, ok := sel.X.(*ast.TypeAssertExpr)
	if !ok {
		return false
	}
	in, ok := ta.X.(*ast.SelectorExpr)
	return ok && in.Sel.Name == "self_"
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	dispatchers := map[*types.Func]*ast.FuncDecl{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv != nil && isDispatcher(fd) {
				dispatchers[rw.Info.Defs[fd.Name].(*types.Func)] = fd
			}
		}
	}
	added := map[*ast.File][]string{}
	n := 0
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || !strings.HasSuffix(fd.Name.Name, "_impl") {
				continue
			}
			recv := rw.Info.Defs[fd.Name].(*types.Func).Type().(*types.Signature).Recv().Type()
			m := strings.TrimSuffix(fd.Name.Name, "_impl")
			obj, index, _ := types.LookupFieldOrMethod(recv, true, rw.Pkg, m)
			disp, ok := obj.(*types.Func)
			if !ok || len(index) < 2 { // m must be promoted from an embedded class (depth >= 1)
				continue
			}
			dfd, ok := dispatchers[disp]
			if !ok {
				continue
			}
			// The dispatcher's signature (the one interfaces expect), with this class as receiver.
			p, ok := recv.(*types.Pointer)
			if !ok {
				continue
			}
			cname := p.Elem().(*types.Named).Obj().Name()
			params := rw.Text(dfd.Type.Params)
			var args []string
			for _, fl := range dfd.Type.Params.List {
				for _, nm := range fl.Names {
					args = append(args, nm.Name)
				}
			}
			res := ""
			if dfd.Type.Results != nil {
				res = " " + rw.Text(dfd.Type.Results)
			}
			ret := "return "
			if dfd.Type.Results == nil {
				ret = ""
			}
			// The dispatcher's own statement (this.self_.(interface{...}).m_impl(...)), for objects
			// of a subclass reached through a *C: only when the object is exactly a C is its
			// m_impl the one to call.
			fallback := rw.Text(dfd.Body.List[0])
			added[f] = append(added[f], fmt.Sprintf("\n// %s: an object that is exactly a %s calls its %s_impl directly (post-flip); others dispatch.\nfunc (this *%s) %s%s%s {\n\tif this.self_ == any(this) {\n\t\t%sthis.%s_impl(%s)\n%s\t}\n\t%s\n}\n",
				m, cname, m, cname, m, params, res, ret, m, strings.Join(args, ", "), map[bool]string{true: "\t\treturn\n", false: ""}[ret == ""], fallback))
			n++
		}
	}
	fmt.Fprintf(os.Stderr, "dispatchers: %d; methods added to the classes that implement them: %d\n", len(dispatchers), n)
	if report {
		return
	}
	var files []*ast.File
	for f := range added {
		files = append(files, f)
	}
	sort.Slice(files, func(i, j int) bool { return rw.Fset.File(files[i].Pos()).Name() < rw.Fset.File(files[j].Pos()).Name() })
	for _, f := range files {
		tf := rw.Fset.File(f.Pos())
		s := string(rw.Src[tf]) + strings.Join(added[f], "")
		if err := os.WriteFile(tf.Name(), []byte(s), 0o644); err != nil {
			panic(err)
		}
	}
	fmt.Fprintf(os.Stderr, "files changed: %d\n", len(files))
}
