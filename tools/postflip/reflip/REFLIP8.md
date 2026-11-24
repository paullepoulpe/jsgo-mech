# The eighth flip (2026-11-24): the seventh flip's list on main's translator with T3 (kind goptional, piece seqcoll), piece objarg and piece cmp

STATUS: DONE (gated, timed; section 8). The list replays unattended from the base to the same tree as the working replay;
three intermediate gates and the final gate gave four MATCH and runtime tests ok. The final gate builds only with the Go
compiler's backend run serially (-c=1, section 5): the package alone needs more than 18 GB with the default backend, on
the seventh tree as well.

What this is: main at f47fbe67 (main-libgen-t45 merged; docs/PLAN-STATUS.md "Translator after the seventh flip") flipped,
and the seventh flip's final step list (steps.txt of main-r14-flip7 d76a852a: 273 commits) replayed on it, as the seventh
flip was made (REFLIP7.md).

    overlay branch   main-r15-flip8        worktree $B/wt/flip8     (steps.txt, toolfix.sh 37-51, patches, port402-8.py)
    replay worktrees $B/wt/replay8         (the working replay; tags flip8-pass1 (first complete pass, before toolfix 49-51),
                                            flip8-pass2 (the final tree))
                     $B/wt/replay8b        (the unattended replay from the base)
    tree worktree    $B/wt/tree8           (branch flip8-tree, the final tree; cache $B/cache-tree8)
    gate worktree    $B/wt/g8              (detached; the three intermediate gates)
    driver           tools/postflip/reflip/replay8.sh [NAME]   (replay7.sh with this flip's base and worktrees)

## 1. The translation

main's translator, tools/j2go/run.sh on main f47fbe67 (worktree flip8): 814 g_*.go, md5 of `cat go/closure/g_*.go`
fab9e90cd1e8e5cdb9032035e9e6b295, the md5 asked for. go/closure as a whole (g_*.go, externals.tsv, the runtime) is
byte-identical (diff -r) to the tree gated as t45b (worktree libt4, main-libgen-t45 1f1f109d; main differs from it by
docs/PLAN-STATUS.md only). Gate of that tree, $B/logs/gate-t45b.log: selftest: PASS; ok jsgomech/closure (cached); checks,
bundle, downlevel, advanced MATCH; runtime lines 21306, test lines 4845.

## 2. Base and overlay construction (as the seventh flip: 55dfdf14, fdd18d3c, a65f120a, f8d67cef, b187e8bf)

    31313cc6  freeze: tools/j2go/run.sh's output on main f47fbe67 as it is (814 g_*.go, externals.tsv); md5 fab9e90c..
    b65ee636  tools/postflip and tools/protogen as of the seventh flip's overlay (main-r14-flip7 d76a852a), with replay8.sh.
              rename-names.txt: 252 names (250 + tmapOf_, tsetOf_). Top-level names of this freeze and its runtime that
              the seventh flip's freeze did not have: 27 (GOptional and its functions, goptional_, isGOptional_,
              optGOptionalAs_/Of_, the four SequencedCollection_*T_ twins, Map_getK_/containsKeyK_/removeK_,
              Set_containsE_/removeE_, the interfaces tmapOf_/tsetOf_, ComparableT, cmpTypedT_, cmpTest_, 5 tests).
              tmapOf_ and tsetOf_ (rt_lgobj.go) are declared in the seventh flip's final tree (rt_tnested.go, other
              functions) and named in added lines of post-flip patches (6 each): they get the j prefix. The other 25 are
              neither. Gone: 3 (BoxedOf, Optional_baseOf, icast_SequencedCollection; see steps 153 and c13b5bff below).
              (computed with a go/parser lister of top-level names over the three trees, session scratch)
    ad69c248  prep: gofmt; reflip/rename.sh (252 names).
    29432ef6  prep: rt_fn_erased.go and rt_lg_erased.go removed = THE BASE (build tags j2go_nofntypes, j2go_nolibgen).
              rt_lg_gopt.go, rt_lgfn.go and the tests tagged !j2go_nofntypes stay (default build).
    274fef12  replay8.sh names the base 29432ef6.

