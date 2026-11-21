# The fifth flip (2026-11-21): the in-bounds list on main's translator with rule "function types"

STATUS: IN PROGRESS. See "Where this stands" at the end; everything above it is what was done.

What this is: main at 024a9c5f flipped (the translator with rule "function types" on, the Java
tree through prep 20) and the in-bounds step list of the fourth flip (steps.txt of
main-r11-flip4-inbounds, 02da66e7, through step 558) replayed on it.

    overlay branch   main-r12-flip5        worktree $B/wt/flip5
    replay worktree  $B/wt/replay5         (per-step commits, detached)
    driver           tools/postflip/reflip/replay5.sh [NAME]   (REFLIP_OVERLAY as before)
    scratch          $B/wt/replay5x        (detached; tool experiments, not part of the result)

## Base and overlay construction (mirrors the fourth flip: 6ee376d0, 8b9c7e77, dfdb5197)

    376c6936  freeze: tools/j2go/run.sh's output on main 024a9c5f as it is (813 g_*.go,
              externals.tsv). Byte-identical to the tree gated on main-libgen-b1 (worktree agW).
              .cache/build was rebuilt first with tools/gate0/build.sh (the copied cache was older
              than prep 19/20).
    e9b9516e  tools/postflip and tools/protogen as of the fourth flip's overlay (02da66e7), with
              replay5.sh (replay4i.sh with this flip's base and names)
    f6a53423  prep: reflip/rename.sh (the same 246 names) and gofmt. No name new in this flip (61
              top-level names against dfdb5197) is declared by a post-flip patch or in the fourth
              flip's final tree, so rename-names.txt is unchanged.
    4b50a363  prep: go/closure/rt_fn_erased.go removed = THE BASE. It is the runtime of the other
              translation (build tag j2go_nofntypes); the post-flip tools and reflip/tc read every
              .go file without build constraints and saw its declarations twice.

A finding about the fourth flip: its base dfdb5197 freezes main at 7d37e6e9 (prep 18). REFLIP4.md's
first paragraph says "prep steps through 20", which that base does not have. So this flip brings
to the post-flip steps, beside rule "function types": prep 19 and 20 (the Simple* collections keep
their type parameters and take typed keys), rules "typed builder chains", "identity functions" and
"field layout". Several steps lose their subject to those, not to "function types" (below).

## The runtime patches and the erased shape of a functional interface

The post-flip runtime patches were written when Supplier, Function, Comparator and the other 12
library functional interfaces were erased Go interfaces. Main's runtime names their erased shape
X_e (= X[any, ...]) wherever it wrote X (commit 1eb3e4f7 is that substitution over rt_*.go).
reflip/fnpatch.py makes the same substitution in a patch (code of the hunks of rt_*.go files,
not comments, not g_*.go hunks) and rewrites the pre-image blobs so that a 3-way merge still
works. 19 active patches changed by it (057 058 075 080 105 125 142 147 159 210 276 283 284 317
323 402 470 480 510; 003 and 011 by hand before the script existed). What still failed after that
is in the table.

## Per-step table: dropped, adapted, regenerated

(every line is a `#=5` note in steps.txt at the step; counts are from the tools' output and from
git grep at the commit before the step)

| step | kind | what | why |
|---|---|---|---|
| 003, 057, 058, 075, 080, 142, 317, 323 | rt | regenerated | X -> X_e (fnpatch.py); 057 also: ordinalCmpOne_ made with Comparator_of_ (no cmpFunc_ in main's runtime) |
| 105 | rt | dropped | Comparator combinators as objects of their own types (intKeyCmp_, reversedCmp_, ...) so that reversed() and the PriorityQueue can type-switch on a comparator. A Comparator is a Go func now; the combinators are rt_fn.go's typed closures; the functions the patch rewrote are not in the runtime (0 definitions) |
| 125 | rt | adapted | without Comparator_comparingInt_f; Iterables_filter_f, mapComputeIfAbsent_f, mapComputeIfAbsentObj_f as before |
| 147 | rt | dropped | only rewrites Comparator_comparingInt_f |
| 159 | rt | adapted | one conflict: the context held Functions_identity (now in rt_fn.go) |
| 210 | rt | adapted | Comparator_of_ / cmpReversed_ for cmpFunc_ / Comparator_reversed; the test compares a comparator with fnSame_ |
| 276 | rt | adapted | AtomicIntegerArray.updateAndGet_f alone (IntUnaryOperator is outside the rule: 1 literal). The six other _f variants dropped: their functions take a Go func from the translator; 0 adapter literals of those interfaces in the tree (72 &X_fn{ in all: 64 of 12 interfaces of the program or outside the rule, Iterable 4, Runnable 2, ThreadFactory 1, IntUnaryOperator 1) |
| 283 | rt | adapted | without Optional_base.transform_f |
| 284 | rt | dropped | four _f variants of functions that take a Go func; step 285 was a no-op already |
| 330 | rt (test) | dropped | reflip4-289a33a9-tests-rt.patch is main's 60a9e130, in the base |
| 336 | tool + rt | adapted | toolfix 28 and reflip5-fnconv-rt.patch: a function type as an element of a typed collection (Node.PROP_MAP_FOR_EQUALITY) |
| 402 | tool + rt | adapted | toolfix 29 (53 typed computeIfAbsent calls are ComputeIfAbsent); runtime patch regenerated: the four J-family files main changed are deleted as they are; JTMap.computeIfAbsent leaves rt_fn.go; ComputeIfAbsent throws for an immutable map; Comparator_of_ / Function_of_ in the sorted view and the tests. unify 7,409 rewrites (fourth flip 7,294). Runtime tests ok, TestUnifiedFamily's golden trace unchanged |
| 416 | manual | dropped | subject gone: the lambda is a Go func the compiler keeps on the stack ("func literal does not escape", go build -gcflags=-m); 0 &Function_fn{ |
| 417 | tool | adapted | toolfix 30: adapters reads a converted function literal (102 Optional.filter -> filterFn_, 6 Map_computeIfAbsent -> _f) |
| 419 | manual | dropped | subject gone from getOptionalNodeComparator (a typed func, no adapter, no cast); see open items for what is NOT the same on the comparison path |
| 422 | manual | regenerated | three hunks resolved as the patch's own substitution on the new text |
| 433 | rt | dropped | unboxedPred_ / unboxedCons_: tfnargs has no subject, nothing names them |
| 458 | rt | adapted | its test in a new rt_cmp_test.go (105, which made that file, is dropped) |
| 470 | rt | adapted | sortSliceF_ / sortSlicePF_ alone; Predicates_alwaysTrueT_ is rt_fn.go's alwaysTrueT_ |
| 480 | rt | adapted | Comparator_of_ for cmpFunc_ |
| 510 | rt | regenerated | two conflicts in rt_guava.go; cmpReversed_ |
| 553 | tool | adapted | toolfix 31 (optstr's call graph: forwarder methods of function types, generic literals per instantiation) |

## Tool fixes added (toolfix.sh 28-31; each has its reasoning in the script)

- 28 rw.ConvFor, colls, native: a function type as an element (box_(), fnUnbox_[F], isnull).
- 29 unify: the translator's typed computeIfAbsent is ComputeIfAbsent.
- 30 adapters: a function literal converted to a function type is the typed lambda.
- 31 optstr (cha.go): x.m(..) of a function type's forwarder method is the call x(..) (through
  the field's known stores when x is such a field); the method value x.m is x; a literal of
  generic code is reached per instantiation.

## Steps that run and find nothing, or less: their subject is in the translator's output

Counted with git grep at the commit before the step, and from the tools' own reports (the
replay log, $B/wt/replay5/.replay.log, against the fourth flip's $B/wt/replay4r/.replay.log).
`&X_tfn` literals: 0 in the base and before step 417 (fourth flip: 861). Adapter type
declarations in the base: 59 (fourth flip's base: 253).

| step | tool | fourth flip | here | why |
|---|---|---|---|---|
| 29, 35, 121 | strvar -rule A | 18 string variants of Simple* methods | 0 | prep 19/20: the Simple* collections are generic with typed keys from the translator |
| 122 | strkeys | 100 calls pass the string key to a helper | 0 | the same |
| 69 | lambdas | 64 lambdas hoisted | 41 | a library lambda is a func literal; the 41 are the program's own interfaces |
| 76, 136, 255, 277 | funcparams | 17 / 133 / 8 / 2 parameters | 4 / 50 / 3 / 0 | the library half is func types already; 277: 1 runtime call to a _f variant (updateAndGet_f) |
| 257 | generics -classes | 11 classes made generic | 4 | the Simple* classes are generic from the translator |
| 425 | adapters (second run) | 6 TMap.computeIfAbsent -> ComputeIfAbsentObj | 0 | see open items |
| 434, 437 | tfnargs | 62 + 30 arguments | 0 + 0 | the parameters are Predicate_base[Node], Consumer[Node], Supplier[string] from the translator |
| 443 | ident | 19 comparisons (60 JSCompObjects_identical left) | 0 (1 mention left: the declaration) | rule "identity functions" |
| 457 | lamparams | 183 lambda parameters, 8 adapter types | 0 | no adapter objects. The lambdas at an erased place are written at the place's type and convert on entry (147 literals X[any..](func..) in the final tree); moving the cast into an adapter, as the fourth flip's step did, would add an object and a call |
| 467, 468 | implsplit | 23 + 5 methods split | 14 + 0 | the classes implementing a library functional interface have typed methods |
| 469, 471 | tfnparams | 7 + 6 parameters | 0 + 0 | LogFile.log takes Supplier[string]; NodeUtil's predicates Predicate_base[Node] |
| 477, 486 | deadtfn | 39 + 28 adapter types deleted (140 stay) | 1 + 0 (3 stay) | there are no library adapter types |
| 482 | fntypes | 8 groups, 11 parameters | 0 | open item |

## Census against the fourth flip's final tree (inb-selfgen, 6525013f)

tools/postflip/census.py on both go/closure trees (fourth | fifth); rows that differ:

| row | fourth | fifth | |
|---|---|---|---|
| Java collections (erased): constructors | 120 | 114 | better |
| typed TList/TMap/TSet constructors | 448 | 464 | see the map item below |
| Guava immutable collections / builders | 155 | 113 | better (typed builder chains) |
| anonymous interface assertions .(interface{ | 0 | 6 | WORSE, explained below |
| interface casts icast_X( | 207 | 176 | better |
| class casts to_X( / cast_[T]( | 769 | 740 | better |
| instanceof is_X( | 71 | 78 | WORSE, explained below |
| type assertions x.(T) | 2,118 | 2,110 | better |
| type switches | 27 | 26 | better |
| boxing to any | 1,182 | 1,016 | better |
| unboxing from any | 256 | 235 | better |
| lambda objects &X_fn{func | 45 | 37 | better |
| lambda adapter literals (all adapter values) | 587 | 39 | better |
| throw as panic( | 3,528 | 3,518 | |
| Java equals/hashCode protocol | 77 | 76 | better |
| String_* UTF-16 helpers | 122 | 121 | better |
| threads/executors emulation | 56 | 53 | better |
| fields/params declared any (approx) | 680 | 629 | better |
| ... Object slots only | 526 | 479 | better |

Equal: native slices 100, native maps 0, foreach_ 105, explicit iterators 125, streams 0, self_
dispatch 0, Optional 73, catch_/recover 54, defer 84, StringBuilder 5, enum objects 1 / 1,
Preconditions_ 6, reflect 5, unsafe 1. Generated lines 262,788 -> 261,915; runtime lines (without
tests) 43,105 -> 42,925; runtime Test functions 78 -> 82.

The two rows that are worse have one cause, and it is not rule "function types": the Simple*
collections are generic classes from the translator (prep 19), and the translator writes
`instanceof` of a generic class as a marker-interface test, is_X(v) with
`_, ok := v.(interface{ isa_X_() })`. 4 such helpers (SimpleBiMap, SimpleListMultimap,
SimpleMultiset, SimpleSetMultimap), their 3 calls, and 2 assertions written in place = 6
anonymous interface assertions and 7 is_X mentions. On the fourth flip those classes were erased
at the flip and made generic by step 257, after the instanceof steps had run. No post-flip tool
names an isa_X_ marker interface (step 558 names `interface{ as_X() *X }` only: 15 of 15 here).
Not fixed.

## Coverage that the census does not count: typed variants of functional APIs (OPEN)

The fourth flip's later steps gave typed lambda adapters to typed variants of runtime functions.
Those steps read the adapter literal `&X_tfn_T{f}`; on this flip the same lambda is a function
literal at the erased place's type with its conversions on entry, `X[any..](func(p0 any..) {
var v T = K(p0); .. })`, or, for a function value, the translator's converting closure
`func() X[any] { f_ := E; ..; return func(p0 any..) R { return f_(K(p0)..) } }()`. The tools do
not read either form, so these calls stay on the erased API (final trees, fourth | fifth):

| | fourth | fifth | step |
|---|---|---|---|
| Go maps with mapComputeIfAbsentT_ | 20 | 0 | 403 native + 486 tfnapi: on this flip the call is TMap.ComputeIfAbsent from step 402 on (toolfix 29), which native does not take as a map operation, so those maps stay TMaps: .ComputeIfAbsent( 27 -> 53, map[..] type mentions 530 -> 495, typed constructors 448 -> 464 |
| forEachT_ (typed list / map) | 8 | 0 | 486 tfnapi: Map_forEach 3 -> 9, Iterable_forEach 4 -> 7 |
| mergeT_ / computeT_ | 3 / 2 | 0 / 0 | 486 tfnapi: Map_merge 0 -> 3, Map_compute 0 -> 2 |
| typed sorts (sortSliceF_, sortSlicePF_, sortTListF_, sortArrF_) | 23 | 12 | 472, 481, 486 tcmp: List_sort / Collections_sort 6 -> 16. Of the 10, 8 have the typed comparator at hand in the converting closure (the combinators are typed now, which the fourth flip's tcmp had to leave: "a comparator from the runtime's combinators") |
| Compiler_runInCompilerThreadT_ | 6 | 0 | 488 tgeneric: runInCompilerThread 11 -> 17 |
| ComputeIfAbsentObj | 6 | 0 | 425 adapters (second run) |
| fntypes groups | 8 | 0 | 482: "10 groups: used other than by calling it or passing it to the group"; not looked into |

What a fix needs (not made): one reader shared by tfnapi, tcmp, tgeneric and adapters that gives
the typed function of the two forms above (for the literal: parameters typed and the entry
statements `var v T = K(p)` dropped when K is the unboxing of the wanted type, which is
lamparams' analysis; for the converting closure: E when E cannot be null), and native taking
TMap.ComputeIfAbsent as mapComputeIfAbsentT_ (whose runtime function arrives only with patch 485).
Each changes the tree from its step on (425 at the earliest), so it needs a replay from there.

## Other open items

- PERFORMANCE, NOT MEASURED (no timing was run): steps 105, 147 and 419 are dropped. On the fourth
  flip the data-flow work queue's PriorityQueue called an int-key comparator object directly
  (intKeyCmp_ holding nodePriority_). Here a comparison goes through the translator's conversion
  closure (two junboxT_ assertions), the comparingInt closure and two interface calls of
  getPriority. The typed PriorityQueue that would remove this is the collection stage's.
- tconv_.isNull has one more nil test (the isnull hook of reflip5-fnconv-rt.patch) on every null
  test of a typed collection's element. Not measured.
- The _f variants left by 125 (Iterables_filter_f, mapComputeIfAbsent_f, mapComputeIfAbsentObj_f)
  and funcObj_ (252) were kept as the fourth flip had them; whether generated code still uses each
  was not checked one by one (Map_computeIfAbsent_f has 6 calls from step 417).
- The `#=5` notes for the steps of the table "run and find nothing" were added to steps.txt after
  the unattended replay (comment lines only; the replay does not read them).
- gate.sh's header prints GATE FAILED for a post-flip tree (the j2go self-test line), as before.

## Where this stands

(filled in at the end)
