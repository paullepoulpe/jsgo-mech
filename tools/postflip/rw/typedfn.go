package rw

// reflip33 (fifth flip): the typed function of an argument at an erased functional place.
//
// The post-flip tools tfnapi, tcmp and tgeneric give the runtime's typed variants the function f
// of a typed lambda adapter &X_tfn_T{f}. The translator's rule "function types" writes no
// adapter object: the same lambda is a function literal at the erased place's type that
// converts its arguments itself,
//
//	X[any, any](func(a_ any, b any) any { var a T = K(a_); _ = a; ..; return B(r) })
//	X[any](func(p0 any) { this.m(K(p0)) })
//
// and a function value given to an erased place is the translator's converting closure
//
//	func() X[any] { f_ := E; if f_ == nil { return nil }; return func(p0 any) R { return f_(K(p0)) } }()
//
// TypedFn reads the three forms (and a literal that already has the wanted signature, with or
// without a conversion to a function type) and gives the code of the function of signature
// want:
//   - the adapter's f, when f has the signature;
//   - E of a converting closure, when E has the signature, cannot be null (a function literal,
//     a method or function name, a conversion of one, or a call of a function all of whose
//     returns are such expressions: the runtime's Comparator combinators), and each argument of
//     f_ is the parameter at its position under the unboxing of its type;
//   - the literal with its parameters and result typed: a parameter p of type any whose wanted
//     type is T takes T when every use of p is the operand of an unboxing K(p) of T; K(p) is
//     then p (for `var v T = K(p)` at the head of the body: the parameter is v and the
//     statement, with its `_ = v`, goes). A result of type any whose wanted type is R is R
//     when every return of the literal returns B(e), B the boxing of R and e an R (then e), or
//     nil for a pointer or interface R, or a call of a parameterless literal whose returns
//     qualify (the translation of a conditional expression).
//
// Proof obligation. The typed variant calls the function with the collection's values where the
// erased API called it with their boxes, and reads its result where the erased API unboxed
// it. So for every x of T: K(box(x)) == x, and for every e of R: unbox(B(e)) == e, null
// included. K and B qualify when they are the unbox and box functions of the package's own
// conversions of that type, the tconv_[T]{box: B, unbox: K} values every typed collection of T
// is made with (all conversions of the type must agree), or the unboxing rw.ConvFor names for
// the type. With raw (the consumer is a plain cast_[R] of the result) a return may also give
// any(e) or e itself: cast_[R] of an interface value holding e is e.
//
// The function is robust to both forms: on a tree with adapter objects it answers as the
// tools' own readers did.

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

type tfConv struct {
	unbox map[types.Object]bool // the functions the conversions of the type unbox with
	box   map[types.Object]bool // the functions or methods they box with
	raw   bool                  // a conversion boxes with func(v T) any { return v }
	mixed bool                  // a conversion whose box or unbox is not one of the forms above
	elem  types.Type
}

var tfConvs []*tfConv
var tfConvsDone bool

func tfBase(e ast.Expr) types.Object {
	switch f := Unparen(e).(type) {
	case *ast.Ident:
		return Info.Uses[f]
	case *ast.IndexExpr:
		return tfBase(f.X)
	case *ast.IndexListExpr:
		return tfBase(f.X)
	case *ast.SelectorExpr:
		return Info.Uses[f.Sel]
	}
	return nil
}

func tfIsAny(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	it, ok := t.Underlying().(*types.Interface)
	return ok && it.Empty()
}

