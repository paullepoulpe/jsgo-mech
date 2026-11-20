// Command strkeys is a post-flip step: a String key passed to an erased Java map or set, or to
// a method that has a String variant, is passed unboxed.
//
// 1. A method m with a variant m_s, whose parameters are m's with some Object parameters typed
// string: x.m(..., strobj_(s), ...) is x.m_s(..., s, ...) when the arguments at all those
// positions are strobj_ boxes. Every such variant is m with those parameters boxed on entry
// (the convention of the _s methods of the generated classes, post-flip manual steps, and of
// rt_charseq.go), so the call computes the same without the boxes.
//
// 2. Erased collections:
// The runtime has, for the erased collections, helpers that take a String key unboxed
// (Map_get_s, Map_put_s, Map_containsKey_s, Map_remove_s, Map_getOrDefault_s,
// Map_putIfAbsent_s, Collection_contains_s, Collection_add_s, Collection_remove_s): on a hash
// map or set they look the string up without boxing it, on any other collection they call the
// method with strobj_(k), so each is the method called with the boxed key. j2go wrote them for
// the calls it saw with a String key; calls whose key became a string later (an Object slot
// typed by objslots, a generic class instantiated by generics) still box. Here
// m.get(strobj_(k)) on a receiver declared as an erased Map is Map_get_s(m, k), and so on;
// add and remove go to the set helpers only on a receiver declared as a Set (List.add of a
// string stores the box anyway, and a List's remove(Object) is remove_Object).
//
// usage: strkeys [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

// helper by receiver kind and method: the key is the method's first argument
var mapHelpers = map[string]string{"get": "Map_get_s", "put": "Map_put_s", "containsKey": "Map_containsKey_s",
	"remove": "Map_remove_s", "getOrDefault": "Map_getOrDefault_s", "putIfAbsent": "Map_putIfAbsent_s"}
var setHelpers = map[string]string{"contains": "Collection_contains_s", "add": "Collection_add_s", "remove": "Collection_remove_s"}
var collHelpers = map[string]string{"contains": "Collection_contains_s"}

var mapTypes = map[string]bool{"Map": true, "SortedMap": true, "NavigableMap": true, "LinkedHashMap": true, "HashMap": true, "TreeMap": true}
var setTypes = map[string]bool{"Set": true, "SortedSet": true, "NavigableSet": true, "LinkedHashSet": true, "HashSet": true, "TreeSet": true}
var collTypes = map[string]bool{"Collection": true, "List": true, "ArrayList": true, "Deque": true, "Queue": true, "ArrayDeque": true}

// strArgs returns the indices of the arguments of c that are strobj_ boxes, and the boxed strings.
func strArgs(c *ast.CallExpr) map[int]ast.Expr {
	m := map[int]ast.Expr{}
	for i, a := range c.Args {
		if a, ok := rw.Unparen(a).(*ast.CallExpr); ok && len(a.Args) == 1 {
			if id, ok := a.Fun.(*ast.Ident); ok && id.Name == "strobj_" {
				m[i] = a.Args[0]
			}
		}
	}
	return m
}

// variant rewrites x.m(args) to x.m_s(args) when the receiver has a method m_s whose
// parameters are m's with some Object parameters typed string, and the arguments at those
// positions are strobj_ boxes: by the convention of every _s method, m_s(s) is m(strobj_(s))
// without the box.
func variant(c *ast.CallExpr, sx *ast.SelectorExpr, sel *types.Selection, stats, cands map[string]int) bool {
	m, ok := sel.Obj().(*types.Func)
	if !ok || strings.HasSuffix(m.Name(), "_s") {
		return false
	}
	recv := sel.Recv()
	o, _, _ := types.LookupFieldOrMethod(recv, true, rw.Pkg, m.Name()+"_s")
	ms, _ := o.(*types.Func)
	sig := m.Type().(*types.Signature)
	sa := strArgs(c)
	if ms == nil {
		// report calls of generated methods that box a String into an Object parameter
		if fn := filepath.Base(rw.Fset.Position(m.Pos()).Filename); strings.HasPrefix(fn, "g_") && !sig.Variadic() {
			for i := range sa {
				if i < sig.Params().Len() && isAnyT(sig.Params().At(i).Type()) {
					cands[fmt.Sprintf("%s.%s#%d", recvName(recv), m.Name(), i)]++
				}
			}
		}
		return false
	}
	// Only variants that follow the convention: those of generated classes (post-flip manual
	// steps) and of rt_charseq.go. The runtime's other _s methods are fast paths for a key its
	// caller has checked is not null (ArrayList.contains_s, LinkedHashMap.get_s...).
	if fn := filepath.Base(rw.Fset.Position(ms.Pos()).Filename); !strings.HasPrefix(fn, "g_") && fn != "rt_charseq.go" {
		return false
	}
	ssig := ms.Type().(*types.Signature)
	if ssig.Params().Len() != sig.Params().Len() || ssig.Variadic() || sig.Variadic() || !types.Identical(ssig.Results(), sig.Results()) {
		return false
	}
	var pos []int
	for i := 0; i < sig.Params().Len(); i++ {
		p, q := sig.Params().At(i).Type(), ssig.Params().At(i).Type()
		if types.Identical(p, q) {
			continue
		}
		if !isAnyT(p) || !isStringT(q) || sa[i] == nil {
			return false
		}
		pos = append(pos, i)
	}
	if len(pos) == 0 {
		return false
	}
	nm := sx.Sel
	rw.Rewrite[nm] = func() string { return nm.Name + "_s" }
	for _, i := range pos {
		x := sa[i]
		rw.Rewrite[c.Args[i]] = func() string { return rw.Render(x) }
	}
	stats[recvName(recv)+"."+m.Name()+" -> "+m.Name()+"_s"]++
	return true
}

