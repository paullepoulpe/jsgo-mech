// Command implsplit is a post-flip step: a class's method that implements a method of a
// Java library interface erased in the runtime (Comparator.compare, Predicate.apply,
// Function.apply, Comparable.compareTo, ...) is split as javac compiles it: the method with
// the parameter types the class declares, and the bridge with the interface's erased
// signature that casts its arguments and calls it.
//
//	func (this *JSChunkGraph_InverseDepthComparator) compare(m1_ any, m2_ any) int32 {
//		var m1 *JSChunk = cast_[*JSChunk](m1_)
//		_ = m1
//		var m2 *JSChunk = cast_[*JSChunk](m2_)
//		_ = m2
//		return JSChunkGraph_depthCompare(m2, m1)
//	}
//	  ->
//	func (this *JSChunkGraph_InverseDepthComparator) compareT_(m1 *JSChunk, m2 *JSChunk) int32 {
//		return JSChunkGraph_depthCompare(m2, m1)
//	}
//	func (this *JSChunkGraph_InverseDepthComparator) compare(m1_ any, m2_ any) int32 {
//		return this.compareT_(cast_[*JSChunk](m1_), cast_[*JSChunk](m2_))
//	}
//
// Java's `class InverseDepthComparator implements Comparator<JSChunk>` declares
// compare(JSChunk, JSChunk); javac adds the bridge compare(Object, Object) that checkcasts its
// arguments, because the interface's method is erased. j2go writes the one method with the
// erased signature and the casts as its first statements (`var x T = K(x_)`, `_ = x`). Here the
// method keeps the erased signature as the bridge (the runtime's sorts, sets and predicates
// call it through the erased interface), and the body is the typed method <name>T_, the casts
// done by the bridge in the same order, before anything else, as before.
//
// Callers that hold the class itself call the typed method: x.m(B(e), ...) with each B the
// boxing its parameter's cast undoes (strobj_ / str_, e.box_() / Node_of_, obj_ / cast_[T] of a
// pointer) is x.mT_(e, ...), and unboxedPred_[T](x.m) / unboxedCons_[T](x.m) (tfnargs: the
// erased method boxing the argument it is given) is the method value x.mT_. A method an
// override of which in a subclass exists, a class that is a lambda adapter (its function is
// typed already), and a method whose first statements are not its parameters' casts are left,
// counted. A rerun changes nothing.
//
// usage: implsplit [-n] [-v] <dir of package closure>
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

var qual = func(p *types.Package) string {
	if p == rw.Pkg {
		return ""
	}
	return p.Name()
}

// the methods of the Java library interfaces j2go's stubs declare erased
var ifaceMethods = map[string]bool{"compare": true, "compareTo": true, "apply": true, "test": true, "accept": true, "applyAsInt": true}

type split struct {
	fd     *ast.FuncDecl
	fo     *types.Func
	class  *types.Named
	casts  []string     // K of each parameter
	names  []string     // the typed parameter names
	types  []types.Type // their types
	nstmts int          // the cast statements to drop
}

var (
	verbose bool
	counts  = map[string]int{}
	lefts   = map[string][]string{}
)

