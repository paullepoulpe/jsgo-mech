# The fourth flip, and its in-bounds variant (2026-11-20)

What this is: the project's current pre-flip state (main at dfdb5197's parent: Java prep steps
through 20, the translator with the rules of 2026-11-19) flipped and replayed with the whole
step history of the lead post-flip line (reflip-trial-r18, steps through 540). It is the first
tree in which the plan exists as one piece: Java prep, translator, post-flip steps, replayed
unattended from the flip base.

Two step lists, two trees:

| | branch (overlay / tree) | steps | manual patches |
|---|---|---|---|
| all steps | main-r11-flip4 / reflip4-replayed (caf3434f) | 298 | 32 |
| in bounds | main-r11-flip4-inbounds / reflip4-inbounds (b690d508) | 273 | 8 |

Both: runtime tests ok; checks, bundle, downlevel, advanced MATCH; an unattended replay from
the base reproduces the gated tree. Drivers: reflip/replay4.sh, reflip/replay4i.sh. The driver
stops on a manual patch that does not apply (it used to drop it and go on).

## Why there is an in-bounds variant

The plan allows a gain from a gated Java prep step, a translator rule, a post-flip tool, a
handful of same-pattern manual edits, or a minimal handwritten runtime, and no hand-written
algorithmic change. An audit of the lead line's manual patches on 2026-11-20 found 40 active
ones (44 files, about 1,400 changed lines), most of them changes of a data structure or an
algorithm in pass code: caches (getAstAnalyzer, InlineVariables' memo cells), a free list of
SubtypeCheckers, a hand-made variable table for scopes, variable-keyed maps on a hand-made
map, graph nodes with their edge lists inline, a work queue that marks nodes, log messages
built only for a recording log, and so on. Those are out of bounds. The numbers reported for
the lead line before this date include them and are not results of the plan.

The in-bounds list comments out every manual patch of that kind (`#=4i ... out of bounds` in
steps.txt, 24 patches and the (2) part of 422) and the runtime patches that exist only for
them (042-rt, 462-rt, 530-rt). Runtime kept though its first user is gone, because later steps
build on the same files: rt_varmap.go (021-024), rt_scopevars.go and rt_strid.go (108, 429).
The scopeVars_ table and varMap_ are unused there and are to be deleted by a later step.

Manual patches left in bounds: 8, in 17 files, 325 changed lines, all typing or notation that
a tool could do (to become tool steps): 029, 030 (manual part), 035, 121, 416, 419, 422 (typed
reads only; its nodeOrFail_ and fast to_ parts are borderline and to be looked at), 428.

## What the replay found

- Tools that needed a fix for this translator's notation (toolfix.sh 11-17): visit, strids,
  enumin, tfnparams, optstr, sbuild, funcparams. They now see through the explicit upcasts of
  rule "implicit conversions", the tostrR_/equalsR_ forms of "typed Object calls" and the
  names of "covariant overrides".
- Step 402 (one typed collection family) was rewritten for this translator, which also emits
  typed immutables, builders and sorted collections (reflip/402-fourth-flip.txt). Structural
  note for the plan: the translator emits one family and the post-flip steps another, and a
  step merges them. The translator should emit the family the plan ends with.
- Post-flip steps that are now no-ops or nearly, because main does their work before the
  flip: covret (460, 474), fbound (464), nnassert (476); mostly: gmaps (453), castfold -nilup
  (456), tviews (479), trips (512: 46 of 88).
- Not applied: optstr (490). Its proof fails on two fields (Compiler.idGeneratorMap,
  PrototypeObjectType.className): a statement before the field's assignment reaches a read of
  it through the typed collection runtime (tchT_ -> objequals_; newTSortedMap_ ->
  Comparator.compare). The proof was not weakened. 489, 491, 492 are applied and inert.
- A pre-flip runtime test named the erased ImmutableList type, which a post-flip step renames
  (fixed on main, 60a9e130).
- Not re-examined: manual steps dropped by earlier flips with the generic reason "no longer
  applies" (018, 027, 040, 134/193/195/251, 154, 179, 303); the pre-402 tool counts that fell
  against the third flip (eqhash, tostr, anyconv) were not checked site by site.

## Time, 18% of googleapis-bench (7,886 files), alternating rounds, same output hash

runs14/t11_gs3500.runs (lead line against the fourth flip) and runs14/t12_gs3500.runs:

| | total | CHECKS | OPTIMIZATIONS | FINALIZATIONS |
|---|---|---|---|---|
| pre-flip Java (t12) | 94.5 / 94.2 s | 40.5 / 39.7 | 42.4 / 43.0 | 11.5 / 11.5 |
| fourth flip, all steps (t12) | 94.8 / 94.2 s | 48.7 / 48.6 | 37.4 / 36.9 | 8.6 / 8.8 |
| fourth flip, in bounds (t12) | 106.6 / 106.5 s | 53.7 / 53.3 | 42.6 / 42.9 | 10.2 / 10.2 |
| lead line at 540 (t11) | 89.4 / 89.3 s | 46.1 / 45.5 | 35.2 / 35.5 | 8.1 / 8.3 |
| fourth flip, all steps (t11) | 94.4 / 94.5 s | 48.4 / 48.8 | 37.1 / 37.1 | 8.8 / 8.6 |
| pre-flip Java (t11) | 97.0 / 96.1 s | 41.3 / 41.0 | 43.9 / 43.0 | 11.8 / 12.1 |

- In bounds, Java/Go is 0.885 at this size: the plan's result today. CHECKS falls further
  behind on larger inputs, so the full benchmark will be lower.
- The out-of-bounds manual patches are worth 12 s (11%): about 5 s in CHECKS, 5.5 s in
  OPTIMIZATIONS, 1.5 s in FINALIZATIONS.
- The fourth flip with all steps is 5 s (5-6%) slower than the lead line: not attributed yet
  (optstr not applied; several tools do less here).

## Where the 12 s are (CPU profile difference, in bounds minus all steps)

Largest items: lookups of a scope's variables by name through the erased map (htable_.findStr,
utf16Hash_: about 1.3 s in CHECKS and 3.3 s in OPTIMIZATIONS, where the hand-made scope table
and getVarById_ used to be); variable-keyed typed maps hashing through Object.hashCode
(TMap[*Var,..].findH, identityHash_: 1-2 s in OPTIMIZATIONS, where varMap_ used to be). Both
have an allowed form: the scope's map as a typed map by the collection tools, and identity
hashing for key classes without equals/hashCode by a tool. In progress as steps 550, 551.
