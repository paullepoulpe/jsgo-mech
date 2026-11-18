# Java-isms left in the transpiled Go, and where each is fixed

Census of `go/closure/g_*.go` on `postflip4` (817 files, 257k lines), the cost of each pattern
on the main thread (flat CPU time on typed-full, 61.9s in all, background GC workers
excluded), and how jsgo, the hand port, writes the same thing. Flat time counts only the
pattern's own code: the allocations it causes are under malloc/GC, and code it keeps the Go
compiler from inlining is under its callers, so these numbers are lower bounds.

jsgo's own report (`post_flip_optimizations.md`, §6.2) gives the scale. Its post-flip
baseline, the hand port with a node store and none of its 23 Go-only optimizations, single
threaded, runs typed-full in 10.3s on its machine. That is roughly 5x the transpiled Go per
unit of hardware, so most of the distance to jsgo is in how the port writes Java constructs
(`PORTING.md` §4-6), not in jsgo's later optimizations. The same report shows a heap
`*Node` variant 1.7x slower than the node store.

| Java-ism | sites in g_*.go | main-thread CPU (flat) | jsgo | addressed today by | best layer |
|---|---:|---:|---|---|---|
| Collections, erased (`ArrayList`, `LinkedHashMap`, `HashSet`...): constructors | 456 | 4.6s (7.5%) runtime collections + 4.2s (6.8%) Go maps underneath | `[]T` for lists (624 `make([]`), Go maps where order cannot be observed (489 `make(map[`), an ordered map/set only where it can (~100) | steps 1-5: 62 of 571 lists to `[]T`, 114 of 702 maps/sets to Go maps, 661/371/259 declarations typed (`TList`/`TMap`/`TSet`) | post-flip tools: whether a list is aliased and whether iteration order is observable needs whole-package type information, which the post-flip tools have and j2go does not |
| Collections, typed runtime (`TList`/`TMap`/`TSet`) | 480 | 1.4s (2.2%) | as above | step 3-5 | post-flip: a typed list that is never shared or viewed becomes `[]T` (extend `slices`) |
| Guava immutables (`ImmutableList.of`, builders) | 876 | 0.9s (1.5%) | `[]T` | none | post-flip: an immutable list is a list never mutated, so `[]T` (extend `slices`) |
| for-each through iterator objects (`foreach_`, `.each()`), explicit iterators | 1,360 + 136 | inside the collections rows | `range` | step 93ecb59 (no iterator object for some types) | follows the collections: a `[]T` or Go map is ranged over |
| Boxing into `any` (`obj_`, `strobj_`, `.box_()`, `any(...)`) and unboxing (`str_`, assertions) | 14,342 + 2,235; ~2,400 fields/params of type `any` | 0.7s (1.1%) flat; boxing is the largest allocation site (`strobj_`, 14.8M objects on typed-full) | generics: `any` appears on ~1,400 lines of jscomp+rhino, mostly for Java `Object` | colls (collections only); prep step 10 (`Optional<String>`) | j2go: emit Go generics for Java generic classes and methods instead of erasing them with aliases (`ListOf[T] = List`); post-flip for what j2go cannot type |
| Class casts `to_X(v)` and instanceof `is_X(v)` | 3,417 + 408 | 2.3s (3.7%) | `x.(*X)` for a class without subclasses; an interface assertion only for hierarchies | none | j2go: `to_X` asserts to an anonymous interface `interface{ as_X() *X }`, an itab lookup per cast; for a class that nothing extends it can be `v.(*X)`, one comparison. j2go knows the hierarchy |
| Interface casts `icast_X(v)` | 2,572 | (in the row above) | rarely needed: values are typed | colls removes some | follows generics and typed collections |
| Virtual dispatch through `self_` (`this.self_.(interface{ m_impl() }).m_impl()`) | 1,806 dispatchers; 2,135 anonymous-interface assertions | 0.7s (1.1%) flat in itab lookups, plus no inlining through any dispatcher | a named interface per class hierarchy; base methods call `b.self.M()` on a typed `self` (65 sites) | step 6: single-implementation dispatchers (134 of 547) | j2go: give each hierarchy a named interface and type `self_` with it, so a virtual call is an itab call with no assertion; keep step 6 for the single-implementation case |
| Lambdas as objects (`&Function_fn{func...}`) | 1,110 | 2.6s (4.1%) in closures and their wrappers; the wrapper is always on the heap | Go closures and func types (`type FooFunc func(...)`), which stay on the stack when they do not escape | `postflip4-wip11` (tool step `lambdas`: the 516 that capture nothing are made once) | j2go: a functional interface (`Function`, `Predicate`, `Consumer`, `Supplier`...) as a Go func type, a lambda as a func literal, `f.apply(x)` as `f(x)`; then the compiler's escape analysis keeps the non-escaping ones off the heap. The non-capturing step stays a post-flip tool until then |
| Enums as objects (`*Node_Prop` embedding `Enum`, `.ordinal()`) | 1,226; 30 EnumSet/EnumMap | ~0 flat; each use loads through a pointer | `type Foo int32` constants; fields as tables indexed by ordinal; `EnumSet` as a bit set | tokens step (the token is a byte) | j2go: an enum without per-constant bodies as an integer type with tables; `EnumSet` of it as a bit set (runtime) |
| Node properties as a linked list (`lookupProperty`) and node fields in four columns | — | node record access 10.6s (17.2%), of which `lookupProperty` 2.2s | one 36-byte pointer-free record per node; boolean properties as bits in it (`propFlags`), the rest in side tables | translator layout rules (`handles.txt`: hot/cold columns, string slot) | pre-flip Java step: Node keeps its boolean properties in an int bit field (a change Java can take with no loss); then j2go puts that word in the hot record |
| equals/hashCode protocol | 127 + 130 methods; 716 `objequals_`/`objhash_` calls | 0.5s (0.9%) | Go maps for identity-hashed keys, Java-ordered hash maps only for value-hashed keys | maps step; runtime hash tables | post-flip: identity-hashed keys (Node, Var, any class without equals) go to Go maps (extend `maps`) |
| `Preconditions` with message arguments | 2,494 calls, 889 with arguments | 0.07s flat; their arguments are boxed on every call, even when the check passes | fixed-arity forms, no boxing | none | post-flip tool: typed fixed-arity overloads, so arguments are boxed only on failure (j2go could do it too; a tool is simpler to review) |
| `Optional` | 1,858 | 0.07s | `(T, bool)` results | prep step 10, step 7 (a value) | done; `Optional<String>` unboxed measured neutral (`postflip4-wip7`) |
| Exceptions (`panic`, `catch_`, `defer`) | 1,660 + 93 | ~0 | the same (panic, `Try` closures, defer) | — | nothing to do |
| Strings with UTF-16 semantics (`String_*`) | 2,329 | 0.8s (1.3%) | the same helpers over WTF-8 | runtime (charAt fast path) | runtime, as jsgo |
| `StringBuilder` | 426 | small | `strings.Builder` | — | runtime |
| Threads, executors (`Thread`, `CompilerExecutor`, `submit`) | 78 | ~0 (run inline) | sequential | runtime runs them inline | nothing to do |
| `synchronized`, `ThreadLocal`, atomics | 1, 4, 5 | 0 | a re-entrant monitor (jsgo is parallel) | translated away (one goroutine) | nothing to do |
| reflection, `unsafe` | 5 and 1 in g_*.go (18 and 22 in the runtime) | 0 | — | — | nothing to do |

