# Post-flip steps (branch `postflip4`)

This branch is the post-flip half of the migration plan. Each commit is one of:

- **translator**: the flip itself, i.e. the Go that `tools/j2go/run.sh` and
  `tools/prune/prune.sh` produce from `main`. The node layout comes from j2go rules
  (`tools/j2go/handles.txt`), not from edits of the generated Go.
- **step**: a tool in `tools/postflip` rewrites the generated Go. The tool is in the tree, and
  rerunning it on the same input gives the same commit (`replay.sh` replays steps 3-7).
- **runtime**: a change to the handwritten Go runtime (`go/closure/rt_*.go`).
- **manual**: a hand edit of generated files. It is one reviewable change per commit, and the
  commit message says what it changes, why it keeps Java's behaviour, and what it measured.
  Each one is kept only if it measured as a gain; neutral ones were reverted.

Every commit passes `tools/prune/verify.sh`: the runtime tests and four projects compiled
byte-identically with pre-flip Java. From step 7 on, the translator self-test no longer
applies. There is no PGO and no GOGC setting.

## Against pre-flip Java

Medians of interleaved runs on one machine, nothing else running. Host speed varies by a few
percent between runs, so only ratios within one run are comparable.

| state of the branch | bundle | typed-full |
|---|---:|---:|
| tool steps and runtime up to "getFirst" | 1.38x | 1.36x |
| plus the hash table runtime (table copy, open-addressed index, hash in slot) | 1.29x | 1.29x |
| plus the seven measured hand edits (d12f878..3d4d302) | 1.18x | 1.30x |
| plus the allocation and lookup steps through 20896bb (3 runs) | **1.045x** (30.1s / 28.8s) | **1.19x** (63.3s / 53.2s) |
| plus the two CFG steps (2 runs; Java not in that run) | ~1.03x (29.7s) | ~1.16x (61.7s) |
| plus the SubtypeChecker free list, e86bef7 (3 runs) | 1.025x (29.95s / 29.22s) | 1.14x (61.7s / 54.3s) |

After e86bef7 the commits were measured against the commit before them (below), not against
Java; together they took bundle -2.7% and typed-full -4.4% (medians, 5 runs).

jsgo, the hand-written port, takes 8.1s on bundle and 18.1s on typed-full on the same machine,
on one core. It allocates 0.8M objects on bundle where this branch allocates 125M.

## Commits

Gains are measured on the same machine against the binary before the commit. "tracer" means
the per-pass times of `--tracer_mode=TIMING_ONLY`, used for changes too small to resolve in
wall-clock time (run-to-run noise is about 2%).

### Translator, tool steps, runtime

| commit | kind | change | measured |
|---|---|---|---|
| flip (`main` 1812ade) | translator | generated Go; Node's cold fields in a record of their own, the string payload in a column | bundle -2.5%, typed-full -2.8% (on transpiled Go) |
| steps 1-2 | step | `slices`: 62 of 571 lists -> `[]T`; `maps`: 114 of 702 maps/sets -> Go maps | about -2% (postflip3) |
| steps 3-5 | step | `colls`: lists, maps and sets typed across method boundaries (661, 371, 259 declarations) | neutral to +3% (postflip3) |
| step 6 | step | `dispatch`: single-implementation dispatch (134 of 547) is a concrete type assertion | neutral |
| step 7 | step | `optional`: `java.util.Optional` is a value | neutral |
| token bytes | step | `tokens/tokswitch.py`: 379 switches and 172 comparisons on the token byte | |
| overrides | step | `overrides`: 1,529 overriding classes get the method, not only `_impl` | |
| typed copies (2) | runtime | an immutable collection keeps its typed copy | |
| ArrayDeque head room | runtime | `push` writes into room at the head | |
| getFirst, UTF-16 memo | runtime | `getFirst` of a hash set starts at its first live entry; UTF-16 lookups remember their position | |
| table copy | runtime | copying a hash table into an empty one copies its index (no `hashCode` calls) | bundle -3.0%, typed-full -2.2% |
| open-addressed index | runtime | the hash table index is one slot array with stored hashes, not up to three Go maps | bundle -3.0%, typed-full -1.6% |
| hash in slot | runtime | a slot holds the key's hash beside its position (one cache miss per probe) | bundle -2.3% |
| TreeMap by string | runtime | a naturally ordered TreeMap of strings is looked up by an unboxed string | typed-full: 4.3M boxed keys fewer |
| Node keys | runtime | a Node key of a hash table hashes as its handle value | bundle -2.8% |
| charAt | runtime | ASCII checked eight bytes at a time; the last string index is checked first | typed-full parseInputs 4.9s -> 4.1-4.4s (tracer) |
| 93ecb59 | runtime | for-each over ArrayDeque, LinkedList and PriorityQueue walks their items, no iterator object | with eb573b6 |

