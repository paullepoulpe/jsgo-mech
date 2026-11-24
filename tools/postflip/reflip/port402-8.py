# port402-8.py: the eighth flip's port of the runtime files of piece objarg (rt_lgobj.go) and piece cmp (its fast path
# in the J family's tsortCmp_, rt_jtsorted.go) onto the post-flip family at step 402, after `t unify` (which renames
# JTMap, JTSet, get, remove in rt_lgobj.go) and the seventh flip's 402 patch (which deletes the J family, rt_jtsorted.go
# among it, as it is now). Run in go/closure. Every edit must match exactly the counted number of times. Its result is
# part of patches/402-a5732999-rt.patch (regenerated); this script is the record of how that part was made.
import sys
def edit(path, pairs):
    s = open(path).read()
    for old, new, n in pairs:
        c = s.count(old)
        if c != n:
            sys.exit("%s: %d (want %d): %r" % (path, c, n, old[:70]))
        s = s.replace(old, new)
    open(path, "w").write(s)

edit("rt_lgobj.go", [
    # the typed map's and set's erased views are tmapView_ (field m) and tsetView_ (field s, the set itself) in the
    # post-flip family; the sorted views tsortedMapView_ / tsortedSetView_ embed them, as the J family's did
    ("func (v *jtmapView_[K, V]) tmap_() unsafe.Pointer { return unsafe.Pointer(v.m) }\n",
     "func (v *tmapView_[K, V]) tmap_() unsafe.Pointer { return unsafe.Pointer(v.m) }\n", 1),
    ("func (v *jtsetView_[K]) tset_() unsafe.Pointer    { return unsafe.Pointer(v.s) }\n",
     "func (v *tsetView_[K]) tset_() unsafe.Pointer    { return unsafe.Pointer(v.s) }\n", 1),
    # a TSet is its TMap of K to struct{}: the key conversion is kc (JTSet's c)
    ("x != t.c.null", "x != t.kc.null", 2),
    ("(jtmapView_.", "(tmapView_.", 3),
    ("(jtsetView_.", "(tsetView_.", 2),
])
edit("rt_ttree.go", [
    # piece cmp: natural ordering of a class with compareTo_t compares by it (cmpTypedT_), as tsortCmp_ did before the
    # boxed comparison; the same three lines
    ("\treturn func(a, b K) int32 { return compareNatural_(kc.box(a), kc.box(b)) }\n}\n",
     "\treturn func(a, b K) int32 {\n\t\tif r, ok := cmpTypedT_(a, b); ok {\n\t\t\treturn r\n\t\t}\n\t\treturn compareNatural_(kc.box(a), kc.box(b))\n\t}\n}\n", 1),
])
edit("rt_lgcmp_test.go", [
    # the test's typed sorted set: a key added goes to its place (TSet.Add, as port402-7 for rt_lg_t2.go), and a removed
    # position is isDead in the post-flip family
    ("s.addSorted(", "s.Add(", 1),
    ("!s.dead(i)", "!s.isDead(i)", 1),
])
print("port402-8: ok")
