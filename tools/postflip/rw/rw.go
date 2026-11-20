// Package rw is the rewriting engine shared by the post-flip steps: it loads and type-checks
// package closure, and renders source Text with registered rewrites of sub-expressions applied.
package rw

import (
	"fmt"
	"go/ast"
	"go/importer"
	"go/parser"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

var (
	Fset    = token.NewFileSet()
	Info    = &types.Info{Types: map[ast.Expr]types.TypeAndValue{}, Defs: map[*ast.Ident]types.Object{}, Uses: map[*ast.Ident]types.Object{}, Selections: map[*ast.SelectorExpr]*types.Selection{}}
	Parent  = map[ast.Node]ast.Node{}
	Src     = map[*token.File][]byte{}
	Rewrite = map[ast.Node]func() string{}
	Pkg     *types.Package
)

func Text(n ast.Node) string {
	f := Fset.File(n.Pos())
	return string(Src[f][f.Offset(n.Pos()):f.Offset(n.End())])
}

// Render is the source of n with the rewrites of its descendants applied.
func Render(n ast.Node) string {
	if g, ok := Rewrite[n]; ok {
		return g()
	}
	return RenderChildren(n)
}

func RenderChildren(n ast.Node) string {
	f := Fset.File(n.Pos())
	base := f.Offset(n.Pos())
	s := Src[f][base:f.Offset(n.End())]
	var out strings.Builder
	last := 0
	ast.Inspect(n, func(c ast.Node) bool {
		if c == nil || c == n {
			return true
		}
		if g, ok := Rewrite[c]; ok {
			a, b := f.Offset(c.Pos())-base, f.Offset(c.End())-base
			out.Write(s[last:a])
			out.WriteString(g())
			last = b
			return false
		}
		return true
	})
	out.Write(s[last:])
	return out.String()
}

func Up(n ast.Node) ast.Node {
	p := Parent[n]
	for {
		if pe, ok := p.(*ast.ParenExpr); ok {
			p = Parent[pe]
			continue
		}
		return p
	}
}

func Unparen(e ast.Expr) ast.Expr {
	for {
		p, ok := e.(*ast.ParenExpr)
		if !ok {
			return e
		}
		e = p.X
	}
}

// Pure reports whether evaluating e twice is the same as evaluating it once.
func Pure(e ast.Expr) bool {
	switch x := Unparen(e).(type) {
	case *ast.Ident:
		return true
	case *ast.SelectorExpr:
		return Pure(x.X)
	case *ast.CallExpr:
		if s, ok := x.Fun.(*ast.SelectorExpr); ok && len(x.Args) == 0 && (s.Sel.Name == "ps_" || s.Sel.Name == "ns_") {
			return Pure(s.X)
		}
	}
	return false
}

func CallName(e ast.Expr) (string, *ast.CallExpr) {
	c, ok := Unparen(e).(*ast.CallExpr)
	if !ok {
		return "", nil
	}
	switch f := c.Fun.(type) {
	case *ast.Ident:
		return f.Name, c
	case *ast.IndexExpr:
		if id, ok := f.X.(*ast.Ident); ok {
			return id.Name + "[" + Text(f.Index) + "]", c
		}
	}
	return "", c
}

func HasFunc(name string) bool {
	_, ok := Pkg.Scope().Lookup(name).(*types.Func)
	return ok
}

func HasMethod(t types.Type, name string) bool {
	o, _, _ := types.LookupFieldOrMethod(t, true, Pkg, name)
	_, ok := o.(*types.Func)
	return ok
}

// Conv describes how a value of the element type is boxed into `any` and taken out again.
type Conv struct {
	Box   func(e string) string
	Unbox func(e string) string
	// isUnbox reports whether call c (with one argument) is an unboxing to exactly the element type.
	IsUnbox func(name string) bool
	OK      bool
}

func ConvFor(t types.Type) Conv {
	if ReflipGeneric_(t, true) {
		return Conv{IsUnbox: func(string) bool { return false }}
	}
	id := func(e string) string { return e }
	switch u := types.Unalias(t).(type) {
	case *types.Basic:
		if u.Kind() == types.String {
			return Conv{func(e string) string { return "strobj_(" + e + ")" }, func(e string) string { return "str_(" + e + ")" }, func(n string) bool { return n == "str_" }, true}
		}
	case *types.Interface:
		if u.Empty() {
			return Conv{id, id, func(string) bool { return false }, true}
		}
	case *types.Named:
		name := u.Obj().Name()
		if _, isIface := u.Underlying().(*types.Interface); isIface {
			if HasFunc("icast_" + name) {
				return Conv{func(e string) string { return "any(" + e + ")" }, func(e string) string { return "icast_" + name + "(" + e + ")" }, func(n string) bool { return n == "icast_"+name }, true}
			}
		} else if HasMethod(u, "box_") && HasFunc(name+"_of_") {
			return Conv{func(e string) string { return e + ".box_()" }, func(e string) string { return name + "_of_(" + e + ")" }, func(n string) bool { return n == name+"_of_" }, true}
		}
	case *types.Pointer:
		if n, ok := u.Elem().(*types.Named); ok {
			name := n.Obj().Name()
			box := func(e string) string { return "obj_(" + e + ")" }
			// self() gives the most derived object and is nil-safe where the class declares it;
			// a class that only inherits it has no subclasses, so the pointer is the object.
			if o, ix, _ := types.LookupFieldOrMethod(u, true, Pkg, "self"); o != nil && len(ix) == 1 {
				box = func(e string) string { return e + ".self()" }
			}
			un := "cast_[*" + name + "]"
			if HasFunc("to_" + name) {
				un = "to_" + name
			}
			return Conv{box, func(e string) string { return un + "(" + e + ")" }, func(n string) bool { return n == "cast_[*"+name+"]" || n == "to_"+name }, true}
		}
	}
	return Conv{}
}

// Unboxed returns the code storing boxed expression a (of type any) as a value of type t.
func Unboxed(cv Conv, t types.Type, a ast.Expr) string {
	a = Unparen(a)
	if tv, ok := Info.Types[a]; ok && tv.Type != nil && types.Identical(tv.Type, t) { // reflip21: a String passed as itself
		if b, ok := t.Underlying().(*types.Basic); ok && b.Kind() == types.String {
			return Render(a)
		}
	}
	// obj_(x), strobj_(x), any(x), x.self(), x.box_() of an x whose type is t: x itself.
	if call, ok := a.(*ast.CallExpr); ok {
		var inner ast.Expr
		if id, ok := call.Fun.(*ast.Ident); ok && len(call.Args) == 1 && (id.Name == "obj_" || id.Name == "strobj_" || id.Name == "any") {
			inner = call.Args[0]
		} else if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
			inner = s.X
		}
		if inner != nil {
			if tv, ok := Info.Types[inner]; ok && types.Identical(tv.Type, t) {
				return Render(inner)
			}
		}
	}
	return cv.Unbox(Render(a))
}

