// Command jclass classifies every match of the census (census.py) by sub-cause, for JAVAISMS4.md.
// It reads the matches dumped by matches.py (row, file, byte offset, line, matched text, source
// line), finds the syntax node at each offset, and writes one line per match:
//
//	row TAB file TAB line TAB hot TAB sub-cause TAB detail TAB extra TAB matched text
//
// It rewrites nothing. usage: jclass <dir of package closure> <matches.tsv> > sites.tsv
package main

import (
	"bufio"
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

	"postflip/anyflow"
	"postflip/rw"
)

var fileOf = map[string]*ast.File{}
var tfileOf = map[string]*token.File{}

func ts(t types.Type) string {
	if t == nil {
		return "?"
	}
	return types.TypeString(t, func(*types.Package) string { return "" })
}

// pathAt: the nodes of file f that contain byte offset off, outermost first.
func pathAt(file string, off int) []ast.Node {
	f := fileOf[file]
	tf := tfileOf[file]
	if f == nil {
		return nil
	}
	pos := tf.Pos(off)
	var path []ast.Node
	ast.Inspect(f, func(n ast.Node) bool {
		if n == nil {
			return false
		}
		if n.Pos() <= pos && pos < n.End() {
			path = append(path, n)
			return true
		}
		return false
	})
	return path
}

func pathOfPos(p token.Pos) []ast.Node {
	pp := rw.Fset.Position(p)
	return pathAt(filepath.Base(pp.Filename), pp.Offset)
}

var hotFiles = map[string]string{}

func initHot() {
	for _, n := range []string{"TypeInference", "TypeCheck", "TypedScopeCreator", "NodeTraversal", "ControlFlowAnalysis", "DataFlowAnalysis", "Node", "JSType"} {
		hotFiles["g_"+n+".go"] = "H"
	}
	// the JSType classes: the struct types that embed JSType, transitively
	if o, ok := rw.Pkg.Scope().Lookup("JSType").(*types.TypeName); ok {
		for _, s := range anyflow.WithSubs(o) {
			hotFiles[filepath.Base(rw.Fset.Position(s.Pos()).Filename)] = "H"
		}
	}
	// near the list (my additions, marked h): the type checker's helpers
	for _, n := range []string{"SubtypeChecker", "EqualityChecker", "JSTypeRegistry", "TemplateTypeMap", "TemplateTypeReplacer", "TypeValidator", "TypedScope", "TypedVar", "AbstractScope", "AbstractVar", "Scope", "Var", "ScopedName", "LinkedFlowScope", "FlowScope", "SemanticReverseAbstractInterpreter", "ChainableReverseAbstractInterpreter", "ControlFlowGraph", "LinkedDirectedGraph", "DiGraph", "Graph", "FunctionTypeBuilder", "JSTypeExpression", "NodeUtil", "PropertyMap", "Property", "InvocationTemplateTypeMatcher", "TypeInferencePass", "TypeCheckPass", "UnionType_Builder", "JSDocInfo"} {
		if hotFiles["g_"+n+".go"] == "" {
			hotFiles["g_"+n+".go"] = "h"
		}
	}
}

func hot(file string) string {
	if h := hotFiles[file]; h != "" {
		return h
	}
	return "-"
}

var ifaceMethods = map[string]bool{}

func enclosingFunc(path []ast.Node) (decl *ast.FuncDecl, lit *ast.FuncLit) {
	for i := len(path) - 1; i >= 0; i-- {
		switch x := path[i].(type) {
		case *ast.FuncLit:
			if lit == nil {
				lit = x
			}
		case *ast.FuncDecl:
			return x, lit
		}
	}
	return nil, lit
}

func isHelperName(n string) bool {
	return strings.HasPrefix(n, "icast_") || strings.HasPrefix(n, "to_") || strings.HasPrefix(n, "gcast_") || strings.HasPrefix(n, "is_") || n == "Node_of_"
}

func unwrap(x ast.Expr) ast.Expr {
	x = rw.Unparen(x)
	for {
		if cc, ok := x.(*ast.CallExpr); ok && len(cc.Args) == 1 {
			if tv, ok := rw.Info.Types[cc.Fun]; ok && tv.IsType() {
				x = rw.Unparen(cc.Args[0])
				continue
			}
		}
		break
	}
	return x
}

func recvName(recv ast.Expr) string {
	if recv == nil {
		return ""
	}
	t := rw.Info.Types[recv].Type
	if tn := anyflow.Named(t); tn != nil {
		return tn.Name()
	}
	return ts(t)
}

func declMethodKind(fd *ast.FuncDecl) string {
	if fd.Recv == nil {
		return "function " + fd.Name.Name
	}
	base := fd.Name.Name
	if ifaceMethods[base] {
		return "method listed by an interface: " + base
	}
	return "method " + base
}

// srcOf says where the value of an Object-typed expression comes from.
var carrier types.Type

