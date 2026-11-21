#!/usr/bin/env python3
"""collmap.py <dir>: the tables of COLLECTIONS-PLAN.md from the translator's report J2GO_COLLMAP=<dir>
(decls.tsv, slots.tsv, origins.tsv; tools/j2go/src/j2go/CollMap.java)."""
import sys, re, collections as C
D = sys.argv[1]
def rd(n): return [l.rstrip('\n').split('\t') for l in open(f'{D}/{n}.tsv')]
decls, slots, origins = rd('decls'), rd('slots'), rd('origins')
short = lambda q: re.sub(r'\b(?:[a-z_0-9]+\.)+', '', q)

# qualified owner of a library method, by "Simple.method" (from the origins)
libm = {}
for o in origins:
    if o[1].startswith('lib-'):
        q = o[2].rsplit('/', 1)[0]; cls, m = q.rsplit('.', 1)
        libm[cls.rsplit('.', 1)[-1] + '.' + m] = (o[1], cls)

VIEWM = {'keySet', 'values', 'entrySet', 'navigableKeySet', 'descendingMap', 'descendingSet', 'tailSet', 'headSet', 'subList', 'subMap', 'tailMap', 'headMap', 'asMap', 'inverse', 'elementSet', 'row', 'rowKeySet', 'columnKeySet', 'cellSet', 'asList', 'reverse'}
ELEMM = {'get', 'getOrDefault', 'computeIfAbsent', 'remove', 'put', 'getValue', 'getKey', 'next', 'peek', 'pop', 'poll', 'getFirst', 'getLast', 'removeFirst', 'removeLast', 'putIfAbsent', 'merge', 'compute', 'orElse', 'orElseGet', 'orElseThrow', 'getUnchecked', 'apply', 'call'}
def proto(cls): return (cls.startswith('com.google.javascript.') or cls.startswith('com.google.protobuf.') or cls.startswith('com.google.debugging.')) and not cls.endswith('.base.Io')
def libclass(kind, cls, m):
    """the piece a library method that gives a collection belongs to"""
    if proto(cls): return '4 protobuf list'
    if kind == 'lib-static': return '3 runtime function (static)'
    c = cls.rsplit('.', 1)[-1]
    if m in VIEWM and (cls.startswith('java.util') or cls.startswith('com.google.common.collect') or 'Simple' in c or 'Linked' in c): return '2 view of a collection (%s)' % ('map view' if m in ('keySet', 'values', 'entrySet') else 'other view')
    if m in ELEMM: return '5 element of a collection or result of a function value (nested collection)'
    return '3 runtime method (instance)'
def cls_why(w):
    m = re.match(r'assigned the result of (\S+)\.(\S+)$', w)
    if m:
        k = libm.get(m.group(1) + '.' + m.group(2))
        if k: return libclass(k[0], k[1], m.group(2))
        return '6 result of a method of the program (type-variable result, or not a declaration of the rule)'
    if w.startswith('assigned a new '): return '1 constructor (a class or constructor the typed family lacks: %s)' % w[15:]
    if w.startswith('assigned a variable that is not typed'): return '6 a variable that is no declaration of the rule (another type, lambda parameter)'
    if w.startswith('the variable of a for-each loop'): return '5 element of a collection or result of a function value (nested collection)'
    if w.startswith('method '): return '7 use: a method the view does not have (%s)' % w[7:]
    if w.startswith('reaches a place of type'): return '7 use: a place of the class type (%s)' % w[24:]
    if 'lambda implements' in w or 'method reference' in w or 'MEMBER_REFERENCE' in w: return '8 lambda / method reference'
    if w.startswith('read at another instantiation'): return '9 generic class read at another instantiation'
    if w.startswith('not a loop over the entries') or w.startswith('no collection enters'): return '2 entry not from a loop over a typed map'
    if 'enters no typed declaration' in w or 'ends in no declaration' in w: return 'c builder chain ends in an erased place'
    if 'annotated field' in w or 'handle class' in w or 'record component' in w: return 'a field read by a library / handle / record'
    return 'b other (%s)' % w

S = {r[0]: r for r in slots}
def simulate(removed):
    """slots typed when the reason classes in `removed` no longer erase"""
    erased = set()
    for r in slots:
        ws = [w for w in r[5].split(' | ') if w]
        if any(cls_why(w)[0] not in removed for w in ws): erased.add(r[0])
    work = list(erased)
    while work:
        s = work.pop()
        for t in S[s][6].split(','):
            if t != '-' and t not in erased: erased.add(t); work.append(t)
    return len(slots) - len(erased)

print('== 1. Declarations of a collection type: %d' % len(decls))
t = C.defaultdict(C.Counter)
for d in decls: t[short(d[2])][d[5]] += 1
print('%-34s %6s %6s' % ('type', 'typed', 'erased'))
for k, c in sorted(t.items(), key=lambda kv: -sum(kv[1].values())): print('%-34s %6d %6d' % (k, c['typed'], c['erased']))
print('%-34s %6d %6d' % ('TOTAL', sum(c['typed'] for c in t.values()), sum(c['erased'] for c in t.values())))

print('\n== 2. Erased declarations by first-level reason')
er = [d for d in decls if d[5] == 'erased']
c = C.Counter(); ex = C.defaultdict(C.Counter)
root = {}
def roots(sid, seen=None):
    """direct reasons, or those of the erased declarations it exchanges values with"""
    return None
# reverse taint edges: who erases me
rev = C.defaultdict(list)
for r in slots:
    for x in r[6].split(','):
        if x != '-': rev[x].append(r[0])
