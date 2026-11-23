// Command enums is a post-flip step: a Java enum becomes an integer type.
//
// j2go writes an enum as a class whose constants are objects: a struct embedding the runtime's
// Enum (name_, ordinal_, self_), a constructor per Java constructor, one package variable per
// constant set in the static initializer, and pointer-typed values everywhere:
//
//	type Tri struct {
//		Enum
//		value_ int32
//	}
//	var Tri_FALSE *Tri
//	func Tri_clinit() { ...; Tri_FALSE = new_Tri("FALSE", 0, -1); ... }
//	func (this *Tri) or(x *Tri) *Tri { if this.value_ > x.value_ { ... } }
//
// Here it becomes a uint8 whose zero value is Java's null and whose constants are ordinal + 1, in
// ordinal order; fields are tables indexed by the constant; the runtime's Enum record of each
// constant (name, ordinal, the constants of its enum for EnumSet) is a table too:
//
//	type Tri uint8
//	var Tri_enum_ = [3]Enum{{self_: Tri_FALSE, name_: "FALSE", ordinal_: 0}, ...}
//	var Tri_value__ [3]int32
//	const Tri_FALSE Tri = 1
//	func Tri_clinit() { ...; Tri_FALSE.init_Tri(-1); ... }
//	func (this Tri) or(x Tri) Tri { if Tri_value__[this-1] > Tri_value__[x-1] { ... } }
//
// Zero as null keeps Java's defaults: a field, array element or map miss of enum type is null, as
// the nil pointer was. The rewrite at each use:
//   - the type *T is T (declarations, conversions, assertions, type arguments, receivers);
//   - nil of type *T is 0; x == nil is x == 0;
//   - x.f, a field of the enum, is T_f_[x-1]; x.Enum.name(), x.name_, x.ordinal_ are x.name()
//     and x.ordinal(); x.up_Enum() is x.enum_();
//   - the constructors are gone; T_X = new_T("X", k, args) in the static initializer is
//     T_X.init_T(args), which fills the field tables, or nothing when the enum has no fields;
//   - each enum gets name, ordinal, toString (unless it has its own), compareTo, self and enum_
//     as value methods, so code in the `any` world (Object_toString, EnumSet, ordinalCmp_,
//     Class.getEnumConstants, boxValue_) sees what it saw; to_Enum and is_Enum (g_zstubs.go)
//     assert the runtime's enumConst_ (enum_()), which object enums have too. A constant boxed
//     into an `any` is a byte: Go boxes it without allocating (runtime.staticuint64s).
//
// Identity: equality of constants is equality of the integers. Enum.hashCode is identity in
// Java; the runtime hashes a converted constant by the address of its Enum record (identityHash_),
// as it hashed the object before (the record was at the object's address). compareTo is
// ordinal order, as before.
//
// An enum is left as it is when it is declared in the handwritten runtime, when a constant has a
// body (a class extends the enum), when the runtime refers to it, or when a value is used in a way
// the rewrite does not know (dereferenced, its self_ read, ...); the tool lists them with -n.
//
// The Token enum: the tokens step stores a Node's token as ordinal + 1 in a byte, which is now
// the Token value itself: Token_rhino_ord1_(t) is uint8(t) and Token_rhino_byOrd1_[b] is
// Token_rhino(b).
//
// usage: enums [-n] [-survey] <dir of package closure>
package main

