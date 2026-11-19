// Command fniface is a post-flip step: a function interface that only its lambda adapter
// implements becomes a Go function type.
//
// j2go writes a Java functional interface I as a Go interface with one method m, and a lambda
// of that type as an adapter object &I_fn{func...} whose method m calls the function. Where no
// class implements I (every value of the type is a lambda or nil), the interface says nothing
// a function type does not:
//
//	type I[K, V] interface { m(K, V, V) V }      ->  type I[K, V] func(K, V, V) V
//	&I_fn[K, V]{func(k K, a V, b V) V {...}}     ->  I[K, V](func(k K, a V, b V) V {...})
//	x.m(k, a, b)                                 ->  x(k, a, b)
//
// and the adapter type and its method are deleted. A call no longer goes through an interface
// and a lambda that does not escape needs no object. A Go function value cannot be compared or
// hashed, so an interface is converted only when every use of a value of the type is one of:
// calling m; being assigned to, passed to, returned as or stored in a slot declared with the
// type; a comparison with nil. Any other use (a conversion to another interface or to any,
// which is where equals, hashCode and maps of the Java object would come in; a method value; a
// type assertion to the type) leaves the interface as it is, with the reason. Not converted
// either: an interface some other type has the method for (the same name, and a signature
// that is the interface method's for some choice of the interface's type parameters),
// one with more than one method or with embedded interfaces, and one declared outside the
// generated files. The package is type-checked after the rewrite (a function type is not
// comparable: a use as a map key or under a comparable constraint fails there).
//
// usage: fniface [-n] [-only I,J,...] <dir of package closure>
package main

import (
	"flag"
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

type cand struct {
	name    string
	obj     *types.TypeName // I
	spec    *ast.TypeSpec
	method  *ast.Field
	adapter *types.TypeName // I_fn
	aDecl   *ast.GenDecl
	aMethod *ast.FuncDecl
	why     string
	edits   []func()
	lambdas int
	calls   int
}

func generated(p token.Pos) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(p).Filename), "g_")
}

func originOf(t types.Type) *types.TypeName {
	if n, ok := t.(*types.Named); ok {
		return n.Origin().Obj()
	}
	return nil
}

var cands = map[*types.TypeName]*cand{}     // by I
var byAdapter = map[*types.TypeName]*cand{} // by I_fn

func (c *cand) fail(n ast.Node, why string) {
	if c.why == "" {
		c.why = why + " (" + filepath.Base(rw.Fset.Position(n.Pos()).Filename) + ":" + fmt.Sprint(rw.Fset.Position(n.Pos()).Line) + ")"
	}
}

