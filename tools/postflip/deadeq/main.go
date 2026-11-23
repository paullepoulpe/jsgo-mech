// Command deadeq is a post-flip step: an equals or hashCode method that nothing can call is not
// kept.
//
// j2go translates every Java equals(Object) and hashCode(), and the records' generated ones, as
// methods equals(any) bool and hashCode() int32. In Java they exist for whoever may hold the
// object as an Object: a HashMap, Objects.equals, a List.contains. In the Go program the callers
// are known. A method M of class C runs only
//
//   - when it is named: x.equals(y), (*C).equals passed to equalsBy_, this.C.equals(o) of a
//     subclass, the typed form equals_C that the eqtyped step made; or
//   - dynamically: Object_equals / Object_hashCode (and what the runtime builds on them:
//     objequals_, objhash_, the hashing collections, contains, indexOf) ask a value held in an
//     `any` for the method by an interface assertion, and x.equals(y) on a value of an
//     interface type that declares it dispatches to it.
//
// The dynamic callers only ever see a value that was converted to an interface. Package anyflow
// follows every such value through the whole package, the handwritten runtime included (its doc
// comment gives the model: slots, edges, bindings of type arguments, and the sinks, where a
// value goes somewhere that is not followed). Here a call x.equals(..) or x.hashCode() on a value
// of an interface type is a sink too. A value of class D "has" M when M is the method that
// (*D).equals / (*D).hashCode names: C itself and the classes that embed C without declaring
// their own.
//
// M is live when
//   - a value that has it reaches a sink;
//   - it is named by code that is itself live (a use inside an equals or hashCode that is dead
//     does not count; the fixed point is taken);
//   - its class is converted to, or asserted from, an interface type that declares the method
//     (the program would not compile without it).
//
// Every other M is deleted, with the typed form equals_C and the _impl twin that only deleted
// methods named. Java's behaviour is kept because a deleted method has no caller: nothing names
// it, and no value that has it can be where the runtime looks for it by assertion.
//
// Left methods are counted by the first reason (-v lists every reason per class with the first
// place and path). Running the step again changes nothing: what is left is live.
//
// With -trap a dead method is kept and its body replaced by a panic naming it: a build of that
// tree run over the test inputs checks the step (nothing may panic).
//
// usage: deadeq [-n] [-v] [-trap] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/anyflow"
	"postflip/rw"
)

type meth struct {
	fn      *types.Func
	decl    *ast.FuncDecl
	class   *types.TypeName
	kind    string // "equals" or "hashCode"
	reasons map[string]string
	usedBy  []*meth // candidate methods whose bodies name this one
}

// trap (-trap, for checking the step): a dead method is not deleted, its body is replaced by a
// panic, so that a run of the compiler shows that nothing calls it.
var trap bool

var (
	cands   = map[*types.Func]*meth{}
	carrier = map[*types.TypeName][]*meth{} // struct type -> the candidate methods its values have
)

func main() {
	args := os.Args[1:]
	report, verbose := false, false
	for len(args) > 1 {
		switch args[0] {
		case "-n":
			report = true
		case "-v":
			verbose = true
		case "-trap":
			trap = true
		}
		args = args[1:]
	}
	anyflow.Load(args[0])
	// The candidates: the equals and hashCode methods of the generated classes, and for each
	// struct type the ones its values have.
	for _, n := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		if _, ok := tn.Type().Underlying().(*types.Struct); !ok {
			continue
		}
		for _, name := range []string{"equals", "hashCode"} {
			o, _, _ := types.LookupFieldOrMethod(types.NewPointer(tn.Type()), true, rw.Pkg, name)
			fo, _ := o.(*types.Func)
			if fo == nil {
				continue
			}
			fo = fo.Origin()
			fd := anyflow.DeclOf[fo]
			if fd == nil || !anyflow.InG(fo.Pos()) {
				continue
			}
			m := cands[fo]
			if m == nil {
				m = &meth{fn: fo, decl: fd, class: anyflow.Named(fo.Type().(*types.Signature).Recv().Type()), kind: name, reasons: map[string]string{}}
				cands[fo] = m
			}
			carrier[tn] = append(carrier[tn], m)
		}
	}
	// A class converted to an interface type that declares equals or hashCode needs the method.
	needs := func(cs []*types.TypeName, it types.Type, at ast.Node) {
		if it == nil || !types.IsInterface(it) {
			return
		}
		for _, name := range []string{"equals", "hashCode"} {
			if o, _, _ := types.LookupFieldOrMethod(it, true, rw.Pkg, name); o != nil {
				for _, c := range cs {
					for _, m := range carrier[c] {
						if m.kind == name {
							if _, ok := m.reasons["its class is used as an interface type that declares "+name]; !ok {
								m.reasons["its class is used as an interface type that declares "+name] = anyflow.Where(at)
							}
						}
					}
				}
			}
		}
	}
	anyflow.OnConvert = func(cs []*types.TypeName, direct bool, it types.Type, at ast.Node) {
		if direct {
			needs(cs, it, at)
		}
	}
	anyflow.EqualsSinks = true
	// The body of a method that is dead never runs: its conversions and calls are not
	// followed. Start with every candidate's body left out and add the bodies of those found
	// live until nothing changes.
	for _, m := range cands {
		anyflow.Skip[m.decl] = true
	}
	var all []*meth
	for _, m := range cands {
		all = append(all, m)
	}
	sort.Slice(all, func(i, j int) bool { return all[i].fn.Pos() < all[j].fn.Pos() })
	live := map[*meth]bool{}
	for round := 1; ; round++ {
		for _, m := range all {
			m.reasons = map[string]string{}
			m.usedBy = nil
		}
		anyflow.Reset()
		anyflow.Build()
		clear(live)
		solve(all, live, needs)
		grown := false
		for _, m := range all {
			if live[m] && anyflow.Skip[m.decl] {
				delete(anyflow.Skip, m.decl)
				grown = true
			}
		}
		if !grown {
			break
		}
	}
	finish(all, live, report, verbose)
}

