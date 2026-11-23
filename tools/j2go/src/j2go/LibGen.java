package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.TreePath;
import com.sun.source.util.TreePathScanner;
import com.sun.source.util.Trees;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Rule "typed library interfaces" (J2GO_NOLIBGEN=kind,... turns kinds off; `all` every kind): a
 * place whose Java type is a library collection interface with type arguments holds the typed
 * interface of the runtime (rt_lg.go), which is the erased interface plus the methods that give
 * or take elements, at their Java type and under the name with the suffix _t, as javac's class
 * files have a method and its bridge:
 *
 * <pre>
 *   type IteratorT[T any] interface { Iterator; next_t() T }
 *   type IterableT[T any] interface { Iterable; iterator_t() IteratorT[T] }
 * </pre>
 *
 * Kinds: `iter` (java.util.Iterator, java.lang.Iterable), `coll` (java.util.Collection), `list`
 * (java.util.List), `set` (java.util.Set), `map` (java.util.Map, java.util.Map.Entry), `deque`
 * (java.util.Queue, java.util.Deque: rule candidate T2), `guava` (PeekingIterator, LoadingCache: T2).
 *
 * - Types: the interfaces of an enabled kind are typed declarations (Ctx.typedClasses).
 *   `Iterator<Node>` is `IteratorT[Node]`. A wildcard is its bound, an erased type variable what
 *   the erased code has for it; a use whose type arguments are all `any` in Go (`Iterable<?>`,
 *   `Iterator<Object>`, `Iterable<Integer>`) is the erased interface, as without the rule.
 * - A call of `next` / `iterator` on a typed receiver is the call of `next_t` / `iterator_t`,
 *   whose result needs no cast; a for-each loop over a typed Iterable runs on a typed cursor
 *   (eachOf_).
 * - A class of the program that implements the interface directly and has no subclass has the
 *   method with its Java types (`next_t() Node`) and the bridge with the erased name and
 *   signature, so its objects are values of both interfaces (native). The typed collections of
 *   the runtime are native too (TList[T] is an IterableT[T]).
 * - Typed to erased: the same object. Where it may be a wrapper or a typed collection, the
 *   object that erased code knows is taken out (X_un_ for the erased interface, lgBox_ for
 *   Object), so identity, instanceof and the runtime's own type tests see what they saw.
 * - Erased to typed (the result of an erased method, a declaration that stays erased, an
 *   Object): xAs_(tc, v), which gives v itself where its object is native at that type and else
 *   wraps it; the wrapper unboxes each element as the erased code cast it after the call. An
 *   unchecked cast therefore behaves as in Java: nothing is checked until an element is read.
 * - Two different instantiations meet through the erased form.
 *
 * J2GO_LIBGEN=1 reports what the rule converts and what it leaves erased (=2: by type).
 */
final class LibGen {
  static final Map<String, List<String>> KINDS = new LinkedHashMap<>();

  static {
    KINDS.put("iter", List.of("java.util.Iterator", "java.lang.Iterable"));
    KINDS.put("coll", List.of("java.util.Collection"));
    KINDS.put("list", List.of("java.util.List"));
    KINDS.put("set", List.of("java.util.Set"));
    KINDS.put("map", List.of("java.util.Map", "java.util.Map.Entry"));
    // (rule candidate T2: Queue and Deque, with their typed methods in rt_lg_t2.go)
    KINDS.put("deque", List.of("java.util.Queue", "java.util.Deque"));
    // (rule candidate T2: two Guava interfaces whose values the runtime makes erased)
    KINDS.put("guava", List.of("com.google.common.collect.PeekingIterator", "com.google.common.cache.LoadingCache"));
  }

  /**
   * The methods of the interfaces of the collection kinds that have a typed form (name/arity);
   * every other method of theirs keeps the erased signature of the runtime, which the typed
   * interface embeds. (Of the kind `iter`, every method whose signature names the type
   * parameter has one.) Collection has none of its own: it is a typed Iterable.
   */
  static final Map<String, Set<String>> TYPED_METHODS = Map.of(
      "java.util.Collection", Set.of(),
      "java.util.List", Set.of("get/1"),
      "java.util.Set", Set.of(),
      "java.util.Map", Set.of("get/1", "keySet/0", "values/0", "entrySet/0"),
      "java.util.Map.Entry", Set.of("getKey/0", "getValue/0"),
      "java.util.Queue", Set.of("peek/0", "poll/0", "remove/0"),
      "java.util.Deque", Set.of("pop/0", "push/1"),
      "com.google.common.collect.PeekingIterator", Set.of("peek/0"),
      "com.google.common.cache.LoadingCache", Set.of("getUnchecked/1", "get/1"));

  /**
   * Piece `meth` (J2GO_NOLIBGEN=meth is the translation without it): the methods that take or
   * give an element and have a typed form in addition to TYPED_METHODS: the argument enters at
   * its type (no box of a String or of a value of a type parameter), the result needs no cast.
   * (`List.remove/1` is remove(int); methods whose parameters are all Object, as contains and
   * containsKey, have nothing to type.)
   */
  static final Map<String, Set<String>> TYPED_METHODS2 = Map.of(
      "java.util.Collection", Set.of("add/1"),
      "java.util.List", Set.of("set/2", "remove/1"),
      "java.util.Map", Set.of("put/2", "remove/1"),
      "java.util.Map.Entry", Set.of("setValue/1"));

  /**
   * Piece `cls` (J2GO_NOLIBGEN=cls is the translation without it): a place whose Java type is
   * one of the collection classes that the typed collections stand for (the key), with type
   * arguments, holds the typed interface of the class's collection interface (the value):
   * `ImmutableList<X>` and `ArrayList<X>` are `ListT[X]`. What enters is typed by the place as
   * for the interface (`new ArrayList<>()`, `ImmutableList.of(..)`: the typed collection's
   * view). The class's own methods (ImmutableList.reverse) are called on the object erased code
   * knows (the twin of an immutable collection); the signatures of the runtime keep the class.
   */
  static final Map<String, String> CLASSES = Map.ofEntries(
      Map.entry("java.util.ArrayList", "java.util.List"),
      Map.entry("java.util.LinkedHashSet", "java.util.Set"),
      Map.entry("java.util.HashSet", "java.util.Set"),
      Map.entry("java.util.LinkedHashMap", "java.util.Map"),
      Map.entry("java.util.HashMap", "java.util.Map"),
      Map.entry("com.google.common.collect.ImmutableList", "java.util.List"),
      Map.entry("com.google.common.collect.ImmutableSet", "java.util.Set"),
      Map.entry("com.google.common.collect.ImmutableCollection", "java.util.Collection"),
      Map.entry("com.google.common.collect.ImmutableMap", "java.util.Map"));

