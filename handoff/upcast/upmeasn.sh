#!/bin/bash
# upmeasn.sh <outlabel> <rb> <rf> label...: memstats + timing of several builds in one interleaved run each, under the gate lock
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
o=$1; rb=$2; rf=$3; shift 3
bins=""; for l in "$@"; do bins="$bins $l=$S/proto/m_$l"; done
{
flock $S/gate.lock bash -c "python3 $S/proto/j2mem.py bundle $bins; python3 $S/proto/fb.py bundle $rb upE=$S/proto/m_tg_erased $bins"
flock $S/gate.lock bash -c "python3 $S/proto/j2mem.py typed-full $bins; python3 $S/proto/fb.py typed-full $rf upE=$S/proto/m_tg_erased $bins"
echo DONE
} > $S/upmeas_$o.log 2>&1
