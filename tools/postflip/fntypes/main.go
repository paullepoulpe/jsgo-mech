// Command fntypes is a post-flip step: a parameter of Go function type whose erased argument
// (`any`, Java's type argument the translation erased: Predicate<JSType> as func(any) bool)
// every function given to it casts to one type T on entry takes T instead:
//
//	func JSTypeIterations_anyTypeMatches(predicate func(any) bool, ...)     func JSTypeIterations_anyTypeMatches(predicate func(*JSType) bool, ...)
//		... predicate(types.get(i).self()) ...                         ->       ... predicate(types.get(i)) ...
//	JSTypeIterations_anyTypeMatches(func(p0 any) bool {                     JSTypeIterations_anyTypeMatches(func(p0 *JSType) bool {
//		return type_.equals_JSType(to_JSType(p0)) }, this)                      return type_.equals_JSType(p0) }, this)
//
// funcparams made these parameters Go funcs over Java's erased functional interfaces; the
// translation then casts the Object argument at the top of each lambda, as javac's bridge does
// before the lambda's body runs. A group is the parameters that pass the function to one
// another (anyTypeMatches -> anyTypeMatchesImmutableList); it is typed at position k when
//   - every use of each parameter calls it or passes it to a parameter of the group;
//   - each function of the package declaring one is only called (never a value), and a method
//     is not one an interface lists;
//   - every value given to the group is a function literal, a package function (a named
//     lambda) used only as such values, or the method value o.m of a bridge m whose typed mT_
//     takes T (implsplit's bridges: o.mT_ then, o evaluated where it was);
//   - every use of those functions' k-th parameter is an unboxing to T (cast_[T], to_X,
//     icast_X, str_, Node_of_, unbox_[T], .(T), .(X_as).as_X()) or a read that is boxed again;
//     at least one casts, and all agree on T;
//   - every call of the parameter passes at k the box of a T (x.self(), obj_(x), strobj_(x),
//     x.box_()) or a T.
// Then the call passes x itself, and the function receives it where it received the box and
// unboxed it: for a non-null x the cast gives x back (the class model's self() is the most
// derived object; to_X of it is x's part X, x itself), and a box of null is null in both.
// Results stay as they are. Left (counted by group): the reasons above. A rerun changes
// nothing.
//
// usage: fntypes [-n] [-v] <dir of package closure>
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

var (
	verbose bool
	done    = map[string]int{}
	left    = map[string]int{}
	where   = map[string][]string{}
)

func typeOf(e ast.Expr) types.Type {
	if tv, ok := rw.Info.Types[e]; ok {
		return tv.Type
	}
	return nil
}

func isAny(t types.Type) bool {
	i, ok := types.Unalias(t).(*types.Interface)
	return ok && i.Empty()
}

// slot is a parameter of function type with an erased argument.
type slot struct {
	obj   *types.Var
	field *ast.Field
	fd    *ast.FuncDecl
	sig   *types.Signature // the parameter's func type
	calls []*ast.CallExpr  // p(args)
}

// value is a function given to a group.
type value struct {
	lit    *ast.FuncLit
	decl   *ast.FuncDecl     // a named package function
	bridge *ast.SelectorExpr // o.m of an implsplit bridge
	tmeth  *types.Func       // its mT_
	at     ast.Expr          // the argument expression
}

var (
	slots   = map[*types.Var]*slot{}
	parent  = map[*types.Var]*types.Var{}
	bad     = map[*types.Var]string{}
	values  = map[*types.Var][]value{}
	declOf  = map[*types.Func]*ast.FuncDecl{}
	ifaceMs = map[string]bool{}
)

func find(o *types.Var) *types.Var {
	if p, ok := parent[o]; ok && p != o {
		r := find(p)
		parent[o] = r
		return r
	}
	parent[o] = o
	return o
}

func union(a, b *types.Var) { parent[find(a)] = find(b) }

func funcSig(t types.Type) *types.Signature {
	s, _ := types.Unalias(t).(*types.Signature)
	return s
}

func erased(s *types.Signature) bool {
	if s == nil {
		return false
	}
	for i := 0; i < s.Params().Len(); i++ {
		if isAny(s.Params().At(i).Type()) {
			return true
		}
	}
	return false
}

