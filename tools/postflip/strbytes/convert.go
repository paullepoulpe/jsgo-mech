package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"sort"
	"strconv"
	"strings"

	"postflip/rw"
)

// check returns why site si cannot be converted ("" if it can).
func (a *an) check(si *site) string {
	if si.bad != "" {
		return si.bad
	}
	if !si.visited {
		return "not reached by the analysis"
	}
	c := si.call
	argOK := func(i int, ks kind) string {
		v := si.args[i]
		if v.k == 0 {
			return "not reached by the analysis"
		}
		if v.k&^ks == 0 {
			return ""
		}
		switch {
		case v.k&kMid != 0:
			if w := a.scanWhy(c); w != "" {
				return "a loop steps over chars not known to be ASCII: " + w
			}
			return "a position steps over a char not known to be ASCII"
		case v.k&kPrev != 0:
			return "a position steps back over a char not known to be ASCII"
		}
		return "a position that the step cannot follow"
	}
	switch si.name {
	case "String_length":
		if why := a.useOK(c); why != "" {
			return "length() is used as a number: " + strings.TrimPrefix(why, "a position ")
		}
		return ""
	case "String_charAt":
		ks := consistent | kPrev
		if si.scanOK {
			ks |= kMid
		}
		return argOK(1, ks)
	case "String_substring":
		if w := argOK(1, consistent); w != "" {
			return w
		}
		return argOK(2, consistent)
	case "String_substring_int":
		return argOK(1, consistent)
	case "String_startsWith_String_int":
		if _, ok := needle(si.name, c); !ok {
			return "startsWith of a String that is not an ASCII constant"
		}
		return argOK(2, consistent)
	}
	// indexOf / lastIndexOf
	if _, ok := needle(si.name, c); !ok {
		return "indexOf of a char or String that is not an ASCII constant"
	}
	if why := a.useOK(c); why != "" {
		return "its result is used as a number: " + strings.TrimPrefix(why, "a position ")
	}
	if len(c.Args) == 3 {
		ks := consistent
		if strings.HasPrefix(si.name, "String_indexOf") {
			ks |= kMid
		}
		return argOK(2, ks)
	}
	return ""
}

// convert registers the rewrites of the convertible sites; with the index variables, all
// their sites must be convertible.
func (a *an) convert() bool {
	ok := map[*ast.CallExpr]bool{}
	why := map[*ast.CallExpr]string{}
	for c, si := range a.sites {
		w := a.check(si)
		if w == "" {
			if shadowed(a.fd, "len") || shadowed(a.fd, "strings") {
				w = "the function declares a variable named len or strings"
			}
		}
		if w != "" {
			why[c] = w
			continue
		}
		ok[c] = true
	}
	// a position op in the arguments of another goes with it
	for changed := true; changed; {
		changed = false
		for c := range ok {
			if q := a.consumer(c); q != nil && !ok[q] {
				delete(ok, c)
				why[c] = "it is a position for a call that stays: " + why[q]
				changed = true
			}
		}
	}
	for c, w := range why {
		if a.useIV && (a.mentionsPos2(c) || a.involved(c)) {
			return false
		}
		if prev := leftWhy[c]; prev != "" && !a.useIV && w == notPos {
			continue // the reason the index variables were not converted
		}
		leftWhy[c] = w
	}
	for c := range ok {
		a.rewrite(a.sites[c])
		delete(leftWhy, c)
	}
	// char membership tests in ASCII constants
	for c := range ok {
		if a.sites[c].name != "String_charAt" {
			continue
		}
		a.membership(c)
	}
	for _, u := range a.charVarUses() {
		a.membership(u)
	}
	return true
}

// consumer returns the op on s in whose position argument the result of op c is.
func (a *an) consumer(c *ast.CallExpr) *ast.CallExpr {
	for p := rw.Parent[c]; p != nil; p = rw.Parent[p] {
		switch x := p.(type) {
		case *ast.CallExpr:
			if _, q := a.opOn(x); q != nil {
				return q
			}
		case ast.Stmt, *ast.FuncLit:
			return nil
		}
	}
	return nil
}

// involved reports whether the result of op c is a position the index variables take.
func (a *an) involved(c *ast.CallExpr) bool {
	name, _ := callName(c)
	if !posOps[name] {
		return false
	}
	return a.useOK(c) == ""
}

func (a *an) charVarUses() []ast.Expr {
	var out []ast.Expr
	for v := range a.cv {
		for _, u := range rw.UsesOf[v] {
			if u.Pos() >= a.fd.Pos() && u.Pos() < a.fd.End() {
				out = append(out, u)
			}
		}
	}
	return out
}

