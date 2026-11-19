// Command valobj is a post-flip step: the small immutable classes listed in classes.txt (next
// to this file) become Go values instead of heap objects.
//
//	type SourceRange struct{ start_, end_ *SourcePosition_util }   type SourceRange struct{ start_, end_ SourcePosition_util }
//	func new_SourceRange(s, e *SourcePosition_util) *SourceRange -> func new_SourceRange(s, e SourcePosition_util) SourceRange
//	func (this *SourceRange) toString() string                     func (this SourceRange) toString() string
//	location_ *SourceRange (a field of Token, ParseTree)           location_ SourceRange
//	obj_(this.start_) given to String_format                       any(this.start_)
//	tc_p_C = &tconv_[*C]{box: obj_[*C], unbox: cast_[*C]}          tc_p_C = &tconv_[C]{box: func(v C) any ..., unbox: ...}
//
// Go's collector makes an allocation much dearer than Java's, and the hand port writes such
// classes as values. The tool does not discover the classes: classes.txt names them, and says by
// hand what the tool cannot check cheaply (that a field holding one is assigned before it is
// read). For every listed class C the tool CHECKS the following on the type-checked package and
// refuses to run (exit 1, every failing site printed, nothing written) if one fails:
//
//   - shape: C is a non-generic struct of a g_ file that embeds no class, is embedded by none,
//     has no self_ / vt_ field and no equals / hashCode; no rt_ file names it; as a value (the
//     listed classes among its fields counted as values) it is at most 48 bytes and does not
//     contain itself. 48 bytes is a SourceRange of two SourcePositions: 8 scalar words, which the
//     amd64 register ABI still passes and returns in registers (9 integer registers), and no
//     more than the three heap objects (16 + 24 + 24 bytes, plus the 8-byte pointer) it replaces.
//   - immutable: a field of C is written only through the receiver of C's zero_ / finit_ /
//     init_ methods or through `this` in new_C*; no field's address is taken; those methods are
//     called only on `this` inside new_C* and each other; new_C* is `this := &C{}` ... `return
//     this`, the only allocations of C; inside them `this` is used only as `this.f` / `this.m()`
//     (never handed on), so the value new_C returns is the finished object and no copy is made
//     before it is finished.
//   - identity-free: no expression of type *C is an operand of == / != or a switch tag, has
//     its address taken, is asserted, or is converted to another type. Every *C value is used
//     only as the receiver of a field read or a called method of C, or stored / passed /
//     returned into a slot whose type is exactly *C (so no interface, no `any`, no map key or
//     set element: the only generic code instantiated with *C is the list below). The one
//     conversion allowed is obj_(x) as an argument of String_format / Strings_lenientFormat of
//     a class with its own toString: the box is only printed through C.toString (a toString
//     that throws inside lenientFormat prints an identity hash: an arbitrary number in Java
//     too). It becomes any(x): obj_ would box a value equal to the zero value as null. Likewise
//     tostr_(x) / tostrNN_(x) (String.valueOf / toString of a value that may be null) become
//     x.toString(): the value is never null.
//   - null-free, the part that is mechanical: no nil of type *C anywhere (assigned, passed,
//     returned, compared); no local of the type declared without a value, no named result, no
//     package variable, no make with a length, new, array, map, channel, pointer to *C, type
//     assertion, slice expression of []*C, keyed slice literal, or struct literal with fields;
//     no function with such a result recovers a panic; no struct with a field of the type is
//     used by value. Values are made by new_C*, and reach a slot only from an expression of type
//     *C: a call of a g_ function or method or func value declared to return *C, a parameter, a
//     local, an element of a []*C or of a list from the generic code below (which never yields a
//     zero: it throws), or a field.
//   - fields of type *C (in any class): each must be listed in classes.txt, as `field D.f`
//     (the hand argument there says why every constructor of D assigns it before anything
//     reads it; the tool checks only that it is assigned, and that every write of it is
//     `this.f = x` in an init_ / finit_ method of D unless the entry is `field+ D.f`, whose
//     later writes the list argues too) or as `nullable D.f`. A nullable field stays a pointer
//     (to a private copy): the tool checks that it is only assigned a value of the type
//     (rewritten to ptr_C(x), a copy on the heap), compared with nil, or dereferenced on the
//     spot (x.f.g, x.f.m(): a null there throws as before). No zero value stands for null.
//   - generic code: the runtime's generic types and functions that may be instantiated with *C
//     are TList / ImmutableList (get, size, isEmpty, the field items, and asImm only as
//     obj_(l.asImm()) given to a format call), ImmutableList_Builder (add, build), tconv_ (the
//     conversion tc_p_C, given only as an argument), immutableListOf_, immutableListBuilder_,
//     immAdd_, immBuilt_, sliceAt_, and append / len / cap / range / index on []*C. Read by
//     hand (rt_tlist.go, rt_timm.go, rt_native.go): they store and return elements, throw on a
//     bad index, test `c.box(x) == nil` (never true for a value: the box is the value), and box
//     elements only for the erased twin that a format call prints. None compares, hashes or
//     unboxes an element. Any other instantiation fails the check.
//
// Why that is enough: with no null and no identity use, a pointer to an immutable object and a
// copy of its fields cannot be told apart: every read gives the same field values, every method
// runs on the same fields, and printing goes through toString. The classes are converted
// together; each check is made on the program as it is, and the conversion of one class changes
// no value another class's check speaks of (a zero SourceRange that nothing reads holds two zero
// SourcePositions that nothing reads).
//
// Rewrites: every type spelling *C is C (not the receivers of zero_/finit_/init_, which keep
// writing through the pointer to new_C's local, and not a nullable field); `&C{}` is `C{}`;
// obj_(x) is any(x); the element conversion boxes the value itself; a nullable field's
// assignments take ptr_C(x), declared after the type. A rerun finds the classes already values
// and changes nothing.
//
// usage: valobj [-n] <dir of package closure>     (-n: check and report only)
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"runtime"
	"sort"
	"strings"

	"postflip/rw"
)

const sizeLimit = 48

type class struct {
	name      string
	tn        *types.TypeName
	named     *types.Named
	spec      *ast.TypeSpec
	fields    map[*types.Var]bool
	lifecycle map[*types.Func]bool
	news      map[*ast.FuncDecl]bool
	thisVars  map[types.Object]bool // `this` of new_C* and of the lifecycle methods
	toString  bool
	conv      map[*types.Var]*ast.ValueSpec // tc_p_C
	boxes     []*ast.CallExpr               // obj_(x) -> any(x)
	tostrs    []*ast.CallExpr               // tostr_(x) -> x.toString()
	nstars    int
	done      bool // already a value (a rerun)
}

type failure struct {
	pos token.Position
	msg string
}

var (
	classes   []*class
	byNamed   = map[*types.Named]*class{}
	listed    = map[string]string{} // "D.f" -> "field" | "field+" | "nullable"
	listSeen  = map[string]bool{}
	nullable  = map[*types.Var]*class{}
	nullWrite []*ast.AssignStmt
	failures  []failure
	declOf    = map[*types.Func]*ast.FuncDecl{}
	counts    = map[string]int{}
)

