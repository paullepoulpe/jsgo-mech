// Command valplain is a post-flip step: a class that is an immutable record whose identity no
// code observes is a Go struct used by value. It is valcls's second rule: valcls converts the
// classes that declare an all-field equals (their values are compared); this tool converts
// classes WITHOUT equals and hashCode (Java compares them by identity), when it proves that
// nothing ever compares, hashes or boxes them.
//
//	type SourcePosition struct{ source *SourceFile; offset, line_, column_ int32 }
//	func new_SourcePosition(...) *SourcePosition     ->  func new_SourcePosition(...) SourcePosition
//	func (this *SourcePosition) getOffset() int32    ->  func (this SourcePosition) getOffset() int32
//	type SourceRange struct{ start_, end_ *SourcePosition }  ->  struct{ start_, end_ SourcePosition }
//	location_ *SourceRange (a field of Token)        ->  location_ SourceRange
//	*TList[*SourcePosition] with tc_p_SourcePosition ->  *TList[SourcePosition] with tc_SourcePosition
//
// The hand port writes such classes as plain values (jsgo: parser.SourcePosition, SourceRange,
// `location SourceRange` parameters). The tool converts a class C when it proves:
//
//   - shape: C is a struct of the generated code with at least one field, no type parameters, no
//     equals or hashCode, no embedded class, no class embeds it, no self_ / vt_ dispatch; the
//     handwritten runtime does not name it; C is not already used by value.
//   - immutable: every write of a field of C is `this.f = ...` in C's zero_/finit_/init_ methods
//     (no field's address is taken; no field is a struct, an array or a type with pointer
//     methods, which code could change in place); those methods use `this` for nothing else,
//     and are called only on `this` by each other and by C's constructors, which are exactly
//     `this := &C{}; this.init_C(args without this)...; return this`: no copy of the object
//     exists before its last write.
//   - identity is never observed: every expression of type *C is a variable, a field, a call
//     result, an element, nil or the constructor's &C{}, and is used only to read a field, to
//     call a method of C, to be stored, passed or returned at type *C, to be compared with nil,
//     or to be printed (obj_(x) given to String_format with a literal format that has no %h,
//     objstr_, Object_toString, String_valueOf_Object or a StringBuilder; tostr_ / jtostr_ of
//     it). No == or != of two *C,
//     no switch on one, no conversion to an interface (no box that code can compare, hash, cast
//     back or use as a key), no type assertion to *C, no method value, no address of a variable
//     of the type, no map keyed by it, no field of a struct that is used by value or has no name
//     (its == would compare the pointer). Generic code is instantiated with *C only where the
//     tool knows that it leaves elements alone (below); any other generic type or function
//     (a set, a map, Optional, contains, a sort) may compare or hash and fails the class.
//   - size: the struct, with the fields that become values counted as such, is at most
//     -maxwords words (default 6: SourceRange, two positions of three words each, is what jsgo
//     passes by value).
//
// Null. Java's variables of the class may be null, a Go struct cannot. The tool decides, per
// variable ("slot": a field, a parameter, a result, a local), whether it becomes a C or stays a
// *C. The slots that stay pointers are the least set closed under:
//
//   - a slot that nil is assigned, passed or returned to, or that is compared with nil;
//   - a local or package variable declared without a value, a named result, a variable of a
//     comma-ok or several-result assignment, the result of a function that recovers a panic in
//     a deferred call (it returns the zero value);
//   - a field for which neither field proof holds: (1) fieldproof.go, optstr's: every allocation
//     of the class and of each class below it is `this := &T{}` followed by a constructor chain
//     that assigns the field before anything can read it; where the object escapes before, no
//     code the remaining statements can reach reads the field (the call graph of cha.go), or
//     the escape is `this.g = F(this)` with F a constructor that only stores it in fields that
//     no code ever reads but as `x.h.k` (inert: the object cannot be got out of them); (2)
//     every read of the field, anywhere, is `p.f` with p a never-assigned parameter or receiver
//     of a function that is only called directly, each call right after `v.f = ...` on the
//     variable it passes, or with such a parameter of its caller (a builder's field);
//   - E, one slot per class for every *C written inside another type (an argument of a generic
//     type or function, an element, a parameter or result of a func type, of a function used as
//     a value, of a method with the name of a method that is one: an interface's and its
//     implementations' signatures stay equal), when one of them is an element of a slice, array,
//     map or channel or a variadic parameter (their zero elements can be read);
//   - a slot that receives the value of a slot that stays a pointer.
//
// Every other slot holds only values that come from a constructor (by induction over the flow:
// each store into it is from a constructor call or from another such slot), so it is never null
// and its zero value is never read: it becomes a C. Where a value flows from a C slot into a *C
// slot the tool writes x.ref_() (a method it adds: `func (this C) ref_() *C { return &this }`,
// a fresh pointer to a copy; the same for a value printed through obj_, so a box is always a
// *C, as the class registries expect). A copy cannot be told from the original: no field
// changes after construction and nothing observes the address. The other direction does not
// exist (the pointer slots are closed under flow), so no nil is ever dereferenced into a value.
//
// Generic code that leaves elements alone (checked-in names, read in rt_tlist.go, rt_timm.go):
// the types TList / ImmutableList, ImmutableList_Builder and tconv_; the functions
// tlistEmptyImm_, immutableListBuilder_, immutableListBuilderN_; the methods get, size,
// isEmpty, add, build, and asList when the erased view is only printed (an argument of a
// printing function, or a new local whose every use is one). They store and return elements,
// make up no zero element (get throws out of bounds) and compare none; add's null check asks
// the conversion, and tc_C boxes a value as a non-null pointer to a copy (what erased code
// printed before) and unboxes such a pointer. When E becomes C these are instantiated with C.
//
// Methods take value receivers. A method called on an expression that stays a pointer
// dereferences it at the call, where the pointer-receiver method did at its first use of
// `this`; the tool requires such a method to read a field of `this` before anything else can
// happen (strict), so the NullPointerException of a null receiver comes at the same point. A
// method that tests its receiver for nil fails the class.
//
// Pays. A class that passes the proof is converted when
//
//   - some field of a struct becomes a value of it (the object is laid out inside its holder:
//     that allocation is gone; an object held only by locals, parameters and results gains
//     nothing certain);
//   - no func literal in its methods mentions the receiver (a captured value receiver is copied
//     to the heap at each call);
//   - the ref_() copies are at most one per -copies (default 10) type spellings that become
//     values (a copy is an allocation where a pointer was shared).
//
// Classes are independent (each has its own slots), except for size: a class is measured with
// the classes inside it already decided (innermost first), so SourceRange is measured with its
// two positions as values.
//
// Printed: per converted class its size, the type spellings rewritten, the copies, and the
// spellings that stay pointers by reason (-v: each one with the flow that makes it so); per
// class that is an immutable record but is left, the first property that fails and the number
// of places (-v); counted, the classes that fail shape or immutability. A rerun finds nothing
// to convert (a converted class is used by value).
//
// usage: valplain [-n] [-v] [-maxwords N] [-copies N] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strconv"
	"strings"

	"postflip/rw"
)

type class struct {
	n      *types.Named
	name   string
	ptr    types.Type
	fields map[*types.Var]bool
	why    string // the first property that fails ("" while a candidate)
	nwhy   map[string]int
	whyAt  string
	early  bool // failed shape or immutability (counted, not listed)
	news   []*ast.FuncDecl
	recvs  []recvCall
	boxes  []flow // values boxed for printing
	nstar  int
	nfield int // fields of structs that become values
	nref   int
	words  int64
	sized  bool
}

type recvCall struct {
	e ast.Expr
	m *types.Func
}

// flow: the value of expression e (source slot src; nil: not a slot) goes to slot dst.
type flow struct {
	c        *class
	e        ast.Expr
	src, dst any
	srcKind  int
}

const (
	kSlot  = iota // a variable, field or call result: the slot decides
	kNil          // nil: a pointer
	kFresh        // &C{} in the constructor: a value
	kBad
)

var (
	verbose  bool
	maxWords = int64(6)
	payRatio = 10
	classes  = map[*types.Named]*class{}
	order    []*class
	flows    []flow
	parent   = map[any]any{}    // union-find over slots (*types.Var, or the *class for its E)
	seed     = map[any]string{} // slot -> why it stays a pointer
	seedAt   = map[any]string{}
	lifeRecv = map[types.Object]*class{} // receivers of zero_/finit_/init_ methods
	valRecv  = map[types.Object]*class{} // receivers of the other methods
	sizes    = types.SizesFor("gc", "amd64")
)

func find(k any) any {
	for {
		p, ok := parent[k]
		if !ok || p == k {
			return k
		}
		k = p
	}
}

func union(a, b any) {
	a, b = find(a), find(b)
	if a != b {
		// the class's E is kept as the representative
		if _, ok := a.(*class); ok {
			parent[b] = a
		} else {
			parent[a] = b
		}
	}
}

func pin(k any, at ast.Node, why string) {
	if _, ok := seed[k]; !ok {
		seed[k] = why
		if at != nil {
			seedAt[k] = pos(at)
		}
	}
}

func pos(n ast.Node) string {
	p := rw.Fset.Position(n.Pos())
	return fmt.Sprintf("%s:%d", p.Filename[strings.LastIndex(p.Filename, "/")+1:], p.Line)
}

func (c *class) fail(at ast.Node, why string) {
	if c.nwhy == nil {
		c.nwhy = map[string]int{}
	}
	c.nwhy[why]++
	if c.why == "" {
		c.why = why
		if at != nil {
			c.whyAt = pos(at)
		}
	}
}

