#!/usr/bin/env bash
# Regenerates tools/j2go/init-order.txt: builds an instrumented copy of the
# translation in which every function first initializes the classes whose
# statics it touches, runs all static initializers, and records the order in
# which they completed. The normal build then runs them in that order without
# any guards.
set -euo pipefail
cd "$(dirname "$0")/../.."
tmp=.cache/j2go-initorder
rm -rf "$tmp" && mkdir -p "$tmp/closure" "$tmp/cmd/mech"
cp go/go.mod "$tmp/" && cp go/cmd/mech/main.go "$tmp/cmd/mech/" && cp go/closure/rt_*.go "$tmp/closure/"
[[ -f go/go.sum ]] && cp go/go.sum "$tmp/"
J2GO_INIT_GUARDS=1 bash tools/j2go/regen.sh "$tmp/closure" >/dev/null
(cd "$tmp" && go build -o mech ./cmd/mech)
"$tmp/mech" -init-order >tools/j2go/init-order.txt 2>"$tmp/failures.txt" || true
echo "init order: $(wc -l <tools/j2go/init-order.txt) classes, $(wc -l <"$tmp/failures.txt") failures (see $tmp/failures.txt)"
