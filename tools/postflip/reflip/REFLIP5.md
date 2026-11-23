# The fifth flip (2026-11-21): the in-bounds list on main's translator with rule "function types"

STATUS: the list replays unattended from the base to a gated tree (four MATCH). The open items of
the first part (typed variants of functional APIs, the data-flow queue's comparator, two census
rows) are closed in "Second part" at the end, which has the final replay, gate and census; the
sections between describe the first part as it stood (tip aa141ba3).

What this is: main at 024a9c5f flipped (the translator with rule "function types" on, the Java
tree through prep 20) and the in-bounds step list of the fourth flip (steps.txt of
main-r11-flip4-inbounds, 02da66e7, through step 558) replayed on it.

    overlay branch   main-r12-flip5        worktree $B/wt/flip5
    replay worktrees $B/wt/replay5c        (the replay of the final list, per-step commits, detached)
                     $B/wt/replay5b        (the second unattended replay of it)
                     $B/wt/replay5         (the first pass, made with stops and without toolfix 32: not a result)
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
| 336 | tool + rt | adapted | toolfix 28 and reflip5-fnconv-rt.patch: a function type as an element of a typed collection (Node.PROP_MAP_FOR_EQUALITY); toolfix 32: an upcast helper's parameter is no slot (see "The first complete pass") |
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
| 556 | rt | regenerated | one conflict in rt_tsorted.go (the line 402 changed) |
| 553 | tool | adapted | toolfix 31 (optstr's call graph: forwarder methods of function types, generic literals per instantiation) |

## Tool fixes added (toolfix.sh 28-32; each has its reasoning in the script)

- 28 rw.ConvFor, colls, native: a function type as an element (box_(), fnUnbox_[F], isnull).
- 29 unify: the translator's typed computeIfAbsent is ComputeIfAbsent.
- 30 adapters: a function literal converted to a function type is the typed lambda.
- 31 optstr (cha.go): x.m(..) of a function type's forwarder method is the call x(..) (through
  the field's known stores when x is such a field); the method value x.m is x; a literal of
  generic code is reached per instantiation. Only for the tool version of step 553.
- 32 colls: the translator's upcast helpers X_as_Y (g_0casts.go) have fixed signatures.

The list: 270 commits replayed (fourth flip: 276), 163 with tool runs, 117 runtime patches (121),
1 manual patch (3). 41 `#=5` notes in steps.txt.

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

tools/postflip/census.py on both go/closure trees (fourth | fifth: the gated tree of replay5c,
go/ 6b6d5eac; the two outputs are $B/runs/census_flip4_selfgen_final.txt and
$B/runs/census_flip5_final.txt); rows that differ:

| row | fourth | fifth | |
|---|---|---|---|
| Java collections (erased): constructors | 120 | 114 | better |
| typed TList/TMap/TSet constructors | 448 | 464 | see the map item below |
| Guava immutable collections / builders | 155 | 149 | better (typed builder chains) |
| anonymous interface assertions .(interface{ | 0 | 6 | WORSE, explained below |
| interface casts icast_X( | 207 | 176 | better |
| class casts to_X( / cast_[T]( | 769 | 728 | better |
| instanceof is_X( | 71 | 78 | WORSE, explained below |
| type assertions x.(T) | 2,118 | 2,110 | better |
| type switches | 27 | 26 | better |
| boxing to any | 1,182 | 1,017 | better |
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
- The `#=5` notes for the steps of the table "run and find nothing", and the note of toolfix 32
  at step 336, were added to steps.txt after the replay of replay5c (comment lines only); the
  second replay (replay5b) ran with them.
- $B/gocache was full (47 MB free on /mnt/data) when this work began: files of the shared Go build
  cache not used for 12 hours (later 8 to 10) were deleted three times to go on.
- gate.sh's header prints GATE FAILED for a post-flip tree (the j2go self-test line), as before.

## The first complete pass and its gate: FAILED, one tool hole (toolfix 32)

The list first replayed to the end in $B/wt/replay5 (270 commits, tip 1d20bbf3, go/ tree c6ec0c7e;
tag flip5-pass1 marks an earlier state of that worktree). Its gate ($B/gate.sh tree5a flip5a,
log $B/logs/gate-flip5a.log): runtime tests ok, but checks, bundle, downlevel and advanced all
DIFF: the compiler failed at start with "interface conversion: interface {} is string, not
*closure.CompilerInput" in CommandLineConfig's initializer. Cause: step 336 (colls, immlist kind)
made the parameter of the translator's upcast helper ImmutableList_as_List a slot and typed the
class of everything passed through it as lists of *CompilerInput (35 constructors, lists of
strings and Nodes among them); the type check of the replay cannot see it (cast_ of an `any`).
toolfix 32 fixes the tool (helper signatures are fixed); with it the immlist kind reports what
the fourth flip's did (6 constructors in typed classes, 3 declarations typed, 1 class). The fix
applies to every colls run from step 3 on, so the whole list is replayed again.

The census and the counts of "coverage that the census does not count" above were taken again on
the tree of the replay with toolfix 32 (against the first pass: Guava immutables 113 -> 149, the 35
constructors that were wrongly typed; class casts 740 -> 728; boxing 1,016 -> 1,017; the open-item
counts are the same).

## Where this stands

Done and seen:

- The final list (steps.txt with 41 `#=5` notes, toolfix.sh 7, 9-32, the overlay patches) replayed
  from the base 4b50a363 in one unattended run, twice, each in a worktree reset to the base:
  `bash tools/postflip/reflip/replay5.sh replay5c`: `REPLAY DONE`, 270 commits, tip aa141ba3,
  go/ tree 6b6d5eac6a137659576d95791d7bdb70843a0f25 (no patch regenerated on the way);
  `bash tools/postflip/reflip/replay5.sh replay5b` (after the comment notes were added): `REPLAY
  DONE`, 270 commits, tip 2860300a (tag flip5-replay5b), the same go/ tree 6b6d5eac.
- The gate of that tree: branch flip5-tree (worktree $B/wt/tree5, at aa141ba3),
  `$B/gate.sh tree5 flip5`, log $B/logs/gate-flip5.log:

      GATE FAILED flip5        (the header: the j2go self-test line, as for every post-flip tree)
      tests before: 82
      lstat closure/g_*.go: no such file or directory
      removed 0 declarations in 0 rounds
      ok  	jsgomech/closure	3.649s
      checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
      bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
      downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
      advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
      tests after: 82

Not done:

- The typed variants of functional APIs (the OPEN table): 20 Go maps, 13 forEachT_ / mergeT_ /
  computeT_ calls, 11 typed sorts, 6 runInCompilerThreadT_, 6 ComputeIfAbsentObj, fntypes' 8
  groups. The tools were not adapted; what an adaptation needs is written under the table.
- No time or allocation was measured. Two things may cost time against the fourth flip and were
  not looked at with a profile: the data-flow queue's comparator path (105, 147, 419 dropped) and
  the erased API calls of the OPEN table.
- The per-step census of both histories (census.py at every replay commit) was made for the
  fourth flip only, and not used: the comparison here is of the final trees, the tools' reports
  per step (the two .replay.log files) and git grep counts at the steps named.
- Nothing on main was changed. REFLIP4.md's first paragraph (which prep steps its base has) is as
  it was.

## Second part (2026-11-21): the open items closed

This section supersedes "Coverage that the census does not count (OPEN)", the two WORSE census
rows, and the first two "Other open items" above; those are left as they were written. What was
built is in toolfix.sh 33-36 (each with its rule and proof obligation), two new steps (559, 560),
two runtime patches (reflip5-ciat-rt.patch, reflip5-pqt-rt.patch) and two regenerated ones (125,
485). Final state:

    bash tools/postflip/reflip/replay5.sh replay5d   (a fresh worktree, one unattended run from the base 4b50a363)
    REPLAY DONE: 272 commits (270 + steps 559, 560), no patch regenerated on the way,
    tip 2abbf4e4, go/ tree 4e3d4ae6b2cd5eaa8bdae5fc5b3de436c2194746   (worktree $B/wt/replay5d)
    tree worktree $B/wt/tree5d (branch flip5-tree-d); $B/gate.sh tree5d flip5d, log $B/logs/gate-flip5d.log:

      GATE FAILED flip5d       (the header: the j2go self-test line, as for every post-flip tree)
      tests before: 83
      lstat closure/g_*.go: no such file or directory
      removed 0 declarations in 0 rounds
      ok  	jsgomech/closure	2.876s
      checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
      bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
      downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
      advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
      tests after: 83

Before it, the steps from 403 on were replayed in a scratch worktree on the first part's commits
and that tree was gated too (four MATCH, log $B/logs/gate-flip5s.log); the final tree differs
from it by patch 125's dead function alone. No time or allocation was measured.

### 1. Typed variants of functional APIs (toolfix 33, 34)

One reader, rw.TypedFn (rw/typedfn.go, written by toolfix 33 into the tool versions from step 472
on), gives tfnapi, tcmp and tgeneric the typed function of an argument at an erased functional
place, in the three forms: the adapter literal `&X_tfn_T{f}` (as the tools read it before, so
the tools work on both translations); the translator's literal at the erased type, which is
retyped (a parameter `p any` whose every use is the operand of the unboxing K(p) of the wanted
type T takes T; `var v T = K(p_)` at the head of the body makes the parameter v; a result is
typed when every return gives the boxing of a value of the wanted type, nil for a pointer or
interface, or a parameterless literal's call whose returns qualify); and the converting closure
`func() X[any] { f_ := E; ..; return func(p0 any) R { return f_(K(p0)) } }()`, which is E when
E cannot be null (a literal, a function or method name, or a call of a function all of whose
returns are such: rt_fn.go's Comparator combinators). Proof obligation: K(box(x)) == x and
unbox(B(e)) == e for the type's own conversion; K and B qualify when they are the unbox and box
functions of the package's `tconv_[T]{box, unbox}` literals for that type (all of them must
agree), which is what every typed collection of T is made with, or the unboxing rw.ConvFor
names. For tgeneric, with a plain cast_[T] around the call, a return may give the T itself.

native (toolfix 34): `m.ComputeIfAbsent(k, f)` of a typed map, which is what the translator's
typed computeIfAbsent is from step 402 on, is the map operation `mapComputeIfAbsentT_(vc, m, k,
f)` (the same function of the map's contents, independent of order). The runtime function is
needed at step 403, so it is the new patch reflip5-ciat-rt.patch there and patch 485 is
regenerated without it. The tool applies the rule only when the runtime has the function.

Counts in the final trees (git grep over g_*.go; fourth flip | fifth flip before | now):

| | fourth | fifth before | now | |
|---|---|---|---|---|
| mapComputeIfAbsentT_ (Go maps given a computeIfAbsent) | 20 | 0 | 20 | native at step 403: 142 maps become Go maps (91 classes), as the fourth flip (121, 77 before) |
| forEachT_ | 8 | 0 | 8 | tfnapi at 486: 2 lists, 6 maps |
| mergeT_ / computeT_ | 3 / 2 | 0 / 0 | 3 / 2 | tfnapi at 486 |
| typed sorts (sortSliceF_ 9, sortSlicePF_ 2, sortTListF_ 8, sortArrF_ 5) | 23 | 12 | 24 | tcmp at 472 (6) and 481 (8 natural, 5 arrays, 5 chains). One more than the fourth flip (sortTListF_ 7 -> 8). Left: 5 comparators that are no literal and no never-null chain (3 are package variables; the fourth flip left 3 package variables, 1 combinator over Objects and 1 other), 2 natural sorts of Objects |
| Compiler_runInCompilerThreadT_ | 6 | 0 | 6 | tgeneric at 488 |
| ComputeIfAbsentObj | 6 | 0 | 0 | the translator emits the final form: the six lambdas are typed func(K) V literals given to ComputeIfAbsent, with no boxed result (ColorSerializer, LinkedDirectedGraph, LinkedUndirectedGraph, ModuleMapCreator, OptimizeCalls, PreprocessorSymbolTable) |
| .ComputeIfAbsent( | 27 | 53 | 33 | = 27 + the 6 above |
| fntypes parameters | 11 | 0 | 0 | 10 of the 11 have their function type from the translator (steps.txt, note at 482); 1 NOT recovered: CmdLineParser_Option.flag's setter is Consumer[any] (fourth flip: func(bool)). fntypes reads parameters of plain func types only; not extended |
| erased calls left: Map_forEach / Iterable_forEach / Map_merge / Map_compute | 3 / 4 / 0 / 0 | 9 / 7 / 3 / 2 | 3 / 5 / 0 / 0 | Iterable_forEach 5: tfnapi's own "an erased collection" (one is ImmutableList_as_Iterable(vars.asImm()), a helper form the tool does not read) |
| erased calls left: List_sort / runInCompilerThread | 4 / 13 | 12 / 19 | 4 / 13 | |

fntypes' "10 groups used other than by calling it or passing it to the group" are the parameter
f of the translator runtime's constructors X_of_(f) (rt_fn.go), no program parameter.

### 2. The data-flow work queue's comparator (toolfix 35, step 559)

What the two trees do per comparison of the heap (read from the code; nothing was timed). Fourth
flip: PriorityQueue.siftUp/siftDown see an *intKeyCmp_ and call its compare on the concrete
type, which calls the key function nodePriority_ twice (a type switch to the one node class and
getPriority on it). Fifth flip before: the heap calls the erased comparator, which is the
translator's converting closure `func(p0, p1 any) int32 { return f_(junboxT_[T](p0),
junboxT_[T](p1)) }` (two generic assertions to an interface type), then the comparingInt closure
(one more for reversed), which calls the method expression (DiGraphNode).getPriority twice
through the interface. So it was worse: about four more calls and two interface assertions per
comparison.

Recovered with a rule, not a patch of generated code (tcmp, toolfix 35; step 559):

    new_PriorityQueue(func() Comparator[any] { f_ := E; if f_ == nil { return nil }
        return func(p0 any, p1 any) int32 { return f_(K(p0), K(p1)) } }())
      ->  newPriorityQueueT_(E, K)

2 queues (DataFlowAnalysis_UniqueQueue with junboxT_[T]; ControlFlowAnalysis's worklist with
icast_). The runtime function (rt_pqt.go, reflip5-pqt-rt.patch) makes the same PriorityQueue
with the same erased comparator, and keeps K(x) beside each element once computed; the heap
moves compare those with E. Proof obligation: K is a pure cast (the same value or the same
exception for the same interface value, no effect; a list of names: cast_, junboxT_, str_,
to_X, icast_X, gcast_X, X_of_, with signature func(any) T, the same for both arguments), so a
repeated K(x) can be skipped; K(x) is first evaluated at the same call as before, and E is
called with the same pairs in the same order (TestPriorityQueueTyped compares both queues over
random operations, an element of the wrong type included); a kept value is used only for the
identical interface value, so nothing inherited from ArrayList can make it stale; a null E
gives the natural-order queue as the null closure did. A comparison is now the comparingInt
closure and two getPriority calls through the method expression. The fourth flip's direct call
on the concrete node class (manual 419's nodePriority_) is not reproduced: that is a
devirtualisation of a method expression, which no rule here does.

### 3. The two census rows (toolfix 36, step 560)

isanames (a new tool, written by toolfix 36) names the marker interface `interface{ isa_X_() }`
of an instanceof test of a generic class X_isa (declared where X's marker method is) and deletes
an is_X helper of a marker that nothing calls; tswitch reads an else-if link that is a
disjunction of instanceof tests of the chain's value as a case with several types. Step 560: 6
anonymous marker interfaces named (4 classes), LogsGson.write's chain becomes one type switch
(`case SimpleListMultimap_isa, SimpleSetMultimap_isa, SimpleBiMap_isa:`), the 4 helpers go.
vdispatch -rest (558) was not extended: it names the `as_X` interfaces of casts, and these are
instanceof tests.

### 4. The previous record's unverified items

- Iterables_filter_f (step 125): dead on this flip (no g_*.go at any commit of the replay, no
  other runtime, no tool source names it). Patch 125 regenerated without it.
- mapComputeIfAbsent_f, mapComputeIfAbsentObj_f (125): no generated code calls them at any
  commit either, but native, funcparams and colls write them for a computeIfAbsent_f call, and
  rt_nulllist_test.go tests the first: kept. Map_computeIfAbsent_f: 6 calls. updateAndGet_f: 1.
- funcObj_ (252): 12 users (the func-object aliases of the program's own functional interfaces).

### 5. The translator's next stage (typed collection interfaces)

Nothing done for it. What is robust to both forms: rw.TypedFn answers for adapter literals and
for the function-type forms; native's ComputeIfAbsent rule and tcmp's PriorityQueue rule apply
only when the runtime has their functions; isanames and the tswitch link find nothing when the
translator writes no marker tests.

### Census: fourth flip (inb-selfgen 6525013f) | fifth flip before (aa141ba3) | now (2abbf4e4)

tools/postflip/census.py; outputs $B/runs/census_flip4_selfgen_final.txt, census_flip5_final.txt,
census_flip5_part2_final.txt. Rows that differ between any two:

| row | fourth | before | now | against the fourth |
|---|---|---|---|---|
| Java collections (erased): constructors | 120 | 114 | 112 | better |
| typed TList/TMap/TSet constructors | 448 | 464 | 450 | newTMap_ 174 / 191 / 177, newTList_ 126 / 125 / 125, newTSet_ 135 all; map[ type mentions 530 / 495 / 530. Which sites make the 2 more than the fourth flip was not looked at |
| Guava immutable collections / builders | 155 | 149 | 149 | better |
| anonymous interface assertions .(interface{ | 0 | 6 | 0 | equal |
| interface casts icast_X( | 207 | 176 | 164 | better |
| class casts to_X( / cast_[T]( | 769 | 728 | 699 | better |
| instanceof is_X( | 71 | 78 | 71 | equal |
| type assertions x.(T) | 2,118 | 2,110 | 2,110 | better |
| type switches | 27 | 26 | 27 | equal |
| boxing to any | 1,182 | 1,017 | 999 | better |
| unboxing from any | 256 | 235 | 224 | better |
| lambda objects &X_fn{func | 45 | 37 | 37 | better |
| lambda adapter literals (all adapter values) | 587 | 39 | 39 | better |
| throw as panic( | 3,528 | 3,518 | 3,518 | |
| Java equals/hashCode protocol | 77 | 76 | 76 | better |
| String_* UTF-16 helpers | 122 | 121 | 121 | better |
| threads/executors emulation | 56 | 53 | 53 | better |
| fields/params declared any (approx) | 680 | 629 | 571 | better |
| ... Object slots only | 526 | 479 | 425 | better |

Equal in all three: native slices 100, native maps made 0, foreach_ 105, explicit iterators 125,
streams 0, self_ dispatch 0, Optional 73, catch_/recover 54, defer 84, StringBuilder 5, enum
objects 1 / 1, Preconditions_ 6, reflect 5, unsafe 1 (the census counts g_*.go; rt_pqt.go uses
unsafe for the identity of two interface values). Generated lines 262,788 / 261,915 / 261,791;
runtime lines without tests 43,105 / 42,925 / 43,064; runtime Test functions 78 / 82 / 83.

### Not done, not verified

- No time or allocation was measured, for the queue (item 2) or anything else.
- fntypes: the CmdLineParser flag setter (1 parameter) stays Consumer[any].
- The fourth flip's nodePriority_ (getPriority on the concrete node class) has no counterpart.
- tfnapi does not read a list given through the translator's upcast helper
  (ImmutableList_as_Iterable(l.asImm())): 1 forEach.
- The two typed constructors more than the fourth flip were not traced to their sites.
- Worktrees: $B/wt/tree5a (the first pass's failed tree) and its cache were removed; replay5
  (its log is cited above) and replay5x (uncommitted experiments of the first part) were left.
