// Prep step 17 checker: the graph classes keep their type parameters through j2go.
//
// j2go (tools/j2go/src/j2go/GenScan.java) keeps a type parameter of a generic class of the
// program as a Go type parameter only if no mention of the class has a wildcard or a raw use at
// that position; one `DiGraph<N, ?>` in a method signature erases DiGraph's E, and with it E of
// every graph class it shares E with (Graph, DiGraph, UndiGraph, their nodes and edges, the
// Linked* implementations, ControlFlowGraph). A local variable (or for-each variable) that is
// never assigned again does not erase: GenScan refines its declared type from its one value.
//
// Parses the given source roots (files under a /testing/ directory are skipped), collects the
// generic classes declared in package com.google.javascript.jscomp.graph, and prints
// "file:line: kind: type" for every mention of one of them with a wildcard type argument:
//   signature  a method's parameter or result, a field, a supertype, a type argument of a
//              call or of a `new` (each erases the type parameter);
//   local      a local or for-each variable (GenScan refines it; listed, not counted).
// Exit status 1 if there is a signature mention.
// Run (JDK 21, parse-only):
//   javac -d /tmp/scan17 tools/prep/17_find_graph_wildcards/Scan.java && java -cp /tmp/scan17 Scan java/src

import com.sun.source.tree.ClassTree;
import com.sun.source.tree.CompilationUnitTree;
import com.sun.source.tree.EnhancedForLoopTree;
import com.sun.source.tree.ParameterizedTypeTree;
import com.sun.source.tree.Tree;
import com.sun.source.tree.VariableTree;
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
  private static final String GRAPH_PACKAGE = "com.google.javascript.jscomp.graph";

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
    List<CompilationUnitTree> parsed = new ArrayList<>();
    for (CompilationUnitTree unit : task.parse()) {
      parsed.add(unit);
    }

    // The generic classes of the graph package, by simple name (nested ones included).
    TreeSet<String> generic = new TreeSet<>();
    for (CompilationUnitTree unit : parsed) {
      if (unit.getPackageName() == null || !unit.getPackageName().toString().equals(GRAPH_PACKAGE)) {
        continue;
      }
      new TreePathScanner<Void, Void>() {
        @Override
        public Void visitClass(ClassTree tree, Void unused) {
          if (!tree.getTypeParameters().isEmpty()) {
            generic.add(tree.getSimpleName().toString());
          }
          return super.visitClass(tree, unused);
        }
      }.scan(unit, null);
    }

    int[] counts = new int[2]; // signature mentions, local mentions
    for (CompilationUnitTree unit : parsed) {
      String name = unit.getSourceFile().getName();
      new TreePathScanner<Void, Void>() {
        @Override
        public Void visitParameterizedType(ParameterizedTypeTree tree, Void unused) {
          String simple = tree.getType().toString();
          simple = simple.substring(simple.lastIndexOf('.') + 1);
          boolean wild = false;
          for (Tree a : tree.getTypeArguments()) {
            wild |= a.getKind() == Tree.Kind.UNBOUNDED_WILDCARD
                || a.getKind() == Tree.Kind.EXTENDS_WILDCARD
                || a.getKind() == Tree.Kind.SUPER_WILDCARD;
          }
          if (wild && generic.contains(simple)) {
            boolean local = isLocalType(getCurrentPath());
            long line = unit.getLineMap().getLineNumber(positions.getStartPosition(unit, tree));
            System.out.println(name + ":" + line + ": " + (local ? "local" : "signature") + ": " + tree);
            counts[local ? 1 : 0]++;
          }
          return super.visitParameterizedType(tree, unused);
        }
      }.scan(unit, null);
    }
    System.out.println(
        "graph classes: " + generic.size() + "; wildcard mentions in signatures: " + counts[0]
            + ", in locals (refined by j2go): " + counts[1]);
    System.exit(counts[0] == 0 ? 0 : 1);
  }

  /**
   * Whether the parameterized type at path is the declared type of a local or for-each variable
   * (the type itself, not a type argument nested in it, which GenScan refines with it).
   */
  private static boolean isLocalType(TreePath path) {
    for (TreePath p = path.getParentPath(); p != null; p = p.getParentPath()) {
      Tree t = p.getLeaf();
      if (t instanceof VariableTree v) {
        Tree owner = p.getParentPath().getLeaf();
        boolean field = owner instanceof ClassTree;
        boolean parameter = owner.getKind() == Tree.Kind.METHOD || owner.getKind() == Tree.Kind.LAMBDA_EXPRESSION;
        return !field && !parameter && (owner instanceof EnhancedForLoopTree || isBlockLike(owner));
      }
      if (!(t instanceof ParameterizedTypeTree) && t.getKind() != Tree.Kind.ARRAY_TYPE) {
        return false;
      }
    }
    return false;
  }

  private static boolean isBlockLike(Tree owner) {
    return switch (owner.getKind()) {
      case BLOCK, CASE, FOR_LOOP, TRY -> true;
      default -> false;
    };
  }
}
