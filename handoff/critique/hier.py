import re,os,collections
J='/home/user/jm-merge/java/src/'
decl=re.compile(r'\b(?:(abstract|final|sealed|static|public|private|protected|non-sealed)\s+)*(class|interface|enum|record)\s+(\w+)(?:<[^{]*?>)?\s*(?:\([^{]*?\))?\s*((?:extends|implements)[^{]*)?\{',re.S)
types={}; supers=collections.defaultdict(set); big=''
for d,_,fs in os.walk(J):
    for f in fs:
        if not f.endswith('.java'): continue
        s=open(os.path.join(d,f)).read(); big+=s
        s2=re.sub(r'/\*.*?\*/','',s,flags=re.S); s2=re.sub(r'//[^\n]*','',s2)
        for m in decl.finditer(s2):
            mods=s2[max(0,m.start()-60):m.start()+len(m.group(0))]
            kind,name,tail=m.group(2),m.group(3),m.group(4) or ''
            abstract=bool(re.search(r'\babstract\s+(\w+\s+)*class\s+'+name+r'\b',mods))
            types.setdefault(name,(kind,abstract))
            tail=re.sub(r'<[^<>]*(<[^<>]*(<[^<>]*>[^<>]*)*>[^<>]*)*>','',tail)
            for sup in re.findall(r'[\w.]+',tail):
                if sup in('extends','implements','permits'): continue
                supers[name].add(sup.split('.')[-1])
subs=collections.defaultdict(set)
for c,ss in supers.items():
    for s in ss: subs[s].add(c)
def concrete_desc(t,seen=None):
    seen=seen or set(); out=set()
    for c in subs.get(t,()):
        if c in seen: continue
        seen.add(c)
        k=types.get(c,('class',False))
        if k[0] in('class','record','enum') and not k[1]: out.add(c)
        out|=concrete_desc(c,seen)
    return out
single=[]
for t,(kind,abstract) in types.items():
    if kind=='interface' or abstract:
        cd=concrete_desc(t)
        anon=len(re.findall(r'new\s+(?:\w+\.)*'+t+r'(?:<[^>]*>)?\s*\([^;{]*\)\s*\{',big))
        lam=bool(re.search(r'@FunctionalInterface\s+(public\s+)?interface\s+'+t+r'\b',big))
        if len(cd)==1 and anon==0 and not lam: single.append((t,kind,list(cd)[0]))
print('interfaces+abstract classes:',sum(1 for t,(k,a) in types.items() if k=='interface' or a))
print('with exactly one concrete named implementer, no anonymous impl, not @FunctionalInterface:',len(single))
for t,k,c in sorted(single):
    uses=len(re.findall(r'\b'+t+r'\b',big))
    casts=len(re.findall(r'\(\s*'+c+r'(?:<[^>]*>)?\s*\)\s*[\w(]',big))
    print(f'  {t} ({k}) -> {c}; mentions {uses}; explicit casts to {c}: {casts}')