func fail(n ast.Node, c *class, why string) {
	failures = append(failures, failure{rw.Fset.Position(n.Pos()), c.name + ": " + why})
}

func short(n ast.Node) string {
	s := strings.Join(strings.Fields(rw.Text(n)), " ")
	if len(s) > 80 {
		s = s[:80] + "..."
	}
	return s
}

func generated(n ast.Node) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "g_")
}

func readList() []string {
	_, self, _, _ := runtime.Caller(0)
	b, err := os.ReadFile(filepath.Join(filepath.Dir(self), "classes.txt"))
	if err != nil {
		b, err = os.ReadFile(filepath.Join("valobj", "classes.txt"))
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "valobj:", err)
		os.Exit(1)
	}
	var names []string
	for i, line := range strings.Split(string(b), "\n") {
		if j := strings.Index(line, "#"); j >= 0 {
			line = line[:j]
		}
		w := strings.Fields(line)
		switch {
		case len(w) == 0:
		case len(w) == 2 && w[0] == "class":
			names = append(names, w[1])
		case len(w) == 2 && (w[0] == "field" || w[0] == "field+" || w[0] == "nullable"):
			listed[w[1]] = w[0]
		default:
			fmt.Fprintf(os.Stderr, "valobj: classes.txt:%d: not `class C`, `field D.f`, `field+ D.f` or `nullable D.f`\n", i+1)
			os.Exit(1)
		}
	}
	return names
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
	}
	names := readList()
	rw.Load(os.Args[len(os.Args)-1])
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok {
				if fn, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
					declOf[fn] = fd
				}
			}
		}
	}
	for _, name := range names {
		tn, _ := rw.Pkg.Scope().Lookup(name).(*types.TypeName)
		if tn == nil {
			fmt.Fprintf(os.Stderr, "valobj: classes.txt: no type %s\n", name)
			os.Exit(1)
		}
		named, _ := tn.Type().(*types.Named)
		if named == nil || tn.IsAlias() {
			fmt.Fprintf(os.Stderr, "valobj: classes.txt: %s is not a declared type\n", name)
			os.Exit(1)
		}
		c := &class{name: name, tn: tn, named: named, fields: map[*types.Var]bool{},
			lifecycle: map[*types.Func]bool{}, news: map[*ast.FuncDecl]bool{}, thisVars: map[types.Object]bool{}, conv: map[*types.Var]*ast.ValueSpec{}}
		classes = append(classes, c)
		byNamed[named] = c
	}
	for _, c := range classes {
		shape(c)
	}
	if len(failures) == 0 {
		for _, c := range classes {
			spellings(c)
		}
		variables()
		instances()
		expressions()
		recovers()
		todo := false
		for _, c := range classes {
			todo = todo || !c.done
		}
		for k := range listed {
			if !listSeen[k] && todo {
				failures = append(failures, failure{token.Position{Filename: "classes.txt"}, "classes.txt: " + k + " is not a field of a listed class's type"})
			}
		}
	}
	if len(failures) > 0 {
		sort.SliceStable(failures, func(i, j int) bool {
			a, b := failures[i].pos, failures[j].pos
			if a.Filename != b.Filename {
				return a.Filename < b.Filename
			}
			return a.Offset < b.Offset
		})
		per := map[string]int{}
		first := map[string]string{}
		for _, f := range failures {
			fmt.Fprintf(os.Stderr, "  %s:%d: %s\n", filepath.Base(f.pos.Filename), f.pos.Line, f.msg)
			parts := strings.SplitN(f.msg, ": ", 2)
			if per[parts[0]]++; per[parts[0]] == 1 && len(parts) == 2 {
				first[parts[0]] = parts[1]
			}
		}
		for _, c := range classes {
			if per[c.name] > 0 {
				fmt.Fprintf(os.Stderr, "  left: %s: %d sites, the first: %s\n", c.name, per[c.name], first[c.name])
			}
		}
		fmt.Fprintf(os.Stderr, "valobj: %d checks failed, nothing written\n", len(failures))
		os.Exit(1)
	}
	total := 0
	for _, c := range classes {
		if c.done {
			fmt.Fprintf(os.Stderr, "  %s: already a value\n", c.name)
			continue
		}
		rewrite(c)
		total += c.nstars
		nn := []string{}
		for f, fc := range nullable {
			if fc == c {
				nn = append(nn, f.Name())
			}
		}
		sort.Strings(nn)
		fmt.Fprintf(os.Stderr, "  %s: a value of %d bytes; %d type spellings, %d boxes for printing, %d tostr_, %d element conversions, %d nullable fields kept as pointers %v\n",
			c.name, valueSize(c.named, nil), c.nstars, len(c.boxes), len(c.tostrs), len(c.conv), len(nn), nn)
	}
	keys := []string{}
	for k := range counts {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  checked: %5d %s\n", counts[k], k)
	}
	if report || total == 0 {
		fmt.Fprintf(os.Stderr, "valobj: all checks pass; nothing written\n")
		return
	}
	fmt.Fprintf(os.Stderr, "valobj: %d classes; files changed: %d\n", len(classes), rw.SaveChecked())
}

// ---- types

func namedOf(t types.Type) *types.Named {
	if t == nil {
		return nil
	}
	n, _ := types.Unalias(t).(*types.Named)
	if n == nil {
		return nil
	}
	return n.Origin()
}

// ptrClass: t is *C of a listed class.
func ptrClass(t types.Type) *class {
	if t == nil {
		return nil
	}
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	if n, ok := types.Unalias(p.Elem()).(*types.Named); ok {
		return byNamed[n]
	}
	return nil
}

// Kinds of a type that mentions a listed class.
const (
	kNone    = iota
	kDirect  // *C
	kSlice   // []*C
	kList    // *TList[*C]
	kBuilder // *ImmutableList_Builder[*C]
	kConv    // *tconv_[*C] (or tconv_[*C], the literal)
	kFunc    // a func type
	kValue   // C itself
	kOther
)

var mentionMemo = map[types.Type]map[*class]bool{}

// mentioned: the listed classes t names (not through the fields of a named struct).
func mentioned(t types.Type) map[*class]bool {
	if t == nil {
		return nil
	}
	if m, ok := mentionMemo[t]; ok {
		return m
	}
	m := map[*class]bool{}
	mentionMemo[t] = m
	add := func(u types.Type) {
		for c := range mentioned(u) {
			m[c] = true
		}
	}
	switch u := types.Unalias(t).(type) {
	case *types.Named:
		if c := byNamed[u.Origin()]; c != nil {
			m[c] = true
		}
		for i := 0; i < u.TypeArgs().Len(); i++ {
			add(u.TypeArgs().At(i))
		}
	case *types.Pointer:
		add(u.Elem())
	case *types.Slice:
		add(u.Elem())
	case *types.Array:
		add(u.Elem())
	case *types.Chan:
		add(u.Elem())
	case *types.Map:
		add(u.Key())
		add(u.Elem())
	case *types.Tuple:
		for i := 0; i < u.Len(); i++ {
			add(u.At(i).Type())
		}
	case *types.Signature:
		add(u.Params())
		add(u.Results())
	case *types.Struct:
		for i := 0; i < u.NumFields(); i++ {
			add(u.Field(i).Type())
		}
	}
	return m
}

