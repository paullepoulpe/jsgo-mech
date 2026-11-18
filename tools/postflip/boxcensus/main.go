// Command boxcensus attributes boxing allocations to the erased declarations that cause them.
// It is a measurement, not a rewrite: it changes no file.
//
// A boxing site is a value of a non-interface type that becomes an `any` (or another
// interface): explicitly through strobj_ / obj_ / any(...), or implicitly as an argument,
// assigned value, returned value or composite-literal element whose slot has an interface type.
// Each site's sink is the declaration that wants the interface: the parameter of the called
// function or method (and, for a method called on a field or variable, that field or variable,
// which is the erased Java declaration such as `PMap<String, OverlaySlot> slots`), the field or
// variable assigned, or the function returned from.
//
// The allocation counts come from an allocation profile, already reduced (by a pprof reader
// outside the tree) to lines: <first generated-code line> TAB <allocating line> TAB <runtime
// function called from the generated line, or -> TAB <objects>, for samples whose allocating
// line has only interface conversions among its heap escapes (go build -gcflags=-m). Objects of
// a line are split evenly among the allocating boxing sites on it.
//
// usage: boxcensus <dir of package closure> <boxsites.tsv> [N]
// (BOXCENSUS_SINK=<sink> also lists the lines of that sink)
//
// Without a profile, -static lists the explicit boxes (obj_, strobj_, any(...), .box_(), the
// census row "boxing to any") by the cause of the box: the kind of declaration that wants the
// interface (an erased collection or other runtime API, a generated Object field, parameter or
// result, a variadic parameter, an []any element, a comparison...), with the most frequent
// declarations of each kind: boxcensus -static <dir> [N].
package main

import (
	"bufio"
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"

	"postflip/rw"
)

type site struct {
	cat      string // -static: the kind of sink
	line     string // g_File.go:123
	explicit string // strobj_, obj_, any, or "" (implicit)
	alloc    bool   // the conversion allocates (not a pointer, handle box, constant or small value)
	sink     string // the generic declaration: function/method parameter, field, result
	holder   string // the erased declaration the method was called on, if any
}

var sites []*site
var byLine = map[string][]*site{}

func lineOf(p token.Pos) string {
	pp := rw.Fset.Position(p)
	return fmt.Sprintf("%s:%d", filepath.Base(pp.Filename), pp.Line)
}

// allocates reports whether converting a value of type t (expression e) to an interface
// allocates: pointer-shaped values and constants do not.
func allocates(t types.Type, e ast.Expr) bool {
	if tv, ok := rw.Info.Types[e]; ok && tv.Value != nil {
		return false
	}
	switch u := t.Underlying().(type) {
	case *types.Pointer, *types.Map, *types.Chan, *types.Signature, *types.Interface:
		return false
	case *types.Basic:
		switch u.Kind() {
		case types.Bool, types.UntypedBool, types.Int8, types.Uint8, types.UntypedNil:
			return false
		}
		return true
	case *types.Struct:
		return u.NumFields() > 0
	}
	return true
}

func objName(o types.Object) string {
	switch x := o.(type) {
	case *types.Func:
		if sig, ok := x.Type().(*types.Signature); ok && sig.Recv() != nil {
			return typeName(sig.Recv().Type()) + "." + x.Name()
		}
		return x.Name()
	case *types.Var:
		if x.IsField() {
			return "field " + fieldOwner(x) + "." + x.Name()
		}
		if x.Parent() == rw.Pkg.Scope() {
			return "global " + x.Name()
		}
		return "local"
	}
	return o.Name()
}

var fieldOwners map[*types.Var]string

func fieldOwner(f *types.Var) string {
	if fieldOwners == nil {
		fieldOwners = map[*types.Var]string{}
		for _, n := range rw.Pkg.Scope().Names() {
			tn, ok := rw.Pkg.Scope().Lookup(n).(*types.TypeName)
			if !ok {
				continue
			}
			if st, ok := tn.Type().Underlying().(*types.Struct); ok {
				for i := 0; i < st.NumFields(); i++ {
					fieldOwners[st.Field(i)] = n
				}
			}
		}
	}
	if s, ok := fieldOwners[f]; ok {
		return s
	}
	return "?"
}

func typeName(t types.Type) string {
	if p, ok := t.(*types.Pointer); ok {
		t = p.Elem()
	}
	if n, ok := types.Unalias(t).(*types.Named); ok {
		return n.Obj().Name()
	}
	return types.TypeString(t, func(*types.Package) string { return "" })
}

