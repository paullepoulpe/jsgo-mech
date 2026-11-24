package main

// generics -classes: the generic classes of the translated Java code keep their type parameters.
//
// j2go erases a Java generic class C<K, V> to a plain struct C whose type-variable fields,
// parameters and results are `any`, so every value stored into one is boxed (strobj_, obj_,
// self(), box_()) and every value read from one is unboxed (str_, cast_, to_X, icast_X,
// Node_of_), as javac's checkcasts do. This mode makes such a class a Go generic type, as j2go
// would if it emitted Go generics for Java generics, and types its uses:
//
//  0. (declarations) The class's Java declaration (java/src) says which of its fields, record
//     components, constructor and method parameters and results are declared with a type
//     variable (or with the class itself over type variables, `C<V, K>`, also as the elements
//     of a slice or Go map the native steps made): those Go slots, `any` (or `*C`), become K
//     (or *C[V, K]). Other generic classes in a body (the entries a class keeps in an erased
//     collection) stay erased there, as the objects in the collection are made and cast back
//     erased. The struct is C[K, V comparable]; methods
//     take `this *C[K, V]`; constructors and static generic methods are generic functions; a
//     non-static inner class (one with an outer_ field) takes the outer class's parameters.
//     Methods that implement a Java interface (Iterable.iterator...) or Object methods keep
//     their erased signatures, so the class still implements the same Go interfaces. Every
//     other use of the class is C[any, any], the erased class itself. The class bodies are
//     then repaired to a fixed point: a type-variable value that meets an Object is boxed as
//     the erased code boxed it (boxNull_: null is nil, strobj_, self(), box_()); an Object that
//     meets a type variable is unboxed (unboxT_: nil is Java null, a most-derived object gives
//     its view of the class asked for, as to_X does); nil is null_[K](); a type-variable field
//     starts as null_[K]() (Java's null, the null string for a String); a constructor or
//     factory call or a local takes the type arguments its context states. The runtime's uses
//     (rt_io.go, rt_json.go and their tests) are the erased class. A class whose body does not
//     type-check this way (a field of another class holds it erased, a cast takes an erased
//     object where the Java declaration is typed, a bounded type parameter, a class hierarchy)
//     is left erased, and the step starts again without it.
//  1. (uses) Every C[any, any] outside the class bodies, and every call of its constructors and
//     generic factories, is a site. Sites that exchange values (assignment, argument, result,
//     composite literal, comparison; through method results and fields the type arguments
//     follow) are one class of sites, found by type-checking the package with a distinct
//     placeholder type for every site's type arguments and unifying the placeholders that meet.
//     A class of sites takes type argument S when what the erased code reads from it is cast
//     to S (javac's checkcast names the type argument) and every value written to it is an S
//     (or a subclass or implementation of it); with no cast, when every value written is
//     statically an S. A value boxed into an Object or cast from one (cast_, is_, a type switch)
//     carries its instantiation at run time, so all sites whose values are boxed or cast are
//     one class per Java class. Conflicting or unknown evidence, and type arguments that have
//     no null of their own (primitives: Boolean, Integer; arrays), leave the sites erased (any);
//     so do all sites of a class whose body boxes its own instances.
//     Then the uses are repaired to a fixed point: the unboxing around a value read with its
//     type goes, a typed value that meets an Object is boxed as the erased code boxed it, a
//     boxed or Object value that meets a typed parameter is unboxed or unwrapped.
//
// Rerunning the mode on its output changes nothing.
//
// usage: generics -classes [-n] [-only C1,C2] <dir of package closure>

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"postflip/rw"
)

type gclass struct {
	name    string
	fqn     string
	j       *jClass
	jf      *javaFile
	jpath   []*jClass
	tparams []string
	outer   *gclass
	pinned  map[string]bool
}

var (
	family      = map[string]*gclass{} // Go name -> class made generic
	byFQN       = map[string]*gclass{}
	leftClasses = map[string]string{}  // Java generic classes left erased, with the reason
	familyFuncs = map[string]*gclass{} // constructors and static methods of family classes
	javaRoot    string
)

// ---- discovery

var reName = regexp.MustCompile(`"(\w+)":\s+"([^"]+)"`)

func goJavaNames(dir string) map[string]string {
	b, err := os.ReadFile(filepath.Join(dir, "g_0names.go"))
	if err != nil {
		panic(err)
	}
	out := map[string]string{}
	for _, m := range reName.FindAllStringSubmatch(string(b), -1) {
		out[m[1]] = m[2]
	}
	return out
}

// findJava finds the Java declaration of a class by its binary name (pkg.Outer$Inner).
func findJava(fqn string) (*javaFile, []*jClass) {
	segs := strings.Split(fqn, ".")
	k := 0
	for k < len(segs) && (segs[k] == "" || segs[k][0] < 'A' || segs[k][0] > 'Z') {
		k++
	}
	if k >= len(segs) {
		return nil, nil
	}
	parts := strings.Split(strings.Join(segs[k:], "."), "$")
	path := filepath.Join(append([]string{javaRoot}, append(segs[:k], parts[0]+".java")...)...)
	jf := parseJava(path)
	if jf == nil {
		return nil, nil
	}
	c := jf.types[parts[0]]
	if c == nil {
		return jf, nil
	}
	chain := []*jClass{c}
	for _, p := range parts[1:] {
		c = c.nested[p]
		if c == nil {
			return jf, nil
		}
		chain = append(chain, c)
	}
	return jf, chain
}

func structOf(name string) *types.Named {
	tn, ok := rw.Pkg.Scope().Lookup(name).(*types.TypeName)
	if !ok || tn.IsAlias() {
		return nil
	}
	n, ok := tn.Type().(*types.Named)
	if !ok {
		return nil
	}
	if _, ok := n.Underlying().(*types.Struct); !ok {
		return nil
	}
	return n
}

func declFile(o types.Object) string { return filepath.Base(rw.Fset.Position(o.Pos()).Filename) }

func discover(dir string, only map[string]bool) {
	names := goJavaNames(dir)
	gonames := make([]string, 0, len(names))
	for g := range names {
		gonames = append(gonames, g)
	}
	sort.Strings(gonames)
	// embedded generated classes, both ways
	embeds := map[string][]string{}
	for _, g := range gonames {
		n := structOf(g)
		if n == nil {
			continue
		}
		st := n.Underlying().(*types.Struct)
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			if !f.Embedded() {
				continue
			}
			if en, ok := types.Unalias(f.Type()).(*types.Named); ok && strings.HasPrefix(declFile(en.Obj()), "g_") {
				embeds[g] = append(embeds[g], en.Obj().Name())
			}
		}
	}
	embeddedBy := map[string]string{}
	for _, g := range gonames {
		for _, e := range embeds[g] {
			if embeddedBy[e] == "" {
				embeddedBy[e] = g
			}
		}
	}
	for _, g := range gonames {
		n := structOf(g)
		if n == nil || !strings.HasPrefix(declFile(n.Obj()), "g_") {
			continue
		}
		jf, chain := findJava(names[g])
		if chain == nil {
			continue
		}
		j := chain[len(chain)-1]
		if len(j.tparams) == 0 {
			continue
		}
		fqn := names[g]
		switch {
		case n.TypeParams() != nil:
			continue // already generic
		case only != nil && !only[g]:
			leftClasses[fqn] = "not selected (-only)"
		case j.bounded:
			leftClasses[fqn] = "a bounded type parameter (j2go erases it to its bound, not to Object)"
		case j.superName != "":
			leftClasses[fqn] = "extends " + j.superName + " (a class hierarchy: its dispatch and casts assert to one Go type per class)"
		case embeddedBy[g] != "":
			leftClasses[fqn] = "extended by " + embeddedBy[g] + " (a class hierarchy)"
		case len(embeds[g]) > 0:
			leftClasses[fqn] = "embeds " + embeds[g][0]
		default:
			gc := &gclass{name: g, fqn: fqn, j: j, jf: jf, jpath: chain, tparams: append([]string(nil), j.tparams...)}
			family[g] = gc
			byFQN[fqn] = gc
		}
	}
	// non-static inner classes: the outer class's type parameters
	for changed := true; changed; {
		changed = false
		for _, g := range gonames {
			if family[g] != nil {
				continue
			}
			n := structOf(g)
			if n == nil || !strings.HasPrefix(declFile(n.Obj()), "g_") || n.TypeParams() != nil {
				continue
			}
			st := n.Underlying().(*types.Struct)
			for i := 0; i < st.NumFields(); i++ {
				f := st.Field(i)
				if f.Name() != "outer_" {
					continue
				}
				p, ok := f.Type().(*types.Pointer)
				if !ok {
					continue
				}
				on, ok := p.Elem().(*types.Named)
				if !ok || family[on.Obj().Name()] == nil {
					continue
				}
				outer := family[on.Obj().Name()]
				jf, chain := findJava(names[g])
				var j *jClass
				if chain != nil {
					j = chain[len(chain)-1]
					if len(j.tparams) > 0 {
						break // an inner class with type parameters of its own
					}
				}
				for _, e := range embeds[g] {
					if family[e] == nil {
						leftClasses[outer.fqn] = "inner class " + g + " extends " + e
					}
				}
				gc := &gclass{name: g, fqn: names[g], j: j, jf: jf, jpath: chain, tparams: outer.tparams, outer: outer}
				family[g] = gc
				byFQN[names[g]] = gc
				changed = true
			}
		}
	}
	// an outer class left for its inner class
	for g, gc := range family {
		root := gc
		for root.outer != nil {
			root = root.outer
		}
		if _, left := leftClasses[root.fqn]; left {
			delete(family, g)
			delete(byFQN, gc.fqn)
		}
	}
	// pinned methods: those of the Java interfaces the class declares, and Object's
	for _, gc := range family {
		gc.pinned = map[string]bool{"equals": true, "hashCode": true, "toString": true, "compareTo": true, "clone": true, "self": true}
		if gc.j == nil {
			continue
		}
		for _, in := range gc.j.ifaces {
			goName := strings.ReplaceAll(in, ".", "_")
			tn, ok := rw.Pkg.Scope().Lookup(goName).(*types.TypeName)
			if !ok {
				continue
			}
			if it, ok := tn.Type().Underlying().(*types.Interface); ok {
				for i := 0; i < it.NumMethods(); i++ {
					gc.pinned[it.Method(i).Name()] = true
				}
			}
		}
	}
}

