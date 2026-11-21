// Command strview is a post-flip step: code that works in the UTF-16 units of a string works
// on a view of the string that has the units at hand.
//
// A Java String is a sequence of UTF-16 units: charAt(i) is unit i, length() the number of
// units, substring(a, b) the units a to b. The transpiled Go keeps the text as WTF-8, and the
// String_ helpers find a unit by decoding the string from its start (or a checkpoint) on each
// call. The step strbytes turned the positions of a function into byte offsets where nothing
// can tell the difference; what it left is code that does work in units: a lexer or parser
// that keeps its string and its position in fields, a loop that passes its chars on as UTF-16
// units, a length used as a count, a position that is a parameter or a result. That code gets
// what the hand-written port gave its scanners: the string as units, worked out once
// (rt_strview.go: utf16_ is the string, whether it is all ASCII, and its units if not).
//
//	String_length(s)                -> sU16.length()
//	String_charAt(s, i)             -> sU16.charAt(i)
//	String_substring(s, a, b)       -> sU16.substring(a, b)
//	... substring_int, indexOf, indexOf_int_int, indexOf_String, indexOf_String_int,
//	    lastIndexOf, lastIndexOf_String, lastIndexOf_int_int, startsWith_String_int,
//	    codePointAt, regionMatches
//
// Each method gives what the String_ helper gives (and throws what it throws); the positions
// stay unit indexes, so nothing else changes. The view of s is made
//
//   - for a string field f that every assignment sets in a statement of its own
//     (x.f = e, x a variable or this): a field fU16 next to f, set after each assignment of f
//     (x.fU16 = utf16Kept_(x.f), which has the units of a long text too: an object reads its
//     string again and again; x.fU16 = y.gU16 when the value is another viewed field y.g).
//     A field that is never assigned has the view of "", as f is "";
//
//   - for a parameter or a local variable that is given one value: a local variable sU16 :=
//     utf16Of_(s), before the first statement that uses it, in the innermost block that holds
//     all the uses, outside the loops and function literals around them (as far as s is in
//     scope);
//
//   - for a local variable or parameter that is assigned again, in statements of their own:
//     a variable sU16 declared with s (a parameter's at the start of the function), set after
//     each assignment of s.
//
//   - for a package constant that is not ASCII: a package variable CU16 = utf16Kept_(C)
//     declared after it;
//
//   - for the result of a call x.m() without arguments (o.get() of an Optional<String>
//     variable, a getter) or an element a[i] that a function uses twice or more: a variable mU16 that keeps
//     the view with the string it is of, mU16.of(x.m()), so that the call is still made where
//     it was and a new string gets a new view;
//
//   - for a string field that only its constructors assign: in each method that uses it
//     twice or more, a local view as for a parameter; the field gets a view field for uses
//     that are each the only one in their function, and when a position is a field of the
//     same object (a lexer's "char at the cursor": its methods run once per char).
//
// A view is made only where it is used twice or more, or in a loop that keeps it (a single
// length() or charAt reads the string once as it is). The rest is left and counted, by reason. Rerunning
// the step changes nothing: the calls are gone, and a field that has its view keeps it.
//
// usage: strview [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/constant"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

// ops are the String_ helpers that the view has a method for, with their number of arguments.
var ops = map[string]int{
	"String_length": 1, "String_charAt": 2, "String_substring": 3, "String_substring_int": 2,
	"String_indexOf": 2, "String_indexOf_int_int": 3, "String_indexOf_String": 2, "String_indexOf_String_int": 3,
	"String_lastIndexOf": 2, "String_lastIndexOf_String": 2, "String_lastIndexOf_int_int": 3, "String_startsWith_String_int": 3,
	"String_codePointAt": 2, "String_regionMatches": 6, "String_regionMatches_int_String_int_int": 5,
}

var count = map[string]int{}

var verbose = os.Getenv("STRVIEW_VERBOSE") != ""

func left(why string, calls ...*ast.CallExpr) {
	count["left: "+why] += len(calls)
	if verbose {
		for _, c := range calls {
			fmt.Fprintf(os.Stderr, "%s: %s\n", rw.Fset.Position(c.Pos()), why)
		}
	}
}

func isString(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

func objOf(id *ast.Ident) types.Object {
	if o := rw.Info.Uses[id]; o != nil {
		return o
	}
	return rw.Info.Defs[id]
}

// container reports whether n holds a list of statements, and the list.
func container(n ast.Node) ([]ast.Stmt, bool) {
	switch b := n.(type) {
	case *ast.BlockStmt:
		// The body of a switch or select holds clauses, not statements to put one before.
		switch rw.Parent[b].(type) {
		case *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
			return nil, false
		}
		return b.List, true
	case *ast.CaseClause:
		return b.Body, true
	case *ast.CommClause:
		return b.Body, true
	}
	return nil, false
}

// level is a statement list and the statement of it that holds a node.
type level struct {
	in   ast.Node
	stmt ast.Stmt
	loop bool // a loop or function literal lies between the node and this list
}

// levels are the statement lists around n, innermost first, up to the body of fd.
func levels(n ast.Node, fd *ast.FuncDecl) []level {
	var out []level
	loop := false
	child := n
	for p := rw.Parent[n]; p != nil; child, p = p, rw.Parent[p] {
		if list, ok := container(p); ok {
			for _, s := range list {
				if s == child {
					out = append(out, level{p, s, loop})
				}
			}
		}
		switch p.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit:
			loop = true
		}
		if p == fd {
			break
		}
	}
	return out
}

// stmtInList reports whether s is a statement of a statement list.
func stmtInList(s ast.Stmt) bool {
	list, ok := container(rw.Parent[s])
	if !ok {
		return false
	}
	for _, x := range list {
		if x == s {
			return true
		}
	}
	return false
}

var before = map[ast.Stmt][]string{}
var after = map[ast.Stmt][]string{}

type op struct {
	call *ast.CallExpr
	name string
	fd   *ast.FuncDecl
}

