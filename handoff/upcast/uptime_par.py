# uptime_par.py <log> <prev> <this>: the timing paragraph of a commit message
import sys,re,collections
log,prev,this=sys.argv[1:4]
d=collections.defaultdict(lambda: collections.defaultdict(list)); loads=collections.defaultdict(list); mem={}
for l in open(log):
    m=re.match(r'\s+(\S+) (\S+) wall=([\d.]+) cpu=([\d.]+) round=(\d+) load=([\d.]+)',l)
    if m: d[m.group(1)][m.group(2)].append((float(m.group(4)),float(m.group(3)))); loads[m.group(1)].append(float(m.group(6)))
    m=re.match(r'(\S+) (\S+) objects=(\S+) bytes=(\S+)',l)
    if m: mem[(m.group(1),m.group(2))]=(m.group(3),m.group(4))
def mean(v): return sum(v)/len(v)
out=[]
al=[]
for task in ['bundle','typed-full']:
    if (task,prev) in mem and (task,this) in mem:
        a,b=mem[(task,prev)],mem[(task,this)]
        al.append('%s %s -> %s objects, %s -> %s bytes'%(task,a[0],b[0],a[1],b[1]))
out.append('Allocations (JSGO_MEMSTATS, one run each; previous commit -> this): '+'; '.join(al)+'.')
out.append('Time (fb.py, rounds alternating forward and backward under the lock, no PGO; erased = J2GO_ERASED=1 output, prev = previous commit, this; measured in one run together with the neighbouring commits; other agents were using the host: load average %s). cpu seconds:'%', '.join('%.1f to %.1f during %s'%(min(loads[t]),max(loads[t]),t) for t in d))
names={'upE':'erased',prev:'prev',this:'this'}
means={}
for task,bl in d.items():
    parts=[]
    for lab in ['upE',prev,this]:
        v=bl[lab]; parts.append('%s %s'%(names[lab],'/'.join('%.2f'%c for c,_ in v))); means[(task,lab)]=mean([c for c,_ in v])
    out.append('  %s, %d rounds: %s'%(task,len(bl[this]),', '.join(parts)))
w=[]
for task,bl in d.items():
    w.append('%s %s'%(task,', '.join('%s %s'%(names[lab],'/'.join('%.1f'%x for _,x in bl[lab])) for lab in ['upE',prev,this])))
out.append('wall seconds: '+'; '.join(w)+'.')
ms=[]
for task in d:
    e,p,t=means[(task,'upE')],means[(task,prev)],means[(task,this)]
    ms.append('%s cpu erased %.2f, prev %.2f, this %.2f (%+.1f%% against prev, %+.1f%% against erased)'%(task,e,p,t,(t/p-1)*100,(t/e-1)*100))
out.append('Means: '+'; '.join(ms)+'.')
import textwrap
for o in out:
    if o.startswith('  ') or '\n' in o: print(o)
    else: print(textwrap.fill(o,95))
