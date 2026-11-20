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
# 18. (fourth flip, in bounds) checks: the translator's rules P2 and P4 write a precondition whose checked value or one message
#    argument has a known type as a typed variant of the runtime function: Preconditions_check..._H(c, [t,] n) takes a handle n and
#    boxes it only when the check fails (n.box_()); Preconditions_checkNotNull..._T(v, ...) and ..._Str(v, ...) take the checked value
#    at its own type (null is the zero value of T, nullstr_ of a String), return it typed, and take the message arguments as they
#    are (an Object message is String.valueOf(canon_(msg))). The tool read the erased names only, so it left all of these (1,044 of
#    2,503 checks: the lead line's translator wrote none of them and its step made 2,497 of 2,501 inline). A typed variant is the
#    erased check of the same name with: the handle read as boxed by box_() (_H); the checked value tested at its type by the
#    tool's own null test (_T, _Str); a single Object message read under the boxing canon_ gives it (a String strobj_, a class
#    pointer self(), anything else canon_ itself). Every rewrite is then the tool's own, with the runtime function's exception and
#    message.
reflip_edit tools/postflip/checks/main.go reflip18 <<'E18'
func unbox(e ast.Expr) (string, ast.Expr) {
	e = rw.Unparen(e)
=====
func unbox(e ast.Expr) (string, ast.Expr) {
	e = rw.Unparen(e)
	if b, ok := reflipBox_[e]; ok { // reflip18: an argument a typed variant boxes itself
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
	case box == "%s" && reflipChecked_[in]: // reflip18: the value of a typed checkNotNull, tested at its type
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
				_, ptr := at.Underlying().(*types.Pointer)
				switch {
				case ab != nil && ab.Kind() == types.String:
					reflipBox_[a] = "strobj_(%s)"
				case ptr && rw.HasMethod(at, "self") && reflipPrimary_(a):
					reflipBox_[a] = "%s.self()"
				default:
					reflipBox_[a] = "canon_(%s)"
				}
			}
		}
		return true
	}
	return false
}
E18
# 19. (fourth flip, in bounds) sbuild: two more forms of this translator that kept 41 StringBuilder slots (16 "passed to jtostr_",
#    25 "returned as another type"; the third flip's tool fix e8c5ab2d on reflip2-bridge, which the lead line's tools never had
#    because its translator wrote neither form):
#    - jtostr_(sb), the translator's String.valueOf at the argument's type (rt_conv.go: objstr_(obj_(sb)), "null" for nil), is the
#      read tostr_(sb) is, handled the same way;
#    - StringBuilder_as_Appendable(sb), the nil-keeping upcast helper of g_0casts.go (rule "implicit conversions"), is where the
#      erased output had the builder itself in an Appendable position: the helper's parameter is no slot, the argument is a builder
#      passed as an Appendable (appendable_(sb), which keeps nil too) and the helper call around a converted builder is dropped.
if [[ -f $bg ]] && ! grep -q 'reflip19' $bg && grep -q 'fn.Name() == "tostrR_")' $bg; then
  perl -pi -e 's/\|\| fn\.Name\(\) == "tostrR_"\) && len\(x\.Args\) == 1/|| fn.Name() == "tostrR_" || fn.Name() == "jtostr_") \&\& len(x.Args) == 1/' $bg
  grep -q '"jtostr_")' $bg || { echo "toolfix: sbuild: the tostr_ line was not found" >&2; exit 1; }
  reflip_edit $bg reflip19 <<'E19'