func main() {
	report := false
	args := os.Args[1 : len(os.Args)-1]
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "-n":
			report = true
		case "-v":
			verbose = true
		case "-copies":
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintln(os.Stderr, "valplain: -copies N")
				os.Exit(2)
			}
			payRatio = n
		case "-maxwords":
			i++
			n, err := strconv.Atoi(args[i])
			if err != nil {
				fmt.Fprintln(os.Stderr, "valplain: -maxwords N")
				os.Exit(2)
			}
			maxWords = int64(n)
		}
	}
	rw.Load(os.Args[len(os.Args)-1])
	index()
	candidates()
	scan()
	elems()
	fieldSeeds()
	null := solve()
	isVal := func(k any) bool { return !null[find(k)] }
	post(isVal)
	conv := rewrite(isVal, null)
	_ = conv

	// report
	early := map[string][]string{}
	var near []*class
	for _, c := range order {
		switch {
		case c.why == "":
		case c.early:
			early[c.why] = append(early[c.why], c.name)
		default:
			near = append(near, c)
		}
	}
	keys := []string{}
	for k := range early {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  not a candidate: %4d %s\n", len(early[k]), k)
		if verbose {
			fmt.Fprintf(os.Stderr, "        %s\n", strings.Join(early[k], " "))
		}
	}
	byWhy := map[string]int{}
	for _, c := range near {
		byWhy[c.why]++
	}
	keys = keys[:0]
	for k := range byWhy {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", byWhy[k], k)
	}
	if verbose {
		for _, c := range near {
			fmt.Fprintf(os.Stderr, "  left: %s: %s (%d places, first %s)\n", c.name, c.why, c.nwhy[c.why], c.whyAt)
		}
	}
	names := []string{}
	for _, c := range conv {
		names = append(names, c.name)
	}
	fmt.Fprintf(os.Stderr, "  %4d classes without equals and hashCode; %d immutable records; %d are values: %s\n", len(order), len(near)+len(conv), len(conv), strings.Join(names, " "))
	if report || len(conv) == 0 {
		fmt.Fprintln(os.Stderr, "valplain: nothing written")
		return
	}
	fmt.Fprintf(os.Stderr, "valplain: files changed: %d\n", rw.SaveChecked())
}

func namedOf(t types.Type) *types.Named {
	if t == nil {
		return nil
	}
	n, _ := types.Unalias(t).(*types.Named)
	if n == nil || n.Obj().Pkg() != rw.Pkg {
		return nil
	}
	return n.Origin()
}

func derefT(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		return p.Elem()
	}
	return t
}

// clsOf is the candidate class c when t is *c.
func clsOf(t types.Type) *class {
	if t == nil {
		return nil
	}
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	if n := namedOf(p.Elem()); n != nil {
		return classes[n]
	}
	return nil
}

func generated(n ast.Node) bool {
	name := rw.Fset.Position(n.Pos()).Filename
	return strings.HasPrefix(name[strings.LastIndex(name, "/")+1:], "g_")
}

func isLifecycle(c *class, name string) bool {
	return name == "zero_"+c.name || name == "finit_"+c.name || name == "init_"+c.name
}

func enclosingDecl(n ast.Node) *ast.FuncDecl {
	for p := ast.Node(n); p != nil; p = rw.Parent[p] {
		if fd, ok := p.(*ast.FuncDecl); ok {
			return fd
		}
	}
	return nil
}

// ---- shape and immutability

func candidates() {
	embedded := map[*types.Named]bool{}
	scope := rw.Pkg.Scope()
	for _, name := range scope.Names() {
		tn, ok := scope.Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		n, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		st, ok := n.Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			if st.Field(i).Embedded() {
				if e := namedOf(derefT(st.Field(i).Type())); e != nil {
					embedded[e] = true
				}
			}
		}
		if n.TypeParams().Len() != 0 {
			continue
		}
		p := types.NewPointer(n)
		if o, _, _ := types.LookupFieldOrMethod(p, true, rw.Pkg, "equals"); o != nil {
			continue
		}
		if o, _, _ := types.LookupFieldOrMethod(p, true, rw.Pkg, "hashCode"); o != nil {
			continue
		}
		c := &class{n: n, name: name, ptr: p, fields: map[*types.Var]bool{}}
		classes[n] = c
		order = append(order, c)
	}
	byValue := usedByValue()
	for _, c := range order {
		st := c.n.Underlying().(*types.Struct)
		early := func(why string) {
			if c.why == "" {
				c.why, c.early = why, true
			}
		}
		if embedded[c.n] {
			early("a class embeds it (a subclass)")
		}
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			if f.Embedded() {
				early("it embeds a class (a superclass)")
			}
			if f.Name() == "self_" || strings.HasPrefix(f.Name(), "vt_") {
				early("it dispatches through self_ / vt_")
			}
			switch f.Type().Underlying().(type) {
			case *types.Struct, *types.Array:
				early("a field held by value that can change in place (a struct, an array, a type with pointer methods)")
			case *types.Pointer, *types.Interface:
			default:
				// a named slice, map or number whose methods take its address
				if types.NewMethodSet(types.NewPointer(f.Type())).Len() > types.NewMethodSet(f.Type()).Len() {
					early("a field held by value that can change in place (a struct, an array, a type with pointer methods)")
				}
			}
			c.fields[f] = true
		}
		if st.NumFields() == 0 {
			early("no fields")
		}
		if byValue[c.n.Obj()] {
			early("already used by value")
		}
		if c.why == "" {
			if why := immutable(c); why != "" {
				early(why)
			}
		}
	}
	for _, c := range order {
		for i := 0; i < c.n.NumMethods(); i++ {
			m := c.n.Method(i)
			fd := funcDecls[m]
			if fd == nil {
				continue
			}
			if r := recvOf(fd); r != nil {
				if isLifecycle(c, m.Name()) {
					lifeRecv[r] = c
				} else {
					valRecv[r] = c
				}
			}
		}
	}
}

func recvOf(fd *ast.FuncDecl) types.Object {
	if fd.Recv == nil || len(fd.Recv.List) != 1 || len(fd.Recv.List[0].Names) != 1 {
		return nil
	}
	return rw.Info.Defs[fd.Recv.List[0].Names[0]]
}

// usedByValue: the struct types that some code holds otherwise than through a pointer (a
// variable, field, element or type argument of the struct type itself, new(T), a T{} that is
// not &T{}), and the structs they embed.
func usedByValue() map[*types.TypeName]bool {
	out := map[*types.TypeName]bool{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			id, ok := x.(*ast.Ident)
			if !ok {
				return true
			}
			tn, ok := rw.Info.Uses[id].(*types.TypeName)
			if !ok || tn.Pkg() != rw.Pkg {
				return true
			}
			if _, ok := tn.Type().Underlying().(*types.Struct); !ok {
				return true
			}
			var cur ast.Node = id
			switch p := rw.Parent[cur].(type) {
			case *ast.IndexExpr:
				if p.X == ast.Expr(id) {
					cur = p
				}
			case *ast.IndexListExpr:
				if p.X == ast.Expr(id) {
					cur = p
				}
			}
			switch p := rw.Parent[cur].(type) {
			case *ast.StarExpr:
				return true
			case *ast.CompositeLit:
				if u, ok := rw.Parent[p].(*ast.UnaryExpr); ok && u.Op == token.AND && p.Type == cur {
					return true
				}
			case *ast.Field:
				if _, ok := rw.Parent[rw.Parent[p]].(*ast.StructType); ok && len(p.Names) == 0 {
					return true // embedded: a superclass
				}
			}
			out[tn] = true
			return true
		})
	}
	// what a struct used by value embeds is used by value
	for changed := true; changed; {
		changed = false
		for tn, ss := range supers {
			if out[tn] {
				for _, s := range ss {
					if !out[s] {
						out[s], changed = true, true
					}
				}
			}
		}
	}
	return out
}

// immutable checks the constructors and the writes of c's fields.
func immutable(c *class) string {
	for i := 0; i < c.n.NumMethods(); i++ {
		if fd := funcDecls[c.n.Method(i)]; fd == nil || !generated(fd) {
			return "a method outside the generated code"
		}
	}
	// the constructors: every C{} literal is the allocation of a new_C function of the fixed shape
	n := 0
	for _, lit := range lits {
		if namedOf(rw.Info.TypeOf(lit)) != c.n {
			continue
		}
		n++
		fd := enclosingDecl(lit)
		if fd == nil || fd.Recv != nil || !strings.HasPrefix(fd.Name.Name, "new_"+c.name) || len(lit.Elts) != 0 || len(fd.Body.List) < 2 {
			return "allocated outside a new_ constructor"
		}
		as, ok := fd.Body.List[0].(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
			return "allocated outside a new_ constructor"
		}
		u, ok := as.Rhs[0].(*ast.UnaryExpr)
		if !ok || u.Op != token.AND || u.X != ast.Expr(lit) {
			return "allocated outside a new_ constructor"
		}
		this := rw.Info.Defs[as.Lhs[0].(*ast.Ident)]
		last := len(fd.Body.List) - 1
		for _, st := range fd.Body.List[1:last] {
			es, ok := st.(*ast.ExprStmt)
			if !ok {
				return "a constructor that does more than run init_"
			}
			call, ok := es.X.(*ast.CallExpr)
			if !ok {
				return "a constructor that does more than run init_"
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok || !isLifecycle(c, sel.Sel.Name) || !isObj(sel.X, this) {
				return "a constructor that does more than run init_"
			}
			for _, a := range call.Args {
				if mentions(a, this) {
					return "a constructor that does more than run init_"
				}
			}
		}
		r, ok := fd.Body.List[last].(*ast.ReturnStmt)
		if !ok || len(r.Results) != 1 || !isObj(r.Results[0], this) {
			return "a constructor that does more than run init_"
		}
		c.news = append(c.news, fd)
	}
	if n == 0 {
		return "no constructor"
	}
	// the lifecycle methods are called on `this` by constructors and by each other
	for i := 0; i < c.n.NumMethods(); i++ {
		m := c.n.Method(i)
		if !isLifecycle(c, m.Name()) {
			continue
		}
		for _, u := range rw.UsesOf[m] {
			fd := enclosingDecl(u)
			sel, _ := rw.Parent[u].(*ast.SelectorExpr)
			var call *ast.CallExpr
			if sel != nil {
				call, _ = rw.Parent[sel].(*ast.CallExpr)
			}
			if fd == nil || sel == nil || call == nil || call.Fun != ast.Expr(sel) {
				return "its constructor is run by more than new_ (a subclass, a reset)"
			}
			isNew := false
			for _, nf := range c.news {
				isNew = isNew || nf == fd
			}
			inLife := fd.Recv != nil && isLifecycle(c, fd.Name.Name) && namedOf(derefT(rw.Info.TypeOf(fd.Recv.List[0].Type))) == c.n && isObj(sel.X, recvOf(fd))
			if !isNew && !inLife {
				return "its constructor is run by more than new_ (a subclass, a reset)"
			}
		}
	}
	why := ""
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			if why != "" {
				return false
			}
			written := func(l ast.Expr, at ast.Node) {
				s, ok := rw.Unparen(l).(*ast.SelectorExpr)
				if !ok {
					return
				}
				if v, ok := rw.Info.Uses[s.Sel].(*types.Var); ok && c.fields[v] {
					fd := enclosingDecl(at)
					if fd == nil || fd.Recv == nil || !isLifecycle(c, fd.Name.Name) || !isObj(s.X, recvOf(fd)) {
						why = "a field is written outside its constructor"
					}
				}
			}
			switch n := x.(type) {
			case *ast.AssignStmt:
				for _, l := range n.Lhs {
					written(l, n)
				}
			case *ast.RangeStmt:
				if n.Key != nil {
					written(n.Key, nil)
				}
				if n.Value != nil {
					written(n.Value, nil)
				}
			case *ast.IncDecStmt:
				written(n.X, nil)
			case *ast.UnaryExpr:
				if n.Op == token.AND {
					if s, ok := rw.Unparen(n.X).(*ast.SelectorExpr); ok {
						if v, ok := rw.Info.Uses[s.Sel].(*types.Var); ok && c.fields[v] {
							why = "a field's address is taken"
						}
					}
				}
			}
			return true
		})
	}
	return why
}

