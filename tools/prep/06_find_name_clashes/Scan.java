// Prep step 06b scanner / rewriter: finds fields whose simple name is also the name of a method in
// the same class hierarchy, and fields that hide a field of a supertype.
//
// For every class, interface, enum or record C in the given sources (nested, local and anonymous
// ones included; outer classes do not count as part of the hierarchy), a field F declared in C
// (static or not; enum constants and record components are exempt) is reported when
//   own-method        C declares a method named F,
//   inherited-method  a supertype of C (class or interface) has a method named F: any method of a
//                     supertype that is in the scanned sources, any method C inherits from a
//                     library supertype,
//   subclass-method   a subtype of C in the scanned sources declares a method named F,
//   shadows-field     a supertype of C has a field named F (same visibility rule as for methods).
// Each field is printed once, as file:line: Class.field: reasons. Exit status 1 if any is found.
//
// A reported non-static, non-transient field of a java.io.Serializable class is marked
// [serialized]: renaming it changes the serialized form.
//
// With --rewrite, every reported field `foo` is renamed to `fooField` (a constant `FOO_BAR` to
// `FOO_BAR_FIELD`): the declaration and every identifier / member select that javac attributes to
// that field symbol, in all scanned sources.
// Nothing is rewritten if a rename target already exists as a name anywhere in the hierarchy or if
// a static import could refer to a renamed field.
//
// Run from the repo root after tools/gate0/build.sh (JDK 21; attribution needs the libraries):
//   X=--add-exports=jdk.compiler/com.sun.tools.javac.tree=ALL-UNNAMED
//   javac $X -d /tmp/scan06 tools/prep/06_find_name_clashes/Scan.java
//   java $X -Xss16m -cp /tmp/scan06 Scan [--rewrite] \
//       ".cache/build/stock-libs.jar:$(ls .cache/deps/*.jar | tr '\n' ':')" java/src java/test

import com.sun.source.tree.ClassTree;
import com.sun.source.tree.CompilationUnitTree;
import com.sun.source.tree.IdentifierTree;
import com.sun.source.tree.ImportTree;
import com.sun.source.tree.MemberSelectTree;
import com.sun.source.tree.Tree;
import com.sun.source.tree.VariableTree;
import com.sun.source.util.JavacTask;
import com.sun.source.util.SourcePositions;
import com.sun.source.util.TreePath;
import com.sun.source.util.TreePathScanner;
import com.sun.source.util.Trees;
import com.sun.tools.javac.tree.JCTree;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.stream.Stream;
import javax.lang.model.element.Element;
import javax.lang.model.element.ElementKind;
import javax.lang.model.element.Modifier;
import javax.lang.model.element.TypeElement;
import javax.lang.model.element.VariableElement;
import javax.lang.model.type.DeclaredType;
import javax.lang.model.type.TypeMirror;
import javax.lang.model.util.Elements;
import javax.lang.model.util.Types;
import javax.tools.Diagnostic;
import javax.tools.JavaCompiler;
import javax.tools.JavaFileObject;
import javax.tools.StandardJavaFileManager;
import javax.tools.ToolProvider;

public final class Scan {
  private record Edit(int start, String oldText, String newText) {}

