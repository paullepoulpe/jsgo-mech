package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"strings"

	"postflip/rw"
)

func set(n ast.Node, f func() string) {
	if _, ok := rw.Rewrite[n]; ok {
		panic(fmt.Sprintf("fbound: two rewrites of %s: %s", rw.Fset.Position(n.Pos()), rw.Text(n)))
	}
	rw.Rewrite[n] = f
}

// guarded: a rewrite of a node inside a decl the step writes out applies only while the copy
// is rendered; the base's own decl (when it stays) is rendered as it was.
func guarded(n ast.Node, f func() string) func() string {
	if copyDecl[declOf(n)] == nil {
		return f
	}
	return func() string {
		if cur == nil {
			return rw.RenderChildren(n)
		}
		return f()
	}
}

func register() {
	for _, f := range rw.Files {
		if !isG(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.Ident:
				if copyDecl[declOf(x)] != nil {
					if v, ok := rw.Info.Uses[x].(*types.Var); ok && !v.IsField() && mentionsErased(v.Type()) {
						set(x, guarded(x, func() string { return wrapUp(x, x.Name) }))
					}
					if tn, ok := rw.Info.Uses[x].(*types.TypeName); ok && bases[tn] != nil {
						set(x, func() string {
							if cur != nil {
								return cur.sigma[tn].Name()
							}
							return x.Name
						})
					}
				}
			case *ast.FuncLit:
				if call, ok := rw.Parent[x].(*ast.CallExpr); ok && call.Fun == x && copyDecl[declOf(x)] != nil && x.Type.Results != nil && len(x.Type.Results.List) == 1 {
					res := x.Type.Results
					set(res, guarded(res, func() string {
						if et := expected(call); et != nil && !types.Identical(et, subst(old(res.List[0].Type), cur)) {
							count("a literal called at once returns the typed result", x)
							return tstr(et)
						}
						return rw.RenderChildren(res)
					}))
				}
			case *ast.CallExpr:
				if g, _, _, _ := groupCall(x); g != nil {
					set(x, guarded(x, func() string { return renderGroupCall(x) }))
					return true
				}
				if isUpCall(x) {
					set(x, guarded(x, func() string { return renderUp(x) }))
					return true
				}
				if t, _ := castOf(x); t != nil || strings.HasPrefix(callName(x), "iup_") {
					set(x, guarded(x, func() string { return renderConv(x) }))
					return true
				}
			case *ast.TypeAssertExpr:
				if x.Type != nil {
					set(x, guarded(x, func() string { return renderConv(x) }))
				}
			case *ast.SelectorExpr:
				if s := rw.Info.Selections[x]; s != nil && s.Kind() == types.FieldVal && movedOwner(s.Obj().(*types.Var)) != nil {
					set(x, guarded(x, func() string { return wrapUp(x, rw.RenderChildren(x)) }))
				}
			}
			return true
		})
	}
	registerDecls()
}

func callName(c *ast.CallExpr) string {
	n, _ := rw.CallName(c)
	return n
}

func isUpCall(c *ast.CallExpr) bool {
	sel, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || len(c.Args) != 0 || !strings.HasPrefix(sel.Sel.Name, "up_") {
		return false
	}
	for _, b := range bases {
		if sel.Sel.Name == "up_"+b.tn.Name() {
			return true
		}
	}
	return false
}

// renderUp: x.up_B() given to a slot now typed as x is x.
func renderUp(c *ast.CallExpr) string {
	x := c.Fun.(*ast.SelectorExpr).X
	xt := nt(x)
	if subs[ptrNamed(xt)] != nil {
		if et := expected(c); et != nil && types.Identical(et, xt) {
			count("an upcast x.up_B() given to a slot now typed as x is x", c)
			return rw.Render(x)
		}
	}
	return rw.RenderChildren(c)
}

// wrapUp: a value whose type the step made σ(B), in a slot that keeps the erased B (or an
// interface, any), is upcast to B: the value the erased code had there.
func wrapUp(e ast.Expr, text string) string {
	ot, nwt := old(e), nt(e)
	if nwt == nil || ot == nil || types.Identical(ot, nwt) {
		return text
	}
	bn := ptrNamed(ot)
	if bases[bn] == nil {
		return text
	}
	et := expected(e)
	if et == nil || types.Identical(et, nwt) {
		return text
	}
	if !types.IsInterface(et) && !types.Identical(et, ot) {
		fmt.Fprintf(os.Stderr, "fbound: %s: %s typed %s in a slot of %s\n", rw.Fset.Position(e.Pos()), rw.Text(e), tstr(nwt), tstr(et))
		os.Exit(1)
	}
	count("a typed result in a slot of the erased type is upcast (up_B)", e)
	if needsParen(text) {
		text = "(" + text + ")"
	}
	return text + ".up_" + bn.Name() + "()"
}

