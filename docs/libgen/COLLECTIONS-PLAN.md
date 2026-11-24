# Typed collections, the other half: map of the work, plan, and living record

Branches `main-libgen-b3` (worktree `/mnt/data/ubuntu/jm/wt/agZ`; sections 0 to 3) and
`main-libgen-b4` (worktree `/mnt/data/ubuntu/jm/wt/agZ2`; section 4), on top of `main-libgen-b2`
c8a3f9d5 (LIBGEN-STUDY.md sections 7 and 8: rules "function types" and "typed library
interfaces", kind `iter`). This file is the handover: a successor continues from it alone.
**The current state, what is left and the traps are in section 5** (stage b5, branch
`main-libgen-b5`, worktree `/mnt/data/ubuntu/jm/wt/agZ3`; 5.7 replaces 4.6; sections 0 to 4 are
the records of the stages before, kept as written).
**Section 6 records piece `fns` (rule candidate T1 of `docs/javaisms6/JAVAISMS6.md`, branch
`main-libgen-t1`); section 7 records rule candidate T2** (branch
`main-libgen-t2`, worktree `/mnt/data/ubuntu/jm/wt/libt2`: kinds `sorted`, `enumset`, `cmaps`,
`deque`, `guava`; 7.6 adds to 5.7 what is left of the library types; written as section 6 on its
branch, renumbered 7 by the merge of both into main-libgen-t12).
**Section 9 records candidate T3** (branch `main-libgen-t3`: kind `goptional`, Guava's `Optional`
as a value library class, and piece `seqcoll`, the `SequencedCollection` twins).
**Section 10 records JAVAISMS6.md's candidate "T3"** (Object-parameter collection methods typed;
branch `main-libgen-t4`: piece `objarg`; its current form is 10.8).
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

## 6. Record of piece `fns` (candidate T1 of docs/javaisms6/JAVAISMS6.md; branch `main-libgen-t1`, worktree `/mnt/data/ubuntu/jm/wt/libt1`, 2026-11-23)

On top of main 697100e9. Scratch files: `/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/t1/`
(`jc.sh` rebuild the translator; `tx.sh` translate; `st.sh` self-test; `pipe.sh <label>` translate +
build + gate under one hold of the lock; `count.sh <base> <new>` the counts below; `basebin.sh` main's
translation with main's runtime as `$B/bin/m_t1_base`; `tm.sh` timing; `alloff.sh` the all-off
translation by main's translator and this one; `offall.sh` / `off.sh` the gates with switches off;
`rep.sh` the J2GO_LIBGEN=2 report, `rep.log`; `base/` and `new/` the two translations). Every heavy
command ran under `flock /mnt/data/ubuntu/jm/heavy.lock` (shared with the agent of candidate T2).

### 6.1 What it is

T1: the runtime's static library functions and the default methods of the library interfaces were
erased (`Iterables_getFirst(p0 Iterable, p1 any) any`), so a call on a typed collection took its
erased form (`List_un_(x)`, `Iterable(..)`), boxed its arguments, cast its result, and a
lambda given to it was a converting closure. Piece `fns` of rule "typed library interfaces"
(`J2GO_NOLIBGEN=fns`; it needs rule "function types", so `J2GO_NOFNTYPES=1` turns it off too)
adds typed twins (`go/closure/rt_lgfn.go`, 320 lines, tag `!j2go_nofntypes`), Go generic
functions at the Java generic signature with the typed interfaces of rt_lg.go and the func types
of rule "function types", and a name list in the translator (`tools/j2go/src/j2go/LibFns.java`,
`LibFns.TYPED`), the mechanism of `mapComputeIfAbsentT_` / `sortSliceF_` (which are post-flip
tools; this is the translator). Each entry has the twin's Go signature as a template (`$i` the
i-th type argument). A call becomes the twin's, instantiated with javac's type arguments (a static:
its own type parameters as inferred; a default method: the receiver's typed interface's), where:

- the Go types that javac's types give for the call's parameters and result are the template's
  (text comparison: an element type a typed interface cannot hold, a wildcard, an erased type
  variable fail here) and the runtime declares the twin (`Conv.rtGenerics`);
- every collection argument and the receiver enter at their typed interface as they are (a typed
  collection, or a typed interface of the same element types): variance or an erased argument
  would need a wrapper, so the call stays erased;
- a typed collection's own method (`TMap.computeIfAbsent`) is not replaced;
- the result does not go to an `Object` (`ExprEmitter.lfErased`, set from `lgWantErased`): Java
  casts nothing there, so an element of another class that unchecked code put into the collection
  passes, where the twin would cast it. (Found by the self-test: LibGen.java's `wrong` map,
  `String.valueOf(wrong.get("k"))`.)

Twins (17 functions): `Iterables.getFirst / getLast / getOnlyElement / get / indexOf / filter /
transform` (filter and transform give a typed Iterable: `iterViewT_`, `filterIterT_`,
`mapIterT_`), `MoreObjects.firstNonNull`, `Collections.sort(List)` and `(List, Comparator)`,
`List.sort`, `Iterable.forEach`, `Map.forEach / replaceAll / computeIfAbsent / getOrDefault /
merge`, and `Map.get / getOrDefault` with a String key on a map of String keys (the twins of the
helpers `Map_get_s` / `Map_getOrDefault_s`: the typed map's own lookup, else the erased helper and
`unboxT_`). Two translator places outside the name list, same switch: `new TreeSet<>(comparator)`
/ `new TreeMap<>(comparator)` of a typed collection take the comparator at the key type
(`newTSortedSetCT_` / `newTSortedMapCT_`; the sorted set keeps it as `teq_.cmpT`, a `func(K, K)
int32`, and compares keys without boxing them; erased code asking for `comparator()` gets an
adapter), and `Splitter.splitToList` of a String takes it unboxed (`splitToListST_`). Each twin
does what the erased function does, step by step (the same calls on the collection in the same
order: List_sortT_ copies with `get_t`, sorts with `sort.SliceStable` as `sortSlice_` does, sets
back with `set_t`; natural order compares two Strings with `String_compareTo`, else the boxes).

Left erased, by function (`LibFns.REFUSED`, reported by `J2GO_LIBGEN=2`):
`Lists.reverse` (6 calls) and `Lists.transform` (2): live List views; a twin needs a List class
of the runtime at the element type (not a minimal runtime). `SequencedCollection.getFirst /
getLast / removeLast / addLast` (72): no typed interface; the receivers are Deques (candidate T2;
5 are a List, `contextStack`). `CacheLoader.from` (1): LoadingCache (T2). `Map.putIfAbsent` (24
calls of the root; the 2 erased helper calls left have erased receivers). Not attempted: the 24
"runtime method whose Java signature is typed or overloaded" sites (`BigInteger.compareTo`,
`Enum.compareTo`, `ComparisonChain.compare`): instance methods of runtime classes, not generic
functions; they need a rule for runtime method signatures. `Comparator.comparing` with a key typed
`any` (2): already generic in the runtime; the key's type is what javac inferred. `Object.clone`
(3): not generic.

Left erased, by site (default translation, `rep.log`): the result enters an Object 12 (`Map.get`
with a String key), the receiver is erased 6 (`Map.get` 3, `computeIfAbsent` 2, `getOrDefault`
1), a parameter's Go type is not the twin's 1 (`Iterables.transform` to `Function_base[F, any]`).

### 6.2 Counts (`count.sh`, main's default translation `base/` against this one `new/`, `g_*.go`)

Calls of the erased function -> left, and of the twin: getFirst 14 -> 0 (14), getLast 9 -> 0 (9),
getOnlyElement 19 -> 0 (19), get 5 -> 0 (5), indexOf 1 -> 0, filter 5 -> 0, transform 5 -> 1 (4),
firstNonNull 5 -> 0, Collections_sort 10 -> 0, Collections_sort_List_Comparator 4 -> 0, List_sort 12
-> 0 (List_sortT_ 16 with those 4), Iterable_forEach 7 -> 0, Map_forEach 9 -> 0, Map_replaceAll 1
-> 0, Map_computeIfAbsent 6 -> 2 (4), Map_getOrDefault 1 -> 0, Map_merge 3 -> 0, Map_get_s 22 -> 5
(17), Map_getOrDefault_s 3 -> 1 (2), newTSortedSetC_ 9 -> 0 (9), splitToListT_ 22 -> 0 (22):
**172 calls of 182 typed** (163 by the name list, 9 sorted sets; 22 splitToList).

Census rows (`pf5/tools/postflip/census.py` on each directory), **-310 sites**:

| row | main | t1 | |
|---|---:|---:|---:|
| for-each via iterator objects | 782 | 780 | -2 |
| interface casts `icast_X(` | 322 | 298 | -24 |
| class casts | 1,255 | 1,165 | -90 |
| boxing to any | 1,601 | 1,554 | -47 |
| unboxing from any | 210 | 155 | -55 |
| Optional | 1,401 | 1,402 | +1 (a line count: one call split differently) |
| fields/params declared any (approx) | 1,378 | 1,286 | -92 |

Conversions at the boundary: `Map_un_(` 66 -> 29, `List_un_(` 97 -> 80, `Collection_un_(` 56
-> 41, `Set_un_(` 82 -> 79, `Iterable_un_(` 60 -> 58, `lgBox_(` 86 -> 83, `iterableAs_(` 56 ->
55, `listAs_(` 78 -> 76, `setAs_(` 40 -> 38, `mapAs_(` 48 -> 47; converting closures `func()
Comparator[any]` 27 -> 8, `BiConsumer[any, any](` 9 -> 0, `Consumer[any](` 47 -> 40,
`Comparator[any](` 3 -> 0, `Function_base[any, any](` 7 -> 3, `BiFunction[any, any, any](` 6 -> 2.

These are counts on the translator's output. JAVAISMS6.md's 283 are sites of the sixth flip's tree
(post-flip tools applied, which rewrite some of these calls themselves, `tfnapi`, `native`); the
census of a flip of this translation was not made.

### 6.3 Timing (gast.py, flags3500 subset, 3 rounds alternating; `tm-t1.log`)

`t1base` = main's translation with main's runtime (`basebin.sh`), `t1` = this branch (gate
binary `m_t1_a`): cpu mean 165.42 s (min 164.92) against 166.34 s (min 165.08), +0.6% mean, +0.1%
min: **neutral** (within the spread of each binary's runs, 1.3 s). Output sha 72b57cd286c9 in all
six runs. No allocation profile was taken.

### 6.4 Switches and gates

With `J2GO_NOLIBGEN=fns` the generated files are byte-identical to main's default translation (md5
of `cat g_*.go` f630308fb905a07052ac0ea004b9214c, both); with `J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all`
main's translator and this one give the same files (973b5b6d3e4f71ab1cc82872360f9759,
`alloff.sh`). Default translation: 88d6f66b25b35b59b1ed0b8e6ef4a9c3. Not behind the switch: the
runtime (`rt_lgfn.go`, `teq_.cmpT` and its test in `tsortCmp_`, which is nil without the
piece), and `ExprEmitter.lfErased` (read only by the piece).

Self-test: `selftest/lang/LibFns.java`: `typed()` has 33 twin calls, `erased()` 4 calls that must
stay erased (variance: `Iterables.getFirst(List<Dog>, Animal)`; `Iterable<?>`; a `Map<Object,
String>` read with a String; `Lists.reverse`) and no twin; `run.sh` counts both in the
generated `g_LibFns.go` (when `J2GO_NOLIBGEN` and `J2GO_NOFNTYPES` are unset), and both print
what Java prints.

Gate, default translation (`$B/gate.sh libt1 t1_a`, the tree of commit 7a414f32 before it was
committed): `GATE OK t1_a`

```
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.941s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20685  test lines: 4524
runtime lines of the default build: 20441  off-mode extras (build-tagged files): 244
```

(main: 20117 lines of the default build; +320 `rt_lgfn.go`, +4 `teq_.cmpT`.)

Switches off (`offall.sh fns alloff`, `tools/prune/verify.sh` with the switches; logs
`verify-fns.log`, `verify-alloff.log`): both selftest PASS, `ok`, 4 MATCH, with the same lines as
above; the default translation made again afterwards was identical.

### 6.5 Not done, unverified, overlaps

- No Go unit tests for the twins (the self-test compares them with Java; the tree's e2e compares
  the outputs). No CHECKS allocation profile.
