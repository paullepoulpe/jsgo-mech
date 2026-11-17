package main

// -kind immlist, immset, immmap: Guava's ImmutableList<T>, ImmutableSet<E> and
// ImmutableMap<K, V> (slots declared ImmutableListOf[T], *ErasedImmutableList...) become the
// typed *ImmutableList[T], *ImmutableSet[K] and *ImmutableMap[K, V] of rt_timm.go (the typed
// lists, sets and maps of rt_tlist.go and rt_tmap.go, immutable), with the class analysis of
// the other kinds. Run after step immnames, in the order immlist, immset, immmap, immlistb,
// immsetb, immmapb (the builders, below).
//
// An immutable collection never changes, so typing one changes no aliasing; what must be kept
// is its identity and its erased face. The typed collection keeps a twin, the runtime's
// erased collection with the same elements, which erased code sees (asImm, keeping every
// conversion the code made around the value, so null stays null): made from an erased one
// (tlistImm_, tsetFrom_, tmapFrom_), the typed collection's twin is that one; made typed, it
// gets a twin, boxed, the first time erased code needs it, and the twin keeps the typed
// collection, so a value crossing to erased code and back is the same object.
//
//   - ImmutableList.of(a, b...) is immutableListOf_(tc, a, b...) with the elements unboxed
//     where the code boxed them (a null throws NullPointerException as Guava's of does; of()
//     is the one empty list of the type, whose twin is the runtime's empty list); copyOf(x)
//     is immutableListCopyOf_ (an immutable collection gives its asList() itself, anything
//     else is copied and checked); the same for sets, and ImmutableMap.of(k1, v1...) with
//     the keys and values unboxed, nulls and duplicate keys thrown as ImmutableMap.of throws
//     them. Any other immutable collection from erased code (a builder's build(), a cast from
//     Object, a runtime result, a generic class's type-parameter slot) is converted
//     (tlistImm_, tsetFrom_, tmapFrom_, cached on it). An of/copyOf converted on the spot by
//     an earlier step's typed class (tlistImm_(tc, ImmutableList.of(...))...) is made typed.
//   - Reads are typed: get, size, isEmpty; contains, containsKey, get and getOrDefault of a
//     set or map when the argument is a value of the element type (an Object of another
//     class gives false or null in Java, so those go through the twin); for-each loops over
//     the elements (range immElems_(x) is range x.All(), the element unboxed where the code
//     unboxed it; foreach_ cursors as for the other kinds). Changes (add, put...) and every
//     other method go through the twin, which throws or answers as before.
//   - An ImmutableSortedSet or ImmutableSortedMap stored into an ImmutableSet or ImmutableMap
//     slot (it is one in Java) leaves the class erased: its lookups use its comparator.
//   - Methods are joined only within a family that can be called for one another (an
//     interface method and its implementations, a method and those of the types embedding
//     its receiver, m and m_impl), not by name and erased signature alone; slots stating type
//     arguments that cannot meet (Object and another type: an erased type variable) are kept
//     apart, values converted between them. Enum constants (integer types) are elements too.
//
// Element types are the declared type arguments, or, for raw declarations (static fields j2go
// wrote without them), the static types of the elements the code stores (the arguments of
// of(...)); erased code seeing such a class does not matter, as it cannot store into it.

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"regexp"
	"strings"

	"postflip/rw"
)

