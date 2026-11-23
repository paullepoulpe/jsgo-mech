# summary.py: the summary table and the RULE totals of JAVAISMS6.md from verdicts.tsv (classify6.py) and census.out
import collections
census=[l for l in open("census.out")]
counts=[int(l.split()[0]) for l in census]
names=["erased Java collection constructors","typed TList/TMap/TSet constructors","Guava immutables / builders","native Go slices made","native Go maps made","for-each via iterator objects","explicit Iterator use","streams","dispatch through self_.(interface)","anonymous interface assertions","interface casts `icast_X(`","class casts `to_X(` / `cast_[`","instanceof `is_X(`","type assertions `x.(T)`","type switches","boxing to any (explicit)","unboxing from any","lambda objects `&X_fn{func` (subset of next row)","lambda adapter literals","Optional","try/catch emulation (recover, catch_)","panic(","defer","equals/hashCode protocol","String_* UTF-16 helpers","StringBuilder","enum objects: struct","enum objects: call","Preconditions_","threads/executors","reflect","unsafe"]
v=collections.defaultdict(collections.Counter); rl=collections.defaultdict(collections.Counter)
hot={}
for l in open("sc.tsv"):
    f=l.split("\t"); hot[f[1]]=hot.get(f[1]) or f[3]=="H"
tot=collections.Counter(); h=collections.Counter()
for l in open("verdicts.tsv"):
    r,f,ln,ver,lab=l.rstrip("\n").split("\t")
    k="RULE" if ver.startswith("RULE") else ver
    v[int(r)][k]+=1
    if k=="RULE":
        rl[int(r)][ver.split()[1]]+=1
        if r!="17":
            tot[ver.split()[1]]+=1
            if hot.get(f): h[ver.split()[1]]+=1
print("| row | count | DONE | RULE | REASON | not a site | not split | RULE by rule |")
print("|---|---:|---:|---:|---:|---:|---:|---|")
ok=True
rows=list(enumerate(names))+[(100,"fields/params declared `any` (Object slots only)")]
for i,nm in rows:
    c=v[i]; n=sum(c.values()); want=counts[i] if i<100 else int(census[-1].split()[0])
    if n!=want: ok=False
    f=lambda k: f"{c[k]:,}" if c[k] else ""
    rr=", ".join("%s %d"%(a,b) for a,b in sorted(rl[i].items(), key=lambda x:-x[1]))
    print(f"| {nm} | {want:,} | {f('DONE')} | {f('RULE')} | {f('REASON')} | {f('not a site')} | {f('UNSPLIT')} | {rr} |")
print("\nall rows add up:",ok)
print("\nRULE totals (census matches; row 17 is a subset of row 18 and is not added):")
for k,n in tot.most_common(): print(f"  {k:12s} {n:5d}   in hot files {h[k]}")
