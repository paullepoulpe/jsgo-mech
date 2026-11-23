// Command generics is a post-flip step: java.util.Optional keeps its type argument.
//
// j2go erases Java generics: a declaration keeps its type argument only through an alias
// (`type OptionalOf[T0 any] = Optional`), so an Optional<String> holds its String boxed in an
// `any` (strobj_ allocates the string header) and every read unboxes it (str_). This step makes
// the runtime's Optional generic (rt_base.go: Optional[T]) and the alias Optional[T0], and then
// repairs the generated code until it type-checks, as j2go would have to if it emitted Go
// generics for Java generics:
//
//  1. (syntax) A bare `Optional`, which j2go writes where it does not keep the type argument
//     (interfaces, dispatchers, conditional expressions), is Optional[any], Java's raw or
//     wildcard Optional; Optional_empty() is Optional_empty[any]().
//  2. (overriding) Methods with one name and one erased signature (m and m_impl, interface and
//     class methods, the anonymous interfaces of dispatchers) must keep identical signatures, or
//     a class stops implementing an interface, which for a dispatcher's type assertion would be
//     a run-time failure. Where the declarations that state a type argument agree, the others
//     take it; where they disagree, all are Optional[any].
//  3. (boundaries) Each type error is repaired where it is: an Optional.of / ofNullable / empty
//     or a conditional expression producing an Optional[any] where an Optional[S] is wanted is
//     made an Optional[S] (its argument unboxed); any other Optional crossing between Optional[S]
//     and Optional[any] is converted (optConv_, optErase_: in Java both are the same object); an
//     `any` argument where an S is wanted is unboxed.
//  4. (uses) The content read from an Optional[S] (get, orElse, orElseThrow) is an S: the
//     unboxing around it (str_, cast_, to_X, Node_of_) goes, and where it still flows into an
//     `any` it is boxed the way the erased code boxed it (strobj_, obj_, self(), box_()), so
//     null still boxes to nil; filter with a lambda that only unboxes its argument becomes
//     filterFn_ with a typed func; an Optional[S] boxed into an Object is erased first, so code
//     unboxing Objects to Optional[any] still finds one.
//
// The steps repeat until the package type-checks and nothing changes; rerunning the tool on
// its output changes nothing.
//
// usage: generics [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"postflip/rw"
)

var stats = map[string]int{}

// load parses and type-checks the package, tolerating type errors, into rw's globals.
func load(dir string) []types.Error { return loadWith(dir, nil) }

// loadWith is load with the files of mem (by path) in place of those on disk, and its extra
// files added.
func loadWith(dir string, mem map[string][]byte) []types.Error {
	rw.Fset = token.NewFileSet()
	rw.Info = &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}, Instances: map[*ast.Ident]types.Instance{}, Implicits: map[ast.Node]types.Object{}}
	rw.Parent = map[ast.Node]ast.Node{}
	rw.Src = map[*token.File][]byte{}
	rw.Rewrite = map[ast.Node]func() string{}
	rw.Files = nil
	rw.UsesOf = map[types.Object][]*ast.Ident{}
	names, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	for n := range mem {
		if _, err := os.Stat(n); err != nil {
			names = append(names, n)
		}
	}
	sort.Strings(names)
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, ok := mem[n]
		var err error
		if !ok {
			b, err = os.ReadFile(n)
		}
		if err != nil {
			panic(err)
		}
		f, err := parser.ParseFile(rw.Fset, n, b, parser.SkipObjectResolution)
		if err != nil {
			panic(err)
		}
		rw.Src[rw.Fset.File(f.Pos())] = b
		rw.Files = append(rw.Files, f)
	}
	var errs []types.Error
	conf := types.Config{Importer: importer.ForCompiler(rw.Fset, "source", nil), Error: func(err error) {
		if te, ok := err.(types.Error); ok && !te.Soft {
			errs = append(errs, te)
		}
	}}
	rw.Pkg, _ = conf.Check("closure", rw.Fset, rw.Files, rw.Info)
	for _, f := range rw.Files {
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if len(stack) > 0 {
				rw.Parent[n] = stack[len(stack)-1]
			}
			stack = append(stack, n)
			return true
		})
	}
	return errs
}