  /**
   * Rule candidate T2 (docs/javaisms6): more library collection types whose places hold the typed
   * interface of their collection interface, as piece `cls` does for CLASSES, each kind behind
   * its own switch (the value: the interface, then the kind; J2GO_NOLIBGEN=kind is the
   * translation without it). The class's own methods are called on the object erased code knows.
   */
  static final Map<String, String[]> CLASSES_T2 = Map.ofEntries(
      Map.entry("com.google.common.collect.ImmutableSortedSet", new String[] {"java.util.Set", "sorted"}),
      Map.entry("com.google.common.collect.ImmutableSortedMap", new String[] {"java.util.Map", "sorted"}),
      Map.entry("java.util.SortedSet", new String[] {"java.util.Set", "sorted"}),
      Map.entry("java.util.EnumSet", new String[] {"java.util.Set", "enumset"}),
      Map.entry("java.util.concurrent.ConcurrentHashMap", new String[] {"java.util.Map", "cmaps"}),
      Map.entry("java.util.concurrent.ConcurrentMap", new String[] {"java.util.Map", "cmaps"}),
      Map.entry("java.util.IdentityHashMap", new String[] {"java.util.Map", "cmaps"}),
      Map.entry("java.util.EnumMap", new String[] {"java.util.Map", "cmaps"}),
      Map.entry("java.util.PriorityQueue", new String[] {"java.util.Queue", "deque"}));

  /**
   * The typed makers of the kinds of CLASSES_T2 (rt_lg_t2.go): a library function ("class.method"
   * with the erasures of its parameters) that makes a fresh collection, for a place of the typed
   * interface: the runtime's function that makes the typed collection of the same kind (the
   * value: the function, the collection kind of Ctx, the kind of CLASSES_T2). Another overload
   * (`EnumSet.of(e1, e2, e3)`) makes the erased object, which enters through a wrapper; one with
   * a parameter of the class itself (`EnumSet.copyOf(EnumSet)`, `complementOf`) is refused.
   */
  static final Map<String, String[]> MAKERS_T2 = Map.ofEntries(
      Map.entry("java.util.EnumSet.noneOf(java.lang.Class)", new String[] {"tenumSetNoneOf_", "set", "enumset"}),
      Map.entry("java.util.EnumSet.allOf(java.lang.Class)", new String[] {"tenumSetAllOf_", "set", "enumset"}),
      Map.entry("java.util.EnumSet.of(java.lang.Enum)", new String[] {"tenumSetOf_", "set", "enumset"}),
      Map.entry("java.util.EnumSet.of(java.lang.Enum,java.lang.Enum[])", new String[] {"tenumSetOfArr_", "set", "enumset"}),
      Map.entry("java.util.EnumSet.copyOf(java.util.Collection)", new String[] {"tenumSetCopyOf_", "set", "enumset"}));

  /** The class whose places the makers of a kind of MAKERS_T2 serve: a collection made for such a place is no object of the class. */
  static final Map<String, String> MADE_T2 = Map.of("enumset", "java.util.EnumSet");

  /** Whether the places of class qn (its kind on) may hold a collection that MAKERS_T2 made. */
  static boolean madeT2(String qn) {
    for (var e : MADE_T2.entrySet()) if (on(e.getKey()) && qn.equals(e.getValue())) return true;
    return false;
  }

  /** The entry of MAKERS_T2 for library function m (its kind on), in the form of Ctx.TYPED_MAKERS, or null. */
  static String[] t2Maker(ExecutableElement m) {
    String[] e = MAKERS_T2.get(sigKey(m));
    return e != null && on(e[2]) ? new String[] {e[0], e[1], null} : null;
  }

  /** "class.method(erasures of the parameters)" of library method m. */
  static String sigKey(ExecutableElement m) {
    StringBuilder k = new StringBuilder(((TypeElement) m.getEnclosingElement()).getQualifiedName() + "." + m.getSimpleName() + "(");
    for (VariableElement p : m.getParameters()) {
      TypeMirror t = p.asType();
      k.append(k.charAt(k.length() - 1) == '(' ? "" : ",").append(erasureName(t));
    }
    return k.append(")").toString();
  }

  private static String erasureName(TypeMirror t) {
    if (t.getKind() == TypeKind.ARRAY) return erasureName(((ArrayType) t).getComponentType()) + "[]";
    if (t.getKind() == TypeKind.TYPEVAR) return erasureName(((TypeVariable) t).getUpperBound());
    if (t.getKind() == TypeKind.DECLARED) return Ctx.typeElement(t).getQualifiedName().toString();
    return t.toString();
  }

  /** The collection kind (Ctx) of the typed collection that a maker's call of type t makes for a typed place (ExprEmitter.lgMade), or null. */
  Ctx.CollKind t2Kind(TypeMirror t) {
    if (!t2(t) || !madeT2(Ctx.typeElement(t).getQualifiedName().toString())) return null;
    DeclaredType v = cx.superView(t, cx.elems.getTypeElement(CLASSES_T2.get(Ctx.typeElement(t).getQualifiedName().toString())[0]));
    return v == null ? null : cx.collKindOf(v);
  }

  /** The collection interface whose typed interface a place of class qn holds (CLASSES, CLASSES_T2 where its kind is on), or null. */
  static String clsIface(String qn) {
    String iq = CLASSES.get(qn);
    if (iq != null) return iq;
    String[] e = CLASSES_T2.get(qn);
    return e != null && on(e[1]) ? e[0] : null;
  }

  /** Whether t is a use of a class of CLASSES_T2 whose kind is on. */
  static boolean t2(TypeMirror t) {
    if (t == null || t.getKind() != TypeKind.DECLARED) return false;
    String[] e = CLASSES_T2.get(Ctx.typeElement(t).getQualifiedName().toString());
    return e != null && on(e[1]);
  }

  /** Above 0 while a type is erased as the runtime declares it (rt): a collection class stays the class. */
  int rtDepth;
  /** Above 0 while cls decides a type: Go type names are given in the order of their first use, so none may be asked for. */
  int deciding;

