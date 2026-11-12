#!/usr/bin/env bash
# Translator self-test: lang/Lang.java must print the same transcript when run
# by Java and when translated to Go.
set -euo pipefail
cd "$(dirname "$0")/../../.."
t=.cache/j2go-selftest
rm -rf "$t" && mkdir -p "$t/java" "$t/go/closure" "$t/go/cmd/t"
javac -nowarn -d "$t/java" tools/j2go/selftest/lang/Lang.java
env -u JAVA_TOOL_OPTIONS java -Dstdout.encoding=UTF-8 -cp "$t/java" lang.Lang >"$t/expected.txt"
# The test class is translated together with the whole tree, so that the runtime it needs is all there.
cp go/go.mod "$t/go/" && cp go/closure/rt_*.go "$t/go/closure/" && rm -f "$t"/go/closure/*_test.go
[[ -d go/closure/resources ]] && cp -r go/closure/resources "$t/go/closure/"
{ cat .cache/j2go/files.txt; echo tools/j2go/selftest/lang/Lang.java; } >"$t/files.txt"
java -Xss64m -Xmx8g --add-modules jdk.compiler -cp "${J2GO_CLASSES:-.cache/j2go}" j2go.Main ".cache/build/stock-libs.jar:$(ls .cache/deps/auto-value-annotations-*.jar)" "$t/go/closure" - @"$t/files.txt" 2>&1 | grep -v '^Picked up' || true
cat >"$t/go/cmd/t/main.go" <<'EOF'
package main

import (
	"fmt"

	"jsgomech/closure"
)

func main() { fmt.Print(closure.SelfTest()) }
EOF
cat >"$t/go/closure/rt_selftest.go" <<'EOF'
package closure

func SelfTest() string { return Lang_run() }
EOF
(cd "$t/go" && gofmt -w closure/g_*.go && go build -o ../t ./cmd/t)
"$t/t" >"$t/actual.txt" || true
if diff "$t/expected.txt" "$t/actual.txt"; then echo "selftest: PASS"; else echo "selftest: FAIL"; exit 1; fi
