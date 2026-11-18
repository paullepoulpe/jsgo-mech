package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;
import javax.lang.model.util.*;

/** Whole-program model: classes, hierarchy, names, method groups, type mapping and coercions. */
final class Ctx {
  final Trees trees;
  final Types types;
  final Elements elems;

  /** Every class declared in the translated sources, including nested, local and anonymous ones. */
  final LinkedHashSet<TypeElement> source = new LinkedHashSet<>();
  final Map<TypeElement, List<TypeElement>> subclasses = new HashMap<>();
  final Map<TypeElement, String> classNames = new HashMap<>();
  final Map<String, TypeElement> classByName = new HashMap<>();
  /** External (library) members and types the translation refers to: Go spelling -> description. */
  final TreeMap<String, String> externals = new TreeMap<>();

  enum K { PRIM, STR, CLS, IFACE, ANY, ARR, NULL, VOID, TVAR }

  // ---------------------------------------------------------------- typed generic classes

  /**
   * Generic classes of the program that are Go generic types (GenScan): C<X> is *C[X], and their
   * type variables (and those of their static generic methods) are Go type parameters.
   */
  final Set<TypeElement> typedClasses = new HashSet<>();
  /** The type parameters that are Go type parameters (GenScan.kept); the others are erased. */
  final Set<TypeParameterElement> keptVars = new HashSet<>();
  /** Erased class type parameters that every use instantiates with their bound: as type arguments, the bound. */
  final Set<TypeParameterElement> fixedVars = new HashSet<>();

  /**
   * Generic instance methods with a kept type parameter: Go generic functions taking the receiver
   * first (GenScan.genericFunction: they override nothing and nothing overrides them).
   */
  final Set<ExecutableElement> genFns = new HashSet<>();

  /** The Go name of generic function m: Owner_m (Owner_m_this if a static method has m's name). */
  String genFnName(ExecutableElement m) {
    TypeElement owner = ownerOf(m);
    String n = className(owner) + "_" + safeMember(m.getSimpleName().toString());
    for (Element e : owner.getEnclosedElements()) {
      if (e.getKind() == ElementKind.METHOD && e.getModifiers().contains(Modifier.STATIC) && e.getSimpleName().equals(m.getSimpleName())) return n + "_this";
    }
    long same = owner.getEnclosedElements().stream().filter(e -> e.getKind() == ElementKind.METHOD && e.getSimpleName().equals(m.getSimpleName())).count();
    return same > 1 ? n + sigToken(m) : n;
  }

  /** The type parameter list of generic function m: its class's, then its own kept ones. */
  String genFnTparamsDecl(ExecutableElement m) {
    String c = goTparamsDecl(ownerOf(m)), o = typeParamsDecl(m.getTypeParameters());
    if (c.isEmpty()) return o;
    if (o.isEmpty()) return c;
    return c.substring(0, c.length() - 1) + ", " + o.substring(1);
  }

  /**
   * The Go constraint of kept type parameter tp: `comparable`, with its bound if that is an
   * interface of the program. Every type that instantiates a kept type parameter is comparable
   * in Go (a pointer, an interface, a string, a handle, a box), so the constraint costs
   * nothing, and a type parameter can then be the element type of a typed set or the key type
   * of a typed map (rule "typed collections"). J2GO_NOCOMPARABLE=1: `any` or the bound.
   */
  String constraint(TypeParameterElement tp) {
    String b = bound(tp);
    if (notComparable.contains(tp) || "1".equals(System.getenv("J2GO_NOCOMPARABLE"))) return b == null ? "any" : b;
    return b == null ? "comparable" : "interface{ " + b + "; comparable }";
  }

  /**
   * The kept type parameters that are not `comparable`: some use instantiates them with a type
   * that Go cannot compare (an array: Outcome<byte[]>), or with another such type parameter.
   */
  final Set<TypeParameterElement> notComparable = new HashSet<>();

  /** Finds notComparable: every type that the program names or that an expression has, and the type arguments javac infers for the calls of generic methods. */
  void findComparable(List<? extends CompilationUnitTree> units) {
    Map<TypeParameterElement, Set<TypeParameterElement>> gives = new HashMap<>();  // X -> the parameters that X instantiates
    Set<TypeMirror> seen = Collections.newSetFromMap(new IdentityHashMap<>());
    class Scan {
      void arg(TypeParameterElement p, TypeMirror a) {
        if (a == null || !keptVars.contains(p)) return;
        if (a.getKind() == TypeKind.WILDCARD) {
          arg(p, ((WildcardType) a).getExtendsBound());
          arg(p, ((WildcardType) a).getSuperBound());
        } else if (a.getKind() == TypeKind.ARRAY) {
          notComparable.add(p);
        } else if (typedVar(a)) {
          gives.computeIfAbsent((TypeParameterElement) ((TypeVariable) a).asElement(), k -> new HashSet<>()).add(p);
        }
      }

      void type(TypeMirror t) {
        if (t == null || !seen.add(t)) return;
        if (t.getKind() == TypeKind.ARRAY) {
          type(((ArrayType) t).getComponentType());
        } else if (t.getKind() == TypeKind.WILDCARD) {
          type(((WildcardType) t).getExtendsBound());
          type(((WildcardType) t).getSuperBound());
        } else if (t.getKind() == TypeKind.DECLARED) {
          DeclaredType d = (DeclaredType) t;
          List<? extends TypeParameterElement> ps = typeElement(d).getTypeParameters();
          for (int i = 0; i < d.getTypeArguments().size() && i < ps.size(); i++) {
            arg(ps.get(i), d.getTypeArguments().get(i));
            type(d.getTypeArguments().get(i));
          }
          type(d.getEnclosingType());
        } else if (t.getKind() == TypeKind.EXECUTABLE) {
          type(((ExecutableType) t).getReturnType());
          for (TypeMirror p : ((ExecutableType) t).getParameterTypes()) type(p);
        }
      }
    }
    Scan sc = new Scan();
    for (CompilationUnitTree cu : units) {
      new TreePathScanner<Void, Void>() {
        @Override
        public Void scan(Tree t, Void v) {
          if (t == null) return null;
          TreePath p = new TreePath(getCurrentPath(), t);
          try {
            TypeMirror tm = trees.getTypeMirror(p);
            sc.type(tm);
            if (t instanceof MethodInvocationTree mi && trees.getElement(new TreePath(p, mi.getMethodSelect())) instanceof ExecutableElement m
                && trees.getTypeMirror(new TreePath(p, mi.getMethodSelect())) instanceof ExecutableType inst && !m.getTypeParameters().isEmpty()) {
              // the type arguments javac inferred for a generic method
              Map<Element, TypeMirror> u = new HashMap<>();
              ExprEmitter.unify(m.getReturnType(), inst.getReturnType(), u);
              for (int i = 0; i < m.getParameters().size() && i < inst.getParameterTypes().size(); i++) ExprEmitter.unify(m.getParameters().get(i).asType(), inst.getParameterTypes().get(i), u);
              for (TypeParameterElement tp : m.getTypeParameters()) sc.arg(tp, u.get(tp));
            }
          } catch (RuntimeException ex) {
            // a tree without attribution
          }
          return super.scan(t, v);
        }
      }.scan(new TreePath(cu), null);
    }
    ArrayDeque<TypeParameterElement> work = new ArrayDeque<>(notComparable);
    while (!work.isEmpty()) {
      for (TypeParameterElement p : gives.getOrDefault(work.poll(), Set.of())) if (notComparable.add(p)) work.add(p);
    }
  }

  /** The Go type of the bound of kept type parameter tp where that is an interface of the program; else null. */
  String bound(TypeParameterElement tp) {
    List<? extends TypeMirror> bs = tp.getBounds();
    return bs.size() == 1 && kind(bs.get(0)) == K.IFACE && isSource(typeElement(bs.get(0))) ? goType(bs.get(0)) : null;
  }

  /** A type variable that is a Go type parameter: of a typed declaration, or of a static generic method of one. */
  boolean typedVar(TypeMirror t) {
    return t != null && t.getKind() == TypeKind.TYPEVAR && ((TypeVariable) t).asElement() instanceof TypeParameterElement tp && keptVars.contains(tp);
  }

  /** The kept type parameters among tps. */
  List<TypeParameterElement> keptOf(List<? extends TypeParameterElement> tps) {
    List<TypeParameterElement> out = new ArrayList<>();
    for (TypeParameterElement tp : tps) if (keptVars.contains(tp)) out.add(tp);
    return out;
  }

  /**
   * t with the kept type parameters of owner replaced by site's view of owner's type arguments
   * (asMemberOf, but an erased type parameter stays itself: erase() makes it its bound).
   */
  TypeMirror atSite(TypeMirror t, TypeElement owner, TypeMirror site) {
    if (site == null || !typedClasses.contains(owner)) return t;
    DeclaredType v = superView(erase(site), owner);
    if (v == null || v.getTypeArguments().isEmpty()) return t;
    Map<Element, TypeMirror> m = new HashMap<>();
    List<? extends TypeParameterElement> ps = owner.getTypeParameters();
    for (int i = 0; i < ps.size(); i++) if (keptVars.contains(ps.get(i))) m.put(ps.get(i), v.getTypeArguments().get(i));
    return subst(t, m, 0);
  }

  TypeMirror subst(TypeMirror t, Map<Element, TypeMirror> m, int depth) {
    if (t == null || m.isEmpty() || depth > 12) return t;
    switch (t.getKind()) {
      case TYPEVAR: {
        TypeMirror r = m.get(((TypeVariable) t).asElement());
        return r != null ? r : t;
      }
      case ARRAY:
        return types.getArrayType(subst(((ArrayType) t).getComponentType(), m, depth + 1));
      case WILDCARD: {
        WildcardType w = (WildcardType) t;
        return types.getWildcardType(subst(w.getExtendsBound(), m, depth + 1), subst(w.getSuperBound(), m, depth + 1));
      }
      case DECLARED: {
        DeclaredType d = (DeclaredType) t;
        if (d.getTypeArguments().isEmpty()) return t;
        TypeMirror[] as = new TypeMirror[d.getTypeArguments().size()];
        for (int i = 0; i < as.length; i++) as[i] = subst(d.getTypeArguments().get(i), m, depth + 1);
        TypeElement te = typeElement(d);
        if (d.getEnclosingType() instanceof DeclaredType enc && !enc.getTypeArguments().isEmpty()) {
          return types.getDeclaredType((DeclaredType) subst(enc, m, depth + 1), te, as);
        }
        return types.getDeclaredType(te, as);
      }
      default:
        return t;
    }
  }

  /**
   * Value library classes (Rule "Optional is a value"): library generic classes that the runtime
   * implements as a generic Go struct whose values are immutable and have no identity the program
   * looks at (java.util.Optional[T]: its zero value is Java's null). They are typed
   * declarations whatever the program does with them: a wildcard, raw or erased type argument
   * is `any` at that site only (Optional[any]), and a value passes between two instantiations
   * by a conversion of its content (optAs_), where Java passes the same object.
   */
  final Set<TypeElement> valueLibs = new HashSet<>();

  boolean isValueLib(TypeElement te) {
    return te != null && valueLibs.contains(te);
  }

  boolean isValueLibType(TypeMirror t) {
    return t != null && t.getKind() == TypeKind.DECLARED && valueLibs.contains(typeElement(t));
  }

  void registerValueLibs() {
    for (String qn : List.of("java.util.Optional")) {
      TypeElement te = elems.getTypeElement(qn);
      if (te == null) continue;
      valueLibs.add(te);
      typedClasses.add(te);
      keptVars.addAll(te.getTypeParameters());
      for (Element e : te.getEnclosedElements()) {
        if (e instanceof ExecutableElement m && m.getModifiers().contains(Modifier.STATIC)) keptVars.addAll(m.getTypeParameters());
      }
    }
  }

  // ---------------------------------------------------------------- typed collections

  /**
   * Typed collections (rule "typed collections", Colls): a declaration (a field, a local
   * variable, a parameter or the result of a method) of a collection type that only ever holds
   * collections the program makes for it (`new ArrayList<>(..)`, null, or the value of another
   * such declaration) holds the runtime's typed collection (*TList[X], *TSet[X], *TDeque[X],
   * *TMap[K, V]; rt_tlist.go, rt_tset.go, rt_tdeque.go, rt_tmap.go), whose elements are X
   * values, not boxes. The translator sees such a declaration, and the expressions that read
   * it, at the type TList<X> / TSet<X> / TDeque<X> / TMap<K, V> of a class that exists for it
   * alone (STUBS: the typed methods of the runtime's type); every other site that the
   * collection reaches sees it as Java's List / Set / Deque / Map through its one erased view
   * (asList(), asSet(), asDeque(), asMap()). TEntry<K, V> is the entry of a typed map in a
   * for-each loop over its entries.
   */
  static final Map<String, String> STUBS = Map.of(
      "TListBuilder", """
      package j2go.rt;
      public final class TListBuilder<T> {
        public native TListBuilder<T> add(T v);
        public native TListBuilder<T> addAll(java.lang.Iterable<? extends T> c);
        public native TList<T> build();
      }
      """,
      "TSetBuilder", """
      package j2go.rt;
      public final class TSetBuilder<T> {
        public native TSetBuilder<T> add(T v);
        public native TSetBuilder<T> addAll(java.lang.Iterable<? extends T> c);
        public native TSet<T> build();
      }
      """,
      "TMapBuilder", """
      package j2go.rt;
      public final class TMapBuilder<K, V> {
        public native TMapBuilder<K, V> put(K k, V v);
        public native TMapBuilder<K, V> putAll(java.util.Map<? extends K, ? extends V> m);
        public native TMap<K, V> build();
        public native TMap<K, V> buildOrThrow();
        public native TMap<K, V> buildKeepingLast();
      }
      """,
      "TList", """
      package j2go.rt;
      public final class TList<T> {
        public native int size();
        public native boolean isEmpty();
        public native T get(int i);
        public native T set(int i, T v);
        public native boolean add(T v);
        public native void insert(int i, T v);
        public native T removeAt(int i);
        public native void clear();
        public native java.util.Iterator<T> iterator();
      }
      """,
      "TSet", """
      package j2go.rt;
      public final class TSet<T> {
        public native int size();
        public native boolean isEmpty();
        public native boolean add(T v);
        public native boolean contains(T v);
        public native boolean remove(T v);
        public native void clear();
        public native java.util.Iterator<T> iterator();
      }
      """,
      "TDeque", """
      package j2go.rt;
      public final class TDeque<T> {
        public native int size();
        public native boolean isEmpty();
        public native boolean add(T v);
        public native void push(T v);
        public native T pop();
        public native T poll();
        public native T peek();
        public native T peekLast();
        public native void clear();
        public native java.util.Iterator<T> iterator();
      }
      """,
      "TMap", """
      package j2go.rt;
      public final class TMap<K, V> {
        public native int size();
        public native boolean isEmpty();
        public native V get(K k);
        public native boolean containsKey(K k);
        public native V getOrDefault(K k, V def);
        public native V put(K k, V v);
        public native V putIfAbsent(K k, V v);
        public native V computeIfAbsent(K k, java.util.function.Function<? super K, ? extends V> f);
        public native V remove(K k);
        public native void clear();
      }
      """,
      "TEntry", """
      package j2go.rt;
      public final class TEntry<K, V> {
        public native K getKey();
        public native V getValue();
        public native V setValue(V v);
      }
      """);