func genericOf(t types.Type, name string) bool {
	n, ok := types.Unalias(t).(*types.Named)
	return ok && n.Obj().Pkg() == rw.Pkg && n.Obj().Name() == name && n.TypeArgs().Len() == 1 && ptrClass(n.TypeArgs().At(0)) != nil
}

func kindOf(t types.Type) int {
	if len(mentioned(t)) == 0 {
		return kNone
	}
	switch u := types.Unalias(t).(type) {
	case *types.Named:
		if byNamed[u.Origin()] != nil {
			return kValue
		}
		if genericOf(u, "tconv_") {
			return kConv
		}
	case *types.Pointer:
		if ptrClass(u) != nil {
			return kDirect
		}
		switch {
		case genericOf(u.Elem(), "TList"):
			return kList
		case genericOf(u.Elem(), "ImmutableList_Builder"):
			return kBuilder
		case genericOf(u.Elem(), "tconv_"):
			return kConv
		}
	case *types.Slice:
		if ptrClass(u.Elem()) != nil {
			return kSlice
		}
	case *types.Signature:
		return kFunc
	}
	return kOther
}

// valueSize is the size of class n as a value, the listed classes among its fields as values;
// -1 if it contains itself.
func valueSize(n *types.Named, busy map[*types.Named]bool) int64 {
	if busy == nil {
		busy = map[*types.Named]bool{}
	}
	if busy[n] {
		return -1
	}
	busy[n] = true
	defer delete(busy, n)
	st := n.Underlying().(*types.Struct)
	sizes := types.SizesFor("gc", "amd64")
	var off, maxAlign int64 = 0, 1
	for i := 0; i < st.NumFields(); i++ {
		ft := st.Field(i).Type()
		size, align := sizes.Sizeof(ft), sizes.Alignof(ft)
		if c := ptrClass(ft); c != nil && listed[n.Obj().Name()+"."+st.Field(i).Name()] != "nullable" {
			size = valueSize(c.named, busy)
			if size < 0 {
				return -1
			}
			align = 8
		} else if c := byNamed[namedOf(ft)]; c != nil {
			size = valueSize(c.named, busy)
			if size < 0 {
				return -1
			}
		}
		off = (off + align - 1) / align * align
		off += size
		if align > maxAlign {
			maxAlign = align
		}
	}
	return (off + maxAlign - 1) / maxAlign * maxAlign
}

// ---- shape and immutability

func isLifecycleName(c *class, name string) bool {
	return name == "zero_"+c.name || name == "finit_"+c.name || strings.HasPrefix(name, "init_"+c.name)
}

func enclosingDecl(n ast.Node) *ast.FuncDecl {
	for p := n; p != nil; p = rw.Parent[p] {
		if fd, ok := p.(*ast.FuncDecl); ok {
			return fd
		}
	}
	return nil
}

func recvVar(fd *ast.FuncDecl) types.Object {
	if fd.Recv == nil || len(fd.Recv.List) != 1 || len(fd.Recv.List[0].Names) != 1 {
		return nil
	}
	return rw.Info.Defs[fd.Recv.List[0].Names[0]]
}

func isUse(e ast.Expr, o types.Object) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	return ok && o != nil && rw.Info.Uses[id] == o
}

func shape(c *class) {
	var decl ast.Node
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if ts, ok := n.(*ast.TypeSpec); ok && rw.Info.Defs[ts.Name] == c.tn {
				c.spec = ts
				decl = ts
			}
			return c.spec == nil
		})
	}
	st, ok := c.named.Underlying().(*types.Struct)
	if !ok || c.named.TypeParams().Len() > 0 || !generated(decl) {
		fail(decl, c, "not a non-generic struct of a g_ file")
		return
	}
	if st.NumFields() == 0 {
		fail(decl, c, "no fields")
	}
	for i := 0; i < st.NumFields(); i++ {
		f := st.Field(i)
		if f.Embedded() {
			fail(decl, c, "it embeds "+f.Name()+" (a superclass)")
		}
		if f.Name() == "self_" || strings.HasPrefix(f.Name(), "vt_") {
			fail(decl, c, "it dispatches through "+f.Name())
		}
		c.fields[f] = true
	}
	// embedded by none; named by no runtime file
	for _, id := range rw.UsesOf[c.tn] {
		if !generated(id) {
			fail(id, c, "a runtime file names it")
		}
		if fl, ok := rw.Parent[id].(*ast.Field); ok && len(fl.Names) == 0 {
			if _, inStruct := rw.Parent[rw.Parent[fl]].(*ast.StructType); inStruct {
				fail(id, c, "a class embeds it (a subclass)")
			}
		}
	}
	for i := 0; i < c.named.NumMethods(); i++ {
		m := c.named.Method(i)
		switch {
		case m.Name() == "equals" || m.Name() == "hashCode":
			fail(declOf[m], c, "it has its own "+m.Name()+" (valcls's case)")
		case m.Name() == "toString":
			c.toString = true
		case isLifecycleName(c, m.Name()):
			c.lifecycle[m] = true
			c.thisVars[recvVar(declOf[m])] = true
		}
		if _, isPtr := m.Type().(*types.Signature).Recv().Type().(*types.Pointer); !isPtr {
			c.done = true
		}
	}
	// new_C*: `this := &C{}` ... `return this`
	for fn, fd := range declOf {
		if fd.Recv != nil || !strings.HasPrefix(fn.Name(), "new_"+c.name) {
			continue
		}
		res := fn.Type().(*types.Signature).Results()
		if res.Len() != 1 {
			continue
		}
		if ptrClass(res.At(0).Type()) != c {
			if namedOf(res.At(0).Type()) != c.named {
				continue // new_C_Sub: another class
			}
			c.done = true
		}
		body := fd.Body.List
		ok := len(body) >= 2
		if ok {
			as, isAs := body[0].(*ast.AssignStmt)
			ok = isAs && as.Tok == token.DEFINE && len(as.Lhs) == 1 && len(as.Rhs) == 1 && newLit(as.Rhs[0], c)
			if ok {
				this := rw.Info.Defs[as.Lhs[0].(*ast.Ident)]
				r, isR := body[len(body)-1].(*ast.ReturnStmt)
				ok = isR && len(r.Results) == 1 && isUse(r.Results[0], this)
				c.thisVars[this] = true
			}
		}
		if !ok {
			fail(fd, c, "a constructor is not `this := &C{}` ... `return this`")
		}
		c.news[fd] = true
	}
	if len(c.news) == 0 {
		fail(decl, c, "no new_ constructor")
	}
	// fields: written only through `this` of the constructor chain; no address taken
	for f := range c.fields {
		for _, id := range rw.UsesOf[f] {
			sel, ok := rw.Parent[id].(*ast.SelectorExpr)
			if !ok || sel.Sel != id {
				fail(id, c, "a field named otherwise than x.f")
				continue
			}
			write := false
			switch p := rw.Up(sel).(type) {
			case *ast.AssignStmt:
				for _, l := range p.Lhs {
					write = write || rw.Unparen(l) == ast.Expr(sel)
				}
			case *ast.IncDecStmt:
				write = true
			case *ast.UnaryExpr:
				if p.Op == token.AND {
					fail(p, c, "a field's address is taken")
				}
			}
			if write {
				id2, _ := rw.Unparen(sel.X).(*ast.Ident)
				if id2 == nil || !c.thisVars[rw.Info.Uses[id2]] {
					fail(sel, c, "field "+f.Name()+" is written outside the constructor chain")
				}
			}
		}
	}
	// the lifecycle methods are called only on `this` of the constructor chain
	for m := range c.lifecycle {
		for _, id := range rw.UsesOf[m] {
			sel, _ := rw.Parent[id].(*ast.SelectorExpr)
			okc := false
			if sel != nil {
				if call, isCall := rw.Parent[sel].(*ast.CallExpr); isCall && call.Fun == ast.Expr(sel) {
					id2, _ := rw.Unparen(sel.X).(*ast.Ident)
					okc = id2 != nil && c.thisVars[rw.Info.Uses[id2]]
				}
			}
			if !okc {
				fail(id, c, "its constructor "+m.Name()+" is run otherwise than on `this` of new_")
			}
		}
	}
	// `this` of the constructor chain: only this.f / this.m(), its definition and new_'s return
	for this := range c.thisVars {
		for _, id := range rw.UsesOf[this] {
			switch p := rw.Up(id).(type) {
			case *ast.SelectorExpr:
				if rw.Unparen(p.X) == ast.Expr(id) {
					continue
				}
			case *ast.ReturnStmt:
				if fd := enclosingDecl(id); fd != nil && c.news[fd] && fd.Body.List[len(fd.Body.List)-1] == ast.Stmt(p) {
					continue
				}
			}
			fail(id, c, "`this` is handed on while the object is constructed ("+short(rw.Up(id))+")")
		}
	}
	if sz := valueSize(c.named, nil); sz < 0 {
		fail(decl, c, "as a value it contains itself")
	} else if sz > sizeLimit {
		fail(decl, c, fmt.Sprintf("as a value it is %d bytes (limit %d)", sz, sizeLimit))
	}
}

