// Command iters is a post-flip step: an explicit Iterator over an immutable collection becomes
// a cursor of the runtime's own type, *immCursor_ (rt_iter.go), instead of an Iterator
// interface value holding a heap-allocated iterator object:
//
//	var ps IteratorOf[*FunctionType_Parameter] = f.getParameters().iterator()
//	->  var ps *immCursor_ = immCursorOf_(f.getParameters())
//
// Iterators that the translated code drives by hand are the ones a for-each loop cannot
// express: two sequences walked in step (parameters against arguments), an iterator advanced
// before a loop or inside another loop, an iterator passed to a helper that continues it. Their
// variables and parameters exchange iterators, so they are joined into classes (an assignment,
// a declaration's value, an argument passed to a parameter of a function or method declared in
// generated code). A class becomes *immCursor_ when
//   - every iterator that enters it is C.iterator() with C of static type *ImmutableCollection
//     (an ImmutableList, ImmutableSet...), nil, or another slot of the class, and
//   - every use of its slots is hasNext(), next() or remove() on it, or passing it on within
//     the class; a parameter's function is only called (not used as a value), is not a method
//     that an interface of the package declares (it would no longer implement it), and no slot
//     is a result, a field, or captured by anything that keeps it.
//
// Why Java's behaviour is kept: immCursor_ walks the collection's elements as the immutable
// collection's iterator did (ImmutableCollection.iterator: a sliceIter_ over elems_(), the
// elements in iteration order, a multiset's with their repetitions): hasNext tests the
// position, next returns the element and advances or throws NoSuchElementException at the
// end, remove throws UnsupportedOperationException (the sliceIter_ of an immutable collection
// has neither a list nor a delete function). A null collection throws NullPointerException
// where C.iterator() did (immCursorOf_ reads its elements). The cursor is a pointer, so
// passing it to a helper that advances it shares the position as the iterator object did; it
// is small and does not escape a function that only calls it, so it stays on the stack.
//
// Deterministic; a second run changes nothing. usage: iters [-n] <dir of package closure>
package main