// membership rewrites String_indexOf(C, int32(x)) of an ASCII constant C and a char x of s
// (now a byte) into strings.IndexByte.
func (a *an) membership(x ast.Expr) {
	var e ast.Expr = x
	for {
		p, ok := rw.Up(e).(ast.Expr)
		if !ok || conversion(p) == nil {
			break
		}
		e = p
	}
	p, ok := rw.Up(e).(*ast.CallExpr)
	if !ok {
		return
	}
	if name, _ := callName(p); name != "String_indexOf" || len(p.Args) != 2 || rw.Unparen(p.Args[1]) != rw.Unparen(e) {
		return
	}
	cs, ok := constString(p.Args[0])
	if !ok || !asciiOnly(cs) {
		return
	}
	if _, done := rw.Rewrite[p]; done {
		return
	}
	note("indexOf(C, c) of an ASCII String constant C and a char of a converted string: strings.IndexByte(C, byte(c))", p)
	a.needStrings(p)
	isByte := false
	if _, c := a.opOn(rw.Unparen(x)); c != nil {
		isByte = true // rewritten to a byte read
	}
	rw.Rewrite[p] = func() string {
		arg := rw.Render(x)
		if !isByte {
			arg = "byte(" + arg + ")"
		}
		return "int32(strings.IndexByte(" + rw.Render(p.Args[0]) + ", " + arg + "))"
	}
}

func (a *an) needStrings(c ast.Node) {
	for _, f := range rw.Files {
		if f.Pos() <= c.Pos() && c.Pos() < f.End() {
			needImport[f] = true
		}
	}
}

var needImport = map[*ast.File]bool{}

// shadowed reports whether fd declares an object named name.
func shadowed(fd *ast.FuncDecl, name string) bool {
	found := false
	ast.Inspect(fd, func(n ast.Node) bool {
		if id, ok := n.(*ast.Ident); ok && id.Name == name && rw.Info.Defs[id] != nil {
			found = true
		}
		return !found
	})
	return found
}

func untyped(e ast.Expr) bool {
	e = rw.Unparen(e)
	if _, ok := e.(*ast.BasicLit); ok {
		return true
	}
	if id, ok := e.(*ast.Ident); ok {
		if c, ok := rw.Info.Uses[id].(*types.Const); ok {
			if b, ok := c.Type().(*types.Basic); ok && b.Info()&types.IsUntyped != 0 {
				return true
			}
		}
	}
	return false
}

// byteOK reports whether a byte can stand where char c was: in a conversion, compared with an
// untyped constant, or switched on with untyped cases.
func byteOK(c *ast.CallExpr) bool {
	switch p := rw.Up(c).(type) {
	case *ast.CallExpr:
		return conversion(p) != nil
	case *ast.BinaryExpr:
		o := p.X
		if rw.Unparen(p.X) == ast.Expr(c) {
			o = p.Y
		}
		return untyped(o)
	case *ast.SwitchStmt:
		for _, cc := range p.Body.List {
			for _, e := range cc.(*ast.CaseClause).List {
				if !untyped(e) {
					return false
				}
			}
		}
		return true
	}
	return false
}

func lit(k int64) string { return strconv.QuoteRune(rune(k)) }

