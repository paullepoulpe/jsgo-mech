# Java-isms left in the in-bounds tree (agQr, b59f6ea1), re-derived

Tree: `/mnt/data/ubuntu/jm/wt/agQr` (branch inb-toolgaps-tree, b59f6ea1), `go/closure/g_*.go`:
827 files, 262,733 lines. Reference: jsgo at 71ff08f4 (`/home/ubuntu/work/jsgo`, 710 non-test
files, 402,963 lines under `internal/`). Nothing in the tree was modified, built or gated; tools
that can write ran with `-n` on a copy (`javaisms/tree/closure`), and the copy was compared with
the tree after each run (52 runs, 0 changed it).

All paths below are under
`/mnt/data/tmp/claude-1000/-home-ubuntu-work/9db246f3-01a3-48c8-9bf5-1c94d1ed78d6/scratchpad/javaisms/`
unless absolute.

## 0. What the answer is

1. **The census understates what is left.** It counts spellings. Several rows are near zero
   because the construct was renamed, not removed: StringBuilder is 5, but `strBuilder_` (the
   runtime's Java StringBuilder) is named 252 times and `strings.Builder` 0 times; Preconditions
   is 6 because the checks are inline (681 `panic(new_NullPointerException...`); "dispatch through
   self_" is 0, but 1,618 of the 2,117 type assertions are the dispatchers in another form;
   "native Go maps made" is 0 against jsgo's 557; explicit boxing is 1,233, and another 3,056
   typed values are converted to `any` implicitly (section 3).
2. **One root cause covers most of the countable rest: Java's library generics are still erased
   in Go.** `ListOf[T]`, `IteratorOf[T]`, `MapOf[K,V]`, `SupplierOf[T]`, `ComparatorOf[T]` and 47
   more are aliases of untyped interfaces (`g_0aliases.go`, 52 aliases). The translator knows the
   element type and writes it in the alias, and the value still travels as `any`. About 2,020
   census sites across eight rows are this one thing (rule R1 below). jsgo has none of it.
3. **About 6,100 sites are what jsgo also writes** (panics for unchecked exceptions, recover at
   boundaries, defer, comma-ok assertions for instanceof, `Equals(other any)`, UTF-16 string
   helpers, `any`-valued Node and JSDocInfo property bags, a `self_` field for virtual calls).
4. **Where jsgo is different and no rule is offered**, it got there by rewriting a data structure
   by hand (flow states in a side slice instead of graph annotations, the node's type in a typed
   table, index loops instead of iterators, plain Go maps audited one by one for order
   independence). Those are counted as REASON and as a difference from jsgo, not as equivalence.

### Summary table

Command: `cd /mnt/data/ubuntu/jm/wt/agQr/go/closure && python3 /mnt/data/ubuntu/jm/wt/agQ/tools/postflip/census.py`
(output in `census.out`; `census.py` is identical in agQ and agQr). Columns: DONE = the remaining
form is what jsgo has too; RULE = a general rule or an existing tool would remove it; REASON =
stays, with a reason; "not a site" = a helper declaration, a function used as a value, a comment,
a string literal or a name match; "not split" = I could not assign the sites to a verdict by
script. Every row adds up to its count.

| row | count | DONE | RULE | REASON | not a site | not split |
|---|---:|---:|---:|---:|---:|---:|
| erased Java collection constructors | 120 | 22 | 83 | 15 | | |
| typed TList/TMap/TSet constructors | 448 | | | | | 448 |
| Guava immutables / builders | 155 | | 155 | | | |
| native Go slices made | 100 | 100 | | | | |
| native Go maps made | 0 | | | | | |
| for-each via iterator objects | 105 | | 96 | | | 9 |
| explicit Iterator use | 125 | | | 125 | | |
| anonymous interface assertions | 16 | | 16 | | | |
| interface casts `icast_X(` | 207 | 51 | 82 | 8 | 66 | |
| class casts `to_X(` / `cast_[` | 769 | 74 | 361 | 39 | 295 | |
| instanceof `is_X(` | 71 | 54 | | | 17 | |
| type assertions `x.(T)` | 2,117 | 1,933 | 48 | | 136 | |
| type switches | 27 | 3 | | | 24 | |
| boxing to any (explicit) | 1,233 | 323 | 626 | 123 | 9 | 152 |
| unboxing from any | 256 | 6 | 246 | 3 | 1 | |
| lambda objects `&X_fn{func` (subset of next row) | 45 | 19 | 7 | | | 19 |
| lambda adapter literals | 587 | | 496 | | | 91 |
| Optional | 73 | | 63 | | 10 | |
| try/catch emulation (recover, catch_) | 54 | 54 | | | | |
| panic( | 3,528 | 3,528 | | | | |
| defer | 84 | 84 | | | | |
| equals/hashCode protocol | 143 | 53 | 90 | | | |
| String_* UTF-16 helpers | 122 | 122 | | | | |
| StringBuilder | 5 | | 2 | | 3 | |
| enum objects (struct, call) | 1 + 1 | | 2 | | | |
| Preconditions_ | 6 | 6 | | | | |
| threads/executors | 56 | 28 | | 15 | 13 | |
| reflect | 5 | | | | 5 | |
| unsafe | 1 | | | | 1 | |
| fields/params declared `any` (Object slots) | 542 | 97 | 238 | 111 | 6 | 90 |

The row "fields/params declared any (approx)" is 696 = 542 + 140 cast-helper operands + 9
`return any(` + 5 type parameters (script: the heredoc in section 14, output `matches_any.tsv`).

### RULE candidates, ranked by sites

Sites are census matches; a rule's count is the sum of the sub-causes assigned to it in the
sections below. "Hot" means sites in the files of the list given for this task (TypeInference,
TypeCheck, TypedScopeCreator, NodeTraversal, ControlFlowAnalysis, DataFlowAnalysis, Node, and the
files declaring a struct in the JSType hierarchy: 35 files with sites in all; flag `H` in `sites.tsv`).

| # | rule | sites | hot-path relevance |
|---|---|---:|---|
| R1 | **Generic library interfaces.** Emit Java's generic library interfaces and functional interfaces as Go generic types and typed func types with the type arguments javac assigned (`Iterator[T]`, `List[T]`, `Map[K,V]`, `Map_Entry[K,V]`, `func() T`, `func(T) R`, `func(a, b T) int32`, `Optional[T]`), instead of the 52 `XOf[...] = X` aliases. | about 2,020: casts 361 + 82 + 48, unboxing 228, explicit boxing 524, adapters 496, `any` parameters 215, Optional 63; the collection rows are under R2 | **Yes.** `Iterator.next()` casts of `*FunctionType_Parameter` in TypeInference (3), TypeCheck (2+1), TypedScopeCreator (2), FunctionType (2), SubtypeChecker (4); 19 adapters passed to runtime functions in hot files; every bridge is an extra call that blocks inlining. The largest effect is indirect: typed collections stop needing `tconv_` box/unbox descriptors (282 declared, 2,234 uses) |
| R2 | **Collections typed at the constructor**, following R1: `new_ArrayList_0()`, `new_LinkedHashMap()`, `ImmutableList_of(...)` take the element types of their Java declaration (the `colls` tool does this; it stops today at flows into erased generic methods, which R1 removes). | 83 + 155 + 96 for-each cursors = 334 | Low directly (3 + 6 + 2 sites in hot files) |
| R3 | **Run `eqhash` again after step 402.** The tool, unchanged, reports `rewritten: 66` on this tree (`dry/eqhash.out`): the `objequals_(obj_(a), obj_(b))` / `objhash_(obj_(a))` pairs in the key descriptors of `g_0casts.go` that `unify` wrote after eqhash last ran. | 66 calls + 72 boxes | **Yes, indirectly**: these are the hash and equality functions of typed maps keyed by classes with equals/hashCode |
| R4 | **Typed generics for program classes whose type parameter is still erased**: `SimpleSetMultimap`, `SimpleTable`, `SourcePosition<T>.getItem`, `JSDocInfo.Property<T>` readers. | 30 boxes + 18 unboxings = 48 | No |
| R5 | **Object fields that hold one boxed primitive** (`Boolean`/`Integer` used as tri-state): the flow model sees one type in 7 fields; type them as an optional of that type. | 7 fields + 7 boxes | 2 fields are in hot files: `JSType.templateCheckResult`, `SubtypeChecker.isUsingStructuralTyping` |
| R6 | **`self_` typed with the hierarchy's interface for the 16 classes where it is still `any`** (generic classes and small hierarchies: Graph, GraphColoring, DataFlowAnalysis, LinkedDirectedGraph nodes and edges, ParseTree, ...) and named `X_as` interfaces for the 15 generic classes whose `to_X` asserts to an anonymous interface. | 16 fields + 16 assertions | **Yes**: `DataFlowAnalysis.self_`, the graph node and edge classes |
| R7 | Objects.equals of two erased views of typed collections (18), `Objects_requireNonNull` as an inline nil check (6), two `new_StringBuilder` into `Appendable` slots (2), `Node_Prop` as an integer enum (2). | 28 | `Node_Prop` is in Node |

Not a census row but larger than any of them: section 3 lists what the census does not see.

### Things that could not be verified

- No benchmark or profile was run. Every "hot" flag is a statement about which file a site is in,
  not a measurement.
- Typed collection constructors (448) and the declarations behind them (955 list, 659 map, 839
  set declarations per `native -n`) are not split into "could be a slice or Go map" and "must stay
  ordered". The `native` tool reports its first obstacle per declaration, not per constructor, and
  its commonest map reason is a bare method name ("method Put", 103 declarations), which I could
  not interpret as a semantic reason.
- 152 explicit boxes passed to parameters of generated functions and 90 `any` parameters are not
  assigned to a verdict (listed in sections 8 and 14).
- "carrier raw" casts (the receiver's static type names no element type): I did not check in the
  Java source whether the Java declaration was raw or the alias was lost on the way. They are
  counted under R1 on the assumption that it was lost; that is an assumption.
- R1's proof obligation involves Java wildcards; I did not count how many sites pass a
  `List<Sub>` where `Iterable<? extends Super>` is wanted. That number decides how much of R1 is
  mechanical.
- The flow model (`anyflow`) merges every implementation of an interface method and every erased
  collection into one slot, so "holds one dynamic type" could be decided only for fields and for
  parameters by their own use, not for Object results.
- jsgo evidence was gathered by a search agent and I re-read 21 of its citations; the rest is as
  reported. jsgo's `PORTING.md` is out of date on collections and was not used.
- jsgo forms not looked up: `LogFile.log`, `MoreObjects.toStringHelper`, how jsgo walks
  `FunctionType` parameter lists (only that it has 16 `.HasNext()` sites in total).

## 1. Method

| step | script or tool | output |
|---|---|---|
| census | `census.py` (above) | `census.out` |
| every match with file, byte offset, line | `matches.py` (same regexes, read from `census.py`) | `matches.tsv` (11,156 lines with the `any` matches in `matches_all.tsv`) |
| sub-cause of every match | `tools/postflip/jclass/main.go` (new; uses the project's `rw` and `anyflow` packages for types and whole-package flow): finds the syntax node at each offset | `sites.tsv`, one line per match, so every breakdown sums to its row by construction |
| grouped tables used below | `tables.py`, `summ.py <row> <columns...>` | `tables.out` |
| boxing sinks | `tools/postflip/boxcensus` with a dump of each site added (my copy only), joined by offset: `boxjoin.py` | `boxsites.tsv`, `box_joined.tsv` |
| how each `any` parameter is used in its function | `anyuse.py` | printed |
| the project's own census tools | `castcensus -v`, `boxcensus -static`, `excensus` | `castcensus.out`, `boxcensus.out`, `excensus.out` |
| why each tool left what it left | `dryruns.sh`: 52 `-n` runs of the tree's own tools on the copy | `dry/*.out` |
| what no row counts | `outside.sh` | `outside.out` |
| jsgo | `/mnt/data/ubuntu/jm/wt/agQ/tools/postflip/reflip/jsgocensus.py /home/ubuntu/work/jsgo` | `jsgocensus.out` |

`jclass` left 0 matches unclassified in the cast, assertion, boxing and unboxing rows. Its "not
code" class (30 matches: comments in `g_0casts.go` and `g_0enumin.go`, the Java-name table
`g_0names.go`) is reported per row.

jsgo's census for scale: 570 type assertions, 78 type switches, 1,724 `panic(`, 58 `recover()`,
304 `defer`, 732 `make([]`, 557 `make(map[`, 22 Java-collection helper constructors, 16 explicit
iterator uses, 0 adapter literals, 281 `any` declarations, 279 `strings.Builder`.

## 2. R1 in detail, because eight rows depend on it

What the tree has: `type IteratorOf[T0 any] = Iterator`, `type ListOf[T0 any] = List`,
`type SupplierOf[T0 any] = Supplier`, and 49 more (`grep -c '^type \w*Of\[.*= ' g_0aliases.go`
gives 52). `var parameters IteratorOf[*FunctionType_Parameter] = ...` documents the element type;
`parameters.next()` returns `any`, and the caller writes
`cast_[*FunctionType_Parameter](parameters.next())` (g_TypeCheck.go:1496).

`jclass` checks, for every cast and unboxing whose operand comes from a call, a for-each or a
func value, whether the static type of the receiver names the cast's target as a type argument.
Result over class casts, interface casts and unboxings (script at the end of the `jclass` runs,
and `tables.out`):

| operand | target named by the alias | receiver raw or names another type |
|---|---:|---:|
| result of a Java-interface method (`Iterator.next`, `Map.get`, `List.get`, `Entry.getKey/getValue`, `Function.apply`, `Supplier.get`) | 63 + 5 + 48 = 116 | 24 + 29 + 29 = 82 |
| for-each element (`foreach_` cursor, `immElems_` range) | 45 + 1 + 27 = 73 | 21 + 13 + 38 = 72 |
| result of a runtime function or method (`Map_get_s`, `Optional.get`, `Iterables_getFirst`, ...) | 25 + 1 + 17 = 43 | 44 + 15 + 27 = 86 |

So for 232 casts the element type is in the declaration and only the Go type is erased. The 240
others are second-order: the receiver is itself the result of an erased call
(`icast_Map_Entry(it_1.v)` then `entry.getValue()`).

The other faces of the same erasure:

- **Bridges.** A class implementing `Comparator<T>` gets
  `compare(a_ any, b_ any) int32 { return this.compareT_(cast_[*X](a_), cast_[*X](b_)) }`
  (g_SymbolTable.go:1864). `anyuse.py`: of 352 `any` parameters of methods that an interface lists,
  178 are used only under one unboxing to one type (apply 55, accept 37, compare 24, test 12,
  compareTo 11, equalValues_impl 10, applyAsInt 8, ...). They are single-typed by construction.
- **Adapters.** `(&Supplier_tfn_string{func() string {...}}).get` builds an object only to take
  its erased method value `func() any`: 176 of the 587 adapter literals.
- **Boxing at the call.** `this.inputsById.get(obj_(id))`, `Collection.add(obj_(x))`: 98 explicit
  boxes into parameters of erased interface methods, 134 into erased results of bridges.

jsgo: no aliases, no bridges, no adapters. Lists are `[]T`; ordered maps are
`jutil.OrderedMap[K, V]` (`internal/jutil/ordered_map.go:19`); functional interfaces are func
types (`internal/jscomp/pass_factory.go:63-69`,
`internal/jscomp/node_util_traverse.go:317 func NodeUtil_VisitPreOrder(node rhino.Node, visitor func(rhino.Node))`);
comparators are `func(a, b T) int` given to `slices.SortFunc` (35 uses); `Comparable` is a typed
`CompareTo(that *T) int` (`internal/rhino/js_doc_info.go:392`). jsgocensus: 0 adapter literals.

Proof obligation for R1: Java's generics are checked by javac, so a site converts when no raw
type, unchecked cast or `@SuppressWarnings("unchecked")` reaches it. The part that is not free is
variance: Java passes `List<Sub>` to `Iterable<? extends Super>`, and Go's generics have no
variance, so each such flow needs a conversion (a view or a copy) or the site stays erased. Go
also has no generic methods, so `<T> T foo(Class<T>)` on an interface stays erased. The project's
own note says the translator on branch main-r8-typed already emits generics for generic classes,
hierarchies, typed lambdas and `Optional[T]`, "collections next"; this tree is that state: the
program's classes are generic (`TMap[K, V]`, `DataFlowAnalysis_LinearFlowState[L]`), the Java
library's interfaces are not.

## 3. What the census does not count

Command: `cd /mnt/data/ubuntu/jm/wt/agQr/go/closure && bash outside.sh` (regexes are printed
with each count in `outside.out`).

| Java-ism | count | jsgo |
|---|---:|---|
| virtual dispatch: dispatcher calls `this.self_.m_impl(...)` | 1,637 | `t.self().M()`: 103 (`internal/rhino`), a `self_` field of interface type (`jstype_js_type.go:238-241`) |
| methods named `*_impl` | 1,785 | none; methods are interface methods |
| `.self()` calls | 644 | 103 |
| explicit upcasts `.up_X()` | 1,656 | none: embedding and interfaces convert implicitly |
| nil-keeping conversions to a Java interface `X_as_Y(` | 714 | 2 helpers of that kind (`jsTypeOrNil`, `jsTypeStaticScopeOrNil`) |
| implicit conversions of a typed value to `any`, without `panic(x)` and `self_:` initialisers | 3,056 (301 allocate) | not counted in jsgo |
| typed collections in declarations `*TList[` / `*TMap[` / `*TSet[` | 2,545 | `[]T`, `map[K]V`; `OrderedMap[`/`OrderedSet[`/`OrderedStringMap[` 537 type mentions |
| erased collection interfaces in declarations (`ListOf[`, `MapOf[`, `SetOf[`, `IterableOf[`, `IteratorOf[`, ...) | 726 | none |
| erased views of typed collections `.asList()` / `.asMap()` / `.asSet()` / `.asImm()` | 834 | none |
| element-conversion descriptors `tc_X` (box, unbox, hash, eq per element type) | 282 declared, 2,234 uses | none |
| Java collection method calls on erased interfaces (`.get(`, `.put(`, `.add(`, `.contains(`, `.size(`, ...) | 3,364 | not comparable by regex |
| null-String sentinel `nullstr_` | 1,364 | no single convention: `(string, bool)` 243 functions, `base.OptString` 132, `*string` 52 |
| `Optional<String>` helpers `optstr*_(` | 1,871 | `OptString` |
| `sstr_(` (a Java String read as a Go string) | 1,295 | none |
| toString protocol helpers (`tostr*_`, `objstr_`, `jtostr_`) | 1,320 without `sstr_` | `jutil.ToJavaString` for boxed values |
| `strBuilder_` (runtime StringBuilder) | 252 | `strings.Builder` 279, no wrapper type |
| equals methods declared / hashCode methods declared | 109 / 119 | 70 / 58 |
| typed and identity equals helpers (`eqR_`, `equalsR_`, `equalsBy_`, ...) | 205 | none |
| Java exception objects constructed `new_X(Exception|Error)(` | 3,551 | the same model: 60 exception struct types in `jutil/exceptions.go` |
| inline null checks `panic(new_NullPointerException` | 681 | `jutil.CheckNotNull` 209 |
| `X_valueOf(` | 234 | 7 `_ValueOf` functions, which panic the same way (`rhino/token.go:465-471`) |
| Go `error` results | 0 | some: `regex.ParseRegExp(...) (RegExpTree, error)` |

The 3,056 implicit conversions come from `boxsites.tsv` (9,577 implicit conversions to an
interface, 8,353 of them to the empty interface; 3,497 are the argument of `panic` and 1,800 are stores into `self_` fields). Largest sinks: `objstr_` 270,
`Strings_lenientFormat` 180, `SimpleListMultimap.put` 154, erased immutable-map builders 58.

## 4. Class casts `to_X(` / `cast_[T](` : 769

`python3 summ.py 11 sub`, grouped by `tables.py` ("row 11").

| sub-cause | sites | hot | verdict |
|---|---:|---:|---|
| `unbox: cast_[*X]` in a `tconv_` descriptor (the function as a value) | 243 | 0 | not a site; goes with the descriptors (section 3) |
| helper declarations `func to_X(v any)` | 38 | 5 | not a site |
| fallback `cast_` inside `gcast_` helpers | 6 | 0 | not a site |
| comments | 6 | 0 | not code |
| `arrmap_(..., cast_[*T])` | 2 | 0 | not a site |
| parameter of an erased-signature method (bridge) | 129 | 0 | RULE R1 |
| parameter of a func literal typed `any` | 5 | 0 | RULE R1 |
| result of a Java-interface method, type named by alias | 63 | 10 | RULE R1 |
| the same, receiver raw | 24 | 1 | RULE R1 (assumption, see "not verified") |
| for-each element, type named by alias | 45 | 1 | RULE R1 |
| for-each element, receiver raw | 21 | 1 | RULE R1 |
| result of a runtime function, type named | 25 | 1 | RULE R1 |
| result of a runtime function, raw (without Node.getProp) | 37 | 4 | RULE R1 |
| result of a func value `func() any` | 12 | 2 | RULE R1 |
| `Node.getProp(...)` | 7 | 7 | DONE: jsgo's `GetProp` returns `any` (`rhino/node_props.go:553`) and readers assert through `node_cast[T]` (`rhino/node.go:301`, 14 uses) |
| `self()` of a typed value: a downcast in a class hierarchy | 45 | 7 | DONE: jsgo writes `t.(rhino.FunctionType)`, `parent.(*TypedScope)` (`jscomp/typed_scope_creator.go:458-462`) |
| caught exception after `is_X(ex_)` | 12 | 4 | DONE: jsgo asserts the recovered value (`jscomp/compiler_pipeline.go:729-759`) |
| other casts after an instanceof test of the same expression | 4 | 2 | DONE: jsgo has the same test-then-assert pair at `typed_scope_creator.go:458-462` |
| `Node.getFeatureSet()` | 6 | 0 | DONE: `rhino/node_props_accessors.go:221 GetFeatureSet() any`, callers assert |
| `Annotatable_getAnnotation(...)` | 14 | 5 | REASON, a difference: Java casts here too; jsgo removed the annotation by hand (`jscomp/data_flow_analysis.go:403,422`: states in a slice indexed by node) |
| other Object results of generated code (scope creators, `jstypeOrColor_get_`, `payload_get_`, `getPropWithDefault`, `getStaticSourceFile`) | 8 | 2 | REASON: declared Object or an interface in Java; jsgo typed the node's type by hand (`rhino/node_flags.go:54-59`) |
| Object field read | 8 | 0 | REASON |
| `boxT_` of a type parameter, `[]any` element, three single cases | 9 | 1 | REASON |

Sum: 295 not sites + 361 RULE + 74 DONE + 39 REASON = 769.

Casts after an instanceof check on the same value: 16 (`grep GUARDED sites.tsv`), 12 of them on
caught exceptions. The steps `patinst`, `instof` and `castfold -instof` already turned the others
into comma-ok assertions (201 in section 6).

`castfold -assert -n` explains why the casts keep the `cast_` helper instead of `v.(T)`: 601
casts "operand may be null and the value is stored or passed on". That reason holds: Java's cast
passes null and Go's assertion does not.

Hot-path note: `Node.getJSType()` (g_Node.go:2106) reads an `any` from a handle table and calls
`to_JSType`, an assertion to the `JSType_as` interface followed by a method call. jsgo reads a
typed table. That is the REASON row above, and by position it is the hottest cast in the tree.

## 5. Interface casts `icast_X(` : 207

| sub-cause | sites | hot | verdict |
|---|---:|---:|---|
| helper declarations | 66 | 0 | not a site |
| `self()` of a typed value cast to an interface (cross-cast) | 46 | 3 | DONE: jsgo asserts the same way (`callback.(NodeTraversal_ScopedCallback)` style; `rhino/jstype_equality_checker.go:336`) |
| result of a Java-interface method | 34 | 0 | RULE R1 |
| result of a runtime function | 16 | 3 | RULE R1 |
| for-each element | 14 | 0 | RULE R1 |
| bridge parameter | 10 | 0 | RULE R1 |
| result of a func value | 8 | 0 | RULE R1 |
| `JSDocInfo.getProp(...)` | 5 | 0 | DONE: jsgo `jsdocGet[T]` (`rhino/js_doc_info.go:80`, 90 uses) |
| `boxT_` of a type parameter in generic graph code | 7 | 2 | REASON: Go cannot assert a type-parameter value without converting it to an interface; `gcasts -n` leaves 5 for "no class of the type parameters implements the interface". jsgo's graph has concrete node types (`graph/linked_directed_graph.go:47-51`), written by hand |
| Object field | 1 | 0 | REASON |

Sum: 66 + 51 + 82 + 8 = 207.

## 6. Type assertions `x.(T)` : 2,117, type switches: 27, anonymous interfaces: 16

| sub-cause | sites | hot | verdict |
|---|---:|---:|---|
| dispatch: `if p, _ := this.self_.(*C); p == this { direct call }`, C a leaf class | 1,204 | 270 | DONE in kind, see below |
| the same, C has subclasses | 158 | 127 | DONE in kind |
| dispatch: `this.self_.(*C).m_impl()` (one implementation) | 256 | 1 | DONE in kind |
| bodies of `icast_X`, `to_X`, `is_X`, `Node_of_` | 136 | 10 | not a site |
| instanceof as `p, ok := v.(T)` | 201 | 26 | DONE: jsgo has 284 comma-ok assertions (`jscomp/type_check_declarations.go:584`) |
| of which on an `equals(Object)` parameter | 74 | 3 | DONE: jsgo `Equals(other any)` then an assertion (`jscomp/var.go:568`) |
| Java cast folded to `v.(T)`: `self()` of a typed value | 114 | 3 | DONE |
| Java cast folded to `v.(T)`: an erased result or parameter | 48 | 4 | RULE R1 (12 have the type named by the alias) |

Sum: 1,618 + 136 + 201 + 114 + 48 = 2,117.

The dispatch rows are 76% of this row and the census row named after them shows 0. jsgo has the
same mechanism for the same reason (a base struct cannot call the overriding method):
`self_ JSType` in `JSTypeBase`, set by the constructor (`rhino/jstype_named_type.go:128`). So this
is not a Java-ism jsgo avoided. The differences are of degree: jsgo's `JSType` is an interface, so
callers dispatch directly and only base-class code goes through `self()` (103 sites); here every
virtual method of every class with subclasses has a dispatcher (1,637) and its `_impl` (1,785).
The exact-class test is this project's own devirtualisation, and it costs one type-word compare.
398 of these assertions are in hot files. I have no measurement saying it is slower than jsgo's
interface call, and none saying it is not.

Type switches: 18 in `to_X` helpers and 6 in `gcast_` helpers (not sites); 3 real ones
(`JSType_visit`, `JSType_visit_2`, `RegExpTree.simplify`): DONE, jsgo has 78, and writes its
visitors as type switches (`rhino/jstype_template_type_replacer.go:211`).

Anonymous interface assertions: 16, all bodies of `to_X` for generic classes
(`interface{ as_DiGraph_() any }`), RULE R6.

## 7. instanceof `is_X(` : 71

`excensus.out` (its kinds sum to 71): 17 helper declarations; 21 catch-clause tests
`is_X(ex_)`; 27 inside `&&` / `||`; 3 for-conditions; 1 switch case; 2 other expressions.

Verdict: 54 DONE. jsgo tests a recovered value by type the same way, and has the one-line helper
form too: `jscomp/type_check_visit.go:75-80`, `typeCheck_isFunctionType`, commented as Java's
`type instanceof FunctionType`. 19 of the sites are in hot files (`for is_ProxyObjectType(current.self())`
in g_ObjectType.go:493; `!is_[*Color](this.jstypeOrColor_get_())` in g_Node.go:2149).

## 8. Boxing to any (explicit) : 1,233

`python3 boxjoin.py -v`: 1,207 boxes known to `boxcensus` + 9 in comments + 17 `any(x).(T)`
where x already has an interface type (Go's spelling of instanceof on an interface value).
672 of all 10,784 conversions to an interface allocate (`boxcensus.out`); of the explicit ones,
by sink below.

| callee or sink that takes the box | sites | allocate | hot | verdict |
|---|---:|---:|---:|---|
| variadic runtime argument: `Strings_lenientFormat` 137, `String_format` 16, others | 164 | 25 | 29 | DONE: jsgo's messages are `...any` too (`jutil/preconditions.go:38,288`); they are in failing branches here |
| variadic generated argument: `LogFile.log`, `Msg.format`, `Parser.reportError` | 45 | 34 | 0 | DONE in kind (Java varargs of Object); jsgo's form of `LogFile.log` not looked up |
| comparison `obj_(a) == obj_(b)` | 43 | 0 | 17 | DONE: identity comparison of values of two static types needs an interface in Go |
| `JSDocInfo_Builder.populateProp` / `setProp` | 31 | | 0 | DONE: `rhino/js_doc_info_builder.go:731,802` take `value any` |
| `Node.putProp` | 10 | 0 | 10 | DONE: `rhino/node_props.go:677 PutProp(prop, value any)` |
| elements of `[]any` literals | 13 | 3 | 0 | DONE |
| `any(x).(T)` on an interface value | 17 | 0 | | DONE |
| runtime `objequals_` / `objhash_` (key descriptors in `g_0casts.go`) | 72 | | 0 | RULE R3 (existing tool) |
| other runtime function parameters: `splitToListT_` 19, `equalsBy_` 18, `objequalsBy_` 17, `Map_put_s` 16, `ImmutableList_of_Object` 14, ... | 183 | 42 of the 255 | 8 | RULE R1/R2 |
| parameters of erased interface methods: `Collection.add`, `Map.get`, `Map.containsKey`, `List.indexOf` | 98 | 9 | 4 | RULE R1 |
| erased results: 72 returns of generated functions (bridges `apply`, `get`, `call`, `load`), 62 returns of func literals | 134 | 50 | 8 | RULE R1 |
| arguments of func values and func-typed fields | 28 | 8 | 0 | RULE R1 |
| `ComparisonChain.compare` 12, `BigInteger.compareTo` 10 | 22 | 12 | 0 | RULE R1 (Comparable) |
| other runtime collection methods (TMap.Put, TreeMap, EnumSet, EnumMap, immutable builders, Ordering, ...) | 59 | | 3 | RULE R1/R2 |
| `SimpleSetMultimap.put_s` 18, `SimpleTable.get` 12 | 30 | | 0 | RULE R4 |
| `MoreObjects_ToStringHelper.add` 52, `strBuilder_.append_Object` 9, `Joiner.join` 8, `Logger.log` 1 | 70 | 22 | 1 | REASON: the Java API takes Object; jsgo's form not looked up |
| `new_AssertionError_Object` | 21 | | | REASON: `AssertionError(Object)` |
| `var x any = any(1.0)` locals (float64 20, int32 5) | 25 | 23 | 3 | REASON, a difference: Java `Double`/`Integer` locals used as nullable numbers; jsgo returns `(T, bool)` by hand (1,112 such functions) |
| Object fields assigned | 7 | 0 | 1 | RULE R5 (counted there) / REASON; counted here as REASON |
| other parameters of generated functions | 152 | | | not split |
| comments | 9 | | | not code |

Sum: DONE 323, RULE 626, REASON 123, not split 152, not code 9 = 1,233.

## 9. Unboxing : 256

`python3 summ.py 16 sub`. By matched text: `str_(` 154, `Node_of_(` 100 (one is the helper's
declaration), `.(bool)` 2.

| operand | sites | verdict |
|---|---:|---|
| result of a Java-interface method (`Supplier.get` 22, `Entry.getKey` 20, `Iterator.next` 15, `List.get` 11, ...) | 77 | RULE R1 (48 with the type named) |
| for-each element | 65 | RULE R1 (27 named) |
| bridge parameter (`apply` 23, ...) | 39 | RULE R1 |
| result of a runtime function, without `Node.getProp` | 40 | RULE R1 (17 named) |
| result of a func value | 7 | RULE R1 |
| `SourcePosition.getItem` 12 and other generic program classes 6 | 18 | RULE R4 |
| `Node.getProp` | 4 | DONE (as in section 4) |
| `.(bool)` comma-ok | 2 | DONE |
| Object field or parameter | 3 | REASON |
| declaration of `Node_of_` | 1 | not a site |

Sum: 246 RULE + 6 DONE + 3 REASON + 1 = 256.

## 10. Lambda objects (45) and adapter literals (587)

`tables.out`, "row 18". Adapter types: `Function` 239, `Supplier` 123, `Consumer` 84,
`Function_base` 20, `Predicate_base` 18, `Predicate` 13, `CompilerPass` 13, `ToIntFunction` 12,
`Supplier_base` 14, `NodeUtil_Visitor` 7, `BiConsumer` 6, `Comparator` 5, 18 others 33. 542 are
the typed `_tfn_` kind, 45 the object kind `_fn{func` (the row "lambda objects").
What they wrap: capturing func literal 327, non-capturing func literal 215, named function 27,
method expression 18.

| where the adapter goes | sites | hot | verdict |
|---|---:|---:|---|
| nowhere: made to take its erased method value, `(&X_tfn{f}).get` 83, `.accept` 47, `.apply` 36, `.test` 10 | 176 | 4 | RULE R1 |
| `PassFactory.Builder.setInternalFactory` 175, `setPreconditionCheck` 11, `setCondition` 2 | 188 | 0 | RULE R1; jsgo: `internalFactory func(compiler AbstractCompiler) CompilerPass` (`jscomp/pass_factory.go:63-69`). Run once per pass, so not a speed item |
| command-line parser options | 64 | 0 | RULE R1 |
| runtime functions (`ImmutableMap_Builder.put` 15, `Comparator_comparingInt` 7, `Map_computeIfAbsent` 6, `Suppliers_memoize` 5, `Iterables_transform` 5, ...) | 68 | 19 | RULE R1 |
| other generated code 40, returned 31, stored 20 | 91 | 6 | not split |

So 68 go to runtime functions and 292 to generated code as arguments; 176 are consumed on the
spot; 51 are returned or stored.

The 45 object lambdas (`excensus.out`): 13 capturing and passed to a function that stores them,
21 capturing and returned as an interface value, 2 stored, 9 non-capturing made once. `fnobj -n`
gives the reason they are objects: "a class implements it too". jsgo has the same form for the
same reason, 7 func types with methods: `CompilerPassFunc` (`jscomp/pass_factory.go:219`),
`Source_TransformerFunc` (`bundle/source.go:337`), `ModuleLoader_ModuleResolverFactoryFunc`,
`Matcher`. So `CompilerPass_fn` 13, `Source_Transformer_fn` 4, `ModuleResolverFactory_fn` 1,
`Matcher_fn` 1 are DONE (19). `NodeUtil_Visitor_fn` 7 is RULE: jsgo's visitor is a plain
`func(rhino.Node)`, and `funcparams -n` shows why it stays ("given another value",
g_CheckJSDoc.go:498). The other 19 are not split.

Tool state: `deadtfn -n` would delete 5 adapter types nothing names; `etaconv -n` leaves 232 func
literals whose "body is another call"; `tfnargs -n` leaves 67 because "an interface has a method
of the function's name" and 48 because "the adapter's method converts its arguments or result in
a way the rule does not know". Those two reasons are R1 seen from the tool's side.

## 11. Collections

**Erased constructors: 120.** By type (`tables.out`, "row 0"): ArrayList 45, LinkedHashSet 20,
LinkedHashMap 18, BitSet 22, TreeMap 4, EnumMap 3, TreeSet 2, ArrayDeque 2, PriorityQueue 2,
LinkedList 1, IdentityHashMap 1. By destination: a field 36, the result of a func literal 29
(`computeIfAbsent(k, k -> new LinkedHashSet<>())`), a declared local 21, a runtime call 18, other 16.

- BitSet 22: DONE. jsgo has `jutil.BitSet` (`jutil/bitset.go:13`), a word array like this one.
- ArrayList, LinkedHashMap, LinkedHashSet 83: RULE R2. `colls -n` says why its proof stops:
  "result of a method of a generic type" (183 + 63 + 69 declarations), "function literal used as
  a value" (69 + 8 + 16), "ComputeIfAbsent of the runtime takes a func(K) V, through which other
  values may enter". All are erased signatures. No reason here holds against jsgo.
- TreeMap, TreeSet, PriorityQueue, EnumMap, IdentityHashMap, ArrayDeque, LinkedList 15: REASON, a
  difference. jsgo has no such types; it replaced each by hand (a slice worklist in
  `jscomp/control_flow_analysis.go:79-84`, a bitmask for a TreeSet, struct-keyed Go maps).

**Typed constructors: 448** (`newTMap_` 174, `newTSet_` 135, `newTList_` 126, `newTListN_` 13),
22 in hot files. These are generic, insertion-ordered containers that carry a `tconv_`
descriptor so that erased code can see them. Against jsgo: a `TMap`/`TSet` is jsgo's
`OrderedMap`/`OrderedSet` in kind where order is observable, and a `TList` should be a `[]T`.
jsgo makes 557 plain Go maps; this tree makes 0. jsgo allowed a plain map only with a written
`// order-independent:` reason per range loop, checked by `tools/maprangecheck` (58 such
comments); that is a hand audit. `native -n` on this tree converts nothing and lists first
obstacles per declaration: lists "result of a generic build" 233, results of the runtime's
immutable-list functions 318, "a view stored" 46; maps "method Put" 103, `tmapEmptyImm_` 77,
"keys compared by equals" 76; sets "keys compared by equals" 202, "method Add" 102. Not split;
see "not verified". Honest status: still a Java collection library with Go type parameters.

**Guava immutables: 155.** 99 are built and at once seen through `ImmutableList_as_List` /
`_as_Iterable` / `_as_Collection` / `ImmutableSet_as_Set` / `ImmutableMap_as_Map`, an erased
immutable given to an erased interface. RULE R2 for all; jsgo uses `[]T`.

**Native slices made: 100.** DONE (jsgo 732).

**For-each via iterator objects: 105.** 96 `foreach_(x)` over erased collections: RULE R2 (the
loop follows the collection's type). 9 `.each()` cursors over typed lists and sets: not split;
the `native` tool keeps a cursor where the loop body can change the collection. jsgo ranges over
slices and over `All()` (`iter.Seq`, 135 loops), and has 2 `ForEach` callbacks.

**Explicit iterators: 125** (`.iterator()` 52, `.hasNext()` 73), 24 in hot files. 12 iterators
are `IteratorOf[*FunctionType_Parameter]`: the parameter walks of TypeCheck, TypeInference,
FunctionType, SubtypeChecker and TypedScopeCreator, where two lists are walked in step. REASON, a
difference: jsgo has 16 `.HasNext()` sites in all and three iterator types
(`jscomp/node_iterators.go:123,298`, a JSON scanner), so its parameter walks are not iterator
objects; rewriting a paired iterator walk into an index loop is an algorithmic edit. R1 would
type the iterators (and remove the 34 + 15 casts on `next()`), not remove them. `iters -n`: "47
an iterator that is not an immutable collection's enters it".

## 12. Optional : 73

10 in the name table. 29 uses of the type name. 34 calls of the erased Guava Optional:
`Optional_base_absent` 20, `Optional_base_of` 12, `Optional_base_fromNullable` 2, mostly
`CompilerOptions` fields declared `Optional_baseOf[BoxedOf[bool]]`.

Verdict: RULE R1 for the 63 code sites. The tree already has a generic `Optional[T]` (3 uses);
`Optional_baseOf[T]` is one of the 52 aliases. jsgo has no generic Optional: `base.OptString`
(`base/opt_string.go:5`), `(T, bool)` results, nilable pointers, and for these same
`CompilerOptions` fields a struct with `value any` (`jscomp/compiler_options_conformance.go:828`,
13 uses). So for the CompilerOptions fields jsgo is no better typed than this tree.

`Optional<String>` is outside the row: 1,871 `optstr*_` calls. `optstr -n`: 0 rewrites, 1 erased
map of Optionals kept.

## 13. Exceptions, equals, strings, the small rows

**try/catch: 54** (27 `recover()`, 27 `catch_(r_)`), **panic: 3,528**, **defer: 84**:
`excensus.out`, whose kinds sum to each row. DONE for all. jsgo: `panic(jutil.NewX(...))` 1,724
times with Java's exception classes as Go types (`jutil/exceptions.go:29-204`), 58 `recover()` in
inline deferred functions (`jscomp/node_traversal.go:325-329`), 304 defers. The 9 typed catches
that use an exception as control flow (number format, regexp syntax) are a small difference:
jsgo returns a Go `error` from its regexp parser (`regex/reg_exp_parser.go:116`); this tree has 0
`error` results. That is a hand change in jsgo.

**equals/hashCode: 143.** `objequals_` 37, `objhash_` 37, `Objects_equals` 28, `.hashCode()` 15,
`.equals(` 10, `Objects_requireNonNull` 6, `Objects_hash` 4, `Objects_hashCode` 3,
`Object_equals` 3.

- 66 of the `objequals_` / `objhash_` calls: RULE R3. `dry/eqhash.out` begins `rewritten: 66`.
- `Objects_equals` of two erased views of typed collections (Map 8, List 6, Set 4) 18, and
  `Objects_requireNonNull` 6: RULE R7 (REFLIP4.md records both as open against the lead line).
- The other 53: DONE. jsgo has `jutil.ObjectsEquals` (`jutil/objects.go:34`, 22 calls),
  `ObjectHashCode` (10), 70 `Equals` methods of which 67 take `any`, 58 `HashCode` methods.

Outside the row, the protocol itself: 109 equals and 119 hashCode methods declared, against 70 and
58. jsgo keys its maps by comparable value structs (`jscomp/typed_scope_creator.go:40`, 15 such
key types) where this tree hashes through `hashCode`; that is a hand change per map.

**String_*: 122.** `String_length` 48, `String_format` 25, `substring` 16, `charAt` 8, `join` 6,
others 19. `dry/strnative.out` gives a reason per call, summing to 122: "length (units) of a
string not known to be ASCII" 48, "format: arguments are not an array literal" 20, "substring
(unit indexes)" 16, and so on. DONE: jsgo keeps WTF-8 strings and the same helpers
(`jutil/jstring.go`: `StringLength` 87 calls, `StringCharAt` 22, `StringSubstring` 35,
`StringFormat` 20). 11 sites in hot files.

**StringBuilder: 5.** 2 in the name table, 1 type name in a descriptor, 2
`Appendable(new_StringBuilder())`: RULE R7 (a manual step, 303, on the lead line). The row hides
the fact in section 3: `strBuilder_` 252 mentions, `strings.Builder` 0. jsgo: `strings.Builder`
279, no wrapper, no `setLength`/`insert`/`deleteCharAt`. Not counted by the census; by the
project's goal it is a Java-ism handled by a runtime type, and whether that matches jsgo depends
on which builders use `setLength`, `insert` or `charAt`, which I did not count.

**Enum objects: 1 + 1.** `Node_Prop` embeds `Enum` (g_Node.go:3032) and is compared by
`.Enum.name()` once. RULE R7: jsgo `type Node_Prop int32` (`rhino/node_props.go:49`).

**Preconditions: 6.** All are `checkNotNull` used for its value inside an expression. DONE:
`jutil.CheckNotNull[T any](reference T) T` (`jutil/preconditions.go:236`).

**Threads/executors: 56.** 13 in the name table or a comment. Code: `CompilerExecutor` 14 and
`Thread` 14: DONE, jsgo has an inline `CompilerExecutor` (`jscomp/compiler_pipeline.go:690-701`)
and a one-thread `Thread` (`jutil/logger.go:370-378`). `ExecutorService` 5, `Future` 4,
`Callable_fn` 3, `.submit(` 3 (PrebuildAst, PrebuildDependencyInfo, CompilerExecutor): REASON, a
difference; jsgo has no `ExecutorService` and wrote goroutine pools by hand
(`jscomp/prebuild_dependency_info_parallel.go:58-63`).

**reflect: 5, unsafe: 1.** None is a use: `reflect.` appears inside string literals, and
`unsafe.Add(...)` at g_FunctionArgumentInjector.go:152 is a local variable named `unsafe`.

## 14. Fields and parameters declared `any` : 542 (Object slots)

Matches dumped by the heredoc that wrote `matches_any.tsv` (census.py's two regexes and its own
split): 30 fields, 511 parameters, 1 `if any(this) == obj`. The regex does not see results or
locals; `objslots -n` counts 1,496 `any` slots in all and converts 0 (`objslots.out`: 463 results
and 323 parameters "method listed by an interface", 97 range variables, 72 func literal
parameters, 129 variables "a value that is not a box").

Parameters, by how the function uses them (`anyuse.py`):

| declaration | sites | use | verdict |
|---|---:|---|---|
| method that an interface lists | 178 | only unboxed to one type: a type parameter erased by Java, single-typed by construction | RULE R1 |
| `equals` / `equals_impl` parameters | 95 | tested by instanceof or comma-ok, then unboxed: heterogeneous by contract | DONE: 67 jsgo `Equals(o any)` |
| other parameters of interface-listed methods | 79 | passed on as Object 65 (`test` 19, `clone` 5, `appendTo` 5, `get` 4, ...), unused 14 | not split |
| func literal: command-line `bool`/`integer` option setters | 37 | a boxed Boolean/Integer | RULE R1 |
| func literal: `tconv_` descriptor functions | 24 | the erased side of a typed collection | REASON while the descriptors exist |
| other func literals | 11 | | not split |
| parameters of generated methods 57 and functions 25 | 82 | 69 only passed on as Object (`JSDocInfo.equalValues` 12, `logObject`, `setItem`, ...) | REASON: Object in Java. jsgo has 56 `any` parameters besides `Equals`, 15 of them on the same JSDocInfo and Node property functions |
| cast helpers 3, a func type 1, name table 1, `if any` 1 | 6 | | not sites |

Fields (30), with what the flow model sees stored (`sites.tsv`, row 100, kind `field`):

| fields | n | holds | verdict |
|---|---:|---|---|
| `self_` typed `any` (Graph, GraphColoring, DataFlowAnalysis, the four graph node and edge classes, ParseTree, SimpleReference, ...) | 16 | 1 to 81 classes of one hierarchy | RULE R6 |
| `CommandLineRunner_Flags.browserFeaturesetYear`, `.polymerVersion` (int32), `.parseInlineSourceMaps_`, `JSType.templateCheckResult`, `OptimizeCalls_Builder.considerExterns`, `SubtypeChecker.isUsingStructuralTyping` (bool), `Source_Lazy_1.cap_instance` (string) | 7 | one type | RULE R5 |
| `JSDocInfo.propertyValues`, `Node_ObjectPropListItem.objectValue` | 2 | heterogeneous | DONE: `rhino/js_doc_info.go:75`, `rhino/node_props.go:382` are `any` |
| `SourcePosition_rhino.item` (Node or string), `Property.name_`, `GlobalNamespace_Name.refsForNode`, `DisambiguateProperties_TypeEdgeJson.value_`, `ClosureBundler.minifier` | 5 | two types, or merged with every erased slot | REASON: `SourcePosition<T>` and `Property` hold a type parameter or two types in Java |

Sum: DONE 97, RULE 238, REASON 111, not sites 6, not split 90 = 542.

Which slots are genuinely heterogeneous: the `equals` parameters, the two property bags, the
recover values, `SourcePosition.item`. Which are type parameters erased by Java: the 178 bridge
parameters and, by the same construction, most of the 463 `any` results that `objslots` leaves.
Which hold a single type though declared Object: the 7 fields above; for Object results the model
cannot say.
