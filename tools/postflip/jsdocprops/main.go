// Command jsdocprops is a post-flip step: JSDocInfo's property constants get their value
// type back, and the reads of a property's value are typed.
//
// Java declares the properties of a JSDocInfo as constants of a generic class,
//
//	private static class Property<T> { T get(JSDocInfo info) { return (T) info.getPropertyValueByIndex(...); } }
//	private static final Property<String> DESCRIPTION = new Property<>("description");
//	private static final TypeProperty TYPE = new TypeProperty("type");   // TypeProperty extends Property<JSTypeExpression>
//
// and keeps the values of a JSDocInfo in one Object array, packed by the properties' bits (a
// single value alone, null for none). Property<T>.get casts the stored Object to T once, and
// Builder.getProp / getPropWithDefault likewise. j2go erases T: the constants are
// *JSDocInfo_Property, get returns any, and every reader casts the value back:
// str_(JSDocInfo_DESCRIPTION.get(this)), cast_[*JSTypeExpression](JSDocInfo_TYPE.get(this)),
// cast_[*LinkedHashMap](this.getProp(JSDocInfo_PARAMETERS.up_JSDocInfo_Property())) — javac's
// checkcasts at the callers of a generic method.
//
// Here each constant carries its T as Java declares it: a constant of a subclass that binds T
// (TypeProperty, TypeListProperty, TypeMapProperty, NodeMapProperty) keeps its class, which gets
// valueT_ / getT_ returning that T; a constant declared Property<X> becomes a
// JSDocInfo_PropertyOf_[X] (the *JSDocInfo_Property, and X). The reads are typed: K(P.get(info))
// with K the cast to P's T is P.getT_(info), K(this.getProp(P)) is
// JSDocInfo_Builder_getPropT_(this, P), K(this.getPropWithDefault(P, s)) its ...WithDefaultT_;
// a cast to an interface P's T implements is iup_I of the typed read. The one conversion of
// the stored value is in valueT_, where Java has its (T): junboxT_[T] (null is T's null, a value
// of another type the erased cast's ClassCastException). The storage stays Java's: the packed
// Object array, the bitset, the TreeMap of the builder, so the order of the values and every
// read of the bag (asMap, toBuilder, equality) are unchanged. (The hand port jsgo keeps the same
// erased bag and casts at each read with jsdocGet[T]; a field per property would make every
// JSDocInfo 36 slots wide where it is three words.)
//
// T's Go type is the type the reads cast to: one class, basic or enum type for every read of
// the constant (an interface cast where the class is P's T is an upcast). Constants whose reads
// disagree, or that no read casts, are left as they are, counted.
//
// The generated helpers go to g_0jsdocprops.go. A rerun finds no erased constant left.
//
// usage: jsdocprops [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

var qual = func(p *types.Package) string {
	if p == rw.Pkg {
		return ""
	}
	return p.Name()
}

const propName = "JSDocInfo_Property"

type konst struct {
	v      *types.Var
	spec   *ast.ValueSpec
	sub    *types.TypeName // the subclass the constant is declared with, or nil (Property<X>)
	t      types.Type      // T
	reads  []*read
	why    string
	assign []*ast.AssignStmt
}

type read struct {
	k       *ast.CallExpr // the cast
	inner   *ast.CallExpr // P.get(info) / this.getProp(P) / this.getPropWithDefault(P, s)
	kind    string        // "get", "getProp", "getPropWithDefault"
	target  types.Type
	recv    ast.Expr // P (or this, in a subclass method)
	builder ast.Expr
}

