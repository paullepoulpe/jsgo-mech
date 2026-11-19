# jsgo performance history: what was done, and what jsgo-mech can take from it

Repository studied: `/home/ubuntu/work/jsgo` at `71ff08f4` (2026-11-12), read-only. No builds or benchmarks were run; every number below is quoted from a jsgo commit message or a jsgo document, with its source. Companion data: `commits.tsv` (one row per performance commit) and `not_perf_candidates.tsv` in this directory. Paths such as `docs/...` and `post_flip_optimizations.md` are relative to the jsgo repository.

## Headline findings

1. **jsgo's own history is the jsgo-mech experiment run backwards.** jsgo began (2026-10-16/17) as a rule-based 1:1 hand port (`PORTING.md`: `self` back-pointers, `*string` for nullable strings, Java-emulating `jutil` collections, a `[]uint16` scanner, re-entrant monitors). On that port jsgo was "0.65–0.74× of warm Java's speed" (`docs/performance-roadmap.md:25-32`), with "~37% of CPU samples to memory management" (`:41-47`). That is where jsgo-mech is now.
2. **The "L0 / post-flip baseline" is not a measured mechanical translation.** It is the hand-optimised tree with 23 named optimisations switched off (`post_flip_optimizations.md:11`, `:68`; `migration_plan.md:286`). Everything else in the ~280 gain-bearing commits is still in L0: the 32-byte pointer-free node record, string ids, flat collections, unboxed nullable strings, devirtualised types, a hand-written protobuf codec, parser/scanner changes, and many pass-specific fast paths that are not among the 23. The plan's claim that mechanical Go + NodeStore reaches L0 carries an explicit "+10%–18% allowance (estimated)" (`migration_plan.md:301`). The L0 binary (`jsgo-mincore-switches`) is not in the repository.
3. **The one pre-flip item jsgo's documents isolate is the node store, and it is the largest single measured effect in the history.** Same tree, heap `*Node` instead of NodeStore, 1 thread: 221.31 s / 11,112 MB against 133.98 s / 6,542 MB (`migration_plan.md:297`); CHECKS alone 135.98 s against 73.08 s, CPU 269.2 s against 82.0 s (`:294`). Java at 4 threads is 208.7 s / 12,610 MB, CHECKS 97.55 s. jsgo with heap nodes loses to Java by 6 % overall and 39 % in CHECKS.
4. **Of the measured post-flip gain (L0 → HEAD, −46.5 s on googleapis-bench), about 80 % is out of bounds for jsgo-mech**: 63 % is parallelism added to passes, 14 % pass-specific algorithms, 1.6 % PGO. The largest in-bounds candidate is the traversal token masks (−6.6 s, 14 %), and only if the masks are derived by a tool rather than declared by hand as jsgo did.
5. **Timing matters for the mechanically reproducible changes.** Unboxing, collection and devirtualisation changes gave large allocation cuts on the raw 1:1 port (checks-only 52.9 M → 32.7 M mallocs across `46219a0a` … `eba38339`, per their commit bodies). The same kinds of change applied late (rounds 62–90) measured "performance-neutral" to 1.04× (`docs/benchmarks.md` sections for `a95c4a93`, `7afbb2fe`, `65c3133d`). jsgo-mech is in the early situation.

## (i) Method and coverage

- **History**: `git log --all` has 2,452 commits (2,072 non-merge, 380 merges) across `main` (2,158) and 97 remote branches (294 commits not on main), dated 2026-04-17 to 2026-11-12: 1,009 commits on 2026-04-17..25, none between, 1,443 on 2026-10-16..11-12.
- **April (1,009 commits)** is the "first attempt" (`legacy/`), deleted by `66bdf96b` on 2026-10-16. Subjects were screened by keyword only; none is a performance commit.
- **October–November (1,211 non-merge commits)**: every subject was read. 490 were selected as candidates (keyword match plus manual additions; identical subjects across branches collapsed to one row, preferring the copy on main). For each, the commit message and `--stat` were read, and the diff or the docs where the message was empty.
- **Result**: 374 performance rows; 116 candidates rejected (`not_perf_candidates.tsv`).

