// Command icasts is a post-flip step for the typed translator's text: a cast to an interface
// type that is written with the generic cast_ outside generic code calls a named cast
// function, as the casts to the other interfaces do.
//
//	cast_[DiGraph_DiGraphNode[Node]](x)       ->  icast_DiGraph_DiGraphNode_Node(x)
//	elemsAs_[DiGraph_DiGraphEdge[Node]](c)    ->  elemsBy_(c, icast_DiGraph_DiGraphEdge_Node)
//
// j2go writes a cast to an interface X as icast_X (g_0casts.go), and a cast to an
// instantiated generic interface as cast_[X[A]] (rt_core.go). cast_ is a generic function:
// its type assertion finds the interface table through the dictionary and the runtime's
// table of tables at every call (runtime.assertE2I, getitab), where the assertion in a
// function that is not generic has the compiler's cache at the call site. icast_X_A is that
// function for the instantiation: `nil` gives nil, anything else is asserted to the interface,
// as cast_ does (the same ClassCastException from a failed assertion). The functions are
// appended to g_0casts.go, one per distinct type, named by the type's text with its
// punctuation as underscores.
//
// Left, counted: a cast whose type mentions a type parameter (the code is generic: there is
// no one type to name; the class's own code should work on the class, as the manual step for
// the directed graph does). A cast to a type that is not an interface (a class pointer, an
// enum, a slice) is a comparison of type words in either form and is not touched. A rerun
// changes nothing.
//
// usage: icasts [-n] <dir of package closure>
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

// funcName is the cast function of the type written as text.
func funcName(text string) string {
	var b strings.Builder
	b.WriteString("icast_")
	last := byte('_')
	for i := 0; i < len(text); i++ {
		c := text[i]
		ident := c == '_' || c >= '0' && c <= '9' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z'
		if !ident {
			c = '_'
		}
		if c == '_' && last == '_' {
			continue
		}
		b.WriteByte(c)
		last = c
	}
	return strings.TrimRight(b.String(), "_")
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
	}
	rw.Load(os.Args[len(os.Args)-1])
	done := map[string]int{}
	left := map[string]int{}
	need := map[string]string{} // function -> type text
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 {
				return true
			}
			ix, ok := call.Fun.(*ast.IndexExpr)
			if !ok {
				return true
			}
			id, ok := ix.X.(*ast.Ident)
			if !ok || (id.Name != "cast_" && id.Name != "elemsAs_") {
				return true
			}
			t := rw.Info.Types[ix.Index].Type
			if t == nil {
				return true
			}
			if _, isParam := types.Unalias(t).(*types.TypeParam); isParam {
				return true // T itself: the cast of a generic class to its parameter
			}
			if _, isIface := t.Underlying().(*types.Interface); !isIface {
				return true
			}
			if t.Underlying().(*types.Interface).NumMethods() == 0 {
				return true // any: nothing is asserted
			}
			if rw.ReflipGeneric_(t, false) {
				left[id.Name+": the interface type mentions a type parameter (generic code)"]++
				return true
			}
			text := rw.Text(ix.Index)
			name := funcName(text)
			if prev, has := need[name]; has && prev != text {
				left[id.Name+": two types with one function name"]++
				return true
			}
			need[name] = text
			if id.Name == "cast_" {
				done["cast_[I] -> icast_I"]++
				rw.Rewrite[ix] = func() string { return name }
			} else {
				done["elemsAs_[I](c) -> elemsBy_(c, icast_I)"]++
				rw.Rewrite[call] = func() string { return "elemsBy_(" + rw.Render(call.Args[0]) + ", " + name + ")" }
			}
			return true
		})
	}
	// The functions the package lacks go after the last declaration of g_0casts.go.
	var names []string
	for n := range need {
		if rw.Pkg.Scope().Lookup(n) == nil {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	if len(names) > 0 {
		var last ast.Decl
		for _, f := range rw.Files {
			if filepath.Base(rw.Fset.File(f.Pos()).Name()) == "g_0casts.go" {
				last = f.Decls[len(f.Decls)-1]
			}
		}
		if last == nil {
			fmt.Fprintln(os.Stderr, "icasts: no g_0casts.go")
			os.Exit(1)
		}
		var b strings.Builder
		for _, n := range names {
			fmt.Fprintf(&b, "\n\nfunc %s(v any) %s {\n\tif v == nil {\n\t\treturn nil\n\t}\n\treturn v.(%s)\n}", n, need[n], need[n])
		}
		rw.Rewrite[last] = func() string { return rw.RenderChildren(last) + b.String() }
	}
	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
	}
	total := 0
	keys = keys[:0]
	for k, n := range done {
		keys = append(keys, k)
		total += n
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", done[k], k)
	}
	if report {
		fmt.Fprintf(os.Stderr, "icasts: %d casts, %d new functions\n", total, len(names))
		return
	}
	fmt.Fprintf(os.Stderr, "icasts: %d casts, %d new functions; files changed: %d\n", total, len(names), rw.SaveChecked())
}
