// Command gmaps is a post-flip step: an erased map or set that a generic class holds in a field
// and keys by a type parameter is a typed map or set, with the conversion of the type parameter
// taken from the instantiation.
//
//	nodes_ MapOf[N, *LinkedDiGraphNode[N, E]]                 nodes_ *TMap[N, *LinkedDiGraphNode[N, E]]
//	this.nodes_ = new_LinkedHashMap()                     ->  this.nodes_ = newTMap_(tconvT_[N](), tconvT_[*LinkedDiGraphNode[N, E]]())
//	to_LinkedDiGraphNode[N, E](this.nodes_.get(boxT_(v)))     this.nodes_.Get(v)
//
// The post-flip collection steps (colls, native) type the collections whose key and value types
// are the program's classes: the conversion of a type is a package variable (tc_X, tch_X). A
// collection keyed by a generic class's type parameter has no such variable, so it stayed an
// erased LinkedHashMap of boxes (boxT_ at every lookup, the erased table's hashing): the graph's
// node map (LinkedDirectedGraph, LinkedUndirectedGraph), StandardUnionFind's element map,
// Timeline, LinkedIdentityHashMap's inner map, the data-flow queue's seen set. The hand port
// keys them by the type parameter (jsgo: LinkedDirectedGraph[N, E comparable] with a map of
// N). The runtime's tconvT_[T]() / tconvIdT_[T]() (rt_generic.go) give the conversion of a type
// parameter's values, made once per instantiation: boxed and unboxed as the erased code did,
// compared by Go's == where that is Java's equals (or identity, for an IdentityHashMap), else
// by the hashCode and equals the erased table used. A typed map keeps insertion order as
// LinkedHashMap does (and as the runtime's IdentityHashMap does).
//
// A field qualifies when its declared type is MapOf / LinkedHashMapOf / IdentityHashMapOf /
// SetOf / LinkedHashSetOf of a key that is not Object, and its class's constructor (finit_,
// init_) is the only code that assigns it, from new_LinkedHashMap() / new_IdentityHashMap() /
// new_LinkedHashSet(). Its reads then go:
//   - get, containsKey, put, remove (a map), add, contains, remove (a set), size, isEmpty,
//     clear with a key that is a box of a value of the key type (boxT_(k), obj_(k), k.self(),
//     or a local defined as one and used only so) and a put value that is a box of a value of
//     the value type: the typed method (Get, Has, Put, Delete, Add, Len, IsEmpty, Clear) on the
//     values; a looked-up value unboxed by its caller is the value, read as an Object it is
//     boxed again (boxT_: the box the map held);
//   - a typed loop over values() or keySet() (elemsAs_ of the value or key type): Values(),
//     Keys() (the typed map's iteration, in the same order);
//   - anything else (another method, a key that is not a box, the map given to erased code):
//     through the typed map's view asMap() / asSet(), one per map, with its identity, which
//     boxes and unboxes with the same conversion.
//
// A typed map needs a comparable key type: a type parameter that keys one is declared
// comparable, and so is every type parameter that instantiates it (followed through every
// instantiation, generic types and functions); a concrete type argument must be comparable
// (pointers, interfaces, strings, handles: all of them here). Left, counted: fields assigned
// elsewhere or from another map, keys of type Object (SimpleMultiset, SimpleBiMap and the
// multimaps, which j2go erased), a key type parameter constrained by an interface (Go cannot
// add comparable to it without a new constraint type), a field given to a generic function
// whose constraint its view would not meet (tostrNN_), a type argument that is not comparable.
// A rerun finds no erased field of these kinds keyed by a type parameter.
//
// usage: gmaps [-n] [-v] <dir of package closure>
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
	leftAt  = map[string][]string{}
	verbose bool
)

func skip(why string, n ast.Node) {
	left[why]++
	leftAt[why] = append(leftAt[why], rw.Fset.Position(n.Pos()).String())
}

