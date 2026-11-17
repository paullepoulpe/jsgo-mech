// Command strnative is a post-flip step: Java String methods become Go string operations where
// the UTF-16 view of the string cannot make a difference.
//
// A Java String is a sequence of UTF-16 units; the transpiled Go keeps a string as WTF-8 bytes
// and goes through runtime helpers (String_length, String_charAt, String_indexOf, ...) that
// count units. Many calls do not need units:
//
//   - Methods that do not take or return a unit index are the same on the bytes: isEmpty,
//     startsWith, endsWith, contains, replace(CharSequence, CharSequence), valueOf of an int,
//     a long, a boolean, a double or an Object, toString, and replace(char, char) and
//     valueOf(char) of a constant char (its WTF-8 encoding is a constant string). They become
//     s == "", strings.HasPrefix, strings.HasSuffix, strings.Contains, strings.ReplaceAll,
//     strconv.Itoa / FormatInt / FormatBool, Double_toString, objstr_, s, and string literals.
//     A String argument that the helper checked for Java null (csStr_) is still checked unless
//     it is a constant.
//   - length() compared with 0 (== 0, != 0, > 0, <= 0, < 1, >= 1) is len(s) compared with 0.
//   - indexOf / lastIndexOf of a constant ASCII char or of a String, whose result is only
//     compared with -1 or 0 in a way that only tells whether it is -1 (== -1, != -1, > -1,
//     <= -1, < 0, >= 0), is strings.Contains: a WTF-8 string holds an ASCII byte only as that
//     character, and a substring at a unit index exactly when at a byte index.
//   - On a string that is provably ASCII, units are bytes: length() is int32(len(s)), indexOf /
//     lastIndexOf of a constant ASCII char or of a String are strings.IndexByte /
//     LastIndexByte / Index / LastIndex. A string is provably ASCII when it is an ASCII
//     constant, a concatenation of such, the decimal form of a number, a substring, trim or
//     ASCII case change of a provably ASCII string, or a local variable, field or package
//     function result all of whose values are provably ASCII (a fixpoint over the package).
//   - i := s.indexOf(c) of an ASCII char (or ASCII String constant) c whose uses are only
//     comparisons with -1 or 0 and the bounds s.substring(0, i) / s.substring(i + k) of the
//     same s is a byte index: strings.IndexByte, and s[:i] / s[i+k:] (see indexVar).
//   - String.format of a constant template with only %s, %d, %n and %% is a concatenation
//     (see format).
//
// charAt and substring of an ASCII string are left: s[i] and s[a:b] would throw Go's index
// error (ArrayIndexOutOfBoundsException with Go's message) where Java throws
// StringIndexOutOfBoundsException with its own, and the catch blocks of CheckRegExp and
// ReportUntranspilableFeatures report that message.
//
// The rest are left and counted, by reason.
//
// usage: strnative [-n] <dir of package closure>
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
	"strconv"
	"strings"

	"postflip/rw"
)

var count = map[string]int{}

var verbose = os.Getenv("STRNATIVE_VERBOSE") != ""

func leave(reason string, n ast.Node) {
	count["left: "+reason]++
	if verbose {
		fmt.Fprintf(os.Stderr, "%s: %s\n", rw.Fset.Position(n.Pos()), reason)
	}
}

// imports are the packages each file needs, by file.
var imports = map[*ast.File]map[string]bool{}

var fileOf = map[ast.Node]*ast.File{}

func need(c ast.Node, pkg string) {
	f := fileOf[c]
	if imports[f] == nil {
		imports[f] = map[string]bool{}
	}
	imports[f][pkg] = true
}

func isConst(e ast.Expr) bool {
	tv, ok := rw.Info.Types[e]
	return ok && tv.Value != nil
}

func constString(e ast.Expr) (string, bool) {
	tv, ok := rw.Info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.String {
		return "", false
	}
	return constant.StringVal(tv.Value), true
}

func constInt(e ast.Expr) (int64, bool) {
	tv, ok := rw.Info.Types[e]
	if !ok || tv.Value == nil || tv.Value.Kind() != constant.Int {
		return 0, false
	}
	return constant.Int64Val(tv.Value)
}

const nullstr = "\x00\xff<null>\xff\x00"

func asciiOnly(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] >= 0x80 {
			return false
		}
	}
	return true
}

// primary renders e as an operand of a binary operator of precedence 4 or 5 (==, +).
func primary(e ast.Expr) string {
	switch x := rw.Unparen(e).(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr, *ast.BasicLit, *ast.SliceExpr:
		return rw.Render(x)
	case *ast.BinaryExpr:
		if x.Op.Precedence() > token.EQL.Precedence() {
			return rw.Render(x)
		}
	}
	return "(" + rw.Render(e) + ")"
}

// ---- provably ASCII strings

// asciiObj holds, for variables and functions, whether all their values are provably ASCII;
// the fixpoint starts from true for every candidate and removes those with a non-ASCII value.
var asciiObj = map[types.Object]bool{}

// values are the expressions assigned to each candidate object (nil: an unknown value).
var values = map[types.Object][]ast.Expr{}

func isString(t types.Type) bool {
	b, ok := t.Underlying().(*types.Basic)
	return ok && b.Kind() == types.String
}

