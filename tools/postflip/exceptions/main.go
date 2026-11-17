// Command exceptions is a post-flip step: the try/catch emulation and the unreachable code j2go
// writes around Java's control flow are simplified where they cannot make a difference.
//
// 1. Unreachable statements. j2go ends a function whose Java body cannot complete normally
// with panic("unreachable") (Go requires a terminating statement where Java's own flow
// analysis knew), and writes `return 0` after the `return 1` of a try block and `return`
// after the throw of a catch block. A statement that follows a terminating statement (Go
// spec: return, a panic call, a block / if-else / for without condition or break / switch or
// select with default whose clauses all end in one) in the same list never runs; it is
// deleted. The function still ends in a terminating statement, which the type check of the
// rewritten package confirms.
//
// 2. Catch clauses that cannot catch anything. A Java try/catch is emulated with
//
//	_ = func() (code_ int) {
//		defer func() {
//			if r_ := recover(); r_ != nil {
//				ex_ := catch_(r_)
//				if is_IOException(ex_) { ...; return }
//				panic(r_)
//			}
//		}()
//		<try block>
//		return 0
//	}()
//
// A clause is dead when no exception the try block can throw is an instance of its type: the
// try block's exceptions are the classes it can throw (transitively, over a call graph of
// static calls, interface calls to every method of that name and signature on a type of the
// package that implements the interface, and calls of function values to every function
// literal, function or method value of that signature), plus the exceptions any Go code can
// raise (catch_: NullPointerException, ArrayIndexOutOfBoundsException, ClassCastException,
// ArithmeticException, RuntimeException). A throw is panic(x): of a new object of class C,
// C; of a value of static type *C, C and its subclasses; of an interface value, anything; a
// rethrow panic(r_) in a catch block, nothing new (what the try block threw). A dead clause is
// deleted. When no clause and no finally block is left, the defer is deleted, and when the try
// block then returns only its final `return 0`, the function literal is a plain block.
//
// usage: exceptions [-n] <dir of package closure>
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

var count = map[string]int{}

var verbose = os.Getenv("EXCEPTIONS_VERBOSE") != ""

func note(what string, n ast.Node) {
	count[what]++
	if verbose {
		fmt.Fprintf(os.Stderr, "%s: %s\n", rw.Fset.Position(n.Pos()), what)
	}
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])

	tries()
	unreachable()
	census()

	var keys []string
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%5d %s\n", count[k], k)
	}
	if !report {
		fmt.Printf("exceptions: %d files changed\n", rw.SaveChecked())
	}
}

// ---- unreachable statements

// isPanic reports whether s is a call of the builtin panic.
func isPanic(s ast.Stmt) bool {
	es, ok := s.(*ast.ExprStmt)
	if !ok {
		return false
	}
	c, ok := es.X.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := c.Fun.(*ast.Ident)
	if !ok {
		return false
	}
	_, builtin := rw.Info.Uses[id].(*types.Builtin)
	return builtin && id.Name == "panic"
}

// terminating is the Go specification's terminating statement.
func terminating(s ast.Stmt) bool {
	switch x := s.(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return x.Tok == token.GOTO
	case *ast.ExprStmt:
		return isPanic(x)
	case *ast.BlockStmt:
		return listTerminating(x.List)
	case *ast.IfStmt:
		if x.Else == nil {
			return false
		}
		return listTerminating(x.Body.List) && terminating(x.Else)
	case *ast.ForStmt:
		return x.Cond == nil && !hasBreak(x.Body, nil)
	case *ast.LabeledStmt:
		if f, ok := x.Stmt.(*ast.ForStmt); ok {
			return f.Cond == nil && !hasBreak(f.Body, x.Label)
		}
		return terminating(x.Stmt)
	case *ast.SwitchStmt:
		return clausesTerminating(x.Body, nil)
	case *ast.TypeSwitchStmt:
		return clausesTerminating(x.Body, nil)
	case *ast.SelectStmt:
		return clausesTerminating(x.Body, nil)
	}
	return false
}

func listTerminating(l []ast.Stmt) bool {
	return len(l) > 0 && terminating(l[len(l)-1])
}

