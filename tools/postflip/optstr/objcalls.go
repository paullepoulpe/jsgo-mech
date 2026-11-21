package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"strings"

	"postflip/rw"
)

// Typed Object calls in the call graph (cha.go). The translator writes x.toString() of an x
// held at its static type, where the method is Object's, as a generic runtime wrapper:
// jtostr_(x) (String.valueOf) or tostrR_(x). Inside the wrapper the value is an `any`, and
// Object_toString asks for its toString through an interface assertion: followed blindly, one
// such call reaches every toString of the program. But the wrapper's argument has a static
// type at the call, and when that is a pointer to a class C of the package, the value is nil
// or a pointer to a C: Go converts the pointer to `any` with exactly the static pointer type
// (obj_ returns its argument), so the method Object_toString finds is the toString in the
// method set of *C, or none (defaultToString_). The call then has that one edge.
//
// This rests on what the four runtime functions do, so the tool holds their text (objWant) and
// uses the rule only while the runtime's functions are that text, token for token; otherwise it
// says so and follows the wrappers as ordinary calls (every toString).
var objWant = map[string]string{
	"jtostr_": `func jtostr_[T comparable](v T) string { return objstr_(obj_(v)) }`,
	"tostrR_": `func tostrR_[T comparable](v T) string { return Object_toString(obj_(v)) }`,
	"obj_": `func obj_[T comparable](x T) any {
	var zero T
	if x == zero {
		return nil
	}
	return x
}`,
	"objstr_": `func objstr_(v any) string {
	if v == nil {
		return "null"
	}
	return Object_toString(v)
}`,
	"Object_toString": `func Object_toString(v any) string {
	switch x := v.(type) {
	case nil:
		panic(new_NullPointerException_0().self())
	case string:
		return x
	case int32:
		return istr_(int64(x))
	case int64:
		return istr_(x)
	case int16:
		return istr_(int64(x))
	case int8:
		return istr_(int64(x))
	case bool:
		return boolstr_(x)
	case uint16:
		return chrstr_(x)
	case float64:
		return dstr_(x)
	case float32:
		return fstr_(x)
	case interface{ toString() string }:
		return x.toString()
	}
	return defaultToString_(v)
}`,
}

var objWrappers = map[string]bool{"jtostr_": true, "tostrR_": true}

var objRule = 0 // 0 not checked, 1 holds, 2 the runtime differs

func objRuleHolds() bool {
	if objRule == 0 {
		objRule = 1
		norm := func(s string) string { return strings.Join(strings.Fields(s), " ") }
		for name, want := range objWant {
			fn, _ := rw.Pkg.Scope().Lookup(name).(*types.Func)
			var fd *ast.FuncDecl
			if fn != nil {
				fd = funcDecls[fn]
			}
			if fd == nil || norm(rw.Text(fd)) != norm(want) {
				fmt.Fprintf(os.Stderr, "optstr: the runtime's %s is not the function the typed toString rule knows: every toString is followed\n", name)
				objRule = 2
			}
		}
	}
	return objRule == 1
}

// objCallEdges: the callees of the typed Object call x (a call of fn), if the rule applies.
func objCallEdges(x *ast.CallExpr, fn *types.Func) ([]ast.Node, bool) {
	if !objWrappers[fn.Name()] || fn.Pkg() != rw.Pkg || fn.Type().(*types.Signature).Recv() != nil || len(x.Args) != 1 {
		return nil, false
	}
	p, ok := types.Unalias(rw.Info.TypeOf(x.Args[0])).(*types.Pointer)
	if !ok {
		return nil, false
	}
	named, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || named.Obj().Pkg() != rw.Pkg {
		return nil, false
	}
	if _, ok := named.Underlying().(*types.Struct); !ok {
		return nil, false
	}
	if !objRuleHolds() {
		return nil, false
	}
	o, _, _ := types.LookupFieldOrMethod(p, false, rw.Pkg, "toString")
	m, _ := o.(*types.Func)
	if m == nil {
		d, _ := rw.Pkg.Scope().Lookup("defaultToString_").(*types.Func)
		if d == nil || funcDecls[d] == nil {
			return nil, false
		}
		return []ast.Node{funcDecls[d]}, true
	}
	sig := m.Type().(*types.Signature)
	if sig.Params().Len() != 0 || sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), types.Typ[types.String]) {
		return nil, false
	}
	fd := funcDecls[m.Origin()]
	if fd == nil {
		return nil, false
	}
	return []ast.Node{fd}, true
}
