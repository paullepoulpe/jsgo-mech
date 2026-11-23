// Command lgcount (JAVAISMS6, read-only): what is left of the erased Java library generics in g_*.go.
//
//	lgcount <dir>            summary
//	lgcount <dir> -sites     plus one line per boundary call (file:line kind callee consumer)
//
// 1. declared slots (struct fields, parameters, results, locals) by the category of their type;
// 2. boundary conversions between typed and erased library values (X_un_, xAs_, lgBox_), by the
//    erased function or method that the value goes to or comes from.
package main

import (
	"fmt"
	"go/ast"
	"go/types"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strings"

	"postflip/rw"
)

var typedLib = map[string]bool{"IteratorT": true, "IterableT": true, "CollectionT": true, "ListT": true, "SetT": true, "MapT": true, "Map_EntryT": true}
var erasedColl = map[string]bool{"Iterator": true, "Iterable": true, "Collection": true, "List": true, "Set": true, "Map": true, "Map_Entry": true}
var fnTypes = map[string]bool{"Function": true, "Supplier": true, "Consumer": true, "Predicate": true, "Comparator": true, "BiFunction": true, "BiConsumer": true, "BiPredicate": true, "ToIntFunction": true, "Callable": true, "UnaryOperator": true, "BinaryOperator": true, "Function_base": true, "Predicate_base": true, "Supplier_base": true}

func declFile(o types.Object) string {
	if o == nil || !o.Pos().IsValid() {
		return ""
	}
	return filepath.Base(rw.Fset.Position(o.Pos()).Filename)
}

func isAny(t types.Type) bool {
	if a, ok := t.(*types.Alias); ok {
		if a.Obj().Name() == "any" {
			return true
		}
	}
	i, ok := types.Unalias(t).(*types.Interface)
	return ok && i.Empty()
}

func cat(t types.Type) string {
	if a, ok := t.(*types.Alias); ok && strings.HasSuffix(a.Obj().Name(), "Of") {
		return "erased alias " + a.Obj().Name() + "[...]"
	}
	if isAny(t) {
		return "any"
	}
	u := types.Unalias(t)
	if p, ok := u.(*types.Pointer); ok {
		u = types.Unalias(p.Elem())
	}
	n, ok := u.(*types.Named)
	if !ok {
		return "other"
	}
	name := n.Obj().Name()
	f := declFile(n.Obj())
	lib := strings.HasPrefix(f, "rt_") || f == "g_zstubs.go"
	anyArg := false
	if ta := n.TypeArgs(); ta != nil {
		for i := 0; i < ta.Len(); i++ {
			if isAny(ta.At(i)) {
				anyArg = true
			}
		}
	}
	switch {
	case typedLib[name] && anyArg:
		return "typed library interface, an `any` type argument"
	case typedLib[name]:
		return "typed library interface"
	case erasedColl[name]:
		return "erased collection interface (raw)"
	case fnTypes[name] && anyArg:
		return "library func type, an `any` type argument"
	case fnTypes[name]:
		return "library func type"
	case lib && n.TypeArgs() != nil && anyArg:
		return "other runtime generic, an `any` type argument"
	case lib && n.TypeArgs() != nil:
		return "other runtime generic"
	case lib:
		if _, ok := n.Underlying().(*types.Interface); ok {
			return "other library interface (erased)"
		}
		return "other library type"
	}
	return "program type"
}

