# joins the census "boxing" matches (matches.tsv row 15) with boxcensus's site dump (boxsites.tsv) by file+offset
import collections,sys
box={}
for l in open("boxsites.tsv"):
    f=l.rstrip("\n").split("\t")
    if f[2]: box[(f[0],int(f[1]))]=f
hotf={}
for l in open("sites.tsv"):
    f=l.split("\t"); hotf[f[1]]=f[3]
notcode=set()
for l in open("sites.tsv"):
    f=l.rstrip("\n").split("\t")
    if f[0]=="15" and f[4].startswith("NOT CODE"): notcode.add((f[1],f[2]))
cat=collections.Counter(); sink=collections.defaultdict(collections.Counter); alloc=collections.Counter(); hot=collections.Counter(); vt=collections.defaultdict(collections.Counter)
un=[]; n=0
out=open("box_joined.tsv","w")
for l in open("matches.tsv"):
    f=l.rstrip("\n").split("\t")
    if f[0]!="15": continue
    n+=1
    key=(f[1],int(f[2]))
    b=box.get(key)
    if b is None:
        if (f[1],f[3]) in notcode: c="NOT CODE: in a comment"
        else:
            c="conversion of a value that is already an interface, or a helper use boxcensus does not count"; un.append(f)
        cat[c]+=1; out.write("\t".join([f[1],f[3],c,"","","",f[4]])+"\n"); continue
    c=b[3]; cat[c]+=1; s=b[4]+((" on "+b[5]) if b[5] else ""); sink[c][s]+=1
    if b[6]=="true": alloc[c]+=1
    if hotf.get(f[1],"-")=="H": hot[c]+=1
    vt[c][b[7]]+=1
    out.write("\t".join([f[1],f[3],c,s,b[6],b[7],f[4],hotf.get(f[1],"-")])+"\n")
print(n,"matches")
for c,v in cat.most_common():
    print("%5d  %s   (allocating %d, in hot files %d)"%(v,c,alloc[c],hot[c]))
    if "-v" in sys.argv:
        for s,k in sink[c].most_common(8): print("        %4d %s"%(k,s))
        print("        value types:", ", ".join("%s %d"%(a,b) for a,b in vt[c].most_common(6)))
print("unmatched examples:")
for f in un[:12]: print("   ",f[1],f[3],f[5][:150])