## Order of work

By measured cost and number of sites:
1. Casts and dispatch (j2go): concrete assertions for classes without subclasses, named
   interfaces with a typed `self_`. About 3,800 cast sites and 1,800 dispatchers; 4.8% flat
   plus the inlining it enables. To measure it before changing j2go, a post-flip tool can do
   the same rewrite with type information, as step 6 did.
2. Lambdas as func types (j2go), and until then the non-capturing step.
3. Collections to native slices and maps (extend the `slices` and `maps` tools to typed
   lists, immutables and identity-keyed maps), with `range` loops.
4. Generics instead of erasure (j2go), which removes most of the boxing and casts that remain.
5. Node boolean properties as bits (pre-flip Java step, then a j2go layout rule).
6. Enums as integers (j2go); Preconditions without boxing (post-flip tool).

## Status on postflip5 (final)

Census of g_*.go at the postflip5 tip against postflip4, and the post-flip step that handles each
Java-ism (what is left is listed with its reason in the step's commit message):

| Java-ism | postflip4 | postflip5 | step(s) |
|---|---:|---:|---|
| dispatch through `self_` with an anonymous-interface assertion | 1,806 | 0 | `dispatch` (step 6), `vdispatch -retype/-sub/-enums`, `selfeq` |
| anonymous-interface assertions (all) | 2,135 | 302 | the above; `casts` (small hierarchies); the rest are casts over the JSType/ObjectType hierarchies, where a type switch measured no faster |
| erased Java collection constructors | 456 | 278 | `colls` (list/map/set/deque), `native`; left: values boxed into Object fields cast back elsewhere, raw declarations, TreeSet/TreeMap, BitSet (already a word array) |
| typed TList/TMap/TSet | 480 | 401 | `native` (to []T / Go maps where nothing shares a list while it changes and nothing observes a map's order) |
| Guava immutables | 876 | 748 | `native -kind imm`, `immbuild`; left: immutables used as collections or compared by value |
| iterator for-each | 1,360 | 1,137 | `native` (range), runtime foreach_ over slices without iterator objects |
| boxing into any | 14,342 | 10,150 | `generics` (Optional[T]), `arraycopy`, `precond`, `objslots`, `charseq`, `strkeys`, PMap[K,V], `enums` (an integer enum boxes without allocating) |
| class casts / instanceof | 3,417 / 408 | 3,163 / 398 | `casts`; most remaining casts are concrete assertions already (j2go's cast_[*X]) |
| lambda objects | 1,110 | 417 | `lambdas` (non-capturing made once), `funcparams` (Go func values) |
| Optional | 1,858 | 103 | prep step 10, step 7, `generics` |
| equals/hashCode protocol | 909 | 633 | `eqhash`; left: values of interface type (immutables, Optional), floats |
| enums as objects | 205 structs embed Enum | 13 | `enums` (uint8 types); left: enums whose constants have bodies, and runtime enums |
| StringBuilder | 426 | 283 | `sbuild`; left: builders passed on or using charAt/setLength/insert |
| thread/executor emulation | 78 | 47 | manual (runInCompilerThread runs its task directly); left: names in stubs and two same-thread checks |
| Preconditions with message arguments | 889 boxed per call | boxed only on failure | `precond` |
| exceptions, String_* UTF-16 helpers, reflect, unsafe | | | being converted by post-flip steps (round 8, see "After round 7" below); not left as "not translation artefacts" |


## After round 7 (merged tree 85416d7a)

Census (`tools/postflip/census.py`, g_*.go), cc8beb35 -> 85416d7a: erased collection constructors 278 -> 228;
typed TList/TMap/TSet constructors 401 -> 431 (erased ones that became typed); Guava immutables
748 -> 748; for-each cursors 198 -> 101; explicit iterators 123 -> 102; interface casts 969 -> 959;
class casts 2,192 -> 2,018; instanceof 79 -> 72; boxing 6,039 -> 5,175 before the tostr re-run
(-613 after it); unboxing 791 -> 704; lambda objects 396 -> 201; Optional 103 -> 85; try/catch
emulation 68 -> 62; panic 3,470 -> 3,517 (inline checks panic directly); defer 91 -> 88;
equals/hashCode 153 -> 48; String_* 1,184 -> 973; StringBuilder 24 -> 24; Preconditions 76 -> 3;
threads 19 -> 13; reflect 5; unsafe 1; any declarations 2,128 -> 1,852.

Round 8 (in progress), one branch each: Guava immutables (postflip5-r8-imm), erased slots, casts
and generic interfaces (postflip5-r8-casts), the exception model and the misc rows
(postflip5-r8-exc), String_* and StringBuilder (postflip5-r8-str), the remaining Java collections
and typed collections to Go slices and maps (postflip5-r8-colls).

## After round 8 (72b0f693)

Census (`tools/postflip/census.py`), round 7 (2d204c0f) -> round 8: String_* 973 -> 122;
StringBuilder 24 -> 3; Guava immutables 748 -> 218; boxing 5,175 -> 3,072; class casts 2,018 ->
1,512; interface casts 959 -> 879; unboxing 704 -> 607; erased collection constructors 228 ->
194; typed TList/TMap/TSet 431 -> 339; lambda objects 201 -> 140; try/catch emulation 62 -> 50;
Optional 85 -> 78; defer 88 -> 82; panic 3,517 -> 3,492; for-each cursors 101 -> 99; explicit
iterators 102 -> 97; equals/hashCode 48 -> 52 and any declarations 1,852 -> 1,877 (conversion
variables of the immutable steps). Every site left is counted with its reason in the step
commits (see POSTFLIP.md, round 8). panic with an error value, defer for finally, boundary
catch-alls and rethrows are the Go idiom; the threads, reflect and unsafe rows are name matches,
not uses. The translator now keeps Java's types as Go generics on branch main-r8-typed (generic
classes, interfaces, hierarchies, typed lambdas, Optional[T]; collections next) and Node is a
store the collector never scans on main-r8-nodestore; a re-flip with both will make several of
these steps no-ops.