func mentions(n ast.Node, o types.Object) bool {
	found := false
	ast.Inspect(n, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok && rw.Info.Uses[id] == o {
			found = true
		}
		return !found
	})
	return found
}

// ---- the uses

type callee struct {
	fn      *types.Func // a declared function or method (its origin)
	sig     *types.Signature
	lit     bool
	conv    bool
	builtin string
}

func calleeOf(call *ast.CallExpr) callee {
	if tv, ok := rw.Info.Types[call.Fun]; ok && tv.IsType() {
		return callee{conv: true}
	}
	fun := rw.Unparen(call.Fun)
	switch f := fun.(type) {
	case *ast.FuncLit:
		sig, _ := rw.Info.TypeOf(f).(*types.Signature)
		return callee{sig: sig, lit: true}
	case *ast.IndexExpr:
		fun = f.X
	case *ast.IndexListExpr:
		fun = f.X
	}
	var id *ast.Ident
	switch f := fun.(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	}
	if id != nil {
		switch o := rw.Info.Uses[id].(type) {
		case *types.Func:
			o = o.Origin()
			return callee{fn: o, sig: o.Type().(*types.Signature)}
		case *types.Builtin:
			return callee{builtin: o.Name()}
		}
	}
	return callee{}
}

func hasTypeParam(t types.Type) bool {
	switch u := types.Unalias(t).(type) {
	case *types.TypeParam:
		return true
	case *types.Pointer:
		return hasTypeParam(u.Elem())
	case *types.Slice:
		return hasTypeParam(u.Elem())
	case *types.Named:
		for i := 0; i < u.TypeArgs().Len(); i++ {
			if hasTypeParam(u.TypeArgs().At(i)) {
				return true
			}
		}
	}
	return false
}

