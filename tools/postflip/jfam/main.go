// Command jfam is a post-flip step of a re-flip: the typed collections that the translator
// writes join the typed collections of the post-flip runtime.
//
// A typed j2go writes `List<X>` as its own typed list (after tools/postflip/reflip/rename.sh:
// *JTList[X], made by jnewTList_, jnewTListN_, jtlistFrom_ and tlistCopy_ with an element conversion
// jtc_X of type *jtconv_[X]; rt_jtlist.go). The post-flip runtime has the same list under the
// names the post-flip tools know (*TList[X], newTList_, newTListN_, tlistFrom_, tlistCopyT_, *tconv_[X];
// rt_tlist.go), and the tools that follow (native, ranges, iters, tnames...) only reach that
// one. The two were written from the same code: the same items slice, the same erased view
// (asList), the same cursor of a for-each loop (each), the same exceptions.
//
// -kind list: every *JTList[X] in the generated code becomes a *TList[X] and each of its four
// constructors the post-flip one. All of them at once: a list flows between declarations, and
// the two are different Go types. The element conversion handed to a constructor becomes
// tc_X where the package has that variable with the same element type (the conversions of one
// type box and unbox alike: a List<X> is boxed as javac's erasure has it), and otherwise
// c.t_(), the *tconv_[X] of a *jtconv_[X] (runtime: made once per conversion; the conversion
// of a type variable of a generic class is one of these). The package must type-check
// afterwards, or nothing is written. A rerun finds nothing.
//
// -kind convs: a conversion of the translator's family (a package variable of type
// *jtconv_[X]) whose name is tc_X is renamed jtc_X, at its declaration and every use.
// rename.sh gave the j prefix only to the names that collided with the post-flip family's at
// the time; tc_X is the name the post-flip tools give the *tconv_[X] of X and declare when the
// package lacks it, so a translator conversion of that name is in their way. Only names change.
//
// -kind keyed: every *JTSet[K] and *JTMap[K, V] becomes the post-flip *TSet[K] and *TMap[K, V]
// (rt_tmap.go: the same insertion-ordered set and map), again all at once:
//   - constructors: jnewTSet_ newTSet_, newTSetN_ newTSetCap_, jtsetFrom_(c, from)
//     tsetFrom_(c, from, false), tsetCopy_ tsetCopyT_, jnewTMap_ newTMap_, newTMapN_
//     newTMapCap_, jtmapFrom_ tmapFromMap_, tmapCopy_ tmapCopyT_;
//   - methods, by the names the post-flip tools gave the typed set and map (tnames): size Len,
//     isEmpty IsEmpty, clear Clear, has Has, add Add, del Delete, get Get, getOr GetOr, put
//     Put, putIfAbsent PutIfAbsent, addAllT AddAll; computeIfAbsent, putAllT, each, eachKey,
//     eachValue, eachEntry, iterator, asSet and asMap keep their names (rt_jbridge.go has the
//     ones the post-flip set and map lacked, the translator's code on their representation);
//   - *TEntry[K, V], the entry of a for-each loop over a typed map's entries, is *TEntryP[K, V].
//
// A conversion handed to a constructor is c.t_() for the elements of a set and the keys of a
// map: a conversion that compares by equals and hashCode (byEquals) gives the post-flip one
// its hash and eq functions there, the comparison through boxes that the translator's set made.
// The values of a map take tc_X where the package has it, as lists do. A method the mapping
// does not list stops the tool.
//
// On a flip whose translator types Guava's immutable lists (a *JTList[X] that never changes), -kind
// list maps their constructors and builder as well: jtlistEmptyImm_ tlistEmptyImm_,
// jimmutableListOf_ immutableListOf_, jimmutableListCopyOf_ immutableListCopyOf_ (and
// ...CopyOfArr_), jtlistImm_ tlistImm_, *TListBuilder[X] *ImmutableList_Builder[X] (add, addAll,
// build: the same names), and two the post-flip runtime lacked (rt_jbridge.go):
// immutableListCopyOfT_ immutableListCopyOfTP_ and splitToListT_ splitToListTP_ (a list that
// cannot be changed and is no ImmutableList: TList.unmod). After -kind convs every named
// conversion jtc_X has its post-flip one, tc_X.
//
// Not converted by -kind list, and counted: typed sets and maps (JTSet, JTMap), which compare
// keys through byEquals where the post-flip ones have hash and eq functions.
//
// usage: jfam [-n] -kind list|convs|keyed <dir of package closure>
package main

import (
	"flag"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"postflip/rw"
)

