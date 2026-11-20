// Command tgeneric is a post-flip step: a call of a Java generic method (<T> T m(...)) that the
// translation erased, on typed values, whose result the caller casts back, is the method's
// typed form (rt_tgen.go):
//
//	cast_[*X](Iterables_getOnlyElement(Iterable(l.asList())))     ->  l.getOnlyElementT_()
//	str_(Iterables_getFirst(Iterable(sliceView_(tc, s)), nil))    ->  sliceGetFirstT_(tc, s, nil)
//	Node_of_(Iterables_getLast(Iterable(sliceViewP_(tc, p))))     ->  sliceGetLastT_(tc, *p)
//	cast_[*X](this.runInCompilerThread((&Callable_tfn_X{f}).call))  ->  Compiler_runInCompilerThreadT_(this, f)
//
// The collection is a typed list (asList() / asImm()), a typed set (asSet(), getOnlyElement) or a
// native slice seen through sliceView_ / sliceViewP_. The typed forms return the item the erased
// call returned and make the erased call on every other path, so they throw the same exceptions
// with the same messages; runInCompilerThread runs the same callable in the same executor and
// takes the result from the call instead of from its box. The result is read unboxed where the
// code cast it, boxed elsewhere. Left (counted): erased collections, results not cast. A rerun
// changes nothing.
//
// usage: tgeneric [-n] [-v] <dir of package closure>
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

var (
	verbose bool
	done    = map[string]int{}
	left    = map[string][]string{}
)

func leave(n ast.Node, why string) {
	left[why] = append(left[why], rw.Fset.Position(n.Pos()).String())
}

func generic(t types.Type, name string) []types.Type {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.Origin().Obj().Name() != name || n.Obj().Pkg() != rw.Pkg {
		return nil
	}
	var out []types.Type
	for i := 0; i < n.TypeArgs().Len(); i++ {
		out = append(out, n.TypeArgs().At(i))
	}
	return out
}

// read: the rewrite of read n (of type any) whose value of type t is code().
func read(t types.Type, n ast.Expr, code func() string) (rw.Edit, bool) {
	cv := rw.ConvFor(t)
	switch p := rw.Up(n).(type) {
	case *ast.TypeAssertExpr:
		if rw.Unparen(p.X) == n && p.Type != nil && types.Identical(rw.Info.TypeOf(p), t) {
			return rw.Edit{Node: p, Gen: code}, true
		}
	case *ast.CallExpr:
		if len(p.Args) == 1 && rw.Unparen(p.Args[0]) == n {
			name, _ := rw.CallName(p)
			if (strings.HasPrefix(name, "cast_[") || strings.HasPrefix(name, "unbox_[")) && types.Identical(rw.Info.TypeOf(p), t) || cv.OK && name != "" && cv.IsUnbox(name) {
				return rw.Edit{Node: p, Gen: code}, true
			}
		}
	}
	if !cv.OK {
		return rw.Edit{}, false
	}
	return rw.Edit{Node: n, Gen: func() string { return cv.Box(code()) }}, true
}

func stripIter(e ast.Expr) ast.Expr {
	e = rw.Unparen(e)
	if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 {
		if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() {
			return rw.Unparen(c.Args[0])
		}
		if n, _ := rw.CallName(c); n == "iup_Iterable" {
			return rw.Unparen(c.Args[0])
		}
	}
	return e
}

