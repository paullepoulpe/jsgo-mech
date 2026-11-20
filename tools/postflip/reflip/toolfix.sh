#!/usr/bin/env bash
# Trial re-flip: fixes applied to tools/postflip as of every step's commit, before the step's
# tools run (replay-all.sh -toolfix). Idempotent. Each fix is a rule the tools of the first flip
# could do without because the frozen code then had no Go generic types of its own.
set -euo pipefail
rwgo=tools/postflip/rw/rw.go
# 1. Uses of a field or method of a generic type inside its methods (the receiver's own type
#    parameters) and at instantiations are other types.Object values than the declaration's.
#    The tools find the uses of a declaration by object identity (rw.UsesOf, rw.Info.Uses), so
#    they saw none of them and rewrote the declaration alone. Uses now name the origin object.
if [[ -f $rwgo ]] && ! grep -q 'reflip: origin objects' $rwgo; then
  perl -0pi -e 's/(\tfor id, o := range Info\.Uses \{\n\t\tUsesOf\[o\] = append)/\t\/\/ reflip: origin objects (a use of a generic type\x27s field or method names its declaration)\n\tfor id, o := range Info.Uses {\n\t\tswitch v := o.(type) {\n\t\tcase *types.Var:\n\t\t\tif v.Origin() != v {\n\t\t\t\tInfo.Uses[id] = v.Origin()\n\t\t\t}\n\t\tcase *types.Func:\n\t\t\tif v.Origin() != v {\n\t\t\t\tInfo.Uses[id] = v.Origin()\n\t\t\t}\n\t\t}\n\t}\n$1/' $rwgo
  grep -q 'reflip: origin objects' $rwgo || { echo "toolfix: rw.go: the Uses loop was not found" >&2; exit 1; }
fi
# 2. rw.ConvFor gives the boxing of an element type by its name (cast_[*Name], icast_Name,
#    obj_). A type that mentions a type parameter, or (for the rw versions before funcObj_) an
#    instance of a generic type, has no such spelling: no known boxing, the site stays.
if [[ -f $rwgo ]] && ! grep -q 'ReflipGeneric_' $rwgo; then
  inst=true; grep -q 'TypeArgs().Len() > 0' $rwgo && inst=false
  perl -0pi -e 's/(func ConvFor\(t types\.Type\) Conv \{\n)/$1\tif ReflipGeneric_(t, '$inst') {\n\t\treturn Conv{IsUnbox: func(string) bool { return false }}\n\t}\n/' $rwgo
  cat >> $rwgo <<'G'

// ReflipGeneric_ reports whether t mentions a type parameter or (inst) an instance of a generic
// type.
func ReflipGeneric_(t types.Type, inst bool) bool {
	seen := map[types.Type]bool{}
	var walk func(t types.Type) bool
	walk = func(t types.Type) bool {
		if t == nil || seen[t] {
			return false
		}
		seen[t] = true
		switch u := t.(type) {
		case *types.TypeParam:
			return true
		case *types.Alias:
			for i := 0; i < u.TypeArgs().Len(); i++ {
				if walk(u.TypeArgs().At(i)) {
					return true
				}
			}
			return walk(types.Unalias(u))
		case *types.Named:
			if inst && u.TypeArgs().Len() > 0 {
				return true
			}
			for i := 0; i < u.TypeArgs().Len(); i++ {
				if walk(u.TypeArgs().At(i)) {
					return true
				}
			}
		case *types.Pointer:
			return walk(u.Elem())
		case *types.Slice:
			return walk(u.Elem())
		case *types.Array:
			return walk(u.Elem())
		case *types.Map:
			return walk(u.Key()) || walk(u.Elem())
		}
		return false
	}
	return walk(t)
}
G
  grep -q 'if ReflipGeneric_' $rwgo || { echo "toolfix: rw.go: ConvFor was not found" >&2; exit 1; }
