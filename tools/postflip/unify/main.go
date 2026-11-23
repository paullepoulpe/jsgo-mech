// Command unify is a step of the re-flip: the typed collections that the translator writes
// (JTList, JTMap, JTSet with element conversions jtconv_; j2go rule "typed collections",
// renamed by tools/postflip/reflip/rename.sh) become the typed collections of the post-flip
// runtime (TList, TMap, TSet with tconv_; rt_tlist.go, rt_tmap.go), so that the package has one
// family and the later collection steps (colls, native, ranges, iters, tnames) see every typed
// collection.
//
// The two families are the same design (the translator's is derived from post-flip step 3):
// a list is a slice of T, a map and a set keep their entries in insertion order with removed
// positions marked dead, erased code sees each through its one view. What the generated code
// names is rewritten:
//
//	types      JTList TList, JTMap TMap, JTSet TSet, jtconv_ tconv_
//	makers     jnewTList_ newTList_, jnewTListN_ newTListN_, jnewTMap_ newTMap_, jnewTSet_
//	           newTSet_, jtlistFrom_ tlistFrom_; jtsetFrom_(c, x) tsetFrom_(c, x, false) and
//	           jtmapFrom_(k, v, x) tmapFrom_(k, v, x, false) (false: not an immutable copy)
//	methods    JTMap: size Len, isEmpty IsEmpty, get Get, has Has, getOr GetOr, put Put,
//	           putIfAbsent PutIfAbsent, del Delete, clear Clear; eachKey() and eachValue() are
//	           each(), whose cursor has the key in k and the value in v (a key cursor's .v is .k)
//	           JTSet: size Len, isEmpty IsEmpty, add Add, has Has, del Delete, clear Clear,
//	           addAllT AddAll
//	           (the Go names tools/postflip/tnames gave TMap and TSet; JTList's methods have
//	           TList's names already)
//	conversions  a variable jtc_X is tc_X, tce_X (elements compared by equals and hashCode)
//	           is tch_X: the post-flip names. Where the post-flip steps declared that variable
//	           already for the same element type, the translator's declaration goes and its
//	           uses name the existing one; else the declaration is renamed. `byEquals: true`
//	           becomes the hash and eq functions tconv_ has for such keys: Java's hashCode and
//	           equals of the boxed element, which is what the translator's family called.
//
// Everything else the translator's family offers keeps its name and is provided on the
// post-flip types by the runtime patch of the same step (newTMapN_, newTSetN_, tlistCopy_,
// tmapCopy_, tsetCopy_, putAllT, eachEntry and TEntry, iterator, TDeque): the step is this
// tool's run plus that patch, and the package type-checks only with both (so the tool writes
// with rw.Save, not rw.SaveChecked; the step's check is the build).
//
// Behaviour: the same operations on the same order of entries. A typed map or set of the
// translator compares keys with Go's == unless its conversion says byEquals; the post-flip
// TMap does the same unless its conversion has hash and eq.
//
// Rerunning the step changes nothing (no J-name is left). usage: unify [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"postflip/rw"
)

var objNames = map[string]string{
	"JTList": "TList", "JTMap": "TMap", "JTSet": "TSet", "jtconv_": "tconv_",
	"jnewTList_": "newTList_", "jnewTListN_": "newTListN_", "jnewTMap_": "newTMap_", "jnewTSet_": "newTSet_",
	"jtlistFrom_": "tlistFrom_", "jtsetFrom_": "tsetFrom_", "jtmapFrom_": "tmapFrom_",
}

// immArg are the makers that take one more argument in the post-flip family: imm, false.
var immArg = map[string]bool{"jtsetFrom_": true, "jtmapFrom_": true}

var methodNames = map[string]map[string]string{
	"JTMap": {"size": "Len", "isEmpty": "IsEmpty", "get": "Get", "has": "Has", "getOr": "GetOr", "put": "Put",
		"putIfAbsent": "PutIfAbsent", "del": "Delete", "clear": "Clear", "eachKey": "each", "eachValue": "each"},
	"JTSet": {"size": "Len", "isEmpty": "IsEmpty", "add": "Add", "has": "Has", "del": "Delete", "clear": "Clear",
		"addAllT": "AddAll"},
}

func named(t types.Type) *types.Named {
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}
	n, _ := types.Unalias(t).(*types.Named)
	return n
}

func recvName(f *types.Func) string {
	sig := f.Type().(*types.Signature)
	if sig.Recv() == nil {
		return ""
	}
	if n := named(sig.Recv().Type()); n != nil {
		return n.Obj().Name()
	}
	return ""
}

// own reports whether the file is the translator's runtime family itself (rt_jt*.go), which
// the step's runtime patch removes.
func own(n ast.Node) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "rt_jt")
}

