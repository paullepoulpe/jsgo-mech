# Java-isms left in the sixth flip's tree (tree6h, 88e20f2d), re-derived

Tree: `/mnt/data/ubuntu/jm/wt/tree6h` (branch flip6-tree-h, 88e20f2d, go/closure tree 03874c98),
`go/closure/g_*.go`: 824 files, 262,105 lines; runtime `rt_*.go` without tests: 66 files, 43,980 lines.
Reference: jsgo at 71ff08f4 (`/home/ubuntu/work/jsgo`, `internal/`), the same commit as JAVAISMS4.md.
Model: JAVAISMS4.md of main-r11-flip4-inbounds (the fourth flip's tree agQr), same method.

Nothing under `/mnt/data/ubuntu/jm/wt` was modified, built or gated. Every script ran on copies:
`tree/` (read-only copy of tree6h's `go/`) and `work/closure` (the copy the tools ran on; restored from
tree6h after every tool run and compared with tree6h after each, 0 differences at the end). The heavy
lock was not taken; nothing heavier than loading the package with go/types (~6 s, 0.9 GB) ran.

All paths below are under
`docs/javaisms6/` (originally written in the session scratchpad; the tree copy, binaries and Go cache are not kept)
unless absolute.

## 0. What the answer is

1. **The census is the one of REFLIP6.md and gives the same numbers.** `tools/postflip/census.py` is
   byte-identical in flip6 and tree6h; on tree6h it prints exactly REFLIP6.md's "sixth (replay6
   c62309bd)" column (step 561 changed no census row). Its explanations of the rows that rose against
   the fifth flip (icast +19 in g_0casts.go's unbox functions, the 4 typed sorts of entries, Optional +2,
   panic +5, any +24) are kept below; the 4 typed sorts of entries are counted REASON with REFLIP6.md's
   own reason (a rule was written and dropped), not as RULE.
