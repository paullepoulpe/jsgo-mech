#!/usr/bin/env bash
# tg_bench.sh <label> <rounds> bin1=... bin2=... : typed-full timing (alternating) + memstats for typed-full and bundle
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; cd $S/proto
label=$1; rounds=$2; shift 2
flock $S/gate.lock bash -c "python3 -u fb.py typed-full $rounds $* 2> $S/tg_$label.runs; python3 -u j2mem.py typed-full $* > $S/tg_$label.mem; python3 -u j2mem.py bundle $* >> $S/tg_$label.mem"
cat $S/tg_$label.runs $S/tg_$label.mem
