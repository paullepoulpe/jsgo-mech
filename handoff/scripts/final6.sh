#!/usr/bin/env bash
S=/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad; cd $S/proto; J=/home/user/jsgo-mech/.cache/build/candidate.sh
flock $S/gate.lock python3 -u fb.py typed-full 6 java=$J noloops=$S/proto/m_noloops loops=$S/proto/m_loops 2> $S/f6_tf.runs
flock $S/gate.lock python3 -u fb.py bundle 6 java=$J noloops=$S/proto/m_noloops loops=$S/proto/m_loops 2> $S/f6_b.runs
python3 - <<'PY'
import re,statistics,collections
for t in ('f6_tf','f6_b'):
    w=collections.defaultdict(dict)
    for l in open(f'/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad/{t}.runs'):
        m=re.match(r'\s+\S+ (\S+) wall=(\S+) cpu=\S+ round=(\d+)',l)
        if m: w[int(m[3])][m[1]]=float(m[2])
    J=[w[k]['java'] for k in w]
    for b in ('noloops','loops'):
        r=[w[k]['java']/w[k][b] for k in sorted(w)]; G=[w[k][b] for k in w]
        print(t,b,"Java/Go per round",' '.join(f'{x:.3f}' for x in r),"| medians %.3f mean %.3f (java %.2f go %.2f)"%(statistics.median(J)/statistics.median(G),statistics.mean(r),statistics.median(J),statistics.median(G)))
    d=[100*(w[k]['loops']-w[k]['noloops'])/w[k]['noloops'] for k in sorted(w)]; print(t,"loops vs noloops",' '.join(f'{x:+.1f}' for x in d),"mean %+.2f%%"%statistics.mean(d))
PY
for t in typed-full bundle; do for l in java noloops loops; do echo "$t $l $(cat fb/$t/$l/out.js fb/$t/$l/stdout fb/$t/$l/stderr | sha256sum | cut -c1-12)"; done; done
echo F6 DONE