// newLit: e is &C{} (or C{} on a rerun).
func newLit(e ast.Expr, c *class) bool {
	e = rw.Unparen(e)
	if u, ok := e.(*ast.UnaryExpr); ok && u.Op == token.AND {
		e = u.X
	}
	cl, ok := e.(*ast.CompositeLit)
	return ok && len(cl.Elts) == 0 && namedOf(rw.Info.TypeOf(cl)) == c.named
}

// ---- type spellings

var genericTypes = map[string]bool{"TList": true, "ImmutableList": true, "ImmutableList_Builder": true, "tconv_": true}

// spellings checks where the type's name is written.
func spellings(c *class) {
	if c.done {
		return
	}
	for _, id := range rw.UsesOf[c.tn] {
		star, isStar := rw.Parent[id].(*ast.StarExpr)
		if !isStar {
			if cl, ok := rw.Parent[id].(*ast.CompositeLit); ok && cl.Type == ast.Expr(id) && len(cl.Elts) == 0 {
				if fd := enclosingDecl(id); fd != nil && c.news[fd] {
					continue
				}
			}
			fail(id, c, "named otherwise than as *C ("+short(rw.Parent[id])+")")
			continue
		}
		var cur ast.Node = star
	up:
		for {
			switch p := rw.Parent[cur].(type) {
			case *ast.ArrayType:
				if p.Len != nil {
					fail(p, c, "an array of the type")
					break up
				}
				cur = p
			case *ast.IndexExpr:
				x, _ := p.X.(*ast.Ident)
				if p.Index != cur || x == nil {
					fail(p, c, "a type spelling the tool does not follow ("+short(p)+")")
					break up
				} else if _, isFn := rw.Info.Uses[x].(*types.Func); isFn {
					break up // an explicit instantiation of a function: instances()
				} else if !genericTypes[x.Name] {
					fail(p, c, "generic type "+x.Name+" instantiated with the class")
					break up
				}
				cur = p
			case *ast.StarExpr:
				if _, isIx := p.X.(*ast.IndexExpr); !isIx {
					fail(p, c, "a pointer to the type")
					break up
				}
				cur = p
			case *ast.Field:
				break up // a parameter, result, receiver or field: variables()
			case *ast.ValueSpec, *ast.CompositeLit:
				break up
			case *ast.CallExpr:
				if fn, isId := p.Fun.(*ast.Ident); isId && fn.Name == "make" && len(p.Args) >= 2 && p.Args[0] == cur {
					if lit, isLit := p.Args[1].(*ast.BasicLit); isLit && lit.Value == "0" {
						break up
					}
				}
				fail(p, c, "make with a length, new or a conversion ("+short(p)+")")
				break up
			default:
				fail(p, c, fmt.Sprintf("a type spelling the tool does not follow (%T: %s)", p, short(p)))
				break up
			}
		}
	}
}

// ---- variables

func fieldOwner(id *ast.Ident) *types.TypeName {
	for p := rw.Parent[id]; p != nil; p = rw.Parent[p] {
		if ts, ok := p.(*ast.TypeSpec); ok {
			tn, _ := rw.Info.Defs[ts.Name].(*types.TypeName)
			return tn
		}
		if _, ok := p.(*ast.FuncDecl); ok {
			return nil
		}
	}
	return nil
}