// holderOf names the erased declaration that receiver expression x reads, if it is one.
func holderOf(x ast.Expr) string {
	switch v := rw.Unparen(x).(type) {
	case *ast.SelectorExpr:
		if o, ok := rw.Info.Uses[v.Sel].(*types.Var); ok && o.IsField() {
			return "field " + fieldOwner(o) + "." + o.Name() + " " + typeName(o.Type())
		}
	case *ast.Ident:
		if o, ok := rw.Info.Uses[v].(*types.Var); ok {
			if o.Parent() == rw.Pkg.Scope() {
				return "global " + o.Name()
			}
			return "local/param " + typeName(o.Type())
		}
	case *ast.CallExpr:
		if name, _ := rw.CallName(v); name != "" {
			return "result of " + name
		}
		if s, ok := v.Fun.(*ast.SelectorExpr); ok {
			if sel := rw.Info.Selections[s]; sel != nil {
				return "result of " + objName(sel.Obj())
			}
		}
	}
	return ""
}

// sinkOf describes where boxed expression e (whose value is an interface) goes.
func sinkOf(e ast.Expr) (sink, holder string) {
	p := rw.Up(e)
	switch x := p.(type) {
	case *ast.CallExpr:
		idx := -1
		for i, a := range x.Args {
			if rw.Unparen(a) == rw.Unparen(e) {
				idx = i
			}
		}
		if idx < 0 {
			return "callee", ""
		}
		switch f := rw.Unparen(x.Fun).(type) {
		case *ast.Ident:
			if o := rw.Info.Uses[f]; o != nil {
				return fmt.Sprintf("%s#%d", objName(o), idx), ""
			}
		case *ast.IndexExpr:
			if id, ok := f.X.(*ast.Ident); ok {
				return fmt.Sprintf("%s#%d", id.Name, idx), ""
			}
		case *ast.SelectorExpr:
			if sel := rw.Info.Selections[f]; sel != nil {
				if sel.Kind() == types.FieldVal {
					return fmt.Sprintf("func field %s#%d", objName(sel.Obj()), idx), ""
				}
				return fmt.Sprintf("%s#%d", objName(sel.Obj()), idx), holderOf(f.X)
			}
			if o := rw.Info.Uses[f.Sel]; o != nil {
				return fmt.Sprintf("%s#%d", objName(o), idx), ""
			}
		}
		return "call", ""
	case *ast.AssignStmt:
		for i, r := range x.Rhs {
			if rw.Unparen(r) == rw.Unparen(e) && i < len(x.Lhs) {
				switch l := x.Lhs[i].(type) {
				case *ast.SelectorExpr:
					if o, ok := rw.Info.Uses[l.Sel].(*types.Var); ok {
						return objName(o), ""
					}
				case *ast.Ident:
					if o, ok := rw.Info.Uses[l].(*types.Var); ok {
						return objName(o), ""
					}
					return "local", ""
				case *ast.IndexExpr:
					return "array element " + typeName(rw.Info.Types[l.X].Type), ""
				}
			}
		}
		return "assign", ""
	case *ast.ValueSpec:
		return "local", ""
	case *ast.ReturnStmt:
		for q := ast.Node(x); q != nil; q = rw.Parent[q] {
			switch fn := q.(type) {
			case *ast.FuncLit:
				return "return of func literal", ""
			case *ast.FuncDecl:
				return "return of " + objName(rw.Info.Defs[fn.Name]), ""
			}
		}
	case *ast.CompositeLit:
		return "element of " + typeName(rw.Info.Types[x].Type), ""
	case *ast.KeyValueExpr:
		if id, ok := x.Key.(*ast.Ident); ok {
			if cl, ok := rw.Up(x).(*ast.CompositeLit); ok {
				return "field " + typeName(rw.Info.Types[cl].Type) + "." + id.Name, ""
			}
		}
		return "key/value", ""
	case *ast.BinaryExpr:
		return "comparison", ""
	}
	return fmt.Sprintf("%T", p), ""
}

