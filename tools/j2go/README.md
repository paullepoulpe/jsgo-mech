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

A line `Class::field string-slot` in `handles.txt` (`Node::payload` until it became a
string-id field, below) names
a field of a handle class that is declared `Object` and often holds a `String`.
Its record gets a second slot `field_s_ string` (the null-string sentinel when
the field holds no String): assigning an expression of static type `String`
writes that slot directly, `field instanceof String s` and `(String) field`
read it, a copy from the same field of another object copies both slots, and
any other read or write goes through generated `field_get_()`/`field_set_(v)`,
which behave as the single `Object` field would. The string slot is a column of
its own (`field_s_()` returns a pointer into a chunk of strings): most reads of
such a field are of its String, and a column keeps them dense instead of
spreading them over the larger `C_ps_` records. The Java keeps one field, so
the object does not grow on the JVM (a 57th byte in `Node` costs Java 4%).

A line `Class::field string-id` (today `Node::payload` and `Node::originalName`) stores a
field as a `uint32` instead of a Go string or interface, so that the record holding it has no
Go pointer (`go/closure/rt_ids.go`). A field declared `String` holds an id into one
process-wide table of interned strings: `strid_(s)` interns (a hash lookup per store),
`strid_str_(id)` reads (one more load); id 0 is the null string and id 1 the empty one, so a
zeroed record reads as Java's default. A field declared `Object` that holds Strings and other
values holds a value code: a String's id, an integral number inline (29 bits), any other number
as an index into a table of doubles, any other object as an index into a table of objects (a
new entry per store, so identity is kept). Reads, tests, casts and copies are those of a
string-slot field, on the code: `field instanceof String` and `field instanceof Double` read
no table, storing a `double` needs no box, `field == null` is `code == 0`, and a copy from the
same field of another object copies the code (for a `String` field, the id).

A line `Class::field handle-table [Flag]` (today `Node::jstypeOrColor`, flag `JSType`) stores
a field declared `Object` whose values repeat across objects as a `uint32` handle into a table
that holds each distinct value once (`htab_set_`, `rt_ids.go`; a value is found again by its
interface words, so by identity). Bit 0 of the handle says whether the value is an instance
of the flag type, so `field instanceof Flag` is a bit test; any other read is a table load.
The table must not keep a value alive longer than the fields holding it would (the `JSType`s
that a second type-check replaces are garbage in Java): an entry counts the fields holding
its handle (`htab_set_` and, for a copy from the same field of another object, `htab_copy_`)
and is freed when the count returns to zero. An object of a handle class is never freed, so
what its fields hold stays, as it does for every other field of such an object.

A line `Class::field runtime GoType` (today `Node::propListHead`, `Node_props_`) gives a field
a storage the handwritten runtime defines: the record holds a `GoType`, which must hold no Go
pointer; a read of the field is `x.f_get_()`, a write `x.f_set_(v)`, `x.f == null` is
`x.f_isnull_()`, a copy from the same field of another object `x.f_copy_(y)`, and `x.f_p_()`
is the slot itself; all but the last are runtime methods that convert between `GoType` and
the Java value. A line `Class::method runtime` names a method (all overloads of that name)
that the runtime defines in place of the translation, with the Go name and signature the
translation would give it, so that the code using the field most can work on `GoType`
directly. For `Node` these are `getProp`, `getIntProp`, `getBooleanProp`, `putProp`,
`putIntProp` and `removeProp` (`go/closure/rt_node_props.go`); every other method that reads
the property list (debug strings, validation, serialization, equivalence) is translated and
reads the list Java would hold, which `propListHead_get_` builds from the bits.

A line `Class::field cold` names a pointer-free field of a handle class that
traversals do not read (today `Node::previous`, `linenoCharno`, `length`,
`originalName`, `jstypeOrColor` and `propListHead`). Cold
fields, and the handle's own value that `box_` points at, go to a second record
`C_cs_`, so that `C_ns_`, the record a traversal reads for every node, stays
small: `Node_ns_` is 16 bytes (next, first, parent, token), four nodes to a
cache line. A line `Class::field column` (today `Node::payload`) keeps a pointer-free
field in an array of its own, one element per object, instead of a record: most reads of
`Node.payload` are of its String, and a column of 4-byte codes keeps them dense. Only the
layout changes; reads and writes of the field are the same. A class whose fields are all
pointer-free has no `C_ps_` record.

**Type arguments in declarations.** Generics are erased, but the declared type
of a field, local or parameter whose Java type is a parameterized library type
is written through a generic alias: `List<Reference>` becomes
`ListOf[*Reference]`, with `type ListOf[T0 any] = List` in `g_0aliases.go`
(`BoxedOf[int32]` for `Integer`; a type variable or unbounded wildcard is
`any`). An alias is its right-hand side, so nothing changes for the compiler or
at run time; the Java type arguments stay in the source, checked as Go types,
for the post-flip steps that give collections real element types.

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
