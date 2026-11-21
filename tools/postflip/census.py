# Java-ism census of the transpiled Go: counts each row's pattern in go/closure/g_*.go.
# usage: cd go/closure && python3 ../../tools/postflip/census.py   (the rows of JAVAISMS.md)
import re,glob,collections
pats=[
 ("Java collections (erased): constructors", r"\bnew_(ArrayList|LinkedList|ArrayDeque|HashMap|LinkedHashMap|HashSet|LinkedHashSet|TreeMap|TreeSet|IdentityHashMap|LinkedIdentityHashMap|PriorityQueue|EnumMap|EnumSet|BitSet|Stack|Vector)\w*\("),
 ("Java collections: typed TList/TMap/TSet constructors", r"\bnew(TList|TMap|TSet)\w*_?\("),
 ("Guava immutable collections / builders", r"\b(Immutable\w+_(of|copyOf|builder)\w*|new_Immutable\w+|immList_|immSet_|immMap_)\("),
 ("native Go slices made", r"\bmake\(\[\]"),
 ("native Go maps made", r"\bmake\(map\["),
 ("for-each via iterator objects (foreach_/each)", r"\bforeach_\(|\.each\(\)"),
 ("explicit Iterator use (.iterator()/hasNext)", r"\.iterator\(\)|\.hasNext\(\)"),
 ("streams (Stream/Collectors)", r"\bStream\w*_|\bCollectors_|\.stream\(\)"),
 ("dynamic dispatch through self_ (this.self_.(interface{...}))", r"self_\.\(interface"),
 ("anonymous interface assertions .(interface{", r"\.\(interface \{|\.\(interface\{"),
 ("interface casts icast_X(", r"\bicast_\w+\("),
 ("class casts to_X( / cast_[T](", r"\bto_[A-Z]\w*\(|\bcast_\["),
 ("instanceof is_X(", r"\bis_[A-Z\[]\w*"),
 ("boxing to any: obj_( / strobj_( / .box_() / any(", r"\bobj_\(|\bstrobj_\(|\.box_\(\)|\bany\("),
 ("unboxing from any: str_( / unboxing casts", r"\bstr_\(|\bNode_of_\(|\.\(int32\)|\.\(bool\)|\.\(string\)"),
 ("lambda objects &X_fn{func", r"_fn\{func"),
 ("Optional", r"\bOptional_\w+\(|\bOptional\b"),
 ("try/catch emulation: recover() / catch_ (Java try/catch)", r"\bcatch_\w*\(|\brecover\(\)"),
 ("throw as panic( (the Go idiom for an unchecked exception)", r"\bpanic\("),
 ("defer", r"\bdefer\b"),
 ("Java equals/hashCode protocol", r"\.equals\(|\.hashCode\(\)|\bObject_equals\(|\bObjects_\w+\(|\bobjequals_\(|\bobjhash_\("),
 ("String_* UTF-16 helpers (charAt, substring, indexOf...)", r"\bString_\w+\("),
 ("StringBuilder", r"\bStringBuilder\b|\bnew_StringBuilder"),
 ("enum objects: structs embedding Enum", r"^type \w+ struct \{\n\tEnum\n"),
 ("enum objects: calls on object enums (.Enum.x())", r"\.Enum\.(ordinal|name|toString)\(\)"),
 ("Preconditions_ checks", r"\bPreconditions_\w+\("),
 ("threads/executors/synchronized emulation", r"\b(Thread|ExecutorService|AbstractExecutorService|CompilerExecutor|Callable_fn|Future)\b|\.submit\(|sync\.Mutex"),
 ("reflect", r"\breflect\."),
 ("unsafe", r"\bunsafe\."),
]
files=sorted(glob.glob("g_*.go"))
txt={f:open(f).read() for f in files}
# declarations of type any
anydecl=0
for f,t in txt.items():
    anydecl+=len(re.findall(r"^\s+\w+\s+any\b",t,re.M))+len(re.findall(r"\b\w+ any[,)]",t))
for name,p in pats:
    rx=re.compile(p, re.M); n=sum(len(rx.findall(t)) for t in txt.values())
    nf=sum(1 for t in txt.values() if rx.search(t))
    print(f"{n:8d} in {nf:4d} files  {name}")
print(f"{anydecl:8d}                 fields/params declared any (approx)")

