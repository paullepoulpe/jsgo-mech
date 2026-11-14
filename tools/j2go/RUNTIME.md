# Writing the Go runtime (`go/closure/rt_*.go`)

The translated compiler calls JDK, Guava, Gson, protobuf and args4j code. That
code is written by hand here, in Go, in the same package as the translated
code. `go/closure/g_zstubs.go` (generated) holds a placeholder that panics for
every library type, method and function that is still missing; implementing
one means writing the real declaration **with exactly the same name and
signature** in an `rt_*.go` file and regenerating, after which the placeholder
disappears.

    tools/j2go/regen.sh go/closure      # refresh g_*.go against the rt_*.go files (10 s)
    cd go && go build ./... && go test ./closure/

Behaviour must match the Java library: the goal is byte-identical compiler
output. Write from the library's documented behaviour (Javadoc), never by
copying JDK source.

## Classes implemented by hand

A Java class listed in `tools/j2go/handwritten.txt` is not translated: the
translator treats it as a library class, and the runtime implements it like any
other. There are two. `com.google.javascript.jscomp.base.Io` holds the 25 operations that need
the operating system or a codec of the platform's library (files and directories as bytes, the
standard streams, bundled resources, UTF-8, DEFLATE, gzip, base64); it is `go/closure/rt_io.go`,
330 lines on Go's standard library, written to the doc comments of `Io.java`, which are the
whole contract, error messages included. Charsets, paths, globs, zip archives, URIs and text
sinks are translated Java on top of it. Nothing else in the runtime imitates `java.io`,
`java.nio`, `java.util.zip` or Guava's `io` package (`rt_io_java.go` keeps the few in-memory
objects that translated code still hands to the SAX parser). `com.google.javascript.jscomp.base.Json`, which parses
a JSON document into the translated `SimpleJsonValue`, is `go/closure/rt_json.go`,
on `encoding/json`; the runtime has no Gson. `rt_io_test.go` and
`rt_json_test.go` check both against `go/closure/testdata/io/*.tsv`, which
`tools/iotable/run.sh` writes by calling the Java classes on a few thousand
inputs and on a directory tree it builds.

## The data model

| Java | Go |
|---|---|
| `int`, `long`, `short`, `byte`, `char` | `int32`, `int64`, `int16`, `int8`, `uint16` |
| `boolean`, `double`, `float` | `bool`, `float64`, `float32` |
| `String` | `string`; a null String is the constant `nullstr_` |
| `Object`, type variables, `Integer`/`Long`/… boxes, `Number`, `CharSequence`, `Comparable` | `any` |
| a class `Foo` | `*Foo` (pointer to struct) |
| a handle class `Node` (`tools/j2go/handles.txt`) | `Node`, a `uint32` handle; 0 is null |
| an interface `Bar` | Go interface `Bar` |
| `T[]` | `[]T` (`Object[]` and every array of a type variable is `[]any`) |

Generics are erased: collections hold `any`.

**Values inside `any`:** a `String` is a Go `string` (and a null String is
`nil`, never `nullstr_`: use `strobj_(s)` to box and `str_(v)` to unbox); an
`Integer` is an `int32`, `Long` an `int64`, `Character` a `uint16`, `Boolean`
a `bool`, `Double` a `float64`; an object is a pointer to its struct (an object of a handle class is the
handle value: box with `n.box_()`, unbox with `Node_of_(v)`); `null` is
`nil`. Never put a typed nil pointer in an `any` or an interface: use
`obj_(p)`, which returns a nil `any` for a nil pointer.

**Strings** are UTF-8 Go strings, but every index a Java API takes or returns
counts UTF-16 code units. Use the functions in `rt_string.go`
(`String_length`, `String_charAt`, `String_substring`, `String_indexOf…`,
`String_toCharArray`, `fromUnits_`) instead of `len` and slicing unless the
string is known to be ASCII (`allASCII_(s)`).

**Equality and hashing** of values in `any` follow Java: `objequals_(a, b)`
(null-safe `Objects.equals`), `objhash_(a)`, `objstr_(a)` (`String.valueOf`),
`compareNatural_(a, b)` (`Comparable.compareTo`). A type takes part by having
methods `equals(any) bool`, `hashCode() int32`, `toString() string`,
`compareTo(any) int32`. Give every collection-like type `equals`, `hashCode`
and `toString` with the Java semantics.

