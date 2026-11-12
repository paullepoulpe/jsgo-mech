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
