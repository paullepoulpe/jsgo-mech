#!/usr/bin/env bash
# tg_time.sh <label> <full rounds> <bundle rounds> <java:0|1> name=bin ... : timings under the lock
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; cd $S/proto
label=$1; fr=$2; br=$3; jv=$4; shift 4
J=/home/user/jsgo-mech/.cache/build/candidate.sh
(
flock 9
: > $S/tg_${label}_mem.txt
for a in "$@"; do case $a in *erased*|*prev*) ;; *) python3 -u j2mem.py typed-full $a >> $S/tg_${label}_mem.txt; python3 -u j2mem.py bundle $a >> $S/tg_${label}_mem.txt;; esac; done
[ $br -gt 0 ] && python3 -u fb.py bundle $br "$@" 2> $S/tg_${label}_bundle.runs
[ $fr -gt 0 ] && python3 -u fb.py typed-full $fr "$@" 2> $S/tg_${label}_full.runs
if [ $jv = 1 ]; then python3 -u fb.py bundle 1 ${label}java=$J 2> $S/tg_${label}_java.runs; python3 -u fb.py typed-full 1 ${label}java=$J 2>> $S/tg_${label}_java.runs; fi
) 9>>$S/gate.lock
cat $S/tg_${label}_mem.txt $S/tg_${label}_bundle.runs $S/tg_${label}_full.runs $S/tg_${label}_java.runs 2>/dev/null
