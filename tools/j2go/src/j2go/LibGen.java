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
 * (java.util.List), `set` (java.util.Set), `map` (java.util.Map, java.util.Map.Entry).
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
      "java.util.Map.Entry", Set.of("getKey/0", "getValue/0"));

  /** Whether root method r of one of the interfaces has its typed signature on a typed receiver. */
  boolean typedMethod(ExecutableElement r) {
    Set<String> ms = TYPED_METHODS.get(cx.ownerOf(r).getQualifiedName().toString());
    return ms == null || ms.contains(r.getSimpleName() + "/" + r.getParameters().size());
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
    Map<Element, TypeMirror> m = new HashMap<>();
    for (TypeElement lib : libs) for (TypeParameterElement tp : lib.getTypeParameters()) m.put(tp, object());
    TypeMirror e = cx.erase(cx.subst(t, m, 0));
    return is(e) ? cx.types.erasure(e) : e;
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
      }

      @Override
      public Void visitTypeCast(TypeCastTree t, Void v) {
        TypeMirror target = trees.getTypeMirror(new TreePath(getCurrentPath(), t.getType()));
        raw(t, target, 0);
        concrete(t, target, "cast to");
        TypeMirror from = trees.getTypeMirror(new TreePath(getCurrentPath(), t.getExpression()));
        if (target != null && from != null && target.getKind() == TypeKind.DECLARED && isLib(Ctx.typeElement(target)) && is(cx.erase(target))
            && from.getKind() != TypeKind.NULL && !cx.types.isSubtype(from, target) && !uncheckedOk.contains(where())) {
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
      public Void visitNewClass(NewClassTree t, Void v) {
        if (t.getClassBody() == null) raw(t, trees.getTypeMirror(getCurrentPath()), 0);
        return super.visitNewClass(t, v);
      }
    }.scan(cu, null);
  }
}