func left(why string, n ast.Node) {
	lefts[why] = append(lefts[why], rw.Fset.Position(n.Pos()).String())
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	// classes embedding another (for overrides)
	embedders := map[*types.Named][]*types.Named{}
	for _, n := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		nt, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		if st, ok := nt.Underlying().(*types.Struct); ok {
			for i := 0; i < st.NumFields(); i++ {
				if f := st.Field(i); f.Embedded() {
					if e, ok := types.Unalias(f.Type()).(*types.Named); ok {
						embedders[e] = append(embedders[e], nt)
					}
				}
			}
		}
	}
	splits := map[*types.Func]*split{}
	var order []*split
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Body == nil || !ifaceMethods[fd.Name.Name] {
				continue
			}
			fo := rw.Info.Defs[fd.Name].(*types.Func)
			sig := fo.Type().(*types.Signature)
			cls := ptrNamed(sig.Recv().Type())
			if cls == nil || cls.TypeParams().Len() > 0 || sig.Params().Len() == 0 {
				continue
			}
			allAny := true
			for i := 0; i < sig.Params().Len(); i++ {
				if !isAny(sig.Params().At(i).Type()) {
					allAny = false
				}
			}
			if !allAny {
				continue
			}
			name := cls.Obj().Name()
			if strings.Contains(name, "_tfn") || strings.HasSuffix(name, "_fn") {
				continue // a lambda adapter: its function is typed already
			}
			s := &split{fd: fd, fo: fo, class: cls}
			if why := s.casts_(); why != "" {
				left(why, fd)
				continue
			}
			if len(embedders[cls]) > 0 {
				left("a class with subclasses (an override may exist)", fd)
				continue
			}
			if rw.HasMethod(types.NewPointer(cls), fd.Name.Name+"T_") {
				continue // a rerun
			}
			splits[fo] = s
			order = append(order, s)
		}
	}
	// the callers on the class
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			sel, ok := n.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			s := rw.Info.Selections[sel]
			if s == nil || s.Kind() != types.MethodVal {
				return true
			}
			sp := splits[s.Obj().(*types.Func)]
			if sp == nil {
				return true
			}
			if c, ok := rw.Parent[sel].(*ast.CallExpr); ok && c.Fun == sel {
				// x.m(B(e), ...)
				var es []ast.Expr
				for i, a := range c.Args {
					e := unbox(sp.casts[i], a)
					if e == nil || !types.Identical(rw.Info.TypeOf(e), sp.types[i]) {
						left("a call on the class with an argument that is not a box of the parameter's type", c)
						return true
					}
					es = append(es, e)
				}
				counts["calls on the class with boxed arguments call the typed method"]++
				rw.Rewrite[c] = func() string {
					var as []string
					for _, e := range es {
						as = append(as, rw.Render(e))
					}
					return rw.Render(sel.X) + "." + sp.fd.Name.Name + "T_(" + strings.Join(as, ", ") + ")"
				}
				return true
			}
			// unboxedPred_[T](x.m), unboxedCons_[T](x.m)
			if c, ok := rw.Up(sel).(*ast.CallExpr); ok && len(c.Args) == 1 && rw.Unparen(c.Args[0]) == sel {
				name, _ := rw.CallName(c)
				if (strings.HasPrefix(name, "unboxedPred_[") || strings.HasPrefix(name, "unboxedCons_[")) && len(sp.casts) == 1 && sp.casts[0] == "Node_of_" {
					counts["unboxedPred_ / unboxedCons_ of the method value are the typed method value"]++
					rw.Rewrite[c] = func() string { return rw.Render(sel.X) + "." + sp.fd.Name.Name + "T_" }
					return true
				}
			}
			return true
		})
	}
	sort.Slice(order, func(i, j int) bool { return order[i].fd.Pos() < order[j].fd.Pos() })
	for _, s := range order {
		s := s
		counts["methods split into the typed method and the bridge"]++
		rw.Rewrite[s.fd] = func() string { return s.text() }
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", counts[k], k)
	}
	if verbose {
		for _, s := range order {
			fmt.Fprintf(os.Stderr, "        %s.%s\n", s.class.Obj().Name(), s.fd.Name.Name)
		}
	}
	keys = keys[:0]
	for k := range lefts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", len(lefts[k]), k)
		if verbose {
			for _, p := range lefts[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	if report || len(order) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "implsplit: files changed: %d\n", rw.SaveChecked())
}

func ptrNamed(t types.Type) *types.Named {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		if n, ok := types.Unalias(p.Elem()).(*types.Named); ok {
			return n
		}
	}
	return nil
}

func isAny(t types.Type) bool {
	it, ok := types.Unalias(t).Underlying().(*types.Interface)
	return ok && it.Empty()
}