import (
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

type slot struct {
	v      *types.Var
	typ    ast.Expr // the declared type to rewrite, if any
	parent *slot
	poison string
	srcs   []ast.Expr // C.iterator() calls entering the slot
	n      int        // uses
}

var (
	slots   = map[*types.Var]*slot{}
	iterT   types.Type
	immType types.Type
	inGen   = map[*ast.File]bool{}
	left    = map[string]int{}
)

func find(s *slot) *slot {
	for s.parent != s {
		s.parent = s.parent.parent
		s = s.parent
	}
	return s
}

func union(a, b *slot) {
	a, b = find(a), find(b)
	if a == b {
		return
	}
	if a.v.Pos() > b.v.Pos() {
		a, b = b, a
	}
	b.parent = a
	if a.poison == "" {
		a.poison = b.poison
	}
	a.srcs = append(a.srcs, b.srcs...)
}

func poison(s *slot, why string) {
	r := find(s)
	if r.poison == "" {
		r.poison = why
	}
}

func isIter(t types.Type) bool { return t != nil && types.Identical(t, iterT) }

func isImm(t types.Type) bool {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	return ok && n.Obj().Name() == "ImmutableCollection" && n.Obj().Parent() == rw.Pkg.Scope()
}

func slotOf(e ast.Expr) *slot {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok {
		return nil
	}
	o, _ := rw.Info.Uses[id].(*types.Var)
	if o == nil {
		o, _ = rw.Info.Defs[id].(*types.Var)
	}
	return slots[o]
}

// immIterator returns C when e is C.iterator() with C an immutable collection.
func immIterator(e ast.Expr) ast.Expr {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return nil
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != "iterator" || !isImm(rw.Info.Types[s.X].Type) {
		return nil
	}
	return s.X
}

// enter records value e flowing into slot s.
func enter(s *slot, e ast.Expr) {
	e = rw.Unparen(e)
	if id, ok := e.(*ast.Ident); ok && id.Name == "nil" {
		return
	}
	if o := slotOf(e); o != nil {
		union(s, o)
		return
	}
	if immIterator(e) != nil {
		r := find(s)
		r.srcs = append(r.srcs, e)
		return
	}
	poison(s, "an iterator that is not an immutable collection's enters it")
}

// ifaceMethods are the method names that some interface of the package declares.
func ifaceMethods() map[string]bool {
	out := map[string]bool{}
	for _, n := range rw.Pkg.Scope().Names() {
		o, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok {
			continue
		}
		if it, ok := o.Type().Underlying().(*types.Interface); ok {
			for i := 0; i < it.NumMethods(); i++ {
				out[it.Method(i).Name()] = true
			}
		}
	}
	return out
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	iterT = rw.Pkg.Scope().Lookup("Iterator").Type()
	if rw.Pkg.Scope().Lookup("immCursorOf_") == nil {
		fmt.Fprintln(os.Stderr, "iters: the runtime has no immCursorOf_ (rt_iter.go)")
		os.Exit(1)
	}
	im := ifaceMethods()
	// Slots: local variables and parameters of type Iterator declared in generated code.
	funcOf := map[*types.Var]*types.Func{}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		inGen[f] = true
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			fo := rw.Info.Defs[fd.Name].(*types.Func)
			for _, fl := range fd.Type.Params.List {
				for _, nm := range fl.Names {
					if v := rw.Info.Defs[nm].(*types.Var); isIter(v.Type()) {
						s := &slot{v: v, typ: fl.Type}
						s.parent = s
						slots[v] = s
						funcOf[v] = fo
						if len(fl.Names) > 1 {
							s.poison = "a parameter sharing its declaration"
						}
						if fd.Recv != nil && im[fd.Name.Name] {
							s.poison = "a parameter of a method an interface declares"
						}
					}
				}
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.ValueSpec:
					for _, nm := range x.Names {
						if v, _ := rw.Info.Defs[nm].(*types.Var); v != nil && isIter(v.Type()) {
							s := &slot{v: v, typ: x.Type}
							s.parent = s
							slots[v] = s
							if len(x.Names) > 1 {
								s.poison = "a variable sharing its declaration"
							}
						}
					}
				case *ast.AssignStmt:
					if x.Tok == token.DEFINE {
						for _, l := range x.Lhs {
							if id, ok := l.(*ast.Ident); ok {
								if v, _ := rw.Info.Defs[id].(*types.Var); v != nil && isIter(v.Type()) {
									s := &slot{v: v}
									s.parent = s
									slots[v] = s
								}
							}
						}
					}
				case *ast.FuncLit:
					for _, fl := range x.Type.Params.List {
						for _, nm := range fl.Names {
							if v := rw.Info.Defs[nm].(*types.Var); isIter(v.Type()) {
								s := &slot{v: v, poison: "a function literal's parameter"}
								s.parent = s
								slots[v] = s
							}
						}
					}
				}
				return true
			})
		}
	}
	// Flows into slots and uses of slots.
	for _, f := range rw.Files {
		gen := inGen[f]
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.ValueSpec:
				for i, nm := range x.Names {
					if v, _ := rw.Info.Defs[nm].(*types.Var); slots[v] != nil && i < len(x.Values) {
						s := slots[v]
						if len(x.Values) != len(x.Names) {
							poison(s, "a multi-value declaration")
							continue
						}
						enter(s, x.Values[i])
					}
				}
			case *ast.AssignStmt:
				if len(x.Lhs) != len(x.Rhs) {
					for _, l := range x.Lhs {
						if s := slotOf(l); s != nil {
							poison(s, "assigned from a call with several results")
						}
					}
					break
				}
				for i, l := range x.Lhs {
					if s := slotOf(l); s != nil {
						enter(s, x.Rhs[i])
					}
				}
			case *ast.CallExpr:
				var id *ast.Ident
				switch fn := x.Fun.(type) {
				case *ast.Ident:
					id = fn
				case *ast.SelectorExpr:
					id = fn.Sel
				}
				var sig *types.Signature
				if id != nil {
					if fo, ok := rw.Info.Uses[id].(*types.Func); ok {
						sig = fo.Type().(*types.Signature)
					}
				}
				for i, a := range x.Args {
					var p *types.Var
					if sig != nil && i < sig.Params().Len() && !(sig.Variadic() && i >= sig.Params().Len()-1) {
						p = sig.Params().At(i)
					}
					if ps := slots[p]; ps != nil {
						enter(ps, a)
					} else if s := slotOf(a); s != nil {
						poison(s, "passed to code that wants an Iterator")
					}
				}
			case *ast.Ident:
				o, _ := rw.Info.Uses[x].(*types.Var)
				s := slots[o]
				if s == nil {
					// a function with slot parameters used other than by a call
					if fo, ok := rw.Info.Uses[x].(*types.Func); ok {
						sig := fo.Type().(*types.Signature)
						var callee bool
						switch p := rw.Parent[x].(type) {
						case *ast.CallExpr:
							callee = p.Fun == x
						case *ast.SelectorExpr:
							if c, ok := rw.Parent[p].(*ast.CallExpr); ok && c.Fun == p && p.Sel == x {
								callee = true
							}
						}
						if !callee {
							for i := 0; i < sig.Params().Len(); i++ {
								if ps := slots[sig.Params().At(i)]; ps != nil {
									poison(ps, "its function is used as a value")
								}
							}
						}
					}
					return true
				}
				find(s).n++
				if !gen {
					poison(s, "used by the runtime")
					return true
				}
				switch p := rw.Parent[x].(type) {
				case *ast.SelectorExpr:
					if c, ok := rw.Parent[p].(*ast.CallExpr); ok && c.Fun == p && len(c.Args) == 0 && (p.Sel.Name == "hasNext" || p.Sel.Name == "next" || p.Sel.Name == "remove") {
						return true
					}
					poison(s, "a method other than hasNext, next, remove")
				case *ast.AssignStmt:
					for i, r := range p.Rhs {
						if r == x && (len(p.Lhs) != len(p.Rhs) || slotOf(p.Lhs[i]) == nil) {
							poison(s, "assigned to something else")
						}
					}
				case *ast.ValueSpec:
					for i, v := range p.Values {
						if v == x && (i >= len(p.Names) || slotOf(p.Names[i]) == nil) {
							poison(s, "declares something else")
						}
					}
				case *ast.CallExpr:
					// an argument: handled with the call
				case *ast.ReturnStmt:
					poison(s, "returned")
				default:
					poison(s, fmt.Sprintf("used in a %T", p))
				}
			}
			return true
		})
	}
	// Variables captured by function literals other than ones called at once are fine (the
	// cursor is a pointer, as the iterator was); results are not slots, so returns poison.
	roots := map[*slot][]*slot{}
	var all []*slot
	for _, s := range slots {
		all = append(all, s)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].v.Pos() < all[j].v.Pos() })
	for _, s := range all {
		r := find(s)
		roots[r] = append(roots[r], s)
	}
	var rs []*slot
	for r := range roots {
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool { return rs[i].v.Pos() < rs[j].v.Pos() })
	nslots, nsrc := 0, 0
	for _, r := range rs {
		if r.poison == "" && len(r.srcs) == 0 {
			r.poison = "no iterator enters it"
		}
		if r.poison != "" {
			left[r.poison] += len(roots[r])
			continue
		}
		for _, s := range roots[r] {
			if s.typ != nil {
				rw.Rewrite[s.typ] = func() string { return "*immCursor_" }
			}
			nslots++
		}
		for _, src := range r.srcs {
			c := immIterator(src)
			rw.Rewrite[rw.Unparen(src)] = func() string { return "immCursorOf_(" + rw.Render(c) + ")" }
			nsrc++
		}
	}
	fmt.Printf("iters: %d variables and parameters become *immCursor_, %d iterators of immutable collections\n", nslots, nsrc)
	var ks []string
	for k := range left {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Printf("  left: %4d %s\n", left[k], k)
	}
	if report {
		return
	}
	fmt.Printf("files changed: %d\n", rw.SaveChecked())
}