// bridgeOf returns the typed method of a bridge method m (body `return this.mT_(...)`).
func bridgeOf(f *types.Func) *types.Func {
	fd := declOf[f]
	if fd == nil || fd.Recv == nil || len(fd.Body.List) != 1 {
		return nil
	}
	r, ok := fd.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(r.Results) != 1 {
		return nil
	}
	c, ok := rw.Unparen(r.Results[0]).(*ast.CallExpr)
	if !ok {
		return nil
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != fd.Name.Name+"T_" {
		return nil
	}
	tf, _ := rw.Info.Uses[s.Sel].(*types.Func)
	return tf
}

// unboxTo returns the expression a use u of a parameter is unboxed by, and the type it gives.
func unboxTo(u *ast.Ident) (ast.Expr, types.Type) {
	p := rw.Up(u)
	switch x := p.(type) {
	case *ast.TypeAssertExpr:
		if x.Type == nil {
			return nil, nil
		}
		// .(X_as).as_X(): the translation's inline to_X
		if s, ok := rw.Up(x).(*ast.SelectorExpr); ok && s.X == x && strings.HasPrefix(s.Sel.Name, "as_") {
			if c, ok := rw.Parent[s].(*ast.CallExpr); ok && c.Fun == s && len(c.Args) == 0 {
				return c, typeOf(c)
			}
		}
		return x, typeOf(x)
	case *ast.CallExpr:
		if len(x.Args) != 1 || rw.Unparen(x.Args[0]) != ast.Expr(u) {
			return nil, nil
		}
		name, _ := rw.CallName(x)
		switch {
		case strings.HasPrefix(name, "cast_[") || strings.HasPrefix(name, "unbox_[") || strings.HasPrefix(name, "to_") || strings.HasPrefix(name, "icast_") || name == "str_" || name == "Node_of_":
			return x, typeOf(x)
		}
	}
	return nil, nil
}

// boxOf returns x when a is the box of x (x.self(), obj_(x), strobj_(x), any(x), x.box_()).
func boxOf(a ast.Expr) ast.Expr {
	call, ok := rw.Unparen(a).(*ast.CallExpr)
	if !ok {
		return nil
	}
	if id, ok := call.Fun.(*ast.Ident); ok && len(call.Args) == 1 && (id.Name == "obj_" || id.Name == "strobj_" || id.Name == "any") {
		return call.Args[0]
	}
	if ix, ok := call.Fun.(*ast.IndexExpr); ok && len(call.Args) == 1 {
		if id, ok := ix.X.(*ast.Ident); ok && id.Name == "obj_" {
			return call.Args[0]
		}
	}
	if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
		return s.X
	}
	return nil
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	for _, n := range rw.Pkg.Scope().Names() {
		if tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName); ok {
			if it, ok := tn.Type().Underlying().(*types.Interface); ok {
				for i := 0; i < it.NumMethods(); i++ {
					ifaceMs[it.Method(i).Name()] = true
				}
			}
		}
	}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
				declOf[rw.Info.Defs[fd.Name].(*types.Func)] = fd
			}
		}
	}
	// the slots
	for fo, fd := range declOf {
		if !strings.HasPrefix(rw.Fset.File(fd.Pos()).Name()[strings.LastIndex(rw.Fset.File(fd.Pos()).Name(), "/")+1:], "g_") {
			continue
		}
		for _, fl := range fd.Type.Params.List {
			for _, id := range fl.Names {
				o, _ := rw.Info.Defs[id].(*types.Var)
				if o == nil || !erased(funcSig(o.Type())) {
					continue
				}
				s := &slot{obj: o, field: fl, fd: fd, sig: funcSig(o.Type())}
				slots[o] = s
				find(o)
				if len(fl.Names) != 1 {
					bad[o] = "declared with other parameters in one field"
				}
				if fd.Recv != nil && ifaceMs[fd.Name.Name] {
					bad[o] = "its method is listed by an interface"
				}
				if fd.Type.TypeParams != nil || fd.Recv != nil && len(fd.Recv.List) > 0 && strings.Contains(rw.Text(fd.Recv.List[0].Type), "[") {
					bad[o] = "a generic function"
				}
				_ = fo
			}
		}
	}
	// uses of the slots: calls and passing on
	for o, s := range slots {
		for _, u := range rw.UsesOf[o] {
			p := rw.Up(u)
			if c, ok := p.(*ast.CallExpr); ok {
				if rw.Unparen(c.Fun) == ast.Expr(u) {
					s.calls = append(s.calls, c)
					continue
				}
				if q := paramSlot(c, u); q != nil {
					union(o, q)
					continue
				}
			}
			bad[o] = "used other than by calling it or passing it to the group"
		}
	}
	// values given at the calls of each slot's function
	for o, s := range slots {
		fo := rw.Info.Defs[s.fd.Name].(*types.Func)
		idx := paramIndex(s.fd, o)
		for _, u := range rw.UsesOf[fo] {
			fun := rw.VarExpr(u)
			c, ok := rw.Parent[fun].(*ast.CallExpr)
			if !ok || c.Fun != fun {
				bad[o] = "its function is used as a value"
				continue
			}
			if idx >= len(c.Args) {
				bad[o] = "a variadic call"
				continue
			}
			a := rw.Unparen(c.Args[idx])
			switch x := a.(type) {
			case *ast.FuncLit:
				values[o] = append(values[o], value{lit: x, at: a})
				continue
			case *ast.Ident:
				if q, ok := rw.Info.Uses[x].(*types.Var); ok && slots[q] != nil {
					union(o, q)
					continue
				}
				if f, ok := rw.Info.Uses[x].(*types.Func); ok && declOf[f] != nil && declOf[f].Recv == nil {
					values[o] = append(values[o], value{decl: declOf[f], at: a})
					continue
				}
			case *ast.SelectorExpr:
				if f, ok := rw.Info.Uses[x.Sel].(*types.Func); ok {
					if sel := rw.Info.Selections[x]; sel != nil && sel.Kind() == types.MethodVal {
						if tf := bridgeOf(f); tf != nil {
							values[o] = append(values[o], value{bridge: x, tmeth: tf, at: a})
							continue
						}
					}
				}
			}
			bad[o] = "given another value than a function literal, a package function or a bridge's method value"
		}
	}
	// the groups
	groups := map[*types.Var][]*types.Var{}
	for o := range slots {
		r := find(o)
		groups[r] = append(groups[r], o)
	}
	var roots []*types.Var
	for r := range groups {
		roots = append(roots, r)
	}
	for _, r := range roots {
		g := groups[r]
		sort.Slice(g, func(i, j int) bool { return g[i].Pos() < g[j].Pos() })
		groups[r] = g
	}
	sort.Slice(roots, func(i, j int) bool { return groups[roots[i]][0].Pos() < groups[roots[j]][0].Pos() })
	namedUses := map[*types.Func]int{}
	for _, r := range roots {
		for _, o := range groups[r] {
			for _, v := range values[o] {
				if v.decl != nil {
					namedUses[rw.Info.Defs[v.decl.Name].(*types.Func)]++
				}
			}
		}
	}
	for _, r := range roots {
		convert(groups[r], namedUses)
	}
	var ks []string
	for k := range done {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(os.Stderr, "fntypes: %d %s\n", done[k], k)
	}
	ks = ks[:0]
	for k := range left {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(os.Stderr, "fntypes: left: %d %s\n", left[k], k)
		if verbose {
			for _, w := range where[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", w)
			}
		}
	}
	if report || done["groups typed"] == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "fntypes: files changed: %d\n", rw.SaveChecked())
}

