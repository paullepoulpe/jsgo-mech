package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.TreePath;
import com.sun.source.util.TreePathScanner;
import com.sun.source.util.Trees;
import java.io.IOException;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;
import javax.lang.model.util.Types;

/**
 * STUDY ONLY (J2GO_LIBGEN=dir): a census of Java's library generic types in the program, from
 * javac's attribution: declarations (decls.tsv), flows of values into places of such types
 * (flows.tsv), classes implementing them (impls.tsv), uses of functional-interface values
 * (fnuses.tsv) and unchecked casts (casts.tsv). No rule uses it.
 */
final class LibGenScan {
  final Ctx cx;
  final Trees trees;
  final Types types;
  final List<String> decls = new ArrayList<>(), flows = new ArrayList<>(), impls = new ArrayList<>(), fnuses = new ArrayList<>(), casts = new ArrayList<>(), overrides = new ArrayList<>(), libapi = new ArrayList<>(), objparams = new ArrayList<>(), instofs = new ArrayList<>();
  final Map<Element, Integer> ids = new HashMap<>();
  final Map<Element, String[]> declRows = new LinkedHashMap<>();
  final Map<Element, TreeMap<String, Integer>> uses = new HashMap<>();

  LibGenScan(Ctx cx, Trees trees) {
    this.cx = cx;
    this.trees = trees;
    this.types = cx.types;
  }

  int id(Element e) {
    return ids.computeIfAbsent(e, k -> ids.size() + 1);
  }

  static boolean captured(TypeMirror t) {
    return t != null && t.getKind() == TypeKind.TYPEVAR && t.toString().startsWith("capture");
  }

  boolean stub(TypeElement te) {
    return te.getQualifiedName().toString().startsWith("j2go.rt.");
  }

  /** A parameterized or raw use of a generic library class or interface. */
  boolean libGen(TypeMirror t) {
    if (t == null || t.getKind() != TypeKind.DECLARED) return false;
    TypeElement te = Ctx.typeElement(t);
    return te != null && !cx.isSource(te) && !stub(te) && !te.getTypeParameters().isEmpty();
  }

  /** A library functional interface (generic or not), not an iteration or comparable type. */
  boolean libFn(TypeMirror t) {
    if (t == null || t.getKind() != TypeKind.DECLARED) return false;
    TypeElement te = Ctx.typeElement(t);
    if (te == null || cx.isSource(te) || stub(te) || !te.getKind().isInterface()) return false;
    return family(te).equals("functional");
  }

  final Map<TypeElement, String> famCache = new HashMap<>();

  String family(TypeElement te) {
    return famCache.computeIfAbsent(te, k -> {
      String qn = te.getQualifiedName().toString();
      TypeMirror er = types.erasure(te.asType());
      if (sub(er, "java.util.Iterator")) return "iteration";
      if (qn.equals("java.lang.Iterable")) return "iteration";
      if (qn.equals("java.lang.Comparable")) return "comparable";
      if (qn.equals("java.util.Optional") || qn.equals("com.google.common.base.Optional")) return "optional";
      if (sub(er, "java.util.Collection") || sub(er, "java.util.Map") || sub(er, "java.util.Map.Entry")) return "collection";
      if (qn.startsWith("com.google.common.collect.") && !te.getKind().isInterface()) return "collection";
      if (qn.startsWith("com.google.common.collect.") && cx.functionalMethod(te) == null) return "collection";
      if (te.getKind().isInterface() && cx.functionalMethod(te) != null) return "functional";
      if (qn.equals("java.lang.Class")) return "class";
      return "other";
    });
  }

  boolean sub(TypeMirror er, String qn) {
    TypeElement s = cx.elems.getTypeElement(qn);
    return s != null && types.isSubtype(er, types.erasure(s.asType()));
  }

  static final List<String> ORDER = List.of("raw", "wild", "super", "extends", "tvar-erased", "object", "box", "tvar-kept", "exact");

  static String worse(String a, String b) {
    return ORDER.indexOf(a) <= ORDER.indexOf(b) ? a : b;
  }