func generated(n ast.Node) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.File(n.Pos()).Name()), "g_")
}

func qual(p *types.Package) string { return "" }

func tstr(t types.Type) string { return types.TypeString(t, qual) }

// optArg is S when t is Optional[S] (through aliases), else nil.
func optArg(t types.Type) types.Type {
	if t == nil {
		return nil
	}
	n, ok := types.Unalias(t).(*types.Named)
	if !ok || n.Obj().Name() != "Optional" || n.Obj().Pkg() != rw.Pkg || n.TypeArgs().Len() != 1 {
		return nil
	}
	return n.TypeArgs().At(0)
}

func isAny(t types.Type) bool {
	i, ok := types.Unalias(t).(*types.Interface)
	return ok && i.Empty()
}

func isIface(t types.Type) bool {
	_, ok := t.Underlying().(*types.Interface)
	return ok
}

// ---- 1. syntax

func phase1() int {
	n := 0
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			switch v := x.(type) {
			case *ast.Ident:
				if v.Name != "Optional" {
					return true
				}
				if ix, ok := rw.Parent[v].(*ast.IndexExpr); ok && ix.X == v {
					return true
				}
				if ts, ok := rw.Parent[v].(*ast.TypeSpec); ok && ts.Name == v {
					return true
				}
				if s, ok := rw.Parent[v].(*ast.SelectorExpr); ok && s.Sel == v {
					return true
				}
				rw.Rewrite[v] = func() string { return "Optional[any]" }
				n++
			case *ast.CallExpr:
				if id, ok := v.Fun.(*ast.Ident); ok && id.Name == "Optional_empty" && len(v.Args) == 0 {
					rw.Rewrite[v] = func() string { return "Optional_empty[any]()" }
					n++
				}
			case *ast.TypeSpec:
				// type OptionalOf[T0 any] = Optional
				if v.Name.Name == "OptionalOf" && v.TypeParams != nil {
					if id, ok := v.Type.(*ast.Ident); ok && id.Name == "Optional" {
						rw.Rewrite[id] = func() string { return "Optional[T0]" }
						n++
					}
					return false
				}
			}
			return true
		})
	}
	stats["1: bare Optional and Optional_empty() given a type argument"] += n
	return n
}

// ---- 2. overriding

type decl struct {
	ft   *ast.FuncType
	sig  *types.Signature
	desc string
}

// slotTypes are the top-level parameter and result types of ft with their expressions.
func slotExprs(ft *ast.FuncType) []ast.Expr {
	var out []ast.Expr
	for _, fl := range []*ast.FieldList{ft.Params, ft.Results} {
		if fl == nil {
			continue
		}
		for _, f := range fl.List {
			k := len(f.Names)
			if k == 0 {
				k = 1
			}
			for i := 0; i < k; i++ {
				out = append(out, f.Type)
			}
		}
	}
	return out
}

func erasedSig(sig *types.Signature) string {
	var b strings.Builder
	for _, tu := range []*types.Tuple{sig.Params(), sig.Results()} {
		for i := 0; i < tu.Len(); i++ {
			t := tu.At(i).Type()
			if optArg(t) != nil {
				b.WriteString("Optional,")
			} else {
				b.WriteString(tstr(types.Unalias(t)) + ",")
			}
		}
		b.WriteString("|")
	}
	return b.String()
}

// pinned are the override groups erased because their stated type arguments disagree: their
// results are not retyped from the values they return.
var pinned = map[string]bool{}

func groupKey(name string, sig *types.Signature) string {
	return strings.TrimSuffix(name, "_impl") + "/" + erasedSig(sig)
}

