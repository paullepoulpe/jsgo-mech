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

// survey prints, for every enum, how its values are used.
func survey(enums []*enumInfo) {
	byPtr := map[types.Type]*enumInfo{}
	for _, e := range enums {
		byPtr[e.ptr] = e
	}
	ptrOf := func(x ast.Expr) *enumInfo {
		tv, ok := rw.Info.Types[x]
		if !ok || tv.Type == nil || !tv.IsValue() {
			return nil
		}
		for t, e := range byPtr {
			if types.Identical(tv.Type, t) {
				return e
			}
		}
		return nil
	}
	_ = ptrOf
	counts := map[string]int{}
	per := map[*enumInfo]map[string]int{}
	add := func(e *enumInfo, k string) {
		counts[k]++
		if per[e] == nil {
			per[e] = map[string]int{}
		}
		per[e][k]++
	}
	// index pointer types for speed
	ptrIdx := map[*types.Named]*enumInfo{}
	for _, e := range enums {
		ptrIdx[e.named] = e
	}
	quick := func(x ast.Expr) *enumInfo {
		tv, ok := rw.Info.Types[x]
		if !ok || tv.Type == nil || !tv.IsValue() {
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
		return ptrIdx[n]
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			x, ok := n.(ast.Expr)
			if !ok {
				return true
			}
			e := quick(x)
			if e == nil {
				return true
			}
			if id, ok := x.(*ast.Ident); ok && id.Name == "nil" {
				add(e, "nil")
				return true
			}
			switch p := rw.Up(x).(type) {
			case *ast.SelectorExpr:
				if s := rw.Info.Selections[p]; s != nil {
					k := "sel:" + p.Sel.Name
					if s.Kind() == types.FieldVal {
						if len(s.Index()) == 1 && p.Sel.Name != "Enum" {
							k = "ownfield"
						} else {
							k = "field:" + p.Sel.Name
						}
					} else if len(s.Index()) == 1 {
						k = "ownmethod"
					}
					add(e, k)
				}
			case *ast.StarExpr:
				add(e, "deref")
			case *ast.UnaryExpr:
				add(e, "unary"+p.Op.String())
			case *ast.BinaryExpr:
				o := p.X
				if rw.Unparen(o) == x {
					o = p.Y
				}
				if quick(o) != nil {
					add(e, "cmp")
				} else if tv := rw.Info.Types[o]; tv.Type != nil && types.IsInterface(tv.Type) {
					add(e, "cmp-iface")
				} else {
					add(e, "cmp-other")
				}
			default:
				if t := targetType(x); t != nil && types.IsInterface(t) {
					add(e, "toiface:"+shortType(t))
				}
			}
			return true
		})
	}
	// TypeName uses
	for _, e := range enums {
		for _, id := range rw.UsesOf[e.obj] {
			switch p := rw.Parent[id].(type) {
			case *ast.StarExpr:
				_ = p
			case *ast.CompositeLit:
				add(e, "T{}")
			case *ast.Field:
				add(e, "embedded")
			default:
				add(e, fmt.Sprintf("typeuse:%T", p))
			}
		}
	}
	var ks []string
	for k := range counts {
		ks = append(ks, k)
	}
	sort.Strings(ks)
	for _, k := range ks {
		fmt.Printf("%7d %s\n", counts[k], k)
	}
	for _, e := range enums {
		var parts []string
		for k, v := range per[e] {
			if strings.HasPrefix(k, "sel:") || k == "cmp" || k == "nil" || k == "ownmethod" {
				continue
			}
			parts = append(parts, fmt.Sprintf("%s=%d", k, v))
		}
		sort.Strings(parts)
		fmt.Fprintf(os.Stdout, "ENUM %s n=%d fields=%d sub=%v %s\n", e.name, len(e.consts), len(e.fields), e.subclassed, strings.Join(parts, " "))
	}
}

func shortType(t types.Type) string {
	return types.TypeString(t, func(*types.Package) string { return "" })
}

// targetType is the type the context of x converts it to, when x is an operand of an
// assignment, call argument, return, composite literal element or send.
func targetType(x ast.Expr) types.Type {
	n := ast.Node(x)
	for {
		if pe, ok := rw.Parent[n].(*ast.ParenExpr); ok {
			n = pe
			continue
		}
		break
	}
	p := rw.Parent[n]
	typeOf := func(e ast.Expr) types.Type {
		if tv, ok := rw.Info.Types[e]; ok {
			return tv.Type
		}
		return nil
	}
	switch p := p.(type) {
	case *ast.AssignStmt:
		if p.Tok == token.DEFINE {
			return nil
		}
		for i, r := range p.Rhs {
			if r == n && len(p.Lhs) == len(p.Rhs) {
				return typeOf(p.Lhs[i])
			}
		}
	case *ast.ValueSpec:
		if p.Type != nil {
			return typeOf(p.Type)
		}
	case *ast.CallExpr:
		ft, ok := typeOf(p.Fun).(*types.Signature)
		if !ok {
			if tv := rw.Info.Types[p.Fun]; tv.IsType() {
				return tv.Type
			}
			return nil
		}
		for i, a := range p.Args {
			if a == n {
				ps := ft.Params()
				if ft.Variadic() && i >= ps.Len()-1 {
					if p.Ellipsis.IsValid() {
						return ps.At(ps.Len() - 1).Type()
					}
					return ps.At(ps.Len() - 1).Type().(*types.Slice).Elem()
				}
				if i < ps.Len() {
					return ps.At(i).Type()
				}
			}
		}
	case *ast.ReturnStmt:
		// find enclosing func
		var q ast.Node = p
		for q != nil {
			switch f := q.(type) {
			case *ast.FuncDecl:
				return resultAt(rw.Info.Defs[f.Name].Type().(*types.Signature), p, n)
			case *ast.FuncLit:
				return resultAt(typeOf(f).(*types.Signature), p, n)
			}
			q = rw.Parent[q]
		}
	case *ast.CompositeLit:
		switch u := typeOf(p).Underlying().(type) {
		case *types.Slice:
			return u.Elem()
		case *types.Array:
			return u.Elem()
		case *types.Struct:
			for i, el := range p.Elts {
				if el == n {
					return u.Field(i).Type()
				}
			}
		}
	case *ast.KeyValueExpr:
		cl, ok := rw.Parent[p].(*ast.CompositeLit)
		if !ok {
			return nil
		}
		switch u := typeOf(cl).Underlying().(type) {
		case *types.Slice:
			return u.Elem()
		case *types.Map:
			if p.Key == n {
				return u.Key()
			}
			return u.Elem()
		case *types.Struct:
			if id, ok := p.Key.(*ast.Ident); ok {
				for i := 0; i < u.NumFields(); i++ {
					if u.Field(i).Name() == id.Name {
						return u.Field(i).Type()
					}
				}
			}
		}
	case *ast.SendStmt:
		if p.Value == n {
			return typeOf(p.Chan).Underlying().(*types.Chan).Elem()
		}
	case *ast.IndexExpr:
		if p.Index == n {
			if m, ok := typeOf(p.X).Underlying().(*types.Map); ok {
				return m.Key()
			}
		}
	}
	return nil
}

func resultAt(sig *types.Signature, r *ast.ReturnStmt, n ast.Node) types.Type {
	for i, e := range r.Results {
		if e == n && sig.Results().Len() == len(r.Results) {
			return sig.Results().At(i).Type()
		}
	}
	return nil
}
