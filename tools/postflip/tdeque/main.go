// Command tdeque is a post-flip step: a typed deque (j2go's *TDeque[T], an ArrayDeque) used only
// at one end is the typed list form of colls -kind deque, which native then makes a slice.
//
//	contextStack *TDeque[*Context]                   contextStack *TList[*Context]
//	this.contextStack = newTDeque_(tc_p_Context)      this.contextStack = newTList_(tc_p_Context)
//	this.contextStack.push(c)                    ->   this.contextStack.push_(c)
//	this.contextStack.peek()                          this.contextStack.peekTop_()
//	this.contextStack.pop()                           this.contextStack.pop_()
//
// The translator types ArrayDeque<T> as *TDeque[T], a list whose first item is the deque's head
// (push writes in front, into room it keeps there); post-flip step colls -kind deque types the
// erased deques as *TList[T] whose last item is the head of a deque used as a stack (push_,
// pop_, peekTop_ at the end of the list: an append and a truncation) or whose first item is the
// head of a queue (add at the end, popFirst_, peekFirst_ at the start), and native turns those
// lists into slices with the slice helpers (slicePop_, slicePeekTop_...). The hand port keeps
// such stacks as slices (jsgo: CheckSuper.contextStack []superContext, GuardedCallback's
// context stack).
//
// The deques that exchange values (assignment, argument, return, a copy) form a class
// (union-find); a class is converted when every value entering it is a new deque (newTDeque_,
// newTDequeN_, tdequeFrom_ of a collection, tdequeCopy_ of a deque of the class) and every use
// is one of: push, pop, poll, peek (the head); add, peekLast (the tail); getFirst, getLast,
// removeLast through asDeque (SequencedCollection_X(SequencedCollection(d.asDeque())), whose
// value is dropped or asserted or unboxed to the element type); size, isEmpty, clear; a for-each
// from the head (each, All); a comparison with nil; a flow to another slot of the class. A
// class that pushes is a stack: its head is the end of the list (push_, pop_, pollTop_,
// peekTop_, eachRev, AllRev; tdequeFrom_ is tlistFromRev_). A class that adds at the tail and
// does not push is a queue: the list in the deque's order (popFirst_, pollFirst_, peekFirst_,
// each, All; tlistFrom_). Each operation keeps its result for an empty deque (null for poll
// and peek, NoSuchElementException for pop) and the order of a for-each from the head, so the
// program sees the same values. Left, counted by class: a class used at both ends (push and
// add), seen through another method (iterator, get) or as an erased Deque (asDeque given to
// addAll, a Joiner, a message, a loop over the view), or given a value
// that is not a new deque. A rerun finds no deque of a converted class.
//
// usage: tdeque [-n] [-v] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"sort"
	"strings"

	"postflip/rw"
)

var (
	parent  = map[any]any{}
	bad     = map[any]string{}
	badAt   = map[any]token.Pos{}
	uses    = map[any][]use{}
	members = map[any][]any{}
	verbose bool
)

type use struct {
	call *ast.CallExpr // the method call (nil for a constructor)
	m    string
}

