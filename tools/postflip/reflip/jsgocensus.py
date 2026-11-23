# The census rows (census.py) counted in the hand port jsgo, by the idiom jsgo writes for the
# same Java-ism, for the "what stays" table of REFLIP.md. Non-test Go files under internal/.
# usage: python3 tools/postflip/reflip/jsgocensus.py [jsgo checkout]   (default /home/user/jsgo)
import re,glob,os,sys
root=sys.argv[1] if len(sys.argv)>1 else "/home/user/jsgo"
files=[f for f in glob.glob(os.path.join(root,"internal","**","*.go"),recursive=True) if not f.endswith("_test.go")]
txt={f:open(f,encoding="utf-8",errors="replace").read() for f in files}
pats=[
 ("collections from the Java-collection helpers (jutil.New*Map/Set/List/Deque)", r"\bjutil\.New\w*(Map|Set|List|Deque|Queue)\w*\["),
 ("native Go slices made (make([])", r"\bmake\(\[\]"),
 ("native Go maps made (make(map[)", r"\bmake\(map\["),
 ("explicit Iterator use (.Iterator()/HasNext())", r"\.Iterator\(\)|\.HasNext\(\)"),
 ("type switches (.(type))", r"\.\(type\)"),
 ("type assertions x.(T) (comma-ok and direct; .(type) is the row above)", r"\.\((?!type\))\*?[A-Za-z_][\w.]*(\[[^\]]*\])?\)"),
 ("panic(", r"\bpanic\("),
 ("recover()", r"\brecover\(\)"),
 ("defer", r"\bdefer\b"),
 ("Equals/HashCode calls and methods", r"\.Equals\(|\.HashCode\(\)|func \([^)]*\) (Equals|HashCode)\("),
 ("Optional", r"\bOptional\b|\bOptional\["),
 ("Optional-of-string: base.OptString spellings", r"\bOptString\b"),
 ("Optional-of-string: SomeString( / NoString() calls", r"\bSomeString\(|\bNoString\(\)"),
 ("Optional-of-string: functions returning (string, bool)", r"^func .*\) \(string, bool\) \{"),
 ("func values given where Java has a lambda adapter (no adapter literals): &X_tfn_Y{ / &X_fn{", r"&\w+_t?fn(_\w+)?\{"),
 ("Preconditions (jutil.Check*)", r"\bjutil\.Check\w*\("),
 ("strings.Builder", r"\bstrings\.Builder\b"),
 ("UTF-16 string helpers (jutil string functions)", r"\bjutil\.(String\w+|CharAt|CodePointAt|Substring\w*|IndexOf\w*|LastIndexOf\w*|ToJavaString)\("),
 ("threads/sync (sync., go statements)", r"\bsync\.\w+|\bgo func\(|\bgo [a-zA-Z_.]+\("),
 ("reflect", r"\breflect\."),
 ("unsafe", r"\bunsafe\."),
]
for name,p in pats:
    rx=re.compile(p,re.M); n=sum(len(rx.findall(t)) for t in txt.values()); nf=sum(1 for t in txt.values() if rx.search(t))
    print(f"{n:8d} in {nf:4d} files  {name}")
# the any row split as census.py splits it (its anyslots function, read from census.py)
src=open(os.path.join(os.path.dirname(os.path.abspath(__file__)),"..","census.py")).read(); ns={}
exec(src[src.index("def anyslots"):src.index("for name,p in pats")],{"re":re},ns)
a=ns["anyslots"](txt.values())
print(f"{a['total']:8d}                 fields/params declared any (approx; census.py's regex)")
print(f"{a['total']-a['tparam']-a['ret']-a['helper']:8d}                 fields/params declared any, Object slots only (less {a['tparam']} type parameters, {a['ret']} return any(, {a['helper']} cast-helper operands; census.py's split)")
print(f"{sum(t.count(chr(10)) for t in txt.values()):8d} lines in {len(files)} files")
