#!/usr/bin/env bash
set -euo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; M=$S/mstep2.sh; G=$S/gate.sh
cd /home/user/jm-merge; P="cd tools/postflip && go run"
$M firstrt feccb53 git cherry-pick -n feccb53
git cherry-pick 70ec089 >/dev/null
$M ranges e7adc0c "$P ./ranges ../../go/closure"
$G r6colls
git cherry-pick 8dbcb54 f251bc3 >/dev/null
$M fpfix b4fc384 "bash tools/postflip/funcparams/run.sh"
$M tostrrt 5f37669 git cherry-pick -n 5f37669
git cherry-pick 383cfff >/dev/null
$M tostr 03e5397 "$P ./tostr ../../go/closure"
git cherry-pick 609b6fe >/dev/null
$M anyconv ba1e6b3 "$P ./anyconv ../../go/closure"
$M appendable 4301531 git cherry-pick -n 4301531
$M strkeys2 d02050b "$P ./strkeys ../../go/closure"
$G r6boxing
git cherry-pick ee5807f >/dev/null
$M checks ab52020 "$P ./checks ../../go/closure"
git cherry-pick 7ee8c97 b249562 >/dev/null
$M strnative e0865cf "$P ./strnative ../../go/closure"
$G r6precond
git cherry-pick 2651563 >/dev/null
echo MERGE9 DONE