// asDequeUse classifies x.asDeque() at e's parent: "nilcmp" (compared with nil), "removeLast"
// (SequencedCollection_removeLast(SequencedCollection(x.asDeque())) whose value is dropped or
// unboxed to the element type), or "".
func asDequeUse(e ast.Expr, sel *ast.SelectorExpr) (string, ast.Node) {
	call, ok := rw.Up(sel).(*ast.CallExpr)
	if !ok || call.Fun != ast.Expr(sel) || len(call.Args) != 0 {
		return "", nil
	}
	if sel.Sel.Name == "asCollT" { // reflip50: a typed twin of piece seqcoll on the deque's typed view
		outer, ok := rw.Up(call).(*ast.CallExpr)
		if !ok || len(outer.Args) != 1 || rw.Unparen(outer.Args[0]) != ast.Expr(call) {
			return "", nil
		}
		name, _ := rw.CallName(outer)
		if i := strings.IndexByte(name, '['); i >= 0 {
			name = name[:i]
		}
		kind := map[string]string{"SequencedCollection_removeLastT_": "removeLast", "SequencedCollection_getFirstT_": "getFirst", "SequencedCollection_getLastT_": "getLast"}[name]
		if kind == "" {
			return "", nil
		}
		return kind, outer
	}
	if un := map[string]string{"asDequeT": "Deque_un_", "asQueueT": "Queue_un_"}[sel.Sel.Name]; un != "" { // reflip46
		u, ok := rw.Up(call).(*ast.CallExpr)
		if !ok || len(u.Args) != 1 || rw.Unparen(u.Args[0]) != ast.Expr(call) {
			return "", nil
		}
		if nm, _ := rw.CallName(u); nm != un {
			return "", nil
		}
		call = u
	}
	switch p := rw.Up(call).(type) {
	case *ast.BinaryExpr:
		if (p.Op == token.EQL || p.Op == token.NEQ) && (isNil(p.X) || isNil(p.Y)) {
			return "nilcmp", call
		}
	case *ast.CallExpr:
		if name, _ := rw.CallName(p); name != "SequencedCollection" || len(p.Args) != 1 {
			return "", nil
		}
		outer, ok := rw.Up(p).(*ast.CallExpr)
		if !ok || len(outer.Args) != 1 {
			return "", nil
		}
		name, _ := rw.CallName(outer)
		kind := map[string]string{"SequencedCollection_removeLast": "removeLast", "SequencedCollection_getFirst": "getFirst", "SequencedCollection_getLast": "getLast"}[name]
		if kind == "" {
			return "", nil
		}
		if _, ok := rw.Up(outer).(*ast.ExprStmt); ok {
			return kind, outer
		}
		// the element asserted to its type (a deque holds no null: Java's ArrayDeque refuses it)
		if ta, ok := rw.Up(outer).(*ast.TypeAssertExpr); ok && ta.Type != nil {
			if elem := elemOf(typeOf(e)); elem != nil && types.Identical(typeOf(ta.Type), elem) {
				return kind, ta
			}
		}
		if u, ok := rw.Up(outer).(*ast.CallExpr); ok && len(u.Args) == 1 {
			elem := elemOf(typeOf(e))
			name, _ := rw.CallName(u)
			if elem != nil && types.Identical(typeOf(u), elem) && (strings.HasSuffix(name, "_of_") || strings.HasPrefix(name, "to_") || strings.HasPrefix(name, "cast_[") || name == "str_" || strings.HasPrefix(name, "unbox_[") || strings.HasPrefix(name, "icast_")) {
				return kind, u
			}
		}
	}
	return "", nil
}

// elemOf is T of *TDeque[T].
func elemOf(t types.Type) types.Type {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.TypeArgs().Len() != 1 {
		return nil
	}
	return n.TypeArgs().At(0)
}

func find(x any) any {
	p, ok := parent[x]
	if !ok || p == x {
		parent[x] = x
		return x
	}
	r := find(p)
	parent[x] = r
	return r
}

func union(a, b any) {
	ra, rb := find(a), find(b)
	if ra == rb {
		return
	}
	parent[ra] = rb
	if w, ok := bad[ra]; ok {
		if _, ok2 := bad[rb]; !ok2 {
			bad[rb], badAt[rb] = w, badAt[ra]
		}
	}
	uses[rb] = append(uses[rb], uses[ra]...)
}

func poison(x any, why string, at ast.Node) {
	r := find(x)
	if _, ok := bad[r]; !ok {
		bad[r] = why
		badAt[r] = at.Pos()
	}
}

func isDeque(t types.Type) bool {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	return ok && n.Obj().Pkg() == rw.Pkg && n.Obj().Name() == "TDeque"
}

func typeOf(e ast.Expr) types.Type { return rw.Info.Types[e].Type }

func generated(n ast.Node) bool {
	name := rw.Fset.Position(n.Pos()).Filename
	return strings.HasPrefix(name[strings.LastIndex(name, "/")+1:], "g_")
}