// resolveJava finds the generified class a Java type name in class gc's source names.
func resolveJava(name string, gc *gclass) *gclass {
	if gc.jf == nil {
		return nil
	}
	parts := strings.Split(name, ".")
	first := parts[0]
	rest := ""
	if len(parts) > 1 {
		rest = "$" + strings.Join(parts[1:], "$")
	}
	// enclosing classes, innermost first
	enc := gc
	for enc != nil && enc.jpath == nil {
		enc = enc.outer
	}
	if enc != nil {
		base := enc.fqn
		for i := len(enc.jpath) - 1; i >= 0; i-- {
			c := enc.jpath[i]
			if c.name == first {
				if g := byFQN[base+rest]; g != nil {
					return g
				}
			}
			if c.nested[first] != nil {
				if g := byFQN[base+"$"+first+rest]; g != nil {
					return g
				}
			}
			if k := strings.LastIndex(base, "$"); k >= 0 {
				base = base[:k]
			}
		}
	}
	pkg := gc.jf.pkg
	if g := byFQN[pkg+"."+first+rest]; g != nil {
		return g
	}
	for _, im := range gc.jf.imports {
		if strings.HasSuffix(im, "."+first) {
			// a.b.Outer.Inner: try each split into package and nested classes
			segs := strings.Split(im, ".")
			for k := 1; k < len(segs); k++ {
				fq := strings.Join(segs[:k], ".") + "." + strings.Join(segs[k:], "$") + rest
				if g := byFQN[fq]; g != nil {
					return g
				}
			}
		}
	}
	return nil
}

// ---- slot mapping from the Java declaration

func contains(xs []string, s string) bool {
	for _, x := range xs {
		if x == s {
			return true
		}
	}
	return false
}

// mapSlot is the Go type a slot of Go type goT takes when Java declares it jt with type
// variables scope in scope, or "".
func mapSlot(jt *jType, scope []string, goT types.Type, gc *gclass) string {
	if jt == nil || jt.dims > 0 || goT == nil {
		return ""
	}
	if contains(scope, jt.name) && len(jt.args) == 0 {
		if isAny(goT) {
			return jt.name
		}
		return ""
	}
	// a collection the native steps made a slice or a Go map
	switch u := types.Unalias(goT).(type) {
	case *types.Slice:
		if len(jt.args) == 1 {
			if r := mapSlot(jt.args[0], scope, u.Elem(), gc); r != "" {
				return "[]" + r
			}
		}
		return ""
	case *types.Map:
		if len(jt.args) == 2 {
			rk := mapSlot(jt.args[0], scope, u.Key(), gc)
			rv := mapSlot(jt.args[1], scope, u.Elem(), gc)
			if rk == "" && rv == "" {
				return ""
			}
			if rk == "" {
				rk = tstr(u.Key())
			}
			if rv == "" {
				rv = tstr(u.Elem())
			}
			return "map[" + rk + "]" + rv
		}
		return ""
	}
	tgt := resolveJava(jt.name, gc)
	if tgt == nil || len(jt.args) != len(tgt.tparams) || len(jt.args) == 0 {
		return ""
	}
	// only the class itself (or an enclosing one): objects of another generic class, such as
	// the entries a class keeps in an erased collection, may be made and cast back erased
	own := false
	for c := gc; c != nil; c = c.outer {
		if c == tgt {
			own = true
		}
	}
	if !own {
		return ""
	}
	p, ok := types.Unalias(goT).(*types.Pointer)
	if !ok {
		return ""
	}
	if n, ok := p.Elem().(*types.Named); !ok || n.Obj().Name() != tgt.name {
		return ""
	}
	var args []string
	for _, a := range jt.args {
		if a.dims > 0 || len(a.args) > 0 || !contains(scope, a.name) {
			return ""
		}
		args = append(args, a.name)
	}
	return "*" + tgt.name + "[" + strings.Join(args, ", ") + "]"
}

func trimU(s string) string { return strings.TrimRight(s, "_") }

// javaMember finds the Java member that Go function or method goName (with parameters
// params) translates: kind ctor or method, static or not.
func javaMember(gc *gclass, goName string, params []string, kind string, static bool) *jMember {
	if gc.j == nil {
		return nil
	}
	var found *jMember
	for _, m := range gc.j.members {
		if m.kind != kind || kind == "method" && m.static != static {
			continue
		}
		if kind == "method" && !(goName == m.name || goName == m.name+"_impl" || strings.HasPrefix(goName, m.name+"_")) {
			continue
		}
		if len(m.params) != len(params) {
			continue
		}
		ok := true
		for i, p := range m.params {
			if trimU(p.name) != trimU(params[i]) {
				ok = false
			}
		}
		if !ok {
			continue
		}
		if found != nil {
			return nil // ambiguous
		}
		found = m
	}
	return found
}

func javaField(gc *gclass, goName string) *jMember {
	if gc.j == nil {
		return nil
	}
	for _, m := range gc.j.members {
		if m.kind == "field" && !m.static && (m.name == goName || trimU(goName) == m.name) {
			return m
		}
	}
	return nil
}

func fieldNames(ft *ast.FuncType) []string {
	var out []string
	if ft.Params == nil {
		return out
	}
	for _, f := range ft.Params.List {
		for _, n := range f.Names {
			out = append(out, n.Name)
		}
		if len(f.Names) == 0 {
			out = append(out, "_")
		}
	}
	return out
}

// funcOwner is the family class a top-level function belongs to (constructor or static
// method), with the Java member it translates.
func funcOwner(fd *ast.FuncDecl) (*gclass, *jMember) {
	name := fd.Name.Name
	params := fieldNames(fd.Type)
	if strings.HasPrefix(name, "new_") {
		if gc := family[name[4:]]; gc != nil {
			return gc, javaMember(gc, "", params, "ctor", false)
		}
	}
	// the longest family class name that prefixes it
	var best *gclass
	for g, gc := range family {
		if strings.HasPrefix(name, g+"_") && (best == nil || len(g) > len(best.name)) {
			best = gc
		}
	}
	if best == nil {
		return nil, nil
	}
	m := javaMember(best, strings.TrimPrefix(name, best.name+"_"), params, "method", true)
	if m == nil {
		return nil, nil
	}
	return best, m
}

// ---- type helpers

func isTypeParam(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.TypeParam)
	return ok
}

func recvClass(fd *ast.FuncDecl) *gclass {
	if fd.Recv == nil || len(fd.Recv.List) != 1 {
		return nil
	}
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	switch x := t.(type) {
	case *ast.Ident:
		return family[x.Name]
	case *ast.IndexExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			return family[id.Name]
		}
	case *ast.IndexListExpr:
		if id, ok := x.X.(*ast.Ident); ok {
			return family[id.Name]
		}
	}
	return nil
}

func recvIdent(fd *ast.FuncDecl) *ast.Ident {
	t := fd.Recv.List[0].Type
	if s, ok := t.(*ast.StarExpr); ok {
		t = s.X
	}
	id, _ := t.(*ast.Ident)
	return id
}

// inBody reports whether n is inside the declaration of a family class or of one of its
// methods, constructors or static methods.
func inBody(n ast.Node) bool {
	for q := n; q != nil; q = rw.Parent[q] {
		switch d := q.(type) {
		case *ast.FuncDecl:
			if recvClass(d) != nil {
				return true
			}
			if _, ok := familyFuncs[d.Name.Name]; ok {
				return true
			}
			return false
		case *ast.TypeSpec:
			return family[d.Name.Name] != nil
		}
	}
	return false
}

func typeArgsText(n int, s string) string {
	a := make([]string, n)
	for i := range a {
		a[i] = s
	}
	return strings.Join(a, ", ")
}

// ---- 0. declarations

func declare() {
	// static and constructor functions first, so that inBody knows them
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil {
				if gc, _ := funcOwner(fd); gc != nil {
					familyFuncs[fd.Name.Name] = gc
				}
			}
		}
	}
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.GenDecl:
				for _, sp := range x.Specs {
					ts, ok := sp.(*ast.TypeSpec)
					if !ok || family[ts.Name.Name] == nil || ts.TypeParams != nil {
						continue
					}
					gc := family[ts.Name.Name]
					rw.Rewrite[ts.Name] = func() string { return gc.name + "[" + strings.Join(gc.tparams, ", ") + " comparable]" }
					stats["0: classes made generic"]++
					st, ok := ts.Type.(*ast.StructType)
					if !ok {
						continue
					}
					for _, fl := range st.Fields.List {
						if len(fl.Names) != 1 {
							continue
						}
						if gc.outer != nil && fl.Names[0].Name == "outer_" {
							r := "*" + gc.outer.name + "[" + strings.Join(gc.tparams, ", ") + "]"
							rw.Rewrite[fl.Type] = func() string { return r }
							continue
						}
						jm := javaField(gc, fl.Names[0].Name)
						if jm == nil {
							continue
						}
						if r := mapSlot(jm.typ, gc.tparams, rw.Info.Types[fl.Type].Type, gc); r != "" {
							rw.Rewrite[fl.Type] = func() string { return r }
							stats["0: fields typed"]++
						}
					}
				}
			case *ast.FuncDecl:
				if gc := recvClass(x); gc != nil {
					if id := recvIdent(x); id != nil {
						rw.Rewrite[id] = func() string { return gc.name + "[" + strings.Join(gc.tparams, ", ") + "]" }
					}
					name := x.Name.Name
					var jm *jMember
					if strings.HasPrefix(name, "init_") && strings.TrimPrefix(name, "init_") == gc.name {
						jm = javaMember(gc, "", fieldNames(x.Type), "ctor", false)
					} else if !gc.pinned[name] && !gc.pinned[strings.TrimSuffix(name, "_impl")] {
						jm = javaMember(gc, name, fieldNames(x.Type), "method", false)
					}
					if jm != nil && len(jm.tparams) == 0 {
						mapFunc(x, jm, gc.tparams, gc)
					}
					continue
				}
				gc, jm := funcOwner(x)
				if gc == nil {
					continue
				}
				scope := gc.tparams
				if jm != nil && jm.kind == "method" {
					scope = jm.tparams
				}
				var used []string
				if jm != nil {
					used = mapFunc(x, jm, scope, gc)
				}
				if strings.HasPrefix(x.Name.Name, "new_") {
					used = gc.tparams
					addNullInits(x, gc)
					if gc.outer != nil {
						for _, fl := range x.Type.Params.List {
							if len(fl.Names) == 1 && fl.Names[0].Name == "outer_" {
								r := "*" + gc.outer.name + "[" + strings.Join(gc.tparams, ", ") + "]"
								rw.Rewrite[fl.Type] = func() string { return r }
							}
						}
					}
					if x.Type.Results != nil && len(x.Type.Results.List) == 1 {
						r := "*" + gc.name + "[" + strings.Join(gc.tparams, ", ") + "]"
						rw.Rewrite[x.Type.Results.List[0].Type] = func() string { return r }
					}
				}
				if len(used) > 0 {
					fn := x.Name.Name
					u := used
					rw.Rewrite[x.Name] = func() string { return fn + "[" + strings.Join(u, ", ") + " comparable]" }
					stats["0: constructors and static methods made generic"]++
				}
			}
		}
	}
	// every other use of a family class is the erased class (in the runtime too)
	for _, f := range rw.Files {
		if !generated(f) && !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "rt_") {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			id, ok := x.(*ast.Ident)
			if !ok {
				return true
			}
			tn, ok := rw.Info.Uses[id].(*types.TypeName)
			if !ok || family[tn.Name()] == nil || tn.Pkg() != rw.Pkg {
				return true
			}
			if _, done := rw.Rewrite[id]; done {
				return true
			}
			switch p := rw.Parent[id].(type) {
			case *ast.IndexExpr:
				if p.X == id {
					return true
				}
			case *ast.IndexListExpr:
				if p.X == id {
					return true
				}
			}
			gc := family[tn.Name()]
			rw.Rewrite[id] = func() string { return gc.name + "[" + typeArgsText(len(gc.tparams), "any") + "]" }
			stats["0: other uses as the erased class C[any]"]++
			return true
		})
	}
}