func main() {
	dry := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	fillTaken()

	// The fields of the structs, and the structs written as literals without field names.
	type fieldDecl struct {
		f  *ast.Field
		st *types.Struct
	}
	fieldDecls := map[*types.Var]fieldDecl{}
	unkeyed := map[*types.Struct]bool{}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.StructType:
				st, _ := rw.Info.Types[x].Type.(*types.Struct)
				for _, fl := range x.Fields.List {
					for _, id := range fl.Names {
						if v, ok := rw.Info.Defs[id].(*types.Var); ok {
							fieldDecls[v] = fieldDecl{fl, st}
						}
					}
				}
			case *ast.CompositeLit:
				if tv, ok := rw.Info.Types[x]; ok && len(x.Elts) > 0 {
					if st, ok := tv.Type.Underlying().(*types.Struct); ok {
						if _, kv := x.Elts[0].(*ast.KeyValueExpr); !kv {
							unkeyed[st] = true
						}
					}
				}
			}
			return true
		})
	}

	converted := 0
	consumed := map[*ast.CallExpr]bool{}
	constOps := map[*types.Const][]op{}
	var consts []*types.Const
	memoOps := map[string][]op{}
	var memoKeys []string
	localOps := map[*types.Var][]op{}
	fieldOps := map[*types.Var][]op{}
	var locals, fields []*types.Var
	other := map[string]int{}
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_") {
			continue
		}
		for _, d := range f.Decls {
			fd, _ := d.(*ast.FuncDecl)
			ast.Inspect(d, func(n ast.Node) bool {
				name, c := rw.CallName(nodeExpr(n))
				if c == nil || c != n || !strings.HasPrefix(name, "String_") {
					return true
				}
				if consumed[c] {
					return true
				}
				if ops[name] != len(c.Args) {
					other[name]++
					return true
				}
				if name == "String_length" && capacityHint(c) {
					count["converted: length that is only a capacity for a builder: len(s) (bytes)"]++
					converted++
					rw.Rewrite[c] = func() string { return "int32(len(" + rw.Render(c.Args[0]) + "))" }
					return true
				}
				if name == "String_substring_int" {
					if p, inner := afterPrefix(c); p != nil {
						count["converted: substring past a prefix that the enclosing if tested: s[len(p):]"]++
						converted++
						if inner != nil {
							consumed[inner] = true
							converted++
						}
						rw.Rewrite[c] = func() string { return rw.Render(c.Args[0]) + "[len(" + rw.Render(p) + "):]" }
						return true
					}
				}
				if fd == nil || fd.Body == nil {
					left("outside a function", c)
					return true
				}
				o := op{c, name, fd}
				switch s := rw.Unparen(c.Args[0]).(type) {
				case *ast.Ident:
					switch v := objOf(s).(type) {
					case *types.Var:
						if v.Parent() == rw.Pkg.Scope() {
							left("on a package variable", c)
						} else {
							if localOps[v] == nil {
								locals = append(locals, v)
							}
							localOps[v] = append(localOps[v], o)
						}
					case *types.Const:
						if v.Parent() != rw.Pkg.Scope() {
							left("on a constant of a function", c)
						} else {
							if constOps[v] == nil {
								consts = append(consts, v)
							}
							constOps[v] = append(constOps[v], o)
						}
					default:
						left("on a name that is not a variable", c)
					}
				case *ast.SelectorExpr:
					sel := rw.Info.Selections[s]
					if sel == nil || sel.Kind() != types.FieldVal {
						left("on a qualified name that is not a field", c)
					} else if !rw.Pure(s.X) {
						left("on a field of a computed object", c)
					} else {
						v := sel.Obj().(*types.Var).Origin()
						if fieldOps[v] == nil {
							fields = append(fields, v)
						}
						fieldOps[v] = append(fieldOps[v], o)
					}
				case *ast.CallExpr:
					if f, ok := s.Fun.(*ast.SelectorExpr); ok && len(s.Args) == 0 && rw.Pure(f.X) {
						k := fmt.Sprint(fd.Pos(), " ", rw.Text(s))
						if memoOps[k] == nil {
							memoKeys = append(memoKeys, k)
						}
						memoOps[k] = append(memoOps[k], o)
					} else {
						left("on the result of a call with arguments", c)
					}
				case *ast.IndexExpr:
					if rw.Pure(s.X) && rw.Pure(s.Index) {
						k := fmt.Sprint(fd.Pos(), " ", rw.Text(s))
						if memoOps[k] == nil {
							memoKeys = append(memoKeys, k)
						}
						memoOps[k] = append(memoOps[k], o)
					} else {
						left("on an element of an array or list at a computed index", c)
					}
				case *ast.BasicLit:
					left("on a constant", c)
				default:
					left("on an expression", c)
				}
				return true
			})
		}
	}

	type fieldView struct {
		us   []op
		v    *types.Var
		expr string
		base string
	}
	var fieldViews []fieldView
	convert := func(o op, view string) {
		c := o.call
		converted++
		count["converted: "+strings.TrimPrefix(o.name, "String_")]++
		rw.Rewrite[c] = func() string {
			var args []string
			for _, a := range c.Args[1:] {
				args = append(args, rw.Render(a))
			}
			return view + "." + strings.TrimPrefix(o.name, "String_") + "(" + strings.Join(args, ", ") + ")"
		}
	}
	calls := func(us []op) []*ast.CallExpr {
		var cs []*ast.CallExpr
		for _, o := range us {
			cs = append(cs, o.call)
		}
		return cs
	}
	inLoop := func(o op) bool {
		for p := rw.Parent[o.call]; p != nil && p != ast.Node(o.fd); p = rw.Parent[p] {
			switch p.(type) {
			case *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit:
				return true
			}
		}
		return false
	}
	worth := func(us []op) bool {
		if len(us) >= 2 {
			return true
		}
		return inLoop(us[0])
	}

	// A variable that is assigned in ways the step does not follow is treated as an expression
	// that gives a string anew: its view is kept with the string, as for the result of a call.
	later := func(us []op) {
		k := fmt.Sprint(us[0].fd.Pos(), " var ", us[0].call.Pos())
		memoKeys = append(memoKeys, k)
		memoOps[k] = us
	}

	// Parameters and local variables: us are the operations of one function on the string
	// expr, which is the variable v or a field of the object v (base is the name the view's is
	// made from).
	nLocals, nMemos := 0, 0
	local := func(us []op, v *types.Var, expr, base string) {
		fd := us[0].fd
		if !worth(us) {
			left(onlyOne, calls(us)...)
			return
		}
		viewName := newName(fd, base)

		// The definition of v and its other assignments.
		var def ast.Node // *ast.Field (parameter), *ast.ValueSpec, *ast.AssignStmt (:=), *ast.RangeStmt
		var assigns []*ast.AssignStmt
		why := ""
		ast.Inspect(fd, func(n ast.Node) bool {
			id, ok := n.(*ast.Ident)
			if !ok || why != "" {
				return why == ""
			}
			if rw.Info.Defs[id] == types.Object(v) {
				switch p := rw.Parent[id].(type) {
				case *ast.Field:
					if fl, ok := rw.Parent[p].(*ast.FieldList); ok {
						if ft, ok := rw.Parent[fl].(*ast.FuncType); ok && ft.Results == fl {
							why = "the variable is a named result"
						}
					}
					def = p
				case *ast.ValueSpec:
					if len(p.Names) != 1 {
						why = "the variable is declared with others"
					}
					def = p
				case *ast.AssignStmt:
					if len(p.Lhs) != 1 {
						why = "the variable is declared with others"
					}
					def = p
				case *ast.RangeStmt:
					def = p
				default:
					why = "the variable is declared in a way the step does not follow"
				}
				return true
			}
			if rw.Info.Uses[id] != types.Object(v) {
				return true
			}
			switch up := rw.Up(id).(type) {
			case *ast.AssignStmt:
				for _, l := range up.Lhs {
					if rw.Unparen(l) == ast.Expr(id) {
						if len(up.Lhs) != 1 || !stmtInList(up) {
							why = "the variable is assigned in a statement that is not a plain assignment of its own"
						}
						assigns = append(assigns, up)
					}
				}
			case *ast.UnaryExpr:
				if up.Op == token.AND {
					why = "the variable's address is taken"
				}
			case *ast.RangeStmt:
				if up.Key == ast.Expr(id) || up.Value == ast.Expr(id) {
					why = "the variable is assigned by a range clause"
				}
			case *ast.IncDecStmt:
				why = "the variable is assigned in a way the step does not follow"
			}
			return true
		})
		if why == "" && def == nil {
			why = "the variable belongs to an enclosing function"
		}
		if why == "" && hasGoto(fd) {
			why = "the function has a goto"
		}
		if why != "" {
			if why == "the function has a goto" || def == nil {
				left("on a variable: "+why, calls(us)...)
				return
			}
			later(us) // the view is kept with the string the variable holds
			return
		}

		if len(assigns) > 0 && expr != v.Name() {
			left("on a field of a variable that is assigned again", calls(us)...)
			return
		}
		if len(assigns) == 0 {
			// A view of the same one value that an earlier run made, in scope at every use.
			if ev := viewOf(fd, expr); ev != nil && inScope(ev, us) {
				count["uses of a view that the function has"] += len(us)
				nLocals++
				for _, o := range us {
					convert(o, ev.Name())
				}
				return
			}
		}
		decl := viewName + " := utf16Of_(" + expr + ")"
		set := viewName + " = utf16Of_(" + expr + ")"
		if len(assigns) == 0 {
			// One value: the view is made before the first use, outside the loops around the uses.
			// The statement lists that hold every use, innermost first.
			common := levels(us[0].call, fd)
			for _, o := range us[1:] {
				in := map[ast.Node]bool{}
				for _, l := range levels(o.call, fd) {
					in[l.in] = true
				}
				var keep []level
				for _, l := range common {
					if in[l.in] {
						keep = append(keep, l)
					}
				}
				common = keep
			}
			// In each list, the first statement that holds a use, and whether the uses in it are
			// in a loop below the list.
			for i := range common {
				common[i].stmt, common[i].loop = nil, false
				for _, o := range us {
					for _, l := range levels(o.call, fd) {
						if l.in == common[i].in {
							if common[i].stmt == nil || l.stmt.Pos() < common[i].stmt.Pos() {
								common[i].stmt = l.stmt
							}
							common[i].loop = common[i].loop || l.loop
						}
					}
				}
			}
			// v is in scope at a list's statement when its definition comes before the statement
			// and is in the list (or the list is in the scope that the definition opens).
			valid := func(l level) bool {
				switch d := def.(type) {
				case *ast.Field:
					// a parameter of fd, or of a function literal that must hold the list
					for p := rw.Parent[d]; p != nil; p = rw.Parent[p] {
						if lit, ok := p.(*ast.FuncLit); ok {
							return lit.Body.Pos() <= l.in.Pos() && l.in.End() <= lit.Body.End()
						}
						if p == ast.Node(fd) {
							return true
						}
					}
					return false
				case *ast.RangeStmt:
					return d.Body.Pos() <= l.in.Pos() && l.in.End() <= d.Body.End()
				}
				if def.End() > l.stmt.Pos() {
					return false
				}
				// the definition is a statement of a list around (or equal to) l.in, or the init of
				// a statement whose body holds l.in
				for p := rw.Parent[def]; p != nil; p = rw.Parent[p] {
					switch s := p.(type) {
					case *ast.BlockStmt, *ast.CaseClause, *ast.CommClause:
						return p.Pos() <= l.in.Pos() && l.in.End() <= p.End()
					case *ast.IfStmt:
						if s.Init != nil && s.Init.Pos() <= def.Pos() && def.End() <= s.Init.End() {
							return s.Body.Pos() <= l.in.Pos() && l.in.End() <= s.End()
						}
					case *ast.ForStmt:
						if s.Init != nil && s.Init.Pos() <= def.Pos() && def.End() <= s.Init.End() {
							return s.Body.Pos() <= l.in.Pos() && l.in.End() <= s.Body.End()
						}
					case *ast.SwitchStmt:
						if s.Init != nil && s.Init.Pos() <= def.Pos() && def.End() <= s.Init.End() {
							return s.Body.Pos() <= l.in.Pos() && l.in.End() <= s.Body.End()
						}
					}
				}
				return false
			}
			pick := -1
			if len(common) > 0 && valid(common[0]) {
				pick = 0
				// Out of the loops: the outermost list in scope that has a loop below it.
				for i := 1; i < len(common) && valid(common[i]); i++ {
					if loopBetween(common[pick].in, common[i].in) {
						pick = i
					}
				}
			}
			if pick < 0 {
				left("on a variable: it is used in the statement that declares it", calls(us)...)
				return
			}
			if len(us) == 1 && !loopBetween(us[0].call, common[pick].in) {
				left(onlyOne, calls(us)...)
				return
			}
			before[common[pick].stmt] = append(before[common[pick].stmt], decl)
			count["views of a parameter or a variable with one value"]++
		} else {
			// Assigned again: the view is declared with v and follows each assignment. A single
			// use is worth it in a loop that does not assign v.
			if len(us) == 1 {
				kept := false
				for p := rw.Parent[us[0].call]; p != nil && p != ast.Node(fd); p = rw.Parent[p] {
					switch p.(type) {
					case *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit:
						in := false
						for _, a := range assigns {
							in = in || p.Pos() <= a.Pos() && a.End() <= p.End()
						}
						kept = kept || !in
					}
				}
				if !kept {
					left(onlyOne, calls(us)...)
					return
				}
			}
			ok := true
			switch d := def.(type) {
			case *ast.Field:
				for p := rw.Parent[d]; p != nil; p = rw.Parent[p] {
					if lit, isLit := p.(*ast.FuncLit); isLit {
						if len(lit.Body.List) > 0 {
							before[lit.Body.List[0]] = append(before[lit.Body.List[0]], decl)
						}
						break
					}
					if p == ast.Node(fd) {
						before[fd.Body.List[0]] = append(before[fd.Body.List[0]], decl)
						break
					}
				}
			case *ast.ValueSpec:
				gd, _ := rw.Parent[d].(*ast.GenDecl)
				ds, _ := rw.Parent[gd].(*ast.DeclStmt)
				if gd == nil || ds == nil || len(gd.Specs) != 1 || !stmtInList(ds) {
					ok = false
				} else if len(d.Values) == 0 {
					after[ds] = append(after[ds], "var "+viewName+" utf16_")
				} else {
					after[ds] = append(after[ds], decl)
				}
			case *ast.AssignStmt:
				if !stmtInList(d) {
					ok = false
				} else {
					after[d] = append(after[d], decl)
				}
			default:
				ok = false
			}
			if !ok {
				later(us)
				return
			}
			// An assignment that no use can follow (every use is before it, outside loops and
			// function literals) needs no view.
			opInLit := false
			for _, o := range us {
				opInLit = opInLit || litBetween(o.call, fd)
			}
			for _, a := range assigns {
				dead := !opInLit && !loopBetween(a, fd)
				for _, o := range us {
					dead = dead && o.call.End() <= a.End() // a use in the assigned value comes before
				}
				if dead {
					continue
				}
				after[a] = append(after[a], set)
				count["view assignments"]++
			}
			count["views of a variable that is assigned again"]++
		}
		usedNames[fd][viewName] = true
		nLocals++
		for _, o := range us {
			convert(o, viewName)
		}
	}

	// Results of calls x.m() and elements a[i]: the view is kept with the string it is of.
	memos := func() {
		for _, k := range memoKeys {
			us := memoOps[k]
			fd := us[0].fd
			if !worth(us) {
				left("the only UTF-16 operation on the result of a call in its function, not in a loop: it reads the string once", calls(us)...)
				continue
			}
			if hasGoto(fd) {
				left("on the result of a call: the function has a goto", calls(us)...)
				continue
			}
			if _, elem := rw.Unparen(us[0].call.Args[0]).(*ast.IndexExpr); elem && len(us) == 1 {
				left("the only UTF-16 operation on an element a[i] in its function (a loop reads another element each time): it reads the string once", calls(us)...)
				continue
			}
			var base string
			switch x := rw.Unparen(us[0].call.Args[0]).(type) {
			case *ast.Ident:
				base = x.Name
			case *ast.IndexExpr:
				base = "elem"
				switch a := rw.Unparen(x.X).(type) {
				case *ast.Ident:
					base = a.Name
				case *ast.SelectorExpr:
					base = a.Sel.Name
				}
			case *ast.CallExpr:
				f := x.Fun.(*ast.SelectorExpr)
				base = f.Sel.Name
				if id, ok := rw.Unparen(f.X).(*ast.Ident); ok && base == "get" {
					base = id.Name
				}
			}
			if ev := keptViewOf(fd, rw.Text(rw.Unparen(us[0].call.Args[0]))); ev != nil && inScope(ev, us) {
				count["uses of a view that the function has"] += len(us)
				nMemos++
				for _, o := range us {
					convert(o, ev.Name()+".of("+rw.Text(o.call.Args[0])+")")
				}
				continue
			}
			viewName := newName(fd, base)
			common := levels(us[0].call, fd)
			for _, o := range us[1:] {
				in := map[ast.Node]bool{}
				for _, l := range levels(o.call, fd) {
					in[l.in] = true
				}
				var keep []level
				for _, l := range common {
					if in[l.in] {
						keep = append(keep, l)
					}
				}
				common = keep
			}
			// Outside the loops around the uses, inside the function literal that holds them.
			pick := 0
			for i := 1; i < len(common); i++ {
				if litBetween(common[pick].in, common[i].in) {
					break
				}
				if loopBetween(common[pick].in, common[i].in) {
					pick = i
				}
			}
			var first ast.Stmt
			for _, o := range us {
				for _, l := range levels(o.call, fd) {
					if l.in == common[pick].in && (first == nil || l.stmt.Pos() < first.Pos()) {
						first = l.stmt
					}
				}
			}
			if len(us) == 1 && !loopBetween(us[0].call, common[pick].in) {
				left("the only UTF-16 operation on the result of a call in its function, not in a loop: it reads the string once", calls(us)...)
				continue
			}
			before[first] = append(before[first], "var "+viewName+" utf16_")
			usedNames[fd][viewName] = true
			count["views kept with the string that a call x.m() or a variable gives"]++
			nMemos++
			for _, o := range us {
				o := o
				convert(o, viewName+".of("+rw.Text(o.call.Args[0])+")")
			}
		}
	}

	// The assignments of view fields to write, once every field that gets a view is known: a
	// field that is assigned another viewed field (this.contents = file.contents) takes that
	// field's view, and any other value gets a view of its own.
	type pendingWrite struct {
		stmt           *ast.AssignStmt
		x, view, fname string
	}
	var pending []pendingWrite
	viewed := map[*types.Var]bool{}
	type upgrade struct {
		old ast.Stmt
		w   pendingWrite
	}
	var upgrades []upgrade

	// The view fields that an earlier run added: every assignment of the string must still be
	// followed by the view's (a later step may have written a new assignment).
	var viewFields []*types.Var
	for fv := range fieldDecls {
		viewFields = append(viewFields, fv)
	}
	sort.Slice(viewFields, func(i, j int) bool { return viewFields[i].Pos() < viewFields[j].Pos() })
	for _, vf := range viewFields {
		nt, ok := vf.Type().(*types.Named)
		if !ok || nt.Obj().Name() != "utf16_" || !strings.HasSuffix(vf.Name(), "U16") {
			continue
		}
		var fv *types.Var
		st := fieldDecls[vf].st
		for i := 0; i < st.NumFields(); i++ {
			if st.Field(i).Name() == strings.TrimSuffix(vf.Name(), "U16") && isString(st.Field(i).Type()) {
				fv = st.Field(i)
			}
		}
		if fv == nil {
			continue
		}
		viewed[fv] = true
		for _, id := range rw.UsesOf[fv] {
			sel, ok := rw.Parent[id].(*ast.SelectorExpr)
			if !ok || sel.Sel != id {
				count["STALE VIEW: "+fv.Name()+" is set in a way the step does not follow"]++
				continue
			}
			as, ok := rw.Up(sel).(*ast.AssignStmt)
			if !ok {
				continue
			}
			isLhs := false
			for _, l := range as.Lhs {
				isLhs = isLhs || rw.Unparen(l) == ast.Expr(sel)
			}
			if !isLhs {
				continue
			}
			if len(as.Lhs) != 1 || !stmtInList(as) || !rw.Pure(sel.X) {
				count["STALE VIEW: "+fv.Name()+" is assigned in a statement that is not a plain assignment of its own"]++
				continue
			}
			x := rw.Text(sel.X)
			list, _ := container(rw.Parent[as])
			followed := false
			for i, st := range list {
				if st == ast.Stmt(as) && i+1 < len(list) && strings.HasPrefix(rw.Text(list[i+1]), x+"."+vf.Name()+" = ") {
					followed = true
					// The first version of the step made every view with utf16Of_.
					if rw.Text(list[i+1]) == x+"."+vf.Name()+" = utf16Of_("+x+"."+fv.Name()+")" {
						upgrades = append(upgrades, upgrade{list[i+1], pendingWrite{as, x, vf.Name(), fv.Name()}})
					}
				}
			}
			if !followed {
				pending = append(pending, pendingWrite{as, x, vf.Name(), fv.Name()})
				count["view field assignments added after an assignment that a later step wrote"]++
			}
		}
	}

	// Fields.
	nFields := 0
	for _, fv := range fields {
		us := fieldOps[fv]
		fdl, ok := fieldDecls[fv]
		if !ok {
			left("on a field of a type of the runtime", calls(us)...)
			continue
		}
		viewName := fv.Name() + "U16"
		has := false
		for i := 0; i < fdl.st.NumFields(); i++ {
			if fdl.st.Field(i).Name() == viewName {
				has = true
			}
		}
		why := ""
		type write struct {
			stmt *ast.AssignStmt
			x    ast.Expr
		}
		var writes []write
		final := true
		if !has {
			if unkeyed[fdl.st] {
				why = "the field's struct is written as a literal without field names"
			}
			for _, id := range rw.UsesOf[fv] {
				if why != "" {
					break
				}
				if kv, ok := rw.Parent[id].(*ast.KeyValueExpr); ok && kv.Key == ast.Expr(id) {
					why = "the field is set in a struct literal"
					break
				}
				sel, ok := rw.Parent[id].(*ast.SelectorExpr)
				if !ok || sel.Sel != id {
					why = "the field is used in a way the step does not follow"
					break
				}
				switch up := rw.Up(sel).(type) {
				case *ast.AssignStmt:
					isLhs := false
					for _, l := range up.Lhs {
						if rw.Unparen(l) == ast.Expr(sel) {
							isLhs = true
						}
					}
					if !isLhs {
						break
					}
					if len(up.Lhs) != 1 || !stmtInList(up) || !rw.Pure(sel.X) {
						why = "the field is assigned in a statement that is not a plain assignment of its own"
						break
					}
					writes = append(writes, write{up, sel.X})
					if id, ok := rw.Unparen(sel.X).(*ast.Ident); !ok || id.Name != "this" || !isCtor(funcOf(up)) {
						final = false
					}
				case *ast.UnaryExpr:
					if up.Op == token.AND {
						why = "the field's address is taken"
					}
				case *ast.RangeStmt:
					if up.Key == ast.Expr(sel) || up.Value == ast.Expr(sel) {
						why = "the field is assigned by a range clause"
					}
				}
			}
			if why == "" && taken[viewName] {
				why = "the name of the view field is taken"
			}
		}
		if !has {
			// A field that only its constructors assign (to this) is one string in any other method: the
			// methods that use it twice or more make a view of their own, and the field gets one
			// only for uses that are each the only one of a call (a lexer's "char at the cursor").
			var single []op
			groups := map[string][]op{}
			var order []string
			// A position that is a field of the same object (charAt(this.s, this.pos)) is a
			// lexer's cursor: its methods are called once per char, so the view belongs to the
			// object and not to each call.
			cursor := false
			for _, o := range us {
				x, _ := rw.Unparen(rw.Unparen(o.call.Args[0]).(*ast.SelectorExpr).X).(*ast.Ident)
				for _, a := range o.call.Args[1:] {
					ast.Inspect(a, func(n ast.Node) bool {
						if sel, ok := n.(*ast.SelectorExpr); ok && x != nil {
							if id, ok := rw.Unparen(sel.X).(*ast.Ident); ok && objOf(id) == objOf(x) {
								if fs := rw.Info.Selections[sel]; fs != nil && fs.Kind() == types.FieldVal {
									if b, ok := fs.Type().Underlying().(*types.Basic); ok && b.Info()&types.IsInteger != 0 {
										cursor = true
									}
								}
							}
						}
						return true
					})
				}
			}
			for _, o := range us {
				x, _ := rw.Unparen(rw.Unparen(o.call.Args[0]).(*ast.SelectorExpr).X).(*ast.Ident)
				if why != "" || !final || x == nil || isCtor(o.fd) || cursor {
					single = append(single, o)
					continue
				}
				k := fmt.Sprint(o.fd.Pos(), " ", x.Name)
				if groups[k] == nil {
					order = append(order, k)
				}
				groups[k] = append(groups[k], o)
			}
			for _, k := range order {
				if !worth(groups[k]) {
					single = append(single, groups[k]...)
					groups[k] = nil
				}
			}
			if len(single) == 0 || !worth(single) || why != "" {
				for _, k := range order {
					if g := groups[k]; g != nil {
						x := rw.Unparen(g[0].call.Args[0]).(*ast.SelectorExpr).X.(*ast.Ident)
						fieldViews = append(fieldViews, fieldView{g, objOf(x).(*types.Var), x.Name + "." + fv.Name(), fv.Name()})
					}
				}
				if len(single) > 0 {
					if why != "" && worth(single) {
						left("on a field: "+why, calls(single)...)
					} else {
						left("the only UTF-16 operation on a field in its function, not in a loop: it reads the string once", calls(single)...)
					}
				}
				continue
			}
		}
		nFields++
		if !has {
			count["view fields added"]++
			if verbose {
				fmt.Fprintf(os.Stderr, "%s: field %s gets a view (%d operations, %d assignments)\n", rw.Fset.Position(fdl.f.Pos()), fv.Name(), len(us), len(writes))
			}
			fl := fdl.f
			prev := rw.Rewrite[fl]
			rw.Rewrite[fl] = func() string {
				var s string
				if prev != nil {
					s = prev()
				} else {
					s = rw.RenderChildren(fl)
				}
				return s + "\n" + viewName + " utf16_"
			}
			viewed[fv] = true
			for _, w := range writes {
				pending = append(pending, pendingWrite{w.stmt, rw.Text(w.x), viewName, fv.Name()})
				count["view field assignments"]++
			}
		}
		for _, o := range us {
			s := rw.Unparen(o.call.Args[0]).(*ast.SelectorExpr)
			convert(o, rw.Text(s.X)+"."+viewName)
		}
	}

	viewAssign := func(w pendingWrite) string {
		if w.stmt.Tok == token.ASSIGN && len(w.stmt.Rhs) == 1 {
			if sel, ok := rw.Unparen(w.stmt.Rhs[0]).(*ast.SelectorExpr); ok && rw.Pure(sel.X) {
				if fs := rw.Info.Selections[sel]; fs != nil && fs.Kind() == types.FieldVal && viewed[fs.Obj().(*types.Var).Origin()] {
					count["view field assignments that take the view of the assigned field"]++
					return w.x + "." + w.view + " = " + rw.Text(sel.X) + "." + sel.Sel.Name + "U16"
				}
			}
		}
		return w.x + "." + w.view + " = utf16Kept_(" + w.x + "." + w.fname + ")"
	}
	for _, w := range pending {
		after[w.stmt] = append(after[w.stmt], viewAssign(w))
	}
	for _, u := range upgrades {
		text := viewAssign(u.w)
		rw.Rewrite[u.old] = func() string { return text }
		count["view field assignments of the first version (utf16Of_) rewritten"]++
	}
	// A package constant's view of the first version.
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			gd, ok := d.(*ast.GenDecl)
			if !ok || gd.Tok != token.VAR {
				continue
			}
			for _, sp := range gd.Specs {
				vs := sp.(*ast.ValueSpec)
				if len(vs.Names) != 1 || len(vs.Values) != 1 || !strings.HasSuffix(vs.Names[0].Name, "U16") {
					continue
				}
				name, c := rw.CallName(vs.Values[0])
				if name != "utf16Of_" || len(c.Args) != 1 {
					continue
				}
				if _, isConst := objOf(identOf(c.Args[0])).(*types.Const); isConst {
					fun := c.Fun
					rw.Rewrite[fun] = func() string { return "utf16Kept_" }
					count["views of a package constant of the first version (utf16Of_) rewritten"]++
				}
			}
		}
	}

	// A package constant that is not ASCII (strnative reads the ASCII ones as bytes): its
	// view is a package variable declared after it.
	nConsts := 0
	for _, cv := range consts {
		us := constOps[cv]
		var decl *ast.GenDecl
		for _, f := range rw.Files {
			for _, d := range f.Decls {
				if gd, ok := d.(*ast.GenDecl); ok && gd.Tok == token.CONST {
					for _, sp := range gd.Specs {
						for _, id := range sp.(*ast.ValueSpec).Names {
							if rw.Info.Defs[id] == types.Object(cv) {
								decl = gd
							}
						}
					}
				}
			}
		}
		viewName := cv.Name() + "U16"
		ev, _ := rw.Pkg.Scope().Lookup(viewName).(*types.Var)
		if ev == nil && (!worth(us) || decl == nil || taken[viewName]) {
			left("the only UTF-16 operation on a constant, not in a loop: it reads the string once", calls(us)...)
			continue
		}
		if ev == nil {
			count["views of a package constant"]++
			prev := rw.Rewrite[decl]
			d := decl
			rw.Rewrite[d] = func() string {
				var t string
				if prev != nil {
					t = prev()
				} else {
					t = rw.RenderChildren(d)
				}
				return t + "\n\n// " + viewName + " is the UTF-16 view of " + cv.Name() + " (post-flip step strview).\nvar " + viewName + " = utf16Kept_(" + cv.Name() + ")"
			}
		}
		nConsts++
		for _, o := range us {
			convert(o, viewName)
		}
	}
	for _, g := range fieldViews {
		local(g.us, g.v, g.expr, g.base)
	}
	for _, v := range locals {
		local(localOps[v], v, v.Name(), v.Name())
	}
	memos()

	stmts := map[ast.Stmt]bool{}
	for s := range before {
		stmts[s] = true
	}
	for s := range after {
		stmts[s] = true
	}
	for s := range stmts {
		s := s
		prev := rw.Rewrite[s]
		rw.Rewrite[s] = func() string {
			var b strings.Builder
			for _, l := range before[s] {
				b.WriteString(l + "\n")
			}
			if prev != nil {
				b.WriteString(prev())
			} else {
				b.WriteString(rw.RenderChildren(s))
			}
			for _, l := range after[s] {
				b.WriteString("\n" + l)
			}
			return b.String()
		}
	}

	var keys []string
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%5d %s\n", count[k], k)
	}
	keys = keys[:0]
	nOther := 0
	for k, n := range other {
		keys = append(keys, k)
		nOther += n
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Printf("%5d not an operation of the view: %s\n", other[k], k)
	}
	fmt.Printf("strview: %d String_ calls converted (%d view fields, %d views of variables and of fields in a function, %d of results of calls, %d of constants); %d other String_ calls\n", converted, nFields, nLocals, nMemos, nConsts, nOther)
	if !dry {
		fmt.Printf("%d files changed\n", rw.SaveChecked())
	}
}

