# stages.py: assigns the accounting's R1 sites of rows 10, 11, 13, 16 (casts, assertions, unboxings) and 18 (adapters) to the
# stage of the rule that would remove them, by the interface method named in sites.tsv's detail column.
import collections as C,re
J='/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/javaisms'
names=["row","file","line","hot","sub","detail","extra","matched"]
S=[dict(zip(names,l.rstrip("\n").split("\t"))) for l in open(J+"/sites.tsv")]
FN={'apply','accept','compare','test','applyAsInt','applyAsLong','call','run','load','get'}
def stage(d):
    s=re.sub(r"^(local (Object \(any\)|a Java interface) initialised from: )+","",d["sub"]); det=d["detail"]
    if s.startswith("Java cast folded to a direct assertion of: "): s=s.split(": ",1)[1]; s=re.sub(r"^(local .*? initialised from: )+","",s)
    if "method listed by an interface" in s:
        m=det.split(": ")[-1]
        if m in FN: return 'F functional'
        if m=='compareTo': return 'K Comparable'
        return 'X other bridge ('+m[:18]+')' if m in('equalValues_impl','equals','clone_impl','valueT_') else 'X other bridge'
    if "func literal" in s or "func value" in s: return 'F functional'
    if s.startswith("element of a for-each") or s.startswith("range variable"):
        return 'I iteration' if 'Iterable' in det else 'C collection'
    if "method called through a Java interface" in s or "runtime function/method" in s:
        if re.match(r'(Supplier|Function|Predicate|Comparator|Consumer|Callable)',det): return 'F functional'
        if re.match(r'(Iterator|ListIterator|PeekingIterator)\.',det) or re.match(r'(Iterables_|Iterators_|first_|elemsAs_)',det): return 'I iteration'
        if re.match(r'(Optional_base)',det): return 'O Guava Optional'
        if re.match(r'(Map|List|Set|Deque|Queue|Collection|LinkedHashMap|ArrayList|Erased|Immutable|TEntry|TMap|TList|ConcurrentMap|PriorityQueue|TreeMap|Map_|Sets_|Lists_|Collections_|Multi)',det): return 'C collection'
        return 'R other runtime result ('+det[:22]+')'
    return None
def sel(d):
    r=d["row"]; s=d["sub"]
    if r in("10","11","16"):
        if s.startswith(("NOT CODE","helper declaration","cast function used as a value","inside a cast helper")): return False
        if "GUARDED" in d["extra"] or "catch_" in d["detail"] or d["matched"].startswith(".("): return False
        s2=re.sub(r"^(local (Object \(any\)|a Java interface) initialised from: )+","",s)
        if s2.startswith("self()") or "getProp" in d["detail"] or "Annotatable" in d["detail"] or "getFeatureSet" in d["detail"]: return False
        return stage(d) is not None
    if r=="13": return s.startswith("Java cast folded") and "self()" not in s
    return False
t=C.defaultdict(C.Counter); hot=C.Counter()
for d in S:
    if sel(d):
        st=stage(d) or 'R other'
        st=st if st[0] in 'FKICO' else 'X/R other'
        t[st][{"10":"icast","11":"cast","13":"assert","16":"unbox"}[d["row"]]]+=1
        if d["hot"]=="H": hot[st]+=1
    elif d["row"]=="18":
        t['F functional']['adapter']+=1
        if d["hot"]=="H": hot['F functional']+=1
cols=['cast','icast','assert','unbox','adapter']
print(f"{'stage':18s}"+"".join(f"{c:>8s}" for c in cols)+"   total  hot")
tot=C.Counter()
for k in sorted(t):
    print(f"{k:18s}"+"".join(f"{t[k][c]:8d}" for c in cols)+f"  {sum(t[k].values()):6d} {hot[k]:4d}"); tot.update(t[k])
print(f"{'TOTAL':18s}"+"".join(f"{tot[c]:8d}" for c in cols)+f"  {sum(tot.values()):6d}")