func unify() int {
	groups := map[string][]decl{}
	addDecl := func(name string, ft *ast.FuncType, sig *types.Signature, desc string) {
		if ft == nil || sig == nil || !strings.Contains(erasedSig(sig), "Optional,") {
			return
		}
		k := groupKey(name, sig)
		groups[k] = append(groups[k], decl{ft, sig, desc})
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			switch v := x.(type) {
			case *ast.FuncDecl:
				if v.Recv == nil {
					return true
				}
				if o, ok := rw.Info.Defs[v.Name].(*types.Func); ok {
					addDecl(v.Name.Name, v.Type, o.Type().(*types.Signature), rw.Fset.Position(v.Pos()).String())
				}
			case *ast.InterfaceType:
				for _, m := range v.Methods.List {
					ft, ok := m.Type.(*ast.FuncType)
					if !ok || len(m.Names) != 1 {
						continue
					}
					if o, ok := rw.Info.Defs[m.Names[0]].(*types.Func); ok {
						addDecl(m.Names[0].Name, ft, o.Type().(*types.Signature), rw.Fset.Position(m.Pos()).String())
					}
				}
			}
			return true
		})
	}
	keys := make([]string, 0, len(groups))
	for k := range groups {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	n := 0
	for _, k := range keys {
		ds := groups[k]
		slots := func(d decl) []types.Type {
			var out []types.Type
			for _, tu := range []*types.Tuple{d.sig.Params(), d.sig.Results()} {
				for i := 0; i < tu.Len(); i++ {
					out = append(out, optArg(tu.At(i).Type()))
				}
			}
			return out
		}
		width := len(slots(ds[0]))
		for i := 0; i < width; i++ {
			var stated []types.Type
			mixed := false
			for _, d := range ds {
				s := slots(d)[i]
				if s == nil {
					continue
				}
				if !isAny(s) {
					seen := false
					for _, t := range stated {
						if types.Identical(t, s) {
							seen = true
						}
					}
					if !seen {
						stated = append(stated, s)
					}
				} else {
					mixed = true
				}
			}
			if len(stated) == 0 || len(stated) == 1 && !mixed {
				continue
			}
			want := "Optional[any]"
			if len(stated) == 1 {
				want = "OptionalOf[" + tstr(stated[0]) + "]"
			}
			for _, d := range ds {
				s := slots(d)[i]
				if s == nil || len(stated) == 1 && types.Identical(s, stated[0]) || len(stated) != 1 && isAny(s) {
					continue
				}
				e := slotExprs(d.ft)[i]
				if _, done := rw.Rewrite[e]; done {
					continue
				}
				w := want
				rw.Rewrite[e] = func() string { return w }
				n++
				if len(stated) != 1 {
					pinned[k] = true
				}
				if len(stated) == 1 {
					stats["2: override slots given the stated type argument"]++
				} else {
					stats["2: override slots erased (stated type arguments disagree)"]++
				}
			}
		}
	}
	return n
}

// ---- 3. boundaries

// parseUse splits "cannot use X (DESC type HAVE) as WANT value in CONTEXT".
func parseUse(msg string) (x, want string, ok bool) {
	if !strings.HasPrefix(msg, "cannot use ") {
		return "", "", false
	}
	msg = strings.TrimPrefix(msg, "cannot use ")
	i := strings.LastIndex(msg, " value in ")
	if i < 0 {
		return "", "", false
	}
	head := msg[:i]
	j := strings.LastIndex(head, ") as ")
	if j < 0 {
		return "", "", false
	}
	want = head[j+len(") as "):]
	depth := 0
	for k := j; k >= 0; k-- {
		switch head[k] {
		case ')':
			depth++
		case '(':
			depth--
			if depth == 0 {
				if k == 0 || head[k-1] != ' ' {
					return "", "", false
				}
				return head[:k-1], want, true
			}
		}
	}
	return "", "", false
}

var reNil = regexp.MustCompile(`^cannot use nil as (.*) value in (.*)$`)
var reNotIface = regexp.MustCompile(`^invalid operation: (.*) \(.*type (.*)\) is not an interface$`)

