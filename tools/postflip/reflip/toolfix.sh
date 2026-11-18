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
  perl -0pi -e 's/(func ConvFor\(t types\.Type\) Conv \{\n)/$1\tif ReflipGeneric_(t, '$inst') {\n\t\treturn Conv{}\n\t}\n/' $rwgo
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
    s/(\t\t\treturn value\{kind: vSlot, k: resultKey\(groupOf\(f\), 0\)\})/\t\t\tif f.Origin() != f { \/\/ reflip: generic instance\n\t\t\t\treturn value{kind: vForeign, why: "result of a method of a generic type"}\n\t\t\t}\n$1/;
    s/(\treturn paramKey\(groupOf\(f\), i\), true)/\tif f.Origin() != f { \/\/ reflip: generic instance\n\t\tpoison(paramKey(groupOf(f), i), "argument of a method of a generic type")\n\t}\n$1/;
    s/(func slotOfVar\(v \*types\.Var\) int \{\n\tk := varKey\(v\)\n)/$1\tif v.Origin() != v || rw.ReflipGeneric_(v.Type(), false) { \/\/ reflip: generic instance\n\t\tpoison(k, "variable of a type parameter\x27s type")\n\t}\n/;
  ' $cg
  [[ $(grep -c 'reflip:' $cg) == 3 ]] || { echo "toolfix: colls: anchors not found" >&2; exit 1; }
fi
# 4. native: the same rule as 3 (the tool has colls' slots as graph nodes).
ng=tools/postflip/native/main.go
if [[ -f $ng ]] && ! grep -q 'reflip:' $ng; then
  perl -0pi -e '
    s/(\t\t\treturn value\{kind: vSlot, n: resultNode\(groupOf\(f\), 0\)\})/\t\t\tif f.Origin() != f { \/\/ reflip: generic instance\n\t\t\t\treturn value{kind: vForeign, why: "result of a method of a generic type"}\n\t\t\t}\n$1/;
    s/(\treturn paramNode\(groupOf\(f\), i\), true)/\tif f.Origin() != f { \/\/ reflip: generic instance\n\t\tpoison(paramNode(groupOf(f), i), "argument of a method of a generic type")\n\t}\n$1/;
    s/(func varNode\(v \*types\.Var\) \*node \{\n\tn := get\(fmt\.Sprintf\("v\|%d", v\.Pos\(\)\)\)\n)/$1\tif v.Origin() != v || rw.ReflipGeneric_(v.Type(), false) { \/\/ reflip: generic instance\n\t\tpoison(n, "variable of a type parameter\x27s type")\n\t}\n/;
  ' $ng
  [[ $(grep -c 'reflip:' $ng) == 3 ]] || { echo "toolfix: native: anchors not found" >&2; exit 1; }
fi