  /**
   * Ctx.erase of a use t of a class of CLASSES (piece `cls`), or null where the class itself is
   * the type: the typed interface where the interface's use is typed and the typed family can
   * hold the elements (so that what is made for the place is typed: Ctx.typedArgs). A use of a
   * class of CONCRETE that stays erased (`ArrayList<?>`) is the erased interface: a collection
   * made for a typed place of the class is no object of the class.
   */
  TypeMirror cls(TypeMirror t) {
    if (rtDepth > 0 || !on("cls") || libs.isEmpty() || t.getKind() != TypeKind.DECLARED) return null;
    DeclaredType d = (DeclaredType) t;
    if (d.getTypeArguments().isEmpty()) return null;
    String qn = Ctx.typeElement(t).getQualifiedName().toString();
    String iq = clsIface(qn);
    if (iq == null) return null;
    TypeElement lib = cx.elems.getTypeElement(iq);
    if (!isLib(lib)) return null;
    DeclaredType v = cx.superView(t, lib);
    TypeMirror r = null;
    if (v != null) {
      TypeMirror n = erase(v);
      Ctx.CollKind ck = cx.collKindOf(t);
      if (qn.endsWith(".ImmutableCollection")) ck = cx.collKindOf(cx.types.getDeclaredType(cx.elems.getTypeElement("java.util.List"), d.getTypeArguments().toArray(new TypeMirror[0])));
      // (a class of CLASSES_T2: whether the typed family can hold the elements of its interface's use)
      if (!CLASSES.containsKey(qn)) ck = cx.collKindOf(v);
      deciding++;
      boolean holds = ck != null && cx.typedArgs(ck, t) != null;
      deciding--;
      if (is(n) && holds) r = n;
      else if (CONCRETE.contains(qn) && wideUse(d)) r = cx.types.erasure(v);
    }
    return r;
  }

  /** Whether use d has a wildcard or an erased type variable among its type arguments (a collection of any element type enters it). */
  private boolean wideUse(DeclaredType d) {
    for (TypeMirror a : d.getTypeArguments()) {
      if (a.getKind() == TypeKind.WILDCARD || FnTypes.captured(a) || a.getKind() == TypeKind.TYPEVAR && !cx.typedVar(a)) return true;
    }
    return false;
  }

  /**
   * Whether method e, called on a receiver of collection class type site (isCls), is a method
   * that the place's interface has: its root is declared by the interface or by one above it
   * (or by Object).
   */
  boolean served(ExecutableElement e, TypeMirror site) {
    TypeElement ro = cx.ownerOf(cx.root(e));
    // (a class of CLASSES_T2: a method of another interface, NavigableSet.descendingSet, is the class's own)
    if (site.getKind() == TypeKind.DECLARED && CLASSES_T2.containsKey(Ctx.typeElement(site).getQualifiedName().toString()) && !ro.getQualifiedName().contentEquals("java.lang.Object")) {
      TypeMirror c = cls(site);
      return c != null && cx.types.isSubtype(cx.types.erasure(c), cx.types.erasure(ro.asType()));
    }
    // (a method of Object; a method of another interface, SequencedCollection.getFirst: the object is converted as any value of the place's interface)
    if (ro.getQualifiedName().contentEquals("java.lang.Object") || ro.getKind().isInterface() && !isLib(ro)) return true;
    TypeMirror c = cls(site);
    return c != null && cx.types.isSubtype(cx.types.erasure(c), cx.types.erasure(ro.asType()));
  }

  /**
   * The methods that a class of CONCRETE declares itself (no method of an interface) and that
   * the typed collection's view has too, each with the Go interface of that one method: a call
   * on a place of the class goes through it, on the object erased code knows (an ArrayList, or
   * the view of a list made for a typed place). Any other such method is refused (check).
   */
  static final Map<String, String> OWN = Map.of("java.util.ArrayList.trimToSize", "interface{ trimToSize() }");

  /** The entry of OWN for method e called on a receiver of collection class type site, or null. */
  String own(ExecutableElement e, TypeMirror site) {
    TypeElement o = cx.ownerOf(cx.root(e));
    return CONCRETE.contains(Ctx.typeElement(site).getQualifiedName().toString()) ? OWN.get(o.getQualifiedName() + "." + e.getSimpleName()) : null;
  }

  /** Whether t is a use of a class of CONCRETE: a collection made typed for it is its own view for erased code too (no twin). */
  boolean concrete(TypeMirror t) {
    return t != null && t.getKind() == TypeKind.DECLARED && CONCRETE.contains(Ctx.typeElement(t).getQualifiedName().toString());
  }

  /** Whether Java type t is a use of a collection class whose place holds an interface (cls). */
  boolean isCls(TypeMirror t) {
    return t != null && t.getKind() == TypeKind.DECLARED && cls(t) != null;
  }

  /** The Go-level type of an expression that makes or gives an object of the class of t itself (`new ArrayList<>()`, the result of a runtime function): the erased class. */
  TypeMirror made(TypeMirror t) {
    return isCls(t) ? cx.types.erasure(t) : cx.erase(t);
  }

  /** Whether root method r of one of the interfaces has its typed signature on a typed receiver. */
  boolean typedMethod(ExecutableElement r) {
    String oq = cx.ownerOf(r).getQualifiedName().toString(), sig = r.getSimpleName() + "/" + r.getParameters().size();
    Set<String> ms = TYPED_METHODS.get(oq);
    if (ms != null && !ms.contains(sig) && !(on("meth") && TYPED_METHODS2.getOrDefault(oq, Set.of()).contains(sig))) return false;
    // (a result that is an interface of a kind which is off: the runtime's typed method gives the
    // typed interface, which the translation does not name, so the erased method serves the call)
    TypeMirror rt = r.getReturnType();
    return ms == null || rt.getKind() != TypeKind.DECLARED || kindOf(Ctx.typeElement(rt)) == null || isLib(Ctx.typeElement(rt));
  }

