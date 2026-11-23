// Command varargs is a post-flip step: a Java varargs parameter `Object... args` is a Go variadic
// parameter `args ...any`, and its calls pass the values themselves.
//
// j2go writes the parameter as a slice and every call as the array Java's compiler makes:
//
//	func Strings_lenientFormat(template string, args []any) string
//	Strings_lenientFormat("%s: %s", []any{a, b})
//
// The step writes
//
//	func Strings_lenientFormat(template string, args ...any) string
//	Strings_lenientFormat("%s: %s", a, b)
//
// A variadic parameter is a slice inside its function, and a call with listed values makes a new
// slice of them, in order, as the literal did: the bodies do not change. The functions taken are
// those whose last parameter is a []any that every call fills with a literal `[]any{...}`, with
// nil, or by forwarding the caller's own such parameter (which is then spread: `args...`). That
// is how a Java varargs method is called; a method that is also given an existing array
// (toArray, Arrays.asList of an array variable) stays as it is. Methods go by name: an
// interface's method, the methods that implement it and the dispatch twins change together or
// not at all (every method of the name must qualify), and a method that is used as a function
// value is left, since the value's type would change.
//
// One difference is kept out: `f(x, []any{})` passes an empty array that is not null, `f(x)`
// passes a nil slice. Where a function of the family (or one it forwards to) can tell them apart
// (it does anything with the parameter but len, range, index, or forwarding it), the empty
// call is written `f(x, []any{}...)`, which passes the same empty slice as before; elsewhere
// `f(x)`.
//
// The declarations changed are in the generated code and in the handwritten runtime (the
// signature only: `args []any` becomes `args ...any`), and the calls in both and in the tests.
// Left, counted: families with a call that passes another slice, a method used as a value, a
// method of the name without the parameter. A rerun changes nothing (a variadic parameter no
// longer matches, a spread argument is not a literal).
//
// usage: varargs [-n [-v]] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

type family struct {
	name    string // "m:<method name>" or "f:<function name>"
	decls   []*ast.FuncDecl
	specs   []*ast.Field // interface method specifications
	calls   []*ast.CallExpr
	blocked string
	nilSens bool
}

var (
	fams   = map[string]*family{}
	famOf  = map[types.Object]*family{}
	lastOf = map[types.Object]*family{} // the last parameter of a declaration -> its family
)

func isAnySlice(t types.Type) bool {
	if t == nil {
		return false
	}
	s, ok := types.Unalias(t).(*types.Slice)
	if !ok {
		return false
	}
	i, ok := s.Elem().Underlying().(*types.Interface)
	return ok && i.NumMethods() == 0 && !i.IsComparable()
}

func lastParamType(sig *types.Signature) types.Type {
	if sig.Variadic() || sig.Params().Len() == 0 {
		return nil
	}
	return sig.Params().At(sig.Params().Len() - 1).Type()
}

func get(name string) *family {
	f := fams[name]
	if f == nil {
		f = &family{name: name}
		fams[name] = f
	}
	return f
}

// Methods are grouped by the interfaces that tie their signatures together: an interface's
// method and the method of that name of every type that implements the interface.
var parent = map[*types.Func]*types.Func{}

func find(f *types.Func) *types.Func {
	for parent[f] != nil && parent[f] != f {
		f = parent[f]
	}
	return f
}

func union(a, b *types.Func) {
	a, b = find(a), find(b)
	if a == b {
		return
	}
	if a.Pos() > b.Pos() {
		a, b = b, a
	}
	parent[b] = a
}

