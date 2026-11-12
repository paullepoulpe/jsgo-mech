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

## The data model

| Java | Go |
|---|---|
| `int`, `long`, `short`, `byte`, `char` | `int32`, `int64`, `int16`, `int8`, `uint16` |
| `boolean`, `double`, `float` | `bool`, `float64`, `float32` |
| `String` | `string`; a null String is the constant `nullstr_` |
| `Object`, type variables, `Integer`/`Long`/… boxes, `Number`, `CharSequence`, `Comparable` | `any` |
| a class `Foo` | `*Foo` (pointer to struct) |
| an interface `Bar` | Go interface `Bar` |
| `T[]` | `[]T` (`Object[]` and every array of a type variable is `[]any`) |

Generics are erased: collections hold `any`.

**Values inside `any`:** a `String` is a Go `string` (and a null String is
`nil`, never `nullstr_`: use `strobj_(s)` to box and `str_(v)` to unbox); an
`Integer` is an `int32`, `Long` an `int64`, `Character` a `uint16`, `Boolean`
a `bool`, `Double` a `float64`; an object is a pointer to its struct; `null` is
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
