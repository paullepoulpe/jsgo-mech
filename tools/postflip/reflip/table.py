#!/usr/bin/env python3
"""Prints the replay table of the trial re-flip: per original commit of steps.txt, how it was
replayed on the new flip (as-is / adapted / no-op / dropped / not reached) and why.

usage: table.py [-counts] [last replayed commit]     (run from the repository root)
Reads tools/postflip/steps.txt (the original list) and tools/postflip/reflip/steps.txt (the
edited copy: `#= <commit> <status>: <reason>` lines, dropped lines commented `#- `)."""
import sys, re, collections
args = [a for a in sys.argv[1:] if not a.startswith('-')]
last = args[0] if args else None
orig = collections.OrderedDict(); title = {}
cur = None
for l in open('tools/postflip/steps.txt'):
    m = re.match(r'# (\d+)\. (.*)', l)
    if m: cur = m.group(2).strip(); continue
    if l.startswith('#') or not l.strip(): continue
    c, act, arg = l.split(None, 2)
    orig.setdefault(c, []).append((act, arg.strip())); title.setdefault(c, cur)
notes = collections.defaultdict(list); live = collections.defaultdict(int)
for l in open('tools/postflip/reflip/steps.txt'):
    m = re.match(r'#= (\w+) (\w+): (.*)', l)
    if m: notes[m.group(1)].append((m.group(2), m.group(3).strip())); continue
    if l.startswith('#') or not l.strip(): continue
    live[l.split()[0]] += 1
rows = []; reached = True; counts = collections.Counter()
for c, lines in orig.items():
    kind = 'tool' if any(a == 'run' for a, _ in lines) else 'manual' if any(x.endswith('-manual.patch') for _, x in lines) else 'runtime'
    sts = [s for s, _ in notes[c]]
    if not reached: st = 'not reached'
    elif live[c] == 0: st = 'dropped' if 'dropped' in sts else 'no-op'
    elif 'dropped' in sts: st = 'partly dropped'
    elif 'adapted' in sts: st = 'adapted'
    elif 'noop' in sts: st = 'no-op'
    else: st = 'as-is'
    counts[(kind, st)] += 1
    rows.append((c, kind, st, title[c], ' | '.join(w for _, w in notes[c])))
    if c == last: reached = False
if '-counts' not in sys.argv:
    print('| commit | kind | replay | step | why |\n|---|---|---|---|---|')
    for r in rows: print('| %s | %s | %s | %s | %s |' % (r[0], r[1], r[2], r[3][:90], r[4]))
    print()
kinds = ['tool', 'runtime', 'manual']; sts = ['as-is', 'adapted', 'no-op', 'partly dropped', 'dropped', 'not reached']
print('| kind | ' + ' | '.join(sts) + ' | total |\n|---|' + '---|' * (len(sts) + 1))
for k in kinds:
    print('| %s | ' % k + ' | '.join(str(counts[(k, s)]) for s in sts) + ' | %d |' % sum(counts[(k, s)] for s in sts))
print('| all | ' + ' | '.join(str(sum(counts[(k, s)] for k in kinds)) for s in sts) + ' | %d |' % len(rows))