func clausesTerminating(body *ast.BlockStmt, label *ast.Ident) bool {
	def := false
	for _, c := range body.List {
		var l []ast.Stmt
		switch cc := c.(type) {
		case *ast.CaseClause:
			l = cc.Body
			def = def || cc.List == nil
		case *ast.CommClause:
			l = cc.Body
			def = def || cc.Comm == nil
		}
		ends := listTerminating(l)
		if len(l) > 0 {
			if b, ok := l[len(l)-1].(*ast.BranchStmt); ok && b.Tok == token.FALLTHROUGH {
				ends = true
			}
		}
		if !ends || hasBreak(&ast.BlockStmt{List: l}, label) {
			return false
		}
	}
	if _, isSelect := rw.Parent[body].(*ast.SelectStmt); isSelect {
		return true
	}
	return def
}

// hasBreak reports whether n has a break that leaves the statement around n: an unlabeled
// break not inside a nested for, switch or select, or a break of label.
func hasBreak(n ast.Node, label *ast.Ident) bool {
	found := false
	var walk func(n ast.Node, nested bool)
	walk = func(n ast.Node, nested bool) {
		ast.Inspect(n, func(m ast.Node) bool {
			if found {
				return false
			}
			switch x := m.(type) {
			case *ast.FuncLit:
				return false
			case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				if m != n {
					walk(x, true)
					return false
				}
			case *ast.BranchStmt:
				if x.Tok == token.BREAK && ((x.Label == nil && !nested) || (x.Label != nil && label != nil && x.Label.Name == label.Name)) {
					found = true
				}
			}
			return true
		})
	}
	walk(n, false)
	return found
}

// unreachable deletes the statements after a terminating statement of the same list.
func unreachable() {
	for _, f := range rw.Files {
		if !generated(f) {
			continue // the handwritten runtime (rt_*.go) is read, not rewritten
		}
		ast.Inspect(f, func(n ast.Node) bool {
			var list []ast.Stmt
			switch x := n.(type) {
			case *ast.BlockStmt:
				list = x.List
			case *ast.CaseClause:
				list = x.Body
			case *ast.CommClause:
				list = x.Body
			default:
				return true
			}
			for i := 1; i < len(list); i++ {
				if !terminating(list[i-1]) {
					continue
				}
				for _, s := range list[i:] {
					if _, ok := rw.Rewrite[s]; ok {
						continue
					}
					if hasLabel(s) {
						count["left: unreachable statement with a label (a goto target)"]++
						continue
					}
					switch {
					case isPanic(s) && strings.Contains(rw.Text(s), `panic("unreachable")`):
						note(`unreachable: panic("unreachable") deleted`, s)
					case isReturn(s):
						note("unreachable: return deleted", s)
					default:
						note("unreachable: other statement deleted", s)
					}
					rw.Rewrite[s] = func() string { return rw.Del }
				}
				break
			}
			return true
		})
	}
}

func isReturn(s ast.Stmt) bool { _, ok := s.(*ast.ReturnStmt); return ok }

func hasLabel(s ast.Stmt) bool {
	found := false
	ast.Inspect(s, func(n ast.Node) bool {
		if _, ok := n.(*ast.LabeledStmt); ok {
			found = true
		}
		return !found
	})
	return found
}

// ---- try/catch

// try is one emulated try statement.
type try struct {
	lit     *ast.FuncLit     // func() (code_ int) { ... }
	stmt    ast.Stmt         // the statement calling it
	defers  []*ast.DeferStmt // finally (a defer without recover) and catch defers
	catch   *ast.DeferStmt
	clauses []*ast.IfStmt // if is_X(ex_) || ... { ... }
	body    []ast.Stmt    // the try block
}

// findTry recognises the catch defer of j2go's try emulation.
func findTry(d *ast.DeferStmt) (*ast.IfStmt, bool) {
	fl, ok := d.Call.Fun.(*ast.FuncLit)
	if !ok || len(fl.Body.List) != 1 {
		return nil, false
	}
	is, ok := fl.Body.List[0].(*ast.IfStmt)
	if !ok || is.Init == nil || is.Else != nil || !strings.HasPrefix(rw.Text(is.Init), "r_ := recover()") || rw.Text(is.Cond) != "r_ != nil" {
		return nil, false
	}
	l := is.Body.List
	if len(l) < 2 || rw.Text(l[0]) != "ex_ := catch_(r_)" || rw.Text(l[len(l)-1]) != "panic(r_)" {
		return nil, false
	}
	return is, true
}

