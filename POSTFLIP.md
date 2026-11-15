# Post-flip steps (branch `postflip3`)

The branch starts from the Go that `tools/j2go` generates from `main` ("postflip: freeze the
generated Go") and changes the Go directly, one step per commit, each made by a tool in
`tools/postflip` (`replay.sh` replays steps 3-7). A step makes the code more idiomatic Go; it
does not change algorithms. Gate for every step: `tools/prune/verify.sh` (runtime tests, four
projects byte-identical with Java). From step 7 the translator self-test no longer applies.

| step | tool | change | coverage |
|---|---|---|---|
| 1 | `slices` | lists that never leave their owner become `[]T` | 62 of 571 |
| 2 | `maps` | maps/sets never iterated become `map[K]V` | 114 of 702 |
| 3 | `colls -kind list` | lists typed across method boundaries (`*TList[T]`) | 661 of 1,014 |
| 4 | `colls -kind map` | maps typed across method boundaries (`*TMap[K, V]`) | 371 of 576 |
| 5 | `colls -kind set` | sets typed across method boundaries (`*TSet[K]`) | 259 of 608 |
| 6 | `dispatch` | single-implementation dispatch is a concrete type assertion | 134 of 547 |
| 7 | `optional` + sed | `java.util.Optional` is a value | all |
| (8) | `lambdas` (not applied) | lambdas as function values: unsound, Go function values are not comparable and a `Runnable` is a set element (typed-full crashed) | - |

## Speed

Medians of 3 interleaved cold runs, one machine, nothing else running, outputs identical:

| | frozen | step 2 | step 3 | step 4 | step 5 | step 6 | step 7 |
|---|---:|---:|---:|---:|---:|---:|---:|
| bundle wall | 33.4s | 32.5s | 32.5s | 33.8s | 35.0s | 35.2s | 34.9s |
| bundle CPU | 39.5s | 38.6s | 38.7s | 40.4s | 42.2s | 42.3s | 42.5s |
| bundle allocated | 9.97G | 9.66G | 9.57G | 9.53G | 11.14G | 11.14G | 11.15G |
| typed-full wall | 69.1s | 68.2s | 70.8s | 71.6s | 70.4s | 71.8s | 70.3s |
| typed-full CPU | 87.0s | 84.7s | 89.0s | 90.2s | 89.3s | 89.3s | 88.8s |

Run-to-run spread is about 3-4% (an earlier run of the same step-4 binary measured -8% on
bundle). No step is a clear gain; steps 1-2 are about -2%, typed collections are neutral to
+3%, typed sets allocate 1.6 GB more on bundle (+4%), steps 6 and 7 are neutral. All steps
together: bundle +4.7%, typed-full +1.6%.

Why typed collections do not pay: boxing a pointer into `any` costs nothing in Go, so the
typed collections remove type assertions but not allocations (mallocs 183M -> 172M); a typed
map still hashes every lookup, and every place where a typed collection meets erased code
(484 views) adds a conversion.

## What is still not idiomatic (after step 7)

| pattern | before the steps | after |
|---|---:|---:|
| erased collection declarations | 2,920 | 1,339 (1,287 typed, 49 native) |
| views where typed collections meet erased code | - | 484 |
| boxing (`obj_`, `strobj_`) | 10,185 | 9,543 |
| unboxing (`cast_`, `to_X`, `icast_X`, `str_`, ...) | 9,419 | 8,467 |
| Java collection constructors | 950 | 425 |
| dispatch by anonymous-interface assertion | 545 | 411 |
| fields, variables, parameters of type `any` | 2,263 | 2,450 |
| Java library helpers (`String_x`, `Objects_x`, ...) | 6,806 | 6,798 |
| overload-suffixed method names | 6,116 | 6,116 |
| constructor triples (`new_X`/`init_X`/`finit_X`) | 4,498 | 4,498 |
| `.self()` calls / upcasts | 3,943 | 3,682 |
| closures called on the spot | 1,929 | 1,929 |
| `panic` for throw | 1,588 | 1,588 |
| null-string handling | 1,522 | 1,523 |
| for-each cursors (vs 343 `range`) | 1,436 | 1,435 |
| lambda adapter objects | 1,110 | 1,113 |
| static initializers (`_clinit`) | 561 | 561 |
| enum types as pointers to objects | 206 | 206 |

## Node store

Still needed: measured on the frozen code (the steps do not touch `Node`), plain `*Node`
pointers instead of the store cost 5-8% wall-clock and about 20% CPU (bundle 42.8s / 61.6s
against 40.7s / 48.2s; with the typed string slot 37.4s / 44.2s).
