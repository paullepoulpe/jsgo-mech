#!/usr/bin/env bash
# Checks the Go build: self-test, runtime unit tests (tests of removed functions are dropped
# first and listed), and the four byte comparisons against Java.
#   tools/prune/verify.sh [e2e case ...]
set -uo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
bin=$root/.cache/prune
mkdir -p "$bin"
gobin=$(go env GOPATH)/bin
cd "$root"
bash tools/j2go/selftest/run.sh 2>&1 | tail -1
cd go
python3 ../tools/prune/dropbroken.py --tests | tail -3
"$gobin/goimports" -w closure/rt_*_test.go 2>/dev/null
# (J2GO_NOFNTYPES=1: the tree was translated without rule "function types"; tools/j2go/README.md)
tags=()
[[ ${J2GO_NOFNTYPES:-} == 1 ]] && tags=(-tags j2go_nofntypes)
# (J2GO_NOLIBGEN=...: a piece of rule "typed library interfaces" is off; the erased functions that only such a translation names: rt_lg_erased.go)
[[ -n ${J2GO_NOLIBGEN:-} ]] && tags=(-tags "${tags[1]:+${tags[1]},}j2go_nolibgen")
go test "${tags[@]}" ./closure 2>&1 | grep -v "Fatal Error" | cut -c1-200 | tail -4
go build "${tags[@]}" -o "$bin/mech" ./cmd/mech || exit 1
# The wrapper keeps a runaway compile from taking the machine down.
printf '#!/bin/sh\nulimit -v 9000000\nexport GOMEMLIMIT=6GiB\nexec timeout 900 %s "$@"\n' "$bin/mech" >"$bin/mechsafe"
chmod +x "$bin/mechsafe"
cd ..
python3 -I tools/gate0/e2e.py --go "$bin/mechsafe" "$@" 2>&1 | tail -4
echo "runtime lines: $(ls go/closure/rt_*.go | grep -v '_test\|proto_gen' | xargs cat | wc -l)  test lines: $(cat go/closure/rt_*_test.go | wc -l)"
# the default build's runtime, and the files that only a translation with a switch off compiles (build tags j2go_nofntypes, j2go_nolibgen)
echo "runtime lines of the default build: $(ls go/closure/rt_*.go | grep -v '_test\|proto_gen' | xargs grep -L '^//go:build j2go' | xargs cat | wc -l)  off-mode extras (build-tagged files): $(ls go/closure/rt_*.go | grep -v '_test\|proto_gen' | xargs grep -l '^//go:build j2go' | xargs cat | wc -l)"
