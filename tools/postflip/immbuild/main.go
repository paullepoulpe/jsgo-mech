// Command immbuild is a post-flip step: an ImmutableList that is made only to be copied into a
// typed slice (sliceImm_, step native -kind imm) is built as that slice directly.
//
// After step native -kind imm an ImmutableList<T> slot is a []T, and a list from the erased
// runtime (a builder, ImmutableList.copyOf) is copied into it by sliceImm_: the list, its
// []any, a typed copy and its cache are four objects for one slice. Here:
//
//   - A local builder (ImmutableList.builder(), new ImmutableList.Builder<>(),
//     builderWithExpectedSize(int32(len(x)))) whose every use is add(x) or addAll(it) as a
//     statement, or sliceImm_(tc, b.build()) with tc's element type the builder's, becomes the
//     []T it builds: b.add(x) is b = immAdd_(tc, b, x) (x unboxed where the code boxed it;
//     immAddObj_ otherwise), b.addAll(it) is b = immAddAll_(tc, b, it), and
//     sliceImm_(tc, b.build()) is immBuilt_(b). A null element throws NullPointerException
//     "at index i" as the runtime builder does; build() is the slice so far with its capacity
//     cut, so later adds never reach a list already built, and an empty build is an empty
//     slice, not nil (Java's null).
//   - sliceImm_(tc, ImmutableList_copyOf(x)) and copyOf_Iterable(x) are immCopyOf_(tc, x): an
//     immutable list is converted as before (sliceImm_ of its asList()); any other collection
//     is copied once, with the same null check.
//     A copy of a slice seen as a list (sliceView_) is immCopySlice_ (null throws as before).
//   - sliceImm_(tc, ImmutableList_copyOf_ObjectArr(arrmap_(a, box))) of an array a of the
//     element type is immCopyArr_(tc, a) (the elements, checked for null, copied: the array
//     can change later; an empty or null array is an empty list, as before).
//   - sliceImm_(tc, ImmutableList_of(a, b...)) is sliceOf_(tc, a, b...) with the arguments
//     unboxed (sliceOf_ rejects a null as Guava does), ImmutableList_of() is []T{}.
//
// A builder that is a field, a parameter, captured by a function literal, passed on, chained
// (b.add(x).add(y)) or built for anything but sliceImm_ stays.
//
// usage: immbuild [-v] <dir of package closure>
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

var verbose = flag.Bool("v", false, "list the builders that stay, with the reason")

// elemOf is T of a *tconv_[T] expression, or nil.
func elemOf(e ast.Expr) types.Type {
	tv, ok := rw.Info.Types[e]
	if !ok {
		return nil
	}
	p, ok := types.Unalias(tv.Type).(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.Obj().Name() != "tconv_" || n.TypeArgs().Len() != 1 {
		return nil
	}
	return n.TypeArgs().At(0)
}

// builderCall is the call b.m(...) of which id (b) is the receiver, or nil.
func builderCall(id *ast.Ident) (string, *ast.CallExpr) {
	sel, ok := rw.Parent[id].(*ast.SelectorExpr)
	if !ok || sel.X != id {
		return "", nil
	}
	call, ok := rw.Parent[sel].(*ast.CallExpr)
	if !ok || call.Fun != sel {
		return "", nil
	}
	return sel.Sel.Name, call
}

// stmtOf is the expression statement whose expression is call, or nil.
func stmtOf(call *ast.CallExpr) *ast.ExprStmt {
	s, _ := rw.Parent[call].(*ast.ExprStmt)
	return s
}

// sliceImmOf is the sliceImm_(tc, e) call of which e is the second argument, or nil.
func sliceImmOf(e ast.Expr) *ast.CallExpr {
	p, ok := rw.Up(e).(*ast.CallExpr)
	if !ok || len(p.Args) != 2 || rw.Unparen(p.Args[1]) != e {
		return nil
	}
	if name, _ := rw.CallName(p); name != "sliceImm_" {
		return nil
	}
	return p
}

// funcOf is the innermost function (declaration or literal) holding n.
func funcOf(n ast.Node) ast.Node {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch p.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return p
		}
	}
	return nil
}

func contains(outer, inner ast.Node) bool {
	return outer.Pos() <= inner.Pos() && inner.End() <= outer.End()
}