// known: does the carrier's alias (XOf[T]) name the target type as a type argument?
func known(target types.Type) string {
	c := carrier
	if c == nil {
		return "no carrier type"
	}
	want := ts(target)
	found := ""
	var walk func(t types.Type, d int)
	walk = func(t types.Type, d int) {
		if t == nil || d > 4 || found != "" {
			return
		}
		var args *types.TypeList
		switch x := t.(type) {
		case *types.Alias:
			args = x.TypeArgs()
			if args == nil || args.Len() == 0 {
				walk(x.Rhs(), d+1)
			}
		case *types.Named:
			args = x.TypeArgs()
		case *types.Pointer:
			walk(x.Elem(), d+1)
		case *types.Signature:
			for i := 0; i < x.Results().Len(); i++ {
				walk(x.Results().At(i).Type(), d+1)
			}
		}
		if args != nil {
			for i := 0; i < args.Len(); i++ {
				if ts(args.At(i)) == want {
					found = ts(t)
					return
				}
				walk(args.At(i), d+1)
			}
		}
	}
	walk(c, 0)
	if found != "" {
		return "ELEMENT TYPE KNOWN statically: carrier " + strings.SplitN(ts(c), "[", 2)[0] + "[..] names the target"
	}
	cs := ts(c)
	if strings.Contains(cs, "[") {
		return "carrier has type arguments but not the target: " + cs
	}
	return "carrier is raw: " + cs
}

func srcOf(x ast.Expr, depth int) (kind, detail string) {
	if depth == 0 {
		carrier = nil
	}
	x = unwrap(x)
	xt := rw.Info.Types[x].Type
	switch e := x.(type) {
	case *ast.CallExpr:
		obj, name, recv := anyflow.Callee(e)
		fo, _ := obj.(*types.Func)
		switch {
		case fo == nil:
			carrier = rw.Info.Types[rw.Unparen(e.Fun)].Type
			if id, ok := rw.Unparen(e.Fun).(*ast.Ident); ok {
				if o := rw.Info.Uses[id]; o != nil {
					carrier = o.Type()
				}
			}
			return "result of a func value (a lambda's Object result)", name
		case name == "self":
			return "self() of a typed value (down/cross-cast in a class hierarchy)", recvName(recv)
		case name == "obj_" || name == "box_" || name == "strobj_" || name == "boxT_":
			at := ""
			if len(e.Args) == 1 {
				at = ts(rw.Info.Types[e.Args[0]].Type)
			} else if recv != nil {
				at = ts(rw.Info.Types[recv].Type)
			}
			return "typed value boxed on the spot (" + name + ")", at
		case !anyflow.InG(fo.Pos()):
			k := name
			if recv != nil {
				k = recvName(recv) + "." + name
				carrier = rw.Info.Types[recv].Type
			} else if len(e.Args) > 0 {
				carrier = rw.Info.Types[unwrap(e.Args[0])].Type
			}
			return "Object result of a runtime function/method (erased collection element, property bag, cache)", k
		default:
			k := name
			if recv != nil {
				k = recvName(recv) + "." + name
				carrier = rw.Info.Types[recv].Type
			}
			if sig := fo.Type().(*types.Signature); sig.Recv() != nil && types.IsInterface(sig.Recv().Type()) {
				return "Object result of a method called through a Java interface of the program", k
			}
			return "Object result of a generated function/method", k
		}
	case *ast.Ident, *ast.SelectorExpr:
		var o types.Object
		if id, ok := e.(*ast.Ident); ok {
			o = rw.Info.Uses[id]
		} else {
			o = rw.Info.Uses[e.(*ast.SelectorExpr).Sel]
		}
		v, _ := o.(*types.Var)
		if v == nil {
			return "other", rw.Text(x)
		}
		ifc := "Object (any)"
		if xt != nil && !isEmptyIface(xt) {
			ifc = "a Java interface"
		}
		if v.IsField() {
			if v.Name() == "v" {
				if se, ok := e.(*ast.SelectorExpr); ok {
					if tn := anyflow.Named(rw.Info.Types[se.X].Type); tn != nil && tn.Name() == "each_" {
						return "element of a for-each over an erased collection (cursor it.v)", cursorSource(se.X)
					}
				}
			}
			own := ""
			if se, ok := e.(*ast.SelectorExpr); ok {
				own = recvName(se.X) + "."
			}
			return "field declared " + ifc, own + v.Name() + " " + ts(xt)
		}
		return varKind(v, ifc, depth)
	case *ast.IndexExpr:
		return "element of a slice or map of Objects", ts(rw.Info.Types[e.X].Type)
	case *ast.TypeAssertExpr:
		return "a type assertion's value", rw.Text(e.Type)
	}
	return "other", fmt.Sprintf("%T", x)
}

func isEmptyIface(t types.Type) bool {
	i, ok := t.Underlying().(*types.Interface)
	return ok && i.NumMethods() == 0 && !isTP(t)
}

func isTP(t types.Type) bool {
	_, ok := types.Unalias(t).(*types.TypeParam)
	return ok
}

// cursorSource: for `it := foreach_(X)` the type of X.
func cursorSource(it ast.Expr) string {
	id, ok := rw.Unparen(it).(*ast.Ident)
	if !ok {
		return "?"
	}
	o := rw.Info.Uses[id]
	if o == nil {
		return "?"
	}
	path := pathOfPos(o.Pos())
	for i := len(path) - 1; i >= 0; i-- {
		if as, ok := path[i].(*ast.AssignStmt); ok && len(as.Rhs) == 1 {
			if c, ok := as.Rhs[0].(*ast.CallExpr); ok {
				_, name, recv := anyflow.Callee(c)
				if name == "foreach_" && len(c.Args) == 1 {
					carrier = rw.Info.Types[unwrap(c.Args[0])].Type
					return "foreach_(" + ts(rw.Info.Types[unwrap(c.Args[0])].Type) + ")"
				}
				if recv != nil {
					return recvName(recv) + "." + name + "()"
				}
				return name
			}
		}
	}
	return "?"
}

