# The casts of go/closure counted like for like with jsgo (CRITIQUE2.md section 1). jsgocensus.py
# counts every x.(T) of jsgo, comma-ok instanceof tests included, and its type switches. Here the
# same regex runs over g_*.go and its matches are split into what jsgo has no counterpart for
# (the class model's this.self_.(T) dispatch, the bodies of the cast helpers) and the casts jsgo
# writes as assertions: the cast helpers' calls (icast_X / to_X / cast_[T] / gcast_X, written
# inline in jsgo), is_X calls (jsgo's comma-ok instanceof), comma-ok tests, direct assertions.
# usage: cd go/closure && python3 ../../tools/postflip/reflip/likeforlike.py [jsgo checkout]
import re, glob, os, sys, collections
A = re.compile(r"\.\((?!type\))\*?[A-Za-z_][\w.]*(\[[^\]]*\])?\)")
HELPER = re.compile(r"func (icast_|to_|gcast_|is_)")
c = collections.Counter()
for f in sorted(glob.glob("g_*.go")):
    fn = ""
    for line in open(f):
        if line.startswith("func "):
            fn = line
        c["switch"] += len(re.findall(r"\.\(type\)", line))
        decl = line.startswith("func ")
        if not decl:
            c["cast call"] += len(re.findall(r"\bicast_\w+\(|\bto_[A-Z]\w*\(|\bcast_\[[^()]*?\]\(|\bgcast_\w+(\[[^()]*?\])?\(", line))
            c["is_ call"] += len(re.findall(r"\bis_[A-Z\[]\w*", line))
        for m in A.finditer(line):
            pre = line[: m.start()]
            if pre.endswith("self_"):
                k = "self_ dispatch"
            elif HELPER.match(fn):
                k = "helper body"
            elif re.match(r"\s*(if )?(\w+|_), \w+ :?= ", line):
                k = "comma-ok"
            else:
                k = "direct"
            c[k] += 1
ours = c["cast call"] + c["is_ call"] + c["comma-ok"] + c["direct"] + c["switch"]
root = sys.argv[1] if len(sys.argv) > 1 else "/home/user/jsgo"
js = collections.Counter()
for f in glob.glob(os.path.join(root, "internal", "**", "*.go"), recursive=True):
    if f.endswith("_test.go"):
        continue
    t = open(f, encoding="utf-8", errors="replace").read()
    js["assert"] += len(A.findall(t))
    js["switch"] += len(re.findall(r"\.\(type\)", t))
print(f"{c['self_ dispatch'] + c['helper body'] + c['comma-ok'] + c['direct']:8d}  x.(T) in g_*.go (jsgocensus.py's regex)")
print(f"{c['self_ dispatch']:8d}    this.self_.(T) dispatch (the class model; jsgo: Go interfaces)")
print(f"{c['helper body']:8d}    bodies of the cast helpers icast_/to_/gcast_/is_ (counted at their calls below)")
print(f"{c['comma-ok']:8d}    comma-ok tests")
print(f"{c['direct']:8d}    direct assertions (ParseTree's asX(), Object results)")
print(f"{c['cast call']:8d}  cast helper calls icast_X( / to_X( / cast_[T]( / gcast_X (jsgo writes them as x.(T))")
print(f"{c['is_ call']:8d}  is_X instanceof calls (jsgo: comma-ok)")
print(f"{c['switch']:8d}  type switches")
print(f"{ours:8d}  casts like for like (calls + is_ + comma-ok + direct + switches)")
print(f"{js['assert'] + js['switch']:8d}  jsgo: {js['assert']} x.(T) + {js['switch']} type switches")
