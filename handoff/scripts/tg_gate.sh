#!/usr/bin/env bash
# gate for main-r8-typed (jm-j2go): regenerate, then verify under the lock
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; cd /home/user/jm-j2go
find /root/.cache/go-build -type f -mmin +180 -delete 2>/dev/null
bash tools/j2go/run.sh > $S/tg_gate-$1.log 2>&1 || { tail $S/tg_gate-$1.log; echo "RUN FAILED"; exit 1; }
flock $S/gate.lock bash tools/prune/verify.sh >> $S/tg_gate-$1.log 2>&1
if [[ $(grep -c ": MATCH" $S/tg_gate-$1.log) != 4 ]] || ! grep -q "^ok" $S/tg_gate-$1.log || ! grep -q 'selftest: PASS' $S/tg_gate-$1.log; then tail -12 $S/tg_gate-$1.log; echo "GATE FAILED $1"; exit 1; fi
tail -9 $S/tg_gate-$1.log; echo "GATE OK $1"
