// Prep step 14 checker: I/O types outside the facade.
//
// Attributes the production sources (java/src without */testing/, same flags as
// tools/gate0/build.sh) and reports every reference, in any file other than the facade
// (com/google/javascript/jscomp/base/Io.java) and the JSON facade
// (com/google/javascript/jscomp/base/Json.java, which hands the JSON library a
// java.io.StringReader over the text to parse), to a type of a forbidden package:
//   java.io.*, java.nio.file.*, java.nio.channels.*, java.nio.charset.*, java.util.zip.*,
//   java.util.jar.*, com.google.common.io.*, java.net.*
// A reference is: a use of the type's name, a call of / reference to one of its members, or an
// expression whose static type is such a type (so System.out and Class.getResourceAsStream
// count even though no forbidden name is written).
//
// Always allowed (not reported): java.io.Serializable, java.io.IOException,
// java.io.UncheckedIOException (the error type carried by Outcome).
// Exemptions (reported as EXEMPT, do not fail the check), all of them in-memory glue for a
// library API that is replaced by a later step; see EXEMPT below:
//   * java.io.ByteArrayInputStream / StringReader for javax.xml's InputSource in XtbMessageBundle;
//   * java.io.ObjectInputStream in JSChunk.readObject (Java serialization hook).
//
// Run from the repo root after tools/gate0/build.sh (JDK 21):
//   java tools/prep/14_find_io_uses/Scan.java            # findings, one line per file+type
//   java tools/prep/14_find_io_uses/Scan.java --detail   # inventory: type -> member -> count -> files,
//                                                        # plus every "throws IOException" method
// Exit status 1 if a non-exempt finding exists.

import com.sun.source.tree.CompilationUnitTree;
import com.sun.source.tree.ExpressionTree;
import com.sun.source.tree.IdentifierTree;
import com.sun.source.tree.MemberReferenceTree;
import com.sun.source.tree.MemberSelectTree;
import com.sun.source.tree.MethodInvocationTree;
import com.sun.source.tree.MethodTree;
import com.sun.source.tree.NewClassTree;
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
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;
import java.util.stream.Stream;
import javax.lang.model.element.Element;
import javax.lang.model.element.ElementKind;
import javax.lang.model.element.TypeElement;
import javax.lang.model.type.ArrayType;
import javax.lang.model.type.DeclaredType;
import javax.lang.model.type.TypeKind;
import javax.lang.model.type.TypeMirror;
import javax.tools.Diagnostic;
import javax.tools.JavaCompiler;
import javax.tools.StandardJavaFileManager;
import javax.tools.ToolProvider;

public final class Scan {
  static final String FACADE = "java/src/com/google/javascript/jscomp/base/Io.java";

  // The JSON library reads from a java.io.Reader: Json.parse wraps its text in a StringReader.
  static final String JSON_FACADE = "java/src/com/google/javascript/jscomp/base/Json.java";

  static final String[] FORBIDDEN_PACKAGES = {
    "java.io", "java.nio.file", "java.nio.channels", "java.nio.charset", "java.util.zip",
    "java.util.jar", "com.google.common.io", "java.net",
  };

  static final Set<String> ALLOWED_TYPES =
      Set.of("java.io.Serializable", "java.io.IOException", "java.io.UncheckedIOException");

  // "file: type" pairs. Keep this list short; every entry is explained in the header.
  static final Set<String> EXEMPT = new TreeSet<>();

  static void exempt(String file, String... types) {
    for (String type : types) {
      EXEMPT.add("java/src/com/google/" + file + ": java.io." + type);
    }
  }

  static {
    // javax.xml InputSource over in-memory bytes / an empty string.
    exempt("javascript/jscomp/XtbMessageBundle.java", "ByteArrayInputStream", "StringReader");
    // Java serialization hook.
    exempt("javascript/jscomp/JSChunk.java", "ObjectInputStream");
  }

  static Trees trees;
  // type -> member -> file -> count
  static final Map<String, Map<String, Map<String, Integer>>> uses = new TreeMap<>();
  static final List<String> throwers = new ArrayList<>();

