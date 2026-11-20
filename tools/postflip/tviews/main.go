// Command tviews is a post-flip step: code that reads a typed immutable collection through its
// erased twin (asImm(), the ErasedImmutableMap / ImmutableCollection erased code sees) reads the
// typed collection itself, with no boxing of the elements and no cast of what it reads:
//
//	for _, el := range immElems_(cast_[*ErasedImmutableSet](M.asImm().entrySet())) {
//		entry := icast_Map_Entry(el)                         for i, entryKey := range M.keys {
//		... str_(entry.getKey()) ...                  ->         entryValue := M.vals[i]
//		... cast_[*V](entry.getValue()) ...                      ... entryKey ... entryValue ...
//
//	range immElems_(cast_[*ErasedImmutableSet](M.asImm().keySet()))  ->  range M.keys
//	range immElems_(cast_[*ImmutableCollection](M.asImm().values()))  ->  range M.vals
//	cast_[V](M.asImm().get(k))                                        ->  M.Get(k)
//	var c *immCursor_ = immCursorOf_(L.asImm()); cast_[*T](c.next())  ->  var c *immCursorT_[*T] = immCursorTOf_(L); c.next()
//
// M is a *TMap[K, V] (ImmutableMap[K, V]) and L a *TList[T] (ImmutableList[T]). asImm() is
// only taken of an immutable collection (Java's static type is the Guava immutable type): its
// twin holds the typed collection's keys, values or elements boxed, in the same order
// (rt_timm.go), and neither ever changes. So the twin's entry, key, value and element
// iterators visit exactly M.keys / M.vals / L.items, and the twin's get(k) finds what M.Get(k)
// finds (the typed map hashes and compares its keys as the erased one does; a missing key
// gives the conversion's null, which is what the cast of the erased null gave). A read the
// code unboxed to the element type takes the typed value; any other read gets it boxed as the
// twin boxed it. A null M or L fails where the call on the nil twin failed (a nil dereference
// before the loop's first step). A range expression M that is not a variable or field path is
// evaluated once, into a variable before the loop.
//
// A cursor (iters' immCursor_ over an immutable list, for Java's explicit Iterator) becomes the
// typed cursor immCursorT_[T] (rt_iter.go) when every value of its variable, and of every
// parameter it is passed to, is an immCursorOf_(L.asImm()) of a list of the same element type,
// and the cursor is used only by hasNext / next / remove and by passing it to such parameters.
//
// Left (counted): an entry used otherwise than by getKey()/getValue(), a key or value type with
// no boxing conversion, reads the code converts to another type, cursors that meet other
// values. A rerun changes nothing (the rewritten code has no asImm()).
//
// usage: tviews [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

var (
	done    = map[string]int{}
	left    = map[string]int{}
	verbose bool
	where   = map[string][]string{}
)

func leave(n ast.Node, why string) {
	left[why]++
	if verbose {
		where[why] = append(where[why], rw.Fset.Position(n.Pos()).String())
	}
}

func typeOf(e ast.Expr) types.Type {
	if tv, ok := rw.Info.Types[e]; ok {
		return tv.Type
	}
	return nil
}

// generic returns the type arguments of t when t is a pointer to an instance of the package's
// generic type name.
func generic(t types.Type, name string) []types.Type {
	if t == nil {
		return nil
	}
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.Origin().Obj().Name() != name || n.Obj().Pkg() != rw.Pkg {
		return nil
	}
	var out []types.Type
	for i := 0; i < n.TypeArgs().Len(); i++ {
		out = append(out, n.TypeArgs().At(i))
	}
	return out
}

// call0 returns (receiver, name) of X.m() with no arguments.
func call0(e ast.Expr) (ast.Expr, string) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 0 {
		return nil, ""
	}
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil, ""
	}
	return s.X, s.Sel.Name
}

// asImmOf returns X when e is X.asImm() (through parentheses).
func asImmOf(e ast.Expr) ast.Expr {
	x, m := call0(e)
	if m != "asImm" {
		return nil
	}
	return x
}

