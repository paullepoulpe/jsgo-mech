#!/bin/bash
# upmeas.sh <prevlabel> <thislabel> [bundle rounds] [typed-full rounds]: memstats + interleaved timing, under the gate lock
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
p=$1; t=$2; rb=${3:-3}; rf=${4:-2}
out=$S/upmeas_$t.log
{
flock $S/gate.lock python3 $S/proto/j2mem.py bundle $p=$S/proto/m_$p $t=$S/proto/m_$t
flock $S/gate.lock python3 $S/proto/j2mem.py typed-full $p=$S/proto/m_$p $t=$S/proto/m_$t
flock $S/gate.lock python3 $S/proto/fb.py bundle $rb upE=$S/proto/m_tg_erased $p=$S/proto/m_$p $t=$S/proto/m_$t
flock $S/gate.lock python3 $S/proto/fb.py typed-full $rf upE=$S/proto/m_tg_erased $p=$S/proto/m_$p $t=$S/proto/m_$t
echo DONE
} > $out 2>&1