// assertClauseType is the class of a catch clause that castfold -instof wrote as a comma-ok
// assertion: `if e, ok := ex_.(*X); ok {` or `if p, ok := ex_.(X_as); ok {`.
func assertClauseType(ci *ast.IfStmt) ([]string, bool) {
	as, ok := ci.Init.(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 2 || len(as.Rhs) != 1 {
		return nil, false
	}
	ta, ok := as.Rhs[0].(*ast.TypeAssertExpr)
	if !ok || ta.Type == nil || rw.Text(ta.X) != "ex_" || rw.Text(ci.Cond) != rw.Text(as.Lhs[1]) {
		return nil, false
	}
	t := rw.Text(ta.Type)
	switch {
	case strings.HasPrefix(t, "*"):
		return []string{t[1:]}, true
	case strings.HasSuffix(t, "_as"):
		return []string{strings.TrimSuffix(t, "_as")}, true
	}
	return nil, false
}

// clauseTypes are the classes a catch clause tests: is_X(ex_) or is_[*X](ex_), joined by ||.
func clauseTypes(e ast.Expr) ([]string, bool) {
	e = rw.Unparen(e)
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.LOR {
		l, ok1 := clauseTypes(b.X)
		r, ok2 := clauseTypes(b.Y)
		return append(l, r...), ok1 && ok2
	}
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 1 || rw.Text(c.Args[0]) != "ex_" {
		return nil, false
	}
	name, _ := rw.CallName(c)
	switch {
	case strings.HasPrefix(name, "is_[*"):
		return []string{strings.TrimSuffix(strings.TrimPrefix(name, "is_[*"), "]")}, true
	case strings.HasPrefix(name, "is_"):
		return []string{strings.TrimPrefix(name, "is_")}, true
	}
	return nil, false
}

func tries() {
	buildGraph()
	for _, f := range rw.Files {
		if !generated(f) {
			continue // the handwritten runtime (rt_*.go) is read, not rewritten
		}
		ast.Inspect(f, func(n ast.Node) bool {
			d, ok := n.(*ast.DeferStmt)
			if !ok {
				return true
			}
			is, ok := findTry(d)
			if !ok {
				return true
			}
			lit, ok := rw.Parent[rw.Parent[d]].(*ast.FuncLit)
			if !ok {
				note("left: catch defer not in a function literal", d)
				return true
			}
			analyse(lit, d, is)
			return true
		})
	}
}

// implicit are the exceptions catch_ makes of Go runtime panics, which any code can raise.
var implicit = []string{"NullPointerException", "ArrayIndexOutOfBoundsException", "ClassCastException", "ArithmeticException", "RuntimeException"}

func analyse(lit *ast.FuncLit, d *ast.DeferStmt, is *ast.IfStmt) {
	// The try block: the statements of the literal after its defers.
	var body []ast.Stmt
	idx := -1
	for i, s := range lit.Body.List {
		if s == ast.Stmt(d) {
			idx = i
		}
	}
	if idx < 0 {
		note("left: catch defer not at the top of its function literal", d)
		return
	}
	finally := false
	for _, s := range lit.Body.List[:idx] {
		if _, ok := s.(*ast.DeferStmt); ok {
			finally = true
		}
	}
	body = lit.Body.List[idx+1:]
	set := throwsOf(body)
	clauses := is.Body.List[1 : len(is.Body.List)-1]
	live := 0
	var dead []*ast.IfStmt
	for _, c := range clauses {
		ci, ok := c.(*ast.IfStmt)
		if !ok || ci.Else != nil {
			note("left: catch block of an unknown shape", c)
			return
		}
		var ts []string
		if ci.Init != nil {
			ts, ok = assertClauseType(ci)
		} else {
			ts, ok = clauseTypes(ci.Cond)
		}
		if !ok {
			note("left: catch block of an unknown shape", c)
			return
		}
		var keep []string
		for _, t := range ts {
			if set.top || set.mayBe(t) {
				keep = append(keep, t)
			}
		}
		switch {
		case len(keep) == len(ts):
			live++
			note("catch clause kept: the try block can throw "+strings.Join(ts, " | ")+" ("+set.witness(ts)+")", ci)
		case len(keep) == 0:
			dead = append(dead, ci)
			note("catch clause deleted: the try block cannot throw "+strings.Join(ts, " | "), ci)
		default:
			live++
			note("catch clause narrowed: the try block cannot throw some of "+strings.Join(ts, " | "), ci)
			cond := ci.Cond
			k := keep
			rw.Rewrite[cond] = func() string {
				var parts []string
				for _, t := range k {
					parts = append(parts, "is_"+t+"(ex_)")
				}
				return strings.Join(parts, " || ")
			}
		}
	}
	for _, ci := range dead {
		rw.Rewrite[ci] = func() string { return rw.Del }
	}
	if live > 0 || len(dead) == 0 {
		return
	}
	// No clause is left: the try statement only runs its block.
	rw.Rewrite[d] = func() string { return rw.Del }
	note("try emulation without catch clauses: defer deleted", d)
	if finally {
		return
	}
	// _ = func() (code_ int) { <block>; return 0 }() -> { <block> }
	as, ok := rw.Up(rw.Parent[lit]).(*ast.AssignStmt)
	call, _ := rw.Parent[lit].(*ast.CallExpr)
	if !ok || call == nil || len(as.Lhs) != 1 || rw.Text(as.Lhs[0]) != "_" || len(body) == 0 || rw.Text(body[len(body)-1]) != "return 0" {
		return
	}
	returns := 0
	for _, s := range body {
		ast.Inspect(s, func(n ast.Node) bool {
			switch n.(type) {
			case *ast.FuncLit:
				return false
			case *ast.ReturnStmt:
				returns++
			}
			return true
		})
	}
	if returns != 1 || usesName(body, "code_") {
		return
	}
	note("try emulation without catch clauses: a block", lit)
	rw.Rewrite[as] = func() string {
		var out []string
		for _, s := range body[:len(body)-1] {
			out = append(out, rw.Render(s))
		}
		return "{\n" + strings.Join(out, "\n") + "\n}"
	}
}

func usesName(l []ast.Stmt, name string) bool {
	found := false
	for _, s := range l {
		ast.Inspect(s, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok && id.Name == name {
				found = true
			}
			return !found
		})
	}
	return found
}

