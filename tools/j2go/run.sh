#!/usr/bin/env bash
# Builds j2go and translates the Java tree (java/src, without test helpers) into go/closure/g_*.go.
set -euo pipefail
cd "$(dirname "$0")/../.."
bash tools/fetch.sh
mkdir -p .cache/j2go go/closure
javac -nowarn -d .cache/j2go --add-modules jdk.compiler tools/j2go/src/j2go/*.java
# Classes listed in tools/j2go/handwritten.txt are not translated: the Go runtime implements
# them by hand (rt_*.go) and the translator sees them as library classes, from the classes that
# tools/gate0/build.sh compiled.
find java/src -name '*.java' -not -path '*/testing/*' | grep -v -F -f <(sed -e 's/[[:space:]]*#.*//' -e '/^$/d' tools/j2go/handwritten.txt) | LC_ALL=C sort >.cache/j2go/files.txt
[[ -d .cache/build/main ]] || bash tools/gate0/build.sh
order=tools/j2go/init-order.txt
[[ -f $order ]] || order=-
java -Xss64m -Xmx8g --add-modules jdk.compiler -cp .cache/j2go j2go.Main \
  .cache/build/stock-libs.jar:.cache/build/main go/closure "$order" @.cache/j2go/files.txt
