#!/usr/bin/env bash
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
while pgrep -f "fb.py typed-full 4" >/dev/null; do sleep 10; done
cd $S/proto
J=/home/user/jsgo-mech/.cache/build/candidate.sh; G=$S/proto/m_funcparams2
python3 -u fb.py typed-full 8 java=$J final=$G 2> $S/h2h_tf.runs
python3 -u fb.py bundle 8 java=$J final=$G 2> $S/h2h_b.runs
cd $S/why; for r in 1 2; do python3 run.py typed-full f5_t$r -- $G ++ --tracer_mode=TIMING_ONLY; python3 run.py typed-full j5_t$r -- $J ++ --tracer_mode=TIMING_ONLY; done
echo H2H DONE
