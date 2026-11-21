// Command lamparams is a post-flip step: a lambda parameter that Java types by the functional
// interface's instantiation, and that the translation receives as `any` and casts on entry,
// takes the cast's type; the lambda adapter casts the argument instead.
//
//	setInternalFactory(&Function_tfn_any_CompilerPass{func(compiler_ any) CompilerPass {
//		var compiler *AbstractCompiler = to_AbstractCompiler(compiler_)
//		_ = compiler
//		return new_CheckSideEffects(compiler, ...)
//	}})
//	  ->
//	setInternalFactory(&Function_tfn_any_CompilerPass_cAbstractCompiler{func(compiler *AbstractCompiler) CompilerPass {
//		return new_CheckSideEffects(compiler, ...)
//	}})
//
//	func (this *Function_tfn_any_CompilerPass_cAbstractCompiler) apply(p0 any) any {
//		return this.f(to_AbstractCompiler(p0))
//	}
//
// The typed translator writes a Java lambda as an adapter object &X_tfn_T{f} (or the erased
// &X_fn{f}) whose interface method converts the erased arguments and calls f. It types f's
// parameters where the conversion is an unboxing or cast_ / icast_ (j2go's adapters with
// str_, Node_of_, cast_[*C], icast_I), and leaves `any` where the parameter's type is a class
// with subclasses (to_X), an interface j2go did not cast in the adapter, or an erased type
// argument; then the lambda's first statement casts it. That cast is javac's: a lambda's
// synthetic method takes the instantiated parameter types, and the bridge the metafactory
// generates casts the Object argument before the body runs (a method reference the same,
// before the referenced method is called). So the cast belongs to the adapter, as j2go
// does for the classes it typed.
//
// A parameter p of type any of a func literal that is the function of an adapter literal is
// converted when its only use is the cast K(p) (cast_[T], to_X or icast_I), and the cast is
// the first thing the lambda evaluates:
//   - the first statement is `var v T = K(p)` or `v := K(p)` (a lambda's typed parameter);
//     p is renamed v and gets the type T, the statement and a following `_ = v` go;
//   - or the body is one statement (a return, an expression) whose evaluation starts with
//     K(p): K(p) is the statement's expression, or the receiver or an argument of a call of a
//     function named by a name, or of a method of a name or a field of this, and every
//     operand before it is such a name or the cast of a parameter converted before it,
//     recursively (a method reference); K(p) is p, of type T.
// and every method of the adapter that calls f passes it the method's own parameter at p's
// position unconverted. The lambdas of one adapter type with the same casts share a new
// adapter type (g_0lamparams.go): the original's methods with K applied to the argument at
// each converted position, and f's parameters of the cast types. The adapter then evaluates
// K(p0) where the lambda's first step did, with the same value and failure (K is a cast of the
// value the interface method was given; a ClassCastException or nil as before), so Java's
// behaviour is kept for every argument.
//
// Left, counted: parameters used other than by one cast (passed on as Objects, compared,
// several casts), casts not the lambda's first step, func literals that are not an
// adapter's function (given to a func-typed parameter: the callee's signature), and
// adapters whose methods convert the argument themselves.
//
// A rerun changes nothing (a converted parameter is no longer `any`).
//
// usage: lamparams [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

const outFile = "g_0lamparams.go"

var qual = func(p *types.Package) string {
	if p == rw.Pkg {
		return ""
	}
	return p.Name()
}

// adapter: a named struct type with one field f of a func type, and its methods.
type adapter struct {
	tn      *types.TypeName
	fsig    *types.Signature
	decl    *ast.TypeSpec
	methods []*ast.FuncDecl
	// passes[i]: every method calling this.f passes its own parameter unconverted at i.
	passes []bool
	// calls: the this.f call of each method that has one.
	calls map[*ast.FuncDecl]*ast.CallExpr
}

var adapters = map[*types.TypeName]*adapter{}

func isAny(t types.Type) bool {
	it, ok := t.Underlying().(*types.Interface)
	return ok && it.Empty() && !isTypeParam(t)
}

func isTypeParam(t types.Type) bool {
	_, ok := t.(*types.TypeParam)
	return ok
}

