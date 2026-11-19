package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"strings"

	"postflip/rw"
)

type stats struct {
	consts, tables, stars, nils, fieldSites, enumSites, inits, ctors, tokenSites int
}

func (s stats) String() string {
	return fmt.Sprintf("types *T -> T: %d; nil -> 0: %d; field reads/writes -> tables: %d; Enum field/method sites: %d; constructor calls -> init or nothing: %d (init methods kept: %d); Token byte sites: %d",
		s.stars, s.nils, s.fieldSites, s.enumSites, s.ctors, s.inits, s.tokenSites)
}

func tableName(e *enumInfo, f string) string {
	n := e.name + "_" + f + "_"
	if rw.Pkg.Scope().Lookup(n) != nil {
		n = e.name + "_" + f + "_tab_"
	}
	if rw.Pkg.Scope().Lookup(n) != nil {
		panic("table name taken: " + n)
	}
	return n
}

func rewrite(conv []*enumInfo) stats {
	var st stats
	for _, e := range conv {
		e := e
		n := len(e.consts)
		st.consts += n
		// constants
		for k, c := range e.consts {
			k, c := k, c
			rw.Rewrite[e.constDecls[c]] = func() string { return fmt.Sprintf("const %s %s = %d", c, e.name, k+1) }
		}
		// the type and its tables and methods
		tables := map[string]string{}
		for _, f := range e.fields {
			tables[f.Name()] = tableName(e, f.Name())
		}
		st.tables += len(e.fields)
		stt := e.decl.Specs[0].(*ast.TypeSpec).Type.(*ast.StructType)
		rw.Rewrite[e.decl] = func() string {
			var b strings.Builder
			fmt.Fprintf(&b, "type %s uint8\n\n", e.name)
			fmt.Fprintf(&b, "// %s_enum_ is the java.lang.Enum record of each constant, by ordinal; a constant is ordinal + 1 and 0 is null.\n", e.name)
			fmt.Fprintf(&b, "var %s_enum_ = [%d]Enum{\n", e.name, n)
			for k, c := range e.consts {
				fmt.Fprintf(&b, "\t{self_: %s, name_: %q, ordinal_: %d},\n", c, e.cnames[k], k)
			}
			b.WriteString("}\n")
			for _, fl := range stt.Fields.List {
				if len(fl.Names) == 0 {
					continue
				}
				for _, nm := range fl.Names {
					fmt.Fprintf(&b, "\nvar %s [%d]%s\n", tables[nm.Name], n, rw.Render(fl.Type))
				}
			}
			T := e.name
			fmt.Fprintf(&b, "\nfunc (this %s) enum_() *Enum {\n\tif this == 0 {\n\t\treturn nil\n\t}\n\treturn &%s_enum_[this-1]\n}\n", T, T)
			fmt.Fprintf(&b, "\nfunc (this %s) name() string { return this.enum_().name_ }\n", T)
			fmt.Fprintf(&b, "\nfunc (this %s) ordinal() int32 {\n\tif this == 0 {\n\t\treturn this.enum_().ordinal_ // null: dereferences nil, as a null enum object did\n\t}\n\treturn int32(this) - 1 // constants are ordinal+1\n}\n", T)
			if !e.ownMethods["toString"] {
				fmt.Fprintf(&b, "\nfunc (this %s) toString() string { return this.enum_().name_ }\n", T)
			}
			fmt.Fprintf(&b, "\nfunc (this %s) compareTo(o any) int32 { return this.enum_().compareTo(o) }\n", T)
			fmt.Fprintf(&b, "\nfunc (this %s) self() any {\n\tif this == 0 {\n\t\treturn nil\n\t}\n\treturn this\n}", T)
			return b.String()
		}
		for _, fd := range e.news {
			rw.Rewrite[fd] = func() string { return rw.Del }
		}
		if e.upEnum != nil {
			rw.Rewrite[e.upEnum] = func() string { return rw.Del }
		}
		// init methods: dead when they only call init_Enum and dead init methods
		isEnumInit := func(s ast.Stmt) bool {
			es, ok := s.(*ast.ExprStmt)
			return ok && strings.HasPrefix(rw.Text(es), "this.Enum.init_Enum(")
		}
		callee := func(s ast.Stmt) string {
			es, ok := s.(*ast.ExprStmt)
			if !ok {
				return ""
			}
			c, ok := es.X.(*ast.CallExpr)
			if !ok {
				return ""
			}
			sel, ok := c.Fun.(*ast.SelectorExpr)
			if !ok || rw.Text(sel.X) != "this" || e.inits[sel.Sel.Name] == nil {
				return ""
			}
			return sel.Sel.Name
		}
		nparams := func(fd *ast.FuncDecl) int {
			k := 0
			for _, f := range fd.Type.Params.List {
				for _, nm := range f.Names {
					if nm.Name != "name_" && nm.Name != "ordinal_" {
						k++
					}
				}
			}
			return k
		}
		for changed := true; changed; {
			changed = false
			for name, fd := range e.inits {
				if e.deadInits[name] || (e.keepNameOrd || nparams(fd) > 0) {
					continue
				}
				dead := true
				for _, s := range fd.Body.List {
					if !isEnumInit(s) && !e.deadInits[callee(s)] {
						dead = false
					}
				}
				if dead {
					e.deadInits[name] = true
					changed = true
				}
			}
		}
		for name, fd := range e.inits {
			fd := fd
			if e.deadInits[name] {
				rw.Rewrite[fd] = func() string { return rw.Del }
				continue
			}
			st.inits++
			for _, s := range fd.Body.List {
				if isEnumInit(s) || e.deadInits[callee(s)] {
					rw.Rewrite[s] = func() string { return rw.Del }
				} else if c := callee(s); c != "" && !e.keepNameOrd {
					call := s.(*ast.ExprStmt).X.(*ast.CallExpr)
					rw.Rewrite[call] = func() string { return rw.Render(call.Fun) + "(" + renderArgs(call.Args[2:]) + ")" }
				}
			}
			if !e.keepNameOrd {
				pl := fd.Type.Params
				rw.Rewrite[pl] = func() string {
					var ps []string
					for _, f := range pl.List {
						var names []string
						for _, nm := range f.Names {
							if nm.Name != "name_" && nm.Name != "ordinal_" {
								names = append(names, nm.Name)
							}
						}
						if len(names) > 0 {
							ps = append(ps, strings.Join(names, ", ")+" "+rw.Render(f.Type))
						}
					}
					return "(" + strings.Join(ps, ", ") + ")"
				}
			}
		}
		// the static initializer
		for _, s := range e.clinit.Body.List {
			c, call := newAssign(e, s)
			if call == nil {
				continue
			}
			st.ctors++
			init, zero := standardNew(e, e.news[call.Fun.(*ast.Ident).Name])
			pre := ""
			if zero != "" {
				pre = c + "." + zero + "()"
			}
			if e.deadInits[init] {
				rw.Rewrite[s] = func() string {
					if pre != "" {
						return pre
					}
					return rw.Del
				}
				continue
			}
			if pre != "" {
				pre += "\n\t"
			}
			args := call.Args
			if !e.keepNameOrd {
				args = args[2:]
			}
			rw.Rewrite[s] = func() string { return pre + c + "." + init + "(" + renderArgs(args) + ")" }
		}
	}

	// uses
	for _, f := range rw.Files {
		if !isGen(f.Pos()) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.StarExpr:
				if id, ok := x.X.(*ast.Ident); ok {
					if tn, ok := rw.Info.Uses[id].(*types.TypeName); ok {
						if nt, ok := tn.Type().(*types.Named); ok && byNamed[nt] != nil {
							rw.Rewrite[x] = func() string { return id.Name }
							st.stars++
						}
					}
				}
			case *ast.BinaryExpr:
				if x.Op == token.EQL || x.Op == token.NEQ {
					a, b := x.X, x.Y
					if isNilIdent(a) {
						a, b = b, a
					}
					if isNilIdent(b) && ptrEnum(a) != nil {
						rw.Rewrite[rw.Unparen(b)] = func() string { return "0" }
						st.nils++
					}
				}
			case *ast.Ident:
				if isNilIdent(x) && (ptrEnum(x) != nil || isEnumPtr(targetType(x)) || isEnumCase(x)) {
					if _, done := rw.Rewrite[x]; !done {
						rw.Rewrite[x] = func() string { return "0" }
						st.nils++
					}
				}
			case *ast.SelectorExpr:
				e := ptrEnum(x.X)
				if e == nil {
					return true
				}
				s := rw.Info.Selections[x]
				if s == nil {
					return true
				}
				recv := x.X
				if s.Kind() == types.MethodVal {
					if x.Sel.Name == "up_Enum" {
						rw.Rewrite[x] = func() string { return rw.Render(recv) + ".enum_" }
						st.enumSites++
					}
					return true
				}
				switch x.Sel.Name {
				case "Enum":
					q, ok := rw.Up(x).(*ast.SelectorExpr)
					if !ok {
						return true
					}
					switch q.Sel.Name {
					case "name", "ordinal", "toString":
						rw.Rewrite[x] = func() string { return rw.Render(recv) }
						st.enumSites++
					case "name_", "ordinal_":
						m := strings.TrimSuffix(q.Sel.Name, "_")
						rw.Rewrite[q] = func() string { return rw.Render(recv) + "." + m + "()" }
						st.enumSites++
					}
				case "name_", "ordinal_":
					m := strings.TrimSuffix(x.Sel.Name, "_")
					rw.Rewrite[x] = func() string { return rw.Render(recv) + "." + m + "()" }
					st.enumSites++
				case "values_":
					rw.Rewrite[x] = func() string { return rw.Render(recv) + ".enum_().values_" }
					st.enumSites++
				case "self_":
				default:
					if len(s.Index()) == 1 {
						tab := tableName(e, x.Sel.Name)
						rw.Rewrite[x] = func() string { return tab + "[" + rw.Render(recv) + "-1]" }
						st.fieldSites++
					}
				}
			}
			return true
		})
	}
	tokenBytes(&st)
	enumCasts()
	return st
}

