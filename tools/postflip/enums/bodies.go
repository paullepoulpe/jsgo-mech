package main

// The -bodies phase: an enum whose constants have bodies becomes an enum without them.
//
// j2go writes a constant with a body as a class of its own (C) embedding the enum's class (T),
// with a constructor, an init method, its Enum boilerplate (values, valueOf, a static initializer
// that registers an empty anonymous enum) and the constant's methods as m_impl. T's abstract
// method m is a dispatcher. vdispatch -enums gave T a field selfI_ (an interface T_I of the
// m_impl methods, or, after funcparams, a func value) set by each constant's constructor, and
// each C an override of m that calls its own m_impl when the object is exactly a C:
//
//	type T struct { Enum; selfI_ T_I }
//	func (this *T) m(p0 P) R { return this.selfI_.m_impl(p0) }
//	type T_1 struct { T }
//	func new_T_1(name_ string, ordinal_ int32) *T_1 { ...; this.selfI_ = this; ... }
//	func (this *T_1) m_impl(x P) R { body1 }
//	func (this *T_1) m(p0 P) R { if p, _ := this.self_.(*T_1); p == this { ... } ... }
//	T_A = new_T_1("A", 0).up_T()
//
// Here, as jsgo writes it, T's method m switches on the constant and each constant's body is a
// case (constants whose bodies are the same text share a case); the classes C, their
// boilerplate, selfI_, T_I and the overrides are gone, and each constant is made by T's own
// constructor:
//
//	type T struct { Enum }
//	func (this *T) m(x P) R {
//		switch this {
//		case T_A:
//			body1
//		...
//		default:
//			panic(new_NullPointerException_0().self()) // a call on null
//		}
//	}
//	T_A = new_T("A", 0)
//
// The enum is then an ordinary enum, which the main phase (run next, without -bodies) makes an
// integer type. A body is a case of the switch only when it is safe there: the constants agree
// on the parameter names, and the body has no label or goto (labels are per function) and no
// break that would leave the switch; otherwise it stays a method of T named m_<CONSTANT> that
// the case calls. Java's behaviour: an enum constant is a singleton, so `this` is exactly one of
// the constants, and the switch selects the body Java's virtual call selected; a null `this`
// throws NullPointerException after the arguments are evaluated, as the call on null did (the
// dispatcher dereferenced the nil object). Inside a body, this.up_T() (the constant as its enum)
// is this. The anonymous classes' static initializers only registered an empty enum under the
// name "<anonymous ...>", which nothing looks up.
//
// An enum is left as it is (and listed) when a constant class has fields or methods other than
// these, an init method or constructor that does more, or a dispatcher the phase does not know.
// The phase type-checks the result before writing (rw.SaveChecked).

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

type bodyEnum struct {
	t        *enumInfo
	consts   []*constClass // by ordinal
	why      string
	disp     []*dispatcher
	selfI    *types.Var
	ifaceDec *ast.GenDecl // type T_I interface
	selfFn   *ast.FuncDecl
	toFn     *ast.FuncDecl
}

type constClass struct {
	name     string
	tn       *types.TypeName
	constVar string // T_A
	cname    string // A
	decls    []ast.Node
	newFn    *ast.FuncDecl
	clinit   *ast.AssignStmt // T_A = new_C("A", k).up_T()
	call     *ast.CallExpr   // new_C("A", k)
	impls    map[string]*ast.FuncDecl
	override map[string]*ast.FuncDecl
}

type dispatcher struct {
	fd   *ast.FuncDecl
	impl string
}

func funcDecls() map[string]*ast.FuncDecl {
	m := map[string]*ast.FuncDecl{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil {
				m[fd.Name.Name] = fd
			}
		}
	}
	return m
}

func recvName(fd *ast.FuncDecl) string {
	if fd.Recv == nil {
		return ""
	}
	rt := fd.Recv.List[0].Type
	if st, ok := rt.(*ast.StarExpr); ok {
		rt = st.X
	}
	if id, ok := rt.(*ast.Ident); ok {
		return id.Name
	}
	return ""
}

func paramNames(fd *ast.FuncDecl) []string {
	var ps []string
	for _, f := range fd.Type.Params.List {
		for _, n := range f.Names {
			ps = append(ps, n.Name)
		}
	}
	return ps
}