func isIface(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

func is(t types.Type, c *class) bool { return t != nil && types.Identical(t, c.ptr) }

// printers take a value of a class only to print it (rt_core.go, rt_conv.go).
var printers = map[string]bool{"tostr_": true, "tostrNN_": true, "jtostr_": true, "tostrR_": true}

// printSinks take an Object only to print it.
var printSinks = map[string]bool{"String_format": true, "objstr_": true, "Object_toString": true, "String_valueOf_Object": true}

func printed(box *ast.CallExpr) bool {
	up, ok := rw.Up(box).(*ast.CallExpr)
	if !ok {
		return false
	}
	if n, _ := rw.CallName(up); printSinks[n] {
		return n != "String_format" || plainFormat(up)
	}
	if sel, ok := up.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "append_Object" {
		return strings.HasSuffix(types.TypeString(types.Unalias(rw.Info.TypeOf(sel.X)), nil), ".StringBuilder")
	}
	return false
}

// plainFormat: the format of a String_format call is a literal without %h / %H (the hash code:
// of an object without hashCode, its identity).
func plainFormat(call *ast.CallExpr) bool {
	if len(call.Args) == 0 {
		return false
	}
	lit, ok := rw.Unparen(call.Args[0]).(*ast.BasicLit)
	return ok && lit.Kind == token.STRING && !strings.Contains(lit.Value, "%h") && !strings.Contains(lit.Value, "%H")
}

// source: where the value of e (of type *c) comes from.
func source(e ast.Expr, c *class) (any, int) {
	switch x := e.(type) {
	case *ast.Ident:
		if x.Name == "nil" && rw.Info.Uses[x] == types.Universe.Lookup("nil") {
			return nil, kNil
		}
		o := rw.Info.Uses[x]
		if o == nil {
			o = rw.Info.Defs[x]
		}
		if v, ok := o.(*types.Var); ok {
			return v.Origin(), kSlot
		}
		c.fail(e, "a value of the class from a non-variable identifier")
	case *ast.SelectorExpr:
		if sel := rw.Info.Selections[x]; sel != nil && sel.Kind() == types.FieldVal {
			return sel.Obj().(*types.Var).Origin(), kSlot
		}
		c.fail(e, "a value of the class from a selector that is not a field")
	case *ast.CallExpr:
		cl := calleeOf(x)
		switch {
		case cl.conv:
			if len(x.Args) == 1 {
				if id, ok := rw.Unparen(x.Args[0]).(*ast.Ident); ok && id.Name == "nil" {
					return nil, kNil // (*C)(nil): a type token, no object
				}
			}
			c.fail(e, "a conversion to the class")
		case cl.builtin != "":
			c.fail(e, "a value of the class from "+cl.builtin)
		case cl.sig != nil:
			if cl.sig.Results().Len() != 1 {
				c.fail(e, "a value of the class from a call with several results")
				break
			}
			r := cl.sig.Results().At(0)
			if is(r.Type(), c) {
				return r, kSlot
			}
			return c, kSlot // a type parameter's value: generic code
		default:
			return c, kSlot // a func value's result
		}
	case *ast.UnaryExpr:
		if x.Op == token.AND {
			if _, ok := x.X.(*ast.CompositeLit); ok {
				return nil, kFresh
			}
		}
		c.fail(e, "a value of the class from a unary expression")
	case *ast.IndexExpr:
		return c, kSlot
	case *ast.TypeAssertExpr:
		c.fail(e, "a type assertion to the class (a cast of an Object)")
	default:
		c.fail(e, fmt.Sprintf("a value of the class from a %T", e))
	}
	return nil, kBad
}

func scan() {
	// methods of one name keep equal signatures (an interface and its implementations)
	type sigAt struct {
		name string
		res  bool
		i    int
	}
	groups := map[sigAt][]*types.Var{}
	generic := map[sigAt]bool{}
	addSig := func(fn *types.Func) {
		sig := fn.Type().(*types.Signature)
		for _, tup := range []*types.Tuple{sig.Params(), sig.Results()} {
			for i := 0; i < tup.Len(); i++ {
				k := sigAt{fn.Name(), tup == sig.Results(), i}
				if clsOf(tup.At(i).Type()) != nil {
					groups[k] = append(groups[k], tup.At(i))
				} else if hasTypeParam(tup.At(i).Type()) {
					generic[k] = true
				}
			}
		}
	}
	for fn, fd := range funcDecls {
		if fd.Recv != nil {
			addSig(fn)
		}
	}
	scope := rw.Pkg.Scope()
	for _, name := range scope.Names() {
		if tn, ok := scope.Lookup(name).(*types.TypeName); ok {
			if it, ok := tn.Type().Underlying().(*types.Interface); ok {
				for i := 0; i < it.NumExplicitMethods(); i++ {
					addSig(it.ExplicitMethod(i))
				}
			}
		}
	}
	gkeys := make([]sigAt, 0, len(groups))
	for k := range groups {
		gkeys = append(gkeys, k)
	}
	sort.Slice(gkeys, func(i, j int) bool {
		a, b := gkeys[i], gkeys[j]
		if a.name != b.name {
			return a.name < b.name
		}
		if a.res != b.res {
			return b.res
		}
		return a.i < b.i
	})
	for _, k := range gkeys {
		vs := groups[k]
		for _, v := range vs[1:] {
			if types.Identical(v.Type(), vs[0].Type()) {
				union(v, vs[0])
			}
		}
		if generic[k] {
			for _, v := range vs {
				c := clsOf(v.Type())
				union(v, c)
			}
		}
	}

	valueSig := func(sig *types.Signature) {
		if sig == nil {
			return
		}
		for _, tup := range []*types.Tuple{sig.Params(), sig.Results()} {
			for i := 0; i < tup.Len(); i++ {
				if c := clsOf(tup.At(i).Type()); c != nil {
					union(tup.At(i), c)
				}
			}
		}
	}

	for _, f := range rw.Files {
		gen := generated(f)
		ast.Inspect(f, func(x ast.Node) bool {
			switch n := x.(type) {
			case *ast.Ident:
				switch o := rw.Info.Uses[n].(type) {
				case *types.TypeName:
					if !gen {
						if c := classes[namedOf(o.Type())]; c != nil {
							c.fail(n, "the handwritten runtime names it")
						}
					}
				case *types.Func:
					// a function used as a value: its signature is a func type's
					var cur ast.Node = n
					if s, ok := rw.Parent[cur].(*ast.SelectorExpr); ok && s.Sel == n {
						cur = s
					}
					switch p := rw.Parent[cur].(type) {
					case *ast.IndexExpr:
						if p.X == cur {
							cur = p
						}
					case *ast.IndexListExpr:
						if p.X == cur {
							cur = p
						}
					}
					if call, ok := rw.Up(cur).(*ast.CallExpr); !ok || rw.Unparen(call.Fun) != cur {
						valueSig(o.Origin().Type().(*types.Signature))
					}
				}
			case *ast.FuncLit:
				if call, ok := rw.Up(n).(*ast.CallExpr); !ok || rw.Unparen(call.Fun) != ast.Expr(n) {
					sig, _ := rw.Info.TypeOf(n).(*types.Signature)
					valueSig(sig)
				}
				results(n.Type, n.Body, rw.Info.TypeOf(n))
			case *ast.FuncDecl:
				if fn, ok := rw.Info.Defs[n.Name].(*types.Func); ok && n.Body != nil {
					results(n.Type, n.Body, fn.Type())
				}
			case *ast.ValueSpec:
				for i, nm := range n.Names {
					v, ok := rw.Info.Defs[nm].(*types.Var)
					if !ok {
						continue
					}
					c := clsOf(v.Type())
					if c == nil {
						continue
					}
					if i > 0 {
						union(v, rw.Info.Defs[n.Names[0]])
					}
					if len(n.Values) == 0 {
						pin(v, nm, "a variable declared without a value")
					} else if v.Parent() == rw.Pkg.Scope() {
						pin(v, nm, "a package variable")
					}
					if len(n.Values) == 1 && len(n.Names) > 1 {
						pin(v, nm, "defined from a call with several results")
					}
				}
			case *ast.AssignStmt:
				if len(n.Rhs) == 1 && len(n.Lhs) > 1 {
					for _, l := range n.Lhs {
						if c := clsOf(typeOfLhs(l)); c != nil {
							if k, kind := source(rw.Unparen(l), c); kind == kSlot {
								pin(k, l, "assigned from a call with several results, a comma-ok")
							}
						}
					}
				}
			case *ast.RangeStmt:
				for _, kv := range []ast.Expr{n.Key, n.Value} {
					if kv == nil {
						continue
					}
					if c := clsOf(typeOfLhs(kv)); c != nil {
						if k, kind := source(rw.Unparen(kv), c); kind == kSlot {
							union(k, c) // an element of what is ranged over
						}
					}
				}
			case *ast.SwitchStmt:
				if n.Tag != nil {
					if c := clsOf(rw.Info.TypeOf(n.Tag)); c != nil {
						c.fail(n.Tag, "compared (identity): a switch on it")
					}
				}
			case *ast.Field:
				if st, ok := rw.Parent[rw.Parent[n]].(*ast.StructType); ok {
					if _, named := rw.Parent[st].(*ast.TypeSpec); !named {
						for _, c := range append(mentioned(rw.Info.TypeOf(n.Type)), clsOf(rw.Info.TypeOf(n.Type))) {
							if c != nil {
								c.fail(n, "a field of an unnamed struct type")
							}
						}
					}
				}
				// several names of one type spelling are one slot
				for i, nm := range n.Names {
					if i > 0 {
						if v, ok := rw.Info.Defs[nm].(*types.Var); ok && clsOf(v.Type()) != nil {
							union(v, rw.Info.Defs[n.Names[0]])
						}
					}
				}
			}
			e, ok := x.(ast.Expr)
			if !ok {
				return true
			}
			if _, isParen := e.(*ast.ParenExpr); isParen {
				return true
			}
			tv, ok := rw.Info.Types[e]
			if !ok || tv.IsType() {
				return true
			}
			c := clsOf(tv.Type)
			if id, ok := e.(*ast.Ident); ok && c == nil && id.Name == "nil" {
				// go/types leaves nil untyped where it is assigned, passed or returned
				c = clsOf(ctxType(e))
			}
			if c == nil {
				return true
			}
			if !gen {
				c.fail(e, "the handwritten runtime names it")
				return true
			}
			use(e, c)
			return true
		})
	}
}

// ---- E: the *C written inside other types

// The typed containers of the runtime whose code neither compares, hashes nor makes up an
// element (rt_tlist.go, rt_timm.go): a list keeps what add was given and get returns it.
var elemTypes = map[string]bool{"TList": true, "ImmutableList": true, "ImmutableList_Builder": true, "tconv_": true}

// Their functions and methods that only store, return or print elements (no zero value: get
// throws out of bounds; no comparison: not contains, indexOf, remove, equals; add's null check
// asks the conversion, which boxes a value as a non-null pointer).
var elemFuncs = map[string]bool{"tlistEmptyImm_": true, "immutableListBuilder_": true, "immutableListBuilderN_": true}
var elemMethods = map[string]bool{"get": true, "size": true, "isEmpty": true, "add": true, "build": true, "asList": true}

// Generic functions that keep their instantiation with *C whatever E is (their argument is
// given as a pointer: source's printers and obj_).
var boxFuncs = map[string]bool{"obj_": true, "tostr_": true, "tostrNN_": true, "jtostr_": true, "tostrR_": true}

func typeMentions(t types.Type, c *class, seen map[types.Type]bool) bool {
	if t == nil || seen[t] {
		return false
	}
	seen[t] = true
	if is(t, c) {
		return true
	}
	if a, ok := t.(*types.Alias); ok {
		for i := 0; i < a.TypeArgs().Len(); i++ {
			if typeMentions(a.TypeArgs().At(i), c, seen) {
				return true
			}
		}
	}
	switch u := types.Unalias(t).(type) {
	case *types.Pointer:
		return typeMentions(u.Elem(), c, seen)
	case *types.Slice:
		return typeMentions(u.Elem(), c, seen)
	case *types.Array:
		return typeMentions(u.Elem(), c, seen)
	case *types.Chan:
		return typeMentions(u.Elem(), c, seen)
	case *types.Map:
		return typeMentions(u.Key(), c, seen) || typeMentions(u.Elem(), c, seen)
	case *types.Signature:
		for _, tup := range []*types.Tuple{u.Params(), u.Results()} {
			for i := 0; i < tup.Len(); i++ {
				if typeMentions(tup.At(i).Type(), c, seen) {
					return true
				}
			}
		}
	case *types.Named:
		for i := 0; i < u.TypeArgs().Len(); i++ {
			if typeMentions(u.TypeArgs().At(i), c, seen) {
				return true
			}
		}
	}
	return false
}

// mentioned: the candidate classes that t has inside it (not t itself being *C).
func mentioned(t types.Type) []*class {
	if t == nil || clsOf(t) != nil {
		return nil
	}
	var out []*class
	var walk func(t types.Type, depth int)
	walk = func(t types.Type, depth int) {
		if t == nil || depth > 6 {
			return
		}
		if c := clsOf(t); c != nil {
			out = append(out, c)
			return
		}
		if a, ok := t.(*types.Alias); ok {
			for i := 0; i < a.TypeArgs().Len(); i++ {
				walk(a.TypeArgs().At(i), depth+1)
			}
		}
		switch u := types.Unalias(t).(type) {
		case *types.Pointer:
			walk(u.Elem(), depth+1)
		case *types.Slice:
			walk(u.Elem(), depth+1)
		case *types.Array:
			walk(u.Elem(), depth+1)
		case *types.Chan:
			walk(u.Elem(), depth+1)
		case *types.Map:
			walk(u.Key(), depth+1)
			walk(u.Elem(), depth+1)
		case *types.Signature:
			for _, tup := range []*types.Tuple{u.Params(), u.Results()} {
				for i := 0; i < tup.Len(); i++ {
					walk(tup.At(i).Type(), depth+1)
				}
			}
		case *types.Named:
			for i := 0; i < u.TypeArgs().Len(); i++ {
				walk(u.TypeArgs().At(i), depth+1)
			}
		}
	}
	walk(t, 0)
	return out
}

// printOnly: the erased view made at call (x.asList()) is only printed: it is an argument of a
// printing function, or the value of a new local variable whose every use is one.
func printOnly(call *ast.CallExpr) bool {
	isPrintArg := func(e ast.Expr) bool {
		up, ok := rw.Up(e).(*ast.CallExpr)
		if !ok {
			return false
		}
		n, _ := rw.CallName(up)
		return printSinks[n] && (n != "String_format" || plainFormat(up)) || n == "Strings_lenientFormat"
	}
	if isPrintArg(call) {
		return true
	}
	as, ok := rw.Up(call).(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != len(as.Rhs) {
		return false
	}
	for i, r := range as.Rhs {
		if rw.Unparen(r) != ast.Expr(call) {
			continue
		}
		id, ok := as.Lhs[i].(*ast.Ident)
		if !ok {
			return false
		}
		v := rw.Info.Defs[id]
		if v == nil || len(rw.UsesOf[v]) == 0 {
			return v != nil
		}
		for _, u := range rw.UsesOf[v] {
			if !isPrintArg(u) {
				return false
			}
		}
		return true
	}
	return false
}

// elems pins E of every class with a *C inside a type or generic code the lists above do not
// cover.
func elems() {
	convDecl := func(n ast.Node) bool {
		for p := n; p != nil; p = rw.Parent[p] {
			if vs, ok := p.(*ast.ValueSpec); ok && len(vs.Names) == 1 {
				if v, ok := rw.Info.Defs[vs.Names[0]].(*types.Var); ok && v.Parent() == rw.Pkg.Scope() {
					if n := namedOf(derefT(v.Type())); n != nil && n.Obj().Name() == "tconv_" {
						return true
					}
				}
			}
		}
		return false
	}
	ids := make([]*ast.Ident, 0, len(rw.Info.Instances))
	for id := range rw.Info.Instances {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].Pos() < ids[j].Pos() })
	for _, id := range ids {
		inst := rw.Info.Instances[id]
		for i := 0; i < inst.TypeArgs.Len(); i++ {
			ta := inst.TypeArgs.At(i)
			cs := mentioned(ta)
			if c := clsOf(ta); c != nil {
				cs = append(cs, c)
			}
			for _, c := range cs {
				switch o := rw.Info.Uses[id].(type) {
				case *types.TypeName:
					if !elemTypes[o.Name()] {
						c.fail(id, "an argument of generic code that may compare, hash or box it: "+o.Name())
					}
				case *types.Func:
					if !elemFuncs[o.Name()] && !boxFuncs[o.Name()] && !convDecl(id) {
						c.fail(id, "an argument of generic code that may compare, hash or box it: "+o.Name())
					}
				default:
					c.fail(id, "an argument of generic code that may compare, hash or box it")
				}
			}
		}
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			switch n := x.(type) {
			case *ast.SelectorExpr:
				tv, ok := rw.Info.Types[n.X]
				if !ok || tv.IsType() {
					return true
				}
				nt, _ := types.Unalias(derefT(tv.Type)).(*types.Named)
				if nt == nil || nt.TypeArgs().Len() == 0 {
					return true
				}
				for _, c := range mentioned(nt) {
					call, isCall := rw.Up(n).(*ast.CallExpr)
					isCall = isCall && rw.Unparen(call.Fun) == ast.Expr(n)
					switch {
					case !elemMethods[n.Sel.Name] || !isCall:
						c.fail(n, "a container of it used through a method not known to leave elements alone: "+n.Sel.Name)
					case n.Sel.Name == "asList" && !printOnly(call):
						c.fail(n, "a container of it seen as an erased collection")
					}
				}
			case *ast.StarExpr:
				tv, ok := rw.Info.Types[n]
				if !ok || !tv.IsType() {
					return true
				}
				c := clsOf(tv.Type)
				if c == nil {
					return true
				}
				if key, _, recv := slotOfType(n, c); recv || key != any(c) {
					return true
				}
				// a nested spelling: what it is nested in
				for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
					switch p.(type) {
					case *ast.ArrayType:
						pin(c, n, "an element of a slice or array")
					case *ast.MapType:
						if q := p.(*ast.MapType); q.Key.Pos() <= n.Pos() && n.End() <= q.Key.End() {
							c.fail(n, "a key of a map")
						}
						pin(c, n, "an element of a map")
					case *ast.ChanType:
						pin(c, n, "an element of a channel")
					case *ast.Ellipsis:
						pin(c, n, "a variadic parameter")
					case ast.Stmt, *ast.FuncDecl, *ast.GenDecl, *ast.CallExpr, *ast.CompositeLit:
						return true
					}
				}
			}
			return true
		})
	}
}

