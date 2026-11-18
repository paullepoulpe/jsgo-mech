# Handoff, 2026-11-18: the session moves from the cloud container to the VPS

What lived only in the cloud container's scratch directory, kept here so that the work can go on
elsewhere. Nothing here is part of the plan's provenance chain; the plan is on the branches.

## Branches (all pushed)

| branch | what | state |
|---|---|---|
| postflip5 | post-flip steps through round 8; `tools/postflip/replay-all.sh` rebuilds its go/ from the flip | gated; typed-full Java/Go 1.053, bundle 1.116 on an idle host (POSTFLIP.md) |
| main | pre-flip tree and translator; PGO removed | gated |
| main-r8-nodestore | GC-free node store: no Go pointer per node (60 bytes, 0 pointer words) | gated; not merged into main (awaiting the user) |
| main-r8-typed | j2go keeps Java's types: generic classes/interfaces/hierarchies, typed lambdas, Optional[T], typed collections (slices 1-12+) | gated up to the last non-WIP commit |
| main-r8-upcast | the pressure test's translator rules P1-P7 (leaf-class upcasts, generic checkNotNull, literals for Object, Preconditions messages unboxed, aliases, ...), merged with main-r8-typed | gated up to the last non-WIP commit |
| reflip-trial | main-r8-typed + main-r8-nodestore merged, a trial flip, and the post-flip replay on it (tool fixes, replay table) | see its REFLIP-TRIAL.md / last commits |
| postflip5-r8-* | the round-8 agent branches (merged into postflip5) | done |

## Here

- `scripts/`: the shared agent brief `r8_common.md` (house rules, gate, measurement rule), the merge
  and gate helpers (`mstep2.sh`, `gate.sh`, `theirs.py`, `enumfix.py`), the translator agents'
  helpers (`tg_*.sh`), the census copies.
- `proto/`: the benchmark harness `fb.py` (interleaved forward/backward rounds; per-run lines on
  stderr: task label wall cpu round load), `m.py`, `j2mem.py`, and the other analysis scripts.
  Pre-flip Java is run with `.cache/build/candidate.sh` (built by tools/gate0/build.sh).
- `runs/`: raw timing runs (`*.runs`) and census snapshots behind the numbers in POSTFLIP.md,
  e.g. `q8_tf.runs` / `q8_b.runs` (round 8 against pre-flip Java on an idle host).
- `ptest/`: the pressure test of what the translator still erases: `report.md`, the prototype
  patches (P1-P5), the site attribution (`sites.tsv`, `decls.tsv`, `classify.py`).
- `critique/`: the scripts behind the comparison with the hand port (jsgo) summarized in
  POSTFLIP.md, "Planned: what is left after the re-flip, checked against the hand port".

## Next

1. On the VPS: toolchains (JDK 21, Go >= 1.24, Python 3), `tools/fetch.sh`, the gate0 build of
   pre-flip Java, and googleapis-bench (jsgo's harness) with CPU and heap profiles.
2. Finish main-r8-typed (Guava immutables, tree collections, the rules in POSTFLIP.md's
   "Planned" section), merge main-r8-upcast, then the real re-flip with the node store and
   `replay-all.sh`.
3. The post-flip round after the re-flip (POSTFLIP.md).