// cand is a qualifying field.
type cand struct {
	f      *types.Var
	set    bool
	id     bool // IdentityHashMap
	k, v   types.Type
	kx, vx ast.Expr // the type argument expressions of the declaration
	decl   *ast.Field
	alias  string
	ctor   []*ast.CallExpr
	view   string // asMap or asSet
	t2     bool   // reflip48: declared MapT[K, V], assigned mapAs_(kc, vc, Map(new_X()))
}

func typeOf(e ast.Expr) types.Type { return rw.Info.Types[e].Type }

func isAny(t types.Type) bool {
	i, ok := types.Unalias(t).(*types.Interface)
	return ok && i.Empty()
}

var aliasKinds = map[string]string{"MapOf": "map", "LinkedHashMapOf": "map", "IdentityHashMapOf": "idmap", "SetOf": "set", "LinkedHashSetOf": "set", "MapT": "map"} // reflip48: MapT
var ctorOf = map[string]string{"map": "new_LinkedHashMap", "idmap": "new_IdentityHashMap", "set": "new_LinkedHashSet"}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	cands := map[*types.Var]*cand{}
	// 1. fields of generic classes
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, s := range gd.Specs {
				ts := s.(*ast.TypeSpec)
				st, ok := ts.Type.(*ast.StructType)
				if !ok || ts.TypeParams == nil {
					continue
				}
				for _, fld := range st.Fields.List {
					if len(fld.Names) != 1 {
						continue
					}
					alias, args := aliasOf(fld.Type)
					kind := aliasKinds[alias]
					if kind == "" {
						continue
					}
					v := rw.Info.Defs[fld.Names[0]].(*types.Var)
					c := &cand{f: v, set: kind == "set", id: kind == "idmap", decl: fld, alias: alias, view: "asMap"}
					if alias == "MapT" { // reflip48
						c.t2, c.view = true, "asMapT"
					}
					if c.set {
						c.view = "asSet"
					}
					c.kx = args[0]
					c.k = typeOf(args[0])
					if !c.set {
						c.vx = args[1]
						c.v = typeOf(args[1])
					}
					if isAny(c.k) {
						skip("keyed by Object", fld)
						continue
					}
					cands[v] = c
				}
			}
		}
	}
	// 2. assignments: only the constructor, from the erased constructor of the kind
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			as, ok := x.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, l := range as.Lhs {
				s, ok := l.(*ast.SelectorExpr)
				if !ok {
					continue
				}
				v, _ := rw.Info.Uses[s.Sel].(*types.Var)
				if v == nil {
					continue
				}
				c := cands[v.Origin()]
				if c == nil {
					continue
				}
				fd := enclosingDecl(as)
				ok1 := fd != nil && fd.Recv != nil && (strings.HasPrefix(fd.Name.Name, "finit_") || strings.HasPrefix(fd.Name.Name, "init_"))
				call, ok2 := as.Rhs[i].(*ast.CallExpr)
				kind := "map"
				if c.set {
					kind = "set"
				} else if c.id {
					kind = "idmap"
				}
				if c.t2 && ok1 && ok2 { // reflip48: mapAs_(kc, vc, Map(new_IdentityHashMap())) / Map(new_LinkedHashMap())
					if k := reflip48Ctor(call); k != "" && (len(c.ctor) == 0 || c.id == (k == "idmap")) {
						c.id = k == "idmap"
						c.ctor = append(c.ctor, call)
						continue
					}
				}
				if !ok1 || !ok2 || len(call.Args) != 0 || funName(call) != ctorOf[kind] {
					skip("assigned outside the constructor or from another map", as)
					delete(cands, v.Origin())
					continue
				}
				c.ctor = append(c.ctor, call)
			}
			return true
		})
	}
	for v, c := range cands {
		if len(c.ctor) == 0 {
			skip("never assigned a new map", c.decl)
			delete(cands, v)
		}
	}
	// a field given as such to a generic function (its view would not meet the constraint)
	for v, c := range cands {
		for _, u := range rw.UsesOf[v] {
			sel, ok := rw.Parent[u].(*ast.SelectorExpr)
			if !ok {
				continue
			}
			call, ok := rw.Up(sel).(*ast.CallExpr)
			if !ok || call.Fun == ast.Expr(sel) {
				continue
			}
			if id, ok := call.Fun.(*ast.Ident); ok {
				if fn, ok := rw.Info.Uses[id].(*types.Func); ok && fn.Type().(*types.Signature).TypeParams().Len() > 0 && fn.Name() != "obj_" {
					skip("given to a generic function ("+fn.Name()+")", c.decl)
					delete(cands, v)
					break
				}
			}
		}
	}
	for v := range cands {
		candFields[v] = true
	}
	// 3. comparable keys: the type parameters to declare comparable
	need := map[*types.TypeParam]bool{}
	for _, c := range cands {
		mine := map[*types.TypeParam]bool{}
		if !markComparable(c.k, mine) || !cascade(mine) {
			skip("a key type that cannot be comparable (an instantiation with a type that is not)", c.decl)
			delete(cands, c.f)
			continue
		}
		if why := constraintsAny(mine); why != "" {
			skip(why, c.decl)
			delete(cands, c.f)
			continue
		}
		for tp := range mine {
			need[tp] = true
			if os.Getenv("GMAPS_DEBUG") != "" {
				fmt.Fprintf(os.Stderr, "need %s %s (for %s)\n", tp.Obj().Name(), rw.Fset.Position(tp.Obj().Pos()), c.f.Name())
			}
		}
	}
	// 4. rewrites
	for _, c := range cands {
		convert(c)
	}
	rewriteConstraints(need)
	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
		if verbose {
			for _, p := range leftAt[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	keys = keys[:0]
	for k := range done {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", done[k], k)
	}
	if report {
		fmt.Fprintln(os.Stderr, "gmaps: dry run, nothing written")
		return
	}
	fmt.Fprintf(os.Stderr, "gmaps: files changed: %d\n", rw.SaveChecked())
}

// aliasOf: a type expression A[X, Y] of an alias A: its name and arguments.
func aliasOf(e ast.Expr) (string, []ast.Expr) {
	switch x := e.(type) {
	case *ast.IndexExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			return id.Name, []ast.Expr{x.Index}
		}
	case *ast.IndexListExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			return id.Name, x.Indices
		}
	}
	return "", nil
}