// casts_: the first statements are `var x T = K(x_)` and `_ = x` for every parameter x_, in order.
func (s *split) casts_() string {
	var params []*ast.Ident
	for _, f := range s.fd.Type.Params.List {
		params = append(params, f.Names...)
	}
	st := s.fd.Body.List
	if len(st) < 2*len(params)+1 {
		return "the method does not start with its parameters' casts"
	}
	for i, p := range params {
		ds, ok := st[2*i].(*ast.DeclStmt)
		if !ok {
			return "the method does not start with its parameters' casts"
		}
		gd := ds.Decl.(*ast.GenDecl)
		if len(gd.Specs) != 1 {
			return "the method does not start with its parameters' casts"
		}
		vs := gd.Specs[0].(*ast.ValueSpec)
		if len(vs.Names) != 1 || len(vs.Values) != 1 || vs.Type == nil {
			return "the method does not start with its parameters' casts"
		}
		c, ok := vs.Values[0].(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			return "the method does not start with its parameters' casts"
		}
		if id, ok := c.Args[0].(*ast.Ident); !ok || rw.Info.Uses[id] != rw.Info.Defs[p] {
			return "the method does not start with its parameters' casts"
		}
		k, _ := rw.CallName(c)
		if !(k == "Node_of_" || k == "str_" || strings.HasPrefix(k, "cast_[") || strings.HasPrefix(k, "to_") || strings.HasPrefix(k, "icast_")) {
			return "a parameter's conversion is not a cast"
		}
		as, ok := st[2*i+1].(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 || rw.Text(as.Lhs[0]) != "_" || rw.Text(as.Rhs[0]) != vs.Names[0].Name {
			return "the method does not start with its parameters' casts"
		}
		// the erased parameter is used only by its cast
		for _, u := range rw.UsesOf[rw.Info.Defs[p]] {
			if u != c.Args[0] {
				return "the erased parameter is used besides its cast"
			}
		}
		s.casts = append(s.casts, rw.Text(c.Fun))
		s.names = append(s.names, vs.Names[0].Name)
		s.types = append(s.types, rw.Info.TypeOf(vs.Type))
	}
	s.nstmts = 2 * len(params)
	return ""
}

// unbox: the e of a B(e) the cast K undoes.
func unbox(k string, a ast.Expr) ast.Expr {
	c, ok := rw.Unparen(a).(*ast.CallExpr)
	if !ok {
		return nil
	}
	name, _ := rw.CallName(c)
	switch {
	case k == "str_" && name == "strobj_" && len(c.Args) == 1:
		return c.Args[0]
	case k == "Node_of_" && len(c.Args) == 0:
		if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "box_" {
			return s.X
		}
	case strings.HasPrefix(k, "cast_[") && (name == "obj_" || strings.HasPrefix(name, "obj_[")) && len(c.Args) == 1:
		if _, ok := rw.Info.TypeOf(c.Args[0]).(*types.Pointer); ok {
			return c.Args[0]
		}
	}
	return nil
}

func (s *split) text() string {
	fd := s.fd
	recv := rw.Text(fd.Recv)
	res := ""
	if fd.Type.Results != nil {
		res = " " + rw.Text(fd.Type.Results)
	}
	var ps, args []string
	var eparams []string
	i := 0
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			ps = append(ps, s.names[i]+" "+types.TypeString(s.types[i], qual))
			args = append(args, s.casts[i]+"("+n.Name+")")
			eparams = append(eparams, n.Name+" any")
			i++
		}
	}
	// the body without the casts (their lines dropped; comments kept)
	for _, st := range fd.Body.List[:s.nstmts] {
		rw.Rewrite[st] = func() string { return rw.Del }
	}
	body := rw.Render(fd.Body)
	ret := "return "
	if fd.Type.Results == nil {
		ret = ""
	}
	return fmt.Sprintf("// %sT_ is %s with the parameter types the class declares (implsplit).\nfunc %s %sT_(%s)%s %s\n\n"+
		"// %s is the bridge of the interface's erased signature (javac's): it casts its arguments and calls %sT_.\nfunc %s %s(%s)%s {\n\t%sthis.%sT_(%s)\n}",
		fd.Name.Name, fd.Name.Name, recv, fd.Name.Name, strings.Join(ps, ", "), res, body,
		fd.Name.Name, fd.Name.Name, recv, fd.Name.Name, strings.Join(eparams, ", "), res, ret, fd.Name.Name, strings.Join(args, ", "))
}
