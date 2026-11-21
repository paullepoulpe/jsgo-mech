// Package anyflow follows the values that the program holds in interface types (Java's Object and
// the Java interfaces): which classes are converted to an interface, and where such a value can
// go from there. The post-flip steps that need whole-program flow (deadeq: can anything call
// this equals; objflow: what does this Object slot hold) share it.
//
// The model is flow-insensitive and field-based, over the whole package, the handwritten runtime
// included:
//
//   - a slot is a variable, field, parameter or result whose type is an interface or a slice,
//     array or map of interfaces (a container is one slot with its elements: what is stored in
//     an element is in the slot, what is read from an element comes from it);
//   - a class is a named struct type; a type parameter stands for the classes its
//     instantiations bind to it;
//   - an edge is a store: an assignment, an argument given to a parameter (for a call through an
//     interface: to the parameter of every method of that name whose receiver implements the
//     interface), a returned value, an element of a composite literal, append and copy, a range
//     variable, a type switch's variable. A conversion of a typed value to an interface starts a
//     path at the destination slot; a type assertion back to a class ends nothing (the typed
//     value's own conversions are followed where they happen);
//   - instantiation binds: the classes in a type argument of a generic function are bound to
//     its type parameter; the type arguments of a generic type are bound to the receiver type
//     parameters of a method where the method is named on a value of the type, and to those of
//     all its methods where a value of the type is converted to an interface (any of them may
//     then be called). An interface value stored into a generic object can be read by all its
//     methods;
//   - what the model does not follow is a sink: a store through a pointer or into an element of
//     something that is no slot, an argument or result of a function value, a function literal's
//     result (unless the literal is called on the spot), a channel, a panic, an argument of
//     another package's function (but for the listed ones that only read or reorder it), an
//     address taken (but to read the interface words through unsafe.Pointer). A read from what
//     the model does not follow yields nothing, which is sound because every store there was a
//     sink.
//
// Small functions that return their own parameter (obj_, checkNotNull: found by their bodies)
// are followed per call, so that their callers are not merged. Four runtime primitives are
// modelled by hand, each checked against its source: boxT_ and boxValue_ (the most derived
// object of the argument), a tconv_'s box (the value boxed) and unbox (a cast), Object_clone (an
// object of the argument's class, made by reflection).
package anyflow

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strings"

	"postflip/rw"
)

// A Src is where a value comes from: classes and type parameters (a typed value; Direct when
// the value is one object of the class, not a collection or function over it) and slots.
type Src struct {
	Cs     []*types.TypeName
	Direct bool
	Slots  []types.Object
	// Other concrete types (a string, a number, an enum, a handle, a typed slice...), for the
	// steps that ask what a slot holds.
	Ts []types.Type
}

func (s *Src) Empty() bool { return len(s.Cs) == 0 && len(s.Slots) == 0 && len(s.Ts) == 0 }

var (
	// Succ: values in a slot (or bound to a type parameter) also reach these.
	Succ = map[types.Object][]types.Object{}
	// SinkWhy: the slot's values reach something the model does not follow, or an
	// equals/hashCode call on an interface value.
	SinkWhy = map[types.Object]string{}
	// Into: the slots and type parameters a class's values are converted or bound into.
	Into = map[*types.TypeName]map[types.Object]string{}
	// Sunk: the sinks a class's values meet directly, with the first place.
	Sunk = map[*types.TypeName]map[string]string{}
	// TypesInto: other concrete types stored into a slot.
	TypesInto = map[types.Object][]types.Type{}
	// ClassInto: the classes stored into a slot directly (the reverse of Into for slots).
	ClassInto = map[types.Object][]*types.TypeName{}
	// Subs: the struct types that embed a struct type.
	Subs = map[*types.TypeName][]*types.TypeName{}
	// DeclOf: the declaration of each method and function of the package.
	DeclOf = map[*types.Func]*ast.FuncDecl{}
	ByName = map[string][]*types.Func{}
	IsG    = map[*token.File]bool{}
	// OnConvert is called for every conversion of a typed value (of the classes cs) to the
	// interface type it; a step that needs the conversions sets it before Build.
	OnConvert func(cs []*types.TypeName, direct bool, it types.Type, at ast.Node)

	declTP = map[*types.TypeName]*types.TypeName{}
	// tpArgs: the type arguments bound to a type parameter; tpConv: a value of the type
	// parameter is converted to an interface somewhere (so the methods of a generic type bound
	// to it can be called on that value).
	tpArgs    = map[*types.TypeName][]types.Type{}
	tpConv    = map[*types.TypeName]bool{}
	passParam = map[*types.Func][]int{}
	litVars   = map[*ast.FuncLit][]types.Object{}
	synth     = map[ast.Node]types.Object{}
	boundAll  = map[string]bool{}
)

// ---- function values ----
//
// A function slot is a variable, field, parameter or result of a function type. Before the
// values are followed, the functions are: which declared functions, method values, method
// expressions and function literals each function slot can hold (stores of function values,
// to a fixed point), which slots can also hold a function that is not known (read from an
// element, returned by a call through a function value, a parameter of a function that
// escapes), and which functions escape (stored where the model does not follow). A call
// through a function slot whose functions are all known is then a call of each of them.

// An FTarget is a known function value: a declared function or method (Recv: a method
// expression, whose first argument is the receiver) or a function literal.
type FTarget struct {
	Fn   *types.Func
	Lit  *ast.FuncLit
	Recv bool
}

var (
	phase    int
	fInto    = map[types.Object]map[FTarget]bool{}
	fSucc    = map[types.Object][]types.Object{}
	fUnknown = map[types.Object]bool{}
	fEscSlot = map[types.Object]bool{}
	// Escaped: the function can be called by code that is not followed.
	Escaped  = map[FTarget]bool{}
	seenFunc = map[ast.Node]bool{}
)

func isFuncType(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	_, ok := t.Underlying().(*types.Signature)
	return ok
}

func funcVar(o types.Object) bool {
	v, ok := o.(*types.Var)
	return ok && v.Pkg() == rw.Pkg && isFuncType(v.Type())
}

func slotty(o types.Object) bool { return o != nil && (Tracked(o) || funcVar(o)) }

// funcSrc: the functions the function-typed expression e can be.
func funcSrc(e ast.Expr) (ts []FTarget, slots []types.Object, unknown bool) {
	e = rw.Unparen(e)
	if tv, ok := rw.Info.Types[e]; ok && tv.IsNil() {
		return
	}
	switch x := e.(type) {
	case *ast.FuncLit:
		seenFunc[x] = true
		return []FTarget{{Lit: x}}, nil, false
	case *ast.Ident:
		switch o := rw.Info.Uses[x].(type) {
		case *types.Func:
			seenFunc[x] = true
			if o.Pkg() != rw.Pkg || DeclOf[o] == nil {
				return nil, nil, true
			}
			return []FTarget{{Fn: o}}, nil, false
		case *types.Var:
			if funcVar(o) {
				return nil, []types.Object{o}, false
			}
		}
		return nil, nil, true
	case *ast.SelectorExpr:
		switch o := rw.Info.Uses[x.Sel].(type) {
		case *types.Func:
			seenFunc[x.Sel] = true
			if o.Pkg() != rw.Pkg || DeclOf[o] == nil {
				return nil, nil, true // an interface's method, another package's function
			}
			if sn := rw.Info.Selections[x]; sn != nil && sn.Kind() == types.MethodExpr {
				return []FTarget{{Fn: o, Recv: true}}, nil, false
			}
			return []FTarget{{Fn: o}}, nil, false
		case *types.Var:
			if funcVar(o) {
				return nil, []types.Object{o}, false
			}
		}
		return nil, nil, true
	case *ast.IndexExpr:
		if tv, ok := rw.Info.Types[x.X]; ok && isFuncType(tv.Type) {
			return funcSrc(x.X) // an instantiated generic function
		}
	case *ast.IndexListExpr:
		return funcSrc(x.X)
	case *ast.CallExpr:
		if isConversion(x) {
			return funcSrc(x.Args[0])
		}
		if lit, ok := rw.Unparen(x.Fun).(*ast.FuncLit); ok {
			if sig, ok := rw.Info.Types[lit].Type.(*types.Signature); ok && sig.Results().Len() == 1 {
				return nil, []types.Object{sig.Results().At(0)}, false
			}
		}
		obj, _, _ := Callee(x)
		if f, ok := obj.(*types.Func); ok && f.Pkg() == rw.Pkg {
			fs := []*types.Func{f}
			if isInterfaceMethod(f) {
				fs, _ = Implementers(f)
				if len(fs) == 0 {
					return nil, nil, true
				}
			}
			for _, m := range fs {
				if rs := m.Type().(*types.Signature).Results(); rs.Len() == 1 && funcVar(rs.At(0)) {
					slots = append(slots, rs.At(0))
				} else {
					unknown = true
				}
			}
			return
		}
	}
	return nil, nil, true
}