// tfTable lists the package's conversions: every composite literal tconv_[T]{..}.
func tfTable() {
	if tfConvsDone {
		return
	}
	tfConvsDone = true
	for _, f := range Files {
		ast.Inspect(f, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			nt, ok := types.Unalias(Info.TypeOf(cl)).(*types.Named)
			if !ok || nt.Origin().Obj().Name() != "tconv_" || nt.Obj().Pkg() != Pkg || nt.TypeArgs().Len() != 1 {
				return true
			}
			c := &tfConv{unbox: map[types.Object]bool{}, box: map[types.Object]bool{}, elem: nt.TypeArgs().At(0)}
			for _, el := range cl.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					c.mixed = true
					continue
				}
				key, _ := kv.Key.(*ast.Ident)
				if key == nil || key.Name != "box" && key.Name != "unbox" {
					continue
				}
				v := Unparen(kv.Value)
				if fl, ok := v.(*ast.FuncLit); ok {
					// func(v any) T { return K(v) } / func(v T) any { return v } / { return B(v) }
					var ret ast.Expr
					if len(fl.Body.List) == 1 && fl.Type.Params.NumFields() == 1 && len(fl.Type.Params.List[0].Names) == 1 {
						if rs, ok := fl.Body.List[0].(*ast.ReturnStmt); ok && len(rs.Results) == 1 {
							ret = Unparen(rs.Results[0])
						}
					}
					p := ""
					if ret != nil {
						p = fl.Type.Params.List[0].Names[0].Name
					}
					if id, ok := ret.(*ast.Ident); ok && key.Name == "box" && id.Name == p {
						c.raw = true
						continue
					}
					if call, ok := ret.(*ast.CallExpr); ok && len(call.Args) == 1 {
						if id, ok := Unparen(call.Args[0]).(*ast.Ident); ok && id.Name == p {
							if o := tfBase(call.Fun); o != nil {
								if key.Name == "box" {
									c.box[o] = true
								} else {
									c.unbox[o] = true
								}
								continue
							}
						}
					}
					c.mixed = true
					continue
				}
				o := tfBase(v)
				if _, isFn := o.(*types.Func); !isFn {
					c.mixed = true
					continue
				}
				if key.Name == "box" {
					c.box[o] = true
				} else {
					c.unbox[o] = true
				}
			}
			tfConvs = append(tfConvs, c)
			return true
		})
	}
}

// tfSame: the conversion's element type is t (for a conversion made by a generic function: the
// same generic type at the function's own type parameters).
func tfSame(elem, t types.Type) bool {
	if types.Identical(elem, t) {
		return true
	}
	a, b := types.Unalias(elem), types.Unalias(t)
	if pa, ok := a.(*types.Pointer); ok {
		pb, ok := b.(*types.Pointer)
		if !ok {
			return false
		}
		a, b = types.Unalias(pa.Elem()), types.Unalias(pb.Elem())
	}
	na, ok1 := a.(*types.Named)
	nb, ok2 := b.(*types.Named)
	if !ok1 || !ok2 || na.Origin() != nb.Origin() || na.TypeArgs().Len() == 0 || na.TypeArgs().Len() != nb.TypeArgs().Len() {
		return false
	}
	seen := map[types.Type]bool{}
	for i := 0; i < na.TypeArgs().Len(); i++ {
		tp, ok := types.Unalias(na.TypeArgs().At(i)).(*types.TypeParam)
		if !ok || seen[tp] {
			return false
		}
		seen[tp] = true
	}
	return true
}

// tfConvOf: what the conversions of t agree on (nil: none, or they differ).
func tfConvOf(t types.Type) *tfConv {
	tfTable()
	var all *tfConv
	for _, c := range tfConvs {
		if !tfSame(c.elem, t) {
			continue
		}
		if c.mixed {
			return nil
		}
		if all == nil {
			all = c
			continue
		}
		if c.raw != all.raw || len(c.box) != len(all.box) || len(c.unbox) != len(all.unbox) {
			return nil
		}
		for o := range c.box {
			if !all.box[o] {
				return nil
			}
		}
		for o := range c.unbox {
			if !all.unbox[o] {
				return nil
			}
		}
	}
	return all
}

// IsUnboxOf reports whether k is K(x) with K the unboxing of t.
func IsUnboxOf(k *ast.CallExpr, t types.Type) bool {
	if len(k.Args) != 1 || !types.Identical(Info.TypeOf(k), t) || !tfIsAny(Info.TypeOf(k.Args[0])) {
		return false
	}
	if tv, ok := Info.Types[k.Fun]; ok && tv.IsType() {
		return false
	}
	if c := tfConvOf(t); c != nil && len(c.unbox) == 1 {
		if o := tfBase(k.Fun); o != nil && c.unbox[o] {
			return true
		}
	}
	cv := ConvFor(t)
	name, _ := CallName(k)
	return cv.OK && name != "" && cv.IsUnbox(name)
}