func generated(p token.Pos) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(p).Filename), "g_")
}

func lookup(name string) types.Object {
	o := rw.Pkg.Scope().Lookup(name)
	return o
}

func must(name string) types.Object {
	o := lookup(name)
	if o == nil {
		fmt.Fprintf(os.Stderr, "the package does not declare %s\n", name)
		os.Exit(1)
	}
	return o
}

// elem is the type argument of a *tconv_[X] or *jtconv_[X].
func elem(t types.Type) types.Type {
	if p, ok := t.(*types.Pointer); ok {
		if n, ok := p.Elem().(*types.Named); ok && n.TypeArgs().Len() == 1 {
			return n.TypeArgs().At(0)
		}
	}
	return nil
}

func main() {
	dry := flag.Bool("n", false, "report only")
	kind := flag.String("kind", "", "list")
	flag.Parse()
	if *kind != "list" && *kind != "convs" && *kind != "keyed" || flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: jfam [-n] -kind list|convs|keyed <dir of package closure>")
		os.Exit(2)
	}
	rw.Load(flag.Arg(0))
	if *kind == "convs" {
		convs(*dry)
		return
	}
	if *kind == "keyed" {
		keyed(*dry)
		return
	}
	jlist := lookup("JTList")
	if jlist == nil {
		fmt.Fprintln(os.Stderr, "jfam -kind list: no JTList in the package; nothing to do")
		return
	}
	must("TList")
	jbuilder := lookup("TListBuilder")
	if jbuilder != nil {
		must("ImmutableList_Builder")
	}
	ctors := map[types.Object]string{}
	for j, t := range map[string]string{"jnewTList_": "newTList_", "jnewTListN_": "newTListN_", "jtlistFrom_": "tlistFrom_", "tlistCopy_": "tlistCopyT_",
		// the typed immutable lists and their builder (the second trial's translator)
		"jtlistEmptyImm_": "tlistEmptyImm_", "jimmutableListOf_": "immutableListOf_", "jimmutableListCopyOf_": "immutableListCopyOf_",
		"jimmutableListCopyOfArr_": "immutableListCopyOfArr_", "immutableListCopyOfT_": "immutableListCopyOfTP_", "jtlistImm_": "tlistImm_",
		"splitToListT_": "splitToListTP_", "newTListBuilder_": "immutableListBuilder_", "newTListBuilderN_": "immutableListBuilderN_"} {
		must(t)
		if o := lookup(j); o != nil {
			ctors[o] = t
		}
	}
	nType, nCtor, nNamed, nMade := 0, 0, 0, 0
	left := map[string]int{}
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			switch x := x.(type) {
			case *ast.Ident:
				switch o := rw.Info.Uses[x]; {
				case o == jlist:
					nType++
					rw.Rewrite[x] = func() string { return "TList" }
				case o != nil && o == jbuilder:
					nType++
					rw.Rewrite[x] = func() string { return "ImmutableList_Builder" }
				case o != nil && o.Name() == "TDeque" && o.Parent() == rw.Pkg.Scope():
					left["TDeque: a typed deque is its own type on the translator's list (its own kind)"]++
				case o != nil && (o.Name() == "JTSet" || o.Name() == "JTMap") && o.Parent() == rw.Pkg.Scope():
					left[o.Name()+": keys compared through byEquals (not this kind)"]++
				}
			case *ast.CallExpr:
				fun := x.Fun
				if ix, ok := fun.(*ast.IndexExpr); ok { // an explicit instantiation
					fun = ix.X
				}
				id, ok := fun.(*ast.Ident)
				if !ok {
					return true
				}
				target, ok := ctors[rw.Info.Uses[id]]
				if !ok {
					return true
				}
				nCtor++
				rw.Rewrite[id] = func() string { return target }
				conv := x.Args[0]
				if cid, ok := rw.Unparen(conv).(*ast.Ident); ok && strings.HasPrefix(cid.Name, "jtc_") {
					if tv, ok := lookup(cid.Name[1:]).(*types.Var); ok {
						if a, b := elem(tv.Type()), elem(rw.Info.TypeOf(conv)); a != nil && b != nil && types.Identical(a, b) {
							nNamed++
							name := cid.Name[1:]
							rw.Rewrite[conv] = func() string { return name }
							return true
						}
					}
				}
				nMade++
				rw.Rewrite[conv] = func() string { return rw.RenderChildren(conv) + ".t_()" }
			}
			return true
		})
	}
	fmt.Fprintf(os.Stderr, "jfam -kind list: %d mentions of JTList are TList; %d constructor calls (%d with the post-flip conversion of the same name, %d with t_())\n", nType, nCtor, nNamed, nMade)
	keys := make([]string, 0, len(left))
	for k := range left {
		keys = append(keys, k)
	}
	for _, k := range sortStrings(keys) {
		fmt.Fprintf(os.Stderr, "left: %d %s\n", left[k], k)
	}
	if !*dry {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
}

