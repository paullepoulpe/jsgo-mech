package main

import (
	"go/ast"
	"go/types"
	"sort"

	"postflip/rw"
)

// The instantiations of the package's generic functions and types, for the call graph's sites
// in generic code (cha.go: valueCallees).
//
// Every instantiation that exists at run time is made by code that names the generic function
// or type with type arguments (written or inferred: go/types records both in Info.Instances),
// in a context that is not generic, or in a generic function, method or type declaration whose
// own type parameters are bound, at run time, to one of its instantiations. So the set
// Inst(G) of ground type-argument tuples of G is the least fixed point of: for each recorded
// instance G[A] in context H, A itself if H is not generic, else A with H's type parameters
// replaced by each tuple of Inst(H) (a method's context is its receiver type's). Polymorphic
// recursion could make the set infinite: a G whose set passes maxInst is marked open, and so
// is everything instantiated from it; a site in an open context matches with its type
// parameters standing for any type.

const maxInst = 4000

type instSite struct {
	obj   types.Object
	targs []types.Type
	ctx   types.Object // nil: not in generic code
	ctxTP []*types.TypeParam
}

var (
	instBuilt bool
	instOf    = map[types.Object][][]types.Type{}
	instKeys  = map[types.Object]map[string]bool{}
	instOpen  = map[types.Object]bool{}
)

// genericContext: the generic declaration node n is in (a function with type parameters, a
// method of a generic type, a generic type declaration), with its type parameters.
func genericContext(n ast.Node) (types.Object, []*types.TypeParam) {
	for ; n != nil; n = rw.Parent[n] {
		switch x := n.(type) {
		case *ast.FuncDecl:
			fn, _ := rw.Info.Defs[x.Name].(*types.Func)
			if fn == nil {
				return nil, nil
			}
			sig := fn.Type().(*types.Signature)
			if tp := sig.TypeParams(); tp.Len() > 0 {
				return fn, tpList(tp)
			}
			if tp := sig.RecvTypeParams(); tp.Len() > 0 {
				rt := sig.Recv().Type()
				if p, ok := types.Unalias(rt).(*types.Pointer); ok {
					rt = p.Elem()
				}
				if named, ok := types.Unalias(rt).(*types.Named); ok {
					return named.Origin().Obj(), tpList(tp)
				}
			}
			return nil, nil
		case *ast.TypeSpec:
			if tn, ok := rw.Info.Defs[x.Name].(*types.TypeName); ok {
				if named, ok := tn.Type().(*types.Named); ok && named.TypeParams().Len() > 0 {
					return tn, tpList(named.TypeParams())
				}
				if al, ok := tn.Type().(*types.Alias); ok && al.TypeParams().Len() > 0 {
					return tn, tpList(al.TypeParams())
				}
			}
		}
	}
	return nil, nil
}

func tpList(l *types.TypeParamList) []*types.TypeParam {
	out := make([]*types.TypeParam, l.Len())
	for i := range out {
		out[i] = l.At(i)
	}
	return out
}