// solve finds the live methods on the built graph.
func solve(all []*meth, live map[*meth]bool, needs func(cs []*types.TypeName, it types.Type, at ast.Node)) {
	// A type assertion or a type switch case from such an interface to the class.
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			var x ast.Expr
			var ts []ast.Expr
			switch a := n.(type) {
			case *ast.TypeAssertExpr:
				if a.Type != nil {
					x, ts = a.X, []ast.Expr{a.Type}
				}
			case *ast.CaseClause:
				if sw, ok := rw.Parent[rw.Parent[a]].(*ast.TypeSwitchStmt); ok {
					switch as := sw.Assign.(type) {
					case *ast.AssignStmt:
						x = rw.Unparen(as.Rhs[0]).(*ast.TypeAssertExpr).X
					case *ast.ExprStmt:
						x = rw.Unparen(as.X).(*ast.TypeAssertExpr).X
					}
					ts = a.List
				}
			}
			if x == nil {
				return true
			}
			xt := rw.Info.Types[x].Type
			for _, te := range ts {
				if tt := rw.Info.Types[te].Type; tt != nil && xt != nil && !anyflow.IsIface(tt) {
					if cs, direct := anyflow.Mentions(tt); direct {
						needs(cs, xt, te)
					}
				}
			}
			return true
		})
	}
	for c, rs := range anyflow.Sunk {
		for _, m := range carrier[c] {
			for r, at := range rs {
				k := "a value reaches: " + r
				// held by a generic type or function: name the one that takes it to the sink
				if i := strings.Index(at, " -> type parameter "); i >= 0 {
					owner := at[i+len(" -> type parameter "):]
					if j := strings.Index(owner, " of "); j >= 0 {
						owner = owner[j+4:]
					}
					if j := strings.Index(owner, " -> "); j >= 0 {
						owner = owner[:j]
					}
					k = "a value, as a type argument of " + owner + ", reaches: " + r
				}
				if c != m.class {
					at = "as " + c.Name() + ": " + at
				}
				if old, ok := m.reasons[k]; !ok || at < old {
					m.reasons[k] = at
				}
			}
		}
	}
	// Named uses.
	for _, m := range all {
		for _, id := range rw.UsesOf[m.fn] {
			var by *meth
			if fd := anyflow.EnclosingDecl(id); fd != nil {
				if fo, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					by = cands[fo]
				}
			}
			if by != nil {
				if by != m {
					m.usedBy = append(m.usedBy, by)
				}
				continue
			}
			k := "named: " + useKind(id)
			if _, ok := m.reasons[k]; !ok {
				m.reasons[k] = anyflow.Where(id)
			}
		}
	}
	for _, m := range all {
		if len(m.reasons) > 0 {
			live[m] = true
		}
	}
	for changed := true; changed; {
		changed = false
		for _, m := range all {
			if live[m] {
				continue
			}
			for _, by := range m.usedBy {
				if live[by] {
					live[m] = true
					m.reasons["named by a live equals or hashCode"] = by.class.Name() + "." + by.kind
					changed = true
					break
				}
			}
		}
	}
}

