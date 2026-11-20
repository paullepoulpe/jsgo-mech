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
| the replayed tree, in bounds | reflip4-inbounds, then inb-hashmemo, inb-optstr | 273 replayed steps + 552, 553; runtime tests ok; checks, bundle, downlevel, advanced MATCH |
| the same flip with every step of the older lead line | main-r11-flip4 / reflip4-replayed | 298 steps; includes out-of-bounds manual patches; reference only |
| the older lead line | reflip-trial-r18 | flipped from an older translator; reference only |

Post-flip tools and their records live on the flip branches, not on main: main is the
pre-flip state by construction.

## What "in bounds" means here

An audit on 2026-11-20 found that the lead post-flip line carried 40 manual patches, most of
them hand-written changes of a data structure or algorithm in pass code (caches, a free list,
hand-made tables, restructured graph nodes and work queues). The in-bounds step list leaves
out the 24 of that kind and the runtime that existed only for them. 8 manual patches remain
(17 files, 325 changed lines), all typing or notation; turning them into tool steps is in
progress. Numbers reported before this date for post-flip trees include the out-of-bounds
patches and are not results of the plan.

## Speed against pre-flip Java (googleapis-bench subset, 7,886 of 44,577 files, same output)

| tree | total | CHECKS | OPTIMIZATIONS | FINALIZATIONS |
|---|---|---|---|---|
| pre-flip Java | 95.5 / 95.2 s | 40.8 / 40.6 | 42.6 / 42.6 | 12.1 / 12.0 |
| the plan, in bounds (after step 553) | 105.9 / 105.2 s | 53.8 / 53.5 | 42.1 / 41.8 | 10.1 / 10.0 |

Java/Go 0.90 on this subset. Not measured in bounds on larger inputs; on earlier trees CHECKS
fell further behind as the input grew, so the full benchmark is expected to be lower. With the
out-of-bounds patches the same flip is level with Java on this subset (94.2-94.8 s).

The deficit is the CHECKS stage. Its profile is flat (no function of translated code above
2%); both compilers run the passes on one thread.

## Open

- Speed: not met in bounds.
- The 8 manual patches as tools; unattended replay including steps 552 and 553.
- Runtime audit: the scope table (unused in bounds, to delete), the front table of node
  property lists and the per-type lookup memos (stopgaps), node keys hashed as handles (to
  read).
- Structural: the translator emits one typed collection family and the post-flip steps
  another, merged by a step that has had to be rewritten at each flip. The translator should
  emit the family the plan ends with.
- Java-ism accounting against jsgo (docs/jsgo-history, and JAVAISMS.md on the older post-flip
  branches) has not been redone on the in-bounds tree.
- A full-size benchmark run has not been made for any tree since 2026-11-19 (memory on the
  measurement host).