  /** One kind of typed collection. */
  static final class CollKind {
    final String name;  // list, set, deque, map, entry; listBuilder, setBuilder, mapBuilder
    /** Of a builder (Guava's ImmutableList.Builder): the kind of collection it builds; else null. A builder has no erased view. */
    CollKind builds;
    /** Of a builder: the static methods that make one (their qualified names), each with the suffix of its runtime function. */
    Map<String, String> factories = Map.of();
    TypeElement stub;  // TList, TSet
    final String view, viewType;  // asList, java.util.List
    /** The type of the argument of `new C<>(from)`: a Collection, or a Map for a map. */
    String fromType = "java.util.Collection";
    /** The types of the declarations that can hold one. */
    final Set<String> declared;
    /** The classes whose `new` makes one, each with the name its runtime functions carry (newT<x>_, t<x>From_, t<x>Copy_). */
    final Map<String, String> made;
    /** The types of the places that its erased view serves. */
    final Set<String> viewTypes;
    /**
     * Guava's immutable class of the kind (ImmutableList for a list), or null: a declaration of
     * that type is one of the kind too, holding the typed collection with its `twin` set
     * (rt_timm.go), and an expression of that static type enters a typed declaration through
     * t<kind>Imm_ (the typed collection of the erased object, made once).
     */
    String immClass;
    /** The classes that an immutable typed collection is seen as through asImm() (immClass and its superclasses). */
    Set<String> immTypes = Set.of();
    /**
     * The sorted class of the kind (TreeSet for a set), or null: `new TreeSet<>(..)` makes the
     * typed collection in its sorted mode (rt_tsorted.go), and a declaration of that type is one
     * of the kind.
     */
    String sortedClass;
    /** The interfaces that a sorted typed collection is seen as through asSorted() (SortedSet, NavigableSet). */
    Set<String> sortedTypes = Set.of();

    CollKind(String name, String view, String viewType, Set<String> declared, Map<String, String> made, Set<String> viewTypes) {
      this.name = name;
      this.view = view;
      this.viewType = viewType;
      this.declared = new HashSet<>(declared);
      this.made = new HashMap<>(made);
      this.viewTypes = new HashSet<>(viewTypes);
    }

    /** Whether te is an interface that a sorted collection of this kind is seen as through asSorted(). */
    boolean sortedType(TypeElement te) {
      return te != null && sortedTypes.contains(te.getQualifiedName().toString());
    }

    /** Whether the kind has two type arguments (a key and a value). */
    boolean pair() {
      return name.startsWith("map") || name.equals("entry");
    }

    /** Whether the kind compares its (first) type argument: the elements of a set, the keys of a map. */
    boolean compares() {
      return name.startsWith("set") || name.startsWith("map");
    }

    /** The suffix of the runtime function of the factory method m of a builder kind ("" or "N"), or null. */
    String factory(Element m) {
      if (!(m instanceof ExecutableElement me) || !me.getModifiers().contains(Modifier.STATIC)) return null;
      return factories.get(((TypeElement) me.getEnclosingElement()).getQualifiedName() + "." + me.getSimpleName());
    }

    /** Whether te is a class that an immutable collection of this kind is seen as through asImm(). */
    boolean immType(TypeElement te) {
      return te != null && immTypes.contains(te.getQualifiedName().toString());
    }
  }

  final List<CollKind> collKinds = new ArrayList<>();
  /** The kind of the entries of typed maps (TEntry), or null. */
  CollKind entryKind;

  void registerTypedColls() {
    CollKind list = new CollKind("list", "asList", "java.util.List", Set.of("java.util.List", "java.util.ArrayList"),
        Map.of("java.util.ArrayList", "List"),
        Set.of("java.util.List", "java.util.Collection", "java.lang.Iterable", "java.util.SequencedCollection", "java.lang.Object"));
    CollKind set = new CollKind("set", "asSet", "java.util.Set", Set.of("java.util.Set", "java.util.HashSet", "java.util.LinkedHashSet"),
        Map.of("java.util.LinkedHashSet", "Set"),
        Set.of("java.util.Set", "java.util.Collection", "java.lang.Iterable", "java.lang.Object"));
    CollKind deque = new CollKind("deque", "asDeque", "java.util.Deque", Set.of("java.util.Deque", "java.util.ArrayDeque", "java.util.Queue"),
        Map.of("java.util.ArrayDeque", "Deque"),
        Set.of("java.util.Deque", "java.util.Queue", "java.util.Collection", "java.lang.Iterable", "java.util.SequencedCollection", "java.lang.Object"));
    CollKind map = new CollKind("map", "asMap", "java.util.Map", Set.of("java.util.Map", "java.util.HashMap", "java.util.LinkedHashMap"),
        Map.of("java.util.LinkedHashMap", "Map"),
        Set.of("java.util.Map", "java.util.SequencedMap", "java.lang.Object"));
    map.fromType = "java.util.Map";
    // the entry of a typed map: the variable of a for-each loop over map.entrySet() (Colls)
    CollKind entry = new CollKind("entry", "asEntry", "java.util.Map.Entry", Set.of(), Map.of(), Set.of("java.util.Map.Entry", "java.lang.Object"));
    list.stub = elems.getTypeElement("j2go.rt.TList");
    set.stub = elems.getTypeElement("j2go.rt.TSet");
    deque.stub = elems.getTypeElement("j2go.rt.TDeque");
    map.stub = elems.getTypeElement("j2go.rt.TMap");
    entry.stub = elems.getTypeElement("j2go.rt.TEntry");
    boolean nomaps = "1".equals(System.getenv("J2GO_COLLS_NOMAPS"));
    List<CollKind> kinds = new ArrayList<>(List.of(list, set, deque, map, entry));
    if (!"1".equals(System.getenv("J2GO_COLLS_NOSORTED"))) {
      for (CollKind k : List.of(set, map)) {
        String cap = k == set ? "Set" : "Map";
        k.sortedClass = "java.util.Tree" + cap;
        k.sortedTypes = Set.of("java.util.Sorted" + cap, "java.util.Navigable" + cap);
        k.declared.add(k.sortedClass);
        k.declared.addAll(k.sortedTypes);
        k.made.put(k.sortedClass, "Sorted" + cap);
        k.viewTypes.addAll(k.sortedTypes);
        if (k == set) k.viewTypes.add("java.util.SequencedSet");
      }
    }
    if (!"1".equals(System.getenv("J2GO_COLLS_NOIMM"))) {
      String g = "com.google.common.collect.";
      for (CollKind k : List.of(list, set, map)) {
        String cap = Character.toUpperCase(k.name.charAt(0)) + k.name.substring(1);
        k.immClass = g + "Immutable" + cap;
        k.immTypes = k == map ? Set.of(k.immClass) : Set.of(k.immClass, g + "ImmutableCollection");
        k.declared.add(k.immClass);
        k.viewTypes.addAll(k.immTypes);
        if ("1".equals(System.getenv("J2GO_COLLS_NOBUILDERS"))) continue;
        // its builder: typed where every use is add, addAll, put, putAll or a build (Colls)
        CollKind b = new CollKind(k.name + "Builder", null, null, Set.of(k.immClass + ".Builder"), Map.of(k.immClass + ".Builder", cap + "Builder"), Set.of());
        b.builds = k;
        b.factories = Map.of(k.immClass + ".builder", "", k.immClass + ".builderWithExpectedSize", "N");
        b.stub = elems.getTypeElement("j2go.rt.T" + cap + "Builder");
        kinds.add(b);
      }
    }
    for (CollKind k : kinds) {
      if (k.stub == null) continue;
      if (k == set && "1".equals(System.getenv("J2GO_COLLS_NOSETS"))) continue;
      if (k == deque && "1".equals(System.getenv("J2GO_COLLS_NODEQUES"))) continue;
      if ((k == map || k == entry) && nomaps) continue;
      if (k == entry) entryKind = k;
      collKinds.add(k);
      typedClasses.add(k.stub);
      keptVars.addAll(k.stub.getTypeParameters());
    }
  }

  /**
   * Library methods that make a list for their caller alone, each with the runtime function
   * that makes the typed list instead (its arguments: the conversion, the receiver, the
   * method's arguments) and the list's element type: Splitter.splitToList is splitToListT_.
   */
  static final Map<String, String[]> TYPED_MAKERS = Map.of(
      "com.google.common.base.Splitter.splitToList", new String[] {"splitToListT_", "java.lang.String"});

  /** The entry of TYPED_MAKERS for method m where it makes a collection for kind k with type arguments args; else null. */
  String[] typedMaker(Element m, CollKind k, List<TypeMirror> args) {
    if (!(m instanceof ExecutableElement me) || !k.name.equals("list") || args.size() != 1 || "1".equals(System.getenv("J2GO_COLLS_NOMAKERS"))) return null;
    String[] tm = TYPED_MAKERS.get(((TypeElement) me.getEnclosingElement()).getQualifiedName() + "." + me.getSimpleName());
    return tm != null && types.isSameType(args.get(0), elems.getTypeElement(tm[1]).asType()) ? tm : null;
  }

  /** The kind of typed collection that type t (TList<X>, TSet<X>) is, or null. */
  CollKind typedColl(TypeMirror t) {
    if (t == null || t.getKind() != TypeKind.DECLARED) return null;
    TypeElement te = typeElement(t);
    for (CollKind k : collKinds) if (k.stub.equals(te)) return k;
    return null;
  }

  boolean isTypedColl(TypeMirror t) {
    return typedColl(t) != null;
  }

  /** The kind of typed collection that a declaration of type t (List<X>, HashSet<X>) can hold, or null. */
  CollKind collKindOf(TypeMirror t) {
    if (t == null || t.getKind() != TypeKind.DECLARED) return null;
    String qn = typeElement(t).getQualifiedName().toString();
    for (CollKind k : collKinds) if (k.declared.contains(qn)) return k;
    return null;
  }

  /**
   * The type arguments of the typed collection that a declaration of collection type t can
   * hold (the element type; the key and value types of a map), or null if it can hold none.
   */
  List<TypeMirror> typedArgs(TypeMirror t) {
    return typedArgs(collKindOf(t), t);
  }

  List<TypeMirror> typedArgs(CollKind ck, TypeMirror t) {
    if (ck == null || t.getKind() != TypeKind.DECLARED) return null;
    DeclaredType d = (DeclaredType) t;
    if (d.getTypeArguments().size() != (ck.pair() ? 2 : 1)) return null;
    List<TypeMirror> out = new ArrayList<>();
    for (int i = 0; i < d.getTypeArguments().size(); i++) {
      // the elements of a set and the keys of a map are compared
      TypeMirror a = typedArg(d.getTypeArguments().get(i), ck.compares() && i == 0);
      if (a == null) return null;
      out.add(a);
    }
    return out;
  }

  /**
   * The type at which a typed collection holds elements (keys, values) of declared type a, or
   * null if it cannot hold them:
   * - a String, a class or an interface without type arguments, at that type;
   * - a boxed primitive or Object, at that type (`any` in Go: the box, as in erased code);
   * - a library type with type arguments (a List<Node>), at its erasure: the erased List, as
   *   such a value is everywhere;
   * - a generic class of the program, at the Go type it has (an instantiation of a Go generic
   *   type, or its erasure), unless a type argument is a wildcard;
   * - a type variable: itself where it is a Go type parameter (not compared: its constraint is
   *   `any`), else what it is erased to.
   * An element that the collection compares (compared: the elements of a set, the keys of a
   * map) has no type arguments and is not Object or a floating-point box. Where Go's == is
   * Java's equals for its type (a String, an enum, a class of the program without equals of
   * its own: goEquals; a boxed primitive: the boxes of two equal values of one type are == as
   * Go interfaces) the collection compares with ==; for any other class or interface it
   * compares as java.util.HashMap does, by hashCode and equals (byEquals).
   */
  TypeMirror typedArg(TypeMirror a, boolean compared) {
    boolean simple = !"1".equals(System.getenv("J2GO_COLLS_SIMPLE"));
    boolean generic = simple && !"1".equals(System.getenv("J2GO_COLLS_NOGENERIC"));
    if (a.getKind() == TypeKind.TYPEVAR && generic) {
      // a Go type parameter (its constraint is `comparable`: a set or a map can compare it)
      if (typedVar(a)) return compared && !constraint((TypeParameterElement) ((TypeVariable) a).asElement()).contains("comparable") ? null : a;
      // an erased type variable: what it is erased to (its bound)
      TypeMirror e = erase(a);
      return e.getKind() == TypeKind.DECLARED ? typedArg(e, compared) : null;
    }
    if (a.getKind() == TypeKind.WILDCARD && generic && !"1".equals(System.getenv("J2GO_COLLS_NOWILDCARDS"))) {
      // `? extends X`: the collection of X that the declaration reads (what enters it is a
      // collection made for a declaration with the same element type, so it is one of X)
      TypeMirror b = ((WildcardType) a).getExtendsBound();
      return b == null ? null : typedArg(b, compared);
    }
    if (a.getKind() != TypeKind.DECLARED) return null;
    TypeElement ae = typeElement(a);
    if (ae.getQualifiedName().contentEquals("java.lang.Object")) return compared || !simple ? null : a;
    TypeMirror prim = unboxed(a);
    if (prim != null) {
      if (!simple || compared && (prim.getKind() == TypeKind.DOUBLE || prim.getKind() == TypeKind.FLOAT)) return null;
      return a;
    }
    K k = kind(a);
    if (k != K.STR && k != K.CLS && k != K.IFACE) return null;
    if (isSource(ae) && (ae.getNestingKind() == NestingKind.LOCAL || ae.getNestingKind() == NestingKind.ANONYMOUS)) return null;
    if (isValueLib(ae)) return null;
    if (!((DeclaredType) a).getTypeArguments().isEmpty() || !ae.getTypeParameters().isEmpty()) {
      if (!simple || typedClasses.contains(ae) && !isSource(ae)) return null;
      if (isSource(ae)) {
        // a generic class of the program: the Go type it has (an instantiation of a Go generic
        // type, or its erasure), unless a type argument is a wildcard
        if (!generic || hasWildcard(a)) return null;
        return typedDecl(erase(a)) ? a : types.erasure(a);
      }
      // a library type: erased whatever its type arguments are
      if (compared) return null;
      if (!goType(a).equals(goType(types.erasure(a)))) return null;
      return types.erasure(a);
    }
    if (compared && !goEquals(ae) && "1".equals(System.getenv("J2GO_COLLS_NOEQUALS"))) return null;
    return a;
  }