func leave(members []*types.Var, why string) {
	left["groups: "+why]++
	if verbose {
		var names []string
		for _, o := range members {
			names = append(names, slots[o].fd.Name.Name+"."+o.Name())
		}
		sort.Strings(names)
		where["groups: "+why] = append(where["groups: "+why], strings.Join(names, ", "))
	}
}

// paramSlot returns the slot of the parameter that argument u of call c is given to.
func paramSlot(c *ast.CallExpr, u *ast.Ident) *types.Var {
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
	fd := declOf[fo]
	if fd == nil {
		return nil
	}
	for i, a := range c.Args {
		if rw.Unparen(a) == ast.Expr(u) {
			k := 0
			for _, fl := range fd.Type.Params.List {
				for _, n := range fl.Names {
					if k == i {
						q, _ := rw.Info.Defs[n].(*types.Var)
						if slots[q] != nil {
							return q
						}
						return nil
					}
					k++
				}
			}
		}
	}
	return nil
}

func paramIndex(fd *ast.FuncDecl, o types.Object) int {
	k := 0
	for _, fl := range fd.Type.Params.List {
		for _, n := range fl.Names {
			if rw.Info.Defs[n] == o {
				return k
			}
			k++
		}
	}
	return -1
}

// params returns the parameter identifiers of a value's function, and its signature.
func (v value) params() ([]*ast.Ident, *ast.FuncType) {
	var ft *ast.FuncType
	switch {
	case v.lit != nil:
		ft = v.lit.Type
	case v.decl != nil:
		ft = v.decl.Type
	default:
		return nil, nil
	}
	var ids []*ast.Ident
	for _, fl := range ft.Params.List {
		if len(fl.Names) == 0 {
			ids = append(ids, nil)
		}
		ids = append(ids, fl.Names...)
	}
	return ids, ft
}

