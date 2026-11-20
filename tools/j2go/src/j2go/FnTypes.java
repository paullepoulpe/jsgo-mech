package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.TreePath;
import com.sun.source.util.TreePathScanner;
import com.sun.source.util.Trees;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Rule "function types" (J2GO_NOFNTYPES=1 turns it off): a library functional interface is a
 * named Go func type of its Java type arguments, with the interface's method declared on it.
 *
 * <pre>
 *   type Predicate[T any] func(T) bool
 *   func (f Predicate[T]) test(p0 T) bool { return f(p0) }
 * </pre>
 *
 * Without the rule such an interface is erased (`type Predicate interface { test(any) bool }`),
 * a lambda is a func literal inside an adapter object (`&Predicate_tfn_Node{func(n Node) bool
 * {..}}`) and a class implementing it has the erased method (`test(p0 any)`), which casts.
 * With it:
 *
 * - the interfaces of LIBS are typed declarations as java.util.Optional is (Ctx.typedClasses,
 *   their type parameters kept): `Function<Node, String>` is `Function[Node, string]`. A
 *   wildcard is its bound (`? super X` and `? extends X` are X, `?` is `any`), an erased type
 *   variable, `Object` and a boxed primitive are `any` (norm);
 * - a lambda or a method reference is the func literal at the type of the place it goes to;
 * - a call of the interface's method is a call of the method declared on the func type (Go
 *   calls it statically); a null function is nil, and calling it is the nil dereference that
 *   the runtime reports as a NullPointerException;
 * - a class of the program that implements the interface has the method with its Java types
 *   and no bridge; where an object of it is given for the interface, the value is its method
 *   value (`x.apply`), nil for null;
 * - a function meets another instantiation of its interface (variance, an erased type variable
 *   of a generic method, the erased signature of a runtime function) through a closure that
 *   converts the arguments and the result as the erased code converted them (coerce);
 * - a function meets `Object` (an element of an erased collection, a type variable) as its
 *   instantiation at `any` in a box (`f.box_()`, nil for null), so that the box has one Go type
 *   whatever the instantiation was.
 *
 * A Go func value has no identity and no equals. Java gives a lambda no identity a program
 * may rely on, but a program can still write the comparison; the translator refuses (check):
 * `==` between two functional values, `instanceof` and casts of or to one, one as a key of a
 * map or an element of a set, and one given where `Object` is declared.
 */
final class FnTypes {
  /** J2GO_NOFNTYPES=1: the erased translation of library functional interfaces. */
  static final boolean ON = !"1".equals(System.getenv("J2GO_NOFNTYPES"));
  /** J2GO_FNTYPES=1 reports what the rule left converting. */
  static final boolean REPORT = System.getenv("J2GO_FNTYPES") != null;

  /**
   * The library functional interfaces that are Go func types. A name list: the runtime must
   * know each (a runtime class implementing one keeps the erased method, and a default method
   * the program calls is a method of the func type in rt_fn.go).
   */
  static final List<String> LIBS = List.of(
      "java.util.function.Supplier", "java.util.function.Function", "java.util.function.Predicate",
      "java.util.function.Consumer", "java.util.function.BiFunction", "java.util.function.BiConsumer",
      "java.util.function.BiPredicate", "java.util.function.ToIntFunction", "java.util.function.ToLongFunction",
      "java.util.function.IntFunction", "com.google.common.base.Supplier", "com.google.common.base.Function",
      "com.google.common.base.Predicate", "java.util.Comparator", "java.util.concurrent.Callable");

  /**
   * Library classes whose static generic methods the runtime has as Go generic functions of
   * the method's type parameters (Predicates.alwaysTrue[T]), as it has those of the functional
   * interfaces themselves and of java.util.Optional.
   */
  static final List<String> STATIC_OWNERS = List.of(
      "com.google.common.base.Predicates", "com.google.common.base.Suppliers", "com.google.common.base.Functions");

  final Set<TypeElement> staticOwners = new HashSet<>();

