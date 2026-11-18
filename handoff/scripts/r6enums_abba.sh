#!/bin/bash
cd /tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad/why
for r in 1 2 3; do
  for b in base new new base; do
    if [ $b = base ]; then bin=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad/r6enums/m_base; else bin=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad/r6enums/m_new; fi
    python3 run.py typed-full r6e_${b}_$r$RANDOM JSGO_MEMSTATS=@D/memstats.json -- /tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad/why/lim.sh $bin ++ --tracer_mode=TIMING_ONLY
  done
done
