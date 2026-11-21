// Command anyconv is a post-flip step: a value that already is an interface is not boxed again
// where an Object is wanted.
//
// j2go writes Java's widening to Object as an explicit conversion, any(x) or obj_(x), also when
// x is a value of an interface type (a List, a Function, an Object already). Converting an
// interface value to `any` keeps its dynamic type and value, nil stays nil; obj_(x) of an
// interface x is nil when x is nil and x otherwise, the same. So where the slot the value goes to
// is itself `any` (a parameter declared `any`, an []any element, a result or variable of type
// `any`, the operand of == / != against an `any`), the conversion does nothing and goes:
// `f(any(list))` is `f(list)`, `return obj_(fn)` is `return fn`.
//
// Left: conversions whose slot is not `any` (a := declaration, whose variable takes the type;
// a parameter of a type parameter, whose type argument it fixes; a method called on the box),
// and, unless -all is given, arguments of the runtime helpers other steps match by their
// argument's form (foreach_: the collections steps; icast_/cast_/to_/is_: the casts step;
// Preconditions: the precondition step; Object_equals, Objects_*, objequals_, objhash_: the
// equals step). Rerunning the step changes nothing.
//
// usage: anyconv [-n] [-all] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

func isIface(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

func isAny(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	it, ok := t.Underlying().(*types.Interface)
	return ok && it.Empty()
}

// others are the helpers whose arguments other steps match by form (see the doc comment).
func others(name string) string {
	switch {
	case name == "foreach_":
		return "foreach_ (collections steps)"
	case strings.HasPrefix(name, "icast_"), strings.HasPrefix(name, "cast_"), strings.HasPrefix(name, "to_"), strings.HasPrefix(name, "is_"):
		return "casts (casts step)"
	case strings.HasPrefix(name, "Preconditions_"):
		return "Preconditions (precondition step)"
	case name == "Object_equals", strings.HasPrefix(name, "Objects_"), strings.HasPrefix(name, "objequals"), strings.HasPrefix(name, "objhash"), name == "Object_hashCode":
		return "equals/hashCode (equals step)"
	}
	return ""
}

// declParam is the declared type of parameter i of the function called by c (the generic
// declaration's, so that a type parameter is seen as one), or nil.
func declParam(c *ast.CallExpr, i int) (types.Type, string) {
	fun := rw.Unparen(c.Fun)
	if ix, ok := fun.(*ast.IndexExpr); ok {
		fun = ix.X
	} else if ix, ok := fun.(*ast.IndexListExpr); ok {
		fun = ix.X
	}
	var sig *types.Signature
	name := ""
	switch f := fun.(type) {
	case *ast.Ident:
		if fn, ok := rw.Info.Uses[f].(*types.Func); ok {
			sig, name = fn.Origin().Type().(*types.Signature), fn.Name()
		} else if tv, ok := rw.Info.Types[f]; ok && !tv.IsType() {
			sig, _ = tv.Type.Underlying().(*types.Signature)
		}
	case *ast.SelectorExpr:
		if s, ok := rw.Info.Selections[f]; ok {
			if fn, ok := s.Obj().(*types.Func); ok {
				sig, name = fn.Origin().Type().(*types.Signature), fn.Name()
			} else {
				sig, _ = s.Type().Underlying().(*types.Signature)
			}
		} else if fn, ok := rw.Info.Uses[f.Sel].(*types.Func); ok {
			sig, name = fn.Origin().Type().(*types.Signature), fn.Name()
		}
	case *ast.FuncLit:
		sig, _ = rw.Info.Types[f].Type.(*types.Signature)
	}
	if sig == nil {
		return nil, name
	}
	n := sig.Params().Len()
	if sig.Variadic() && i >= n-1 {
		if c.Ellipsis.IsValid() {
			return nil, name
		}
		return sig.Params().At(n - 1).Type().(*types.Slice).Elem(), name
	}
	if i < n {
		return sig.Params().At(i).Type(), name
	}
	return nil, name
}

func main() {
	report, all := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		switch a {
		case "-n":
			report = true
		case "-all":
			all = true
		}
	}
	rw.Load(os.Args[len(os.Args)-1])
	stats := map[string]int{}
	left := map[string]int{}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		var results []*types.Tuple // the enclosing functions' results
		var visit func(n ast.Node) bool
		visit = func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				if fn, ok := rw.Info.Defs[x.Name].(*types.Func); ok && x.Body != nil {
					results = append(results, fn.Type().(*types.Signature).Results())
					ast.Inspect(x.Body, visit)
					results = results[:len(results)-1]
				}
				return false
			case *ast.FuncLit:
				results = append(results, rw.Info.Types[x].Type.(*types.Signature).Results())
				ast.Inspect(x.Body, visit)
				results = results[:len(results)-1]
				return false
			case *ast.CallExpr:
				conv(x, results, all, stats, left)
			}
			return true
		}
		ast.Inspect(f, visit)
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
	fmt.Printf("anyconv: %d conversions of interface values to Object removed\n", total)
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
	fmt.Printf("files changed: %d\n", rw.SaveChecked())
}