func variables() {
	holders := map[*types.Named]bool{} // structs with a field of a listed class's type
	var ids []*ast.Ident
	for id := range rw.Info.Defs {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].Pos() < ids[j].Pos() })
	for _, id := range ids {
		v, isVar := rw.Info.Defs[id].(*types.Var)
		if !isVar {
			continue
		}
		for c := range mentioned(v.Type()) {
			k := kindOf(v.Type())
			if c.done {
				continue
			}
			switch {
			case k == kOther:
				fail(id, c, "a variable of a type the tool does not follow ("+typeStr(v.Type())+")")
			case k == kValue:
				fail(id, c, "a variable holding the class by value")
			case k != kDirect:
				counts["variables holding lists, slices or funcs of the type (their zero is no value of the class)"]++
			case v.IsField():
				owner := fieldOwner(id)
				if owner == nil {
					fail(id, c, "a field of an unnamed struct")
					continue
				}
				key := owner.Name() + "." + v.Name()
				listSeen[key] = true
				if on, _ := owner.Type().(*types.Named); on != nil && byNamed[on] == nil {
					holders[on] = true
				}
				switch listed[key] {
				case "field", "field+":
					fieldWrites(c, owner, v, key)
					counts["fields listed as assigned before any read (argued in classes.txt)"]++
				case "nullable":
					nullable[v] = c
					counts["fields listed as nullable: kept pointers"]++
				default:
					fail(id, c, "field "+key+" is not in classes.txt (field: assigned before any read, argued there; or nullable)")
				}
			case v.Parent() == rw.Pkg.Scope():
				fail(id, c, "a package variable of the type")
			default:
				switch p := rw.Parent[id].(type) {
				case *ast.Field:
					fl := rw.Parent[p].(*ast.FieldList)
					if ft, isFT := rw.Parent[fl].(*ast.FuncType); isFT && ft.Results == fl {
						fail(id, c, "a named result (its zero value)")
					} else {
						counts["parameters and receivers (given a value by every call)"]++
					}
				case *ast.ValueSpec:
					if len(p.Values) == 0 {
						fail(id, c, "a local declared without a value")
					} else {
						counts["locals declared with a value"]++
					}
				case *ast.AssignStmt, *ast.RangeStmt:
					counts["locals declared with a value"]++
				default:
					fail(id, c, fmt.Sprintf("a variable declared in a %T", p))
				}
			}
		}
	}
	// a struct with such a field is only used through pointers and made by &T{}
	for _, id := range ids {
		v, isVar := rw.Info.Defs[id].(*types.Var)
		if !isVar || v.Embedded() {
			continue
		}
		t := types.Unalias(v.Type())
		for {
			switch u := t.(type) {
			case *types.Slice:
				t = types.Unalias(u.Elem())
				continue
			case *types.Array:
				t = types.Unalias(u.Elem())
				continue
			case *types.Map:
				t = types.Unalias(u.Elem())
				continue
			}
			break
		}
		if n, ok := t.(*types.Named); ok && holders[n.Origin()] {
			failures = append(failures, failure{rw.Fset.Position(id.Pos()), "classes.txt: struct " + n.Obj().Name() + " with a field of a listed class is used by value"})
		}
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.CompositeLit:
				if nt := namedOf(rw.Info.TypeOf(x)); nt != nil && holders[nt] {
					u, isAddr := rw.Up(x).(*ast.UnaryExpr)
					if len(x.Elts) > 0 || !isAddr || u.Op != token.AND {
						failures = append(failures, failure{rw.Fset.Position(x.Pos()), "classes.txt: struct " + nt.Obj().Name() + " with a field of a listed class is made otherwise than by &T{}"})
					}
				}
			case *ast.CallExpr:
				if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "new" && len(x.Args) == 1 {
					if nt := namedOf(rw.Info.TypeOf(x.Args[0])); nt != nil && (holders[nt] || byNamed[nt] != nil) {
						failures = append(failures, failure{rw.Fset.Position(x.Pos()), "classes.txt: new of a listed class or of a struct with a field of one"})
					}
				}
			}
			return true
		})
	}
}

// fieldWrites: a `field` entry is assigned, and every write of it is `this.f = x` in an
// init_/finit_ method of its class (the hand argument in classes.txt says why that comes
// before any read); a `field+` entry is also written later, which the list argues.
func fieldWrites(c *class, owner *types.TypeName, f *types.Var, key string) {
	n := 0
	for _, id := range rw.UsesOf[f] {
		sel, _ := rw.Parent[id].(*ast.SelectorExpr)
		if sel == nil {
			continue
		}
		as, ok := rw.Up(sel).(*ast.AssignStmt)
		if !ok {
			continue
		}
		lhs := false
		for _, l := range as.Lhs {
			lhs = lhs || rw.Unparen(l) == ast.Expr(sel)
		}
		if !lhs {
			continue
		}
		fd := enclosingDecl(as)
		inCtor := fd != nil && fd.Recv != nil && isUse(sel.X, recvVar(fd)) &&
			(fd.Name.Name == "finit_"+owner.Name() || strings.HasPrefix(fd.Name.Name, "init_"+owner.Name()))
		if inCtor || listed[key] == "field+" {
			n++
		} else {
			fail(as, c, "field "+key+" (listed as `field`) is written outside init_/finit_ of "+owner.Name()+" (list it as field+ and argue the write)")
		}
	}
	if n == 0 {
		failures = append(failures, failure{rw.Fset.Position(f.Pos()), c.name + ": field " + key + " is never assigned (by an init_/finit_ of " + owner.Name() + ", for a `field` entry)"})
	}
}

// ---- generic code

var genericFuncs = map[string]bool{"immutableListOf_": true, "immutableListBuilder_": true, "immAdd_": true, "immBuilt_": true, "sliceAt_": true}

var formatFuncs = map[string]bool{"String_format": true, "Strings_lenientFormat": true}

// inConvDecl: n is inside the declaration of an element conversion (tc_p_C).
func inConvDecl(n ast.Node) (*ast.ValueSpec, *class) {
	for p := n; p != nil; p = rw.Parent[p] {
		if vs, ok := p.(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 1 {
			if v, ok := rw.Info.Defs[vs.Names[0]].(*types.Var); ok && v.Parent() == rw.Pkg.Scope() && kindOf(v.Type()) == kConv {
				for c := range mentioned(v.Type()) {
					return vs, c
				}
			}
		}
	}
	return nil, nil
}

// formatArg: e is an argument after the first of a format call.
func formatArg(e ast.Expr) bool {
	call, ok := rw.Up(e).(*ast.CallExpr)
	if !ok {
		return false
	}
	name, _ := rw.CallName(call)
	if !formatFuncs[name] {
		return false
	}
	for i, a := range call.Args {
		if rw.Unparen(a) == e {
			return i > 0
		}
	}
	return false
}

func instances() {
	var ids []*ast.Ident
	for id := range rw.Info.Instances {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].Pos() < ids[j].Pos() })
	for _, id := range ids {
		inst := rw.Info.Instances[id]
		for i := 0; i < inst.TypeArgs.Len(); i++ {
			ta := inst.TypeArgs.At(i)
			for c := range mentioned(ta) {
				if c.done {
					continue
				}
				if ptrClass(ta) != c {
					fail(id, c, "generic "+id.Name+" instantiated with a type holding the class ("+typeStr(ta)+")")
					continue
				}
				vs, cc := inConvDecl(id)
				switch {
				case !generated(id):
					fail(id, c, "a runtime file instantiates "+id.Name+" with the class")
				case genericTypes[id.Name]:
					counts["instantiations of TList / ImmutableList / ImmutableList_Builder / tconv_"]++
				case genericFuncs[id.Name]:
					counts["calls of immutableListOf_ / immutableListBuilder_ / immAdd_ / immBuilt_ / sliceAt_"]++
				case vs != nil && cc == c && (id.Name == "obj_" || id.Name == "cast_"):
					c.conv[rw.Info.Defs[vs.Names[0]].(*types.Var)] = vs
				case id.Name == "tostr_" || id.Name == "tostrNN_":
					// String.valueOf(x) / x.toString() of a value that is never null: x.toString()
					// (tostr_ would print a value equal to the zero value as "null")
					call, _ := rw.Up(id).(*ast.CallExpr)
					if call == nil || call.Fun != ast.Expr(id) || len(call.Args) != 1 {
						fail(id, c, id.Name+" not called on a value")
					} else {
						c.tostrs = append(c.tostrs, call)
					}
				case id.Name == "obj_":
					call, _ := rw.Up(id).(*ast.CallExpr)
					if call == nil || call.Fun != ast.Expr(id) || !formatArg(call) {
						fail(id, c, "boxed otherwise than for a format call ("+short(rw.Up(id))+")")
					} else if !c.toString {
						fail(id, c, "boxed for a format call, but the class has no toString (its identity would print)")
					} else {
						c.boxes = append(c.boxes, call)
					}
				default:
					fail(id, c, "generic "+id.Name+" instantiated with the class")
				}
			}
		}
	}
}