  /**
   * Library methods (of the root declaration) that only read some of their arguments: they
   * neither keep them, return them nor test their class. The value: the index of the first such
   * argument. (Piece `readers`, ExprEmitter.lgReaders.) Preconditions' checks format their
   * message arguments when they fail, and return only their first argument.
   */
  static final Map<String, Integer> READERS = Map.ofEntries(
      Map.entry("com.google.common.collect.Iterables.getLast", 0),
      Map.entry("com.google.common.collect.Iterables.getFirst", 0),
      Map.entry("com.google.common.collect.Iterables.getOnlyElement", 0),
      Map.entry("com.google.common.collect.Iterables.isEmpty", 0),
      Map.entry("com.google.common.collect.Iterables.size", 0),
      Map.entry("com.google.common.base.Preconditions.checkArgument", 1),
      Map.entry("com.google.common.base.Preconditions.checkState", 1),
      Map.entry("com.google.common.base.Preconditions.checkNotNull", 1),
      Map.entry("java.util.Collection.addAll", 0),
      Map.entry("java.util.Collection.containsAll", 0),
      Map.entry("java.util.Collection.removeAll", 0),
      Map.entry("java.util.Map.putAll", 0));

  static final Set<String> OFF = new HashSet<>();

  static {
    String v = System.getenv("J2GO_NOLIBGEN");
    if (v != null) for (String k : v.split(",")) OFF.add(k.trim());
  }

  static boolean on(String kind) {
    return !OFF.contains(kind) && !OFF.contains("all") && System.getenv("J2GO_ERASED") == null;
  }

  static final boolean REPORT = System.getenv("J2GO_LIBGEN") != null;

  final Ctx cx;
  final Set<TypeElement> libs = new LinkedHashSet<>();
  final TreeMap<String, Integer> counts = new TreeMap<>();
  final TreeMap<String, Integer> sites = new TreeMap<>();

  LibGen(Ctx cx) {
    this.cx = cx;
  }

  void register() {
    for (var k : KINDS.entrySet()) {
      if (!on(k.getKey())) continue;
      for (String qn : k.getValue()) {
        TypeElement te = cx.elems.getTypeElement(qn);
        if (te == null) continue;
        libs.add(te);
        cx.typedClasses.add(te);
        cx.keptVars.addAll(te.getTypeParameters());
      }
    }
  }

  void note(String kind, String site) {
    if (!REPORT) return;
    counts.merge(kind, 1, Integer::sum);
    if (site != null) sites.merge(kind + ": " + site, 1, Integer::sum);
  }

  void report() {
    if (!REPORT) return;
    counts.forEach((k, n) -> System.err.println("libgen: " + n + " " + k));
    if ("2".equals(System.getenv("J2GO_LIBGEN"))) sites.forEach((k, n) -> System.err.println("libgen: site " + n + " " + k));
  }

  /** The kind (of KINDS) that the library interface te belongs to, or null. */
  static String kindOf(TypeElement te) {
    if (te == null) return null;
    String qn = te.getQualifiedName().toString();
    for (var k : KINDS.entrySet()) if (k.getValue().contains(qn)) return k.getKey();
    return null;
  }

  /** The erasure of what type argument a stands for in a declaration: a wildcard is its bound. */
  private TypeMirror argBound(TypeMirror a) {
    if (a.getKind() == TypeKind.WILDCARD) {
      WildcardType w = (WildcardType) a;
      a = w.getExtendsBound() != null ? w.getExtendsBound() : object();
    }
    return cx.types.erasure(a);
  }

  /**
   * Piece `cov` (J2GO_NOLIBGEN=cov is the translation without it): whether result type sub of an
   * overriding method narrows result type sup of the method it overrides, where both are the
   * same library interface of an enabled kind: some type argument of sub is, by its erasure,
   * another type than sup's (`Iterable<Var>` under `Iterable<? extends AbstractVar>`; not
   * `List<X>` under `List<? extends X>`). Decided on javac's types alone. Such an override is a
   * method group of its own (rule "covariant overrides"): a caller whose receiver is the
   * narrowing class gets the collection at its element type, with no wrapper; the bridge of the
   * wider group converts.
   */
  boolean narrows(TypeMirror sub, TypeMirror sup) {
    if (!on("cov") || sub.getKind() != TypeKind.DECLARED || sup.getKind() != TypeKind.DECLARED) return false;
    String k = kindOf(Ctx.typeElement(sub));
    if (k == null || !on(k)) return false;
    List<? extends TypeMirror> as = ((DeclaredType) sub).getTypeArguments(), bs = ((DeclaredType) sup).getTypeArguments();
    if (as.isEmpty() || as.size() != bs.size()) return false;
    for (int i = 0; i < as.size(); i++) {
      // (a type variable in the overridden result: an implementation of a generic method, no narrowing)
      if (bs.get(i).getKind() == TypeKind.TYPEVAR || as.get(i).getKind() == TypeKind.TYPEVAR) continue;
      TypeMirror x = argBound(as.get(i)), y = argBound(bs.get(i));
      if (!cx.types.isSameType(x, y) && cx.types.isSubtype(x, y)) return true;
    }
    return false;
  }

  /** The type arguments of t as part of a method name: Of_Var. */
  String argTokens(TypeMirror t) {
    StringBuilder sb = new StringBuilder("Of");
    for (TypeMirror a : ((DeclaredType) t).getTypeArguments()) sb.append("_").append(cx.typeToken(argBound(a)));
    return sb.toString();
  }

  /** Whether lib is an interface of kind `iter`, whose typed methods classes of the program can have themselves. */
  boolean nativeKind(TypeElement lib) {
    return "iter".equals(kindOf(lib));
  }

  boolean isLib(TypeElement te) {
    return te != null && libs.contains(te);
  }

  TypeMirror object() {
    return cx.elems.getTypeElement("java.lang.Object").asType();
  }

  /** The type argument that stands for a (FnTypes.normArg); a type parameter of one of the interfaces themselves is Object. */
  TypeMirror normArg(TypeMirror a) {
    return cx.fn.normArg(a, 0);
  }

  /** t (a use of one of the interfaces) with the type arguments its Go type has. */
  DeclaredType norm(TypeMirror t) {
    DeclaredType d = (DeclaredType) t;
    TypeElement te = Ctx.typeElement(t);
    int n = te.getTypeParameters().size();
    TypeMirror[] as = new TypeMirror[n];
    boolean has = d.getTypeArguments().size() == n;
    for (int i = 0; i < n; i++) as[i] = has ? normArg(d.getTypeArguments().get(i)) : object();
    return cx.types.getDeclaredType(te, as);
  }

  /** Whether a is a type parameter of one of the interfaces themselves. */
  boolean ownVar(TypeMirror a) {
    return a.getKind() == TypeKind.TYPEVAR && ((TypeVariable) a).asElement() instanceof TypeParameterElement tp && tp.getGenericElement() instanceof TypeElement g && isLib(g);
  }

