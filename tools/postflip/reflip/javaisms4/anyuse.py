# For every `name any` parameter the census counts (sites.tsv row 100, categories line/param), reads the
# enclosing function's text and says how the parameter is used: only under one unboxing to one type
# (statically single-typed: an erasure bridge), only under comma-ok tests, passed on as Object, etc.
import re,collections,sys
R="/mnt/data/ubuntu/jm/wt/agQr/go/closure/"
src={}
def text(f):
    if f not in src: src[f]=open(R+f).read().split("\n")
    return src[f]
unb=r"(cast_\[[^()]*\]|to_\w+|icast_\w+|gcast_\w+(\[[^()]*\])?|str_|Node_of_|unboxT_\[[^()]*\]|unbox_\w*)"
res=collections.Counter(); ex=collections.defaultdict(list); bykind=collections.defaultdict(collections.Counter)
for l in open("sites.tsv"):
    f=l.rstrip("\n").split("\t")
    if f[0]!="100": continue
    cat,decl=f[7].split(":",1)
    if cat not in("param",): continue
    kind=f[4]
    if kind.startswith("NOT CODE"): continue
    name=decl.split()[0]
    L=text(f[1]); i=int(f[2])-1
    # function body: from the declaration line to the line where braces balance
    depth=0; body=[]; started=False
    line0=L[i]; 
    # start counting braces after the parameter's position in the line
    pos=line0.find(decl.split(",")[0].rstrip(")"))
    seg=line0[pos:]
    j=i
    while True:
        s=seg if j==i else L[j]
        for ch in s:
            if ch=="{": depth+=1; started=True
            elif ch=="}": depth-=1
        body.append(s)
        if started and depth<=0: break
        j+=1
        if j>=len(L) or j-i>400: break
    b="\n".join(body)
    b=b[b.find("{")+1:] if "{" in b else b
    uses=[m.start() for m in re.finditer(r"(?<![\w.])"+re.escape(name)+r"(?![\w])",b)]
    n=len(uses)
    under=collections.Counter(m.group(1) for m in re.finditer(unb+r"\("+re.escape(name)+r"\)",b))
    commaok=len(re.findall(r"(?<![\w.])"+re.escape(name)+r"\.\(",b))
    nilcmp=len(re.findall(r"(?<![\w.])"+re.escape(name)+r"\s*[!=]=\s*nil",b))
    isx=len(re.findall(r"is_(\[[^()]*\]|\w+)\("+re.escape(name)+r"\)",b))
    nu=sum(under.values())
    if n==0: k="unused in the body"
    elif nu+nilcmp==n and len(under)==1: k="only unboxed to ONE type (+nil tests): single-typed by construction"
    elif nu+nilcmp==n: k="unboxed to more than one type"
    elif nu+commaok+isx+nilcmp==n and (commaok or isx): k="only tested/asserted (instanceof, comma-ok) and unboxed: heterogeneous by design"
    elif nu>0: k="unboxed and also passed on as an Object"
    elif commaok+isx>0: k="tested (instanceof) and also passed on as an Object"
    else: k="only passed on / used as an Object (equals, hashCode, toString, stored, forwarded)"
    res[(kind,k)]+=1; bykind[(kind,k)][f[5]]+=1
tot=collections.Counter()
for (kind,k),v in res.items(): tot[kind]+=v
for kind in tot:
    print("==",kind,tot[kind])
    for (kk,k),v in sorted(res.items(),key=lambda x:-x[1]):
        if kk!=kind: continue
        print("   %4d %s"%(v,k)); print("          "+"; ".join("%s %d"%(a.replace("method listed by an interface: ",""),b) for a,b in bykind[(kk,k)].most_common(9)))