// ---- expressions

var listMethods = map[int]map[string]bool{
	kList:    {"get": true, "size": true, "isEmpty": true, "asImm": true, "items": true},
	kBuilder: {"add": true, "build": true},
}

func expressions() {
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			if vs, ok := n.(*ast.ValueSpec); ok {
				if cv, c := inConvDecl(vs); cv == vs {
					// the conversion's declaration: &tconv_[*C]{box: obj_[*C], unbox: cast_[*C]}
					want := "&tconv_[*" + c.name + "]{box: obj_[*" + c.name + "], unbox: cast_[*" + c.name + "]}"
					if !c.done && rw.Text(vs.Values[0]) != want {
						fail(vs, c, "its element conversion is not "+want)
					}
					return false
				}
			}
			e, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			if _, isParen := e.(*ast.ParenExpr); isParen {
				return true
			}
			if isNilIdent(e) {
				// nil keeps the type "untyped nil": it is a null of the class where it goes to
				// a slot of the type (a comparison with a value fails at the value)
				if _, isCmp := rw.Up(e).(*ast.BinaryExpr); !isCmp {
					if st, why := slotType(e); why == "" {
						if c := ptrClass(st); c != nil && !c.done {
							fail(e, c, "nil of the type (null) in "+short(rw.Up(e)))
						}
					}
				}
				return true
			}
			tv, ok := rw.Info.Types[e]
			if !ok || tv.IsType() || tv.Type == nil {
				return true
			}
			k := kindOf(tv.Type)
			if k == kNone {
				return true
			}
			for c := range mentioned(tv.Type) {
				if c.done {
					continue
				}
				switch k {
				case kDirect:
					direct(e, c, tv)
				case kSlice, kList, kBuilder:
					container(e, c, k)
				case kConv:
					call, isCall := rw.Up(e).(*ast.CallExpr)
					if _, isId := e.(*ast.Ident); !isId || !isCall || rw.Unparen(call.Fun) == e {
						fail(e, c, "the element conversion is used otherwise than as an argument ("+short(rw.Up(e))+")")
					}
				case kFunc:
				case kValue:
					if !(newLit(e, c) && c.news[enclosingDecl(e)]) {
						fail(e, c, "the class by value ("+short(e)+")")
					}
				default:
					fail(e, c, "an expression of a type the tool does not follow ("+typeStr(tv.Type)+": "+short(e)+")")
				}
			}
			return true
		})
	}
}

func isThisOfChain(e ast.Expr, c *class) bool {
	id, ok := e.(*ast.Ident)
	if !ok {
		return false
	}
	o := rw.Info.Uses[id]
	if o == nil {
		o = rw.Info.Defs[id]
	}
	return c.thisVars[o]
}

// calleeOK: the call's function is one whose results are expressions the tool checks (a
// function or method declared in a g_ file, an interface method of one, a func value, a
// function literal called in place) or one of the runtime's listed generic functions and
// methods. "" or the reason.
func calleeOK(call *ast.CallExpr, k int) string {
	fun := rw.Unparen(call.Fun)
	if tv, ok := rw.Info.Types[fun]; ok && tv.IsType() {
		return "a conversion to the type"
	}
	var id *ast.Ident
	switch f := fun.(type) {
	case *ast.FuncLit:
		return ""
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
		if sel := rw.Info.Selections[f]; sel != nil && sel.Kind() == types.MethodVal {
			if rk := kindOf(sel.Recv()); rk == kList || rk == kBuilder {
				if !listMethods[rk][id.Name] {
					return "method " + id.Name + " of a list of the class"
				}
				return ""
			}
		}
	case *ast.IndexExpr:
		id, _ = f.X.(*ast.Ident)
	}
	if id == nil {
		return "a call the tool does not follow (" + short(call.Fun) + ")"
	}
	switch o := rw.Info.Uses[id].(type) {
	case *types.Func:
		if _, isInst := rw.Info.Instances[id]; isInst {
			if genericFuncs[id.Name] {
				return ""
			}
			return "generic " + id.Name
		}
		if o.Pkg() != rw.Pkg || !strings.HasPrefix(filepath.Base(rw.Fset.Position(o.Pos()).Filename), "g_") {
			return "a function not of the generated code (" + id.Name + ")"
		}
		return ""
	case *types.Var:
		return "" // a func value: every func of the type returns a checked expression
	case *types.Builtin:
		if (id.Name == "append" || id.Name == "make") && k == kSlice {
			return ""
		}
		return "builtin " + id.Name
	}
	return "a call the tool does not follow (" + short(call.Fun) + ")"
}

// slotType is the type of the slot expression e's value goes to (a parameter, an assigned
// variable, a result, a slice element), or the reason there is none.
func slotType(e ast.Expr) (types.Type, string) {
	switch p := rw.Up(e).(type) {
	case *ast.CallExpr:
		if rw.Unparen(p.Fun) == e {
			return nil, "called"
		}
		ftv, ok := rw.Info.Types[p.Fun]
		if !ok || ftv.IsType() {
			return nil, "converted (" + short(p) + ")"
		}
		sig, ok := types.Unalias(ftv.Type).Underlying().(*types.Signature)
		if !ok {
			return nil, "given to " + short(p.Fun)
		}
		for i, a := range p.Args {
			if rw.Unparen(a) != e {
				continue
			}
			n := sig.Params().Len()
			if sig.Variadic() && i >= n-1 {
				st := sig.Params().At(n - 1).Type()
				if p.Ellipsis.IsValid() {
					return st, ""
				}
				return st.(*types.Slice).Elem(), ""
			}
			if i < n {
				return sig.Params().At(i).Type(), ""
			}
		}
		return nil, "given to " + short(p.Fun)
	case *ast.AssignStmt:
		for _, l := range p.Lhs {
			if rw.Unparen(l) == e {
				return rw.Info.TypeOf(e), "" // written to
			}
		}
		if len(p.Lhs) != len(p.Rhs) {
			return nil, "assigned from a tuple"
		}
		for i, r := range p.Rhs {
			if rw.Unparen(r) == e {
				if id, ok := p.Lhs[i].(*ast.Ident); ok {
					if o := rw.Info.Defs[id]; o != nil {
						return o.Type(), ""
					}
					if id.Name == "_" {
						return rw.Info.TypeOf(e), ""
					}
				}
				return rw.Info.TypeOf(p.Lhs[i]), ""
			}
		}
	case *ast.ValueSpec:
		for i, v := range p.Values {
			if rw.Unparen(v) == e && i < len(p.Names) && len(p.Names) == len(p.Values) {
				if o := rw.Info.Defs[p.Names[i]]; o != nil {
					return o.Type(), ""
				}
				return rw.Info.TypeOf(e), "" // _
			}
		}
	case *ast.ReturnStmt:
		var ft *ast.FuncType
		for q := rw.Parent[p]; q != nil && ft == nil; q = rw.Parent[q] {
			switch f := q.(type) {
			case *ast.FuncLit:
				ft = f.Type
			case *ast.FuncDecl:
				ft = f.Type
			}
		}
		if ft != nil && ft.Results != nil && len(p.Results) == ft.Results.NumFields() {
			i := 0
			for _, fl := range ft.Results.List {
				k := len(fl.Names)
				if k == 0 {
					k = 1
				}
				for j := 0; j < k; j++ {
					if rw.Unparen(p.Results[i]) == e {
						return rw.Info.TypeOf(fl.Type), ""
					}
					i++
				}
			}
		}
	case *ast.CompositeLit:
		if s, ok := types.Unalias(rw.Info.TypeOf(p)).Underlying().(*types.Slice); ok {
			return s.Elem(), ""
		}
		return nil, "an element of a composite literal"
	case *ast.ExprStmt:
		return rw.Info.TypeOf(e), "" // a result dropped
	case *ast.RangeStmt:
		if rw.Unparen(p.X) != e {
			return rw.Info.TypeOf(e), "" // the loop variable written to
		}
		return nil, "ranged over"
	default:
		return nil, fmt.Sprintf("used in a %T (%s)", p, short(p))
	}
	return nil, "used in " + short(rw.Up(e))
}