func findAdapters() {
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.TYPE {
				continue
			}
			for _, s := range gd.Specs {
				ts := s.(*ast.TypeSpec)
				if ts.TypeParams != nil {
					continue
				}
				tn, _ := rw.Info.Defs[ts.Name].(*types.TypeName)
				if tn == nil {
					continue
				}
				st, ok := tn.Type().Underlying().(*types.Struct)
				if !ok || st.NumFields() != 1 || st.Field(0).Name() != "f" {
					continue
				}
				sig, ok := st.Field(0).Type().(*types.Signature)
				if !ok {
					continue
				}
				adapters[tn] = &adapter{tn: tn, fsig: sig, decl: ts, calls: map[*ast.FuncDecl]*ast.CallExpr{}}
			}
		}
	}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv == nil || len(fd.Recv.List) != 1 {
				continue
			}
			rt := rw.Info.TypeOf(fd.Recv.List[0].Type)
			pt, ok := rt.(*types.Pointer)
			if !ok {
				continue
			}
			nt, ok := pt.Elem().(*types.Named)
			if !ok {
				continue
			}
			a := adapters[nt.Obj()]
			if a == nil {
				continue
			}
			a.methods = append(a.methods, fd)
		}
	}
	for _, a := range adapters {
		n := a.fsig.Params().Len()
		a.passes = make([]bool, n)
		for i := range a.passes {
			a.passes[i] = true
		}
		if a.fsig.Variadic() {
			for i := range a.passes {
				a.passes[i] = false
			}
		}
		for _, m := range a.methods {
			var call *ast.CallExpr
			ast.Inspect(m.Body, func(x ast.Node) bool {
				c, ok := x.(*ast.CallExpr)
				if ok && rw.Text(c.Fun) == "this.f" {
					if call != nil {
						call = &ast.CallExpr{} // two calls: unknown
					}
					if call == nil {
						call = c
					}
				}
				return true
			})
			if call == nil {
				continue
			}
			if call.Fun == nil || len(call.Args) != n {
				for i := range a.passes {
					a.passes[i] = false
				}
				continue
			}
			a.calls[m] = call
			params := map[types.Object]int{}
			i := 0
			for _, fld := range m.Type.Params.List {
				for _, nm := range fld.Names {
					params[rw.Info.Defs[nm]] = i
					i++
				}
			}
			for j, arg := range call.Args {
				id, ok := arg.(*ast.Ident)
				if !ok {
					a.passes[j] = false
					continue
				}
				if k, ok := params[rw.Info.Uses[id]]; !ok || k != j {
					a.passes[j] = false
				}
			}
		}
		if len(a.calls) == 0 {
			for i := range a.passes {
				a.passes[i] = false
			}
		}
	}
}

// castOf returns the cast function's text and the result type if e is K(x) with K a cast
// (cast_[T], to_X, icast_I) of exactly x.
func castOf(e ast.Expr) (string, types.Type, ast.Expr) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 1 {
		return "", nil, nil
	}
	name := ""
	switch f := c.Fun.(type) {
	case *ast.Ident:
		name = f.Name
		if !(strings.HasPrefix(name, "to_") || strings.HasPrefix(name, "icast_")) {
			return "", nil, nil
		}
		if o, ok := rw.Info.Uses[f].(*types.Func); !ok || o.Pkg() != rw.Pkg || o.Parent() != rw.Pkg.Scope() {
			return "", nil, nil
		}
	case *ast.IndexExpr:
		id, ok := f.X.(*ast.Ident)
		if !ok || id.Name != "cast_" {
			return "", nil, nil
		}
		if o, ok := rw.Info.Uses[id].(*types.Func); !ok || o.Pkg() != rw.Pkg {
			return "", nil, nil
		}
		name = "cast_"
	default:
		return "", nil, nil
	}
	sig, ok := rw.Info.TypeOf(c.Fun).(*types.Signature)
	if !ok || sig.Params().Len() != 1 || !isAny(sig.Params().At(0).Type()) || sig.Results().Len() != 1 {
		return "", nil, nil
	}
	_ = name
	return rw.Text(c.Fun), sig.Results().At(0).Type(), c.Args[0]
}

// convCast: the casts of the parameters already converted in the lambda being looked at
// (they become names).
var convCast = map[ast.Expr]bool{}