// ---- what code can throw

// exset is a set of exception classes (by name) that some code can throw; top means any.
type exset struct {
	top     bool
	topWhy  string
	via     map[string]*node  // class -> the callee it came from
	classes map[string]string // class -> a function that throws it (witness)
}

func (s *exset) add(c, where string) bool {
	if s.classes == nil {
		s.classes = map[string]string{}
	}
	if _, ok := s.classes[c]; ok {
		return false
	}
	s.classes[c] = where
	return true
}

func (s *exset) union(o *exset, from *node) bool {
	changed := false
	if o.top && !s.top {
		s.top = true
		s.topWhy = o.topWhy
		changed = true
	}
	for c, w := range o.classes {
		if s.add(c, w) {
			if s.via == nil {
				s.via = map[string]*node{}
			}
			s.via[c] = from
			changed = true
		}
	}
	return changed
}

// chain is the call path by which class c reaches the set.
func (s *exset) chain(c string) string {
	var out []string
	seen := map[*node]bool{}
	for n := s.via[c]; n != nil && !seen[n] && len(out) < 12; n = n.throws.via[c] {
		seen[n] = true
		out = append(out, n.name)
	}
	return strings.Join(out, " -> ")
}

// mayBe reports whether a class of the set is t or a subclass of t.
func (s *exset) mayBe(t string) bool {
	for c := range s.classes {
		if subclass(c, t) {
			return true
		}
	}
	return false
}

func (s *exset) witness(ts []string) string {
	if s.top {
		return "it can throw a value of unknown class: " + s.topWhy
	}
	var ws []string
	for c, w := range s.classes {
		for _, t := range ts {
			if subclass(c, t) {
				ws = append(ws, c+" from "+w+" via "+s.chain(c))
			}
		}
	}
	sort.Strings(ws)
	if len(ws) > 2 {
		ws = append(ws[:2], "...")
	}
	return strings.Join(ws, ", ")
}

// subclass reports whether class c is t or extends it (*c has as_t).
func subclass(c, t string) bool {
	if c == t {
		return true
	}
	o, ok := rw.Pkg.Scope().Lookup(c).(*types.TypeName)
	if !ok {
		return false
	}
	m, _, _ := types.LookupFieldOrMethod(types.NewPointer(o.Type()), true, rw.Pkg, "as_"+t)
	_, ok = m.(*types.Func)
	return ok
}