// mapFunc retypes the parameters and result of fd that Java declares with type variables of
// scope, and returns the type variables used, in scope order.
func mapFunc(fd *ast.FuncDecl, jm *jMember, scope []string, gc *gclass) []string {
	usedSet := map[string]bool{}
	var mapped []string
	i := 0
	for _, fl := range fd.Type.Params.List {
		k := len(fl.Names)
		if k == 0 {
			k = 1
		}
		var r string
		for j := 0; j < k; j++ {
			if i+j >= len(jm.params) {
				return nil
			}
			rj := mapSlot(jm.params[i+j].typ, scope, rw.Info.Types[fl.Type].Type, gc)
			if j > 0 && rj != r {
				r = ""
				break
			}
			r = rj
		}
		i += k
		if r != "" {
			rr := r
			rw.Rewrite[fl.Type] = func() string { return rr }
			mapped = append(mapped, rr)
			stats["0: parameters typed"]++
		}
	}
	if jm.typ != nil && fd.Type.Results != nil && len(fd.Type.Results.List) == 1 && len(fd.Type.Results.List[0].Names) <= 1 {
		rt := fd.Type.Results.List[0].Type
		if r := mapSlot(jm.typ, scope, rw.Info.Types[rt].Type, gc); r != "" {
			rw.Rewrite[rt] = func() string { return r }
			mapped = append(mapped, r)
			stats["0: results typed"]++
		}
	}
	for _, m := range mapped {
		for _, tp := range scope {
			if regexp.MustCompile(`\b` + tp + `\b`).MatchString(m) {
				usedSet[tp] = true
			}
		}
	}
	var used []string
	for _, tp := range scope {
		if usedSet[tp] {
			used = append(used, tp)
		}
	}
	return used
}

// addNullInits makes the `this := &C{}` of constructor fd a &C[K, V]{} whose type-variable
// fields start as Java's null.
func addNullInits(fd *ast.FuncDecl, gc *gclass) {
	ast.Inspect(fd.Body, func(x ast.Node) bool {
		cl, ok := x.(*ast.CompositeLit)
		if !ok {
			return true
		}
		id, ok := cl.Type.(*ast.Ident)
		if !ok || id.Name != gc.name {
			return true
		}
		st, ok := rw.Info.Types[cl].Type.Underlying().(*types.Struct)
		if !ok {
			return false
		}
		var inits []string
		for i := 0; i < st.NumFields(); i++ {
			f := st.Field(i)
			jm := javaField(gc, f.Name())
			if jm == nil {
				continue
			}
			if r := mapSlot(jm.typ, gc.tparams, f.Type(), gc); r != "" && contains(gc.tparams, r) {
				inits = append(inits, f.Name()+": null_["+r+"]()")
			}
		}
		text := gc.name + "[" + strings.Join(gc.tparams, ", ") + "]{" + strings.Join(inits, ", ") + "}"
		if len(cl.Elts) == 0 {
			rw.Rewrite[cl] = func() string { return text }
		} else {
			rw.Rewrite[id] = func() string { return gc.name + "[" + strings.Join(gc.tparams, ", ") + "]" }
		}
		return false
	})
}

// ---- repairs

var (
	reMismatchNil = regexp.MustCompile(`^invalid operation: (.*) (==|!=) nil \(mismatched types (.*) and untyped nil\)$`)
	reCannotInfer = regexp.MustCompile(`^in call to (.*), cannot infer (\w+)`)
	reInferred    = regexp.MustCompile(`^in call to (.*), type (.*) of (.*) does not match inferred type (.*) for (.*)$`)
	reGenericUse  = regexp.MustCompile(`^cannot use generic (function|type) (\S+) without instantiation`)
)

// nullOf is Java null as a value of type t.
func nullOf(t types.Type) string {
	if b, ok := types.Unalias(t).(*types.Basic); ok && b.Kind() == types.String {
		return "nullstr_"
	}
	if isIface(t) && !isTypeParam(t) {
		return "nil"
	}
	if _, ok := types.Unalias(t).(*types.Pointer); ok {
		return "nil"
	}
	return "null_[" + tstr(t) + "]()"
}

// boxConv is how a value of type t is boxed and unboxed as the erased code does it.
func boxConv(t types.Type) rw.Conv {
	if isTypeParam(t) {
		ts := tstr(t)
		return rw.Conv{Box: func(e string) string { return "boxNull_(" + e + ")" }, Unbox: func(e string) string { return "unboxT_[" + ts + "](" + e + ")" },
			IsUnbox: func(n string) bool { return n == "unboxT_["+ts+"]" }, OK: true}
	}
	cv := rw.ConvFor(t)
	if cv.OK {
		return cv
	}
	ts := tstr(t)
	return rw.Conv{Box: func(e string) string { return "obj_(" + e + ")" }, Unbox: func(e string) string { return "cast_[" + ts + "](" + e + ")" },
		IsUnbox: func(n string) bool { return n == "cast_["+ts+"]" }, OK: true}
}

// sameOrigin reports whether h and w are instantiations of one generic named type (through a
// pointer), returning them.
func sameOrigin(h, w types.Type) (*types.Named, *types.Named) {
	h, w = types.Unalias(h), types.Unalias(w)
	if hp, ok := h.(*types.Pointer); ok {
		if wp, ok := w.(*types.Pointer); ok {
			h, w = hp.Elem(), wp.Elem()
		}
	}
	hn, ok1 := h.(*types.Named)
	wn, ok2 := w.(*types.Named)
	if !ok1 || !ok2 || hn.Origin() != wn.Origin() || hn.TypeArgs().Len() == 0 {
		return nil, nil
	}
	return hn, wn
}

func argsText(n *types.Named) string {
	var a []string
	for i := 0; i < n.TypeArgs().Len(); i++ {
		a = append(a, tstr(n.TypeArgs().At(i)))
	}
	return strings.Join(a, ", ")
}

// funcObj is the function a call expression's Fun names, with its explicit instantiation.
func funcObj(fun ast.Expr) (*types.Func, *ast.Ident, ast.Expr) {
	fun = rw.Unparen(fun)
	var idx ast.Expr
	switch x := fun.(type) {
	case *ast.IndexExpr:
		idx, fun = x, x.X
	case *ast.IndexListExpr:
		idx, fun = x, x.X
	}
	id, ok := fun.(*ast.Ident)
	if !ok {
		return nil, nil, nil
	}
	f, _ := rw.Info.Uses[id].(*types.Func)
	return f, id, idx
}

// instantiateFor rewrites the call of generic function f (at fun) so that its result is of
// type want, if its result is a family class over its type parameters.
func instantiateFor(call *ast.CallExpr, want types.Type) bool {
	f, id, idx := funcObj(call.Fun)
	if f == nil {
		return false
	}
	sig := f.Origin().Type().(*types.Signature)
	if sig.TypeParams() == nil || sig.Results().Len() != 1 {
		return false
	}
	rn, wn := sameOrigin(sig.Results().At(0).Type(), want)
	if rn == nil {
		// a raw C[any] context gives any
		return false
	}
	args := make([]string, sig.TypeParams().Len())
	for i := 0; i < rn.TypeArgs().Len(); i++ {
		if tp, ok := rn.TypeArgs().At(i).(*types.TypeParam); ok {
			for k := 0; k < sig.TypeParams().Len(); k++ {
				if sig.TypeParams().At(k) == tp {
					args[k] = tstr(wn.TypeArgs().At(i))
				}
			}
		}
	}
	for _, a := range args {
		if a == "" {
			return false
		}
	}
	name := id.Name
	text := name + "[" + strings.Join(args, ", ") + "]"
	if idx != nil {
		rw.Rewrite[idx] = func() string { return text }
	} else {
		rw.Rewrite[id] = func() string { return text }
	}
	return true
}

// retypeDecl gives the declaration of local variable (or parameter) id the type want.
func retypeDecl(id *ast.Ident, want types.Type) bool {
	v, ok := rw.Info.Uses[id].(*types.Var)
	if ok && v.IsField() {
		return retypeField(v, want)
	}
	if !ok || v.Parent() == rw.Pkg.Scope() {
		return false
	}
	for _, f := range rw.Files {
		if rw.Fset.File(f.Pos()) != rw.Fset.File(v.Pos()) {
			continue
		}
		var te ast.Expr
		ast.Inspect(f, func(x ast.Node) bool {
			if x == nil || te != nil || x.Pos() > v.Pos() || x.End() < v.Pos() {
				return x == nil || te == nil
			}
			switch d := x.(type) {
			case *ast.ValueSpec:
				for _, n := range d.Names {
					if n.Pos() == v.Pos() && d.Type != nil && len(d.Names) == 1 {
						te = d.Type
					}
				}
			}
			return true
		})
		if te == nil || !inBody(te) {
			return false
		}
		if _, done := rw.Rewrite[te]; done {
			return true
		}
		w := tstr(want)
		rw.Rewrite[te] = func() string { return w }
		stats["repair: locals typed from their context"]++
		return true
	}
	return false
}

// boxInner is x when e is obj_(x), strobj_(x), any(x), x.self() or x.box_().
func boxInner(e ast.Expr) ast.Expr {
	call, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil
	}
	if id, ok := call.Fun.(*ast.Ident); ok && len(call.Args) == 1 && (id.Name == "obj_" || id.Name == "strobj_" || id.Name == "any" || id.Name == "boxNull_") {
		return call.Args[0]
	}
	if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
		return s.X
	}
	return nil
}

