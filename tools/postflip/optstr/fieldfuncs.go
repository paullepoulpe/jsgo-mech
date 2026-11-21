package main

import (
	"go/ast"
	"go/token"
	"go/types"

	"postflip/rw"
)

// Func-typed struct fields in the call graph (cha.go). A call x.f(...) whose callee is a struct
// field f of func type runs the value stored in that field. A field's value is its zero (nil:
// the call panics), a value some statement of the package stored in the field (an assignment
// to a selector of f, or an element of a composite literal of the struct), or a copy of the
// same field of another struct value (a whole-struct assignment, Object_clone). So when every
// store to f in the package (generated code and runtime) stores nil, a function literal, or a
// function or method declared in the package and named as a value, and the field's address is never taken, the
// call has edges to exactly those functions (for a method value of an interface, to every
// implementing method). When some store has another form (a variable, a parameter, a call
// result), or the field is assigned in a multi-value assignment or a range clause, the field is
// open and the call is matched by signature like any other call of a func value.
//
// Bound adapters. The translator passes a typed lambda where erased code wants a method as
// `(&A{lit}).m`: the method value of a fresh object of an adapter struct A whose func field
// holds the lambda. All adapters of one A share the field, so by the rule above a call of the
// field in m could run any lambda ever wrapped in an A. But the receiver of this method value
// is the object the literal makes and nothing else refers to it; when the field is never
// assigned outside composite literals and its address is never taken (it is immutable once
// made), a call `this.f(...)` on m's receiver, reached through this method value, runs the
// function that literal put in f. The value stored is then m bound to that literal (boundFn):
// the same body with its receiver's fields resolved from the literal. A call of the field on any
// other expression inside m, and m reached any other way, follow the rule above.

type fieldStore struct {
	srcs     []ast.Node
	open     bool
	assigned bool // stored by an assignment (not only by composite literals), or its address taken
}

// boundFn is a method reached through the method value of a composite literal: its receiver's
// func fields hold what the literal gave them.
type boundFn struct {
	*ast.FuncDecl
	recv types.Object
	bind map[*types.Var][]ast.Node
}

var boundFns = map[ast.Expr]*boundFn{}

// litFields: the func-typed fields a composite literal sets, with the functions it gives them;
// ok is false when one of them is given something else than a known function.
func litFields(cl *ast.CompositeLit) (map[*types.Var][]ast.Node, bool) {
	t := rw.Info.TypeOf(cl)
	if t == nil {
		return nil, false
	}
	st, ok := types.Unalias(t).Underlying().(*types.Struct)
	if !ok {
		return nil, false
	}
	out := map[*types.Var][]ast.Node{}
	for i, el := range cl.Elts {
		var f *types.Var
		val := el
		if kv, ok := el.(*ast.KeyValueExpr); ok {
			id, ok := kv.Key.(*ast.Ident)
			if !ok {
				return nil, false
			}
			f, _ = rw.Info.Uses[id].(*types.Var)
			val = kv.Value
		} else if i < st.NumFields() {
			f = st.Field(i)
		}
		if f == nil {
			return nil, false
		}
		if !isFuncType(f.Type()) {
			continue
		}
		srcs, ok := valueSources(val)
		if !ok {
			return nil, false
		}
		out[f.Origin()] = srcs
	}
	return out, true
}

var (
	fieldStores      = map[*types.Var]*fieldStore{}
	fieldStoresBuilt bool
)

func isFuncType(t types.Type) bool {
	if t == nil {
		return false
	}
	_, ok := types.Unalias(t).Underlying().(*types.Signature)
	return ok
}

func storeOf(f *types.Var) *fieldStore {
	f = f.Origin()
	s := fieldStores[f]
	if s == nil {
		s = &fieldStore{}
		fieldStores[f] = s
	}
	return s
}

// storeValue records that e is stored in f.
func storeValue(f *types.Var, e ast.Expr) {
	s := storeOf(f)
	srcs, ok := valueSources(e)
	if !ok {
		s.open = true
		return
	}
	s.srcs = append(s.srcs, srcs...)
}