// exprAt finds the largest expression starting at pos whose text, as go/types prints it, is s.
func exprAt(pos token.Pos, s string) ast.Expr {
	var best ast.Expr
	for _, f := range rw.Files {
		if rw.Fset.File(f.Pos()) != rw.Fset.File(pos) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			if x == nil || x.Pos() > pos || x.End() < pos {
				return false
			}
			if e, ok := x.(ast.Expr); ok && e.Pos() == pos && types.ExprString(e) == s {
				if best == nil || e.End() > best.End() {
					best = e
				}
			}
			return true
		})
	}
	return best
}

func conv(t types.Type) rw.Conv {
	cv := rw.ConvFor(t)
	if !cv.OK {
		ts := tstr(t)
		cv = rw.Conv{Box: func(e string) string { return "any(" + e + ")" }, Unbox: func(e string) string { return "cast_[" + ts + "](" + e + ")" }, IsUnbox: func(n string) bool { return n == "cast_["+ts+"]" }, OK: true}
	}
	return cv
}

// slotType is the type that the context of e wants e to have, where it is a declared slot.
func slotType(e ast.Expr) types.Type {
	p := rw.Up(e)
	switch x := p.(type) {
	case *ast.CallExpr:
		sig, ok := rw.Info.Types[x.Fun].Type.(*types.Signature)
		if !ok {
			return nil
		}
		for i, a := range x.Args {
			if rw.Unparen(a) != rw.Unparen(e) {
				continue
			}
			if sig.Variadic() && i >= sig.Params().Len()-1 {
				if x.Ellipsis.IsValid() {
					return sig.Params().At(sig.Params().Len() - 1).Type()
				}
				return sig.Params().At(sig.Params().Len() - 1).Type().(*types.Slice).Elem()
			}
			if i < sig.Params().Len() {
				return sig.Params().At(i).Type()
			}
		}
	case *ast.AssignStmt:
		if x.Tok == token.DEFINE || len(x.Lhs) != len(x.Rhs) {
			return nil
		}
		for i, r := range x.Rhs {
			if rw.Unparen(r) == rw.Unparen(e) {
				return rw.Info.Types[x.Lhs[i]].Type
			}
		}
	case *ast.ValueSpec:
		if x.Type != nil {
			return rw.Info.Types[x.Type].Type
		}
	case *ast.ReturnStmt:
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
				if rw.Unparen(r) == rw.Unparen(e) {
					return res.At(i).Type()
				}
			}
		}
	case *ast.CompositeLit:
		tv := rw.Info.Types[x]
		if tv.Type == nil {
			return nil
		}
		switch u := tv.Type.Underlying().(type) {
		case *types.Slice:
			return u.Elem()
		case *types.Struct:
			for i, el := range x.Elts {
				if rw.Unparen(el) == rw.Unparen(e) && i < u.NumFields() {
					return u.Field(i).Type()
				}
			}
		}
	case *ast.KeyValueExpr:
		if x.Value != e {
			return nil
		}
		if id, ok := x.Key.(*ast.Ident); ok {
			if o, ok := rw.Info.Uses[id].(*types.Var); ok {
				return o.Type()
			}
		}
	case *ast.BinaryExpr:
		if x.Op == token.EQL || x.Op == token.NEQ {
			other := x.X
			if rw.Unparen(x.X) == rw.Unparen(e) {
				other = x.Y
			}
			if t := rw.Info.Types[other].Type; t != nil && isIface(t) {
				return t
			}
		}
	}
	return nil
}

// redundantConv is the optConv_(x, func(v any) S {...}) call whose func literal starts at pos,
// if x is an Optional[S].
func redundantConv(pos token.Pos) *ast.CallExpr {
	var found *ast.CallExpr
	for _, f := range rw.Files {
		if rw.Fset.File(f.Pos()) != rw.Fset.File(pos) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			if x == nil || x.Pos() > pos || x.End() < pos {
				return false
			}
			c, ok := x.(*ast.CallExpr)
			if !ok || len(c.Args) != 2 || c.Args[1].Pos() != pos {
				return true
			}
			if id, ok := c.Fun.(*ast.Ident); !ok || id.Name != "optConv_" {
				return true
			}
			fl, ok := c.Args[1].(*ast.FuncLit)
			if !ok || fl.Type.Results == nil {
				return true
			}
			a := optArg(rw.Info.Types[c.Args[0]].Type)
			if a != nil && types.Identical(a, rw.Info.Types[fl.Type.Results.List[0].Type].Type) {
				found = c
			}
			return true
		})
	}
	return found
}

