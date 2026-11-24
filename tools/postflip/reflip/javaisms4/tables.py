# tables.py: the grouped counts used in JAVAISMS4.md, from sites.tsv (jclass), box_joined.tsv (boxjoin.py)
import collections,re
names=["row","file","line","hot","sub","detail","extra","matched"]
S=[dict(zip(names,l.rstrip("\n").split("\t"))) for l in open("sites.tsv")]
def ex(d,i):
    p=d["extra"].split(" | "); return p[i] if len(p)>i else ""
def grp(rows,key):
    c=collections.Counter(); h=collections.Counter()
    for d in rows:
        k=key(d); c[k]+=1
        if d["hot"]=="H": h[k]+=1
    return c,h
def show(title,rows,key):
    c,h=grp(rows,key)
    print("\n### %s  (total %d, in hot files %d)"%(title,sum(c.values()),sum(h.values())))
    for k,v in c.most_common(): print("| %s | %d | %d |"%(k,v,h[k]))
def castkey(d):
    s=d["sub"]
    if s.startswith("NOT CODE"): return "A0 not code (comment)"
    if s.startswith("helper declaration"): return "A1 helper declaration"
    if s.startswith("cast function used as a value"): return "A2 cast function as a value: "+s.split(": ",1)[1][:60]
    if s.startswith("inside a cast helper"): return "A3 fallback cast inside a gcast_ helper"
    g="GUARDED" in d["extra"]
    kn=ex(d,3)
    known = kn.startswith("ELEMENT TYPE KNOWN")
    s=re.sub(r"^local (Object \(any\)|a Java interface) initialised from: ","",s)
    s=re.sub(r"^local (Object \(any\)|a Java interface) initialised from: ","",s)
    if "catch_" in d["detail"] or (g and "Exception" in d["extra"] or g and "Throwable" in d["extra"]): return "B1 caught exception value (catch_(r)) cast after is_X(ex_) test"
    if g: return "B2 after an instanceof test of the same expression (not exceptions)"
    if s.startswith("self()"): return "C self(): downcast/cross-cast of a typed value in a class hierarchy"
    if "method listed by an interface" in s: return "D parameter of an erased-signature method (bridge: compare/apply/accept/equals...)"
    if "func literal" in s: return "D2 parameter of a func literal typed any"
    if s.startswith("element of a for-each") or s.startswith("range variable"):
        return "E for-each element of an erased collection: "+("element type named by the XOf[T] alias" if known else "carrier raw / does not name it")
    if "method called through a Java interface" in s:
        return "F result of an erased Java-interface method (Iterator.next, Map.get, List.get, Entry.getKey/Value, Function.apply...): "+("type named by alias" if known else "carrier raw / does not name it")
    if "runtime function/method" in s:
        return "G result of a runtime function/method (Map_get_s, Node.getProp, Optional.get, Iterables...): "+("type named by alias" if known else "carrier raw / does not name it")
    if "generated function/method" in s: return "H Object result of a generated function/method: "+d["detail"].split(".")[-1]
    if "func value" in s: return "I result of a func value typed func() any (Supplier/Function as func)"
    if s.startswith("field declared"): return "J Object field read"
    if "boxed on the spot" in s: return "K typed value boxed on the spot (boxT_ of a type parameter)"
    if "slice or map of Objects" in s: return "L element of []any / map of any"
    return "Z other: "+s[:70]