// uncastSet strips cast_[*ErasedImmutableSet](e) / cast_[*ImmutableCollection](e).
func uncastSet(e ast.Expr) ast.Expr {
	if name, c := rw.CallName(e); c != nil && len(c.Args) == 1 && (name == "cast_[*ErasedImmutableSet]" || name == "cast_[*ImmutableCollection]") {
		return rw.Unparen(c.Args[0])
	}
	return rw.Unparen(e)
}

// read is the rewrite of a read n (of type any) whose value, of type t, is code(): an
// enclosing unboxing to t (the conversion's, cast_[t] or the assertion .(t)) is replaced by
// the value; any other read gets the value boxed as the twin boxed it.
func read(t types.Type, n ast.Expr, code func() string) (rw.Edit, bool) {
	cv := rw.ConvFor(t)
	switch p := rw.Up(n).(type) {
	case *ast.TypeAssertExpr:
		if rw.Unparen(p.X) == n && p.Type != nil && types.Identical(typeOf(p), t) {
			return rw.Edit{Node: p, Gen: code}, true
		}
	case *ast.CallExpr:
		if len(p.Args) == 1 && rw.Unparen(p.Args[0]) == n {
			name, _ := rw.CallName(p)
			if (strings.HasPrefix(name, "cast_[") || strings.HasPrefix(name, "unbox_[")) && types.Identical(typeOf(p), t) || cv.OK && name != "" && cv.IsUnbox(name) {
				return rw.Edit{Node: p, Gen: code}, true
			}
		}
	}
	if !cv.OK {
		return rw.Edit{}, false
	}
	return rw.Edit{Node: n, Gen: func() string { return cv.Box(code()) }}, true
}

// fn holds the names declared or used in a function, for fresh names.
type fn struct{ names map[string]bool }

func (f *fn) fresh(base string) string {
	n := base
	for k := 2; f.names[n]; k++ {
		n = fmt.Sprintf("%s%d", base, k)
	}
	f.names[n] = true
	return n
}

func assignedOrAddressed(o types.Object) bool {
	for _, u := range rw.UsesOf[o] {
		switch p := rw.Parent[u].(type) {
		case *ast.AssignStmt:
			for _, l := range p.Lhs {
				if l == u {
					return true
				}
			}
		case *ast.UnaryExpr:
			if p.Op == token.AND {
				return true
			}
		case *ast.IncDecStmt:
			return true
		}
	}
	return false
}