// retypeSlot gives the declaration that wants e as an Optional[any] the type Optional[S] when
// it is the result of a function (or of a method whose override group is not erased) or a local
// variable declared with e as its value. It reports whether it did.
func retypeSlot(e ast.Expr, s types.Type) bool {
	st := tstr(s)
	switch x := rw.Up(e).(type) {
	case *ast.ReturnStmt:
		if len(x.Results) != 1 {
			return false
		}
		for q := ast.Node(x); q != nil; q = rw.Parent[q] {
			switch fn := q.(type) {
			case *ast.FuncLit:
				return false
			case *ast.FuncDecl:
				o, _ := rw.Info.Defs[fn.Name].(*types.Func)
				if o == nil || fn.Type.Results == nil || len(fn.Type.Results.List) != 1 || pinned[groupKey(fn.Name.Name, o.Type().(*types.Signature))] {
					return false
				}
				r := fn.Type.Results.List[0].Type
				if _, done := rw.Rewrite[r]; done {
					return true
				}
				rw.Rewrite[r] = func() string { return "OptionalOf[" + st + "]" }
				stats["3: function results typed from what they return"]++
				return true
			}
		}
	case *ast.ValueSpec:
		if len(x.Values) == 1 && x.Type != nil && rw.Unparen(x.Values[0]) == rw.Unparen(e) {
			if _, done := rw.Rewrite[x.Type]; done {
				return true
			}
			rw.Rewrite[x.Type] = func() string { return "OptionalOf[" + st + "]" }
			stats["3: local variables typed from their value"]++
			return true
		}
	}
	return false
}

// unboxed is the code of e (an any that has a rewrite of its own) as a value of type t.
func unboxed(cv rw.Conv, t types.Type, e ast.Expr) string {
	if call, ok := rw.Unparen(e).(*ast.CallExpr); ok && len(call.Args) == 1 {
		if id, ok := call.Fun.(*ast.Ident); ok && (id.Name == "obj_" || id.Name == "strobj_" || id.Name == "any") {
			if tv, ok := rw.Info.Types[call.Args[0]]; ok && types.Identical(tv.Type, t) {
				return rw.Render(call.Args[0])
			}
		}
	}
	if id, ok := rw.Unparen(e).(*ast.Ident); ok && id.Name == "nil" {
		return cv.Unbox("nil")
	}
	return cv.Unbox(rw.RenderChildren(e))
}

// retype makes producer e (of an Optional[any]) produce an Optional[S], if it is one this tool
// can retype: Optional.of / ofNullable / empty, or a conditional expression (an immediately
// called func literal). It reports whether it did.
func retype(e ast.Expr, s types.Type) bool {
	e = rw.Unparen(e)
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	st := tstr(s)
	switch f := call.Fun.(type) {
	case *ast.Ident:
		if (f.Name == "Optional_of" || f.Name == "Optional_ofNullable") && len(call.Args) == 1 {
			cv := conv(s)
			a := call.Args[0]
			name := f.Name
			rw.Rewrite[call] = func() string {
				return name + "[" + st + "](" + rw.Unboxed(cv, s, a) + ")"
			}
			stats["3: Optional.of / ofNullable typed"]++
			return true
		}
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok && id.Name == "Optional_empty" {
			rw.Rewrite[f.Index] = func() string { return st }
			stats["3: Optional.empty typed"]++
			return true
		}
		if id, ok := f.X.(*ast.Ident); ok && (id.Name == "Optional_of" || id.Name == "Optional_ofNullable") && len(call.Args) == 1 {
			cv := conv(s)
			a := call.Args[0]
			name := id.Name
			rw.Rewrite[call] = func() string {
				return name + "[" + st + "](" + rw.Unboxed(cv, s, a) + ")"
			}
			stats["3: Optional.of / ofNullable typed"]++
			return true
		}
	case *ast.SelectorExpr:
		// o.filter(p) is o: retype o
		if f.Sel.Name == "filter" && len(call.Args) == 1 {
			if a := optArg(rw.Info.Types[f.X].Type); a != nil && isAny(a) {
				return retype(f.X, s)
			}
		}
	case *ast.FuncLit:
		if len(call.Args) == 0 && f.Type.Results != nil && len(f.Type.Results.List) == 1 {
			r := f.Type.Results.List[0].Type
			rw.Rewrite[r] = func() string { return "OptionalOf[" + st + "]" }
			stats["3: conditional expressions typed"]++
			return true
		}
	}
	return false
}