func isIface(t types.Type) bool {
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

func add(e ast.Expr, valueType types.Type, explicit string, boxed ast.Expr) {
	s := &site{line: lineOf(e.Pos()), explicit: explicit, alloc: allocates(valueType, e)}
	if strings.HasSuffix(explicit, ".box_") || explicit == ".self" {
		s.alloc = false
	}
	s.sink, s.holder = sinkOf(boxed)
	s.cat = category(boxed)
	sites = append(sites, s)
	byLine[s.line] = append(byLine[s.line], s)
}

// implicit records e if it has a non-interface type and its slot has interface type slot.
func implicit(e ast.Expr, slot types.Type) {
	if slot == nil || !isIface(slot) {
		return
	}
	tv, ok := rw.Info.Types[e]
	if !ok || tv.Type == nil || isIface(tv.Type) || tv.IsNil() {
		return
	}
	add(e, tv.Type, "", e)
}

var static bool

func main() {
	if len(os.Args) > 1 && os.Args[1] == "-static" {
		static = true
		os.Args = append(os.Args[:1], os.Args[2:]...)
	}
	rw.Load(os.Args[1])
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		var results []*types.Tuple
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncDecl:
				if o, ok := rw.Info.Defs[x.Name].(*types.Func); ok {
					results = append(results[:0], o.Type().(*types.Signature).Results())
				}
			case *ast.CallExpr:
				name, _ := rw.CallName(x)
				if (name == "strobj_" || strings.HasPrefix(name, "obj_") || name == "any") && len(x.Args) == 1 {
					if tv, ok := rw.Info.Types[x.Args[0]]; ok && !isIface(tv.Type) {
						add(x.Args[0], tv.Type, name, x)
						return true
					}
				}
				if s, ok := x.Fun.(*ast.SelectorExpr); ok && len(x.Args) == 0 && (s.Sel.Name == "box_" || s.Sel.Name == "self") {
					// handle boxes and self() allocate nothing; only -static lists box_()
					if static && s.Sel.Name == "box_" {
						add(s.X, rw.Info.Types[s.X].Type, ".box_", x)
					}
					return true
				}
				if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
					return true
				}
				sig, ok := rw.Info.Types[x.Fun].Type.(*types.Signature)
				if !ok {
					return true
				}
				for i, a := range x.Args {
					var pt types.Type
					if sig.Variadic() && i >= sig.Params().Len()-1 {
						if x.Ellipsis.IsValid() {
							continue
						}
						pt = sig.Params().At(sig.Params().Len() - 1).Type().(*types.Slice).Elem()
					} else if i < sig.Params().Len() {
						pt = sig.Params().At(i).Type()
					}
					implicit(a, pt)
				}
			case *ast.AssignStmt:
				if len(x.Lhs) == len(x.Rhs) && x.Tok != token.DEFINE {
					for i := range x.Lhs {
						if tv, ok := rw.Info.Types[x.Lhs[i]]; ok {
							implicit(x.Rhs[i], tv.Type)
						}
					}
				}
			case *ast.ValueSpec:
				if x.Type != nil {
					for _, v := range x.Values {
						implicit(v, rw.Info.Types[x.Type].Type)
					}
				}
			case *ast.ReturnStmt:
				// the enclosing function's results
				var res *types.Tuple
				for q := ast.Node(x); q != nil; q = rw.Parent[q] {
					if fl, ok := q.(*ast.FuncLit); ok {
						res = rw.Info.Types[fl].Type.(*types.Signature).Results()
						break
					}
					if fd, ok := q.(*ast.FuncDecl); ok {
						res = rw.Info.Defs[fd.Name].Type().(*types.Signature).Results()
						break
					}
				}
				if res != nil && res.Len() == len(x.Results) {
					for i, r := range x.Results {
						implicit(r, res.At(i).Type())
					}
				}
			case *ast.CompositeLit:
				tv := rw.Info.Types[x]
				if tv.Type == nil {
					return true
				}
				switch u := tv.Type.Underlying().(type) {
				case *types.Slice:
					for _, el := range x.Elts {
						implicit(el, u.Elem())
					}
				case *types.Struct:
					for i, el := range x.Elts {
						if kv, ok := el.(*ast.KeyValueExpr); ok {
							if id, ok := kv.Key.(*ast.Ident); ok {
								if o, ok := rw.Info.Uses[id].(*types.Var); ok {
									implicit(kv.Value, o.Type())
								}
							}
						} else if i < u.NumFields() {
							implicit(el, u.Field(i).Type())
						}
					}
				}
			}
			return true
		})
	}
	nAlloc := 0
	for _, s := range sites {
		if s.alloc {
			nAlloc++
		}
	}
	fmt.Printf("boxing sites in g_*.go: %d (allocating: %d)\n", len(sites), nAlloc)
	if static {
		top := 15
		if len(os.Args) > 2 {
			top, _ = strconv.Atoi(os.Args[2])
		}
		staticReport(top)
		return
	}
	if len(os.Args) < 3 {
		return
	}
	top := 40
	if len(os.Args) > 3 {
		top, _ = strconv.Atoi(os.Args[3])
	}
	strobjLine := ""
	if o, ok := rw.Pkg.Scope().Lookup("strobj_").(*types.Func); ok {
		// the allocating line of strobj_ is its last statement, `return s`
		for _, f := range rw.Files {
			for _, d := range f.Decls {
				if fd, ok := d.(*ast.FuncDecl); ok && rw.Info.Defs[fd.Name] == o {
					strobjLine = lineOf(fd.Body.List[len(fd.Body.List)-1].Pos())
				}
			}
		}
	}
	bySink := map[string]float64{}
	byHolder := map[string]float64{}
	byClass := map[string]float64{}
	var total, unplaced float64
	fh, err := os.Open(os.Args[2])
	if err != nil {
		panic(err)
	}
	sc := bufio.NewScanner(fh)
	for sc.Scan() {
		c := strings.Split(sc.Text(), "\t")
		v, _ := strconv.ParseFloat(c[3], 64)
		total += v
		gl, leaf, callee := c[0], c[1], c[2]
		var cands []*site
		switch {
		case leaf == strobjLine: // inside strobj_
			for _, s := range byLine[gl] {
				if s.explicit == "strobj_" && s.alloc {
					cands = append(cands, s)
				}
			}
		case leaf == gl:
			for _, s := range byLine[gl] {
				if s.alloc && s.explicit != "strobj_" {
					cands = append(cands, s)
				}
			}
		default:
			k := "runtime " + callee
			bySink[k] += v
			byClass[k] += v
			continue
		}
		if len(cands) == 0 {
			unplaced += v
			bySink["? "+gl+" "+leaf] += v
			continue
		}
		for _, s := range cands {
			w := v / float64(len(cands))
			if ws := os.Getenv("BOXCENSUS_SINK"); ws != "" && s.sink == ws {
				fmt.Printf("  %s %.0f\n", gl, w)
			}
			bySink[s.sink] += w
			cl := s.sink
			if i := strings.Index(cl, "#"); i >= 0 {
				cl = cl[:i]
			}
			if i := strings.Index(cl, "."); i >= 0 && !strings.HasPrefix(cl, "field ") {
				cl = cl[:i]
			}
			byClass[cl] += w
			if s.holder != "" {
				byHolder[s.holder+" -> "+s.sink] += w
			}
		}
	}
	dump := func(title string, m map[string]float64) {
		type kv struct {
			k string
			v float64
		}
		var l []kv
		for k, v := range m {
			l = append(l, kv{k, v})
		}
		sort.Slice(l, func(i, j int) bool { return l[i].v > l[j].v || l[i].v == l[j].v && l[i].k < l[j].k })
		fmt.Printf("\n%s\n", title)
		for i, e := range l {
			if i >= top {
				break
			}
			fmt.Printf("%10.0f %5.1f%%  %s\n", e.v, 100*e.v/total, e.k)
		}
	}
	fmt.Printf("boxing objects in profile: %.0f (unplaced %.0f)\n", total, unplaced)
	dump("by generic declaration (function, method or class):", byClass)
	dump("by parameter / field / result:", bySink)
	dump("by erased declaration the method is called on:", byHolder)
}