func varKind(v *types.Var, ifc string, depth int) (string, string) {
	path := pathOfPos(v.Pos())
	if len(path) == 0 {
		return "variable declared " + ifc, "?"
	}
	// parameter?
	for i := len(path) - 1; i >= 0; i-- {
		switch x := path[i].(type) {
		case *ast.FieldList:
			if i > 0 {
				if ft, ok := path[i-1].(*ast.FuncType); ok && ft.Params == x {
					if i > 1 {
						switch fn := path[i-2].(type) {
						case *ast.FuncLit:
							return "parameter of a func literal declared " + ifc, litContext(fn)
						case *ast.FuncDecl:
							return "parameter declared " + ifc + " of a " + declMethodKindShort(fn), declMethodKind(fn)
						}
					}
				}
			}
		case *ast.RangeStmt:
			if depth < 2 {
				src := unwrap(x.X)
				if c, ok := src.(*ast.CallExpr); ok {
					_, name, _ := anyflow.Callee(c)
					arg := ""
					if len(c.Args) > 0 {
						arg = ts(rw.Info.Types[unwrap(c.Args[0])].Type)
						carrier = rw.Info.Types[unwrap(c.Args[0])].Type
					}
					return "range variable over an erased collection", name + "(" + arg + ")"
				}
				return "range variable over an erased collection", ts(rw.Info.Types[src].Type)
			}
		case *ast.AssignStmt:
			if x.Tok == token.DEFINE && len(x.Rhs) == 1 && len(x.Lhs) == 1 && depth < 2 {
				k, d := srcOf(x.Rhs[0], depth+1)
				return "local " + ifc + " initialised from: " + k, d
			}
			if x.Tok == token.DEFINE {
				return "local " + ifc + " (tuple assignment)", rw.Text(x.Rhs[0])
			}
		case *ast.ValueSpec:
			if len(x.Values) == 1 && depth < 2 {
				k, d := srcOf(x.Values[0], depth+1)
				return "local " + ifc + " initialised from: " + k, d
			}
			return "local " + ifc + " declared without a value", v.Name()
		case *ast.TypeSwitchStmt:
			return "type switch variable", v.Name()
		case *ast.FuncDecl, *ast.FuncLit:
			return "variable declared " + ifc, v.Name()
		}
	}
	return "variable declared " + ifc, v.Name()
}

func declMethodKindShort(fd *ast.FuncDecl) string {
	if fd.Recv == nil {
		return "generated function"
	}
	if ifaceMethods[fd.Name.Name] {
		return "method listed by an interface (erased signature)"
	}
	return "generated method"
}

// litContext: what a func literal is given to.
func litContext(lit *ast.FuncLit) string {
	p := rw.Parent[lit]
	for {
		if pe, ok := p.(*ast.ParenExpr); ok {
			p = rw.Parent[pe]
			continue
		}
		break
	}
	switch x := p.(type) {
	case *ast.CompositeLit:
		return "adapter literal " + ts(rw.Info.Types[x].Type)
	case *ast.KeyValueExpr:
		if cl, ok := rw.Parent[x].(*ast.CompositeLit); ok {
			return "field of " + ts(rw.Info.Types[cl].Type)
		}
	case *ast.CallExpr:
		_, name, recv := anyflow.Callee(x)
		if recv != nil {
			return "argument of " + recvName(recv) + "." + name
		}
		return "argument of " + name
	case *ast.AssignStmt, *ast.ValueSpec:
		return "assigned to a variable of type " + ts(rw.Info.Types[lit].Type)
	case *ast.ReturnStmt:
		return "returned"
	}
	return fmt.Sprintf("%T", p)
}

var isRx = regexp.MustCompile(`(!?)is_(\[[^\]]*\]|\w+)\(`)

// guard: is the cast of operand (text t) at node n protected by an instanceof test of the same
// expression (an enclosing if / && / case, or an earlier `if !is_X(t) { leave }`)?
func guard(n ast.Node, t string) string {
	pos := "is_" // marker
	_ = pos
	has := func(cond string) (posTest, negTest bool) {
		for _, m := range isRx.FindAllStringSubmatchIndex(cond, -1) {
			rest := cond[m[1]:]
			if strings.HasPrefix(rest, t+")") {
				if cond[m[2]:m[3]] == "!" {
					negTest = true
				} else {
					posTest = true
				}
			}
		}
		// comma-ok or getClass comparisons of the same value
		if strings.Contains(cond, "Object_getClass("+t+")") || strings.Contains(cond, t+".getClass()") {
			posTest = true
		}
		return
	}
	child := n
	for p := rw.Parent[n]; p != nil; child, p = p, rw.Parent[p] {
		switch x := p.(type) {
		case *ast.IfStmt:
			c := rw.Text(x.Cond)
			ps, ng := has(c)
			if child == ast.Node(x.Body) && ps {
				return "inside `if is_X(v)`"
			}
			if child == x.Else && ng {
				return "inside the else of `if !is_X(v)`"
			}
		case *ast.BinaryExpr:
			if x.Op == token.LAND && child == ast.Node(x.Y) {
				if ps, _ := has(rw.Text(x.X)); ps {
					return "right of `is_X(v) &&`"
				}
			}
			if x.Op == token.LOR && child == ast.Node(x.Y) {
				if _, ng := has(rw.Text(x.X)); ng {
					return "right of `!is_X(v) ||`"
				}
			}
		case *ast.CaseClause:
			for _, e := range x.List {
				if ps, _ := has(rw.Text(e)); ps {
					return "inside `case is_X(v)`"
				}
			}
		case *ast.ForStmt:
			if x.Cond != nil && child == ast.Node(x.Body) {
				if ps, _ := has(rw.Text(x.Cond)); ps {
					return "inside `for is_X(v)`"
				}
			}
		case *ast.BlockStmt:
			for _, s := range x.List {
				if s == child {
					break
				}
				if is, ok := s.(*ast.IfStmt); ok && is.Else == nil && len(is.Body.List) > 0 {
					last := is.Body.List[len(is.Body.List)-1]
					leaves := false
					switch l := last.(type) {
					case *ast.ReturnStmt, *ast.BranchStmt:
						leaves = true
					case *ast.ExprStmt:
						if c, ok := l.X.(*ast.CallExpr); ok {
							if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "panic" {
								leaves = true
							}
						}
					}
					if leaves {
						if _, ng := has(rw.Text(is.Cond)); ng {
							return "after `if !is_X(v) { leave }`"
						}
						c := rw.Text(is.Cond)
						if strings.Contains(c, "getClass") && strings.Contains(c, t) && strings.Contains(c, "!=") {
							return "after `if getClass() != v.getClass() { leave }`"
						}
					}
				}
			}
		case *ast.FuncDecl, *ast.FuncLit:
			return ""
		}
	}
	return ""
}