func repair(errs []types.Error) int {
	n := 0
	done := map[ast.Node]bool{}
	mark := func(e ast.Node) bool {
		for q := e; q != nil; q = rw.Parent[q] {
			if done[q] {
				return false
			}
			if _, ok := rw.Rewrite[q]; ok {
				return false
			}
		}
		done[e] = true
		return true
	}
	var unknown []string
	for _, te := range errs {
		msg := te.Msg
		pos := rw.Fset.Position(te.Pos).String()
		if x, _, ok := parseUse(msg); ok {
			e := exprAt(te.Pos, x)
			if e != nil && isRuntime(e) {
				// the runtime takes the erased class: only its calls are instantiated
				if call, ok := rw.Unparen(e).(*ast.CallExpr); ok && mark(e) {
					if want := slotType(e); want != nil && instantiateFor(call, want) {
						stats["repair: runtime calls instantiated as the erased class"]++
						n++
						continue
					}
				}
			}
			if e == nil || !generated(e) {
				unknown = append(unknown, pos+": "+msg)
				continue
			}
			have := rw.Info.Types[e].Type
			want := slotType(e)
			if have == nil || want == nil {
				unknown = append(unknown, pos+": "+msg+" (slot not found)")
				continue
			}
			if !mark(e) {
				continue
			}
			if hn, wn := sameOrigin(have, want); hn != nil {
				// a producer or a local of the erased class meets a typed one
				if hasAnyArg(wn) && !hasAnyArg(hn) && retypeWantSlot(e, have) {
					n++
					continue
				}
				if call, ok := rw.Unparen(e).(*ast.CallExpr); ok && reinstantiate(call, want) {
					stats["repair: constructor and factory calls instantiated from their context"]++
					n++
					continue
				}
				if id, ok := rw.Unparen(e).(*ast.Ident); ok && retypeDecl(id, want) {
					n++
					continue
				}
				if se, ok := rw.Unparen(e).(*ast.SelectorExpr); ok && hasAnyArg(hn) && retypeDecl(se.Sel, want) {
					n++
					continue
				}
				if cl, ok := rw.Unparen(e).(*ast.CompositeLit); ok && inBody(cl) {
					w := tstr(want)
					rw.Rewrite[cl.Type] = func() string { return w }
					stats["repair: composite literals typed from their context"]++
					n++
					continue
				}
				if u, ok := rw.Unparen(e).(*ast.UnaryExpr); ok && u.Op == token.AND {
					if cl, ok := u.X.(*ast.CompositeLit); ok {
						w := tstr(wn)
						rw.Rewrite[cl.Type] = func() string { return w }
						stats["repair: composite literals typed from their context"]++
						n++
						continue
					}
				}
				// the context side is a local declared erased
				if p, ok := rw.Up(e).(*ast.AssignStmt); ok && len(p.Lhs) == 1 {
					if id, ok := p.Lhs[0].(*ast.Ident); ok && retypeDecl(id, have) {
						n++
						continue
					}
				}
				_ = hn
				unknown = append(unknown, pos+": "+msg)
				continue
			}
			switch {
			case isAny(have) && (!isIface(want) || !isAny(want)):
				cv := boxConv(want)
				w := want
				ee := e
				rw.Rewrite[e] = func() string {
					if in := boxInner(ee); in != nil {
						if tv, ok := rw.Info.Types[in]; ok && types.Identical(tv.Type, w) {
							return rw.Render(in)
						}
					}
					return cv.Unbox(rw.RenderChildren(ee))
				}
				stats["repair: Objects unboxed where a typed value is wanted"]++
				n++
			case isIface(have) && isTypeParam(want):
				cv := boxConv(want)
				ee := e
				rw.Rewrite[e] = func() string { return cv.Unbox(rw.RenderChildren(ee)) }
				stats["repair: Objects unboxed where a typed value is wanted"]++
				n++
			case !isIface(have) && !isIface(want) && (isTypeParam(have) || isTypeParam(want)):
				// between a type variable and a concrete type: through the box
				hb, wu := boxConv(have), boxConv(want)
				ee := e
				rw.Rewrite[e] = func() string { return wu.Unbox(hb.Box(rw.RenderChildren(ee))) }
				stats["repair: values converted through their box"]++
				n++
			default:
				unknown = append(unknown, pos+": "+msg)
			}
			continue
		}
		if m := reNil.FindStringSubmatch(msg); m != nil {
			e := exprAt(te.Pos, "nil")
			if e != nil && generated(e) && mark(e) {
				if want := slotType(e); want != nil {
					s := nullOf(want)
					rw.Rewrite[e] = func() string { return s }
					stats["repair: null as a typed value"]++
					n++
					continue
				}
			}
		}
		if m := reMismatchNil.FindStringSubmatch(msg); m != nil {
			if b := binaryAt(te.Pos); b != nil && mark(b) {
				nilSide, other := b.Y, b.X
				if id, ok := b.X.(*ast.Ident); ok && id.Name == "nil" {
					nilSide, other = b.X, b.Y
				}
				if t := rw.Info.Types[other].Type; t != nil {
					s := nullOf(t)
					rw.Rewrite[nilSide] = func() string { return s }
					stats["repair: comparisons with null as a typed value"]++
					n++
					continue
				}
			}
		}
		if m := reCannotInfer.FindStringSubmatch(msg); m != nil {
			if call := callAt(te.Pos); call != nil && mark(call) {
				want := slotType(call)
				if want != nil && instantiateFor(call, want) {
					stats["repair: constructor and factory calls instantiated from their context"]++
					n++
					continue
				}
				// no typed context: the erased class
				if f, id, idx := funcObj(call.Fun); f != nil && idx == nil {
					k := f.Type().(*types.Signature).TypeParams().Len()
					name := id.Name
					rw.Rewrite[id] = func() string { return name + "[" + typeArgsText(k, "any") + "]" }
					stats["repair: constructor and factory calls of the erased class"]++
					n++
					continue
				}
			}
		}
		if m := reInferred.FindStringSubmatch(msg); m != nil {
			// in call to F, type X of ARG does not match inferred type Y for P
			if call := callAt(te.Pos); call != nil && mark(call) {
				if instantiateFromArgs(call) {
					stats["repair: generic calls instantiated from their typed arguments"]++
					n++
					continue
				}
			}
		}
		if m := reNotIface.FindStringSubmatch(msg); m != nil {
			e := exprAt(te.Pos, m[1])
			if e != nil && generated(e) && mark(e) {
				if t := rw.Info.Types[e].Type; t != nil {
					cv := boxConv(t)
					rw.Rewrite[e] = func() string { return cv.Box(rw.RenderChildren(e)) }
					stats["repair: typed values boxed for a type assertion"]++
					n++
					continue
				}
			}
		}
		unknown = append(unknown, pos+": "+msg)
	}
	if n == 0 && len(unknown) > 0 {
		for _, u := range unknown {
			fmt.Fprintln(os.Stderr, "unrepaired:", u)
		}
		failed = errs
	}
	return n
}

// failed are the type errors of the last round that the repairs could not reduce.
var failed []types.Error

func nodeAt(pos token.Pos, pred func(ast.Node) bool) ast.Node {
	var best ast.Node
	for _, f := range rw.Files {
		if rw.Fset.File(f.Pos()) != rw.Fset.File(pos) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			if x == nil || x.Pos() > pos || x.End() < pos {
				return false
			}
			if pred(x) {
				best = x // innermost
			}
			return true
		})
	}
	return best
}

func binaryAt(pos token.Pos) *ast.BinaryExpr {
	n := nodeAt(pos, func(x ast.Node) bool {
		b, ok := x.(*ast.BinaryExpr)
		if !ok || b.Op != token.EQL && b.Op != token.NEQ {
			return false
		}
		for _, s := range []ast.Expr{b.X, b.Y} {
			if id, ok := s.(*ast.Ident); ok && id.Name == "nil" {
				return true
			}
		}
		return false
	})
	b, _ := n.(*ast.BinaryExpr)
	return b
}

func callAt(pos token.Pos) *ast.CallExpr {
	n := nodeAt(pos, func(x ast.Node) bool {
		c, ok := x.(*ast.CallExpr)
		if !ok {
			return false
		}
		f, _, _ := funcObj(c.Fun)
		return f != nil && f.Type().(*types.Signature).TypeParams() != nil
	})
	c, _ := n.(*ast.CallExpr)
	return c
}

// ctxType is slotType, and the key of a map index and the operand of a conversion to an
// interface type.
func ctxType(e ast.Expr) types.Type {
	if t := slotType(e); t != nil {
		return t
	}
	switch p := rw.Up(e).(type) {
	case *ast.IndexExpr:
		if rw.Unparen(p.Index) == rw.Unparen(e) {
			if m, ok := rw.Info.Types[p.X].Type.Underlying().(*types.Map); ok {
				return m.Key()
			}
		}
	case *ast.CallExpr:
		if tv, ok := rw.Info.Types[p.Fun]; ok && tv.IsType() && len(p.Args) == 1 && isIface(tv.Type) {
			return tv.Type
		}
	case *ast.SendStmt:
		if rw.Unparen(p.Value) == rw.Unparen(e) {
			if c, ok := rw.Info.Types[p.Chan].Type.Underlying().(*types.Chan); ok {
				return c.Elem()
			}
		}
	}
	return nil
}

// newlyTyped reports whether e reads a type-variable member of a family class (a method or
// generic function result, or a field).
func newlyTyped(e ast.Expr) bool {
	switch x := rw.Unparen(e).(type) {
	case *ast.CallExpr:
		var obj types.Object
		switch f := rw.Unparen(x.Fun).(type) {
		case *ast.SelectorExpr:
			if sel := rw.Info.Selections[f]; sel != nil {
				obj = sel.Obj()
			}
		default:
			fo, _, _ := funcObj(f)
			if fo != nil {
				obj = fo
			}
		}
		fn, ok := obj.(*types.Func)
		if !ok {
			return false
		}
		sig := fn.Origin().Type().(*types.Signature)
		if sig.Results().Len() != 1 || !isTypeParam(sig.Results().At(0).Type()) {
			return false
		}
		if r := sig.Recv(); r != nil {
			t := r.Type()
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			n, ok := t.(*types.Named)
			return ok && family[n.Origin().Obj().Name()] != nil
		}
		return familyFuncs[fn.Name()] != nil
	case *ast.SelectorExpr:
		sel := rw.Info.Selections[x]
		if sel == nil || sel.Kind() != types.FieldVal {
			return false
		}
		v := sel.Obj().(*types.Var)
		if !isTypeParam(v.Origin().Type()) {
			return false
		}
		t := sel.Recv()
		if p, ok := t.(*types.Pointer); ok {
			t = p.Elem()
		}
		n, ok := types.Unalias(t).(*types.Named)
		return ok && family[n.Origin().Obj().Name()] != nil
	}
	return false
}

// boxes boxes the type-variable values (in the class bodies) and the newly typed reads (in
// the uses) that meet an Object, as the erased code boxed them, and drops the unboxing
// around a typed read.
func boxes() int {
	n := 0
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			e, ok := x.(ast.Expr)
			if !ok {
				return true
			}
			tv, ok := rw.Info.Types[e]
			if !ok || tv.Type == nil || tv.IsType() {
				return true
			}
			t := tv.Type
			if !isTypeParam(t) && (isIface(t) || !newlyTyped(e)) {
				return true
			}
			cv := boxConv(t)
			// explicit boxing
			if p, ok := rw.Up(e).(*ast.CallExpr); ok && len(p.Args) == 1 && rw.Unparen(p.Args[0]) == rw.Unparen(e) {
				name, _ := rw.CallName(p)
				if name == "obj_" || name == "any" {
					want := cv.Box(rw.Text(e))
					if rw.Text(p) != want {
						rw.Rewrite[p] = func() string { return cv.Box(rw.Render(e)) }
						stats["boxes: explicit boxing of a typed value made the erased code's"]++
						n++
					}
					return false
				}
				if name != "" && cv.IsUnbox(name) {
					rw.Rewrite[p] = func() string { return rw.Render(e) }
					stats["boxes: unboxing of a typed read removed"]++
					n++
					return false
				}
				if name == "boxNull_" || name == "strobj_" || strings.HasPrefix(name, "unboxT_") {
					return false
				}
			}
			if s, ok := rw.Up(e).(*ast.SelectorExpr); ok && s.X == e && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
				return false
			}
			if st := ctxType(e); st != nil && isIface(st) && !isTypeParam(st) {
				rw.Rewrite[e] = func() string { return cv.Box(rw.RenderChildren(e)) }
				if isTypeParam(t) {
					stats["boxes: type-variable values boxed where they meet an Object"]++
				} else {
					stats["boxes: typed reads boxed where they meet an Object"]++
				}
				n++
				return false
			}
			return true
		})
	}
	return n
}