// enumCasts: casts to java.lang.Enum assert the runtime's enumConst_ (enum_(), which an integer
// enum has as a value method and an object enum inherits from Enum) instead of as_Enum.
func enumCasts() {
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || !isGen(fd.Pos()) || strings.Contains(rw.Text(fd), "enumConst_") {
				continue
			}
			switch fd.Name.Name {
			case "to_Enum":
				rw.Rewrite[fd] = func() string {
					return "func to_Enum(v any) *Enum {\n\tif v == nil {\n\t\treturn nil\n\t}\n\treturn v.(enumConst_).enum_()\n}"
				}
			case "is_Enum":
				rw.Rewrite[fd] = func() string {
					return "func is_Enum(v any) bool {\n\t_, ok := v.(enumConst_)\n\treturn ok\n}"
				}
			}
		}
	}
}

func renderArgs(as []ast.Expr) string {
	var out []string
	for _, a := range as {
		out = append(out, rw.Render(a))
	}
	return strings.Join(out, ", ")
}

// tokenBytes: a Node's token byte is the Token value (ordinal + 1).
func tokenBytes(st *stats) {
	tok, _ := rw.Pkg.Scope().Lookup("Token_rhino").(*types.TypeName)
	if tok == nil || byNamed[tok.Type().(*types.Named)] == nil {
		return
	}
	ord1, _ := rw.Pkg.Scope().Lookup("Token_rhino_ord1_").(*types.Func)
	byOrd1, _ := rw.Pkg.Scope().Lookup("Token_rhino_byOrd1_").(*types.Var)
	if ord1 == nil || byOrd1 == nil {
		return
	}
	for _, id := range rw.UsesOf[ord1] {
		call, ok := rw.Parent[id].(*ast.CallExpr)
		if !ok || call.Fun != id || len(call.Args) != 1 {
			panic("Token_rhino_ord1_ used other than called")
		}
		a := call.Args[0]
		rw.Rewrite[call] = func() string { return "uint8(" + rw.Render(a) + ")" }
		st.tokenSites++
	}
	for _, id := range rw.UsesOf[byOrd1] {
		switch p := rw.Parent[id].(type) {
		case *ast.IndexExpr:
			if p.X != id {
				panic("Token_rhino_byOrd1_ used other than indexed")
			}
			if as, ok := rw.Parent[p].(*ast.AssignStmt); ok && as.Lhs[0] == p {
				// the loop of the static initializer that fills the table
				loop := rw.Parent[rw.Parent[as]].(*ast.RangeStmt)
				rw.Rewrite[loop] = func() string { return rw.Del }
				continue
			}
			ix := p.Index
			rw.Rewrite[p] = func() string { return "Token_rhino(" + rw.Render(ix) + ")" }
			st.tokenSites++
		default:
			panic(fmt.Sprintf("Token_rhino_byOrd1_ used as %T", p))
		}
	}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch d := d.(type) {
			case *ast.FuncDecl:
				if d.Recv == nil && d.Name.Name == "Token_rhino_ord1_" {
					rw.Rewrite[d] = func() string { return rw.Del }
				}
			case *ast.GenDecl:
				if d.Tok == token.VAR && len(d.Specs) == 1 && d.Specs[0].(*ast.ValueSpec).Names[0].Name == "Token_rhino_byOrd1_" {
					rw.Rewrite[d] = func() string { return rw.Del }
				}
			}
		}
	}
}

func isEnumPtr(t types.Type) bool {
	if t == nil {
		return false
	}
	p, ok := t.(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := p.Elem().(*types.Named)
	return ok && byNamed[n] != nil
}

// isEnumCase reports whether nil is a case of a switch on an enum value.
func isEnumCase(x ast.Expr) bool {
	cc, ok := rw.Parent[x].(*ast.CaseClause)
	if !ok {
		return false
	}
	sw, ok := rw.Parent[rw.Parent[cc]].(*ast.SwitchStmt)
	return ok && sw.Tag != nil && ptrEnum(sw.Tag) != nil
}
