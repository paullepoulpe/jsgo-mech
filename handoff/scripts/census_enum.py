import re,glob,sys
d=sys.argv[1]
files=sorted(glob.glob(d+"/g_*.go")); allt="\n".join(open(f).read() for f in files)
rtt="\n".join(open(f).read() for f in glob.glob(d+"/rt_*.go") if not f.endswith("_test.go"))
old=len(re.findall(r"\.ordinal\(\)|\bEnum\b",allt))
emb=re.findall(r"^type (\w+) struct \{\n\tEnum\n",allt,re.M)
rtemb=re.findall(r"^type (\w+) struct \{\n\tEnum\n",rtt,re.M)
names=sorted(set(emb)|set(rtemb))
ptr=len(re.findall(r"\*(?:%s)\b"%"|".join(names),allt)) if names else 0
objord=len(re.findall(r"\.Enum\.(?:ordinal|name|toString)\(\)",allt))
em=len(re.findall(r"^type EnumMap struct\{ TreeMap \}",rtt,re.M))
print(f"old pattern {old}; structs embedding Enum: g_ {len(emb)}, runtime {len(rtemb)}; *E uses in g_ {ptr}; x.Enum.ordinal/name/toString() {objord}; EnumMap as TreeMap {em}")
