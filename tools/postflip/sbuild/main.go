// Command sbuild is a post-flip step: StringBuilders that only the translated code sees are
// strBuilder_ values, a strings.Builder of the appended bytes.
//
// j2go translates `StringBuilder sb = new StringBuilder()` as a pointer to the runtime's
// StringBuilder, which keeps Java's UTF-16 units (bytes while everything is ASCII) and copies
// them into a new string at toString. strBuilder_ (rt_strbuf.go) appends the strings' bytes to
// a strings.Builder and returns them without a copy; it has every StringBuilder method that the
// translated code calls, with the same results (UTF-16 lengths and indexes: it counts units as
// they are appended, reads by index on its byte fast path while everything is ASCII, and for
// what it cannot do on bytes, or a string that is not valid UTF-8, it continues as a
// StringBuilder holding what was appended so far).
//
// The step works on classes of slots, as the collection steps do: the local variables,
// parameters, results and fields of type *StringBuilder, joined when a value flows from one to
// another (an assignment or initialization, an argument, a return, a field of a composite
// literal, a comparison), the parameters and results of a method with those of the interface
// methods it implements, and a new builder with the slot it goes to. A class becomes
// *strBuilder_ as a whole when every value entering it is a new builder, nil or another slot
// of it, and every use of a value of it is a method call (append...; a call's result that is
// the builder again, through j2go's cast_[*StringBuilder], continues the chain), a read as a
// string (Object_toString(obj_(sb)) and the like: sb.toString(), the builder is never null),
// a comparison, or a flow into a slot of the class. A builder boxed into an Object (an
// Appendable for a code printer, a CharSequence), passed to the runtime, stored in a
// collection, or a parameter of a function value keeps its class a StringBuilder.
//
// A local variable that only receives new builders and is only passed to parameters (or used
// in place) is the builder itself: `var sb strBuilder_`, passed as &sb; a reset (sb = new
// StringBuilder()) then is sb = strBuilder_{} when the builder is never passed on. Every other
// slot is a *strBuilder_, and new builders are &strBuilder_{}, strBuilderCap_(n) or
// strBuilderOf_(s).
//
// usage: sbuild [-n] <dir of package closure>
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

var (
	sbPtr     types.Type // *StringBuilder
	fastPtr   types.Type // *strBuilder_
	fastMeths *types.MethodSet
)

// key is a node of the union-find: a *types.Var (slot) or an *ast.CallExpr (a new builder).
type key = any

var (
	parent  = map[key]key{}
	blocked = map[key]string{} // a member's reason for keeping its class a StringBuilder
)

func find(k key) key {
	p, ok := parent[k]
	if !ok {
		parent[k] = k
		return k
	}
	if p == k {
		return k
	}
	r := find(p)
	parent[k] = r
	return r
}

func union(a, b key) {
	ra, rb := find(a), find(b)
	if ra != rb {
		parent[ra] = rb
	}
}

func block(k key, why string) {
	if _, ok := blocked[k]; !ok {
		blocked[k] = why
	}
	find(k)
}

// slot facts
var (
	isSlot    = map[*types.Var]bool{}
	declType  = map[*types.Var]ast.Expr{}       // the declared type expression
	localDecl = map[*types.Var]ast.Stmt{}       // a local's declaration statement
	localInit = map[*types.Var]ast.Expr{}       // its initial value
	isLocal   = map[*types.Var]bool{}           //
	results   = map[ast.Node]*types.Signature{} // FuncDecl / FuncLit -> its signature
	ifaceMeth = map[*types.Func]bool{}          // interface methods with slots
)

func slotVar(o types.Object) (*types.Var, bool) {
	v, ok := o.(*types.Var)
	if !ok || !isSlot[v] {
		return nil, false
	}
	return v, true
}

func isG(n ast.Node) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "g_")
}

func main() {
	rw.Load(os.Args[len(os.Args)-1])
	report := os.Args[1] == "-n"
	sbPtr = types.NewPointer(rw.Pkg.Scope().Lookup("StringBuilder").Type())
	fo := rw.Pkg.Scope().Lookup("strBuilder_")
	if fo == nil {
		fmt.Fprintln(os.Stderr, "the runtime has no strBuilder_ (rt_strbuf.go)")
		os.Exit(1)
	}
	fastPtr = types.NewPointer(fo.Type())
	fastMeths = types.NewMethodSet(fastPtr)
	collect()
	joinMethods()
	helpers()
	for _, f := range rw.Files {
		flows(f)
		uses(f)
	}
	convert(report)
}

func isSB(t types.Type) bool { return t != nil && types.Identical(t, sbPtr) }

// fieldVars maps the fields of a field list to the variables of tuple (or struct) members.
func fieldList(fl *ast.FieldList, vars func(i int) *types.Var, g bool, why string) {
	if fl == nil {
		return
	}
	i := 0
	for _, f := range fl.List {
		n := len(f.Names)
		if n == 0 {
			n = 1
		}
		var first *types.Var
		for j := 0; j < n; j++ {
			v := vars(i)
			i++
			if v == nil || !isSB(v.Type()) {
				continue
			}
			isSlot[v] = true
			declType[v] = f.Type
			find(v)
			if !g {
				block(v, "declared in the runtime")
			}
			if why != "" {
				block(v, why)
			}
			if first != nil {
				union(first, v) // one type expression for both
			}
			first = v
		}
	}
}

