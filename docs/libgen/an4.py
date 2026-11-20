# an4.py: the category table of the study (flows and declarations by family and category)
import sys,collections as C,re
S=sys.argv[1]
def rd(n): return [l.rstrip('\n').split('\t') for l in open(f'{S}/{n}.tsv')]
D=rd('decls'); F=rd('flows')
fams=['functional','iteration','collection','comparable','optional','class','other']
FL=[r for r in F if r[3] in('init','assign','return','arg','lambdaret') and not r[4].startswith('to-')]
def vcat(r):
    c=r[4]
    return {'exact':'a exact','null':'a exact','ext-same':'b1 wildcard, same bound','sup-same':'b1 wildcard, same bound','wild-same':'b1 wildcard, same bound','src-wild':'b1 wildcard, same bound',
            'ext-sub':'b2 covariant (A<:B)','sup-wider':'b2 contravariant','wild-any':'b3 unbounded ?'}.get(c,c)
print("FLOWS by variance category")
t=C.defaultdict(C.Counter)
for r in FL: t[vcat(r)][r[5]]+=1
print(f"{'':28s}"+"".join(f"{f:>12s}" for f in fams)+"   total")
for k in sorted(t): print(f"{k:28s}"+"".join(f"{t[k][f]:12d}" for f in fams)+f"  {sum(t[k].values()):6d}")
print("\nFLOWS by kind of type argument of the target (worst argument)")
t=C.defaultdict(C.Counter)
for r in FL: t[r[9]][r[5]]+=1
for k in sorted(t): print(f"{k:28s}"+"".join(f"{t[k][f]:12d}" for f in fams)+f"  {sum(t[k].values()):6d}")
print("\nARG FLOWS by owner of the type variables in the callee's declared parameter")
t=C.defaultdict(C.Counter)
for r in FL:
    if r[3]=='arg': t[('prog ' if r[11].startswith('prog') else 'lib  ')+r[12]][r[5]]+=1
for k in sorted(t): print(f"{k:28s}"+"".join(f"{t[k][f]:12d}" for f in fams)+f"  {sum(t[k].values()):6d}")
print("\nDECLS by category")
t=C.defaultdict(C.Counter)
for r in D: t[r[6]][r[4]]+=1
for k in sorted(t): print(f"{k:28s}"+"".join(f"{t[k][f]:12d}" for f in fams)+f"  {sum(t[k].values()):6d}")
t=C.Counter()
for r in D: t[(r[4],r[3])]+=1
print("\nDECLS by family and kind:",sorted(t.items()))
print("\nBOX arguments in declarations: which box, which position")
c=C.Counter()
for r in D:
    if r[6]=='box':
        pos=[i for i,x in enumerate(r[7].split(',')) if x=='box']
        boxes=re.findall(r'java\.lang\.(Integer|Boolean|Long|Double|Character|Float|Short|Byte)',r[8])
        c[(r[4],re.sub(r'^.*\.','',r[5]) if r[4]!='collection' else ('map' if 'Map' in r[5] and 'Entry' not in r[5] else 'entry' if 'Entry' in r[5] else 'list/set/coll'),'pos'+''.join(map(str,pos)) if pos else 'nested',boxes[0] if boxes else '?')]+=1
for k,n in c.most_common(30): print(f"  {n:4d} {k}")
print("\nOBJECT arguments in declarations")
for r in D:
    if r[6]=='object': print("   ",r[0]+':'+r[1],r[2],r[3],r[8].replace('java.util.','').replace('java.lang.','')[:110])
print("\nERASED type variables in declarations")
for r in D:
    if r[6]=='tvar-erased': print("   ",r[0]+':'+r[1],r[2],r[3],r[8].replace('java.util.','').replace('java.lang.','')[:110])
