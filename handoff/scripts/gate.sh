#!/usr/bin/env bash
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; cd /home/user/jm-merge
find /root/.cache/go-build -type f -mmin +40 -delete 2>/dev/null
flock $S/gate.lock bash tools/prune/verify.sh > $S/gate-$1.log 2>&1
if [[ $(grep -c ": MATCH" $S/gate-$1.log) != 4 ]] || ! grep -q "^ok" $S/gate-$1.log; then tail -12 $S/gate-$1.log; echo "GATE FAILED $1"; exit 1; fi
echo "GATE OK $1 at $(git log --oneline -1 | cut -c1-60)"; for i in 1 2 3; do git push -q origin postflip5 2>&1 | tail -1 && break; sleep 5; done
