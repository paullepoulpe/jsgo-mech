// Command charseq is a post-flip step: a String passed to a CharSequence parameter of the
// runtime is passed as a string.
//
// Java's CharSequence is `any` in the runtime, so j2go boxes a String argument (strobj_) and the
// runtime takes it out again (csString_): an allocation per call for a string that is not a
// constant. rt_charseq.go has, for the runtime functions listed below, a variant F_s whose
// CharSequence parameters are strings and whose body is F's with csStr_ (csString_ of strobj_:
// a null String throws NullPointerException) in place of csString_. A call F(..., strobj_(s),
// ...) whose CharSequence arguments are all such boxes becomes F_s(..., s, ...): the same
// result, the same exception for null, no box.
//
// Left as they are: calls through an interface (Appendable.append_CharSequence: its
// implementers would all need the variant) and calls with an argument that is not strobj_ (a
// StringBuilder or another CharSequence object). Rerunning the step changes nothing.
//
// usage: charseq [-n] <dir of package closure>
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

// the runtime functions (receiver type name, or "" for a function) and the indices of their
// CharSequence parameters
var table = []struct {
	recv, name string
	params     []int
}{
	{"Pattern", "matcher", []int{0}},
	{"", "Pattern_matches", []int{1}},
	{"", "String_contains", []int{1}},
	{"", "String_replace", []int{1, 2}},
	{"", "String_join", []int{0}},
	{"", "String_join_CharSequence_CharSequenceArr", []int{0}},
	{"StringBuilder", "append_CharSequence", []int{0}},
	{"StringBuilder", "append_CharSequence_int_int", []int{0}},
	{"Splitter", "split", []int{0}},
	{"Splitter", "splitToList", []int{0}},
	{"CharMatcher", "matchesAllOf", []int{0}},
	{"CharMatcher", "trimFrom", []int{0}},
}

func runtimeFile(o types.Object) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(o.Pos()).Filename), "rt_")
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	stats := map[string]int{}
	left := map[string]int{}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			var nameId *ast.Ident
			recv := ""
			switch fn := rw.Unparen(c.Fun).(type) {
			case *ast.Ident:
				nameId = fn
			case *ast.SelectorExpr:
				nameId = fn.Sel
				if sel := rw.Info.Selections[fn]; sel != nil {
					t := sel.Recv()
					if p, ok := t.(*types.Pointer); ok {
						t = p.Elem()
					}
					if nt, ok := types.Unalias(t).(*types.Named); ok {
						recv = nt.Obj().Name()
						if _, isIface := nt.Underlying().(*types.Interface); isIface {
							recv = "interface " + recv
						}
					}
				}
			default:
				return true
			}
			o, ok := rw.Info.Uses[nameId].(*types.Func)
			if !ok {
				return true
			}
			for _, e := range table {
				if e.name != o.Name() {
					continue
				}
				key := e.name
				if e.recv != "" {
					key = e.recv + "." + e.name
				}
				if e.recv != recv {
					if strings.HasPrefix(recv, "interface ") {
						left[key+": through "+recv]++
					}
					continue
				}
				if !runtimeFile(o) {
					continue
				}
				var inner []ast.Expr
				for _, i := range e.params {
					if i >= len(c.Args) {
						break
					}
					a, ok := rw.Unparen(c.Args[i]).(*ast.CallExpr)
					if !ok || len(a.Args) != 1 {
						break
					}
					if id, ok := a.Fun.(*ast.Ident); !ok || id.Name != "strobj_" {
						break
					}
					inner = append(inner, a.Args[0])
				}
				if len(inner) != len(e.params) {
					left[key+": a CharSequence that is not a boxed String"]++
					continue
				}
				nm := nameId
				rw.Rewrite[nm] = func() string { return nm.Name + "_s" }
				for k, i := range e.params {
					x := inner[k]
					rw.Rewrite[c.Args[i]] = func() string { return rw.Render(x) }
				}
				stats[key]++
			}
			return true
		})
	}
	keys := func(m map[string]int) []string {
		var l []string
		for k := range m {
			l = append(l, k)
		}
		sort.Strings(l)
		return l
	}
	total := 0
	for _, v := range stats {
		total += v
	}
	fmt.Printf("charseq: %d calls pass a string\n", total)
	for _, k := range keys(stats) {
		fmt.Printf("  %5d %s\n", stats[k], k)
	}
	fmt.Println("  left:")
	for _, k := range keys(left) {
		fmt.Printf("  %5d %s\n", left[k], k)
	}
	if report {
		return
	}
	fmt.Printf("files changed: %d\n", rw.Save())
}