import (
	"flag"
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

type enumInfo struct {
	name       string
	obj        *types.TypeName
	named      *types.Named
	ptr        types.Type
	st         *types.Struct
	fields     []*types.Var
	consts     []string // constant variable names by ordinal
	cnames     []string // Java names by ordinal
	subclassed bool
	clinit     *ast.FuncDecl
	decl       *ast.GenDecl // type T struct {...}
	news       map[string]*ast.FuncDecl
	inits      map[string]*ast.FuncDecl
	upEnum     *ast.FuncDecl
	constDecls map[string]*ast.GenDecl
	ownMethods map[string]bool
	why        string
	// keepNameOrd: the init methods keep their name_ and ordinal_ parameters.
	keepNameOrd bool
	deadInits   map[string]bool
}

func isGen(pos token.Pos) bool {
	return strings.HasPrefix(filepath.Base(rw.Fset.Position(pos).Filename), "g_")
}

func findEnums() []*enumInfo {
	enumT := rw.Pkg.Scope().Lookup("Enum").Type()
	var out []*enumInfo
	byNamed := map[*types.Named]*enumInfo{}
	sc := rw.Pkg.Scope()
	for _, name := range sc.Names() {
		tn, ok := sc.Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		n, ok := tn.Type().(*types.Named)
		if !ok {
			continue
		}
		st, ok := n.Underlying().(*types.Struct)
		if !ok || st.NumFields() == 0 || !st.Field(0).Embedded() || !types.Identical(st.Field(0).Type(), enumT) {
			continue
		}
		e := &enumInfo{name: name, obj: tn, named: n, ptr: types.NewPointer(n), st: st,
			news: map[string]*ast.FuncDecl{}, inits: map[string]*ast.FuncDecl{}, constDecls: map[string]*ast.GenDecl{},
			ownMethods: map[string]bool{}, deadInits: map[string]bool{}}
		for i := 1; i < st.NumFields(); i++ {
			e.fields = append(e.fields, st.Field(i))
		}
		for i := 0; i < n.NumMethods(); i++ {
			e.ownMethods[n.Method(i).Name()] = true
		}
		out = append(out, e)
		byNamed[n] = e
	}
	for _, name := range sc.Names() {
		tn, ok := sc.Lookup(name).(*types.TypeName)
		if !ok {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok {
			continue
		}
		for i := 0; i < st.NumFields(); i++ {
			if f := st.Field(i); f.Embedded() {
				if fn, ok := f.Type().(*types.Named); ok && byNamed[fn] != nil {
					byNamed[fn].subclassed = true
				}
			}
		}
	}
	byName := map[string]*enumInfo{}
	for _, e := range out {
		byName[e.name] = e
	}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				if d.Tok == token.TYPE && len(d.Specs) == 1 {
					if ts := d.Specs[0].(*ast.TypeSpec); byName[ts.Name.Name] != nil {
						byName[ts.Name.Name].decl = d
					}
				}
				if d.Tok == token.VAR && len(d.Specs) == 1 {
					vs := d.Specs[0].(*ast.ValueSpec)
					if len(vs.Names) == 1 && len(vs.Values) == 0 {
						if st, ok := vs.Type.(*ast.StarExpr); ok {
							if id, ok := st.X.(*ast.Ident); ok && byName[id.Name] != nil {
								byName[id.Name].constDecls[vs.Names[0].Name] = d
							}
						}
					}
				}
			case *ast.FuncDecl:
				if d.Recv != nil {
					rt := d.Recv.List[0].Type
					if st, ok := rt.(*ast.StarExpr); ok {
						rt = st.X
					}
					if id, ok := rt.(*ast.Ident); ok && byName[id.Name] != nil {
						e := byName[id.Name]
						if d.Name.Name == "up_Enum" {
							e.upEnum = d
						} else if d.Name.Name == "init_"+e.name || strings.HasPrefix(d.Name.Name, "init_"+e.name+"_") {
							e.inits[d.Name.Name] = d
						}
					}
					continue
				}
				for _, e := range out {
					if d.Name.Name == e.name+"_clinit" {
						e.clinit = d
					}
					if d.Name.Name == "new_"+e.name || strings.HasPrefix(d.Name.Name, "new_"+e.name+"_") {
						if r := d.Type.Results; r != nil && len(r.List) == 1 {
							if st, ok := r.List[0].Type.(*ast.StarExpr); ok {
								if id, ok := st.X.(*ast.Ident); ok && id.Name == e.name {
									e.news[d.Name.Name] = d
								}
							}
						}
					}
				}
			}
		}
	}
	for _, e := range out {
		if e.clinit == nil {
			continue
		}
		for _, s := range e.clinit.Body.List {
			lhs, call := newAssign(e, s)
			if call == nil {
				continue
			}
			lit, ok1 := call.Args[0].(*ast.BasicLit)
			ord, ok2 := call.Args[1].(*ast.BasicLit)
			if !ok1 || !ok2 {
				continue
			}
			k, _ := strconv.Atoi(ord.Value)
			jn, _ := strconv.Unquote(lit.Value)
			for len(e.consts) <= k {
				e.consts = append(e.consts, "")
				e.cnames = append(e.cnames, "")
			}
			e.consts[k] = lhs
			e.cnames[k] = jn
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].name < out[j].name })
	return out
}