// fixpoint repairs type errors, then boxes, until the package type-checks and nothing changes.
// fixpoint repairs and boxes until the package type-checks and nothing changes. It returns the
// type errors left when it cannot get there.
func fixpoint(dir string, report bool) []types.Error {
	var last []types.Error
	for iter := 0; iter < 30; iter++ {
		errs := load(dir)
		failed = nil
		n, what := 0, ""
		if len(errs) > 0 {
			n, what = repair(errs), "type errors"
		} else {
			n, what = boxes(), "boxes"
		}
		fmt.Fprintf(os.Stderr, "  round %d: %d errors, %d rewrites (%s)\n", iter, len(errs), n, what)
		if failed != nil {
			return failed
		}
		if n == 0 || report {
			return nil
		}
		last = errs
		rw.Save()
	}
	fmt.Fprintln(os.Stderr, "no fixed point")
	if len(last) == 0 {
		last = load(dir)
	}
	return last
}

// owners are the family classes (their outermost classes) whose bodies hold the errors.
func owners(errs []types.Error) []*gclass {
	seen := map[*gclass]bool{}
	var out []*gclass
	for _, te := range errs {
		var owner *gclass
		for _, f := range rw.Files {
			if rw.Fset.File(f.Pos()) != rw.Fset.File(te.Pos) {
				continue
			}
			for _, d := range f.Decls {
				if d.Pos() > te.Pos || d.End() < te.Pos {
					continue
				}
				switch x := d.(type) {
				case *ast.FuncDecl:
					if gc := recvClass(x); gc != nil {
						owner = gc
					} else if gc := familyFuncs[x.Name.Name]; gc != nil {
						owner = gc
					}
				case *ast.GenDecl:
					for _, sp := range x.Specs {
						if ts, ok := sp.(*ast.TypeSpec); ok && family[ts.Name.Name] != nil && ts.Pos() <= te.Pos && te.Pos <= ts.End() {
							owner = family[ts.Name.Name]
						}
					}
				}
			}
		}
		if owner == nil {
			continue
		}
		for owner.outer != nil {
			owner = owner.outer
		}
		if !seen[owner] {
			seen[owner] = true
			out = append(out, owner)
		}
	}
	return out
}

// ---- 1. uses

type site struct {
	node  ast.Node // the instantiation (Index(List)Expr) or the function ident
	name  string   // class or function
	n     int
	call  bool // a call without explicit instantiation (node is the ident)
	pos   string
	fixed bool
}

func collectSites() []*site {
	var out []*site
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			switch v := x.(type) {
			case *ast.IndexExpr, *ast.IndexListExpr:
				var xid *ast.Ident
				var idx []ast.Expr
				if ie, ok := v.(*ast.IndexExpr); ok {
					xid, _ = ie.X.(*ast.Ident)
					idx = []ast.Expr{ie.Index}
				} else {
					il := v.(*ast.IndexListExpr)
					xid, _ = il.X.(*ast.Ident)
					idx = il.Indices
				}
				if xid == nil {
					return true
				}
				allAny := true
				for _, a := range idx {
					if id, ok := a.(*ast.Ident); !ok || id.Name != "any" {
						allAny = false
					}
				}
				if !allAny || inBody(v) {
					return true
				}
				switch o := rw.Info.Uses[xid].(type) {
				case *types.TypeName:
					if family[o.Name()] != nil {
						out = append(out, &site{node: v, name: o.Name(), n: len(idx)})
						return false
					}
				case *types.Func:
					if familyFuncs[o.Name()] != nil {
						out = append(out, &site{node: v, name: o.Name(), n: len(idx)})
						return false
					}
				}
				return true
			case *ast.CallExpr:
				id, ok := rw.Unparen(v.Fun).(*ast.Ident)
				if !ok || inBody(v) {
					return true
				}
				fo, ok := rw.Info.Uses[id].(*types.Func)
				if !ok || familyFuncs[fo.Name()] == nil {
					return true
				}
				tps := fo.Type().(*types.Signature).TypeParams()
				if tps == nil {
					return true
				}
				out = append(out, &site{node: id, name: id.Name, n: tps.Len(), call: true})
			}
			return true
		})
	}
	return out
}

func siteText(s *site, args []string) string {
	return s.name + "[" + strings.Join(args, ", ") + "]"
}

// ---- probing

type ev struct {
	exact   map[string]types.Type
	writes  map[string]types.Type
	untyped bool
	bad     string
}

var (
	ufParent = map[string]string{}
	evs      = map[string]*ev{}
)

func ufFind(k string) string {
	if p, ok := ufParent[k]; ok && p != k {
		r := ufFind(p)
		ufParent[k] = r
		return r
	}
	ufParent[k] = k
	return k
}

func ufUnion(a, b string) {
	ra, rb := ufFind(a), ufFind(b)
	if ra != rb {
		if ra < rb {
			ufParent[rb] = ra
		} else {
			ufParent[ra] = rb
		}
	}
}

func evOf(k string) *ev {
	if e := evs[k]; e != nil {
		return e
	}
	e := &ev{exact: map[string]types.Type{}, writes: map[string]types.Type{}}
	evs[k] = e
	return e
}

// forceAny are the classes whose bodies box or cast values of their own (or another family
// class's) instantiation over type variables: any instance may meet an Object, so all stay
// erased.
var forceAny = map[string]bool{}

var reTag = regexp.MustCompile(`^pr_(\d+)_(\d+)$`)

func tagOf(t types.Type) string {
	n, ok := types.Unalias(t).(*types.Named)
	if !ok {
		return ""
	}
	if m := reTag.FindStringSubmatch(n.Obj().Name()); m != nil {
		return "s:" + m[1] + ":" + m[2]
	}
	return ""
}

func exactEv(tag string, t types.Type, why string) {
	t = types.Unalias(t)
	if hasTag(t) {
		evOf(tag).bad = "its type argument is another erased instantiation"
		return
	}
	if isTypeParam(t) {
		evOf(tag).bad = "meets a type variable of a class body"
		return
	}
	evOf(tag).exact[tstr(t)] = t
}

func writeEv(tag string, h types.Type, e ast.Expr) {
	h = types.Unalias(h)
	if b, ok := h.(*types.Basic); ok && b.Kind() == types.UntypedNil {
		return
	}
	if isTypeParam(h) {
		evOf(tag).bad = "meets a type variable of a class body"
		return
	}
	if isAny(h) {
		if e != nil {
			if in := boxInner(e); in != nil {
				if t := rw.Info.Types[in].Type; t != nil && !isAny(t) {
					writeEv(tag, t, nil)
					return
				}
			}
		}
		evOf(tag).untyped = true
		return
	}
	if hasTag(h) {
		evOf(tag).bad = "its type argument is another erased instantiation"
		return
	}
	evOf(tag).writes[tstr(h)] = h
}

// match unifies the placeholders where a value of type h meets a slot of type w.
func match(h, w types.Type, e ast.Expr, top bool) {
	if h == nil || w == nil {
		return
	}
	h, w = types.Unalias(h), types.Unalias(w)
	if types.Identical(h, w) {
		return
	}
	th, tw := tagOf(h), tagOf(w)
	switch {
	case th != "" && tw != "":
		ufUnion(th, tw)
		return
	case tw != "":
		if top {
			writeEv(tw, h, e)
		} else {
			exactEv(tw, h, "type argument")
		}
		return
	case th != "":
		if !top || !isIface(w) {
			exactEv(th, w, "type argument")
		}
		return
	}
	switch hx := h.(type) {
	case *types.Pointer:
		if wx, ok := w.(*types.Pointer); ok {
			match(hx.Elem(), wx.Elem(), nil, false)
		}
	case *types.Slice:
		if wx, ok := w.(*types.Slice); ok {
			match(hx.Elem(), wx.Elem(), nil, false)
		}
	case *types.Array:
		if wx, ok := w.(*types.Array); ok {
			match(hx.Elem(), wx.Elem(), nil, false)
		}
	case *types.Map:
		if wx, ok := w.(*types.Map); ok {
			match(hx.Key(), wx.Key(), nil, false)
			match(hx.Elem(), wx.Elem(), nil, false)
		}
	case *types.Chan:
		if wx, ok := w.(*types.Chan); ok {
			match(hx.Elem(), wx.Elem(), nil, false)
		}
	case *types.Signature:
		if wx, ok := w.(*types.Signature); ok && hx.Params().Len() == wx.Params().Len() && hx.Results().Len() == wx.Results().Len() {
			for i := 0; i < hx.Params().Len(); i++ {
				match(hx.Params().At(i).Type(), wx.Params().At(i).Type(), nil, false)
			}
			for i := 0; i < hx.Results().Len(); i++ {
				match(hx.Results().At(i).Type(), wx.Results().At(i).Type(), nil, false)
			}
		}
	case *types.Named:
		if wx, ok := w.(*types.Named); ok && hx.Origin() == wx.Origin() && hx.TypeArgs().Len() == wx.TypeArgs().Len() {
			for i := 0; i < hx.TypeArgs().Len(); i++ {
				match(hx.TypeArgs().At(i), wx.TypeArgs().At(i), nil, false)
			}
		}
	}
}

// worldOf unites the placeholders of the family instantiations in t with their class's
// run-time instantiation: t's values are boxed into an Object or cast from one.
func worldOf(t types.Type) {
	seen := map[types.Type]bool{}
	var walk func(t types.Type)
	walk = func(t types.Type) {
		t = types.Unalias(t)
		if t == nil || seen[t] {
			return
		}
		seen[t] = true
		switch x := t.(type) {
		case *types.Pointer:
			walk(x.Elem())
		case *types.Slice:
			walk(x.Elem())
		case *types.Array:
			walk(x.Elem())
		case *types.Map:
			walk(x.Key())
			walk(x.Elem())
		case *types.Named:
			if x.TypeArgs().Len() > 0 {
				if gc := family[x.Origin().Obj().Name()]; gc != nil {
					for i := 0; i < x.TypeArgs().Len(); i++ {
						a := x.TypeArgs().At(i)
						w := "w:" + gc.name + ":" + strconv.Itoa(i)
						if isTypeParam(a) {
							// a body boxes or casts instances of any instantiation
							forceAny[gc.name] = true
						}
						if tg := tagOf(a); tg != "" {
							ufUnion(tg, w)
						} else {
							exactEv(w, a, "boxed")
						}
					}
				}
				for i := 0; i < x.TypeArgs().Len(); i++ {
					walk(x.TypeArgs().At(i))
				}
			}
		}
	}
	walk(t)
}