func funcStore(v ast.Expr, dsts []types.Object) {
	if phase != 1 {
		return
	}
	ts, slots, unk := funcSrc(v)
	var fd []types.Object
	for _, d := range dsts {
		if funcVar(d) {
			fd = append(fd, d)
		}
	}
	if len(fd) == 0 {
		for _, t := range ts {
			Escaped[t] = true
		}
		for _, o := range slots {
			fEscSlot[o] = true
		}
		return
	}
	for _, d := range fd {
		for _, t := range ts {
			if fInto[d] == nil {
				fInto[d] = map[FTarget]bool{}
			}
			fInto[d][t] = true
		}
		for _, o := range slots {
			fSucc[o] = append(fSucc[o], d)
		}
		if unk {
			fUnknown[d] = true
		}
	}
}

// funcSolve takes the function stores to their fixed point.
func funcSolve() {
	// a function used where no store was seen escapes
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				if Skip[x] {
					return false
				}
			case *ast.FuncLit:
				if !seenFunc[x] {
					if c, ok := rw.Up(x).(*ast.CallExpr); !ok || rw.Unparen(c.Fun) != ast.Expr(x) {
						Escaped[FTarget{Lit: x}] = true
					}
				}
			case *ast.Ident:
				fo, ok := rw.Info.Uses[x].(*types.Func)
				if !ok || seenFunc[x] || DeclOf[fo] == nil {
					break
				}
				var e ast.Expr = x
				if s, ok := rw.Parent[x].(*ast.SelectorExpr); ok && s.Sel == x {
					e = s
				}
				p := rw.Up(e)
				if ix, ok := p.(*ast.IndexExpr); ok && ix.X == e {
					e, p = ix, rw.Up(ix)
				}
				if ix, ok := p.(*ast.IndexListExpr); ok && ix.X == e {
					e, p = ix, rw.Up(ix)
				}
				if c, ok := p.(*ast.CallExpr); ok && rw.Unparen(c.Fun) == e {
					break
				}
				Escaped[FTarget{Fn: fo}] = true
				Escaped[FTarget{Fn: fo, Recv: true}] = true
			}
			return true
		})
	}
	for changed := true; changed; {
		changed = false
		for o, ss := range fSucc {
			for _, d := range ss {
				for t := range fInto[o] {
					if !fInto[d][t] {
						if fInto[d] == nil {
							fInto[d] = map[FTarget]bool{}
						}
						fInto[d][t] = true
						changed = true
					}
				}
				if fUnknown[o] && !fUnknown[d] {
					fUnknown[d] = true
					changed = true
				}
				if fEscSlot[d] && !fEscSlot[o] {
					fEscSlot[o] = true
					changed = true
				}
			}
		}
		for o := range fEscSlot {
			for t := range fInto[o] {
				if !Escaped[t] {
					Escaped[t] = true
					changed = true
				}
			}
		}
		// the function parameters of an escaped function hold functions that are not known
		for t := range Escaped {
			var ps *types.Tuple
			if t.Fn != nil {
				ps = t.Fn.Type().(*types.Signature).Params()
			} else if sig, ok := rw.Info.Types[t.Lit].Type.(*types.Signature); ok {
				ps = sig.Params()
			}
			for i := 0; ps != nil && i < ps.Len(); i++ {
				if p := ps.At(i); funcVar(p) && !fUnknown[p] {
					fUnknown[p] = true
					changed = true
				}
			}
		}
	}
}

// Targets: the functions a call through the function-typed expression fun can run; ok is false
// when one of them is not known.
func Targets(fun ast.Expr) (ts []FTarget, ok bool) {
	t0, slots, unk := funcSrc(fun)
	if unk {
		return nil, false
	}
	set := map[FTarget]bool{}
	for _, t := range t0 {
		set[t] = true
	}
	for _, o := range slots {
		if fUnknown[o] {
			return nil, false
		}
		for t := range fInto[o] {
			set[t] = true
		}
	}
	for t := range set {
		ts = append(ts, t)
	}
	if len(ts) == 0 {
		return nil, false // no store was seen: not known
	}
	sort.Slice(ts, func(i, j int) bool { return tpos(ts[i]) < tpos(ts[j]) })
	return ts, true
}

func tpos(t FTarget) token.Pos {
	if t.Fn != nil {
		return t.Fn.Pos()
	}
	return t.Lit.Pos()
}

func tsig(t FTarget) *types.Signature {
	if t.Fn != nil {
		return t.Fn.Type().(*types.Signature)
	}
	sig, _ := rw.Info.Types[t.Lit].Type.(*types.Signature)
	return sig
}

// EqualsSinks makes a call x.equals(..) / x.hashCode() on a value of an interface type a sink of
// x (deadeq); without it only the model's blind spots are sinks.
var EqualsSinks bool

// Skip: function declarations whose bodies are not followed (a step that has shown them
// never to run).
var Skip = map[*ast.FuncDecl]bool{}

// Reset forgets the graph (not the loaded package), for a Build with another Skip.
func Reset() {
	Succ = map[types.Object][]types.Object{}
	SinkWhy = map[types.Object]string{}
	Into = map[*types.TypeName]map[types.Object]string{}
	Sunk = map[*types.TypeName]map[string]string{}
	TypesInto = map[types.Object][]types.Type{}
	ClassInto = map[types.Object][]*types.TypeName{}
	passParam = map[*types.Func][]int{}
	litVars = map[*ast.FuncLit][]types.Object{}
	boundAll = map[string]bool{}
	tpArgs = map[*types.TypeName][]types.Type{}
	tpConv = map[*types.TypeName]bool{}
	fInto = map[types.Object]map[FTarget]bool{}
	fSucc = map[types.Object][]types.Object{}
	fUnknown = map[types.Object]bool{}
	fEscSlot = map[types.Object]bool{}
	Escaped = map[FTarget]bool{}
	seenFunc = map[ast.Node]bool{}
}

func skipped(n ast.Node) bool {
	if len(Skip) == 0 {
		return false
	}
	fd := EnclosingDecl(n)
	return fd != nil && Skip[fd]
}

// TPOwner names the function or method a type parameter belongs to.
var TPOwner = map[*types.TypeName]string{}

func InG(pos token.Pos) bool { return IsG[rw.Fset.File(pos)] }

func Where(n ast.Node) string {
	p := rw.Fset.Position(n.Pos())
	return fmt.Sprintf("%s:%d", p.Filename[strings.LastIndex(p.Filename, "/")+1:], p.Line)
}

func Named(t types.Type) *types.TypeName {
	if t == nil {
		return nil
	}
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := types.Unalias(t).(*types.Named); ok {
		return n.Obj()
	}
	return nil
}

func IsIface(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	return types.IsInterface(t)
}

// IsSlotType: an interface, or a slice, array or map of interfaces.
func IsSlotType(t types.Type) bool {
	for d := 0; d < 6 && t != nil; d++ {
		if IsIface(t) {
			return true
		}
		if _, ok := types.Unalias(t).(*types.TypeParam); ok {
			return false
		}
		switch x := t.Underlying().(type) {
		case *types.Slice:
			t = x.Elem()
		case *types.Array:
			t = x.Elem()
		case *types.Map:
			if IsSlotType(x.Key()) {
				return true
			}
			t = x.Elem()
		default:
			return false
		}
	}
	return false
}

func IsTP(c *types.TypeName) bool {
	_, ok := c.Type().(*types.TypeParam)
	return ok
}

func isClass(tn *types.TypeName) bool {
	if tn.Pkg() != rw.Pkg {
		return false
	}
	_, ok := tn.Type().Underlying().(*types.Struct)
	return ok
}

