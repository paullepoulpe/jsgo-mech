# link.py: joins the accounting's R1 census sites (javaisms/sites.tsv, Go tree agQr) to the Java-side categories of
# the method they are in (scan/decls.tsv, scan/flows.tsv). The join is by file and method name: an estimate.
import sys,re,collections as C,os,bisect
J='/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/javaisms'
S=sys.argv[1]; Q='/mnt/data/ubuntu/jm/wt/agQr/go/closure'
names=["row","file","line","hot","sub","detail","extra","matched"]
sites=[dict(zip(names,l.rstrip("\n").split("\t"))) for l in open(J+"/sites.tsv")]
def ex(d,i):
    p=d["extra"].split(" | "); return p[i] if len(p)>i else ""
def castkey(d):
    s=d["sub"]
    if s.startswith(("NOT CODE","helper declaration","cast function used as a value","inside a cast helper")): return None
    g="GUARDED" in d["extra"]
    s=re.sub(r"^(local (Object \(any\)|a Java interface) initialised from: )+","",s)
    if "catch_" in d["detail"] or g: return None
    if s.startswith("self()"): return None
    if "method listed by an interface" in s: return "bridge-param"
    if "func literal" in s: return "funclit-param"
    if s.startswith("element of a for-each") or s.startswith("range variable"): return "foreach-elem"
    if "method called through a Java interface" in s: return "iface-result"
    if "runtime function/method" in s: return None if "getProp" in d["detail"] or "Annotatable" in d["detail"] else "runtime-result"
    if "func value" in s: return "funcvalue-result"
    return None
sel=[]
for d in sites:
    r=d["row"]; k=None
    if r in("10","11"): k=castkey(d)
    elif r=="16": k=None if d["matched"].startswith(".(") else castkey(d)
    elif r=="13":
        if d["sub"].startswith("Java cast folded") and "self()" not in d["sub"]: k="folded-cast"
    elif r=="18": k="adapter:"+d["sub"][:40]
    elif r=="15":
        s=d["sub"]
        if s in("argument of a generated interface method","returned from a method listed by an interface (erased signature)","returned from a func literal","argument of a func value"): k="box:"+s[:34]
    elif r=="100":
        if d["sub"].startswith("parameter of a method listed by an interface"): k="anyparam-bridge"
        elif d["sub"].startswith("parameter of a func literal"): k="anyparam-funclit"
    if k: sel.append((d,k))
print("selected sites:",len(sel), C.Counter(d["row"] for d,k in sel))
# Go: enclosing func by line
funcs={}
def gofunc(f,line):
    if f not in funcs:
        L=[]; 
        try:
            for i,l in enumerate(open(os.path.join(Q,f)),1):
                m=re.match(r'func (?:\(\w+ \*?([\w]+)(?:\[[^\]]*\])?\) )?(\w+)',l)
                if m: L.append((i,m.group(1),m.group(2)))
        except FileNotFoundError: pass
        funcs[f]=L
    L=funcs[f]; i=bisect.bisect_right([x[0] for x in L],line)-1
    return L[i] if i>=0 else None
# Java side: per (file, method) categories
def rd(n): return [l.rstrip('\n').split('\t') for l in open(f'{S}/{n}.tsv')]
D=rd('decls'); F=rd('flows')
ORDER=['variance','wildcard','erased-tvar','object','box','exact']
def dcat(c):
    return {'extends':'wildcard','super':'wildcard','wild':'wildcard','tvar-erased':'erased-tvar','object':'object','box':'box'}.get(c,'exact')
jm=C.defaultdict(set); jmeth=C.defaultdict(set)
def add(file,where,cat):
    meth=where.split('.',1)[1] if '.' in where else '<clinit>'
    jm[(file,meth)].add(cat); jmeth[file].add(meth)
for r in D:
    if r[4] in('optional','class','other'): continue
    add(r[0],r[2] if r[3]!='return' else r[2],dcat(r[6]))
for r in F:
    if r[5] in('optional','class','other','prog-functional'): continue
    c=r[4]
    if c in('ext-sub','sup-wider','wild-any'): cat='variance'
    elif c in('ext-same','sup-same','wild-same','src-wild'): cat='wildcard'
    else:
        cat=dcat(r[9]) if r[3]!='foreach' else dcat(r[4])
        if 'erased' in r[12]: cat='erased-tvar'
    add(r[0],r[2],cat)
I=rd('impls'); ji=C.defaultdict(set)
for r in I:
    if r[4] in('functional','iteration','comparable','collection'): ji[r[0]].add(dcat(r[6]))
how=C.Counter()
res=C.Counter(); bykind=C.defaultdict(C.Counter); unl=C.Counter()
for d,k in sel:
    f=d["file"]; jf=f[2:-3]+'.java'
    if jf not in jmeth:
        jf2=re.sub(r'_[a-z]+$','',f[2:-3])+'.java'
        jf=jf2
    g=gofunc(f,int(d["line"]))
    cat='unlinked'
    if f=='g_zstubs.go': cat='adapter-infra'
    elif g and jf in jmeth:
        name=g[2]; cands=jmeth[jf]
        best=None
        base=name.split('_')[0]
        if name.startswith('finit_') or name.endswith('_clinit'): best='<clinit>' if '<clinit>' in cands else None
        elif g[1] and base in cands: best=base
        elif name.startswith(('new_','init_','finit_')): best='<init>' if '<init>' in cands else None
        elif name.endswith('_clinit'): best='<clinit>' if '<clinit>' in cands else None
        else:
            for m in sorted(cands,key=len,reverse=True):
                if name==m or name.endswith('_'+m) or ('_'+m+'_') in name or name.startswith(m+'_'): best=m; break
        if best:
            cs=jm[(jf,best)]
            cat=min(cs,key=ORDER.index); how['method']+=1
    if cat=='unlinked':
        bridge=k in('bridge-param','anyparam-bridge') or k.startswith('box:returned from a method')
        if bridge and jf in ji: cat=min(ji[jf],key=ORDER.index); how['implements (file)']+=1
        elif jf in jmeth and (jf,'<clinit>') in jm and (g is None or g[2].endswith('clinit') or not g[1] and g[2].startswith(('init','finit'))): cat=min(jm[(jf,'<clinit>')],key=ORDER.index); how['initializer']+=1
        elif jf in jmeth:
            allc=set().union(*[jm[(jf,m)] for m in jmeth[jf]])
            if allc<= {'exact'}: cat='exact'; how['file has only exact']+=1
            else: unl[f]+=1
        else: unl[f]+=1
    res[cat]+=1; bykind[k.split(':')[0] if d["row"]!="18" else 'adapter'][cat]+=1
print("category of the Java method a site is in (worst category present in that method):")
tot=sum(res.values())
for c in ORDER+['adapter-infra','unlinked']: print(f"  {c:12s} {res[c]:5d} {100*res[c]/tot:5.1f}%")
print("by site kind:")
for k,c in sorted(bykind.items(),key=lambda x:-sum(x[1].values())):
    print(f"  {k:18s} {sum(c.values()):4d} | "+" ".join(f"{x}={c[x]}" for x in ORDER+['adapter-infra','unlinked']))
print("linked by:",how.most_common())
print("unlinked top files:",unl.most_common(8))