// collect finds the slots and their declarations.
func collect() {
	for _, f := range rw.Files {
		g := isG(f)
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				fn, ok := rw.Info.Defs[x.Name].(*types.Func)
				if !ok {
					return true
				}
				sig := fn.Type().(*types.Signature)
				results[x] = sig
				fieldList(x.Type.Params, func(i int) *types.Var { return sig.Params().At(i) }, g, "")
				fieldList(x.Type.Results, func(i int) *types.Var { return sig.Results().At(i) }, g, "")
			case *ast.FuncLit:
				sig, ok := rw.Info.Types[x].Type.(*types.Signature)
				if !ok {
					return true
				}
				results[x] = sig
				fieldList(x.Type.Params, func(i int) *types.Var { return sig.Params().At(i) }, g, "a parameter of a function value")
				fieldList(x.Type.Results, func(i int) *types.Var { return sig.Results().At(i) }, g, "a result of a function value")
			case *ast.InterfaceType:
				for _, m := range x.Methods.List {
					if len(m.Names) != 1 {
						continue
					}
					fn, ok := rw.Info.Defs[m.Names[0]].(*types.Func)
					ft, ok2 := m.Type.(*ast.FuncType)
					if !ok || !ok2 {
						continue
					}
					sig := fn.Type().(*types.Signature)
					fieldList(ft.Params, func(i int) *types.Var { return sig.Params().At(i) }, g, "")
					fieldList(ft.Results, func(i int) *types.Var { return sig.Results().At(i) }, g, "")
					ifaceMeth[fn] = true
				}
			case *ast.StructType:
				var vs []*types.Var
				for _, fd := range x.Fields.List {
					for _, nm := range fd.Names {
						v, _ := rw.Info.Defs[nm].(*types.Var)
						vs = append(vs, v)
					}
					if len(fd.Names) == 0 {
						vs = append(vs, nil)
					}
				}
				fieldList(x.Fields, func(i int) *types.Var { return vs[i] }, g, "")
			case *ast.ValueSpec:
				for i, nm := range x.Names {
					v, ok := rw.Info.Defs[nm].(*types.Var)
					if !ok || !isSB(v.Type()) {
						continue
					}
					isSlot[v] = true
					find(v)
					if x.Type != nil {
						declType[v] = x.Type
					}
					if !g {
						block(v, "declared in the runtime")
					}
					if ds, ok := rw.Parent[x].(*ast.GenDecl); ok {
						if st, ok := rw.Parent[ds].(*ast.DeclStmt); ok {
							isLocal[v] = true
							if len(x.Names) == 1 {
								localDecl[v] = st
							}
						} else {
							block(v, "a package-level variable")
						}
					}
					if i < len(x.Values) && len(x.Names) == len(x.Values) {
						localInit[v] = x.Values[i]
					}
					if len(x.Names) > 1 {
						union(v, rw.Info.Defs[x.Names[0]].(*types.Var))
					}
				}
			case *ast.AssignStmt:
				if x.Tok != token.DEFINE {
					return true
				}
				for i, l := range x.Lhs {
					id, ok := l.(*ast.Ident)
					if !ok {
						continue
					}
					v, ok := rw.Info.Defs[id].(*types.Var)
					if !ok || !isSB(v.Type()) {
						continue
					}
					isSlot[v] = true
					isLocal[v] = true
					find(v)
					if !g {
						block(v, "declared in the runtime")
					}
					if len(x.Lhs) == 1 && len(x.Rhs) == 1 {
						localDecl[v] = x
						localInit[v] = x.Rhs[0]
					} else if len(x.Lhs) != len(x.Rhs) {
						block(v, "declared from a call with several results")
					} else {
						localInit[v] = x.Rhs[i]
					}
				}
			}
			return true
		})
	}
}

// joinMethods joins the slots of interface methods with those of the methods implementing
// them.
func joinMethods() {
	var named []*types.Named
	sc := rw.Pkg.Scope()
	for _, n := range sc.Names() {
		tn, ok := sc.Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		if nt, ok := tn.Type().(*types.Named); ok {
			named = append(named, nt)
		}
	}
	var ims []*types.Func
	for m := range ifaceMeth {
		ims = append(ims, m)
	}
	sort.Slice(ims, func(i, j int) bool { return ims[i].Pos() < ims[j].Pos() })
	for _, im := range ims {
		recv := im.Type().(*types.Signature).Recv()
		if recv == nil {
			continue
		}
		it, ok := recv.Type().Underlying().(*types.Interface)
		if !ok {
			continue
		}
		for _, nt := range named {
			var impl types.Type
			if nt.TypeParams() != nil {
				if types.NewMethodSet(types.NewPointer(nt)).Lookup(rw.Pkg, im.Name()) != nil {
					// A generic type with a method of the name: its slots are joined when its
					// signature has slots where the interface method has them.
					if m := lookup(types.NewPointer(nt), im.Name()); sameShape(im, m) {
						joinSigs(im, m)
					}
				}
				continue
			}
			switch {
			case types.Implements(nt, it):
				impl = nt
			case types.Implements(types.NewPointer(nt), it):
				impl = types.NewPointer(nt)
			default:
				continue
			}
			joinSigs(im, lookup(impl, im.Name()))
		}
	}
}

func lookup(t types.Type, name string) *types.Func {
	o, _, _ := types.LookupFieldOrMethod(t, true, rw.Pkg, name)
	f, _ := o.(*types.Func)
	return f
}

// sameShape reports whether a and b have as many parameters and results, slots at the same
// positions.
func sameShape(a, b *types.Func) bool {
	if a == nil || b == nil {
		return false
	}
	sa, sb := a.Origin().Type().(*types.Signature), b.Origin().Type().(*types.Signature)
	same := func(x, y *types.Tuple) bool {
		if x.Len() != y.Len() {
			return false
		}
		for i := 0; i < x.Len(); i++ {
			if isSB(x.At(i).Type()) != isSB(y.At(i).Type()) {
				return false
			}
		}
		return true
	}
	return same(sa.Params(), sb.Params()) && same(sa.Results(), sb.Results())
}

