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

/**
 * Report only (J2GO_COLLMAP=dir; no rule reads it): where collections are erased and why, from
 * javac's attribution and the decisions of rule "typed collections" (Colls).
 *
 * - decls.tsv: every declaration (field, local, parameter, result of a method of the program)
 *   whose type is a collection type (a java.util.Collection, a Map, a Map.Entry, a class of
 *   Guava's collect package): typed or erased, and every reason found for an erased one.
 * - slots.tsv: Colls' declarations with all their reasons and the edges "erased if that one is".
 * - origins.tsv: every expression that makes a collection value which is not the value of a
 *   declaration (a `new`, the result of a library method, a cast), with the place it enters.
 *
 * docs/libgen/collmap.py makes the tables of docs/libgen/COLLECTIONS-PLAN.md from them.
 */
final class CollMap {
  final Ctx cx;
  final Trees trees;
  final Colls colls;
  final List<String> decls = new ArrayList<>(), origins = new ArrayList<>(), slotRows = new ArrayList<>();
  final IdentityHashMap<Colls.Slot, Integer> slotIds = new IdentityHashMap<>();

  CollMap(Ctx cx, Trees trees, Colls colls) {
    this.cx = cx;
    this.trees = trees;
    this.colls = colls;
    for (Colls.Slot s : colls.slots.values()) slotIds.computeIfAbsent(s, k -> slotIds.size() + 1);
    for (Colls.Slot s : colls.chains.values()) if (s != null) slotIds.computeIfAbsent(s, k -> slotIds.size() + 1);
  }

  boolean sub(TypeMirror er, String qn) {
    TypeElement s = cx.elems.getTypeElement(qn);
    return s != null && cx.types.isSubtype(er, cx.types.erasure(s.asType()));
  }

  /** Whether t is a collection type (not an Iterable or Iterator alone: rule "typed library interfaces" has them). */
  boolean coll(TypeMirror t) {
    if (t == null || t.getKind() != TypeKind.DECLARED) return false;
    TypeElement te = Ctx.typeElement(t);
    if (te == null || cx.isSource(te)) return false;
    String qn = te.getQualifiedName().toString();
    if (qn.startsWith("j2go.rt.")) return true;
    TypeMirror er = cx.types.erasure(t);
    if (sub(er, "java.util.Collection") || sub(er, "java.util.Map") || sub(er, "java.util.Map.Entry")) return true;
    return qn.startsWith("com.google.common.collect.") && !te.getTypeParameters().isEmpty() && cx.functionalMethod(te) == null
        && !sub(er, "java.util.Iterator") && !sub(er, "java.util.Comparator");
  }

  static String qn(TypeMirror t) {
    TypeElement te = t == null || t.getKind() != TypeKind.DECLARED ? null : Ctx.typeElement(t);
    return te == null ? String.valueOf(t) : te.getQualifiedName().toString();
  }

  static String clean(Object o) {
    return String.valueOf(o).replaceAll("[\t\n]", " ").replaceAll("@[A-Za-z.]+ ", "");
  }

  /** What one type argument is for a typed collection: ok, box, object, wild, extends, super, tvar-kept, tvar-erased, libgeneric (a collection of collections), other. */
  String argCat(TypeMirror a) {
    switch (a.getKind()) {
      case WILDCARD: {
        WildcardType w = (WildcardType) a;
        if (w.getExtendsBound() != null) return "extends";
        return w.getSuperBound() != null ? "super" : "wild";
      }
      case TYPEVAR:
        if (a.toString().startsWith("capture")) return "wild";
        return cx.typedVar(a) ? "tvar-kept" : "tvar-erased";
      case ARRAY:
        return "array";
      case DECLARED: {
        TypeElement te = Ctx.typeElement(a);
        if (te.getQualifiedName().contentEquals("java.lang.Object")) return "object";
        if (cx.unboxed(a) != null) return "box";
        if (!cx.isSource(te) && !te.getTypeParameters().isEmpty()) return coll(a) ? "nested-coll" : cx.isValueLibType(a) ? "optional" : "libgeneric";
        return "ok";
      }
      default:
        return "other";
    }
  }

