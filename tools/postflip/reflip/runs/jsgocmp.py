# jsgocmp.py: the same syntactic counts on jsgo's hand-ported Go and on a jsgo-mech tree
import re,glob,sys,os
def files(root,mech):
    if mech: return sorted(glob.glob(root+'/g_*.go'))
    return [f for f in glob.glob(root+'/**/*.go',recursive=True) if not f.endswith('_test.go')]
rows=[
 ('non-test Go lines', None),
 ('type assertions x.(T), comma-ok or not', r'\.\((?!type\))[A-Za-z_*\[\]. ,]+\)'),
 ('cast helper calls to_X( / cast_[T]( / icast_X( (mech only)', r'\b(to_[A-Z]\w*|icast_\w+)\(|\bcast_\['),
 ('type switches', r'switch [^\n{]*\.\(type\)'),
 ('instanceof helper calls is_X( (mech only)', r'\bis_[A-Z\[]\w*'),
 ('mentions of any / interface{}', r'\bany\b|interface\{\}'),
 ('equals methods declared', r'^func \([^)]*\) (Equals|equals|equals_impl)\('),
 ('hashCode methods declared', r'^func \([^)]*\) (HashCode|hashCode|hashCode_impl)\('),
 ('recover()', r'\brecover\(\)'),
 ('defer statements', r'^\s*defer\b'),
 ('panic(', r'\bpanic\('),
 ('function-typed parameters in signatures', r'^func [^\n]*\bfunc\('),
 ('go statements', r'^\s*go (func\b|[A-Za-z_.]+\()'),
 ('sync. uses', r'\bsync\.'),
 ('strings.Builder / strBuilder_', r'strings\.Builder|\bstrBuilder_\b'),
]
trees=[('jsgo (hand port)','/home/ubuntu/work/jsgo/internal',False)]+[(a.split('=')[0],a.split('=')[1],True) for a in sys.argv[1:]]
data={}
for name,root,mech in trees:
    src=[open(f,errors='replace').read() for f in files(root,mech)]
    d={}
    for label,rx in rows:
        if rx is None: d[label]=sum(s.count('\n') for s in src)
        else:
            r=re.compile(rx,re.M); d[label]=sum(len(r.findall(s)) for s in src)
    data[name]=d
print('| count | '+' | '.join(n for n,_,_ in trees)+' |'); print('|---|'+'---:|'*len(trees))
for label,_ in rows: print('| %s | %s |'%(label,' | '.join(format(data[n][label],',') for n,_,_ in trees)))