func needsParen(s string) bool {
	return strings.ContainsAny(s, " ")
}

// renderGroupCall renders a call of a group's method: the typed method on a subclass
// receiver (renamed for an implementation's name, a super call, a folded T_ cast).
func renderGroupCall(c *ast.CallExpr) string {
	g, s, sup, _ := groupCall(c)
	if s == nil {
		if fp := castAround(c); fp != nil {
			left("a cast of the result of a call on the erased base", fp)
		}
		return rw.RenderChildren(c)
	}
	return wrapUp(c, renderTypedCall(c, g, s, sup))
}

func renderTypedCall(c *ast.CallExpr, g *group, s *sub, sup bool) string {
	sel := c.Fun.(*ast.SelectorExpr)
	name := sel.Sel.Name
	recv := rw.Render(sel.X)
	switch {
	case sup:
		recv = rw.Render(sel.X.(*ast.SelectorExpr).X)
		if g.superUsed[s] {
			name = g.name + "Super_"
		} else {
			name = g.tname
		}
		count("Java's super call is a call of the base's code written out", c)
	case name == g.implName && g.implName != g.name:
		name = g.tname
	case g.ires != nil && foldParent(c) != nil:
		name = g.tname
	}
	var args []string
	for _, a := range c.Args {
		args = append(args, rw.Render(a))
	}
	return recv + "." + name + "(" + strings.Join(args, ", ") + ")"
}

// castAround: the cast (through self()) around a call, if any.
func castAround(c *ast.CallExpr) ast.Node {
	p := rw.Up(c)
	if sel, ok := p.(*ast.SelectorExpr); ok && sel.Sel.Name == "self" {
		if sc, ok := rw.Up(sel).(*ast.CallExpr); ok && sc.Fun == sel {
			p = rw.Up(sc)
			if ta, ok := p.(*ast.TypeAssertExpr); ok {
				return ta
			}
		}
	}
	if kc, ok := p.(*ast.CallExpr); ok {
		if t, _ := castOf(kc); t != nil {
			return kc
		}
	}
	return nil
}

// renderConv: a cast or conversion: folded when it takes a group call's typed result to its
// own type; a conversion of a typed value to the interface in a slot now typed as the value
// is the value.
func renderConv(k ast.Expr) string {
	// the operand through self()
	var x ast.Expr
	switch kk := k.(type) {
	case *ast.CallExpr:
		if len(kk.Args) != 1 {
			return rw.RenderChildren(k)
		}
		x = kk.Args[0]
	case *ast.TypeAssertExpr:
		x = kk.X
	}
	if y := selfOf(x); y != nil {
		x = y
	}
	x = rw.Unparen(x)
	if c, ok := x.(*ast.CallExpr); ok {
		if g, s, sup, _ := groupCall(c); g != nil && s != nil {
			if foldParent(c) == k {
				switch kk := k.(type) {
				case *ast.TypeAssertExpr:
					if g.ires != nil {
						count("an assertion of an interface result, dereferenced at once, is a call of the typed method (T_)", kk)
					} else {
						count("x.self().(T) of a typed result, dereferenced at once, is the result", kk)
					}
				case *ast.CallExpr:
					if t, _ := castOf(kk); t != nil && bases[ptrNamed(t)] != nil {
						if bases[ptrNamed(foldedType(kk))] != nil {
							count("a cast of a typed result to the erased base (a slot of AbstractScope<?, ?>) is its up_B", kk)
							return renderTypedCall(c, g, s, sup) + ".up_" + ptrNamed(t).Name() + "()"
						}
					}
					if t, _ := castOf(kk); t != nil && types.IsInterface(t) {
						count("a cast of a typed result to an interface it implements is iup_I of it", kk)
						return "iup_" + ptrOrName(t) + "(" + renderTypedCall(c, g, s, sup) + ")"
					}
					if g.ires != nil {
						count("a cast of an interface result is a call of the typed method (T_)", kk)
					} else {
						count("a cast of a typed result's self() to its type is the result", kk)
					}
				}
				return renderTypedCall(c, g, s, sup)
			}
			if inCopy(k) && retConv(k, x) {
				count("a conversion to the interface in a typed return is the value", k)
				return rw.Render(x)
			}
			if _, isAssert := k.(*ast.TypeAssertExpr); !isAssert {
				left("a cast of a typed result to another type", k)
			}
			return rw.RenderChildren(k)
		}
	}
	if inCopy(k) && retConv(k, x) {
		count("a conversion to the interface in a typed return is the value", k)
		return rw.Render(x)
	}
	return rw.RenderChildren(k)
}

