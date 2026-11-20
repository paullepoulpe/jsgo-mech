import csv,sys,collections as C
S=sys.argv[1]
def rd(n): return [l.rstrip('\n').split('\t') for l in open(f'{S}/{n}.tsv')]
D=rd('decls'); F=rd('flows')
short=lambda q:q.replace('java.util.function.','juf.').replace('java.util.','').replace('java.lang.','').replace('com.google.common.collect.','g.').replace('com.google.common.base.','gb.').replace('com.google.common.','gc.')
# decl table per interface x kind and cat
print("== DECLS per type: total | field param return local foreachvar lambdaparam | typedNow | cats")
t=C.defaultdict(lambda:C.Counter())
for r in D:
    k=(r[4],short(r[5])); t[k]['n']+=1; t[k]['k:'+r[3]]+=1; t[k]['c:'+r[6]]+=1; t[k]['typed']+=int(r[9])
for k,c in sorted(t.items(),key=lambda x:(x[0][0],-x[1]['n'])):
    print(f"{k[0]:11s} {k[1]:34s} {c['n']:5d} | "+" ".join(f"{c['k:'+x]:4d}" for x in ['field','param','return','local','foreachvar','lambdaparam'])+f" | {c['typed']:4d} | "+" ".join(f"{x[2:]}={n}" for x,n in sorted(c.items()) if x.startswith('c:')))
print()
print("== FLOWS (init/assign/return/arg/lambdaret) per target type x cat")
t=C.defaultdict(C.Counter)
for r in F:
    if r[3] in('init','assign','return','arg','lambdaret') and not r[4].startswith('to-'):
        t[(r[5],short(r[6]))][r[4]]+=1
cats=['exact','null','ext-same','sup-same','wild-same','ext-sub','sup-wider','wild-any','src-wild','raw-source','raw-target','no-view']
print(f"{'':46s}"+" ".join(f"{c:>9s}" for c in cats))
tot=C.Counter()
for k,c in sorted(t.items(),key=lambda x:(x[0][0],-sum(x[1].values()))):
    print(f"{k[0]:11s} {k[1]:34s}"+" ".join(f"{c[x]:9d}" for x in cats)); tot.update(c)
print(f"{'TOTAL':46s}"+" ".join(f"{tot[x]:9d}" for x in cats), sum(tot.values()))
fam=C.defaultdict(C.Counter)
for k,c in t.items(): fam[k[0]].update(c)
for k,c in fam.items(): print(f"{k:46s}"+" ".join(f"{c[x]:9d}" for x in cats), sum(c.values()))