// Mentions lists the classes and type parameters that t mentions; direct reports that t is a
// pointer to a class or a type parameter itself.
func Mentions(t types.Type) (cs []*types.TypeName, direct bool) {
	seen := map[*types.TypeName]bool{}
	var walk func(t types.Type, d int)
	walk = func(t types.Type, d int) {
		if d > 6 || t == nil {
			return
		}
		switch x := types.Unalias(t).(type) {
		case *types.TypeParam:
			if !seen[x.Obj()] {
				seen[x.Obj()] = true
				cs = append(cs, x.Obj())
			}
		case *types.Pointer:
			walk(x.Elem(), d+1)
		case *types.Slice:
			walk(x.Elem(), d+1)
		case *types.Array:
			walk(x.Elem(), d+1)
		case *types.Map:
			walk(x.Key(), d+1)
			walk(x.Elem(), d+1)
		case *types.Chan:
			walk(x.Elem(), d+1)
		case *types.Signature:
			for i := 0; i < x.Params().Len(); i++ {
				walk(x.Params().At(i).Type(), d+1)
			}
			for i := 0; i < x.Results().Len(); i++ {
				walk(x.Results().At(i).Type(), d+1)
			}
		case *types.Named:
			if isClass(x.Obj()) && !seen[x.Obj()] {
				seen[x.Obj()] = true
				cs = append(cs, x.Obj())
			}
			for i := 0; i < x.TypeArgs().Len(); i++ {
				walk(x.TypeArgs().At(i), d+1)
			}
		}
	}
	walk(t, 0)
	switch x := types.Unalias(t).(type) {
	case *types.Pointer:
		if n, ok := types.Unalias(x.Elem()).(*types.Named); ok && isClass(n.Obj()) {
			direct = true
		}
	case *types.TypeParam:
		direct = true
	}
	return
}

func tparams(t types.Type) []*types.TypeName {
	cs, _ := Mentions(t)
	out := cs[:0:0]
	for _, c := range cs {
		if IsTP(c) {
			out = append(out, c)
		}
	}
	return out
}

func Tracked(o types.Object) bool {
	v, ok := o.(*types.Var)
	return ok && v.Pkg() == rw.Pkg && IsSlotType(v.Type())
}

// WithSubs is tn and the struct types that embed it, transitively.
func WithSubs(tn *types.TypeName) []*types.TypeName {
	var out []*types.TypeName
	seen := map[*types.TypeName]bool{}
	var walk func(*types.TypeName)
	walk = func(t *types.TypeName) {
		if seen[t] {
			return
		}
		seen[t] = true
		out = append(out, t)
		for _, s := range Subs[t] {
			walk(s)
		}
	}
	walk(tn)
	return out
}

func Callee(c *ast.CallExpr) (obj types.Object, name string, recv ast.Expr) {
	fun := rw.Unparen(c.Fun)
	if ix, ok := fun.(*ast.IndexExpr); ok {
		if tv, ok := rw.Info.Types[ix.X]; ok && tv.Type != nil {
			if _, isSig := tv.Type.Underlying().(*types.Signature); isSig {
				fun = rw.Unparen(ix.X)
			}
		}
	}
	if ix, ok := fun.(*ast.IndexListExpr); ok {
		fun = rw.Unparen(ix.X)
	}
	switch f := fun.(type) {
	case *ast.Ident:
		return rw.Info.Uses[f], f.Name, nil
	case *ast.SelectorExpr:
		if _, isPkg := rw.Info.Uses[idOf(f.X)].(*types.PkgName); isPkg {
			return rw.Info.Uses[f.Sel], idOf(f.X).Name + "." + f.Sel.Name, nil
		}
		return rw.Info.Uses[f.Sel], f.Sel.Name, f.X
	}
	return nil, "", nil
}

func idOf(e ast.Expr) *ast.Ident {
	id, _ := rw.Unparen(e).(*ast.Ident)
	if id == nil {
		return &ast.Ident{}
	}
	return id
}

func isConversion(c *ast.CallExpr) bool {
	tv, ok := rw.Info.Types[c.Fun]
	return ok && tv.IsType()
}

// Implementers of the method an interface call names; generic reports that a method of a generic
// type may be the one called (its type parameters are not bound by such a call).
func Implementers(f *types.Func) (ms []*types.Func, generic bool) {
	sig := f.Type().(*types.Signature)
	if sig.Recv() == nil {
		return nil, true
	}
	it, ok := sig.Recv().Type().Underlying().(*types.Interface)
	if !ok {
		return nil, true
	}
	for _, m := range ByName[f.Name()] {
		r := m.Type().(*types.Signature).Recv().Type()
		if n := Named(r); n != nil {
			if nt, ok := n.Type().(*types.Named); ok && nt.TypeParams().Len() > 0 {
				// the uninstantiated type: compare the method's shape by name only
				ms = append(ms, m)
				generic = true
				continue
			}
		}
		if types.Implements(r, it) || types.Implements(types.NewPointer(r), it) {
			ms = append(ms, m)
		}
	}
	return
}

// extSafe: functions of other packages that read their arguments (format them, inspect their
// type, compare them with ==) or reorder a slice in place, and return none of them.
func extSafe(name string) bool {
	if i := strings.Index(name, "."); i > 0 {
		switch name[:i] {
		case "fmt", "strings", "strconv", "errors", "os", "io", "math", "bytes", "utf8", "utf16", "unicode", "sort", "time", "bufio", "filepath", "path", "atomic", "runtime", "debug", "binary", "bits", "hash", "maphash", "unsafe":
			return true
		}
		switch name {
		case "reflect.TypeOf", "reflect.ValueOf", "reflect.TypeFor", "reflect.DeepEqual",
			"slices.Sort", "slices.SortFunc", "slices.SortStableFunc", "slices.Contains", "slices.Index", "slices.IndexFunc",
			"slices.ContainsFunc", "slices.Equal", "slices.Reverse", "slices.BinarySearch", "slices.BinarySearchFunc", "slices.Max", "slices.Min",
			"maps.Keys", "maps.Values", "maps.Copy", "maps.Clone":
			return true
		}
	}
	return false
}

// extPass: functions of other packages whose result holds their first argument's elements.
var extPass = map[string]bool{
	"slices.Clone": true, "slices.Insert": true, "slices.Delete": true, "slices.Grow": true, "slices.Clip": true,
	"slices.Compact": true, "slices.CompactFunc": true, "slices.DeleteFunc": true, "slices.Concat": true, "slices.Collect": true,
	"maps.Clone": true,
}

func synthSlot(n ast.Node, t types.Type) types.Object {
	if o, ok := synth[n]; ok {
		return o
	}
	o := types.NewVar(n.Pos(), rw.Pkg, "lit_", t)
	synth[n] = o
	return o
}

func litResults(lit *ast.FuncLit) []types.Object {
	if r, ok := litVars[lit]; ok {
		return r
	}
	var out []types.Object
	if sig, ok := rw.Info.Types[lit].Type.(*types.Signature); ok {
		for i := 0; i < sig.Results().Len(); i++ {
			if v := sig.Results().At(i); IsSlotType(v.Type()) {
				out = append(out, v)
			}
		}
	}
	litVars[lit] = out
	return out
}

func resultSlots(f *types.Func) []types.Object {
	var out []types.Object
	rs := f.Type().(*types.Signature).Results()
	for i := 0; i < rs.Len(); i++ {
		if Tracked(rs.At(i)) {
			out = append(out, rs.At(i))
		}
	}
	return out
}

func union(a *Src, b Src) {
	a.Cs = append(a.Cs, b.Cs...)
	a.Slots = append(a.Slots, b.Slots...)
	a.Ts = append(a.Ts, b.Ts...)
	if len(b.Cs) > 0 && !b.Direct {
		a.Direct = false
	} else if len(a.Cs) == len(b.Cs) {
		a.Direct = b.Direct
	}
}

