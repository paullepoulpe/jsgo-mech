// Command strvar is a post-flip step: the variants of generated functions that take a key
// unboxed, written by rule (they were post-flip manual steps 29, 30, 35, 121 and 428).
//
// j2go passes a String where Java declares Object as a boxed string (strobj_(s)), and a value
// of a type parameter as boxT_(x). The runtime has helpers that take such a key as it is; this
// tool makes the generated code reach them.
//
// A. String variants of methods. For a method m of a generated class whose first parameter p
// is an Object (any), the variant
//
//	func (this *C) m_s(p string, ...) R      m's body, with every use of p rewritten:
//	    X.get(p), X.put(p, v), X.containsKey(p), X.remove(p)...   Map_get_s(X, p)...   X an erased Map
//	    X.contains(p), X.add(p), X.remove(p)                      Collection_contains_s(X, p)...  X an erased Set
//	    r.n(p, ...)                                               r.n_s(p, ...)   n a method of the class with a variant
//	    any other use of p                                        strobj_(p)
//
// computes m(strobj_(p)): the helpers are the methods called with the boxed key (strkeys'
// rule 2 and the runtime's contract for them), n_s is n with its key boxed (this rule, by
// induction), and every other use reads the box. A method whose body assigns p has no variant.
// The variant is written only where it is used and gains: some call in the package passes
// strobj_(e) as m's first argument on a receiver of the class (or a variant written here calls
// it), and some use of p in it goes to a helper or to another variant. The calls themselves are
// rewritten by strkeys (its rule 1: x.m(strobj_(s)) is x.m_s(s) when m_s exists).
//
// B. String variants of runtime interface methods (strFns): str_(x.intern(strobj_(s))) on an
// Interner is Interner_intern_s(x, s), when the runtime declares that function with that
// signature (its contract: in.intern(s) for a string).
//
// C. Typed twins of hash functions. A generated function f(k any) R whose body uses k only as
// the argument of Object_hashCode, called as f(boxT_(x)) with x of a type parameter's type,
// gets the twin
//
//	func fOf_[K any](k K) R       f's body with Object_hashCode(k) written hashCodeT_(k)
//
// and those calls become fOf_(x). hashCodeT_(v) is Object_hashCode(boxT_(v)) without making
// the box (rt_generic.go), so fOf_(x) is f(boxT_(x)).
//
// Each rule is inactive where its runtime functions do not exist yet, and a rerun changes
// nothing (an existing variant is not written again).
//
// -rule A, -rule B, -rule C run one rule alone (the steps that were manual patches at different
// places of the history each run theirs); without it all three run.
//
// usage: strvar [-n] [-rule A|B|C] <dir of package closure>
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

var mapHelpers = map[string]string{"get": "Map_get_s", "put": "Map_put_s", "containsKey": "Map_containsKey_s",
	"remove": "Map_remove_s", "getOrDefault": "Map_getOrDefault_s", "putIfAbsent": "Map_putIfAbsent_s"}
var setHelpers = map[string]string{"contains": "Collection_contains_s", "add": "Collection_add_s", "remove": "Collection_remove_s"}
var mapTypes = map[string]bool{"Map": true, "SortedMap": true, "NavigableMap": true, "LinkedHashMap": true, "HashMap": true, "TreeMap": true}
var setTypes = map[string]bool{"Set": true, "SortedSet": true, "NavigableSet": true, "LinkedHashSet": true, "HashSet": true, "TreeSet": true}

// strFns: runtime functions that are an interface method called with a string, by the
// interface and the method.
var strFns = map[[2]string]string{{"Interner", "intern"}: "Interner_intern_s"}

func generated(pos token.Pos) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(pos).Filename), "g_")
}

func at(n ast.Node) string {
	p := rw.Fset.Position(n.Pos())
	return fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
}

func hasFunc(name string) bool {
	_, ok := rw.Pkg.Scope().Lookup(name).(*types.Func)
	return ok
}

func typeName(t types.Type) string {
	if t == nil {
		return ""
	}
	t = types.Unalias(t)
	if p, ok := t.(*types.Pointer); ok {
		t = types.Unalias(p.Elem())
	}
	if n, ok := t.(*types.Named); ok && n.Obj().Pkg() == rw.Pkg {
		return n.Obj().Name()
	}
	return ""
}

func isAny(t types.Type) bool {
	i, ok := types.Unalias(t).(*types.Interface)
	return ok && i.Empty()
}

// classOf is the generated class a pointer type points to.
func classOf(t types.Type) *types.Named {
	if t == nil {
		return nil
	}
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok {
		return nil
	}
	if _, ok := n.Underlying().(*types.Struct); !ok {
		return nil
	}
	return n.Origin()
}