func typeOfLhs(l ast.Expr) types.Type {
	if id, ok := l.(*ast.Ident); ok {
		if o := rw.Info.Defs[id]; o != nil {
			return o.Type()
		}
	}
	return rw.Info.TypeOf(l)
}

// ctxType: the type the place of e expects (for nil).
func ctxType(e ast.Expr) types.Type {
	switch u := rw.Up(e).(type) {
	case *ast.CallExpr:
		if tv, ok := rw.Info.Types[u.Fun]; ok && tv.IsType() {
			return tv.Type
		}
		sig, _ := types.Unalias(rw.Info.TypeOf(u.Fun)).Underlying().(*types.Signature)
		if sig == nil {
			return nil
		}
		for i, a := range u.Args {
			if rw.Unparen(a) != e {
				continue
			}
			n := sig.Params().Len()
			if sig.Variadic() && i >= n-1 {
				t := sig.Params().At(n - 1).Type()
				if sl, ok := t.(*types.Slice); ok && !u.Ellipsis.IsValid() {
					return sl.Elem()
				}
				return t
			}
			if i < n {
				return sig.Params().At(i).Type()
			}
		}
	case *ast.AssignStmt:
		for i, r := range u.Rhs {
			if rw.Unparen(r) == e && len(u.Lhs) == len(u.Rhs) {
				return typeOfLhs(u.Lhs[i])
			}
		}
	case *ast.ValueSpec:
		for i, v := range u.Values {
			if rw.Unparen(v) == e && len(u.Names) == len(u.Values) {
				if o := rw.Info.Defs[u.Names[i]]; o != nil {
					return o.Type()
				}
			}
		}
	case *ast.ReturnStmt:
		sig := enclosingSig(u)
		for i, r := range u.Results {
			if rw.Unparen(r) == e && sig != nil && sig.Results().Len() == len(u.Results) {
				return sig.Results().At(i).Type()
			}
		}
	case *ast.BinaryExpr:
		if rw.Unparen(u.X) == e {
			return rw.Info.TypeOf(u.Y)
		}
		return rw.Info.TypeOf(u.X)
	case *ast.KeyValueExpr:
		if lit, ok := rw.Up(u).(*ast.CompositeLit); ok {
			switch t := rw.Info.TypeOf(lit).Underlying().(type) {
			case *types.Struct:
				if id, ok := u.Key.(*ast.Ident); ok {
					if v, ok := rw.Info.Uses[id].(*types.Var); ok {
						return v.Type()
					}
				}
			case *types.Map:
				return t.Elem()
			case *types.Slice:
				return t.Elem()
			}
		}
	case *ast.CompositeLit:
		switch t := rw.Info.TypeOf(u).Underlying().(type) {
		case *types.Slice:
			return t.Elem()
		case *types.Array:
			return t.Elem()
		}
	}
	return nil
}

// results: the named results of a function and the results of one that recovers a panic are
// zero values at some return.
func results(ft *ast.FuncType, body *ast.BlockStmt, t types.Type) {
	sig, ok := t.(*types.Signature)
	if !ok || ft.Results == nil {
		return
	}
	any := false
	for i := 0; i < sig.Results().Len(); i++ {
		r := sig.Results().At(i)
		if clsOf(r.Type()) == nil {
			continue
		}
		any = true
		if r.Name() != "" && r.Name() != "_" {
			pin(r, ft, "a named result")
		}
	}
	if !any {
		return
	}
	ast.Inspect(body, func(m ast.Node) bool {
		switch y := m.(type) {
		case *ast.FuncLit:
			return false
		case *ast.DeferStmt:
			ast.Inspect(y, func(k ast.Node) bool {
				if call, ok := k.(*ast.CallExpr); ok {
					if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "recover" {
						for i := 0; i < sig.Results().Len(); i++ {
							if clsOf(sig.Results().At(i).Type()) != nil {
								pin(sig.Results().At(i), ft, "the result of a function that recovers a panic")
							}
						}
					}
				}
				return true
			})
			return false
		}
		return true
	})
}

func enclosingSig(n ast.Node) *types.Signature {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		switch f := p.(type) {
		case *ast.FuncLit:
			sig, _ := rw.Info.TypeOf(f).(*types.Signature)
			return sig
		case *ast.FuncDecl:
			if fn, ok := rw.Info.Defs[f.Name].(*types.Func); ok {
				return fn.Type().(*types.Signature)
			}
			return nil
		}
	}
	return nil
}

// lvalue: the slot an assignment to l writes.
func lvalue(l ast.Expr, c *class) (any, bool) {
	switch x := rw.Unparen(l).(type) {
	case *ast.Ident:
		if x.Name == "_" {
			return nil, true
		}
		o := rw.Info.Defs[x]
		if o == nil {
			o = rw.Info.Uses[x]
		}
		if v, ok := o.(*types.Var); ok {
			return v.Origin(), true
		}
	case *ast.SelectorExpr:
		if sel := rw.Info.Selections[x]; sel != nil && sel.Kind() == types.FieldVal {
			return sel.Obj().(*types.Var).Origin(), true
		}
	case *ast.IndexExpr:
		return c, true
	}
	return nil, false
}