func funName(call *ast.CallExpr) string {
	f := call.Fun
	switch x := f.(type) {
	case *ast.IndexExpr:
		f = x.X
	case *ast.IndexListExpr:
		f = x.X
	}
	switch x := f.(type) {
	case *ast.Ident:
		return x.Name
	case *ast.SelectorExpr:
		return x.Sel.Name
	}
	return ""
}

func enclosingDecl(n ast.Node) *ast.FuncDecl {
	for p := ast.Node(n); p != nil; p = rw.Parent[p] {
		if fd, ok := p.(*ast.FuncDecl); ok {
			return fd
		}
	}
	return nil
}

// same reports whether a and b are the same type, a receiver's type parameters being the ones
// of the type's declaration (each method of a generic type has its own).
func same(a, b types.Type) bool {
	if a == nil || b == nil {
		return false
	}
	if types.Identical(a, b) {
		return true
	}
	return norm(a) == norm(b)
}

func norm(t types.Type) string {
	if len(declTP) == 0 {
		indexTypeParams()
	}
	switch x := t.(type) {
	case *types.TypeParam:
		d := declTP[x]
		if d == nil {
			d = x
		}
		return fmt.Sprintf("TP%p", d)
	case *types.Pointer:
		return "*" + norm(x.Elem())
	case *types.Slice:
		return "[]" + norm(x.Elem())
	case *types.Named:
		s := x.Obj().Pkg().Path() + "." + x.Obj().Name()
		if x.TypeArgs().Len() > 0 {
			var as []string
			for i := 0; i < x.TypeArgs().Len(); i++ {
				as = append(as, norm(x.TypeArgs().At(i)))
			}
			s += "[" + strings.Join(as, ",") + "]"
		}
		return s
	case *types.Alias:
		return norm(types.Unalias(x))
	}
	return types.TypeString(t, nil)
}