// method is a candidate of rule A.
type method struct {
	fd    *ast.FuncDecl
	fn    *types.Func
	cls   *types.Named
	p     *types.Var
	uses  []use
	bad   string
	gain  bool
	want  bool
	exist bool // m_s is declared already
}

type use struct {
	id     *ast.Ident
	call   *ast.CallExpr // the call rewritten, for a helper or a variant
	helper string        // the helper
	callee *method       // or the method whose variant is called
}

var methods = map[*types.Func]*method{}

func calleeMethod(c *ast.CallExpr) (*types.Func, *ast.SelectorExpr) {
	sel, ok := rw.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok {
		return nil, nil
	}
	sn := rw.Info.Selections[sel]
	if sn == nil || sn.Kind() != types.MethodVal {
		return nil, nil
	}
	fn, _ := sn.Obj().(*types.Func)
	if fn == nil {
		return nil, nil
	}
	return fn.Origin(), sel
}

func ruleA() (int, []string) {
	// candidates
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || fd.Body == nil || fd.Type.TypeParams != nil {
				continue
			}
			fn, _ := rw.Info.Defs[fd.Name].(*types.Func)
			if fn == nil || strings.HasSuffix(fd.Name.Name, "_s") {
				continue
			}
			sig := fn.Type().(*types.Signature)
			cls := classOf(sig.Recv().Type())
			if cls == nil || sig.Params().Len() == 0 || !isAny(sig.Params().At(0).Type()) {
				continue
			}
			if len(fd.Type.Params.List) == 0 || len(fd.Type.Params.List[0].Names) != 1 {
				continue
			}
			m := &method{fd: fd, fn: fn, cls: cls, p: sig.Params().At(0)}
			if o, _, _ := types.LookupFieldOrMethod(types.NewPointer(cls), true, rw.Pkg, fd.Name.Name+"_s"); o != nil {
				m.exist = true
			}
			methods[fn] = m
		}
	}
	// uses of the parameter
	for _, m := range methods {
		for _, id := range rw.UsesOf[m.p] {
			var e ast.Expr = id
			for {
				pe, ok := rw.Parent[e].(*ast.ParenExpr)
				if !ok {
					break
				}
				e = pe
			}
			u := use{id: id}
			switch p := rw.Parent[e].(type) {
			case *ast.AssignStmt:
				for _, l := range p.Lhs {
					if l == e {
						m.bad = "the parameter is assigned"
					}
				}
			case *ast.IncDecStmt:
				m.bad = "the parameter is assigned"
			case *ast.UnaryExpr:
				if p.Op == token.AND {
					m.bad = "the parameter's address is taken"
				}
			case *ast.CallExpr:
				if len(p.Args) > 0 && p.Args[0] == e {
					if fn, sel := calleeMethod(p); fn != nil {
						recv := typeName(rw.Info.TypeOf(sel.X))
						switch {
						case mapTypes[recv] && mapHelpers[sel.Sel.Name] != "" && hasFunc(mapHelpers[sel.Sel.Name]):
							u.call, u.helper = p, mapHelpers[sel.Sel.Name]
						case setTypes[recv] && setHelpers[sel.Sel.Name] != "" && hasFunc(setHelpers[sel.Sel.Name]):
							u.call, u.helper = p, setHelpers[sel.Sel.Name]
						default:
							if cm := methods[fn]; cm != nil && classOf(rw.Info.TypeOf(sel.X)) == cm.cls {
								u.call, u.callee = p, cm
							}
						}
					}
				}
			}
			m.uses = append(m.uses, u)
		}
	}
	// gain: greatest fixed point (a variant that only calls variants without gain has none)
	for _, m := range methods {
		m.gain = m.bad == ""
	}
	for changed := true; changed; {
		changed = false
		for _, m := range methods {
			if !m.gain {
				continue
			}
			g := false
			for _, u := range m.uses {
				g = g || u.helper != "" || u.callee != nil && (u.callee.gain || u.callee.exist)
			}
			if !g {
				m.gain = false
				changed = true
			}
		}
	}
	// wanted: called with a boxed string, or by a wanted variant
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) == 0 {
				return true
			}
			fn, sel := calleeMethod(c)
			m := methods[fn]
			if m == nil || classOf(rw.Info.TypeOf(sel.X)) != m.cls {
				return true
			}
			if name, _ := rw.CallName(c.Args[0]); name == "strobj_" {
				m.want = true
			}
			return true
		})
	}
	for changed := true; changed; {
		changed = false
		for _, m := range methods {
			if !m.want || !m.gain {
				continue
			}
			for _, u := range m.uses {
				if u.callee != nil && u.callee.gain && !u.callee.want {
					u.callee.want = true
					changed = true
				}
			}
		}
	}
	var list []*method
	for _, m := range methods {
		if m.want && m.gain && !m.exist {
			list = append(list, m)
		}
	}
	sort.Slice(list, func(i, j int) bool { return list[i].fd.Pos() < list[j].fd.Pos() })
	var names []string
	for _, m := range list {
		m := m
		names = append(names, m.cls.Obj().Name()+"."+m.fd.Name.Name+"_s")
		fd := m.fd
		rw.Rewrite[fd] = func() string {
			orig := rw.RenderChildren(fd)
			// the clone: rendered with the rewrites of its parameter in place, then removed
			var set []ast.Node
			put := func(n ast.Node, g func() string) {
				rw.Rewrite[n] = g
				set = append(set, n)
			}
			put(fd.Name, func() string { return fd.Name.Name + "_s" })
			put(fd.Type.Params.List[0].Type, func() string { return "string" })
			pname := fd.Type.Params.List[0].Names[0].Name
			for _, u := range m.uses {
				u := u
				switch {
				case u.helper != "":
					sel := rw.Unparen(u.call.Fun).(*ast.SelectorExpr)
					put(u.call, func() string {
						args := []string{rw.Render(sel.X), pname}
						for _, a := range u.call.Args[1:] {
							args = append(args, rw.Render(a))
						}
						return u.helper + "(" + strings.Join(args, ", ") + ")"
					})
				case u.callee != nil && (u.callee.exist || u.callee.want && u.callee.gain):
					sel := rw.Unparen(u.call.Fun).(*ast.SelectorExpr)
					put(sel.Sel, func() string { return sel.Sel.Name + "_s" })
				default:
					put(u.id, func() string { return "strobj_(" + pname + ")" })
				}
			}
			clone := rw.RenderChildren(fd)
			for _, n := range set {
				delete(rw.Rewrite, n)
			}
			return orig + "\n\n// " + fd.Name.Name + "_s is " + fd.Name.Name + " for a string key that is not boxed (post-flip step strvar).\n" + clone
		}
	}
	return len(list), names
}