// exception classes: the named types whose pointer has as_Throwable.
var classes []string

func subclassesOf(t string) []string {
	var out []string
	for _, c := range classes {
		if subclass(c, t) {
			out = append(out, c)
		}
	}
	return out
}

// node is a function of the call graph: a declaration or a literal.
type node struct {
	name    string
	body    *ast.BlockStmt
	throws  exset
	calls   []*node
	callers []*node
}

var (
	nodes    = map[ast.Node]*node{} // *ast.FuncDecl / *ast.FuncLit -> node
	funcDecl = map[*types.Func]*ast.FuncDecl{}
	bySig    = map[string][]*node{}       // signature -> functions used as values
	methods  = map[string][]*types.Func{} // method name -> methods of the package's types
)

func sigKey(t types.Type) string {
	s, ok := t.Underlying().(*types.Signature)
	if !ok {
		return ""
	}
	return types.TypeString(types.NewSignatureType(nil, nil, nil, s.Params(), s.Results(), s.Variadic()), nil)
}

func buildGraph() {
	sc := rw.Pkg.Scope()
	for _, n := range sc.Names() {
		tn, ok := sc.Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		nt, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		if m, _, _ := types.LookupFieldOrMethod(types.NewPointer(nt), true, rw.Pkg, "as_Throwable"); m != nil {
			if _, isIface := nt.Underlying().(*types.Interface); !isIface {
				classes = append(classes, n)
			}
		}
		for i := 0; i < nt.NumMethods(); i++ {
			m := nt.Method(i)
			methods[m.Name()] = append(methods[m.Name()], m)
		}
	}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			o := rw.Info.Defs[fd.Name].(*types.Func)
			funcDecl[o] = fd
			nodes[fd] = &node{name: fd.Name.Name, body: fd.Body}
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if fl, ok := n.(*ast.FuncLit); ok {
				nodes[fl] = &node{name: "func literal at " + rw.Fset.Position(fl.Pos()).String(), body: fl.Body}
				if c, ok := rw.Up(fl).(*ast.CallExpr); !ok || rw.Unparen(c.Fun) != fl {
					k := sigKey(rw.Info.Types[fl].Type)
					bySig[k] = append(bySig[k], nodes[fl])
				}
			}
			return true
		})
	}
	// Functions and methods used as values.
	for id, o := range rw.Info.Uses {
		f, ok := o.(*types.Func)
		if !ok || funcDecl[f] == nil {
			continue
		}
		var e ast.Expr = id
		if s, ok := rw.Parent[id].(*ast.SelectorExpr); ok && s.Sel == id {
			e = s
		}
		if c, ok := rw.Up(e).(*ast.CallExpr); ok && rw.Unparen(c.Fun) == e {
			continue
		}
		if ix, ok := rw.Parent[e].(*ast.IndexExpr); ok {
			if c, ok := rw.Up(ix).(*ast.CallExpr); ok && rw.Unparen(c.Fun) == ix {
				continue
			}
		}
		k := sigKey(f.Type())
		n := nodes[funcDecl[f]]
		bySig[k] = append(bySig[k], n)
	}
	for k := range bySig {
		sort.Slice(bySig[k], func(i, j int) bool { return bySig[k][i].body.Pos() < bySig[k][j].body.Pos() })
	}
	// Edges and own throws.
	for _, n := range sortedNodes() {
		scan(n)
	}
	// Fixpoint: a function throws what its callees throw.
	work := sortedNodes()
	for len(work) > 0 {
		n := work[len(work)-1]
		work = work[:len(work)-1]
		for _, c := range n.callers {
			if c.throws.union(&n.throws, n) {
				work = append(work, c)
			}
		}
	}
}

func sortedNodes() []*node {
	var out []*node
	for _, n := range nodes {
		out = append(out, n)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].body.Pos() < out[j].body.Pos() })
	return out
}

func edge(from, to *node) {
	if to == nil {
		return
	}
	from.calls = append(from.calls, to)
	to.callers = append(to.callers, from)
}

// implementers are the methods named m that an interface call of iface can reach.
var implCache = map[string][]*node{}