func hasTag(t types.Type) bool {
	found := false
	var walk func(t types.Type)
	walk = func(t types.Type) {
		t = types.Unalias(t)
		if found || t == nil {
			return
		}
		if tagOf(t) != "" {
			found = true
			return
		}
		switch x := t.(type) {
		case *types.Pointer:
			walk(x.Elem())
		case *types.Slice:
			walk(x.Elem())
		case *types.Map:
			walk(x.Key())
			walk(x.Elem())
		case *types.Named:
			for i := 0; i < x.TypeArgs().Len(); i++ {
				walk(x.TypeArgs().At(i))
			}
		case *types.Signature:
			for i := 0; i < x.Params().Len(); i++ {
				walk(x.Params().At(i).Type())
			}
			for i := 0; i < x.Results().Len(); i++ {
				walk(x.Results().At(i).Type())
			}
		}
	}
	walk(t)
	return found
}

var unboxNames = regexp.MustCompile(`^(str_|Node_of_|cast_\[.*\]|to_\w+|icast_\w+|unbox_\[.*\]|unboxT_\[.*\])$`)

var reTagArgs = regexp.MustCompile(`\[pr_\d+_\d+(, pr_\d+_\d+)*\]`)

// overrides unifies the placeholders of methods of one name and erased signature (a method and
// its _impl, interface methods and their implementations, the named interfaces of
// dispatchers): their signatures must stay identical for the classes to implement them.
func overrides() {
	groups := map[string][]*types.Signature{}
	var keys []string
	add := func(name string, sig *types.Signature) {
		if sig == nil || !hasTag(sig) {
			return
		}
		var ts []string
		for _, tu := range []*types.Tuple{sig.Params(), sig.Results()} {
			for i := 0; i < tu.Len(); i++ {
				ts = append(ts, tstr(tu.At(i).Type()))
			}
			ts = append(ts, "|")
		}
		k := strings.TrimSuffix(name, "_impl") + "/" + reTagArgs.ReplaceAllString(strings.Join(ts, ","), "[]")
		if groups[k] == nil {
			keys = append(keys, k)
		}
		groups[k] = append(groups[k], sig)
	}
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			switch v := x.(type) {
			case *ast.FuncDecl:
				if v.Recv != nil {
					if o, ok := rw.Info.Defs[v.Name].(*types.Func); ok {
						add(v.Name.Name, o.Type().(*types.Signature))
					}
				}
			case *ast.InterfaceType:
				for _, m := range v.Methods.List {
					if len(m.Names) == 1 {
						if o, ok := rw.Info.Defs[m.Names[0]].(*types.Func); ok {
							add(m.Names[0].Name, o.Type().(*types.Signature))
						}
					}
				}
			}
			return true
		})
	}
	for _, k := range keys {
		g := groups[k]
		for _, sig := range g[1:] {
			stripped := func(s *types.Signature) *types.Signature {
				return types.NewSignatureType(nil, nil, nil, s.Params(), s.Results(), s.Variadic())
			}
			match(stripped(sig), stripped(g[0]), nil, false)
		}
	}
}

func gather() {
	overrides()
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			switch v := x.(type) {
			case *ast.TypeAssertExpr:
				if v.Type != nil {
					if t := rw.Info.Types[v.Type].Type; t != nil && hasFamilyInst(t) {
						worldOf(t)
					}
					if tg := tagOf(rw.Info.Types[v.X].Type); tg != "" {
						if t := rw.Info.Types[v.Type].Type; t != nil {
							exactEv(tg, t, "assertion")
						}
					}
				}
			case *ast.CaseClause:
				if ts, ok := rw.Parent[rw.Parent[v]].(*ast.TypeSwitchStmt); ok && ts != nil {
					for _, c := range v.List {
						if t := rw.Info.Types[c].Type; t != nil && hasFamilyInst(t) {
							worldOf(t)
						}
					}
				}
			case *ast.IndexExpr, *ast.IndexListExpr:
				// an instantiation of a runtime generic function (cast_, is_, unbox_...)
				var xe ast.Expr
				var idx []ast.Expr
				if ie, ok := v.(*ast.IndexExpr); ok {
					xe, idx = ie.X, []ast.Expr{ie.Index}
				} else {
					il := v.(*ast.IndexListExpr)
					xe, idx = il.X, il.Indices
				}
				if id, ok := xe.(*ast.Ident); ok {
					if fo, ok := rw.Info.Uses[id].(*types.Func); ok && familyFuncs[fo.Name()] == nil {
						for _, a := range idx {
							if t := rw.Info.Types[a].Type; t != nil && hasFamilyInst(t) {
								worldOf(t)
							}
						}
					}
				}
			case *ast.CallExpr:
				// reflipSvar: x.m_s(s, ...) is x.m(strobj_(s), ...): a string written to m's first slot
				if sel, ok := v.Fun.(*ast.SelectorExpr); ok && strings.HasSuffix(sel.Sel.Name, "_s") && len(v.Args) > 0 {
					if sn := rw.Info.Selections[sel]; sn != nil && sn.Kind() == types.MethodVal {
						if at := rw.Info.Types[v.Args[0]].Type; at != nil && types.Identical(types.Unalias(at), types.Typ[types.String]) {
							if mo, _, _ := types.LookupFieldOrMethod(sn.Recv(), true, rw.Pkg, strings.TrimSuffix(sel.Sel.Name, "_s")); mo != nil {
								if mf, ok := mo.(*types.Func); ok {
									if ms := mf.Type().(*types.Signature); ms.Params().Len() > 0 {
										if tg := tagOf(ms.Params().At(0).Type()); tg != "" {
											writeEv(tg, types.Typ[types.String], nil)
										}
									}
								}
							}
						}
					}
				}
				if len(v.Args) == 1 {
					if name, _ := rw.CallName(v); name != "" && unboxNames.MatchString(name) {
						if tg := tagOf(rw.Info.Types[v.Args[0]].Type); tg != "" {
							if t := rw.Info.Types[v].Type; t != nil {
								exactEv(tg, t, "cast")
							}
						}
					}
				}
			}
			e, ok := x.(ast.Expr)
			if !ok {
				return true
			}
			tv, ok := rw.Info.Types[e]
			if !ok || tv.IsType() || tv.Type == nil {
				return true
			}
			h := tv.Type
			w := ctxType(e)
			if hasTag(h) || w != nil && hasTag(w) {
				match(h, w, e, true)
			}
			if hasFamilyInst(h) && !isIface(h) {
				// boxed into an Object
				{
					if w != nil && isIface(w) {
						worldOf(h)
					}
					if p, ok := rw.Up(e).(*ast.CallExpr); ok && len(p.Args) == 1 {
						if name, _ := rw.CallName(p); name == "any" || strings.HasPrefix(name, "obj_") {
							worldOf(h)
						}
					}
					if s, ok := rw.Up(e).(*ast.SelectorExpr); ok && s.X == e && s.Sel.Name == "self" {
						worldOf(h)
					}
				}
			}
			return true
		})
	}
}

// allowed reports whether a type argument t keeps Java's null distinct (classes, strings,
// handles, interfaces, integer enums with 0 for null).
func allowed(t types.Type) bool {
	t = types.Unalias(t)
	switch x := t.(type) {
	case *types.Pointer:
		n, ok := types.Unalias(x.Elem()).(*types.Named)
		if !ok {
			return false
		}
		_, ok = n.Underlying().(*types.Struct)
		return ok
	case *types.Basic:
		return x.Kind() == types.String
	case *types.Named:
		if _, ok := x.Underlying().(*types.Interface); ok {
			return true
		}
		if _, ok := x.Underlying().(*types.Basic); ok {
			return rw.HasMethod(x, "box_") || rw.HasMethod(x, "ordinal")
		}
	}
	return false
}

func assignable(w, x types.Type) bool {
	if types.Identical(w, x) {
		return true
	}
	if isIface(x) {
		return types.AssignableTo(w, x)
	}
	if xp, ok := x.(*types.Pointer); ok {
		if n, ok := xp.Elem().(*types.Named); ok {
			return rw.HasMethod(w, "as_"+n.Obj().Name()) && rw.HasFunc("to_"+n.Obj().Name())
		}
	}
	return false
}

// decide is the type argument of a class of placeholders, "any" when erased.
func decide(root string, members []string) (string, string) {
	exact := map[string]types.Type{}
	writes := map[string]types.Type{}
	untyped := false
	for _, m := range members {
		e := evs[m]
		if e == nil {
			continue
		}
		if e.bad != "" {
			return "any", e.bad
		}
		for k, t := range e.exact {
			exact[k] = t
		}
		for k, t := range e.writes {
			writes[k] = t
		}
		untyped = untyped || e.untyped
	}
	var x types.Type
	var xs string
	switch {
	case len(exact) > 1:
		return "any", "conflicting type arguments"
	case len(exact) == 1:
		for k, t := range exact {
			xs, x = k, t
		}
		if isAny(x) {
			return "any", "an erased slot (Object, a wildcard or a raw type)"
		}
		for _, w := range writes {
			if !assignable(w, x) {
				return "any", "a value of another type is stored"
			}
		}
	case len(writes) == 1 && !untyped:
		for k, t := range writes {
			xs, x = k, t
		}
	case len(writes) == 0 && !untyped:
		return "any", "nothing typed is stored or read"
	default:
		return "any", "Objects of unknown type are stored"
	}
	if !allowed(x) {
		return "any", "a type argument without a null of its own (" + xs + ")"
	}
	return xs, ""
}

func renderAll() map[string][]byte {
	out := map[string][]byte{}
	for _, f := range rw.Files {
		tf := rw.Fset.File(f.Pos())
		s := string(rw.Src[tf][:tf.Offset(f.Pos())]) + rw.RenderChildren(f) + string(rw.Src[tf][tf.Offset(f.End()):])
		if s != string(rw.Src[tf]) {
			out[tf.Name()] = []byte(s)
		}
	}
	return out
}

func registerSites(sites []*site, args func(i int, s *site) []string) {
	for i, s := range sites {
		a := args(i, s)
		if a == nil {
			continue
		}
		text := siteText(s, a)
		rw.Rewrite[s.node] = func() string { return text }
	}
}