// loop handles a range statement over immElems_(...asImm().entrySet()/keySet()/values()).
func loop(f *fn, r *ast.RangeStmt) {
	name, call := rw.CallName(r.X)
	if call == nil || name != "immElems_" || len(call.Args) != 1 {
		return
	}
	view, m := call0(uncastSet(call.Args[0]))
	if m != "entrySet" && m != "keySet" && m != "values" {
		return
	}
	M := asImmOf(view)
	if M == nil {
		return
	}
	kv := generic(typeOf(M), "TMap")
	if kv == nil {
		leave(r, "loop: the view of an erased map")
		return
	}
	if r.Tok == token.ASSIGN || r.Key != nil && rw.Text(r.Key) != "_" {
		leave(r, "loop: range variables assigned or indexed")
		return
	}
	var el *ast.Ident
	if r.Value != nil {
		el, _ = r.Value.(*ast.Ident)
		if el == nil || rw.Info.Defs[el] == nil {
			leave(r, "loop: range variables assigned or indexed")
			return
		}
		if assignedOrAddressed(rw.Info.Defs[el]) {
			leave(r, "loop: the element variable is assigned or its address taken")
			return
		}
	}
	// the statement holding the loop (its label's), for a variable of the map before it
	stmt := ast.Node(r)
	if l, ok := rw.Parent[r].(*ast.LabeledStmt); ok {
		stmt = l
	}
	if _, ok := rw.Parent[stmt].(*ast.BlockStmt); !ok && !rw.Pure(M) {
		leave(r, "loop: the map is an expression and the loop is not in a block")
		return
	}
	var edits []rw.Edit
	if m != "entrySet" {
		t, field := kv[0], "keys"
		if m == "values" {
			t, field = kv[1], "vals"
		}
		if el != nil {
			for _, u := range rw.UsesOf[rw.Info.Defs[el]] {
				u := u
				e, ok := read(t, u, func() string { return u.Name })
				if !ok {
					leave(r, "loop: an element type with no boxing conversion, read boxed")
					return
				}
				edits = append(edits, e)
			}
		}
		Ms := M
		edits = append(edits, rw.Edit{Node: call, Gen: func() string { return rw.Render(Ms) + "." + field }})
		for _, e := range edits {
			rw.Rewrite[e.Node] = e.Gen
		}
		done["loops over the "+m+"() of a typed immutable map's twin: range over its "+field]++
		return
	}
	// entrySet: the first statement is `entry := icast_Map_Entry(el)`, el used nowhere else, and
	// entry used only by getKey() / getValue().
	if el == nil || len(r.Body.List) == 0 {
		leave(r, "loop: entry loop without the entry statement")
		return
	}
	var entry *ast.Ident
	switch s := r.Body.List[0].(type) {
	case *ast.AssignStmt:
		if s.Tok == token.DEFINE && len(s.Lhs) == 1 && len(s.Rhs) == 1 {
			if n, c := rw.CallName(s.Rhs[0]); n == "icast_Map_Entry" && identName(c.Args[0]) == el.Name {
				entry, _ = s.Lhs[0].(*ast.Ident)
			}
		}
	case *ast.DeclStmt:
		if gd, ok := s.Decl.(*ast.GenDecl); ok && len(gd.Specs) == 1 {
			if vs, ok := gd.Specs[0].(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 1 {
				if n, c := rw.CallName(vs.Values[0]); n == "icast_Map_Entry" && identName(c.Args[0]) == el.Name {
					entry = vs.Names[0]
				}
			}
		}
	}
	if entry == nil || len(rw.UsesOf[rw.Info.Defs[el]]) != 1 {
		leave(r, "loop: entry loop without the entry statement")
		return
	}
	eo := rw.Info.Defs[entry]
	if assignedOrAddressed(eo) {
		leave(r, "loop: the entry variable is assigned")
		return
	}
	var getK, getV []*ast.CallExpr
	for _, u := range rw.UsesOf[eo] {
		s, ok := rw.Parent[u].(*ast.SelectorExpr)
		c, ok2 := rw.Parent[s].(*ast.CallExpr)
		if !ok || !ok2 || s.X != u || c.Fun != s || len(c.Args) != 0 || s.Sel.Name != "getKey" && s.Sel.Name != "getValue" {
			leave(r, "loop: the entry is used other than by getKey() and getValue()")
			return
		}
		if s.Sel.Name == "getKey" {
			getK = append(getK, c)
		} else {
			getV = append(getV, c)
		}
	}
	kn, vn := "_", ""
	if len(getK) > 0 {
		kn = f.fresh(entry.Name + "Key")
	}
	if len(getV) > 0 {
		vn = f.fresh(entry.Name + "Value")
	}
	for _, c := range getK {
		e, ok := read(kv[0], c, func() string { return kn })
		if !ok {
			leave(r, "loop: a key type with no boxing conversion, read boxed")
			return
		}
		edits = append(edits, e)
	}
	for _, c := range getV {
		e, ok := read(kv[1], c, func() string { return vn })
		if !ok {
			leave(r, "loop: a value type with no boxing conversion, read boxed")
			return
		}
		edits = append(edits, e)
	}
	for _, e := range edits {
		rw.Rewrite[e.Node] = e.Gen
	}
	ms := ""
	if !rw.Pure(M) {
		ms = f.fresh("m")
	}
	ix := "_"
	if vn != "" {
		ix = f.fresh("i")
	}
	first := r.Body.List[0]
	Ms := M
	mapText := func() string {
		if ms != "" {
			return ms
		}
		return rw.Render(Ms)
	}
	rw.Rewrite[first] = func() string {
		if vn == "" {
			return rw.Del
		}
		return vn + " := " + mapText() + ".vals[" + ix + "]"
	}
	header := "for " + ix + ", " + kn + " := range "
	if kn == "_" {
		header = "for " + ix + " := range "
	}
	body := r.Body
	loopText := func() string { return header + mapText() + ".keys " + rw.Render(body) }
	rw.Rewrite[r] = loopText
	if ms != "" {
		label := ""
		if l, ok := stmt.(*ast.LabeledStmt); ok {
			label = l.Label.Name + ":\n"
		}
		indent := indentOf(stmt)
		rw.Rewrite[stmt] = func() string {
			return ms + " := " + rw.Render(Ms) + "\n" + label + indent + loopText()
		}
	}
	done["loops over the entrySet() of a typed immutable map's twin: range over its keys and values"]++
}

func identName(e ast.Expr) string {
	if id, ok := rw.Unparen(e).(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

// indentOf is the indentation of the line n starts on.
func indentOf(n ast.Node) string {
	f := rw.Fset.File(n.Pos())
	src := rw.Src[f]
	off := f.Offset(n.Pos())
	i := off
	for i > 0 && src[i-1] != '\n' {
		i--
	}
	return string(src[i:off])
}

// get handles cast_[V](M.asImm().get(k)) (or V's unboxing of it).
func get(c *ast.CallExpr) {
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != "get" || len(c.Args) != 1 {
		return
	}
	M := asImmOf(s.X)
	if M == nil {
		return
	}
	kv := generic(typeOf(M), "TMap")
	if kv == nil {
		return
	}
	// the key: of type K, or K boxed
	a := rw.Unparen(c.Args[0])
	var key ast.Expr
	if types.Identical(typeOf(a), kv[0]) {
		key = a
	} else if inner := unbox(a); inner != nil && types.Identical(typeOf(inner), kv[0]) {
		key = inner
	}
	if key == nil {
		leave(c, "get: the key is not of the map's key type")
		return
	}
	Ms, ks := M, key
	e, ok := read(kv[1], c, func() string { return rw.Render(Ms) + ".Get(" + rw.Render(ks) + ")" })
	if !ok || e.Node == ast.Node(c) {
		leave(c, "get: the value is read boxed")
		return
	}
	rw.Rewrite[e.Node] = e.Gen
	done["get(k) of a typed immutable map's twin: M.Get(k)"]++
}

// unbox returns x of obj_(x), strobj_(x), any(x), x.self(), x.box_().
func unbox(a ast.Expr) ast.Expr {
	call, ok := rw.Unparen(a).(*ast.CallExpr)
	if !ok {
		return nil
	}
	if id, ok := call.Fun.(*ast.Ident); ok && len(call.Args) == 1 && (id.Name == "obj_" || id.Name == "strobj_" || id.Name == "any") {
		return call.Args[0]
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
	for _, file := range rw.Files {
		for _, d := range file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			f := &fn{names: map[string]bool{}}
			ast.Inspect(fd, func(n ast.Node) bool {
				if id, ok := n.(*ast.Ident); ok {
					f.names[id.Name] = true
				}
				return true
			})
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.RangeStmt:
					loop(f, x)
				case *ast.CallExpr:
					get(x)
				}
				return true
			})
		}
	}
	cursors()
	print := func(m map[string]int, head string) {
		var ks []string
		for k := range m {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			fmt.Fprintf(os.Stderr, "tviews: %s %d %s\n", head, m[k], k)
			for _, w := range where[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", w)
			}
		}
	}
	print(done, "")
	print(left, "left:")
	if report {
		return
	}
	fmt.Fprintf(os.Stderr, "tviews: files changed: %d\n", rw.SaveChecked())
}
