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

**Covariant overrides.** Go has no covariant results. A method of a class or of an interface
that overrides one with a wider result type (both class or interface types, the overridden one
not a type variable) starts a method group of its own, named after the overridden group and its
result (`getScope` -> `getScope_rScope`), with its own signature (`Ctx.covariant`). A call that
javac resolves to the override (the receiver's static type is its class or interface, or below)
has the narrower result and no cast; a call through the wider type goes through a bridge, as
javac's bridge method does. `J2GO_NO_COVARIANT=1` turns the rule off, `J2GO_NO_COVARIANT_IFACE=1`
its interface part; `J2GO_COVARIANT=1` lists the overrides it applies to (106 in the tree, 6 of
them interface methods).

- *In a class*: the class implements the overridden group with a bridge that calls the method
  and widens the result (`Ctx.covBridges`, `Emitter.covBridges`).
- *In an interface* (`StaticTypedScope.getParentScope` narrowing `StaticScope`'s): the Go
  interface has the narrower method and, through the interface it embeds, the wider one, so
  everything under it needs a Go method of both groups. For each class under the interface,
  superclasses first, the method `x` that runs for the narrowing method is looked up
  (`Ctx.implOf`: a method of the class or of a superclass, else a default method); each of the
  two groups that no class of the chain has yet gets a bridge to `x`'s group in that class
  (`analyzeMethods`, "bridges of narrowing interface methods"). A subclass that overrides `x`
  is reached through the dispatch of `x`'s group. An abstract class that leaves the method to
  its subclasses gets the dispatching stubs of both groups, and the subclasses the bridges. A
  class method that narrows a superclass's method and has the result of the interface's
  method joins the interface's group (`Ctx.covLeft`) and bridges the superclass's. Where two
  groups have one Go name and signature in a class (two interfaces narrowing one method to
  one type), one method is both (`Ctx.covAlias`). The lambda adapter `I_fn` of a functional
  interface under the narrowing one gets the same bridges (`Emitter.interfaceDecl`), and a
  default method that a narrowing method overrides is not the body of its group any more
  (`Ctx.covOverridden`). `Ctx.implementsInGo` counts the bridges.
- *Not in an interface* (one group, as before, listed as `not covariant` by `J2GO_COVARIANT=1`)
  when some class under it takes the method from a superclass outside the interface the wider
  group comes from: that method's own group has the wider group's Go name with another result.
  (Such a class does not implement the Go interface without the rule either.) An interface
  that narrows a method of `Object` also keeps one group.

`J2GO_IMPLEMENTS=1` lists every (class, program interface above it) pair, from which a file of
`var _ I = *new(C)` lines checks the method sets with the Go compiler.

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
- Piece `ownwild` (`J2GO_NOLIBGEN=ownwild` translates without it; `GenScan.ownWild`): inside
  generic class C's own code a variable written `C<?>` that only holds a C of the receiver's
  instantiation is `C<T>`, so the wildcard does not erase C for the whole program. A parameter
  `C<?>` of an instance method of C that overrides nothing and that nothing overrides, never
  assigned, never referenced (`::`), whose every call passes an argument of the receiver's
  instantiation (`SourcePosition.isSamePositionAs(SourcePosition<?> that)`: `that` is
  `*SourcePosition[T]`); and in C's `equals(Object o)`, `if (o instanceof C) { C<?> that = (C<?>)
  o; ...`: `that` is `C<T>` and the test is of the instantiation (`is_[*C[T]](o)`). Go then
  answers false for an object of another instantiation, where Java runs the block: the method
  must be listed with the argument that the block answers false too in
  `tools/j2go/ownwild-equals.txt` (`AbstractCommandLineRunner.FlagEntry.equals`: instantiated with
  two enums), and every type argument written for C must be an enum, a type variable or a
  wildcard. Anything else is left (counted in the `J2GO_LIBGEN` report, `ownwild: left: ...`);
  `selftest/lang/OwnWild.java` has two typed classes and eight left.
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

**Function types** (`FnTypes`; `J2GO_NOFNTYPES=1` is the translation without the rule,
`J2GO_FNTYPES=1` reports what it still converts, `=2` by type). A library functional interface
of the list `FnTypes.LIBS` (`java.util.function`'s `Supplier`, `Function`, `Predicate`,
`Consumer`, `BiFunction`, `BiConsumer`, `BiPredicate`, `ToIntFunction`, `ToLongFunction`,
`IntFunction`; Guava's `Supplier`, `Function`, `Predicate`; `java.util.Comparator`;
`java.util.concurrent.Callable`) is a named Go func type of its Java type arguments, with the
interface's method declared on it (`g_zstubs.go`):

```go
type Predicate[T any] func(T) bool
func (f Predicate[T]) test(p0 T) bool  { return f(p0) }
func (f Predicate[T]) apply(p0 T) bool { return f(p0) }   // Guava's Predicate is the same type
type Predicate_base[T any] = Predicate[T]
```

- Types: these interfaces are typed declarations as `java.util.Optional` is. A wildcard is its
  bound (`? super X` and `? extends X` are X, `?` is `any`); an erased type variable is what the
  erased code has for it (its bound); `Object` and a boxed primitive are `any` (`FnTypes.norm`,
  applied by `Ctx.erase`).
- A lambda or a method reference is the func literal at the type of the place it goes to
  (`Predicate_base[Node](func(p0 Node) bool { return p0.isIn() })`); there is no adapter object.
- A call of the interface's method is the method of the func type (`pred.apply(n)`), which Go
  calls statically. Null is nil; calling it is the nil dereference that the runtime reports as
  a `NullPointerException`.
- A class of the program that implements the interface has the method with its Java types
  (`apply(n Node) bool`) and no bridge with the erased signature. An object of it given where
  the interface is wanted is its method value, nil for null. A class of the runtime, and a
  class of the program under one (`extends Ordering<X>`), keeps the erased method
  (`FnTypes.erasedAt`): the runtime calls it.
- A function meets another instantiation of its interface through a closure that converts the
  arguments and the result as the erased code converted them (`FnTypes.between`): variance
  (`Predicate<Object>` for `Predicate<? super Node>`), an erased type variable of a generic
  method (`Callable<T>` is `Callable[any]`), the erased signature of a runtime function
  (`Iterables.filter` takes a `Predicate_base[any]`). A lambda written at such a place is
  written at the place's type and needs no closure.
- A function meets `Object` (an element of an erased collection, a type variable) at its
  instantiation at `any`, in a box that is the closure's pointer at a type of its own
  (`f.box_()`, `fnUnbox_`; `rt_fn.go`): no allocation, nil for null, comparable, and two boxes
  are equal exactly when they box the same closure.
- `Preconditions.checkNotNull(f, ...)` / `Objects.requireNonNull(f, ...)` of a function test its
  box and give the function. The members that the translator generates for a record compare,
  hash and print a function component by its closure (`fnSame_`, `fnHash_`, `fnStr_`).
- The static generic methods of these interfaces and of Guava's `Predicates`, `Suppliers` and
  `Functions` are Go generic functions of the runtime, called with javac's type arguments
  (`Comparator_comparingInt[*Node](..)`, `Predicates_alwaysTrue[Node]()`); the default methods
  of `Comparator` are methods of the func type.

Records with function components: the `equals`, `hashCode` and `toString` that the translator
generates for a record go over every component, so a record with a functional component
(`PassFactory`, `CheckAccessControls.PropertyReference`, `ChangeVerifier.EqualsResult`) compares
functions although no source line does. This is accepted, not refused: two components are equal
when they are the same closure (`fnSame_`: a Go func value is one pointer to its closure, and
the pointers are compared), which is what Java's reference comparison of the two lambda objects
gives as long as the value is passed around at one type. A function that went through a
converting closure (another instantiation, an erased collection) is another closure, as a
re-evaluated lambda expression is another object in Java. `rt_fn_test.go` has a test that fails
if a func value stops being one pointer (`TestFnValueIsOnePointer`).

A Go func value has no identity and no `equals`. The translator stops, naming each place
(`function types: refused: File.java:12: ...`), when the program compares two functional values
with `==`, tests or casts one (`instanceof`, a cast of or to one that is not an upcast), calls
`equals` / `hashCode` / `toString` of `Object` on one, uses one as the key of a map or an element
of a set (also through the type parameters of the program's own classes whose fields are such
maps and sets), or gives one where `Object` is declared (`FnTypes.check`). None of these
occurs in the tree.

The runtime is one for both translations: its code takes and gives functions at `any` through
names that `g_zstubs.go` declares either way (`Function_e`, `Comparator_e`, ... and
`Function_of_(func)`), and the functions whose signature differs are in `rt_fn.go` (the rule) and
`rt_fn_erased.go` (build tag `j2go_nofntypes`). So the translation without the rule is
`J2GO_NOFNTYPES=1 tools/j2go/run.sh` and `go build -tags j2go_nofntypes`;
`J2GO_NOFNTYPES=1 tools/prune/verify.sh` and the self-test pass the tag.

Not in the rule: `Comparable` (still `any`; a class's `compareTo` keeps the erased signature and
its bridge), the functional interfaces without type parameters (`Runnable`, `BooleanSupplier`:
their adapters `Runnable_fn` have the Java types already), and the program's own functional
interfaces (`CompilerPass`, `NodeUtil.Visitor`: adapters `I_fn`).

**Typed library interfaces** (`LibGen`; `J2GO_NOLIBGEN=iter` turns the kind off, `all` every
kind and piece; `J2GO_LIBGEN=1` reports what the rule converts, `=2` by type). A place whose Java type is
`Iterator<X>` or `Iterable<X>` (kind `iter`) holds the typed interface of the runtime
(`rt_lg.go`): the erased interface plus the methods that give elements, at their Java type and
under the name with the suffix `_t`, as a class file has a method and its bridge.

```go
type IteratorT[T any] interface { Iterator; next_t() T }
type IterableT[T any] interface { Iterable; iterator_t() IteratorT[T] }

var parameters IteratorT[*FunctionType_Parameter] = functionType.getParameters().iterator_t()
parameter = parameters.next_t()                       // was cast_[*FunctionType_Parameter](parameters.next())
it_1 := eachOf_(scope.getVarIterable())               // for (Var v : ...): it_1.v is a *Var
```

- A wildcard is its bound; `Iterable<?>` and `Iterable<Object>` are `IterableT[any]`. The
  erased interface (`Iterable`, without type arguments) is what the runtime's own functions and
  collections take and give (`LibGen.rt`): the signature of a library method is erased as before.
- Every typed value is a value of the erased interface too, the same object, so it goes to
  erased code as it is; where it may be a wrapper or a typed collection the object that erased
  code knows is taken out first (`Iterable_un_`; `lgBox_` for `Object`, `==`, `instanceof`,
  `equals`), so identity and the runtime's own type tests see what they saw before.
- An erased value becomes a typed one through `iterableAs_(tc_X, v)` / `iteratorAs_`: `v` itself
  where its object has the typed methods at that type, else a wrapper that unboxes each
  element as the erased code cast it after the call. Nothing is checked before an element is
  read, as with an unchecked cast in Java. Two instantiations (`Iterable<Dog>` for `Iterable<?
  extends Animal>`) meet the same way.
- Native objects, which need no wrapper: the typed collections (`*TList[T]`, `*TSet[T]`,
  `*TDeque[T]` are `IterableT[T]` themselves, `l.asIterable()`; their iterators are
  `IteratorT[T]`), their erased views, the typed list of an immutable list, and a class of the
  program that names the interface among its interfaces and has no subclass: it has the method
  with its Java types (`next_t() Node`) and the bridge with the erased name and signature. Any
  other class of the program (one with subclasses, one under a library class such as
  `AbstractIterator`, one that gets the interface through an interface of the program) keeps
  the erased method and is wrapped where a typed value is wanted.
- A for-each loop over a typed Iterable runs on the typed cursor `eachOf_` (over a typed list
  it reads the slice in place); over the result of an erased method it is the erased loop as
  before, not a wrapper.
- A raw use of one of the interfaces is refused (`typed library interfaces: refused: ...`); the
  tree has none.

The collection kinds (`coll`: `java.util.Collection`; `list`: `java.util.List`; `set`:
`java.util.Set`) have the same form. A place of type `Collection<X>` / `List<X>` / `Set<X>` that
rule "typed collections" does not make a concrete typed collection holds `CollectionT[X]` /
`ListT[X]` / `SetT[X]`:

```go
type CollectionT[T any] interface { Collection; iterator_t() IteratorT[T] }
type ListT[T any] interface { List; iterator_t() IteratorT[T]; get_t(i int32) T }
type SetT[T any] interface { Set; iterator_t() IteratorT[T] }
```

- Which methods have a typed form is a table (`LibGen.TYPED_METHODS`: `List.get`); every other
  method of the interface is the erased one that the typed interface embeds, with the runtime's
  signature (`l.add(obj)`, `l.addAll(Collection_un_(c))`), so the coverage can grow method by
  method without touching the places.
- A typed collection of rule "typed collections" enters as its view, which is part of it and
  has the typed methods: `l.asListT()`, `s.asSetT()`, `l.asCollT()` (a list, a set, a deque). The view serves
  an immutable typed collection too; erased code and `Object` get the object they knew
  (`List_un_`, `lgBox_`: the view of a collection that can change, the twin of an immutable one).
- **Typed by place** (piece `place`): an expression that makes a new collection for a place of a
  typed interface with the collection's own element type is the runtime's typed collection:
  `new ArrayList<>(..)`, `ImmutableList.of(..)`, `ImmutableList.copyOf(..)`, the runtime's
  constructors (`Ctx.TYPED_MAKERS`) are `newTList_(tc_X).asListT()`,
  `immutableListOf_[X](tc_X, a, b).asCollT()`... For a place of an erased type (`Object`, a
  parameter of a runtime function) the expression makes the erased collection as before, which
  is the cheapest form for erased readers.
- Piece `views`: `values()` of a typed map is `m.valuesT()`, the map itself as
  `CollectionT[V]` (no object, no box; also of an immutable map), `keySet()` is `m.keySetT()`
  (`SetT[K]`, the same way), and `keySet()` / `values()`
  of the erased view of a typed map are native Iterables. `Collections.unmodifiableX(x)` where a
  typed interface is wanted is `x` at that type (the runtime's is its argument).
- Piece `makers`: Guava's constructors with an expected size are the runtime's typed
  constructors where their value enters a typed declaration.
- Piece `cov`: an override in a class whose result is the same interface at a narrower element
  type (`Iterable<Var>` under `Iterable<? extends AbstractVar>`) is a method group of its own,
  as rule "covariant overrides" makes for class results (`getVarIterable_rIterableOf_Var`): a
  caller that has the narrowing class reads the elements at their type and no wrapper is made;
  the bridge of the wider group converts.
- Variance: a `List<? extends X>` or `List<?>` place stays the erased `List`
  (`LibGen.wildErased`). A list of a narrower element type enters such a place (the graph
  classes' `List<LinkedDiGraphEdge>` where `List<? extends DiGraphEdge>` is declared); as a
  typed List of the wider type it would be a second object per list, which was measured as one
  allocation per call of the graph nodes' `getInEdges` / `getOutEdges`. Erased, the list enters
  as it is and the reader casts each element. A local of such a type that is assigned once
  takes the type of its initializer (`GenScan.tryRefine`, as for the program's own generic
  types), so `List<? extends DiGraphNode<N, E>> next = graph.getDirectedSuccNodes(n)` is the
  typed list of the narrowing override. `Collection<? extends X>` and `Iterable<? extends X>`
  are typed at X (a narrower collection enters through a wrapper).
- A call whose result enters a place of type `Object` (`one.equals(list.get(0))`,
  `String.valueOf(it.next())`) is the erased method: javac casts the result to the element type
  only where that type is wanted, so an element of another class that unchecked code put there
  passes, as in Java.
- Anything else of the static type of the erased interface (a declaration of the class
  `ArrayList<X>`, the result of a runtime function, an element of a collection of collections)
  enters through `listAs_(tc_X, v)` / `collectionAs_`: `v` itself where its object is native,
  the view of the typed list of an erased immutable list, else a wrapper (one allocation).
  `J2GO_LIBGEN=1` counts these sites, `=2` lists them by type.

Kind `map` (`java.util.Map`, `java.util.Map.Entry`):

```go
type MapT[K any, V any] interface {
	Map
	get_t(k any) V; keySet_t() SetT[K]; values_t() CollectionT[V]; entrySet_t() SetT[Map_EntryT[K, V]]
}
type Map_EntryT[K any, V any] interface { Map_Entry; getKey_t() K; getValue_t() V }
```

- A typed map enters as its view (`m.asMapT()`; also an immutable or sorted one), an erased map
  through `mapAs_(tc_K, tc_V, v)`, an entry through `map_EntryAs_`. `new LinkedHashMap<>()`,
  `ImmutableMap.of(..)` made for a `MapT` place are the typed map (piece `place`).
  `entrySet()` of a typed map is `m.entrySetT()`; its entries are the objects the erased view
  gives (one per entry, as in Java), which have the typed methods.
- A lookup by a String key stays the runtime's helper on the erased object
  (`Map_get_s(Map_un_(m), k)`: no box for the key); the helpers and `instanceof` see the object
  erased code knows, never a wrapper.
- The variable of a for-each loop over the entries of an erased map holds the erased entry (a
  typed one would be a wrapper per entry).
- Piece `elems`: a collection that is an element, key or value of a typed collection is held at
  its typed interface: `Map<K, List<X>>` is `*TMap[K, ListT[X]]`, and `computeIfAbsent(k, x ->
  new ArrayList<>())` makes the typed list (before: the erased `List`, a cast at every read).
- Piece `wide`: a place whose type arguments are all the wildcard `?` (`Iterable<?>`, `Map<?,
  ?>`), or one of whose type arguments is an erased type variable (`<V> void f(Map<V, X> m)`),
  stays the erased interface: collections of every element type enter it, and each would need a
  wrapper. (`Collection<Object>` and `? extends V` at V = Object stay typed at `any`: that is
  also what a member of a Go generic class has at that instantiation.)

- Piece `nncoll`: `checkNotNull(x)` / `requireNonNull(x)` of a typed collection is the typed
  check of the collection itself (`Preconditions_checkNotNull_T(x)`: a pointer test). As an
  `Object` it was the collection's erased view, which for an immutable collection is its twin,
  made and boxed for the check alone.
- Piece `readers`: a variable or field holding a typed list, set or map that is an argument of
  a library method which only reads it (`LibGen.READERS`, a list of 12: `Iterables.getLast`,
  the message arguments of `Preconditions.checkArgument` / `checkState`, `Collection.addAll` /
  `containsAll` / `removeAll`, `Map.putAll` ...) is given as the collection's view
  (`x.asListT()`), which is part of it; as any other erased argument an immutable typed
  collection is its twin, made with every element boxed for that one read.

- Piece `cls`: a place whose type is one of the collection classes that the typed collections
  stand for, with type arguments (`LibGen.CLASSES`: `ArrayList`, `LinkedHashSet`, `HashSet`,
  `LinkedHashMap`, `HashMap`, `ImmutableList`, `ImmutableSet`, `ImmutableCollection`,
  `ImmutableMap`), and that rule "typed collections" does not make a concrete typed collection
  (a method of a record, a lambda, a result read through a type variable), holds the typed
  interface of the class's collection interface: `ImmutableList<X>` and `ArrayList<X>` are
  `ListT[X]`, `ImmutableMap<K, V>` is `MapT[K, V]`, `ImmutableCollection<X>` is
  `CollectionT[X]`. What is made for the place is typed by it as for the interface
  (`ImmutableList.of(..)`, `new ArrayList<>()`, a typed builder's `build()`: the typed
  collection's view), so a typed immutable list is no longer turned into its erased twin for
  such a result and found again by the caller.
  - Only where the typed family can hold the elements (`Ctx.typedArgs`): `ImmutableMap<Object,
    X>`, `ArrayList<LocalClass>`, `ImmutableList<String[]>`, a key with type arguments, an erased
    type variable or a wildcard keep the class (`*ImmutableList`), because everything entering
    would need a wrapper. A wide use of a class of `LibGen.CONCRETE` (`ArrayList<?>`) is the
    erased interface (`List`): a list made for a typed place of the class is no `*ArrayList`.
  - The signatures of the runtime keep the class (`LibGen.rt`); an object of the class enters
    through `listAs_` (the view of the typed collection of an erased immutable one).
  - A method that only the class has is called on the object erased code knows:
    `names.reverse()` is `cast_[*ImmutableList](lgBox_(names)).reverse()` (the twin). A method
    of another interface (`SequencedCollection.getFirst`) gets the object as any erased
    parameter does. Of the methods that a class of `CONCRETE` declares itself only those of
    `LibGen.OWN` (`ArrayList.trimToSize`), which the view has too, are translated, through the
    interface of that one method; any other is refused.
  - A value of such a place entering a declaration of rule "typed collections" is the typed
    collection whose view it is (`tlistOfT_`, `tsetOfT_`, `tmapOfT_`), without the twin.
  - A cast to the class is the cast to the class first (`ClassCastException` as in Java), then
    the place's type.
  - A new collection of a class of `CONCRETE` whose own type is such a place, entering an
    `Object` or an erased type variable (`return new LinkedHashMap<>(arg)` where the group's
    result is a type variable; `ArrayList::new` for a `Supplier<T>` at `T` = `List<X>` or
    `ArrayList<X>`), is made typed by its own type: the place of the class that reads it back
    gets a native value, not a wrapper per read. (An immutable one stays erased there: for
    `Object` it would be its twin.)
  - An erased `LinkedHashMap` that typed code reads (made where its key and value types are
    erased type variables, as the maps of `JSDocInfo`'s properties) keeps the wrapper made for
    it, and the wrapper its typed key set and values (`mapAs_`, `rt_lg.go`): one wrapper per
    map, not one per read.
- Piece `meth`: the methods that take an element at its type have a typed form too
  (`LibGen.TYPED_METHODS2`: `Collection.add` is `add_t(x T) bool`, `List.set` / `remove(int)`,
  `Map.put` / `remove`, `Map.Entry.setValue`): no box of a String or of a value of a type
  parameter for the argument, no cast of the result. Methods whose parameters are all `Object`
  (`contains`, `containsKey`, `remove(Object)`, `indexOf`) have nothing to type and stay the
  erased ones (a String key goes through the helpers `Map_get_s`...). `J2GO_LIBGEN=1` counts
  the erased methods still called on a typed receiver, and those of them that box an argument
  or give an element as `Object`.
- Piece `fns` (`LibFns`; needs rule "function types"): the runtime's static library functions
  and the default methods of the interfaces have typed twins at their Java generic signatures
  (`go/closure/rt_lgfn.go`, built with rule "function types" only): `Iterables.getFirst` is
  `Iterables_getFirstT_[T](IterableT[T], T) T`, `Map.forEach` is `Map_forEachT_[K, V](MapT[K, V],
  BiConsumer[K, V])`, `Map.get` with a String key on a map of String keys is `Map_get_sT_[V]`.
  A name list (`LibFns.TYPED`, with the Go signature of each twin as a template) chooses them; a
  call is the twin's, with javac's type arguments, where the Go types that javac's types give
  for the call are the template's, every collection argument and the receiver enter at their
  typed interface as they are (a typed collection, a typed interface of the same element
  types: no wrapper), and the result does not go to an `Object` (Java casts nothing there).
  Every other call keeps the erased function; `J2GO_LIBGEN=2` lists them by reason, and the
  functions of the same kind without a twin (`LibFns.REFUSED`: `Lists.reverse` / `transform`,
  live views; `SequencedCollection`'s methods, `CacheLoader.from`, `Map.putIfAbsent`). Also:
  `new TreeSet<>(comparator)` / `new TreeMap<>(comparator)` of a typed collection take the
  comparator at the key type (`newTSortedSetCT_`; the sorted set compares its keys without
  boxes, `teq_.cmpT`), and `Splitter.splitToList` of a String takes it unboxed
  (`splitToListST_`). `selftest/lang/LibFns.java` has the twins (33 calls, counted by
  `selftest/run.sh`) and calls that must stay erased (4).
- Kinds of rule candidate T2 (`docs/javaisms6/JAVAISMS6.md`; `LibGen.CLASSES_T2`, each kind its
  own switch): more library types whose places hold the typed interface of their collection
  interface, by the mechanism of piece `cls`. A method of the type that its collection
  interface (or one above it) does not declare, including the methods of other interfaces
  (`NavigableSet.descendingSet`), is called on the object erased code knows.
  - Kind `sorted`: `ImmutableSortedSet<X>` and `SortedSet<X>` are `SetT[X]`,
    `ImmutableSortedMap<K, V>` is `MapT[K, V]`. Nothing is made typed for them: an erased sorted
    immutable enters through `setAs_` / `mapAs_` (one wrapper per value; its elements are read
    in the erased collection's order, unboxed as the erased loop cast them). `TreeMap` is not
    in the kind: its 4 places have a key with type arguments (`Property<?>`), and a
    `TreeMap` made for a typed place is the typed sorted map's view, on which the class's
    navigable methods would fail.
  - Kind `enumset`: `EnumSet<E>` is `SetT[E]`, and what `EnumSet.noneOf` / `allOf` / `of(e)` /
    `of(e, rest...)` / `copyOf` make for such a place (`LibGen.MAKERS_T2`) is the typed set in
    its sorted mode with the ordinal comparator of the runtime's `EnumSet` (`rt_lg_t2.go`:
    `tenumSetNoneOf_` ...), the typed twin of the same structure. Such a set is no `EnumSet`
    object, so `instanceof EnumSet`, a cast to it and a method that only `EnumSet` declares are
    refused (the tree has none), as is a library method with a parameter of the class
    (`EnumSet.copyOf(EnumSet)`, `complementOf`: its argument may be such a set); another
    overload (`EnumSet.of(a, b, c)`) makes the erased object, which enters through `setAs_`.
  - Kind `cmaps`: `ConcurrentHashMap<K, V>`, `ConcurrentMap<K, V>`, `IdentityHashMap<K, V>`,
    `EnumMap<K, V>` are `MapT[K, V]`. Nothing is made typed for them (the typed map compares
    keys by `equals` and keeps no lock): the erased map enters through `mapAs_`, one wrapper
    per value, and keeps its own comparison and order.
  - Kind `deque` (a kind of `LibGen.KINDS`, as `list`): `Queue<X>` is `QueueT[X]` and `Deque<X>`
    is `DequeT[X]` (`rt_lg_t2.go`: the erased interface, the methods of `CollectionT[X]`, and
    typed `peek` / `poll` / `remove()`, `pop` / `push`); `PriorityQueue<X>` is `QueueT[X]`
    (`LibGen.CLASSES_T2`). The typed deque of rule "typed collections" (`TDeque`) is native
    through its erased view (`asDequeT()` / `asQueueT()`), so `new ArrayDeque<>()` for such a
    place is typed by it; any other object (an erased `ArrayDeque`, a `LinkedList`, a
    `PriorityQueue`) enters through `dequeAs_` / `queueAs_`. Of the methods that `Deque`
    shares with `SequencedCollection` (`getFirst`, `addFirst` ...) none is typed by the kind:
    they are calls of the runtime's helpers on the erased object, unless piece `seqcoll` has a
    twin for them (below).
  - Piece `seqcoll` (`LibFns.TYPED_SEQ`, the mechanism of piece `fns`, and off with it; needs rule "function
    types" as `fns` does): `SequencedCollection.getFirst` / `getLast` / `removeLast` /
    `addLast`, an interface with no typed interface of its own, have typed twins
    (`SequencedCollection_getFirstT_[X](c)`, `rt_lgfn.go`) whose receiver is the typed
    `Collection` of the interface it extends (`LibFns.RECV_AT`): a typed deque or list enters as
    its view (`d.asCollT()`, was `SequencedCollection(Deque_un_(d.asDequeT()))`), a `DequeT` /
    `ListT` as it is; the result needs no cast.
  - Piece `objarg` (`LibObj`, rule candidate T3 of docs/javaisms6): `Map.get` / `containsKey` /
    `remove(Object)` on a Map receiver and `contains` / `remove(Object)` on a Set receiver (a
    typed interface, or a typed collection whose own methods do not serve the call), with an
    argument whose static Java type is the key or element type (or a subtype) and whose Go form is
    a value of that type (no field or result of a type variable's type, which Go has as `any`),
    are typed twins (`Map_getK_[K, V](m, k)`, `Set_containsE_[T](s, x)`, `rt_lgobj.go`) that take
    the argument unboxed: the typed map's or set's own lookup where the object is its view, else
    the erased method with `boxT_(k)`; a null argument always takes the erased method. Only for
    the key / element types on `tools/j2go/objarg-pairs.txt` (the self-test adds
    `selftest/objarg-pairs.txt`; `J2GO_OBJARG_PAIRS` replaces the list): the types for which the
    translation has a typed map or set, so the twins make Go compile no typed map that the
    translation does not have. The translation fails (exit 3, "objarg: the pair list is stale")
    when the list is not exactly the pairs that the calls ask for and that have a typed map or set
    (only reported when another switch is off); `J2GO_OBJARG_PAIRS_OUT=<file>` writes the list it
    should be. An argument of a wider type (Object, a supertype, another interface), a key type
    that is `any` in Go, a wildcard or not comparable, a String that the String-key helpers serve,
    a result that enters an Object, a List or Collection receiver (no typed lookup) and a pair not
    on the list keep the erased call (`J2GO_LIBGEN=2`: "Object-parameter methods", by reason).
  - Piece `cmp` (`LibCmp`; rule candidate T4 of `docs/javaisms6/JAVAISMS6.md`): a class of the
    program that implements `Comparable<C>` of its own type has `compareTo_t(o *C) int32` at its
    Java types and the bridge `compareTo(p0 any)`, which casts its argument and calls it
    (javac's bridge). The runtime's typed natural orders (`naturalT_` of the typed sorts,
    `tsortCmp_` of the typed sorted collections, `Comparator_naturalOrder[T]`,
    `Comparator_comparing[T, U]`) call `compareTo_t` through `ComparableT[T]` (`rt_lgcmp.go`)
    before they box the values for `compareNatural_`, which calls the bridge; a call
    `x.compareTo(y)` on a receiver of the class is `x.compareTo_t(y)`. Left erased (counted in
    `J2GO_LIBGEN`'s report by reason): raw `Comparable`, `Comparable` of another type than the
    class (a supertype, another instantiation), `Comparable` through a superclass or an
    interface of the program, a class with a subclass, a handle class, a `compareTo` that also
    implements another interface's method. `selftest/lang/Cmp.java` has two typed classes and
    six left erased; the self-test counts them.
  - Kind `guava` (a kind of `LibGen.KINDS`): `PeekingIterator<X>` is `PeekingIteratorT[X]`
    (typed `next` and `peek`), `LoadingCache<K, V>` is `LoadingCacheT[K, V]` (typed `get` and
    `getUnchecked`). The runtime makes both erased (`Iterators.peekingIterator`,
    `CacheBuilder.build`), so a value enters through `peekingIteratorAs_` / `loadingCacheAs_`,
    one wrapper per value.
  - Kind `goptional` (`Ctx.gopt`; it needs rule "function types"): Guava's
    `com.google.common.base.Optional<X>` is a value library class, as `java.util.Optional` is
    (below): `GOptional[X]` (`rt_lg_gopt.go`), a generic struct whose zero value is null, its
    content held as an `X`; `Optional.of` / `absent` / `fromNullable` are generic functions that
    allocate nothing (`GOptional_of[X](x)`, was `Optional_base_of(obj_(x))`), and `get` /
    `or` / `orNull` give an `X` (no cast). The erased class of the runtime (`*Optional_base`)
    keeps its name and is what the translation without the kind names. The methods are those of
    `LibGen.GOPT_METHODS`, which also gives each its Go name: `or` has three overloads of one
    name and arity, `or(T)` is `or`, `or(Optional)` is `or_Optional` (whose parameter, a
    wildcard, is `GOptional[any]`), whatever the program's uses of each (the rule for other
    library overloads). A wildcard or erased type argument is `any` at that site, as for
    `java.util.Optional`, and a member's type there has `Object` for it (`Ctx.atSite`), so the
    result of `get` on an `Optional<? extends X>` is cast where it is used.

  - Piece `nbox` (`LibNbox`; rule candidate R5 of `docs/javaisms6/JAVAISMS6.md`): a field or a
    method parameter declared as a boxed primitive (`Boolean`, `Integer`, ...: a value that may
    be null, the tri-state flags) is `Nbox[T]` (`rt_nbox.go`: the primitive and a flag, zero
    value null), and a method parameter of a library function type with a boxed type argument
    (`Consumer<Boolean> set`) is the function type at `Nbox[T]`, with the lambdas given for it.
    It is the value-library mechanism of `java.util.Optional` (`Ctx.valueLibs`: `== null` is
    `null_()`, Object is `box_()`) for a stand-in class `j2go.rt.Nbox<X>` that only the
    translator sees (`Main.withStubs`); Optional's own Go type does not fit (its Object form is
    the Optional, a Boolean's is the bool; it has three states). Conversions (`Ctx.coerce`):
    `nbox_(v)` from a primitive, `nboxOf_[T](o)` from an Object (nil is null), `n.get_()` to a
    primitive (NullPointerException for null), `n.box_()` to an Object. Left, counted by reason
    in the `J2GO_LIBGEN` report (`nbox: left ...`): a variable with a use the conversions do not
    serve as Java does (a method called on it, `++` / `+=` of it, an assignment used as a value,
    a string concatenation, `==` with anything but null or a primitive, other forms), a
    parameter of a constructor or of a method that overrides, is overridden or is referenced
    (`::`), a function parameter passed on as a value or given anything but a lambda, a field
    of a record, an interface or a handle class. `selftest/lang/TriBox.java` counts them.
  - Piece `owniter` (`StmtEmitter.ownLoop`; rule candidate T6 of `docs/javaisms6/JAVAISMS6.md`):
    a for-each loop over a value whose static type is a class of the program that has the typed
    `Iterable` methods itself (`LibGen.nativeClass`: it names `Iterable<X>` among its
    interfaces, has no subclass and is no handle: `Node.AncestorIterable`, `ReferenceCollection`,
    `SimpleMultiset<E>`) runs on the typed cursor, `it := eachOf_[Node](n.getAncestors())`: the
    class's `iterator_t()`, whose iterator gives each element at its type (was
    `foreach_(obj_(x))`, the erased `iterator()` bridge, a box per element and a cast of it). A
    loop over a call of one of EnumSet's typed makers of kind `enumset` (`EnumSet.allOf(X.class)`
    ...) runs on the typed set that the maker makes for a typed place (`tenumSetAllOf_(tc_X,
    ..).each()`: the same constants in ordinal order; was the erased `EnumSet` boxed for the
    erased loop). Left erased, counted in the `J2GO_LIBGEN` report (`owniter: loops left erased:
    ...`): a class whose `Iterable` comes through a library superclass (`Graph.GraphAnnotationState
    extends ArrayList<AnnotationState>`: its iterator is the runtime's erased one), through a
    superclass or an interface of the program, a class with a subclass, a handle class, a loop
    over an interface of the program, an EnumSet function without a typed maker.
    `selftest/lang/OwnIter.java` counts the cursors.
  - Piece `hstatic` (`Colls`; rule candidate T7 of `docs/javaisms6/JAVAISMS6.md`): a static field
    of a handle class is a Go variable as every other static field (only the instance fields of a
    handle live in its chunks), so rule "typed collections" decides it as it decides those; it
    was erased as "a field of a handle class". In the tree this is `Node.PROP_MAP_FOR_EQUALITY`
    (`ImmutableMap<Prop, Function<Node, Object>>`): its builder chain is the typed builder of
    `Function[Node, any]` values (`newTMapBuilder_(tc_p_Node_Prop, tc_Function_Node__any_)`),
    where each of the 15 functions went through a converting closure to `Function[any, any]` and
    a box, and each `get` in `Node.isEquivalentTo` through the element conversion's closure back.
    `J2GO_LIBGEN`'s report counts the static fields of handle classes (`hstatic: ...`); the pair
    of that map on `objarg-pairs.txt` is marked `@off hstatic` (asked for only with the piece off,
    `LibObj.COND`). `selftest/lang/HStatic.java` counts the declarations.
  - Pieces of the eighth flip's residue (`docs/javaisms6/RESIDUE8.md`; recorded in
    `docs/libgen/COLLECTIONS-PLAN.md` section 16), each its own switch:
    - `objlist` (`LibObj.listTwin`, `rt_objlist.go`): `contains` / `remove(Object)` / `indexOf` /
      `lastIndexOf` on a List or Collection receiver with an argument of the element type, where
      Java's `x.equals(e)` is Go's `==` (a String, an enum, a class of the program in whose
      hierarchy no class declares `equals`): `TList_indexOfE_[T](l, x)` scans the typed list
      with `==` (a `ListT` / `CollectionT` receiver: `List_*E_` / `Collection_*E_`, the scan
      where the object is a typed list's view, only for element types on `objarg-pairs.txt`,
      lines `@on objlist`); a null argument takes the erased method.
    - `freshloop` (`StmtEmitter.freshLoop`, `rt_freshloop.go`): a for-each over
      `ImmutableList.of(..)` / `copyOf(..)` / `sortedCopyOf(c)` makes the typed list at its element
      type and runs on its cursor; over `Lists.reverse(l)` of an ImmutableList a typed cursor
      backwards.
    - `mkext` (`ExprEmitter.mkextNew`, `mkextFreshArg`, `immutableOf`; `rt_mkext.go`): for a typed
      place, `new EnumMap<>(K.class)` (the typed sorted map with the ordinal comparator), `new
      IdentityHashMap<>()` (keys compared by `==`), `new ConcurrentHashMap<>()`, `new
      LinkedList<>()` for a Deque, `new C<>(ImmutableX.of(..))`, `ImmutableSet.copyOf(E[])`,
      `ImmutableMap.of` with 6 to 10 entries, `of(..)` of a subclass's values for a `? extends`
      place. The four classes are in `LibGen.MADE_T2` (their tests, casts, own methods refused).
    - `gsorted` (`ExprEmitter.gsortedOf`, `rt_gsorted.go`): `ImmutableSortedSet.copyOf(c)` /
      `copyOfSorted(s)` / `of()`, `ImmutableSortedMap.copyOf(m)`, `ImmutableList.sortedCopyOf(c)`
      and `sortedCopyOf(cmp, c)` for a typed place (and the values of a fresh
      `ImmutableSortedMap.copyOf(m)` in a for-each) are typed sorted immutables, whose twin
      (`asImm`) is the runtime's ImmutableSortedSet / ImmutableSortedMap.

  - Piece `proto` (`LibProto`; rule candidate T5 of `docs/javaisms6/JAVAISMS6.md`; needs kinds
    `list` and `iter`): a call of a protobuf message's list accessor whose element is a message
    or a String (`List<Foo> getFooList()`, `ProtocolStringList getBarList()`), on the message,
    its builder or its `OrBuilder` interface, is the call of the typed accessor that
    `tools/protogen` writes next to the erased one (its rule T5): `getFooList_t() ListT[*Foo]`,
    `getBarList_t() ListT[string]` (a new typed list, `pbListT_` in `rt_proto.go`); a call of
    `Msg.toBuilder()` is `toBuilder_t() *Msg_Builder`. So a for-each over it runs on the typed
    cursor and its elements need no cast. A local of type `ProtocolStringList` (written `var`)
    that holds such a list and is never assigned again is a `List<String>` (`GenScan.refine`).
    Left, counted in the `J2GO_LIBGEN` report (`proto: list accessors left erased: ...`): a
    list of enums, numbers or bytes, and the OrBuilder / Builder lists (`List<? extends ..>`).
    jsgo's generated messages have the same typed forms (`GetValueList() []string`,
    `Clone() *Requirement`). With the piece on, `rt_proto_gen.go` must have protogen's typed
    accessors (protogen's default); a translation without it compiles with either.

  - Pieces `rtm`, `fns2` and `seqstr` (`docs/javaisms6/RESIDUE8.md`, its rules "typed runtime
    instance methods", "more LibFns twins" and "`Conv.strSeq` name list"; COLLECTIONS-PLAN.md
    section 15): name lists of twins in the runtime (`rt_resid.go`, `rt_lgfn2.go`), each doing
    what the erased method does with its arguments at their Java types.
    - `rtm` (`LibFns.RT_METHODS`, `TYPED_RT`): a method of a runtime class on a receiver of that
      class (`BigInteger.compareTo(BigInteger)` is `x.compareTo_t(y)`, `ComparisonChain.compare`
      of two Strings `compare_s`, `Enum.compareTo` of a program enum `compareTo_t` (the
      ordinals), `(BitSet) bs.clone()` `bs.clone_t()`, no box and no cast), and methods of Guava's
      immutable classes and of `TreeSet` / `Map` on a typed collection: `ImmutableList.subList` /
      `reverse` of a typed immutable list give the typed immutable list (`subListImm_`, or
      `ImmutableList_subListT_` on the typed List of a place of the class), not the erased twin's
      result made typed again; `TreeSet.tailSet` the typed sorted set (`tailSetT_`),
      `Map.containsValue` of a value of the value's Go type `containsValueT_`; `List.add(int, E)`
      of a typed List `List_add_intT_`.
    - `fns2` (`LibFns.TYPED_F2`, as piece `fns`; needs rule "function types"): `Arrays.setAll` of
      an array of a class type (no converted copy), `Comparator.comparing` /
      `thenComparing` with a String key (the keys compared as Strings, not boxed),
      `Lists.transform` whose result is the argument of a method that reads it once
      (`LibFns.READS_ONCE`: `ImmutableList.Builder.addAll`) is `Iterables.transform`'s twin at
      `Iterable`, `Splitter.split` of a String an `IterableT[string]` (`split_sT`),
      `Io.listFiles` a `ListT[string]`, `Map.putIfAbsent` / `replace` with a String key on a
      typed map, `Collections.singletonList` a typed list.
    - `seqstr` (`Conv.STR_SEQ2`): more String variants for CharSequence parameters
      (`TextFormat.merge`, `Ascii.equalsIgnoreCase`, `Integer.parseInt(CharSequence, int, int,
      int)`, `CharMatcher.lastIndexIn` / `collapseFrom`).
    Calls that do not fit (another Go type, a receiver that is erased, a key that is no String)
    keep the erased method, counted by reason in the `J2GO_LIBGEN` report (`typed runtime
    methods (rtm): calls left (...)`); `J2GO_LIBGEN` also names the entries of the lists that no
    call of the translation took. `selftest/lang/Resid.java` has the twins and calls left;
    `selftest/run.sh` counts them.

Refused (`typed library interfaces: refused: File.java:line: ...`, nothing is written):

- a raw use of one of the interfaces;
- an unchecked cast to a use of one of them (`(List<String>) object`, `(Map<String, X>)
  mapOfOther`), unless the method it is in is listed in `tools/j2go/unchecked-casts.txt` (2
  methods of the tree, each read; `J2GO_UNCHECKED_CASTS=file:file` names other lists, as the
  self-test does). A collection made for a typed place holds its elements at their Go type and
  refuses an element of another type when it is added, where Java's erased collection takes
  it and fails at a later read; an unchecked cast is where such an element could come from.
  A cast to `List<?>` claims nothing and passes;
- `instanceof` of, or a cast to, one of the collection classes that the typed collections stand
  for (`ArrayList`, `HashSet`, `LinkedHashSet`, `HashMap`, `LinkedHashMap`, `TreeSet`, `TreeMap`,
  `ArrayDeque`: `LibGen.CONCRETE`): a collection made for a typed place is the typed
  collection's view, no object of that class. The tree has none. (The immutable classes are
  not refused: erased code sees an immutable typed collection as its twin, Guava's object.)

- an unchecked cast to a use of a class of `LibGen.CLASSES_T2` whose kind is on
  (`(ImmutableSortedSet<String>) sortedOfAny`), for the reason above (tools/j2go/unchecked-casts.txt
  applies too; the tree has none);
- `instanceof` of, a cast to, a method declared only by, or a library parameter of a class
  whose typed places the makers of `LibGen.MAKERS_T2` serve (`EnumSet`, kind `enumset`);
- a method that a class of `LibGen.CONCRETE` declares itself (no method of an interface or of
  `Object`: `ArrayList.ensureCapacity`), called on a receiver of the class with type arguments,
  unless `LibGen.OWN` lists it (piece `cls`; the tree has none).

- kind `goptional`: an unchecked cast to a Guava `Optional<X>` (the cast converts the content,
  `optGOptionalOf_`, where Java's checks the class only); `==` / `!=` between a Guava Optional
  and anything but `null` (a value has no identity); a call of, or a reference to, a method of
  Guava's Optional that `LibGen.GOPT_METHODS` does not list (the third overload of `or`,
  `or(Supplier)`, which the runtime has in neither form; `asSet`, `toJavaUtil` ...); a
  reference to one of its statics (`Optional::fromNullable`: a Go generic function, which a
  reference gives no type arguments). The tree has none.

`tools/j2go/selftest/refused/RefusedLib.java` has one use of each kind (15); the self-test
fails unless the translator refuses exactly those and writes nothing.

Each kind and piece is a switch: `J2GO_NOLIBGEN=iter,coll,list,set,map,makers,views,place,cov,elems,wide,nncoll,readers,cls,meth,fns,sorted,enumset,cmaps,deque,guava,goptional,seqcoll,objarg,cmp,ownwild,nbox,proto,owniter,hstatic,rtm,fns2,seqstr,objlist,freshloop,mkext,gsorted`
(`all`: every one). A translation with any of them off is built with `-tags j2go_nolibgen`
(`tools/prune/verify.sh` and the self-test do that when `J2GO_NOLIBGEN` is set): the erased
factories that only such a translation names are in `go/closure/rt_lg_erased.go`, outside the
default build, as `rt_fn_erased.go` is for rule "function types".

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
`*TList[X]` (`rt_tlist.go`: a slice of X values, not of boxes), one of type `Set<X>` /
`HashSet<X>` / `LinkedHashSet<X>` a typed set, `*TSet[X]` (`rt_tset.go`: the X values in
insertion order, a Go map from element to position once the set is not small), one of type
`Deque<X>` / `ArrayDeque<X>` / `Queue<X>` a typed deque, `*TDeque[X]` (`rt_tdeque.go`: a typed
list whose first item is the head, as the erased ArrayDeque is an ArrayList), and one of type
`Map<K, V>` / `HashMap<K, V>` / `LinkedHashMap<K, V>` a typed map, `*TMap[K, V]` (`rt_tmap.go`:
the keys and the values in insertion order, a Go map from key to position once the map is not
small), when the translator can see that it only ever holds collections the program makes for
it (`Colls`; `J2GO_COLLS=1` reports the counts and why declarations stay erased, `J2GO_COLLS=2`
each one). The decision is made declaration by declaration: a field, a local variable, and the
parameters and the result of a method whose signature is its own (a constructor, a static or
private method, an instance method that overrides nothing and that nothing overrides; not one
named by a method reference). A parameter or the result of the methods of a group of
overriding methods is one declaration (the methods have one signature in Go): it is typed in
every method of the group or in none. Such a group is every method connected by "overrides"
in some class of the program (a method that a class inherits is connected to the interface
method it implements there), all of them methods of the program without type parameters,
with the same collection type at that position; a group that a lambda or a method reference
implements keeps the erased signature. A declaration is typed when

- each type argument is a type that a typed collection holds (`Ctx.typedArg`): a String, a
  class, an interface or a handle without type arguments; a boxed primitive or `Object`, held
  as the box it is in erased code (`List<Integer>` is a `*TList[any]`, `Map<String, Integer>` a
  `*TMap[string, any]`); or a library type with type arguments, held at its erasure
  (`Map<String, List<Node>>` is a `*TMap[string, List]`, `List<List<String>>` a `*TList[List]`:
  the element is the erased `List`, as it is everywhere else); a generic class of the program,
  at the Go type it has (`List<DiGraphNode<N, E>>` in `DiGraph<N, E>` is a
  `*TList[DiGraph_DiGraphNode[N, E]]`; no wildcard among its type arguments); or a type
  variable: the Go type parameter where it is one (`List<T>` in a `Stack<T>` that is a Go
  generic type is a `*TList[T]`), else what it is erased to (its bound); a wildcard
  `? extends X` is X (`List<? extends JSType>` is a `*TList[*JSType]`: only a list made for a
  declaration with element type X enters it). The elements of a set
  and the keys of a map are compared: their type is not a type variable that is a Go type
  parameter (its constraint is `any`, and a typed set needs `comparable`), not a library type
  with type arguments, and not `Object`, `Float` or `Double`. Where Go's `==` is Java's `equals` for the type, the collection compares with
  `==` (a Go map from element to position once it is not small): a String, an enum, a class of
  the program (a handle class too) that inherits `equals` and `hashCode` from Object, as all
  its subclasses do (`Ctx.goEquals`), and a boxed primitive (two boxes of equal values of one
  type are `==` as Go interfaces; `Set<Character>` is a `*TSet[any]`). For any other class or
  interface (one with its own `equals`, a library class) the collection finds an element as
  `java.util.HashMap` does, by the hash code it had when it was added, then by identity or
  `equals` (`Ctx.byEquals`; the conversion `tce_p_JSType` says so; `teq_` in `rt_tset.go`).
  A declaration whose type arguments name a Go type parameter is typed only if every
  expression that reads it has the declaration's own type arguments: inside the generic class
  `items` is a `List<T>`, but `c.items` of a `Stack<String>`, or `items` in a subclass
  `Words extends Stack<String>`, is a `List<String>`, and such a use keeps it erased;
- every value that enters it is a new collection of its kind (`new ArrayList<>()`,
  `new ArrayList<>(n)`, `new ArrayList<>(collection)`; `new LinkedHashSet<>(..)`, the only set
  the program makes; `new ArrayDeque<>(..)`; `new LinkedHashMap<>()`, `(n)`, `(map)`, the only
  hash map the program makes), `null`, or the value of another typed declaration of the same
  kind and type arguments (an argument enters the parameter, a returned expression the result;
  `Collections.unmodifiableList(x)` / `Set` / `Map` is `x`, as the runtime's is). A declaration
  that nothing but `null` enters (a parameter of a method the program never calls, or calls
  with `null`) is typed too;
- every use is one the typed collection or its view serves: a `List` / `Set` / `Deque` / `Map`
  method, a for-each loop, `==`, a string concatenation, or a place of type `List` / `Set` /
  `Deque` / `Queue` / `Map`, `Collection`, `Iterable`, `Object` or a type variable (the result
  of a lambda is a place of the type that its interface's method returns for the lambda). A place
  that wants the class (`ArrayList`, `LinkedHashSet`, `ArrayDeque`, `LinkedHashMap`) must be a
  typed declaration itself; `subList`, `listIterator` and `getClass` keep the declaration
  erased.

`list.get(i)`, `add`, `set`, `remove(int)`, `size`, `isEmpty`, `clear`, `iterator`, a set's
`add`, `contains` and `remove` of an X (`s.contains(x)`, `s.remove(x)`), a deque's `push`, `pop`,
`poll`, `peek`, `peekLast`, `add` (and `addFirst`, `removeFirst`...), a map's `get`, `put`,
`containsKey`, `remove`, `getOrDefault`, `putIfAbsent` and `computeIfAbsent` with a key that is
a K (`m.containsKey(k)`, `m.remove(k)`, `m.getOrDefault(k, v)`), and the for-each loop (`it := list.each()`) work
on the X values. A for-each loop over `m.keySet()`, `m.values()` or `m.entrySet()` of a typed
map reads the map itself (`it := m.eachKey()`, `m.eachValue()`, `m.eachEntry()`); the variable
of the loop over the entries is a `*TEntry[K, V]` (`getKey`, `getValue`, `setValue` on the K
and V values; an entry that goes anywhere else goes as Java's `Map.Entry` through
`e.asEntry()`). Any other method, and every place that is not a typed declaration (a wildcard
or raw type, `Object`, a library parameter, a method of a group of overriding methods), sees
the collection as Java's `List` / `Set` / `Deque` / `Map` through `list.asList()` /
`set.asSet()` / `deque.asDeque()` / `m.asMap()`. That is the collection's one erased view, a
field of the collection itself (so a collection that erased code sees is still one object, as
the Java collection is), which boxes and casts as the erased code did: the Java object stays one object
wherever it goes (`==`, changes made on either side), `null` is `nil` on both sides, and a
typed declaration never holds a collection that erased code made, so no conversion from the
erased form exists. Inside the translator a typed declaration, and each expression that reads
it, has the type `TList<X>` / `TSet<X>` / `TDeque<X>` / `TMap<K, V>` / `TEntry<K, V>` of a
class that exists for it alone (`Ctx.STUBS`: the typed methods), and `Ctx.coerce` turns that
type into any other through the view. The element conversions are generated per element type
(`tc_string`, `tc_p_JSType` in `g_0casts.go`) with the conversions the erased code makes for
that type; an element type that is a Go type parameter has the conversions of a value of a
type parameter (`tcT_[T]()`: `boxT_`, `unboxT_`), and an instantiation of a generic class that
names type parameters has a generic function with the conversions of the class
(`tc_p_DiGraph_DiGraphNode_N__E_[N, E]()`); both are one object per instantiation (`single_`). The runtime's erased code reads a view without an iterator object (`foreach_`,
`stepper_`) and looks a String up in it without a box (`contains_s`, `get_s`...).

Guava's immutable collections are typed collections too. A declaration of type
`ImmutableList<X>` / `ImmutableSet<X>` / `ImmutableMap<K, V>` is one of the list, set or map
kind, and holds the same Go type, `*TList[X]` / `*TSet[X]` / `*TMap[K, V]`, with its `twin` set
(`rt_timm.go`): a collection that never changes (every change throws
`UnsupportedOperationException`, as Guava's do). So a `List<X>` declaration that holds an
`ArrayList` on one path and `ImmutableList.of()` on another is one typed list, as it is one
`List` in Java. Erased code sees an immutable typed collection as its twin, the runtime's
`ImmutableList` / `ImmutableSet` / `ImmutableMap` with the same elements, boxed and made the
first time erased code needs it (`asImm()`, which `asList()` / `asSet()` / `asMap()` return for
an immutable collection; a place of type `ImmutableList` or `ImmutableCollection`, and a
method that javac finds in those classes, `reverse()`, `asList()`, `subList`, gets the twin
directly). The other way, an expression of the static type `ImmutableList<X>` enters a typed
declaration whatever made its value: a typed declaration gives the typed list itself, and for
an erased one (the result of a library method, of a method whose result is not typed, a cast)
`tlistImm_(tc, x)` / `tsetImm_` / `tmapImm_` gives the typed collection of that object, made
once and remembered by the erased object (`typed_`). A Java object is thus one typed object
and one erased object, each found from the other: `==` and "`copyOf` of an immutable collection
is the collection" hold on both sides, and because the contents never change the two cannot
be told apart. Such an expression therefore erases nothing and nothing erases it (no
"exchanges values" between an `ImmutableList` declaration and its sources). Where the typed
declaration is the target, `ImmutableList.of(a, b)` is the typed factory
(`immutableListOf_[X](tc, a, b)`, `immutableSetOf_`, `immutableMapOf2_(kc, vc, k1, v1, k2, v2)`;
`tlistEmptyImm_(tc)` for `of()`: one empty collection per element type, whose twin is the
runtime's one empty collection) and `ImmutableList.copyOf(c)` is `immutableListCopyOfT_` of a
typed collection of the same type (an immutable one is itself), `immutableListCopyOfArr_` of an
array, or `immutableListCopyOf_` of an erased collection. Guava's semantics are the runtime's
(`rt_guava.go`): insertion order, `NullPointerException` for null elements, keys and values
with Guava's messages, `IllegalArgumentException` for a key given twice.

Their builders are typed as well: a declaration of type `ImmutableList.Builder<X>` /
`ImmutableSet.Builder<X>` / `ImmutableMap.Builder<K, V>` holds a `*TListBuilder[X]` /
`*TSetBuilder[X]` / `*TMapBuilder[K, V]` (the elements as they are; `build()` is the typed
immutable collection) when every value that enters it is a new builder
(`ImmutableList.builder()`, `builderWithExpectedSize(n)`, `new ImmutableList.Builder<>()`) or
that of another typed builder declaration, and every use is `add`, `addAll`, `put`, `putAll`
(whose result is the builder again), `build`, `buildOrThrow` or `buildKeepingLast`, or a
place that is a typed builder declaration: a builder has no erased view.
`J2GO_COLLS_NOIMM=1` and `J2GO_COLLS_NOBUILDERS=1` are the rules without them.

A builder that is used where it is made, a *builder chain*
(`ImmutableMap.<String, Annotation>builder().put("a", A).put("b", B).buildOrThrow()`,
`new ImmutableSet.Builder<Var>().addAll(xs).addAll(ys).build()`), is a declaration without a
name (`Colls.chains`; `J2GO_COLLS_NOCHAINS=1` is the translation without the rule). Its type
arguments are the ones javac gives the expression that makes the builder, its uses are the
calls of the chain. It is a typed builder (`newTMapBuilder_(tc_string,
tc_p_Annotation).put("a", A).put("b", B).buildOrThrow()`, the keys and values as they are) when
every call of the chain is a typed method of the builder and the chain ends in a typed
declaration: the collection it builds enters a typed declaration of that kind and type
arguments (an initializer, an assignment, a returned value, an argument; also through `? :`),
or the builder itself enters a typed builder declaration
(`ImmutableList.Builder<X> b = ImmutableList.<X>builder().add(x)`, which was erased before:
only a new builder could enter a typed builder declaration). Without the rule such a chain
boxed every element into the erased builder, and `tmapImm_` then made the typed collection
from the erased one. A chain stays erased when its collection goes to an erased place (an
argument for a `Collection` parameter of a method group, the receiver of a call, a field of a
handle class): that place needs the boxed collection, and a typed builder would only add a
second one. It also stays erased when javac's types are not ones a typed collection holds
(`ImmutableMap.builder()` without type arguments in a chain is a `Builder<Object, Object>`; a
method's type variable) and when a call is not a typed builder method (`add(E...)`).
The same holds for `ImmutableList.of(a, b)` as an expression: where a typed declaration is the
target it is the typed factory already; the ones left are consumed by a place that takes
any collection (a `Collection` or `Iterable` parameter, a for-each loop, the copy constructor
of another collection).

Sorted collections are the same Go types in a sorted mode (`rt_tsorted.go`). A declaration
of type `TreeSet<X>` / `SortedSet<X>` / `NavigableSet<X>` is one of the set kind, one of type
`TreeMap<K, V>` / `SortedMap<K, V>` / `NavigableMap<K, V>` one of the map kind, and
`new TreeSet<>()`, `new TreeSet<>(comparator)`, `new TreeSet<>(collection)` (and the same of
`TreeMap`) make the typed set or map with its keys kept in the order of the comparator, or
their natural ordering, and found by binary search with it, as the runtime's erased `TreeSet`
finds its elements (`newTSortedSet_(tc)`, `newTSortedSetC_(tc, cmp)`, `tsortedSetFrom_`). So a
`Set<X>` that holds a `TreeSet` on one path and a `LinkedHashSet` on another is one `*TSet[X]`.
Natural ordering compares two Strings as they are; every other comparison is made on the
boxes (the comparator is erased code). Erased code sees a sorted collection through its
sorted view (the view of the hash mode with `tailSet` and `descendingSet`, and the class name
`TreeSet`), which a place of type `SortedSet` / `NavigableSet` gets through `asSorted()`; a
place of the class `TreeSet` itself must be a typed declaration. An `ImmutableSortedSet` /
`ImmutableSortedMap` that reaches a declaration of type `ImmutableSet` / `ImmutableMap` is the
typed collection in its sorted mode with its comparator. `J2GO_COLLS_NOSORTED=1` is the rule
without them.

Two differences from the JVM are accepted. An element of the wrong type that unchecked code
puts into a typed collection through its view fails there with the `ClassCastException` that
Java throws at the first typed read (or never, if the element is never read at its type). And
the view is not the Go type `*ArrayList` / `*LinkedHashSet` / `*ArrayDeque` / `*LinkedHashMap`:
its `getClass()` is the Java class, but `x instanceof ArrayList` or a cast to `ArrayList` of a
view that reached erased code would fail (the program has neither; a declaration of the class
type that a typed collection reaches is typed too, or the collection stays erased).
`J2GO_ERASED=1` skips the rule.

**Conversions without a box** (`Conv.java`). The erased translation converts between two Go
types through Object: `icast_I(obj_(v))`. Where the Java conversion needs no check, these rules
keep the value at its static type. Each has a switch `J2GO_NO<RULE>=1` that gives the previous
translation, for bisecting. `J2GO_BOXES=1` reports the conversions to `Object` that the
translation still makes: how many by kind of value (`boxes: kind 1119 String`), and for
arguments by callee, parameter and kind (`boxes: arg 81 String.format [varargs] <- primitive`).

- *Static upcast* (`Conv.upcast`, `J2GO_NOUPCAST`). A class without subclasses becomes an
  interface that it implements as Go converts a pointer to an interface. `this` and a new
  object are never null: `NodeTraversal_Callback(this)`, `List(new_ArrayList_0())`. Any other
  value goes through a generated function that keeps null as the nil interface (a nil pointer
  converted by Go would be an interface that is not nil), one per pair of types in
  `g_0casts.go`:

  ```go
  func Compiler_as_SourceExcerptProvider(v *Compiler) SourceExcerptProvider {
  	if v == nil {
  		return nil
  	}
  	return v
  }
  ```

  The interface value is the one the type assertion made: the pointer of a class without
  subclasses is the object itself. A class of the program must have every method of the Go
  interface (`Ctx.implementsInGo`); for a class of the runtime (`*ArrayList`, `*ImmutableList`,
  the view of a typed collection) the Go compiler checks the conversion. What stays
  `icast_I(v.self())`: a class with subclasses (its value may be a part of a larger object),
  a program class that lacks a method of the Go interface (a method implementing the methods
  of two interfaces is in one group only), and values that are not statically subtypes (an
  erased result).

- *Typed non-null checks* (`Conv.nonNull`, `J2GO_NONONNULL`). `<T> T checkNotNull(T, ...)` of
  Guava's `Preconditions` and `<T> T requireNonNull(T, ...)` of `java.util.Objects` return
  their first argument. The call is a Go generic function of the argument's static type
  (`rt_conv.go`), which tests the value against the null of that type and returns it:
  `this.compiler_ = Preconditions_checkNotNull_T(compiler)` where the erased translation was
  `cast_[*Compiler](Preconditions_checkNotNull(obj_(compiler)))`. `_T` is for a class, an
  interface or a handle (null is nil, the zero value), `_Str` for a `String` (null is the null
  string). Arguments that are typed collections or value types (`Optional`) and boxed
  primitives keep the function taking an object.

- *Bare arguments* (`Conv.bare`, `J2GO_NOBARE`). An argument for an `Object` parameter (or an
  element of `Object...`) that cannot be null, a string literal or a new object of a class
  without subclasses, is passed as it is: `sb.append("x")`, `list.add(new_Entry(k, v))`. Go
  converts it to the interface value that `strobj_("x")` / `obj_(new_Entry(k, v))` return for
  a value that is not null. Only arguments: there the parameter gives the expression its type.

- *Message arguments* (`Conv.msgArg`, `Conv.handleMsg`, `J2GO_NOMSG`). An `Object` parameter
  after the first of `Preconditions.checkState` / `checkArgument` / `checkNotNull` is only
  formatted, and only when the check fails. A `String` or an object of a class is passed at
  its own type: `Preconditions_checkState_boolean_String_Object(ok, "bad type %s", t)`. The
  runtime makes it the object that the erased translation passed before it formats the
  message (`canon_` in `rt_conv.go`: null for the null string and for a nil pointer, the most
  derived object for a class with subclasses). A handle (a `Node`) as the one message
  argument calls a variant generic in the handle type, which boxes it when the check fails:
  `Preconditions_checkState_boolean_Object_H(n.isName(), n)`. Interfaces, primitives, typed
  collections and value types are converted to `Object` as everywhere else.

- *One Go type* (`Conv.sameGoType`, `J2GO_NOALIAS`). Two library classes that the runtime
  declares as one Go type (`type ImmutableList = ImmutableCollection`; the translator reads
  the alias declarations of `rt_*.go`) convert as the value itself: `this.names = b.build()`
  where the erased translation asserted the type to itself,
  `cast_[*ImmutableList](obj_(b.build()))` (it cannot fail and keeps nil). For the same
  reason such a class becomes an interface by the static upcast although the declared
  result is not a subtype in Java (`Builder.build()` is declared `*ImmutableCollection` in
  the runtime; as a `List` it is `ImmutableCollection_as_List(b.build())`).

- *Typed Object methods* (`Conv.typedEquals` / `typedHash` / `typedStr`, `J2GO_NOTYPEDOBJ`).
  The members that the translator generates for a record, and string concatenation, apply
  `Objects.equals`, `Objects.hashCode` and `String.valueOf` to values whose static type is
  known. They are written at that type:

  ```go
  func (this *Property) equals(p0 any) bool {
  	o, ok := p0.(*Property)
  	if !ok || o == nil {
  		return false
  	}
  	return true && this.name_ == o.name_ && this.kind_ == o.kind_ && equals_(this.owner_, o.owner_)
  }
  ```

  Two values of an enum, or of a class of the program without subclasses that has no
  `equals` but `Object`'s, are compared with `==` (what `Objects.equals` does for them).
  Otherwise a pointer to a class without subclasses or an interface goes to the generic
  runtime functions `equals_(a, b)`, `hash_(v)`, `tostr_(v)` (`rt_conv.go`; null is the zero
  value), a handle to `equalsH_` / `hashH_` / `tostrH_`. In a record's `hashCode` a `String`
  is `strhash_(s)` (0 for null), an `int` is itself, the other primitives are `boolhash_`,
  `longhash_`, `dblhash_`; in `toString` they are converted as string concatenation converts
  them. `"" + v` is `tostr_(v)` when `v` has its type by declaration (a variable, a field or
  a method's result of that erased type, a new object); a value that would be cast to its
  static type first (the result of an erased generic) is still converted as an Object, without
  the cast. What keeps the conversion to Object: a class with subclasses (`objstr_(v.self())`:
  the most derived object decides), typed collections (their view), value types.

- *Typed Object calls* (`ExprEmitter.typedObjCall` / `typedObjStatic`, `J2GO_NOOBJCALL`). The
  same for the calls that the program writes. `x.equals(y)`, `x.hashCode()` and `x.toString()`
  where the method javac finds is `Object`'s (the receiver's class, a library class, an enum
  or a class of the program, has no Go method of its own for it) were calls of a runtime
  function on boxes: `Object_toString(obj_(sb))`, `Object_equals(obj_(mode), obj_(Mode_NONE))`.
  A receiver that is a pointer to a class without subclasses, an interface or a handle now
  stays at its static type: `tostrR_(sb)`, `hashR_(v)`, `equalsR_(x, y)` (`rt_conv.go`;
  `tostrRH_`... for a handle) make it the object that the erased translation passed and call
  `Object`'s method, so a null receiver throws `NullPointerException` as before. An argument
  of `equals` that has the receiver's Go type by its declaration (as for string concatenation
  above: a variable, a field, a method's result, a new object) stays typed too,
  `equalsRR_(a, b)`; where `equals` of that type is identity (an enum, a class of the program
  without subclasses that has only `Object`'s `equals`) the call is `eqR_(mode, Mode_NONE)`:
  a test of the receiver for null and `==`. Any other argument is converted to `Object` as
  before (it is not cast to its static type first: `equals` takes an `Object`).
  `Objects.equals(a, b)` of two values that have one type by their declarations is `a == b`
  for two Strings and for a type whose `equals` is identity, else `equals_(a, b)` /
  `equalsH_`; `Objects.hashCode(v)` is `strhash_(v)` / `hash_(v)` / `hashH_(v)` and
  `String.valueOf(v)` of an object `tostr_(v)` / `tostrH_(v)`. What keeps the conversion to
  `Object`: a receiver or value of a class with subclasses (`v.self()`), a typed collection
  (its view), a value type (`Optional`: `box_()`), and `Objects.hash(...)`.

- *Identity functions* (`ExprEmitter.identityFn` / `identityCall`, `J2GO_NOIDENT`). A static
  method of the program whose whole body is `return a == b` on its two `Object` parameters
  (`JSCompObjects.identical`, which the compiler calls where a style check forbids `==`) is
  Java's `==`, and a call of it is translated as that comparison of its two arguments:
  `(joinedType == slotA.getType())` where the erased translation was
  `JSCompObjects_identical(joinedType.self(), slotA.getType().self())`. `==` keeps two operands
  of one Go type at that type (two pointers to a class, with or without subclasses, two
  handles, two values of one interface, two Strings), widens a class to its superclass, and
  compares the boxes of anything else, as the call did. What keeps the conversions to
  `Object`: an operand that does not have its static type by declaration (the result of an
  erased generic would be cast to it by `==`; the function never casts: `(asAny == asAny)`,
  still without the call). What keeps the call: a primitive argument (the function compares
  its box) and a value type (`Optional`).

- *Implicit conversions* (`Conv.implicit`, `J2GO_NOIMPLICIT`). A primitive, an interface value
  or an array that becomes an `Object` is `any(int32(n))`, `any(scope)`, `any(arr)` in the
  erased translation: a Go conversion, with no test and no call. Where Go declares the place
  `any` it is not written, since Go makes the same conversion there: an argument for a
  parameter of type `any` (`map.put(name, int32(n))`, `Objects_requireNonNull(nodes.asList(),
  "nodes")`), an element of a `[]any` built for `Object...`
  (`String_format("%d of %s", []any{int32(i), strobj_(s)})`), the collection of a for-each
  loop (`foreach_(scope.getVarIterable())`), an operand of a string concatenation
  (`objstr_(list.asList())`), and the value that a function with result `any` returns. This
  is notation only: the compiled program is the same. A parameter is known to be `any` for a
  method or function of the program (its signature is the translator's; type arguments are
  written at every call), for a method of a library type (a Go method has no type parameters
  of its own) and for a library function that is not generic in the runtime (the translator
  reads the names of the generic functions of `rt_*.go`; for such a function Go would infer a
  type argument from the argument, so its arguments keep their conversions). What keeps
  `any(x)`: `v := any(x)`, comparisons (`any(a) == b`), assignments, the argument of a cast
  (`icast_I(any(this))`), and the conversions that are not Go's: `obj_`, `strobj_`, `box_()`
  (null for a nil pointer, the null string, handle 0), `boxT_`.

- *String for CharSequence* (`Conv.strSeq`, `J2GO_NOSTRSEQ`; `J2GO_STRSEQ=1` reports the calls
  left). `CharSequence` is `any` in Go, so a `String` passed for one is boxed, and a string in
  an interface is an allocation. A call of a library method with a `String` for each of its
  `CharSequence` parameters calls a variant that takes Go strings, where the runtime has one
  (`Conv.STR_SEQ`; suffix `_s`, in `rt_conv.go` or next to the function it varies):
  `String_contains_s(name, prefix)`, `String_replace_s`, `String_join_s`,
  `pattern.matcher_s(line)`, `matcher.reset_s`, `Pattern_matches_s`, `splitter.split_s` /
  `splitToList_s`, `charMatcher.matchesAllOf_s`, `CharMatcher_anyOf_s`. The variant does what
  the method does with the text of the sequence; the null string is the
  `NullPointerException` of a null sequence. `Appendable.append(CharSequence)` is a method
  that classes of the program implement too, so its variant is a function taking the receiver
  first: `Appendable_append_s(out, s)` appends the string to a `StringBuilder` (null appends
  "null") and gives any other `Appendable` the boxed sequence. A method of the *program* that
  declares a `CharSequence` keeps the box: it tests the sequence for null.

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

Every per-object array of a handle class (its records and columns) is registered with its
element type in `handleColumns_` (`rt_handles.go`). `Node`'s are a 16-byte `Node_ns_`, a
40-byte `Node_cs_` and the 4-byte `payload` column: 60 bytes per node and no Go pointer, which
`TestNodeColumnsHoldNoPointers` (`rt_handles_test.go`) asserts by reflection over the
registered types, so that a later rule cannot bring a pointer back unnoticed.

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

- **Field layout.** The fields of a class's struct are written in the order of their Go
  alignment, widest first, and in Java's order among fields of one alignment
  (`Ctx.layoutRank`, `Emitter.structDecl`): 8 bytes (pointers, interfaces, strings, slices,
  `int64`, `float64`), then the fields of a Go type parameter and the runtime's value types
  holding one (`Optional[T]`: 8 bytes for a pointer, 4 for a handle, so this place leaves no
  hole either way), then 4 bytes (`int32`, `float32`, handles), 2, 1. Go lays a struct out as
  it is written and pads each field to its alignment, so a `boolean` between two references
  costs 8 bytes; in this order a struct has no padding but at its end. The embedded superclass
  struct stays first, then `self_`, the dispatch table, the enclosing instance and the captured
  variables with the declared fields, each by its alignment. Only the declaration changes:
  every use names the field (the translation writes no positional composite literal of a class
  struct, compares no struct value and converts none; the runtime finds `self_` and an embedded
  superclass by name or by type, `Object_clone` and `embeddedPtr_`), and field initializers
  and constructors run in Java's order. The records and columns of a handle class have their
  own layout (above). `J2GO_NOLAYOUT=1` writes Java's order. Padding over the 2,172 generated
  structs 3,639 -> 2,415 bytes (what is left is at the end of a struct); 115 structs are
  smaller, 77 of them by an allocation size class (`Var` 72 -> 64 bytes, so 80 -> 64 allocated;
  `HamtPMap` 56 -> 48, so 64 -> 48).
  Packing the `boolean` fields of a class into one flag word was measured on top of this and
  not done: with the booleans next to each other at the end of the struct, bits instead of
  bytes change the size of 17 of the 144 structs with two or more booleans and the allocation
  size class of 4 (`CompilerOptions`, `Compiler`, `InlineFunctions.FunctionState`,
  `RemoveUnusedCode.Builder`), about 0.26 MB of the 2,250 MB the bundle workload allocates.
- **Report: value candidates** (`J2GO_VALUES=1`, `ValueScan`; no rule uses it). Lists the
  classes that could be Go values instead of heap objects, a class or record without
  superclass or subclasses, not inner, with only final fields and none of its own type (428),
  with the size of its struct, whether it has its own `equals`, and counts of what a rule
  would have to exclude or handle: `==` between two of them, comparisons with `null`, `null`
  given where the class is expected, places where one becomes an `Object` or an interface (a
  box in Go), mentions as a type argument, `instanceof` and casts, `synchronized`, identity
  collections, `this` escaping the constructor, fields and arrays that start as `null`.

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