// ---- comparable

// declTP maps a type parameter (a receiver's, or a type's or function's own) to the one its
// declaration lists (where the constraint is written).
var declTP = map[*types.TypeParam]*types.TypeParam{}
var tpField = map[*types.TypeParam]*ast.Field{}
var tpIdent = map[*types.TypeParam]*ast.Ident{}

func indexTypeParams() {
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.GenDecl:
				for _, s := range x.Specs {
					if ts, ok := s.(*ast.TypeSpec); ok && ts.TypeParams != nil {
						for _, fl := range ts.TypeParams.List {
							for _, nm := range fl.Names {
								if tn, ok := rw.Info.Defs[nm].(*types.TypeName); ok {
									tp := tn.Type().(*types.TypeParam)
									declTP[tp], tpField[tp], tpIdent[tp] = tp, fl, nm
								}
							}
						}
					}
				}
			case *ast.FuncDecl:
				if x.Type.TypeParams != nil {
					for _, fl := range x.Type.TypeParams.List {
						for _, nm := range fl.Names {
							if tn, ok := rw.Info.Defs[nm].(*types.TypeName); ok {
								tp := tn.Type().(*types.TypeParam)
								declTP[tp], tpField[tp], tpIdent[tp] = tp, fl, nm
							}
						}
					}
				}
				if x.Recv != nil && len(x.Recv.List) == 1 {
					rt := x.Recv.List[0].Type
					if s, ok := rt.(*ast.StarExpr); ok {
						rt = s.X
					}
					var ids []ast.Expr
					var base *ast.Ident
					switch r := rt.(type) {
					case *ast.IndexExpr:
						base, _ = r.X.(*ast.Ident)
						ids = []ast.Expr{r.Index}
					case *ast.IndexListExpr:
						base, _ = r.X.(*ast.Ident)
						ids = r.Indices
					}
					if base == nil {
						continue
					}
					tn, _ := rw.Info.Uses[base].(*types.TypeName)
					if tn == nil {
						continue
					}
					named := tn.Type().(*types.Named)
					for i, e := range ids {
						id, ok := e.(*ast.Ident)
						if !ok {
							continue
						}
						if rtn, ok := rw.Info.Defs[id].(*types.TypeName); ok {
							if rtp, ok := rtn.Type().(*types.TypeParam); ok && i < named.TypeParams().Len() {
								declTP[rtp] = named.TypeParams().At(i)
							}
						}
					}
				}
			}
		}
	}
}

// markComparable adds to need the type parameters t is (a key type must be comparable); false
// if t cannot be.
func markComparable(t types.Type, need map[*types.TypeParam]bool) bool {
	if declTP == nil || len(declTP) == 0 {
		indexTypeParams()
	}
	if tp, ok := t.(*types.TypeParam); ok {
		d := declTP[tp]
		if d == nil {
			return false
		}
		need[d] = true
		return true
	}
	return types.Comparable(t)
}

// cascade: every instantiation that gives a type parameter that must be comparable a type
// argument makes that argument comparable too.
func cascade(need map[*types.TypeParam]bool) bool {
	ok := true
	for changed := true; changed; {
		changed = false
		for id, inst := range rw.Info.Instances {
			o := rw.Info.Uses[id]
			var tps *types.TypeParamList
			switch x := o.(type) {
			case *types.TypeName:
				if n, isN := x.Type().(*types.Named); isN {
					tps = n.Origin().TypeParams()
				}
			case *types.Func:
				tps = x.Origin().Type().(*types.Signature).TypeParams()
				if tps == nil || tps.Len() == 0 {
					if r := x.Origin().Type().(*types.Signature).Recv(); r != nil {
						continue // a method: its receiver's instantiation is listed on the type
					}
				}
			}
			if tps == nil {
				continue
			}
			for i := 0; i < tps.Len() && i < inst.TypeArgs.Len(); i++ {
				if !need[declTP[tps.At(i)]] && !need[tps.At(i)] {
					continue
				}
				arg := inst.TypeArgs.At(i)
				if tp, isTP := arg.(*types.TypeParam); isTP {
					d := declTP[tp]
					if d == nil {
						ok = false
						continue
					}
					if !need[d] {
						need[d] = true
						changed = true
					}
					continue
				}
				if !types.Comparable(arg) {
					ok = false
				}
			}
		}
	}
	return ok
}