  /**
   * Whether a typed set or map compares elements of type a (one that typedArg accepts as a
   * compared type) by equals and hashCode instead of Go's ==: a class with its own equals, an
   * interface, a library class.
   */
  boolean byEquals(TypeMirror a) {
    // a type parameter: decided for each instantiation when its conversion is made (tceT_)
    if (a.getKind() == TypeKind.TYPEVAR) return true;
    if (unboxed(a) != null) return false;
    return !goEquals(typeElement(a));
  }

  private boolean hasWildcard(TypeMirror t) {
    if (t.getKind() == TypeKind.WILDCARD) return true;
    if (t.getKind() == TypeKind.ARRAY) return hasWildcard(((ArrayType) t).getComponentType());
    if (t.getKind() != TypeKind.DECLARED) return false;
    for (TypeMirror x : ((DeclaredType) t).getTypeArguments()) if (hasWildcard(x)) return true;
    return false;
  }

  /**
   * Whether call t of method m is Collections.unmodifiableList / Set / Map: the runtime's is its
   * argument itself (rt_coll.go), so a typed collection passes through it as it is.
   */
  static boolean isUnmodifiable(Element m, com.sun.source.tree.MethodInvocationTree t) {
    if (!(m instanceof ExecutableElement me) || t.getArguments().size() != 1) return false;
    String n = me.getSimpleName().toString();
    return (n.equals("unmodifiableList") || n.equals("unmodifiableSet") || n.equals("unmodifiableMap"))
        && ((TypeElement) me.getEnclosingElement()).getQualifiedName().contentEquals("java.util.Collections");
  }

  /** The type variables that are Go type parameters among those that t names, in order. */
  void tvarsOf(TypeMirror t, Set<TypeParameterElement> out) {
    if (t == null) return;
    if (t.getKind() == TypeKind.TYPEVAR) {
      if (typedVar(t)) out.add((TypeParameterElement) ((TypeVariable) t).asElement());
    } else if (t.getKind() == TypeKind.ARRAY) {
      tvarsOf(((ArrayType) t).getComponentType(), out);
    } else if (t.getKind() == TypeKind.DECLARED) {
      for (TypeMirror x : ((DeclaredType) t).getTypeArguments()) tvarsOf(x, out);
    }
  }

  /** Whether the Go type of t names a Go type parameter (a type variable that is kept). */
  boolean open(TypeMirror t) {
    if (t == null) return false;
    if (t.getKind() == TypeKind.TYPEVAR) return typedVar(t);
    if (t.getKind() == TypeKind.ARRAY) return open(((ArrayType) t).getComponentType());
    if (t.getKind() != TypeKind.DECLARED) return false;
    for (TypeMirror x : ((DeclaredType) t).getTypeArguments()) if (open(x)) return true;
    return false;
  }

  private final Map<TypeElement, Boolean> goEqualsCache = new HashMap<>();

  /**
   * Whether Go's == on values of class te is Java's equals (and so finds an element of a hash
   * set exactly when Java does): a String, an enum, or a class of the program (a handle class
   * too: its number) that inherits Object's equals and hashCode, as every subclass of it does.
   */
  boolean goEquals(TypeElement te) {
    if (te.getQualifiedName().contentEquals("java.lang.String")) return true;
    if (!isSource(te)) return false;
    Boolean c = goEqualsCache.get(te);
    if (c != null) return c;
    boolean ok = te.getKind() == ElementKind.ENUM || te.getKind() == ElementKind.CLASS && !ownEquals(te, true) && !ownEquals(te, false);
    goEqualsCache.put(te, ok);
    return ok;
  }

  /** Whether te, a superclass of it (up) or a subclass of it (down) declares equals or hashCode, or is not a class of the program. */
  private boolean ownEquals(TypeElement te, boolean up) {
    if (te.getQualifiedName().contentEquals("java.lang.Object")) return false;
    if (!isSource(te) || te.getKind() != ElementKind.CLASS) return true;
    for (Element e : te.getEnclosedElements()) {
      if (e instanceof ExecutableElement m && !m.getModifiers().contains(Modifier.STATIC)
          && (m.getSimpleName().contentEquals("equals") && m.getParameters().size() == 1 || m.getSimpleName().contentEquals("hashCode") && m.getParameters().isEmpty())) return true;
    }
    if (up) {
      TypeElement sup = superclass(te);
      return sup != null && ownEquals(sup, true);
    }
    for (TypeElement sub : subclasses.getOrDefault(te, List.of())) if (ownEquals(sub, false)) return true;
    return false;
  }

  /** The typed collection type of kind k with type arguments args: TList<elem>, TMap<key, value>. */
  TypeMirror typedOf(CollKind k, List<TypeMirror> args) {
    return types.getDeclaredType(k.stub, args.toArray(new TypeMirror[0]));
  }

  /** The type the erased view of typed collection type t has: List<X> for a TList<X>. */
  TypeMirror typedView(TypeMirror t) {
    return types.getDeclaredType(elems.getTypeElement(typedColl(t).viewType), ((DeclaredType) t).getTypeArguments().toArray(new TypeMirror[0]));
  }

  /**
   * The typed method of typed collection type recv that implements method m of its Java type,
   * called with arguments of types args; null if the view serves the call.
   */
  ExecutableElement typedMethod(TypeMirror recv, ExecutableElement m, List<TypeMirror> args) {
    CollKind ck = typedColl(recv);
    String n = m.getSimpleName().toString();
    int np = m.getParameters().size();
    boolean int0 = np > 0 && m.getParameters().get(0).asType().getKind() == TypeKind.INT;
    String tn = null;
    if (ck.builds != null) {
      // a builder: the methods that take the elements, and the builds
      TypeMirror p0 = np == 1 ? types.erasure(m.getParameters().get(0).asType()) : null;
      String p0n = p0 != null && p0.getKind() == TypeKind.DECLARED ? typeElement(p0).getQualifiedName().toString() : "";
      switch (n + "/" + np) {
        case "add/1": tn = m.isVarArgs() || ck.pair() ? null : n; break;
        case "addAll/1": tn = p0n.equals("java.lang.Iterable") && !ck.pair() ? n : null; break;
        case "put/2": tn = ck.pair() ? n : null; break;
        case "putAll/1": tn = p0n.equals("java.util.Map") && ck.pair() ? n : null; break;
        case "build/0": tn = n; break;
        case "buildOrThrow/0", "buildKeepingLast/0": tn = ck.pair() ? n : null; break;
        default: break;
      }
    } else if (ck.name.equals("list")) {
      switch (n + "/" + np) {
        case "size/0", "isEmpty/0", "clear/0", "set/2", "iterator/0", "add/1": tn = n; break;
        case "get/1": tn = int0 ? n : null; break;
        case "add/2": tn = "insert"; break;
        case "remove/1": tn = int0 ? "removeAt" : null; break;
        default: break;
      }
    } else if (ck.name.equals("deque")) {
      // the head is the first item, as in the erased ArrayDeque (rt_coll.go)
      switch (n + "/" + np) {
        case "size/0", "isEmpty/0", "clear/0", "iterator/0", "add/1", "push/1", "pop/0", "poll/0", "peek/0", "peekLast/0": tn = n; break;
        case "addLast/1": tn = "add"; break;
        case "addFirst/1": tn = "push"; break;
        case "removeFirst/0", "remove/0": tn = "pop"; break;
        case "pollFirst/0": tn = "poll"; break;
        case "peekFirst/0": tn = "peek"; break;
        default: break;
      }
    } else if (ck.name.equals("map")) {
      // get, containsKey and remove take an Object: typed when the argument is a key by its static type
      TypeMirror key = ((DeclaredType) recv).getTypeArguments().get(0);
      boolean keyArg = args != null && args.size() >= 1 && isElement(args.get(0), key);
      switch (n + "/" + np) {
        case "size/0", "isEmpty/0", "clear/0", "put/2", "putIfAbsent/2": tn = n; break;
        case "get/1": tn = keyArg ? "get" : null; break;
        case "containsKey/1": tn = keyArg ? n : null; break;
        case "remove/1": tn = keyArg ? n : null; break;
        case "getOrDefault/2": tn = keyArg ? n : null; break;
        case "computeIfAbsent/2": tn = keyArg ? n : null; break;
        default: break;
      }
    } else if (ck.name.equals("entry")) {
      switch (n + "/" + np) {
        case "getKey/0", "getValue/0", "setValue/1": tn = n; break;
        default: break;
      }
    } else {
      // contains and remove take an Object: typed when the argument is an element by its static type
      TypeMirror elem = ((DeclaredType) recv).getTypeArguments().get(0);
      boolean elemArg = args != null && args.size() == 1 && isElement(args.get(0), elem);
      switch (n + "/" + np) {
        case "size/0", "isEmpty/0", "clear/0", "iterator/0", "add/1": tn = n; break;
        case "contains/1": tn = elemArg ? n : null; break;
        case "remove/1": tn = elemArg ? n : null; break;
        default: break;
      }
    }
    if (tn == null) return null;
    for (Element e : ck.stub.getEnclosedElements()) {
      if (e instanceof ExecutableElement t && t.getSimpleName().contentEquals(tn)) return t;
    }
    return null;
  }

  /** Whether an argument of static type arg is an element of type elem: null, a subtype, or the primitive of the box. */
  boolean isElement(TypeMirror arg, TypeMirror elem) {
    if (arg == null) return false;
    if (arg.getKind() == TypeKind.NULL) return true;
    if (arg.getKind().isPrimitive()) return types.isSameType(types.boxedClass((PrimitiveType) arg).asType(), elem);
    return types.isSubtype(types.erasure(arg), types.erasure(elem));
  }

  /** The conversions of the type arguments of typed collection type t, as the arguments of a runtime function: "tc_string, tc_p_Node". */
  String tconvsOf(TypeMirror t) {
    String tc = "";
    CollKind ck = typedColl(t);
    for (TypeMirror a : ((DeclaredType) t).getTypeArguments()) {
      // the elements of a set and the keys of a map are compared
      boolean compared = tc.isEmpty() && ck.compares();
      tc += (tc.isEmpty() ? "" : ", ") + tconv(a, compared && byEquals(a));
    }
    return tc;
  }

  /** The element conversions of typed collections: name -> declaration (g_0casts.go). */
  final TreeMap<String, String> tconvs = new TreeMap<>();

  /** The name of the conversion of element type elem (tc_string, tc_p_JSType), declared on first use. */
  String tconv(TypeMirror elem) {
    return tconv(elem, false);
  }

  /**
   * The conversion of element type elem; eq: for elements that a set or a map compares by
   * equals and hashCode (tce_p_JSType).
   */
  String tconv(TypeMirror elem, boolean eq) {
    String g = goType(elem);
    // a type that names a Go type parameter: the conversions of a value of a type parameter
    // (boxT_, unboxT_), one object per instantiation (rt_tlist.go)
    if (kind(elem) == K.TVAR) return (eq ? "tceT_[" : "tcT_[") + g + "]()";
    if (open(elem)) {
      // an instantiation of a generic class of the program that names type parameters: a generic
      // function with the conversions the erased code makes for the class, one object per
      // instantiation (single_)
      LinkedHashSet<TypeParameterElement> tps = new LinkedHashSet<>();
      tvarsOf(elem, tps);
      String decl = "", use = "";
      for (TypeParameterElement tp : tps) {
        decl += (decl.isEmpty() ? "" : ", ") + tvName(tp) + " " + constraint(tp);
        use += (use.isEmpty() ? "" : ", ") + tvName(tp);
      }
      String fn = (eq ? "tce_" : "tc_") + g.replace("*", "p_").replaceAll("[^A-Za-z0-9_]", "_");
      if (!tconvs.containsKey(fn)) {
        TypeMirror object = elems.getTypeElement("java.lang.Object").asType();
        tconvs.put(fn, "");
        // a null pointer is null (its self() would be called through the superclass it embeds)
        String guard = g.startsWith("*") ? "if v == nil {\n\t\t\t\t\treturn nil\n\t\t\t\t}\n\t\t\t\t" : "";
        tconvs.put(fn, "var " + fn + "_single_ singles_\n\nfunc " + fn + "[" + decl + "]() *tconv_[" + g + "] {\n\treturn single_(&" + fn + "_single_, func() *tconv_[" + g + "] {\n\t\treturn &tconv_["
            + g + "]{\n\t\t\tbox: func(v " + g + ") any {\n\t\t\t\t" + guard + "return " + coerce("v", elem, object) + "\n\t\t\t},\n\t\t\tunbox: func(v any) " + g + " { return "
            + coerce("v", object, elem) + " },\n" + (eq ? "\t\t\tbyEquals: true,\n" : "") + "\t\t}\n\t})\n}\n");
      }
      return fn + "[" + use + "]()";
    }
    // a type that is `any` in Go (Object, a boxed primitive) has the conversion of its Java type
    String name = (eq ? "tce_" : "tc_") + (kind(elem) == K.ANY ? className(typeElement(elem)) : g.replace("*", "p_").replaceAll("[^A-Za-z0-9_]", "_"));
    if (!tconvs.containsKey(name)) {
      TypeMirror object = elems.getTypeElement("java.lang.Object").asType();
      tconvs.put(name, "");
      tconvs.put(name, "var " + name + " = &tconv_[" + g + "]{\n\tbox:   func(v " + g + ") any { return " + coerce("v", elem, object)
          + " },\n\tunbox: func(v any) " + g + " { return " + coerce("v", object, elem) + " },\n"
          + (kind(elem) == K.STR ? "\tnull:  nullstr_,\n" : "") + (eq ? "\tbyEquals: true,\n" : "") + "}\n");
    }
    return name;
  }

