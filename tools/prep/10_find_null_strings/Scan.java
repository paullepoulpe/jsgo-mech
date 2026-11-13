// Prep step 10 scanner: a String is never null.
//
// Attributes java/src and java/test (one javac task, same flags as tools/gate0/build.sh) and
// reports, per file:line, every place where an expression of static type java.lang.String is
// treated as possibly null:
//
//   nullable-decl   (1) a field, parameter, local, record component, lambda parameter or method
//                       return of type String annotated with an annotation named Nullable, and a
//                       `@Nullable String` type use anywhere else (type argument, array element);
//   null-literal    (2) a `null` literal (parentheses and casts looked through) converted to
//                       String: initialiser of / assignment to a String variable, returned from a
//                       method or lambda whose return type is String, argument for a parameter
//                       whose (instantiated) type is String - of any method or constructor,
//                       library ones included -, a branch of a conditional expression of type
//                       String, an element of a String[] initialiser, a `(String) null` cast;
//   null-check      (3) `e == null` / `e != null` where e has static type String, and calls of
//                       java.util.Objects.isNull/nonNull/requireNonNull/requireNonNullElse/
//                       requireNonNullElseGet/equals/hashCode,
//                       com.google.common.base.Strings.isNullOrEmpty/nullToEmpty/emptyToNull,
//                       com.google.common.base.Preconditions.checkNotNull, Verify.verifyNotNull,
//                       com.google.common.base.Objects.equal and MoreObjects.firstNonNull with a
//                       String-typed argument;
//   of-nullable     (4) `Optional.ofNullable(e)` (java.util or Guava `fromNullable`) where e has
//                       static type String and is NOT directly a call of a library method or
//                       of a source method declared to return a type variable (`T get()` of a
//                       container): ofNullable is allowed only at the boundary where a library
//                       or a generic container hands out a null.
//
//   unguarded-get   (5) `X.get()` on an Optional<String> that is neither
//                       * under a presence check of the same expression text in the same method
//                         (inside `if (X.isPresent())`, after `if (X.isEmpty()) return/throw/
//                         continue/break`, right of `X.isPresent() &&` or `X.isEmpty() ||`, in
//                         the matching branch of a conditional, in a `while (X.isPresent())`
//                         body, after checkState/checkArgument/verify(X.isPresent()), after a
//                         statement `X = Optional.of(..)`; `X.orElse("").isEmpty()` counts as
//                         `X.isEmpty()` for a non-empty value), nor
//                       * a read of a final or effectively final variable initialised with
//                         `Optional.of(..)`, nor `Optional.of(..).get()` itself, nor
//                       * dereferenced at once (`X.get().foo()`), or passed for a parameter that
//                         the called source method dereferences (`p.foo()`, `switch (p)`, hands
//                         to a library method that rejects null, or passes on to a parameter
//                         that does), or passed to such a library method (Guava Immutable*
//                         builders and Interner, String methods, Splitter, Pattern.matcher,
//                         Path.of, StringReader, URI): there the code before this step
//                         threw a NullPointerException for an absent value, so nothing changed.
//                       The finding carries the key that reviewed.txt uses for the site:
//                       enclosing class#method :: text of X :: callee.
//                       Convention: a plain `get()` is one of the three provable forms above; a
//                       presence that was established by reading the code is written
//                       `X.orElseThrow()` (same behaviour, marks a reviewed assertion). The
//                       summary counts each form.
//   unproven-of     (6) `Optional.of(e)` where e is a call of a library method that is not known
//                       to return non-null (java.lang, Guava string helpers, toString()): it
//                       needs `Optional.ofNullable`; or where e is a String variable or a call of
//                       a source method that is "tainted": its value can come (through
//                       assignments, arguments and returns, transitively) from a null literal, an
//                       `X.orElse(null)`, or a library call known to return null (Map.get,
//                       Matcher.group, readLine, getProperty, ...). The summary also counts the other forms:
//                       e proven non-null (literal, concatenation, `X.get()` / `orElseThrow()` /
//                       `orElse(literal)`, constant, nonnull.txt entry), and e a String parameter
//                       / local, field, or source-method result - non-null by its declared type,
//                       which is the invariant this step establishes; --instrument probes those
//                       too.
//
// --instrument rewrites the production sources in place for a measurement run (never commit the
// result): every unguarded get becomes `NullProbe.get(X, "File.java:line")` and every unproven of
// `NullProbe.of(e, "File.java:line")`, where com.google.javascript.jscomp.base.NullProbe (written
// by this mode) records the site when the value is absent / null and carries on with null /
// Optional.empty() as the code before this step did; the recorded sites are appended to
// $NULLPROBE_OUT (default /tmp/nullprobe.txt) at JVM exit.
//
// Production code is java/src without */testing/; everything else is counted as "test" and only
// summarised (use --tests to list those findings too). A finding is EXEMPT when exempt.txt (next
// to this file) has a line that is either
//   lib:<owner class>#<method>      a library method that genuinely takes a null String argument
//   <path>::<trimmed source line>   one specific line
// (text after " ## " is the justification). Exit status 1 if a non-exempt production finding
// exists.
//
// With --info, also lists (not counted) calls of library methods that are known to return a null
// String (Map.get, Matcher.group, BufferedReader.readLine, System.getProperty/getenv, ...) and
// whose result is not passed straight to Optional.ofNullable.
//
// Run from the repo root after tools/gate0/build.sh (JDK 21; attribution needs the libraries):
//   X="--add-exports=jdk.compiler/com.sun.tools.javac.tree=ALL-UNNAMED
//      --add-exports=jdk.compiler/com.sun.tools.javac.code=ALL-UNNAMED
//      --add-exports=jdk.compiler/com.sun.tools.javac.api=ALL-UNNAMED
//      --add-exports=jdk.compiler/com.sun.tools.javac.util=ALL-UNNAMED"
//   javac $X -d /tmp/scan10 tools/prep/10_find_null_strings/Scan.java
//   java $X -Xss16m -cp /tmp/scan10 Scan [--tests] [--info] [--summary] [--instrument] \
//       ".cache/build/stock-libs.jar:$(ls .cache/deps/*.jar | tr '\n' ':')" java/src java/test

