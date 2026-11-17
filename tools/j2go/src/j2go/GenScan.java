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

  void erase(TypeParameterElement tp, String w) {
    if (pwhy.containsKey(tp) && pwhy.get(tp) == null) pwhy.put(tp, w);
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
          TreePath p = new TreePath(getCurrentPath(), t);
          TypeMirror tm = null;
          try {
            tm = trees.getTypeMirror(p);
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
        if (tm != null && tm.getKind() == TypeKind.ARRAY && !(x instanceof NewArrayTree)) eraseIn(((ArrayType) tm).getComponentType(), "a cast to an array" + where(), 0);
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