var wordV = regexp.MustCompile(`\bv\b`)

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
	}
	rw.WithTests = true
	rw.Load(os.Args[len(os.Args)-1])
	counts := map[string]int{}
	scope := rw.Pkg.Scope()

	// Conversion variables: var X = &jtconv_[T]{...}.
	convTo := map[types.Object]string{}
	for _, file := range rw.Files {
		for _, d := range file.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || len(gd.Specs) != 1 {
				continue
			}
			vs, ok := gd.Specs[0].(*ast.ValueSpec)
			if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 {
				continue
			}
			o := rw.Info.Defs[vs.Names[0]]
			if o == nil {
				continue
			}
			nt := named(o.Type())
			if nt == nil || nt.Obj().Name() != "jtconv_" {
				continue
			}
			name := o.Name()
			to := name
			switch {
			case strings.HasPrefix(name, "jtc_"):
				to = name[1:]
			case strings.HasPrefix(name, "tce_"):
				to = "tch_" + name[4:]
			}
			lit := vs.Values[0].(*ast.UnaryExpr).X.(*ast.CompositeLit)
			var box *ast.FuncLit
			var byEq *ast.KeyValueExpr
			for _, el := range lit.Elts {
				kv := el.(*ast.KeyValueExpr)
				switch kv.Key.(*ast.Ident).Name {
				case "box":
					box = kv.Value.(*ast.FuncLit)
				case "byEquals":
					byEq = kv
				}
			}
			if to != name {
				if old := scope.Lookup(to); old != nil {
					ont := named(old.Type())
					if ont != nil && ont.Obj().Name() == "tconv_" && types.Identical(ont.TypeArgs().At(0), nt.TypeArgs().At(0)) && (byEq != nil) == strings.HasPrefix(to, "tch_") {
						convTo[o] = to
						counts["conversions that the post-flip steps declared already (the translator's declaration removed)"]++
						rw.Rewrite[gd] = func() string { return rw.DelDoc }
						continue
					}
					counts["left: conversions whose post-flip name is declared for another type (name kept)"]++
					to = name
				}
			}
			if to != name {
				convTo[o] = to
				counts["conversions renamed"]++
				id := vs.Names[0]
				rw.Rewrite[id] = func() string { return to }
			}
			if byEq != nil {
				// box is func(v T) any { return E }: hash and eq through E, as the family did.
				ret, ok := box.Body.List[0].(*ast.ReturnStmt)
				if !ok || len(box.Body.List) != 1 || box.Type.Params.List[0].Names[0].Name != "v" {
					panic("unify: unexpected box function of " + name)
				}
				e := rw.Text(ret.Results[0])
				t := rw.Text(box.Type.Params.List[0].Type)
				sub := func(n string) string { return wordV.ReplaceAllString(e, n) }
				counts["conversions compared by equals: byEquals becomes hash and eq"]++
				rw.Rewrite[byEq] = func() string {
					return fmt.Sprintf("hash: func(v %s) int32 { return objhash_(%s) },\n\teq: func(a, b %s) bool { return objequals_(%s, %s) }", t, e, t, sub("a"), sub("b"))
				}
			}
		}
	}

	for id, o := range rw.Info.Uses {
		if own(id) {
			continue
		}
		if to, ok := convTo[o]; ok {
			rw.Rewrite[id] = func() string { return to }
			continue
		}
		switch x := o.(type) {
		case *types.TypeName, *types.Func:
			if f, ok := x.(*types.Func); ok && recvName(f) != "" {
				if to, ok := methodNames[recvName(f.Origin())][f.Name()]; ok {
					counts[recvName(f.Origin())+"."+f.Name()+" -> "+to]++
					rw.Rewrite[id] = func() string { return to }
				}
				continue
			}
			if o.Pkg() != rw.Pkg || o.Parent() != scope {
				continue
			}
			if to, ok := objNames[o.Name()]; ok {
				counts[o.Name()+" -> "+to]++
				rw.Rewrite[id] = func() string { return to }
			}
		}
	}
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			if n == nil || own(n) {
				return n != nil
			}
			switch x := n.(type) {
			case *ast.SelectorExpr:
				// the element of a key cursor: .v of tmapKeys_ is .k of the map's cursor
				if x.Sel.Name == "v" {
					if nt := named(rw.Info.Types[x.X].Type); nt != nil && nt.Obj().Name() == "tmapKeys_" {
						counts["key cursor .v -> .k"]++
						rw.Rewrite[x.Sel] = func() string { return "k" }
					}
				}
			case *ast.CallExpr:
				fn := x.Fun
				if ix, ok := fn.(*ast.IndexExpr); ok {
					fn = ix.X
				}
				if ix, ok := fn.(*ast.IndexListExpr); ok {
					fn = ix.X
				}
				if id, ok := fn.(*ast.Ident); ok && immArg[id.Name] && rw.Info.Uses[id] == scope.Lookup(id.Name) {
					rw.Rewrite[x] = func() string { return rw.Render(x.Fun) + "(" + renderArgs(x) + ", false)" }
				}
			}
			return true
		})
	}

	keys := []string{}
	total := 0
	for k, n := range counts {
		keys = append(keys, k)
		total += n
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", counts[k], k)
	}
	if report {
		fmt.Fprintf(os.Stderr, "unify: %d rewrites\n", total)
		return
	}
	fmt.Fprintf(os.Stderr, "unify: %d rewrites; files changed: %d\n", total, rw.Save())
}

func renderArgs(c *ast.CallExpr) string {
	var parts []string
	for _, a := range c.Args {
		parts = append(parts, rw.Render(a))
	}
	return strings.Join(parts, ", ")
}