// taken are the names of all fields and methods selected or declared in the package.
var taken = map[string]bool{}

func fillTaken() {
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.SelectorExpr:
				taken[x.Sel.Name] = true
			case *ast.Field:
				for _, id := range x.Names {
					taken[id.Name] = true
				}
			case *ast.FuncDecl:
				taken[x.Name.Name] = true
			}
			return true
		})
	}
}

func nodeExpr(n ast.Node) ast.Expr {
	e, _ := n.(ast.Expr)
	if e == nil {
		return &ast.BadExpr{}
	}
	return e
}

const onlyOne = "the only UTF-16 operation on the string in its function, or in a loop that gives it a new value each time: it reads the string once"

var usedNames = map[*ast.FuncDecl]map[string]bool{}

// newName is a name for a view of base that fd does not use.
func newName(fd *ast.FuncDecl, base string) string {
	if usedNames[fd] == nil {
		m := map[string]bool{}
		ast.Inspect(fd, func(n ast.Node) bool {
			if id, ok := n.(*ast.Ident); ok {
				m[id.Name] = true
			}
			return true
		})
		usedNames[fd] = m
	}
	name := base + "U16"
	for k := 2; usedNames[fd][name] || rw.Pkg.Scope().Lookup(name) != nil; k++ {
		name = fmt.Sprintf("%sU16_%d", base, k)
	}
	return name
}