var (
	propTN   *types.TypeName
	subT     = map[*types.TypeName]types.Type{} // subclass -> T
	konsts   = map[*types.Var]*konst{}
	verbose  bool
	counts   = map[string]int{}
	leftWhy  = map[string][]string{}
	genFile  = "g_0jsdocprops.go"
	helperTs = map[string]bool{}
)

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	dir := os.Args[len(os.Args)-1]
	rw.Load(dir)
	propTN, _ = rw.Pkg.Scope().Lookup(propName).(*types.TypeName)
	if propTN == nil {
		fmt.Fprintln(os.Stderr, "jsdocprops: no JSDocInfo_Property")
		return
	}
	// the subclasses that bind T: from their Java `extends Property<X>`, X's Go type from the reads
	for _, n := range rw.Pkg.Scope().Names() {
		tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok || st.NumFields() == 0 || !st.Field(0).Embedded() {
			continue
		}
		if nt, ok := st.Field(0).Type().(*types.Named); ok && nt.Obj() == propTN && !rw.HasMethod(types.NewPointer(tn.Type()), "getT_") {
			subT[tn] = nil // (a class with getT_ has its T already: a rerun)
		}
	}
	// the constants
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok {
				continue
			}
			for _, sp := range gd.Specs {
				vs, ok := sp.(*ast.ValueSpec)
				if !ok || vs.Type == nil || len(vs.Names) != 1 || len(vs.Values) != 0 {
					continue
				}
				v := rw.Info.Defs[vs.Names[0]].(*types.Var)
				pt, ok := v.Type().(*types.Pointer)
				if !ok {
					continue
				}
				nt, ok := pt.Elem().(*types.Named)
				if !ok {
					continue
				}
				if nt.Obj() == propTN {
					konsts[v] = &konst{v: v, spec: vs}
				} else if _, ok := subT[nt.Obj()]; ok {
					konsts[v] = &konst{v: v, spec: vs, sub: nt.Obj()}
				}
			}
		}
	}
	if len(konsts) == 0 {
		fmt.Fprintln(os.Stderr, "jsdocprops: no erased property constant (done already)")
		return
	}
	// the reads and the assignments
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				if len(x.Lhs) == 1 {
					if id, ok := x.Lhs[0].(*ast.Ident); ok {
						if k := konsts[varOf(id)]; k != nil {
							k.assign = append(k.assign, x)
						}
					}
				}
			case *ast.CallExpr:
				r := readOf(x)
				if r == nil {
					return true
				}
				if k := konstOf(r.recv); k != nil {
					k.reads = append(k.reads, r)
				} else if r.kind == "get" {
					// this.get(info) in a subclass's own method
					if tn := ptrNamed(rw.Info.TypeOf(r.recv)); tn != nil {
						if _, ok := subT[tn]; ok {
							subReads[tn] = append(subReads[tn], r)
						}
					}
				}
			}
			return true
		})
	}
	// T per subclass: the reads of its constants and of this in its methods
	for tn := range subT {
		var ts []types.Type
		for _, k := range konsts {
			if k.sub == tn {
				for _, r := range k.reads {
					ts = append(ts, r.target)
				}
			}
		}
		for _, r := range subReads[tn] {
			ts = append(ts, r.target)
		}
		subT[tn] = pickT(ts)
	}
	for _, k := range sortedKonsts() {
		if k.sub != nil {
			k.t = subT[k.sub]
			if k.t == nil {
				k.why = "a subclass whose reads disagree or that no read casts"
			}
			continue
		}
		var ts []types.Type
		for _, r := range k.reads {
			ts = append(ts, r.target)
		}
		k.t = pickT(ts)
		if k.t == nil {
			if len(ts) == 0 {
				k.why = "no read casts its value"
			} else {
				k.why = "its reads cast to different types"
			}
		}
		if k.t != nil && len(k.assign) != 1 {
			k.t, k.why = nil, "not assigned once"
		}
	}
	register()
	printCounts()
	if report || len(counts) == 0 {
		return // nothing typed (a rerun: the constants left have no typed read)
	}
	writeHelpers(dir)
	fmt.Fprintf(os.Stderr, "jsdocprops: files changed: %d\n", rw.SaveChecked())
}

var subReads = map[*types.TypeName][]*read{}

func varOf(id *ast.Ident) *types.Var {
	v, _ := rw.Info.Uses[id].(*types.Var)
	return v
}

func ptrNamed(t types.Type) *types.TypeName {
	if t == nil {
		return nil
	}
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		if n, ok := types.Unalias(p.Elem()).(*types.Named); ok {
			return n.Obj()
		}
	}
	return nil
}

func sortedKonsts() []*konst {
	var out []*konst
	for _, k := range konsts {
		out = append(out, k)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].v.Pos() < out[j].v.Pos() })
	return out
}

