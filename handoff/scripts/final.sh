#!/usr/bin/env bash
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad
while pgrep -f merge7.sh >/dev/null; do sleep 10; done
grep -q "MERGE7 DONE" $S/merge7.out || { echo "merge7 did not finish"; exit 1; }
cd /home/user/jm-merge/go && go build -o $S/proto/m_final ./cmd/mech || exit 1
cd $S/proto; J=/home/user/jsgo-mech/.cache/build/candidate.sh
flock $S/gate.lock python3 -u fb.py typed-full 8 java=$J go=$S/proto/m_final 2> $S/final_tf.runs
flock $S/gate.lock python3 -u fb.py bundle 6 java=$J go=$S/proto/m_final 2> $S/final_b.runs
python3 - <<'PY'
import re,statistics,collections
for t in ('final_tf','final_b'):
    w=collections.defaultdict(dict)
    for l in open(f'/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad/{t}.runs'):
        m=re.match(r'\s+\S+ (\S+) wall=(\S+) cpu=\S+ round=(\d+)',l)
        if m: w[int(m[3])][m[1]]=float(m[2])
    r=[w[k]['java']/w[k]['go'] for k in sorted(w)]; J=[w[k]['java'] for k in w]; G=[w[k]['go'] for k in w]
    print(t,"Java/Go per round", ' '.join(f'{x:.3f}' for x in r))
    print("   java median %.2f min %.2f | go median %.2f min %.2f | Java/Go medians %.3f, mean %.3f, mins %.3f"%(statistics.median(J),min(J),statistics.median(G),min(G),statistics.median(J)/statistics.median(G),statistics.mean(r),min(J)/min(G)))
PY
for t in typed-full bundle; do for l in java go; do echo "$t $l $(cat fb/$t/$l/out.js fb/$t/$l/stdout fb/$t/$l/stderr | sha256sum | cut -c1-12)"; done; done
echo FINAL DONE
