#!/usr/bin/env bash
set -euo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; M=$S/mstep2.sh; G=$S/gate.sh
cd /home/user/jm-merge; P="cd tools/postflip && go run"
$M eqhash 88ce971 true
$M sbrt 069b28f git cherry-pick -n 069b28f
git cherry-pick 4e6eab9 >/dev/null
$M sbuild 4498322 "bash tools/postflip/sbuild/run.sh"
$M threads aff1e30 git cherry-pick -n aff1e30
$G misc
echo MERGE3 DONE
