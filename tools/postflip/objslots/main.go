// Command objslots is a post-flip step: a field, parameter, result or variable that Java
// declares as Object (or as a type variable, which j2go erases to `any`) and that only ever holds
// values of one Go type gets that type.
//
// j2go erases what it cannot type to `any`: a value is boxed when stored (strobj_, obj_, self(),
// box_(), any()) and unboxed when read (str_, cast_[*X], to_X, Node_of_, icast_X). When every
// value that can reach a slot is the box of a value of one type T, or nil, the slot can hold the
// T itself. The slots that exchange values (an assignment, an argument passed to a parameter,
// a returned value, a := of one into another) form a class, found with union-find over the whole
// package, and a class changes type as a whole or not at all. Its values then go:
//
//   - stored: `slot = obj_(x)` is `slot = x`; nil is T's null (nil, nullstr_ or the zero handle);
//   - read and unboxed to T: `str_(slot)`, `to_X(slot)`, `cast_[*X](slot)` is `slot`;
//   - read and compared with nil: `slot == nil` (for a string, `slot == nullstr_`);
//   - read into anything else (an Object argument, a comparison, a cast to another class): it is
//     boxed the way every source boxed it, so the value seen there is the same as before.
//
// Boxing and the matching unboxing are inverse (null boxes to nil and nil unboxes to null), so a
// value read back is the one stored; the box built at the remaining reads is the box the source
// built. The step keeps Java's behaviour: only where the program stored and read one type does
// anything change, and there the same value is seen.
//
// A slot stays `any` (and with it its class) when:
//   - a value of another kind can reach it: a value that is not a box (a map or list element, an
//     unboxed generic result, a value of an interface type), boxes of two types, a tuple;
//   - its function is used other than by calling it (a method value, a func value), is called
//     from the handwritten runtime, is generic or variadic there, or is a method whose name some
//     interface lists (overriding, dispatch: signatures must stay identical);
//   - it is a parameter of a func literal (its type is the functional interface's), a range
//     variable, or its address is taken;
//   - it is a string field of a class without a zero_ method (a String field starts as null,
//     which the zero value of a Go string is not).
//
// Rerunning the step on its output changes nothing (no slot of the remaining classes can change).
//
// usage: objslots [-n] <dir of package closure>
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

type slotKind int

const (
	kField slotKind = iota
	kParam
	kResult
	kLocal
)

var kindName = []string{"field", "param", "result", "variable"}

type slot struct {
	v     *types.Var
	kind  slotKind
	fn    *types.Func // param/result: the function
	bad   string      // why it must stay any ("" if fine)
	types []types.Type // the types of the boxes reaching it, directly or through other slots
	from  []*slot      // the slots whose values it receives
	nbox  int          // boxes stored into it
	reads []ast.Expr   // the expressions reading it
	// declaration
	typeExpr ast.Expr
	field    *ast.Field // the field list entry (param/result/struct field) holding it
	spec     *ast.ValueSpec
	zeroOf   string // struct field: the struct type name
}

var (
	slots = map[*types.Var]*slot{}
	resOf = map[*types.Func]*slot{}
	stats = map[string]int{}
)

func markBad(s *slot, why string) {
	if s.bad == "" {
		s.bad = why
	}
}

// addType adds t to the types reaching s and reports whether it was new.
func addType(s *slot, t types.Type) bool {
	for _, u := range s.types {
		if types.Identical(u, t) {
			return false
		}
	}
	s.types = append(s.types, t)
	return true
}

func generated(n ast.Node) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.File(n.Pos()).Name()), "g_")
}

func isAny(t types.Type) bool {
	i, ok := types.Unalias(t).(*types.Interface)
	return ok && i.Empty() && !i.IsComparable()
}

func newSlot(v *types.Var, k slotKind) *slot {
	s := &slot{v: v, kind: k}
	slots[v] = s
	return s
}

