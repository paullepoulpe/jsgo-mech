// Command optof is a post-flip step: Java's `Optional.of(s).equals(o)` of a String s and an
// Optional<String> o, which the eqhash step wrote as
//
//	optequalsOn_(Optional_of(strobj_(s)), optErase_(o))     (and the operands swapped)
//
// becomes optOfEquals_(s, o) (optEqualsOf_(o, s)), runtime helpers in rt_eqhash.go that compare
// the strings without boxing them or building the Optional: a NullPointerException when s is
// null (Optional.of), then the content of o compared with s, false when o is null or empty
// (optEqualsOf_: a NullPointerException when o is null, as o.equals does). String.equals of
// the boxed contents is ==, and an Optional never holds a null String. Java computes
// Optional.of(s) (which throws for a null s) before o, the helper computes both first: when s is
// not a constant and o not a variable or a field of one, s is checked first, by
// optOfEquals_(strNonNull_(s), o): Go evaluates the calls of an argument list from left to right,
// so strNonNull_ throws before a call in o runs, as Optional.of does.
//
// usage: optof [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	done, checked := 0, 0
	why := map[string]int{}
	isString := func(e ast.Expr) bool {
		b, ok := types.Unalias(rw.Info.Types[e].Type).(*types.Basic)
		return ok && b.Kind() == types.String
	}
	isOptString := func(e ast.Expr) bool {
		n, ok := types.Unalias(rw.Info.Types[e].Type).(*types.Named)
		return ok && n.Obj().Name() == "Optional" && n.TypeArgs().Len() == 1 && types.Identical(n.TypeArgs().At(0), types.Typ[types.String])
	}
	// of: Optional_of(strobj_(s)) -> s
	of := func(e ast.Expr) ast.Expr {
		name, c := rw.CallName(e)
		if name != "Optional_of" || len(c.Args) != 1 {
			return nil
		}
		bn, b := rw.CallName(c.Args[0])
		if bn != "strobj_" || len(b.Args) != 1 || !isString(b.Args[0]) {
			return nil
		}
		return b.Args[0]
	}
	// erased: optErase_(o) of an Optional[string] o -> o
	erased := func(e ast.Expr) ast.Expr {
		name, c := rw.CallName(e)
		if name != "optErase_" || len(c.Args) != 1 || !isOptString(c.Args[0]) {
			return nil
		}
		return c.Args[0]
	}
	for _, f := range rw.Files {
		fn := rw.Fset.Position(f.Pos()).Filename
		if !strings.HasPrefix(fn[strings.LastIndex(fn, "/")+1:], "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			name, c := rw.CallName(e)
			if name != "optequalsOn_" || len(c.Args) != 2 {
				return true
			}
			if s, o := of(c.Args[0]), erased(c.Args[1]); s != nil && o != nil {
				done++
				if !constant(s) && !readPath(o) {
					// Java's Optional.of(s) throws for a null s before o is computed: so does
					// strNonNull_(s), the first call of the argument list.
					checked++
					rw.Rewrite[c] = func() string { return "optOfEquals_(strNonNull_(" + rw.Render(s) + "), " + rw.Render(o) + ")" }
					return false
				}
				rw.Rewrite[c] = func() string { return "optOfEquals_(" + rw.Render(s) + ", " + rw.Render(o) + ")" }
				return false
			}
			if o, s := erased(c.Args[0]), of(c.Args[1]); s != nil && o != nil {
				done++
				rw.Rewrite[c] = func() string { return "optEqualsOf_(" + rw.Render(o) + ", " + rw.Render(s) + ")" }
				return false
			}
			if of(c.Args[0]) != nil || of(c.Args[1]) != nil {
				why["the Optional operand is not an Optional<String>"]++
			}
			return true
		})
	}
	keys := []string{}
	for k := range why {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", why[k], k)
	}
	if report {
		fmt.Fprintf(os.Stderr, "optof: %d Optional.of(s).equals(o) rewritten (%d with s checked by strNonNull_ before o is computed)\n", done, checked)
		return
	}
	fmt.Fprintf(os.Stderr, "optof: %d Optional.of(s).equals(o) rewritten (%d with s checked by strNonNull_ before o is computed); files changed: %d\n", done, checked, rw.SaveChecked())
}

func constant(e ast.Expr) bool {
	tv, ok := rw.Info.Types[e]
	return ok && tv.Value != nil
}

// readPath: a variable or a chain of fields of one; computing it can do nothing else.
func readPath(e ast.Expr) bool {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		_, ok := rw.Info.Uses[x].(*types.Var)
		return ok
	case *ast.SelectorExpr:
		sel, ok := rw.Info.Selections[x]
		return ok && sel.Kind() == types.FieldVal && readPath(x.X)
	}
	return false
}