func immKinds() map[string]*kindDef {
	look := func(n string) types.Type { return rw.Pkg.Scope().Lookup(n).Type() }
	ptr := func(n string) types.Type { return types.NewPointer(look(n)) }
	return map[string]*kindDef{
		"immlist": {
			imm: "ImmutableList", name: "immlist", base: "list", aliases: []string{"ImmutableListOf"}, nargs: 1,
			newFns: map[string]int{}, copyFns: map[string]bool{}, immutable: map[string]bool{"ImmutableCollection": true},
			view:   "asImm",
			views:  []types.Type{ptr("ImmutableCollection"), look("List"), look("Collection"), look("Iterable"), look("SequencedCollection")},
			goType: func(c *class) string { return "*ImmutableList[" + c.elemText[0] + "]" },
			fromCode: func(c *class, from string, imm bool) string {
				return "tlistImm_(" + c.convName(0) + ", " + from + ")"
			},
		},
		"immset": {
			imm: "ImmutableSet", name: "immset", base: "set", aliases: []string{"ImmutableSetOf"}, nargs: 1,
			newFns: map[string]int{}, copyFns: map[string]bool{}, immutable: map[string]bool{"ImmutableCollection": true},
			view:    "asImm",
			views:   []types.Type{ptr("ImmutableCollection"), look("Set"), look("Collection"), look("Iterable")},
			helpers: map[string]string{"Collection_contains_s": "contains"},
			goType:  func(c *class) string { return "*ImmutableSet[" + c.elemText[0] + "]" },
			fromCode: func(c *class, from string, imm bool) string {
				return "tsetFrom_(" + c.convName(0) + ", " + from + ", true)"
			},
		},
		"immlistb": builderKind("immlistb", "ImmutableList", "list", 1, "immutableListBuilder_"),
		"immsetb":  builderKind("immsetb", "ImmutableSet", "set", 1, "immutableSetBuilder_"),
		"immmapb":  builderKind("immmapb", "ImmutableMap", "map", 2, "immutableMapBuilder_"),
		"immmap": {
			imm: "ImmutableMap", name: "immmap", base: "map", aliases: []string{"ImmutableMapOf"}, nargs: 2,
			newFns: map[string]int{}, copyFns: map[string]bool{}, immutable: map[string]bool{"ErasedImmutableMap": true},
			view:    "asImm",
			views:   []types.Type{ptr("ErasedImmutableMap"), look("Map")},
			helpers: map[string]string{"Map_get_s": "get", "Map_containsKey_s": "containsKey", "Map_getOrDefault_s": "getOrDefault"},
			goType:  func(c *class) string { return "*ImmutableMap[" + c.elemText[0] + ", " + c.elemText[1] + "]" },
			fromCode: func(c *class, from string, imm bool) string {
				return "tmapFrom_(" + c.convName(0) + ", " + c.convName(1) + ", " + from + ", true)"
			},
		},
	}
}

// immName is the Guava name of the erased immutable type t states: ImmutableList for
// *ErasedImmutableList and ImmutableListOf[T], ImmutableCollection for the struct itself, ""
// for other types, the typed ImmutableList[T] among them.
func immName(t types.Type) string {
	switch x := t.(type) {
	case *types.Alias:
		n := x.Obj().Name()
		if x.TypeArgs().Len() > 0 && strings.HasSuffix(n, "Of") {
			return strings.TrimSuffix(n, "Of")
		}
		return immName(x.Rhs())
	case *types.Pointer:
		switch e := x.Elem().(type) {
		case *types.Alias:
			if e.TypeArgs().Len() > 0 {
				return "" // a typed collection
			}
			return strings.TrimPrefix(e.Obj().Name(), "Erased")
		case *types.Named:
			if e.TypeArgs().Len() > 0 {
				return ""
			}
			return strings.TrimPrefix(e.Obj().Name(), "Erased")
		}
	}
	return ""
}

var immCtorRx = regexp.MustCompile(`^(Immutable\w+_(of|copyOf|builder)\w*|new_Immutable\w+|immList_|immSet_|immMap_)$`)

// isCtor reports whether e is a call the census counts as an immutable constructor.
func isCtor(e ast.Expr) bool {
	n, c := rw.CallName(e)
	return c != nil && immCtorRx.MatchString(n)
}

// immValue classifies call x (named n) of the kind's immutable type made by the runtime.
func immValue(x *ast.CallExpr, n string) (value, bool) {
	if _, ok := rw.Unparen(x.Fun).(*ast.FuncLit); ok {
		return value{}, false
	}
	if f := calleeFunc(x); f != nil && generated(f.Pos()) {
		return value{}, false
	}
	if fam, ok := strings.CutSuffix(K.imm, "_Builder"); ok {
		switch {
		case (n == fam+"_builder" || n == "new_"+K.imm) && len(x.Args) == 0:
			return value{kind: vOf}, true
		case n == fam+"_builderWithExpectedSize" && len(x.Args) == 1:
			return value{kind: vOf, args: x.Args}, true
		}
		return value{kind: vForeign, why: "result of " + n}, true
	}
	switch {
	case strings.HasPrefix(n, K.imm+"_of"):
		args := x.Args
		if strings.HasSuffix(n, "Arr") && len(args) > 0 {
			cl, ok := rw.Unparen(args[len(args)-1]).(*ast.CompositeLit)
			if !ok {
				return value{kind: vImm, from: x}, true
			}
			args = append(args[:len(args)-1:len(args)-1], cl.Elts...)
		}
		if K.base == "map" && len(args)%2 != 0 {
			return value{kind: vImm, from: x}, true
		}
		return value{kind: vOf, args: args}, true
	case n == K.imm+"_copyOf" || n == K.imm+"_copyOf_Iterable":
		if len(x.Args) == 1 {
			return value{kind: vCopyOf, args: x.Args}, true
		}
	}
	return value{kind: vImm, from: x}, true
}