// ascii reports whether the string e is provably ASCII.
func ascii(e ast.Expr) bool {
	e = rw.Unparen(e)
	if s, ok := constString(e); ok {
		return asciiOnly(s)
	}
	switch x := e.(type) {
	case *ast.BinaryExpr:
		return x.Op == token.ADD && ascii(x.X) && ascii(x.Y)
	case *ast.Ident:
		o := rw.Info.Uses[x]
		return o != nil && asciiObj[o]
	case *ast.SelectorExpr:
		if sel := rw.Info.Selections[x]; sel != nil && sel.Kind() == types.FieldVal {
			return asciiObj[sel.Obj()]
		}
		if o := rw.Info.Uses[x.Sel]; o != nil {
			return asciiObj[o]
		}
	case *ast.CallExpr:
		name, _ := rw.CallName(x)
		switch name {
		case "istr_", "String_valueOf", "String_valueOf_long", "boolstr_", "String_valueOf_boolean":
			return true
		case "String_substring", "String_substring_int", "String_trim", "asciiLower_", "asciiUpper_", "String_toLowerCase", "String_toUpperCase", "sstr_":
			return len(x.Args) > 0 && ascii(x.Args[0])
		}
		if s, ok := x.Fun.(*ast.SelectorExpr); ok && s.X != nil {
			if p, ok := s.X.(*ast.Ident); ok && p.Name == "strconv" {
				return s.Sel.Name == "Itoa" || s.Sel.Name == "FormatInt" || s.Sel.Name == "FormatBool"
			}
		}
		if id, ok := x.Fun.(*ast.Ident); ok {
			if f, ok := rw.Info.Uses[id].(*types.Func); ok {
				return asciiObj[f]
			}
		}
	}
	return false
}

// collect records the values of the string-typed local variables, fields and package-level
// variables, and of the single string results of package-level functions.
func collect() {
	unknown := func(o types.Object) { values[o] = append(values[o], nil) }
	add := func(o types.Object, e ast.Expr) {
		if o != nil && isString(o.Type()) {
			values[o] = append(values[o], e)
		}
	}
	objOf := func(l ast.Expr) types.Object {
		switch x := rw.Unparen(l).(type) {
		case *ast.Ident:
			if o := rw.Info.Defs[x]; o != nil {
				return o
			}
			return rw.Info.Uses[x]
		case *ast.SelectorExpr:
			if sel := rw.Info.Selections[x]; sel != nil && sel.Kind() == types.FieldVal {
				return sel.Obj()
			}
			return rw.Info.Uses[x.Sel]
		}
		return nil
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.AssignStmt:
				if len(x.Lhs) == len(x.Rhs) {
					for i, l := range x.Lhs {
						o := objOf(l)
						if o != nil && isString(o.Type()) {
							if x.Tok == token.ASSIGN || x.Tok == token.DEFINE {
								add(o, x.Rhs[i])
							} else {
								add(o, &ast.BinaryExpr{X: l, Op: token.ADD, Y: x.Rhs[i]}) // s += e
								rw.Info.Types[values[o][len(values[o])-1]] = types.TypeAndValue{}
							}
						}
					}
				} else {
					for _, l := range x.Lhs {
						if o := objOf(l); o != nil {
							unknown(o)
						}
					}
				}
			case *ast.ValueSpec:
				for i, id := range x.Names {
					o := rw.Info.Defs[id]
					if o == nil || !isString(o.Type()) {
						continue
					}
					switch {
					case len(x.Values) == len(x.Names):
						add(o, x.Values[i])
					case len(x.Values) == 0:
						// the zero value "" is ASCII
						if _, ok := values[o]; !ok {
							values[o] = []ast.Expr{}
						}
					default:
						unknown(o)
					}
				}
			case *ast.FuncType:
				// Parameters and named results start with a value from the caller.
				for _, fl := range []*ast.FieldList{x.Params, x.Results} {
					if fl == nil {
						continue
					}
					for _, fd := range fl.List {
						for _, id := range fd.Names {
							if o := rw.Info.Defs[id]; o != nil {
								unknown(o)
							}
						}
					}
				}
			case *ast.RangeStmt:
				for _, l := range []ast.Expr{x.Key, x.Value} {
					if l != nil {
						if o := objOf(l); o != nil {
							unknown(o)
						}
					}
				}
			case *ast.UnaryExpr:
				if x.Op == token.AND {
					if o := objOf(x.X); o != nil {
						unknown(o)
					}
				}
			case *ast.CompositeLit:
				for _, el := range x.Elts {
					if kv, ok := el.(*ast.KeyValueExpr); ok {
						if id, ok := kv.Key.(*ast.Ident); ok {
							if o, ok := rw.Info.Uses[id].(*types.Var); ok && o.IsField() {
								add(o, kv.Value)
							}
						}
					} else if t, ok := rw.Info.Types[x].Type.Underlying().(*types.Struct); ok {
						for i := 0; i < t.NumFields(); i++ {
							unknown(t.Field(i))
						}
					}
				}
			case *ast.FuncDecl:
				o, _ := rw.Info.Defs[x.Name].(*types.Func)
				if o == nil || x.Recv != nil || x.Body == nil {
					return true
				}
				sig := o.Type().(*types.Signature)
				if sig.Results().Len() != 1 || !isString(sig.Results().At(0).Type()) {
					return true
				}
				if sig.Results().At(0).Name() != "" {
					unknown(o)
					return true
				}
				values[o] = []ast.Expr{}
				ast.Inspect(x.Body, func(m ast.Node) bool {
					if _, ok := m.(*ast.FuncLit); ok {
						return false
					}
					if r, ok := m.(*ast.ReturnStmt); ok && len(r.Results) == 1 {
						values[o] = append(values[o], r.Results[0])
					}
					return true
				})
				// The function is used as a value: its results are still its returns.
			}
			return true
		})
	}
	// Any other string variable declared without a value we saw: unknown.
	for _, o := range rw.Info.Defs {
		v, ok := o.(*types.Var)
		if !ok || !isString(v.Type()) {
			continue
		}
		if _, seen := values[v]; !seen && !v.IsField() {
			// declared without a value we saw (parameters, named results): unknown
			unknown(v)
		}
	}
	// Fields of types that j2go copies or that the runtime sets: every field we saw no value
	// for is "" until assigned, and assigned only where we saw it.
	for o := range values {
		asciiObj[o] = true
	}
	for changed := true; changed; {
		changed = false
		for o, vs := range values {
			if !asciiObj[o] {
				continue
			}
			for _, v := range vs {
				if v == nil || !asciiVal(v) {
					asciiObj[o] = false
					changed = true
					break
				}
			}
		}
	}
}

