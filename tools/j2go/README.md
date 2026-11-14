# j2go

Translates the Java tree (`java/src`) to one Go package, `go/closure`.
`tools/j2go/run.sh` builds and runs it; the output files are `go/closure/g_*.go`
(not committed while the translation is still changing). The handwritten Go
runtime lives next to them in `go/closure/rt_*.go`.

It is a javac front end (`com.sun.source` public API only) plus an emitter.
javac's attribution supplies the type of every expression and the declaration
every name refers to, so the emitter never guesses.

## How Java maps to Go

**One package.** Every class lands in `package closure`, so Java names are kept
as they are (methods stay lower-case) and there are no import cycles or
visibility rules to reproduce. A class is named by its simple name, a nested
class `Outer_Inner`; a simple name used in two packages gets the package's last
segment appended.

**Types.**

| Java | Go |
|---|---|
| `int`, `long`, `char`, `boolean`, `double`… | `int32`, `int64`, `uint16`, `bool`, `float64`… |
| `String` | `string` (UTF-8); `null` is the constant `nullstr_` |
| class, enum, record | pointer to a struct |
| handle class (see below) | `uint32` handle |
| interface | Go interface |
| `Object`, boxed primitives, type variables | `any` |
| array | slice |

**Generics are erased**, as in the JVM: `List<String>` is `List`, its elements
are `any`, and reads convert back (`str_(list.get(0))`). The emitter inserts
those conversions wherever the static type and the erased declaration differ,
the way javac inserts `checkcast`. `Ctx.coerce` is the single place that knows
how to convert between two Java types.

**Inheritance.** A subclass struct embeds its superclass struct. A method
overridden somewhere below gets a stub on the top-most class that dispatches
through `self_` (the most-derived object, stored once at construction) to
`name_impl`; other methods are plain Go methods. `x.up_Super()` converts a
pointer to a superclass pointer, `to_Sub(v)` converts back, and a class-typed
value placed in an `any` or an interface is always the most-derived pointer, so
that identity and type tests work.

**Handle classes.** A class named in `tools/j2go/handles.txt` (today only
`com.google.javascript.rhino.Node`) is not a pointer to a struct but a 32-bit
handle, `type C uint32`, with 0 for `null`. Its instance fields are split by
type into two generated records per object: `C_ns_` for the fields that hold no
Go pointer (primitives, references to handle classes, and enum-typed fields,
kept as ordinal+1 and converted through the table `E_byOrd1_`), and `C_ps_` for
the rest. Records live in chunks of 65536 that are allocated on demand and
never moved or freed; a chunk of `C_ns_` records has no pointers, so the
collector never scans it and never follows a link between two such objects.
`x.f` becomes `x.ns_().f` or `x.ps_().f`; the accessor is an index into a table
of chunks. Chunk 0 is never allocated, so using the fields of handle 0 is a Go
nil dereference, which the runtime already reports as a
`NullPointerException`. Methods take the handle by value
(`func (this C) m()`), `new_C…` returns a handle from `C_alloc_()`, and a handle
that goes into an `any` (erased generics, `Object`) is boxed by `x.box_()` and
unboxed by `C_of_(v)`; `null` is a nil `any`, never a boxed 0. Everything else
(arrays `[]C`, `==`, `instanceof`, casts, lambdas, varargs) follows from the Go
type. The translator refuses a handle class that is not a plain class without a
superclass or subclasses, is an inner class, declares `finalize`, or is used
with `synchronized`, `wait`/`notify`, `Object.clone` or a weak reference;
nothing in the translator knows a class by name. `JSGO_HANDLE_STATS=1` makes the
Go program print, at exit, how many objects of each handle class exist and how
often each field is set. `go/closure/rt_handles.go` is the handwritten part.

**Constructors.** `new_C(args)` allocates, `init_C(args)` runs the constructor
body (calling the superclass's `init_`), `finit_C()` runs field initializers.
Overloads are told apart by a suffix made of the parameter types.

**Statics.** Static fields are package variables `C_name`, assigned in
`C_clinit()`. `g_0init.go` calls every `_clinit` in the order the Java program
initializes its classes (`tools/j2go/init-order.txt`, taken from a JVM log).
Compile-time constants become Go constants.

**Exceptions** are panics carrying the exception object; `try` becomes a
function literal with deferred `recover` (catch) and deferred cleanup (finally).
`return`, `break` and `continue` that leave a `try` are carried out of the
function literal through a result code.

**Lambdas and method references** become a function literal wrapped in the
adapter struct of the target interface (`&Predicate_fn{func(x any) bool {…}}`).

**Other lowering.** `c ? a : b`, `x = y` used as a value, and `i++` used as a
value become immediately-called function literals. Numeric promotion is spelled
out. Local and anonymous classes become top-level structs that hold their
captured variables and enclosing instance.

## Performance-motivated rules

These rules change no behaviour; each exists because a measurement said so (the commit that
introduced it has the numbers). All are keyed on Java types and methods, never on a class of
the translated program.

- **String keys are not boxed for a lookup.** A call whose root method is `Map.get`,
  `containsKey`, `remove(Object)`, `put`, `getOrDefault`, `putIfAbsent` or `computeIfAbsent`,
  `Collection.contains` or `remove(Object)`, or `add` on a static `Set` type, and whose key or
  element argument is statically a `String`, becomes a call of a runtime function that takes
  the Go string: `m.get(s)` is `Map_get_s(m, s)` (`ExprEmitter.stringKeyHelper`; runtime side in
  `rt_coll.go`, "String keys without a box"). Otherwise the string would be converted to `any`
  first, which allocates; that was a third of all allocations. A `put` or `add` boxes the key
  only when it is new. The runtime takes the fast path only for its own hash maps, hash sets
  and immutable collections; any other receiver gets the boxed call. `strobj_("literal")`
  needs no rule: Go allocates nothing for a constant.
- **Casts to an interface are not generic.** A conversion to an interface type `I` is
  `icast_I(v)`, a function generated once per interface in `g_0casts.go`, not `cast_[I](v)`:
  Go caches an interface type assertion per assertion site, but inside a generic function it
  cannot and searches the global method-table cache every time (7 ns against 2 ns). Casts to
  a concrete class keep `cast_[*T]`, which is a pointer comparison either way.
- **For-each has no iterator object.** `for (T x : c)` over an `Iterable` is
  `it := foreach_(c)` followed by `for it.next() { x := it.v … }`. The cursor is a local
  struct; for the runtime's lists, immutable collections, hash sets and map views it walks
  the backing slice in place, as their iterators do, and it holds an `Iterator` only for
  anything else. The cursor must be declared before the `for`, not in its init clause, where
  Go would copy it at every iteration.
- **Profile-guided optimization.** `go/cmd/mech/default.pgo` is a merged CPU profile of the
  four gate projects; `go build` uses it by itself. `tools/pgo/regen.sh` regenerates it.

What the runtime does for the same reason (entries of hash tables stored by value, small
tables searched linearly, no per-entry or per-view objects) is described in `RUNTIME.md`.

## Library code

Calls into the JDK, Guava, Gson, protobuf and args4j are translated by rule: an
instance method stays a method call on a Go type of the same name, a static
method becomes `Owner_name(...)`, methods of `String`, boxed types and `Object`
become receiver-first functions (`String_length(s)`). Overloads get a
parameter-type suffix.

`g_zstubs.go` is generated: a placeholder that panics for every library type and
member the translation uses and `rt_*.go` does not define yet. The package
therefore always builds, and a run names the first missing piece.
`externals.tsv` lists everything the translation asks of the runtime.
