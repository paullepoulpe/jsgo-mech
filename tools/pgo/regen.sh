#!/usr/bin/env bash
# Regenerates go/cmd/mech/default.pgo, the CPU profile that `go build` uses for
# profile-guided optimization of the translated compiler (Go reads default.pgo in the main
# package's directory by itself; `go build -pgo=off` ignores it).
#
# The profile is the merge of one CPU profile per gate project (tools/gate0/e2e.py: a
# type-check, a SIMPLE bundle, a transpilation to ES5 and an ADVANCED compile), taken with the
# binary that tools/prune/verify.sh built. Run it after a change to the translator or the
# runtime that moves much code; a stale profile only loses some of the gain.
#   tools/pgo/regen.sh && (cd go && go build ./...) && tools/prune/verify.sh
set -euo pipefail
root=$(cd "$(dirname "$0")/../.." && pwd)
bin=$root/.cache/prune/mechsafe
[[ -x $bin ]] || { echo "run tools/prune/verify.sh first"; exit 1; }
dir=$root/.cache/pgo
rm -rf "$dir" && mkdir -p "$dir"
cd "$root"
for c in checks bundle downlevel advanced; do
  JSGO_CPU_PROFILE=$dir/$c.prof python3 -I tools/gate0/e2e.py --go "$bin" "$c" | tail -1
done
go tool pprof -proto "$dir"/*.prof >"$dir/merged.pgo" 2>/dev/null
mv "$dir/merged.pgo" go/cmd/mech/default.pgo
ls -l go/cmd/mech/default.pgo