// boxOf returns the boxed value x and its type when e is the canonical box of x for its type
// (strobj_(x) for a string, obj_(x) or x.self() for a class pointer, x.box_() for a handle,
// any(x) for an interface), as rw.ConvFor builds it.
func boxOf(e ast.Expr) (ast.Expr, types.Type) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil, nil
	}
	var inner ast.Expr
	var form string
	if id, ok := c.Fun.(*ast.Ident); ok && len(c.Args) == 1 && (id.Name == "obj_" || id.Name == "strobj_" || id.Name == "any") {
		inner, form = c.Args[0], id.Name+"(X)"
	} else if s, ok := c.Fun.(*ast.SelectorExpr); ok && len(c.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
		inner, form = s.X, "X."+s.Sel.Name+"()"
	}
	if inner == nil {
		return nil, nil
	}
	tv, ok := rw.Info.Types[inner]
	if !ok || tv.Type == nil || tv.IsNil() || isAny(tv.Type) {
		return nil, nil
	}
	cv := rw.ConvFor(tv.Type)
	if !cv.OK || cv.Box("X") != form {
		return nil, nil
	}
	return inner, tv.Type
}

// objOfPtr returns x and its type for obj_(x) or x.self() of a class pointer x: unboxed to the
// pointer's own type, both give x back (self() is the most derived object, whose as_X is x).
func objOfPtr(e ast.Expr) (ast.Expr, types.Type) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil, nil
	}
	var inner ast.Expr
	if id, ok := c.Fun.(*ast.Ident); ok && len(c.Args) == 1 && id.Name == "obj_" {
		inner = c.Args[0]
	} else if s, ok := c.Fun.(*ast.SelectorExpr); ok && len(c.Args) == 0 && s.Sel.Name == "self" {
		inner = s.X
	}
	if inner == nil {
		return nil, nil
	}
	tv, ok := rw.Info.Types[inner]
	if !ok || tv.Type == nil {
		return nil, nil
	}
	if p, ok := tv.Type.(*types.Pointer); ok {
		if _, ok := p.Elem().(*types.Named); ok {
			return inner, tv.Type
		}
	}
	return nil, nil
}

func isNil(e ast.Expr) bool {
	tv, ok := rw.Info.Types[rw.Unparen(e)]
	return ok && tv.IsNil()
}

// slotRead returns the slot that expression e reads directly: a variable or field, or the
// result of a call of a function with a result slot.
func slotRead(e ast.Expr) *slot {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		if v, ok := rw.Info.Uses[x].(*types.Var); ok {
			return slots[v]
		}
	case *ast.SelectorExpr:
		if v, ok := rw.Info.Uses[x.Sel].(*types.Var); ok {
			if sel := rw.Info.Selections[x]; sel == nil || sel.Kind() == types.FieldVal {
				return slots[v]
			}
		}
	case *ast.CallExpr:
		if f := calledFunc(x); f != nil {
			return resOf[f]
		}
	}
	return nil
}

func calledFunc(c *ast.CallExpr) *types.Func {
	switch f := rw.Unparen(c.Fun).(type) {
	case *ast.Ident:
		o, _ := rw.Info.Uses[f].(*types.Func)
		return o
	case *ast.SelectorExpr:
		if sel := rw.Info.Selections[f]; sel != nil {
			if sel.Kind() != types.MethodVal {
				return nil
			}
		}
		o, _ := rw.Info.Uses[f.Sel].(*types.Func)
		return o
	}
	return nil
}

// sources records the value e stored into slot s.
var sourceOf = map[ast.Expr]*slot{}

