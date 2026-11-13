// Prep step 07 scanner / rewriter: same-arity overloads.
//
// Attributes java/src and java/test (one javac task, same flags as tools/gate0/build.sh) and
// prints every group of methods that have the same name and the same parameter count but are
// different methods (not overrides of one another) and are members of one class: declared in it
// or inherited from supertypes compiled from java/src or java/test. Static and instance methods
// share the namespace, varargs counts as one parameter. Also prints classes with two constructors
// of the same parameter count. A group all of whose methods override library methods
// (Appendable.append, OutputStream.write, ...) or are compiler-provided (enum valueOf) is printed
// as EXEMPT. Exit status 1 if a non-exempt group exists.
//
// Run from the repo root after tools/gate0/build.sh (JDK 21):
//   java tools/prep/07_find_same_arity_overloads/Scan.java                 # report
//   java tools/prep/07_find_same_arity_overloads/Scan.java --plan          # report + rename plan
//   java tools/prep/07_find_same_arity_overloads/Scan.java --apply         # rewrite the sources
//   java tools/prep/07_find_same_arity_overloads/Scan.java --dump FILE     # resolution map
//
// Rename rule (--plan / --apply), methods only (constructors are reported, not rewritten):
//   * methods related by overriding (or by being the same signature inherited from two
//     supertypes, or static hiding) form one family and are always renamed together;
//   * within the families of one name and arity that meet in some class, families are taken in
//     order "overrides a library method first, then most call sites": a family keeps its name
//     unless it meets, in some class, a family that already kept it;
//   * a renamed family gets the name from names.tsv (next to this file; lines
//     "pkg.Class#name(erased,param,types)<TAB>newName") if listed there, else name + the simple
//     type name(s) of the parameter(s) that distinguish it from the families it meets
//     (int -> Int, T[] -> TArray, T... -> TVarargs);
//   * every invocation, method reference and static import is rewritten from the method symbol
//     javac resolved it to BEFORE the change, so no call can silently rebind.
// For constructors the report lists every use ("CTORUSE" lines carry the offsets of "new T");
// tools/prep/07_ctor_factories.py turns the listed "new T(...)" into factory calls.
// --dump writes, for every method declaration, its family and, for every invocation / method
// reference / constructor call, the declaration it resolves to, with declarations identified by
// (file, ordinal) rather than by name: the dumps before and after --apply must be identical.

import com.sun.source.tree.ClassTree;
import com.sun.source.tree.CompilationUnitTree;
import com.sun.source.tree.ExpressionTree;
import com.sun.source.tree.IdentifierTree;
import com.sun.source.tree.ImportTree;
import com.sun.source.tree.MemberReferenceTree;
import com.sun.source.tree.MemberSelectTree;
import com.sun.source.tree.MethodInvocationTree;
import com.sun.source.tree.MethodTree;
import com.sun.source.tree.NewClassTree;
import com.sun.source.tree.Tree;
import com.sun.source.util.JavacTask;
import com.sun.source.util.SourcePositions;
import com.sun.source.util.TreePath;
import com.sun.source.util.TreePathScanner;
import com.sun.source.util.Trees;
import java.io.File;
import java.io.PrintWriter;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.HashMap;
import java.util.HashSet;
import java.util.IdentityHashMap;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;
import java.util.stream.Collectors;
import java.util.stream.Stream;
import javax.lang.model.element.Element;
import javax.lang.model.element.ElementKind;
import javax.lang.model.element.ExecutableElement;
import javax.lang.model.element.Modifier;
import javax.lang.model.element.TypeElement;
import javax.lang.model.element.VariableElement;
import javax.lang.model.type.ArrayType;
import javax.lang.model.type.DeclaredType;
import javax.lang.model.type.ExecutableType;
import javax.lang.model.type.TypeKind;
import javax.lang.model.type.TypeMirror;
import javax.lang.model.util.Elements;
import javax.lang.model.util.Types;
import javax.tools.Diagnostic;
import javax.tools.JavaCompiler;
import javax.tools.JavaFileObject;
import javax.tools.StandardJavaFileManager;
import javax.tools.ToolProvider;