func classUses(dir string, report bool) {
	load(dir)
	sites := collectSites()
	for _, st := range sites {
		p := rw.Fset.Position(st.node.Pos())
		st.pos = fmt.Sprintf("%s:%d", filepath.Base(p.Filename), p.Line)
	}
	fmt.Fprintf(os.Stderr, "uses: %d sites\n", len(sites))
	if len(sites) == 0 {
		return
	}
	registerSites(sites, func(i int, s *site) []string {
		var a []string
		for k := 0; k < s.n; k++ {
			a = append(a, fmt.Sprintf("pr_%d_%d", i, k))
		}
		return a
	})
	mem := renderAll()
	var b strings.Builder
	b.WriteString("package closure\n\n")
	for i, s := range sites {
		for k := 0; k < s.n; k++ {
			fmt.Fprintf(&b, "type pr_%d_%d struct{}\n", i, k)
		}
	}
	probe := filepath.Join(dir, "zz_probe_generics.go")
	mem[probe] = []byte(b.String())
	errs := loadWith(dir, mem)
	fmt.Fprintf(os.Stderr, "probe: %d type errors\n", len(errs))
	gather()
	// a generic call whose inference fails records no signature: its arguments meet through
	// the error message, which prints both types
	reTagName := regexp.MustCompile(`pr_(\d+)_(\d+)`)
	for _, te := range errs {
		if m := reInferred.FindStringSubmatch(te.Msg); m != nil {
			a := reTagName.FindAllStringSubmatch(m[2], -1)
			b := reTagName.FindAllStringSubmatch(m[4], -1)
			if len(a) == len(b) {
				for i := range a {
					ufUnion("s:"+a[i][1]+":"+a[i][2], "s:"+b[i][1]+":"+b[i][2])
				}
			}
		}
	}
	// classes of placeholders
	groups := map[string][]string{}
	for i, s := range sites {
		for k := 0; k < s.n; k++ {
			key := fmt.Sprintf("s:%d:%d", i, k)
			r := ufFind(key)
			groups[r] = append(groups[r], key)
		}
	}
	for k := range evs {
		r := ufFind(k)
		if strings.HasPrefix(k, "w:") {
			groups[r] = append(groups[r], k)
		}
	}
	decided := map[string]string{}
	whyAny := map[string]int{}
	roots := make([]string, 0, len(groups))
	for r := range groups {
		roots = append(roots, r)
	}
	sort.Strings(roots)
	for _, r := range roots {
		ms := groups[r]
		sort.Strings(ms)
		ms = uniq(ms)
		d, why := decide(r, ms)
		decided[r] = d
		nsite := 0
		for _, m := range ms {
			if strings.HasPrefix(m, "s:") {
				nsite++
			}
		}
		if why != "" {
			whyAny[why] += nsite
		} else {
			stats["1: type arguments decided (site slots)"] += nsite
		}
		if dbg := os.Getenv("GENERICS_DEBUG"); dbg != "" {
			var where []string
			for _, m := range ms {
				if strings.HasPrefix(m, "s:") {
					var i int
					fmt.Sscanf(strings.Split(m, ":")[1], "%d", &i)
					where = append(where, sites[i].pos)
				} else {
					where = append(where, m)
				}
			}
			if strings.Contains(strings.Join(where, " "), dbg) {
				fmt.Fprintf(os.Stderr, "group %s (%d): %s %s %v\n", r, len(ms), d, why, where)
				for _, m := range ms {
					if e := evs[m]; e != nil {
						fmt.Fprintf(os.Stderr, "   %s exact=%v writes=%v untyped=%v bad=%s\n", m, keysOf(e.exact), keysOf(e.writes), e.untyped, e.bad)
					}
				}
			}
		}
	}
	for w, n := range whyAny {
		stats["1: site slots left erased: "+w] += n
	}
	// final rendering on the saved package
	load(dir)
	sites2 := collectSites()
	if len(sites2) != len(sites) {
		panic("sites differ between loads")
	}
	typedSites := 0
	for c := range forceAny {
		stats["1: classes left erased at every site: a class body boxes or casts its own instances ("+c+")"]++
	}
	registerSites(sites2, func(i int, s *site) []string {
		cls := s.name
		if gc := familyFuncs[cls]; gc != nil {
			cls = gc.name
		}
		if forceAny[cls] {
			return nil
		}
		var a []string
		allAny := true
		for k := 0; k < s.n; k++ {
			d := decided[ufFind(fmt.Sprintf("s:%d:%d", i, k))]
			if d == "" {
				d = "any"
			}
			if d != "any" {
				allAny = false
			}
			a = append(a, d)
		}
		if allAny {
			// a call Go would instantiate otherwise from its arguments is written erased
			if id, ok := s.node.(*ast.Ident); ok && s.call {
				if inst, ok := rw.Info.Instances[id]; ok {
					for k := 0; k < inst.TypeArgs.Len(); k++ {
						if !isAny(inst.TypeArgs.At(k)) {
							stats["1: calls written erased where Go would infer a typed instantiation"]++
							return a
						}
					}
				}
			}
			return nil
		}
		typedSites++
		return a
	})
	stats["1: sites typed"] = typedSites
	stats["1: sites left erased"] = len(sites) - typedSites
	if !report {
		rw.Save()
	}
}

func uniq(xs []string) []string {
	var out []string
	for i, x := range xs {
		if i == 0 || x != xs[i-1] {
			out = append(out, x)
		}
	}
	return out
}

func classesMain(args []string) {
	report := false
	var only map[string]bool
	dir := args[len(args)-1]
	for i := 0; i < len(args)-1; i++ {
		switch args[i] {
		case "-n":
			report = true
		case "-only":
			only = map[string]bool{}
			for _, c := range strings.Split(args[i+1], ",") {
				only[c] = true
			}
			i++
		}
	}
	javaRoot = filepath.Join(dir, "..", "..", "java", "src")
	load(dir)
	orig := map[string][]byte{}
	for tf, b := range rw.Src {
		orig[tf.Name()] = b
	}
	tests, _ := filepath.Glob(filepath.Join(dir, "rt_*_test.go"))
	for _, t := range tests {
		b, _ := os.ReadFile(t)
		orig[t] = b
	}
	excluded := map[string]string{}
	restore := func() {
		for n, b := range orig {
			if cur, err := os.ReadFile(n); err != nil || string(cur) != string(b) {
				if err := os.WriteFile(n, b, 0o644); err != nil {
					panic(err)
				}
			}
		}
	}
	for attempt := 0; ; attempt++ {
		if attempt > 0 {
			restore()
			load(dir)
			family, byFQN, leftClasses, familyFuncs = map[string]*gclass{}, map[string]*gclass{}, map[string]string{}, map[string]*gclass{}
			forceAny = map[string]bool{}
			stats = map[string]int{}
		}
		discover(dir, only)
		for fqn, why := range excluded {
			for g, gc := range family {
				root := gc
				for root.outer != nil {
					root = root.outer
				}
				if root.fqn == fqn {
					delete(family, g)
					delete(byFQN, gc.fqn)
				}
			}
			leftClasses[fqn] = why
		}
		if len(family) == 0 {
			break
		}
		declare()
		if report {
			break
		}
		rw.Save()
		eraseInTests(dir)
		fmt.Fprintln(os.Stderr, "class bodies:")
		left := fixpoint(dir, report)
		if left == nil {
			load(dir)
			if hoistBoxes() > 0 {
				rw.Save()
				left = fixpoint(dir, report)
			}
		}
		if left == nil {
			break
		}
		bad := owners(left)
		if len(bad) == 0 {
			for _, e := range left {
				fmt.Fprintln(os.Stderr, "error outside the class bodies:", rw.Fset.Position(e.Pos), e.Msg)
			}
			os.Exit(1)
		}
		for _, gc := range bad {
			msg := left[0].Msg
			for _, e := range left {
				if o := owners([]types.Error{e}); len(o) == 1 && o[0] == gc {
					msg = e.Msg
					break
				}
			}
			excluded[gc.fqn] = "its body does not type-check with type parameters: " + msg
			fmt.Fprintf(os.Stderr, "excluding %s: %s\n", gc.name, msg)
		}
	}
	names := make([]string, 0, len(family))
	for g := range family {
		names = append(names, g)
	}
	sort.Strings(names)
	fmt.Fprintf(os.Stderr, "classes made generic: %s\n", strings.Join(names, " "))
	if len(family) > 0 && !report {
		classUses(dir, report)
		fmt.Fprintln(os.Stderr, "uses:")
		if left := fixpoint(dir, report); left != nil {
			for _, e := range left {
				fmt.Fprintln(os.Stderr, "error in the uses:", rw.Fset.Position(e.Pos), e.Msg)
			}
			os.Exit(1)
		}
		load(dir)
		if bad := checkBoxedInstances(); len(bad) > 0 {
			for _, b := range bad {
				fmt.Fprintln(os.Stderr, "boxed and cast at different instantiations:", b)
			}
			os.Exit(1)
		}
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%6d %s\n", stats[k], k)
	}
	left := make([]string, 0, len(leftClasses))
	for k := range leftClasses {
		left = append(left, k)
	}
	sort.Strings(left)
	for _, k := range left {
		fmt.Printf("  left: %s: %s\n", k, leftClasses[k])
	}
}

// eraseInTests writes the family classes used in the runtime's tests (not type-checked with
// the package) as the erased class C[any].
func eraseInTests(dir string) {
	names, _ := filepath.Glob(filepath.Join(dir, "rt_*_test.go"))
	for _, n := range names {
		b, err := os.ReadFile(n)
		if err != nil {
			panic(err)
		}
		s := string(b)
		for g, gc := range family {
			rx := regexp.MustCompile(`\*` + g + `\b(\[)?`)
			s = rx.ReplaceAllStringFunc(s, func(m string) string {
				if strings.HasSuffix(m, "[") {
					return m
				}
				return m + "[" + typeArgsText(len(gc.tparams), "any") + "]"
			})
		}
		if s != string(b) {
			if err := os.WriteFile(n, []byte(s), 0o644); err != nil {
				panic(err)
			}
		}
	}
}

func isRuntime(n ast.Node) bool {
	b := filepath.Base(rw.Fset.File(n.Pos()).Name())
	return strings.HasPrefix(b, "rt_") && !strings.HasSuffix(b, "_test.go")
}

func hasAnyArg(n *types.Named) bool {
	for i := 0; i < n.TypeArgs().Len(); i++ {
		if isAny(n.TypeArgs().At(i)) {
			return true
		}
	}
	return false
}

// retypeWantSlot gives the local that e initializes or is assigned to the type have.
func retypeWantSlot(e ast.Expr, have types.Type) bool {
	switch p := rw.Up(e).(type) {
	case *ast.ValueSpec:
		if len(p.Names) == 1 && len(p.Values) == 1 && p.Type != nil && inBody(p) {
			if _, done := rw.Rewrite[p.Type]; done {
				return true
			}
			h := tstr(have)
			rw.Rewrite[p.Type] = func() string { return h }
			stats["repair: locals typed from their value"]++
			return true
		}
	case *ast.AssignStmt:
		if len(p.Lhs) == 1 {
			switch l := p.Lhs[0].(type) {
			case *ast.Ident:
				return retypeDecl(l, have)
			case *ast.SelectorExpr:
				return retypeDecl(l.Sel, have)
			}
		}
	}
	return false
}