  /**
   * Ctx.erase of a use of one of the interfaces: the typed type. A use without type arguments
   * is the erased interface of the runtime (the source has no raw use: check; the translator
   * makes one with rt and Types.erasure), as is the interface's own declaration type.
   */
  TypeMirror erase(TypeMirror t) {
    DeclaredType d = (DeclaredType) t;
    if (d.getTypeArguments().isEmpty()) return t;
    for (TypeMirror a : d.getTypeArguments()) if (ownVar(a)) return cx.types.erasure(t);
    if (wildErased(t)) {
      note("places left erased: a List with a wildcard (variance)", String.valueOf(t));
      return cx.types.erasure(t);
    }
    if (on("wide")) {
      String why = wide(d);
      if (why != null) {
        note("places left erased: " + why, String.valueOf(t));
        return cx.types.erasure(t);
      }
    }
    return norm(t);
  }

  /**
   * Piece `wide` (J2GO_NOLIBGEN=wide is the translation without it): why a use d of one of the
   * interfaces stays the erased interface, or null. Collections of every element type enter
   * such a place, and as a typed interface each of another element type would need a wrapper,
   * while its typed methods give nothing the erased ones do not:
   * - every type argument is the unbounded wildcard (`Iterable<?>`, `Map<?, ?>`);
   * - a type argument is an erased type variable (`<V> void addEvent(V v, Map<V, Event<V>> m)`:
   *   the variable is Object in Go, the callers' maps have their key types).
   * Decided on the type as written. (`Collection<Object>` and `Iterable<? extends V>` at V =
   * Object are typed at `any`: they are also what a use of a Go type parameter becomes at that
   * instantiation, where the Go type is the typed interface. A member of a Go generic class
   * whose type becomes one of the two forms above by substitution would not compile; the tree
   * has none.)
   */
  String wide(DeclaredType d) {
    boolean all = true;
    for (TypeMirror a : d.getTypeArguments()) {
      if (a.getKind() == TypeKind.TYPEVAR && !cx.typedVar(a) && !FnTypes.captured(a)) return "a type argument is an erased type variable";
      all &= unbounded(a);
    }
    return all ? "every type argument is the wildcard `?`" : null;
  }

  /** Whether every type argument of d is the wildcard `?`: a cast to such a type claims nothing about the elements. */
  boolean allUnbounded(DeclaredType d) {
    for (TypeMirror a : d.getTypeArguments()) if (!unbounded(a)) return false;
    return true;
  }

  /** Whether type argument a is the wildcard `?`, as written or captured (the type of an expression). */
  private boolean unbounded(TypeMirror a) {
    if (a.getKind() == TypeKind.WILDCARD) return ((WildcardType) a).getExtendsBound() == null && ((WildcardType) a).getSuperBound() == null;
    if (!FnTypes.captured(a)) return false;
    TypeVariable v = (TypeVariable) a;
    TypeMirror lo = v.getLowerBound(), up = v.getUpperBound();
    return (lo == null || lo.getKind() == TypeKind.NULL) && up != null && up.getKind() == TypeKind.DECLARED && Ctx.typeElement(up).getQualifiedName().contentEquals("java.lang.Object");
  }

  /**
   * Type t of the signature of a library method as the runtime declares it: one of the
   * interfaces is erased there (the runtime's collections are), and a type parameter of the
   * interfaces is Object.
   */
  TypeMirror rt(TypeMirror t) {
    if (libs.isEmpty() || t == null) return cx.erase(t);
    if (t.getKind().isPrimitive() || t.getKind() == TypeKind.VOID) return t;
    Map<Element, TypeMirror> m = new HashMap<>();
    for (TypeElement lib : libs) for (TypeParameterElement tp : lib.getTypeParameters()) m.put(tp, object());
    rtDepth++;
    try {
      TypeMirror e = cx.erase(cx.subst(t, m, 0));
      return is(e) ? cx.types.erasure(e) : e;
    } finally {
      rtDepth--;
    }
  }

  /**
   * Whether a use t of one of the interfaces stays the erased interface because of variance: a
   * `List<? extends X>` or `List<?>`. A list of a narrower element type enters such a place (the
   * graph classes' `List<LinkedDiGraphEdge>` where `List<? extends DiGraphEdge>` is declared),
   * and as a typed List of the wider type it would need a second object per list: measured, one
   * allocation per call of the graph nodes' getInEdges / getOutEdges, also when the object that
   * has the list keeps the view (most lists are asked for once). Erased, the list enters as it
   * is and the reader casts each element, as before the kind. The narrowing override itself is
   * typed (piece `cov`), so a caller that has the narrowing class reads without a cast.
   */
  boolean wildErased(TypeMirror t) {
    if (!"list".equals(kindOf(Ctx.typeElement(t)))) return false;
    for (TypeMirror a : ((DeclaredType) t).getTypeArguments()) {
      // (the type of an expression has the wildcard captured)
      if (a.getKind() == TypeKind.WILDCARD || a.getKind() == TypeKind.TYPEVAR && a.toString().startsWith("capture")) return true;
    }
    return false;
  }

  /** Whether (erased) type t is a typed use of one of the interfaces. */
  boolean is(TypeMirror t) {
    return t != null && t.getKind() == TypeKind.DECLARED && libs.contains(Ctx.typeElement(t)) && !((DeclaredType) t).getTypeArguments().isEmpty();
  }

  /** The Go name of the typed interface of te. */
  String typedName(TypeElement te) {
    return cx.className(te) + "T";
  }

  /**
   * Whether class c of the program has the typed methods of interface lib itself: it names lib
   * among its interfaces and has no subclass (so the methods are not dispatched).
   */
  boolean nativeClass(TypeElement c, TypeElement lib) {
    if (c == null || !cx.isSource(c) || c.getKind().isInterface() || !cx.isLeaf(c) || cx.isHandle(c) || !"iter".equals(kindOf(lib))) return false;
    for (TypeMirror st : c.getInterfaces()) if (lib.equals(Ctx.typeElement(st))) return true;
    return false;
  }