  /** A parameterized type of a typed class. */
  boolean typedDecl(TypeMirror t) {
    return t != null && t.getKind() == TypeKind.DECLARED && typedClasses.contains(typeElement(t)) && !((DeclaredType) t).getTypeArguments().isEmpty();
  }

  /** The Go name of a type parameter: its Java name, unless a Go type of the package has it. */
  String tvName(TypeParameterElement tp) {
    String n = tp.getSimpleName().toString();
    return classByName.containsKey(n) ? n + "_" : n;
  }

  String tvName(TypeMirror t) {
    return tvName((TypeParameterElement) ((TypeVariable) t).asElement());
  }

  /** The Go type parameter list for the kept ones of tps: "[K any, V any]", or "". */
  String typeParamsDecl(List<? extends TypeParameterElement> tps0) {
    List<TypeParameterElement> tps = keptOf(tps0);
    if (tps.isEmpty()) return "";
    StringBuilder sb = new StringBuilder("[");
    for (int i = 0; i < tps.size(); i++) sb.append(i > 0 ? ", " : "").append(tvName(tps.get(i))).append(" ").append(constraint(tps.get(i)));
    return sb.append("]").toString();
  }

  /** The Go type arguments naming the kept ones of tps themselves: "[K, V]", or "". */
  String typeParamsUse(List<? extends TypeParameterElement> tps0) {
    List<TypeParameterElement> tps = keptOf(tps0);
    if (tps.isEmpty()) return "";
    StringBuilder sb = new StringBuilder("[");
    for (int i = 0; i < tps.size(); i++) sb.append(i > 0 ? ", " : "").append(tvName(tps.get(i)));
    return sb.append("]").toString();
  }

  /** The Go type argument for Java type argument a: boxed primitives, Object and other erased types are any. */
  String goTypeArg(TypeMirror a) {
    if (a == null) return "any";
    if (typedVar(a)) return tvName(a);
    if (a.getKind() == TypeKind.TYPEVAR && ((TypeVariable) a).asElement() instanceof TypeParameterElement tp && fixedVars.contains(tp)) {
      return goTypeArg(types.erasure(tp.getBounds().get(0)));
    }
    switch (a.getKind()) {
      case TYPEVAR, WILDCARD, INTERSECTION, UNION: return "any";
      default: break;
    }
    K k = kind(a);
    if (k == K.ANY || k == K.NULL || k == K.VOID) return "any";
    return goType(a);
  }

  /** "[any, any]": the type arguments of a raw use of value library class te. */
  String rawArgs(TypeElement te) {
    StringBuilder sb = new StringBuilder("[");
    for (int i = 0; i < te.getTypeParameters().size(); i++) sb.append(i > 0 ? ", any" : "any");
    return sb.append("]").toString();
  }

  /** "[X, Y]" for the type arguments of typed type t at its declaration's kept type parameters. */
  String typeArgsOf(TypeMirror t) {
    StringBuilder sb = new StringBuilder("[");
    List<? extends TypeMirror> as = ((DeclaredType) t).getTypeArguments();
    List<? extends TypeParameterElement> ps = typeElement(t).getTypeParameters();
    boolean first = true;
    for (int i = 0; i < as.size(); i++) {
      if (!keptVars.contains(ps.get(i))) continue;
      sb.append(first ? "" : ", ").append(goTypeArg(as.get(i)));
      first = false;
    }
    return sb.append("]").toString();
  }

  /** Library types that are represented as Go `any`. */
  static final Set<String> ANY_TYPES = Set.of(
      "java.lang.Object", "java.lang.Number", "java.lang.Integer", "java.lang.Long", "java.lang.Short",
      "java.lang.Byte", "java.lang.Character", "java.lang.Boolean", "java.lang.Double", "java.lang.Float",
      "java.lang.Void", "java.lang.CharSequence", "java.lang.Comparable", "java.lang.Cloneable",
      "java.io.Serializable", "java.lang.Record", "java.lang.constant.Constable",
      "java.lang.constant.ConstantDesc");

  /** Library classes written to be extended: they carry self_ and as_X() like translated classes. */
  static final Set<String> EXTENDABLE = new HashSet<>(List.of(
      "java.lang.Throwable", "java.lang.Exception", "java.lang.RuntimeException", "java.lang.Error",
      "java.io.IOException", "java.lang.IllegalStateException", "java.lang.IllegalArgumentException",
      "java.lang.UnsupportedOperationException", "java.util.AbstractSet", "java.util.AbstractMap",
      "java.util.AbstractList", "java.util.AbstractCollection", "java.util.AbstractQueue",
      "com.google.common.collect.AbstractIterator", "com.google.common.collect.Ordering",
      "com.google.common.collect.ForwardingMap", "com.google.common.collect.ForwardingSet",
      "java.io.OutputStream", "java.io.InputStream", "java.io.Writer", "java.io.Reader",
      "java.io.FilterOutputStream", "java.nio.file.SimpleFileVisitor", "java.util.logging.Handler",
      "java.util.logging.Formatter", "java.util.concurrent.ThreadPoolExecutor", "java.lang.Thread",
      "org.kohsuke.args4j.spi.OptionHandler", "org.kohsuke.args4j.spi.StringOptionHandler",
      "org.kohsuke.args4j.spi.BooleanOptionHandler", "org.kohsuke.args4j.spi.IntOptionHandler",
      "org.kohsuke.args4j.spi.StopOptionHandler", "com.google.gson.TypeAdapter",
      "java.lang.Enum", "java.util.ArrayList", "com.google.gson.reflect.TypeToken",
      "com.google.common.cache.CacheLoader", "com.google.common.escape.Escaper",
      "java.util.zip.GZIPOutputStream", "java.io.FilterInputStream", "org.kohsuke.args4j.OptionDef", "com.google.gson.JsonElement"));

  static final Set<String> GO_RESERVED = new HashSet<>(List.of(
      "break", "default", "func", "interface", "select", "case", "defer", "go", "map", "struct", "chan",
      "else", "goto", "package", "switch", "const", "fallthrough", "if", "range", "type", "continue",
      "for", "import", "return", "var", "append", "bool", "byte", "cap", "clear", "close", "complex",
      "copy", "delete", "error", "false", "imag", "int", "iota", "len", "make", "max", "min", "new",
      "nil", "panic", "print", "println", "real", "recover", "rune", "string", "true", "uint", "any",
      "init", "main", "float32", "float64", "int8", "int16", "int32", "int64", "uint8", "uint16",
      "uint32", "uint64", "uintptr", "comparable", "this"));

  // ---------------------------------------------------------------- handle classes

  /** Classes whose objects are 32-bit handles into a store instead of pointers (tools/j2go/handles.txt). */
  final LinkedHashSet<TypeElement> handles = new LinkedHashSet<>();
  /** Enums that are the type of a field of a handle class: such a field holds ordinal+1. */
  final LinkedHashSet<TypeElement> handleEnums = new LinkedHashSet<>();
  /** Violations of what a handle class may be or may be used for. */
  final List<String> handleErrors = new ArrayList<>();

  boolean isHandle(TypeElement te) {
    return te != null && handles.contains(te);
  }

  boolean isHandleType(TypeMirror t) {
    t = erase(t);
    return t != null && t.getKind() == TypeKind.DECLARED && isHandle(typeElement(t));
  }

  /** The Go type of a reference to class te: a pointer to its struct, or the handle type itself. */
  String ref(TypeElement te) {
    List<TypeParameterElement> tps = goTparams(te);
    if (!tps.isEmpty()) return "*" + className(te) + tparamNames(tps);
    return isHandle(te) ? className(te) : "*" + className(te);
  }

  private final Map<TypeElement, List<TypeParameterElement>> goTparamsCache = new HashMap<>();

  /**
   * The Go type parameters of source class te: its kept own type parameters, and for a local or
   * anonymous class those of the scopes it is declared in (the kept type parameters of enclosing
   * generic methods, and of the enclosing class when it has an enclosing instance). Such a class
   * is only named inside those scopes, where the type parameters have the same names.
   */
  List<TypeParameterElement> goTparams(TypeElement te) {
    List<TypeParameterElement> c = goTparamsCache.get(te);
    if (c != null) return c;
    List<TypeParameterElement> out = new ArrayList<>();
    if (te.getNestingKind() == NestingKind.LOCAL || te.getNestingKind() == NestingKind.ANONYMOUS) {
      for (Element e = te.getEnclosingElement(); e != null; e = e.getEnclosingElement()) {
        if (e instanceof ExecutableElement m) out.addAll(0, keptOf(m.getTypeParameters()));
        if (e instanceof TypeElement o) {
          TypeElement lo = lexicalOuter.getOrDefault(te, o);
          if (localWithOuter.contains(te)) out.addAll(0, goTparams(lo));
          break;
        }
      }
    }
    out.addAll(keptOf(te.getTypeParameters()));
    goTparamsCache.put(te, out);
    return out;
  }

  /** The Go struct type that class te embeds for its superclass (or ancestor) a: a typed one at te's view of it. */
  String superStruct(TypeElement te, TypeElement a) {
    if (typedClasses.contains(a)) return className(a) + viewArgs(te.asType(), a);
    if (isSource(a) && !goTparams(a).isEmpty()) return className(a) + goTparamsUse(a);
    return className(a);
  }

  /** "[K any, V any]" for goTparams(te), or "". */
  String goTparamsDecl(TypeElement te) {
    List<TypeParameterElement> tps = goTparams(te);
    if (tps.isEmpty()) return "";
    StringBuilder sb = new StringBuilder("[");
    for (int i = 0; i < tps.size(); i++) sb.append(i > 0 ? ", " : "").append(tvName(tps.get(i))).append(" ").append(constraint(tps.get(i)));
    return sb.append("]").toString();
  }

  /** "[K, V]" for goTparams(te), or "". */
  String goTparamsUse(TypeElement te) {
    return tparamNames(goTparams(te));
  }

  String tparamNames(List<TypeParameterElement> tps) {
    if (tps.isEmpty()) return "";
    StringBuilder sb = new StringBuilder("[");
    for (int i = 0; i < tps.size(); i++) sb.append(i > 0 ? ", " : "").append(tvName(tps.get(i)));
    return sb.append("]").toString();
  }

  /** Go spelling of Java null at type t. */
  String nil(TypeMirror t) {
    if (kind(t) == K.STR) return "nullstr_";
    if (kind(t) == K.TVAR) return "null_[" + goType(t) + "]()";
    if (isValueLibType(erase(t))) return goType(t) + "{}";
    return isHandleType(t) ? "0" : "nil";
  }

  static List<VariableElement> instanceFields(TypeElement te) {
    List<VariableElement> out = new ArrayList<>();
    for (Element e : te.getEnclosedElements()) {
      if (e.getKind() == ElementKind.FIELD && !e.getModifiers().contains(Modifier.STATIC)) out.add((VariableElement) e);
    }
    return out;
  }

  /** The enum a handle-class field of type t stores by ordinal, or null. */
  TypeElement storedEnum(TypeMirror t) {
    TypeElement te = kind(t) == K.CLS ? typeElement(erase(t)) : null;
    return te != null && te.getKind() == ElementKind.ENUM && isSource(te) ? te : null;
  }

  /** True if field f of a handle class holds no Go pointer: it lives in the record the collector never scans. */
  boolean pointerFree(VariableElement f) {
    return kind(f.asType()) == K.PRIM || isHandleType(f.asType()) || storedEnum(f.asType()) != null;
  }

  /** Go type of the stored form of handle-class field f. */
  String storedType(VariableElement f) {
    TypeElement en = storedEnum(f.asType());
    if (en == null) return declType(f.asType());
    return enumConstants(en) < 256 ? "uint8" : "uint16";
  }

  static int enumConstants(TypeElement en) {
    int n = 0;
    for (Element e : en.getEnclosedElements()) if (e.getKind() == ElementKind.ENUM_CONSTANT) n++;
    return n;
  }

  /**
   * Fields of handle classes, declared Object, that have a second slot of Go type string (handles.txt,
   * "Class::field string-slot"): a String stored in such a field goes to the string slot unboxed.
   */
  final Set<VariableElement> strSlots = new HashSet<>();

  boolean strSlot(VariableElement f) {
    return strSlots.contains(f);
  }

  /**
   * Pointer-free fields of handle classes that the program reads rarely (handles.txt, "Class::field cold"):
   * they go to a second pointer-free record, so that the first one stays small.
   */
  final Set<VariableElement> coldFields = new HashSet<>();

  boolean cold(VariableElement f) {
    return coldFields.contains(f);
  }

  /** The Go lvalue of the string slot of field f of recv: a column of its own. */
  String strSlotLoc(String recv, VariableElement f) {
    return "(*" + recv + "." + fieldName(f) + "_s_())";
  }

  /** The storage location of instance field f of the object recv (a Go lvalue). */
  String fieldLoc(String recv, VariableElement f) {
    if (!isHandle(ownerOf(f))) return recv + "." + fieldName(f);
    return recv + (cold(f) ? ".cs_()." : pointerFree(f) ? ".ns_()." : ".ps_().") + fieldName(f);
  }