// typeText is the source of type t in package closure.
func typeText(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string { return "" })
}

// unboxed is the argument x of b.add(x) as a T, if the code boxed a T (obj_, strobj_, any,
// self, box_), else "".
func unboxed(t types.Type, x ast.Expr) string {
	cv := rw.ConvFor(t)
	if !cv.OK {
		return ""
	}
	s := rw.Unboxed(cv, t, x)
	if s == cv.Unbox(rw.Render(rw.Unparen(x))) {
		return ""
	}
	return s
}

type builder struct {
	v      *types.Var
	decl   ast.Node // the *ast.DeclStmt or *ast.AssignStmt
	init   ast.Expr
	elem   types.Type
	tc     string // the element conversion named at its sliceImm_ calls
	adds   []*ast.CallExpr
	alls   []*ast.CallExpr
	builds []*ast.CallExpr // the sliceImm_ calls
}

func main() {
	flag.Parse()
	rw.Load(flag.Arg(0))

	// Local builders: declared with an initializer that makes an empty builder.
	var builders []*builder
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			var id *ast.Ident
			var init ast.Expr
			switch s := n.(type) {
			case *ast.DeclStmt:
				g := s.Decl.(*ast.GenDecl)
				if g.Tok != token.VAR || len(g.Specs) != 1 {
					return true
				}
				vs := g.Specs[0].(*ast.ValueSpec)
				if len(vs.Names) != 1 || len(vs.Values) != 1 {
					return true
				}
				id, init = vs.Names[0], vs.Values[0]
			case *ast.AssignStmt:
				if s.Tok != token.DEFINE || len(s.Lhs) != 1 || len(s.Rhs) != 1 {
					return true
				}
				id, _ = s.Lhs[0].(*ast.Ident)
				init = s.Rhs[0]
			default:
				return true
			}
			if id == nil || !emptyBuilder(init) {
				return true
			}
			v, ok := rw.Info.Defs[id].(*types.Var)
			if !ok {
				return true
			}
			builders = append(builders, &builder{v: v, decl: n, init: init})
			return true
		})
	}

	stay := map[string]int{}
	var stayAt []string
	reject := func(b *builder, why string) {
		stay[why]++
		p := rw.Fset.Position(b.v.Pos())
		stayAt = append(stayAt, fmt.Sprintf("  %s: %s  [%s:%d]", b.v.Name(), why, filepath.Base(p.Filename), p.Line))
	}
	nb, nAdd, nAddObj, nAll, nBuild := 0, 0, 0, 0, 0
	declFunc := map[*builder]ast.Node{}