public final class Scan {
  static Trees trees;
  static Elements elements;
  static Types types;
  static SourcePositions positions;

  static final class Decl {
    ExecutableElement el;
    String file;
    int namePos; // -1 for constructors
    int ordinal;
    long line;
    int uses;
  }

  static final class Use {
    String file;
    int pos; // offset of the method name, -1 if there is none (constructor calls)
    String name;
    ExecutableElement target;
    boolean bare;
    String at; // file:line and source text of a constructor call
    int newStart = -1; // for "new T(...)": offsets of "new" and of the end of T
    int newTypeEnd = -1;
  }

  static final class StaticImport {
    String file;
    int start;
    int end;
    TypeElement cls;
    String name;
  }

  static final Map<ExecutableElement, Decl> decls = new LinkedHashMap<>();
  static final Map<String, List<Use>> usesByFile = new LinkedHashMap<>();
  static final List<StaticImport> staticImports = new ArrayList<>();
  static final Set<TypeElement> sourceClasses = new LinkedHashSet<>();
  static final Map<String, String> sources = new HashMap<>();
  static final Map<ExecutableElement, ExecutableElement> parent = new IdentityHashMap<>();

  static ExecutableElement find(ExecutableElement e) {
    ExecutableElement p = parent.get(e);
    if (p == null || p == e) {
      return e;
    }
    ExecutableElement r = find(p);
    parent.put(e, r);
    return r;
  }

  static void union(ExecutableElement a, ExecutableElement b) {
    a = find(a);
    b = find(b);
    if (a != b) {
      parent.put(a, b);
    }
  }

