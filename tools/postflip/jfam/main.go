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

// keyed makes the translator's typed sets and maps the post-flip ones.
func keyed(dry bool) {
	jset, jmap, jentry := lookup("JTSet"), lookup("JTMap"), lookup("TEntry")
	if jset == nil && jmap == nil {
		fmt.Fprintln(os.Stderr, "jfam -kind keyed: no JTSet or JTMap in the package; nothing to do")
		return
	}
	types_ := map[types.Object]string{jset: "TSet", jmap: "TMap", jentry: "TEntryP"}
	for _, t := range []string{"TSet", "TMap", "TEntryP"} {
		must(t)
	}
	type ctor struct {
		to    string
		keys  int // conversions compared as keys (the first of them)
		convs int
		extra string
	}
	ctors := map[types.Object]ctor{}
	for j, c := range map[string]ctor{
		"jnewTSet_": {"newTSet_", 1, 1, ""}, "newTSetN_": {"newTSetCap_", 1, 1, ""}, "jtsetFrom_": {"tsetFrom_", 1, 1, ", false"},
		"tsetCopy_": {"tsetCopyT_", 1, 1, ""}, "jnewTMap_": {"newTMap_", 1, 2, ""}, "newTMapN_": {"newTMapCap_", 1, 2, ""},
		"jtmapFrom_": {"tmapFromMap_", 1, 2, ""}, "tmapCopy_": {"tmapCopyT_", 1, 2, ""},
	} {
		must(c.to)
		if o := lookup(j); o != nil {
			ctors[o] = c
		}
	}
	methods := map[string]string{"size": "Len", "isEmpty": "IsEmpty", "clear": "Clear", "has": "Has", "add": "Add", "del": "Delete",
		"get": "Get", "getOr": "GetOr", "put": "Put", "putIfAbsent": "PutIfAbsent", "addAllT": "AddAll",
		"computeIfAbsent": "computeIfAbsent", "putAllT": "putAllT", "each": "each", "eachKey": "eachKey", "eachValue": "eachValue",
		"eachEntry": "eachEntry", "iterator": "iterator", "asSet": "asSet", "asMap": "asMap"}
	// recvOf is the JTSet or JTMap a method object belongs to, or nil.
	recvOf := func(o types.Object) types.Object {
		f, ok := o.(*types.Func)
		if !ok {
			return nil
		}
		sig := f.Origin().Type().(*types.Signature)
		if sig.Recv() == nil {
			return nil
		}
		t := sig.Recv().Type()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		if n, ok := t.(*types.Named); ok && (n.Origin().Obj() == jset || n.Origin().Obj() == jmap) && n.Origin().Obj() != nil {
			return n.Origin().Obj()
		}
		return nil
	}
	nType, nCtor, nNamed, nMade, nMeth := 0, 0, 0, 0, 0
	renamed := map[string]int{}
	bad := map[string]int{}
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			switch x := x.(type) {
			case *ast.Ident:
				o := rw.Info.Uses[x]
				if o == nil {
					return true
				}
				if nw, ok := types_[o]; ok {
					nType++
					rw.Rewrite[x] = func() string { return nw }
				}
			case *ast.SelectorExpr:
				o := rw.Info.Uses[x.Sel]
				if r := recvOf(o); r != nil {
					nw, ok := methods[x.Sel.Name]
					if !ok {
						bad[r.Name()+"."+x.Sel.Name]++
						return true
					}
					nMeth++
					if nw != x.Sel.Name {
						renamed[r.Name()+"."+x.Sel.Name+" "+nw]++
						sel := x.Sel
						rw.Rewrite[sel] = func() string { return nw }
					}
				}
			case *ast.CallExpr:
				fun := x.Fun
				if ix, ok := fun.(*ast.IndexExpr); ok {
					fun = ix.X
				} else if ix, ok := fun.(*ast.IndexListExpr); ok {
					fun = ix.X
				}
				id, ok := fun.(*ast.Ident)
				if !ok {
					return true
				}
				c, ok := ctors[rw.Info.Uses[id]]
				if !ok {
					return true
				}
				nCtor++
				rw.Rewrite[id] = func() string { return c.to }
				for i := 0; i < c.convs; i++ {
					convArg(x.Args[i], i >= c.keys, &nNamed, &nMade)
				}
				if c.extra != "" {
					last := x.Args[len(x.Args)-1]
					if _, taken := rw.Rewrite[last]; taken {
						bad["a constructor whose last argument is a conversion"]++
						return true
					}
					rw.Rewrite[last] = func() string { return rw.RenderChildren(last) + c.extra }
				}
			}
			return true
		})
	}
	fmt.Fprintf(os.Stderr, "jfam -kind keyed: %d mentions of JTSet, JTMap and TEntry are TSet, TMap and TEntryP; %d constructor calls (conversions: %d post-flip ones of the same name, %d with t_()); %d method calls\n", nType, nCtor, nNamed, nMade, nMeth)
	for _, k := range sortStrings(keysOf(renamed)) {
		fmt.Fprintf(os.Stderr, "  %d %s\n", renamed[k], k)
	}
	if len(bad) > 0 {
		for _, k := range sortStrings(keysOf(bad)) {
			fmt.Fprintf(os.Stderr, "no mapping: %d %s\n", bad[k], k)
		}
		fmt.Fprintln(os.Stderr, "nothing written")
		os.Exit(1)
	}
	if !dry {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
}

func keysOf(m map[string]int) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	return keys
}