func visit(c *ast.CallExpr) {
	name, _ := rw.CallName(c)
	var m string
	switch name {
	case "Iterables_getOnlyElement":
		m = "getOnlyElement"
	case "Iterables_getFirst":
		m = "getFirst"
	case "Iterables_getLast":
		m = "getLast"
	default:
		if s, ok := c.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "runInCompilerThread" && len(c.Args) == 1 {
			compilerThread(c, s)
		}
		return
	}
	if m == "getFirst" && len(c.Args) != 2 || m != "getFirst" && len(c.Args) != 1 {
		return
	}
	v := stripIter(c.Args[0])
	var elem types.Type
	var code func() string
	vc, ok := v.(*ast.CallExpr)
	if !ok {
		leave(c, m+": an erased collection")
		return
	}
	def := func() string {
		if m == "getFirst" {
			return rw.Render(c.Args[1])
		}
		return ""
	}
	if s, ok := vc.Fun.(*ast.SelectorExpr); ok && len(vc.Args) == 0 {
		x := s.X
		switch {
		case (s.Sel.Name == "asList" || s.Sel.Name == "asImm") && generic(rw.Info.TypeOf(x), "TList") != nil:
			elem = generic(rw.Info.TypeOf(x), "TList")[0]
			code = func() string { return rw.Render(x) + "." + m + "T_(" + def() + ")" }
		case s.Sel.Name == "asSet" && m == "getOnlyElement" && generic(rw.Info.TypeOf(x), "TSet") != nil:
			elem = generic(rw.Info.TypeOf(x), "TSet")[0]
			code = func() string { return rw.Render(x) + ".getOnlyElementT_()" }
		}
	} else if vn, _ := rw.CallName(vc); len(vc.Args) == 2 && (vn == "sliceView_" || vn == "sliceViewP_") {
		st := rw.Info.TypeOf(vc.Args[1])
		deref := ""
		if vn == "sliceViewP_" {
			st = st.(*types.Pointer).Elem()
			deref = "*"
		}
		elem = st.Underlying().(*types.Slice).Elem()
		tc, sl := vc.Args[0], vc.Args[1]
		fn := "slice" + strings.ToUpper(m[:1]) + m[1:] + "T_"
		code = func() string {
			s := fn + "(" + rw.Render(tc) + ", " + deref + rw.Render(sl)
			if m == "getFirst" {
				s += ", " + def()
			}
			return s + ")"
		}
	}
	if code == nil {
		leave(c, m+": an erased collection")
		return
	}
	e, ok := read(elem, c, code)
	if !ok || e.Node == ast.Node(c) {
		leave(c, m+": the result is not cast to the element type (given to erased code)")
		return
	}
	rw.Rewrite[e.Node] = e.Gen
	done["Iterables."+m+" of a typed collection, cast back: its typed form"]++
}

func compilerThread(c *ast.CallExpr, s *ast.SelectorExpr) {
	p, ok := types.Unalias(rw.Info.TypeOf(s.X)).(*types.Pointer)
	if !ok {
		return
	}
	if n, ok := types.Unalias(p.Elem()).(*types.Named); !ok || n.Obj().Name() != "Compiler" {
		return
	}
	mv, ok := rw.Unparen(c.Args[0]).(*ast.SelectorExpr)
	if !ok || mv.Sel.Name != "call" {
		leave(c, "runInCompilerThread: an erased callable (func() any, result unused)")
		return
	}
	u, ok := rw.Unparen(mv.X).(*ast.UnaryExpr)
	if !ok {
		leave(c, "runInCompilerThread: not a typed callable adapter")
		return
	}
	cl, ok := u.X.(*ast.CompositeLit)
	if !ok || len(cl.Elts) != 1 || !strings.Contains(rw.Text(cl.Type), "_tfn") {
		leave(c, "runInCompilerThread: not a typed callable adapter")
		return
	}
	f := cl.Elts[0]
	sig, ok := types.Unalias(rw.Info.TypeOf(f)).(*types.Signature)
	if !ok || sig.Params().Len() != 0 || sig.Results().Len() != 1 {
		leave(c, "runInCompilerThread: not a typed callable adapter")
		return
	}
	x := s.X
	e, ok := read(sig.Results().At(0).Type(), c, func() string {
		return "Compiler_runInCompilerThreadT_(" + rw.Render(x) + ", " + rw.Render(f) + ")"
	})
	if !ok || e.Node == ast.Node(c) {
		leave(c, "runInCompilerThread: the result is not cast back")
		return
	}
	rw.Rewrite[e.Node] = e.Gen
	done["Compiler.runInCompilerThread of a typed callable, cast back: Compiler_runInCompilerThreadT_"]++
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok {
				visit(c)
			}
			return true
		})
	}
	var ks []string
	total := 0
	for k := range done {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(os.Stderr, "tgeneric: %d %s\n", done[k], k)
		total += done[k]
	}
	ks = ks[:0]
	for k := range left {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(os.Stderr, "tgeneric: left: %d %s\n", len(left[k]), k)
		if verbose {
			for _, w := range left[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", w)
			}
		}
	}
	if report || total == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "tgeneric: files changed: %d\n", rw.SaveChecked())
}
