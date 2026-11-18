package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Rule "typed collections": decides, declaration by declaration, which fields, local variables,
 * parameters and method results of a collection type hold the runtime's typed collection
 * instead of the erased one: a *TList[X] (rt_tlist.go) for List<X> / ArrayList<X>, a *TSet[X]
 * (rt_tset.go) for Set<X> / HashSet<X> / LinkedHashSet<X>, a *TDeque[X] (rt_tdeque.go) for
 * Deque<X> / ArrayDeque<X> / Queue<X>, a *TMap[K, V] (rt_tmap.go) for Map<K, V> / HashMap<K, V>
 * / LinkedHashMap<K, V> (Ctx.collKinds). The variable of a for-each loop over the entries of a
 * typed map (`for (Map.Entry<K, V> e : m.entrySet())`) is a *TEntry[K, V].
 *
 * A declaration is typed when
 * - each type argument is a type a typed collection holds (Ctx.typedArg): a String, a class, an
 *   interface or a handle without type arguments; a boxed primitive or Object (the box, `any`);
 *   a library type with type arguments (at its erasure). The elements of a set and the keys of
 *   a map are compared, so their type is one whose Go == is Java's equals: a String, an enum, a
 *   class of the program without equals of its own (Ctx.goEquals), a boxed primitive that is
 *   not a floating-point number;
 * - every value that enters it is a new collection of its kind (`new ArrayList<>()`,
 *   `new ArrayList<>(n)`, `new ArrayList<>(collection)`; `new LinkedHashSet<>(..)`), null, or
 *   the value of another typed declaration (so the declaration never holds a collection that
 *   erased code made); an argument enters the parameter, a returned expression the result; and
 * - every use of it is one the typed collection or its erased view serves as the Java class
 *   did: a call of a List / Set method (not subList, listIterator, getClass: the view is not
 *   the Java class there), a for-each loop, ==, a string concatenation, or a place of a type
 *   the view has (a variable, parameter, result or cast of type List / Set, Collection,
 *   Iterable, Object or a type variable) -- not a place that wants the class (ArrayList,
 *   LinkedHashSet) itself, unless that place is a typed declaration too.
 * Parameters and results are those of methods whose signature is their own (closedMethod).
 *
 * Java's behaviour is kept: the collection is one object wherever it goes (asList() / asSet()
 * is its one view), null is nil, and the view implements List / Set as the erased class in
 * rt_coll.go does. J2GO_COLLS=1 reports the counts and why declarations stay erased,
 * J2GO_COLLS=2 every declaration; J2GO_ERASED=1 skips the rule.
 */
final class Colls {
  final Ctx cx;
  final Trees trees;

  static final class Slot {
    Ctx.CollKind kind;
    Element v;  // a field, a local variable, a parameter; or a method, for its result
    List<TypeMirror> args;  // the element type; the key and value types of a map
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

  static final Set<String> NOT_ON_VIEW = Set.of("subList", "listIterator", "getClass", "wait", "notify", "notifyAll", "reversed", "spliterator");

  boolean same(Slot a, Slot b) {
    return a.kind == b.kind && sameArgs(a.args, b.args);
  }

  boolean sameArgs(List<TypeMirror> a, List<TypeMirror> b) {
    if (a == null || b == null || a.size() != b.size()) return false;
    for (int i = 0; i < a.size(); i++) if (!cx.types.isSameType(a.get(i), b.get(i))) return false;
    return true;
  }

  void poison(Slot s, String why) {
    if (s.why == null) s.why = why;
  }

  static ExpressionTree strip(ExpressionTree t) {
    while (t instanceof ParenthesizedTree p) t = p.getExpression();
    return t;
  }

  Slot candidate(Element e, TypeMirror type) {
    return candidate(e, type, cx.collKindOf(type));
  }

  Slot candidate(Element e, TypeMirror type, Ctx.CollKind kind) {
    List<TypeMirror> args = cx.typedArgs(kind, type);
    if (args == null) {
      if (kind != null && kind != cx.entryKind) untypable.merge(kind.name + ": " + whyNot(kind, type), 1, Integer::sum);
      return null;
    }
    Slot s = new Slot();
    s.kind = kind;
    s.v = e;
    s.args = args;
    slots.put(e, s);
    return s;
  }

  /** Declarations of a collection type whose type arguments no typed collection has, by reason (the report). */
  final Map<String, Integer> untypable = new TreeMap<>();

