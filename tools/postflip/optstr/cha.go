package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"

	"postflip/rw"
)

// A class-hierarchy call graph of the package (generated code and runtime), for the fields whose
// constructor lets the object escape before it assigns the field: the code that can run before
// the assignment must not read the field on any object. Edges: a call of a function or of a
// concrete method goes to its declaration; a call of an interface method (or a type parameter's)
// to every method of that name; a call of a func value, a range over a func, or a func value
// handed to code outside the package, to every function used as a value with that signature;
// a function to the function literals it makes.

var (
	chaNodes   = map[ast.Node][]ast.Node{} // callees, computed once per node
	chaDone    = map[ast.Node]bool{}
	methodsBy  = map[string][]ast.Node{}
	valueFuncs []valueFunc
	readsField = map[ast.Node]map[*types.Var]bool{}
	chaBuilt   bool
)

type valueFunc struct {
	sig  *types.Signature
	node ast.Node
}

func buildCHA() {
	if chaBuilt {
		return
	}
	chaBuilt = true
	for fn, fd := range funcDecls {
		if fd.Recv != nil && fd.Body != nil {
			methodsBy[fn.Name()] = append(methodsBy[fn.Name()], fd)
		}
	}
	for _, f := range rw.Files {
		ast.Inspect(f, func(n ast.Node) bool {
			switch x := n.(type) {
			case *ast.FuncLit:
				if c, ok := rw.Parent[x].(*ast.CallExpr); ok && c.Fun == ast.Expr(x) {
					return true // called in place: an edge of its caller
				}
				if sig, ok := rw.Info.TypeOf(x).(*types.Signature); ok {
					valueFuncs = append(valueFuncs, valueFunc{sig, x})
				}
			case *ast.Ident:
				fn, ok := rw.Info.Uses[x].(*types.Func)
				if !ok {
					return true
				}
				var cur ast.Node = x
				if s, ok := rw.Parent[x].(*ast.SelectorExpr); ok && s.Sel == x {
					cur = s
				}
				if ix, ok := rw.Parent[cur].(*ast.IndexExpr); ok && ix.X == cur {
					cur = ix
				}
				if c, ok := rw.Parent[cur].(*ast.CallExpr); ok && c.Fun == cur {
					return true
				}
				fd := funcDecls[fn.Origin()]
				if fd == nil || fd.Body == nil {
					return true
				}
				sig := fn.Type().(*types.Signature)
				valueFuncs = append(valueFuncs, valueFunc{types.NewSignatureType(nil, nil, nil, sig.Params(), sig.Results(), sig.Variadic()), fd})
			}
			return true
		})
	}
}

func bodyOf(n ast.Node) *ast.BlockStmt {
	switch x := n.(type) {
	case *ast.FuncDecl:
		return x.Body
	case *ast.FuncLit:
		return x.Body
	}
	return nil
}

func sameSig(a, b *types.Signature) bool {
	return types.Identical(types.NewSignatureType(nil, nil, nil, a.Params(), a.Results(), a.Variadic()), types.NewSignatureType(nil, nil, nil, b.Params(), b.Results(), b.Variadic()))
}

func valueCallees(sig *types.Signature) []ast.Node {
	var out []ast.Node
	for _, v := range valueFuncs {
		if sameSig(v.sig, sig) {
			out = append(out, v.node)
		}
	}
	return out
}

// edges: the callees of the code in n (a statement, expression or function body), and the
// fields it reads.
func edges(n ast.Node, root ast.Node) ([]ast.Node, map[*types.Var]bool) {
	var out []ast.Node
	reads := map[*types.Var]bool{}
	ast.Inspect(n, func(c ast.Node) bool {
		switch x := c.(type) {
		case *ast.FuncLit:
			if x != root {
				if p, ok := rw.Parent[x].(*ast.CallExpr); !ok || p.Fun != ast.Expr(x) {
					out = append(out, x)
					return false
				}
			}
		case *ast.SelectorExpr:
			if s := rw.Info.Selections[x]; s != nil && s.Kind() == types.FieldVal {
				if a, ok := rw.Parent[x].(*ast.AssignStmt); !ok || !isLhs(a, x) {
					reads[s.Obj().(*types.Var)] = true
				}
			}
		case *ast.RangeStmt:
			if sig, ok := types.Unalias(rw.Info.TypeOf(x.X)).Underlying().(*types.Signature); ok {
				out = append(out, valueCallees(sig)...)
			}
		case *ast.CallExpr:
			if tv, ok := rw.Info.Types[x.Fun]; ok && tv.IsType() {
				return true
			}
			fun := rw.Unparen(x.Fun)
			if _, ok := fun.(*ast.FuncLit); ok {
				return true
			}
			var fn *types.Func
			var recvIface bool
			switch f := fun.(type) {
			case *ast.SelectorExpr:
				if s := rw.Info.Selections[f]; s != nil {
					if s.Kind() == types.FieldVal {
						break
					}
					fn, _ = s.Obj().(*types.Func)
					if fn != nil {
						// the method's own receiver: an interface's (also when promoted through
						// an embedded interface) or a type parameter's constraint
						if r := fn.Type().(*types.Signature).Recv(); r != nil {
							_, recvIface = r.Type().Underlying().(*types.Interface)
						}
						if _, ok := s.Recv().(*types.TypeParam); ok {
							recvIface = true
						}
					}
				} else {
					fn, _ = rw.Info.Uses[f.Sel].(*types.Func)
				}
			default:
				fn, _ = fnOf(fun)
			}
			if fn != nil {
				if recvIface {
					out = append(out, implementers(fn)...)
				} else if fd := funcDecls[fn.Origin()]; fd != nil {
					out = append(out, fd)
				}
				if fn.Pkg() != rw.Pkg {
					// code outside the package calls the funcs it is given, and the methods of
					// the values it is given (fmt's String / Error / Format, sort's Less / Swap /
					// Len, io's Read / Write / Close)
					for _, a := range x.Args {
						t := rw.Info.TypeOf(a)
						if sig, ok := types.Unalias(t).Underlying().(*types.Signature); ok {
							out = append(out, valueCallees(sig)...)
							continue
						}
						if p, ok := types.Unalias(t).(*types.Pointer); ok {
							t = p.Elem()
						}
						if n, ok := types.Unalias(t).(*types.Named); ok && n.Obj().Pkg() == rw.Pkg {
							if _, isI := n.Underlying().(*types.Interface); !isI {
								for i := 0; i < n.NumMethods(); i++ {
									if fd := funcDecls[n.Method(i)]; fd != nil {
										out = append(out, fd)
									}
								}
								continue
							}
						}
						if _, isI := types.Unalias(t).Underlying().(*types.Interface); isI {
							for _, m := range []string{"String", "Error", "Format", "GoString", "Less", "Swap", "Len", "Read", "Write", "Close", "Unwrap", "Is", "As"} {
								out = append(out, methodsBy[m]...)
							}
						}
					}
				}
			} else if _, isB := rw.Info.Uses[identOf(fun)].(*types.Builtin); !isB {
				if sig, ok := types.Unalias(rw.Info.TypeOf(fun)).Underlying().(*types.Signature); ok {
					out = append(out, valueCallees(sig)...)
				}
			}
		}
		return true
	})
	return out, reads
}