next:
	for _, b := range builders {
		declFunc[b] = funcOf(b.decl)
		for _, use := range rw.UsesOf[b.v] {
			if funcOf(use) != declFunc[b] {
				reject(b, "captured by a function literal")
				continue next
			}
			m, call := builderCall(use)
			switch {
			case call == nil:
				reject(b, "used other than as a receiver")
				continue next
			case (m == "add" || m == "addAll") && len(call.Args) == 1 && stmtOf(call) != nil:
				for _, u := range rw.UsesOf[b.v] {
					if contains(call.Args[0], u) {
						reject(b, "used in its own add")
						continue next
					}
				}
				if m == "add" {
					b.adds = append(b.adds, call)
				} else {
					b.alls = append(b.alls, call)
				}
			case m == "build" && len(call.Args) == 0:
				si := sliceImmOf(call)
				if si == nil {
					reject(b, "built for other than sliceImm_")
					continue next
				}
				t := elemOf(si.Args[0])
				if t == nil {
					reject(b, "sliceImm_ with an unknown conversion")
					continue next
				}
				if b.elem != nil && !types.Identical(b.elem, t) {
					reject(b, "built into slices of two element types")
					continue next
				}
				b.elem, b.tc = t, rw.Render(si.Args[0])
				b.builds = append(b.builds, si)
			default:
				reject(b, "calls "+m+" other than as a statement")
				continue next
			}
		}
		if len(b.builds) == 0 {
			reject(b, "never built")
			continue
		}
		// The declared type argument, where there is one, must be the element type.
		if ds, ok := b.decl.(*ast.DeclStmt); ok {
			vs := ds.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec)
			if vs.Type != nil {
				args := rw.AliasArgs(vs.Type, "ImmutableList_BuilderOf")
				if len(args) != 1 || !types.Identical(rw.Info.Types[args[0]].Type, b.elem) {
					reject(b, "declared with another element type")
					continue
				}
			}
		}
		b := b
		nb++
		tc, et := b.tc, typeText(b.elem)
		if ds, ok := b.decl.(*ast.DeclStmt); ok {
			vs := ds.Decl.(*ast.GenDecl).Specs[0].(*ast.ValueSpec)
			if vs.Type != nil {
				et = rw.Text(rw.AliasArgs(vs.Type, "ImmutableList_BuilderOf")[0])
			}
		}
		name := b.v.Name()
		rw.Rewrite[b.decl] = func() string {
			init := ""
			if n := expectedSize(b.init); n != nil {
				init = " = make([]" + et + ", 0, " + rw.Render(n) + ")"
			}
			if _, ok := b.decl.(*ast.AssignStmt); ok {
				if init == "" {
					return "var " + name + " []" + et
				}
				return name + " := make([]" + et + ", 0, " + rw.Render(expectedSize(b.init)) + ")"
			}
			return "var " + name + " []" + et + init
		}
		for _, c := range b.adds {
			c := c
			if s := unboxed(b.elem, c.Args[0]); s != "" {
				nAdd++
				rw.Rewrite[stmtOf(c)] = func() string {
					return name + " = immAdd_(" + tc + ", " + name + ", " + rw.Unboxed(rw.ConvFor(b.elem), b.elem, c.Args[0]) + ")"
				}
			} else {
				nAddObj++
				rw.Rewrite[stmtOf(c)] = func() string {
					return name + " = immAddObj_(" + tc + ", " + name + ", " + rw.Render(c.Args[0]) + ")"
				}
			}
		}
		for _, c := range b.alls {
			c := c
			nAll++
			rw.Rewrite[stmtOf(c)] = func() string {
				return name + " = immAddAll_(" + tc + ", " + name + ", " + rw.Render(c.Args[0]) + ")"
			}
		}
		for _, si := range b.builds {
			nBuild++
			rw.Rewrite[si] = func() string { return "immBuilt_(" + name + ")" }
		}
	}

	// sliceImm_ of copyOf and of(...) made for it.
	nCopy, nArr, nOf := 0, 0, 0
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			si, ok := n.(*ast.CallExpr)
			if !ok || len(si.Args) != 2 {
				return true
			}
			if name, _ := rw.CallName(si); name != "sliceImm_" {
				return true
			}
			if _, done := rw.Rewrite[si]; done {
				return true
			}
			t := elemOf(si.Args[0])
			name, inner := rw.CallName(si.Args[1])
			if t == nil || inner == nil {
				return true
			}
			tc := si.Args[0]
			switch {
			case (name == "ImmutableList_copyOf" || name == "ImmutableList_copyOf_Iterable") && len(inner.Args) == 1:
				nCopy++
				if v := viewed(t, inner.Args[0]); v != nil {
					rw.Rewrite[si] = func() string { return "immCopySlice_(" + rw.Render(tc) + ", " + rw.Render(v) + ")" }
					break
				}
				rw.Rewrite[si] = func() string {
					return "immCopyOf_(" + rw.Render(tc) + ", " + rw.Render(inner.Args[0]) + ")"
				}
			case name == "ImmutableList_copyOf_ObjectArr" && len(inner.Args) == 1:
				if a := boxedArray(t, inner.Args[0]); a != nil {
					nArr++
					rw.Rewrite[si] = func() string { return "immCopyArr_(" + rw.Render(tc) + ", " + rw.Render(a) + ")" }
				}
			case name == "ImmutableList_of" && len(inner.Args) == 0:
				nOf++
				rw.Rewrite[si] = func() string { return "[]" + typeText(t) + "{}" }
			case strings.HasPrefix(name, "ImmutableList_of_Object") && !strings.HasSuffix(name, "Arr"):
				cv := rw.ConvFor(t)
				if !cv.OK {
					return true
				}
				nOf++
				rw.Rewrite[si] = func() string {
					parts := []string{rw.Render(tc)}
					for _, a := range inner.Args {
						parts = append(parts, rw.Unboxed(cv, t, a))
					}
					return "sliceOf_(" + strings.Join(parts, ", ") + ")"
				}
			}
			return true
		})
	}

	files := rw.Save()
	fmt.Printf("immbuild: %d local builders become slices (%d add typed, %d add boxed, %d addAll, %d build); sliceImm_ of copyOf %d, of an array %d, of(...) %d; files changed: %d\n",
		nb, nAdd, nAddObj, nAll, nBuild, nCopy, nArr, nOf, files)
	var whys []string
	for w := range stay {
		whys = append(whys, w)
	}
	sort.Strings(whys)
	for _, w := range whys {
		fmt.Printf("  stays (%d builders): %s\n", stay[w], w)
	}
	if *verbose {
		sort.Strings(stayAt)
		fmt.Println(strings.Join(stayAt, "\n"))
	}
}