fi
# 3. colls: slots that exchange values form a class; the slots of a function are found by its
#    name and signature. A method of a generic type called at an instantiation (V = Set) has the
#    instantiated signature, which no declaration has, and a field or variable whose declared
#    type is a type parameter is no collection declaration: a collection that meets one cannot
#    change type (the type argument at the instantiation would have to change with it).
cg=tools/postflip/colls/main.go
if [[ -f $cg ]] && ! grep -q 'reflip:' $cg; then
  perl -0pi -e '
    s/(\t\t\treturn value\{kind: vSlot, k: resultKey\(groupOf\(f\), 0\)\})/\t\t\tif f.Origin() != f { \/\/ reflip: generic instance\n\t\t\t\tpoison(resultKey(groupOf(f.Origin()), 0), "result of a method of a generic type")\n\t\t\t\treturn value{kind: vForeign, why: "result of a method of a generic type"}\n\t\t\t}\n$1/;
    s/(\treturn paramKey\(groupOf\(f\), i\), true)/\tif f.Origin() != f { \/\/ reflip: generic instance\n\t\tpoison(paramKey(groupOf(f), i), "argument of a method of a generic type")\n\t\tpoison(paramKey(groupOf(f.Origin()), i), "argument of a method of a generic type")\n\t}\n$1/;
    s/(func slotOfVar\(v \*types\.Var\) int \{\n\tk := varKey\(v\)\n)/$1\tif v.Origin() != v || rw.ReflipGeneric_(v.Type(), false) { \/\/ reflip: generic instance\n\t\tpoison(k, "variable of a type parameter\x27s type")\n\t}\n/;
  ' $cg
  [[ $(grep -c 'reflip:' $cg) == 3 ]] || { echo "toolfix: colls: anchors not found" >&2; exit 1; }
fi
# 4. native: the same rule as 3 (the tool has colls' slots as graph nodes).
ng=tools/postflip/native/main.go
if [[ -f $ng ]] && ! grep -q 'reflip:' $ng; then
  perl -0pi -e '
    s/(\t\t\treturn value\{kind: vSlot, n: resultNode\(groupOf\(f\), 0\)\})/\t\t\tif f.Origin() != f { \/\/ reflip: generic instance\n\t\t\t\tpoison(resultNode(groupOf(f.Origin()), 0), "result of a method of a generic type")\n\t\t\t\treturn value{kind: vForeign, why: "result of a method of a generic type"}\n\t\t\t}\n$1/;
    s/(\treturn paramNode\(groupOf\(f\), i\), true)/\tif f.Origin() != f { \/\/ reflip: generic instance\n\t\tpoison(paramNode(groupOf(f), i), "argument of a method of a generic type")\n\t\tpoison(paramNode(groupOf(f.Origin()), i), "argument of a method of a generic type")\n\t}\n$1/;
    s/(func varNode\(v \*types\.Var\) \*node \{\n\tn := get\(fmt\.Sprintf\("v\|%d", v\.Pos\(\)\)\)\n)/$1\tif v.Origin() != v || rw.ReflipGeneric_(v.Type(), false) { \/\/ reflip: generic instance\n\t\tpoison(n, "variable of a type parameter\x27s type")\n\t}\n/;
  ' $ng
  [[ $(grep -c 'reflip:' $ng) == 3 ]] || { echo "toolfix: native: anchors not found" >&2; exit 1; }
fi
# 5. funcparams: an object passed where a func is now wanted becomes its method value, o.m. The
#    erased code never passed a literal &T{...} there that was not a lambda wrapper F_fn (those
#    are unwrapped); j2go's typed adapters (&Consumer_tfn_Node{func...}) are such literals, and
#    &T{...}.m is not Go: (&T{...}).m.
fg=tools/postflip/funcparams/main.go
if [[ -f $fg ]] && grep -q 'return rw.Render(o) + "." + m' $fg && ! grep -q 'reflipRecv_' $fg; then
  perl -pi -e 's/return rw\.RenderChildren\(o\) \+ "\." \+ m/return reflipRecv_(o, rw.RenderChildren(o)) + "." + m/; s/return rw\.Render\(o\) \+ "\." \+ m/return reflipRecv_(o, rw.Render(o)) + "." + m/' $fg
  cat >> $fg <<'G'

// reflipRecv_ parenthesizes the text s of receiver o where a selector would not bind to it.
func reflipRecv_(o ast.Expr, s string) string {
	if _, ok := o.(*ast.UnaryExpr); ok {
		return "(" + s + ")"
	}
	return s
}
G
fi
# 6. colls: `x := conv(el)` (a short declaration of a collection variable) is what native's range
#    loops over immElems_ leave where the element is erased. The flow pass takes the type of the
#    assigned place from go/types' expression types, which have no entry for a defining
#    identifier, so the value "left for a place that is no collection" and the variable's class
#    could be typed with the declaration unchanged. First flip: the same loops ranged over
#    typed slices (their variables are "elements of a Go slice", poisoned). Now the
#    variable of a short declaration is a slot that stays erased (the value it is given is
#    viewed at the declaration, as the flow pass already writes it).
if [[ -f $cg ]] && ! grep -q 'reflip6' $cg; then
  perl -0pi -e 's/(\t\t\tcase \*ast\.AssignStmt:\n\t\t\t\tif len\(x\.Lhs\) != len\(x\.Rhs\) \{\n\t\t\t\t\treturn true\n\t\t\t\t\}\n\t\t\t\tfor i, r := range x\.Rhs \{\n)(\t\t\t\t\tif other\(r\) && isList\(typeOf\(x\.Lhs\[i\]\)\) \{)/$1\t\t\t\t\tif dv := varOf(x.Lhs[i]); x.Tok == token.DEFINE \&\& dv != nil \&\& isList(dv.Type()) \&\& !other(r) { \/\/ reflip6\n\t\t\t\t\t\tpoison(slotOfVar(dv), "variable of a short declaration")\n\t\t\t\t\t}\n$2/' $cg
fi
# 7. (second trial) precond: the typed translator writes checkNotNull as a Go generic function of
#    the argument's static type (Preconditions_checkNotNull_..._T, _Str: rule P2) and passes
#    message arguments at their own type (P4). The tool's rewrite `if v == nil { check(nil, ...) }`
#    is for the erased forms (v an interface): on these it compares a handle or a string with nil
#    and leaves T without an argument to infer it from. Such calls are left, counted.
pc=tools/postflip/precond/main.go
if [[ -f $pc ]] && ! grep -q 'reflip7' $pc; then
  perl -0pi -e 's/(\t\t\tnotNull := strings\.HasPrefix\(id\.Name, "Preconditions_checkNotNull"\)\n)/\t\t\tif strings.HasSuffix(id.Name, "_T") || strings.HasSuffix(id.Name, "_Str") { \/\/ reflip7\n\t\t\t\tcount["generic checkNotNull of the typed translator, left"]++\n\t\t\t\treturn true\n\t\t\t}\n$1/' $pc
  grep -q 'reflip7' $pc || { echo "toolfix: precond: the checkNotNull line was not found" >&2; exit 1; }
fi
# 9. (second trial) colls: `var x Map = it.v`, the element of a for-each cursor over one of the
#    translator's typed collections (a *JTList[Map]: its elements are erased collections). The
#    flow pass follows values of the collection kinds through variables, fields, calls and
#    containers it knows; a cursor's element field is none of them, so the variable's class could
#    be typed with the cursor still yielding the erased type. The variable is a slot that stays
#    erased.
if [[ -f $cg ]] && ! grep -q 'reflip9' $cg && grep -q '"strings"' $cg; then
  perl -0pi -e 's/(\t\t\t\tfor i, r := range x\.Values \{\n)(\t\t\t\t\tif v, ok := rw\.Info\.Defs\[x\.Names\[i\]\]\.\(\*types\.Var\); ok && other\(r\) && isList\(v\.Type\(\)\) \{)/$1\t\t\t\t\tif v, ok := rw.Info.Defs[x.Names[i]].(*types.Var); ok \&\& isList(v.Type()) { \/\/ reflip9\n\t\t\t\t\t\tif se, ok := r.(*ast.SelectorExpr); ok \&\& se.Sel.Name == "v" \&\& strings.Contains(types.TypeString(rw.Info.TypeOf(se.X), nil), "Each_") {\n\t\t\t\t\t\t\tpoison(slotOfVar(v), "element of a cursor over a typed collection of the translator")\n\t\t\t\t\t\t}\n\t\t\t\t\t}\n$2/' $cg
fi
# 10. (second trial) native: a result or parameter whose collection type mentions a type
#    parameter (`[]InputT` of a method of SortedDependencies[InputT]), used inside the generic
#    type itself: the method object there is its own origin, so fix 3 (instances) did not see it,
#    and the uses that need the element's boxing (x.asList() -> sliceView_(tc, x)) have none to
#    write. The declaration stays, as variables of such types already do.
ng=tools/postflip/native/main.go
if [[ -f $ng ]] && ! grep -q 'reflip10' $ng && grep -q 'reflip: generic instance' $ng; then
  perl -0pi -e 's/(\t\t\tif f\.Origin\(\) != f \{ \/\/ reflip: generic instance\n\t\t\t\tpoison\(resultNode\(groupOf\(f\.Origin\(\)\), 0\), "result of a method of a generic type"\))/\t\t\tif rw.ReflipGeneric_(f.Type().(*types.Signature).Results().At(0).Type(), false) { \/\/ reflip10\n\t\t\t\tpoison(resultNode(groupOf(f), 0), "result of a type parameter\x27s type")\n\t\t\t\treturn value{kind: vForeign, why: "result of a type parameter\x27s type"}\n\t\t\t}\n$1/' $ng
  perl -0pi -e 's/(\t\t\t\tflow\(resultNode\(g, i\), e, true\)\n)/\t\t\t\tif rw.ReflipGeneric_(resultType(x, i), false) { \/\/ reflip10\n\t\t\t\t\tpoison(resultNode(g, i), "result of a type parameter\x27s type")\n\t\t\t\t}\n$1/' $ng
  perl -0pi -e 's/(\tif f\.Origin\(\) != f \{ \/\/ reflip: generic instance\n\t\tpoison\(paramNode\(groupOf\(f\), i\), "argument of a method of a generic type"\))/\tif rw.ReflipGeneric_(sig.Params().At(i).Type(), false) { \/\/ reflip10\n\t\tpoison(paramNode(groupOf(f), i), "parameter of a type parameter\x27s type")\n\t}\n$1/' $ng
fi
# 11. (fourth flip) visit: the translator's rule "implicit conversions" writes a Java upcast of a visitor to its interface as a Go
#    conversion, Visitor(this), and the nil-keeping upcast helpers X_as_Visitor(v *X) Visitor { if v == nil { return nil }; return v }
#    (g_0casts.go); the erased output had icast_Visitor(obj_(this)) there. The tool follows visitor values through classes, slots
#    and assertions: a conversion to a visitor interface is the value of its operand, the type name it spells is a slot of that
#    value's class (it becomes Visitor[T](x)), and a nil stored, passed or returned as a visitor is no value of any class.
vg=tools/postflip/visit/main.go
if [[ -f $vg ]] && ! grep -q 'reflip11' $vg; then
  perl -0pi -e '
    s/(func valueNode\(e ast\.Expr\) any \{\n\te = rw\.Unparen\(e\)\n)/$1\tif a := reflipConvArg_(e); a != nil { \/\/ reflip11\n\t\treturn valueNode(a)\n\t}\n/;
    s/(\tcase \*ast\.TypeAssertExpr:\n\t\treturn x\n\t\}\n\treturn nil\n\}\n\n\/\/ ---- rewriting)/\tcase *ast.CallExpr: \/\/ reflip11: the type name of a conversion Visitor(x)\n\t\tif reflipConvArg_(x) != nil \&\& rw.Unparen(x.Fun) == ast.Expr(id) {\n\t\t\treturn x\n\t\t}\n$1/;
    s/(\t\t\tcase \*ast\.CallExpr:\n)(\t\t\t\tfn := calledFunc\(n\)\n\t\t\t\tif fn == nil \{\n\t\t\t\t\treturn true\n\t\t\t\t\}\n\t\t\t\tsig, ok := fn\.Type\(\)\.\(\*types\.Signature\)\n\t\t\t\tif !ok \|\| sig\.Variadic\(\))/$1\t\t\t\tif a := reflipConvArg_(n); a != nil { \/\/ reflip11\n\t\t\t\t\tif vn := valueNode(a); vn != nil {\n\t\t\t\t\t\tunion(n, vn)\n\t\t\t\t\t} else if !reflipNil_(a) {\n\t\t\t\t\t\tfail(a, "a value %s converted to a visitor that is not a visitor class or slot", rw.Text(a))\n\t\t\t\t\t}\n\t\t\t\t\treturn true\n\t\t\t\t}\n$2/;
    s/(\t\t\t\t\t\tif vn := valueNode\(a\); vn != nil \{\n\t\t\t\t\t\t\tunion\(p\.Origin\(\), vn\)\n\t\t\t\t\t\t\} else) \{/$1 if !reflipNil_(a) { \/\/ reflip11/;
    s/(\t\t\t\t\tif vn := valueNode\(n\.Results\[0\]\); vn != nil \{\n\t\t\t\t\t\tunion\(ft, vn\)\n\t\t\t\t\t\} else) \{/$1 if !reflipNil_(n.Results[0]) { \/\/ reflip11/;
    s/(\tif lt == nil \|\| !isVisitorType\(lt\)) \{/$1 || reflipNil_(r) { \/\/ reflip11/;
  ' $vg
  cat >> $vg <<'G'

// reflipConvArg_ is the operand x of a conversion I(x) to a visitor interface I, or nil.
func reflipConvArg_(e ast.Expr) ast.Expr {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 1 {
		return nil
	}
	id, ok := rw.Unparen(c.Fun).(*ast.Ident)
	if !ok {
		return nil
	}
	tn, ok := rw.Info.Uses[id].(*types.TypeName)
	if !ok || !isVisitorType(tn.Type()) {
		return nil
	}
	return c.Args[0]
}

// reflipNil_ reports whether e is the predeclared nil.
func reflipNil_(e ast.Expr) bool {
	id, ok := rw.Unparen(e).(*ast.Ident)
	if !ok {
		return false
	}
	_, isNil := rw.Info.Uses[id].(*types.Nil)
	return isNil
}
G
  [[ $(grep -c 'reflip11' $vg) == 6 ]] || { echo "toolfix: visit: anchors not found" >&2; exit 1; }
fi
