#!/usr/bin/env bash
# Translator self-test: the programs in lang/ must print the same transcript when run
# by Java and when translated to Go. lang/Handles.java exercises handle classes: its
# classes listed in selftest/handles.txt are plain classes on the JVM and handles in Go.
set -euo pipefail
cd "$(dirname "$0")/../../.."
# the methods of the self-test programs whose unchecked casts rule "typed library interfaces" lets pass
export J2GO_UNCHECKED_CASTS=tools/j2go/unchecked-casts.txt:tools/j2go/selftest/unchecked-casts.txt
# piece `objarg`: the key / element types with a typed map or set (the tree's and lang/LibObj.java's)
export J2GO_OBJARG_PAIRS=tools/j2go/objarg-pairs.txt:tools/j2go/selftest/objarg-pairs.txt
t=.cache/j2go-selftest
rm -rf "$t" && mkdir -p "$t/java" "$t/go/closure" "$t/go/cmd/t"
# lang/Imm.java uses Guava (typed immutable collections).
javac -nowarn -cp .cache/build/stock-libs.jar -d "$t/java" tools/j2go/selftest/lang/*.java
for c in Lang Lang2 Handles Generics Generics2 Generics3 Colls Imm Unboxed Covariant CovariantIface Layout FnTypes LibGen LibFns GOpt LibObj Cmp; do env -u JAVA_TOOL_OPTIONS java -Dstdout.encoding=UTF-8 -cp "$t/java:.cache/build/stock-libs.jar" lang.$c; done >"$t/expected.txt"
# The test class is translated together with the whole tree, so that the runtime it needs is all there.
cp go/go.mod "$t/go/" && cp go/closure/rt_*.go tools/j2go/selftest/rt_*.go "$t/go/closure/" && rm -f "$t"/go/closure/*_test.go
[[ -d go/closure/resources ]] && cp -r go/closure/resources "$t/go/closure/"
{ cat .cache/j2go/files.txt; ls tools/j2go/selftest/lang/*.java; } >"$t/files.txt"
# Instrumented output, so that static initialization happens on first use as in Java.
J2GO_HANDLES=tools/j2go/handles.txt:tools/j2go/selftest/handles.txt J2GO_INIT_GUARDS=1 java -Xss64m -Xmx8g --add-modules jdk.compiler -cp "${J2GO_CLASSES:-.cache/j2go}" j2go.Main ".cache/build/stock-libs.jar:.cache/build/main:$(ls .cache/deps/auto-value-annotations-*.jar)" "$t/go/closure" - @"$t/files.txt" 2>&1 | grep -v '^Picked up' | tee "$t/j2go.log" || true
if grep -q '^objarg: the pair list is stale:' "$t/j2go.log"; then echo "selftest: FAIL (objarg: the pair lists are stale)"; exit 1; fi
cat >"$t/go/cmd/t/main.go" <<'EOF'
package main

import (
	"fmt"
	"os"

	"jsgomech/closure"
)

func main() {
	if len(os.Args) > 1 {
		fmt.Println(closure.SelfTestSizes())
		return
	}
	fmt.Print(closure.SelfTest())
}
EOF
cat >"$t/go/closure/rt_selftest.go" <<'EOF'
package closure

import (
	"fmt"
	"unsafe"
)

func SelfTest() string { return Lang_run() + Lang2_run() + Handles_run() + Generics_run() + Generics2_run() + Generics3_run() + Colls_run() + Imm_run() + Unboxed_run() + Covariant_run() + CovariantIface_run() + Layout_run() + FnTypes_run() + LibGen_run() + LibFns_run() + GOpt_run() + LibObj_run() + Cmp_run() }

// SelfTestSizes: the sizes of the structs of lang/Layout.java.
func SelfTestSizes() string {
	return fmt.Sprintf("Mixed=%d Sub=%d Slot=%d Rec=%d Inner=%d Local=%d", unsafe.Sizeof(Layout_Mixed{}), unsafe.Sizeof(Layout_Sub{}),
		unsafe.Sizeof(Layout_Slot[string, *Handles_Tag]{}), unsafe.Sizeof(Layout_Rec{}), unsafe.Sizeof(Layout_Inner{}), unsafe.Sizeof(Layout_1Local{}))
}
EOF
# Rule "function types" off (J2GO_NOFNTYPES=1): the runtime's erased forms (rt_fn_erased.go).
tags=()
[[ ${J2GO_NOFNTYPES:-} == 1 ]] && tags=(-tags j2go_nofntypes)
# A piece of rule "typed library interfaces" off (J2GO_NOLIBGEN=...): the erased functions only such a translation names (rt_lg_erased.go).
[[ -n ${J2GO_NOLIBGEN:-} ]] && tags=(-tags "${tags[1]:+${tags[1]},}j2go_nolibgen")
(cd "$t/go" && gofmt -w closure/g_*.go && go build "${tags[@]}" -o ../t ./cmd/t)
"$t/t" >"$t/actual.txt" || true
# Rule "field layout" (lang/Layout.java): the structs have no padding but at their end.
sizes="Mixed=136 Sub=168 Slot=40 Rec=48 Inner=24 Local=40"
[[ ${J2GO_NOLAYOUT:-} == 1 ]] && sizes="Mixed=168 Sub=208 Slot=56 Rec=72 Inner=32 Local=48"
echo "$sizes" >>"$t/expected.txt" && "$t/t" sizes >>"$t/actual.txt"
if ! diff "$t/expected.txt" "$t/actual.txt"; then echo "selftest: FAIL"; exit 1; fi
if [[ -z ${J2GO_NOLIBGEN:-} && ${J2GO_NOFNTYPES:-} != 1 ]]; then
  # Rule "typed library interfaces", piece `fns` (lang/LibFns.java): typed() calls the typed twins
  # (39 of them, 6 of piece `seqcoll`), erased() none (variance, an erased argument, keys that are not Strings, a
  # function without a twin: 4 calls of the erased functions).
  twin='\b(Iterables_[a-zA-Z]+T_|MoreObjects_firstNonNullT_|List_sortT_|Collections_sortT_|Iterable_forEachT_|Map_[a-zA-Z_]+T_|SequencedCollection_[a-zA-Z]+T_)\[|\b(newTSortedSetCT_|newTSortedMapCT_|splitToListST_)\('
  erased='\b(Iterables_getFirst|Map_get_s|Lists_reverse)\('
  g="$t/go/closure/g_LibFns.go"
  nt=$(awk '/^func LibFns_typed\(/,/^}/' "$g" | { grep -oE "$twin" || true; } | wc -l)
  nx=$(awk '/^func LibFns_erased\(/,/^}/' "$g" | { grep -oE "$twin" || true; } | wc -l)
  ne=$(awk '/^func LibFns_erased\(/,/^}/' "$g" | { grep -oE "$erased" || true; } | wc -l)
  if [[ $nt != 39 || $nx != 0 || $ne != 4 ]]; then echo "selftest: FAIL (typed library functions: $nt twins in typed(), $nx and $ne erased calls in erased())"; exit 1; fi
fi
if [[ -z ${J2GO_NOLIBGEN:-} ]]; then
  # Rule "typed library interfaces", piece `objarg` (lang/LibObj.java): typed() and the methods it calls have 25 calls of
  # the typed twins (Map and Set receivers of key types on the pair lists) and 28 erased calls (List / Collection
  # receivers, pairs with no typed map or set, results that enter an Object); erased() and wide() / wild() / obj() no
  # twin (an Object, a supertype, another interface, Object keys, a wildcard, a result that enters an Object, a field of a
  # type variable's type: 13 erased calls).
  twin='\b(Map_getK_|Map_containsKeyK_|Map_removeK_|Set_containsE_|Set_removeE_)\['
  erased='\.(get_t|get|containsKey|contains|remove_Object|indexOf|lastIndexOf)\('
  g="$t/go/closure/g_LibObj.go"
  fns() { awk -v re="$1" '$0 ~ "^func (\\(this \\*)?LibObj_(" re ")[[(]" {f=1} f {print} f && /^}/ {f=0}' "$g"; }
  nt=$(fns 'maps|change|keys|colls|lists|sets|strs|Reg.T.. look|typed' | { grep -oE "$twin" || true; } | wc -l)
  ny=$(fns 'maps|change|keys|colls|lists|sets|strs|Reg.T.. look|typed' | { grep -oE "$erased" || true; } | wc -l)
  nx=$(fns 'wide|erased|wild|obj' | { grep -oE "$twin" || true; } | wc -l)
  ne=$(fns 'wide|erased|wild|obj' | { grep -oE "$erased" || true; } | wc -l)
  if [[ $nt != 25 || $ny != 28 || $nx != 0 || $ne != 13 ]]; then echo "selftest: FAIL (Object-parameter methods: $nt twins and $ny erased calls in typed(), $nx and $ne in erased())"; exit 1; fi
fi
if [[ ! ,${J2GO_NOLIBGEN:-}, =~ ,(cmp|all), ]]; then
  # Rule "typed library interfaces", piece `cmp` (lang/Cmp.java): exactly the two classes that implement Comparable
  # of their own type (Ver, Box) have compareTo_t and the bridge; the six others keep the erased compareTo alone
  # (LibCmp's reasons; 7 compareTo methods: the two bridges, Base's dispatching stub and four, Derived has compareTo_impl);
  # direct() calls compareTo_t (1 call).
  g="$t/go/closure/g_Cmp.go"
  nd=$(grep -cE '^func \(this \*Cmp_[A-Za-z]+(\[[^]]*\])?\) compareTo_t\(' "$g" || true)
  nb=$(grep -cE '^	return this\.compareTo_t\(cast_\[\*Cmp_[A-Za-z]+(\[[^]]*\])?\]\(p0\)\)$' "$g" || true)
  ne=$(grep -cE '^func \(this \*Cmp_[A-Za-z]+(\[[^]]*\])?\) compareTo\(' "$g" || true)
  nc=$(awk '/^func Cmp_direct\(/,/^}/' "$g" | { grep -oE '\.compareTo_t\(' || true; } | wc -l)
  if [[ $nd != 2 || $nb != 2 || $ne != 7 || $nc != 1 ]]; then echo "selftest: FAIL (piece cmp: $nd compareTo_t, $nb bridges, $ne compareTo, $nc direct calls)"; exit 1; fi
fi
if [[ ${J2GO_NOFNTYPES:-} != 1 ]]; then
  # Rule "function types": the translator must refuse every use in refused/Refused.java (14) and write nothing.
  mkdir -p "$t/refused" && { cat "$t/files.txt"; echo tools/j2go/selftest/refused/Refused.java; } >"$t/refused/files.txt"
  n=$(java -Xss64m -Xmx8g --add-modules jdk.compiler -cp "${J2GO_CLASSES:-.cache/j2go}" j2go.Main ".cache/build/stock-libs.jar:.cache/build/main:$(ls .cache/deps/auto-value-annotations-*.jar)" "$t/refused" - @"$t/refused/files.txt" 2>&1 | grep -c '^function types: refused: Refused.java' || true)
  if [[ $n != 14 ]] || ls "$t/refused" | grep -q '^g_'; then echo "selftest: FAIL (function types: $n of 14 uses refused)"; exit 1; fi
fi
if [[ -z ${J2GO_NOLIBGEN:-} ]]; then
  # Rule "typed library interfaces" (every kind and piece on): the translator must refuse every marked use in refused/RefusedLib.java (15) and write nothing.
  mkdir -p "$t/refusedlib" && { cat .cache/j2go/files.txt; echo tools/j2go/selftest/refused/RefusedLib.java; } >"$t/refusedlib/files.txt"
  n=$(java -Xss64m -Xmx8g --add-modules jdk.compiler -cp "${J2GO_CLASSES:-.cache/j2go}" j2go.Main ".cache/build/stock-libs.jar:.cache/build/main:$(ls .cache/deps/auto-value-annotations-*.jar)" "$t/refusedlib" - @"$t/refusedlib/files.txt" 2>&1 | grep -c '^typed library interfaces: refused: RefusedLib.java' || true)
  if [[ $n != 15 ]] || ls "$t/refusedlib" | grep -q '^g_'; then echo "selftest: FAIL (typed library interfaces: $n of 15 uses refused)"; exit 1; fi
fi
echo "selftest: PASS"
