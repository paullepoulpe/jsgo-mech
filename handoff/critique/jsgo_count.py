import re,glob,os,collections,sys
root=sys.argv[1]; mode=sys.argv[2]  # jsgo | ours
if mode=='jsgo':
    files=[os.path.join(d,f) for d,_,fs in os.walk(root) for f in fs if f.endswith('.go') and not f.endswith('_test.go') and 'testing' not in d and 'testjunit' not in d]
else:
    files=sorted(glob.glob(root+'/'+sys.argv[3]))
txt={f:open(f).read() for f in files}
print(len(files),'files',sum(t.count('\n') for t in txt.values()),'lines')
A=r'\.\((\*?[A-Za-z_][\w.]*(?:\[[^\]\n]*\])?|interface ?\{[^}\n]*\})\)'
pats=[
 ('assert all .(T)',A),
 ('assert comma-ok', r'\w+, (?:ok\w*|\w*[oO]k|is\w+|\w+) :?= [^\n=]*'+A),
 ('assert comma-_ (nullable cast)', r'\w+, _ :?= [^\n=]*'+A),
 ('_, ok (instanceof)', r'_, \w+ :?= [^\n=]*'+A),
 ('type switch', r'\.\(type\)'),
 ('...any', r'\.\.\.any\b'),
 ('map[any]', r'map\[any\]'),
 ('[]any', r'\[\]any\b'),
 ('field any', r'^\t+\w+(?:, \w+)*\s+any\s*(?://.*)?$'),
 ('param any', r'\b\w+ any[,)]'),
 ('result any', r'\) (?:any|\(any[,)]|\([^()\n]*, any\)) ?\{'),
 ('var any', r'\bvar \w+ any\b'),
 ('typeparam any', r'\[\w+(?:, \w+)* any[\],]|, \w+ any\]'),
 ('Equals(any) decl', r'^func \([^)]*\) [Ee]quals\w*\(\w+ any\) bool'),
 ('typed equals decl', r'^func \([^)]*\) [Ee]quals\w*\(\w+ [^a)][^)]*\) bool'),
 ('HashCode decl', r'^func \([^)]*\) [Hh]ashCode\w*\(\) (?:int32|int)'),
 ('.Equals( call', r'\.[Ee]quals\w*\('),
 ('.HashCode() call', r'\.[Hh]ashCode\(\)'),
 ('ObjectsEquals-like', r'\b(?:jutil\.)?(?:ObjectsEquals|ObjectsEqual|ObjectEquals|Objects_equals|Object_equals|objequals_|Objects_equal)\w*\('),
 ('ObjectHashCode-like', r'\b(?:jutil\.)?(?:ObjectHashCode|ObjectsHashCode|ObjectsHash|Objects_hash\w*|objhash_|Objects_hashCode)\('),
 ('any( conv', r'\bany\('),
 ('Sprintf/format', r'\b(?:fmt\.Sprintf|jutil\.StringFormat|String_format|jutil\.Format)\w*\('),
]
for n,p in pats:
    rx=re.compile(p,re.M); c=sum(len(rx.findall(t)) for t in txt.values()); nf=sum(1 for t in txt.values() if rx.search(t))
    print(f'{c:7d} in {nf:4d} files  {n}')
if len(sys.argv)>4 or mode=='jsgo':
    tg=collections.Counter()
    for t in txt.values():
        for m in re.finditer(A,t): tg[m.group(1)]+=1
    print(tg.most_common(40))
