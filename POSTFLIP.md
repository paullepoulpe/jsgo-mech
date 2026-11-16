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
- A string-specialized `Optional`: prep step 10's `Optional<String>` boxes the string at 912
  sites (about 5.5M boxings on typed-full).
- Scope lookups that do not create empty block scopes: 48% of the 3.3M scopes built on bundle
  are empty.