// callsWithParams reports whether call passes fd's parameters on, in order.
func callsWithParams(fd *ast.FuncDecl, call *ast.CallExpr) bool {
	ps := paramNames(fd)
	if len(ps) != len(call.Args) {
		return false
	}
	for i, a := range call.Args {
		if rw.Text(a) != ps[i] {
			return false
		}
	}
	return true
}

// singleCall is the call of a body that is one statement `return f(...)` or `f(...)`.
func singleCall(fd *ast.FuncDecl) *ast.CallExpr {
	if fd.Body == nil || len(fd.Body.List) != 1 {
		return nil
	}
	var x ast.Expr
	switch s := fd.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(s.Results) == 1 {
			x = s.Results[0]
		}
	case *ast.ExprStmt:
		x = s.X
	}
	c, _ := x.(*ast.CallExpr)
	return c
}

// inlinable reports whether a method body can be a case of a switch in another function.
func inlinable(body *ast.BlockStmt) bool {
	ok := true
	var walk func(n ast.Node, loops int)
	walk = func(n ast.Node, depth int) {
		ast.Inspect(n, func(c ast.Node) bool {
			if c == n {
				return true
			}
			switch c := c.(type) {
			case *ast.FuncLit:
				return false
			case *ast.LabeledStmt:
				ok = false
			case *ast.BranchStmt:
				if c.Tok == token.GOTO || c.Tok == token.FALLTHROUGH || (c.Tok == token.BREAK && depth == 0) {
					ok = false
				}
			case *ast.ForStmt, *ast.RangeStmt, *ast.SwitchStmt, *ast.TypeSwitchStmt, *ast.SelectStmt:
				walk(c, depth+1)
				return false
			}
			return true
		})
	}
	walk(body, 0)
	return ok
}

