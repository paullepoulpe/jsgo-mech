package main

// castfold -instof: an instanceof that guards an if statement is a comma-ok type assertion, and
// the casts of the same value that it guards use the asserted value.
//
// j2go writes `if (v instanceof X) { X t = (X) v; ... }` and the pattern-matching form as two
// helper calls, each an assertion:
//
//	if is_[*X](v) {                       if p, ok := v.(*X); ok {
//		var t *X = cast_[*X](v)      ->       ...p...
//
// For a class with subclasses, is_X asserts to X_as (the interface of as_X, see casts -named)
// and to_X(v) is p.as_X(). The rewrites, for an if statement without an init statement whose
// condition is exactly is_K(v) or !is_K(v):
//   - positive: `if p, ok := v.(T); ok {`, T the type is_K asserts to; in its body (not its
//     else branch) every cast of v to the same type is p (cast_[T](v), icast_I(v) for T = I) or
//     p.as_X() (to_X(v) for T = X_as). When the body starts with `var t T = <cast of v>` (or
//     t := ...), t is the bound name and the declaration goes. Without such casts the value is
//     dropped: `if _, ok := v.(T); ok {`.
//   - negative, when the body ends the block (return, panic, break, continue) and casts of v
//     follow in the same block: `p, ok := v.(T)` before it and `if !ok {`; the casts that follow
//     are p. Otherwise `if _, ok := v.(T); !ok {`.
// The casts run only when the assertion succeeded, so v is not nil and the cast's value is the
// asserted one: cast_ and icast_ assert v to the same type, and to_X calls as_X on it as p.as_X()
// does. Casts are bound only for v a local variable or parameter that nothing assigns after its
// declaration, so v holds the same value at the cast as at the test. New names are chosen so
// that they appear nowhere else in the enclosing function. An instanceof inside a larger
// condition (&&, ||, a return value) is left: Go has no expression form of a type test.

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"

	"postflip/rw"
)

// assertedBy returns the type expression is-helper call c asserts its argument to, and its type.
func assertedBy(c *ast.CallExpr) (string, types.Type) {
	switch f := c.Fun.(type) {
	case *ast.IndexExpr:
		return rw.Text(f.Index), rw.Info.TypeOf(f.Index)
	case *ast.Ident:
		fn, ok := rw.Info.Uses[f].(*types.Func)
		if !ok {
			return "", nil
		}
		d := funcDecl(fn)
		if d == nil || d.Body == nil || len(d.Body.List) != 2 {
			return "", nil
		}
		as, ok := d.Body.List[0].(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 2 || len(as.Rhs) != 1 || rw.Text(as.Lhs[0]) != "_" {
			return "", nil
		}
		ta, ok := as.Rhs[0].(*ast.TypeAssertExpr)
		if !ok || ta.Type == nil || rw.Text(ta.X) != d.Type.Params.List[0].Names[0].Name {
			return "", nil
		}
		if ret, ok := d.Body.List[1].(*ast.ReturnStmt); !ok || len(ret.Results) != 1 || rw.Text(ret.Results[0]) != rw.Text(as.Lhs[1]) {
			return "", nil
		}
		switch t := ta.Type.(type) {
		case *ast.Ident:
		case *ast.StarExpr:
			if _, ok := t.X.(*ast.Ident); !ok {
				return "", nil
			}
		default:
			return "", nil // an anonymous interface (runtime helpers)
		}
		return rw.Text(ta.Type), rw.Info.TypeOf(ta.Type)
	}
	return "", nil
}

// castValue returns, for a cast c of v, the expression of its value given p := v.(t), or "".
func castValue(c *ast.CallExpr, v *types.Var, t types.Type) func(p string) string {
	kind, target := castOf(c)
	if kind != "to" && kind != "cast" && kind != "icast" || len(c.Args) != 1 {
		return nil
	}
	id, ok := rw.Unparen(c.Args[0]).(*ast.Ident)
	if !ok || rw.Info.Uses[id] != v {
		return nil
	}
	if types.Identical(target, t) {
		return func(p string) string { return p }
	}
	if kind == "to" {
		x := classOf(target)
		if nt, ok := types.Unalias(t).(*types.Named); ok && x != nil && nt.Obj().Name() == x.Obj().Name()+"_as" {
			return func(p string) string { return p + ".as_" + x.Obj().Name() + "()" }
		}
	}
	return nil
}

type names struct{ used map[string]bool }

func namesIn(n ast.Node) *names {
	u := map[string]bool{}
	ast.Inspect(n, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok {
			u[id.Name] = true
		}
		return true
	})
	return &names{u}
}

func (ns *names) fresh(base string) string {
	name := base
	for i := 2; ns.used[name]; i++ {
		name = fmt.Sprintf("%s%d", base, i)
	}
	ns.used[name] = true
	return name
}

func terminates(b *ast.BlockStmt) bool {
	if len(b.List) == 0 {
		return false
	}
	switch s := b.List[len(b.List)-1].(type) {
	case *ast.ReturnStmt, *ast.BranchStmt:
		return true
	case *ast.ExprStmt:
		if c, ok := s.X.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "panic" && rw.Info.Uses[id] == types.Universe.Lookup("panic") {
				return true
			}
		}
	}
	return false
}

