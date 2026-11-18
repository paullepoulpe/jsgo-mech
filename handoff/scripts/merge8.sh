#!/usr/bin/env bash
set -euo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; M=$S/mstep2.sh; G=$S/gate.sh
cd /home/user/jm-merge; P="cd tools/postflip && go run"
git cherry-pick dadd3b1 >/dev/null
$M enumbodies 2ad54a4 "bash tools/postflip/enums/run.sh"
$M enummap 020782f git cherry-pick -n 020782f
$M rtenums d5f64a8 git cherry-pick -n d5f64a8
$G r6enums
git cherry-pick 1dd1925 >/dev/null
$M castfold a9e205b "$P ./castfold ../../go/closure"
$M nnt fb69094 git cherry-pick -n fb69094
$M castfold2 3100ea1 "$P ./castfold ../../go/closure"
git cherry-pick 2c33f70 >/dev/null
$M named 037619a "$P ./casts -named ../../go/closure"
$M instof b4924dc "$P ./castfold -instof ../../go/closure"
$M named2 e409561 "$P ./casts -named ../../go/closure"
$G r6casts
echo MERGE8 DONE