// konstOf: the constant e names (P, or P.up_JSDocInfo_Property()).
func konstOf(e ast.Expr) *konst {
	e = rw.Unparen(e)
	if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 0 {
		if sel, ok := c.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "up_"+propName {
			e = sel.X
		}
	}
	if id, ok := e.(*ast.Ident); ok {
		return konsts[varOf(id)]
	}
	return nil
}

// castTarget: the type a cast call converts to (cast_[T], to_X, icast_I, str_), or nil.
func castTarget(c *ast.CallExpr) types.Type {
	if len(c.Args) != 1 {
		return nil
	}
	name, _ := rw.CallName(c)
	if !(strings.HasPrefix(name, "cast_[") || strings.HasPrefix(name, "to_") || strings.HasPrefix(name, "icast_") || name == "str_") {
		return nil
	}
	sig, ok := rw.Info.TypeOf(c.Fun).(*types.Signature)
	if !ok || sig.Results().Len() != 1 || sig.Params().Len() != 1 {
		return nil
	}
	if it, ok := sig.Params().At(0).Type().Underlying().(*types.Interface); !ok || !it.Empty() {
		return nil
	}
	return sig.Results().At(0).Type()
}

// readOf: K(P.get(info)), K(b.getProp(P)), K(b.getPropWithDefault(P, s)).
func readOf(k *ast.CallExpr) *read {
	t := castTarget(k)
	if t == nil {
		return nil
	}
	inner, ok := rw.Unparen(k.Args[0]).(*ast.CallExpr)
	if !ok {
		return nil
	}
	sel, ok := inner.Fun.(*ast.SelectorExpr)
	if !ok {
		return nil
	}
	s := rw.Info.Selections[sel]
	if s == nil || s.Kind() != types.MethodVal {
		return nil
	}
	recvTN := ptrNamed(s.Obj().Type().(*types.Signature).Recv().Type())
	switch {
	case sel.Sel.Name == "get" && recvTN == propTN && len(inner.Args) == 1:
		return &read{k: k, inner: inner, kind: "get", target: t, recv: sel.X}
	case sel.Sel.Name == "getProp" && recvTN != nil && recvTN.Name() == "JSDocInfo_Builder" && len(inner.Args) == 1:
		return &read{k: k, inner: inner, kind: "getProp", target: t, recv: inner.Args[0], builder: sel.X}
	case sel.Sel.Name == "getPropWithDefault" && recvTN != nil && recvTN.Name() == "JSDocInfo_Builder" && len(inner.Args) == 2:
		return &read{k: k, inner: inner, kind: "getPropWithDefault", target: t, recv: inner.Args[0], builder: sel.X}
	}
	return nil
}

// pickT: the one non-interface type the reads cast to (the interfaces it implements are
// upcasts), else the one interface.
func pickT(ts []types.Type) types.Type {
	var cls, ifs []types.Type
	add := func(l *[]types.Type, t types.Type) {
		for _, x := range *l {
			if types.Identical(x, t) {
				return
			}
		}
		*l = append(*l, t)
	}
	for _, t := range ts {
		if types.IsInterface(t) {
			add(&ifs, t)
		} else {
			add(&cls, t)
		}
	}
	switch {
	case len(cls) == 1:
		for _, i := range ifs {
			if !types.Implements(cls[0], i.Underlying().(*types.Interface)) {
				return nil
			}
		}
		return cls[0]
	case len(cls) == 0 && len(ifs) == 1:
		return ifs[0]
	}
	return nil
}

func tstr(t types.Type) string { return types.TypeString(t, qual) }

func count(what string) { counts[what]++ }

func left(why string, n ast.Node) {
	leftWhy[why] = append(leftWhy[why], rw.Fset.Position(n.Pos()).String())
}

