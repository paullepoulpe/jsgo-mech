// Command gcasts is a post-flip step for the typed translator's text: a cast to an interface
// type that mentions a type parameter (in generic code) first tries, by a type switch, the
// classes of the code's own type parameters that implement the interface.
//
//	cast_[UndiGraph_UndiGraphNode[N, E]](x)
//	    ->  gcast_UndiGraph_UndiGraphNode_N_E[N, E](x)
//	elemsAs_[UndiGraph_UndiGraphEdge[N, E]](c)
//	    ->  elemsBy_(c, gcast_UndiGraph_UndiGraphEdge_N_E[N, E])
//
//	func gcast_UndiGraph_UndiGraphNode_N_E[N any, E any](v any) UndiGraph_UndiGraphNode[N, E] {
//		switch p := v.(type) {
//		case *LinkedUndirectedGraph_AnnotatedLinkedUndirectedGraphNode[N, E]:
//			return p
//		case *LinkedUndirectedGraph_LinkedUndirectedGraphNode[N, E]:
//			return p
//		}
//		return cast_[UndiGraph_UndiGraphNode[N, E]](v)
//	}
//
// In a generic function the assertion of cast_ to an interface whose type comes from the
// dictionary is a lookup in the runtime's table of interface tables at every call
// (runtime.assertE2I2 -> getitab: no cache per call site, which non-generic code has, and
// which is why step icasts could name a function for the casts outside generic code). A type
// switch case of a class type is a comparison of type words, and converting the class pointer
// to the interface takes the interface table from the dictionary. This is what the manual
// step 422 did by hand for the directed graph (to_LinkedDiGraphNode trying the two classes a
// graph makes); here it is a rule.
//
// The classes tried are the generated classes C whose type parameters are those of the code
// (the same names, in order: the receiver's of a method, a generic function's own) and for
// which *C instantiated with them implements the interface: then `return p` compiles, and it
// is the value v.(I) would give for that dynamic type. Every other value (nil included) goes to
// cast_ as before: the same result, the same ClassCastException. So Java's behaviour is kept for
// every value; the classes only decide which values take the shorter path.
//
// One function per interface type as written and type parameter list, appended to
// g_0casts.go. Left, counted: no class of the code's type parameters implements the
// interface (the code works on another class's nodes: DominatorTree, ControlFlowGraph[N], the
// generic algorithms over a graph of any class), or more than four do, or two functions would
// have one name. A rerun changes nothing (gcast_ calls are not casts).
//
// usage: gcasts [-n] [-v] <dir of package closure>
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

const maxArms = 4