**Exceptions** are panics whose value is the exception object:
`panic(new_IllegalStateException_String("msg").self())`. Constructors exist
for the standard shapes: `new_X_0()`, `new_X_String(msg)`,
`new_X_String_Throwable(msg, cause)`, `new_X_Throwable(cause)`.

**Functional interfaces** (`Function`, `Predicate`, `Supplier`, `Comparator`,
`Consumer`…) are Go interfaces with the Java method (`f.apply(x)`,
`p.test(x)`, `c.compare(a, b)`). To make one from a Go closure use its adapter
struct: `&Function_fn{func(x any) any { … }}` (see `g_zstubs.go` for which
exist), or `&cmpFunc_{…}` for a `Comparator`.

**Classes that translated code extends** (the placeholder has a `self_ any`
field and `as_X`/`to_X`/`is_X`/`self()` functions) must keep those, and call
methods the subclass may override through `this.self_`, e.g.
`this.self_.(interface{ computeNext() any }).computeNext()`.

## Collections and allocation

Most objects the compiler allocates are collections and what hangs off them, so the
collections in `rt_coll.go` are written to allocate little:

- A hash map or set is one `htable_`: the keys in insertion order in one slice, the values in
  a second (a set has none), and nothing per entry. A removed entry leaves a marker in the key
  slice until the table is compacted.
- A table of up to `hsmall_` (8) entries has no index: lookups scan the key slice. Only
  strings and keys compared by identity or Go equality live in such a table; the first key
  with its own `equals`/`hashCode` builds the index, so that it is found by hash code and
  `equals` exactly as in Java.
- `Map.Entry` objects exist only for entries that an `entrySet()` iteration hands out. The
  table remembers them, so a mapping has one entry object, which sees later `put`s and whose
  `setValue` writes through.
- `keySet()`, `values()` and `entrySet()` are the table itself under another Go type.
- `Map_get_s` and its siblings take a Go string key (see "Performance-motivated rules" in
  `README.md`); `foreach_` is the cursor of translated for-each loops. Both take a fast path
  only when the receiver is exactly one of the runtime's own types, so a translated class
  that implements or extends a collection keeps its own methods.

`rt_coll_test.go` checks maps and sets against a linear model under random operations, and
the for-each cursor against the iterators. To see where allocations come from, run the
compiler with `JSGO_MEM_PROFILE=<file>` (then `go tool pprof -sample_index=alloc_objects`),
`JSGO_MEMSTATS=<file>` for totals, `JSGO_CPU_PROFILE=<file>` for CPU.

## Rules

- Names and signatures come from `g_zstubs.go`. Do not rename; do not change a
  signature. Interface types (`List`, `Map`, `Iterator`, …) are generated from
  what the translated code calls: do **not** declare them by hand. Your structs
  must have every method of each interface they are used as (a missing method
  shows up as a failed type assertion at run time), plus `equals`, `hashCode`,
  `toString` where Java defines them. If you need a method an interface lacks,
  assert to an inline interface: `x.(interface{ foo() int32 })`.
- A struct you write replaces the placeholder struct of the same name. You do
  not need to mirror the JDK's class hierarchy.
- Unexported helpers you add end in an underscore (`joinAny_`) so they can never
  clash with a translated name. Put everything for your area in files named
  `rt_<area>*.go` and tests in `rt_<area>*_test.go`; do not edit other
  `rt_*.go` files or any `g_*.go` file (they are overwritten).
- Hash-based collections iterate in insertion order (`LinkedHashMap`,
  `LinkedHashSet` in `rt_coll.go`); `Immutable*` collections keep the order
  they were built in, as in Guava.
- Existing building blocks: `ArrayList`/`listOf_(items)`, `LinkedHashMap`,
  `LinkedHashSet`, `TreeMap`, `TreeSet`, `ArrayDeque`, `sliceIter_`,
  `funcIter_`, `toSlice_(iterable)`, `sortSlice_`, `StringBuilder`.
- Single goroutine for now: no locking is needed, and executors may run tasks
  inline.
- Test with ordinary Go tests in package `closure` that call your functions and
  compare with what the Java library returns for the same inputs.