// joinSigs joins the slots of two methods' signatures, position by position.
func joinSigs(a, b *types.Func) {
	if a == nil || b == nil {
		return
	}
	sa, sb := a.Origin().Type().(*types.Signature), b.Origin().Type().(*types.Signature)
	join := func(x, y *types.Tuple) {
		for i := 0; i < x.Len() && i < y.Len(); i++ {
			if isSlot[x.At(i)] && isSlot[y.At(i)] {
				union(x.At(i), y.At(i))
			} else {
				for _, v := range []*types.Var{x.At(i), y.At(i)} {
					if isSlot[v] {
						block(v, "an implementation of "+a.Name()+" has another type there")
					}
				}
			}
		}
	}
	join(sa.Params(), sb.Params())
	join(sa.Results(), sb.Results())
}

// callee returns the function object a call calls, if it is a static function or a method.
func callee(c *ast.CallExpr) *types.Func {
	switch f := rw.Unparen(c.Fun).(type) {
	case *ast.Ident:
		fn, _ := rw.Info.Uses[f].(*types.Func)
		return fn
	case *ast.SelectorExpr:
		if s := rw.Info.Selections[f]; s != nil {
			if s.Kind() == types.MethodVal {
				fn, _ := s.Obj().(*types.Func)
				return fn
			}
			return nil
		}
		fn, _ := rw.Info.Uses[f.Sel].(*types.Func)
		return fn
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			fn, _ := rw.Info.Uses[id].(*types.Func)
			return fn
		}
	}
	return nil
}

func paramVar(fn *types.Func, i int) *types.Var {
	sig := fn.Origin().Type().(*types.Signature)
	if sig.Variadic() && i >= sig.Params().Len()-1 {
		return nil
	}
	if i >= sig.Params().Len() {
		return nil
	}
	return sig.Params().At(i)
}

func resultVar(fn *types.Func) *types.Var {
	sig := fn.Origin().Type().(*types.Signature)
	if sig.Results().Len() != 1 {
		return nil
	}
	return sig.Results().At(0)
}

// variants are the runtime methods taking a *StringBuilder that have a *strBuilder_ variant
// (rt_strbuf.go), by receiver type and name: the parameter, and the call, follow the builder.
var variants = map[string]string{"Joiner.appendTo": "appendToStr_"}

// variant is the variant of the method call c and the index of its builder parameter.
func variant(c *ast.CallExpr) (string, int) {
	fn := callee(c)
	if fn == nil {
		return "", -1
	}
	sig := fn.Type().(*types.Signature)
	if sig.Recv() == nil {
		return "", -1
	}
	rt := sig.Recv().Type()
	if p, ok := rt.(*types.Pointer); ok {
		rt = p.Elem()
	}
	nt, ok := types.Unalias(rt).(*types.Named)
	if !ok {
		return "", -1
	}
	v := variants[nt.Obj().Name()+"."+fn.Name()]
	if v == "" {
		return "", -1
	}
	for i := 0; i < sig.Params().Len(); i++ {
		if isSB(sig.Params().At(i).Type()) {
			return v, i
		}
	}
	return "", -1
}

var variantCalls = map[key][]*ast.CallExpr{}

var newCtors = map[string]bool{"new_StringBuilder": true, "new_StringBuilder_int": true, "new_StringBuilder_String": true}

// origin is what a builder expression is: a slot, a new builder (its call), nil, or unknown.
func origin(e ast.Expr) (key, bool) {
	e = rw.Unparen(e)
	switch x := e.(type) {
	case *ast.Ident:
		if x.Name == "nil" {
			return nil, true
		}
		if v, ok := slotVar(rw.Info.Uses[x]); ok {
			return v, true
		}
	case *ast.SelectorExpr:
		if v, ok := slotVar(rw.Info.Uses[x.Sel]); ok {
			if s := rw.Info.Selections[x]; s != nil && s.Kind() == types.FieldVal {
				return v, true
			}
		}
	case *ast.CallExpr:
		if name, _ := rw.CallName(x); newCtors[name] {
			return x, true
		}
		if name, _ := rw.CallName(x); strings.HasPrefix(name, "cast_[*StringBuilder]") && len(x.Args) == 1 {
			return origin(x.Args[0])
		}
		if s, ok := rw.Unparen(x.Fun).(*ast.SelectorExpr); ok && chaining(s) {
			return origin(s.X)
		}
		if fn := callee(x); fn != nil {
			if v := resultVar(fn); v != nil && isSlot[v] {
				return v, true
			}
		}
	}
	return nil, false
}

// chaining reports whether s, the method selector of a call, is a builder method that returns
// the builder, called on a builder expression.
func chaining(s *ast.SelectorExpr) bool {
	if _, ok := origin(s.X); !ok || isNil(s.X) {
		return false
	}
	sel := fastMeths.Lookup(rw.Pkg, s.Sel.Name)
	if sel == nil {
		return false
	}
	r := sel.Obj().Type().(*types.Signature).Results()
	return r.Len() == 1 && types.Identical(r.At(0).Type(), fastPtr)
}

func isNil(e ast.Expr) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	return ok && id.Name == "nil"
}