func register() {
	done := map[*ast.CallExpr]bool{}
	reads := func(k *konst) []*read { return k.reads }
	for _, k := range sortedKonsts() {
		k := k
		if k.t == nil {
			for _, r := range reads(k) {
				left("the constant: "+k.why, r.k)
			}
			if len(k.reads) == 0 {
				left("the constant: "+k.why, k.spec)
			}
			continue
		}
		if k.sub == nil {
			// var P JSDocInfo_PropertyOf_[T]; P = JSDocInfo_PropertyOf_[T]{rhs}
			tt := "JSDocInfo_PropertyOf_[" + tstr(k.t) + "]"
			helperTs["of"] = true
			typ := k.spec.Type
			rw.Rewrite[typ] = func() string { return tt }
			as := k.assign[0]
			rhs := as.Rhs[0]
			rw.Rewrite[rhs] = func() string {
				r := rw.RenderChildren(rhs)
				if tn := ptrNamed(rw.Info.TypeOf(rhs)); tn != propTN {
					r += ".up_" + propName + "()"
				}
				return tt + "{" + r + "}"
			}
			count("constants declared Property<X> are JSDocInfo_PropertyOf_[X]")
			// the other uses: the Property itself
			for _, id := range rw.UsesOf[k.v] {
				id := id
				p := rw.Parent[id]
				if sel, ok := p.(*ast.SelectorExpr); ok && sel.X == id {
					if sel.Sel.Name == "up_"+propName {
						if c, ok := rw.Parent[sel].(*ast.CallExpr); ok {
							rw.Rewrite[c] = func() string { return id.Name + "." + propName }
						}
					}
					continue // a field or method of the Property, promoted
				}
				if as, ok := p.(*ast.AssignStmt); ok && as.Lhs[0] == id {
					continue
				}
				rw.Rewrite[id] = func() string { return id.Name + "." + propName }
			}
		} else {
			count("constants of " + k.sub.Name())
		}
		for _, r := range k.reads {
			registerRead(r, k.t, k.sub, done)
		}
	}
	for tn, rs := range subReads {
		if subT[tn] == nil {
			for _, r := range rs {
				left("a subclass whose reads disagree", r.k)
			}
			continue
		}
		for _, r := range rs {
			registerRead(r, subT[tn], tn, done)
		}
	}
}

func registerRead(r *read, t types.Type, sub *types.TypeName, done map[*ast.CallExpr]bool) {
	if done[r.k] {
		return
	}
	done[r.k] = true
	up := ""
	if !types.Identical(r.target, t) {
		if !types.IsInterface(r.target) || !rw.HasFunc("iup_"+named(r.target)) {
			left("a read cast to another type than the constant's", r.k)
			return
		}
		up = "iup_" + named(r.target)
	}
	var text func() string
	switch r.kind {
	case "get":
		recv := r.recv
		if k := konstOf(recv); k != nil {
			recv = rw.Unparen(recv)
			if c, ok := recv.(*ast.CallExpr); ok {
				recv = c.Fun.(*ast.SelectorExpr).X
			}
		}
		text = func() string { return rw.Render(recv) + ".getT_(" + rw.Render(r.inner.Args[0]) + ")" }
		count("P.get(info) cast to its T is P.getT_(info)")
	case "getProp", "getPropWithDefault":
		k := konstOf(r.recv)
		name := "JSDocInfo_Builder_getPropT_"
		if r.kind == "getPropWithDefault" {
			name = "JSDocInfo_Builder_getPropWithDefaultT_"
			helperTs["withDefault"] = true
		}
		helperTs["getProp"] = true
		text = func() string {
			s := name + "(" + rw.Render(r.builder) + ", " + k.v.Name()
			if r.kind == "getPropWithDefault" {
				s += ", " + rw.Render(r.inner.Args[1])
			}
			return s + ")"
		}
		count("Builder." + r.kind + "(P) cast to its T is the typed " + name)
	}
	if sub != nil {
		helperTs["sub:"+sub.Name()+":"+tstr(t)] = true
	}
	helperTs["iface"] = true
	if up != "" {
		count("a read cast to an interface the constant's T implements is iup_I of the typed read")
		inner := text
		text = func() string { return up + "(" + inner() + ")" }
	}
	rw.Rewrite[r.k] = text
}

func named(t types.Type) string {
	if n, ok := types.Unalias(t).(*types.Named); ok {
		return n.Obj().Name()
	}
	return tstr(t)
}