// inRuntime reports whether o is declared in a handwritten file (rt_*.go).
func inRuntime(o types.Object) bool {
	return o != nil && o.Pos().IsValid() && strings.HasPrefix(filepath.Base(rw.Fset.Position(o.Pos()).Filename), "rt_")
}

// category is the kind of declaration that makes boxed (an interface-typed expression) an
// interface: what would have to be typed for the box to go.
func category(boxed ast.Expr) string {
	p := rw.Up(boxed)
	switch x := p.(type) {
	case *ast.CallExpr:
		idx := -1
		for i, a := range x.Args {
			if rw.Unparen(a) == rw.Unparen(boxed) {
				idx = i
			}
		}
		if idx < 0 {
			return "method called on a box"
		}
		var o types.Object
		recv := ""
		switch f := rw.Unparen(x.Fun).(type) {
		case *ast.Ident:
			o = rw.Info.Uses[f]
		case *ast.IndexExpr:
			if id, ok := f.X.(*ast.Ident); ok {
				o = rw.Info.Uses[id]
			}
		case *ast.SelectorExpr:
			if sel := rw.Info.Selections[f]; sel != nil {
				if sel.Kind() == types.FieldVal {
					return "argument of a func-typed field"
				}
				o = sel.Obj()
				recv = typeName(sel.Recv())
			} else {
				o = rw.Info.Uses[f.Sel]
			}
		}
		if o == nil {
			return "argument of a func value"
		}
		if sig, ok := o.Type().(*types.Signature); ok && sig.Variadic() && idx >= sig.Params().Len()-1 {
			if inRuntime(o) {
				return "variadic runtime argument"
			}
			return "variadic generated argument"
		}
		if inRuntime(o) {
			if recv != "" {
				return "runtime method parameter (" + recv + ")"
			}
			return "runtime function parameter"
		}
		if _, ok := o.(*types.Var); ok {
			return "argument of a func value"
		}
		if recv != "" {
			if _, ok := types.Unalias(rw.Info.Types[x.Fun.(*ast.SelectorExpr).X].Type).Underlying().(*types.Interface); ok {
				return "generated interface method parameter"
			}
		}
		return "generated parameter"
	case *ast.AssignStmt:
		for i, r := range x.Rhs {
			if rw.Unparen(r) == rw.Unparen(boxed) && i < len(x.Lhs) {
				switch l := x.Lhs[i].(type) {
				case *ast.SelectorExpr:
					if o, ok := rw.Info.Uses[l.Sel].(*types.Var); ok && o.IsField() {
						if inRuntime(o) {
							return "runtime field"
						}
						return "generated field"
					}
				case *ast.IndexExpr:
					return "array element"
				}
				return "local variable"
			}
		}
		return "assignment"
	case *ast.ValueSpec:
		return "local variable"
	case *ast.ReturnStmt:
		for q := ast.Node(x); q != nil; q = rw.Parent[q] {
			switch fn := q.(type) {
			case *ast.FuncLit:
				return "result of a func literal"
			case *ast.FuncDecl:
				if inRuntime(rw.Info.Defs[fn.Name]) {
					return "runtime result"
				}
				return "generated result"
			}
		}
	case *ast.CompositeLit:
		return "element of a composite literal"
	case *ast.KeyValueExpr:
		return "field in a composite literal"
	case *ast.BinaryExpr:
		return "comparison"
	case *ast.TypeAssertExpr:
		return "type assertion of a box"
	case *ast.SelectorExpr:
		return "method called on a box"
	}
	return fmt.Sprintf("other (%T)", p)
}

