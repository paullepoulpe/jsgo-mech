// Command tostr is a post-flip step: String.valueOf and Object.toString of a value whose static
// type is known take the value itself, not its box.
//
// Java's `"" + x`, String.valueOf(x) and x.toString() on an Object are objstr_, String_valueOf_Object
// and Object_toString in the runtime, all taking `any`; j2go boxes the argument first (obj_,
// any(...), strobj_, x.box_()). Object_toString then picks the conversion from the dynamic type
// of the box: a boxed number or boolean is printed as Java prints it, anything else is asked for
// its toString(), and objstr_ / String_valueOf_Object print null as "null". When the static type
// of the boxed value is known, the dynamic type of the box is that type (or the box is nil for
// null), so the same conversion can be chosen statically:
//
//   - a class pointer, handle or integer enum with a toString() method, boxed so that null is nil
//     (obj_(x), x.box_()): objstr_ / String_valueOf_Object are tostr_(x) ("null" for the zero
//     value, x.toString() otherwise), Object_toString is tostrNN_(x) (NullPointerException for
//     the zero value); boxed with any(x), which keeps a nil pointer non-nil, all three are
//     x.toString(), which is what Object_toString called on that box;
//   - a String boxed by strobj_: objstr_ / String_valueOf_Object are sstr_(s) ("null" for null);
//   - a bool, int8/16/32/64, char (uint16), double or float boxed by any(...): boolstr_, istr_,
//     chrstr_, dstr_ or fstr_, the functions Object_toString calls for those boxes.
//
// The method called is the one the box's dynamic type has, so a virtual toString still
// dispatches as before. Left: arguments of an interface type (the box is the value), values
// whose type has no toString() (Object.toString prints the class name and identity hash), a
// String boxed by any(...) (a null String would print as the null marker, not "null"), and
// values of a type parameter. Rerunning the step changes nothing.
//
// usage: tostr [-n] <dir of package closure>
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

func runtimeFile(o types.Object) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(o.Pos()).Filename), "rt_")
}

// hasToString reports whether values of t have a method toString() string.
func hasToString(t types.Type) bool {
	if _, ok := t.Underlying().(*types.Interface); ok {
		return false
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	o, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, "toString")
	fn, ok := o.(*types.Func)
	if !ok {
		return false
	}
	sig := fn.Type().(*types.Signature)
	return sig.Params().Len() == 0 && sig.Results().Len() == 1 && types.Identical(sig.Results().At(0).Type(), types.Typ[types.String])
}

// basicStr is the runtime function Object_toString uses for a box of basic type t, and the
// conversion its argument needs.
func basicStr(t types.Type) (fn, conv string) {
	b, ok := t.(*types.Basic)
	if !ok {
		return "", ""
	}
	switch b.Kind() {
	case types.Bool:
		return "boolstr_", ""
	case types.Int64:
		return "istr_", ""
	case types.Int32, types.Int16, types.Int8:
		return "istr_", "int64"
	case types.Uint16:
		return "chrstr_", ""
	case types.Float64:
		return "dstr_", ""
	case types.Float32:
		return "fstr_", ""
	}
	return "", ""
}

// handleBox reports whether c is x.box_() of a handle class (the box is nil for the zero handle
// and has x's type otherwise).
func handleBox(c *ast.CallExpr) (ast.Expr, bool) {
	s, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || s.Sel.Name != "box_" || len(c.Args) != 0 {
		return nil, false
	}
	tv, ok := rw.Info.Types[s.X]
	if !ok {
		return nil, false
	}
	b, ok := tv.Type.Underlying().(*types.Basic)
	if !ok || b.Info()&types.IsInteger == 0 {
		return nil, false
	}
	if _, ok := types.Unalias(tv.Type).(*types.Named); !ok {
		return nil, false
	}
	return s.X, true
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
			if !ok || len(c.Args) != 1 {
				return true
			}
			id, ok := c.Fun.(*ast.Ident)
			if !ok {
				return true
			}
			o, ok := rw.Info.Uses[id].(*types.Func)
			if !ok || !runtimeFile(o) {
				return true
			}
			nn := false // Object_toString: null throws
			switch o.Name() {
			case "objstr_", "String_valueOf_Object":
			case "Object_toString":
				nn = true
			default:
				return true
			}
			fname := o.Name()
			arg, ok := rw.Unparen(c.Args[0]).(*ast.CallExpr)
			if !ok {
				return true
			}
			var x ast.Expr
			box := ""
			if hx, ok := handleBox(arg); ok {
				x, box = hx, "box_"
			} else if bid, ok := arg.Fun.(*ast.Ident); ok && len(arg.Args) == 1 {
				switch bid.Name {
				case "obj_", "strobj_", "any":
					x, box = arg.Args[0], bid.Name
				}
			}
			if x == nil {
				return true
			}
			tv, ok := rw.Info.Types[x]
			if !ok || tv.Type == nil {
				return true
			}
			t := tv.Type
			render := func() string { return rw.Render(x) }
			switch {
			case box == "strobj_" && !nn:
				stats[fname+": String"]++
				rw.Rewrite[c] = func() string { return "sstr_(" + render() + ")" }
			case box == "any":
				if fn, conv := basicStr(t); fn != "" {
					stats[fname+": "+t.String()]++
					rw.Rewrite[c] = func() string {
						if conv != "" {
							return fn + "(" + conv + "(" + render() + "))"
						}
						return fn + "(" + render() + ")"
					}
				} else if hasToString(t) {
					stats[fname+": toString() of any(x)"]++
					rw.Rewrite[c] = func() string { return rw.Render(x) + ".toString()" }
				} else {
					left[fname+": any(...) of "+kindOf(t)]++
				}
			case (box == "obj_" || box == "box_") && hasToString(t):
				helper := "tostr_"
				if nn {
					helper = "tostrNN_"
				}
				stats[fname+": toString() of "+box]++
				rw.Rewrite[c] = func() string { return helper + "(" + render() + ")" }
			default:
				left[fname+": "+box+" of "+kindOf(t)]++
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
	fmt.Printf("tostr: %d conversions take the value unboxed\n", total)
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

// kindOf describes a type that is left boxed.
func kindOf(t types.Type) string {
	switch u := t.Underlying().(type) {
	case *types.Interface:
		return "an interface"
	case *types.Slice:
		return "an array"
	case *types.Basic:
		if _, ok := types.Unalias(t).(*types.Named); ok {
			return "a named " + u.String() + " without toString"
		}
		return u.String()
	case *types.Pointer:
		return "a class without toString"
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return "a type parameter"
	}
	return t.String()
}