import com.sun.source.tree.AnnotatedTypeTree;
import com.sun.source.tree.AnnotationTree;
import com.sun.source.tree.AssignmentTree;
import com.sun.source.tree.BinaryTree;
import com.sun.source.tree.CompilationUnitTree;
import com.sun.source.tree.ConditionalExpressionTree;
import com.sun.source.tree.ExpressionTree;
import com.sun.source.tree.LambdaExpressionTree;
import com.sun.source.tree.MethodInvocationTree;
import com.sun.source.tree.MethodTree;
import com.sun.source.tree.NewArrayTree;
import com.sun.source.tree.NewClassTree;
import com.sun.source.tree.ReturnTree;
import com.sun.source.tree.Tree;
import com.sun.source.tree.TypeCastTree;
import com.sun.source.tree.VariableTree;
import com.sun.source.util.JavacTask;
import com.sun.source.util.SourcePositions;
import com.sun.source.util.TreePath;
import com.sun.source.util.TreePathScanner;
import com.sun.source.util.Trees;
import com.sun.tools.javac.api.BasicJavacTask;
import com.sun.tools.javac.code.Symbol;
import com.sun.tools.javac.code.Type;
import com.sun.tools.javac.code.Types;
import com.sun.tools.javac.tree.JCTree;
import com.sun.tools.javac.tree.TreeInfo;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.HashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import javax.tools.Diagnostic;
import javax.tools.JavaCompiler;
import javax.tools.JavaFileObject;
import javax.tools.StandardJavaFileManager;
import javax.tools.ToolProvider;

public final class Scan {
  private static final Set<String> CHECK_CALLS =
      Set.of(
          "java.util.Objects#isNull",
          "java.util.Objects#nonNull",
          "java.util.Objects#requireNonNull",
          "java.util.Objects#requireNonNullElse",
          "java.util.Objects#requireNonNullElseGet",
          "java.util.Objects#equals",
          "java.util.Objects#hashCode",
          "com.google.common.base.Strings#isNullOrEmpty",
          "com.google.common.base.Strings#nullToEmpty",
          "com.google.common.base.Strings#emptyToNull",
          "com.google.common.base.Preconditions#checkNotNull",
          "com.google.common.base.Verify#verifyNotNull",
          "com.google.common.base.Objects#equal",
          "com.google.common.base.MoreObjects#firstNonNull");
  private static final Set<String> OF_NULLABLE =
      Set.of("java.util.Optional#ofNullable", "com.google.common.base.Optional#fromNullable");
  private static final Set<String> NULL_RETURNING =
      Set.of(
          "java.util.Map#get",
          "java.util.Map#remove",
          "java.util.Map#put",
          "java.util.HashMap#get",
          "java.util.LinkedHashMap#get",
          "java.util.TreeMap#get",
          "com.google.common.collect.ImmutableMap#get",
          "com.google.re2j.Matcher#group",
          "java.io.BufferedReader#readLine",
          "java.lang.System#getProperty",
          "java.lang.System#getenv",
          "java.util.Properties#getProperty",
          "java.util.Deque#peek",
          "java.util.Deque#poll",
          "java.util.Queue#peek",
          "java.util.Queue#poll",
          "java.nio.file.Path#getFileName");

  private static Types types;
  private static com.sun.tools.javac.util.Context context;
  private static Trees trees;
  private static SourcePositions positions;
  private static final Set<String> exemptLibs = new HashSet<>();
  private static final Set<String> exemptLines = new HashSet<>();
  private static final Set<String> usedExemptions = new HashSet<>();

  private record Finding(String file, long line, String category, String message, String exempt) {}

  private static final List<Finding> findings = new ArrayList<>();
  private static final List<String> infos = new ArrayList<>();
  private static final Map<String, int[]> getForms = new TreeMap<>();
  private static final Set<String> nonNull = new HashSet<>();
  private static boolean instrument;

  private record Edit(int start, int end, String text) {}

