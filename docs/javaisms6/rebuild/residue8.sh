#!/bin/bash
# residue8.sh ACCOUNTING_DIR OUTDIR: RESIDUE8.md's classification of the sites the eighth flip's accounting still marks
# RULE T1/T2/T3/T4/R4/R5 (docs/javaisms6/RESIDUE8.md).
#   ACCOUNTING_DIR  run.sh's output on the eighth flip's tree ($B/runs/jism-flip8, made by
#                   `docs/javaisms6/rebuild/run.sh $B/wt/tree8/go/closure $B/runs/jism-flip8`)
#   OUTDIR          gets base/ and main/ (two translations of this worktree), residue8.tsv, residue8.out
# Translates twice (tools/j2go/run.sh: Java with an 8 GB heap; run it as one heavy job, under the lock):
#   base/: J2GO_NOLIBGEN=ownwild,nbox,proto  = the translator the eighth flip was made from (main f47fbe67), md5 fab9e90c
#   main/: J2GO_NOLIBGEN=proto              = main 73e902a8's default translation, md5 13f040cd
# (md5 of `cat g_*.go`; the script stops if either differs). Leaves go/closure with the default translation.
set -euo pipefail
[ $# -eq 2 ] || { echo "usage: $0 ACCOUNTING_DIR OUTDIR" >&2; exit 2; }
H=$(cd "$(dirname "$0")" && pwd)
ACC=$(cd "$1" && pwd)
mkdir -p "$2"; O=$(cd "$2" && pwd)
cd "$H/../../.."
tx() {  # tx DIR MD5 [VAR=val]
  local d=$1 want=$2; shift 2
  ( for kv in "$@"; do export "$kv"; done; bash tools/j2go/run.sh > "$O/$(basename "$d").log" 2>&1 )
  rm -rf "$d"; mkdir -p "$d"; cp go/closure/g_*.go "$d/"
  local got; got=$(cat "$d"/g_*.go | md5sum | cut -c1-32)
  echo "$(basename "$d"): $got"
  [[ $got == "$want"* ]] || { echo "$(basename "$d"): want $want" >&2; exit 1; }
}
tx "$O/base" fab9e90c J2GO_NOLIBGEN=ownwild,nbox,proto
tx "$O/main" 13f040cd J2GO_NOLIBGEN=proto
bash tools/j2go/run.sh > "$O/default.log" 2>&1
cd "$O" && python3 "$H/py/residue8.py" "$ACC" "$O/base" "$O/main" > residue8.out
echo "residue8.out, residue8.tsv in $O"
