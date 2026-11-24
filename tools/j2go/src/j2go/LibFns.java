package j2go;

import com.sun.source.tree.ExpressionTree;
import com.sun.source.tree.MethodInvocationTree;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Rule "typed library interfaces", piece `fns` (J2GO_NOLIBGEN=fns is the translation without it;
 * it needs rule "function types", J2GO_NOFNTYPES=1 turns it off too): the runtime's static
 * library functions and the default methods of the library interfaces have typed twins at
 * their Java generic signatures (go/closure/rt_lgfn.go: Go generic functions whose collections
 * are the typed interfaces of the rule and whose functions are the func types of rule "function
 * types"). A call of one of the methods of TYPED is the call of its twin, instantiated with
 * javac's type arguments (a static method: its own type parameters as javac inferred them; a
 * default method: the type arguments of the receiver's typed interface), where
 *
 * - the twin's Go signature (the templates of Fn, $i the i-th type argument) is the one that
 *   javac's types give for the call (the parameters' and the result's Go types are compared as
 *   text: a type argument that a typed interface cannot hold, a wildcard, an erased type
 *   variable fail here), and the runtime declares it (Conv.rtGenerics);
 * - every argument (and the receiver) of a typed interface type enters as it is: it is a typed
 *   collection or a typed interface of the same element types (a collection of an erased place
 *   or of other element types, variance, would need a wrapper, which costs more than the erased
 *   call's casts);
 * - the call's type is javac's (GenScan.shared gives some calls another type: they stay erased).
 *
 * Any other call keeps the erased function, as before (J2GO_LIBGEN=2 lists them by reason).
 * REFUSED lists the library functions of the same kind that have no twin, and why.
 */
final class LibFns {
  /** A typed twin: its Go name, the receiver's Go type (a default method; "" for a static), the parameters' and the result's ("" for void). */
  record Fn(String go, String recv, List<String> params, String ret) {}

  /** The methods with a typed twin, by key (owner.name(erased parameter types)). */
  static final Map<String, Fn> TYPED = new LinkedHashMap<>();
  /**
   * Piece `seqcoll` (J2GO_NOLIBGEN=seqcoll is the translation without it): the default methods of
   * java.util.SequencedCollection, an interface with no typed interface of its own, with typed
   * twins on the typed interface of the interface it extends (RECV_AT): a receiver enters at its
   * typed Collection (a typed deque or list of rule "typed collections" as its view, a DequeT or
   * ListT as it is). Without the piece they are among REFUSED.
   */
  static final Map<String, Fn> TYPED_SEQ = new LinkedHashMap<>();
  /** The typed interface whose view of the receiver a twin of TYPED_SEQ takes, by the owner of the method. */
  static final Map<String, String> RECV_AT = Map.of("java.util.SequencedCollection", "java.util.Collection");

  /** The methods of the same kind that keep the erased function, and why (reported, J2GO_LIBGEN). */
  static final Map<String, String> REFUSED = new LinkedHashMap<>();

  /**
   * Piece `fns2` (J2GO_NOLIBGEN=fns2 is the translation without it; needs rule "function types"):
   * more twins of the kind of TYPED (RESIDUE8.md, rule "more LibFns twins"). A Go name with type
   * arguments ("F[$0]") gives them itself.
   */
  static final Map<String, Fn> TYPED_F2 = new LinkedHashMap<>();
  /** Piece `rtm`: twins of the kind of TYPED that need no function type (List.add(int, E)). */
  static final Map<String, Fn> TYPED_RT = new LinkedHashMap<>();
  static final String COMPARING = "java.util.Comparator.comparing(java.util.function.Function)";
  /** Twins whose call is typed only as the argument of a method of READS_ONCE, at Iterable (the result is a view that is no List). */
  static final Set<String> ITERABLE_ONCE = new HashSet<>();
  /** Library methods that read an Iterable argument once and keep nothing of it (key: LibFns.key, of the method or its root). */
  static final Set<String> READS_ONCE = new HashSet<>();

  /**
   * Piece `rtm` (J2GO_NOLIBGEN=rtm is the translation without it; RESIDUE8.md, rule "typed
   * runtime instance methods"): a runtime method with a typed twin. recv: the receiver's class
   * (its erasure must be that class; "java.lang.Enum": a program enum) or the kind of typed
   * collection it must be ("list", "set", "map"); go: the twin's Go method; ret: the result's
   * Java type ("=" the call's, "@" the receiver's typed collection type); params: the Java types
   * the arguments are given at ("java.lang.String": the argument must be a String; "$i": the
   * receiver's i-th type argument, whose Go form the argument must have).
   */
  record RtM(String recv, String go, String ret, List<String> params) {}
  static final Map<String, RtM> RT_METHODS = new LinkedHashMap<>();

  static void rt(String key, String recv, String go, String ret, String... params) {
    RT_METHODS.put(key, new RtM(recv, go, ret, List.of(params)));
  }

  static void typed(String key, String go, String recv, String ret, String... params) {
    TYPED.put(key, new Fn(go, recv, List.of(params), ret));
  }

  static {
    String it = "com.google.common.collect.Iterables.";
    typed(it + "getFirst(java.lang.Iterable,java.lang.Object)", "Iterables_getFirstT_", "", "$0", "IterableT[$0]", "$0");
    typed(it + "getLast(java.lang.Iterable)", "Iterables_getLastT_", "", "$0", "IterableT[$0]");
    typed(it + "getOnlyElement(java.lang.Iterable)", "Iterables_getOnlyElementT_", "", "$0", "IterableT[$0]");
    typed(it + "get(java.lang.Iterable,int)", "Iterables_getT_", "", "$0", "IterableT[$0]", "int32");
    typed(it + "indexOf(java.lang.Iterable,com.google.common.base.Predicate)", "Iterables_indexOfT_", "", "int32", "IterableT[$0]", "Predicate_base[$0]");
    typed(it + "filter(java.lang.Iterable,com.google.common.base.Predicate)", "Iterables_filterT_", "", "IterableT[$0]", "IterableT[$0]", "Predicate_base[$0]");
    typed(it + "transform(java.lang.Iterable,com.google.common.base.Function)", "Iterables_transformT_", "", "IterableT[$1]", "IterableT[$0]", "Function_base[$0, $1]");
    typed("com.google.common.base.MoreObjects.firstNonNull(java.lang.Object,java.lang.Object)", "MoreObjects_firstNonNullT_", "", "$0", "$0", "$0");
    typed("java.util.Collections.sort(java.util.List,java.util.Comparator)", "List_sortT_", "", "", "ListT[$0]", "Comparator[$0]");
    typed("java.util.Collections.sort(java.util.List)", "Collections_sortT_", "", "", "ListT[$0]");
    typed("java.lang.Iterable.forEach(java.util.function.Consumer)", "Iterable_forEachT_", "IterableT[$0]", "", "Consumer[$0]");
    typed("java.util.List.sort(java.util.Comparator)", "List_sortT_", "ListT[$0]", "", "Comparator[$0]");
    typed("java.util.Map.forEach(java.util.function.BiConsumer)", "Map_forEachT_", "MapT[$0, $1]", "", "BiConsumer[$0, $1]");
    typed("java.util.Map.replaceAll(java.util.function.BiFunction)", "Map_replaceAllT_", "MapT[$0, $1]", "", "BiFunction[$0, $1, $1]");
    typed("java.util.Map.computeIfAbsent(java.lang.Object,java.util.function.Function)", "Map_computeIfAbsentT_", "MapT[$0, $1]", "$1", "$0", "Function[$0, $1]");
    typed("java.util.Map.getOrDefault(java.lang.Object,java.lang.Object)", "Map_getOrDefaultT_", "MapT[$0, $1]", "$1", "any", "$1");
    typed("java.util.Map.merge(java.lang.Object,java.lang.Object,java.util.function.BiFunction)", "Map_mergeT_", "MapT[$0, $1]", "$1", "$0", "$1", "BiFunction[$1, $1, $1]");
    // a String key on a map of String keys (what Map_get_s / Map_getOrDefault_s are for erased code: ExprEmitter.stringKeyHelper)
    typed("java.util.Map.get(java.lang.Object)#s", "Map_get_sT_", "MapT[string, $1]", "$1", "string");
    typed("java.util.Map.getOrDefault(java.lang.Object,java.lang.Object)#s", "Map_getOrDefault_sT_", "MapT[string, $1]", "$1", "string", "$1");

    String sq = "java.util.SequencedCollection.";
    TYPED_SEQ.put(sq + "getFirst()", new Fn("SequencedCollection_getFirstT_", "CollectionT[$0]", List.of(), "$0"));
    TYPED_SEQ.put(sq + "getLast()", new Fn("SequencedCollection_getLastT_", "CollectionT[$0]", List.of(), "$0"));
    TYPED_SEQ.put(sq + "removeLast()", new Fn("SequencedCollection_removeLastT_", "CollectionT[$0]", List.of(), "$0"));
    TYPED_SEQ.put(sq + "addLast(java.lang.Object)", new Fn("SequencedCollection_addLastT_", "CollectionT[$0]", List.of("$0"), ""));

    String ls = "com.google.common.collect.Lists.";
    String view = "its result is a live view of the list (a List class of the runtime at the element type would be needed: not a minimal runtime)";
    REFUSED.put(ls + "reverse(java.util.List)", view);
    REFUSED.put(ls + "transform(java.util.List,com.google.common.base.Function)", view);
    REFUSED.put("java.util.SequencedCollection.getFirst()", "a method of SequencedCollection, which has no typed interface (its receivers here are Deques: candidate T2)");
    REFUSED.put("java.util.SequencedCollection.getLast()", "as SequencedCollection.getFirst");
    REFUSED.put("java.util.SequencedCollection.removeLast()", "as SequencedCollection.getFirst");
    REFUSED.put("java.util.SequencedCollection.addLast(java.lang.Object)", "as SequencedCollection.getFirst");
    REFUSED.put("java.util.Map.putIfAbsent(java.lang.Object,java.lang.Object)", "its receivers in the tree are erased maps (an Object, a TreeMap)");
    REFUSED.put("com.google.common.cache.CacheLoader.from(com.google.common.base.Function)", "LoadingCache has no typed interface (candidate T2)");

    // piece `fns2` (RESIDUE8 rule "more LibFns twins")
    String f2 = "java.util.";
    TYPED_F2.put(f2 + "Arrays.setAll(java.lang.Object[],java.util.function.IntFunction)", new Fn("Arrays_setAllT_[$0]", "", List.of("[]$0", "IntFunction[$0]"), ""));
    TYPED_F2.put(f2 + "Comparator.comparing(java.util.function.Function)", new Fn("Comparator_comparingS_[$0]", "", List.of("Function[$0, string]"), "Comparator[$0]"));
    TYPED_F2.put("com.google.common.collect.Lists.transform(java.util.List,com.google.common.base.Function)", new Fn("Iterables_transformT_[$0, $1]", "", List.of("ListT[$0]", "Function_base[$0, $1]"), "ListT[$1]"));
    TYPED_F2.put("com.google.javascript.jscomp.base.Io.listFiles(java.lang.String)", new Fn("Io_listFilesT_", "", List.of("string"), "ListT[string]"));
    TYPED_F2.put(f2 + "Map.putIfAbsent(java.lang.Object,java.lang.Object)#s", new Fn("Map_putIfAbsent_sT_[$1]", "MapT[string, $1]", List.of("string", "$1"), "$1"));
    TYPED_F2.put(f2 + "Map.replace(java.lang.Object,java.lang.Object,java.lang.Object)#s", new Fn("Map_replace_sT_[$1]", "MapT[string, $1]", List.of("string", "$1", "$1"), "bool"));
    TYPED_F2.put(f2 + "Collections.singletonList(java.lang.Object)", new Fn("Collections_singletonListT_[$0]", "", List.of("$0"), "ListT[$0]"));
    // (a List.transform whose result is the argument of a method that reads it once: the twin of
    // Iterables.transform, a view that is no List; the call's type is Iterable)
    ITERABLE_ONCE.add("com.google.common.collect.Lists.transform(java.util.List,com.google.common.base.Function)");
    READS_ONCE.add("com.google.common.collect.ImmutableCollection.Builder.addAll(java.lang.Iterable)");
    READS_ONCE.add("com.google.common.collect.ImmutableList.Builder.addAll(java.lang.Iterable)");

    // piece `rtm` (RESIDUE8 rule "typed runtime instance methods"): List.add(int, E) of a typed list
    TYPED_RT.put(f2 + "List.add(int,java.lang.Object)", new Fn("List_add_intT_[$0]", "ListT[$0]", List.of("int32", "$0"), ""));
    // methods of a runtime class, on a receiver of that class
    rt("java.math.BigInteger.compareTo(java.math.BigInteger)", "java.math.BigInteger", "compareTo_t", "=", "java.math.BigInteger");
    rt("com.google.common.collect.ComparisonChain.compare(java.lang.Comparable,java.lang.Comparable)", "com.google.common.collect.ComparisonChain", "compare_s", "=", "java.lang.String", "java.lang.String");
    rt("java.lang.Enum.compareTo(java.lang.Enum)", "java.lang.Enum", "compareTo_t", "=", "java.lang.Enum");
    rt("java.util.BitSet.clone()", "java.util.BitSet", "clone_t", "java.util.BitSet");
    // (piece `fns2`: Splitter.split of a String, an Iterable of Strings)
    rt("com.google.common.base.Splitter.split(java.lang.CharSequence)", "com.google.common.base.Splitter", "split_sT", "=", "java.lang.String");
    // methods of a typed collection: the receiver's kind; "@" the receiver's typed collection
    // type, "$i" its i-th type argument (as RT_METHODS otherwise)
    rt("com.google.common.collect.ImmutableList.subList(int,int)", "list", "subListImm_", "@", "int", "int");
    rt("com.google.common.collect.ImmutableList.reverse()", "list", "reverseImm_", "@");
    rt("java.util.TreeSet.tailSet(java.lang.Object,boolean)", "set", "tailSetT_", "@", "$0", "boolean");
    rt("java.util.Map.containsValue(java.lang.Object)", "map", "containsValueT_", "=", "$1");
  }

  /** The keys of TYPED_F2, TYPED_RT and RT_METHODS that some call took (the J2GO_LIBGEN report names the others). */
  final Set<String> used = new HashSet<>();

  static boolean onF2() {
    return LibGen.on("fns2") && FnTypes.ON;
  }

  static boolean onRt() {
    return LibGen.on("rtm");
  }

  /** Piece `nboxarg` (needs pieces `nbox` and `fns2`): boxed type arguments of function types at Nbox (LibNbox). */
  static boolean nboxArgOn() {
    return LibGen.on("nboxarg") && LibNbox.on() && onF2();
  }

  static final Set<TypeKind> INTEGRAL = EnumSet.of(TypeKind.BYTE, TypeKind.SHORT, TypeKind.INT, TypeKind.LONG, TypeKind.CHAR);

  /** Whether library method e reads an Iterable argument once (READS_ONCE), with piece `fns2` on. */
  boolean readsOnce(ExecutableElement e) {
    if (!onF2() || cx.isSource(cx.ownerOf(e))) return false;
    return READS_ONCE.contains(key(cx, e)) || READS_ONCE.contains(key(cx, cx.root(e)));
  }

  final Ctx cx;
  final LibGen lg;

  LibFns(Ctx cx) {
    this.cx = cx;
    this.lg = cx.lg;
  }

  static boolean on() {
    return LibGen.on("fns") && FnTypes.ON;
  }

  /** The key of method m in TYPED and REFUSED. */
  static String key(Ctx cx, ExecutableElement m) {
    StringJoiner ps = new StringJoiner(",", "(", ")");
    for (VariableElement p : m.getParameters()) ps.add(cx.types.erasure(p.asType()).toString());
    return cx.ownerOf(m).getQualifiedName() + "." + m.getSimpleName() + ps;
  }

  /** Template t with $i replaced by type argument i. */
  static String fill(String t, List<String> ta) {
    for (int i = ta.size() - 1; i >= 0; i--) t = t.replace("$" + i, ta.get(i));
    return t;
  }

  /** Reports that the call of key stays erased, and why; returns null (the erased call). */
  <T> T left(String key, String why) {
    lg.note("typed library functions: calls left erased (" + why + ")", key);
    return null;
  }

  /**
   * Whether an argument of (Java) type at enters a parameter of typed interface type p as it is:
   * null; a typed collection whose typed view p is; a value of a typed interface whose view of
   * p's interface is p.
   */
  boolean enters(TypeMirror at, TypeMirror p) {
    if (at == null || at.getKind() == TypeKind.NULL) return true;
    TypeMirror et = cx.erase(at);
    if (cx.typedColl(et) != null) return lg.fromTypedColl("x", et, p) != null;
    DeclaredType v = lg.typedView(at, Ctx.typeElement(p));
    return v != null && cx.goType(v).equals(cx.goType(p));
  }

  /**
   * The call of the typed twin of library method e (a static one, or a default method on
   * receiver recv), or null where the erased call stays (ExprEmitter.special).
   */
  ExprEmitter.CallResult call(ExprEmitter x, ExecutableElement e, ExprEmitter.Recv recv, List<ExprEmitter.Arg> as) {
    boolean toObject = x.lfErased;
    x.lfErased = false;
    MethodInvocationTree self = x.lfTree;
    x.lfTree = null;
    if ((onRt() || onF2()) && !cx.isSource(cx.ownerOf(e))) {
      // pieces `rtm` and `fns2`: a typed runtime method, a typed Comparator.thenComparing
      ExprEmitter.CallResult c = rtMethod(x, e, recv, as, toObject);
      if (c == null) c = thenComparing(x, e, recv, as);
      if (c != null) return c;
    }
    if (!on() && !onF2() && !onRt() || lg.libs.isEmpty() || cx.isSource(cx.ownerOf(e))) return null;
    boolean st = e.getModifiers().contains(Modifier.STATIC);
    if (!st && recv == null) return null;
    ExecutableElement r = st ? e : cx.root(e);
    String key = key(cx, r);
    // a String key: the twin of the String-key helper (ExprEmitter.stringKeyHelper)
    if (!st && !as.isEmpty() && (on() && TYPED.containsKey(key + "#s") || onF2() && TYPED_F2.containsKey(key + "#s")) && cx.kind(as.get(0).type) == Ctx.K.STR) key += "#s";
    Fn f = on() ? TYPED.get(key) : null;
    if (f == null && on() && LibGen.on("seqcoll")) f = TYPED_SEQ.get(key);
    boolean mine = false;
    if (f == null && onF2() && TYPED_F2.containsKey(key)) {
      f = TYPED_F2.get(key);
      mine = true;
      // (Lists.transform: only as the argument of a method that reads it once)
      if (ITERABLE_ONCE.contains(key) && (self == null || !x.readOnce.contains(self))) f = null;
    }
    if (f == null && onRt() && TYPED_RT.containsKey(key)) {
      f = TYPED_RT.get(key);
      mine = true;
    }
    if (f == null) {
      String why = on() ? REFUSED.get(key) : null;
      if (why == null && ITERABLE_ONCE.contains(key) && onF2()) why = "its result is a live view of the list, and the call is no argument of a method that reads it once";
      return why == null ? null : left(key, "no typed twin: " + why);
    }
    if (as.size() != r.getParameters().size() || r.isVarArgs()) return left(key, "arity");
    // (an element given as Object: Java casts nothing there, so an element of another class that
    // unchecked code put into the collection passes; the twin would give it at its type)
    if (toObject && f.ret().matches("\\$\\d")) return left(key, "the result enters an Object");
    String goName = f.go().contains("[") ? f.go().substring(0, f.go().indexOf('[')) : f.go();
    // (a twin of piece `fns2` without type arguments is no generic function)
    if (!(mine && !f.go().contains("[")) && !cx.conv.rtGenerics.contains(goName)) return left(key, "the runtime has no " + goName);
    List<String> ta = new ArrayList<>();
    List<TypeMirror> params = new ArrayList<>();
    TypeMirror ret;
    String rc = null;
    if (st) {
      // javac's type arguments for the method's own type parameters
      ExecutableType inst = x.curInst;
      if (x.retOverride != null) return left(key, "the call's type is given (GenScan.shared)");
      if (inst == null || inst.getParameterTypes().size() != r.getParameters().size()) return left(key, "no instantiated type");
      Map<Element, TypeMirror> m = new HashMap<>();
      ExprEmitter.unify(r.getReturnType(), inst.getReturnType(), m);
      for (int i = 0; i < r.getParameters().size(); i++) ExprEmitter.unify(r.getParameters().get(i).asType(), inst.getParameterTypes().get(i), m);
      if (mine && key.equals(COMPARING) && r.getTypeParameters().size() == 2) {
        // (a key that javac's recorded inference leaves at another type than the String that the
        // method reference gives, `new TreeSet<>(comparing(Export::getExportedPath))`: the
        // referenced method's result is the key type)
        TypeMirror u = m.get(r.getTypeParameters().get(1));
        ExpressionTree a0 = as.get(0).tree;
        if ((u == null || cx.kind(u) != Ctx.K.STR) && a0 instanceof com.sun.source.tree.MemberReferenceTree mr
            && x.elemOf(mr) instanceof ExecutableElement me && me.getParameters().isEmpty() && !me.getModifiers().contains(Modifier.STATIC)
            && me.getTypeParameters().isEmpty() && cx.kind(me.getReturnType()) == Ctx.K.STR) {
          m.put(r.getTypeParameters().get(1), me.getReturnType());
        }
      }
      for (TypeParameterElement tp : r.getTypeParameters()) {
        TypeMirror a = m.get(tp);
        if (a == null) return left(key, "a type argument is not inferred");
        ta.add(cx.goTypeArg(a));
      }
      for (VariableElement pe : r.getParameters()) params.add(cx.erase(cx.subst(pe.asType(), m, 0)));
      ret = cx.erase(cx.subst(r.getReturnType(), m, 0));
      if (mine && key.equals(COMPARING) && r.getTypeParameters().size() == 2) {
        // piece `nboxarg`: a key that is a boxed integral primitive (Integer, Long, ...) is held as Nbox, the
        // function given at Function<T, Nbox<X>> (LibNbox), the twin Comparator_comparingN_
        TypeMirror u = m.get(r.getTypeParameters().get(1));
        DeclaredType nf = nboxArgOn() ? cx.nboxFn(params.get(0)) : null;
        TypeMirror pu = u == null ? null : cx.unboxed(u);
        if (nf != null && pu != null && !u.getKind().isPrimitive() && INTEGRAL.contains(pu.getKind())) {
          f = new Fn("Comparator_comparingN_[$0, " + cx.goType(pu) + "]", "", List.of("Function[$0, Nbox[" + cx.goType(pu) + "]]"), "Comparator[$0]");
          params.set(0, nf);
          ta.set(1, cx.goType(pu));
          lg.note("nboxarg: Comparator.comparing with a boxed key at Nbox (typed)", key);
        }
      }
    } else {
      // the receiver at the typed interface of the method's owner (piece `seqcoll`: of the
      // interface the owner extends, RECV_AT)
      TypeElement own = cx.ownerOf(r), lib = own;
      String up = TYPED_SEQ.containsKey(key) ? RECV_AT.get(own.getQualifiedName().toString()) : null;
      if (up != null) lib = cx.elems.getTypeElement(up);
      if (!lg.isLib(lib) || recv.type == null) return left(key, "the owner has no typed interface");
      TypeMirror at = cx.erase(recv.type);
      DeclaredType view;
      Ctx.CollKind k = cx.typedColl(at);
      if (k != null) {
        // (a method the typed collection has itself, TMap.computeIfAbsent: that one)
        List<TypeMirror> ats = new ArrayList<>();
        for (ExprEmitter.Arg a : as) ats.add(a.type);
        if (cx.typedMethod(recv.type, e, ats) != null) return null;
        List<? extends TypeMirror> sa = ((DeclaredType) at).getTypeArguments();
        if (k.builds != null || sa.size() != lib.getTypeParameters().size()) return left(key, "the receiver is no typed collection of the owner");
        TypeMirror to = cx.erase(cx.types.getDeclaredType(lib, sa.toArray(new TypeMirror[0])));
        rc = lg.is(to) ? lg.fromTypedColl(recv.code, at, to) : null;
        if (rc == null) return left(key, "the receiver is no typed collection of the owner");
        view = (DeclaredType) to;
      } else {
        view = lg.typedView(recv.type, lib);
        if (view == null) return left(key, "the receiver is erased");
        rc = cx.coerce(recv.code, recv.type, view);
      }
      if (!cx.goType(view).equals(fill(f.recv(), ta = goArgs(view)))) return left(key, "the receiver's type arguments");
      // (the owner itself at the view's type arguments, where the view is of the interface it extends)
      DeclaredType at0 = own.equals(lib) ? view : cx.types.getDeclaredType(own, view.getTypeArguments().toArray(new TypeMirror[0]));
      ExecutableType mt = (ExecutableType) cx.types.asMemberOf(at0, r);
      for (TypeMirror p : mt.getParameterTypes()) params.add(cx.erase(p));
      ret = cx.erase(mt.getReturnType());
      // (the String key at its type)
      if (key.endsWith("#s")) params.set(0, cx.elems.getTypeElement("java.lang.String").asType());
    }
    // (piece `fns2`: a list of Objects for an erased place is the erased list that its readers know)
    if (mine && key.equals("java.util.Collections.singletonList(java.lang.Object)") && ta.get(0).equals("any")) return left(key, "an element of type Object: the erased list serves");
    // the twin's Go signature is the call's
    for (int i = 0; i < params.size(); i++) {
      if (!cx.goType(params.get(i)).equals(fill(f.params().get(i), ta))) return left(key, "a parameter's Go type is not the twin's");
    }
    if (f.ret().isEmpty() ? ret.getKind() != TypeKind.VOID : ret.getKind() == TypeKind.VOID || !cx.goType(ret).equals(fill(f.ret(), ta))) return left(key, "the result's Go type is not the twin's");
    if (ITERABLE_ONCE.contains(key)) {
      // (the call's type: Iterable of the list's element type)
      ret = cx.erase(cx.types.getDeclaredType(cx.elems.getTypeElement("java.lang.Iterable"), ((DeclaredType) ret).getTypeArguments().toArray(new TypeMirror[0])));
    }
    // collections enter as they are
    for (int i = 0; i < params.size(); i++) {
      if (lg.is(params.get(i)) && !enters(as.get(i).type, params.get(i))) return left(key, "a collection argument is erased or of other element types");
    }
    if (st) x.curInst = null;
    if (mine && !st && !r.isDefault()) {
      // (the erased method stays declared by the runtime's interface, whose own code calls it:
      // the generated stub of the interface keeps it, as when the call named it)
      List<TypeMirror> ep = new ArrayList<>(cx.groupParams(r));
      ep.replaceAll(lg::rt);
      cx.noteMethodOn(cx.ownerOf(r), cx.methodName(r), ep, lg.rt(cx.groupReturn(r)));
    }
    if (mine) {
      used.add(key);
      lg.note("typed library functions (" + (TYPED_RT.containsKey(key) ? "rtm" : "fns2") + "): calls of the typed twin", key);
    } else lg.note("typed library functions: calls of the typed twin", key);
    String al = x.argList(r, params, as);
    // (the twins of the String-key helpers have the value type alone)
    String head = f.go().contains("[") ? fill(f.go(), ta) : f.go() + "[" + String.join(", ", key.endsWith("#s") ? ta.subList(1, ta.size()) : ta) + "]";
    if (mine && head.endsWith("[]")) head = head.substring(0, head.length() - 2);
    String code = head + "(" + (rc == null ? "" : rc + (al.isEmpty() ? "" : ", ")) + al + ")";
    return new ExprEmitter.CallResult(code, ret);
  }

  /**
   * Pieces `rtm` and `fns2`: the call of the typed twin of a runtime method (RT_METHODS) on a
   * receiver of its class or typed collection kind, or null where the call stays as it was.
   */
  ExprEmitter.CallResult rtMethod(ExprEmitter x, ExecutableElement e, ExprEmitter.Recv recv, List<ExprEmitter.Arg> as, boolean toObject) {
    if (recv == null || recv.type == null || e.getModifiers().contains(Modifier.STATIC)) return null;
    String key = key(cx, e);
    RtM m = RT_METHODS.get(key);
    if (m == null) m = RT_METHODS.get(key = key(cx, cx.root(e)));
    if (m == null) return null;
    String piece = m.go().equals("split_sT") ? "fns2" : "rtm";
    if (piece.equals("fns2") ? !onF2() : !onRt()) return null;
    String what = "typed runtime methods (" + piece + "): ";
    if (as.size() != e.getParameters().size() || e.isVarArgs()) return leftRt(what, key, "arity");
    TypeMirror rt = cx.erase(recv.type);
    if (rt == null || rt.getKind() != TypeKind.DECLARED) return leftRt(what, key, "the receiver is no class");
    List<? extends TypeMirror> targs = List.of();
    TypeMirror self = rt;
    String fn = null;
    switch (m.recv()) {
      case "list", "set", "map" -> {
        Ctx.CollKind k = cx.typedColl(rt);
        if (k != null) {
          if (!k.name.equals(m.recv()) || k.builds != null) return leftRt(what, key, "the receiver is no typed collection of the kind");
          targs = ((DeclaredType) rt).getTypeArguments();
        } else if (m.recv().equals("list") && cx.lg.isCls(recv.type)) {
          // a place of the class that holds its typed List (piece cls): the twin taking that List
          DeclaredType v = cx.lg.typedView(recv.type, cx.elems.getTypeElement("java.util.List"));
          Ctx.CollKind lk = cx.collKinds.stream().filter(c -> c.name.equals("list")).findFirst().orElse(null);
          if (v == null || lk == null || !cx.goType(cx.erase(v)).startsWith("ListT[")) return leftRt(what, key, "the receiver is erased");
          targs = v.getTypeArguments();
          self = cx.typedOf(lk, new ArrayList<>(targs));
          if (self == null || !cx.goType(self).startsWith("*TList[")) return leftRt(what, key, "no typed list of the element type");
          fn = "ImmutableList_" + m.go().replace("Imm_", "T_");
          if (!cx.conv.rtGenerics.contains(fn)) return leftRt(what, key, "the runtime has no " + fn);
          fn += "[" + cx.goTypeArg(targs.get(0)) + "]";
          recv = new ExprEmitter.Recv(cx.coerce(recv.code, recv.type, cx.erase(v)), cx.erase(v));
        } else return leftRt(what, key, "the receiver is no typed collection");
      }
      case "java.lang.Enum" -> {
        TypeElement te = Ctx.typeElement(rt);
        if (te == null || te.getKind() != ElementKind.ENUM || !cx.isSource(te) || !cx.goType(rt).startsWith("*")) return leftRt(what, key, "the receiver is no enum object of the program");
      }
      default -> {
        if (!Ctx.typeElement(rt).getQualifiedName().contentEquals(m.recv())) return leftRt(what, key, "the receiver is of another class");
      }
    }
    // the arguments at the twin's types
    List<TypeMirror> params = new ArrayList<>();
    for (int i = 0; i < as.size(); i++) {
      String p = m.params().get(i);
      TypeMirror at = as.get(i).type == null ? null : cx.erase(as.get(i).type);
      TypeMirror pt;
      if (p.equals("int")) pt = cx.types.getPrimitiveType(TypeKind.INT);
      else if (p.equals("boolean")) pt = cx.types.getPrimitiveType(TypeKind.BOOLEAN);
      else if (p.startsWith("$")) {
        pt = cx.erase(targs.get(Integer.parseInt(p.substring(1))));
        if (at == null || at.getKind() == TypeKind.NULL || !cx.types.isAssignable(at, pt) || !cx.goType(at).equals(cx.goType(pt))) return leftRt(what, key, "an argument is not a value of the type argument's Go type");
      } else {
        pt = cx.types.erasure(cx.elems.getTypeElement(p).asType());
        if (p.equals("java.lang.String") ? at == null || cx.kind(at) != Ctx.K.STR : at == null || !cx.types.isAssignable(at, pt)) return leftRt(what, key, "an argument is not of the twin's type");
      }
      params.add(pt);
    }
    ExecutableElement root = cx.root(e);
    if (cx.ownerOf(root).getKind().isInterface() && !root.isDefault()) {
      // (the erased method stays declared by the runtime's interface, whose own code calls it)
      List<TypeMirror> ep = new ArrayList<>(cx.groupParams(root));
      ep.replaceAll(lg::rt);
      cx.noteMethodOn(cx.ownerOf(root), cx.methodName(root), ep, lg.rt(cx.groupReturn(root)));
    }
    TypeMirror ret = m.ret().equals("=") ? cx.erase(e.getReturnType()) : m.ret().equals("@") ? self : cx.types.erasure(cx.elems.getTypeElement(m.ret()).asType());
    used.add(key);
    lg.note(what + "calls of the typed twin", key);
    String al = x.argList(e, params, as);
    String code = fn != null ? fn + "(" + recv.code + (al.isEmpty() ? "" : ", " + al) + ")" : ExprEmitter.atomOrParen(recv.code) + "." + m.go() + "(" + al + ")";
    return new ExprEmitter.CallResult(code, ret);
  }

  <T> T leftRt(String what, String key, String why) {
    lg.note(what + "calls left (" + why + ")", key);
    return null;
  }

  /**
   * Piece `fns2`: `Comparator.thenComparing(Function)` whose key is a String, on a typed
   * Comparator: the twin that compares the keys as Strings (Comparator_thenComparingS_), without
   * boxing them; else null.
   */
  ExprEmitter.CallResult thenComparing(ExprEmitter x, ExecutableElement e, ExprEmitter.Recv recv, List<ExprEmitter.Arg> as) {
    if (!onF2() || recv == null || recv.type == null || as.size() != 1 || e.getModifiers().contains(Modifier.STATIC)) return null;
    String key = key(cx, e);
    if (!key.equals("java.util.Comparator.thenComparing(java.util.function.Function)")) return null;
    String what = "typed library functions (fns2): ";
    ExecutableType inst = x.curInst;
    if (inst == null || inst.getParameterTypes().size() != 1 || e.getTypeParameters().size() != 1) return leftRt(what, key, "no instantiated type");
    Map<Element, TypeMirror> m = new HashMap<>();
    ExprEmitter.unify(e.getParameters().get(0).asType(), inst.getParameterTypes().get(0), m);
    TypeMirror u = m.get(e.getTypeParameters().get(0));
    if (u == null || cx.kind(u) != Ctx.K.STR) return leftRt(what, key, "the key is no String");
    TypeMirror ct = cx.erase(recv.type);
    if (!cx.fn.is(ct) || ((DeclaredType) ct).getTypeArguments().size() != 1) return leftRt(what, key, "the receiver is no typed Comparator");
    TypeMirror t = ((DeclaredType) ct).getTypeArguments().get(0);
    TypeMirror ft = cx.erase(cx.types.getDeclaredType(cx.elems.getTypeElement("java.util.function.Function"), t, u));
    String gt = cx.goTypeArg(t);
    if (!cx.goType(ct).equals("Comparator[" + gt + "]") || !cx.goType(ft).equals("Function[" + gt + ", string]")) return leftRt(what, key, "the Go types are not the twin's");
    if (!cx.conv.rtGenerics.contains("Comparator_thenComparingS_")) return leftRt(what, key, "the runtime has no Comparator_thenComparingS_");
    used.add(key);
    lg.note(what + "calls of the typed twin", key);
    return new ExprEmitter.CallResult("Comparator_thenComparingS_[" + gt + "](" + recv.code + ", " + x.argList(e, List.of(ft), as) + ")", ct);
  }

  /** J2GO_LIBGEN: the names of the lists of pieces `rtm` and `fns2` that no call took (a stale entry). */
  void reportUnused() {
    if (!LibGen.REPORT) return;
    List<String> all = new ArrayList<>();
    if (onF2()) { all.addAll(TYPED_F2.keySet()); all.add("java.util.Comparator.thenComparing(java.util.function.Function)"); }
    if (onRt()) all.addAll(TYPED_RT.keySet());
    for (var en : RT_METHODS.entrySet()) if (en.getValue().go().equals("split_sT") ? onF2() : onRt()) all.add(en.getKey());
    for (String k : all) if (!used.contains(k)) System.err.println("libgen: rtm/fns2: on the list with no call: " + k);
  }

  /** The Go type arguments of typed interface type v. */
  List<String> goArgs(DeclaredType v) {
    List<String> out = new ArrayList<>();
    for (TypeMirror a : v.getTypeArguments()) out.add(cx.goTypeArg(a));
    return out;
  }

  /**
   * `new TreeSet<>(comparator)` / `new TreeMap<>(comparator)` of a typed collection of type
   * target (the runtime's newTSortedSetC_ / newTSortedMapC_, whose comparator compares the boxes
   * of the keys): the name of the twin that takes the comparator at the key type, where its type
   * Comparator[K] is the one of the typed collection's key type; else null. pt: the erased
   * parameter's type (Comparator).
   */
  String sortedComparator(String fn, TypeMirror target, TypeMirror pt) {
    if (!on() || !fn.equals("SortedSet") && !fn.equals("SortedMap")) return null;
    String go = "newT" + fn + "CT_";
    if (!cx.conv.rtGenerics.contains(go)) return left(go, "the runtime has no " + go);
    TypeMirror k = ((DeclaredType) target).getTypeArguments().get(0);
    TypeMirror ct = cx.erase(cx.types.getDeclaredType(Ctx.typeElement(pt), k));
    String gk = cx.goTypeArg(k), gt = cx.goType(target);
    if (!cx.goType(ct).equals("Comparator[" + gk + "]") || !gt.startsWith("*T" + fn.substring(6) + "[" + gk + (fn.equals("SortedSet") ? "]" : ", "))) return left(go, "the comparator's Go type is not the key type's");
    lg.note("typed library functions: calls of the typed twin", go);
    return go;
  }

  /** The twin of typed maker mk (Ctx.TYPED_MAKERS) that takes argument a, of Java type at, as a Go string (it is a String), or null. */
  String stringMaker(String mk, ExpressionTree a, TypeMirror at) {
    if (!on() || !mk.equals("splitToListT_") || cx.kind(at) != Ctx.K.STR) return null;
    lg.note("typed library functions: calls of the typed twin", "splitToListST_");
    return "splitToListST_";
  }
}
