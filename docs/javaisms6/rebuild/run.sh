#!/bin/bash
# run.sh TREE_GO_CLOSURE_DIR OUTDIR
# Replays the JAVAISMS6.md (docs/javaisms6) Java-ism accounting pipeline on a go/closure directory:
#   census.py -> census.out; matches.py -> matches.tsv; jclass -> sites.tsv; ctx -> ctx.tsv -> sc.tsv;
#   boxcensusd (BOXDUMP) -> boxsites.tsv; boxjoin.py -> box_joined.tsv, boxjoin.out; -> box_sc.tsv;
#   mk_matches_any.py -> matches_any.tsv; mk_anyslots.py -> anyslots.tsv;
#   classify6.py -> verdicts.tsv, verdict_tables.out; summary.py -> summary.out.
# The tree is only read (tools load it with go/types; OUTDIR/tree/go/closure is a symlink to it, for classify6.py).
set -euo pipefail
[ $# -eq 2 ] || { echo "usage: $0 TREE_GO_CLOSURE_DIR OUTDIR" >&2; exit 2; }
H=$(cd "$(dirname "$0")" && pwd)
TREE=$(cd "$1" && pwd)
mkdir -p "$2"; O=$(cd "$2" && pwd)
source /mnt/data/ubuntu/jm/env.sh
export GOFLAGS=-mod=mod
P=$H/py; BIN=$O/bin
step() { echo "[$(date +%T)] $*" >&2; }

step build jclass ctx boxcensusd
mkdir -p "$BIN"
(cd "$H/tools" && for t in jclass ctx boxcensusd; do go build -o "$BIN/$t" ./$t; done)

mkdir -p "$O/tree/go"; ln -sfn "$TREE" "$O/tree/go/closure"
cd "$O"
cp "$P/boxjoin.py" "$P/classify6.py" "$P/summary.py" .

step census.py
(cd "$TREE" && python3 "$P/census.py") > census.out
step matches.py
(cd "$TREE" && python3 "$P/matches.py" "$P/census.py") > matches.tsv
step jclass
"$BIN/jclass" "$TREE" matches.tsv > sites.tsv 2> jclass.err
step ctx
awk -F'\t' '{print $2"\t"$3"\t"$1"\t"$4}' matches.tsv | "$BIN/ctx" "$TREE" > ctx.tsv 2> ctx.err
paste sites.tsv <(cut -f5- ctx.tsv) > sc.tsv
step boxcensusd
BOXDUMP=$O/boxsites.tsv "$BIN/boxcensusd" -static "$TREE" > boxcensus.out 2>&1
step boxjoin.py
python3 boxjoin.py -v > boxjoin.out
paste <(awk -F'\t' '$1==15' sc.tsv) box_joined.tsv > box_sc.tsv
awk -F'\t' '$2!=$12||$3!=$13{bad++} END{if(bad){print "box_sc.tsv: "bad" misjoined lines" > "/dev/stderr"; exit 1}}' box_sc.tsv
step matches_any / anyslots
(cd "$TREE" && python3 "$P/mk_matches_any.py") > matches_any.tsv
python3 "$P/mk_anyslots.py" > anyslots.log
step classify6.py
python3 classify6.py > verdict_tables.out
step summary.py
python3 summary.py > summary.out
step done: $O