  final Ctx cx;
  final Set<TypeElement> libs = new LinkedHashSet<>();
  /** What the rule converts, by kind (REPORT). */
  final TreeMap<String, Integer> counts = new TreeMap<>();
  final TreeMap<String, Integer> sites = new TreeMap<>();

  FnTypes(Ctx cx) {
    this.cx = cx;
  }

  void register() {
    if (!ON) return;
    for (String qn : LIBS) {
      TypeElement te = cx.elems.getTypeElement(qn);
      if (te == null || te.getTypeParameters().isEmpty()) continue;
      libs.add(te);
      staticOwners.add(te);
      cx.typedClasses.add(te);
      cx.keptVars.addAll(te.getTypeParameters());
      // its static generic methods are Go generic functions of the runtime (Comparator.comparing)
    }
    for (String qn : STATIC_OWNERS) {
      TypeElement te = cx.elems.getTypeElement(qn);
      if (te != null) staticOwners.add(te);
    }
    for (TypeElement te : staticOwners) {
      for (Element e : te.getEnclosedElements()) {
        if (e instanceof ExecutableElement m && m.getModifiers().contains(Modifier.STATIC)) cx.keptVars.addAll(m.getTypeParameters());
      }
    }
  }

  /** Whether the static generic methods of library class te are Go generic functions. */
  boolean genericStatics(TypeElement te) {
    return te != null && staticOwners.contains(te);
  }

  boolean isLib(TypeElement te) {
    return te != null && libs.contains(te);
  }

  boolean is(TypeMirror t) {
    return t != null && t.getKind() == TypeKind.DECLARED && libs.contains(Ctx.typeElement(t));
  }

  void note(String kind, String site) {
    counts.merge(kind, 1, Integer::sum);
    if (site != null) sites.merge(kind + ": " + site, 1, Integer::sum);
  }

  void report() {
    if (!REPORT) return;
    counts.forEach((k, n) -> System.err.println("fntypes: " + n + " " + k));
    if ("2".equals(System.getenv("J2GO_FNTYPES"))) sites.forEach((k, n) -> System.err.println("fntypes: site " + n + " " + k));
  }

  TypeMirror object() {
    return cx.elems.getTypeElement("java.lang.Object").asType();
  }

  static boolean captured(TypeMirror t) {
    return t != null && t.getKind() == TypeKind.TYPEVAR && t.toString().startsWith("capture");
  }

  /** The type argument that stands for a: a wildcard or a captured type variable is its bound. */
  TypeMirror normArg(TypeMirror a, int depth) {
    if (a == null || depth > 8) return object();
    if (a.getKind() == TypeKind.WILDCARD) {
      WildcardType w = (WildcardType) a;
      TypeMirror b = w.getExtendsBound() != null ? w.getExtendsBound() : w.getSuperBound();
      return b == null ? object() : normArg(b, depth + 1);
    }
    if (captured(a)) {
      TypeVariable v = (TypeVariable) a;
      TypeMirror lo = v.getLowerBound();
      if (lo != null && lo.getKind() != TypeKind.NULL) return normArg(lo, depth + 1);
      return normArg(v.getUpperBound(), depth + 1);
    }
    if (a.getKind() == TypeKind.INTERSECTION) return normArg(((IntersectionType) a).getBounds().get(0), depth + 1);
    // an erased type variable: what the erased code has in its place (its bound)
    if (a.getKind() == TypeKind.TYPEVAR && !cx.typedVar(a)) return normArg(cx.erase(a), depth + 1);
    if (is(a)) return norm(a, depth + 1);
    return a;
  }

  /** Functional type t with the type arguments the Go type has (a raw use: Object for each). */
  TypeMirror norm(TypeMirror t) {
    return norm(t, 0);
  }

