// Command trips is a post-flip step: a typed collection that goes through its erased form and
// straight back, inside one expression. The typed-collection steps (colls, slices, immbuild,
// strnative) each convert at the boundary of what they typed; where two of them meet, the
// generated code builds the erased view or the erased immutable twin of a typed collection
// (objects of their own, kept on the collection), or a list of boxed elements, only for the
// next call to unbox it again. With L a *TList[T], S and U *TSet[K], a a []T, P a *[]T, SP a
// *Splitter and tc the element conversion of T:
//
//	sliceImm_(tc, L.asImm())                               -> tlistSlice_(tc, L)
//	sliceImm_(tc, ImmutableList_copyOf_ObjectArr(arrmap_(a, (*X).self)))    (T = *X)
//	                                                       -> immCopyArr_(tc, a)
//	immutableListCopyOf_(tc, L.asList())                   -> tlistImmCopy_(tc, L)
//	immCopyOf_(tc, sliceViewP_(tc, P))                     -> immCopySlice_(tc, *P)
//	sliceAddAll_(tc, P, sliceView_(tc, a))                 -> sliceAddSlice_(P, a)
//	sliceAddAll_(tc, P, L.asImm() or L.asList())           -> sliceAddList_(P, L)
//	tsetFrom_(tc, S.asSet(), false)                        -> tsetCopy_(tc, S)
//	S.asSet().addAll(U.asSet() or U.asImm())               -> tsetAddSet_(S, U)
//	S.asSet().addAll(L.asImm() or L.asList())              -> tsetAddList_(S, L)
//	S.asSet().addAll(c)                    any other c     -> tsetAddAllOf_(S, c)
//	sliceImm_(tc_string, SP.splitToList_s(x))              -> SP.splitStrs_(x)
//	immCopyOf_(tc_string, Iterables_skip(SP.split_s(x), n)) -> sliceSkip_(SP.splitStrs_(x), n)
//
// (the erased operand may be wrapped in Collection(..) or iup_Collection(..)). The helpers
// (rt_timm.go, rt_native.go, rt_base_text.go) do what the pair of conversions did to the typed
// collection and give the same elements in the same order, the same null and the same
// exceptions; rt_timm_test.go (TestTypedRoundTrips) compares each with the pair it replaces.
// Where the shortcut is not what the conversions do (a list whose conversion object is not
// tc, or whose twin has another typed copy) the helper performs the conversions. What is no
// longer made is the view, the twin or the boxed list, which nothing else in the expression
// can see.
//
// Why the program cannot tell: each conversion is right for a collection that really leaves for
// erased code; that the erased value here is consumed at once by the inverse conversion is only
// visible to a step that looks at both calls. The step checks the types (the typed operand is a
// *TList / *TSet / slice of the element type of the conversion it meets, the splitter a
// *Splitter; SaveChecked type-checks the result, whose helpers take both sides at one element
// type), refuses to run if the runtime lacks a helper, and leaves every other shape.
//
// usage: trips [-n] <dir of package closure>
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

func named(t types.Type) *types.Named {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	n, _ := types.Unalias(p.Elem()).(*types.Named)
	if n == nil || n.Obj().Pkg() != rw.Pkg {
		return nil
	}
	return n
}

func typeOf(e ast.Expr) types.Type {
	if tv, ok := rw.Info.Types[e]; ok && tv.Type != nil {
		return tv.Type
	}
	return types.Typ[types.Invalid]
}

// typedOf: e's type as *TList[T] / *TSet[K] (through the ImmutableList / ImmutableSet aliases):
// the generic type's name and its element type.
func typedOf(e ast.Expr) (string, types.Type) {
	n := named(typeOf(e))
	if n == nil || n.TypeArgs().Len() != 1 {
		return "", nil
	}
	return n.Obj().Name(), n.TypeArgs().At(0)
}

// convOf: the element type T of a *tconv_[T] expression.
func convOf(e ast.Expr) types.Type {
	n := named(typeOf(e))
	if n == nil || n.Obj().Name() != "tconv_" || n.TypeArgs().Len() != 1 {
		return nil
	}
	return n.TypeArgs().At(0)
}