// Edit is one rewrite: node is replaced by Gen().
type Edit struct {
	Node ast.Node
	Gen  func() string
}

// Read returns the rewrite of a read n (of type any) whose unboxed value is code(): the
// enclosing unboxing call is replaced if it is the one for the type, else the value is boxed.
func Read(cv Conv, n ast.Expr, code func() string) Edit {
	if p, ok := Up(n).(*ast.CallExpr); ok && len(p.Args) == 1 && Unparen(p.Args[0]) == n {
		if name, _ := CallName(p); name != "" && cv.IsUnbox(name) {
			return Edit{p, code}
		}
	}
	return Edit{n, func() string { return cv.Box(code()) }}
}

// Files are the parsed files of the package.
var Files []*ast.File

// UsesOf maps an object to its uses, in source order.
var UsesOf = map[types.Object][]*ast.Ident{}

// Load parses and type-checks the package in dir.
func Load(dir string) {
	names, _ := filepath.Glob(filepath.Join(dir, "*.go"))
	for _, n := range names {
		if strings.HasSuffix(n, "_test.go") {
			continue
		}
		b, err := os.ReadFile(n)
		if err != nil {
			panic(err)
		}
		f, err := parser.ParseFile(Fset, n, b, parser.SkipObjectResolution)
		if err != nil {
			panic(err)
		}
		Src[Fset.File(f.Pos())] = b
		Files = append(Files, f)
	}
	conf := types.Config{Importer: importer.ForCompiler(Fset, "source", nil), Error: func(err error) { fmt.Fprintln(os.Stderr, err) }}
	var err error
	Pkg, err = conf.Check("closure", Fset, Files, Info)
	if err != nil {
		os.Exit(1)
	}
	for _, f := range Files {
		var stack []ast.Node
		ast.Inspect(f, func(n ast.Node) bool {
			if n == nil {
				stack = stack[:len(stack)-1]
				return true
			}
			if len(stack) > 0 {
				Parent[n] = stack[len(stack)-1]
			}
			stack = append(stack, n)
			return true
		})
	}
	// reflip: origin objects (a use of a generic type's field or method names its declaration)
	for id, o := range Info.Uses {
		switch v := o.(type) {
		case *types.Var:
			if v.Origin() != v {
				Info.Uses[id] = v.Origin()
			}
		case *types.Func:
			if v.Origin() != v {
				Info.Uses[id] = v.Origin()
			}
		}
	}
	for id, o := range Info.Uses {
		UsesOf[o] = append(UsesOf[o], id)
	}
	for _, us := range UsesOf {
		sort.Slice(us, func(i, j int) bool { return us[i].Pos() < us[j].Pos() })
	}
}