  TypeMirror norm(TypeMirror t, int depth) {
    DeclaredType d = (DeclaredType) t;
    TypeElement te = Ctx.typeElement(t);
    int n = te.getTypeParameters().size();
    TypeMirror[] as = new TypeMirror[n];
    boolean has = d.getTypeArguments().size() == n, same = has;
    for (int i = 0; i < n; i++) {
      TypeMirror a = has ? d.getTypeArguments().get(i) : null;
      as[i] = a == null ? object() : normArg(a, depth);
      same &= as[i] == a;
    }
    return same ? t : cx.types.getDeclaredType(te, as);
  }

  /** The instantiation of t's interface at Object: the form a function has in a box. */
  TypeMirror raw(TypeMirror t) {
    TypeElement te = Ctx.typeElement(t);
    TypeMirror[] as = new TypeMirror[te.getTypeParameters().size()];
    Arrays.fill(as, object());
    return cx.types.getDeclaredType(te, as);
  }

  ExecutableElement sam(TypeMirror t) {
    return cx.functionalMethod(Ctx.typeElement(t));
  }

  /** Whether class or interface c gets the functional interface fi through a library type that is not itself one of LIBS (its method is the runtime's, erased). */
  boolean viaLibrary(TypeElement c, TypeElement fi) {
    if (c == null || isLib(c)) return false;
    if (!cx.isSource(c)) return true;
    TypeMirror fe = cx.types.erasure(fi.asType());
    for (TypeMirror st : cx.types.directSupertypes(c.asType())) {
      TypeElement s = Ctx.typeElement(st);
      if (s == null || s.getQualifiedName().contentEquals("java.lang.Object") || isLib(s)) continue;
      if (!cx.types.isSubtype(cx.types.erasure(st), fe)) continue;
      if (viaLibrary(s, fi)) return true;
    }
    return false;
  }

  /**
   * Whether the method group rooted in functional interface fi has its erased signature for a
   * receiver of type site: a class of the runtime, or a class of the program under one.
   */
  boolean erasedAt(TypeMirror site, TypeElement fi) {
    if (site == null) return false;
    TypeMirror s = site.getKind() == TypeKind.TYPEVAR ? ((TypeVariable) site).getUpperBound() : site;
    TypeElement se = Ctx.typeElement(s);
    return se != null && viaLibrary(se, fi);
  }

  /** `code` (a value of functional type from) as a value of functional type to: itself, or a closure converting arguments and result. */
  String between(String code, TypeMirror from, TypeMirror to) {
    String gf = cx.goType(from), gt = cx.goType(to);
    if (gf.equals(gt)) return code;
    ExecutableElement mf = sam(from), mt = sam(to);
    List<TypeMirror> pf = cx.groupParams(mf, from), pt = cx.groupParams(mt, to);
    TypeMirror rf = cx.groupReturn(mf, from), rt = cx.groupReturn(mt, to);
    boolean same = pf.size() == pt.size() && cx.goType(rf).equals(cx.goType(rt));
    for (int i = 0; same && i < pf.size(); i++) same = cx.goType(pf.get(i)).equals(cx.goType(pt.get(i)));
    // the same func type under two names (Guava's Predicate is java.util's)
    if (same) return gt + "(" + code + ")";
    if (pf.size() != pt.size()) throw new IllegalStateException("function types: " + from + " to " + to);
    note("closures converting between two instantiations", gf + " -> " + gt);
    StringBuilder ps = new StringBuilder(), as = new StringBuilder();
    for (int i = 0; i < pt.size(); i++) {
      ps.append(i > 0 ? ", " : "").append("p").append(i).append(" ").append(cx.goType(pt.get(i)));
      as.append(i > 0 ? ", " : "").append(cx.coerce("p" + i, pt.get(i), pf.get(i)));
    }
    boolean isVoid = rt.getKind() == TypeKind.VOID;
    String call = "f_(" + as + ")";
    String body = isVoid ? call : "return " + cx.coerce(call, rf, rt);
    String gr = cx.goType(rt);
    return "func() " + gt + " { f_ := " + code + "; if f_ == nil { return nil }; return func(" + ps + ")" + (gr.isEmpty() ? "" : " " + gr) + " { " + body + " } }()";
  }

