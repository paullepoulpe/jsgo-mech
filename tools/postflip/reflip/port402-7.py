# port402-7.py: the seventh flip's port of the runtime files of T1 (rt_lgfn.go, piece fns) and T2
# (rt_lg_t2.go) onto the post-flip family at step 402, after `t unify` (which renames jtconv_,
# JTSet, JTMap, JTList, jtc_X, get/getOrDefault of a typed map in them) and the sixth flip's 402
# patch (which deletes the J family). Run in go/closure. Every edit must match exactly the
# counted number of times. Its result is part of patches/402-a5732999-rt.patch (regenerated);
# this script is the record of how that part was made.
import sys
def edit(path, pairs):
    s = open(path).read()
    for old, new, n in pairs:
        c = s.count(old)
        if c != n:
            sys.exit("%s: %d (want %d): %r" % (path, c, n, old[:70]))
        s = s.replace(old, new)
    open(path, "w").write(s)

edit("rt_lgfn.go", [
    # the typed map's erased view is tmapView_ in the post-flip family
    ("m.(*jtmapView_[string, V])", "m.(*tmapView_[string, V])", 2),
    # the typed comparator: the post-flip sorted mode keeps the key comparison at the key type
    # (tsort_.cmp, which ttreeCmp_ otherwise makes from the erased comparator by boxing)
    ("its keys with it as they are (tsortCmp_);", "its keys with it as they are (tsort_.cmp);", 1),
    ("\t\ts.eq.cmpT = (func(K, K) int32)(cmp)\n", "\t\ts.x.sort.cmp = (func(K, K) int32)(cmp)\n", 1),
    ("\tm := newTSortedMapC_(kc, vc, cmpErased_(kc, cmp))\n\tif cmp != nil {\n\t\tm.eq.cmpT = (func(K, K) int32)(cmp)\n",
     "\tm := newTMap_(kc, vc)\n\tm.setSorted_(cmpErased_(kc, cmp))\n\tif cmp != nil {\n\t\tm.x.sort.cmp = (func(K, K) int32)(cmp)\n", 1),
    # Splitter.splitToList of a String: the list splitToListT_ makes in the post-flip family
    ("\tl := &TList[string]{c: c, items: sp.splitStrs_(s), twin: unmodMark_}\n\tl.view.l = l\n\treturn l\n",
     "\treturn &TList[string]{c: c, items: sp.splitStrs_(s), imm: true, unmod: true}\n", 1),
])
edit("rt_lg_t2.go", [
    # a key added to a typed set in its sorted mode goes to its place (TSet.Add -> putSorted)
    ("s.addSorted(", "s.Add(", 4),
    # the typed deque's erased view is made on demand (TDeque.asDeque), a *tdequeFace_
    ("\treturn &d.dview\n", "\td.asDeque()\n\treturn d.dview\n", 2),
    ("(v *jtdequeView_[T])", "(v *tdequeFace_[T])", 5),
])
print("port402-7: ok")
