import sys,collections,re
def strip_ann(s): return re.sub(r'@[\w.]+\s*','',s)
def classify(d):
    f=d['frame'].split('<'); f0=f[0]; f1=f[1] if len(f)>1 else ''; f2=f[2] if len(f)>2 else ''
    kf,kt=d['kf'],d['kt']; arg=strip_ann(d['arg']); res=strip_ann(d['res']); head=d['head']
    b,c,ic,u=d['n']
    def ptype(s):
        m=re.match(r'(arg|result) (\S+?)(#\d+)? : (.*) \[(src|lib)\]$',s)
        return (m.group(2),m.group(4),m.group(5)) if m else None
    if f0.startswith('tconv'): return 'T typed-collection element converter (glue to the erased view)',''
    if f0.startswith('recordMembers'): return 'R record equals/hashCode/toString written through Object',kf
    if f0.startswith('forEach') or f1.startswith('forEach'):
        return 'F for-each over an erased collection (source as any + element cast)', re.sub(r'<.*','',d['frm']) if f0=='forEach:486' else 'var'
    if f0.startswith('bridges') or f0.startswith('extBridges'): return 'G bridge method: this as the interface',''
    if f0.startswith('instanceOf') or f0.startswith('tryStmt') or f0.startswith('switchBody'): return 'X instanceof pattern / catch / switch pattern binding (explicit type test in Java)',''
    if f0.startswith('lambda') or f0.startswith('memberRef') or f2.startswith('lambda'): return 'L lambda / method reference adapter of an erased functional interface',f0
    if f0.startswith('params'): return 'P parameter of an overriding method declared wider in the root',''
    if f0.startswith('finit') or f0.startswith('statics') or f0.startswith('structDecl') or f0.startswith('special') : pass
    explicit = f2.startswith('expr:341')
    a=ptype(arg) if arg else None; r=ptype(res) if res else None
    if kf=='CLS' and kt=='IFACE':
        leaf = 'obj_(' in head or 'any(this)' in head or 'func()' in head
        return 'U class -> interface it implements (static upcast in Java)', ('leaf' if leaf else 'non-leaf')+(' new' if 'obj_(new_' in head else ' this' if '(this)' in head else '')
    if kf=='TVAR' or kt=='TVAR': return 'V value of a kept type parameter meets an erased slot (boxT_/unboxT_)',''
    if b>0 and c==0 and u==0:
        if a:
            callee,pt,where=a
            base=pt.replace('...','[]')
            if re.fullmatch(r'java\.lang\.Object(\[\])?',base):
                if re.search(r'Preconditions\.|String\.format|lenientFormat|ToStringHelper|AssertionError|Exception\.<init>|LogFile|Verify\.|String\.valueOf|toStringHelper|SimpleFormat|MoreObjects|StringBuilder\.append|PrintStream|Joiner',callee): return 'O1 Object / Object... parameter: message and format arguments',callee+' '+kf
                if re.search(r'\.equals$|Objects\.equals|identical|Objects\.hash|Objects\.equal$|Objects\.hashCode|deepEquals',callee): return 'O2 Object parameter: equals / hash / identity',callee+' '+kf
                if where=='lib': return 'O3 Object parameter of a library collection lookup (get/contains/remove/indexOf)',callee+' '+kf
                return 'O4 Object parameter of a program method',callee+' '+kf
            if 'CharSequence' in base or 'Comparable' in base or 'Appendable' in base or 'Iterable' in base and kf!='IFACE': 
                if kf=='STR': return 'S String passed where CharSequence / Comparable is declared',callee
            if re.search(r'(^|[<\s,])[A-Z]\d?($|[>\[\],\s])',base) or re.fullmatch(r'[A-Z]\d?(\[\])?',base):
                return ('E1 argument of an erased library generic (type variable parameter)' if where=='lib' else 'E2 argument of an erased program generic (type variable parameter)'),callee
            return 'A argument: other ('+kf+'->'+kt+')',callee+' : '+pt
        if f0.startswith('call:'): return 'C receiver boxed for an Object/CharSequence/Comparable/default-method helper',kf
        return 'D assigned / returned / initialised into an Object- or interface-typed declaration ('+kf+'->'+kt+')',f2
    # casts / unboxes
    if explicit: return 'J explicit cast written in the Java source',kf+'->'+kt
    if r:
        callee,rt,where=r
        if re.search(r'(^|[<\s,])[A-Z]\d?($|[>\[\],\s])',rt) or re.fullmatch(r'[A-Z]\d?(\[\])?',rt):
            if kf=='CLS' and kt=='CLS': return 'K result of a library method declared on a supertype (covariant result lost)',callee
            return ('E1r result of an erased library generic (type variable result)' if where=='lib' else 'E2r result of an erased program generic (type variable result)'),callee
        if re.fullmatch(r'java\.lang\.Object',rt): return 'O5 result declared Object',callee
        if kf=='CLS' and kt=='CLS' or kf=='IFACE' and kt=='CLS': return 'K result of a method declared with a wider result than its use (covariant / group result)',callee+' : '+rt
        return 'Z result: other',callee+' : '+rt+' '+kf+'->'+kt
    return 'Y other ('+kf+'->'+kt+')',d['frame']
rows=[l.rstrip('\n').split('\t') for l in open('sites_after.tsv')]
tot=collections.defaultdict(lambda:[0,0,0,0]); sub=collections.defaultdict(collections.Counter); ex=collections.defaultdict(list)
for r in rows:
    r+=['']*(14-len(r))
    d=dict(n=tuple(int(x) for x in r[:4]),kf=r[4],kt=r[5],frm=r[6],to=r[7],pos=r[8],frame=r[9],arg=r[10],res=r[11],note=r[12],head=r[13])
    k,s=classify(d)
    for i in range(4): tot[k][i]+=d['n'][i]
    sub[k][s]+=sum(d['n']); ex[k].append(d['pos']+'  '+d['head'][:70])
want=sys.argv[1] if len(sys.argv)>1 else None
nsub=int(sys.argv[2]) if len(sys.argv)>2 else 6
print('%6s %6s %6s %6s  cause'%('box','cast','icast','unbox'))
for k,v in sorted(tot.items(),key=lambda kv:-sum(kv[1])):
    if want and not k.startswith(want): continue
    print('%6d %6d %6d %6d  %s'%(*v,k))
    if want:
        for s,n in sub[k].most_common(nsub): print('           %5d %s'%(n,s))
        import random; random.seed(1)
        for e in random.sample(ex[k],min(6,len(ex[k]))): print('        e.g.',e)
print('%6d %6d %6d %6d  total'%tuple(sum(v[i] for v in tot.values()) for i in range(4)))
