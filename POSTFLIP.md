# Post-flip steps (branch `postflip`)

This branch starts from the Go that `tools/j2go` generates from `main` (commit "postflip:
freeze the generated Go") and changes the Go directly, one step per commit. A step makes the
code more idiomatic Go; it does not introduce new algorithms. The gate for every step is
`tools/prune/verify.sh`: translator self-test, runtime tests, and four projects byte-identical
with Java.

| step | how | what |
|---|---|---|
| 1 | `tools/postflip/slices` | 62 of 571 lists that never leave their owner become `[]T` |
| 2 | `tools/postflip/maps` | 114 of 702 maps and sets that are never iterated become `map[K]V` / `map[K]struct{}` |
| 3 (manual) | by hand, same pattern | typed collections for six hot groups that escape their owner (`rt_typed.go`) |

`tools/postflip/rw` is the shared rewriting engine: it type-checks package `closure`, and a
step registers replacements for sub-expressions; text outside them is untouched. The element
types come from the generic aliases j2go writes in declarations (`ListOf[*Reference]`).

Wall-clock, medians of 3 interleaved cold runs, outputs identical (Java from the nine-task run
on the same machine):

| task | Java | frozen Go | steps 1-2 | steps 1-3 |
|---|---:|---:|---:|---:|
| bundle | 26.0s | 41.0s | 38.6s (-5.9%) | 35.7s (-12.9%) |
| shaka-blockly | 26.5s | 34.7s | 34.7s (0%) | 33.9s (-2.2%) |
| typed-app | 10.7s | 10.9s | 11.0s (0%) | 10.7s (-1.8%) |
| typed-full | 51.2s | 77.6s | 78.2s (0%) | 74.7s (-3.7%) |

Tried and not kept: all of a node's fields in one record instead of two (bundle +1%,
typed-full +7%: the traversal wants the small pointer-free record).

Where the time is after step 3 (CPU of the whole process): typed-full 97.6s = collection
(GC) 20%, allocation 8%, hashing 16%, lists and iterators 2%, Node fields 17%, casts 3%,
everything else 34%; shaka-blockly has the same shape.

The collector is not what the wall-clock waits for: with collection switched off entirely
(`GOGC=off`, bundle after step 3) CPU time drops from 40s to 30s and wall-clock stays at
35-37s (34-36s with the collector on). What is left is the compiler thread's own work, about
30s on bundle against Java's 26s of wall-clock.

## Is the node store still needed?

Yes. Three builds generated from `main` (the node store is a translator setting, so this is
measured on the code before the post-flip steps, which do not touch `Node`), medians of 3:

| task | `*Node` pointers | store | store + typed string slot (what is shipped) |
|---|---:|---:|---:|
| bundle wall / CPU | 42.8s / 61.6s | 40.7s / 48.2s | 37.4s / 44.2s |
| typed-app | 12.2s / 16.8s | 11.4s / 14.0s | 10.9s / 13.4s |
| typed-full | 90.7s / 128.9s | 83.5s / 103.8s | 76.6s / 95.7s |

The store alone is 5-8% of wall-clock and about 20% of CPU; with the string slot, 11-16% of
wall-clock. One record per node instead of two is slower (above).

## What is still not idiomatic Go

Counted on this branch after step 3 (813 files, 242,624 lines, 1,992 struct types):

| pattern | occurrences | state |
|---|---:|---|
| collections still erased (`ListOf[...]`, `MapOf[...]`, ... declarations) | 2,920 | 176 converted by tools, about 20 by hand |
| values boxed into `any` (`obj_`, `strobj_`, `.box_()`) | 11,784 | follows the collections |
| values taken out of `any` (`cast_[T]`, `to_X`, `icast_X`, `str_`, `X_of_`) | 9,419 | follows the collections |
| fields, variables and parameters of type `any` | 2,263 | not addressed |
| Java library helpers (`String_x(`, `Objects_x(`, `Preconditions_x(`, ...) | 6,806 | not addressed |
| method names with overload suffixes (`add_int_Object`) | 6,116 | not addressed |
| constructor triples (`new_X` / `init_X` / `finit_X`) | 4,498 | not addressed |
| calls for the most derived object and upcasts (`.self()`, `.up_X()`) | 3,943 | not addressed |
| closures called on the spot (`func() T { ... }()`) | 1,929 | not addressed |
| `panic(` for `throw`; `recover()` | 1,588; 36 | not addressed |
| `Optional` objects created | 1,463 | not addressed |
| for-each cursors instead of `range` | 1,436 (343 `range`) | not addressed |
| getters `getX()` | 1,278 | not addressed |
| null-safe string conversions (`sstr_`) and the null-string sentinel | 1,207; 315 | not addressed |
| lambda objects (`&X_fn{...}`) | 1,110 | not addressed |
| Java collections constructed (`new_ArrayList...`) | 950 | reduced by steps 1-3 |
| static initializer functions (`X_clinit`) | 561 | not addressed |
| virtual dispatch by type assertion on `self_` | 545 | not addressed (118 have one implementation) |
| node store accessors (`.ns_()`, `.ps_()`) | 512 | kept on purpose |
| enum types that are pointers to objects | 206 | not addressed |
| panicking stubs for library members no tested path reached | 23 | not addressed |

One package holds everything. The handwritten runtime (about 15.5k lines without the generated
protobuf code) is all still referenced: the collection runtime (`rt_coll*.go`, `rt_guava*.go`,
5.0k lines) can only go when no erased collection is left.