// constraintsAny says why the declarations of tps cannot be made comparable ("" if each is any
// or comparable already).
func constraintsAny(tps map[*types.TypeParam]bool) string {
	for tp := range tps {
		fl := tpField[tp]
		if fl == nil {
			return "a type parameter whose declaration is not found"
		}
		if con, ok := fl.Type.(*ast.Ident); !ok || con.Name != "any" && con.Name != "comparable" {
			return "a key type parameter (or one that instantiates it) is constrained by an interface"
		}
	}
	return ""
}

func rewriteConstraints(need map[*types.TypeParam]bool) {
	byField := map[*ast.Field][]*ast.Ident{}
	for tp := range need {
		fl := tpField[tp]
		if fl == nil {
			continue
		}
		byField[fl] = append(byField[fl], tpIdent[tp])
	}
	for fl, ids := range byField {
		con, ok := fl.Type.(*ast.Ident)
		if !ok || con.Name != "any" && con.Name != "comparable" {
			fmt.Fprintf(os.Stderr, "gmaps: %s: a constraint that is not any: %s\n", rw.Fset.Position(fl.Pos()), rw.Text(fl.Type))
			os.Exit(1)
		}
		if con.Name == "comparable" {
			continue
		}
		want := map[*ast.Ident]bool{}
		for _, id := range ids {
			want[id] = true
		}
		fl := fl
		var parts []string
		var cur []string
		curC := ""
		flush := func() {
			if len(cur) > 0 {
				parts = append(parts, strings.Join(cur, ", ")+" "+curC)
			}
			cur = nil
		}
		for _, nm := range fl.Names {
			c := "any"
			if want[nm] {
				c = "comparable"
			}
			if c != curC {
				flush()
				curC = c
			}
			cur = append(cur, nm.Name)
		}
		flush()
		txt := strings.Join(parts, ", ")
		rw.Rewrite[fl] = func() string { return txt }
		done["type parameters declared comparable"] += len(ids)
	}
}

// ---- rewriting a field

func convert(c *cand) {
	kx := rw.Text(c.kx)
	var tm, ctor string
	switch {
	case c.set:
		tm = "*TSet[" + kx + "]"
		ctor = "newTSet_(tconvT_[" + kx + "]())"
	case c.id:
		tm = "*TMap[" + kx + ", " + rw.Text(c.vx) + "]"
		ctor = "newTMap_(tconvIdT_[" + kx + "](), tconvT_[" + rw.Text(c.vx) + "]())"
	default:
		tm = "*TMap[" + kx + ", " + rw.Text(c.vx) + "]"
		ctor = "newTMap_(tconvT_[" + kx + "](), tconvT_[" + rw.Text(c.vx) + "]())"
	}
	rw.Rewrite[c.decl.Type] = func() string { return tm }
	for _, call := range c.ctor {
		rw.Rewrite[call] = func() string { return ctor }
	}
	done[c.alias+" fields -> typed"]++
	for _, u := range rw.UsesOf[c.f] {
		sel, ok := rw.Parent[u].(*ast.SelectorExpr)
		if !ok || sel.Sel != u {
			continue
		}
		if as, ok := rw.Parent[sel].(*ast.AssignStmt); ok {
			isL := false
			for _, l := range as.Lhs {
				isL = isL || l == ast.Expr(sel)
			}
			if isL {
				continue
			}
		}
		read(c, sel)
	}
}

