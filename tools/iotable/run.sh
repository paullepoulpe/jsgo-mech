#!/usr/bin/env bash
# Regenerates go/closure/testdata/io/*.tsv: what the Java Io facade (java/src/.../base/Io.java)
# returns for the inputs listed in IoTable.java. go/closure/rt_io_test.go checks the Go
# implementation (go/closure/rt_io.go) against these tables.
# Needs: tools/gate0/build.sh has compiled the Java tree into .cache/build.
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
cp="$root/.cache/build/main:$root/.cache/build/stock-libs.jar"
mkdir -p "$root/.cache/iotable" "$root/go/closure/testdata/io"
javac -nowarn -cp "$cp" -d "$root/.cache/iotable" "$root/tools/iotable/IoTable.java"
# In a UTF-8 locale (in the POSIX locale the JDK refuses file names that are not ASCII) and in an
# empty directory, where the generator builds the tree its glob table walks.
work=$(mktemp -d)
(cd "$work" && env -u JAVA_TOOL_OPTIONS LC_ALL=C.UTF-8 java -cp "$root/.cache/iotable:$cp" IoTable "$root/go/closure/testdata/io")
rm -rf "$work"
wc -l "$root"/go/closure/testdata/io/*.tsv
