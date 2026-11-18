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
| `Object`, boxed primitives, erased type variables | `any` |
| array | slice |

**Generics are erased** by default, as in the JVM: `List<String>` is `List`, its elements
are `any`, and reads convert back (`str_(list.get(0))`). The emitter inserts
those conversions wherever the static type and the erased declaration differ,
the way javac inserts `checkcast`. `Ctx.coerce` is the single place that knows
how to convert between two Java types.

**Typed generics.** The program's own generic classes, interfaces and methods keep their type
parameters where that is sound (`GenScan`; `J2GO_GENERICS=1` reports every declaration,
`J2GO_ERASED=1` translates everything erased). A type parameter is kept when every mention of
its declaration names, at its position, a concrete type or a kept type variable; a wildcard or
raw use there, or an erased type variable, erases that parameter alone (`DiGraph<Node, ?>`
somewhere makes `DiGraph[N]`, with the second parameter `any` everywhere). Then:

- `C<X>` is `*C[X]`, `I<X>` is `I[X]`; a subclass struct embeds its superclass at its view of it
  (`ControlFlowGraph[N]` embeds `LinkedDirectedGraph[N, *ControlFlowGraph_Branch]`), and
  `to_C[X]`, `up_C`, dispatch through `self_` work as for other classes; `x instanceof C` is the
  marker method `isa_C_` (any instantiation).
- A method group whose root a typed declaration declares has the root's signature at the
  receiver's (or implementing class's) view of that declaration; the lambda adapter of a typed
  functional interface is generic (`I_fn[X]`).
- Local and anonymous classes take the type parameters of the scope they are declared in.
- A bounded type parameter is kept when its bound is an interface of the program (the Go
  constraint: `DataFlowAnalysis[N any, L LatticeElement]`); a class bound erases it to the
  bound, and one that every use instantiates with exactly its bound stands for the bound as a
  type argument (`Outcome<B>` in `AbstractCommandLineRunner<A, B extends CompilerOptions>` is
  `Outcome[*CompilerOptions]`).
- Generic methods are Go generic functions when their type variables are needed as type
  arguments of a kept parameter (or are factories of a typed class); an instance method that
  overrides nothing and that nothing overrides becomes a function taking the receiver first
  (`CmdLineParser_Option_enumValue[E any](this, values []E, ...)`). Other method type variables
  are erased to their bound.
- A value of a type parameter meets `Object` as the erased code boxed it (`boxT_`, `unboxT_`,
  `null_`, `isnull_`, `eqT_`, and `hashCodeT_`/`equalsT_`/`toStringT_` without boxing a String;
  `go/closure/rt_generic.go`).
- Wildcards written for brevity do not erase. A local variable (or the variable of a for-each
  loop) that is never assigned again and whose declared type has wildcard type arguments is
  typed from its initializer or from the elements it iterates over
  (`for (DiGraphEdge<?, E> edge : getEdgesInDirection(a, b))` is a `DiGraph_DiGraphEdge[N]`);
  the expressions that read it are typed from that (`GenScan.refine`). A static final field
  `C<?, ?>` initialized with `new C<>(...)`, which the program only casts to an instantiation
  or compares with one (`==`), is one object per instantiation, made on first use by a generic
  function (`HamtPMap_EMPTY[K, V]()`, `single_` in `rt_generic.go`); a static final empty array
  `C<?, ?>[]` that is only cast is an empty array of the instantiation at each cast, and
  `(C<K, V>[]) new C<?, ?>[n]` is a new array of the instantiation (`GenScan.shared`). The
  constructor of a shared static must have no effect but the new object: it runs once per
  instantiation, on first use.
- A class without subclasses converts to a typed interface it implements as Go converts it
  (null stays the nil interface), not through `cast_`.
- Performance rule: generic Go code does not assert a value to an interface type that names a
  type parameter where it can be avoided (Go caches such an assertion per site in ordinary code,
  but looks the method table up on every call when the interface comes from a type parameter).
  A generic class with dispatched methods holds its most derived object a second time, as its
  typed dispatch interface (`vt_C_ C_vt_[N]`, set next to `self_`), and its stubs call through
  it; `to_C[X](v)` asks for the view through an interface without type parameters
  (`as_C_() any`) and asserts the pointer type.

A lambda whose target is a library functional interface is a func of its Java types inside a
typed adapter when a parameter or the result that the erased signature has as `any` has a type
that one type assertion reaches (a String, a handle, a class without subclasses, an interface,
an array). The adapter is generated per instantiation (`&Consumer_tfn_Node{func(name Node)
{...}}`, `Function_tfn_string_JSChunk`; `g_zstubs.go`) and converts at the erased signature
with the conversions the erased code makes for those types (`Node_of_`, `str_`, `cast_[*C]`,
`icast_I`; `strobj_`, `obj_`, `box_()`), so it costs what the erased lambda cost. Only an
adapter whose types name a type variable is generic (`Function_tfn[N, any]`, converting with
`unboxT_`/`boxT_`).

**`java.util.Optional` is a value.** The runtime's `Optional[T]` (`rt_base.go`) is a generic
struct, not a heap object: `Optional<X>` is `Optional[X]`, its content held as an `X`, and
`Optional.of`/`empty`/`ofNullable` are generic functions that allocate nothing. Its zero value
is Java's `null` (a field never assigned; `o == null` is `o.null_()`), and every method of a
null Optional throws `NullPointerException` as a call on null does. A wildcard, raw or erased
type argument is `any` at that site only (`Optional[any]`): an Optional passes between two
instantiations by a conversion of its content (`optAs_`), and to and from `Object` by
`box_()`/`optOf_` (nil for null), where Java passes one object; `equals` compares the contents
of any two instantiations. Java gives an Optional no identity a program may use (it is a
value-based class), and the translator refuses `==` between two Optionals. `J2GO_ERASED=1`
keeps this representation (the runtime has no other).

