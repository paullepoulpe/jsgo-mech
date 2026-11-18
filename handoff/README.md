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

## State at the move (all pushed)

- **main-r8-typed @ 4936ec55.** Slices 11 (Guava immutables, 466f7df5) and 12 (TreeSet/TreeMap sorted mode, 2b9b5e94) are gated; 12 lacks its typed-full timing. Slices 13 (type variables as keys, 1be86b45) and 14 (Splitter.splitToList, 4936ec55) are WIP and not gated: run `tools/j2go/run.sh` and the gate, and bisect with `J2GO_NOCOMPARABLE=1` / `J2GO_COLLS_NOMAKERS=1` if it fails. Raw census after slice 11: boxing 9,061, interface casts 1,977, class casts 2,615, erased immutables 196.
- **main-r8-upcast @ 3f2a8ac9.**
  - P1-P5, typed Object methods (records and string concatenation), and String for CharSequence are each gated.
  - The top commit merges main-r8-typed 466f7df5 and is WIP, not gated.
  - Three commit messages are shifted by one: P3 is "WIP P3 bare", P4 carries P3's message, and P5 carries P4's. The note commit b0520c58 has the mapping, and the correct texts are in `upcast/`. A force-push to reword them needs the user's permission.
  - P5 (alias) may be slower on typed-full: 94.20 against 87.87 CPU s, in 2 rounds; `J2GO_NOALIAS=1` turns it off. Re-measure it.
  - Raw census on the merged tree: boxing 4,430, interface casts 514, class casts 2,376, unboxing 516.
- **reflip-trial @ c6049668.** typed + node store, a trial flip, and the replay of all 233 go/-changing commits.
  - Per category: 178 as-is, 12 adapted, 5 no-op, 4 partly dropped, 34 dropped (most of them manual patches of code j2go now writes differently).
  - The replayed tree is gated: runtime tests ok, four MATCH.
  - Not measured yet (milestone timing and allocations).
  - Write-up: `tools/postflip/reflip/REFLIP.md`. Main blocker for the real re-flip: two typed-collection runtimes (j2go's, renamed J* for the trial, and the post-flip one).
