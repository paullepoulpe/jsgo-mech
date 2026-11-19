# jsgo's performance history, read for jsgo-mech (19 Nov 2026)

A read-only review of the full git history of jsgo (the hand-made Go port of the same compiler,
at 71ff08f4): every commit whose purpose or measured effect was speed or memory, classified by
kind and by how jsgo-mech could obtain the same gain under its rules. Made by a research agent
from jsgo's commits and documents; no builds or benchmarks were run, and every number is quoted
from jsgo with its source. Classification was done by five parallel readers under one rubric;
not every diff was re-read.

- `REPORT.md`: method and coverage, taxonomy, distributions, what jsgo's flip baseline contains
  that a mechanical translation does not, 22 ranked opportunities, what is out of bounds.
- `commits.tsv`: 374 performance commits, one row each (class, areas, measured effect and its
  source, feasibility for jsgo-mech and the mechanical form it would take).
- `not_perf_candidates.tsv`: 116 keyword candidates judged not to be performance commits.

Three findings that change how jsgo's figures should be read here:
1. jsgo's "L0 / post-flip baseline" (1.6x faster than Java with its 23 named optimizations off)
   is the hand-optimized tree with switches off, not a measured mechanical translation; its
   plan gives the claim that mechanical Go reaches it an estimated 10-18% allowance.
2. jsgo's first rule-based 1:1 port ran at 0.65-0.74x of warm Java, with about 37% of CPU in
   memory management: where jsgo-mech is now.
3. The one pre-flip item jsgo isolates is the pointer-free node store, and it is the largest
   single measured effect in its history (same tree with heap nodes: 221s and 11.1 GB against
   134s and 6.5 GB; CHECKS 136s against 73s).

About 79% of jsgo's measured gain after that baseline is out of bounds for jsgo-mech (63%
parallelism added to passes, 14% pass-specific algorithms, 1.6% PGO).