// flows records the flows into slots: assignments, initializations, arguments, returns and
// composite literal fields.
func flows(f *ast.File) {
	into := func(t *types.Var, e ast.Expr, where string) {
		o, ok := origin(e)
		switch {
		case !ok:
			block(t, "gets a builder from "+describe(e)+" ("+where+")")
		case o != nil:
			union(o, t)
		}
	}
	ast.Inspect(f, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			if len(x.Lhs) == len(x.Rhs) {
				for i, l := range x.Lhs {
					var v *types.Var
					switch l := rw.Unparen(l).(type) {
					case *ast.Ident:
						if o, ok := rw.Info.Defs[l].(*types.Var); ok && isSlot[o] {
							v = o
						} else if o, ok := slotVar(rw.Info.Uses[l]); ok {
							v = o
						}
					case *ast.SelectorExpr:
						v, _ = slotVar(rw.Info.Uses[l.Sel])
					}
					if v != nil {
						into(v, x.Rhs[i], "assignment")
					}
				}
			}
		case *ast.ValueSpec:
			for i, nm := range x.Names {
				if v, ok := rw.Info.Defs[nm].(*types.Var); ok && isSlot[v] && i < len(x.Values) {
					into(v, x.Values[i], "initialization")
				}
			}
		case *ast.ReturnStmt:
			sig := enclosing(x)
			if sig != nil && sig.Results().Len() == len(x.Results) {
				for i, r := range x.Results {
					if v := sig.Results().At(i); isSlot[v] {
						into(v, r, "return")
					}
				}
			}
		case *ast.CallExpr:
			_, vi := variant(x)
			if fn := callee(x); fn != nil {
				for i, a := range x.Args {
					if i == vi {
						continue
					}
					if v := paramVar(fn, i); v != nil && isSlot[v] {
						into(v, a, "argument")
					}
				}
			}
		case *ast.KeyValueExpr:
			if id, ok := x.Key.(*ast.Ident); ok {
				if v, ok := slotVar(rw.Info.Uses[id]); ok {
					into(v, x.Value, "composite literal")
				}
			}
		}
		return true
	})
}

func describe(e ast.Expr) string {
	e = rw.Unparen(e)
	if name, c := rw.CallName(e); c != nil {
		if name != "" {
			return name
		}
		if s, ok := c.Fun.(*ast.SelectorExpr); ok {
			return "." + s.Sel.Name + "()"
		}
	}
	return fmt.Sprintf("%T", e)
}

// per-slot facts gathered from uses
var (
	argUses   = map[*types.Var][]*ast.Ident{} // a local passed as an argument
	otherFlow = map[*types.Var]bool{}         // a local used as a value otherwise (assigned, returned, compared)
	asStrings = map[key][]*ast.CallExpr{}     // reads as a string, by origin
	casts     = map[key][]*ast.CallExpr{}     // cast_[*StringBuilder] calls of a chain, by origin
	news      []*ast.CallExpr                 // new builders
)

// uses checks what is done with every builder expression.
func uses(f *ast.File) {
	g := isG(f)
	ast.Inspect(f, func(n ast.Node) bool {
		var k key
		var root ast.Expr
		switch x := n.(type) {
		case *ast.Ident:
			if fn, ok := rw.Info.Uses[x].(*types.Func); ok {
				funcValue(fn, x)
				return true
			}
			v, ok := slotVar(rw.Info.Uses[x])
			if !ok {
				return true
			}
			if s, ok := rw.Parent[x].(*ast.SelectorExpr); ok && s.Sel == x {
				root = s
			} else {
				root = x
			}
			k = v
			if kv, ok := rw.Parent[x].(*ast.KeyValueExpr); ok && kv.Key == x {
				return true
			}
		case *ast.CallExpr:
			name, _ := rw.CallName(x)
			if newCtors[name] {
				k, root = x, x
				news = append(news, x)
			} else if fn := callee(x); fn != nil {
				if v := resultVar(fn); v != nil && isSlot[v] {
					k, root = v, x
				}
			}
			if k == nil {
				return true
			}
		default:
			return true
		}
		if !g {
			block(k, "used in the runtime")
			return true
		}
		context(k, root)
		return true
	})
}

// funcValue blocks the slots of fn's signature when fn, named by id, is used as a value (not
// called): calls of function values are not followed.
func funcValue(fn *types.Func, id *ast.Ident) {
	var e ast.Expr = id
	if s, ok := rw.Parent[id].(*ast.SelectorExpr); ok && s.Sel == id {
		e = s
	}
	if ix, ok := rw.Up(e).(*ast.IndexExpr); ok && ix.X == e {
		e = ix
	}
	if c, ok := rw.Up(e).(*ast.CallExpr); ok && rw.Unparen(c.Fun) == e {
		return
	}
	sig := fn.Origin().Type().(*types.Signature)
	for _, t := range []*types.Tuple{sig.Params(), sig.Results()} {
		for i := 0; i < t.Len(); i++ {
			if isSlot[t.At(i)] {
				block(t.At(i), "a parameter or result of a function used as a value")
			}
		}
	}
}