// keyArg is the code of a lookup key a given to the typed map, or "".
func keyArg(c *cand, a ast.Expr, t types.Type) func() string {
	a = rw.Unparen(a)
	if b := boxed(a); b != nil {
		if bt := typeOf(b); bt != nil && same(bt, t) {
			return func() string { return rw.Render(b) }
		}
		return nil
	}
	if at := typeOf(a); at != nil && !isAny(at) && same(at, t) {
		return func() string { return rw.Render(a) }
	}
	// a local defined as a box of a value of t and used only as a key of typed calls
	if id, ok := a.(*ast.Ident); ok {
		if v, ok := rw.Info.Uses[id].(*types.Var); ok {
			if b := boxedLocal(v, t); b != nil {
				return func() string { return rw.Render(a) }
			}
		}
	}
	return nil
}

// boxedLocals: locals defined as a box (key := boxT_(x)) whose definition becomes the value.
var boxedLocals = map[*types.Var]ast.Expr{}

func boxedLocal(v *types.Var, t types.Type) ast.Expr {
	if b, ok := boxedLocals[v]; ok {
		return b
	}
	var def *ast.AssignStmt
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			if as, ok := x.(*ast.AssignStmt); ok && as.Tok == token.DEFINE && len(as.Lhs) == 1 {
				if id, ok := as.Lhs[0].(*ast.Ident); ok && rw.Info.Defs[id] == v {
					def = as
				}
			}
			return def == nil
		})
		if def != nil {
			break
		}
	}
	if def == nil {
		return nil
	}
	b := boxed(def.Rhs[0])
	if b == nil || !same(typeOf(b), t) {
		return nil
	}
	// every use: an argument of a call on a candidate field (checked by the caller as a key)
	for _, u := range rw.UsesOf[v] {
		call, ok := rw.Parent[u].(*ast.CallExpr)
		if !ok || len(call.Args) == 0 || call.Args[0] != ast.Expr(u) {
			return nil
		}
		s, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		inner, ok := s.X.(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		fv, _ := rw.Info.Uses[inner.Sel].(*types.Var)
		if fv == nil || !isCandField(fv.Origin()) {
			return nil
		}
		switch s.Sel.Name {
		case "get", "containsKey", "put", "remove", "remove_Object", "add", "contains":
		default:
			return nil
		}
	}
	boxedLocals[v] = b
	rw.Rewrite[def.Rhs[0]] = func() string { return rw.Render(b) }
	done["locals holding a boxed key: the key"]++
	return b
}

var candFields = map[*types.Var]bool{}

func isCandField(v *types.Var) bool { return candFields[v] }

func boxed(e ast.Expr) ast.Expr {
	call, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil
	}
	name := funName(call)
	if len(call.Args) == 1 && (name == "obj_" || name == "strobj_" || name == "boxT_") {
		if _, isSel := call.Fun.(*ast.SelectorExpr); !isSel {
			return call.Args[0]
		}
	}
	if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
		return s.X
	}
	return nil
}

// readV: the typed call's value (of type v) where the erased call's Object was read.
func readV(call *ast.CallExpr, v types.Type, code func() string) {
	p := rw.Up(call)
	if pc, ok := p.(*ast.CallExpr); ok && len(pc.Args) == 1 && rw.Unparen(pc.Args[0]) == ast.Expr(call) {
		name := funName(pc)
		if pt := typeOf(pc); pt != nil && same(pt, v) && (strings.HasPrefix(name, "to_") || name == "cast_" || name == "junboxT_" || name == "unboxT_") {
			rw.Rewrite[pc] = code
			done["unboxings of looked-up values dropped"]++
			return
		}
	}
	if _, ok := p.(*ast.ExprStmt); ok {
		rw.Rewrite[call] = code
		return
	}
	rw.Rewrite[call] = func() string { return "boxT_(" + code() + ")" }
	done["looked-up values read as Object: boxed again"]++
}

