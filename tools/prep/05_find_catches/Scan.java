// Prep step 05 scanner: prints "file:line: catch (Types)" for every catch clause in the given
// source roots (files under a /testing/ directory are skipped), then totals: catch clauses,
// try statements having at least one catch, files, and a count per caught exception type.
// Run (JDK 21, parse-only):
//   javac -d /tmp/scan05 tools/prep/05_find_catches/Scan.java && java -cp /tmp/scan05 Scan java/src

import com.sun.source.tree.CatchTree;
import com.sun.source.tree.CompilationUnitTree;
import com.sun.source.tree.Tree;
import com.sun.source.tree.TryTree;
import com.sun.source.tree.UnionTypeTree;
import com.sun.source.util.JavacTask;
import com.sun.source.util.SourcePositions;
import com.sun.source.util.TreeScanner;
import com.sun.source.util.Trees;
import java.io.File;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.List;
import java.util.Map;
import java.util.TreeMap;
import java.util.TreeSet;
import java.util.stream.Stream;
import javax.tools.JavaCompiler;
import javax.tools.JavaFileObject;
import javax.tools.StandardJavaFileManager;
import javax.tools.ToolProvider;

public final class Scan {
  public static void main(String[] args) throws Exception {
    List<File> files = new ArrayList<>();
    for (String root : args) {
      try (Stream<Path> s = Files.walk(Paths.get(root))) {
        for (Path p : new TreeSet<>(s.toList())) {
          String n = p.toString();
          if (n.endsWith(".java") && !n.contains("/testing/")) {
            files.add(p.toFile());
          }
        }
      }
    }
    JavaCompiler compiler = ToolProvider.getSystemJavaCompiler();
    StandardJavaFileManager fm = compiler.getStandardFileManager(null, null, null);
    Iterable<? extends JavaFileObject> units = fm.getJavaFileObjectsFromFiles(files);
    JavacTask task = (JavacTask) compiler.getTask(null, fm, d -> {}, null, null, units);
    SourcePositions positions = Trees.instance(task).getSourcePositions();
    int[] counts = new int[2]; // catch clauses, try statements with a catch
    TreeSet<String> filesWithCatch = new TreeSet<>();
    Map<String, Integer> byType = new TreeMap<>();
    for (CompilationUnitTree unit : task.parse()) {
      String name = unit.getSourceFile().getName();
      new TreeScanner<Void, Void>() {
        @Override
        public Void visitTry(TryTree tree, Void unused) {
          if (!tree.getCatches().isEmpty()) {
            counts[1]++;
          }
          return super.visitTry(tree, unused);
        }

        @Override
        public Void visitCatch(CatchTree tree, Void unused) {
          Tree type = tree.getParameter().getType();
          List<? extends Tree> alternatives =
              type instanceof UnionTypeTree ? ((UnionTypeTree) type).getTypeAlternatives() : List.of(type);
          for (Tree alternative : alternatives) {
            byType.merge(alternative.toString(), 1, Integer::sum);
          }
          long line = unit.getLineMap().getLineNumber(positions.getStartPosition(unit, tree));
          System.out.println(name + ":" + line + ": catch (" + type + ")");
          counts[0]++;
          filesWithCatch.add(name);
          return super.visitCatch(tree, unused);
        }
      }.scan(unit, null);
    }
    System.out.println(
        "catch clauses: " + counts[0] + ", try statements with catch: " + counts[1]
            + ", files: " + filesWithCatch.size());
    for (Map.Entry<String, Integer> e : byType.entrySet()) {
      System.out.println("  " + e.getValue() + " " + e.getKey());
    }
  }
}
