package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.io.*;
import java.nio.file.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.tools.*;

/**
 * Translates Java sources to Go.
 *
 * <pre>java -cp ... j2go.Main &lt;classpath&gt; &lt;outdir&gt; &lt;init-order-file or -&gt; @files.txt</pre>
 */
public final class Main {
  /**
   * A handle is a number, not an object: code may compare handles with == and hash them by identity,
   * but it cannot lock one, wait on one, clone one bitwise or hold one weakly.
   */
  static void checkHandleUses(Ctx cx, Trees trees, CompilationUnitTree cu) {
    new TreePathScanner<Void, Void>() {
      javax.lang.model.type.TypeMirror typeOf(Tree t) {
        try {
          return trees.getTypeMirror(new TreePath(getCurrentPath(), t));
        } catch (RuntimeException ex) {
          return null;
        }
      }

      void fail(Tree t, TypeElement te, String what) {
        long pos = trees.getSourcePositions().getStartPosition(cu, t);
        cx.handleErrors.add(te.getQualifiedName() + ": " + what + " at " + cu.getSourceFile().getName() + ":" + (pos < 0 ? -1 : cu.getLineMap().getLineNumber(pos)));
      }

      TypeElement handleOf(Tree t) {
        javax.lang.model.type.TypeMirror tm = typeOf(t);
        return tm != null && cx.isHandleType(tm) ? Ctx.typeElement(cx.erase(tm)) : null;
      }

      TypeElement enclosingHandle() {
        for (TreePath p = getCurrentPath(); p != null; p = p.getParentPath()) {
          if (p.getLeaf() instanceof ClassTree) return trees.getElement(p) instanceof TypeElement te && cx.isHandle(te) ? te : null;
        }
        return null;
      }

      @Override
      public Void visitSynchronized(SynchronizedTree t, Void v) {
        TypeElement te = handleOf(t.getExpression());
        if (te != null) fail(t, te, "synchronized on it");
        return super.visitSynchronized(t, v);
      }

      @Override
      public Void visitMethod(MethodTree t, Void v) {
        if (t.getModifiers().getFlags().contains(Modifier.SYNCHRONIZED) && !t.getModifiers().getFlags().contains(Modifier.STATIC)) {
          TypeElement te = enclosingHandle();
          if (te != null) fail(t, te, "synchronized instance method");
        }
        return super.visitMethod(t, v);
      }

      @Override
      public Void visitMethodInvocation(MethodInvocationTree t, Void v) {
        Element e = trees.getElement(new TreePath(getCurrentPath(), t.getMethodSelect()));
        if (e instanceof ExecutableElement m && !m.getModifiers().contains(Modifier.STATIC)
            && ((TypeElement) m.getEnclosingElement()).getQualifiedName().contentEquals("java.lang.Object")
            && Set.of("wait", "notify", "notifyAll", "clone", "finalize").contains(m.getSimpleName().toString())) {
          TypeElement te = t.getMethodSelect() instanceof MemberSelectTree ms && !ms.getExpression().toString().equals("super")
              ? handleOf(ms.getExpression()) : enclosingHandle();
          if (te != null) fail(t, te, "Object." + m.getSimpleName() + "() on it");
        }
        return super.visitMethodInvocation(t, v);
      }

      @Override
      public Void visitNewClass(NewClassTree t, Void v) {
        javax.lang.model.type.TypeMirror tm = typeOf(t);
        TypeElement made = tm == null ? null : Ctx.typeElement(cx.erase(tm));
        if (made != null && (made.getQualifiedName().toString().startsWith("java.lang.ref.") || made.getQualifiedName().contentEquals("java.util.WeakHashMap"))) {
          for (ExpressionTree arg : t.getArguments()) {
            TypeElement te = handleOf(arg);
            if (te != null) fail(t, te, "held by a " + made.getSimpleName());
          }
          if (tm instanceof javax.lang.model.type.DeclaredType dt) {
            for (javax.lang.model.type.TypeMirror ta : dt.getTypeArguments()) {
              if (cx.isHandleType(ta)) fail(t, Ctx.typeElement(cx.erase(ta)), "held by a " + made.getSimpleName());
            }
          }
        }
        return super.visitNewClass(t, v);
      }
    }.scan(cu, null);
  }

