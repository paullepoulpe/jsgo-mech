package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Rule "typed collections": decides, declaration by declaration, which fields and local
 * variables of type List<X> / ArrayList<X> hold a typed list (*TList[X], rt_tlist.go) instead of
 * the erased ArrayList.
 *
 * A declaration is typed when
 * - X is a type that has one Go type and no type arguments (a String, a class, an interface, a
 *   handle);
 * - every value that enters it is `new ArrayList<>()`, `new ArrayList<>(n)`,
 *   `new ArrayList<>(collection)`, null, or the value of another typed declaration (so the
 *   declaration never holds a list that erased code made); and
 * - every use of it is one the typed list or its erased view serves as the ArrayList did: a
 *   call of a List method (not subList, listIterator, getClass: the view is not an ArrayList
 *   there), a for-each loop, ==, a string concatenation, or a place of a type the view has (a
 *   variable, parameter, result or cast of type List, Collection, Iterable, Object or a type
 *   variable) -- not a place that wants the class ArrayList itself, unless that place is a typed
 *   declaration too.
 *
 * Java's behaviour is kept: the list is one object wherever it goes (TList.asList() is its one
 * view), null is nil, and the view implements List as ArrayList in rt_coll.go does.
 * J2GO_COLLS=1 reports every declaration and why it stays erased; J2GO_ERASED=1 skips the rule.
 */
final class Colls {
  final Ctx cx;
  final Trees trees;

  static final class Slot {
    Element v;  // a field, a local variable, a parameter; or a method, for its result
    TypeMirror elem;
    String why;
    boolean made;  // a list enters it (a new ArrayList, or the value of another candidate)
    final List<Tree> uses = new ArrayList<>();
    final List<Slot> taints = new ArrayList<>();  // erased if this one is
  }

  final Map<Element, Slot> slots = new LinkedHashMap<>();
  /** Methods whose parameters and result can be candidates: nothing overrides them, they override nothing. */
  final Set<ExecutableElement> closed = new HashSet<>();

  Colls(Ctx cx, Trees trees) {
    this.cx = cx;
    this.trees = trees;
  }

  static final Set<String> VIEW_TYPES = Set.of("java.util.List", "java.util.Collection", "java.lang.Iterable", "java.util.SequencedCollection", "java.lang.Object");
  static final Set<String> NOT_ON_VIEW = Set.of("subList", "listIterator", "getClass", "wait", "notify", "notifyAll", "reversed", "spliterator");

  void poison(Slot s, String why) {
    if (s.why == null) s.why = why;
  }

  static ExpressionTree strip(ExpressionTree t) {
    while (t instanceof ParenthesizedTree p) t = p.getExpression();
    return t;
  }

  Slot candidate(Element e, TypeMirror type) {
    TypeMirror elem = cx.tlistElem(type);
    if (elem == null) return null;
    Slot s = new Slot();
    s.v = e;
    s.elem = elem;
    slots.put(e, s);
    return s;
  }

  /**
   * Whether the parameters and the result of method m can be candidates: a constructor, a static
   * or a private method of the program with a body and no type parameters (its signature is its
   * own: it is in no group of overriding methods, and only calls that name it reach it).
   */
  boolean closedMethod(ExecutableElement m, MethodTree t) {
    if (t.getBody() == null || !m.getTypeParameters().isEmpty()) return false;
    TypeElement owner = (TypeElement) m.getEnclosingElement();
    if (owner.getKind() == ElementKind.RECORD || owner.getKind() == ElementKind.ANNOTATION_TYPE) return false;
    if (m.getKind() == ElementKind.CONSTRUCTOR) return owner.getNestingKind() != NestingKind.ANONYMOUS;
    if (m.getKind() != ElementKind.METHOD) return false;
    if (m.getSimpleName().contentEquals("main")) return false;
    if (m.getModifiers().contains(Modifier.STATIC) || m.getModifiers().contains(Modifier.PRIVATE)) return true;
    // an instance method of a class that overrides nothing and that nothing overrides
    if ("1".equals(System.getenv("J2GO_COLLS_NOINSTANCE")) || owner.getKind().isInterface()) return false;
    if (overridden == null) {
      overridden = new HashSet<>();
      for (TypeElement te : cx.source) {
        for (Element e : te.getEnclosedElements()) {
          if (e instanceof ExecutableElement x && cx.isInstanceMethod(x) && cx.root(x) != x) {
            // every method x overrides
            for (TypeElement st : cx.allSupertypes(te)) {
              for (Element c : st.getEnclosedElements()) {
                if (c instanceof ExecutableElement y && y != x && y.getSimpleName().equals(x.getSimpleName()) && cx.elems.overrides(x, y, te)) overridden.add(y);
              }
            }
          }
        }
      }
    }
    return cx.root(m) == m && !overridden.contains(m);
  }