  /** Go expression reading instance field f of recv, at the Go type of f's Java type. */
  String fieldGet(String recv, VariableElement f) {
    if (strSlot(f)) return recv + "." + fieldName(f) + "_get_()";
    String loc = fieldLoc(recv, f);
    TypeElement en = isHandle(ownerOf(f)) ? storedEnum(f.asType()) : null;
    return en == null ? loc : className(en) + "_byOrd1_[" + loc + "]";
  }

  /** Go statement storing value (of the Go type of f's Java type) into instance field f of recv. */
  String fieldSet(String recv, VariableElement f, String value) {
    if (strSlot(f)) return recv + "." + fieldName(f) + "_set_(" + value + ")";
    return fieldLoc(recv, f) + " = " + stored(f, value);
  }

  String stored(VariableElement f, String value) {
    TypeElement en = f.getModifiers().contains(Modifier.STATIC) || !isHandle(ownerOf(f)) ? null : storedEnum(f.asType());
    return en == null ? value : className(en) + "_ord1_(" + value + ")";
  }

  /** Registers the classes named in handles.txt and checks that each may be a handle class. */
  void registerHandles(List<String> names) {
    List<String> slots = new ArrayList<>();
    for (String qn : names) {
      if (qn.contains("::")) {
        slots.add(qn);
        continue;
      }
      TypeElement te = elems.getTypeElement(qn);
      if (te == null || !isSource(te)) {
        handleErrors.add(qn + ": not a class of the translated sources");
        continue;
      }
      TypeElement sup = typeElement(te.getSuperclass());
      if (te.getKind() != ElementKind.CLASS) handleErrors.add(qn + ": is " + te.getKind().toString().toLowerCase() + ", not a plain class");
      else if (sup == null || !sup.getQualifiedName().contentEquals("java.lang.Object")) handleErrors.add(qn + ": extends " + sup);
      if (subclasses.containsKey(te)) handleErrors.add(qn + ": has subclasses: " + subclasses.get(te));
      if (te.getNestingKind() != NestingKind.TOP_LEVEL && !te.getModifiers().contains(Modifier.STATIC)) {
        handleErrors.add(qn + ": is an inner or local class (it would need an enclosing instance)");
      }
      for (Element e : te.getEnclosedElements()) {
        if (e.getKind() == ElementKind.METHOD && e.getSimpleName().contentEquals("finalize") && ((ExecutableElement) e).getParameters().isEmpty()) {
          handleErrors.add(qn + ": declares finalize(); handles are never collected");
        }
      }
      handles.add(te);
    }
    for (TypeElement te : handles) {
      for (VariableElement f : instanceFields(te)) {
        TypeElement en = storedEnum(f.asType());
        if (en != null) handleEnums.add(en);
      }
    }
    for (String line : slots) {
      java.util.regex.Matcher m = java.util.regex.Pattern.compile("(\\S+)::(\\w+)\\s+(string-slot|cold)").matcher(line);
      TypeElement te = m.matches() ? elems.getTypeElement(m.group(1)) : null;
      VariableElement field = null;
      if (te != null && handles.contains(te)) {
        for (VariableElement f : instanceFields(te)) if (f.getSimpleName().contentEquals(m.group(2))) field = f;
      }
      if (field == null) handleErrors.add(line + ": not \"Class::field string-slot\" or \"Class::field cold\" naming a field of a handle class");
      else if (m.group(3).equals("cold")) {
        if (!pointerFree(field)) handleErrors.add(line + ": a cold field must be pointer-free");
        else coldFields.add(field);
      } else if (!types.isSameType(field.asType(), elems.getTypeElement("java.lang.Object").asType())) handleErrors.add(line + ": the field is not declared Object");
      else strSlots.add(field);
    }
  }

  Ctx(Trees trees, Types types, Elements elems) {
    this.trees = trees;
    this.types = types;
    this.elems = elems;
  }

  // ---------------------------------------------------------------- classes

  boolean isSource(TypeElement te) {
    return source.contains(te);
  }

  static TypeElement typeElement(TypeMirror t) {
    if (t == null) return null;
    if (t.getKind() == TypeKind.DECLARED) return (TypeElement) ((DeclaredType) t).asElement();
    return null;
  }

  /** The superclass, or null for none / java.lang.Object. Package-private library classes are skipped. */
  TypeElement superclass(TypeElement te) {
    if (te.getKind().isInterface()) return null;
    TypeElement s = typeElement(te.getSuperclass());
    while (s != null && !isSource(s) && !s.getModifiers().contains(Modifier.PUBLIC)) s = typeElement(s.getSuperclass());
    if (s == null || s.getQualifiedName().contentEquals("java.lang.Object")) return null;
    if (ANY_TYPES.contains(s.getQualifiedName().toString())) return null;
    return s;
  }

  final Map<TypeElement, List<VariableElement>> captureCache = new HashMap<>();
  /** Local and anonymous classes created where an enclosing instance exists. */
  final Set<TypeElement> localWithOuter = new HashSet<>();
  /** The lexically enclosing class of local and anonymous classes. */
  final Map<TypeElement, TypeElement> lexicalOuter = new HashMap<>();
  /** Static-initializer functions, in emission order. */
  final List<String> clinits = new ArrayList<>();
  final Map<String, String> clinitOwner = new LinkedHashMap<>();

  /** Instrumented output: every function first makes sure the classes whose statics it touches are initialized. */
  boolean initGuards = System.getenv("J2GO_INIT_GUARDS") != null;
  private final Map<TypeElement, Boolean> clinitCache = new HashMap<>();

  /** True if the class has a static initializer function. */
  boolean hasClinit(TypeElement te) {
    if (!isSource(te)) return false;
    return clinitCache.computeIfAbsent(te, k -> {
      if (te.getKind() == ElementKind.ENUM) return true;
      for (Element e : te.getEnclosedElements()) {
        if (e.getKind() == ElementKind.ENUM_CONSTANT) return true;
        if (e.getKind() == ElementKind.FIELD && e.getModifiers().contains(Modifier.STATIC)) {
          VariableElement f = (VariableElement) e;
          Object cv = f.getConstantValue();
          if (cv == null) return true;
          if (cv instanceof Double d && (d.isNaN() || d.isInfinite())) return true;
          if (cv instanceof Float fl && (fl.isNaN() || fl.isInfinite())) return true;
        }
      }
      ClassTree ct = trees.getTree(te);
      if (ct != null) {
        for (Tree m : ct.getMembers()) if (m instanceof BlockTree b && b.isStatic()) return true;
      }
      return false;
    });
  }

  boolean isExtendable(TypeElement te) {
    if (EXTENDABLE.contains(te.getQualifiedName().toString())) return true;
    return sub(te.asType(), elems.getTypeElement("java.lang.Throwable").asType());
  }

  boolean isLeaf(TypeElement te) {
    if (isSource(te)) return !subclasses.containsKey(te);
    return !isExtendable(te);
  }

  boolean hasOuter(TypeElement te) {
    switch (te.getNestingKind()) {
      case MEMBER:
        return te.getKind() == ElementKind.CLASS && !te.getModifiers().contains(Modifier.STATIC)
            && !te.getEnclosingElement().getKind().isInterface();
      case LOCAL, ANONYMOUS:
        return localWithOuter.contains(te);
      default:
        return false;
    }
  }

  TypeElement enclosingClass(TypeElement te) {
    TypeElement o = lexicalOuter.get(te);
    if (o != null) return o;
    Element e = te.getEnclosingElement();
    while (e != null && !(e instanceof TypeElement)) e = e.getEnclosingElement();
    return (TypeElement) e;
  }

  /** True if objects of this class carry self_ (the class is in a hierarchy with subclassing). */
  boolean hasSelf(TypeElement te) {
    for (TypeElement c = te; c != null; c = superclass(c)) {
      if (!isLeaf(c)) return true;
    }
    return false;
  }

  /** True if this source class is the one that declares the self_ field. */
  boolean declaresSelf(TypeElement te) {
    if (!hasSelf(te)) return false;
    TypeElement s = superclass(te);
    if (s == null) return true;
    if (isSource(s)) return false;
    return !isExtendable(s);
  }

  boolean isSubclass(TypeElement sub, TypeElement sup) {
    for (TypeElement c = sub; c != null; c = superclass(c)) if (c.equals(sup)) return true;
    return false;
  }

  void registerClass(TypeElement te) {
    source.add(te);
  }

  /** Builds the subclass map; only now is it known which superclasses are translated classes. */
  void registerSourcesDone() {
    for (TypeElement te : source) {
      TypeElement s = superclass(te);
      if (s != null) subclasses.computeIfAbsent(s, k -> new ArrayList<>()).add(te);
    }
  }

  /** Assigns Go names to all source classes; call after every class is registered. */
  void assignClassNames() {
    Map<String, List<TypeElement>> byName = new TreeMap<>();
    for (TypeElement te : source) byName.computeIfAbsent(rawName(te), k -> new ArrayList<>()).add(te);
    for (var e : byName.entrySet()) {
      List<TypeElement> l = e.getValue();
      for (TypeElement te : l) {
        String n = e.getKey();
        if (l.size() > 1) {
          String pkg = elems.getPackageOf(te).getQualifiedName().toString();
          n = n + "_" + pkg.substring(pkg.lastIndexOf('.') + 1);
        }
        while (classByName.containsKey(n)) n += "_";
        classNames.put(te, n);
        classByName.put(n, te);
      }
    }
  }

  private String rawName(TypeElement te) {
    String bin = elems.getBinaryName(te).toString();
    String pkg = elems.getPackageOf(te).getQualifiedName().toString();
    if (!pkg.isEmpty() && bin.startsWith(pkg + ".")) bin = bin.substring(pkg.length() + 1);
    return bin.replace('$', '_').replace('.', '_');
  }

  private static final String[] JDK_PACKAGES = {
    "java.lang", "java.util", "java.util.function", "java.io", "java.util.concurrent", "java.nio.file"
  };

  String className(TypeElement te) {
    String n = classNames.get(te);
    if (n != null) return n;
    n = rawName(te);
    String pkg = elems.getPackageOf(te).getQualifiedName().toString();
    // A library class keeps its simple name unless a translated class or a core JDK class has it;
    // the rule must not depend on which class is met first.
    boolean taken = classByName.containsKey(n);
    if (!taken && !pkg.startsWith("java.") && te.getNestingKind() == NestingKind.TOP_LEVEL) {
      for (String jdk : JDK_PACKAGES) {
        if (elems.getTypeElement(jdk + "." + n) != null) taken = true;
      }
    }
    if (taken) {
      n = n + "_" + pkg.substring(pkg.lastIndexOf('.') + 1);
      while (classByName.containsKey(n)) n += "_";
    }
    classNames.put(te, n);
    classByName.put(n, te);
    noteType(te);
    return n;
  }

  // ---------------------------------------------------------------- types

  TypeMirror erase(TypeMirror t) {
    if (t == null) return null;
    switch (t.getKind()) {
      case NULL, VOID, NONE, ERROR, PACKAGE, EXECUTABLE, OTHER, MODULE:
        return t;
      default:
        // the types the Go code keeps: type variables of typed classes, their parameterized types
        if (typedVar(t) || typedDecl(t)) return t;
        if (t.getKind() == TypeKind.ARRAY) {
          // an array of a kept type: its component's Go type ([]N)
          TypeMirror c = ((ArrayType) t).getComponentType(), ec = erase(c);
          if (ec.getKind() != TypeKind.TYPEVAR && types.isSameType(ec, types.erasure(c)) && !typedDecl(ec)) return types.erasure(t);
          return ec == c ? t : types.getArrayType(ec);
        }
        if (t.getKind() == TypeKind.TYPEVAR) {
          // an erased type variable is its bound, which may be a typed declaration (T extends G<N>)
          TypeMirror b = ((TypeVariable) t).getUpperBound();
          if (b != null && b.getKind() == TypeKind.DECLARED && typedDecl(b)) return b;
        }
        return types.erasure(t);
    }
  }

  K kind(TypeMirror t0) {
    TypeMirror t = erase(t0);
    if (t == null) return K.ANY;
    switch (t.getKind()) {
      case BOOLEAN, BYTE, SHORT, INT, LONG, CHAR, FLOAT, DOUBLE:
        return K.PRIM;
      case ARRAY:
        return K.ARR;
      case NULL:
        return K.NULL;
      case TYPEVAR:
        return typedVar(t) ? K.TVAR : K.ANY;
      case VOID, NONE:
        return K.VOID;
      case DECLARED: {
        TypeElement te = typeElement(t);
        String qn = te.getQualifiedName().toString();
        if (qn.equals("java.lang.String")) return K.STR;
        if (ANY_TYPES.contains(qn)) return K.ANY;
        if (te.getKind().isInterface()) return K.IFACE;
        return K.CLS;
      }
      default:
        return K.ANY;
    }
  }

  static String primName(TypeKind k) {
    switch (k) {
      case BOOLEAN: return "bool";
      case BYTE: return "int8";
      case SHORT: return "int16";
      case INT: return "int32";
      case LONG: return "int64";
      case CHAR: return "uint16";
      case FLOAT: return "float32";
      case DOUBLE: return "float64";
      default: throw new IllegalArgumentException(k.toString());
    }
  }

  String goType(TypeMirror t0) {
    TypeMirror t = erase(t0);
    switch (kind(t)) {
      case PRIM: return primName(t.getKind());
      case STR: return "string";
      case ARR: return "[]" + goType(((ArrayType) t).getComponentType());
      case CLS:
        if (isValueLibType(t)) return className(typeElement(t)) + (typedDecl(t) ? typeArgsOf(t) : rawArgs(typeElement(t)));
        return typedDecl(t) ? "*" + className(typeElement(t)) + typeArgsOf(t) : ref(typeElement(t));
      case IFACE: return typedDecl(t) ? className(typeElement(t)) + typeArgsOf(t) : className(typeElement(t));
      case TVAR: return tvName(t);
      case VOID: return "";
      default: return "any";
    }
  }

