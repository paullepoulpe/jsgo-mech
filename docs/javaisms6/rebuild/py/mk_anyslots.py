# anyslots.tsv: the "slot" lines of matches_any.tsv split by kind (cwd = OUTDIR; reads tree/go/closure).
# Verbatim from the JAVAISMS6 session (it ran as an inline heredoc; not checked in); the counters it
# prints are diagnostics only.
import re,collections
src={}
rows=[]
for l in open("matches_any.tsv"):
    f,off,k,_,m=l.rstrip("\n").split("\t")
    if k!="slot": continue
    if f not in src: src[f]=open("tree/go/closure/"+f).read()
    t=src[f]; off=int(off)
    ln=t.count("\n",0,off)+1
    ls=t.rfind("\n",0,off)+1; line=t[ls:t.find("\n",off)]
    pre=t[ls:off]
    if re.match(r"\s+\w+\s+any\b",line) and "func" not in pre: kind="field"
    elif "func(" in pre[pre.find("func")+4:] or re.search(r"func\(", pre) and not re.match(r"func ",line.strip()): kind="funclit"
    elif line.startswith("func ("): kind="method"
    elif line.startswith("func "): kind="function"
    else: kind="other"
    name=""
    mm=re.match(r"func \([^)]*\) (\w+)",line) or re.match(r"func (\w+)",line)
    if mm: name=mm.group(1)
    rows.append((f,ln,kind,name,m,line.strip()[:160]))
c=collections.Counter((r[2]) for r in rows); print(c)
with open("anyslots.tsv","w") as o:
    for r in rows: o.write("\t".join(map(str,r))+"\n")
c2=collections.Counter(r[3] for r in rows if r[2]=="method"); print(c2.most_common(40))
c3=collections.Counter(r[0] for r in rows if r[2]=="funclit"); print(c3.most_common(10))
print([r for r in rows if r[2]=="field"][:40])
