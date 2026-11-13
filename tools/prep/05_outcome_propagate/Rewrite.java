// Prep step 05 rewriter: turns an unchecked exception that is thrown and propagated through a
// call chain into returned Outcome values.
//
// Attributes java/src and java/test (one javac task, same flags as tools/gate0/build.sh), then
//   1. finds every `throw` whose expression has the given exception type; the enclosing methods
//      are the origins;
//   2. closes that set over "calls a method of the set" (callers), "is overridden by / overrides
//      a method of the set" (so a whole override family changes together); methods whose key
//      "pkg.Class#name(params)" matches the --exclude regex, constructors, and everything under
//      java/test are roots: they are never added to the set, and propagation stops at them;
//   3. rewrites each method M of the set:
//        - return type T -> Outcome<T> (void -> Outcome<Void>, primitives boxed);
//        - `throw X;` (X of the exception type) -> `return Outcome.err(X);`
//        - `return e;` -> `return Outcome.ok(e);`, `return;` -> `return Outcome.ok();`, unless e
//          is itself a call of a method of the set (then the outcome is returned as it is);
//        - a void M whose body does not end in a return gets a final `return Outcome.ok();`
//        - a call C of a method of the set that is
//            a whole statement             C;         -> Outcome<R> v = C; CHECK
//            a local variable initializer  T x = C;   -> Outcome<T> xOutcome = C; CHECK
//                                                        T x = xOutcome.get();
//            the value of an assignment    x = C;     -> Outcome<R> v = C; CHECK x = v.get();
//          where CHECK is `if (v.isErr()) { return Outcome.err(v.error()); }` and v is named
//          after the callee or the variable (numbered when the name is already used in M);
//   4. prints "MANUAL file:line reason" for everything it does not rewrite: a call of a method
//      of the set in any other position (argument, condition, lambda, ...), in a root method, or
//      inside a try statement with a catch clause that would have caught the exception; throws
//      inside lambdas; constructors that can throw.
// Lambda bodies and nested classes inside M are not touched (their returns are their own).
//
// Run from the repo root after tools/gate0/build.sh (JDK 21):
//   java tools/prep/05_outcome_propagate/Rewrite.java FlagUsageException [--exclude REGEX] [--apply]
// Without --apply it only prints the plan ("CONVERT ..." and "MANUAL ..." lines).

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.*;
import java.util.*;
import java.util.regex.Pattern;
import java.util.stream.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;
import javax.tools.*;

public final class Rewrite {
  record Edit(int start, int end, String text) {}

  static Trees trees;
  static javax.lang.model.util.Elements elements;
  static javax.lang.model.util.Types types;
  static String exc;

  static String key(Element e) {
    return e.getEnclosingElement() + "#" + e;
  }

  static boolean isTest(CompilationUnitTree u) {
    return u.getSourceFile().getName().startsWith("java/test/");
  }