func implementers(iface *types.Interface, m string) []*node {
	key := types.TypeString(iface, nil) + "." + m
	if ns, ok := implCache[key]; ok {
		return ns
	}
	var out []*node
	for _, f := range methods[m] {
		recv := f.Type().(*types.Signature).Recv().Type()
		base := recv
		if p, ok := recv.(*types.Pointer); ok {
			base = p.Elem()
		}
		if types.Implements(recv, iface) || types.Implements(types.NewPointer(base), iface) {
			if fd := funcDecl[f]; fd != nil {
				out = append(out, nodes[fd])
			}
		}
	}
	implCache[key] = out
	return out
}

// scan records the calls and throws of n's own body (not of the literals in it).
func scan(n *node) {
	where := n.name
	var walk func(x ast.Node) bool
	walk = func(x ast.Node) bool {
		switch v := x.(type) {
		case *ast.FuncLit:
			if v.Body != n.body {
				return false
			}
		case *ast.CallExpr:
			call(n, v, where)
		}
		return true
	}
	ast.Inspect(n.body, walk)
}

func call(n *node, c *ast.CallExpr, where string) {
	fun := rw.Unparen(c.Fun)
	if tv, ok := rw.Info.Types[fun]; ok && tv.IsType() {
		return // a conversion
	}
	switch f := fun.(type) {
	case *ast.FuncLit:
		edge(n, nodes[f])
		return
	case *ast.Ident:
		switch o := rw.Info.Uses[f].(type) {
		case *types.Builtin:
			if o.Name() == "panic" && len(c.Args) == 1 {
				throw(n, c.Args[0], where)
			}
			return
		case *types.Func:
			if fd := funcDecl[o]; fd != nil {
				edge(n, nodes[fd])
			}
			return
		}
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			if o, ok := rw.Info.Uses[id].(*types.Func); ok {
				if fd := funcDecl[o]; fd != nil {
					edge(n, nodes[fd])
				}
				return
			}
		}
	case *ast.IndexListExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			if o, ok := rw.Info.Uses[id].(*types.Func); ok {
				if fd := funcDecl[o]; fd != nil {
					edge(n, nodes[fd])
				}
				return
			}
		}
	case *ast.SelectorExpr:
		if sel := rw.Info.Selections[f]; sel != nil && sel.Kind() == types.MethodVal {
			m := sel.Obj().(*types.Func)
			if iface, ok := sel.Recv().Underlying().(*types.Interface); ok {
				for _, t := range implementers(iface, m.Name()) {
					edge(n, t)
				}
				// Methods of interfaces of other packages (reflect.Type, error) that no type of
				// the package implements raise only Go runtime panics (implicit).
				return
			}
			if fd := funcDecl[m.Origin()]; fd != nil {
				edge(n, nodes[fd])
			}
			return
		}
		if o, ok := rw.Info.Uses[f.Sel].(*types.Func); ok {
			if fd := funcDecl[o]; fd != nil {
				edge(n, nodes[fd])
			}
			return // a function of another package: only Go runtime panics (implicit)
		}
	}
	// A call of a function value: every function of that signature used as a value.
	t := rw.Info.Types[fun].Type
	if t == nil {
		n.throws.top, n.throws.topWhy = true, where+": call of "+rw.Text(fun)
		return
	}
	for _, to := range bySig[sigKey(t)] {
		edge(n, to)
	}
}

// throw records the classes panic(x) can throw.
func throw(n *node, x ast.Expr, where string) {
	if rethrow(x) {
		return // a rethrow of what the function around the deferred call threw
	}
	cs, ok := classesOf(x, 0)
	if ok {
		cs = guarded(x, cs)
	}
	if !ok {
		n.throws.top, n.throws.topWhy = true, where+": panic("+rw.Text(x)+")"
		return
	}
	for _, c := range cs {
		n.throws.add(c, where)
	}
}

// guarded narrows the classes of a thrown variable to those of the is_X tests on it of the if
// statements around the throw: if is_A(s) || is_B(s) { panic(s) }.
func guarded(x ast.Expr, cs []string) []string {
	id, ok := rw.Unparen(x).(*ast.Ident)
	if !ok {
		return cs
	}
	for p := rw.Parent[x]; p != nil; p = rw.Parent[p] {
		if _, ok := p.(*ast.FuncLit); ok {
			break
		}
		is, ok := p.(*ast.IfStmt)
		if !ok {
			continue
		}
		ts, ok := testsOn(is.Cond, id.Name)
		if !ok || !contains(is.Body, x) {
			continue
		}
		var out []string
		for _, c := range cs {
			for _, t := range ts {
				if subclass(c, t) {
					out = append(out, c)
					break
				}
			}
		}
		cs = out
	}
	return cs
}

