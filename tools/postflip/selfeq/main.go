// Command selfeq is a post-flip step: the check "is this object exactly the class whose method
// runs" in a dispatcher compares one type word and one pointer, without a runtime call.
//
// The overrides step gives a class C that overrides m its own m, which calls m_impl directly
// when the object is exactly a C and dispatches otherwise:
//
//	if this.self_ == any(this) {
//
// Go compares two interface values by comparing their types and then calling runtime.efaceeq
// for the data words, even when one side is a pointer converted on the spot; the call also
// makes the dispatcher spill its arguments. There are about 1,500 such dispatchers, and every
// virtual call through an exact object pays it. This step writes the check as a type assertion
// to the receiver's pointer type:
//
//	if p, _ := this.self_.(*C); p == this {
//
// which is true exactly when the comparison was (the dynamic type of self_ is *C and its data is
// this; on any other type p is nil, and this is not nil, or reading this.self_ would have
// panicked). The compiler emits it as one comparison of the type (or itab) word with a constant
// and one of the pointers. A receiver type that the type of self_ cannot hold (the assertion
// would not compile; the comparison is always false) is left alone.
//
// Only an if statement without an init statement whose whole condition is the comparison is
// rewritten, so the step can be rerun (a rewritten one has an init statement).
//
// usage: selfeq [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"strings"

	"postflip/rw"
)

func main() {
	dry := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	n, left := 0, 0
	for _, f := range rw.Files {
		if !strings.HasPrefix(rw.Fset.File(f.Pos()).Name()[strings.LastIndex(rw.Fset.File(f.Pos()).Name(), "/")+1:], "g_") {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			is, ok := x.(*ast.IfStmt)
			if !ok || is.Init != nil {
				return true
			}
			be, ok := is.Cond.(*ast.BinaryExpr)
			if !ok || be.Op != token.EQL {
				return true
			}
			sel, ok := be.X.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "self_" {
				return true
			}
			recv, ok := sel.X.(*ast.Ident)
			if !ok {
				return true
			}
			conv, ok := be.Y.(*ast.CallExpr)
			if !ok || len(conv.Args) != 1 {
				return true
			}
			if fn, ok := conv.Fun.(*ast.Ident); !ok || fn.Name != "any" {
				return true
			}
			arg, ok := conv.Args[0].(*ast.Ident)
			if !ok || arg.Name != recv.Name || rw.Info.Uses[arg] != rw.Info.Uses[recv] {
				return true
			}
			pt, ok := rw.Info.TypeOf(arg).(*types.Pointer)
			if !ok {
				return true
			}
			nt, ok := pt.Elem().(*types.Named)
			if !ok {
				return true
			}
			it, ok := rw.Info.TypeOf(sel).Underlying().(*types.Interface)
			if !ok {
				return true
			}
			if !types.AssertableTo(it, pt) {
				left++
				return true
			}
			n++
			cond := be
			rw.Rewrite[cond] = func() string {
				return fmt.Sprintf("p, _ := %s.self_.(*%s); p == %s", recv.Name, nt.Obj().Name(), recv.Name)
			}
			return true
		})
	}
	fmt.Fprintf(os.Stderr, "selfeq: %d dispatcher checks rewritten, %d left (the type of self_ cannot hold the receiver type)\n", n, left)
	if dry {
		return
	}
	fmt.Fprintf(os.Stderr, "selfeq: %d files written\n", rw.SaveChecked())
}