// funcOf is the function declaration around n.
func funcOf(n ast.Node) *ast.FuncDecl {
	for ; n != nil; n = rw.Parent[n] {
		if fd, ok := n.(*ast.FuncDecl); ok {
			return fd
		}
	}
	return nil
}

// isCtor reports whether fd is part of a constructor of its receiver's class C: zero_C sets
// the fields' defaults, finit_C runs the field initializers, init_C (init_C_<parameter types>
// when overloaded) is a constructor's body. A Java method cannot have these names (j2go
// writes a method init of C as init, and one that ends in the class name is not a method of
// the class).
func isCtor(fd *ast.FuncDecl) bool {
	if fd == nil || fd.Recv == nil || len(fd.Recv.List) != 1 {
		return false
	}
	t := fd.Recv.List[0].Type
	if st, ok := t.(*ast.StarExpr); ok {
		t = st.X
	}
	if ix, ok := t.(*ast.IndexExpr); ok {
		t = ix.X
	}
	id, ok := t.(*ast.Ident)
	if !ok {
		return false
	}
	n, c := fd.Name.Name, id.Name
	return n == "zero_"+c || n == "finit_"+c || n == "init_"+c || strings.HasPrefix(n, "init_"+c+"_")
}

func litBetween(in, out ast.Node) bool {
	for p := in; p != nil && p != out; p = rw.Parent[p] {
		if _, ok := p.(*ast.FuncLit); ok {
			return true
		}
	}
	return false
}