var (
	headOps = map[string]bool{"push": true, "pop": true, "poll": true, "peek": true}
	okOps   = map[string]bool{"push": true, "pop": true, "poll": true, "peek": true, "peekLast": true, "add": true, "size": true, "isEmpty": true, "clear": true, "each": true, "All": true}
)

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
		verbose = verbose || a == "-v"
	}
	rw.Load(os.Args[len(os.Args)-1])
	// slots: variables (fields, locals, params) and function results of type *TDeque[T]
	resultNode := func(ft *ast.FuncType) any { return ft }
	var funcResult func(e ast.Expr) any
	funcResult = func(e ast.Expr) any {
		call, ok := rw.Unparen(e).(*ast.CallExpr)
		if !ok {
			return nil
		}
		var id *ast.Ident
		switch f := rw.Unparen(call.Fun).(type) {
		case *ast.Ident:
			id = f
		case *ast.SelectorExpr:
			id = f.Sel
		case *ast.FuncLit:
			return resultNode(f.Type)
		}
		if id == nil {
			return nil
		}
		fn, _ := rw.Info.Uses[id].(*types.Func)
		if fn == nil {
			return nil
		}
		return fn.Origin()
	}
	// node of a deque-valued expression: its slot, a constructor (fresh), or nil (unknown).
	var nodeOf func(e ast.Expr) (any, string)
	nodeOf = func(e ast.Expr) (any, string) {
		e = rw.Unparen(e)
		switch x := e.(type) {
		case *ast.Ident:
			if x.Name == "nil" {
				return nil, "nil"
			}
			if v, ok := rw.Info.Uses[x].(*types.Var); ok {
				return v.Origin(), ""
			}
		case *ast.SelectorExpr:
			if v, ok := rw.Info.Uses[x.Sel].(*types.Var); ok {
				return v.Origin(), ""
			}
		case *ast.CallExpr:
			name, _ := rw.CallName(x)
			switch {
			case name == "newTDeque_" || name == "newTDequeN_" || name == "tdequeFrom_":
				return nil, "new"
			case name == "tdequeCopy_":
				return nil, "copy"
			}
			if n := funcResult(x); n != nil {
				return n, ""
			}
		}
		return nil, "?"
	}
	// flows
	link := func(dst any, src ast.Expr, at ast.Node) {
		n, kind := nodeOf(src)
		switch {
		case n != nil:
			union(dst, n)
		case kind == "new" || kind == "nil":
			uses[find(dst)] = append(uses[find(dst)], use{m: "new"})
		case kind == "copy":
			c := rw.Unparen(src).(*ast.CallExpr)
			if sn, _ := nodeOf(c.Args[1]); sn != nil {
				union(dst, sn)
			} else {
				poison(dst, "a copy of a deque that is not a slot", at)
			}
		default:
			poison(dst, "a value that is not a new deque: "+rw.Text(src), at)
		}
	}
	type siteT struct {
		node any
		e    ast.Expr
	}
	var sites []siteT // every deque-valued slot read, with its node
	for _, f := range rw.Files {
		gen := generated(f)
		if !gen {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			switch n := x.(type) {
			case *ast.Field:
				for _, nm := range n.Names {
					if v, ok := rw.Info.Defs[nm].(*types.Var); ok && isDeque(v.Type()) {
						find(v.Origin())
						members[find(v.Origin())] = append(members[find(v.Origin())], v.Origin())
						if !gen {
							poison(v.Origin(), "declared in the runtime", n)
						}
					}
				}
				if len(n.Names) == 0 {
					if fl, ok := rw.Parent[n].(*ast.FieldList); ok {
						if ft, ok := rw.Parent[fl].(*ast.FuncType); ok && ft.Results == fl && isDeque(typeOf(n.Type)) {
							if fd, ok := rw.Parent[ft].(*ast.FuncDecl); ok {
								if fn, ok := rw.Info.Defs[fd.Name].(*types.Func); ok {
									union(ft, fn)
									if !gen {
										poison(fn, "a runtime function's result", n)
									}
								}
							} else {
								find(ft)
							}
						}
					}
				}
			case *ast.AssignStmt:
				if len(n.Lhs) == len(n.Rhs) {
					for i, l := range n.Lhs {
						lt := typeOf(l)
						if lt == nil {
							if id, ok := l.(*ast.Ident); ok && rw.Info.Defs[id] != nil {
								lt = rw.Info.Defs[id].Type()
							}
						}
						if lt == nil || !isDeque(lt) {
							continue
						}
						var dst any
						if id, ok := l.(*ast.Ident); ok {
							if v, ok := rw.Info.Defs[id].(*types.Var); ok {
								dst = v
							} else if v, ok := rw.Info.Uses[id].(*types.Var); ok {
								dst = v.Origin()
							}
						} else if s, ok := l.(*ast.SelectorExpr); ok {
							if v, ok := rw.Info.Uses[s.Sel].(*types.Var); ok {
								dst = v.Origin()
							}
						}
						if dst == nil {
							continue
						}
						link(dst, n.Rhs[i], n)
					}
				}
			case *ast.ValueSpec:
				for i, nm := range n.Names {
					v, ok := rw.Info.Defs[nm].(*types.Var)
					if !ok || !isDeque(v.Type()) {
						continue
					}
					if i < len(n.Values) {
						link(v, n.Values[i], n)
					} else {
						uses[find(v)] = append(uses[find(v)], use{m: "new"}) // nil
					}
				}
			case *ast.ReturnStmt:
				var ft *ast.FuncType
				for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
					if fl, ok := p.(*ast.FuncLit); ok {
						ft = fl.Type
						break
					}
					if fd, ok := p.(*ast.FuncDecl); ok {
						ft = fd.Type
						break
					}
				}
				if ft == nil || ft.Results == nil || len(n.Results) != 1 || !isDeque(typeOf(ft.Results.List[0].Type)) {
					return true
				}
				var dst any = ft
				if fd, ok := rw.Parent[ft].(*ast.FuncDecl); ok {
					dst = rw.Info.Defs[fd.Name].(*types.Func)
				}
				link(dst, n.Results[0], n)
			case *ast.CallExpr:
				// arguments to deque parameters
				var id *ast.Ident
				switch f := rw.Unparen(n.Fun).(type) {
				case *ast.Ident:
					id = f
				case *ast.SelectorExpr:
					id = f.Sel
				case *ast.IndexExpr:
					if i, ok := f.X.(*ast.Ident); ok {
						id = i
					}
				}
				fn, _ := rw.Info.Uses[id].(*types.Func)
				if fn == nil {
					return true
				}
				sig := fn.Origin().Type().(*types.Signature)
				for i, a := range n.Args {
					if !isDeque(typeOf(a)) {
						continue
					}
					if i < sig.Params().Len() && isDeque(sig.Params().At(i).Type()) && !sig.Variadic() && fn.Pkg() == rw.Pkg && !strings.HasSuffix(fn.Name(), "_") {
						link(sig.Params().At(i).Origin(), a, n)
					}
				}
			}
			return true
		})
	}
	// uses of every deque-valued slot read
	type special struct {
		node any
		kind string
		e    ast.Expr
		at   ast.Node // the node replaced
	}
	var specials []special
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			e, ok := x.(ast.Expr)
			if !ok {
				return true
			}
			if !isDeque(typeOf(e)) {
				return true
			}
			n, kind := nodeOf(e)
			if n == nil {
				return true // constructors and copies are handled by their flows
			}
			_ = kind
			p := rw.Up(e)
			switch u := p.(type) {
			case *ast.SelectorExpr:
				if rw.Unparen(u.X) != e {
					return true
				}
				if u.Sel.Name == "asDeque" || u.Sel.Name == "asDequeT" || u.Sel.Name == "asQueueT" || u.Sel.Name == "asCollT" { // reflip46, reflip50
					if k, at := asDequeUse(e, u); k != "" {
						specials = append(specials, special{n, k, e, at})
						if k != "nilcmp" {
							uses[find(n)] = append(uses[find(n)], use{nil, k})
						}
						return true
					}
				}
				call, ok := rw.Up(u).(*ast.CallExpr)
				if !ok || call.Fun != ast.Expr(u) {
					poison(n, "a method value or field of the deque: "+u.Sel.Name, u)
					return true
				}
				if !okOps[u.Sel.Name] {
					poison(n, "used through "+u.Sel.Name, u)
					return true
				}
				uses[find(n)] = append(uses[find(n)], use{call, u.Sel.Name})
				sites = append(sites, siteT{n, e})
			case *ast.BinaryExpr:
				if (u.Op == token.EQL || u.Op == token.NEQ) && (isNil(u.X) || isNil(u.Y)) {
					return true
				}
				poison(n, "compared", u)
			case *ast.AssignStmt, *ast.ValueSpec, *ast.ReturnStmt:
				// flows (handled above); a store into a non-deque slot is a type error anyway
			case *ast.CallExpr:
				name, _ := rw.CallName(u)
				if name == "tdequeCopy_" {
					return true
				}
				if fn := funcResultFn(u); fn != nil && fn.Pkg() == rw.Pkg && !strings.HasSuffix(fn.Name(), "_") {
					sig := fn.Origin().Type().(*types.Signature)
					ok := false
					for i, a := range u.Args {
						if rw.Unparen(a) == e && i < sig.Params().Len() && isDeque(sig.Params().At(i).Type()) {
							ok = true
						}
					}
					if ok {
						return true
					}
				}
				poison(n, "passed to "+rw.Text(u.Fun), u)
			default:
				poison(n, fmt.Sprintf("used in a %T", p), e)
			}
			return true
		})
	}
	// decide per class
	type clsInfo struct {
		mode string
		why  string
	}
	info := map[any]*clsInfo{}
	for x := range parent {
		r := find(x)
		if info[r] != nil {
			continue
		}
		ci := &clsInfo{}
		info[r] = ci
		if w, ok := bad[r]; ok {
			ci.why = w
			continue
		}
		push, add := false, false
		for _, u := range uses[r] {
			push = push || u.m == "push"
			add = add || u.m == "add"
		}
		switch {
		case push && add:
			ci.why = "used at both ends (push and add)"
		case push:
			ci.mode = "stack"
		default:
			ci.mode = "queue"
		}
	}
	done := map[string]int{}
	left := map[string]int{}
	leftAt := map[string][]string{}
	for r, ci := range info {
		if ci.why != "" {
			left[ci.why]++
			leftAt[ci.why] = append(leftAt[ci.why], rw.Fset.Position(badAt[r]).String())
			continue
		}
		done["deque classes -> typed lists ("+ci.mode+")"]++
	}
	modeOf := func(n any) string {
		ci := info[find(n)]
		if ci == nil || ci.why != "" {
			return ""
		}
		return ci.mode
	}
	stackM := map[string]string{"push": "push_", "pop": "pop_", "poll": "pollTop_", "peek": "peekTop_", "each": "eachRev", "All": "AllRev", "peekLast": "peekFirst_", "removeLast": "popFirst_", "getFirst": "getTop_", "getLast": "getFirst_"}
	queueM := map[string]string{"pop": "popFirst_", "poll": "pollFirst_", "peek": "peekFirst_", "peekLast": "peekTop_", "removeLast": "pop_", "getFirst": "getFirst_", "getLast": "getTop_"}
	for _, sp := range specials {
		mode := modeOf(sp.node)
		if mode == "" {
			continue
		}
		sp := sp
		switch sp.kind {
		case "nilcmp":
			rw.Rewrite[sp.at] = func() string { return rw.Render(sp.e) }
			done["asDeque() compared with nil -> the list"]++
		default:
			nm := queueM[sp.kind]
			if mode == "stack" {
				nm = stackM[sp.kind]
			}
			rw.Rewrite[sp.at] = func() string { return rw.Render(sp.e) + "." + nm + "()" }
			done["SequencedCollection_"+sp.kind+"(asDeque()) -> "+nm]++
		}
	}
	// method calls
	for _, s := range sites {
		mode := modeOf(s.node)
		if mode == "" {
			continue
		}
		sel := rw.Up(s.e).(*ast.SelectorExpr)
		m := sel.Sel.Name
		tbl := queueM
		if mode == "stack" {
			tbl = stackM
		}
		if nm, ok := tbl[m]; ok {
			rw.Rewrite[sel.Sel] = func() string { return nm }
			done["calls: "+m+" -> "+nm]++
		}
	}
	// type expressions, constructors and copies
	for _, f := range rw.Files {
		ast.Inspect(f, func(x ast.Node) bool {
			switch n := x.(type) {
			case *ast.StarExpr:
				if !isDeque(typeOf(n)) || !rw.Info.Types[n].IsType() {
					return true
				}
				node := typeNode(n)
				if node == nil || modeOf(node) == "" {
					return true
				}
				ix, ok := n.X.(*ast.IndexExpr)
				if !ok {
					return true
				}
				id, ok := ix.X.(*ast.Ident)
				if !ok {
					return true
				}
				rw.Rewrite[id] = func() string { return "TList" }
				done["declarations *TDeque -> *TList"]++
				return false
			case *ast.CallExpr:
				name, _ := rw.CallName(n)
				if name != "newTDeque_" && name != "newTDequeN_" && name != "tdequeFrom_" && name != "tdequeCopy_" {
					return true
				}
				dst := destOf(n)
				if dst == nil {
					return true
				}
				mode := modeOf(dst)
				if mode == "" {
					return true
				}
				id := n.Fun.(*ast.Ident)
				nm := map[string]string{"newTDeque_": "newTList_", "newTDequeN_": "newTListN_", "tdequeCopy_": "tlistCopy_", "tdequeFrom_": "tlistFrom_"}[name]
				if name == "tdequeFrom_" && mode == "stack" {
					nm = "tlistFromRev_"
				}
				rw.Rewrite[id] = func() string { return nm }
				done["constructors: "+name+" -> "+nm]++
			}
			return true
		})
	}
	keys := []string{}
	for k := range left {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  left: %4d %s\n", left[k], k)
		if verbose {
			for _, p := range leftAt[k] {
				fmt.Fprintf(os.Stderr, "        %s\n", p)
			}
		}
	}
	keys = keys[:0]
	for k := range done {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		fmt.Fprintf(os.Stderr, "  %4d %s\n", done[k], k)
	}
	if report {
		fmt.Fprintln(os.Stderr, "tdeque: dry run, nothing written")
		return
	}
	fmt.Fprintf(os.Stderr, "tdeque: files changed: %d\n", rw.SaveChecked())
}

