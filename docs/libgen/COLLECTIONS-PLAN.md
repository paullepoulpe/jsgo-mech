# Typed collections, the other half: map of the work, plan, and living record

Branch `main-libgen-b3` (worktree `/mnt/data/ubuntu/jm/wt/agZ`), on top of `main-libgen-b2`
c8a3f9d5 (LIBGEN-STUDY.md sections 7 and 8: rules "function types" and "typed library
interfaces", kind `iter`). This file is the handover: a successor continues from it alone.
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

## 3. Record

(filled in as pieces land)