func targetKind(t types.Type) string {
	if t == nil {
		return "?"
	}
	if isTP(t) {
		return "a type parameter"
	}
	if types.IsInterface(t) {
		return "interface"
	}
	if p, ok := types.Unalias(t).(*types.Pointer); ok {
		if n, ok := types.Unalias(p.Elem()).(*types.Named); ok {
			if _, ok := n.Underlying().(*types.Struct); ok {
				g := ""
				if n.TypeArgs() != nil && n.TypeArgs().Len() > 0 {
					g = ", generic instance"
				}
				if len(anyflow.Subs[n.Origin().Obj()]) > 0 {
					return "class with subclasses" + g
				}
				return "leaf class" + g
			}
		}
	}
	return "value type (" + ts(t.Underlying()) + ")"
}

type out struct {
	sub, detail, extra string
}

func castSite(path []ast.Node, row int) out {
	// innermost ident at the offset
	var id *ast.Ident
	for i := len(path) - 1; i >= 0; i-- {
		if x, ok := path[i].(*ast.Ident); ok {
			id = x
			break
		}
	}
	if id == nil {
		return out{"UNCLASSIFIED", "no identifier", ""}
	}
	fd, _ := enclosingFunc(path)
	if fd != nil && fd.Name == id {
		return out{"helper declaration (func " + helperFamily(id.Name) + "(v any))", id.Name, ""}
	}
	// the call whose function this identifier is
	var call *ast.CallExpr
	var n ast.Node = id
	for p := rw.Parent[n]; p != nil; n, p = p, rw.Parent[p] {
		if ix, ok := p.(*ast.IndexExpr); ok && ix.X == n {
			continue
		}
		if ix, ok := p.(*ast.IndexListExpr); ok && ix.X == n {
			continue
		}
		if c, ok := p.(*ast.CallExpr); ok && c.Fun == n {
			call = c
		}
		break
	}
	if call == nil {
		// a function value
		ctx := "other"
		switch p := rw.Parent[n].(type) {
		case *ast.KeyValueExpr:
			if cl, ok := rw.Parent[p].(*ast.CompositeLit); ok {
				ctx = "field `" + rw.Text(p.Key) + "` of a " + strings.SplitN(ts(rw.Info.Types[cl].Type), "[", 2)[0] + " element-conversion descriptor"
			}
		case *ast.CallExpr:
			_, name, _ := anyflow.Callee(p)
			ctx = "argument of " + name
		}
		return out{"cast function used as a value (not a cast site): " + ctx, rw.Text(n.(ast.Expr)), ""}
	}
	if fd != nil && fd.Recv == nil && isHelperName(fd.Name.Name) {
		return out{"inside a cast helper's body (" + helperFamily(fd.Name.Name) + ": the fallback cast)", fd.Name.Name, ""}
	}
	tt := rw.Info.Types[call].Type
	k, d := srcOf(call.Args[0], 0)
	g := guard(call, rw.Text(unwrap(call.Args[0])))
	if g == "" {
		g = guard(call, rw.Text(rw.Unparen(call.Args[0])))
	}
	if g == "" {
		g = "no instanceof test of the same expression in scope"
	} else {
		g = "GUARDED: " + g
	}
	return out{k, d, targetKind(tt) + " | " + g + " | target " + ts(tt) + " | " + known(tt)}
}

func helperFamily(n string) string {
	for _, p := range []string{"icast_", "gcast_", "to_", "is_"} {
		if strings.HasPrefix(n, p) {
			return p + "X"
		}
	}
	return n
}

