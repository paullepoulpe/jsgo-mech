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