func sortStrings(s []string) []string {
	for i := 1; i < len(s); i++ {
		for j := i; j > 0 && s[j] < s[j-1]; j-- {
			s[j], s[j-1] = s[j-1], s[j]
		}
	}
	return s
}

// convs renames the translator conversions named tc_X to jtc_X.
func convs(dry bool) {
	jt, _ := must("jtconv_").(*types.TypeName)
	objs := map[types.Object]string{}
	for _, name := range rw.Pkg.Scope().Names() {
		v, ok := rw.Pkg.Scope().Lookup(name).(*types.Var)
		if !ok || !strings.HasPrefix(name, "tc_") {
			continue
		}
		p, ok := v.Type().(*types.Pointer)
		if !ok {
			continue
		}
		if n, ok := p.Elem().(*types.Named); !ok || n.Origin().Obj() != jt {
			continue
		}
		if !generated(v.Pos()) {
			fmt.Fprintf(os.Stderr, "left: %s is declared by the handwritten runtime\n", name)
			continue
		}
		if lookup("j"+name) != nil {
			fmt.Fprintf(os.Stderr, "left: %s: the package already declares j%s\n", name, name)
			continue
		}
		objs[v] = "j" + name
	}
	uses := 0
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			if id, ok := x.(*ast.Ident); ok {
				o := rw.Info.Uses[id]
				if o == nil {
					o = rw.Info.Defs[id]
				}
				if nw, ok := objs[o]; ok {
					uses++
					rw.Rewrite[id] = func() string { return nw }
				}
			}
			return true
		})
	}
	fmt.Fprintf(os.Stderr, "jfam -kind convs: %d translator conversions named tc_X are jtc_X, at %d identifiers\n", len(objs), uses)
	// Every named translator conversion jtc_X gets its post-flip conversion tc_X: the package's
	// own when it has one of the same element type (then jtc_X.t_() is that one), else
	// tc_X = jtc_X.t_(). One *tconv_[X] per X: the typed immutables keep one empty collection
	// and one typed copy per conversion.
	const gen = "g_zconv_jfam.go"
	renamedTo := map[string]types.Type{} // jtc_X -> its type, for all named translator conversions
	for _, name := range rw.Pkg.Scope().Names() {
		v, ok := rw.Pkg.Scope().Lookup(name).(*types.Var)
		if !ok {
			continue
		}
		p, ok := v.Type().(*types.Pointer)
		if !ok {
			continue
		}
		if n, ok := p.Elem().(*types.Named); !ok || n.Origin().Obj() != jt {
			continue
		}
		if nw, ok := objs[v]; ok {
			renamedTo[nw] = v.Type()
		} else if strings.HasPrefix(name, "jtc_") {
			renamedTo[name] = v.Type()
		}
	}
	var pairs, decls, clash []string
	for _, jn := range sortStrings(keysOfTypes(renamedTo)) {
		tn := jn[1:]
		tv, _ := lookup(tn).(*types.Var)
		if tv != nil && objs[tv] != "" {
			tv = nil // the translator conversion that is being renamed away from this name
		}
		switch {
		case tv == nil || filepath.Base(rw.Fset.Position(tv.Pos()).Filename) == gen:
			decls = append(decls, fmt.Sprintf("var %s = %s.t_()", tn, jn))
		case elem(tv.Type()) != nil && types.Identical(elem(tv.Type()), elem(renamedTo[jn])) && !strings.Contains(tv.Type().String(), "jtconv_"):
			pairs = append(pairs, fmt.Sprintf("\t%s.post = %s", jn, tn))
		default:
			clash = append(clash, tn)
		}
	}
	src := "package closure\n\n// Generated by tools/postflip/jfam -kind convs: the post-flip conversion of each named translator\n// conversion (jtc_X.t_() is tc_X).\n\n"
	if len(pairs) > 0 {
		src += "func init() {\n" + strings.Join(pairs, "\n") + "\n}\n\n"
	}
	src += strings.Join(decls, "\n") + "\n"
	fmt.Fprintf(os.Stderr, "jfam -kind convs: %d named translator conversions; %d share the package's tc_X, %d get tc_X = jtc_X.t_()\n", len(renamedTo), len(pairs), len(decls))
	if len(clash) > 0 {
		fmt.Fprintf(os.Stderr, "left: %d tc_X of another element type (their lists take jtc_X.t_()): %s\n", len(clash), strings.Join(clash, " "))
	}
	if !dry {
		n := rw.SaveChecked()
		path := filepath.Join(filepath.Dir(rw.Fset.Position(rw.Files[0].Pos()).Filename), gen)
		if old, err := os.ReadFile(path); err != nil || string(old) != src {
			if err := os.WriteFile(path, []byte(src), 0o644); err != nil {
				fmt.Fprintln(os.Stderr, err)
				os.Exit(1)
			}
			n++
		}
		fmt.Fprintf(os.Stderr, "files changed: %d\n", n)
	}
}

