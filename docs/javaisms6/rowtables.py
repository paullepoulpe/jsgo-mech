# rowtables.py: verdict_tables.out (classify6.py) as the per-row markdown tables of JAVAISMS6.md, sub-causes with one stem merged
import re,collections
names={0:"erased Java collection constructors",1:"typed TList/TMap/TSet constructors",2:"Guava immutables / builders",3:"native Go slices made",5:"for-each via iterator objects",6:"explicit Iterator use",10:"interface casts icast_X(",11:"class casts to_X( / cast_[",12:"instanceof is_X(",13:"type assertions x.(T)",14:"type switches",15:"boxing to any (explicit)",16:"unboxing from any",17:"lambda objects &X_fn{func",18:"lambda adapter literals",19:"Optional",20:"try/catch emulation",21:"panic(",22:"defer",23:"equals/hashCode protocol",24:"String_* helpers",25:"StringBuilder",26:"enum objects: struct",27:"enum objects: call",28:"Preconditions_",29:"threads/executors",30:"reflect",31:"unsafe",100:"fields/params declared any (Object slots)"}
STEM=r"(erased runtime library function|value of an erased library type|Object in Java|Object slot holding values of two or more types in Java|Object or interface result in Java|argument of an erased runtime library function|erased Guava immutable or builder|bridge of a class implementing a library interface|Object field holding two or more types|typed TList/TMap/TSet constructor)[: ]*\(?(.*?)\)?$"
out=[]; buf=collections.OrderedDict()
def flush():
    for (v,base),items in buf.items():
        n=sum(c for _,c in items)
        if len(items)==1 and items[0][0]=="": out.append(f"| {v} | {base} | {n} |")
        else: out.append(f"| {v} | {base}: " + ", ".join(f"{k} {c}" for k,c in sorted(items,key=lambda x:-x[1])) + f" | {n} |")
    buf.clear()
for l in open("verdict_tables.out"):
    l=l.rstrip("\n")
    m=re.match(r"### row (\d+): (\d+)  \((.*)\)",l)
    if m:
        flush(); r=int(m.group(1))
        out.append(f"\n#### Row {r}: {names.get(r,r)}, {int(m.group(2)):,} ({m.group(3).replace('UNSPLIT','not split')})\n\n| verdict | sub-cause | sites |\n|---|---|---:|"); continue
    m=re.match(r"\| (.+?) \| (.+) \| (\d+) \|$",l)
    if m:
        v,lab,n=m.group(1).replace("UNSPLIT","not split"),m.group(2),int(m.group(3))
        mm=re.match(STEM,lab)
        if mm: buf.setdefault((v,mm.group(1)),[]).append((mm.group(2).strip(":( "),n))
        else: buf.setdefault((v,lab),[]).append(("",n))
flush()
print("\n".join(out))
