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
// Not converted by -kind list, and counted: typed sets and maps (JTSet, JTMap), which compare
// keys through byEquals where the post-flip ones have hash and eq functions.
//
// usage: jfam [-n] -kind list|convs <dir of package closure>
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
	if *kind != "list" && *kind != "convs" || flag.NArg() != 1 {
		fmt.Fprintln(os.Stderr, "usage: jfam [-n] -kind list|convs <dir of package closure>")
		os.Exit(2)
	}
	rw.Load(flag.Arg(0))
	if *kind == "convs" {
		convs(*dry)
		return
	}
	jlist := lookup("JTList")
	if jlist == nil {
		fmt.Fprintln(os.Stderr, "jfam -kind list: no JTList in the package; nothing to do")
		return
	}
	must("TList")
	ctors := map[types.Object]string{}
	for j, t := range map[string]string{"jnewTList_": "newTList_", "jnewTListN_": "newTListN_", "jtlistFrom_": "tlistFrom_", "tlistCopy_": "tlistCopyT_"} {
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
	if !dry {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
}
