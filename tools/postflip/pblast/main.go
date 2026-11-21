// Command pblast is a post-flip step: a protobuf builder's last build is buildLastT_(), which
// turns the builder's allocation into the message instead of copying it into a new one:
//
//	return builder.buildT_()                        ->   return builder.buildLastT_()
//	G_newBuilder().setA(a).setB(b).buildT_()        ->   G_newBuilder().setA(a).setB(b).buildLastT_()
//
// Java's build() makes a message object beside the builder; the translation does the same
// (pbData_.build_ copies the builder's struct of fields into a new message). When nothing can
// use the builder after the call, the copy cannot be told from the builder's own storage, and
// the runtime's message and builder of one class have one layout (rt_proto.go, buildLast_).
// A serialized AST has one builder and one message per node.
//
// What the step has to show is that the builder is not used after the call. It shows it for
// two shapes:
//
//	A. the receiver is a fresh builder: an expression G_newBuilder() followed by zero or more
//	   generated builder methods that return their receiver. No name refers to it.
//	B. the call is the result of a return statement of the function that declares the
//	   receiver, an owned local variable (the receiver may be followed by generated methods
//	   that return it). The function ends there, so no later code of this activation runs,
//	   and an owned variable's builder is reachable from nowhere else.
//
// A local variable is owned when it is declared once with a fresh value, never assigned again
// and never has its address taken, and each use is one of:
//   - the receiver of a generated builder method (rt_proto_gen.go), whose result, when the
//     method returns its receiver, is dropped, or is again such a receiver, or is returned
//     as in B;
//   - an argument of a call to a function or method whose parameter in that position is
//     borrowed: every use of the parameter is of the kinds in this list. A call through an
//     interface is resolved by name: every method of the package with that name and number
//     of parameters must borrow it;
//   - a use inside a function literal counts only if the literal is called where it stands.
//
// A fresh value is G_newBuilder(), a generated receiver-returning method on a fresh value, or
// a call of a function (not through an interface) each of whose return statements returns a
// fresh value or an owned local variable of its own.
//
// The generated builder methods are checked, not assumed: in the body of each one used, the
// receiver `this` occurs only as the operand of a field or method selection that is not under
// an address-of, or as the whole result of a return (then the method "returns its receiver").
// So such a method keeps no reference to the builder anywhere but in its result. What it
// calls on the receiver is the handwritten runtime (rt_proto.go), which stores no builder
// except as an element of another builder's field, where build_ replaces it by a built copy.
//
// The step refuses to run if buildLastT_ is missing from a builder class whose call it would
// rewrite. A rerun changes nothing.
//
// usage: pblast [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"strings"

	"postflip/rw"
)

var (
	decls   = map[*types.Func]*ast.FuncDecl{}
	byName  = map[string][]*ast.FuncDecl{}
	verbose bool
)

func fileOf(n ast.Node) string { return filepath.Base(rw.Fset.File(n.Pos()).Name()) }

// isBuilder reports whether t is a protobuf builder class with a typed build.
func isBuilder(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := t.(*types.Pointer); !ok {
		return false
	}
	if pb, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, "pbb_"); pb == nil {
		return false
	}
	o, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, "buildT_")
	_, ok := o.(*types.Func)
	return ok
}

// method classification of a generated builder method
const (
	mNo    = iota // not a generated builder method, or one that may keep its receiver
	mChain        // returns its receiver
	mRead         // keeps no reference to its receiver, in its result either
)

var methodKind = map[*types.Func]int{}

func genMethod(fn *types.Func) int {
	if k, ok := methodKind[fn]; ok {
		return k
	}
	k := genMethod1(fn)
	methodKind[fn] = k
	return k
}

