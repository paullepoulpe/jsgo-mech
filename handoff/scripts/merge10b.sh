#!/usr/bin/env bash
set -euo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; M=$S/mstep2.sh; G=$S/gate.sh
cd /home/user/jm-merge; P="cd tools/postflip && go run"
$M thr3 1801f83 true
$M hhrt 8735e93 git cherry-pick -n 8735e93
git cherry-pick 5bcbb35 >/dev/null
$M eqhash3 d63e2ad "bash tools/postflip/eqhash/run.sh"
$M sbbytes eb7bbe3 git cherry-pick -n eb7bbe3
$G r6eqsb
$M exceptions f6699ed "$P ./exceptions ../../go/closure"
$M castfold3 a9e205b "$P ./castfold ../../go/closure"
$M named3 e409561 "$P ./casts -named ../../go/closure"
$M anyconvall ba1e6b3 "$P ./anyconv -all ../../go/closure"
$G r6final_noloops
cp /home/user/jm-merge/.cache/prune/mech $S/proto/m_noloops
$M loops e7adc0c "$P ./ranges -loops ../../go/closure"
$G r6final
cp /home/user/jm-merge/.cache/prune/mech $S/proto/m_loops
echo MERGE10 DONE