// valueSources: the functions the func-typed expression e can be, when e is nil, a function
// literal or a declared function or method named as a value (possibly converted to a func type).
func valueSources(e ast.Expr) ([]ast.Node, bool) {
	e = rw.Unparen(e)
	for {
		c, ok := e.(*ast.CallExpr)
		if !ok || len(c.Args) != 1 {
			break
		}
		tv, ok := rw.Info.Types[c.Fun]
		if !ok || !tv.IsType() || !isFuncType(tv.Type) {
			break
		}
		e = rw.Unparen(c.Args[0]) // a conversion to a func type
	}
	switch x := e.(type) {
	case *ast.FuncLit:
		return []ast.Node{x}, true
	case *ast.Ident:
		if _, ok := rw.Info.Uses[x].(*types.Nil); ok {
			return nil, true
		}
	}
	var id *ast.Ident
	switch x := e.(type) {
	case *ast.Ident:
		id = x
	case *ast.SelectorExpr:
		id = x.Sel
	case *ast.IndexExpr:
		switch y := rw.Unparen(x.X).(type) {
		case *ast.Ident:
			id = y
		case *ast.SelectorExpr:
			id = y.Sel
		}
	}
	if id == nil {
		return nil, false
	}
	fn, ok := rw.Info.Uses[id].(*types.Func)
	if !ok {
		return nil, false
	}
	if fn.Pkg() != rw.Pkg {
		return nil, false // a function of another package: not followed here
	}
	if r := fn.Type().(*types.Signature).Recv(); r != nil {
		if _, isI := r.Type().Underlying().(*types.Interface); isI {
			return implementers(fn), true
		}
	}
	fd := funcDecls[fn.Origin()]
	if fd == nil {
		return nil, false
	}
	// (&A{lit}).m: m bound to the literal's object
	if sel, ok := e.(*ast.SelectorExpr); ok && fd.Recv != nil && len(fd.Recv.List) == 1 && len(fd.Recv.List[0].Names) == 1 {
		x := rw.Unparen(sel.X)
		if u, ok := x.(*ast.UnaryExpr); ok && u.Op == token.AND {
			x = rw.Unparen(u.X)
		}
		if cl, ok := x.(*ast.CompositeLit); ok {
			if b, ok := boundFns[e]; ok {
				return []ast.Node{b}, true
			}
			if bind, ok := litFields(cl); ok && len(bind) > 0 {
				b := &boundFn{FuncDecl: fd, recv: rw.Info.Defs[fd.Recv.List[0].Names[0]], bind: bind}
				boundFns[e] = b
				return []ast.Node{b}, true
			}
		}
	}
	return []ast.Node{fd}, true
}

func buildFieldStores() {
	if fieldStoresBuilt {
		return
	}
	fieldStoresBuilt = true
	fieldOf := func(e ast.Expr) *types.Var {
		sel, ok := rw.Unparen(e).(*ast.SelectorExpr)
		if !ok {
			return nil
		}
		sn := rw.Info.Selections[sel]
		if sn == nil || sn.Kind() != types.FieldVal {
			return nil
		}
		v, _ := sn.Obj().(*types.Var)
		if v == nil || !isFuncType(v.Type()) {
			return nil
		}
		return v
	}
	for _, file := range rw.Files {
		ast.Inspect(file, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				for i, l := range x.Lhs {
					if f := fieldOf(l); f != nil {
						storeOf(f).assigned = true
						if len(x.Lhs) == len(x.Rhs) && (x.Tok == token.ASSIGN || x.Tok == token.DEFINE) {
							storeValue(f, x.Rhs[i])
						} else {
							storeOf(f).open = true
						}
					}
				}
			case *ast.RangeStmt:
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if e != nil {
						if f := fieldOf(e); f != nil {
							storeOf(f).open = true
							storeOf(f).assigned = true
						}
					}
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					if f := fieldOf(x.X); f != nil {
						storeOf(f).open = true
						storeOf(f).assigned = true
					}
				}
			case *ast.CompositeLit:
				t := rw.Info.TypeOf(x)
				if t == nil {
					return true
				}
				st, ok := types.Unalias(t).Underlying().(*types.Struct)
				if !ok {
					return true
				}
				for i, el := range x.Elts {
					var f *types.Var
					val := el
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						if id, ok := kv.Key.(*ast.Ident); ok {
							f, _ = rw.Info.Uses[id].(*types.Var)
						}
						val = kv.Value
						if f == nil {
							// a key that names no field: every func field of the struct is open
							for j := 0; j < st.NumFields(); j++ {
								if isFuncType(st.Field(j).Type()) {
									storeOf(st.Field(j)).open = true
								}
							}
							continue
						}
					} else if i < st.NumFields() {
						f = st.Field(i)
					}
					if f != nil && isFuncType(f.Type()) {
						storeValue(f, val)
					}
				}
			}
			return true
		})
	}
}

// fieldCallees: the functions a call of the func-typed field selected by sel can run, if every
// store to the field is known.
func fieldCallees(sel *ast.SelectorExpr, root ast.Node) ([]ast.Node, bool) {
	sn := rw.Info.Selections[sel]
	if sn == nil || sn.Kind() != types.FieldVal {
		return nil, false
	}
	f, _ := sn.Obj().(*types.Var)
	if f == nil || !isFuncType(f.Type()) {
		return nil, false
	}
	buildFieldStores()
	s := fieldStores[f.Origin()]
	if b, ok := root.(*boundFn); ok && b.recv != nil {
		// the field of the bound receiver itself, immutable once made
		if id, ok := rw.Unparen(sel.X).(*ast.Ident); ok && rw.Info.Uses[id] == b.recv {
			if srcs, ok := b.bind[f.Origin()]; ok && (s == nil || !s.assigned) {
				return srcs, true
			}
		}
	}
	if s == nil {
		return nil, true // never stored: nil
	}
	if s.open {
		return nil, false
	}
	return s.srcs, true
}
