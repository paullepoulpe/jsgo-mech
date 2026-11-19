// Command enumcmp is a post-flip step: an enum's compareTo is java.lang.Enum's, which is final;
// the per-enum forwarders that j2go writes go, and their callers call Enum's.
//
//	func (this CheckAccessControls_Visibility) compareTo(o any) int32 {   (deleted)
//		return this.enum_().compareTo(o)
//	}
//	superclassVisibility.compareTo(obj_(subclassVisibility))
//	  ->
//	superclassVisibility.enum_().compareTo(obj_(subclassVisibility))
//
// Java's Enum.compareTo(E o) is final: no enum overrides it. j2go writes it on every enum type
// (an integer type whose constants index a table of Enum records) as a forwarder to the
// record's compareTo, with an Object parameter, so that the type satisfies the erased
// Comparable that the runtime's natural order asserts (compareNatural_: `interface{
// compareTo(any) int32 }`). The runtime patch of this step gives compareNatural_ the same call
// for any value with an enum record (`interface{ enum_() *Enum }`), after the compareTo case,
// so a natural-order comparison of enum values reaches Enum.compareTo as before.
//
// An enum's forwarder is deleted when its body is exactly `return this.enum_().compareTo(o)`
// and nothing names it other than by a call or a method value on a value of the enum's type
// (no interface type of the package lists compareTo; the runtime's assertion is the only
// dynamic use). Each such call x.compareTo(a) is x.enum_().compareTo(a): enum_() of x is what
// the forwarder evaluated first (it throws Java's NullPointerException for the null constant
// 0 there as here), then the same compareTo with the same argument. Java's behaviour is kept
// for every value.
//
// Left, counted: enums whose compareTo is not the plain forwarder, or is named otherwise. A
// rerun changes nothing.
//
// usage: enumcmp [-n] <dir of package closure>
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

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
	}
	rw.Load(os.Args[len(os.Args)-1])

	left := map[string]int{}
	// interfaces of the package that list compareTo: an enum converted to one needs the method.
	for _, n := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok {
			continue
		}
		if it, ok := tn.Type().Underlying().(*types.Interface); ok {
			for i := 0; i < it.NumMethods(); i++ {
				if it.Method(i).Name() == "compareTo" {
					fmt.Fprintf(os.Stderr, "enumcmp: interface %s lists compareTo: nothing done\n", n)
					os.Exit(1)
				}
			}
		}
	}
	fwd := map[*types.Func]*ast.FuncDecl{}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Name.Name != "compareTo" {
				continue
			}
			m := rw.Info.Defs[fd.Name].(*types.Func)
			recv := m.Type().(*types.Signature).Recv().Type()
			if _, ok := recv.Underlying().(*types.Basic); !ok {
				continue // a class
			}
			if !rw.HasMethod(recv, "enum_") {
				continue
			}
			if len(fd.Body.List) != 1 || strings.Join(strings.Fields(rw.Text(fd.Body.List[0])), " ") != "return this.enum_().compareTo(o)" ||
				fd.Type.Params.NumFields() != 1 || rw.Text(fd.Type.Params.List[0].Names[0]) != "o" {
				left["compareTo is not the forwarder to the Enum record"]++
				continue
			}
			fwd[m] = fd
		}
	}
	// every use of a forwarder: a selector on a value of the enum's type
	var uses []*ast.SelectorExpr
	bad := map[*types.Func]bool{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok {
				return true
			}
			m, ok := rw.Info.Uses[id].(*types.Func)
			if !ok || fwd[m] == nil {
				return true
			}
			sel, ok := rw.Parent[id].(*ast.SelectorExpr)
			if !ok || sel.Sel != id {
				bad[m] = true
				return true
			}
			if s := rw.Info.Selections[sel]; s == nil || s.Kind() != types.MethodVal || !types.Identical(rw.Info.TypeOf(sel.X), m.Type().(*types.Signature).Recv().Type()) {
				bad[m] = true
				return true
			}
			uses = append(uses, sel)
			return true
		})
	}
	ncall := 0
	for _, sel := range uses {
		m := rw.Info.Uses[sel.Sel].(*types.Func)
		if bad[m] {
			continue
		}
		sel := sel
		rw.Rewrite[sel] = func() string { return rw.Render(sel.X) + ".enum_().compareTo" }
		ncall++
	}
	var names []string
	for m, fd := range fwd {
		if bad[m] {
			left["compareTo named other than by a call on the enum's type"]++
			continue
		}
		fd := fd
		rw.Rewrite[fd] = func() string { return rw.DelDoc }
		names = append(names, m.Type().(*types.Signature).Recv().Type().(*types.Named).Obj().Name())
	}
	sort.Strings(names)
	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
	}
	fmt.Fprintf(os.Stderr, "enumcmp: %d forwarders deleted, %d calls of Enum.compareTo\n", len(names), ncall)
	if report || len(names) == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "enumcmp: files changed: %d\n", rw.SaveChecked())
}