func ruleB() int {
	n := 0
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			outer, ok := x.(*ast.CallExpr)
			if !ok || len(outer.Args) != 1 {
				return true
			}
			if name, _ := rw.CallName(outer); name != "str_" {
				return true
			}
			inner, ok := rw.Unparen(outer.Args[0]).(*ast.CallExpr)
			if !ok || len(inner.Args) != 1 {
				return true
			}
			fn, sel := calleeMethod(inner)
			if fn == nil {
				return true
			}
			helper := strFns[[2]string{typeName(rw.Info.TypeOf(sel.X)), sel.Sel.Name}]
			if helper == "" {
				return true
			}
			if _, isIface := types.Unalias(rw.Info.TypeOf(sel.X)).Underlying().(*types.Interface); !isIface {
				return true
			}
			hf, _ := rw.Pkg.Scope().Lookup(helper).(*types.Func)
			if hf == nil {
				return true
			}
			hs := hf.Type().(*types.Signature)
			if hs.Params().Len() != 2 || !types.Identical(hs.Params().At(0).Type(), rw.Info.TypeOf(sel.X)) ||
				!types.Identical(hs.Params().At(1).Type(), types.Typ[types.String]) || hs.Results().Len() != 1 || !types.Identical(hs.Results().At(0).Type(), types.Typ[types.String]) {
				return true
			}
			arg, ok := rw.Unparen(inner.Args[0]).(*ast.CallExpr)
			if !ok || len(arg.Args) != 1 {
				return true
			}
			if name, _ := rw.CallName(arg); name != "strobj_" {
				return true
			}
			n++
			rw.Rewrite[outer] = func() string { return helper + "(" + rw.Render(sel.X) + ", " + rw.Render(arg.Args[0]) + ")" }
			return false
		})
	}
	return n
}

