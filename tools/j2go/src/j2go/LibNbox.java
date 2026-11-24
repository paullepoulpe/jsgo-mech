package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.TreePath;
import com.sun.source.util.TreePathScanner;
import com.sun.source.util.Trees;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Rule "typed library interfaces", piece `nbox` (J2GO_NOLIBGEN=nbox is the translation without
 * it; rule candidate R5 of docs/javaisms6/JAVAISMS6.md): a field or a method parameter declared
 * as a boxed primitive (`Boolean`, `Integer`, ...: a value that may be null, the tri-state flags
 * of the program) is a value of the runtime's `Nbox[T]` (rt_nbox.go), a primitive and a flag
 * whose zero value is Java's null, instead of an `any` holding the box:
 *
 * <pre>
 *   private @Nullable Boolean templateCheckResult = null;     templateCheckResult Nbox[bool]
 *   if (templateCheckResult != null) {                         if !this.templateCheckResult.null_() {
 *     return templateCheckResult;                                return this.templateCheckResult.get_()
 * </pre>
 *
 * and a parameter of a library function type with a boxed type argument (`Consumer<Boolean>
 * set`, `Supplier<@Nullable Integer> get`) is that function type at Nbox[T]
 * (`Consumer[Nbox[bool]]`), with the lambdas given for it (their parameters are Nbox[T] too):
 * the 38 option setters of CommandLineRunner are `func(v Nbox[bool])`, not `func(v any)`.
 *
 * It is the value-library mechanism of java.util.Optional (Ctx.valueLibs: zero value null,
 * `== null` is null_(), Object is box_()), for a stand-in class j2go.rt.Nbox<X> that only the
 * translator sees (Main.withStubs), so that the types the emitter works with say which variables
 * hold one (Ctx.refinedVars, refinedTrees; expressions read from them have the type Nbox<X>).
 * java.util.Optional's own Go type does not fit: its Object form is the Optional itself, where a
 * Boolean's is the bool, and a Boolean has two states (null, a value), not three. Null and the
 * value are told apart by the flag, for every value of X (false and 0 are values).
 *
 * Conversions (Ctx.coerce): a primitive becomes `nbox_(v)`, an Object or a box `nboxOf_[T](o)`
 * (nil is null), null `Nbox[T]{}`; to a primitive `n.get_()` (NullPointerException for null, as
 * Java's unboxing), to an Object or a box `n.box_()` (nil for null).
 *
 * A variable is left as it was (counted by reason in J2GO_LIBGEN's report, `nbox: left: ...`)
 * where one of its uses is not one the conversions above serve as Java does: a method called on
 * it, `++` / `+=`, an assignment used as a value, a string concatenation, a cast, `==` with
 * anything but null or a primitive (the identity of a box), or any other form not listed in
 * use(); a parameter also when its method overrides or is overridden, is referenced (::), is
 * a constructor, or (a function parameter) when the function is passed on as a value or a call
 * gives anything but a lambda for it; a field of a record or a handle class.
 */
final class LibNbox {
  static final String STUB = "j2go.rt.Nbox";

  static boolean on() {
    return LibGen.on("nbox");
  }

  final Ctx cx;
  final Trees trees;
  final GenScan gs;
  TypeElement nbox;

  LibNbox(Ctx cx, Trees trees, GenScan gs) {
    this.cx = cx;
    this.trees = trees;
    this.gs = gs;
  }

  /** The boxed primitive type t, or null. */
  TypeMirror box(TypeMirror t) {
    if (t == null || t.getKind() != TypeKind.DECLARED || !((DeclaredType) t).getTypeArguments().isEmpty()) return null;
    try {
      cx.types.unboxedType(t);
    } catch (IllegalArgumentException e) {
      return null;
    }
    return cx.types.erasure(t);
  }

  DeclaredType nboxOf(TypeMirror boxed) {
    return cx.types.getDeclaredType(nbox, box(boxed));
  }

  /** A library function type with a boxed type argument, as the same type with Nbox there; else null. */
  DeclaredType fnOf(TypeMirror t) {
    if (!FnTypes.ON || !(t instanceof DeclaredType d) || t.getKind() != TypeKind.DECLARED || !cx.fn.isLib(Ctx.typeElement(d))) return null;
    List<? extends TypeMirror> as = d.getTypeArguments();
    TypeMirror[] out = new TypeMirror[as.size()];
    boolean any = false;
    for (int i = 0; i < out.length; i++) {
      TypeMirror a = as.get(i);
      if (box(a) != null) {
        out[i] = nboxOf(a);
        any = true;
      } else {
        out[i] = a;
      }
    }
    return any ? cx.types.getDeclaredType(Ctx.typeElement(d), out) : null;
  }

  /** A candidate: a field or parameter, its refined type, its uses, why it is left (null: typed). */
  static final class Cand {
    final Element e;
    final TypeMirror refined;
    final boolean fn;
    Tree decl;
    boolean lambdaParam;
    final List<TreePath> uses = new ArrayList<>();
    String why;
    /** (fn) the lambdas given for it and their own candidates (the parameters at boxed positions). */
    final List<LambdaExpressionTree> lambdas = new ArrayList<>();
    final List<Cand> lambdaParams = new ArrayList<>();
    /** (piece `nboxarg`) the method references given for it, and the function candidates it is passed to or assigned from. */
    final List<MemberReferenceTree> refs = new ArrayList<>();
    final Set<Cand> links = new LinkedHashSet<>();

    Cand(Element e, TypeMirror refined, boolean fn) {
      this.e = e;
      this.refined = refined;
      this.fn = fn;
    }

    void left(String w) {
      if (why == null) why = w;
    }
  }

  final Map<Element, Cand> cands = new LinkedHashMap<>();
  final Map<ExecutableElement, List<Cand>> byMethod = new HashMap<>();

  void run(List<? extends CompilationUnitTree> units) {
    nbox = cx.elems.getTypeElement(STUB);
    if (nbox == null) return;
    // 1. the candidates: fields and method parameters declared as a box or a function type with a boxed type argument
    for (CompilationUnitTree cu : units) {
      new TreePathScanner<Void, Void>() {
        @Override
        public Void visitVariable(VariableTree t, Void v) {
          Element e = trees.getElement(getCurrentPath());
          if (e instanceof VariableElement ve) {
            TypeMirror b = box(ve.asType());
            DeclaredType f = b == null ? fnOf(ve.asType()) : null;
            if ((b != null || f != null) && (e.getKind() == ElementKind.FIELD || e.getKind() == ElementKind.PARAMETER && e.getEnclosingElement() instanceof ExecutableElement pm && pm.getParameters().contains(e))) {
              Cand c = new Cand(e, b != null ? nboxOf(b) : f, f != null);
              c.decl = t;
              if (e.getKind() == ElementKind.FIELD) {
                TypeElement owner = (TypeElement) e.getEnclosingElement();
                if (f != null && !argOn()) c.left("a field of a function type");
                else if (f != null && t.getInitializer() != null) c.left("a field of a function type with an initializer");
                else if (owner.getKind() == ElementKind.RECORD || owner.getKind().isInterface() || cx.handles.contains(owner)) c.left("a field of a record, an interface or a handle class");
              } else {
                ExecutableElement m = (ExecutableElement) e.getEnclosingElement();
                if (m.getKind() != ElementKind.METHOD && !(argOn() && f != null && m.getKind() == ElementKind.CONSTRUCTOR)) c.left("a parameter of a constructor");
                else if (m.getModifiers().contains(Modifier.ABSTRACT) || m.getModifiers().contains(Modifier.NATIVE)) c.left("a parameter of an abstract method");
                else if (cx.root(m) != m || gs.overridden.contains(m)) c.left("a parameter of a method that overrides or is overridden");
                byMethod.computeIfAbsent(m, k -> new ArrayList<>()).add(c);
              }
              cands.put(e, c);
            }
          }
          return super.visitVariable(t, v);
        }
      }.scan(cu, null);
    }
    if (cands.isEmpty()) return;
    // 2. the uses of each candidate, the calls and references of the methods of parameter candidates
    for (CompilationUnitTree cu : units) {
      new TreePathScanner<Void, Void>() {
        void use() {
          Element e = trees.getElement(getCurrentPath());
          Cand c = e == null ? null : cands.get(e);
          if (c != null) c.uses.add(getCurrentPath());
        }

        @Override
        public Void visitIdentifier(IdentifierTree t, Void v) {
          use();
          return super.visitIdentifier(t, v);
        }

        @Override
        public Void visitMemberSelect(MemberSelectTree t, Void v) {
          use();
          return super.visitMemberSelect(t, v);
        }

        @Override
        public Void visitLambdaExpression(LambdaExpressionTree t, Void v) {
          for (VariableTree vp : t.getParameters()) lambdaParamElems.put(vp, trees.getElement(new TreePath(getCurrentPath(), vp)));
          return super.visitLambdaExpression(t, v);
        }

        @Override
        public Void visitMemberReference(MemberReferenceTree t, Void v) {
          if (trees.getElement(getCurrentPath()) instanceof ExecutableElement m && byMethod.containsKey(m)) {
            for (Cand c : byMethod.get(m)) c.left("a parameter of a method referenced with ::");
          }
          return super.visitMemberReference(t, v);
        }

        @Override
        public Void visitMethodInvocation(MethodInvocationTree t, Void v) {
          if (trees.getElement(new TreePath(getCurrentPath(), t.getMethodSelect())) instanceof ExecutableElement m && byMethod.containsKey(m)) given(m, t.getArguments());
          return super.visitMethodInvocation(t, v);
        }

        @Override
        public Void visitNewClass(NewClassTree t, Void v) {
          // (piece `nboxarg`: a constructor's function parameter)
          if (argOn() && trees.getElement(getCurrentPath()) instanceof ExecutableElement m && byMethod.containsKey(m)) given(m, t.getArguments());
          return super.visitNewClass(t, v);
        }

        /** What a call of m gives for its function parameter candidates. */
        void given(ExecutableElement m, List<? extends ExpressionTree> args) {
          for (Cand c : byMethod.get(m)) {
            if (!c.fn) continue;
            int i = m.getParameters().indexOf((VariableElement) c.e);
            if (m.isVarArgs() && i == m.getParameters().size() - 1) {
              c.left("a parameter of a function type given as varargs");
              continue;
            }
            ExpressionTree a = GenScan.stripParens(args.get(i));
            if (a instanceof LambdaExpressionTree l) c.lambdas.add(l);
            else if (argOn() && a instanceof MemberReferenceTree r) c.refs.add(r);
            // (piece `nboxarg`: another function candidate of the same type, passed on: decided with it, fnUse)
            else if (argOn() && cands.get(trees.getElement(new TreePath(getCurrentPath(), a))) instanceof Cand o && o.fn && cx.types.isSameType(o.refined, c.refined)) {
              c.links.add(o);
              o.links.add(c);
            }
            else c.left(argOn() ? "a function parameter given something else than a lambda, a method reference or a candidate" : "a function parameter given something else than a lambda");
          }
        }
      }.scan(cu, null);
    }
    // 3. the lambdas' parameters at the boxed positions: candidates too, decided with the function parameter
    for (Cand c : new ArrayList<>(cands.values())) {
      if (!c.fn || c.why != null) continue;
      ExecutableElement fm = cx.functionalMethod(Ctx.typeElement(c.refined));
      List<? extends TypeMirror> pts = fm == null ? List.of() : ((ExecutableType) cx.types.asMemberOf((DeclaredType) c.refined, fm)).getParameterTypes();
      for (LambdaExpressionTree l : c.lambdas) {
        for (int i = 0; i < l.getParameters().size() && i < pts.size(); i++) {
          if (!cx.isValueLibType(pts.get(i)) || !Ctx.typeElement(pts.get(i)).equals(nbox)) continue;
          Element pe = lambdaParamElems.get(l.getParameters().get(i));
          if (pe == null) {
            c.left("a lambda parameter not found");
            continue;
          }
          Cand lc = new Cand(pe, pts.get(i), false);
          lc.decl = l.getParameters().get(i);
          lc.lambdaParam = true;
          c.lambdaParams.add(lc);
          cands.put(pe, lc);
        }
      }
    }
    // the uses of the lambdas' parameters
    Set<Element> lps = new HashSet<>();
    for (Cand c : cands.values()) for (Cand lc : c.lambdaParams) lps.add(lc.e);
    if (!lps.isEmpty()) {
      for (CompilationUnitTree cu : units) {
        new TreePathScanner<Void, Void>() {
          @Override
          public Void visitIdentifier(IdentifierTree t, Void v) {
            Element e = trees.getElement(getCurrentPath());
            if (e != null && lps.contains(e)) cands.get(e).uses.add(getCurrentPath());
            return super.visitIdentifier(t, v);
          }
        }.scan(cu, null);
      }
    }
    // 4. decide
    for (Cand c : cands.values()) {
      if (c.why != null) continue;
      for (TreePath u : c.uses) {
        String w = c.fn ? fnUse(c, u) : use(u);
        if (w != null) {
          c.left(w);
          break;
        }
      }
    }
    for (Cand c : cands.values()) {
      if (!c.fn) continue;
      for (Cand lc : c.lambdaParams) if (lc.why != null) c.left("a lambda given for it whose parameter is left: " + lc.why);
    }
    // (piece `nboxarg`: function candidates passed on to one another are typed together)
    for (boolean changed = true; changed; ) {
      changed = false;
      for (Cand c : cands.values()) {
        if (c.why != null) continue;
        for (Cand o : c.links) {
          if (o.why != null) {
            c.left("passed on to or from a function candidate that is left: " + o.why);
            changed = true;
            break;
          }
        }
      }
    }
    for (Cand c : cands.values()) {
      if (!c.fn) continue;
      if (c.why != null) for (Cand lc : c.lambdaParams) lc.left("the lambda's function parameter is left");
    }
    // 5. refine
    for (Cand c : cands.values()) {
      String kind = c.e.getKind() == ElementKind.FIELD ? "field" : c.fn ? "function parameter" : c.lambdaParam ? "lambda parameter" : "parameter";
      String site = c.e.getEnclosingElement() + "." + c.e;
      if (c.why != null) {
        cx.lg.note("nbox: left " + kind + ": " + c.why, site);
        continue;
      }
      cx.lg.note("nbox: typed " + kind, site);
      cx.refinedVars.put(c.e, c.refined);
      if (c.decl != null) cx.refinedTrees.put(c.decl, c.refined);
      for (TreePath u : c.uses) {
        refineUp(u, c.refined);
        if (c.fn && u.getParentPath().getLeaf() instanceof MemberSelectTree sel && u.getParentPath().getParentPath().getLeaf() instanceof MethodInvocationTree mi
            && trees.getElement(u.getParentPath()) instanceof ExecutableElement m) {
          refineUp(u.getParentPath().getParentPath(), ((ExecutableType) cx.types.asMemberOf((DeclaredType) c.refined, m)).getReturnType());
        }
      }
      for (LambdaExpressionTree l : c.lambdas) cx.refinedTrees.put(l, c.refined);
      for (MemberReferenceTree r : c.refs) cx.refinedTrees.put(r, c.refined);
    }
  }

  private final Map<Tree, Element> lambdaParamElems = new IdentityHashMap<>();

  /**
   * Why the use at path u of a scalar candidate is not one the conversions serve, or null. The
   * read's value is Nbox<X>: it must go where the emitter converts it to the type wanted there.
   */
  String use(TreePath u) {
    TreePath p = u.getParentPath();
    Tree child = u.getLeaf();
    while (p.getLeaf() instanceof ParenthesizedTree) {
      child = p.getLeaf();
      p = p.getParentPath();
    }
    Tree parent = p.getLeaf();
    switch (parent) {
      case AssignmentTree a -> {
        if (a.getVariable() == child && !(p.getParentPath().getLeaf() instanceof ExpressionStatementTree) && !voidLambdaBody(p)) return "an assignment used as a value";
        return null;
      }
      case VariableTree vt -> {
        return null;
      }
      case ReturnTree r -> {
        return null;
      }
      case MethodInvocationTree mi -> {
        if (mi.getMethodSelect() == child) return "a method reference or call on it";
        return null;
      }
      case NewClassTree nc -> {
        return nc.getArguments().contains(child) ? null : "an enclosing instance";
      }
      case LambdaExpressionTree l -> {
        return null;
      }
      case MemberSelectTree ms -> {
        return "a method called on it or a member selected";
      }
      case IfTree it -> {
        return null;
      }
      case WhileLoopTree w -> {
        return null;
      }
      case ConditionalExpressionTree ce -> {
        return null;
      }
      case UnaryTree un -> {
        return un.getKind() == Tree.Kind.LOGICAL_COMPLEMENT ? null : "an operand of " + un.getKind();
      }
      case CompoundAssignmentTree ca -> {
        return ca.getVariable() == child ? "updated by a compound assignment" : null;
      }
      case BinaryTree b -> {
        Tree other = b.getLeftOperand() == child ? b.getRightOperand() : b.getLeftOperand();
        TypeMirror ot = trees.getTypeMirror(new TreePath(p, other));
        if (b.getKind() == Tree.Kind.CONDITIONAL_AND || b.getKind() == Tree.Kind.CONDITIONAL_OR) return null;
        if (b.getKind() == Tree.Kind.PLUS && ot != null && ot.toString().equals("java.lang.String")) return "a string concatenation";
        if (ot != null && ot.getKind() == TypeKind.NULL && (b.getKind() == Tree.Kind.EQUAL_TO || b.getKind() == Tree.Kind.NOT_EQUAL_TO)) return null;
        if (ot != null && ot.getKind().isPrimitive()) return null;
        return "an operand of " + b.getKind() + " with " + ot;
      }
      default -> {
        return "a use as " + parent.getKind();
      }
    }
  }

  /** The expression at u, and the parentheses around it, have type t. */
  void refineUp(TreePath u, TypeMirror t) {
    for (TreePath p = u; p != null; p = p.getParentPath()) {
      cx.refinedTrees.put(p.getLeaf(), t);
      if (!(p.getParentPath().getLeaf() instanceof ParenthesizedTree)) break;
    }
  }

  /** Whether the expression at p is the body of a lambda whose function returns nothing (a statement). */
  boolean voidLambdaBody(TreePath p) {
    if (!(p.getParentPath().getLeaf() instanceof LambdaExpressionTree)) return false;
    TypeMirror lt = trees.getTypeMirror(p.getParentPath());
    ExecutableElement fm = lt == null || lt.getKind() != TypeKind.DECLARED ? null : cx.functionalMethod(Ctx.typeElement(lt));
    return fm != null && fm.getReturnType().getKind() == TypeKind.VOID;
  }

  /** Piece `nboxarg` (needs rule "function types" and piece `fns2`'s switch set): function fields, constructor parameters, method references, candidates passed on. */
  static boolean argOn() {
    return LibFns.nboxArgOn();
  }

  /** Why the use at u of a function candidate c is not a call of its method, or null. */
  String fnUse(Cand c, TreePath u) {
    TreePath p = u.getParentPath();
    if (argOn()) {
      Tree child = u.getLeaf();
      while (p.getLeaf() instanceof ParenthesizedTree) {
        child = p.getLeaf();
        p = p.getParentPath();
      }
      // an assignment of the field: from a candidate (linked), a lambda or a method reference
      if (p.getLeaf() instanceof AssignmentTree a && a.getVariable() == child) {
        if (!(p.getParentPath().getLeaf() instanceof ExpressionStatementTree)) return "an assignment used as a value";
        ExpressionTree x = GenScan.stripParens(a.getExpression());
        if (x instanceof LambdaExpressionTree l) c.lambdas.add(l);
        else if (x instanceof MemberReferenceTree r) c.refs.add(r);
        else if (cands.get(trees.getElement(new TreePath(p, x))) instanceof Cand o && o.fn && cx.types.isSameType(o.refined, c.refined)) {
          c.links.add(o);
          o.links.add(c);
        } else return "assigned something else than a lambda, a method reference or a candidate";
        return null;
      }
      // the value of an assignment of a candidate field: linked
      if (p.getLeaf() instanceof AssignmentTree a && a.getExpression() == child) {
        if (cands.get(trees.getElement(new TreePath(p, a.getVariable()))) instanceof Cand o && o.fn && cx.types.isSameType(o.refined, c.refined)) {
          c.links.add(o);
          o.links.add(c);
          return null;
        }
        return "passed on as a value";
      }
      // an argument for a function parameter candidate of the same type: linked
      Element callee = p.getLeaf() instanceof MethodInvocationTree mi0 ? trees.getElement(new TreePath(p, mi0.getMethodSelect())) : p.getLeaf() instanceof NewClassTree ? trees.getElement(p) : null;
      if (callee instanceof ExecutableElement m) {
        List<? extends ExpressionTree> args = p.getLeaf() instanceof MethodInvocationTree mi ? mi.getArguments() : ((NewClassTree) p.getLeaf()).getArguments();
        int i = args.indexOf(child);
        if (i >= 0 && i < m.getParameters().size() && !(m.isVarArgs() && i == m.getParameters().size() - 1)
            && cands.get(m.getParameters().get(i)) instanceof Cand o && o.fn && cx.types.isSameType(o.refined, c.refined)) {
          c.links.add(o);
          o.links.add(c);
          return null;
        }
      }
      p = u.getParentPath();
    }
    if (p.getLeaf() instanceof MemberSelectTree && p.getParentPath().getLeaf() instanceof MethodInvocationTree mi && mi.getMethodSelect() == p.getLeaf()) {
      // the call's result: a boxed value, used as a scalar candidate's read would be
      TypeMirror r = trees.getTypeMirror(p.getParentPath());
      if (r != null && box(r) != null) return use(p.getParentPath());
      return null;
    }
    return "passed on as a value";
  }
}
