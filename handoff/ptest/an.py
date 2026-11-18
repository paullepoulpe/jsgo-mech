import sys,collections,re
rows=[l.rstrip('\n').split('\t') for l in open('sites.tsv')]
col=int(sys.argv[1]); keyf=sys.argv[2]; n=int(sys.argv[3]) if len(sys.argv)>3 else 40
flt=sys.argv[4] if len(sys.argv)>4 else None
c=collections.Counter(); ex={}
for r in rows:
    r+=['']*(14-len(r))
    w=int(r[col])
    if w<=0: continue
    d=dict(b=r[0],c=r[1],ic=r[2],u=r[3],kf=r[4],kt=r[5],frm=r[6],to=r[7],pos=r[8],frame=r[9],arg=r[10],res=r[11],note=r[12],head=r[13])
    if flt and not eval(flt,{},d|{'re':re}): continue
    k=eval(keyf,{},d|{'re':re})
    c[k]+=w; ex.setdefault(k,[]).append(d['pos']+' '+d['head'][:50])
print(sum(c.values()))
for k,v in c.most_common(n): print(v,k,'|',ex[k][0],'|',ex[k][len(ex[k])//2])