  /** Generic aliases used by declType: alias name -> its declaration. */
  final TreeMap<String, String> typeAliases = new TreeMap<>();

  /**
   * The Go type to write in a declaration for Java type t. Same type as goType(t); where t is a
   * parameterized library type, it is spelled through a generic alias (`ListOf[*Reference]` is
   * `List`), so that the Java type arguments stay in the Go source, checked by the compiler.
   */
  String declType(TypeMirror t) {
    String g = goType(t);
    if (t == null || t.getKind() != TypeKind.DECLARED) return g;
    List<? extends TypeMirror> args = ((DeclaredType) t).getTypeArguments();
    TypeElement te = typeElement(erase(t));
    K k = kind(erase(t));
    if (args.isEmpty() || te == null || isSource(te) || isValueLib(te) || isTypedColl(t) || (k != K.CLS && k != K.IFACE)) return g;
    String alias = className(te) + "Of";
    StringBuilder ps = new StringBuilder(), as = new StringBuilder();
    for (int i = 0; i < args.size(); i++) {
      ps.append(i > 0 ? ", " : "").append("T").append(i);
      as.append(i > 0 ? ", " : "").append(typeArg(args.get(i)));
    }
    typeAliases.put(alias, "type " + alias + "[" + ps + " any] = " + g);
    return alias + "[" + as + "]";
  }

  private String typeArg(TypeMirror a) {
    if (a.getKind() == TypeKind.WILDCARD) {
      TypeMirror b = ((WildcardType) a).getExtendsBound();
      return b == null ? "any" : typeArg(b);
    }
    if (typedVar(a)) return tvName(a);
    if (a.getKind() == TypeKind.TYPEVAR) return "any";
    TypeMirror u = unboxed(a);
    if (u != null && !a.getKind().isPrimitive()) {
      typeAliases.put("BoxedOf", "type BoxedOf[T0 any] = any");
      return "BoxedOf[" + primName(u.getKind()) + "]";
    }
    return declType(a);
  }

  /** The type a boxed type unboxes to, or the type itself if primitive, else null. */
  TypeMirror unboxed(TypeMirror t) {
    if (t == null) return null;
    if (t.getKind().isPrimitive()) return t;
    try {
      return types.unboxedType(erase(t));
    } catch (IllegalArgumentException e) {
      return null;
    }
  }

  /** Expression (taking a canonical `any`) that yields a class-typed value as its most derived object. */
  String canon(String code, TypeMirror from) {
    TypeElement te = typeElement(erase(from));
    if (isHandle(te) || isValueLib(te)) return code + ".box_()";
    if (code.equals("this") && !hasSelf(te)) return "any(this)";
    if (hasSelf(te) && !isLeaf(te)) return code + ".self()";
    return "obj_(" + code + ")";
  }

  private final Map<String, Boolean> implCache = new HashMap<>();

  /**
   * Whether the Go type of class fe has every method of the Go interface of iface: each method of
   * iface is implemented in fe by a method of the same group (one Java method implementing the
   * methods of two interfaces is in one group only: the other interface's method is not there).
   */
  boolean implementsInGo(TypeElement fe, TypeElement iface) {
    return implCache.computeIfAbsent(fe.getQualifiedName() + " " + iface.getQualifiedName(), k -> {
      List<ExecutableElement> mine = new ArrayList<>();
      for (Element e : elems.getAllMembers(fe)) if (e instanceof ExecutableElement m && isInstanceMethod(m)) mine.add(m);
      for (Element e : elems.getAllMembers(iface)) {
        if (!(e instanceof ExecutableElement im) || !isInstanceMethod(im) || ownerOf(im).getQualifiedName().contentEquals("java.lang.Object")) continue;
        if (im.getModifiers().contains(Modifier.PRIVATE)) continue;
        boolean found = false;
        for (ExecutableElement m : mine) {
          if (m == im) {
            // inherited as it is: a default method (bridged) or a method of an abstract class
            found = im.isDefault();
            break;
          }
          if (m.getSimpleName().equals(im.getSimpleName()) && elems.overrides(m, im, fe)) {
            if (root(m) != root(im)) return false;
            found = true;
            break;
          }
        }
        if (!found) return false;
      }
      return true;
    });
  }

  /** The conversions that need no box (Conv). */
  final Conv conv = new Conv(this);

  /** Go interface types that some conversion casts to: each gets a function icast_I. */
  final TreeSet<String> icasts = new TreeSet<>();

  /** Go code converting {@code code}, an expression of Java type {@code from}, to Java type {@code to}. */
  String coerce(String code, TypeMirror from, TypeMirror to) {
    from = erase(from);
    to = erase(to);
    K kf = kind(from), kt = kind(to);
    if (kt == K.VOID) return code;
    if (kf == K.NULL) return nil(to);
    if (kf == K.VOID) return code;
    String gf = goType(from), gt = goType(to);
    if (gf.equals(gt)) return code;
    TypeMirror object = elems.getTypeElement("java.lang.Object").asType();
    // A typed collection meets every other type as Java's List or Set, through its erased view
    // (the one object that erased code sees); nothing but a typed collection of the same type
    // becomes one.
    if (isTypedColl(from)) {
      if (isTypedColl(to)) throw new IllegalStateException("typed collections of two types: " + gf + " to " + gt);
      // a place of Guava's immutable class: the immutable collection's erased twin (rt_timm.go)
      if (to.getKind() == TypeKind.DECLARED && typedColl(from).immType(typeElement(to))) return code + ".asImm()";
      // a place of type SortedSet / NavigableSet: the sorted collection's erased view
      if (to.getKind() == TypeKind.DECLARED && typedColl(from).sortedType(typeElement(to))) return code + ".asSorted()";
      if (typedColl(from).view == null) throw new IllegalStateException("a typed builder (" + gf + ") reaches a place of type " + gt);
      return coerce(code + "." + typedColl(from).view + "()", typedView(from), to);
    }
    if (isTypedColl(to)) {
      // An immutable collection that erased code made (an expression of the static type
      // ImmutableList: Colls.inflow) enters as the typed collection of that object.
      CollKind ck = typedColl(to);
      if (ck.immClass == null) throw new IllegalStateException("a value of type " + from + " cannot become the typed collection " + gt);
      TypeMirror ic = types.erasure(elems.getTypeElement(ck.immClass).asType());
      return "t" + ck.name + "Imm_(" + tconvsOf(to) + ", " + coerce(code, from, ic) + ")";
    }
    // A value library class: between two instantiations the content is converted; from an
    // object it is unboxed (nil is null, the zero value).
    if (isValueLibType(to)) {
      String ta = gt.substring(gt.indexOf('['));
      String fn = "opt" + (className(typeElement(to)).equals("Optional") ? "" : className(typeElement(to)));
      if (isValueLibType(from) && typeElement(from).equals(typeElement(to))) return fn + "As_" + ta + "(" + code + ")";
      return fn + "Of_" + ta + "(" + coerce(code, from, object) + ")";
    }
    // A value of a type parameter meets another type through its box, as in the erased code.
    if (kf == K.TVAR) return kt == K.ANY ? "boxT_(" + code + ")" : coerce("boxT_(" + code + ")", object, to);
    if (kt == K.TVAR) return "unboxT_[" + gt + "](" + coerce(code, from, object) + ")";
    switch (kt) {
      case ANY:
        switch (kf) {
          case PRIM: return "any(" + gf + "(" + code + "))";
          case STR: return "strobj_(" + code + ")";
          case CLS: return canon(code, from);
          case ARR: return "any(" + code + ")";
          default: return "any(" + code + ")";
        }
      case IFACE: {
        // Performance rule: a cast to an interface goes through a generated non-generic function
        // (icast_I, in g_0casts.go). Go caches a type assertion to an interface per assertion
        // site; inside the generic cast_ it cannot, and looks the method table up each time.
        // A generic interface of the program (typed) has no icast_: its instantiations are many.
        if (kf == K.CLS) {
          String up = conv.upcast(code, from, to, gf, gt);
          if (up != null) return up;
        }
        String cast = typedDecl(to) ? "cast_[" + gt + "]" : "icast_" + gt;
        if (!typedDecl(to)) icasts.add(gt);
        switch (kf) {
          case CLS: {
            // A class without subclasses that implements the typed interface: Go converts it
            // (performance rule: cast_ of a generic interface is looked up at run time on every
            // call). A null pointer is the nil interface.
            TypeElement fe = typeElement(from);
            DeclaredType view = typedDecl(to) && fe != null && isSource(fe) && isLeaf(fe) && !isHandle(fe) ? superView(from, typeElement(to)) : null;
            if (view != null && goType(view).equals(gt) && implementsInGo(fe, typeElement(to))) return "func() " + gt + " { if v := " + code + "; v != nil { return v }; return nil }()";
            return cast + "(" + canon(code, from) + ")";
          }
          case IFACE:
            if (types.isSubtype(from, to)) return gt + "(" + code + ")";
            return cast + "(" + code + ")";
          case STR: return cast + "(strobj_(" + code + "))";
          case PRIM: return cast + "(any(" + gf + "(" + code + ")))";
          default: return cast + "(" + code + ")";
        }
      }
      case CLS: {
        TypeElement tt = typeElement(to);
        String src;
        switch (kf) {
          case CLS: {
            TypeElement tf = typeElement(from);
            if (isSubclass(tf, tt)) return code + ".up_" + className(tt) + "()";
            if (conv.sameGoType(gf, gt)) return code;
            src = canon(code, from);
            break;
          }
          case STR: src = "strobj_(" + code + ")"; break;
          case PRIM: src = "any(" + code + ")"; break;
          default: src = code;
        }
        if (isHandle(tt)) return gt + "_of_(" + src + ")";
        if (isLeaf(tt)) return "cast_[" + gt + "](" + src + ")";
        return "to_" + className(tt) + (typedDecl(to) ? typeArgsOf(to) : goTparamsUse(tt)) + "(" + src + ")";
      }
      case STR:
        switch (kf) {
          case CLS: return "str_(" + canon(code, from) + ")";
          case PRIM: return "str_(any(" + code + "))";
          default: return "str_(" + code + ")";
        }
      case PRIM:
        switch (kf) {
          case PRIM: {
            if (to.getKind() == TypeKind.BOOLEAN || from.getKind() == TypeKind.BOOLEAN) return code;
            boolean ff = from.getKind() == TypeKind.FLOAT || from.getKind() == TypeKind.DOUBLE;
            boolean tf = to.getKind() == TypeKind.FLOAT || to.getKind() == TypeKind.DOUBLE;
            if (ff && !tf) return "f2i_[" + gt + "](float64(" + code + "))";
            return gt + "(" + code + ")";
          }
          case ANY: case IFACE: {
            // Unboxing; a boxed value of another primitive type first unboxes to its own type.
            return "unbox_[" + gt + "](" + code + ")";
          }
          default: return "unbox_[" + gt + "](any(" + code + "))";
        }
      case ARR:
        switch (kf) {
          case ARR: {
            // Another element type: a converted copy.
            TypeMirror cf = ((ArrayType) from).getComponentType(), ct = ((ArrayType) to).getComponentType();
            return "arrmap_(" + code + ", func(v_ " + goType(cf) + ") " + goType(ct) + " { return " + coerce("v_", cf, ct) + " })";
          }
          default: return "cast_[" + gt + "](" + code + ")";
        }
      default:
        return code;
    }
  }

  // ---------------------------------------------------------------- method groups

  private final Map<ExecutableElement, ExecutableElement> rootCache = new HashMap<>();

  /** The root-most method that {@code m} overrides or implements (itself if none). */
  ExecutableElement root(ExecutableElement m) {
    ExecutableElement r = rootCache.get(m);
    if (r != null) return r;
    r = m;
    if (m.getKind() == ElementKind.METHOD && !m.getModifiers().contains(Modifier.STATIC)
        && !m.getModifiers().contains(Modifier.PRIVATE)) {
      TypeElement owner = (TypeElement) m.getEnclosingElement();
      ExecutableElement found = findOverridden(m, owner, owner, new HashSet<>());
      if (found != null) r = root(found);
    }
    rootCache.put(m, r);
    return r;
  }

  private ExecutableElement findOverridden(ExecutableElement m, TypeElement owner, TypeElement in, Set<TypeElement> seen) {
    // Superclass chain first, then interfaces, so that the choice is deterministic.
    List<TypeMirror> supers = new ArrayList<>();
    if (in.getSuperclass().getKind() == TypeKind.DECLARED) supers.add(in.getSuperclass());
    supers.addAll(in.getInterfaces());
    if (in.getKind().isInterface() && in == owner) {
      // Interfaces implicitly declare Object's public methods.
      supers.add(elems.getTypeElement("java.lang.Object").asType());
    }
    for (TypeMirror st : supers) {
      TypeElement s = typeElement(st);
      if (s == null || !seen.add(s)) continue;
      // A package-private library class (AbstractStringBuilder) is an implementation detail: look through it.
      boolean hidden = !isSource(s) && !s.getModifiers().contains(Modifier.PUBLIC);
      if (!hidden) for (Element e : s.getEnclosedElements()) {
        if (e.getKind() != ElementKind.METHOD || !e.getSimpleName().equals(m.getSimpleName())) continue;
        ExecutableElement m2 = (ExecutableElement) e;
        if (m2.getModifiers().contains(Modifier.STATIC) || m2.getModifiers().contains(Modifier.PRIVATE)) continue;
        if (elems.overrides(m, m2, owner)) return m2;
      }
      ExecutableElement deeper = findOverridden(m, owner, s, seen);
      if (deeper != null) return deeper;
    }
    return null;
  }

  TypeElement ownerOf(Element e) {
    return (TypeElement) e.getEnclosingElement();
  }

  /** Erased parameter types of the group {@code m} belongs to. */
  List<TypeMirror> groupParams(ExecutableElement m) {
    ExecutableElement r = root(m);
    List<TypeMirror> out = new ArrayList<>();
    for (VariableElement p : r.getParameters()) out.add(erase(vtype(p)));
    return out;
  }

  TypeMirror groupReturn(ExecutableElement m) {
    return erase(rtype(root(m)));
  }

