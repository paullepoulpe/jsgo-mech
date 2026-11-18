import sys,re,collections,importlib.util,io,contextlib
sys.argv=['classify.py']
P='/tmp/claude-0/-home-user/a720bb77-2dda-5adf-9c58-a03e59d56cc8/scratchpad/ptest/'
src=open(P+'classify.py').read().split("rows=[l.rstrip")[0]
exec(src)
J='/home/user/jm-merge/java/src/'
out=open('jx.tsv','w')
cache={}
def line(pos):
    f,l=pos.rsplit(':',1)
    if f not in cache:
        try: cache[f]=open(J+f).read().split('\n')
        except Exception: cache[f]=[]
    L=cache[f]; l=int(l)
    return L[l-1].strip() if 0<l<=len(L) else ''
for l in open(P+'sites.tsv'):
    r=l.rstrip('\n').split('\t'); r+=['']*(14-len(r))
    d=dict(n=tuple(int(x) for x in r[:4]),kf=r[4],kt=r[5],frm=r[6],to=r[7],pos=r[8],frame=r[9],arg=r[10],res=r[11],note=r[12],head=r[13])
    k,s=classify(d)
    if k[0] in 'JX' and k[1]==' ':
        out.write('\t'.join([k[0],d['pos'],d['kf']+'->'+d['kt'],d['frm'],d['to'],d['head'][:60],line(d['pos'])])+'\n')
