// Command colls is post-flip steps 3-5: lists, maps and sets become typed collections
// (*TList[T], *TMap[K, V], *TSet[K]; rt_tlist.go, rt_tmap.go) across method boundaries.
//
// Every place that can hold a collection of the kind (a variable, field, parameter or result
// declared in generated code) is a slot. Slots that exchange values (assignment, argument
// passing, return, comparison) are joined into classes; methods are joined across overriding
// by name and signature, so that interfaces stay satisfied. A class becomes typed when
//   - every value that enters it is a new collection, a copy of one, an immutable one, nil,
//     or comes from another slot of the class,
//   - its slots that state element types agree on them (for covariant overrides, on the type
//     all stated ones are assignable to); map and set keys must compare in Go as in Java, and
//   - no slot of it belongs to code that cannot change (the runtime, library stubs) or is used
//     in a way this tool does not understand.
//
// The common operations become typed calls; any other method, and any value that leaves for
// code expecting an erased collection or Object, goes through a view (asList, asMap, asSet)
// with the collection's identity.
//
// -kind deque: an ArrayDeque used only as a stack (push, pop, peek, poll, getFirst, size,
// isEmpty, clear; also addFirst, removeFirst...) becomes a *TList[T] whose end is the deque's
// head (its order reversed: a for-each loop walks it from the end, eachRev), or a deque used
// at its tail (add, removeLast, getLast..., and a queue: add, poll) a *TList[T] in its own
// order. A deque pushed at its head and used at its tail, or seen as an erased collection,
// stays erased.
//
// usage: colls -kind list|map|set|deque [-n] [-v] <dir of package closure>
//
//	(-n: report only; -v: list the classes)
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"postflip/rw"
)

// ---- union-find over slots

type class struct {
	elem       []types.Type // element types: T of a list or set; K, V of a map
	elemText   []string
	cv         []rw.Conv
	poison     []string
	elems      [][]ast.Expr // stated element types (alias arguments), per type argument
	typeExpr   []ast.Expr   // declarations to rewrite to *TList[T]
	edits      []func(c *class) rw.Edit
	slots      int
	strKey     bool   // used through a string-key helper: the key must be a string
	hashKey    bool   // keys compare by Java's hashCode/equals, not by Go's ==
	dqHead     string // a deque used at its head (push, pop, peek...): the first such use
	dqTail     string // a deque used at its tail (add, removeLast...): the first such use
	dqIter     bool   // a deque iterated
	dqHeadPush string // a deque pushed at its head: the first such use
	anyAt      string // where a value is boxed into an Object, if anywhere
}

// ---- kinds

type kindDef struct {
	name      string
	held      []types.Type // the erased types that hold a collection of the kind
	aliases   []string
	nargs     int
	conv      []string // conversions that keep the object
	newFns    map[string]int
	copyFns   map[string]bool
	immutable map[string]bool
	view      string
	helpers   map[string]string
	goType    func(c *class) string
	newCode   func(c *class, cap ast.Expr) string
	fromCode  func(c *class, from string, imm bool) string
}

var K *kindDef

// converted are the classes that become typed (root key -> class), known when edits render.
var converted = map[int]*class{}

func kinds() map[string]*kindDef {
	look := func(n string) types.Type { return rw.Pkg.Scope().Lookup(n).Type() }
	ptr := func(n string) types.Type { return types.NewPointer(look(n)) }
	return map[string]*kindDef{
		"list": {
			name: "list", held: []types.Type{look("List"), ptr("ArrayList")}, aliases: []string{"ListOf", "ArrayListOf"}, nargs: 1,
			conv:      []string{"Collections_unmodifiableList", "icast_List", "to_ArrayList", "cast_[*ArrayList]", "List"},
			newFns:    map[string]int{"new_ArrayList_0": 0, "new_ArrayList_int": 1},
			copyFns:   map[string]bool{"new_ArrayList_Collection": true},
			immutable: map[string]bool{"ImmutableList": true, "ImmutableCollection": true},
			view:      "asList",
			goType:    func(c *class) string { return "*TList[" + c.elemText[0] + "]" },
			newCode: func(c *class, n ast.Expr) string {
				if n == nil {
					return "newTList_(" + c.convName(0) + ")"
				}
				return "newTListN_(" + c.convName(0) + ", " + rw.Render(n) + ")"
			},
			fromCode: func(c *class, from string, imm bool) string {
				if imm {
					return "tlistImm_(" + c.convName(0) + ", " + from + ")"
				}
				return "tlistFrom_(" + c.convName(0) + ", " + from + ")"
			},
		},
		"deque": {
			// An ArrayDeque used only as a stack: *TList[T] whose end is the deque's head.
			name: "deque", held: []types.Type{look("Deque"), ptr("ArrayDeque")}, aliases: []string{"DequeOf", "ArrayDequeOf"}, nargs: 1,
			conv:      []string{"icast_Deque", "to_ArrayDeque", "cast_[*ArrayDeque]", "Deque"},
			newFns:    map[string]int{"new_ArrayDeque": 0, "new_ArrayDeque_int": 1},
			copyFns:   map[string]bool{},
			immutable: map[string]bool{},
			goType:    func(c *class) string { return "*TList[" + c.elemText[0] + "]" },
			newCode: func(c *class, n ast.Expr) string {
				if n == nil {
					return "newTList_(" + c.convName(0) + ")"
				}
				return "newTListN_(" + c.convName(0) + ", " + rw.Render(n) + ")"
			},
		},
		"map": {
			name: "map", held: []types.Type{look("Map"), ptr("LinkedHashMap")}, aliases: []string{"MapOf", "LinkedHashMapOf"}, nargs: 2,
			conv:      []string{"Collections_unmodifiableMap", "icast_Map", "to_LinkedHashMap", "cast_[*LinkedHashMap]", "Map"},
			newFns:    map[string]int{"new_LinkedHashMap": 0, "Maps_newLinkedHashMapWithExpectedSize": 1},
			copyFns:   map[string]bool{"new_LinkedHashMap_Map": true},
			immutable: map[string]bool{"ImmutableMap": true},
			view:      "asMap",
			helpers: map[string]string{"Map_get_s": "get", "Map_put_s": "put", "Map_containsKey_s": "containsKey", "Map_remove_s": "remove",
				"Map_getOrDefault_s": "getOrDefault", "Map_putIfAbsent_s": "putIfAbsent", "Map_computeIfAbsent_s": "computeIfAbsent"},
			goType:  func(c *class) string { return "*TMap[" + c.elemText[0] + ", " + c.elemText[1] + "]" },
			newCode: func(c *class, n ast.Expr) string { return "newTMap_(" + c.convName(0) + ", " + c.convName(1) + ")" },
			fromCode: func(c *class, from string, imm bool) string {
				return fmt.Sprintf("tmapFrom_(%s, %s, %s, %v)", c.convName(0), c.convName(1), from, imm)
			},
		},
		"set": {
			name: "set", held: []types.Type{look("Set"), ptr("LinkedHashSet")}, aliases: []string{"SetOf", "LinkedHashSetOf"}, nargs: 1,
			conv:      []string{"Collections_unmodifiableSet", "icast_Set", "to_LinkedHashSet", "cast_[*LinkedHashSet]", "Set"},
			newFns:    map[string]int{"new_LinkedHashSet": 0, "Sets_newLinkedHashSetWithExpectedSize": 1},
			copyFns:   map[string]bool{"new_LinkedHashSet_Collection": true},
			immutable: map[string]bool{"ImmutableSet": true, "ImmutableCollection": true},
			view:      "asSet",
			helpers:   map[string]string{"Collection_contains_s": "contains", "Collection_add_s": "add", "Collection_remove_s": "remove"},
			goType:    func(c *class) string { return "*TSet[" + c.elemText[0] + "]" },
			newCode:   func(c *class, n ast.Expr) string { return "newTSet_(" + c.convName(0) + ")" },
			fromCode: func(c *class, from string, imm bool) string {
				return fmt.Sprintf("tsetFrom_(%s, %s, %v)", c.convName(0), from, imm)
			},
		},
	}
}