// groupMethods unions the methods named in names.
func groupMethods(names map[string]bool) {
	var ifaces []*types.Interface
	seen := map[*types.Interface]bool{}
	var byName = map[string][]*types.Func{} // methods of generic types and of anonymous use
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.InterfaceType:
				if it, ok := rw.Info.Types[x].Type.(*types.Interface); ok && !seen[it] {
					seen[it] = true
					ifaces = append(ifaces, it)
				}
			case *ast.FuncDecl:
				if fo, ok := rw.Info.Defs[x.Name].(*types.Func); ok && x.Recv != nil && names[fo.Name()] {
					byName[fo.Name()] = append(byName[fo.Name()], fo)
				}
			}
			return true
		})
	}
	var named []*types.Named
	for _, n := range rw.Pkg.Scope().Names() {
		if tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName); ok && !tn.IsAlias() {
			if nt, ok := tn.Type().(*types.Named); ok {
				named = append(named, nt)
			}
		}
	}
	for _, it := range ifaces {
		var ms []*types.Func
		for i := 0; i < it.NumMethods(); i++ {
			if m := it.Method(i); names[m.Name()] {
				ms = append(ms, m)
			}
		}
		if len(ms) == 0 {
			continue
		}
		for _, nt := range named {
			if nt.TypeParams().Len() > 0 {
				// a generic type: its methods of the name go with the interface's
				for _, m := range ms {
					for _, fo := range byName[m.Name()] {
						if r := fo.Type().(*types.Signature).Recv(); r != nil {
							t := r.Type()
							if p, ok := t.(*types.Pointer); ok {
								t = p.Elem()
							}
							if n2, ok := t.(*types.Named); ok && n2.Origin() == nt {
								union(m, fo)
							}
						}
					}
				}
				continue
			}
			var t types.Type = nt
			if !types.Implements(t, it) {
				t = types.NewPointer(nt)
				if !types.Implements(t, it) {
					continue
				}
			}
			for _, m := range ms {
				o, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, m.Name())
				if fo, ok := o.(*types.Func); ok {
					union(m, fo.Origin())
				}
			}
		}
	}
}

func block(f *family, why string) {
	if f.blocked == "" {
		f.blocked = why
	}
}