func main() {
	dry := flag.Bool("n", false, "report, change nothing")
	only := flag.String("only", "", "comma-separated interface names (default: every candidate)")
	flag.Parse()
	rw.Load(flag.Arg(0))
	want := map[string]bool{}
	for _, s := range strings.Split(*only, ",") {
		if s != "" {
			want[s] = true
		}
	}
	// Candidates: type I interface { m(...) } in a generated file, with type I_fn struct { f func }.
	methodsNamed := map[string][]*ast.FuncDecl{}
	typeDecls := map[string]*ast.GenDecl{}
	typeSpecs := map[string]*ast.TypeSpec{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.FuncDecl:
				if x.Recv != nil {
					methodsNamed[x.Name.Name] = append(methodsNamed[x.Name.Name], x)
				}
			case *ast.GenDecl:
				if x.Tok == token.TYPE && len(x.Specs) == 1 {
					ts := x.Specs[0].(*ast.TypeSpec)
					typeDecls[ts.Name.Name], typeSpecs[ts.Name.Name] = x, ts
				}
			}
		}
	}
	recvName := func(fd *ast.FuncDecl) string {
		t := fd.Recv.List[0].Type
		if s, ok := t.(*ast.StarExpr); ok {
			t = s.X
		}
		switch x := t.(type) {
		case *ast.IndexExpr:
			t = x.X
		case *ast.IndexListExpr:
			t = x.X
		}
		if id, ok := t.(*ast.Ident); ok {
			return id.Name
		}
		return ""
	}
	for name, ts := range typeSpecs {
		it, ok := ts.Type.(*ast.InterfaceType)
		if !ok || !generated(ts.Pos()) || len(it.Methods.List) != 1 || len(it.Methods.List[0].Names) != 1 {
			continue
		}
		ats, ok := typeSpecs[name+"_fn"]
		if !ok || !generated(ats.Pos()) {
			continue
		}
		if len(want) > 0 && !want[name] {
			continue
		}
		c := &cand{name: name, spec: ts, method: it.Methods.List[0], aDecl: typeDecls[name+"_fn"]}
		c.obj, _ = rw.Info.Defs[ts.Name].(*types.TypeName)
		c.adapter, _ = rw.Info.Defs[ats.Name].(*types.TypeName)
		if c.obj == nil || c.adapter == nil {
			continue
		}
		cands[c.obj], byAdapter[c.adapter] = c, c
		m := c.method.Names[0].Name
		for _, fd := range methodsNamed[m] {
			if recvName(fd) == name+"_fn" {
				c.aMethod = fd
			} else if fo, ok := rw.Info.Defs[fd.Name].(*types.Func); ok && mayBe(c.obj, fo) {
				c.fail(fd, "another type has the method: "+recvName(fd))
			}
		}
		if c.aMethod == nil {
			c.fail(ts, "the adapter has no method "+m)
		}
		// For an interface without type parameters the type checker says who implements it.
		if nt, ok := c.obj.Type().(*types.Named); ok && nt.TypeParams().Len() == 0 {
			it := nt.Underlying().(*types.Interface)
			sc := rw.Pkg.Scope()
			for _, on := range sc.Names() {
				tn, ok := sc.Lookup(on).(*types.TypeName)
				if !ok || tn == c.adapter || tn == c.obj || tn.IsAlias() {
					continue
				}
				if _, isIface := tn.Type().Underlying().(*types.Interface); isIface {
					continue
				}
				if on2, ok := tn.Type().(*types.Named); ok && on2.TypeParams().Len() > 0 {
					continue // a generic type: left to the signature test above
				}
				if types.Implements(tn.Type(), it) || types.Implements(types.NewPointer(tn.Type()), it) {
					c.fail(ts, "implemented by "+on)
					break
				}
			}
		}
		// Another interface with such a method could receive the value.
		for other, ots := range typeSpecs {
			oit, ok := ots.Type.(*ast.InterfaceType)
			if !ok || other == name {
				continue
			}
			for _, fl := range oit.Methods.List {
				if len(fl.Names) == 0 {
					if rw.Text(fl.Type) == name || strings.HasPrefix(rw.Text(fl.Type), name+"[") {
						c.fail(ots, "embedded in interface "+other)
					}
					continue
				}
				if fo, ok := rw.Info.Defs[fl.Names[0]].(*types.Func); ok && fl.Names[0].Name == m && mayBe(c.obj, fo) {
					c.fail(ots, "interface "+other+" has the method")
				}
			}
		}
	}
	// destIs: the slot a value in this position goes into is declared with type I.
	isI := func(t types.Type, c *cand) bool { return t != nil && originOf(t) == c.obj }
	var destOK func(e ast.Expr, c *cand) (bool, string)
	destOK = func(e ast.Expr, c *cand) (bool, string) {
		p := rw.Up(e)
		switch x := p.(type) {
		case *ast.CallExpr:
			for i, a := range x.Args {
				if rw.Unparen(a) != rw.Unparen(e) {
					continue
				}
				tv := rw.Info.Types[x.Fun]
				if tv.IsType() {
					return isI(tv.Type, c), "converted to " + rw.Text(x.Fun)
				}
				sig, ok := tv.Type.Underlying().(*types.Signature)
				if !ok || sig.Variadic() && i >= sig.Params().Len()-1 || i >= sig.Params().Len() {
					return false, "passed to " + rw.Text(x.Fun)
				}
				return isI(sig.Params().At(i).Type(), c), "passed to " + rw.Text(x.Fun)
			}
			return false, "used in a call"
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				if rw.Unparen(l) == rw.Unparen(e) {
					return true, ""
				}
			}
			if len(x.Lhs) != len(x.Rhs) {
				return false, "assigned from a call with several results"
			}
			for i, r := range x.Rhs {
				if rw.Unparen(r) == rw.Unparen(e) {
					var t types.Type
					if id, ok := x.Lhs[i].(*ast.Ident); ok && rw.Info.Defs[id] != nil {
						t = rw.Info.Defs[id].Type()
					} else {
						t = rw.Info.Types[x.Lhs[i]].Type
					}
					return isI(t, c), "assigned to " + rw.Text(x.Lhs[i])
				}
			}
		case *ast.ValueSpec:
			if x.Type == nil {
				return true, ""
			}
			return isI(rw.Info.Types[x.Type].Type, c), "initializes a variable of type " + rw.Text(x.Type)
		case *ast.ReturnStmt:
			var sig *types.Signature
			for n := ast.Node(x); n != nil; n = rw.Parent[n] {
				if fd, ok := n.(*ast.FuncDecl); ok {
					sig = rw.Info.Defs[fd.Name].Type().(*types.Signature)
					break
				}
				if fl, ok := n.(*ast.FuncLit); ok {
					sig, _ = rw.Info.Types[fl].Type.(*types.Signature)
					break
				}
			}
			for i, r := range x.Results {
				if rw.Unparen(r) == rw.Unparen(e) && sig != nil && len(x.Results) == sig.Results().Len() {
					return isI(sig.Results().At(i).Type(), c), "returned as another type"
				}
			}
			return false, "returned"
		case *ast.BinaryExpr:
			o := x.X
			if rw.Unparen(x.X) == rw.Unparen(e) {
				o = x.Y
			}
			if (x.Op == token.EQL || x.Op == token.NEQ) && rw.Text(o) == "nil" {
				return true, ""
			}
			return false, "compared with " + rw.Text(o)
		case *ast.KeyValueExpr:
			if cl, ok := rw.Parent[x].(*ast.CompositeLit); ok && x.Value == e {
				if st, ok := rw.Info.Types[cl].Type.Underlying().(*types.Struct); ok {
					if id, ok := x.Key.(*ast.Ident); ok {
						for i := 0; i < st.NumFields(); i++ {
							if st.Field(i).Name() == id.Name {
								return isI(st.Field(i).Type(), c), "stored in field " + id.Name
							}
						}
					}
				}
			}
			return false, "in a composite literal"
		case *ast.ExprStmt:
			return false, "an expression statement"
		}
		return false, fmt.Sprintf("used in %T", p)
	}
	for _, f := range rw.Files {
		gen := generated(f.Pos())
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.TypeAssertExpr:
				if x.Type != nil {
					if c := cands[originOf(rw.Info.Types[x.Type].Type)]; c != nil {
						c.fail(x, "a type assertion to the interface")
					}
				}
			case *ast.Ident:
				// The adapter's name outside its declaration, its method and a lambda literal.
				if tn, ok := rw.Info.Uses[x].(*types.TypeName); ok {
					if c := byAdapter[tn]; c != nil {
						in := false
						for p := rw.Parent[x]; p != nil; p = rw.Parent[p] {
							if p == ast.Node(c.aMethod) || p == ast.Node(c.aDecl) {
								in = true
							}
							if cl, ok := p.(*ast.CompositeLit); ok {
								if u, ok := rw.Parent[cl].(*ast.UnaryExpr); ok && u.Op == token.AND && len(cl.Elts) == 1 {
									t := cl.Type
									switch y := t.(type) {
									case *ast.IndexExpr:
										t = y.X
									case *ast.IndexListExpr:
										t = y.X
									}
									if t == ast.Expr(x) {
										in = true
									}
								}
								break
							}
						}
						if !in {
							c.fail(x, "the adapter type is named outside a lambda literal")
						}
					}
				}
			case *ast.UnaryExpr:
				// &I_fn[..]{F}
				cl, ok := x.X.(*ast.CompositeLit)
				if !ok || x.Op != token.AND || len(cl.Elts) != 1 {
					return true
				}
				pt, ok := rw.Info.Types[x].Type.(*types.Pointer)
				if !ok {
					return true
				}
				c := byAdapter[originOf(pt.Elem())]
				if c == nil {
					return true
				}
				if !gen {
					c.fail(x, "a lambda literal outside the generated files")
					return true
				}
				if ok, why := destOK(x, c); !ok {
					c.fail(x, "a lambda "+why)
					return true
				}
				targs := ""
				switch y := cl.Type.(type) {
				case *ast.IndexExpr:
					targs = "[" + rw.Text(y.Index) + "]"
				case *ast.IndexListExpr:
					var as []string
					for _, a := range y.Indices {
						as = append(as, rw.Text(a))
					}
					targs = "[" + strings.Join(as, ", ") + "]"
				}
				fn := cl.Elts[0]
				if kv, ok := fn.(*ast.KeyValueExpr); ok {
					fn = kv.Value
				}
				lit, name := x, c.name
				c.lambdas++
				c.edits = append(c.edits, func() {
					rw.Rewrite[lit] = func() string { return name + targs + "(" + rw.Render(fn) + ")" }
				})
			}
			// A value of type I.
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			tv, ok := rw.Info.Types[e]
			if !ok || !tv.IsValue() {
				return true
			}
			c := cands[originOf(tv.Type)]
			if c == nil {
				return true
			}
			if _, ok := e.(*ast.ParenExpr); ok {
				return true
			}
			if tv.IsNil() {
				return true
			}
			if !gen {
				c.fail(e, "a value of the type outside the generated files")
				return true
			}
			if sel, ok := rw.Up(e).(*ast.SelectorExpr); ok && rw.Unparen(sel.X) == e {
				call, isCall := rw.Parent[sel].(*ast.CallExpr)
				if sel.Sel.Name != c.method.Names[0].Name || !isCall || call.Fun != ast.Expr(sel) {
					c.fail(sel, "selector "+sel.Sel.Name+" not called")
					return true
				}
				s, x := sel, e
				c.calls++
				c.edits = append(c.edits, func() { rw.Rewrite[s] = func() string { return rw.Render(x) } })
				return true
			}
			if ok, why := destOK(e, c); !ok {
				c.fail(e, "a value "+why)
			}
			return true
		})
	}
	var names []string
	for _, c := range cands {
		names = append(names, c.name)
	}
	sort.Strings(names)
	done, lambdas, calls := 0, 0, 0
	for _, n := range names {
		var c *cand
		for _, x := range cands {
			if x.name == n {
				c = x
			}
		}
		if c.why != "" {
			fmt.Fprintf(os.Stderr, "  left: %s: %s\n", c.name, c.why)
			continue
		}
		fmt.Fprintf(os.Stderr, "  %s: a function type (%d lambdas, %d calls)\n", c.name, c.lambdas, c.calls)
		done++
		lambdas += c.lambdas
		calls += c.calls
		for _, e := range c.edits {
			e()
		}
		ft := c.method.Type.(*ast.FuncType)
		text := "func" + rw.Text(ft.Params)
		if ft.Results != nil {
			text += " " + rw.Text(ft.Results)
		}
		it := c.spec.Type
		rw.Rewrite[it] = func() string { return text }
		ad, am := c.aDecl, c.aMethod
		rw.Rewrite[ad] = func() string { return rw.Del }
		rw.Rewrite[am] = func() string { return rw.Del }
	}
	fmt.Fprintf(os.Stderr, "fniface: %d of %d function interfaces with a lambda adapter become function types (%d lambdas, %d calls)\n", done, len(names), lambdas, calls)
	if !*dry {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
}