// immFlow records the constructor v (of or copyOf) flowing into slot dest at top.
func immFlow(dest int, v value, top ast.Expr) {
	args := v.args
	switch v.kind {
	case vOf:
		if !isBuilder() {
			for i, a := range args {
				note(dest, i%K.nargs, a)
			}
		}
		edit(dest, func(c *class) rw.Edit {
			return rw.Edit{Node: top, Gen: func() string { return immOfCode(c, args) }}
		})
	case vCopyOf:
		inferBad(dest, -1, "an erased collection is copied in")
		edit(dest, func(c *class) rw.Edit {
			return rw.Edit{Node: top, Gen: func() string {
				switch K.base {
				case "list":
					return "immutableListCopyOf_(" + c.convName(0) + ", " + rw.Render(args[0]) + ")"
				case "set":
					return "immutableSetCopyOf_(" + c.convName(0) + ", " + rw.Render(args[0]) + ")"
				}
				return "immutableMapCopyOf_(" + c.convName(0) + ", " + c.convName(1) + ", " + rw.Render(args[0]) + ")"
			}}
		})
	}
}

// immOfCode is the typed of(args...) of class c.
func immOfCode(c *class, args []ast.Expr) string {
	if isBuilder() {
		ps := []string{}
		for i := 0; i < K.nargs; i++ {
			ps = append(ps, c.convName(i))
		}
		f := builderCtor[K.name]
		if len(args) == 1 {
			f = strings.TrimSuffix(f, "_") + "N_"
			ps = append(ps, rw.Render(args[0]))
		}
		return f + "(" + strings.Join(ps, ", ") + ")"
	}
	un := func(i int, a ast.Expr) string { return unboxElem(c.cv[i], c.elem[i], a) }
	switch K.base {
	case "list", "set":
		ps := []string{c.convName(0)}
		for _, a := range args {
			ps = append(ps, un(0, a))
		}
		f := map[string]string{"list": "immutableListOf_", "set": "immutableSetOf_"}[K.base]
		if _, ok := types.Unalias(c.elem[0]).Underlying().(*types.Interface); ok && len(args) > 0 {
			f += "[" + c.elemText[0] + "]" // the elements may be of classes that implement it
		}
		return f + "(" + strings.Join(ps, ", ") + ")"
	}
	var ks, vs []string
	for i := 0; i < len(args); i += 2 {
		ks = append(ks, un(0, args[i]))
		vs = append(vs, un(1, args[i+1]))
	}
	return "immutableMapOf_(" + c.convName(0) + ", " + c.convName(1) + ", []" + c.elemText[0] + "{" + strings.Join(ks, ", ") +
		"}, []" + c.elemText[1] + "{" + strings.Join(vs, ", ") + "})"
}

// immForeign says why value r, of another type than the kind's, cannot enter a class of the
// kind ("" if it can, converted).
func immForeign(r ast.Expr) string {
	switch n := immName(typeOf(r)); n {
	case "ImmutableSortedSet", "ImmutableSortedMap":
		return "an " + n + " is stored (its lookups use its comparator)"
	case "ImmutableCollection":
		// b.build() of a builder of the kind (b.add(x).build() too).
		if call, ok := rw.Unparen(r).(*ast.CallExpr); ok && len(call.Args) == 0 {
			if s, ok := call.Fun.(*ast.SelectorExpr); ok && (s.Sel.Name == "build" || s.Sel.Name == "buildOrThrow") && builderRoot(s.X) == K.imm+"_Builder" {
				return ""
			}
		}
		return "an immutable collection of another kind is stored"
	}
	return ""
}

// exactArg is the unboxed code of argument a when it is a value of the class's type argument i
// boxed (obj_(x), strobj_(x), x.self()...), or a string for a class of strings.
func exactArg(c *class, i int, a ast.Expr, isStr bool) (string, bool) {
	a = rw.Unparen(a)
	if isStr {
		if isString(c.elem[i]) {
			return rw.Render(a), true
		}
		return "", false
	}
	if call, ok := a.(*ast.CallExpr); ok {
		var inner ast.Expr
		if id, ok := call.Fun.(*ast.Ident); ok && len(call.Args) == 1 && (id.Name == "obj_" || id.Name == "strobj_" || id.Name == "any") {
			inner = call.Args[0]
		} else if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
			inner = s.X
		}
		if inner != nil && types.Identical(typeOf(inner), c.elem[i]) {
			return rw.Render(inner), true
		}
	}
	return "", false
}

