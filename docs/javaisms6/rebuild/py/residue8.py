# residue8.py: RESIDUE8.md, a verdict for every site of the eighth flip's Java-ism accounting that is still
# marked RULE T1, T2, T3, T4, R4 or R5 (classify6.py's verdicts.tsv), now that those rules are on main.
#
# usage: python3 residue8.py ACCOUNTING_DIR BASE_DIR MAIN_DIR [-v] > residue8.out
#   ACCOUNTING_DIR  the output of run.sh on the eighth flip's tree ($B/runs/jism-flip8: verdicts.tsv, tree/go/closure)
#   BASE_DIR        g_*.go of the translator the eighth flip was made from (main f47fbe67, md5 fab9e90c; the same
#                   files as main 73e902a8 with J2GO_NOLIBGEN=ownwild,nbox,proto)
#   MAIN_DIR        g_*.go of main's current translation (73e902a8 default, md5 13f040cd; = J2GO_NOLIBGEN=proto
#                   on branch main-t5-resid)
# writes residue8.tsv (one line per site: rule, row, site, function, category, verdict, check) and prints the
# markdown tables of RESIDUE8.md (per rule: category, sites, verdict, reason or rule, jsgo citation) and the totals.
#
# Each site gets the first category of CATS whose rule, census row, label and source line match. Then the
# "taken on main" check, for every site: the category's check pattern is counted in the site's enclosing function
# in BASE_DIR and in MAIN_DIR; where MAIN has k fewer, the first k sites of that category in that function are
# "taken on main" (the rules ownwild / nbox, written after the flip, removed the construct), whatever the
# category's own verdict.
import collections, re, sys

ACC, BASE, MAIN = sys.argv[1:4]
VERBOSE = "-v" in sys.argv
TREE = ACC + "/tree/go/closure/"

_src = {}
def src(d, f):
    k = (d, f)
    if k not in _src:
        try:
            _src[k] = open(d + "/" + f).read().split("\n")
        except FileNotFoundError:
            _src[k] = None
    return _src[k]

FUNC = re.compile(r"^func (?:\(\w+ \*?(\w+)(?:\[[^\]]*\])?\) )?(\w+)")
def funcs(d, f):
    """{key: [lines]} of the Go file: key = Receiver.name or name."""
    L = src(d, f)
    out = {}
    if L is None:
        return out
    cur = None
    for l in L:
        m = FUNC.match(l)
        if m:
            cur = (m.group(1) + "." if m.group(1) else "") + m.group(2)
            out.setdefault(cur, [])
        if cur:
            out[cur].append(l)
        if l == "}":
            cur = None
    return out

def enclosing(f, ln):
    L = src(TREE, f)
    for j in range(ln - 1, -1, -1):
        m = FUNC.match(L[j])
        if m:
            return (m.group(1) + "." if m.group(1) else "") + m.group(2)
        if j < ln - 1 and L[j] == "}":
            return ""  # outside a function (a struct field, a var)
    return ""

def base_key(fs, key):
    """the function of a translation that the tree's function key comes from (post-flip steps add _impl, compareT_)"""
    for k in (key, re.sub(r"_impl$", "", key), re.sub(r"_impl(\d*)$", r"_\1", key)):
        if k in fs:
            return k
    return None

# ---------------------------------------------------------------------------------------------------------------
# Categories: (rule, id, rows or None, label regex, source-line regex, verdict, text, jsgo, check regex)
#   verdict: "gap" (a further mechanical rule could take it: text = the rule) or "reason" (text = the counted reason)
G, R = "gap", "reason"
CATS = []
def cat(rule, cid, rows, lab, line, verdict, text, jsgo, chk=None):
    CATS.append(dict(rule=rule, id=cid, rows=rows, lab=re.compile(lab), line=re.compile(line), verdict=verdict, text=text, jsgo=jsgo,
                     chk=re.compile(chk) if chk else None))

MANUAL = {}  # (file, line) of the tree -> (verdict, evidence): sites whose line the check cannot place in the translation
exec(open(__file__.replace("residue8.py", "residue8_cats.py")).read())

# ---------------------------------------------------------------------------------------------------------------
sites = []
for l in open(ACC + "/verdicts.tsv"):
    r, f, ln, v, lab = l.rstrip("\n").split("\t")
    if not v.startswith("RULE ") or r == "17":
        continue
    rule = v.split()[1]
    if rule not in ("T1", "T2", "T3", "T4", "R4", "R5"):
        continue
    text = src(TREE, f)[int(ln) - 1]
    c = next((c for c in CATS if c["rule"] == rule and (c["rows"] is None or int(r) in c["rows"]) and c["lab"].search(lab) and c["line"].search(f + "\t" + text)), None)
    sites.append(dict(rule=rule, row=int(r), file=f, line=int(ln), lab=lab, text=text.strip(), fn=enclosing(f, int(ln)), cat=c))

unmatched = [s for s in sites if s["cat"] is None]
if unmatched:
    for s in unmatched:
        print("UNMATCHED", s["rule"], s["row"], f'{s["file"]}:{s["line"]}', s["lab"], s["text"][:200], sep="\t", file=sys.stderr)
    sys.exit(1)