func read(c *cand, sel *ast.SelectorExpr) {
	view := func(why string) {
		if os.Getenv("GMAPS_DEBUG") != "" {
			fmt.Fprintf(os.Stderr, "view %s: %s: %s\n", why, rw.Fset.Position(sel.Pos()), rw.Text(rw.Up(rw.Up(sel))))
		}
		rw.Rewrite[sel] = func() string { return rw.RenderChildren(sel) + "." + c.view + "()" }
		done["reads through the view ("+why+")"]++
	}
	// icast_Map(obj_(X.f)) -> X.f.asMap()
	if ob, ok := rw.Up(sel).(*ast.CallExpr); ok && boxed(ob) == ast.Expr(sel) {
		if ic, ok := rw.Up(ob).(*ast.CallExpr); ok && (funName(ic) == "icast_Map" || funName(ic) == "icast_Set") {
			rw.Rewrite[ic] = func() string { return rw.RenderChildren(sel) + "." + c.view + "()" }
			done["reads through the view (given to erased code)"]++
			return
		}
	}
	m, ok := rw.Up(sel).(*ast.SelectorExpr)
	if !ok || m.X != ast.Expr(sel) {
		view("given to erased code")
		return
	}
	call, ok := rw.Up(m).(*ast.CallExpr)
	if !ok || call.Fun != ast.Expr(m) {
		view("a method value")
		return
	}
	recv := func() string { return rw.RenderChildren(sel) }
	ck, cv := c.k, c.v
	if a, ok := typeOf(sel).(*types.Alias); ok && a.TypeArgs().Len() >= 1 {
		ck = a.TypeArgs().At(0)
		if !c.set && a.TypeArgs().Len() >= 2 {
			cv = a.TypeArgs().At(1)
		}
	}
	args := call.Args
	typed := func(name string, codeArgs ...func() string) func() string {
		return func() string {
			var as []string
			for _, a := range codeArgs {
				as = append(as, a())
			}
			return recv() + "." + name + "(" + strings.Join(as, ", ") + ")"
		}
	}
	nm := m.Sel.Name
	if c.t2 { // reflip48: the typed interface's get_t / put_t, whose values are typed already
		switch {
		case nm == "get_t" && len(args) == 1:
			if k := keyArg(c, args[0], ck); k != nil {
				rw.Rewrite[call] = typed("Get", k)
				done["get_t -> Get"]++
				return
			}
		case nm == "put_t" && len(args) == 2:
			k := keyArg(c, args[0], ck)
			v := keyArg(c, args[1], cv)
			if k != nil && v != nil {
				rw.Rewrite[call] = typed("Put", k, v)
				done["put_t -> Put"]++
				return
			}
		}
	}
	switch {
	case !c.set && nm == "get" && len(args) == 1:
		if k := keyArg(c, args[0], ck); k != nil {
			readV(call, cv, typed("Get", k))
			done["get -> Get"]++
			return
		}
	case !c.set && nm == "containsKey" && len(args) == 1, c.set && nm == "contains" && len(args) == 1:
		if k := keyArg(c, args[0], ck); k != nil {
			rw.Rewrite[call] = typed("Has", k)
			done[nm+" -> Has"]++
			return
		}
	case !c.set && nm == "put" && len(args) == 2:
		k := keyArg(c, args[0], ck)
		v := keyArg(c, args[1], cv)
		if k != nil && v != nil {
			readV(call, cv, typed("Put", k, v))
			done["put -> Put"]++
			return
		}
	case !c.set && (nm == "remove" || nm == "remove_Object") && len(args) == 1:
		if k := keyArg(c, args[0], ck); k != nil {
			readV(call, cv, typed("Delete", k))
			done["remove -> Delete"]++
			return
		}
	case c.set && nm == "add" && len(args) == 1:
		if k := keyArg(c, args[0], ck); k != nil {
			rw.Rewrite[call] = typed("Add", k)
			done["add -> Add"]++
			return
		}
	case c.set && (nm == "remove" || nm == "remove_Object") && len(args) == 1:
		if k := keyArg(c, args[0], ck); k != nil {
			rw.Rewrite[call] = typed("Delete", k)
			done["remove -> Delete"]++
			return
		}
	case nm == "size" && len(args) == 0:
		rw.Rewrite[call] = typed("Len")
		done["size -> Len"]++
		return
	case nm == "isEmpty" && len(args) == 0:
		rw.Rewrite[call] = typed("IsEmpty")
		done["isEmpty -> IsEmpty"]++
		return
	case nm == "clear" && len(args) == 0:
		rw.Rewrite[call] = typed("Clear")
		done["clear -> Clear"]++
		return
	case (nm == "values" || nm == "keySet") && len(args) == 0 && !c.set:
		// elemsAs_[T](X.f.values()) of the value (key) type in a range: Values() (Keys())
		if ec, ok := rw.Up(call).(*ast.CallExpr); ok && len(ec.Args) == 1 && funName(ec) == "elems_" {
			// an untyped loop: the loop variable is the value; its unboxings go, other reads box it
			want, it := cv, "Values"
			if nm == "keySet" {
				want, it = ck, "Keys"
			}
			if rs, inRange := rw.Up(ec).(*ast.RangeStmt); inRange && rs.Value == nil && rs.Key != nil {
				if kid, ok := rs.Key.(*ast.Ident); ok {
					if kv, ok := rw.Info.Defs[kid].(*types.Var); ok {
						rw.Rewrite[ec] = typed(it)
						for _, u := range rw.UsesOf[kv] {
							u := u
							if pc, ok := rw.Up(u).(*ast.CallExpr); ok && len(pc.Args) == 1 && rw.Unparen(pc.Args[0]) == ast.Expr(u) {
								name := funName(pc)
								if same(typeOf(pc), want) && (strings.HasPrefix(name, "to_") || name == "cast_" || name == "junboxT_" || name == "unboxT_") {
									rw.Rewrite[pc] = func() string { return u.Name }
									done["unboxings of loop values dropped"]++
									continue
								}
							}
							rw.Rewrite[u] = func() string { return "boxT_(" + u.Name + ")" }
							done["loop values read as Object: boxed again"]++
						}
						done["untyped loops over "+nm+"() -> "+it+"()"]++
						return
					}
				}
			}
		}
		if ec, ok := rw.Up(call).(*ast.CallExpr); ok && len(ec.Args) == 1 && funName(ec) == "elemsAs_" {
			if ix, ok := ec.Fun.(*ast.IndexExpr); ok {
				want, it := cv, "Values"
				if nm == "keySet" {
					want, it = ck, "Keys"
				}
				if same(typeOf(ix.Index), want) {
					if _, inRange := rw.Up(ec).(*ast.RangeStmt); inRange {
						rw.Rewrite[ec] = typed(it)
						done["typed loops over "+nm+"() -> "+it+"()"]++
						return
					}
				}
			}
		}
	}
	view(nm)
}

// reflip48Ctor: call is mapAs_(kc, vc, Map(new_IdentityHashMap())) ("idmap") or of Map(new_LinkedHashMap()) ("map").
func reflip48Ctor(call *ast.CallExpr) string {
	if funName(call) != "mapAs_" || len(call.Args) != 3 {
		return ""
	}
	m, ok := rw.Unparen(call.Args[2]).(*ast.CallExpr)
	if !ok || funName(m) != "Map" || len(m.Args) != 1 {
		return ""
	}
	n, ok := rw.Unparen(m.Args[0]).(*ast.CallExpr)
	if !ok || len(n.Args) != 0 {
		return ""
	}
	switch funName(n) {
	case "new_IdentityHashMap":
		return "idmap"
	case "new_LinkedHashMap":
		return "map"
	}
	return ""
}