func (a *an) rewrite(si *site) {
	c := si.call
	r := func(i int) string { return rw.Render(c.Args[i]) }
	s := func() string { return rw.Render(c.Args[0]) }
	did := func(what string, gen func() string) {
		note(what, c)
		rw.Rewrite[c] = gen
	}
	// int32(x) of a result already converted to int32
	conv32 := func(x string) string {
		if p, ok := rw.Up(c).(*ast.CallExpr); ok && conversion(p) != nil && isType(rw.Info.Types[p].Type, types.Int32) {
			return x
		}
		return "int32(" + x + ")"
	}
	needle := func() string {
		if k, ok := constInt(c.Args[1]); ok {
			return lit(k)
		}
		return r(1)
	}
	switch si.name {
	case "String_length":
		did("length() as a position: int32(len(s))", func() string { return conv32("len(" + s() + ")") })
	case "String_charAt":
		raw := si.args[1].only(kIn)
		wrap := func(x string) string {
			if byteOK(c) {
				return x
			}
			return "uint16(" + x + ")"
		}
		if raw {
			did("charAt at a byte offset in range: s[i]", func() string { return wrap(s() + "[" + r(1) + "]") })
		} else {
			did("charAt at a byte offset: byteAt_(s, i)", func() string { return wrap("byteAt_(" + s() + ", " + r(1) + ")") })
		}
	case "String_substring":
		av, bv := si.args[1], si.args[2]
		zero := false
		if k, ok := constInt(c.Args[1]); ok && k == 0 {
			zero = true
		}
		if av.only(kIn|kEnd) && bv.only(kIn|kEnd) && (bv.only(kEnd) || zero) {
			did("substring at byte offsets in order: s[a:b]", func() string {
				if zero {
					return s() + "[:" + r(2) + "]"
				}
				return s() + "[" + r(1) + ":" + r(2) + "]"
			})
		} else {
			did("substring at byte offsets: cutBytes_(s, a, b)", func() string { return "cutBytes_(" + s() + ", " + r(1) + ", " + r(2) + ")" })
		}
	case "String_substring_int":
		if si.args[1].only(kIn | kEnd) {
			did("substring(a) at a byte offset in range: s[a:]", func() string { return s() + "[" + r(1) + ":]" })
		} else {
			did("substring(a) at a byte offset: cutBytesFrom_(s, a)", func() string { return "cutBytesFrom_(" + s() + ", " + r(1) + ")" })
		}
	case "String_indexOf":
		a.needStrings(c)
		did("indexOf(C): strings.IndexByte", func() string { return conv32("strings.IndexByte(" + s() + ", " + needle() + ")") })
	case "String_lastIndexOf":
		a.needStrings(c)
		did("lastIndexOf(C): strings.LastIndexByte", func() string { return conv32("strings.LastIndexByte(" + s() + ", " + needle() + ")") })
	case "String_indexOf_String":
		a.needStrings(c)
		did("indexOf(String): strings.Index", func() string { return conv32("strings.Index(" + s() + ", " + r(1) + ")") })
	case "String_lastIndexOf_String":
		a.needStrings(c)
		did("lastIndexOf(String): strings.LastIndex", func() string { return conv32("strings.LastIndex(" + s() + ", " + r(1) + ")") })
	case "String_indexOf_int_int":
		did("indexOf(C, from): indexByteFrom_", func() string { return "indexByteFrom_(" + s() + ", " + needle() + ", " + r(2) + ")" })
	case "String_lastIndexOf_int_int":
		did("lastIndexOf(C, from): lastIndexByteFrom_", func() string { return "lastIndexByteFrom_(" + s() + ", " + needle() + ", " + r(2) + ")" })
	case "String_indexOf_String_int":
		did("indexOf(String, from): indexFrom_", func() string { return "indexFrom_(" + s() + ", " + r(1) + ", " + r(2) + ")" })
	case "String_lastIndexOf_String_int":
		did("lastIndexOf(String, from): lastIndexFrom_", func() string { return "lastIndexFrom_(" + s() + ", " + r(1) + ", " + r(2) + ")" })
	case "String_startsWith_String_int":
		did("startsWith(p, off): hasPrefixAt_", func() string { return "hasPrefixAt_(" + s() + ", " + r(1) + ", " + r(2) + ")" })
	}
}

// ---- what is left

func leftovers() {
	for _, f := range rw.Files {
		if !strings.HasPrefix(filepath(f), "g_") {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := c.Fun.(*ast.Ident)
			if !ok || !strings.HasPrefix(id.Name, "String_") {
				return true
			}
			if _, ok := rw.Info.Uses[id].(*types.Func); !ok {
				return true
			}
			if _, done := rw.Rewrite[c]; done {
				return true
			}
			why := leftWhy[c]
			if why == "" {
				why = defaultWhy(id.Name, c)
			}
			note("left: "+strings.TrimPrefix(id.Name, "String_")+": "+why, c)
			return true
		})
	}
}

func defaultWhy(name string, c *ast.CallExpr) string {
	if !stringOps[name] {
		return "not a position op (handled elsewhere or not about positions)"
	}
	if len(c.Args) == 0 {
		return "?"
	}
	x := rw.Unparen(c.Args[0])
	if _, ok := constString(x); ok {
		return "on a constant"
	}
	switch e := x.(type) {
	case *ast.Ident:
		v, _ := rw.Info.Uses[e].(*types.Var)
		if v != nil && v.Parent() == rw.Pkg.Scope() {
			return "on a package variable"
		}
		if inLit(c) {
			return "on a variable of an enclosing function, in a function literal"
		}
		return "on a variable that is not a string local (" + fmt.Sprint(rw.Info.Types[e].Type) + ")"
	case *ast.SelectorExpr:
		if sel := rw.Info.Selections[e]; sel != nil && sel.Kind() == types.FieldVal {
			return "on a field"
		}
		return "on a selector"
	case *ast.CallExpr:
		return "on a call result"
	case *ast.IndexExpr:
		return "on an array element"
	}
	return "on an expression"
}

func inLit(n ast.Node) bool {
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		if _, ok := p.(*ast.FuncLit); ok {
			return true
		}
		if _, ok := p.(*ast.FuncDecl); ok {
			return false
		}
	}
	return false
}

func addImports() {
	for f := range needImport {
		have := false
		for _, im := range f.Imports {
			if p, _ := strconv.Unquote(im.Path.Value); p == "strings" {
				have = true
			}
		}
		if have {
			continue
		}
		var gd *ast.GenDecl
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
				gd = g
				break
			}
		}
		if gd == nil {
			name := f.Name
			rw.Rewrite[name] = func() string { return name.Name + "\n\nimport \"strings\"" }
			continue
		}
		g := gd
		rw.Rewrite[g] = func() string {
			var specs []string
			for _, s := range g.Specs {
				specs = append(specs, rw.Text(s))
			}
			specs = append(specs, `"strings"`)
			sort.Strings(specs)
			return "import (\n" + strings.Join(specs, "\n") + "\n)"
		}
	}
}