  /** Ctx.coerce where from or to is a functional type (both erased already). */
  String coerce(String code, TypeMirror from, TypeMirror to) {
    boolean ff = is(from), ft = is(to);
    Ctx.K kf = cx.kind(from), kt = cx.kind(to);
    if (ff && ft) return between(code, from, to);
    if (ff) {
      if (kt == Ctx.K.ANY || kt == Ctx.K.TVAR) {
        note("functions boxed", cx.goType(from));
        String box = between(code, from, raw(from)) + ".box_()";
        return kt == Ctx.K.ANY ? box : "unboxT_[" + cx.goType(to) + "](" + box + ")";
      }
      throw new IllegalStateException("function types: a " + cx.goType(from) + " cannot become a " + cx.goType(to));
    }
    TypeElement fi = Ctx.typeElement(to);
    if (kf == Ctx.K.ANY || kf == Ctx.K.TVAR) {
      note("functions unboxed", cx.goType(to));
      String any = kf == Ctx.K.TVAR ? "boxT_(" + code + ")" : code;
      TypeMirror r = raw(to);
      return between("fnUnbox_[" + cx.goType(r) + "](" + any + ")", r, to);
    }
    if (kf == Ctx.K.CLS || kf == Ctx.K.IFACE) {
      // an object of a class implementing the interface: its method value
      TypeMirror f0 = from.getKind() == TypeKind.TYPEVAR ? ((TypeVariable) from).getUpperBound() : from;
      DeclaredType view = cx.superView(f0, fi);
      if (view == null) throw new IllegalStateException("function types: a " + cx.goType(from) + " is no " + cx.goType(to));
      TypeMirror shape = erasedAt(from, fi) ? raw(to) : norm(view);
      // the method the object has: that of the most specific functional interface it implements
      ExecutableElement m = sam(to);
      for (TypeElement l : libs) {
        if (cx.superView(f0, l) != null && cx.types.isSubtype(cx.types.erasure(l.asType()), cx.types.erasure(Ctx.typeElement(cx.erase(view)).asType()))
            && cx.types.isSubtype(cx.types.erasure(l.asType()), cx.types.erasure(cx.ownerOf(m).asType()))) m = cx.functionalMethod(l);
      }
      String name = cx.methodName(m);
      String gs = cx.goType(shape);
      note("objects given as a function (method values)", cx.goType(from) + " as " + cx.goType(to));
      if (!cx.isSource(Ctx.typeElement(f0))) {
        // a method of the runtime's type, with the erased signature
        TypeElement fe = Ctx.typeElement(f0);
        cx.noteType(fe);
        cx.noteMethodOn(fe, name, cx.groupParams(m, shape), cx.groupReturn(m, shape));
      }
      String mv;
      if (code.equals("this")) mv = gs + "(this." + name + ")";
      else mv = "func() " + gs + " { v_ := " + code + "; if v_ == " + cx.nil(from) + " { return nil }; return v_." + name + " }()";
      return between(mv, shape, to);
    }
    throw new IllegalStateException("function types: a " + cx.goType(from) + " cannot become a " + cx.goType(to));
  }

  // ------------------------------------------------------------------ refusals

  final List<String> refusals = new ArrayList<>();
  /** Type parameters of classes of the program whose values are keys of a map or elements of a set (keyed). */
  final Set<TypeParameterElement> keyedVars = new HashSet<>();

  static final Map<String, int[]> KEYED_LIBS = Map.of(
      "java.util.Map", new int[] {0}, "java.util.Set", new int[] {0},
      "com.google.common.collect.Multimap", new int[] {0}, "com.google.common.collect.Multiset", new int[] {0},
      "com.google.common.collect.Table", new int[] {0, 1}, "com.google.common.collect.BiMap", new int[] {0, 1},
      "com.google.common.collect.ImmutableMap.Builder", new int[] {0}, "com.google.common.collect.ImmutableSet.Builder", new int[] {0},
      "com.google.common.cache.Cache", new int[] {0});

