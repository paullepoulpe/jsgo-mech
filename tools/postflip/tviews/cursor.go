package main

import (
	"go/ast"
	"go/token"
	"go/types"

	"postflip/rw"
)

// cursors types the immCursor_ variables and parameters whose every value is
// immCursorOf_(L.asImm()) of a *TList[T] of one T.
func cursors() {
	cur := rw.Pkg.Scope().Lookup("immCursor_")
	if cur == nil {
		return
	}
	isCur := func(t types.Type) bool {
		p, ok := types.Unalias(t).(*types.Pointer)
		return ok && types.Identical(p.Elem(), cur.Type())
	}
	// union-find over the cursor variables and parameters
	parent := map[types.Object]types.Object{}
	var find func(o types.Object) types.Object
	find = func(o types.Object) types.Object {
		if p, ok := parent[o]; ok && p != o {
			r := find(p)
			parent[o] = r
			return r
		}
		parent[o] = o
		return o
	}
	union := func(a, b types.Object) { parent[find(a)] = find(b) }
	type src struct {
		call *ast.CallExpr // immCursorOf_(L.asImm())
		list ast.Expr
		elem types.Type
	}
	var objs []types.Object
	srcs := map[types.Object][]src{}
	bad := map[types.Object]string{}
	declTypes := map[types.Object]ast.Expr{}
	// a value given to cursor slot o: a cursor of the package, or immCursorOf_(L.asImm())
	give := func(o types.Object, v ast.Expr) {
		v = rw.Unparen(v)
		if id, ok := v.(*ast.Ident); ok {
			if vo := rw.Info.Uses[id]; vo != nil && isCur(vo.Type()) {
				union(o, vo)
				return
			}
		}
		if n, c := rw.CallName(v); n == "immCursorOf_" && len(c.Args) == 1 {
			if L := asImmOf(c.Args[0]); L != nil {
				if ts := generic(typeOf(L), "TList"); ts != nil {
					srcs[o] = append(srcs[o], src{c, L, ts[0]})
					return
				}
			}
		}
		bad[o] = "given another value than immCursorOf_ of a typed immutable list"
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.ValueSpec:
				for i, id := range x.Names {
					o := rw.Info.Defs[id]
					if o == nil || !isCur(o.Type()) {
						continue
					}
					objs = append(objs, o)
					find(o)
					if x.Type != nil {
						declTypes[o] = x.Type
					}
					if i < len(x.Values) {
						give(o, x.Values[i])
					}
				}
			case *ast.AssignStmt:
				for i, l := range x.Lhs {
					id, ok := l.(*ast.Ident)
					if !ok || i >= len(x.Rhs) || len(x.Lhs) != len(x.Rhs) {
						continue
					}
					o := rw.Info.Defs[id]
					if o == nil {
						o = rw.Info.Uses[id]
					}
					if o == nil || !isCur(o.Type()) {
						continue
					}
					if x.Tok == token.DEFINE && rw.Info.Defs[id] != nil {
						objs = append(objs, o)
					}
					find(o)
					give(o, x.Rhs[i])
				}
			case *ast.FuncDecl:
				for _, fl := range x.Type.Params.List {
					for _, id := range fl.Names {
						o := rw.Info.Defs[id]
						if o != nil && isCur(o.Type()) {
							objs = append(objs, o)
							find(o)
							declTypes[o] = fl.Type
							fo := rw.Info.Defs[x.Name]
							for _, u := range rw.UsesOf[fo] {
								fun := rw.VarExpr(u)
								c, ok := rw.Parent[fun].(*ast.CallExpr)
								if !ok || c.Fun != fun {
									bad[o] = "its function is used as a value"
									continue
								}
								idx := paramIndex(x, id)
								if idx < len(c.Args) {
									give(o, c.Args[idx])
								}
							}
							if x.Recv != nil && interfaceLists(x.Name.Name) {
								bad[o] = "its method is listed by an interface"
							}
						}
					}
				}
			}
			return true
		})
	}
	// the uses of every slot
	nexts := map[types.Object][]*ast.CallExpr{}
	for _, o := range objs {
		for _, u := range rw.UsesOf[o] {
			p := rw.Up(u)
			if s, ok := p.(*ast.SelectorExpr); ok && s.X == u {
				c, ok := rw.Parent[s].(*ast.CallExpr)
				if ok && c.Fun == s && len(c.Args) == 0 && (s.Sel.Name == "hasNext" || s.Sel.Name == "remove") {
					continue
				}
				if ok && c.Fun == s && len(c.Args) == 0 && s.Sel.Name == "next" {
					nexts[o] = append(nexts[o], c)
					continue
				}
			}
			if c, ok := p.(*ast.CallExpr); ok && c.Fun != ast.Expr(u) {
				// an argument: fine if the parameter is a cursor slot (unioned by give)
				if fo := calleeOf(c); fo != nil && isCur(paramType(fo, argIndex(c, u))) {
					continue
				}
			}
			if as, ok := p.(*ast.AssignStmt); ok {
				lhs := false
				for _, l := range as.Lhs {
					lhs = lhs || l == ast.Expr(u)
				}
				if lhs {
					continue
				}
			}
			bad[o] = "used other than by hasNext / next / remove or as a cursor argument"
		}
	}
	// classes
	class := map[types.Object][]types.Object{}
	for _, o := range objs {
		r := find(o)
		class[r] = append(class[r], o)
	}
	for _, members := range class {
		var elem types.Type
		why := ""
		nsrc := 0
		for _, o := range members {
			if b := bad[o]; b != "" {
				why = b
			}
			for _, s := range srcs[o] {
				nsrc++
				if elem == nil {
					elem = s.elem
				} else if !types.Identical(elem, s.elem) {
					why = "lists of different element types"
				}
			}
		}
		if why == "" && elem == nil {
			why = "never given a cursor of a typed list"
		}
		var edits []rw.Edit
		if why == "" {
			ts := types.TypeString(elem, types.RelativeTo(rw.Pkg))
			for _, o := range members {
				for _, c := range nexts[o] {
					c := c
					if _, ok := rw.Parent[c].(*ast.ExprStmt); ok {
						continue
					}
					e, ok := read(elem, c, func() string { return rw.RenderChildren(c) })
					if !ok {
						why = "an element type with no boxing conversion, read boxed"
						break
					}
					edits = append(edits, e)
				}
				if t := declTypes[o]; t != nil {
					edits = append(edits, rw.Edit{Node: t, Gen: func() string { return "*immCursorT_[" + ts + "]" }})
				}
				for _, s := range srcs[o] {
					s := s
					edits = append(edits, rw.Edit{Node: s.call, Gen: func() string { return "immCursorTOf_(" + rw.Render(s.list) + ")" }})
				}
			}
		}
		if why != "" {
			left["cursor: "+why] += len(members)
			if verbose {
				for _, o := range members {
					where["cursor: "+why] = append(where["cursor: "+why], rw.Fset.Position(o.Pos()).String())
				}
			}
			continue
		}
		for _, e := range edits {
			rw.Rewrite[e.Node] = e.Gen
		}
		done["cursor variables and parameters over typed immutable lists: immCursorT_"] += len(members)
		for _, o := range members {
			done["cursor: immCursorOf_(L.asImm()) -> immCursorTOf_(L)"] += len(srcs[o])
			done["cursor: next() read typed"] += len(nexts[o])
		}
	}
}

