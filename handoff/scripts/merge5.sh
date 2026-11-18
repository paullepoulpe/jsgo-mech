#!/usr/bin/env bash
set -euo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; M=$S/mstep2.sh; G=$S/gate.sh
cd /home/user/jm-merge; P="cd tools/postflip && go run"
$M enumcopy ecf1363 git cherry-pick -n ecf1363
$M strunitsrt 1b68f8a git cherry-pick -n 1b68f8a
git cherry-pick 8a7745e >/dev/null
$M strunits bce8c44 "$P ./strunits ../../go/closure"
$G codegen
echo MERGE5 DONE
