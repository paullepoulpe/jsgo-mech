// Command htabself is a post-flip step: a class whose objects are stored in the handle-table
// fields of handle classes (rt_ids.go, htab_set_) keeps its own handle, so that storing one of
// its objects finds the handle in the object instead of looking its interface words up in the
// table's cache and map.
//
//	type JSType struct {                       type JSType struct {
//		...                                     ...
//	}                                   ->      htabId_ uint32 // its handle while a field holds it
//	                                        }
//	                                        func (this *JSType) htabSlot_() *uint32 { return &this.htabId_ }
//
// The classes are found by type: a setter of generated code that stores its parameter with
// htab_set_ (Node.jstypeOrColor_set_), and every call of it (or of htab_set_ itself) whose
// argument is a pointer to a class *C, the object x.self() of a class pointer x of type *C, or
// obj_(x) of one, names C. The field goes in C; the method htabSlot_ is promoted to every class
// that embeds C, so the most derived object that x.self() gives has it too. An argument of
// another form (an interface value of unknown class) is counted as left; the runtime keeps the
// map for such values.
//
// The runtime (rt_ids.go, patch reflip6-htabself-rt.patch) asserts a stored value to
// htabHolder_ (interface{ htabSlot_() *uint32 }) and uses the slot as the index of the value's
// entry: the slot holds the entry's handle when the value's entry was made while the slot was
// 0; every other entry is in the map as before. A slot is believed only when the entry it names
// holds the same interface words as the stored value, so which entry a value finds is the same
// as with the map alone, whatever the slot holds (identity stays the interface words).
//
// Proof obligation (what makes the slot an index, not a guess), checked here:
//
//   - The slot is written by the runtime only: no declaration of the package names htabId_ or
//     htabSlot_ other than this tool's own (a rerun finds them and changes nothing).
//   - An object's slot stays with that object: no value of a struct type that holds C by value
//     (C itself, a class embedding it, an array of them) is copied, compared, passed, returned,
//     assigned or kept in a variable; such a value is only made by &T{...} (keyed, so the new
//     field starts at 0) or used to select a field or method or to take an address. Overwriting
//     a held object with another's contents would leave its slot naming the other's entry (a
//     second entry for one value), and == of two such structs would compare the field.
//   - No generic instantiation takes such a struct type as a type argument.
//   - C is a non-generic struct declared in generated code; its pointer type has no other
//     htabSlot_ (none is declared at all, above), so the promoted method is C's.
//
// Not checked here: that the runtime does not copy such objects through unsafe or reflect (the
// runtime is hand-written and holds no such copy; rt_ids.go reads the slot through the method).
//
// usage: htabself [-n] [-v] <dir of package closure>
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

const fieldName, methodName = "htabId_", "htabSlot_"