// context checks the use of builder expression e, of origin k.
func context(k key, e ast.Expr) {
	v, _ := k.(*types.Var)
	local := v != nil && isLocal[v] && e == ast.Expr(rootIdent(e))
	for {
		p := rw.Up(e)
		switch x := p.(type) {
		case *ast.SelectorExpr:
			if rw.Unparen(x.X) != e {
				block(k, "used as a selector")
				return
			}
			c, isCall := rw.Up(x).(*ast.CallExpr)
			sel := fastMeths.Lookup(rw.Pkg, x.Sel.Name)
			if !isCall || rw.Unparen(c.Fun) != ast.Expr(x) || sel == nil {
				block(k, "calls "+x.Sel.Name)
				return
			}
			r := sel.Obj().Type().(*types.Signature).Results()
			if r.Len() == 1 && types.Identical(r.At(0).Type(), fastPtr) {
				e = c // the builder again
				local = false
				continue
			}
			return
		case *ast.CallExpr:
			name, _ := rw.CallName(x)
			if strings.HasPrefix(name, "cast_[*StringBuilder]") {
				casts[k] = append(casts[k], x)
				e = x
				local = false
				continue
			}
			if (name == "obj_" || name == "any") && len(x.Args) == 1 {
				if s := asString(x); s != nil {
					asStrings[k] = append(asStrings[k], s)
					return
				}
				if csReader(x) {
					// Read as a CharSequence (csString_: its toString), which a boxed
					// *strBuilder_ has; a local builder is boxed by its address.
					if local {
						argUses[v] = append(argUses[v], rootIdent(e))
					}
					return
				}
				if h := helperArg(x); h != nil {
					// An Appendable for a helper function that has (or gets) a *strBuilder_
					// variant.
					helperCalls[k] = append(helperCalls[k], helperCall{h, x, e})
					if local {
						argUses[v] = append(argUses[v], rootIdent(e))
					}
					return
				}
				block(k, "boxed ("+describe(rw.Up(x).(ast.Expr))+")")
				return
			}
			if _, vi := variant(x); vi >= 0 && vi < len(x.Args) && rw.Unparen(x.Args[vi]) == e {
				if _, stmt := rw.Up(x).(*ast.ExprStmt); !stmt {
					block(k, "passed to a runtime method whose result is used")
					return
				}
				variantCalls[k] = append(variantCalls[k], x)
				if local {
					argUses[v] = append(argUses[v], rootIdent(e))
				}
				return
			}
			if fn := callee(x); fn != nil {
				// tostr_/tostrNN_ (the tostr step's String.valueOf / toString) are generic over
				// any comparable type with toString(): a *strBuilder_ passes unchanged.
				if (fn.Name() == "tostr_" || fn.Name() == "tostrNN_" || fn.Name() == "tostrR_" || fn.Name() == "jtostr_") && len(x.Args) == 1 && rw.Unparen(x.Args[0]) == e {
					return
				}
				for i, a := range x.Args {
					if rw.Unparen(a) == e {
						if pv := paramVar(fn, i); pv != nil && isSlot[pv] {
							if local {
								argUses[v] = append(argUses[v], rootIdent(e))
							} else if v != nil {
								otherFlow[v] = true
							}
							return
						}
						block(k, "passed to "+fn.Name())
						return
					}
				}
			}
			block(k, "passed to "+describe(x))
			return
		case *ast.ExprStmt:
			return
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				if rw.Unparen(l) == e {
					return // assigned to
				}
			}
			for i, r := range x.Rhs {
				if rw.Unparen(r) == e && len(x.Lhs) == len(x.Rhs) && targetSlot(x.Lhs[i]) {
					if v != nil {
						otherFlow[v] = true
					}
					return
				}
			}
			block(k, "assigned to a "+describe(x.Lhs[0]))
			return
		case *ast.ValueSpec:
			for i, val := range x.Values {
				if rw.Unparen(val) == e && i < len(x.Names) {
					if o, ok := rw.Info.Defs[x.Names[i]].(*types.Var); ok && isSlot[o] {
						if v != nil {
							otherFlow[v] = true
						}
						return
					}
				}
			}
			block(k, "initializes a variable of another type")
			return
		case *ast.ReturnStmt:
			// flows() checked the result slot; a result of another type is blocked there or
			// here.
			if v != nil {
				otherFlow[v] = true
			}
			if !returnsSlot(x, e) {
				block(k, "returned as another type")
			}
			return
		case *ast.KeyValueExpr:
			if id, ok := x.Key.(*ast.Ident); ok {
				if _, ok := slotVar(rw.Info.Uses[id]); ok && x.Value == e {
					if v != nil {
						otherFlow[v] = true
					}
					return
				}
			}
			block(k, "in a composite literal")
			return
		case *ast.BinaryExpr:
			if x.Op == token.EQL || x.Op == token.NEQ {
				other := x.X
				if rw.Unparen(other) == e {
					other = x.Y
				}
				if v != nil {
					otherFlow[v] = true
				}
				if isNil(other) {
					return
				}
				if o, ok := origin(other); ok && o != nil {
					union(k, o)
					return
				}
			}
			block(k, "in a binary expression")
			return
		default:
			block(k, fmt.Sprintf("used as %T", p))
			return
		}
	}
}

func rootIdent(e ast.Expr) *ast.Ident {
	id, _ := rw.Unparen(e).(*ast.Ident)
	return id
}

func targetSlot(l ast.Expr) bool {
	switch l := rw.Unparen(l).(type) {
	case *ast.Ident:
		if o, ok := rw.Info.Defs[l].(*types.Var); ok && isSlot[o] {
			return true
		}
		_, ok := slotVar(rw.Info.Uses[l])
		return ok
	case *ast.SelectorExpr:
		_, ok := slotVar(rw.Info.Uses[l.Sel])
		return ok
	}
	return false
}

// enclosing is the signature of the function declaration or literal that n is in.
func enclosing(n ast.Node) *types.Signature {
	for n = rw.Parent[n]; n != nil; n = rw.Parent[n] {
		switch n.(type) {
		case *ast.FuncDecl, *ast.FuncLit:
			return results[n]
		}
	}
	return nil
}

func returnsSlot(r *ast.ReturnStmt, e ast.Expr) bool {
	sig := enclosing(r)
	if sig == nil {
		return false
	}
	for i, x := range r.Results {
		if rw.Unparen(x) == e && i < sig.Results().Len() {
			return isSlot[sig.Results().At(i)]
		}
	}
	return false
}

// asString is the call that reads builder box b (obj_(sb)) as a string: Object_toString(b),
// String_valueOf_Object(b) or objstr_(b) (the builder is never null).
func asString(b *ast.CallExpr) *ast.CallExpr {
	p, ok := rw.Up(b).(*ast.CallExpr)
	if !ok || len(p.Args) != 1 || rw.Unparen(p.Args[0]) != ast.Expr(b) {
		return nil
	}
	switch name, _ := rw.CallName(p); name {
	case "Object_toString", "String_valueOf_Object", "objstr_", "tostrR_": // reflip16
		return p
	}
	return nil
}