// newAssign returns the constant and the constructor call of a statement X = new_T...(...).
func newAssign(e *enumInfo, s ast.Stmt) (string, *ast.CallExpr) {
	as, ok := s.(*ast.AssignStmt)
	if !ok || len(as.Lhs) != 1 || as.Tok != token.ASSIGN {
		return "", nil
	}
	call, ok := as.Rhs[0].(*ast.CallExpr)
	if !ok || len(call.Args) < 2 {
		return "", nil
	}
	id, ok := call.Fun.(*ast.Ident)
	if !ok || e.news[id.Name] == nil {
		return "", nil
	}
	lid, ok := as.Lhs[0].(*ast.Ident)
	if !ok {
		return "", nil
	}
	return lid.Name, call
}

// standardNew reports whether fd is this := &T{}; this.self_ = this; [this.zero_T();]
// this.init_T...(params); return this, and returns the init method it calls and the zero method.
func standardNew(e *enumInfo, fd *ast.FuncDecl) (string, string) {
	b := fd.Body.List
	zero := ""
	if len(b) == 5 && rw.Text(b[2]) == "this.zero_"+e.name+"()" {
		zero = "zero_" + e.name
		b = append(b[:2:2], b[3:]...)
	}
	if len(b) != 4 {
		return "", ""
	}
	if !strings.HasPrefix(rw.Text(b[0]), "this := &"+e.name+"{}") || rw.Text(b[1]) != "this.self_ = this" || rw.Text(b[3]) != "return this" {
		return "", ""
	}
	es, ok := b[2].(*ast.ExprStmt)
	if !ok {
		return "", ""
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok {
		return "", ""
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok || rw.Text(sel.X) != "this" || e.inits[sel.Sel.Name] == nil {
		return "", ""
	}
	// the parameters are passed on in order
	var ps []string
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			ps = append(ps, n.Name)
		}
	}
	if len(ps) != len(call.Args) {
		return "", ""
	}
	for i, a := range call.Args {
		if rw.Text(a) != ps[i] {
			return "", ""
		}
	}
	return sel.Sel.Name, zero
}

var byNamed = map[*types.Named]*enumInfo{}

// ptrEnum is the enum whose pointer type is the type of x.
func ptrEnum(x ast.Expr) *enumInfo {
	tv, ok := rw.Info.Types[x]
	if !ok || tv.Type == nil {
		return nil
	}
	p, ok := tv.Type.(*types.Pointer)
	if !ok {
		return nil
	}
	n, ok := p.Elem().(*types.Named)
	if !ok {
		return nil
	}
	return byNamed[n]
}

func enclosingFunc(n ast.Node) *ast.FuncDecl {
	for n != nil {
		if fd, ok := n.(*ast.FuncDecl); ok {
			return fd
		}
		n = rw.Parent[n]
	}
	return nil
}

func isNilIdent(x ast.Expr) bool {
	id, ok := rw.Unparen(x).(*ast.Ident)
	return ok && id.Name == "nil" && rw.Info.Uses[id] == types.Universe.Lookup("nil")
}