// use checks the one use of expression e, of type *c, and records where its value goes.
func use(e ast.Expr, c *class) {
	p := rw.Up(e)
	// the left-hand side of an assignment is a slot written, not a value used
	switch u := p.(type) {
	case *ast.AssignStmt:
		for _, l := range u.Lhs {
			if rw.Unparen(l) == e {
				return
			}
		}
	case *ast.RangeStmt:
		if u.Key == e || u.Value == e {
			return
		}
	case *ast.ValueSpec:
		for _, nm := range u.Names {
			if ast.Expr(nm) == e {
				return
			}
		}
	}
	src, kind := source(e, c)
	if kind == kBad {
		return
	}
	// `this` of a constructor method is the object being written: it is only selected from
	if id, ok := e.(*ast.Ident); ok {
		if lc := lifeRecv[rw.Info.Uses[id]]; lc != nil {
			if s, ok := p.(*ast.SelectorExpr); !ok || rw.Unparen(s.X) != e {
				lc.fail(e, "a constructor hands out the object it is writing")
			}
			return
		}
	}
	to := func(dst any) {
		if dst != nil {
			flows = append(flows, flow{c: c, e: e, src: src, dst: dst, srcKind: kind})
		}
	}
	toType := func(t types.Type, dst any, what string) {
		switch {
		case is(t, c):
			to(dst)
		case isIface(t):
			if kind == kNil {
				return // a nil pointer is no object (a type token)
			}
			c.fail(e, "converted to an interface (boxed): "+what)
		default:
			c.fail(e, "used at another type: "+what)
		}
	}
	switch u := p.(type) {
	case *ast.SelectorExpr:
		if rw.Unparen(u.X) != e {
			c.fail(e, "selected")
			return
		}
		sel := rw.Info.Selections[u]
		if sel == nil {
			c.fail(e, "a selector on the class")
			return
		}
		if sel.Kind() == types.FieldVal {
			return
		}
		m, _ := sel.Obj().(*types.Func)
		if m == nil || namedOf(derefT(m.Type().(*types.Signature).Recv().Type())) != c.n {
			c.fail(e, "a method not of the class called on it")
			return
		}
		if call, ok := rw.Up(u).(*ast.CallExpr); !ok || rw.Unparen(call.Fun) != ast.Expr(u) {
			c.fail(e, "a method value of the class")
			return
		}
		if !isLifecycle(c, m.Name()) {
			c.recvs = append(c.recvs, recvCall{e, m.Origin()})
		}
	case *ast.CallExpr:
		cl := calleeOf(u)
		arg := -1
		for i, a := range u.Args {
			if rw.Unparen(a) == e {
				arg = i
			}
		}
		if arg < 0 {
			c.fail(e, "called")
			return
		}
		switch {
		case cl.conv:
			toType(rw.Info.TypeOf(u.Fun), nil, "a conversion")
		case cl.builtin == "append" && arg > 0:
			to(c)
		case cl.builtin != "":
			c.fail(e, "passed to "+cl.builtin)
		case cl.sig != nil:
			n := cl.sig.Params().Len()
			i, variadic := arg, false
			if cl.sig.Variadic() && arg >= n-1 {
				i, variadic = n-1, !u.Ellipsis.IsValid()
			}
			if i >= n {
				c.fail(e, "passed beyond the parameters")
				return
			}
			pv := cl.sig.Params().At(i)
			pt := pv.Type()
			if variadic {
				pt = pt.(*types.Slice).Elem()
			}
			name := ""
			if cl.fn != nil {
				name = cl.fn.Name()
			}
			switch {
			case is(pt, c) && variadic:
				to(c)
			case is(pt, c):
				to(pv)
			case hasTypeParam(pt):
				switch {
				case name == "obj_":
					if !printed(u) {
						c.fail(e, "boxed (obj_) for more than printing")
						return
					}
					c.boxes = append(c.boxes, flow{c: c, e: e, src: src, srcKind: kind})
				case printers[name]:
					c.boxes = append(c.boxes, flow{c: c, e: e, src: src, srcKind: kind})
				default:
					to(c) // generic code: it keeps its instantiation with *C
				}
			default:
				toType(pt, nil, "an argument of "+name)
			}
		default:
			// a func value
			sig, _ := types.Unalias(rw.Info.TypeOf(u.Fun)).Underlying().(*types.Signature)
			if sig == nil || arg >= sig.Params().Len() || sig.Variadic() {
				c.fail(e, "passed to a func value")
				return
			}
			toType(sig.Params().At(arg).Type(), c, "an argument of a func value")
		}
	case *ast.AssignStmt:
		if u.Tok != token.ASSIGN && u.Tok != token.DEFINE {
			c.fail(e, "an assignment operator")
			return
		}
		for i, r := range u.Rhs {
			if rw.Unparen(r) != e {
				continue
			}
			if len(u.Lhs) != len(u.Rhs) {
				c.fail(e, "assigned in a tuple")
				return
			}
			dst, ok := lvalue(u.Lhs[i], c)
			if !ok {
				c.fail(e, "stored through "+fmt.Sprintf("%T", u.Lhs[i]))
				return
			}
			if dst == nil {
				return
			}
			toType(typeOfLhs(u.Lhs[i]), dst, "a store")
		}
	case *ast.ValueSpec:
		for i, v := range u.Values {
			if rw.Unparen(v) != e {
				continue
			}
			if len(u.Names) != len(u.Values) {
				c.fail(e, "assigned in a tuple")
				return
			}
			o := rw.Info.Defs[u.Names[i]]
			if o == nil {
				return // _
			}
			toType(o.Type(), o, "a variable")
		}
	case *ast.ReturnStmt:
		sig := enclosingSig(u)
		for i, r := range u.Results {
			if rw.Unparen(r) != e {
				continue
			}
			if sig == nil || sig.Results().Len() != len(u.Results) {
				c.fail(e, "returned in a tuple")
				return
			}
			toType(sig.Results().At(i).Type(), sig.Results().At(i), "a result")
		}
	case *ast.BinaryExpr:
		if u.Op != token.EQL && u.Op != token.NEQ {
			c.fail(e, "an operand")
			return
		}
		other := u.X
		if rw.Unparen(u.X) == e {
			other = u.Y
		}
		_, okind := source(rw.Unparen(other), c)
		if id, ok := rw.Unparen(other).(*ast.Ident); !ok || id.Name != "nil" || okind != kNil {
			if kind != kNil {
				c.fail(e, "compared (identity)")
			}
			return
		}
		if kind != kSlot {
			c.fail(e, "a constructor's result compared with nil")
			return
		}
		pin(src, e, "compared with nil")
	case *ast.KeyValueExpr:
		lit, _ := rw.Up(u).(*ast.CompositeLit)
		if lit == nil || u.Value != ast.Expr(e) && rw.Unparen(u.Value) != e {
			c.fail(e, "a key")
			return
		}
		if _, ok := rw.Info.TypeOf(lit).Underlying().(*types.Struct); ok {
			if id, ok := u.Key.(*ast.Ident); ok {
				if v, ok := rw.Info.Uses[id].(*types.Var); ok {
					toType(v.Type(), v.Origin(), "a field of a literal")
					return
				}
			}
		}
		c.fail(e, "an element of a literal")
	case *ast.CompositeLit:
		switch t := rw.Info.TypeOf(u).Underlying().(type) {
		case *types.Slice:
			toType(t.Elem(), c, "an element of a literal")
		case *types.Array:
			toType(t.Elem(), c, "an element of a literal")
		default:
			c.fail(e, "an element of a literal")
		}
	case *ast.ExprStmt:
	case *ast.DeferStmt:
	case *ast.GoStmt:
	case *ast.IndexExpr:
		c.fail(e, "an index or a key")
	case *ast.StarExpr:
		c.fail(e, "dereferenced as a whole")
	case *ast.UnaryExpr:
		c.fail(e, "its address taken")
	case *ast.CaseClause:
		c.fail(e, "compared (identity): a case")
	default:
		c.fail(e, fmt.Sprintf("used in %T", p))
	}
}

// ---- fields

func fieldSeeds() {
	byValue := usedByValue()
	fields := make([]*types.Var, 0, len(ownerOf))
	for f := range ownerOf {
		fields = append(fields, f)
	}
	sort.Slice(fields, func(i, j int) bool { return fields[i].Pos() < fields[j].Pos() })
	for _, f := range fields {
		tn := ownerOf[f]
		c := clsOf(f.Type())
		if c == nil || c.why != "" {
			continue
		}
		if byValue[tn] {
			// the struct's == would compare the pointer, and its zero value needs no constructor
			c.fail(nil, "a field of a struct used by value ("+tn.Name()+")")
			continue
		}
		if tn.Parent() != rw.Pkg.Scope() {
			c.fail(nil, "a field of a local struct type")
			continue
		}
		if _, done := seed[f]; done {
			continue
		}
		named, _ := tn.Type().(*types.Named)
		if named == nil || named.TypeParams().Len() > 0 {
			pin(f, nil, "a field of a generic struct")
			continue
		}
		if writtenBeforeReads(f) {
			continue
		}
		if why := fieldProof(tn, f); why != "" {
			if verbose {
				fmt.Fprintf(os.Stderr, "  field %s.%s stays a pointer: %s\n", tn.Name(), f.Name(), why)
			}
			pin(f, nil, "a field not proved assigned by its constructors")
		}
	}
}

// writtenBeforeReads: every read of field f, anywhere, is `p.f` with p a parameter or receiver
// that its function never assigns and that holds an object whose f was written before the call
// (onEntry). A builder's field that a setter writes right before the object is handed to the
// one constructor that reads it is the case: its zero value is never read.
func writtenBeforeReads(f *types.Var) bool {
	if len(rw.UsesOf[f]) == 0 {
		return false
	}
	nread := 0
	for _, id := range rw.UsesOf[f] {
		sel, ok := rw.Parent[id].(*ast.SelectorExpr)
		if !ok || sel.Sel != id {
			return false
		}
		if a, ok := rw.Parent[sel].(*ast.AssignStmt); ok && isLhs(a, sel) && a.Tok == token.ASSIGN {
			continue
		}
		nread++
		x, ok := sel.X.(*ast.Ident)
		if !ok {
			return false
		}
		fd, i := paramOf(x)
		if fd == nil || !onEntry(fd, i, f, map[*ast.FuncDecl]bool{}) {
			return false
		}
	}
	return nread > 0
}

// paramOf: id names a parameter (index i) or the receiver (i = -1) of the function declaration
// that directly encloses it (no func literal between), and the function never assigns it.
func paramOf(id *ast.Ident) (*ast.FuncDecl, int) {
	o := rw.Info.Uses[id]
	if o == nil {
		return nil, 0
	}
	var fd *ast.FuncDecl
	for p := rw.Parent[id]; p != nil; p = rw.Parent[p] {
		if _, ok := p.(*ast.FuncLit); ok {
			return nil, 0
		}
		if d, ok := p.(*ast.FuncDecl); ok {
			fd = d
			break
		}
	}
	if fd == nil || fd.Body == nil {
		return nil, 0
	}
	idx, found := 0, false
	if recvOf(fd) == o {
		idx, found = -1, true
	}
	i := 0
	for _, fl := range fd.Type.Params.List {
		for _, n := range fl.Names {
			if rw.Info.Defs[n] == o {
				idx, found = i, true
			}
			i++
		}
		if len(fl.Names) == 0 {
			i++
		}
	}
	if !found {
		return nil, 0
	}
	assigned := false
	ast.Inspect(fd.Body, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.AssignStmt:
			for _, l := range x.Lhs {
				if lid, ok := l.(*ast.Ident); ok && rw.Info.Uses[lid] == o {
					assigned = true
				}
			}
		case *ast.UnaryExpr:
			if lid, ok := x.X.(*ast.Ident); ok && x.Op == token.AND && rw.Info.Uses[lid] == o {
				assigned = true
			}
		case *ast.IncDecStmt:
			if lid, ok := x.X.(*ast.Ident); ok && rw.Info.Uses[lid] == o {
				assigned = true
			}
		}
		return !assigned
	})
	if assigned {
		return nil, 0
	}
	return fd, idx
}

