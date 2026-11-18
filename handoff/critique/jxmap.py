import re,collections,sys
from jmap import *
A=re.compile(r'\.\((\*?[A-Za-z_][\w.]*(?:\[[^\]\n]*\])?|interface ?\{[^}\n]*\})\)')
rows=[l.rstrip('\n').split('\t') for l in open('jx.tsv')]
seen=set(); out=open('jxmap.tsv','w'); tally=collections.Counter()
for r in rows:
    k,pos,kk,frm,to,head,jl=r
    if (k,pos,to) in seen: continue
    seen.add((k,pos,to))
    c,m,cs=lookup(pos)
    grp='other'
    f=pos.split('/')[-1]
    if f.startswith('ParseTree.java') or (f.startswith('Token.java') and 'this' in jl): grp='asX self-cast'
    elif re.search(r'\.error\(\)',jl): grp='Outcome.error()'
    elif m in('equals',): grp='equals'
    elif re.search(r'getProp\(|getFeatureSet\(',jl): grp='prop bag'
    elif k=='X': grp='instanceof'
    if not cs: res='no-jsgo-func'
    else:
        body='\n'.join(x[3] for x in cs)
        t=to.split('<')[0].split('.')[-1]
        na=len([1 for mm in A.finditer(body) if mm.group(1)!='type'])
        res=('typeswitch ' if '.(type)' in body else '')+('assert-target ' if re.search(r'\.\(\*?(\w+\.)?'+re.escape(t)+r'\b',body) or re.search(r'case \*?(\w+\.)?'+re.escape(t)+r'\b',body) else ('assert-other ' if na else ''))+('node_cast ' if 'node_cast' in body or 'jsdocGet' in body else '')
        res=res.strip() or 'NONE'
    tally[(k,grp,res)]+=1
    out.write('\t'.join([k,grp,res,pos,c+'.'+str(m),to.split('<')[0].split('.')[-1],';'.join('%s:%d'%(x[0],x[1]) for x in cs[:2]),jl[:110]])+'\n')
for (k,g,r),n in sorted(tally.items()): print(f'{n:4d} {k} {g:18s} {r}')