  String argCats(TypeMirror t) {
    if (!(t instanceof DeclaredType d)) return "-";
    if (d.getTypeArguments().isEmpty()) return Ctx.typeElement(t).getTypeParameters().isEmpty() ? "nongeneric" : "raw";
    StringBuilder sb = new StringBuilder();
    for (TypeMirror a : d.getTypeArguments()) sb.append(sb.length() > 0 ? "," : "").append(argCat(a));
    return sb.toString();
  }

  String where(Element e) {
    String s = "";
    for (Element x = e; x != null && x.getKind() != ElementKind.PACKAGE; x = x.getEnclosingElement()) {
      s = x.getSimpleName() + (s.isEmpty() ? "" : "." + s);
    }
    return s;
  }

  /** typed / erased, by what the translator emits for declaration e. */
  String state(Element e) {
    if (e instanceof ExecutableElement m) return cx.typedResults.containsKey(m) ? "typed" : "erased";
    return cx.refinedVars.containsKey(e) && cx.isTypedColl(cx.refinedVars.get(e)) ? "typed" : "erased";
  }

  void decl(Element e, TypeMirror t, String kind, String noSlotWhy) {
    if (!coll(t)) return;
    Colls.Slot s = colls.slots.get(e);
    Ctx.CollKind ck = cx.collKindOf(t);
    String why = s != null ? String.join(" | ", s.whys)
        : ck == null ? "no kind for the type"
        : cx.typedArgs(ck, t) == null ? "type arguments: " + colls.whyNot(ck, t)
        : noSlotWhy;
    // (last column: the form of an erased declaration under rule "typed library interfaces": L2 a typed interface, L3 erased)
    String st = state(e);
    String form = st.equals("typed") ? "L1" : cx.lg.is(cx.erase(t)) ? "L2" : "L3";
    decls.add(String.join("\t", kind, where(e), qn(t), clean(t), argCats(t), st, s == null ? "-" : "" + slotIds.get(s), ck == null ? "-" : ck.name, clean(why), form));
  }

  int line(CompilationUnitTree cu, Tree t) {
    return (int) cu.getLineMap().getLineNumber(trees.getSourcePositions().getStartPosition(cu, t));
  }