func contains(n ast.Node, x ast.Node) bool { return n.Pos() <= x.Pos() && x.End() <= n.End() }

// testsOn are the classes of is_X(v) || ... (or the type-assertion helpers) on variable v.
func testsOn(e ast.Expr, v string) ([]string, bool) {
	e = rw.Unparen(e)
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.LOR {
		l, ok1 := testsOn(b.X, v)
		r, ok2 := testsOn(b.Y, v)
		return append(l, r...), ok1 && ok2
	}
	c, ok := e.(*ast.CallExpr)
	if !ok || len(c.Args) != 1 || rw.Text(c.Args[0]) != v {
		return nil, false
	}
	name, _ := rw.CallName(c)
	if strings.HasPrefix(name, "is_") && !strings.HasPrefix(name, "is_[") {
		return []string{strings.TrimPrefix(name, "is_")}, true
	}
	return nil, false
}

// rethrow reports whether x is the recovered value, or the exception catch_ / caught_ made of
// it: rethrowing it raises nothing that the function did not throw already.
func rethrow(x ast.Expr) bool {
	id, ok := rw.Unparen(x).(*ast.Ident)
	if !ok {
		return false
	}
	v, ok := rw.Info.Uses[id].(*types.Var)
	if !ok {
		return false
	}
	rhs := singleValue(v)
	if rhs == nil {
		return false
	}
	if rw.Text(rhs) == "recover()" {
		return true
	}
	name, c := rw.CallName(rhs)
	return (name == "catch_" || name == "caught_") && c != nil && len(c.Args) == 1 && rethrow(c.Args[0])
}

var defValue map[types.Object]ast.Expr

// singleValue is the value of local variable v when it is defined by v := e and never
// assigned again.
func singleValue(v *types.Var) ast.Expr {
	if defValue == nil {
		defValue = map[types.Object]ast.Expr{}
		for d, o := range rw.Info.Defs {
			if as, ok := rw.Parent[d].(*ast.AssignStmt); ok && as.Tok == token.DEFINE && len(as.Lhs) == 1 && len(as.Rhs) == 1 && as.Lhs[0] == ast.Expr(d) {
				defValue[o] = as.Rhs[0]
			}
		}
	}
	e := defValue[v]
	if e == nil {
		return nil
	}
	for _, u := range rw.UsesOf[v] {
		if as, ok := rw.Parent[u].(*ast.AssignStmt); ok {
			for _, l := range as.Lhs {
				if l == ast.Expr(u) {
					return nil
				}
			}
		}
		if un, ok := rw.Parent[u].(*ast.UnaryExpr); ok && un.Op == token.AND {
			return nil
		}
	}
	return e
}