var (
	parent = map[int]int{}
	data   = map[int]*class{}
	keys   = map[string]int{}
	nkeys  int
)

func key(s string) int {
	if k, ok := keys[s]; ok {
		return k
	}
	nkeys++
	keys[s] = nkeys
	parent[nkeys] = nkeys
	data[nkeys] = &class{slots: 1, elems: make([][]ast.Expr, K.nargs)}
	return nkeys
}

func find(k int) int {
	for parent[k] != k {
		parent[k] = parent[parent[k]]
		k = parent[k]
	}
	return k
}

func union(a, b int) {
	a, b = find(a), find(b)
	if a == b {
		return
	}
	parent[b] = a
	da, db := data[a], data[b]
	da.poison = append(da.poison, db.poison...)
	for i := range db.elems {
		da.elems[i] = append(da.elems[i], db.elems[i]...)
	}
	da.typeExpr = append(da.typeExpr, db.typeExpr...)
	da.edits = append(da.edits, db.edits...)
	da.slots += db.slots
	da.strKey = da.strKey || db.strKey
	if da.dqHead == "" {
		da.dqHead = db.dqHead
	}
	if da.dqTail == "" {
		da.dqTail = db.dqTail
	}
	da.dqIter = da.dqIter || db.dqIter
	if da.dqHeadPush == "" {
		da.dqHeadPush = db.dqHeadPush
	}
	if da.anyAt == "" {
		da.anyAt = db.anyAt
	}
	delete(data, b)
}

func cls(k int) *class { return data[find(k)] }

var poisonAt = map[string]string{}

func poison(k int, why string) {
	c := cls(k)
	c.poison = append(c.poison, why)
	if _, ok := poisonAt[why]; !ok && curPos.IsValid() {
		p := rw.Fset.Position(curPos)
		poisonAt[why] = fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
	}
}

var curPos token.Pos

// ctorDest is the slot each constructor of the kind (a new or copied collection) flows into;
// ctorWhy says why one flows into none.
var (
	ctorDest = map[ast.Expr]int{}
	ctorWhy  = map[ast.Expr]string{}
)

func edit(k int, f func(c *class) rw.Edit) { c := cls(k); c.edits = append(c.edits, f) }

// ---- types and files

// isList reports whether t holds a collection of the kind being converted.
func isList(t types.Type) bool {
	if t == nil {
		return false
	}
	t = types.Unalias(t)
	for _, k := range K.held {
		if types.Identical(t, k) {
			return true
		}
	}
	return false
}

func typeOf(e ast.Expr) types.Type {
	if tv, ok := rw.Info.Types[e]; ok {
		return tv.Type
	}
	return nil
}

func generated(p token.Pos) bool {
	b := filepath.Base(rw.Fset.Position(p).Filename)
	return strings.HasPrefix(b, "g_") && b != "g_zstubs.go" && b != "g_0aliases.go"
}

// ---- slot keys

func varKey(v *types.Var) int { return key(fmt.Sprintf("v|%d", v.Pos())) }

var funcKeys = map[*types.Func]string{}

func groupOf(f *types.Func) string {
	if g, ok := funcKeys[f]; ok {
		return g
	}
	sig := f.Type().(*types.Signature)
	var g string
	if sig.Recv() == nil {
		g = "f|" + f.Name()
	} else {
		var ps []string
		for i := 0; i < sig.Params().Len(); i++ {
			ps = append(ps, types.TypeString(types.Unalias(sig.Params().At(i).Type()), nil))
		}
		var rs []string
		for i := 0; i < sig.Results().Len(); i++ {
			rs = append(rs, types.TypeString(types.Unalias(sig.Results().At(i).Type()), nil))
		}
		g = "m|" + strings.TrimSuffix(f.Name(), "_impl") + "(" + strings.Join(ps, ",") + ")" + strings.Join(rs, ",")
	}
	funcKeys[f] = g
	return g
}

func paramKey(g string, i int) int  { return key(fmt.Sprintf("%s:p%d", g, i)) }
func resultKey(g string, i int) int { return key(fmt.Sprintf("%s:r%d", g, i)) }

// declare registers the parameters and results of a function type as slots of group g.
var groupFixed = map[string][]string{}
var groupGen = map[string]int{}

func declare(g string, ft *ast.FuncType, fixed bool, what string) {
	if fixed {
		groupFixed[g] = append(groupFixed[g], rw.Fset.Position(ft.Pos()).String())
	} else {
		groupGen[g]++
	}
	walk := func(fl *ast.FieldList, mk func(string, int) int) {
		if fl == nil {
			return
		}
		i := 0
		for _, f := range fl.List {
			n := len(f.Names)
			if n == 0 {
				n = 1
			}
			for j := 0; j < n; j++ {
				if isList(typeOf(f.Type)) {
					k := mk(g, i)
					if fixed {
						poison(k, what)
					} else {
						declareType(k, f.Type)
					}
					if j < len(f.Names) {
						if v, ok := rw.Info.Defs[f.Names[j]].(*types.Var); ok {
							union(k, varKey(v))
						}
					}
				}
				i++
			}
		}
	}
	walk(ft.Params, paramKey)
	walk(ft.Results, resultKey)
}

var declared = map[ast.Expr]bool{}

func declareType(k int, te ast.Expr) {
	if declared[te] {
		return
	}
	declared[te] = true
	c := cls(k)
	c.typeExpr = append(c.typeExpr, te)
	if a := rw.AliasArgs(te, K.aliases...); len(a) == K.nargs {
		for i := range a {
			c.elems[i] = append(c.elems[i], a[i])
		}
	}
}

// ---- values

const (
	vSlot = iota
	vNew
	vNil
	vForeign
	vImm  // an immutable list from erased code: copied into an immutable TList
	vCopy // new ArrayList(collection)
)

type value struct {
	kind int
	k    int
	why  string
	cap  ast.Expr // capacity of a new list
	from ast.Expr // the erased collection of vImm and vCopy
}

func isImmutable(t types.Type) bool {
	if t == nil {
		return false
	}
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	return ok && K.immutable[n.Obj().Name()]
}

// freshImmutable are runtime functions that return a new unmodifiable list.
var freshImmutable = map[string]bool{"splitToList": true}

var transparent map[string]bool

// unwrap strips the conversions that keep the same list object: it returns the innermost
// list-valued expression of a chain of such conversions.
func unwrap(e ast.Expr) ast.Expr {
	best := e
	for {
		e = rw.Unparen(e)
		if isList(typeOf(e)) {
			best = e
		}
		if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 {
			if n, _ := rw.CallName(c); transparent[n] {
				e = c.Args[0]
				continue
			}
		}
		if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 0 {
			if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "self" {
				e = s.X
				continue
			}
		}
		return best
	}
}

