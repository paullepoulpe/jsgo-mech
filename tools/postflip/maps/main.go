// Command maps is post-flip step 2: maps and sets that are never iterated and never leave their
// owner become builtin Go maps.
//
// A struct field or local variable declared MapOf[K, V], LinkedHashMapOf[K, V], SetOf[K] or
// LinkedHashSetOf[K] is rewritten to map[K]V or map[K]struct{} when
//   - K is string, a handle type, or a pointer to a class without equals/hashCode of its own
//     (Go's == on K is then Java's key equality),
//   - every assignment to it is a new LinkedHashMap / LinkedHashSet (or nil), and
//   - every other use is get, put (as a statement), containsKey, remove (as a statement), size,
//     isEmpty, clear; for a set: contains, add, remove (as a statement), size, isEmpty, clear.
//
// Nothing here iterates, so the insertion order the Java collection kept is not observable.
//
// usage: maps [-n] <dir of package closure>     (-n: report only)
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"

	"postflip/rw"
)

type cand struct {
	obj      *types.Var
	typeExpr ast.Expr
	keyExpr  ast.Expr
	valExpr  ast.Expr // nil for a set
	key, val types.Type
	kc, vc   rw.Conv
	reason   string
	edits    []rw.Edit
}

func (c *cand) goType() string {
	if c.valExpr == nil {
		return "map[" + rw.Text(c.keyExpr) + "]struct{}"
	}
	return "map[" + rw.Text(c.keyExpr) + "]" + rw.Text(c.valExpr)
}

func (c *cand) fail(why string) {
	if c.reason == "" {
		c.reason = why
	}
}

func (c *cand) edit(n ast.Node, gen func() string) {
	c.edits = append(c.edits, rw.Edit{Node: n, Gen: gen})
}

func isNew(e ast.Expr, set bool) bool {
	e = rw.Unparen(e)
	if id, ok := e.(*ast.Ident); ok && id.Name == "nil" {
		return true
	}
	name, call := rw.CallName(e)
	if (name == "icast_Map" || name == "icast_Set") && len(call.Args) == 1 {
		if n2, c2 := rw.CallName(call.Args[0]); n2 == "obj_" && len(c2.Args) == 1 {
			name, call = rw.CallName(c2.Args[0])
		}
	}
	if set {
		return name == "new_LinkedHashSet" && len(call.Args) == 0
	}
	return name == "new_LinkedHashMap" && len(call.Args) == 0
}

func isNil(e ast.Expr) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	return ok && id.Name == "nil"
}

// keyKind reports whether Go's == on t is Java's equality for keys of this type.
func keyOK(t types.Type, hashed *types.Interface) bool {
	switch u := types.Unalias(t).(type) {
	case *types.Basic:
		return u.Kind() == types.String
	case *types.Named:
		_, isIface := u.Underlying().(*types.Interface)
		return !isIface && rw.HasMethod(u, "box_")
	case *types.Pointer:
		if _, ok := u.Elem().(*types.Named); ok {
			return !types.Implements(u, hashed)
		}
	}
	return false
}

func isString(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Kind() == types.String
}

// access rewrites one operation. recv is the variable expression, m the Java method, args the
// arguments after the receiver; strKey says that the key argument is already an unboxed string.
func (c *cand) access(call *ast.CallExpr, recv ast.Expr, m string, args []ast.Expr, strKey bool) {
	V := func() string { return rw.Render(recv) }
	key := func() string {
		if strKey {
			return rw.Render(args[0])
		}
		return rw.Unboxed(c.kc, c.key, args[0])
	}
	_, isStmt := rw.Up(call).(*ast.ExprStmt)
	set := c.valExpr == nil
	switch {
	case m == "size" && len(args) == 0:
		c.edit(call, func() string { return "int32(len(" + V() + "))" })
	case m == "isEmpty" && len(args) == 0:
		c.edit(call, func() string { return "(len(" + V() + ") == 0)" })
	case m == "clear" && len(args) == 0 && isStmt:
		c.edit(call, func() string { return "clear(" + V() + ")" })
	case m == "remove" && len(args) == 1 && isStmt:
		c.edit(call, func() string { return "delete(" + V() + ", " + key() + ")" })
	case !set && m == "get" && len(args) == 1:
		c.edits = append(c.edits, rw.Read(c.vc, call, func() string {
			if isString(c.val) {
				return "mapStr_(" + V() + ", " + key() + ")"
			}
			return V() + "[" + key() + "]"
		}))
	case !set && m == "put" && len(args) == 2 && isStmt:
		c.edit(call, func() string { return V() + "[" + key() + "] = " + rw.Unboxed(c.vc, c.val, args[1]) })
	case !set && m == "containsKey" && len(args) == 1, set && m == "contains" && len(args) == 1:
		c.edit(call, func() string { return "has_(" + V() + ", " + key() + ")" })
	case set && m == "add" && len(args) == 1:
		if isStmt {
			c.edit(call, func() string { return V() + "[" + key() + "] = struct{}{}" })
		} else {
			c.edit(call, func() string { return "setAdd_(" + V() + ", " + key() + ")" })
		}
	default:
		what := "method " + m
		if !isStmt && (m == "put" || m == "remove" || m == "clear") {
			what += " used as a value"
		}
		c.fail(what)
	}
}