// check sets e.why when the enum cannot be converted.
func check(enums []*enumInfo) {
	for _, e := range enums {
		switch {
		case !isGen(e.obj.Pos()):
			e.why = "declared in the runtime"
		case e.subclassed:
			e.why = "a constant has a body (subclass)"
		case e.decl == nil || e.clinit == nil:
			e.why = "no declaration or static initializer found"
		case len(e.consts) == 0:
			e.why = "no constants found"
		case len(e.consts) > 254:
			e.why = "more than 254 constants"
		}
		if e.why != "" {
			continue
		}
		for k, c := range e.consts {
			if c == "" || e.constDecls[c] == nil {
				e.why = fmt.Sprintf("constant %d not found", k)
				break
			}
		}
		for n, fd := range e.news {
			if i, _ := standardNew(e, fd); i == "" {
				e.why = "constructor " + n + " is not the standard one"
			}
		}
	}
	// uses of constant variables, constructors and init methods
	for _, e := range enums {
		if e.why != "" {
			continue
		}
		for _, c := range e.consts {
			obj := rw.Pkg.Scope().Lookup(c)
			for _, id := range rw.UsesOf[obj] {
				if !isGen(id.Pos()) {
					e.why = "the runtime uses " + c
					break
				}
				if as, ok := rw.Parent[id].(*ast.AssignStmt); ok && len(as.Lhs) == 1 && as.Lhs[0] == id {
					if l, call := newAssign(e, as); l == "" || call == nil || enclosingFunc(as) != e.clinit {
						e.why = "constant " + c + " is assigned outside its initializer"
					}
					continue
				}
				if u, ok := rw.Parent[id].(*ast.UnaryExpr); ok && u.Op == token.AND {
					e.why = "address of constant " + c
				}
			}
		}
		for n := range e.news {
			for _, id := range rw.UsesOf[rw.Pkg.Scope().Lookup(n)] {
				call, ok := rw.Parent[id].(*ast.CallExpr)
				st := rw.Parent[call]
				if !ok || call.Fun != id || st == nil || rw.Parent[st] == nil || enclosingFunc(call) != e.clinit {
					e.why = "constructor " + n + " is called outside the initializer"
					break
				}
				if as, ok := st.(*ast.AssignStmt); !ok {
					e.why = "constructor " + n + " is not assigned to a constant"
				} else if l, _ := newAssign(e, as); l == "" {
					e.why = "constructor " + n + " is not assigned to a constant"
				}
			}
		}
		for n, fd := range e.inits {
			obj := rw.Info.Defs[fd.Name]
			for _, id := range rw.UsesOf[obj] {
				f := enclosingFunc(id)
				if f == nil || (e.news[f.Name.Name] == nil && e.inits[f.Name.Name] == nil) {
					e.why = "init method " + n + " is called from " + fmt.Sprint(f.Name.Name)
				}
			}
		}
		// the Enum fields of an init method's receiver: only this.Enum.init_Enum(name_, ordinal_)
		for _, fd := range e.inits {
			ps := fd.Type.Params.List
			if len(ps) == 0 || len(ps[0].Names) == 0 || ps[0].Names[0].Name != "name_" {
				e.why = "init method without name_"
				continue
			}
			names := map[types.Object]bool{}
			for _, f := range ps {
				for _, n := range f.Names {
					if n.Name == "name_" || n.Name == "ordinal_" {
						names[rw.Info.Defs[n]] = true
					}
				}
			}
			ast.Inspect(fd.Body, func(n ast.Node) bool {
				id, ok := n.(*ast.Ident)
				if !ok || !names[rw.Info.Uses[id]] {
					return true
				}
				call, ok := rw.Parent[id].(*ast.CallExpr)
				if ok {
					if i := argIndex(call, id); i == 0 || i == 1 {
						if s, ok := call.Fun.(*ast.SelectorExpr); ok && (s.Sel.Name == "init_Enum" || e.inits[s.Sel.Name] != nil) {
							return true
						}
					}
				}
				e.keepNameOrd = true
				return true
			})
		}
	}
	// the type name
	for _, e := range enums {
		if e.why != "" {
			continue
		}
		for _, id := range rw.UsesOf[e.obj] {
			if !isGen(id.Pos()) {
				e.why = "the runtime uses the type"
				break
			}
			switch p := rw.Parent[id].(type) {
			case *ast.StarExpr:
			case *ast.CompositeLit:
				if f := enclosingFunc(p); f == nil || e.news[f.Name.Name] == nil {
					e.why = "composite literal of the struct"
				}
			default:
				e.why = fmt.Sprintf("type used as %T", p)
			}
		}
	}
	// values of type *T
	for _, f := range rw.Files {
		gen := isGen(f.Pos())
		ast.Inspect(f, func(n ast.Node) bool {
			x, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			e := ptrEnum(x)
			if e == nil || e.why != "" {
				return true
			}
			if tv := rw.Info.Types[x]; !tv.IsValue() && !tv.IsNil() {
				return true
			}
			if !gen {
				e.why = "the runtime uses a value"
				return true
			}
			fd := enclosingFunc(x)
			inNew := fd != nil && e.news[fd.Name.Name] != nil
			switch p := rw.Up(x).(type) {
			case *ast.StarExpr:
				e.why = "dereferenced"
			case *ast.SelectorExpr:
				s := rw.Info.Selections[p]
				if s == nil || s.Kind() != types.FieldVal {
					return true
				}
				switch p.Sel.Name {
				case "self_":
					if !inNew {
						e.why = "self_ read"
					}
				case "name_", "ordinal_", "values_":
				case "Enum":
					q, ok := rw.Up(p).(*ast.SelectorExpr)
					if !ok {
						if !(fd == e.upEnum) {
							e.why = "Enum field used"
						}
						return true
					}
					switch q.Sel.Name {
					case "name", "ordinal", "toString", "name_", "ordinal_":
					case "init_Enum":
						if fd == nil || e.inits[fd.Name.Name] == nil {
							e.why = "init_Enum outside init"
						}
					default:
						e.why = "Enum." + q.Sel.Name + " used"
					}
				default:
					if len(s.Index()) != 1 {
						e.why = "promoted field " + p.Sel.Name
					}
				}
			}
			return true
		})
	}
}