// mayBe: fn's signature is the method of interface type i for some type arguments (so fn's
// receiver may implement an instance of it).
func mayBe(i *types.TypeName, fn *types.Func) bool {
	it, ok := i.Type().Underlying().(*types.Interface)
	if !ok || it.NumMethods() != 1 {
		return true
	}
	a, b := it.Method(0).Type().(*types.Signature), fn.Type().(*types.Signature)
	if a.Params().Len() != b.Params().Len() || a.Results().Len() != b.Results().Len() || a.Variadic() != b.Variadic() {
		return false
	}
	bind := map[*types.TypeParam]types.Type{}
	for k := 0; k < a.Params().Len(); k++ {
		if !unify(a.Params().At(k).Type(), b.Params().At(k).Type(), bind) {
			return false
		}
	}
	for k := 0; k < a.Results().Len(); k++ {
		if !unify(a.Results().At(k).Type(), b.Results().At(k).Type(), bind) {
			return false
		}
	}
	return true
}

func unify(a, b types.Type, bind map[*types.TypeParam]types.Type) bool {
	switch x := a.(type) {
	case *types.TypeParam:
		if t, ok := bind[x]; ok {
			return types.Identical(t, b)
		}
		bind[x] = b
		return true
	case *types.Pointer:
		y, ok := b.(*types.Pointer)
		return ok && unify(x.Elem(), y.Elem(), bind)
	case *types.Slice:
		y, ok := b.(*types.Slice)
		return ok && unify(x.Elem(), y.Elem(), bind)
	case *types.Named:
		y, ok := b.(*types.Named)
		if !ok || x.Origin() != y.Origin() || x.TypeArgs().Len() != y.TypeArgs().Len() {
			return false
		}
		for k := 0; k < x.TypeArgs().Len(); k++ {
			if !unify(x.TypeArgs().At(k), y.TypeArgs().At(k), bind) {
				return false
			}
		}
		return true
	}
	return types.Identical(a, b)
}