  Set<ExecutableElement> overridden;
  int openResults, openParams;

  /** Pass 1: the declarations that could be typed. */
  void candidates(CompilationUnitTree cu) {
    new TreePathScanner<Void, Void>() {
      @Override
      public Void visitMethod(MethodTree t, Void v) {
        Element e = trees.getElement(getCurrentPath());
        if (e instanceof ExecutableElement m && closedMethod(m, t) && !"1".equals(System.getenv("J2GO_COLLS_NOMETHODS"))) {
          closed.add(m);
          candidate(m, m.getReturnType());
        } else if (e instanceof ExecutableElement m) {
          // not candidates: the signature belongs to a group of overriding methods (or to a lambda's interface)
          if (cx.tlistElem(m.getReturnType()) != null) openResults++;
          for (VariableElement p : m.getParameters()) if (cx.tlistElem(p.asType()) != null) openParams++;
        }
        return super.visitMethod(t, v);
      }

      @Override
      public Void visitVariable(VariableTree t, Void v) {
        Element e = trees.getElement(getCurrentPath());
        Tree parent = getCurrentPath().getParentPath().getLeaf();
        boolean param = e != null && e.getKind() == ElementKind.PARAMETER && parent instanceof MethodTree mt && mt.getParameters().contains(t)
            && closed.contains(e.getEnclosingElement())
            && !(((ExecutableElement) e.getEnclosingElement()).isVarArgs() && mt.getParameters().indexOf(t) == mt.getParameters().size() - 1);
        if (e instanceof VariableElement ve && (e.getKind() == ElementKind.FIELD || e.getKind() == ElementKind.LOCAL_VARIABLE || param)) {
          Slot s = candidate(ve, ve.asType());
          if (s != null) {
            if (parent instanceof EnhancedForLoopTree) poison(s, "the variable of a for-each loop");
            if (e.getKind() == ElementKind.FIELD) {
              TypeElement owner = (TypeElement) e.getEnclosingElement();
              if (owner.getKind() == ElementKind.RECORD) poison(s, "a record component");
              else if (cx.isHandle(owner)) poison(s, "a field of a handle class");
              else if (!e.getAnnotationMirrors().isEmpty()) poison(s, "an annotated field (read by a library)");
              else if ("locals".equals(System.getenv("J2GO_COLLS_ONLY"))) poison(s, "a field");
            }
          }
        }
        return super.visitVariable(t, v);
      }
    }.scan(cu, null);
  }

