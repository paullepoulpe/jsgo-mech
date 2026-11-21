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
# 12. (fourth flip) strids: the tool turns scope.getVar(n.getString()) into scope.getVarById_(n.getStringId_()) for the method
#    getVar of AbstractScope or TypedScope. With the Java prep step "scope push-down" and the translator's rule "covariant
#    overrides", getVar is declared on Scope and TypedScope and spelled with its result (getVar_String_rVar,
#    getVar_String_rTypedVar): the call is rewritten for any spelling of getVar(String) on a scope class that has getVarById_.
sg=tools/postflip/strids/main.go
if [[ -f $sg ]] && ! grep -q 'reflip12' $sg && grep -q 'sel.Sel.Name != "getVar" || len(x.Args) != 1' $sg; then
  perl -0pi -e '
    s/if !ok \|\| sel\.Sel\.Name != "getVar" \|\| len\(x\.Args\) != 1 \{/if !ok || (sel.Sel.Name != "getVar" \&\& !strings.HasPrefix(sel.Sel.Name, "getVar_String_r")) || len(x.Args) != 1 { \/\/ reflip12/;
    s/if !ok \|\| \(nt\.Obj\(\)\.Name\(\) != "AbstractScope" && nt\.Obj\(\)\.Name\(\) != "TypedScope"\) \{/if !ok || (nt.Obj().Name() != "AbstractScope" \&\& nt.Obj().Name() != "TypedScope" \&\& nt.Obj().Name() != "Scope") || !rw.HasMethod(types.NewPointer(nt), "getVarById_") { \/\/ reflip12/;
  ' $sg
  [[ $(grep -c 'reflip12' $sg) == 2 ]] || { echo "toolfix: strids: anchors not found" >&2; exit 1; }
fi
# reflip_edit FILE MARK <<pairs: exact-text edits of a tool source (python: each OLD must occur exactly once), skipped once MARK is in
# the file, and when the first OLD is not in the file at all (an earlier version of the tool). The pairs are separated by lines "=====" (between OLD and NEW) and "-----" (between edits); a line "+++++" starts text
# appended to the file.
reflip_edit() {
  local f=$1 mark=$2
  [[ -f $f ]] || { cat > /dev/null; return 0; }
  if grep -q "$mark" "$f"; then cat > /dev/null; return 0; fi
  python3 -c '
import sys
f, mark = sys.argv[1], sys.argv[2]
s = open(f).read()
spec = sys.stdin.read()
spec, _, tail = spec.partition("\n+++++\n")
for n, ed in enumerate(spec.split("\n-----\n")):
    old, sep, new = ed.partition("\n=====\n")
    if n == 0 and sep and s.count(old.strip("\n")) == 0:
        sys.exit(0)  # a version of the tool without the code the fix is for (the step that needs it has it)
    if not sep:
        sys.exit("toolfix: bad edit spec for " + f)
    old, new = old.strip("\n"), new.strip("\n")
    if s.count(old) != 1:
        sys.exit("toolfix: %s: anchor found %d times: %s" % (f, s.count(old), old[:90]))
    s = s.replace(old, new)
if mark not in s + tail:
    sys.exit("toolfix: the edits of %s do not carry the mark %s" % (f, mark))
open(f, "w").write(s + tail)
' "$f" "$mark"
}
# 13. (fourth flip) enumin: a constant immutable enum set is now a typed immutable set (the translator's typed Guava immutables and step
#    402): `var X *TSet[E]`, `X = tsetImm_(tc, Sets_immutableEnumSet(A.enum_(), []*Enum{...}))`, tested by `X.Has(x)` with the value
#    bare. The tool read the erased forms only (X an *ImmutableCollection, X.contains(obj_(x))) and found no test. The typed set
#    holds exactly the constants the erased one is built from and Has(x) is contains(x) (null, E's 0, is in no set), so the same
#    switch stands for both.
reflip_edit tools/postflip/enumin/main.go reflip13 <<'E13'
			name, call := rw.CallName(as[0].Rhs[0])
			if name != "Sets_immutableEnumSet" || len(call.Args) != 2 {
=====
			name, call := rw.CallName(as[0].Rhs[0])
			if (name == "tsetImm_" || strings.HasPrefix(name, "tsetImm_[")) && len(call.Args) == 2 { // reflip13: the typed immutable set of the erased one
				name, call = rw.CallName(call.Args[1])
			}
			if name != "Sets_immutableEnumSet" || len(call.Args) != 2 {
-----
			if !ok || sel.Sel.Name != "contains" {
				return true
			}
			name, arg := rw.CallName(call.Args[0])
			if arg == nil || (name != "obj_" && !strings.HasPrefix(name, "obj_[")) || len(arg.Args) != 1 {
				return true
			}
			x := arg.Args[0]
=====
			if !ok || (sel.Sel.Name != "contains" && sel.Sel.Name != "Has") {
				return true
			}
			var x ast.Expr
			if reflipTSet_(rw.Info.Types[sel.X].Type) { // reflip13: a typed set takes the value bare
				x = call.Args[0]
			} else {
				name, arg := rw.CallName(call.Args[0])
				if sel.Sel.Name != "contains" || arg == nil || (name != "obj_" && !strings.HasPrefix(name, "obj_[")) || len(arg.Args) != 1 {
					return true
				}
				x = arg.Args[0]
			}
-----
			if t, ok := v.Type().(*types.Pointer); !ok || !isNamed(t.Elem(), "ImmutableCollection") {
=====
			if t, ok := v.Type().(*types.Pointer); (!ok || !isNamed(t.Elem(), "ImmutableCollection")) && !reflipTSet_(v.Type()) {
+++++

// reflipTSet_ reports whether t is *TSet[E], the typed set.
func reflipTSet_(t types.Type) bool {
	p, ok := types.Unalias(t).(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	return ok && n.Origin().Obj().Name() == "TSet"
}
E13
# 14. (fourth flip) tfnparams: a named object given where a functional interface is taken (LogFile.log(new RemovalLogRecord(..)))
#    was passed as the class pointer; the translator's rule "implicit conversions" now writes the Java upcast out, Supplier(new_C(..))
#    or the nil-keeping helper C_as_Supplier(x) of g_0casts.go. The tool's "named object" is the operand of such an upcast (the
#    object is never null, so both are the object), and the rewrite takes the typed method value of the operand.
reflip_edit tools/postflip/tfnparams/main.go reflip14 <<'E14'
func namedObj(e ast.Expr) *types.Named {
	return namedObjN(e, 3)
}
=====
func namedObj(e ast.Expr) *types.Named {
	return namedObjN(reflipUnwrap_(e), 3) // reflip14
}
-----
		rw.Rewrite[a] = func() string { return rw.RenderChildren(a) + "." + g.method + "T_" }
=====
		rw.Rewrite[a] = func() string {
			if in := reflipUnwrap_(a); in != a {
				return rw.Render(in) + "." + g.method + "T_"
			}
			return rw.RenderChildren(a) + "." + g.method + "T_"
		}
+++++

// reflipUnwrap_ is the operand x of an upcast I(x) to an interface type, or of a nil-keeping
// upcast helper C_as_I(x) (`if v == nil { return nil }; return v`); else e.
func reflipUnwrap_(e ast.Expr) ast.Expr {
	c, ok := rw.Unparen(e).(*ast.CallExpr)
	if !ok || len(c.Args) != 1 {
		return e
	}
	id, ok := rw.Unparen(c.Fun).(*ast.Ident)
	if !ok {
		return e
	}
	switch o := rw.Info.Uses[id].(type) {
	case *types.TypeName:
		if types.IsInterface(o.Type()) {
			return c.Args[0]
		}
	case *types.Func:
		fd := declOf[o]
		if fd == nil || fd.Body == nil || len(fd.Body.List) != 2 || !strings.Contains(o.Name(), "_as_") || len(fd.Type.Params.List) != 1 || len(fd.Type.Params.List[0].Names) != 1 {
			return e
		}
		_, isIf := fd.Body.List[0].(*ast.IfStmt)
		r, isRet := fd.Body.List[1].(*ast.ReturnStmt)
		if !isIf || !isRet || len(r.Results) != 1 {
			return e
		}
		if rid, ok := r.Results[0].(*ast.Ident); ok && rw.Info.Uses[rid] == rw.Info.Defs[fd.Type.Params.List[0].Names[0]] {
			return c.Args[0]
		}
	}
	return e
}
E14
# 15. (fourth flip) optstr: the one erased map that holds Optional<String> values (ContextualRenamer.declarations) is "kept" when its
#    only assignment is a new LinkedHashMap / HashMap; the translator's rule "implicit conversions" writes that assignment with the
#    upcast, Map(new_LinkedHashMap()). The value assigned is the same new map: the proof reads the operand of the conversion.
reflip_edit tools/postflip/optstr/proof.go reflip15 <<'E15'
			if name, _ := rw.CallName(p.Rhs[0]); name != "new_LinkedHashMap" && name != "new_HashMap" {
=====
			rhs := p.Rhs[0]
			if c, ok := rw.Unparen(rhs).(*ast.CallExpr); ok && len(c.Args) == 1 { // reflip15: Map(new_LinkedHashMap())
				if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() && types.IsInterface(tv.Type) {
					rhs = c.Args[0]
				}
			}
			if name, _ := rw.CallName(rhs); name != "new_LinkedHashMap" && name != "new_HashMap" {
E15
# 16. (fourth flip) sbuild: the translator's rule "typed Object calls" writes sb.toString() of a StringBuilder as tostrR_(sb)
#    (rt_conv.go: Object_toString(obj_(sb)), the form the tool knows, as one generic call). The tool took tostrR_ for an unknown
#    function the builder is passed to and converted no builder at all (0 of 119 / 205 slots; the third flip: 39 builders, 41
#    classes). tostrR_(sb) is read as the Object_toString(obj_(sb)) it is (first version of the tool) and as tostrNN_(sb), the
#    tostr step's spelling of the same call (later versions).
bg=tools/postflip/sbuild/main.go
if [[ -f $bg ]] && ! grep -q 'reflip16' $bg; then
  perl -pi -e 's/^(\s*)case "Object_toString", "String_valueOf_Object", "objstr_":$/$1case "Object_toString", "String_valueOf_Object", "objstr_", "tostrR_": \/\/ reflip16/; s/\(fn\.Name\(\) == "tostr_" \|\| fn\.Name\(\) == "tostrNN_"\)/(fn.Name() == "tostr_" || fn.Name() == "tostrNN_" || fn.Name() == "tostrR_")/' $bg
  grep -q 'reflip16' $bg || { echo "toolfix: sbuild: anchors not found" >&2; exit 1; }
fi
# 17. (fourth flip) funcparams: an object certainly not null given where a function is now taken becomes its method value (o.m). The
#    tool read the object under the erased upcast icast_I(obj_(o)); the translator's rule "implicit conversions" writes the upcast
#    as the conversion I(o) (RhinoErrorReporter_forOldRhino(ErrorHandler(this))). The object of a conversion to an interface type is
#    its operand.
reflip_edit tools/postflip/funcparams/main.go reflip17 <<'E17'
func boxed(e ast.Expr) ast.Expr {
	e = rw.Unparen(e)
=====
func boxed(e ast.Expr) ast.Expr {
	e = rw.Unparen(e)
	if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 { // reflip17: the upcast written out, I(x)
		if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() && types.IsInterface(tv.Type) {
			e = rw.Unparen(c.Args[0])
		}
	}
E17
# 20. (in-bounds line) generics -classes: the String variants of a generic class's methods (m_s: m with its key typed string, which
#    the step strvar writes where the manual steps 29, 35 and 121 wrote them by hand) are no Java declarations, so their key is
#    no type-variable slot, and a call x.m_s(s) said nothing about x's type argument: an object whose every keyed call had become a
#    variant's (SimpleBiMap, a SimpleListMultimap, a SimpleSetMultimap) stayed erased, C[any, V]. x.m_s(s, ...) is x.m(strobj_(s), ...)
#    by the convention of the variants (strkeys' rule 1 rewrites the one to the other on that ground), so the string is written to
#    the slot of m's first parameter: evidence for that type argument like the boxed string it stands for.
reflip_edit tools/postflip/generics/classes.go reflipSvar <<'E20'
			case *ast.CallExpr:
				if len(v.Args) == 1 {
					if name, _ := rw.CallName(v); name != "" && unboxNames.MatchString(name) {
=====
			case *ast.CallExpr:
				// reflipSvar: x.m_s(s, ...) is x.m(strobj_(s), ...): a string written to m's first slot
				if sel, ok := v.Fun.(*ast.SelectorExpr); ok && strings.HasSuffix(sel.Sel.Name, "_s") && len(v.Args) > 0 {
					if sn := rw.Info.Selections[sel]; sn != nil && sn.Kind() == types.MethodVal {
						if at := rw.Info.Types[v.Args[0]].Type; at != nil && types.Identical(types.Unalias(at), types.Typ[types.String]) {
							if mo, _, _ := types.LookupFieldOrMethod(sn.Recv(), true, rw.Pkg, strings.TrimSuffix(sel.Sel.Name, "_s")); mo != nil {
								if mf, ok := mo.(*types.Func); ok {
									if ms := mf.Type().(*types.Signature); ms.Params().Len() > 0 {
										if tg := tagOf(ms.Params().At(0).Type()); tg != "" {
											writeEv(tg, types.Typ[types.String], nil)
										}
									}
								}
							}
						}
					}
				}
				if len(v.Args) == 1 {
					if name, _ := rw.CallName(v); name != "" && unboxNames.MatchString(name) {
E20
# 21. (fourth flip, in bounds) checks: the translator's rules P2 and P4 write a precondition whose checked value or one message
#    argument has a known type as a typed variant of the runtime function: Preconditions_check..._H(c, [t,] n) takes a handle n and
#    boxes it only when the check fails (n.box_()); Preconditions_checkNotNull..._T(v, ...) and ..._Str(v, ...) take the checked value
#    at its own type (null is the zero value of T, nullstr_ of a String), return it typed, and take the message arguments as they
#    are (an Object message is String.valueOf(canon_(msg))). The tool read the erased names only, so it left all of these (1,044 of
#    2,503 checks: the lead line's translator wrote none of them and its step made 2,497 of 2,501 inline). A typed variant is the
#    erased check of the same name with: the handle read as boxed by box_() (_H); the checked value tested at its type by the
#    tool's own null test (_T, _Str); a single Object message read under the boxing canon_ gives it (a String strobj_, anything
#    else canon_ itself, as the runtime function writes it). Every rewrite is then the tool's own, with the runtime function's exception and
#    message.
reflip_edit tools/postflip/checks/main.go reflip21 <<'E21'
func unbox(e ast.Expr) (string, ast.Expr) {
	e = rw.Unparen(e)
=====
func unbox(e ast.Expr) (string, ast.Expr) {
	e = rw.Unparen(e)
	if b, ok := reflipBox_[e]; ok { // reflip21: an argument a typed variant boxes itself
		return b, e
	}
-----
		if box, inner := unbox(x); box != "%s" {
			return late(inner)
		}
=====
		if box, inner := unbox(x); box != "%s" && inner != ast.Expr(x) {
			return late(inner)
		}
-----
	switch {
	case box == "%s" || box == "any(%s)":
		if _, isIface := u.(*types.Interface); !isIface {
=====
	switch {
	case box == "%s" && reflipChecked_[in]: // reflip21: the value of a typed checkNotNull, tested at its type
		test, ok := nullTest(t, primary(in))
		return test, in, ok
	case box == "%s" || box == "any(%s)":
		if _, isIface := u.(*types.Interface); !isIface {
-----
		case rw.HasMethod(t, "box_") && u.Info()&types.IsInteger != 0:
			return v + " == 0", true
=====
		case rw.HasMethod(t, "box_") && u.Info()&types.IsInteger != 0:
			return v + " == 0", true
		case reflipTypedNull_ && rw.HasMethod(t, "enum_") && u.Info()&types.IsInteger != 0: // an enum constant: null is 0
			return v + " == 0", true
-----
	parts := strings.Split(strings.TrimPrefix(id.Name, "Preconditions_check"), "_")
	k := check{call: c, kind: parts[0]}
=====
	parts := strings.Split(strings.TrimPrefix(id.Name, "Preconditions_check"), "_")
	variant := ""
	if n := len(parts); n > 1 && (parts[n-1] == "H" || parts[n-1] == "T" || parts[n-1] == "Str") {
		variant, parts = parts[n-1], parts[:n-1]
	}
	k := check{call: c, kind: parts[0]}
-----
	if len(k.sig) != len(c.Args)-1 {
		return check{}, false
	}
	return k, true
=====
	if len(k.sig) != len(c.Args)-1 {
		return check{}, false
	}
	if variant != "" && !reflipVariant_(k, variant) {
		return check{}, false
	}
	return k, true
+++++

// reflipBox_ gives the boxing (a format, as unbox returns) of a message argument that a typed
// variant of a check boxes itself; reflipChecked_ are the values typed checkNotNulls check.
var reflipBox_ = map[ast.Expr]string{}
var reflipChecked_ = map[ast.Expr]bool{}
var reflipSeen_ = map[*ast.CallExpr]bool{}

// reflipTypedNull_: the null test of an enum value (0) is known; set once a typed variant is seen
// (the erased forms never ask it: an enum constant is boxed by self()).
var reflipTypedNull_ = true

// reflipVariant_ reads the typed variant (_H, _T, _Str) of check k as the erased check of the
// same name: it records how the runtime function boxes the arguments it takes typed. It is
// false (the call is left, counted) for a form the tool has no exact rewrite of.
func reflipVariant_(k check, variant string) bool {
	c := k.call
	if ok, seen := reflipSeen_[c]; seen {
		return ok
	}
	ok := reflipVariant1_(k, variant)
	reflipSeen_[c] = ok
	if !ok {
		leave("left: typed variant _"+variant+" in a form without a rewrite", c)
	}
	return ok
}

func reflipPrimary_(e ast.Expr) bool {
	switch e.(type) {
	case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr:
		return true
	}
	return false
}

func reflipVariant1_(k check, variant string) bool {
	c := k.call
	switch variant {
	case "H": // the last argument is a handle, boxed by box_() where the check fails
		if k.kind == "NotNull" || len(c.Args) < 2 {
			return false
		}
		a := rw.Unparen(c.Args[len(c.Args)-1])
		t := rw.Info.Types[a].Type
		if t == nil || !rw.HasMethod(t, "box_") || !reflipPrimary_(a) {
			return false
		}
		reflipBox_[a] = "%s.box_()"
		return true
	case "T", "Str": // the checked value at its own type; message arguments as they are
		if k.kind != "NotNull" {
			return false
		}
		v := rw.Unparen(c.Args[0])
		t := rw.Info.Types[v].Type
		if t == nil {
			return false
		}
		b, _ := t.Underlying().(*types.Basic)
		if (b != nil && b.Kind() == types.String) != (variant == "Str") {
			return false
		}
		if _, ok := nullTest(t, "v"); !ok {
			return false
		}
		reflipChecked_[v] = true
		if len(k.sig) == 1 { // one Object message: String.valueOf(canon_(msg))
			a := rw.Unparen(c.Args[1])
			at := rw.Info.Types[a].Type
			if box, _ := unbox(a); box == "%s" && at != nil && !rw.Info.Types[a].IsNil() {
				ab, _ := at.Underlying().(*types.Basic)
				switch {
				case ab != nil && ab.Kind() == types.String:
					reflipBox_[a] = "strobj_(%s)"
				default:
					reflipBox_[a] = "canon_(%s)"
				}
			}
		}
		return true
	}
	return false
}
E21
# 22. (fourth flip, in bounds) sbuild: two more forms of this translator that kept 41 StringBuilder slots (16 "passed to jtostr_",
#    25 "returned as another type"; the third flip's tool fix e8c5ab2d on reflip2-bridge, which the lead line's tools never had
#    because its translator wrote neither form):
#    - jtostr_(sb), the translator's String.valueOf at the argument's type (rt_conv.go: objstr_(obj_(sb)), "null" for nil), is the
#      read tostr_(sb) is, handled the same way;
#    - StringBuilder_as_Appendable(sb), the nil-keeping upcast helper of g_0casts.go (rule "implicit conversions"), is where the
#      erased output had the builder itself in an Appendable position: the helper's parameter is no slot, the argument is a builder
#      passed as an Appendable (appendable_(sb), which keeps nil too) and the helper call around a converted builder is dropped.
if [[ -f $bg ]] && ! grep -q 'reflip22' $bg && grep -q 'fn.Name() == "tostrR_")' $bg; then
  perl -pi -e 's/\|\| fn\.Name\(\) == "tostrR_"\) && len\(x\.Args\) == 1/|| fn.Name() == "tostrR_" || fn.Name() == "jtostr_") \&\& len(x.Args) == 1/' $bg
  grep -q '"jtostr_")' $bg || { echo "toolfix: sbuild: the tostr_ line was not found" >&2; exit 1; }
  reflip_edit $bg reflip22 <<'E22'
func appendArg(k key, e ast.Expr, local bool, v *types.Var) {
=====
// reflipAsAppendable_ is the translator's upcast of a *StringBuilder to Appendable (reflip22); reflipUpcasts_ are its calls
// around a builder of each class.
const reflipAsAppendable_ = "StringBuilder_as_Appendable"

var reflipUpcasts_ = map[key][]*ast.CallExpr{}

func appendArg(k key, e ast.Expr, local bool, v *types.Var) {
-----
				sig := fn.Type().(*types.Signature)
				results[x] = sig
				fieldList(x.Type.Params, func(i int) *types.Var { return sig.Params().At(i) }, g, "")
=====
				sig := fn.Type().(*types.Signature)
				results[x] = sig
				if x.Recv == nil && x.Name.Name == reflipAsAppendable_ { // reflip22: its parameter is no slot
					return true
				}
				fieldList(x.Type.Params, func(i int) *types.Var { return sig.Params().At(i) }, g, "")
-----
				for i, a := range x.Args {
					if rw.Unparen(a) == e {
						if pv := paramVar(fn, i); pv != nil && isSlot[pv] {
=====
				if fn.Name() == reflipAsAppendable_ && len(x.Args) == 1 && rw.Unparen(x.Args[0]) == e { // reflip22
					appendArg(k, e, local, v)
					reflipUpcasts_[k] = append(reflipUpcasts_[k], x)
					return
				}
				for i, a := range x.Args {
					if rw.Unparen(a) == e {
						if pv := paramVar(fn, i); pv != nil && isSlot[pv] {
-----
	if nApp > 0 {
		fmt.Fprintf(os.Stderr, "builders passed as an Appendable (appendable_): %d\n", nApp)
	}
=====
	for k, cs := range reflipUpcasts_ { // reflip22: the upcast helper around a converted builder goes
		if !conv[k] {
			continue
		}
		for _, c := range cs {
			c := c
			rw.Rewrite[c] = func() string { return rw.Render(c.Args[0]) }
		}
	}
	if nApp > 0 {
		fmt.Fprintf(os.Stderr, "builders passed as an Appendable (appendable_): %d\n", nApp)
	}
E22
  grep -q 'reflip22' $bg || echo '// reflip22: jtostr_ only (this version has no Appendable arguments)' >> $bg
fi
# 23. (fourth flip, in bounds) eqhash: the translator's rules "typed Object methods" and "typed Object calls" write the protocol's
#    calls on values held at their static type as generic helpers of rt_conv.go, each defined as the erased call the tool reads:
#    equals_(a, b) = objequals_(obj_(a), obj_(b)), hash_(v) = objhash_(obj_(v)), equalsR_(a, B) = Object_equals(obj_(a), B),
#    equalsRR_(a, b) = Object_equals(obj_(a), obj_(b)), hashR_(v) = Object_hashCode(obj_(v)), the H variants with box_() for
#    obj_, and Objects_requireNonNull_T / _Str / _Object_T / _Object_Str, requireNonNull of a value at its type. The tool read the
#    erased names only and left all of them (step 189: 80 calls rewritten; the third flip 294 changed lines in 92 files). A typed
#    helper is read as its definition: the arguments it boxes are classified as obj_(x) / x.box_() of their static type (two
#    arguments of one T must have one static type), and where the tool's rewrite passes the second operand on boxed it is written
#    with the helper's boxing. A typed requireNonNull is its own unboxed value: as a statement or the value of an assignment it is
#    the inline nil check; elsewhere it stays (it is already what requireNonNull_ is). A call the tool does not decide is untouched.
eg=tools/postflip/eqhash/main.go
if [[ -f $eg ]] && ! grep -q 'reflip23' $eg && grep -q 'case "Objects_requireNonNull", "Objects_requireNonNull_Object":' $eg; then
  perl -pi -e 's/rw\.Render\(B\)/reflipBoxed_(B)/g; s/\brender\(B\)/func() string { return reflipBoxed_(B) }/g' $eg
  reflip_edit $eg reflip23 <<'E23'
func unbox(e ast.Expr) (boxed, bool) {
	e = rw.Unparen(e)
=====
func unbox(e ast.Expr) (boxed, bool) {
	e = rw.Unparen(e)
	if via, ok := reflipVia_[e]; ok { // reflip23: an argument a typed helper boxes itself
		tv, ok := rw.Info.Types[e]
		if !ok || tv.Type == nil {
			return boxed{}, false
		}
		if via == "strobj_" {
			return boxed{x: e, t: types.Typ[types.String]}, true
		}
		return classify(e, types.Unalias(tv.Type), via)
	}
-----
	k := ""
	name := fnName(c)
	switch name {
=====
	k := ""
	name := reflipName_(c, fnName(c))
	switch name {
-----
	if k != "" {
		counts[name+": "+k]++
	} else {
=====
	if o := fnName(c); o != name {
		name = o + " (" + name + ")"
	}
	if k != "" {
		counts[name+": "+k]++
	} else {
-----
		if k == "" {
			left[name+": "+why(c.Args[0])]++
			return true
		}
=====
		if k == "" {
			if o := fnName(c); o != name {
				name = o + " (" + name + ")"
			}
			left[name+": "+why(c.Args[0])]++
			return true
		}
-----
	if p, ok := rw.Up(c).(*ast.CallExpr); ok && len(p.Args) == 1 && rw.Unparen(p.Args[0]) == c {
=====
	if p, ok := rw.Up(c).(*ast.CallExpr); !reflipTypedRNN_[c] && ok && len(p.Args) == 1 && rw.Unparen(p.Args[0]) == c {
-----
	if top == ast.Expr(c) {
		return ""
	}
=====
	if top == ast.Expr(c) && !reflipTypedRNN_[c] {
		return ""
	}
-----
	if b, ok := ba.t.Underlying().(*types.Basic); ok && b.Kind() == types.String {
		return ""
	}
	x := ba.x
=====
	if b, ok := ba.t.Underlying().(*types.Basic); ok && b.Kind() == types.String {
		return ""
	}
	if reflipTypedRNN_[c] { // already the unboxed value
		return ""
	}
	x := ba.x
+++++

// reflipVia_ gives the boxing (obj_, box_, strobj_) that a typed helper of rt_conv.go applies to
// an argument it takes at its static type; reflipTypedRNN_ are the typed requireNonNull calls.
var reflipVia_ = map[ast.Expr]string{}
var reflipTypedRNN_ = map[*ast.CallExpr]bool{}

// reflipHelpers_: typed helper -> the erased function its body calls and the boxing of each
// argument ("" for an argument passed on as it is).
var reflipHelpers_ = map[string][]string{
	"equals_":                          {"objequals_", "obj_", "obj_"},
	"equalsH_":                         {"objequals_", "box_", "box_"},
	"hash_":                            {"objhash_", "obj_"},
	"hashH_":                           {"objhash_", "box_"},
	"equalsR_":                         {"Object_equals", "obj_", ""},
	"equalsRR_":                        {"Object_equals", "obj_", "obj_"},
	"equalsRH_":                        {"Object_equals", "box_", ""},
	"equalsRRH_":                       {"Object_equals", "box_", "box_"},
	"hashR_":                           {"Object_hashCode", "obj_"},
	"hashRH_":                          {"Object_hashCode", "box_"},
	"Objects_requireNonNull_T":         {"Objects_requireNonNull", "obj_", ""},
	"Objects_requireNonNull_Str":       {"Objects_requireNonNull", "strobj_", ""},
	"Objects_requireNonNull_Object_T":  {"Objects_requireNonNull_Object", "obj_"},
	"Objects_requireNonNull_Object_Str": {"Objects_requireNonNull_Object", "strobj_"},
}

// reflipName_ is the erased function that typed helper call c is, with the boxings of its
// arguments recorded; name itself for any other call.
func reflipName_(c *ast.CallExpr, name string) string {
	h, ok := reflipHelpers_[name]
	if !ok || len(c.Args) != len(h)-1 {
		return name
	}
	if _, isFunc := rw.Info.Uses[c.Fun.(*ast.Ident)].(*types.Func); !isFunc {
		return name
	}
	var first types.Type
	for i, via := range h[1:] {
		if via == "" {
			continue
		}
		tv, ok := rw.Info.Types[rw.Unparen(c.Args[i])]
		if !ok || tv.Type == nil || tv.IsNil() {
			return name
		}
		if first == nil {
			first = tv.Type
		} else if !types.Identical(first, tv.Type) {
			return name // one T for two static types: an implicit conversion the tool does not see
		}
	}
	for i, via := range h[1:] {
		if via != "" {
			reflipVia_[rw.Unparen(c.Args[i])] = via
		}
	}
	if strings.HasPrefix(name, "Objects_requireNonNull") {
		reflipTypedRNN_[c] = true
	}
	return h[0]
}

// reflipBoxed_ renders operand e as the box the erased call was given: with the helper's boxing
// when a typed helper took it at its static type.
func reflipBoxed_(e ast.Expr) string {
	switch reflipVia_[rw.Unparen(e)] {
	case "obj_":
		return "obj_(" + rw.Render(e) + ")"
	case "strobj_":
		return "strobj_(" + rw.Render(e) + ")"
	case "box_":
		switch rw.Unparen(e).(type) {
		case *ast.Ident, *ast.SelectorExpr, *ast.CallExpr, *ast.IndexExpr:
			return rw.Render(rw.Unparen(e)) + ".box_()"
		}
		return "(" + rw.Render(e) + ").box_()"
	}
	return rw.Render(e)
}
E23
fi
# 24. (fourth flip, in bounds) rw.Unboxed: "the code storing boxed expression a (of type any) as a value of type t" drops a boxing
#    call whose operand has type t (strobj_(x), obj_(x), ...); any other expression is unboxed by the type's conversion. The
#    translator's rule P4 passes a String where an Object is taken as the string itself (put("CLASS_DECL", v), the erased output had
#    strobj_("CLASS_DECL")), so the typed immutable builders of step 336 got their constant keys as str_("CLASS_DECL"): 236
#    str_( of a string (third flip: the same 236; lead line: 0). A value whose static type is already the string type t is its own
#    unboxing (str_ of a Go string in an `any` is the string).
reflip_edit tools/postflip/rw/rw.go reflip24 <<'E24'
func Unboxed(cv Conv, t types.Type, a ast.Expr) string {
	a = Unparen(a)
=====
func Unboxed(cv Conv, t types.Type, a ast.Expr) string {
	a = Unparen(a)
	if tv, ok := Info.Types[a]; ok && tv.Type != nil && types.Identical(tv.Type, t) { // reflip24: a String passed as itself
		if b, ok := t.Underlying().(*types.Basic); ok && b.Kind() == types.String {
			return Render(a)
		}
	}
E24
# 25. (fourth flip, in bounds) funcparams: fix 17 reads the object under the upcast written as a conversion, I(o). The same rule of
#    the translator writes the upcast of a value that may be null as the nil-keeping helper C_as_I(o) of g_0casts.go
#    (FunctionArgumentInjector_ReferencedAfterSideEffect_as_NodeUtil_Visitor(collector)); the tool took it for "another value" and
#    kept every parameter such a call reaches an interface (NodeUtil.Visitor: 13 lambda objects &NodeUtil_Visitor_fn{func...} the
#    lead line has as Go functions). The object of the helper is its operand; whether it is null is the tool's own question
#    (nonNullLocal and the rest), as for the erased icast_I(obj_(o)).
reflip_edit tools/postflip/funcparams/main.go reflip25 <<'E25'
	if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 { // reflip17: the upcast written out, I(x)
		if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() && types.IsInterface(tv.Type) {
			e = rw.Unparen(c.Args[0])
		}
	}
=====
	if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 { // reflip17: the upcast written out, I(x)
		if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() && types.IsInterface(tv.Type) {
			e = rw.Unparen(c.Args[0])
		} else if id, ok := c.Fun.(*ast.Ident); ok && strings.Contains(id.Name, "_as_") { // reflip25: the nil-keeping upcast helper C_as_I(x)
			if fn, ok := rw.Info.Uses[id].(*types.Func); ok && strings.HasSuffix(rw.Fset.Position(fn.Pos()).Filename, "g_0casts.go") {
				e = rw.Unparen(c.Args[0])
			}
		}
	}
E25
# 26. (fourth flip, in bounds) exceptions: the translator's typed collection runtime (rt_jtimm.go, until step 402 unifies the
#    families) panics with the value frozen_{} for a change to an immutable typed collection, and catch_ makes it Java's
#    UnsupportedOperationException (rt_lang.go). The tool took it for a value of unknown class that every caller of a typed
#    collection could throw and kept every catch clause such a call reaches ("it can throw a value of unknown class:
#    computeIfAbsent: panic(frozen_{})"). The class is known (the third flip's tool fix 9cddf058 on reflip2-bridge).
reflip_edit tools/postflip/exceptions/main.go reflip26 <<'E26'
	if c, ok := x.(*ast.CallExpr); ok {
		if id, ok := c.Fun.(*ast.Ident); ok {
			if strings.HasPrefix(id.Name, "new_") {
=====
	if cl, ok := x.(*ast.CompositeLit); ok && len(cl.Elts) == 0 && rw.Text(cl.Type) == "frozen_" { // reflip26
		return []string{"UnsupportedOperationException"}, true
	}
	if c, ok := x.(*ast.CallExpr); ok {
		if id, ok := c.Fun.(*ast.Ident); ok {
			if strings.HasPrefix(id.Name, "new_") {
E26
# 27. (fourth flip, in bounds) eqhash: Objects.requireNonNull(x, msg) of a typed collection is written by this translator with the
#    collection's erased view passed as it is, Objects_requireNonNull(names.asSet(), "names") (the erased output boxed the value:
#    any(names)); the tool reads a boxing call only and left these 27 (lead line: 0). An interface value passed as it is is null
#    exactly when it is the nil interface: as a statement the call is the inline check `if names.asSet() == nil { panic(NPE(msg)) }`
#    (the form the third flip has). Other uses of the result stay.
if [[ -f $eg ]] && ! grep -q 'reflip27' $eg && grep -q 'reflip23' $eg; then
  reflip_edit $eg reflip27 <<'E27'
	ba, ok := unbox(c.Args[0])
	if !ok {
		return ""
	}
	if b, isBasic := ba.t.(*types.Basic); isBasic && b.Kind() != types.String {
=====
	ba, ok := unbox(c.Args[0])
	if !ok { // reflip27: an interface value passed as it is, checked as a statement
		e := rw.Unparen(c.Args[0])
		tv := rw.Info.Types[e]
		if _, isStmt := rw.Up(c).(*ast.ExprStmt); !isStmt || tv.Type == nil || tv.IsNil() || !isIface(tv.Type) {
			return ""
		}
		ba = boxed{x: e, t: types.Unalias(tv.Type)}
	}
	if b, isBasic := ba.t.(*types.Basic); isBasic && b.Kind() != types.String {
E27
fi
# 28. (fifth flip) rw.ConvFor, colls, native: an element type that is one of the translator's function types (rule "function types":
#    a named Go func type with a box_ method, Function[any, any], Supplier[Node]). rw.ConvFor took such a type for a handle type,
#    because the stubs also declare a function Function_of_ (the conversion from a plain func; for a handle type X, X_of_ unboxes),
#    and wrote the unboxing Function_of_(v) of an `any`, which does not compile (first met at step 336: Node.PROP_MAP_FOR_EQUALITY,
#    an ImmutableMap<Prop, Function<Node, Object>>). A function's box is v.box_() and its unboxing is the runtime's fnUnbox_[F](v)
#    (rt_fn.go), as the translator writes it. A conversion object the tools declare for such a type gets isnull (a Go func compares
#    with nil only; tconv_.isNull's any(v) == any(c.null) would panic), as the translator's own conversions have it; the runtime
#    side is reflip5-fnconv-rt.patch.
reflip_edit tools/postflip/rw/rw.go reflip28 <<'E28'
		} else if HasMethod(u, "box_") && HasFunc(name+"_of_") {
=====
		} else if _, isFn := u.Underlying().(*types.Signature); isFn && HasMethod(u, "box_") && HasFunc("fnUnbox_") { // reflip28: a function type
			ts := types.TypeString(t, types.RelativeTo(Pkg))
			return Conv{func(e string) string { return e + ".box_()" }, func(e string) string { return "fnUnbox_[" + ts + "](" + e + ")" }, func(string) bool { return false }, true}
		} else if HasMethod(u, "box_") && HasFunc(name+"_of_") {
E28
reflip_edit tools/postflip/colls/main.go reflip28 <<'E28'
		if isString(t) {
			extra = ", null: nullstr_"
		}
		if hash {
=====
		if isString(t) {
			extra = ", null: nullstr_"
		}
		if _, isFn := t.Underlying().(*types.Signature); isFn { // reflip28: a function type is compared with nil only
			extra = fmt.Sprintf(", isnull: func(v %s) bool { return v == nil }", text)
		}
		if hash {
E28
reflip_edit tools/postflip/native/main.go reflip28 <<'E28'
		if isString(c.elem) {
			null = ", null: nullstr_"
		}
		immConvs[n] = fmt.Sprintf(
=====
		if isString(c.elem) {
			null = ", null: nullstr_"
		}
		if _, isFn := c.elem.Underlying().(*types.Signature); isFn { // reflip28: a function type is compared with nil only
			null = fmt.Sprintf(", isnull: func(v %s) bool { return v == nil }", c.elemText)
		}
		immConvs[n] = fmt.Sprintf(
E28
# 29. (fifth flip) unify: the translator's rule "function types" gives its typed map a typed Map.computeIfAbsent,
#    m.computeIfAbsent(k, Function[K, V](func(k K) V {..})) (rt_fn.go; 53 calls where the fourth flip had the erased
#    computeIfAbsent(k, &Function_tfn_K_V{..}) that steps 434-437 tfnargs then made ComputeIfAbsent). The post-flip family's method
#    of that name takes an erased function, and its method for a typed one is ComputeIfAbsent(k, func(K) V) (tnames): the
#    translator's call is that method, as containsKey is Has. A Function[K, V] is assignable to func(K) V. The runtime side (the
#    translator's method leaves rt_fn.go with its family; ComputeIfAbsent throws for an immutable map as it did) is in this flip's
#    402 runtime patch.
reflip_edit tools/postflip/unify/main.go reflip29 <<'E29'
		"putIfAbsent": "PutIfAbsent", "remove": "Delete", "clear": "Clear", "eachKey": "each", "eachValue": "each"},
=====
		"putIfAbsent": "PutIfAbsent", "remove": "Delete", "clear": "Clear", "eachKey": "each", "eachValue": "each",
		"computeIfAbsent": "ComputeIfAbsent"}, // reflip29
E29
# 30. (fifth flip) adapters: a typed lambda given to a library method is no adapter object &X_tfn_T{func..} on this translator
#    (rule "function types") but the function literal converted to the parameter's function type, Predicate[string](func(s string)
#    bool {..}). The tool's rule is about the literal: "the argument must be ... one function literal of the type named"; the
#    conversion of a function literal is that literal (never null, as the adapter literal was). Read so, Optional[T].filter of a
#    literal is filterFn_ again (102 calls: the form step 553 optstr reads; filter itself differs from filterFn_ only by the null
#    check of the predicate) and Map_computeIfAbsent of a func(any) any literal is Map_computeIfAbsent_f.
reflip_edit tools/postflip/adapters/main.go reflip30 <<'E30'
	funcOf := func(arg ast.Expr, erased bool) (*ast.FuncLit, string) {
		u, ok := arg.(*ast.UnaryExpr)
=====
	funcOf := func(arg ast.Expr, erased bool) (*ast.FuncLit, string) {
		if c, ok := arg.(*ast.CallExpr); ok && len(c.Args) == 1 { // reflip30: a function literal converted to a function type
			if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() {
				if _, isFn := tv.Type.Underlying().(*types.Signature); isFn {
					if fl, ok := rw.Unparen(c.Args[0]).(*ast.FuncLit); ok {
						return fl, ""
					}
				}
			}
		}
		u, ok := arg.(*ast.UnaryExpr)
E30
# 31. (fifth flip) optstr, call graph (cha.go): the translator's rule "function types" gives each function type the interface's
#    method as a forwarder, func (f Supplier[T]) get() T { return f() } (g_zstubs.go), and writes a call of a functional parameter
#    as that method, s.get(). The call graph took s.get() for a call of the generic method and its body's f() for a call of a func
#    value of signature func() T with T standing for any type: every func() X of the program, so that building an empty typed
#    collection (tchT_ -> single_ -> mk()) "reached" Callable.call and through it a lambda of Compiler.saveState that reads
#    Compiler.idGeneratorMap; the proof of that field failed (the fourth flip's step 553 proved it). A call x.m(..) of such a
#    forwarder runs the func value x and nothing else: it has the edges of the call x(..), at x's own type; and the method value
#    x.m is the value x, which is listed where it was made. Sound for the same closed world as before (a func value runs only
#    where a callee has func type); forwarders are recognised by their body, a single call of the receiver with the parameters.
#    (Only the call graph of step 553's tool, 48db3634, has the code this is for: objCallEdges.)
if [[ -f tools/postflip/optstr/cha.go ]] && grep -q 'objCallEdges(x, fn)' tools/postflip/optstr/cha.go; then
reflip_edit tools/postflip/optstr/cha.go reflip31 <<'E31'
				fd := funcDecls[fn.Origin()]
				if fd == nil || fd.Body == nil {
					return true
				}
				sig := fn.Type().(*types.Signature)
=====
				fd := funcDecls[fn.Origin()]
				if fd == nil || fd.Body == nil {
					return true
				}
				if fnForwarder_(fn) { // reflip31: the method value x.m of a function type is the value x
					return true
				}
				sig := fn.Type().(*types.Signature)
-----
			if fn != nil {
				if tgt, ok := objCallEdges(x, fn); ok {
=====
			if fn != nil {
				if fs, ok := fun.(*ast.SelectorExpr); ok && fnForwarder_(fn) { // reflip31: x.m(..) of a function type is x(..)
					if xs, ok := rw.Unparen(fs.X).(*ast.SelectorExpr); ok && fieldCalled(xs, root, &out) {
						return true // a func-typed field whose stores are all known (fieldfuncs.go)
					}
					if sig, ok := types.Unalias(rw.Info.TypeOf(fs.X)).Underlying().(*types.Signature); ok {
						out = append(out, valueCallees(sig, x)...)
						return true
					}
				}
				if tgt, ok := objCallEdges(x, fn); ok {
-----
	var out []ast.Node
	for _, v := range valueFuncs {
		for _, vs := range v.sigs() {
=====
	var out []ast.Node
	for _, v := range valueFuncs {
		if lit, ok := v.node.(*ast.FuncLit); ok && mentionsTP(v.sig) { // reflip31: a literal of generic code, per instantiation
			if tps, tuples, ok := contextOf(v.site); ok && len(tuples) > 0 {
				for _, tu := range tuples {
					m := map[*types.TypeParam]types.Type{}
					for i, tp := range tps {
						m[tp] = tu[i]
					}
					if sameSig(subst(v.sig, m).(*types.Signature), sig) {
						out = append(out, instLitOf_(lit, tps, m))
					}
				}
				continue
			}
		}
		for _, vs := range v.sigs() {
-----
	tps, tuples, ok := contextOf(site)
	if !ok || len(tuples) == 0 {
		return matching(sig)
	}
	seen := map[ast.Node]bool{}
=====
	tps, tuples, ok := contextOf(site)
	if !ok || len(tuples) == 0 {
		return matching(sig)
	}
	if curInst_ != nil { // reflip31: inside a literal reached at one instantiation of its generic code
		all := true
		for _, tp := range tps {
			if _, ok := curInst_[tp]; !ok {
				all = false
			}
		}
		if all {
			return matching(subst(sig, curInst_).(*types.Signature))
		}
	}
	seen := map[ast.Node]bool{}
-----
		chaDone[n] = true
		out, reads := edges(bodyOf(n), n)
=====
		chaDone[n] = true
		if il, ok := n.(*instLit_); ok { // reflip31
			curInst_ = il.m
		}
		out, reads := edges(bodyOf(n), n)
		curInst_ = nil
-----
	case *boundFn:
		return x.Body
	}
	return nil
=====
	case *boundFn:
		return x.Body
	case *instLit_: // reflip31
		return x.Body
	}
	return nil
+++++

// fnForwarder_ (reflip31) reports whether fn is a method of a named function type whose body only calls the receiver with
// the method's parameters: the methods the translator's rule "function types" gives its function types (f.apply(x) is f(x)).
func fnForwarder_(fn *types.Func) bool {
	fn = fn.Origin()
	sig, ok := fn.Type().(*types.Signature)
	if !ok || sig.Recv() == nil {
		return false
	}
	if _, ok := types.Unalias(sig.Recv().Type()).Underlying().(*types.Signature); !ok {
		return false
	}
	fd := funcDecls[fn]
	if fd == nil || fd.Body == nil || len(fd.Body.List) != 1 || fd.Recv == nil || len(fd.Recv.List) != 1 || len(fd.Recv.List[0].Names) != 1 {
		return false
	}
	var e ast.Expr
	switch s := fd.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(s.Results) != 1 {
			return false
		}
		e = s.Results[0]
	case *ast.ExprStmt:
		e = s.X
	default:
		return false
	}
	c, ok := e.(*ast.CallExpr)
	if !ok {
		return false
	}
	id, ok := c.Fun.(*ast.Ident)
	if !ok || id.Name != fd.Recv.List[0].Names[0].Name {
		return false
	}
	n := 0
	for _, p := range fd.Type.Params.List {
		for _, pn := range p.Names {
			if n >= len(c.Args) {
				return false
			}
			if a, ok := c.Args[n].(*ast.Ident); !ok || a.Name != pn.Name {
				return false
			}
			n++
		}
	}
	return n == len(c.Args)
}

// instLit_ (reflip31) is a function literal written in generic code, as reached at one instantiation of that code (the type
// arguments m): a call of a func value inside it whose type is written with the code's type parameters has that instantiation's
// signature, not every instantiation's. Suppliers.memoize's closure reached as a func() string calls its s as a func() string.
type instLit_ struct {
	*ast.FuncLit
	m map[*types.TypeParam]types.Type
}

var (
	instLits_ = map[string]*instLit_{}
	curInst_  map[*types.TypeParam]types.Type
)

func instLitOf_(lit *ast.FuncLit, tps []*types.TypeParam, m map[*types.TypeParam]types.Type) *instLit_ {
	key := fmt.Sprint(lit.Pos())
	for _, tp := range tps {
		key += "|" + types.TypeString(m[tp], nil)
	}
	if il, ok := instLits_[key]; ok {
		return il
	}
	il := &instLit_{FuncLit: lit, m: m}
	instLits_[key] = il
	return il
}
E31
fi
# 32. (fifth flip) colls: the translator's nil-keeping upcast helpers of g_0casts.go (ImmutableList_as_List(v *ErasedImmutableList)
#    List: rule "implicit conversions") are functions of generated code, so the tool made their parameter a slot like any other:
#    one class with every argument given to the helper anywhere, whatever its element type. On the fourth flip no slot of that
#    class stated an element type and it stayed erased. On this flip one declaration of the class states *CompilerInput, and step
#    336 typed the class: 35 constructors given to ImmutableList_as_List became immutableListOf_(tc_p_CompilerInput,
#    cast_[*CompilerInput](strobj_(..))), lists of strings and Nodes among them; the type check passes (cast_ of an `any`) and the
#    compiler failed at start (the first gate of this flip: ClassCastException in CommandLineConfig's initializer). A value passes
#    through such a helper unchanged whatever it holds: its signature is fixed, as the runtime's are.
reflip_edit tools/postflip/colls/main.go reflip32 <<'E32'
					declare(groupOf(o), x.Type, !generated(x.Pos()), "signature of the runtime")
=====
					if x.Recv == nil && strings.Contains(x.Name.Name, "_as_") && filepath.Base(rw.Fset.Position(x.Pos()).Filename) == "g_0casts.go" { // reflip32
						declare(groupOf(o), x.Type, true, "an upcast helper of the translator (values of every element type pass through it)")
					} else {
						declare(groupOf(o), x.Type, !generated(x.Pos()), "signature of the runtime")
					}
E32
# 33. (fifth flip, second part) rw.TypedFn, one reader for tfnapi, tcmp and tgeneric: the typed function of an argument at an erased
#    functional place. The tools read the typed lambda adapter &X_tfn_T{f} and gave f to the runtime's typed variant. Rule "function
#    types" of the translator writes no adapter object: the lambda is a function literal at the erased place's type that converts on
#    entry, X[any..](func(p_ any) { var p T = K(p_); .. }) or X[any..](func(p0 any) { m(K(p0)) }), and a function value given to an
#    erased place is the converting closure func() X[any] { f_ := E; if f_ == nil { return nil }; return func(p0 any) R { return
#    f_(K(p0)) } }(). The tools read neither, so no call reached forEachT_ / mergeT_ / computeT_, the typed sorts of comparator
#    chains or Compiler_runInCompilerThreadT_ (fourth flip: 8 / 3 / 2, 23, 6). The reader, its rules and its proof obligation
#    (K(box(x)) == x for the type's own conversion tconv_[T]{box, unbox}; E never null) are at the head of rw/typedfn.go below.
#    It answers for the adapter literal as the tools did, so the tools work on both forms (a later flip may change which one the
#    translator writes). Only the tool versions from tcmp's first step (472) on get the file.
if [[ -d tools/postflip/rw && ! -f tools/postflip/rw/typedfn.go ]] && [[ -d tools/postflip/tcmp || -d tools/postflip/tfnapi || -d tools/postflip/tgeneric ]]; then
cat > tools/postflip/rw/typedfn.go <<'E33'
package rw

// reflip33 (fifth flip): the typed function of an argument at an erased functional place.
//
// The post-flip tools tfnapi, tcmp and tgeneric give the runtime's typed variants the function f
// of a typed lambda adapter &X_tfn_T{f}. The translator's rule "function types" writes no
// adapter object: the same lambda is a function literal at the erased place's type that
// converts its arguments itself,
//
//	X[any, any](func(a_ any, b any) any { var a T = K(a_); _ = a; ..; return B(r) })
//	X[any](func(p0 any) { this.m(K(p0)) })
//
// and a function value given to an erased place is the translator's converting closure
//
//	func() X[any] { f_ := E; if f_ == nil { return nil }; return func(p0 any) R { return f_(K(p0)) } }()
//
// TypedFn reads the three forms (and a literal that already has the wanted signature, with or
// without a conversion to a function type) and gives the code of the function of signature
// want:
//   - the adapter's f, when f has the signature;
//   - E of a converting closure, when E has the signature, cannot be null (a function literal,
//     a method or function name, a conversion of one, or a call of a function all of whose
//     returns are such expressions: the runtime's Comparator combinators), and each argument of
//     f_ is the parameter at its position under the unboxing of its type;
//   - the literal with its parameters and result typed: a parameter p of type any whose wanted
//     type is T takes T when every use of p is the operand of an unboxing K(p) of T; K(p) is
//     then p (for `var v T = K(p)` at the head of the body: the parameter is v and the
//     statement, with its `_ = v`, goes). A result of type any whose wanted type is R is R
//     when every return of the literal returns B(e), B the boxing of R and e an R (then e), or
//     nil for a pointer or interface R, or a call of a parameterless literal whose returns
//     qualify (the translation of a conditional expression).
//
// Proof obligation. The typed variant calls the function with the collection's values where the
// erased API called it with their boxes, and reads its result where the erased API unboxed
// it. So for every x of T: K(box(x)) == x, and for every e of R: unbox(B(e)) == e, null
// included. K and B qualify when they are the unbox and box functions of the package's own
// conversions of that type, the tconv_[T]{box: B, unbox: K} values every typed collection of T
// is made with (all conversions of the type must agree), or the unboxing rw.ConvFor names for
// the type. With raw (the consumer is a plain cast_[R] of the result) a return may also give
// any(e) or e itself: cast_[R] of an interface value holding e is e.
//
// The function is robust to both forms: on a tree with adapter objects it answers as the
// tools' own readers did.

import (
	"go/ast"
	"go/token"
	"go/types"
	"strings"
)

type tfConv struct {
	unbox map[types.Object]bool // the functions the conversions of the type unbox with
	box   map[types.Object]bool // the functions or methods they box with
	raw   bool                  // a conversion boxes with func(v T) any { return v }
	mixed bool                  // a conversion whose box or unbox is not one of the forms above
	elem  types.Type
}

var tfConvs []*tfConv
var tfConvsDone bool

func tfBase(e ast.Expr) types.Object {
	switch f := Unparen(e).(type) {
	case *ast.Ident:
		return Info.Uses[f]
	case *ast.IndexExpr:
		return tfBase(f.X)
	case *ast.IndexListExpr:
		return tfBase(f.X)
	case *ast.SelectorExpr:
		return Info.Uses[f.Sel]
	}
	return nil
}

func tfIsAny(t types.Type) bool {
	if t == nil {
		return false
	}
	if _, ok := types.Unalias(t).(*types.TypeParam); ok {
		return false
	}
	it, ok := t.Underlying().(*types.Interface)
	return ok && it.Empty()
}

// tfTable lists the package's conversions: every composite literal tconv_[T]{..}.
func tfTable() {
	if tfConvsDone {
		return
	}
	tfConvsDone = true
	for _, f := range Files {
		ast.Inspect(f, func(n ast.Node) bool {
			cl, ok := n.(*ast.CompositeLit)
			if !ok {
				return true
			}
			nt, ok := types.Unalias(Info.TypeOf(cl)).(*types.Named)
			if !ok || nt.Origin().Obj().Name() != "tconv_" || nt.Obj().Pkg() != Pkg || nt.TypeArgs().Len() != 1 {
				return true
			}
			c := &tfConv{unbox: map[types.Object]bool{}, box: map[types.Object]bool{}, elem: nt.TypeArgs().At(0)}
			for _, el := range cl.Elts {
				kv, ok := el.(*ast.KeyValueExpr)
				if !ok {
					c.mixed = true
					continue
				}
				key, _ := kv.Key.(*ast.Ident)
				if key == nil || key.Name != "box" && key.Name != "unbox" {
					continue
				}
				v := Unparen(kv.Value)
				if fl, ok := v.(*ast.FuncLit); ok {
					// func(v any) T { return K(v) } / func(v T) any { return v } / { return B(v) }
					var ret ast.Expr
					if len(fl.Body.List) == 1 && fl.Type.Params.NumFields() == 1 && len(fl.Type.Params.List[0].Names) == 1 {
						if rs, ok := fl.Body.List[0].(*ast.ReturnStmt); ok && len(rs.Results) == 1 {
							ret = Unparen(rs.Results[0])
						}
					}
					p := ""
					if ret != nil {
						p = fl.Type.Params.List[0].Names[0].Name
					}
					if id, ok := ret.(*ast.Ident); ok && key.Name == "box" && id.Name == p {
						c.raw = true
						continue
					}
					if call, ok := ret.(*ast.CallExpr); ok && len(call.Args) == 1 {
						if id, ok := Unparen(call.Args[0]).(*ast.Ident); ok && id.Name == p {
							if o := tfBase(call.Fun); o != nil {
								if key.Name == "box" {
									c.box[o] = true
								} else {
									c.unbox[o] = true
								}
								continue
							}
						}
					}
					c.mixed = true
					continue
				}
				o := tfBase(v)
				if _, isFn := o.(*types.Func); !isFn {
					c.mixed = true
					continue
				}
				if key.Name == "box" {
					c.box[o] = true
				} else {
					c.unbox[o] = true
				}
			}
			tfConvs = append(tfConvs, c)
			return true
		})
	}
}

// tfSame: the conversion's element type is t (for a conversion made by a generic function: the
// same generic type at the function's own type parameters).
func tfSame(elem, t types.Type) bool {
	if types.Identical(elem, t) {
		return true
	}
	a, b := types.Unalias(elem), types.Unalias(t)
	if pa, ok := a.(*types.Pointer); ok {
		pb, ok := b.(*types.Pointer)
		if !ok {
			return false
		}
		a, b = types.Unalias(pa.Elem()), types.Unalias(pb.Elem())
	}
	na, ok1 := a.(*types.Named)
	nb, ok2 := b.(*types.Named)
	if !ok1 || !ok2 || na.Origin() != nb.Origin() || na.TypeArgs().Len() == 0 || na.TypeArgs().Len() != nb.TypeArgs().Len() {
		return false
	}
	seen := map[types.Type]bool{}
	for i := 0; i < na.TypeArgs().Len(); i++ {
		tp, ok := types.Unalias(na.TypeArgs().At(i)).(*types.TypeParam)
		if !ok || seen[tp] {
			return false
		}
		seen[tp] = true
	}
	return true
}

// tfConvOf: what the conversions of t agree on (nil: none, or they differ).
func tfConvOf(t types.Type) *tfConv {
	tfTable()
	var all *tfConv
	for _, c := range tfConvs {
		if !tfSame(c.elem, t) {
			continue
		}
		if c.mixed {
			return nil
		}
		if all == nil {
			all = c
			continue
		}
		if c.raw != all.raw || len(c.box) != len(all.box) || len(c.unbox) != len(all.unbox) {
			return nil
		}
		for o := range c.box {
			if !all.box[o] {
				return nil
			}
		}
		for o := range c.unbox {
			if !all.unbox[o] {
				return nil
			}
		}
	}
	return all
}

// IsUnboxOf reports whether k is K(x) with K the unboxing of t.
func IsUnboxOf(k *ast.CallExpr, t types.Type) bool {
	if len(k.Args) != 1 || !types.Identical(Info.TypeOf(k), t) || !tfIsAny(Info.TypeOf(k.Args[0])) {
		return false
	}
	if tv, ok := Info.Types[k.Fun]; ok && tv.IsType() {
		return false
	}
	if c := tfConvOf(t); c != nil && len(c.unbox) == 1 {
		if o := tfBase(k.Fun); o != nil && c.unbox[o] {
			return true
		}
	}
	cv := ConvFor(t)
	name, _ := CallName(k)
	return cv.OK && name != "" && cv.IsUnbox(name)
}

// boxedAs returns e's operand when e is B(x) / x.B() with B the boxing of t and x a t.
func tfBoxed(e ast.Expr, t types.Type, raw bool) (ast.Expr, bool) {
	e = Unparen(e)
	if raw {
		if tv, ok := Info.Types[e]; ok && tv.Type != nil && !tv.IsNil() && types.Identical(tv.Type, t) {
			return e, true
		}
	}
	call, ok := e.(*ast.CallExpr)
	if !ok {
		return nil, false
	}
	var inner ast.Expr
	var fn types.Object
	isAnyConv := false
	if tv, ok := Info.Types[call.Fun]; ok && tv.IsType() {
		if !tfIsAny(tv.Type) || len(call.Args) != 1 {
			return nil, false
		}
		inner, isAnyConv = call.Args[0], true
	} else if s, ok := call.Fun.(*ast.SelectorExpr); ok && len(call.Args) == 0 && Info.Selections[s] != nil {
		inner, fn = s.X, Info.Uses[s.Sel]
	} else if len(call.Args) == 1 {
		inner, fn = call.Args[0], tfBase(call.Fun)
	}
	if inner == nil || !types.Identical(Info.TypeOf(inner), t) {
		return nil, false
	}
	c := tfConvOf(t)
	if isAnyConv {
		return inner, raw || c != nil && c.raw && len(c.box) == 0
	}
	if c != nil && len(c.box) == 1 && !c.raw && fn != nil && c.box[fn] {
		return inner, true
	}
	if c == nil {
		// no conversion of the type in the package: the boxing rw.Unboxed reads
		cv := ConvFor(t)
		if id, ok := call.Fun.(*ast.Ident); ok && cv.OK && (id.Name == "obj_" || id.Name == "strobj_") {
			return inner, true
		}
		if s, ok := call.Fun.(*ast.SelectorExpr); ok && cv.OK && (s.Sel.Name == "self" || s.Sel.Name == "box_") {
			return inner, true
		}
	}
	return nil, false
}

func tfSig(e ast.Expr) *types.Signature {
	t := Info.TypeOf(e)
	if t == nil {
		return nil
	}
	s, _ := types.Unalias(t).Underlying().(*types.Signature)
	return s
}

func tfSameSig(a, b *types.Signature) bool {
	if a == nil || b == nil || a.Variadic() || b.Variadic() || a.TypeParams().Len() > 0 {
		return false
	}
	return types.Identical(types.NewSignatureType(nil, nil, nil, a.Params(), a.Results(), false), types.NewSignatureType(nil, nil, nil, b.Params(), b.Results(), false))
}

func tfTypeText(t types.Type) string {
	return types.TypeString(t, func(p *types.Package) string {
		if p == Pkg {
			return ""
		}
		return p.Name()
	})
}

var tfDecls map[types.Object]*ast.FuncDecl
var tfNever = map[types.Object]int{} // 1: being decided, 2: never returns nil, 3: may

// NonNullFn reports whether the function-typed expression e cannot be nil.
func NonNullFn(e ast.Expr) bool {
	e = Unparen(e)
	switch x := e.(type) {
	case *ast.FuncLit:
		return true
	case *ast.Ident:
		_, ok := Info.Uses[x].(*types.Func)
		return ok
	case *ast.SelectorExpr:
		_, ok := Info.Uses[x.Sel].(*types.Func)
		return ok
	case *ast.IndexExpr, *ast.IndexListExpr:
		_, ok := tfBase(e).(*types.Func)
		return ok
	case *ast.CallExpr:
		if tv, ok := Info.Types[x.Fun]; ok && tv.IsType() {
			return len(x.Args) == 1 && NonNullFn(x.Args[0])
		}
		fn, ok := tfBase(x.Fun).(*types.Func)
		if !ok {
			return false
		}
		if tfDecls == nil {
			tfDecls = map[types.Object]*ast.FuncDecl{}
			for _, f := range Files {
				for _, d := range f.Decls {
					if fd, ok := d.(*ast.FuncDecl); ok && fd.Body != nil {
						tfDecls[Info.Defs[fd.Name]] = fd
					}
				}
			}
		}
		fn = fn.Origin()
		switch tfNever[fn] {
		case 1, 3:
			return false
		case 2:
			return true
		}
		tfNever[fn] = 1
		fd := tfDecls[fn]
		ok = fd != nil && fd.Type.Results != nil && fd.Type.Results.NumFields() == 1 && len(fd.Type.Results.List[0].Names) == 0
		n := 0
		if ok {
			for _, rs := range tfReturns(fd.Body) {
				n++
				if len(rs.Results) != 1 || !NonNullFn(rs.Results[0]) {
					ok = false
				}
			}
		}
		ok = ok && n > 0
		tfNever[fn] = 3
		if ok {
			tfNever[fn] = 2
		}
		return ok
	}
	return false
}

// tfReturns: the return statements of a function body (not those of literals inside it).
func tfReturns(b *ast.BlockStmt) []*ast.ReturnStmt {
	var out []*ast.ReturnStmt
	ast.Inspect(b, func(n ast.Node) bool {
		switch x := n.(type) {
		case *ast.FuncLit:
			return false
		case *ast.ReturnStmt:
			out = append(out, x)
		}
		return true
	})
	return out
}

func tfAdapter(e ast.Expr) ast.Expr {
	u, ok := Unparen(e).(*ast.UnaryExpr)
	if !ok || u.Op != token.AND {
		return nil
	}
	cl, ok := u.X.(*ast.CompositeLit)
	if !ok || len(cl.Elts) != 1 {
		return nil
	}
	var name string
	switch t := cl.Type.(type) {
	case *ast.Ident:
		name = t.Name
	case *ast.IndexExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			name = id.Name
		}
	case *ast.IndexListExpr:
		if id, ok := t.X.(*ast.Ident); ok {
			name = id.Name
		}
	}
	if !strings.Contains(name, "_tfn") {
		return nil
	}
	if _, ok := cl.Elts[0].(*ast.KeyValueExpr); ok {
		return nil
	}
	return cl.Elts[0]
}

// tfClosure returns E of the translator's converting closure when it stands for a function of
// signature want.
func tfClosure(e ast.Expr, want *types.Signature) ast.Expr {
	E, _ := tfClosureX(e, want, IsUnboxOf, true)
	return E
}

// ClosureOf returns E and the calls K(p) of the translator's converting closure over a function
// value E (which may be null: the closure is then null), each K accepted by okK for the type of
// E's parameter at its position.
func ClosureOf(e ast.Expr, okK func(k *ast.CallExpr, t types.Type) bool) (ast.Expr, []*ast.CallExpr) {
	return tfClosureX(e, nil, okK, false)
}

func tfClosureX(e ast.Expr, want *types.Signature, okK func(k *ast.CallExpr, t types.Type) bool, nonNull bool) (ast.Expr, []*ast.CallExpr) {
	E, ks := tfClosure1(e, want, okK, nonNull)
	if E == nil {
		return nil, nil
	}
	return E, ks
}

func tfClosure1(e ast.Expr, want *types.Signature, okK func(k *ast.CallExpr, t types.Type) bool, nonNull bool) (ast.Expr, []*ast.CallExpr) {
	var ks []*ast.CallExpr
	call, ok := Unparen(e).(*ast.CallExpr)
	if !ok || len(call.Args) != 0 {
		return nil, nil
	}
	fl, ok := call.Fun.(*ast.FuncLit)
	if !ok || fl.Type.Params.NumFields() != 0 || len(fl.Body.List) != 3 {
		return nil, nil
	}
	as, ok := fl.Body.List[0].(*ast.AssignStmt)
	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 1 || len(as.Rhs) != 1 {
		return nil, nil
	}
	fid, ok := as.Lhs[0].(*ast.Ident)
	if !ok {
		return nil, nil
	}
	fobj := Info.Defs[fid]
	is, ok := fl.Body.List[1].(*ast.IfStmt)
	if !ok || is.Init != nil || is.Else != nil || len(is.Body.List) != 1 {
		return nil, nil
	}
	if be, ok := Unparen(is.Cond).(*ast.BinaryExpr); !ok || be.Op != token.EQL || Text(be.X) != fid.Name || Text(be.Y) != "nil" {
		return nil, nil
	}
	if rs, ok := is.Body.List[0].(*ast.ReturnStmt); !ok || len(rs.Results) != 1 || Text(rs.Results[0]) != "nil" {
		return nil, nil
	}
	rs, ok := fl.Body.List[2].(*ast.ReturnStmt)
	if !ok || len(rs.Results) != 1 {
		return nil, nil
	}
	g, ok := Unparen(rs.Results[0]).(*ast.FuncLit)
	if !ok || len(g.Body.List) != 1 {
		return nil, nil
	}
	E := as.Rhs[0]
	if want == nil {
		want = tfSig(E)
		if want == nil || want.Variadic() {
			return nil, nil
		}
	}
	if !tfSameSig(tfSig(E), want) || nonNull && !NonNullFn(E) {
		return nil, nil
	}
	gs := tfSig(g)
	if gs == nil || gs.Params().Len() != want.Params().Len() || gs.Results().Len() != want.Results().Len() {
		return nil, nil
	}
	var inner ast.Expr
	switch st := g.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(st.Results) != 1 || want.Results().Len() != 1 {
			return nil, nil
		}
		inner = Unparen(st.Results[0])
		if !types.Identical(gs.Results().At(0).Type(), want.Results().At(0).Type()) {
			x, ok := tfBoxed(inner, want.Results().At(0).Type(), false)
			if !ok || !tfIsAny(gs.Results().At(0).Type()) {
				return nil, nil
			}
			inner = Unparen(x)
		}
	case *ast.ExprStmt:
		if want.Results().Len() != 0 {
			return nil, nil
		}
		inner = Unparen(st.X)
	default:
		return nil, nil
	}
	fc, ok := inner.(*ast.CallExpr)
	if !ok || len(fc.Args) != want.Params().Len() {
		return nil, nil
	}
	if id, ok := Unparen(fc.Fun).(*ast.Ident); !ok || Info.Uses[id] != fobj {
		return nil, nil
	}
	if len(UsesOf[fobj]) != 2 {
		return nil, nil
	}
	var params []types.Object
	for _, f := range g.Type.Params.List {
		if len(f.Names) == 0 {
			return nil, nil
		}
		for _, n := range f.Names {
			params = append(params, Info.Defs[n])
		}
	}
	for i, a := range fc.Args {
		a = Unparen(a)
		t := want.Params().At(i).Type()
		if types.Identical(gs.Params().At(i).Type(), t) {
			if id, ok := a.(*ast.Ident); !ok || Info.Uses[id] != params[i] {
				return nil, nil
			}
			continue
		}
		k, ok := a.(*ast.CallExpr)
		if !ok || len(k.Args) != 1 || !types.Identical(Info.TypeOf(k), t) || !okK(k, t) {
			return nil, nil
		}
		ks = append(ks, k)
		if id, ok := Unparen(k.Args[0]).(*ast.Ident); !ok || Info.Uses[id] != params[i] {
			return nil, nil
		}
	}
	return E, ks
}

type tfEdits map[ast.Node]func() string

func tfLine(p token.Pos) int { return Fset.Position(p).Line }

// tfRetype collects the edits that give lit the signature want.
func tfRetype(lit *ast.FuncLit, want *types.Signature, raw bool, ed tfEdits) bool {
	got := tfSig(lit)
	if got == nil || got.Variadic() || got.Params().Len() != want.Params().Len() || got.Results().Len() != want.Results().Len() {
		return false
	}
	type entry struct {
		stmt *ast.DeclStmt
		name *ast.Ident
		v    *ast.Ident
		typ  types.Type
		k    *ast.CallExpr
		fld  *ast.Field
	}
	var entries []entry
	i := 0
	for _, fld := range lit.Type.Params.List {
		n := len(fld.Names)
		if n == 0 {
			n = 1
		}
		for j := 0; j < n; j++ {
			t := want.Params().At(i).Type()
			pt := got.Params().At(i).Type()
			i++
			if types.Identical(pt, t) {
				continue
			}
			if !tfIsAny(pt) || len(fld.Names) > 1 {
				return false
			}
			txt := tfTypeText(t)
			ed[fld.Type] = func() string { return txt }
			if len(fld.Names) == 0 || fld.Names[0].Name == "_" {
				continue
			}
			name := fld.Names[0]
			uses := UsesOf[Info.Defs[name]]
			for _, u := range uses {
				k, ok := Up(u).(*ast.CallExpr)
				if !ok || len(k.Args) != 1 || Unparen(k.Args[0]) != ast.Expr(u) || !IsUnboxOf(k, t) {
					return false
				}
				pn := name.Name
				ed[k] = func() string { return pn }
			}
			if len(uses) == 1 {
				k := Up(uses[0]).(*ast.CallExpr)
				if vs, ok := Up(k).(*ast.ValueSpec); ok && len(vs.Names) == 1 && len(vs.Values) == 1 && vs.Names[0].Name != "_" {
					if gd, ok := Parent[vs].(*ast.GenDecl); ok && len(gd.Specs) == 1 {
						if ds, ok := Parent[gd].(*ast.DeclStmt); ok && Parent[ds] == ast.Node(lit.Body) {
							if o := Info.Defs[vs.Names[0]]; o != nil && types.Identical(o.Type(), t) {
								entries = append(entries, entry{ds, name, vs.Names[0], t, k, fld})
							}
						}
					}
				}
			}
		}
	}
	// `var v T = K(p)` at the head of the body: the parameter is v. The statements must be the
	// body's first (with their `_ = v`), each on lines of its own.
	isEntry := map[ast.Stmt]*entry{}
	for i := range entries {
		isEntry[entries[i].stmt] = &entries[i]
	}
	blank := func(s ast.Stmt) string {
		as, ok := s.(*ast.AssignStmt)
		if !ok || as.Tok != token.ASSIGN || len(as.Lhs) != 1 || len(as.Rhs) != 1 || Text(as.Lhs[0]) != "_" {
			return ""
		}
		if id, ok := as.Rhs[0].(*ast.Ident); ok {
			return id.Name
		}
		return ""
	}
	ownLines := func(idx int) bool {
		l := lit.Body.List
		before, after := tfLine(lit.Body.Lbrace), tfLine(lit.Body.Rbrace)
		if idx > 0 {
			before = tfLine(l[idx-1].End())
		}
		if idx+1 < len(l) {
			after = tfLine(l[idx+1].Pos())
		}
		return before < tfLine(l[idx].Pos()) && tfLine(l[idx].End()) < after
	}
	declared := map[string]bool{}
	for idx, s := range lit.Body.List {
		if e := isEntry[s]; e != nil && ownLines(idx) {
			declared[e.v.Name] = true
			v := e.v.Name
			delete(ed, e.k)
			ed[e.name] = func() string { return v }
			ed[s] = func() string { return Del }
			continue
		}
		if b := blank(s); b != "" && declared[b] && ownLines(idx) {
			ed[s] = func() string { return Del }
			continue
		}
		break
	}
	if want.Results().Len() == 1 && !types.Identical(got.Results().At(0).Type(), want.Results().At(0).Type()) {
		return tfResult(lit, want.Results().At(0).Type(), raw, ed)
	}
	return true
}

// tfResult: lit's result (any) becomes r.
func tfResult(lit *ast.FuncLit, r types.Type, raw bool, ed tfEdits) bool {
	if lit.Type.Results == nil || len(lit.Type.Results.List) != 1 {
		return false
	}
	fld := lit.Type.Results.List[0]
	if !tfIsAny(Info.TypeOf(fld.Type)) || len(fld.Names) > 1 {
		return false
	}
	if len(fld.Names) == 1 && len(UsesOf[Info.Defs[fld.Names[0]]]) > 0 {
		return false
	}
	txt := tfTypeText(r)
	ed[fld.Type] = func() string { return txt }
	rets := tfReturns(lit.Body)
	if len(rets) == 0 {
		return false
	}
	for _, rs := range rets {
		if len(rs.Results) != 1 {
			return false
		}
		e := Unparen(rs.Results[0])
		if tv, ok := Info.Types[e]; ok && tv.IsNil() {
			switch r.Underlying().(type) {
			case *types.Pointer, *types.Interface:
				continue
			case *types.Slice, *types.Map:
				if raw {
					continue
				}
			}
			return false
		}
		if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 0 {
			if fl, ok := c.Fun.(*ast.FuncLit); ok && fl.Type.Params.NumFields() == 0 {
				if s := tfSig(fl); s != nil && s.Results().Len() == 1 && tfIsAny(s.Results().At(0).Type()) {
					if !tfResult(fl, r, raw, ed) {
						return false
					}
					continue
				}
			}
		}
		inner, ok := tfBoxed(e, r, raw)
		if !ok {
			return false
		}
		if inner != e {
			in, at := inner, rs.Results[0]
			ed[at] = func() string { return Render(in) }
		}
	}
	return true
}

// TypedFn: the code of the function of signature want that arg stands for (see the head of the
// file); false when arg has none of the forms.
func TypedFn(arg ast.Expr, want *types.Signature, raw bool) (func() string, bool) {
	arg = Unparen(arg)
	if f := tfAdapter(arg); f != nil {
		if !tfSameSig(tfSig(f), want) {
			return nil, false
		}
		return func() string { return Render(f) }, true
	}
	if E := tfClosure(arg, want); E != nil {
		return func() string { return Render(E) }, true
	}
	if c, ok := arg.(*ast.CallExpr); ok && len(c.Args) == 1 {
		if tv, ok := Info.Types[c.Fun]; ok && tv.IsType() {
			if _, isFn := tv.Type.Underlying().(*types.Signature); isFn {
				arg = Unparen(c.Args[0])
			}
		}
	}
	lit, ok := arg.(*ast.FuncLit)
	if !ok {
		return nil, false
	}
	if tfSameSig(tfSig(lit), want) {
		return func() string { return Render(lit) }, true
	}
	ed := tfEdits{}
	if !tfRetype(lit, want, raw, ed) {
		return nil, false
	}
	return func() string {
		for n, g := range ed {
			if _, dup := Rewrite[n]; dup {
				panic("TypedFn: a node of the literal is rewritten twice: " + Text(n))
			}
			Rewrite[n] = g
		}
		s := Render(lit)
		for n := range ed {
			delete(Rewrite, n)
		}
		return s
	}, true
}
E33
fi
#    tfnapi: fnOf is the reader.
tf=tools/postflip/tfnapi/main.go
if [[ -f $tf ]] && ! grep -q reflip33 $tf; then
  python3 - "$tf" <<'P33'
import sys
p = sys.argv[1]; s = open(p).read()
a, b = s.index('// fnOf returns f when'), s.index('// listOf returns L')
new = """// fnOf returns the code of the typed function a stands for (reflip33: rw.TypedFn reads the adapter
// literal, the translator's function literal at the erased type and its converting closure).
func fnOf(a ast.Expr, want *types.Signature) func() string {
	f, ok := rw.TypedFn(a, want, false)
	if !ok {
		return nil
	}
	return f
}

"""
if s.count('rw.Render(f)') != 6:
    sys.exit("toolfix: tfnapi: rw.Render(f) found %d times" % s.count('rw.Render(f)'))
s = (s[:a] + new + s[b:]).replace('rw.Render(f)', 'f()')
open(p, 'w').write(s)
P33
fi
#    tcmp: a comparator that is none of the tool's own forms is asked of the reader (the converting closure of a chain of the
#    translator runtime's typed Comparator combinators, which never return null: every return of rt_fn.go's combinators is a
#    function literal or such a call; a literal typed on entry).
reflip_edit tools/postflip/tcmp/main.go reflip33 <<'E33b'
	want := cmpSig(t)
	// &X{f}: a typed lambda adapter
=====
	want := cmpSig(t)
	if g, ok := rw.TypedFn(e, want, false); ok { // reflip33
		return g()
	}
	// &X{f}: a typed lambda adapter
E33b
#    tgeneric: K(c.runInCompilerThread(X)) with X any of the reader's forms of a func() T, T the type K unboxes to. With K a plain
#    cast_[T] a return of the callable may give the T itself (cast_[T] of an interface value holding e is e).
reflip_edit tools/postflip/tgeneric/main.go reflip33 <<'E33c'
	mv, ok := rw.Unparen(c.Args[0]).(*ast.SelectorExpr)
	if !ok || mv.Sel.Name != "call" {
=====
	if p, ok := rw.Up(c).(*ast.CallExpr); ok && len(p.Args) == 1 && rw.Unparen(p.Args[0]) == ast.Expr(c) { // reflip33
		if tv, isT := rw.Info.Types[p.Fun]; !(isT && tv.IsType()) && rw.Info.TypeOf(p) != nil {
			rt := rw.Info.TypeOf(p)
			pname, _ := rw.CallName(p)
			want := types.NewSignatureType(nil, nil, nil, nil, types.NewTuple(types.NewParam(0, nil, "", rt)), false)
			if g, ok := rw.TypedFn(c.Args[0], want, strings.HasPrefix(pname, "cast_[")); ok {
				x := s.X
				if e, ok := read(rt, c, func() string { return "Compiler_runInCompilerThreadT_(" + rw.Render(x) + ", " + g() + ")" }); ok && e.Node == ast.Node(p) {
					rw.Rewrite[e.Node] = e.Gen
					done["Compiler.runInCompilerThread of a typed callable, cast back: Compiler_runInCompilerThreadT_"]++
					return
				}
			}
		}
	}
	mv, ok := rw.Unparen(c.Args[0]).(*ast.SelectorExpr)
	if !ok || mv.Sel.Name != "call" {
E33c
# 34. (fifth flip, second part) native: a typed map's m.ComputeIfAbsent(k, f) is a map operation. On the fourth flip the call a
#    native map kind met was the erased m.computeIfAbsent(k, &Function_tfn_K_V{f}), which it wrote as mapComputeIfAbsent_ and
#    step 486 tfnapi as mapComputeIfAbsentT_(vc, m, k, f): 20 calls, 20 Go maps. On this flip the translator's typed
#    computeIfAbsent is TMap.ComputeIfAbsent(k, func(K) V) from step 402 on (toolfix 29), which native took for an unknown
#    method: those maps stayed TMaps. ComputeIfAbsent and mapComputeIfAbsentT_ are the same function of the map's contents: the
#    value of k if there is one that is not null, else f(k), stored unless null (rt_tmap.go, rt_tfnapi.go); it does not depend on
#    the order. A conversion of a function literal to a function type is the literal. The rule applies when the runtime has
#    mapComputeIfAbsentT_ (this flip's reflip5-ciat-rt.patch at step 403; patch 485 has the rest of rt_tfnapi.go).
reflip_edit tools/postflip/native/main.go reflip34 <<'E34'
	case !set && (m == "computeIfAbsent" || m == "computeIfAbsent_f") && len(args) == 2 && kc != "":
=====
	case !set && m == "ComputeIfAbsent" && len(args) == 2 && rw.HasFunc("mapComputeIfAbsentT_"): // reflip34
		changes()
		fn := args[1]
		if c, ok := rw.Unparen(fn).(*ast.CallExpr); ok && len(c.Args) == 1 {
			if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() {
				if fl, ok := rw.Unparen(c.Args[0]).(*ast.FuncLit); ok {
					fn = fl
				}
			}
		}
		edit(n, call, func() string {
			return "mapComputeIfAbsentT_(" + vc + ", " + R() + ", " + A(0) + ", " + rw.Render(fn) + ")"
		})
	case !set && (m == "computeIfAbsent" || m == "computeIfAbsent_f") && len(args) == 2 && kc != "":
E34
# 35. (fifth flip, second part) tcmp: a PriorityQueue made with the converting closure of a typed comparator keeps its elements
#    unboxed (the data-flow work queue; the fourth flip's steps 105, 147 and the manual 419 gave that queue an int-key comparator
#    object the heap called directly; a Comparator is a Go func on this flip and the heap called the converting closure, two
#    junboxT_ and the comparator's own closures per comparison).
#      new_PriorityQueue(func() Comparator[any] { f_ := E; if f_ == nil { return nil }; return func(p0 any, p1 any) int32 { return f_(K(p0), K(p1)) } }())
#        ->  newPriorityQueueT_(E, K)                     (also new_PriorityQueue_int_Comparator(n, ..) with a constant n)
#    The runtime function (rt_pqt.go, step 559's patch) makes the same queue with the same erased comparator and keeps K(x)
#    beside each element once computed. Proof obligation: K is a pure cast, the same result or the same exception for the same
#    interface value and no effect, so that a repeated K(x) can be skipped: K is one of the cast and unboxing functions by name
#    (cast_, junboxT_, str_, to_X, icast_X, gcast_X, X_of_: a list of names) with the signature func(any) T, the same K for both
#    arguments, T the comparator's parameter type. E is evaluated once where the closure evaluated it; a null E gives the
#    natural-order queue in the runtime function, as the null closure did. The heap's part of the proof (same comparisons in the
#    same order, K first evaluated at the same call) is at the head of rt_pqt.go and in its test. Applies when the runtime has
#    newPriorityQueueT_.
reflip_edit tools/postflip/tcmp/main.go reflip35 <<'E35'
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			name, _ := rw.CallName(c)
=====
			c, ok := n.(*ast.CallExpr)
			if !ok {
				return true
			}
			if pqueue35(c) { // reflip35
				return true
			}
			name, _ := rw.CallName(c)
+++++

// pqueue35 (reflip35): new_PriorityQueue(C), C the converting closure of a typed comparator E with the pure cast K, is
// newPriorityQueueT_(E, K).
func pqueue35(c *ast.CallExpr) bool {
	name, _ := rw.CallName(c)
	var arg ast.Expr
	switch {
	case name == "new_PriorityQueue" && len(c.Args) == 1:
		arg = c.Args[0]
	case name == "new_PriorityQueue_int_Comparator" && len(c.Args) == 2 && rw.Info.Types[c.Args[0]].Value != nil:
		arg = c.Args[1]
	default:
		return false
	}
	if !rw.HasFunc("newPriorityQueueT_") {
		return false
	}
	if id, ok := rw.Unparen(arg).(*ast.Ident); ok && id.Name == "nil" {
		return false
	}
	pure := func(k *ast.CallExpr, t types.Type) bool {
		var id *ast.Ident
		switch f := k.Fun.(type) {
		case *ast.Ident:
			id = f
		case *ast.IndexExpr:
			id, _ = f.X.(*ast.Ident)
		case *ast.IndexListExpr:
			id, _ = f.X.(*ast.Ident)
		}
		if id == nil {
			return false
		}
		if fn, ok := rw.Info.Uses[id].(*types.Func); !ok || fn.Pkg() != rw.Pkg || fn.Parent() != rw.Pkg.Scope() {
			return false
		}
		sig, ok := rw.Info.TypeOf(k.Fun).(*types.Signature)
		if !ok || sig.Params().Len() != 1 || sig.Results().Len() != 1 || !types.Identical(sig.Results().At(0).Type(), t) {
			return false
		}
		if it, ok := sig.Params().At(0).Type().Underlying().(*types.Interface); !ok || !it.Empty() {
			return false
		}
		n := id.Name
		return n == "cast_" || n == "junboxT_" || n == "str_" || strings.HasPrefix(n, "to_") || strings.HasPrefix(n, "icast_") || strings.HasPrefix(n, "gcast_") || strings.HasSuffix(n, "_of_")
	}
	E, ks := rw.ClosureOf(arg, pure)
	if E == nil || len(ks) != 2 || rw.Text(ks[0].Fun) != rw.Text(ks[1].Fun) {
		left("a PriorityQueue whose comparator is not the converting closure of a typed comparator with a pure cast", c)
		return false
	}
	k := ks[0].Fun
	rw.Rewrite[c] = func() string { return "newPriorityQueueT_(" + rw.Render(E) + ", " + rw.Text(k) + ")" }
	counts["PriorityQueues made with the converting closure of a typed comparator keep their elements unboxed"]++
	return true
}
E35
# 36. (fifth flip, second part) the instanceof tests of generic classes: tswitch reads a disjunction of tests, and a new tool
#    isanames names the marker interfaces. The translator writes `v instanceof X` of a generic class as the assertion
#    v.(interface{ isa_X_() }) (in place, or in a helper is_X): the Simple* collections are generic from the translator on this
#    flip (prep 19) and their instanceof tests reach the end of the list as 4 helpers, 3 calls of them in one condition and 2
#    assertions in place (census: anonymous interface assertions 0 -> 6, is_X 71 -> 78). (a) tswitch: a link of an if / else-if
#    chain may be `if is_A(S) || is_B(S) ..`, instanceof tests of the chain's value alone; it is the case `case A, B:` (taken
#    when one assertion succeeds, as the disjunction is true; no name bound). (b) isanames (written here; its rule and why the
#    named interface is the same test are at the head of the file): interface{ isa_X_() } is X_isa, declared where X's marker
#    method is; an is_X helper of a marker that nothing calls any more is deleted. Both only in the tool versions that have
#    vdispatch -rest (step 558 on): they run in step 560.
ts=tools/postflip/tswitch/main.go
if [[ -f $ts && -f tools/postflip/vdispatch/rest.go ]] && ! grep -q 'func orTests' $ts; then
  patch -s -p1 --no-backup-if-mismatch <<'E36' || { echo "toolfix: tswitch: the edits of fix 36 do not apply" >&2; exit 1; }
diff --git a/tools/postflip/tswitch/main.go b/tools/postflip/tswitch/main.go
index a5499c7aa..817c22f6e 100644
--- a/tools/postflip/tswitch/main.go
+++ b/tools/postflip/tswitch/main.go
@@ -25,6 +25,11 @@
 // level deeper than the chain has it. S is evaluated once instead of once per
 // test, which is the same because it is pure.
 //
+// A link may also be `if is_[A](S) || is_B(S) ...`, a disjunction of instanceof tests of S
+// alone (the generic is_[T], or a generated helper is_X whose body is the assertion to a named
+// type): it is the case `case A, B:`, taken when the value's type is one of them, as the
+// disjunction is true when one of the assertions succeeds. It binds no name.
+//
 // The switch binds one name: links that bind must bind the same name, and a link that does not,
 // and the default, must not mention that name (the switch would shadow it there). The ok
 // variables are not used anywhere else. Left, counted: chains that do not meet these. A rerun
@@ -52,6 +57,95 @@ type link struct {
 	ok   *ast.Ident
 	typ  ast.Expr
 	subj ast.Expr
+	// typs: the types of a link whose condition is a disjunction of instanceof tests of one
+	// value (is_[A](v) || is_B(v) ...): a case with several types. Such a link binds nothing
+	// and has no ok variable.
+	typs []ast.Expr
+}
+
+// isTest returns the tested value and type when e is is_[T](v), or is_X(v) of a generated
+// helper without type parameters whose body is `_, ok := v.(T); return ok`.
+func isTest(e ast.Expr) (ast.Expr, ast.Expr) {
+	c, ok := rw.Unparen(e).(*ast.CallExpr)
+	if !ok || len(c.Args) != 1 {
+		return nil, nil
+	}
+	switch f := c.Fun.(type) {
+	case *ast.IndexExpr:
+		if id, ok := f.X.(*ast.Ident); ok && id.Name == "is_" {
+			if fn, ok := rw.Info.Uses[id].(*types.Func); ok && fn.Pkg() == rw.Pkg && fn.Parent() == rw.Pkg.Scope() {
+				return c.Args[0], f.Index
+			}
+		}
+	case *ast.Ident:
+		fn, ok := rw.Info.Uses[f].(*types.Func)
+		if !ok || !strings.HasPrefix(f.Name, "is_") || fn.Pkg() != rw.Pkg || fn.Parent() != rw.Pkg.Scope() {
+			return nil, nil
+		}
+		if t := helperType(fn); t != nil {
+			return c.Args[0], t
+		}
+	}
+	return nil, nil
+}
+
+var helperDecls map[types.Object]*ast.FuncDecl
+
+// helperType is T of func is_X(v any) bool { _, ok := v.(T); return ok }, T a named type.
+func helperType(fn *types.Func) ast.Expr {
+	if helperDecls == nil {
+		helperDecls = map[types.Object]*ast.FuncDecl{}
+		for _, f := range rw.Files {
+			for _, d := range f.Decls {
+				if fd, ok := d.(*ast.FuncDecl); ok && fd.Recv == nil && fd.Body != nil {
+					helperDecls[rw.Info.Defs[fd.Name]] = fd
+				}
+			}
+		}
+	}
+	fd := helperDecls[fn]
+	if fd == nil || fd.Type.TypeParams != nil || fd.Type.Params.NumFields() != 1 || len(fd.Type.Params.List[0].Names) != 1 || len(fd.Body.List) != 2 {
+		return nil
+	}
+	as, ok := fd.Body.List[0].(*ast.AssignStmt)
+	if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 2 || len(as.Rhs) != 1 || rw.Text(as.Lhs[0]) != "_" {
+		return nil
+	}
+	ta, ok := as.Rhs[0].(*ast.TypeAssertExpr)
+	if !ok || ta.Type == nil || rw.Text(ta.X) != fd.Type.Params.List[0].Names[0].Name {
+		return nil
+	}
+	rs, ok := fd.Body.List[1].(*ast.ReturnStmt)
+	if !ok || len(rs.Results) != 1 || rw.Text(rs.Results[0]) != rw.Text(as.Lhs[1]) {
+		return nil
+	}
+	switch t := ta.Type.(type) {
+	case *ast.Ident:
+		return t
+	case *ast.StarExpr:
+		if _, ok := t.X.(*ast.Ident); ok {
+			return t
+		}
+	}
+	return nil
+}
+
+// orTests: the tests of a disjunction a || b || ..., each an instanceof test of the same value.
+func orTests(e ast.Expr) (subj ast.Expr, typs []ast.Expr) {
+	e = rw.Unparen(e)
+	if b, ok := e.(*ast.BinaryExpr); ok && b.Op == token.LOR {
+		s1, t1 := orTests(b.X)
+		s2, t2 := orTests(b.Y)
+		if s1 == nil || s2 == nil || rw.Text(s1) != rw.Text(s2) {
+			return nil, nil
+		}
+		return s1, append(t1, t2...)
+	}
+	s, t := isTest(e)
+	if s == nil {
+		return nil, nil
+	}
+	return s, []ast.Expr{t}
 }
 
 func main() {
@@ -83,6 +177,12 @@ func main() {
 	}
 	// asLink is ifs as a link, or nil.
 	asLink := func(ifs *ast.IfStmt) *link {
+		if ifs.Init == nil {
+			if subj, typs := orTests(ifs.Cond); len(typs) >= 2 {
+				return &link{ifs: ifs, subj: subj, typs: typs}
+			}
+			return nil
+		}
 		as, ok := ifs.Init.(*ast.AssignStmt)
 		if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 2 || len(as.Rhs) != 1 {
 			return nil
@@ -97,7 +197,7 @@ func main() {
 		if !ok1 || !ok2 || !ok3 || okId.Name == "_" || cond.Name != okId.Name {
 			return nil
 		}
-		l := &link{ifs: ifs, ok: okId, typ: ta.Type, subj: ta.X}
+		l := &link{ifs: ifs, ok: okId, typ: ta.Type, subj: ta.X, typs: []ast.Expr{ta.Type}}
 		if v.Name != "_" {
 			l.bind = v
 		}
@@ -120,6 +220,9 @@ func main() {
 	}
 	// usedOnlyAsCond: the ok variable is read only by its if's condition.
 	usedOnlyAsCond := func(l *link) bool {
+		if l.ok == nil {
+			return true
+		}
 		obj := rw.Info.Defs[l.ok]
 		for _, u := range rw.UsesOf[obj] {
 			if u != l.ifs.Cond {
@@ -199,12 +302,14 @@ func main() {
 			// The cases' types must be distinct (Go rejects a duplicate case).
 			seen := map[string]bool{}
 			for _, l := range chain {
-				t := types.TypeString(rw.Info.Types[l.typ].Type, nil)
-				if seen[t] {
-					skip("two links test one type", ifs)
-					return true
+				for _, typ := range l.typs {
+					t := types.TypeString(rw.Info.Types[typ].Type, nil)
+					if seen[t] {
+						skip("two links test one type", ifs)
+						return true
+					}
+					seen[t] = true
 				}
-				seen[t] = true
 			}
 			done[fmt.Sprintf("if/else-if chain of %d type tests -> type switch", len(chain))]++
 			rw.Rewrite[ifs] = func() string {
@@ -215,7 +320,11 @@ func main() {
 				}
 				b.WriteString(rw.Render(first.subj) + ".(type) {\n")
 				for _, l := range chain {
-					b.WriteString("case " + rw.Render(l.typ) + ":\n")
+					var ts []string
+					for _, typ := range l.typs {
+						ts = append(ts, rw.Render(typ))
+					}
+					b.WriteString("case " + strings.Join(ts, ", ") + ":\n")
 					b.WriteString(blockBody(rw.RenderChildren(l.ifs.Body)))
 				}
 				if rest != nil {
E36
fi
if [[ -f tools/postflip/vdispatch/rest.go && ! -f tools/postflip/isanames/main.go ]]; then
mkdir -p tools/postflip/isanames
cat > tools/postflip/isanames/main.go <<'E36b'
// Command isanames is a post-flip step (fifth flip): the marker interface of an instanceof test
// of a generic class gets a name.
//
//	_, ok := v.(interface{ isa_X_() })      ->  _, ok := v.(X_isa)
//	type X_isa interface{ isa_X_() }            (declared after the marker method of X)
//
// The translator writes `v instanceof X` of a generic class X (any type arguments: Java tests
// the erased class) as an assertion to the anonymous interface of X's marker method
// func (this *X[P...]) isa_X_() {}, which X and the classes that embed it have. The named
// interface has the same method set, so every assertion, type switch case and helper is_X
// gives the same result. X_isa is declared once, where the marker method is, when the name is
// free in the package.
//
// A generated helper func is_X(v any) bool { _, ok := v.(X_isa); return ok } that no code
// calls is deleted (tswitch and instof write its callers as assertions and cases).
//
// A rerun changes nothing.
//
// usage: isanames [-n] <dir of package closure>
package main

import (
	"fmt"
	"go/ast"
	"go/token"
	"go/types"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"postflip/rw"
)

// marker returns the method name when t is interface{ isa_X_() }.
func marker(t ast.Expr) string {
	it, ok := t.(*ast.InterfaceType)
	if !ok || it.Methods == nil || len(it.Methods.List) != 1 {
		return ""
	}
	f := it.Methods.List[0]
	ft, ok := f.Type.(*ast.FuncType)
	if !ok || len(f.Names) != 1 || ft.Params.NumFields() != 0 || ft.Results.NumFields() != 0 || ft.TypeParams != nil {
		return ""
	}
	if n := f.Names[0].Name; strings.HasPrefix(n, "isa_") && strings.HasSuffix(n, "_") && len(n) > 5 {
		return n
	}
	return ""
}

func main() {
	report := false
	for _, a := range os.Args[1 : len(os.Args)-1] {
		report = report || a == "-n"
	}
	rw.Load(os.Args[len(os.Args)-1])
	gen := func(n ast.Node) bool {
		return strings.HasPrefix(filepath.Base(rw.Fset.Position(n.Pos()).Filename), "g_")
	}
	// the marker methods, and the names already declared for them
	methods := map[string]*ast.FuncDecl{}
	declared := map[string]bool{}
	for _, f := range rw.Files {
		for _, d := range f.Decls {
			switch x := d.(type) {
			case *ast.FuncDecl:
				if gen(x) && x.Recv != nil && x.Body != nil && len(x.Body.List) == 0 && x.Type.Params.NumFields() == 0 && x.Type.Results.NumFields() == 0 &&
					strings.HasPrefix(x.Name.Name, "isa_") && strings.HasSuffix(x.Name.Name, "_") {
					methods[x.Name.Name] = x
				}
			case *ast.GenDecl:
				if x.Tok != token.TYPE {
					continue
				}
				for _, s := range x.Specs {
					ts := s.(*ast.TypeSpec)
					if m := marker(ts.Type); m != "" && ts.TypeParams == nil && ts.Name.Name == strings.TrimSuffix(strings.TrimPrefix(m, "isa_"), "_")+"_isa" {
						declared[m] = true
					}
				}
			}
		}
	}
	named, taken := map[string]int{}, map[string]int{}
	for _, f := range rw.Files {
		if !gen(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			it, ok := n.(*ast.InterfaceType)
			if !ok {
				return true
			}
			m := marker(it)
			if m == "" || methods[m] == nil {
				return true
			}
			if ts, ok := rw.Parent[it].(*ast.TypeSpec); ok && ts.Type == ast.Expr(it) {
				return true // a declaration
			}
			name := strings.TrimSuffix(strings.TrimPrefix(m, "isa_"), "_") + "_isa"
			if !declared[m] && rw.Pkg.Scope().Lookup(name) != nil {
				taken[name]++
				return true
			}
			named[m]++
			rw.Rewrite[it] = func() string { return name }
			return true
		})
	}
	var ms []string
	for m := range named {
		ms = append(ms, m)
	}
	sort.Strings(ms)
	ndecl := 0
	for _, m := range ms {
		if declared[m] {
			continue
		}
		ndecl++
		fd := methods[m]
		x := strings.TrimSuffix(strings.TrimPrefix(m, "isa_"), "_")
		rw.Rewrite[fd] = func() string {
			return rw.RenderChildren(fd) + "\n\n// " + x + "_isa is implemented by " + x + ", at any type arguments, and by the classes that extend it:\n// `instanceof " + x + "` asserts to it (post-flip).\ntype " + x + "_isa interface{ " + m + "() }"
		}
	}
	// helpers is_X of a marker interface that nothing calls
	ndead := 0
	for _, f := range rw.Files {
		if !gen(f) {
			continue
		}
		for _, d := range f.Decls {
			fd, ok := d.(*ast.FuncDecl)
			if !ok || fd.Recv != nil || fd.Body == nil || !strings.HasPrefix(fd.Name.Name, "is_") || len(fd.Body.List) != 2 || fd.Type.Params.NumFields() != 1 {
				continue
			}
			as, ok := fd.Body.List[0].(*ast.AssignStmt)
			if !ok || as.Tok != token.DEFINE || len(as.Lhs) != 2 || len(as.Rhs) != 1 {
				continue
			}
			ta, ok := as.Rhs[0].(*ast.TypeAssertExpr)
			if !ok || ta.Type == nil {
				continue
			}
			rs, ok := fd.Body.List[1].(*ast.ReturnStmt)
			if !ok || len(rs.Results) != 1 || rw.Text(rs.Results[0]) != rw.Text(as.Lhs[1]) {
				continue
			}
			m := marker(ta.Type)
			if m == "" {
				if tv, ok := rw.Info.Types[ta.Type]; ok {
					if nt, ok := types.Unalias(tv.Type).(*types.Named); ok && strings.HasSuffix(nt.Obj().Name(), "_isa") {
						if it, ok := nt.Underlying().(*types.Interface); ok && it.NumMethods() == 1 && it.Method(0).Name() == "isa_"+strings.TrimSuffix(nt.Obj().Name(), "_isa")+"_" {
							m = it.Method(0).Name()
						}
					}
				}
			}
			if m == "" || methods[m] == nil || len(rw.UsesOf[rw.Info.Defs[fd.Name]]) != 0 {
				continue
			}
			ndead++
			rw.Rewrite[fd] = func() string { return rw.DelDoc }
		}
	}
	total := 0
	for _, n := range named {
		total += n
	}
	var tk []string
	for n, c := range taken {
		tk = append(tk, fmt.Sprintf("%s (%d)", n, c))
	}
	sort.Strings(tk)
	fmt.Fprintf(os.Stderr, "isanames: %d anonymous marker interfaces named (%d classes, %d X_isa declared); %d uncalled is_X helpers of a marker deleted; left: %d names taken %v\n", total, len(named), ndecl, ndead, len(taken), tk)
	if report || total+ndead == 0 {
		return
	}
	fmt.Fprintf(os.Stderr, "isanames: files changed: %d\n", rw.SaveChecked())
}
E36b
fi
# 37. (sixth flip) ranges: the translator's rule "typed library interfaces" writes a for-each loop over a place of a typed
#    library interface (IterableT[T], CollectionT[T], ListT[T], SetT[T]: 501 loops in the flip, where the fifth flip had
#    foreach_(c) and a cast of the element in the loop's first statement) with the typed cursor of rt_lg.go,
#    `it := jeachOf_(X); for it.next() { var x T = it.v .. }` (eachOf_ before reflip/rename.sh), and an explicit iterator of
#    such a place as `var it IteratorT[T] = C.iterator_t()` read with it.next_t(). The tool read foreach_ and C.iterator() only.
#    (a) jeachOf_(X) is a cursor source of its own kind: with -loops the loop is `for x := range elemsT_(X)`. elemsT_
#    (rt_lg_iter.go, this flip's patch reflip6-elemst-rt.patch at step 202) makes the same cursor jeachOf_(X) when the loop
#    starts and yields its elements, so the walk is the cursor's by construction; x is the first statement's variable when that
#    is `var x T = it.v` with T the cursor's element type (the tool's own rule for typed cursors), and the tool's conditions on
#    the cursor's other uses and on bodies that return are unchanged (-index leaves this kind: it has no index form).
#    (b) `var it IteratorT[T] = C.iterator_t()` used only for the loop's hasNext and one next_t() (or next()) in the first
#    statement is the tool's iterator loop: `for x := range elemsT_(C)` when the statement is `var x T = it.next_t()`. That is
#    the tool's existing claim for elems_ at the typed interface: the cursor of jeachOf_ reads a typed collection of the runtime
#    in place exactly as its iterator does, reads a wrapped erased collection with foreach_ (the same claim one level down) and
#    calls C.iterator_t() for anything else.
reflip_edit tools/postflip/ranges/main.go reflip37 <<'E37'
		case "foreach_":
			return id, &source{kind: "erased", x: c.Args[0]}
=====
		case "foreach_":
			return id, &source{kind: "erased", x: c.Args[0]}
		case "jeachOf_": // reflip37: the translator's typed for-each cursor over an IterableT[T]
			if el := typeArgs(typeOf(as.Rhs[0])); len(el) == 1 {
				return id, &source{kind: "typed", x: c.Args[0], elem: el[0]}
			}
			return nil, nil
-----
		case "erased":
			c := rw.Render(stripAny(src.x))
			seq = "elems_(" + c + ")"
=====
		case "typed": // reflip37
			seq = "elemsT_(" + rw.Render(src.x) + ")"
		case "erased":
			c := rw.Render(stripAny(src.x))
			seq = "elems_(" + c + ")"
-----
	if named(typeOf(id)) != "Iterator" && !isIteratorAlias(rw.Info.Defs[id]) {
		return nil, nil
	}
=====
	if named(typeOf(id)) == "IteratorT" { // reflip37: an iterator of a typed library interface
		if recv, m := method(val); recv != nil && m == "iterator_t" {
			return id, recv
		}
		return nil, nil
	}
	if named(typeOf(id)) != "Iterator" && !isIteratorAlias(rw.Info.Defs[id]) {
		return nil, nil
	}
-----
		if recv, m := method(call); recv != nil && m == "next" && objOf(recv) == it {
			nextCall, r = call, typeOf(call)
=====
		if recv, m := method(call); recv != nil && (m == "next" || m == "next_t") && objOf(recv) == it {
			nextCall, r = call, typeOf(call)
			reflip37typed = m == "next_t"
-----
	if seq == "" {
=====
	if seq == "" && reflip37typed {
		seq = "elemsT_(" + rw.Render(c) + ")"
		done["iterator over a typed Iterable"]++
	}
	if seq == "" {
-----
	var nextCall ast.Expr
	var fText string
=====
	var nextCall ast.Expr
	reflip37typed := false
	var fText string
E37
# 38. (sixth flip) trips: the tool refuses to run when the runtime lacks one of the functions it names. Its list has the helpers
#    it writes (tlistSlice_ .. sliceSkip_: needed) and the erased functions whose calls it reads in a round trip (sliceImm_ ..
#    Iterables_skip). One of the second kind, ImmutableList_copyOf_ObjectArr, is not in main's default runtime any more (rule
#    "typed library interfaces", piece cls: no generated code calls it; 0 mentions in g_*.go before step 512). A function the
#    runtime does not have is called nowhere in a package that type-checks, so the round trips through it have no subject and
#    every other pattern is as it was: the tool says so and goes on. A missing helper it writes still stops it.
reflip_edit tools/postflip/trips/main.go reflip38 <<'E38'
		if !rw.HasFunc(h) {
			fmt.Fprintf(os.Stderr, "trips: the runtime has no %s: nothing done\n", h)
			os.Exit(1)
		}
=====
		if !rw.HasFunc(h) {
			if h == "ImmutableList_copyOf_ObjectArr" { // reflip38: a function read in a pattern, not written
				fmt.Fprintf(os.Stderr, "trips: the runtime has no %s: the round trips through it have no subject\n", h)
				continue
			}
			fmt.Fprintf(os.Stderr, "trips: the runtime has no %s: nothing done\n", h)
			os.Exit(1)
		}
E38
# 39. (sixth flip) optstr: the one map that holds Optional<String> objects for erased code (ContextualRenamer.declarations, an
#    erased LinkedHashMap whose toString prints them; fix 15) is written by rule "typed library interfaces" behind its typed
#    interface: the field is MapT[string, Optional[string]], assigned mapAs_(tc_string, tc_Optional_string_, Map(new_LinkedHashMap()))
#    (the wrapper of rt_lg.go over the same erased map, with the package's conversion of its values), written with
#    m.put_t(k, o) and read as before under Map_un_(m) (Map_containsKey_s, optOf_[string](Map_getOrDefault_s(Map_un_(m), k,
#    o.box_()))) and lgBox_(m) (the toString helper). The proof knew the alias MapOf[string, Optional[string]] and failed on 8
#    obligations, so step 553 rewrote nothing (1,378 type spellings, 3,429 calls on the fifth flip). The same rule read at the
#    new notation: the map is kept when it is that field, its one assignment is the wrapper of a new LinkedHashMap / HashMap,
#    and its uses are those reads and m.put_t(k, o) as a statement (the old value it returns, null for a new key, is not
#    used; o is an Optional<String> of the program, never null by the tool's other proofs, as o.box_() was in Map_put_s).
#    What is kept as it is, Optional[string] spelled: the field's type, and the declaration of a conversion
#    *tconv_[Optional[string]] whose every use is the value conversion of such a wrapper (its box and unbox are the Optional
#    object's, also for the null a get of an absent key gives). At m.put_t(k, o) the value, a string after the step, is the
#    Optional again: optOf_[string](optstrBox_(o)), the Optional of the object erased code sees for o (optstrBox_ and optOf_
#    are inverse on non-null Optionals: rt_optstr.go). Everything else is the tool's.
reflip_edit tools/postflip/optstr/proof.go reflip39 <<'E39'
	} else if !ok || n.Obj().Name() != "MapOf" {
		return false
	}
	for _, id := range rw.UsesOf[v] {
		sel, ok := rw.Parent[id].(*ast.SelectorExpr)
		if !ok {
			return false
		}
		switch p := rw.Up(sel).(type) {
=====
	} else if ok && n.Obj().Name() == "MapT" && n.TypeArgs().Len() == 2 && isOptStr(n.TypeArgs().At(1)) { // reflip39: the map behind its typed interface
	} else if !ok || n.Obj().Name() != "MapOf" {
		return false
	}
	for _, id := range rw.UsesOf[v] {
		sel, ok := rw.Parent[id].(*ast.SelectorExpr)
		if !ok {
			return false
		}
		up := rw.Up(sel)
		if c, ok := up.(*ast.CallExpr); ok && len(c.Args) == 1 && rw.Unparen(c.Args[0]) == ast.Expr(sel) { // reflip39: Map_un_(m), lgBox_(m): the erased map
			if name, _ := rw.CallName(c); name == "Map_un_" || name == "lgBox_" {
				up = rw.Up(c)
			}
		}
		if ps, ok := up.(*ast.SelectorExpr); ok { // reflip39: m.put_t(k, o) as a statement, o an Optional<String>
			c, isCall := rw.Up(ps).(*ast.CallExpr)
			if !isCall || c.Fun != ast.Expr(ps) || ps.Sel.Name != "put_t" || len(c.Args) != 2 || !isOptStr(rw.Info.TypeOf(c.Args[1])) {
				return false
			}
			if _, isStmt := rw.Up(c).(*ast.ExprStmt); !isStmt {
				return false
			}
			continue
		}
		switch p := up.(type) {
-----
			rhs := p.Rhs[0]
			if c, ok := rw.Unparen(rhs).(*ast.CallExpr); ok && len(c.Args) == 1 { // reflip15: Map(new_LinkedHashMap())
=====
			rhs := p.Rhs[0]
			if c, ok := rw.Unparen(rhs).(*ast.CallExpr); ok && len(c.Args) == 3 { // reflip39: mapAs_(kc, vc, Map(new_LinkedHashMap()))
				if name, _ := rw.CallName(c); name == "mapAs_" {
					rhs = c.Args[2]
				}
			}
			if c, ok := rw.Unparen(rhs).(*ast.CallExpr); ok && len(c.Args) == 1 { // reflip15: Map(new_LinkedHashMap())
-----
	sel, ok := g.Args[0].(*ast.SelectorExpr)
	if !ok {
		return "map not a field"
	}
=====
	a0 := rw.Unparen(g.Args[0])
	if c, isC := a0.(*ast.CallExpr); isC && len(c.Args) == 1 { // reflip39: Map_un_(m)
		if name, _ := rw.CallName(c); name == "Map_un_" {
			a0 = rw.Unparen(c.Args[0])
		}
	}
	sel, ok := a0.(*ast.SelectorExpr)
	if !ok {
		return "map not a field"
	}
-----
			} else {
				fail(id, "package variable holding Optionals in a container")
			}
=====
			} else if reflip39Conv(v) {
				proved["conversion of a kept map's values (the Optional objects erased code sees)"]++
			} else {
				fail(id, "package variable holding Optionals in a container")
			}
-----
			case "MapOf":
				// the kept map: checked with its field
=====
			case "MapOf":
				// the kept map: checked with its field
			case "MapT", "mapAs_", "tconv_": // reflip39: the kept map's typed interface, its wrapper, its value conversion
				if !reflip39In(id) {
					fail(id, "generic code instantiated with Optional[string] ("+id.Name+")")
				}
+++++

// reflip39Conv: v is a conversion *tconv_[Optional[string]] of the package whose every use is the
// value conversion of a kept map's wrapper, mapAs_(kc, v, ..) assigned to the kept field.
func reflip39Conv(v *types.Var) bool {
	p, ok := types.Unalias(v.Type()).(*types.Pointer)
	if !ok {
		return false
	}
	n, ok := types.Unalias(p.Elem()).(*types.Named)
	if !ok || n.Obj().Name() != "tconv_" || n.TypeArgs().Len() != 1 || !isOptStr(n.TypeArgs().At(0)) || len(rw.UsesOf[v]) == 0 {
		return false
	}
	for _, id := range rw.UsesOf[v] {
		c, ok := rw.Up(id).(*ast.CallExpr)
		if !ok || len(c.Args) != 3 || rw.Unparen(c.Args[1]) != ast.Expr(id) {
			return false
		}
		if name, _ := rw.CallName(c); name != "mapAs_" {
			return false
		}
		as, ok := rw.Up(c).(*ast.AssignStmt)
		if !ok || len(as.Lhs) != 1 {
			return false
		}
		sel, ok := as.Lhs[0].(*ast.SelectorExpr)
		if !ok {
			return false
		}
		f, _ := rw.Info.Uses[sel.Sel].(*types.Var)
		if f == nil || !isMapKept(f) {
			return false
		}
	}
	return true
}

// reflip39In: id is in the declaration of a kept map field (its type), in the assignment of
// its wrapper, or in the declaration of its value conversion.
func reflip39In(id *ast.Ident) bool {
	for n := ast.Node(id); n != nil; n = rw.Parent[n] {
		switch x := n.(type) {
		case *ast.Field:
			for _, nm := range x.Names {
				if v, ok := rw.Info.Defs[nm].(*types.Var); ok && v.IsField() && isMapKept(v) {
					return true
				}
			}
			return false
		case *ast.AssignStmt:
			if len(x.Lhs) == 1 {
				if sel, ok := x.Lhs[0].(*ast.SelectorExpr); ok {
					if v, ok := rw.Info.Uses[sel.Sel].(*types.Var); ok && v.IsField() && isMapKept(v) {
						return true
					}
				}
			}
			return false
		case *ast.ValueSpec:
			for _, nm := range x.Names {
				if v, ok := rw.Info.Defs[nm].(*types.Var); ok && reflip39Conv(v) {
					return true
				}
			}
			return false
		case *ast.FuncDecl:
			return false
		}
	}
	return false
}

// reflip39Put: c is m.put_t(k, o) of a kept map field m.
func reflip39Put(c *ast.CallExpr) bool {
	ps, ok := c.Fun.(*ast.SelectorExpr)
	if !ok || ps.Sel.Name != "put_t" || len(c.Args) != 2 {
		return false
	}
	sel, ok := rw.Unparen(ps.X).(*ast.SelectorExpr)
	if !ok {
		return false
	}
	v, _ := rw.Info.Uses[sel.Sel].(*types.Var)
	return v != nil && v.IsField() && isMapKept(v)
}
E39
reflip_edit tools/postflip/optstr/rewrite.go reflip39 <<'E39b'
					} else if !declaredInGenerated(s.Obj()) {
						fail(x, "runtime method returning Optional<String> ("+sel.Sel.Name+")")
=====
					} else if !declaredInGenerated(s.Obj()) && !reflip39Put(x) { // reflip39
						fail(x, "runtime method returning Optional<String> ("+sel.Sel.Name+")")
-----
	inner := map[ast.Node]bool{} // calls a rewrite of their `!` replaces
=====
	// reflip39: the declaration of a kept map's value conversion keeps its spellings; a value put
	// through the kept map's typed interface is the Optional of the object erased code sees
	for id, o := range rw.Info.Defs {
		if v, ok := o.(*types.Var); ok && v.Parent() == rw.Pkg.Scope() && reflip39Conv(v) {
			if vs, ok := rw.Parent[id].(*ast.ValueSpec); ok {
				ast.Inspect(vs, func(n ast.Node) bool {
					if n != nil {
						skip[n] = true
					}
					return true
				})
				left["type spellings kept: the value conversion of an erased map holding Optional objects"] = append(left["type spellings kept: the value conversion of an erased map holding Optional objects"], rw.Fset.Position(id.Pos()).String())
			}
		}
	}
	for _, f := range rw.Files {
		if !generated(f) {
			continue
		}
		ast.Inspect(f, func(n ast.Node) bool {
			if c, ok := n.(*ast.CallExpr); ok && reflip39Put(c) {
				c := c
				done["m.put_t(k, o) of the kept map -> m.put_t(k, optOf_[string](optstrBox_(o)))"]++
				rw.Rewrite[c] = func() string {
					return rw.Render(c.Fun) + "(" + rw.Render(c.Args[0]) + ", optOf_[string](optstrBox_(" + rw.Render(c.Args[1]) + ")))"
				}
			}
			return true
		})
	}
	inner := map[ast.Node]bool{} // calls a rewrite of their `!` replaces
E39b
