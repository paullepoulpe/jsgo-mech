package main

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"postflip/rw"
)

// optMethods are the methods of Optional[string] the rewrite has a string form for.
var optMethods = map[string]bool{"isPresent": true, "isEmpty": true, "get": true, "orElseThrow": true, "orElse": true, "filterFn_": true, "box_": true}

// fnOf is the package-level function a call's Fun names (through an instantiation), and its
// type arguments.
func fnOf(fun ast.Expr) (*types.Func, *ast.Ident) {
	var id *ast.Ident
	switch f := rw.Unparen(fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.IndexExpr:
		id, _ = f.X.(*ast.Ident)
	case *ast.IndexListExpr:
		id, _ = f.X.(*ast.Ident)
	}
	if id == nil {
		return nil, nil
	}
	fn, _ := rw.Info.Uses[id].(*types.Func)
	return fn, id
}

// strInst reports whether ident id (a generic function) is instantiated with string.
func strInst(id *ast.Ident) bool {
	inst, ok := rw.Info.Instances[id]
	return ok && inst.TypeArgs.Len() == 1 && types.Identical(inst.TypeArgs.At(0), types.Typ[types.String])
}

func declaredInGenerated(o types.Object) bool {
	p := rw.Fset.Position(o.Pos()).Filename
	return strings.HasPrefix(p[strings.LastIndex(p, "/")+1:], "g_")
}

var optFuncs = map[string]bool{"Optional_of": true, "Optional_ofNullable": true, "Optional_empty": true, "optOf_": true, "optEquals_": true, "optEqualsNull_": true, "optHash_": true}

// checkExprs checks that every value of the type is made and used in a form the rewrite keeps.
func checkExprs() bool {
	ok := true
	fail := func(n ast.Node, why string) {
		leave(n, why)
		ok = false
	}
	for e, tv := range rw.Info.Types {
		if !generated(e) || !tv.IsValue() || !isOptStr(tv.Type) {
			continue
		}
		// how the value is made
		switch x := e.(type) {
		case *ast.ParenExpr, *ast.Ident, *ast.SelectorExpr:
		case *ast.IndexExpr:
			id, isId := x.X.(*ast.Ident)
			if v, isV := rw.Info.Uses[id].(*types.Var); !isId || !isV || v.Parent() != rw.Pkg.Scope() {
				fail(x, "read of a container of Optionals")
			}
		case *ast.CallExpr:
			if _, isFL := rw.Unparen(x.Fun).(*ast.FuncLit); isFL {
				break
			}
			if fn, id := fnOf(x.Fun); fn != nil {
				switch {
				case optFuncs[fn.Name()]:
					if !strInst(id) {
						fail(x, "Optional function of another instantiation")
					} else if fn.Name() == "optOf_" && !unboxOK[x] {
						fail(x, "cast of an Object to Optional<String> not proved non-null")
					}
				case declaredInGenerated(fn):
				case runtimeOpt[fn.Name()]:
				default:
					fail(x, "runtime function returning Optional<String> ("+fn.Name()+")")
				}
				break
			}
			if sel, isSel := x.Fun.(*ast.SelectorExpr); isSel {
				s := rw.Info.Selections[sel]
				if s != nil && s.Kind() == types.MethodVal {
					if isOptStr(rw.Info.TypeOf(sel.X)) {
						if sel.Sel.Name != "filterFn_" {
							fail(x, "Optional method "+sel.Sel.Name)
						}
					} else if !declaredInGenerated(s.Obj()) {
						fail(x, "runtime method returning Optional<String> ("+sel.Sel.Name+")")
					}
					break
				}
			}
			// a call of a func value: its functions are the package's (their results are checked)
		default:
			fail(e, "made otherwise ("+short(e)+")")
		}
		// how the value is used
		var cur ast.Node = e
		for {
			if p, isP := rw.Parent[cur].(*ast.ParenExpr); isP {
				cur = p
				continue
			}
			break
		}
		switch p := rw.Parent[cur].(type) {
		case *ast.SelectorExpr:
			s := rw.Info.Selections[p]
			if s == nil || s.Kind() != types.MethodVal || !optMethods[p.Sel.Name] {
				fail(p, "selector ."+p.Sel.Name+" of an Optional")
			} else if c, isC := rw.Parent[p].(*ast.CallExpr); !isC || c.Fun != ast.Expr(p) {
				fail(p, "method value of an Optional")
			}
		case *ast.CallExpr:
			if p.Fun == cur {
				break
			}
			if fn, id := fnOf(p.Fun); fn != nil && (fn.Name() == "optEquals_" || fn.Name() == "optEqualsNull_" || fn.Name() == "optHash_") {
				if !strInst(id) {
					fail(p, "Optional helper of another instantiation")
				}
				break
			}
			sig, isSig := types.Unalias(rw.Info.TypeOf(p.Fun)).Underlying().(*types.Signature)
			if !isSig {
				fail(p, "conversion of an Optional")
				break
			}
			for i, a := range p.Args {
				if a != cur {
					continue
				}
				var pt types.Type
				if sig.Variadic() && i >= sig.Params().Len()-1 {
					pt = sig.Params().At(sig.Params().Len() - 1).Type().(*types.Slice).Elem()
					if p.Ellipsis.IsValid() {
						pt = sig.Params().At(sig.Params().Len() - 1).Type()
					}
				} else {
					pt = sig.Params().At(i).Type()
				}
				if !isOptStr(pt) {
					fail(p, "passed as "+types.TypeString(pt, types.RelativeTo(rw.Pkg)))
				}
			}
		case *ast.AssignStmt:
			for i, r := range p.Rhs {
				if r == cur && len(p.Lhs) == len(p.Rhs) {
					if id, isId := p.Lhs[i].(*ast.Ident); isId && id.Name == "_" {
						continue
					}
					if !isOptStr(rw.Info.TypeOf(p.Lhs[i])) {
						fail(p, "assigned to another type")
					}
				}
			}
		case *ast.ValueSpec:
			if p.Type == nil || !isOptStr(rw.Info.TypeOf(p.Type)) {
				fail(p, "declared with another type")
			}
		case *ast.ReturnStmt:
			if !returnsOpt(p, cur) {
				fail(p, "returned as another type")
			}
		case *ast.ExprStmt, *ast.IndexExpr:
			// a discarded call result; an index of the enum array is checked as a value above
		default:
			fail(cur, "used as "+short(rw.Parent[cur]))
		}
	}
	// runtime functions of the type used other than called as above
	for id, o := range rw.Info.Uses {
		fn, isF := o.(*types.Func)
		if !isF || !generated(id) || declaredInGenerated(fn) {
			continue
		}
		t := rw.Info.TypeOf(id)
		if t == nil || !contains(t) {
			continue
		}
		var call *ast.CallExpr
		var cur ast.Node = id
		if ix, isIx := rw.Parent[id].(*ast.IndexExpr); isIx && ix.X == ast.Expr(id) {
			cur = ix
		}
		if s, isS := rw.Parent[id].(*ast.SelectorExpr); isS && s.Sel == id {
			cur = s
		}
		call, _ = rw.Parent[cur].(*ast.CallExpr)
		if call == nil || call.Fun != cur {
			fail(id, "runtime function of the type used as a value ("+id.Name+")")
		}
	}
	return ok
}

