# jsgo-mech

A trial of the mechanical Java-to-Go migration plan for Closure Compiler
(`migration_plan.md` in [jsgo](https://github.com/paullepoulpe/jsgo)): can a
throwaway transpiler plus a small handwritten Go runtime reproduce the Java
compiler byte for byte, and how large and how fast is the result?

Nothing here is copied from the existing jsgo port.

## Layout

- `java/`: Closure Compiler's Java sources and tests, imported from upstream
  tag `v20261104` (git tag `closure-v20261104` here). The Ant task, its test
  and one file upstream does not ship (`ConformanceReporterUtil.java`) are left
  out. Prep steps change this tree, one commit and one `prep-NN-<name>` tag
  each; `git tag -l 'prep-*'` lists them.
- `tools/gate0/`: the check every prep step must pass (`gate.sh`): build with
  plain javac, all JUnit suites, and byte-for-byte output comparison with the
  stock release on real projects. `perf.sh` times the stock release against
  the built tree on jsgo's benchmark tasks, using jsgo's own harness.
- `tools/prep/`: the script behind each scripted prep step.
- `tools/tag-preps.sh`: creates the `closure-*` and `prep-*` tags from the
  commit subjects.
- `pin.env`: the pinned Closure Compiler release.
- `tools/fetch.sh`: downloads that release's sources and jar into `.cache/`.
- `tools/apisurface/`: lists the library methods Closure's Java calls
  (`external-api.tsv`). That list is what the handwritten Go runtime has to
  cover.

## Status

**Phase 1 (commit `6d0fa52`): translate the unprepped tree.** `tools/j2go`
translated the release as it is (only prep 01 applied), with every difficulty
absorbed by the transpiler or by a handwritten Go runtime of 27.8k lines. The
result matched Java byte for byte and was 1.81x slower than cold Java over the
nine benchmark tasks.

**Phase 2: prep steps, then translate again.** All of the plan's thirteen prep
steps and three more are on `main`. Each passed `tools/gate0/gate.sh` (all
JUnit suites, four real projects compared byte for byte with the stock
release) before landing. `git log --grep '^prep('` lists them; the tools that
found or rewrote the sites, and the checkers that keep each rule true, are in
`tools/prep/`.

| step | what | measured (plan's figure) |
|---|---|---|
| 01, 01b | insertion-ordered collections | 38 files |
| 02, 02b | data races | 4 sites |
| 03 | no Mockito / DiffUtils in tests | 15 files; needed a 110-line diff routine |
| 04 | `Node` has no subclasses | 1 file |
| 05a-d | expected failures as `Outcome<T>` (short chains) | catch clauses 157 -> 106 (plan: ~4 left) |
| 05e-l | the same through long call chains, 8 exception families | 106 -> 91; 42 files |
| 06a | `@AutoValue` -> records | 22 types (32) |
| 06b | fields named like methods | 208 fields, 982 sites (~160) |
| 07 | same-arity overloads | 267 groups, 5,171 call sites (~40 groups) |
| 08 | no generic virtual methods | 4 families, 106 sites |
| 09, 09b | no side effects inside expressions | 192 sites + flag table |
| 10a-f | `String` never null (`Optional<String>`) | 412 files; findings 1,591 -> 158 |
| 11a | streams -> loops | 54 files |
| 11b, 11c | re2j regexes; plain format strings | 45 files; 6 calls (12) |
| 12a-d | own multimap/multiset/table/bimap | 5 classes, 519 code lines (~800 lines) |
| 13a, 13b | no Gson binding, no args4j | 19 sites; a 680-line flag parser |
| 14a-e | all I/O behind `jscomp.base.Io` (not in the plan) | 86 files; facade 600 lines |
| 15a-b | JSON behind `jscomp.base.Json` (not in the plan) | 26 files; catches 91 -> 56 |

Notes on individual steps:

- Step 10 is behaviour-preserving but incomplete: 158 places remain where a
  String can be null (most are `x.orElse(null)` at sites where presence could
  not be proven), so the Go side still needs its null-string sentinel.
  `tools/prep/10_find_null_strings` lists them.
- Steps 14 and 15 introduce *hand-implemented classes*: `Io.java` and
  `Json.java` are not translated (`tools/j2go/handwritten.txt`); the Go runtime
  implements them on Go's standard library (`rt_io.go`, `rt_json.go`). The
  Java versions wrap the JDK and Gson.
- Step 11b changes regex syntax for user-supplied patterns (RE2); step 15
  accepts only standard JSON; steps 05/14/15 turn several stack traces into
  one-line errors.

Java speed of the fully prepped tree against the stock release (nine tasks,
3 cold runs): 225.2s stock, 233.1s prepped (+3.5%), outputs identical; measured
on a busy machine with per-task swings of -18% to +29%, so not yet conclusive.

**The Go translation of the fully prepped tree** matches Java byte for byte on
the four gate projects and on all nine benchmark tasks. The handwritten
runtime is 15.5k lines (27.8k in phase 1): the args4j emulation, streams, the
Java-regex translator, the Guava multimap family, the java.io / java.nio /
zip class hierarchies and all of Gson are gone. `tools/prune/prune.sh` removes
runtime functions that no translated code references; `tools/prune/verify.sh`
runs the checks.

Speed, same harness, quiet machine (3 cold runs, medians):

| task | Java | Go | Go/Java |
|---|---:|---:|---:|
| typed-app | 11.5s | 13.8s | 1.20x |
| incremental-ijs-check | 7.3s | 8.9s | 1.22x |
| downlevel-simple | 8.8s | 10.8s | 1.23x |
| checks-only | 13.5s | 21.7s | 1.61x |
| generate-ijs | 5.8s | 9.9s | 1.72x |
| conformance-checks | 17.4s | 32.2s | 1.85x |
| shaka-blockly | 25.0s | 47.2s | 1.89x |
| bundle | 27.9s | 59.3s | 2.12x |
| typed-full | 47.0s | 104.1s | 2.21x |
| sum | 164.2s | 307.8s | 1.88x |

The prep steps made the result smaller and easier to review; they did not make
it faster (1.81x before, 1.88x after).

**Not done.** The transpiler still lowers try/catch to closures (56 catch
clauses remain in the Java), keeps the null-string sentinel and erases
generics. `rt_io.go` is 1.6k lines because `Io.java` still exposes JDK
behaviour; a step that simplifies the facade is in progress. The plan's
`NodeStore` is not built. Tests are not translated. Git tags could not be
pushed from the authoring session: run `tools/tag-preps.sh --push`.

The sections below describe phase 1 in detail.

## Phase 1 status (2026-11-13)

The whole compiler translates and runs. `tools/j2go` (4.5k lines of Java, on
javac's own front end) turns the 812 production Java files (280k lines) into
238k lines of Go in one package, `go/closure`; `go/cmd/mech` is the resulting
command-line compiler. The generated files (`go/closure/g_*.go`) are not
checked in: `bash tools/j2go/run.sh` regenerates them in about a minute.

**Correctness.** The Go binary's stdout, stderr, exit status and output files
are byte-identical to the Java release on every case tried:

- the four cases of `tools/gate0/e2e.py --go <binary>` (Closure Library
  type-check; three + lodash + react-dom SIMPLE with source map; Angular +
  rxjs + core-js to ES5; Shaka + Blockly ADVANCED with types and source map);
- all nine tasks of jsgo's benchmark harness (`tools/bench/bench.py` there).

The JUnit suites are not translated yet, so that is the whole evidence.

**Speed.** jsgo's harness, 3 cold runs each, medians, on a 4-vCPU / 15 GB
container (a slow machine: Java itself takes 2-3x longer here than in the
plan's tables):

| task                  | Java wall | Go wall | Go/Java | Java CPU | Go CPU | Java RSS | Go RSS |
|-----------------------|----------:|--------:|--------:|---------:|-------:|---------:|-------:|
| typed-app             |   12.7s   |  15.1s  |  1.19x  |   40.9s  |  21.7s |   577M   |  353M  |
| downlevel-simple      |    9.2s   |  11.0s  |  1.20x  |   29.5s  |  15.3s |   409M   |  244M  |
| incremental-ijs-check |    8.6s   |  10.9s  |  1.26x  |   27.7s  |  16.4s |   827M   |  718M  |
| generate-ijs          |    7.1s   |  10.8s  |  1.51x  |   22.1s  |  14.6s |   824M   |  603M  |
| checks-only           |   14.7s   |  23.9s  |  1.62x  |   44.9s  |  32.9s |  1233M   |  974M  |
| shaka-blockly         |   28.5s   |  50.4s  |  1.77x  |   78.1s  |  74.1s |   879M   |  667M  |
| conformance-checks    |   20.4s   |  38.3s  |  1.87x  |   59.2s  |  53.2s |  1305M   | 1353M  |
| typed-full            |   53.6s   | 111.3s  |  2.08x  |  129.2s  | 159.9s |  1403M   | 1507M  |
| bundle                |   28.5s   |  60.1s  |  2.11x  |   67.4s  |  83.7s |  1657M   |  665M  |
| sum                   |  183.3s   | 331.7s  |  1.81x  |          |        |          |        |

So the straightforward translation is 1.2-2.1x slower than cold Java in wall
time, at similar or lower CPU time and memory. It does not reproduce the plan's
"2.2-5.1x faster than cold Java". The profile is flat: no single pass or runtime
function dominates. The cost is spread over allocation and garbage collection
(objects are about twice the size of Java's; the collector does not compact),
boxing of strings and numbers into `any` (generics are erased), interface type
assertions, and hash-map lookups. googleapis was not run (no protoc here, and
it needs more memory than this machine has).

**Size of the handwritten part.** `go/closure/rt_*.go`, the runtime the
generated code calls (JDK, Guava, Gson, protobuf, args4j, regex, I/O): 27.8k
lines, plus 5.9k lines of its tests and 16.8k lines of generated protobuf
messages (`tools/protogen`). The plan budgets 7.1k lines, after thirteen Java
prep steps; only prep 01 has been done here, so the two figures are not for
the same Java tree, but the gap is the number to look at.

**Design decisions made along the way** (details in `tools/j2go/README.md`
and `tools/j2go/RUNTIME.md`):

- one Go package with Java's names; classes are structs with the superclass
  embedded, virtual calls go through a `self_` back-pointer;
- generics are erased to `any`, with conversions inserted by the transpiler;
- `String` is a Go string with UTF-16 index semantics; a null `String` is a
  reserved constant; exceptions are panics;
- static initialisers run eagerly in an order recorded from an instrumented
  run (`tools/j2go/init-order.sh`), not lazily as in Java;
- library calls keep their Java names and are implemented by hand in
  `rt_*.go`; args4j's reflection is replaced by tables the transpiler emits;
- threads run inline: the Go compiler is single-threaded apart from the
  garbage collector.

**Known gaps.**

- No translated tests; `g_zstubs.go` still holds panicking placeholders for
  library members no tested path has reached.
- Eager static initialisation, inline threads, and structural interface
  checks (`instanceof List` is true for anything with List's methods) are
  semantic differences from Java that the byte comparisons have not hit.
- Go does not fix the order of a plain variable read against a call in the
  same expression; the transpiler only guards the cases with an embedded
  assignment.
- Prep 01 leaves `HashBiMap`, `Collectors.toSet/toMap` and
  `ConcurrentHashMap` iteration untouched.
- Git tags could not be pushed from the session that wrote this; run
  `tools/tag-preps.sh --push`.