func source(s *slot, e ast.Expr) {
	e = rw.Unparen(e)
	sourceOf[e] = s
	if isNil(e) {
		return
	}
	if r := slotRead(e); r != nil {
		s.from = append(s.from, r)
		return
	}
	if _, t := boxOf(e); t != nil {
		addType(s, t)
		s.nbox++
		return
	}
	if os.Getenv("OBJSLOTS_DEBUG") != "" && s.bad == "" {
		tv := rw.Info.Types[e]
		fmt.Fprintf(os.Stderr, "notbox %s %s %s: %s [%v]\n", rw.Fset.Position(e.Pos()), kindName[s.kind], s.v.Name(), rw.Text(e), tv.Type)
	}
	markBad(s, "a value that is not a box")
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])

	// Method names that some interface lists: their signatures are tied to it.
	ifaceMethods := map[string]bool{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if it, ok := n.(*ast.InterfaceType); ok {
				if t, ok := rw.Info.Types[it].Type.(*types.Interface); ok {
					for i := 0; i < t.NumMethods(); i++ {
						ifaceMethods[t.Method(i).Name()] = true
					}
				}
			}
			return true
		})
	}

	// ---- slots
	var funcs []*ast.FuncDecl
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.GenDecl:
				for _, sp := range x.Specs {
					ts, ok := sp.(*ast.TypeSpec)
					if !ok {
						continue
					}
					st, ok := ts.Type.(*ast.StructType)
					if !ok || ts.TypeParams != nil {
						continue
					}
					for _, fl := range st.Fields.List {
						for _, nm := range fl.Names {
							if v, ok := rw.Info.Defs[nm].(*types.Var); ok && isAny(v.Type()) {
								s := newSlot(v, kField)
								s.typeExpr, s.field, s.zeroOf = fl.Type, fl, ts.Name.Name
							}
						}
					}
				}
			case *ast.FuncDecl:
				funcs = append(funcs, x)
			}
		}
	}
	for _, fd := range funcs {
		fo, ok := rw.Info.Defs[fd.Name].(*types.Func)
		if !ok {
			continue
		}
		sig := fo.Type().(*types.Signature)
		why := ""
		switch {
		case fd.Type.TypeParams != nil || (sig.Recv() != nil && sig.RecvTypeParams().Len() > 0):
			why = "generic function"
		case sig.Recv() != nil && ifaceMethods[fo.Name()]:
			why = "method listed by an interface"
		case fd.Name.Name == "init" || fd.Name.Name == "main":
			why = "init"
		case sig.Recv() == nil && (strings.HasPrefix(fo.Name(), "to_") || strings.HasPrefix(fo.Name(), "is_") || strings.HasPrefix(fo.Name(), "icast_")):
			why = "a cast helper"
		}
		if why == "" {
			for _, u := range rw.UsesOf[fo] {
				if !generated(u) {
					why = "used by the runtime"
					break
				}
				var fun ast.Node = u
				if s, ok := rw.Parent[u].(*ast.SelectorExpr); ok && s.Sel == u {
					fun = s
					if sel := rw.Info.Selections[s]; sel != nil && sel.Kind() != types.MethodVal {
						why = "method expression"
						break
					}
				}
				if c, ok := rw.Up(fun).(*ast.CallExpr); !ok || rw.Unparen(c.Fun) != fun {
					why = "used as a value"
					break
				}
			}
		}
		n := 0
		for _, fl := range fd.Type.Params.List {
			for _, nm := range fl.Names {
				v := sig.Params().At(n)
				n++
				if !isAny(v.Type()) {
					continue
				}
				s := newSlot(v, kParam)
				s.fn, s.typeExpr, s.field = fo, fl.Type, fl
				if sig.Variadic() && n == sig.Params().Len() {
					s.bad = "variadic"
				} else if why != "" {
					s.bad = why
				}
				_ = nm
			}
			if len(fl.Names) == 0 {
				n++
			}
		}
		if sig.Results().Len() == 1 && isAny(sig.Results().At(0).Type()) {
			s := newSlot(sig.Results().At(0), kResult)
			s.fn, s.typeExpr, s.field = fo, fd.Type.Results.List[0].Type, fd.Type.Results.List[0]
			resOf[fo] = s
			if len(fd.Type.Results.List[0].Names) > 0 {
				s.bad = "named result"
			} else if why != "" {
				s.bad = why
			}
		}
	}
	// Variables: from their definitions.
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.ValueSpec:
				for i, nm := range x.Names {
					v, ok := rw.Info.Defs[nm].(*types.Var)
					if !ok || !isAny(v.Type()) || slots[v] != nil {
						continue
					}
					s := newSlot(v, kLocal)
					s.spec = x
					if len(x.Names) > 1 {
						s.bad = "several names in one declaration"
					}
					if len(x.Values) > 0 && len(x.Values) != len(x.Names) {
						s.bad = "a tuple"
					}
					_ = i
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
					if !ok || !isAny(v.Type()) {
						continue
					}
					s := newSlot(v, kLocal)
					if len(x.Lhs) != len(x.Rhs) {
						s.bad = "a tuple"
					} else {
						_ = i
					}
				}
			case *ast.RangeStmt:
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if id, ok := e.(*ast.Ident); ok {
						if v, ok := rw.Info.Defs[id].(*types.Var); ok && isAny(v.Type()) {
							newSlot(v, kLocal).bad = "range variable"
						}
					}
				}
			case *ast.FuncLit:
				for _, fl := range append(append([]*ast.Field{}, x.Type.Params.List...), fieldsOf(x.Type.Results)...) {
					for _, nm := range fl.Names {
						if v, ok := rw.Info.Defs[nm].(*types.Var); ok && isAny(v.Type()) {
							newSlot(v, kLocal).bad = "func literal parameter"
						}
					}
				}
			}
			return true
		})
	}
	for v, s := range slots {
		for _, u := range rw.UsesOf[v] {
			if !generated(u) {
				s.bad = "used by the runtime"
			}
		}
	}

	// ---- sources
	var retFunc func(n ast.Node) *types.Func
	retFunc = func(n ast.Node) *types.Func {
		for q := rw.Parent[n]; q != nil; q = rw.Parent[q] {
			switch fn := q.(type) {
			case *ast.FuncLit:
				return nil
			case *ast.FuncDecl:
				o, _ := rw.Info.Defs[fn.Name].(*types.Func)
				return o
			}
		}
		return nil
	}
	lhsSlot := func(l ast.Expr) *slot {
		switch x := rw.Unparen(l).(type) {
		case *ast.Ident:
			if v, ok := rw.Info.Uses[x].(*types.Var); ok {
				return slots[v]
			}
			if v, ok := rw.Info.Defs[x].(*types.Var); ok {
				return slots[v]
			}
		case *ast.SelectorExpr:
			if v, ok := rw.Info.Uses[x.Sel].(*types.Var); ok {
				return slots[v]
			}
		}
		return nil
	}
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.ValueSpec:
				if len(x.Values) == len(x.Names) {
					for i, nm := range x.Names {
						if v, ok := rw.Info.Defs[nm].(*types.Var); ok && slots[v] != nil {
							source(slots[v], x.Values[i])
						}
					}
				}
			case *ast.RangeStmt:
				for _, e := range []ast.Expr{x.Key, x.Value} {
					if e != nil {
						if s := lhsSlot(e); s != nil {
							markBad(s, "range variable")
						}
					}
				}
			case *ast.AssignStmt:
				if x.Tok != token.ASSIGN && x.Tok != token.DEFINE {
					return true
				}
				for i, l := range x.Lhs {
					s := lhsSlot(l)
					if s == nil {
						continue
					}
					if len(x.Lhs) != len(x.Rhs) {
						markBad(s, "a tuple")
					} else {
						source(s, x.Rhs[i])
					}
				}
			case *ast.ReturnStmt:
				fo := retFunc(x)
				if fo == nil || resOf[fo] == nil {
					return true
				}
				if len(x.Results) == 1 {
					source(resOf[fo], x.Results[0])
				} else {
					markBad(resOf[fo], "a tuple")
				}
			case *ast.CallExpr:
				fo := calledFunc(x)
				if fo == nil {
					return true
				}
				sig, ok := fo.Type().(*types.Signature)
				if !ok {
					return true
				}
				for i, a := range x.Args {
					if i >= sig.Params().Len() {
						break
					}
					if s := slots[sig.Params().At(i)]; s != nil && s.kind == kParam && s.fn == fo {
						if x.Ellipsis.IsValid() {
							markBad(s, "variadic")
						}
						source(s, a)
					}
				}
			case *ast.CompositeLit:
				for _, el := range x.Elts {
					kv, ok := el.(*ast.KeyValueExpr)
					if !ok {
						if st, ok := rw.Info.Types[x].Type.Underlying().(*types.Struct); ok {
							for i := 0; i < st.NumFields(); i++ {
								if s := slots[st.Field(i)]; s != nil {
									markBad(s, "positional composite literal")
								}
							}
						}
						break
					}
					if id, ok := kv.Key.(*ast.Ident); ok {
						if v, ok := rw.Info.Uses[id].(*types.Var); ok && slots[v] != nil {
							source(slots[v], kv.Value)
						}
					}
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					if s := lhsSlot(x.X); s != nil {
						markBad(s, "address taken")
					}
				}
			case *ast.IncDecStmt:
				if s := lhsSlot(x.X); s != nil {
					markBad(s, "a tuple")
				}
			}
			return true
		})
	}
	// Strings start as null: a string field needs its class's zero_ method.
	zeroFns := map[string]*ast.FuncDecl{}
	for _, fd := range funcs {
		if fd.Recv != nil && strings.HasPrefix(fd.Name.Name, "zero_") {
			zeroFns[fd.Name.Name] = fd
		}
	}

	for v, s := range slots {
		if s.kind == kResult {
			for _, u := range rw.UsesOf[s.fn] {
				var fun ast.Expr = u
				if sx, ok := rw.Parent[u].(*ast.SelectorExpr); ok && sx.Sel == u {
					fun = sx
				}
				if c, ok := rw.Up(fun).(*ast.CallExpr); ok {
					s.reads = append(s.reads, c)
				}
			}
			continue
		}
		for _, u := range rw.UsesOf[v] {
			if e := rw.VarExpr(u); !isWrite(e) {
				s.reads = append(s.reads, e)
			}
		}
	}

	// ---- decide
	// The types reaching each slot, through the slots it receives from (a fixpoint; two types
	// are as many as more).
	for changed := true; changed; {
		changed = false
		for _, s := range slots {
			for _, r := range s.from {
				for _, t := range r.types {
					if len(s.types) < 2 && addType(s, t) {
						changed = true
					}
				}
			}
		}
	}
	for _, s := range slots {
		switch {
		case s.bad != "":
		case len(s.types) > 1:
			s.bad = "boxes of two types"
		case len(s.types) == 0:
			s.bad = "no box stored"
		case s.kind == kField && isString(s.types[0]) && zeroFns["zero_"+s.zeroOf] == nil:
			s.bad = "string field of a class without zero_"
		}
	}
	// A slot receiving from a slot that stays any stays any too.
	for changed := true; changed; {
		changed = false
		for _, s := range slots {
			if s.bad != "" {
				continue
			}
			for _, r := range s.from {
				if r.bad != "" {
					s.bad = "receives from a slot that stays any (" + r.bad + ")"
					changed = true
					break
				}
			}
		}
	}

	conv := func(s *slot) (types.Type, bool) {
		if s.bad != "" {
			return nil, false
		}
		return s.types[0], true
	}

	nullOf := func(t types.Type) string {
		if isString(t) {
			return "nullstr_"
		}
		if _, ok := t.Underlying().(*types.Basic); ok {
			return "0"
		}
		return "nil"
	}
	readOf := func(r ast.Expr, s *slot, dry bool) int {
		t := s.types[0]
		if src := sourceOf[rw.Unparen(r)]; src != nil {
			if _, ok := conv(src); ok {
				return 0
			}
		}
		cv := rw.ConvFor(t)
		p := rw.Up(r)
		if c, ok := p.(*ast.CallExpr); ok && len(c.Args) == 1 && rw.Unparen(c.Args[0]) == r {
			if name, _ := rw.CallName(c); name != "" && cv.IsUnbox(name) {
				if !dry {
					rw.Rewrite[c] = func() string { return rw.RenderChildren(r) }
					stats["unboxing dropped"]++
				}
				return 1
			}
			if id, ok := c.Fun.(*ast.Ident); ok && (id.Name == "obj_" || id.Name == "any") {
				if !dry {
					rw.Rewrite[c] = func() string { return cv.Box(rw.RenderChildren(r)) }
					stats["boxed again"]++
				}
				return 0
			}
		}
		if b, ok := p.(*ast.BinaryExpr); ok && (b.Op == token.EQL || b.Op == token.NEQ) {
			other := b.Y
			if rw.Unparen(b.Y) == r {
				other = b.X
			}
			if isNil(other) {
				if n := nullOf(t); n != "nil" && !dry {
					o := rw.Unparen(other)
					rw.Rewrite[o] = func() string { return n }
				}
				if !dry {
					stats["null test"]++
				}
				return 0
			}
		}
		if !dry {
			rw.Rewrite[r] = func() string { return cv.Box(rw.RenderChildren(r)) }
			stats["boxed again"]++
		}
		return -1
	}
	// A slot changes type only where that removes more conversions than it adds: each box
	// stored and each unboxing read count one, each read that must box again minus one.
	for changed := true; changed; {
		changed = false
		var drop []*slot
		for _, s := range slots {
			if s.bad != "" {
				continue
			}
			g := s.nbox
			for _, r := range s.reads {
				g += readOf(r, s, true)
			}
			if g <= 0 {
				drop = append(drop, s)
			}
		}
		for _, s := range drop {
			s.bad = "no gain"
			changed = true
		}
		for again := true; again; {
			again = false
			for _, s := range slots {
				if s.bad != "" {
					continue
				}
				for _, r := range s.from {
					if r.bad != "" {
						s.bad = "receives from a slot that stays any (" + r.bad + ")"
						again = true
						break
					}
				}
			}
		}
	}

	// ---- rewrite
	tstr := func(t types.Type) string { return types.TypeString(t, func(*types.Package) string { return "" }) }
	changedDecl := map[*ast.Field]bool{}
	nullFields := map[string][]*types.Var{}
	for _, s := range slots {
		t, ok := conv(s)
		if !ok {
			continue
		}
		cls := kindName[s.kind]
		stats[cls]++
		if s.field != nil && !changedDecl[s.field] {
			changedDecl[s.field] = true
			fl := s.field
			if len(fl.Names) <= 1 {
				rw.Rewrite[s.typeExpr] = func() string { return tstr(t) }
			} else {
				// split the entry: a, b any -> a T, b any
				rw.Rewrite[fl] = func() string {
					var parts []string
					for _, nm := range fl.Names {
						tt := "any"
						if o, ok := rw.Info.Defs[nm].(*types.Var); ok && slots[o] != nil {
							if t2, ok := conv(slots[o]); ok {
								tt = tstr(t2)
							}
						}
						parts = append(parts, nm.Name+" "+tt)
					}
					return strings.Join(parts, ", ")
				}
			}
		}
		if s.spec != nil && s.spec.Type != nil {
			sp := s.spec
			rw.Rewrite[sp.Type] = func() string { return tstr(t) }
			if len(sp.Values) == 0 && isString(t) {
				rw.Rewrite[sp] = func() string { return sp.Names[0].Name + " " + tstr(t) + " = nullstr_" }
			}
		}
		if s.kind == kField && isString(t) {
			nullFields[s.zeroOf] = append(nullFields[s.zeroOf], s.v)
		}
	}
	// a String field starts as null: its class's zero_ method sets it
	for cls, vs := range nullFields {
		sort.Slice(vs, func(i, j int) bool { return vs[i].Pos() < vs[j].Pos() })
		zf := zeroFns["zero_"+cls]
		body := zf.Body
		recv := zf.Recv.List[0].Names[0].Name
		rw.Rewrite[body] = func() string {
			txt := rw.RenderChildren(body)
			i := strings.LastIndex(txt, "}")
			for _, v := range vs {
				txt = txt[:i] + "\t" + recv + "." + v.Name() + " = nullstr_\n" + txt[i:]
				i = strings.LastIndex(txt, "}")
			}
			return txt
		}
	}
	// sources
	for e, s := range sourceOf {
		t, ok := conv(s)
		if !ok {
			continue
		}
		e := e
		if isNil(e) {
			rw.Rewrite[e] = func() string { return nullOf(t) }
			stats["nil stored"]++
			continue
		}
		if inner, _ := boxOf(e); inner != nil {
			rw.Rewrite[e] = func() string { return rw.Render(inner) }
			stats["box dropped"]++
		}
	}
	// reads
	for _, s := range slots {
		if _, ok := conv(s); ok {
			for _, r := range s.reads {
				readOf(r, s, false)
			}
		}
	}

	// Casts that unbox a box of the type they cast to: to_X(obj_(x)), cast_[*X](x.self()),
	// str_(strobj_(s)), Node_of_(n.box_()), icast_I(any(i)) is the value itself.
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok || len(c.Args) != 1 || rw.Rewrite[c] != nil {
				return true
			}
			name, _ := rw.CallName(c)
			if name == "" {
				return true
			}
			inner, t := boxOf(c.Args[0])
			if inner == nil {
				inner, t = objOfPtr(c.Args[0])
			}
			if inner == nil || !rw.ConvFor(t).IsUnbox(name) {
				return true
			}
			rw.Rewrite[c] = func() string { return rw.Render(inner) }
			stats["cast of its own box"]++
			return true
		})
	}

	// ---- report
	byReason := map[string]int{}
	byReasonKind := map[string]int{}
	for _, s := range slots {
		if s.bad != "" {
			why := s.bad
			if strings.HasPrefix(why, "receives from") {
				why = "receives from a slot that stays any"
			}
			byReason[why]++
			byReasonKind[kindName[s.kind]+": "+why]++
		}
	}
	keys := func(m map[string]int) []string {
		var l []string
		for k := range m {
			l = append(l, k)
		}
		sort.Strings(l)
		return l
	}
	total := 0
	for _, k := range []string{"field", "param", "result", "variable"} {
		total += stats[k]
	}
	fmt.Printf("objslots: %d of %d any slots get a type (fields %d, params %d, results %d, variables %d)\n", total, len(slots), stats["field"], stats["param"], stats["result"], stats["variable"])
	fmt.Printf("  boxes dropped %d, nil stored %d, unboxings dropped %d, null tests %d, boxed again at reads %d\n", stats["box dropped"], stats["nil stored"], stats["unboxing dropped"], stats["null test"], stats["boxed again"])
	fmt.Printf("  casts of a box of their own type dropped: %d\n", stats["cast of its own box"])
	fmt.Println("  left any:")
	for _, k := range keys(byReasonKind) {
		fmt.Printf("    %5d %s\n", byReasonKind[k], k)
	}
	if report {
		var l []string
		for _, s := range slots {
			if t, ok := conv(s); ok {
				p := rw.Fset.Position(s.v.Pos())
				l = append(l, fmt.Sprintf("%s:%d %s %s %s", filepath.Base(p.Filename), p.Line, kindName[s.kind], s.v.Name(), tstr(t)))
			}
		}
		sort.Strings(l)
		for _, x := range l {
			fmt.Println(x)
		}
		return
	}
	fmt.Printf("files changed: %d\n", rw.Save())
}

func fieldsOf(fl *ast.FieldList) []*ast.Field {
	if fl == nil {
		return nil
	}
	return fl.List
}

func isString(t types.Type) bool {
	b, ok := types.Unalias(t).(*types.Basic)
	return ok && b.Kind() == types.String
}

// isWrite reports whether e (a variable or field) is assigned rather than read.
func isWrite(e ast.Expr) bool {
	switch p := rw.Up(e).(type) {
	case *ast.AssignStmt:
		for _, l := range p.Lhs {
			if rw.Unparen(l) == e {
				return true
			}
		}
	case *ast.KeyValueExpr:
		return p.Key == e
	}
	return false
}
