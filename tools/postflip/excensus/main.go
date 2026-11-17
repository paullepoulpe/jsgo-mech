// Command excensus counts, by kind, what is left of the Java-isms that the exception steps
// (throws, tryfn, exceptions) and their neighbours deal with: panics, recovers and defers,
// lambda objects, instanceof helpers inside expressions, Optional, equals/hashCode, threads,
// reflect and unsafe. It changes nothing; the kinds are the reasons the steps' commit messages
// give for leaving a site.
//
// usage: excensus <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"postflip/rw"
)

var count = map[string]map[string]int{}

func add(row, kind string) {
	if count[row] == nil {
		count[row] = map[string]int{}
	}
	count[row][kind]++
}

func main() {
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		name := filepath.Base(rw.Fset.File(f.Pos()).Name())
		if !strings.HasPrefix(name, "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CallExpr:
				call(x)
			case *ast.DeferStmt:
				deferStmt(x)
			case *ast.CompositeLit:
				lambda(x)
			case *ast.FuncDecl:
				if strings.HasPrefix(x.Name.Name, "is_") && x.Recv == nil {
					add("instanceof is_X(", "declaration of a generated is_X helper (a class with subclasses: an assertion to X_as)")
				}
			}
			return true
		})
	}
	var rows []string
	for r := range count {
		rows = append(rows, r)
	}
	sort.Strings(rows)
	for _, r := range rows {
		total := 0
		var ks []string
		for k, v := range count[r] {
			ks = append(ks, k)
			total += v
		}
		sort.Strings(ks)
		fmt.Printf("%s: %d\n", r, total)
		for _, k := range ks {
			fmt.Printf("  %5d %s\n", count[r][k], k)
		}
	}
}

func generatedCallee(fun ast.Expr) (string, bool) {
	var o types.Object
	switch f := fun.(type) {
	case *ast.Ident:
		o = rw.Info.Uses[f]
	case *ast.SelectorExpr:
		o = rw.Info.Uses[f.Sel]
	case *ast.IndexExpr:
		return generatedCallee(f.X)
	case *ast.IndexListExpr:
		return generatedCallee(f.X)
	}
	if o == nil || !o.Pos().IsValid() {
		return "", false
	}
	return o.Name(), strings.HasPrefix(filepath.Base(rw.Fset.Position(o.Pos()).Filename), "g_")
}

func enclosingFunc(n ast.Node) ast.Node {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch p.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return p
		}
	}
	return nil
}

func call(c *ast.CallExpr) {
	id, ok := c.Fun.(*ast.Ident)
	if ok {
		if _, b := rw.Info.Uses[id].(*types.Builtin); b {
			switch id.Name {
			case "panic":
				panicKind(c)
			case "recover":
				recoverKind(c)
			}
			return
		}
		if id.Name == "catch_" {
			add("catch_", "catch_(r_): the recovered value as a Java exception (Go runtime errors as NullPointerException etc.), one per recover")
		}
	}
	name, _ := rw.CallName(c)
	if strings.HasPrefix(name, "is_") || strings.HasPrefix(name, "is_[") {
		instanceof(c)
	}
}

