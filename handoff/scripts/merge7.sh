#!/usr/bin/env bash
set -euo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; M=$S/mstep2.sh; G=$S/gate.sh
while pgrep -f merge6.sh >/dev/null; do sleep 10; done
grep -q "MERGE6 DONE" $S/merge6.out || { echo "merge6 did not finish"; exit 1; }
cd /home/user/jm-merge; git fetch -q origin postflip5-r5-inlvar
$M firstlive 31c9580 git cherry-pick -n 31c9580
$M propscol 2d21e0f git cherry-pick -n 2d21e0f
$G inlvar
echo MERGE7 DONE
