#!/usr/bin/env bash
set -euo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; M=$S/mstep2.sh; G=$S/gate.sh
while pgrep -f merge5.sh >/dev/null; do sleep 10; done
grep -q "MERGE5 DONE" $S/merge5.out || { echo "merge5 did not finish"; exit 1; }
cd /home/user/jm-merge; git fetch -q origin postflip5-r5-fsiv; P="cd tools/postflip && go run"
$M cmpintf e03e188 git cherry-pick -n e03e188
git cherry-pick 89d5cee >/dev/null
$M arrcopyof db429a9 "$P ./arraycopy ../../go/closure"
$M queuedin 449a191 git cherry-pick -n 449a191
$M ntroom fed0f5b git cherry-pick -n fed0f5b
$M astpos f40411d git cherry-pick -n f40411d
$G fsiv
echo MERGE6 DONE