func bodies(enums []*enumInfo) {
	fns := funcDecls()
	byT := map[*types.Named]*bodyEnum{}
	var bes []*bodyEnum
	for _, e := range enums {
		if e.subclassed && isGen(e.obj.Pos()) {
			be := &bodyEnum{t: e}
			byT[e.named] = be
			bes = append(bes, be)
		}
	}
	// the constant classes: types whose only field is the embedded enum
	classes := map[string]*constClass{}
	sc := rw.Pkg.Scope()
	for _, name := range sc.Names() {
		tn, ok := sc.Lookup(name).(*types.TypeName)
		if !ok || tn.IsAlias() {
			continue
		}
		st, ok := tn.Type().Underlying().(*types.Struct)
		if !ok || st.NumFields() == 0 || !st.Field(0).Embedded() {
			continue
		}
		n, ok := st.Field(0).Type().(*types.Named)
		if !ok || byT[n] == nil {
			continue
		}
		be := byT[n]
		if st.NumFields() != 1 {
			be.why = "constant class " + name + " has fields"
			continue
		}
		classes[name] = &constClass{name: name, tn: tn, impls: map[string]*ast.FuncDecl{}, override: map[string]*ast.FuncDecl{}}
	}
	classOf := map[*constClass]*bodyEnum{}
	for _, c := range classes {
		classOf[c] = byT[c.tn.Type().Underlying().(*types.Struct).Field(0).Type().(*types.Named)]
	}
	// declarations
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.GenDecl:
				for _, s := range d.Specs {
					var names []string
					switch s := s.(type) {
					case *ast.TypeSpec:
						names = []string{s.Name.Name}
					case *ast.ValueSpec:
						for _, n := range s.Names {
							names = append(names, n.Name)
						}
					}
					for _, n := range names {
						for _, c := range classes {
							if n == c.name || n == c.name+"_values_" || n == c.name+"_clinit_done" {
								if len(d.Specs) != 1 || len(names) != 1 {
									classOf[c].why = "declaration of " + n + " shares its line"
								}
								c.decls = append(c.decls, d)
							}
						}
						for _, be := range bes {
							if n == be.t.name+"_I" {
								be.ifaceDec = d
							}
						}
					}
				}
			case *ast.FuncDecl:
				r := recvName(d)
				if c := classes[r]; c != nil {
					switch {
					case d.Name.Name == "up_"+classOf[c].t.name || d.Name.Name == "up_Enum" || d.Name.Name == "as_"+c.name:
						c.decls = append(c.decls, d)
					case d.Name.Name == "init_"+c.name:
						// this.T.init_T(name_, ordinal_)
						call := singleCall(d)
						if call == nil || !callsWithParams(d, call) || rw.Text(call.Fun) != "this."+classOf[c].t.name+".init_"+classOf[c].t.name {
							classOf[c].why = "init method of " + c.name + " does more than init the enum"
						}
						c.decls = append(c.decls, d)
					case strings.HasSuffix(d.Name.Name, "_impl"):
						c.impls[d.Name.Name] = d
					default:
						c.override[d.Name.Name] = d
					}
					continue
				}
				if r != "" {
					continue
				}
				for _, c := range classes {
					switch d.Name.Name {
					case "new_" + c.name:
						c.newFn = d
						c.decls = append(c.decls, d)
					case c.name + "_values", c.name + "_valueOf", c.name + "_clinit", "to_" + c.name, "is_" + c.name:
						c.decls = append(c.decls, d)
					}
				}
			}
		}
	}
	for _, be := range bes {
		T := be.t.name
		if be.why != "" {
			continue
		}
		if be.t.clinit == nil {
			be.why = "no static initializer"
			continue
		}
		// constants
		for _, s := range be.t.clinit.Body.List {
			as, ok := s.(*ast.AssignStmt)
			if !ok || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
				continue
			}
			up, ok := as.Rhs[0].(*ast.CallExpr)
			if !ok || len(up.Args) != 0 {
				continue
			}
			sel, ok := up.Fun.(*ast.SelectorExpr)
			if !ok || sel.Sel.Name != "up_"+T {
				continue
			}
			call, ok := sel.X.(*ast.CallExpr)
			if !ok || len(call.Args) != 2 {
				continue
			}
			id, ok := call.Fun.(*ast.Ident)
			if !ok || !strings.HasPrefix(id.Name, "new_") || classes[id.Name[4:]] == nil {
				continue
			}
			c := classes[id.Name[4:]]
			lit, ok := call.Args[0].(*ast.BasicLit)
			if !ok || lit.Kind != token.STRING {
				continue
			}
			c.clinit, c.call, c.constVar = as, call, rw.Text(as.Lhs[0])
			c.cname = strings.Trim(lit.Value, `"`)
			be.consts = append(be.consts, c)
		}
		for _, c := range classes {
			if classOf[c] == be && c.clinit == nil {
				be.why = "constant class " + c.name + " is not made in the static initializer"
			}
		}
		if be.why != "" {
			continue
		}
		// T's own constructor: the one of init_T(name_, ordinal_)
		if fd := fns["new_"+T]; fd != nil {
			if i, _ := standardNew(be.t, fd); i != "init_"+T {
				be.why = "new_" + T + " is not the standard constructor"
				continue
			}
		}
		// the selfI_ field and the dispatchers
		st := be.t.st
		for i := 1; i < st.NumFields(); i++ {
			if st.Field(i).Name() == "selfI_" && st.NumFields() == 2 {
				be.selfI = st.Field(i)
			} else {
				be.why = "the enum has field " + st.Field(i).Name()
			}
		}
		if be.why != "" {
			continue
		}
		// func-valued selfI_: the method each constructor stores
		funcImpl := ""
		for _, c := range be.consts {
			if c.newFn == nil {
				be.why = "no constructor for " + c.name
				break
			}
			for _, s := range c.newFn.Body.List {
				if as, ok := s.(*ast.AssignStmt); ok && rw.Text(as.Lhs[0]) == "this.selfI_" {
					r := rw.Text(as.Rhs[0])
					if r != "this" {
						r = strings.TrimPrefix(r, "this.")
						if funcImpl != "" && funcImpl != r {
							be.why = "constructors store different selfI_ methods"
						}
						funcImpl = r
					}
				}
			}
			if i, _ := standardNewIgnoringSelfI(c); !i {
				be.why = "constructor of " + c.name + " is not the standard one"
			}
		}
		if be.why != "" {
			continue
		}
		for _, f := range rw.Files {
			for _, d := range f.Decls {
				fd, ok := d.(*ast.FuncDecl)
				if !ok || recvName(fd) != T {
					continue
				}
				switch fd.Name.Name {
				case "self":
					if strings.Contains(rw.Text(fd.Body), "return this.self_") {
						be.selfFn = fd
					}
					continue
				}
				call := singleCall(fd)
				if call == nil || !callsWithParams(fd, call) {
					continue
				}
				fun := rw.Text(call.Fun)
				impl := ""
				switch {
				case fun == "this.selfI_":
					impl = funcImpl
				case strings.HasPrefix(fun, "this.selfI_."):
					impl = strings.TrimPrefix(fun, "this.selfI_.")
				case strings.HasPrefix(fun, "this.self_.("):
					if s, ok := call.Fun.(*ast.SelectorExpr); ok {
						impl = s.Sel.Name
					}
				default:
					continue
				}
				if impl == "" {
					be.why = "dispatcher " + fd.Name.Name + " without a known method"
					continue
				}
				be.disp = append(be.disp, &dispatcher{fd: fd, impl: impl})
			}
		}
		if be.why != "" {
			continue
		}
		// every constant implements every dispatched method, and nothing else
		implemented := map[string]bool{}
		for _, d := range be.disp {
			implemented[d.impl] = true
			for _, c := range be.consts {
				if c.impls[d.impl] == nil {
					be.why = c.name + " has no " + d.impl
				}
			}
		}
		for _, c := range be.consts {
			for m := range c.impls {
				if !implemented[m] {
					be.why = c.name + "." + m + " is not dispatched to"
				}
			}
			for m, fd := range c.override {
				found := false
				for _, d := range be.disp {
					found = found || d.fd.Name.Name == m
				}
				if !found || !strings.Contains(rw.Text(fd.Body), "this.self_.(*"+c.name+")") {
					be.why = c.name + "." + m + " is not an exact-object override of a dispatcher"
				}
			}
		}
	}
	getClassArgs(byT)

	var conv []*bodyEnum
	for _, be := range bes {
		if be.why != "" {
			fmt.Fprintf(os.Stderr, "bodies: left %s: %s\n", be.t.name, be.why)
			continue
		}
		conv = append(conv, be)
	}
	inl, meth, cases := 0, 0, 0
	for _, be := range conv {
		be := be
		T := be.t.name
		for _, c := range be.consts {
			for _, d := range c.decls {
				rw.Rewrite[d] = func() string { return rw.DelDoc }
			}
			for _, fd := range c.override {
				rw.Rewrite[fd] = func() string { return rw.DelDoc }
			}
			call := c.call
			rw.Rewrite[c.clinit.Rhs[0]] = func() string { return "new_" + T + "(" + renderArgs(call.Args) + ")" }
		}
		if fns["new_"+T] == nil {
			// T's constructor, after T's methods (the first constant's constructor's place)
			nf := be.consts[0].newFn
			rw.Rewrite[nf] = func() string {
				return fmt.Sprintf("func new_%s(name_ string, ordinal_ int32) *%s {\n\tthis := &%s{}\n\tthis.self_ = this\n\tthis.init_%s(name_, ordinal_)\n\treturn this\n}", T, T, T, T)
			}
		}
		if be.selfI != nil {
			stt := be.t.decl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
			for _, fl := range stt.Fields.List {
				if len(fl.Names) == 1 && fl.Names[0].Name == "selfI_" {
					rw.Rewrite[fl] = func() string { return rw.Del }
				}
			}
		}
		if be.ifaceDec != nil {
			d := be.ifaceDec
			rw.Rewrite[d] = func() string { return rw.DelDoc }
		}
		if be.selfFn != nil {
			rw.Rewrite[be.selfFn] = func() string { return rw.DelDoc }
		}
		// this.up_T() in the bodies is this
		for _, c := range be.consts {
			for _, fd := range c.impls {
				ast.Inspect(fd.Body, func(n ast.Node) bool {
					call, ok := n.(*ast.CallExpr)
					if !ok || len(call.Args) != 0 {
						return true
					}
					if s, ok := call.Fun.(*ast.SelectorExpr); ok && s.Sel.Name == "up_"+T && rw.Text(s.X) == "this" {
						rw.Rewrite[call] = func() string { return "this" }
					}
					return true
				})
			}
		}
		for _, d := range be.disp {
			d := d
			impls := make([]*ast.FuncDecl, len(be.consts))
			inline := true
			for i, c := range be.consts {
				impls[i] = c.impls[d.impl]
				inline = inline && inlinable(impls[i].Body) && strings.Join(paramNames(impls[i]), ",") == strings.Join(paramNames(impls[0]), ",")
			}
			for i, c := range be.consts {
				fd := impls[i]
				if inline {
					rw.Rewrite[fd] = func() string { return rw.DelDoc }
					inl++
				} else {
					mname := strings.TrimSuffix(d.impl, "_impl") + "_" + c.cname
					rw.Rewrite[fd.Recv.List[0].Type] = func() string { return "*" + T }
					rw.Rewrite[fd.Name] = func() string { return mname }
					meth++
				}
			}
			var params string
			if inline {
				params = rw.Render(impls[0].Type.Params)
			} else {
				params = rw.Render(d.fd.Type.Params)
			}
			res := ""
			if d.fd.Type.Results != nil {
				res = " " + rw.Render(d.fd.Type.Results)
			}
			call := singleCall(d.fd)
			text := ""
			rw.Rewrite[d.fd] = func() string {
				if text != "" {
					return text
				}
				var b strings.Builder
				fmt.Fprintf(&b, "func (this *%s) %s%s%s {\n\tswitch this {\n", T, d.fd.Name.Name, params, res)
				// constants whose bodies are the same text share a case, at the first one
				var order []string
				group := map[string][]string{}
				for i, c := range be.consts {
					var body string
					if inline {
						s := rw.Render(impls[i].Body)
						body = strings.TrimSpace(s[1 : len(s)-1])
					} else {
						mname := strings.TrimSuffix(d.impl, "_impl") + "_" + c.cname
						body = "this." + mname + "(" + renderArgs(call.Args) + ")"
						if res != "" {
							body = "return " + body
						}
					}
					if group[body] == nil {
						order = append(order, body)
					}
					group[body] = append(group[body], c.constVar)
				}
				for _, body := range order {
					fmt.Fprintf(&b, "\tcase %s:\n\t\t%s\n", strings.Join(group[body], ", "), body)
					cases++
				}
				b.WriteString("\tdefault:\n\t\tpanic(new_NullPointerException_0().self()) // a call on null\n\t}\n}")
				text = b.String()
				return text
			}
		}
	}
	// the anonymous classes' static initializers in the package init
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Name.Name != "init" || fd.Recv != nil {
				continue
			}
			for _, s := range fd.Body.List {
				for _, be := range conv {
					for _, c := range be.consts {
						if rw.Text(s) == "runClinit_("+c.name+"_clinit)" {
							rw.Rewrite[s] = func() string { return rw.Del }
						}
					}
				}
			}
		}
	}
	// to_T: the casts step's type switch over the constant classes is the plain cast again
	for _, be := range conv {
		fd := funcDecls()["to_"+be.t.name]
		if fd == nil || !strings.Contains(rw.Text(fd.Body), "switch p := v.(type)") {
			continue
		}
		T := be.t.name
		rw.Rewrite[fd] = func() string {
			return fmt.Sprintf("func to_%s(v any) *%s {\n\tif v == nil {\n\t\treturn nil\n\t}\n\treturn v.(interface{ as_%s() *%s }).as_%s()\n}", T, T, T, T, T)
		}
	}
	var names []string
	for _, be := range conv {
		names = append(names, be.t.name)
	}
	sort.Strings(names)
	fmt.Fprintf(os.Stderr, "bodies: %d of %d enums with constant bodies lose their constant classes (%s); %d bodies are switch cases (%d cases), %d stay methods; files changed: %d\n",
		len(conv), len(bes), strings.Join(names, " "), inl, cases, meth, rw.SaveChecked())
}