// Del, as the text of a rewrite of a statement, deletes the statement and its line.
const Del = "\x00del\x00"

// dropDeleted removes the lines that hold only a deleted statement.
func dropDeleted(s string) string {
	if !strings.Contains(s, Del) {
		return s
	}
	lines := strings.SplitAfter(s, "\n")
	out := lines[:0]
	for _, l := range lines {
		if strings.TrimSpace(l) == Del {
			continue
		}
		if strings.Contains(l, Del) {
			panic("a deleted statement shares its line: " + l)
		}
		out = append(out, l)
	}
	return strings.Join(out, "")
}

// Save writes back every file that the registered rewrites change, and returns their number.
func Save() int {
	changed := 0
	for _, f := range Files {
		tf := Fset.File(f.Pos())
		// The file node starts at `package`; keep what precedes it.
		out := dropDeleted(string(Src[tf][:tf.Offset(f.Pos())]) + RenderChildren(f) + string(Src[tf][tf.Offset(f.End()):]))
		if out != string(Src[tf]) {
			if err := os.WriteFile(tf.Name(), []byte(out), 0o644); err != nil {
				panic(err)
			}
			changed++
		}
	}
	return changed
}

// VarExpr is the expression naming a variable at use id: the identifier, or the selector whose
// field it is.
func VarExpr(id *ast.Ident) ast.Expr {
	if s, ok := Parent[id].(*ast.SelectorExpr); ok && s.Sel == id {
		return s
	}
	return id
}

// AliasArgs returns the type arguments of e if it is an instantiation of one of the named aliases.
func AliasArgs(e ast.Expr, names ...string) []ast.Expr {
	var x ast.Expr
	var args []ast.Expr
	switch ix := e.(type) {
	case *ast.IndexExpr:
		x, args = ix.X, []ast.Expr{ix.Index}
	case *ast.IndexListExpr:
		x, args = ix.X, ix.Indices
	default:
		return nil
	}
	if id, ok := x.(*ast.Ident); ok {
		for _, n := range names {
			if id.Name == n {
				return args
			}
		}
	}
	return nil
}

// ReflipGeneric_ reports whether t mentions a type parameter or (inst) an instance of a generic
// type.
func ReflipGeneric_(t types.Type, inst bool) bool {
	seen := map[types.Type]bool{}
	var walk func(t types.Type) bool
	walk = func(t types.Type) bool {
		if t == nil || seen[t] {
			return false
		}
		seen[t] = true
		switch u := t.(type) {
		case *types.TypeParam:
			return true
		case *types.Alias:
			for i := 0; i < u.TypeArgs().Len(); i++ {
				if walk(u.TypeArgs().At(i)) {
					return true
				}
			}
			return walk(types.Unalias(u))
		case *types.Named:
			if inst && u.TypeArgs().Len() > 0 {
				return true
			}
			for i := 0; i < u.TypeArgs().Len(); i++ {
				if walk(u.TypeArgs().At(i)) {
					return true
				}
			}
		case *types.Pointer:
			return walk(u.Elem())
		case *types.Slice:
			return walk(u.Elem())
		case *types.Array:
			return walk(u.Elem())
		case *types.Map:
			return walk(u.Key()) || walk(u.Elem())
		}
		return false
	}
	return walk(t)
}