func printCounts() {
	var ks []string
	for k := range counts {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", counts[k], k)
	}
	for _, k := range sortedKonsts() {
		if k.t != nil {
			fmt.Fprintf(os.Stderr, "        %s: %s\n", k.v.Name(), tstr(k.t))
		}
	}
	ks = nil
	for k := range leftWhy {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", len(leftWhy[k]), k)
		if verbose {
			for _, p := range leftWhy[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
}

func writeHelpers(dir string) {
	var b strings.Builder
	b.WriteString(`// Code generated by tools/postflip/jsdocprops. DO NOT EDIT.

package closure

// JSDocInfo.Property<T> with its T (post-flip jsdocprops): a constant knows the type of its
// value, and the one conversion of the stored Object is valueT_, Java's (T) in Property.get.

// JSDocInfo_PropertyT_ is a property constant of value type T: the Property, and Java's (T).
type JSDocInfo_PropertyT_[T any] interface {
	up_JSDocInfo_Property() *JSDocInfo_Property
	valueT_(v any) T
}
`)
	if helperTs["of"] {
		b.WriteString(`
// JSDocInfo_PropertyOf_ is a constant Java declares Property<T>.
type JSDocInfo_PropertyOf_[T any] struct {
	*JSDocInfo_Property
}

func (p JSDocInfo_PropertyOf_[T]) up_JSDocInfo_Property() *JSDocInfo_Property {
	return p.JSDocInfo_Property
}

// valueT_ is Java's (T) of a stored value: the value itself, null as T's null, another type
// the erased cast's ClassCastException (junboxT_).
func (p JSDocInfo_PropertyOf_[T]) valueT_(v any) T {
	if t, ok := v.(T); ok {
		return t
	}
	return junboxT_[T](v)
}

// getT_ is Property.get with its result typed.
func (p JSDocInfo_PropertyOf_[T]) getT_(info *JSDocInfo) T {
	return p.valueT_(p.get(info))
}
`)
	}
	var subs []string
	for k := range helperTs {
		if strings.HasPrefix(k, "sub:") {
			subs = append(subs, k)
		}
	}
	sort.Strings(subs)
	for _, s := range subs {
		f := strings.SplitN(s, ":", 3)
		fmt.Fprintf(&b, `
// valueT_ is Java's (T) of a stored value of a %[1]s (Property<%[2]s>): the value itself,
// null as T's null, another type the erased cast's ClassCastException (junboxT_).
func (this *%[1]s) valueT_(v any) %[2]s {
	if t, ok := v.(%[2]s); ok {
		return t
	}
	return junboxT_[%[2]s](v)
}

// getT_ is Property.get with its result typed.
func (this *%[1]s) getT_(info *JSDocInfo) %[2]s {
	return this.valueT_(this.get(info))
}
`, f[1], f[2])
	}
	if helperTs["getProp"] {
		b.WriteString(`
// JSDocInfo_Builder_getPropT_ is Builder.getProp with its result typed.
func JSDocInfo_Builder_getPropT_[T any, P JSDocInfo_PropertyT_[T]](this *JSDocInfo_Builder, prop P) T {
	return prop.valueT_(this.getProp(prop.up_JSDocInfo_Property()))
}
`)
	}
	if helperTs["withDefault"] {
		b.WriteString(`
// JSDocInfo_Builder_getPropWithDefaultT_ is Builder.getPropWithDefault with its result typed.
func JSDocInfo_Builder_getPropWithDefaultT_[T any, P JSDocInfo_PropertyT_[T]](this *JSDocInfo_Builder, prop P, supplier func() any) T {
	return prop.valueT_(this.getPropWithDefault(prop.up_JSDocInfo_Property(), supplier))
}
`)
	}
	path := filepath.Join(dir, genFile)
	src := []byte(b.String())
	if err := os.WriteFile(path, src, 0o644); err != nil {
		panic(err)
	}
	f, err := parser.ParseFile(rw.Fset, path, src, parser.SkipObjectResolution)
	if err != nil {
		panic(err)
	}
	rw.Src[rw.Fset.File(f.Pos())] = src
	rw.Files = append(rw.Files, f)
}
