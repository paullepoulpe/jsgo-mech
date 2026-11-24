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
# piece `ownwild`: the equals methods whose cast to C<?> was read (the tree's and lang/OwnWild.java's)
export J2GO_OWNWILD_EQUALS=tools/j2go/ownwild-equals.txt:tools/j2go/selftest/ownwild-equals.txt
t=.cache/j2go-selftest
rm -rf "$t" && mkdir -p "$t/java" "$t/go/closure" "$t/go/cmd/t"
# lang/Imm.java uses Guava (typed immutable collections).
javac -nowarn -cp .cache/build/stock-libs.jar -d "$t/java" tools/j2go/selftest/lang/*.java
for c in Lang Lang2 Handles Generics Generics2 Generics3 Colls Imm Unboxed Covariant CovariantIface Layout FnTypes LibGen LibFns GOpt LibObj Cmp OwnWild TriBox OwnIter HStatic Resid RtCls; do env -u JAVA_TOOL_OPTIONS java -Dstdout.encoding=UTF-8 -cp "$t/java:.cache/build/stock-libs.jar" lang.$c; done >"$t/expected.txt"
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

func SelfTest() string { return Lang_run() + Lang2_run() + Handles_run() + Generics_run() + Generics2_run() + Generics3_run() + Colls_run() + Imm_run() + Unboxed_run() + Covariant_run() + CovariantIface_run() + Layout_run() + FnTypes_run() + LibGen_run() + LibFns_run() + GOpt_run() + LibObj_run() + Cmp_run() + OwnWild_run() + TriBox_run() + OwnIter_run() + HStatic_run() + Resid_run() + RtCls_run() }

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
if [[ ! ,${J2GO_NOLIBGEN:-}, =~ ,(ownwild|all), ]]; then
  # Piece `ownwild` of rule "typed generics" (lang/OwnWild.java): Pos (a parameter Pos<?> of its own method) and Entry
  # (a listed equals) are Go generic types; the eight other classes keep their wildcard (GenScan.ownWild's reasons) and
  # stay erased; Entry's instanceof is the test of its instantiation (1).
  g="$t/go/closure/g_OwnWild.go"
  ny=$(grep -cE '^type OwnWild_(Pos|Entry)\[T [a-z]+\] struct' "$g" || true)
  nx=$(grep -cE '^type OwnWild_(Over|Mixed|Stat|Asg|Ref|Unl|Str|NoGuard) struct' "$g" || true)
  ni=$(grep -cF 'is_[*OwnWild_Entry[T]](' "$g" || true)
  if [[ $ny != 2 || $nx != 8 || $ni != 1 ]]; then echo "selftest: FAIL (piece ownwild: $ny typed, $nx erased, $ni typed tests)"; exit 1; fi
fi
if [[ ! ,${J2GO_NOLIBGEN:-}, =~ ,(nbox|all), && ${J2GO_NOFNTYPES:-} != 1 ]]; then
  # Piece `nbox` (lang/TriBox.java): the fields year, flag, seen and the static tri are Nbox[T] (3 + 1), the fields called,
  # counter, shown, a1, a2 stay any (5: a method called on it, ++, a string concatenation, == of two boxes); setYear, setTri,
  # opt, num take Nbox / function types at Nbox (4), with 5 lambdas func(v Nbox[..]); each, pass, opt2 keep Consumer[any]
  # (a method reference, passed on as a value, not a lambda: 3).
  g="$t/go/closure/g_TriBox.go"
  nf=$(grep -cE '^	[a-z_0-9]+ +Nbox\[' "$g" || true)
  ns=$(grep -cE '^var TriBox_[a-z]+ Nbox\[' "$g" || true)
  nl=$(grep -cE '^	(called|counter|shown|a1|a2) +any$' "$g" || true)
  nv=$(grep -cF 'func(v Nbox[' "$g" || true)
  np=$(grep -cE '^func .*\((y|b|get|set) (Nbox|Supplier\[Nbox|Consumer\[Nbox)' "$g" || true)
  nx=$(grep -cE '^func TriBox_(each|pass|opt2)\(c Consumer\[any\]\)' "$g" || true)
  # (piece `nboxarg` on: a method reference, a candidate passed on to a candidate are typed too: each, pass, opt2 take
  # Consumer[Nbox[..]], and pass's lambda is the sixth)
  wv=5 wx=3; [[ ! ,${J2GO_NOLIBGEN:-}, =~ ,(nboxarg|fns2), ]] && wv=6 wx=0
  if [[ $nf != 3 || $ns != 1 || $nl != 5 || $nv != $wv || $np != 4 || $nx != $wx ]]; then echo "selftest: FAIL (piece nbox: $nf $ns fields typed, $nl left, $nv lambdas, $np signatures, $nx left)"; exit 1; fi
fi
if [[ ! ,${J2GO_NOLIBGEN:-}, =~ ,(hstatic|all), && ${J2GO_NOFNTYPES:-} != 1 ]]; then
  # Piece `hstatic` (lang/HStatic.java): the static fields GETTERS (a builder chain of functions) and MAKERS of the handle
  # class H are typed collections (2); names stays erased (a value that is no new collection enters it); GETTERS' chain is the
  # typed builder (1), with no converting closure to Function[any, any] (0).
  g="$t/go/closure/g_HStatic.go"
  nt=$(grep -cE '^var HStatic_H_(GETTERS \*TMap\[\*HStatic_P, Function\[HStatic_H, any\]\]|MAKERS \*TList\[Supplier\[HStatic_H\]\])$' "$g" || true)
  ne=$(grep -cE '^var HStatic_H_names \*T' "$g" || true)
  nb=$(grep -cF 'newTMapBuilder_(tc_p_HStatic_P, tc_Function_HStatic_H__any_)' "$g" || true)
  nc=$(grep -cF 'func() Function[any, any] {' "$g" || true)
  if [[ $nt != 2 || $ne != 0 || $nb != 1 || $nc != 0 ]]; then echo "selftest: FAIL (piece hstatic: $nt typed, $ne names typed, $nb typed builders, $nc converting closures)"; exit 1; fi
fi
if [[ ! ,${J2GO_NOLIBGEN:-}, =~ ,(owniter|iter|enumset|all), ]]; then
  # Piece `owniter` (lang/OwnIter.java): six loops over Words, Countdown, Bag (classes with iterator_t) run on eachOf_, five
  # over EnumSet makers on the typed set's cursor; the loops over Row (a library superclass), Base (a subclass), SeqImpl and
  # Seq (an interface of the program) and a variable-arity EnumSet.of stay erased (5 foreach_).
  g="$t/go/closure/g_OwnIter.go"
  ny=$(grep -cE ':= eachOf_\[(string|any|\*OwnIter_Color)\]\(' "$g" || true)
  ns=$(grep -cE ':= tenumSet(AllOf|NoneOf|Of|OfArr|CopyOf)_\(tc_p_OwnIter_Color, .*\)\.each\(\)$' "$g" || true)
  nx=$(grep -cE ':= foreach_\(' "$g" || true)
  if [[ $ny != 6 || $ns != 5 || $nx != 5 ]]; then echo "selftest: FAIL (piece owniter: $ny typed cursors, $ns EnumSet cursors, $nx erased loops)"; exit 1; fi
fi
if [[ ! ,${J2GO_NOLIBGEN:-}, =~ ,(rtm|fns2|seqstr|all), && ${J2GO_NOFNTYPES:-} != 1 ]]; then
  # Pieces `rtm`, `fns2`, `seqstr` (lang/Resid.java): rtm() has 20 calls of the typed twins (BigInteger and Enum
  # compareTo_t 6 (and 3 compareTo_t of piece cmp), BitSet clone_t, subList / reverse of typed immutable lists 6, tailSetT_ 2,
  # containsValueT_ 2), fns2() 11 (setAll, comparing, transform read once, Splitter.split, putIfAbsent 4, replace 2,
  # singletonList), seqstr() 5 variants, the static comparators 2, Pair.compareTo 2 compare_s; left() none and 3 erased
  # calls (a key that is no String, a transform that is not read once, a value of another Go type).
  twin='\.(compareTo_t|compare_s|clone_t|subListImm_|reverseImm_|tailSetT_|containsValueT_|split_sT|lastIndexIn_s|collapseFrom_s)\(|\b(ImmutableList_(subList|reverse)T_|List_add_intT_|Arrays_setAllT_|Comparator_(comparing|thenComparing)S_|Iterables_transformT_|Map_putIfAbsent_sT_|Map_replace_sT_|Collections_singletonListT_)\[|\b(Ascii_equalsIgnoreCase_s|Integer_parseInt_s_int_int_int)\('
  erased='\bComparator_comparing\[|\bLists_transform\(|\.containsValue\('
  g="$t/go/closure/g_Resid.go"
  cnt() { awk -v re="$1" '$0 ~ "^func (\\(this \\*[A-Za-z_]+\\) )?(" re ")\\(" {f=1} f {print} f && /^}/ {f=0}' "$g" | { grep -oE "$2" || true; } | wc -l; }
  n1=$(cnt Resid_rtm "$twin"); n2=$(cnt Resid_fns2 "$twin"); n3=$(cnt Resid_seqstr "$twin"); n4=$(cnt Resid_clinit "$twin"); n5=$(cnt compareTo_t "$twin")
  nx=$(cnt Resid_left "$twin"); ne=$(cnt Resid_left "$erased")
  if [[ $n1 != 20 || $n2 != 11 || $n3 != 5 || $n4 != 2 || $n5 != 2 || $nx != 0 || $ne != 3 ]]; then echo "selftest: FAIL (pieces rtm/fns2/seqstr: $n1 $n2 $n3 $n4 $n5 twins, $nx and $ne erased calls in left())"; exit 1; fi
fi
if [[ ! ,${J2GO_NOLIBGEN:-}, =~ ,(rtcls|all), && ${J2GO_NOFNTYPES:-} != 1 ]]; then
  # Piece `rtcls` (lang/RtCls.java): the program's subclasses of AbstractIterator, CacheLoader and Ordering have the
  # overriding method at their type arguments (Countdown.computeNext, ByLength.compare, the anonymous Ordering's compare,
  # the anonymous CacheLoader's load: 4); ThreadLocal (field and maker: 2), TreeRangeSet.create (2), Range.closed (5) and
  # CacheLoader.from (1) are the typed runtime classes' own.
  g="$t/go/closure/g_RtCls.go"
  nm=$(grep -cE '^func \(this \*RtCls_[A-Za-z_0-9]+\) (computeNext\(\) string|compare\([a-z]+ (string|\*RtCls_Pos),|load\(key \*RtCls_Key\))' "$g" || true)
  nl=$(grep -oE 'ThreadLocalT\[' "$g" | wc -l); nr=$(grep -oE 'TreeRangeSetT_create\[' "$g" | wc -l)
  nc=$(grep -oE 'RangeT_closed\[' "$g" | wc -l); nf=$(grep -oE 'CacheLoaderT_from\[' "$g" | wc -l)
  if [[ $nm != 4 || $nl != 2 || $nr != 2 || $nc != 5 || $nf != 1 ]]; then echo "selftest: FAIL (piece rtcls: $nm typed methods, $nl $nr $nc $nf typed runtime calls)"; exit 1; fi
fi
if [[ ! ,${J2GO_NOLIBGEN:-}, =~ ,(annqual|ownwild|ownwild2|all), ]]; then
  # Pieces `annqual` and `ownwild2` (lang/RtCls.java): Lazy (a field `RtCls.@N Lazy<String>`, an equals of the expression
  # form, listed, one instantiation) is a Go generic type with the test of its instantiation (1); Two (listed, two
  # instantiations) and Unl (not listed) stay erased (2).
  g="$t/go/closure/g_RtCls.go"
  ny=$(grep -cE '^type RtCls_Lazy\[T [a-z]+\] struct' "$g" || true)
  nx=$(grep -cE '^type RtCls_(Two|Unl) struct' "$g" || true)
  ni=$(grep -cF 'is_[interface{ as_RtCls_Lazy() *RtCls_Lazy[T] }](' "$g" || true)
  if [[ $ny != 1 || $nx != 2 || $ni != 1 ]]; then echo "selftest: FAIL (pieces annqual/ownwild2: $ny typed, $nx erased, $ni typed tests)"; exit 1; fi
fi
if [[ ! ,${J2GO_NOLIBGEN:-}, =~ ,(nboxarg|nbox|fns2|all), && ${J2GO_NOFNTYPES:-} != 1 ]]; then
  # Piece `nboxarg` (lang/RtCls.java): Comparator.comparing with an Integer key is Comparator_comparingN_ (3), with a
  # Double key the erased-key twin (1); Pool's function field (a constructor parameter given a method reference and a
  # lambda) is Function[string, Nbox[int32]] (1); Other's (given a variable) stays Function[string, any] (1).
  g="$t/go/closure/g_RtCls.go"
  nn=$(grep -oE 'Comparator_comparingN_\[string, int32\]' "$g" | wc -l); ne=$(grep -oE 'Comparator_comparing\[string, any\]' "$g" | wc -l)
  np=$(grep -cE '^	indexFn +Function\[string, Nbox\[int32\]\]$' "$g" || true); no=$(grep -cE '^	fn +Function\[string, any\]$' "$g" || true)
  if [[ $nn != 3 || $ne != 1 || $np != 1 || $no != 1 ]]; then echo "selftest: FAIL (piece nboxarg: $nn typed comparing, $ne left, $np typed field, $no left)"; exit 1; fi
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
