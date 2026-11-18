import sys,subprocess,re,collections
B,P=sys.argv[1],sys.argv[2]
out=subprocess.run(["go","tool","pprof","-top","-nodecount=1000000","-nodefraction=0","-edgefraction=0",B,P],capture_output=True,text=True).stdout.split('\n')
tot=float(re.search(r'Total samples = ([0-9.]+)s',"\n".join(out)).group(1))
def sec(v):
    if v.endswith("ms"): return float(v[:-2])/1000
    return float(v.rstrip("s") or 0)
cats=[
 ("GC mark/scan/barriers",r'runtime\.(scan|grey|findObject|gcDrain|gcMark|gcBgMark|gcw|\(\*gcWork\)|\(\*gcBits\)|markBits|typePointers|\(\*mspan\)\.(heapBits|typePointersOf|markBitsForIndex|objIndex|base|scanObjects)|tryDeferToSpanScan|spanSetScan|\(\*spanInlineMarkBits\)|\(\*spanQueue\)|\(\*localSpanQueue\)|\(\*spanSPMC\)|heapBits|bulkBarrier|wbBuf|\(\*wbBuf\)|gcWriteBarrier|wbZero|wbMove|gcFlushBgCredit|gcAssist|\(\*lfstack\)|trygetfull|getempty|putfull|handoff|markroot|spanOf|\(\*mheap\)\.spanOf|pageIndexOf|arenaIndex|\(\*limiterEvent\)|\(\*gcCPULimiterState\)|gcParkAssist|\(\*objptr|objectHeaders)|internal/runtime/gc\.|runtime\.(greentea|\(\*gcWork\)\.|scanSpan|scanObject)'),
 ("malloc/sweep/alloc helpers",r'runtime\.(malloc|nextFree|\(\*mcache\)|\(\*mcentral\)|\(\*mheap\)|newobject|growslice|makeslice|makemap|memclr|\(\*mspan\)|heapSetType|\(\*sweepLocked\)|sweepone|bgsweep|deductSweep|publicationBarrier|profilealloc|roundupsize|convT|\(\*spanSet\)|newMarkBits|newAllocBits|\(\*pageAlloc\)|\(\*pageCache\)|sysUsed|sysUnused|madvise|\(\*fixalloc\)|gcTrigger|gcStart|gcController|\(\*gcControllerState\)|rawstring|concatstring|slicebytetostring|stringtoslicebyte|rawbyteslice|newarray|memmove|mProf_|\(\*bucket\)|stkbucket|callers|tracebackPCs|\(\*unwinder\)|funcspdelta|pcvalue|findfunc|step|readvarint|saveblockevent|fpTracebackPCs|pcdatavalue|funcMaxSPDelta|funcInfo)|internal/runtime/gc\.SizeToSizeClass'),
 ("maps + hashing + equality",r'(runtime\.(map|aeshash|memhash|strhash|nilinter|efaceeq|ifaceeq|interhash|typehash|memequal|strequal|f64hash|interequal)|internal/runtime/maps|aeshashbody|memeqbody|internal/bytealg|cmpbody|indexbytebody)'),
 ("interface asserts/itab",r'runtime\.(assert|getitab|typeAssert|interfaceSwitch|\(\*itabTableType\)|itab|panicdottype|typesEqual)'),
 ("scheduler/OS/idle/other runtime",r'^(runtime|internal/|syscall|sync|os\.|io\.|bufio|time\.)'),
 ("runtime collections (rt_coll: htable_, ArrayList, each_, ...)",r'closure\.(\(\*htable_\)|\(\*ArrayList\)|\(\*LinkedHash|\(\*each_\)|foreach_|Map_|Collection_|\(\*mapKeys_\)|\(\*mapValues_\)|\(\*mapEntries_\)|\(\*hiter_\)|\(\*sliceIter_\)|\(\*ImmutableCollection\)|\(\*ImmutableMap\)|hashMapOf_|hashSetOf_|asMap_|asColl_|imm[A-Z])'),
]
g=collections.Counter(); on=False
for l in out:
    if 'flat%' in l: on=True; continue
    if not on or not l.strip(): continue
    f=l.split(None,5); name=f[5]; v=sec(f[0])
    for c,rx in cats:
        if re.search(rx,name): g[c]+=v; break
    else: g["translated code + rest of the handwritten runtime (flat)"]+=v
for c,v in sorted(g.items(), key=lambda x:-x[1]): print(f"{c:62s} {v:7.2f}s {100*v/tot:5.1f}%")
print(f"{'total samples':62s} {tot:7.2f}s")