// classesOf is the classes of the exception objects expression x can be: a new object of class
// C (through obj_, any, self(), up_X() / as_X(), which keep the object), a value of static type
// *C (C and its subclasses), a string (catch_ makes it a RuntimeException), or the result of a
// package function all of whose returns are such expressions.
func classesOf(x ast.Expr, depth int) ([]string, bool) {
	x = rw.Unparen(x)
	for {
		c, ok := x.(*ast.CallExpr)
		if !ok {
			break
		}
		if id, ok := c.Fun.(*ast.Ident); ok && (id.Name == "obj_" || id.Name == "any") && len(c.Args) == 1 {
			x = rw.Unparen(c.Args[0])
			continue
		}
		if ix, ok := c.Fun.(*ast.IndexExpr); ok && rw.Text(ix.X) == "obj_" && len(c.Args) == 1 {
			x = rw.Unparen(c.Args[0])
			continue
		}
		if s, ok := c.Fun.(*ast.SelectorExpr); ok && len(c.Args) == 0 && (s.Sel.Name == "self" || strings.HasPrefix(s.Sel.Name, "up_") || strings.HasPrefix(s.Sel.Name, "as_")) {
			x = rw.Unparen(s.X)
			continue
		}
		break
	}
	if c, ok := x.(*ast.CallExpr); ok {
		if id, ok := c.Fun.(*ast.Ident); ok {
			if strings.HasPrefix(id.Name, "new_") {
				if t, ok := rw.Info.Types[c].Type.(*types.Pointer); ok {
					if nt, ok := t.Elem().(*types.Named); ok {
						return []string{nt.Obj().Name()}, true
					}
				}
			}
			if f, ok := rw.Info.Uses[id].(*types.Func); ok && funcDecl[f] != nil && depth < 3 {
				if sig := f.Type().(*types.Signature); sig.Results().Len() == 1 {
					if it, ok := sig.Results().At(0).Type().Underlying().(*types.Interface); ok && it.Empty() {
						return returnClasses(funcDecl[f], depth+1)
					}
				}
			}
		}
	}
	if id, ok := x.(*ast.Ident); ok && depth < 4 {
		if v, ok := rw.Info.Uses[id].(*types.Var); ok && !v.IsField() {
			if e := singleValue(v); e != nil {
				return classesOf(e, depth+1)
			}
		}
	}
	// x.self_: the object of x, of x's class or a subclass.
	if sel, ok := x.(*ast.SelectorExpr); ok && sel.Sel.Name == "self_" {
		x = sel.X
	}
	t := rw.Info.Types[x].Type
	if t == nil {
		return nil, false
	}
	if p, ok := t.(*types.Pointer); ok {
		if nt, ok := p.Elem().(*types.Named); ok {
			return subclassesOf(nt.Obj().Name()), true
		}
	}
	if b, ok := t.Underlying().(*types.Basic); ok && (b.Kind() == types.String || b.Kind() == types.UntypedString) {
		return []string{"RuntimeException"}, true
	}
	return nil, false
}

func returnClasses(fd *ast.FuncDecl, depth int) ([]string, bool) {
	var out []string
	ok := true
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch r := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			if len(r.Results) != 1 {
				ok = false
				return false
			}
			cs, good := classesOf(r.Results[0], depth)
			if !good {
				ok = false
			}
			out = append(out, cs...)
		}
		return ok
	})
	return out, ok
}

// throwsOf is what the statements of a try block can throw.
func throwsOf(body []ast.Stmt) *exset {
	n := &node{name: "the try block"}
	for _, s := range body {
		ast.Inspect(s, func(x ast.Node) bool {
			switch v := x.(type) {
			case *ast.FuncLit:
				if c, ok := rw.Up(v).(*ast.CallExpr); ok && rw.Unparen(c.Fun) == v {
					return true // called here
				}
				return false
			case *ast.CallExpr:
				call(n, v, "the try block")
			}
			return true
		})
	}
	set := &exset{}
	set.union(&n.throws, nil)
	for _, c := range n.calls {
		set.union(&c.throws, c)
	}
	for _, c := range implicit {
		set.add(c, "Go runtime errors")
	}
	return set
}

// census counts the panic calls that are left, by kind, for the step's report.
func census() {
	for _, f := range rw.Files {
		if !generated(f) {
			continue // the handwritten runtime (rt_*.go) is read, not rewritten
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if g, ok := rw.Rewrite[n]; ok && g() == rw.Del {
				return false
			}
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := c.Fun.(*ast.Ident)
			if !ok || id.Name != "panic" || len(c.Args) != 1 {
				return true
			}
			if _, ok := rw.Info.Uses[id].(*types.Builtin); !ok {
				return true
			}
			a := rw.Text(c.Args[0])
			kind := "throw of another value"
			switch {
			case a == "r_" || rethrow(c.Args[0]):
				kind = "rethrow of a caught exception not handled by the catch block"
			case a == `"unreachable"`:
				kind = `panic("unreachable") ending a function whose end Go cannot prove unreachable`
			case strings.HasPrefix(a, "enumValueOfFail_("):
				kind = "Enum.valueOf of an unknown name (IllegalArgumentException)"
			case strings.HasPrefix(a, "new_"):
				kind = "throw of a new exception (Java throw, or an inline Guava check)"
			case strings.HasPrefix(a, "obj_(new_"):
				kind = "throw of a new exception (Java throw, or an inline Guava check)"
			default:
				if cs, ok := classesOf(c.Args[0], 0); ok && len(cs) > 0 {
					kind = "throw of an exception object held in a variable or made by a helper"
				}
			}
			count["panic left: "+kind]++
			return true
		})
	}
}

// generated reports whether f is a file j2go generated (g_*.go).
func generated(f *ast.File) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_")
}