// method: e as a call X.name(args) of a method; X and the call.
func method(e ast.Expr, name string, nargs int) (ast.Expr, *ast.CallExpr) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != nargs {
		return nil, nil
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != name {
		return nil, nil
	}
	if sel, ok := rw.Info.Selections[s]; !ok || sel.Kind() != types.MethodVal {
		return nil, nil
	}
	return s.X, c
}

// fn: e as a call name(args..) of that package function; its arguments.
func fn(e ast.Expr, name string, nargs int) []ast.Expr {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != nargs {
		return nil
	}
	if n, _ := rw.CallName(c); n != name {
		return nil
	}
	return c.Args
}

// erased: e without a conversion to Collection around it (Collection(x), iup_Collection(x)).
func erased(e ast.Expr) ast.Expr {
	e = rw.Unparen(e)
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 1 {
		return e
	}
	switch f := c.Fun.(type) {
	case *ast.Ident:
		if f.Name == "iup_Collection" {
			return reflip42un(c.Args[0])
		}
		if reflip42un(e) != e {
			return reflip42un(e)
		}
		if tn, ok := rw.Info.Uses[f].(*types.TypeName); ok && tn.Name() == "Collection" && tn.Pkg() == rw.Pkg {
			return reflip42un(c.Args[0])
		}
	}
	return e
}

// listOf: e as L.asImm() or L.asList() of a typed list of T; L.
func listOf(e ast.Expr, t types.Type) ast.Expr {
	for _, m := range []string{"asImm", "asList", "asListT", "asCollT", "asIterable"} { // reflip42
		if l, _ := method(e, m, 0); l != nil {
			if g, el := typedOf(l); g == "TList" && types.Identical(el, t) {
				return l
			}
		}
	}
	return nil
}

// setOf: e as U.asSet() or U.asImm() of a typed set of K; U.
func setOf(e ast.Expr, k types.Type) ast.Expr {
	for _, m := range []string{"asSet", "asImm", "asSetT", "asCollT", "asIterable"} { // reflip42
		if u, _ := method(e, m, 0); u != nil {
			if g, el := typedOf(u); g == "TSet" && types.Identical(el, k) {
				return u
			}
		}
	}
	return nil
}

func isSplitter(e ast.Expr) bool {
	n := named(typeOf(e))
	return n != nil && n.Obj().Name() == "Splitter"
}

func isString(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Kind() == types.String
}

func sliceElem(e ast.Expr) types.Type {
	if s, ok := types.Unalias(typeOf(e)).(*types.Slice); ok {
		return s.Elem()
	}
	return nil
}

func ptrSliceElem(e ast.Expr) types.Type {
	if p, ok := types.Unalias(typeOf(e)).(*types.Pointer); ok {
		if s, ok := types.Unalias(p.Elem()).(*types.Slice); ok {
			return s.Elem()
		}
	}
	return nil
}