  /**
   * The group's signature as a value of type site sees it: for a group whose root is declared by a
   * typed generic class or interface, the root's signature at site's view of that declaration
   * (PMap.get(K) through a PMap<String, V> is get(string)); otherwise null (the erased signature).
   */
  boolean groupAt(ExecutableElement m, TypeMirror site) {
    return site != null && typedClasses.contains(ownerOf(root(m)));
  }

  /** groupParams(m) as a value of type site sees them (groupAt). */
  List<TypeMirror> groupParams(ExecutableElement m, TypeMirror site) {
    if (!groupAt(m, site)) return groupParams(m);
    ExecutableElement r = root(m);
    List<TypeMirror> out = new ArrayList<>();
    for (VariableElement p : r.getParameters()) out.add(erase(atSite(vtype(p), ownerOf(r), site)));
    return out;
  }

  /** groupReturn(m) as a value of type site sees it (groupAt). */
  TypeMirror groupReturn(ExecutableElement m, TypeMirror site) {
    if (!groupAt(m, site)) return groupReturn(m);
    ExecutableElement r = root(m);
    return erase(atSite(rtype(r), ownerOf(r), site));
  }

  /** The supertype of site (or site itself) whose class is target, with site's type arguments substituted; null if none. */
  DeclaredType superView(TypeMirror site, TypeElement target) {
    if (site != null && site.getKind() == TypeKind.TYPEVAR) site = ((TypeVariable) site).getUpperBound();
    if (site == null || site.getKind() != TypeKind.DECLARED) return null;
    if (typeElement(site).equals(target)) return (DeclaredType) site;
    for (TypeMirror st : types.directSupertypes(site)) {
      DeclaredType d = superView(st, target);
      if (d != null) return d;
    }
    return null;
  }

  /** "[X, Y]" for site's view of typed declaration target (superView), or "" when target is not typed. */
  String viewArgs(TypeMirror site, TypeElement target) {
    if (!typedClasses.contains(target)) return "";
    DeclaredType d = superView(site, target);
    if (d == null) throw new IllegalStateException("no view of " + target + " from " + site);
    return typeArgsOf(d);
  }

  /** Subtyping of the erasures (class hierarchy), whatever type arguments the Go code keeps. */
  boolean sub(TypeMirror a, TypeMirror b) {
    return types.isSubtype(types.erasure(a), types.erasure(b));
  }

  // ---- naming of methods

  /** Go names of method groups (keyed by root). */
  final Map<ExecutableElement, String> methodNames = new HashMap<>();
  /** Roots whose group has two or more concrete declarations along one class chain: dispatched through self_. */
  final Map<TypeElement, Set<ExecutableElement>> virtualAt = new HashMap<>();
  /** For each source class, the roots it declares (instance methods), including synthesized default bridges. */
  final Map<TypeElement, Map<ExecutableElement, ExecutableElement>> declared = new HashMap<>();
  final Set<String> instanceMethodNames = new HashSet<>();
  final Map<Element, String> staticNames = new HashMap<>();
  final Map<VariableElement, String> fieldNames = new HashMap<>();
  /** Default methods that a class must bridge: class -> list of interface default methods. */
  final Map<TypeElement, List<ExecutableElement>> defaultBridges = new HashMap<>();

  static String typeToken(TypeMirror t) {
    switch (t.getKind()) {
      case ARRAY: return typeToken(((ArrayType) t).getComponentType()) + "Arr";
      case DECLARED: return ((DeclaredType) t).asElement().getSimpleName().toString();
      default: return t.toString();
    }
  }

  String sigToken(ExecutableElement m) {
    StringBuilder sb = new StringBuilder();
    for (VariableElement p : m.getParameters()) sb.append('_').append(typeToken(types.erasure(p.asType())));
    return sb.toString();
  }

  static boolean isInstanceMethod(Element e) {
    return e.getKind() == ElementKind.METHOD && !e.getModifiers().contains(Modifier.STATIC);
  }

  List<TypeElement> allSupertypes(TypeElement te) {
    LinkedHashSet<TypeElement> out = new LinkedHashSet<>();
    Deque<TypeElement> work = new ArrayDeque<>();
    work.add(te);
    while (!work.isEmpty()) {
      TypeElement c = work.poll();
      for (TypeMirror st : types.directSupertypes(c.asType())) {
        TypeElement s = typeElement(st);
        if (s != null && out.add(s)) work.add(s);
      }
    }
    return new ArrayList<>(out);
  }

  void analyzeMethods() {
    // 1. Declared roots per class; names in use per class (declared + inherited from source supertypes).
    for (TypeElement te : source) {
      Map<ExecutableElement, ExecutableElement> d = new LinkedHashMap<>();
      for (Element e : te.getEnclosedElements()) {
        if (isInstanceMethod(e)) {
          d.put(root((ExecutableElement) e), (ExecutableElement) e);
          noteExtRoot((ExecutableElement) e);
        }
      }
      declared.put(te, d);
    }
    // 2. Default-method bridges: the top-most class under each interface default method gets a bridge
    //    unless it or a superclass declares the method.
    for (TypeElement te : source) {
      if (te.getKind().isInterface()) continue;
      TypeElement sup = superclass(te);
      for (TypeElement st : allSupertypes(te)) {
        if (!st.getKind().isInterface() || !isSource(st)) continue;
        if (sup != null && sub(sup.asType(), st.asType())) continue;
        for (Element e : st.getEnclosedElements()) {
          if (!isInstanceMethod(e) || !((ExecutableElement) e).isDefault()) continue;
          ExecutableElement dm = (ExecutableElement) e;
          ExecutableElement r = root(dm);
          boolean has = false;
          for (TypeElement c = te; c != null && !has; c = superclass(c)) {
            if (isSource(c) && declared.get(c).containsKey(r)) has = true;
          }
          // A more specific default in a sub-interface wins: skip if another supertype redeclares it.
          if (!has) {
            for (TypeElement other : allSupertypes(te)) {
              if (other.equals(st) || !isSource(other) || !other.getKind().isInterface()) continue;
              if (!sub(other.asType(), st.asType())) continue;
              ExecutableElement od = declared.get(other).get(r);
              if (od != null && od.isDefault()) has = true;
            }
          }
          if (!has) {
            defaultBridges.computeIfAbsent(te, k -> new ArrayList<>()).add(dm);
            declared.get(te).put(r, dm);
          }
        }
      }
    }
    for (TypeElement te : source) {
      if (te.getKind().isInterface()) continue;
      TypeElement sup = superclass(te);
      for (ExecutableElement dm : extDefaultOverrides(te)) {
        ExecutableElement r = root(dm);
        if (sup != null && sub(sup.asType(), ownerOf(dm).asType())) continue;
        boolean has = false;
        for (TypeElement c = te; c != null && !has; c = superclass(c)) {
          if (isSource(c) && declared.get(c).containsKey(r)) has = true;
        }
        if (!has) {
          extBridges.computeIfAbsent(te, k -> new ArrayList<>()).add(dm);
          declared.get(te).put(r, dm);
          noteExtRoot(dm);
        }
      }
    }
    // 3. Name conflicts: within the members visible in one class, two groups with one Java name.
    Map<ExecutableElement, Set<ExecutableElement>> conflicts = new HashMap<>();
    for (TypeElement te : source) {
      Map<String, Set<ExecutableElement>> byName = new HashMap<>();
      List<TypeElement> chain = new ArrayList<>();
      chain.add(te);
      chain.addAll(allSupertypes(te));
      for (TypeElement c : chain) {
        if (isSource(c)) {
          for (ExecutableElement r : declared.get(c).keySet()) {
            byName.computeIfAbsent(r.getSimpleName().toString(), k -> new LinkedHashSet<>()).add(r);
          }
        }
      }
      for (Set<ExecutableElement> set : byName.values()) {
        if (set.size() < 2) continue;
        for (ExecutableElement r : set) conflicts.computeIfAbsent(r, k -> new LinkedHashSet<>()).addAll(set);
      }
    }
    for (TypeElement te : source) {
      for (ExecutableElement r : declared.get(te).keySet()) {
        if (methodNames.containsKey(r)) continue;
        methodNames.put(r, nameRoot(r, conflicts.get(r)));
      }
    }
    instanceMethodNames.addAll(methodNames.values());
    // 4. Virtual dispatch: for each class chain, a group declared concretely at two or more levels.
    for (TypeElement te : source) {
      if (te.getKind().isInterface()) continue;
      for (ExecutableElement r : declared.get(te).keySet()) {
        TypeElement top = te;
        for (TypeElement c = superclass(te); c != null && isSource(c); c = superclass(c)) {
          if (memberOf(r, c)) top = c;
        }
        if (top != te) virtualAt.computeIfAbsent(top, k -> new LinkedHashSet<>()).add(r);
      }
    }
    // Abstract declarations at the top are virtual too: there must be something to dispatch to.
    for (TypeElement te : source) {
      if (te.getKind().isInterface()) continue;
      for (var en : declared.get(te).entrySet()) {
        if (en.getValue().getModifiers().contains(Modifier.ABSTRACT) && !en.getValue().isDefault()) {
          TypeElement top = te;
          for (TypeElement c = superclass(te); c != null && isSource(c); c = superclass(c)) {
            if (memberOf(en.getKey(), c)) top = c;
          }
          virtualAt.computeIfAbsent(top, k -> new LinkedHashSet<>()).add(en.getKey());
        }
      }
    }
    // An abstract class that takes a method from an interface without declaring it, while subclasses
    // implement it: the stub lives on that class.
    for (TypeElement te : source) {
      if (te.getKind().isInterface()) continue;
      for (ExecutableElement r : declared.get(te).keySet()) {
        TypeElement top = null;
        for (TypeElement c = superclass(te); c != null && isSource(c); c = superclass(c)) {
          if (memberOf(r, c)) top = c;
        }
        if (top != null) virtualAt.computeIfAbsent(top, k -> new LinkedHashSet<>()).add(r);
      }
    }
  }

  /**
   * Default methods of library interfaces above {@code te} that implement an abstract method of a
   * super-interface (Guava's Predicate.test implementing java.util.function.Predicate.test): the Go
   * interface asks for the method, so implementers need a bridge to the default.
   */
  List<ExecutableElement> extDefaultOverrides(TypeElement te) {
    List<ExecutableElement> out = new ArrayList<>();
    Set<ExecutableElement> seen = new HashSet<>();
    List<TypeElement> all = new ArrayList<>();
    if (te.getKind().isInterface()) all.add(te);
    all.addAll(allSupertypes(te));
    for (TypeElement st : all) {
      if (isSource(st) || !st.getKind().isInterface() || kind(st.asType()) != K.IFACE) continue;
      for (Element e : st.getEnclosedElements()) {
        if (!isInstanceMethod(e) || !((ExecutableElement) e).isDefault()) continue;
        ExecutableElement dm = (ExecutableElement) e, r = root(dm);
        if (r == dm || r.isDefault() || kind(ownerOf(r).asType()) != K.IFACE) continue;
        if (seen.add(r)) out.add(dm);
      }
    }
    return out;
  }

  final Map<TypeElement, List<ExecutableElement>> extBridges = new HashMap<>();

  /** True if group {@code r} is a member of class {@code c}: c declares it or is a subtype of r's owner. */
  boolean memberOf(ExecutableElement r, TypeElement c) {
    if (isSource(c) && declared.get(c).containsKey(r)) return true;
    TypeElement owner = ownerOf(r);
    if (owner.getQualifiedName().contentEquals("java.lang.Object")) return false;
    return sub(c.asType(), owner.asType());
  }

  /** The top-most source class above-or-at {@code te} of which group r is a member. */
  TypeElement topOf(ExecutableElement r, TypeElement te) {
    TypeElement top = te;
    for (TypeElement c = superclass(te); c != null && isSource(c); c = superclass(c)) {
      if (memberOf(r, c)) top = c;
    }
    return top;
  }

  boolean isVirtual(ExecutableElement r, TypeElement te) {
    Set<ExecutableElement> s = virtualAt.get(topOf(r, te));
    return s != null && s.contains(r);
  }

  private String nameRoot(ExecutableElement r, Set<ExecutableElement> conflict) {
    String base = safeMember(r.getSimpleName().toString());
    TypeElement owner = ownerOf(r);
    if (!isSource(owner)) return externalMethodName(r);
    if (conflict == null) return base;
    boolean arityUnique = true;
    for (ExecutableElement o : conflict) {
      if (o != r && o.getParameters().size() == r.getParameters().size()) arityUnique = false;
    }
    // The external group in a conflict keeps the plain name.
    boolean anyExternal = false;
    for (ExecutableElement o : conflict) if (!isSource(ownerOf(o))) anyExternal = true;
    if (!anyExternal) {
      // The first-declared, lowest-arity group keeps the plain name when arities differ.
      ExecutableElement best = null;
      for (ExecutableElement o : conflict) {
        if (best == null || o.getParameters().size() < best.getParameters().size()) best = o;
      }
      if (best == r && arityUnique) return base;
    }
    if (arityUnique) return base + "_" + r.getParameters().size();
    return base + sigToken(r);
  }

  static String safe(String n) {
    if (n.indexOf('$') >= 0) n = n.replace("$", "_d_");
    return GO_RESERVED.contains(n) ? n + "_" : n;
  }

  static final Set<String> GO_KEYWORDS = Set.of(
      "break", "default", "func", "interface", "select", "case", "defer", "go", "map", "struct", "chan",
      "else", "goto", "package", "switch", "const", "fallthrough", "if", "range", "type", "continue",
      "for", "import", "return", "var");

  /** Names of methods and fields: only Go keywords are a problem there. */
  static String safeMember(String n) {
    if (n.indexOf('$') >= 0) n = n.replace("$", "_d_");
    return GO_KEYWORDS.contains(n) ? n + "_" : n;
  }

  /** How often each library method or constructor is used (instance methods: by group root). */
  final Map<ExecutableElement, Integer> useCount = new HashMap<>();
  /** Every type that appears as the type of an expression, plus the source classes. */
  final Set<TypeElement> seenTypes = new LinkedHashSet<>();
  private final Map<TypeElement, Set<TypeElement>> subsCache = new HashMap<>();
  private Map<ExecutableElement, String> extNames;

