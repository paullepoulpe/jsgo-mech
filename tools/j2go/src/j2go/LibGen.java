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
 * Kinds: `iter` (java.util.Iterator, java.lang.Iterable).
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
  }

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
    return norm(t);
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
    if (c == null || !cx.isSource(c) || c.getKind().isInterface() || !cx.isLeaf(c) || cx.isHandle(c)) return false;
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
    return isLib(lib) && typedView(site, lib) != null;
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

  /** A typed collection (list, set, deque) as the Iterable of its elements: itself. */
  String fromTypedColl(String code, TypeMirror from, TypeMirror to) {
    if (!is(to) || !Ctx.typeElement(to).getQualifiedName().contentEquals("java.lang.Iterable")) return null;
    Ctx.CollKind k = cx.typedColl(from);
    if (k == null || k.builds != null || k.pair()) return null;
    TypeMirror elem = ((DeclaredType) from).getTypeArguments().get(0);
    if (!cx.goTypeArg(elem).equals(cx.goTypeArg(((DeclaredType) to).getTypeArguments().get(0)))) return null;
    return code + ".asIterable()";
  }

  /** Ctx.coerce where from or to is a typed use of one of the interfaces (both erased); null: as any other interface. */
  String coerce(String code, TypeMirror from, TypeMirror to) {
    Ctx.K kf = cx.kind(from), kt = cx.kind(to);
    if (is(to)) {
      TypeElement lib = Ctx.typeElement(to);
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

  // ------------------------------------------------------------------ refusals

  final List<String> refusals = new ArrayList<>();

  /** A raw use of one of the interfaces has no element type to give the typed methods: refused (the tree has none). */
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

      @Override
      public Void visitTypeCast(TypeCastTree t, Void v) {
        raw(t, trees.getTypeMirror(new TreePath(getCurrentPath(), t.getType())), 0);
        return super.visitTypeCast(t, v);
      }

      @Override
      public Void visitNewClass(NewClassTree t, Void v) {
        if (t.getClassBody() == null) raw(t, trees.getTypeMirror(getCurrentPath()), 0);
        return super.visitNewClass(t, v);
      }
    }.scan(cu, null);
  }
}
