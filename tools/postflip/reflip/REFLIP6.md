# The sixth flip (2026-11-21..23): the fifth flip's list on main's translator with rule "typed library interfaces"

STATUS: DONE (step 561 added at the end on 2026-11-23, its own section at the end). The list replays unattended from the base to a gated tree (four MATCH, runtime tests ok); every census row
and tracked count that is worse than the fifth flip's is explained by cause with counts below, or fixed (toolfix 40
second half, 42, 43, 44). Open items at the end.

What this is: main at 4f2b73a4 flipped (the translator with rule "typed library interfaces" on, all
rules on, the Java tree through prep 20) and the fifth flip's final step list (steps.txt of
main-r12-flip5, a634273a, through step 560) replayed on it. The first part (2026-11-21, up to
f4d588d2) made the list replay to the end; the second part (2026-11-23) took the census and the
tracked counts against the fifth flip, fixed the tools where they no longer read a form
(toolfix 40 second half, 42, 43, 44) and explains the rest with counts.

    overlay branch   main-r13-flip6        worktree $B/wt/flip6     (steps.txt, toolfix.sh 37-44, patches)
    replay worktrees $B/wt/replay6         (the working replay, detached, per-step commits; tags flip6-pass0-402,
                                            flip6-pass1, flip6-pass2 mark the first part's passes)
                     $B/wt/replay6b        (the unattended replay from the base)
    tree worktree    $B/wt/tree6           (branch flip6-tree, the gated tree)
    driver           tools/postflip/reflip/replay6.sh [NAME]   (REFLIP_OVERLAY as before)

## Base and overlay construction (as the fifth flip: 376c6936, e9b9516e, f6a53423, 4b50a363)

    57031208  freeze: tools/j2go/run.sh's output on main 4f2b73a4 as it is (814 g_*.go, externals.tsv);
              byte-identical to the tree gated on main-libgen-b5 (worktree agZ3).
    91e8a048  tools/postflip and tools/protogen as of the fifth flip's overlay (main-r12-flip5, a634273a),
              with replay6.sh (replay5.sh with this flip's base, overlay and replay worktree).
              rename-names.txt: 4 names more (250): eachOf_ (the translator's typed for-each cursor of
              rt_lg.go; post-flip patch 001 declares the slice cursor of that name) and tc_IndexProvider,
              tc_p_Node_Prop, tc_p_PolyfillUsageFinder_PolyfillUsage (conversions the translator now declares
              and the post-flip tools declare too). The other 118 top-level names new against the fifth
              flip's freeze are declared by no post-flip runtime patch and not in the fifth flip's final tree.
    3c672fd7  prep: gofmt; reflip/rename.sh (250 names: j2go's typed runtime family out of the post-flip
              family's way; eachOf_ becomes jeachOf_).
    64ac1148  prep: go/closure/rt_fn_erased.go and rt_lg_erased.go removed = THE BASE. They are the runtime
              of the other translations (build tags j2go_nofntypes, j2go_nolibgen); the post-flip tools and
              reflip/tc read every .go file without build constraints and would see their declarations twice.
    7c57ac5c  replay6.sh names the base 64ac1148.

The list: 271 commits replayed (fifth flip: 272; step 495 dropped), 165 with tool runs (165), 121 runtime
patches (119: + reflip6-elemst-rt, reflip6-tripst-rt, reflip6-nativet-rt, - 495), 1 manual patch (1).

## The runtime patches (first part)

The translator's runtime of rule "typed library interfaces" (rt_lg.go: the interfaces IterableT,
CollectionT, ListT, SetT, MapT, Map_EntryT, IteratorT; the views asListT / asSetT / asMapT / asCollT /
asIterable; X_un_, lgBox_, listAs_ .. map_EntryAs_, tlistOfT_; the conversions tconv_ of typed
collections as elements in g_0casts.go) sits on the translator's own typed family until step 402
unifies the two families. The patches that met it are in the table; step 402's runtime patch ports
rt_lg.go onto the post-flip family.

## Per-step table: dropped, adapted, regenerated

(every line is a `#=6` note in steps.txt at the step: 22 notes; counts from the tools' output in the
replay logs and from git grep of the trees)

| step | kind | what | why |
|---|---|---|---|
| 012 (1646c20a) | rt | regenerated | the same hunks (0 changed +/- lines), context re-made against the base's rt_coll.go |
| 013, 014, 015 | rt | regenerated (3-way merge by the driver) | context only (0 changed +/- lines) |
| 143 | rt | regenerated | the same hunks but one: the EnumSet case of ImmutableSet_copyOf_Iterable goes with that function, which main deleted (62ebd50a: no translation names it); the case of ImmutableSet_copyOf stays, the patch's own (rt_guava.go:543 of the final tree) |
| 202 (4e23efbd) | tool + rt | adapted | toolfix 37 (ranges reads the typed cursor jeachOf_(X) and `var it IteratorT[T] = C.iterator_t()`), runtime elemsT_ (reflip6-elemst-rt.patch): 518 sites, 92 erased, 426 typed (fifth flip 520: 512 erased, 8 typed collections), 103 left for a body that returns (103) |
| 322 (2c5fe6be) | rt (test) | regenerated | the test patch against the base's test files |
| 323 | rt | regenerated | 11 files against the base runtime (rt_lg.go, rt_jtimm.go among them), 52 changed lines against the fifth flip's patch |
| 402 | tool + rt | regenerated | rt_lg.go ported onto the post-flip family (TList, TMap, TSet, TDeque); the J-family files of the base deleted as they are; 21 files, 583 changed lines against the fifth flip's patch; runtime tests ok after the step |
| 403 (8c9c3988) | tool + rt | adapted | toolfix 41 and 44, runtime patch reflip6-nativet-rt.patch: native -kind list 423 declarations become slices (233 classes; 269 pointers in 130 classes), as the fifth flip (423, 233; 269, 130); map 145 (fifth 142), set 178 (178). Without 44: 375 (210); without 41 and 44: 259 (157) |
| 422 | manual | regenerated | the same edits; context moved by one line (CoalesceVariableNames) |
| 472 (d2350734) | tool | note | toolfix 40 reaches tcmp's later version only; 1 sort here (fifth 6), the others at 481 / 486 |
| 486 (f9d61144) | tool | adapted | toolfix 40 (with I(X_un_(v)), second part): forEachT_ 8, computeT_ 2, mergeT_ 3, as the fifth flip |
| 495 | rt | dropped | its one hunk boxes only the range of tlistView_.subList; the translator's runtime has a typed subList on the same storage since piece `list`, carried to tlistView_ by 402 |
| 511 | rt | regenerated | its test builds the erased list with immList_ (main's runtime has no ImmutableList_copyOf_ObjectArr) |
| 512 (e9d53d6c) | tool + rt | adapted | toolfix 38 and 42, runtime patch reflip6-tripst-rt.patch: 44 round trips (15 + 4 + 17 set addAll, 3 + 3 slice addAll, 2 sliceImm_); fifth flip 47 (its 3 immutableListCopyOf_(tc, L.asList()) are immutableListCopyOfT_ from the translator); without 42: 7 |
| 520, 521 | rt | regenerated (3-way merge by the driver) | context; 521's test declares tc_string itself (no step before 402 does on this flip) |
| 553 (684afd83) | tool | adapted | toolfix 39 (optstr: the kept map of Optional objects behind its typed interface) |
| 554b, 556 | rt | regenerated | rt_lg.go's typed iterators, cursors and views follow the patches' changes of the post-flip family (10 files / 38 lines; 8 files / 214 lines) |
| 189 (90376a5f) | tool | adapted | toolfix 43 (eqhash, second part): 344 calls decided (326 without; the 18 more are the lgBox_ operands) |

## Tool fixes added (toolfix.sh 37-44; each has its rule and proof obligation in the script)

- 37 ranges: the translator's typed for-each cursor jeachOf_(X) and the typed iterator are loop sources
  (`for x := range elemsT_(X)`), with elemsT_ (rt_lg_iter.go) making the same cursor.
- 38 trips: a function the runtime lacks that the tool only reads in a pattern (ImmutableList_copyOf_ObjectArr)
  leaves that pattern without subject instead of stopping the tool.
- 39 optstr: the one map of Optional<String> objects written behind its typed interface
  (MapT[string, Optional[string]] with tc_Optional_string_) is kept as the fifth flip kept the erased one.
- 40 tcmp, tfnapi, tgeneric: a typed collection given to an erased library function through its views for
  typed code (X_un_(v), asListT / asSetT / asMapT / asCollT / asIterable). Second part: tfnapi also reads
  I(X_un_(v)) (Iterable(List_un_(L.asListT())): the 2 Iterable.forEach of ClosureRewriteModule; this edit was
  the uncommitted change of the first part, never replayed by it; it is in the second part's replays).
- 41 native: asListT / asCollT / asIterable are views as asList is; the set kind reads S.asSetT().addAll.
- 42 trips (second part): the round trips through the views for typed code; the receiver form S.asSetT().addAll
  takes helpers that keep the typed view's behaviour for an immutable set (tsetAddAllOfT_, tsetAddSetT_,
  tsetAddListT_: rt_tripst.go, reflip6-tripst-rt.patch, TestTypedRoundTripsT).
- 43 eqhash (second part): lgBox_(v) of a value of a typed library interface XT is a value of the erased
  interface X that XT embeds; only the decisions that call through the value are taken (equalsDyn_ /
  objhashDyn_ forms), identity and method-expression forms are left.
- 44 native (second part): (a) tlistOfT_(c, x) is an ImmutableList, its slice sliceOfT_ / slicePtrOfT_;
  (b) a parameter of a typed library interface (IterableT, CollectionT, ListT) that its group only reads
  (typed cursor, elemsT_, get_t, X_un_ for a reader) gets the typed live view slicePtrViewT_ of the slice
  (rt_nativet.go, reflip6-nativet-rt.patch, TestNativeTypedViews). 44a only in the versions of native that
  know pointers to slices (an earlier version, step 205's, has no class ptr).

Every step's tools (166 tool revisions of the list) take toolfix.sh without error and a second run changes
nothing; every fixed version of native, trips, eqhash and tfnapi builds (19 versions).

Tried and not kept (second part): a composite unboxing for rw.TypedFn (the 4 sorts of typed entries, below)
and tcmp's step-472 version reading List_un_ (it found 6 more views at 472, all with comparators the tool
does not read; the later versions take them). Neither is in toolfix.sh.

## Intermediate gates (four MATCH, runtime tests ok in each)

    first part:  step 142 (gate-flip6-a-071, tree 561b8341), step 402 (b-402, 3b4630c8, before toolfix 37),
                 step 494 (c-248, 7664b406), end of pass 1 (d-pass1, f6376364), end of pass 2 (e-pass2, 899c4e20)
    second part: the working replay with toolfix 40 (second half) and 42-44, replayed from step 189
                 (replay6, tag flip6-pass3, tip c62309bd, go/ tree 310a7533): gate-flip6-f-pass3.log:
                 tests before: 100 / ok jsgomech/closure / checks, bundle, downlevel, advanced MATCH / tests after: 100

## Census: fourth flip (inb-selfgen 6525013f) | fifth (tree5d, 2abbf4e4) | sixth, first part (replay6 899c4e20) | sixth (replay6 c62309bd)

tools/postflip/census.py of the overlay (with the Object-slot row) over go/closure/g_*.go; the fourth column
from REFLIP5.md. Rows that differ between the fifth and the sixth:

| row | fourth | fifth | sixth (1st part) | sixth | against the fifth |
|---|---|---|---|---|---|
| Java collections (erased): constructors | 120 | 112 | 40 | 40 | better |
| typed TList/TMap/TSet constructors | 448 | 450 | 542 | 525 | (more of them are slices now: 17 fewer than the first part) |
| Guava immutable collections / builders | 155 | 149 | 43 | 43 | better |
| for-each via iterator objects | 105 | 105 | 34 | 34 | better |
| explicit Iterator use | 125 | 125 | 73 | 73 | better |
| interface casts icast_X( | 207 | 164 | 183 | 183 | explained below |
| class casts | 769 | 699 | 553 | 553 | better |
| type assertions x.(T) | 2,118 | 2,110 | 2,047 | 2,047 | better |
| boxing to any | 1,182 | 999 | 938 | 938 | better |
| unboxing from any | 256 | 224 | 133 | 131 | better |
| Optional | 73 | 73 | 75 | 75 | explained below |
| throw as panic( | 3,528 | 3,518 | 3,523 | 3,523 | explained below |
| Java equals/hashCode protocol | 77 | 76 | 84 | 72 | better (toolfix 43) |
| unsafe | 1 | 1 | 2 | 1 | equal (toolfix 42) |
| fields/params declared any | 680 | 571 | 597 | 595 | explained below |
| ... Object slots only | 526 | 425 | 450 | 448 | explained below |

Equal fifth / sixth: native slices made 100, native maps made 0, streams 0, self_ dispatch 0, anonymous
interface assertions 0, instanceof 71, type switches 27, lambda objects 37, adapter literals 39, catch_ 54,
defer 84, String_ helpers 121, StringBuilder 5, enum objects 1 / 1, Preconditions_ 6, threads 53, reflect 5.
Generated lines 261,791 -> 262,097; runtime Test functions (gate) 83 -> 100.

### The rows that are higher than the fifth flip's, by cause

- interface casts 164 -> 183 (+19). Every cause is a form the translator now writes; no tool lost a pattern:
  - +19 in g_0casts.go: the unbox functions of the conversions of typed collections as elements
    (`unbox: func(v any) ListT[X] { return listAs_(tc_X, icast_List(v)) }`: 9 List, 6 Map, 4 Map_Entry).
    The fifth flip's translator had no typed collection as an element, so no such conversion.
  - +8: the converting closures of the 4 sorts of typed entries (two map_EntryAs_(.., icast_Map_Entry(p))
    each), see "typed sorts" below.
  - DiGraph node and edge casts: 22 calls and 2 function values against 14 calls and 11 function values.
    The same casts (upcasts of x.self() of graph nodes); the translator's typed for-each puts them in the
    loop's first statement as calls where the fifth flip passed them as functions to elemsBy_, which the
    census row does not count. icast_ used as a function value in all: 95 -> 44.
  - the property bag and bridges of JSDocInfo (14 in JSDocInfo, 3 in g_0jsdocprops.go): Java's cast to List /
    Map of a stored value is the interface cast with the typed view (listAs_(tc, icast_List(v))) where the
    fifth flip's translator cast to the class (to_ArrayList / to_LinkedHashMap: 8 there, 0 now).
  - fewer: icast_Collection 9 -> 3, icast_Iterable 7 -> 1, others -6.
- Optional 73 -> 75 (+2): the two mentions of Optional[string] in the declaration of the conversion
  tc_Optional_string_ (g_0casts.go), which the translator declares for the typed map
  MakeDeclaredNamesUnique.ContextualRenamer.declarations (MapT[string, Optional[string]], kept by optstr with
  toolfix 39). The fifth flip's map was erased and had no conversion.
- throw as panic( 3,518 -> 3,523 (+5): 4 in Color.init_Color, where the translator writes the null checks of
  four typed set parameters inline (`if x == nil { panic(NPE) }`) and the fifth flip had
  Objects_requireNonNull(x.asSet(), ..) (4 fewer in the equals/hashCode row); 1 translator stub in
  g_zstubs.go (AbstractList.init_ArrayList_int).
- fields/params declared any 571 -> 595 (+24), Object slots 425 -> 448 (+23): +29 in g_0casts.go (the unbox
  functions above: `func(v any)` of 49 conversion literals against 21), +8 the (p0 any, p1 any) of the 4 entry
  comparator closures, +1 PolyfillUsageFinder; -15 RegExpTree. (The first part had 2 more: the forEach
  closures of ClosureRewriteModule, which toolfix 40's second half makes forEachT_.)

### Tracked counts (fourth | fifth | sixth first part | sixth)

| | fourth | fifth | 1st part | sixth | |
|---|---|---|---|---|---|
| mapComputeIfAbsentT_ | 20 | 20 | 20 | 20 | |
| forEachT_ | 8 | 8 | 6 | 8 | toolfix 40, second half |
| mergeT_ / computeT_ | 3 / 2 | 3 / 2 | 3 / 2 | 3 / 2 | |
| typed sorts (sortSliceF_, sortSlicePF_, sortTListF_, sortArrF_) | 23 | 24 | 20 | 20 | explained below |
| Compiler_runInCompilerThreadT_ | 6 | 6 | 6 | 6 | |
| tsetAddSet_ / tsetAddList_ / tsetAddAllOf_ (and the T forms) | | 36 | 0 | 36 | toolfix 42 |
| native -kind list at step 403: declarations made slices | | 423 | 259 (375 with 41) | 423 | toolfix 41, 44 |
| `map[` occurrences in g_*.go (grep -o) | | 704 | 701 | 701 | see open items |

Typed sorts 24 -> 20: the 4 sorts of entries (PerformanceTracker, ProfilingReport, PruningAnalysis,
TypeInferencePass). The fifth flip sorted erased Map_Entry values with a comparator over Map_Entry; here the
elements are typed entries Map_EntryT[K, V] and the translator's converting closure unboxes each erased
element with map_EntryAs_(tc_K, tc_V, icast_Map_Entry(p)). The typed sort needs K(box(x)) == x (fix 33's
obligation); for an entry that is a wrapper of an erased entry (rt_lg.go's map_EntryAs) lgBox_ gives the
wrapped entry and map_EntryAs_ makes a new wrapper, so the closure does not call the comparator with the
sorted objects themselves. The tool refuses it by its proof, rightly; a fix would need the comparator's
equality on equivalent wrappers, which no rule here shows. A rule was written for it and dropped (above).

## The unattended replay from the base and its gate

    bash tools/postflip/reflip/replay6.sh replay6b   (overlay e7d680ff1; a fresh worktree, one run from the base 64ac1148)
    REPLAY DONE: replayed 271 commits, no patch regenerated on the way, tip 2c5e324f,
    go/ tree 310a7533082073b370fbbb6f292784becf0b9491 (= the working replay's, replay6 c62309bd),
    go/closure tree 8d1e0145a02539f9d10f97304efbabb4ea498b5d   (worktree $B/wt/replay6b)
    tree worktree $B/wt/tree6 (branch flip6-tree, at 2c5e324f); $B/gate.sh tree6 flip6, log $B/logs/gate-flip6.log:

      GATE FAILED flip6        (the header: the j2go self-test line, as for every post-flip tree)
      tests before: 100
      lstat closure/g_*.go: no such file or directory
      removed 0 declarations in 0 rounds
      ok  	jsgomech/closure	3.013s
      checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
      bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
      downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
      advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
      runtime lines: 26061  test lines: 10023
      runtime lines of the default build: 26061  off-mode extras (build-tagged files): 0
      tests after: 100

No time or allocation was measured.

## Open items, not verified

- No time or allocation was measured (none was to be). The new runtime helpers (tsetAdd*T_, sliceOfT_,
  slicePtrViewT_) replace boxing round trips and views; whether that is faster was not timed.
- The 4 typed sorts of entries stay erased sorts with the converting closure (above).
- `map[` occurrences 704 -> 701 against the fifth flip with the same number of maps and sets made at step 403
  in the set kind (178) and 3 more in the map kind (145 / 142): the classes differ (SerializeTypesToPointers'
  set of property names, TypedAstSerializer's usedPropertyNames and Compiler's map keyed by Class stay
  typed here; Compiler's licensesFromChunks / currentChunkLicencesInTDeps and collections of AmbiguateProperties
  and ReplaceIdGenerators are Go maps here and not there). Not traced further.
- Patch 143's dropped EnumSet case: closed (steps.txt, note at 143): it was the case of
  ImmutableSet_copyOf_Iterable, a function main deleted (62ebd50a); ImmutableSet_copyOf keeps its case from the patch.
- Step 472: toolfix 40 does not reach tcmp's version of that step; nothing is lost at the end (481 / 486 take
  the sorts), the per-step count differs (1 against 6).
- The runtime tests of the gate of the working replay ran once on that tree (2.992 s); the second gate run
  (after its cache link was repaired) shows `(cached)` for the same tree.
- Worktrees: x5 and x6 (scratch of the second part) removed; replay6 (the working replay, tags flip6-pass0-402,
  flip6-pass1, flip6-pass2, flip6-pass3) kept with its cache $B/cache-replay6w; the first part's logs are
  kept ($B/logs/replay6-pass2.replay.log is the working replay's tool log up to the first part's end).

## Step 561 (2026-11-23): node type handles found by the object (htabself)

A Node keeps its JSType or Color as a uint32 handle (Node.jstypeOrColor, rt_ids.go's table of distinct
values). Storing one (setJSType, setColor) looked the value's interface words up in the table's
direct-mapped cache and, on a miss, its Go map. The rule (tools/postflip/htabself, rule and proof
obligation at its head): a class whose objects the generated setters store with htab_set_ (found by
type at the setters' calls: x.self() or obj_(x) of a class pointer x) gets a field htabId_ and a method
htabSlot_ (promoted to every class that embeds it); the runtime patch reflip6-htabself-rt.patch makes
htabRef_ assert the value to htabHolder_ and find its entry through the slot. The slot is believed only
when the entry it names holds the same interface words as the value; an entry made while the slot was
taken (the same object under another type) goes to the map, so which entry a value finds, the counts and
the reuse of freed entries are those of the map alone (TestHandleTableSlots: a model of the table without
slots over 50,000 random stores, copies, clears and pins of holders, embedded holders, their embedded
part, plain values and holders with a foreign handle in their slot). Values of other classes (the
source files that htab_pin_ keeps) take the map as before. Obligations the tool checks: htabId_ and
htabSlot_ are declared by nobody else; no value of a struct type holding a slot is copied, compared,
passed, returned, kept in a variable or used as a type argument (only &T{...} keyed, selection, &).
Not checked: copies through unsafe or reflect in the runtime.

    step 561: key 36636047 (tool dfff53a2, runtime patch reflip6-htabself-rt.patch), steps.txt at the end
    htabself: 2 sinks (htab_set_, Node.jstypeOrColor_set_), 2 classes, 2 stores: JSType (27 classes embed
              it by value), Color; left 0; 2 files (g_JSType.go, g_Color.go). Sizes unchanged (padding).

Measured (CHECKS on the 3500 subset, one thread, two CPU runs per binary; before = $B/bin/m_flip6, the tree
of tree6; the saved state sha256 1cb35c70ccb8... in all six runs):

| cum (s) | before | after |
|---|---|---|
| htab_set_ | 0.54 / 0.57 | 0.10 / 0.09 |
| htabRef_ | 0.53 / 0.58 | 0.10 / 0.11 |
| Node.setJSType | 0.75 / 0.81 | 0.28 / 0.30 |
| runtime.mapaccess2 (all callers) | 0.60 / 0.65 | 0.23 / 0.22 |
| Node.getJSType (not this step's subject) | 1.17 / 1.24 | 1.10 / 1.12 |
| total samples | 63.09 / 63.43 | 62.61 / 61.53 |

Mallocs (JSGO_MEMSTATS) 269,976,120 -> 269,856,352; bytes allocated 13,423,737,160 -> 13,394,585,496.
The typed read path for getJSType (handle -> *JSType without the any round trip) was not made: the slot
gives an object's handle, not a handle's object; a typed read needs a second rewrite of the generated
readers and its own obligation (as_JSType is the receiver of the promoted htabSlot_), not a consequence
of this one. getJSType's cost (to_JSType's assertion, ~0.6 s flat here) is left as it was.

    bash tools/postflip/reflip/replay6.sh replay6h   (a tail replay: $B/wt/replay6h made from replay6b's tip 2c5e324f)
    (replay6h was removed after the gate; its commit 88e20f2d is branch flip6-tree-h; its tool log is $B/logs/replay6h.replay.log)
    REPLAY DONE: replayed 1 commit (step 561), tip 88e20f2d, go/ tree 04cbc7cffec3c32c8d7bcf9bc40ba05036c37c09,
    go/closure tree 03874c988bbd6f536ac8ef1c82ce68ce5552e0eb (the step's four files byte-identical to the
    measured tree); tree worktree $B/wt/tree6h (branch flip6-tree-h, at 88e20f2d); $B/gate.sh tree6h flip6h,
    log $B/logs/gate-flip6h.log:

      GATE FAILED flip6h       (the header: the j2go self-test line, as for every post-flip tree)
      tests before: 101
      lstat closure/g_*.go: no such file or directory
      removed 0 declarations in 0 rounds
      ok  	jsgomech/closure	3.060s
      checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
      bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
      downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
      advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
      runtime lines: 26129  test lines: 10182
      runtime lines of the default build: 26129  off-mode extras (build-tagged files): 0
      tests after: 101

The list now: 272 commits replayed (+ step 561), 166 with tool runs, 122 runtime patches, 1 manual patch.
Not done: the replay from the base with step 561 (the tail replay resumes on replay6b's commits, whose
overlay e7d680ff1 differs from this one by the 143 note, step 561 and this record only).

## Step 562 (2026-11-23): final collection fields kept by value (inlinefld)

A Java field `final List<X> f = new ArrayList<>();` was a pointer field *TList[X] that finit_ set from
newTList_ / newTListN_ (and the same for maps and sets): one object for the header besides the holder.
The rule (tools/postflip/inlinefld, rule and proof obligation at its head): a field of a generated class
of type *TList[T], *TMap[K, V] or *TSet[K] assigned exactly once in the package, by its class's finit_ as
a statement of the body, from one of the six constructors, becomes a field of type TList[T] (etc.);
finit_ makes it in place with the constructor's twin (reflip6-inlinefld-rt.patch: initTList_, initTListN_,
initTMap_, initTMapN_, initTSet_, initTSetN_ in rt_tlist.go and rt_tmap.go, the same fields and the same
capacity; TestInPlaceCollections), and every use of it as a value is &x.f (a selection from it, x.f.m(...),
is left: Go takes the field's address). The collection's identity is the field's address, fixed for the
holder's life. Obligations the tool checks (a field or a class that fails is refused, with its reason):
the field is not compared (nil tests included: such a field is refused, its test not rewritten), its
address is not taken, it is not a literal's key, not read through a value, in a zero_ method or outside
generated code; the class's objects are not copied, compared, kept by value or used as a type argument
(htabself's check), are made only by an empty &T{} in a new_ function and never given to Object_clone;
finit_ is called once by each constructor of the class, as a statement, by no constructor that delegates to
another that calls it, the constructors are called only by new_ functions and init_ methods, and what a
constructor does before finit_ does not let the object escape (statements that do not mention it, field
assignments from expressions that do not, calls of its methods whose bodies pass the same test: the
superclass's init_ chain). Not checked: that new_ and the subclass constructors follow j2go's pattern.
Self-test: tools/postflip/inlinefld/inlinefld_test.go, four classes kept and one negative case per reason.

    step 562: key 6fe460ee (tool eed6e16c, runtime patch reflip6-inlinefld-rt.patch), steps.txt at the end
    inlinefld: 540 collection fields; 168 kept by value (117 classes; 84 reads as values become &x.f);
               refused 372: assigned outside its class's finit_ 296, initialized other than by a typed
               collection's constructor 51, tested for nil 11, read through a value 8, assigned more than
               once 5, its class's constructor lets the object escape before finit_ 1
               (JsMessageExtractor_ExtractMessagesVisitor); no class refused for copies, literals,
               Object_clone or its constructors; 103 files; a rerun changes nothing (168 already by value).

Measured (gast.py on the 3500 subset, three rounds in alternating order, one thread; before = $B/bin/m_flip6h,
the tree of tree6h; after = $B/bin/m_flip6i; sha 72b57cd286c9 in all six runs, checks.state sha256
1cb35c70ccb8... for both binaries; the host had a load of 1.2-2.7 from other work):

| run | before total CPU / wall | after total CPU / wall | before CHECKS CPU / wall | after CHECKS CPU / wall |
|---|---|---|---|---|
| round 0 | 120.43 / 106.31 | 119.15 / 101.29 | 63.67 / 56.38 | 62.94 / 51.04 |
| round 1 | 119.59 / 101.01 | 119.18 / 101.13 | 62.50 / 50.71 | 63.09 / 51.25 |
| round 2 | 119.99 / 101.71 | 122.13 / 103.52 | 62.62 / 51.11 | 64.37 / 51.99 |

No CPU gain above the noise (means 120.00 -> 120.15 s total, 62.93 -> 63.47 s CHECKS). Mallocs
(JSGO_MEMSTATS, CHECKS alone) 269,917,679 -> 259,993,507 (-9.9 M, -3.7%); bytes allocated
13,395,805,328 -> 13,323,017,208. The step is kept for its allocations and as the one rule; it does not
pay for itself in time on this subset.

    bash tools/postflip/reflip/replay6.sh replay6i   (a tail replay: $B/wt/replay6i made from tree6h's tip 88e20f2d)
    (replay6i was removed after the replay; its commit 11ae55ab is branch flip6-tree-i; its tool log is $B/logs/replay6i.replay.log)
    REPLAY DONE: replayed 1 commit (step 562), tip 11ae55ab, go/ tree ece9fb99082670197180353df425b19916de6955,
    go/closure tree c6ef191272615a28867caf3c7a05635b00c16c54 (= the measured tree's); tree worktree $B/wt/tree6i
    (branch flip6-tree-i, at 11ae55ab; its cache $B/cache-tree6i); $B/gate.sh tree6i flip6i, log $B/logs/gate-flip6i.log:

      GATE FAILED flip6i       (the header: the j2go self-test line, as for every post-flip tree)
      tests before: 102
      lstat closure/g_*.go: no such file or directory
      removed 0 declarations in 0 rounds
      ok  	jsgomech/closure	3.027s
      checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
      bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
      downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
      advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
      runtime lines: 26156  test lines: 10238
      runtime lines of the default build: 26156  off-mode extras (build-tagged files): 0
      tests after: 102

The list now: 272 commits replayed (+ steps 561, 562), 167 with tool runs, 123 runtime patches, 1 manual patch.
Not done: the replay from the base with steps 561 and 562; a CPU profile of the step (only gast totals and
malloc counts were taken); whether the heap kept alive by an interior pointer (a list that outlives its
holder now keeps the holder) costs anything on the full benchmark.