  /** The type arguments of t that its class compares (hashes): keys of maps, elements of sets, keyed type parameters of the program's classes. */
  void keyedArgs(TypeMirror t, List<TypeMirror> out) {
    if (t == null || t.getKind() != TypeKind.DECLARED) return;
    DeclaredType d = (DeclaredType) t;
    if (d.getTypeArguments().isEmpty()) return;
    TypeElement te = Ctx.typeElement(t);
    for (var e : KEYED_LIBS.entrySet()) {
      TypeElement k = cx.elems.getTypeElement(e.getKey());
      if (k == null || !cx.types.isSubtype(cx.types.erasure(t), cx.types.erasure(k.asType()))) continue;
      DeclaredType v = cx.superView(t, k);
      if (v == null) continue;
      for (int i : e.getValue()) if (i < v.getTypeArguments().size()) out.add(v.getTypeArguments().get(i));
    }
    if (cx.isSource(te)) {
      List<? extends TypeParameterElement> ps = te.getTypeParameters();
      for (int i = 0; i < ps.size() && i < d.getTypeArguments().size(); i++) if (keyedVars.contains(ps.get(i))) out.add(d.getTypeArguments().get(i));
    }
  }

  /** Every type that t mentions (itself, its type arguments, component types), depth first. */
  void mentions(TypeMirror t, List<TypeMirror> out, int depth) {
    if (t == null || depth > 8) return;
    switch (t.getKind()) {
      case DECLARED:
        out.add(t);
        for (TypeMirror a : ((DeclaredType) t).getTypeArguments()) mentions(a, out, depth + 1);
        return;
      case ARRAY:
        mentions(((ArrayType) t).getComponentType(), out, depth + 1);
        return;
      case WILDCARD:
        mentions(((WildcardType) t).getExtendsBound(), out, depth + 1);
        mentions(((WildcardType) t).getSuperBound(), out, depth + 1);
        return;
      default:
    }
  }

  /** keyedVars: a type parameter that a field of its class (or of a class nested in it) names at a compared position. */
  void findKeyed(Collection<TypeElement> sources) {
    boolean changed = true;
    while (changed) {
      changed = false;
      for (TypeElement te : sources) {
        for (Element e : te.getEnclosedElements()) {
          if (e.getKind() != ElementKind.FIELD) continue;
          List<TypeMirror> ms = new ArrayList<>();
          mentions(e.asType(), ms, 0);
          for (TypeMirror m : ms) {
            List<TypeMirror> ks = new ArrayList<>();
            keyedArgs(m, ks);
            for (TypeMirror k : ks) {
              if (k.getKind() == TypeKind.TYPEVAR && ((TypeVariable) k).asElement() instanceof TypeParameterElement tp && cx.isSource(ownerClass(tp)) && keyedVars.add(tp)) changed = true;
            }
          }
        }
      }
    }
  }

  static TypeElement ownerClass(TypeParameterElement tp) {
    Element g = tp.getGenericElement();
    while (g != null && !(g instanceof TypeElement)) g = g.getEnclosingElement();
    return (TypeElement) g;
  }

  boolean fn(TypeMirror t) {
    if (t == null) return false;
    if (captured(t)) t = ((TypeVariable) t).getUpperBound();
    return is(t);
  }

