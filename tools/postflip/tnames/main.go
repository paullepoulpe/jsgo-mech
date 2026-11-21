// Command tnames is a post-flip step after colls, native and nilmaps: the typed ordered map
// and set (TMap, TSet: rt_tmap.go) get Go names for the methods that still had Java's.
//
// TMap and TSet are plain generic Go types (an ordered map with a Go-map index), kept where
// the order of a Java LinkedHashMap is observable or keys compare by equals. Their methods
// were named as Java's so that step 4 could turn m.put(k, v) on an erased Map into the same
// call on the typed one. Now:
//
//	TMap: get Get, put Put, containsKey Has, remove Delete, size Len, isEmpty IsEmpty,
//	      clear Clear, getOrDefault GetOr, putIfAbsent PutIfAbsent
//	TSet: add Add, contains Has, remove Delete, addAllT AddAll (size, isEmpty and clear are
//	      the embedded map's)
//
// A method is renamed at its declaration and at every use (go/types: the uses of the method
// object, also through the embedding of TMap in TSet), in the generated code, the runtime
// and its tests alike; the package must type-check afterwards, or nothing is written. The
// behaviour is the same: only names change. The views erased code uses (asMap, asSet), the
// cursors and the internal methods keep their names.
//
// computeIfAbsent_f(k, f) took a boxed function (func(any) any: the key boxed for it, its
// result unboxed). It becomes ComputeIfAbsent(k, f) with f a func(K) V, where f is a function
// literal or a named function that only this call uses: its parameter becomes a K (an unboxing of it in the body, str_(p) or cast_[*T](p),
// is the parameter itself; any other use boxes it as the map did), and each result is unboxed
// as the map did (obj_(x) of an x that is a V is x). Java's computeIfAbsent stores nothing
// and answers null when the function returns null; so does ComputeIfAbsent. A call with a
// function other calls share, or on a key or value type without a known conversion (a typed
// collection, boxed as its view), is left and counted.
//
// colls and native know the Java names: before they run again, tnames -undo gives the methods
// their Java names back (tools/postflip/colls/run.sh does both). ComputeIfAbsent stays.
//
// usage: tnames [-n] [-undo] <dir of package closure>
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

var names = map[string]map[string]string{
	"TMap": {"get": "Get", "put": "Put", "containsKey": "Has", "remove": "Delete", "size": "Len", "isEmpty": "IsEmpty",
		"clear": "Clear", "getOrDefault": "GetOr", "putIfAbsent": "PutIfAbsent"},
	"TSet": {"add": "Add", "contains": "Has", "remove": "Delete", "addAllT": "AddAll"},
}

// funcDecls are the declarations of the generated functions.
var funcDecls = map[*types.Func]*ast.FuncDecl{}

func recvName(f *types.Func) string {
	sig := f.Type().(*types.Signature)
	if sig.Recv() == nil {
		return ""
	}
	t := sig.Recv().Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := types.Unalias(t).(*types.Named); ok {
		return n.Obj().Name()
	}
	return ""
}

func main() {
	report, undo := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		undo = undo || a == "-undo"
	}
	rw.WithTests = true
	rw.Load(os.Args[len(os.Args)-1])
	table := names
	if undo {
		table = map[string]map[string]string{}
		for t, m := range names {
			table[t] = map[string]string{}
			for a, b := range m {
				table[t][b] = a
			}
		}
	}
	// The methods to rename, and their declarations.
	renamed := map[*types.Func]string{}
	decls, uses := 0, 0
	counts := map[string]int{}
	for id, o := range rw.Info.Defs {
		f, ok := o.(*types.Func)
		if !ok {
			continue
		}
		if to, ok := table[recvName(f)][f.Name()]; ok {
			renamed[f] = to
			decls++
			rw.Rewrite[id] = func() string { return to }
		}
	}
	for id, o := range rw.Info.Uses {
		f, ok := o.(*types.Func)
		if !ok {
			continue
		}
		if to, ok := renamed[f.Origin()]; ok {
			uses++
			counts[recvName(f.Origin())+"."+f.Name()+" -> "+to]++
			rw.Rewrite[id] = func() string { return to }
		}
	}
	for _, file := range rw.Files {
		for _, d := range file.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				if o, ok := rw.Info.Defs[fd.Name].(*types.Func); ok && strings.HasPrefix(rw.Fset.Position(fd.Pos()).Filename[strings.LastIndex(rw.Fset.Position(fd.Pos()).Filename, "/")+1:], "g_") {
					funcDecls[o] = fd
				}
			}
		}
	}
	// computeIfAbsent_f with a function literal: a typed function.
	left := map[string]int{}
	typed := 0
	if !undo {
		for _, file := range rw.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok || len(c.Args) != 2 {
					return true
				}
				sel, ok := c.Fun.(*ast.SelectorExpr)
				if !ok || sel.Sel.Name != "computeIfAbsent_f" {
					return true
				}
				f, ok := rw.Info.Uses[sel.Sel].(*types.Func)
				if !ok || recvName(f.Origin()) != "TMap" {
					return true
				}
				if why := typedFunc(c, sel); why != "" {
					left[why]++
				} else {
					typed++
				}
				return true
			})
		}
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d uses of %s\n", counts[k], k)
	}
	keys = keys[:0]
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d computeIfAbsent_f: %s\n", left[k], k)
	}
	msg := fmt.Sprintf("tnames: %d methods of TMap and TSet renamed at %d uses; %d computeIfAbsent_f calls take a typed function", decls, uses, typed)
	if report {
		fmt.Fprintln(os.Stderr, msg)
		return
	}
	fmt.Fprintf(os.Stderr, "%s; files changed: %d\n", msg, rw.SaveChecked())
}