  /** The category of one type argument. */
  String argCat(TypeMirror a, int depth) {
    if (a == null || depth > 6) return "exact";
    switch (a.getKind()) {
      case WILDCARD: {
        WildcardType w = (WildcardType) a;
        if (w.getExtendsBound() != null) return worse("extends", argCat(w.getExtendsBound(), depth + 1));
        if (w.getSuperBound() != null) return worse("super", argCat(w.getSuperBound(), depth + 1));
        return "wild";
      }
      case TYPEVAR:
        if (captured(a)) return "wild";
        return cx.typedVar(a) ? "tvar-kept" : "tvar-erased";
      case ARRAY:
        return argCat(((ArrayType) a).getComponentType(), depth + 1);
      case DECLARED: {
        TypeElement te = Ctx.typeElement(a);
        if (te.getQualifiedName().contentEquals("java.lang.Object")) return "object";
        if (cx.unboxed(a) != null) return "box";
        return typeCat(a, depth + 1);
      }
      default:
        return "exact";
    }
  }

  /** The category of a declared type: the worst of its type arguments; raw when it names none of a generic class. */
  String typeCat(TypeMirror t, int depth) {
    if (t == null || t.getKind() != TypeKind.DECLARED) return "exact";
    DeclaredType d = (DeclaredType) t;
    TypeElement te = Ctx.typeElement(t);
    if (d.getTypeArguments().isEmpty()) return te.getTypeParameters().isEmpty() ? "exact" : (cx.isSource(te) ? "exact" : "raw");
    String c = "exact";
    for (TypeMirror a : d.getTypeArguments()) c = worse(c, argCat(a, depth));
    return c;
  }

  /** The categories of the top-level type arguments, joined. */
  String argCats(TypeMirror t) {
    if (!(t instanceof DeclaredType d) || d.getTypeArguments().isEmpty()) return "-";
    StringBuilder sb = new StringBuilder();
    for (TypeMirror a : d.getTypeArguments()) sb.append(sb.length() > 0 ? "," : "").append(argCat(a, 0));
    return sb.toString();
  }

  String qn(TypeMirror t) {
    TypeElement te = Ctx.typeElement(t);
    return te == null ? String.valueOf(t) : te.getQualifiedName().toString();
  }

  static String clean(Object o) {
    return String.valueOf(o).replace('\t', ' ').replace('\n', ' ');
  }

  TypeMirror uncapture(TypeMirror a) {
    if (captured(a)) {
      TypeMirror b = ((TypeVariable) a).getUpperBound();
      return b;
    }
    return a;
  }

  static final List<String> FORDER = List.of("raw-target", "raw-source", "sup-wider", "ext-sub", "wild-any", "src-wild", "sup-same", "ext-same", "wild-same", "exact", "null");

  static String fworse(String a, String b) {
    return FORDER.indexOf(a) <= FORDER.indexOf(b) ? a : b;
  }

  /** How a value of static type s enters a place of library generic type t. */
  String flowCat(TypeMirror s, TypeMirror t) {
    if (s == null || s.getKind() == TypeKind.NULL) return "null";
    DeclaredType td = (DeclaredType) t;
    TypeElement te = Ctx.typeElement(t);
    if (td.getTypeArguments().isEmpty()) return "raw-target";
    TypeMirror s0 = uncapture(s);
    DeclaredType sv = null;
    try {
      sv = cx.superView(s0, te);
    } catch (RuntimeException ex) {
      // fall through
    }
    if (sv == null) return "no-view";
    if (sv.getTypeArguments().isEmpty()) return "raw-source";
    String c = "exact";
    for (int i = 0; i < td.getTypeArguments().size() && i < sv.getTypeArguments().size(); i++) {
      TypeMirror ta = td.getTypeArguments().get(i), sa = sv.getTypeArguments().get(i);
      boolean swild = sa.getKind() == TypeKind.WILDCARD || captured(sa);
      TypeMirror sb = sa;
      if (sa.getKind() == TypeKind.WILDCARD) {
        WildcardType w = (WildcardType) sa;
        sb = w.getExtendsBound() != null ? w.getExtendsBound() : w.getSuperBound() != null ? w.getSuperBound() : cx.elems.getTypeElement("java.lang.Object").asType();
      } else if (captured(sa)) {
        sb = ((TypeVariable) sa).getUpperBound();
      }
      String k;
      if (ta.getKind() == TypeKind.WILDCARD) {
        WildcardType w = (WildcardType) ta;
        if (w.getExtendsBound() != null) k = same(sb, w.getExtendsBound()) ? "ext-same" : "ext-sub";
        else if (w.getSuperBound() != null) k = same(sb, w.getSuperBound()) ? "sup-same" : "sup-wider";
        else k = qn(sb).equals("java.lang.Object") ? "wild-same" : "wild-any";
      } else {
        k = swild ? "src-wild" : same(sa, ta) ? "exact" : "src-wild";
      }
      c = fworse(c, k);
    }
    return c;
  }