func assertSite(path []ast.Node, off token.Pos) out {
	var ta *ast.TypeAssertExpr
	for i := len(path) - 1; i >= 0; i-- {
		if x, ok := path[i].(*ast.TypeAssertExpr); ok && x.Lparen == off+1 {
			ta = x
			break
		}
	}
	if ta == nil {
		return out{"UNCLASSIFIED", "no type assertion at offset", ""}
	}
	fd, _ := enclosingFunc(path)
	tt := rw.Info.Types[ta.Type].Type
	tk := targetKind(tt)
	if types.IsInterface(tt) && strings.HasSuffix(rw.Text(ta.Type), "_as") {
		tk = "X_as interface (class with subclasses)"
	}
	xtxt := rw.Text(ta.X)
	commaok := false
	form := "direct v.(T)"
	switch p := rw.Parent[ta].(type) {
	case *ast.AssignStmt:
		if len(p.Lhs) == 2 && len(p.Rhs) == 1 {
			commaok = true
			form = "comma-ok"
			if id, ok := p.Lhs[1].(*ast.Ident); ok && id.Name == "_" {
				form = "p, _ := v.(T)"
			}
		}
	case *ast.ValueSpec:
		if len(p.Names) == 2 {
			commaok = true
			form = "comma-ok"
		}
	}
	_ = commaok
	if fd != nil && fd.Recv == nil && (isHelperName(fd.Name.Name) || fd.Name.Name == "cast_" || fd.Name.Name == "is_") {
		return out{"body of a cast/instanceof helper (" + helperFamily(fd.Name.Name) + ")", fd.Name.Name, tk + " | " + form}
	}
	if strings.HasSuffix(xtxt, ".self_") || xtxt == "self_" {
		xt := rw.Info.Types[ta.X].Type
		slot := "self_ typed " + ts(xt)
		if isEmptyIface(xt) {
			slot = "self_ typed any"
		}
		if form == "p, _ := v.(T)" {
			return out{"virtual dispatch: `if p, _ := this.self_.(*C); p == this` exact-class test before a direct call", slot, tk + " | " + form}
		}
		if form == "direct v.(T)" {
			if _, ok := rw.Parent[ta].(*ast.SelectorExpr); ok {
				return out{"virtual dispatch: this.self_.(T).m() (call through self_ after an assertion)", slot, tk + " | " + form}
			}
		}
		return out{"other use of this.self_.(T)", slot, tk + " | " + form}
	}
	k, d := srcOf(ta.X, 0)
	if form != "direct v.(T)" {
		return out{"instanceof as comma-ok assertion (Go idiom) of: " + k, d, tk + " | " + form + " | target " + ts(tt)}
	}
	g := guard(ta, rw.Text(unwrap(ta.X)))
	if g == "" {
		g = "no instanceof test of the same expression in scope"
	} else {
		g = "GUARDED: " + g
	}
	return out{"Java cast folded to a direct assertion of: " + k, d, tk + " | " + g + " | target " + ts(tt) + " | " + known(tt)}
}

func callAt(path []ast.Node, pos token.Pos) *ast.CallExpr {
	for i := len(path) - 1; i >= 0; i-- {
		if c, ok := path[i].(*ast.CallExpr); ok {
			f := rw.Unparen(c.Fun)
			if ix, ok := f.(*ast.IndexExpr); ok {
				f = ix.X
			}
			if ix, ok := f.(*ast.IndexListExpr); ok {
				f = ix.X
			}
			switch x := f.(type) {
			case *ast.Ident:
				if x.Pos() == pos {
					return c
				}
			case *ast.SelectorExpr:
				if x.Sel.Pos() == pos || x.Sel.Pos() == pos+1 || x.Pos() == pos {
					return c
				}
			}
		}
	}
	return nil
}