  void scan(CompilationUnitTree cu) {
    String file = Path.of(cu.getSourceFile().getName()).getFileName().toString();
    new TreePathScanner<Void, Void>() {
      Element elemOf(TreePath p) {
        try {
          return trees.getElement(p);
        } catch (RuntimeException ex) {
          return null;
        }
      }

      TypeMirror typeOf(TreePath p) {
        try {
          return trees.getTypeMirror(p);
        } catch (RuntimeException ex) {
          return null;
        }
      }

      @Override
      public Void visitMethod(MethodTree t, Void v) {
        if (elemOf(getCurrentPath()) instanceof ExecutableElement m && m.getKind() == ElementKind.METHOD) {
          List<ExecutableElement> g = Ctx.isInstanceMethod(m) && !m.getModifiers().contains(Modifier.PRIVATE) ? colls.groupOf(m) : null;
          decl(m, m.getReturnType(), "result", "not a candidate: " + colls.whyNoGroup(m, g));
        }
        return super.visitMethod(t, v);
      }

      @Override
      public Void visitVariable(VariableTree t, Void v) {
        Element e = elemOf(getCurrentPath());
        if (e instanceof VariableElement ve) {
          Tree parent = getCurrentPath().getParentPath().getLeaf();
          String kind = e.getKind() == ElementKind.FIELD ? "field" : parent instanceof EnhancedForLoopTree ? "foreach"
              : e.getKind() == ElementKind.PARAMETER ? (parent instanceof LambdaExpressionTree ? "lambdaparam" : "param") : "local";
          String why = "not a candidate";
          if (e.getKind() == ElementKind.PARAMETER && e.getEnclosingElement() instanceof ExecutableElement m) {
            List<ExecutableElement> g = m.getKind() == ElementKind.METHOD && Ctx.isInstanceMethod(m) && !m.getModifiers().contains(Modifier.PRIVATE) ? colls.groupOf(m) : null;
            why = parent instanceof LambdaExpressionTree ? "not a candidate: a parameter of a lambda" : "not a candidate: " + colls.whyNoGroup(m, g);
          }
          decl(ve, ve.asType(), kind, why);
        }
        return super.visitVariable(t, v);
      }

      /** The place that the value of the expression at path p enters. */
      String[] consumer(TreePath p) {
        Tree child = p.getLeaf();
        TreePath pp = p.getParentPath();
        while (pp.getLeaf() instanceof ParenthesizedTree || pp.getLeaf() instanceof ConditionalExpressionTree c && c.getCondition() != child) {
          child = pp.getLeaf();
          pp = pp.getParentPath();
        }
        Tree parent = pp.getLeaf();
        Element target = null;
        String what;
        if (parent instanceof VariableTree vt && vt.getInitializer() == child) {
          target = elemOf(pp);
          what = "init";
        } else if (parent instanceof AssignmentTree a && a.getExpression() == child) {
          target = elemOf(new TreePath(pp, a.getVariable()));
          what = "assign";
        } else if (parent instanceof ReturnTree) {
          what = "return";
          for (TreePath q = pp; q != null; q = q.getParentPath()) {
            if (q.getLeaf() instanceof LambdaExpressionTree) {
              what = "lambda-return";
              break;
            }
            if (q.getLeaf() instanceof MethodTree) {
              target = elemOf(q);
              break;
            }
            if (q.getLeaf() instanceof ClassTree) break;
          }
        } else if (parent instanceof LambdaExpressionTree) {
          what = "lambda-return";
        } else if (parent instanceof MemberSelectTree ms && ms.getExpression() == child) {
          Element m = elemOf(pp);
          return new String[] {"receiver", m == null ? "?" : cx.ownerOf(m).getSimpleName() + "." + m.getSimpleName(), "-", "-"};
        } else if (parent instanceof EnhancedForLoopTree f && f.getExpression() == child) {
          return new String[] {"foreach", "-", "-", "-"};
        } else if (parent instanceof MethodInvocationTree || parent instanceof NewClassTree) {
          List<? extends ExpressionTree> args = parent instanceof MethodInvocationTree mi ? mi.getArguments() : ((NewClassTree) parent).getArguments();
          int idx = args.indexOf(child);
          Element c = parent instanceof MethodInvocationTree mi ? elemOf(new TreePath(pp, mi.getMethodSelect())) : elemOf(pp);
          if (idx < 0 || !(c instanceof ExecutableElement ex)) return new String[] {"other", parent.getKind().toString(), "-", "-"};
          TypeElement owner = cx.ownerOf(ex);
          int np = ex.getParameters().size();
          VariableElement pe = np == 0 ? null : ex.getParameters().get(Math.min(idx, np - 1));
          String callee = owner.getQualifiedName() + "." + ex.getSimpleName();
          if (!cx.isSource(owner)) return new String[] {"libarg", callee, pe == null ? "-" : clean(pe.asType()), "-"};
          target = pe;
          what = "arg";
        } else if (parent instanceof ExpressionStatementTree) {
          return new String[] {"unused", "-", "-", "-"};
        } else {
          return new String[] {"other", parent.getKind().toString(), "-", "-"};
        }
        if (target == null) return new String[] {what, "-", "-", "-"};
        TypeMirror tt = target instanceof ExecutableElement m ? m.getReturnType() : target.asType();
        Colls.Slot ts = colls.slots.get(target);
        return new String[] {what, where(target), clean(tt), state(target) + (ts == null ? "" : ":" + slotIds.get(ts))};
      }

      void origin(Tree t, String kind, String detail, String recv) {
        TypeMirror ty = typeOf(getCurrentPath());
        if (!coll(ty)) return;
        String[] c = consumer(getCurrentPath());
        Colls.Slot ch = colls.chains.get(t);
        origins.add(String.join("\t", file + ":" + line(cu, t), kind, detail, recv, qn(ty), clean(ty), argCats(ty),
            cx.refinedTrees.containsKey(t) ? "refined" : ch != null ? "chain:" + slotIds.get(ch) : "-", c[0], c[1], c[2], c[3]));
      }

      @Override
      public Void visitNewClass(NewClassTree t, Void v) {
        TypeMirror ty = typeOf(getCurrentPath());
        if (coll(ty)) origin(t, t.getClassBody() != null ? "new-anon" : "new", qn(ty) + "/" + t.getArguments().size(), "-");
        return super.visitNewClass(t, v);
      }

      @Override
      public Void visitTypeCast(TypeCastTree t, Void v) {
        origin(t, "cast", "-", "-");
        return super.visitTypeCast(t, v);
      }

      @Override
      public Void visitMethodInvocation(MethodInvocationTree t, Void v) {
        Element c = elemOf(new TreePath(getCurrentPath(), t.getMethodSelect()));
        if (c instanceof ExecutableElement m) {
          TypeElement owner = cx.ownerOf(m);
          boolean lib = !cx.isSource(owner);
          String detail = owner.getQualifiedName() + "." + m.getSimpleName() + "/" + m.getParameters().size();
          if (m.getModifiers().contains(Modifier.STATIC)) {
            if (lib) origin(t, "lib-static", detail, "-");
          } else {
            String recv = "-";
            if (t.getMethodSelect() instanceof MemberSelectTree ms) {
              TreePath rp = new TreePath(new TreePath(getCurrentPath(), ms), ms.getExpression());
              TypeMirror rt = cx.refinedTrees.get(ms.getExpression());
              TypeMirror jt = typeOf(rp);
              recv = rt != null && cx.isTypedColl(rt) ? "typed:" + cx.typedColl(rt).name : coll(jt) ? "erased:" + qn(jt) + "<" + argCats(jt) + ">" : "obj:" + qn(jt);
              if (!(rt != null && cx.isTypedColl(rt)) && strip(ms.getExpression()) instanceof MethodInvocationTree) recv += " (a call)";
            }
            if (lib) origin(t, "lib-instance", detail, recv);
            else if (!cx.typedResults.containsKey(m) && coll(typeOf(getCurrentPath())) && !coll(m.getReturnType())) {
              // a method of the program whose declared result is a type variable (Outcome<T>.get): no declaration has the collection type
              origin(t, "prog-tvar-result", detail, recv);
            }
          }
        }
        return super.visitMethodInvocation(t, v);
      }

      ExpressionTree strip(ExpressionTree x) {
        return Colls.strip(x);
      }
    }.scan(cu, null);
  }

  void write(Path dir) throws IOException {
    Files.createDirectories(dir);
    for (var e : slotIds.entrySet()) {
      Colls.Slot s = e.getKey();
      StringBuilder taints = new StringBuilder();
      for (Colls.Slot t : s.taints) taints.append(taints.length() > 0 ? "," : "").append(slotIds.get(t));
      boolean typed = s.why == null;
      slotRows.add(String.join("\t", "" + e.getValue(), s.kind.name, s.v == null ? "(builder chain)" : where(s.v), "" + s.args, typed ? "typed" : "erased",
          clean(String.join(" | ", s.whys)), taints.length() == 0 ? "-" : taints.toString(), s.v == null ? "chain" : s.all != null ? "group" + s.all.size() : "decl", s.made ? "made" : "unmade",
          s.ended ? "ended" : "-"));
    }
    Files.write(dir.resolve("decls.tsv"), decls);
    Files.write(dir.resolve("slots.tsv"), slotRows);
    Files.write(dir.resolve("origins.tsv"), origins);
  }
}