// returnsOpt: the result e returns is of type Optional[string] in its function's signature.
func returnsOpt(r *ast.ReturnStmt, e ast.Node) bool {
	var sig *types.Signature
	for p := rw.Parent[r]; p != nil; p = rw.Parent[p] {
		if fl, ok := p.(*ast.FuncLit); ok {
			sig, _ = rw.Info.TypeOf(fl).(*types.Signature)
			break
		}
		if fd, ok := p.(*ast.FuncDecl); ok {
			sig = rw.Info.Defs[fd.Name].Type().(*types.Signature)
			break
		}
	}
	if sig == nil {
		return false
	}
	for i, x := range r.Results {
		if x == e && i < sig.Results().Len() {
			return isOptStr(sig.Results().At(i).Type())
		}
	}
	return false
}

// needParens: a comparison standing in for node n needs parentheses there.
func needParens(n ast.Node) bool {
	switch p := rw.Parent[n].(type) {
	case *ast.BinaryExpr:
		return p.Op != token.LAND && p.Op != token.LOR
	case *ast.UnaryExpr, *ast.StarExpr, *ast.SelectorExpr, *ast.IndexExpr:
		return true
	case *ast.CallExpr:
		return p.Fun == n
	}
	return false
}

func cmp(n ast.Node, a, op, b string) string {
	s := a + " " + op + " " + b
	if needParens(n) {
		return "(" + s + ")"
	}
	return s
}

func isStrLit(e ast.Expr) bool {
	l, ok := rw.Unparen(e).(*ast.BasicLit)
	return ok && l.Kind == token.STRING
}

func isNullstr(e ast.Expr) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	return ok && id.Name == "nullstr_"
}