  /** Pass 2: what enters each candidate and how it is used. */
  void flows(CompilationUnitTree cu) {
    new TreePathScanner<Void, Void>() {
      TypeMirror typeOf(Tree t) {
        try {
          return trees.getTypeMirror(new TreePath(getCurrentPath(), t));
        } catch (RuntimeException ex) {
          return null;
        }
      }

      Element elemOf(TreePath p) {
        try {
          return trees.getElement(p);
        } catch (RuntimeException ex) {
          return null;
        }
      }

      Slot slotOf(ExpressionTree t, TreePath parent) {
        t = strip(t);
        if (t instanceof MethodInvocationTree mi) return slots.get(elemOf(new TreePath(new TreePath(parent, t), mi.getMethodSelect())));
        if (!(t instanceof IdentifierTree) && !(t instanceof MemberSelectTree)) return null;
        Element e = elemOf(new TreePath(parent, t));
        return e instanceof VariableElement ? slots.get(e) : null;
      }

      /** The method (not a lambda) whose body the current tree is in, or null. */
      ExecutableElement enclosingMethod(TreePath p) {
        for (TreePath q = p; q != null; q = q.getParentPath()) {
          if (q.getLeaf() instanceof LambdaExpressionTree || q.getLeaf() instanceof ClassTree) return null;
          if (q.getLeaf() instanceof MethodTree) return elemOf(q) instanceof ExecutableElement ex ? ex : null;
        }
        return null;
      }

      /** A value enters s. */
      void inflow(Slot s, ExpressionTree x, TreePath parent) {
        x = strip(x);
        switch (x.getKind()) {
          case NULL_LITERAL:
            return;
          case CONDITIONAL_EXPRESSION: {
            ConditionalExpressionTree c = (ConditionalExpressionTree) x;
            TreePath cp = new TreePath(parent, x);
            inflow(s, c.getTrueExpression(), cp);
            inflow(s, c.getFalseExpression(), cp);
            return;
          }
          case NEW_CLASS: {
            NewClassTree n = (NewClassTree) x;
            TypeMirror made = typeOf(x);
            TypeElement me = made == null ? null : Ctx.typeElement(cx.types.erasure(made));
            if (n.getClassBody() == null && me != null && me.getQualifiedName().contentEquals("java.util.ArrayList") && n.getArguments().size() <= 1) {
              TypeMirror elem = cx.tlistElem(made);
              if (elem != null && cx.types.isSameType(elem, s.elem)) {
                s.made = true;
                return;
              }
            }
            poison(s, "assigned a new " + (me == null ? "?" : me.getSimpleName()));
            return;
          }
          case IDENTIFIER, MEMBER_SELECT: {
            Slot from = slotOf(x, parent);
            if (from != null && cx.types.isSameType(from.elem, s.elem)) {
              from.taints.add(s);
              s.made = true;
              return;
            }
            poison(s, "assigned a variable that is not typed");
            return;
          }
          case METHOD_INVOCATION: {
            Slot from = slotOf(x, parent);
            if (from != null && cx.types.isSameType(from.elem, s.elem)) {
              from.taints.add(s);
              s.made = true;
              return;
            }
            Element m = elemOf(new TreePath(new TreePath(parent, x), ((MethodInvocationTree) x).getMethodSelect()));
            poison(s, "assigned the result of " + (m == null ? "a call" : m.getEnclosingElement().getSimpleName() + "." + m.getSimpleName()));
            return;
          }
          default:
            poison(s, "assigned a " + x.getKind());
        }
      }

      /** The value of s reaches a place of type target (target slot ts, if the place is a candidate). */
      void escape(Slot s, TypeMirror target, Slot ts) {
        TypeElement te = target == null ? null : Ctx.typeElement(cx.types.erasure(target));
        if (te != null && VIEW_TYPES.contains(te.getQualifiedName().toString())) return;
        if (ts != null && cx.types.isSameType(ts.elem, s.elem)) {
          ts.taints.add(s);  // an ArrayList variable: it takes the typed list or s stays erased
          return;
        }
        poison(s, "reaches a place of type " + (te == null ? String.valueOf(target) : te.getSimpleName().toString()));
      }

      /** The candidate parameter that argument child of call parent goes to, or null. */
      Slot argSlot(Tree parent, Tree child, TreePath pp) {
        if (!(parent instanceof MethodInvocationTree) && !(parent instanceof NewClassTree)) return null;
        List<? extends ExpressionTree> args = parent instanceof MethodInvocationTree mi ? mi.getArguments() : ((NewClassTree) parent).getArguments();
        int idx = args.indexOf(child);
        Element c = parent instanceof MethodInvocationTree mi ? elemOf(new TreePath(pp, mi.getMethodSelect())) : elemOf(pp);
        if (idx < 0 || !(c instanceof ExecutableElement ex) || idx >= ex.getParameters().size()) return null;
        return slots.get(ex.getParameters().get(idx));
      }

      /** The arguments of a call enter the candidate parameters. */
      void call(Element c, List<? extends ExpressionTree> args) {
        if (!(c instanceof ExecutableElement ex)) return;
        for (int i = 0; i < ex.getParameters().size(); i++) {
          Slot ps = slots.get(ex.getParameters().get(i));
          if (ps == null) continue;
          if (i >= args.size()) poison(ps, "a call without its argument");
          else inflow(ps, args.get(i), getCurrentPath());
        }
      }

      void use(Slot s) {
        TreePath path = getCurrentPath();
        Tree child = path.getLeaf();
        s.uses.add(child);
        TreePath pp = path.getParentPath();
        boolean viaCond = false;
        while (pp.getLeaf() instanceof ParenthesizedTree || pp.getLeaf() instanceof ConditionalExpressionTree c && c.getCondition() != child) {
          if (pp.getLeaf() instanceof ConditionalExpressionTree) viaCond = true;
          child = pp.getLeaf();
          pp = pp.getParentPath();
        }
        Tree parent = pp.getLeaf();
        if (viaCond) {
          // a branch of a conditional expression: the conditional's own type decides, unless it
          // is what enters a candidate
          boolean intoSlot = parent instanceof VariableTree vt && vt.getInitializer() == child && slots.containsKey(elemOf(pp))
              || parent instanceof AssignmentTree a && a.getExpression() == child && slotOf(a.getVariable(), pp) != null
              || parent instanceof ReturnTree && enclosingMethod(pp) != null && slots.containsKey(enclosingMethod(pp))
              || argSlot(parent, child, pp) != null;
          if (!intoSlot) {
            escape(s, typeOf(child), null);
            return;
          }
        }
        if (parent instanceof MemberSelectTree ms && ms.getExpression() == child) {
          Tree gp = pp.getParentPath().getLeaf();
          Element m = gp instanceof MethodInvocationTree mi && mi.getMethodSelect() == parent ? elemOf(pp) : null;
          if (!(m instanceof ExecutableElement me)) {
            poison(s, "used as " + parent);
            return;
          }
          if (cx.tlistMethod(me) != null) return;
          TypeElement ro = cx.ownerOf(cx.root(me));
          if (!VIEW_TYPES.contains(ro.getQualifiedName().toString()) || NOT_ON_VIEW.contains(me.getSimpleName().toString())) {
            poison(s, "method " + ro.getSimpleName() + "." + me.getSimpleName());
          }
          return;
        }
        if (parent instanceof EnhancedForLoopTree f && f.getExpression() == child) return;
        if (parent instanceof VariableTree vt && vt.getInitializer() == child) {
          Element ve = elemOf(pp);
          escape(s, ve == null ? null : ve.asType(), slots.get(ve));
          return;
        }
        if (parent instanceof AssignmentTree a) {
          if (a.getVariable() == child) return;  // assigned: visitAssignment
          escape(s, typeOf(a.getVariable()), slotOf(a.getVariable(), pp));
          return;
        }
        if (parent instanceof ReturnTree) {
          ExecutableElement ex = enclosingMethod(pp);
          if (ex != null) escape(s, ex.getReturnType(), slots.get(ex));
          else poison(s, "returned from a lambda");
          return;
        }
        if (parent instanceof MethodInvocationTree || parent instanceof NewClassTree) {
          List<? extends ExpressionTree> args = parent instanceof MethodInvocationTree mi ? mi.getArguments() : ((NewClassTree) parent).getArguments();
          int idx = args.indexOf(child);
          Element c = parent instanceof MethodInvocationTree mi ? elemOf(new TreePath(pp, mi.getMethodSelect())) : elemOf(pp);
          if (idx < 0 || !(c instanceof ExecutableElement ex)) {
            poison(s, "used in " + parent.getKind());
            return;
          }
          int np = ex.getParameters().size();
          if (ex.isVarArgs() && idx >= np - 1 || idx >= np) {
            poison(s, "a variable argument of " + ex.getSimpleName());
            return;
          }
          escape(s, ex.getParameters().get(idx).asType(), slots.get(ex.getParameters().get(idx)));
          return;
        }
        if (parent instanceof BinaryTree b) {
          if (b.getKind() == Tree.Kind.EQUAL_TO || b.getKind() == Tree.Kind.NOT_EQUAL_TO || b.getKind() == Tree.Kind.PLUS) return;
        }
        if (parent instanceof TypeCastTree tc) {
          escape(s, typeOf(tc), null);
          return;
        }
        if (parent instanceof ExpressionStatementTree) return;  // a call whose result is not used
        poison(s, "used in " + parent.getKind());
      }

      @Override
      public Void visitVariable(VariableTree t, Void v) {
        Slot s = slots.get(elemOf(getCurrentPath()));
        if (s != null && t.getInitializer() != null) inflow(s, t.getInitializer(), getCurrentPath());
        return super.visitVariable(t, v);
      }

      @Override
      public Void visitAssignment(AssignmentTree t, Void v) {
        Slot s = slotOf(t.getVariable(), getCurrentPath());
        if (s != null) inflow(s, t.getExpression(), getCurrentPath());
        return super.visitAssignment(t, v);
      }

      @Override
      public Void visitIdentifier(IdentifierTree t, Void v) {
        Element e = elemOf(getCurrentPath());
        Slot s = e instanceof VariableElement ? slots.get(e) : null;
        if (s != null) use(s);
        return super.visitIdentifier(t, v);
      }

      @Override
      public Void visitMemberSelect(MemberSelectTree t, Void v) {
        Element e = elemOf(getCurrentPath());
        Slot s = e != null && e.getKind() == ElementKind.FIELD ? slots.get(e) : null;
        if (s != null) use(s);
        return super.visitMemberSelect(t, v);
      }

      @Override
      public Void visitMethodInvocation(MethodInvocationTree t, Void v) {
        Element c = elemOf(new TreePath(getCurrentPath(), t.getMethodSelect()));
        call(c, t.getArguments());
        Slot rs = c instanceof ExecutableElement ? slots.get(c) : null;
        if (rs != null) use(rs);
        return super.visitMethodInvocation(t, v);
      }

      @Override
      public Void visitNewClass(NewClassTree t, Void v) {
        Element c = elemOf(getCurrentPath());
        if (t.getClassBody() != null) {
          // an anonymous class passes its arguments on to the constructor of its superclass
          TypeMirror tm = typeOf(t);
          TypeElement te = tm == null ? null : Ctx.typeElement(cx.types.erasure(tm));
          TypeElement sup = te == null ? null : cx.superclass(te);
          if (sup != null && !t.getArguments().isEmpty()) {
            for (Element m : sup.getEnclosedElements()) {
              if (m.getKind() != ElementKind.CONSTRUCTOR) continue;
              for (VariableElement p : ((ExecutableElement) m).getParameters()) {
                if (slots.get(p) != null) poison(slots.get(p), "a constructor that an anonymous class calls");
              }
            }
          }
        } else {
          call(c, t.getArguments());
        }
        return super.visitNewClass(t, v);
      }

      @Override
      public Void visitReturn(ReturnTree t, Void v) {
        ExecutableElement ex = t.getExpression() == null ? null : enclosingMethod(getCurrentPath());
        Slot rs = ex == null ? null : slots.get(ex);
        if (rs != null) inflow(rs, t.getExpression(), getCurrentPath());
        return super.visitReturn(t, v);
      }

      @Override
      public Void visitMemberReference(MemberReferenceTree t, Void v) {
        // a method named by a method reference is called at the erased signature
        if (elemOf(getCurrentPath()) instanceof ExecutableElement ex) {
          if (slots.get(ex) != null) poison(slots.get(ex), "named by a method reference");
          for (VariableElement p : ex.getParameters()) if (slots.get(p) != null) poison(slots.get(p), "of a method named by a method reference");
        }
        return super.visitMemberReference(t, v);
      }
    }.scan(cu, null);
  }