  /** The typed view of lib that a value of (erased) type site has for its methods, or null: site is a typed use, or a native class. */
  DeclaredType typedView(TypeMirror site, TypeElement lib) {
    if (site == null) return null;
    TypeMirror s = cx.erase(site);
    if (s.getKind() != TypeKind.DECLARED) return null;
    TypeElement se = Ctx.typeElement(s);
    if (!is(s) && !nativeClass(se, lib)) return null;
    DeclaredType v = cx.superView(s, lib);
    if (v == null) return null;
    TypeMirror n = erase(v);
    return is(n) ? (DeclaredType) n : null;
  }

  /** Whether the group of m (rooted in one of the interfaces) has its typed signature for a receiver of type site. */
  boolean typedAt(ExecutableElement m, TypeMirror site) {
    TypeElement lib = cx.ownerOf(cx.root(m));
    return isLib(lib) && typedMethod(cx.root(m)) && typedView(site, lib) != null;
  }

  /** Whether root method r of one of the interfaces has a typed form of its own (its signature names a type parameter): the Go method r_t. */
  boolean renamed(ExecutableElement r) {
    if (!isLib(cx.ownerOf(r)) || r.isDefault() || r.getModifiers().contains(Modifier.STATIC)) return false;
    Set<TypeParameterElement> tv = new LinkedHashSet<>();
    cx.tvarsOf(r.getReturnType(), tv);
    for (VariableElement p : r.getParameters()) cx.tvarsOf(p.asType(), tv);
    return !tv.isEmpty();
  }

  /** The Go name of the method of m's group for a receiver of type site. */
  String methodName(ExecutableElement m, TypeMirror site) {
    String n = cx.methodName(m);
    return typedAt(m, site) && renamed(cx.root(m)) ? n + "_t" : n;
  }

  /** The erased signature of root r of one of the interfaces: its type parameters at Object. */
  TypeMirror erasedOf(TypeMirror t, TypeElement lib) {
    return rt(t);
  }

  String asName(TypeElement lib) {
    String n = cx.className(lib);
    return Character.toLowerCase(n.charAt(0)) + n.substring(1) + "As_";
  }

  /**
   * A typed collection (list, set, deque) as the Iterable of its elements: itself; as the
   * Collection / List / Set of its elements, a typed map as the Map of its keys and values, a
   * typed entry as the Map.Entry: its view, which has the typed methods (asCollT, asListT,
   * asSetT, asMapT, asEntryT). Null where from is no typed collection of to's kind and type
   * arguments.
   */
  String fromTypedColl(String code, TypeMirror from, TypeMirror to) {
    if (!is(to)) return null;
    Ctx.CollKind k = cx.typedColl(from);
    if (k == null || k.builds != null) return null;
    String as;
    switch (Ctx.typeElement(to).getQualifiedName().toString()) {
      case "java.lang.Iterable": as = k.pair() ? null : ".asIterable()"; break;
      case "java.util.Collection": as = k.pair() ? null : ".asCollT()"; break;
      case "java.util.List": as = k.name.equals("list") ? ".asListT()" : null; break;
      case "java.util.Set": as = k.name.equals("set") ? ".asSetT()" : null; break;
      case "java.util.Map": as = k.name.equals("map") ? ".asMapT()" : null; break;
      case "java.util.Map.Entry": as = k == cx.entryKind ? ".asEntryT()" : null; break;
      case "java.util.Queue": as = k.name.equals("deque") ? ".asQueueT()" : null; break;
      case "java.util.Deque": as = k.name.equals("deque") ? ".asDequeT()" : null; break;
      default: as = null;
    }
    if (as == null) return null;
    List<? extends TypeMirror> fa = ((DeclaredType) from).getTypeArguments(), ta = ((DeclaredType) to).getTypeArguments();
    if (fa.size() != ta.size()) return null;
    for (int i = 0; i < fa.size(); i++) if (!cx.goTypeArg(fa.get(i)).equals(cx.goTypeArg(ta.get(i)))) return null;
    return ExprEmitter.atomOrParen(code) + as;
  }

  /** Ctx.coerce where from or to is a typed use of one of the interfaces (both erased); null: as any other interface. */
  String coerce(String code, TypeMirror from, TypeMirror to) {
    Ctx.K kf = cx.kind(from), kt = cx.kind(to);
    if (is(to)) {
      TypeElement lib = Ctx.typeElement(to);
      if (is(from) && !lib.equals(Ctx.typeElement(from))) {
        // a typed interface above from's, at the same element type: the same value (CollectionT[X] is an IterableT[X])
        DeclaredType v = cx.superView(from, lib);
        if (v != null && is(erase(v)) && cx.goType(erase(v)).equals(cx.goType(to))) return code;
      }
      if (!is(from) && from.getKind() == TypeKind.DECLARED && nativeClass(Ctx.typeElement(from), lib)) {
        DeclaredType v = typedView(from, lib);
        // an object of a native class: Go converts it (Ctx.coerce, as for the program's typed interfaces)
        if (v != null && cx.goType(v).equals(cx.goType(to))) return null;
      }
      TypeMirror er = cx.types.erasure(to);
      note("erased to typed (" + asName(lib) + ")", cx.goType(from) + " -> " + cx.goType(to));
      StringBuilder tc = new StringBuilder();
      for (TypeMirror a : ((DeclaredType) to).getTypeArguments()) {
        // (an element that is an erased library type, whatever type variables its Java type names)
        TypeMirror ea = cx.erase(a);
        tc.append(cx.tconv(ea)).append(", ");
      }
      return asName(lib) + "(" + tc + cx.coerce(code, from, er) + ")";
    }
    // from is typed
    TypeElement lib = Ctx.typeElement(from);
    if (kt == Ctx.K.ANY || kt == Ctx.K.TVAR) {
      String box = "lgBox_(" + code + ")";
      return kt == Ctx.K.ANY ? box : "unboxT_[" + cx.goType(to) + "](" + box + ")";
    }
    TypeMirror er = cx.types.erasure(from);
    if (cx.types.isSubtype(er, cx.types.erasure(to))) {
      // the erased interface, or one above it: the object erased code knows
      return cx.coerce(cx.className(lib) + "_un_(" + code + ")", er, to);
    }
    return cx.coerce("lgBox_(" + code + ")", object(), to);
  }

  /** A typed view of a typed map: the method of the runtime's typed map that gives it, and its type. */
  record View(String method, TypeMirror type) {}

