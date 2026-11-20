# summ.py <row> [cols...] : counts of sites.tsv for a census row grouped by the given columns
# columns: file line hot sub detail extra matched; extra can be split with extra0, extra1, extra2 (" | " parts)
import sys,collections
row=sys.argv[1]; cols=sys.argv[2:] or ["sub"]
top=1000
names=["row","file","line","hot","sub","detail","extra","matched"]
c=collections.Counter(); n=0
for l in open(__file__.rsplit("/",1)[0]+"/sites.tsv"):
    f=l.rstrip("\n").split("\t")
    if f[0]!=row: continue
    d=dict(zip(names,f))
    for i,p in enumerate(d["extra"].split(" | ")): d["extra%d"%i]=p
    filt=[a for a in cols if "=" in a]
    ok=True
    for a in filt:
        k,v=a.split("=",1)
        if k=="top": top=int(v); continue
        if v.startswith("~"):
            if v[1:] not in d.get(k,""): ok=False
        elif d.get(k,"")!=v: ok=False
    if not ok: continue
    n+=1
    c[tuple(d.get(a,"") for a in cols if "=" not in a)]+=1
print(n,"sites")
for k,v in c.most_common(top): print("%6d  %s"%(v," || ".join(k)))
