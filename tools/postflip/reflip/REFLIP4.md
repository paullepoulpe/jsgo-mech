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

## Steps on the in-bounds line after the replay

550, identity keys: NOT MADE. The premise (the variable classes have no equals/hashCode) was
wrong: AbstractVar extends ScopedName, whose final equals/hashCode compare name and scope
root. A census of the 37 equals-compared key conversions found no class an identity rule
would convert.

551, colls: a field that holds the empty immutable map or a map made in its class is a typed
map (Scope.vars, TypedScope.vars; 21 declarations in 5 classes; tool 855a5071, step a2c2775c on
branch inb-keys). Gated (four MATCH). RECORDED, NOT ACTIVE: CPU did not drop in any stage (one
run: 128.66 -> 130.53 s; the lookup cost moved from the erased table to the typed map's
index).

552, hashmemo (tool a994e38c, step 27632e6f on branch inb-hashmemo): a class whose hashCode is
provably constant (not overridden or dispatched; a straight-line body over fields assigned
only in constructors; only stable hashes combined; `this` does not escape a constructor before
the hashed fields are set) keeps the value once computed, 0 meaning not computed, as
java.lang.String does. Java caches a String's hash in the String; the translated code
recomputed it on every ScopedName.hashCode and InputId.hashCode. 22 classes written of 123
that declare hashCode (13 more proved but hash no string; 88 fail an obligation, listed by the
tool's report). Cost: 4 bytes per object, 16 allocated bytes for a Var (64 -> 80). Gate: four
MATCH. Time, 2 alternating rounds, 18% subset (runs14/t13_gs3500.runs): 107.58 / 107.58 s
before, 105.04 / 105.01 s after (-2.4%); CPU 129.6 / 128.7 -> 126.2 / 126.3 s; OPTIMIZATIONS
43.1 / 43.2 -> 41.6 / 41.3 s. Peak RSS within run-to-run spread. ACTIVE. Open: its replay
through the driver has not been run; the tool has no test of its own.

In bounds after 552: 105.0 s against pre-flip Java's 94.3 s (t12): Java/Go about 0.90.

553, optstr (the lead line's 490, which did not prove on this flip; tool 48db3634, step
684afd83 on branch inb-optstr): the tool's call graph had an edge from a function to every
function literal it makes, so building an empty typed collection "reached" the hash and
compare closures it only creates. The analysis now reaches a function value only where it can
be run (a call through a func-typed expression, matched by signature at every generic
instantiation; a func-typed field's stores when they are all known; the typed-lambda adapter
and tostr idioms of this translator). It adds edges the old one missed (TMap.findH -> a
conversion's hash) as well as removing false ones. No field or class is named. Counts as on
the lead line: 177 fields proved, 1,378 type spellings, 3,429 calls (lead 3,430). Gate: four
MATCH. Time, 2 alternating rounds with Java in the run (runs14/t14_gs3500.runs): before
104.64 / 105.80 s, after 105.95 / 105.23 s; CPU 125.0 / 126.3 -> 127.1 / 127.6 s. NEUTRAL in
time: active as the removal of a Java-ism (Optional<String> objects), not as a speed step.
Open for 552 and 553: replay through the driver not run; no tool tests; the closed-world
premises of 553 (no func value run through reflection or unsafe) are argued, not checked.

Standing after 553, 18% subset (t14): pre-flip Java 95.5 / 95.2 s (CHECKS 40.8 / 40.6, OPT
42.6 / 42.6, FIN 12.1 / 12.0); in bounds 105.9 / 105.2 s (CHECKS 53.8 / 53.5, OPT 42.1 / 41.8,
FIN 10.1 / 10.0). Java/Go 0.90. The whole deficit is CHECKS (0.76); the two later stages are
level or ahead.

CHECKS CPU of the in-bounds tree by category (profile of 553's binary, 65.8 s): translated
compiler code 31.6 s (48%), collector background marking and sweeping 12.5 s (19%, off the
main thread), runtime collections 5.5 s, allocation 5.0 s, Go maps and hashing 4.6 s, node
store accessors 2.0 s, other runtime 3.3 s. The main thread's time is about the stage's wall
time. The profile is flat: no function of the translated code is above 2%. Both compilers run
the passes on one thread (Java's numParallelThreads defaults to 1; its extra CPU is the JIT
and a parallel collector).