// loopBetween reports whether a loop or function literal lies between the statement list in
// and the statement list out around it.
func loopBetween(in, out ast.Node) bool {
	for p := in; p != nil && p != out; p = rw.Parent[p] {
		switch p.(type) {
		case *ast.ForStmt, *ast.RangeStmt, *ast.FuncLit:
			return true
		}
	}
	return false
}

var gotos = map[*ast.FuncDecl]int{}

func hasGoto(fd *ast.FuncDecl) bool {
	if g, ok := gotos[fd]; ok {
		return g > 0
	}
	g := -1
	ast.Inspect(fd, func(n ast.Node) bool {
		if b, ok := n.(*ast.BranchStmt); ok && b.Tok == token.GOTO {
			g = 1
		}
		return true
	})
	gotos[fd] = g
	return g > 0
}

// capacityHint reports whether the value of c (a String_length) only reaches the capacity of
// a builder, through sums and products: b.grow(n) and strBuilderCap_(n) (new StringBuilder(n))
// make room for n bytes and do nothing else, and the bytes of a string are at least its units.
func capacityHint(c *ast.CallExpr) bool {
	var n ast.Node = c
	for {
		p := rw.Up(n)
		switch x := p.(type) {
		case *ast.BinaryExpr:
			if x.Op != token.ADD && x.Op != token.MUL {
				return false
			}
			n = x
			continue
		case *ast.CallExpr:
			if len(x.Args) != 1 || rw.Unparen(x.Args[0]) != rw.Unparen(n.(ast.Expr)) {
				return false
			}
			switch f := x.Fun.(type) {
			case *ast.Ident:
				return f.Name == "strBuilderCap_"
			case *ast.SelectorExpr:
				if f.Sel.Name != "grow" {
					return false
				}
				t := rw.Info.Types[f.X].Type
				if pt, ok := t.(*types.Pointer); ok {
					t = pt.Elem()
				}
				nt, ok := t.(*types.Named)
				return ok && nt.Obj().Name() == "strBuilder_"
			}
		}
		return false
	}
}