// standardNewIgnoringSelfI: this := &C{}; this.self_ = this; [this.selfI_ = ...;] this.init_C(params); return this.
func standardNewIgnoringSelfI(c *constClass) (bool, string) {
	var b []ast.Stmt
	for _, s := range c.newFn.Body.List {
		if as, ok := s.(*ast.AssignStmt); ok && rw.Text(as.Lhs[0]) == "this.selfI_" {
			continue
		}
		b = append(b, s)
	}
	if len(b) != 4 || rw.Text(b[0]) != "this := &"+c.name+"{}" || rw.Text(b[1]) != "this.self_ = this" || rw.Text(b[3]) != "return this" {
		return false, ""
	}
	es, ok := b[2].(*ast.ExprStmt)
	if !ok {
		return false, ""
	}
	call, ok := es.X.(*ast.CallExpr)
	if !ok || rw.Text(call.Fun) != "this.init_"+c.name || !callsWithParams(c.newFn, call) {
		return false, ""
	}
	return true, ""
}

// getClassArgs: Object_getClass of a constant names its anonymous class in Java, the enum here.
func getClassArgs(byT map[*types.Named]*bodyEnum) {
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) != 1 || rw.Text(call.Fun) != "Object_getClass" {
				return true
			}
			t := rw.Info.TypeOf(call.Args[0])
			if p, ok := t.(*types.Pointer); ok {
				t = p.Elem()
			}
			if n, ok := t.(*types.Named); ok {
				if be := byT[n]; be != nil {
					be.why = "getClass of a constant"
				}
			}
			return true
		})
	}
}