func ruleC() (int, int) {
	if !hasFunc("hashCodeT_") || !hasFunc("boxT_") || !hasFunc("Object_hashCode") {
		return 0, 0
	}
	type twin struct {
		fd    *ast.FuncDecl
		calls []*ast.CallExpr // the Object_hashCode(k) calls of the body
		sites []*ast.CallExpr
		exist bool
	}
	twins := map[*types.Func]*twin{}
	qualifies := func(fn *types.Func) *twin {
		if t, ok := twins[fn]; ok {
			return t
		}
		twins[fn] = nil
		var fd *ast.FuncDecl
		for _, f := range rw.Files {
			for _, d := range f.Decls {
				if x, ok := d.(*ast.FuncDecl); ok && rw.Info.Defs[x.Name] == types.Object(fn) {
					fd = x
				}
			}
		}
		if fd == nil || fd.Body == nil || fd.Recv != nil || fd.Type.TypeParams != nil || !generated(fd.Pos()) {
			return nil
		}
		sig := fn.Type().(*types.Signature)
		if sig.Params().Len() != 1 || !isAny(sig.Params().At(0).Type()) || len(fd.Type.Params.List) != 1 || len(fd.Type.Params.List[0].Names) != 1 {
			return nil
		}
		t := &twin{fd: fd}
		for _, id := range rw.UsesOf[sig.Params().At(0)] {
			c, ok := rw.Parent[id].(*ast.CallExpr)
			if !ok || len(c.Args) != 1 || c.Args[0] != ast.Expr(id) {
				return nil
			}
			if name, _ := rw.CallName(c); name != "Object_hashCode" {
				return nil
			}
			t.calls = append(t.calls, c)
		}
		if len(t.calls) == 0 {
			return nil
		}
		t.exist = hasFunc(fd.Name.Name + "Of_")
		twins[fn] = t
		return t
	}
	nSites := 0
	for _, f := range rw.Files {
		if !generated(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			c, ok := x.(*ast.CallExpr)
			if !ok || len(c.Args) != 1 {
				return true
			}
			id, ok := rw.Unparen(c.Fun).(*ast.Ident)
			if !ok {
				return true
			}
			fn, ok := rw.Info.Uses[id].(*types.Func)
			if !ok || fn.Pkg() != rw.Pkg {
				return true
			}
			arg, ok := rw.Unparen(c.Args[0]).(*ast.CallExpr)
			if !ok || len(arg.Args) != 1 {
				return true
			}
			if name, _ := rw.CallName(arg); name != "boxT_" {
				return true
			}
			if _, ok := types.Unalias(rw.Info.TypeOf(arg.Args[0])).(*types.TypeParam); !ok {
				return true
			}
			t := qualifies(fn)
			if t == nil {
				return true
			}
			t.sites = append(t.sites, c)
			nSites++
			rw.Rewrite[c] = func() string { return id.Name + "Of_(" + rw.Render(arg.Args[0]) + ")" }
			return false
		})
	}
	nTwins := 0
	for _, t := range twins {
		if t == nil || len(t.sites) == 0 || t.exist {
			continue
		}
		nTwins++
		t := t
		fd := t.fd
		rw.Rewrite[fd] = func() string {
			orig := rw.RenderChildren(fd)
			var set []ast.Node
			put := func(n ast.Node, g func() string) {
				rw.Rewrite[n] = g
				set = append(set, n)
			}
			pname := fd.Type.Params.List[0].Names[0].Name
			put(fd.Name, func() string { return fd.Name.Name + "Of_[K any]" })
			put(fd.Type.Params.List[0].Type, func() string { return "K" })
			for _, c := range t.calls {
				put(c, func() string { return "hashCodeT_(" + pname + ")" })
			}
			clone := rw.RenderChildren(fd)
			for _, n := range set {
				delete(rw.Rewrite, n)
			}
			return orig + "\n\n// " + fd.Name.Name + "Of_ is " + fd.Name.Name + " of a value of a type parameter, which is not boxed for its hash code (post-flip step strvar).\n" + clone
		}
	}
	return nTwins, nSites
}

func main() {
	dry, only := false, ""
	for i := 1; i < len(os.Args)-1; i++ {
		switch os.Args[i] {
		case "-n":
			dry = true
		case "-rule":
			i++
			only = os.Args[i]
		default:
			fmt.Fprintln(os.Stderr, "usage: strvar [-n] [-rule A|B|C] <dir of package closure>")
			os.Exit(2)
		}
	}
	rw.Load(os.Args[len(os.Args)-1])
	var (
		nA, nB, nT, nS int
		names          []string
	)
	if only == "" || only == "A" {
		nA, names = ruleA()
	}
	if only == "" || only == "B" {
		nB = ruleB()
	}
	if only == "" || only == "C" {
		nT, nS = ruleC()
	}
	left := 0
	for _, m := range methods {
		if m.want && !m.gain && !m.exist {
			left++
		}
	}
	fmt.Fprintf(os.Stderr, "strvar: %d string variants of methods written (%s); %d methods called with a boxed string have no variant (no use of the key gains, or it is assigned); %d interface calls with a string go to the runtime's string function; %d typed hash twins written, %d calls go to them\n",
		nA, strings.Join(names, ", "), left, nB, nT, nS)
	if dry {
		return
	}
	fmt.Fprintf(os.Stderr, "strvar: %d files written\n", rw.SaveChecked())
}