var ifaceMethods map[string]bool

// onEntry: at every call of fd, the object passed as parameter i (the receiver for -1) has had
// its field f written: the argument is a variable v and the statement right before the call's
// statement is `v.f = ...`, or the argument is such a parameter of the caller, for which the
// same holds. fd is called only directly: it is never used as a value, and (a method) no
// interface declares a method of its name.
func onEntry(fd *ast.FuncDecl, i int, f *types.Var, busy map[*ast.FuncDecl]bool) bool {
	if busy[fd] {
		return false
	}
	busy[fd] = true
	defer delete(busy, fd)
	if ifaceMethods == nil {
		ifaceMethods = map[string]bool{}
		scope := rw.Pkg.Scope()
		for _, name := range scope.Names() {
			if tn, ok := scope.Lookup(name).(*types.TypeName); ok {
				if it, ok := tn.Type().Underlying().(*types.Interface); ok {
					for k := 0; k < it.NumMethods(); k++ {
						ifaceMethods[it.Method(k).Name()] = true
					}
				}
			}
		}
		for _, file := range rw.Files {
			ast.Inspect(file, func(n ast.Node) bool {
				if it, ok := n.(*ast.InterfaceType); ok {
					for _, m := range it.Methods.List {
						for _, nm := range m.Names {
							ifaceMethods[nm.Name] = true
						}
					}
				}
				return true
			})
		}
	}
	fn, _ := rw.Info.Defs[fd.Name].(*types.Func)
	if fn == nil || fd.Recv != nil && ifaceMethods[fd.Name.Name] {
		return false
	}
	if fd.Type.TypeParams != nil {
		return false
	}
	uses := rw.UsesOf[fn]
	if len(uses) == 0 {
		return false
	}
	for _, u := range uses {
		var cur ast.Expr = u
		var recvX ast.Expr
		if s, ok := rw.Parent[u].(*ast.SelectorExpr); ok && s.Sel == u {
			cur, recvX = s, s.X
		}
		call, ok := rw.Parent[cur].(*ast.CallExpr)
		if !ok || call.Fun != cur {
			return false // used as a value
		}
		var arg ast.Expr
		if i < 0 {
			arg = recvX
		} else if i < len(call.Args) && !call.Ellipsis.IsValid() {
			arg = call.Args[i]
		}
		if arg == nil {
			return false
		}
		aid, ok := rw.Unparen(arg).(*ast.Ident)
		if !ok {
			return false
		}
		v := rw.Info.Uses[aid]
		// the statement of the call, and the one before it
		var st ast.Stmt
		var blk *ast.BlockStmt
		for p := ast.Node(call); p != nil; p = rw.Parent[p] {
			if _, isLit := p.(*ast.FuncLit); isLit {
				break
			}
			if b, isBlk := rw.Parent[p].(*ast.BlockStmt); isBlk {
				st, blk = p.(ast.Stmt), b
				break
			}
		}
		written := false
		if blk != nil {
			for k, s := range blk.List {
				if s != st || k == 0 {
					continue
				}
				if a, ok := blk.List[k-1].(*ast.AssignStmt); ok && a.Tok == token.ASSIGN && len(a.Lhs) == 1 && len(a.Rhs) == 1 {
					if sel, ok := a.Lhs[0].(*ast.SelectorExpr); ok && isObj(sel.X, v) {
						if sl := rw.Info.Selections[sel]; sl != nil && sl.Obj() == f && !mentions(a.Rhs[0], v) {
							written = true
						}
					}
				}
			}
		}
		if written {
			continue
		}
		cfd, ci := paramOf(aid)
		if cfd == nil || !onEntry(cfd, ci, f, busy) {
			return false
		}
	}
	return true
}

// ---- which slots stay pointers

func solve() map[any]bool {
	null := map[any]bool{}
	next := map[any][]any{}
	nextAt := map[any][]flow{}
	for _, f := range flows {
		d := find(f.dst)
		if f.srcKind == kNil {
			pin(d, f.e, "nil is stored in it")
			continue
		}
		if f.srcKind == kSlot {
			next[find(f.src)] = append(next[find(f.src)], d)
			nextAt[find(f.src)] = append(nextAt[find(f.src)], f)
		}
	}
	var work []any
	for k := range seed {
		r := find(k)
		if !null[r] {
			null[r] = true
			work = append(work, r)
		}
	}
	for len(work) > 0 {
		k := work[len(work)-1]
		work = work[:len(work)-1]
		for i, d := range next[k] {
			if !null[d] {
				null[d] = true
				cause[d] = nextAt[k][i]
				work = append(work, d)
			}
		}
	}
	return null
}

// cause: the flow that made a slot a pointer (for -v).
var cause = map[any]flow{}

func slotName(k any) string {
	switch x := k.(type) {
	case *types.Var:
		if tn := ownerOf[x]; tn != nil {
			return tn.Name() + "." + x.Name()
		}
		p := rw.Fset.Position(x.Pos())
		return fmt.Sprintf("%s (%s:%d)", x.Name(), p.Filename[strings.LastIndex(p.Filename, "/")+1:], p.Line)
	case *class:
		return "E(" + x.name + ")"
	}
	return "?"
}

// chain: why slot k stays a pointer, back to a seed.
func chain(k any) string {
	r := find(k)
	out := ""
	for i := 0; i < 8; i++ {
		f, ok := cause[r]
		if !ok {
			break
		}
		out += " <- " + slotName(f.src) + " at " + pos(f.e)
		r = find(f.src)
	}
	return out + ": " + why(r, nil)
}

// strict: m reads a field of its receiver before anything else can happen, so a null receiver
// fails at the same point whether the call or the method's first statement dereferences it.
func strict(m *types.Func, busy map[*types.Func]bool) bool {
	fd := funcDecls[m]
	if fd == nil || fd.Body == nil || len(fd.Body.List) == 0 || busy[m] {
		return false
	}
	busy[m] = true
	defer delete(busy, m)
	this := recvOf(fd)
	var root ast.Node
	switch s := fd.Body.List[0].(type) {
	case *ast.ReturnStmt:
		root = s
	case *ast.ExprStmt:
		root = s
	case *ast.IfStmt:
		if s.Init != nil {
			return false
		}
		root = s.Cond
	default:
		return false
	}
	// the first dereference, in source order
	var first ast.Node
	ast.Inspect(root, func(n ast.Node) bool {
		if first != nil {
			return false
		}
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.SelectorExpr:
			if !isObj(x.X, this) {
				return true
			}
			sel := rw.Info.Selections[x]
			if sel == nil {
				return true
			}
			if sel.Kind() == types.FieldVal {
				first = x
				return false
			}
			if call, ok := rw.Parent[x].(*ast.CallExpr); ok && call.Fun == ast.Expr(x) && len(call.Args) == 0 {
				if cm, ok := sel.Obj().(*types.Func); ok && strict(cm.Origin(), busy) {
					first = call
					return false
				}
			}
		}
		return true
	})
	if first == nil {
		return false
	}
	ok := true
	ast.Inspect(root, func(n ast.Node) bool {
		if call, isCall := n.(*ast.CallExpr); isCall && call != first {
			if tv, isT := rw.Info.Types[call.Fun]; isT && tv.IsType() {
				return true
			}
			// a call that starts before the dereference and does not have it among its arguments
			inArgs := false
			for _, a := range call.Args {
				inArgs = inArgs || a.Pos() <= first.Pos() && first.End() <= a.End()
			}
			if call.Pos() < first.Pos() && !inArgs {
				ok = false
			}
		}
		return ok
	})
	return ok
}

// post: the checks that need the slots decided, and the sizes.
func post(isVal func(any) bool) {
	for _, c := range order {
		if c.why != "" {
			continue
		}
		for _, fd := range c.news {
			this := rw.Info.Defs[fd.Body.List[0].(*ast.AssignStmt).Lhs[0].(*ast.Ident)]
			if !isVal(this) {
				c.fail(fd, "the constructor's object is compared with nil")
			}
		}
		for i := 0; i < c.n.NumMethods(); i++ {
			fd := funcDecls[c.n.Method(i)]
			if fd == nil || isLifecycle(c, fd.Name.Name) {
				continue
			}
			this := recvOf(fd)
			if this == nil {
				continue
			}
			if !isVal(this) {
				c.fail(fd, "a method tests its receiver for nil")
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				if lit, ok := n.(*ast.FuncLit); ok && mentions(lit, this) {
					c.fail(lit, "does not pay: a func literal of a method holds the receiver (a value receiver would be copied to the heap)")
					return false
				}
				return true
			})
		}
		for _, r := range c.recvs {
			src, kind := source(r.e, c)
			if kind == kNil || kind == kSlot && !isVal(src) {
				if !strict(r.m, map[*types.Func]bool{}) {
					c.fail(r.e, "a method that does not read its receiver first is called on a value that may be null")
				}
			}
		}
	}
	var size func(c *class, busy map[*class]bool) int64
	size = func(c *class, busy map[*class]bool) int64 {
		if c.sized {
			return c.words
		}
		if busy[c] {
			c.fail(nil, "it holds itself by value")
			return 1
		}
		busy[c] = true
		st := c.n.Underlying().(*types.Struct)
		var offs, align int64 = 0, 1
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			sz, al := sizes.Sizeof(f.Type()), sizes.Alignof(f.Type())
			if fc := clsOf(f.Type()); fc != nil && fc.why == "" && isVal(f) {
				if w := size(fc, busy); fc.why == "" {
					sz, al = w*8, 8
				}
			}
			offs = (offs+al-1)/al*al + sz
			if al > align {
				align = al
			}
		}
		offs = (offs + align - 1) / align * align
		c.words, c.sized = (offs+7)/8, true
		delete(busy, c)
		if c.why == "" && c.words > maxWords {
			c.fail(nil, fmt.Sprintf("larger than %d words", maxWords))
		}
		return c.words
	}
	for _, c := range order {
		if c.why == "" {
			size(c, map[*class]bool{})
		}
	}
}