func argIndex(call *ast.CallExpr, x ast.Expr) int {
	for i, a := range call.Args {
		if rw.Unparen(a) == x {
			return i
		}
	}
	return -1
}

func main() {
	surv := flag.Bool("survey", false, "print how enum values are used")
	dry := flag.Bool("n", false, "list the enums and why some are left; change nothing")
	flag.Parse()
	rw.Load(flag.Arg(0))
	enums := findEnums()
	for _, e := range enums {
		byNamed[e.named] = e
	}
	if *surv {
		survey(enums)
		return
	}
	check(enums)
	var conv []*enumInfo
	left := map[string][]string{}
	for _, e := range enums {
		if e.why == "" {
			conv = append(conv, e)
		} else {
			left[e.why] = append(left[e.why], e.name)
		}
	}
	var whys []string
	for w := range left {
		whys = append(whys, w)
	}
	sort.Strings(whys)
	for _, w := range whys {
		fmt.Fprintf(os.Stderr, "left (%s): %d: %s\n", w, len(left[w]), strings.Join(left[w], " "))
	}
	if *dry {
		for _, e := range conv {
			fmt.Fprintf(os.Stderr, "convert %s (%d constants, %d fields)\n", e.name, len(e.consts), len(e.fields))
		}
		return
	}
	for n, e := range byNamed {
		if e.why != "" {
			delete(byNamed, n)
		}
	}
	st := rewrite(conv)
	fmt.Fprintf(os.Stderr, "enums: %d of %d become integer types (%d constants, %d field tables); %s; files changed: %d\n",
		len(conv), len(enums), st.consts, st.tables, st.String(), rw.Save())
}