| Kind | Rows | On main | Meaning |
|---|---:|---:|---|
| PERF | 200 | 200 | purpose or measured effect is speed or memory |
| REFACTOR-PERF | 55 | 55 | idiomatisation that replaces a Java idiom on hot paths, or claims/measures a perf effect |
| PERF-EXPERIMENT | 26 | 0 | prototypes on side branches |
| PERF-FIX | 49 | 17 | fixes a bug, race or parity break introduced by a performance commit |
| PERF-REVERT | 44 | 40 | removes or simplifies away an earlier optimisation |
| **Total** | **374** | **312** | |

"Gain-bearing" = PERF + REFACTOR-PERF + PERF-EXPERIMENT = 281 rows. 221 of the 374 rows (196 of the 281) carry a stated measurement.

- **How it was decided**: a commit is a performance commit if its message, docs or diff show the purpose or a measured effect on time, allocation or memory. "Round N" commits bundle many items; they are classified by the dominant item, with items listed in `what`. Classification was done by five parallel readers under one written rubric, plus 9 rows by hand; outputs and key rows were reviewed, but not all 374 diffs were re-read.
- **Could not be determined**
  - 195 of the 374 commits have an empty message. They were classified from subject, file list, added declarations and docs; such rows say "inferred:" in `notes`.
  - Many measurements cover a group of commits (roadmap rounds, benchmark sections covering 2–5 commits). The row says "not separated" or "together".
  - Whether the 32 branch-only fix commits reached main under another subject; which earlier commit each fix repairs (mostly inferred from test names).
  - Measurements come from four machines and several Go versions; figures from different rows are not comparable.
  - jsgo-mech was not inspected. A RUNTIME item may already exist there.

## (ii) Taxonomy

| Code | Definition |
|---|---|
| LAYOUT | Data representation: node store, ids/handles instead of pointers, struct packing, pointer-free storage, side tables, compaction |
| ALLOC | Allocation avoidance: slabs/arenas/pools, preallocation, scratch reuse, avoiding boxing, escapes and closures, value types |
| ALGO | Different algorithm or complexity, caches/memoisation, indexes, incremental recomputation, pass-level pre-filters |
| PAR | Parallelism and the locking changes that enable it |
| DISPATCH | Traversal/dispatch shortcuts: token bitmasks, visitor pre-filters, kind tags, devirtualisation, inlining |
| STRING | UTF-8 vs UTF-16, interning and string ids, zero-copy slicing |
| IO | I/O, serialisation (protobuf, TypedAST, save-state), compression, source-map output |
| BUILD | PGO profile, GOGC/GOMEMLIMIT, toolchain |
| COLL | Replacing Java-emulating/boxed collections by typed or native ones, or a faster collection runtime |
| JAVAISM | Faster only because a Java idiom (monitors, boxed nullables, varargs Preconditions, reflection, class hierarchies) became the Go idiom |
| RETAIN | Lifetime: releasing or not pinning memory (added from the data) |

Feasibility codes: TRANSLATOR, JAVA-PREP, POSTFLIP-TOOL, RUNTIME, NOT-ALLOWED (algorithmic / parallelism / pgo-gc), ALREADY-GENERAL, N/A for fixes and reverts.

## (iii) Distribution by class

| Class | All 374 rows | Gain-bearing (281) | Share of 281 | As secondary | With a stated measurement |
|---|---:|---:|---:|---:|---:|
| ALLOC | 90 | 72 | 25.6 % | 41 | 48 |
| LAYOUT | 51 | 45 | 16.0 % | 27 | 36 |
| COLL | 35 | 29 | 10.3 % | 25 | 22 |
| ALGO | 44 | 28 | 10.0 % | 24 | 14 |
| PAR | 34 | 27 | 9.6 % | 6 | 20 |
| JAVAISM | 27 | 24 | 8.5 % | 23 | 18 |
| STRING | 23 | 19 | 6.8 % | 15 | 16 |
| DISPATCH | 25 | 17 | 6.0 % | 22 | 8 |
| RETAIN | 32 | 7 | 2.5 % | 8 | 6 |
| BUILD | 7 | 7 | 2.5 % | 3 | 3 |
| IO | 6 | 6 | 2.1 % | 16 | 5 |