// retConv: k converts x to an interface (iup_I(x), icast_I(x.self()), x.self().(I)) where the
// slot now takes x's own type.
func retConv(k ast.Expr, x ast.Expr) bool {
	kt := old(k)
	if kt == nil || !types.IsInterface(kt) {
		return false
	}
	et := expected(k)
	return et != nil && types.Identical(et, nt(x)) && subs[ptrNamed(nt(x))] != nil
}

// ---- decls

func docOf(fd *ast.FuncDecl) string {
	f := rw.Fset.File(fd.Pos())
	src := rw.Src[f]
	a := f.Offset(fd.Pos())
	start := a
	for {
		i := start - 1
		if i < 0 || src[i] != '\n' {
			break
		}
		j := i
		for j > 0 && src[j-1] != '\n' {
			j--
		}
		if !strings.HasPrefix(strings.TrimSpace(string(src[j:i])), "//") {
			break
		}
		start = j
	}
	return string(src[start:a])
}

func sigma(s *sub) string {
	var parts []string
	for _, p := range s.b.jc.params {
		er := erasedOf(s.b, p)
		parts = append(parts, p+" = "+s.sigma[er].Name())
	}
	return strings.Join(parts, ", ")
}

func renderAs(s *sub, g *group, typed bool, f func() string) string {
	sc, sg, st := cur, curGroup, curTyped
	cur, curGroup, curTyped = s, g, typed
	defer func() { cur, curGroup, curTyped = sc, sg, st }()
	return f()
}

func paramNames(fd *ast.FuncDecl) []string {
	var names []string
	for _, fl := range fd.Type.Params.List {
		for _, n := range fl.Names {
			names = append(names, n.Name)
		}
	}
	return names
}

// typedText: the method written out for s, and its bridges.
func typedText(g *group, s *sub) string {
	src := g.impl
	own := false
	if o := s.decls[g.implName]; o != nil && copyDecl[o] == g {
		src, own = o, true
	}
	var b strings.Builder
	text := renderAs(s, g, true, func() string {
		params := rw.Render(src.Type.Params)
		results := ""
		if src.Type.Results != nil {
			results = " " + rw.Render(src.Type.Results)
		}
		if g.ires != nil {
			results = " " + tstr(paramType(g, s))
		}
		return "func (this *" + s.tn.Name() + ") " + g.tname + params + results + " " + rw.Render(src.Body)
	})
	doc := docOf(src)
	if own {
		fmt.Fprintf(&b, "%s// %s is %s's override of %s.%s, typed (fbound: %s).\n%s", doc, g.tname, s.tn.Name(), g.b.tn.Name(), javaName(g), sigma(s), text)
	} else {
		fmt.Fprintf(&b, "%s// %s is %s.%s written out for %s (fbound: %s).\n%s", doc, g.tname, g.b.tn.Name(), javaName(g), s.tn.Name(), sigma(s), text)
	}
	params := renderAs(s, g, false, func() string { return rw.Render(src.Type.Params) })
	args := strings.Join(paramNames(src), ", ")
	if g.ires != nil {
		in := ptrOrName(g.ires)
		fmt.Fprintf(&b, "\n\n// %s is %s with the result of the interface's type (javac's bridge; fbound).\nfunc (this *%s) %s%s %s {\n\treturn iup_%s(this.%s(%s))\n}",
			g.name, g.tname, s.tn.Name(), g.name, params, tstr(g.ires), in, g.tname, args)
	}
	if g.superUsed[s] {
		st := renderAs(s, g, true, func() string {
			results := ""
			if g.impl.Type.Results != nil {
				results = " " + rw.Render(g.impl.Type.Results)
			}
			return "func (this *" + s.tn.Name() + ") " + g.name + "Super_" + rw.Render(g.impl.Type.Params) + results + " " + rw.Render(g.impl.Body)
		})
		fmt.Fprintf(&b, "\n\n%s// %sSuper_ is %s.%s written out for %s, the method %s's override calls as super (fbound: %s).\n%s",
			docOf(g.impl), g.name, g.b.tn.Name(), javaName(g), s.tn.Name(), g.tname, sigma(s), st)
	}
	if g.kept && (g.d != nil || g.toDisp) {
		dn := dispName(g)
		var gf *ast.FuncDecl = g.d
		if gf == nil {
			gf = g.impl
		}
		sig := rw.Info.Defs[gf.Name].Type().(*types.Signature)
		var ps []string
		for i := 0; i < sig.Params().Len(); i++ {
			if mentionsErased(sig.Params().At(i).Type()) {
				fmt.Fprintf(os.Stderr, "fbound: %s.%s takes the erased type and is called on the erased base: not handled\n", g.b.tn.Name(), g.name)
				os.Exit(1)
			}
			ps = append(ps, fmt.Sprintf("p%d %s", i, tstr(sig.Params().At(i).Type())))
		}
		var as []string
		for i := range ps {
			as = append(as, fmt.Sprintf("p%d", i))
		}
		call := "this." + g.tname + "(" + strings.Join(as, ", ") + ")"
		res := ""
		ret := "\t" + call
		if sig.Results().Len() == 1 {
			rt := sig.Results().At(0).Type()
			res = " " + tstr(rt)
			switch {
			case bases[ptrNamed(rt)] != nil:
				call += ".up_" + ptrNamed(rt).Name() + "()"
			case g.ires != nil:
				call = "iup_" + ptrOrName(g.ires) + "(" + call + ")"
			}
			ret = "\treturn " + call
		}
		fmt.Fprintf(&b, "\n\n// %s is %s for the erased %s's dispatch (javac's bridge; fbound).\nfunc (this *%s) %s(%s)%s {\n%s\n}",
			dn, g.tname, g.b.tn.Name(), s.tn.Name(), dn, strings.Join(ps, ", "), res, ret)
	}
	return b.String()
}

