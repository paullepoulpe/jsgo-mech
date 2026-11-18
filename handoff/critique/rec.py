import re,os,collections
J='/home/user/jm-merge/java/src/'
recs=[]
for d,_,fs in os.walk(J):
    for f in fs:
        if not f.endswith('.java'): continue
        s=open(os.path.join(d,f)).read()
        for m in re.finditer(r'\brecord (\w+)(?:<[^>]*>)?\s*\(',s):
            if re.search(r'[\w.>]\s*$',s[max(0,m.start()-1):m.start()]) and not re.search(r'(public|private|protected|static|final|\n)\s*$',s[:m.start()]): continue
            i=m.end(); depth=1
            while depth and i<len(s):
                if s[i]=='(':depth+=1
                elif s[i]==')':depth-=1
                i+=1
            comps=s[m.end():i-1]
            comps=re.sub(r'@\w+(\([^)]*\))?\s*','',comps)
            # split top-level commas
            parts=[];cur='';dd=0
            for ch in comps:
                if ch=='<':dd+=1
                if ch=='>':dd-=1
                if ch==',' and dd==0: parts.append(cur.strip());cur=''
                else: cur+=ch
            if cur.strip(): parts.append(cur.strip())
            types=[' '.join(p.split()[:-1]) for p in parts]
            body_eq = False
            recs.append((f,m.group(1),types))
prim={'int','long','boolean','char','short','byte','String'}
cnt=collections.Counter(); tc=collections.Counter()
for f,n,ts in recs:
    kinds=set()
    for t in ts:
        b=t.split('<')[0]
        if b in prim: k='prim/str'
        elif b in('double','float','Double','Float'): k='float'
        elif re.match(r'(Immutable|List|Set|Map|Collection|Iterable|Optional|Linked|Multimap|Supplier|Function|Predicate)',b) or '[]' in t: k='coll/other-lib'
        elif b in('Integer','Boolean','Long'): k='boxed'
        else: k='ref'
        kinds.add(k); tc[b]+=1
    key='+'.join(sorted(kinds)) or 'empty'
    cnt[key]+=1
print(len(recs),'records'); 
for k,v in cnt.most_common(): print(v,k)
print(tc.most_common(40))
print('---- value-use of records (crude)')
alls={}
for d,_,fs in os.walk(J):
    for f in fs:
        if f.endswith('.java'): alls[f]=open(os.path.join(d,f)).read()
big='\n'.join(alls.values())
used=[];unused=[]
for f,n,ts in recs:
    if n in('which',): continue
    pat=r'(Set|Multiset|HashSet|LinkedHashSet|ImmutableSet|TreeSet|SortedSet)<(\w+\.)?%s>|(Map|HashMap|LinkedHashMap|ImmutableMap|Multimap|SetMultimap|ListMultimap|Table|BiMap|TreeMap)<(\w+\.)?%s\s*,|Multimap<[^<>]*,\s*(\w+\.)?%s>|Equivalence<%s>'%(n,n,n,n)
    m=re.search(pat,big)
    (used if m else unused).append((n,f,m.group(0) if m else ''))
print('records used as set element / map key / setmultimap value:',len(used)); print([u[0]+':'+u[2] for u in used])
print('records with no such use:',len(unused)); print([u[0] for u in unused])