# The taken-on-main check, per site: the site's line is found in its scope (the enclosing function, else the file) of
# BASE_DIR (exactly, else the closest line, difflib), the scope of MAIN_DIR is aligned with it line by line
# (difflib), and the check pattern (the category's, else the census row's) is counted in the base line and in the
# main line it became. Fewer in MAIN (or the line gone): "taken on main".
import difflib
ROWCHK = {0: r"new_\w+\(", 2: r"Immutable\w*_(of|copyOf|builder)|new_Immutable", 5: r"foreach_\(", 10: r"icast_", 11: r"cast_\[|to_\w+\(",
          13: r"\.\(\*?\w", 15: r"obj_\(|strobj_\(|\.box_\(\)|any\(", 16: r"str_\(|Node_of_\(|unbox_\[", 100: r"\bany\b"}
def scope(d, f, fn):
    if fn:
        fs = funcs(d, f)
        k = base_key(fs, fn)
        if k is not None:
            return k, [x.strip() for x in fs[k]]
    L = src(d, f)
    return None, [x.strip() for x in L] if L is not None else []
for s in sites:
    s["verdict"], s["check"] = s["cat"]["verdict"], ""
    rx = s["cat"]["chk"] or re.compile(ROWCHK[s["row"]])
    if src(BASE, s["file"]) == src(MAIN, s["file"]):
        s["check"] = "file unchanged"
        continue
    if (s["file"], s["line"]) in MANUAL:
        s["verdict"], s["check"] = MANUAL[(s["file"], s["line"])]
        continue
    k, B = scope(BASE, s["file"], s["fn"])
    _, M = scope(MAIN, s["file"], k if k else "") if k else scope(MAIN, s["file"], "")
    if not B:
        s["check"] = "file not in the translation"
        continue
    if s["text"] in B:
        bi, how = B.index(s["text"]), "same line"
    else:
        cm = difflib.get_close_matches(s["text"], B, n=1, cutoff=0.6)
        if not cm:
            s["check"] = "no such line in the translation"
            continue
        bi, how = B.index(cm[0]), "closest line"
    nb = len(rx.findall(B[bi]))
    mline = None
    for tag, i1, i2, j1, j2 in difflib.SequenceMatcher(None, B, M, autojunk=False).get_opcodes():
        if i1 <= bi < i2:
            if tag == "equal" or (tag == "replace" and i2 - i1 == j2 - j1):
                mline = M[j1 + bi - i1]
            elif tag == "replace":
                cm = difflib.get_close_matches(B[bi], M[j1:j2], n=1, cutoff=0.5)
                mline = cm[0] if cm else None
            break
    nm = len(rx.findall(mline)) if mline is not None else 0
    s["check"] = f"{how}; pattern {nb} -> {nm}" + ("" if mline is not None else " (line gone)")
    if nm < nb:
        s["verdict"] = "taken"

with open("residue8.tsv", "w") as out:
    for s in sites:
        print(s["rule"], s["row"], f'{s["file"]}:{s["line"]}', s["fn"], s["cat"]["id"], s["verdict"], s["check"], sep="\t", file=out)

# ---------------------------------------------------------------------------------------------------------------
VN = {"gap": "gap for a rule", "reason": "counted reason", "taken": "taken on main"}
want = collections.Counter(s["rule"] for s in sites)
print("Sites by rule (the accounting's RULE counts, census row 17 not added):", ", ".join(f"{k} {want[k]}" for k in ("T1", "T2", "T3", "T4", "R4", "R5")))
print()
tot = collections.Counter()
for rule in ("T1", "T2", "T3", "T4", "R4", "R5"):
    ss = [s for s in sites if s["rule"] == rule]
    print(f"### {rule}: {len(ss)} sites\n")
    print("| category | sites | verdict | reason, or the rule that would take them | jsgo |")
    print("|---|---:|---|---|---|")
    bycat = collections.OrderedDict()
    for c in CATS:
        if c["rule"] == rule:
            bycat[c["id"]] = [s for s in ss if s["cat"] is c]
    for cid, cs in bycat.items():
        if not cs:
            continue
        c = cs[0]["cat"]
        vc = collections.Counter(s["verdict"] for s in cs)
        vs = ", ".join(f"{VN[v]} {n}" for v, n in vc.most_common())
        print(f"| {cid} | {len(cs)} | {vs} | {c['text']} | {c['jsgo']} |")
    vt = collections.Counter(s["verdict"] for s in ss)
    tot.update({(rule, v): n for v, n in vt.items()})
    print(f"\n{rule} total {len(ss)}: " + ", ".join(f"{VN[v]} {vt[v]}" for v in ("gap", "reason", "taken") if vt[v]) + "\n")
    print("<details><summary>sites</summary>\n")
    print("| site (eighth flip tree) | function | census row | category | verdict | check (base -> main) |")
    print("|---|---|---:|---|---|---|")
    for s in ss:
        print(f"| {s['file']}:{s['line']} | {s['fn']} | {s['row']} | {s['cat']['id']} | {VN[s['verdict']]} | {s['check']} |")
    print("\n</details>\n")
print("### Totals\n")
print("| rule | sites | gap for a rule | counted reason | taken on main |")
print("|---|---:|---:|---:|---:|")
for rule in ("T1", "T2", "T3", "T4", "R4", "R5"):
    print(f"| {rule} | {want[rule]} | {tot[(rule, 'gap')]} | {tot[(rule, 'reason')]} | {tot[(rule, 'taken')]} |")
print(f"| all | {len(sites)} | {sum(v for (r, k), v in tot.items() if k == 'gap')} | {sum(v for (r, k), v in tot.items() if k == 'reason')} | {sum(v for (r, k), v in tot.items() if k == 'taken')} |")
print()
gaps = collections.Counter()
for s in sites:
    if s["verdict"] == "gap":
        gaps[s["cat"]["id"]] += 1
print("Gaps by category:", ", ".join(f"{k} {v}" for k, v in gaps.most_common()))