func panicKind(c *ast.CallExpr) {
	a := rw.Unparen(c.Args[0])
	t := rw.Text(a)
	row := "panic("
	if fn, ok := enclosingFunc(c).(*ast.FuncLit); ok {
		if d, ok := rw.Parent[rw.Parent[fn]].(*ast.DeferStmt); ok && strings.Contains(rw.Text(d), "recover()") {
			if t == "r_" {
				add(row, "rethrow panic(r_) in a catch defer: a recover that does not handle the value passes it on (Go idiom)")
			} else {
				add(row, "throw inside a catch clause (a Java catch block that throws: wraps the cause or rethrows it)")
			}
			return
		}
	}
	switch {
	case t == `"unreachable"`:
		add(row, `panic("unreachable") after a try whose code is read (Go's terminating-statement rule)`)
	case isString(a):
		add(row, "panic with a string: abstract-method fallback or j2go stub that Java never reaches (Go idiom for an impossible state)")
	case strings.HasPrefix(t, "enumValueOfFail_("):
		add(row, "Enum.valueOf of an unknown name: a new IllegalArgumentException (Go error value)")
	default:
		cls := "?"
		if tt := rw.Info.Types[a].Type; tt != nil {
			if p, ok := tt.(*types.Pointer); ok {
				if nt, ok := p.Elem().(*types.Named); ok {
					cls = nt.Obj().Name()
				}
			} else {
				cls = "a value of type " + types.TypeString(tt, func(*types.Package) string { return "" })
			}
		}
		switch cls {
		case "IllegalStateException", "IllegalArgumentException", "NullPointerException", "UnsupportedOperationException", "AssertionError", "IndexOutOfBoundsException", "NoSuchElementException", "ArithmeticException", "Error", "ClassCastException", "ArrayIndexOutOfBoundsException":
			add(row, "panic(err) of an unchecked "+groupOf(cls)+" (a programmer error: the Go idiom)")
		default:
			if strings.HasPrefix(cls, "a value") {
				add(row, "panic of "+cls+" (an exception held in a variable, its most derived object)")
			} else {
				add(row, "panic(err) of a "+cls+" that a catch clause or the runner reports (Java's failure path, kept as the error value)")
			}
		}
	}
}

func groupOf(cls string) string {
	switch cls {
	case "AssertionError", "Error":
		return "Error (AssertionError)"
	}
	return "RuntimeException (" + cls + ")"
}

func isString(e ast.Expr) bool {
	t := rw.Info.Types[e].Type
	if t == nil {
		return false
	}
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsString != 0
}

// recoverKind: what the catch defer catches.
func recoverKind(c *ast.CallExpr) {
	d := rw.Parent[rw.Parent[enclosingFunc(c)]]
	txt := rw.Text(d)
	row := "recover()"
	var cls []string
	for _, m := range regexp.MustCompile(`is_(\w+)\(ex_\)|ex_\.\(\*?(\w+?)(?:_as)?\)`).FindAllStringSubmatch(txt, -1) {
		cls = append(cls, m[1]+m[2])
	}
	all := strings.Join(cls, "|")
	// Does every catch clause end by throwing (a wrap or a rethrow)?
	wraps := true
	if is, ok := rw.Parent[rw.Parent[c]].(*ast.IfStmt); ok && len(is.Body.List) >= 2 {
		for _, st := range is.Body.List[1 : len(is.Body.List)-1] {
			ci, ok := st.(*ast.IfStmt)
			if !ok || !throwsAtEnd(ci.Body) {
				wraps = false
			}
		}
	}
	where := "in a try literal"
	if _, ok := rw.Parent[d].(*ast.BlockStmt); ok {
		if _, ok := rw.Parent[rw.Parent[d]].(*ast.FuncDecl); ok {
			where = "deferred by the function the try ends"
		}
	}
	switch {
	case len(cls) == 0:
		add(row, "recover of anything, first failure kept (PrebuildAst/PrebuildDependencyInfo: Java's executor future), "+where)
	case regexp.MustCompile(`^(Throwable|Exception|Error\|Exception|Error|RuntimeException)$`).MatchString(all):
		if wraps {
			add(row, "boundary catch-all ("+all+") that wraps the failure with context and throws it on (recover, wrap, re-panic: the cause chain reaches the top), "+where)
		} else {
			add(row, "boundary catch-all ("+all+") that reports or carries any failure (the runner, the executor; Go idiom: recover at the boundary), "+where)
		}
	case wraps:
		add(row, "typed catch ("+all+") that wraps a failure raised inside a callee and throws it on, "+where)
	default:
		add(row, "typed catch ("+all+") handling a failure raised inside a callee (the runtime's parsers, Go runtime errors as IndexOutOfBounds/NumberFormat...): exceptions as control flow, "+where)
	}
}

