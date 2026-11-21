# Typed collections, the other half: map of the work, plan, and living record

Branches `main-libgen-b3` (worktree `/mnt/data/ubuntu/jm/wt/agZ`; sections 0 to 3) and
`main-libgen-b4` (worktree `/mnt/data/ubuntu/jm/wt/agZ2`; section 4), on top of `main-libgen-b2`
c8a3f9d5 (LIBGEN-STUDY.md sections 7 and 8: rules "function types" and "typed library
interfaces", kind `iter`). This file is the handover: a successor continues from it alone.
**The current state, what is left and the traps are in section 4** (4.6 replaces 3.6; sections
0 to 3 are the record of the stage before, kept as written).
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