func ptrOrName(t types.Type) string {
	if n, ok := types.Unalias(t).(*types.Named); ok {
		return n.Obj().Name()
	}
	return tstr(t)
}

func javaName(g *group) string { return g.name }

func dispName(g *group) string {
	if g.d != nil {
		return g.implName
	}
	return g.name + "_impl"
}

func registerDecls() {
	appendTo := map[*sub][]string{}
	for _, g := range groups {
		g := g
		for _, s := range g.b.subs {
			s := s
			var o, w *ast.FuncDecl
			if d := s.decls[g.implName]; d != nil && copyDecl[d] == g {
				o = d
			}
			if d := s.decls[g.name]; d != nil && ownerSub[d] == s && d != o {
				w = d
			}
			// the subclass's override and dispatch wrapper go; the typed method and its
			// bridges follow the subclass's declarations
			if o != nil {
				set(o, func() string { return rw.DelDoc })
			}
			if w != nil {
				set(w, func() string { return rw.DelDoc })
			}
			appendTo[s] = append(appendTo[s], g.name)
		}
		// the base
		del := func() string { return rw.DelDoc }
		if !g.kept {
			if g.d != nil {
				set(g.d, del)
			}
			if g.impl != nil {
				set(g.impl, del)
			}
		} else {
			if g.d != nil && g.impl != nil {
				set(g.impl, del)
			}
			if g.toDisp {
				fd := g.impl
				set(fd, func() string {
					sig := rw.Info.Defs[fd.Name].Type().(*types.Signature)
					ret := "\t"
					if sig.Results().Len() == 1 {
						ret = "\treturn "
					}
					return fmt.Sprintf("// %s dispatches to the subclass's %s (fbound).\nfunc (this *%s) %s%s%s {\n%sthis.self_.%s(%s)\n}",
						g.name, g.tname, g.b.tn.Name(), g.name, rw.Text(fd.Type.Params), resultsText(fd), ret, dispName(g), strings.Join(paramNames(fd), ", "))
				})
			}
		}
	}
	// appended methods: after the subclass file's last declaration
	for s, names := range appendTo {
		s, names := s, names
		// after the last declaration that stays (the overrides and wrappers go)
		var last ast.Decl
		for i := len(s.file.Decls) - 1; i >= 0; i-- {
			if fd, ok := s.file.Decls[i].(*ast.FuncDecl); ok && ownerSub[fd] == s {
				continue
			}
			last = s.file.Decls[i]
			break
		}
		prev := rw.Rewrite[last]
		delete(rw.Rewrite, last)
		set(last, func() string {
			t := ""
			if prev != nil {
				t = prev()
				if t == rw.DelDoc || t == rw.Del {
					panic("fbound: the last declaration of " + s.tn.Name() + "'s file is deleted")
				}
			} else {
				t = rw.RenderChildren(last)
			}
			for _, g := range groups {
				if g.b != s.b {
					continue
				}
				for _, n := range names {
					if n == g.name {
						t += "\n\n" + typedText(g, s)
					}
				}
			}
			return t
		})
	}
	for _, b := range bases {
		b := b
		// moved fields
		lines := strings.Split(rw.Text(b.spec), "\n")
		movedLine := map[string]string{}
		var keep []string
		for _, l := range lines {
			fs := strings.Fields(l)
			drop := false
			for _, m := range b.moved {
				if len(fs) > 0 && fs[0] == m.Name() {
					drop = true
					movedLine[m.Name()] = l
				}
			}
			if !drop {
				keep = append(keep, l)
			}
		}
		set(b.spec, func() string { return strings.Join(keep, "\n") })
		for _, s := range b.subs {
			s := s
			set(s.spec, func() string {
				var out []string
				for _, l := range strings.Split(rw.Text(s.spec), "\n") {
					out = append(out, l)
					if strings.TrimSpace(l) == b.tn.Name() {
						for _, m := range b.moved {
							l := movedLine[m.Name()]
							comment := ""
							if i := strings.Index(l, "//"); i >= 0 {
								comment = " " + l[i:]
							}
							out = append(out, "\t"+m.Name()+" "+tstr(subst(m.Type(), s))+comment)
						}
					}
				}
				return strings.Join(out, "\n")
			})
		}
		// the dispatch interface
		if b.iface != nil {
			set(b.iface, func() string {
				var out []string
				for _, l := range strings.Split(rw.Text(b.iface), "\n") {
					fs := strings.Fields(l)
					drop := false
					if len(fs) > 0 {
						name := strings.SplitN(fs[0], "(", 2)[0]
						for _, g := range groups {
							if g.b == b && g.d != nil && !g.kept && g.implName == name {
								drop = true
							}
						}
					}
					if strings.TrimSpace(l) == "}" {
						for _, g := range groups {
							if g.b == b && g.toDisp {
								fd := g.impl
								sig := rw.Info.Defs[fd.Name].Type().(*types.Signature)
								var ps []string
								for i := 0; i < sig.Params().Len(); i++ {
									ps = append(ps, tstr(sig.Params().At(i).Type()))
								}
								r := ""
								if sig.Results().Len() == 1 {
									r = " " + tstr(sig.Results().At(0).Type())
								}
								out = append(out, "\t"+dispName(g)+"("+strings.Join(ps, ", ")+")"+r)
							}
						}
					}
					if !drop {
						out = append(out, l)
					}
				}
				return strings.Join(out, "\n")
			})
		}
		if b.newFn != nil {
			set(b.newFn, func() string { return rw.DelDoc })
		}
		// a dispatch with the fast path for an object that is exactly a B, in a method the base
		// keeps: no object is a B any more (it is never constructed) and *B no longer has the
		// methods of the dispatch interface, so the dispatch is the method
		for _, fd := range b.order {
			fd := fd
			if gdecls[fd] || isWrapper(fd) == "" || !lostIfaceMethod(b, fd) {
				continue
			}
			set(fd.Body, func() string {
				count("a kept method's fast path for an object that is exactly the base goes (none is constructed)", fd)
				return "{\n\t" + rw.Render(fd.Body.List[1]) + "\n}"
			})
		}
	}
}

func resultsText(fd *ast.FuncDecl) string {
	if fd.Type.Results == nil {
		return ""
	}
	return " " + rw.Text(fd.Type.Results)
}

// lostIfaceMethod: the wrapper fd asserts self_ to *B, and B loses a method of self_'s interface.
func lostIfaceMethod(b *base, fd *ast.FuncDecl) bool {
	is := fd.Body.List[0].(*ast.IfStmt)
	ta := is.Init.(*ast.AssignStmt).Rhs[0].(*ast.TypeAssertExpr)
	if ptrNamed(rw.Info.Types[ta.Type].Type) != b.tn {
		return false
	}
	it, ok := old(ta.X).Underlying().(*types.Interface)
	if !ok {
		return false
	}
	for i := 0; i < it.NumMethods(); i++ {
		if d := b.decls[it.Method(i).Name()]; d != nil {
			if g := copyDecl[d]; g != nil && !g.kept {
				return true
			}
		}
	}
	return false
}