// stableText reports whether e is an expression that gives the same string when evaluated
// again with nothing in between: a variable, a field of one, a constant, or o.get().
func stableText(e ast.Expr) bool {
	e = rw.Unparen(e)
	if tv, ok := rw.Info.Types[e]; ok && tv.Value != nil {
		return true
	}
	if c, ok := e.(*ast.CallExpr); ok {
		f, ok := c.Fun.(*ast.SelectorExpr)
		return ok && len(c.Args) == 0 && f.Sel.Name == "get" && rw.Pure(f.X)
	}
	return rw.Pure(e)
}

// hasPrefixCall returns s and p of a call strings.HasPrefix(s, p).
func hasPrefixCall(e ast.Expr) (s, p ast.Expr) {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 2 {
		return nil, nil
	}
	f, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || f.Sel.Name != "HasPrefix" {
		return nil, nil
	}
	if id, ok := f.X.(*ast.Ident); !ok || id.Name != "strings" {
		return nil, nil
	}
	return c.Args[0], c.Args[1]
}

// afterPrefix recognizes s.substring(p.length()) in the first statement of the body of
// `if s.startsWith(p)` (or of `if b`, where the statement before the if is b := s.startsWith(p)
// and nothing else assigns b): s starts with the bytes of p, so the unit index p.length() in s
// is the byte offset len(p), and the substring is s[len(p):]. It returns p and the
// String_length call of the argument, if it is one.
func afterPrefix(c *ast.CallExpr) (ast.Expr, *ast.CallExpr) {
	s := c.Args[0]
	var p ast.Expr
	var inner *ast.CallExpr
	arg := rw.Unparen(c.Args[1])
	if name, lc := rw.CallName(arg); name == "String_length" && len(lc.Args) == 1 {
		p, inner = lc.Args[0], lc
	} else if name == "int32" && len(lc.Args) == 1 {
		// int32(len(P)) of an ASCII constant P, as the earlier steps write P.length()
		if n, ll := rw.CallName(lc.Args[0]); n == "len" && len(ll.Args) == 1 {
			tv := rw.Info.Types[ll.Args[0]]
			if tv.Value == nil || tv.Value.Kind() != constant.String {
				return nil, nil
			}
			for _, b := range []byte(constant.StringVal(tv.Value)) {
				if b >= 0x80 {
					return nil, nil
				}
			}
			p = ll.Args[0]
		}
	}
	if p == nil || !stableText(s) || !stableText(p) {
		return nil, nil
	}
	// the statement of c, which must be the first of an if body
	var stmt ast.Stmt
	for n := ast.Node(c); n != nil; n = rw.Parent[n] {
		if _, ok := n.(*ast.FuncLit); ok {
			return nil, nil
		}
		if st, ok := n.(ast.Stmt); ok && stmtInList(st) {
			stmt = st
			break
		}
	}
	if stmt == nil {
		return nil, nil
	}
	body, ok := rw.Parent[stmt].(*ast.BlockStmt)
	if !ok || body.List[0] != stmt {
		return nil, nil
	}
	ifs, ok := rw.Parent[body].(*ast.IfStmt)
	if !ok || ifs.Body != body || ifs.Init != nil {
		return nil, nil
	}
	cs, cp := hasPrefixCall(ifs.Cond)
	if cs == nil {
		// if b, after b := strings.HasPrefix(s, p)
		id, ok := rw.Unparen(ifs.Cond).(*ast.Ident)
		if !ok || !stmtInList(ifs) {
			return nil, nil
		}
		b, ok := rw.Info.Uses[id].(*types.Var)
		if !ok {
			return nil, nil
		}
		list, _ := container(rw.Parent[ifs])
		var prev ast.Stmt
		for i, st := range list {
			if st == ast.Stmt(ifs) && i > 0 {
				prev = list[i-1]
			}
		}
		ds, ok := prev.(*ast.DeclStmt)
		if !ok {
			return nil, nil
		}
		gd := ds.Decl.(*ast.GenDecl)
		if len(gd.Specs) != 1 {
			return nil, nil
		}
		vs, ok := gd.Specs[0].(*ast.ValueSpec)
		if !ok || len(vs.Names) != 1 || len(vs.Values) != 1 || rw.Info.Defs[vs.Names[0]] != types.Object(b) {
			return nil, nil
		}
		for _, use := range rw.UsesOf[b] {
			if as, ok := rw.Up(use).(*ast.AssignStmt); ok {
				for _, l := range as.Lhs {
					if rw.Unparen(l) == ast.Expr(use) {
						return nil, nil
					}
				}
			}
			if u, ok := rw.Up(use).(*ast.UnaryExpr); ok && u.Op == token.AND {
				return nil, nil
			}
		}
		cs, cp = hasPrefixCall(vs.Values[0])
		if cs == nil {
			return nil, nil
		}
	}
	if rw.Text(rw.Unparen(cs)) != rw.Text(rw.Unparen(s)) || rw.Text(rw.Unparen(cp)) != rw.Text(rw.Unparen(p)) {
		return nil, nil
	}
	return p, inner
}