  /**
   * Piece `views` (J2GO_NOLIBGEN=views is the translation without it): the typed view that call
   * m (with nargs arguments) on a typed map of type recv gives, or null where the erased view
   * serves the call: `values()` declared with result java.util.Collection, where Collection is a
   * typed interface, is the Collection of the map's values (valuesT), `keySet()` the Set of its
   * keys (keySetT), `entrySet()` the Set of its entries (entrySetT).
   */
  View mapView(Ctx.CollKind rk, TypeMirror recv, ExecutableElement m, int nargs) {
    if (!on("views") || !rk.name.equals("map") || nargs != 0) return null;
    String n = m.getSimpleName().toString();
    if (!n.equals("values") && !n.equals("keySet") && !n.equals("entrySet")) return null;
    TypeElement coll = cx.elems.getTypeElement(n.equals("values") ? "java.util.Collection" : "java.util.Set");
    if (!isLib(coll) || !coll.equals(Ctx.typeElement(cx.types.erasure(m.getReturnType())))) return null;
    List<? extends TypeMirror> ra = ((DeclaredType) recv).getTypeArguments();
    TypeMirror elem;
    if (n.equals("entrySet")) {
      TypeElement entry = cx.elems.getTypeElement("java.util.Map.Entry");
      if (!isLib(entry)) return null;
      elem = cx.types.getDeclaredType(entry, ra.get(0), ra.get(1));
      if (!is(cx.erase(elem))) return null;
    } else {
      elem = ra.get(n.equals("values") ? 1 : 0);
    }
    TypeMirror t = cx.erase(cx.types.getDeclaredType(coll, elem));
    return is(t) ? new View(n + "T", t) : null;
  }

  /**
   * Whether call t of library method m gives its one argument back (the runtime's
   * Collections.unmodifiableCollection / List / Set / Map / SortedSet return it: rt_coll.go), so
   * that its value at a typed interface is the argument's.
   */
  boolean passes(Element m, MethodInvocationTree t) {
    if (!(m instanceof ExecutableElement me) || t.getArguments().size() != 1 || !on("views")) return false;
    String n = me.getSimpleName().toString();
    return (n.equals("unmodifiableCollection") || n.equals("unmodifiableList") || n.equals("unmodifiableSet") || n.equals("unmodifiableMap"))
        && ((TypeElement) me.getEnclosingElement()).getQualifiedName().contentEquals("java.util.Collections");
  }

  // ------------------------------------------------------------------ refusals

  final List<String> refusals = new ArrayList<>();

  /**
   * The collection classes that the typed collections stand for where a collection is made for a
   * typed place (piece `place`, and rule "typed collections"): such a collection is the typed
   * one's view, no object of the class, so a test for the class or a cast to it is refused. (The
   * immutable classes are not among them: erased code sees an immutable typed collection as its
   * twin, an object of Guava's class.)
   */
  static final Set<String> CONCRETE = Set.of("java.util.ArrayList", "java.util.HashSet", "java.util.LinkedHashSet", "java.util.HashMap",
      "java.util.LinkedHashMap", "java.util.TreeSet", "java.util.TreeMap", "java.util.ArrayDeque");

  /** The methods ("Outer.Inner.method") whose unchecked casts to a typed library interface were read and are safe: tools/j2go/unchecked-casts.txt. */
  final Set<String> uncheckedOk = new HashSet<>();

  void readUncheckedOk(String file) {
    try {
      for (String l : java.nio.file.Files.readAllLines(java.nio.file.Path.of(file))) {
        l = l.replaceAll("#.*", "").trim();
        if (!l.isEmpty()) uncheckedOk.add(l);
      }
    } catch (java.io.IOException e) {
      throw new java.io.UncheckedIOException(e);
    }
  }

