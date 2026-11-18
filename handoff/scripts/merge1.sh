#!/usr/bin/env bash
set -euo pipefail
M=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad/mstep.sh
cd /home/user/jm-merge
P="cd tools/postflip && go run"
$M fshift c11aac4 git cherry-pick -n c11aac4
$M gedge b255176 git cherry-pick -n b255176
$M enumset 16f78b4 git cherry-pick -n 16f78b4
git cherry-pick e6986d8 5b4c978 >/dev/null
$M nlist 62394e4 "$P ./native -kind list ../../go/closure"
$M nmap 24e33d4 "$P ./native -kind map ../../go/closure"
$M nset cbd1c19 "$P ./native -kind set ../../go/closure"
$M nimm 8708575 "$P ./native -kind imm ../../go/closure"
git cherry-pick 6409415 >/dev/null
$M optional d2f2137 "git diff 0760e9c d2f2137 -- go/closure/rt_base.go go/closure/rt_core.go go/closure/rt_io.go go/closure/rt_io_test.go | git apply -3 && (cd tools/postflip && go run ./generics ../../go/closure)"
git cherry-pick b15d028 >/dev/null
$M arraycopy c0c7a33 "git diff 0760e9c c0c7a33 -- go/closure/rt_num.go | git apply -3 && (cd tools/postflip && go run ./arraycopy ../../go/closure)"
git cherry-pick 26b01a1 >/dev/null
$M lambdas 5d7622c "bash tools/postflip/lambdas/run.sh"
git cherry-pick fb11402 >/dev/null
$M vdispatch dd67202 "$P ./vdispatch -retype ../../go/closure"
git cherry-pick f29e193 >/dev/null
$M casts 338b19f "$P ./casts ../../go/closure"
git cherry-pick dd9186d >/dev/null
$M funcrt bfb2b24 git cherry-pick -n bfb2b24
$M funcparams a876cbd "bash tools/postflip/funcparams/run.sh"
git cherry-pick b59c751 >/dev/null
$M precond 2671de1 "$P ./precond ../../go/closure"
$M nodeprops 1d5ef95 git cherry-pick -n 1d5ef95
git log --oneline postflip4..HEAD