func fixErrors(errs []types.Error) int {
	n := 0
	done := map[ast.Node]bool{}
	mark := func(e ast.Node) bool {
		for q := e; q != nil; q = rw.Parent[q] {
			if done[q] {
				return false
			}
		}
		done[e] = true
		return true
	}
	var unknown []string
	for _, te := range errs {
		msg := te.Msg
		if x, wantText, ok := parseUse(msg); ok {
			e := exprAt(te.Pos, x)
			if e == nil || !generated(e) {
				unknown = append(unknown, rw.Fset.Position(te.Pos).String()+": "+msg)
				continue
			}
			have := rw.Info.Types[e].Type
			want := slotType(e)
			if have == nil || want == nil {
				unknown = append(unknown, rw.Fset.Position(te.Pos).String()+": "+msg+" (slot not found)")
				continue
			}
			if tstr(want) != wantText {
				fmt.Fprintf(os.Stderr, "note: %s: slot %s, message %s\n", rw.Fset.Position(te.Pos), tstr(want), wantText)
			}
			if !mark(e) {
				continue
			}
			hs, ws := optArg(have), optArg(want)
			switch {
			case hs != nil && ws != nil && isAny(hs) && !isAny(ws):
				if retype(e, ws) {
					n++
					continue
				}
				cv := conv(ws)
				wst := tstr(ws)
				rw.Rewrite[e] = func() string {
					return "optConv_(" + rw.RenderChildren(e) + ", func(v any) " + wst + " { return " + cv.Unbox("v") + " })"
				}
				stats["3: Optional[any] converted to Optional[S]"]++
				n++
			case hs != nil && ws != nil && isAny(ws) && retypeSlot(e, hs):
				n++
			case hs != nil && ws != nil && isAny(ws):
				rw.Rewrite[e] = func() string { return "optErase_(" + rw.RenderChildren(e) + ")" }
				stats["3: Optional[S] erased to Optional[any]"]++
				n++
			case hs != nil && ws != nil:
				cv := conv(ws)
				wst := tstr(ws)
				rw.Rewrite[e] = func() string {
					return "optConv_(optErase_(" + rw.RenderChildren(e) + "), func(v any) " + wst + " { return " + cv.Unbox("v") + " })"
				}
				stats["3: Optional[S] converted to Optional[T]"]++
				n++
			case isAny(have) && !isIface(want):
				cv := conv(want)
				w := want
				rw.Rewrite[e] = func() string { return unboxed(cv, w, e) }
				stats["3: any argument unboxed for a typed Optional"]++
				n++
			default:
				unknown = append(unknown, rw.Fset.Position(te.Pos).String()+": "+msg)
			}
			continue
		}
		if m := reNil.FindStringSubmatch(msg); m != nil && m[1] == "string" {
			e := exprAt(te.Pos, "nil")
			if e != nil && generated(e) && mark(e) {
				rw.Rewrite[e] = func() string { return "nullstr_" }
				stats["3: nil String argument as nullstr_"]++
				n++
				continue
			}
		}
		if m := reNotIface.FindStringSubmatch(msg); m != nil {
			e := exprAt(te.Pos, m[1])
			if e != nil && generated(e) && mark(e) {
				if t := rw.Info.Types[e].Type; t != nil {
					cv := conv(t)
					rw.Rewrite[e] = func() string { return cv.Box(rw.RenderChildren(e)) }
					stats["3: typed content boxed for a type assertion"]++
					n++
					continue
				}
			}
		}
		if strings.HasPrefix(msg, "in call to optConv_, type func(v any)") {
			// a conversion inserted in an earlier round whose operand has since been typed
			if c := redundantConv(te.Pos); c != nil && mark(c) {
				x := c.Args[0]
				rw.Rewrite[c] = func() string { return rw.Render(x) }
				stats["3: conversions made redundant by a later retyping removed"]++
				n++
				continue
			}
		}
		unknown = append(unknown, rw.Fset.Position(te.Pos).String()+": "+msg)
	}
	if n == 0 && len(unknown) > 0 {
		for _, u := range unknown {
			fmt.Fprintln(os.Stderr, "unrepaired:", u)
		}
		os.Exit(1)
	}
	return n
}