def rootwhys(sid):
    seen, out, work = {sid}, [], [sid]
    while work:
        s = work.pop()
        ws = [w for w in S[s][5].split(' | ') if w]
        out += [(s, w) for w in ws]
        for p in rev[s]:
            if p not in seen and S[p][4] == 'erased': seen.add(p); work.append(p)
    return out
for d in er:
    if d[6] == '-':
        k = 'no declaration of the rule: ' + (re.sub(r'type arguments: .*', 'type arguments no typed collection holds', d[8]))
        c[k] += 1; ex[k][short(d[2]) + (': ' + d[8][16:] if d[8].startswith('type arg') else '')] += 1
    else:
        rw = rootwhys(d[6]); ks = sorted({cls_why(w) for _, w in rw})
        k = ks[0] if len(ks) == 1 else 'several: ' + ' + '.join(sorted({x[0] for x in ks}))
        c[k] += 1; ex[k][short(d[2])] += 1
for k, n in sorted(c.items(), key=lambda kv: kv[0]): print('%5d  %s   [%s]' % (n, k, ', '.join('%s %d' % kv for kv in ex[k].most_common(6))))

print('\n== 3. Colls declarations (%d, of which typed %d): typed after the reason classes are removed, cumulatively' % (len(slots), simulate(set())))
order = [('3', 'runtime functions / methods give typed results'), ('2', 'views of typed collections are typed (keySet, values, entrySet, subList, entries)'), ('5', 'nested collections typed (element of a collection is a typed collection)'),
         ('1', 'the constructors the typed family lacks'), ('4', 'protobuf lists typed'), ('6', 'program results of type-variable type, non-rule variables'), ('7', 'uses the view lacks'), ('8', 'lambdas and method references'), ('9', 'generic instantiations'), ('a', 'fields read by a library, handles, records'), ('b', 'other'), ('c', 'chains')]
rem = set(); prev = simulate(set())
for k, name in order:
    alone = simulate({k}) - simulate(set())
    rem.add(k); n = simulate(rem)
    print('%5d (+%4d; alone +%4d)  %s %s' % (n, n - prev, alone, k, name)); prev = n

print('\n== 4. Direct reasons on Colls declarations (a declaration can have several)')
c = C.Counter()
for r in slots:
    for w in r[5].split(' | '):
        if w: c[cls_why(w)] += 1
for k, n in sorted(c.items(), key=lambda kv: (kv[0][0], -kv[1])): print('%5d  %s' % (n, k))

print('\n== 5. Expressions that make a collection value (origins): %d' % len(origins))
def made_typed(o): return o[7] != '-' or o[11].startswith('typed')
def ocls(o):
    k, q = o[1], o[2].rsplit('/', 1)[0]
    if k in ('new', 'new-anon'): return '1 new'
    if k == 'cast': return 'x cast'
    if k == 'prog-tvar-result': return '6 program method with a type-variable result'
    cls, m = q.rsplit('.', 1)
    return libclass(k, cls, m)
t = C.defaultdict(C.Counter)
def native(o): return o[8] == 'foreach' and o[3].startswith('typed:map') and ocls(o).startswith('2')
for o in origins: t[ocls(o)]['typed' if made_typed(o) else 'native' if native(o) else 'erased'] += 1
print('%-84s %6s %6s %6s' % ('', 'typed', 'loop', 'erased'))
print('(typed: the value enters a typed declaration or is one; loop: a for-each over a view of a typed map, which reads the map)')
for k in sorted(t): print('%-84s %6d %6d %6d' % (k, t[k]['typed'], t[k]['native'], t[k]['erased']))
print('%-84s %6d %6d %6d' % ('TOTAL', sum(c['typed'] for c in t.values()), sum(c['native'] for c in t.values()), sum(c['erased'] for c in t.values())))

print('\n== 6. Erased origins by what makes them, with the place the value enters')
t = C.defaultdict(C.Counter); tot = C.Counter(); args = C.defaultdict(C.Counter)
def cons(o):
    w = o[8]
    if w in ('init', 'assign', 'return', 'arg'):
        return 'decl:' + ('typed' if o[11].startswith('typed') else 'erased')
    return w
for o in origins:
    if made_typed(o) or native(o): continue
    k = ocls(o)[0] + ' ' + short(o[2]) + ('' if o[3] == '-' or not o[3].startswith('typed') else ' [on a typed %s]' % o[3][6:])
    tot[k] += 1; t[k][cons(o)] += 1; args[k][o[6]] += 1
for k, n in sorted(tot.items(), key=lambda kv: (kv[0][0], -kv[1])):
    if n >= int(sys.argv[2] if len(sys.argv) > 2 else 3): print('%5d  %-58s %s   args: %s' % (n, k, ' '.join('%s=%d' % kv for kv in t[k].most_common()), ' '.join('%s=%d' % kv for kv in args[k].most_common(4))))
print('\n== 7. Erased origins by the worst kind of type argument')
BAD = ['raw', 'wild', 'tvar-erased', 'object', 'super', 'extends', 'array', 'other', 'optional', 'libgeneric', 'nested-coll', 'box', 'tvar-kept', 'nongeneric', 'ok']
c = C.Counter()
for o in origins:
    if made_typed(o): continue
    cats = o[6].split(',')
    c[min(cats, key=lambda x: BAD.index(x) if x in BAD else 0)] += 1
for k, n in c.most_common(): print('%5d  %s' % (n, k))
print('\n== 8. Erased declarations by the worst kind of type argument')
c = C.Counter()
for d in er:
    cats = d[4].split(',')
    c[min(cats, key=lambda x: BAD.index(x) if x in BAD else 0)] += 1
for k, n in c.most_common(): print('%5d  %s' % (n, k))