// reinstantiate makes producer call (a constructor or factory, or a method
// returning its receiver's type called on such a producer) produce a want.
func reinstantiate(call *ast.CallExpr, want types.Type) bool {
	if instantiateFor(call, want) {
		return true
	}
	// A cast (cast_[*C[...]]) is not re-instantiated: a boxed object keeps the instantiation it
	// was made with.
	if sel, ok := call.Fun.(*ast.SelectorExpr); ok {
		if inner, ok := rw.Unparen(sel.X).(*ast.CallExpr); ok {
			if types.Identical(rw.Info.Types[inner].Type, rw.Info.Types[call].Type) {
				return reinstantiate(inner, want)
			}
		}
	}
	return false
}

// retypeField gives a field of a family class that has no Java declaration of its own (an
// anonymous class's captured variables) the type want.
func retypeField(v *types.Var, want types.Type) bool {
	for _, f := range rw.Files {
		if rw.Fset.File(f.Pos()) != rw.Fset.File(v.Pos()) {
			continue
		}
		var te ast.Expr
		var owner string
		ast.Inspect(f, func(x ast.Node) bool {
			if ts, ok := x.(*ast.TypeSpec); ok {
				owner = ts.Name.Name
			}
			if fl, ok := x.(*ast.Field); ok && len(fl.Names) == 1 && fl.Names[0].Pos() == v.Pos() {
				te = fl.Type
			}
			return te == nil
		})
		gc := family[owner]
		if te == nil || gc == nil || gc.j != nil {
			return false
		}
		if _, done := rw.Rewrite[te]; done {
			return true
		}
		w := tstr(want)
		rw.Rewrite[te] = func() string { return w }
		stats["repair: fields of anonymous classes typed from their values"]++
		return true
	}
	return false
}

// instantiateFromArgs instantiates a generic call explicitly from the arguments whose types
// are family classes over no erased type argument (the others are then repaired one by one).
func instantiateFromArgs(call *ast.CallExpr) bool {
	f, id, idx := funcObj(call.Fun)
	if f == nil || idx != nil {
		return false
	}
	sig := f.Origin().Type().(*types.Signature)
	tps := sig.TypeParams()
	args := make([]string, tps.Len())
	for i, a := range call.Args {
		if i >= sig.Params().Len() {
			break
		}
		at := rw.Info.Types[a].Type
		pn, an := sameOrigin(sig.Params().At(i).Type(), at)
		if pn == nil || hasAnyArg(an) {
			continue
		}
		for k := 0; k < pn.TypeArgs().Len(); k++ {
			if tp, ok := pn.TypeArgs().At(k).(*types.TypeParam); ok {
				for j := 0; j < tps.Len(); j++ {
					if tps.At(j) == tp && args[j] == "" {
						args[j] = tstr(an.TypeArgs().At(k))
					}
				}
			}
		}
	}
	for _, a := range args {
		if a == "" {
			return false
		}
	}
	name := id.Name
	text := name + "[" + strings.Join(args, ", ") + "]"
	rw.Rewrite[id] = func() string { return text }
	return true
}

func keysOf(m map[string]types.Type) []string {
	var out []string
	for k := range m {
		out = append(out, k)
	}
	sort.Strings(out)
	return out
}

// hasFamilyInst reports whether t mentions an instantiation of a family class.
func hasFamilyInst(t types.Type) bool {
	found := false
	var walk func(t types.Type, depth int)
	walk = func(t types.Type, depth int) {
		t = types.Unalias(t)
		if found || t == nil || depth > 8 {
			return
		}
		switch x := t.(type) {
		case *types.Pointer:
			walk(x.Elem(), depth+1)
		case *types.Slice:
			walk(x.Elem(), depth+1)
		case *types.Map:
			walk(x.Key(), depth+1)
			walk(x.Elem(), depth+1)
		case *types.Named:
			if x.TypeArgs().Len() > 0 && family[x.Origin().Obj().Name()] != nil {
				found = true
				return
			}
			for i := 0; i < x.TypeArgs().Len(); i++ {
				walk(x.TypeArgs().At(i), depth+1)
			}
		}
	}
	walk(t, 0)
	return found
}

// checkBoxedInstances verifies, on the final package, that every family class whose values are
// cast from an Object is boxed and cast at one instantiation only (an instantiation over a
// body's type variables counts as unknown): a type assertion must see the instantiation the
// object was made with. It returns the classes that break this.
func checkBoxedInstances() []string {
	boxed := map[string]map[string]string{} // class -> instantiation -> a position
	cast := map[string]map[string]string{}
	add := func(m map[string]map[string]string, t types.Type, pos token.Pos) {
		var walk func(t types.Type, depth int)
		walk = func(t types.Type, depth int) {
			t = types.Unalias(t)
			if t == nil || depth > 8 {
				return
			}
			switch x := t.(type) {
			case *types.Pointer:
				walk(x.Elem(), depth+1)
			case *types.Slice:
				walk(x.Elem(), depth+1)
			case *types.Map:
				walk(x.Key(), depth+1)
				walk(x.Elem(), depth+1)
			case *types.Named:
				if x.TypeArgs().Len() > 0 {
					if gc := family[x.Origin().Obj().Name()]; gc != nil {
						k := argsText(x)
						for i := 0; i < x.TypeArgs().Len(); i++ {
							if isTypeParam(x.TypeArgs().At(i)) {
								k = "(type variables)"
							}
						}
						if m[gc.name] == nil {
							m[gc.name] = map[string]string{}
						}
						if m[gc.name][k] == "" {
							m[gc.name][k] = rw.Fset.Position(pos).String()
						}
					}
					for i := 0; i < x.TypeArgs().Len(); i++ {
						walk(x.TypeArgs().At(i), depth+1)
					}
				}
			}
		}
		walk(t, 0)
	}
	for _, f := range rw.Files {
		if !generated(f) && !isRuntime(f) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			switch v := x.(type) {
			case *ast.TypeAssertExpr:
				if v.Type != nil {
					add(cast, rw.Info.Types[v.Type].Type, v.Pos())
				}
			case *ast.CaseClause:
				if _, ok := rw.Parent[rw.Parent[v]].(*ast.TypeSwitchStmt); ok {
					for _, c := range v.List {
						add(cast, rw.Info.Types[c].Type, c.Pos())
					}
				}
			case *ast.IndexExpr, *ast.IndexListExpr:
				var xe ast.Expr
				var idx []ast.Expr
				if ie, ok := v.(*ast.IndexExpr); ok {
					xe, idx = ie.X, []ast.Expr{ie.Index}
				} else {
					il := v.(*ast.IndexListExpr)
					xe, idx = il.X, il.Indices
				}
				if id, ok := xe.(*ast.Ident); ok && (id.Name == "cast_" || id.Name == "is_" || id.Name == "unboxT_") {
					for _, a := range idx {
						add(cast, rw.Info.Types[a].Type, a.Pos())
					}
				}
			}
			e, ok := x.(ast.Expr)
			if !ok {
				return true
			}
			tv, ok := rw.Info.Types[e]
			if !ok || tv.IsType() || tv.Type == nil || isIface(tv.Type) || !hasFamilyInst(tv.Type) {
				return true
			}
			isBox := false
			if w := ctxType(e); w != nil && isIface(w) {
				isBox = true
			}
			if p, ok := rw.Up(e).(*ast.CallExpr); ok && len(p.Args) == 1 {
				if name, _ := rw.CallName(p); name == "any" || strings.HasPrefix(name, "obj_") || name == "boxNull_" {
					isBox = true
				}
			}
			if s, ok := rw.Up(e).(*ast.SelectorExpr); ok && s.X == e && s.Sel.Name == "self" {
				isBox = true
			}
			if isBox {
				add(boxed, tv.Type, e.Pos())
			}
			return true
		})
	}
	var bad []string
	for c, cs := range cast {
		for ci, cpos := range cs {
			for bi, bpos := range boxed[c] {
				if ci != bi || ci == "(type variables)" {
					bad = append(bad, fmt.Sprintf("%s: cast at %s as [%s], boxed at %s as [%s]", c, cpos, ci, bpos, bi))
				}
			}
		}
	}
	sort.Strings(bad)
	return bad
}

// hoistBoxes boxes a type-variable parameter once per call where a body boxes it more than once
// (`keyBox_ := boxNull_(key)` first): the erased code had one box, the caller's, for all its
// uses. boxNull_ has no side effects and the parameter is never assigned, so the box is the
// same value at every use.
func hoistBoxes() int {
	n := 0
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Body == nil || len(fd.Body.List) == 0 || !inBody(fd) {
				continue
			}
			boxes := map[*types.Var][]*ast.CallExpr{}
			assigned := map[*types.Var]bool{}
			names := map[string]bool{}
			ast.Inspect(fd.Body, func(x ast.Node) bool {
				switch v := x.(type) {
				case *ast.Ident:
					names[v.Name] = true
				case *ast.AssignStmt:
					for _, l := range v.Lhs {
						if id, ok := l.(*ast.Ident); ok {
							if o, ok := rw.Info.Uses[id].(*types.Var); ok {
								assigned[o] = true
							}
						}
					}
				case *ast.UnaryExpr:
					if id, ok := v.X.(*ast.Ident); ok && v.Op == token.AND {
						if o, ok := rw.Info.Uses[id].(*types.Var); ok {
							assigned[o] = true
						}
					}
				case *ast.CallExpr:
					if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "boxNull_" && len(v.Args) == 1 {
						if a, ok := v.Args[0].(*ast.Ident); ok {
							if o, ok := rw.Info.Uses[a].(*types.Var); ok && isTypeParam(o.Type()) {
								boxes[o] = append(boxes[o], v)
							}
						}
					}
				}
				return true
			})
			params := map[*types.Var]bool{}
			for _, fl := range fd.Type.Params.List {
				for _, nm := range fl.Names {
					if o, ok := rw.Info.Defs[nm].(*types.Var); ok {
						params[o] = true
					}
				}
			}
			var hoisted []string
			var vars []*types.Var
			for o := range boxes {
				vars = append(vars, o)
			}
			sort.Slice(vars, func(i, j int) bool { return vars[i].Pos() < vars[j].Pos() })
			for _, o := range vars {
				calls := boxes[o]
				if len(calls) < 2 || !params[o] || assigned[o] {
					continue
				}
				bn := o.Name() + "Box_"
				if names[bn] {
					continue
				}
				for _, c := range calls {
					rw.Rewrite[c] = func() string { return bn }
				}
				hoisted = append(hoisted, bn+" := boxNull_("+o.Name()+")")
				stats["boxes: type-variable parameters boxed once per call"]++
				n++
			}
			if len(hoisted) > 0 {
				first := fd.Body.List[0]
				h := strings.Join(hoisted, "\n")
				rw.Rewrite[first] = func() string { return h + "\n" + rw.RenderChildren(first) }
			}
		}
	}
	return n
}