2. **R1 (erased library generics) is mostly gone; what is left is the edge of the typed world.** The
   rule "typed library interfaces" made List/Set/Map/Collection/Iterable/Iterator/Map.Entry typed
   (`ListT[T]` = erased `List` + `_t` methods), and "function types" made the 12 functional interfaces
   Go func types (`type Function[T, R] func(T) R`). Aliases 52 -> 34; erased collection aliases in
   declarations 726 -> 31; adapter literals 587 -> 39 (typed `_tfn_` adapters 542 -> 0). The census
   sites that JAVAISMS4.md gave to R1 + R2 (2,351 by its own tables) are 752 here, under seven
   narrower rules T1-T7 (section 2). What replaced R1 is (a) the runtime's static library functions,
   which are still erased (`Iterables_getFirst`, `Map_get_s`, `Map_forEach`, `List_sort`,
   `newTSortedSetC_`'s comparator, ...): 283 sites, rule T1; (b) the 34 library types the new rule did
   not reach (Guava immutables and their Erased* twins, Guava Optional, ImmutableSorted*, Deque,
   EnumSet/EnumMap, TreeMap, LoadingCache, PeekingIterator, Ordering, ...): 228 sites, rule T2; and
   (c) the conversions at the boundary between the two worlds, which the census does not count: 743
   calls of `X_un_` (366), `xAs_` wrappers (268), `lgBox_` (82) and `elemsAs_` (27).
3. **Most sites are what jsgo also writes:** 6,725 of 9,253 matches (panics, recover, defer,
   dispatchers, comma-ok instanceof, `Equals(other any)`, property bags, UTF-16 helpers, `make([]T)`).
4. **The tree's own tools are at a fixed point except four small items** (section 4): `tostr` (4 boxes),
   `nnchecks` (2 Preconditions), `casts -named` (8 dead helper declarations, not sites), `ranges -index`
   (5 reverse cursors, not a census row), `etaconv` (40 descriptor literals, not sites).
5. **Where jsgo is different and no rule is offered** it rewrote by hand: flow states in a side slice
   instead of graph annotations, concrete graph types instead of `? extends` edges, index loops instead
   of iterator walks, `(T, bool)` results instead of nullable boxed numbers, goroutine pools instead of
   ExecutorService. Those are REASON and a stated difference, not equivalence.

### Summary table

Command: `cd tree/go/closure && python3 ../../../tools_postflip/census.py` (output `census.out`), then
the pipeline of section 1; table printed by `python3 summary.py` (`summary.out`). Columns: DONE = the
remaining form is what jsgo has too; RULE = a general rule or an existing tool would remove it (named in
the last column, defined in section 5); REASON = stays, with a reason that holds against jsgo (sections
6-7 give it with the jsgo file:line); "not a site" = a helper declaration, a function used as a value,
a descriptor's own function, a comment, a string literal or a name match; "not split" = I could not
assign the sites to a verdict by script. Every row adds up to its count (`summary.py` checks it and
prints `all rows add up: True`).

| row | count | DONE | RULE | REASON | not a site | not split | RULE by rule |
|---|---:|---:|---:|---:|---:|---:|---|
| erased Java collection constructors | 40 | 22 | 9 |  |  | 9 | T2 9 |
| typed TList/TMap/TSet constructors | 525 |  |  |  |  | 525 |  |
| Guava immutables / builders | 43 |  | 43 |  |  |  | T2 43 |
| native Go slices made | 100 | 100 |  |  |  |  |  |
| native Go maps made | 0 |  |  |  |  |  |  |
| for-each via iterator objects | 34 |  | 22 | 3 |  | 9 | T5 11, T6 7, T1 3, T2 1 |
| explicit Iterator use | 73 |  |  | 73 |  |  |  |
| streams | 0 |  |  |  |  |  |  |
| dispatch through self_.(interface) | 0 |  |  |  |  |  |  |
| anonymous interface assertions | 0 |  |  |  |  |  |  |
| interface casts `icast_X(` | 183 | 65 | 16 | 17 | 85 |  | T2 10, T1 6 |
| class casts `to_X(` / `cast_[` | 553 | 95 | 125 | 27 | 306 |  | T1 60, T2 33, T4 18, R4 7, T5 4, T6 2, T7 1 |
| instanceof `is_X(` | 71 | 54 |  |  | 17 |  |  |
| type assertions `x.(T)` | 2,047 | 1,897 | 15 | 4 | 131 |  | T1 7, T2 6, T5 2 |
| type switches | 27 | 4 |  |  | 23 |  |  |
| boxing to any (explicit) | 938 | 417 | 297 | 211 | 13 |  | T1 92, T2 71, T3 62, T6 19, T7 17, R4 16, T4 11, R5 5, E-tostr 4 |
| unboxing from any | 131 | 6 | 117 | 7 | 1 |  | T1 49, T2 20, T7 16, R4 13, T5 9, T4 5, T6 5 |
| lambda objects `&X_fn{func` (subset of next row) | 37 | 23 | 9 | 1 |  | 4 | F1 9 |
| lambda adapter literals | 39 | 25 | 9 | 1 |  | 4 | F1 9 |
| Optional | 75 | 31 | 34 |  | 10 |  | T2 34 |
| try/catch emulation (recover, catch_) | 54 | 54 |  |  |  |  |  |
| panic( | 3,523 | 3,523 |  |  |  |  |  |
| defer | 84 | 84 |  |  |  |  |  |
| equals/hashCode protocol | 72 | 54 | 18 |  |  |  | R7 18 |
| String_* UTF-16 helpers | 121 | 121 |  |  |  |  |  |
| StringBuilder | 5 |  | 2 |  | 3 |  | R7 2 |
| enum objects: struct | 1 |  | 1 |  |  |  | R7 1 |
| enum objects: call | 1 |  | 1 |  |  |  | R7 1 |
| Preconditions_ | 6 | 4 | 2 |  |  |  | E-nnchecks 2 |
| threads/executors | 53 | 28 |  | 12 | 13 |  |  |
| reflect | 5 |  |  |  | 5 |  |  |
| unsafe | 1 |  |  |  | 1 |  |  |
| fields/params declared `any` (Object slots only) | 448 | 141 | 175 | 70 | 62 |  | T1 66, R5 51, T4 29, T7 23, R4 5, T2 1 |


Totals over all rows (row 17 is a subset of row 18 and is not added): 9,253 matches; DONE 6,725, RULE
886, REASON 425, not a site 670, not split 547.

### RULE candidates, ranked by sites

A rule's count is the sum of the census matches given to it in the row tables (section 7); one Java
construct can show in two rows (a converting closure's `p0 any` is an Object slot and its `cast_[T](p0)`
a class cast), as in JAVAISMS4.md. "Hot files" = matches in the files of JAVAISMS4.md's list
(TypeInference, TypeCheck, TypedScopeCreator, NodeTraversal, ControlFlowAnalysis, DataFlowAnalysis,
Node and the JSType classes; `jclass`'s flag H), a statement about position, not a measurement.

| # | rule (one line) | sites | in hot files | kind of change |
|---|---|---:|---:|---|
| T1 | **Typed runtime library functions**: give the runtime's static Java-library functions their Java generic signatures as Go generics with func-typed parameters (`Iterables.getFirst/get/getOnlyElement/getLast/filter/transform/indexOf`, `Lists.transform/reverse`, `Map.forEach/computeIfAbsent/getOrDefault/replaceAll` as `Map_*_s`, `Iterable.forEach`, `List.sort`, `Collections.sort`, `MoreObjects.firstNonNull`, `Comparator.comparing`, `SequencedCollection.getFirst/getLast/removeLast`, `Splitter.splitToList(String)`, `BigInteger.compareTo(BigInteger)`, `ComparisonChain.compare`, the comparator of `newTSortedSetC_`), and have the translator call them with javac's type arguments when the argument is a typed interface | 283 | 14 | runtime functions + translator name list; the same mechanism as the existing `mapComputeIfAbsentT_`, `forEachT_`, `sortSliceF_` |
| T2 | **Typed library interfaces for the 34 types the rule did not reach**: the remaining `XOf[...] = X` aliases (Guava immutables and the Erased* twins, ImmutableSorted*, Guava `Optional_base`, Deque/Queue/PriorityQueue, EnumSet/EnumMap, TreeMap, IdentityHashMap, LoadingCache/CacheBuilder, PeekingIterator, Ordering, ThreadLocal, ConcurrentMap) get the typed twin `XT[...]` (erased + `_t` methods) the rule already gives List/Map/Set | 228 | 11 | translator rule (extend LibGen's list) + runtime twins; typed Guava immutables already exist (post-flip 327) |
| T4 | **Typed Comparable and bridges of classes implementing library interfaces**: `ComparableT[T]` with `compareTo_t`, the runtime's sorts and sorted collections call it; then the existing `deadbridge` removes the erased `compareTo(any)`/`compare(any, any)` bridges and their casts; same for classes implementing Iterator<T>/Collection/CacheLoader/Supplier | 63 | 2 | translator rule + runtime + existing tool |
| T3 | **Object-parameter collection methods typed when the argument's static type is the element type**: `Map.get/containsKey/remove`, `Collection.contains/remove`, `List.indexOf` (Java declares them `Object`) call the typed method when javac's static type of the argument is K/E | 62 | 8 | translator rule (typed interfaces: `get_t` etc. taking K) |
| T7 | **Func types as type arguments**: a typed container or conversion whose element is a func type (`ImmutableMap<Prop, Function<Node, Object>>`, `Supplier<Node>` held as `Supplier[any]`) gets a descriptor for that func type instead of `tc_Function_any__any_` and its converting closures | 57 | 36 (all in `Node_clinit`, a one-time initializer) | translator rule |
| R5 | **Tri-state Boolean/Integer typed as an optional of that type** (as JAVAISMS4.md): 7 fields, their setters and the 38 command-line option closures `Consumer[any](func(v any) {...})` | 56 | 2 | translator rule (boxed primitive -> optional) |
| R4 | **Generic program classes whose type parameter is still erased**: `SourcePosition<T>` (getItem/setItem/item), `AbstractCommandLineRunner.FlagEntry<T>` (flag reads, constructor, addMultiFlagValue); jsgo has both generic (`rhino/static_source_file.go:189 SourcePosition[T comparable]`, `cli/command_line_config_specs.go:184 AbstractCommandLineRunner_FlagEntry[T ...]`) | 41 | 0 | translator rule (as for the other generic program classes) |
| T6 | **Typed for-each over a typed value boxed to Object**: `foreach_(obj_(x))` / `elemsAs_(obj_(x))` where x is a program class implementing `Iterable<T>` (`Node.AncestorIterable`) or an EnumSet: the class gets `iterator_t` and the loop is the typed for-each | 33 | 4 | translator rule |
| T5 | **Typed protobuf list accessors**: protogen emits `getXList() ListT[X]` (and a typed `toBuilder`) instead of erased `List` | 26 | 0 | tools/protogen |
| R7 | As JAVAISMS4.md: `Objects.equals` of two erased views of typed collections (18), `Appendable(new_StringBuilder())` (2), `Node_Prop` as an integer enum (2; `enums` refuses: "the runtime uses Node_Prop_CONSTANT_VAR_FLAGS") | 22 | 2 | existing manual step 303 / a same-pattern runtime edit + `enums` |
| F1 | **Lambda objects where jsgo passes a plain func**: `NodeUtil_Visitor` (7; jsgo `NodeUtil_VisitPreOrder(node, visitor func(rhino.Node))`, `jscomp/node_util_traverse.go:317`) and the deferred `Runnable`s of TypedScopeCreator (2; jsgo `deferAction(key rhino.Node, fn func())`, `jscomp/typed_scope_creator_builder.go:141`); `fnobj` refuses because a class implements the interface too (ChangeVerifier_1, AbstractCommandLineRunner), so the class side needs a method value | 9 | 2 | existing tool extended (funcparams' method-value mode) |
| E | **Existing tools that still apply on this tree**: `tostr` (4 `objstr_(obj_(x))`), `nnchecks` (2 `Preconditions_checkNotNullT`) | 6 | 2 | re-run the tool |

### Not split (547)

- typed constructors `newTMap_`/`newTSet_`/`newTList_` (525): as in JAVAISMS4.md, not split into "could be
  a slice or a Go map" and "must stay ordered". `native -kind list|map|set` converts nothing on this
  tree and reports first obstacles per declaration (map kind: "keys compared by equals" 82,
  `tmapEmptyImm_` results 79, "result of a generic buildOrThrow" 65, `immutableMapCopyOfT_` 45, ...;
  `dry/native_kind_*.out`), not per constructor.
- 9 erased `ArrayList`/`LinkedHashMap`/`LinkedHashSet` constructors wrapped at once by
  `listAs_`/`mapAs_` or copied into an immutable: `colls` converts nothing and its per-declaration
  reasons (`dry/colls_kind_*.out`) were not matched to these sites.
- 9 `.each()` cursors over typed lists and sets (the `native` tool keeps a cursor where the loop body
  may change the collection; not checked per site).
- 4 `Iterable_fn` lambda objects given to `iterableAs_` (HamtPMap 2, NodeUtil, ObjectType); jsgo's form
  not looked up.

### Things that could not be verified

- No rule T1-T7 was written or run. "Would remove" is a statement about the form of each site (which
  erased function or type it passes through), not a tested rewrite. The variance obligation of
  JAVAISMS4.md stays open: I did not count the T1/T2 sites where Java passes `List<Sub>` to
  `Iterable<? extends Super>`; the 3 + 1 sites that visibly need variance (`getInEdges()`/`getOutEdges()`
  of `List<? extends DiGraphEdge>`) are REASON.
- The verdicts are assigned by `classify6.py` from `jclass`'s sub-causes, the enclosing calls (`ctx`)
  and the source line (regexes). I read every site of the cast, unboxing and assertion rows' non-trivial
  sub-causes (295 + 27 lines, `nontriv.txt`) and every "not split" line before giving it a rule; boxing
  and Object slots were assigned by sink and function name, and I read only the groups, not every
  line.
- No benchmark or profile was run (none was to be). Hot-file counts are positions.
- jsgo citations: the 25 reused from JAVAISMS4.md (same jsgo commit) were spot-checked line by line
  (all at the cited line); the new ones (`typed_scope_creator_builder.go:141`, `node_util_traverse.go:317`,
  `js_doc_info.go:259`, `static_source_file.go:189`, `command_line_config_specs.go:184`,
  `ordered_map.go:150,219,224`, `ir_factory.go:181`, `check_conformance.go:975`, `base_transpiler.go:232`,
  `deps_module_loader.go:446`, `check_conformance.go:125`, `type_check_calls_props.go:538`,
  `jstype_helpers.go:640`) were read by me.
- Eight tool invocations of the step list name six tools that exist only at older tool revisions
  (`hashmemo`, `isanames`, `pblast`, `strvar` x3, `trips`, `valplain`; steps with `tools <rev>`) and were
  not re-run.
  `optstr` exits 1 ("not proved, nothing rewritten").
- The runtime (`rt_*.go`) is not accounted site by site: it is the "minimal runtime" that handles the
  Java library. The census regexes over it, for scale (`census_rt.py`, `census_rt.out`): type
  assertions 803, explicit boxing 329, panic 325, Object slots 638, Iterator use 173, String_ 148,
  `unsafe.` 808 (673 of them in the protobuf runtime rt_proto*.go), `reflect.` 79 (rt_generic.go 23).

## 1. Method

| step | script or tool | output |
|---|---|---|
| census | `tools_postflip/census.py` (copy of flip6's; identical to tree6h's) | `census.out` |
| every match with file, offset, line | `matches.py` (JAVAISMS4.md's, regexes read from census.py) | `matches.tsv` (8,842 lines) |
| sub-cause of every match | `tools_postflip/jclass` (JAVAISMS4.md's tool, unchanged, built against flip6's `rw`/`anyflow`) | `sites.tsv` (8,842 lines, same order) |
| enclosing function, enclosing calls, static type of every match | `tools_postflip/ctx` (new, read-only) | `ctx.tsv`, joined as `sc.tsv` |
| boxing sinks | `boxcensusd` = boxcensus with JAVAISMS4.md's site dump (`BOXDUMP`), joined by `boxjoin.py` | `boxsites.tsv`, `box_joined.tsv`, `box_sc.tsv`, `boxjoin.out` |
| Object slots | census.py's two regexes and split, dumped with offsets | `matches_any.tsv`, `anyslots.tsv` |
| a verdict for every match | `classify6.py` (new; one function per row) | `verdicts.tsv` (9,290 lines incl. row 17), `verdict_tables.out` |
| summary, rule totals, row tables | `summary.py`, `rowtables.py` | `summary.out`, section 7 |
| what is left of the erased library generics | `tools_postflip/lgcount` (new, read-only): declared slots by type category, boundary conversions by consumer | `lgcount.out` |
| what no row counts | JAVAISMS4.md's `outside.sh` | `outside.out` |
| the tree's own tools on a copy | `dryruns6.sh`: the 102 distinct `t TOOL ARGS` invocations of reflip/steps.txt plus the 7 `run.sh`-driven tools, each run on `work/closure`, diff recorded, copy restored | `dryruns6.log`, `dry/*.out`, `dry/*.diff` |
| jsgo | flip6's `reflip/jsgocensus.py` | `jsgocensus.out` |

## 2. R1: what remains and what replaced it

What the fourth flip had: 52 aliases `type ListOf[T0 any] = List` and the like; a value declared
`IteratorOf[*FunctionType_Parameter]` still traveled as `any`, so every `next()` was cast, every
`Comparator<T>` class had an `any` bridge, and 587 adapter literals wrapped functions as objects.

What the two translator rules did (counts on tree6h, `lgcount.out` and `outside.out`):

| | fourth flip (JAVAISMS4.md) | tree6h |
|---|---:|---:|
| `type XOf[...] = X` aliases in g_0aliases.go | 52 | 34 |
| erased collection aliases in declarations (`ListOf[`, `MapOf[`, `SetOf[`, `IterableOf[`, `IteratorOf[`, ...; outside.sh's regex) | 726 | 31 |
| all `XOf[` alias mentions outside g_0aliases.go | not counted | 171 |
| declared slots typed with a typed library interface `ListT[T]` ... `Map_EntryT[K, V]` | 0 | 1,108 (+46 with an `any` type argument) |
| declared slots typed with a library func type (`Function[T, R]`, `Supplier[T]`, ... 12 types in g_zstubs.go) | 0 | 467 (+129 with an `any` type argument) |
| declared slots of a raw erased collection interface (`List`, `Map`, `Iterator`, ...) | not counted | 178 (158 of them results: the erased side of bridges) |
| adapter literals (census row) | 587 | 39 |
| of which typed `_tfn_` adapters | 542 | 0 |
| erased views `.asList()` / `.asMap()` / ... | 834 | 472 |

The census sites JAVAISMS4.md gave to R1 and R2, row by row, against the sites of the rules that
continue them here (T1-T7):

| row | R1 + R2 (fourth flip) | T1-T7 (tree6h) |
|---|---:|---:|
| erased collection constructors | 83 | 9 |
| Guava immutables / builders | 155 | 43 |
| for-each via iterator objects | 96 | 22 |
| interface casts | 82 | 16 |
| class casts | 361 | 118 |
| type assertions | 48 | 15 |
| explicit boxing | 524 | 272 |
| unboxing | 228 | 104 |
| adapter literals | 496 | 0 |
| Optional | 63 | 34 |
| Object slots | 215 | 119 |
| **total** | **2,351** | **752** |

(The fourth flip's column is the sum of its own R1/R2 table entries; T5 protobuf accessors, 26, are in
the right column because they are the same erasure seen through the protobuf runtime.)

What replaced R1 — the boundary between typed and erased code, which no census row counts
(`lgcount`, "boundary conversions", 743 calls in g_*.go):

| conversion | calls | what it is |
|---|---:|---|
| `X_un_(v)` (`List_un_`, `Map_un_`, `Collection_un_`, `Iterable_un_`, ...) | 366 | a typed value handed to erased code: the object a wrapper wraps |
| `xAs_(tc, v)` (`listAs_` 78, `iterableAs_` 56, `mapAs_` 48, `setAs_` 40, `collectionAs_` 30, `map_EntryAs_` 12, `iteratorAs_` 4) | 268 | an erased value made typed: an assertion or a converting wrapper |
| `lgBox_(v)` | 82 | a typed value as an Object |
| `elemsAs_[T](v)` | 27 | a typed range over an erased collection |

Where they go (`lgcount.out`, "by consumer"): returned 41 + 38 + 28 + 16 + 15 + 13, declared or
assigned 34 + 22 + 18 + 12 + ...; erased runtime functions `immutableListCopyOf_` 32,
`immutableSetCopyOf_` 23, `Iterator_remove` 17, `Map_get_s` 17, `Collection_contains_s` 16,
`Object_toString` 13, `tsetAddAllOfT_` 11, `equalsDyn_` 11, `tlistFrom_` 10, `immutableMapCopyOf_` 9,
`Iterables_isEmpty` 8, `sliceFrom_` 8, `Iterables_concat` 7, `List_sort` 7, ... So the R1 that remains
is not erased interfaces in the program's own code any more; it is (a) the runtime's static library
functions, which take and return erased values (rule T1), (b) the library types not yet typed (T2),
and (c) the conversions that join typed code to both. The conversion descriptors grew with the typed
code: `tc_X` 282 declared / 2,234 uses -> 316 / 2,755.

jsgo has none of the boundary: `jutil.OrderedMap[K, V]` with `Get(k K) V` and
`ComputeIfAbsent(k K, mapping func(K) V) V` (`internal/jutil/ordered_map.go:150,219`), slices, typed
`CompareTo(that *T) int` (`rhino/js_doc_info.go:392`, `parsing/ir_factory.go:181`), typed protobuf
lists (`jscomp/check_conformance.go:975 len(extendingRequirement.GetValueList())`).

## 3. What the census does not count (fourth flip -> tree6h)

Command: `cd tree/go/closure && bash ../../../outside.sh` (`outside.out`); implicit conversions from
`boxsites.tsv`.

| Java-ism | fourth flip | tree6h | jsgo |
|---|---:|---:|---|
| dispatcher calls `this.self_.m_impl(...)` | 1,637 | 1,646 | `t.self().M()` 103 |
| exact-class tests `p, _ := this.self_.(*C); p == this` | (in 1,618 assertions) | 1,332 | none |
| methods named `*_impl` | 1,785 | 1,788 | none |
| `.self()` calls | 644 | 476 | 103 |
| explicit upcasts `.up_X()` | 1,656 | 1,668 | none |
| nil-keeping conversions `X_as_Y(` | 714 | 556 | 2 helpers |
| implicit conversions of a typed value to `any` (without `panic(x)` and `self_` stores) | 3,056 | 2,647, of which 1,598 are `regGoEquals_` registrations at package init (294 allocate) | not counted |
| typed collections in declarations `*TList[`/`*TMap[`/`*TSet[` | 2,545 | 2,385 | `[]T`, `map[K]V`, OrderedMap |
| erased views of typed collections | 834 | 472 | none |
| element-conversion descriptors `tc_X` declared / used | 282 / 2,234 | 316 / 2,755 | none |
| Java collection method calls by name (`.get(`, `.put(`, `.add(`, ...) | 3,364 | 3,185 | not comparable |
| null-String sentinel `nullstr_` | 1,364 | 1,363 | `(string, bool)` 243 functions, `OptString` |
| `optstr*_(` | 1,871 | 1,869 | `OptString` |
| toString protocol helpers incl. `sstr_` | 2,615 | 2,592 | `jutil.ToJavaString` |
| `strBuilder_` / `strings.Builder` | 252 / 0 | 252 / 0 | 0 / 279 |
| equals / hashCode methods declared | 109 / 119 | 93 / 103 | 70 / 58 |
| typed and identity equals helpers | 205 | 204 | none |
| Java exception objects constructed | 3,551 | 3,545 | 60 exception types (`jutil/exceptions.go`) |
| inline null checks `panic(new_NullPointerException` | 681 | 685 | `jutil.CheckNotNull` 209 |
| `X_valueOf(` | 234 | 234 | 7 `_ValueOf` functions |
| Go `error` results | 0 | 0 | some (`regex.ParseRegExp`) |

## 4. The tree's own tools on a copy

`dryruns6.sh` ran every distinct tool invocation of `reflip/steps.txt` (102: `t TOOL ARGS`) with the
overlay's current tool sources (flip6, `tools/postflip`, copied) on `work/closure`, then the 7
`run.sh`-driven tools (eqhash, funcparams, selfeq, sbuild, strcuts, lambdas, enums: their `go run`
lines without the build). Result (`dryruns6.log`, `dry/`): of the 102, 88 change nothing (`optstr`
among them; it exits 1, "not proved, nothing rewritten"), 8 name 6 tools that exist only at older
revisions, 6 change the copy; the 7 `run.sh` tools change nothing:

| tool | change on this tree | census effect |
|---|---|---|
| `tostr` | 4 `objstr_(obj_(x))` / `objstr_(x.box_())` -> `tostr_(x)` (g_CrossChunkCodeMotion.go:533, g_InlineAndCollapseProperties.go:1078, :1290) | boxing -4: RULE E |
| `nnchecks` | 2 `Preconditions_checkNotNullT(...)` -> inline checks (g_TemplatizedType.go:64, :72) | Preconditions -2: RULE E |
| `casts -named` | removes 7 `icast_X` and 1 `is_X` helper that nothing calls | not sites (helper declarations) |
| `ranges -index` | 5 `eachRevOf_` cursors over slices -> index loops (ControlFlowAnalysis, Es6NormalizeClasses 2, Es6RewriteGenerators, MakeDeclaredNamesUnique) | no census row (`eachRevOf_` is not matched) |
| `etaconv` | 40 descriptor literals `func(v X) int32 { return objhashId_(v) }` -> `objhashId_[X]` | not sites |
| `tnames -undo` | 2,622 lines: renames the typed collections' Go method names back to Java's | not a Java-ism remover; a one-off step of the list, not idempotent by design |

Reasons the tools give for what they leave, used in section 7: `fnobj`: every remaining lambda object
is of an interface "a class implements it too"; `enums`: "left (the runtime uses
Node_Prop_CONSTANT_VAR_FLAGS): 1: Node_Prop"; `eqhash`: the 8 `objequals_` left compare interface
values of unbounded dynamic type; `charseq`: CharSequence "through interface Appendable" 5, "not a boxed
String" 6; `iters`: 0 iterators converted.

## 5. The rules, with their obligations

- **T1** is the shape the project already uses (`mapComputeIfAbsentT_`, `forEachT_`, `mergeT_`,
  `computeT_`, `sortSliceF_`, `tsetAddAllOfT_`): a typed runtime function next to the erased one and a
  translator name list mapping the Java method to it when the arguments are typed. Obligation: javac's
  type arguments at the call; a wildcard argument needs a typed view (variance, not counted).
  Exception, from REFLIP6.md: the 4 typed sorts of `Map.Entry` elements (PerformanceTracker,
  ProfilingReport, PruningAnalysis, TypeInferencePass) are not T1: the typed sort needs
  `K(box(x)) == x`, which entry wrappers break; a rule was written and dropped. They are REASON
  (8 casts + 8 parameters).
- **T2** extends the typed-interfaces rule's list of library types. The Guava typed immutables already
  exist in the runtime (post-flip 327-330); the 43 Guava census sites and 34 Optional sites are erased
  forms the translator still writes for them. Not a data-structure change: the same Java class, typed.
  This changes JAVAISMS4.md's verdict on the erased TreeMap/EnumMap/Deque/IdentityHashMap constructors
  (REASON there, "jsgo replaced each by hand"): the project now has typed twins of the same kind
  (rt_tdeque.go, rt_tsorted.go), so a mechanical typing exists and the hand rewrite is not the only way.
- **T3** keeps Java's semantics: `get(Object)` with a K-typed argument is the same call; with an
  argument of another static type it stays erased.
- **T4** needs T1's sorts to call `compareTo_t`; then the bridges are dead and `deadbridge` (in the
  list, a no-op today) removes them.
- **T5, T6, T7, R4, R5** are translator or protogen rules of the kind the two new rules are.
- **F1** is an extension of `funcparams`/`fnobj` (a class implementing the interface passes a method
  value); jsgo shows the target form.
- None of these rewrites what an algorithm does or which structure a pass uses.

## 6. Reasons that hold against jsgo (REASON rows, 425)

Grouped from `verdicts.tsv` (the row tables of section 7 have every label):

| reason | sites | jsgo |
|---|---:|---|
| Java API takes Object or CharSequence: toString helpers (`MoreObjects.ToStringHelper.add`, `objstr_`), `StringBuilder.append(Object)`, Joiner, Appendable, and lambdas computing such arguments (boxing 83 + 32; CharSequence parameters 14, which `charseq` leaves: "through interface Appendable", "not a boxed String") | 129 | jsgo's ToStringHelper form not looked up (as in JAVAISMS4.md); jsgo writes `string` for CharSequence |
| Java signature takes Object: exception and message arguments (`AssertionError(Object)`, MalformedTypedAstException, ChangeVerifier, Parser.reportExpectedError), args4j option values, LogFile.log(Object), LogsGson, Property keys, TypeTransformation entries (boxing 58; Object parameters 43) | 101 | Object in Java; jsgo has 56 `any` parameters besides `Equals` (JAVAISMS4.md's count) |
| explicit Iterator walks (68; 5 more are `this.hasNext()` inside program iterators) | 73 | 16 `.HasNext()` in all; `iters` converts 0 here |
| Object slots holding two or more types (Property.name, GlobalNamespace.Name.refsForNode, LogsGson_write, NodeTraversal's scope stack `Deque<Object>`, CompilerOptions' Object values, JSDocInfo.Property.isDefault, ClosureBundler.minifier, TypeEdgeJson.value) | 28 | jsgo keeps `any` for heterogeneous bags (`rhino/node_props.go:382`) |
| Double/Integer used as a nullable number | 29 | a difference: jsgo returns `(T, bool)` by hand (1,112 such functions, JAVAISMS4.md's count) |
| typed sorts of typed entries (REFLIP6.md "typed sorts") | 16 | jsgo sorts slices of entries it typed by hand |
| graph annotations `Annotatable_getAnnotation` | 14 | a difference: flow states in a slice indexed by node (`jscomp/data_flow_analysis.go:403`), a hand rewrite |
| ExecutorService / Future / submit / ThreadFactory | 13 | a difference: goroutine pools by hand (`jscomp/prebuild_dependency_info_parallel.go:58-63`) |
| `boxT_` of a type parameter | 10 | Go cannot assert a type-parameter value; jsgo's graph nodes are concrete types (`graph/linked_directed_graph.go:47`) |
| Object or interface results in Java (`getTypedScopeCreator`, `getScopeCreator`, `getStaticSourceFile`, `jstypeOrColor_get_`, `payload_get_`) | 6 | a difference: jsgo reads the node's type from a typed table (`rhino/node_flags.go:54 GetJSType() JSType`) |
| Java wildcard `List<? extends DiGraphEdge<N, E>>` | 4 | jsgo's graph is concrete, by hand |
| `LinkedDirectedGraph<ColorGraphNode, Object>` edge values | 2 | Object in Java |
| **total** | **425** | |

## 7. The rows, site by site

Generated by `rowtables.py` from `verdict_tables.out`; sub-causes with one stem are merged on one line.
Notes on rows whose verdict differs from JAVAISMS4.md:

- **Interface and class casts, unboxing.** The R1 sub-causes of JAVAISMS4.md (result of
  `Iterator.next`, `Map.get`, bridge parameters, for-each elements) are now split by where the erased
  value comes from: an erased runtime function (T1), an erased library type (T2), a Comparable/
  Comparator bridge (T4), a protobuf accessor (T5), a typed value boxed for a for-each (T6), a
  func-typed conversion (T7). The 255 `cast_[*X]` in `unbox:` fields and the 25 casts inside unbox
  functions of g_0casts.go (REFLIP6.md's "+19 ... unbox functions of the conversions of typed
  collections as elements") are not sites: they go with the descriptors (section 3).
- **JSDocInfo.Property's value methods** (`clone`, `equalValues`, `getTypeExpressions`, 13 icasts + 4
  casts + 2 boxes + 43 parameters): JAVAISMS4.md counted them under R4; jsgo has the same `any`
  signature (`func (p *JSDocInfo_Property) EqualValues(left any, right any) bool`,
  `rhino/js_doc_info.go:259`), so DONE here.
- **The equality helpers' Object argument** (`objequalsVia_`, `equalsBy_`, `objequalsBy_`, 60 boxes):
  JAVAISMS4.md put `equalsBy_`/`objequalsBy_` under R1/R2; they pass the Object argument of
  `equals(Object)`, which jsgo also takes as `any` (`jscomp/var.go:568`): DONE.
- **Lambda objects**: every one left is of an interface a class implements too (`fnobj`); jsgo writes
  an interface value for that case as well, a func type with a method (`CompilerPassFunc`,
  `jscomp/pass_factory.go:219`; `Source_TransformerFunc`, `bundle/source.go:337`;
  `ModuleLoader_ModuleResolverFactoryFunc`, `jscomp/deps_module_loader.go:425`) or a small struct
  (`baseTranspiler_absoluteLocationMapping`, `transpile/base_transpiler.go:232`;
  `moduleLoader_noopErrorHandler`, `jscomp/deps_module_loader.go:446`;
  `checkConformance_NodePrecondition`, `jscomp/check_conformance.go:125`;
  `typeCheck_inputProvider`, `jscomp/type_check_calls_props.go:538`): DONE 25 of 39 adapter literals
  (incl. the 2 Precondition method expressions); F1 9; ThreadFactory REASON 1; not split 4.
- **Optional**: the 28 mentions of `Optional[T]` (java.util.Optional as a value type, step 7) are DONE
  (jsgo: `(T, bool)` results and nilable pointers); the 34 Guava `Optional_base_*` calls are T2.
- **equals/hashCode**: the 66 `objequals_`/`objhash_` pairs of JAVAISMS4.md's R3 are gone (eqhash ran
  at 130/189); the 8 `objequals_` left compare interface values of unbounded dynamic type (`eqhash`'s
  own report) and are DONE with the other 46.

#### Row 0: erased Java collection constructors, 40 (DONE 22, RULE T2 9, not split 9)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | BitSet (jsgo jutil.BitSet, jutil/bitset.go:13) | 22 |
| RULE T2 | erased library class with no typed twin used here (Deque, EnumMap, TreeMap, IdentityHashMap) | 9 |
| not split | ArrayList / LinkedHashMap / LinkedHashSet wrapped at once by listAs_/mapAs_ or copied (colls left them; per-site reason not read) | 9 |

#### Row 1: typed TList/TMap/TSet constructors, 525 (not split 525)

| verdict | sub-cause | sites |
|---|---|---:|
| not split | typed TList/TMap/TSet constructor: newTMap_ 186, newTList_ 167, newTSet_ 152, newTListN_ 14, newTMapN_ 4, newTSetN_ 2 | 525 |

#### Row 2: Guava immutables / builders, 43 (RULE T2 43)

| verdict | sub-cause | sites |
|---|---|---:|
| RULE T2 | erased Guava immutable or builder: ImmutableList_copyOf_Iterable 7, ImmutableSet_builder 5, ImmutableList_of_Object.. 4, ImmutableList_copyOf 4, ImmutableSet_copyOf_Object..Arr 3, ImmutableSortedMap_copyOf 3, ImmutableSortedSet_copyOf 2, ImmutableMap_builder 2, ImmutableList_of 2, ImmutableMap_of_Object.. 2, ImmutableSortedSet_copyOfSorted 2, ImmutableList_builder 1, ImmutableSortedSet_of 1, ImmutableMap_of 1, new_ImmutableSortedSet_Builder 1, ImmutableSet_of_Object.. 1, new_ImmutableMap_Builder 1, ImmutableSortedSet_copyOf_Comparator_Collection 1 | 43 |

#### Row 3: native Go slices made, 100 (DONE 100)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | make([]T) (jsgo 732) | 100 |

#### Row 5: for-each via iterator objects, 34 (REASON 3, RULE T1 3, RULE T2 1, RULE T5 11, RULE T6 7, not split 9)

| verdict | sub-cause | sites |
|---|---|---:|
| REASON | Java wildcard List<? extends DiGraphEdge<N, E>> (no variance in Go; jsgo's graph is concrete, by hand) | 3 |
| RULE T1 | for-each over an erased runtime library function's result | 3 |
| RULE T2 | for-each over an erased Deque | 1 |
| RULE T5 | for-each over a protobuf list accessor (erased List) | 11 |
| RULE T6 | for-each over a typed value boxed to Object (program Iterable<T>, EnumSet) | 7 |
| not split | .each() cursor over a typed list or set (native keeps a cursor where the loop may change the collection; not split per site) | 9 |

#### Row 6: explicit Iterator use, 73 (REASON 73)

| verdict | sub-cause | sites |
|---|---|---:|
| REASON | explicit Iterator walk written by hand in Java (iters converts 0 on this tree; a range or index loop instead is a rewrite of the walk; jsgo has 16 HasNext) | 68 |
| REASON | hasNext() inside a program class implementing Iterator (Java's protocol; jsgo has 3 iterator types) | 5 |

#### Row 10: interface casts icast_X(, 183 (DONE 65, REASON 17, RULE T1 6, RULE T2 10, not a site 85)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | self() of a typed value: down/cross-cast in a class hierarchy | 44 |
| DONE | JSDocInfo.Property value methods take Object (jsgo EqualValues(left any, right any), rhino/js_doc_info.go:259) | 13 |
| DONE | Node / JSDocInfo property bag read (jsgo: GetProp any, node_cast / jsdocGet) | 7 |
| DONE | cross-cast of a typed interface value to another interface it implements | 1 |
| REASON | typed sort of typed entries (REFLIP6 'typed sorts': wrappers break K(box(x)) == x; a rule was tried and dropped) | 8 |
| REASON | boxT_ of a type parameter (Go cannot assert a type-parameter value) | 7 |
| REASON | Java wildcard List<? extends DiGraphEdge<N, E>> (Go has no variance; jsgo's graph is concrete, by hand) | 1 |
| REASON | Object slot holding values of two or more types in Java: GlobalNamespace_Name.refsForNode any 1 | 1 |
| RULE T1 | erased runtime library function: Map_forEach 2, Map_get_s 1, Map_computeIfAbsent_f 1, SequencedCollection_getFirst 1, Iterables_transform 1 | 6 |
| RULE T2 | value of an erased library type: Deque 2, Optional_base 2, PriorityQueue 1, immElems_(ImmutableSetOf 1, immElems_(*ImmutableSortedSet) 1 | 7 |
| RULE T2 | converting closure for an erased Guava type (sorted immutables, Optional.transform) | 2 |
| RULE T2 | entry of an erased library map (ImmutableSortedSet / erased entries) | 1 |
| not a site | helper declaration | 60 |
| not a site | inside the unbox function of a tconv_ descriptor (goes with the descriptors, section 3) | 25 |

#### Row 11: class casts to_X( / cast_[, 553 (DONE 95, REASON 27, RULE R4 7, RULE T1 60, RULE T2 33, RULE T4 18, RULE T5 4, RULE T6 2, RULE T7 1, not a site 306)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | self() of a typed value: down/cross-cast in a class hierarchy | 52 |
| DONE | Node / JSDocInfo property bag read (jsgo: GetProp any, node_cast / jsdocGet) | 14 |
| DONE | caught exception asserted after its catch test | 12 |
| DONE | Java downcast of an interface value to its class (jsgo asserts too) | 5 |
| DONE | equals(Object) parameter (jsgo Equals(any)) | 4 |
| DONE | JSDocInfo.Property value methods take Object (jsgo EqualValues(left any, right any), rhino/js_doc_info.go:259) | 4 |
| DONE | cast after an instanceof test of the same value | 4 |
| REASON | graph annotation (jsgo: flow states in a side slice, by hand) | 14 |
| REASON | boxT_ of a type parameter (Go cannot assert a type-parameter value) | 3 |
| REASON | Object or interface result in Java: AbstractCompiler.getTypedScopeCreator 2, NodeTraversal.getScopeCreator 1, Node.payload_get_ 1, Node.jstypeOrColor_get_ 1, Node.getStaticSourceFile 1 | 6 |
| REASON | Object slot holding values of two or more types in Java: GlobalNamespace_Name.refsForNode any 1, function LogsGson_write 1, method getNodeRootFromScopeObj 1, sliceAt_ 1 | 4 |
| RULE R4 | generic program class with an erased type parameter (AbstractCommandLineRunner.FlagEntry<T>.flag) | 7 |
| RULE T1 | erased runtime library function: newTSortedSetC_ 14, Map_computeIfAbsent_f 8, Map_get_s 7, Iterables_getOnlyElement 4, Iterables_getFirst 3, Object_clone 3, Iterables_filter 3, Map_forEach 2, List_sort 2, SequencedCollection_getLast 2, Map_replaceAll 2, CacheLoader_from 1, Iterable_forEach 1, Map_getOrDefault_s 1, Map_getOrDefault 1, Iterables_indexOf 1, MoreObjects_firstNonNull 1 | 56 |
| RULE T1 | element of toArray's []any (typed toArray) | 3 |
| RULE T1 | result of a func value typed func() any (converting closure) | 1 |
| RULE T2 | value of an erased library type: immElems_(ImmutableSortedSetOf 5, Optional_base 5, immElems_(*ErasedImmutableList) 5, ImmutableCollection 4, LoadingCache 2, ErasedImmutableMap 2, PeekingIterator 2, foreach_(DequeOf 1, immElems_(ImmutableListOf 1, IdentityHashMap 1, MapOf alias 1, ThreadLocal 1 | 30 |
| RULE T2 | entry of an erased library map (ImmutableSortedSet / erased entries) | 2 |
| RULE T2 | converting closure for an erased Guava type (sorted immutables, Optional.transform) | 1 |
| RULE T4 | bridge of Comparable/Comparator implemented by a class (erased callers: the runtime's sorts) | 17 |
| RULE T4 | bridge of a class implementing a library interface: load 1 | 1 |
| RULE T5 | protobuf list accessor returning an erased List | 3 |
| RULE T5 | protobuf list accessor or builder returning an erased value | 1 |
| RULE T6 | for-each over a typed value boxed to Object (a program class implementing Iterable<T>, EnumSet) | 2 |
| RULE T7 | converting closure of a func-typed conversion in g_0casts.go | 1 |
| not a site | cast function as a value (tconv_ unbox field) | 255 |
| not a site | helper declaration | 36 |
| not a site | comment | 6 |
| not a site | fallback inside a gcast_ helper | 6 |
| not a site | cast function as a value (arrmap_) | 2 |
| not a site | inside the unbox function of a tconv_ descriptor (goes with the descriptors, section 3) | 1 |

#### Row 12: instanceof is_X(, 71 (DONE 54, not a site 17)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | instanceof (jsgo typeCheck_isFunctionType one-line helpers, type_check_visit.go:75-80) | 54 |
| not a site | is_X helper declaration | 17 |

#### Row 13: type assertions x.(T), 2,047 (DONE 1897, REASON 4, RULE T1 7, RULE T2 6, RULE T5 2, not a site 131)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | dispatcher (exact-class test or single implementation; jsgo self_ JSType, jstype_named_type.go:128) | 1588 |
| DONE | instanceof as comma-ok assertion (jsgo 284 comma-ok assertions) | 195 |
| DONE | cast folded to v.(T): self() of a typed value or guarded | 109 |
| DONE | Java downcast of an interface value to its class | 2 |
| DONE | Node property bag read | 2 |
| DONE | equals(Object) parameter | 1 |
| REASON | Object slot holding values of two or more types in Java | 4 |
| RULE T1 | erased runtime library function: or converting closure (argument of Function_base 2, or converting closure (argument of Consumer 2, or converting closure (Iterables_getFirst 1, or converting closure (argument of Predicate_base 1, or converting closure (SequencedCollection_getLast 1 | 7 |
| RULE T2 | erased Guava immutable twin read back (lgBox_(..).(*ErasedImmutable..)) | 3 |
| RULE T2 | value of an erased library type: Optional_base 2 | 2 |
| RULE T2 | entry of an erased library map | 1 |
| RULE T5 | protobuf builder returning an erased value | 2 |
| not a site | body of a cast/instanceof helper | 129 |
| not a site | inside the unbox function of a tconv_ descriptor | 2 |

#### Row 14: type switches, 27 (DONE 4, not a site 23)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | type switch (jsgo 78; visitors as type switches, jstype_template_type_replacer.go:211) | 4 |
| not a site | type switch in a to_X / gcast_X helper | 23 |

#### Row 15: boxing to any (explicit), 938 (DONE 417, REASON 211, RULE E-tostr 4, RULE R4 16, RULE R5 5, RULE T1 92, RULE T2 71, RULE T3 62, RULE T4 11, RULE T6 19, RULE T7 17, not a site 13)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | variadic runtime argument (Strings_lenientFormat, String_format: ...any as jsgo) | 162 |
| DONE | the Object argument of equals (jsgo Equals(other any)) | 60 |
| DONE | identity comparison of values of two static types (needs an interface in Go) | 51 |
| DONE | Node / JSDocInfo property bag store (jsgo PutProp(prop, value any)) | 51 |
| DONE | variadic generated argument (Java varargs of Object: LogFile.log, Msg.format, Parser.report) | 45 |
| DONE | any(x).(T) on an interface value (Go's instanceof on an interface) | 18 |
| DONE | element of a []any literal | 15 |
| DONE | Object API of the JDK (getClass, identityHashCode, AutoCloseable close) | 12 |
| DONE | JSDocInfo.Property value methods take Object (jsgo EqualValues(left any, right any), rhino/js_doc_info.go:259) | 2 |
| DONE | JSDocInfo property values packed as Object (jsgo propertyValues any) | 1 |
| REASON | Java API takes Object or CharSequence (toString helpers, StringBuilder.append(Object), Joiner, Appendable) | 83 |
| REASON | Java signature takes Object (exception or message argument) | 49 |
| REASON | Java API takes Object (toString / format argument computed in a lambda) | 32 |
| REASON | Double/Integer local used as a nullable number (jsgo: (T, bool) results by hand) | 25 |
| REASON | Java signature takes Object (args4j option values, LogsGson, Property key, TypeTransformation entries, Logger) | 6 |
| REASON | Object result holding two types in Java | 4 |
| REASON | Double/Integer result used as a nullable number (jsgo: (T, bool) results by hand) | 4 |
| REASON | Object result in Java (return of CanCastToVisitor.castCastToHelper) | 2 |
| REASON | Object field holding two types (GlobalNamespace.Name.refsForNode) | 2 |
| REASON | Deque<Object> holding Nodes and scopes (NodeTraversal's scope stack) | 2 |
| REASON | Java LinkedDirectedGraph<ColorGraphNode, Object>: Object edge values | 1 |
| REASON | Object result in Java (return of ColorId.toLogsGson) | 1 |
| RULE E-tostr | objstr_(obj_(x)): the existing tool tostr rewrites it to tostr_(x) on this tree | 4 |
| RULE R4 | generic program class with an erased type parameter (AbstractCommandLineRunner.FlagEntry<T>) | 10 |
| RULE R4 | generic program class with an erased type parameter (SourcePosition<T>.setItem) | 6 |
| RULE R5 | Object field holding one boxed primitive (tri-state Boolean/Integer) | 5 |
| RULE T1 | runtime method whose Java signature is typed or overloaded (BigInteger.compareTo(BigInteger), ComparisonChain.compare(Comparable), Enum.compareTo(E)) | 24 |
| RULE T1 | argument of an erased runtime library function: splitToListT_ 22, Map_computeIfAbsent_f 6, MoreObjects_firstNonNull 6, Object_clone 3, TextFormat_merge 1, Collections_singletonList 1, Map_put_s 1, Map_putIfAbsent_s 1, Ascii_equalsIgnoreCase 1, Integer_parseInt_CharSequence_int_int_int 1, Map_replace 1 | 44 |
| RULE T1 | result of a lambda given to an erased runtime library function | 22 |
| RULE T1 | key of Comparator.comparing typed any (the runtime's comparing erases U extends Comparable) | 2 |
| RULE T2 | argument of an erased library type's method (Guava immutables, Optional, Ordering, TreeMap, EnumSet...) | 70 |
| RULE T2 | argument of an erased library type's method (ThreadLocal) | 1 |
| RULE T3 | Object parameter of a typed collection method (Map.get/containsKey/remove, Collection.contains, List.indexOf take Object in Java) | 50 |
| RULE T3 | erased element argument of a collection method (Map.put, List.add, typed map Put of an erased value) | 12 |
| RULE T4 | bridge of a program class implementing Iterator<T> (next returns Object) | 7 |
| RULE T4 | bridge of a program class implementing a library interface (clone, CacheLoader.load) | 3 |
| RULE T4 | bridge of a class implementing a library interface: Source.Lazy, Supplier<String> 1 | 1 |
| RULE T6 | for-each over a typed value boxed to Object | 19 |
| RULE T7 | converting closure of a func value with a type argument any (func-typed type arguments) | 15 |
| RULE T7 | argument of a func value with an erased type argument | 2 |
| not a site | comment | 9 |
| not a site | box function of a tconv_ descriptor (goes with the descriptors) | 4 |

#### Row 16: unboxing from any, 131 (DONE 6, REASON 7, RULE R4 13, RULE T1 49, RULE T2 20, RULE T4 5, RULE T5 9, RULE T6 5, RULE T7 16, not a site 1)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | Node / JSDocInfo property bag read (jsgo: GetProp any, node_cast / jsdocGet) | 4 |
| DONE | comma-ok test of an Object | 2 |
| REASON | Object slot holding values of two or more types in Java: TEntry.getValue 1, function LogsGson_write 1, method getNodeRootFromScopeObj 1, sliceAt_ 1, slicePeekTop_ 1, Property.name_ any 1, TMap.Get 1 | 7 |
| RULE R4 | generic program class with an erased type parameter (SourcePosition<T>) | 13 |
| RULE T1 | erased runtime library function: Map_get_s 12, Iterables_getFirst 5, Iterables_get 5, List_sort 4, Iterables_getOnlyElement 3, Iterables_transform 2, Map_forEach 2, newTSortedSetC_ 2, MoreObjects_firstNonNull 2, Lists_transform 2, Iterable_forEach 1, Iterables_filter 1, Lists_reverse (for-each over its result 1, GatherModuleMetadata_DOT_SPLITTER (for-each over its result 1, SequencedCollection_removeLast 1, Io_listFiles (for-each over its result 1, Iterables_getLast 1 | 46 |
| RULE T1 | result of a func value typed func() any (converting closure) | 3 |
| RULE T2 | entry of an erased library map (ImmutableSortedSet / erased entries) | 5 |
| RULE T2 | value of an erased library type: immElems_(ImmutableSortedSetOf 5, immElems_(*ErasedImmutableList) 3, Optional_base 2, elems_(IterableOf 1 | 11 |
| RULE T2 | converting closure for an erased Guava type (sorted immutables, Optional.transform) | 4 |
| RULE T4 | bridge of a class implementing a library interface: Source_Lazy.get 2, add 1 | 3 |
| RULE T4 | bridge of Comparable/Comparator implemented by a class (erased callers: the runtime's sorts) | 2 |
| RULE T5 | protobuf list accessor returning an erased List | 9 |
| RULE T6 | for-each over a typed value boxed to Object (a program class implementing Iterable<T>, EnumSet) | 5 |
| RULE T7 | func type as a type argument of a typed container (descriptor tc_Function_any__any_) | 15 |
| RULE T7 | converting closure of a func-typed conversion in g_0casts.go | 1 |
| not a site | helper declaration | 1 |

#### Row 17: lambda objects &X_fn{func, 37 (DONE 23, REASON 1, RULE F1 9, not split 4)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | lambda object of an interface a class implements too (jsgo: func type with a method or a small struct) | 23 |
| REASON | executor plumbing (jsgo wrote goroutine pools by hand) | 1 |
| RULE F1 | lambda object where jsgo passes a plain func (NodeUtil_VisitPreOrder func(rhino.Node); deferAction(fn func())) | 9 |
| not split | lambda object Iterable (Iterable lambda given to iterableAs_; jsgo form not looked up) | 4 |

#### Row 18: lambda adapter literals, 39 (DONE 25, REASON 1, RULE F1 9, not split 4)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | lambda object of an interface a class implements too (jsgo: func type with a method or a small struct) | 25 |
| REASON | executor plumbing (jsgo wrote goroutine pools by hand) | 1 |
| RULE F1 | lambda object where jsgo passes a plain func (NodeUtil_VisitPreOrder func(rhino.Node); deferAction(fn func())) | 9 |
| not split | lambda object Iterable (Iterable lambda given to iterableAs_; jsgo form not looked up) | 4 |

#### Row 19: Optional, 75 (DONE 31, RULE T2 34, not a site 10)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | java.util.Optional as the typed value Optional[T] (jsgo: (T, bool) results, nilable pointers) | 31 |
| RULE T2 | erased Guava Optional (Optional_base_*) | 34 |
| not a site | string literal | 10 |

#### Row 20: try/catch emulation, 54 (DONE 54)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | try/catch emulation (recover at boundaries, catch_ of the recovered value; jsgo 58 recover()) | 54 |

#### Row 21: panic(, 3,523 (DONE 3523)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | panic of a Java exception (jsgo panic(jutil.NewX(..)) 1,724) | 3523 |

#### Row 22: defer, 84 (DONE 84)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | defer (finally, try-with-resources, catch) | 84 |

#### Row 23: equals/hashCode protocol, 72 (DONE 54, RULE R7 18)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | equals/hashCode protocol on Object values (jsgo ObjectsEquals objects.go:34, 70 Equals methods) | 52 |
| DONE | requireNonNull used for its value (jsgo jutil.CheckNotNull[T]) | 2 |
| RULE R7 | Objects.equals of two erased views of typed collections | 18 |

#### Row 24: String_* helpers, 121 (DONE 121)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | UTF-16 String helper (jsgo jutil/jstring.go StringLength, StringCharAt...) | 121 |

#### Row 25: StringBuilder, 5 (RULE R7 2, not a site 3)

| verdict | sub-cause | sites |
|---|---|---:|
| RULE R7 | Appendable(new_StringBuilder()) (manual step 303 on the lead line) | 2 |
| not a site | name table | 2 |
| not a site | StringBuilder_as_Appendable helper declaration | 1 |

#### Row 26: enum objects: struct, 1 (RULE R7 1)

| verdict | sub-cause | sites |
|---|---|---:|
| RULE R7 | Node_Prop as an object enum: enums refuses (the runtime names Node_Prop_CONSTANT_VAR_FLAGS); jsgo type Node_Prop int32 (rhino/node_props.go:49) | 1 |

#### Row 27: enum objects: call, 1 (RULE R7 1)

| verdict | sub-cause | sites |
|---|---|---:|
| RULE R7 | Node_Prop as an object enum: enums refuses (the runtime names Node_Prop_CONSTANT_VAR_FLAGS); jsgo type Node_Prop int32 (rhino/node_props.go:49) | 1 |

#### Row 28: Preconditions_, 6 (DONE 4, RULE E-nnchecks 2)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | Preconditions.checkNotNull (jsgo jutil.CheckNotNull, preconditions.go:236) | 4 |
| RULE E-nnchecks | the existing tool nnchecks makes it an inline check on this tree | 2 |

#### Row 29: threads/executors, 53 (DONE 28, REASON 12, not a site 13)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | CompilerExecutor / one-thread Thread (jsgo compiler_pipeline.go:690-701, jutil/logger.go:370-378) | 28 |
| REASON | ExecutorService / Future / submit (jsgo wrote goroutine pools by hand, prebuild_dependency_info_parallel.go:58-63) | 12 |
| not a site | name table or comment | 13 |

#### Row 30: reflect, 5 (not a site 5)

| verdict | sub-cause | sites |
|---|---|---:|
| not a site | string literal (reflect.) or a local named unsafe | 5 |

#### Row 31: unsafe, 1 (not a site 1)

| verdict | sub-cause | sites |
|---|---|---:|
| not a site | string literal (reflect.) or a local named unsafe | 1 |

#### Row 100: fields/params declared any (Object slots), 448 (DONE 141, REASON 70, RULE R4 5, RULE R5 51, RULE T1 66, RULE T2 1, RULE T4 29, RULE T7 23, not a site 62)

| verdict | sub-cause | sites |
|---|---|---:|
| DONE | equals(Object) parameter (jsgo Equals(other any), var.go:568) | 79 |
| DONE | JSDocInfo.Property value methods take Object (jsgo EqualValues(left any, right any), rhino/js_doc_info.go:259) | 43 |
| DONE | Node / JSDocInfo property bag (jsgo PutProp(prop, value any)) | 11 |
| DONE | typed reader of the JSDocInfo property bag | 5 |
| DONE | property bag (jsgo js_doc_info.go:75, node_props.go:382 are any) | 2 |
| DONE | typed reader of the JSDocInfo property bag (jsgo jsdocGet[T]) | 1 |
| REASON | CharSequence or Object in Java (charseq leaves it: through interface Appendable / not a boxed String) | 14 |
| REASON | typed sort of typed entries (REFLIP6 'typed sorts': wrappers break K(box(x)) == x; a rule was tried and dropped) | 8 |
| REASON | Object in Java: logObject 3, logJsonObject 3, ChangeVerifier_EqualsResult_notEqual 2, JSCompObjects_identical 2, logObject_impl 2, logJsonObject_impl 2, init_ClosureBundler_Transpiler_Transpiler_EvalMode_Optional_String_Object_Map_boolean 1, new_ClosureBundler_Transpiler_Transpiler_EvalMode_Optional_String_Object_Map_boolean 1, traverseEdge 1, CmdLineParser_Option_addValue 1, CmdLineParser_Option_asList 1, cfgEdge_ 1, favors 1, DotFormatter_formatNodeName 1, HamtPMap_hash 1, newNumberNode 1, J2clEqualitySameRewriterPass_isSafeNumber 1, addSupertypeEdges 1, LogsGson_toJson 1, LogsGson_write 1, init_MalformedTypedAstException_Object 1, new_MalformedTypedAstException_Object 1, init_MalformedTypedAstException_Object_Throwable 1, new_MalformedTypedAstException_Object_Throwable 1, MalformedTypedAstException_checkWellFormed_boolean_String_Object 1, setReflectedObject 1, getNodeRootFromScopeObj 1, reportExpectedError 1, isReasonableDoubleValue 1, init_Property 1, new_Property 1, join_1 1, addExtension 1, addNewEntry 1, new_AssertionError_Object 1 | 43 |
| REASON | Object edge values of LinkedDirectedGraph<ColorGraphNode, Object> | 1 |
| REASON | Object field holding two or more types: minifier 1, value_ 1, refsForNode 1, name_ 1 | 4 |
| RULE R4 | SourcePosition<T>.setItem: an erased type parameter | 4 |
| RULE R4 | SourcePosition<T>.item: an erased type parameter | 1 |
| RULE R5 | command-line option setter of a nullable boxed flag (Consumer[any] for Boolean/Integer) | 38 |
| RULE R5 | Object field holding one boxed primitive or one type (tri-state Boolean/Integer, Source.Lazy's String) | 7 |
| RULE R5 | setter of a tri-state boxed flag | 3 |
| RULE R5 | command-line option setter of a nullable boxed flag | 1 |
| RULE R5 | Integer value of a typed map held as any (nullable boxed Integer) | 1 |
| RULE R5 | Source.Lazy's captured String held as Object | 1 |
| RULE T1 | parameter of a converting closure given to an erased runtime library function | 37 |
| RULE T1 | parameter of a converting closure (typed function value given to an erased runtime function) | 29 |
| RULE T2 | erased library map (MapOf alias) of Timeline | 1 |
| RULE T4 | bridge of Comparable/Comparator (erased callers: the runtime's sorts) | 19 |
| RULE T4 | bridge of a class implementing a library interface: apply 2, contains 2, put 2, load 1, add 1, containsKey 1, containsValue 1 | 10 |
| RULE T7 | func type as a type argument of a typed container (Node.PROP_MAP_FOR_EQUALITY) | 15 |
| RULE T7 | converting closure of a func-typed conversion (g_0casts.go) | 8 |
| not a site | parameter of a tconv_ descriptor's unbox function | 51 |
| not a site | j2go stub of a library class (g_zstubs.go: members not yet in the runtime) | 7 |
| not a site | type parameter's helper parameter (to_X of a generic class) | 2 |
| not a site | `if any(this) == x` (regex match) | 2 |
