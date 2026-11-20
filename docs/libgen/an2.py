import sys,collections as C,re
S=sys.argv[1]
def rd(n): return [l.rstrip('\n').split('\t') for l in open(f'{S}/{n}.tsv')]
D=rd('decls'); F=rd('flows'); I=rd('impls'); U=rd('fnuses'); K=rd('casts')
sh=lambda q:re.sub(r'com\.google\.javascript\.(jscomp|rhino)\.([a-z]+\.)*','',q.replace('java.util.function.','').replace('java.util.','').replace('java.lang.','').replace('com.google.common.collect.','').replace('com.google.common.base.','gb.').replace('com.google.common.',''))
byid={r[11]:r for r in D}
FL=[r for r in F if r[3] in('init','assign','return','arg','lambdaret')]
def show(cat,lim=200):
    rows=[r for r in FL if r[4]==cat]
    print(f"\n== {cat}: {len(rows)}")
    c=C.Counter()
    for r in rows:
        c[(r[3],sh(r[11]) if r[3]=='arg' else ('decl:'+byid[r[14]][3] if r[14] in byid else '-'))]+=1
    for k,n in c.most_common(40): print(f"  {n:4d} {k[0]:9s} {k[1]}")
    for r in rows[:lim]:
        print(f"    {r[0]}:{r[1]} {r[2]} [{r[3]}] {sh(r[7])} -> {sh(r[8])} src={sh(r[10])[:70]} callee={sh(r[11])}")
which=sys.argv[2]
if which=='var':
    show('ext-sub'); show('sup-wider'); show('src-wild'); 
if which=='wild':
    show('wild-any',0); show('wild-same',0)
    rows=[r for r in FL if r[4]=='wild-any']
    c=C.Counter((r[5],sh(r[6])) for r in rows); print(c.most_common())
if which=='gm':
    print("== arg flows by tv-owner class of the callee's declared parameter (col own) x callee kind")
    c=C.Counter(); top=C.defaultdict(C.Counter)
    for r in FL:
        if r[3]!='arg' or r[4].startswith('to-'): continue
        c[(r[5],r[12])]+=1; top[(r[5],r[12])][sh(r[11]).split('#')[0]]+=1
    for k,n in sorted(c.items()): 
        print(f"  {n:5d} {k[0]:11s} {k[1]:28s} top: "+", ".join(f"{a}={b}" for a,b in top[k].most_common(6)))
if which=='src':
    print("== source kinds of flows into library generic places, by family")
    c=C.Counter()
    for r in FL:
        if r[4].startswith('to-'): continue
        s=r[10]; s=re.sub(r':\d+$','',s)
        if s.startswith('call:'):
            p=s.split(':'); s='call:'+p[1]+(':'+p[3] if len(p)>3 else '')
        c[(r[5],s)]+=1
    for k,n in sorted(c.items(),key=lambda x:(x[0][0],-x[1])): print(f"  {n:5d} {k[0]:11s} {k[1]}")
if which=='toobj':
    rows=[r for r in F if r[4].startswith('to-')]
    c=C.Counter((r[4],r[5],sh(r[11]).split('#')[0] if r[3]=='arg' else r[3]) for r in rows)
    print(len(rows)); 
    for k,n in c.most_common(45): print(f"  {n:4d} {k}")
if which=='casts':
    c=C.Counter((r[3],r[4]) for r in K); print(c)
    for r in K: print("   ",r[0]+':'+r[1],r[2],r[3],sh(r[6]),'->',sh(r[7]))
if which=='impls':
    c=C.Counter((r[4],sh(r[5]),r[6],r[3]) for r in I)
    for k,n in sorted(c.items(),key=lambda x:(x[0][0],-x[1])): print(f"  {n:4d} {k}")
if which=='fnuses':
    c=C.defaultdict(C.Counter)
    for r in U: c[sh(r[3])][r[4]]+=1
    tot=C.Counter()
    for k,v in sorted(c.items(),key=lambda x:-sum(x[1].values())):
        print(f"  {k:28s} {sum(v.values()):5d} "+" ".join(f"{a}={b}" for a,b in v.most_common())); tot.update(v)
    print(tot.most_common())
if which=='foreach':
    rows=[r for r in F if r[3]=='foreach']
    c=C.Counter((r[4],r[5],r[9]) for r in rows)
    for k,n in sorted(c.items()): print(f"  {n:5d} {k}")
    c=C.Counter((r[5],sh(r[6])) for r in rows)
    print(c.most_common(30))
    for r in rows:
        if r[4] in('wild','object','tvar-erased'): print("   ",r[0]+':'+r[1],r[2],sh(r[7]),'var:',sh(r[8]),r[9])