  boolean same(TypeMirror a, TypeMirror b) {
    try {
      return types.isSameType(a, b);
    } catch (RuntimeException ex) {
      return false;
    }
  }

  /** The owners of the type variables that declared parameter type p names: gm-lib, gm-prog-kept, gm-prog-erased, gc-lib, gc-prog-kept, gc-prog-erased. */
  void tvOwners(TypeMirror p, Set<String> out, int depth) {
    if (p == null || depth > 6) return;
    switch (p.getKind()) {
      case TYPEVAR: {
        if (captured(p)) return;
        Element tpe = ((TypeVariable) p).asElement();
        if (!(tpe instanceof TypeParameterElement tp)) return;
        Element g = tp.getGenericElement();
        boolean method = g instanceof ExecutableElement;
        TypeElement owner = method ? (TypeElement) g.getEnclosingElement() : (TypeElement) g;
        boolean prog = cx.isSource(owner);
        out.add((method ? "gm-" : "gc-") + (prog ? (cx.keptVars.contains(tp) ? "prog-kept" : "prog-erased") : "lib"));
        return;
      }
      case ARRAY:
        tvOwners(((ArrayType) p).getComponentType(), out, depth + 1);
        return;
      case WILDCARD: {
        WildcardType w = (WildcardType) p;
        tvOwners(w.getExtendsBound(), out, depth + 1);
        tvOwners(w.getSuperBound(), out, depth + 1);
        return;
      }
      case DECLARED:
        for (TypeMirror a : ((DeclaredType) p).getTypeArguments()) tvOwners(a, out, depth + 1);
        return;
      default:
    }
  }