// Sources of the value of e.
func Sources(e ast.Expr) (s Src) {
	e = rw.Unparen(e)
	tv, ok := rw.Info.Types[e]
	if !ok || tv.Type == nil || tv.IsNil() {
		return
	}
	if !IsSlotType(tv.Type) {
		s.Cs, s.Direct = Mentions(tv.Type)
		if len(s.Cs) == 0 {
			s.Ts = []types.Type{tv.Type}
		}
		return
	}
	switch x := e.(type) {
	case *ast.Ident:
		if o := rw.Info.Uses[x]; o != nil && Tracked(o) {
			s.Slots = append(s.Slots, o)
		}
	case *ast.SelectorExpr:
		if o := rw.Info.Uses[x.Sel]; o != nil && Tracked(o) {
			s.Slots = append(s.Slots, o)
		}
	case *ast.IndexExpr:
		if xt := rw.Info.Types[x.X]; xt.Type != nil && IsSlotType(xt.Type) && !xt.IsType() {
			return Sources(x.X)
		}
	case *ast.SliceExpr:
		return Sources(x.X)
	case *ast.TypeAssertExpr:
		return Sources(x.X)
	case *ast.CompositeLit:
		s.Slots = append(s.Slots, synthSlot(x, tv.Type))
	case *ast.CallExpr:
		if isConversion(x) {
			return Sources(x.Args[0])
		}
		if lit, ok := rw.Unparen(x.Fun).(*ast.FuncLit); ok {
			s.Slots = append(s.Slots, litResults(lit)...)
			return
		}
		obj, name, recv := Callee(x)
		if _, isB := obj.(*types.Builtin); isB && name == "append" {
			s.Slots = append(s.Slots, synthSlot(x, tv.Type))
			return
		}
		if recv != nil && name == "box" && len(x.Args) == 1 {
			if tn := Named(rw.Info.Types[recv].Type); tn != nil && tn.Name() == "tconv_" {
				a := Sources(x.Args[0])
				if a.Direct {
					var cs []*types.TypeName
					for _, c := range a.Cs {
						if IsTP(c) {
							cs = append(cs, c)
						} else {
							cs = append(cs, WithSubs(c)...)
						}
					}
					a.Cs = cs
				}
				return a
			}
		}
		f, _ := obj.(*types.Func)
		if f == nil {
			// a call through a function value: the results of the functions it can be
			if ts, ok := Targets(x.Fun); ok {
				for _, t := range ts {
					if t.Fn != nil {
						s.Slots = append(s.Slots, resultSlots(t.Fn)...)
					} else {
						s.Slots = append(s.Slots, litResults(t.Lit)...)
					}
				}
			}
			return
		}
		if f.Pkg() != rw.Pkg {
			if extPass[name] && len(x.Args) > 0 {
				return Sources(x.Args[0])
			}
			return
		}
		if recv == nil && name == "Object_clone" && len(x.Args) == 1 {
			// a copy of the argument: an object of its class (made by reflection)
			s = Sources(x.Args[0])
			s.Slots = append(s.Slots, resultSlots(f)...)
			return
		}
		if recv == nil && (name == "boxT_" || name == "boxValue_") && len(x.Args) == 1 {
			a := Sources(x.Args[0])
			if a.Direct {
				var cs []*types.TypeName
				for _, c := range a.Cs {
					if IsTP(c) {
						cs = append(cs, c)
					} else {
						cs = append(cs, WithSubs(c)...)
					}
				}
				a.Cs = cs
			}
			return a
		}
		if recv != nil && name == "box" && len(x.Args) == 1 {
			if tn := Named(rw.Info.Types[recv].Type); tn != nil && tn.Name() == "tconv_" {
				a := Sources(x.Args[0])
				if a.Direct {
					var cs []*types.TypeName
					for _, c := range a.Cs {
						if IsTP(c) {
							cs = append(cs, c)
						} else {
							cs = append(cs, WithSubs(c)...)
						}
					}
					a.Cs = cs
				}
				return a
			}
		}
		sig := f.Type().(*types.Signature)
		if sig.Recv() != nil && types.IsInterface(sig.Recv().Type()) {
			ms, _ := Implementers(f)
			for _, m := range ms {
				s.Slots = append(s.Slots, resultSlots(m)...)
			}
			return
		}
		s.Slots = append(s.Slots, resultSlots(f)...)
		for _, i := range passParam[f] {
			if i < len(x.Args) {
				a := Sources(x.Args[i])
				union(&s, a)
			}
		}
	}
	return
}

func sinkClass(c *types.TypeName, reason string, at string) {
	if Sunk[c] == nil {
		Sunk[c] = map[string]string{}
	}
	if _, ok := Sunk[c][reason]; !ok {
		Sunk[c][reason] = at
	}
}

// Sink: the values of s reach something that is not followed.
func Sink(s Src, reason string, at ast.Node) {
	if phase == 1 {
		return
	}
	w := Where(at)
	for _, c := range s.Cs {
		if IsTP(c) {
			if _, ok := SinkWhy[c]; !ok {
				SinkWhy[c] = reason + " (" + w + ")"
			}
			continue
		}
		sinkClass(c, reason, w)
	}
	for _, o := range s.Slots {
		if _, ok := SinkWhy[o]; !ok {
			SinkWhy[o] = reason + " (" + w + ")"
		}
	}
}

func flowAt(s Src, to types.Object, at ast.Node) {
	if phase == 1 {
		return
	}
	for _, c := range s.Cs {
		if IsTP(c) {
			Succ[c] = append(Succ[c], to)
			continue
		}
		if Into[c] == nil {
			Into[c] = map[types.Object]string{}
		}
		if _, ok := Into[c][to]; !ok {
			Into[c][to] = Where(at)
			ClassInto[to] = append(ClassInto[to], c)
		}
	}
	for _, o := range s.Slots {
		if o != to {
			Succ[o] = append(Succ[o], to)
		}
	}
	if len(s.Ts) > 0 {
		TypesInto[to] = append(TypesInto[to], s.Ts...)
	}
}

// bindAll binds the type arguments of the generic types reachable in t's method set to the
// receiver type parameters of every method: a value of t is held as an interface, so any of
// them may be called.
func bindAll(t types.Type, at ast.Node) {
	key := types.TypeString(t, nil)
	if boundAll[key] {
		return
	}
	boundAll[key] = true
	ms := types.NewMethodSet(t)
	for i := 0; i < ms.Len(); i++ {
		bindSelection(ms.At(i), at)
	}
}

func bindSelection(sn *types.Selection, at ast.Node) {
	fo, ok := sn.Obj().(*types.Func)
	if !ok {
		return
	}
	fo = fo.Origin()
	rtp := fo.Type().(*types.Signature).RecvTypeParams()
	if rtp.Len() == 0 {
		return
	}
	t := sn.Recv()
	idx := sn.Index()
	for _, i := range idx[:len(idx)-1] {
		if p, ok := t.Underlying().(*types.Pointer); ok {
			t = p.Elem()
		}
		st, ok := t.Underlying().(*types.Struct)
		if !ok {
			t = nil
			break
		}
		t = st.Field(i).Type()
	}
	var nt *types.Named
	if t != nil {
		if p, ok := types.Unalias(t).(*types.Pointer); ok {
			t = p.Elem()
		}
		nt, _ = types.Unalias(t).(*types.Named)
	}
	if nt == nil || nt.TypeArgs().Len() != rtp.Len() {
		cs, _ := Mentions(sn.Recv())
		Sink(Src{Cs: cs}, "a method of a generic type reached through an embedding that is not followed", at)
		return
	}
	for i := 0; i < rtp.Len(); i++ {
		cs, _ := Mentions(nt.TypeArgs().At(i))
		flowAt(Src{Cs: cs}, rtp.At(i).Obj(), at)
		tpArgs[rtp.At(i).Obj()] = append(tpArgs[rtp.At(i).Obj()], nt.TypeArgs().At(i))
	}
}

// store is a value stored into a destination of type dt; dsts are the destination's slots when
// it has any.
func store(v ast.Expr, dt types.Type, dsts []types.Object, what string) {
	tv, ok := rw.Info.Types[v]
	if !ok || tv.Type == nil || dt == nil || tv.IsNil() {
		return
	}
	if isFuncType(tv.Type) {
		funcStore(v, dsts)
		if phase == 1 || !IsIface(dt) {
			return
		}
	}
	if phase == 1 {
		return
	}
	{
		var ds []types.Object
		for _, d := range dsts {
			if !funcVar(d) {
				ds = append(ds, d)
			}
		}
		dsts = ds
	}
	s := Sources(v)
	if !IsSlotType(tv.Type) {
		if !IsIface(dt) {
			return // typed to typed: no conversion
		}
		if OnConvert != nil {
			OnConvert(s.Cs, s.Direct, dt, v)
		}
		if tp, ok := types.Unalias(tv.Type).(*types.TypeParam); ok {
			tpConv[tp.Obj()] = true
		}
		// held as an interface: the methods of the generic types in its method set may be called
		bindAll(tv.Type, v)
		if len(s.Cs) > 0 && !s.Direct {
			// a typed collection or function as an Object: its elements stay typed
			s.Ts = append(s.Ts, tv.Type)
			s.Cs = nil
		}
	} else if tps := tparams(dt); len(tps) > 0 {
		// an interface value given to a type parameter: it is in the generic code now
		if s.Empty() {
			return
		}
		for _, tp := range tps {
			if d := declTP[tp]; d != nil {
				tp = d
			}
			flowAt(s, tp, v)
		}
		return
	} else if !IsSlotType(dt) {
		return
	}
	if s.Empty() {
		return
	}
	if len(dsts) > 0 {
		for _, d := range dsts {
			flowAt(s, d, v)
		}
		return
	}
	Sink(s, what, v)
}