func keysOfTypes(m map[string]types.Type) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}

// convArg is the rewrite of a conversion argument: tc_X when named (and the package has it for
// the same type), else c.t_().
func convArg(conv ast.Expr, named bool, nNamed, nMade *int) {
	if cid, ok := rw.Unparen(conv).(*ast.Ident); named && ok && strings.HasPrefix(cid.Name, "jtc_") {
		if tv, ok := lookup(cid.Name[1:]).(*types.Var); ok {
			if a, b := elem(tv.Type()), elem(rw.Info.TypeOf(conv)); a != nil && b != nil && types.Identical(a, b) {
				*nNamed++
				name := cid.Name[1:]
				rw.Rewrite[conv] = func() string { return name }
				return
			}
		}
	}
	*nMade++
	rw.Rewrite[conv] = func() string { return rw.RenderChildren(conv) + ".t_()" }
}

// keyed makes the translator's typed sets and maps the post-flip ones, declaration group by
// declaration group: a group is the declarations (variables, fields, parameters, results,
// builders, cursors, entries) that a value can flow between; a group that a sorted set or map
// can reach (newTSortedSet_ ..., which the translator keeps in the same Go type), or that meets
// code the tool does not follow, stays as it is and is counted.
func keyed(dry bool) {
	jset, jmap := lookup("JTSet"), lookup("JTMap")
	if jset == nil && jmap == nil {
		fmt.Fprintln(os.Stderr, "jfam -kind keyed: no JTSet or JTMap in the package; nothing to do")
		return
	}
	// The types of the kind and their post-flip names ("" for cursors: their type is never written).
	typeNames := map[string]string{"JTSet": "TSet", "JTMap": "TMap", "TEntry": "TEntryP", "TSetBuilder": "ImmutableSet_Builder",
		"TMapBuilder": "ImmutableMap_Builder", "tmapEntries_": "", "tmapKeys_": "", "tmapVals_": "", "jtsetEach_": ""}
	jtypes := map[types.Object]string{}
	for j, t := range typeNames {
		if o := lookup(j); o != nil {
			jtypes[o] = t
			if t != "" {
				must(t)
			}
		}
	}
	type ctor struct {
		to     string
		keys   int // conversions compared as keys (the first of them)
		convs  int
		extra  string
		sorted bool
	}
	ctors := map[types.Object]ctor{}
	for j, c := range map[string]ctor{
		"jnewTSet_": {to: "newTSet_", keys: 1, convs: 1}, "newTSetN_": {to: "newTSetCap_", keys: 1, convs: 1},
		"jtsetFrom_": {to: "tsetFrom_", keys: 1, convs: 1, extra: ", false"}, "tsetCopy_": {to: "tsetCopyT_", keys: 1, convs: 1},
		"jnewTMap_": {to: "newTMap_", keys: 1, convs: 2}, "newTMapN_": {to: "newTMapCap_", keys: 1, convs: 2},
		"jtmapFrom_": {to: "tmapFromMap_", keys: 1, convs: 2}, "tmapCopy_": {to: "tmapCopyT_", keys: 1, convs: 2},
		// typed immutables and their builders
		"jtsetEmptyImm_": {to: "tsetEmptyImm_", keys: 1, convs: 1}, "jimmutableSetOf_": {to: "immutableSetOf_", keys: 1, convs: 1},
		"jimmutableSetCopyOf_": {to: "immutableSetCopyOf_", keys: 1, convs: 1}, "immutableSetCopyOfT_": {to: "immutableSetCopyOfTP_", keys: 1, convs: 1},
		"tsetImm_": {to: "tsetFrom_", keys: 1, convs: 1, extra: ", true"}, "newTSetBuilder_": {to: "immutableSetBuilder_", keys: 1, convs: 1},
		"newTSetBuilderN_": {to: "immutableSetBuilderN_", keys: 1, convs: 1},
		"jtmapEmptyImm_":   {to: "tmapEmptyImm_", keys: 1, convs: 2}, "jimmutableMapCopyOf_": {to: "immutableMapCopyOf_", keys: 1, convs: 2},
		"immutableMapCopyOfT_": {to: "immutableMapCopyOfTP_", keys: 1, convs: 2}, "tmapImm_": {to: "tmapFrom_", keys: 1, convs: 2, extra: ", true"},
		"immutableMapOf1_": {to: "immutableMapOf1P_", keys: 1, convs: 2}, "immutableMapOf2_": {to: "immutableMapOf2P_", keys: 1, convs: 2},
		"immutableMapOf3_": {to: "immutableMapOf3P_", keys: 1, convs: 2}, "immutableMapOf4_": {to: "immutableMapOf4P_", keys: 1, convs: 2},
		"immutableMapOf5_": {to: "immutableMapOf5P_", keys: 1, convs: 2}, "newTMapBuilder_": {to: "immutableMapBuilder_", keys: 1, convs: 2},
		// sorted sets and maps: the same Go type in the translator's family, another in the post-flip one
		"newTSortedSet_": {sorted: true}, "newTSortedSetC_": {sorted: true}, "tsortedSetFrom_": {sorted: true}, "tsortedSetCopy_": {sorted: true},
		"newTSortedMap_": {sorted: true}, "newTSortedMapC_": {sorted: true}, "tsortedMapFrom_": {sorted: true}, "tsortedMapCopy_": {sorted: true},
	} {
		if o := lookup(j); o != nil {
			if !c.sorted {
				must(c.to)
			}
			ctors[o] = c
		}
	}
	// The methods of the kind's types that generated code may call, and their post-flip names.
	methods := map[string]string{"size": "Len", "isEmpty": "IsEmpty", "clear": "Clear", "has": "Has", "contains": "Has", "containsKey": "Has",
		"add": "Add", "del": "Delete", "remove": "Delete", "get": "Get", "getOr": "GetOr", "getOrDefault": "GetOr", "put": "Put",
		"putIfAbsent": "PutIfAbsent", "addAllT": "AddAll", "computeIfAbsent": "computeIfAbsent", "putAllT": "putAllT", "each": "each",
		"eachKey": "eachKey", "eachValue": "eachValue", "eachEntry": "eachEntry", "iterator": "iterator", "asSet": "asSet", "asMap": "asMap",
		"asImm": "asImm", "next": "next", "entry": "entry", "getKey": "getKey", "getValue": "getValue", "setValue": "setValue", "asEntry": "asEntry"}
	builderMethods := map[string]bool{"add": true, "addAll": true, "build": true, "put": true, "putAll": true, "buildOrThrow": true, "buildKeepingLast": true}

	// jkind: the kind's type of t (its Go name), or "".
	jkind := func(t types.Type) string {
		if t == nil {
			return ""
		}
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		if n, ok := t.(*types.Named); ok {
			if _, ok := jtypes[n.Origin().Obj()]; ok {
				return n.Origin().Obj().Name()
			}
		}
		return ""
	}
	// Union-find over the declaration groups.
	parent := map[string]string{}
	var find func(k string) string
	find = func(k string) string {
		p, ok := parent[k]
		if !ok {
			parent[k] = k
			return k
		}
		if p == k {
			return k
		}
		r := find(p)
		parent[k] = r
		return r
	}
	why := map[string]string{} // root -> why its group stays
	poison := func(k, reason string) {
		if k == "" {
			return
		}
		r := find(k)
		if why[r] == "" {
			why[r] = reason
		}
	}
	union := func(a, b string) {
		if a == "" || b == "" {
			return
		}
		ra, rb := find(a), find(b)
		if ra == rb {
			return
		}
		parent[ra] = rb
		if why[rb] == "" {
			why[rb] = why[ra]
		}
	}
	varKey := func(v *types.Var) string { return fmt.Sprintf("v%p", v.Origin()) }
	fnKey := func(f *types.Func, what string, i int) string {
		f = f.Origin()
		sig := f.Type().(*types.Signature)
		if sig.Recv() != nil { // methods of one name and arity are one group (overriding, interfaces)
			return fmt.Sprintf("%s:%s/%d:%d", what, f.Name(), sig.Params().Len(), i)
		}
		return fmt.Sprintf("%s%p:%d", what, f, i)
	}
	calleeOf := func(c *ast.CallExpr) *types.Func {
		fun := rw.Unparen(c.Fun)
		if ix, ok := fun.(*ast.IndexExpr); ok {
			fun = ix.X
		} else if ix, ok := fun.(*ast.IndexListExpr); ok {
			fun = ix.X
		}
		switch x := fun.(type) {
		case *ast.Ident:
			f, _ := rw.Info.Uses[x].(*types.Func)
			return f
		case *ast.SelectorExpr:
			f, _ := rw.Info.Uses[x.Sel].(*types.Func)
			return f
		}
		return nil
	}
	unknown := 0
	fresh := func(reason string) string {
		unknown++
		k := fmt.Sprintf("?%d", unknown)
		poison(k, reason)
		return k
	}
	// slot is the group key of an expression of the kind ("" for nil).
	var slot func(e ast.Expr) string
	slot = func(e ast.Expr) string {
		switch x := rw.Unparen(e).(type) {
		case *ast.Ident:
			if x.Name == "nil" {
				return ""
			}
			if v, ok := rw.Info.Uses[x].(*types.Var); ok {
				return varKey(v)
			}
			if v, ok := rw.Info.Defs[x].(*types.Var); ok {
				return varKey(v)
			}
		case *ast.SelectorExpr:
			if v, ok := rw.Info.Uses[x.Sel].(*types.Var); ok {
				if !generated(v.Pos()) { // a field of a runtime type (a cursor's element): the group of its holder
					if k := jkind(rw.Info.TypeOf(x.X)); k != "" {
						return slot(x.X)
					}
					return fresh("a field of a runtime type")
				}
				return varKey(v)
			}
		case *ast.CallExpr:
			f := calleeOf(x)
			if f == nil {
				return fresh("the result of a function value or a conversion")
			}
			if c, ok := ctors[f.Origin()]; ok {
				k := fmt.Sprintf("c%d", x.Pos())
				if c.sorted {
					poison(k, "a sorted set or map (the translator's one type for both; the post-flip family has another)")
				}
				return k
			}
			if sel, ok := rw.Unparen(x.Fun).(*ast.SelectorExpr); ok && jkind(rw.Info.TypeOf(sel.X)) != "" && !generated(f.Pos()) {
				// a method of the kind's types (build of a builder, entry of a cursor...): the group of its receiver
				k := slot(sel.X)
				if _, ok := methods[f.Name()]; !ok && !builderMethods[f.Name()] {
					poison(k, "method "+f.Name()+" of the translator's family has no post-flip counterpart")
				}
				return k
			}
			if !generated(f.Pos()) {
				return fresh("the result of the runtime's " + f.Name())
			}
			return fnKey(f, "r", 0)
		}
		return fresh("an expression the tool does not follow")
	}
	flow := func(a, b ast.Expr) {
		if jkind(rw.Info.TypeOf(a)) == "" && jkind(rw.Info.TypeOf(b)) == "" {
			return
		}
		union(slot(a), slot(b))
	}
	// declKey: the group of each type expression that declares something of the kind.
	declKey := map[ast.Expr]string{}
	fieldsOf := func(fl *ast.FieldList, key func(i int, name *ast.Ident) string) {
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
				var name *ast.Ident
				if len(f.Names) > 0 {
					name = f.Names[j]
				}
				k := key(i, name)
				if prev, ok := declKey[f.Type]; ok {
					union(prev, k)
				}
				declKey[f.Type] = k
				i++
			}
		}
	}
	type frame struct {
		fn  *types.Func
		lit *ast.FuncLit
	}
	var visit func(n ast.Node, fr frame)
	visitChildren := func(n ast.Node, fr frame) {
		ast.Inspect(n, func(c ast.Node) bool {
			if c == n || c == nil {
				return true
			}
			visit(c, fr)
			return false
		})
	}
	visit = func(n ast.Node, fr frame) {
		switch x := n.(type) {
		case *ast.FuncDecl:
			f, _ := rw.Info.Defs[x.Name].(*types.Func)
			if f == nil {
				return
			}
			pv := func(what string) func(i int, name *ast.Ident) string {
				return func(i int, name *ast.Ident) string {
					k := fnKey(f, what, i)
					if name != nil {
						if v, ok := rw.Info.Defs[name].(*types.Var); ok {
							union(varKey(v), k)
						}
					}
					return k
				}
			}
			fieldsOf(x.Type.Params, pv("p"))
			fieldsOf(x.Type.Results, pv("r"))
			if x.Body != nil {
				visitChildren(x.Body, frame{fn: f})
			}
			return
		case *ast.FuncLit:
			bad := func(i int, name *ast.Ident) string {
				k := fresh("a parameter or result of a function literal")
				if name != nil {
					if v, ok := rw.Info.Defs[name].(*types.Var); ok {
						union(varKey(v), k)
					}
				}
				return k
			}
			fieldsOf(x.Type.Params, bad)
			fieldsOf(x.Type.Results, bad)
			visitChildren(x.Body, frame{lit: x})
			return
		case *ast.StructType:
			fieldsOf(x.Fields, func(i int, name *ast.Ident) string {
				if name != nil {
					if v, ok := rw.Info.Defs[name].(*types.Var); ok {
						return varKey(v)
					}
				}
				return fresh("an embedded field")
			})
		case *ast.InterfaceType:
			for _, m := range x.Methods.List {
				ft, ok := m.Type.(*ast.FuncType)
				if !ok || len(m.Names) != 1 {
					continue
				}
				f, _ := rw.Info.Defs[m.Names[0]].(*types.Func)
				if f == nil {
					continue
				}
				fieldsOf(ft.Params, func(i int, _ *ast.Ident) string { return fnKey(f, "p", i) })
				fieldsOf(ft.Results, func(i int, _ *ast.Ident) string { return fnKey(f, "r", i) })
			}
			return
		case *ast.FuncType: // a function type that is no declaration's: its values are not followed
			bad := func(i int, _ *ast.Ident) string { return fresh("a parameter or result of a function type") }
			fieldsOf(x.Params, bad)
			fieldsOf(x.Results, bad)
			return
		case *ast.ValueSpec:
			for i, name := range x.Names {
				v, _ := rw.Info.Defs[name].(*types.Var)
				if v == nil {
					continue
				}
				if x.Type != nil {
					if prev, ok := declKey[x.Type]; ok {
						union(prev, varKey(v))
					}
					declKey[x.Type] = varKey(v)
				}
				if len(x.Values) == len(x.Names) {
					flow(name, x.Values[i])
				} else if len(x.Values) > 0 && jkind(v.Type()) != "" {
					poison(varKey(v), "one of several results of a call")
				}
			}
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i := range x.Lhs {
					flow(x.Lhs[i], x.Rhs[i])
				}
			} else {
				for _, l := range x.Lhs {
					if jkind(rw.Info.TypeOf(l)) != "" {
						poison(slot(l), "one of several results of a call")
					}
				}
			}
		case *ast.ReturnStmt:
			for i, r := range x.Results {
				if jkind(rw.Info.TypeOf(r)) == "" {
					continue
				}
				switch {
				case fr.lit != nil:
					poison(slot(r), "returned by a function literal")
				case fr.fn != nil && len(x.Results) == fr.fn.Type().(*types.Signature).Results().Len():
					union(slot(r), fnKey(fr.fn, "r", i))
				default:
					poison(slot(r), "returned as one of several results")
				}
			}
		case *ast.RangeStmt:
			for _, e := range []ast.Expr{x.Key, x.Value} {
				if e != nil && jkind(rw.Info.TypeOf(e)) != "" {
					poison(slot(e), "an element of a Go slice or map")
				}
			}
		case *ast.CompositeLit:
			for _, el := range x.Elts {
				if kv, ok := el.(*ast.KeyValueExpr); ok {
					if id, ok := kv.Key.(*ast.Ident); ok {
						if v, ok := rw.Info.Uses[id].(*types.Var); ok && v.IsField() {
							if jkind(v.Type()) != "" || jkind(rw.Info.TypeOf(kv.Value)) != "" {
								union(varKey(v), slot(kv.Value))
							}
							continue
						}
					}
					el = kv.Value
				}
				if jkind(rw.Info.TypeOf(el)) != "" {
					poison(slot(el), "an element of a composite literal")
				}
			}
		case *ast.CallExpr:
			f := calleeOf(x)
			var sig *types.Signature
			if f != nil {
				sig = f.Origin().Type().(*types.Signature)
			}
			_, isCtor := ctors[originOf(f)]
			var recv ast.Expr
			if sel, ok := rw.Unparen(x.Fun).(*ast.SelectorExpr); ok && f != nil && sig.Recv() != nil {
				recv = sel.X
			}
			for i, a := range x.Args {
				if jkind(rw.Info.TypeOf(a)) == "" {
					continue
				}
				switch {
				case f == nil:
					poison(slot(a), "an argument of a function value or a conversion")
				case isCtor: // a copy constructor: the copy and its source convert together
					union(slot(a), slot(x))
				case !generated(f.Pos()) && recv != nil && jkind(rw.Info.TypeOf(recv)) != "":
					union(slot(a), slot(recv)) // addAllT, putAllT: receiver and argument together
				case !generated(f.Pos()):
					poison(slot(a), "an argument of the runtime's "+f.Name())
				case sig.Variadic() && i >= sig.Params().Len()-1:
					poison(slot(a), "a variadic argument")
				default:
					union(slot(a), fnKey(f, "p", i))
				}
			}
			if recv != nil && !generated(f.Pos()) && jkind(rw.Info.TypeOf(recv)) != "" {
				if _, ok := methods[f.Name()]; !ok && !builderMethods[f.Name()] {
					poison(slot(recv), "method "+f.Name()+" of the translator's family has no post-flip counterpart")
				}
			}
		}
		visitChildren(n, fr)
	}
	for _, f := range rw.Files {
		if generated(f.Pos()) {
			visit(f, frame{})
		}
	}

	// The rewrites: every mention, constructor call and method call of a group that does not stay.
	stays := func(k string) bool { return k == "" || why[find(k)] != "" }
	nType, nCtor, nNamed, nMade, nMeth := 0, 0, 0, 0, 0
	left := map[string]int{}
	renamed := map[string]int{}
	// mentionKey: the group of a mention of a type of the kind, through the declaration it is the type of.
	var stack []ast.Node
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		stack = stack[:0]
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			stack = append(stack, n)
			switch x := n.(type) {
			case *ast.Ident:
				o := rw.Info.Uses[x]
				nw, ok := jtypes[o]
				if !ok || nw == "" {
					return true
				}
				key, found := "", false
				for i := len(stack) - 2; i >= 0 && !found; i-- {
					if e, ok := stack[i].(ast.Expr); ok {
						key, found = declKey[e]
					} else {
						break
					}
				}
				if !found {
					left[o.Name()+": a mention that is no declaration's type (an instantiation, a conversion)"]++
					return true
				}
				if stays(key) {
					left[o.Name()+": "+why[find(key)]]++
					return true
				}
				nType++
				rw.Rewrite[x] = func() string { return nw }
			case *ast.CallExpr:
				f := calleeOf(x)
				if f == nil {
					return true
				}
				if c, ok := ctors[f.Origin()]; ok {
					if c.sorted || stays(slot(x)) {
						return true
					}
					fun := rw.Unparen(x.Fun)
					if ix, ok := fun.(*ast.IndexExpr); ok {
						fun = ix.X
					} else if ix, ok := fun.(*ast.IndexListExpr); ok {
						fun = ix.X
					}
					id, ok := fun.(*ast.Ident)
					if !ok {
						return true
					}
					nCtor++
					rw.Rewrite[id] = func() string { return c.to }
					for i := 0; i < c.convs; i++ {
						convArg(x.Args[i], true, &nNamed, &nMade)
					}
					if c.extra != "" {
						last := x.Args[len(x.Args)-1]
						if _, taken := rw.Rewrite[last]; !taken {
							rw.Rewrite[last] = func() string { return rw.RenderChildren(last) + c.extra }
						}
					}
					return true
				}
				sel, ok := rw.Unparen(x.Fun).(*ast.SelectorExpr)
				if !ok || generated(f.Pos()) {
					return true
				}
				k := jkind(rw.Info.TypeOf(sel.X))
				if k == "" || stays(slot(sel.X)) {
					return true
				}
				nMeth++
				if nw, ok := methods[f.Name()]; ok && nw != f.Name() && (k == "JTSet" || k == "JTMap") {
					renamed[k+"."+f.Name()+" "+nw]++
					s := sel.Sel
					rw.Rewrite[s] = func() string { return nw }
				}
			}
			return true
		})
	}
	fmt.Fprintf(os.Stderr, "jfam -kind keyed: %d mentions of the translator's typed sets, maps, entries and builders are the post-flip ones; %d constructor calls (conversions: %d named tc_X, %d with t_()); %d method calls\n", nType, nCtor, nNamed, nMade, nMeth)
	for _, k := range sortStrings(keysOf(renamed)) {
		fmt.Fprintf(os.Stderr, "  %d %s\n", renamed[k], k)
	}
	for _, k := range sortStrings(keysOf(left)) {
		fmt.Fprintf(os.Stderr, "left: %d %s\n", left[k], k)
	}
	if !dry {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
}

func originOf(f *types.Func) types.Object {
	if f == nil {
		return nil
	}
	return f.Origin()
}

func keysOf(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