func convert(members []*types.Var, namedUses map[*types.Func]int) {
	for _, o := range members {
		if b := bad[o]; b != "" {
			leave(members, b)
			return
		}
	}
	sig := slots[members[0]].sig
	for _, o := range members[1:] {
		if !types.Identical(slots[o].sig, sig) {
			leave(members, "parameters of different function types")
			return
		}
	}
	var vals []value
	for _, o := range members {
		vals = append(vals, values[o]...)
	}
	if len(vals) == 0 {
		leave(members, "never given a function")
		return
	}
	for _, v := range vals {
		if v.decl != nil {
			f := rw.Info.Defs[v.decl.Name].(*types.Func)
			if len(rw.UsesOf[f]) != namedUses[f] {
				leave(members, "a named function given to it is also used elsewhere")
				return
			}
		}
	}
	// the type of each erased position
	n := sig.Params().Len()
	elem := make([]types.Type, n)
	type rd struct {
		use *ast.Ident
		pos int
	}
	var reads []rd
	for k := 0; k < n; k++ {
		if !isAny(sig.Params().At(k).Type()) {
			continue
		}
		var t types.Type
		ok := true
		for _, v := range vals {
			if v.bridge != nil {
				ms := v.tmeth.Type().(*types.Signature)
				ps := ms.Params()
				if ps.Len() != n || !types.Identical(ms.Results(), sig.Results()) {
					ok = false
					break
				}
				bt := ps.At(k).Type()
				if t != nil && !types.Identical(t, bt) {
					ok = false
					break
				}
				t = bt
				continue
			}
			ids, _ := v.params()
			if len(ids) != n {
				ok = false
				break
			}
			if ids[k] == nil || ids[k].Name == "_" {
				continue
			}
			for _, u := range rw.UsesOf[rw.Info.Defs[ids[k]]] {
				reads = append(reads, rd{u, k})
				if _, ut := unboxTo(u); ut != nil {
					if t != nil && !types.Identical(t, ut) {
						ok = false
						break
					}
					t = ut
				}
			}
		}
		if ok && t != nil && !isAny(t) {
			elem[k] = t
		}
	}
	typed := false
	for _, t := range elem {
		typed = typed || t != nil
	}
	if !typed {
		leave(members, "no function given to it casts the erased argument, or they disagree")
		return
	}
	// every call passes a box of T (or a T) at the typed positions
	var edits []rw.Edit
	for _, o := range members {
		for _, c := range slots[o].calls {
			for k, t := range elem {
				if t == nil {
					continue
				}
				a := c.Args[k]
				if types.Identical(typeOf(a), t) {
					continue
				}
				x := boxOf(a)
				if x == nil || !types.Identical(typeOf(x), t) {
					leave(members, "a call passes a value that is not the box of the type")
					return
				}
				aa, xx := a, x
				edits = append(edits, rw.Edit{Node: aa, Gen: func() string { return rw.Render(xx) }})
			}
		}
	}
	// a parameter read once, by the first statement `var x T = cast(p)` (with `_ = x` after
	// it): the parameter is x and the statements go, as lamparams writes them
	renamed := map[*ast.Ident]bool{}
	for _, v := range vals {
		ids, _ := v.params()
		var body *ast.BlockStmt
		switch {
		case v.lit != nil:
			body = v.lit.Body
		case v.decl != nil:
			body = v.decl.Body
		default:
			continue
		}
		for k, id := range ids {
			if k >= n || elem[k] == nil || id == nil || id.Name == "_" || len(body.List) == 0 {
				continue
			}
			us := rw.UsesOf[rw.Info.Defs[id]]
			if len(us) != 1 {
				continue
			}
			e, ut := unboxTo(us[0])
			if ut == nil || !types.Identical(ut, elem[k]) {
				continue
			}
			ds, ok := body.List[0].(*ast.DeclStmt)
			if !ok {
				continue
			}
			gd := ds.Decl.(*ast.GenDecl)
			vs, ok := gd.Specs[0].(*ast.ValueSpec)
			if !ok || len(gd.Specs) != 1 || len(vs.Names) != 1 || len(vs.Values) != 1 || rw.Unparen(vs.Values[0]) != e || !types.Identical(rw.Info.Defs[vs.Names[0]].Type(), elem[k]) {
				continue
			}
			x := vs.Names[0].Name
			renamed[us[0]] = true
			edits = append(edits, rw.Edit{Node: id, Gen: func() string { return x }}, rw.Edit{Node: ds, Gen: func() string { return rw.Del }})
			if len(body.List) > 1 {
				if as, ok := body.List[1].(*ast.AssignStmt); ok && len(as.Lhs) == 1 && len(as.Rhs) == 1 && rw.Text(as.Lhs[0]) == "_" && rw.Text(as.Rhs[0]) == x {
					edits = append(edits, rw.Edit{Node: as, Gen: func() string { return rw.Del }})
				}
			}
			done["parameters named as the variable their function cast them into"]++
		}
	}
	// the reads of the functions' parameters
	for _, r := range reads {
		t := elem[r.pos]
		if t == nil || renamed[r.use] {
			continue
		}
		u := r.use
		if e, ut := unboxTo(u); ut != nil && types.Identical(ut, t) {
			edits = append(edits, rw.Edit{Node: e, Gen: func() string { return u.Name }})
			continue
		}
		cv := rw.ConvFor(t)
		if !cv.OK {
			leave(members, "a read of the argument kept boxed, of a type with no boxing conversion")
			return
		}
		edits = append(edits, rw.Edit{Node: u, Gen: func() string { return cv.Box(u.Name) }})
	}
	ts := func(t types.Type) string { return types.TypeString(t, types.RelativeTo(rw.Pkg)) }
	// the new function type
	newSig := func() string {
		var ps []string
		for k := 0; k < n; k++ {
			if elem[k] != nil {
				ps = append(ps, ts(elem[k]))
			} else {
				ps = append(ps, ts(sig.Params().At(k).Type()))
			}
		}
		res := ""
		switch sig.Results().Len() {
		case 0:
		case 1:
			res = " " + ts(sig.Results().At(0).Type())
		default:
			var rs []string
			for i := 0; i < sig.Results().Len(); i++ {
				rs = append(rs, ts(sig.Results().At(i).Type()))
			}
			res = " (" + strings.Join(rs, ", ") + ")"
		}
		return "func(" + strings.Join(ps, ", ") + ")" + res
	}()
	for _, o := range members {
		fl := slots[o].field
		edits = append(edits, rw.Edit{Node: fl.Type, Gen: func() string { return newSig }})
	}
	// the values' parameter types
	seenDecl := map[*ast.FuncDecl]bool{}
	for _, v := range vals {
		v := v
		if v.bridge != nil {
			b := v.bridge
			name := v.tmeth.Name()
			edits = append(edits, rw.Edit{Node: b.Sel, Gen: func() string { return name }})
			continue
		}
		if v.decl != nil {
			if seenDecl[v.decl] {
				continue
			}
			seenDecl[v.decl] = true
		}
		_, ft := v.params()
		for _, fl := range ft.Params.List {
			fl := fl
			var t types.Type
			k := 0
			for _, f2 := range ft.Params.List {
				if f2 == fl {
					break
				}
				k += max(1, len(f2.Names))
			}
			for j := 0; j < max(1, len(fl.Names)); j++ {
				if k+j < n && elem[k+j] != nil {
					if t != nil && !types.Identical(t, elem[k+j]) {
						leave(members, "a function declares two parameters of different new types in one field")
						return
					}
					t = elem[k+j]
				}
			}
			if t == nil {
				continue
			}
			if len(fl.Names) > 1 {
				for j := 0; j < len(fl.Names); j++ {
					if elem[k+j] == nil {
						leave(members, "a function declares two parameters of different new types in one field")
						return
					}
				}
			}
			tt := t
			edits = append(edits, rw.Edit{Node: fl.Type, Gen: func() string { return ts(tt) }})
		}
	}
	for _, e := range edits {
		rw.Rewrite[e.Node] = e.Gen
	}
	done["groups typed"]++
	done["parameters typed"] += len(members)
	for _, v := range vals {
		switch {
		case v.lit != nil:
			done["function literals given to them"]++
		case v.decl != nil:
			done["package functions (named lambdas) given to them"]++
		default:
			done["bridges' method values given to them, now the typed method"]++
		}
	}
	if verbose {
		var names []string
		for _, o := range members {
			names = append(names, slots[o].fd.Name.Name+"."+o.Name())
		}
		sort.Strings(names)
		fmt.Fprintf(os.Stderr, "        typed: %s -> %s\n", strings.Join(names, ", "), newSig)
	}
}