func enclosingSig(n ast.Node) (sig *types.Signature, lit *ast.FuncLit, fn *types.Func) {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch f := p.(type) {
		case *ast.FuncLit:
			sig, _ := rw.Info.Types[f].Type.(*types.Signature)
			return sig, f, nil
		case *ast.FuncDecl:
			if o, ok := rw.Info.Defs[f.Name].(*types.Func); ok {
				return o.Type().(*types.Signature), nil, o
			}
			return nil, nil, nil
		}
	}
	return nil, nil, nil
}

// EnclosingDecl is the function declaration around n.
func EnclosingDecl(n ast.Node) *ast.FuncDecl {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		if f, ok := p.(*ast.FuncDecl); ok {
			return f
		}
	}
	return nil
}

// lhsSlots are the slots a left side stores into; ok is false when the place is not followed.
func lhsSlots(e ast.Expr) (slots []types.Object, ok bool) {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		o := rw.Info.Defs[x]
		if o == nil {
			o = rw.Info.Uses[x]
		}
		if slotty(o) {
			return []types.Object{o}, true
		}
		return nil, o != nil
	case *ast.SelectorExpr:
		if o := rw.Info.Uses[x.Sel]; slotty(o) {
			return []types.Object{o}, true
		}
		return nil, false
	case *ast.IndexExpr:
		s := Sources(x.X)
		return s.Slots, len(s.Slots) > 0
	case *ast.SliceExpr:
		s := Sources(x.X)
		return s.Slots, len(s.Slots) > 0
	}
	return nil, false
}

func call(c *ast.CallExpr) {
	if isConversion(c) {
		return // Sources looks through it; the context decides
	}
	obj, name, recv := Callee(c)
	if _, isB := obj.(*types.Builtin); isB {
		switch name {
		case "panic":
			Sink(Sources(c.Args[0]), "thrown (panic)", c)
		case "append":
			t := rw.Info.Types[c].Type
			if t == nil {
				return
			}
			st, ok := t.Underlying().(*types.Slice)
			if ok && isFuncType(st.Elem()) {
				for _, a := range c.Args[1:] {
					if !c.Ellipsis.IsValid() {
						funcStore(a, nil)
					}
				}
				return
			}
			if !ok || !IsSlotType(st.Elem()) {
				return
			}
			dst := []types.Object{synthSlot(c, t)}
			store(c.Args[0], t, dst, "append")
			for _, a := range c.Args[1:] {
				if c.Ellipsis.IsValid() {
					store(a, t, dst, "append")
				} else {
					store(a, st.Elem(), dst, "append")
				}
			}
		case "copy":
			if dt := rw.Info.Types[c.Args[0]].Type; dt != nil && IsSlotType(dt) {
				ds, _ := lhsSlots(c.Args[0])
				if len(ds) == 0 {
					ds = Sources(c.Args[0]).Slots
				}
				store(c.Args[1], dt, ds, "copied into a slice that is not followed")
			}
		}
		return
	}
	f, _ := obj.(*types.Func)
	if recv != nil {
		rt := rw.Info.Types[recv].Type
		if EqualsSinks && rt != nil && IsIface(rt) && (name == "equals" || name == "hashCode") {
			Sink(Sources(recv), "x."+name+"() on a value of an interface type", c)
		}
		if tn := Named(rt); tn != nil && tn.Name() == "tconv_" && (name == "unbox" || name == "box" || name == "hash" || name == "eq") {
			return // modelled: box in Sources, unbox a cast, hash/eq typed
		}
	}
	if f == nil {
		// a call of a function value
		ts, ok := Targets(c.Fun)
		for _, t := range ts {
			if sig := tsig(t); sig == nil || sig.TypeParams().Len() > 0 || sig.RecvTypeParams().Len() > 0 {
				ok = false // a generic function as a value: its type parameters are not bound here
			}
		}
		if ok && phase == 2 {
			for _, t := range ts {
				sig := tsig(t)
				if sig == nil {
					continue
				}
				off := 0
				if t.Recv {
					off = 1
				}
				np := sig.Params().Len()
				for i, a := range c.Args {
					j := i - off
					if j < 0 {
						continue // the receiver: typed
					}
					var p *types.Var
					var pt types.Type
					switch {
					case sig.Variadic() && j >= np-1 && !c.Ellipsis.IsValid():
						p = sig.Params().At(np - 1)
						pt = p.Type().(*types.Slice).Elem()
					case j < np:
						p = sig.Params().At(j)
						pt = p.Type()
					default:
						continue
					}
					var ds []types.Object
					if slotty(p) {
						ds = []types.Object{p}
					}
					store(a, pt, ds, "argument of a function value")
				}
			}
			return
		}
		if tv := rw.Info.Types[c.Fun]; tv.Type != nil {
			if sig, ok := tv.Type.Underlying().(*types.Signature); ok {
				for i, a := range c.Args {
					if i < sig.Params().Len() || sig.Variadic() {
						var pt types.Type
						if sig.Variadic() && i >= sig.Params().Len()-1 && !c.Ellipsis.IsValid() {
							pt = sig.Params().At(sig.Params().Len() - 1).Type().(*types.Slice).Elem()
						} else {
							pt = sig.Params().At(i).Type()
						}
						store(a, pt, nil, "argument of a function value")
					}
				}
			}
		}
		return
	}
	sig := f.Type().(*types.Signature) // the declared (generic) signature
	if f.Pkg() != rw.Pkg {
		if extSafe(name) || name == "unsafe.Pointer" {
			return
		}
		if tv := rw.Info.Types[c.Fun]; tv.Type != nil {
			if is, ok := tv.Type.(*types.Signature); ok {
				sig = is // instantiated: a type parameter bound to an interface type converts
			}
		}
		for i, a := range c.Args {
			if extPass[name] && i == 0 {
				continue
			}
			var pt types.Type
			np := sig.Params().Len()
			if sig.Variadic() && i >= np-1 && !c.Ellipsis.IsValid() {
				pt = sig.Params().At(np - 1).Type().(*types.Slice).Elem()
			} else if i < np {
				pt = sig.Params().At(i).Type()
			}
			if pt != nil {
				store(a, pt, nil, "argument of "+name+" (another package)")
			}
		}
		return
	}
	if recv == nil && (name == "boxT_" || name == "boxValue_") {
		return // modelled in Sources
	}
	viaIface := sig.Recv() != nil && types.IsInterface(sig.Recv().Type())
	var impl []*types.Func
	generic := false
	if viaIface {
		impl, generic = Implementers(f)
	}
	np := sig.Params().Len()
	var isig *types.Signature // the instantiated signature
	if tv := rw.Info.Types[c.Fun]; tv.Type != nil {
		isig, _ = tv.Type.(*types.Signature)
	}
	for i, a := range c.Args {
		var p *types.Var
		var pt types.Type
		switch {
		case sig.Variadic() && i >= np-1 && !c.Ellipsis.IsValid():
			p = sig.Params().At(np - 1)
			pt = p.Type().(*types.Slice).Elem()
		case i < np:
			p = sig.Params().At(i)
			pt = p.Type()
		default:
			continue
		}
		if viaIface {
			if len(tparams(pt)) > 0 && isig != nil && i < isig.Params().Len() && IsIface(isig.Params().At(i).Type()) {
				// a method of a generic interface instantiated with an interface type
				store(a, isig.Params().At(i).Type(), nil, "argument of "+name+" of a generic interface")
				continue
			}
			if len(impl) == 0 {
				store(a, pt, nil, "argument of "+name+", which no method of the package implements")
				continue
			}
			var ds []types.Object
			for _, m := range impl {
				ms := m.Type().(*types.Signature)
				if i < ms.Params().Len() {
					mp := ms.Params().At(i)
					if slotty(mp) {
						ds = append(ds, mp)
					} else if tps := tparams(mp.Type()); len(tps) > 0 {
						for _, tp := range tps {
							if d := declTP[tp]; d != nil {
								tp = d
							}
							ds = append(ds, tp)
						}
					}
				}
			}
			_ = generic
			if len(ds) == 0 {
				continue // every implementation takes a typed value: the conversion ends there
			}
			store(a, pt, ds, "argument of "+name)
			continue
		}
		var ds []types.Object
		if slotty(p) {
			ds = []types.Object{p}
		}
		if tps := tparams(pt); len(tps) > 0 && isig != nil {
			// a typed value given to a type parameter that is instantiated with an interface
			// type: converted to it, and in the generic code from here on
			var ipt types.Type
			inp := isig.Params().Len()
			switch {
			case isig.Variadic() && i >= inp-1 && !c.Ellipsis.IsValid():
				ipt = isig.Params().At(inp - 1).Type().(*types.Slice).Elem()
			case i < inp:
				ipt = isig.Params().At(i).Type()
			}
			if tv := rw.Info.Types[a]; ipt != nil && IsIface(ipt) && tv.Type != nil && !IsSlotType(tv.Type) {
				ds = nil
				for _, tp := range tps {
					if d := declTP[tp]; d != nil {
						tp = d
					}
					ds = append(ds, tp)
				}
				store(a, ipt, ds, "argument of "+name)
				continue
			}
		}
		store(a, pt, ds, "argument of "+name)
	}
}