// immUse handles the call of method m on a collection of an imm kind: the typed reads.
func immUse(k int, recv ast.Expr, m string, call *ast.CallExpr, read func(int, string, ...func(*class) string), raw func(int) func(*class) string) bool {
	args := call.Args
	n := len(args)
	_, isStmt := rw.Up(call).(*ast.ExprStmt)
	R := func() string { return rw.Render(recv) }
	if isBuilder() {
		builderUse(k, recv, m, call)
		return true
	}
	if (m == "size" || m == "isEmpty") && n == 0 {
		return true
	}
	if K.base == "list" && m == "get" && n == 1 {
		read(0, "get", raw(0))
		return true
	}
	// lookup: typed when the argument is a value of the key type, else through the twin.
	lookup := func(name, typed string, isStr bool, ci int) bool {
		for _, a := range args[:1] {
			note(k, 0, a)
		}
		viewCall := func() string {
			var ps []string
			for _, a := range args {
				ps = append(ps, rw.Render(a))
			}
			return R() + ".asImm()." + name + "(" + strings.Join(ps, ", ") + ")"
		}
		edit(k, func(c *class) rw.Edit {
			key, ok := exactArg(c, 0, args[0], isStr)
			if !ok {
				return rw.Edit{Node: call, Gen: viewCall}
			}
			g := func() string {
				ps := []string{key}
				if n == 2 {
					ps = append(ps, rw.Unboxed(c.cv[1], c.elem[1], args[1]))
				}
				return R() + "." + typed + "(" + strings.Join(ps, ", ") + ")"
			}
			if ci < 0 || isStmt {
				return rw.Edit{Node: call, Gen: g}
			}
			return rw.Read(c.cv[ci], call, g)
		})
		return true
	}
	switch K.base {
	case "set":
		switch {
		case m == "contains" && n == 1:
			return lookup(m, "contains", false, -1)
		case m == "contains_s" && n == 1:
			return lookup(m, "contains", true, -1)
		}
	case "map":
		switch {
		case m == "get" && n == 1:
			return lookup(m, "get", false, 1)
		case m == "get_s" && n == 1:
			return lookup(m, "get", true, 1)
		case m == "containsKey" && n == 1:
			return lookup(m, "containsKey", false, -1)
		case m == "containsKey_s" && n == 1:
			return lookup(m, "containsKey", true, -1)
		case m == "getOrDefault" && n == 2:
			note(k, 1, args[1])
			return lookup(m, "getOrDefault", false, 1)
		}
	}
	return false
}

// immRange handles `for _, el := range immElems_(x)` over a collection of class k: a range over
// x.All(), each use of el unboxed where the code unboxed it, boxed elsewhere.
func immRange(k int, e ast.Expr, call *ast.CallExpr) bool {
	r, ok := rw.Parent[call].(*ast.RangeStmt)
	if !ok || r.X != call || r.Tok == token.ASSIGN || K.base == "map" {
		return false
	}
	inner := unwrap(e)
	if r.Key == nil {
		edit(k, func(c *class) rw.Edit {
			return rw.Edit{Node: call, Gen: func() string { return rw.Render(inner) + ".All()" }}
		})
		return true
	}
	key, _ := r.Key.(*ast.Ident)
	val, _ := r.Value.(*ast.Ident)
	if key == nil || key.Name != "_" || val == nil {
		return false
	}
	obj := rw.Info.Defs[val]
	if obj == nil {
		return false
	}
	uses := rw.UsesOf[obj]
	for _, u := range uses {
		if as, ok := rw.Parent[u].(*ast.AssignStmt); ok {
			for _, l := range as.Lhs {
				if l == u {
					return false
				}
			}
		}
		if _, ok := rw.Parent[u].(*ast.UnaryExpr); ok {
			return false // &el
		}
	}
	edit(k, func(c *class) rw.Edit {
		return rw.Edit{Node: r, Gen: func() string {
			return "for " + val.Name + " := range " + rw.Render(inner) + ".All() " + rw.Render(r.Body)
		}}
	})
	for _, u := range uses {
		u := u
		edit(k, func(c *class) rw.Edit { return rw.Read(c.cv[0], u, func() string { return u.Name }) })
	}
	return true
}