  void scan(CompilationUnitTree cu) {
    String file = new java.io.File(cu.getSourceFile().getName()).getName();
    new TreePathScanner<Void, Void>() {
      final ArrayDeque<String> where = new ArrayDeque<>();
      final ArrayDeque<Object[]> rets = new ArrayDeque<>();  // {TypeMirror target, Element decl or null}

      long line(Tree t) {
        long p = trees.getSourcePositions().getStartPosition(cu, t);
        return p < 0 ? 0 : cu.getLineMap().getLineNumber(p);
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

      String w() {
        return where.isEmpty() ? "-" : where.peek();
      }

      ExpressionTree strip(ExpressionTree x) {
        while (x instanceof ParenthesizedTree p) x = p.getExpression();
        return x;
      }

      /** What kind of expression the source of a flow is. */
      String srcKind(ExpressionTree x0) {
        ExpressionTree x = strip(x0);
        if (x instanceof LambdaExpressionTree) return "lambda";
        if (x instanceof MemberReferenceTree) return "mref";
        if (x instanceof NewClassTree nc) return nc.getClassBody() != null ? "anon" : "new";
        if (x instanceof TypeCastTree) return "cast";
        if (x instanceof LiteralTree) return "null";
        if (x instanceof MethodInvocationTree mi) {
          Element e = elemOf(mi.getMethodSelect());
          if (e instanceof ExecutableElement m) {
            TypeElement o = (TypeElement) m.getEnclosingElement();
            Set<String> own = new TreeSet<>();
            tvOwners(m.getReturnType(), own, 0);
            return "call:" + (cx.isSource(o) ? "prog" : "lib") + ":" + o.getQualifiedName() + "." + m.getSimpleName() + (own.isEmpty() ? "" : ":" + String.join("+", own));
          }
          return "call:?";
        }
        if (x instanceof IdentifierTree || x instanceof MemberSelectTree) {
          Element e = elemOf(x);
          if (e instanceof VariableElement v) return (v.getKind() == ElementKind.FIELD ? "field:" : v.getKind() == ElementKind.PARAMETER ? "param:" : "local:") + id(v);
        }
        return "expr";
      }

      /** Records the flow of expression x into a place of type t (callee: for an argument; tdecl: the declaration that is the place, or null). */
      void flow(String kind, ExpressionTree x0, TypeMirror t, TypeMirror declared, String callee, Element tdecl) {
        if (x0 == null || t == null) return;
        ExpressionTree x = strip(x0);
        if (x instanceof ConditionalExpressionTree c) {
          flow(kind, c.getTrueExpression(), t, declared, callee, tdecl);
          flow(kind, c.getFalseExpression(), t, declared, callee, tdecl);
          return;
        }
        TypeMirror s = typeOf(x);
        if (s == null) return;
        boolean tg = libGen(t), sg = libGen(uncapture(s));
        boolean tfn = libFn(t);
        if (!tg && !tfn) {
          // a library generic value entering Object or a type variable
          if (!sg) return;
          String tk = t.getKind() == TypeKind.TYPEVAR ? "typevar" : qn(t).equals("java.lang.Object") ? "object" : null;
          if (tk == null) return;
          flows.add(String.join("\t", file, "" + line(x), w(), kind, "to-" + tk, family(Ctx.typeElement(uncapture(s))), qn(uncapture(s)), clean(s), clean(t), "-", srcKind(x), clean(callee), "-", "-",
              tdecl == null ? "0" : "" + id(tdecl), "-"));
          return;
        }
        TypeElement te = Ctx.typeElement(t);
        String cat = tg ? flowCat(s, t) : "exact";
        if (s.getKind() == TypeKind.NULL) cat = "null";
        Set<String> own = new TreeSet<>();
        if (declared != null) tvOwners(declared, own, 0);
        TypeMirror su = uncapture(s);
        String up = su.getKind() == TypeKind.DECLARED && !Ctx.typeElement(su).equals(te) ? qn(su) : "-";
        flows.add(String.join("\t", file, "" + line(x), w(), kind, cat, family(te), qn(t), clean(s), clean(t), tg ? typeCat(t, 0) : "nongeneric", srcKind(x), clean(callee),
            own.isEmpty() ? "-" : String.join("+", own), up, tdecl == null ? "0" : "" + id(tdecl), tg ? argCats(t) : "-"));
      }

      void decl(String kind, Element e, TypeMirror t, Tree at) {
        if (!libGen(t) && !libFn(t)) return;
        TypeElement te = Ctx.typeElement(t);
        boolean typedNow = e instanceof ExecutableElement m ? cx.typedResults.get(m) != null && cx.isTypedColl(cx.typedResults.get(m))
            : cx.isTypedColl(cx.vtype(e)) || (cx.refinedVars.get(e) != null && !cx.goType(cx.refinedVars.get(e)).equals(cx.goType(types.erasure(t))));
        String goNow;
        try {
          goNow = e instanceof ExecutableElement m ? cx.declType(cx.rtype(m)) : cx.declType(cx.vtype(e));
        } catch (RuntimeException ex) {
          goNow = "?";
        }
        declRows.put(e, new String[] {file, "" + line(at), w(), kind, family(te), qn(t), libGen(t) ? typeCat(t, 0) : "nongeneric", libGen(t) ? argCats(t) : "-", clean(t), typedNow ? "1" : "0", clean(goNow), "" + id(e),
            clean(e.getSimpleName())});
      }

      @Override
      public Void visitClass(ClassTree t, Void v) {
        Element e = elemOf(t);
        String n = e instanceof TypeElement te ? (te.getQualifiedName().length() > 0 ? te.getQualifiedName().toString() : "anon@" + file + ":" + line(t)) : "?";
        if (e instanceof TypeElement te) {
          for (TypeMirror st : types.directSupertypes(te.asType())) {
            if (libGen(st) || libFn(st)) {
              impls.add(String.join("\t", file, "" + line(t), n, te.getNestingKind().toString(), family(Ctx.typeElement(st)), qn(st), libGen(st) ? typeCat(st, 0) : "nongeneric", clean(st)));
            }
          }
        }
        where.push(n.substring(n.lastIndexOf('.') + 1));
        try {
          return super.visitClass(t, v);
        } finally {
          where.pop();
        }
      }

      @Override
      public Void visitMethod(MethodTree t, Void v) {
        Element e = elemOf(t);
        String cls = where.isEmpty() ? "?" : where.peek();
        where.push(cls + "." + t.getName());
        TypeMirror rt = e instanceof ExecutableElement m ? m.getReturnType() : null;
        if (e instanceof ExecutableElement m && rt != null && rt.getKind() != TypeKind.VOID) decl("return", m, rt, t);
        if (e instanceof ExecutableElement m && rt != null && (libGen(rt) || libFn(rt))) {
          ExecutableElement r = cx.root(m);
          String how = "root";
          String rrs = "-";
          if (r != m) {
            try {
              TypeMirror rr = ((ExecutableType) types.asMemberOf((DeclaredType) ((TypeElement) m.getEnclosingElement()).asType(), r)).getReturnType();
              rrs = clean(rr);
              if (same(rr, rt)) how = "same";
              else if (rr.getKind() == TypeKind.DECLARED && same(types.erasure(rr), types.erasure(rt))) how = "elem-differs:" + (libGen(rr) ? flowCat(rt, rr) : "?");
              else how = "container-differs:" + (libGen(rr) ? flowCat(rt, rr) : "root-not-generic");
            } catch (RuntimeException ex) {
              how = "error";
            }
          }
          TypeElement ro = (TypeElement) r.getEnclosingElement();
          overrides.add(String.join("\t", file, "" + line(t), w(), family(Ctx.typeElement(rt)), qn(rt), how, clean(rt), rrs, (cx.isSource(ro) ? "prog:" : "lib:") + ro.getQualifiedName() + "." + r.getSimpleName()));
        }
        rets.push(new Object[] {rt, e});
        try {
          return super.visitMethod(t, v);
        } finally {
          rets.pop();
          where.pop();
        }
      }

      @Override
      public Void visitVariable(VariableTree t, Void v) {
        Element e = elemOf(t);
        if (e instanceof VariableElement ve) {
          Tree parent = getCurrentPath().getParentPath().getLeaf();
          String kind = ve.getKind() == ElementKind.FIELD ? "field" : ve.getKind() == ElementKind.PARAMETER ? (parent instanceof LambdaExpressionTree ? "lambdaparam" : "param")
              : parent instanceof EnhancedForLoopTree ? "foreachvar" : ve.getKind() == ElementKind.EXCEPTION_PARAMETER ? "catch" : "local";
          decl(kind, ve, ve.asType(), t);
          if (t.getInitializer() != null) flow("init", t.getInitializer(), ve.asType(), null, "-", ve);
        }
        return super.visitVariable(t, v);
      }

      @Override
      public Void visitAssignment(AssignmentTree t, Void v) {
        TypeMirror lt = typeOf(t.getVariable());
        Element le = elemOf(t.getVariable());
        flow("assign", t.getExpression(), lt, null, "-", le instanceof VariableElement ? le : null);
        return super.visitAssignment(t, v);
      }

      @Override
      public Void visitReturn(ReturnTree t, Void v) {
        if (t.getExpression() != null && !rets.isEmpty() && rets.peek()[0] != null) {
          TypeMirror rt = (TypeMirror) rets.peek()[0];
          if (rt.getKind() != TypeKind.VOID) flow("return", t.getExpression(), rt, null, "-", (Element) rets.peek()[1]);
        }
        return super.visitReturn(t, v);
      }

      @Override
      public Void visitLambdaExpression(LambdaExpressionTree t, Void v) {
        TypeMirror ft = typeOf(t);
        TypeMirror rt = null;
        if (ft instanceof DeclaredType dt) {
          TypeElement fi = Ctx.typeElement(ft);
          ExecutableElement m = fi == null ? null : cx.functionalMethod(fi);
          if (m != null) {
            try {
              rt = ((ExecutableType) types.asMemberOf(dt, m)).getReturnType();
            } catch (RuntimeException ex) {
              rt = null;
            }
          }
          if (fi != null && cx.isSource(fi)) {
            flows.add(String.join("\t", file, "" + line(t), w(), "lambda-prog", "exact", "prog-functional", qn(ft), clean(ft), clean(ft), "-", "lambda", "-", "-", "-", "0", "-"));
          }
        }
        if (rt != null && rt.getKind() != TypeKind.VOID && t.getBody() instanceof ExpressionTree body) flow("lambdaret", body, rt, null, "-", null);
        rets.push(new Object[] {rt, null});
        try {
          return super.visitLambdaExpression(t, v);
        } finally {
          rets.pop();
        }
      }

      void args(ExecutableElement m, ExecutableType inst, List<? extends ExpressionTree> as) {
        TypeElement o = (TypeElement) m.getEnclosingElement();
        String callee = (cx.isSource(o) ? "prog:" : "lib:") + o.getQualifiedName() + "." + m.getSimpleName();
        if (!cx.isSource(o)) {
          boolean gen = libGen(m.getReturnType()) || m.getReturnType().getKind() == TypeKind.TYPEVAR;
          for (VariableElement pe : m.getParameters()) gen |= libGen(pe.asType()) || libFn(pe.asType()) || pe.asType().getKind() == TypeKind.TYPEVAR;
          gen |= libFn(m.getReturnType());
          libapi.add(String.join("\t", o.getQualifiedName(), m.getSimpleName() + cx.sigToken(m), m.getModifiers().contains(Modifier.STATIC) || m.getKind() == ElementKind.CONSTRUCTOR ? "static" : "instance",
              m.getTypeParameters().isEmpty() ? "-" : "gm", o.getTypeParameters().isEmpty() ? "-" : "gc", gen ? "generic-sig" : "-", family(o)));
        }
        List<? extends TypeMirror> ips = inst == null ? null : inst.getParameterTypes();
        List<? extends VariableElement> dps = m.getParameters();
        for (int i = 0; i < as.size(); i++) {
          int pi = Math.min(i, dps.size() - 1);
          if (pi < 0) break;
          TypeMirror dp = dps.get(pi).asType();
          TypeMirror ip = ips != null && pi < ips.size() ? ips.get(pi) : dp;
          if (m.isVarArgs() && pi == dps.size() - 1 && ip.getKind() == TypeKind.ARRAY) {
            TypeMirror at = typeOf(as.get(i));
            if (!(as.size() == dps.size() && at != null && (at.getKind() == TypeKind.ARRAY || at.getKind() == TypeKind.NULL))) {
              ip = ((ArrayType) ip).getComponentType();
              dp = dp.getKind() == TypeKind.ARRAY ? ((ArrayType) dp).getComponentType() : dp;
            }
          }
          flow("arg", as.get(i), ip, dp, callee + "#" + pi, cx.isSource(o) ? dps.get(pi) : null);
        }
      }

      @Override
      public Void visitMethodInvocation(MethodInvocationTree t, Void v) {
        Element e = elemOf(t.getMethodSelect());
        TypeMirror mt = typeOf(t.getMethodSelect());
        if (e instanceof ExecutableElement m0 && t.getMethodSelect() instanceof MemberSelectTree ms0 && !m0.getModifiers().contains(Modifier.STATIC)) {
          TypeElement o0 = (TypeElement) m0.getEnclosingElement();
          TypeMirror rt0 = typeOf(ms0.getExpression());
          String mn = m0.getSimpleName().toString();
          if (!cx.isSource(o0) && rt0 != null && libGen(uncapture(rt0)) && m0.getParameters().size() >= 1 && qn(m0.getParameters().get(0).asType()).equals("java.lang.Object")
              && m0.getParameters().get(0).asType().getKind() == TypeKind.DECLARED
              && Set.of("get", "containsKey", "remove", "contains", "indexOf", "lastIndexOf", "containsValue", "getOrDefault").contains(mn)) {
            TypeMirror ru = uncapture(rt0);
            boolean map = sub(types.erasure(ru), "java.util.Map");
            TypeElement view = cx.elems.getTypeElement(map ? "java.util.Map" : "java.util.Collection");
            DeclaredType dv = null;
            try {
              dv = cx.superView(ru, view);
            } catch (RuntimeException ex) {
              dv = null;
            }
            String rel = "no-view";
            if (dv != null && !dv.getTypeArguments().isEmpty()) {
              TypeMirror ta = dv.getTypeArguments().get(map && mn.equals("containsValue") ? 1 : 0);
              if (ta.getKind() == TypeKind.WILDCARD) ta = ((WildcardType) ta).getExtendsBound() != null ? ((WildcardType) ta).getExtendsBound() : cx.elems.getTypeElement("java.lang.Object").asType();
              ta = uncapture(ta);
              TypeMirror at = typeOf(t.getArguments().get(0));
              if (at == null) rel = "?";
              else if (at.getKind() == TypeKind.NULL) rel = "null";
              else {
                TypeMirror ab = at.getKind().isPrimitive() ? types.boxedClass((PrimitiveType) at).asType() : uncapture(at);
                try {
                  rel = same(ab, ta) ? "same" : types.isSubtype(ab, ta) ? "subtype" : types.isSubtype(ta, ab) ? "supertype" : "unrelated";
                } catch (RuntimeException ex) {
                  rel = "?";
                }
              }
            }
            objparams.add(String.join("\t", file, "" + line(t), w(), qn(ru), mn, rel, clean(rt0)));
          }
        }
        if (e instanceof ExecutableElement m) args(m, mt instanceof ExecutableType et ? et : null, t.getArguments());
        return super.visitMethodInvocation(t, v);
      }

      @Override
      public Void visitNewClass(NewClassTree t, Void v) {
        Element e = trees.getElement(getCurrentPath());
        TypeMirror ty = trees.getTypeMirror(getCurrentPath());
        if (e instanceof ExecutableElement m) {
          ExecutableType inst = null;
          try {
            if (ty instanceof DeclaredType dt && t.getClassBody() == null) inst = (ExecutableType) types.asMemberOf(dt, m);
          } catch (RuntimeException ex) {
            inst = null;
          }
          args(m, inst, t.getArguments());
        }
        if (ty != null && libGen(ty) && t.getClassBody() == null) {
          flows.add(String.join("\t", file, "" + line(t), w(), "new", typeCat(ty, 0), family(Ctx.typeElement(ty)), qn(ty), clean(ty), clean(ty), typeCat(ty, 0), "new", "-", "-", "-", "0", argCats(ty)));
        }
        return super.visitNewClass(t, v);
      }

      @Override
      public Void visitEnhancedForLoop(EnhancedForLoopTree t, Void v) {
        TypeMirror s = typeOf(t.getExpression());
        if (s != null && s.getKind() != TypeKind.ARRAY) {
          TypeMirror su = uncapture(s);
          TypeElement it = cx.elems.getTypeElement("java.lang.Iterable");
          DeclaredType view = null;
          try {
            view = cx.superView(su, it);
          } catch (RuntimeException ex) {
            view = null;
          }
          String cat = view == null ? "no-view" : view.getTypeArguments().isEmpty() ? "raw" : argCat(view.getTypeArguments().get(0), 0);
          TypeMirror vt = elemOf(t.getVariable()) instanceof VariableElement ve ? ve.asType() : null;
          String elemSame = view != null && !view.getTypeArguments().isEmpty() && vt != null ? (same(types.erasure(uncapture(boundOf(view.getTypeArguments().get(0)))), types.erasure(vt)) ? "same" : "differs") : "-";
          flows.add(String.join("\t", file, "" + line(t), w(), "foreach", cat, su.getKind() == TypeKind.DECLARED ? family(Ctx.typeElement(su)) : "?", qn(su), clean(s), clean(vt), elemSame, srcKind(t.getExpression()), "-", "-", "-", "0", "-"));
        }
        return super.visitEnhancedForLoop(t, v);
      }

      TypeMirror boundOf(TypeMirror a) {
        if (a.getKind() == TypeKind.WILDCARD) {
          WildcardType w = (WildcardType) a;
          return w.getExtendsBound() != null ? w.getExtendsBound() : cx.elems.getTypeElement("java.lang.Object").asType();
        }
        return a;
      }

      @Override
      public Void visitInstanceOf(InstanceOfTree t, Void v) {
        TypeMirror to = t.getType() != null ? typeOf(t.getType()) : null;
        if (to == null && t.getPattern() instanceof BindingPatternTree bp) to = typeOf(bp.getVariable().getType());
        if (to != null && (libGen(to) || libFn(to))) instofs.add(String.join("\t", file, "" + line(t), w(), family(Ctx.typeElement(to)), qn(to), clean(typeOf(t.getExpression())), clean(to)));
        return super.visitInstanceOf(t, v);
      }

      @Override
      public Void visitTypeCast(TypeCastTree t, Void v) {
        TypeMirror to = typeOf(t.getType()), from = typeOf(t.getExpression());
        if (to != null && from != null && (libGen(to) || to.getKind() == TypeKind.TYPEVAR || (to.getKind() == TypeKind.DECLARED && !((DeclaredType) to).getTypeArguments().isEmpty()))) {
          String k;
          if (to.getKind() == TypeKind.TYPEVAR) k = "to-typevar";
          else if (((DeclaredType) to).getTypeArguments().isEmpty()) k = "to-raw";
          else {
            boolean allWild = true;
            for (TypeMirror a : ((DeclaredType) to).getTypeArguments()) allWild &= a.getKind() == TypeKind.WILDCARD && ((WildcardType) a).getExtendsBound() == null && ((WildcardType) a).getSuperBound() == null;
            boolean safe;
            try {
              safe = allWild || types.isSubtype(from, to) || (types.isSubtype(to, uncapture(from)) && typeCat(uncapture(from), 0).matches("exact|box|object|tvar-kept|tvar-erased"));
            } catch (RuntimeException ex) {
              safe = false;
            }
            k = safe ? "checked" : "unchecked";
          }
          casts.add(String.join("\t", file, "" + line(t), w(), k, libGen(to) ? family(Ctx.typeElement(to)) : cx.isSource(Ctx.typeElement(to)) || to.getKind() == TypeKind.TYPEVAR ? "program" : "lib-nongeneric", qn(to), clean(from), clean(to)));
        }
        return super.visitTypeCast(t, v);
      }

      void use(Element e, String k) {
        uses.computeIfAbsent(e, x -> new TreeMap<>()).merge(k, 1, Integer::sum);
      }

      /** The use that the expression at the current path (a variable of a library generic type, or any functional value) is put to. */
      String useKind() {
        TreePath p = getCurrentPath();
        Tree self = p.getLeaf();
        TreePath pp = p.getParentPath();
        while (pp != null && pp.getLeaf() instanceof ParenthesizedTree) {
          self = pp.getLeaf();
          pp = pp.getParentPath();
        }
        if (pp == null) return "other";
        Tree parent = pp.getLeaf();
        if (parent instanceof MemberSelectTree ms && ms.getExpression() == self) {
          Tree gp = pp.getParentPath() == null ? null : pp.getParentPath().getLeaf();
          if (gp instanceof MethodInvocationTree mi && mi.getMethodSelect() == parent) return "call:" + ms.getIdentifier();
          return "member:" + ms.getIdentifier();
        }
        if (parent instanceof MemberReferenceTree mr) return "mref:" + mr.getName();
        if (parent instanceof EnhancedForLoopTree f && f.getExpression() == self) return "foreach";
        if (parent instanceof BinaryTree b && (b.getKind() == Tree.Kind.EQUAL_TO || b.getKind() == Tree.Kind.NOT_EQUAL_TO)) {
          Tree other = b.getLeftOperand() == self ? b.getRightOperand() : b.getLeftOperand();
          return other.getKind() == Tree.Kind.NULL_LITERAL ? "cmp-null" : "cmp-identity";
        }
        if (parent instanceof BinaryTree) return "concat";
        if (parent instanceof InstanceOfTree) return "instanceof";
        if (parent instanceof TypeCastTree) return "cast";
        if (parent instanceof AssignmentTree a) return a.getVariable() == self ? "assigned" : "flow";
        if (parent instanceof CompoundAssignmentTree) return "concat";
        if (parent instanceof VariableTree || parent instanceof ReturnTree || parent instanceof MethodInvocationTree || parent instanceof NewClassTree || parent instanceof ConditionalExpressionTree
            || parent instanceof LambdaExpressionTree || parent instanceof NewArrayTree) {
          if (parent instanceof ConditionalExpressionTree c && c.getCondition() == self) return "other";
          return "flow";
        }
        if (parent instanceof SynchronizedTree) return "synchronized";
        return "other:" + parent.getKind();
      }

      void valueUse(Tree t) {
        Element e = trees.getElement(getCurrentPath());
        TypeMirror ty = trees.getTypeMirror(getCurrentPath());
        String k = null;
        if (e instanceof VariableElement ve && (libGen(ve.asType()) || libFn(ve.asType()))) {
          k = useKind();
          use(ve, k);
        }
        TypeMirror tu = ty == null ? null : uncapture(ty);
        if (tu != null && libFn(tu)) {
          if (k == null) k = useKind();
          TypeElement fi = Ctx.typeElement(tu);
          ExecutableElement sam = cx.functionalMethod(fi);
          String kk = k;
          if (k.startsWith("call:") && sam != null && k.equals("call:" + sam.getSimpleName())) kk = "call-sam";
          fnuses.add(String.join("\t", file, "" + line(t), w(), qn(tu), kk, e instanceof VariableElement ve2 ? ve2.getKind().toString() : "expr"));
        }
      }

      @Override
      public Void visitIdentifier(IdentifierTree t, Void v) {
        valueUse(t);
        return super.visitIdentifier(t, v);
      }

      @Override
      public Void visitMemberSelect(MemberSelectTree t, Void v) {
        valueUse(t);
        return super.visitMemberSelect(t, v);
      }

    }.scan(cu, null);
  }

  void write(Path dir) throws IOException {
    Files.createDirectories(dir);
    for (var e : declRows.entrySet()) {
      TreeMap<String, Integer> u = uses.get(e.getKey());
      StringBuilder sb = new StringBuilder();
      if (u != null) u.forEach((k, n) -> sb.append(sb.length() > 0 ? ";" : "").append(k).append("=").append(n));
      decls.add(String.join("\t", e.getValue()) + "\t" + (sb.length() == 0 ? "-" : sb));
    }
    Files.write(dir.resolve("decls.tsv"), decls);
    Files.write(dir.resolve("flows.tsv"), flows);
    Files.write(dir.resolve("impls.tsv"), impls);
    Files.write(dir.resolve("fnuses.tsv"), fnuses);
    Files.write(dir.resolve("casts.tsv"), casts);
    Files.write(dir.resolve("overrides.tsv"), overrides);
    Files.write(dir.resolve("libapi.tsv"), libapi);
    Files.write(dir.resolve("objparams.tsv"), objparams);
    Files.write(dir.resolve("instofs.tsv"), instofs);
    System.err.println("libgen: decls=" + decls.size() + " flows=" + flows.size() + " impls=" + impls.size() + " fnuses=" + fnuses.size() + " casts=" + casts.size());
  }
}