// asciiVal is ascii for a recorded value, which may be a synthetic s += e.
func asciiVal(e ast.Expr) bool {
	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.ADD && rw.Info.Types[b].Type == nil {
		return ascii(b.X) && ascii(b.Y)
	}
	return ascii(e)
}

// ---- rewrites

// shadowed reports whether a declaration in the function around n names pkg.
var shadowFuncs = map[ast.Node]map[string]bool{}

func shadowed(n ast.Node, pkg string) bool {
	var fn ast.Node
	for p := rw.Parent[n]; p != nil; p = rw.Parent[p] {
		if _, ok := p.(*ast.FuncDecl); ok {
			fn = p
		}
	}
	if fn == nil {
		return false
	}
	names, ok := shadowFuncs[fn]
	if !ok {
		names = map[string]bool{}
		ast.Inspect(fn, func(m ast.Node) bool {
			if id, ok := m.(*ast.Ident); ok && rw.Info.Defs[id] != nil {
				names[id.Name] = true
			}
			return true
		})
		shadowFuncs[fn] = names
	}
	return names[pkg]
}

// lit renders a Go string literal.
func lit(s string) string { return strconv.Quote(s) }

// unitString is the WTF-8 encoding of UTF-16 unit c (not a surrogate).
func unitString(c int64) (string, bool) {
	if c < 0 || c > 0xFFFF || (c >= 0xD800 && c <= 0xDFFF) {
		return "", false
	}
	return string(rune(c)), true
}

// nullChecked renders a String argument the helper checks for Java null: a constant as is,
// anything else through csStr_.
func nullChecked(e ast.Expr) string {
	if s, ok := constString(e); ok && s != nullstr {
		return rw.Render(e)
	}
	return "csStr_(" + rw.Render(e) + ")"
}

// presence reports whether the comparison around call c (index result) only tells whether it
// is -1; then it returns the comparison and whether it is true when the index is not -1.
func presence(c ast.Expr) (*ast.BinaryExpr, bool, bool) {
	b, ok := rw.Up(c).(*ast.BinaryExpr)
	if !ok {
		return nil, false, false
	}
	op := b.Op
	var k int64
	if rw.Unparen(b.X) == c {
		var ok bool
		if k, ok = constInt(b.Y); !ok {
			return nil, false, false
		}
	} else {
		var ok bool
		if k, ok = constInt(b.X); !ok {
			return nil, false, false
		}
		// k op idx  ->  idx op' k
		op = map[token.Token]token.Token{token.EQL: token.EQL, token.NEQ: token.NEQ, token.LSS: token.GTR, token.GTR: token.LSS, token.LEQ: token.GEQ, token.GEQ: token.LEQ}[op]
	}
	switch {
	case k == -1 && (op == token.EQL || op == token.LEQ):
		return b, false, true
	case k == -1 && (op == token.NEQ || op == token.GTR):
		return b, true, true
	case k == 0 && op == token.LSS:
		return b, false, true
	case k == 0 && op == token.GEQ:
		return b, true, true
	}
	return nil, false, false
}

// emptiness reports whether the comparison around length call c only tells whether the length
// is 0, and returns the comparison and whether it is true for the empty string.
func emptiness(c ast.Expr) (*ast.BinaryExpr, bool, bool) {
	b, ok := rw.Up(c).(*ast.BinaryExpr)
	if !ok {
		return nil, false, false
	}
	op := b.Op
	var k int64
	if rw.Unparen(b.X) == c {
		if k, ok = constInt(b.Y); !ok {
			return nil, false, false
		}
	} else {
		if k, ok = constInt(b.X); !ok {
			return nil, false, false
		}
		op = map[token.Token]token.Token{token.EQL: token.EQL, token.NEQ: token.NEQ, token.LSS: token.GTR, token.GTR: token.LSS, token.LEQ: token.GEQ, token.GEQ: token.LEQ}[op]
	}
	switch {
	case k == 0 && (op == token.EQL || op == token.LEQ), k == 1 && op == token.LSS:
		return b, true, true
	case k == 0 && (op == token.NEQ || op == token.GTR), k == 1 && op == token.GEQ:
		return b, false, true
	}
	return nil, false, false
}