// boxedAs returns e's operand when e is B(x) / x.B() with B the boxing of t and x a t.
func tfBoxed(e ast.Expr, t types.Type, raw bool) (ast.Expr, bool) {
	e = Unparen(e)
	if raw {
		if tv, ok := Info.Types[e]; ok && tv.Type != nil && !tv.IsNil() && types.Identical(tv.Type, t) {
			return e, true
		}
	}
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	var inner ast.Expr
	var fn types.Object
	isAnyConv := false
	if tv, ok := Info.Types[call.Fun]; ok && tv.IsType() {
		if !tfIsAny(tv.Type) || len(call.Args) != 1 {
			return nil, false
		}
		inner, isAnyConv = call.Args[0], true
	} else if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && Info.Selections[s] != nil {
		inner, fn = s.X, Info.Uses[s.Sel]
	} else if len(call.Args) == 1 {
		inner, fn = call.Args[0], tfBase(call.Fun)
	}
	if inner == nil || !types.Identical(Info.TypeOf(inner), t) {
		return nil, false
	}
	c := tfConvOf(t)
	if isAnyConv {
		return inner, raw || c != nil && c.raw && len(c.box) == 0
	}
	if c != nil && len(c.box) == 1 && !c.raw && fn != nil && c.box[fn] {
		return inner, true
	}
	if c == nil {
		// no conversion of the type in the package: the boxing rw.Unboxed reads
		cv := ConvFor(t)
		if id, ok := call.Fun.(*ast.Ident); ok && cv.OK && (id.Name == "obj_" || id.Name == "strobj_") {
			return inner, true
		}
		if s, ok := call.Fun.(*ast.SelectorExpr); ok && cv.OK && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
			return inner, true
		}
	}
	return nil, false
}

func tfSig(e ast.Expr) *types.Signature {
	t := Info.TypeOf(e)
	if t == nil {
		return nil
	}
	s, _ := types.Unalias(t).Underlying().(*types.Signature)
	return s
}

func tfSameSig(a, b *types.Signature) bool {
	if a == nil || b == nil || a.Variadic() || b.Variadic() || a.TypeParams().Len() > 0 {
		return false
	}
	return types.Identical(types.NewSignatureType(nil, nil, nil, a.Params(), a.Results(), false), types.NewSignatureType(nil, nil, nil, b.Params(), b.Results(), false))
}

func tfTypeText(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == Pkg {
			return ""
		}
		return p.Name()
	})
}

var tfDecls map[types.Object]*ast.FuncDecl
var tfNever = map[types.Object]int{} // 1: being decided, 2: never returns nil, 3: may

// NonNullFn reports whether the function-typed expression e cannot be nil.
func NonNullFn(e ast.Expr) bool {
	e = Unparen(e)
	switch x := e.(type) {
	case *ast.FuncLit:
		return true
	case *ast.Ident:
		_, ok := Info.Uses[x].(*types.Func)
		return ok
	case *ast.SelectorExpr:
		_, ok := Info.Uses[x.Sel].(*types.Func)
		return ok
	case *ast.IndexExpr, *ast.IndexListExpr:
		_, ok := tfBase(e).(*types.Func)
		return ok
	case *ast.CallExpr:
		if tv, ok := Info.Types[x.Fun]; ok && tv.IsType() {
			return len(x.Args) == 1 && NonNullFn(x.Args[0])
		}
		fn, ok := tfBase(x.Fun).(*types.Func)
		if !ok {
			return false
		}
		if tfDecls == nil {
			tfDecls = map[types.Object]*ast.FuncDecl{}
			for _, f := range Files {
				for _, d := range f.Decls {
					if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
						tfDecls[Info.Defs[fd.Name]] = fd
					}
				}
			}
		}
		fn = fn.Origin()
		switch tfNever[fn] {
		case 1, 3:
			return false
		case 2:
			return true
		}
		tfNever[fn] = 1
		fd := tfDecls[fn]
		ok = fd != nil && fd.Type.Results != nil && fd.Type.Results.NumFields() == 1 && len(fd.Type.Results.List[0].Names) == 0
		n := 0
		if ok {
			for _, rs := range tfReturns(fd.Body) {
				n++
				if len(rs.Results) != 1 || !NonNullFn(rs.Results[0]) {
					ok = false
				}
			}
		}
		ok = ok && n > 0
		tfNever[fn] = 3
		if ok {
			tfNever[fn] = 2
		}
		return ok
	}
	return false
}

// tfReturns: the return statements of a function body (not those of literals inside it).
func tfReturns(b *ast.BlockStmt) []*ast.ReturnStmt {
	var out []*ast.ReturnStmt
	ast.Inspect(b, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			out = append(out, x)
		}
		return true
	})
	return out
}