func isNilIdent(e ast.Expr) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok || id.Name != "nil" {
		return false
	}
	_, isNil := rw.Info.Uses[id].(*types.Nil)
	return isNil
}

// direct checks an expression of type *C: how the value is made and where it goes.
func direct(e ast.Expr, c *class, tv types.TypeAndValue) {
	if isThisOfChain(e, c) {
		return // shape()
	}
	var nullableField *types.Var
	// how it is made
	switch x := e.(type) {
	case *ast.Ident:
		if isNilIdent(x) || tv.IsNil() {
			fail(e, c, "nil of the type (null) in "+short(rw.Up(e)))
			return
		}
		o := rw.Info.Uses[x]
		if o == nil {
			o = rw.Info.Defs[x]
		}
		if _, isVar := o.(*types.Var); !isVar {
			fail(e, c, "a value from "+x.Name)
		}
		counts["reads and writes of variables of the type"]++
	case *ast.SelectorExpr:
		sel := rw.Info.Selections[x]
		if sel == nil || sel.Kind() != types.FieldVal {
			fail(e, c, "a value from "+short(e))
			return
		}
		if f := sel.Obj().(*types.Var); nullable[f] != nil {
			nullableField = f
		}
		counts["reads and writes of fields of the type"]++
	case *ast.CallExpr:
		if why := calleeOK(x, kDirect); why != "" {
			fail(e, c, "a value from "+why)
		}
		counts["calls returning the type (g_ functions, methods and func values; list.get, sliceAt_)"]++
	case *ast.UnaryExpr:
		if x.Op != token.AND || !newLit(x, c) || !c.news[enclosingDecl(x)] {
			fail(e, c, "a value from "+short(e))
		}
		return
	case *ast.IndexExpr:
		if kindOf(rw.Info.TypeOf(x.X)) != kSlice {
			fail(e, c, "an element of "+typeStr(rw.Info.TypeOf(x.X)))
		}
		counts["elements of slices of the type"]++
	default:
		fail(e, c, fmt.Sprintf("a value from a %T (%s)", e, short(e)))
	}
	// where it goes
	p := rw.Up(e)
	if nullableField != nil {
		switch u := p.(type) {
		case *ast.SelectorExpr:
			if rw.Unparen(u.X) == e {
				counts["nullable field dereferenced on the spot (a null throws as before)"]++
				return
			}
		case *ast.BinaryExpr:
			if (u.Op == token.EQL || u.Op == token.NEQ) && (isNilIdent(u.X) || isNilIdent(u.Y)) {
				counts["nullable field compared with nil"]++
				return
			}
		case *ast.AssignStmt:
			if len(u.Lhs) == 1 && len(u.Rhs) == 1 && rw.Unparen(u.Lhs[0]) == e && u.Tok == token.ASSIGN && !isNilIdent(u.Rhs[0]) && ptrClass(rw.Info.TypeOf(u.Rhs[0])) == c {
				nullWrite = append(nullWrite, u)
				counts["nullable field assigned a value (a copy on the heap)"]++
				return
			}
		}
		fail(e, c, "a nullable field used otherwise than assigned a value, compared with nil or dereferenced ("+short(p)+")")
		return
	}
	switch u := p.(type) {
	case *ast.SelectorExpr:
		if rw.Unparen(u.X) != e {
			break
		}
		sel := rw.Info.Selections[u]
		if sel == nil {
			fail(e, c, "selected from otherwise than a field or method ("+short(u)+")")
			return
		}
		if sel.Kind() == types.FieldVal {
			counts["field reads of a value"]++
			return
		}
		if call, ok := rw.Up(u).(*ast.CallExpr); !ok || rw.Unparen(call.Fun) != ast.Expr(u) {
			fail(e, c, "a method value ("+short(u)+")")
			return
		}
		counts["method calls on a value"]++
		return
	case *ast.BinaryExpr:
		fail(e, c, "compared: "+short(u))
		return
	case *ast.UnaryExpr:
		fail(e, c, "operand of "+u.Op.String()+": "+short(u))
		return
	case *ast.StarExpr, *ast.TypeAssertExpr, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.CaseClause, *ast.KeyValueExpr, *ast.IndexExpr, *ast.SliceExpr:
		fail(e, c, fmt.Sprintf("used in a %T: %s", p, short(p)))
		return
	}
	if call, ok := p.(*ast.CallExpr); ok {
		if name, _ := rw.CallName(call); name == "tostr_" || name == "tostrNN_" {
			counts["tostr_ / tostrNN_ of a value (its toString)"]++
			return // instances()
		}
	}
	st, why := slotType(e)
	if why != "" {
		fail(e, c, why)
		return
	}
	// (obj_(x) has a parameter of the type itself: instances() checks where its box goes)
	if ptrClass(st) != c {
		fail(e, c, "stored, passed or returned where the type is not *"+c.name+" but "+typeStr(st)+" ("+short(p)+")")
	}
}

func typeStr(t types.Type) string {
	if t == nil {
		return "unknown"
	}
	return types.TypeString(t, types.RelativeTo(rw.Pkg))
}