  public static void main(String[] argv) throws Exception {
    List<String> args = new ArrayList<>(List.of(argv));
    boolean listTests = args.remove("--tests");
    boolean info = args.remove("--info");
    instrument = args.remove("--instrument");
    Path nonNullFile = Paths.get("tools/prep/10_find_null_strings/nonnull.txt");
    if (Files.exists(nonNullFile)) {
      for (String line : Files.readAllLines(nonNullFile, StandardCharsets.UTF_8)) {
        int why = line.indexOf(" ## ");
        String key = (why >= 0 ? line.substring(0, why) : line).trim();
        if (!key.isEmpty() && !key.startsWith("#")) {
          nonNull.add(key);
        }
      }
    }
    boolean summaryOnly = args.remove("--summary");
    String classpath = args.remove(0);
    List<File> files = new ArrayList<>();
    for (String root : args) {
      List<Path> found = new ArrayList<>();
      try (var paths = Files.walk(Paths.get(root))) {
        paths.filter(p -> p.toString().endsWith(".java")).forEach(found::add);
      }
      found.sort(null);
      for (Path p : found) {
        files.add(p.toFile());
      }
    }
    Path exemptFile = Paths.get("tools/prep/10_find_null_strings/exempt.txt");
    if (Files.exists(exemptFile)) {
      for (String line : Files.readAllLines(exemptFile, StandardCharsets.UTF_8)) {
        int why = line.indexOf(" ## ");
        String key = (why >= 0 ? line.substring(0, why) : line).trim();
        if (key.isEmpty() || key.startsWith("#")) {
          continue;
        }
        if (key.startsWith("lib:")) {
          exemptLibs.add(key.substring(4));
        } else {
          exemptLines.add(key);
        }
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
    trees = Trees.instance(task);
    positions = trees.getSourcePositions();
    List<CompilationUnitTree> parsed = new ArrayList<>();
    task.parse().forEach(parsed::add);
    task.analyze();
    if (errors[0] > 0) {
      System.err.println("sources do not compile: " + errors[0] + " errors");
      System.exit(2);
    }
    types = Types.instance(((BasicJavacTask) task).getContext());
    context = ((BasicJavacTask) task).getContext();
    do {
      taintGrew = false;
      for (CompilationUnitTree unit : parsed) {
        if (isProduction(unit.getSourceFile().getName())) {
          new DerefCollector().scan(unit, null);
        }
      }
    } while (taintGrew);
    for (CompilationUnitTree unit : parsed) {
      if (!isProduction(unit.getSourceFile().getName())) {
        new DerefCollector().scan(unit, null);
      }
    }
    boolean grew = true;
    while (grew) {
      grew = false;
      // A parameter of an overridden method is dereferenced when an override dereferences it.
      for (Symbol p : new ArrayList<>(derefParams)) {
        if (p.owner instanceof Symbol.MethodSymbol m
            && !m.isStatic()
            && !m.isConstructor()
            && m.owner instanceof Symbol.ClassSymbol c
            && m.params != null) {
          int i = m.params.indexOf(p);
          for (Type sup : types.closure(c.type)) {
            if (sup.tsym == c) {
              continue;
            }
            for (Symbol o : sup.tsym.members().getSymbolsByName(m.name)) {
              if (o instanceof Symbol.MethodSymbol other
                  && !isLibrary(other)
                  && other.params != null
                  && i >= 0
                  && i < other.params.size()
                  && m.overrides(other, c, types, false)) {
                grew |= derefParams.add(other.params.get(i));
              }
            }
          }
        }
      }
      for (Map.Entry<Symbol, Set<Symbol>> e : passedOn.entrySet()) {
        if (!derefParams.contains(e.getKey())) {
          for (Symbol q : e.getValue()) {
            if (derefParams.contains(q)) {
              grew |= derefParams.add(e.getKey());
              break;
            }
          }
        }
      }
    }
    for (CompilationUnitTree unit : parsed) {
      Visitor v = new Visitor(unit, info);
      v.scan(unit, null);
      v.applyEdits();
    }
    if (instrument) {
      Files.writeString(
          Paths.get("java/src/com/google/javascript/jscomp/base/NullProbe.java"), NULL_PROBE);
    }

    Map<String, int[]> byCategory = new TreeMap<>();
    Map<String, int[]> byPackage = new TreeMap<>();
    int prod = 0;
    int exempt = 0;
    int test = 0;
    for (Finding f : findings) {
      boolean isProd = isProduction(f.file);
      int[] c = byCategory.computeIfAbsent(f.category, k -> new int[3]);
      if (!isProd) {
        test++;
        c[2]++;
        if (listTests && !summaryOnly) {
          System.out.println(f.file + ":" + f.line + ": [test " + f.category + "] " + f.message);
        }
        continue;
      }
      if (f.exempt != null) {
        exempt++;
        c[1]++;
        if (!summaryOnly) {
          System.out.println(
              f.file + ":" + f.line + ": [EXEMPT " + f.category + "] " + f.message);
        }
        continue;
      }
      prod++;
      c[0]++;
      String pkg = f.file.substring(0, f.file.lastIndexOf('/'));
      pkg = pkg.substring(pkg.indexOf("com/google/"));
      byPackage.computeIfAbsent(pkg, k -> new int[1])[0]++;
      if (!summaryOnly) {
        System.out.println(f.file + ":" + f.line + ": [" + f.category + "] " + f.message);
      }
    }
    if (info) {
      for (String s : infos) {
        System.out.println(s);
      }
      System.out.println("info (production, not counted): " + infos.size());
    }
    for (String key : exemptLines) {
      if (!usedExemptions.contains(key)) {
        System.out.println("UNUSED EXEMPTION: " + key);
      }
    }
    for (String key : exemptLibs) {
      if (!usedExemptions.contains("lib:" + key)) {
        System.out.println("UNUSED EXEMPTION: lib:" + key);
      }
    }
    System.out.println("Optional<String> reads in production code:");
    for (Map.Entry<String, int[]> e : getForms.entrySet()) {
      System.out.println("  " + e.getKey() + ": " + e.getValue()[0]);
    }
    int taintedMethods = 0;
    for (Symbol t : tainted) {
      if (t instanceof Symbol.MethodSymbol) {
        taintedMethods++;
      }
    }
    System.out.println(
        "  String variables that can hold a value from a source of nulls (tainted): "
            + (tainted.size() - taintedMethods)
            + ", source methods that can return one: "
            + taintedMethods);
    System.out.println("category: production / exempt / test");
    for (Map.Entry<String, int[]> e : byCategory.entrySet()) {
      int[] c = e.getValue();
      System.out.println("  " + e.getKey() + ": " + c[0] + " / " + c[1] + " / " + c[2]);
    }
    System.out.println("production findings by package:");
    for (Map.Entry<String, int[]> e : byPackage.entrySet()) {
      System.out.println("  " + e.getKey() + ": " + e.getValue()[0]);
    }
    System.out.println(
        "null strings: production " + prod + ", exempt " + exempt + ", test " + test);
    System.exit(prod > 0 ? 1 : 0);
  }

  private static final String NULL_PROBE =
      """
      package com.google.javascript.jscomp.base;

      import java.io.FileWriter;
      import java.util.Optional;
      import java.util.Set;
      import java.util.concurrent.ConcurrentHashMap;

      /** Measurement only (Scan.java --instrument); never committed. */
      public final class NullProbe {
        private static final Set<String> sites = ConcurrentHashMap.newKeySet();

        static {
          Runtime.getRuntime().addShutdownHook(new Thread(NullProbe::dump));
        }

        private static void dump() {
          if (sites.isEmpty()) {
            return;
          }
          String out = System.getenv("NULLPROBE_OUT");
          try (FileWriter w = new FileWriter(out == null ? "/tmp/nullprobe.txt" : out, true)) {
            StringBuilder sb = new StringBuilder();
            for (String s : sites) {
              sb.append(s).append('\\n');
            }
            w.write(sb.toString());
          } catch (Exception e) {
            // measurement only
          }
        }

        public static String get(Optional<String> value, String site) {
          if (value.isEmpty()) {
            sites.add("GET " + site);
            return null;
          }
          return value.get();
        }

        public static Optional<String> of(String value, String site) {
          if (value == null) {
            sites.add("OF " + site);
          }
          return Optional.ofNullable(value);
        }
      }
      """;

  private static boolean isOptionalOfString(Tree tree) {
    if (!(tree instanceof JCTree jc) || jc.type == null || jc.type.tsym == null) {
      return false;
    }
    return jc.type.tsym.getQualifiedName().contentEquals("java.util.Optional")
        && jc.type.getTypeArguments().size() == 1
        && isString(jc.type.getTypeArguments().get(0));
  }

  private static ExpressionTree parens(ExpressionTree e) {
    while (e instanceof JCTree.JCParens p) {
      e = p.expr;
    }
    return e;
  }

  /** String parameters that their method dereferences (`p.foo()`, `switch (p)`), transitively. */
  private static final Set<Symbol> derefParams = new HashSet<>();

  private static final Map<Symbol, Set<Symbol>> passedOn = new java.util.HashMap<>();

  private static Symbol stringParameter(ExpressionTree e) {
    e = parens(e);
    if (e instanceof com.sun.source.tree.IdentifierTree
        && TreeInfo.symbol((JCTree) e) instanceof Symbol.VarSymbol v
        && isString(v.type)
        && (v.flags() & com.sun.tools.javac.code.Flags.PARAMETER) != 0) {
      return v;
    }
    return null;
  }

  /** The declared parameter for argument i of a source method, else null. */
  private static Symbol declaredParameter(Symbol sym, int i) {
    if (!(sym instanceof Symbol.MethodSymbol m) || isLibrary(m) || m.params == null) {
      return null;
    }
    if (i >= m.params.size() || (m.isVarArgs() && i >= m.params.size() - 1)) {
      return null;
    }
    return m.params.get(i);
  }

  /** Library methods that throw a NullPointerException for a null (String) argument. */
  private static boolean rejectsNull(Symbol callee) {
    if (callee == null || callee.owner == null || !isLibrary(callee)) {
      return false;
    }
    String owner = callee.owner.getQualifiedName().toString();
    String name = callee.name.toString();
    if (name.equals("contains") || name.equals("containsKey") || name.equals("equals")
        || name.equals("get") || name.equals("indexOf") && !owner.equals("java.lang.String")) {
      return false;
    }
    return owner.startsWith("com.google.common.collect.Immutable")
        || owner.equals("com.google.common.collect.Interner")
        || owner.equals("java.lang.String")
        || owner.equals("com.google.common.base.Splitter")
        || owner.equals("java.io.StringReader")
        || owner.equals("java.net.URI")
        || (owner.equals("com.google.re2j.Pattern") && name.equals("matcher"))
        || (owner.equals("java.nio.file.Path") && name.equals("of"))
        || (owner.equals("com.google.common.base.Preconditions") && name.equals("checkNotNull"));
  }

  /**
   * String variables and source methods whose value can come from a source of nulls that this
   * step has not closed: a null literal, `X.orElse(null)`, a library call known to return null
   * (Map.get, Matcher.group, readLine, getProperty, ...), or another tainted variable / method.
   */
  private static final Set<Symbol> tainted = new HashSet<>();

  private static boolean taintGrew;

  private static boolean taintedExpression(ExpressionTree e) {
    if (e == null) {
      return false;
    }
    e = parens(e);
    if (e.getKind() == Tree.Kind.NULL_LITERAL) {
      return true;
    }
    if (e instanceof ConditionalExpressionTree c) {
      return taintedExpression(c.getTrueExpression()) || taintedExpression(c.getFalseExpression());
    }
    if (e instanceof JCTree.JCMethodInvocation call) {
      Symbol sym = TreeInfo.symbol(call.meth);
      if (sym == null || !isString(call.type)) {
        return false;
      }
      if (sym.name.contentEquals("orElse") && call.args.size() == 1 && isNull(call.args.get(0))) {
        return true;
      }
      return isLibrary(sym)
          ? NULL_RETURNING.contains(key(sym)) || Visitor.isMapGet(sym)
          : tainted.contains(sym);
    }
    if (e instanceof com.sun.source.tree.IdentifierTree
        || e instanceof com.sun.source.tree.MemberSelectTree) {
      return tainted.contains(TreeInfo.symbol((JCTree) e));
    }
    return false;
  }

  private static void taint(Symbol sym, ExpressionTree source) {
    if (sym != null && isString(sym instanceof Symbol.MethodSymbol m ? m.getReturnType() : sym.type)
        && taintedExpression(source)) {
      taintGrew |= tainted.add(sym);
    }
  }

  private static final class DerefCollector extends com.sun.source.util.TreeScanner<Void, Void> {
    private final java.util.ArrayDeque<Symbol> methods = new java.util.ArrayDeque<>();

    @Override
    public Void visitMethod(MethodTree node, Void unused) {
      methods.push(((JCTree.JCMethodDecl) node).sym);
      super.visitMethod(node, unused);
      methods.pop();
      return null;
    }

    @Override
    public Void visitLambdaExpression(LambdaExpressionTree node, Void unused) {
      methods.push(com.sun.tools.javac.code.Symtab.instance(context).noSymbol);
      super.visitLambdaExpression(node, unused);
      methods.pop();
      return null;
    }

    @Override
    public Void visitReturn(ReturnTree node, Void unused) {
      if (!methods.isEmpty() && methods.peek() instanceof Symbol.MethodSymbol m) {
        taint(m, node.getExpression());
      }
      return super.visitReturn(node, unused);
    }

    @Override
    public Void visitVariable(VariableTree node, Void unused) {
      taint(((JCTree.JCVariableDecl) node).sym, node.getInitializer());
      return super.visitVariable(node, unused);
    }

    @Override
    public Void visitAssignment(AssignmentTree node, Void unused) {
      taint(TreeInfo.symbol((JCTree) node.getVariable()), node.getExpression());
      return super.visitAssignment(node, unused);
    }

    private void deref(ExpressionTree e) {
      Symbol p = stringParameter(e);
      if (p != null) {
        derefParams.add(p);
      }
    }

    private void arguments(Symbol callee, List<? extends ExpressionTree> args) {
      for (int i = 0; i < args.size(); i++) {
        Symbol p = stringParameter(args.get(i));
        Symbol q = declaredParameter(callee, i);
        if (q != null && isString(q.type) && taintedExpression(args.get(i))) {
          taintGrew |= tainted.add(q);
        }
        if (p != null && q != null) {
          passedOn.computeIfAbsent(p, k -> new HashSet<>()).add(q);
        }
        if (p != null && rejectsNull(callee) && !(i > 0 && callee.name.contentEquals("checkNotNull"))) {
          derefParams.add(p);
        }
      }
    }

    @Override
    public Void visitMemberSelect(com.sun.source.tree.MemberSelectTree node, Void unused) {
      deref(node.getExpression());
      return super.visitMemberSelect(node, unused);
    }

    @Override
    public Void visitSwitch(com.sun.source.tree.SwitchTree node, Void unused) {
      deref(node.getExpression());
      return super.visitSwitch(node, unused);
    }

    @Override
    public Void visitSwitchExpression(com.sun.source.tree.SwitchExpressionTree node, Void unused) {
      deref(node.getExpression());
      return super.visitSwitchExpression(node, unused);
    }

    @Override
    public Void visitMethodInvocation(MethodInvocationTree node, Void unused) {
      arguments(TreeInfo.symbol(((JCTree.JCMethodInvocation) node).meth), node.getArguments());
      return super.visitMethodInvocation(node, unused);
    }

    @Override
    public Void visitNewClass(NewClassTree node, Void unused) {
      arguments(((JCTree.JCNewClass) node).constructor, node.getArguments());
      return super.visitNewClass(node, unused);
    }
  }

  private static boolean isProduction(String file) {
    return file.contains("java/src/") && !file.contains("/testing/");
  }

  private static boolean isString(Type t) {
    return t != null
        && t.tsym != null
        && t.tsym.getQualifiedName().contentEquals("java.lang.String")
        && !t.hasTag(com.sun.tools.javac.code.TypeTag.ARRAY);
  }

  private static boolean isString(Tree tree) {
    return tree instanceof JCTree jc && isString(jc.type);
  }

  private static boolean isStringArray(Type t) {
    return t != null
        && t.hasTag(com.sun.tools.javac.code.TypeTag.ARRAY)
        && isString(((Type.ArrayType) t).elemtype);
  }

  private static ExpressionTree strip(ExpressionTree e) {
    while (true) {
      if (e instanceof JCTree.JCParens p) {
        e = p.expr;
      } else if (e instanceof JCTree.JCTypeCast c) {
        e = c.expr;
      } else {
        return e;
      }
    }
  }

  private static boolean isNull(ExpressionTree e) {
    return e != null && strip(e).getKind() == Tree.Kind.NULL_LITERAL;
  }

  private static boolean isLibrary(Symbol sym) {
    Symbol.ClassSymbol c = sym.outermostClass();
    return c == null
        || (c.classfile != null && c.classfile.getKind() == JavaFileObject.Kind.CLASS)
        || c.sourcefile == null;
  }

  private static String key(Symbol sym) {
    return sym.owner.getQualifiedName() + "#" + sym.name;
  }

  private static final class Visitor extends TreePathScanner<Void, Void> {
    private final CompilationUnitTree unit;
    private final String file;
    private final boolean info;
    private final List<String> lines;
    private final Set<Tree> wrapped = new HashSet<>();
    private final List<Edit> edits = new ArrayList<>();
    private String src;

    private int start(Tree t) {
      return (int) positions.getStartPosition(unit, t);
    }

    private int end(Tree t) {
      return (int) positions.getEndPosition(unit, t);
    }

    private String text(Tree t) {
      String s = src.substring(start(t), end(t));
      return s.startsWith("this.") ? s.substring(5) : s;
    }

    void applyEdits() throws Exception {
      if (!instrument || edits.isEmpty()) {
        return;
      }
      edits.sort(
          (a, b) ->
              a.start != b.start
                  ? Integer.compare(a.start, b.start)
                  : Integer.compare(a.end - a.start, b.end - b.start));
      StringBuilder out = new StringBuilder();
      int pos = 0;
      for (Edit e : edits) {
        if (e.start < pos) {
          throw new IllegalStateException("overlapping edits in " + file);
        }
        out.append(src, pos, e.start).append(e.text);
        pos = e.end;
      }
      out.append(src, pos, src.length());
      Files.writeString(Paths.get(file), out.toString(), StandardCharsets.UTF_8);
    }

    private boolean optionalTest(ExpressionTree cond, String t, String method) {
      return cond instanceof JCTree.JCMethodInvocation call
          && call.args.isEmpty()
          && call.meth instanceof JCTree.JCFieldAccess select
          && select.name.contentEquals(method)
          && text(select.selected).equals(t);
    }

    /** cond == true implies that t is present. */
    private boolean whenTrue(ExpressionTree cond, String t) {
      cond = parens(cond);
      if (optionalTest(cond, t, "isPresent")) {
        return true;
      }
      if (cond.getKind() == Tree.Kind.CONDITIONAL_AND) {
        BinaryTree b = (BinaryTree) cond;
        return whenTrue(b.getLeftOperand(), t) || whenTrue(b.getRightOperand(), t);
      }
      if (cond.getKind() == Tree.Kind.LOGICAL_COMPLEMENT) {
        return whenFalse(((com.sun.source.tree.UnaryTree) cond).getExpression(), t);
      }
      return false;
    }

    /** cond == false implies that t is present. */
    private boolean whenFalse(ExpressionTree cond, String t) {
      cond = parens(cond);
      if (optionalTest(cond, t, "isEmpty")) {
        return true;
      }
      // X.orElse("").isEmpty()
      if (cond instanceof JCTree.JCMethodInvocation call
          && call.args.isEmpty()
          && call.meth instanceof JCTree.JCFieldAccess select
          && select.name.contentEquals("isEmpty")
          && parens(select.selected) instanceof JCTree.JCMethodInvocation inner
          && inner.meth instanceof JCTree.JCFieldAccess innerSelect
          && innerSelect.name.contentEquals("orElse")
          && inner.args.size() == 1
          && inner.args.get(0).getKind() == Tree.Kind.STRING_LITERAL
          && text(innerSelect.selected).equals(t)) {
        return true;
      }
      if (cond.getKind() == Tree.Kind.CONDITIONAL_OR) {
        BinaryTree b = (BinaryTree) cond;
        return whenFalse(b.getLeftOperand(), t) || whenFalse(b.getRightOperand(), t);
      }
      if (cond.getKind() == Tree.Kind.LOGICAL_COMPLEMENT) {
        return whenTrue(((com.sun.source.tree.UnaryTree) cond).getExpression(), t);
      }
      return false;
    }

    private static boolean abrupt(com.sun.source.tree.StatementTree s) {
      if (s == null) {
        return false;
      }
      switch (s.getKind()) {
        case RETURN, THROW, CONTINUE, BREAK:
          return true;
        case BLOCK:
          List<? extends com.sun.source.tree.StatementTree> list =
              ((com.sun.source.tree.BlockTree) s).getStatements();
          return !list.isEmpty() && abrupt(list.get(list.size() - 1));
        default:
          return false;
      }
    }

    private boolean statementGuards(com.sun.source.tree.StatementTree s, String t) {
      if (s instanceof com.sun.source.tree.IfTree i) {
        if (abrupt(i.getThenStatement()) && whenFalse(i.getCondition(), t)) {
          return true;
        }
        return abrupt(i.getElseStatement()) && whenTrue(i.getCondition(), t);
      }
      // `X = Optional.of(..);` (also `X = Optional.of(X.get() + ..)`) as an earlier statement
      if (s instanceof com.sun.source.tree.ExpressionStatementTree as
          && as.getExpression() instanceof AssignmentTree assign
          && text(assign.getVariable()).equals(t)
          && isOptionalOf(parens(assign.getExpression()))) {
        return true;
      }
      if (s instanceof com.sun.source.tree.ExpressionStatementTree es
          && es.getExpression() instanceof MethodInvocationTree call
          && !call.getArguments().isEmpty()) {
        Symbol sym = TreeInfo.symbol(((JCTree.JCMethodInvocation) call).meth);
        String n = sym == null ? "" : sym.name.toString();
        if (n.equals("checkState") || n.equals("checkArgument") || n.equals("verify")) {
          return whenTrue(call.getArguments().get(0), t);
        }
      }
      return false;
    }

    /** Whether the expression with text t at the current path is under a presence check. */
    private boolean guarded(String t) {
      Tree child = getCurrentPath().getLeaf();
      for (TreePath p = getCurrentPath().getParentPath(); p != null; p = p.getParentPath()) {
        Tree a = p.getLeaf();
        if (a instanceof MethodTree
            || a instanceof LambdaExpressionTree
            || a instanceof com.sun.source.tree.ClassTree) {
          return false;
        }
        if (a instanceof com.sun.source.tree.IfTree i) {
          if (child == i.getThenStatement() && whenTrue(i.getCondition(), t)) {
            return true;
          }
          if (child == i.getElseStatement() && whenFalse(i.getCondition(), t)) {
            return true;
          }
        } else if (a instanceof ConditionalExpressionTree c) {
          if (child == c.getTrueExpression() && whenTrue(c.getCondition(), t)) {
            return true;
          }
          if (child == c.getFalseExpression() && whenFalse(c.getCondition(), t)) {
            return true;
          }
        } else if (a instanceof BinaryTree b) {
          if (a.getKind() == Tree.Kind.CONDITIONAL_AND
              && child == b.getRightOperand()
              && whenTrue(b.getLeftOperand(), t)) {
            return true;
          }
          if (a.getKind() == Tree.Kind.CONDITIONAL_OR
              && child == b.getRightOperand()
              && whenFalse(b.getLeftOperand(), t)) {
            return true;
          }
        } else if (a instanceof com.sun.source.tree.WhileLoopTree w) {
          if (child == w.getStatement() && whenTrue(w.getCondition(), t)) {
            return true;
          }
        } else if (a instanceof com.sun.source.tree.BlockTree
            || a instanceof com.sun.source.tree.CaseTree) {
          List<? extends com.sun.source.tree.StatementTree> statements =
              a instanceof com.sun.source.tree.BlockTree b
                  ? b.getStatements()
                  : ((com.sun.source.tree.CaseTree) a).getStatements();
          if (statements != null) {
            for (com.sun.source.tree.StatementTree s : statements) {
              if (s == child) {
                break;
              }
              if (statementGuards(s, t)) {
                return true;
              }
            }
          }
        }
        child = a;
      }
      return false;
    }

    private static boolean isOptionalOf(ExpressionTree e) {
      Symbol sym = e instanceof JCTree.JCMethodInvocation c ? TreeInfo.symbol(c.meth) : null;
      return sym != null && key(sym).equals("java.util.Optional#of");
    }

    private final Map<Symbol, Boolean> initialisedWithOf = new java.util.HashMap<>();

    private static void form(String name) {
      getForms.computeIfAbsent(name, k -> new int[1])[0]++;
    }

    private String site(Tree t) {
      return file.substring(file.lastIndexOf('/') + 1)
          + ":"
          + unit.getLineMap().getLineNumber(start(t));
    }

    /** `receiver.get()` on an Optional<String>. */
    private void optionalGet(MethodInvocationTree node, ExpressionTree receiver) {
      if (!isProduction(file)) {
        return;
      }
      ExpressionTree r = parens(receiver);
      Symbol sym =
          r instanceof com.sun.source.tree.IdentifierTree ? TreeInfo.symbol((JCTree) r) : null;
      if (guarded(text(r))) {
        form("get(), under a presence check");
      } else if (isOptionalOf(r) || (sym != null && Boolean.TRUE.equals(initialisedWithOf.get(sym)))) {
        form("get(), value built with Optional.of");
      } else if (getCurrentPath().getParentPath().getLeaf() instanceof com.sun.source.tree.MemberSelectTree) {
        form("get(), dereferenced at once (was a NullPointerException)");
      } else {
        Tree parent = getCurrentPath().getParentPath().getLeaf();
        Symbol callee = null;
        List<? extends ExpressionTree> args = null;
        if (parent instanceof JCTree.JCMethodInvocation call) {
          callee = TreeInfo.symbol(call.meth);
          args = call.args;
        } else if (parent instanceof JCTree.JCNewClass call) {
          callee = call.constructor;
          args = call.args;
        }
        int index = args == null ? -1 : args.indexOf(node);
        if (index >= 0 && derefParams.contains(declaredParameter(callee, index))) {
          form("get(), argument that the callee dereferences (was a NullPointerException)");
          return;
        }
        if (index >= 0 && rejectsNull(callee)) {
          form("get(), argument of a library method that rejects null (was a NullPointerException)");
          return;
        }
        String where = "?";
        for (TreePath p = getCurrentPath(); p != null; p = p.getParentPath()) {
          if (p.getLeaf() instanceof JCTree.JCMethodDecl m && m.sym != null) {
            where = m.sym.owner.getQualifiedName() + "#" + m.sym.name;
            break;
          }
          if (p.getLeaf() instanceof JCTree.JCClassDecl c && c.sym != null) {
            where = c.sym.getQualifiedName() + "#";
            break;
          }
        }
        String sink =
            callee != null && index >= 0
                ? callee.owner.name + "." + callee.name
                : parent.getKind().toString();
        report(
            node,
            "unguarded-get",
            "get() outside a presence check  KEY " + where + " :: " + text(r) + " :: " + sink,
            null);
        if (instrument) {
          edits.add(new Edit(start(node), start(node), "com.google.javascript.jscomp.base.NullProbe.get("));
          edits.add(new Edit(end(receiver), end(node), ", \"" + site(node) + "\")"));
        }
      }
    }

    private boolean provenNonNull(ExpressionTree e) {
      e = parens(e);
      switch (e.getKind()) {
        case STRING_LITERAL, PLUS:
          return true;
        case CONDITIONAL_EXPRESSION:
          ConditionalExpressionTree c = (ConditionalExpressionTree) e;
          return provenNonNull(c.getTrueExpression()) && provenNonNull(c.getFalseExpression());
        default:
          break;
      }
      if (e instanceof JCTree.JCMethodInvocation call) {
        Symbol sym = TreeInfo.symbol(call.meth);
        if (sym == null) {
          return false;
        }
        String owner = sym.owner.getQualifiedName().toString();
        String name = sym.name.toString();
        if (call.meth instanceof JCTree.JCFieldAccess select && isOptionalOfString(select.selected)) {
          return name.equals("get")
              || name.equals("orElseThrow")
              || (name.equals("orElse") && !isNull(call.args.get(0)));
        }
        if (name.equals("toString") && call.args.isEmpty()) {
          return true;
        }
        if (isLibrary(sym)) {
          return !owner.equals("java.lang.System")
              && !owner.equals("java.lang.Class")
              && (owner.startsWith("java.lang.")
                  || owner.equals("com.google.common.base.Joiner")
                  || owner.equals("com.google.common.base.Ascii")
                  || owner.equals("com.google.common.base.CharMatcher")
                  || (owner.equals("com.google.common.base.Strings") && !name.equals("emptyToNull")));
        }
        return nonNull.contains(owner + "#" + name + "()");
      }
      if (e instanceof com.sun.source.tree.IdentifierTree || e instanceof com.sun.source.tree.MemberSelectTree) {
        Symbol sym = TreeInfo.symbol((JCTree) e);
        if (sym instanceof Symbol.VarSymbol v) {
          if (v.getConstValue() != null) {
            return true;
          }
          if (v.owner instanceof Symbol.ClassSymbol) {
            return nonNull.contains(v.owner.getQualifiedName() + "#" + v.name);
          }
          if (v.owner instanceof Symbol.MethodSymbol m && (v.flags() & com.sun.tools.javac.code.Flags.PARAMETER) != 0) {
            int i = m.params == null ? -1 : m.params.indexOf(v);
            return nonNull.contains(m.owner.getQualifiedName() + "#" + m.name + "(" + i + ")");
          }
        }
      }
      return false;
    }

    /** `Optional.of(arg)` with a String argument. */
    private void optionalOf(MethodInvocationTree node, ExpressionTree arg) {
      if (!isProduction(file)) {
        return;
      }
      if (provenNonNull(arg)) {
        form("Optional.of(e), e proven non-null");
        return;
      }
      ExpressionTree e = parens(arg);
      Symbol callee = e instanceof JCTree.JCMethodInvocation c ? TreeInfo.symbol(c.meth) : null;
      if (callee != null && isLibrary(callee)) {
        report(node, "unproven-of", "Optional.of of a library call that may return null", null);
      } else if (taintedExpression(e)) {
        report(
            node,
            "unproven-of",
            "Optional.of of a value that can come from a source of nulls (tainted)",
            null);
      } else if (callee != null) {
        form("Optional.of(e), e a call of a source method declared to return String");
      } else if (e instanceof com.sun.source.tree.IdentifierTree
          && TreeInfo.symbol((JCTree) e) instanceof Symbol.VarSymbol v
          && !(v.owner instanceof Symbol.ClassSymbol)) {
        form("Optional.of(e), e a String parameter or local");
      } else {
        form("Optional.of(e), e a String field or other expression");
      }
      if (instrument) {
        edits.add(new Edit(start(node), start(arg), "com.google.javascript.jscomp.base.NullProbe.of("));
        edits.add(new Edit(end(arg), end(arg), ", \"" + site(node) + "\""));
      }
    }

    Visitor(CompilationUnitTree unit, boolean info) throws Exception {
      this.unit = unit;
      this.file = unit.getSourceFile().getName();
      this.info = info;
      this.lines = Files.readAllLines(Paths.get(file), StandardCharsets.UTF_8);
      this.src = unit.getSourceFile().getCharContent(true).toString();
    }

    private void report(Tree at, String category, String message, String libKey) {
      long line = unit.getLineMap().getLineNumber(positions.getStartPosition(unit, at));
      String text = line >= 1 && line <= lines.size() ? lines.get((int) line - 1).trim() : "";
      String exempt = null;
      String lineKey = file + "::" + text;
      if (exemptLines.contains(lineKey)) {
        exempt = lineKey;
        usedExemptions.add(lineKey);
      } else if (libKey != null && exemptLibs.contains(libKey)) {
        exempt = "lib:" + libKey;
        usedExemptions.add(exempt);
      }
      findings.add(new Finding(file, line, category, message + "  |  " + text, exempt));
    }

    private static boolean hasNullable(List<? extends AnnotationTree> annotations) {
      for (AnnotationTree a : annotations) {
        String name = a.getAnnotationType().toString();
        if (name.equals("Nullable") || name.endsWith(".Nullable")) {
          return true;
        }
      }
      return false;
    }

    @Override
    public Void visitVariable(VariableTree node, Void unused) {
      JCTree.JCVariableDecl decl = (JCTree.JCVariableDecl) node;
      if (decl.sym != null
          && node.getInitializer() != null
          && isOptionalOf(parens(node.getInitializer()))
          && (decl.sym.flags()
                  & (com.sun.tools.javac.code.Flags.FINAL
                      | com.sun.tools.javac.code.Flags.EFFECTIVELY_FINAL))
              != 0) {
        initialisedWithOf.put(decl.sym, true);
      }
      Type type = decl.sym != null ? decl.sym.type : decl.type;
      if (isString(type)) {
        if (hasNullable(node.getModifiers().getAnnotations())) {
          report(node, "nullable-decl", "@Nullable String " + node.getName(), null);
        }
        if (isNull(node.getInitializer())) {
          report(node, "null-literal", "String " + node.getName() + " = null", null);
        }
      }
      return super.visitVariable(node, unused);
    }

    @Override
    public Void visitMethod(MethodTree node, Void unused) {
      JCTree.JCMethodDecl decl = (JCTree.JCMethodDecl) node;
      if (decl.sym != null
          && isString(decl.sym.getReturnType())
          && hasNullable(node.getModifiers().getAnnotations())) {
        report(node, "nullable-decl", "@Nullable String " + node.getName() + "(...)", null);
      }
      return super.visitMethod(node, unused);
    }

    @Override
    public Void visitAnnotatedType(AnnotatedTypeTree node, Void unused) {
      if (hasNullable(node.getAnnotations()) && isString(((JCTree) node).type)) {
        report(node, "nullable-decl", "@Nullable String type use", null);
      }
      return super.visitAnnotatedType(node, unused);
    }

    @Override
    public Void visitAssignment(AssignmentTree node, Void unused) {
      if (isNull(node.getExpression()) && isString(node.getVariable())) {
        report(node, "null-literal", node.getVariable() + " = null", null);
      }
      return super.visitAssignment(node, unused);
    }

    @Override
    public Void visitTypeCast(TypeCastTree node, Void unused) {
      if (isString(node) && isNull(node.getExpression())) {
        report(node, "null-literal", "(String) null", null);
      }
      return super.visitTypeCast(node, unused);
    }

    private Type enclosingReturnType() {
      for (TreePath p = getCurrentPath(); p != null; p = p.getParentPath()) {
        Tree t = p.getLeaf();
        if (t instanceof JCTree.JCLambda lambda) {
          return lambdaReturn(lambda);
        }
        if (t instanceof JCTree.JCMethodDecl method) {
          return method.sym == null ? null : method.sym.getReturnType();
        }
        if (t instanceof JCTree.JCClassDecl) {
          return null;
        }
      }
      return null;
    }

    private static Type lambdaReturn(JCTree.JCLambda lambda) {
      if (lambda.type == null || lambda.type.isErroneous()) {
        return null;
      }
      Type descriptor = types.findDescriptorType(lambda.type);
      return descriptor == null ? null : descriptor.getReturnType();
    }

    @Override
    public Void visitReturn(ReturnTree node, Void unused) {
      if (isNull(node.getExpression()) && isString(enclosingReturnType())) {
        report(node, "null-literal", "return null from a String method", null);
      }
      return super.visitReturn(node, unused);
    }

    @Override
    public Void visitLambdaExpression(LambdaExpressionTree node, Void unused) {
      if (node.getBodyKind() == LambdaExpressionTree.BodyKind.EXPRESSION
          && isNull((ExpressionTree) node.getBody())
          && isString(lambdaReturn((JCTree.JCLambda) node))) {
        report(node, "null-literal", "lambda returning null as String", null);
      }
      return super.visitLambdaExpression(node, unused);
    }

    @Override
    public Void visitConditionalExpression(ConditionalExpressionTree node, Void unused) {
      if (isString(node) && (isNull(node.getTrueExpression()) || isNull(node.getFalseExpression()))) {
        report(node, "null-literal", "null branch of a String conditional", null);
      }
      return super.visitConditionalExpression(node, unused);
    }

    @Override
    public Void visitNewArray(NewArrayTree node, Void unused) {
      if (node.getInitializers() != null && isStringArray(((JCTree) node).type)) {
        for (ExpressionTree e : node.getInitializers()) {
          if (isNull(e)) {
            report(e, "null-literal", "null element of a String[] initialiser", null);
          }
        }
      }
      return super.visitNewArray(node, unused);
    }

    private void checkArguments(
        Tree call,
        Symbol sym,
        Type methodType,
        Type varargsElement,
        List<? extends ExpressionTree> arguments) {
      if (sym == null || methodType == null || !(methodType instanceof Type.MethodType)) {
        return;
      }
      List<Type> formals = methodType.getParameterTypes();
      for (int i = 0; i < arguments.size(); i++) {
        ExpressionTree arg = arguments.get(i);
        if (!isNull(arg)) {
          continue;
        }
        Type formal;
        if (varargsElement != null && i >= formals.size() - 1) {
          formal = varargsElement;
        } else if (i < formals.size()) {
          formal = formals.get(i);
        } else {
          continue;
        }
        if (isString(formal)) {
          boolean library = isLibrary(sym);
          report(
              arg,
              "null-literal",
              "null passed for String parameter " + i + " of " + (library ? "library " : "") + key(sym),
              library ? key(sym) : null);
        }
      }
    }

    @Override
    public Void visitMethodInvocation(MethodInvocationTree node, Void unused) {
      JCTree.JCMethodInvocation call = (JCTree.JCMethodInvocation) node;
      Symbol sym = TreeInfo.symbol(call.meth);
      checkArguments(node, sym, call.meth.type, call.varargsElement, node.getArguments());
      if (call.meth instanceof JCTree.JCFieldAccess select
          && node.getArguments().isEmpty()
          && isOptionalOfString(select.selected)) {
        if (select.name.contentEquals("get")) {
          optionalGet(node, select.selected);
        } else if (select.name.contentEquals("orElseThrow") && isProduction(file)) {
          form("orElseThrow(), asserted: checkNotNull in the old code, or established by reading");
        }
      }
      if (sym != null && sym.owner != null) {
        String key = key(sym);
        if (key.equals("java.util.Optional#of")
            && node.getArguments().size() == 1
            && isString(node.getArguments().get(0))) {
          optionalOf(node, node.getArguments().get(0));
        }
        if (CHECK_CALLS.contains(key)) {
          // Only the first argument is the checked value, except for the two-value comparisons
          // and for firstNonNull / requireNonNullElse, where the second one is too.
          boolean firstOnly =
              !(key.endsWith("#equals")
                  || key.endsWith("#equal")
                  || key.endsWith("#firstNonNull")
                  || key.endsWith("#requireNonNullElse"));
          int checked = Math.min(node.getArguments().size(), firstOnly ? 1 : 2);
          for (int i = 0; i < checked; i++) {
            if (isString(node.getArguments().get(i))) {
              report(node, "null-check", sym.owner.name + "." + sym.name + " on a String", null);
              break;
            }
          }
        }
        if (OF_NULLABLE.contains(key) && node.getArguments().size() == 1) {
          ExpressionTree arg = strip(node.getArguments().get(0));
          if (isString(arg)) {
            Symbol inner =
                arg instanceof JCTree.JCMethodInvocation m ? TreeInfo.symbol(m.meth) : null;
            // A source method declared to return a type variable (a container's `T get()`)
            // is a boundary like a library call.
            boolean generic =
                inner instanceof Symbol.MethodSymbol m
                    && m.getReturnType().hasTag(com.sun.tools.javac.code.TypeTag.TYPEVAR);
            if (inner != null && (isLibrary(inner) || generic)) {
              wrapped.add(arg);
            } else {
              report(node, "of-nullable", "ofNullable of a String that is not a library call", null);
            }
          }
        }
        if (info
            && isProduction(file)
            && isString(node)
            && isLibrary(sym)
            && !wrapped.contains(node)
            && (NULL_RETURNING.contains(key) || isMapGet(sym))) {
          long line = unit.getLineMap().getLineNumber(positions.getStartPosition(unit, node));
          infos.add(
              file + ":" + line + ": [info] " + key + " returns a nullable String  |  "
                  + lines.get((int) line - 1).trim());
        }
      }
      return super.visitMethodInvocation(node, unused);
    }

    private static boolean isMapGet(Symbol sym) {
      if (!sym.name.contentEquals("get") || !(sym.owner instanceof Symbol.ClassSymbol owner)) {
        return false;
      }
      for (Type t : types.closure(owner.type)) {
        if (t.tsym.getQualifiedName().contentEquals("java.util.Map")) {
          return true;
        }
      }
      return false;
    }

    @Override
    public Void visitNewClass(NewClassTree node, Void unused) {
      JCTree.JCNewClass call = (JCTree.JCNewClass) node;
      checkArguments(node, call.constructor, call.constructorType, call.varargsElement, node.getArguments());
      return super.visitNewClass(node, unused);
    }

    @Override
    public Void visitBinary(BinaryTree node, Void unused) {
      if (node.getKind() == Tree.Kind.EQUAL_TO || node.getKind() == Tree.Kind.NOT_EQUAL_TO) {
        ExpressionTree l = node.getLeftOperand();
        ExpressionTree r = node.getRightOperand();
        if ((isNull(r) && isString(strip(l)) && !isNull(l))
            || (isNull(l) && isString(strip(r)) && !isNull(r))) {
          report(
              node,
              "null-check",
              "String " + (node.getKind() == Tree.Kind.EQUAL_TO ? "== null" : "!= null"),
              null);
        }
      }
      return super.visitBinary(node, unused);
    }
  }
}