// pureName: a name, a field of this (never nil) or a package-qualified name, or the cast of a
// parameter converted before (by position, the order of the bridge's casts): evaluating it has
// no effect and cannot fail.
func pureName(e ast.Expr) bool {
	if convCast[rw.Unparen(e)] {
		return true
	}
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		if sel := rw.Info.Selections[x]; sel != nil && sel.Kind() != types.FieldVal {
			return false
		}
		if id, ok := x.X.(*ast.Ident); ok {
			if _, ok := rw.Info.Uses[id].(*types.PkgName); ok {
				return true
			}
			return id.Name == "this" // a field of the (non-nil) receiver
		}
	}
	return false
}

// startsWith reports whether evaluating e starts with evaluating target.
func startsWith(e, target ast.Expr) bool {
	e = rw.Unparen(e)
	if e == target {
		return true
	}
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	fun := c.Fun
	if ix, ok := fun.(*ast.IndexExpr); ok {
		fun = ix.X
	}
	if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() {
		// a conversion
		return len(c.Args) == 1 && startsWith(c.Args[0], target)
	}
	switch f := fun.(type) {
	case *ast.Ident:
	case *ast.SelectorExpr:
		if sel := rw.Info.Selections[f]; sel != nil && sel.Kind() == types.MethodVal {
			if contains(f.X, target) {
				return startsWith(f.X, target)
			}
			if !pureName(f.X) {
				return false
			}
		} else if _, ok := rw.Info.Uses[f.Sel].(*types.Func); ok {
			// a package-qualified function
		} else {
			return false
		}
	default:
		return false
	}
	for _, a := range c.Args {
		if contains(a, target) {
			return startsWith(a, target)
		}
		if !pureName(a) {
			return false
		}
	}
	return false
}

func contains(e ast.Node, target ast.Node) bool {
	found := false
	ast.Inspect(e, func(n ast.Node) bool {
		if n == target {
			found = true
		}
		return !found
	})
	return found
}

type conv struct {
	k    string     // cast text, "" if the position is not converted
	t    types.Type // the cast's result type
	name string     // the parameter's new name
}

type site struct {
	lit    *ast.CompositeLit
	fl     *ast.FuncLit
	a      *adapter
	convs  []conv
	params []*ast.Ident // the func literal's parameter names by position
	edits  []func()
}