func same(a, b types.Type) bool { return a != nil && b != nil && types.Identical(a, b) }

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	for _, h := range []string{"tlistSlice_", "immCopyArr_", "tlistImmCopy_", "immCopySlice_", "sliceAddSlice_", "sliceAddList_", "tsetCopy_", "tsetAddSet_", "tsetAddList_", "tsetAddAllOf_", "sliceSkip_",
		"sliceImm_", "tsetFrom_", "arrmap_", "ImmutableList_copyOf_ObjectArr", "immutableListCopyOf_", "immCopyOf_", "sliceViewP_", "sliceView_", "sliceAddAll_", "Iterables_skip"} {
		if !rw.HasFunc(h) {
			if h == "ImmutableList_copyOf_ObjectArr" { // reflip38: a function read in a pattern, not written
				fmt.Fprintf(os.Stderr, "trips: the runtime has no %s: the round trips through it have no subject\n", h)
				continue
			}
			fmt.Fprintf(os.Stderr, "trips: the runtime has no %s: nothing done\n", h)
			os.Exit(1)
		}
	}
	if sp := rw.Pkg.Scope().Lookup("Splitter"); sp == nil || !rw.HasMethod(types.NewPointer(sp.Type()), "splitStrs_") {
		fmt.Fprintln(os.Stderr, "trips: the runtime has no (*Splitter).splitStrs_: nothing done")
		os.Exit(1)
	}
	done := map[string]int{}
	left := map[string]int{}
	r := rw.Render
	put := func(c *ast.CallExpr, what string, text func() string) bool {
		done[what]++
		rw.Rewrite[c] = text
		return false
	}
	for _, f := range rw.Files {
		name := rw.Fset.Position(f.Pos()).Filename
		if !strings.HasPrefix(name[strings.LastIndex(name, "/")+1:], "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			cn, _ := rw.CallName(c)
			switch {
			case cn == "sliceImm_" && len(c.Args) == 2:
				tc := c.Args[0]
				t := convOf(tc)
				if t == nil {
					return true
				}
				in := erased(c.Args[1])
				if l, _ := method(in, "asImm", 0); l != nil {
					if g, el := typedOf(l); g == "TList" && same(el, t) {
						return put(c, "sliceImm_(tc, L.asImm()) -> tlistSlice_(tc, L)", func() string { return "tlistSlice_(" + r(tc) + ", " + r(l) + ")" })
					}
					left["sliceImm_ of asImm() of something else than a typed list of the element type"]++
					return true
				}
				if sp, sc := method(in, "splitToList_s", 1); sp != nil && isSplitter(sp) && isString(t) {
					x := sc.Args[0]
					return put(c, "sliceImm_(tc_string, SP.splitToList_s(x)) -> SP.splitStrs_(x)", func() string { return r(sp) + ".splitStrs_(" + r(x) + ")" })
				}
				if cp := fn(in, "ImmutableList_copyOf_ObjectArr", 1); cp != nil {
					am := fn(cp[0], "arrmap_", 2)
					if am == nil {
						return true
					}
					a, box := am[0], rw.Unparen(am[1])
					// box is (*X).self and a a []*X with T = *X: the array's own elements, boxed.
					sel, ok := box.(*ast.SelectorExpr)
					if ok && sel.Sel.Name == "self" && same(sliceElem(a), t) && rw.Info.Types[sel.X].IsType() && same(rw.Info.Types[sel.X].Type, t) {
						return put(c, "sliceImm_(tc, ImmutableList_copyOf_ObjectArr(arrmap_(a, (*X).self))) -> immCopyArr_(tc, a)", func() string { return "immCopyArr_(" + r(tc) + ", " + r(a) + ")" })
					}
					left["sliceImm_ of a copy of an array boxed otherwise than by (*X).self"]++
				}
			case cn == "immutableListCopyOf_" && len(c.Args) == 2:
				tc := c.Args[0]
				if t := convOf(tc); t != nil {
					if l, _ := method(erased(c.Args[1]), "asList", 0); l != nil {
						if g, el := typedOf(l); g == "TList" && same(el, t) {
							return put(c, "immutableListCopyOf_(tc, L.asList()) -> tlistImmCopy_(tc, L)", func() string { return "tlistImmCopy_(" + r(tc) + ", " + r(l) + ")" })
						}
					}
				}
			case cn == "immCopyOf_" && len(c.Args) == 2:
				tc := c.Args[0]
				t := convOf(tc)
				if t == nil {
					return true
				}
				in := erased(c.Args[1])
				if v := fn(in, "sliceViewP_", 2); v != nil && same(convOf(v[0]), t) && same(ptrSliceElem(v[1]), t) {
					p := v[1]
					return put(c, "immCopyOf_(tc, sliceViewP_(tc, P)) -> immCopySlice_(tc, *P)", func() string {
						if _, ok := rw.Unparen(p).(*ast.Ident); ok {
							return "immCopySlice_(" + r(tc) + ", *" + r(p) + ")"
						}
						return "immCopySlice_(" + r(tc) + ", *(" + r(p) + "))"
					})
				}
				if sk := fn(in, "Iterables_skip", 2); sk != nil && isString(t) {
					if sp, sc := method(sk[0], "split_s", 1); sp != nil && isSplitter(sp) {
						x, cnt := sc.Args[0], sk[1]
						return put(c, "immCopyOf_(tc_string, Iterables_skip(SP.split_s(x), n)) -> sliceSkip_(SP.splitStrs_(x), n)", func() string {
							return "sliceSkip_(" + r(sp) + ".splitStrs_(" + r(x) + "), " + r(cnt) + ")"
						})
					}
				}
			case cn == "sliceAddAll_" && len(c.Args) == 3:
				t := convOf(c.Args[0])
				p := c.Args[1]
				if t == nil || !same(ptrSliceElem(p), t) {
					return true
				}
				in := erased(c.Args[2])
				if v := fn(in, "sliceView_", 2); v != nil && same(convOf(v[0]), t) && same(sliceElem(v[1]), t) {
					a := v[1]
					return put(c, "sliceAddAll_(tc, P, sliceView_(tc, a)) -> sliceAddSlice_(P, a)", func() string { return "sliceAddSlice_(" + r(p) + ", " + r(a) + ")" })
				}
				if l := listOf(in, t); l != nil {
					return put(c, "sliceAddAll_(tc, P, the view or twin of a typed list L) -> sliceAddList_(P, L)", func() string { return "sliceAddList_(" + r(p) + ", " + r(l) + ")" })
				}
			case cn == "tsetFrom_" && len(c.Args) == 3:
				tc := c.Args[0]
				t := convOf(tc)
				if id, ok := c.Args[2].(*ast.Ident); !ok || id.Name != "false" || rw.Info.Uses[id] != types.Universe.Lookup("false") || t == nil {
					return true
				}
				if s, _ := method(erased(c.Args[1]), "asSet", 0); s != nil {
					if g, el := typedOf(s); g == "TSet" && same(el, t) {
						return put(c, "tsetFrom_(tc, S.asSet(), false) -> tsetCopy_(tc, S)", func() string { return "tsetCopy_(" + r(tc) + ", " + r(s) + ")" })
					}
				}
			default:
				// S.asSet().addAll(c)
				fs, ok := c.Fun.(*ast.SelectorExpr)
				if !ok || fs.Sel.Name != "addAll" || len(c.Args) != 1 {
					return true
				}
				s, _ := method(fs.X, "asSet", 0)
				T := "" // reflip42: S.asSetT().addAll / S.asCollT().addAll: the helpers of the typed view
				for _, m := range []string{"asSetT", "asCollT"} {
					if s == nil && rw.HasFunc("tsetAddAllOfT_") {
						s, _ = method(fs.X, m, 0)
						T = "T"
					}
				}
				if s == nil {
					return true
				}
				g, k := typedOf(s)
				if g != "TSet" {
					return true
				}
				arg := c.Args[0]
				in := erased(arg)
				if u := setOf(in, k); u != nil {
					return put(c, "S.asSet"+T+"().addAll(the view or twin of a typed set U) -> tsetAddSet"+T+"_(S, U)", func() string { return "tsetAddSet" + T + "_(" + r(s) + ", " + r(u) + ")" })
				}
				if l := listOf(in, k); l != nil {
					return put(c, "S.asSet"+T+"().addAll(the view or twin of a typed list L) -> tsetAddList"+T+"_(S, L)", func() string { return "tsetAddList" + T + "_(" + r(s) + ", " + r(l) + ")" })
				}
				return put(c, "S.asSet"+T+"().addAll(c) -> tsetAddAllOf"+T+"_(S, c)", func() string { return "tsetAddAllOf" + T + "_(" + r(s) + ", " + r(arg) + ")" })
			}
			return true
		})
	}
	total := 0
	for i, m := range []map[string]int{done, left} {
		keys := []string{}
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			if i == 0 {
				total += m[k]
			}
			fmt.Fprintf(os.Stderr, "  %s %4d %s\n", []string{"done:", "left:"}[i], m[k], k)
		}
	}
	if report {
		fmt.Fprintf(os.Stderr, "trips: %d round trips through an erased collection rewritten\n", total)
		return
	}
	fmt.Fprintf(os.Stderr, "trips: %d round trips through an erased collection rewritten; files changed: %d\n", total, rw.SaveChecked())
}

// reflip42un: e without X_un_(..) around it (List_un_, Collection_un_, Set_un_, Iterable_un_ of the package).
func reflip42un(e ast.Expr) ast.Expr {
	e = rw.Unparen(e)
	for {
		c, ok := e.(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			return e
		}
		f, ok := c.Fun.(*ast.Ident)
		if !ok || f.Name != "List_un_" && f.Name != "Collection_un_" && f.Name != "Set_un_" && f.Name != "Iterable_un_" {
			return e
		}
		if fo, ok := rw.Info.Uses[f].(*types.Func); !ok || fo.Pkg() != rw.Pkg {
			return e
		}
		e = rw.Unparen(c.Args[0])
	}
}
