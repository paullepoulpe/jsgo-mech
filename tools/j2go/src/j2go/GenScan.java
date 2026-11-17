package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Which type parameters of the program's generic classes and interfaces are Go type parameters
 * (Rule "typed generics"). A type parameter P of declaration D is kept when every mention of D in
 * the program names, at P's position, a type that one Go type stands for: a concrete type, or a
 * type variable that is kept too (fixpoint). Then an object made as D<..X..> is only ever seen
 * with X at that position, and D[X] is sound. A wildcard or a raw use at P's position, or a type
 * variable that stays erased, erases P alone: it is `any` in every instantiation of D, so all
 * views of one object still have one Go type.
 *
 * A declaration is left wholly erased when it is a record, a handle class or an inner class,
 * extends a class or is extended, holds local or anonymous classes, has generic instance methods,
 * or is tested with instanceof (Go has no test for "some instantiation of D"). A bounded type
 * parameter stays erased. Type variables of static generic methods of a declaration that is not
 * wholly erased are Go type parameters of the function (unless bounded).
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
      else if (te.getSuperclass().getKind() == TypeKind.DECLARED
          && !Ctx.typeElement(te.getSuperclass()).getQualifiedName().contentEquals("java.lang.Object")) w = "extends a class";
      else if (cx.handles.contains(te)) w = "a handle class";
      // Implementing an interface of the program is fine: a typed one is seen at the class's
      // view of it, an erased one keeps its erased signatures (converted at the method's entry).
      if (w == null) {
        for (Element e : te.getEnclosedElements()) {
          if (e instanceof ExecutableElement m && !m.getTypeParameters().isEmpty() && !m.getModifiers().contains(Modifier.STATIC)) w = "has generic instance methods (" + m.getSimpleName() + ")";
          if (e instanceof TypeElement n && !n.getModifiers().contains(Modifier.STATIC) && !n.getKind().isInterface()
              && n.getKind() != ElementKind.ENUM && n.getKind() != ElementKind.RECORD) w = "holds an inner class";
        }
      }
      if (w != null) why.put(te, w);
      for (TypeParameterElement tp : te.getTypeParameters()) pwhy.put(tp, bounded(tp) ? "bounded" : null);
    }
    for (TypeElement te : sources) {
      TypeElement s = te.getSuperclass().getKind() == TypeKind.DECLARED ? Ctx.typeElement(te.getSuperclass()) : null;
      if (s != null && why.containsKey(s) && why.get(s) == null) why.put(s, "extended by " + te.getSimpleName());
      if (te.getNestingKind() == NestingKind.LOCAL || te.getNestingKind() == NestingKind.ANONYMOUS) {
        for (Element o = te.getEnclosingElement(); o != null; o = o.getEnclosingElement()) {
          if (o instanceof TypeElement ot && why.containsKey(ot) && why.get(ot) == null) why.put(ot, "holds a local or anonymous class (in " + te.getEnclosingElement().getSimpleName() + ")");
        }
      }
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
              if (args.isEmpty()) erase(te, "used raw" + where());
              for (int i = 0; i < args.size() && i < ps.size(); i++) {
                TypeMirror a = args.get(i);
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
      public Void visitInstanceOf(InstanceOfTree t, Void v) {
        // `x instanceof C` asks about every instantiation: Go has no such test for a generic type
        Tree tt = t.getType() != null ? t.getType() : t.getPattern();
        if (tt != null) {
          TypeMirror tm = trees.getTypeMirror(new TreePath(getCurrentPath(), tt));
          TypeElement te = tm != null && tm.getKind() == TypeKind.DECLARED ? Ctx.typeElement(tm) : null;
          if (te != null && why.containsKey(te) && why.get(te) == null) why.put(te, "tested with instanceof" + where());
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

  /** A type variable that is a Go type parameter. */
  boolean kept(TypeParameterElement tp) {
    Element o = tp.getGenericElement();
    if (o instanceof TypeElement t) return why.containsKey(t) && why.get(t) == null && pwhy.get(tp) == null;
    // a static generic method of a declaration that keeps a type parameter: a generic function
    if (!(o instanceof ExecutableElement m && m.getModifiers().contains(Modifier.STATIC) && !bounded(tp)
        && m.getEnclosingElement() instanceof TypeElement ot && why.containsKey(ot) && why.get(ot) == null)) return false;
    for (TypeParameterElement cp : ((TypeElement) m.getEnclosingElement()).getTypeParameters()) if (pwhy.get(cp) == null) return true;
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
          if (!kept(o)) {
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
    return kept;
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
          else es.add(tp.getSimpleName() + " erased: " + pwhy.get(tp));
        }
        sb.append(ks.isEmpty() ? "ERASED" : "TYPED " + ks);
        if (!es.isEmpty()) sb.append("; ").append(String.join("; ", es));
      }
      System.err.println(sb);
    });
  }
}
