# The seventh flip (2026-11-23..24): the sixth flip's list on main's translator with T1 (piece `fns`) and T2 (kinds sorted, enumset, cmaps, deque, guava)

STATUS: DONE (see "Where it stands" at the end): the list replays unattended from the base to a gated tree (four MATCH, runtime tests ok).

What this is: main at 1bee513e (the merge of main-libgen-t1 and main-libgen-t2, gated as t12_a; docs/libgen/COLLECTIONS-PLAN.md
section 8) flipped, and the sixth flip's final step list (steps.txt of main-r13-flip6 081b385e, through step 562: 273 commits)
replayed on it, as the sixth flip was made (REFLIP6.md).

    overlay branch   main-r14-flip7        worktree $B/wt/flip7     (steps.txt, toolfix.sh 37-48, patches, port402-7.py)
    replay worktrees $B/wt/replay7         (the working replay, per-step commits; tags flip7-pass1 (first complete pass,
                                            e2e failing, see 3), flip7-pass2 (the gated tree))
                     $B/wt/replay7b        (the unattended replay from the base)
    tree worktree    $B/wt/tree7           (branch flip7-tree, the gated tree; cache $B/cache-tree7)
    driver           tools/postflip/reflip/replay7.sh [NAME]   (replay6.sh with this flip's base and worktrees)

## 1. The merge (main-libgen-t12, fast-forwarded into main)

    0d090a2f  merge main-libgen-t1 (915c0e06, piece fns) into main 697100e9: no conflict
    c8f05ea6  merge main-libgen-t2 (43cfd88f): conflicts in docs/libgen/COLLECTIONS-PLAN.md (both appended a section 6:
              T2's is section 7, its subsections and cross-references renumbered, the header note says so) and
              tools/j2go/README.md (both bullets kept; the switch list ends fns,sorted,enumset,cmaps,deque,guava).
              Ctx.java, ExprEmitter.java, selftest/run.sh merged without conflict; nothing else edited.
    1bee513e  COLLECTIONS-PLAN.md section 8 (this merge's record); main fast-forwarded 697100e9 -> 1bee513e.

md5 of `cat go/closure/g_*.go`, merged translator, merged runtime in the output directory:

| translation | md5 | must equal |
|---|---|---|
| everything off (J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all) | 973b5b6d3e4f71ab1cc82872360f9759 | main's all-off: equal |
| fns and the five T2 kinds off | f630308fb905a07052ac0ea004b9214c | main's default: equal |
| fns off | c7100373dd7ddbf3c4bb8fb4a6dba29c | T2's default: equal |
| the five T2 kinds off | 88d6f66b25b35b59b1ed0b8e6ef4a9c3 | T1's default: equal |
| default | 4440048ae31591b5be2bc353c8655e43 | the freeze below |

    $B/gate.sh libt12 t12_a:  GATE OK t12_a
      selftest: PASS / ok jsgomech/closure (cached) / checks, bundle, downlevel, advanced MATCH
      runtime lines: 20941  test lines: 4626; default build 20697, off-mode extras 244; tests 47
    (the first gate run printed `... 2 more` as its self-test line, the tail of a Java stack trace; the self-test alone
     and the gate run again printed `selftest: PASS`; kept as $B/logs/gate-t12_a.first-failed.log; cause not found)

## 2. Base and overlay construction (as the sixth flip: 57031208, 91e8a048, 3c672fd7, 64ac1148, 7c57ac5c)

    55dfdf14  freeze: tools/j2go/run.sh's output on main 1bee513e as it is (814 g_*.go, externals.tsv); md5 4440048a..,
              byte-identical (g_*.go and externals.tsv) to the tree gated as t12_a.
    fdd18d3c  tools/postflip and tools/protogen as of the sixth flip's overlay (main-r13-flip6 081b385e), with replay7.sh.
              rename-names.txt unchanged (250 names): the top-level names of this freeze and its runtime that the sixth
              flip's freeze did not have are 65 (T1's twins, T2's QueueT / DequeT / PeekingIteratorT / LoadingCacheT with
              their _un_ and As_ functions, tenumSet*_, 11 tc_ conversions, 4 tests); none is declared in the sixth flip's
              final tree (flip6-tree-i) or named in an added line of a patch of tools/postflip/patches or reflip/patches.
              19 names are gone against that freeze (the XOf aliases and as_ helpers of the T2 types, tc_Deque,
              icast_Runnable); the only patches that name some of them are manual patches not in the list.
    a65f120a  prep: gofmt; reflip/rename.sh (250 names; the J family).
    f8d67cef  prep: go/closure/rt_fn_erased.go and rt_lg_erased.go removed = THE BASE (build tags j2go_nofntypes,
              j2go_nolibgen). rt_lgfn.go and rt_lg_t2_fn_test.go (tag !j2go_nofntypes, default build) stay.
    b187e8bf  replay7.sh names the base f8d67cef.

The runtime of the base against the sixth flip's base 64ac1148: rt_lgfn.go (T1, 320 lines), rt_lg_t2.go (T2, 256),
their tests, and T1's 4 lines in rt_jtset.go / rt_jtsorted.go (teq_.cmpT); nothing else.

The list: 273 commits replayed (sixth: 273), 167 with tool runs (167), 125 patch lines (124: + reflip7-enumnull-rt),
of which 1 manual patch (422), as the sixth flip. Dropped: none.

## 3. Per-step table: dropped, adapted, regenerated, added

(every line is a `#=7` note in steps.txt at the step: 7 notes; counts from the tools' output in the replay logs)

| step | kind | what | why |
|---|---|---|---|
| 402 (a5732999) | rt | regenerated | the sixth flip's patch, with rt_jtset.go and rt_jtsorted.go deleted as they are now (T1's teq_.cmpT), and T1's rt_lgfn.go and T2's rt_lg_t2.go ported onto the post-flip family after unify's renames, by reflip/port402-7.py (tmapView_; the typed comparator in tsort_.cmp, the post-flip sorted mode's own typed key comparison; splitToListST_ makes the list splitToListT_ makes; TSet.Add for addSorted; tdequeFace_ with the typed deque methods, asDequeT through asDeque). 23 files (sixth 21), 45 changed +/- lines against the sixth flip's patch. Runtime tests ok after the step |
| 403 (8c9c3988) | tool | adapted | toolfix 45 (native): native -kind list 423 declarations become slices (233 classes; 269 pointers in 130 classes), as the sixth flip; without 45: 388 (211; 247, 120) |
| 99 (e9c01a2d) | rt | added | reflip7-enumnull-rt.patch: jisnull_ takes the zero of an integer enum as null. Without it the first complete pass (tag flip7-pass1, 273 commits) failed all four e2e comparisons: Map_mergeT_ (T1) of GatherGetterAndSetterProperties' map of PropertyAccessKind called the merge function with the null enum (index out of range in unionWith, INTERNAL COMPILER ERROR). In the sixth flip the same call was mergeT_ (tfnapi), which tests null with the map's conversion (vc.isNull) |
| 451 (235bd03d) | tool | adapted | toolfix 46 (tdeque): 66 deque classes become typed lists (27 queues, 39 stacks), its native -kind list 73 slices (61 classes), as the sixth flip; without 46: 52 (18, 34), 58 (47); 19 classes "used through asDequeT" |
| 453 (2cd604d6) | tool | adapted | toolfix 48 (gmaps): LinkedIdentityHashMap.innerMap is *TMap[K, ..] with tconvIdT_[K] (2 Get, 1 Put), as the sixth flip; without 48 it stays a wrapper of the erased IdentityHashMap and step 553 stops (below) |
| 553 (684afd83) | tool | adapted | toolfix 47 (optstr): the twin read of the kept map; 1,379 Optional[string] -> string (sixth 1,378: the twin's type argument) |
| 556 (c5b33687) | rt | regenerated | the sixth flip's patch and rt_lgfn.go following it (the typed map is its own view: v.tm() for v.m), 9 files (8) |

The sixth flip's regenerated patches (012-015, 143, 322, 323, 495 dropped, 511, 520, 521, 554b, 422) applied as they are
(no 3-way regeneration on the way: $B/runs/replay7-adapted.txt was never written).

### Steps that find less because the translator already did it (counted; not adapted)

| step | tool | sixth flip | seventh | what took its place |
|---|---|---|---|---|
| 472 (d2350734) | tcmp | 1 typed sort | 0 | List_sortT_ / Collections_sortT_ (T1) |
| 481 (af12fcda) | tcmp | 14 typed sorts (8 natural, 6 with a comparator), 13 files | 0, 3 files | the same |
| 486 (f9d61144) | tfnapi | 11 (2 Iterable.forEach, 6 Map.forEach, 3 Map.merge), 9 files | 0, 1 file | Iterable_forEachT_, Map_forEachT_, Map_mergeT_ (T1) |
| 488 (78fc86ad) | tgeneric | 25 (5 getFirst, 8 getLast, 12 getOnlyElement), 21 files | 0, 1 file | Iterables_getFirstT_ / getLastT_ / getOnlyElementT_ (T1) |
| 417 (ab26441f) | adapters | 108 calls take the function (6 Map_computeIfAbsent) | 102 | Map_computeIfAbsentT_ (T1) |
| 90, 218, 403 (762dc496, 71fb1236, 8c9c3988) | colls -kind deque / treeset | 9-11 deque, 2 treeset declarations in classes (none typed) | 1, 1 | QueueT / DequeT, SetT (T2) |
| 63 (11648d2f) and later | native -kind imm | 28-29 ImmutableCollection declarations (1 slice) | 12-13 (1 slice) | ImmutableSortedSet / Map as SetT / MapT (T2) |

Tracked counts (fourth | fifth | sixth | seventh; grep -o over g_*.go of the final trees):

| | fifth | sixth (flip6-tree-i) | seventh | |
|---|---|---|---|---|
| mapComputeIfAbsentT_ | 20 | 20 | 20 | |
| forEachT_ (post-flip) | 8 | 8 | 0 | Map_forEachT_ 9, Iterable_forEachT_ 7 (T1) |
| mergeT_ / computeT_ | 3 / 2 | 3 / 2 | 0 / 2 | Map_mergeT_ 3 (T1) |
| typed sorts (sortSliceF_, sortSlicePF_, sortTListF_, sortArrF_) | 24 | 20 | 5 | List_sortT_ 16, Collections_sortT_ 10 (T1) |
| Compiler_runInCompilerThreadT_ | 6 | 6 | 6 | |
| tsetAdd* | 36 | 36 | 36 | |
| `map[` occurrences | 704 | 701 | 701 | |
| T1 twins (all) | | 0 | 167 | |

The twins of piece fns over a native slice take the typed live view slicePtrViewT_ (42 uses, sixth 18) where the sixth
flip's tgeneric wrote sliceGetOnlyElementT_ and tcmp sortSliceF_ on the slice; no rule here rewrites a twin into those
forms (not written; the CPU on the subset is level, section 6).

## 4. Tool fixes added (toolfix.sh 45-48; each has its rule and proof obligation in the script)

- 45 native: the twins of the runtime's readers and sorters (Iterables_getFirstT_ / getLastT_ / getOnlyElementT_,
  List_sortT_ / Collections_sortT_) are readers and sorters; the view made for the call is the typed live view
  slicePtrViewT_(c, &L) of an addressable slot (or the slot's pointer).
- 46 tdeque: Deque_un_(d.asDequeT()) and Queue_un_(d.asQueueT()) (kind deque) are d.asDeque().
- 47 optstr: Map_getOrDefault_sT_[Optional[string]](m, k, d) of the kept map is its erased read, rewritten to
  optstrUnbox_(Map_getOrDefault_s(Map_un_(m), k, optstrBox_(d))). (The first version recursed through isMapKept;
  the committed one reads the call's shape there and the kept field elsewhere.)
- 48 gmaps: a MapT[K, V] field of a generic class assigned mapAs_(kc, vc, Map(new_IdentityHashMap() /
  new_LinkedHashMap())) is the candidate of that kind; get_t / put_t are its get / put. Without it tconvT_ has no
  instantiation in the program, optstr's call graph takes equalsTT_[T] at every two-parameter signature and its proof
  of Compiler.idGeneratorMap fails (the path ran from SourceFile's FilePaths_clean through stringUnits_'s yield to
  mapEquals_ and Supplier.get; found by dumping the reachable sets of both flips).

Runtime patch added: reflip7-enumnull-rt.patch (rt_generic.go jisnull_, test TestGenericNullOfIntegerEnum).
Runtime patches regenerated: 402 (with port402-7.py), 556.

## 5. Gates (four MATCH, runtime tests ok in each)

Intermediate, on the working replay (worktree replay7, the second pass, after the enum-null patch), at the commits the
sixth flip gated (same subjects):

    commit 71  (cd535c7c, ~step 142)  $B/gate.sh g7a flip7-a-071: tests 53 -> 52 (dropbroken removes
               TestEnumSetAgainstTreeSet there, as in the sixth flip's a-071: 48 -> 47); ok 2.024s; 4 MATCH
    step 402   (d11be6ad)             $B/gate.sh g7b flip7-b-402: tests 76; ok 2.912s; 4 MATCH
    commit 248 (fe51b543)             $B/gate.sh g7c flip7-c-248: tests 83; ok 2.898s; 4 MATCH

The first complete pass (tag flip7-pass1, f6fee3c5, before the enum-null patch): $B/gate.sh tree7 flip7a: ok 3.006s;
checks, bundle, downlevel, advanced DIFF (INTERNAL COMPILER ERROR, section 3, step 99). The same tree with the
patch applied by hand (not committed, only to see that nothing else differs): 4 MATCH (flip7a2).

Final (the working replay's tip 7a9def2e = branch flip7-tree, worktree tree7): $B/gate.sh tree7 flip7, log
$B/logs/gate-flip7.log:

      GATE FAILED flip7        (the header: the j2go self-test line, as for every post-flip tree)
      tests before: 107
      lstat closure/g_*.go: no such file or directory
      removed 0 declarations in 0 rounds
      ok  	jsgomech/closure	2.972s
      checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
      bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
      downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
      advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
      runtime lines: 26739  test lines: 10359
      runtime lines of the default build: 26739  off-mode extras (build-tagged files): 0
      tests after: 107

Binary: $B/bin/m_flip7.

## 6. Census: sixth (flip6-tree-h 88e20f2d, REFLIP6's last column) | sixth with step 562 (flip6-tree-i 11ae55ab) | seventh (flip7-tree 7a9def2e)

tools/postflip/census.py of the overlay (byte-identical to the sixth flip's) over go/closure/g_*.go. Rows that differ:

| row | sixth (6h) | sixth (6i) | seventh | against 6i |
|---|---|---|---|---|
| Java collections (erased): constructors | 40 | 40 | 38 | -2 |
| typed TList/TMap/TSet constructors | 525 | 357 | 357 | (step 562 made 168 fields in place) |
| for-each via iterator objects | 34 | 34 | 32 | -2 |
| interface casts icast_X( | 183 | 183 | 167 | -16 |
| class casts | 553 | 553 | 501 | -52 |
| type assertions x.(T) | 2,047 | 2,047 | 2,039 | -8 |
| boxing to any | 938 | 938 | 885 | -53 |
| unboxing from any | 131 | 131 | 82 | -49 |
| try/catch emulation | 54 | 54 | 52 | -2 |
| throw as panic( | 3,523 | 3,523 | 3,520 | -3 |
| defer | 84 | 84 | 83 | -1 |
| fields/params declared any | 595 | 595 | 536 | -59 |
| ... Object slots only | 448 | 448 | 390 | -58 |

Equal in all three: Guava immutables 43, native slices made 100, native maps 0, explicit Iterator use 73, streams 0,
self_ dispatch 0, anonymous interface assertions 0, instanceof 71, type switches 27, lambda objects 37, adapter
literals 39, Optional 75, equals/hashCode 72, String_ helpers 121, StringBuilder 5, enum objects 1 / 1,
Preconditions_ 6, threads 53, reflect 5, unsafe 1. No row is higher than the sixth flip's.

Spellings the census does not count (grep -o over g_*.go, flip6-tree-i | seventh): XOf[ alias uses outside g_0aliases.go
175 | 115; X_un_( 366 | 321; xAs_( wrappers 256 | 260; lgBox_( 82 | 85; Map_un_( 55 | 36; erased Iterables_ calls 70 | 38;
EnumSet_ calls 33 | 7. Generated lines 262,105 | 261,955. Runtime (gate): 26,156 -> 26,739 lines (+320 rt_lgfn.go, +256
rt_lg_t2.go, +6 reflip7-enumnull-rt, ...); Test functions 102 -> 107.

### The Java-ism accounting (docs/javaisms6/classify6.py and summary.py, unchanged)

Run on the seventh tree with the pipeline of JAVAISMS6.md section 1 replayed by a script (it reproduces every
checked-in tsv, verdicts.tsv and summary.out of tree6h byte for byte: 15 files; it needs boxcensusd and the two join
scripts, which were never checked in, recovered from the session that wrote JAVAISMS6.md). classify6.py's rules name
functions, fields and files of tree6h, so its verdicts on another tree are only as good as those names; rows add up.

| | tree6h | seventh |
|---|---:|---:|
| census matches (verdicts.tsv lines, row 17 included) | 9,290 | 8,876 |
| DONE (the form jsgo has too) | 6,748 | 6,740 |
| RULE T1 (generic static library functions) | 283 | 89 |
| RULE T2 (the remaining library types) | 228 | 192 |
| REASON | 426 | 410 |
| not a site | 670 | 679 |
| not split | 551 | 383 (the typed constructors row: 525 -> 357, step 562) |

Rows that moved (seventh against tree6h): erased constructors 40 -> 38 (T2 9 -> 7); for-each via iterators 34 -> 32
(T6 7 -> 6, T2 1 -> 0); interface casts 183 -> 167 (RULE 16 -> 8: T2 10 -> 7, T1 6 -> 1; REASON 17 -> 9); class casts
553 -> 501 (RULE 125 -> 64: T1 60 -> 11, T2 33 -> 22, T6 2 -> 1; not a site 306 -> 315); type assertions 2,047 -> 2,039
(T1 7 -> 2); boxing 938 -> 885 (RULE 297 -> 244: T1 92 -> 50, T2 71 -> 59, T3 62 -> 63); unboxing 131 -> 82 (RULE 117
-> 68: T1 49 -> 7, T2 20 -> 13); recover/catch 54 -> 52, panic 3,523 -> 3,520, defer 84 -> 83 (DONE); Object slots
448 -> 390 (RULE 175 -> 124: T1 66 -> 15; REASON 70 -> 62). RULE totals: T1 283 -> 89, T2 228 -> 192, T3 62 -> 63,
T6 33 -> 30, T7 57 -> 58; the others equal.

## 7. The unattended replay from the base

    bash tools/postflip/reflip/replay7.sh replay7b   (overlay aab068d2; a fresh worktree $B/wt/replay7b, one run from f8d67cef)
    REPLAY DONE: replayed 273 commits, no patch regenerated on the way ($B/runs/replay7b-adapted.txt not written),
    tip 272782ac, go/ tree 1e42bf3d0bcd891084e967db946d5e4b0237560a (= the gated tree's, flip7-tree 7a9def2e),
    go/closure tree 5fd69e430e4bf546ca0068f1bea4bfcc6d2f6198; the whole commit tree 8d62979c (= the working replay's,
    tools/postflip included). Log $B/logs/replay7b.log, tool log $B/wt/replay7b/.replay.log.

The working replay was not one run: it stopped at step 402 (patch regenerated), was rewound to its commit of step 402 for toolfix 45 (step 403 on), ran
to 553 (optstr failed), was rewound to before step 451 for toolfixes 46-48, stopped at 556 (patch regenerated), and
was rewound to before step 99 for the enum-null patch. The unattended replay is the record that the list as committed
gives the gated tree.

## 8. Timing (CPU first)

    flock heavy.lock python3 proto/gast.py $B/gasub/flags3500.txt 3 f6i=$B/bin/m_flip6i f7=$B/bin/m_flip7
    (log $B/logs/gast-flip7.log; one thread; alternating order; output sha 72b57cd286c9 in all six runs; load 1.2-1.8)

| run | f6i total CPU / wall | f7 total CPU / wall | f6i CHECKS CPU | f7 CHECKS CPU | f6i OPT / FIN CPU | f7 OPT / FIN CPU |
|---|---|---|---|---|---|---|
| round 0 | 116.75 / 103.32 | 115.71 / 98.93 | 61.95 | 60.74 | 43.86 / 10.94 | 43.96 / 11.01 |
| round 1 | 116.32 / 99.18 | 116.06 / 98.55 | 61.52 | 61.15 | 43.87 / 10.93 | 43.69 / 11.22 |
| round 2 | 116.81 / 98.91 | 115.50 / 98.11 | 61.84 | 60.41 | 43.70 / 11.26 | 44.19 / 10.91 |
| mean | 116.63 | 115.76 (-0.7%) | 61.77 | 60.77 (-1.6%) | 43.81 / 11.04 | 43.95 / 11.05 |

Every f7 run's CHECKS CPU is below every f6i run's (by 0.4 to 1.5 s); total CPU -0.9 s on the means, within one binary's
spread elsewhere (about 1 s). OPTIMIZATIONS and FINALIZATIONS level. An earlier one-round run of the first pass with the
enum-null patch applied by hand (m_flip7a2, not the gated binary): 117.75 against 117.68 s. The full benchmark was not
run (to be run by the user). No allocation counts or profiles were taken.

## Where it stands

STATUS: DONE. The sixth flip's list (273 commits) replays unattended from the seventh flip's base to the gated tree
(four MATCH, runtime tests ok); no step dropped; 4 adapted with tool fixes 45-48, 2 runtime patches regenerated (402,
556), 1 runtime patch added (reflip7-enumnull-rt, step 99); no census row is higher than the sixth flip's.

Open items, not verified:
- The full benchmark (not run here). Subset CPU -0.7% (CHECKS -1.6%), three rounds.
- The first gate run of the merged translator printed a Java stack trace's tail as its self-test line; two later runs
  passed. Not explained.
- reflip7-enumnull-rt.patch fixes jisnull_ for integer enums; rt_core.go's isnull_ (the post-flip family's) has the same
  shape and was not looked at for integer enums (no failure seen; the sixth flip has it too).
- The twins of piece fns over native slices take a live typed view per call (slicePtrViewT_, 42 uses) where the sixth
  flip's tgeneric / tcmp wrote slice functions (sliceGetOnlyElementT_, sortSliceF_); no rule rewrites the twins into
  those forms. Not timed separately.
- classify6.py's verdicts on this tree rest on tree6h's names (its own caveat); the pipeline driver that reproduced
  JAVAISMS6's inputs (session scratch, jism/run.sh) depends on files never checked in (boxcensusd, two join scripts).
- LinkedIdentityHashMap.innerMap is back to the sixth flip's TMap only through toolfix 48; the other IdentityHashMap
  place of kind cmaps (TemplateTypeReplacer.visitedObjectTypes, a non-generic class, MapT over mapAs_) is not a gmaps
  candidate (gmaps reads generic classes only), as in the sixth flip (where it was an erased field).