// sinkOf: what an expression's value is given to.
func sinkOf(e ast.Expr) (string, string) {
	var n ast.Node = e
	p := rw.Parent[n]
	for {
		if pe, ok := p.(*ast.ParenExpr); ok {
			n, p = pe, rw.Parent[pe]
			continue
		}
		if c, ok := p.(*ast.CallExpr); ok && len(c.Args) == 1 && c.Args[0] == n {
			if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() {
				n, p = c, rw.Parent[c]
				continue
			}
		}
		break
	}
	switch x := p.(type) {
	case *ast.CallExpr:
		if x.Fun == n {
			return "called", ""
		}
		obj, name, recv := anyflow.Callee(x)
		fo, _ := obj.(*types.Func)
		k := name
		if recv != nil {
			k = recvName(recv) + "." + name
		}
		if fo == nil {
			if name == "append" || name == "panic" {
				return "argument of builtin " + name, name
			}
			return "argument of a func value", k
		}
		where := "generated"
		if !anyflow.InG(fo.Pos()) {
			where = "runtime"
		}
		if sig := fo.Type().(*types.Signature); sig.Recv() != nil && types.IsInterface(sig.Recv().Type()) {
			where += " interface"
		}
		kind := "function"
		if recv != nil {
			kind = "method"
		}
		return "argument of a " + where + " " + kind, k
	case *ast.SelectorExpr:
		if x.X == n {
			if c, ok := rw.Parent[x].(*ast.CallExpr); ok && c.Fun == ast.Node(x) {
				obj := rw.Info.Uses[x.Sel]
				where := "generated"
				if obj != nil && !anyflow.InG(obj.Pos()) {
					where = "runtime"
				}
				return "receiver of a " + where + " method call", recvName(e) + "." + x.Sel.Name
			}
			return "receiver of a field/method selection", x.Sel.Name
		}
	case *ast.AssignStmt:
		for i, r := range x.Rhs {
			if r == n && i < len(x.Lhs) {
				l := rw.Unparen(x.Lhs[i])
				lt := ts(rw.Info.Types[l].Type)
				if x.Tok == token.DEFINE {
					if id, ok := l.(*ast.Ident); ok {
						if o := rw.Info.Defs[id]; o != nil {
							lt = ts(o.Type())
						}
					}
					return "assigned to a new local", lt
				}
				switch l := l.(type) {
				case *ast.SelectorExpr:
					return "assigned to a field", recvName(l.X) + "." + l.Sel.Name + " " + lt
				case *ast.IndexExpr:
					return "assigned to an element", ts(rw.Info.Types[l.X].Type)
				}
				return "assigned to a variable", lt
			}
		}
		return "assignment", ""
	case *ast.ValueSpec:
		t := ""
		if x.Type != nil {
			t = rw.Text(x.Type)
		}
		return "initialises a declared variable", t
	case *ast.ReturnStmt:
		for q := ast.Node(x); q != nil; q = rw.Parent[q] {
			switch fn := q.(type) {
			case *ast.FuncLit:
				return "returned from a func literal", litContext(fn)
			case *ast.FuncDecl:
				return "returned from a " + declMethodKindShort(fn), declMethodKind(fn)
			}
		}
	case *ast.CompositeLit:
		return "element of a composite literal", ts(rw.Info.Types[x].Type)
	case *ast.KeyValueExpr:
		if cl, ok := rw.Parent[x].(*ast.CompositeLit); ok {
			return "field/element of a composite literal", ts(rw.Info.Types[cl].Type) + "." + rw.Text(x.Key)
		}
	case *ast.BinaryExpr:
		return "operand of " + x.Op.String(), rw.Text(x)
	case *ast.RangeStmt:
		return "ranged over", ""
	case *ast.ExprStmt:
		return "expression statement (result unused)", ""
	case *ast.IfStmt, *ast.ForStmt, *ast.SwitchStmt:
		return "condition/tag", ""
	case *ast.CaseClause:
		return "case value", ""
	case *ast.TypeAssertExpr:
		return "operand of a type assertion", rw.Text(x.Type)
	case *ast.UnaryExpr:
		return "operand of " + x.Op.String(), ""
	case *ast.IndexExpr:
		return "index/indexed", ""
	case *ast.SendStmt, *ast.DeferStmt, *ast.GoStmt:
		return fmt.Sprintf("%T", p), ""
	}
	return fmt.Sprintf("other (%T)", p), ""
}

func identAt(path []ast.Node) *ast.Ident {
	for i := len(path) - 1; i >= 0; i-- {
		if x, ok := path[i].(*ast.Ident); ok {
			return x
		}
	}
	return nil
}

// generic site: the call (or use) at the match and its sink
func callSite(path []ast.Node, pos token.Pos, matched string) out {
	id := identAt(path)
	fd, _ := enclosingFunc(path)
	if id != nil && fd != nil && fd.Name == id {
		return out{"declaration of the function itself", id.Name, ""}
	}
	c := callAt(path, pos)
	if c == nil {
		// find any call whose Fun contains pos
		for i := len(path) - 1; i >= 0; i-- {
			if cc, ok := path[i].(*ast.CallExpr); ok && cc.Fun.Pos() <= pos+1 && pos < cc.Fun.End() {
				c = cc
				break
			}
		}
	}
	if c == nil {
		if id != nil {
			if o := rw.Info.Uses[id]; o != nil {
				if _, ok := o.(*types.TypeName); ok {
					return out{"type name", id.Name, ""}
				}
			}
			if o := rw.Info.Defs[id]; o != nil {
				return out{"declaration", id.Name + " " + fmt.Sprintf("%T", o), ""}
			}
			return out{"not a call (value or name)", id.Name, ""}
		}
		return out{"UNCLASSIFIED", "no call", ""}
	}
	k, d := sinkOf(c)
	at := ""
	if len(c.Args) > 0 {
		at = ts(rw.Info.Types[c.Args[0]].Type)
	}
	rt := ts(rw.Info.Types[c].Type)
	return out{k, d, "arg0 " + at + " | result " + rt}
}

// ---- any slots ----

var reachMemo = map[types.Object]map[string]bool{}
var pred = map[types.Object][]types.Object{}

func buildPred() {
	for from, tos := range anyflow.Succ {
		for _, to := range tos {
			pred[to] = append(pred[to], from)
		}
	}
}

// reach: the concrete types the model sees stored into slot o (through any chain of slots).
func reach(o types.Object) (set map[string]bool, slots int) {
	set = map[string]bool{}
	seen := map[types.Object]bool{}
	var walk func(o types.Object)
	walk = func(o types.Object) {
		if seen[o] {
			return
		}
		seen[o] = true
		if tn, ok := o.(*types.TypeName); ok {
			set["<type parameter "+tn.Name()+" of "+anyflow.TPOwner[tn]+">"] = true
		}
		for _, c := range anyflow.ClassInto[o] {
			set["*"+c.Name()] = true
		}
		for _, t := range anyflow.TypesInto[o] {
			set[ts(t)] = true
		}
		for _, p := range pred[o] {
			walk(p)
		}
	}
	walk(o)
	return set, len(seen)
}

