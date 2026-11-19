# Branches for the next re-flip (status 2026-11-19)

## What a real re-flip is built from

    main-r8-prep7  =  main-r8-typed  +  main-r8-upcast  +  prep 17 (#7)
    re-flip base   =  main-r8-prep7  merged with  main-r8-nodestore (2b4a7142)

then `tools/j2go/run.sh`, the re-flip's rename (reflip-trial's
`tools/postflip/reflip/rename.sh`, with its name list regenerated, see below) and the post-flip
replay of reflip-trial's `tools/postflip/reflip/steps.txt` with its patch overlay.

reflip-trial was built from main-r8-typed at **fc2650a1** plus main-r8-nodestore (merge
6b59b4a0). The re-flip base above is newer on the typed side as well: typed slices 7-14
(acc6fe55 .. 4936ec55), all of main-r8-upcast's rules P1-P7, and prep 17.

The merge with main-r8-nodestore has one textual conflict, in
`tools/j2go/src/j2go/ExprEmitter.java` (comparison with null: nodestore's handle / string-id /
runtime-field tests against typed's `isnull_` / `.null_()` tests). Keep both, nodestore's first,
exactly as reflip-trial's merge 6b59b4a0 resolved it. That merge is not gated yet.

## What is gated

Gate = `tools/j2go/run.sh`, then `tools/prune/verify.sh`, under gate.lock: selftest, runtime
tests (no test dropped), checks / bundle / downlevel / advanced byte-identical to Java.

| branch | commit | what | gate |
|---|---|---|---|
| main-r8-typed | 1be86b45 | slice 13: kept type parameters are comparable; type variables as set elements / map keys | PASS (16 tests) |
| main-r8-typed | 4936ec55 | slice 14: Splitter.splitToList makes a typed list | PASS (16 tests; selftest incl. Imm.split) |
| main-r8-typed | 7a89cef7 | note: both gated, no fix needed | (no change) |
| main-r8-upcast | 270cbdea | merge of main-r8-typed 7a89cef7 (gates the WIP merge 3f2a8ac9 below it too) | PASS (18 tests) |
| main-r8-prep7 | ad6e084d | tools/prep/17_find_graph_wildcards (scanner, no Java change) | n/a |
| main-r8-prep7 | ed385861 | DominatorTree: `<N, E> ... DiGraph<N, E>` for `DiGraph<N, ?>` (2 signatures) | gate 0 PASS (447 suites, 30,393 tests, 0 failures; 4 projects identical to stock); Go gate PASS (18 tests) |
| main-r8-nodestore | 2b4a7142 | node store | gated by its own session, not re-gated here |
| prep7 + nodestore | - | the re-flip base | not built, not gated |

Not measured here (timing is the measurement agent's): the typed slices 13-14, the upcast merge,
prep 17's Java speed against stock (`tools/gate0/perf.sh`; every earlier prep step was within
1.5%), and the open question of b0520c58 (the "one Go type" rule measured +7% typed-full cpu
once; a re-measurement never finished).

j2go needed no change for #7: with the two signatures, GenScan reports `TYPED [N, E]` for Graph,
DiGraph, UndiGraph, their nodes and edges, the Linked* graphs, SubGraph, AdjacencyGraph,
GraphColoring, GraphReachability, FixedPointGraphTraversal, CheckPathsBetweenNodes and
LowestCommonAncestorFinder (`J2GO_GENERICS=1`); before it, E was erased (`type DiGraph[N
comparable] struct`).

## Census of the translator output

`tools/postflip/census.py` (reflip-trial's) on the regenerated `go/closure`, gofmt'd (the
"declared any" rows count gofmt's alignment, so every column is gofmt'd; 7a89cef7's message
gives 1854 / 1244 for its last column from unformatted output: 1868 / 1255 formatted). Rows that
change:

| row | typed 2b9b5e94 | 1be86b45 | 4936ec55 | upcast 270cbdea | prep7 |
|---|---:|---:|---:|---:|---:|
| erased collection constructors | 153 | 137 | 136 | 136 | 136 |
| typed TList/TMap/TSet constructors | 1025 | 1042 | 1043 | 1043 | 1043 |
| Guava immutables / builders | 192 | 188 | 187 | 187 | 187 |
| for-each via iterator objects | 1294 | 1286 | 1286 | 1286 | 1286 |
| interface casts icast_X( | 1932 | 1912 | 1910 | 504 | 504 |
| class casts to_X( / cast_[T]( | 2602 | 2593 | 2593 | 2356 | 2342 |
| type assertions x.(T) | 212 | 212 | 212 | 149 | 149 |
| boxing to any | 8982 | 8956 | 8949 | 4406 | 4402 |
| unboxing from any | 602 | 602 | 567 | 472 | 472 |
| Java equals/hashCode protocol | 892 | 892 | 892 | 640 | 640 |
| StringBuilder | 360 | 360 | 360 | 361 | 361 |
| threads/executors/synchronized | 70 | 71 | 71 | 71 | 71 |
| fields/params declared any | 1969 | 1868 | 1868 | 1805 | 1796 |
| ... Object slots only | 1251 | 1255 | 1255 | 1255 | 1246 |

(1969 -> 1868 at slice 13 is mostly the census's own: 107 -> 4 type parameters are written
`any`, the others `comparable`.) Prep 17 adds 19 `[..., any]` instantiations: the program's own
E = Object / Void (ColorGraphBuilder, CoalesceVariableNames, AmbiguateProperties,
DisambiguateProperties).

## Expected conflicts with reflip-trial's post-flip steps

Method: the lines each manual patch of reflip-trial's overlay removes or keeps as context (after
the re-flip's rename), looked up in the gofmt'd translator output before and after a change;
a line that the "before" output has and the "after" output does not is text a rule changed.
Before/after: main-r8-typed 4936ec55 -> upcast 270cbdea (rules P1-P7), and upcast -> prep7 (#7).
Runtime patches were checked the same way against the runtime files.

**Upcast rules P1-P7, manual patches** (5 of 28 touch changed text; small, re-made by hand or
dropped):

| patch | file | changed lines | rule |
|---|---|---:|---|
| 018-d12f878c | g_AbstractScope.go | 4 of 37 | P1/P5 (`icast_Map(obj_(ImmutableMap_of()))` is `ImmutableMap_as_Map(..)`), P4 (`checkState_..._H` with the Node message argument unboxed) |
| 040-601ba6be | g_NodeUtil.go | 1 of 16 | P6 (`objstr_(n.box_())` in a concatenation is `tostrH_(n)`) |
| 121-0febb8b5 | g_SimpleListMultimap.go, g_SimpleSetMultimap.go | 2 + 1 | P3 (`strobj_("immutable")` passed bare) |
| 418-43d498fa | g_MakeDeclaredNamesUnique.go | 1 of 37 | P1 (`icast_..._VariableInfo(obj_(..))` is `..._as_..._VariableInfo(..)`) |
| 423-a97d418c | g_Compiler.go | 1 of 50 | P3 (`obj_(new_InputId(..))` as a map key) |

**Upcast rules, runtime: name clashes (the build breaks, not a patch conflict).** j2go's runtime
now declares names that post-flip steps also declare in their runtime patches:
`tostr_` (P6, rt_conv.go; post-flip tostr step 176 / 261, rt_core.go), `csStr_`,
`String_contains_s`, `String_replace_s`, `String_join_s` (P7, rt_conv.go; post-flip charseq
step 120, rt_charseq.go). Either rename-names.txt is regenerated to include them (rename.sh
prefixes j2go's), or, better, the steps are dropped as "done by j2go", as step 7 (Optional) was.
No runtime patch of the overlay edits text the upcast merge changed.

**Upcast rules, post-flip tools that meet new forms** (no textual conflict; they find fewer or
different sites, and their counts in steps.txt change):
- P2 emits `Preconditions_checkNotNull_T` / `Objects_requireNonNull_T` (generic, no cast back);
  castfold (161, 163, 200, 455) and nnchecks (205, 239, 242) know the erased forms and their own
  `Preconditions_checkNotNullT`: the 147 + 76 + 73 sites they converted are mostly done by j2go,
  and nnchecks would need the `_T` names to inline them as Go checks.
- P4 emits `Preconditions_check..._H` message forms; precond (78) and checks (182: 2,497 of
  2,501 inline Go checks) must accept them.
- P1 (static upcast, 1910 -> 504 icast_) and P5 (one Go type: `ImmutableCollection_as_List(..)`)
  leave casts, castfold, anyconv, objslots, instof, covret and immbuild / immnames (104, 326)
  fewer sites; the new `X_as_Y` conversions are forms no post-flip tool knows.
- P3 (bare arguments, boxing 8949 -> 4406) and P6 (typed Object methods): anyconv / objslots
  (115, 178, 201, 260), tostr, eqtyped (250), deadeq (438), valcls (450), sbuild.
- P7 (CharSequence variants): charseq (120), strnative / strview / strunits.

**Prep 17 (#7), manual patches** (6 of 28, all graph code; these patches did by hand on erased
E what j2go now does, so they are expected to be dropped or re-made, not merged):

| patch | file | changed lines |
|---|---|---:|
| 406-3da0ef84 | g_MustBeReachingVariableDef.go | 1 of 54 (`GraphNode[Node]` is `GraphNode[Node, *ControlFlowGraph_Branch]`) |
| 408-42cfbc2c | g_LinkedDirectedGraph.go | 4 of 20 (`edgeValue_ any` / `junboxT_[E]`) |
| 409-dc3eb3ea | g_ControlFlowAnalysis.go, g_DataFlowAnalysis.go | 1 + 1 |
| 419-88f8ba50 | g_ControlFlowAnalysis.go | 3 of 6 |
| 420-ae56a16d | g_ControlFlowAnalysis.go | 3 of 29 |
| 422-e964d92f | g_LinkedDirectedGraph.go, g_DataFlowAnalysis.go, g_ControlFlowAnalysis.go, g_DeadAssignmentsElimination.go, g_MustBeReachingVariableDef.go | 13 + 8 + 1 + 1 + 1 |

Toolfix 2 of reflip-trial (colls leaves element types that mention a type parameter) and
REFLIP.md's handover item 2 (#4, generic tconv) meet the graph collections again with E kept.

The typed slices since fc2650a1 (reflip-trial's base) change more of the generated text than
P1-P7 do; this file does not list those.
