# dumps every census match: row-index TAB file TAB byte-offset TAB line TAB matched text TAB source line
# usage: cd go/closure && python3 matches.py /path/census.py > matches.tsv
import re,glob,sys
src=open(sys.argv[1]).read()
pre=src[:src.index("files=sorted")]
ns={}
exec(pre,ns)
pats=ns['pats']
files=sorted(glob.glob("g_*.go"))
for f in files:
    t=open(f).read()
    for i,(name,p) in enumerate(pats):
        for m in re.finditer(p,t,re.M):
            ls=t.rfind("\n",0,m.start())+1; le=t.find("\n",m.start())
            g=m.group(0).replace("\n","\\n").replace("\t","\\t")
            print(f"{i}\t{f}\t{m.start()}\t{t.count(chr(10),0,m.start())+1}\t{g}\t{t[ls:le].strip()}")