// conv removes c if it is any(x) or obj_(x) of an interface value x whose slot is `any`.
func conv(c *ast.CallExpr, results []*types.Tuple, all bool, stats, left map[string]int) {
	id, ok := c.Fun.(*ast.Ident)
	if !ok || len(c.Args) != 1 || (id.Name != "any" && id.Name != "obj_") {
		return
	}
	if id.Name == "any" {
		if _, ok := rw.Info.Uses[id].(*types.TypeName); !ok {
			return
		}
	} else if fn, ok := rw.Info.Uses[id].(*types.Func); !ok || !strings.HasPrefix(filepath.Base(rw.Fset.Position(fn.Pos()).Filename), "rt_") {
		return
	}
	x := c.Args[0]
	if !isIface(rw.Info.Types[x].Type) {
		return
	}
	form := id.Name + "(interface)"
	slot := ""
	p := rw.Up(c)
	switch u := p.(type) {
	case *ast.CallExpr:
		idx := -1
		for i, a := range u.Args {
			if rw.Unparen(a) == c {
				idx = i
			}
		}
		if idx < 0 {
			left[form+": method called on the box"]++
			return
		}
		t, name := declParam(u, idx)
		if o := others(name); o != "" && !all {
			left[form+": argument of "+o]++
			return
		}
		if !isAny(t) {
			left[form+": argument of a parameter that is not Object"]++
			return
		}
		slot = "argument"
	case *ast.CompositeLit:
		if t := rw.Info.Types[u].Type; t != nil {
			switch ct := t.Underlying().(type) {
			case *types.Slice:
				if isAny(ct.Elem()) {
					slot = "[]any element"
				}
			case *types.Array:
				if isAny(ct.Elem()) {
					slot = "[]any element"
				}
			}
		}
	case *ast.KeyValueExpr:
		if u.Value == c || rw.Unparen(u.Value) == c {
			if kid, ok := u.Key.(*ast.Ident); ok {
				if v, ok := rw.Info.Uses[kid].(*types.Var); ok && v.IsField() && isAny(v.Type()) {
					slot = "field in a composite literal"
				}
			}
		}
	case *ast.ReturnStmt:
		if len(results) > 0 {
			res := results[len(results)-1]
			if res.Len() == len(u.Results) {
				for i, r := range u.Results {
					if rw.Unparen(r) == c && isAny(res.At(i).Type()) {
						slot = "result"
					}
				}
			}
		}
	case *ast.AssignStmt:
		if u.Tok == token.ASSIGN && len(u.Lhs) == len(u.Rhs) {
			for i, r := range u.Rhs {
				if rw.Unparen(r) == c && isAny(rw.Info.Types[u.Lhs[i]].Type) {
					slot = "assignment"
				}
			}
		}
	case *ast.ValueSpec:
		if u.Type != nil && isAny(rw.Info.Types[u.Type].Type) {
			slot = "variable"
		}
	case *ast.BinaryExpr:
		if u.Op == token.EQL || u.Op == token.NEQ {
			other := u.X
			if rw.Unparen(u.X) == c {
				other = u.Y
			}
			if isAny(rw.Info.Types[other].Type) {
				slot = "comparison with an Object"
				// both operands lose their conversion: their types must still compare
				if oc, ok := rw.Unparen(other).(*ast.CallExpr); ok && len(oc.Args) == 1 {
					if oid, ok := oc.Fun.(*ast.Ident); ok && (oid.Name == "any" || oid.Name == "obj_") {
						a, b := rw.Info.Types[x].Type, rw.Info.Types[oc.Args[0]].Type
						if isIface(b) && !types.AssignableTo(a, b) && !types.AssignableTo(b, a) {
							slot = ""
						}
					}
				}
			}
		}
	}
	if slot == "" {
		left[form+fmt.Sprintf(": in a %T whose slot is not Object", p)]++
		return
	}
	stats[form+" as "+slot]++
	rw.Rewrite[c] = func() string { return rw.Render(x) }
}
