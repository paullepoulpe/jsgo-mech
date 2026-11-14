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
runtime is 14.2k lines (27.8k in phase 1): the args4j emulation, streams, the
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

**NodeStore (handle classes).** `Node` is now a 32-bit handle into a store
instead of a heap pointer, as a general translator feature: a class listed in
`tools/j2go/handles.txt` becomes `type C uint32`, its fields live in two
generated records per object (28 pointer-free bytes plus the handle's own
value, and 56 bytes of pointer-carrying fields for `Node`; 112 bytes as a heap
struct before) kept in chunks that are never moved or freed
(`tools/j2go/README.md`, "Handle classes"; the handwritten part is the 82 lines
of `go/closure/rt_handles.go`). Outputs are unchanged: self-test, four gate
projects and nine benchmark tasks all match Java byte for byte.

What it buys, measured on a busy 4-vCPU machine (3 runs, medians; the two
columns were measured 40 minutes apart, so single tasks swing by 10%):

| task | Java wall | Go before: wall / CPU / RSS | Go after: wall / CPU / RSS |
|---|---:|---:|---:|
| typed-app | 11.8s | 15.3s / 21.6s / 327M | 14.7s / 18.9s / 361M |
| incremental-ijs-check | 9.9s | 10.3s / 14.4s / 721M | 11.6s / 15.8s / 671M |
| downlevel-simple | 8.8s | 10.1s / 14.0s / 221M | 11.2s / 13.7s / 227M |
| checks-only | 14.6s | 22.2s / 30.5s / 982M | 24.6s / 31.0s / 931M |
| generate-ijs | 7.0s | 10.5s / 13.8s / 610M | 11.5s / 13.8s / 629M |
| conformance-checks | 22.8s | 38.2s / 50.9s / 1403M | 36.3s / 45.3s / 1294M |
| shaka-blockly | 34.5s | 50.2s / 69.4s / 655M | 48.5s / 63.2s / 730M |
| bundle | 24.3s | 55.1s / 78.3s / 639M | 60.3s / 72.5s / 694M |
| typed-full | 54.1s | 115.2s / 159.7s / 1532M | 111.4s / 142.0s / 1547M |
| sum | 187.8s | 327.2s / 452.6s | 330.1s / 416.1s |

CPU time falls by 8% (9-14% in interleaved runs of the four gate projects);
wall time does not move beyond the noise (+1% here, -1% to -5% interleaved).
The saving is collector work: on the ADVANCED gate project the mark workers go
from 23.5s to 15.6s of 76s CPU, but they run on other cores, so the compile
does not finish sooner. The code that walks the tree is no faster: a field
read is two loads instead of one, and a node's pointer-carrying fields
(string payload, type, properties) are now a second cache miss
(`Node.getString` 2.6s -> 4.4s). Boxing a handle into `any` cost 2.9% of CPU
until boxes were made allocation-free. Nodes are never freed: on that project
1.70M nodes are created and 35% are detached at exit; peak RSS changes by -8%
to +11% depending on the task. The large gains the plan expects from a
NodeStore do not appear with this translator; allocation of other objects
(16%), the remaining marking (25%), boxed strings and hash lookups dominate.

**Cheap performance items (branch `wip/perf1`).** A measurement study found that the
translated compiler allocates 500 times more objects than a hand-written port, a third of them
Java Strings boxed into `any` as lookup keys. Six cheap remedies are now in the translator and
the runtime (`tools/j2go/README.md`, "Performance-motivated rules"; `tools/j2go/RUNTIME.md`,
"Collections and allocation"), one commit each with its measurement. On the `bundle` task
(interleaved runs, medians of 3 or more; the machine is noisy, so walls of different rows are
not comparable, only the two sides of one row):

