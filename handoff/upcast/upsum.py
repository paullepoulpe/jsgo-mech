import sys,re,collections
d=collections.defaultdict(lambda: collections.defaultdict(list)); loads=collections.defaultdict(list)
for l in open(sys.argv[1]):
    m=re.match(r'\s+(\S+) (\S+) wall=([\d.]+) cpu=([\d.]+) round=(\d+) load=([\d.]+)',l)
    if m:
        d[m.group(1)][m.group(2)].append((float(m.group(4)),float(m.group(3)))); loads[m.group(1)].append(float(m.group(6)))
    elif 'objects' in l: print(l.strip())
for task,bl in d.items():
    print(task, 'load %.1f-%.1f'%(min(loads[task]),max(loads[task])))
    for lab,v in bl.items():
        print('  %-6s cpu %s mean %.2f | wall %s mean %.1f'%(lab,'/'.join('%.2f'%c for c,_ in v),sum(c for c,_ in v)/len(v),'/'.join('%.1f'%w for _,w in v),sum(w for _,w in v)/len(v)))
