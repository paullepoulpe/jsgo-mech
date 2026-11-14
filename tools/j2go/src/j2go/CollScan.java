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
    Set<String> bareUses = new TreeSet<>();
    VariableElement self;
  }

  static final Map<VariableElement, Info> vars = new LinkedHashMap<>();
  /** Parameter of a source method -> the collection variables passed to it. */
  static final Map<VariableElement, List<Element>> params = new LinkedHashMap<>();
  /** How parameters of collection type are used inside their method. */
  static final Map<VariableElement, Info> paramUse = new LinkedHashMap<>();

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

  /** Like kindOf, and also the supertypes a parameter receiving such a collection may declare. */
  static String kindOfWide(Ctx cx, TypeMirror t) {
    String k = kindOf(cx, t);
    if (k != null) return k;
    TypeElement te = t == null || t.getKind() != TypeKind.DECLARED ? null : Ctx.typeElement(cx.erase(t));
    if (te == null) return null;
    switch (te.getQualifiedName().toString()) {
      case "java.util.Collection", "java.lang.Iterable": return "iterable";
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
        boolean param = e.getKind() == ElementKind.PARAMETER;
        if (e.getKind() != ElementKind.FIELD && e.getKind() != ElementKind.LOCAL_VARIABLE && !param) return null;
        String k = param ? kindOfWide(cx, e.asType()) : kindOf(cx, e.asType());
        if (k == null) return null;
        return (param ? paramUse : vars).computeIfAbsent(v, x -> {
          Info i = new Info();
          i.kind = k;
          i.self = v;
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
        i.bareUses.add(bareKind(t, parent));
      }

      /** What a use of a collection variable other than as receiver or for-each source is. */
      String bareKind(Tree t, Tree parent) {
        while (parent instanceof ParenthesizedTree) parent = null;
        if (parent instanceof BinaryTree b && (b.getKind() == Tree.Kind.EQUAL_TO || b.getKind() == Tree.Kind.NOT_EQUAL_TO)) return "compared";
        if (parent instanceof ReturnTree) return "returned";
        if (parent instanceof VariableTree || parent instanceof AssignmentTree) return "assigned to another variable";
        if (parent instanceof LambdaExpressionTree) return "lambda result";
        if (parent instanceof MethodInvocationTree || parent instanceof NewClassTree) {
          Element c = trees.getElement(getCurrentPath().getParentPath());
          if (!(c instanceof ExecutableElement ex)) return "argument (unresolved)";
          TypeElement owner = (TypeElement) ex.getEnclosingElement();
          if (!cx.isSource(owner)) return "argument to library: " + owner.getSimpleName() + "." + ex.getSimpleName();
          List<? extends ExpressionTree> args = parent instanceof MethodInvocationTree mi ? mi.getArguments() : ((NewClassTree) parent).getArguments();
          int idx = args.indexOf(t);
          if (idx < 0 || idx >= ex.getParameters().size()) return "argument to source method (varargs or receiver)";
          boolean open = !(ex.getModifiers().contains(Modifier.PRIVATE) || ex.getModifiers().contains(Modifier.STATIC) || ex.getModifiers().contains(Modifier.FINAL)
              || owner.getModifiers().contains(Modifier.FINAL) || ex.getKind() == ElementKind.CONSTRUCTOR);
          params.computeIfAbsent(ex.getParameters().get(idx), x -> new ArrayList<>()).add(trees.getElement(getCurrentPath()));
          return open ? "argument to overridable source method" : "argument to source method";
        }
        return "other: " + parent.getKind();
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
    // One hop: variables whose only bare uses are arguments to non-overridable source methods whose
    // parameter is itself never used bare (only as receiver or for-each source) and never reassigned.
    Map<String, Integer> uses = new TreeMap<>();
    int[] hop = new int[4];
    int libOnly = 0, libOrHop = 0, retOnly = 0;
    for (Info i : vars.values()) {
      if (!i.onlyNew || i.inits == 0 || !i.bare) continue;
      hop[0]++;
      boolean benign = true, oneHop = true;
      for (String u : i.bareUses) {
        uses.merge(u.startsWith("argument to library") ? u : u, 1, Integer::sum);
        if (u.equals("compared")) continue;
        benign = false;
        if (!u.equals("argument to source method")) oneHop = false;
      }
      boolean lib = true, libHop = true, ret = false;
      for (String u : i.bareUses) {
        boolean l = u.equals("compared") || u.startsWith("argument to library");
        if (!l) lib = false;
        if (!l && !u.equals("argument to source method")) libHop = false;
        if (u.equals("returned")) ret = true;
      }
      if (lib && !benign) libOnly++;
      if (libHop && !lib) {
        boolean ok2 = true;
        for (var pe : params.entrySet()) {
          if (!pe.getValue().contains(i.self)) continue;
          Info pi = paramUse.get(pe.getKey());
          if (pi != null && (pi.bare || pi.inits > 0)) ok2 = false;
        }
        if (ok2) libOrHop++;
      }
      if (ret) retOnly++;
      if (benign) hop[1]++;
      else if (oneHop) {
        boolean ok = true;
        for (var pe : params.entrySet()) {
          if (!pe.getValue().contains(i.self)) continue;
          Info pi = paramUse.get(pe.getKey());
          if (pi == null) continue;  // parameter never used
          if (pi.bare || pi.inits > 0) ok = false;
        }
        if (ok) hop[2]++; else hop[3]++;
      }
    }
    System.err.println("collections: typeable and used bare: " + hop[0] + "; only compared (to null): " + hop[1]
        + "; only passed to closed source methods that use the parameter as receiver/for-each: " + hop[2] + "; passed to source methods that pass it on: " + hop[3]);
    System.err.println("collections: only library arguments: " + libOnly + "; library arguments plus one closed hop: " + libOrHop + "; returned from a method (among other uses): " + retOnly);
    List<Map.Entry<String, Integer>> ul = new ArrayList<>(uses.entrySet());
    ul.sort((a, b) -> b.getValue() - a.getValue());
    for (int k = 0; k < Math.min(40, ul.size()); k++) System.err.println("collections: bare use: " + ul.get(k).getValue() + " variables: " + ul.get(k).getKey());
    if (System.getenv("J2GO_COLLECTIONS").equals("2")) {
      for (Info i : vars.values()) {
        System.err.println("collection\t" + i.kind + "\t" + (i.field ? "field" : "local") + "\t" + i.where + "\t" + i.elem + "\tonlyNew=" + (i.onlyNew && i.inits > 0)
            + "\tbare=" + i.bare + "\titerated=" + i.iterated + "\t" + i.methods);
      }
    }
  }
}