  public static void main(String[] argv) throws Exception {
    List<String> args = new ArrayList<>(List.of(argv));
    boolean rewrite = args.remove("--rewrite");
    String classpath = args.remove(0);
    List<File> files = new ArrayList<>();
    for (String root : args) {
      try (Stream<Path> paths = Files.walk(Paths.get(root))) {
        paths
            .filter(p -> p.toString().endsWith(".java"))
            .sorted()
            .forEach(p -> files.add(p.toFile()));
      }
    }
    JavaCompiler compiler = ToolProvider.getSystemJavaCompiler();
    StandardJavaFileManager fm = compiler.getStandardFileManager(null, null, null);
    Iterable<? extends JavaFileObject> units = fm.getJavaFileObjectsFromFiles(files);
    int[] errors = new int[1];
    JavacTask task =
        (JavacTask)
            compiler.getTask(
                null,
                fm,
                d -> {
                  if (d.getKind() == Diagnostic.Kind.ERROR) {
                    errors[0]++;
                    System.err.println(d);
                  }
                },
                List.of("-proc:none", "-nowarn", "-Xlint:none", "-cp", classpath),
                null,
                units);
    Trees trees = Trees.instance(task);
    SourcePositions positions = trees.getSourcePositions();
    List<CompilationUnitTree> parsed = new ArrayList<>();
    task.parse().forEach(parsed::add);
    task.analyze();
    if (errors[0] > 0) {
      System.err.println("sources do not compile: " + errors[0] + " errors");
      System.exit(2);
    }
    Elements elements = task.getElements();
    Types types = task.getTypes();

    // Every type declared in the sources, with where it is.
    Map<TypeElement, String> sourceTypes = new LinkedHashMap<>();
    Map<VariableElement, String> fieldLocation = new LinkedHashMap<>();
    for (CompilationUnitTree unit : parsed) {
      String file = unit.getSourceFile().getName();
      new TreePathScanner<Void, Void>() {
        @Override
        public Void visitClass(ClassTree node, Void unused) {
          Element e = trees.getElement(getCurrentPath());
          if (e instanceof TypeElement type) {
            sourceTypes.put(type, file);
            for (Tree member : node.getMembers()) {
              if (member instanceof VariableTree) {
                Element f = ((JCTree.JCVariableDecl) member).sym;
                long line =
                    unit.getLineMap().getLineNumber(((JCTree.JCVariableDecl) member).pos);
                fieldLocation.put((VariableElement) f, file + ":" + line);
              }
            }
          }
          return super.visitClass(node, unused);
        }
      }.scan(unit, null);
    }
    Map<TypeElement, Set<TypeElement>> supertypes = new LinkedHashMap<>();
    Map<TypeElement, Set<TypeElement>> subtypes = new LinkedHashMap<>();
    for (TypeElement type : sourceTypes.keySet()) {
      Set<TypeElement> supers = new LinkedHashSet<>();
      collectSupertypes(types, type.asType(), supers);
      supers.remove(type);
      supertypes.put(type, supers);
      for (TypeElement s : supers) {
        subtypes.computeIfAbsent(s, k -> new LinkedHashSet<>()).add(type);
      }
    }

    // field -> reasons
    Map<VariableElement, List<String>> clashes = new LinkedHashMap<>();
    Map<String, Integer> counts = new TreeMap<>();
    for (TypeElement type : sourceTypes.keySet()) {
      for (Element member : type.getEnclosedElements()) {
        if (member.getKind() != ElementKind.FIELD) {
          continue; // enum constants are ENUM_CONSTANT
        }
        if (type.getKind() == ElementKind.RECORD
            && !member.getModifiers().contains(Modifier.STATIC)) {
          continue; // record component
        }
        String name = member.getSimpleName().toString();
        List<String> reasons = new ArrayList<>();
        if (declares(type, name, true)) {
          reasons.add("own-method");
        }
        for (TypeElement s : supertypes.get(type)) {
          boolean inSources = sourceTypes.containsKey(s);
          if (inSources ? declares(s, name, true) : inherits(elements, type, s, name, true)) {
            reasons.add("inherited-method(" + s.getSimpleName() + ")");
          }
          if (inSources ? declares(s, name, false) : inherits(elements, type, s, name, false)) {
            reasons.add("shadows-field(" + s.getSimpleName() + ")");
          }
        }
        for (TypeElement s : subtypes.getOrDefault(type, Set.of())) {
          if (declares(s, name, true)) {
            reasons.add("subclass-method(" + describe(s) + ")");
          }
        }
        if (!reasons.isEmpty()) {
          clashes.put((VariableElement) member, reasons);
          String where = fieldLocation.get(member);
          String root =
              where.contains("/testing/")
                  ? "testing"
                  : where.startsWith("java/test") ? "test" : "main";
          String kind =
              reasons.stream().anyMatch(r -> r.contains("method")) ? "method-clash" : "shadow-only";
          counts.merge(root + " " + kind, 1, Integer::sum);
          boolean serialized =
              types.isAssignable(
                      types.erasure(type.asType()),
                      elements.getTypeElement("java.io.Serializable").asType())
                  && type.getKind() != ElementKind.ENUM // enums are serialized by constant name
                  && !member.getModifiers().contains(Modifier.STATIC)
                  && !member.getModifiers().contains(Modifier.TRANSIENT);
          System.out.println(
              where
                  + ": "
                  + describe(type)
                  + "."
                  + name
                  + ": "
                  + String.join(", ", reasons)
                  + (serialized ? " [serialized]" : ""));
        }
      }
    }
    System.out.println("fields to rename: " + clashes.size() + " " + counts);
    if (!rewrite) {
      System.exit(clashes.isEmpty() ? 0 : 1);
    }

    // Refuse to create a new clash: the new name must not be a member anywhere in the hierarchy.
    int problems = 0;
    for (VariableElement field : clashes.keySet()) {
      TypeElement type = (TypeElement) field.getEnclosingElement();
      String target = target(field.getSimpleName().toString());
      Set<TypeElement> related = new LinkedHashSet<>(supertypes.get(type));
      related.add(type);
      related.addAll(subtypes.getOrDefault(type, Set.of()));
      for (TypeElement r : related) {
        if (declares(r, target, true) || declares(r, target, false)) {
          System.err.println("target exists: " + describe(r) + "." + target);
          problems++;
        }
      }
    }

    Map<String, List<Edit>> edits = new LinkedHashMap<>();
    Set<String> targetNames = new LinkedHashSet<>();
    for (VariableElement f : clashes.keySet()) {
      targetNames.add(target(f.getSimpleName().toString()));
    }
    int[] problemsRef = {problems};
    for (CompilationUnitTree unit : parsed) {
      String file = unit.getSourceFile().getName();
      List<Edit> fileEdits = new ArrayList<>();
      new TreePathScanner<Void, Void>() {
        private void rename(int start, String name) {
          fileEdits.add(new Edit(start, name, target(name)));
        }

        @Override
        public Void visitImport(ImportTree node, Void unused) {
          if (node.isStatic()) {
            MemberSelectTree select = (MemberSelectTree) node.getQualifiedIdentifier();
            String name = select.getIdentifier().toString();
            Element owner =
                trees.getElement(new TreePath(getCurrentPath(), select.getExpression()));
            if (owner instanceof TypeElement type) {
              for (VariableElement f : clashes.keySet()) {
                if (f.getEnclosingElement().equals(type) && f.getSimpleName().contentEquals(name)) {
                  System.err.println(file + ": static import of renamed field " + name);
                  problemsRef[0]++;
                }
              }
            }
          }
          return null;
        }

        @Override
        public Void visitVariable(VariableTree node, Void unused) {
          JCTree.JCVariableDecl decl = (JCTree.JCVariableDecl) node;
          if (decl.sym != null && clashes.containsKey(decl.sym)) {
            rename(decl.pos, node.getName().toString());
          } else if (targetNames.contains(node.getName().toString())) {
            // A local or parameter that already has a target name could capture a renamed use.
            System.err.println(
                file + ": existing variable named " + node.getName() + " (check for capture)");
          }
          return super.visitVariable(node, unused);
        }

        @Override
        public Void visitIdentifier(IdentifierTree node, Void unused) {
          Element e = trees.getElement(getCurrentPath());
          if (e != null && clashes.containsKey(e)) {
            rename((int) positions.getStartPosition(unit, node), node.getName().toString());
          }
          return super.visitIdentifier(node, unused);
        }

        @Override
        public Void visitMemberSelect(MemberSelectTree node, Void unused) {
          Element e = trees.getElement(getCurrentPath());
          if (e != null && clashes.containsKey(e)) {
            String name = node.getIdentifier().toString();
            rename((int) positions.getEndPosition(unit, node) - name.length(), name);
          }
          return super.visitMemberSelect(node, unused);
        }
      }.scan(unit, null);
      if (!fileEdits.isEmpty()) {
        edits.put(file, fileEdits);
      }
    }
    problems = problemsRef[0];
    if (problems > 0) {
      System.err.println(problems + " problem(s); nothing rewritten");
      System.exit(2);
    }
    int sites = 0;
    for (Map.Entry<String, List<Edit>> entry : edits.entrySet()) {
      Path path = Paths.get(entry.getKey());
      String text = Files.readString(path, StandardCharsets.UTF_8);
      List<Edit> fileEdits = new ArrayList<>(new LinkedHashSet<>(entry.getValue()));
      fileEdits.sort((a, b) -> Integer.compare(b.start(), a.start()));
      StringBuilder sb = new StringBuilder(text);
      for (Edit edit : fileEdits) {
        int end = edit.start() + edit.oldText().length();
        if (!text.substring(edit.start(), end).equals(edit.oldText())) {
          throw new IllegalStateException(
              entry.getKey() + "@" + edit.start() + ": expected " + edit.oldText());
        }
        sb.replace(edit.start(), end, edit.newText());
        sites++;
      }
      Files.writeString(path, sb.toString(), StandardCharsets.UTF_8);
    }
    System.out.println(
        "renamed "
            + clashes.size()
            + " fields at "
            + sites
            + " sites in "
            + edits.size()
            + " files");
  }

