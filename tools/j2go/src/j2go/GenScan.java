package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Which generic classes of the translated program become Go generic types (Rule "typed generic
 * classes"): a class whose every mention in the program, inside and outside its body, names
 * type arguments that one Go type each stands for, so that an object made as C[X] is only ever
 * seen as C[X]. A class is kept erased when it is extended or extends a class, is an interface,
 * has a bounded type parameter, holds non-static inner, local or anonymous classes (they would
 * need its type parameters), has generic methods of its own, or is mentioned raw, with a
 * wildcard or with a type variable other than its own inside its own body.
 */
final class GenScan {
  /** Candidate classes and, for those left erased, why. */
  final Map<TypeElement, String> why = new LinkedHashMap<>();

  final Ctx cx;
  final Trees trees;

  GenScan(Ctx cx, Trees trees) {
    this.cx = cx;
    this.trees = trees;
  }

  void candidates(Collection<TypeElement> sources) {
    for (TypeElement te : sources) {
      if (te.getTypeParameters().isEmpty()) continue;
      why.put(te, null);
      String w = null;
      if (te.getKind() != ElementKind.CLASS) w = "an interface or record";
      else if (te.getNestingKind() != NestingKind.TOP_LEVEL && !te.getModifiers().contains(Modifier.STATIC)) w = "an inner class";
      else if (te.getSuperclass().getKind() == TypeKind.DECLARED
          && !Ctx.typeElement(te.getSuperclass()).getQualifiedName().contentEquals("java.lang.Object")) w = "extends a class";
      else if (cx.handles.contains(te)) w = "a handle class";
      if (w == null) {
        for (TypeParameterElement tp : te.getTypeParameters()) {
          for (TypeMirror b : tp.getBounds()) {
            if (!(b instanceof DeclaredType d && Ctx.typeElement(d).getQualifiedName().contentEquals("java.lang.Object"))) w = "a bounded type parameter";
          }
        }
      }
      if (w == null) {
        for (TypeMirror i : te.getInterfaces()) {
          if (cx.isSource(Ctx.typeElement(i))) w = "implements an interface of the program";
        }
      }
      if (w == null) {
        for (Element e : te.getEnclosedElements()) {
          if (e instanceof ExecutableElement m && !m.getTypeParameters().isEmpty() && !m.getModifiers().contains(Modifier.STATIC)) w = "has generic instance methods";
          if (e instanceof TypeElement n && !n.getModifiers().contains(Modifier.STATIC) && !n.getKind().isInterface()
              && n.getKind() != ElementKind.ENUM && n.getKind() != ElementKind.RECORD) w = "holds an inner class";
        }
      }
      if (w != null) why.put(te, w);
    }
    for (TypeElement te : sources) {
      TypeElement s = te.getSuperclass().getKind() == TypeKind.DECLARED ? Ctx.typeElement(te.getSuperclass()) : null;
      if (s != null && why.containsKey(s) && why.get(s) == null) why.put(s, "extended by " + te.getSimpleName());
      if (te.getNestingKind() == NestingKind.LOCAL || te.getNestingKind() == NestingKind.ANONYMOUS) {
        for (Element o = te.getEnclosingElement(); o != null; o = o.getEnclosingElement()) {
          if (o instanceof TypeElement ot && why.containsKey(ot) && why.get(ot) == null) why.put(ot, "holds a local or anonymous class");
        }
      }
    }
  }

  void scan(CompilationUnitTree cu) {
    new TreePathScanner<Void, Void>() {
      TypeElement ownerTop() {
        for (TreePath p = getCurrentPath(); p != null; p = p.getParentPath()) {
          if (p.getLeaf() instanceof ClassTree && trees.getElement(p) instanceof TypeElement te && why.containsKey(te)) return te;
        }
        return null;
      }

      void check(TypeMirror t, TypeElement in, int depth) {
        if (t == null || depth > 8) return;
        switch (t.getKind()) {
          case ARRAY -> check(((ArrayType) t).getComponentType(), in, depth + 1);
          case DECLARED -> {
            DeclaredType d = (DeclaredType) t;
            TypeElement te = Ctx.typeElement(d);
            List<? extends TypeMirror> args = d.getTypeArguments();
            if (why.containsKey(te) && why.get(te) == null) {
              if (args.isEmpty()) why.put(te, "used raw");
              for (TypeMirror a : args) {
                if (a.getKind() == TypeKind.WILDCARD) why.put(te, "used with a wildcard");
                else if (a.getKind() == TypeKind.TYPEVAR) {
                  Element owner = ((TypeVariable) a).asElement().getEnclosingElement();
                  // its own type variables in its body, and those of its static generic factories
                  boolean factory = owner instanceof ExecutableElement m && m.getEnclosingElement() == in;
                  if (owner != in && !factory) why.put(te, "used with a type variable other than its own: " + a + " in " + owner);
                } else if (a.getKind() == TypeKind.INTERSECTION || a.getKind() == TypeKind.UNION) why.put(te, "used with an intersection type");
              }
            }
            for (TypeMirror a : args) check(a, in, depth + 1);
          }
          case WILDCARD -> {
            WildcardType w = (WildcardType) t;
            check(w.getExtendsBound(), in, depth + 1);
            check(w.getSuperBound(), in, depth + 1);
          }
          default -> {
          }
        }
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
          if (tm != null && !(t instanceof ClassTree) && !typeName) check(tm, ownerTop(), 0);
        }
        return super.scan(t, v);
      }

      @Override
      public Void visitMethod(MethodTree t, Void v) {
        if (trees.getElement(getCurrentPath()) instanceof ExecutableElement m) {
          TypeElement in = ownerTop();
          check(m.getReturnType(), in, 0);
          for (VariableElement p : m.getParameters()) check(p.asType(), in, 0);
        }
        return super.visitMethod(t, v);
      }

      @Override
      public Void visitClass(ClassTree t, Void v) {
        if (trees.getElement(getCurrentPath()) instanceof TypeElement te) {
          TypeElement in = ownerTop();
          check(te.getSuperclass(), in, 0);
          for (TypeMirror i : te.getInterfaces()) check(i, in, 0);
        }
        return super.visitClass(t, v);
      }
    }.scan(cu, null);
  }

  Set<TypeElement> typed() {
    Set<TypeElement> out = new LinkedHashSet<>();
    why.forEach((te, w) -> {
      if (w == null) out.add(te);
    });
    return out;
  }

  void report() {
    why.forEach((te, w) -> System.err.println("generic class " + te.getQualifiedName() + ": " + (w == null ? "TYPED" : w)));
  }
}