// DeclOfTP is a node at the type parameter's declaration (for a place in a message).
func DeclOfTP(tp *types.TypeName) ast.Node {
	return &ast.Ident{NamePos: tp.Pos(), Name: tp.Name()}
}

func isInterfaceMethod(f *types.Func) bool {
	sig := f.Type().(*types.Signature)
	return sig.Recv() != nil && types.IsInterface(sig.Recv().Type())
}

// Load loads the package for a step that uses the flow.
func Load(dir string) {
	rw.Info.Instances = map[*ast.Ident]types.Instance{}
	rw.Info.Implicits = map[ast.Node]types.Object{}
	rw.Load(dir)
	for _, f := range rw.Files {
		tf := rw.Fset.File(f.Pos())
		fn := tf.Name()
		IsG[tf] = strings.HasPrefix(fn[strings.LastIndex(fn, "/")+1:], "g_")
	}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok {
				continue
			}
			fo, _ := rw.Info.Defs[fd.Name].(*types.Func)
			if fo == nil {
				continue
			}
			DeclOf[fo] = fd
			{
				sg := fo.Type().(*types.Signature)
				for i := 0; i < sg.TypeParams().Len(); i++ {
					TPOwner[sg.TypeParams().At(i).Obj()] = fo.Name()
				}
				for i := 0; i < sg.RecvTypeParams().Len(); i++ {
					if tn := Named(sg.Recv().Type()); tn != nil {
						TPOwner[sg.RecvTypeParams().At(i).Obj()] = tn.Name() + "." + fo.Name()
					}
				}
			}
			if fd.Recv == nil {
				continue
			}
			ByName[fo.Name()] = append(ByName[fo.Name()], fo)
			sg := fo.Type().(*types.Signature)
			if rtp := sg.RecvTypeParams(); rtp.Len() > 0 {
				if tn := Named(sg.Recv().Type()); tn != nil {
					if nt, ok := tn.Type().(*types.Named); ok && nt.TypeParams().Len() == rtp.Len() {
						for i := 0; i < rtp.Len(); i++ {
							declTP[rtp.At(i).Obj()] = nt.TypeParams().At(i).Obj()
						}
					}
				}
			}
		}
	}
	for _, fs := range ByName {
		sort.Slice(fs, func(i, j int) bool { return fs[i].Pos() < fs[j].Pos() })
	}
	for _, n := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		if nt, ok := tn.Type().(*types.Named); ok {
			for i := 0; i < nt.TypeParams().Len(); i++ {
				TPOwner[nt.TypeParams().At(i).Obj()] = tn.Name() + " (a value stored in it)"
			}
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			if fl := st.Field(i); fl.Embedded() {
				if b := Named(fl.Type()); b != nil {
					Subs[b] = append(Subs[b], tn)
				}
			}
		}
	}
}

// findPass finds the functions that return a parameter of theirs as it is: a call's value is
// then the argument (per call), and the return is no edge to the function's result slot.
func findPass() map[*ast.ReturnStmt]bool {
	skip := map[*ast.ReturnStmt]bool{}
	for fo, fd := range DeclOf {
		if fd.Recv != nil || fd.Body == nil {
			continue
		}
		sig := fo.Type().(*types.Signature)
		if sig.Results().Len() != 1 || !IsSlotType(sig.Results().At(0).Type()) {
			continue
		}
		// never used as a value
		asValue := false
		for _, id := range rw.UsesOf[fo] {
			var e ast.Expr = id
			p := rw.Up(e)
			if ix, ok := p.(*ast.IndexExpr); ok && ix.X == e {
				e, p = ix, rw.Up(ix)
			}
			if c, ok := p.(*ast.CallExpr); !ok || rw.Unparen(c.Fun) != e {
				asValue = true
			}
		}
		if asValue {
			continue
		}
		params := map[types.Object]int{}
		for i := 0; i < sig.Params().Len(); i++ {
			params[sig.Params().At(i)] = i
		}
		assigned := map[types.Object]bool{}
		var rets []*ast.ReturnStmt
		nested := false
		ast.Inspect(fd.Body, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncLit:
				nested = true
			case *ast.AssignStmt:
				for _, l := range x.Lhs {
					if id, ok := rw.Unparen(l).(*ast.Ident); ok {
						if o := rw.Info.Uses[id]; o != nil {
							assigned[o] = true
						}
					}
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					if id, ok := rw.Unparen(x.X).(*ast.Ident); ok {
						if o := rw.Info.Uses[id]; o != nil {
							assigned[o] = true
						}
					}
				}
			case *ast.ReturnStmt:
				rets = append(rets, x)
			}
			return true
		})
		if nested {
			continue
		}
		var pass []int
		var passRets []*ast.ReturnStmt
		for _, r := range rets {
			if len(r.Results) != 1 {
				continue
			}
			e := rw.Unparen(r.Results[0])
			for {
				if c, ok := e.(*ast.CallExpr); ok && isConversion(c) {
					e = rw.Unparen(c.Args[0])
					continue
				}
				break
			}
			id, ok := e.(*ast.Ident)
			if !ok {
				continue
			}
			o := rw.Info.Uses[id]
			i, isParam := params[o]
			if !isParam || assigned[o] {
				continue
			}
			found := false
			for _, j := range pass {
				found = found || j == i
			}
			if !found {
				pass = append(pass, i)
			}
			passRets = append(passRets, r)
		}
		if len(pass) > 0 {
			sort.Ints(pass)
			passParam[fo] = pass
			for _, r := range passRets {
				skip[r] = true
			}
		}
	}
	return skip
}