  /** Decides, and gives the typed declarations and the expressions that read them their type. */
  void decide() {
    ArrayDeque<Slot> work = new ArrayDeque<>();
    for (Slot s : slots.values()) {
      if (s.why == null && !s.made) s.why = "no list enters it";
    }
    for (Slot s : slots.values()) if (s.why != null) work.add(s);
    while (!work.isEmpty()) {
      Slot s = work.poll();
      for (Slot t : s.taints) {
        if (t.why == null) {
          t.why = "exchanges values with " + s.v.getEnclosingElement().getSimpleName() + "." + s.v.getSimpleName() + ", which is erased";
          work.add(t);
        }
      }
    }
    int typed = 0, fields = 0, params = 0, results = 0;
    Map<String, Integer> whys = new TreeMap<>(), elems = new TreeMap<>();
    boolean report = System.getenv("J2GO_COLLS") != null;
    for (Slot s : slots.values()) {
      if (s.why != null) {
        whys.merge(s.why.replaceAll("exchanges values with .*", "exchanges values with an erased declaration"), 1, Integer::sum);
        if ("2".equals(System.getenv("J2GO_COLLS"))) System.err.println("colls: erased " + s.v.getEnclosingElement() + "." + s.v + ": " + s.why);
        continue;
      }
      typed++;
      if (s.v.getKind() == ElementKind.FIELD) fields++;
      if (s.v.getKind() == ElementKind.PARAMETER) params++;
      elems.merge(s.elem.toString(), 1, Integer::sum);
      TypeMirror t = cx.tlistOf(s.elem);
      if (s.v instanceof ExecutableElement m) {
        results++;
        cx.typedResults.put(m, t);
      } else {
        cx.refinedVars.put(s.v, t);
      }
      for (Tree u : s.uses) cx.refinedTrees.put(u, t);
    }
    System.err.println("typed collections: " + typed + " of " + slots.size() + " List/ArrayList declarations are typed lists (" + fields + " fields, "
        + (typed - fields - params - results) + " locals, " + params + " parameters, " + results + " results)");
    if (report) {
      System.err.println("colls: not candidates: " + openParams + " parameters and " + openResults + " results of methods in groups of overriding methods");
      List<Map.Entry<String, Integer>> l = new ArrayList<>(whys.entrySet());
      l.sort((a, b) -> b.getValue() - a.getValue());
      for (var e : l) System.err.println("colls: erased: " + e.getValue() + " " + e.getKey());
      l = new ArrayList<>(elems.entrySet());
      l.sort((a, b) -> b.getValue() - a.getValue());
      for (var e : l) System.err.println("colls: typed element: " + e.getValue() + " " + e.getKey());
    }
  }
}