// typedFunc rewrites m.computeIfAbsent_f(k, func(p any) any {...}) to m.ComputeIfAbsent(k,
// func(p K) V {...}); it returns why it cannot.
func typedFunc(c *ast.CallExpr, sel *ast.SelectorExpr) string {
	// The function: a literal, or a named function of the generated code that only this call uses.
	var ftype *ast.FuncType
	var body *ast.BlockStmt
	switch x := rw.Unparen(c.Args[1]).(type) {
	case *ast.FuncLit:
		ftype, body = x.Type, x.Body
	case *ast.Ident:
		o, _ := rw.Info.Uses[x].(*types.Func)
		fd := funcDecls[o]
		switch {
		case fd == nil || fd.Recv != nil || fd.Body == nil || fd.Type.TypeParams != nil:
			return "the function is not a literal or a plain named function"
		case len(rw.UsesOf[o]) != 1:
			return "the named function is used by other calls too"
		}
		ftype, body = fd.Type, fd.Body
	default:
		return "the function is not a literal or a plain named function"
	}
	if len(ftype.Params.List) != 1 || len(ftype.Params.List[0].Names) != 1 {
		return "the function does not name its one parameter"
	}
	cs := typeArgs(rw.Info.TypeOf(sel.X))
	if len(cs) != 2 {
		return "the receiver is not a typed map"
	}
	kt, vt := cs[0], cs[1]
	kc, vc := convFor(kt), convFor(vt)
	if !kc.OK {
		return "key type " + types.TypeString(kt, types.RelativeTo(rw.Pkg)) + " has no known conversion"
	}
	if !vc.OK {
		return "value type " + types.TypeString(vt, types.RelativeTo(rw.Pkg)) + " has no known conversion"
	}
	q := types.RelativeTo(rw.Pkg)
	bad := ""
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(x.Results) != 1 {
				bad = "a return without one value"
			}
		}
		return true
	})
	if bad != "" || ftype.Results == nil {
		return "the function does not return one value everywhere"
	}
	// The parameter: unboxed where the body unboxes it, boxed anywhere else.
	pid := ftype.Params.List[0].Names[0]
	pv := rw.Info.Defs[pid]
	if pid.Name != "_" {
		for _, u := range rw.UsesOf[pv] {
			u := u
			if p, ok := rw.Up(u).(*ast.CallExpr); ok && len(p.Args) == 1 && rw.Unparen(p.Args[0]) == ast.Expr(u) {
				if name, _ := rw.CallName(p); name != "" && kc.IsUnbox(name) {
					rw.Rewrite[p] = func() string { return u.Name }
					continue
				}
			}
			rw.Rewrite[u] = func() string { return kc.Box(u.Name) }
		}
	}
	// The results: unboxed as the map did.
	ast.Inspect(body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			r := x.Results[0]
			if id, ok := rw.Unparen(r).(*ast.Ident); ok && id.Name == "nil" {
				// Java's null: the map's null value (nothing is stored).
				rw.Rewrite[r] = func() string { return nullOf(vt) }
				return true
			}
			rw.Rewrite[r] = func() string {
				if it, ok := types.Unalias(vt).Underlying().(*types.Interface); ok && !it.Empty() {
					// An interface value: unboxed by its cast, which answers null for a null
					// object of any class (x itself could be a nil pointer in an interface).
					return vc.Unbox(rw.RenderChildren(r))
				}
				// obj_(x), strobj_(x), x.self() of an x that is a V: x itself.
				if call, ok := rw.Unparen(r).(*ast.CallExpr); ok {
					var inner ast.Expr
					if id, ok := call.Fun.(*ast.Ident); ok && len(call.Args) == 1 && (id.Name == "obj_" || id.Name == "strobj_" || id.Name == "any") {
						inner = call.Args[0]
					} else if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
						inner = s.X
					}
					if inner != nil && types.Identical(rw.Info.TypeOf(inner), vt) {
						return rw.Render(inner)
					}
				}
				return vc.Unbox(rw.RenderChildren(r))
			}
		}
		return true
	})
	rw.Rewrite[ftype.Params] = func() string { return "(" + pid.Name + " " + types.TypeString(kt, q) + ")" }
	rw.Rewrite[ftype.Results] = func() string { return types.TypeString(vt, q) }
	rw.Rewrite[sel.Sel] = func() string { return "ComputeIfAbsent" }
	return ""
}

func nullOf(t types.Type) string {
	if b, ok := types.Unalias(t).(*types.Basic); ok && b.Kind() == types.String {
		return "nullstr_"
	}
	return "nil"
}

func typeArgs(t types.Type) []types.Type {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || n.TypeArgs() == nil {
		return nil
	}
	out := make([]types.Type, n.TypeArgs().Len())
	for i := range out {
		out[i] = n.TypeArgs().At(i)
	}
	return out
}

// convFor is the boxing of a map's key or value type: rw.ConvFor's, for the types whose null
// is the nil the erased function returned (pointers, interfaces) and for strings.
func convFor(t types.Type) rw.Conv {
	switch u := types.Unalias(t).(type) {
	case *types.Basic:
		if u.Kind() != types.String {
			return rw.Conv{}
		}
	case *types.Pointer:
		if n, ok := types.Unalias(u.Elem()).(*types.Named); ok && strings.HasPrefix(n.Obj().Name(), "T") && n.TypeArgs() != nil {
			return rw.Conv{} // a typed collection: boxed as its view
		}
	}
	return rw.ConvFor(t)
}