  /**
   * The uses of a functional value that a Go func value cannot serve; each is an error that
   * names its place (Main stops the translation).
   */
  void check(Trees trees, CompilationUnitTree cu) {
    if (!ON) return;
    String file = new java.io.File(cu.getSourceFile().getName()).getName();
    new TreePathScanner<Void, Void>() {
      final ArrayDeque<TypeMirror> rets = new ArrayDeque<>();

      void refuse(Tree t, String what) {
        long p = trees.getSourcePositions().getStartPosition(cu, t);
        refusals.add(file + ":" + (p < 0 ? 0 : cu.getLineMap().getLineNumber(p)) + ": " + what + ": " + t.toString().replace('\n', ' ').replaceAll("\\s+", " "));
      }

      TypeMirror typeOf(Tree t) {
        try {
          return trees.getTypeMirror(new TreePath(getCurrentPath(), t));
        } catch (RuntimeException ex) {
          return null;
        }
      }

      Element elemOf(Tree t) {
        try {
          return trees.getElement(new TreePath(getCurrentPath(), t));
        } catch (RuntimeException ex) {
          return null;
        }
      }

      boolean isObject(TypeMirror t) {
        return t != null && t.getKind() == TypeKind.DECLARED && Ctx.typeElement(t).getQualifiedName().contentEquals("java.lang.Object");
      }

      /** x enters a place declared `target`. */
      void flow(ExpressionTree x, TypeMirror target, String what) {
        if (x == null || target == null) return;
        while (x instanceof ParenthesizedTree p) x = p.getExpression();
        if (x instanceof ConditionalExpressionTree c) {
          flow(c.getTrueExpression(), target, what);
          flow(c.getFalseExpression(), target, what);
          return;
        }
        if (isObject(target) && fn(typeOf(x))) refuse(x, "a functional value given where Object is declared (" + what + ")");
      }

      void keyed(Tree at, TypeMirror t) {
        List<TypeMirror> ms = new ArrayList<>();
        mentions(t, ms, 0);
        for (TypeMirror m : ms) {
          List<TypeMirror> ks = new ArrayList<>();
          keyedArgs(m, ks);
          for (TypeMirror k : ks) {
            TypeMirror b = k.getKind() == TypeKind.WILDCARD ? normArg(k, 0) : k;
            if (fn(b)) refuse(at, "a functional value as a key of a map or an element of a set (" + m + ")");
          }
        }
      }

      @Override
      public Void visitBinary(BinaryTree t, Void v) {
        if (t.getKind() == Tree.Kind.EQUAL_TO || t.getKind() == Tree.Kind.NOT_EQUAL_TO) {
          TypeMirror l = typeOf(t.getLeftOperand()), r = typeOf(t.getRightOperand());
          boolean nl = l != null && l.getKind() == TypeKind.NULL, nr = r != null && r.getKind() == TypeKind.NULL;
          if (!nl && !nr && (fn(l) || fn(r))) refuse(t, "identity of a functional value");
        }
        return super.visitBinary(t, v);
      }

      @Override
      public Void visitInstanceOf(InstanceOfTree t, Void v) {
        TypeMirror to = t.getType() != null ? typeOf(t.getType()) : null;
        if (to == null && t.getPattern() instanceof BindingPatternTree bp) to = typeOf(bp.getVariable().getType());
        if (fn(typeOf(t.getExpression())) || fn(to)) refuse(t, "instanceof of a functional value");
        return super.visitInstanceOf(t, v);
      }

      @Override
      public Void visitTypeCast(TypeCastTree t, Void v) {
        TypeMirror to = typeOf(t.getType()), from = typeOf(t.getExpression());
        ExpressionTree x = t.getExpression();
        while (x instanceof ParenthesizedTree p) x = p.getExpression();
        boolean poly = x instanceof LambdaExpressionTree || x instanceof MemberReferenceTree;
        if (!poly && (fn(to) || fn(from))) {
          boolean up;
          try {
            up = from != null && to != null && cx.types.isSubtype(from, to) && fn(to);
          } catch (RuntimeException ex) {
            up = false;
          }
          if (!up) refuse(t, "cast of or to a functional value");
        }
        return super.visitTypeCast(t, v);
      }

      @Override
      public Void visitVariable(VariableTree t, Void v) {
        Element e = elemOf(t);
        if (e instanceof VariableElement ve) {
          keyed(t, ve.asType());
          if (t.getInitializer() != null) flow(t.getInitializer(), ve.asType(), "initializer");
        }
        return super.visitVariable(t, v);
      }

      @Override
      public Void visitAssignment(AssignmentTree t, Void v) {
        flow(t.getExpression(), typeOf(t.getVariable()), "assignment");
        return super.visitAssignment(t, v);
      }

      @Override
      public Void visitMethod(MethodTree t, Void v) {
        Element e = elemOf(t);
        TypeMirror rt = e instanceof ExecutableElement m ? m.getReturnType() : null;
        if (rt != null) keyed(t.getReturnType() != null ? t.getReturnType() : t, rt);
        rets.push(rt == null ? cx.types.getNoType(TypeKind.VOID) : rt);
        try {
          return super.visitMethod(t, v);
        } finally {
          rets.pop();
        }
      }

      @Override
      public Void visitLambdaExpression(LambdaExpressionTree t, Void v) {
        TypeMirror ft = typeOf(t), rt = null;
        if (ft instanceof DeclaredType dt) {
          ExecutableElement m = cx.functionalMethod(Ctx.typeElement(ft));
          try {
            if (m != null) rt = ((ExecutableType) cx.types.asMemberOf(dt, m)).getReturnType();
          } catch (RuntimeException ex) {
            rt = null;
          }
          // the declared result of the interface's method: Object only where the method says so
          if (m != null && rt != null && m.getReturnType().getKind() == TypeKind.TYPEVAR) rt = null;
        }
        if (rt != null && t.getBody() instanceof ExpressionTree body) flow(body, rt, "result of a lambda");
        rets.push(rt == null ? cx.types.getNoType(TypeKind.VOID) : rt);
        try {
          return super.visitLambdaExpression(t, v);
        } finally {
          rets.pop();
        }
      }

      @Override
      public Void visitReturn(ReturnTree t, Void v) {
        if (t.getExpression() != null && !rets.isEmpty()) flow(t.getExpression(), rets.peek(), "returned value");
        return super.visitReturn(t, v);
      }

      void args(ExecutableElement m, List<? extends ExpressionTree> as) {
        List<? extends VariableElement> ps = m.getParameters();
        for (int i = 0; i < as.size() && !ps.isEmpty(); i++) {
          int pi = Math.min(i, ps.size() - 1);
          TypeMirror p = ps.get(pi).asType();
          if (m.isVarArgs() && pi == ps.size() - 1 && p.getKind() == TypeKind.ARRAY) {
            TypeMirror at = typeOf(as.get(i));
            if (!(as.size() == ps.size() && at != null && (at.getKind() == TypeKind.ARRAY || at.getKind() == TypeKind.NULL))) p = ((ArrayType) p).getComponentType();
          }
          flow(as.get(i), p, "argument " + (i + 1) + " of " + cx.ownerOf(m).getSimpleName() + "." + m.getSimpleName());
        }
      }

      @Override
      public Void visitMethodInvocation(MethodInvocationTree t, Void v) {
        if (elemOf(t.getMethodSelect()) instanceof ExecutableElement m) {
          args(m, t.getArguments());
          if (t.getMethodSelect() instanceof MemberSelectTree ms && !m.getModifiers().contains(Modifier.STATIC)
              && cx.ownerOf(m).getQualifiedName().contentEquals("java.lang.Object") && fn(typeOf(ms.getExpression()))) {
            refuse(t, "Object." + m.getSimpleName() + " of a functional value");
          }
        }
        TypeMirror ty = trees.getTypeMirror(getCurrentPath());
        if (ty != null) keyed(t, ty);
        return super.visitMethodInvocation(t, v);
      }

      @Override
      public Void visitNewArray(NewArrayTree t, Void v) {
        TypeMirror ty = trees.getTypeMirror(getCurrentPath());
        if (ty != null && ty.getKind() == TypeKind.ARRAY && t.getInitializers() != null) {
          for (ExpressionTree x : t.getInitializers()) flow(x, ((ArrayType) ty).getComponentType(), "element of an array");
        }
        return super.visitNewArray(t, v);
      }

      @Override
      public Void visitNewClass(NewClassTree t, Void v) {
        if (trees.getElement(getCurrentPath()) instanceof ExecutableElement m) args(m, t.getArguments());
        TypeMirror ty = trees.getTypeMirror(getCurrentPath());
        if (ty != null) keyed(t, ty);
        return super.visitNewClass(t, v);
      }
    }.scan(cu, null);
  }
}