func calleeFunc(c *ast.CallExpr) *types.Func {
	switch f := rw.Unparen(c.Fun).(type) {
	case *ast.Ident:
		o, _ := rw.Info.Uses[f].(*types.Func)
		return o
	case *ast.SelectorExpr:
		if s, ok := rw.Info.Selections[f]; ok {
			o, _ := s.Obj().(*types.Func)
			return o
		}
		o, _ := rw.Info.Uses[f.Sel].(*types.Func)
		return o
	}
	return nil
}

func varOf(e ast.Expr) *types.Var {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		if v, ok := rw.Info.Uses[x].(*types.Var); ok {
			return v
		}
		v, _ := rw.Info.Defs[x].(*types.Var)
		return v
	case *ast.SelectorExpr:
		if s, ok := rw.Info.Selections[x]; ok && s.Kind() == types.FieldVal {
			v, _ := s.Obj().(*types.Var)
			return v
		}
		v, _ := rw.Info.Uses[x.Sel].(*types.Var)
		return v
	}
	return nil
}

// slotOfVar is the slot of a variable, or 0 if the variable cannot change (runtime code).
func slotOfVar(v *types.Var) int {
	k := varKey(v)
	if v.Origin() != v || rw.ReflipGeneric_(v.Type(), false) { // reflip: generic instance
		poison(k, "variable of a type parameter's type")
	}
	if !generated(v.Pos()) {
		poison(k, "variable of the runtime")
	}
	return k
}

func valueOf(e ast.Expr) value {
	e = unwrap(e)
	switch x := e.(type) {
	case *ast.Ident:
		if x.Name == "nil" {
			return value{kind: vNil}
		}
	case *ast.CallExpr:
		n, c := rw.CallName(x)
		if transparent[n] {
			// The innermost list-valued conversion of something that is not a list.
			in := x.Args[0]
			for {
				if c2, ok := rw.Unparen(in).(*ast.CallExpr); ok && len(c2.Args) == 1 {
					if n2, _ := rw.CallName(c2); n2 == "obj_" || n2 == "any" {
						in = c2.Args[0]
						continue
					}
				}
				break
			}
			if isImmutable(typeOf(in)) {
				return value{kind: vImm, from: in}
			}
			return value{kind: vForeign, why: "converted from " + rw.Text(x.Args[0])[:min(40, len(rw.Text(x.Args[0])))]}
		}
		if a, ok := K.newFns[n]; ok && len(c.Args) == a {
			if a == 1 {
				return value{kind: vNew, cap: c.Args[0]}
			}
			return value{kind: vNew}
		}
		if K.copyFns[n] && len(c.Args) == 1 {
			return value{kind: vCopy, from: c.Args[0]}
		}
		if fl, ok := rw.Unparen(x.Fun).(*ast.FuncLit); ok {
			return value{kind: vSlot, k: resultKey(litGroup(fl), 0)}
		}
		if f := calleeFunc(x); f != nil && isList(f.Type().(*types.Signature).Results().At(0).Type()) {
			if !generated(f.Pos()) {
				if freshImmutable[f.Name()] {
					return value{kind: vImm, from: x}
				}
				return value{kind: vForeign, why: "result of " + f.Name()}
			}
			if f.Origin() != f { // reflip: generic instance
				poison(resultKey(groupOf(f.Origin()), 0), "result of a method of a generic type")
				return value{kind: vForeign, why: "result of a method of a generic type"}
			}
			return value{kind: vSlot, k: resultKey(groupOf(f), 0)}
		}
		if n == "" {
			n = "a call"
		}
		return value{kind: vForeign, why: "result of " + n}
	}
	if v := varOf(e); v != nil {
		return value{kind: vSlot, k: slotOfVar(v)}
	}
	return value{kind: vForeign, why: fmt.Sprintf("%T", e)}
}

func litGroup(fl *ast.FuncLit) string { return fmt.Sprintf("lit|%d", fl.Pos()) }

// ---- contexts

// climb goes up from a list-valued expression through the conversions that keep the object.
func climb(e ast.Expr) ast.Expr {
	for {
		p := rw.Parent[e]
		switch x := p.(type) {
		case *ast.ParenExpr:
			e = x
			continue
		case *ast.CallExpr:
			if len(x.Args) == 1 && x.Args[0] == e {
				if n, _ := rw.CallName(x); transparent[n] {
					e = x
					continue
				}
			}
		case *ast.SelectorExpr:
			if x.Sel.Name == "self" && x.X == e {
				if c, ok := rw.Parent[x].(*ast.CallExpr); ok && c.Fun == x && len(c.Args) == 0 {
					e = c
					continue
				}
			}
		}
		return e
	}
}

// flow records that the value of expression src (whose outermost conversion is top) goes to slot dest.
func flow(dest int, src ast.Expr, top ast.Expr) {
	v := valueOf(src)
	if v.kind == vNew || v.kind == vCopy {
		ctorDest[unwrap(src)] = dest
	}
	switch v.kind {
	case vNil:
	case vForeign:
		poison(dest, v.why)
	case vNew:
		capExpr := v.cap
		edit(dest, func(c *class) rw.Edit {
			return rw.Edit{Node: top, Gen: func() string { return K.newCode(c, capExpr) }}
		})
	case vImm, vCopy:
		from, imm := v.from, v.kind == vImm
		edit(dest, func(c *class) rw.Edit {
			return rw.Edit{Node: top, Gen: func() string {
				r := rw.Render
				if from == top {
					r = rw.RenderChildren
				}
				return K.fromCode(c, r(from), imm)
			}}
		})
	case vSlot:
		union(dest, v.k)
		inner := unwrap(src)
		if top != inner {
			edit(dest, func(c *class) rw.Edit { return rw.Edit{Node: top, Gen: func() string { return rw.Render(inner) }} })
		}
	}
}

func enclosingFunc(n ast.Node) string {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch f := p.(type) {
		case *ast.FuncLit:
			return litGroup(f)
		case *ast.FuncDecl:
			if o, ok := rw.Info.Defs[f.Name].(*types.Func); ok {
				return groupOf(o)
			}
			return ""
		}
	}
	return ""
}

// paramIndexKey is the slot of argument i of call c, or 0 if the callee is not generated code.
func paramIndexKey(c *ast.CallExpr, i int) (int, bool) {
	if fl, ok := rw.Unparen(c.Fun).(*ast.FuncLit); ok {
		return paramKey(litGroup(fl), i), true
	}
	f := calleeFunc(c)
	if f == nil || !generated(f.Pos()) {
		return 0, false
	}
	sig := f.Type().(*types.Signature)
	if sig.Variadic() && i >= sig.Params().Len()-1 {
		return 0, false
	}
	if i >= sig.Params().Len() || !isList(sig.Params().At(i).Type()) {
		return 0, false
	}
	if f.Origin() != f { // reflip: generic instance
		poison(paramKey(groupOf(f), i), "argument of a method of a generic type")
		poison(paramKey(groupOf(f.Origin()), i), "argument of a method of a generic type")
	}
	return paramKey(groupOf(f), i), true
}