func anySlot(path []ast.Node, matched string) out {
	// the declared identifier: the first identifier of the matched text
	var id *ast.Ident
	for i := len(path) - 1; i >= 0; i-- {
		if x, ok := path[i].(*ast.Ident); ok {
			id = x
			break
		}
	}
	if id == nil {
		return out{"UNCLASSIFIED", "no identifier", ""}
	}
	o := rw.Info.Defs[id]
	v, _ := o.(*types.Var)
	if v == nil {
		if u := rw.Info.Uses[id]; u != nil {
			return out{"not a declaration: use of " + fmt.Sprintf("%T", u), id.Name, ""}
		}
		return out{"not a variable declaration", id.Name, ""}
	}
	if !isEmptyIface(v.Type()) {
		return out{"not an Object slot: declared " + ts(v.Type()), id.Name, ""}
	}
	var kind, detail string
	switch {
	case v.IsField():
		kind = "field"
		for i := len(path) - 1; i >= 0; i-- {
			if tsp, ok := path[i].(*ast.TypeSpec); ok {
				detail = tsp.Name.Name + "." + v.Name()
				break
			}
		}
	default:
		kind, detail = "variable", v.Name()
		for i := len(path) - 1; i >= 0; i-- {
			if fl, ok := path[i].(*ast.FieldList); ok && i > 0 {
				if ft, ok := path[i-1].(*ast.FuncType); ok {
					which := "parameter"
					if ft.Results == fl {
						which = "named result"
					}
					if i > 1 {
						switch fn := path[i-2].(type) {
						case *ast.FuncLit:
							kind, detail = which+" of a func literal", litContext(fn)
						case *ast.FuncDecl:
							if fn.Recv == nil && isHelperName(fn.Name.Name) {
								kind, detail = which+" of a cast helper", fn.Name.Name
							} else {
								kind, detail = which+" of a "+declMethodKindShort(fn), declMethodKind(fn)
							}
						default:
							kind, detail = which+" in a func type (a field's or parameter's signature)", rw.Text(ft)
						}
					} else {
						kind, detail = which+" in a func type", rw.Text(ft)
					}
				}
				break
			}
			if rs, ok := path[i].(*ast.RangeStmt); ok {
				kind = "range variable"
				src := unwrap(rs.X)
				if c, ok := src.(*ast.CallExpr); ok {
					_, name, _ := anyflow.Callee(c)
					detail = name
				} else {
					detail = ts(rw.Info.Types[src].Type)
				}
				break
			}
			if vs, ok := path[i].(*ast.ValueSpec); ok {
				kind = "local/global var declaration"
				if len(vs.Values) == 1 {
					k, _ := srcOf(vs.Values[0], 1)
					detail = "initialised from: " + k
				} else {
					detail = "no initial value"
				}
				break
			}
		}
	}
	set, n := reach(v)
	var keys []string
	for k := range set {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	why := anyflow.SinkWhy[v]
	holds := ""
	switch {
	case len(keys) == 0:
		holds = "0 types seen by the flow model (fed only by what it does not follow, or never stored)"
	case len(keys) == 1:
		holds = "1 type: " + keys[0]
	default:
		// one class hierarchy?
		holds = fmt.Sprintf("%d types", len(keys))
		if len(keys) <= 6 {
			holds += ": " + strings.Join(keys, ", ")
		}
	}
	_ = why
	return out{kind, detail, holds + fmt.Sprintf(" | slots merged %d", n)}
}

func main() {
	dir := os.Args[1]
	anyflow.Load(dir)
	anyflow.Build()
	buildPred()
	for _, f := range rw.Files {
		tf := rw.Fset.File(f.Pos())
		b := filepath.Base(tf.Name())
		fileOf[b] = f
		tfileOf[b] = tf
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
	initHot()
	in, err := os.Open(os.Args[2])
	if err != nil {
		panic(err)
	}
	sc := bufio.NewScanner(in)
	sc.Buffer(make([]byte, 1<<20), 1<<26)
	w := bufio.NewWriter(os.Stdout)
	defer w.Flush()
	for sc.Scan() {
		f := strings.SplitN(sc.Text(), "\t", 6)
		if len(f) < 6 {
			continue
		}
		row, _ := strconv.Atoi(f[0])
		file := f[1]
		off, _ := strconv.Atoi(f[2])
		matched := f[4]
		path := pathAt(file, off)
		pos := tfileOf[file].Pos(off)
		var o out
		nc := notCode(file, path, pos)
		switch {
		case nc != "":
			o = out{nc, "", ""}
			fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row, file, f[3], hot(file), o.sub, o.detail, o.extra, matched)
			continue
		}
		switch row {
		case 14:
			o = typeSwitch(path)
		case 10, 11:
			o = castSite(path, row)
		case 16:
			if strings.HasPrefix(matched, ".(") {
				o = assertSite(path, pos)
			} else {
				o = castSite(path, row)
			}
		case 13, 9:
			if row == 9 {
				// `.(interface{`: the assertion starts at the dot
				o = anonAssert(path, pos)
			} else {
				o = assertSite(path, pos)
			}
		case 100:
			o = anySlot(path, matched)
		default:
			p2 := pos
			if strings.HasPrefix(matched, ".") {
				p2 = pos + 1
			}
			if strings.HasPrefix(matched, "&") {
				o = adapterSite(pathAt(file, off+1))
			} else if strings.HasSuffix(matched, "_fn{func") {
				o = adapterSite(path)
			} else {
				o = callSite(pathAt(file, off+len(matched)-len(strings.TrimLeft(matched, "."))), p2, matched)
			}
		}
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\t%s\t%s\t%s\n", row, file, f[3], hot(file), o.sub, o.detail, o.extra, matched)
	}
}

func anonAssert(path []ast.Node, pos token.Pos) out {
	for i := len(path) - 1; i >= 0; i-- {
		if x, ok := path[i].(*ast.TypeAssertExpr); ok && x.Lparen == pos+1 {
			fd, _ := enclosingFunc(path)
			fn := ""
			if fd != nil {
				fn = fd.Name.Name
			}
			k, d := srcOf(x.X, 0)
			return out{k, d, "in " + fn + " | " + rw.Text(x.Type)}
		}
	}
	return out{"UNCLASSIFIED", "", ""}
}

// adapterSite: a composite literal &X_fn{...} / &X_tfn_Y{...}
func adapterSite(path []ast.Node) out {
	var cl *ast.CompositeLit
	for i := len(path) - 1; i >= 0; i-- {
		if x, ok := path[i].(*ast.CompositeLit); ok {
			cl = x
			break
		}
	}
	if cl == nil {
		return out{"UNCLASSIFIED", "no composite literal", ""}
	}
	tname := rw.Text(cl.Type)
	// what the literal wraps
	wraps := "?"
	captures := "?"
	if len(cl.Elts) == 1 {
		e := cl.Elts[0]
		if kv, ok := e.(*ast.KeyValueExpr); ok {
			e = kv.Value
		}
		switch x := rw.Unparen(e).(type) {
		case *ast.FuncLit:
			wraps = "func literal"
			if capt(x) {
				captures = "capturing"
			} else {
				captures = "non-capturing"
			}
		case *ast.Ident:
			wraps, captures = "named function", "non-capturing"
			if _, ok := rw.Info.Uses[x].(*types.Var); ok {
				wraps, captures = "func-typed variable", "holds a variable"
			}
		case *ast.SelectorExpr:
			if sel := rw.Info.Selections[x]; sel != nil && sel.Kind() == types.MethodVal {
				wraps, captures = "bound method value", "capturing (receiver)"
			} else if sel != nil && sel.Kind() == types.MethodExpr {
				wraps, captures = "method expression", "non-capturing"
			} else {
				wraps, captures = "func-typed field", "holds a variable"
			}
		default:
			wraps = fmt.Sprintf("%T", x)
		}
	}
	// the unary & around it
	var e ast.Expr = cl
	if u, ok := rw.Parent[cl].(*ast.UnaryExpr); ok {
		e = u
	}
	k, d := sinkOf(e)
	// at package level?
	fd, lit := enclosingFunc(path)
	lvl := "in a function"
	if fd == nil && lit == nil {
		lvl = "package-level variable initialiser"
	} else if fd == nil {
		lvl = "package-level initialiser (inside a func literal)"
	}
	return out{k, d, tname + " | " + wraps + " | " + captures + " | " + lvl}
}

// capt: does the func literal use a variable declared outside it (other than package-level)?
func capt(lit *ast.FuncLit) bool {
	found := false
	ast.Inspect(lit.Body, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok || found {
			return !found
		}
		v, ok := rw.Info.Uses[id].(*types.Var)
		if !ok || v.IsField() {
			return true
		}
		if v.Parent() == rw.Pkg.Scope() || v.Parent() == types.Universe {
			return true
		}
		if v.Pos() < lit.Pos() || v.Pos() >= lit.End() {
			found = true
		}
		return true
	})
	return found
}