func isNil(e ast.Expr) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	return ok && id.Name == "nil"
}

func funcResultFn(call *ast.CallExpr) *types.Func {
	var id *ast.Ident
	switch f := rw.Unparen(call.Fun).(type) {
	case *ast.Ident:
		id = f
	case *ast.SelectorExpr:
		id = f.Sel
	case *ast.IndexExpr:
		if i, ok := f.X.(*ast.Ident); ok {
			id = i
		}
	}
	fn, _ := rw.Info.Uses[id].(*types.Func)
	return fn
}

// typeNode is the slot a *TDeque[T] type expression declares.
func typeNode(s *ast.StarExpr) any {
	p := rw.Parent[s]
	switch x := p.(type) {
	case *ast.Field:
		if len(x.Names) > 0 {
			if v, ok := rw.Info.Defs[x.Names[0]].(*types.Var); ok {
				return v.Origin()
			}
		}
		fl, _ := rw.Parent[x].(*ast.FieldList)
		if ft, ok := rw.Parent[fl].(*ast.FuncType); ok && ft.Results == fl {
			if fd, ok := rw.Parent[ft].(*ast.FuncDecl); ok {
				return rw.Info.Defs[fd.Name].(*types.Func)
			}
			return ft
		}
	case *ast.ValueSpec:
		if len(x.Names) == 1 {
			return rw.Info.Defs[x.Names[0]].(*types.Var)
		}
	}
	return nil
}

