# Post-flip steps (branch `postflip4`)

This branch is the post-flip half of the migration plan, built only from reproducible pieces.
Each commit is one of:

- **translator**: the flip itself, i.e. the Go that `tools/j2go/run.sh` and
  `tools/prune/prune.sh` produce from `main`. The node layout comes from j2go rules
  (`tools/j2go/handles.txt`), not from edits of the generated Go.
- **step**: a tool in `tools/postflip` rewrites the generated Go. The tool is in the tree, and
  rerunning it on the same input gives the same commit (`replay.sh` replays steps 3-7).
- **runtime**: a change to the handwritten Go runtime (`go/closure/rt_*.go`). No generated file
  is edited.

No commit edits a generated `g_*.go` file by hand. Every commit passes `tools/prune/verify.sh`:
the runtime tests and four projects compiled byte-identically with pre-flip Java. From step 7
on, the translator self-test no longer applies. There is no PGO and no GOGC setting.

## Commits

Gains are wall-clock time on one machine, from interleaved runs of the binaries before and
after the commit (2 runs each, nothing else running).

| commit | kind | change | bundle | typed-full |
|---|---|---|---:|---:|
| flip (`main` 1812ade) | translator | generated Go; Node's cold fields in a record of their own, the string payload in a column | -2.5% | -2.8% |
| steps 1-2 | step | `slices`: 62 of 571 lists -> `[]T`; `maps`: 114 of 702 maps/sets -> Go maps | | |
| steps 3-5 | step | `colls`: lists, maps and sets typed across method boundaries (661, 371, 259 declarations) | | |
| step 6 | step | `dispatch`: single-implementation dispatch (134 of 547) is a concrete type assertion | | |
| step 7 | step | `optional`: `java.util.Optional` is a value | | |
| token bytes | step | `tokens/tokswitch.py`: 379 switches and 172 comparisons on the token byte | | |
| overrides | step | `overrides`: 1,529 overriding classes get the method, not only `_impl` | | |
| typed copies (2) | runtime | an immutable collection keeps its typed copy | | |
| ArrayDeque head room | runtime | `push` writes into room at the head | | |
| getFirst, UTF-16 memo | runtime | `getFirst` of a hash set starts at the first live entry; UTF-16 lookups remember their position | | |
| table copy | runtime | copying a hash table into an empty one copies its index (no `hashCode` calls) | -3.0% | -2.2% |
| open-addressed index | runtime | the hash table index is one slot array with stored hashes, not up to three Go maps | -3.0% | -1.6% |
| hash in slot | runtime | a slot holds the key's hash beside its position (one cache miss per probe) | -2.3% | ~0 |

The steps before "table copy" were measured together on the earlier `postflip3` branch
(POSTFLIP.md there): steps 1-2 about -2%; typed collections, dispatch and `Optional` between
neutral and +3%. Their purpose is idiomatic code; they do not buy speed.

## Against pre-flip Java

Interleaved runs on the same machine:

| | pre-flip Java | plan through the runtime commits up to "getFirst" | plan, all commits |
|---|---:|---:|---:|
| bundle | 28.0s | 39.3s (1.38x) | 36.2s (**1.29x**) |
| typed-full | 53.4s | 72.0s (1.36x) | 68.9s (**1.29x**) |

## Tried and not kept

All of these were measured as neutral, or only on the old branch, and were removed:

- Collections typed when their key is an interface that only identity-compared classes
  implement (a widening of `colls`): 10 more declarations, neutral.
- A `TreeMap` lookup by string without boxing the key: neutral.
- Index slots only for hash tables with more than 8 entries: neutral.
- GOGC=200: collector tuning, against the plan's rules.
- PGO: build configuration, against the plan's rules.

## Hand edits of the earlier branch (not in the plan)

`postflip3` also had 11 one-off edits of generated files in hot passes. Among them:

- Var-keyed maps in RemoveUnusedCode, MustBeReachingVariableDef and ReferenceCollector.
- A Go map index for graph nodes.
- Typed loops and maps in the rename inverter.
- A typed reference list.
- A mask of a node's properties.
- The unremovable-log message built only when logging is on.

Together they were worth 5.8s on bundle and 4.3s on typed-full (postflip3 against this branch
before "table copy"). They are left out of the plan because one-off edits are not a
reproducible step.

To see which of their patterns is worth turning into a tool step, each group was applied to
this branch with all commits (cherry-picked onto a scratch branch, all gates pass) and timed
against it on bundle (interleaved, 2 runs each; the branch itself took 35.9s):

| group | edits | bundle |
|---|---|---:|
| G3 | rename inverter sets as Go maps (also split per implementing class); `AbstractScope.vars` typed and created lazily; data-flow seen set as a Go map; typed loops in the inverter | **-1.9s (-5.4%)** |
| G1 | Var-keyed maps (`varMap_`, keyed by name and scope root) in RemoveUnusedCode, MustBeReachingVariableDef and ReferenceCollector | -0.8s (-2.2%) |
| G2 | Go map index for graph nodes (on top of G1) | +0.6s against G1 |
| G4 | typed reference list | +0.7s |
| G5 | mask of a node's properties | +0.6s |
| G6 | unremovable-log message only when logging is on | 0 |

Most of what the hand edits used to buy, the runtime commits now give everywhere. Only two
patterns still pay: G3 and G1. Splitting G3 (base 36.6s in that run):

| part | bundle |
|---|---:|
| rename-inverter sets as Go maps, `AbstractScope.vars` typed, data-flow seen set as a Go map | -0.8s |
| plus the inverter's shadow set split per implementing class | -1.3s |
| plus typed loops over the inverter's declared variables, one-pass source line scans | -2.4s |

What blocks the tools on these collections:
- `DataFlowAnalysis.seenSet` and `AbstractScope.vars` hold a Java type variable, which j2go
  erases to `any`. No step can type a collection of `any`; step 2 alone leaves 138 maps and
  sets with a key that is "compared by equals, or is not a string, handle or class".
- The inverter's sets are iterated, but only to fill other sets. Step 2 only takes
  collections that are never iterated.
- `Var` has `equals` (in `ScopedName`: name and scope root). Steps 4-5 only take keys that Go's
  `==` compares as Java does.

The general steps these would need, in order of measured value:
1. Iteration that only fills unordered collections does not make order observable (a widening
   of step 2).
2. A collection keyed by a class whose `equals` compares a fixed tuple of comparable fields is
   keyed by that tuple (a widening of steps 4-5; `Var` is the main case).
3. A type variable erases to its bound (a translator rule; it touches all generic code).

## Where the remaining time goes

Profile of bundle with all commits (41s CPU):

| bucket | share |
|---|---:|
| GC | 14% |
| hash tables and hashing | 16% |
| Node record reads | 13% |
| allocation | 8% |
| casts and interface conversions | 5% |
| translated pass code | 36% |

Passes furthest from Java on bundle:

| pass | Go / Java |
|---|---:|
| flowSensitiveInlineVariables | 2.0x |
| collapseObjectLiterals | 1.95x |
| removeUnusedCode | 1.67x |
| inlineVariables | 1.65x |
| inferConsts | 1.62x |
| invertContextualRenaming | 1.55x |
| coalesceVariableNames | 1.55x |

No single item is above about 2% of the time.
