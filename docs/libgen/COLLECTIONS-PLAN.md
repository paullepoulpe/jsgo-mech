# Typed collections, the other half: map of the work, plan, and living record

Branches `main-libgen-b3` (worktree `/mnt/data/ubuntu/jm/wt/agZ`; sections 0 to 3) and
`main-libgen-b4` (worktree `/mnt/data/ubuntu/jm/wt/agZ2`; section 4), on top of `main-libgen-b2`
c8a3f9d5 (LIBGEN-STUDY.md sections 7 and 8: rules "function types" and "typed library
interfaces", kind `iter`). This file is the handover: a successor continues from it alone.
**The current state, what is left and the traps are in section 5** (stage b5, branch
`main-libgen-b5`, worktree `/mnt/data/ubuntu/jm/wt/agZ3`; 5.7 replaces 4.6; sections 0 to 4 are
the records of the stages before, kept as written).
**Section 6 records rule candidate T2 of `docs/javaisms6/JAVAISMS6.md`** (branch
`main-libgen-t2`, worktree `/mnt/data/ubuntu/jm/wt/libt2`: kinds `sorted`, `enumset`, `cmaps`,
`deque`, `guava`; 6.6 adds to 5.7 what is left of the library types).
Scratch files of this stage: `/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/b3/`
(`tr.sh` translate + build, `trto.sh <dir>` translate into a scratch directory that has the
runtime copied in -- the translator reads `rt_*.go` of its output directory, so a directory
without them gives a different translation --, `map*/` reports, gate logs).

## 0. The map (translator's own data)

Instrument: `tools/j2go/src/j2go/CollMap.java`, report switch `J2GO_COLLMAP=<dir>` (no rule
reads it; the generated files are byte-identical with and without it). It runs after rule
"typed collections" (`Colls`) has decided and writes

- `decls.tsv`: every declaration (field, local, parameter, for-each variable, result of a
  method of the program) whose Java type is a collection type (a `java.util.Collection`, a
  `Map`, a `Map.Entry`, a generic class of Guava's collect package), typed or erased, with every
  reason found (`Colls.Slot.whys`: all reasons, not only the first as `J2GO_COLLS=2` prints);
- `slots.tsv`: the declarations of `Colls` with the edges "erased if that one is";
- `origins.tsv`: every expression that makes a collection value which is not the value of a
  declaration (`new`, the result of a library method, a cast, the result of a program method
  whose declared result is a type variable), its javac type, the kind of each type argument,
  and the place its value enters.

`docs/libgen/collmap.py <dir> [min]` makes the tables; `docs/libgen/collmap-b2.out` is its
output for the tree this stage starts from. Numbers below are from it.

### 0.1 Declarations: 3,867 of a collection type, 3,051 typed, 816 erased

| reason an erased declaration is erased (root reasons, through "exchanges values with") | decls | piece |
|---|---:|---|
| the type has no kind in `Colls`: `Collection` 119, `EnumSet` 16, `ImmutableSortedSet` 15 (+ its builder 5), `Map.Entry` outside a loop 11, `ConcurrentMap`/`ConcurrentHashMap` 8, `ImmutableCollection` 5, `ImmutableSortedMap` (+ builder) 8, `TreeMap` 4, other 2 | 193 | K (kinds), 1 |
| result of a method of the program that is no declaration of the rule: its declared result is a type variable (`Outcome<T>.get` 22 direct, `JSDocInfo.Property.get` 16, `SimpleSetMultimap/ListMultimap.get` 10, `Builder.getProp*` 6, `Supplier.get`) or the graph classes' wildcard results (12) | 123 | K + nested, 6 |
| the method is not a candidate: group with different collection types there 53, a lambda's parameter 20, record / annotation / handle method 16, method with type parameters 12, no body 8, interface nothing implements 1 | 110 | K |
| element of another collection (`Map.get`, `computeIfAbsent`, `Entry.getValue`, a for-each variable: collections of collections) | 66 | K + nested |
| a variable that is no declaration of the rule (a `Collection`, a lambda parameter, another instantiation) | 45 | K |
| result of a runtime function (static: `Arrays.asList`, `Lists.reverse`, `Ordering.sortedCopy`, `Maps.newLinkedHashMapWithExpectedSize`, `Sets.*`, `Collections.nCopies`, `EnumSet.of`, ...) 38, of a runtime instance method 13 | 51 | 3 |
| type arguments no typed collection holds: wildcard key or element 9, key with type arguments 10, `Object` key 3, array element 2, other | 38 | stays (see 0.4), part K |
| lambda / method reference (the method is named by one, or implements a functional interface) | 31 | K |
| entry not from a loop over the entries of a typed map | 29 | 2 |
| view of a collection: `keySet` / `values` 16, `subList` / `tailSet` / multimap views 5 | 21 | 2 |
| generic class of the program read at another instantiation | 17 | K |
| a constructor the typed family lacks: `ConcurrentHashMap` 6, `EnumMap` 2, anonymous 3, `LinkedList`, `PriorityQueue`, `CopyOnWriteArrayList` | 14 | 1 |
| protobuf repeated field (`getXList`) | 9 | 4 |
| a use the view lacks (`subList` 7, `getFirst` 2, a place of the class type 4) | 13 | K |
| other (switch expression, `case`, cast, varargs, annotated field) | 17 | K |
| several of the above | 39 | |

By the kind of the worst type argument of the 816: exact class / interface / String 610, a Go
type parameter 55, a collection as element 41, `? extends X` 34, a box 29, **`?` 14, `Object` 12,
erased type variable 7**, other library generic 8, array 4. So about 780 of the 816 are erased
for a reason other than their Java type; 33 are erased "really".

### 0.2 What removing each reason class alone gives to `Colls` (declarations of its kinds, 3,423)

`Colls` types a declaration as the concrete `*TList[X]` / `*TSet[X]` / `*TMap[K,V]` only if every
value entering it is one. Simulation over `slots.tsv` (reasons removed, taint re-propagated):

| reason class removed | declarations typed (2,954 today) | gain |
|---|---:|---:|
| 3 runtime functions and methods give typed results | 2,995 | +41 |
| 2 views of typed collections and their entries | 3,044 | +49 |
| 5 nested collections | 3,109 | +65 |
| 1 the constructors the typed family lacks | 3,127 | +18 |
| 4 protobuf lists | 3,144 | +17 |
| 6 program results of type-variable type, variables of other types | 3,329 | +185 |
| 7, 8, 9, a, b, c (uses, lambdas, instantiations, annotated fields, other) | 3,423 | +94 |

No single class is large: the erasure is a long tail (the largest single functions block 3
declarations each). The counts say that the lever is not more proofs in `Colls` but a typed
form that needs no proof: the kinds (section 1).

### 0.3 Expressions that make a collection value: 3,808

| what makes the value | enters a typed declaration | for-each over a view of a typed map (reads the map) | erased |
|---|---:|---:|---:|
| `new` | 1,045 | | 117 |
| view of a map (`keySet`, `values`, `entrySet`) | 4 | 152 | 169 |
| other view (`subList`, `tailSet`, ...) | 8 | | 12 |
| runtime function (static) | 742 | | 319 |
| runtime method (instance) | 518 | | 76 |
| protobuf repeated field | 0 | | 89 |
| element of a collection / result of a function value | 314 | | 157 |
| program method with a type-variable result | 13 | | 67 |
| cast | 0 | | 6 |
| total | 2,644 | 152 | 1,012 |

The 1,012 erased ones by the worst type argument: exact 795, collection as element 159, Go type
parameter 74, non-generic (protobuf string lists) 51, box 29, other library generic 20,
**`Object` 13, erased type variable 11, `?` 6**, array 5.

Top functions (erased origins; where the value goes; full list in `collmap-b2.out` section 6):

| origin (sites of 4 or more in `collmap-b2.out` section 6, summed over overloads and map classes) | erased sites | goes to |
|---|---:|---|
| `ImmutableList.of(..)` 58, `ImmutableSet.of(..)` 22, `ImmutableMap.of()` 10 | 90 | erased declaration 76 (results of method groups, `Collection`) |
| `Map.keySet()` on a typed map 46, `values()` 39, `entrySet()` 7 (not counting for-each loops, which read the map) | 92 | library argument 58, erased declaration 20, receiver 14 |
| `Map.values()` / `entrySet()` / `keySet()` on an erased map | 57 | for-each 31, erased declaration 13, library argument 12 |
| `new ArrayList<>(..)` | 52 | erased declaration 44, library argument 4, lambda result 3 |
| `new LinkedHashSet<>()` 17, `new LinkedHashMap<>()` 15, `new ConcurrentHashMap<>()` 8 | 40 | erased declaration 22, lambda result 13 (`computeIfAbsent`) |
| `Collections.unmodifiable*` | 37 | erased declaration 36 |
| `ImmutableList.copyOf` | 25 | erased declaration 14, for-each 5, library argument 4 |
| `ImmutableList.Builder.build()` 22, `ImmutableSet.Builder.build()` 6 (typed builders) | 28 | erased declaration 24 |
| `Arrays.asList` | 19 | erased declaration 11, library argument 6 |
| `Outcome.get` 32, `JSDocInfo.Property.get` 21, `Builder.getProp` 7 (type-variable results) | 60 | erased declaration 51 |
| `Map.Entry.getValue` 26, `Map.get` 31, `computeIfAbsent` 10, `Supplier.get` 9 (a collection as element) | 76 | erased declaration 45, receiver, library argument |
| `Requirement.getValueList` 21 and the other protobuf lists | 89 | for-each, erased declaration |

(`Objects.requireNonNull` 24, `checkNotNull` 10, `Map.put` 11, `ImmutableMap.Builder.put` 27 are
calls whose collection-typed result is not used or is the builder; they are no origins in fact.)

Conversion spellings in the generated text today (`g_[A-Z]*.go`): `iterableAs_(` 179,
`iteratorAs_(` 22, `X_un_(` 85, `tlistImm_(` / `tsetImm_(` / `tmapImm_(` 38 / 28 / 18 (an erased
immutable entering a typed declaration: makes the typed twin once), `.asMap().keySet/values/entrySet()` 92,
`tlistFrom_` / `tsetFrom_` / `tmapFrom_` 36, constructors of the erased family (`new_ArrayList`, ...) 102.
`baseline.sh` of the starting tree: `scratchpad/b3/baseline-0.out` (as LIBGEN-STUDY.md section 8, "after").

### 0.4 What must stay erased at the end (the target)

Declarations whose Java type argument is `?` (14), `Object` (12; `Map<String, Object>` is
already a `*TMap[string, any]`: "erased" there means only that the element is `any`), an erased
type variable of a program generic method (7), and the two unchecked casts
(`GlobalNamespace.Name.castRefsForNodeMap`, `JSTypeRegistry.createRecordType`). Everything else
in 0.1 is to be typed.

## 1. Design

Three levels for a place of a collection type `C<X>`:

- **L1** (today, rule "typed collections"): the concrete typed collection `*TList[X]`, when
  `Colls` proves that only such objects enter. Fastest form; kept.
- **L2** (new, the collection kinds of rule "typed library interfaces"): the typed interface
  `ListT[X]` / `CollectionT[X]` / `SetT[X]` / `MapT[K,V]` / `EntryT[K,V]` in the form `iter`
  established: the erased interface plus the methods that give elements at their Java type under
  the name `_t` (`get_t`, `getKey_t`, `values_t`, ...). No proof is needed: any value enters,
  natively if its object has the typed methods (the view of a typed collection, a typed view of
  a typed map, a typed runtime result), else through a wrapper (`listAs_(tc, v)`), which costs an
  allocation. Methods that only take elements keep the erased method until a typed one is added
  (the interface embeds the erased one, so coverage can grow method by method).
- **L3**: the erased interface, only where 0.4 says so.

"The other half" is what keeps wrappers from being allocated: **a value is made typed by the
place it enters.** An origin expression (`new ArrayList<>()`, `Lists.newArrayList()`,
`m.values()`, `ImmutableList.of(..)`, a protobuf list) whose value enters an L1 place is the
concrete typed collection (today only constructors, builders and immutable factories; piece A
adds the runtime's factories); entering an L2 place it is a native object of the typed interface
(the typed collection's view, a typed map view); entering an erased place (`Object`, `?`, a
library parameter) it stays the erased object, which is the cheapest form for erased readers.
Nested collections (`Map<K, List<X>>`) are an L2 element type: `*TMap[K, ListT[X]]`.

Why not make every origin typed first, independent of the kinds: a typed collection read
through its erased view boxes on every read (a String allocates), so typing an origin whose
consumers are still erased is a pessimisation until its consumers are typed. Hence the order
below: origins that already have typed consumers first, then each kind together with the
origins that feed its places.

## 2. Pieces, in build order

Each piece: a switch, self-test, runtime tests, gate (`$B/gate.sh agZ <label>`), CHECKS of the
18% subset with allocation, `baseline.sh`, runtime lines.

| # | piece | switch | what | expected |
|---|---|---|---|---|
| A | makers | `J2GO_NOLIBGEN=makers` | runtime factories that make a fresh collection are typed where they enter an L1 declaration (`Maps.newLinkedHashMapWithExpectedSize`, `Sets.newLinkedHashSetWithExpectedSize`, `Lists.newArrayList*`, `Sets.newHashSet*`, `Maps.newHashMap*`, `Ordering.sortedCopy`, ...: a table, as `TYPED_MAKERS`) | up to +41 L1 declarations (`Scope.vars`, `TypedScope.vars`) |
| B | views | `J2GO_NOLIBGEN=views` | `keySet()` / `values()` / `entrySet()` of a typed map are native typed objects (first for `IterableT`; with the kinds `SetT[K]` / `CollectionT[V]` / `SetT[EntryT[K,V]]`) | the +3.8 M `iterableAs_` wrappers of section 8 gone; 123 view origins native |
| C | kind `coll` | `J2GO_NOLIBGEN=coll` | `Collection<X>` places are `CollectionT[X]` (= `Collection` + `IterableT[X]`) | 119 declarations, the 306 `addAll`-style flows unchanged (library parameters stay erased) |
| D | kind `list` | `=list` | `List<X>` places that are not L1: `ListT[X]` (`get_t`, `set_t`, `remove_t`), typed-by-place `new ArrayList`, `ImmutableList.of/copyOf`, builders' `build`, `Collections.unmodifiableList/emptyList`, `Arrays.asList`, `subList` | 250 + 38 + 30 declarations |
| E | kind `set` | `=set` | `SetT[X]`, with the set origins | about 130 declarations |
| F | kind `map` (+ `entry`) | `=map` | `MapT[K,V]`, `EntryT[K,V]`, typed views natively, nested collections as L2 elements | about 190 declarations, the 157 nested origins |
| G | protobuf lists | `=proto` | `tools/protogen` emits typed repeated fields | 89 origins, 9-17 declarations |
| H | variance | | graph classes' `getOutEdges` / `getInEdges`: a read-only covariant view made once | 21 overrides, 43 flows |
| I | deletion | | erased classes nothing constructs any more | runtime lines |

State of each piece: section 3.

## 3. Record (state at the end of this session, 2026-11-20)

**How far: pieces A (makers), B (views, the part that the kinds built so far need), C (kind
`coll`), D (kind `list`, with typed-by-place and the variance decision) are built and gated.
Kinds `set` and `map`/`entry`, protobuf lists, the class-typed declarations, the deletion of
erased runtime code and the refusal for unchecked casts are NOT built.** The order differs
from section 2 as planned there and from the brief's guess ((3), (2), (1), then the kinds):
the map showed that origins are a long tail whose consumers are erased places, so each kind was
built together with the origins that feed it (section 1, last paragraph).

### 3.1 Commits (branch `main-libgen-b3`, on c8a3f9d5)

| commit | what | switch (`J2GO_NOLIBGEN=`) |
|---|---|---|
| 3cd3524c | this file, `CollMap.java`, `collmap.py`, `Colls.Slot.whys` (report only) | `J2GO_COLLMAP=<dir>` |
| 7b097dba | runtime: `keySet()` / `values()` of the view of a typed map are native typed Iterables | (runtime) |
| a421fb83 | pieces `makers` and `cov` (for `Iterable`) | `makers`, `cov` |
| ba3ad013 | kind `coll`; `valuesT()`; `Collections.unmodifiableX(x)` passes through | `coll`, `views` |
| 6c7533b9 | kind `list`; piece `place`; variance of Lists; erased method for an `Object` consumer; typed `subList`; README | `list`, `place` |

With `J2GO_NOLIBGEN=makers,views,place,cov,coll,list` the generated files are byte-identical
to those of the starting tree (checked: `scratchpad/b3/outMine` against `out0`). Not behind a
switch: the runtime changes (`tlistView_.subList` is a typed list on the same items instead of
a boxed copy; `TMap.delAt` refuses an immutable map; the new methods), and "a typed call whose
result enters an `Object` is the erased method", which also applies to kind `iter` (no site of
the tree changed by it with the other switches off).

### 3.2 Counts

`baseline.sh` (`g_[A-Z]*.go`), start -> end: `cast_[` 924 -> 787, `icast_X(` 292 -> 238,
`str_(` 228 -> 196, `Node_of_(` 135 -> 98, `obj_(` 569 -> 557, `strobj_(` 402 -> 396,
erased views `.asList()` etc. 1,117 -> 873, `foreach_(` 478 -> 272 (`eachOf_(` 146 -> 350),
`XOf[` alias uses 833 -> 485, `tc_` uses 2,439 -> 2,679 (the conversions take one).
New spellings: `ListT[` 440, `CollectionT[` 151, `.asListT()` 221, `.asCollT()` 61,
`.valuesT()` 42, `get_t(` 34, `X_un_(` 85 -> 367 (typed value given to an erased parameter:
one type test, no allocation).

Declarations of a collection type (3,867; `J2GO_COLLMAP`, `scratchpad/b3/mapE`):

| state | start | end |
|---|---:|---:|
| L1 concrete typed collection | 3,051 | 3,067 |
| L2 typed interface (`ListT`, `CollectionT`) | 0 | 342 |
| erased: `List` with a wildcard (variance, see 3.4) | | 23 |
| erased: kind not built: set (`Set`, `EnumSet`, `ImmutableSortedSet`, ...) | | 143 |
| erased: kind not built: map / entry | | 194 |
| erased: class-typed (`ArrayList<X>`, `ImmutableList<X>`), builders, deques, other | | 98 |
| erased, total | 816 | 458 |

Conversions left (`J2GO_LIBGEN=1`): `iterableAs_` 179 -> 119, `iteratorAs_` 22 -> 14,
`collectionAs_` 110, `listAs_` 84 (results of erased program methods and runtime functions,
erased `ImmutableList` / `ArrayList` declarations, `Arrays.asList`, elements of collections of
collections), 268 uses of a `List` with a wildcard left erased. Whether one allocates is
decided at run time (native check); see 3.3 for what does.

Runtime lines (verify.sh): 19,832 -> 20,040 (+208: `rt_lg.go`); test lines 4,032 -> 4,240.
**Nothing was deleted**: every erased class is still constructed (erased places, kinds not
built). The requirement "the runtime shrinks" is not met at this point.

### 3.3 Allocation (CHECKS of the 18% subset, one run each, state sha256 1cb35c70ccb88501 in every run)

| tree (binary in `scratchpad/b3/`) | alloc_objects | alloc_space | objects without the protobuf pool functions |
|---|---:|---:|---:|
| start, b2 tip (`../b2/mech_iter`) | 375.2 M | 19.16 GB | 340.7 M |
| makers + views + cov (`mech_AB`) | 365.6 M | 18.44 GB | 337.5 M |
| + coll (`mech_C`; had lost `Collection_add_s`: +2.3 M boxes, fixed in the list commit) | 375.2 M | 19.12 GB | |
| + list, first form (`mech_D`): 7.7 M covariant wrappers of the graph classes | 380.0 M | 19.63 GB | |
| + list, wildcard Lists erased (`mech_D3`) | 361.4 M | 18.69 GB | 334.1 M |
| + typed subList, final (`mech_D4`) | 371.3 M | 19.04 GB | 335.2 M |

The protobuf builders are pooled and their count moves by several million between runs of one
binary (`pbDesc_.newMsg_`, `newBuilder_`, `pbData_`, the two `init.func7xx` they call: 27.3 M in
the D3 run, 36.2 M in the D4 run, 34.5 M in the start run); the last column leaves them out and
is the comparable one: **340.7 M -> 335.2 M (-1.6%)**. D3 -> D4 otherwise differs by -448 MB
(`tlistView_.boxed` under `subList` in `AbstractCommandLineRunner.createJsChunks`: the old
`subList` of a typed list's view boxed the whole list on every call; found by this profile).

The `iter` kind's +3.8 M `iterableAs_` wrappers: **gone, confirmed** (3.42 M in the start
profile; `iterableAs_` is below the profile's listing threshold in D3/D4). They needed three
things, not one: `Scope.vars` typed (`makers`), the narrowing overrides
`Scope.getVarIterable(): Iterable<Var>` in groups of their own (`cov`), and `valuesT()` (the
map is `ImmutableMap.of()` until the first variable is declared, and the erased view of an
immutable typed map is its boxed twin, never native).

Wrappers still allocated in CHECKS (D3 profile): `listAs_` 1.4 M in
`DataFlowAnalysis.analyze` (the local `nextNodes` is initialized by a conditional of two
narrowed results; `tryRefine` does not take a conditional's captured type), `collectionAs_`
about 0.7 M (`TypedScopeCreator.NormalScopeBuilder.visitPostorder` 0.36 M,
`JSDocInfo.getParameterNames` 0.17 M, `TemplateTypeReplacer.forInference` 0.08 M: a `Set` /
`keySet()` or an erased `ImmutableList` given as a `Collection<X>`; kinds `set` and the
class-typed declarations remove them).

### 3.4 Variance (brief item 6): what was done

- Same-bound wildcards are typed at the bound for `Iterable` and `Collection`.
- Narrowing overrides in classes are method groups of their own (`cov`, on rule "covariant
  overrides"): 6 scope methods (`Iterable`), and the graph classes' `getInEdges` /
  `getOutEdges` / `getEdges` / `getDirectedSuccNodes` / `getDirectedPredNodes` (`List`). A caller
  with the narrowing class reads typed, without a wrapper.
- The wide side of the graph family: **stays erased** (`LibGen.wildErased`: a `List<? extends
  X>` / `List<?>` place is the erased `List`). Tried and measured first: typed wide `ListT`
  with a wrapper per call (+7.7 M objects), then the view kept in a field of the object that
  has the bridge (`listUp_`, an atomic pointer per bridge): still 3.7 M allocations, because a
  graph node's edge list is asked for about once per node, and the nodes grew by the fields (+52 MB in `new_..AnnotatedLinkedDiGraphNode`). A form that
  allocates nothing needs the view to be part of the list (a second view struct in every
  `TList`: 48 -> 64 bytes for 22.8 M lists in CHECKS) and was not built. So through
  `DiGraphNode.getInEdges()` the code is what it was before this stage (erased list, cast per
  element, no allocation); `listUp_` was removed again.
- 4 locals of wildcard List type take the narrowed type of their initializer (`tryRefine`).

### 3.5 Semantics, refusals, tests

- Gate lines of the final tree are in 3.7. Self-test `lang/LibGen.java`: `views()`, `colls()`,
  `lists()` (identity, `equals`/`hashCode`, null, `UnsupportedOperationException` on immutables,
  `IndexOutOfBounds`, `NoSuchElement`, live views, iterator `remove`, late
  `ClassCastException` after an unchecked cast, collections of lists, an `AbstractCollection`
  subclass). Runtime tests: `TestLgTypedMapViews`, `TestLgCollection`, `TestLgValuesT`,
  `TestLgList`, `TestLgSubList`.
- Refusals: the raw-use refusal of `iter` covers `Collection` and `List` (LibGen.check; the
  tree has 0). **Not built: the refusal for unchecked casts on library generics** (the two
  known ones behave as in Java: the wrapper checks when an element is read) and a refusal for
  `instanceof` / casts to the classes `ArrayList` etc. (a typed-by-place list is a view, not a
  `*ArrayList`; the README says the tree has neither; nothing checks it).
- Differences from Java seen while testing, all older than this stage and unchanged:
  `Arrays.asList` and `Collections.emptyList` of the runtime are lists that accept `add`;
  `o instanceof Set` is true of a list view.
- ConcurrentModificationException: no test added; the typed iterators are the existing ones.

### 3.6 Left to do, in the order I would take it

(State at the end of stage b3. Items 1, 2, 5 and parts of 4 and 6 were done in stage b4:
section 4; the list that is current is 4.6.)

1. Kind `set` (`SetT[X]`: `Set` + `iterator_t`; `keySetT()` of a typed map; typed-by-place for
   `new LinkedHashSet`, `ImmutableSet.of/copyOf`; `fromTypedColl` for the set kind). Same shape
   as `coll`; about 143 declarations. `LibGen.TYPED_METHODS`, `KINDS`, `rt_lg.go`.
2. Kind `map` + `entry` (`MapT[K,V]` with `get_t` / `values_t` / `keySet_t` / `entrySet_t`,
   `EntryT[K,V]`): about 194 declarations and the nested collections. Then make the element
   type of collections the typed interface (`Ctx.typedArg`: `lg.isLib(ae)` returns the erasure
   today), so `Map<K, List<X>>` is `*TMap[K, ListT[X]]` and `computeIfAbsent(k, k -> new
   ArrayList<>())` is typed by place.
3. Class-typed declarations (`ArrayList<X>`, `ImmutableList<X>`, `LinkedHashMap<K,V>` that
   `Colls` leaves erased: 98) and more typed methods (`add_t`, `set_t`, `remove_t`, `contains_t`).
4. Typed results of the remaining runtime functions (`Arrays.asList`, `Lists.reverse`,
   `Collections.emptyList` / `singletonList`, `Ordering.sortedCopy`, `Sets.union` ...) and
   protobuf lists (`tools/protogen`; 89 origins): only after 1-3, by the wrapper counts of a
   CHECKS profile (`go tool pprof -peek 'listAs_|collectionAs_'`), which is how every decision
   above was made.
5. Refusals (unchecked casts with a name list; casts to the collection classes).
6. Deletion: only when a profile and `grep new_ArrayList` (22 sites left in `g_[A-Z]*.go`) show
   an erased class is no longer constructed.
7. The no-object covariant view (3.4), if the graph casts matter.

Traps met: the translator reads `rt_*.go` of its output directory; `Ctx.erase` of a library
type parameter is a Go type parameter once its interface is a kind (use `lg.rt` where the
runtime's signature is meant: that lost `Collection_add_s`); javac's type of an expression has
wildcards captured (`wildErased`); a typed method whose result goes to `Object` must be the
erased one; the erased interface in `g_zstubs.go` only has the methods some call names, so a
typed call also notes the erased method.

### 3.7 Gate lines

Final tree (code of 6c7533b9), default translation, `$B/gate.sh agZ b3_D4` (log
`/mnt/data/ubuntu/jm/logs/gate-b3_D4.log`, copy in `scratchpad/b3/`):

```
GATE OK b3_D4
selftest: PASS
ok  	jsgomech/closure	(cached)
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20040  test lines: 4240
```

(the test line says "cached": the same test binary had printed `ok jsgomech/closure 1.947s`
in the run just before the gate.) Same tree, every switch of the three stages off
(`J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all`, translated with `-tags j2go_nofntypes`,
`tools/prune/verify.sh`; `scratchpad/b3/verify-alloff.log`):

```
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.973s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20040  test lines: 4240
```

Earlier gates: `b3_AB` (makers, views, cov), `b3_C` (coll), `b3_D3` (list before the subList
change): GATE OK each (logs beside the above). Not gated one by one: each switch of this stage
alone off.

### 3.8 Fifth flip: post-flip steps, judged from the emitted text (none was run)

In addition to LIBGEN-STUDY.md 7 and 8:

- `colls` (3-5, 87-90, 218, 322, 336, 338, 403), `gmaps` (453): they type collections across
  methods where the translator left `List` / `Collection`; 342 of those declarations are now
  `ListT[X]` / `CollectionT[X]`, which they do not know. Expect them to find less and to need
  the new type names as "already typed, through an interface".
- `unify` (402) and the `JT*` rename: the typed family has new methods (`asListT`, `asCollT`,
  `valuesT`, `get_t`, `unwrap_` on the views) and `tlistView_.subList` now returns a typed list;
  its runtime patch touches `rt_tlist.go` / `rt_tmap.go` next to them and will need regenerating.
- `ranges` (170, 202, 226, 227, 404, 454): 350 loops are `eachOf_` cursors (146 before).
- `iters`, `tviews` (479), `tgeneric` (488), `trips` (512): `x.asCollT()` / `x.asListT()` are
  views of the same object; the round trips those steps removed (`asList()` then a cast per
  element) are not written at these sites.
- `castfold`, `casts`, `anyconv`, `objslots`, `icasts`, `gcasts`: fewer sites (`cast_[` -137,
  `icast_` -54), same text.
- `native` / `slices` / `maps` (1, 60-63, 91, 92, 451), `immbuild` (104): unchanged subject
  (L1 collections), +16 declarations; a `ListT[X]` place is not a candidate for a slice.
- `strkeys` (123, 180): `Collection_add_s` etc. unchanged in count.
- New for any tool that follows types by name: `ListT[`, `CollectionT[`, method groups named
  `m_rListOf_X` / `m_rIterableOf_X` / `m_rCollectionOf_X` for narrowing overrides.

## 4. Record of stage b4 (branch `main-libgen-b4`, worktree `/mnt/data/ubuntu/jm/wt/agZ2`, 2026-11-21)

On top of `main-libgen-b3` 4c069875. Scratch files: `/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/b4/`
(`tr.sh` translate + build; `trto.sh <dir>` translate into a scratch directory; `st.sh` self-test
with the first difference; `checks.sh <binary> <label>` CHECKS with an allocation profile;
`pool.sh <alloc dir>` totals without the protobuf pool; `pdiff.py A B [min]` per-function
difference of two `-top` listings; `forms.sh <map dir>` declarations by form; `off.sh` /
`offall.sh` the gates with switches off; `dead.py` / `delete.py` unreferenced runtime functions;
gate logs `gate-b4_*.log`, `verify-*.log`).

**How far: 3.6 items 1 (kind `set`), 2 (kind `map` / `entry`, typed elements), 5 (refusals) are
built, gated and committed; of item 4 the three things the CHECKS profile showed (null checks,
reading library methods, runtime parameter types); item 6 (deletion) only for what a build
shows unreferenced in every translation (14 functions, 52 lines). Item 3 (class-typed declarations, more typed methods), the
protobuf lists and item 7 (wide side of the graph family) are NOT built.** Reasons and designs:
4.6.

### 4.1 Commits

| commit | what | switch (`J2GO_NOLIBGEN=`) |
|---|---|---|
| 185c0254 | kind `set` (`SetT[X]`, `asSetT`, `setAs_`, `keySetT()`); `collectionAs_` / `iterableAs_` find the typed set of an erased immutable set; `CollMap` writes the form L1 / L2 / L3 of each declaration | `set` (`views` for `keySetT`) |
| 8f61c6a8 | kind `map` (`MapT[K,V]`, `Map_EntryT[K,V]`, `asMapT`, `mapAs_`, `map_EntryAs_`, `entrySetT()`); piece `elems` (collections as typed elements: `*TMap[K, ListT[X]]`); piece `wide` (places of `?` / of an erased type variable stay erased); four fixes (below) | `map`, `elems`, `wide` |
| d9aa4aba | refusals (unchecked casts with a name list, collection classes) with a negative self-test; piece `nncoll` (typed null checks of typed collections); the erased view of a typed collection has the erased type | `nncoll` |
| f060be6f | piece `readers` (a typed collection given to a reading library method is its view, not its twin) | `readers` |
| f170a5e8 | each switch of the stage off alone translates and passes (three fixes found by those gates, 4.7) | |
| 62ebd50a | deletion of the 14 runtime functions that no translation names (4.5) | |

Byte comparisons of the generated files (`scratchpad/b4/`): with everything off
(`J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all`, `outOff`) they are identical to the b3 tip's with
everything off (`../b3/outOff`), all 814 files. With `set` off at commit 185c0254 they were
identical to the b3 tip's default translation (`out0b` against `out0`). **With the six switches
of this stage off (`set,map,elems,wide,nncoll,readers`, `outNew6off`) the final translator's
output is not the b3 tip's: 107 of 814 files differ, 274 lines**, because four corrections to
the kinds of the stages before are not behind a switch of their own: the parameters of the
runtime's typed factories and of the String-key helpers have the runtime's type (`LibGen.rt`:
no `collectionAs_(tc_Object, ..)` wrapper per call), the helpers get `X_un_(x)`, `instanceof`
of a typed interface tests the erased one, and the erased view `x.asList()` has the erased
type (no `lgBox_`). Also not behind a switch: the runtime changes (`tmapEmptyImm_`,
`TMap.entryAt`, the new methods).

### 4.2 Counts

Declarations of a collection type (3,867), by form (`J2GO_COLLMAP`, last column of `decls.tsv`;
`forms.sh`):

| state | b3 tip | + set | + map, elems, wide (final) |
|---|---:|---:|---:|
| L1 concrete typed collection | 3,067 | 3,067 | 3,067 |
| L2 typed interface | 342 | 436 | 558 |
| L3 erased | 458 | 364 | 242 |

The 242 erased ones: class-typed with a typed family 133 (`ArrayList` 38, `ImmutableList` 30,
`LinkedHashMap` 20, `ImmutableMap` 20, `ImmutableSet` 11, `LinkedHashSet` 5,
`ImmutableCollection` 5, `TreeMap` 4); a class without a typed family 56 (`EnumSet` 16,
`ImmutableSortedSet` 15, `ConcurrentMap` / `ConcurrentHashMap` 8, `Deque` / `Queue` /
`PriorityQueue` 7, `ImmutableSortedMap` 4, `IdentityHashMap` 2, `SortedSet`, `RangeSet`,
`EnumMap`, `ProtocolStringList`); builders 17; interface-typed 36: `List<? extends X>` 21
(variance, 3.4) and 15 that piece `wide` keeps erased (`?`: `Map<?, ?>` 7, `List<?>` 2,
`Collection<?>` 2, `Map.Entry<?, ?>` 1; an erased type variable: 3), which is the target of
0.4. (Piece `wide` moved 13 declarations from L2 back to L3: 571 -> 558.)

`baseline.sh` (`g_[A-Z]*.go`), b3 tip -> final: `cast_[` 787 -> 723, `icast_X(` 238 -> 195,
`str_(` 196 -> 140, `Node_of_(` 98 -> 91, `obj_(` 557 -> 555, `strobj_(` 396 -> 390, erased views
`.asList()` etc. 873 -> 572, `foreach_(` 272 -> 169, `XOf[` alias uses 485 -> 282, `unboxT_[` 37 -> 25.

Conversions left (`J2GO_LIBGEN=1`, final): `iterableAs_` 70, `iteratorAs_` 7, `collectionAs_` 36, `listAs_` 76, `setAs_` 36, `mapAs_` 28, `map_EntryAs_` 12; uses left erased: 291 of a `List` with a wildcard, 255 of a place of `?`, 422 of a place with an erased type variable; 73 arguments given as views (piece `readers`)
(b3 tip: `iterableAs_` 119, `iteratorAs_` 14, `collectionAs_` 110, `listAs_` 84).
Erased constructors left in the generated code: 37 sites (`new_ConcurrentHashMap` 9,
`new_LinkedHashSet*` 7, `new_LinkedHashMap*` 5, `new_ArrayList_*` 3, `new_EnumMap` 3, ...; b3
tip: 102); `new_ArrayList_0` has none.

Runtime lines (verify.sh): 20,040 -> 20,293 before the deletion -> 20,241 (`rt_lg.go` 434 ->
687; start of the whole rule: 19,832). Test lines 4,240 -> 4,396.

### 4.3 Allocation (CHECKS of the 18% subset, one run each, state sha256 1cb35c70ccb88501 in every run)

| tree (binary in `scratchpad/b4/`) | alloc_objects | alloc_space | objects without the protobuf pool functions |
|---|---:|---:|---:|
| b3 tip (`../b3/mech_D4`) | 371.3 M | 19.04 GB | 335.2 M |
| + set (`mech_set`) | 370.5 M | 18.99 GB | 334.9 M |
| + map, elems, first form (`mech_map0`) | 360.9 M | 18.19 GB | 335.0 M |
| + wide, the four fixes (`mech_map2`) | 370.1 M | 18.96 GB | 334.5 M |
| + refusals, nncoll (`mech_ref`) | 360.9 M | 18.16 GB | 331.9 M |
| + readers (`mech_rd`) | 361.6 M | 18.20 GB | 331.2 M |
| final tree, after the deletion (`mech_del`) | 363.0 M | 18.35 GB | 331.2 M |

Against the start of the rule's collection stages (b2 tip, 340.7 M): **340.7 M -> 331.2 M (-2.8%)**, of which this stage 335.2 M -> 331.2 M (-1.2%). The pool functions
moved between 26 M and 36 M from run to run, so only the last column compares.

What the profile showed, and what was done (the method: `pdiff.py` of two `-top
-nodefraction=0` listings, then `-peek` of the function):

- kind `set`: `collectionAs_` -346 k (a `Set` given as a `Collection`), `setAs_` +201 k
  (`TemplateTypeReplacer.initSeenTypes`: `Sets.newIdentityHashSet()` in a `Set<JSType>` field,
  one wrapper per replacer; left, see 4.6).
- kind `map`, first form: `map_EntryAs_` +364 k (`JSDocInfo.init`: the loop variable over the
  entries of an erased `TreeMap`: fixed, the variable of an erased loop holds the erased entry),
  `mapAs_[any, any]` +134 k and `collectionAs_[any]` / `iterableAs_[any]` 540 k that the kinds
  `iter` and `coll` already had (`Iterable<?>` / `Collection<?>` / `Map<?, ?>` places and the
  parameter of `ImmutableList.copyOf`: fixed by piece `wide` and `LibGen.rt`), `mapAs_` +56 k
  (`Timeline.addEvent`, `<V> .. Map<V, Event<V>>`: piece `wide`), `tmapEmptyImm_` +1.08 M
  (`Scope.getOwnImplicitSlot`: the runtime made an empty map on every call for the third pair
  of types of two conversions: fixed in `rt_timm.go`).
- `TList.asImm` + `boxAll_` 3.0 M + 1.5 M (older than this stage): a typed immutable list read
  by erased code as its twin. `checkNotNull(list)` was 1.8 M of it (piece `nncoll`),
  `Iterables.getLast`, the message arguments of `checkArgument` and `addAll` of an immutable
  list about 0.7 M (piece `readers`).

Wrappers still allocated in CHECKS (`mech_ref`): `iterableAs_` 0.6 M
(`TypedScope.getDeclarativelyUnboundVarsWithoutTypes` 0.39 M: the result of `Iterables.filter`;
`JSDocInfo.TypeMapProperty.getTypeExpressions` 0.16 M; `ObjectType.getImplicitPrototypeChain`
0.04 M: a lambda as an Iterable), `setAs_` 0.18 M (`initSeenTypes`), `collectionAs_` 0.3 M
(the graph classes' `getNodes`, variance), `iteratorAs_` 0.14 M
(`FunctionTypeBuilder.inferParameterTypes`). Twins still made: `TList[string].asImm` 1.4 M with
`boxAll_` 1.3 M, of which `DependencyInfo.getRequiredSymbols` 1.3 M (an `ImmutableList<String>`
result of a default method of an interface: class-typed, 4.6), `JSTypeIterations.mapTypes*`
0.37 M (a type-variable result).

### 4.4 Semantics, refusals, tests

- Self-test `lang/LibGen.java`: `sets()` (seven origins of a `Set`, identity, `equals` /
  `hashCode`, null element, `UnsupportedOperationException`, `NoSuchElementException`, a
  `TreeSet`, live `keySet()` of a map that is immutable at first, keys with `equals`, sets as
  elements, late `ClassCastException`), `maps()` (six origins of a `Map`, entries that write
  through, live views and removal through them, `Map<String, List<String>>` and a map of maps
  of sets with `computeIfAbsent`, pattern `instanceof` of `Map<?, ?>` / `Collection<?>` /
  `Map.Entry<?, ?>` on typed and erased objects, wildcard maps, late `ClassCastException`).
  Runtime tests `TestLgSet`, `TestLgKeySetT`, `TestLgMap`.
- Refusals (`LibGen.check`; the translation stops and writes nothing), each with a use in
  `tools/j2go/selftest/refused/RefusedLib.java` (the self-test fails unless exactly its 6 marked
  uses are refused): a raw use; an unchecked cast to a use of a typed interface, between two
  instantiations and from `Object`, outside the methods of `tools/j2go/unchecked-casts.txt` (the
  2 of the tree: `GlobalNamespace.Name.castRefsForNodeMap`, `JSTypeRegistry.createRecordType`;
  the self-test programs have `selftest/unchecked-casts.txt`); `instanceof` of and a cast to a
  class of `LibGen.CONCRETE` (the tree has none). A cast to `List<?>` passes.
- Found wrong and fixed: `x instanceof Collection<?> c` tested the typed interface since kind
  `coll` (false for an erased `ArrayList`; one site, `LogsGson`, which no gate input reaches);
  now the erased interface. The String-key helpers (`Collection_add_s`, `Map_get_s`) were given
  wrappers since kind `coll` and took their boxing path; they get `X_un_(x)`.
- Differences from Java seen while testing, older than this stage and unchanged:
  `Collections.unmodifiableSet(x)` of the runtime is `x` (it accepts `add`); a box of the wrong
  class read through a wrapper panics in `unbox_` with a Go error where Java throws
  `ClassCastException` (`Map<String, Integer>` holding a String; a class element throws the
  Java exception); the runtime has no `Collections.emptySet`, `Sets.newHashSet`, 4-argument
  `ImmutableMap.of`.
- Iteration order: every typed form reads the typed collection's own storage (insertion order,
  sorted order for the sorted ones), the wrappers the erased collection's iterator; no order
  is computed anew. ConcurrentModificationException: no test added (as 3.5).

### 4.5 Deletion

No `deadcode` on this host (`tools/prune/prune.sh` needs it; installing it was not permitted),
so the candidates are the top-level runtime functions whose name occurs nowhere but in their
own declaration: not in the generated code, the remaining runtime, the self-test's translation
(`scratchpad/b4/dead.py`, iterated to a fixpoint), computed for the default translation and for
the translation with every switch off (`J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all`), each with its
self-test.

- **Deleted (commit 62ebd50a): the 14 functions that neither translation names and the
  translator's usage data does not list** (43 lines of code, 52 with their comments):
  `ImmutableList_of_Object_Object_Object`, the 13-argument `ImmutableList_of`,
  `ImmutableList_builderWithExpectedSize`, `ImmutableSet_of` with 4 and 5 arguments,
  `ImmutableSet_builderWithExpectedSize`, `ImmutableSet_copyOf_Iterable`, the 4-pair
  `ImmutableMap_of`, `new_TreeSet_Comparator`, `new_TreeMap_Comparator`, `new_ArrayDeque_int`,
  `SequencedCollection_addFirst` / `removeFirst`, `equalsH_`. Runtime lines 20,293 -> 20,241.
  Kept although no generated call names them: 5 that the translator notes as used and would
  write a placeholder for (`CharMatcher_anyOf`, `Objects_requireNonNull_Object`,
  `Pattern_matches`, `Preconditions_checkNotNull_Object_String_Object_Object`, `String_join`:
  a first deletion of them changed `g_zstubs.go`), and 2 with a runtime test
  (`new_ArrayDeque_Collection`, the 3-pair `ImmutableMap_of`).
- **Not deleted: 13 erased functions that only the translation with switches off names**
  (about 35 lines): `ImmutableList_of` with 5 to 11 arguments, `ImmutableList_copyOf_ObjectArr`,
  `Collections_unmodifiableCollection`, `Collections_unmodifiableSet`,
  `Maps_newLinkedHashMapWithExpectedSize`, `Sets_newLinkedHashSetWithExpectedSize`,
  `new_TreeSet`. The typed forms have replaced them in the default translation; deleting them
  ends `J2GO_NOLIBGEN=all` (and `place` / `makers` / `views` off), and every piece is to stay
  behind its switch. They go when the switches go.
- No erased class is dead: `TreeSet` (no site of generated code makes one any more) is still
  made by the runtime (`rt_tsorted.go`: `descendingSet`, `tailSet`; the multimaps).

Why the runtime does not shrink below its start (19,832; it is 20,241): the erased classes are
all still constructed, by the runtime itself (`Lists.newArrayList`, `Arrays.asList`, the
protobuf lists, the immutable twins, `LinkedHashSet` under `Sets.*`) and by 37 sites of
generated code in places that stay erased; and the typed interfaces added 687 lines
(`rt_lg.go`) for which no erased code can go while the erased interface is what the runtime's
own functions take and give. What would let whole classes go: the class-typed declarations
and typed runtime results (4.6), after which `ArrayList` / `LinkedHashMap` / `LinkedHashSet`
are made only inside the runtime, and then the switches' end.

### 4.6 Left to do, in the order I would take it

1. **Class-typed declarations** (139: `ArrayList<X>`, `ImmutableList<X>`, `LinkedHashMap<K,V>`,
   `ImmutableMap<K,V>`, ...; 3.6 item 3). Not built. Most are erased because their method is no
   candidate of `Colls` (a method of a record / AutoValue class 16, lambdas 8, another
   instantiation 7). The largest cost seen is `DependencyInfo.getRequiredSymbols():
   ImmutableList<String>` (2.6 M objects in CHECKS: the typed list is turned into its twin for
   the result and found again by the caller). Design: a kind whose places hold the kind's
   interface (`ImmutableList<X>` -> `ListT[X]`, `ArrayList<X>` -> `ListT[X]`), the class's own
   methods called on the erased object (`lgBox_`: the twin of an immutable collection, as rule
   "typed collections" does); it needs `LibGen` to tell the place's Go type (`typedName`) from
   the erased form (a class, not an interface: `X_un_` and `rt` assume an interface), and
   `LibGen.CONCRETE` already refuses the tests that would see the difference. Then more typed
   methods (`TYPED_METHODS`: `add_t`, `contains_t`, `put_t`), which only then let erased methods
   go.
2. **Typed runtime results**, by the profile (4.3, last paragraph): `Iterables.filter` (0.39 M
   wrappers), `Sets.newIdentityHashSet` (a typed set compared by `==`; only for a class element),
   the protobuf lists (not in the CHECKS profile at all: `getXList` is read by erased loops).
3. Piece `readers` reads a name list (`LibGen.READERS`, 12 methods); other reading methods
   (`Joiner.join`, `Iterables.concat`, `String.valueOf`) can be added as the profile shows them.
4. The wide side of the graph family (3.4): not touched.

Traps met in this stage: names of Go types are given in the order of their first use, so a
rule must not ask for a Go type (`goTypeArg`, `goType`) while deciding a type in `erase` (it
renamed `Node` to `Node_`); a type that is all `any` (`Collection<Object>`) cannot be told from
a member of a Go generic class at that instantiation, so `wide` is decided on the type as
written (`?`, an erased type variable) only; the runtime's helper functions tell their argument
by its Go type, so a typed value must reach them as `X_un_(x)`; `tools/j2go/selftest/run.sh` is
read by bash while it runs (do not edit it during a gate); the mech-cache copied by `mkwt.sh`
has a stale `.cache/j2go/files.txt` (regenerate it as `tools/j2go/run.sh` does); one compile of
the package takes about 10 GB, so two gates at once on this host (24 GB; another agent's
replays compile too) stop making progress: `waitmem.sh` waits for memory before each; the
translator writes a placeholder in `g_zstubs.go` for a runtime function it notes as used even
where no call of it is emitted, so "unreferenced" has to be checked against the placeholders
too (4.5); a switch that is only ever gated together with the others hides mixed states (set
off with map on did not compile, wide off refused the tree: 4.7).

### 4.7 Gate lines

Final tree (code of 62ebd50a), default translation, `$B/gate.sh agZ2 b4_del` (log
`/mnt/data/ubuntu/jm/logs/gate-b4_del.log`, copy in `scratchpad/b4/`): `GATE OK b4_del`

```
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	(cached)
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20241  test lines: 4396
```

(the test line says "cached": the same command had printed `ok  jsgomech/closure 1.981s` just
before the gate.) Same tree, every switch of the three stages off (translated with the
switches, `-tags j2go_nofntypes`, `tools/prune/verify.sh`; `scratchpad/b4/verify-alloff2.log`):

```
switches: J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	2.348s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20241  test lines: 4396
```

Each switch of this stage off alone, on the tree before the deletion
(`scratchpad/b4/verify-<switch>.log`; `set` and `wide`: code of f170a5e8; `map`, `elems`,
`nncoll`, `readers`: code of f060be6f, not run again after f170a5e8, which changed two self-test
programs and two places of the translator that act only with `set` or `wide` off; the generated
files of the default translation are byte-identical from f060be6f to the final tree, md5 in
`default.md5`):

```
switches: J2GO_NOLIBGEN=set
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.910s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20293  test lines: 4396
switches: J2GO_NOLIBGEN=map
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.907s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20293  test lines: 4396
switches: J2GO_NOLIBGEN=elems
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.934s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20293  test lines: 4396
switches: J2GO_NOLIBGEN=wide
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.924s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20293  test lines: 4396
switches: J2GO_NOLIBGEN=nncoll
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	2.001s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20293  test lines: 4396
switches: J2GO_NOLIBGEN=readers
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.932s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20293  test lines: 4396
```

`map`, `elems`, `nncoll`, `readers` passed on the first run (code of f060be6f). `set` and `wide`
did not, and the first run with everything off failed its self-test; what was wrong and the
fixes are commit f170a5e8: with `set` off and `map` on the self-test's translation did not
compile (`MapT.keySet_t` gives `SetT`); with `wide` off the translator refused the tree (three
casts to `Collection<?>` / `Map<?, ?>` / `Iterable<?>` counted as unchecked); with everything
off the new self-test programs called factories the erased runtime does not have. The three
runs above are of the fixed tree. Not gated: the switches of the earlier stages (`iter`,
`coll`, `list`, `makers`, `views`, `place`, `cov`) off one by one; combinations of switches.

Earlier gates of this stage, GATE OK each (logs in `scratchpad/b4/`): `b4_set` (185c0254),
`b4_map2` (8f61c6a8), `b4_ref` (d9aa4aba; its first run failed because `selftest/run.sh` was
edited while it ran), `b4_rd` (f060be6f), `b4_final` (f170a5e8).

### 4.8 Fifth flip: additions to 3.8

- New type names for any tool that follows types by name: `SetT[`, `MapT[`, `Map_EntryT[`;
  `*TMap[K, ListT[X]]` and the like (a typed interface as the element of a typed collection:
  `gmaps`, `native` / `slices` / `maps` must not take a `ListT` element for an erased `List`).
- New methods on the typed family: `asSetT`, `asMapT`, `asEntryT`, `keySetT`, `entrySetT`,
  `get_t` / `keySet_t` / `values_t` / `entrySet_t` on `tmapView_`, `getKey_t` / `getValue_t` on
  `tentryView_`, `entryAt`; `unify` (402) and the view patches (554a/b, 556) touch the same files.
- `Preconditions_checkNotNull_T(x)` on typed collections and `x.asListT()` arguments of the
  reading library methods replace `x.asList()`: `trips` / `tviews` find fewer round trips.
- `casts`, `icasts`, `castfold`: fewer sites (counts in 4.2), same text.

## 5. Record of stage b5 (branch `main-libgen-b5`, worktree `/mnt/data/ubuntu/jm/wt/agZ3`, 2026-11-21)

On top of `main-libgen-b4` 4f24b46f. Scratch files: `/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/b5/`
(`jc.sh` rebuild the translator; `tx.sh [VAR=val ..]` translate into `go/closure`; `gb.sh` go
build; `st.sh` self-test with the first difference; `pipe.sh <label>` build + runtime tests +
self-test + gate + CHECKS profile under one hold of the lock; `off.sh` / `offall.sh` the gates
with switches off; `sttr.sh <dir>` translate the tree and the self-test programs into a scratch
directory without building; `dead.py`, `move.py` the off-mode-only functions; `checks.sh`,
`pool.sh`, `pdiff.py` as in b4; reports `map0` (start) .. `map3` (final), gate logs).

**Every heavy command of this stage ran under `flock /mnt/data/ubuntu/jm/heavy.lock`** (the
host is shared with another agent's replays; one compile of the package takes about 10 GB). The
other agent holds the lock for whole replays (up to an hour), which is what bounded this
stage's turnaround.

**How far: 4.6 item 1 (class-typed declarations, piece `cls`), more typed methods (piece
`meth`) and the split of the off-mode-only runtime functions into a build-tagged file are
built, gated and committed. A typed `Sets.newIdentityHashSet` (piece `idset`) was built, gated,
measured as a loss and removed again (5.4). Builders, the classes without a typed family, typed
`Iterables.filter` and the wide side of the graph family are NOT built** (5.7 says what each
needs).

### 5.1 Commits

| commit | what | switch (`J2GO_NOLIBGEN=`) |
|---|---|---|
| 899d2cfd | pieces `cls`, `meth`, `idset` with their self-tests (`LibGen.classes()`, `methods()`, the 7th refused use) and runtime tests; `rt_lg_erased.go` and the two line counts of `verify.sh` | `cls`, `meth`, `idset` |
| 2ccac50a | piece `idset` removed again (5.4) | |
| fb9ccbc4 | self-test: `LibGen.classes()` valid with `cls` off (found by the gate with everything off, 5.8) | |
| (the commit after) | this section | |

The three pieces are one commit because they were gated as one tree (the lock, see above); a
first form of `cls` alone was gated before (`GATE OK b5_cls`, log in the scratch directory),
then changed by what its profile showed (5.2). With `cls,meth,idset` off the generated files
are byte-identical to the b4 tip's default translation (md5 of `cat g_*.go`:
`cdf1456c9462563279164d57560c180a`, checked after every change of the translator). Not behind
a switch: the runtime changes (`LinkedHashMap.typed_`, the kept views of `mapAs`, the new
methods, `tlistView_.trimToSize`), and the four corrections of stage b4 that 4.1 lists (they
repair wrong or slow translations of the kinds before: a wrapper given to a String-key helper,
`instanceof` of the typed interface; putting an error behind a switch was judged pointless, so
with every switch of stages b4 and b5 off the translator's output differs from the b3 tip's by
exactly the 274 lines in 107 files that 4.1 describes).

### 5.2 Piece `cls`: class-typed declarations (`J2GO_NOLIBGEN=cls`)

The 133 declarations of 4.2 whose type is a collection class with a typed family. Form (as
designed in 4.6, README "Piece `cls`"): `Ctx.erase` of a use of a class of `LibGen.CLASSES`
(`ArrayList`, `LinkedHashSet`, `HashSet`, `LinkedHashMap`, `HashMap`, `ImmutableList`,
`ImmutableSet`, `ImmutableCollection`, `ImmutableMap`) with type arguments is the typed
interface of its collection interface (`LibGen.cls`), where the interface's use is typed and
the typed family can hold the elements (`Ctx.typedArgs`). Everything else follows from the
existing kinds: typed by place, `fromTypedColl`, `listAs_`. What had to be added:

- `LibGen.rt` (the runtime's signatures) and `LibGen.made` (the type of `new C<>()` and of the
  results of runtime functions) keep the class: `rtDepth` switches `cls` off while a signature
  is erased.
- Methods that only the class has: `LibGen.served` tells them; the receiver becomes the erased
  object (`cast_[*ImmutableList](lgBox_(x))`, the twin). 3 sites in the tree
  (`ImmutableList.reverse` 2, `ImmutableCollection.asList` 1). `SequencedCollection.getFirst`
  on a `LinkedHashSet` (1 site) goes to the runtime's helper as any erased argument.
- A class of `LibGen.CONCRETE` (mutable): a collection made for a typed place of it is the
  typed collection's view, no `*ArrayList`. So (a) a wide use (`ArrayList<?>`) is the erased
  interface `List`, never the class; (b) of the methods such a class declares itself only
  `LibGen.OWN` (`ArrayList.trimToSize`, which the view has: `rt_lg.go`) is translated, through
  `cast_[interface{ trimToSize() }](lgBox_(x))`; any other is refused (new refusal, 7th use of
  `RefusedLib.java`; the tree has none); (c) casts to and `instanceof` of these classes were
  refused already.
- An immutable class place entering an L1 declaration: `tlistOfT_` / `tsetOfT_` / `tmapOfT_`
  (the typed collection whose view the value is; before: `tlistImm_(cast_(lgBox_(x)))`, which
  made the twin to find the typed collection again). 40 sites.
- Casts to a class of `CLASSES` cast to the class first, then convert (`ClassCastException`
  kept).
- `Ctx.typedArg`: an element whose type is such a class is held at its typed interface (as an
  interface element, piece `elems`).
- `Ctx.declType`: no alias `ListOf[..]` for a typed place.
- **Trap met again (4.6): no Go type may be named while `erase` decides.** `cls` asks
  `typedArgs`, whose library-generic branch calls `goType`; `LibGen.deciding` makes that branch
  answer "cannot hold" instead (the first build renamed `Node` to `Node_`). A cache of `cls`
  keyed by the type's text was wrong too (asked before the collection kinds are registered;
  two type variables named `E`): there is none now, the translation time is unchanged (10 s).

What the CHECKS profile showed for the first form, and what was done:

- `mapAs_` +759 k, with `setAs_` +164 k and `collectionAs_` +171 k under it: `JSDocInfo`'s
  parameter maps. They are made by `JSDocInfo.Builder.putPropEntry`, `<K, V> ..
  getPropWithDefault(prop, LinkedHashMap::new)`: K and V are erased type variables there, so the
  map is an erased `LinkedHashMap`, and every read at a `LinkedHashMap<String,
  JSTypeExpression>` place (`getParameterType`, once per lookup) made a wrapper. Fixed in the
  runtime, generally: **an erased `LinkedHashMap` keeps the wrapper made for it**
  (`LinkedHashMap.typed_`, an atomic pointer, 8 bytes per map; checked against the map and the
  conversions, so a copied struct or another instantiation gets its own), and the wrapper keeps
  its typed key set and values. And in the translator: **a new collection of a mutable class
  entering an `Object` / erased type variable is made typed by its own type** (8 sites:
  `TypeMapProperty.clone`, `ArrayList::new` for `Supplier<T>` at `T = List<String>`, ...), so
  the readers get a native value.
- `LinkedHashSet<DiGraphNode<N, E>>` (`FixedPointGraphTraversal.workSet`, 146 k sets in CHECKS)
  is now a typed set made by place: +1.26 M objects against -0.31 M (numbers in 5.5). An
  element type that is an interface is compared by `equals` / `hashCode`, for which the typed
  set keeps `teq_`; this is what every L1 set of an interface costs, not new to the piece, but
  this declaration is hot. Left as it is: the remedy is in `rt_tset.go` (5.5, 5.7).
- gone as intended: `TList[string].asImm` 257 k -> 11 k, `boxAll_[string]` 298 k -> 65 k (the
  `DependencyInfo` results; the 1.4 M of 4.3 was measured before piece `readers`, which had
  taken most of it), `iterableAs_` 602 k -> 424 k.

Two declarations went from L1 to L2: `UnionFind.allEquivalenceClasses` and its implementation
(`ImmutableList<ImmutableSet<E>>`): the element is now the typed `SetT[E]`, whose type differs
between the interface's `E` and the class's, so the group "does not have the same typed
collection type" for `Colls`; the result is `ListT[SetT[E]]`.

### 5.3 Piece `meth`: more typed methods (`J2GO_NOLIBGEN=meth`)

Report first (`J2GO_LIBGEN=1`, new lines "erased methods on a typed receiver"): of 440 calls of
an erased method of a typed interface on a typed receiver, 371 box nothing and give no element
(`size`, `isEmpty`, `addAll`, `contains` of a pointer, ...); 69 box an argument (a String, a
value of a type parameter; a handle's box is not an allocation) or give an element as `Object`.
Typed (`LibGen.TYPED_METHODS2`): `Collection.add`, `List.set`, `List.remove(int)`, `Map.put`,
`Map.remove`, `Map.Entry.setValue`: `add_t` 65 calls, `put_t` 30, `remove_t` 7, `set_t` 1,
`setValue_t` 1 in the generated code. 27 remain: calls whose result enters an `Object` (the
erased method on purpose, 3.4) and `contains` / `containsKey` / `remove(Object)` / `indexOf`
with a value of a type parameter (their parameter is `Object` in Java; a typed variant would be
a second method per interface that applies only when the argument has the element type: not
built). A String element goes to `add_t` now, not to the helper `Collection_add_s`.
Runtime: 9 methods on the typed interfaces, 31 lines of implementations on the wrappers and
views.

### 5.4 The remaining conversions of 4.3; piece `idset`, built and removed

- `Sets.newIdentityHashSet()` (0.18 M `setAs_` wrappers in `TemplateTypeReplacer`): **built as
  piece `idset`, gated (commit 899d2cfd, `GATE OK b5_meth`), measured, removed** (the commit
  after). Form: for a set of a class or interface of the program `newTSet_(tc_X)`, the typed
  set that compares with Go's `==` (an entry of `Ctx.TYPED_MAKERS`, the conversion chosen in
  `ExprEmitter.immutableOf`); `TemplateTypeReplacer.seenTypes` became an L1 declaration. The
  profile: the wrapper went (`setAs_` -0.18 M, `new_LinkedHashSet` / `htable_.insertPlain`
  -0.42 M), but the typed set allocated more than the erased one: `TSet[*JSType].add0` +0.65 M
  (the key slice, each growth step, and the index, a Go map, above 8 elements), `delAt` +0.19 M
  (the deletion marks; the replacer adds and removes a type per visit), `newTSet_` +0.17 M:
  **+0.4 M objects and +32 MB in all**. The erased set with its one wrapper is the cheaper
  form for this use, so the piece went. What would make it a gain: a typed set whose index is
  not a Go map (the erased `htable_`'s open addressing at the element type).
- `Iterables.filter` (0.39 M `iterableAs_`): **not built**. A typed result needs a typed
  filtering Iterable and its iterator in the runtime (about 30 lines) for one allocation of the
  four the call makes; judged against "minimal runtime".
- the graph classes' `getNodes` (0.3 M `collectionAs_`): variance, 3.4; not touched.

### 5.5 Counts

Declarations of a collection type (3,867), by form (`forms.sh`, `map0` -> `map3`):

| state | b4 tip | b5 |
|---|---:|---:|
| L1 concrete typed collection | 3,067 | 3,065 |
| L2 typed interface | 558 | 670 |
| L3 erased | 242 | 132 |

The 132: class-typed that keep the class 19 (`ImmutableMap` with a key that has type arguments
or is `Object` 11, `ImmutableList<String[]>` 1, an erased type variable 4, `ArrayList<LocalClass>`
1, `ImmutableSet<ImmutableSet<String>>` 1, `ImmutableList<? extends JSType>` 1) and 2
`ArrayList<?>` (now the erased interface `List`); a class without a typed family 56 (4.2);
builders 17; `TreeMap` 4 (`TreeMap<Property<?>, Object>`: the key has type arguments, and the
class has methods of `NavigableMap`; not in `CLASSES`); interface-typed 36 (`List<? extends X>`
21, and the 15 of piece `wide`).

`baseline.sh` (`g_[A-Z]*.go`), b4 tip -> b5: `cast_[` 723 -> 675, `icast_X(` 195 -> 202,
`str_(` 140 -> 127, `Node_of_(` 91 -> 79, `obj_(` 555 -> 481, `strobj_(` 390 -> 379, `boxT_(`
101 -> 90, erased views `.asList()` etc. 572 -> 539, `foreach_(` 169 -> 121, `XOf[` alias uses
282 -> 185.

Conversions left (`J2GO_LIBGEN=1`): `iterableAs_` 70 -> 56, `iteratorAs_` 7 -> 4,
`collectionAs_` 36 -> 30, `listAs_` 76 -> 78, `setAs_` 36 -> 40, `mapAs_` 28 -> 48,
`map_EntryAs_` 12 (the map and list sites that grew are reads of an `Object` or of a
type-variable result at a class-typed place, which were `cast_[*LinkedHashMap]` before; whether
one allocates is decided at run time, 5.2). The counts of "places left erased" are not
comparable with 4.2: `cls` asks `erase` for the interface of every class use.
Erased constructors left in the generated code: 30 sites (b4: 37).

Runtime lines (verify.sh): 20,241 -> 20,361 in all; **of the default build 20,066 -> 20,117**
(b4's 20,241 had the 175 lines of `rt_fn_erased.go` in it), off-mode extras (build-tagged
files) 175 -> 244 (5.6). `rt_lg.go` 687 -> 824: the typed methods 45 lines, `tlistOfT_` and its
like 27, the kept wrapper of a `LinkedHashMap` and its views 38, comments. Test lines 4,396 ->
4,524 (`TestLgCls`, `TestLgMapWrapperKept`, `TestLgMeth`).

Allocation (CHECKS of the 18% subset, one run each, state sha256 1cb35c70ccb88501 in every run):

| tree (binary in `scratchpad/b5/`) | alloc_objects | alloc_space | objects without the protobuf pool functions |
|---|---:|---:|---:|
| b4 tip (`../b4/mech_del`) | 363.0 M | 18.35 GB | 331.2 M |
| + cls, first form (`mech_cls`) | 362.9 M | 18.31 GB | 331.1 M |
| + cls with the kept wrapper and typed-by-own-type, meth, idset (`mech_b5_meth`) | 361.3 M | 18.22 GB | 330.8 M |
| the same without idset (`mech_b5_fin`, final) | 358.3 M | 17.98 GB | 330.2 M |

Against the start of the rule's collection stages (b2 tip, 340.7 M): 340.7 M -> 330.2 M (-3.1%);
this stage 331.2 M -> 330.2 M (-0.3%). By function, b4 tip -> final (`pdiff.py` of two `-top
-nodefraction=0` listings): `strobj_` -1.27 M (Strings given to `add_t` / `put_t` at their
type), `TList[string].asImm` -0.25 M and `boxAll_[string]` -0.24 M (the `DependencyInfo`
results), `iterableAs_` -0.17 M; against them the graph work set as a typed set +1.26 M (`TSet.add0` and
`delAt` 0.51 M, `teq_.add` 0.37 M, `teq_.index` 0.24 M, `newTSet_` 0.15 M) for -0.31 M of
`htable_.insertPlain` / `new_LinkedHashSet` (5.2), and the wrappers of `JSDocInfo`'s maps +0.5 M (`mapAs_` 0.17 M: the wrapper and its
atomic box, once per map; the typed key set and values 0.33 M, once per wrapper; before `cls`
these reads were casts without an object, in the first form of `cls` they were 1.09 M).
**So piece `cls` by itself raises the object count: +0.5 M at `JSDocInfo`'s maps and +0.95 M at
the graph work set, against -0.5 M of twins and -0.2 M of conversions (about +0.8 M, +0.25%);
piece `meth` (-1.2 M boxes of Strings) is the gain of the stage, and the two together are
331.2 M -> 330.2 M.** Both raises are properties of the runtime, not of the rule's form: an
erased map made at type variables needs one wrapper, and the typed set compares elements of an
interface type through `teq_` (a slice of hash codes, a slice of chains and a Go map per set),
which allocates about 8 objects per small set where the erased `htable_` allocates 2. `teq_`
already cost 0.88 M objects in the b4 tip (`teq_.add`, the L1 sets and maps of interface
elements); a `teq_` with one allocation and open addressing would repay both. Not done (5.7).

### 5.6 The runtime's size: the default build and the off-mode extras

The requirement "minimal runtime" is about what the default translation needs. The erased
forms that only a translation with a switch off names must stay as long as the switches do
(every piece behind its switch), but they are not the default build's runtime. **Proposal,
done for the part that is mechanical:**

1. Off-mode-only code lives in build-tagged files, as `rt_fn_erased.go` (tag `j2go_nofntypes`)
   does for rule "function types": **`go/closure/rt_lg_erased.go`, tag `j2go_nolibgen`**.
   `tools/prune/verify.sh` and `tools/j2go/selftest/run.sh` pass `-tags j2go_nolibgen` whenever
   `J2GO_NOLIBGEN` is set (any value), next to `j2go_nofntypes` for `J2GO_NOFNTYPES=1`. (The
   older debugging switches `J2GO_COLLS_NO*` and `J2GO_ERASED=1` name erased factories too; a
   translation with one of them is built with the tag by hand. Not gated in this stage.)
2. What goes there is found, not judged: the top-level runtime functions that the default
   translation of the tree, the default translation of the self-test, the runtime outside
   their own declarations and the unit tests do not name, while the translation with every
   switch off does (`scratchpad/b5/sttr.sh` translates both without building;
   `dead.py <tree> <selftest>` lists the unnamed functions of each; `move.py` moves the
   difference with the comments above each function). 15 functions, 43 lines were moved:
   `ImmutableList_of` with 5 to 11 arguments, `ImmutableList_copyOf_ObjectArr`, the
   varargs `ImmutableSet_of`, `Collections_unmodifiableCollection` / `Set`,
   `Maps_newLinkedHashMapWithExpectedSize`, `Sets_newLinkedHashSetWithExpectedSize`,
   `new_ArrayList_int`, `to_ArrayList`.
   (`new_TreeSet` is off-mode-only too but has a unit test, and stayed.)
3. `verify.sh` prints both numbers: `runtime lines of the default build: N  off-mode extras
   (build-tagged files): M` (files whose first line is `//go:build j2go...`; `rt_fn.go`, tagged
   `!j2go_nofntypes`, is the default build's). The old line `runtime lines:` (the sum) stays,
   because `gate.sh` and the earlier records use it.
4. **Not done, and what it needs:** the function-level search does not see methods, so the
   erased *methods* and types that only off-mode code reaches (methods of `*ArrayList` that
   only erased call sites use, the wrappers' erased paths) are still in the default build. A
   tool that knows reachability is needed (`deadcode` is not installed on this host and
   installing it was not permitted, 4.5). With it: run it on `cmd/mech` built from the default
   translation and from the all-off translation (and each with the self-test's `cmd/t`), move
   what only the second reaches. Methods can move to a tagged file like functions, as long as
   no interface of the default build needs them (the erased interfaces of `g_zstubs.go` only
   have the methods some call names, so the compiler says so).

Numbers (verify.sh of the final tree): see 5.8.

### 5.7 Left to do, in the order I would take it

1. (Done in this stage: every switch gated off alone, 5.8. `scratchpad/b5/offall.sh [switch
   ...]` repeats it, about 6 minutes a switch when the lock is free; run it again after any
   change of the translator or of the self-test programs.)
2. **Builders (17 erased).** Not built. By reason: the sorted immutables' builders 9
   (`ImmutableSortedSet.Builder` 5, `ImmutableSortedMap.Builder` 4: no typed family, item 3); a
   type argument the typed family cannot hold 3 (`Object` key, `String[]`, a key with type
   arguments: stay); 5 that `Colls` leaves erased although the typed builder could hold the
   elements: `builder::add` / `builder::put` as a method reference 3
   (`InvocationTemplateTypeMatcher.copyMatchedTypes`, `ColorPool.Builder.build`,
   `JSTypeExpression.getAllTypeNodes`), a lambda's parameter 1 (`DefaultPassConfig`), "reaches a
   place of type void" 1 (`JSTypeExpression.getAllTypeNames`: the builder's `add` is the body of
   a lambda).
   A builder has no interface to be typed "by place" through: its typed form is the concrete
   `*TListBuilder[X]`, which needs the proof of `Colls` that every use is `add` / `addAll` /
   `put` / `putAll` / `build`. So these 5 are work in `Colls` (a method reference to `add` as
   a supported use; a lambda's parameter as a declaration of the rule), not in `LibGen`.
3. **Classes without a typed family (56).** What each needs:
   - `EnumSet` 16 (`FeatureSet`, `ExpressionDecomposer`, `Node`): an entry of `CLASSES` (to
     `Set`) and typed makers for `EnumSet.noneOf` / `of` / `copyOf` / `allOf` / `complementOf`
     and `clone()`: a typed set sorted by ordinal (`rt_tsorted.go` has the sorted mode; the
     comparator by ordinal and the five factories are new, about 40 runtime lines). Without
     the makers every `EnumSet` entering a typed place would be wrapped.
   - `ImmutableSortedSet` 15 (+ 5 builders) and `ImmutableSortedMap` 4 (+ 4 builders): entries
     of `CLASSES`; the typed sorted immutable exists (`tsetImmNew_` of an `icSorted_` twin), but
     `copyOf` / `of` / `naturalOrder().build()` / `orderedBy(..)` are not typed makers, and
     `first()` / `headSet()` / `descendingSet()` would be calls on the twin.
   - `ConcurrentMap` / `ConcurrentHashMap` 8: the runtime's `ConcurrentHashMap` is a
     `LinkedHashMap` without a lock, so `MapT` with a typed map made by place would behave the
     same; `ConcurrentMap` is an interface and needs to be a class of `CLASSES` that is an
     interface, or a kind of its own. Not done because nothing here tests concurrent use.
   - `Deque` / `Queue` / `PriorityQueue` 7: a kind `deque` (`DequeT[T]` with typed `peek` /
     `poll` / `pop` / `push`): the size of kind `list`. `PriorityQueue` has no typed family.
   - `IdentityHashMap` 2, `EnumMap` 1, `RangeSet` 1, `SortedSet` 1, `ProtocolStringList` 1
     (protobuf lists), `Map.Entry<?, ?>` 1: singles; stay.
4. **`teq_` (`rt_tset.go`): the typed sets and maps whose elements are compared by `equals`**
   allocate a slice of hash codes, a slice of chains and a Go map (above 8 elements) per
   collection, growing each: 1.25 M `teq_.add` + 0.45 M `teq_.index` objects in CHECKS, of which
   the graph work set that piece `cls` typed is 0.6 M. One allocation for both slices and open
   addressing (as the erased `htable_` has) would make the typed set as cheap as the erased
   one; it would also make piece `idset` (5.4) a gain.
5. **Typed variants of the `Object`-parameter methods** (`contains`, `containsKey`,
   `remove(Object)`, `indexOf`: 15 sites that box a value of a type parameter), `Iterables.filter`
   (5.4), reading library methods of piece `readers` as the profile shows them.
6. **The wide side of the graph family** (3.4), the protobuf lists: not touched.
7. **Off-mode methods out of the default build** (5.6 item 4).

Traps met in this stage (in addition to 4.6): `pkill -f <script>` from a shell whose own command
line contains the script's name kills that shell; the other agent holds the heavy lock for
whole replays, so queue work as one script that takes the lock once (`pipe.sh`), and keep the
tree untouched while a queued script waits (it reads `.cache/j2go`, the runtime files and the
self-test programs when it gets the lock, not when it was started); `dead.py` reads the
build-tagged files too (it lists `rt_fn_erased.go`'s functions as off-mode-only: leave them out);
a class place under `elems` off must hold the erased interface for a class of `CONCRETE`
(`Ctx.typedArg`); the report counts of "places left erased" count calls of `erase`, not places; a self-test program must also be
valid with its piece off: with `cls` off rule "typed collections" types `ArrayList<String> al =
new ArrayList<>()`, and a read of the same object through a type variable (`<T> T prop(k)`, an
unchecked cast that `unchecked-casts.txt` lets pass) into a declaration of the class casts the
typed list's view to `*ArrayList` and fails, an old limit of that rule which `cls` removes; the
program makes such objects where only the type variable is declared (`propOr(k,
ArrayList::new)`).

### 5.8 Gate lines

Final tree (the code of 2ccac50a with the self-test program of the commit after it; the generated
files are byte-identical to those gated as `b5_fin`), default translation, `$B/gate.sh agZ3
b5_final` under the lock, run after the sixteen runs below (log
`/mnt/data/ubuntu/jm/logs/gate-b5_final.log`, copy in `scratchpad/b5/`): `GATE OK b5_final`

```
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.916s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
```

Earlier gates of this stage, GATE OK each: `b5_fin` (2ccac50a, before the self-test program
changed; the CHECKS profile of 5.5 is of its binary), `b5_cls` (first form of `cls`, not committed by itself), `b5_meth` (899d2cfd; its first run
failed to compile: the runtime's wrapper called `List.remove`, which the erased interface of
that translation does not have, because no call names it; the wrappers now call the erased
object through the interface of the one method).

Switches off (`scratchpad/b5/offall.sh`: translate with the switch, build with `-tags
j2go_nolibgen`, `tools/prune/verify.sh`, each under the lock; logs `scratchpad/b5/verify-<switch>.log`):

All sixteen runs are of one tree: the code of 2ccac50a with the self-test program of the commit
after it (the first run with everything off failed its self-test: `LibGen.classes()` read a
list that rule "typed collections" had typed back through a type variable into a declaration
of the class, which only works with `cls` on; the program was changed, 5.7 traps, and the suite
started again). **Every switch of the three stages was gated off alone this time, and
everything off; all passed on the first run of the fixed tree.** After the last run the default
translation was made again and compared: identical (md5 of `cat g_*.go`).

```
switches: J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.928s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=cls
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.984s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=meth
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.902s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=iter
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	2.116s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=coll
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.997s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=list
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.952s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=makers
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.973s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=views
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.964s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=place
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.919s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=cov
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.943s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=map
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.951s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=elems
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.991s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=nncoll
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.971s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=readers
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.938s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=set
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.953s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
switches: J2GO_NOLIBGEN=wide
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.948s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20361  test lines: 4524
runtime lines of the default build: 20117  off-mode extras (build-tagged files): 244
```

Not gated: combinations of switches other than "all"; the older switches `J2GO_COLLS_NO*` and
`J2GO_ERASED=1` (5.6).

### 5.9 Fifth flip: additions to 3.8 and 4.8

- Places that were `*ImmutableList` / `*ArrayList` / `*LinkedHashMap` / `*ImmutableMap` /
  `*ImmutableSet` (aliases `ImmutableListOf[X]`, `ArrayListOf[X]`, ...: 97 fewer alias uses) are
  `ListT[X]` / `MapT[K, V]` / `SetT[X]` / `CollectionT[X]`: `colls`, `gmaps`, `immbuild` (104) and
  every step that looks for a declaration of an immutable class by its Go type find fewer.
  Results of record and AutoValue accessors and of interface methods (`DependencyInfo`,
  `JSChunk`, `CompilerInput`) are among them.
- New runtime names: `tlistOfT_` / `tsetOfT_` / `tmapOfT_` (40 sites, where `tlistImm_(tc,
  cast_[*ImmutableList](..))` was), `add_t` / `put_t` / `remove_t` / `set_t` / `setValue_t` on the
  typed interfaces, their wrappers and the views; `LinkedHashMap.typed_`; `mapAs.ks` / `vs`;
  `tlistView_.trimToSize`. `unify` (402) and the view patches touch `rt_lg.go` next to them.
- `Collection_add_s` is called less (a String goes to `add_t` on a typed receiver): `strkeys`
  (123, 180) see fewer sites.
- `rt_lg_erased.go` (tag `j2go_nolibgen`) is not part of the default build: a step that edits
  runtime functions by name must not expect `ImmutableList_of_Object_...` (5 to 11 arguments),
  `to_ArrayList`, `new_ArrayList_int` there.

## 6. Record of rule candidate T2 (branch `main-libgen-t2`, worktree `/mnt/data/ubuntu/jm/wt/libt2`, 2026-11-23)

On top of `main` 697100e9 (stage b5 merged, `docs/javaisms6/JAVAISMS6.md` written). Task: rule
candidate T2 of JAVAISMS6.md section 5 ("typed library interfaces for the 34 types the rule did
not reach", 228 census sites of the sixth flip's tree), as new kinds of the same switch
`J2GO_NOLIBGEN`, each kind its own commit, gated on, and with the kind off the translation of
the commit before. Scratch files: `/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/t2/`
(`kind.sh <label> <all T2 kinds> <this kind> <md5 before>`: under one hold of the lock, rebuild
the translator, translate with this kind off, with every T2 kind off and with everything off and
compare the md5 of `cat g_*.go` with the commit before / main's default / main's all-off
translation, then the default translation, go build, runtime tests, self-test, gate;
`tm.sh <tag> <rounds> label=bin ..`: `proto/gast.py` on `gasub/flags3500.txt` under the lock;
`cdiff.sh A B`: `census.py` rows of two translations and the T2 spellings (`t2sites.py`);
`out-<label>/`: the generated files of each kind; `kind-<label>.out`, `tm-<label>.out`, gate logs).

A second agent did candidate T1 (typed runtime library functions, branch `main-libgen-t1`) at the
same time; nothing here touches the runtime's static library functions or their name lists.
Overlap with that branch is limited to `tools/j2go/src/j2go/LibGen.java` (new tables and
functions next to the existing ones, three small edits inside `cls`, `served` and `check`),
one line each in `Ctx.typedMaker` and `ExprEmitter.lgMade`, the self-test files
(`selftest/lang/LibGen.java`, `selftest/refused/RefusedLib.java`, the count in `selftest/run.sh`)
and `tools/j2go/README.md`; the runtime additions are in new files (`go/closure/rt_lg_t2.go`,
`rt_lg_t2_test.go`).

**How far: five kinds are built, gated and committed: `sorted`, `enumset`, `cmaps`, `deque`,
`guava`. Not built: Guava `Optional`, `Ordering`, `ThreadLocal`, `CacheBuilder`, `TreeMap`, the
sorted immutables' builders and typed makers, the erased Guava immutables that do not enter a
typed place** (6.6 says what each needs). Every heavy command ran under
`flock /mnt/data/ubuntu/jm/heavy.lock` (the other agent's gates and timings held it for up to 15
minutes at a time).

### 6.1 Commits

| commit | kind (`J2GO_NOLIBGEN=`) | what |
|---|---|---|
| 88df15cc | `sorted` | `ImmutableSortedSet<X>`, `SortedSet<X>` are `SetT[X]`, `ImmutableSortedMap<K, V>` is `MapT[K, V]` (`LibGen.CLASSES_T2`, the mechanism of piece `cls`); refusal of an unchecked cast to a class of `CLASSES_T2` |
| d76811ea | `enumset` | `EnumSet<E>` is `SetT[E]`; `EnumSet.noneOf` / `allOf` / `of(e)` / `of(e, rest...)` / `copyOf(Collection)` for such a place make the typed sorted set with the ordinal comparator (`LibGen.MAKERS_T2`, `rt_lg_t2.go`); refusals: `instanceof` / cast / own method / library parameter of `EnumSet` |
| fb6166a9 | `cmaps` | `ConcurrentHashMap`, `ConcurrentMap`, `IdentityHashMap`, `EnumMap` are `MapT[K, V]` (erased objects through `mapAs_`) |
| 696a196e | `deque` | a kind of `LibGen.KINDS`: `Queue<X>` is `QueueT[X]`, `Deque<X>` is `DequeT[X]` (typed `peek`, `poll`, `remove()`, `pop`, `push`); `PriorityQueue<X>` is `QueueT[X]`; the typed deque (`TDeque`) is native |
| 0547b0a7 | `guava` | a kind of `LibGen.KINDS`: `PeekingIterator<X>` is `PeekingIteratorT[X]` (typed `next`, `peek`), `LoadingCache<K, V>` is `LoadingCacheT[K, V]` (typed `get`, `getUnchecked`) |
| df2817d5 | | runtime test of kind `guava`: the `LoadingCache` half tagged `!j2go_nofntypes` (6.5) |
| (the commit after) | | this section |

**Byte identity, checked for every commit by `kind.sh` before its gate** (md5 of `cat
go/closure/g_*.go`, the same translator binary, the runtime files of the commit in the output
directory):

| translation | must equal | sorted | enumset | cmaps | deque | guava |
|---|---|---|---|---|---|---|
| this kind off | the commit before (default) | f630308f = main | 831bc194 | c3afb8fb | da2191ed | 01a6e9f1 |
| every T2 kind off | main's default, `f630308fb905a07052ac0ea004b9214c` | equal | equal | equal | equal | equal |
| everything off (`J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all`) | main's all-off, `973b5b6d3e4f71ab1cc82872360f9759` | equal | equal | equal | equal | equal |
| default | | 831bc194 | c3afb8fb | da2191ed | 01a6e9f1 | c7100373 |

(main's two translations were made with main's translator, compiled from `git show HEAD:` of
`tools/j2go/src/j2go/*.java` into a scratch directory, `main-classes/`.) So with the kinds of
this stage off the translator writes exactly what main's writes; the runtime additions
(`rt_lg_t2.go`) do not change any translation.

### 6.2 The form

- **`LibGen.CLASSES_T2`** (class or interface -> collection interface, kind): `cls` of piece
  `cls` answers for these too when the kind is on (`LibGen.clsIface`), and decides whether the
  typed family can hold the elements by the collection kind of the interface's use (the
  classes have no collection kind of `Ctx` themselves). A method that the place's interface
  (or one above it) does not declare is called on the object erased code knows
  (`LibGen.served`): for these classes also the methods of other interfaces
  (`NavigableSet.descendingSet`, `tailSet`), which for the classes of `CLASSES` go to the
  runtime's helpers. Nothing else of `cls` changed.
- **`LibGen.MAKERS_T2`** (kind `enumset` only): a library function, keyed by its parameters'
  erasures, that makes a fresh collection for a typed place: `Ctx.typedMaker` reads it after
  `Ctx.TYPED_MAKERS`, `ExprEmitter.lgMade` takes the collection kind of the class's interface
  (`LibGen.t2Kind`). So `EnumSet.noneOf(Feature.class)` for an `EnumSet<Feature>` place is
  `tenumSetNoneOf_(tc_p_FeatureSet_Feature, classOf_(..)).asSetT()`: the typed set in its sorted
  mode (`rt_tsorted.go`) with `ordinalCmp_()`, the comparator of the runtime's `EnumSet`, which
  is a `TreeSet` with that comparator (`rt_coll2.go`). Same structure, same order, typed.
- **Kind `deque`** is a kind of `LibGen.KINDS` like `list`: the typed interfaces `QueueT` and
  `DequeT` (`rt_lg_t2.go`) are the erased interface, the methods of `CollectionT`, and typed
  `peek_t` / `poll_t` / `remove_0_t` / `pop_t` / `push_t` (`LibGen.TYPED_METHODS`). The view of
  the typed deque (`tdequeView_`) has them, so a `TDeque` enters natively (`asDequeT()`,
  `asQueueT()`, `LibGen.fromTypedColl`); the wrappers (`dequeAs_`, `queueAs_`) call the erased
  object through the interface of the one method (the trap of 5.8). `Deque`'s methods that
  `SequencedCollection` declares first (`getFirst`, `addFirst`, `removeLast` ...) have their
  root there and stay the runtime's helpers on the erased object.
- **Kind `guava`**: the same for `PeekingIterator` (typed `next_t`, `peek_t`) and `LoadingCache`
  (typed `get_t`, `getUnchecked_t`, key and value at their types); the runtime makes both
  erased (`Iterators.peekingIterator`, `CacheBuilder.build`), so each value is one wrapper.

### 6.3 Counts

Sites, counted on the pre-flip translation (`g_*.go` of the tree, the translator's own output;
the census of JAVAISMS6.md is of the sixth flip's tree, after the post-flip steps, and its 228
T2 sites cannot be recounted without a flip): `cdiff.sh` runs `tools/postflip/census.py` (the
copy in `$B/wt/pf5`) on both translations and `t2sites.py` counts the T2 spellings in
`g_[A-Z]*.go` (the census rows count `g_0*.go` too, where every new typed conversion adds a
`tc_` descriptor with an `any` parameter and a `cast_`; that is why "fields/params declared
any" rises). Each column is against the kind before; the last against main.

| census row (`census.py`) | sorted | enumset | cmaps | deque | guava | main -> final |
|---|---:|---:|---:|---:|---:|---:|
| Java collections (erased): constructors | 0 | 0 | 0 | -2 | 0 | 44 -> 42 (-2) |
| for-each via iterator objects (foreach_/each) | -14 | -1 | -1 | -1 | 0 | 782 -> 765 (-17) |
| interface casts icast_X( | -2 | 0 | -1 | -1 | 0 | 322 -> 318 (-4) |
| class casts to_X( / cast_[T]( | -1 | 0 | -4 | -1 | -1 | 1255 -> 1248 (-7) |
| boxing to any: obj_( / strobj_( / .box_() / any( | -7 | -4 | -7 | +1 | +2 | 1601 -> 1586 (-15) |
| unboxing from any: str_( / unboxing casts | -7 | 0 | -2 | 0 | 0 | 210 -> 201 (-9) |
| Java equals/hashCode protocol | +2 | 0 | +2 | 0 | 0 | 437 -> 441 (+4) |
| fields/params declared any (approx) | +5 | +1 | 0 | +2 | +3 | 1378 -> 1389 (+11) |

| T2 spelling (`t2sites.py`, `g_[A-Z]*.go`) | sorted | enumset | cmaps | deque | guava | main -> final |
|---|---:|---:|---:|---:|---:|---:|
| alias uses XOf[ of T2 types | -21 | -17 | -12 | -7 | -3 | 123 -> 63 (-60) |
| foreach_(obj_(..)) erased loops | -10 | -1 | 0 | 0 | 0 | 41 -> 30 (-11) |
| foreach_( all | -14 | -1 | -1 | -1 | 0 | 121 -> 104 (-17) |
| cast_[ all | -6 | -1 | -3 | -1 | -4 | 675 -> 660 (-15) |
| str_( all | -7 | 0 | 0 | 0 | 0 | 127 -> 120 (-7) |
| obj_( all | -12 | -5 | -5 | 0 | -1 | 481 -> 458 (-23) |
| icast_ all | -2 | 0 | -1 | -2 | 0 | 202 -> 197 (-5) |
| EnumSet_ calls | 0 | -25 | 0 | 0 | 0 | 31 -> 6 (-25) |
| xAs_ wrappers | +14 | -16 | +6 | +3 | +3 | 243 -> 253 (+10) |
| lgBox_( | +7 | 0 | +4 | 0 | 0 | 61 -> 72 (+11) |
| X_un_( | +1 | +1 | +13 | +36 | 0 | 388 -> 439 (+51) |

Reading them: the erased alias uses of the T2 types (`ImmutableSortedSetOf[`, `EnumSetOf[`,
`DequeOf[` ...) fell from 123 to 63: what is left is `Optional_baseOf` 38 (not built),
`ImmutableSortedSet_BuilderOf` / `ImmutableSortedMap_BuilderOf` 9 (builders, not built),
`TreeMapOf` 5, `OrderingOf` 5, `CacheBuilderOf` 5, `RangeSetOf` 1 (6.6). Casts and boxes fell where a
read of an element went through the erased interface before (`cast_[X](it.v)` of a for-each
over `foreach_(obj_(s))`, `str_(..)` of a `String` element, `obj_(x)` of an argument). The
conversions at the boundary rose instead: `X_un_(` +51 (a typed value given to an erased
parameter or receiver: a type test, no allocation; 36 are `Deque_un_(d.asDequeT())` where an
erased method or a `SequencedCollection` helper is called on a typed `TDeque` of rule "typed
collections", which was `d.asDeque()` before, and 13 `Map_un_(m)` where a `cmaps` map reaches
the String-key helpers `Map_get_s` / `Map_putIfAbsent_s`), `xAs_`
wrappers +10 net (the T2 values made erased by the runtime now enter through a wrapper,
while the typed `EnumSet` makers removed 16 `setAs_` of `FeatureSet`), `lgBox_(` +11.

`J2GO_LIBGEN=1` of the final translation against main's (`libgen:` lines; scratch `lg-*.txt`;
as in 5.5, the "places left erased" lines count calls of `erase`, not places):

| `J2GO_LIBGEN=1` line | main | final |
|---|---:|---:|
| erased methods on a typed receiver (no box, no element result) | 330 | 351 |
| erased methods on a typed receiver that box or give an element as Object | 27 | 29 |
| erased to typed (dequeAs_) | 0 | 2 |
| erased to typed (iterableAs_) | 56 | 55 |
| erased to typed (loadingCacheAs_) | 0 | 2 |
| erased to typed (mapAs_) | 48 | 58 |
| erased to typed (peekingIteratorAs_) | 0 | 1 |
| erased to typed (queueAs_) | 0 | 2 |
| erased to typed (setAs_) | 40 | 35 |
| methods of a collection class called on the erased object | 3 | 4 |
| places left erased: a List with a wildcard (variance) | 377 | 351 |
| places left erased: a type argument is an erased type variable | 864 | 906 |
| places left erased: every type argument is the wildcard `?` | 267 | 256 |

Refusals: the tree has none of any kind (every translation above exited 0). New refused uses
in `selftest/refused/RefusedLib.java`: 8 (unchecked cast to a class of `CLASSES_T2`, kind
`sorted`), 9 (`instanceof EnumSet`) and 10 (a library parameter of type `EnumSet`:
`EnumSet.copyOf(EnumSet)`), kind `enumset`; the self-test fails unless exactly the 10 marked
uses are refused. Kind `deque` adds no reason: the raw-use and unchecked-cast refusals cover
`Queue` and `Deque` as interfaces of a kind; one self-test program (`Colls.deques`, which casts
an `Object` back to `Deque<String>` on purpose) is now listed in
`selftest/unchecked-casts.txt` with the others of its kind.

Runtime lines (verify.sh, default build): 20,117 -> 20,373 (`rt_lg_t2.go` 256 lines:
the `EnumSet` makers 42, `QueueT` / `DequeT` with wrappers and views 127, `PeekingIteratorT` /
`LoadingCacheT` with wrappers 79, the file comment 8); off-mode extras unchanged (244); test lines 4,524 ->
4,626 (`TestLgT2EnumSet`, `TestLgT2Deque`, `TestLgT2Guava`, `TestLgT2GuavaCache`).

### 6.4 Timing (CPU, googleapis 18% subset)

`proto/gast.py gasub/flags3500.txt 3` (three stages, alternating order, `--num_parallel_threads=1`),
each kind's gate binary against the kind before's, under the lock; the output sha was
`72b57cd286c9` in every run.

| kind | before: mean (min) CPU s | after: mean (min) CPU s | change of the means |
|---|---:|---:|---:|
| sorted (base: main's translation, `m_t2_base`) | 165.45 (164.55) | 166.65 (164.99) | +0.7% |
| enumset | 167.43 (166.67) | 168.61 (167.91) | +0.7% |
| cmaps | 167.04 (165.90) | 167.62 (166.86) | +0.3% |
| deque | 167.44 (167.22) | 167.91 (167.40) | +0.3% |
| guava | 168.38 (167.96) | 167.61 (166.52) | -0.5% |
| all five: main's translation (`m_t2_base`) against the final (`m_t2_guava`), 4 rounds | 165.27 (163.87) | 165.84 (165.17) | +0.3% |

The spread of one binary within a session is about 1.5 s (1%), and the same binary measured in
two sessions differed by 0.8 s (`t2sorted` 166.65 and 167.43), so no single kind's result is
outside the noise. Neutral, as expected: the sites are in cold code (options, conformance,
serialization, `FeatureSet`'s constants, the transpiler cache), except
`PropertyMap.getAllKeys` / `ObjectType.getPropertyNames` (kind `sorted`: one wrapper per cached
key set; its loops read the erased set as before), `Node.isEquivalentTo` (kind `enumset`: its
`EnumSet<Prop>` is now the typed sorted set, the same comparisons), `LinkedIdentityHashMap`
(kind `cmaps`: one wrapper per map, reads through it), `DataFlowAnalysis.UniqueQueue` and
`ControlFlowAnalysis.prioritizeFromEntryNode` (kind `deque`: the work queue is a typed
`TDeque` or a wrapped `PriorityQueue`).

### 6.5 Gate lines

Each kind, default translation, `$B/gate.sh libt2 t2_<kind>` under the lock (logs
`/mnt/data/ubuntu/jm/logs/gate-t2_<kind>.log`, copies in the scratch directory). Every one
printed `GATE OK t2_<kind>` and these lines (the runtime counts and the test line differ: see
the table):

```
selftest: PASS
ok  	jsgomech/closure	(cached)
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
```

| gate | runtime lines | test lines | default build | runtime tests |
|---|---:|---:|---:|---|
| t2_sorted | 20361 | 4524 | 20117 | `ok jsgomech/closure 1.973s` (run before the gate) |
| t2_enumset | 20409 | 4567 | 20165 | `ok 1.939s` |
| t2_cmaps | 20409 | 4567 | 20165 | `ok 1.943s` |
| t2_deque | 20537 | 4599 | 20293 | `ok 2.034s` |
| t2_guava | 20617 | 4616 | 20373 | `ok 2.041s` (in the gate: the first `kind.sh` run of this kind failed to compile its new runtime test, `new_ArrayList` is `new_ArrayList_0`; fixed before its gate started) |

("cached" in the gate: the same test binary had run just before it, in `kind.sh`.)

Each T2 kind off alone, and everything off (`J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all`), on the final
code (`offall.sh`: translate with the switch, build with `-tags j2go_nolibgen` (and
`j2go_nofntypes`), `tools/prune/verify.sh`, each under the lock; logs `verify-<switch>.log`; after
the runs the default translation was made again and was identical, md5 `c7100373`). All passed;
the lines of each (`runtime lines: 20617  test lines: 4626`, `runtime lines of the default build:
20373  off-mode extras (build-tagged files): 244` in every one):

```
switches: J2GO_NOLIBGEN=sorted      selftest: PASS  ok  jsgomech/closure 2.029s  checks/bundle/downlevel/advanced: MATCH (the four lines above)
switches: J2GO_NOLIBGEN=enumset     selftest: PASS  ok  jsgomech/closure 1.993s  MATCH x4
switches: J2GO_NOLIBGEN=cmaps       selftest: PASS  ok  jsgomech/closure 1.980s  MATCH x4
switches: J2GO_NOLIBGEN=deque       selftest: PASS  ok  jsgomech/closure 1.997s  MATCH x4
switches: J2GO_NOLIBGEN=guava       selftest: PASS  ok  jsgomech/closure 1.970s  MATCH x4
switches: J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.980s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
```

The first run with everything off failed: its four MATCH lines and the self-test passed, but
the runtime tests did not compile (`TestLgT2Guava` gave a Go func literal for a
`Function_base_e`, which with `J2GO_NOFNTYPES=1` is an interface; log `verify-alloff-first.log`).
Fixed in commit df2817d5 (the `LoadingCache` half of the test in `rt_lg_t2_fn_test.go`, tagged
`!j2go_nofntypes` as `rt_fn_test.go` is); the five single-kind runs above already had the fixed
file; the run with everything off was repeated (the lines above), and the default translation
was gated again as `t2_final` after it:

```
GATE OK t2_final
tree: df2817d58 runtime test of kind guava: the LoadingCache part needs rule "function types" (b  dirty: 1
selftest: PASS
ok  	jsgomech/closure	1.946s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20617  test lines: 4626
runtime lines of the default build: 20373  off-mode extras (build-tagged files): 244
```

(`dirty: 1` is the untracked `.cache` link of the worktree.)

### 6.6 Not built, and what each needs

- **Guava `Optional`** (38 alias uses, JAVAISMS6's 34 `Optional_base_*` sites). It is a class,
  and the runtime's `Optional_base` a struct, so the rule's form (typed interface = erased
  interface + `_t` methods) needs: `Optional_base` among the libraries of a kind although it is
  no interface (`Ctx.goType` case `CLS` would answer the typed name; `X_un_` a generic function
  `Optional_base_un_[T]`), a typed interface with `isPresent`, `get_t`, `or_t`, `orNull_t`,
  `transform`, `equals`, `unwrap_` (`or` has three overloads of one name and arity:
  `TYPED_METHODS` keys `name/arity` cannot tell them apart), and typed makers for `of` /
  `absent` / `fromNullable` (a generic struct with the value at its type and the erased object
  made when erased code asks; `absent()` a per-type singleton through `single_`), which the
  makers' path of `lgMade` (collection kinds only) does not serve. Without the makers every
  `Optional` would be two objects. Not started.
- **`TreeMap`** (4 places, all `TreeMap<JSDocInfo.Property<?>, Object>`): the key has type
  arguments, which the typed family does not hold, so the kind would change nothing; and a
  `TreeMap` made for a typed place is the typed sorted map's view, on which `TreeMap`'s own
  navigable methods would fail (it is in `LibGen.CONCRETE`; the own-method refusal of piece
  `cls` is only for `CLASSES`).
- **The sorted immutables' builders and typed makers** (`ImmutableSortedSet.naturalOrder()`,
  `copyOf`, `copyOfSorted`: 9 builder places, 10 factory calls of the census row "Guava
  immutable collections / builders"): kind `sorted` lets the erased objects
  enter through `setAs_` / `mapAs_`. A typed `ImmutableSortedSet` exists (`tsetImmNew_` of an
  `icSorted_` twin, `rt_timm.go`); the builders would need `Colls`' proof as the others (5.7
  item 2).
- **`Ordering<T>`** (5 fields of `SymbolTable`), **`CacheBuilder<K, V>`** (5, all at `Object,
  Object`), **`ThreadLocal<T>`** (1): classes whose methods the translator calls through the
  runtime's static functions (`Ordering_*`, rule candidate T1's subject) or that hold nothing
  typed; not touched.
- **The erased Guava immutables that enter no typed place** (49 calls of the census row "Guava
  immutable collections / builders" in the pre-flip translation, unchanged by this stage):
  `for (x : ImmutableList.copyOf(..))` (5), arguments of the runtime's static functions
  (`Lists.reverse`, `Lists.transform`, `ImmutableList.of(..)` as a message argument: T1's
  subject), builders whose chain `Colls` does not type. A for-each over a fresh immutable copy
  could be typed by its loop variable (the copy made typed, the loop `eachOf_`); not built.

### 6.7 Traps met

- `LibGen.check` runs before the collection kinds of `Ctx` are registered, so `cx.erase` of a
  class of `CLASSES_T2` there is still the class: the unchecked-cast refusal for those classes
  is decided on the type as written (a use with type arguments), not on `is(cx.erase(t))`.
- A library method with a parameter of the class whose places the makers serve
  (`EnumSet.copyOf(EnumSet)`) gets `cast_[*EnumSet](lgBox_(x))` for its argument, which fails
  for a set made by the makers: refused, not served.
- `EnumSet.of` has fixed-arity overloads (`of(e1, e2, e3)`) that the runtime does not have at
  all; only `of(e)` and `of(e, rest...)` are makers, keyed by the parameters' erasures.
- The self-test's `Deque` program (`lang/Colls.java`) has an intentional unchecked cast that
  the existing refusal reaches as soon as `Deque` is a kind.
- `tdequeView_` embeds `tlistView_`, so a typed deque's view is also a `ListT`; its own
  `unwrap_` (rt_lg.go) keeps `lgBox_` on the deque view.

### 6.8 Not verified

- The 228 T2 sites of JAVAISMS6.md are sites of the sixth flip's tree; no flip was made, so how
  many of them these kinds remove after the post-flip steps is not known. The tables of 6.3
  are of the translator's own output, before any post-flip step, with JAVAISMS6's census
  script but not its classifier (`classify6.py` reads the sites of `jclass` on a flipped tree).
- No allocation profile (CHECKS with `JSGO_MEM_PROFILE`) was taken; the wrappers that kinds
  `sorted`, `cmaps`, `deque` and `guava` add (one per value entering a typed place from an
  erased one) and the ones `enumset` removes were not counted at run time.
- Combinations of switches other than "one T2 kind off", "every T2 kind off" (byte-compared
  only, not gated) and "everything off" were not translated; the switches of the earlier
  stages were not gated off again (their translations are byte-identical to main's, 6.1).
- Concurrent use of the `cmaps` maps (the runtime's `ConcurrentHashMap` is a `LinkedHashMap`
  without a lock, before and after) and `ConcurrentModificationException` through the new
  wrappers were not tested.
- Post-flip steps: not run. Tools that follow types by name meet new names (`QueueT[`,
  `DequeT[`, `PeekingIteratorT[`, `LoadingCacheT[`, `queueAs_`, `dequeAs_`,
  `peekingIteratorAs_`, `loadingCacheAs_`, `tenumSet*_`, `asDequeT()` / `asQueueT()` where
  `asDeque()` was) and fewer `XOf[` aliases (6.3).