func paramIndex(fd *ast.FuncDecl, id *ast.Ident) int {
	i := 0
	for _, fl := range fd.Type.Params.List {
		for _, n := range fl.Names {
			if n == id {
				return i
			}
			i++
		}
	}
	return -1
}

func calleeOf(c *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch f := rw.Unparen(c.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	}
	if id == nil {
		return nil
	}
	fo, _ := rw.Info.Uses[id].(*types.Func)
	return fo
}

func argIndex(c *ast.CallExpr, u *ast.Ident) int {
	for i, a := range c.Args {
		if rw.Unparen(a) == ast.Expr(u) {
			return i
		}
	}
	return -1
}

func paramType(f *types.Func, i int) types.Type {
	sig := f.Type().(*types.Signature)
	if i < 0 || i >= sig.Params().Len() {
		return types.Typ[types.Invalid]
	}
	return sig.Params().At(i).Type()
}

// interfaceLists reports whether an interface type of the package lists a method of this name.
func interfaceLists(name string) bool {
	for _, n := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok {
			continue
		}
		if it, ok := tn.Type().Underlying().(*types.Interface); ok {
			for i := 0; i < it.NumMethods(); i++ {
				if it.Method(i).Name() == name {
					return true
				}
			}
		}
	}
	return false
}