func genMethod1(fn *types.Func) int {
	d := decls[fn]
	if d == nil || d.Recv == nil || d.Body == nil || fileOf(d) != "rt_proto_gen.go" {
		return mNo
	}
	if len(d.Recv.List) != 1 || len(d.Recv.List[0].Names) != 1 || d.Recv.List[0].Names[0].Name != "this" {
		return mNo
	}
	recv := rw.Info.Defs[d.Recv.List[0].Names[0]]
	if recv == nil || !isBuilder(recv.Type()) {
		return mNo
	}
	returnsThis, other, bad := false, false, false
	ast.Inspect(d.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			bad = true
		case *ast.ReturnStmt:
			if len(x.Results) == 1 {
				if id, ok := rw.Unparen(x.Results[0]).(*ast.Ident); ok && rw.Info.Uses[id] == recv {
					returnsThis = true
					return false
				}
			}
			if len(x.Results) > 0 {
				other = true
			}
		case *ast.Ident:
			if rw.Info.Uses[x] != recv {
				return true
			}
			// the operand of a selection, not under an address-of
			s, ok := rw.Up(x).(*ast.SelectorExpr)
			if !ok || rw.Unparen(s.X) != ast.Expr(x) {
				bad = true
				return true
			}
			var e ast.Node = s
			for {
				p := rw.Up(e)
				if ps, ok := p.(*ast.SelectorExpr); ok {
					e = ps
					continue
				}
				if u, ok := p.(*ast.UnaryExpr); ok && u.Op == token.AND {
					bad = true
				}
				break
			}
		}
		return true
	})
	if bad || returnsThis && other {
		return mNo
	}
	if returnsThis {
		return mChain
	}
	return mRead
}

// methodCall returns the call whose receiver expression is e, with the method called.
func methodCall(e ast.Expr) (*ast.CallExpr, *types.Func) {
	var n ast.Node = e
	for {
		p, ok := rw.Parent[n].(*ast.ParenExpr)
		if !ok {
			break
		}
		n = p
	}
	s, ok := rw.Parent[n].(*ast.SelectorExpr)
	if !ok || s.X != n.(ast.Expr) {
		return nil, nil
	}
	c, ok := rw.Parent[s].(*ast.CallExpr)
	if !ok || c.Fun != ast.Expr(s) {
		return nil, nil
	}
	fn, _ := rw.Info.Uses[s.Sel].(*types.Func)
	return c, fn
}

// enclosing returns the function declaration or literal around n.
func enclosing(n ast.Node) ast.Node {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch p.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return p
		}
	}
	return nil
}

// home returns the function declaration around n when n is in its body proper or in function
// literals that are called where they stand; nil otherwise.
func home(n ast.Node) *ast.FuncDecl {
	for {
		switch f := enclosing(n).(type) {
		case *ast.FuncDecl:
			return f
		case *ast.FuncLit:
			c, ok := rw.Up(f).(*ast.CallExpr)
			if !ok || rw.Unparen(c.Fun) != ast.Expr(f) {
				return nil
			}
			switch rw.Up(c).(type) {
			case *ast.DeferStmt, *ast.GoStmt:
				return nil
			}
			n = f
		default:
			return nil
		}
	}
}

// useKind of an occurrence e of a builder value (a variable, or a receiver-returning call on one)
const (
	uBad    = iota
	uOK     // a use that keeps no reference
	uBuild  // return <e>.buildT_() of the declaring function: the last build (call recorded)
	uReturn // return <e>
)

// use classifies the occurrence e of the builder held by a variable declared in fd.
func use(e ast.Expr, fd *ast.FuncDecl, builds *[]*ast.CallExpr, why *string) int {
	if c, fn := methodCall(e); c != nil {
		if fn == nil {
			*why = "method not resolved"
			return uBad
		}
		if fn.Name() == "buildT_" && isBuilder(rw.Info.TypeOf(e)) {
			if r, ok := rw.Up(c).(*ast.ReturnStmt); ok && len(r.Results) == 1 && enclosing(r) == ast.Node(fd) {
				*builds = append(*builds, c)
				return uBuild
			}
			return uOK // a build in the middle: copies, the builder goes on
		}
		switch genMethod(fn) {
		case mRead:
			return uOK
		case mChain:
			if _, ok := rw.Up(c).(*ast.ExprStmt); ok {
				return uOK
			}
			return use(c, fd, builds, why)
		}
		*why = "method " + fn.Name() + " is not a generated builder method that keeps no reference"
		return uBad
	}
	switch p := rw.Up(e).(type) {
	case *ast.ReturnStmt:
		if len(p.Results) == 1 && enclosing(p) == ast.Node(fd) {
			return uReturn
		}
	case *ast.CallExpr:
		for i, a := range p.Args {
			if rw.Unparen(a) == rw.Unparen(e) {
				if p.Ellipsis.IsValid() {
					break
				}
				if borrowedArg(p, i, why) {
					return uOK
				}
				return uBad
			}
		}
	}
	*why = "used at " + rw.Fset.Position(e.Pos()).String()
	return uBad
}