// boolResult registers the rewrite of bool call or comparison n as the comparison text cmp
// (whose negation is neg): under !, the negation; under == or != (bool), in parentheses.
func boolResult(n ast.Expr, cmp, neg func() string) {
	switch p := rw.Up(n).(type) {
	case *ast.UnaryExpr:
		if p.Op == token.NOT {
			rw.Rewrite[p] = neg
			return
		}
	case *ast.BinaryExpr:
		if p.Op == token.EQL || p.Op == token.NEQ {
			rw.Rewrite[n] = func() string { return "(" + cmp() + ")" }
			return
		}
	}
	rw.Rewrite[n] = cmp
}

func main() {
	report := len(os.Args) > 2 && os.Args[1] == "-n"
	rw.Load(os.Args[len(os.Args)-1])
	collect()

	for _, f := range rw.Files {
		if !generated(f) {
			continue // the handwritten runtime (rt_*.go) is read, not rewritten
		}
		f := f
		ast.Inspect(f, func(n ast.Node) bool {
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			id, ok := c.Fun.(*ast.Ident)
			if !ok || !strings.HasPrefix(id.Name, "String_") {
				return true
			}
			if fn, ok := rw.Info.Uses[id].(*types.Func); !ok || fn.Parent() != rw.Pkg.Scope() {
				return true
			}
			fileOf[c] = f
			call(id.Name, c)
			return true
		})
	}
	addImports()
	var keys []string
	for k := range count {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	done := 0
	for _, k := range keys {
		fmt.Printf("%5d %s\n", count[k], k)
		if !strings.HasPrefix(k, "left") {
			done += count[k]
		}
	}
	fmt.Printf("%5d String_ calls become Go string operations\n", done)
	if !report {
		fmt.Printf("strnative: %d files changed\n", rw.SaveChecked())
	}
}

func pkgCall(c *ast.CallExpr, pkg string) bool {
	if shadowed(c, pkg) {
		leave("the function declares a variable named "+pkg, c)
		return false
	}
	need(c, pkg)
	return true
}

func call(name string, c *ast.CallExpr) {
	if _, done := rw.Rewrite[c]; done {
		return // rewritten with the indexOf whose result it uses
	}
	a := c.Args
	r := func(i int) string { return rw.Render(a[i]) }
	did := func(what string, gen func() string) {
		count[what]++
		if verbose {
			fmt.Fprintf(os.Stderr, "%s: %s\n", rw.Fset.Position(c.Pos()), what)
		}
		rw.Rewrite[c] = gen
	}
	switch name {
	case "String_isEmpty":
		count["isEmpty: s == \"\""]++
		boolResult(c, func() string { return primary(a[0]) + ` == ""` }, func() string { return primary(a[0]) + ` != ""` })
	case "String_startsWith", "String_endsWith":
		if !pkgCall(c, "strings") {
			return
		}
		fn := map[string]string{"String_startsWith": "HasPrefix", "String_endsWith": "HasSuffix"}[name]
		did(name[7:]+": strings."+fn, func() string { return "strings." + fn + "(" + r(0) + ", " + r(1) + ")" })
	case "String_contains_s":
		if !pkgCall(c, "strings") {
			return
		}
		did("contains: strings.Contains", func() string { return "strings.Contains(" + r(0) + ", " + nullChecked(a[1]) + ")" })
	case "String_replace_s":
		if !pkgCall(c, "strings") {
			return
		}
		did("replace(CharSequence, CharSequence): strings.ReplaceAll", func() string {
			return "strings.ReplaceAll(" + r(0) + ", " + nullChecked(a[1]) + ", " + nullChecked(a[2]) + ")"
		})
	case "String_replace_char_char":
		x, ok1 := constInt(a[1])
		y, ok2 := constInt(a[2])
		xs, ok3 := unitString(x)
		ys, ok4 := unitString(y)
		if !ok1 || !ok2 || !ok3 || !ok4 {
			leave("replace(char, char) of a char that is not a constant (or a surrogate)", c)
			return
		}
		if !pkgCall(c, "strings") {
			return
		}
		did("replace(char, char) of constant chars: strings.ReplaceAll", func() string { return "strings.ReplaceAll(" + r(0) + ", " + lit(xs) + ", " + lit(ys) + ")" })
	case "String_valueOf":
		if !pkgCall(c, "strconv") {
			return
		}
		did("valueOf(int): strconv.Itoa", func() string { return "strconv.Itoa(int(" + r(0) + "))" })
	case "String_valueOf_long":
		if !pkgCall(c, "strconv") {
			return
		}
		did("valueOf(long): strconv.FormatInt", func() string { return "strconv.FormatInt(" + r(0) + ", 10)" })
	case "String_valueOf_boolean":
		if !pkgCall(c, "strconv") {
			return
		}
		did("valueOf(boolean): strconv.FormatBool", func() string { return "strconv.FormatBool(" + r(0) + ")" })
	case "String_valueOf_double":
		did("valueOf(double): Double_toString", func() string { return "Double_toString(" + r(0) + ")" })
	case "String_valueOf_Object":
		did("valueOf(Object): objstr_", func() string { return "objstr_(" + r(0) + ")" })
	case "String_toString":
		did("toString: the string", func() string { return primary(a[0]) })
	case "String_valueOf_char":
		x, ok := constInt(a[0])
		s, ok2 := unitString(x)
		if !ok || !ok2 {
			leave("valueOf(char) of a char that is not a constant: its WTF-8 encoding", c)
			return
		}
		did("valueOf(char) of a constant: a string literal", func() string { return lit(s) })
	case "String_concat":
		s1, ok1 := constString(a[0])
		s2, ok2 := constString(a[1])
		if !ok1 || !ok2 || s1 == nullstr || s2 == nullstr {
			leave("concat of a String that may be null (NullPointerException)", c)
			return
		}
		did("concat of constants: +", func() string { return primary(a[0]) + " + " + primary(a[1]) })
	case "String_length":
		if b, empty, ok := emptiness(c); ok {
			count["length compared with 0: len(s)"]++
			test := " != 0"
			if empty {
				test = " == 0"
			}
			rw.Rewrite[b] = func() string { return "len(" + r(0) + ")" + test }
			return
		}
		if ascii(a[0]) {
			did("length of an ASCII string: int32(len(s))", func() string { return "int32(len(" + r(0) + "))" })
			return
		}
		leave("length (units) of a string not known to be ASCII", c)
	case "String_indexOf", "String_lastIndexOf", "String_indexOf_String", "String_lastIndexOf_String":
		var sub string // the searched string, rendered
		isChar := name == "String_indexOf" || name == "String_lastIndexOf"
		if isChar {
			x, ok := constInt(a[1])
			if !ok || x < 0 || x >= 0x80 {
				leave(name[7:]+" of a char that is not a constant ASCII char", c)
				return
			}
			sub = strconv.QuoteRune(rune(x))
		} else {
			sub = r(1)
		}
		if b, present, ok := presence(c); ok {
			if !pkgCall(c, "strings") {
				return
			}
			count[name[7:]+" compared with -1: strings.Contains"]++
			s := sub
			if isChar {
				x, _ := constInt(a[1])
				s = lit(string(rune(x)))
			}
			neg := "!"
			if present {
				neg = ""
			}
			rw.Rewrite[b] = func() string {
				sv := s
				if !isChar {
					sv = rw.Render(a[1])
				}
				return neg + "strings.Contains(" + r(0) + ", " + sv + ")"
			}
			return
		}
		if !ascii(a[0]) {
			if why := indexVar(name, c, sub, isChar); why != "" {
				leave(name[7:]+" (a unit index) in a string not known to be ASCII: "+why, c)
			}
			return
		}
		if !pkgCall(c, "strings") {
			return
		}
		fn := map[string]string{"String_indexOf": "IndexByte", "String_lastIndexOf": "LastIndexByte", "String_indexOf_String": "Index", "String_lastIndexOf_String": "LastIndex"}[name]
		did(name[7:]+" in an ASCII string: strings."+fn, func() string {
			sv := sub
			if !isChar {
				sv = rw.Render(a[1])
			}
			return "int32(strings." + fn + "(" + r(0) + ", " + sv + "))"
		})
	case "String_charAt":
		if ascii(a[0]) {
			leave("charAt of an ASCII string (s[i] would throw Go's index error, not StringIndexOutOfBoundsException)", c)
			return
		}
		leave("charAt (a UTF-16 unit) of a string not known to be ASCII", c)
	case "String_substring", "String_substring_int":
		if ascii(a[0]) {
			leave("substring of an ASCII string (s[a:b] would throw Go's slice error, not StringIndexOutOfBoundsException)", c)
			return
		}
		leave("substring (unit indexes) of a string not known to be ASCII", c)
	case "String_format":
		if why := format(c); why != "" {
			leave("format: "+why, c)
		}
	case "String_trim":
		leave("trim: Java trims chars <= U+0020, not Unicode spaces (strings.TrimSpace)", c)
	case "String_hashCode":
		leave("hashCode: Java's hash over UTF-16 units", c)
	case "String_compareTo":
		leave("compareTo: UTF-16 unit order and differences, not byte order", c)
	case "String_toLowerCase", "String_toUpperCase", "String_equalsIgnoreCase":
		leave(strings.TrimPrefix(name, "String_")+": Java's Unicode case mapping per UTF-16 unit", c)
	case "String_toCharArray", "String_codePointAt", "String_codePointCount", "String_indexOf_int_int", "String_indexOf_String_int", "String_lastIndexOf_int_int", "String_lastIndexOf_String_int", "String_startsWith_String_int", "String_regionMatches", "String_regionMatches_int_String_int_int", "String_valueOf_charArr_int_int":
		leave(strings.TrimPrefix(name, "String_")+": takes or returns UTF-16 units or unit indexes", c)
	case "String_join_s", "String_join_CharSequence_CharSequenceArr_s":
		leave("join over an Iterable / CharSequence[] of Objects (String.valueOf of each)", c)
	case "String_repeat":
		leave("repeat: Java throws IllegalArgumentException for a negative count", c)
	default:
		leave(strings.TrimPrefix(name, "String_")+": not handled", c)
	}
}

// addImports adds the import declarations the rewrites need.
func addImports() {
	for f, pkgs := range imports {
		have := map[string]bool{}
		for _, im := range f.Imports {
			p, _ := strconv.Unquote(im.Path.Value)
			have[p] = true
		}
		var add []string
		for p := range pkgs {
			if !have[p] {
				add = append(add, lit(p))
			}
		}
		if len(add) == 0 {
			continue
		}
		sort.Strings(add)
		var gd *ast.GenDecl
		for _, d := range f.Decls {
			if g, ok := d.(*ast.GenDecl); ok && g.Tok == token.IMPORT {
				gd = g
				break
			}
		}
		if gd == nil {
			name := f.Name
			rw.Rewrite[name] = func() string {
				if len(add) == 1 {
					return name.Name + "\n\nimport " + add[0]
				}
				return name.Name + "\n\nimport (\n" + strings.Join(add, "\n") + "\n)"
			}
			continue
		}
		g := gd
		rw.Rewrite[g] = func() string {
			var specs []string
			for _, s := range g.Specs {
				specs = append(specs, rw.Text(s))
			}
			specs = append(specs, add...)
			sort.Strings(specs)
			return "import (\n" + strings.Join(specs, "\n") + "\n)"
		}
	}
}

// ---- an index of an ASCII char used only to cut the same string

// indexVar rewrites i := s.indexOf(c) (c an ASCII char, or a String constant of ASCII chars)
// whose uses all keep their meaning when i is the byte index of the match instead of its unit
// index. Both name the same position of s, so s.substring(0, i) and s.substring(i + k), for
// 0 <= k <= len(c) (the k units after that position are the match's ASCII bytes), cut s where
// s[:i] and s[i+k:] cut it; and the two indexes are both -1 when c is absent and both 0 when s
// starts with it, so comparisons with -1 or 0 are the same. A bound that Java rejects for
// i == -1 (substring(0, -1), substring(-1 + k) for k != 1) is rewritten only where a test of i
// around it (if i != -1 {...}, or an earlier if i == -1 { ... return }) rules -1 out, since Go
// would throw another exception. s and i must be local variables that are not assigned again.
// It returns the reason when it does not apply.
func indexVar(name string, c *ast.CallExpr, sub string, isChar bool) string {
	subLen := 1
	if !isChar {
		cs, ok := constString(c.Args[1])
		if !ok || !asciiOnly(cs) || cs == "" {
			return "searched string not an ASCII constant"
		}
		subLen = len(cs)
	}
	sid, ok := rw.Unparen(c.Args[0]).(*ast.Ident)
	if !ok {
		return "the string is not a variable"
	}
	sv, ok := rw.Info.Uses[sid].(*types.Var)
	if !ok || sv.IsField() || sv.Parent() == rw.Pkg.Scope() || assignedAfterDef(sv) {
		return "the string is not a local variable assigned once"
	}
	var iv *types.Var
	switch p := rw.Up(c).(type) {
	case *ast.ValueSpec:
		if len(p.Names) == 1 && len(p.Values) == 1 {
			iv, _ = rw.Info.Defs[p.Names[0]].(*types.Var)
		}
	case *ast.AssignStmt:
		if p.Tok == token.DEFINE && len(p.Lhs) == 1 && len(p.Rhs) == 1 {
			if id, ok := p.Lhs[0].(*ast.Ident); ok {
				iv, _ = rw.Info.Defs[id].(*types.Var)
			}
		}
	}
	if iv == nil {
		return "the index is not stored in a new variable"
	}
	if assignedAfterDef(iv) {
		return "the index variable is assigned again"
	}
	type cut struct {
		call *ast.CallExpr
		gen  func() string
	}
	var cuts []cut
	for _, u := range rw.UsesOf[iv] {
		p := rw.Up(u)
		// comparisons with -1 or 0
		if b, ok := p.(*ast.BinaryExpr); ok {
			var other ast.Expr
			if rw.Unparen(b.X) == ast.Expr(u) {
				other = b.Y
			} else {
				other = b.X
			}
			if k, ok := constInt(other); ok && (k == -1 || k == 0) {
				if _, cmp := flippedOps[b.Op]; cmp {
					continue
				}
			}
		}
		// i or i + k as a bound of a substring of s
		e, k := ast.Expr(u), int64(0)
		if b, ok := p.(*ast.BinaryExpr); ok && b.Op == token.ADD && rw.Unparen(b.X) == ast.Expr(u) {
			kk, ok := constInt(b.Y)
			if name, lc := rw.CallName(b.Y); !ok && name == "String_length" && len(lc.Args) == 1 {
				if cs, isConst := constString(lc.Args[0]); isConst && asciiOnly(cs) {
					kk, ok = int64(len(cs)), true // the length of an ASCII constant, rewritten to int32(len(...))
				}
			}
			if !ok || kk < 0 || kk > int64(subLen) {
				return "the index is used in other arithmetic"
			}
			e, k = b, kk
			p = rw.Up(b)
		}
		sc, ok := p.(*ast.CallExpr)
		if !ok {
			return "the index has another use"
		}
		cn, _ := rw.CallName(sc)
		if (cn != "String_substring" && cn != "String_substring_int") || len(sc.Args) < 2 {
			return "the index has another use"
		}
		if id, ok := rw.Unparen(sc.Args[0]).(*ast.Ident); !ok || rw.Info.Uses[id] != sv {
			return "the index cuts another string"
		}
		var gen func() string
		switch {
		case cn == "String_substring_int" && rw.Unparen(sc.Args[1]) == e:
			if k != 1 && !nonNegative(u, iv) {
				return "a substring bound that Java rejects when the index is -1 is not guarded"
			}
			gen = func() string { return rw.Render(sc.Args[0]) + "[" + rw.Render(e) + ":]" }
		case cn == "String_substring" && len(sc.Args) == 3 && rw.Unparen(sc.Args[2]) == e && k == 0 && isZero(sc.Args[1]):
			if !nonNegative(u, iv) {
				return "a substring bound that Java rejects when the index is -1 is not guarded"
			}
			gen = func() string { return rw.Render(sc.Args[0]) + "[:" + rw.Render(e) + "]" }
		default:
			return "the index is a substring bound with another bound"
		}
		cuts = append(cuts, cut{sc, gen})
	}
	if !pkgCall(c, "strings") {
		return ""
	}
	fn := map[string]string{"String_indexOf": "IndexByte", "String_lastIndexOf": "LastIndexByte", "String_indexOf_String": "Index", "String_lastIndexOf_String": "LastIndex"}[name]
	count[name[7:]+" of an ASCII char, used to cut the same string: strings."+fn+" and s[a:b]"]++
	count["substring at such an index: s[a:b]"] += len(cuts)
	rw.Rewrite[c] = func() string {
		sv := sub
		if !isChar {
			sv = rw.Render(c.Args[1])
		}
		return "int32(strings." + fn + "(" + rw.Render(c.Args[0]) + ", " + sv + "))"
	}
	for _, ct := range cuts {
		rw.Rewrite[ct.call] = ct.gen
	}
	return ""
}

var flippedOps = map[token.Token]bool{token.EQL: true, token.NEQ: true, token.LSS: true, token.GTR: true, token.LEQ: true, token.GEQ: true}

func isZero(e ast.Expr) bool {
	k, ok := constInt(e)
	return ok && k == 0
}

// assignedAfterDef reports whether v is assigned anywhere but its definition, or its address
// is taken.
func assignedAfterDef(v *types.Var) bool {
	for _, u := range rw.UsesOf[v] {
		switch p := rw.Parent[u].(type) {
		case *ast.AssignStmt:
			for _, l := range p.Lhs {
				if l == ast.Expr(u) {
					return true
				}
			}
		case *ast.IncDecStmt:
			return true
		case *ast.UnaryExpr:
			if p.Op == token.AND {
				return true
			}
		}
	}
	return false
}

// nonNegative reports whether use u of i is guarded against i == -1: inside the body of an if
// whose condition is (a conjunction with) i != -1, i >= 0, i > -1, i > 0 or i >= 1, or after an
// if statement of the same or an enclosing list whose condition is i == -1, i < 0 or i <= -1
// (or a disjunction with it) and whose body ends in a terminating statement.
func nonNegative(u *ast.Ident, iv *types.Var) bool {
	var child ast.Node = u
	for p := rw.Parent[u]; p != nil; child, p = p, rw.Parent[p] {
		switch x := p.(type) {
		case *ast.FuncLit, *ast.FuncDecl:
			return false
		case *ast.IfStmt:
			if child == ast.Node(x.Body) && impliesPresent(x.Cond, iv) {
				return true
			}
		case *ast.BlockStmt:
			for _, s := range x.List {
				if s == child {
					break
				}
				if is, ok := s.(*ast.IfStmt); ok && is.Init == nil && is.Else == nil && impliesAbsent(is.Cond, iv) && listEndsTerminating(is.Body.List) {
					return true
				}
			}
		}
	}
	return false
}

func listEndsTerminating(l []ast.Stmt) bool {
	if len(l) == 0 {
		return false
	}
	switch x := l[len(l)-1].(type) {
	case *ast.ReturnStmt:
		return true
	case *ast.BranchStmt:
		return x.Tok == token.CONTINUE || x.Tok == token.BREAK || x.Tok == token.GOTO
	case *ast.ExprStmt:
		if c, ok := x.X.(*ast.CallExpr); ok {
			if id, ok := c.Fun.(*ast.Ident); ok && id.Name == "panic" {
				return true
			}
		}
	}
	return false
}

// cmpOn returns the comparison of iv with a constant in e: the operator with iv on the left.
func cmpOn(e ast.Expr, iv *types.Var) (token.Token, int64, bool) {
	b, ok := rw.Unparen(e).(*ast.BinaryExpr)
	if !ok {
		return 0, 0, false
	}
	if id, ok := rw.Unparen(b.X).(*ast.Ident); ok && rw.Info.Uses[id] == iv {
		k, ok := constInt(b.Y)
		return b.Op, k, ok
	}
	return 0, 0, false
}

func impliesPresent(e ast.Expr, iv *types.Var) bool {
	if b, ok := rw.Unparen(e).(*ast.BinaryExpr); ok && b.Op == token.LAND {
		return impliesPresent(b.X, iv) || impliesPresent(b.Y, iv)
	}
	op, k, ok := cmpOn(e, iv)
	if !ok {
		return false
	}
	return (op == token.NEQ && k == -1) || (op == token.GEQ && k >= 0) || (op == token.GTR && k >= -1)
}

func impliesAbsent(e ast.Expr, iv *types.Var) bool {
	if b, ok := rw.Unparen(e).(*ast.BinaryExpr); ok && b.Op == token.LOR {
		return impliesAbsent(b.X, iv) || impliesAbsent(b.Y, iv)
	}
	op, k, ok := cmpOn(e, iv)
	if !ok {
		return false
	}
	// the if body runs at least whenever i == -1
	return (op == token.EQL && k == -1) || (op == token.LSS && k >= 0) || (op == token.LEQ && k >= -1)
}

// ---- String.format of %s and %d only

// format rewrites String.format(template, args) whose template is a constant with only %s, %d
// (without flags, width, precision or index), %n and %% into a concatenation: %s of an Object
// is String.valueOf (objstr_; of a String sstr_ / the constant), %d of an int or long its
// decimal form (istr_), as the runtime's Formatter does. Java evaluates every argument before
// formatting any; the concatenation formats each after evaluating it, which is the same when
// formatting runs no code of the program (Strings and numbers), or when there is a single
// argument. Every operand of the concatenation is a constant or a call, so Go evaluates them
// left to right.
func format(c *ast.CallExpr) string {
	t, ok := constString(c.Args[0])
	if !ok || t == nullstr {
		return "template is not a constant"
	}
	cl, ok := rw.Unparen(c.Args[1]).(*ast.CompositeLit)
	if !ok {
		return "arguments are not an array literal"
	}
	var parts []func() string
	var convs []byte
	lit := ""
	flush := func() {
		if lit != "" {
			l := lit
			parts = append(parts, func() string { return strconv.Quote(l) })
			lit = ""
		}
	}
	for i := 0; i < len(t); i++ {
		if t[i] != '%' {
			lit += string(t[i])
			continue
		}
		if i+1 >= len(t) {
			return "malformed template"
		}
		switch t[i+1] {
		case 's', 'd':
			flush()
			convs = append(convs, t[i+1])
			parts = append(parts, nil) // filled below
		case 'n':
			lit += "\n"
		case '%':
			lit += "%"
		default:
			return "conversion other than %s, %d, %n, %% (or with flags, width or precision)"
		}
		i++
	}
	flush()
	if len(convs) != len(cl.Elts) {
		return "the number of arguments is not the number of conversions"
	}
	k := 0
	for i := range parts {
		if parts[i] != nil {
			continue
		}
		a := cl.Elts[k]
		conv := convs[k]
		k++
		box, in := unboxArg(a)
		it := rw.Info.Types[in].Type
		if it == nil {
			return "argument of unknown type"
		}
		switch {
		case conv == 'd':
			b, ok := it.Underlying().(*types.Basic)
			if !ok || (b.Kind() != types.Int32 && b.Kind() != types.Int64 && b.Kind() != types.Int16 && b.Kind() != types.Int8) || box == "" && !isAnyArg(a) {
				return "%d of something other than a Java int, long, short or byte"
			}
			if _, named := it.(*types.Named); named {
				return "%d of an enum value"
			}
			parts[i] = func() string { return "istr_(int64(" + rw.Render(in) + "))" }
		case isString(it) && box == "strobj_":
			if cs, ok := constString(in); ok && cs != nullstr {
				parts[i] = func() string { return rw.Render(in) }
			} else {
				parts[i] = func() string { return "sstr_(" + rw.Render(in) + ")" }
			}
		default:
			if len(cl.Elts) != 1 {
				return "%s of an Object among several arguments (its toString runs after all are evaluated)"
			}
			parts[i] = func() string { return "objstr_(" + rw.Render(a) + ")" }
		}
	}
	count["format of %s/%d only: concatenation"]++
	rw.Rewrite[c] = func() string {
		var out []string
		for _, p := range parts {
			out = append(out, p())
		}
		if len(out) == 0 {
			return `""`
		}
		return "(" + strings.Join(out, " + ") + ")"
	}
	return ""
}

// unboxArg splits a boxed array element: strobj_(s), any(x) of a number, or the element.
func unboxArg(e ast.Expr) (string, ast.Expr) {
	e = rw.Unparen(e)
	if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 {
		if id, ok := c.Fun.(*ast.Ident); ok && (id.Name == "strobj_" || id.Name == "any") {
			return id.Name, rw.Unparen(c.Args[0])
		}
	}
	return "", e
}

// isAnyArg reports whether e is a number converted to any implicitly in the array literal.
func isAnyArg(e ast.Expr) bool {
	b, ok := rw.Info.Types[e].Type.Underlying().(*types.Basic)
	return ok && b.Info()&types.IsInteger != 0
}

// generated reports whether f is a file j2go generated (g_*.go).
func generated(f *ast.File) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.File(f.Pos()).Name()), "g_")
}
