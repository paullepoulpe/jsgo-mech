package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Report only (J2GO_COLLECTIONS=1): how every field and local variable of a java.util collection
 * type is created and used, i.e. which of them could be a typed or native Go collection.
 */
final class CollScan {
  static final class Info {
    String kind, where;
    boolean field, onlyNew = true, bare, iterated;
    int inits;
    String elem = "?", elemKind = "?";
    Set<String> methods = new TreeSet<>();
  }

  static final Map<VariableElement, Info> vars = new LinkedHashMap<>();

  static String kindOf(Ctx cx, TypeMirror t) {
    TypeElement te = t == null || t.getKind() != TypeKind.DECLARED ? null : Ctx.typeElement(cx.erase(t));
    if (te == null) return null;
    switch (te.getQualifiedName().toString()) {
      case "java.util.List", "java.util.ArrayList": return "list";
      case "java.util.Map", "java.util.LinkedHashMap", "java.util.HashMap": return "map";
      case "java.util.Set", "java.util.LinkedHashSet", "java.util.HashSet": return "set";
      case "java.util.Deque", "java.util.ArrayDeque": return "deque";
      default: return null;
    }
  }

  static String elemKind(Ctx cx, TypeMirror a) {
    if (a.getKind() == TypeKind.TYPEVAR || a.getKind() == TypeKind.WILDCARD) return "type variable";
    TypeMirror e = cx.erase(a);
    if (cx.kind(e) == Ctx.K.STR) return "String";
    if (cx.isHandleType(e)) return "handle";
    if (cx.unboxed(e) != null) return "boxed primitive";
    TypeElement te = Ctx.typeElement(e);
    if (te == null) return "other";
    if (te.getKind() == ElementKind.ENUM) return "enum";
    if (!cx.isSource(te)) return "library type";
    for (TypeElement c = te; c != null && cx.isSource(c); c = cx.superclass(c)) {
      for (Element m : c.getEnclosedElements()) {
        if (m.getKind() == ElementKind.METHOD && m.getSimpleName().contentEquals("equals")) return "class with equals";
      }
    }
    return te.getKind().isInterface() ? "interface" : "class, identity";
  }

  static void scan(Ctx cx, Trees trees, CompilationUnitTree cu) {
    new TreePathScanner<Void, Void>() {
      Info info(Element e) {
        if (!(e instanceof VariableElement v)) return null;
        if (e.getKind() != ElementKind.FIELD && e.getKind() != ElementKind.LOCAL_VARIABLE) return null;
        String k = kindOf(cx, e.asType());
        if (k == null) return null;
        return vars.computeIfAbsent(v, x -> {
          Info i = new Info();
          i.kind = k;
          i.field = e.getKind() == ElementKind.FIELD;
          i.where = e.getEnclosingElement() + "." + e.getSimpleName();
          List<? extends TypeMirror> ta = ((DeclaredType) e.asType()).getTypeArguments();
          if (!ta.isEmpty()) {
            i.elem = ta.get(0).toString();
            i.elemKind = elemKind(cx, ta.get(0));
          }
          return i;
        });
      }

      void value(Info i, ExpressionTree init) {
        if (init == null) return;
        while (init instanceof ParenthesizedTree p) init = p.getExpression();
        i.inits++;
        if (init.getKind() == Tree.Kind.NULL_LITERAL) return;
        if (init instanceof NewClassTree n && n.getClassBody() == null) {
          Element c = trees.getElement(new TreePath(getCurrentPath(), n));
          String cn = c == null ? "" : c.getEnclosingElement().toString();
          if (cn.equals("java.util.ArrayList") || cn.equals("java.util.LinkedHashMap") || cn.equals("java.util.HashMap")
              || cn.equals("java.util.LinkedHashSet") || cn.equals("java.util.HashSet") || cn.equals("java.util.ArrayDeque")) return;
        }
        i.onlyNew = false;
      }

      @Override
      public Void visitVariable(VariableTree t, Void v) {
        Info i = info(trees.getElement(getCurrentPath()));
        if (i != null) value(i, t.getInitializer());
        return super.visitVariable(t, v);
      }

      void use(Tree t) {
        Info i = info(trees.getElement(getCurrentPath()));
        if (i == null) return;
        Tree parent = getCurrentPath().getParentPath().getLeaf();
        if (parent instanceof AssignmentTree a && a.getVariable() == t) {
          value(i, a.getExpression());
          return;
        }
        if (parent instanceof MemberSelectTree ms && ms.getExpression() == t) {
          Tree gp = getCurrentPath().getParentPath().getParentPath().getLeaf();
          if (gp instanceof MethodInvocationTree mi && mi.getMethodSelect() == parent) {
            String m = ms.getIdentifier().toString();
            i.methods.add(m);
            if (Set.of("iterator", "forEach", "stream", "entrySet", "keySet", "values", "listIterator").contains(m)) i.iterated = true;
            return;
          }
        }
        if (parent instanceof EnhancedForLoopTree f && f.getExpression() == t) {
          i.iterated = true;
          return;
        }
        if (parent instanceof MemberSelectTree ms2 && ms2 == t) return;
        i.bare = true;
      }

      @Override
      public Void visitIdentifier(IdentifierTree t, Void v) {
        use(t);
        return super.visitIdentifier(t, v);
      }

      @Override
      public Void visitMemberSelect(MemberSelectTree t, Void v) {
        Element e = trees.getElement(getCurrentPath());
        if (e != null && e.getKind() == ElementKind.FIELD) use(t);
        return super.visitMemberSelect(t, v);
      }
    }.scan(cu, null);
  }

  static void report() {
    Map<String, int[]> c = new TreeMap<>();
    Map<String, Integer> elems = new TreeMap<>();
    for (Info i : vars.values()) {
      String k = i.kind + (i.field ? " field" : " local");
      int[] n = c.computeIfAbsent(k, x -> new int[6]);
      n[0]++;
      if (!i.onlyNew || i.inits == 0) continue;
      n[1]++;
      if (!i.bare) n[2]++;
      if (!i.iterated) n[3]++;
      if (!i.bare && !i.iterated) n[4]++;
      if (i.elemKind.equals("type variable")) n[5]++;
      elems.merge(i.kind + " of " + i.elemKind, 1, Integer::sum);
    }
    System.err.println("collections: kind | all | only assigned `new` | ..and never used bare | ..and never iterated | ..neither | ..element is a type variable");
    for (var e : c.entrySet()) System.err.println("collections: " + e.getKey() + " | " + Arrays.toString(e.getValue()));
    for (var e : elems.entrySet()) System.err.println("collections: first type argument, only-new: " + e.getKey() + " = " + e.getValue());
    if (System.getenv("J2GO_COLLECTIONS").equals("2")) {
      for (Info i : vars.values()) {
        System.err.println("collection\t" + i.kind + "\t" + (i.field ? "field" : "local") + "\t" + i.where + "\t" + i.elem + "\tonlyNew=" + (i.onlyNew && i.inits > 0)
            + "\tbare=" + i.bare + "\titerated=" + i.iterated + "\t" + i.methods);
      }
    }
  }
}