// Build makes the flow graph of the loaded package.
func Build() {
	skipRet := findPass()
	for phase = 1; phase <= 2; phase++ {
		if phase == 2 {
			funcSolve()
		}
		for _, f := range rw.Files {
			ast.Inspect(f, func(n ast.Node) bool {
				switch x := n.(type) {
				case *ast.FuncDecl:
					if x.Recv == nil && (x.Name.Name == "boxT_" || x.Name.Name == "boxValue_") {
						return false // modelled by hand
					}
					if Skip[x] {
						return false
					}
				case *ast.KeyValueExpr:
					if k, ok := x.Key.(*ast.Ident); ok && k.Name == "box" {
						if cl, ok := rw.Parent[x].(*ast.CompositeLit); ok {
							if tn := Named(rw.Info.Types[cl].Type); tn != nil && tn.Name() == "tconv_" {
								return false // the element conversion's box: modelled where it is called
							}
						}
					}
				case *ast.CallExpr:
					call(x)
				case *ast.AssignStmt:
					if len(x.Lhs) == len(x.Rhs) {
						for i, l := range x.Lhs {
							if id, ok := l.(*ast.Ident); ok && id.Name == "_" {
								continue
							}
							lt := rw.Info.Types[l].Type
							ds, _ := lhsSlots(l)
							if ix, ok := rw.Unparen(l).(*ast.IndexExpr); ok {
								// m[k] = v: the key is stored too
								if mt := rw.Info.Types[ix.X].Type; mt != nil {
									if m, ok := mt.Underlying().(*types.Map); ok && IsSlotType(m.Key()) {
										store(ix.Index, m.Key(), ds, "a key stored through a pointer or into something that is not followed")
									}
								}
							}
							if lt == nil {
								if id, ok := l.(*ast.Ident); ok {
									if o := rw.Info.Defs[id]; o != nil {
										lt = o.Type()
									}
								}
							}
							store(x.Rhs[i], lt, ds, "stored through a pointer or into something that is not followed")
						}
					} else if len(x.Rhs) == 1 {
						tuple(x.Lhs, x.Rhs[0], x)
					}
				case *ast.ValueSpec:
					if len(x.Names) == len(x.Values) {
						for i, v := range x.Values {
							if o := rw.Info.Defs[x.Names[i]]; o != nil && x.Names[i].Name != "_" {
								var ds []types.Object
								if slotty(o) {
									ds = []types.Object{o}
								}
								store(v, o.Type(), ds, "stored in a variable")
							}
						}
					} else if len(x.Values) == 1 {
						ls := make([]ast.Expr, len(x.Names))
						for i, n := range x.Names {
							ls[i] = n
						}
						tuple(ls, x.Values[0], x)
					}
				case *ast.ReturnStmt:
					if skipRet[x] {
						break
					}
					sig, _, _ := enclosingSig(x)
					if sig == nil {
						break
					}
					if len(x.Results) == 1 && sig.Results().Len() > 1 {
						// return f(): the callee's results are this function's
						if c, ok := rw.Unparen(x.Results[0]).(*ast.CallExpr); ok {
							from := tupleSources(c, sig.Results().Len())
							for i := 0; i < sig.Results().Len(); i++ {
								rv := sig.Results().At(i)
								if from == nil {
									continue
								}
								if Tracked(rv) {
									flowAt(from[i], rv, x)
								}
							}
						}
						break
					}
					if len(x.Results) != sig.Results().Len() {
						break
					}
					for i, r := range x.Results {
						rv := sig.Results().At(i)
						var ds []types.Object
						if slotty(rv) {
							ds = []types.Object{rv}
						}
						store(r, rv.Type(), ds, "returned")
					}
				case *ast.CompositeLit:
					t := rw.Info.Types[x].Type
					if t == nil {
						break
					}
					switch u := t.Underlying().(type) {
					case *types.Struct:
						for i, el := range x.Elts {
							var fv *types.Var
							v := el
							if kv, ok := el.(*ast.KeyValueExpr); ok {
								if k, ok := kv.Key.(*ast.Ident); ok {
									fv, _ = rw.Info.Uses[k].(*types.Var)
								}
								v = kv.Value
							} else if i < u.NumFields() {
								fv = u.Field(i)
							}
							if fv == nil {
								continue
							}
							var ift types.Type // the field's type in this instantiation
							for j := 0; j < u.NumFields(); j++ {
								if u.Field(j).Name() == fv.Name() {
									ift = u.Field(j).Type()
								}
							}
							fv = fv.Origin()
							var ds []types.Object
							if slotty(fv) {
								ds = []types.Object{fv}
							}
							if tps := tparams(fv.Type()); len(tps) > 0 && ift != nil && IsIface(ift) {
								if tv := rw.Info.Types[v]; tv.Type != nil && !IsSlotType(tv.Type) {
									ds = nil
									for _, tp := range tps {
										ds = append(ds, tp)
									}
									store(v, ift, ds, "stored in a field of a generic struct")
									continue
								}
							}
							store(v, fv.Type(), ds, "stored in a field of another package's struct")
						}
					case *types.Slice, *types.Array, *types.Map:
						if !IsSlotType(t) {
							// functions in a collection are not followed
							for _, el := range x.Elts {
								if kv, ok := el.(*ast.KeyValueExpr); ok {
									el = kv.Value
								}
								if tv, ok := rw.Info.Types[el]; ok && isFuncType(tv.Type) {
									funcStore(el, nil)
								}
							}
							break
						}
						dst := []types.Object{synthSlot(x, t)}
						var et, kt types.Type
						switch c := u.(type) {
						case *types.Slice:
							et = c.Elem()
						case *types.Array:
							et = c.Elem()
						case *types.Map:
							et, kt = c.Elem(), c.Key()
						}
						for _, el := range x.Elts {
							if kv, ok := el.(*ast.KeyValueExpr); ok {
								if kt != nil {
									store(kv.Key, kt, dst, "")
								}
								el = kv.Value
							}
							if _, isLit := el.(*ast.CompositeLit); isLit && rw.Info.Types[el].Type == nil {
								continue
							}
							store(el, et, dst, "")
						}
					}
				case *ast.SelectorExpr:
					// x.equals / x.hashCode of an interface value taken as a function value
					if EqualsSinks && (x.Sel.Name == "equals" || x.Sel.Name == "hashCode") {
						if xt := rw.Info.Types[x.X]; xt.Type != nil && !xt.IsType() && IsIface(xt.Type) {
							if c, ok := rw.Up(x).(*ast.CallExpr); !ok || rw.Unparen(c.Fun) != ast.Expr(x) {
								Sink(Sources(x.X), "x."+x.Sel.Name+" of a value of an interface type, as a function value", x)
							}
						}
					}
				case *ast.SendStmt:
					Sink(Sources(x.Value), "sent on a channel", x)
				case *ast.UnaryExpr:
					if x.Op == token.AND {
						if c, ok := rw.Up(x).(*ast.CallExpr); ok && rw.Text(c.Fun) == "unsafe.Pointer" {
							break // the interface's words are read
						}
						if ds, _ := lhsSlots(x.X); len(ds) > 0 {
							if _, isLit := rw.Unparen(x.X).(*ast.CompositeLit); !isLit {
								for _, d := range ds {
									if funcVar(d) {
										fUnknown[d], fEscSlot[d] = true, true
									} else if phase == 2 {
										Sink(Src{Slots: []types.Object{d}}, "its address is taken", x)
									}
								}
							}
						}
					}
				case *ast.TypeSwitchStmt:
					as, ok := x.Assign.(*ast.AssignStmt)
					if !ok {
						break
					}
					ta, ok := rw.Unparen(as.Rhs[0]).(*ast.TypeAssertExpr)
					if !ok {
						break
					}
					s := Sources(ta.X)
					if len(s.Slots) == 0 {
						break
					}
					s.Cs, s.Ts = nil, nil
					for _, cc := range x.Body.List {
						if o := rw.Info.Implicits[cc]; o != nil && Tracked(o) {
							flowAt(s, o, cc)
						}
					}
				case *ast.RangeStmt:
					for _, kv := range []ast.Expr{x.Key, x.Value} {
						if kv != nil {
							if ds, _ := lhsSlots(kv); len(ds) == 1 && funcVar(ds[0]) {
								fUnknown[ds[0]] = true
							}
						}
					}
					xt := rw.Info.Types[x.X].Type
					if xt != nil && !IsSlotType(xt) && x.Tok == token.ASSIGN && phase == 2 {
						// for k, v = range typed: assigned to variables that may be interfaces
						var kt, vt types.Type
						switch u := xt.Underlying().(type) {
						case *types.Slice:
							vt = u.Elem()
						case *types.Array:
							vt = u.Elem()
						case *types.Map:
							kt, vt = u.Key(), u.Elem()
						case *types.Pointer:
							if a, ok := u.Elem().Underlying().(*types.Array); ok {
								vt = a.Elem()
							}
						}
						for i, kv := range []ast.Expr{x.Key, x.Value} {
							et := []types.Type{kt, vt}[i]
							if kv == nil || et == nil {
								continue
							}
							if lt := rw.Info.Types[kv].Type; lt != nil && IsIface(lt) && !IsSlotType(et) {
								cs, direct := Mentions(et)
								src := Src{Cs: cs, Direct: direct}
								if !direct {
									src = Src{Ts: []types.Type{et}}
									bindAll(et, x)
								}
								if ds, _ := lhsSlots(kv); len(ds) > 0 {
									for _, d := range ds {
										flowAt(src, d, x)
									}
								} else {
									Sink(src, "a range variable that is not followed", x)
								}
							}
						}
					}
					if xt == nil || !IsSlotType(xt) {
						break
					}
					s := Sources(x.X)
					s.Cs, s.Ts = nil, nil
					if len(s.Slots) == 0 {
						break
					}
					for _, kv := range []ast.Expr{x.Key, x.Value} {
						if kv == nil {
							continue
						}
						if ds, _ := lhsSlots(kv); len(ds) > 0 {
							for _, d := range ds {
								flowAt(s, d, x)
							}
						}
					}
				}
				return true
			})
		}
	}
	phase = 2
	// Instantiations of generic functions: the classes (and type parameters) in a type argument
	// are bound to the function's type parameter.
	ids := make([]*ast.Ident, 0, len(rw.Info.Instances))
	for id := range rw.Info.Instances {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].Pos() < ids[j].Pos() })
	for _, id := range ids {
		if skipped(id) {
			continue
		}
		inst := rw.Info.Instances[id]
		fo, isFunc := rw.Info.Uses[id].(*types.Func)
		if !isFunc || fo.Pkg() != rw.Pkg {
			continue
		}
		if fo.Name() == "boxT_" || fo.Name() == "boxValue_" {
			continue
		}
		tps := fo.Type().(*types.Signature).TypeParams()
		for i := 0; i < inst.TypeArgs.Len() && i < tps.Len(); i++ {
			cs, _ := Mentions(inst.TypeArgs.At(i))
			flowAt(Src{Cs: cs}, tps.At(i).Obj(), id)
			tpArgs[tps.At(i).Obj()] = append(tpArgs[tps.At(i).Obj()], inst.TypeArgs.At(i))
		}
	}
	// A method of a generic type named on a receiver: the receiver's type arguments are bound to
	// the method's receiver type parameters.
	sels := make([]*ast.SelectorExpr, 0, len(rw.Info.Selections))
	for sel := range rw.Info.Selections {
		sels = append(sels, sel)
	}
	sort.Slice(sels, func(i, j int) bool { return sels[i].Pos() < sels[j].Pos() })
	for _, sel := range sels {
		if !skipped(sel) {
			bindSelection(rw.Info.Selections[sel], sel)
		}
	}
	// An interface value stored into a generic object is read by any of its methods.
	for tp, d := range declTP {
		Succ[d] = append(Succ[d], tp)
	}
	// A value of a type parameter that is converted to an interface: when the type parameter is
	// bound to a generic type (a typed collection as T), that value's methods can be called.
	{
		var conv []*types.TypeName
		for tp := range tpConv {
			conv = append(conv, tp)
		}
		sort.Slice(conv, func(i, j int) bool { return conv[i].Pos() < conv[j].Pos() })
		seen := map[*types.TypeName]bool{}
		for len(conv) > 0 {
			tp := conv[0]
			conv = conv[1:]
			if seen[tp] {
				continue
			}
			seen[tp] = true
			for _, a := range tpArgs[tp] {
				if at, ok := types.Unalias(a).(*types.TypeParam); ok {
					conv = append(conv, at.Obj()) // bound to another type parameter: its bindings
					continue
				}
				if cs, direct := Mentions(a); len(cs) > 0 && !direct {
					bindAll(a, DeclOfTP(tp))
				}
			}
		}
	}
	// Functions and methods used as values: their results go to callers that are not seen.
	fos := make([]*types.Func, 0, len(DeclOf))
	for fo := range DeclOf {
		fos = append(fos, fo)
	}
	sort.Slice(fos, func(i, j int) bool { return fos[i].Pos() < fos[j].Pos() })
	for _, fo := range fos {
		slots := resultSlots(fo)
		if len(slots) == 0 {
			continue
		}
		if Escaped[FTarget{Fn: fo}] || Escaped[FTarget{Fn: fo, Recv: true}] {
			Sink(Src{Slots: slots}, "returned by a function that is used as a value where it is not followed", DeclOf[fo])
		}
	}
	var lits []*ast.FuncLit
	for t := range Escaped {
		if t.Lit != nil {
			lits = append(lits, t.Lit)
		}
	}
	sort.Slice(lits, func(i, j int) bool { return lits[i].Pos() < lits[j].Pos() })
	for _, lit := range lits {
		if rs := litResults(lit); len(rs) > 0 {
			Sink(Src{Slots: rs}, "returned by a function literal that is called where it is not followed", lit)
		}
	}
	// Which slots reach a sink.
	// (breadth first from the sinks, in the order of the source, so that the reason a slot gets
	// is the same on every run)
	objLess := func(a, b types.Object) bool {
		if a.Pos() != b.Pos() {
			return a.Pos() < b.Pos()
		}
		return a.Name() < b.Name()
	}
	pred := map[types.Object][]types.Object{}
	var from []types.Object
	for o := range Succ {
		from = append(from, o)
	}
	sort.Slice(from, func(i, j int) bool { return objLess(from[i], from[j]) })
	for _, o := range from {
		for _, t := range Succ[o] {
			pred[t] = append(pred[t], o)
		}
	}
	var queue []types.Object
	for o := range SinkWhy {
		queue = append(queue, o)
	}
	sort.Slice(queue, func(i, j int) bool { return objLess(queue[i], queue[j]) })
	for len(queue) > 0 {
		t := queue[0]
		queue = queue[1:]
		for _, o := range pred[t] {
			if _, ok := SinkWhy[o]; !ok {
				SinkWhy[o] = SinkWhy[t]
				queue = append(queue, o)
			}
		}
	}
	var classes []*types.TypeName
	for c := range Into {
		classes = append(classes, c)
	}
	sort.Slice(classes, func(i, j int) bool { return objLess(classes[i], classes[j]) })
	for _, c := range classes {
		var os []types.Object
		for o := range Into[c] {
			os = append(os, o)
		}
		sort.Slice(os, func(i, j int) bool { return objLess(os[i], os[j]) })
		for _, o := range os {
			at := Into[c][o]
			if w, ok := SinkWhy[o]; ok {
				r := w
				if i := strings.LastIndex(r, " ("); i > 0 {
					r = r[:i]
				}
				name := o.Name()
				if tp, isTP := o.(*types.TypeName); isTP {
					name = "type parameter " + name + " of " + TPOwner[tp]
				}
				sinkClass(c, r, at+" -> "+name+" -> "+w[strings.LastIndex(w, " (")+2:len(w)-1])
			}
		}
	}
}