The 93 fix/revert rows: 49 fixes (25 on 2026-10-20 alone, after arena rounds 6–9) and 44 reverts, of which 25 are RETAIN (the memory round that deleted slabs, pools and cross-pass caches).

### Share of measured gain per class

Only one data set attributes whole-program time to individual optimisations with one denominator: the L0 → HEAD ladder on googleapis-bench (`post_flip_optimizations.md:126-150`, L0 128.71 s, HEAD 82.20 s, gap 46.51 s).

| Class | Ladder entries | googleapis wall Δ | Share of 46.51 s |
|---|---|---:|---:|
| PAR | #3 TypeCheck −4.46, #4 mid-end −7.75, #7 peephole −1.21, #9 stage-1 −9.08, #11 serde −4.92, #13 InlineFunctions −1.22, #19 codegen −0.54 | −29.18 s | 62.7 % |
| ALGO | #8 ProcessDefines −1.65, #10 InlineObjectLiterals −1.88, #12 incremental RUC −1.92, #16 PropUseIndex −1.27, #2 Ryū −0.05, #21 IF cache +0.10 | −6.67 s | 14.3 % |
| DISPATCH | #5 NodeTraversal bitmasks −6.60, #17 conformance mask 0.00 | −6.60 s | 14.2 % |
| STRING + ALLOC | #14 UTF-8 scanner, WTF-8 checkpoints, parser slabs | −1.68 s | 3.6 % |
| LAYOUT | #15 flat Scope −1.05, #18 flat PropertyMap/HAMT −0.40, #22 moving compaction +0.09 | −1.36 s | 2.9 % |
| IO | #6 FastGzip −0.81 (#23 binary save-state, +2.56 s, is opt-in, not in HEAD) | −0.81 s | 1.7 % |
| BUILD | #1 PGO | −0.74 s | 1.6 % |
| COLL | #20 128-var bitset | −0.15 s | 0.3 % |

Entries sum to −47.19 s against a 46.51 s gap (the document reports 97.5 % additivity for the substrate items, `:74`).

Measured outside that ladder (different machines and baselines, so no shares):
- LAYOUT, node store: 221.31 s → 133.98 s and 11,112 → 6,542 MB at 1 thread (`migration_plan.md:297`).
- LAYOUT, node-id migration vs the previous pointer store: typed-full 30.15 → 24.63 s (−18.3 %), RSS 552 → 426 MB (`docs/memory-research/round3-flat-slices.md`, "Result against main").
- RETAIN/ALLOC, memory round 1 (slabs and pools removed): peak live heap −34 to −62 %, wall +9 to +19 % (`docs/memory-research/round1-results.md`).
- ALLOC/COLL/JAVAISM, first day on the 1:1 port: checks-only mallocs 52.9 M → 32.7 M (`46219a0a` … `eba38339`).
- The 85 gain-bearing rows without a stated number are unmeasured.

## (iv) Distribution by jsgo-mech feasibility

| Judgment | Gain-bearing rows (281) | Share | With a stated measurement | All 374 rows |
|---|---:|---:|---:|---:|
| NOT-ALLOWED | 128 | 45.6 % | 76 | 128 |
| — algorithmic | 98 | 34.9 % | | |
| — parallelism added to passes | 23 | 8.2 % | | |
| — PGO / GC | 7 | 2.5 % | | |
| RUNTIME | 81 | 28.8 % | 65 | 82 |
| TRANSLATOR | 57 | 20.3 % | 42 | 57 |
| POSTFLIP-TOOL | 9 | 3.2 % | 8 | 9 |
| JAVA-PREP | 3 | 1.1 % | 2 | 3 |
| ALREADY-GENERAL | 3 | 1.1 % | 3 | 18 |
| N/A (fixes, reverts) | 0 | | | 77 |

Class × feasibility (gain-bearing rows):

