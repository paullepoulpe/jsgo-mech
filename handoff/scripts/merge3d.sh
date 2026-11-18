#!/usr/bin/env bash
set -euo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; M=$S/mstep2.sh; G=$S/gate.sh
cd /home/user/jm-merge; P="cd tools/postflip && go run"
$M lookup 23e422f true
for c in 7032e7e:blockstk c8636a8:scopestk; do $M ${c#*:} ${c%:*} git cherry-pick -n ${c%:*}; done
$G refcoll
git cherry-pick 5dc2738 >/dev/null
$M objslots 7a56fbd "$P ./objslots ../../go/closure"
$M pmaprt 150574c git cherry-pick -n 150574c
$M pmap 8a921ed git cherry-pick -n 8a921ed
$M csrt f9f5bd8 git cherry-pick -n f9f5bd8
git cherry-pick 432a123 >/dev/null
$M charseq 59923b9 "$P ./charseq ../../go/closure"
$M strkeysm 98e5d8a git cherry-pick -n 98e5d8a
git cherry-pick 3f10cd1 >/dev/null
$M strkeys 2fae76b "$P ./strkeys ../../go/closure"
$M containsrt c63b896 git cherry-pick -n c63b896
$G boxing
$M fprt d54d116 git cherry-pick -n d54d116
git cherry-pick 186858e >/dev/null
$M funcparams2 a1f4d84 "bash tools/postflip/funcparams/run.sh"
$M eqrt 3b2409e git cherry-pick -n 3b2409e
git cherry-pick bbff84a >/dev/null
$M eqhash 88ce971 "bash tools/postflip/eqhash/run.sh"
$M sbrt 069b28f git cherry-pick -n 069b28f
git cherry-pick 4e6eab9 >/dev/null
$M sbuild 4498322 "bash tools/postflip/sbuild/run.sh"
$M threads aff1e30 git cherry-pick -n aff1e30
$G misc
echo MERGE3 DONE
