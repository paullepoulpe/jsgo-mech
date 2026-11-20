import sys,collections as C,re
S=sys.argv[1]
def rd(n): return [l.rstrip('\n').split('\t') for l in open(f'{S}/{n}.tsv')]
D=rd('decls'); F=rd('flows')
sh=lambda q:re.sub(r'com\.google\.javascript\.(jscomp|rhino)\.([a-z]+\.)*','',q.replace('java.util.function.','').replace('java.util.','').replace('java.lang.','').replace('com.google.common.collect.','').replace('com.google.common.base.','gb.').replace('com.google.common.','').replace('@org.jspecify.annotations.Nullable ',''))
READ={'iterator','hasNext','next','get','size','isEmpty','contains','containsAll','containsKey','containsValue','indexOf','stream','forEach','entrySet','keySet','values','getKey','getValue','getFirst','getLast','apply','test','accept','compare','toString','equals','hashCode','toArray','getOrDefault','peek','peekFirst','peekLast','first','last','asList','reverse','subList','isPresent','or','orNull','call','run','getSimpleName','getName','cast','isInstance','getEnumConstants','descendingIterator','lastIndexOf','listIterator'}
W=[r for r in D if r[6] in('extends','super','wild') and r[4]!='optional']
res=C.Counter(); det=C.defaultdict(list)
for r in W:
    u=r[13]
    kinds=C.Counter()
    if u!='-':
        for p in u.split(';'):
            k,n=p.rsplit('=',1); kinds[k]+=int(n)
    cls=set()
    for k in kinds:
        if k in('foreach','cmp-null','assigned'): cls.add('read')
        elif k.startswith('call:'): cls.add('read' if k[5:] in READ else 'write:'+k[5:])
        elif k=='flow': cls.add('flow')
        else: cls.add('other:'+k)
    if not kinds: v='unused'
    elif cls<={'read'}: v='read-only'
    elif cls<={'read','flow'}: v='read+flow'
    else: v='write/other'
    res[(r[4],r[6],r[3],v)]+=1
    det[v].append(f"{r[0]}:{r[1]} {r[2]} [{r[3]}] {sh(r[8])} typedNow={r[9]} go={r[10]} uses={u}")
print("== wildcard declarations (family, wildcard kind, decl kind, how used)")
tot=C.Counter()
for k,n in sorted(res.items()): print(f"  {n:4d} {k}"); tot[(k[0],k[3])]+=n
print(sorted(tot.items()))
for v in det:
    print(f"\n-- {v}: {len(det[v])}")
    for l in det[v][:int(sys.argv[2])]: print("   ",l[:300])