func recvName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := types.Unalias(t).(*types.Named); ok {
		return n.Obj().Name()
	}
	return t.String()
}

func isAnyT(t types.Type) bool {
	i, ok := types.Unalias(t).(*types.Interface)
	return ok && i.Empty()
}

func isStringT(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Kind() == types.String
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	stats := map[string]int{}
	cands := map[string]int{}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) == 0 {
				return true
			}
			sx, ok := rw.Unparen(c.Fun).(*ast.SelectorExpr)
			if !ok {
				return true
			}
			sel := rw.Info.Selections[sx]
			if sel == nil || sel.Kind() != types.MethodVal {
				return true
			}
			if variant(c, sx, sel, stats, cands) {
				return true
			}
			// the receiver's declared type: a runtime collection interface or class
			t := types.Unalias(rw.Info.Types[sx.X].Type)
			if p, ok := t.(*types.Pointer); ok {
				t = types.Unalias(p.Elem())
			}
			nt, ok := t.(*types.Named)
			if !ok || nt.Obj().Pkg() != rw.Pkg {
				return true
			}
			// (the interfaces are in g_zstubs.go, the classes in rt_*.go)
			if fn := filepath.Base(rw.Fset.Position(nt.Obj().Pos()).Filename); !strings.HasPrefix(fn, "rt_") && fn != "g_zstubs.go" {
				return true
			}
			name := nt.Obj().Name()
			var table map[string]string
			switch {
			case mapTypes[name]:
				table = mapHelpers
			case setTypes[name]:
				table = setHelpers
			case collTypes[name]:
				table = collHelpers
			default:
				return true
			}
			h, ok := table[sx.Sel.Name]
			if !ok {
				return true
			}
			a, ok := rw.Unparen(c.Args[0]).(*ast.CallExpr)
			if !ok || len(a.Args) != 1 {
				return true
			}
			if id, ok := a.Fun.(*ast.Ident); !ok || id.Name != "strobj_" {
				return true
			}
			// the helper's other parameters are the method's (any)
			hf, ok := rw.Pkg.Scope().Lookup(h).(*types.Func)
			if !ok || hf.Type().(*types.Signature).Params().Len() != len(c.Args)+1 {
				return true
			}
			recv, key, rest := sx.X, a.Args[0], c.Args[1:]
			rw.Rewrite[c] = func() string {
				s := h + "(" + rw.Render(recv) + ", " + rw.Render(key)
				for _, r := range rest {
					s += ", " + rw.Render(r)
				}
				return s + ")"
			}
			stats[name+"."+sx.Sel.Name+" -> "+h]++
			return true
		})
	}
	var keys []string
	total := 0
	for k, v := range stats {
		keys = append(keys, k)
		total += v
	}
	sort.Strings(keys)
	fmt.Printf("strkeys: %d calls pass the string key to a helper\n", total)
	for _, k := range keys {
		fmt.Printf("  %5d %s\n", stats[k], k)
	}
	if report {
		fmt.Println("  calls of generated methods boxing a String into an Object parameter, with no _s variant:")
		var cl []string
		for k := range cands {
			cl = append(cl, k)
		}
		sort.Slice(cl, func(i, j int) bool { return cands[cl[i]] > cands[cl[j]] || cands[cl[i]] == cands[cl[j]] && cl[i] < cl[j] })
		for i, k := range cl {
			if i < 40 {
				fmt.Printf("  %5d %s\n", cands[k], k)
			}
		}
		return
	}
	fmt.Printf("files changed: %d\n", rw.Save())
}