var helpers = map[string]string{"Map_get_s": "get", "Map_put_s": "put", "Map_containsKey_s": "containsKey", "Map_remove_s": "remove",
	"Collection_contains_s": "contains", "Collection_add_s": "add", "Collection_remove_s": "remove"}

func (c *cand) use(v ast.Expr) {
	switch p := rw.Up(v).(type) {
	case *ast.AssignStmt:
		if len(p.Lhs) == 1 && rw.Unparen(p.Lhs[0]) == v && p.Tok == token.ASSIGN {
			if !isNew(p.Rhs[0], c.valExpr == nil) {
				c.fail("assigned something other than a new LinkedHashMap/LinkedHashSet")
			} else if !isNil(p.Rhs[0]) {
				c.edit(p.Rhs[0], func() string { return c.goType() + "{}" })
			}
			return
		}
		c.fail("used as a value in an assignment")
	case *ast.SelectorExpr:
		call, ok := rw.Up(p).(*ast.CallExpr)
		if !ok || rw.Unparen(call.Fun) != p || rw.Unparen(p.X) != v {
			c.fail("field or method value taken")
			return
		}
		c.access(call, v, p.Sel.Name, call.Args, false)
	case *ast.CallExpr:
		name, _ := rw.CallName(p)
		if m, ok := helpers[name]; ok && len(p.Args) >= 2 && rw.Unparen(p.Args[0]) == v {
			if !isString(c.key) {
				c.fail("string-key helper on a non-string key")
				return
			}
			c.access(p, v, m, p.Args[1:], true)
			return
		}
		if name == "any" {
			c.fail("passed on as a Map/Set (converted to any)")
			return
		}
		if name == "" {
			name = "a method"
		}
		c.fail("argument of a call")
	case *ast.ReturnStmt:
		c.fail("returned")
	case *ast.BinaryExpr:
		c.fail("compared")
	default:
		c.fail(fmt.Sprintf("other use (%T)", p))
	}
}

func main() {
	report := len(os.Args) > 1 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	hashed := rw.Pkg.Scope().Lookup("hashed_").Type().Underlying().(*types.Interface)
	var order []*cand
	add := func(names []*ast.Ident, te ast.Expr) {
		var k, v ast.Expr
		if a := rw.AliasArgs(te, "MapOf", "LinkedHashMapOf"); len(a) == 2 {
			k, v = a[0], a[1]
		} else if a := rw.AliasArgs(te, "SetOf", "LinkedHashSetOf"); len(a) == 1 {
			k = a[0]
		} else {
			return
		}
		for _, id := range names {
			if o, ok := rw.Info.Defs[id].(*types.Var); ok {
				c := &cand{obj: o, typeExpr: te, keyExpr: k, valExpr: v, key: rw.Info.Types[k].Type}
				c.kc = rw.ConvFor(c.key)
				if v != nil {
					c.val = rw.Info.Types[v].Type
					c.vc = rw.ConvFor(c.val)
				}
				order = append(order, c)
			}
		}
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.StructType:
				for _, fl := range x.Fields.List {
					add(fl.Names, fl.Type)
				}
			case *ast.ValueSpec:
				if x.Type != nil {
					if _, isDecl := rw.Parent[rw.Parent[x]].(*ast.DeclStmt); isDecl && len(x.Names) == 1 {
						add(x.Names, x.Type)
					}
				}
			}
			return true
		})
	}
	for _, c := range order {
		if !keyOK(c.key, hashed) || !c.kc.OK {
			c.reason = "key type is compared by equals, or is not a string, handle or class"
			continue
		}
		if c.valExpr != nil && !c.vc.OK {
			c.reason = "value type has no known boxing"
			continue
		}
		for id, o := range rw.Info.Defs {
			if o != c.obj {
				continue
			}
			if spec, ok := rw.Parent[id].(*ast.ValueSpec); ok && len(spec.Values) == 1 {
				if !isNew(spec.Values[0], c.valExpr == nil) {
					c.fail("initialized with something other than a new LinkedHashMap/LinkedHashSet")
				} else if !isNil(spec.Values[0]) {
					c.edit(spec.Values[0], func() string { return c.goType() + "{}" })
				}
			}
		}
		c.edit(c.typeExpr, c.goType)
		for _, id := range rw.UsesOf[c.obj] {
			c.use(rw.VarExpr(id))
		}
	}
	reasons := map[string]int{}
	n := 0
	for _, c := range order {
		if c.reason != "" {
			reasons[c.reason]++
			continue
		}
		n++
		for _, e := range c.edits {
			if _, dup := rw.Rewrite[e.Node]; !dup {
				rw.Rewrite[e.Node] = e.Gen
			}
		}
		if report {
			p := rw.Fset.Position(c.obj.Pos())
			fmt.Printf("map\t%s:%d\t%s\t%s\n", filepath.Base(p.Filename), p.Line, c.obj.Name(), c.goType())
		}
	}
	fmt.Fprintf(os.Stderr, "maps and sets declared with element types: %d; become Go maps: %d\n", len(order), n)
	var rs []string
	for r := range reasons {
		rs = append(rs, r)
	}
	sort.Slice(rs, func(i, j int) bool { return reasons[rs[i]] > reasons[rs[j]] })
	for _, r := range rs {
		fmt.Fprintf(os.Stderr, "  stays (%d): %s\n", reasons[r], r)
	}
	if !report {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.Save())
	}
}
