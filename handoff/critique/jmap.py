import re,os,collections,functools
J='/home/user/jm-merge/java/src/'
G='/home/user/jsgo/internal'
# ---- jsgo index: "Class.method" -> [(file, line, sig, body)]
idx=collections.defaultdict(list)
def build():
    for d,_,fs in os.walk(G):
        if 'testjunit' in d or 'testing' in d: continue
        for f in fs:
            if not f.endswith('.go') or f.endswith('_test.go'): continue
            p=os.path.join(d,f); L=open(p).read().split('\n')
            i=0
            while i<len(L):
                m=re.match(r'// Java: ([\w.$<>]+?)\(',L[i])
                if m:
                    key=m.group(1)
                    j=i
                    while j<len(L) and L[j].startswith('//'): j+=1
                    if j<len(L) and L[j].startswith('func'):
                        k=j
                        while k<len(L) and L[k]!='}' : 
                            if L[k].endswith('}') and k==j and L[k].count('{')==L[k].count('}'): break
                            k+=1
                        idx[key].append((p[len(G)+1:],j+1,L[j],'\n'.join(L[j:k+1])))
                        # also index further "// Java:" lines in same comment block
                    i=j
                i+=1
build()
@functools.lru_cache(None)
def jparse(f):
    """returns list per line (1-based) of 'Outer.Inner.method'"""
    try: src=open(J+f).read()
    except Exception: return []
    # strip comments/strings crudely, keep newlines
    def blank(m): return re.sub(r'[^\n]',' ',m.group(0))
    s=re.sub(r'/\*.*?\*/',blank,src,flags=re.S)
    s=re.sub(r'"""(?:.|\n)*?"""',blank,s)
    s=re.sub(r'"(?:\\.|[^"\\\n])*"',lambda m:'"'+' '*(len(m.group(0))-2)+'"',s)
    s=re.sub(r"'(?:\\.|[^'\\\n])+'",lambda m:"'"+' '*(len(m.group(0))-2)+"'",s)
    s=re.sub(r'//[^\n]*',blank,s)
    out=[]; stack=[]  # (kind,name,depth)
    depth=0; line=1; res={}
    pend=''  # text since last ; { }
    cur=lambda: ('.'.join(n for k,n,_ in stack if k=='c'), next((n for k,n,_ in reversed(stack) if k=='m'),None))
    res[1]=cur()
    for ch in s:
        if ch=='\n':
            line+=1; res[line]=cur(); pend+=' '; continue
        if ch=='{':
            h=pend.strip()
            mc=re.search(r'\b(class|interface|enum|record)\s+(\w+)',h)
            mm=re.search(r'(\w+)\s*\([^()]*(?:\([^()]*\)[^()]*)*\)\s*(?:throws [\w., ]+)?$',h)
            if mc and not re.search(r'\bnew\b',h.split(mc.group(0))[0][-10:]): stack.append(('c',mc.group(2),depth))
            elif mm and mm.group(1) not in ('if','for','while','switch','catch','synchronized','try','new') and not any(k=='m' for k,_,_ in stack) and not re.search(r'\bnew\s+[\w.<>]+\s*\([^{}]*$',h) and '->' not in h and '=' not in h.split('(')[0]:
                stack.append(('m',mm.group(1),depth))
            depth+=1; pend=''
        elif ch=='}':
            depth-=1
            while stack and stack[-1][2]>=depth: stack.pop()
            pend=''
        elif ch==';': pend=''
        else: pend+=ch
        res[line]=cur() if res.get(line,(None,None))[1] is None else res[line]
    return res
def where(pos):
    f,l=pos.rsplit(':',1); r=jparse(f)
    return r.get(int(l),('',None)) if r else ('',None)
def lookup(pos):
    c,m=where(pos)
    if not c: return c,m,[]
    cands=[]
    parts=c.split('.')
    if m is None: return c,m,[]
    name = m
    for n in range(len(parts)):
        key='.'.join(parts[n:])+'.'+name
        if key in idx: cands=idx[key]; break
    if not cands and m==parts[-1]:  # constructor: "Class(" form
        for n in range(len(parts)):
            key='.'.join(parts[n:])
            if key in idx: cands=idx[key]; break
    return c,m,cands
if __name__=='__main__':
    print(len(idx))
    for p in ['com/google/javascript/jscomp/GlobalNamespace.java:1437','com/google/javascript/rhino/Node.java:1290','com/google/javascript/jscomp/TypeInference.java:2841']:
        c,m,cs=lookup(p); print(p,c,m,[(x[0],x[1],x[2][:80]) for x in cs])
