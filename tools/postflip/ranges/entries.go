package main

// -entries: a for-each loop over the entry set (or the values) of a typed map's erased view,
//
//	for e := range elemsBy_(M.asMap().entrySet(), icast_Map_Entry) {   for k, v := range M.All() {
//		... Node_of_(e.getKey()) ... str_(e.getValue()) ...        ->      ... k ... v ...
//
// ranges over the typed map itself, with no view, no entry object per element and no boxing
// of keys and values. The view's entry iterator walks the map's table from its first live
// entry, skipping removed ones and reading the table anew at every step, and each entry holds
// the key and value of its position at that step (rt_tmap.go: entryOut_); M.All() walks the
// table the same way and yields the same key and value. The loop variable may be used only for
// getKey() and getValue() (a setValue, or the entry itself passed on or compared, keeps the
// view). A read that the code unboxes to the map's key or value type takes the typed value;
// any other read gets it boxed as the entry boxed it.

import (
	"go/ast"
	"go/types"
	"strings"

	"postflip/rw"
)

// typedMapView matches M.asMap().<m>() with M of static type *TMap and returns M.
func typedMapView(e ast.Expr, m string) ast.Expr {
	recv, name := method(e)
	if recv == nil || name != m {
		return nil
	}
	mr, mm := method(recv)
	if mr == nil || mm != "asMap" || named(typeOf(mr)) != "TMap" {
		return nil
	}
	return mr
}

func entryRange(f *fn, r *ast.RangeStmt) {
	call, ok := rw.Unparen(r.X).(*ast.CallExpr)
	if !ok || r.Value != nil || r.Key == nil {
		return
	}
	name, _ := rw.CallName(call)
	if fid := identOf(call.Fun); fid == nil || rw.Info.Uses[fid] == nil || rw.Info.Uses[fid].Parent() != rw.Pkg.Scope() {
		return
	}
	key, ok := r.Key.(*ast.Ident)
	if !ok || key.Name == "_" {
		return
	}
	// values(): for v := range elemsBy_/elemsAs_(M.asMap().values(), F) where F unboxes to V.
	if len(call.Args) >= 1 {
		if m := typedMapView(call.Args[0], "values"); m != nil {
			vt := typeArgs(typeOf(m))[1]
			cv := rw.ConvFor(vt)
			var conv string
			switch {
			case name == "elemsBy_" && len(call.Args) == 2:
				conv = rw.Text(call.Args[1])
			case name == "elemsAs_" && len(call.Args) == 1:
				if ix, ok := call.Fun.(*ast.IndexExpr); ok {
					conv = "cast_[" + rw.Text(ix.Index) + "]"
				}
			}
			if conv != "" && cv.OK && cv.IsUnbox(conv) && types.Identical(rw.Info.Defs[key].Type(), vt) {
				ms := m
				body := r.Body
				rw.Rewrite[r] = func() string {
					return "for " + key.Name + " := range " + rw.Render(ms) + ".Values() " + rw.Render(body)
				}
				done["entries: loop over the values of a typed map's view"]++
			} else {
				left["entries: the values of a typed map's view converted to another type"]++
			}
			return
		}
	}
	if name != "elemsBy_" || len(call.Args) != 2 || rw.Text(call.Args[1]) != "icast_Map_Entry" {
		return
	}
	m := typedMapView(call.Args[0], "entrySet")
	if m == nil {
		return
	}
	args := typeArgs(typeOf(m))
	kt, vt := args[0], args[1]
	o := rw.Info.Defs[key]
	var getK, getV []*ast.CallExpr
	for _, u := range rw.UsesOf[o] {
		s, ok := rw.Parent[u].(*ast.SelectorExpr)
		if !ok || s.X != u {
			left["entries: the entry is used other than for getKey and getValue"]++
			return
		}
		c, ok := rw.Parent[s].(*ast.CallExpr)
		if !ok || c.Fun != s || len(c.Args) != 0 || (s.Sel.Name != "getKey" && s.Sel.Name != "getValue") {
			left["entries: the entry is used other than for getKey and getValue"]++
			return
		}
		if s.Sel.Name == "getKey" {
			getK = append(getK, c)
		} else {
			getV = append(getV, c)
		}
	}
	kcv, vcv := rw.ConvFor(kt), rw.ConvFor(vt)
	if len(getK) > 0 && !kcv.OK || len(getV) > 0 && !vcv.OK {
		left["entries: a key or value type without a boxing conversion"]++
		return
	}
	var kn, vn string
	if len(getK) > 0 {
		kn = f.fresh(key.Name + "Key")
	}
	if len(getV) > 0 {
		vn = f.fresh(key.Name + "Value")
	}
	for _, c := range getK {
		ed := rw.Read(kcv, c, func() string { return kn })
		rw.Rewrite[ed.Node] = ed.Gen
	}
	for _, c := range getV {
		ed := rw.Read(vcv, c, func() string { return vn })
		rw.Rewrite[ed.Node] = ed.Gen
	}
	var header string
	ms := "\x00"
	switch {
	case kn != "" && vn != "":
		header = kn + ", " + vn + " := range " + ms + ".All()"
	case kn != "":
		header = kn + " := range " + ms + ".Keys()"
	case vn != "":
		header = vn + " := range " + ms + ".Values()"
	default:
		header = "range " + ms + ".Keys()"
	}
	body := r.Body
	rw.Rewrite[r] = func() string {
		return "for " + strings.Replace(header, ms, rw.Render(m), 1) + " " + rw.Render(body)
	}
	done["entries: loop over the entries of a typed map's view"]++
}