func main() {
	report, verbose := false, false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	dir := os.Args[len(os.Args)-1]
	rw.Load(dir)
	findAdapters()

	left := map[string]int{}
	leftAt := map[string][]string{}
	skip := func(why string, n ast.Node) {
		left[why]++
		if verbose {
			leftAt[why] = append(leftAt[why], rw.Fset.Position(n.Pos()).String()+" "+firstLine(rw.Text(n)))
		}
	}
	done := map[string]int{}
	var sites []*site
	inLit := map[*ast.FuncLit]*ast.CompositeLit{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			lit, ok := n.(*ast.CompositeLit)
			if !ok || len(lit.Elts) != 1 {
				return true
			}
			el := lit.Elts[0]
			if kv, ok := el.(*ast.KeyValueExpr); ok {
				el = kv.Value
			}
			fl, ok := el.(*ast.FuncLit)
			if !ok {
				return true
			}
			nt, ok := rw.Info.TypeOf(lit).(*types.Named)
			if !ok || adapters[nt.Obj()] == nil {
				return true
			}
			inLit[fl] = lit
			return true
		})
	}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			fl, ok := n.(*ast.FuncLit)
			if !ok {
				return true
			}
			var anyParams []*ast.Ident
			var params []*ast.Ident
			for _, fld := range fl.Type.Params.List {
				for _, nm := range fld.Names {
					params = append(params, nm)
					if isAny(rw.Info.TypeOf(fld.Type)) {
						anyParams = append(anyParams, nm)
					}
				}
			}
			if len(anyParams) == 0 {
				return true
			}
			lit := inLit[fl]
			if lit == nil {
				for _, p := range anyParams {
					skip("a func literal that is not an adapter's function (its type is the callee's)", p)
				}
				return true
			}
			a := adapters[rw.Info.TypeOf(lit).(*types.Named).Obj()]
			s := &site{lit: lit, fl: fl, a: a, convs: make([]conv, len(params)), params: params}
			convCast = map[ast.Expr]bool{}
			for i, p := range params {
				s.convs[i].name = p.Name
				if !isAny(rw.Info.TypeOf(p)) {
					continue
				}
				if p.Name == "_" {
					skip("an unnamed parameter", p)
					continue
				}
				if !a.passes[i] {
					skip("the adapter converts the argument itself or does not pass it to f", p)
					continue
				}
				obj := rw.Info.Defs[p]
				uses := rw.UsesOf[obj]
				if len(uses) != 1 {
					if len(uses) == 0 {
						skip("unused", p)
					} else {
						skip("used more than once", p)
					}
					continue
				}
				u := uses[0]
				// the cast around the use
				call, ok := rw.Parent[u].(*ast.CallExpr)
				if !ok {
					skip("used other than by a cast", p)
					continue
				}
				k, t, arg := castOf(call)
				if k == "" || arg != ast.Expr(u) {
					skip("used other than by a cast", p)
					continue
				}
				body := fl.Body.List
				if len(body) > 0 {
					// var v T = K(p) / v := K(p)
					var vid *ast.Ident
					var rhs ast.Expr
					switch st := body[0].(type) {
					case *ast.DeclStmt:
						if gd, ok := st.Decl.(*ast.GenDecl); ok && len(gd.Specs) == 1 {
							if vs, ok := gd.Specs[0].(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 1 {
								vid, rhs = vs.Names[0], vs.Values[0]
							}
						}
					case *ast.AssignStmt:
						if st.Tok == token.DEFINE && len(st.Lhs) == 1 && len(st.Rhs) == 1 {
							vid, _ = st.Lhs[0].(*ast.Ident)
							rhs = st.Rhs[0]
						}
					}
					if vid != nil && rw.Unparen(rhs) == ast.Expr(call) && types.Identical(rw.Info.TypeOf(vid), t) {
						vobj := rw.Info.Defs[vid]
						s.convs[i] = conv{k: k, t: t, name: vid.Name}
						first := body[0]
						s.edits = append(s.edits, func() { rw.Rewrite[first] = func() string { return rw.Del } })
						if len(body) > 1 {
							if as, ok := body[1].(*ast.AssignStmt); ok && len(as.Lhs) == 1 && len(as.Rhs) == 1 && rw.Text(as.Lhs[0]) == "_" {
								if id, ok := as.Rhs[0].(*ast.Ident); ok && rw.Info.Uses[id] == vobj {
									second := body[1]
									s.edits = append(s.edits, func() { rw.Rewrite[second] = func() string { return rw.Del } })
								}
							}
						}
						done["the lambda's typed parameter (var v T = K(p) first)"]++
						continue
					}
				}
				if len(body) == 1 {
					var e ast.Expr
					switch st := body[0].(type) {
					case *ast.ReturnStmt:
						if len(st.Results) == 1 {
							e = st.Results[0]
						}
					case *ast.ExprStmt:
						e = st.X
					}
					if e != nil && startsWith(e, call) {
						s.convs[i] = conv{k: k, t: t, name: p.Name}
						convCast[call] = true
						c := call
						name := p.Name
						s.edits = append(s.edits, func() { rw.Rewrite[c] = func() string { return name } })
						done["a method reference's argument (the body starts with K(p))"]++
						continue
					}
				}
				skip("the cast is not the lambda's first step", p)
			}
			conv := false
			for _, c := range s.convs {
				conv = conv || c.k != ""
			}
			if conv {
				sites = append(sites, s)
			}
			return true
		})
	}

	// new adapter types, one per (adapter, casts)
	type group struct {
		a     *adapter
		convs []conv
		name  string
	}
	groups := map[string]*group{}
	for _, s := range sites {
		var parts []string
		for _, c := range s.convs {
			if c.k == "" {
				parts = append(parts, "x")
			} else {
				parts = append(parts, typeWord(c.t))
			}
		}
		name := s.a.tn.Name() + "_c" + strings.Join(parts, "_")
		if base, ok := strings.CutSuffix(s.a.tn.Name(), "_fn"); ok {
			name = base + "_c" + strings.Join(parts, "_") + "_fn" // still an erased adapter: X_fn
		}
		g := groups[name]
		if g == nil {
			g = &group{a: s.a, convs: s.convs, name: name}
			groups[name] = g
		}
		for _, e := range s.edits {
			e()
		}
		s := s
		rw.Rewrite[s.lit.Type] = func() string { return name }
		rw.Rewrite[s.fl.Type.Params] = func() string {
			var ps []string
			for i, p := range s.params {
				t := s.convs[i].t
				if s.convs[i].k == "" {
					t = rw.Info.TypeOf(p)
				}
				ps = append(ps, s.convs[i].name+" "+types.TypeString(t, qual))
			}
			return "(" + strings.Join(ps, ", ") + ")"
		}
	}
	names := []string{}
	for n := range groups {
		if rw.Pkg.Scope().Lookup(n) == nil {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	var b strings.Builder
	for _, n := range names {
		g := groups[n]
		a := g.a
		var ps []string
		for i := 0; i < a.fsig.Params().Len(); i++ {
			t := a.fsig.Params().At(i).Type()
			if g.convs[i].k != "" {
				t = g.convs[i].t
			}
			ps = append(ps, types.TypeString(t, qual))
		}
		res := ""
		switch a.fsig.Results().Len() {
		case 0:
		case 1:
			res = " " + types.TypeString(a.fsig.Results().At(0).Type(), qual)
		default:
			var rs []string
			for i := 0; i < a.fsig.Results().Len(); i++ {
				rs = append(rs, types.TypeString(a.fsig.Results().At(i).Type(), qual))
			}
			res = " (" + strings.Join(rs, ", ") + ")"
		}
		fmt.Fprintf(&b, "\n// %s is %s with f's parameters cast as the lambda's (lamparams).\ntype %s struct {\n\tf func(%s)%s\n}\n", n, a.tn.Name(), n, strings.Join(ps, ", "), res)
		for _, m := range a.methods {
			txt := rw.Text(m)
			recv := rw.Text(m.Recv)
			txt = strings.Replace(txt, recv, strings.Replace(recv, a.tn.Name(), n, 1), 1)
			if c := a.calls[m]; c != nil {
				var args []string
				for i, arg := range c.Args {
					if g.convs[i].k != "" {
						args = append(args, g.convs[i].k+"("+rw.Text(arg)+")")
					} else {
						args = append(args, rw.Text(arg))
					}
				}
				old := rw.Text(c)
				if strings.Count(txt, old) != 1 {
					panic("ambiguous call text in " + rw.Text(m.Name))
				}
				txt = strings.Replace(txt, old, "this.f("+strings.Join(args, ", ")+")", 1)
			}
			b.WriteString("\n" + txt + "\n")
		}
	}

	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	nleft := 0
	for _, k := range keys {
		nleft += left[k]
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
		if verbose {
			for _, p := range leftAt[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	total := 0
	keys = keys[:0]
	for k, n := range done {
		keys = append(keys, k)
		total += n
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", done[k], k)
	}
	fmt.Fprintf(os.Stderr, "lamparams: %d parameters of %d lambdas typed, %d left; %d adapter types (%d new)\n", total, len(sites), nleft, len(groups), len(names))
	if report || total == 0 {
		return
	}
	decls := b.String()
	var outF *ast.File
	for _, f := range rw.Files {
		if filepath.Base(rw.Fset.File(f.Pos()).Name()) == outFile {
			outF = f
		}
	}
	if len(names) > 0 {
		if outF != nil {
			last := outF.Decls[len(outF.Decls)-1]
			rw.Rewrite[last] = func() string { return rw.RenderChildren(last) + "\n" + decls }
		} else {
			path := filepath.Join(dir, outFile)
			src := []byte("// Code generated by tools/postflip/lamparams. DO NOT EDIT.\n\npackage closure\n\n" +
				"// Lambda adapters whose function takes the parameter types the lambda casts to (lamparams).\n" + decls)
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
	}
	fmt.Fprintf(os.Stderr, "lamparams: files changed: %d\n", rw.SaveChecked())
}

// typeWord names a type in an identifier: *X -> X, a qualified or instantiated name flattened.
func typeWord(t types.Type) string {
	s := types.TypeString(t, qual)
	s = strings.TrimPrefix(s, "*")
	r := strings.NewReplacer("*", "p", "[", "_", "]", "_", ",", "_", " ", "", ".", "_")
	return strings.TrimRight(r.Replace(s), "_")
}

func firstLine(s string) string {
	if i := strings.IndexByte(s, '\n'); i >= 0 {
		s = s[:i]
	}
	if len(s) > 140 {
		s = s[:140]
	}
	return s
}