func identOf(e ast.Expr) *ast.Ident {
	id, _ := e.(*ast.Ident)
	return id
}

func isLhs(a *ast.AssignStmt, e ast.Expr) bool {
	for _, l := range a.Lhs {
		if l == e {
			return a.Tok == 0 || a.Tok.String() == "=" || a.Tok.String() == ":="
		}
	}
	return false
}

func nodeEdges(n ast.Node) []ast.Node {
	if !chaDone[n] {
		chaDone[n] = true
		out, reads := edges(bodyOf(n), n)
		chaNodes[n] = out
		readsField[n] = reads
	}
	return chaNodes[n]
}

// mayRead reports whether running code c (a statement or expression) can read field f.
func mayRead(c ast.Node, f *types.Var) bool {
	buildCHA()
	start, reads := edges(c, nil)
	if reads[f] {
		return true
	}
	seen := map[ast.Node]bool{}
	from := map[ast.Node]ast.Node{}
	work := append([]ast.Node(nil), start...)
	for len(work) > 0 {
		n := work[0]
		work = work[1:]
		if seen[n] || bodyOf(n) == nil {
			continue
		}
		seen[n] = true
		next := nodeEdges(n)
		if readsField[n][f] {
			if os.Getenv("OPTSTR_PATH") != "" {
				fmt.Fprintf(os.Stderr, "path to a read of %s:\n", f.Name())
				for p := n; p != nil; p = from[p] {
					fmt.Fprintf(os.Stderr, "    %s %s\n", rw.Fset.Position(p.Pos()), nodeName(p))
				}
			}
			return true
		}
		for _, x := range next {
			if !seen[x] {
				if _, ok := from[x]; !ok {
					from[x] = n
				}
			}
		}
		work = append(work, next...)
	}
	return false
}

func nodeName(n ast.Node) string {
	if fd, ok := n.(*ast.FuncDecl); ok {
		return fd.Name.Name
	}
	return "func literal"
}

var implCache = map[string][]ast.Node{}

// implementers: the methods named like fn of the package's types that implement the interface
// the call is made on (all methods of the name for a type parameter's constraint).
func implementers(fn *types.Func) []ast.Node {
	var recv types.Type
	if r := fn.Type().(*types.Signature).Recv(); r != nil {
		recv = r.Type()
	}
	if recv == nil {
		return methodsBy[fn.Name()]
	}
	iface, ok := recv.Underlying().(*types.Interface)
	if !ok {
		return methodsBy[fn.Name()]
	}
	key := types.TypeString(recv, nil) + "." + fn.Name()
	if r, ok := implCache[key]; ok {
		return r
	}
	var out []ast.Node
	for _, m := range methodsBy[fn.Name()] {
		fd := m.(*ast.FuncDecl)
		mf := rw.Info.Defs[fd.Name].(*types.Func)
		rt := mf.Type().(*types.Signature).Recv().Type()
		if p, ok := rt.(*types.Pointer); ok {
			rt = p.Elem()
		}
		named, ok := types.Unalias(rt).(*types.Named)
		if !ok {
			continue
		}
		if named.TypeParams().Len() > 0 {
			out = append(out, m) // a generic type: its instances are not enumerated
			continue
		}
		if types.Implements(named, iface) || types.Implements(types.NewPointer(named), iface) || embedsImpl(named, iface) {
			out = append(out, m)
		}
	}
	implCache[key] = out
	return out
}

// embedsImpl: a type that embeds named implements the interface (the method is promoted).
func embedsImpl(named *types.Named, iface *types.Interface) bool {
	for tn := range supers {
		if tn.Type() != named && below(tn, named.Obj()) {
			t := tn.Type()
			if types.Implements(t, iface) || types.Implements(types.NewPointer(t), iface) {
				return true
			}
		}
	}
	return false
}
