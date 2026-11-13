#!/usr/bin/env bash
# Re-runs the translator without rebuilding it, into another output directory.
# Used to refresh the generated placeholders after adding runtime code there.
#   tools/j2go/regen.sh <outdir>          (J2GO_CLASSES overrides the translator's class directory)
set -euo pipefail
out=$(cd "$1" && pwd)
cd "$(dirname "$0")/../.."
order=tools/j2go/init-order.txt
[[ -f $order ]] || order=-
java -Xss64m -Xmx8g --add-modules jdk.compiler -cp "${J2GO_CLASSES:-.cache/j2go}" j2go.Main \
  .cache/build/stock-libs.jar:.cache/build/main "$out" "$order" @.cache/j2go/files.txt 2>&1 | grep -v '^Picked up'
gofmt -w "$out"/g_*.go