| change | mallocs | wall | CPU |
|---|---:|---:|---:|
| profile-guided optimization (`default.pgo`) | 331.8M -> 313.3M | 56.0s -> 53.9s | 67.0s -> 64.3s |
| String keys reach maps and sets unboxed | 313.3M -> 236.7M | 51.8s -> 50.5s | 61.9s -> 59.4s |
| hash-table entries by value, sets without values | 236.9M -> 225.3M | 45.5s -> 44.9s | 53.4s -> 52.9s |
| tables of up to 8 entries searched linearly | 225.3M -> 216.1M | 45.3s -> 41.8s | 52.9s -> 48.8s |
| casts to interfaces through non-generic functions | 216.1M -> 215.3M | 43.3s -> 42.1s | 50.3s -> 49.3s |
| for-each on a stack cursor, no iterator object | 215.3M -> 195.3M | 37.4s -> 37.5s | 44.1s -> 43.7s |
| table capacity 4 at first insert, hash chains in a slice | 195.3M -> 181.5M | 44.1s -> 42.6s | 52.2s -> 49.9s |
| all together | 331.8M -> 181.5M | 54.6s -> 43.6s | 65.4s -> 51.1s |

Nine benchmark tasks with the final build, all byte-identical with Java (3 cold runs,
medians, same harness and machine as above):

| task | Java wall | Go wall | Go/Java | Go CPU | Go RSS |
|---|---:|---:|---:|---:|---:|
| typed-app | 11.4s | 12.2s | 1.07x | 15.0s | 300M |
| incremental-ijs-check | 7.8s | 7.7s | 0.99x | 10.2s | 606M |
| downlevel-simple | 8.8s | 8.7s | 0.99x | 10.2s | 228M |
| checks-only | 14.1s | 17.7s | 1.26x | 21.6s | 848M |
| generate-ijs | 6.4s | 8.8s | 1.36x | 10.2s | 624M |
| conformance-checks | 18.8s | 27.1s | 1.44x | 32.8s | 1201M |
| shaka-blockly | 26.8s | 40.1s | 1.49x | 49.3s | 654M |
| bundle | 26.6s | 42.8s | 1.61x | 50.2s | 726M |
| typed-full | 52.9s | 88.1s | 1.67x | 109.2s | 1421M |
| sum | 173.6s | 253.1s | 1.46x | 308.6s | |

**Representation prototypes and what was taken from them.** Three larger directions were
prototyped on copies of the generated code (byte-identical with Java, interleaved medians of 3,
no PGO):

| prototype | bundle | shaka-blockly | typed-app |
|---|---:|---:|---:|
| typed `String` field for the Node string payload | -6.5% | -6.0% | -1.9% |
| typed collections for six hot class groups (half of all collection time) | -8.4% | -2.6% | -3.0% |
| interned name ids with id-keyed scopes (on top of the two above) | -2% | 0% | 0% |
| slab allocation for 11 short-lived classes | -3.0% | -0.6% | 0% |
| all together | -16.8% | -10.3% | -8.2% |

Slabs for long-lived classes are harmful (RSS x2 to x7, no time gained: one live object pins
its chunk). Typed collections are the largest translator change of the four (reified
generics, about 2.6k lines of runtime collections redone, no variance in Go) for 8-10% on
optimisation-heavy inputs; not built. Taken: the string field, as Java prep step
`04b-node-string-field` (12 sites in `Node.java`, no translator change), plus two runtime
items (immutable enum sets answer `contains` by ordinal; hash lookups test identity before
`equals`, as `java.util.HashMap` does). Measured together against the build above
(interleaved, medians of 3): bundle 42.7s -> 40.3s, shaka-blockly 38.6s -> 35.7s, typed-app
11.3s -> 11.0s.

Per pass, the translation is about 1.5x slower than Java and uniformly so (1.0x to 2.8x); the
hand port is 3.3x *faster than Java* per pass (1.3x to 33x), which no translation of the
Java can reproduce: that part of the gap is hand-rewritten passes and data structures, not
translation overhead.

Tried and left out: a list of capacity up to 2 in one object with its header plus a first
capacity of 4 for `ArrayList` (5% fewer mallocs, no time gained); a profile regenerated after
the changes (not better than the first one, which is the one committed).

**Not done.** The transpiler still lowers try/catch to closures (56 catch
clauses remain in the Java), keeps the null-string sentinel and erases
generics. `rt_io.go` is 330 lines since step 16 reduced `Io.java` to 25
byte-level operations whose doc comments are the whole contract (it was 1.6k
lines against the facade of step 14, which exposed JDK behaviour). Tests are not translated. Git tags could not be
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