func instof(dry bool) {
	stats := map[string]int{}
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil {
				continue
			}
			ns := namesIn(fd)
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				c, ok := n.(*ast.CallExpr)
				if !ok {
					return true
				}
				if kind, _ := castOf(c); kind != "is" && kind != "is_" {
					return true
				}
				// the if statement whose condition is c or !c
				cond := ast.Node(c)
				neg := false
				if u, ok := rw.Up(c).(*ast.UnaryExpr); ok && u.Op == token.NOT {
					cond, neg = u, true
				}
				is, ok := rw.Up(cond).(*ast.IfStmt)
				if !ok || is.Init != nil || rw.Unparen(is.Cond) != cond {
					stats["instanceof left: inside a larger expression"]++
					return true
				}
				ttext, t := assertedBy(c)
				if t == nil {
					stats["instanceof left: helper asserts to an anonymous interface (runtime)"]++
					return true
				}
				operand := c.Args[0]
				var v *types.Var
				if id, ok := rw.Unparen(operand).(*ast.Ident); ok {
					if lv, ok := rw.Info.Uses[id].(*types.Var); ok && !lv.IsField() && lv.Parent() != rw.Pkg.Scope() && !assigned(lv) {
						v = lv
					}
				}
				// the casts the test guards
				type bound struct {
					c   *ast.CallExpr
					val func(string) string
				}
				var casts []bound
				collect := func(n ast.Node) {
					if v == nil {
						return
					}
					ast.Inspect(n, func(x ast.Node) bool {
						if cc, ok := x.(*ast.CallExpr); ok {
							if val := castValue(cc, v, t); val != nil {
								casts = append(casts, bound{cc, val})
							}
						}
						return true
					})
				}
				opText := func() string { return rw.Render(operand) }
				okName := ns.fresh("ok")
				if !neg {
					collect(is.Body)
					if len(casts) == 0 {
						stats["instanceof: if _, ok := v.(T); ok"]++
						rw.Rewrite[cond] = func() string { return fmt.Sprintf("_, %s := %s.(%s); %s", okName, opText(), ttext, okName) }
						return true
					}
					p := ""
					// var t T = cast(v) or t := cast(v) first in the body, with T the asserted type
					if len(is.Body.List) > 0 {
						var lhs *ast.Ident
						var rhs ast.Expr
						switch s := is.Body.List[0].(type) {
						case *ast.AssignStmt:
							if s.Tok == token.DEFINE && len(s.Lhs) == 1 && len(s.Rhs) == 1 {
								lhs, _ = s.Lhs[0].(*ast.Ident)
								rhs = s.Rhs[0]
							}
						case *ast.DeclStmt:
							if gd, ok := s.Decl.(*ast.GenDecl); ok && len(gd.Specs) == 1 {
								if vs, ok := gd.Specs[0].(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 1 {
									lhs, rhs = vs.Names[0], vs.Values[0]
								}
							}
						}
						if lhs != nil && rw.Unparen(rhs) == casts[0].c && casts[0].val("") == "" {
							lv := rw.Info.Defs[lhs].(*types.Var)
							if types.Identical(lv.Type(), t) && !assigned(lv) && !usedOutside(lhs.Name, is.Body, is) {
								p = lhs.Name
								rw.Rewrite[is.Body.List[0]] = func() string { return rw.Del }
								casts = casts[1:]
								stats["casts bound to the asserted value"]++
							}
						}
					}
					if p == "" {
						p = ns.fresh("p")
					}
					for _, b := range casts {
						val := b.val
						rw.Rewrite[b.c] = func() string { return val(p) }
					}
					stats["instanceof: if p, ok := v.(T); ok (casts bound)"]++
					stats["casts bound to the asserted value"] += len(casts)
					rw.Rewrite[cond] = func() string { return fmt.Sprintf("%s, %s := %s.(%s); %s", p, okName, opText(), ttext, okName) }
					return true
				}
				// negative: the casts after the if statement in its block
				blk, ok := rw.Parent[is].(*ast.BlockStmt)
				if ok && is.Else == nil && terminates(is.Body) {
					after := false
					for _, s := range blk.List {
						if after {
							collect(s)
						}
						after = after || s == is
					}
				} else if c2, ok := rw.Parent[is].(*ast.CaseClause); ok && is.Else == nil && terminates(is.Body) {
					after := false
					for _, s := range c2.Body {
						if after {
							collect(s)
						}
						after = after || s == is
					}
				}
				if len(casts) == 0 {
					stats["instanceof: if _, ok := v.(T); !ok"]++
					rw.Rewrite[cond] = func() string { return fmt.Sprintf("_, %s := %s.(%s); !%s", okName, opText(), ttext, okName) }
					return true
				}
				p := ns.fresh("p")
				for _, b := range casts {
					val := b.val
					rw.Rewrite[b.c] = func() string { return val(p) }
				}
				stats["instanceof: p, ok := v.(T); if !ok (casts after it bound)"]++
				stats["casts bound to the asserted value"] += len(casts)
				body := is.Body
				rw.Rewrite[is] = func() string {
					return fmt.Sprintf("%s, %s := %s.(%s)\nif !%s %s", p, okName, opText(), ttext, okName, rw.Render(body))
				}
				return true
			})
		}
	}
	for _, k := range sortedKeys(stats) {
		fmt.Fprintf(os.Stderr, "castfold -instof: %5d %s\n", stats[k], k)
	}
	if dry {
		return
	}
	fmt.Fprintf(os.Stderr, "castfold -instof: %d files written\n", rw.SaveChecked())
}

// usedOutside reports whether an identifier named name appears in n outside inner.
func usedOutside(name string, inner, n ast.Node) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if x == inner {
			return false
		}
		if id, ok := x.(*ast.Ident); ok && id.Name == name {
			found = true
		}
		return !found
	})
	return found
}