show("row 11 class casts",[d for d in S if d["row"]=="11"],castkey)
show("row 10 interface casts",[d for d in S if d["row"]=="10"],castkey)
show("row 16 unboxing",[d for d in S if d["row"]=="16"],lambda d: castkey(d) if not d["matched"].startswith(".(") else "M `.(bool)` comma-ok test of an Object")
def akey(d):
    s=d["sub"]
    if s.startswith("virtual dispatch: `if p"): return "V1 dispatch: exact-class test `p, _ := this.self_.(*C); p == this` ("+ex(d,0)+")"
    if s.startswith("virtual dispatch: this.self_"): return "V2 dispatch: this.self_.(*C).m_impl() (single implementation)"
    if s.startswith("other use of this.self_"): return "V3 other this.self_.(T)"
    if s.startswith("body of"): return "H body of a cast/instanceof helper: "+s.split("(")[1].rstrip(")")
    if s.startswith("instanceof as comma-ok"):
        src=s.split(" of: ",1)[1]
        src=re.sub(r"^(local (Object \(any\)|a Java interface) initialised from: )+","",src)
        if src.startswith("self()"): k="self() of a typed value"
        elif "method listed by an interface" in src: k="parameter of an erased-signature method ("+d["detail"].split(": ")[-1]+")" if "equals" in d["detail"] else "parameter of an erased-signature method (other)"
        else: k="other Object/interface value"
        return "I instanceof as comma-ok `p, ok := v.(T)` on "+k
    if s.startswith("Java cast folded"):
        src=s.split(" of: ",1)[1]
        src=re.sub(r"^(local (Object \(any\)|a Java interface) initialised from: )+","",src)
        g=" [guarded]" if "GUARDED" in d["extra"] else ""
        if src.startswith("self()"): return "C cast folded to v.(T): self() of a typed value"+g
        kn=" (type named by alias)" if ex(d,3).startswith("ELEMENT TYPE KNOWN") else ""
        return "C cast folded to v.(T): erased result/parameter"+kn+g
    return "Z "+s[:60]
show("row 13 type assertions",[d for d in S if d["row"]=="13"],akey)
show("row 14 type switches",[d for d in S if d["row"]=="14"],lambda d:d["sub"].split(" over")[0])
show("row 9 anonymous interface assertions",[d for d in S if d["row"]=="9"],lambda d:"body of a to_X helper of a generic class (interface{ as_X_() any })" if "_() any" in d["extra"] else "body of to_SourceMap_Format")
def adk(d):
    t=ex(d,0); fam=re.sub(r"_t?fn(_.*)?(\[.*)?$","",t)
    s=d["sub"]; det=d["detail"]
    if s=="receiver of a field/method selection": return "M adapter made only to take its erased method value: (&X_tfn{f}).%s"%det
    if "runtime" in s: return "R passed to a runtime function/method"
    if det.endswith("setInternalFactory") or det.endswith("setPreconditionCheck") or det.endswith("setCondition"): return "P passed to PassFactory.Builder."+det.split(".")[-1]
    if "CmdLineParser" in det: return "Q passed to the command-line parser (CmdLineParser options)"
    if "generated" in s and s.startswith("argument"): return "G passed to other generated code"
    if s.startswith("returned"): return "S returned"
    return "T stored in a variable/field"
show("row 18 adapter literals by destination",[d for d in S if d["row"]=="18"],adk)
show("row 18 adapter literals by wrapped function",[d for d in S if d["row"]=="18"],lambda d: ex(d,1)+", "+ex(d,2)+", "+ex(d,3))
show("row 18 by family",[d for d in S if d["row"]=="18"],lambda d: re.sub(r"_t?fn(_.*)?(\[.*)?$","",ex(d,0))+(" (typed _tfn_)" if "_tfn" in ex(d,0) else " (object _fn)"))
show("row 17 lambda objects",[d for d in S if d["row"]=="17"],lambda d: ex(d,0)+" | "+ex(d,2)+" | "+d["sub"])
for r,t in (("0","erased ctors"),("1","typed ctors"),("2","guava")):
    show("row %s %s by constructor"%(r,t),[d for d in S if d["row"]==r],lambda d: re.sub(r"(_Object)+","_Object..",d["matched"].rstrip("(")))
    show("row %s %s by destination"%(r,t),[d for d in S if d["row"]==r],lambda d: d["sub"]+((" :: "+d["detail"]) if r!="1" else ""))
show("row 3 make([]",[d for d in S if d["row"]=="3"],lambda d:d["sub"])
show("row 5 foreach",[d for d in S if d["row"]=="5"],lambda d:d["matched"]+" "+ex(d,1))
show("row 6 iterators",[d for d in S if d["row"]=="6"],lambda d:d["matched"]+" | "+d["sub"])
show("row 12 instanceof",[d for d in S if d["row"]=="12"],lambda d:d["sub"])
show("row 19 Optional",[d for d in S if d["row"]=="19"],lambda d:d["matched"]+" | "+d["sub"])
show("row 23 equals",[d for d in S if d["row"]=="23"],lambda d:d["matched"]+" | "+ex(d,0)+" | "+d["sub"])
show("row 24 String_",[d for d in S if d["row"]=="24"],lambda d:d["matched"])
show("row 29 threads",[d for d in S if d["row"]=="29"],lambda d:d["matched"]+" | "+d["sub"][:40]+" | "+d["file"])