- The census of a flip of this translation (the post-flip tools' rewrites of `Map_computeIfAbsent`,
  `List_sort`, ... now meet twins, which they do not know; `tfnapi` / `native` steps that look for
  the erased calls find fewer).
- Overlap with candidate T2 (branch `main-libgen-t2`): `rt_tset.go` / `rt_tsorted.go` (`teq_`,
  `tsortCmp_`: 4 lines), `Ctx.java` (one field next to `lg`), `ExprEmitter.java` (`special`,
  `newTyped`, `immutableOf`, `invocation`), `selftest/run.sh` (class list), `tools/j2go/README.md`
  (the switch list). The `SequencedCollection` and `CacheLoader` calls are T2's types; once T2
  types Deque / TreeMap, their default methods can join `LibFns.TYPED` (a receiver template).

## 7. Record of rule candidate T2 (branch `main-libgen-t2`, worktree `/mnt/data/ubuntu/jm/wt/libt2`, 2026-11-23)

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
typed place** (7.6 says what each needs). Every heavy command ran under
`flock /mnt/data/ubuntu/jm/heavy.lock` (the other agent's gates and timings held it for up to 15
minutes at a time).

### 7.1 Commits

| commit | kind (`J2GO_NOLIBGEN=`) | what |
|---|---|---|
| 88df15cc | `sorted` | `ImmutableSortedSet<X>`, `SortedSet<X>` are `SetT[X]`, `ImmutableSortedMap<K, V>` is `MapT[K, V]` (`LibGen.CLASSES_T2`, the mechanism of piece `cls`); refusal of an unchecked cast to a class of `CLASSES_T2` |
| d76811ea | `enumset` | `EnumSet<E>` is `SetT[E]`; `EnumSet.noneOf` / `allOf` / `of(e)` / `of(e, rest...)` / `copyOf(Collection)` for such a place make the typed sorted set with the ordinal comparator (`LibGen.MAKERS_T2`, `rt_lg_t2.go`); refusals: `instanceof` / cast / own method / library parameter of `EnumSet` |
| fb6166a9 | `cmaps` | `ConcurrentHashMap`, `ConcurrentMap`, `IdentityHashMap`, `EnumMap` are `MapT[K, V]` (erased objects through `mapAs_`) |
| 696a196e | `deque` | a kind of `LibGen.KINDS`: `Queue<X>` is `QueueT[X]`, `Deque<X>` is `DequeT[X]` (typed `peek`, `poll`, `remove()`, `pop`, `push`); `PriorityQueue<X>` is `QueueT[X]`; the typed deque (`TDeque`) is native |
| 0547b0a7 | `guava` | a kind of `LibGen.KINDS`: `PeekingIterator<X>` is `PeekingIteratorT[X]` (typed `next`, `peek`), `LoadingCache<K, V>` is `LoadingCacheT[K, V]` (typed `get`, `getUnchecked`) |
| df2817d5 | | runtime test of kind `guava`: the `LoadingCache` half tagged `!j2go_nofntypes` (7.5) |
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

### 7.2 The form

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

### 7.3 Counts

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
`TreeMapOf` 5, `OrderingOf` 5, `CacheBuilderOf` 5, `RangeSetOf` 1 (7.6). Casts and boxes fell where a
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

### 7.4 Timing (CPU, googleapis 18% subset)

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

### 7.5 Gate lines

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

### 7.6 Not built, and what each needs

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

### 7.7 Traps met

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

### 7.8 Not verified

- The 228 T2 sites of JAVAISMS6.md are sites of the sixth flip's tree; no flip was made, so how
  many of them these kinds remove after the post-flip steps is not known. The tables of 7.3
  are of the translator's own output, before any post-flip step, with JAVAISMS6's census
  script but not its classifier (`classify6.py` reads the sites of `jclass` on a flipped tree).
- No allocation profile (CHECKS with `JSGO_MEM_PROFILE`) was taken; the wrappers that kinds
  `sorted`, `cmaps`, `deque` and `guava` add (one per value entering a typed place from an
  erased one) and the ones `enumset` removes were not counted at run time.
- Combinations of switches other than "one T2 kind off", "every T2 kind off" (byte-compared
  only, not gated) and "everything off" were not translated; the switches of the earlier
  stages were not gated off again (their translations are byte-identical to main's, 7.1).
- Concurrent use of the `cmaps` maps (the runtime's `ConcurrentHashMap` is a `LinkedHashMap`
  without a lock, before and after) and `ConcurrentModificationException` through the new
  wrappers were not tested.
- Post-flip steps: not run. Tools that follow types by name meet new names (`QueueT[`,
  `DequeT[`, `PeekingIteratorT[`, `LoadingCacheT[`, `queueAs_`, `dequeAs_`,
  `peekingIteratorAs_`, `loadingCacheAs_`, `tenumSet*_`, `asDequeT()` / `asQueueT()` where
  `asDeque()` was) and fewer `XOf[` aliases (7.3).

## 8. The merge of T1 and T2 (branch `main-libgen-t12`, worktree `/mnt/data/ubuntu/jm/wt/libt12`, 2026-11-23)

main 697100e9, then `main-libgen-t1` (915c0e06) and `main-libgen-t2` (43cfd88f) merged in that order.
Conflicts: this file (both appended a section 6; T2's is section 7 here) and `tools/j2go/README.md`
(both bullets kept; the switch list ends `fns,sorted,enumset,cmaps,deque,guava`). The Java sources
merged without conflict (`Ctx.java`, `ExprEmitter.java`, `selftest/run.sh`); no edit was made beyond
the two conflicts. Scratch: `/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/t12/`
(`pipe.sh`, `cross.sh`, their `.out`).

md5 of `cat go/closure/g_*.go`, the merged translator, the merged runtime in the output directory:

| translation | md5 | must equal |
|---|---|---|
| everything off (`J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all`) | 973b5b6d3e4f71ab1cc82872360f9759 | main's all-off: equal |
| fns and the five T2 kinds off | f630308fb905a07052ac0ea004b9214c | main's default: equal |
| fns off | c7100373dd7ddbf3c4bb8fb4a6dba29c | T2's default (7.1): equal |
| the five T2 kinds off | 88d6f66b25b35b59b1ed0b8e6ef4a9c3 | T1's default (6.4): equal |
| default | 4440048ae31591b5be2bc353c8655e43 | (new) |

So each piece off gives
exactly the other branch's translation. Gate of the default translation, `$B/gate.sh libt12 t12_a`:
`GATE OK t12_a`

```
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	(cached)
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 20941  test lines: 4626
runtime lines of the default build: 20697  off-mode extras (build-tagged files): 244
```

(`ok ... 1.975s` in the first gate run, which is kept as `$B/logs/gate-t12_a.first-failed.log`: its
self-test line was `... 2 more`, the tail of a Java stack trace, not `selftest: PASS`; the self-test run
alone right after printed `selftest: PASS`, and so did the gate run again. The cause of the first
output was not found.) The switches-off translations were compared by md5 only, not gated again.

## 9. Record of rule candidate T3: kind `goptional` and piece `seqcoll` (branch `main-libgen-t3`, worktree `/mnt/data/ubuntu/jm/wt/libt3`, 2026-11-24)

On top of main c52c9073 (T1 and T2 merged, section 8). Two pieces, each its own switch value of
`J2GO_NOLIBGEN` and its own commit: Guava's `Optional` typed (what 7.6 says is left of it), and
the `SequencedCollection` methods that T1 left in `LibFns.REFUSED` (6.1). Scratch files:
`/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/t3/`
(`kind.sh <label> <this kind> <md5 before> <new kinds>`: under one hold of the lock, rebuild the
translator, translate with this kind off, with both new kinds off and with everything off, then
the default (`J2GO_LIBGEN=2`), go build, go test, self-test, gate; `cdiff.sh A B`: census rows
and the spellings of this stage; `tm.sh`: gast.py; `offall.sh`: verify.sh with the switches off;
`base/`, `out-t3_gopt/`, `out-t3_seq/`: the translations; `main-classes/`: main's translator,
compiled from `git show HEAD:`; `mainbuild/`: main's go tree with main's translation, built as
`$B/bin/m_t3_base`). Every heavy command ran under `flock /mnt/data/ubuntu/jm/heavy.lock`.

### 9.1 Commits

| commit | switch (`J2GO_NOLIBGEN=`) | what |
|---|---|---|
| 1ab3d9a3 | `goptional` | Guava `Optional<T>` is a value library class, `GOptional[T]` (`rt_lg_gopt.go`) |
| 3b2be4dd | `seqcoll` | typed twins of `SequencedCollection.getFirst / getLast / removeLast / addLast` (`LibFns.TYPED_SEQ`, `rt_lgfn.go`) |
| (the commit after) | | this section |

### 9.2 Kind `goptional`

**The form.** 7.6 proposed the form of the kinds (a typed interface = the erased one plus `_t`
methods, wrappers, makers). It is not that: Guava's `Optional` is a class whose runtime form is a
struct, and the translator already has a rule for exactly such a library class, "Optional is a
value" (`Ctx.valueLibs`, `java.util.Optional` as the generic struct `Optional[T]`). The kind adds
Guava's class to that rule (`Ctx.registerValueLibs`, `Ctx.gopt()`: the kind on and rule "function
types" on), so its places, bridges, statics, `== null`, `instanceof`, boxing and conversions
take the paths `java.util.Optional` takes, with no new mechanism:

- The struct: `GOptional[T]` (`rt_lg_gopt.go`, 195 lines, tag `!j2go_nofntypes`): `{value T;
  present, set bool}`, zero value = null, `GOptional_of / absent / fromNullable[T]` generic
  functions that allocate nothing, `optGOptionalAs_` / `optGOptionalOf_` / `isGOptional_` /
  `box_()` as `optAs_` ... are for `java.util.Optional`. Its Go name is `GOptional`
  (`Ctx.className`, kind on), so the erased class `Optional_base` of `rt_base.go` keeps its
  name and is what the translation with the kind off names: both are in the default build. Each
  method does what `Optional_base`'s does, with the same messages (`orNull` gives `null_[T]()`,
  the null String for a `GOptional[string]`). The two classes' objects are never equal (its
  marker interface `goptional_` is not `optional_`).
- The `or` overloads: `LibGen.GOPT_METHODS` (a name list, by `LibGen.sigKey`: owner, name,
  erasures of the parameters) lists the methods the runtime has and gives each its Go name;
  `Ctx.externalMethodName` takes the name from it for Guava's Optional (kind on). So `or(T)` is
  `or` and `or(Optional)` is `or_Optional` by the parameter's erasure, not by how often the
  program uses each overload (the rule of `Ctx.assignExternalNames` for other library overloads,
  which gives the same two names in this tree). `or(Optional<? extends T>)`: the wildcard is
  `any`, so the method is `or_Optional(o GOptional[any]) GOptional[T]` and the caller converts.
  The third overload, `or(Supplier)`, is refused (the runtime has it in neither form).
