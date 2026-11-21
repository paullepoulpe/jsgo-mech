package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Which type parameters of the program's generic classes, interfaces and methods are Go type
 * parameters (Rule "typed generics"). A type parameter P of declaration D is kept when every
 * mention of D in the program names, at P's position, a type that one Go type stands for: a
 * concrete type, or a type variable that is kept too (fixpoint), or one that stands for its bound
 * (below). Then an object made as D<..X..> is only ever seen with X at that position, and D[X] is
 * sound. A wildcard or a raw use at P's position, or a type variable that stays erased, erases P
 * alone: it is `any` in every instantiation of D, so all views of one object have one Go type.
 *
 * Classes may extend classes (a struct embeds its superclass at its view of it), hold local and
 * anonymous classes (which take the type parameters of their scope) and generic methods.
 * A declaration is left wholly erased when it is a record, a handle class or holds an inner
 * class; an interface also when it is tested with instanceof (Go has no test for "some
 * instantiation of I"; a class has the marker method isa_C_). A cast to an array of a type
 * parameter from anything but a fresh array erases it (Go converts arrays by a copy).
 *
 * A bounded type parameter is kept when its one bound is an interface of the program (the Go
 * constraint). A class type parameter bounded by a class stays erased (it is its bound); when
 * every use instantiates it with exactly that bound (AbstractCommandLineRunner<Compiler,
 * CompilerOptions>) it stands for the bound as a type argument. A type variable of a generic
 * method that can be a Go generic function (static, or an instance method that overrides
 * nothing and that nothing overrides) is kept when its class keeps a type parameter (static
 * factories) or when it is a type argument at a kept position (Outcome<E> in a generic method).
 */
final class GenScan {
  /** Candidate declarations and, for those left wholly erased, why. */
  final Map<TypeElement, String> why = new LinkedHashMap<>();
  /** Type parameters of candidates and, for those erased, why. */
  final Map<TypeParameterElement, String> pwhy = new LinkedHashMap<>();
  /** Type parameter -> the type variables named at its position in the uses of its declaration. */
  final Map<TypeParameterElement, Set<TypeParameterElement>> deps = new LinkedHashMap<>();

  final Ctx cx;
  final Trees trees;

  GenScan(Ctx cx, Trees trees) {
    this.cx = cx;
    this.trees = trees;
  }

  static boolean bounded(TypeParameterElement tp) {
    for (TypeMirror b : tp.getBounds()) {
      if (!(b instanceof DeclaredType d && Ctx.typeElement(d).getQualifiedName().contentEquals("java.lang.Object"))) return true;
    }
    return false;
  }

  /**
   * Why bounded type parameter tp cannot be a Go type parameter, or null: it can when its one
   * bound is an interface of the program (the Go constraint; String, boxed values and Object, which
   * Go types could not satisfy it with, cannot implement it either).
   */
  String boundWhy(TypeParameterElement tp) {
    if (!bounded(tp)) return null;
    if (tp.getBounds().size() != 1) return "bounded by an intersection";
    TypeMirror b = tp.getBounds().get(0);
    TypeElement bt = b.getKind() == TypeKind.DECLARED ? Ctx.typeElement(b) : null;
    if (bt == null || !bt.getKind().isInterface() || !cx.isSource(bt)) return "bounded by a class or a library interface";
    return null;
  }

  /**
   * Class type parameters erased to their bound that every use outside the class's body
   * instantiates with exactly that bound (AbstractCommandLineRunner<Compiler, CompilerOptions>):
   * as a type argument they stand for the bound's Go type, one type for every instantiation.
   */
  final Set<TypeParameterElement> unfixed = new HashSet<>();

  boolean fixed(TypeParameterElement tp) {
    return tp.getGenericElement() instanceof TypeElement && boundWhy(tp) != null && !unfixed.contains(tp);
  }

  void candidates(Collection<TypeElement> sources) {
    for (TypeElement te : sources) {
      if (te.getTypeParameters().isEmpty()) continue;
      why.put(te, null);
      String w = null;
      boolean iface = te.getKind() == ElementKind.INTERFACE;
      if (te.getKind() != ElementKind.CLASS && !iface) w = "a record or annotation";
      else if (iface) {
        // a generic interface of the program: a generic Go interface (I<X> is I[X])
      } else if (te.getNestingKind() != NestingKind.TOP_LEVEL && !te.getModifiers().contains(Modifier.STATIC)) w = "an inner class";
      else if (cx.handles.contains(te)) w = "a handle class";
      // Implementing an interface of the program is fine: a typed one is seen at the class's
      // view of it, an erased one keeps its erased signatures (converted at the method's entry).
      if (w == null) {
        for (Element e : te.getEnclosedElements()) {
          if (e instanceof TypeElement n && !n.getModifiers().contains(Modifier.STATIC) && !n.getKind().isInterface()
              && n.getKind() != ElementKind.ENUM && n.getKind() != ElementKind.RECORD) w = "holds an inner class";
        }
      }
      if (w != null) why.put(te, w);
      for (TypeParameterElement tp : te.getTypeParameters()) pwhy.put(tp, boundWhy(tp));
    }
  }

  void erase(TypeElement te, String w) {
    for (TypeParameterElement tp : te.getTypeParameters()) erase(tp, w);
  }

  /** J2GO_GENERICS=2: every reason to erase a type parameter, not only the first. */
  static final boolean ALL = "2".equals(System.getenv("J2GO_GENERICS"));

  void erase(TypeParameterElement tp, String w) {
    if (ALL && pwhy.containsKey(tp)) System.err.println("erase " + tp.getGenericElement() + "." + tp + ": " + w);
    if (pwhy.containsKey(tp) && pwhy.get(tp) == null) pwhy.put(tp, w);
  }

  /**
   * Rule "refined locals": a local variable (or the variable of a for-each loop) that is never
   * assigned again, whose declared type C<..?..> has wildcard type arguments, and whose
   * initializer (or iterated element) has a type whose view of C names a type at such a position,
   * has that type there instead. Java's own typing allows it: the variable only ever holds that
   * one value, and a wildcard promises less than the type it stands for. The expressions that
   * read the variable, and method calls on them, are typed from the refined type. Then a wildcard
   * written only for brevity (`for (DiGraphEdge<?, E> edge : getEdgesInDirection(a, b))`) does
   * not erase the type parameter for the whole program.
   */
  void refine(CompilationUnitTree cu) {
    Set<Element> assigned = new HashSet<>();
    new TreePathScanner<Void, Void>() {
      void note(Tree target) {
        Element e = trees.getElement(new TreePath(getCurrentPath(), target));
        if (e != null) assigned.add(e);
      }

      @Override
      public Void visitAssignment(AssignmentTree t, Void v) {
        note(t.getVariable());
        return super.visitAssignment(t, v);
      }

      @Override
      public Void visitCompoundAssignment(CompoundAssignmentTree t, Void v) {
        note(t.getVariable());
        return super.visitCompoundAssignment(t, v);
      }

      @Override
      public Void visitUnary(UnaryTree t, Void v) {
        switch (t.getKind()) {
          case PREFIX_INCREMENT, PREFIX_DECREMENT, POSTFIX_INCREMENT, POSTFIX_DECREMENT -> note(t.getExpression());
          default -> {
          }
        }
        return super.visitUnary(t, v);
      }
    }.scan(cu, null);
    new TreePathScanner<Void, Void>() {
      TypeMirror ty(Tree child) {
        TypeMirror r = cx.refinedTrees.get(child);
        if (r != null) return r;
        try {
          return trees.getTypeMirror(new TreePath(getCurrentPath(), child));
        } catch (RuntimeException ex) {
          return null;
        }
      }

      @Override
      public Void visitVariable(VariableTree t, Void v) {
        super.visitVariable(t, v);
        Element e = trees.getElement(getCurrentPath());
        if (e != null && e.getKind() == ElementKind.LOCAL_VARIABLE && t.getInitializer() != null && !assigned.contains(e)) tryRefine(e, t, ty(t.getInitializer()));
        return null;
      }

      @Override
      public Void visitEnhancedForLoop(EnhancedForLoopTree t, Void v) {
        scan(t.getExpression(), v);
        Element e = trees.getElement(new TreePath(getCurrentPath(), t.getVariable()));
        if (e != null && !assigned.contains(e)) tryRefine(e, t.getVariable(), elementType(ty(t.getExpression())));
        scan(t.getVariable(), v);
        scan(t.getStatement(), v);
        return null;
      }

      @Override
      public Void visitIdentifier(IdentifierTree t, Void v) {
        TypeMirror r = cx.refinedVars.get(trees.getElement(getCurrentPath()));
        if (r != null) cx.refinedTrees.put(t, r);
        return null;
      }

      @Override
      public Void visitParenthesized(ParenthesizedTree t, Void v) {
        super.visitParenthesized(t, v);
        TypeMirror r = cx.refinedTrees.get(t.getExpression());
        if (r != null) cx.refinedTrees.put(t, r);
        return null;
      }

      @Override
      public Void visitMethodInvocation(MethodInvocationTree t, Void v) {
        super.visitMethodInvocation(t, v);
        if (t.getMethodSelect() instanceof MemberSelectTree sel && cx.refinedTrees.get(sel.getExpression()) instanceof DeclaredType r
            && trees.getElement(new TreePath(getCurrentPath(), sel)) instanceof ExecutableElement m
            && !m.getModifiers().contains(Modifier.STATIC) && m.getTypeParameters().isEmpty()) {
          try {
            cx.refinedTrees.put(t, ((ExecutableType) cx.types.asMemberOf(r, m)).getReturnType());
          } catch (RuntimeException ex) {
            // not a member of the refined type as javac sees it: the tree keeps its own type
          }
        }
        // <T> T m(.., T x, ..) called with a refined argument for x (checkNotNull(child)): the argument's type
        Element me = trees.getElement(new TreePath(getCurrentPath(), t.getMethodSelect()));
        if (me instanceof ExecutableElement m && m.getReturnType().getKind() == TypeKind.TYPEVAR && m.getTypeParameters().contains(((TypeVariable) m.getReturnType()).asElement())) {
          for (int i = 0; i < t.getArguments().size() && i < m.getParameters().size(); i++) {
            TypeMirror r = cx.refinedTrees.get(t.getArguments().get(i));
            if (r != null && cx.types.isSameType(m.getParameters().get(i).asType(), m.getReturnType())) cx.refinedTrees.put(t, r);
          }
        }
        return null;
      }
    }.scan(cu, null);
  }

  /** Whether t is C<?, ..., ?>: a candidate generic class with only unbounded wildcards. */
  boolean allWild(TypeMirror t) {
    if (!(t instanceof DeclaredType d) || t.getKind() != TypeKind.DECLARED || d.getTypeArguments().isEmpty() || !why.containsKey(Ctx.typeElement(d))) return false;
    for (TypeMirror a : d.getTypeArguments()) {
      if (!(a instanceof WildcardType w) || w.getExtendsBound() != null || w.getSuperBound() != null) return false;
    }
    return true;
  }

  /** Whether t is an instantiation of te without wildcards. */
  static boolean instOf(TypeMirror t, TypeElement te) {
    if (!(t instanceof DeclaredType d) || t.getKind() != TypeKind.DECLARED || !te.equals(Ctx.typeElement(d)) || d.getTypeArguments().isEmpty()) return false;
    for (TypeMirror a : d.getTypeArguments()) if (a.getKind() == TypeKind.WILDCARD || captured(a)) return false;
    return true;
  }

  /**
   * Rule "shared statics": a static final field declared C<?, ..., ?> and initialized with
   * `new C<>(...)`, which the program only casts to an instantiation of C or compares (==, !=)
   * with a value of one, is one object per instantiation: a generic function C_F[K, V]() that
   * makes it on first use (single_, rt_generic.go). Java shares one object between all
   * instantiations; nothing can tell, because a value of C<A, B> is only ever compared with
   * values of C<A, B> (the Go types say so). The constructor then runs once per instantiation, on
   * first use instead of at class initialization: it must not have effects beyond the new object.
   * A static final field C<?, ..., ?>[] initialized with an array of length 0, which is only cast
   * to an instantiation's array, is an empty array of that instantiation at each cast (an empty
   * array has no state; the program must not compare it by identity, and a cast is no comparison).
   * A new array of C<?, ..., ?> directly under a cast to an instantiation's array is a new array
   * of that instantiation.
   */
  void shared(List<? extends CompilationUnitTree> units) {
    Map<Element, String> cand = new LinkedHashMap<>();
    Map<Element, VariableTree> decl = new HashMap<>();
    Map<Element, TreePath> declPath = new HashMap<>();
    for (CompilationUnitTree cu : units) {
      new TreePathScanner<Void, Void>() {
        @Override
        public Void visitVariable(VariableTree t, Void v) {
          Element e = trees.getElement(getCurrentPath());
          if (e != null && e.getKind() == ElementKind.FIELD && e.getModifiers().contains(Modifier.STATIC) && e.getModifiers().contains(Modifier.FINAL)
              && t.getInitializer() != null) {
            TypeMirror ft = e.asType();
            ExpressionTree init = t.getInitializer();
            while (init instanceof ParenthesizedTree p) init = p.getExpression();
            String kind = null;
            if (ft.getKind() == TypeKind.ARRAY && allWild(((ArrayType) ft).getComponentType())) {
              if (init instanceof NewArrayTree na && na.getInitializers() == null && na.getDimensions().size() == 1
                  && na.getDimensions().get(0) instanceof LiteralTree lit && Integer.valueOf(0).equals(lit.getValue())) kind = "array";
            } else if (allWild(ft)) {
              if (init instanceof NewClassTree nc && nc.getClassBody() == null
                  && trees.getElement(new TreePath(getCurrentPath(), nc)) instanceof ExecutableElement ctor
                  && ctor.getEnclosingElement().equals(Ctx.typeElement(ft))) kind = "single";
            }
            if (kind != null) {
              cand.put(e, kind);
              decl.put(e, t);
              declPath.put(e, getCurrentPath());
            }
          }
          return super.visitVariable(t, v);
        }
      }.scan(cu, null);
    }
    if (cand.isEmpty()) return;
    Map<Element, String> bad = new HashMap<>();
    Map<Element, List<Tree>> uses = new HashMap<>();
    for (CompilationUnitTree cu : units) {
      new TreePathScanner<Void, Void>() {
        void use() {
          Element e = trees.getElement(getCurrentPath());
          if (!cand.containsKey(e)) return;
          TypeElement te = Ctx.typeElement(e.asType().getKind() == TypeKind.ARRAY ? ((ArrayType) e.asType()).getComponentType() : e.asType());
          TreePath pp = getCurrentPath().getParentPath();
          Tree child = getCurrentPath().getLeaf();
          while (pp.getLeaf() instanceof ParenthesizedTree) {
            child = pp.getLeaf();
            pp = pp.getParentPath();
          }
          boolean ok = false;
          if (pp.getLeaf() instanceof TypeCastTree) {
            TypeMirror tm = trees.getTypeMirror(pp);
            ok = cand.get(e).equals("array") ? tm.getKind() == TypeKind.ARRAY && instOf(((ArrayType) tm).getComponentType(), te) : instOf(tm, te);
          } else if (pp.getLeaf() instanceof BinaryTree b && (b.getKind() == Tree.Kind.EQUAL_TO || b.getKind() == Tree.Kind.NOT_EQUAL_TO) && cand.get(e).equals("single")) {
            Tree other = b.getLeftOperand() == child ? b.getRightOperand() : b.getLeftOperand();
            ok = instOf(trees.getTypeMirror(new TreePath(pp, other)), te);
          }
          if (!ok) bad.putIfAbsent(e, "used at " + cu.getSourceFile().getName().replaceAll(".*/", "") + ":" + cu.getLineMap().getLineNumber(trees.getSourcePositions().getStartPosition(cu, getCurrentPath().getLeaf())));
          uses.computeIfAbsent(e, k -> new ArrayList<>()).add(getCurrentPath().getLeaf());
        }

        @Override
        public Void visitIdentifier(IdentifierTree t, Void v) {
          use();
          return null;
        }

        @Override
        public Void visitMemberSelect(MemberSelectTree t, Void v) {
          use();
          return super.visitMemberSelect(t, v);
        }

        @Override
        public Void visitTypeCast(TypeCastTree t, Void v) {
          // (C<K, V>[]) new C<?, ?>[n]: a new array of the instantiation
          ExpressionTree x = t.getExpression();
          while (x instanceof ParenthesizedTree p) x = p.getExpression();
          TypeMirror tm = trees.getTypeMirror(getCurrentPath());
          if (x instanceof NewArrayTree na && na.getType() != null && tm != null && tm.getKind() == TypeKind.ARRAY) {
            TypeMirror comp = ((ArrayType) tm).getComponentType();
            TypeMirror nt = trees.getTypeMirror(new TreePath(getCurrentPath(), t.getExpression()));
            if (nt != null && nt.getKind() == TypeKind.ARRAY && allWild(((ArrayType) nt).getComponentType())
                && instOf(comp, Ctx.typeElement(((ArrayType) nt).getComponentType()))) {
              cx.refinedTrees.put(na, tm);
              cx.refinedTypeTrees.add(na.getType());
            }
          }
          return super.visitTypeCast(t, v);
        }
      }.scan(cu, null);
    }
    for (var c : cand.entrySet()) {
      Element f = c.getKey();
      if (bad.containsKey(f)) {
        if (System.getenv("J2GO_GENERICS") != null) System.err.println("static " + f.getEnclosingElement() + "." + f + " of a wildcard type is not shared: " + bad.get(f));
        continue;
      }
      cx.sharedStatics.put(f, c.getValue());
      cx.refinedTypeTrees.add(decl.get(f));
      for (Tree u : uses.getOrDefault(f, List.of())) cx.refinedTypeTrees.add(u);
      if (System.getenv("J2GO_GENERICS") != null) System.err.println("shared static " + f.getEnclosingElement() + "." + f + ": " + c.getValue());
      if (c.getValue().equals("single")) {
        // The initializer is translated at C's own type parameters; a call of a generic method
        // among the constructor's arguments returns the parameter's type.
        ExpressionTree init = decl.get(f).getInitializer();
        while (init instanceof ParenthesizedTree p) init = p.getExpression();
        NewClassTree nc = (NewClassTree) init;
        cx.refinedTrees.put(nc, Ctx.typeElement(f.asType()).asType());
        ExecutableElement ctor = (ExecutableElement) trees.getElement(new TreePath(declPath.get(f), nc));
        for (int i = 0; i < nc.getArguments().size() && i < ctor.getParameters().size(); i++) {
          ExpressionTree a = nc.getArguments().get(i);
          while (a instanceof ParenthesizedTree p) a = p.getExpression();
          if (a instanceof MethodInvocationTree) cx.refinedTrees.put(a, ctor.getParameters().get(i).asType());
        }
      }
    }
  }

  static boolean captured(TypeMirror t) {
    return t.getKind() == TypeKind.TYPEVAR && t.toString().startsWith("capture");
  }

  /** The type of the elements a for-each loop over a value of type t iterates over, or null. */
  TypeMirror elementType(TypeMirror t) {
    if (t == null) return null;
    if (t.getKind() == TypeKind.ARRAY) return ((ArrayType) t).getComponentType();
    if (captured(t)) t = ((TypeVariable) t).getUpperBound();
    TypeMirror it = viewOf(t, cx.elems.getTypeElement("java.lang.Iterable"), 0);
    if (!(it instanceof DeclaredType d) || d.getTypeArguments().size() != 1) return null;
    TypeMirror a = d.getTypeArguments().get(0);
    if (a.getKind() == TypeKind.WILDCARD) a = ((WildcardType) a).getExtendsBound();
    if (a != null && captured(a)) a = ((TypeVariable) a).getUpperBound();
    return a;
  }

  /** t's view of declaration te (t itself or the supertype of t that is a te<...>), or null. */
  TypeMirror viewOf(TypeMirror t, TypeElement te, int depth) {
    if (t == null || depth > 12) return null;
    if (t.getKind() == TypeKind.TYPEVAR) return viewOf(((TypeVariable) t).getUpperBound(), te, depth + 1);
    if (t.getKind() == TypeKind.INTERSECTION) {
      for (TypeMirror b : ((IntersectionType) t).getBounds()) {
        TypeMirror r = viewOf(b, te, depth + 1);
        if (r != null) return r;
      }
      return null;
    }
    if (t.getKind() != TypeKind.DECLARED) return null;
    if (te.equals(Ctx.typeElement(t))) return t;
    for (TypeMirror s : cx.types.directSupertypes(t)) {
      TypeMirror r = viewOf(s, te, depth + 1);
      if (r != null) return r;
    }
    return null;
  }

  int nRefined;

  void tryRefine(Element e, VariableTree vt, TypeMirror src) {
    if (src == null || !(e.asType() instanceof DeclaredType decl) || decl.getKind() != TypeKind.DECLARED) return;
    TypeElement te = Ctx.typeElement(decl);
    List<? extends TypeMirror> da = decl.getTypeArguments();
    boolean wild = false;
    for (TypeMirror a : da) wild |= a.getKind() == TypeKind.WILDCARD;
    // (piece `cov` of rule "typed library interfaces": a local of a typed library interface too,
    // `List<? extends DiGraphNode<N, E>> next = graph.getDirectedSuccNodes(n)` with the narrowed result)
    boolean lib = !cx.isSource(te) && LibGen.kindOf(te) != null && LibGen.on(LibGen.kindOf(te)) && LibGen.on("cov");
    if (!wild || !cx.isSource(te) && !lib || (te.getNestingKind() != NestingKind.TOP_LEVEL && !te.getModifiers().contains(Modifier.STATIC) && !te.getKind().isInterface())) return;
    if (!(viewOf(src, te, 0) instanceof DeclaredType view) || view.getTypeArguments().size() != da.size()) return;
    TypeMirror[] args = new TypeMirror[da.size()];
    boolean any = false;
    for (int i = 0; i < args.length; i++) {
      TypeMirror a = da.get(i), s = view.getTypeArguments().get(i);
      if (a.getKind() == TypeKind.WILDCARD && s.getKind() != TypeKind.WILDCARD && !captured(s)) {
        args[i] = s;
        any = true;
      } else {
        args[i] = a;
      }
    }
    if (!any) return;
    TypeMirror refined = cx.types.getDeclaredType(te, args);
    cx.refinedVars.put(e, refined);
    cx.refinedTrees.put(vt, refined);
    cx.refinedTypeTrees.add(vt.getType());
    nRefined++;
    if (System.getenv("J2GO_GENERICS") != null) System.err.println("refined local " + e + ": " + decl + " -> " + refined);
  }

  void scan(CompilationUnitTree cu) {
    new TreePathScanner<Void, Void>() {
      String where() {
        CompilationUnitTree cu = getCurrentPath().getCompilationUnit();
        long pos = trees.getSourcePositions().getStartPosition(cu, getCurrentPath().getLeaf());
        return " (" + cu.getSourceFile().getName().replaceAll(".*/", "") + ":" + cu.getLineMap().getLineNumber(pos) + ")";
      }

      void check(TypeMirror t, int depth) {
        if (t == null || depth > 8) return;
        switch (t.getKind()) {
          case ARRAY -> check(((ArrayType) t).getComponentType(), depth + 1);
          case DECLARED -> {
            DeclaredType d = (DeclaredType) t;
            TypeElement te = Ctx.typeElement(d);
            List<? extends TypeMirror> args = d.getTypeArguments();
            if (why.containsKey(te)) {
              List<? extends TypeParameterElement> ps = te.getTypeParameters();
              if (args.isEmpty()) {
                erase(te, "used raw" + where());
                unfixed.addAll(ps);
              }
              for (int i = 0; i < args.size() && i < ps.size(); i++) {
                TypeMirror a = args.get(i);
                TypeParameterElement pi = ps.get(i);
                boolean exactBound = pi.getBounds().size() == 1 && a.getKind() == TypeKind.DECLARED && ((DeclaredType) a).getTypeArguments().isEmpty()
                    && cx.types.isSameType(cx.types.erasure(a), cx.types.erasure(pi.getBounds().get(0)));
                boolean itself = a.getKind() == TypeKind.TYPEVAR && ((TypeVariable) a).asElement() == pi;
                if (!exactBound && !itself) unfixed.add(pi);
                if (a.getKind() == TypeKind.WILDCARD) erase(ps.get(i), "a wildcard" + where());
                else if (a.getKind() == TypeKind.TYPEVAR) {
                  Element v = ((TypeVariable) a).asElement();
                  if (v instanceof TypeParameterElement vp) {
                    if (vp != ps.get(i)) deps.computeIfAbsent(ps.get(i), k -> new LinkedHashSet<>()).add(vp);
                    if (ALL && captured(a)) System.err.println("erase " + te + "." + ps.get(i) + ": a captured type" + where());
                  } else erase(ps.get(i), "a captured type" + where());
                } else if (a.getKind() == TypeKind.INTERSECTION || a.getKind() == TypeKind.UNION) erase(ps.get(i), "an intersection type" + where());
              }
            }
            for (TypeMirror a : args) check(a, depth + 1);
          }
          case WILDCARD -> {
            WildcardType w = (WildcardType) t;
            check(w.getExtendsBound(), depth + 1);
            check(w.getSuperBound(), depth + 1);
          }
          default -> {
          }
        }
      }

      void checkBounds(List<? extends TypeParameterElement> tps) {
        for (TypeParameterElement tp : tps) for (TypeMirror b : tp.getBounds()) check(b, 0);
      }

      @Override
      public Void scan(Tree t, Void v) {
        if (t != null) {
          // the type written for a refined local is not a use of it
          if (cx.refinedTypeTrees.contains(t)) return null;
          TreePath p = new TreePath(getCurrentPath(), t);
          TypeMirror tm = cx.refinedTrees.get(t);
          try {
            if (tm == null) tm = trees.getTypeMirror(p);
          } catch (RuntimeException ex) {
            // no type for this tree
          }
          // a name of a type (Foo in Foo.m() or new Foo<>()) is not a use of a value of it
          boolean typeName = (t instanceof IdentifierTree || t instanceof MemberSelectTree || t instanceof ParameterizedTypeTree)
              && trees.getElement(p) instanceof TypeElement;
          if (tm != null && !(t instanceof ClassTree) && !typeName) {
            check(tm, 0);
          }
        }
        return super.scan(t, v);
      }

      @Override
      public Void visitTypeCast(TypeCastTree t, Void v) {
        // (T[]) array: Java shares the array; Go converts by a copy, which is only right for a fresh array
        TypeMirror tm = trees.getTypeMirror(getCurrentPath());
        ExpressionTree x = t.getExpression();
        while (x instanceof ParenthesizedTree pt) x = pt.getExpression();
        boolean sharedArray = "array".equals(cx.sharedStatics.get(trees.getElement(new TreePath(getCurrentPath(), t.getExpression()))))
            || "array".equals(cx.sharedStatics.get(trees.getElement(new TreePath(new TreePath(getCurrentPath(), t.getExpression()), x))));
        if (tm != null && tm.getKind() == TypeKind.ARRAY && !(x instanceof NewArrayTree) && !sharedArray) eraseIn(((ArrayType) tm).getComponentType(), "a cast to an array" + where(), 0);
        return super.visitTypeCast(t, v);
      }

      void eraseIn(TypeMirror t, String w, int depth) {
        if (t == null || depth > 8) return;
        switch (t.getKind()) {
          case TYPEVAR -> {
            if (((TypeVariable) t).asElement() instanceof TypeParameterElement tp) {
              if (pwhy.containsKey(tp)) erase(tp, w);
              else erasedMethodVars.put(tp, w);
            }
          }
          case ARRAY -> eraseIn(((ArrayType) t).getComponentType(), w, depth + 1);
          case DECLARED -> {
            TypeElement te = Ctx.typeElement(t);
            if (why.containsKey(te)) erase(te, w);
            for (TypeMirror a : ((DeclaredType) t).getTypeArguments()) eraseIn(a, w, depth + 1);
          }
          default -> {
          }
        }
      }

      @Override
      public Void visitInstanceOf(InstanceOfTree t, Void v) {
        // `x instanceof C` asks about every instantiation: Go has no such test for a generic type
        Tree tt = t.getType() != null ? t.getType() : t.getPattern();
        if (tt != null) {
          TypeMirror tm = trees.getTypeMirror(new TreePath(getCurrentPath(), tt));
          TypeElement te = tm != null && tm.getKind() == TypeKind.DECLARED ? Ctx.typeElement(tm) : null;
          if (te != null && te.getKind().isInterface() && why.containsKey(te) && why.get(te) == null) why.put(te, "tested with instanceof" + where());
        }
        return super.visitInstanceOf(t, v);
      }

      @Override
      public Void visitMethod(MethodTree t, Void v) {
        if (trees.getElement(getCurrentPath()) instanceof ExecutableElement m) {
          check(m.getReturnType(), 0);
          for (VariableElement p : m.getParameters()) check(p.asType(), 0);
          checkBounds(m.getTypeParameters());
        }
        return super.visitMethod(t, v);
      }

      @Override
      public Void visitClass(ClassTree t, Void v) {
        if (trees.getElement(getCurrentPath()) instanceof TypeElement te) {
          check(te.getSuperclass(), 0);
          for (TypeMirror i : te.getInterfaces()) check(i, 0);
          checkBounds(te.getTypeParameters());
        }
        return super.visitClass(t, v);
      }
    }.scan(cu, null);
  }

  /** Type variables of static generic methods that must stay erased, and why. */
  final Map<TypeParameterElement, String> erasedMethodVars = new HashMap<>();

  /** Instance methods of the program that some other method overrides. */
  final Set<ExecutableElement> overridden = new HashSet<>();

  void noteOverrides(Collection<TypeElement> sources) {
    for (TypeElement te : sources) {
      for (Element e : te.getEnclosedElements()) {
        if (e instanceof ExecutableElement m && Ctx.isInstanceMethod(m) && cx.root(m) != m) overridden.add(cx.root(m));
      }
    }
  }

  /**
   * A generic method that can be a Go generic function: a static one, or an instance method of a
   * class that overrides nothing and that nothing overrides (it becomes a function taking the
   * receiver first: Go methods have no type parameters of their own).
   */
  boolean genericFunction(ExecutableElement m) {
    if (m.getKind() != ElementKind.METHOD || !(m.getEnclosingElement() instanceof TypeElement owner) || !cx.isSource(owner)) return false;
    if (m.getModifiers().contains(Modifier.STATIC)) return true;
    return !owner.getKind().isInterface() && !m.getModifiers().contains(Modifier.ABSTRACT) && cx.root(m) == m && !overridden.contains(m)
        && !cx.handles.contains(owner);
  }

  /** A type variable that is a Go type parameter. */
  boolean kept(TypeParameterElement tp) {
    if (erasedMethodVars.containsKey(tp)) return false;
    Element o = tp.getGenericElement();
    if (o instanceof TypeElement t) return why.containsKey(t) && why.get(t) == null && pwhy.get(tp) == null;
    // A type variable of a generic function: kept when the function's class keeps a type parameter
    // (the static factories of rule 1), or when it is a type argument at a kept position (a
    // class bound is then its box's type: the function's code converts to the bound where it is used).
    if (!(o instanceof ExecutableElement m && genericFunction(m))) return false;
    if (tp.getBounds().size() > 1) return false;
    if (m.getModifiers().contains(Modifier.STATIC) && boundWhy(tp) == null && m.getEnclosingElement() instanceof TypeElement ot && why.containsKey(ot) && why.get(ot) == null) {
      for (TypeParameterElement cp : ot.getTypeParameters()) if (pwhy.get(cp) == null) return true;
    }
    for (var d : deps.entrySet()) {
      TypeParameterElement dp = d.getKey();
      if (d.getValue().contains(tp) && dp.getGenericElement() instanceof TypeElement dt && why.get(dt) == null && pwhy.get(dp) == null) return true;
    }
    return false;
  }

  private Set<TypeParameterElement> kept;

  /** The type parameters that are Go type parameters: of candidate declarations and their static generic methods. */
  Set<TypeParameterElement> kept() {
    if (kept != null) return kept;
    for (boolean changed = true; changed; ) {
      changed = false;
      for (var d : deps.entrySet()) {
        if (!kept(d.getKey())) continue;
        for (TypeParameterElement o : d.getValue()) {
          if (!kept(o) && !fixed(o)) {
            pwhy.put(d.getKey(), "fed by the erased type variable " + o + " of " + o.getGenericElement());
            changed = true;
            break;
          }
        }
      }
    }
    kept = new LinkedHashSet<>();
    why.forEach((te, w) -> {
      if (w != null) return;
      for (TypeParameterElement tp : te.getTypeParameters()) if (kept(tp)) kept.add(tp);
      for (Element e : te.getEnclosedElements()) {
        if (e instanceof ExecutableElement m) for (TypeParameterElement tp : m.getTypeParameters()) if (kept(tp)) kept.add(tp);
      }
    });
    // generic functions of any class, kept because they are type arguments at a kept position
    for (Set<TypeParameterElement> vs : deps.values()) {
      for (TypeParameterElement tp : vs) if (tp.getGenericElement() instanceof ExecutableElement && kept(tp)) kept.add(tp);
    }
    return kept;
  }

  /** Class type parameters that are not Go type parameters but stand for their bound (fixed). */
  Set<TypeParameterElement> fixedVars() {
    Set<TypeParameterElement> k = kept(), out = new LinkedHashSet<>();
    for (TypeParameterElement tp : pwhy.keySet()) if (!k.contains(tp) && fixed(tp)) out.add(tp);
    return out;
  }

  /** Declarations with at least one kept type parameter: Go generic types. */
  Set<TypeElement> typed() {
    Set<TypeParameterElement> k = kept();
    Set<TypeElement> out = new LinkedHashSet<>();
    why.forEach((te, w) -> {
      for (TypeParameterElement tp : te.getTypeParameters()) if (k.contains(tp)) out.add(te);
    });
    return out;
  }

  void report() {
    Set<TypeParameterElement> k = kept();
    why.forEach((te, w) -> {
      StringBuilder sb = new StringBuilder("generic " + (te.getKind().isInterface() ? "interface " : "class ") + te.getQualifiedName() + ": ");
      if (w != null) {
        sb.append(w);
      } else {
        List<String> ks = new ArrayList<>(), es = new ArrayList<>();
        for (TypeParameterElement tp : te.getTypeParameters()) {
          if (k.contains(tp)) ks.add(tp.getSimpleName().toString());
          else es.add(tp.getSimpleName() + " erased: " + pwhy.get(tp) + (fixed(tp) ? " (stands for its bound)" : ""));
        }
        sb.append(ks.isEmpty() ? "ERASED" : "TYPED " + ks);
        if (!es.isEmpty()) sb.append("; ").append(String.join("; ", es));
      }
      System.err.println(sb);
    });
  }
}