type paramKey struct {
	d *ast.FuncDecl
	i int
}

var borrowed = map[paramKey]bool{}

// borrowedArg reports whether argument i of call c is only borrowed by whatever c calls.
func borrowedArg(c *ast.CallExpr, i int, why *string) bool {
	var id *ast.Ident
	switch f := rw.Unparen(c.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	}
	if id == nil {
		*why = "call of a function value at " + rw.Fset.Position(c.Pos()).String()
		return false
	}
	fn, _ := rw.Info.Uses[id].(*types.Func)
	if fn == nil {
		*why = "call of a function value at " + rw.Fset.Position(c.Pos()).String()
		return false
	}
	sig := fn.Type().(*types.Signature)
	if sig.Variadic() && i >= sig.Params().Len()-1 {
		*why = "variadic argument of " + fn.Name()
		return false
	}
	var targets []*ast.FuncDecl
	if d := decls[fn]; d != nil {
		targets = []*ast.FuncDecl{d}
	} else if r := sig.Recv(); r != nil && types.IsInterface(r.Type()) {
		for _, d := range byName[fn.Name()] {
			if d.Recv != nil && nparams(d) == sig.Params().Len() {
				targets = append(targets, d)
			}
		}
	}
	if len(targets) == 0 {
		*why = "no declaration of " + fn.Name()
		return false
	}
	for _, d := range targets {
		if !borrows(d, i, why) {
			return false
		}
	}
	return true
}

func nparams(d *ast.FuncDecl) int {
	n := 0
	for _, f := range d.Type.Params.List {
		if len(f.Names) == 0 {
			n++
		}
		n += len(f.Names)
	}
	return n
}

func param(d *ast.FuncDecl, i int) *ast.Ident {
	n := 0
	for _, f := range d.Type.Params.List {
		if len(f.Names) == 0 {
			n++
		}
		for _, id := range f.Names {
			if n == i {
				return id
			}
			n++
		}
	}
	return nil
}

// borrows reports whether parameter i of d is only borrowed: no use of it keeps a reference.
// A parameter under examination counts as borrowed (the property is the largest one closed
// under the rules, as for any "never escapes").
func borrows(d *ast.FuncDecl, i int, why *string) bool {
	k := paramKey{d, i}
	if v, ok := borrowed[k]; ok {
		return v
	}
	borrowed[k] = true
	ok := borrows1(d, i, why)
	borrowed[k] = ok
	return ok
}

func borrows1(d *ast.FuncDecl, i int, why *string) bool {
	id := param(d, i)
	if id == nil || d.Body == nil {
		*why = "parameter " + fmt.Sprint(i) + " of " + d.Name.Name + " has no name or body"
		return false
	}
	if id.Name == "_" {
		return true
	}
	v := rw.Info.Defs[id]
	if v == nil || !isBuilder(v.Type()) {
		*why = "parameter of " + d.Name.Name + " is not of a builder class"
		return false
	}
	var builds []*ast.CallExpr
	for _, u := range rw.UsesOf[v] {
		if !plainRead(u, d, why) {
			return false
		}
		if k := use(u, d, &builds, why); k != uOK && k != uBuild {
			// a parameter's builder is the caller's: returning it would alias it
			if *why == "" {
				*why = "parameter returned by " + d.Name.Name
			}
			return false
		}
	}
	return true
}