// emptyBuilder reports whether e makes a new empty ImmutableList builder.
func emptyBuilder(e ast.Expr) bool {
	name, call := rw.CallName(e)
	switch name {
	case "ImmutableList_builder", "new_ImmutableList_Builder":
		return len(call.Args) == 0
	}
	return expectedSize(e) != nil
}

// expectedSize is n of ImmutableList_builderWithExpectedSize(int32(len(x))) (never negative,
// so Guava's check of it cannot fail), or nil.
func expectedSize(e ast.Expr) ast.Expr {
	name, call := rw.CallName(e)
	if name != "ImmutableList_builderWithExpectedSize" || len(call.Args) != 1 {
		return nil
	}
	if n, c := rw.CallName(call.Args[0]); n == "int32" && len(c.Args) == 1 {
		if l, _ := rw.CallName(c.Args[0]); l == "len" {
			return call.Args[0]
		}
	}
	return nil
}

// boxedArray is a of arrmap_(a, func(v_ T) any { return box(v_) }) where a is a []T and box
// boxes a T (obj_, strobj_, any, self, box_), or nil.
func boxedArray(t types.Type, e ast.Expr) ast.Expr {
	name, call := rw.CallName(e)
	if name != "arrmap_" || len(call.Args) != 2 {
		return nil
	}
	at, ok := types.Unalias(rw.Info.Types[call.Args[0]].Type).(*types.Slice)
	if !ok || !types.Identical(at.Elem(), t) {
		return nil
	}
	fl, ok := call.Args[1].(*ast.FuncLit)
	if !ok || len(fl.Type.Params.List) != 1 || len(fl.Type.Params.List[0].Names) != 1 || len(fl.Body.List) != 1 {
		return nil
	}
	ret, ok := fl.Body.List[0].(*ast.ReturnStmt)
	if !ok || len(ret.Results) != 1 {
		return nil
	}
	p := rw.Info.Defs[fl.Type.Params.List[0].Names[0]]
	var inner ast.Expr
	if c, ok := rw.Unparen(ret.Results[0]).(*ast.CallExpr); ok {
		if id, ok := c.Fun.(*ast.Ident); ok && len(c.Args) == 1 && (id.Name == "obj_" || id.Name == "strobj_" || id.Name == "any") {
			inner = c.Args[0]
		} else if s, ok := c.Fun.(*ast.SelectorExpr); ok && len(c.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
			inner = s.X
		}
	}
	if id, ok := inner.(*ast.Ident); !ok || rw.Info.Uses[id] != p {
		return nil
	}
	return call.Args[0]
}

// viewed is s of [Collection(|Iterable(|icast_Collection(]sliceView_(tc, s)) where s is a []T,
// or nil.
func viewed(t types.Type, e ast.Expr) ast.Expr {
	name, call := rw.CallName(e)
	if (name == "Collection" || name == "Iterable" || name == "icast_Collection") && len(call.Args) == 1 {
		name, call = rw.CallName(call.Args[0])
	}
	if name != "sliceView_" || len(call.Args) != 2 {
		return nil
	}
	st, ok := types.Unalias(rw.Info.Types[call.Args[1]].Type).(*types.Slice)
	if !ok || !types.Identical(st.Elem(), t) {
		return nil
	}
	return call.Args[1]
}