### Manual steps

The first seven come from the earlier `postflip3` branch. They were kept because, measured on
top of the tool steps and runtime, they still paid (bundle, against 35.9s): the rename
inverter group -1.9s (-5.4%) and the Var-keyed maps -0.8s (-2.2%). Together they took bundle
from 1.29x to 1.18x. The groups that no longer paid were left out: the Go map index for graph
nodes (+0.6s), the typed reference list (+0.7s) and the property mask (+0.6s).

| commit | change | measured |
|---|---|---|
| d12f878 | rename-inverter sets and the data-flow seen set as Go maps (order cannot reach the output); `AbstractScope.vars` typed, made at the first declaration | see above |
| 35e845b | the inverter's shadow set is one pointer-keyed map per implementing class | see above |
| 531be9b | typed loops over the inverter's declared variables; source line scans in one pass | see above |
| 8d8b945, 3d0eda4, 99c1ce0, 3d4d302 | `varMap_`: Var-keyed maps keyed by (name, scope root), what `Var.equals` compares | see above |
| 13c7e5b | `Compiler.inputsById` keyed by the id's string | batch below |
| 8f32a53, eecb7a6 | RemoveUnusedCode builds a not-removable message, and its reason, only for a log that records it | batch below |
| 40eaf64 | the scope scanner looks its CompilerInput up once per input | batch below |
| ec6ed2a | RemoveUnusedCode looks polyfill names up without boxing them | batch below |
| 7786eef | interning a string that is already interned does not box it | batch below |
| 751c653 | a graph node's two edge lists are part of the node | batch below |
| 03415e9 | `getAstAnalyzer` hands out the last analyzer while what it is made of is unchanged | batch below |
| 41ceb21 | `Node.getQualifiedName` of a GETPROP chain is built in one string | batch below |
| b50a6f1 | the polyfill finder looks method names up without boxing them | batch below |
| 94c1bb3 | the parser makes a comment's builder only when there is a comment | batch below |
| cba9302 | InlineVariables' memo cells are part of the var expert that owns them | typed-full: 5.8M objects fewer |
| ebd1488 | `isConnectedInDirection(source, value, dest)` compares edge values without a predicate object | typed-full: 3M objects fewer |
| 601ba6b | the scope scanner walks declaration targets through a function value, so it stays on the stack | typed-full: 6.7M objects fewer; -0.3s |
| 20896bb | `varMap_` indexed by slots holding the key's hash; a Var hashes its key once (a field of AbstractVar) | the five passes using it, typed-full (tracer): 17.6s -> 17.2s, 18.4s -> 16.5s |
| a3cb96f | `LinkedDirectedGraph.createNode` gets or creates a node without a function object | with fbb901b |
| fbb901b | `ControlFlowAnalysis.createEdge` looks each graph node up once (six lookups before) | the four CFG passes, bundle (tracer): 7.65s -> 6.91s, 6.90s -> 6.70s |
| e86bef7 | `JSType.isSubtypeOf` takes a SubtypeChecker from a free list on the registry, reset to the constructor's state | typed-full median 68.2s -> 66.3s (5 runs); bundle 33.4s -> 31.7s |
| eb573b6 | data-flow analysis and CFG prioritization walk a node's edges, not a new list of its neighbours (5.3M lists) | with 93ecb59: bundle 31.0s -> 30.2s, typed-full 65.4s -> 63.6s; objects 125.4M -> 117.8M, 272.1M -> 262.1M |
| 0788ad4 | NodeTraversal's scope stack is a `[]any` with room for 8 | with bdfed15 |
| bdfed15 | a ReferenceCollection's list starts with room for four (4.5M growth allocations) | with 0788ad4: typed-full 64.9s -> 63.8s, bundle neutral; objects 262.1M -> 257.8M, 117.8M -> 114.4M |