  void countUse(ExecutableElement e) {
    if (e.getKind() == ElementKind.METHOD && !e.getModifiers().contains(Modifier.STATIC)) {
      useCount.merge(root(e), 1, Integer::sum);
    }
    useCount.merge(e, 1, Integer::sum);
  }

  private Set<TypeElement> subs(TypeElement o) {
    return subsCache.computeIfAbsent(o, k -> {
      Set<TypeElement> out = new HashSet<>();
      TypeMirror ot = erase(o.asType());
      for (TypeElement t : seenTypes) if (sub(t.asType(), ot)) out.add(t);
      out.add(o);
      return out;
    });
  }

  private String stableKey(ExecutableElement e) {
    return ownerOf(e).getQualifiedName() + "." + e.getSimpleName() + sigToken(e);
  }

  /** Orders overloads: most used first. */
  private List<ExecutableElement> byUse(Collection<ExecutableElement> c) {
    List<ExecutableElement> l = new ArrayList<>(c);
    l.sort(Comparator.<ExecutableElement>comparingInt(e -> -useCount.getOrDefault(e, 0)).thenComparing(this::stableKey));
    return l;
  }

  static String overloadSuffix(ExecutableElement e, String sig) {
    return e.getParameters().isEmpty() ? "_0" : sig;
  }

  private void assignExternalNames() {
    extNames = new HashMap<>();
    seenTypes.addAll(source);
    for (Set<ExecutableElement> group : extRootsByName.values()) {
      List<ExecutableElement> plain = new ArrayList<>();
      for (ExecutableElement r : byUse(group)) {
        String name = safeMember(r.getSimpleName().toString());
        boolean clash = false;
        for (ExecutableElement p : plain) {
          if (!Collections.disjoint(subs(ownerOf(r)), subs(ownerOf(p)))) clash = true;
        }
        if (clash) {
          extNames.put(r, name + overloadSuffix(r, sigToken(r)));
        } else {
          extNames.put(r, name);
          plain.add(r);
        }
      }
    }
  }

  // ---- external member names

  /** How often each external overload is used; filled by a pre-pass, used to pick plain names. */
  final Map<String, Map<String, Integer>> externalUse = new HashMap<>();

  /** Library instance-method groups the program uses, by simple name (filled before naming). */
  final Map<String, Set<ExecutableElement>> extRootsByName = new HashMap<>();

  void noteExtRoot(ExecutableElement m) {
    if (m.getKind() != ElementKind.METHOD || m.getModifiers().contains(Modifier.STATIC)) return;
    ExecutableElement r = root(m);
    if (isSource(ownerOf(r))) return;
    extRootsByName.computeIfAbsent(r.getSimpleName().toString(), k -> new LinkedHashSet<>()).add(r);
  }

  String externalMethodName(ExecutableElement m) {
    ExecutableElement r = root(m);
    if (extNames == null) assignExternalNames();
    String n = extNames.get(r);
    return n != null ? n : safeMember(r.getSimpleName().toString());
  }

  boolean rootIsObject(ExecutableElement m) {
    return ownerOf(root(m)).getQualifiedName().contentEquals("java.lang.Object");
  }

  String methodName(ExecutableElement m) {
    ExecutableElement r = root(m);
    String n = methodNames.get(r);
    if (n == null) {
      n = isSource(ownerOf(r)) ? safeMember(r.getSimpleName().toString()) : externalMethodName(r);
      methodNames.put(r, n);
    }
    return n;
  }

  /** Name of the implementation of group-of-m declared in class te: m_ when dispatched through self_. */
  String implName(ExecutableElement m, TypeElement te) {
    String n = methodName(m);
    return isVirtual(root(m), te) ? n + "_impl" : n;
  }

  String staticMethodName(ExecutableElement m) {
    String n = staticNames.get(m);
    if (n != null) return n;
    TypeElement owner = ownerOf(m);
    String base = className(owner) + "_" + m.getSimpleName().toString().replace("$", "_d_");
    int same = 0;
    for (Element e : owner.getEnclosedElements()) {
      if (e.getKind() == ElementKind.METHOD && e.getSimpleName().equals(m.getSimpleName())
          && (e.getModifiers().contains(Modifier.STATIC) || !isSource(owner) || ((ExecutableElement) e).isDefault()
              || owner.getKind().isInterface() && e.getModifiers().contains(Modifier.PRIVATE))) {
        same++;
      }
    }
    if (!isSource(owner)) {
      // Helpers for library types: among the overloads the program uses, the most used has the plain name.
      List<ExecutableElement> used = new ArrayList<>();
      for (Element e : owner.getEnclosedElements()) {
        if (e.getKind() == ElementKind.METHOD && e.getSimpleName().equals(m.getSimpleName())
            && (useCount.containsKey(e) || e == m)) used.add((ExecutableElement) e);
      }
      List<ExecutableElement> sorted = byUse(used);
      n = sorted.get(0) == m ? base : base + overloadSuffix(m, sigToken(m));
      staticNames.put(m, n);
      return n;
    }
    n = same > 1 && !m.getParameters().isEmpty() ? base + sigToken(m) : base;
    if (same > 1 && m.getParameters().isEmpty() && isSource(owner)) n = base + "_0";
    staticNames.put(m, n);
    return n;
  }

  String ctorName(ExecutableElement c, String prefix) {
    TypeElement owner = ownerOf(c);
    int n = 0;
    for (Element e : owner.getEnclosedElements()) if (e.getKind() == ElementKind.CONSTRUCTOR) n++;
    String base = prefix + className(owner);
    if (n <= 1) return base;
    if (!isSource(owner) && !isExtendable(owner)) {
      List<ExecutableElement> used = new ArrayList<>();
      for (Element e : owner.getEnclosedElements()) {
        if (e.getKind() == ElementKind.CONSTRUCTOR && (useCount.containsKey(e) || e == c)) used.add((ExecutableElement) e);
      }
      if (byUse(used).get(0) == c) return base;
    }
    return base + (c.getParameters().isEmpty() ? "_0" : sigToken(c));
  }

  String fieldName(VariableElement f) {
    String n = fieldNames.get(f);
    if (n != null) return n;
    TypeElement owner = ownerOf(f);
    if (f.getModifiers().contains(Modifier.STATIC)) {
      n = className(owner) + "_" + f.getSimpleName().toString().replace("$", "_d_");
    } else {
      n = safeMember(f.getSimpleName().toString());
      if (instanceMethodNames.contains(n) || n.equals("self") || n.equals("toString") || n.equals("hashCode")
          || n.equals("equals") || n.equals("name") || n.equals("ordinal")) {
        n = n + "_";
      }
    }
    fieldNames.put(f, n);
    return n;
  }

  /** The single abstract method of a functional interface type, or null. */
  ExecutableElement functionalMethod(TypeElement te) {
    ExecutableElement found = null;
    Set<ExecutableElement> seenRoots = new HashSet<>();
    for (Element e : elems.getAllMembers(te)) {
      if (e.getKind() != ElementKind.METHOD || !e.getModifiers().contains(Modifier.ABSTRACT)) continue;
      ExecutableElement m = (ExecutableElement) e;
      if (isObjectMethod(m)) continue;
      if (!seenRoots.add(root(m))) continue;
      if (found != null) return null;
      found = m;
    }
    return found;
  }

  boolean isObjectMethod(ExecutableElement m) {
    TypeElement obj = elems.getTypeElement("java.lang.Object");
    for (Element e : obj.getEnclosedElements()) {
      if (e.getKind() == ElementKind.METHOD && e.getSimpleName().equals(m.getSimpleName())
          && e.getModifiers().contains(Modifier.PUBLIC)
          && ((ExecutableElement) e).getParameters().size() == m.getParameters().size()) {
        if (m.getParameters().isEmpty() || elems.overrides(m, (ExecutableElement) e, ownerOf(m))
            || types.isSameType(erase(m.getParameters().get(0).asType()), obj.asType())) {
          return true;
        }
      }
    }
    return false;
  }

  String goSig(List<TypeMirror> params, TypeMirror ret) {
    return goSig(params, ret, false);
  }

  /** goSig; with decl, parameterized library types are spelled through their aliases (declType). */
  String goSig(List<TypeMirror> params, TypeMirror ret, boolean decl) {
    StringBuilder sb = new StringBuilder("(");
    for (int i = 0; i < params.size(); i++) {
      if (i > 0) sb.append(", ");
      sb.append(decl ? declType(params.get(i)) : goType(params.get(i)));
    }
    sb.append(")");
    String r = decl ? declType(ret) : goType(ret);
    if (!r.isEmpty()) sb.append(" ").append(r);
    return sb.toString();
  }

  void noteExternal(String goName, String desc) {
    externals.putIfAbsent(goName, desc);
  }

  /** Library types the translation names, with everything the stub generator needs. */
  final LinkedHashSet<TypeElement> extTypes = new LinkedHashSet<>();
  /** Go function name -> signature text "(params) ret". */
  final TreeMap<String, String> extFuncs = new TreeMap<>();
  /** Library instance methods: "name(sig) ret" -> root owner types. */
  final TreeMap<String, Set<TypeElement>> extMethods = new TreeMap<>();
  final TreeMap<String, String> extVars = new TreeMap<>();
  final Map<TypeElement, TreeMap<String, String>> extFields = new HashMap<>();
  final LinkedHashSet<TypeElement> extAdapters = new LinkedHashSet<>();
  /** Library functional interfaces that lambdas use through a typed adapter X_tfn (Stubs). */
  final LinkedHashSet<TypeElement> extTypedAdapters = new LinkedHashSet<>();

  /**
   * Concrete typed adapters, one per instantiation whose types name no type variable
   * (X_tfn_A_B: {interface, the func's parameter types, its result type, the type arguments}).
   */
  final TreeMap<String, Object[]> extConcreteAdapters = new TreeMap<>();

  /**
   * Refined locals (GenScan.refine): a local variable that is never assigned again and whose
   * declared type has wildcard type arguments has the type arguments of its initializer (or of
   * the elements it iterates over) instead, and the expressions that read it are typed from that.
   */
  final Map<Element, TypeMirror> refinedVars = new HashMap<>();
  final IdentityHashMap<com.sun.source.tree.Tree, TypeMirror> refinedTrees = new IdentityHashMap<>();
  final Set<com.sun.source.tree.Tree> refinedTypeTrees = Collections.newSetFromMap(new IdentityHashMap<>());

  /**
   * Shared statics (GenScan.shared): static final fields of type C<?, ...> (or an empty array of
   * it) that the program only casts to an instantiation of C or compares with one:
   * "single" (one object per instantiation, made by a generic function) or "array" (an empty
   * array: every cast of it is an empty array of the instantiation).
   */
  final Map<Element, String> sharedStatics = new HashMap<>();

  /** The results of methods that return a typed list (rule "typed collections"). */
  final Map<ExecutableElement, TypeMirror> typedResults = new HashMap<>();

  /** The result type of method m: the typed list it returns, or its declared type. */
  TypeMirror rtype(ExecutableElement m) {
    TypeMirror r = typedResults.get(m);
    return r != null ? r : m.getReturnType();
  }

  /** The type of variable e: its refined type, or its declared type. */
  TypeMirror vtype(Element e) {
    TypeMirror r = refinedVars.get(e);
    return r != null ? r : e.asType();
  }

  /** Whether t names a kept type variable. */
  boolean mentionsTypedVar(TypeMirror t) {
    if (t == null) return false;
    switch (t.getKind()) {
      case TYPEVAR: return typedVar(t);
      case ARRAY: return mentionsTypedVar(((ArrayType) t).getComponentType());
      case DECLARED:
        if (!typedDecl(t)) return false;
        for (TypeMirror a : ((DeclaredType) t).getTypeArguments()) if (mentionsTypedVar(a)) return true;
        return false;
      default: return false;
    }
  }

  /**
   * A type that a value in an erased slot reaches with one type assertion (unboxT_ takes the fast
   * path): a String, a handle, a class without subclasses, an interface, an array, a kept type
   * variable. A class with subclasses needs its view (to_C), and Object or a boxed value is `any`.
   */
  boolean assertable(TypeMirror t) {
    if (t == null) return false;
    switch (kind(t)) {
      case STR, IFACE, ARR, TVAR: return true;
      case CLS: return isHandleType(t) || isLeaf(typeElement(erase(t)));
      default: return false;
    }
  }

  void noteType(TypeElement te) {
    if (isSource(te) || !extTypes.add(te)) return;
    for (TypeMirror st : types.directSupertypes(te.asType())) {
      TypeElement s = typeElement(st);
      if (s != null && !s.getQualifiedName().contentEquals("java.lang.Object")) {
        className(s);
      }
    }
  }

  void noteFunc(String name, List<TypeMirror> params, TypeMirror ret) {
    extFuncs.putIfAbsent(name, goSig(params, ret));
  }

  /** Code calling the receiver-first helper of library default method dm from a method of group root(dm). */
  String defaultBridgeBody(ExecutableElement dm, String self) {
    ExecutableElement r = root(dm);
    List<TypeMirror> gp = groupParams(r), own = new ArrayList<>();
    for (VariableElement p : dm.getParameters()) own.add(erase(p.asType()));
    StringBuilder args = new StringBuilder(self);
    for (int i = 0; i < gp.size(); i++) args.append(", ").append(coerce("p" + i, gp.get(i), own.get(i)));
    List<TypeMirror> full = new ArrayList<>();
    full.add(erase(ownerOf(dm).asType()));
    full.addAll(own);
    String fn = staticMethodName(dm);
    noteFunc(fn, full, erase(dm.getReturnType()));
    String call = fn + "(" + args + ")";
    if (groupReturn(r).getKind() == TypeKind.VOID) return call;
    return "return " + coerce(call, erase(dm.getReturnType()), groupReturn(r));
  }

  void noteMethodOn(TypeElement owner, String name, List<TypeMirror> params, TypeMirror ret) {
    className(owner);
    extMethods.computeIfAbsent(name + goSig(params, ret), k -> new LinkedHashSet<>()).add(owner);
  }
}