// plainRead reports whether the use u of a variable declared in d reads it in d's body proper
// (or in literals called where they stand), not assigning it or taking its address.
func plainRead(u *ast.Ident, d *ast.FuncDecl, why *string) bool {
	if home(u) != d {
		*why = "captured by a function literal at " + rw.Fset.Position(u.Pos()).String()
		return false
	}
	switch p := rw.Up(u).(type) {
	case *ast.AssignStmt:
		for _, l := range p.Lhs {
			if rw.Unparen(l) == ast.Expr(u) {
				*why = "assigned again at " + rw.Fset.Position(u.Pos()).String()
				return false
			}
		}
	case *ast.UnaryExpr:
		if p.Op == token.AND {
			*why = "address taken at " + rw.Fset.Position(u.Pos()).String()
			return false
		}
	case *ast.IncDecStmt, *ast.RangeStmt:
		*why = "assigned again at " + rw.Fset.Position(u.Pos()).String()
		return false
	}
	return true
}

type ownInfo struct {
	ok     bool
	builds []*ast.CallExpr // the last builds: return v.buildT_()
}

var (
	owned   = map[types.Object]*ownInfo{}
	defInit = map[types.Object]ast.Expr{}
)

// own examines the local variable v, declared in fd.
func own(v types.Object, fd *ast.FuncDecl, why *string) *ownInfo {
	if o, ok := owned[v]; ok {
		return o
	}
	o := &ownInfo{}
	owned[v] = o // false while under examination
	init := defInit[v]
	if init == nil || !fresh(init, why) {
		if *why == "" {
			*why = "not declared with a fresh builder"
		}
		return o
	}
	for _, u := range rw.UsesOf[v] {
		if !plainRead(u, fd, why) {
			return o
		}
		if use(u, fd, &o.builds, why) == uBad {
			o.builds = nil
			return o
		}
	}
	o.ok = true
	return o
}

var freshFn = map[*ast.FuncDecl]int{} // 1 under examination or no, 2 yes