// ---- 4. uses

var contentMethods = map[string]bool{"get": true, "orElse": true, "orElseThrow": true}

// typedContent reports the content type S if e is get/orElse/orElseThrow on an Optional[S]
// with a non-interface S.
func typedContent(e ast.Expr) types.Type {
	call, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok {
		return nil
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || !contentMethods[sel.Sel.Name] {
		return nil
	}
	s := optArg(rw.Info.Types[sel.X].Type)
	if s == nil || isIface(s) {
		return nil
	}
	return s
}

func uses() int {
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
			// optErase_ of what is (now) an Optional[any]
			if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 {
				if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "optErase_" {
					if a := optArg(rw.Info.Types[c.Args[0]].Type); a != nil && isAny(a) {
						x := c.Args[0]
						rw.Rewrite[c] = func() string { return rw.Render(x) }
						stats["4: erasures made redundant by a later retyping removed"]++
						n++
						return false
					}
				}
			}
			// an Optional[S] boxed into an Object is erased first
			if s := optArg(rw.Info.Types[e].Type); s != nil && !isAny(s) {
				if st := slotType(e); st != nil && isIface(st) {
					rw.Rewrite[e] = func() string { return "optErase_(" + rw.RenderChildren(e) + ")" }
					stats["4: typed Optional erased before boxing"]++
					n++
					return false
				}
				if p, ok := rw.Up(e).(*ast.CallExpr); ok && len(p.Args) == 1 {
					if name, _ := rw.CallName(p); name == "any" || strings.HasPrefix(name, "obj_") {
						rw.Rewrite[e] = func() string { return "optErase_(" + rw.RenderChildren(e) + ")" }
						stats["4: typed Optional erased before boxing"]++
						n++
						return false
					}
				}
			}
			s := typedContent(e)
			if s == nil {
				// filter(lambda) on a typed Optional
				if call, ok := e.(*ast.CallExpr); ok {
					if filterFn(call) {
						n++
						return false
					}
				}
				return true
			}
			cv := conv(s)
			// the unboxing around it goes
			if p, ok := rw.Up(e).(*ast.CallExpr); ok && len(p.Args) == 1 && rw.Unparen(p.Args[0]) == rw.Unparen(e) {
				if name, _ := rw.CallName(p); name != "" && cv.IsUnbox(name) {
					rw.Rewrite[p] = func() string { return rw.Render(e) }
					stats["4: unboxing of typed content removed"]++
					n++
					return false
				}
			}
			// where it flows into an interface, it is boxed as before
			if st := slotType(e); st != nil && isIface(st) {
				rw.Rewrite[e] = func() string { return cv.Box(rw.RenderChildren(e)) }
				stats["4: typed content boxed where an Object is wanted"]++
				n++
				return false
			}
			return true
		})
	}
	return n
}