- A member of a site whose type argument is `any` in Go (a wildcard, an erased type variable):
  `Ctx.atSite` substitutes `Object` for it (kind on, Guava's Optional only), so `get()` on an
  `Optional<? extends Animal>` is `any` and is cast where it is used (found by the self-test: the
  value rule's own substitution gave the bound, `Animal`, to a Go `any`). `transform`'s own type
  parameter is not kept: `transform(f Function_base[T, any]) GOptional[any]`, converted by the
  caller (`optGOptionalAs_`, the one conversion in the tree, `TypedAstDeserializer`).

**Refusals** (`LibGen.check`, kind on; the translation stops; the tree has none of any reason,
every translation exited 0): an unchecked cast to a Guava `Optional<X>` (11: the cast converts the
content, where Java's checks the class only); `==` / `!=` between a Guava Optional and anything
but `null` (12); a call of a method that `GOPT_METHODS` does not list (13: `or(Supplier)`; 14:
`asSet()`); a reference to one of its statics (15: `Optional::fromNullable`, a Go generic
function, which the translator has no type arguments for at a reference; the reference to a
generic static of `java.util.Optional` crashes the translator the same way, untouched).
`selftest/refused/RefusedLib.java` 11 to 15; the self-test fails unless exactly the 15 marked uses
are refused. Counted by reason: 5 reasons, 1 use each in the refused file, 0 in the tree.

**Sites** (`cdiff.sh base out-t3_gopt`, the translator's output `g_*.go`; census.py of `pf5`):

| | main | goptional |
|---|---:|---:|
| `Optional_baseOf[` alias uses | 39 | 0 |
| `*Optional_base` (bridges, interface methods, a getter) | 10 | 0 |
| `Optional_base_*(` calls | 34 | 0 |
| `GOptional[` uses / `GOptional_*[` calls / `optGOptional*` conversions | 0 / 0 / 0 | 47 / 34 / 1 |
| `cast_[` (results of `get` / `orNull` / `or`) | 852 | 844 |
| `obj_(` / `strobj_(` (arguments of `of`, `or`) | 710 / 352 | 699 / 350 |
| `str_(` | 86 | 84 |
| `.box_()` / `any(` | 313 / 158 | 322 / 162 |

`.box_()` +9: a GOptional entering an `Object` (`MoreObjects.toStringHelper(..).add`, 4 in
`CompilerOptions.toString`; the AutoValue `equals` / `hashCode` / `toString` / `requireNonNull` of
`DeserializedAst`, 5) boxes the struct, where `obj_` of the pointer did not allocate (cold code;
see 9.5). `any(` +4: `Optional<Boolean>` holds `any` (a Boolean is `any`), and `GOptional_of[any]`
spells the conversion `any(bool(x))` that `Optional_base_of(bool(x))` left implicit.

Census rows (`census.py`), main -> goptional: interface casts 294 -> 292 (-2), class casts 1,153
-> 1,145 (-8), boxing 1,533 -> 1,533 (0: the four spellings above), unboxing 146 -> 144 (-2),
Optional 1,402 -> 1,368 (-34: the row counts `Optional_\w+(` and the word `Optional`; `GOptional_of[X](`
matches neither, as `Optional_of[X](` of `java.util.Optional` does not either), Java equals/hashCode
protocol 441 -> 443 (+2: `Objects_equals` / `Objects_hashCode` of the box where `equals_` / `hash_`
took the pointer), fields/params declared any 1,295 -> 1,294 (-1).

Runtime test: `rt_lg_gopt_test.go` (`TestLgGOptional`, tag `!j2go_nofntypes`): each method
against `Optional_base`'s (toString, hashCode), equals across instantiations and not with
`java.util.Optional`, the throws. Self-test program `selftest/lang/GOpt.java` (added to
`run.sh`): every listed method, the statics, both overloads of `or`, null and a call on null, the
throws and their messages, `Optional<Boolean>` / `<Integer>`, `Object` and back (`instanceof`,
`Optional<?>` cast, equals, a List of Objects), `Optional<Dog>` for `Optional<? extends Animal>`,
an interface method through its bridge, Optionals as elements of a List and a Map. It prints what
Java prints with the kind on, off, and everything off.

### 9.3 Piece `seqcoll`

`SequencedCollection.getFirst / getLast / removeLast / addLast` join `LibFns` (piece `fns`'s
mechanism; off with it) in a table of their own, `LibFns.TYPED_SEQ` (`J2GO_NOLIBGEN=seqcoll`:
not read, and the calls are reported under `LibFns.REFUSED`'s reason as before). The owner,
`SequencedCollection`, has no typed interface, so the receiver template is the typed interface of
the interface it extends (`LibFns.RECV_AT`: `CollectionT[$0]`): a typed deque or list of rule
"typed collections" enters as its view (`this.blockStack.asCollT()`, no allocation, was
`SequencedCollection(Deque_un_(this.blockStack.asDequeT()))`), a `DequeT` / `ListT` / typed set as
it is; the member's type is taken at the owner with the view's type arguments. The twins
(`rt_lgfn.go`, +47 lines) mirror `rt_coll.go`'s helpers: `getLastT_` reads a `ListT` by index
(the typed deque's view is one, as the erased helper's `List` test finds), else iterates as
`Iterables_getLastT_` does; `removeLastT_` removes by index where the erased one does, else by
`remove_Object` of the boxed element. This is "LibFns.TYPED with receiver templates", the form
T1's 6.5 named; the typed interfaces (T2's form) do not serve, since these methods' root is
`SequencedCollection`, no interface of a kind.

The 72 calls of 6.1 were J2GO_LIBGEN notes, not calls left erased: 30 of the 32 `addLast` notes
were calls on a typed deque that `Ctx.typedMethod` serves (`add`), noted before that test. Calls
of the erased helpers in main's translation: getFirst 13, getLast 15, removeLast 12, addLast 2 =
**42; all 42 are twin calls now, none left erased** (J2GO_LIBGEN=2: 13 / 15 / 12 / 2 "calls of
the typed twin"). Receivers: 34 fields and 7 locals that are typed deques or lists (`asCollT()`;
the 5 Lists of 6.1 are `contextStack` of `FeatureCollector` / `ScriptNodeDeserializer`), 1
`workSet` of `FixedPointGraphTraversal` (a typed set, as it is). No refusal reason is added
(a call that does not fit stays erased with `LibFns.left`'s reasons; the tree has none).

Census rows, goptional -> seqcoll: interface casts 292 -> 290 (-2), class casts 1,145 -> 1,114
(-31), boxing 1,533 -> 1,531 (-2), unboxing 144 -> 142 (-2), fields/params declared any 1,294 ->
1,293 (-1). Spellings: `SequencedCollection(` / `icast_SequencedCollection(` 43 -> 0, `Deque_un_(`
36 -> 0, `List_un_(` 80 -> 75, `cast_[` 844 -> 814, `Node_of_(` 60 -> 58, `obj_(` 699 -> 697.

Self-test: `lang/LibFns.java` `typed()` has 6 more twin calls (a typed deque, a `LinkedHashSet`:
the iterating branch of the twins and `remove_Object`), `seq()` 7 more on `Deque` / `List`
parameters, and an empty deque's `getLast`; `run.sh` counts 39 twins in `typed()` (was 33) with
the twins' names in its pattern.

### 9.4 Byte identity (md5 of `cat go/closure/g_*.go`, the branch's translator, the runtime of the commit)

| translation | must equal | goptional (1ab3d9a3) | seqcoll (3b2be4dd) |
|---|---|---|---|
| this switch off | the commit before's default | 4440048a = main | a3d9a474 = goptional's default |
| both new switches off | main's default 4440048ae31591b5be2bc353c8655e43 | equal | equal |
| everything off (`J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all`) | main's all-off 973b5b6d3e4f71ab1cc82872360f9759 | equal | equal |
| default | | a3d9a47487f38a32f373d245e8b5bdf9 | 627f428904cd1ab870a348b377934bfb |

(main's default was made by main's translator, `main-classes/`, in this worktree: 4440048a, as
section 8 has it.)

### 9.5 Timing (CPU, googleapis 18% subset, `proto/gast.py gasub/flags3500.txt 3`)

Three stages, alternating order, `--num_parallel_threads=1`; output sha `72b57cd286c9` in every
run. Binaries: `m_t3_base` (main's translation with main's runtime, `mainbuild/`), `m_t3_gopt`
(gate binary of 1ab3d9a3), `m_t3_seq` (gate binary of 3b2be4dd). Logs `tm-gopt.log`,
`tm-seq.log`, `tm-all3.log`.

| run | before: mean (min) CPU s | after: mean (min) CPU s | change of the means |
|---|---:|---:|---:|
| goptional: base -> gopt | 159.00 (157.95) | 160.83 (160.22) | +1.2% |
| seqcoll: gopt -> seq | 160.91 (160.05) | 159.65 (159.40) | -0.8% |
| three binaries in one session: base / gopt / seq | 158.89 (158.56) | gopt 160.88 (160.16), seq 159.01 (158.65) | gopt +1.3%, seq +0.1% |

Reading it: the two pieces together (main's translation against the final one, which is what the
translator is measured by) are **neutral** (+0.1% of the means, +0.06% of the minima). The binary
of kind `goptional` alone is slower in all six of its runs (160.16 to 161.73 s against 157.95 to
160.04 for main's), while the final binary, which has all of its changes and the twins on top,
is not: the kind's sites are in cold code (options, conformance, the TypedAST deserializer; the
one per-node site, `ScriptNodeDeserializer` `colorPoolShard.get()`, lost a cast), so the
difference of that one binary is taken for a layout effect of the build, not of the code; it was
not bisected further (no allocation or CPU profile was taken). Piece `seqcoll`'s sites are in hot
code (ReferenceCollector's block stack, PureFunctionIdentifier's function stack, NodeIterators,
IRFactory's visit stack, JSTypeResolver): a twin call on the view replaces the conversion to the
erased interface and a cast of the result; -0.8% against the gopt binary, within about one spread.

### 9.6 Gate lines

Default translation of each commit, `$B/gate.sh libt3 <label>` under the lock (logs
`/mnt/data/ubuntu/jm/logs/gate-t3_gopt.log`, `gate-t3_seq.log`, copies in the scratch directory);
both printed `GATE OK` and these lines (the tree line is the commit before, the work uncommitted):

```
GATE OK t3_gopt
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	(cached)
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 21136  test lines: 4671
runtime lines of the default build: 20892  off-mode extras (build-tagged files): 244
```

`GATE OK t3_seq`: the same lines, with `runtime lines: 21183  test lines: 4671` and `runtime lines
of the default build: 20939`. ("cached": `kind.sh` ran `go test ./closure` just before, `ok ... 1.942s`
and `1.949s`.) Runtime of the default build: main 20,697 (section 8) -> 20,892 (+195,
`rt_lg_gopt.go`) -> 20,939 (+47, the twins in `rt_lgfn.go`); test lines 4,626 -> 4,671
(`rt_lg_gopt_test.go`).

Switches off, on the final code (`offall.sh`: translate with the switch, `tools/prune/verify.sh`
with it; logs `verify-*.log`; the default translation made again afterwards was identical,
627f4289):

```
switches: J2GO_NOLIBGEN=goptional  translation: exit 0 03b59d27bbb4a6a81764934eebbe7682
switches: J2GO_NOLIBGEN=seqcoll  translation: exit 0 a3d9a47487f38a32f373d245e8b5bdf9
switches: J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all  translation: exit 0 973b5b6d3e4f71ab1cc82872360f9759
(each)
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.959s / 1.932s / 1.909s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 21183  test lines: 4671
runtime lines of the default build: 20939  off-mode extras (build-tagged files): 244
```

(`goptional` off on the final code is a new combination, seqcoll on: 03b59d27, compared with
nothing.)

### 9.7 Not done, unverified

- No flip: the census rows above are of the translator's output, not of a flipped tree
  (JAVAISMS6's 34 `Optional_base_*` sites and T1's SequencedCollection sites were counted after
  the post-flip steps); post-flip tools that look for `Optional_base`, `Deque_un_(` or the
  `SequencedCollection_*` helpers meet `GOptional` and the twins.
- No allocation profile: kind `goptional` removes the heap objects of `Optional.of` (the struct is
  a value) and adds a box where a GOptional enters an `Object` (9.2); neither was counted at run
  time.
- Combinations other than "this switch off", "both off" and "everything off" were not
  translated; the switches of the earlier stages were not gated off again.
- `java.util.Optional` has the two gaps found here (a member at a wildcard site gets the bound's
  type; a reference to a generic static crashes the translator); kind `goptional` fixes the first
  for Guava's class only and refuses the second; neither occurs in the tree for `java.util.Optional`
  (it compiles).

## 10. Record of rule candidate "T3" of JAVAISMS6.md: piece `objarg` (branch `main-libgen-t4`, worktree `/mnt/data/ubuntu/jm/wt/libt4`, 2026-11-24)

On top of main ede15525 (section 9). JAVAISMS6.md names this candidate T3; section 9's T3 is
another one (kind `goptional`, piece `seqcoll`), so the branch is `main-libgen-t4`. One piece of
rule "typed library interfaces", switch value `J2GO_NOLIBGEN=objarg`. Scratch files:
`/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/t4/`
(`pipe.sh <label>`: under one hold of the lock, rebuild the translator, translate with the piece
off, with everything off and with the report, then `tools/j2go/run.sh` and the gate; `cdiff.sh A
B`: census rows and this piece's spellings; `tm.sh`: gast.py; `offall.sh`: verify.sh with the
switches off; `basebin.sh`: main's translation with main's runtime, `mainbuild/`, as
`$B/bin/m_t4_base`; `base/` main's default translation, `out-t4_a/` this one; `tx-t4_a-rep.log`
the `J2GO_LIBGEN=2` report). Every heavy command ran under `flock /mnt/data/ubuntu/jm/heavy.lock`.

### 10.1 Commits

| commit | what |
|---|---|
| 369bee50 | piece `objarg`: `LibObj.java`, `rt_lgobj.go`, `rt_lgobj_test.go`, self-test `lang/LibObj.java` |
| 4722cfc2 | the unconverted parameter of a method reference (`memoized::get`) is an argument at its type |
| dc83d1bc | the twins test the view's own type (`*tmapView_[K, V]`); no typed methods on the views (10.5) |
| 3e9cac5b | self-test: no `TreeMap<Dog>` read with an Animal (10.6); README on the twins' form |
| e6df9ca5 | this section (10.1 to 10.7: the first form) |
| e7a2818c | twins only for Map / Set receivers of key types on a checked-in pair list (10.8) |
| (the commit after) | 10.8 |

**The branch's code is 10.8's form; 10.2 to 10.7 record the first one.**

### 10.2 The form

Java declares `Map.get / containsKey / remove(Object)`, `Collection.contains / remove(Object)` and
`List.indexOf / lastIndexOf(Object)` with an Object parameter, so the typed interfaces (rt_lg.go)
have them erased: main called `m.get_t(n.box_())`, `c.contains(obj_(x))`, `l.indexOf(obj_(x))`
on a typed receiver. The piece is the mechanism of piece `fns` (section 6) for these abstract
methods: a name list (`LibObj.TYPED`, by `LibFns.key` of the root method) and typed twins in the
runtime (`go/closure/rt_lgobj.go`, 60 lines, no build tag), chosen in `ExprEmitter.special` right
after `LibFns.call`:

| Java (root) | twin | receiver |
|---|---|---|
| `Map.get(Object)` | `Map_getK_[K, V](m, k) V` | `MapT[K, V]` |
| `Map.containsKey(Object)` | `Map_containsKeyK_[K, V](m, k) bool` | `MapT[K, V]` |
| `Map.remove(Object)` | `Map_removeK_[K, V](m, k) V` | `MapT[K, V]` |
| `Collection.contains(Object)` | `Collection_containsE_[T](c, x) bool` | `CollectionT[T]` |
| `Collection.remove(Object)` | `Collection_removeE_[T](c, x) bool` | `CollectionT[T]` |
| `List.indexOf(Object)` | `List_indexOfE_[T](l, x) int32` | `ListT[T]` |
| `List.lastIndexOf(Object)` | `List_lastIndexOfE_[T](l, x) int32` | `ListT[T]` |

The receiver is a typed interface (`LibGen.typedView`), or a typed collection of rule "typed
collections" whose own typed method does not serve the call (`Ctx.typedMethod` returns null: a
`TList`'s contains / indexOf, a `TDeque`'s contains / remove), which enters as its view
(`LibGen.fromTypedColl`: `.asCollT()`, `.asListT()`). A call that `Ctx.typedMethod` serves
(`TMap.get` of a key, `TSet.contains` of an element: the existing "typed when the argument is a key
by its static type", `Ctx.isElement`) is not this piece's (854 calls, reported as "not this piece").

**Semantics.** Each twin does what the erased call does:

- where the receiver's object is the view of a typed map or set (`*tmapView_[K, V]`,
  `*tsetView_[T]`: a type assertion to the view's own type), it calls the typed map's or set's
  own `get` / `find` / `remove`. That is the view's erased method (`get_t`, `containsKey`,
  `remove_t`, `contains`, `remove_Object`) after it has taken the key out of its box (`tkeyOf_`:
  `x.(K)`, else `kc.unbox`), on the same key: the same `find`, so the same `equals` / `hashCode` of
  the same object for a map or set that compares by equals (`teq_`), `==` otherwise, and the same
  `frozen_` panic of an immutable one;
- any other object (a wrapper `mapAs` / `setAs` / `listAs`, a typed map's key set, a typed list,
  a sorted, deque or enum-set view, an immutable twin) gets the erased method with the argument
  boxed as the erased code boxes a value of its type: `boxT_` (strobj_ for a String, `box_()` for a
  handle, the most derived object, `self()`, for a class, nil for null) - the box the erased call
  made at the call site;
- a null argument (`k == kc.null`) always takes the erased method with nil: what the erased call
  passed (obj_ / strobj_ / box_ of null are nil);
- `List.indexOf / lastIndexOf` have no typed lookup: Java compares by the argument's `equals`, and
  the twin is the erased method with `boxT_(x)` (the box is made in the runtime, not at the call).

The Map and Collection twins take a `comparable` key type (the assertion names the view's
type, `TMap[K comparable, V]`); the translator refuses a key type Go cannot compare
(`LibObj.comparable`). A nil receiver fails the assertion and calls the erased method on nil, as
the erased call did.

The argument is converted to the key type by the translator's own conversion (`Arg.as(K)`), as
for any typed parameter (`put_t`'s key): a subclass argument is its view of the key's class
(`d.up_Animal()`), whose `self()` is the object the erased call boxed. A twin call keeps the
erased method on the erased interface (`noteMethod`, as the `_t` path does), so `g_zstubs.go` is
the same as main's and the runtime's own calls of it compile in every translation.

**Refusals** (the erased call stays, reported by `J2GO_LIBGEN=2` under "Object-parameter methods:
calls left erased (why)"; the translation goes on):

- the argument's static Java type does not fit: it is not the key / element type or a subtype
  (`LibObj.fits`: `Types.isSubtype`, a primitive by its box): `Object`; a supertype; another class
  or interface; the element's class at other type arguments (raw);
- the argument's Go form is not a value of its type: a field or method result whose declared type
  names a type variable (`any` in Go: `as(K)` would cast it, where Java passes the Object without a
  checkcast; found in this tree's `SimpleSetMultimap` iterator, `this.last.key()`); a converted parameter of
  a method reference;
- the key / element type is `any` in Go (Object, an erased type variable) or a wildcard;
- the receiver is erased (no typed view), or a typed collection that is not one of the owner;
- a String that the String-key helpers serve unboxed (`ExprEmitter.stringKeyHelper`:
  `Collection_contains_s`, `Map_containsKey_s`, `Map_remove_s`; `Map.get` of a String is piece
  `fns`' `Map_get_sT_` first);
- `get` / `remove` whose result enters an Object (`lgWantErased`, as `LibFns`: Java casts nothing
  there);
- (Map and Collection) a key type that Go cannot compare: a type parameter without the
  `comparable` constraint (`Ctx.constraint`), a Go func type of rule "function types", a slice, a
  map, a value struct (`Optional`). The tree has none.

### 10.3 Counts (`J2GO_LIBGEN=2` report of the default translation, `tx-t4_a-rep.log`)

Calls of the seven methods (owner a library interface, receiver typed or not):

| | calls |
|---|---:|
| **typed: calls of the twin** | **79** |
| ... `Map.get` 24, `Map.containsKey` 10, `Map.remove` 7, `Collection.contains` 20, `Collection.remove` 10, `List.indexOf` 7, `List.lastIndexOf` 1 | |
| refused: a String the String-key helpers serve (`Collection.contains` 19, `Map.containsKey` 6, `Map.get` 2, `Map.remove` 2) | 29 |
| refused: the receiver is erased (`Map.get` 6, `Map.remove` 4, `Map.containsKey` 2) | 12 |
| refused: the element type is Object in Go (`Map.get` 2, `Collection.contains` 2, `Map.containsKey` 1) | 5 |
| refused: the argument is the result of a method of a type variable's type (`Map.get`, `Map.remove`, `Collection.remove` 1 each) | 3 |
| refused: the argument's static type is a supertype of the element type (`Collection.remove`) | 2 |
| refused: the result enters an Object (`Map.get`) | 1 |
| refused: the argument's static type is Object / another class or interface / other type arguments | 0 / 0 / 0 |
| not this piece: the typed collection's own method (`Map.get` 392, `Collection.contains` 290, `Map.containsKey` 122, `Collection.remove` 28, `Map.remove` 22) | 854 |

JAVAISMS6.md's row 15 has RULE T3 62 = 50 "Object parameter of a typed collection method" + 12
"erased element argument of a collection method". The 50 are sites of the sixth flip's tree
(tree6h, other line numbers); by file, every one of their 21 files has at least as many twin calls
here as it has sites (50 of 50 by that count; not matched line by line). Of them, the one that
needed the second commit: `TypedScopeCreator`'s `this.memoized::get` (a method reference). The 12
"erased element argument" sites (`json.Put("reason", obj_(..))` of maps whose value type is Object,
`externs.add_int_Object(0, obj_(input))`, `hasher.putUnencodedChars(obj_(&name))`) are values that
enter a parameter of type Object or of an erased type: not methods with an Object parameter of a
typed collection, not attempted.

Spellings and census rows (`cdiff.sh base out-t4_a`, the translator's output `g_*.go`, main's
default 627f4289 -> 6f59da7c; census.py of `pf5`): **boxing to any 1,531 -> 1,478 (-53)**, every
other row unchanged. `obj_(` 697 -> 663, `.box_()` 322 -> 305, `boxT_(` 88 -> 73, `strobj_(` 350 ->
348, `.self()` 1,273 -> 1,270, `.up_X()` 1,723 -> 1,725 (a subclass argument at the key's class);
`.get_t(` 68 -> 44, `.remove_t(` 8 -> 1, `.containsKey(` 151 -> 141, `.contains(` 392 -> 372,
`.remove_Object(` 13 -> 3, `.indexOf(` 8 -> 1, `.lastIndexOf(` 1 -> 0; `.asCollT()` 105 -> 114 /
`.asListT()` 305 -> 296 (a typed list's `contains` takes the Collection twin). 79 lines of 34
files differ (one call each); `g_zstubs.go` and `g_0casts.go` are identical.

Self-test `selftest/lang/LibObj.java` (added to `run.sh`): receivers that are typed interfaces
(parameters of methods with a type parameter, which rule "typed collections" leaves to the typed
interfaces) given typed maps, sets, lists, deques, a key set, sorted (TreeMap / TreeSet with a
comparator that takes null), immutable and erased-wrapped collections; keys of the element type, of
a subtype, null (present and absent), keys equal by `equals` but other objects (`Key`), a key of a
type parameter of a generic class (`Reg<T>`: `MapT[T, any]`), Strings in a list, a typed list's
and deque's own view, a method reference `m::get`. `run.sh` counts 51 twin calls in `typed()` and
the methods it calls and, in `wide()` / `erased()` / `wild()` / `obj()`, no twin and 13 erased
calls (an Object, a supertype, another interface; Object keys; a wildcard; a result that enters an
Object; a field of a type variable's type). Both print what Java prints. Go test
`rt_lgobj_test.go`: each twin against the erased method with the boxed argument on a native view
and on a wrapped erased collection, present / absent / null, and the changing methods on two equal
collections that must end equal.

### 10.4 Byte identity (md5 of `cat go/closure/g_*.go`, this branch's translator, `pipe.sh`)

| translation | must equal | 4722cfc2, dc83d1bc, 3e9cac5b |
|---|---|---|
| `J2GO_NOLIBGEN=objarg` | main's default 627f428904cd1ab870a348b377934bfb | equal |
| `J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all` | main's all-off 973b5b6d3e4f71ab1cc82872360f9759 | equal |
| default (`tools/j2go/run.sh`) | | 6f59da7cc5054beddb5e4dc4e1cc4579 |

(main's default 627f4289 was made by main's translator in this worktree before the first change.)

### 10.5 Timing (CPU, googleapis 18% subset, `proto/gast.py gasub/flags3500.txt 3`) and the binary

`m_t4_base` (main's translation 627f4289 with main's runtime, `mainbuild/` from `git archive
main go`) against `m_t4_b` (gate binary of dc83d1bc; 3e9cac5b changes only the self-test and the
README, the same translation and runtime), alternating, `--num_parallel_threads=1`, output sha
`72b57cd286c9` in all six runs (`tm-t4.log`):

| binary | runs (CPU s) | mean | min |
|---|---|---:|---:|
| m_t4_base | 162.57, 161.16, 162.12 | 161.95 | 161.16 |
| m_t4_b | 163.20, 161.58, 161.94 | 162.24 | 161.58 |

+0.18% of the means, +0.26% of the minima: **neutral** (inside each binary's spread, 1.4 s). No
allocation or CPU profile was taken.

The binary grows: 122,878,234 -> 126,753,581 bytes (+3.9 MB; `.text` 23,269,376 -> 24,023,040,
+754 KB). Not the twins themselves (their instantiations are about 25 KB: `Map_getK_` 10.0 KB, `Map_containsKeyK_` 10.0 KB, `Collection_containsE_` 4.7 KB ...): the assertion
`m.(*tmapView_[K, V])` / `c.(*tsetView_[T])` names the view type at the key types of the calls,
and where the program had no typed map or set of those types Go instantiates the whole typed map
or set for them (`tmapView_` 244 -> 249 instantiations, `tsetView_` 168 -> 193, with `TSet`,
`tsortedSetView_`, the entry and key-set views: 7,385 new symbols, 722 KB). Those fast paths can
never be taken (no such collection exists), but the linker cannot know. The first form (369bee50,
methods `get_k` ... on the views, found by an assertion to an interface with a type parameter in
it) was +2.0 MB (+423 KB text): Go keeps such a method for every instantiation of the view types.
Not tried: giving the Collection twins the set test only when the receiver's Java type is a Set
(the 25 new `tsetView_` instantiations are probably from `contains` on lists and collections; not
checked), which might remove most of the growth.

### 10.6 Gate lines

Default translation of 3e9cac5b (`pipe.sh t4_c`: committed, then `tools/j2go/run.sh`, then
`$B/gate.sh libt4 t4_c` under one hold of the lock; log `/mnt/data/ubuntu/jm/logs/gate-t4_c.log`,
copy in the scratch directory). The log's first line:
`tree: 3e9cac5b2 piece objarg: self-test without a Dog TreeMap read with an Animal; README on the  dirty: 1`
(the one entry: `?? .cache`, the symlink to `/mnt/data/ubuntu/jm/cache-libt4`).

```
GATE OK t4_c
tests before: 50
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	2.017s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 21243  test lines: 4778
runtime lines of the default build: 20999  off-mode extras (build-tagged files): 244
tests after: 50
```

Runtime of the default build: main 20,939 (section 9) -> 20,999 (+60, `rt_lgobj.go`); test lines
4,671 -> 4,778 (`rt_lgobj_test.go`). The earlier commits were gated the same way: `GATE OK t4_a`
(4722cfc2, the first form: runtime 21,055) and `GATE OK t4_b` (dc83d1bc), both `dirty: 1` and
the same selftest / ok / four MATCH lines.

Switches off, on 3e9cac5b (`offall.sh`: translate with the switch, `tools/prune/verify.sh` with
it; logs `verify-*.log`; the default made again afterwards was 6f59da7c):

```
switches: J2GO_NOLIBGEN=objarg  translation: exit 0 627f428904cd1ab870a348b377934bfb
switches: J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all  translation: exit 0 973b5b6d3e4f71ab1cc82872360f9759
(each)
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.981s / 1.923s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 21243  test lines: 4778
runtime lines of the default build: 20999  off-mode extras (build-tagged files): 244
```

The same run on dc83d1bc failed the self-test with everything off: the erased translation of
`LibObj.wide`'s second call (a `TreeMap<Dog>` made with a `Comparator<Animal>`, read with an
Animal) panicked with `interface conversion: interface {} is *closure.LibObj_Animal, not
*closure.LibObj_Dog`, where Java compares two Animals. That translation is main's translator's
(the all-off output of the tree is main's, 973b5b6d), so the gap is in the erased translation,
not in this piece; where exactly it casts was not looked for. 3e9cac5b gives that call a
LinkedHashMap; the self-test passes in all three modes (st3.out).

### 10.7 Not done, unverified, overlaps

- No flip: the counts are of the translator's output; JAVAISMS6's 50 sites were matched by file,
  not line by line. Post-flip tools that look for `get_t(x.box_())` / `containsKey(obj_(` meet
  the twins.
- No allocation or CPU profile. The twins remove the box at the call (`Node.box_()`, `obj_`) and,
  on a typed map's or set's view, the `x.(K)` / `kc.unbox` of the erased method; they add a
  type assertion per call; the list twins box in the runtime as before. The binary is 3.9 MB
  larger (10.5).
- No self-test case for the refusal "the element type is not comparable in Go" (the tree has
  none).
- The 12 "erased element argument" sites of RULE T3 (values of type Object) are not attempted.
- Combinations other than "this switch off" and "everything off" were not translated; the
  earlier pieces' switches were not gated off again.
- Overlap with the concurrent branch `main-libgen-t5` (typed Comparable), where both touch:
  `ExprEmitter.special` (4 lines: `toObject` kept before `LibFns.call`, the `LibObj` call after
  it), `Ctx.java` (the field `lo` after `lf`), `selftest/run.sh` (the class list, the
  `SelfTest()` line, a block of its own), `tools/j2go/README.md` (the switch list and one
  paragraph). Everything else is in new files (`LibObj.java`, `rt_lgobj.go`, `rt_lgobj_test.go`,
  `selftest/lang/LibObj.java`).

### 10.8 Revision: Map / Set receivers only, and a pair list (e7a2818c)

**10.2 to 10.6 record the first form** (79 twin calls, +3.9 MB of binary); the branch's code is
this form. Asked to bring the binary's growth near 0, I tried first what 10.5 named (choose the
twin by the receiver's static type; find the view through a method with no type parameter in its
signature, `tmap_() unsafe.Pointer`, and call the typed map's own lookup): +2.5 MB, .text +407 KB.
The rest came from Map receivers whose key / value types no typed map has (they hold wrappers
only, so the typed lookup could never be taken): calling the typed map's lookup at those types
made Go compile the typed map for them (14 new `TMap` dictionaries with their views). Typed
methods on the views instead (`find_k` / `remove_k`): +3.1 MB / +463 KB (Go keeps a method whose
signature names K for every instantiation of the view). The floor, twins that only box in the
runtime: +58 KB / +18 KB, but that only moves the box out of the census, so it was not taken.

The form now (coordinator's decision):

- **By the receiver's static type.** A Map receiver gets `Map_getK_` / `Map_containsKeyK_` /
  `Map_removeK_`, a Set receiver (`Collection.contains / remove` on a `Set`) `Set_containsE_` /
  `Set_removeE_`. Each finds the typed map's or set's view through `tmap_()` / `tset_()` and calls
  its own `find` / `get` / `remove` (a sorted view embeds the view and has the method too); else
  the erased method with `boxT_`; null as before. A List or Collection receiver (`List.indexOf /
  lastIndexOf`, `contains` / `remove` on a List, a Deque, a Collection) has no typed lookup: the
  call stays erased, box and all, at the call site (new refusal reason). The Collection and List
  twins are gone.
- **A checked-in name list, `tools/j2go/objarg-pairs.txt`** (18 pairs: `map Node, *TypedScope`,
  `set *JSType`, ...; type parameters too, `map K, SetT[V]`, `set V`): the key / element types
  for which the translation makes a typed map or set (recorded at `Ctx.tconvsOf`, which every
  constructor and conversion to a typed collection goes through). A twin is called only for a
  pair on it; any other pair keeps the erased call (new refusal reason). The self-test adds
  `tools/j2go/selftest/objarg-pairs.txt` (7 pairs of `lang/*.java`) through
  `J2GO_OBJARG_PAIRS` (paths separated by `:`).
- **Stale list fails.** `LibObj.finish` (end of `Main`) compares the list with the pairs the calls
  asked for that have a typed map or set: a pair on the list that is not one of them, one of them
  not on the list, or a twin called for a pair not on the list fails the translation (exit 3,
  `objarg: the pair list is stale: ...`; `tools/j2go/run.sh` fails with it, the self-test checks
  its own translation's log). With another rule off (`J2GO_NOLIBGEN` / `J2GO_NOFNTYPES` set) it
  only reports. `J2GO_OBJARG_PAIRS_OUT=<file>` writes the list as it should be; both lists were
  generated that way. Tried (`stale.sh`): the list plus `map *Foo, *Bar` -> exit 3 ("on the list
  with no typed map or set or no call [map *Foo, *Bar]"); the list without `map Node, *TypedScope`
  -> exit 3 ("asked for, with a typed map or set, not on the list [map Node, *TypedScope]"); the
  list as checked in -> exit 0, the same translation.

**Counts** (`J2GO_LIBGEN=2`, `tx-t4_d-rep.log`): **51 twin calls** (`Map.get` 20, `Map.containsKey`
10, `Map.remove` 6, `Set.contains` 12, `Set.remove` 3). Refused 80: a String the String-key
helpers serve 29; **a List or Collection receiver 23** (`Collection.contains` 8, `Collection.remove`
7, `List.indexOf` 7, `List.lastIndexOf` 1); the receiver is erased 12; **no typed map or set of
these types 5** (`Map.get` 4, `Map.remove` 1); the element type is Object in Go 5; a method
result of a type variable's type 3; a supertype 2; the result enters an Object 1. Not this piece
854. Of the first form's 79: 23 + 5 are erased again, 79 - 28 = 51.

**Census** (`cdiff.sh base out-t4_d`): boxing to any 1,531 -> 1,496 (**-35**), every other row
unchanged; `obj_(` -20, `.box_()` -15, `boxT_(` -12, `.get_t(` -20, `.remove_t(` -6,
`.containsKey(` -10, `.contains(` -12, `.remove_Object(` -3.

**md5** (`pipe.sh t4_d`): `J2GO_NOLIBGEN=objarg` 627f428904cd1ab870a348b377934bfb (main's
default); everything off 973b5b6d3e4f71ab1cc82872360f9759 (main's all-off); default
870508e1a9a9e10603ea0ba173374b86.

**Binary** (`m_t4_d`, gate binary of e7a2818c, against `m_t4_base` 122,878,234 bytes, .text
23,269,376): 123,787,434 bytes (**+909 KB**), .text 23,435,217 (**+166 KB**). Not d3's +18 KB:
the twins' own instantiations (311 symbols, 49 KB: the twins in generic classes are instantiated
per instantiation of the class), their dictionaries (22 KB), `tmap_` / `tset_` on every view
instantiation (628 small methods), and the typed maps' `find` / `get` / `remove` compiled at
shapes that the program had not called them at (`TMap[*InputId, *CompilerInput]` and the
`SimpleSetMultimap` maps of sets). A pair whose Go text names a class's type parameter (`map K,
SetT[V]`) is checked by its text, so a typed map of another class with type parameters of the same
names counts for it (not seen to happen; it would only cost size).

**Timing** (CPU, flags3500 subset, 3 rounds alternating, `tm-t4d.log`, sha `72b57cd286c9` in all
six runs): `m_t4_base` 162.61, 160.14, 162.30 (mean 161.68, min 160.14); `m_t4_d` 161.51, 161.31,
162.48 (mean 161.77, min 161.31): +0.06% of the means, **neutral**.

**Gate** (`$B/gate.sh libt4 t4_d` after commit and `tools/j2go/run.sh`; log's first line `tree:
e7a2818ca piece objarg: twins only for Map / Set receivers of key types that have a typed   dirty:
1`, the one entry `?? .cache`):

```
GATE OK t4_d
tests before: 50
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.947s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 21263  test lines: 4761
runtime lines of the default build: 21019  off-mode extras (build-tagged files): 244
tests after: 50
```

Switches off, on e7a2818c (`offall.sh`, `tools/prune/verify.sh` with the switch; the default made
again afterwards was 870508e1):

```
switches: J2GO_NOLIBGEN=objarg  translation: exit 0 627f428904cd1ab870a348b377934bfb
switches: J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all  translation: exit 0 973b5b6d3e4f71ab1cc82872360f9759
(each)
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.952s / 1.965s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 21263  test lines: 4761
runtime lines of the default build: 21019  off-mode extras (build-tagged files): 244
```

Self-test (`lang/LibObj.java`, unchanged): `run.sh` now counts 25 twin calls and 28 erased calls
in `typed()` and the methods it calls, and in `wide()` / `erased()` / `wild()` / `obj()` no twin
and 13 erased calls. Go test `rt_lgobj_test.go`: the Map and Set twins against the erased methods
on a typed map's / set's view, a typed map's key set and wrapped erased collections.

Not done: no self-test case for "not comparable"; the stale check was tried by hand (above), not in
the self-test; JAVAISMS6's 50 sites were not matched again (of the first form's coverage, the
List / Collection receivers and the 5 pairs without a typed map or set are erased again).

## 11. Record of rule candidate T4, Comparable half: piece `cmp` (branch `main-libgen-t5`, worktree `/mnt/data/ubuntu/jm/wt/libt5`, 2026-11-24)

On top of main ede15525 (T3 merged, section 9). (Section 10 is left to the concurrent branch
`main-libgen-t4`, typed `Map.get` / `contains` with Object parameters.) Task: the Comparable part
of rule candidate T4 of `docs/javaisms6/JAVAISMS6.md` (63 sites of the sixth flip's tree): a typed
`ComparableT[T]` with `compareTo_t(T)`, implemented by the classes that implement `Comparable<T>`
and called by the runtime's typed natural orders instead of the erased `compareTo(any)`; then
whether the post-flip tool `deadbridge` would delete the bridges. Scratch files:
`/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/t5/`
(`pipe.sh <label>`: under one hold of the lock, rebuild the translator, translate with the piece
off, with everything off, then the default, go build / test, self-test; `offall.sh`: verify.sh
with the switches off; `basebin.sh`: main's translation with main's runtime as
`$B/bin/m_t5_base`; `tm.sh`: gast.py; `out-a1/`, `out-a1-off/`: the default translation and the
one with the piece off; `db/`: the deadbridge scratch package; `tm.out`, `offall.out`,
`run-t5a.log`). Every heavy command ran under `flock /mnt/data/ubuntu/jm/heavy.lock`.

### 11.1 Commits

| commit | switch (`J2GO_NOLIBGEN=`) | what |
|---|---|---|
| 4368ec45 | `cmp` | classes implementing `Comparable<C>` of their own type have `compareTo_t` and the bridge (`LibCmp.java`, `rt_lgcmp.go`) |
| (the commit after) | | this section |

### 11.2 The form

The mechanism is the one the rule already has for program classes implementing `Iterator<T>`
natively (`next_t` and the bridge `next`, `Emitter.lgBridge`), for one more interface, decided by
its own class (`LibCmp`, one field in `Ctx`, a few lines each in `Emitter.method` and
`ExprEmitter`'s method call):

```go
func (this *DiagnosticType) compareTo_t(diagnosticType *DiagnosticType) int32 {
	return String_compareTo(this.key_, diagnosticType.key_)
}

func (this *DiagnosticType) compareTo(p0 any) int32 {
	return this.compareTo_t(cast_[*DiagnosticType](p0))
}
```

(was one `compareTo(diagnosticType_ any)` whose first statements were the same cast). The bridge
is javac's `compareTo(Object)`; the cast is the one the method did on entry, in the same place
(before anything else), so every caller of the erased method sees the same behaviour.

Runtime (`rt_lgcmp.go`, 26 lines, untagged): `type ComparableT[T any] interface { compareTo_t(T)
int32 }` and `cmpTypedT_[T](a, b T) (int32, bool)`: when `a` is a non-null value whose type has
`compareTo_t(T)`, it calls it; else (a null `a`, which `compareNatural_` reports itself; a `T`
that is an interface or `any`; any other type) it answers false and the caller does what it did.
Called, before the boxes are made for `compareNatural_`, by the typed natural orders: `naturalT_`
(the typed sorts of piece `fns`: `Collections_sortT_`, `List_sortT_` without a comparator),
`tsortCmp_` (the typed sorted sets and maps of rule "typed collections", natural ordering),
`Comparator_naturalOrder[T]`, `Comparator_comparing[T, U]` (the key). For a class of the piece
`compareNatural_(box(a), box(b))` called `a.compareTo(b)`, the bridge, whose cast of `b` (a `T`,
boxed by `self()`, which is the object itself for a class without subclasses) is the identity: the
same body runs on the same two objects. Erased code (`compareNatural_`, `sortSlice_`, the erased
`TreeSet`, Guava's builders, `Ordering`, `ComparisonChain`) calls the bridge as before. A call
`x.compareTo(y)` of the program on a receiver of such a class is `x.compareTo_t(y)` (argument at
its type, no `obj_`).

**Classes typed and left erased** (`LibCmp.reason`, counted in the `J2GO_LIBGEN` report of the
default translation, `tx-a1.log`; the tree's translation, `libgen:` lines):

| | classes |
|---|---:|
| typed: `Comparable<C>` of the class itself, no subclass, not a handle, `compareTo` implementing nothing else | **10** |
| left erased: `Comparable` of another type than the class (a supertype or another instantiation): `JSDocInfo.Property<T> implements Comparable<Property<Object>>` | 1 |
| left erased: raw `Comparable` | 0 |
| left erased: `Comparable` through a superclass or an interface of the program | 0 |
| left erased: the class has a subclass | 0 |
| left erased: a handle class | 0 |
| left erased: `compareTo` also implements another interface's method (which needs the erased form) | 0 |

The 10: `DefaultNameGenerator.CharPriority`, `DiagnosticType`, `DisambiguateProperties.
PropertyReferenceIndexJson / PropertyReferenceJson / TypeEdgeJson`, `SerializeTypesToPointers.
TypeMismatchJson`, `CheckRequiresSorted.DestructuringBinding / ImportStatement`,
`IRFactory.LineAndColumn` (a record), `CodeReplacement`: the same 10 classes whose `compareTo`
bridges the seventh flip's tree has (11.5). A refusal here leaves the class as it was (the tree
must translate); it is not a translation error. The tree has no class of the other five reasons;
the self-test has one of each (11.4).

**Call sites.** In the program: 0 (no `x.compareTo(y)` on a receiver of one of the 10 classes;
the 29 `.compareTo(` calls of the Java tree are on Strings, BigIntegers, enums). In the runtime:
the typed natural orders above, which the tree reaches for these classes at 3 places:
`Collections_sortT_[*CheckRequiresSorted_DestructuringBinding]`,
`Collections_sortT_[*CheckRequiresSorted_ImportStatement]`, and
`SimpleSetMultimap_createSorted[*DiagnosticType, string]` (a typed sorted map, `tsortCmp_`).
The others reach the bridge through erased code: `ImmutableSortedSet.naturalOrder()` builders (5
places: the three DisambiguateProperties classes, TypeMismatchJson, CodeReplacement; builders not
typed, 7.6), `ImmutableSortedSet.copyOf` of CodeReplacements (1), `Arrays.sort(Object[])` of
CharPriority (2, erased: `Arrays_sort` has no twin), `TreeRangeSet` of LineAndColumn (1).

### 11.3 Counts (the translator's output, `g_*.go`; main's default = this translation with the piece off)

`J2GO_NOLIBGEN=cmp` against the default: 10 files differ, 80 diff lines; per class the method's
two entry lines (`var x *C = cast_[*C](x_)`, `_ = x`) leave the method, and a 3-line bridge
follows it. Spellings: `compareTo_t(` 0 -> 20 (10 declarations, 10 calls in the bridges),
`) compareTo(` declarations 11 -> 11, `cast_[` 814 -> 814 (the cast moved into the bridge),
`obj_(` 1,055 -> 1,055. **Census rows (`census.py` of `pf5`): no row changes** (both
translations give the same census output). The rule removes no site of the translator's
output: what it changes is which method the typed sorts run (11.2) and, for the flipped tree,
that the classes have the typed method under the translator's name (`compareTo_t`, the
`implsplit` step's being `compareToT_`).

Runtime: default build 20,939 (section 9) -> 20,982 (+26 `rt_lgcmp.go`, +17 net (19 added, 2 removed) in `rt_fn.go`,
`rt_lgfn.go`, `rt_tsorted.go`); test lines 4,671 -> 4,755 (`rt_lgcmp_test.go`: `TestLgCmp`,
`cmpTypedT_`, null and `any` receivers, a typed sorted set; `rt_lgcmp_fn_test.go`, tag
`!j2go_nofntypes`: `TestLgCmpFn`, `naturalT_`, `Comparator_naturalOrder`, `Comparator_comparing`;
each test counts which of `compareTo_t` and the bridge ran).

### 11.4 Self-test, refusals

`selftest/lang/Cmp.java` (added to `run.sh`): `Ver` and generic `Box<T> implements
Comparable<Box<T>>` (typed); `Raw implements Comparable` (raw), `Kid extends Parent implements
Comparable<Parent>` (a supertype), `Base implements Comparable<Base>` with subclass `Derived`
overriding `compareTo` (subclass; Derived: through a superclass), `NamedImpl implements Named`,
`interface Named extends Comparable<Named>` (through an interface of the program), `Both
implements Comparable<Both>, Ord<Both>` with `interface Ord<T> { int compareTo(T o); }` (also
implements another method). It sorts with `Collections.sort`, `List.sort(null)`,
`Comparator.naturalOrder()` and `.reversed()`, `Comparator.comparing`, a `TreeSet` and a
`TreeMap`, `Arrays.sort(Object[])`, calls `compareTo` directly, through `Comparable` and raw
`Comparable` (bridge, and its `ClassCastException`), and prints what Java prints with the piece
on and off. `run.sh` (piece on) counts in the generated `g_Cmp.go`: 2 `compareTo_t`
declarations, 2 bridges (`return this.compareTo_t(cast_[*Cmp_X](p0))`), 7 `compareTo`
declarations in all (the two bridges, Base's dispatching stub and four erased methods; Derived's
is `compareTo_impl`), 1 call of `compareTo_t` in `direct()`; it fails otherwise. So the six
refused classes are checked to keep the erased method alone.

Found while writing it, not this piece's subject and not changed: an object of `Both` given for
`Ord<Both>` fails its cast in Go (with the piece on or off): the class's `compareTo` implements
`Ord.compareTo` too, and the class has no Go method of the name Ord's group has
(`compareTo_Object`). The tree has no such class; the self-test does not make that call.

### 11.5 `deadbridge` (post-flip step 481/482), checked without a flip

`deadbridge` deletes a bridge `m(any) { return this.mT_(casts) }` when no value of its class ever
becomes an interface value (no conversion to an interface or `any`, no generic function given
one, no use of its `tc_p_X` element conversion). Two runs of the tool as of the seventh flip
(`$B/wt/flip7/tools/postflip/deadbridge`, built into the scratch directory, `-n -v`: report
only, nothing written):

- On the seventh flip's tree (`$B/wt/tree7/go/closure`, which has `implsplit`'s `compareToT_`
  and bridges): **0 deleted**; left: the 10 `compareTo` bridges of 11.2 (9 "its element
  conversion tc_p_ boxes its values for erased code", 1 `LineAndColumn` "a value given to a
  generic function") and 4 `compare` bridges (`JSChunkGraph.InverseDepthComparator`: "a method of
  it returns an interface"; `SymbolTable` 5, 6, 7: "assigned to an interface").
- On this branch's translation (a copy of `go/closure` with the default `g_*.go` and the
  default-build runtime, `compareTo_t` renamed `compareToT_` in the copy so that the tool's
  pattern finds the bridges): **0 deleted**; left: the same 10, for the same reasons (9 `tc_p_`,
  1 generic function).

So **deadbridge would remove 0 bridges** after this piece. The reasons are not the callers the
piece changes (the runtime's sorts) but the classes' values themselves: each class is an element
of a typed collection whose `tc_p_` conversion boxes it for erased code, or is given to a generic
function, and erased code does reach the bridge for 8 of the 10 (11.2: Guava's sorted builders,
`Arrays.sort`, `TreeRangeSet`). The bridges are live in this tree, not only unproven dead. The
`compare(any, any)` bridges are of classes extending Guava's `Ordering` (erased, a runtime class)
or anonymous `Comparator`s assigned to erased places; the rule "function types" already types the
classes implementing `Comparator` directly. Neither is changed here.

### 11.6 Byte identity (md5 of `cat go/closure/g_*.go`)

| translation | md5 | must equal |
|---|---|---|
| piece off (`J2GO_NOLIBGEN=cmp`) | 627f428904cd1ab870a348b377934bfb | main's default: equal |
| everything off (`J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all`) | 973b5b6d3e4f71ab1cc82872360f9759 | main's all-off: equal |
| default (`bash tools/j2go/run.sh` of the commit) | 12b2508cba35deb70850a939b75bcdd0 | (new) |

### 11.7 Gate lines

Commit 4368ec45, then `bash tools/j2go/run.sh` (exit 0, md5 12b2508c), then
`$B/gate.sh libt5 t5_cmp` under the lock (log `/mnt/data/ubuntu/jm/logs/gate-t5_cmp.log`):

```
GATE OK t5_cmp
tree: 4368ec455 rule "typed library interfaces": piece cmp: classes implementing Comparable<C> o  dirty: 1
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	2.011s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 21226  test lines: 4755
runtime lines of the default build: 20982  off-mode extras (build-tagged files): 244
```

(`dirty: 1` is the untracked `.cache` link.) Switches off, on the committed code (`offall.sh`:
translate with the switch, `tools/prune/verify.sh` with it; the default translation made again
afterwards was identical, 12b2508c):

```
switches: J2GO_NOLIBGEN=cmp  translation: exit 0 627f428904cd1ab870a348b377934bfb
switches: J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all  translation: exit 0 973b5b6d3e4f71ab1cc82872360f9759
(each)
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	2.018s / 1.939s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 21226  test lines: 4755
runtime lines of the default build: 20982  off-mode extras (build-tagged files): 244
```

### 11.8 Timing (CPU, googleapis 18% subset, `proto/gast.py gasub/flags3500.txt 3`)

Three rounds, alternating order, CPU first; output sha `72b57cd286c9` in all six runs (log
`tm-t5.log`, summary `tm.out`). `base` = main's translation (md5 627f4289) with main's runtime,
built by `basebin.sh` as `$B/bin/m_t5_base`; `new` = the gate binary of 4368ec45, `$B/bin/m_t5_cmp`.

| binary | runs (CPU s) | mean | min |
|---|---|---:|---:|
| base | 161.35, 159.64, 160.71 | 160.57 | 159.64 |
| new | 162.07, 160.68, 162.53 | 161.76 | 160.68 |

+0.7% of the means, +0.65% of the minima, new slower in 2 of 3 rounds; within the spread of one
binary in a session (1.7 s here, about 1.5 s in 7.4), so taken as **neutral, not shown faster**.
The piece changes no code path of the subset that matters: the typed natural orders reach a class
of the piece only in `CheckRequiresSorted` (a lint, off) and `AllowlistWarningsGuard` (not in the
subset's flags, as far as read); every other typed natural comparison now makes one more type
assertion before boxing, which is the likely cost if the difference is real (not profiled).

### 11.9 Overlaps, not done, unverified

- Overlap with `main-libgen-t4` (typed `Map.get` / `contains`, `LibObj.java`, `rt_lgobj.go`):
  both add one field to `Ctx` after `lf` (adjacent lines: a textual conflict at a merge, both
  kept), both edit `selftest/run.sh` (the class list and `SelfTest()` line, and a check block each)
  and `tools/j2go/README.md` (a bullet each, the switch list); `ExprEmitter.java` is edited by both
  in different methods. Everything else is in separate files (`LibCmp.java`, `rt_lgcmp*.go`,
  `selftest/lang/Cmp.java`).
- The rest of T4's 63 sites (classes implementing `Iterator<T>`, `Collection`, `CacheLoader`,
  `Supplier`, `Function.apply`, the `Ordering` / anonymous `Comparator` `compare` bridges) is not
  touched.
- No flip: the census of a flipped tree was not made. `implsplit` would leave the 10 classes
  alone (their `compareTo` no longer starts with its parameter's cast: "left, counted"); this
  was read from the tool's comment, not run. Post-flip tools that look for `compareToT_` (`tcmp`,
  `deadbridge`) meet `compareTo_t`.
- No allocation or CPU profile. `cmpTypedT_` converts `a` to an interface for the type
  assertion on every typed natural comparison that is not of two Strings (no allocation for a
  pointer; for other shapes the conversion does not escape) — not measured beyond 11.8.
- Combinations of switches other than "piece off" and "everything off" were not translated; the
  earlier pieces' switches were not gated off again.

## 12. Record of rule candidates R4 and R5 of JAVAISMS6.md: pieces `ownwild` and `nbox` (branch `main-libgen-t6`, worktree `/mnt/data/ubuntu/jm/wt/libt6`, 2026-11-24)

On top of main 1f92c420 (translation unchanged from f47fbe67, md5 fab9e90c). R4: `SourcePosition<T>`
and `AbstractCommandLineRunner.FlagEntry<T>` were erased (41 sites); R5: tri-state Boolean / Integer
fields, their setters and the 38 command-line option closures `Consumer[any](func(v any) ...)` (56
sites). Scratch: `/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/t6/`
(`hv.sh`: one heavy job = `flock heavy.lock systemd-run --user --scope -p MemoryMax=20G -p
MemorySwapMax=0 env GOENV=$B/goenv-c1`, i.e. `GOFLAGS=-gcflags=jsgomech/closure=-c=1`, the closure
package's backend serial, object code unchanged: every go build / test / gate and both timed binaries
were built so; `both.sh <label> <piece>`: `run.sh` + `gate.sh` + `offall.sh`; `basetm.sh`: base
binary + timing; `out-*`: translations, `*.census`: census output).

### 12.1 Commits

| commit | switch (`J2GO_NOLIBGEN=`) | what |
|---|---|---|
| 4a1df420 | `ownwild` | rule "typed generics": a `C<?>` in C's own code holding a C of the receiver's instantiation is `C<T>` (`GenScan.ownWild`, `tools/j2go/ownwild-equals.txt`, `selftest/lang/OwnWild.java`) |
| f6fa4111 | `nbox` | rule "typed library interfaces": fields and parameters declared as a boxed primitive hold `Nbox[T]` (`LibNbox.java`, `rt_nbox.go`, `selftest/lang/TriBox.java`) |
| (the commit after) | | this section |

### 12.2 Piece `ownwild` (R4)

Why the two classes were erased (`J2GO_GENERICS=2`, every reason): only a wildcard in their own code.
`SourcePosition.isSamePositionAs(SourcePosition<?> that)` (SourcePosition.java:139) and in
`FlagEntry.equals`, `FlagEntry<?> that = (FlagEntry<?>) o` (AbstractCommandLineRunner.java:3587-3589).
jsgo has both generic: `internal/rhino/static_source_file.go:189` `SourcePosition[T comparable]`,
whose `IsSamePositionAs(that *SourcePosition[T])` (:308) is this piece's parameter form, and
`internal/cli/command_line_config_specs.go:184` `AbstractCommandLineRunner_FlagEntry[T
abstractCommandLineRunner_flag]`, whose `Equals` (:195) asserts `o.(*AbstractCommandLineRunner_FlagEntry[T])`
with ok, this piece's equals form.

The rule is general for the parameter form and a name list for the equals form:

- A parameter `C<?>` of an instance method of C that overrides nothing and that nothing overrides,
  never assigned, never referenced with `::`, every call passing an argument whose view of C is the
  receiver's view of C: refined to `C<T>` (mechanically checked; Java's typing of the body holds for
  the subtype `C<T>`).
- `if (o instanceof C) { C<?> that = (C<?>) o; ...` in C's `equals(Object)`: `that` is `C<T>`, the
  cast is to it and the test is `is_[*C[T]](o)`. For an object of another instantiation Go answers
  false where Java runs the block, which the translator cannot prove answers false: the method must
  be listed in `tools/j2go/ownwild-equals.txt` with that argument (FlagEntry is instantiated with
  the enums CheckLevel and JsSourceType, Enum.equals is identity, every flag is an enum constant, so
  the block is false too), and every type argument written for C must be an enum, a type variable or
  a wildcard (checked). A name list is the honest form here: the remaining argument (no null flag)
  is about the program's values, not its types.

```go
func (this *AbstractCommandLineRunner_FlagEntry[T]) equals(o any) bool {
	if is_[*AbstractCommandLineRunner_FlagEntry[T]](o) {
		var that *AbstractCommandLineRunner_FlagEntry[T] = cast_[*AbstractCommandLineRunner_FlagEntry[T]](o)
		return (equalsT_(that.flag_, this.flag_) && (that.value_ == this.value_))
```

Counts (`J2GO_LIBGEN=2` report of the default translation): typed 1 parameter
(`SourcePosition.isSamePositionAs(that)`), 1 equals cast (`AbstractCommandLineRunner.FlagEntry.equals`);
left 1: "a local outside equals(Object)" (`HamtPMap.checkInvariants(child)`). No other reason
occurs in the tree; the self-test has one class per reason (`Over` overridden, `Mixed` a call of
another instantiation, `Stat` static, `Asg` assigned, `Ref` `::`, `Unl` not listed, `Str` listed
but a String instantiation, `NoGuard` no instanceof guard) and checks that `Pos` and `Entry` are Go
generic types, the eight others erased, and one `is_[*OwnWild_Entry[T]](`. Both classes are now
TYPED in the generics report: `AbstractCommandLineRunner_FlagEntry[T comparable]` (57 mentions with
type arguments in `g_*.go`), `SourcePosition_rhino[T comparable]` (32).

Found on the way: `GenScan.scan` checked a method parameter's declared type, not its refined one
(`cx.vtype`); changed (identical when nothing is refined).

### 12.3 Piece `nbox` (R5)

java.util.Optional's Go type (`Optional[T]`, valueLibs) does not fit a nullable Boolean as it is:
its Object form is the Optional itself where a Boolean's is the bool, and it has three states
(null, empty, present). The piece uses Optional's mechanism (a value library class: zero value null,
`== null` is `null_()`, Object is `box_()`, Ctx.coerce converts) for a new runtime type
`Nbox[T]` (`rt_nbox.go`, 44 lines: the primitive and a flag; `nbox_`, `nboxOf_`, `get_` throwing
NullPointerException for null, `null_`, `box_`; test `rt_nbox_test.go`) and a stand-in class
`j2go.rt.Nbox<X>` that only the translator sees (`Main.withStubs`, only when the piece is on), so
that the emitter's types say which variables hold one. Null and the value are told apart by the flag
for every value (false and 0 are values), so no refusal is about telling them apart; the refusals are
uses the conversions do not serve as Java does.

Typed: a field or method parameter declared as a boxed primitive; a method parameter of a library
function type with a boxed type argument (`Consumer<Boolean> set`, `Supplier<@Nullable Integer>
get`) is the function type at `Nbox[T]`, and the lambdas given for it have `Nbox[T]` parameters.

```go
func (this *CmdLineParser_Option) bool(get Supplier[Nbox[bool]], set Consumer[Nbox[bool]]) {
	...	return CmdLineParser_Option_asList(get.get().box_())
	...	set.accept(nbox_[bool](true))
this.parser.option("--print_tree")...bool(Supplier[Nbox[bool]](func() Nbox[bool] {
		return nbox_[bool](this.printTree)
	}), Consumer[Nbox[bool]](func(v Nbox[bool]) {
		this.printTree = v.get_()
	}))
```

Counts (`J2GO_LIBGEN=2` report of the default translation):

| | count |
|---|---:|
| typed fields (`browserFeaturesetYear`, `parseInlineSourceMaps`, `polymerVersion`, `OptimizeCalls.Builder.considerExterns`, `JSType.templateCheckResult`, `SubtypeChecker.isUsingStructuralTyping`) | 6 |
| typed parameters (`setBrowserFeaturesetYear`, `setPolymerVersion`, `JSTypeReconserializer.addSupertypeEdges`, `QualifiedName.join(Character)`, `Node.newNumberNode(Double)`) | 5 |
| typed function parameters (`CmdLineParser.Option.flag/bool/integer`: get, set) | 6 |
| typed lambda parameters (38 option setters of `CommandLineRunner.Flags`, 2 others in CommandLineRunner) | 40 |
| left: a parameter of a method that overrides or is overridden (`appendTo(StringBuilder, Character)` x3, `apply(Integer exitCode)`) | 4 |
| left: a field of a function type (not in the piece) | 4 |
| left: a method called on it (`isReasonableDoubleValue`, `isSafeNumber`) | 2 |
| left: a constructor parameter (function types: `PassFactory`, `ColorSerializer`) | 2 |
| left: a function parameter passed on as a value (`enableTestMode`, `setExitCodeReceiver`) | 2 |
| left: a function parameter given something else than a lambda (`PassFactory.Builder.setCondition`) | 1 |
| left: a field of a record, an interface or a handle class (`Node.NUMBER_PAYLOAD_UNSET`) | 1 |
| left: a parameter of an abstract method | 1 |
| left: a string concatenation (`DotFormatter.formatNodeName`) | 1 |

Locals declared as boxes are not in the piece. `g_CommandLineRunner.go`: `unbox_[` 37 -> 0;
in all `g_*.go`: `Nbox[` 0 -> 96, `func(v Nbox[` 40. The self-test (`lang/TriBox.java`) checks 3 + 1
typed fields, 5 left (method call, `++`, string concatenation, `==` of two boxes), 4 typed
signatures, 5 lambdas, 3 function parameters left (`::`, passed on, not a lambda), and prints
null / NullPointerException / values as Java does.

### 12.4 Byte identity, census, binary (md5 of `cat go/closure/g_*.go`)

| translation | md5 |
|---|---|
| `J2GO_NOLIBGEN=ownwild` at 4a1df420 | fab9e90cd1e8e5cdb9032035e9e6b295 (= main's default) |
| `J2GO_NOLIBGEN=nbox` at f6fa4111 | 995f66540ef033f4ab7d56d2febd8914 (= 4a1df420's default) |
| `J2GO_NOLIBGEN=ownwild,nbox` at f6fa4111 | fab9e90cd1e8e5cdb9032035e9e6b295 (= main's default) |
| `J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all` (both commits) | 973b5b6d3e4f71ab1cc82872360f9759 |
| default at 4a1df420 | 995f66540ef033f4ab7d56d2febd8914 |
| default at f6fa4111 | 13f040cdd148ae568cc7faceb89d1484 |

Census (`census.py` of pf5; main's default -> 4a1df420 -> f6fa4111): dynamic dispatch through self_
523 -> 522 -> 522; class casts 1,114 -> 1,107 -> 1,107; instanceof 439 -> 440 -> 440 (the typed
`is_[`); boxing to any 1,496 -> 1,484 -> 1,487 (`nbox` adds `.box_()` where a value meets Object);
unboxing from any 142 -> 129 -> 129; equals/hashCode 443 -> 442 -> 442; fields/params declared any
1,293 -> 1,287 -> 1,236. Other rows unchanged. Diff lines: 303 in 6 files (ownwild), 438 in 12 files
(nbox).

Binary (`go build ./cmd/mech`, GOENV goenv-c1): main's translation and runtime 123,934,349 bytes;
4a1df420 123,933,082 (-1,267); f6fa4111 123,948,667 (+15,585; +14,318 against main). Runtime lines of
the default build 20,982 -> 21,062 (main) -> 21,106 (`rt_nbox.go`); test lines 4,845 -> 4,877.

### 12.5 Gate lines

`bash tools/j2go/run.sh` of the commit, then `$B/gate.sh libt6 <label>` (logs
`/mnt/data/ubuntu/jm/logs/gate-t6_ownwild.log`, `gate-t6_nbox.log`), both under `hv.sh`:

```
GATE OK t6_ownwild
tree: 4a1df4200 rule "typed generics": piece ownwild: ...  dirty: 1
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.995s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 21306  test lines: 4845

GATE OK t6_nbox
tree: f6fa4111f rule "typed library interfaces": piece nbox: ...  dirty: 1
selftest: PASS
removed 0 declarations in 0 rounds
ok  	jsgomech/closure	1.948s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 21350  test lines: 4877
```

(`dirty: 1` is the `.cache` link.) Switches off, each commit (`offall.sh`: translate with the switch,
`tools/prune/verify.sh` with it): `J2GO_NOLIBGEN=ownwild` / `=nbox` and `J2GO_NOFNTYPES=1
J2GO_NOLIBGEN=all`: selftest PASS, `ok`, the four MATCH lines as above in all four runs; the default
made again afterwards was identical (995f6654, 13f040cd).

### 12.6 Timing (CPU, googleapis 18% subset, `proto/gast.py gasub/flags3500.txt 3`)

`base` = main's translation (fab9e90c) with main's runtime (`git archive main go`), built as
`$B/bin/m_t6_base`; `new` = the gate binary of f6fa4111 (`$B/bin/m_t6_nbox`); both with GOENV
goenv-c1 (`-c=1`), under the 20G scope. The translator is one step: both pieces together. Output
sha 72b57cd286c9 in all six runs (log `tm-t6.log`).

| binary | runs (CPU s) | mean | min |
|---|---|---:|---:|
| base | 161.50, 163.14, 162.37 | 162.34 | 161.50 |
| new | 161.16, 163.29, 161.00 | 161.82 | 161.00 |

-0.3% of the means and of the minima: neutral, within one binary's spread (1.6 s). Neither piece
touches a hot path (command-line parsing, JSDoc markers, JSType's template check flag).

### 12.7 Not done, unverified

- No flip: the census of a flipped tree was not made; post-flip tools meeting `Nbox[` or the generic
  `FlagEntry[T]` / `SourcePosition_rhino[T]` were not run.
- The nbox self-test does not cover `Integer` compared with a primitive of another width, or boxes
  other than Boolean / Integer (the tree's Character and Double parameters are covered only by the
  gate).
- Locals declared as boxes, fields of function types and constructor parameters are not typed.
- `QualifiedName.join(Character)` and `Node.newNumberNode(Double)` are typed although not R5 sites;
  their callers convert at the call (`nbox_`), not measured beyond 12.6.
- No allocation profile.

## 13. Record of rule T5 of JAVAISMS6.md: typed protobuf list accessors (branch `main-t5-resid`, worktree `/mnt/data/ubuntu/jm/wt/t5r`, 2026-11-24)

On top of main 73e902a8. T5 (26 sites on the eighth flip's tree: for-each via iterator objects 11,
unboxing 9, class casts 4, type assertions 2): the runtime's protobuf API (`tools/protogen`,
`go/closure/rt_proto_gen.go`) gave `getFooList() List` and `toBuilder() MessageLite_Builder`, so every
read of a repeated field cast its elements and every `toBuilder()` was asserted to the concrete builder.
jsgo's generated messages have the typed forms: `GetValueList() []string`
(`internal/jscomp/proto_conformance_requirement.go:328`), `GetRequirementList() []*Requirement`
(`internal/jscomp/proto_conformance.go:80`), and a builder that is the message (`Clone() *Requirement`,
`proto_conformance_requirement.go:269`). Scratch: `/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/t5r/`
(`hv.sh` one heavy job: the lock, a 20G scope, GOENV goenv-c1; `runsh.sh`, `t5cycle.sh`, `gatet5.sh`,
`offt5.sh`, `basebin.sh`, `tm.sh`; `out-*` translations).

### 13.1 The form (commit b3c6c76f)

- **protogen, rule T5** (`PROTOGEN_NOT5=1` writes the file without it, byte-identical to main's): next to
  each erased accessor it writes the typed one, so a translation with the translator's piece off still
  compiles against the same file: `getFooList_t() ListT[*Foo]` for a repeated message field and
  `getBarList_t() ListT[string]` for a repeated string field (on the message and on its builder, and in
  the `FooOrBuilder` interface where protogen declares it), and `toBuilder_t() *Foo_Builder` on each
  message. The list is a new typed list each call (`pbListT_` in `rt_proto.go`, 12 lines: a `TList[T]` of
  the elements, as `list_` copies for a builder). Names are fixed (Java name + `_t`), matched against
  `externals.tsv` (`markTyped`, which also checks the result the translator expects); the erased
  overload naming skips them (`typedName`). 47 repeated message/string fields: 94 list accessors and 47
  `toBuilder_t`.
- **translator, piece `proto`** (`J2GO_NOLIBGEN=proto`; `LibProto.java`; needs kinds `list` and `iter`):
  a call of a protoc-generated type's `get*List()` (no parameter, owner a subtype of
  `MessageLiteOrBuilder` outside `com.google.protobuf`) whose result is `List<M>` (M a generated message)
  or `ProtocolStringList` is the call of `getFooList_t()` at `List<M>` / `List<String>`; `Msg.toBuilder()`
  is `toBuilder_t()` at javac's `Msg.Builder`. Everything after is the existing machinery: for-each runs
  `eachOf_`, a typed place takes the list as it is, erased places get `List_un_`. A local of type
  `ProtocolStringList` holding such a list, never assigned again, is refined to `List<String>`
  (`GenScan.refine`; TypedAstDeserializer's `var runtimeLibraryList`). Left, in the `J2GO_LIBGEN`
  report: 11 calls of lists of enums, numbers or bytes; OrBuilder / Builder lists are not touched.
  Report of the default translation: 77 typed list accessor calls, 3 typed `toBuilder` calls.

### 13.2 md5 (`cat go/closure/g_*.go`; `rt_proto_gen.go`)

| translation | g_*.go | rt_proto_gen.go |
|---|---|---|
| main 73e902a8, default (measured first) | 13f040cdd148ae568cc7faceb89d1484 | ced5c8dab95c0fffc6a23dc75b74fdf2 (committed; protogen on main's translation reproduces it) |
| b3c6c76f, `J2GO_NOLIBGEN=proto` / `PROTOGEN_NOT5=1` | 13f040cdd148ae568cc7faceb89d1484 (= main) | ced5c8dab95c0fffc6a23dc75b74fdf2 (= main), 0 warnings |
| b3c6c76f, `J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all` / `PROTOGEN_NOT5=1` | 973b5b6d3e4f71ab1cc82872360f9759 (= main's all-off) | ced5c8dab95c0fffc6a23dc75b74fdf2 |
| b3c6c76f, default (translate, protogen, translate again: identical) | e82e1b4c663bbcf708b17be721263576 | 0cc968f16c52812eae0d51158aefff79 (committed) |

### 13.3 Gate, switches off

`$B/gate.sh t5r t5r_proto` on the committed b3c6c76f (log `$B/logs/gate-t5r_proto.log`):

```
GATE OK t5r_proto
tree: b3c6c76fa rule T5 "typed protobuf list accessors": ...  dirty: 1
selftest: PASS
ok  	jsgomech/closure	1.915s
checks: MATCH (952 inputs, exit 0, 0 output bytes, 4433 stderr bytes)
bundle: MATCH (3 inputs, exit 0, 3094347 output bytes, 9544 stderr bytes)
downlevel: MATCH (2094 inputs, exit 0, 2345059 output bytes, 4847 stderr bytes)
advanced: MATCH (848 inputs, exit 0, 6809562 output bytes, 11082 stderr bytes)
runtime lines: 21362  test lines: 4877
```

(`dirty: 1` at the start is the `.cache` link; the status listing at the end of the log also shows three
docs files of this branch created while the gate ran, not part of the build.) Off-switch runs
(`offt5.sh`: translation with the switch, `tools/prune/verify.sh` with it, the committed typed
`rt_proto_gen.go`): `J2GO_NOLIBGEN=proto` and `J2GO_NOFNTYPES=1 J2GO_NOLIBGEN=all` each selftest PASS,
`ok`, four MATCH; the default made again afterwards was e82e1b4c.

### 13.4 Census, binary, timing

Census (`census.py` of pf5 on the translation, main -> b3c6c76f): for-each via iterator objects 763 ->
733 (-30), interface casts 290 -> 289, class casts 1,107 -> 1,089 (-18), unboxing from any 129 -> 113
(-16); no row higher. All 26 T5 sites of the accounting are gone from the translation (the four
`toBuilder` casts / assertions, 11 for-each, 9 `str_` / casts of elements, the `ProtocolStringList`
read).

Binary (`go build ./cmd/mech`, GOENV goenv-c1): main's translation and runtime (`m_t5r_base`)
124,009,755 bytes; b3c6c76f (`m_t5r_proto`) 124,524,435 (+514,680). Not broken down: the likely cause is that the typed accessors are written for
every repeated message / string field, used or not (94 + 47 methods, each `pbListT_` instantiation its
own code, and the OrBuilder interfaces keep them reachable); writing only the ones `externals.tsv`
names would be the next step (not done, not measured).

Timing (`proto/gast.py gasub/flags3500.txt 3 base=$B/bin/m_t5r_base new=$B/bin/m_t5r_proto`, both
built with goenv-c1, under the lock and the 20G scope; output sha 72b57cd286c9 in all six runs):

| binary | runs (CPU s) | mean | min |
|---|---|---:|---:|
| base | 163.69, 163.76, 165.52 | 164.32 | 163.69 |
| new | 161.99, 164.06, 167.93 | 164.66 | 161.99 |

+0.2% of the means, -1.0% of the minima: neutral (within one binary's spread, up to 6 s here). The sites
are cold (conformance configuration, the typed AST's runtime libraries, compiler state).

### 13.5 Not done

- No self-test program uses protobuf; the piece is covered by the gate only (the e2e inputs exercise
  conformance and the typed-AST paths only as far as they do).
- No flip: the census above is of the translation, not of a flipped tree.
- The binary growth is not broken down (13.4).