The allocation batches were measured as batches (interleaved, 3 runs):
- 13c7e5b..7786eef: bundle 35.1s -> 33.9s (-3.6%), 12M objects fewer.
- 751c653..94c1bb3: bundle 35.4s -> 33.4s; typed-full 24M objects fewer, wall time unchanged.

## Tried and not kept

Each was measured as neutral or slower and reverted:

- Collections typed when their key is an interface that only identity-compared classes
  implement (a widening of `colls`): 10 more declarations, neutral.
- Index slots only for hash tables with more than 8 entries: neutral.
- Lists given a first capacity (as `java.util.ArrayList`): 4.3M objects fewer, neutral.
- A 64-bit filter of declared names per scope, skipping scopes in a lookup: neutral.
- Type switches for dispatchers with 2-4 implementations (a widening of step 6, 302
  dispatchers): neutral. Type assertions are 0.05% of the profile.
- A source range and its two positions in one allocation: 6.7M objects fewer, but slower.
- `computeIfAbsent` with a lambda taking a Go function value (25 sites): within noise.
- A direct priority comparator for the data-flow worklists: neutral.
- `Optional<String>` holding its string unboxed (runtime `Optional_ofStr_` etc. and the tool
  step `tools/postflip/optstr/optstr.py`, 1,899 sites; branch `postflip4-wip7`): typed-full
  2.9M objects fewer, wall time neutral (64.9s -> 65.1s, bundle 30.5s -> 30.7s).
- ArrayDeque with room for 16 from the start, reusing its array when emptied, and addFirst /
  removeFirst on its head room (runtime; `postflip4-wip8`): 0.4M-0.9M objects fewer, neutral.
- A batch of three, measured together against the branch and pre-flip Java (5 runs;
  `postflip4-wip12`): removing the first element of a short list shifts the rest as
  `java.util.ArrayList` does (runtime; the parser's token lookahead regrew its list on almost
  every token); a graph node has room for its first in- and out-edge (manual); and the step
  `tools/postflip/lambdas`, which makes each of 516 lambdas that capture nothing once, as
  HotSpot does. Objects 257.8M -> 238.6M on typed-full (-7.4%) and 114.4M -> 107.5M on bundle,
  bytes almost unchanged; wall and CPU time unchanged (typed-full 61.75s -> 61.86s, bundle
  31.44s -> 31.25s; Java 55.17s and 29.97s in the same run). Fewer objects alone no longer buy
  time.
- GOGC=200 and PGO: against the plan's rules.

## Where the remaining time goes

On typed-full the collector runs on other cores: collection is active about 6% of the wall
time, with no measurable mark assist. Turning it off (a diagnostic, not a setting) makes
bundle slower, 40.8s against 36.0s, because fresh memory is not in cache. So allocation costs
the main thread through the allocation path and locality, not through collection.

Passes furthest from pre-flip Java on typed-full (pass time, tracer):

| pass | Java | this branch | ratio |
|---|---:|---:|---:|
| inferTypes | 6.1s | 8.4s | 1.38x |
| inlineVariables | 2.4s | 4.6s | 1.91x |
| removeUnusedCode | 5.2s | 7.0s | 1.36x |
| flowSensitiveInlineVariables | 2.0s | 3.5s | 1.79x |
| collapseObjectLiterals | 1.7s | 2.9s | 1.68x |
| checkTypes | 1.6s | 2.7s | 1.72x |

These were measured before the varMap and CFG steps, which lowered several of them. The costs
behind them are shared:
- Reference collection and scope creation: used by many passes; jsgo does the same work 3.7x
  faster.
- Scope variable lookups: rehash the name at each level of the scope chain.
- Control-flow graphs and data-flow analysis.
- Type inference: the translated lattice code itself.

Still open:
- Scope lookups that do not create empty block scopes: 48% of the 3.3M scopes built on bundle
  are empty.

## postflip5: the Java-isms batch (JAVAISMS.md)