func main() {
	dir := os.Args[1]
	sites := len(os.Args) > 2 && os.Args[2] == "-sites"
	rw.Load(dir)
	slots := map[string]int{}
	slotKind := map[string]map[string]int{}
	add := func(kind string, t types.Type, n int) {
		c := cat(t)
		slots[c] += n
		if slotKind[c] == nil {
			slotKind[c] = map[string]int{}
		}
		slotKind[c][kind] += n
	}
	inField := map[*ast.Ident]bool{}
	var conv []string
	convRe := regexp.MustCompile(`^(\w+_un_|[a-z]\w*As_|lgBox_)$`)
	for _, f := range rw.Files {
		fn := filepath.Base(rw.Fset.Position(f.Pos()).Filename)
		if !strings.HasPrefix(fn, "g_") {
			continue
		}
		ast.Inspect(f, func(x ast.Node) bool {
			switch x := x.(type) {
			case *ast.FuncType:
				for _, fl := range []*ast.FieldList{x.Params, x.Results} {
					if fl == nil {
						continue
					}
					for _, fd := range fl.List {
						k := "parameter"
						if fl == x.Results {
							k = "result"
						}
						n := len(fd.Names)
						for _, id := range fd.Names {
							inField[id] = true
						}
						if n == 0 {
							n = 1
						}
						add(k, rw.Info.TypeOf(fd.Type), n)
					}
				}
			case *ast.StructType:
				for _, fd := range x.Fields.List {
					n := len(fd.Names)
					for _, id := range fd.Names {
						inField[id] = true
					}
					if n == 0 {
						continue // embedded
					}
					add("field", rw.Info.TypeOf(fd.Type), n)
				}
			case *ast.CallExpr:
				id, ok := ast.Unparen(x.Fun).(*ast.Ident)
				if !ok {
					if ix, ok2 := ast.Unparen(x.Fun).(*ast.IndexExpr); ok2 {
						id, ok = ix.X.(*ast.Ident)
					}
				}
				if !ok || !convRe.MatchString(id.Name) {
					return true
				}
				// the consumer: the nearest enclosing call that is not a conversion, a Go
				// interface conversion or an icast_
				var up ast.Node = x
				cons := "?"
				for {
					up = rw.Parent[up]
					if up == nil {
						break
					}
					if c, ok := up.(*ast.CallExpr); ok {
						name, _ := rw.CallName(c)
						if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() {
							continue
						}
						if convRe.MatchString(name) || strings.HasPrefix(name, "icast_") || name == "" {
							continue
						}
						cons = name
						break
					}
					if _, ok := up.(*ast.ReturnStmt); ok {
						cons = "(returned)"
						break
					}
					if _, ok := up.(*ast.AssignStmt); ok {
						cons = "(assigned)"
						break
					}
					if _, ok := up.(*ast.ValueSpec); ok {
						cons = "(declared)"
						break
					}
					if _, ok := up.(*ast.RangeStmt); ok {
						cons = "(range)"
						break
					}
					if _, ok := up.(*ast.BlockStmt); ok {
						break
					}
				}
				// the argument when it is itself a call (for xAs_: the erased producer)
				prod := ""
				if len(x.Args) > 0 {
					a := x.Args[len(x.Args)-1]
					for {
						c, ok := ast.Unparen(a).(*ast.CallExpr)
						if !ok {
							break
						}
						name, _ := rw.CallName(c)
						if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() || strings.HasPrefix(name, "icast_") {
							if len(c.Args) == 1 {
								a = c.Args[0]
								continue
							}
						}
						prod = name
						break
					}
					if prod == "" {
						switch ast.Unparen(a).(type) {
						case *ast.Ident:
							prod = "(variable)"
						case *ast.SelectorExpr:
							prod = "(field)"
						default:
							prod = "(other)"
						}
					}
				}
				p := rw.Fset.Position(x.Pos())
				kind := id.Name
				if strings.HasSuffix(kind, "_un_") {
					kind = "X_un_"
				}
				conv = append(conv, fmt.Sprintf("%s:%d\t%s\t%s\t%s\t%s", fn, p.Line, kind, id.Name, cons, prod))
			}
			return true
		})
	}
	for id, o := range rw.Info.Defs {
		v, ok := o.(*types.Var)
		if !ok || inField[id] || v.IsField() {
			continue
		}
		fn := filepath.Base(rw.Fset.Position(id.Pos()).Filename)
		if !strings.HasPrefix(fn, "g_") || id.Name == "_" {
			continue
		}
		if v.Parent() == rw.Pkg.Scope() {
			add("package var", v.Type(), 1)
		} else {
			add("local", v.Type(), 1)
		}
	}
	var ks []string
	for k := range slots {
		ks = append(ks, k)
	}
	sort.Slice(ks, func(i, j int) bool { return slots[ks[i]] > slots[ks[j]] })
	fmt.Println("== declared slots in g_*.go by type category")
	tot := 0
	for _, k := range ks {
		tot += slots[k]
		var parts []string
		for kk, n := range slotKind[k] {
			parts = append(parts, fmt.Sprintf("%s %d", kk, n))
		}
		sort.Strings(parts)
		fmt.Printf("%7d  %s  (%s)\n", slots[k], k, strings.Join(parts, ", "))
	}
	fmt.Printf("%7d  total\n", tot)
	sort.Strings(conv)
	fmt.Println("== boundary conversions in g_*.go:", len(conv))
	byKind := map[string]int{}
	byCons := map[string]int{}
	for _, c := range conv {
		f := strings.Split(c, "\t")
		byKind[f[1]]++
		byCons[f[1]+" -> "+f[3]]++
	}
	for k, n := range byKind {
		fmt.Printf("%7d  %s\n", n, k)
	}
	var cs []string
	for k := range byCons {
		cs = append(cs, k)
	}
	sort.Slice(cs, func(i, j int) bool { return byCons[cs[i]] > byCons[cs[j]] || byCons[cs[i]] == byCons[cs[j]] && cs[i] < cs[j] })
	fmt.Println("== by consumer")
	for _, k := range cs {
		fmt.Printf("%7d  %s\n", byCons[k], k)
	}
	if sites {
		fmt.Println("== sites")
		for _, c := range conv {
			fmt.Println(c)
		}
	}
}