// sanitize is the text with its punctuation as single underscores.
func sanitize(text string) string {
	var b strings.Builder
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

func qual(p *types.Package) string { return "" }

func main() {
	report, verbose := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])

	// The generated generic classes.
	var classes []*types.TypeName
	for _, name := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		nt, ok := tn.Type().(*types.Named)
		if !ok || nt.TypeParams().Len() == 0 {
			continue
		}
		if _, ok := nt.Underlying().(*types.Struct); !ok {
			continue
		}
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(tn.Pos()).Name()), "g_") {
			continue
		}
		classes = append(classes, tn)
	}

	done := map[string]int{}
	left := map[string]int{}
	leftAt := map[string][]string{}
	type helper struct{ text, iface string }
	need := map[string]helper{} // function name -> declaration
	leave := func(why string, n ast.Node) {
		left[why]++
		leftAt[why] = append(leftAt[why], rw.Fset.Position(n.Pos()).String())
	}

	for _, file := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(file.Pos()).Name()), "g_") {
			continue
		}
		for _, d := range file.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || strings.HasPrefix(fd.Name.Name, "gcast_") {
				continue // the step's own functions end in the cast they stand for
			}
			fn, _ := rw.Info.Defs[fd.Name].(*types.Func)
			if fn == nil {
				continue
			}
			sig := fn.Type().(*types.Signature)
			tps := sig.RecvTypeParams()
			if tps.Len() == 0 {
				tps = sig.TypeParams()
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
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
					return true
				}
				it, isIface := t.Underlying().(*types.Interface)
				if !isIface || it.NumMethods() == 0 || !rw.ReflipGeneric_(t, false) {
					return true // icasts' sites, or nothing asserted
				}
				if tps.Len() == 0 {
					leave(id.Name+": the code has no type parameters of its own", call)
					return true
				}
				targs := make([]types.Type, tps.Len())
				var params, names []string
				for i := 0; i < tps.Len(); i++ {
					tp := tps.At(i)
					targs[i] = tp
					names = append(names, tp.Obj().Name())
					params = append(params, tp.Obj().Name()+" "+types.TypeString(tp.Constraint(), qual))
				}
				var arms []string
				for _, c := range classes {
					ct := c.Type().(*types.Named)
					if ct.TypeParams().Len() != tps.Len() {
						continue
					}
					same := true
					for i := 0; i < tps.Len(); i++ {
						if ct.TypeParams().At(i).Obj().Name() != names[i] {
							same = false
						}
					}
					if !same {
						continue
					}
					inst, err := types.Instantiate(nil, ct, targs, true)
					if err != nil {
						continue
					}
					if types.Implements(types.NewPointer(inst), it) {
						arms = append(arms, "*"+c.Name()+"["+strings.Join(names, ", ")+"]")
					}
				}
				if len(arms) == 0 {
					leave(id.Name+": no class of the code's type parameters implements the interface", call)
					return true
				}
				if len(arms) > maxArms {
					leave(id.Name+fmt.Sprintf(": more than %d classes of the code's type parameters implement the interface", maxArms), call)
					return true
				}
				itext := rw.Text(ix.Index)
				name := "gcast_" + sanitize(itext)
				var b strings.Builder
				fmt.Fprintf(&b, "\n\n// %s is cast_[%s],\n// trying first the classes of its type parameters that implement it (post-flip step gcasts).\n", name, itext)
				fmt.Fprintf(&b, "func %s[%s](v any) %s {\n\tswitch p := v.(type) {\n", name, strings.Join(params, ", "), itext)
				for _, a := range arms {
					fmt.Fprintf(&b, "\tcase %s:\n\t\treturn p\n", a)
				}
				fmt.Fprintf(&b, "\t}\n\treturn cast_[%s](v)\n}", itext)
				if prev, has := need[name]; has && prev.text != b.String() {
					leave(id.Name+": two functions with one name", call)
					return true
				}
				need[name] = helper{b.String(), itext}
				inst := name + "[" + strings.Join(names, ", ") + "]"
				if id.Name == "cast_" {
					done[fmt.Sprintf("cast_[I] -> gcast_I (%d classes tried)", len(arms))]++
					rw.Rewrite[ix] = func() string { return inst }
				} else {
					done[fmt.Sprintf("elemsAs_[I](c) -> elemsBy_(c, gcast_I) (%d classes tried)", len(arms))]++
					rw.Rewrite[call] = func() string { return "elemsBy_(" + rw.Render(call.Args[0]) + ", " + inst + ")" }
				}
				return true
			})
		}
	}

	// The functions go after the last declaration of g_0casts.go.
	var fnames []string
	for n := range need {
		if rw.Pkg.Scope().Lookup(n) == nil {
			fnames = append(fnames, n)
		}
	}
	sort.Strings(fnames)
	if len(fnames) > 0 {
		var last ast.Decl
		for _, f := range rw.Files {
			if filepath.Base(rw.Fset.File(f.Pos()).Name()) == "g_0casts.go" {
				last = f.Decls[len(f.Decls)-1]
			}
		}
		if last == nil {
			fmt.Fprintln(os.Stderr, "gcasts: no g_0casts.go")
			os.Exit(1)
		}
		var b strings.Builder
		for _, n := range fnames {
			b.WriteString(need[n].text)
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
		if verbose {
			for _, p := range leftAt[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
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
		fmt.Fprintf(os.Stderr, "gcasts: %d casts, %d new functions\n", total, len(fnames))
		return
	}
	fmt.Fprintf(os.Stderr, "gcasts: %d casts, %d new functions; files changed: %d\n", total, len(fnames), rw.SaveChecked())
}
