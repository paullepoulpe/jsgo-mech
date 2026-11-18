#!/usr/bin/env bash
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
cd /home/user/jm-merge
M=$S/mstep2.sh
pick() { bash $M "$1" "$2" "git cherry-pick -x $2" || exit 1; }
run() { local l=$1 src=$2; shift 2; bash $M "$l" "$src" "$*" || exit 1; }
T="cd tools/postflip &&"
pick s_7575 75750583; pick s_eb06 eb0679d9
run s_view c7382040 "($T go run ./strview ../../go/closure && gofmt -w ../../go/closure/g_*.go && go run ./strnative ../../go/closure)"
pick s_6bd3 6bd31eb1
pick s_743e 743eb4ff; pick s_1864 18641ee1
run s_view2 067ee34e "($T go run ./strview ../../go/closure)"
bash $S/gate.sh r8str || exit 1
echo "R8 STR DONE"