// handle classifies the occurrence of list-valued expression e (not an assignment target).
func handle(e ast.Expr) {
	v := valueOf(e)
	top := climb(e)
	p := rw.Parent[top]
	if v.kind == vNew || v.kind == vCopy {
		ctorWhy[e] = fmt.Sprintf("used directly in a %T", p)
		if c, ok := p.(*ast.CallExpr); ok {
			n, _ := rw.CallName(c)
			if n == "" {
				n = rw.Text(c.Fun)
			}
			ctorWhy[e] = "passed directly to " + n
		}
	}
	src := v.k
	if v.kind != vSlot {
		src = 0
	}
	// boundary: the value leaves for code that expects an erased List or an Object.
	boundary := func() {
		if src == 0 {
			return
		}
		if K.view == "" {
			if os.Getenv("COLLS_ANY") != "" {
				pp := rw.Fset.Position(top.Pos())
				fmt.Fprintf(os.Stderr, "ERASED %s:%d %T | %s\n", filepath.Base(pp.Filename), pp.Line, rw.Parent[top], rw.Text(rw.Parent[top].(ast.Node))[:min(120, len(rw.Text(rw.Parent[top].(ast.Node))))])
			}
			poison(src, "seen as an erased "+K.name+" (its order)")
			return
		}
		inner := unwrap(e)
		if top != inner {
			if _, ok := top.(*ast.CallExpr); ok {
				if types.Identical(types.Unalias(typeOf(top)), K.held[0]) {
					edit(src, func(c *class) rw.Edit {
						return rw.Edit{Node: top, Gen: func() string { return rw.Render(inner) + "." + K.view + "()" }}
					})
					return
				}
				if isList(typeOf(top)) {
					poison(src, "converted to a concrete runtime collection for erased code")
					return
				}
			}
			if c := cls(src); c.anyAt == "" && !rw.ReadOnlySink(top, true) {
				p := rw.Fset.Position(top.Pos())
				c.anyAt = fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
			}
			if os.Getenv("COLLS_ANY") != "" {
				pp := rw.Fset.Position(top.Pos())
				ctx := fmt.Sprintf("%T", rw.Parent[top])
				if c, ok := rw.Parent[top].(*ast.CallExpr); ok {
					ctx = "call " + rw.Text(c.Fun)
				}
				fmt.Fprintf(os.Stderr, "ANY %s:%d %s | %s\n", filepath.Base(pp.Filename), pp.Line, ctx, rw.Text(top))
			}
			edit(src, func(c *class) rw.Edit {
				return rw.Edit{Node: top, Gen: func() string { return "any(" + rw.Render(inner) + "." + K.view + "())" }}
			})
			return
		}
		edit(src, func(c *class) rw.Edit {
			return rw.Edit{Node: inner, Gen: func() string { return rw.RenderChildren(inner) + "." + K.view + "()" }}
		})
	}
	switch x := p.(type) {
	case *ast.AssignStmt:
		for i, r := range x.Rhs {
			if r != top {
				continue
			}
			if len(x.Lhs) != len(x.Rhs) {
				if src != 0 {
					poison(src, "multi-value assignment")
				}
				return
			}
			if id, ok := x.Lhs[i].(*ast.Ident); ok && id.Name == "_" {
				return
			}
			if !isList(typeOf(x.Lhs[i])) {
				boundary()
				return
			}
			dv := varOf(x.Lhs[i])
			if dv == nil {
				if src != 0 {
					poison(src, "stored into an element of a map, slice or array")
				}
				return
			}
			flow(slotOfVar(dv), e, top)
			return
		}
		return // e is an assignment target
	case *ast.ValueSpec:
		for i, r := range x.Values {
			if r == top {
				if x.Names[i].Name == "_" {
					return
				}
				dv := rw.Info.Defs[x.Names[i]].(*types.Var)
				if !isList(dv.Type()) {
					boundary()
					return
				}
				flow(slotOfVar(dv), e, top)
			}
		}
		return
	case *ast.ReturnStmt:
		g := enclosingFunc(x)
		for i, r := range x.Results {
			if r == top {
				if !isList(typeOf(top)) {
					boundary() // returned as an Object
					return
				}
				flow(resultKey(g, i), e, top)
			}
		}
		return
	case *ast.CallExpr:
		// SequencedCollection_addFirst(SequencedCollection(d), v) and the like on a deque.
		if n, _ := rw.CallName(x); K.name == "deque" && src != 0 && (n == "SequencedCollection" || n == "icast_SequencedCollection") && len(x.Args) == 1 {
			if c2, ok := rw.Up(x).(*ast.CallExpr); ok && len(c2.Args) >= 1 && rw.Unparen(c2.Args[0]) == x {
				n2, _ := rw.CallName(c2)
				if m, ok := strings.CutPrefix(n2, "SequencedCollection_"); ok {
					want := 1
					if strings.HasPrefix(m, "add") {
						want = 2
					}
					if len(c2.Args) == want {
						seqUse(src, unwrap(e), m, c2)
						return
					}
				}
			}
		}
		// Preconditions.checkNotNull(d) as a statement: only whether the deque is null.
		if n, _ := rw.CallName(x); K.name == "deque" && src != 0 && strings.HasPrefix(n, "Preconditions_checkNotNull") && len(x.Args) >= 1 && x.Args[0] == top {
			if _, stmt := rw.Up(x).(*ast.ExprStmt); stmt {
				inner := unwrap(e)
				edit(src, func(c *class) rw.Edit {
					return rw.Edit{Node: top, Gen: func() string { return "obj_(" + rw.Render(inner) + ")" }}
				})
				return
			}
		}
		if n, _ := rw.CallName(x); src != 0 && len(x.Args) >= 2 && x.Args[0] == top && top == unwrap(e) {
			if m, ok := K.helpers[n]; ok {
				cls(src).strKey = true
				helper(src, top, m, x)
				return
			}
		}
		for i, a := range x.Args {
			if a != top {
				continue
			}
			// it := foreach_(any(l))
			if n, _ := rw.CallName(x); n == "foreach_" && src != 0 && K.name != "map" {
				if K.name == "deque" {
					cls(src).dqIter = true
				}
				iterate(src, e, x, "v", 0)
				return
			}
			if k, ok := paramIndexKey(x, i); ok && isList(typeOf(top)) {
				flow(k, e, top)
				return
			}
			boundary()
			return
		}
		if src != 0 {
			poison(src, "called as a function")
		}
		return
	case *ast.SelectorExpr:
		if x.X == top && top == unwrap(e) {
			if c, ok := rw.Parent[x].(*ast.CallExpr); ok && c.Fun == x {
				if src != 0 {
					use(src, top, x.Sel.Name, c)
				}
				return
			}
		}
		if src != 0 {
			poison(src, "method value or field of the list taken")
		}
		return
	case *ast.BinaryExpr:
		if x.Op == token.EQL || x.Op == token.NEQ {
			other := x.X
			if other == top {
				other = x.Y
			}
			if id, ok := rw.Unparen(other).(*ast.Ident); ok && id.Name == "nil" {
				return
			}
			if isList(typeOf(other)) && top == unwrap(e) {
				if ov := valueOf(other); ov.kind == vSlot && src != 0 {
					union(src, ov.k)
					return
				}
			}
		}
		if src != 0 {
			poison(src, "compared with something that is not a list of the class")
		}
		return
	case *ast.KeyValueExpr:
		if x.Value == top {
			if id, ok := x.Key.(*ast.Ident); ok {
				if fv, ok := rw.Info.Uses[id].(*types.Var); ok && isList(fv.Type()) {
					flow(slotOfVar(fv), e, top)
					return
				}
			}
		}
		boundary()
		return
	case *ast.ExprStmt:
		return
	}
	if src != 0 {
		poison(src, fmt.Sprintf("used in a %T", p))
	}
}