func buildInstances() {
	if instBuilt {
		return
	}
	instBuilt = true
	var sites []instSite
	ids := make([]*ast.Ident, 0, len(rw.Info.Instances))
	for id := range rw.Info.Instances {
		ids = append(ids, id)
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i].Pos() < ids[j].Pos() })
	for _, id := range ids {
		inst := rw.Info.Instances[id]
		ctx, ctxTP := genericContext(id)
		list := func(l *types.TypeList) []types.Type {
			var out []types.Type
			for i := 0; i < l.Len(); i++ {
				out = append(out, l.At(i))
			}
			return out
		}
		switch o := rw.Info.Uses[id].(type) {
		case *types.Func:
			sites = append(sites, instSite{obj: o.Origin(), targs: list(inst.TypeArgs), ctx: ctx, ctxTP: ctxTP})
		case *types.TypeName:
			// the generic type or alias named, and the generic type the instance is (an alias
			// of a generic type instantiates that type: ImmutableMap[K, V] is a TMap[K, V])
			sites = append(sites, instSite{obj: o, targs: list(inst.TypeArgs), ctx: ctx, ctxTP: ctxTP})
			if named, ok := types.Unalias(inst.Type).(*types.Named); ok && named.TypeArgs().Len() > 0 && named.Origin().Obj() != o {
				sites = append(sites, instSite{obj: named.Origin().Obj(), targs: list(named.TypeArgs()), ctx: ctx, ctxTP: ctxTP})
			}
		}
	}
	add := func(o types.Object, tu []types.Type) bool {
		if instOpen[o] {
			return false
		}
		k := ""
		for _, t := range tu {
			k += types.TypeString(t, nil) + ";"
		}
		if instKeys[o] == nil {
			instKeys[o] = map[string]bool{}
		}
		if instKeys[o][k] {
			return false
		}
		if len(instOf[o]) >= maxInst {
			instOpen[o] = true
			return true
		}
		instKeys[o][k] = true
		instOf[o] = append(instOf[o], tu)
		return true
	}
	for changed := true; changed; {
		changed = false
		for _, s := range sites {
			ground := true
			for _, t := range s.targs {
				ground = ground && !mentionsTP(t)
			}
			if ground {
				changed = add(s.obj, s.targs) || changed
				continue
			}
			if s.ctx == nil || instOpen[s.ctx] {
				// type parameters that no known context binds: open
				if !instOpen[s.obj] {
					instOpen[s.obj] = true
					changed = true
				}
				continue
			}
			for _, tu := range instOf[s.ctx] {
				m := map[*types.TypeParam]types.Type{}
				for i, tp := range s.ctxTP {
					if i < len(tu) {
						m[tp] = tu[i]
					}
				}
				out := make([]types.Type, len(s.targs))
				ok := true
				for i, t := range s.targs {
					out[i] = subst(t, m)
					ok = ok && !mentionsTP(out[i])
				}
				if !ok {
					if !instOpen[s.obj] {
						instOpen[s.obj] = true
						changed = true
					}
					continue
				}
				changed = add(s.obj, out) || changed
			}
		}
	}
}

// contextOf: the type parameters of the generic declaration around site and the tuples it is
// instantiated with; ok is false when the site is in no generic code or the set is open.
func contextOf(site ast.Node) ([]*types.TypeParam, [][]types.Type, bool) {
	buildInstances()
	ctx, tps := genericContext(site)
	if ctx == nil || instOpen[ctx] {
		return nil, nil, false
	}
	return tps, instOf[ctx], true
}

// subst is t with the type parameters in m replaced.
func subst(t types.Type, m map[*types.TypeParam]types.Type) types.Type {
	if !mentionsTP(t) {
		return t
	}
	switch u := types.Unalias(t).(type) {
	case *types.TypeParam:
		if r, ok := m[u]; ok {
			return r
		}
		return u
	case *types.Pointer:
		return types.NewPointer(subst(u.Elem(), m))
	case *types.Slice:
		return types.NewSlice(subst(u.Elem(), m))
	case *types.Array:
		return types.NewArray(subst(u.Elem(), m), u.Len())
	case *types.Map:
		return types.NewMap(subst(u.Key(), m), subst(u.Elem(), m))
	case *types.Chan:
		return types.NewChan(u.Dir(), subst(u.Elem(), m))
	case *types.Signature:
		tup := func(l *types.Tuple) *types.Tuple {
			vs := make([]*types.Var, l.Len())
			for i := range vs {
				vs[i] = types.NewVar(l.At(i).Pos(), l.At(i).Pkg(), l.At(i).Name(), subst(l.At(i).Type(), m))
			}
			return types.NewTuple(vs...)
		}
		return types.NewSignatureType(nil, nil, nil, tup(u.Params()), tup(u.Results()), u.Variadic())
	case *types.Named:
		args := make([]types.Type, u.TypeArgs().Len())
		for i := range args {
			args[i] = subst(u.TypeArgs().At(i), m)
		}
		if r, err := types.Instantiate(nil, u.Origin(), args, false); err == nil {
			return r
		}
	}
	return t // a literal struct or interface type: left with its type parameters (they match any type)
}
