#!/usr/bin/env bash
# selftest variant for experiments: translator classes from $S/tgx/cls, extra test files from $S/st_extra/*.java
set -euo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
cd /home/user/jm-j2go
t=$S/tgx_st
rm -rf "$t" && mkdir -p "$t/java" "$t/go/closure" "$t/go/cmd/t" "$t/selftest/lang"
cp tools/j2go/selftest/lang/*.java "$t/selftest/lang/"; cp $S/st_extra/*.java "$t/selftest/lang/" 2>/dev/null || true
classes=$(cd $t/selftest/lang && ls *.java | sed 's/\.java$//' | tr '\n' ' ')
javac -nowarn -d "$t/java" $t/selftest/lang/*.java 2>&1 | grep -v '^Note\|Picked' || true
for c in $classes; do env -u JAVA_TOOL_OPTIONS java -Dstdout.encoding=UTF-8 -cp "$t/java" lang.$c; done >"$t/expected.txt"
cp go/go.mod "$t/go/" && cp go/closure/rt_*.go "$t/go/closure/" && (cp $S/rt_override/*.go "$t/go/closure/" 2>/dev/null || true) && rm -f "$t"/go/closure/*_test.go
cp -r go/closure/resources "$t/go/closure/"
{ cat .cache/j2go/files.txt; ls $t/selftest/lang/*.java; } >"$t/files.txt"
J2GO_HANDLES=tools/j2go/handles.txt:tools/j2go/selftest/handles.txt J2GO_INIT_GUARDS=1 java -Xss64m -Xmx8g --add-modules jdk.compiler -cp $S/tgx/cls j2go.Main ".cache/build/stock-libs.jar:.cache/build/main:$(ls .cache/deps/auto-value-annotations-*.jar)" "$t/go/closure" - @"$t/files.txt" 2>&1 | grep -v '^Picked up' | tail -2 || true
printf 'package main\n\nimport (\n\t"fmt"\n\n\t"jsgomech/closure"\n)\n\nfunc main() { fmt.Print(closure.SelfTest()) }\n' >"$t/go/cmd/t/main.go"
{ printf 'package closure\n\nfunc SelfTest() string { return ""'; for c in $classes; do printf ' + %s_run()' $c; done; printf ' }\n'; } >"$t/go/closure/rt_selftest.go"
(cd "$t/go" && gofmt -w closure/g_*.go && go build -o ../t ./cmd/t)
"$t/t" >"$t/actual.txt" || true
if diff "$t/expected.txt" "$t/actual.txt"; then echo "selftest: PASS"; else echo "selftest: FAIL"; exit 1; fi