func tfAdapter(e ast.Expr) ast.Expr {
	u, ok := Unparen(e).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return nil
	}
	cl, ok := u.X.(*ast.CompositeLit)
	if !ok || len(cl.Elts) != 1 {
		return nil
	}
	var name string
	switch t := cl.Type.(type) {
	case *ast.Ident:
		name = t.Name
	case *ast.IndexExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			name = id.Name
		}
	case *ast.IndexListExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			name = id.Name
		}
	}
	if !strings.Contains(name, "_tfn") {
		return nil
	}
	if _, ok := cl.Elts[0].(*ast.KeyValueExpr); ok {
		return nil
	}
	return cl.Elts[0]
}

// tfClosure returns E of the translator's converting closure when it stands for a function of
// signature want.
func tfClosure(e ast.Expr, want *types.Signature) ast.Expr {
	E, _ := tfClosureX(e, want, IsUnboxOf, true)
	return E
}

// ClosureOf returns E and the calls K(p) of the translator's converting closure over a function
// value E (which may be null: the closure is then null), each K accepted by okK for the type of
// E's parameter at its position.
func ClosureOf(e ast.Expr, okK func(k *ast.CallExpr, t types.Type) bool) (ast.Expr, []*ast.CallExpr) {
	return tfClosureX(e, nil, okK, false)
}

func tfClosureX(e ast.Expr, want *types.Signature, okK func(k *ast.CallExpr, t types.Type) bool, nonNull bool) (ast.Expr, []*ast.CallExpr) {
	E, ks := tfClosure1(e, want, okK, nonNull)
	if E == nil {
		return nil, nil
	}
	return E, ks
}

func tfClosure1(e ast.Expr, want *types.Signature, okK func(k *ast.CallExpr, t types.Type) bool, nonNull bool) (ast.Expr, []*ast.CallExpr) {
	var ks []*ast.CallExpr
	call, ok := Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil, nil
	}
	fl, ok := call.Fun.(*ast.FuncLit)
	if !ok || fl.Type.Params.NumFields() != 0 || len(fl.Body.List) != 3 {
		return nil, nil
	}
	as, ok := fl.Body.List[0].(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return nil, nil
	}
	fid, ok := as.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, nil
	}
	fobj := Info.Defs[fid]
	is, ok := fl.Body.List[1].(*ast.IfStmt)
	if !ok || is.Init != nil || is.Else != nil || len(is.Body.List) != 1 {
		return nil, nil
	}
	if be, ok := Unparen(is.Cond).(*ast.BinaryExpr); !ok || be.Op != token.EQL || Text(be.X) != fid.Name || Text(be.Y) != "nil" {
		return nil, nil
	}
	if rs, ok := is.Body.List[0].(*ast.ReturnStmt); !ok || len(rs.Results) != 1 || Text(rs.Results[0]) != "nil" {
		return nil, nil
	}
	rs, ok := fl.Body.List[2].(*ast.ReturnStmt)
	if !ok || len(rs.Results) != 1 {
		return nil, nil
	}
	g, ok := Unparen(rs.Results[0]).(*ast.FuncLit)
	if !ok || len(g.Body.List) != 1 {
		return nil, nil
	}
	E := as.Rhs[0]
	if want == nil {
		want = tfSig(E)
		if want == nil || want.Variadic() {
			return nil, nil
		}
	}
	if !tfSameSig(tfSig(E), want) || nonNull && !NonNullFn(E) {
		return nil, nil
	}
	gs := tfSig(g)
	if gs == nil || gs.Params().Len() != want.Params().Len() || gs.Results().Len() != want.Results().Len() {
		return nil, nil
	}
	var inner ast.Expr
	switch st := g.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(st.Results) != 1 || want.Results().Len() != 1 {
			return nil, nil
		}
		inner = Unparen(st.Results[0])
		if !types.Identical(gs.Results().At(0).Type(), want.Results().At(0).Type()) {
			x, ok := tfBoxed(inner, want.Results().At(0).Type(), false)
			if !ok || !tfIsAny(gs.Results().At(0).Type()) {
				return nil, nil
			}
			inner = Unparen(x)
		}
	case *ast.ExprStmt:
		if want.Results().Len() != 0 {
			return nil, nil
		}
		inner = Unparen(st.X)
	default:
		return nil, nil
	}
	fc, ok := inner.(*ast.CallExpr)
	if !ok || len(fc.Args) != want.Params().Len() {
		return nil, nil
	}
	if id, ok := Unparen(fc.Fun).(*ast.Ident); !ok || Info.Uses[id] != fobj {
		return nil, nil
	}
	if len(UsesOf[fobj]) != 2 {
		return nil, nil
	}
	var params []types.Object
	for _, f := range g.Type.Params.List {
		if len(f.Names) == 0 {
			return nil, nil
		}
		for _, n := range f.Names {
			params = append(params, Info.Defs[n])
		}
	}
	for i, a := range fc.Args {
		a = Unparen(a)
		t := want.Params().At(i).Type()
		if types.Identical(gs.Params().At(i).Type(), t) {
			if id, ok := a.(*ast.Ident); !ok || Info.Uses[id] != params[i] {
				return nil, nil
			}
			continue
		}
		k, ok := a.(*ast.CallExpr)
		if !ok || len(k.Args) != 1 || !types.Identical(Info.TypeOf(k), t) || !okK(k, t) {
			return nil, nil
		}
		ks = append(ks, k)
		if id, ok := Unparen(k.Args[0]).(*ast.Ident); !ok || Info.Uses[id] != params[i] {
			return nil, nil
		}
	}
	return E, ks
}