func notCode(file string, path []ast.Node, pos token.Pos) string {
	src := rw.Src[tfileOf[file]]
	off := tfileOf[file].Offset(pos)
	ls := strings.LastIndexByte(string(src[:off]), '\n') + 1
	pre := string(src[ls:off])
	if strings.HasPrefix(strings.TrimSpace(pre), "//") {
		return "NOT CODE: in a comment"
	}
	if i := strings.Index(pre, "// "); i >= 0 {
		inExpr := false
		if len(path) > 0 {
			switch path[len(path)-1].(type) {
			case *ast.File, *ast.BlockStmt, *ast.GenDecl, *ast.FuncDecl, *ast.StructType, *ast.FieldList, *ast.CaseClause, *ast.IfStmt:
			default:
				inExpr = true
			}
		}
		if !inExpr {
			return "NOT CODE: in a comment"
		}
	}
	if len(path) > 0 {
		if bl, ok := path[len(path)-1].(*ast.BasicLit); ok && bl.Kind == token.STRING {
			return "NOT CODE: inside a string literal (name table or message text)"
		}
	}
	return ""
}

func typeSwitch(path []ast.Node) out {
	for i := len(path) - 1; i >= 0; i-- {
		if x, ok := path[i].(*ast.TypeSwitchStmt); ok {
			fd, _ := enclosingFunc(path)
			fn := "?"
			if fd != nil {
				fn = fd.Name.Name
				if fd.Recv != nil {
					fn = "method " + fn
				} else if isHelperName(fn) {
					fn = helperFamily(fn) + " helper"
				}
			}
			var ta *ast.TypeAssertExpr
			ast.Inspect(x.Assign, func(n ast.Node) bool {
				if t, ok := n.(*ast.TypeAssertExpr); ok && t.Type == nil {
					ta = t
				}
				return true
			})
			k, d := "?", ""
			if ta != nil {
				k, d = srcOf(ta.X, 0)
			}
			return out{"type switch in " + fn + " over: " + k, d, fmt.Sprintf("%d cases", len(x.Body.List))}
		}
	}
	return out{"UNCLASSIFIED", "no type switch", ""}
}
