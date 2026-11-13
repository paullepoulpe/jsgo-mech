// Prep step 05 scanner: ignored Outcome values.
//
// Attributes java/src and java/test (one javac task, same flags as tools/gate0/build.sh) and
// prints "file:line: ignored Outcome: <expression>" for every expression statement whose
// expression has the type com.google.javascript.jscomp.base.Outcome and is not an assignment, i.e.
// every call of a method returning Outcome whose result is dropped, and "file:line: Outcome lambda body: <expression>"
// for every expression-bodied lambda whose body has that type while the lambda's functional
// interface method returns void (e.g. assertThrows(X.class, () -> f())): there the outcome is
// dropped too, which is intended only when the test expects f() to throw before returning.
// Exit status 1 if an ignored Outcome statement exists (lambda bodies are informational).
//
// Run from the repo root after tools/gate0/build.sh (JDK 21):
//   java tools/prep/05_find_ignored_outcomes/Scan.java
//   java tools/prep/05_find_ignored_outcomes/Scan.java --ends   # also "END<TAB>file<TAB>offset"
//       lines with the end offset of each ignored expression (used to append ".get()" in tests)

import com.sun.source.tree.AssignmentTree;
import com.sun.source.tree.CompilationUnitTree;
import com.sun.source.tree.ExpressionStatementTree;
import com.sun.source.tree.LambdaExpressionTree;
import com.sun.source.tree.Tree;
import com.sun.source.util.JavacTask;
import com.sun.source.util.TreePath;
import com.sun.source.util.TreePathScanner;
import com.sun.source.util.Trees;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import javax.lang.model.element.ExecutableElement;
import javax.lang.model.type.DeclaredType;
import javax.lang.model.type.TypeKind;
import javax.lang.model.type.TypeMirror;
import javax.tools.Diagnostic;
import javax.tools.JavaCompiler;
import javax.tools.StandardJavaFileManager;
import javax.tools.ToolProvider;

public final class Scan {
  static final String OUTCOME = "com.google.javascript.jscomp.base.Outcome";

  public static void main(String[] args) throws Exception {
    List<File> files = new ArrayList<>();
    for (String root : new String[] {"java/src", "java/test"}) {
      try (Stream<Path> paths = Files.walk(Paths.get(root))) {
        paths.filter(p -> p.toString().endsWith(".java")).sorted().forEach(p -> files.add(p.toFile()));
      }
    }
    String deps;
    try (Stream<Path> jars = Files.list(Paths.get(".cache/deps"))) {
      deps =
          jars.map(Path::toString).filter(p -> p.endsWith(".jar")).sorted().collect(Collectors.joining(":"));
    }
    Path out = Files.createTempDirectory("scan05o");
    JavaCompiler compiler = ToolProvider.getSystemJavaCompiler();
    StandardJavaFileManager fm = compiler.getStandardFileManager(null, null, StandardCharsets.UTF_8);
    List<String> errors = new ArrayList<>();
    JavacTask task =
        (JavacTask)
            compiler.getTask(
                null,
                fm,
                d -> {
                  if (d.getKind() == Diagnostic.Kind.ERROR) {
                    errors.add(d.toString());
                  }
                },
                List.of(
                    "-nowarn", "-Xlint:none", "-encoding", "UTF-8", "-parameters", "-proc:none",
                    "-d", out.toString(), "-cp", ".cache/build/stock-libs.jar:" + deps),
                null,
                fm.getJavaFileObjectsFromFiles(files));
    Trees trees = Trees.instance(task);
    List<CompilationUnitTree> units = new ArrayList<>();
    for (CompilationUnitTree u : task.parse()) {
      units.add(u);
    }
    task.analyze();
    if (!errors.isEmpty()) {
      for (String e : errors) {
        System.err.println(e);
      }
      System.exit(2);
    }
    int[] counts = new int[2];
    for (CompilationUnitTree unit : units) {
      new TreePathScanner<Void, Void>() {
        boolean isOutcome(Tree t) {
          TypeMirror m = trees.getTypeMirror(new TreePath(getCurrentPath(), t));
          return m != null
              && m.getKind() == TypeKind.DECLARED
              && ((DeclaredType) m).asElement().toString().equals(OUTCOME);
        }

        String where(Tree t) {
          long pos = trees.getSourcePositions().getStartPosition(unit, t);
          return unit.getSourceFile().getName() + ":" + unit.getLineMap().getLineNumber(pos);
        }

        String text(Tree t) {
          String s = t.toString().replace('\n', ' ');
          return s.length() > 100 ? s.substring(0, 100) + "..." : s;
        }

        @Override
        public Void visitExpressionStatement(ExpressionStatementTree node, Void p) {
          if (!(node.getExpression() instanceof AssignmentTree) && isOutcome(node.getExpression())) {
            counts[0]++;
            System.out.println(where(node) + ": ignored Outcome: " + text(node.getExpression()));
            if (args.length > 0 && args[0].equals("--ends")) {
              System.out.println(
                  "END\t"
                      + unit.getSourceFile().getName()
                      + "\t"
                      + trees.getSourcePositions().getEndPosition(unit, node.getExpression()));
            }
          }
          return super.visitExpressionStatement(node, p);
        }

        @Override
        public Void visitLambdaExpression(LambdaExpressionTree node, Void p) {
          if (node.getBodyKind() == LambdaExpressionTree.BodyKind.EXPRESSION
              && isOutcome(node.getBody())) {
            TypeMirror fi = trees.getTypeMirror(getCurrentPath());
            boolean returnsVoid = false;
            if (fi != null && fi.getKind() == TypeKind.DECLARED) {
              for (javax.lang.model.element.Element e :
                  task.getElements().getAllMembers(
                      (javax.lang.model.element.TypeElement) ((DeclaredType) fi).asElement())) {
                if (e instanceof ExecutableElement x
                    && x.getModifiers().contains(javax.lang.model.element.Modifier.ABSTRACT)
                    && x.getReturnType().getKind() == TypeKind.VOID) {
                  returnsVoid = true;
                }
              }
            }
            if (returnsVoid) {
              counts[1]++;
              System.out.println(where(node) + ": Outcome lambda body: " + text(node.getBody()));
            }
          }
          return super.visitLambdaExpression(node, p);
        }
      }.scan(unit, null);
    }
    System.out.println(
        "ignored Outcome statements: " + counts[0] + ", void lambdas with an Outcome body: " + counts[1]);
    System.exit(counts[0] == 0 ? 0 : 1);
  }
}