  static String forbidden(TypeMirror t) {
    while (t != null && t.getKind() == TypeKind.ARRAY) {
      t = ((ArrayType) t).getComponentType();
    }
    if (t == null || t.getKind() != TypeKind.DECLARED) {
      return null;
    }
    return forbiddenElement(((DeclaredType) t).asElement());
  }

  static String forbiddenElement(Element e) {
    while (e != null && !(e.getKind().isClass() || e.getKind().isInterface())) {
      e = e.getEnclosingElement();
    }
    if (!(e instanceof TypeElement)) {
      return null;
    }
    Element top = e;
    while (top.getEnclosingElement() != null
        && top.getEnclosingElement().getKind() != ElementKind.PACKAGE) {
      top = top.getEnclosingElement();
    }
    if (!(top instanceof TypeElement)) {
      return null;
    }
    String name = ((TypeElement) e).getQualifiedName().toString();
    String topName = ((TypeElement) top).getQualifiedName().toString();
    if (ALLOWED_TYPES.contains(topName)) {
      return null;
    }
    int dot = topName.lastIndexOf('.');
    String pkg = dot < 0 ? "" : topName.substring(0, dot);
    for (String p : FORBIDDEN_PACKAGES) {
      if (pkg.equals(p)) {
        return name;
      }
    }
    return null;
  }

  static void record(String type, String member, String file) {
    uses.computeIfAbsent(type, k -> new TreeMap<>())
        .computeIfAbsent(member, k -> new TreeMap<>())
        .merge(file, 1, Integer::sum);
  }