The runtime of the base against the seventh flip's base f8d67cef: rt_lg_gopt.go (+195) and its test, rt_lgobj.go (+80)
and its test, rt_lgcmp.go (+26) and two tests, rt_lgfn.go (+50: seqcoll's twins, cmpTypedT_ in naturalT_), rt_fn.go
(cmpTypedT_ in Comparator_naturalOrder / comparing), rt_jtsorted.go (cmpTypedT_ in tsortCmp_). Nothing else.

The list: 273 commits (seventh: 273), 167 with tool runs (167), 124 runtime patch lines, 1 manual patch (422), as the
seventh flip (`replay-all.sh -steps reflip/steps.txt -list`, both overlays). Dropped: none.

## 3. Per-step table: dropped, adapted, regenerated, added

(every line is a `#=8` note in steps.txt at the step: 6 notes)

| step | kind | what | why |
|---|---|---|---|
| 153 (d8f40a65) | rt | regenerated | the test's getFirst of a set's erased view casts with `any(s.asSet()).(SequencedCollection)` where it called icast_SequencedCollection: piece seqcoll leaves no erased SequencedCollection cast, so the translator no longer emits that helper and the tests did not build (tnames -undo stopped at 2c5fe6be). 1 line |
| 402 (a5732999) | rt | regenerated | the seventh flip's patch with rt_jtsorted.go deleted as it is now, and the runtime of objarg and cmp ported onto the post-flip family by reflip/port402-8.py: tmap_ / tset_ on tmapView_ / tsetView_ (the sorted views embed them), the set's key conversion kc; cmp's cmpTypedT_ fast path moves from tsortCmp_ to ttreeCmp_ (the post-flip family's comparison of a sorted collection); rt_lgcmp_test.go's sorted set uses TSet.Add / isDead. 26 files (seventh 23), 38 changed +/- lines against the seventh flip's patch (10 comments). Runtime tests ok after the step (gate flip8-b-402; TestLgCmp: compareTo_t called, no bridge call, in a typed sorted set) |
| 403 (8c9c3988) | tool | adapted | toolfix 49 (native: seqcoll's twins on a list's typed view): native -kind list 423 declarations become slices (233 classes), as the seventh flip; without 49: 415 (232; 8 declarations "a view passed to SequencedCollection_addLastT_", FeatureCollector's context stack) |
| 451 (235bd03d) | tool | adapted | toolfix 50 (tdeque: seqcoll's twins on a deque's asCollT()): 66 deque classes become typed lists, native -kind list 73 slices (61 classes), as the seventh flip; without 50: 52 classes (18 queues, 34 stacks), 58 slices (47), 16 classes "used through asCollT" |
| 472/481 (af12fcda) | tool | adapted | toolfix 51 (tcmp and deadbridge read piece cmp's compareTo_t): 5 typed array sorts, deadbridge finds the 10 bridges and leaves them (1 "a value given to a generic function", 9 "its element conversion tc_p_ boxes"), as the seventh flip; without 51: 3 sorts, 2 left "an array sort without a typed comparison of its elements", deadbridge found none |
| 556 (c5b33687) | rt | regenerated | the seventh flip's patch and rt_lgobj.go following it (the typed map or set is its own view): tmap_ / tset_ give v.tm() / v.ts(). 10 files (9), 2 changed lines more. Type check ok; runtime tests at this step stopped by the session for low memory (section 5) |

No patch was regenerated by replay8.sh's 3-way merge ($B/runs/replay8-adapted.txt never written); the 402 conflict
stopped the driver ("CONFLICT", rt_jtsorted.go) and was regenerated by hand as recorded.

### Steps that find less or more because the translator already did it (counted from the tool output of the two
### unattended replays, replay7b and replay8b; not adapted)

| step | tool | seventh | eighth | what took its place |
|---|---|---|---|---|
| 467 (8cb67888) | implsplit | 14 methods split | 4; "does not start with its parameters' casts" 1 -> 11 | piece cmp's compareTo_t + bridge (10 classes) |
| 468 (90bf3805) | implsplit | left 1 | left 11 | the same 10 bridges |
| c13b5bff | optional -guava | comparisons with nil 3, 2 files | 2, 1 file | kind goptional (GOptional is a value); Optional_baseOf's sed finds nothing (the alias is gone) |
| 6503ac1e | casts | 117 call sites of kept to_X | 116 | |
| e9c01a2d | enums | types *T -> T 5,787 | 5,794 | |
| castfold runs (7ce0de72 ..) | castfold | 1,453 casts left | 1,411 | seqcoll (getFirst/getLast results no longer erased), objarg |
| 5e7b118a | castfold -assert | 154 written, 860 left | 139, 833 | the same casts are gone before the step |
| 3caf4058 / c15b748c | castfold | 809 / 797 left | 798 / 786 | |
| c6b6a357, 90376a5f | eqhash | left 220 / 134 | 222 / 136 (Objects_equals / hashCode of .box_() 18 -> 19 each) | GOptional's box_ |
| 90967e31, 2d204c0f | objslots | any slots 2,471 / 2,154 | 2,469 / 2,152 | |
| a5732999 | unify | 7,855 rewrites, 453 files | 7,876, 456 | the typed maps / sets of objarg's pairs |
| 8c9c3988 | tnames | 1,891 / 1,345 uses | 1,904 / 1,358 | the same |
| 28eb1868 icasts, 34f27fc5 gcasts, 17eaf110 enumin, 5ca2e382 lamparams, 20561916 nnassert | | 45, 49, 7, 499, 9 | 44, 48, 6, 498, 7 | |
| 6dcd650f deadeq, 27632e6f hashmemo, 2a41395d valplain | | | +1 left each (GOptional: a generic class / box_) | |
| c0e9eb59 | etaconv | 295 left | 296 | |
| exceptions (5b9d0976 ..) | exceptions | | same counts, one kept catch's path text differs | |
| strbytes (52f30a93, 738dc4f3) | strbytes | | same totals of positions left, their first reason differs | the attribution differs between runs of the same tree too (replay8 and replay8b, whose trees are identical): report order, not the code |

Equal in the two logs otherwise (40 of the tool blocks differ between replay7b's and replay8b's .replay.log, last block
per commit and command; every difference is in the table or a line-number shift of g_0casts.go).

## 4. Tool fixes added (toolfix.sh 49-51; each has its rule and proof obligation in the script)

- 49 native: SequencedCollection_addLastT_[T](L.asCollT(), x) as a statement is an append of x (already a T);
  getFirstT_ / getLastT_ are readers and removeLastT_ a changer of fix 45's typed live view slicePtrViewT_(c, &L). Only
  where fix 45 applies. Applied to the 6 versions of native that carry fix 45 (all build).
- 50 tdeque: SequencedCollection_{removeLast,getFirst,getLast}T_[T](d.asCollT()) of a typed deque is the use of the same
  name through asDeque (the element already at its type).
- 51 tcmp: compareTo_t is a class's typed natural order where it has no compareToT_ (signature (C) int32 and the
  no-subclass condition unchanged); deadbridge: `compareTo(p0) { return this.compareTo_t(..) }` is a bridge.

Runtime patches regenerated: 153 (test), 402 (with port402-8.py), 556. Added: none.

## 5. Gates

Intermediate, on worktree g8 at the replay8 commits with the subjects the seventh flip gated (flip8-pass2):

    commit 71  (f4e71ee8)  $B/gate.sh g8 flip8-a-071: tests 58 -> 57 (dropbroken removes TestEnumSetAgainstTreeSet, as
                            the seventh flip's a-071: 53 -> 52); ok 1.964s; checks, bundle, downlevel, advanced MATCH
    step 402   (64dc33c6)  $B/gate.sh g8 flip8-b-402: tests 81; ok 3.016s; 4 MATCH
    commit 248 (41d7c6f1)  $B/gate.sh g8 flip8-c-248: tests 88; ok 3.004s; 4 MATCH; runtime lines 25870, test lines 8422

Final (the working replay's tip cbb1d976 = branch flip8-tree, worktree tree8). Two runs of `$B/gate.sh tree8 flip8`
(the agent's, and one more) were stopped by the session for low memory during the runtime tests' build: the compile
process alone reached 21.5 GB on the idle 23 GB host. Measured afterwards in a memory-capped scope (systemd-run --user
--scope -p MemoryMax=..., `go build -a ./closure`, peak RSS of the compile process via -toolexec):

    tree7 (flip7-tree 7a9def2e), default backend: killed by the kernel OOM killer at the 10 GB and at the 18 GB cap
    tree8 (flip8-tree cbb1d976), default backend: killed at the 10 GB and at the 18 GB cap
    tree8, -gcflags=jsgomech/closure=-c=1:        built, compile peak 18,031,276 KB, 254.6 s

So the size is not new in this flip (PLAN-STATUS open item "compile memory"); the seventh flip's gate built before the
session began stopping processes under memory pressure. -c sets how many functions the compiler's backend compiles at
once; it does not change the object code (the four comparisons below check the output in any case). env.sh unsets
GOFLAGS, so the flag comes from a GOENV file, $B/goenv-c1 (the user go env plus
GOFLAGS=-gcflags=jsgomech/closure=-c=1), and the gate ran in a 20 GB scope:

    flock $B/heavy.lock systemd-run --user --scope -q -p MemoryMax=20G -p MemorySwapMax=0 \
        env GOENV=$B/goenv-c1 $B/gate.sh tree8 flip8

$B/logs/gate-flip8.log: tree cbb1d976 dirty: 1 (.cache); tests before 112, after 112; removed 0 declarations;
ok jsgomech/closure 2.946s; checks: MATCH (952 inputs), bundle: MATCH (3), downlevel: MATCH (2094), advanced: MATCH
(848); runtime lines 27106, test lines 10578; BUILT. The gate prints GATE FAILED for the selftest line only, as every
post-flip tree (the selftest runs the translator on the renamed tree: "lstat closure/g_*.go: no such file").
$B/bin/m_flip8: 149,064,989 bytes (m_flip7: 147,377,767).

## 6. Census: seventh (flip7-tree 7a9def2e) | eighth (flip8-tree cbb1d976)

tools/postflip/census.py of the overlay (byte-identical to the seventh flip's) over go/closure/g_*.go; the seventh
column reproduces REFLIP7 section 6. Rows that differ:

| row | seventh | eighth | |
|---|---:|---:|---:|
| interface casts icast_X( | 167 | 163 | -4 |
| class casts | 501 | 493 | -8 |
| type assertions x.(T) | 2,039 | 2,035 | -4 |
| boxing to any | 885 | 845 | -40 |
| unboxing from any | 82 | 79 | -3 |
| Optional | 75 | 41 | -34 |
| fields/params declared any | 536 | 534 | -2 |
| ... Object slots only | 390 | 389 | -1 |

Every other row equal. No row is higher than the seventh flip's.

Other counts (grep -o over g_*.go, seventh | eighth): XOf[ outside g_0aliases.go 115 | 74; X_un_( 321 | 316; xAs_( 264 |
262; lgBox_( 85 | 84; Map_un_( 36 | 36; erased Iterables_ 38 | 38; EnumSet_ 7 | 7; generated lines 261,955 | 261,924;
mapComputeIfAbsentT_ 20 | 20; Map_forEachT_ / Iterable_forEachT_ / Map_mergeT_ 9/7/3 both; typed sorts 5 | 5,
List_sortT_ 16, Collections_sortT_ 10 both; tsetAdd* 36; `map[` 701 both; slicePtrViewT_ 42 | 45; compareToT_ 42 | 0,
compareTo_t 0 | 22; GOptional 0 | 84; objarg twins 0 | 51 (Map_*K_ 36, Set_*E_ 15); seqcoll twins left 0 | 6.

### The Java-ism accounting (docs/javaisms6/rebuild/run.sh of main, unchanged)

`bash $MECH/docs/javaisms6/rebuild/run.sh <tree>/go/closure $B/runs/jism-flip{7,8}` on both trees; the seventh tree's
output reproduces REFLIP7 section 6's seventh column (8,876; DONE 6,740; T1 89; T2 192; REASON 410; not a site 679; not
split 383). classify6.py's names are tree6h's (its caveat).

| | seventh | eighth |
|---|---:|---:|
| census matches (verdicts.tsv lines) | 8,876 | 8,782 |
| DONE | 6,740 | 6,741 |
| RULE T1 | 89 | 83 |
| RULE T2 | 192 | 136 |
| RULE T3 | 63 | 28 |
| RULE T7 | 58 | 56 |
| REASON | 410 | 411 |
| not a site | 679 | 677 |
| not split | 383 | 388 (5 boxing sites) |

Rows that moved: interface casts 167 -> 163 (RULE 8 -> 5: T2 7 -> 5, T1 1 -> 0; not a site 85 -> 84); class casts 501 ->
493 (RULE 64 -> 56: T2 22 -> 16, T1 11 -> 9); type assertions 2,039 -> 2,035 (RULE 10 -> 7: T2 6 -> 4, T1 2 -> 1; not a
site 130 -> 129); boxing 885 -> 845 (DONE 417 -> 418, RULE 244 -> 197: T3 63 -> 28, T2 59 -> 49, T7 18 -> 16; REASON 211 ->
212; not split 0 -> 5); unboxing 82 -> 79 (RULE 68 -> 65: T2 13 -> 11, T1 7 -> 6); Optional 75 -> 41 (RULE T2 34 -> 0);
Object slots 390 -> 389 (T1 15 -> 14). Other RULE totals equal (T4 63, R5 56, R4 41, T6 30, T5 26, R7 22, F1 9).

## 7. The unattended replay from the base

    bash tools/postflip/reflip/replay8.sh replay8b   (overlay 044c71ee; a fresh worktree $B/wt/replay8b, one run from 29432ef6)
    REPLAY DONE: replayed 273 commits, no patch regenerated on the way ($B/runs/replay8b-adapted.txt not written),
    tip b7b9e465, go/ tree 76abaa9e53367d5955593536463a983f6e49fa03, go/closure tree d82ca2eaa5a96ca65c16778164df84219cf9057e;
    the whole commit tree 2342dbb9 = the working replay's tip cbb1d976 (flip8-tree), tools/postflip included.

The working replay was not one run: it stopped at 2c5fe6be (step 153's test patch), at step 402 (patch conflict), at
step 556 (type check), then after the first complete pass (flip8-pass1) was rewound to before step 403 for toolfixes
49-51 and run to the end (flip8-pass2).

## 8. Timing

Subset (proto/gast.py $B/gasub/flags3500.txt 3 f7=$B/bin/m_flip7 f8=$B/bin/m_flip8, interleaved, idle host;
$B/logs/gast-f7-f8.log), output sha 72b57cd286c9 in all six runs:

    f7  cpu 120.61 116.54 117.68  (median 117.68)  wall 106.52  99.33  99.75
    f8  cpu 117.61 115.82 116.91  (median 116.91)  wall  99.87  98.41  99.48

The eighth flip is not slower: CPU median -0.7%, within the spread of the runs.

Full benchmark (tools/bench/run_googleapis_bench.py, idle host, back to back, each in a 20 GB systemd scope; inside it
Go reports no memory limit and Java's heap is -Xmx20g as in every earlier run; logs $B/logs/ga-full-f8.log,
ga-full-java7.log), same states and chunks (cae7acad) on both sides:

    eighth flip     583.1 s  CHECKS 308.5  OPTIMIZATIONS 224.6  FINALIZATIONS 50.0
    pre-flip Java   441.5 s  CHECKS 200.2  OPTIMIZATIONS 201.4  FINALIZATIONS 39.9   Java/Go 0.76

(seventh flip's run: 575.7 against 429.5, 0.75.)

## Open items, not verified

- Why compiling the closure package needs more than 18 GB with the default backend (seventh and eighth tree alike);
  which functions dominate was not measured.
- port402-8.py moves piece cmp's fast path into ttreeCmp_; whether the sorted collections' natural order through it is
  faster than through the bridge was not timed.
- The objarg twins (51 calls) and 6 seqcoll twins stay in the final tree; no post-flip tool reads the objarg twins (the
  maps and sets they read are typed maps, not Go maps).