// builderRoot is the immutable type name of the builder at the root of a chain of builder
// calls b.add(x).addAll(y) (whose results the runtime declares as the common builder type).
func builderRoot(e ast.Expr) string {
	for {
		e = rw.Unparen(e)
		if n := immName(typeOf(e)); n != "ImmutableCollection_Builder" {
			return n
		}
		call, ok := e.(*ast.CallExpr)
		if !ok {
			return ""
		}
		s, ok := call.Fun.(*ast.SelectorExpr)
		if !ok {
			return ""
		}
		switch s.Sel.Name {
		case "add", "addAll", "add_ObjectArr", "put", "putAll":
			e = s.X
		default:
			return ""
		}
	}
}

// ---- method families
//
// The other kinds join every method of one name and erased signature, so that interfaces stay
// satisfied. Immutable collections are declared with their element types in many unrelated
// classes (build(), getParameters()...), so the imm kinds join only methods that can be called
// for one another: an interface method and the methods of the types that implement the
// interface, methods of a type and of the types it embeds (Java's subclasses), and a method m
// with its m_impl.

var families map[*types.Func]string

// methodFamily is the suffix that tells apart the families of f's name and erased signature.
func methodFamily(f *types.Func) string {
	if families == nil {
		families = computeFamilies()
	}
	return families[f]
}

func recvNamed(f *types.Func) *types.Named {
	sig := f.Type().(*types.Signature)
	if sig.Recv() == nil {
		return nil
	}
	t := sig.Recv().Type()
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	n, _ := types.Unalias(t).(*types.Named)
	return n
}

// embeds reports whether struct type a embeds b, at any depth.
func embeds(a, b *types.Named, seen map[*types.Named]bool) bool {
	if seen[a] {
		return false
	}
	seen[a] = true
	st, ok := a.Underlying().(*types.Struct)
	if !ok {
		return false
	}
	for i := 0; i < st.NumFields(); i++ {
		fl := st.Field(i)
		if !fl.Embedded() {
			continue
		}
		t := fl.Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		if n, ok := types.Unalias(t).(*types.Named); ok {
			if n.Origin() == b.Origin() || embeds(n, b, seen) {
				return true
			}
		}
	}
	return false
}

func related(a, b *types.Func) bool {
	ra, rb := recvNamed(a), recvNamed(b)
	if ra == nil || rb == nil {
		// A method of an interface literal: the other's receiver must implement it.
		anon, other := a, rb
		if ra != nil {
			anon, other = b, ra
		}
		it, ok := anon.Type().(*types.Signature).Recv().Type().Underlying().(*types.Interface)
		if !ok || other == nil {
			return true
		}
		return types.Implements(types.NewPointer(other), it) || types.Implements(other, it)
	}
	if ra.Origin() == rb.Origin() {
		return true
	}
	ia, aIface := ra.Underlying().(*types.Interface)
	ib, bIface := rb.Underlying().(*types.Interface)
	switch {
	case aIface && bIface:
		return types.Implements(ra, ib) || types.Implements(rb, ia)
	case aIface:
		return types.Implements(types.NewPointer(rb), ia) || types.Implements(rb, ia)
	case bIface:
		return types.Implements(types.NewPointer(ra), ib) || types.Implements(ra, ib)
	}
	return embeds(ra, rb, map[*types.Named]bool{}) || embeds(rb, ra, map[*types.Named]bool{})
}

func computeFamilies() map[*types.Func]string {
	byKey := map[string][]*types.Func{}
	seen := map[*types.Func]bool{}
	add := func(f *types.Func) {
		if f == nil || seen[f] || f.Type().(*types.Signature).Recv() == nil {
			return
		}
		seen[f] = true
		sig := f.Type().(*types.Signature)
		k := strings.TrimSuffix(f.Name(), "_impl")
		for i := 0; i < sig.Params().Len(); i++ {
			k += "," + types.TypeString(types.Unalias(sig.Params().At(i).Type()), nil)
		}
		k += "|"
		for i := 0; i < sig.Results().Len(); i++ {
			k += "," + types.TypeString(types.Unalias(sig.Results().At(i).Type()), nil)
		}
		byKey[k] = append(byKey[k], f)
	}
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				o, _ := rw.Info.Defs[x.Name].(*types.Func)
				add(o)
			case *ast.InterfaceType:
				for _, m := range x.Methods.List {
					for _, id := range m.Names {
						o, _ := rw.Info.Defs[id].(*types.Func)
						add(o)
					}
				}
			}
			return true
		})
	}
	out := map[*types.Func]string{}
	for _, fs := range byKey {
		par := make([]int, len(fs))
		for i := range par {
			par[i] = i
		}
		var root func(int) int
		root = func(i int) int {
			for par[i] != i {
				par[i] = par[par[i]]
				i = par[i]
			}
			return i
		}
		for i := range fs {
			for j := i + 1; j < len(fs); j++ {
				if root(i) != root(j) && related(fs[i], fs[j]) {
					par[root(j)] = root(i)
				}
			}
		}
		for i, f := range fs {
			out[f] = fmt.Sprintf("#%d", fs[root(i)].Pos())
		}
	}
	return out
}