**Typed collections.** A declaration of type `List<X>` / `ArrayList<X>` holds a typed list,
`*TList[X]` (`rt_tlist.go`: a slice of X values, not of boxes), and one of type `Set<X>` /
`HashSet<X>` / `LinkedHashSet<X>` a typed set, `*TSet[X]` (`rt_tset.go`: the X values in
insertion order, a Go map from element to position once the set is not small), one of type
`Deque<X>` / `ArrayDeque<X>` / `Queue<X>` a typed deque, `*TDeque[X]` (`rt_tdeque.go`: a typed
list whose first item is the head, as the erased ArrayDeque is an ArrayList), when the
translator can see that it only ever holds collections the program makes for it (`Colls`;
`J2GO_COLLS=1` reports the counts and why declarations stay erased, `J2GO_COLLS=2` each one).
The decision is made declaration by declaration: a field, a local variable, and the parameters
and the result of a method whose signature is its own (a constructor, a static or private
method, an instance method that overrides nothing and that nothing overrides; not one named by
a method reference). A declaration is typed when

- X has one Go type and no type arguments (a String, a class, an interface, a handle); for a
  set, X is a type whose Go `==` is Java's `equals`: a String, an enum, or a class of the
  program (a handle class too) that inherits `equals` and `hashCode` from Object, as all its
  subclasses do (`Ctx.goEquals`);
- every value that enters it is a new collection of its kind (`new ArrayList<>()`,
  `new ArrayList<>(n)`, `new ArrayList<>(collection)`; `new LinkedHashSet<>(..)`, the only set
  the program makes; `new ArrayDeque<>(..)`), `null`, or the value of another typed
  declaration of the same kind and element
  type (an argument enters the parameter, a returned expression the result);
- every use is one the typed collection or its view serves: a `List` / `Set` / `Deque` method,
  a for-each loop, `==`, a string concatenation, or a place of type `List` / `Set` / `Deque` /
  `Queue`, `Collection`, `Iterable`, `Object` or a type variable. A place that wants the class
  (`ArrayList`, `LinkedHashSet`, `ArrayDeque`) must be a typed declaration itself; `subList`,
  `listIterator` and `getClass` keep the declaration erased.

`list.get(i)`, `add`, `set`, `remove(int)`, `size`, `isEmpty`, `clear`, `iterator`, a set's
`add`, `contains` and `remove` of an X (`s.has(x)`, `s.del(x)`), a deque's `push`, `pop`,
`poll`, `peek`, `peekLast`, `add` (and `addFirst`, `removeFirst`...) and the for-each loop
(`it := list.each()`) work on the X values; any other method, and every place that is not a
typed declaration (a wildcard or raw type, `Object`, a library parameter, a method of a group
of overriding methods), sees the collection as Java's `List` / `Set` / `Deque` through
`list.asList()` / `set.asSet()` / `deque.asDeque()`. That is the collection's one erased view, made on first use, which boxes and
casts as the erased code did: the Java object stays one object wherever it goes (`==`, changes
made on either side), `null` is `nil` on both sides, and a typed declaration never holds a
collection that erased code made, so no conversion from the erased form exists. Inside the
translator a typed declaration, and each expression that reads it, has the type `TList<X>` /
`TSet<X>` / `TDeque<X>` of a class that exists for it alone (`Ctx.STUBS`: the typed methods), and
`Ctx.coerce` turns that type into any other through the view. The element conversions are
generated per element type (`tc_string`, `tc_p_JSType` in `g_0casts.go`) with the conversions
the erased code makes for that type. The runtime's erased code reads a view without an
iterator object (`foreach_`, `stepper_`) and looks a String up in it without a box.

Two differences from the JVM are accepted. An element of the wrong type that unchecked code
puts into a typed collection through its view fails there with the `ClassCastException` that
Java throws at the first typed read (or never, if the element is never read at its type). And
the view is not the Go type `*ArrayList` / `*LinkedHashSet` / `*ArrayDeque`: its `getClass()` is the Java
class, but `x instanceof ArrayList` or a cast to `ArrayList` of a view that reached erased
code would fail (the program has neither; a declaration of the class type that a typed
collection reaches is typed too, or the collection stays erased).
`J2GO_ERASED=1` skips the rule.

**Type arguments in declarations.** Library types stay erased; the declared type of a field,
local or parameter whose Java type is a parameterized library type is written through a generic
alias: `List<Reference>` becomes
`ListOf[*Reference]`, with `type ListOf[T0 any] = List` in `g_0aliases.go`
(`BoxedOf[int32]` for `Integer`; a type variable or unbounded wildcard is
`any`). An alias is its right-hand side, so nothing changes for the compiler or
at run time; the Java type arguments stay in the source, checked as Go types,
for the post-flip steps that give collections real element types.

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

A line `Class::field string-slot` in `handles.txt` (today `Node::payload`) names
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

A line `Class::field cold` names a pointer-free field of a handle class that
traversals do not read (today `Node::previous`, `linenoCharno`, `length`). Cold
fields, and the handle's own value that `box_` points at, go to a third record
`C_cs_`, so that `C_ns_`, the record a traversal reads for every node, stays
small: `Node_ns_` is 16 bytes (next, first, parent, token), four nodes to a
cache line. Only the layout changes; reads and writes of the field are the same.

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