type tfEdits map[ast.Node]func() string

func tfLine(p token.Pos) int { return Fset.Position(p).Line }

// tfRetype collects the edits that give lit the signature want.
func tfRetype(lit *ast.FuncLit, want *types.Signature, raw bool, ed tfEdits) bool {
	got := tfSig(lit)
	if got == nil || got.Variadic() || got.Params().Len() != want.Params().Len() || got.Results().Len() != want.Results().Len() {
		return false
	}
	type entry struct {
		stmt *ast.DeclStmt
		name *ast.Ident
		v    *ast.Ident
		typ  types.Type
		k    *ast.CallExpr
		fld  *ast.Field
	}
	var entries []entry
	i := 0
	for _, fld := range lit.Type.Params.List {
		n := len(fld.Names)
		if n == 0 {
			n = 1
		}
		for j := 0; j < n; j++ {
			t := want.Params().At(i).Type()
			pt := got.Params().At(i).Type()
			i++
			if types.Identical(pt, t) {
				continue
			}
			if !tfIsAny(pt) || len(fld.Names) > 1 {
				return false
			}
			txt := tfTypeText(t)
			ed[fld.Type] = func() string { return txt }
			if len(fld.Names) == 0 || fld.Names[0].Name == "_" {
				continue
			}
			name := fld.Names[0]
			uses := UsesOf[Info.Defs[name]]
			for _, u := range uses {
				k, ok := Up(u).(*ast.CallExpr)
				if !ok || len(k.Args) != 1 || Unparen(k.Args[0]) != ast.Expr(u) || !IsUnboxOf(k, t) {
					return false
				}
				pn := name.Name
				ed[k] = func() string { return pn }
			}
			if len(uses) == 1 {
				k := Up(uses[0]).(*ast.CallExpr)
				if vs, ok := Up(k).(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 1 && vs.Names[0].Name != "_" {
					if gd, ok := Parent[vs].(*ast.GenDecl); ok && len(gd.Specs) == 1 {
						if ds, ok := Parent[gd].(*ast.DeclStmt); ok && Parent[ds] == ast.Node(lit.Body) {
							if o := Info.Defs[vs.Names[0]]; o != nil && types.Identical(o.Type(), t) {
								entries = append(entries, entry{ds, name, vs.Names[0], t, k, fld})
							}
						}
					}
				}
			}
		}
	}
	// `var v T = K(p)` at the head of the body: the parameter is v. The statements must be the
	// body's first (with their `_ = v`), each on lines of its own.
	isEntry := map[ast.Stmt]*entry{}
	for i := range entries {
		isEntry[entries[i].stmt] = &entries[i]
	}
	blank := func(s ast.Stmt) string {
		as, ok := s.(*ast.AssignStmt)
		if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 || Text(as.Lhs[0]) != "_" {
			return ""
		}
		if id, ok := as.Rhs[0].(*ast.Ident); ok {
			return id.Name
		}
		return ""
	}
	ownLines := func(idx int) bool {
		l := lit.Body.List
		before, after := tfLine(lit.Body.Lbrace), tfLine(lit.Body.Rbrace)
		if idx > 0 {
			before = tfLine(l[idx-1].End())
		}
		if idx+1 < len(l) {
			after = tfLine(l[idx+1].Pos())
		}
		return before < tfLine(l[idx].Pos()) && tfLine(l[idx].End()) < after
	}
	declared := map[string]bool{}
	for idx, s := range lit.Body.List {
		if e := isEntry[s]; e != nil && ownLines(idx) {
			declared[e.v.Name] = true
			v := e.v.Name
			delete(ed, e.k)
			ed[e.name] = func() string { return v }
			ed[s] = func() string { return Del }
			continue
		}
		if b := blank(s); b != "" && declared[b] && ownLines(idx) {
			ed[s] = func() string { return Del }
			continue
		}
		break
	}
	if want.Results().Len() == 1 && !types.Identical(got.Results().At(0).Type(), want.Results().At(0).Type()) {
		return tfResult(lit, want.Results().At(0).Type(), raw, ed)
	}
	return true
}

// tfResult: lit's result (any) becomes r.
func tfResult(lit *ast.FuncLit, r types.Type, raw bool, ed tfEdits) bool {
	if lit.Type.Results == nil || len(lit.Type.Results.List) != 1 {
		return false
	}
	fld := lit.Type.Results.List[0]
	if !tfIsAny(Info.TypeOf(fld.Type)) || len(fld.Names) > 1 {
		return false
	}
	if len(fld.Names) == 1 && len(UsesOf[Info.Defs[fld.Names[0]]]) > 0 {
		return false
	}
	txt := tfTypeText(r)
	ed[fld.Type] = func() string { return txt }
	rets := tfReturns(lit.Body)
	if len(rets) == 0 {
		return false
	}
	for _, rs := range rets {
		if len(rs.Results) != 1 {
			return false
		}
		e := Unparen(rs.Results[0])
		if tv, ok := Info.Types[e]; ok && tv.IsNil() {
			switch r.Underlying().(type) {
			case *types.Pointer, *types.Interface:
				continue
			case *types.Slice, *types.Map:
				if raw {
					continue
				}
			}
			return false
		}
		if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 0 {
			if fl, ok := c.Fun.(*ast.FuncLit); ok && fl.Type.Params.NumFields() == 0 {
				if s := tfSig(fl); s != nil && s.Results().Len() == 1 && tfIsAny(s.Results().At(0).Type()) {
					if !tfResult(fl, r, raw, ed) {
						return false
					}
					continue
				}
			}
		}
		inner, ok := tfBoxed(e, r, raw)
		if !ok {
			return false
		}
		if inner != e {
			in, at := inner, rs.Results[0]
			ed[at] = func() string { return Render(in) }
		}
	}
	return true
}

// TypedFn: the code of the function of signature want that arg stands for (see the head of the
// file); false when arg has none of the forms.
func TypedFn(arg ast.Expr, want *types.Signature, raw bool) (func() string, bool) {
	arg = Unparen(arg)
	if f := tfAdapter(arg); f != nil {
		if !tfSameSig(tfSig(f), want) {
			return nil, false
		}
		return func() string { return Render(f) }, true
	}
	if E := tfClosure(arg, want); E != nil {
		return func() string { return Render(E) }, true
	}
	if c, ok := arg.(*ast.CallExpr); ok && len(c.Args) == 1 {
		if tv, ok := Info.Types[c.Fun]; ok && tv.IsType() {
			if _, isFn := tv.Type.Underlying().(*types.Signature); isFn {
				arg = Unparen(c.Args[0])
			}
		}
	}
	lit, ok := arg.(*ast.FuncLit)
	if !ok {
		return nil, false
	}
	if tfSameSig(tfSig(lit), want) {
		return func() string { return Render(lit) }, true
	}
	ed := tfEdits{}
	if !tfRetype(lit, want, raw, ed) {
		return nil, false
	}
	return func() string {
		for n, g := range ed {
			if _, dup := Rewrite[n]; dup {
				panic("TypedFn: a node of the literal is rewritten twice: " + Text(n))
			}
			Rewrite[n] = g
		}
		s := Render(lit)
		for n := range ed {
			delete(Rewrite, n)
		}
		return s
	}, true
}
