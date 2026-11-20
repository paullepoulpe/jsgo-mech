# jsgo-mech: where the migration plan stands (2026-11-20)

This page is the index of the plan across branches. The plan is: gated Java prep steps, then
the translator (tools/j2go) and its handwritten runtime, then the flip (generated Go frozen),
then recorded post-flip steps. Every gain must come from one of those; no hand-written
algorithmic change, no PGO, no collector tuning; output byte-identical.

## The pieces and where they are

| piece | branch | state |
|---|---|---|
| Java prep steps 1-20, translator with the rules of 2026-11-19 | main | gate 0 (30,393 Java tests, 4 projects identical to stock) and the Go gate pass |
| flip of main ("the fourth flip"): overlay with step list, driver, tool fixes | main-r11-flip4-inbounds | tools/postflip/reflip/REFLIP4.md is its record |
| the replayed tree, in bounds | inb-toolgaps-tree (b59f6ea1) | the whole list through step 556; runtime tests ok; checks, bundle, downlevel, advanced MATCH |
| the same flip with every step of the older lead line | main-r11-flip4 / reflip4-replayed | 298 steps; includes out-of-bounds manual patches; reference only |
| the older lead line | reflip-trial-r18 | flipped from an older translator; reference only |

Post-flip tools and their records live on the flip branches, not on main: main is the
pre-flip state by construction.

## What "in bounds" means here

An audit on 2026-11-20 found that the lead post-flip line carried 40 manual patches, most of
them hand-written changes of a data structure or algorithm in pass code (caches, a free list,
hand-made tables, restructured graph nodes and work queues). The in-bounds step list leaves
out the 24 of that kind and the runtime that existed only for them. Of the 8 manual patches
that remained, 5 are tool steps since 2026-11-20 (tool strvar); 3 remain (416, 419, 422: an
inlined computeIfAbsent and typed reads that rest on facts about one class each). Numbers reported before this date for post-flip trees include the out-of-bounds
patches and are not results of the plan.

## Speed against pre-flip Java (idle host, same output, 2026-11-20)

Full googleapis-bench (44,577 files, 7,526 chunks; one run each, same session):

| tree | total | CHECKS | OPTIMIZATIONS | FINALIZATIONS | peak memory |
|---|---|---|---|---|---|
| pre-flip Java | 454.1 s | 213.9 | 199.6 | 40.7 | 14.3 GB |
| the plan, in bounds (through step 556) | 585.5 s | 309.3 | 226.3 | 49.9 | 17.5 GB |

Java/Go 0.78. This is the standing.

The 18% subset (7,886 files; two alternating rounds), used to compare one step with the next:

| tree | total | CHECKS | OPTIMIZATIONS | FINALIZATIONS |
|---|---|---|---|---|
| pre-flip Java | 94.1 / 95.5 s | 40.5 / 41.1 | 41.9 / 42.9 | 11.7 / 11.6 |
| in bounds after step 553 | 103.9 / 104.4 s | 52.8 / 52.7 | 41.4 / 41.5 | 9.7 / 10.1 |
| in bounds through step 556 | 100.7 / 101.0 s | 51.2 / 51.5 | 40.1 / 39.9 | 9.4 / 9.6 |

Java/Go 0.94 on the subset. The subset flatters Go: Go's times grow about in proportion to the
input, Java's grow less (its two later stages are behind Go on the subset and ahead of it on
the full input), so the subset is not a measure of the standing.

Full-size CHECKS by where the CPU goes (profile of the in-bounds tree): translated code 35%,
collector 30% (19% on the subset), Go maps and hashing 13% (7%), allocation 9%, runtime
collections 6%, node store 5%. Nothing in translated code is above 2%. Both compilers run the
passes on one thread.

Steps of 2026-11-20 (CPU on the subset against the previous binary): 554a for-each over typed
views by position (-0.5%), 554b a typed list is its own erased view (-2%), 555 valplain, 20
immutable classes by value (neutral; 38M fewer heap objects in CHECKS), 556 typed maps and sets
are their own views (neutral; CHECKS peak memory -3%), and seven tool fixes that restore the
coverage of existing steps on this translator's output (Preconditions calls 1,047 to 6,
StringBuilder 78 to 5, requireNonNull 81 to 6, str_( 390 to 154).

## Since the standing above (2026-11-20, later)

- Post-flip steps 557 (eqhash re-run at the end of the list: 66 hash and equality functions of
  typed key conversions decided statically; CPU neutral) and 558 (vdispatch -rest: self_ typed in
  the 16 classes where it was `any`, 16 anonymous-interface conversions gone; CPU -0.6% on the
  subset). The in-bounds line ends at step 558 (main-r11-flip4-inbounds 02da66e7, tree
  inb-selfgen 6525013f), gated.
- The Java-ism accounting of the in-bounds tree is JAVAISMS4.md on main-r11-flip4-inbounds. Its
  largest item: Java's library generics are still erased in the Go (about 2,020 sites). The
  study and plan for emitting them typed from the translator is docs/libgen/LIBGEN-STUDY.md.
- Translator rule "function types" (stage 1 of that plan; switch J2GO_NOFNTYPES): 15 library
  functional interfaces are Go func types. On the translator's output: typed adapter literals
  862 to 0, adapter types 200 to 5, CHECKS heap objects -17.4% on the subset; 18% subset
  139.1 / 142.3 s to 135.5 / 135.9 s (pre-flip tree, idle host, same output). Gated with the
  rule on and off.
- In progress: stage 2, typed collection and iteration interfaces; then the fifth flip (the
  step list replayed on the new translator output), which gives the next standing.

## Open

- Speed: not met in bounds (0.78 on the full benchmark).
- A start-to-finish unattended replay of the final list (the list grew while its last replay
  ran, and step 556 was applied on top by hand).
- 3 manual patches (416, 419, 422).
- Coverage still below the older line: explicit iterators over immutable lists (125 against
  107) and Objects.equals of typed views (28 against 11) need new rules.
- Runtime: the scope table and the variable map that nothing used are gone; the helpers of
  step 212 (lihmComputeIfAbsent_ and two more) are dead and still there. The fast paths of
  steps 39, 140, 520, 521 and 535 are inside the runtime's own collections and node store.
- Structural: the translator emits one typed collection family and the post-flip steps
  another, merged by a step that has had to be rewritten at each flip. The translator should
  emit the family the plan ends with.
- Java-ism accounting against jsgo has not been redone item by item on the in-bounds tree;
  the census of the final tree is in REFLIP4.md on main-r11-flip4-inbounds.