// ---- the rewrite

// slotOfType: the slot whose type the spelling star (a *C type expression) is; lifecycle: the
// receiver of a constructor method (stays a pointer); recv: another receiver (a value).
func slotOfType(star *ast.StarExpr, c *class) (key any, lifecycle, recv bool) {
	if pe, ok := rw.Parent[star].(*ast.ParenExpr); ok {
		if call, ok := rw.Parent[pe].(*ast.CallExpr); ok && call.Fun == ast.Expr(pe) {
			return nil, true, false // (*C)(nil): a type token, kept
		}
	}
	switch p := rw.Parent[star].(type) {
	case *ast.ValueSpec:
		if p.Type == ast.Expr(star) && len(p.Names) > 0 {
			if o := rw.Info.Defs[p.Names[0]]; o != nil {
				return o, false, false
			}
		}
	case *ast.Field:
		fl, _ := rw.Parent[p].(*ast.FieldList)
		if fl == nil || p.Type != ast.Expr(star) {
			return c, false, false
		}
		switch owner := rw.Parent[fl].(type) {
		case *ast.StructType:
			if len(p.Names) > 0 {
				if o := rw.Info.Defs[p.Names[0]]; o != nil {
					return o, false, false
				}
			}
		case *ast.FuncDecl:
			return nil, isLifecycle(c, owner.Name.Name), true
		case *ast.FuncType:
			var sig *types.Signature
			switch g := rw.Parent[owner].(type) {
			case *ast.FuncDecl:
				if fn, ok := rw.Info.Defs[g.Name].(*types.Func); ok {
					sig = fn.Type().(*types.Signature)
				}
			case *ast.FuncLit:
				sig, _ = rw.Info.TypeOf(g).(*types.Signature)
			case *ast.Field:
				if _, ok := rw.Parent[rw.Parent[g]].(*ast.InterfaceType); ok && len(g.Names) == 1 {
					if fn, ok := rw.Info.Defs[g.Names[0]].(*types.Func); ok {
						sig = fn.Type().(*types.Signature)
					}
				}
			}
			if sig == nil {
				return c, false, false
			}
			tup := sig.Params()
			if fl == owner.Results {
				tup = sig.Results()
			} else if fl != owner.Params {
				return c, false, false
			}
			i := 0
			for _, fld := range fl.List {
				if fld == p {
					break
				}
				if len(fld.Names) == 0 {
					i++
				} else {
					i += len(fld.Names)
				}
			}
			if i < tup.Len() && is(tup.At(i).Type(), c) {
				return tup.At(i), false, false
			}
		}
	}
	return c, false, false
}

func rewrite(isVal func(any) bool, null map[any]bool) []*class {
	valued := func(f flow) bool {
		return f.srcKind == kFresh || f.srcKind == kSlot && isVal(f.src)
	}
	// the places that become values and the copies, per class
	kept := map[*class]map[string]int{}
	var stars []*ast.StarExpr
	var lits []*ast.UnaryExpr
	var refs []flow
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			switch n := x.(type) {
			case *ast.StarExpr:
				tv, ok := rw.Info.Types[n]
				if !ok || !tv.IsType() {
					return true
				}
				c := clsOf(tv.Type)
				if c == nil || c.why != "" {
					return true
				}
				key, lifecycle, recv := slotOfType(n, c)
				if lifecycle {
					return false
				}
				if !recv && !isVal(key) {
					if kept[c] == nil {
						kept[c] = map[string]int{}
					}
					kept[c][why(key, null)]++
					if verbose && find(key) != any(c) {
						fmt.Fprintf(os.Stderr, "  kept *%s: %s%s\n", c.name, slotName(key), chain(key))
					}
					return false
				}
				c.nstar++
				if v, ok := key.(*types.Var); ok && v.IsField() {
					c.nfield++
				}
				stars = append(stars, n)
				return false
			case *ast.UnaryExpr:
				if n.Op == token.AND {
					if cl, ok := n.X.(*ast.CompositeLit); ok {
						if c := classes[namedOf(rw.Info.TypeOf(cl))]; c != nil && c.why == "" {
							lits = append(lits, n)
							return false
						}
					}
				}
			}
			return true
		})
	}
	for _, f := range flows {
		if f.c.why == "" && valued(f) && !isVal(f.dst) {
			f.c.nref++
			refs = append(refs, f)
		}
	}
	var conv []*class
	for _, c := range order {
		if c.why != "" {
			continue
		}
		for _, f := range c.boxes {
			if valued(f) {
				c.nref++
				refs = append(refs, f)
			}
		}
		if isVal(c) && len(convVars(c)) > 0 {
			c.nref++
		}
		// pays: an object stored in a field is laid out inside its holder (the allocation goes)
		if c.nfield == 0 {
			c.fail(nil, "does not pay: no field of a struct would hold it by value (only locals, parameters and results)")
			continue
		}
		// pays: the copies are few beside the places that hold a value
		if c.nref*payRatio > c.nstar {
			c.fail(nil, fmt.Sprintf("does not pay: more than one ref_() copy per %d places that become values", payRatio))
			if verbose {
				fmt.Fprintf(os.Stderr, "  left: %s: %d copies, %d places\n", c.name, c.nref, c.nstar)
			}
			continue
		}
		conv = append(conv, c)
	}
	for _, n := range stars {
		if c := clsOf(rw.Info.Types[n].Type); c.why == "" {
			name := c.name
			rw.Rewrite[n] = func() string { return name }
		}
	}
	for _, n := range lits {
		cl := n.X.(*ast.CompositeLit)
		if c := classes[namedOf(rw.Info.TypeOf(cl))]; c.why == "" {
			rw.Rewrite[n] = func() string { return rw.Render(cl) }
		}
	}
	for _, f := range refs {
		if f.c.why != "" {
			continue
		}
		e := f.e
		rw.Rewrite[e] = func() string {
			s := rw.RenderChildren(e)
			switch e.(type) {
			case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr:
			default:
				s = "(" + s + ")"
			}
			return s + ".ref_()"
		}
	}
	for _, c := range conv {
		c := c
		if isVal(c) {
			convertConv(c)
		}
		if c.nref > 0 {
			fd := c.news[len(c.news)-1]
			rw.Rewrite[fd] = func() string {
				return rw.RenderChildren(fd) + "\n\n// ref_ is a pointer to a copy of the value, for the places that hold a nullable " + c.name + " (post-flip).\nfunc (this " + c.name + ") ref_() *" + c.name + " { return &this }"
			}
		}
		fmt.Fprintf(os.Stderr, "  value: %s (%d words): %d type spellings rewritten (%d fields), %d ref_() copies", c.name, c.words, c.nstar, c.nfield, c.nref)
		ks := []string{}
		for k := range kept[c] {
			ks = append(ks, k)
		}
		sort.Strings(ks)
		for _, k := range ks {
			fmt.Fprintf(os.Stderr, "; %d kept *%s: %s", kept[c][k], c.name, k)
		}
		fmt.Fprintln(os.Stderr)
	}
	return conv
}

// convVars: the package variables that are the typed containers' conversion of *c.
func convVars(c *class) []*types.Var {
	var convs []*types.Var
	scope := rw.Pkg.Scope()
	for _, name := range scope.Names() {
		v, ok := scope.Lookup(name).(*types.Var)
		if !ok {
			continue
		}
		n, ok := types.Unalias(derefT(v.Type())).(*types.Named)
		if !ok || n.Obj().Name() != "tconv_" || n.TypeArgs().Len() != 1 || !is(n.TypeArgs().At(0), c) {
			continue
		}
		convs = append(convs, v)
	}
	return convs
}

// convertConv rewrites the typed containers' conversion of *C (tc_p_C) for elements that are
// values: tc_C boxes a value as a pointer to a copy (what erased code printed before) and
// unboxes such a pointer.
func convertConv(c *class) {
	convs := convVars(c)
	name := c.name
	tcName := "tc_" + name
	for i, v := range convs {
		var spec *ast.ValueSpec
		for _, f := range rw.Files {
			ast.Inspect(f, func(x ast.Node) bool {
				if vs, ok := x.(*ast.ValueSpec); ok && len(vs.Names) == 1 && rw.Info.Defs[vs.Names[0]] == v {
					spec = vs
				}
				return spec == nil
			})
		}
		if i == 0 {
			rw.Rewrite[spec] = func() string {
				return tcName + " = &tconv_[" + name + "]{\n\tbox:   func(v " + name + ") any { return v.ref_() },\n\tunbox: func(v any) " + name + " { return *cast_[*" + name + "](v) },\n}"
			}
		} else {
			gd := rw.Parent[spec]
			rw.Rewrite[gd] = func() string { return rw.Del }
		}
		for _, u := range rw.UsesOf[v] {
			rw.Rewrite[u] = func() string { return tcName }
		}
	}
}

// why: the reason a slot stays a pointer (its own, or that a pointer flows into it).
func why(key any, null map[any]bool) string {
	r := find(key)
	best := ""
	for k, w := range seed {
		if find(k) == r && (best == "" || w < best) {
			best = w
		}
	}
	if best == "" {
		return "a value that may be null flows into it"
	}
	return best
}

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