// tupleSources are the sources of each result of a call that yields n values.
func tupleSources(c *ast.CallExpr, n int) []Src {
	obj, _, _ := Callee(c)
	fo, ok := obj.(*types.Func)
	if !ok || fo.Pkg() != rw.Pkg {
		return nil
	}
	out := make([]Src, n)
	fs := []*types.Func{fo}
	if isInterfaceMethod(fo) {
		fs, _ = Implementers(fo)
	}
	for _, f := range fs {
		rs := f.Type().(*types.Signature).Results()
		for i := 0; i < n && i < rs.Len(); i++ {
			if Tracked(rs.At(i)) {
				out[i].Slots = append(out[i].Slots, rs.At(i))
			}
		}
	}
	return out
}

// tuple is a, b = f() / v, ok = m[k] / v, ok = x.(T).
func tuple(lhs []ast.Expr, rhs ast.Expr, at ast.Node) {
	var from []Src
	switch r := rw.Unparen(rhs).(type) {
	case *ast.CallExpr:
		from = tupleSources(r, len(lhs))
	case *ast.IndexExpr, *ast.TypeAssertExpr:
		from = []Src{Sources(r.(ast.Expr))}
	}
	for _, l := range lhs {
		if ds, _ := lhsSlots(l); len(ds) == 1 && funcVar(ds[0]) {
			fUnknown[ds[0]] = true
		}
	}
	if phase == 1 {
		return
	}
	for i, l := range lhs {
		if i >= len(from) || len(from[i].Slots) == 0 {
			continue
		}
		if id, ok := l.(*ast.Ident); ok && id.Name == "_" {
			continue
		}
		var lt types.Type
		if tv, ok := rw.Info.Types[l]; ok {
			lt = tv.Type
		} else if id, ok := l.(*ast.Ident); ok {
			if o := rw.Info.Defs[id]; o != nil {
				lt = o.Type()
			}
		}
		if lt == nil || !IsSlotType(lt) {
			continue
		}
		s := Src{Slots: from[i].Slots}
		if ds, _ := lhsSlots(l); len(ds) > 0 {
			for _, d := range ds {
				flowAt(s, d, at)
			}
		} else {
			Sink(s, "stored through a pointer or into something that is not followed", at)
		}
	}
}
