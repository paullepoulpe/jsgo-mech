// Prep step 09 scanner: prints file:line:kind for every assignment / compound assignment / ++ / --
// that is not a statement of its own (expression statement or for-loop init/update); exit 1 if any.
// Run (JDK 21, parse-only): javac -d /tmp/scan09 tools/prep/09_find_embedded_side_effects/Scan.java && java -cp /tmp/scan09 Scan java/src java/test

import com.sun.source.tree.AnnotationTree;
import com.sun.source.tree.AssignmentTree;
import com.sun.source.tree.CompilationUnitTree;
import com.sun.source.tree.CompoundAssignmentTree;
import com.sun.source.tree.ExpressionStatementTree;
import com.sun.source.tree.LambdaExpressionTree;
import com.sun.source.tree.ParenthesizedTree;
import com.sun.source.tree.Tree;
import com.sun.source.tree.UnaryTree;
import com.sun.source.util.JavacTask;
import com.sun.source.util.SourcePositions;
import com.sun.source.util.TreePath;
import com.sun.source.util.TreePathScanner;
import com.sun.source.util.Trees;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;
import java.util.TreeSet;
import java.util.stream.Stream;
import javax.tools.JavaCompiler;
import javax.tools.JavaFileObject;
import javax.tools.StandardJavaFileManager;
import javax.tools.ToolProvider;

public final class Scan {
  public static void main(String[] args) throws Exception {
    int total = 0;
    for (String root : args) {
      total += scanRoot(root);
    }
    System.exit(total == 0 ? 0 : 1);
  }

  private static int scanRoot(String root) throws Exception {
    List<File> files = new ArrayList<>();
    try (Stream<Path> paths = Files.walk(Paths.get(root))) {
      paths
          .filter(p -> p.toString().endsWith(".java"))
          .sorted()
          .forEach(p -> files.add(p.toFile()));
    }
    JavaCompiler compiler = ToolProvider.getSystemJavaCompiler();
    StandardJavaFileManager fm = compiler.getStandardFileManager(null, null, null);
    Iterable<? extends JavaFileObject> units = fm.getJavaFileObjectsFromFiles(files);
    JavacTask task =
        (JavacTask) compiler.getTask(null, fm, d -> {}, List.of("-proc:none"), null, units);
    Trees trees = Trees.instance(task);
    SourcePositions positions = trees.getSourcePositions();
    int[] count = new int[1];
    TreeSet<String> badFiles = new TreeSet<>();
    for (CompilationUnitTree unit : task.parse()) {
      String name = unit.getSourceFile().getName();
      new TreePathScanner<Void, Void>() {
        private void check(Tree node) {
          TreePath parentPath = getCurrentPath().getParentPath();
          while (parentPath.getLeaf() instanceof ParenthesizedTree) {
            parentPath = parentPath.getParentPath();
          }
          Tree parent = parentPath.getLeaf();
          // For-loop init/update entries are ExpressionStatementTrees too.
          if (parent instanceof ExpressionStatementTree) {
            return;
          }
          // `@Foo(name = value)` is parsed as an AssignmentTree; it is not an assignment.
          if (parent instanceof AnnotationTree) {
            return;
          }
          String kind = node.getKind().toString();
          if (parent instanceof LambdaExpressionTree) {
            kind += " (expression-bodied lambda)";
          }
          long line = unit.getLineMap().getLineNumber(positions.getStartPosition(unit, node));
          System.out.println(name + ":" + line + ":" + kind);
          count[0]++;
          badFiles.add(name);
        }

        @Override
        public Void visitAssignment(AssignmentTree node, Void p) {
          check(node);
          return super.visitAssignment(node, p);
        }

        @Override
        public Void visitCompoundAssignment(CompoundAssignmentTree node, Void p) {
          check(node);
          return super.visitCompoundAssignment(node, p);
        }

        @Override
        public Void visitUnary(UnaryTree node, Void p) {
          switch (node.getKind()) {
            case PREFIX_INCREMENT:
            case PREFIX_DECREMENT:
            case POSTFIX_INCREMENT:
            case POSTFIX_DECREMENT:
              check(node);
              break;
            default:
              break;
          }
          return super.visitUnary(node, p);
        }
      }.scan(unit, null);
    }
    System.out.println(
        "TOTAL " + root + ": " + count[0] + " sites in " + badFiles.size() + " files"
            + " (" + files.size() + " files scanned)");
    return count[0];
  }
}