// filterFn rewrites o.filter(&Predicate_fn{func(s_ any) bool { var s S = UNBOX(s_); ... }}) on
// an Optional[S] to o.filterFn_(func(s S) bool { ... }).
func filterFn(call *ast.CallExpr) bool {
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != "filter" || len(call.Args) != 1 {
		return false
	}
	s := optArg(rw.Info.Types[sel.X].Type)
	if s == nil || isIface(s) {
		return false
	}
	u, ok := call.Args[0].(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return false
	}
	cl, ok := u.X.(*ast.CompositeLit)
	if !ok || types.ExprString(cl.Type) != "Predicate_fn" || len(cl.Elts) != 1 {
		return false
	}
	fl, ok := cl.Elts[0].(*ast.FuncLit)
	if !ok || len(fl.Type.Params.List) != 1 || len(fl.Type.Params.List[0].Names) != 1 || len(fl.Body.List) < 2 {
		return false
	}
	pn := fl.Type.Params.List[0].Names[0]
	ds, ok := fl.Body.List[0].(*ast.DeclStmt)
	if !ok {
		return false
	}
	gd := ds.Decl.(*ast.GenDecl)
	if len(gd.Specs) != 1 {
		return false
	}
	vs := gd.Specs[0].(*ast.ValueSpec)
	if len(vs.Names) != 1 || len(vs.Values) != 1 || vs.Type == nil || !types.Identical(rw.Info.Types[vs.Type].Type, s) {
		return false
	}
	cv := conv(s)
	uc, ok := vs.Values[0].(*ast.CallExpr)
	if !ok || len(uc.Args) != 1 {
		return false
	}
	if name, _ := rw.CallName(uc); !cv.IsUnbox(name) {
		return false
	}
	if id, ok := uc.Args[0].(*ast.Ident); !ok || id.Name != pn.Name {
		return false
	}
	// the parameter is used only there
	po := rw.Info.Defs[pn]
	uses := 0
	ast.Inspect(fl.Body, func(x ast.Node) bool {
		if id, ok := x.(*ast.Ident); ok && rw.Info.Uses[id] == po {
			uses++
		}
		return true
	})
	if uses != 1 {
		return false
	}
	name := vs.Names[0].Name
	st := tstr(s)
	rw.Rewrite[call] = func() string {
		var b strings.Builder
		b.WriteString(rw.Render(sel.X) + ".filterFn_(func(" + name + " " + st + ") bool {")
		f := rw.Fset.File(fl.Body.Pos())
		src := rw.Src[f]
		// the statements after the unboxing declaration, with their rewrites
		start := f.Offset(ds.End())
		end := f.Offset(fl.Body.Rbrace)
		var parts strings.Builder
		last := start
		for _, st := range fl.Body.List[1:] {
			a, z := f.Offset(st.Pos()), f.Offset(st.End())
			parts.WriteString(string(src[last:a]))
			parts.WriteString(rw.Render(st))
			last = z
		}
		parts.WriteString(string(src[last:end]))
		b.WriteString(parts.String())
		b.WriteString("})")
		return b.String()
	}
	stats["4: filter with an unboxing lambda as filterFn_"]++
	return true
}

func main() {
	if len(os.Args) > 2 && os.Args[1] == "-classes" {
		classesMain(os.Args[2:])
		return
	}
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	dir := os.Args[len(os.Args)-1]
	load(dir)
	if phase1() > 0 && !report {
		rw.Save()
	}
	for iter := 0; iter < 40; iter++ {
		errs := load(dir)
		n := unify()
		what := "overriding"
		if n == 0 && len(errs) > 0 {
			n, what = fixErrors(errs), "type errors"
		}
		if n == 0 && len(errs) == 0 {
			n, what = uses(), "uses"
		}
		fmt.Fprintf(os.Stderr, "round %d: %d errors, %d rewrites (%s)\n", iter, len(errs), n, what)
		if n == 0 {
			break
		}
		if report {
			break
		}
		rw.Save()
	}
	keys := make([]string, 0, len(stats))
	for k := range stats {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%6d %s\n", stats[k], k)
	}
}
