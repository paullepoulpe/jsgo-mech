#!/usr/bin/env bash
# ABBA: base vs candidate, typed-full and bundle, tracer TIMING_ONLY, memstats
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
A=$S/proto/m_r6base; B=$S/bx_m_cand
cd $S/why
for task in typed-full bundle; do
i=0
for x in A B B A A B; do
  i=$((i+1)); bin=$A; [ $x = B ] && bin=$B
  python3 run.py $task bx_${x}$i JSGO_MEMSTATS=@D/memstats.json -- $bin ++ --tracer_mode=TIMING_ONLY
done
done