// csReaders are the methods that take a CharSequence or an Object only to read it as a string
// (csString_ or objstr_, which call its toString): StringBuilder's and strBuilder_'s appends,
// and Hasher.putUnencodedChars (rt_base_hash.go, the one implementation).
var csReaders = map[string]bool{"append_CharSequence": true, "append_CharSequence_int_int": true,
	"append_Object": true, "putUnencodedChars": true}

// csReader reports whether box b (obj_(sb)) is the first argument of a csReaders method; for
// the appends, the receiver is a builder.
func csReader(b *ast.CallExpr) bool {
	c, ok := rw.Up(b).(*ast.CallExpr)
	if !ok || len(c.Args) == 0 || rw.Unparen(c.Args[0]) != ast.Expr(b) {
		return false
	}
	s, ok := rw.Unparen(c.Fun).(*ast.SelectorExpr)
	if !ok || !csReaders[s.Sel.Name] {
		return false
	}
	if s.Sel.Name == "putUnencodedChars" {
		return true
	}
	t := rw.Info.Types[s.X].Type
	return isSB(t) || (t != nil && types.Identical(t, fastPtr))
}

// Helper functions: a package-level function F with one Appendable parameter that it only
// appends to (calls of Appendable methods, which strBuilder_ has, and the parameter passed on
// to such helpers) gets a variant F_sb taking a *strBuilder_, a copy of F with that parameter
// type and the helpers it passes it to replaced by their variants. A builder boxed as an
// Appendable for F (icast_Appendable(obj_(sb))) then calls F_sb(sb).
type fparam struct {
	fn *types.Func
	i  int
}

var (
	appendableT types.Type
	qual        = map[fparam]bool{}
	fdecls      = map[*types.Func]*ast.FuncDecl{}
	helperCalls = map[key][]helperCall{}
)

type helperCall struct {
	call *ast.CallExpr // F(..., icast_Appendable(obj_(sb)), ...)
	box  *ast.CallExpr // obj_(sb)
	b    ast.Expr      // sb
}

