#!/usr/bin/env bash
# tg_slice.sh <label> <prevbin> : after a gate: build m_tg<label>, census, memstats, interleaved timing java/erased/prev/this
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; J=/home/user/jsgo-mech/.cache/build/candidate.sh
l=$1; prev=$2; this=$S/proto/m_tg$l
cd /home/user/jm-j2go/go && go build -o $this ./cmd/mech || exit 1
(cd closure && python3 /home/user/jm-merge/tools/postflip/census.py > $S/census_$l.txt 2>&1)
cd $S/proto
flock $S/gate.lock bash -c "python3 -u j2mem.py typed-full ${l}x=$this > $S/tg_${l}_mem.txt; python3 -u j2mem.py bundle ${l}x=$this >> $S/tg_${l}_mem.txt
python3 -u fb.py bundle 3 erased=$S/proto/m_tg_erased prev=$prev this=$this 2> $S/tg_${l}_bundle.runs
python3 -u fb.py typed-full 2 erased=$S/proto/m_tg_erased prev=$prev this=$this 2> $S/tg_${l}_full.runs"
cat $S/tg_${l}_mem.txt $S/tg_${l}_bundle.runs $S/tg_${l}_full.runs