// destOf is the slot a constructor's (or copy's) value flows into directly.
func destOf(call *ast.CallExpr) any {
	switch p := rw.Up(call).(type) {
	case *ast.AssignStmt:
		for i, r := range p.Rhs {
			if rw.Unparen(r) == ast.Expr(call) && i < len(p.Lhs) {
				switch l := p.Lhs[i].(type) {
				case *ast.Ident:
					if v, ok := rw.Info.Defs[l].(*types.Var); ok {
						return v
					}
					if v, ok := rw.Info.Uses[l].(*types.Var); ok {
						return v.Origin()
					}
				case *ast.SelectorExpr:
					if v, ok := rw.Info.Uses[l.Sel].(*types.Var); ok {
						return v.Origin()
					}
				}
			}
		}
	case *ast.ValueSpec:
		for i, r := range p.Values {
			if rw.Unparen(r) == ast.Expr(call) {
				return rw.Info.Defs[p.Names[i]].(*types.Var)
			}
		}
	case *ast.ReturnStmt:
		for q := rw.Parent[p]; q != nil; q = rw.Parent[q] {
			if fl, ok := q.(*ast.FuncLit); ok {
				return fl.Type
			}
			if fd, ok := q.(*ast.FuncDecl); ok {
				return rw.Info.Defs[fd.Name].(*types.Func)
			}
		}
	case *ast.CallExpr:
		fn := funcResultFn(p)
		if fn == nil {
			return nil
		}
		sig := fn.Origin().Type().(*types.Signature)
		for i, a := range p.Args {
			if rw.Unparen(a) == ast.Expr(call) && i < sig.Params().Len() {
				return sig.Params().At(i).Origin()
			}
		}
	}
	return nil
}
