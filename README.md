# jsgo-mech

A trial of the mechanical Java-to-Go migration plan for Closure Compiler
(`migration_plan.md` in [jsgo](https://github.com/paullepoulpe/jsgo)): can a
throwaway transpiler plus a small handwritten Go runtime reproduce the Java
compiler byte for byte, and how large and how fast is the result?

Nothing here is copied from the existing jsgo port.

## Layout

- `pin.env`: the pinned Closure Compiler release.
- `tools/fetch.sh`: downloads that release's sources and jar into `.cache/`.
- `tools/apisurface/`: lists the library methods Closure's Java calls
  (`external-api.tsv`). That list is what the handwritten Go runtime has to
  cover.