  public static void main(String[] a) throws Exception {
    JavaCompiler jc = ToolProvider.getSystemJavaCompiler();
    StandardJavaFileManager fm = jc.getStandardFileManager(null, null, null);
    List<File> files = new ArrayList<>();
    for (int i = 3; i < a.length; i++) {
      if (a[i].startsWith("@")) for (String l : Files.readAllLines(Path.of(a[i].substring(1)))) files.add(new File(l));
      else files.add(new File(a[i]));
    }
    Path out = Path.of(a[1]);
    DiagnosticCollector<JavaFileObject> diags = new DiagnosticCollector<>();
    JavacTask task = (JavacTask) jc.getTask(null, fm, diags,
        List.of("-proc:none", "-cp", a[0], "--release", "21", "-Xmaxerrs", "100000", "-parameters"), null,
        fm.getJavaFileObjectsFromFiles(files));
    List<CompilationUnitTree> units = new ArrayList<>();
    for (CompilationUnitTree cu : task.parse()) units.add(cu);
    task.analyze();
    long errs = diags.getDiagnostics().stream().filter(d -> d.getKind() == Diagnostic.Kind.ERROR).count();
    if (errs > 0) {
      diags.getDiagnostics().stream().filter(d -> d.getKind() == Diagnostic.Kind.ERROR).limit(20).forEach(System.err::println);
      System.err.println("javac errors: " + errs);
      System.exit(1);
    }
    Trees trees = Trees.instance(task);
    List<TypeElement> preload = new ArrayList<>();
    Ctx cx = new Ctx(trees, task.getTypes(), task.getElements());
    // Pass 1: every class, and whether local classes have an enclosing instance.
    for (CompilationUnitTree cu : units) {
      new TreePathScanner<Void, Void>() {
        @Override
        public Void visitClass(ClassTree t, Void v) {
          Element e = trees.getElement(getCurrentPath());
          if (e instanceof TypeElement te) {
            cx.registerClass(te);
            if (te.getNestingKind() == NestingKind.LOCAL || te.getNestingKind() == NestingKind.ANONYMOUS) {
              boolean instance = true, decided = false;
              for (TreePath p = getCurrentPath().getParentPath(); p != null; p = p.getParentPath()) {
                Tree n = p.getLeaf();
                if (n instanceof ClassTree) {
                  Element oe = trees.getElement(p);
                  if (oe instanceof TypeElement ot) {
                    cx.lexicalOuter.put(te, ot);
                    if (ot.getKind().isInterface()) instance = decided && instance;
                  }
                  break;
                }
                if (decided) continue;
                if (n instanceof MethodTree m) {
                  instance = !m.getModifiers().getFlags().contains(Modifier.STATIC);
                  decided = true;
                } else if (n instanceof VariableTree f && p.getParentPath().getLeaf() instanceof ClassTree) {
                  Element fe = trees.getElement(p);
                  instance = fe != null && !fe.getModifiers().contains(Modifier.STATIC);
                  decided = true;
                } else if (n instanceof BlockTree b && p.getParentPath().getLeaf() instanceof ClassTree) {
                  instance = !b.isStatic();
                  decided = true;
                }
              }
              if (instance) cx.localWithOuter.add(te);
            }
          }
          return super.visitClass(t, v);
        }
      }.scan(cu, null);
    }
    // Pass 2: the library methods in use, so that their Go names can tell overloads apart.
    for (CompilationUnitTree cu : units) {
      // The translator's own test programs must not change which overload is the most used.
      final boolean counted = !cu.getSourceFile().getName().contains("/selftest/");
      new TreePathScanner<Void, Void>() {
        void note(Tree t) {
          Element e = trees.getElement(new TreePath(getCurrentPath(), t));
          if (e instanceof ExecutableElement m) {
            cx.noteExtRoot(m);
            if (counted) cx.countUse(m);
          }
        }

        @Override
        public Void scan(Tree t, Void v) {
          if (t instanceof ExpressionTree) {
            try {
              javax.lang.model.type.TypeMirror tm = trees.getTypeMirror(new TreePath(getCurrentPath(), t));
              TypeElement te = tm == null ? null : Ctx.typeElement(cx.erase(tm));
              if (te != null) cx.seenTypes.add(te);
            } catch (RuntimeException ex) {
              // no type recorded for this tree
            }
          }
          return super.scan(t, v);
        }

        @Override
        public Void visitNewClass(NewClassTree t, Void v) {
          Element e = trees.getElement(getCurrentPath());
          if (e instanceof ExecutableElement m && counted) cx.countUse(m);
          return super.visitNewClass(t, v);
        }

        @Override
        public Void visitMethodInvocation(MethodInvocationTree t, Void v) {
          note(t.getMethodSelect());
          return super.visitMethodInvocation(t, v);
        }

        @Override
        public Void visitMemberReference(MemberReferenceTree t, Void v) {
          Element e = trees.getElement(getCurrentPath());
          if (e instanceof ExecutableElement m) {
            cx.noteExtRoot(m);
            if (counted) cx.countUse(m);
          }
          return super.visitMemberReference(t, v);
        }
      }.scan(cu, null);
    }
    for (String qn : List.of("java.lang.NullPointerException", "java.lang.ClassCastException", "java.lang.ArithmeticException",
        "java.lang.IndexOutOfBoundsException", "java.lang.ArrayIndexOutOfBoundsException", "java.lang.RuntimeException",
        "java.lang.IllegalArgumentException", "java.lang.IllegalStateException", "java.lang.UnsupportedOperationException",
        "java.lang.NumberFormatException", "java.util.NoSuchElementException", "java.io.IOException", "java.lang.StackOverflowError",
        "java.lang.Error", "java.lang.Exception", "java.lang.Throwable", "java.lang.Enum", "java.lang.Class",
        "java.lang.StringBuilder", "java.io.FileNotFoundException", "java.io.UncheckedIOException",
        "java.util.ConcurrentModificationException", "java.lang.NegativeArraySizeException", "java.lang.CloneNotSupportedException")) {
      preload.add(task.getElements().getTypeElement(qn));
    }
    cx.registerSourcesDone();
    // Handle classes: listed in the files named by J2GO_HANDLES (default tools/j2go/handles.txt).
    List<String> handleNames = new ArrayList<>();
    for (String hf : System.getenv().getOrDefault("J2GO_HANDLES", "tools/j2go/handles.txt").split(":")) {
      if (hf.isEmpty() || !Files.exists(Path.of(hf))) continue;
      for (String l : Files.readAllLines(Path.of(hf))) {
        l = l.replaceAll("#.*", "").trim();
        if (!l.isEmpty()) handleNames.add(l);
      }
    }
    cx.registerHandles(handleNames);
    if (!cx.handles.isEmpty()) {
      for (CompilationUnitTree cu : units) checkHandleUses(cx, trees, cu);
    }
    if (!cx.handleErrors.isEmpty()) {
      for (String e : cx.handleErrors) System.err.println("handle class " + e);
      System.exit(1);
    }
    cx.assignClassNames();
    cx.analyzeMethods();
    Files.createDirectories(out);
    try (var old = Files.list(out)) {
      for (Path p : (Iterable<Path>) old::iterator) {
        if (p.getFileName().toString().startsWith("g_")) Files.delete(p);
      }
    }
    List<String> errors = new ArrayList<>();
    for (CompilationUnitTree cu : units) {
      Emitter em = new Emitter(cx, cu);
      String code;
      try {
        code = em.emitUnit();
      } catch (RuntimeException ex) {
        StringWriter sw = new StringWriter();
        ex.printStackTrace(new PrintWriter(sw));
        errors.add(cu.getSourceFile().getName() + ": FATAL " + sw.toString().lines().limit(6).reduce("", (x, y) -> x + " | " + y));
        continue;
      }
      errors.addAll(em.errors);
      String name = "g_" + new File(cu.getSourceFile().getName()).getName().replace(".java", "");
      String pkg = cu.getPackageName() == null ? "" : cu.getPackageName().toString();
      Path f = out.resolve(name + ".go");
      if (Files.exists(f)) f = out.resolve(name + "_" + pkg.substring(pkg.lastIndexOf('.') + 1) + ".go");
      Files.writeString(f, "// Code generated by j2go from " + pkg.replace('.', '/') + "/" + new File(cu.getSourceFile().getName()).getName() + ". DO NOT EDIT.\n\n" + code);
    }
    // Static initialization, in the order the Java program was observed to initialize classes.
    List<String> order = new ArrayList<>();
    Map<String, String> byOwner = new HashMap<>();
    cx.clinitOwner.forEach((fn, owner) -> byOwner.put(owner, fn));
    Set<String> done = new HashSet<>();
    if (!a[2].equals("-")) {
      for (String l : Files.readAllLines(Path.of(a[2]))) {
        String fn = byOwner.get(l.trim());
        if (fn != null && done.add(fn)) order.add(fn);
      }
    }
    for (String fn : cx.clinits) if (done.add(fn)) order.add(fn);
    StringBuilder ib = new StringBuilder("// Code generated by j2go. DO NOT EDIT.\n\npackage closure\n\nfunc init() {\n");
    for (String fn : order) ib.append("\trunClinit_(").append(fn).append(")\n");
    ib.append("}\n");
    Files.writeString(out.resolve("g_0init.go"), ib.toString());
    for (TypeElement te : preload) cx.className(te);
    Files.writeString(out.resolve("g_zstubs.go"), new Stubs(cx, out).generate());
    // Casts to interface types (see Ctx.coerce).
    StringBuilder cb = new StringBuilder("// Code generated by j2go. DO NOT EDIT.\n\npackage closure\n");
    for (String gt : cx.icasts) {
      cb.append("\nfunc icast_").append(gt).append("(v any) ").append(gt).append(" {\n\tif v == nil {\n\t\treturn nil\n\t}\n\treturn v.(")
          .append(gt).append(")\n}\n");
    }
    Files.writeString(out.resolve("g_0casts.go"), cb.toString());
    // Java names of the Go types, for getClass().
    StringBuilder nb = new StringBuilder("// Code generated by j2go. DO NOT EDIT.\n\npackage closure\n\nvar javaNames_ = map[string]string{\n");
    TreeMap<String, String> jn = new TreeMap<>();
    cx.classNames.forEach((te, n) -> jn.put(n, cx.elems.getBinaryName(te).toString()));
    jn.forEach((n, q) -> nb.append("\t\"").append(n).append("\": \"").append(q).append("\",\n"));
    nb.append("}\n");
    Files.writeString(out.resolve("g_0names.go"), nb.toString());
    StringBuilder xb = new StringBuilder();
    cx.externals.forEach((k, v) -> xb.append(k).append("\t").append(v).append("\n"));
    Files.writeString(out.resolve("externals.tsv"), xb.toString());
    Files.writeString(out.resolve("j2go-errors.txt"), String.join("\n", errors) + "\n");
    System.err.println("units=" + units.size() + " classes=" + cx.source.size() + " externals=" + cx.externals.size() + " unsupported=" + errors.size());
  }
}