  private static String target(String name) {
    return name.matches("[A-Z0-9_]+") ? name + "_FIELD" : name + "Field";
  }

  private static void collectSupertypes(Types types, TypeMirror type, Set<TypeElement> out) {
    for (TypeMirror s : types.directSupertypes(type)) {
      if (s instanceof DeclaredType declared && declared.asElement() instanceof TypeElement e) {
        if (out.add(e)) {
          collectSupertypes(types, e.asType(), out);
        }
      }
    }
  }

  /** Whether {@code type} itself declares a method (or else a field) with the name. */
  private static boolean declares(TypeElement type, String name, boolean method) {
    for (Element member : type.getEnclosedElements()) {
      boolean kindMatches =
          method
              ? member.getKind() == ElementKind.METHOD
              : member.getKind() == ElementKind.FIELD
                  || member.getKind() == ElementKind.ENUM_CONSTANT;
      if (kindMatches && member.getSimpleName().contentEquals(name)) {
        return true;
      }
    }
    return false;
  }

  /** Whether {@code type} inherits a method (or else a field) with the name from {@code from}. */
  private static boolean inherits(
      Elements elements, TypeElement type, TypeElement from, String name, boolean method) {
    for (Element member : elements.getAllMembers(type)) {
      boolean kindMatches =
          method
              ? member.getKind() == ElementKind.METHOD
              : member.getKind() == ElementKind.FIELD
                  || member.getKind() == ElementKind.ENUM_CONSTANT;
      if (kindMatches
          && member.getSimpleName().contentEquals(name)
          && member.getEnclosingElement().equals(from)) {
        return true;
      }
    }
    return false;
  }

  private static String describe(TypeElement type) {
    String name = type.getQualifiedName().toString();
    if (!name.isEmpty()) {
      return name;
    }
    Element outer = type.getEnclosingElement();
    while (outer != null && !(outer instanceof TypeElement)) {
      outer = outer.getEnclosingElement();
    }
    return (outer == null ? "?" : describe((TypeElement) outer))
        + ".<anonymous/local "
        + type.getSimpleName()
        + ">";
  }
}