// fresh reports whether e evaluates to a builder nothing else refers to.
func fresh(e ast.Expr, why *string) bool {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	var id *ast.Ident
	switch f := rw.Unparen(c.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
		fn, _ := rw.Info.Uses[id].(*types.Func)
		if fn != nil && genMethod(fn) == mChain && isBuilder(rw.Info.TypeOf(f.X)) {
			return fresh(f.X, why)
		}
	}
	if id == nil {
		return false
	}
	fn, _ := rw.Info.Uses[id].(*types.Func)
	d := decls[fn]
	if fn == nil || d == nil || d.Body == nil || !isBuilder(rw.Info.TypeOf(c)) {
		return false
	}
	if d.Recv == nil && len(c.Args) == 0 && strings.HasSuffix(d.Name.Name, "_newBuilder") && fileOf(d) == "rt_proto_gen.go" {
		return true
	}
	// a function of the program returning a fresh builder
	if st := freshFn[d]; st != 0 {
		return st == 2
	}
	freshFn[d] = 1
	good, any := true, false
	ast.Inspect(d.Body, func(n ast.Node) bool {
		if _, ok := n.(*ast.FuncLit); ok {
			return false
		}
		r, ok := n.(*ast.ReturnStmt)
		if !ok || !good {
			return good
		}
		any = true
		if len(r.Results) != 1 {
			good = false
			return false
		}
		res := rw.Unparen(r.Results[0])
		if rid, ok := res.(*ast.Ident); ok {
			v, _ := rw.Info.Uses[rid].(*types.Var)
			if v == nil || v.IsField() || v.Parent() == rw.Pkg.Scope() || defInit[v] == nil || !own(v, d, why).ok {
				good = false
			}
			return false
		}
		if !fresh(res, why) {
			good = false
		}
		return false
	})
	if good && any {
		freshFn[d] = 2
		return true
	}
	return false
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				if fn, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					decls[fn] = fd
					byName[fd.Name.Name] = append(byName[fd.Name.Name], fd)
				}
			}
		}
	}
	for id, o := range rw.Info.Defs {
		v, ok := o.(*types.Var)
		if !ok || v.IsField() || v.Parent() == rw.Pkg.Scope() {
			continue
		}
		switch p := rw.Parent[id].(type) {
		case *ast.ValueSpec:
			if len(p.Names) == 1 && len(p.Values) == 1 {
				defInit[v] = p.Values[0]
			}
		case *ast.AssignStmt:
			if p.Tok == token.DEFINE && len(p.Lhs) == 1 && len(p.Rhs) == 1 {
				defInit[v] = p.Rhs[0]
			}
		}
	}
	nA, nB, left, already := 0, 0, 0, 0
	done := map[*ast.CallExpr]bool{}
	rewrite := func(c *ast.CallExpr) {
		s := c.Fun.(*ast.SelectorExpr)
		if !rw.HasMethod(rw.Info.TypeOf(s.X), "buildLastT_") {
			fmt.Fprintf(os.Stderr, "pblast: %s has no buildLastT_ (rt_proto_build.go)\n", rw.Info.TypeOf(s.X))
			os.Exit(1)
		}
		rw.Rewrite[s.Sel] = func() string { return "buildLastT_" }
		done[c] = true
	}
	for _, f := range rw.Files {
		if !strings.HasPrefix(fileOf(f), "g_") {
			continue
		}
		ast.Inspect(f, func(nd ast.Node) bool {
			c, ok := nd.(*ast.CallExpr)
			if !ok || len(c.Args) != 0 {
				return true
			}
			s, ok := c.Fun.(*ast.SelectorExpr)
			if !ok || !isBuilder(rw.Info.TypeOf(s.X)) {
				return true
			}
			if s.Sel.Name == "buildLastT_" {
				already++
				return true
			}
			if s.Sel.Name != "buildT_" {
				return true
			}
			why := ""
			// A: a fresh receiver
			if fresh(s.X, &why) {
				rewrite(c)
				nA++
				return true
			}
			// B: return v[.chain()].buildT_() of the function declaring the owned local v
			x := rw.Unparen(s.X)
			for {
				cc, ok := x.(*ast.CallExpr)
				if !ok {
					break
				}
				cs, ok := rw.Unparen(cc.Fun).(*ast.SelectorExpr)
				if !ok {
					break
				}
				fn, _ := rw.Info.Uses[cs.Sel].(*types.Func)
				if fn == nil || genMethod(fn) != mChain {
					break
				}
				x = rw.Unparen(cs.X)
			}
			if id, ok := x.(*ast.Ident); ok {
				v, _ := rw.Info.Uses[id].(*types.Var)
				fd, _ := enclosing(c).(*ast.FuncDecl)
				if v != nil && fd != nil && !v.IsField() && v.Parent() != rw.Pkg.Scope() && defInit[v] != nil {
					if o := own(v, fd, &why); o.ok {
						for _, b := range o.builds {
							if b == c {
								rewrite(c)
								nB++
								return true
							}
						}
						why = "not the result of a return of the declaring function"
					}
				} else if why == "" {
					why = "receiver is not a local variable declared with a value"
				}
			} else if why == "" {
				why = "receiver is neither fresh nor a local variable"
			}
			left++
			if verbose {
				fmt.Fprintf(os.Stderr, "        left: %s: %s\n", rw.Fset.Position(c.Pos()), why)
			}
			return true
		})
	}
	nm := 0
	for _, k := range methodKind {
		if k != mNo {
			nm++
		}
	}
	nb := 0
	for _, ok := range borrowed {
		if ok {
			nb++
		}
	}
	fmt.Fprintf(os.Stderr, "pblast: %d last builds of a protobuf builder are buildLastT_(): %d of a fresh builder expression, %d of an owned local variable in a return (%d generated builder methods checked to keep no reference, %d parameters borrowed); left %d buildT_() calls; %d already buildLastT_()\n",
		nA+nB, nA, nB, nm, nb, left, already)
	if report || nA+nB == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "pblast: files changed: %d\n", rw.SaveChecked())
}