| Class | TRANSLATOR | JAVA-PREP | POSTFLIP-TOOL | RUNTIME | ALREADY-GENERAL | NOT-ALLOWED |
|---|---:|---:|---:|---:|---:|---:|
| ALLOC | 15 | 0 | 1 | 10 | 0 | 46 |
| LAYOUT | 8 | 1 | 0 | 25 | 1 | 10 |
| COLL | 7 | 0 | 5 | 12 | 0 | 5 |
| ALGO | 0 | 0 | 0 | 3 | 0 | 25 |
| PAR | 0 | 0 | 0 | 4 | 0 | 23 |
| JAVAISM | 21 | 0 | 0 | 2 | 0 | 1 |
| STRING | 0 | 0 | 1 | 15 | 1 | 2 |
| DISPATCH | 6 | 1 | 2 | 1 | 1 | 6 |
| RETAIN | 0 | 1 | 0 | 4 | 0 | 2 |
| IO | 0 | 0 | 0 | 5 | 0 | 1 |
| BUILD | 0 | 0 | 0 | 0 | 0 | 7 |

Weak spot addressed by the 150 allowed rows: MEMORY 52, BOXING 39, CHECKS 15, CASTS 11, LAMBDAS 5, PROTOBUF 4, PARSE-PAR 4, none/other 21.

