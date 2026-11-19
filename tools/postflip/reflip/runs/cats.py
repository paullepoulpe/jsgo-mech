# cats.py <pprof -top output with all nodes>: flat CPU by category
import re,collections,sys
cats=[('GC mark/sweep (background workers)',r'runtime\.(gc|scan|sweep|greyobject|markBits|findObject|spanOf|tryDeferToSpanScan|spanClass|bgsweep|wbBuf|gcDrain|(\(\*(gcWork|mheap|gcBits|spanInlineMarkBits|spanQueue|pallocData|pageAlloc|gcControllerState|limiterEvent|sweepLocked|sweepLocker|activeSweep)\)))|internal/runtime/gc|runtime\.typePointers|runtime\.extractHeapBits|runtime\.heapBits|runtime\.pcvalue|runtime\.arenaIndex|runtime\.bulkBarrier|runtime\.\(\*mspan\)\.(moveInlineMarks|sweep|refillAllocCache|markBitsForIndex|countAlloc|base|inlineMarkBits|scanUtilization|heapBitsSmallForAddr)'),
('allocation (mallocgc, slices, objects, strings)',r'runtime\.(malloc|mallocgc|newobject|makeslice|growslice|memclr|nextFree|(\(\*mcache\))|(\(\*mcentral\))|roundupsize|deductAssistCredit|profilealloc|publicationBarrier|convT|newarray|makemap|rawstring|concatstring|slicebytetostring|stringtoslicebyte|intstring|heapSetType|(\(\*mspan\)))'),
('Go maps and hashing',r'internal/runtime/maps|runtime\.(map|memhash|strhash|aeshash|interhash|nilinterhash|efaceeq|ifaceeq|interequal|nilinterequal|strequal|memequal)|memeqbody|aeshashbody'),
('interface assertions and casts',r'runtime\.(assert|typeAssert|getitab|(\(\*itabTableType\)))| (to_[A-Z]|cast_|icast_|is_[A-Z])'),
('memmove / string compare',r'runtime\.(memmove|cmpstring)|cmpbody|indexbytebody|countbody'),
('scheduler, syscalls, profiling, other runtime',r' (runtime\.|syscall|internal/|os\.|sync|time\.|compress/|bufio|io\.|strings\.|strconv|unicode|sort\.|slices\.|math|bytes\.|hash/)'),
('protobuf runtime (rt_proto)',r' \(?\*?(pb[A-Z]\w*|AbstractMessageLite\w*|CodedOutputStream|CodedInputStream|ByteString)\b|pbAppend|pbSize|pbField|pbValue|pbVarint|pbIsZero'),
('node store accessors (Node.x, htable_, idTable_)',r' Node\.|htable_|idTable_|htab_|nodeStore'),
('runtime collections (TMap, TList, TSet, HamtPMap, scopeVars_, erased)',r'TMap|TList|TSet|scopeVars_|tmap|tlist|slice[A-Z]\w*_|lihm|LinkedHashMap|HashMap|ArrayList|ImmutableList|ImmutableSet|ImmutableMap|HamtPMap|Erased|each_|foreach_|elems'),
]
tot=0; c=collections.Counter()
for line in open(sys.argv[1]):
    m=re.match(r'\s*([0-9.]+)(m?s)\s+[0-9.]+%\s+[0-9.]+%\s+[0-9.]+m?s\s+[0-9.]+%\s+(.*)$',line)
    if not m: continue
    v=float(m.group(1))*(0.001 if m.group(2)=='ms' else 1); name=' '+m.group(3)
    tot+=v
    for k,rx in cats:
        if re.search(rx,name): c[k]+=v; break
    else: c['translated compiler code (everything else)']+=v
print('total flat %.1fs'%tot)
for k,v in sorted(c.items(),key=lambda x:-x[1]): print('%6.2fs %5.1f%%  %s'%(v,100*v/tot,k))