func finish(all []*meth, live map[*meth]bool, report, verbose bool) {
	done := map[string]int{}
	left := map[string]int{}
	for _, m := range all {
		if live[m] {
			keys := []string{}
			for k := range m.reasons {
				keys = append(keys, k)
			}
			sort.Slice(keys, func(i, j int) bool {
				pi, pj := prio(keys[i]), prio(keys[j])
				if pi != pj {
					return pi < pj
				}
				return keys[i] < keys[j]
			})
			left[m.kind+": "+keys[0]]++
			if verbose {
				fmt.Fprintf(os.Stderr, "live %s.%s\n", m.class.Name(), m.kind)
				for _, k := range keys {
					fmt.Fprintf(os.Stderr, "      %s [%s]\n", k, m.reasons[k])
				}
			}
			continue
		}
		done[m.kind]++
		if verbose {
			fmt.Fprintf(os.Stderr, "dead %s.%s\n", m.class.Name(), m.kind)
		}
		if trap {
			name := m.class.Name() + "." + m.kind
			body := m.decl.Body
			rw.Rewrite[body] = func() string { return "{\n\tpanic(\"deadeq: " + name + " called\")\n}" }
			continue
		}
		rw.Rewrite[m.decl] = func() string { return rw.DelDoc }
	}
	if trap {
		fmt.Fprintf(os.Stderr, "deadeq -trap: files changed: %d\n", rw.SaveChecked())
		return
	}
	// Helpers that only deleted methods name: equals_C, equals_impl, hashCode_impl.
	helpers := 0
	fos := make([]*types.Func, 0, len(anyflow.DeclOf))
	for fo := range anyflow.DeclOf {
		fos = append(fos, fo)
	}
	sort.Slice(fos, func(i, j int) bool { return fos[i].Pos() < fos[j].Pos() })
	for changed := true; changed; {
		changed = false
		for _, fo := range fos {
			fd := anyflow.DeclOf[fo]
			if !anyflow.InG(fo.Pos()) || rw.Rewrite[fd] != nil || fd.Recv == nil {
				continue
			}
			n := fo.Name()
			if !(strings.HasPrefix(n, "equals_") || n == "hashCode_impl") {
				continue
			}
			uses := rw.UsesOf[fo]
			if len(uses) == 0 {
				continue
			}
			dead := true
			for _, id := range uses {
				if d := anyflow.EnclosingDecl(id); d == nil || (rw.Rewrite[d] == nil && d != fd) {
					dead = false
					break
				}
			}
			// a method an interface lists stays (the class is converted to it)
			if dead && !inAnyInterface(n) {
				rw.Rewrite[fd] = func() string { return rw.DelDoc }
				helpers++
				changed = true
			}
		}
	}
	for _, k := range []string{"equals", "hashCode"} {
		fmt.Fprintf(os.Stderr, "  deleted: %4d %s methods\n", done[k], k)
	}
	fmt.Fprintf(os.Stderr, "  deleted: %4d typed forms and _impl twins that only they named\n", helpers)
	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
	}
	if report {
		return
	}
	fmt.Fprintf(os.Stderr, "deadeq: files changed: %d\n", rw.SaveChecked())
}

func inAnyInterface(name string) bool {
	for _, n := range rw.Pkg.Scope().Names() {
		if tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName); ok {
			if it, ok := tn.Type().Underlying().(*types.Interface); ok {
				for i := 0; i < it.NumMethods(); i++ {
					if it.Method(i).Name() == name {
						return true
					}
				}
			}
		}
	}
	return false
}

func prio(k string) int {
	switch {
	case strings.HasPrefix(k, "named: "):
		return 0
	case strings.HasPrefix(k, "named by"):
		return 1
	case strings.HasPrefix(k, "its class is used"):
		return 2
	}
	return 3
}

func useKind(id *ast.Ident) string {
	if s, ok := rw.Parent[id].(*ast.SelectorExpr); ok && s.Sel == id {
		if c, ok := rw.Up(s).(*ast.CallExpr); ok && rw.Unparen(c.Fun) == ast.Expr(s) {
			return "called (x." + id.Name + "(...))"
		}
		if tv, ok := rw.Info.Types[s.X]; ok && tv.IsType() {
			return "the method expression (*C)." + id.Name + " (equalsBy_, hashBy_)"
		}
		return "the method value x." + id.Name
	}
	return "named"
}