Seven groups were prototyped in parallel on `postflip4` (branches `postflip4-sw-*`), each gated
on its own, then merged here in dependency order: runtime changes and tool sources cherry-picked,
every tool re-run on the merged tree, each commit gated (runtime tests ok, four projects MATCH).

Measured against pre-flip Java on one machine; ratios are Java time / Go time (above 1: Go is faster):
- bundle, 3 interleaved rounds (medians): 29.94s -> 28.22s; Java/Go 0.941 -> **0.998**
  (best runs 27.85s against Java's 28.14s: 1.010).
- typed-full, 4 rounds alternating forward and backward order, so that each commit is compared
  with its neighbour in the same round (means): 61.94s -> 59.34s; Java/Go 0.875 -> **0.913**.
  Pass time (tracer, 2 runs): 57.7s -> 53.7s against Java's 49.6s (Java/Go 0.863 -> 0.924).
- Objects allocated (exact): typed-full 257.8M -> 200.4M (-22%), bundle 114.4M -> 99.4M.

| commit | kind | change | typed-full step (4 rounds) | typed-full objects |
|---|---|---|---:|---:|
| d1f9316 | - | gofmt of three files | | |
| 57df317 | runtime | removing the first element of a short list shifts the rest | -0.8% (3 of 4 rounds faster) | 254.2M |
| 18181b9 | manual | a graph node has room for its first in- and out-edge | 0.0% | 251.5M |
| e62ff07 | runtime | EnumSet is a bit set; one ordinal comparator | 0.0% | 243.5M |
| 614c26c, d01a506, 98238b0, 11648d2 | step `native` | typed lists (347), maps (100), sets (84), immutable lists (206) become slices and Go maps | within noise | 246.2M, 246.2M, 246.0M, 247.5M |
| df574d1 | step `generics` | `Optional[T]` (2,745 type arguments `string`) | within noise | 232.6M |
| dcb2004 | step `arraycopy` | System.arraycopy between arrays of one type without boxing (11) | within noise | 230.4M |
| c4914da | step `lambdas` | a lambda that captures nothing is made once (414) | within noise | 222.8M |
| fce1c68 | step `vdispatch` | `self_` typed with a named interface per hierarchy; 1,579 of 1,806 dispatch sites call through it | **-2.4% (4 of 4)** | 222.7M |
| 6503ac1 | step `casts` | `to_X`/`is_X` of small hierarchies check the concrete types first (114 + 25 helpers) | within noise | 222.7M |
| a1b0aa5 | runtime | `_f` variants of computeIfAbsent, compute, merge, forEach taking Go func values | within noise | 224.0M |
| 118c067 | step `funcparams` | 62 function-interface parameters only called become Go func values (379 calls, 57 runtime calls) | within noise | 210.0M |
| 4eaa1fe | step `precond` | Preconditions box their message arguments only when they fail (856) | within noise | 203.0M |
| 1cdb142 | manual (prototype) | Node keeps 24 boolean properties as bits in its hot record | within noise | 202.5M |
| ff3de79 | runtime | the Java-signature computeIfAbsent etc. keep their own bodies (a1b0aa5 forwarded through a method value) | within noise | 202.5M |
| 4e902b3 | runtime | removing the first element of a short native slice shifts the rest (the parser lookahead regrew) | within noise | 200.4M |

"Within noise": the step's sign differed between rounds; this host has slow periods of several
minutes, and single steps of 1% cannot be resolved in 4 rounds. Their sum is the -4.2% above.

Notes:
- 1cdb142 does not keep the order of a bit property relative to the list items on failure
  paths (which of two AST violations is reported first, name order in one exception message).
  It is a prototype for its final home, a pre-flip Java step (`Node.propBits`), which changes Java
  and Go alike.
- The immutable-list step copies immutables built by the erased builders into typed slices
  (`sliceImm_` through `tlistImm_`, which also allocates a `TList` header): +1.5M objects.
- Final home of each step (from the prototypes' reports): j2go for `generics`, `lambdas`/
  `funcparams` (functional interfaces as Go func types), `vdispatch` and `casts` (j2go knows the
  hierarchy); post-flip for `native` (needs whole-package aliasing and order analysis) and
  `precond`; runtime for EnumSet; a pre-flip Java step for the node properties.

Where typed-full still trails pre-flip Java (pass time, tracer):

| pass | Java | postflip5 | Java/Go |
|---|---:|---:|---:|
| inlineVariables | 2.37s | 3.61s | 0.66 |
| removeUnusedCode | 5.00s | 5.98s | 0.84 |
| flowSensitiveInlineVariables | 1.88s | 2.85s | 0.66 |
| inferTypes | 6.11s | 6.95s | 0.88 |
| collapseObjectLiterals | 1.62s | 2.30s | 0.70 |
| checkTypes | 1.61s | 2.17s | 0.74 |
| coalesceVariableNames | 1.01s | 1.36s | 0.74 |

## Correction: PGO (13-17 Nov)

`go/cmd/mech/default.pgo` was in the tree from 1fef152c (13 Nov) to c4aefc67 (17 Nov), and
`go build` uses a `default.pgo` in the main package's directory automatically. Every binary
built in that window was profile-guided, including the measurements of rounds 2-5 below. The
profile and `tools/pgo` are removed (c4aefc67). Measured again without PGO (no `-pgo` in
`go version -m`), against pre-flip Java, interleaved, byte-identical outputs:

| tree | typed-full Java/Go (median; rounds) | bundle Java/Go (median; rounds) |
|---|---|---|
| cc8beb35 (end of round 6) built with PGO | 1.038; 0.985-1.201 | 1.147; 1.111-1.182 |
| cc8beb35 without PGO | 1.042; 1.026-1.236, all > 1 | 1.129; 1.113-1.135, all > 1 |
| 2d204c0f (round 7 merged) without PGO | **1.082**; 1.024-1.122, all > 1 | **1.117**; 1.093-1.154, all > 1 |

PGO was worth about nothing on typed-full and 1.6% on bundle; the result does not depend on it.

## postflip5, round 8: Java-isms (merged at 72b0f693)

Six branches from 85416d7a (round 7 + the tostr re-run), each prototyped and gated by one
agent, merged here in this order: exceptions (git merge 862987cd: the branch's steps applied to
a tree that only had documentation commits since its base), casts (castfold -assert re-run),
strings (runtime and tool commits cherry-picked, strbytes, sbuild, strview and strnative re-run),
collections and immutables (runtime and tool commits cherry-picked, tool conflicts resolved by
hand, `tools/postflip/colls/run.sh` re-run: colls kinds to a fixpoint, native, nilmaps, the
immutable kinds between `tnames -undo` and `tnames`). Gated on the final tree: runtime tests ok,
checks/bundle/downlevel/advanced MATCH.

| group (branch) | steps | census (g_*.go, round 7 -> 72b0f693) |
|---|---|---|
| exceptions (r8-exc) | `throws` (3,153 throws of a new exception panic with the exception, a Go error value; exhaustive enum switches throw Java's NPE), `tryfn` (41 tries that end their function defer in it), `exceptions` (dead catches, JDK/Guava interfaces declare no checked exception, a catch around one runtime call is an error return: `Pattern_compileErr_`), `optof`, `fnobj` (15 functional interfaces only lambdas implement are `*funcObj_[F]`), `funcparams` with `_f` runtime variants, Guava Optional as a value | lambdas 201 -> 140, recover/catch 62 -> 50, defer 88 -> 82, Optional 85 -> 78, panic 3,517 -> 3,492 (3,396 panic with an error value, the rest stubs and rethrows) |
| casts (r8-casts) | `castfold -assert`: a cast whose operand cannot be null or is dereferenced at once is a Go type assertion (runtime: a nil-interface assertion is Java's NPE) | class casts 2,018 -> 1,727 (before the collection steps) |
| strings (r8-str) | `strbytes` (positions a function computes are byte offsets where nothing can tell), `sbuild` (builders passed as Appendable), `strview` (code that works in UTF-16 units reads a `utf16_` view: charAt/length/substring/indexOf as the helpers return them; view fields next to scanner/lexer string fields), `strnative` (format of an Object before constants) | String_* 973 -> 122, StringBuilder 24 -> 3 |
| collections (r8-colls) | native: shared typed lists are `*[]T`, a plain `[]T` only if never null; Go maps keyed by Var's and ExportTrace's equals fields; collections held in typed containers typed; `nilmaps` (a nil Go map read throws NPE as Java's null map); `tnames` (Go method names: Get/Put/Has/Delete/Len...) | erased constructors 228 -> 194, typed TList/TMap/TSet 431 -> 339 |
| immutables (r8-imm) | `immnames` (erased Guava types renamed Erased*), runtime ImmutableList[T]/ImmutableSet[K]/ImmutableMap[K,V] and builders with Guava's semantics and an erased twin for identity across erased code, colls imm kinds | Guava immutables 748 -> 218 |

All rows together, round 7 -> round 8: boxing 5,175 -> 3,072, class casts 2,018 -> 1,512,
interface casts 959 -> 879, unboxing 704 -> 607.

Measured (interleaved, no PGO, out.js/stdout/stderr byte-identical with pre-flip Java), against
pre-flip Java and the round-7 binary in the same run: typed-full Java/Go 1.345 (round 7: 1.295),
bundle 1.240 (1.229); round 8 is 3.7% faster than round 7 on typed-full and 0.9% on bundle. The
host was loaded (1.1-6.0) by another agent's builds, which slows pre-flip Java (2-3 cores) more
than Go, so the ratios against Java are overstated; on a quiet host round 7 measured 1.082 and
1.117. A quiet re-measurement replaces this paragraph.

## postflip5, round 7: Java-isms (merged at 2d204c0f)

Five branches (postflip5-r7-erased, -typed, -strings, -misc, -generics), each prototyped and
gated by one agent from cc8beb35, merged here in that order by cherry-picking their tool and
runtime commits and re-running each step's tool on the merged tree; each group gated (runtime
tests ok, four projects MATCH). Then a dry run (-n) of every post-flip tool on the merged tree:
only `tostr` had work left (613 conversions, 85416d7a); `colls -kind set` would convert one class
whose value comes from a generic class's type argument but does not compile (left for round 8).

| group | steps (commands) | census (g_*.go, cc8beb35 -> merged) |
|---|---|---|
| erased collections | `colls/run.sh` (list, map, set, deque, treeset, treemap, idmap, then native); runtime TTreeSet/TTreeMap, LinkedIdentityHashMap on a typed map | erased constructors 278 -> 228 |
| loops and iterators | `ranges -index`, `ranges -entries`, `iters`; runtime immElems_, immCursor_, typed cursors | for-each cursors 198 -> 101, explicit iterators 123 -> 102 |
| strings | `strnative`, `strcuts/run.sh`, `strunits` | String_* 1,184 -> 973 |
| misc | `nnchecks` (hoisting), `exceptions`, `nnchecks` (Optional tests), `optof`, `instof`, `eqtyped`, manual thread names, `passfn`, `funcparams` | Preconditions 76 -> 3, lambdas 396 -> 201, equals/hashCode 153 -> 48, Optional 103 -> 85, instanceof 79 -> 72, try/catch 68 -> 62, threads 19 -> 13 |
| generic classes | `generics/classes.sh` (25 classes are Go generic types), then `castfold`, `castfold -instof`, `casts -named`, `anyconv -all`, `objslots` | boxing 6,039 -> 5,175, unboxing 791 -> 704, class casts 2,192 -> 2,018, interface casts 969 -> 959, any declarations 2,128 -> 1,852 |

## postflip5, rounds 2-5: beating pre-flip Java

These measurements were made with PGO (see the correction above).
Ratios are Java time / Go time (above 1: the transpiled Go + post-flip steps is faster).

**Result** (postflip5 tip, one machine, idle, pre-flip Java and Go alternating ABBA; out.js,
stdout and stderr byte-identical to pre-flip Java on both tasks):

| task | pre-flip Java (median) | postflip5 (median) | Java/Go medians | per round |
|---|---:|---:|---:|---|
| typed-full (8 rounds) | 50.88s | 48.31s | **1.053** | 1.015-1.088, all > 1 |
| bundle (6 rounds) | 26.12s | 23.14s | **1.129** | 1.066-1.246, all > 1 |

The session moved to a different machine between the round-1 measurements above and these, so
only ratios within one run compare. On this machine, at the end of round 1 (tip ff3de79..4e902b3)
typed-full was 0.987 and bundle 1.111.

Each group below was prototyped on its own branch by one agent (gated there), merged here by
re-running its tools and cherry-picking its runtime and manual commits, and gated again as a
group (runtime tests ok, four projects MATCH). Measurements are the agents' per-pass tracer A/B
(ABBA, under a lock so that no two timings overlapped) and the milestone runs on this branch.

| group (branch) | steps | measured |
|---|---|---|
| dispatch (r2-dispatch2) | tool `vdispatch -sub`: a method only part of a hierarchy has is in the root's interface (140 sites); `-enums`: enums with constant bodies dispatch through their own interface (87 sites). No dispatch through an anonymous-interface assertion is left | dispatch microbenchmark: itab call 2.7-3.0ns, anonymous assertion 4.1-5.3ns |
| erased collections (r3-erased) | runtime: typed maps/sets of keys with equals hash as HashMap does; deques as typed lists; tools `colls` (list, map, set, new deque kind) and `native` (list, set) re-run: erased constructors 456 -> 278 | milestone step -0.3% (within noise) |
| enums (r3-enums) | tool `enums`: 192 of 228 enums are uint8 types (0 = null), fields as tables; null-safe `ordinal()`/`name()` (51b6058, f5b5ffe: ordinal is value-1, no table load) | milestone step -1.0% (4 of 4 rounds faster); ordinal -0.45% (6 of 6) |
| node properties | the prototype 1cdb142 reverted (failure-path order deviation); its Java prep form (main-r2-nodeprops, prep 04c) slows pre-flip Java by 1.7% (bundle) and 1.9% (typed-full), so it is not adopted | revert +1.6% on typed-full |
| immutable builders (r2-immbuild) | runtime: sliceImm_ without a TList, Guava-like builder growth; tool `immbuild` (51 builders, 42 copyOf/of sites) | milestone -0.9% (3 of 4); typed-full objects -2.5% |
| data flow (r2-dataflow) | runtime: Comparator combinators as comparator types, PriorityQueue int-key fast path; manual: CFG priority on the node directly, pointer-keyed seen set | 64-node heap round 14.4 -> 4.8us |
| scopes (r2-refcoll) | runtime `scopeVars_` (declaration-ordered, LinkedHashMap semantics); manual: hash a name once per lookup chain, implicit vars as an array by ordinal, block and scope stacks typed | milestone -2.0% (4 of 4); objects -4.0% |
| boxing (r3-boxing) | tools `objslots`, `charseq`, `strkeys`; runtime and manual: PMap/HamtPMap Go generic | objects -1.2% |
| misc (r3-misc) | tool `funcparams` extended (fields, interface groups, Guava predicates; `rw.SaveChecked`); tool `eqhash` (equals/hashCode decided from static types); tool `sbuild` (append-only StringBuilders as strings.Builder); manual: compiler "thread" runs its task directly | objects -1.3% |
| types (r5-types) | tool `selfeq` (exact-object check as a type assertion, no efaceeq call; 1,494 sites); runtime: Node keys first in htable_, string-key TreeMap key slice; manual: Node.jstypeOrColor holds the *JSType (7 sites) | checkTypes -10.7%, typesToColors -10.3%, all passes -2.4% |
| code generation (r5-codegen) | tool `strunits` (loops over a string's UTF-16 units, 20 loops); runtime: ImmutableSet.copyOf(EnumSet) stays an enum set, asciiCharAt_ | time after the passes 2.0s -> 1.6s (Java 1.5s) |
| flow-sensitive inlining (r5-fsiv) | runtime: comparingInt_f is an int-key comparator; tool `arraycopy` covers Arrays.copyOf; manual: work-queue marker in the graph node, NodeTraversal's own scope-stack room, astPosition a Go map | flowSensitiveInlineVariables -7.4%, deadAssignmentsElimination -8.8%, inferTypes -4.3% |
| inlining / purity (r5-inlvar) | runtime: typed-map iteration starts at its first live entry (LinkedHashSet work queues); manual: Node's property-list head in a column of its own (47 accesses, one pattern) | markPureFunctions 1180 -> 785ms (Java 670); passes total -2.6% |

Not kept: `varMap_.slot` for computeIfAbsent in addReference (within noise, possibly slower).