  public static void main(String[] args) throws Exception {
    exc = args[0];
    Pattern exclude = null;
    boolean apply = false;
    for (int i = 1; i < args.length; i++) {
      if (args[i].equals("--exclude")) {
        exclude = Pattern.compile(args[++i]);
      } else if (args[i].equals("--apply")) {
        apply = true;
      }
    }
    List<File> files = new ArrayList<>();
    for (String root : new String[] {"java/src", "java/test"}) {
      try (Stream<Path> paths = Files.walk(Paths.get(root))) {
        paths.filter(p -> p.toString().endsWith(".java")).sorted().forEach(p -> files.add(p.toFile()));
      }
    }
    String deps;
    try (Stream<Path> jars = Files.list(Paths.get(".cache/deps"))) {
      deps = jars.map(Path::toString).filter(p -> p.endsWith(".jar")).sorted().collect(Collectors.joining(":"));
    }
    Path out = Files.createTempDirectory("rewrite05");
    JavaCompiler compiler = ToolProvider.getSystemJavaCompiler();
    StandardJavaFileManager fm = compiler.getStandardFileManager(null, null, StandardCharsets.UTF_8);
    JavacTask task =
        (JavacTask)
            compiler.getTask(
                null, fm, d -> {},
                List.of("-nowarn", "-Xlint:none", "-encoding", "UTF-8", "-parameters", "-proc:none",
                    "-d", out.toString(), "-cp", ".cache/build/stock-libs.jar:" + deps),
                null, fm.getJavaFileObjectsFromFiles(files));
    trees = Trees.instance(task);
    List<CompilationUnitTree> units = new ArrayList<>();
    for (CompilationUnitTree u : task.parse()) {
      units.add(u);
    }
    task.analyze();
    elements = task.getElements();
    types = task.getTypes();

    // Pass 1: call graph, override relation, origins.
    Map<String, Set<String>> callersOf = new HashMap<>();
    Map<String, Set<String>> family = new HashMap<>(); // method -> methods it overrides / is overridden by
    Set<String> origins = new LinkedHashSet<>();
    Set<String> roots = new HashSet<>();
    final Pattern excludeF = exclude;
    for (CompilationUnitTree unit : units) {
      new TreePathScanner<Void, Void>() {
        final Deque<String> stack = new ArrayDeque<>();

        @Override
        public Void visitMethod(MethodTree node, Void p) {
          Element e = trees.getElement(getCurrentPath());
          String k = key(e);
          if (isTest(unit) || e.getKind() == ElementKind.CONSTRUCTOR || (excludeF != null && excludeF.matcher(k).find())) {
            roots.add(k);
          }
          if (e instanceof ExecutableElement ee && e.getKind() == ElementKind.METHOD) {
            TypeElement cls = (TypeElement) e.getEnclosingElement();
            Deque<TypeMirror> q = new ArrayDeque<>(types.directSupertypes(cls.asType()));
            while (!q.isEmpty()) {
              TypeMirror s = q.pop();
              q.addAll(types.directSupertypes(s));
              for (Element m : ((DeclaredType) s).asElement().getEnclosedElements()) {
                if (m instanceof ExecutableElement me && elements.overrides(ee, me, cls)) {
                  family.computeIfAbsent(key(me), x -> new LinkedHashSet<>()).add(k);
                  family.computeIfAbsent(k, x -> new LinkedHashSet<>()).add(key(me));
                }
              }
            }
          }
          stack.push(k);
          super.visitMethod(node, p);
          stack.pop();
          return null;
        }

        void call() {
          Element e = trees.getElement(getCurrentPath());
          if (e != null && !stack.isEmpty()) {
            callersOf.computeIfAbsent(key(e), x -> new LinkedHashSet<>()).add(stack.peek());
          }
        }

        @Override
        public Void visitMethodInvocation(MethodInvocationTree node, Void p) {
          call();
          return super.visitMethodInvocation(node, p);
        }

        @Override
        public Void visitNewClass(NewClassTree node, Void p) {
          call();
          return super.visitNewClass(node, p);
        }

        @Override
        public Void visitMemberReference(MemberReferenceTree node, Void p) {
          call();
          return super.visitMemberReference(node, p);
        }

        @Override
        public Void visitThrow(ThrowTree node, Void p) {
          if (isExc(new TreePath(getCurrentPath(), node.getExpression())) && !stack.isEmpty()) {
            origins.add(stack.peek());
          }
          return super.visitThrow(node, p);
        }
      }.scan(unit, null);
    }
    Set<String> can = new LinkedHashSet<>();
    Deque<String> work = new ArrayDeque<>();
    for (String o : origins) {
      if (!roots.contains(o) && can.add(o)) {
        work.push(o);
      }
    }
    while (!work.isEmpty()) {
      String m = work.pop();
      List<String> next = new ArrayList<>(family.getOrDefault(m, Set.of()));
      next.addAll(callersOf.getOrDefault(m, Set.of()));
      for (String n : next) {
        if (!roots.contains(n) && can.add(n)) {
          work.push(n);
        }
      }
    }

    // Pass 2: edits.
    int[] counts = new int[4]; // methods, throws, call sites, manual
    for (CompilationUnitTree unit : units) {
      String src = unit.getSourceFile().getCharContent(true).toString();
      List<Edit> edits = new ArrayList<>();
      SourcePositions sp = trees.getSourcePositions();
      new TreePathScanner<Void, Void>() {
        String current = null; // key of the enclosing method of the set, when directly inside it
        String enclosing = null; // key of the enclosing method, whatever it is
        int barrier = 0; // > 0 inside a lambda or nested class of the method
        Set<String> names = new HashSet<>();
        final Deque<TryTree> tries = new ArrayDeque<>();

        String where(Tree t) {
          return unit.getSourceFile().getName() + ":" + unit.getLineMap().getLineNumber(sp.getStartPosition(unit, t));
        }

        void manual(Tree t, String why) {
          counts[3]++;
          System.out.println("MANUAL " + where(t) + " " + why + ": " + t.toString().replace('\n', ' ').substring(0, Math.min(90, t.toString().length())));
        }

        int start(Tree t) {
          return (int) sp.getStartPosition(unit, t);
        }

        int end(Tree t) {
          return (int) sp.getEndPosition(unit, t);
        }

        String indent(Tree t) {
          int s = start(t);
          int ls = src.lastIndexOf('\n', s - 1) + 1;
          int i = ls;
          while (src.charAt(i) == ' ') {
            i++;
          }
          return src.substring(ls, i);
        }

        String fresh(String base) {
          String n = base;
          int i = 2;
          while (!names.add(n)) {
            n = base + i++;
          }
          return n;
        }

        @Override
        public Void visitMethod(MethodTree node, Void p) {
          Element e = trees.getElement(getCurrentPath());
          String k = key(e);
          String savedCurrent = current;
          String savedEnclosing = enclosing;
          int savedBarrier = barrier;
          Set<String> savedNames = names;
          enclosing = k;
          barrier = 0;
          if (can.contains(k)) {
            current = k;
            names = new HashSet<>();
            collectNames(node);
            counts[0]++;
            System.out.println("CONVERT " + where(node) + " " + k);
            Tree rt = node.getReturnType();
            edits.add(new Edit(start(rt), end(rt), "Outcome<" + boxed(src.substring(start(rt), end(rt))) + ">"));
            BlockTree body = node.getBody();
            if (body != null && ((ExecutableElement) e).getReturnType().getKind() == TypeKind.VOID) {
              List<? extends StatementTree> st = body.getStatements();
              if (st.isEmpty() || !(st.get(st.size() - 1) instanceof ReturnTree || st.get(st.size() - 1) instanceof ThrowTree)) {
                int close = end(body) - 1;
                int ls = src.lastIndexOf('\n', close) + 1;
                edits.add(new Edit(ls, ls, indent(node) + "  return Outcome.ok();\n"));
              }
            }
          } else {
            current = null;
          }
          super.visitMethod(node, p);
          current = savedCurrent;
          enclosing = savedEnclosing;
          barrier = savedBarrier;
          names = savedNames;
          return null;
        }

        void collectNames(MethodTree node) {
          new TreeScanner<Void, Void>() {
            @Override
            public Void visitVariable(VariableTree v, Void p) {
              names.add(v.getName().toString());
              return super.visitVariable(v, p);
            }
          }.scan(node, null);
        }

        @Override
        public Void visitLambdaExpression(LambdaExpressionTree node, Void p) {
          barrier++;
          super.visitLambdaExpression(node, p);
          barrier--;
          return null;
        }

        @Override
        public Void visitClass(ClassTree node, Void p) {
          barrier++;
          super.visitClass(node, p);
          barrier--;
          return null;
        }

        @Override
        public Void visitTry(TryTree node, Void p) {
          scan(node.getResources(), p);
          tries.push(node);
          scan(node.getBlock(), p);
          tries.pop();
          scan(node.getCatches(), p);
          scan(node.getFinallyBlock(), p);
          return null;
        }

        boolean caught() {
          for (TryTree t : tries) {
            for (CatchTree c : t.getCatches()) {
              String s = c.getParameter().getType().toString();
              for (String alt : s.split("\\|")) {
                alt = alt.trim();
                if (alt.equals("Throwable") || alt.equals("Exception") || alt.equals("RuntimeException") || alt.endsWith(exc)) {
                  return true;
                }
              }
            }
          }
          return false;
        }

        @Override
        public Void visitReturn(ReturnTree node, Void p) {
          if (current != null && barrier == 0) {
            ExpressionTree x = node.getExpression();
            if (x == null) {
              edits.add(new Edit(end(node) - 1, end(node) - 1, " Outcome.ok()"));
            } else if (!isCanCall(new TreePath(getCurrentPath(), x))) {
              edits.add(new Edit(start(x), start(x), "Outcome.ok("));
              edits.add(new Edit(end(x), end(x), ")"));
            }
          }
          return super.visitReturn(node, p);
        }

        @Override
        public Void visitThrow(ThrowTree node, Void p) {
          if (isExc(new TreePath(getCurrentPath(), node.getExpression()))) {
            if (current != null && barrier == 0 && !caught()) {
              counts[1]++;
              ExpressionTree x = node.getExpression();
              edits.add(new Edit(start(node), start(x), "return Outcome.err("));
              edits.add(new Edit(end(x), end(x), ")"));
            } else if (!(current == null && barrier == 0 && tries.isEmpty() && roots.contains(enclosing))) {
              manual(node, "throw not rewritten");
            } else {
              System.out.println("KEPT-THROW " + where(node) + " in root " + enclosing);
            }
          }
          return super.visitThrow(node, p);
        }

        boolean isCanCall(TreePath path) {
          Tree t = path.getLeaf();
          if (!(t instanceof MethodInvocationTree) && !(t instanceof NewClassTree) && !(t instanceof MemberReferenceTree)) {
            return false;
          }
          Element e = trees.getElement(path);
          return e != null && can.contains(key(e));
        }

        void site(Tree node) {
          TreePath path = getCurrentPath();
          if (!isCanCall(path)) {
            return;
          }
          if (current == null || barrier > 0) {
            manual(node, current == null ? "call in a method outside the set (" + enclosing + ")" : "call inside a lambda or nested class");
            return;
          }
          if (caught()) {
            manual(node, "call inside a try that would have caught the exception");
            return;
          }
          Tree parent = path.getParentPath().getLeaf();
          Tree grand = path.getParentPath().getParentPath().getLeaf();
          String callee = trees.getElement(path).getSimpleName().toString();
          String type = simplify(trees.getTypeMirror(path));
          if (parent instanceof ReturnTree) {
            counts[2]++;
            return;
          }
          if (parent instanceof ExpressionStatementTree) {
            counts[2]++;
            String ind = indent(parent);
            String v = fresh(callee + "Outcome");
            edits.add(new Edit(start(parent), start(parent), "Outcome<" + type + "> " + v + " = "));
            edits.add(new Edit(end(parent), end(parent), "\n" + check(ind, v)));
            return;
          }
          if (parent instanceof VariableTree var && var.getInitializer() == node && (grand instanceof BlockTree || grand instanceof CaseTree)) {
            counts[2]++;
            String ind = indent(parent);
            String v = fresh(var.getName() + "Outcome");
            String decl = src.substring(start(parent), start(node)); // "final T x = "
            edits.add(new Edit(start(parent), start(node), "Outcome<" + boxed(src.substring(start(var.getType()), end(var.getType()))) + "> " + v + " = "));
            edits.add(new Edit(end(parent), end(parent), "\n" + check(ind, v) + "\n" + ind + decl + v + ".get();"));
            return;
          }
          if (parent instanceof AssignmentTree asg && asg.getExpression() == node && grand instanceof ExpressionStatementTree) {
            counts[2]++;
            String ind = indent(grand);
            String lhs = src.substring(start(asg.getVariable()), end(asg.getVariable()));
            String v = fresh(lhs.substring(lhs.lastIndexOf('.') + 1) + "Outcome");
            edits.add(new Edit(start(grand), start(node), "Outcome<" + type + "> " + v + " = "));
            edits.add(new Edit(end(grand), end(grand), "\n" + check(ind, v) + "\n" + ind + lhs + " = " + v + ".get();"));
            return;
          }
          manual(node, "call in an expression position");
        }

        @Override
        public Void visitMethodInvocation(MethodInvocationTree node, Void p) {
          site(node);
          return super.visitMethodInvocation(node, p);
        }

        @Override
        public Void visitNewClass(NewClassTree node, Void p) {
          site(node);
          return super.visitNewClass(node, p);
        }

        @Override
        public Void visitMemberReference(MemberReferenceTree node, Void p) {
          site(node);
          return super.visitMemberReference(node, p);
        }
      }.scan(unit, null);
      if (!edits.isEmpty() && apply) {
        edits.sort(Comparator.comparingInt((Edit e) -> e.start).thenComparingInt(e -> e.end));
        StringBuilder sb = new StringBuilder();
        int pos = 0;
        for (Edit e : edits) {
          if (e.start < pos) {
            throw new IllegalStateException("overlapping edits in " + unit.getSourceFile().getName() + " at " + e.start);
          }
          sb.append(src, pos, e.start).append(e.text);
          pos = e.end;
        }
        sb.append(src.substring(pos));
        String result = sb.toString();
        if (!result.contains("import com.google.javascript.jscomp.base.Outcome;")) {
          System.out.println("MANUAL " + unit.getSourceFile().getName() + " needs the Outcome import");
        }
        Files.writeString(Paths.get(unit.getSourceFile().getName()), result);
      }
    }
    System.out.println("methods: " + counts[0] + ", throw sites: " + counts[1] + ", call sites: " + counts[2] + ", manual: " + counts[3]);
  }

  static String check(String ind, String v) {
    return ind + "if (" + v + ".isErr()) {\n" + ind + "  return Outcome.err(" + v + ".error());\n" + ind + "}";
  }

  static boolean isExc(TreePath path) {
    TypeMirror m = trees.getTypeMirror(path);
    return m != null && (m.toString().equals(exc) || m.toString().endsWith("." + exc));
  }

  static String boxed(String t) {
    return switch (t) {
      case "void" -> "Void";
      case "int" -> "Integer";
      case "boolean" -> "Boolean";
      case "long" -> "Long";
      case "double" -> "Double";
      case "char" -> "Character";
      default -> t;
    };
  }

  static String simplify(TypeMirror m) {
    if (m.getKind() == TypeKind.VOID) {
      return "Void";
    }
    if (m.getKind().isPrimitive()) {
      return boxed(m.toString());
    }
    return m.toString().replaceAll("\\b[a-z_]\\w*(\\.[a-z_]\\w*)*\\.(?=[A-Z])", "").replace(",", ", ");
  }
}