func main() {
	report, verbose := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	dir := os.Args[len(os.Args)-1]
	rw.Load(dir)
	pos := func(n ast.Node) string { return rw.Fset.Position(n.Pos()).String() }
	generated := func(n ast.Node) bool {
		return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "g_")
	}
	fail := func(format string, a ...any) {
		fmt.Fprintf(os.Stderr, "htabself: "+format+"\n", a...)
		os.Exit(1)
	}

	// The setters: a function or method of generated code that gives its parameter to htab_set_.
	htabSet, _ := rw.Pkg.Scope().Lookup("htab_set_").(*types.Func)
	if htabSet == nil {
		fmt.Fprintln(os.Stderr, "htabself: no htab_set_ in the runtime: nothing to do")
		return
	}
	type sink struct {
		fn  *types.Func
		arg int
	}
	sinks := []sink{{htabSet, 1}}
	sinkParams := map[types.Object]bool{} // the parameters the setters store
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || !generated(fd) {
				continue
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok || len(c.Args) != 3 {
					return true
				}
				if id, ok := c.Fun.(*ast.Ident); !ok || rw.Info.Uses[id] != htabSet {
					return true
				}
				a, ok := rw.Unparen(c.Args[1]).(*ast.Ident)
				if !ok {
					return true
				}
				i := 0
				for _, fl := range fd.Type.Params.List {
					for _, nm := range fl.Names {
						if rw.Info.Defs[nm] == rw.Info.Uses[a] {
							sinks = append(sinks, sink{rw.Info.Defs[fd.Name].(*types.Func), i})
							sinkParams[rw.Info.Defs[nm]] = true
						}
						i++
					}
				}
				return true
			})
		}
	}

	// The classes of the values stored, by the static type at each call of a sink.
	classOf := func(e ast.Expr) *types.Named {
		ptr := func(t types.Type) *types.Named {
			p, ok := types.Unalias(t).(*types.Pointer)
			if !ok {
				return nil
			}
			n, ok := types.Unalias(p.Elem()).(*types.Named)
			if !ok {
				return nil
			}
			if _, ok := n.Underlying().(*types.Struct); !ok {
				return nil
			}
			return n
		}
		e = rw.Unparen(e)
		if n := ptr(rw.Info.TypeOf(e)); n != nil {
			return n
		}
		c, ok := e.(*ast.CallExpr)
		if !ok || len(c.Args) > 1 {
			return nil
		}
		switch fn := c.Fun.(type) {
		case *ast.SelectorExpr: // x.self() of a class pointer x
			if len(c.Args) == 0 && fn.Sel.Name == "self" {
				return ptr(rw.Info.TypeOf(fn.X))
			}
		case *ast.Ident, *ast.IndexExpr: // obj_(x) of a class pointer x
			if name, _ := rw.CallName(c); (name == "obj_" || strings.HasPrefix(name, "obj_[")) && len(c.Args) == 1 {
				return ptr(rw.Info.TypeOf(c.Args[0]))
			}
		}
		return nil
	}
	classes := map[*types.Named]int{}
	left := map[string][]string{}
	for _, s := range sinks {
		for _, id := range rw.UsesOf[s.fn] {
			c, ok := rw.Up(id).(*ast.CallExpr)
			if !ok {
				if sel, ok2 := rw.Up(id).(*ast.SelectorExpr); ok2 && sel.Sel == id {
					c, ok = rw.Up(sel).(*ast.CallExpr)
				}
			}
			if !ok || s.arg >= len(c.Args) {
				left["a sink used other than by a call"] = append(left["a sink used other than by a call"], pos(id))
				continue
			}
			if a, ok := rw.Unparen(c.Args[s.arg]).(*ast.Ident); ok && sinkParams[rw.Info.Uses[a]] {
				continue // a setter's own store, counted at the setter's calls
			}
			if n := classOf(c.Args[s.arg]); n != nil {
				classes[n]++
			} else {
				why := "a stored value of no class pointer type (" + types.TypeString(rw.Info.TypeOf(c.Args[s.arg]), types.RelativeTo(rw.Pkg)) + ")"
				left[why] = append(left[why], pos(c))
			}
		}
	}

	// Which of them can carry the field.
	decl := map[*types.Named]*ast.TypeSpec{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.TYPE {
				for _, sp := range g.Specs {
					ts := sp.(*ast.TypeSpec)
					if n, ok := rw.Info.Defs[ts.Name].Type().(*types.Named); ok {
						decl[n] = ts
					}
				}
			}
		}
	}
	var holders []*types.Named
	for n := range classes {
		ts := decl[n]
		switch {
		case ts == nil || !generated(ts):
			left["a class of the runtime"] = append(left["a class of the runtime"], n.Obj().Name())
		case n.TypeParams().Len() > 0:
			left["a generic class"] = append(left["a generic class"], n.Obj().Name())
		default:
			holders = append(holders, n)
		}
	}
	sort.Slice(holders, func(i, j int) bool { return holders[i].Obj().Name() < holders[j].Obj().Name() })
	isHolder := map[*types.Named]bool{}
	for _, n := range holders {
		isHolder[n] = true
	}

	// Obligation 1: the names are this tool's alone.
	done := map[*types.Named]bool{}
	for o := range rw.Info.Defs {
		obj := rw.Info.Defs[o]
		if obj == nil || (obj.Name() != fieldName && obj.Name() != methodName) {
			continue
		}
		var owner *types.Named
		switch v := obj.(type) {
		case *types.Var:
			if v.IsField() {
				for n, ts := range decl {
					if st, ok := ts.Type.(*ast.StructType); ok && st.Pos() <= o.Pos() && o.Pos() < st.End() {
						owner = n
					}
				}
			}
		case *types.Func:
			if r := v.Type().(*types.Signature).Recv(); r != nil {
				if n, ok := r.Type().(*types.Named); ok && n.Obj().Name() == "htabHolder_" && types.IsInterface(n) && !generated(o) {
					continue // the runtime's interface that reads the slot
				}
				if p, ok := r.Type().(*types.Pointer); ok {
					owner, _ = p.Elem().(*types.Named)
				}
			}
		}
		if owner == nil || !isHolder[owner] {
			fail("%s is declared at %s, not by this tool for a class it writes: the slot would not be the runtime's alone", obj.Name(), pos(o))
		}
		if obj.Name() == fieldName {
			done[owner] = true
		}
	}

	// Obligation 2: no copy of an object that holds a slot.
	holds := map[types.Type]bool{}
	var holdsSlot func(t types.Type, seen map[types.Type]bool) bool
	holdsSlot = func(t types.Type, seen map[types.Type]bool) bool {
		t = types.Unalias(t)
		if r, ok := holds[t]; ok {
			return r
		}
		if seen[t] {
			return false
		}
		seen[t] = true
		r := false
		switch u := t.(type) {
		case *types.Named:
			r = isHolder[u] || isHolder[u.Origin()] || holdsSlot(u.Underlying(), seen)
		case *types.Struct:
			for i := 0; i < u.NumFields() && !r; i++ {
				r = holdsSlot(u.Field(i).Type(), seen)
			}
		case *types.Array:
			r = holdsSlot(u.Elem(), seen)
		}
		holds[t] = r
		return r
	}
	slotType := func(t types.Type) bool { return t != nil && holdsSlot(t, map[types.Type]bool{}) }
	var bad []string
	for e, tv := range rw.Info.Types {
		if !tv.IsValue() || !slotType(tv.Type) {
			continue
		}
		up := rw.Up(e)
		ok := false
		switch p := up.(type) {
		case *ast.SelectorExpr:
			ok = p.X == rw.Unparen(e) || p.X == e
		case *ast.UnaryExpr:
			ok = p.Op == token.AND
		case *ast.KeyValueExpr: // a field of an enclosing literal that is itself made in place
			ok = p.Value == e && inPlace(e)
		}
		if cl, isLit := rw.Unparen(e).(*ast.CompositeLit); isLit {
			ok = ok && inPlace(cl)
			if len(cl.Elts) > 0 {
				if _, keyed := cl.Elts[0].(*ast.KeyValueExpr); !keyed {
					ok = false
				}
			}
		}
		if !ok {
			bad = append(bad, pos(e)+": "+rw.Text(e))
		}
	}
	for id, obj := range rw.Info.Defs {
		if v, ok := obj.(*types.Var); ok && !v.IsField() && slotType(v.Type()) {
			bad = append(bad, pos(id)+": variable "+id.Name)
		}
	}
	for id, inst := range rw.Info.Instances {
		for i := 0; i < inst.TypeArgs.Len(); i++ {
			if slotType(inst.TypeArgs.At(i)) {
				bad = append(bad, pos(id)+": type argument of "+id.Name)
			}
		}
	}
	if len(bad) > 0 {
		sort.Strings(bad)
		for i, b := range bad {
			if i < 20 {
				fmt.Fprintln(os.Stderr, "  copied: "+b)
			}
		}
		fail("%d values of a class with a slot are copied, compared or kept by value: nothing written", len(bad))
	}

	// The classes reached: every struct of the package that embeds a holder by value.
	reach := map[*types.Named]int{}
	byPtr := 0
	for n := range decl {
		st, ok := n.Underlying().(*types.Struct)
		if !ok || isHolder[n] {
			continue
		}
		for _, h := range holders {
			if embedsByValue(st, h, map[types.Type]bool{}) {
				reach[h]++
			} else if embedsByPtr(st, h) {
				byPtr++
			}
		}
	}

	var lefts []string
	for k := range left {
		lefts = append(lefts, k)
	}
	sort.Strings(lefts)
	for _, k := range lefts {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", len(left[k]), k)
		if verbose {
			for _, p := range left[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	nsites := 0
	for _, h := range holders {
		nsites += classes[h]
		state := "written"
		if done[h] {
			state = "already has its slot"
		}
		fmt.Fprintf(os.Stderr, "  %-24s %d stores, %d classes embed it by value: %s\n", h.Obj().Name(), classes[h], reach[h], state)
	}
	if byPtr > 0 {
		fmt.Fprintf(os.Stderr, "  %d classes embed a holder by pointer (they share its slot; the runtime keeps the map for them)\n", byPtr)
	}
	todo := 0
	for _, h := range holders {
		if done[h] {
			continue
		}
		todo++
		ts := decl[h]
		st := ts.Type.(*ast.StructType)
		name := h.Obj().Name()
		rw.Rewrite[st] = func() string {
			s := rw.RenderChildren(st)
			i := strings.LastIndex(s, "}")
			return s[:i] + "\t" + fieldName + " uint32 // its handle in the table of distinct values while a field holds it, 0 when none (rt_ids.go: the runtime's alone; post-flip htabself)\n" + s[i:]
		}
		g := rw.Parent[ts].(*ast.GenDecl)
		if g.Lparen.IsValid() {
			fail("class %s is declared in a type group", name)
		}
		rw.Rewrite[g] = func() string {
			return rw.RenderChildren(g) + "\n\n// " + methodName + " is where the table of distinct values keeps this object's handle (rt_ids.go, htabHolder_).\nfunc (this *" + name + ") " + methodName + "() *uint32 { return &this." + fieldName + " }"
		}
	}
	fmt.Fprintf(os.Stderr, "htabself: %d sinks, %d classes (%d stores), %d to write\n", len(sinks), len(holders), nsites, todo)
	if report || todo == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "htabself: files changed: %d\n", rw.SaveChecked())
}

// inPlace reports whether the composite literal (or a value in it) is made where it lives: the
// operand of &, or a keyed field value of an enclosing literal that is in place.
func inPlace(e ast.Expr) bool {
	for {
		up := rw.Up(e)
		switch p := up.(type) {
		case *ast.UnaryExpr:
			return p.Op == token.AND
		case *ast.KeyValueExpr:
			lit, ok := rw.Up(p).(*ast.CompositeLit)
			if !ok {
				return false
			}
			e = lit
		default:
			return false
		}
	}
}

func embedsByValue(st *types.Struct, h *types.Named, seen map[types.Type]bool) bool {
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if !f.Embedded() {
			continue
		}
		n, ok := types.Unalias(f.Type()).(*types.Named)
		if !ok || seen[n] {
			continue
		}
		seen[n] = true
		if n == h {
			return true
		}
		if s, ok := n.Underlying().(*types.Struct); ok && embedsByValue(s, h, seen) {
			return true
		}
	}
	return false
}

func embedsByPtr(st *types.Struct, h *types.Named) bool {
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if p, ok := types.Unalias(f.Type()).(*types.Pointer); ok && f.Embedded() && p.Elem() == h {
			return true
		}
	}
	return false
}