  public static void main(String[] args) throws Exception {
    boolean detail = args.length > 0 && args[0].equals("--detail");
    List<File> files = new ArrayList<>();
    try (Stream<Path> paths = Files.walk(Paths.get("java/src"))) {
      for (Path p : new TreeSet<>(paths.toList())) {
        String n = p.toString();
        if (n.endsWith(".java") && !n.contains("/testing/")) {
          files.add(p.toFile());
        }
      }
    }
    StringBuilder deps = new StringBuilder(".cache/build/stock-libs.jar");
    try (Stream<Path> jars = Files.list(Paths.get(".cache/deps"))) {
      for (Path p : new TreeSet<>(jars.toList())) {
        if (p.toString().endsWith(".jar")) {
          deps.append(':').append(p);
        }
      }
    }
    Path out = Files.createTempDirectory("scan14");
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
                    "-d", out.toString(), "-cp", deps.toString()),
                null,
                fm.getJavaFileObjectsFromFiles(files));
    trees = Trees.instance(task);
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
    for (CompilationUnitTree unit : units) {
      String file = Paths.get("").toAbsolutePath().relativize(Paths.get(unit.getSourceFile().toUri())).toString();
      if (file.equals(FACADE) || file.equals(JSON_FACADE)) {
        continue;
      }
      new TreePathScanner<Void, Void>() {
        @Override
        public Void visitIdentifier(IdentifierTree node, Void p) {
          name(node, node.getName().toString());
          return super.visitIdentifier(node, p);
        }

        @Override
        public Void visitMemberSelect(MemberSelectTree node, Void p) {
          name(node, node.getIdentifier().toString());
          return super.visitMemberSelect(node, p);
        }

        @Override
        public Void visitMemberReference(MemberReferenceTree node, Void p) {
          name(node, node.getName().toString());
          return super.visitMemberReference(node, p);
        }

        @Override
        public Void visitNewClass(NewClassTree node, Void p) {
          Element e = trees.getElement(getCurrentPath());
          String t = e == null ? null : forbiddenElement(e);
          if (t != null) {
            record(t, "<init>", file);
          }
          return super.visitNewClass(node, p);
        }

        @Override
        public Void visitMethodInvocation(MethodInvocationTree node, Void p) {
          TypeMirror tm = trees.getTypeMirror(getCurrentPath());
          String t = forbidden(tm);
          Element callee = trees.getElement(new TreePath(getCurrentPath(), node.getMethodSelect()));
          if (t != null && (callee == null || forbiddenElement(callee) == null)) {
            record(t, "(returned by) " + describe(callee), file);
          }
          return super.visitMethodInvocation(node, p);
        }

        @Override
        public Void visitMethod(MethodTree node, Void p) {
          for (ExpressionTree th : node.getThrows()) {
            String s = th.toString();
            if (s.endsWith("IOException")) {
              long line =
                  unit.getLineMap()
                      .getLineNumber(trees.getSourcePositions().getStartPosition(unit, node));
              throwers.add(file + ":" + line + ": " + node.getName() + " throws " + s);
            }
          }
          return super.visitMethod(node, p);
        }

        String describe(Element e) {
          if (e == null) {
            return "?";
          }
          Element c = e.getEnclosingElement();
          return (c instanceof TypeElement te ? te.getQualifiedName() + "." : "") + e.getSimpleName();
        }

        void name(Tree node, String ident) {
          Element e = trees.getElement(getCurrentPath());
          if (e == null || e.getKind() == ElementKind.PACKAGE) {
            return;
          }
          Tree parentTree = getCurrentPath().getParentPath().getLeaf();
          if (parentTree.getKind() == Tree.Kind.IMPORT) {
            return;
          }
          if (e.getKind().isClass() || e.getKind().isInterface()) {
            String t = forbiddenElement(e);
            if (t != null && !(parentTree instanceof NewClassTree nc && nc.getIdentifier() == node)) {
              record(t, "<type name>", file);
            }
            return;
          }
          if (e.getKind() == ElementKind.CONSTRUCTOR) {
            return;
          }
          String owner = forbiddenElement(e.getEnclosingElement());
          boolean member =
              e.getKind() == ElementKind.METHOD
                  || e.getKind() == ElementKind.FIELD
                  || e.getKind() == ElementKind.ENUM_CONSTANT;
          if (member && owner != null) {
            record(owner, ident, file);
            return;
          }
          if (e.getKind() == ElementKind.FIELD) {
            // e.g. System.out: a field of a non-forbidden class whose type is forbidden.
            String t = forbidden(e.asType());
            if (t != null && forbiddenElement(e.getEnclosingElement()) == null) {
              Element c = e.getEnclosingElement();
              boolean ours = trees.getTree(e) != null;
              if (!ours) {
                record(t, "(type of) " + describe(e), file);
              }
            }
          }
        }
      }.scan(unit, null);
    }
    int bad = 0;
    if (detail) {
      for (var te : uses.entrySet()) {
        System.out.println(te.getKey());
        for (var me : te.getValue().entrySet()) {
          int n = 0;
          StringBuilder fs = new StringBuilder();
          for (var fe : me.getValue().entrySet()) {
            n += fe.getValue();
            String f = fe.getKey();
            fs.append(' ').append(f.substring(f.lastIndexOf('/') + 1).replace(".java", ""));
          }
          System.out.println("  " + me.getKey() + "  " + n + " :" + fs);
        }
      }
      System.out.println();
      for (String s : throwers) {
        System.out.println(s);
      }
      System.out.println("throws IOException methods: " + throwers.size());
    }
    // file -> type -> count
    Map<String, Map<String, Integer>> byFile = new TreeMap<>();
    for (var te : uses.entrySet()) {
      for (var me : te.getValue().entrySet()) {
        for (var fe : me.getValue().entrySet()) {
          byFile.computeIfAbsent(fe.getKey(), k -> new TreeMap<>())
              .merge(te.getKey(), fe.getValue(), Integer::sum);
        }
      }
    }
    int exempt = 0;
    for (var fe : byFile.entrySet()) {
      for (var te : fe.getValue().entrySet()) {
        boolean ex = EXEMPT.contains(fe.getKey() + ": " + te.getKey());
        System.out.println(
            (ex ? "EXEMPT " : "") + fe.getKey() + ": " + te.getKey() + " x" + te.getValue());
        if (ex) {
          exempt++;
        } else {
          bad++;
        }
      }
    }
    System.out.println(
        "files with findings: " + byFile.size() + ", findings (file+type): " + bad
            + ", exempt: " + exempt + ", types: " + uses.size());
    System.exit(bad == 0 ? 0 : 1);
  }
}