func deferStmt(d *ast.DeferStmt) {
	txt := rw.Text(d)
	row := "defer"
	switch {
	case strings.Contains(txt, "recover()"):
		add(row, "catch defer (recover) of a try: counted under recover()")
	case strings.HasPrefix(txt, "defer closeResource_("):
		add(row, "try-with-resources: close the resource (Go idiom: defer x.Close())")
	default:
		add(row, "finally block (Go idiom: defer the cleanup)")
	}
}

func lambda(cl *ast.CompositeLit) {
	id, ok := cl.Type.(*ast.Ident)
	if !ok || !strings.HasSuffix(id.Name, "_fn") || len(cl.Elts) != 1 {
		return
	}
	if _, ok := cl.Elts[0].(*ast.FuncLit); !ok {
		return
	}
	row := "lambda objects &X_fn{func"
	u, ok := rw.Parent[cl].(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		add(row, "other")
		return
	}
	p := rw.Up(u)
	if vs, ok := p.(*ast.ValueSpec); ok {
		if _, ok := rw.Parent[rw.Parent[vs]].(*ast.File); ok {
			add(row, "non-capturing lambda made once at package level (the lambdas step; one object per lambda as HotSpot makes it): an object with identity")
			return
		}
	}
	switch q := p.(type) {
	case *ast.CallExpr:
		name, gen := generatedCallee(q.Fun)
		if s, ok := q.Fun.(*ast.SelectorExpr); ok && name == "" {
			name = s.Sel.Name
		}
		if !gen {
			add(row, "capturing lambda passed to the runtime (Guava/JDK API taking the interface, e.g. "+short(name)+")")
		} else {
			add(row, "capturing lambda passed to a generated function whose parameter is stored or handed on (funcparams: not only called)")
		}
	case *ast.ReturnStmt:
		add(row, "capturing lambda returned as an interface value (stored by the caller)")
	case *ast.AssignStmt, *ast.ValueSpec, *ast.KeyValueExpr, *ast.CompositeLit:
		add(row, "capturing lambda stored in a field, variable or collection")
	default:
		add(row, "capturing lambda in another expression")
	}
}

func short(name string) string {
	if i := strings.Index(name, "_"); i > 0 && strings.Count(name, "_") > 1 {
		return name
	}
	return name
}

func instanceof(c *ast.CallExpr) {
	row := "instanceof is_X("
	if len(c.Args) == 1 && rw.Text(c.Args[0]) == "ex_" {
		add(row, "catch clause test is_X(ex_) of a multi-catch (read by the exceptions step)")
		return
	}
	p := rw.Up(c)
	if u, ok := p.(*ast.UnaryExpr); ok && u.Op == token.NOT {
		p = rw.Up(u)
	}
	switch p.(type) {
	case *ast.BinaryExpr:
		add(row, "inside && / || (Go has no comma-ok expression; the one-line helper is inlined)")
	case *ast.ForStmt:
		add(row, "a for condition (re-tested each iteration; no comma-ok form)")
	case *ast.CaseClause:
		add(row, "a switch-true case")
	default:
		add(row, "other expression ("+fmt.Sprintf("%T", p)+")")
	}
}

// throwsAtEnd reports whether a catch block ends in a panic (on every path through its last
// if/else).
func throwsAtEnd(b *ast.BlockStmt) bool {
	if len(b.List) == 0 {
		return false
	}
	switch l := b.List[len(b.List)-1].(type) {
	case *ast.ExprStmt:
		return strings.HasPrefix(rw.Text(l), "panic(")
	case *ast.IfStmt:
		e, ok := l.Else.(*ast.BlockStmt)
		return ok && throwsAtEnd(l.Body) && throwsAtEnd(e)
	}
	return false
}
