#!/usr/bin/env bash
set -euo pipefail
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; M=$S/mstep.sh
cd /home/user/jm-merge
P="cd tools/postflip && go run"
git merge -q --ff-only postflip5-r3-erased
(cd go && go build -o $S/proto/m_erased ./cmd/mech)
git cherry-pick c5f981c 5261184 >/dev/null
$M vsub f60dd3f "$P ./vdispatch -sub ../../go/closure"
$M venums 0b99c58 "$P ./vdispatch -enums ../../go/closure"
git cherry-pick 723c2ee fab3e25 >/dev/null
$M enums 612b0a5 "$P ./enums ../../go/closure"
git log --oneline postflip5@{u}..HEAD | cat