  /** Why no typed collection of kind k has the type arguments of type t (for the report). */
  String whyNot(Ctx.CollKind k, TypeMirror t) {
    List<? extends TypeMirror> as = ((DeclaredType) t).getTypeArguments();
    if (as.isEmpty()) return "a raw type";
    boolean map = k.name.equals("map");
    for (int i = 0; i < as.size(); i++) {
      TypeMirror a = as.get(i);
      String what = map ? (i == 0 ? "key" : "value") : "element";
      boolean compared = k.name.equals("set") || map && i == 0;
      if (cx.typedArg(a, compared) != null) continue;
      if (a.getKind() == TypeKind.WILDCARD) return "a wildcard " + what;
      if (a.getKind() == TypeKind.TYPEVAR) return "a type variable as " + what;
      if (a.getKind() == TypeKind.ARRAY) return "an array as " + what;
      if (a.getKind() != TypeKind.DECLARED) return what + " " + a.getKind();
      TypeElement ae = Ctx.typeElement(a);
      Ctx.K kd = cx.kind(a);
      if (kd != Ctx.K.STR && kd != Ctx.K.CLS && kd != Ctx.K.IFACE) return what + " " + ae.getSimpleName();
      if (cx.isValueLib(ae)) return what + " " + ae.getSimpleName() + " (a value)";
      if (cx.isSource(ae) && (ae.getNestingKind() == NestingKind.LOCAL || ae.getNestingKind() == NestingKind.ANONYMOUS)) return "a local class as " + what;
      if (!((DeclaredType) a).getTypeArguments().isEmpty() || !ae.getTypeParameters().isEmpty()) {
        return what + (compared ? " (compared)" : "") + " a type with type arguments" + (cx.isSource(ae) ? " that is a class of the program" : "");
      }
      return what + (cx.isSource(ae) ? (ae.getKind().isInterface() ? " an interface" : " a class with its own equals") : " a library class (" + ae.getSimpleName() + ")");
    }
    return "?";
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
          if (cx.typedArgs(m.getReturnType()) != null) openResults++;
          for (VariableElement p : m.getParameters()) if (cx.typedArgs(p.asType()) != null) openParams++;
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
        if (e instanceof VariableElement ve && e.getKind() == ElementKind.LOCAL_VARIABLE && parent instanceof EnhancedForLoopTree && cx.entryKind != null
            && ve.asType().getKind() == TypeKind.DECLARED && Ctx.typeElement(ve.asType()).getQualifiedName().contentEquals("java.util.Map.Entry")) {
          // the entry of a map: typed if the loop is over the entries of a typed map (flows)
          candidate(ve, ve.asType(), cx.entryKind);
        } else if (e instanceof VariableElement ve && (e.getKind() == ElementKind.FIELD || e.getKind() == ElementKind.LOCAL_VARIABLE || param)) {
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
            if (n.getClassBody() == null && me != null && s.kind.made.containsKey(me.getQualifiedName().toString()) && n.getArguments().size() <= 1) {
              if (sameArgs(cx.typedArgs(s.kind, made), s.args)) {
                s.made = true;
                return;
              }
            }
            poison(s, "assigned a new " + (me == null ? "?" : me.getSimpleName()));
            return;
          }
          case IDENTIFIER, MEMBER_SELECT: {
            Slot from = slotOf(x, parent);
            if (from != null && same(from, s)) {
              from.taints.add(s);
              s.made = true;
              return;
            }
            poison(s, "assigned a variable that is not typed");
            return;
          }
          case METHOD_INVOCATION: {
            Slot from = slotOf(x, parent);
            if (from != null && same(from, s)) {
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
        if (te != null && s.kind.viewTypes.contains(te.getQualifiedName().toString())) return;
        if (ts != null && same(ts, s)) {
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
          TypeElement ro = cx.ownerOf(cx.root(me));
          if (!s.kind.viewTypes.contains(ro.getQualifiedName().toString()) || NOT_ON_VIEW.contains(me.getSimpleName().toString())) {
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
      public Void visitEnhancedForLoop(EnhancedForLoopTree t, Void v) {
        // the entries of a typed map enter the variable of a loop over map.entrySet()
        Slot s = slots.get(elemOf(new TreePath(getCurrentPath(), t.getVariable())));
        if (s != null && s.kind == cx.entryKind) {
          Slot from = null;
          if (strip(t.getExpression()) instanceof MethodInvocationTree mi && mi.getArguments().isEmpty()
              && mi.getMethodSelect() instanceof MemberSelectTree ms && ms.getIdentifier().contentEquals("entrySet")) {
            from = slotOf(ms.getExpression(), new TreePath(new TreePath(getCurrentPath(), mi), ms));
          }
          if (from != null && from.kind.name.equals("map") && sameArgs(from.args, s.args)) {
            from.taints.add(s);
            s.made = true;
          } else {
            poison(s, "not a loop over the entries of a typed map");
          }
        }
        return super.visitEnhancedForLoop(t, v);
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
      if (s.why == null && !s.made) s.why = "no collection enters it";
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
    Map<String, int[]> counts = new TreeMap<>();
    Map<String, Integer> whys = new TreeMap<>(), elems = new TreeMap<>();
    boolean report = System.getenv("J2GO_COLLS") != null;
    for (Slot s : slots.values()) {
      int[] n = counts.computeIfAbsent(s.kind.name, k -> new int[6]);
      n[0]++;
      if (s.why != null) {
        whys.merge(s.kind.name + ": " + s.why.replaceAll("exchanges values with .*", "exchanges values with an erased declaration"), 1, Integer::sum);
        if ("2".equals(System.getenv("J2GO_COLLS"))) System.err.println("colls: erased " + s.kind.name + " " + s.v.getEnclosingElement() + "." + s.v + ": " + s.why);
        continue;
      }
      n[1]++;
      n[s.v.getKind() == ElementKind.FIELD ? 2 : s.v.getKind() == ElementKind.PARAMETER ? 4 : s.v instanceof ExecutableElement ? 5 : 3]++;
      elems.merge(s.kind.name + " of " + s.args, 1, Integer::sum);
      TypeMirror t = cx.typedOf(s.kind, s.args);
      if (s.v instanceof ExecutableElement m) cx.typedResults.put(m, t);
      else cx.refinedVars.put(s.v, t);
      for (Tree u : s.uses) cx.refinedTrees.put(u, t);
    }
    for (var e : counts.entrySet()) {
      int[] n = e.getValue();
      System.err.println("typed collections: " + n[1] + " of " + n[0] + " " + e.getKey() + " declarations are typed (" + n[2] + " fields, " + n[3] + " locals, "
          + n[4] + " parameters, " + n[5] + " results)");
    }
    if (report) {
      for (var e : untypable.entrySet()) System.err.println("colls: no typed collection for the type: " + e.getValue() + " " + e.getKey());
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