func rewrite() {
	// type spellings, but the kept map's
	skip := map[ast.Node]bool{}
	for v := range mapKept {
		for id, o := range rw.Info.Defs {
			if o == v {
				if f, ok := rw.Parent[id].(*ast.Field); ok {
					ast.Inspect(f.Type, func(n ast.Node) bool {
						skip[n] = true
						return true
					})
					left["type spellings kept: an erased map holding Optional objects (its toString prints them as Optional[...])"] = append(left["type spellings kept: an erased map holding Optional objects (its toString prints them as Optional[...])"], rw.Fset.Position(id.Pos()).String())
				}
			}
		}
	}
	inner := map[ast.Node]bool{} // calls a rewrite of their `!` replaces
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.IndexExpr:
				if tv, ok := rw.Info.Types[x]; ok && tv.IsType() && isOptStr(tv.Type) && !skip[x] {
					done["Optional[string] -> string"]++
					rw.Rewrite[x] = func() string { return "string" }
					return false
				}
			case *ast.UnaryExpr:
				if x.Op != token.NOT {
					return true
				}
				c, ok := rw.Unparen(x.X).(*ast.CallExpr)
				if !ok {
					return true
				}
				if sel, ok := c.Fun.(*ast.SelectorExpr); ok && isOptStr(rw.Info.TypeOf(sel.X)) && (sel.Sel.Name == "isPresent" || sel.Sel.Name == "isEmpty") {
					op := "=="
					if sel.Sel.Name == "isEmpty" {
						op = "!="
					}
					done["!o."+sel.Sel.Name+"() -> o "+op+" nullstr_"]++
					inner[c] = true
					rw.Rewrite[x] = func() string { return cmp(x, rw.Render(sel.X), op, "nullstr_") }
					return true
				}
				if fn, id := fnOf(c.Fun); fn != nil && strInst(id) && (fn.Name() == "optEquals_" || fn.Name() == "optEqualsNull_") {
					done["!"+fn.Name()+"(a, b) -> a != b"]++
					inner[c] = true
					rw.Rewrite[x] = func() string { return cmp(x, rw.Render(c.Args[0]), "!=", rw.Render(c.Args[1])) }
				}
			case *ast.CallExpr:
				if inner[x] {
					return true
				}
				if fn, id := fnOf(x.Fun); fn != nil {
					switch {
					case optFuncs[fn.Name()] && strInst(id):
						name := fn.Name()
						switch name {
						case "Optional_of":
							if isStrLit(x.Args[0]) {
								done["Optional_of[string](literal) -> literal"]++
								rw.Rewrite[x] = func() string { return rw.Render(x.Args[0]) }
							} else {
								done["Optional_of[string](x) -> optstrOf_(x)"]++
								rw.Rewrite[x] = func() string { return "optstrOf_(" + rw.Render(x.Args[0]) + ")" }
							}
						case "Optional_ofNullable":
							done["Optional_ofNullable[string](x) -> x"]++
							rw.Rewrite[x] = func() string { return rw.Render(x.Args[0]) }
						case "Optional_empty":
							done["Optional_empty[string]() -> nullstr_"]++
							rw.Rewrite[x] = func() string { return "nullstr_" }
						case "optEquals_", "optEqualsNull_":
							done[name+"(a, b) -> a == b"]++
							rw.Rewrite[x] = func() string { return cmp(x, rw.Render(x.Args[0]), "==", rw.Render(x.Args[1])) }
						case "optHash_":
							done["optHash_(a) -> optstrHash_(a)"]++
							rw.Rewrite[x] = func() string { return "optstrHash_(" + rw.Render(x.Args[0]) + ")" }
						case "optOf_":
							done["optOf_[string](v) -> optstrUnbox_(v)"]++
							rw.Rewrite[x] = func() string { return "optstrUnbox_(" + rw.Render(x.Args[0]) + ")" }
						}
					case runtimeOpt[fn.Name()]:
						if p, ok := rw.Up(x).(*ast.CallExpr); ok {
							if name, _ := rw.CallName(p); name == "optstr_" {
								return true // a rerun
							}
						}
						done[fn.Name()+"(..) -> optstr_("+fn.Name()+"(..))"]++
						rw.Rewrite[x] = func() string { return "optstr_(" + rw.RenderChildren(x) + ")" }
					}
					return true
				}
				sel, ok := x.Fun.(*ast.SelectorExpr)
				if !ok || !isOptStr(rw.Info.TypeOf(sel.X)) {
					return true
				}
				o := sel.X
				switch sel.Sel.Name {
				case "isPresent":
					done["o.isPresent() -> o != nullstr_"]++
					rw.Rewrite[x] = func() string { return cmp(x, rw.Render(o), "!=", "nullstr_") }
				case "isEmpty":
					done["o.isEmpty() -> o == nullstr_"]++
					rw.Rewrite[x] = func() string { return cmp(x, rw.Render(o), "==", "nullstr_") }
				case "get", "orElseThrow":
					done["o."+sel.Sel.Name+"() -> optstrGet_(o)"]++
					rw.Rewrite[x] = func() string { return "optstrGet_(" + rw.Render(o) + ")" }
				case "orElse":
					if isNullstr(x.Args[0]) {
						done["o.orElse(nullstr_) -> o"]++
						rw.Rewrite[x] = func() string { return rw.Render(o) }
					} else {
						done["o.orElse(y) -> optstrOr_(o, y)"]++
						rw.Rewrite[x] = func() string { return "optstrOr_(" + rw.Render(o) + ", " + rw.Render(x.Args[0]) + ")" }
					}
				case "filterFn_":
					done["o.filterFn_(p) -> optstrFilter_(o, p)"]++
					rw.Rewrite[x] = func() string { return "optstrFilter_(" + rw.Render(o) + ", " + rw.Render(x.Args[0]) + ")" }
				case "box_":
					done["o.box_() -> optstrBox_(o)"]++
					rw.Rewrite[x] = func() string { return "optstrBox_(" + rw.Render(o) + ")" }
				}
			}
			return true
		})
	}
}
