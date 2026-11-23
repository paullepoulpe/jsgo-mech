#!/usr/bin/env bash
# Translator self-test: the programs in lang/ must print the same transcript when run
# by Java and when translated to Go. lang/Handles.java exercises handle classes: its
# classes listed in selftest/handles.txt are plain classes on the JVM and handles in Go.
set -euo pipefail
cd "$(dirname "$0")/../../.."
# the methods of the self-test programs whose unchecked casts rule "typed library interfaces" lets pass
export J2GO_UNCHECKED_CASTS=tools/j2go/unchecked-casts.txt:tools/j2go/selftest/unchecked-casts.txt
t=.cache/j2go-selftest
rm -rf "$t" && mkdir -p "$t/java" "$t/go/closure" "$t/go/cmd/t"
# lang/Imm.java uses Guava (typed immutable collections).
javac -nowarn -cp .cache/build/stock-libs.jar -d "$t/java" tools/j2go/selftest/lang/*.java
for c in Lang Lang2 Handles Generics Generics2 Generics3 Colls Imm Unboxed Covariant CovariantIface Layout FnTypes LibGen; do env -u JAVA_TOOL_OPTIONS java -Dstdout.encoding=UTF-8 -cp "$t/java:.cache/build/stock-libs.jar" lang.$c; done >"$t/expected.txt"
# The test class is translated together with the whole tree, so that the runtime it needs is all there.
cp go/go.mod "$t/go/" && cp go/closure/rt_*.go tools/j2go/selftest/rt_*.go "$t/go/closure/" && rm -f "$t"/go/closure/*_test.go
[[ -d go/closure/resources ]] && cp -r go/closure/resources "$t/go/closure/"
{ cat .cache/j2go/files.txt; ls tools/j2go/selftest/lang/*.java; } >"$t/files.txt"
# Instrumented output, so that static initialization happens on first use as in Java.
J2GO_HANDLES=tools/j2go/handles.txt:tools/j2go/selftest/handles.txt J2GO_INIT_GUARDS=1 java -Xss64m -Xmx8g --add-modules jdk.compiler -cp "${J2GO_CLASSES:-.cache/j2go}" j2go.Main ".cache/build/stock-libs.jar:.cache/build/main:$(ls .cache/deps/auto-value-annotations-*.jar)" "$t/go/closure" - @"$t/files.txt" 2>&1 | grep -v '^Picked up' || true
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

func SelfTest() string { return Lang_run() + Lang2_run() + Handles_run() + Generics_run() + Generics2_run() + Generics3_run() + Colls_run() + Imm_run() + Unboxed_run() + Covariant_run() + CovariantIface_run() + Layout_run() + FnTypes_run() + LibGen_run() }

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
if [[ ${J2GO_NOFNTYPES:-} != 1 ]]; then
  # Rule "function types": the translator must refuse every use in refused/Refused.java (14) and write nothing.
  mkdir -p "$t/refused" && { cat "$t/files.txt"; echo tools/j2go/selftest/refused/Refused.java; } >"$t/refused/files.txt"
  n=$(java -Xss64m -Xmx8g --add-modules jdk.compiler -cp "${J2GO_CLASSES:-.cache/j2go}" j2go.Main ".cache/build/stock-libs.jar:.cache/build/main:$(ls .cache/deps/auto-value-annotations-*.jar)" "$t/refused" - @"$t/refused/files.txt" 2>&1 | grep -c '^function types: refused: Refused.java' || true)
  if [[ $n != 14 ]] || ls "$t/refused" | grep -q '^g_'; then echo "selftest: FAIL (function types: $n of 14 uses refused)"; exit 1; fi
fi
if [[ -z ${J2GO_NOLIBGEN:-} ]]; then
  # Rule "typed library interfaces" (every kind and piece on): the translator must refuse every marked use in refused/RefusedLib.java (10) and write nothing.
  mkdir -p "$t/refusedlib" && { cat .cache/j2go/files.txt; echo tools/j2go/selftest/refused/RefusedLib.java; } >"$t/refusedlib/files.txt"
  n=$(java -Xss64m -Xmx8g --add-modules jdk.compiler -cp "${J2GO_CLASSES:-.cache/j2go}" j2go.Main ".cache/build/stock-libs.jar:.cache/build/main:$(ls .cache/deps/auto-value-annotations-*.jar)" "$t/refusedlib" - @"$t/refusedlib/files.txt" 2>&1 | grep -c '^typed library interfaces: refused: RefusedLib.java' || true)
  if [[ $n != 10 ]] || ls "$t/refusedlib" | grep -q '^g_'; then echo "selftest: FAIL (typed library interfaces: $n of 10 uses refused)"; exit 1; fi
fi
echo "selftest: PASS"