  public static void main(String[] args) throws Exception {
    boolean plan = false;
    boolean apply = false;
    String dump = null;
    for (int i = 0; i < args.length; i++) {
      switch (args[i]) {
        case "--plan" -> plan = true;
        case "--apply" -> apply = true;
        case "--dump" -> dump = args[++i];
        default -> throw new IllegalArgumentException(args[i]);
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
      deps =
          jars.map(Path::toString).filter(p -> p.endsWith(".jar")).sorted().collect(Collectors.joining(":"));
    }
    Path out = Files.createTempDirectory("scan07");
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
                    "-nowarn", "-Xlint:none", "-encoding", "UTF-8", "-parameters",
                    "-d", out.toString(), "-s", out.toString(),
                    "-cp", ".cache/build/stock-libs.jar:" + deps, "-processorpath", deps),
                null,
                fm.getJavaFileObjectsFromFiles(files));
    trees = Trees.instance(task);
    positions = trees.getSourcePositions();
    List<CompilationUnitTree> units = new ArrayList<>();
    for (CompilationUnitTree u : task.parse()) {
      units.add(u);
    }
    task.analyze();
    elements = task.getElements();
    types = task.getTypes();
    if (!errors.isEmpty()) {
      errors.stream().limit(30).forEach(System.err::println);
      System.err.println(errors.size() + " javac errors");
      System.exit(2);
    }
    for (CompilationUnitTree u : units) {
      collect(u);
    }
    buildFamilies();
    for (List<Use> l : usesByFile.values()) {
      for (Use u : l) {
        Decl d = decls.get(u.target);
        if (d != null) {
          d.uses++;
        }
      }
    }
    int bad = report();
    if (dump != null) {
      dump(dump);
    }
    if (plan || apply) {
      rename(apply);
    }
    System.exit(bad == 0 ? 0 : 1);
  }

  // ---------------------------------------------------------------- collection

  static void collect(CompilationUnitTree unit) throws Exception {
    String file = unit.getSourceFile().getName();
    String text = unit.getSourceFile().getCharContent(true).toString();
    sources.put(file, text);
    List<Use> uses = new ArrayList<>();
    usesByFile.put(file, uses);
    int[] ordinal = new int[1];
    new TreePathScanner<Void, Void>() {
      @Override
      public Void visitImport(ImportTree tree, Void p) {
        if (tree.isStatic()) {
          MemberSelectTree sel = (MemberSelectTree) tree.getQualifiedIdentifier();
          String name = sel.getIdentifier().toString();
          if (!name.equals("*")) {
            Element cls = trees.getElement(new TreePath(new TreePath(getCurrentPath(), sel), sel.getExpression()));
            if (cls instanceof TypeElement) {
              StaticImport si = new StaticImport();
              si.file = file;
              si.start = (int) positions.getStartPosition(unit, tree);
              si.end = (int) positions.getEndPosition(unit, tree);
              si.cls = (TypeElement) cls;
              si.name = name;
              staticImports.add(si);
            }
          }
        }
        return null;
      }

      @Override
      public Void visitClass(ClassTree tree, Void p) {
        Element e = trees.getElement(getCurrentPath());
        if (e instanceof TypeElement) {
          sourceClasses.add((TypeElement) e);
        }
        return super.visitClass(tree, p);
      }

      @Override
      public Void visitMethod(MethodTree tree, Void p) {
        Element e = trees.getElement(getCurrentPath());
        if (e instanceof ExecutableElement && positions.getStartPosition(unit, tree) >= 0
            && positions.getEndPosition(unit, tree) >= 0 && !isImplicit(tree, unit)) {
          Decl d = new Decl();
          d.el = (ExecutableElement) e;
          d.file = file;
          d.ordinal = ordinal[0]++;
          d.line = unit.getLineMap().getLineNumber(positions.getStartPosition(unit, tree));
          d.namePos = -1;
          if (e.getKind() == ElementKind.METHOD) {
            String name = tree.getName().toString();
            int from = (int) positions.getEndPosition(unit, tree.getReturnType());
            int to = (int) positions.getEndPosition(unit, tree);
            int at = -1;
            for (int i = text.indexOf(name, from); i >= 0 && i < to; i = text.indexOf(name, i + 1)) {
              int j = i + name.length();
              while (j < to && Character.isWhitespace(text.charAt(j))) {
                j++;
              }
              if (text.charAt(j) == '(' && !Character.isJavaIdentifierPart(text.charAt(i - 1))) {
                at = i;
                break;
              }
            }
            if (at < 0) {
              throw new IllegalStateException("no name for " + e + " in " + file);
            }
            d.namePos = at;
          }
          decls.put(d.el, d);
        }
        return super.visitMethod(tree, p);
      }

      @Override
      public Void visitMethodInvocation(MethodInvocationTree tree, Void p) {
        ExpressionTree sel = tree.getMethodSelect();
        Element e = trees.getElement(new TreePath(getCurrentPath(), sel));
        if (!(e instanceof ExecutableElement)) {
          throw new IllegalStateException("unresolved call " + tree + " in " + file);
        }
        Use u = new Use();
        u.file = file;
        u.target = (ExecutableElement) e;
        u.name = e.getSimpleName().toString();
        u.pos = -1;
        u.at = at(unit, tree);
        if (e.getKind() == ElementKind.METHOD) {
          if (sel instanceof IdentifierTree) {
            u.pos = (int) positions.getStartPosition(unit, sel);
            u.bare = true;
          } else {
            u.pos = (int) positions.getEndPosition(unit, sel) - u.name.length();
          }
          check(text, u, file);
        }
        uses.add(u);
        return super.visitMethodInvocation(tree, p);
      }

      @Override
      public Void visitMemberReference(MemberReferenceTree tree, Void p) {
        Element e = trees.getElement(getCurrentPath());
        if (e instanceof ExecutableElement) {
          Use u = new Use();
          u.file = file;
          u.target = (ExecutableElement) e;
          u.name = e.getSimpleName().toString();
          u.pos = -1;
          if (e.getKind() == ElementKind.METHOD) {
            u.pos = (int) positions.getEndPosition(unit, tree) - u.name.length();
            check(text, u, file);
          }
          uses.add(u);
        } else if (!tree.getName().contentEquals("new")) {
          // Array constructor references (int[]::new) have no symbol; anything else must.
          throw new IllegalStateException("unresolved reference " + tree + " in " + file);
        }
        return super.visitMemberReference(tree, p);
      }

      @Override
      public Void visitNewClass(NewClassTree tree, Void p) {
        Element e = trees.getElement(getCurrentPath());
        if (!(e instanceof ExecutableElement)) {
          throw new IllegalStateException("unresolved new " + tree + " in " + file);
        }
        Use u = new Use();
        u.file = file;
        u.target = (ExecutableElement) e;
        u.name = "<init>";
        u.pos = -1;
        u.at = at(unit, tree);
        u.newStart = (int) positions.getStartPosition(unit, tree);
        u.newTypeEnd = (int) positions.getEndPosition(unit, tree.getIdentifier());
        uses.add(u);
        return super.visitNewClass(tree, p);
      }
    }.scan(unit, null);
  }

  static String at(CompilationUnitTree unit, Tree tree) {
    String text = tree.toString().replaceAll("\\s+", " ");
    return unit.getSourceFile().getName() + ":"
        + unit.getLineMap().getLineNumber(positions.getStartPosition(unit, tree)) + ": "
        + (text.length() > 60 ? text.substring(0, 60) + "..." : text);
  }

  static boolean isImplicit(MethodTree tree, CompilationUnitTree unit) {
    // Default constructors and record members generated by javac carry the class's position.
    return positions.getStartPosition(unit, tree) == positions.getEndPosition(unit, tree);
  }

  static void check(String text, Use u, String file) {
    if (!text.startsWith(u.name, u.pos)) {
      throw new IllegalStateException("name " + u.name + " not at " + u.pos + " in " + file);
    }
  }

  // ---------------------------------------------------------------- families

  static void supers(TypeElement c, Set<TypeElement> out) {
    for (TypeMirror t : types.directSupertypes(c.asType())) {
      Element e = types.asElement(t);
      if (e instanceof TypeElement && out.add((TypeElement) e)) {
        supers((TypeElement) e, out);
      }
    }
  }

  /** Methods of c: declared ones, then the ones it can inherit from its supertypes. */
  static List<ExecutableElement> members(TypeElement c) {
    List<ExecutableElement> all = new ArrayList<>();
    for (Element e : c.getEnclosedElements()) {
      if (e.getKind() == ElementKind.METHOD) {
        all.add((ExecutableElement) e);
      }
    }
    Set<TypeElement> sup = new LinkedHashSet<>();
    supers(c, sup);
    sup.remove(c);
    for (TypeElement s : sup) {
      for (Element e : s.getEnclosedElements()) {
        if (e.getKind() != ElementKind.METHOD || e.getModifiers().contains(Modifier.PRIVATE)) {
          continue;
        }
        if (e.getModifiers().contains(Modifier.STATIC) && s.getKind().isInterface()) {
          continue;
        }
        all.add((ExecutableElement) e);
      }
    }
    return all;
  }

  static String bucketKey(ExecutableElement e) {
    return e.getSimpleName() + "/" + e.getParameters().size();
  }

  static Map<String, List<ExecutableElement>> buckets(TypeElement c) {
    Map<String, List<ExecutableElement>> m = new TreeMap<>();
    for (ExecutableElement e : members(c)) {
      m.computeIfAbsent(bucketKey(e), k -> new ArrayList<>()).add(e);
    }
    return m;
  }

  static boolean related(ExecutableElement a, ExecutableElement b, TypeElement c) {
    try {
      if (elements.overrides(a, b, c) || elements.overrides(b, a, c)) {
        return true;
      }
    } catch (RuntimeException e) {
      // fall through
    }
    try {
      ExecutableType ta = (ExecutableType) types.asMemberOf((DeclaredType) c.asType(), a);
      ExecutableType tb = (ExecutableType) types.asMemberOf((DeclaredType) c.asType(), b);
      return types.isSubsignature(ta, tb) || types.isSubsignature(tb, ta);
    } catch (RuntimeException e) {
      return false;
    }
  }

  static void buildFamilies() {
    for (TypeElement c : sourceClasses) {
      for (List<ExecutableElement> bucket : buckets(c).values()) {
        for (int i = 0; i < bucket.size(); i++) {
          for (int j = i + 1; j < bucket.size(); j++) {
            ExecutableElement a = bucket.get(i);
            ExecutableElement b = bucket.get(j);
            if (find(a) != find(b) && related(a, b, c)) {
              union(a, b);
            }
          }
        }
      }
    }
    for (TypeElement c : sourceClasses) {
      for (ExecutableElement e : members(c)) {
        familyMembers.computeIfAbsent(find(e), k -> new LinkedHashSet<>()).add(e);
      }
    }
  }

  static final Map<ExecutableElement, Set<ExecutableElement>> familyMembers = new IdentityHashMap<>();

  /** A family that cannot be renamed: it has a member that is not a method written in our sources. */
  static boolean fixed(ExecutableElement root) {
    for (ExecutableElement e : familyMembers.get(root)) {
      if (!decls.containsKey(e)) {
        return true;
      }
    }
    return false;
  }

  static int familyUses(ExecutableElement root) {
    int n = 0;
    for (ExecutableElement e : familyMembers.get(root)) {
      Decl d = decls.get(e);
      n += d == null ? 0 : d.uses;
    }
    return n;
  }

  static String erasedParams(ExecutableElement e) {
    List<String> l = new ArrayList<>();
    for (VariableElement v : e.getParameters()) {
      l.add(types.erasure(v.asType()).toString().replaceAll("@\\S+ ", ""));
    }
    return "(" + String.join(",", l) + ")";
  }

  static String sig(ExecutableElement e) {
    return ((TypeElement) e.getEnclosingElement()).getQualifiedName() + "#" + e.getSimpleName() + erasedParams(e);
  }

  static String binarySig(ExecutableElement e) {
    return elements.getBinaryName((TypeElement) e.getEnclosingElement()) + "#" + e.getSimpleName() + erasedParams(e);
  }

  static String where(ExecutableElement e) {
    Decl d = decls.get(e);
    return d == null ? "(library/implicit)" : d.file + ":" + d.line;
  }

  // ---------------------------------------------------------------- report

  static final class Group {
    String key;
    List<ExecutableElement> roots;
    List<TypeElement> classes = new ArrayList<>();
  }

  static Map<String, Group> groups() {
    Map<String, Group> groups = new TreeMap<>();
    for (TypeElement c : sourceClasses) {
      for (Map.Entry<String, List<ExecutableElement>> b : buckets(c).entrySet()) {
        Set<ExecutableElement> roots = new LinkedHashSet<>();
        for (ExecutableElement e : b.getValue()) {
          if (decls.containsKey(e) || sourceClasses.contains(e.getEnclosingElement())) {
            roots.add(find(e));
          }
        }
        if (roots.size() > 1) {
          List<ExecutableElement> sorted = new ArrayList<>(roots);
          sorted.sort(Comparator.comparing(Scan::familyKey));
          String key = b.getKey() + " " + sorted.stream().map(Scan::familyKey).collect(Collectors.joining(" | "));
          Group g = groups.computeIfAbsent(key, k -> new Group());
          g.key = b.getKey();
          g.roots = sorted;
          g.classes.add(c);
        }
      }
    }
    return groups;
  }

  /** The declaration that names a family: the most called one, then the first by signature. */
  static ExecutableElement representative(ExecutableElement root) {
    ExecutableElement best = null;
    for (ExecutableElement e : familyMembers.get(root)) {
      if (!decls.containsKey(e)) {
        continue;
      }
      if (best == null
          || decls.get(e).uses > decls.get(best).uses
          || (decls.get(e).uses == decls.get(best).uses && binarySig(e).compareTo(binarySig(best)) < 0)) {
        best = e;
      }
    }
    return best == null ? root : best;
  }

  static final Map<ExecutableElement, String> familyKeys = new IdentityHashMap<>();

  static String familyKey(ExecutableElement root) {
    return familyKeys.computeIfAbsent(root, r -> binarySig(representative(r)));
  }

  static int report() {
    int bad = 0;
    int exempt = 0;
    Set<ExecutableElement> methods = new HashSet<>();
    for (Group g : groups().values()) {
      boolean ex = g.roots.stream().allMatch(Scan::fixed);
      if (ex) {
        exempt++;
      } else {
        bad++;
      }
      System.out.println(
          (ex ? "EXEMPT " : "GROUP ") + g.key + " in " + g.classes.size() + " class(es): "
              + g.classes.stream().limit(3).map(c -> elements.getBinaryName(c).toString()).collect(Collectors.joining(", "))
              + (g.classes.size() > 3 ? ", ..." : ""));
      for (ExecutableElement r : g.roots) {
        System.out.println("  family" + (fixed(r) ? " [library]" : "") + " uses=" + familyUses(r));
        for (ExecutableElement e : familyMembers.get(r)) {
          if (decls.containsKey(e)) {
            System.out.println("    " + binarySig(e) + "  " + where(e));
            if (!ex) {
              methods.add(e);
            }
          } else if (!sourceClasses.contains(e.getEnclosingElement())) {
            System.out.println("    " + binarySig(e) + "  (library)");
          }
        }
      }
    }
    int ctorGroups = 0;
    int ctors = 0;
    for (TypeElement c : sourceClasses) {
      Map<Integer, List<ExecutableElement>> byArity = new TreeMap<>();
      for (Element e : c.getEnclosedElements()) {
        if (e.getKind() == ElementKind.CONSTRUCTOR) {
          byArity.computeIfAbsent(((ExecutableElement) e).getParameters().size(), k -> new ArrayList<>())
              .add((ExecutableElement) e);
        }
      }
      for (Map.Entry<Integer, List<ExecutableElement>> b : byArity.entrySet()) {
        if (b.getValue().size() > 1) {
          ctorGroups++;
          ctors += b.getValue().size();
          System.out.println("CTOR " + elements.getBinaryName(c) + "/" + b.getKey());
          for (ExecutableElement e : b.getValue()) {
            Decl d = decls.get(e);
            System.out.println("    " + binarySig(e) + "  " + where(e) + " uses=" + (d == null ? 0 : d.uses));
            for (List<Use> l : usesByFile.values()) {
              for (Use u : l) {
                if (u.target == e) {
                  System.out.println("        used at " + u.at);
                  // Machine-readable, for tools/prep/07_ctor_factories.py.
                  System.out.println("CTORUSE\t" + binarySig(e) + "\t" + u.file + "\t" + u.newStart + "\t" + u.newTypeEnd);
                }
              }
            }
          }
        }
      }
    }
    System.out.println(
        "method groups: " + bad + " (declarations in them: " + methods.size() + "), exempt library-override groups: "
            + exempt + ", constructor groups: " + ctorGroups + " (constructors in them: " + ctors + ")");
    return bad + ctorGroups;
  }

  // ---------------------------------------------------------------- dump

  static String declKey(ExecutableElement e) {
    Decl d = decls.get(e);
    if (d != null) {
      return d.file + "#" + d.ordinal;
    }
    String owner = elements.getBinaryName((TypeElement) e.getEnclosingElement()).toString();
    boolean ours = owner.startsWith("com.google.javascript.") || owner.startsWith("com.google.debugging.");
    // Methods of generated classes (AutoValue_*) follow the abstract methods they implement.
    boolean generated = ours && !sourceClasses.contains(e.getEnclosingElement());
    return "LIB:" + owner + "#" + (generated ? "?" : e.getSimpleName()) + erasedParams(e);
  }

  static void dump(String path) throws Exception {
    try (PrintWriter w = new PrintWriter(path, StandardCharsets.UTF_8)) {
      for (Decl d : decls.values()) {
        if (d.el.getKind() != ElementKind.METHOD) {
          continue;
        }
        TreeSet<String> fam = new TreeSet<>();
        for (ExecutableElement e : familyMembers.get(find(d.el))) {
          fam.add(declKey(e));
        }
        w.println("F\t" + declKey(d.el) + "\t" + String.join(" ", fam));
      }
      for (Map.Entry<String, List<Use>> f : usesByFile.entrySet()) {
        int i = 0;
        for (Use u : f.getValue()) {
          w.println("U\t" + f.getKey() + "\t" + (i++) + "\t" + declKey(u.target));
        }
      }
    }
  }

  // ---------------------------------------------------------------- rename

  static String typeWord(TypeMirror t, boolean varargs) {
    t = types.erasure(t);
    if (t.getKind() == TypeKind.ARRAY) {
      return typeWord(((ArrayType) t).getComponentType(), false) + (varargs ? "Varargs" : "Array");
    }
    if (t.getKind().isPrimitive()) {
      String s = t.toString();
      return Character.toUpperCase(s.charAt(0)) + s.substring(1);
    }
    return types.asElement(t).getSimpleName().toString();
  }

  static String word(ExecutableElement e, int i) {
    return typeWord(e.getParameters().get(i).asType(), e.isVarArgs() && i == e.getParameters().size() - 1);
  }

  static void rename(boolean apply) throws Exception {
    Map<String, String> table = new HashMap<>();
    Path names = Paths.get("tools/prep/07_find_same_arity_overloads/names.tsv");
    if (Files.exists(names)) {
      for (String line : Files.readAllLines(names)) {
        if (!line.isBlank() && !line.startsWith("#")) {
          String[] p = line.split("\t");
          table.put(p[0], p[1]);
        }
      }
    }
    // Which families meet in some class.
    Map<ExecutableElement, Set<ExecutableElement>> meets = new IdentityHashMap<>();
    for (Group g : groups().values()) {
      for (ExecutableElement a : g.roots) {
        for (ExecutableElement b : g.roots) {
          if (a != b) {
            meets.computeIfAbsent(a, k -> Collections.newSetFromMap(new IdentityHashMap<>())).add(b);
          }
        }
      }
    }
    List<ExecutableElement> order = new ArrayList<>(meets.keySet());
    order.sort(
        Comparator.comparing((ExecutableElement r) -> !fixed(r))
            .thenComparing(r -> forcedKeep(r, table) ? 0 : 1)
            .thenComparing(r -> -familyUses(r))
            .thenComparing(r -> -familyMembers.get(r).size())
            .thenComparing(Scan::familyKey));
    Set<ExecutableElement> kept = Collections.newSetFromMap(new IdentityHashMap<>());
    Map<ExecutableElement, String> newNames = new IdentityHashMap<>();
    List<String> planLines = new ArrayList<>();
    for (ExecutableElement r : order) {
      boolean clash = false;
      for (ExecutableElement o : meets.get(r)) {
        clash |= kept.contains(o);
      }
      if (!clash || fixed(r)) {
        kept.add(r);
        continue;
      }
      ExecutableElement rep = representative(r);
      String name = rep.getSimpleName().toString();
      String newName = null;
      for (ExecutableElement e : familyMembers.get(r)) {
        String k = binarySig(e);
        if (table.containsKey(k)) {
          newName = table.get(k);
        }
      }
      if (newName == null) {
        int n = rep.getParameters().size();
        List<ExecutableElement> others = new ArrayList<>();
        for (ExecutableElement o : meets.get(r)) {
          others.add(representative(o));
        }
        int single = -1;
        StringBuilder all = new StringBuilder();
        for (int i = 0; i < n; i++) {
          boolean fromAll = true;
          boolean fromAny = false;
          for (ExecutableElement o : others) {
            boolean differs = !word(o, i).equals(word(rep, i));
            fromAll &= differs;
            fromAny |= differs;
          }
          if (fromAll && single < 0) {
            single = i;
          }
          if (fromAny) {
            all.append(word(rep, i));
          }
        }
        newName = name + (single >= 0 ? word(rep, single) : all.toString());
      }
      newNames.put(r, newName);
      planLines.add(
          familyKey(r) + "\t" + newName + "\tuses=" + familyUses(r) + "\tdecls="
              + familyMembers.get(r).stream().filter(decls::containsKey).count());
    }
    Collections.sort(planLines);
    for (String l : planLines) {
      System.out.println("RENAME\t" + l);
    }
    // Simulate: no class may now hold two families under one new name and arity.
    int conflicts = 0;
    for (TypeElement c : sourceClasses) {
      Map<String, Set<ExecutableElement>> m = new TreeMap<>();
      for (ExecutableElement e : members(c)) {
        ExecutableElement r = find(e);
        String nn = newNames.getOrDefault(r, e.getSimpleName().toString());
        m.computeIfAbsent(nn + "/" + e.getParameters().size(), k -> Collections.newSetFromMap(new IdentityHashMap<>())).add(r);
      }
      for (Map.Entry<String, Set<ExecutableElement>> en : m.entrySet()) {
        if (en.getValue().size() > 1 && en.getValue().stream().anyMatch(newNames::containsKey)) {
          conflicts++;
          System.out.println(
              "CONFLICT after rename in " + elements.getBinaryName(c) + ": " + en.getKey() + " <- "
                  + en.getValue().stream().map(Scan::familyKey).collect(Collectors.joining(" | ")));
        }
      }
    }
    System.out.println("families renamed: " + newNames.size() + ", conflicts: " + conflicts);
    if (!apply) {
      return;
    }
    if (conflicts > 0) {
      throw new IllegalStateException("resolve the conflicts in names.tsv first");
    }
    // Edits: file -> offset -> {old, new}; inserts for static imports.
    Map<String, TreeMap<Integer, String[]>> edits = new HashMap<>();
    int declEdits = 0;
    int useEdits = 0;
    int importEdits = 0;
    for (Decl d : decls.values()) {
      String nn = d.el.getKind() == ElementKind.METHOD ? newNames.get(find(d.el)) : null;
      if (nn != null) {
        edits.computeIfAbsent(d.file, k -> new TreeMap<>()).put(d.namePos, new String[] {d.el.getSimpleName().toString(), nn});
        declEdits++;
      }
    }
    for (List<Use> l : usesByFile.values()) {
      for (Use u : l) {
        String nn = u.pos >= 0 ? newNames.get(find(u.target)) : null;
        if (nn != null) {
          edits.computeIfAbsent(u.file, k -> new TreeMap<>()).put(u.pos, new String[] {u.name, nn});
          useEdits++;
        }
      }
    }
    for (StaticImport si : staticImports) {
      TreeSet<String> add = new TreeSet<>();
      boolean keepOld = false;
      for (Element e : elements.getAllMembers(si.cls)) {
        if (e.getSimpleName().contentEquals(si.name) && e.getKind() != ElementKind.METHOD) {
          keepOld = true;
        }
      }
      for (Use u : usesByFile.get(si.file)) {
        if (!u.bare || !u.name.equals(si.name) || !u.target.getModifiers().contains(Modifier.STATIC)) {
          continue;
        }
        if (!types.isSubtype(types.erasure(si.cls.asType()), types.erasure(u.target.getEnclosingElement().asType()))) {
          continue;
        }
        String nn = newNames.get(find(u.target));
        if (nn == null) {
          keepOld = true;
        } else {
          add.add(nn);
        }
      }
      if (add.isEmpty()) {
        continue;
      }
      String text = sources.get(si.file);
      String line = text.substring(si.start, si.end);
      StringBuilder repl = new StringBuilder();
      if (keepOld) {
        repl.append(line);
      }
      for (String nn : add) {
        String l = line.substring(0, line.lastIndexOf('.') + 1) + nn + ";";
        if (!text.contains(l)) {
          repl.append(repl.length() > 0 ? "\n" : "").append(l);
        }
      }
      if (repl.length() == 0) {
        throw new IllegalStateException("static import of " + si.name + " in " + si.file);
      }
      edits.computeIfAbsent(si.file, k -> new TreeMap<>()).put(si.start, new String[] {line, repl.toString()});
      importEdits++;
    }
    for (Map.Entry<String, TreeMap<Integer, String[]>> f : edits.entrySet()) {
      StringBuilder text = new StringBuilder(sources.get(f.getKey()));
      for (Map.Entry<Integer, String[]> e : f.getValue().descendingMap().entrySet()) {
        int at = e.getKey();
        String old = e.getValue()[0];
        if (!text.substring(at, at + old.length()).equals(old)) {
          throw new IllegalStateException("edit mismatch in " + f.getKey() + " at " + at);
        }
        text.replace(at, at + old.length(), e.getValue()[1]);
      }
      Files.writeString(Paths.get(f.getKey()), text.toString(), StandardCharsets.UTF_8);
    }
    System.out.println(
        "applied: files=" + edits.size() + ", declarations=" + declEdits + ", call sites=" + useEdits
            + ", static imports=" + importEdits);
  }

  static boolean forcedKeep(ExecutableElement r, Map<String, String> table) {
    for (ExecutableElement e : familyMembers.get(r)) {
      String v = table.get(binarySig(e));
      if (v != null && v.equals(e.getSimpleName().toString())) {
        return true;
      }
    }
    return false;
  }
}
