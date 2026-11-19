package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Report only (J2GO_VALUES=1): the classes of the program that could be Go values (a struct held
 * and passed by value) instead of pointers to heap objects, and what in the program speaks against
 * it. A candidate is a class or record without superclass or subclasses, not nested in an
 * instance, whose instance fields are all final and none of its own type. For each one the uses
 * of its identity and of null are counted. No rule uses this; it is the list a rule would start
 * from.
 *
 * <p>Columns (tab separated): Go name, Java name, kind, fields, bytes of the Go struct, equals
 * (own or Object's), then the counts: `==` between two values of the class, comparisons with
 * null, null given where the class is expected, places where a value of the class becomes an
 * Object or an interface (a box in Go), mentions as a type argument, instanceof and casts to the
 * class, and the notes (synchronized, identity collections, this escaping its constructor,
 * interfaces it implements, fields or arrays of the class that start as null).
 */
final class ValueScan {
  static final class Info {
    int eqeq, nullCmp, nullGiven, boxed, typeArg, tests;
    final TreeSet<String> notes = new TreeSet<>();
  }

  static final Map<TypeElement, Info> infos = new LinkedHashMap<>();

  static boolean candidate(Ctx cx, TypeElement te) {
    if (te.getKind() != ElementKind.CLASS && te.getKind() != ElementKind.RECORD) return false;
    if (te.getModifiers().contains(Modifier.ABSTRACT) || cx.isHandle(te) || !cx.isLeaf(te) || cx.superclass(te) != null) return false;
    // A local, anonymous or inner class holds its scope.
    if (te.getNestingKind() == NestingKind.ANONYMOUS || te.getNestingKind() == NestingKind.LOCAL) return false;
    if (te.getNestingKind() == NestingKind.MEMBER && !te.getModifiers().contains(Modifier.STATIC)) return false;
    TypeElement sup = Ctx.typeElement(te.getSuperclass());
    if (sup != null && !sup.getQualifiedName().contentEquals("java.lang.Object") && !sup.getQualifiedName().contentEquals("java.lang.Record")) return false;
    List<VariableElement> fs = Ctx.instanceFields(te);
    if (fs.isEmpty()) return false;
    for (VariableElement f : fs) {
      if (!f.getModifiers().contains(Modifier.FINAL)) return false;
      if (te.equals(Ctx.typeElement(cx.erase(f.asType())))) return false; // a value cannot hold itself
    }
    return true;
  }

  static void prepare(Ctx cx) {
    for (TypeElement te : cx.source) if (candidate(cx, te)) infos.put(te, new Info());
  }

  static Info of(Ctx cx, TypeMirror t) {
    if (t == null || t.getKind() != TypeKind.DECLARED) return null;
    return infos.get(Ctx.typeElement(cx.erase(t)));
  }

  static void scan(Ctx cx, Trees trees, CompilationUnitTree cu) {
    new TreePathScanner<Void, Void>() {
      TypeMirror typeOf(Tree t) {
        return t == null ? null : trees.getTypeMirror(new TreePath(getCurrentPath(), t));
      }

      /** A value of static type `from` given where `to` is expected. */
      void flow(ExpressionTree e, TypeMirror to) {
        if (e == null || to == null) return;
        while (e instanceof ParenthesizedTree p) e = p.getExpression();
        if (e.getKind() == Tree.Kind.NULL_LITERAL) {
          Info i = of(cx, to);
          if (i != null) i.nullGiven++;
          return;
        }
        if (e instanceof ConditionalExpressionTree c) {
          flow(c.getTrueExpression(), to);
          flow(c.getFalseExpression(), to);
          return;
        }
        Info i = of(cx, typeOf(e));
        if (i != null && of(cx, to) != i) i.boxed++;
      }

      void args(ExecutableElement m, List<? extends ExpressionTree> as) {
        if (m == null) return;
        List<? extends VariableElement> ps = m.getParameters();
        for (int k = 0; k < as.size(); k++) {
          TypeMirror pt = ps.isEmpty() ? null : ps.get(Math.min(k, ps.size() - 1)).asType();
          if (m.isVarArgs() && k >= ps.size() - 1 && pt instanceof ArrayType at) {
            TypeMirror at0 = typeOf(as.get(k));
            if (!(at0 instanceof ArrayType && as.size() == ps.size())) pt = at.getComponentType();
          }
          flow(as.get(k), pt);
        }
      }

      @Override
      public Void visitBinary(BinaryTree t, Void v) {
        if (t.getKind() == Tree.Kind.EQUAL_TO || t.getKind() == Tree.Kind.NOT_EQUAL_TO) {
          ExpressionTree l = t.getLeftOperand(), r = t.getRightOperand();
          while (l instanceof ParenthesizedTree p) l = p.getExpression();
          while (r instanceof ParenthesizedTree p) r = p.getExpression();
          Info a = of(cx, typeOf(l)), b = of(cx, typeOf(r));
          boolean ln = l.getKind() == Tree.Kind.NULL_LITERAL, rn = r.getKind() == Tree.Kind.NULL_LITERAL;
          if (ln || rn) {
            if (a != null) a.nullCmp++;
            if (b != null) b.nullCmp++;
          } else {
            if (a != null) a.eqeq++;
            if (b != null && b != a) b.eqeq++;
          }
        } else if (t.getKind() == Tree.Kind.PLUS && cx.kind(typeOf(t)) == Ctx.K.STR) {
          // String concatenation calls toString: no box needed by a rule, not counted.
        }
        return super.visitBinary(t, v);
      }

      @Override
      public Void visitVariable(VariableTree t, Void v) {
        Element e = trees.getElement(getCurrentPath());
        if (e != null) {
          flow(t.getInitializer(), e.asType());
          Info i = of(cx, e.asType());
          if (i != null && e.getKind() == ElementKind.FIELD && t.getInitializer() == null && !e.getModifiers().contains(Modifier.FINAL)) {
            i.notes.add("field starts null");
          }
          if (i != null && e.getKind() == ElementKind.LOCAL_VARIABLE && t.getInitializer() == null
              && !(getCurrentPath().getParentPath().getLeaf() instanceof EnhancedForLoopTree)) {
            i.notes.add("local declared without value");
          }
        }
        return super.visitVariable(t, v);
      }

      @Override
      public Void visitAssignment(AssignmentTree t, Void v) {
        flow(t.getExpression(), typeOf(t.getVariable()));
        return super.visitAssignment(t, v);
      }

      @Override
      public Void visitReturn(ReturnTree t, Void v) {
        for (TreePath p = getCurrentPath(); p != null; p = p.getParentPath()) {
          if (p.getLeaf() instanceof LambdaExpressionTree) {
            Info i = of(cx, typeOf(t.getExpression()));
            if (i != null) i.notes.add("returned by a lambda");
            break;
          }
          if (p.getLeaf() instanceof MethodTree) {
            if (trees.getElement(p) instanceof ExecutableElement m) flow(t.getExpression(), m.getReturnType());
            break;
          }
        }
        return super.visitReturn(t, v);
      }

      @Override
      public Void visitMethodInvocation(MethodInvocationTree t, Void v) {
        Element m = trees.getElement(getCurrentPath());
        if (m instanceof ExecutableElement me) {
          args(me, t.getArguments());
          String owner = me.getEnclosingElement().toString(), name = me.getSimpleName().toString();
          if (owner.equals("java.lang.System") && name.equals("identityHashCode")) {
            Info i = of(cx, typeOf(t.getArguments().get(0)));
            if (i != null) i.notes.add("identityHashCode");
          }
          if (owner.equals("java.lang.Object") && Set.of("wait", "notify", "notifyAll").contains(name) && t.getMethodSelect() instanceof MemberSelectTree ms) {
            Info i = of(cx, typeOf(ms.getExpression()));
            if (i != null) i.notes.add("wait/notify");
          }
          // A method of Object or of an interface called on the value: no box, the receiver is known.
        }
        return super.visitMethodInvocation(t, v);
      }

      @Override
      public Void visitNewClass(NewClassTree t, Void v) {
        if (trees.getElement(getCurrentPath()) instanceof ExecutableElement me) args(me, t.getArguments());
        return super.visitNewClass(t, v);
      }

      @Override
      public Void visitNewArray(NewArrayTree t, Void v) {
        TypeMirror at = typeOf(t);
        if (at instanceof ArrayType a) {
          Info i = of(cx, a.getComponentType());
          if (i != null && t.getInitializers() == null) i.notes.add("array made with null elements");
          if (t.getInitializers() != null) for (ExpressionTree e : t.getInitializers()) flow(e, a.getComponentType());
        }
        return super.visitNewArray(t, v);
      }

      @Override
      public Void visitSynchronized(SynchronizedTree t, Void v) {
        Info i = of(cx, typeOf(t.getExpression()));
        if (i != null) i.notes.add("synchronized");
        return super.visitSynchronized(t, v);
      }

      @Override
      public Void visitInstanceOf(InstanceOfTree t, Void v) {
        Info i = t.getType() == null ? null : of(cx, typeOf(t.getType()));
        if (i == null && t.getPattern() instanceof BindingPatternTree bp) i = of(cx, typeOf(bp.getVariable().getType()));
        if (i != null) i.tests++;
        return super.visitInstanceOf(t, v);
      }

      @Override
      public Void visitTypeCast(TypeCastTree t, Void v) {
        Info i = of(cx, typeOf(t.getType()));
        if (i != null && of(cx, typeOf(t.getExpression())) != i) i.tests++;
        flow(t.getExpression(), typeOf(t.getType()));
        return super.visitTypeCast(t, v);
      }

      @Override
      public Void visitParameterizedType(ParameterizedTypeTree t, Void v) {
        String base = String.valueOf(trees.getElement(new TreePath(getCurrentPath(), t.getType())));
        for (Tree a : t.getTypeArguments()) {
          Tree b = a instanceof WildcardTree w ? w.getBound() : a;
          Info i = b == null ? null : of(cx, typeOf(b));
          if (i == null) continue;
          i.typeArg++;
          if (base.contains("IdentityHashMap") || base.contains("WeakReference") || base.contains("WeakHashMap") || base.contains("ThreadLocal")) {
            i.notes.add("in " + base.substring(base.lastIndexOf('.') + 1));
          }
        }
        return super.visitParameterizedType(t, v);
      }

      @Override
      public Void visitIdentifier(IdentifierTree t, Void v) {
        // `this` of a candidate used as a value inside its constructor: the object is known before it is complete.
        if (t.getName().contentEquals("this")) {
          Tree parent = getCurrentPath().getParentPath().getLeaf();
          boolean member = parent instanceof MemberSelectTree ms && ms.getExpression() == t;
          if (!member) {
            for (TreePath p = getCurrentPath(); p != null; p = p.getParentPath()) {
              if (p.getLeaf() instanceof MethodTree mt) {
                if (mt.getName().contentEquals("<init>")) {
                  Info i = of(cx, typeOf(t));
                  if (i != null) i.notes.add("this escapes its constructor");
                }
                break;
              }
            }
          }
        }
        return super.visitIdentifier(t, v);
      }
    }.scan(cu, null);
  }

  static int bytes(Ctx cx, TypeElement te) {
    int n = 0, max = 1;
    for (VariableElement f : Ctx.instanceFields(te)) {
      TypeMirror t = cx.erase(f.asType());
      int s, a = 8;
      switch (cx.kind(t)) {
        case PRIM:
          switch (t.getKind()) {
            case BOOLEAN, BYTE: s = 1; break;
            case SHORT, CHAR: s = 2; break;
            case INT, FLOAT: s = 4; break;
            default: s = 8;
          }
          a = s;
          break;
        case STR, IFACE, ANY: s = 16; break;
        case ARR: s = 24; break;
        case CLS:
          s = cx.isHandleType(t) ? 4 : cx.isValueLibType(t) ? 24 : 8;
          a = cx.isHandleType(t) ? 4 : 8;
          break;
        default: s = 8;
      }
      n += s;
      max = Math.max(max, a);
    }
    return (n + max - 1) / max * max;
  }

  static void report(Ctx cx) {
    System.err.println("values: " + infos.size() + " candidate classes (no superclass or subclass, static, all fields final)");
    for (Map.Entry<TypeElement, Info> e : infos.entrySet()) {
      TypeElement te = e.getKey();
      Info i = e.getValue();
      boolean eq = false, ts = false;
      for (Element m : te.getEnclosedElements()) {
        if (m instanceof ExecutableElement x && x.getSimpleName().contentEquals("equals") && x.getParameters().size() == 1) eq = true;
      }
      if (te.getKind() == ElementKind.RECORD) eq = true;
      List<String> ifs = new ArrayList<>();
      for (TypeMirror t : te.getInterfaces()) ifs.add(String.valueOf(Ctx.typeElement(cx.erase(t)).getSimpleName()));
      if (!ifs.isEmpty()) i.notes.add("implements " + String.join(",", ifs));
      System.err.println("values:\t" + cx.className(te) + "\t" + te.getQualifiedName() + "\t" + (te.getKind() == ElementKind.RECORD ? "record" : "class") + "\t"
          + Ctx.instanceFields(te).size() + "\t" + bytes(cx, te) + "\t" + (eq ? "own equals" : "Object equals") + "\t" + i.eqeq + "\t" + i.nullCmp + "\t" + i.nullGiven
          + "\t" + i.boxed + "\t" + i.typeArg + "\t" + i.tests + "\t" + String.join("; ", i.notes));
    }
  }
}