// helpers finds the helper functions: the candidates, less those that use the parameter
// otherwise, until none changes.
func helpers() {
	if o := rw.Pkg.Scope().Lookup("Appendable"); o != nil {
		appendableT = o.Type()
	} else {
		return
	}
	for _, f := range rw.Files {
		if !isG(f) {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Type.TypeParams != nil || fd.Body == nil {
				continue
			}
			fn, ok := rw.Info.Defs[fd.Name].(*types.Func)
			if !ok {
				continue
			}
			sig := fn.Type().(*types.Signature)
			at := -1
			for i := 0; i < sig.Params().Len(); i++ {
				if types.Identical(sig.Params().At(i).Type(), appendableT) {
					if at >= 0 {
						at = -2
						break
					}
					at = i
				}
			}
			if at >= 0 {
				qual[fparam{fn, at}] = true
				fdecls[fn] = fd
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for fp := range qual {
			p := fp.fn.Type().(*types.Signature).Params().At(fp.i)
			for _, u := range rw.UsesOf[p] {
				if !appendUse(u) {
					delete(qual, fp)
					changed = true
					break
				}
			}
		}
	}
}

// appendUse reports whether the use e of a helper's Appendable parameter (or of an append
// chain on it) is one its variant can have.
func appendUse(e ast.Expr) bool {
	switch x := rw.Up(e).(type) {
	case *ast.ExprStmt:
		return true
	case *ast.SelectorExpr:
		c, ok := rw.Up(x).(*ast.CallExpr)
		if rw.Unparen(x.X) != e || !ok || rw.Unparen(c.Fun) != ast.Expr(x) {
			return false
		}
		sel := fastMeths.Lookup(rw.Pkg, x.Sel.Name)
		if sel == nil {
			return false
		}
		r := sel.Obj().Type().(*types.Signature).Results()
		if r.Len() == 1 && types.Identical(r.At(0).Type(), fastPtr) {
			return appendUse(c)
		}
		return true
	case *ast.CallExpr:
		fn := callee(x)
		if fn == nil {
			return false
		}
		for j, a := range x.Args {
			if rw.Unparen(a) == e {
				return qual[fparam{fn, j}]
			}
		}
	}
	return false
}

// helperArg is the helper call that box b (obj_(sb)), cast to an Appendable, is the
// Appendable argument of.
func helperArg(b *ast.CallExpr) *ast.CallExpr {
	ic, ok := rw.Up(b).(*ast.CallExpr)
	if !ok || len(ic.Args) != 1 || rw.Unparen(ic.Args[0]) != ast.Expr(b) {
		return nil
	}
	if name, _ := rw.CallName(ic); name != "icast_Appendable" {
		return nil
	}
	c, ok := rw.Up(ic).(*ast.CallExpr)
	if !ok {
		return nil
	}
	fn := callee(c)
	if fn == nil {
		return nil
	}
	for j, a := range c.Args {
		if rw.Unparen(a) == ast.Expr(ic) && qual[fparam{fn, j}] {
			return c
		}
	}
	return nil
}

// writeVariants writes the variants of the helpers that converted builders call, and of those
// they call. A helper that nothing else calls any more (its callers are the converted builders
// and the helpers being varied) is changed in place, keeping its name; the others get a copy
// named F_sb. It returns the number of helpers varied.
func writeVariants(need []fparam, sites map[*ast.Ident]bool) int {
	type edit struct {
		a, b token.Pos
		s    func() string
	}
	done := map[fparam]bool{}
	var order []fparam
	edits := map[*types.Func][]edit{}
	for len(need) > 0 {
		fp := need[0]
		need = need[1:]
		if done[fp] {
			continue
		}
		done[fp] = true
		order = append(order, fp)
		p := fp.fn.Type().(*types.Signature).Params().At(fp.i)
		fd := fdecls[fp.fn]
		k := 0
		for _, fl := range fd.Type.Params.List {
			for range max(1, len(fl.Names)) {
				if k == fp.i {
					edits[fp.fn] = append(edits[fp.fn], edit{fl.Type.Pos(), fl.Type.End(), func() string { return "*strBuilder_" }})
				}
				k++
			}
		}
		for _, u := range rw.UsesOf[p] {
			// The helpers the parameter is passed to (directly or as an append chain).
			var e ast.Expr = u
			for {
				if s, ok := rw.Up(e).(*ast.SelectorExpr); ok {
					e = rw.Up(s).(*ast.CallExpr)
					continue
				}
				break
			}
			if c, ok := rw.Up(e).(*ast.CallExpr); ok {
				fn := callee(c)
				for j, a := range c.Args {
					if rw.Unparen(a) == e && fn != nil && qual[fparam{fn, j}] {
						id := calleeIdent(c)
						sites[id] = true
						edits[fp.fn] = append(edits[fp.fn], edit{id.Pos(), id.End(), func() string { return variantName(fn) }})
						need = append(need, fparam{fn, j})
					}
				}
			}
		}
	}
	// In place: every use of the helper is a call being redirected.
	for _, fp := range order {
		inPlace[fp.fn] = true
	}
	for changed := true; changed; {
		changed = false
		for _, fp := range order {
			if !inPlace[fp.fn] {
				continue
			}
			for _, u := range rw.UsesOf[fp.fn] {
				if !sites[u] {
					inPlace[fp.fn] = false
					changed = true
					break
				}
			}
		}
	}
	n := 0
	for _, fp := range order {
		fn := fp.fn
		if !inPlace[fn] && rw.HasFunc(fn.Name()+"_sb") {
			continue // made by an earlier run
		}
		fd := fdecls[fn]
		es := edits[fn]
		if !inPlace[fn] {
			es = append(es, edit{fd.Name.Pos(), fd.Name.End(), func() string { return fn.Name() + "_sb" }})
		}
		sort.Slice(es, func(i, j int) bool { return es[i].a < es[j].a })
		text := func() string {
			src := rw.Text(fd)
			base := fd.Pos()
			var out strings.Builder
			last := 0
			for _, e := range es {
				out.WriteString(src[last : e.a-base])
				out.WriteString(e.s())
				last = int(e.b - base)
			}
			out.WriteString(src[last:])
			return out.String()
		}
		if inPlace[fn] {
			rw.Rewrite[fd] = text
		} else {
			rw.Rewrite[fd] = func() string {
				return rw.RenderChildren(fd) + "\n\n// " + fn.Name() + "_sb is " + fn.Name() + " appending to a *strBuilder_ (post-flip step sbuild).\n" + text()
			}
		}
		n++
	}
	return n
}

// inPlace: the helper is changed in place, not copied.
var inPlace = map[*types.Func]bool{}

func variantName(fn *types.Func) string {
	if inPlace[fn] {
		return fn.Name()
	}
	return fn.Name() + "_sb"
}

func calleeIdent(c *ast.CallExpr) *ast.Ident {
	switch f := rw.Unparen(c.Fun).(type) {
	case *ast.Ident:
		return f
	case *ast.SelectorExpr:
		return f.Sel
	}
	return nil
}

// convert decides the classes and rewrites the converted ones.
func convert(report bool) {
	members := map[key][]key{}
	var roots []key
	for k := range parent {
		r := find(k)
		if _, ok := members[r]; !ok {
			roots = append(roots, r)
		}
		members[r] = append(members[r], k)
	}
	pos := func(k key) token.Pos {
		switch x := k.(type) {
		case *types.Var:
			return x.Pos()
		case ast.Node:
			return x.Pos()
		}
		return 0
	}
	sort.Slice(roots, func(i, j int) bool { return pos(roots[i]) < pos(roots[j]) })
	why := map[string]int{}
	conv := map[key]bool{}
	nClasses, nSlots, nNew := 0, 0, 0
	for _, r := range roots {
		ms := members[r]
		sort.Slice(ms, func(i, j int) bool { return pos(ms[i]) < pos(ms[j]) })
		reason := ""
		for _, m := range ms {
			if b, ok := blocked[m]; ok {
				reason = b
				break
			}
		}
		if reason != "" {
			n := 0
			for _, m := range ms {
				if v, ok := m.(*types.Var); ok && isG(gNode(v)) {
					n++
				}
			}
			if n > 0 {
				why[reason] += n
				if os.Getenv("SBUILD_V") != "" {
					fmt.Fprintf(os.Stderr, "left: %s: %s (%d slots)\n", rw.Fset.Position(pos(r)), reason, n)
				}
			}
			continue
		}
		nClasses++
		for _, m := range ms {
			conv[m] = true
			if _, ok := m.(*types.Var); ok {
				nSlots++
			} else {
				nNew++
			}
		}
	}
	// Value locals: a local that only gets new builders and is only passed to parameters.
	value := map[*types.Var]bool{}
	for k := range conv {
		v, ok := k.(*types.Var)
		if !ok || !isLocal[v] || otherFlow[v] || localDecl[v] == nil {
			continue
		}
		st := localDecl[v]
		if _, inBlock := rw.Parent[st].(*ast.BlockStmt); !inBlock {
			if _, inCase := rw.Parent[st].(*ast.CaseClause); !inCase {
				continue
			}
		}
		init := localInit[v]
		if init == nil {
			continue
		}
		name, _ := rw.CallName(init)
		if !newCtors[name] {
			continue
		}
		ok = true
		resets := 0
		for _, u := range rw.UsesOf[v] {
			if as, isAs := rw.Parent[u].(*ast.AssignStmt); isAs && len(as.Lhs) == 1 && as.Lhs[0] == u {
				if n, _ := rw.CallName(as.Rhs[0]); n == "new_StringBuilder" {
					resets++
					continue
				}
				ok = false
			}
		}
		if ok && (resets == 0 || len(argUses[v]) == 0) {
			value[v] = true
		}
	}
	nValue := 0
	for k := range conv {
		switch x := k.(type) {
		case *types.Var:
			if value[x] {
				nValue++
				rewriteValueLocal(x)
				continue
			}
			if t := declType[x]; t != nil {
				rw.Rewrite[t] = func() string { return "*strBuilder_" }
			}
		}
	}
	for _, c := range news {
		if !conv[c] || rw.Rewrite[c] != nil {
			continue
		}
		c := c
		name, _ := rw.CallName(c)
		switch name {
		case "new_StringBuilder":
			rw.Rewrite[c] = func() string { return "&strBuilder_{}" }
			if s, ok := rw.Up(c).(*ast.SelectorExpr); ok && rw.Unparen(s.X) == ast.Expr(c) {
				rw.Rewrite[c] = func() string { return "(&strBuilder_{})" }
			}
		case "new_StringBuilder_int":
			rw.Rewrite[c] = func() string { return "strBuilderCap_(" + rw.Render(c.Args[0]) + ")" }
		case "new_StringBuilder_String":
			rw.Rewrite[c] = func() string { return "strBuilderOf_(" + rw.Render(c.Args[0]) + ")" }
		}
	}
	nCalls := 0
	for k, cs := range casts {
		if !conv[k] {
			continue
		}
		for _, c := range cs {
			c := c
			rw.Rewrite[c] = func() string { return rw.Render(c.Args[0]) }
		}
	}
	for k, cs := range asStrings {
		if !conv[k] {
			continue
		}
		for _, c := range cs {
			c := c
			b := rw.Unparen(c.Args[0]).(*ast.CallExpr).Args[0]
			rw.Rewrite[c] = func() string { return rw.Render(b) + ".toString()" }
			nCalls++
		}
	}
	for k, cs := range variantCalls {
		if !conv[k] {
			continue
		}
		for _, c := range cs {
			name, _ := variant(c)
			sel := rw.Unparen(c.Fun).(*ast.SelectorExpr).Sel
			rw.Rewrite[sel] = func() string { return name }
			nCalls++
		}
	}
	var need []fparam
	sites := map[*ast.Ident]bool{}
	for k, hs := range helperCalls {
		if !conv[k] {
			continue
		}
		for _, h := range hs {
			h := h
			fn := callee(h.call)
			id := calleeIdent(h.call)
			sites[id] = true
			rw.Rewrite[id] = func() string { return variantName(fn) }
			ic := rw.Up(h.box).(*ast.CallExpr)
			rw.Rewrite[ic] = func() string { return rw.Render(h.b) }
			for j, a := range h.call.Args {
				if rw.Unparen(a) == ast.Expr(ic) {
					need = append(need, fparam{fn, j})
				}
			}
			nCalls++
		}
	}
	sort.Slice(need, func(i, j int) bool { return need[i].fn.Pos() < need[j].fn.Pos() })
	nVariants := writeVariants(need, sites)
	for v, ids := range argUses {
		if !value[v] {
			continue
		}
		for _, id := range ids {
			id := id
			rw.Rewrite[id] = func() string { return "&" + id.Name }
		}
	}
	var ks []string
	left := 0
	for k, c := range why {
		ks = append(ks, fmt.Sprintf("%5d %s", c, k))
		left += c
	}
	sort.Sort(sort.Reverse(sort.StringSlice(ks)))
	fmt.Fprintf(os.Stderr, "classes converted: %d (%d slots, %d of them local builders; %d new builders, %d reads and runtime/helper calls); helper variants: %d; slots left: %d\n", nClasses, nSlots, nValue, nNew, nCalls, nVariants, left)
	for _, k := range ks {
		fmt.Fprintln(os.Stderr, "  "+k)
	}
	if !report {
		fmt.Fprintf(os.Stderr, "files changed: %d\n", rw.SaveChecked())
	}
}

// gNode is a node at v's position (for its file).
func gNode(v *types.Var) ast.Node { return &ast.Ident{NamePos: v.Pos()} }

// rewriteValueLocal makes local v the builder itself.
func rewriteValueLocal(v *types.Var) {
	st, init := localDecl[v], localInit[v]
	name := v.Name()
	_, c := rw.CallName(init)
	ctor, _ := rw.CallName(init)
	switch ctor {
	case "new_StringBuilder_int":
		arg := c.Args[0]
		rw.Rewrite[st] = func() string { return "var " + name + " strBuilder_\n" + name + ".grow(" + rw.Render(arg) + ")" }
	case "new_StringBuilder_String":
		arg := c.Args[0]
		rw.Rewrite[st] = func() string { return "var " + name + " strBuilder_\n" + name + ".append(" + rw.Render(arg) + ")" }
	default:
		rw.Rewrite[st] = func() string { return "var " + name + " strBuilder_" }
	}
	for _, u := range rw.UsesOf[v] {
		if as, ok := rw.Parent[u].(*ast.AssignStmt); ok && len(as.Lhs) == 1 && as.Lhs[0] == u {
			rhs := as.Rhs[0]
			rw.Rewrite[rhs] = func() string { return "strBuilder_{}" }
		}
	}
}
// reflip19: jtostr_ only (this version has no Appendable arguments)
