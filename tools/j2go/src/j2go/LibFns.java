package j2go;

import com.sun.source.tree.ExpressionTree;
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
    if (!on() || lg.libs.isEmpty() || cx.isSource(cx.ownerOf(e))) return null;
    boolean st = e.getModifiers().contains(Modifier.STATIC);
    if (!st && recv == null) return null;
    ExecutableElement r = st ? e : cx.root(e);
    String key = key(cx, r);
    // a String key: the twin of the String-key helper (ExprEmitter.stringKeyHelper)
    if (!st && !as.isEmpty() && TYPED.containsKey(key + "#s") && cx.kind(as.get(0).type) == Ctx.K.STR) key += "#s";
    Fn f = TYPED.get(key);
    if (f == null && LibGen.on("seqcoll")) f = TYPED_SEQ.get(key);
    if (f == null) {
      String why = REFUSED.get(key);
      return why == null ? null : left(key, "no typed twin: " + why);
    }
    if (as.size() != r.getParameters().size() || r.isVarArgs()) return left(key, "arity");
    // (an element given as Object: Java casts nothing there, so an element of another class that
    // unchecked code put into the collection passes; the twin would give it at its type)
    if (toObject && f.ret().matches("\\$\\d")) return left(key, "the result enters an Object");
    if (!cx.conv.rtGenerics.contains(f.go())) return left(key, "the runtime has no " + f.go());
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
      for (TypeParameterElement tp : r.getTypeParameters()) {
        TypeMirror a = m.get(tp);
        if (a == null) return left(key, "a type argument is not inferred");
        ta.add(cx.goTypeArg(a));
      }
      for (VariableElement pe : r.getParameters()) params.add(cx.erase(cx.subst(pe.asType(), m, 0)));
      ret = cx.erase(cx.subst(r.getReturnType(), m, 0));
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
    // the twin's Go signature is the call's
    for (int i = 0; i < params.size(); i++) {
      if (!cx.goType(params.get(i)).equals(fill(f.params().get(i), ta))) return left(key, "a parameter's Go type is not the twin's");
    }
    if (f.ret().isEmpty() ? ret.getKind() != TypeKind.VOID : ret.getKind() == TypeKind.VOID || !cx.goType(ret).equals(fill(f.ret(), ta))) return left(key, "the result's Go type is not the twin's");
    // collections enter as they are
    for (int i = 0; i < params.size(); i++) {
      if (lg.is(params.get(i)) && !enters(as.get(i).type, params.get(i))) return left(key, "a collection argument is erased or of other element types");
    }
    if (st) x.curInst = null;
    lg.note("typed library functions: calls of the typed twin", key);
    String al = x.argList(r, params, as);
    // (the twins of the String-key helpers have the value type alone)
    String code = f.go() + "[" + String.join(", ", key.endsWith("#s") ? ta.subList(1, ta.size()) : ta) + "](" + (rc == null ? "" : rc + (al.isEmpty() ? "" : ", ")) + al + ")";
    return new ExprEmitter.CallResult(code, ret);
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
