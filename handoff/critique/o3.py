import sys,re,collections
P='/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad/ptest/'
src=open(P+'classify.py').read().split("rows=[l.rstrip")[0]
sys.argv=['x']; exec(src)
want=sys.argv[1:] 
import os
cnt=collections.defaultdict(collections.Counter); ex=collections.defaultdict(list)
for l in open(P+'sites.tsv'):
    r=l.rstrip('\n').split('\t'); r+=['']*(14-len(r))
    d=dict(n=tuple(int(x) for x in r[:4]),kf=r[4],kt=r[5],frm=r[6],to=r[7],pos=r[8],frame=r[9],arg=r[10],res=r[11],note=r[12],head=r[13])
    k,s=classify(d)
    code=k.split()[0]
    if code in('O3','O4','O2','O5','D','C','A'):
        callee=re.sub(r'#\d+ :.*','',d['arg'].replace('arg ',''))
        cnt[code][(d['kf'],callee if code!='D' else d['frm'][:50])]+=sum(d['n'])
        ex[code].append(d['pos']+' '+d['frm'][:60]+' | '+d['head'][:60])
for c in ('O3','O4','O2','O5'):
    print('==',c,sum(cnt[c].values()))
    bykf=collections.Counter()
    for (kf,cal),n in cnt[c].items(): bykf[kf]+=n
    print('  by static kind of the argument:',dict(bykf))
    for (kf,cal),n in cnt[c].most_common(14): print('   ',n,kf,cal[-90:])