By measured gain on the ladder: NOT-ALLOWED ≈ 36.6 s (parallelism 29.18, algorithmic ≈ 6.7, PGO 0.74) ≈ 79 %; possibly allowed ≈ 7.8 s (#5 6.60 if tool-derived, #6 0.81, #18 0.40) ≈ 17 %; #14 and #15 (2.73 s) are mixed (WTF-8 offset checkpoints and the scope index are runtime; scanner rewrite, parser slabs, Bloom and skip pointers are hand work).

## What the pre-flip baseline contains that a mechanical translation does not

From `git log --follow internal/rhino/node_store.go`, the docs and current sources.

| Area | State in jsgo HEAD (and so in L0) | Commits | Mechanical equivalent |
|---|---|---|---|
| AST nodes | `type Node uint32`; 32/36-byte `nodeData` with no Go pointer (4 link ids, 32-bit kind-dependent payload, source-file id, property bits, packed line/column, 16-bit length, token, kind flags) in 16,384-record blocks behind a process-wide block directory (`internal/rhino/node.go:878`, `node_store.go:13-46`) | `610961c8` → `7d2d8793` (144 → 96 B) → `3fe3431a` (80) → `2078675b` (64) → `f6aaec57` (32-bit links, 56) → `8ad9a8a0` → `81244cdb`/`fb8adf80` (uint32 id, 48) → `7a39261d` (40) → `7f2de883` (36) → `64a7a488` (32) → `0276ff52` (36, 32-bit source-file id) | RUNTIME store + JAVA-PREP "Node.java storage encapsulation" (`migration_plan.md:82-84`) + translator lowering `Node` to a value handle |
| Node side data | JSType/Color as 32-bit handle in a per-block column; JSDoc and cold fields (~1 % of nodes) in a 16-shard map; original name in a paged column (`node_tables.go:10-31`, `:258-265`, `:327-340`, `:445-450`) | `7a39261d`, `f842d6da`, `2a0fc25f` | RUNTIME |
| Strings | Go strings holding WTF-8 with UTF-16-semantics helpers; interned strings are 32-bit `strpool.ID` in nodes, tokens, Var names, property keys | `eb45acc0`, `e51f666b`, `c944567c`, `d063a329`, `d29c3f47`, `f0a308f8`, `65c3133d` | RUNTIME string pool; id-keyed maps inside passes were hand work |
| Scopes/variables | `Var` 40 B with `nameID uint32` and a pass scratch field; `Scope` with var slice, pointer-free index, `lookupParent` skip pointer (`internal/jscomp/scope.go:44-53`) | `45cf400d`, `c944567c`, `0894209c`, `73613824`, `ec9611dd` | packing is TRANSLATOR; index is a RUNTIME collection; skip pointers and Bloom are algorithmic |
| Types | `JSTypeBase` 32 B, kind tags instead of `Is*Type` virtual calls, `FunctionType`/`Color` concrete | `cd0eaedb`, `67b1a658`, `a8eaf110`, `d1c35b97`, `54998951` | TRANSLATOR (closed-hierarchy rule) |
| Collections | `jutil.OrderedMap/OrderedSet` (entry slice + lazy index), Go maps/slices where order is unobserved; Java-emulating collections deleted | `46219a0a`, `45cf400d`, `5021f936`, `5cd39f45` … `21f33b8b`, `a822a48e`, `7a9c2667` | RUNTIME + TRANSLATOR/POSTFLIP-TOOL lowering |
| Protobuf | No protobuf dependency (`go.mod`); typed message structs generated from the jar's descriptors by `tools/oracle/java/ProtoGoGen.java` plus a hand-written wire runtime (`internal/jscomp/proto_runtime.go:14-23`); TypedAst encoded straight to wire bytes | `0b82dcb8`, `08c04c79`, `59137479`, `2b9a4910` | generator + RUNTIME wire codec |
| Parallel parsing | Port of upstream `PrebuildAst.java` (`internal/jscomp/prebuild_ast_parallel.go:31-34`) with per-worker node-id ranges and a 64-shard string pool | `bd01d6e6`, `3fe3431a`, `30624a61`, `1e84e983` | ALREADY-GENERAL (upstream does it) + RUNTIME executor and thread-safe store |
| Monitors | `sync.Mutex` where non-re-entrant, no goroutine-identity stack walks | `d9b20de3`, `30624a61`, `f0e21869`, `19c8aa38` | TRANSLATOR |
| Nullable strings / boxed values | `string` + flag or sentinel instead of `*string`; nullable enums as values | `01ebfe70`, `b7fd5330`, `4109a142`, `33407955`, `65c3133d`, `bc9b0283` | JAVA-PREP (`Optional<String>`, `migration_plan.md:95`) or TRANSLATOR |
| Pass-specific fast paths outside the 23 | e.g. O(1) CFA position table, zero-clone inlined call args, lazy GlobalNamespace scopes, dense SortedDependencies graph | rounds 4–22, 2A–2V | NOT-ALLOWED; their share of L0's lead is not measured anywhere |

Two cautions from jsgo's own measurements:
- A slab-allocated pointerful 64-byte `Node` tree (main at `20b2f6db`) ran googleapis at 71 + 54 + 10 s = 135 s single-threaded with 7,368 MB peak (`docs/memory-research/round2-results.md`, googleapis table, single runs). The pointer-free store's time advantage over that state was small; its advantage is against one-by-one heap nodes (221 s) and in memory. Inference: for jsgo-mech's 108-byte pointerful store, node size and allocation pattern matter as well as GC tracing.
- The first pointer-free version was a loss: +43 % wall on googleapis CHECKS and OPTIMIZATIONS (same table), because id → record translation did not inline. It became a gain with the block directory (`94b457c8`: typed-full 27.11 → 25.26 s) and on-demand blocks (`fb8adf80`).

## (v) Ranked opportunities for jsgo-mech (allowed under its rules)

| # | What jsgo did (commits) | Measured or stated effect in jsgo | Form in jsgo-mech | Weak spot |
|---:|---|---|---|---|
| 1 | Pointer-free node store, `Node` = uint32 id into on-demand 16,384-record blocks, two-load lookup that inlines (`f6aaec57`, `8ad9a8a0`, `81244cdb`, `0366e4a3`, `fb8adf80`, `94b457c8`) | Heap `*Node` 221.31 s / 11,112 MB → 133.98 s / 6,542 MB at 1 thread; CHECKS 135.98 → 73.08 s (`migration_plan.md:294-297`). Vs the previous pointer store: typed-full −18.3 % wall, −22.8 % RSS (`round3-flat-slices.md`) | RUNTIME store; TRANSLATOR lowers `Node` to a value type, `== null` to `IsNil()`, `Map<Node,V>` keys unboxed (`migration_plan.md:230`). jsgo did the 10,000 type changes with "a type-aware rewrite tool" (`round3-flat-slices.md` §8) | MEMORY, CHECKS |
| 2 | Shrink the record with side tables (`7d2d8793`, `7a39261d`, `7f2de883`, `64a7a488`, `f842d6da`, `2a0fc25f`, `0276ff52`) | 144 → 96 B: checks-only peak RSS 768 → 660 MB (`7d2d8793`); 48 → 40 B: peak live −5 to −11 % (`7a39261d`); 40 → 36 B: typed-full peak live 212/221 → 205/214 MB (`round4-heap-peaks.md` cut 2) | RUNTIME, with JAVA-PREP flattening `Node`'s private subclasses and `PropListItem` behind storage helpers | MEMORY |
| 3 | Store variants measured and rejected: 16-byte handle (+4 % time, +8–12 % RSS), flat slice via package variables (+17 % / +27 % time), flat slice with block directory (RSS +24–31 %) (`9d6b8981`, `81244cdb`, `0366e4a3`) | table in `round3-flat-slices.md` §8 | Design constraint for #1: keep every node accessor under Go's inline budget; no reserved capacity | MEMORY |
| 4 | Collection runtime without per-entry objects: no hash index up to 8 entries, entries in one slice with int32 links, pointer-free open-addressing index (`46219a0a`, `4b90b390`, `45cf400d`, `5021f936`, `5553ed7e`) | typed-app 38.4 M mallocs / 1,776 MB → 33.2 M / 1,473 MB (`46219a0a`); typed-full RSS 923 → 845 MB (`5021f936`). Entries were 31–40 % of allocated bytes on the 1:1 port (`gclab-2026-10-18.md` §3) | RUNTIME only, API unchanged | BOXING, MEMORY, CHECKS |
| 5 | Nullable `String` without `*string`; nullable enum/Boolean as value with sentinel (`a6613443`, `b7fd5330`, `01ebfe70`, `4109a142`, `9e0d389c`, `33407955`, `65c3133d`, `bc9b0283`) | checks-only 45.52 M → 36.79 M mallocs across `b7fd5330`..`4109a142`; googleapis alloc_objects 121.65 M → 87.51 M with other items (`9e0d389c`). Late sweeps neutral (`benchmarks.md` section `65c3133d`) | JAVA-PREP `Optional<String>` (plan PR 10) or TRANSLATOR `(string, ok)`; TRANSLATOR rule for nullable enums | BOXING, CHECKS |
| 6 | `Preconditions` without boxing; typed `checkNotNull` for nodes (`2778cc1f`, `2f10fb16`, `721df604`, `internal/rhino/node.go:901-918`) | parse benchmark 8.3–9.5 s → 6.6–6.8 s per pass; variadic boxing was 8 % of time (`2778cc1f`) | TRANSLATOR: fixed-arity overload at every 1–3 argument call; generic `CheckNotNull[T]` | BOXING |
| 7 | `sync.Mutex` for non-re-entrant `synchronized`, no stack walk for owner identity (`d9b20de3`, `30624a61`, `ff4aed48`, `f0e21869`, `19c8aa38`) | bundle-advanced-ts 43.29 → 12.09 s (`f0e21869`, with a disk-read fix); googleapis OPTIMIZATIONS 50.0 → 45.1 s (`19c8aa38`, with other items) | TRANSLATOR: prove non-re-entrancy per class, else a cheap re-entrant lock | CHECKS/other |
| 8 | String runtime that never converts to UTF-16 per call (`5143a631`, `e4223d2c`, `6e6407af`, `4ee930b4`, `c161e117`, `322f7a4e`, `10df6e7e`, `131eca4b`) | downlevel-whitespace 7.41 → 2.62 s (`5143a631`); checks-only 19.79 → 18.85 s (`e4223d2c`); "~5% in Java emulation" on the 1:1 port (`performance-roadmap.md:51`) | RUNTIME | CHECKS/other |
| 9 | Closure and iterator elimination (`563aa27b`, `e8de9596`, `eba38339`, `61af448a`, `cf1392da`) | typed-app 48.8 M → 40.9 M mallocs (`563aa27b`); googleapis alloc_objects 87.51 M → 60.39 M (`cf1392da`) | RUNTIME iterators; TRANSLATOR: enhanced-for over a runtime collection becomes an index loop; anonymous classes capture into one struct | LAMBDAS |
| 10 | Interned string ids (`eb45acc0`, `e51f666b`, `d063a329`, `d29c3f47`, `07deaeb6`) | parse benchmark 13.35 → 12.31 s (`eb45acc0`); typed-full 1.05× for `d29c3f47`+`07deaeb6` (`benchmarks.md` section `f60f193b`) | RUNTIME `RhinoStringPool` returning ids; node strings stored as ids in the store. Id-keyed maps inside passes were hand edits | MEMORY, CHECKS |
| 11 | Parallel input parsing as upstream does it, made to scale (`bd01d6e6`, `3fe3431a`, `30624a61`, `1e84e983`) | 52,620 files: 80.8 s sequential vs 20.9 s at 4 threads (`prebuild-ast-scaling.md:55`); 100,855 files 105 → 42.6 s with per-worker id ranges (`:8-25`) | ALREADY-GENERAL translation of `PrebuildAst.java` + RUNTIME goroutine `ExecutorService` and concurrent-safe node allocator | PARSE-PAR |
| 12 | Protobuf: typed generated messages, zero-copy decode, streaming shard decode, direct wire encoding (`0b82dcb8`, `08c04c79`, `59137479`, `c40666d8`, `2b9a4910`) | googleapis 1.02×, CHECKS 1.9 % faster (`benchmarks.md` section `2b9a4910`); −6.42 M allocs (`59137479`) | Generator emitting typed structs per message (as `ProtoGoGen.java` does) + small wire RUNTIME | PROTOBUF, CHECKS |
| 13 | Traversal token masks and leaf bypass (`68cad1f2`, `f3eb8b4c`, `2f001f07`, `ba39c769`, `028647ea`; ladder #5) | −6.60 s = −5.1 % googleapis, CHECKS −3.41 s, 0 bugs (`post_flip_optimizations.md:35`, `:132`) | POSTFLIP-TOOL, inferred: derive the mask by static analysis of each `visit` body (token `switch` with no default, or leading `if (!n.isX()) return`). jsgo declared masks by hand, so the tool form is unproven; if it needs per-pass judgment it is out of bounds | CHECKS |
| 14 | Closed hierarchies without virtual dispatch (`efb67a8a`, `cd0eaedb`, `67b1a658`, `a8eaf110`, `2b9a4910`, `d1c35b97`, `54998951`) | JSTypeBase 64 → 32 B: typed-full peak live 505 → 465 MB (`cd0eaedb`); parse tree ~122 → 90 MB (`efb67a8a`); time: googleapis 1.01× (`d1c35b97`) | TRANSLATOR: `self_` only for classes with overridden methods (`migration_plan.md:50`); type-tag switch for sealed hierarchies | CASTS, MEMORY |
| 15 | Small immutable classes as values (`621c2e66`, `59137479`, `22d6ff18`) | bundle peak live 344 → 285 MB (`621c2e66`) | JAVA-PREP marking value classes (records) + TRANSLATOR lowering identity-free records to value structs | MEMORY |
| 16 | Typed hashing and cached hash codes, regex cache (`8b621646`, `4f7a3728`, `870648db`, `721df604`, `063a6d46`, `b0a653ca`) | disambiguateProperties 2,586 → 393 ms (`8b621646`); suspiciousCode 2,699 → 1,212 ms (`4f7a3728`) | RUNTIME; TRANSLATOR emits generic `Equals[K]`/`Hash[K]` with unboxed fast paths | BOXING, CHECKS |
| 17 | Struct field ordering and narrowing (`610961c8`, `d94f9c96`, `73613824`, `ed139fbe`) | Var 40 B, TypedVar 64 B, Property 32 B (`ed139fbe`); typed variables ~13.8 → ~11.4 MB (`73613824`) | TRANSLATOR: order fields by alignment, booleans into one flag word | MEMORY |
| 18 | Unshared lists as slices; lookup-only collections as Go maps (`1ad51cb5`, `5cd39f45`, `63a53705`, `71c154ba`, `10aad291`, `22d6ff18`) | ADVANCED builds 2–3 % faster (`1ad51cb5`); others neutral in time, typed-full RSS −5 % (`benchmarks.md` section `a95c4a93`) | TRANSLATOR with per-declaration usage analysis | BOXING |
| 19 | Retention in the runtime (`9722485a`, `0ccf96e7`, `7330288b`, `8ad9a8a0`, `1881294c`) | bundle 10 MB text + 3.7 MB line offsets no longer pinned (`9722485a`); scannable heap at peak typed-full 183 → 164 MB (`8ad9a8a0`) | RUNTIME (source reader, resources, node tables) | MEMORY |
| 20 | Dead-node reclamation: in-place compaction, pruning dead side-table entries (`1762af68`, `583f730a`, `788cf2cf`) | Without compaction typed-full peak RSS ~410 → ~520 MB (`round4-heap-peaks.md` cut 1). On googleapis moving compaction is +0.09 s at 4 threads, +4.59 s at 1 (`post_flip_optimizations.md:149`); the plan keeps only the 13-line `PruneDeadAux` | RUNTIME plus generated forwarders for every `Node` holder; low priority, 3 dangling-node bug commits | MEMORY |
| 21 | Multi-member gzip writer for state files (ladder #6; introducing commit not identified) | −0.81 s alone; −5.73 s only together with parallel serde (`post_flip_optimizations.md:86`) | RUNTIME output stream. Borderline: it compresses on several goroutines | CHECKS (serializeTypedAst) |
| 22 | Typed, reflection-free `HamtPMap`, flat `PropertyMap` (ladder #18; `b0a653ca`, `62e7aa7e`, `108d2bc0`) | −0.40 s on `CHECKS:inferTypes` (`post_flip_optimizations.md:145`) | TRANSLATOR generic-key rules (HamtPMap is Closure source, not a runtime collection) | CHECKS |

Negative results worth adopting as decisions:
- Slabs, arenas and pools raised peak heap ("Slabs and pools were the main source of excess heap", `round1-results.md` Lessons 1). Removing them cut peak live heap 34–62 % for 9–19 % more wall time; "the cost is allocation itself, not GC". Per-pass slabs later recovered the object count (`8031b62e`: typed-full 30.2 M → 10.6 M) but need hand lifetime reasoning.
- Replacing `LinkedHashMap` by a new `OrderedMap` across 148 files measured googleapis 0.97× (`a822a48e`).
- Removing `unsafe` tricks cost about 4 % then 3 % on googleapis (`e1a9dd5f`, `0ed5c4cc`, `25688342`).
- GC wall-time cost on the 1:1 port was 3–7 % although GC was 22–29 % of CPU (`performance-roadmap.md:107-112`); allocation was 12–15 % of main-thread CPU (`:116-117`).

## (vi) Gains not obtainable under jsgo-mech's rules

| Item | jsgo commits | Share per jsgo's documents |
|---|---|---|
| Parallelism added to passes (stage-1 checks, TypeCheck, mid-end per-function passes, peephole waves, InlineFunctions sharding, TypedAst/state serde, codegen) | `30771cf2`, `81aa894d`, `3c4af5c1`, `6b9683cf`, `de846a4e`, `7b483fef`, `b842912d`, `75bca77c`, `d6afd440`, `e9c069c2` and others (23 rows) | −29.18 s of the 46.51 s gap = 63 % (`post_flip_optimizations.md:130-146`); −22.50 s for #3+#4+#7+#9 with the shared harness (`:88`) |
| Pass-specific algorithms, caches, indexes | `6e08e8e3`, `7c00cd6e`, `7e9a5261`, `618d303a`, `d3453b0e`, `a6c7ea5d`, `3ac34a0e`, `084e8292` | −6.67 s = 14 % of the gap |
| About 90 further algorithmic rows (rounds 4–22, 2A–2V: hand-wired arenas, CFA position table, lazy scopes, zero-clone inlining) | e.g. `bb521c0f`, `abcf69a0`, `f1c0cd2e`, `e5d0987a`, `88070d50`, `2f2e42f1`, `467bd3c0` | Inside L0; never separated from the allowed items. Round-level figures only (e.g. round 6 "−22.1% alloc_space on bundle-mega", `performance-roadmap.md:507`) |
| PGO | `117ca472` and 6 profile refreshes | −0.6 % to −1.8 % googleapis (`post_flip_optimizations.md:31`) |
| GC settings | none adopted | at most the 3–7 % GC tax (`performance-roadmap.md:113-114`) |
| Parser rewrite: UTF-8 scanner, parser slabs, positions as offsets | `99ad2308`, `131eca4b`, `8c546ee4`, `fd5954e5` | #14: −1.68 s = 3.6 %, of which the offset-checkpoint part is runtime |
| Ryū dtoa replacing the transpiled `DToA.java` | `11b8c8d4` | −0.04 % |

jsgo's documents put roughly 79 % of the L0 → HEAD gain out of bounds. They give no figure for how much of L0's own 1.62× lead over Java is out of bounds: the only isolated pre-flip number is the node store's.