// viewOf is the variable that fd declares as name := utf16Of_(expr), if any.
func viewOf(fd *ast.FuncDecl, expr string) (v *types.Var) {
	ast.Inspect(fd, func(n ast.Node) bool {
		as, ok := n.(*ast.AssignStmt)
		if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 || v != nil {
			return v == nil
		}
		if name, c := rw.CallName(as.Rhs[0]); name == "utf16Of_" && len(c.Args) == 1 && rw.Text(rw.Unparen(c.Args[0])) == expr {
			if id, ok := as.Lhs[0].(*ast.Ident); ok {
				v, _ = rw.Info.Defs[id].(*types.Var)
			}
		}
		return true
	})
	return v
}

// keptViewOf is the variable of fd that is only used as name.of(expr), if any.
func keptViewOf(fd *ast.FuncDecl, expr string) *types.Var {
	uses := map[*types.Var]int{}
	bad := map[*types.Var]bool{}
	var order []*types.Var
	ast.Inspect(fd, func(n ast.Node) bool {
		id, ok := n.(*ast.Ident)
		if !ok {
			return true
		}
		v, ok := rw.Info.Uses[id].(*types.Var)
		if !ok || v.IsField() {
			return true
		}
		if nt, ok := v.Type().(*types.Named); !ok || nt.Obj().Name() != "utf16_" {
			return true
		}
		sel, _ := rw.Parent[id].(*ast.SelectorExpr)
		var call *ast.CallExpr
		if sel != nil && sel.X == ast.Expr(id) && sel.Sel.Name == "of" {
			call, _ = rw.Parent[sel].(*ast.CallExpr)
		}
		if call == nil || len(call.Args) != 1 || rw.Text(rw.Unparen(call.Args[0])) != expr {
			bad[v] = true
			return true
		}
		if uses[v] == 0 {
			order = append(order, v)
		}
		uses[v]++
		return true
	})
	for _, v := range order {
		if !bad[v] {
			return v
		}
	}
	return nil
}

// inScope reports whether v is declared before, and in scope at, every operation of us.
func inScope(v *types.Var, us []op) bool {
	for _, o := range us {
		if v.Pos() >= o.call.Pos() || !v.Parent().Contains(o.call.Pos()) {
			return false
		}
	}
	return true
}

func identOf(e ast.Expr) *ast.Ident {
	id, _ := rw.Unparen(e).(*ast.Ident)
	if id == nil {
		return &ast.Ident{}
	}
	return id
}