// container checks an expression holding values of the class: a slice, a list or a list builder.
func container(e ast.Expr, c *class, k int) {
	switch x := e.(type) {
	case *ast.Ident:
		// a variable, or nil (a null list is no value of the class)
	case *ast.SelectorExpr:
		sel := rw.Info.Selections[x]
		if sel == nil || sel.Kind() != types.FieldVal {
			fail(e, c, "a list from "+short(e))
			return
		}
		if rk := kindOf(sel.Recv()); rk == kList || rk == kBuilder {
			if !listMethods[rk][x.Sel.Name] {
				fail(e, c, "field "+x.Sel.Name+" of a list of the class")
			}
		}
	case *ast.CallExpr:
		if why := calleeOK(x, k); why != "" {
			fail(e, c, "a list from "+why)
		}
		if id, ok := x.Fun.(*ast.Ident); ok && id.Name == "make" {
			if _, isB := rw.Info.Uses[id].(*types.Builtin); isB {
				if lit, isLit := x.Args[len(x.Args)-1].(*ast.BasicLit); len(x.Args) < 2 || len(x.Args) == 2 && (!isLit || lit.Value != "0") {
					fail(e, c, "make with a length (zero values): "+short(e))
				} else if lit, isLit := x.Args[1].(*ast.BasicLit); !isLit || lit.Value != "0" {
					fail(e, c, "make with a length (zero values): "+short(e))
				}
			}
		}
	case *ast.CompositeLit:
		if k != kSlice {
			fail(e, c, "a list from "+short(e))
		}
		for _, el := range x.Elts {
			if _, isKV := el.(*ast.KeyValueExpr); isKV {
				fail(el, c, "a keyed slice literal (zero values between)")
			}
		}
	default:
		fail(e, c, fmt.Sprintf("a list from a %T (%s)", e, short(e)))
		return
	}
	counts["expressions holding lists or slices of the type"]++
	p := rw.Up(e)
	switch u := p.(type) {
	case *ast.SelectorExpr:
		if rw.Unparen(u.X) != e {
			break
		}
		if k == kSlice || !listMethods[k][u.Sel.Name] {
			fail(e, c, "selector "+u.Sel.Name+" on a list of the class")
			return
		}
		if u.Sel.Name == "asImm" {
			// obj_(l.asImm()) given to a format call: the erased twin is only printed
			call, _ := rw.Up(u).(*ast.CallExpr)
			var box *ast.CallExpr
			if call != nil {
				box, _ = rw.Up(call).(*ast.CallExpr)
			}
			name := ""
			if box != nil {
				name, _ = rw.CallName(box)
			}
			if name != "obj_" || !formatArg(box) {
				fail(e, c, "the erased view of a list of the class goes elsewhere than to a format call ("+short(rw.Up(u))+")")
			} else if !c.toString {
				fail(e, c, "a list of the class is printed, but the class has no toString")
			}
		}
		return
	case *ast.IndexExpr:
		if k == kSlice && rw.Unparen(u.X) == e {
			return
		}
	case *ast.RangeStmt:
		if k == kSlice {
			return
		}
	case *ast.BinaryExpr:
		if (u.Op == token.EQL || u.Op == token.NEQ) && (isNilIdent(u.X) || isNilIdent(u.Y)) {
			return // a null list
		}
		fail(e, c, "lists of the class compared: "+short(u))
		return
	case *ast.CallExpr:
		if id, ok := rw.Unparen(u.Fun).(*ast.Ident); ok && rw.Unparen(u.Fun) != e {
			if _, isB := rw.Info.Uses[id].(*types.Builtin); isB {
				if k == kSlice && (id.Name == "len" || id.Name == "cap" || id.Name == "append") {
					return
				}
				fail(e, c, "builtin "+id.Name+" on a list of the class")
				return
			}
		}
	case *ast.SliceExpr, *ast.UnaryExpr, *ast.StarExpr, *ast.TypeAssertExpr, *ast.KeyValueExpr:
		fail(e, c, fmt.Sprintf("a list of the class used in a %T: %s", p, short(p)))
		return
	}
	st, why := slotType(e)
	if why != "" {
		fail(e, c, "a list of the class "+why)
		return
	}
	if !types.Identical(types.Unalias(st), types.Unalias(rw.Info.TypeOf(e))) {
		fail(e, c, "a list of the class stored, passed or returned as "+typeStr(st)+" ("+short(p)+")")
	}
}

// recovers: a function returning the type must not recover a panic (it would return the zero
// value).
func recovers() {
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			var ft *ast.FuncType
			var body *ast.BlockStmt
			switch x := n.(type) {
			case *ast.FuncDecl:
				ft, body = x.Type, x.Body
			case *ast.FuncLit:
				ft, body = x.Type, x.Body
			default:
				return true
			}
			if ft.Results == nil || body == nil {
				return true
			}
			var cs []*class
			for _, r := range ft.Results.List {
				for c := range mentioned(rw.Info.TypeOf(r.Type)) {
					if !c.done {
						cs = append(cs, c)
					}
				}
			}
			if len(cs) == 0 {
				return true
			}
			ast.Inspect(body, func(m ast.Node) bool {
				switch y := m.(type) {
				case *ast.FuncLit:
					return false
				case *ast.DeferStmt:
					ast.Inspect(y, func(k ast.Node) bool {
						if call, ok := k.(*ast.CallExpr); ok {
							if id, ok := call.Fun.(*ast.Ident); ok && id.Name == "recover" {
								fail(y, cs[0], "a function returning the type recovers a panic (it would return the zero value)")
							}
						}
						return true
					})
					return false
				}
				return true
			})
			return true
		})
	}
}

// ---- rewrite

func rewrite(c *class) {
	name := c.name
	keep := map[*ast.StarExpr]bool{}
	for m := range c.lifecycle {
		if s, ok := declOf[m].Recv.List[0].Type.(*ast.StarExpr); ok {
			keep[s] = true
		}
	}
	for _, id := range rw.UsesOf[c.tn] {
		switch p := rw.Parent[id].(type) {
		case *ast.StarExpr:
			if keep[p] {
				continue
			}
			if fl, ok := rw.Parent[p].(*ast.Field); ok && len(fl.Names) == 1 {
				if v, ok := rw.Info.Defs[fl.Names[0]].(*types.Var); ok && nullable[v] != nil {
					continue
				}
			}
			if _, c2 := inConvDecl(p); c2 == c {
				continue
			}
			c.nstars++
			rw.Rewrite[p] = func() string { return name }
		case *ast.CompositeLit:
			if u, ok := rw.Parent[p].(*ast.UnaryExpr); ok && u.Op == token.AND {
				rw.Rewrite[u] = func() string { return name + "{}" }
			}
		}
	}
	for _, call := range c.boxes {
		rw.Rewrite[call.Fun] = func() string { return "any" }
	}
	for _, call := range c.tostrs {
		rw.Rewrite[call] = func() string {
			x := rw.Render(call.Args[0])
			switch rw.Unparen(call.Args[0]).(type) {
			case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr:
			default:
				x = "(" + x + ")"
			}
			return x + ".toString()"
		}
	}
	for _, vs := range c.conv {
		rw.Rewrite[vs.Values[0]] = func() string {
			return "&tconv_[" + name + "]{box: func(v " + name + ") any { return v }, unbox: func(v any) " + name + " { return v.(" + name + ") }}"
		}
	}
	ptrs := false
	for _, as := range nullWrite {
		if ptrClass(rw.Info.TypeOf(as.Lhs[0])) != c {
			continue
		}
		ptrs = true
		rhs := as.Rhs[0]
		rw.Rewrite[rhs] = func() string { return "ptr_" + name + "(" + rw.RenderChildren(rhs) + ")" }
	}
	if ptrs {
		gd := rw.Parent[c.spec]
		rw.Rewrite[gd] = func() string {
			return rw.RenderChildren(gd) + "\n\n// ptr_" + name + " is a copy of v on the heap, for a field that may be null.\nfunc ptr_" + name + "(v " + name + ") *" + name + " {\n\treturn &v\n}"
		}
	}
}