func appendArg(k key, e ast.Expr, local bool, v *types.Var) {
=====
// reflipAsAppendable_ is the translator's upcast of a *StringBuilder to Appendable (reflip19); reflipUpcasts_ are its calls
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
				if x.Recv == nil && x.Name.Name == reflipAsAppendable_ { // reflip19: its parameter is no slot
					return true
				}
				fieldList(x.Type.Params, func(i int) *types.Var { return sig.Params().At(i) }, g, "")
-----
				for i, a := range x.Args {
					if rw.Unparen(a) == e {
						if pv := paramVar(fn, i); pv != nil && isSlot[pv] {
=====
				if fn.Name() == reflipAsAppendable_ && len(x.Args) == 1 && rw.Unparen(x.Args[0]) == e { // reflip19
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
	for k, cs := range reflipUpcasts_ { // reflip19: the upcast helper around a converted builder goes
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
E19
  grep -q 'reflip19' $bg || echo '// reflip19: jtostr_ only (this version has no Appendable arguments)' >> $bg
fi
# 20. (fourth flip, in bounds) eqhash: the translator's rules "typed Object methods" and "typed Object calls" write the protocol's
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
if [[ -f $eg ]] && ! grep -q 'reflip20' $eg && grep -q 'case "Objects_requireNonNull", "Objects_requireNonNull_Object":' $eg; then
  perl -pi -e 's/rw\.Render\(B\)/reflipBoxed_(B)/g; s/\brender\(B\)/func() string { return reflipBoxed_(B) }/g' $eg
  reflip_edit $eg reflip20 <<'E20'
func unbox(e ast.Expr) (boxed, bool) {
	e = rw.Unparen(e)
=====
func unbox(e ast.Expr) (boxed, bool) {
	e = rw.Unparen(e)
	if via, ok := reflipVia_[e]; ok { // reflip20: an argument a typed helper boxes itself
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
E20
fi
# 21. (fourth flip, in bounds) rw.Unboxed: "the code storing boxed expression a (of type any) as a value of type t" drops a boxing
#    call whose operand has type t (strobj_(x), obj_(x), ...); any other expression is unboxed by the type's conversion. The
#    translator's rule P4 passes a String where an Object is taken as the string itself (put("CLASS_DECL", v), the erased output had
#    strobj_("CLASS_DECL")), so the typed immutable builders of step 336 got their constant keys as str_("CLASS_DECL"): 236
#    str_( of a string (third flip: the same 236; lead line: 0). A value whose static type is already the string type t is its own
#    unboxing (str_ of a Go string in an `any` is the string).
reflip_edit tools/postflip/rw/rw.go reflip21 <<'E21'
func Unboxed(cv Conv, t types.Type, a ast.Expr) string {
	a = Unparen(a)
=====
func Unboxed(cv Conv, t types.Type, a ast.Expr) string {
	a = Unparen(a)
	if tv, ok := Info.Types[a]; ok && tv.Type != nil && types.Identical(tv.Type, t) { // reflip21: a String passed as itself
		if b, ok := t.Underlying().(*types.Basic); ok && b.Kind() == types.String {
			return Render(a)
		}
	}
E21
# 22. (fourth flip, in bounds) funcparams: fix 17 reads the object under the upcast written as a conversion, I(o). The same rule of
#    the translator writes the upcast of a value that may be null as the nil-keeping helper C_as_I(o) of g_0casts.go
#    (FunctionArgumentInjector_ReferencedAfterSideEffect_as_NodeUtil_Visitor(collector)); the tool took it for "another value" and
#    kept every parameter such a call reaches an interface (NodeUtil.Visitor: 13 lambda objects &NodeUtil_Visitor_fn{func...} the
#    lead line has as Go functions). The object of the helper is its operand; whether it is null is the tool's own question
#    (nonNullLocal and the rest), as for the erased icast_I(obj_(o)).
reflip_edit tools/postflip/funcparams/main.go reflip22 <<'E22'
	if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 { // reflip17: the upcast written out, I(x)
		if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() && types.IsInterface(tv.Type) {
			e = rw.Unparen(c.Args[0])
		}
	}
=====
	if c, ok := e.(*ast.CallExpr); ok && len(c.Args) == 1 { // reflip17: the upcast written out, I(x)
		if tv, ok := rw.Info.Types[c.Fun]; ok && tv.IsType() && types.IsInterface(tv.Type) {
			e = rw.Unparen(c.Args[0])
		} else if id, ok := c.Fun.(*ast.Ident); ok && strings.Contains(id.Name, "_as_") { // reflip22: the nil-keeping upcast helper C_as_I(x)
			if fn, ok := rw.Info.Uses[id].(*types.Func); ok && strings.HasSuffix(rw.Fset.Position(fn.Pos()).Filename, "g_0casts.go") {
				e = rw.Unparen(c.Args[0])
			}
		}
	}
E22
# 23. (fourth flip, in bounds) exceptions: the translator's typed collection runtime (rt_jtimm.go, until step 402 unifies the
#    families) panics with the value frozen_{} for a change to an immutable typed collection, and catch_ makes it Java's
#    UnsupportedOperationException (rt_lang.go). The tool took it for a value of unknown class that every caller of a typed
#    collection could throw and kept every catch clause such a call reaches ("it can throw a value of unknown class:
#    computeIfAbsent: panic(frozen_{})"). The class is known (the third flip's tool fix 9cddf058 on reflip2-bridge).
reflip_edit tools/postflip/exceptions/main.go reflip23 <<'E23'
	if c, ok := x.(*ast.CallExpr); ok {
		if id, ok := c.Fun.(*ast.Ident); ok {
			if strings.HasPrefix(id.Name, "new_") {
=====
	if cl, ok := x.(*ast.CompositeLit); ok && len(cl.Elts) == 0 && rw.Text(cl.Type) == "frozen_" { // reflip23
		return []string{"UnsupportedOperationException"}, true
	}
	if c, ok := x.(*ast.CallExpr); ok {
		if id, ok := c.Fun.(*ast.Ident); ok {
			if strings.HasPrefix(id.Name, "new_") {
E23