func calleeObj(c *ast.CallExpr) types.Object {
	switch f := rw.Unparen(c.Fun).(type) {
	case *ast.Ident:
		return rw.Info.Uses[f]
	case *ast.SelectorExpr:
		return rw.Info.Uses[f.Sel]
	}
	return nil
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	verbose := len(os.Args) > 3 && os.Args[2] == "-v"
	rw.WithTests = true
	rw.Load(os.Args[len(os.Args)-1])
	// The families: a function by itself, methods as the interfaces group them.
	cand := map[string]bool{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			var id *ast.Ident
			switch x := n.(type) {
			case *ast.FuncDecl:
				if x.Recv != nil {
					id = x.Name
				}
			case *ast.Field:
				if _, ok := x.Type.(*ast.FuncType); ok && len(x.Names) == 1 {
					if _, ok := rw.Parent[rw.Parent[x]].(*ast.InterfaceType); ok {
						id = x.Names[0]
					}
				}
			}
			if id != nil {
				if fo, ok := rw.Info.Defs[id].(*types.Func); ok && isAnySlice(lastParamType(fo.Type().(*types.Signature))) {
					cand[fo.Name()] = true
				}
			}
			return true
		})
	}
	groupMethods(cand)
	mname := func(fo *types.Func) string {
		r := find(fo)
		p := rw.Fset.Position(r.Pos())
		return fmt.Sprintf("m:%s@%s:%d:%d", fo.Name(), p.Filename[strings.LastIndex(p.Filename, "/")+1:], p.Line, p.Column)
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				fo, _ := rw.Info.Defs[x.Name].(*types.Func)
				if fo == nil {
					return true
				}
				sig := fo.Type().(*types.Signature)
				var fam *family
				if x.Recv == nil {
					if lt := lastParamType(sig); lt == nil || !isAnySlice(lt) || sig.TypeParams().Len() > 0 {
						return true
					}
					fam = get("f:" + fo.Name())
				} else {
					if !cand[fo.Name()] {
						return true
					}
					fam = get(mname(fo))
					if lt := lastParamType(sig); lt == nil || !isAnySlice(lt) {
						block(fam, "a method tied to it by an interface has no []any as its last parameter")
					}
				}
				fam.decls = append(fam.decls, x)
				famOf[fo] = fam
				if n := sig.Params().Len(); n > 0 {
					lastOf[sig.Params().At(n-1)] = fam
				}
				pl := x.Type.Params.List
				if len(pl) > 0 && len(pl[len(pl)-1].Names) > 1 {
					block(fam, "the parameter is declared with another in one field")
				}
			case *ast.InterfaceType:
				for _, m := range x.Methods.List {
					ft, ok := m.Type.(*ast.FuncType)
					if !ok || len(m.Names) != 1 {
						continue
					}
					fo, _ := rw.Info.Defs[m.Names[0]].(*types.Func)
					if fo == nil {
						continue
					}
					if !cand[fo.Name()] {
						continue
					}
					fam := get(mname(fo))
					fam.specs = append(fam.specs, m)
					famOf[fo] = fam
					sig := fo.Type().(*types.Signature)
					if lt := lastParamType(sig); lt == nil || !isAnySlice(lt) {
						block(fam, "a method tied to it by an interface has no []any as its last parameter")
					}
					if pl := ft.Params.List; len(pl) > 0 && len(pl[len(pl)-1].Names) > 1 {
						block(fam, "the parameter is declared with another in one field")
					}
				}
			}
			return true
		})
	}
	// Uses: calls, and uses as values.
	for id, o := range rw.Info.Uses {
		fo, ok := o.(*types.Func)
		if !ok {
			continue
		}
		fam := famOf[fo]
		if fam == nil {
			continue
		}
		var e ast.Expr = id
		if s, ok := rw.Parent[id].(*ast.SelectorExpr); ok && s.Sel == id {
			e = s
		}
		if c, ok := rw.Up(e).(*ast.CallExpr); ok && rw.Unparen(c.Fun) == e {
			fam.calls = append(fam.calls, c)
			continue
		}
		block(fam, "used as a function value")
	}
	// A family qualifies while every call's last argument is a literal, nil, or the caller's own
	// last parameter of a qualifying family.
	lastArg := func(c *ast.CallExpr) ast.Expr {
		if len(c.Args) == 0 || c.Ellipsis.IsValid() {
			return nil
		}
		return rw.Unparen(c.Args[len(c.Args)-1])
	}
	forwarded := func(a ast.Expr) *family {
		if id, ok := a.(*ast.Ident); ok {
			if o := rw.Info.Uses[id]; o != nil {
				return lastOf[o]
			}
		}
		return nil
	}
	isLit := func(a ast.Expr) bool {
		cl, ok := a.(*ast.CompositeLit)
		if !ok || !isAnySlice(rw.Info.Types[cl].Type) {
			return false
		}
		for _, el := range cl.Elts {
			if _, ok := el.(*ast.KeyValueExpr); ok {
				return false
			}
		}
		return true
	}
	names := make([]string, 0, len(fams))
	for n := range fams {
		names = append(names, n)
	}
	sort.Strings(names)
	for _, n := range names {
		f := fams[n]
		sort.Slice(f.calls, func(i, j int) bool { return f.calls[i].Pos() < f.calls[j].Pos() })
	}
	// A family qualifies when every call's last argument is a literal, nil, or the caller's own
	// last []any parameter (forwarded: written args...).
	for _, n := range names {
		f := fams[n]
		for _, c := range f.calls {
			a := lastArg(c)
			if a == nil {
				block(f, "a call without the argument, or with a spread already")
				break
			}
			if tv := rw.Info.Types[a]; tv.IsNil() || isLit(a) || forwarded(a) != nil {
				continue
			}
			block(f, "a call passes a slice that is not a literal (an existing array)")
			break
		}
	}
	// Taken: the qualifying families with a literal call, and the qualifying families that
	// forward their parameter to a taken one (the varargs methods that pass their arguments on).
	taken := map[*family]bool{}
	for _, n := range names {
		f := fams[n]
		if f.blocked != "" {
			continue
		}
		for _, c := range f.calls {
			if isLit(lastArg(c)) {
				taken[f] = true
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, n := range names {
			g := fams[n]
			if !taken[g] {
				continue
			}
			for _, c := range g.calls {
				if f := forwarded(lastArg(c)); f != nil && f.blocked == "" && !taken[f] {
					taken[f] = true
					changed = true
				}
			}
		}
	}
	for _, n := range names {
		if f := fams[n]; f.blocked == "" && !taken[f] {
			f.blocked = "no call passes a literal"
		}
	}
	// Which families can tell nil from empty.
	sens := func(f *family) bool {
		for _, d := range f.decls {
			if d.Body == nil {
				return true
			}
			fo := rw.Info.Defs[d.Name].(*types.Func)
			sig := fo.Type().(*types.Signature)
			p := sig.Params().At(sig.Params().Len() - 1)
			for _, id := range rw.UsesOf[p] {
				switch up := rw.Up(id).(type) {
				case *ast.CallExpr:
					if fid, ok := up.Fun.(*ast.Ident); ok && fid.Name == "len" {
						continue
					}
					if len(up.Args) > 0 && rw.Unparen(up.Args[len(up.Args)-1]) == ast.Expr(id) {
						if g := famOf[calleeObj(up)]; g != nil && g.blocked == "" && !g.nilSens {
							continue
						}
					}
					return true
				case *ast.RangeStmt:
					if up.X == ast.Expr(id) {
						continue
					}
					return true
				case *ast.IndexExpr:
					if up.X == ast.Expr(id) {
						if as, ok := rw.Up(up).(*ast.AssignStmt); ok {
							isLhs := false
							for _, l := range as.Lhs {
								isLhs = isLhs || rw.Unparen(l) == ast.Expr(up)
							}
							if isLhs {
								return true
							}
						}
						continue
					}
					return true
				default:
					return true
				}
			}
		}
		return false
	}
	for changed := true; changed; {
		changed = false
		for _, n := range names {
			f := fams[n]
			if f.blocked == "" && !f.nilSens && sens(f) {
				f.nilSens = true
				changed = true
			}
		}
	}
	done := map[string]int{}
	left := map[string]int{}
	for _, n := range names {
		f := fams[n]
		if f.blocked != "" {
			lits := 0
			for _, c := range f.calls {
				if a := lastArg(c); a != nil && isLit(a) {
					lits++
				}
			}
			if lits > 0 {
				left[f.blocked+" (literals passed)"] += lits
				if verbose {
					fmt.Fprintf(os.Stderr, "left  %s: %d literals: %s\n", n, lits, f.blocked)
				}
			}
			continue
		}
		done["functions and method names"]++
		if verbose {
			fmt.Fprintf(os.Stderr, "taken %s (%d calls, nil-sensitive %v)\n", n, len(f.calls), f.nilSens)
		}
		for _, d := range f.decls {
			pl := d.Type.Params.List
			t := pl[len(pl)-1].Type
			rw.Rewrite[t] = func() string { return "...any" }
			done["declarations"]++
		}
		for _, m := range f.specs {
			pl := m.Type.(*ast.FuncType).Params.List
			t := pl[len(pl)-1].Type
			rw.Rewrite[t] = func() string { return "...any" }
			done["interface method specifications"]++
		}
		for _, c := range f.calls {
			c := c
			a := rw.Unparen(c.Args[len(c.Args)-1])
			var last func() string
			switch {
			case isLit(a):
				cl := a.(*ast.CompositeLit)
				if len(cl.Elts) == 0 {
					if f.nilSens {
						done["calls: an empty literal kept as []any{}... (the function can tell nil from empty)"]++
						last = func() string { return "[]any{}..." }
					} else {
						done["calls: no values"]++
						last = func() string { return "" }
					}
				} else {
					done["calls: values passed bare"]++
					last = func() string {
						parts := make([]string, len(cl.Elts))
						for i, el := range cl.Elts {
							parts[i] = rw.Render(el)
						}
						return strings.Join(parts, ", ")
					}
				}
			case rw.Info.Types[a].IsNil():
				done["calls: nil spread (nil...)"]++
				last = func() string { return "nil..." }
			default:
				done["calls: the caller's parameter spread (args...)"]++
				last = func() string { return rw.Render(a) + "..." }
			}
			rw.Rewrite[c] = func() string {
				parts := []string{}
				for _, x := range c.Args[:len(c.Args)-1] {
					parts = append(parts, rw.Render(x))
				}
				if l := last(); l != "" {
					parts = append(parts, l)
				}
				return rw.Render(c.Fun) + "(" + strings.Join(parts, ", ") + ")"
			}
		}
	}
	print := func(m map[string]int, what string) {
		keys := []string{}
		for k := range m {
			keys = append(keys, k)
		}
		sort.Strings(keys)
		for _, k := range keys {
			fmt.Fprintf(os.Stderr, "  %s: %4d %s\n", what, m[k], k)
		}
	}
	print(done, "variadic")
	print(left, "left")
	_ = token.NoPos
	if report {
		return
	}
	fmt.Fprintf(os.Stderr, "varargs: files changed: %d\n", rw.SaveChecked())
}
