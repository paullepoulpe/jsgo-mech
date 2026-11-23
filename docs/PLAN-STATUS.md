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

## The fifth flip (2026-11-21)

The in-bounds step list replayed on main's translator output with rule "function types"
(overlay main-r12-flip5 a8761c89, record tools/postflip/reflip/REFLIP5.md; gated tree flip5-tree
aa141ba3). Two unattended replays from the flip base give the same tree (270 commits); runtime
tests ok; checks, bundle, downlevel, advanced MATCH. 1 manual patch is left (422; 416 and 419
lost their subject), 7 steps dropped and 20 that find nothing or less, 5 tool fixes (28-32).

Correction: the fourth flip's base froze main at prep 18, not prep 20 as recorded; the fifth
flip is the first to carry prep 19 and 20 through the post-flip steps.

Idle host, same output as Java, 2026-11-21:

| tree | full benchmark | CHECKS | OPTIMIZATIONS | FINALIZATIONS | 18% subset |
|---|---|---|---|---|---|
| pre-flip Java | 452.1 s | 214.7 | 198.5 | 38.9 | 90.5 / 91.0 s |
| fourth flip, through step 558 | (585.5 s through 556, the day before) | | | | 97.7 / 97.9 s |
| fifth flip | 572.1 s | 304.1 | 218.7 | 49.3 | 97.5 / 97.8 s |

Java/Go 0.79 on the full benchmark (0.93 on the subset). The fifth flip is level with the
fourth in time: the rule's -4% on the translator's output had already been recovered post-flip
by the lambda tools. Its gain is in what the translator emits and in the census: adapter
literals 587 to 39, boxing 1,182 to 1,017, class casts 769 to 728, `any` slots 526 to 479.

Not recovered in the fifth flip (later tools do not read the new lambda form): 20 maps that
were native Go maps, typed forEach / merge / compute, 11 typed sorts, the data-flow work queue's
direct comparator. Two census rows are worse (instanceof helpers 71 to 78, anonymous interface
assertions 0 to 6: instanceof of the now-generic Simple* classes).

Second part of the fifth flip (overlay a634273a, tree flip5-tree-d 2abbf4e4, 272 commits,
unattended, gated): one shared reader of the old and new lambda forms (tool fix 33) brings back
the typed rewrites (native Go maps through computeIfAbsent 0 to 20, typed forEach 8, merge and
compute 3 and 2, typed sorts 12 to 24), a typed priority queue for the two worklists (step 559),
and names for the instanceof marker interfaces (step 560). No census row is worse than the
fourth flip's except typed constructors (450 against 448). In time it is level with the first
part (subset, three alternating rounds: 98.9 / 98.6 / 98.8 s against 99.8 / 98.1 / 98.1 s).

Where the full benchmark's CHECKS time is (profile of the in-bounds tree, 291 s): the compiler's
own thread is busy for about 268 s of it; the collector's background work is 61 s of CPU, of
which 44 s runs on otherwise idle cores. So the wall time is the main thread's: about 25 s of
allocation, 27 s of Go map lookups (maps keyed by node handle 11 s, a set of CompilerInput in
dependency sorting 6 s, the node type handle table 4 s), and the rest translated code with no
function above 2%. Java's whole stage takes 214 s.

Translator, not yet flipped (branches main-libgen-b2, -b3, -b4; each gated on, all off, and the
newer switches off one by one): rule "typed library interfaces", kinds iter, coll, list, set,
map/entry. Erased collection declarations 816 to 242 (133 class-typed ones are the next piece);
on the translator's output cast_[ 1,029 to 723, erased views 1,198 to 572. CPU on the subset,
each stage against the one before: iter +1%, coll and list -2%, set and map neutral. The
runtime has not shrunk (20,241 lines against 19,832).

## The sixth flip (2026-11-23)

The step list replayed on main's translator with both typed-generics rules (function types;
typed library interfaces: kinds iter, coll, list, set, map/entry, cls, meth). Overlay
main-r13-flip6 670d0513, record tools/postflip/reflip/REFLIP6.md, gated tree flip6-tree 2c5e324f.
An unattended replay from the base (271 commits) gives the working replay's tree; runtime tests
ok; checks, bundle, downlevel, advanced MATCH. Tool fixes 37-44, two new runtime patches with
tests, one manual patch (422).

Census against the fifth flip: erased collection constructors 112 to 40, Guava immutables 149 to
43, for-each via iterator objects 105 to 34, explicit Iterator use 125 to 73, class casts 699 to
553, unboxing 224 to 131, boxing 999 to 938, equals/hashCode 76 to 72. Worse, explained in
REFLIP6.md by cause and count: interface casts 164 to 183 and `any` slots 425 to 448 (the unbox
functions and `func(v any)` of the new conversions of typed collections as elements), typed sorts
24 to 20 (four sorts of typed map entries, where the round-trip proof does not hold).

Idle host, same output, 2026-11-23. Full benchmark: sixth flip 576.5 s and fifth flip 575.1 s
back to back (CHECKS 294.3 / 297.2, OPTIMIZATIONS 231.1 / 227.2, FINALIZATIONS 51.0 / 50.7);
pre-flip Java 445.4 s (CHECKS 201.4, OPTIMIZATIONS 203.1, FINALIZATIONS 40.9). A first sixth-flip
run read 595.6 s; the back-to-back pair shows it was the host. Java/Go 0.77 today (Java ran 7 s
faster than on 2026-11-21, so the ratio moved with Java, not with Go). Subset: level with the
fifth flip (119.1-122.3 s CPU both).

So the typed-generics work (both translator rules and their flip) removed most of the
erased-collection Java-isms and did not move the time. Speed is not met.

Step 561 (post-flip tool htabself, overlay main-r13-flip6 3010337d, tree flip6-tree-h 88e20f2d):
JSType, its 27 subclasses and Color keep their node-type handle in a field, so the handle
lookup reads the field and does not use the runtime's identity map. Same output; gated
(checks, bundle, downlevel, advanced MATCH). Profile on the subset: Node.setJSType 0.75 s to
0.28 s. Subset, three alternating rounds: CPU 120.4 s to 118.9 s on average (about -1%), wall
within noise. An unattended replay from the flip base (272 commits, step 561 included) gives
the gated tree exactly (go/ tree 04cbc7cf).

## Open

- Speed: not met in bounds (0.77 on the full benchmark, 2026-11-23). No in-bounds step so far has moved the
  full benchmark by more than about 2%; CHECKS (0.71) is where Go is behind, and its cost is
  spread thin with the collector at 30%.
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