func staticReport(top int) {
	byCat := map[string]int{}
	byKind := map[string]map[string]int{}
	bySink := map[string]map[string]int{}
	for _, s := range sites {
		if s.explicit == "" {
			continue
		}
		byCat[s.cat]++
		if byKind[s.cat] == nil {
			byKind[s.cat] = map[string]int{}
			bySink[s.cat] = map[string]int{}
		}
		byKind[s.cat][s.explicit]++
		k := s.sink
		if s.holder != "" {
			k += " on " + s.holder
		}
		bySink[s.cat][k]++
	}
	type kv struct {
		k string
		v int
	}
	sorted := func(m map[string]int) []kv {
		var l []kv
		n := 0
		for k, v := range m {
			l = append(l, kv{k, v})
			n += v
		}
		sort.Slice(l, func(i, j int) bool { return l[i].v > l[j].v || l[i].v == l[j].v && l[i].k < l[j].k })
		return l
	}
	total := 0
	for _, v := range byCat {
		total += v
	}
	fmt.Printf("explicit boxes: %d\n", total)
	for _, c := range sorted(byCat) {
		var kinds []string
		for _, k := range sorted(byKind[c.k]) {
			kinds = append(kinds, fmt.Sprintf("%s %d", k.k, k.v))
		}
		fmt.Printf("\n%6d  %s  (%s)\n", c.v, c.k, strings.Join(kinds, ", "))
		for i, s := range sorted(bySink[c.k]) {
			if i >= top {
				break
			}
			fmt.Printf("    %5d  %s\n", s.v, s.k)
		}
	}
}