// iterate handles it := foreach_(any(l)) over a collection of class k: the loop walks the
// typed collection; field is the cursor field holding the element and ci its conversion.
func iterate(k int, e ast.Expr, fe *ast.CallExpr, field string, ci int) {
	as, ok := rw.Parent[fe].(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 {
		poison(k, "for-each cursor not declared with :=")
		return
	}
	it := rw.Info.Defs[as.Lhs[0].(*ast.Ident)]
	var reads []*ast.SelectorExpr
	for _, u := range rw.UsesOf[it] {
		s, ok := rw.Parent[u].(*ast.SelectorExpr)
		if !ok || (s.Sel.Name != "v" && s.Sel.Name != "next") {
			poison(k, "for-each cursor used for more than next and v")
			return
		}
		if s.Sel.Name == "v" {
			reads = append(reads, s)
		}
	}
	inner := unwrap(e)
	edit(k, func(c *class) rw.Edit {
		if c.dqHead != "" && c.dqTail == "" {
			// A deque used only at its head: from the end of the typed list.
			return rw.Edit{Node: fe, Gen: func() string { return rw.Render(inner) + ".eachRev()" }}
		}
		return rw.Edit{Node: fe, Gen: func() string { return rw.Render(inner) + ".each()" }}
	})
	for _, s := range reads {
		s := s
		edit(k, func(c *class) rw.Edit {
			return rw.Read(c.cv[ci], s, func() string { return rw.Text(s.X) + "." + field })
		})
	}
}

// use handles the call of method m on a collection of class k (recv is the receiver expression).
func use(k int, recv ast.Expr, m string, call *ast.CallExpr) {
	_, isStmt := rw.Up(call).(*ast.ExprStmt)
	R := func() string { return rw.Render(recv) }
	args := call.Args
	keyArg := func(c *class, i int) string { return rw.Unboxed(c.cv[0], c.elem[0], args[i]) }
	valArg := func(c *class, i int) string { return rw.Unboxed(c.cv[1], c.elem[1], args[i]) }
	plain := func(name string, a ...func(c *class) string) {
		edit(k, func(c *class) rw.Edit {
			return rw.Edit{Node: call, Gen: func() string {
				var ps []string
				for _, f := range a {
					ps = append(ps, f(c))
				}
				return R() + "." + name + "(" + strings.Join(ps, ", ") + ")"
			}}
		})
	}
	// read: a call whose result is an element (conversion ci), unboxed where the code unboxes it.
	read := func(ci int, name string, a ...func(c *class) string) {
		edit(k, func(c *class) rw.Edit {
			g := func() string {
				var ps []string
				for _, f := range a {
					ps = append(ps, f(c))
				}
				return R() + "." + name + "(" + strings.Join(ps, ", ") + ")"
			}
			if isStmt {
				return rw.Edit{Node: call, Gen: g}
			}
			return rw.Read(c.cv[ci], call, g)
		})
	}
	raw := func(i int) func(c *class) string { return func(c *class) string { return rw.Render(args[i]) } }
	key := func(i int) func(c *class) string { return func(c *class) string { return keyArg(c, i) } }
	val := func(i int) func(c *class) string { return func(c *class) string { return valArg(c, i) } }
	n := len(args)
	switch K.name {
	case "list":
		switch {
		case m == "add" && n == 1:
			plain("add", key(0))
			return
		case m == "get" && n == 1:
			read(0, "get", raw(0))
			return
		case m == "set" && n == 2:
			read(0, "set", raw(0), key(1))
			return
		case m == "remove" && n == 1 && isInt(args[0]):
			read(0, "removeAt", raw(0))
			return
		case m == "add_int_Object" && n == 2:
			plain("insert", raw(0), key(1))
			return
		case (m == "size" || m == "isEmpty" || m == "clear" || m == "trimToSize") && n == 0, m == "ensureCapacity" && n == 1:
			return
		}
	case "deque":
		// The deque operations by end and kind (push, pop, peek, poll, get).
		head := map[string]string{"push": "push", "addFirst": "push", "offerFirst": "push", "pop": "pop", "removeFirst": "pop",
			"remove_0": "pop", "peek": "peek", "peekFirst": "peek", "poll": "poll", "pollFirst": "poll",
			"getFirst": "get", "element": "get"}
		tail := map[string]string{"add": "push", "addLast": "push", "offer": "push", "offerLast": "push", "removeLast": "pop",
			"peekLast": "peek", "pollLast": "poll", "getLast": "get"}
		switch {
		case (m == "size" || m == "isEmpty" || m == "clear") && n == 0:
			return
		case head[m] != "" && n == map[bool]int{true: 1}[head[m] == "push"]:
			dequeOp(k, call.Fun.(*ast.SelectorExpr).X, m, true, head[m], call, args)
			return
		case tail[m] != "" && n == map[bool]int{true: 1}[tail[m] == "push"]:
			dequeOp(k, call.Fun.(*ast.SelectorExpr).X, m, false, tail[m], call, args)
			return
		}
		poison(k, "deque method "+m)
		return
	case "set":
		switch {
		case m == "add" && n == 1:
			plain("add", key(0))
			return
		case m == "contains" && n == 1:
			plain("contains", key(0))
			return
		case m == "remove_Object" && n == 1:
			plain("remove", key(0))
			return
		case (m == "size" || m == "isEmpty" || m == "clear") && n == 0:
			return
		}
	case "map":
		switch {
		case m == "get" && n == 1:
			read(1, "get", key(0))
			return
		case m == "put" && n == 2:
			read(1, "put", key(0), val(1))
			return
		case m == "containsKey" && n == 1:
			plain("containsKey", key(0))
			return
		case m == "remove" && n == 1:
			read(1, "remove", key(0))
			return
		case m == "getOrDefault" && n == 2:
			read(1, "getOrDefault", key(0), val(1))
			return
		case m == "putIfAbsent" && n == 2:
			read(1, "putIfAbsent", key(0), val(1))
			return
		case m == "computeIfAbsent" && n == 2:
			read(1, "computeIfAbsent", key(0), raw(1))
			return
		case (m == "size" || m == "isEmpty" || m == "clear") && n == 0:
			return
		case (m == "keySet" || m == "values") && n == 0:
			// for k := range m.keySet() / m.values(): it := foreach_(any(m.keySet()))
			top := climb(call)
			if fe, ok := rw.Parent[top].(*ast.CallExpr); ok {
				if nm, _ := rw.CallName(fe); nm == "foreach_" && len(fe.Args) == 1 && fe.Args[0] == top {
					if m == "keySet" {
						iterate(k, recv, fe, "k", 0)
					} else {
						iterate(k, recv, fe, "v", 1)
					}
					return
				}
			}
		}
	}
	if m == "addAll" && n == 1 && K.name != "map" {
		// addAll(other) where other is a typed collection of the same element type: typed.
		arg := args[0]
		for {
			c2, ok := rw.Unparen(arg).(*ast.CallExpr)
			if !ok || len(c2.Args) != 1 {
				break
			}
			if nm, _ := rw.CallName(c2); nm == "Collection" || nm == "Iterable" || transparent[nm] {
				arg = c2.Args[0]
				continue
			}
			break
		}
		if av := valueOf(arg); av.kind == vSlot {
			inner := unwrap(arg)
			sel := call.Fun.(*ast.SelectorExpr)
			edit(k, func(c *class) rw.Edit {
				return rw.Edit{Node: call, Gen: func() string {
					if o, ok := converted[find(av.k)]; ok && o.elemText[0] == c.elemText[0] {
						return R() + ".addAllT(" + rw.RenderChildren(inner) + ")"
					}
					return R() + "." + K.view + "()." + sel.Sel.Name + "(" + rw.Render(args[0]) + ")"
				}}
			})
			return
		}
	}
	if m == "self" && n == 0 {
		edit(k, func(c *class) rw.Edit {
			return rw.Edit{Node: call, Gen: func() string { return "any(" + R() + "." + K.view + "())" }}
		})
		return
	}
	sel := call.Fun.(*ast.SelectorExpr)
	edit(k, func(c *class) rw.Edit {
		return rw.Edit{Node: sel, Gen: func() string { return R() + "." + K.view + "()." + sel.Sel.Name }}
	})
}

// seqUse handles SequencedCollection_m(SequencedCollection(d), args...) on deque d (recv) as
// the deque method m with the remaining arguments.
func seqUse(k int, recv ast.Expr, m string, call *ast.CallExpr) {
	op := map[string]string{"addFirst": "push", "removeFirst": "pop", "getFirst": "get", "addLast": "push", "removeLast": "pop", "getLast": "get"}[m]
	if op == "" {
		poison(k, "deque helper SequencedCollection_"+m)
		return
	}
	dequeOp(k, recv, m, strings.HasSuffix(m, "First"), op, call, call.Args[1:])
}

// dequeOp rewrites call, the deque method m (operation op at the head or the tail) of class
// k on recv, into a typed list method. Which one is decided by the class: a deque used only at
// its head keeps its head at the end of the list (pushes are appends); any other keeps the
// deque's order, its tail at the end (operations at the head work at index 0, and a class
// that pushes at its head as well stays erased).
func dequeOp(k int, recv ast.Expr, m string, atHead bool, op string, call *ast.CallExpr, args []ast.Expr) {
	_, isStmt := rw.Up(call).(*ast.ExprStmt)
	c := cls(k)
	if atHead && c.dqHead == "" {
		c.dqHead = m
	}
	if !atHead && c.dqTail == "" {
		c.dqTail = m
	}
	if atHead && op == "push" && c.dqHeadPush == "" {
		c.dqHeadPush = m
	}
	edit(k, func(c *class) rw.Edit {
		g := func() string {
			var ps []string
			for _, a := range args {
				ps = append(ps, rw.Unboxed(c.cv[0], c.elem[0], a))
			}
			name := map[string]string{"push": "push_", "pop": "pop_", "peek": "peekTop_", "poll": "pollTop_", "get": "getTop_"}[op]
			if atHead && c.dqTail != "" {
				name = map[string]string{"pop": "popFirst_", "peek": "peekFirst_", "poll": "pollFirst_", "get": "getFirst_"}[op]
			}
			r := rw.Render(recv) + "." + name + "(" + strings.Join(ps, ", ") + ")"
			if op == "push" && !isStmt {
				// add, offer, offerFirst, offerLast return true.
				return "func() bool { " + r + "; return true }()"
			}
			return r
		}
		if isStmt || op == "push" {
			return rw.Edit{Node: call, Gen: g}
		}
		return rw.Read(c.cv[0], call, g)
	})
}

// helper handles a runtime helper that takes the collection first and an unboxed string key
// (Map_get_s(m, k), Collection_contains_s(s, k), ...).
func helper(k int, recv ast.Expr, m string, call *ast.CallExpr) {
	_, isStmt := rw.Up(call).(*ast.ExprStmt)
	R := func() string { return rw.Render(recv) }
	args := call.Args[1:]
	g := func(c *class, name string, n int, valAt int) func() string {
		return func() string {
			ps := []string{rw.Render(args[0])}
			for i := 1; i < n; i++ {
				if i == valAt {
					ps = append(ps, rw.Unboxed(c.cv[1], c.elem[1], args[i]))
				} else {
					ps = append(ps, rw.Render(args[i]))
				}
			}
			return R() + "." + name + "(" + strings.Join(ps, ", ") + ")"
		}
	}
	edit(k, func(c *class) rw.Edit {
		switch m {
		case "get", "remove", "put", "getOrDefault", "putIfAbsent", "computeIfAbsent":
			valAt := -1
			if m == "put" || m == "getOrDefault" || m == "putIfAbsent" {
				valAt = 1
			}
			gen := g(c, m, len(args), valAt)
			if isStmt {
				return rw.Edit{Node: call, Gen: gen}
			}
			return rw.Read(c.cv[1], call, gen)
		}
		name := m
		if m == "remove" {
			name = "remove"
		}
		return rw.Edit{Node: call, Gen: g(c, name, len(args), -1)}
	})
}

func isInt(e ast.Expr) bool {
	b, ok := types.Unalias(typeOf(e)).(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0
}

// ---- element types and conversions

// element returns the element type of type argument i, or why there is none. Keys (i == 0 of a
// map or set) must compare in Go as in Java.
func (c *class) element(i int) (ast.Expr, types.Type, string) {
	var el ast.Expr
	var et types.Type
	var anySlot ast.Expr
	anyOK := K.name == "map" && i == 1
	for _, e := range c.elems[i] {
		if a := rw.AliasArgs(e, "BoxedOf"); a != nil {
			if anyOK {
				continue
			}
			return nil, nil, "element is a boxed primitive"
		}
		t := typeOf(e)
		if t == nil {
			continue
		}
		if it, ok := types.Unalias(t).(*types.Interface); ok && it.Empty() {
			anySlot = e
			continue
		}
		if et == nil {
			el, et = e, t
		} else if !types.Identical(t, et) {
			// Covariant overrides (List<LinkedDiGraphEdge> for List<? extends DiGraphEdge>): the
			// element type is the one every stated type is assignable to.
			switch {
			case types.AssignableTo(et, t):
				el, et = e, t
			case types.AssignableTo(t, et):
			default:
				return nil, nil, "slots disagree on the element type (" + rw.Text(el) + ", " + rw.Text(e) + ")"
			}
		}
	}
	if et == nil {
		if anyOK {
			return nil, types.Universe.Lookup("any").Type(), ""
		}
		return nil, nil, "no slot states an element type"
	}
	// A slot stating Object (any) may hold values of any class: typed as T, a value of another
	// class would fail its conversion where Java stored it. (Steps 3-5 allowed this; the
	// classes they typed are not seen here again.)
	if anySlot != nil {
		return nil, nil, "a slot states Object (" + rw.Text(anySlot) + ") and another " + rw.Text(el)
	}
	for _, e := range c.elems[i] {
		if t := typeOf(e); t != nil && !types.AssignableTo(t, et) {
			if it, ok := types.Unalias(t).(*types.Interface); !ok || !it.Empty() {
				return nil, nil, "slots disagree on the element type (" + rw.Text(el) + ", " + rw.Text(e) + ")"
			}
		}
	}
	if i == 0 && (K.name == "map" || K.name == "set") {
		switch rw.KeyMode(et) {
		case rw.KeyNone:
			return nil, nil, "key type " + rw.Text(el) + " has no typed comparison"
		case rw.KeyHash:
			c.hashKey = true
		}
	}
	return el, et, ""
}

func isString(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Kind() == types.String
}

var convs = map[string]string{} // conversion variable -> declaration

var nonIdent = regexp.MustCompile(`[^A-Za-z0-9_]`)

// convName is the conversion variable of type argument i of the class.
func (c *class) convName(i int) string {
	text, t := c.elemText[i], c.elem[i]
	hash := i == 0 && c.hashKey
	prefix := "tc_"
	if hash {
		prefix = "tch_" // a key conversion with Java's hashCode and equals
	}
	n := prefix + nonIdent.ReplaceAllString(strings.ReplaceAll(text, "*", "p_"), "_")
	if rw.Pkg.Scope().Lookup(n) != nil {
		return n // declared by an earlier step
	}
	if _, ok := convs[n]; !ok {
		cv := rw.ConvFor(t)
		extra := ""
		if isString(t) {
			extra = ", null: nullstr_"
		}
		if hash {
			extra = fmt.Sprintf(",\n\thash: func(v %s) int32 { return objhash_(%s) }, eq: func(a, b %s) bool { return objequals_(%s, %s) }",
				text, cv.Box("v"), text, cv.Box("a"), cv.Box("b"))
		}
		convs[n] = fmt.Sprintf("var %s = &tconv_[%s]{box: func(v %s) any { return %s }, unbox: func(v any) %s { return %s }%s}",
			n, text, text, cv.Box("v"), text, cv.Unbox("v"), extra)
	}
	return n
}

// ---- main

func main() {
	report, verbose, kind := false, false, ""
	for i := 1; i < len(os.Args)-1; i++ {
		switch os.Args[i] {
		case "-n":
			report = true
		case "-v":
			verbose = true
		case "-kind":
			i++
			kind = os.Args[i]
		}
	}
	rw.Load(os.Args[len(os.Args)-1])
	K = kinds()[kind]
	if K == nil {
		fmt.Fprintln(os.Stderr, "usage: colls -kind list|map|set [-n] [-v] <dir>")
		os.Exit(2)
	}
	transparent = map[string]bool{"obj_": true, "any": true}
	for _, c := range K.conv {
		transparent[c] = true
	}

	// Slots: function signatures, interface methods, fields and variables.
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				curPos = x.Pos()
				if o, ok := rw.Info.Defs[x.Name].(*types.Func); ok {
					declare(groupOf(o), x.Type, !generated(x.Pos()), "signature of the runtime")
				}
			case *ast.InterfaceType:
				for _, m := range x.Methods.List {
					ft, ok := m.Type.(*ast.FuncType)
					if !ok || len(m.Names) != 1 {
						continue
					}
					curPos = m.Pos()
					if o, ok := rw.Info.Defs[m.Names[0]].(*types.Func); ok {
						declare(groupOf(o), ft, !generated(x.Pos()), "method of a library interface")
					}
				}
			case *ast.FuncLit:
				declare(litGroup(x), x.Type, !generated(x.Pos()), "function literal of the runtime")
			case *ast.Field:
				if _, inStruct := rw.Parent[rw.Parent[x]].(*ast.StructType); inStruct && isList(typeOf(x.Type)) {
					for _, id := range x.Names {
						if v, ok := rw.Info.Defs[id].(*types.Var); ok {
							k := slotOfVar(v)
							if generated(x.Pos()) {
								declareType(k, x.Type)
							}
						}
					}
				}
			case *ast.RangeStmt:
				// The keys and elements of a Go slice or map (left by an earlier step) are not slots.
				curPos = x.Pos()
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if e == nil {
						continue
					}
					if v := varOf(e); v != nil && isList(v.Type()) {
						poison(slotOfVar(v), "element of a Go slice or map")
					}
				}
			case *ast.AssignStmt:
				// v, ok := m[k], a, b := f(): the values do not come from slots of the class.
				if len(x.Lhs) != len(x.Rhs) {
					curPos = x.Pos()
					for _, e := range x.Lhs {
						if v := varOf(e); v != nil && isList(v.Type()) {
							poison(slotOfVar(v), "assigned from a multi-value expression")
						}
					}
				}
			case *ast.ValueSpec:
				if len(x.Values) > 0 && len(x.Values) != len(x.Names) {
					curPos = x.Pos()
					for _, id := range x.Names {
						if v, ok := rw.Info.Defs[id].(*types.Var); ok && isList(v.Type()) {
							poison(slotOfVar(v), "assigned from a multi-value expression")
						}
					}
				}
				if x.Type != nil && isList(typeOf(x.Type)) {
					for _, id := range x.Names {
						if v, ok := rw.Info.Defs[id].(*types.Var); ok {
							k := slotOfVar(v)
							if generated(x.Pos()) {
								declareType(k, x.Type)
							}
						}
					}
				}
			}
			return true
		})
	}
	// Function literals that are not called on the spot may be called by anything.
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if fl, ok := n.(*ast.FuncLit); ok {
				if c, ok := rw.Parent[fl].(*ast.CallExpr); !ok || c.Fun != fl {
					for _, k := range []string{":p", ":r"} {
						for i := 0; i < 8; i++ {
							if kk, ok := keys[fmt.Sprintf("%s%s%d", litGroup(fl), k, i)]; ok {
								poison(kk, "function literal used as a value")
							}
						}
					}
				}
			}
			// A method of a generated class used as a function value.
			if s, ok := n.(*ast.SelectorExpr); ok {
				if o, ok := rw.Info.Uses[s.Sel].(*types.Func); ok {
					if c, ok := rw.Parent[s].(*ast.CallExpr); !ok || c.Fun != s {
						g := groupOf(o)
						for _, k := range []string{":p", ":r"} {
							for i := 0; i < 8; i++ {
								if kk, ok := keys[fmt.Sprintf("%s%s%d", g, k, i)]; ok {
									poison(kk, "method used as a function value")
								}
							}
						}
					}
				}
			}
			return true
		})
	}
	// The runtime cannot change: what it touches stays erased.
	for _, f := range rw.Files {
		if generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			fn := calleeFunc(c)
			if fn == nil || !generated(fn.Pos()) {
				return true
			}
			sig := fn.Type().(*types.Signature)
			for i := 0; i < sig.Params().Len(); i++ {
				if isList(sig.Params().At(i).Type()) {
					poison(paramKey(groupOf(fn), i), "parameter used by the runtime")
				}
			}
			for i := 0; i < sig.Results().Len(); i++ {
				if isList(sig.Results().At(i).Type()) {
					poison(resultKey(groupOf(fn), i), "result used by the runtime")
				}
			}
			return true
		})
	}
	// Values: every list-valued expression in generated code.
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			switch e.(type) {
			case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr, *ast.TypeAssertExpr, *ast.StarExpr:
			default:
				return true
			}
			if !isList(typeOf(e)) {
				return true
			}
			if tv := rw.Info.Types[e]; tv.IsType() {
				return true
			}
			// Only the innermost expression of a chain of conversions.
			if unwrap(e) != e {
				return true
			}
			if s, ok := rw.Parent[e].(*ast.SelectorExpr); ok && s.Sel == e {
				return true
			}
			curPos = e.Pos()
			handle(e)
			return true
		})
	}
	if os.Getenv("LISTS_GROUPS") != "" {
		for g, fx := range groupFixed {
			if groupGen[g] > 0 {
				for _, k := range []string{":p0", ":p1", ":p2", ":r0"} {
					if kk, ok := keys[g+k]; ok && len(cls(kk).typeExpr) > 0 {
						fmt.Fprintf(os.Stderr, "group %s%s: %d generated, fixed at %s; class decls %d\n", g, k, groupGen[g], filepath.Base(fx[0]), len(cls(kk).typeExpr))
					}
				}
			}
		}
	}
	// Casts from Object to the concrete runtime type (cast_[*ArrayList](o), to_ArrayList(o)) cannot
	// take a view: then no typed collection of the kind may be boxed into an Object.
	concreteCasts := 0
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok && len(c.Args) == 1 && isList(typeOf(c)) && !isList(typeOf(c.Args[0])) {
				if t := types.Unalias(typeOf(c)); t != K.held[0] && !types.Identical(t, K.held[0]) {
					concreteCasts++
				}
			}
			return true
		})
	}
	fmt.Fprintf(os.Stderr, "casts from Object to the concrete runtime %s type: %d\n", K.name, concreteCasts)
	// Decide.
	reasons := map[string]int{}
	classWhy := map[int]string{}
	var good []*class
	nSlots, nGood := 0, 0
	for r, c := range data {
		if find(r) != r {
			continue
		}
		nSlots += c.slots
		why := ""
		if len(c.typeExpr) == 0 && len(c.edits) == 0 {
			continue
		}
		if len(c.poison) > 0 {
			why = c.poison[0]
		} else {
			c.elem, c.elemText, c.cv = make([]types.Type, K.nargs), make([]string, K.nargs), make([]rw.Conv, K.nargs)
			for i := 0; i < K.nargs && why == ""; i++ {
				el, et, w := c.element(i)
				switch {
				case w != "":
					why = w
				case el == nil:
					c.elem[i], c.elemText[i], c.cv[i] = et, "any", rw.ConvFor(et)
				default:
					if cv := rw.ConvFor(et); !cv.OK {
						why = "element type " + rw.Text(el) + " has no known boxing"
					} else {
						c.elem[i], c.elemText[i], c.cv[i] = et, rw.Text(el), cv
					}
				}
			}
			if why == "" && c.anyAt != "" && concreteCasts > 0 {
				why = "boxed into an Object, and Objects are cast back to the concrete runtime type elsewhere"
				poisonAt[why] = c.anyAt
			}
			if why == "" && K.name == "deque" {
				switch {
				case c.dqHeadPush != "" && c.dqTail != "":
					why = "pushed at its head (" + c.dqHeadPush + ") and used at its tail (" + c.dqTail + ")"
				}
			}
			if why == "" && c.strKey && !isString(c.elem[0]) {
				why = "string-key helper on a collection whose key is not a string"
			}
		}
		if why != "" {
			reasons[why] += len(c.typeExpr)
			classWhy[r] = why
			continue
		}
		good = append(good, c)
		converted[r] = c
		nGood += len(c.typeExpr)
	}
	// Constructors of the kind, by what their class decided.
	ctorReasons := map[string]int{}
	ctorAt := map[string]string{}
	nCtor, nCtorGood := 0, 0
	for e, d := range ctorDest {
		nCtor++
		r := find(d)
		if _, ok := converted[r]; ok {
			nCtorGood++
			continue
		}
		why := "class not decided"
		if c := data[r]; c != nil && len(c.poison) > 0 {
			why = c.poison[0]
		} else if c != nil && classWhy[r] != "" {
			why = classWhy[r]
		}
		ctorReasons[why]++
		if _, ok := ctorAt[why]; !ok {
			p := rw.Fset.Position(e.Pos())
			ctorAt[why] = fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
		}
	}
	for e, why := range ctorWhy {
		if _, ok := ctorDest[e]; ok {
			continue
		}
		nCtor++
		ctorReasons[why]++
		if _, ok := ctorAt[why]; !ok {
			p := rw.Fset.Position(e.Pos())
			ctorAt[why] = fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
		}
	}
	if os.Getenv("COLLS_CTORS") != "" {
		fmt.Fprintf(os.Stderr, "%s constructors: %d; in typed classes: %d\n", K.name, nCtor, nCtorGood)
		var cr []string
		for r := range ctorReasons {
			cr = append(cr, r)
		}
		sort.Slice(cr, func(i, j int) bool {
			if ctorReasons[cr[i]] != ctorReasons[cr[j]] {
				return ctorReasons[cr[i]] > ctorReasons[cr[j]]
			}
			return cr[i] < cr[j]
		})
		for _, r := range cr {
			fmt.Fprintf(os.Stderr, "  ctor stays (%d): %s  [%s]\n", ctorReasons[r], r, ctorAt[r])
		}
	}
	total := 0
	for _, n := range reasons {
		total += n
	}
	fmt.Fprintf(os.Stderr, "%s declarations in classes: %d; become typed: %d (%d classes)\n", K.name, total+nGood, nGood, len(good))
	var rs []string
	for r := range reasons {
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool { return reasons[rs[i]] > reasons[rs[j]] })
	for i, r := range rs {
		if i >= 25 {
			break
		}
		fmt.Fprintf(os.Stderr, "  stays (%d declarations): %s  [%s]\n", reasons[r], r, poisonAt[r])
	}
	if verbose {
		for _, c := range good {
			p := rw.Fset.Position(c.typeExpr[0].Pos())
			fmt.Printf("class\t%d decls\t%s:%d\t%s\n", len(c.typeExpr), filepath.Base(p.Filename), p.Line, K.goType(c))
		}
	}
	sort.Slice(good, func(i, j int) bool { return good[i].typeExpr[0].Pos() < good[j].typeExpr[0].Pos() })
	if r := os.Getenv("COLLS_RANGE"); r != "" { // bisection aid: apply only classes [a, b)
		var a, b int
		fmt.Sscanf(r, "%d,%d", &a, &b)
		good = good[a:min(b, len(good))]
		for _, c := range good {
			p := rw.Fset.Position(c.typeExpr[0].Pos())
			fmt.Fprintf(os.Stderr, "applying %s:%d %s\n", filepath.Base(p.Filename), p.Line, K.goType(c))
		}
	}
	if report {
		return
	}
	for _, c := range good {
		c := c
		for _, te := range c.typeExpr {
			te := te
			rw.Rewrite[te] = func() string { return K.goType(c) }
		}
		for _, f := range c.edits {
			e := f(c)
			if _, dup := rw.Rewrite[e.Node]; !dup {
				rw.Rewrite[e.Node] = e.Gen
			}
		}
	}
	dir := os.Args[len(os.Args)-1]
	fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
	// Conversion variables (collected while rendering).
	var names []string
	for n := range convs {
		names = append(names, n)
	}
	sort.Strings(names)
	var sb strings.Builder
	out := filepath.Join(dir, "g_zconv_"+K.name+"s.go")
	if old, err := os.ReadFile(out); err == nil {
		sb.Write(old) // conversions of an earlier run stay; the new ones follow
	} else {
		sb.WriteString("// Code generated by tools/postflip/colls. DO NOT EDIT.\n\npackage closure\n\n// Element conversions of the typed " + K.name + "s.\n\n")
	}
	for _, n := range names {
		sb.WriteString(convs[n] + "\n")
	}
	if len(names) == 0 {
		return
	}
	if err := os.WriteFile(out, []byte(sb.String()), 0o644); err != nil {
		panic(err)
	}
}