// statedApart reports whether slots a and b state type arguments that cannot be one class: one
// states Object (an erased type variable, a generic method's ImmutableList<T>) and the other a
// type, or neither type is assignable to the other. Values flow between them converted.
func statedApart(a, b int) bool {
	sa, sb := stated[a], stated[b]
	if sa == nil || sb == nil {
		return false
	}
	isAny := func(t types.Type) bool {
		it, ok := types.Unalias(t).(*types.Interface)
		return ok && it.Empty()
	}
	for i := range sa {
		ta, tb := typeOf(sa[i]), typeOf(sb[i])
		if ta == nil || tb == nil || types.Identical(ta, tb) {
			continue
		}
		if isAny(ta) != isAny(tb) || !types.AssignableTo(ta, tb) && !types.AssignableTo(tb, ta) {
			return true
		}
	}
	return false
}

// convFlow is a flow from slot src to slot dest of classes kept apart: the value is converted
// on the way, through its twin, whichever of the two classes become typed.
func convFlow(dest, src int, inner, top ast.Expr) {
	inferBad(dest, -1, "an erased collection is copied in")
	gen := func() string {
		var s string
		if inner == top {
			s = rw.RenderChildren(inner)
		} else {
			s = rw.Render(inner)
		}
		if find(src) == find(dest) {
			return s // joined after all, through other flows
		}
		if _, ok := converted[find(src)]; ok {
			s += "." + K.view + "()"
		}
		if d, ok := converted[find(dest)]; ok {
			s = K.fromCode(d, s, true)
		}
		return s
	}
	edit(dest, func(c *class) rw.Edit { return rw.Edit{Node: top, Gen: gen} })
	edit(src, func(c *class) rw.Edit { return rw.Edit{Node: top, Gen: gen} })
}

// enclosingResult is the declared type of result i of the function that return statement r
// is in, or nil.
func enclosingResult(r *ast.ReturnStmt, i int) types.Type {
	for p := rw.Parent[r]; p != nil; p = rw.Parent[p] {
		var sig *types.Signature
		switch f := p.(type) {
		case *ast.FuncLit:
			sig, _ = typeOf(f).(*types.Signature)
		case *ast.FuncDecl:
			if o, ok := rw.Info.Defs[f.Name].(*types.Func); ok {
				sig = o.Type().(*types.Signature)
			}
		default:
			continue
		}
		if sig == nil || i >= sig.Results().Len() {
			return nil
		}
		return sig.Results().At(i).Type()
	}
	return nil
}

// ---- builders: -kind immlistb, immsetb, immmapb
//
// ImmutableList.Builder<T> (ImmutableList_BuilderOf[T]) and the set and map builders become the
// typed ImmutableList_Builder[T]... of rt_timm.go: add and put take the element unboxed,
// addAll and putAll an erased collection, build() is the typed immutable collection. Run
// after -kind immlist, immset, immmap: a build() whose value a typed class converts
// (tlistImm_(tc, b.build()), tsetFrom_, tmapFrom_, sliceImm_) is that class's value itself
// when the element types agree; anywhere else build() gives its twin (asImm), the erased
// collection the code had. A chain b.add(x).addAll(y).build() is typed along, also from a
// builder made on the spot (ImmutableList.builder().add(x)...), which is a class of its own.
// A builder that leaves for erased code, or whose add result is used otherwise than by
// the chain, stays erased.

func builderKind(name, fam, base string, nargs int, ctor string) *kindDef {
	return &kindDef{
		imm: fam + "_Builder", name: name, base: base + "b", aliases: []string{fam + "_BuilderOf"}, nargs: nargs,
		newFns: map[string]int{}, copyFns: map[string]bool{}, immutable: map[string]bool{}, views: []types.Type{},
		goType: func(c *class) string { return "*" + fam + "_Builder[" + strings.Join(c.elemText, ", ") + "]" },
	}
}

func isBuilder() bool { return strings.HasSuffix(K.imm, "_Builder") }

func init() {
	// the constructors' runtime names, keyed "builder" (newFns of a builder kind is not looked up by name)
	builderCtor = map[string]string{"immlistb": "immutableListBuilder_", "immsetb": "immutableSetBuilder_", "immmapb": "immutableMapBuilder_"}
}