  /**
   * Refused (the translation stops): a raw use of one of the interfaces, which has no element
   * type to give the typed methods; an unchecked cast to a use of one of them outside the
   * methods listed in tools/j2go/unchecked-casts.txt; `instanceof` of or a cast to one of the
   * classes CONCRETE.
   */
  void check(Trees trees, CompilationUnitTree cu) {
    if (libs.isEmpty()) return;
    String file = new java.io.File(cu.getSourceFile().getName()).getName();
    new TreePathScanner<Void, Void>() {
      void raw(Tree at, TypeMirror t, int depth) {
        if (t == null || depth > 8) return;
        if (t.getKind() == TypeKind.ARRAY) {
          raw(at, ((ArrayType) t).getComponentType(), depth + 1);
        } else if (t.getKind() == TypeKind.DECLARED) {
          DeclaredType d = (DeclaredType) t;
          if (isLib(Ctx.typeElement(t)) && d.getTypeArguments().isEmpty()) {
            long p = trees.getSourcePositions().getStartPosition(cu, at);
            refusals.add(file + ":" + (p < 0 ? 0 : cu.getLineMap().getLineNumber(p)) + ": raw use of " + Ctx.typeElement(t).getQualifiedName() + ": " + at.toString().replace('\n', ' ').replaceAll("\\s+", " "));
          }
          for (TypeMirror a : d.getTypeArguments()) raw(at, a, depth + 1);
        }
      }

      @Override
      public Void visitVariable(VariableTree t, Void v) {
        Element e = trees.getElement(getCurrentPath());
        if (e != null && t.getType() != null) raw(t, e.asType(), 0);
        return super.visitVariable(t, v);
      }

      @Override
      public Void visitMethod(MethodTree t, Void v) {
        Element e = trees.getElement(getCurrentPath());
        if (e instanceof ExecutableElement m && t.getReturnType() != null) raw(t.getReturnType(), m.getReturnType(), 0);
        return super.visitMethod(t, v);
      }

      /** "Outer.Inner.method" of the method (or the class) that the current tree is in. */
      String where() {
        String m = "";
        for (TreePath p = getCurrentPath(); p != null; p = p.getParentPath()) {
          if (p.getLeaf() instanceof MethodTree mt && m.isEmpty()) m = "." + mt.getName();
          if (p.getLeaf() instanceof ClassTree && trees.getElement(p) instanceof TypeElement te) {
            String c = "";
            for (Element x = te; x instanceof TypeElement; x = x.getEnclosingElement()) c = x.getSimpleName() + (c.isEmpty() ? "" : "." + c);
            return c + m;
          }
        }
        return m;
      }

      void refuse(Tree at, String what) {
        long p = trees.getSourcePositions().getStartPosition(cu, at);
        refusals.add(file + ":" + (p < 0 ? 0 : cu.getLineMap().getLineNumber(p)) + ": " + what + ": " + at.toString().replace('\n', ' ').replaceAll("\\s+", " "));
      }

      /** A test for or a cast to a collection class whose objects the typed collections replace. */
      void concrete(Tree at, TypeMirror target, String what) {
        if (target == null || target.getKind() != TypeKind.DECLARED || !on("place")) return;
        String qn = Ctx.typeElement(target).getQualifiedName().toString();
        if (CONCRETE.contains(qn)) refuse(at, what + " " + qn + " (a collection made for a typed place is no object of that class)");
        if (madeT2(qn)) refuse(at, what + " " + qn + " (a collection made for a typed place by LibGen.MAKERS_T2 is no object of that class)");
      }

      @Override
      public Void visitTypeCast(TypeCastTree t, Void v) {
        TypeMirror target = trees.getTypeMirror(new TreePath(getCurrentPath(), t.getType()));
        raw(t, target, 0);
        concrete(t, target, "cast to");
        TypeMirror from = trees.getTypeMirror(new TreePath(getCurrentPath(), t.getExpression()));
        if (target != null && from != null && target.getKind() == TypeKind.DECLARED && (isLib(Ctx.typeElement(target)) && is(cx.erase(target)) || t2(target) && !((DeclaredType) target).getTypeArguments().isEmpty())
            && !allUnbounded((DeclaredType) target) && from.getKind() != TypeKind.NULL && !cx.types.isSubtype(from, target) && !uncheckedOk.contains(where())) {
          // an unchecked cast: the object may be a collection of other elements (a typed collection
          // made for another place, which refuses an element that Java's would take)
          refuse(t, "unchecked cast in " + where() + " (not in tools/j2go/unchecked-casts.txt)");
        }
        return super.visitTypeCast(t, v);
      }

      @Override
      public Void visitInstanceOf(InstanceOfTree t, Void v) {
        Tree ty = t.getType() != null ? t.getType() : t.getPattern() instanceof BindingPatternTree b ? b.getVariable().getType() : null;
        if (ty != null) concrete(t, trees.getTypeMirror(new TreePath(getCurrentPath(), ty)), "instanceof");
        return super.visitInstanceOf(t, v);
      }

      @Override
      public Void visitMethodInvocation(MethodInvocationTree t, Void v) {
        // a method that a class of CONCRETE declares itself, on a receiver of the class with type
        // arguments (whose place holds the typed interface, piece `cls`): the view of a
        // collection made for the place has only the methods of OWN
        if (on("cls") && t.getMethodSelect() instanceof MemberSelectTree ms && trees.getElement(new TreePath(getCurrentPath(), ms)) instanceof ExecutableElement m
            && !m.getModifiers().contains(Modifier.STATIC)) {
          TypeMirror rt = trees.getTypeMirror(new TreePath(new TreePath(getCurrentPath(), ms), ms.getExpression()));
          String qn = rt != null && rt.getKind() == TypeKind.DECLARED && !((DeclaredType) rt).getTypeArguments().isEmpty() ? Ctx.typeElement(rt).getQualifiedName().toString() : "";
          if (madeT2(qn) && ownMethod(m, Ctx.typeElement(rt)) && !(m.getModifiers().contains(Modifier.STATIC))) {
            refuse(t, "method " + m.getSimpleName() + " of the class " + qn + " (a collection made for its typed place by LibGen.MAKERS_T2 does not have it)");
          }
          if (CONCRETE.contains(qn) && CLASSES.containsKey(qn) && isLib(cx.elems.getTypeElement(CLASSES.get(qn))) && ownMethod(m, Ctx.typeElement(rt))) {
            if (!OWN.containsKey(((TypeElement) m.getEnclosingElement()).getQualifiedName() + "." + m.getSimpleName())) {
              refuse(t, "method " + m.getSimpleName() + " of the collection class " + qn + " (the view of a typed collection does not have it; LibGen.OWN)");
            }
          }
        }
        madeParam(t);
        return super.visitMethodInvocation(t, v);
      }

      /** Whether m, found in collection class c, overrides no method of an interface or of Object (javac's types alone). */
      boolean ownMethod(ExecutableElement m, TypeElement c) {
        TypeElement o = (TypeElement) m.getEnclosingElement();
        if (o.getKind().isInterface() || o.getQualifiedName().contentEquals("java.lang.Object")) return false;
        for (Element x : cx.elems.getAllMembers(c)) {
          if (x instanceof ExecutableElement y && y != m && y.getSimpleName().equals(m.getSimpleName()) && cx.elems.overrides(m, y, c)) return false;
        }
        Deque<TypeMirror> todo = new ArrayDeque<>(cx.types.directSupertypes(c.asType()));
        Set<String> seen = new HashSet<>();
        while (!todo.isEmpty()) {
          TypeMirror st = todo.pop();
          TypeElement se = Ctx.typeElement(st);
          if (se == null || !seen.add(se.getQualifiedName().toString())) continue;
          for (Element x : se.getEnclosedElements()) {
            if (x instanceof ExecutableElement y && y.getSimpleName().equals(m.getSimpleName()) && (se.getKind().isInterface() || se.getQualifiedName().contentEquals("java.lang.Object")) && cx.elems.overrides(m, y, c)) return false;
          }
          todo.addAll(cx.types.directSupertypes(st));
        }
        return true;
      }

      /** A library function or method with a parameter of a class whose typed places MAKERS_T2 serve (EnumSet.copyOf(EnumSet), complementOf): its argument may be no object of the class. */
      void madeParam(MethodInvocationTree t) {
        if (!(trees.getElement(new TreePath(getCurrentPath(), t.getMethodSelect())) instanceof ExecutableElement m) || cx.isSource((TypeElement) m.getEnclosingElement())) return;
        for (VariableElement p : m.getParameters()) {
          if (p.asType().getKind() == TypeKind.DECLARED && madeT2(Ctx.typeElement(p.asType()).getQualifiedName().toString())) {
            refuse(t, "library method " + m.getEnclosingElement().getSimpleName() + "." + m.getSimpleName() + " with a parameter of " + Ctx.typeElement(p.asType()).getQualifiedName() + " (a collection made for a typed place by LibGen.MAKERS_T2 is no object of that class)");
            return;
          }
        }
      }

      @Override
      public Void visitNewClass(NewClassTree t, Void v) {
        if (t.getClassBody() == null) raw(t, trees.getTypeMirror(getCurrentPath()), 0);
        return super.visitNewClass(t, v);
      }
    }.scan(cu, null);
  }
}