var builderCtor map[string]string

// convType is T of a *tconv_[T] expression, or nil.
func convType(e ast.Expr) types.Type {
	p, ok := types.Unalias(typeOf(e)).(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.Obj().Name() != "tconv_" || n.TypeArgs().Len() != 1 {
		return nil
	}
	return n.TypeArgs().At(0)
}

// builderUse handles the call of method m on a builder of class k.
func builderUse(k int, recv ast.Expr, m string, call *ast.CallExpr) {
	args := call.Args
	n := len(args)
	R := func() string { return rw.Render(recv) }
	chain := func() {
		// the builder the call returns: statement, or the receiver of the next builder call
		if _, ok := rw.Up(call).(*ast.ExprStmt); ok {
			return
		}
		if s, ok := rw.Parent[call].(*ast.SelectorExpr); ok && s.X == call {
			if c2, ok := rw.Parent[s].(*ast.CallExpr); ok && c2.Fun == s {
				use(k, call, s.Sel.Name, c2)
				return
			}
		}
		poison(k, "the builder an add returns is used as a value")
	}
	switch {
	case (m == "add" && K.base == "listb" || m == "add" && K.base == "setb") && n == 1:
		note(k, 0, args[0])
		edit(k, func(c *class) rw.Edit {
			return rw.Edit{Node: call, Gen: func() string { return R() + ".add(" + unboxElem(c.cv[0], c.elem[0], args[0]) + ")" }}
		})
		chain()
	case m == "put" && K.base == "mapb" && n == 2:
		note(k, 0, args[0])
		note(k, 1, args[1])
		edit(k, func(c *class) rw.Edit {
			return rw.Edit{Node: call, Gen: func() string {
				return R() + ".put(" + unboxElem(c.cv[0], c.elem[0], args[0]) + ", " + unboxElem(c.cv[1], c.elem[1], args[1]) + ")"
			}}
		})
		chain()
	case (m == "addAll" && K.base != "mapb" || m == "putAll" && K.base == "mapb") && n == 1:
		chain()
	case (m == "build" || m == "buildOrThrow" || K.base == "mapb" && m == "buildKeepingLast") && n == 0:
		builtUse(k, recv, m, call)
	default:
		poison(k, "builder method "+m)
	}
}

// builtUse handles b.build(): the typed collection where a typed class converts it, else its twin.
func builtUse(k int, recv ast.Expr, m string, call *ast.CallExpr) {
	R := func() string { return rw.Render(recv) + "." + m + "()" }
	var conv *ast.CallExpr
	var convs []ast.Expr
	if pc, ok := rw.Up(call).(*ast.CallExpr); ok {
		name, _ := rw.CallName(pc)
		at := map[string]int{"tlistImm_": 1, "sliceImm_": 1, "tsetFrom_": 1, "tmapFrom_": 2}[name]
		want := map[string]string{"listb": "tlistImm_ sliceImm_", "setb": "tsetFrom_", "mapb": "tmapFrom_"}[K.base]
		if at > 0 && len(pc.Args) > at && rw.Unparen(pc.Args[at]) == call && strings.Contains(want, name) {
			conv, convs = pc, pc.Args[:at]
		}
	}
	if conv != nil {
		c := cls(k)
		if c.inferred == nil {
			c.inferred = make([][]types.Type, K.nargs)
		}
		for i, e := range convs {
			if t := convType(e); t != nil {
				c.inferred[i] = append(c.inferred[i], t)
			}
		}
	}
	edit(k, func(c *class) rw.Edit {
		if conv != nil {
			same := true
			for i, e := range convs {
				same = same && types.Identical(convType(e), c.elem[i])
			}
			if same {
				if n, _ := rw.CallName(conv); n == "sliceImm_" {
					return rw.Edit{Node: conv, Gen: func() string { return R() + ".items" }}
				}
				return rw.Edit{Node: conv, Gen: R}
			}
		}
		return rw.Edit{Node: call, Gen: func() string { return R() + ".asImm()" }}
	})
}

// ---- type parameters: a generic class's slot of type T holds an immutable collection for
// some instantiations (Outcome<ImmutableMap<K, V>>); the class's code is not the kind's, so
// values going there leave for erased code and values coming from there are converted.

func isTypeParam(t types.Type) bool {
	_, ok := t.(*types.TypeParam)
	return ok
}

func genericVar(v *types.Var) bool { return isTypeParam(v.Origin().Type()) }

func genericResult(f *types.Func) bool {
	r := f.Origin().Type().(*types.Signature).Results()
	return r.Len() == 1 && isTypeParam(r.At(0).Type())
}

func genericParam(f *types.Func, i int) bool {
	p := f.Origin().Type().(*types.Signature).Params()
	return i < p.Len() && isTypeParam(p.At(i).Type())
}

// ---- constructors converted on the spot
//
// tlistImm_(tc, ImmutableList.of(a, b)), tsetFrom_(tc, ImmutableSet.of(...), true),
// tmapFrom_(kc, vc, ImmutableMap.of(...), true) and sliceImm_(tc, ImmutableList.of(...)) (an
// immutable collection made for a typed class of an earlier step, which copied it) are the
// typed collection made directly; copyOf likewise. The typed copy of the erased collection had
// that collection as its twin, the typed one gets one when erased code needs it: the same
// elements, and no one else held the erased one.

func immDirect(report bool) {
	n := 0
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			pc, ok := x.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, _ := rw.CallName(pc)
			at := map[string]int{"tlistImm_": 1, "sliceImm_": 1, "tsetFrom_": 1, "tmapFrom_": 2}[name]
			want := map[string]string{"list": "tlistImm_ sliceImm_", "set": "tsetFrom_", "map": "tmapFrom_"}[K.base]
			if at == 0 || !strings.Contains(want, name) || len(pc.Args) <= at {
				return true
			}
			if (name == "tsetFrom_" || name == "tmapFrom_") && (len(pc.Args) != at+2 || rw.Text(pc.Args[at+1]) != "true") {
				return true
			}
			arg, ok := rw.Unparen(pc.Args[at]).(*ast.CallExpr)
			if !ok || !isList(typeOf(arg)) {
				return true
			}
			cn, _ := rw.CallName(arg)
			v, ok := immValue(arg, cn)
			if !ok || v.kind != vOf && v.kind != vCopyOf {
				return true
			}
			c := &class{elem: make([]types.Type, K.nargs), elemText: make([]string, K.nargs), cv: make([]rw.Conv, K.nargs)}
			for i := 0; i < K.nargs; i++ {
				t := convType(pc.Args[i])
				if t == nil {
					return true
				}
				cv := convFor(t)
				if !cv.OK {
					return true
				}
				c.elem[i], c.elemText[i], c.cv[i] = t, types.TypeString(t, types.RelativeTo(rw.Pkg)), cv
				if i == 0 && K.base != "list" && keyMode(t) == rw.KeyHash {
					c.hashKey = true
				}
			}
			convs := make([]string, K.nargs)
			for i := range convs {
				convs[i] = rw.Text(pc.Args[i])
			}
			n++
			args := v.args
			kind := v.kind
			rw.Rewrite[pc] = func() string {
				c.convText = convs
				var s string
				if kind == vOf {
					s = immOfCode(c, args)
				} else {
					s = map[string]string{"list": "immutableListCopyOf_(", "set": "immutableSetCopyOf_(", "map": "immutableMapCopyOf_("}[K.base] +
						strings.Join(convs, ", ") + ", " + rw.Render(args[0]) + ")"
				}
				if name == "sliceImm_" {
					s += ".items"
				}
				return s
			}
			return true
		})
	}
	fmt.Fprintf(os.Stderr, "constructors converted on the spot (tlistImm_, tsetFrom_, tmapFrom_, sliceImm_ of of/copyOf): %d\n", n)
}

// unboxElem is rw.Unboxed, and for an element type that is an interface, obj_(new_X(...)) (a
// new object, never null) is the object itself: Go converts it to the interface as the cast
// icast_I(obj_(x)) did.
func unboxElem(cv rw.Conv, t types.Type, a ast.Expr) string {
	if call, ok := rw.Unparen(a).(*ast.CallExpr); ok && len(call.Args) == 1 {
		if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "obj_" {
			inner := rw.Unparen(call.Args[0])
			it, isIface := types.Unalias(t).Underlying().(*types.Interface)
			if n, c2 := rw.CallName(inner); isIface && !it.Empty() && c2 != nil && strings.HasPrefix(n, "new_") && types.AssignableTo(typeOf(inner), t) {
				return rw.Render(inner)
			}
		}
	}
	return rw.Unboxed(cv, t, a)
}

// isEnum reports whether t is a Java enum translated to an integer type (enum_, self()).
func isEnum(t types.Type) bool {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return false
	}
	if b, ok := n.Underlying().(*types.Basic); !ok || b.Info()&types.IsInteger == 0 {
		return false
	}
	return rw.HasMethod(n, "enum_") && rw.HasMethod(n, "self")
}
