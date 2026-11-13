// Prep step 10 rewriter: turns "maybe absent" Strings into java.util.Optional<String> (or, for the
// declarations listed in empty.txt, into Strings that use "" for "absent").
//
// Attributes java/src and java/test like Scan.java, decides which String declarations are
// nullable, and rewrites the declarations and every use from the symbols javac resolved, in all
// scanned sources. Test code is rewritten only where it touches converted declarations.
//
// WHICH DECLARATIONS. A String variable (field, parameter, local) or method return declared in
// production code under one of the --scope path prefixes is NULLABLE when
//   * it is annotated @Nullable, or is listed in force.txt;
//   * a `null` literal (also `(String) null`, a branch of a conditional or of a switch expression)
//     is assigned to it / returned from it, or passed for it by any call (test code included);
//   * production code compares it (the variable, or a call of the method) with null, or passes it
//     to Strings.nullToEmpty / emptyToNull. Not counted: comparisons inside checkState /
//     checkArgument / verify, and `if (param == null) { throw ...; }` (deleted when the parameter
//     stays a String);
//   * a nullable expression that is not under a presence check is assigned to it / returned from
//     it (not for parameters, see below); `E.orElse(null)` counts as a nullable expression;
//   * (methods) its result is the only value ever stored in a local variable that is compared
//     with null;
//   * (fields) it is the field that a one-statement getter returns which is itself annotated
//     @Nullable, or is not an override and has its result compared with null; or it has no initial
//     value and is read into a nullable declaration;
//   * (parameters) some call passes a nullable expression outside a presence check AND the
//     method body either hands the parameter on to a nullable declaration, or never needs it
//     present: it does not dereference it (`p.foo()`, `switch (p)`), store it in a non-nullable
//     variable, or pass it to a
//     library String parameter or to a parameter that does (transitively).
// Methods related by overriding are converted together (return, and each parameter position); a
// lambda's parameters and result are those of the source method it implements; a record
// component, its accessor and the canonical constructor parameter are one declaration. Never
// converted: families that contain a library method, lambdas for library interfaces, and the
// declarations listed in keep.txt.
//
// EMPTY-STRING MODE. The declarations in empty.txt stay Strings and use "" for "absent" (hot
// paths, where "" is never a legitimate value). So do the nullable locals and method returns whose
// every source is such an expression, a null literal or a non-empty string literal (at least one
// of the first kind), and the parameters that are nullable only because they feed such
// declarations.
//
// REWRITE RULES (X: Optional-mode variable or call; E: empty-string-mode one).
//   declaration            `@Nullable String x`      -> `Optional<String> x`     (E: `String x`)
//   field without value    `String x;`               -> `... x = Optional.empty();`  (E: `= ""`)
//   comparison             `X == null` / `X != null` -> `X.isEmpty()` / `X.isPresent()`
//                          `E == null` / `E != null` -> `E.isEmpty()` / `!E.isEmpty()`
//   into an Optional       `null`                    -> `Optional.empty()`
//                          X                         -> unchanged;  `F.orElse(null)` -> `F`
//                          E                         -> `Optional.of(E).filter(s -> !s.isEmpty())`
//                             (`Optional.of(E)` under a presence check)
//                          other expr                -> `Optional.of(expr)`; `Optional.ofNullable(
//                             expr)` when expr is a call of a library method outside java.lang /
//                             Guava's string helpers, or of a source method declared to return a
//                             type variable; in test code always ofNullable (tests keep their
//                             nulls)
//                          `c ? a : b`, switch       -> each branch by these rules
//                          `x += e`                  -> `x = Optional.of(x.orElse("null") + e)`
//                             (`x.get()` under a presence check)
//   into an E              `null` -> `""`;  X -> `X.orElse("")`
//   out of an Optional, X read as a plain String:
//     receiver `X.foo()`, and X under a presence check (inside `if (X != null)`, after
//       `if (X == null) return/throw/continue/break`, right of `X != null &&`, after
//       checkNotNull(X) / checkState(X != null), ...; matched by the source text of X)
//                                                    -> `X.get()`
//     checkNotNull(X), and an argument for a parameter listed in keep.txt (reviewed: the value
//       is present there)                             -> `X.orElseThrow()`
//     argument for a parameter that the called source method dereferences (`p.foo()`,
//       `switch (p)`, hands to a library method that rejects null, or passes on to a parameter
//       that does), or of a library method that rejects null (Guava Immutable* builders and
//       Interner, String methods, Splitter, Pattern.matcher, Path.of, StringReader, URI): null
//       was a NullPointerException there             -> `X.get()`
//     argument for a parameter that the called source method checked with checkNotNull /
//       requireNonNull (the check is deleted by this step) -> `X.orElseThrow()`
//     a site listed in reviewed.txt (`pkg.Class#method :: text of X [:: callee] ## reason`:
//       presence established by reading)              -> `X.orElseThrow()`
//     string concatenation, append / print / valueOf, format and check-message arguments,
//       String varargs of source methods (diagnostic arguments)  -> `X.orElse("null")`
//     `"literal".equals(X)`                          -> `"literal".equals(X.orElse(""))`
//     `a.equals(X)`                                  -> `Optional.of(a).equals(X)`
//     `X1.equals(X2)`, `Objects.equals(X1, X2)`      -> `X1.equals(X2)` on the Optionals
//     `c.contains(X)` / `m.containsKey(X)`, X a name -> `X.isPresent() && c.contains(X.get())`
//     `m.get(X)`, X a name                           -> `X.isPresent() ? m.get(X.get()) : null`
//     anything else, and everything in test code     -> `X.orElse(null)`: what the code did
//       before (Scan.java reports it as a null literal; KEY lines give the reviewed.txt key)
//   `X != null ? X : y`, `X == null ? y : X` (y a literal or a name) -> `X.orElse(y)`
//   `Strings.nullToEmpty(X)` -> `X.orElse("")`;  `Strings.isNullOrEmpty(X)` ->
//   `X.orElse("").isEmpty()`;  for E: `E` and `E.isEmpty()`
//   checkNotNull(s) / requireNonNull(s) / verifyNotNull(s) on a plain String, production code:
//     the statement is deleted, an expression is replaced by `s`; when s is a call of a library
//     method that may return null: `Optional.ofNullable(s).get()` (left alone if there is a
//     message).
// Imports of java.util.Optional are added (fully qualified where Guava's Optional is imported);
// imports of Nullable / checkNotNull / requireNonNull / Objects / Strings that become unused are
// removed.
//
// The lists (next to this file; `pkg.Class#field`, `pkg.Class#method()` for the return,
// `pkg.Class#method(paramIndex)`, constructors are `<init>`; text after " ## " is a comment):
//   keep.txt   never converted          empty.txt  empty-string mode
//   force.txt  nullable although nothing in production code says so
//   reviewed.txt  sites where a nullable value is read as present by review (orElseThrow)
//
// Output: one line per converted declaration (NULLABLE / EMPTYMODE, with the reason), EFLOW /
// NFLOW lines for values that leave the nullable world unchecked (to review), rule counts.
//
// Run from the repo root after tools/gate0/build.sh (JDK 21), with the same javac flags as
// Scan.java:
//   javac $X -d /tmp/scan10 tools/prep/10_find_null_strings/Convert.java
//   java $X -Xss16m -cp /tmp/scan10 Convert [--dry] --scope java/src/ \
//       ".cache/build/stock-libs.jar:$(ls .cache/deps/*.jar | tr '\n' ':')" java/src java/test
// The tool is not idempotent on its own output in one respect: it was run once on the tree
// before this step (the checked-in sources are its output plus the hand edits listed in the
// commit messages); a second run only picks up `.orElse(null)` bridges and new evidence.

import com.sun.source.tree.AnnotationTree;
import com.sun.source.tree.AssignmentTree;
import com.sun.source.tree.BinaryTree;
import com.sun.source.tree.BlockTree;
import com.sun.source.tree.CaseTree;
import com.sun.source.tree.ClassTree;
import com.sun.source.tree.CompilationUnitTree;
import com.sun.source.tree.CompoundAssignmentTree;
import com.sun.source.tree.ConditionalExpressionTree;
import com.sun.source.tree.ExpressionStatementTree;
import com.sun.source.tree.ExpressionTree;
import com.sun.source.tree.IdentifierTree;
import com.sun.source.tree.IfTree;
import com.sun.source.tree.ImportTree;
import com.sun.source.tree.LambdaExpressionTree;
import com.sun.source.tree.MemberSelectTree;
import com.sun.source.tree.MethodInvocationTree;
import com.sun.source.tree.MethodTree;
import com.sun.source.tree.NewClassTree;
import com.sun.source.tree.ReturnTree;
import com.sun.source.tree.StatementTree;
import com.sun.source.tree.Tree;
import com.sun.source.tree.UnaryTree;
import com.sun.source.tree.VariableTree;
import com.sun.source.tree.WhileLoopTree;
import com.sun.source.util.JavacTask;
import com.sun.source.util.SourcePositions;
import com.sun.source.util.TreePath;
import com.sun.source.util.TreePathScanner;
import com.sun.source.util.Trees;
import com.sun.tools.javac.api.BasicJavacTask;
import com.sun.tools.javac.code.Flags;
import com.sun.tools.javac.code.Symbol;
import com.sun.tools.javac.code.Type;
import com.sun.tools.javac.code.TypeTag;
import com.sun.tools.javac.code.Types;
import com.sun.tools.javac.tree.JCTree;
import com.sun.tools.javac.tree.TreeInfo;
import java.io.File;
import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.ArrayList;
import java.util.HashMap;
import java.util.HashSet;
import java.util.IdentityHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import javax.tools.Diagnostic;
import javax.tools.JavaCompiler;
import javax.tools.JavaFileObject;
import javax.tools.StandardJavaFileManager;
import javax.tools.ToolProvider;

public final class Convert {
  private record ParamKey(Symbol.MethodSymbol method, int index) {}

  private record Edit(int start, int end, String text, int kind, int order) {}

  private static final int PREFIX = 0;
  private static final int REPLACE = 1;
  private static final int SUFFIX = 2;

  private static Types types;
  private static Trees trees;
  private static SourcePositions positions;
  private static final List<String> scopes = new ArrayList<>();
  private static final Set<String> keep = new HashSet<>();
  private static final Set<String> force = new HashSet<>();
  private static final Map<Object, Object> parent = new HashMap<>();
  private static final Set<Object> seedable = new HashSet<>();
  private static final Set<Object> pinned = new HashSet<>();
  private static final List<Object> pinnedRaw = new ArrayList<>();
  private static final Set<String> pinnedButNullable = new java.util.TreeSet<>();
  private static final Set<Object> nullable = new HashSet<>();
  private static final Set<String> listedEmpty = new HashSet<>();
  private static final Set<Object> listedEmptyRoots = new HashSet<>();
  private static final Set<Object> empty = new HashSet<>();
  private static final Set<Object> hasEmptySource = new HashSet<>();
  private static final List<String> flows = new ArrayList<>();
  private static final Map<Tree, Object> throwGuards = new IdentityHashMap<>();
  private static final Set<Object> givenAbsent = new HashSet<>();
  private static final Set<Object> usedNullable = new HashSet<>();
  private static final Set<Object> usedOptional = new HashSet<>();
  private static final Set<Object> comparedVars = new HashSet<>();
  private static final Map<Object, Integer> sourceCount = new HashMap<>();
  private static final Map<Object, Symbol.MethodSymbol> onlySource = new HashMap<>();
  private static final Set<Object> noInitialValue = new HashSet<>();
  private static final Set<Object> strictParams = new HashSet<>();
  private static final Set<Object> derefParams = new HashSet<>();
  private static final Set<Object> assertedParams = new HashSet<>();
  private static final Set<String> reviewed = new HashSet<>();
  private static final Set<String> usedReviewed = new HashSet<>();

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
  private static final Map<Object, Integer> callSites = new HashMap<>();
  private static final Map<Object, Set<Object>> passedOn = new HashMap<>();
  private static final Map<Object, String> why = new HashMap<>();
  private static final Map<String, int[]> stats = new TreeMap<>();
  private static boolean changed;
  private static boolean firstRound = true;
  private static boolean cascade;

  private static Object find(Object o) {
    Object p = parent.get(o);
    if (p == null) {
      return o;
    }
    Object root = find(p);
    parent.put(o, root);
    return root;
  }

  private static void union(Object a, Object b) {
    Object ra = find(a);
    Object rb = find(b);
    if (!ra.equals(rb)) {
      parent.put(ra, rb);
    }
  }

  private static Object keyOf(Symbol sym) {
    if (sym instanceof Symbol.VarSymbol v
        && v.owner instanceof Symbol.MethodSymbol m
        && (v.flags() & Flags.PARAMETER) != 0
        && m.params != null) {
      int i = m.params.indexOf(v);
      if (i >= 0) {
        return new ParamKey(m, i);
      }
    }
    return sym;
  }

  /** Converted to Optional<String>. */
  private static boolean isN(Object key) {
    Object root = find(key);
    return nullable.contains(root) && !pinned.contains(root) && !empty.contains(root);
  }

  /** Stays a String, with "" for absent. */
  private static boolean isE(Object key) {
    Object root = find(key);
    return nullable.contains(root) && !pinned.contains(root) && empty.contains(root);
  }

  private static final int PLAIN = 0;
  private static final int OPTIONAL = 1;
  private static final int EMPTY = 2;

  private static int modeOf(Object key) {
    return isN(key) ? OPTIONAL : isE(key) ? EMPTY : PLAIN;
  }

  private static void mark(Object key, String reason) {
    Object root = find(key);
    if (seedable.contains(root) && pinned.contains(root)) {
      pinnedButNullable.add("KEPT " + name(key) + "  (" + reason + ")");
    }
    if (seedable.contains(root) && !pinned.contains(root) && nullable.add(root)) {
      why.put(root, reason);
      changed = true;
    }
  }

  private static String name(Object key) {
    if (key instanceof ParamKey p) {
      return p.method.owner.getQualifiedName() + "#" + p.method.name + "(" + p.index + ")";
    }
    Symbol s = (Symbol) key;
    if (s instanceof Symbol.MethodSymbol) {
      return s.owner.getQualifiedName() + "#" + s.name + "()";
    }
    if (s.owner instanceof Symbol.ClassSymbol) {
      return s.owner.getQualifiedName() + "#" + s.name;
    }
    return s.owner.owner.getQualifiedName() + "#" + s.owner.name + "/" + s.name;
  }

  private static void count(String what) {
    stats.computeIfAbsent(what, k -> new int[1])[0]++;
  }

  private static boolean isProduction(String file) {
    return file.contains("java/src/") && !file.contains("/testing/");
  }

  private static boolean inScope(String file) {
    if (!isProduction(file)) {
      return false;
    }
    for (String s : scopes) {
      if (file.startsWith(s)) {
        return true;
      }
    }
    return false;
  }

  private static boolean isString(Type t) {
    return t != null
        && t.tsym != null
        && !t.hasTag(TypeTag.ARRAY)
        && t.tsym.getQualifiedName().contentEquals("java.lang.String");
  }

  private static boolean isString(Tree tree) {
    return tree instanceof JCTree jc && isString(jc.type);
  }

  private static boolean isOptionalOfString(Tree tree) {
    if (!(tree instanceof JCTree jc) || jc.type == null || jc.type.tsym == null) {
      return false;
    }
    return jc.type.tsym.getQualifiedName().contentEquals("java.util.Optional")
        && jc.type.getTypeArguments().size() == 1
        && isString(jc.type.getTypeArguments().get(0));
  }

  private static ExpressionTree strip(ExpressionTree e) {
    while (e instanceof JCTree.JCParens p) {
      e = p.expr;
    }
    return e;
  }

  private static boolean isNullLiteral(ExpressionTree e) {
    if (e == null) {
      return false;
    }
    e = strip(e);
    // `(String) null` is a null literal too.
    return e.getKind() == Tree.Kind.NULL_LITERAL
        || (e instanceof JCTree.JCTypeCast cast && isNullLiteral(cast.expr));
  }

  private static boolean isLibrary(Symbol sym) {
    Symbol.ClassSymbol c = sym.outermostClass();
    return c == null
        || (c.classfile != null && c.classfile.getKind() == JavaFileObject.Kind.CLASS)
        || c.sourcefile == null;
  }

  private static Symbol callee(ExpressionTree e) {
    return e instanceof JCTree.JCMethodInvocation call ? TreeInfo.symbol(call.meth) : null;
  }

  private static boolean isCall(ExpressionTree e, String owner, String name) {
    Symbol sym = callee(e);
    return sym != null
        && sym.name.contentEquals(name)
        && sym.owner.getQualifiedName().contentEquals(owner);
  }

  /** `E.orElse(null)` on an Optional<String>: returns E, else null. */
  private static ExpressionTree bridgeReceiver(ExpressionTree e) {
    if (e instanceof JCTree.JCMethodInvocation call
        && call.meth instanceof JCTree.JCFieldAccess select
        && select.name.contentEquals("orElse")
        && call.args.size() == 1
        && isNullLiteral(call.args.get(0))
        && isOptionalOfString(select.selected)) {
      return select.selected;
    }
    return null;
  }

  /** The declaration (variable, or method called) that the (stripped) expression reads. */
  private static Object readKey(ExpressionTree e) {
    if (e instanceof IdentifierTree || e instanceof MemberSelectTree) {
      Symbol sym = TreeInfo.symbol((JCTree) e);
      return sym instanceof Symbol.VarSymbol && isString(sym.type) ? keyOf(sym) : null;
    }
    Symbol sym = callee(e);
    return sym instanceof Symbol.MethodSymbol m && isString(m.getReturnType()) ? m : null;
  }

  /** The key of the Optional-mode declaration that the (stripped) expression reads, or null. */
  private static Object optKey(ExpressionTree e) {
    Object key = readKey(e);
    return key != null && isN(key) ? key : null;
  }

  /** The key of the empty-string-mode declaration that the (stripped) expression reads. */
  private static Object empKey(ExpressionTree e) {
    Object key = readKey(e);
    return key != null && isE(key) ? key : null;
  }

  private abstract static class Base extends TreePathScanner<Void, Void> {
    final CompilationUnitTree unit;
    final String file;
    final String src;

    Base(CompilationUnitTree unit) throws Exception {
      this.unit = unit;
      this.file = unit.getSourceFile().getName();
      this.src = unit.getSourceFile().getCharContent(true).toString();
    }

    int start(Tree t) {
      return (int) positions.getStartPosition(unit, t);
    }

    int end(Tree t) {
      return (int) positions.getEndPosition(unit, t);
    }

    String text(Tree t) {
      String s = src.substring(start(t), end(t));
      return s.startsWith("this.") ? s.substring(5) : s;
    }

    /** The text that a presence check must mention to cover the (stripped) expression. */
    String guardText(ExpressionTree e) {
      ExpressionTree receiver = bridgeReceiver(e);
      return text(receiver != null ? receiver : e);
    }

    boolean nullTest(ExpressionTree cond, String t, Tree.Kind kind, String optionalMethod) {
      if (cond.getKind() == kind) {
        BinaryTree b = (BinaryTree) cond;
        ExpressionTree l = strip(b.getLeftOperand());
        ExpressionTree r = strip(b.getRightOperand());
        if (isNullLiteral(r) && guardText(l).equals(t)) {
          return true;
        }
        if (isNullLiteral(l) && guardText(r).equals(t)) {
          return true;
        }
      }
      return cond instanceof JCTree.JCMethodInvocation call
          && call.args.isEmpty()
          && call.meth instanceof JCTree.JCFieldAccess select
          && select.name.contentEquals(optionalMethod)
          && text(select.selected).equals(t);
    }

    /** cond == true implies that t is present. */
    boolean whenTrue(ExpressionTree cond, String t) {
      cond = strip(cond);
      if (nullTest(cond, t, Tree.Kind.NOT_EQUAL_TO, "isPresent")) {
        return true;
      }
      if (cond.getKind() == Tree.Kind.CONDITIONAL_AND) {
        BinaryTree b = (BinaryTree) cond;
        return whenTrue(b.getLeftOperand(), t) || whenTrue(b.getRightOperand(), t);
      }
      if (cond.getKind() == Tree.Kind.LOGICAL_COMPLEMENT) {
        return whenFalse(((UnaryTree) cond).getExpression(), t);
      }
      return false;
    }

    /** cond == false implies that t is present. */
    boolean whenFalse(ExpressionTree cond, String t) {
      cond = strip(cond);
      if (nullTest(cond, t, Tree.Kind.EQUAL_TO, "isEmpty")) {
        return true;
      }
      if (cond.getKind() == Tree.Kind.CONDITIONAL_OR) {
        BinaryTree b = (BinaryTree) cond;
        return whenFalse(b.getLeftOperand(), t) || whenFalse(b.getRightOperand(), t);
      }
      if (cond.getKind() == Tree.Kind.LOGICAL_COMPLEMENT) {
        return whenTrue(((UnaryTree) cond).getExpression(), t);
      }
      if (isCall(cond, "com.google.common.base.Strings", "isNullOrEmpty")) {
        return guardText(strip(((MethodInvocationTree) cond).getArguments().get(0))).equals(t);
      }
      return false;
    }

    static boolean abrupt(StatementTree s) {
      if (s == null) {
        return false;
      }
      switch (s.getKind()) {
        case RETURN, THROW, CONTINUE, BREAK:
          return true;
        case BLOCK:
          List<? extends StatementTree> list = ((BlockTree) s).getStatements();
          return !list.isEmpty() && abrupt(list.get(list.size() - 1));
        default:
          return false;
      }
    }

    boolean statementGuards(StatementTree s, String t) {
      if (s instanceof IfTree i) {
        if (abrupt(i.getThenStatement()) && whenFalse(i.getCondition(), t)) {
          return true;
        }
        return abrupt(i.getElseStatement()) && whenTrue(i.getCondition(), t);
      }
      if (s instanceof ExpressionStatementTree es
          && es.getExpression() instanceof MethodInvocationTree call
          && !call.getArguments().isEmpty()) {
        Symbol sym = callee(call);
        if (sym == null) {
          return false;
        }
        String n = sym.name.toString();
        ExpressionTree first = call.getArguments().get(0);
        if (n.equals("checkNotNull") || n.equals("requireNonNull") || n.equals("verifyNotNull")) {
          return guardText(strip(first)).equals(t);
        }
        if (n.equals("checkState") || n.equals("checkArgument") || n.equals("verify")) {
          return whenTrue(first, t);
        }
      }
      return false;
    }

    /** Whether the expression at the path is under a presence check of its own text. */
    boolean guarded(TreePath path) {
      String t = guardText(strip((ExpressionTree) path.getLeaf()));
      Tree child = path.getLeaf();
      for (TreePath p = path.getParentPath(); p != null; p = p.getParentPath()) {
        Tree a = p.getLeaf();
        if (a instanceof MethodTree || a instanceof LambdaExpressionTree || a instanceof ClassTree) {
          return false;
        }
        if (a instanceof IfTree i) {
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
        } else if (a instanceof WhileLoopTree w) {
          if (child == w.getStatement() && whenTrue(w.getCondition(), t)) {
            return true;
          }
        } else if (a instanceof BlockTree || a instanceof CaseTree) {
          List<? extends StatementTree> statements =
              a instanceof BlockTree b ? b.getStatements() : ((CaseTree) a).getStatements();
          if (statements != null) {
            for (StatementTree s : statements) {
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

    /** The expressions that a switch expression can evaluate to. */
    static List<ExpressionTree> switchResults(com.sun.source.tree.SwitchExpressionTree sw) {
      List<ExpressionTree> out = new ArrayList<>();
      for (CaseTree c : sw.getCases()) {
        if (c.getBody() instanceof ExpressionTree e) {
          out.add(e);
        } else {
          new com.sun.source.util.TreeScanner<Void, Void>() {
            @Override
            public Void visitYield(com.sun.source.tree.YieldTree y, Void unused) {
              out.add(y.getValue());
              return null;
            }

            @Override
            public Void visitSwitchExpression(com.sun.source.tree.SwitchExpressionTree n, Void u) {
              return null;
            }

            @Override
            public Void visitLambdaExpression(LambdaExpressionTree n, Void u) {
              return null;
            }

            @Override
            public Void visitClass(ClassTree n, Void u) {
              return null;
            }
          }.scan(c.getBody() != null ? List.of(c.getBody()) : c.getStatements(), null);
        }
      }
      return out;
    }

    /** null literal, or a nullable expression outside a presence check (conditionals: a branch). */
    boolean yieldsAbsent(TreePath path) {
      ExpressionTree e = strip((ExpressionTree) path.getLeaf());
      if (isNullLiteral(e)) {
        return true;
      }
      if (e instanceof ConditionalExpressionTree c) {
        TreePath cp = pathTo(path, e);
        return yieldsAbsent(new TreePath(cp, c.getTrueExpression()))
            || yieldsAbsent(new TreePath(cp, c.getFalseExpression()));
      }
      if (e instanceof com.sun.source.tree.SwitchExpressionTree sw) {
        for (ExpressionTree r : switchResults(sw)) {
          if (yieldsAbsent(TreePath.getPath(unit, r))) {
            return true;
          }
        }
        return false;
      }
      if (optKey(e) != null || empKey(e) != null || bridgeReceiver(e) != null) {
        return !guarded(path);
      }
      return false;
    }

    /** Path to a node reached from path's leaf through parentheses only. */
    static TreePath pathTo(TreePath path, Tree inner) {
      while (path.getLeaf() != inner) {
        path = new TreePath(path, ((JCTree.JCParens) path.getLeaf()).expr);
      }
      return path;
    }

    Symbol.MethodSymbol enclosingMethod() {
      for (TreePath p = getCurrentPath(); p != null; p = p.getParentPath()) {
        Tree t = p.getLeaf();
        if (t instanceof ClassTree) {
          return null;
        }
        if (t instanceof JCTree.JCLambda lambda) {
          return descriptor(lambda);
        }
        if (t instanceof JCTree.JCMethodDecl m) {
          return m.sym;
        }
      }
      return null;
    }

    /** The source method that the lambda implements, or null (library interface). */
    static Symbol.MethodSymbol descriptor(JCTree.JCLambda lambda) {
      if (lambda.type == null || lambda.type.isErroneous() || lambda.type.tsym == null) {
        return null;
      }
      Symbol s;
      try {
        s = types.findDescriptorSymbol(lambda.type.tsym);
      } catch (RuntimeException e) {
        return null;
      }
      return s instanceof Symbol.MethodSymbol m && !isLibrary(m) ? m : null;
    }

    TreePath child(Tree t) {
      return new TreePath(getCurrentPath(), t);
    }

    static boolean hasNullable(List<? extends AnnotationTree> annotations) {
      for (AnnotationTree a : annotations) {
        String n = a.getAnnotationType().toString();
        if (n.equals("Nullable") || n.endsWith(".Nullable")) {
          return true;
        }
      }
      return false;
    }

    /** The declared parameter symbol for argument i, when it is a String, else null. */
    static Symbol.VarSymbol stringParam(Symbol sym, int i) {
      if (!(sym instanceof Symbol.MethodSymbol m) || isLibrary(m) || m.params == null) {
        return null;
      }
      if (i >= m.params.size() || (m.isVarArgs() && i >= m.params.size() - 1)) {
        return null;
      }
      Symbol.VarSymbol p = m.params.get(i);
      return isString(p.type) ? p : null;
    }
  }

  /** Declarations: what can be converted, override families, records. */
  private static final class Declarations extends Base {
    Declarations(CompilationUnitTree unit) throws Exception {
      super(unit);
    }

    private void declare(Object key) {
      if (inScope(file)) {
        seedable.add(find(key));
      }
      String n = name(key);
      if (keep.contains(n)) {
        pinned.add(find(key));
      }
      if (listedEmpty.contains(n)) {
        listedEmptyRoots.add(find(key));
        empty.add(find(key));
        nullable.add(find(key));
        why.put(find(key), "empty.txt");
      }
    }

    @Override
    public Void visitVariable(VariableTree node, Void unused) {
      Symbol.VarSymbol sym = ((JCTree.JCVariableDecl) node).sym;
      if (sym != null && isString(sym.type)) {
        declare(keyOf(sym));
        if (node.getInitializer() == null
            && sym.owner instanceof Symbol.ClassSymbol
            && (sym.flags() & (Flags.FINAL | Flags.STATIC)) == 0) {
          noInitialValue.add(find(sym));
        }
      }
      return super.visitVariable(node, unused);
    }

    @Override
    public Void visitMethod(MethodTree node, Void unused) {
      Symbol.MethodSymbol sym = ((JCTree.JCMethodDecl) node).sym;
      if (sym != null && isString(sym.getReturnType())) {
        declare(sym);
      }
      return super.visitMethod(node, unused);
    }
  }

  private static void buildFamilies(Symbol.ClassSymbol c) {
    for (Symbol member : c.members().getSymbols()) {
      if (member instanceof Symbol.ClassSymbol nested) {
        nested.complete();
      }
      if (!(member instanceof Symbol.MethodSymbol m)
          || m.isConstructor()
          || m.isStatic()
          || (m.flags() & Flags.PRIVATE) != 0) {
        continue;
      }
      for (Type sup : types.closure(c.type)) {
        if (sup.tsym == c) {
          continue;
        }
        for (Symbol s : sup.tsym.members().getSymbolsByName(m.name)) {
          if (s instanceof Symbol.MethodSymbol other && m != other && m.overrides(other, c, types, false)) {
            boolean library = isLibrary(other);
            union(m, other);
            if (library) {
              pinnedRaw.add(m);
            }
            for (int i = 0; i < m.params.size(); i++) {
              ParamKey mine = new ParamKey(m, i);
              if (library) {
                pinnedRaw.add(mine);
              } else if (other.params != null && i < other.params.size()) {
                union(mine, new ParamKey(other, i));
              }
            }
          }
        }
      }
    }
    if ((c.flags() & Flags.RECORD) != 0) {
      List<? extends Symbol.RecordComponent> components = c.getRecordComponents();
      for (Symbol member : c.members().getSymbols()) {
        if (member instanceof Symbol.VarSymbol field && !field.isStatic()) {
          for (Symbol.RecordComponent rc : components) {
            if (rc.name == field.name && rc.accessor != null) {
              union(field, rc.accessor);
            }
          }
        }
        if (member instanceof Symbol.MethodSymbol ctor
            && ctor.isConstructor()
            && ctor.params != null
            && ctor.params.size() == components.size()) {
          boolean canonical = true;
          for (int i = 0; i < components.size(); i++) {
            canonical &= types.isSameType(ctor.params.get(i).type, components.get(i).type);
          }
          if (canonical) {
            for (int i = 0; i < components.size(); i++) {
              for (Symbol f : c.members().getSymbolsByName(components.get(i).name)) {
                if (f instanceof Symbol.VarSymbol) {
                  union(f, new ParamKey(ctor, i));
                }
              }
            }
          }
        }
      }
    }
  }

  /** Classes first, so that the union-find is complete before anything is marked. */
  private static final class Families extends Base {
    Families(CompilationUnitTree unit) throws Exception {
      super(unit);
    }

    @Override
    public Void visitClass(ClassTree node, Void unused) {
      Symbol.ClassSymbol sym = ((JCTree.JCClassDecl) node).sym;
      if (sym != null) {
        buildFamilies(sym);
      }
      return super.visitClass(node, unused);
    }

    /** A lambda parameter is the parameter of the source method it implements, else fixed. */
    @Override
    public Void visitLambdaExpression(LambdaExpressionTree node, Void unused) {
      Symbol.MethodSymbol m = descriptor((JCTree.JCLambda) node);
      List<? extends VariableTree> params = node.getParameters();
      for (int i = 0; i < params.size(); i++) {
        Symbol.VarSymbol p = ((JCTree.JCVariableDecl) params.get(i)).sym;
        if (p == null || !isString(p.type)) {
          continue;
        }
        if (m != null && m.params != null && i < m.params.size()) {
          union(p, new ParamKey(m, i));
        } else {
          pinnedRaw.add(p);
        }
      }
      return super.visitLambdaExpression(node, unused);
    }
  }

  /** Marks nullable declarations; run to a fixed point. */
  private static final class Infer extends Base {
    /** Test code: only `null` passed for a parameter counts. */
    private final boolean testCode;

    Infer(CompilationUnitTree unit) throws Exception {
      super(unit);
      this.testCode = !isProduction(file);
    }

    @Override
    public Void scan(Tree tree, Void unused) {
      if (testCode
          && (tree instanceof VariableTree
              || tree instanceof ReturnTree
              || tree instanceof AssignmentTree
              || tree instanceof BinaryTree
              || tree instanceof MemberSelectTree)) {
        // Skip this node's own rules but keep looking for calls below it.
        return tree.accept(
            new com.sun.source.util.SimpleTreeVisitor<Void, Void>() {
              @Override
              protected Void defaultAction(Tree t, Void u) {
                return null;
              }

              @Override
              public Void visitVariable(VariableTree t, Void u) {
                return Infer.this.scan(t.getInitializer(), null);
              }

              @Override
              public Void visitReturn(ReturnTree t, Void u) {
                return Infer.this.scan(t.getExpression(), null);
              }

              @Override
              public Void visitAssignment(AssignmentTree t, Void u) {
                Infer.this.scan(t.getVariable(), null);
                return Infer.this.scan(t.getExpression(), null);
              }

              @Override
              public Void visitBinary(BinaryTree t, Void u) {
                Infer.this.scan(t.getLeftOperand(), null);
                return Infer.this.scan(t.getRightOperand(), null);
              }

              @Override
              public Void visitMemberSelect(MemberSelectTree t, Void u) {
                return Infer.this.scan(t.getExpression(), null);
              }
            },
            null);
      }
      return super.scan(tree, unused);
    }

    private String at(Tree t) {
      return file.substring(file.lastIndexOf('/') + 1) + ":" + unit.getLineMap().getLineNumber(start(t));
    }

    @Override
    public Void visitVariable(VariableTree node, Void unused) {
      Symbol.VarSymbol sym = ((JCTree.JCVariableDecl) node).sym;
      if (sym != null && isString(sym.type)) {
        Object key = keyOf(sym);
        if (hasNullable(node.getModifiers().getAnnotations())) {
          mark(key, "@Nullable");
        }
        if (force.contains(name(key))) {
          mark(key, "forced");
        }
        if (node.getInitializer() != null && yieldsAbsent(child(node.getInitializer()))) {
          mark(key, "initialised with an absent value " + at(node));
        }
        if (node.getInitializer() != null) {
          flowsInto(key, node.getInitializer());
          if (modeOf(key) == PLAIN) {
            strictUse(node.getInitializer());
          }
        }
      }
      return super.visitVariable(node, unused);
    }

    @Override
    public Void visitMethod(MethodTree node, Void unused) {
      Symbol.MethodSymbol sym = ((JCTree.JCMethodDecl) node).sym;
      if (sym != null && isString(sym.getReturnType())) {
        if (hasNullable(node.getModifiers().getAnnotations())) {
          mark(sym, "@Nullable");
        }
        if (force.contains(name(sym))) {
          mark(sym, "forced");
        }
        // A nullable getter: its field is nullable.
        String reason = String.valueOf(why.get(find(sym)));
        if (!testCode
            && (isN(sym) || isE(sym))
            && (hasNullable(node.getModifiers().getAnnotations())
                || ((reason.startsWith("compared") || reason.startsWith("its result"))
                    && !node.getModifiers().getAnnotations().toString().contains("Override")))
            && node.getBody() != null
            && node.getBody().getStatements().size() == 1
            && node.getBody().getStatements().get(0) instanceof ReturnTree r
            && r.getExpression() != null
            && (strip(r.getExpression()) instanceof IdentifierTree
                || strip(r.getExpression()) instanceof MemberSelectTree)
            && TreeInfo.symbol((JCTree) strip(r.getExpression())) instanceof Symbol.VarSymbol field
            && field.owner instanceof Symbol.ClassSymbol
            && isString(field.type)) {
          mark(field, "returned by a nullable getter");
        }
      }
      return super.visitMethod(node, unused);
    }

    /**
     * A parameter read straight into a nullable declaration is "used as nullable"; it becomes
     * nullable itself when some call also passes it a nullable expression outside a presence check.
     */
    private void flowsInto(Object sinkKey, ExpressionTree e) {
      e = strip(e);
      if (e instanceof ConditionalExpressionTree c) {
        flowsInto(sinkKey, c.getTrueExpression());
        flowsInto(sinkKey, c.getFalseExpression());
        return;
      }
      if (!(isN(sinkKey) || isE(sinkKey))) {
        return;
      }
      // What flows into a variable that is compared with null: a source method's result makes
      // the method nullable. A field that starts without a value is nullable when it flows into
      // any nullable declaration.
      sourceCount.merge(find(sinkKey), 1, Integer::sum);
      if (comparedVars.contains(find(sinkKey))
          && callee(e) instanceof Symbol.MethodSymbol m
          && isString(m.getReturnType())
          && !isLibrary(m)) {
        onlySource.put(find(sinkKey), m);
      }
      if ((e instanceof IdentifierTree || e instanceof MemberSelectTree)
          && TreeInfo.symbol((JCTree) e) instanceof Symbol.VarSymbol field
          && isString(field.type)
          && noInitialValue.contains(find(field))) {
        mark(field, "has no initial value and goes into a nullable");
      }
      if (!(e instanceof IdentifierTree)) {
        return;
      }
      Symbol sym = TreeInfo.symbol((JCTree) e);
      if (sym instanceof Symbol.VarSymbol && isString(sym.type) && (sym.flags() & Flags.PARAMETER) != 0) {
        usedNullable.add(find(keyOf(sym)));
        if (isN(sinkKey)) {
          usedOptional.add(find(keyOf(sym)));
        }
      }
    }

    @Override
    public Void visitReturn(ReturnTree node, Void unused) {
      Symbol.MethodSymbol m = enclosingMethod();
      if (m != null
          && node.getExpression() != null
          && isString(m.getReturnType())
          && yieldsAbsent(child(node.getExpression()))) {
        mark(m, "returns an absent value " + at(node));
      }
      if (m != null && node.getExpression() != null && isString(m.getReturnType())) {
        flowsInto(m, node.getExpression());
      }
      return super.visitReturn(node, unused);
    }

    @Override
    public Void visitLambdaExpression(LambdaExpressionTree node, Void unused) {
      Symbol.MethodSymbol m = descriptor((JCTree.JCLambda) node);
      if (m != null
          && node.getBodyKind() == LambdaExpressionTree.BodyKind.EXPRESSION
          && isString(m.getReturnType())
          && yieldsAbsent(child(node.getBody()))) {
        mark(m, "returns an absent value " + at(node));
      }
      return super.visitLambdaExpression(node, unused);
    }

    @Override
    public Void visitAssignment(AssignmentTree node, Void unused) {
      Symbol sym = TreeInfo.symbol((JCTree) node.getVariable());
      if (sym instanceof Symbol.VarSymbol
          && isString(sym.type)
          && yieldsAbsent(child(node.getExpression()))) {
        mark(keyOf(sym), "assigned an absent value " + at(node));
      }
      if (sym instanceof Symbol.VarSymbol && isString(sym.type)) {
        flowsInto(keyOf(sym), node.getExpression());
        if (modeOf(keyOf(sym)) == PLAIN) {
          strictUse(node.getExpression());
        }
      }
      return super.visitAssignment(node, unused);
    }

    private static Object paramRoot(ExpressionTree e) {
      e = strip(e);
      if (!(e instanceof IdentifierTree)) {
        return null;
      }
      Symbol sym = TreeInfo.symbol((JCTree) e);
      return sym instanceof Symbol.VarSymbol && isString(sym.type) && (sym.flags() & Flags.PARAMETER) != 0
          ? find(keyOf(sym))
          : null;
    }

    /** The parameter is read where null was never acceptable. */
    private void strictUse(ExpressionTree e) {
      Object root = paramRoot(e);
      if (root != null) {
        strictParams.add(root);
      }
    }

    /** The parameter is dereferenced: null was a NullPointerException. */
    private void derefUse(ExpressionTree e) {
      Object root = paramRoot(e);
      if (root != null) {
        strictParams.add(root);
        derefParams.add(root);
      }
    }

    @Override
    public Void visitMemberSelect(MemberSelectTree node, Void unused) {
      derefUse(node.getExpression());
      return super.visitMemberSelect(node, unused);
    }

    /** `switch (name)` throws on null like a dereference. */
    @Override
    public Void visitSwitch(com.sun.source.tree.SwitchTree node, Void unused) {
      derefUse(node.getExpression());
      return super.visitSwitch(node, unused);
    }

    @Override
    public Void visitSwitchExpression(com.sun.source.tree.SwitchExpressionTree node, Void unused) {
      derefUse(node.getExpression());
      return super.visitSwitchExpression(node, unused);
    }

    /** Strict uses among the arguments of a call: library String parameters; source ones later. */
    private void strictArguments(Symbol sym, Type methodType, List<? extends ExpressionTree> args) {
      if (sym == null || !(methodType instanceof Type.MethodType)) {
        return;
      }
      List<Type> formals = methodType.getParameterTypes();
      for (int i = 0; i < args.size(); i++) {
        Object root = paramRoot(args.get(i));
        if (root == null) {
          continue;
        }
        Symbol.VarSymbol q = stringParam(sym, i);
        if (q != null) {
          passedOn.computeIfAbsent(root, k -> new HashSet<>()).add(find(keyOf(q)));
        } else if (isLibrary(sym) && i < formals.size()) {
          boolean check =
              i == 0
                  && (sym.name.contentEquals("checkNotNull")
                      || sym.name.contentEquals("requireNonNull")
                      || sym.name.contentEquals("verifyNotNull"));
          if (check) {
            // This step deletes the check, so it is tracked apart from the dereferences that
            // stay visible in the code.
            assertedParams.add(root);
            strictParams.add(root);
          } else if (rejectsNull(sym) && !sym.name.contentEquals("checkNotNull")) {
            derefParams.add(root);
          }
          Type formal = formals.get(i);
          if (isString(formal)
              || (formal.tsym != null
                  && formal.tsym.getQualifiedName().contentEquals("java.lang.CharSequence"))) {
            strictParams.add(root);
          }
        }
      }
    }

    private void markExpression(ExpressionTree e, String reason) {
      e = strip(e);
      if (e instanceof IdentifierTree || e instanceof MemberSelectTree) {
        Symbol sym = TreeInfo.symbol((JCTree) e);
        if (sym instanceof Symbol.VarSymbol && isString(sym.type)) {
          mark(keyOf(sym), reason);
          if (reason.startsWith("compared")) {
            comparedVars.add(find(keyOf(sym)));
          }
        }
      } else if (callee(e) instanceof Symbol.MethodSymbol m
          && isString(m.getReturnType())
          && !isLibrary(m)) {
        mark(m, reason);
      }
    }

    /**
     * A comparison that only asserts presence is no evidence of a nullable String: the whole
     * condition of `if (param == null) { throw ...; }` (recorded, Rewrite deletes the statement if
     * the parameter stays a String) and anything inside checkState / checkArgument / verify.
     */
    private boolean assertion(BinaryTree node, ExpressionTree operand) {
      TreePath p = getCurrentPath().getParentPath();
      Tree child = node;
      while (p.getLeaf() instanceof JCTree.JCParens) {
        child = p.getLeaf();
        p = p.getParentPath();
      }
      if (p.getLeaf() instanceof IfTree i
          && i.getCondition() == child
          && node.getKind() == Tree.Kind.EQUAL_TO
          && i.getElseStatement() == null) {
        StatementTree then = i.getThenStatement();
        if (then instanceof BlockTree b && b.getStatements().size() == 1) {
          then = b.getStatements().get(0);
        }
        Symbol sym =
            strip(operand) instanceof IdentifierTree ? TreeInfo.symbol((JCTree) strip(operand)) : null;
        if (then.getKind() == Tree.Kind.THROW
            && sym instanceof Symbol.VarSymbol
            && isString(sym.type)
            && (sym.flags() & Flags.PARAMETER) != 0) {
          throwGuards.put(i, keyOf(sym));
          return true;
        }
      }
      for (; p != null; p = p.getParentPath()) {
        Tree t = p.getLeaf();
        if (t instanceof MethodInvocationTree call) {
          Symbol sym = callee(call);
          String n = sym == null ? "" : sym.name.toString();
          return n.equals("checkState") || n.equals("checkArgument") || n.equals("verify");
        }
        if (!(t instanceof JCTree.JCParens)
            && t.getKind() != Tree.Kind.CONDITIONAL_AND
            && t.getKind() != Tree.Kind.CONDITIONAL_OR
            && t.getKind() != Tree.Kind.LOGICAL_COMPLEMENT) {
          return false;
        }
      }
      return false;
    }

    @Override
    public Void visitBinary(BinaryTree node, Void unused) {
      if (node.getKind() == Tree.Kind.EQUAL_TO || node.getKind() == Tree.Kind.NOT_EQUAL_TO) {
        ExpressionTree operand =
            isNullLiteral(node.getRightOperand())
                ? node.getLeftOperand()
                : isNullLiteral(node.getLeftOperand()) ? node.getRightOperand() : null;
        if (operand != null && isString(strip(operand)) && !assertion(node, operand)) {
          markExpression(operand, "compared with null " + at(node));
        }
      }
      return super.visitBinary(node, unused);
    }

    private void arguments(Symbol sym, List<? extends ExpressionTree> args) {
      for (int i = 0; i < args.size(); i++) {
        Symbol.VarSymbol p = stringParam(sym, i);
        if (p == null) {
          continue;
        }
        ExpressionTree e = strip(args.get(i));
        boolean absent = isNullLiteral(e);
        if (e instanceof ConditionalExpressionTree c) {
          absent = isNullLiteral(c.getTrueExpression()) || isNullLiteral(c.getFalseExpression());
        }
        if (absent) {
          mark(keyOf(p), "null passed " + at(args.get(i)));
        }
        if (testCode) {
          continue;
        }
        if (firstRound) {
          callSites.merge(find(keyOf(p)), 1, Integer::sum);
        }
        flowsInto(keyOf(p), args.get(i));
        if (yieldsAbsent(child(args.get(i)))) {
          givenAbsent.add(find(keyOf(p)));
        }
      }
    }

    @Override
    public Void visitMethodInvocation(MethodInvocationTree node, Void unused) {
      arguments(callee(node), node.getArguments());
      if (testCode) {
        return super.visitMethodInvocation(node, unused);
      }
      strictArguments(callee(node), ((JCTree.JCMethodInvocation) node).meth.type, node.getArguments());
      if (isCall(node, "com.google.common.base.Strings", "nullToEmpty")
          || isCall(node, "com.google.common.base.Strings", "emptyToNull")) {
        markExpression(node.getArguments().get(0), "Strings." + callee(node).name + " " + at(node));
      }
      return super.visitMethodInvocation(node, unused);
    }

    @Override
    public Void visitNewClass(NewClassTree node, Void unused) {
      JCTree.JCNewClass call = (JCTree.JCNewClass) node;
      arguments(call.constructor, node.getArguments());
      if (!testCode) {
        strictArguments(call.constructor, call.constructorType, node.getArguments());
      }
      return super.visitNewClass(node, unused);
    }
  }

  /**
   * Decides which nullable locals and method returns stay Strings with "" for absent: those whose
   * every source is an empty-string-mode expression, a null literal or a non-empty string literal,
   * with at least one of the first kind.
   */
  private static final class Modes extends Base {
    Modes(CompilationUnitTree unit) throws Exception {
      super(unit);
    }

    private int sourceKind(ExpressionTree e) {
      e = strip(e);
      if (isNullLiteral(e)) {
        return 1;
      }
      if (e.getKind() == Tree.Kind.STRING_LITERAL) {
        return ((com.sun.source.tree.LiteralTree) e).getValue().toString().isEmpty() ? 0 : 1;
      }
      if (e instanceof ConditionalExpressionTree c) {
        int a = sourceKind(c.getTrueExpression());
        int b = sourceKind(c.getFalseExpression());
        return a == 0 || b == 0 ? 0 : Math.max(a, b);
      }
      return empKey(e) != null ? 2 : 0;
    }

    private void source(Object key, ExpressionTree e) {
      Object root = find(key);
      if (!empty.contains(root) || listedEmptyRoots.contains(root)) {
        return;
      }
      int kind = sourceKind(e);
      if (kind == 0) {
        empty.remove(root);
        changed = true;
      } else if (kind == 2) {
        hasEmptySource.add(root);
      }
    }

    @Override
    public Void visitVariable(VariableTree node, Void unused) {
      Symbol.VarSymbol sym = ((JCTree.JCVariableDecl) node).sym;
      if (sym != null && isString(sym.type) && node.getInitializer() != null) {
        source(keyOf(sym), node.getInitializer());
      }
      return super.visitVariable(node, unused);
    }

    @Override
    public Void visitAssignment(AssignmentTree node, Void unused) {
      Symbol sym = TreeInfo.symbol((JCTree) node.getVariable());
      if (sym instanceof Symbol.VarSymbol && isString(sym.type)) {
        source(keyOf(sym), node.getExpression());
      }
      return super.visitAssignment(node, unused);
    }

    @Override
    public Void visitReturn(ReturnTree node, Void unused) {
      Symbol.MethodSymbol m = enclosingMethod();
      if (m != null && node.getExpression() != null && isString(m.getReturnType())) {
        source(m, node.getExpression());
      }
      return super.visitReturn(node, unused);
    }
  }

  private static final String OPT = "\u0001Optional";

  private static final class Rewrite extends Base {
    final List<Edit> edits = new ArrayList<>();
    final Map<Tree, Boolean> claimed = new IdentityHashMap<>();
    final Set<Integer> typeEdits = new HashSet<>();
    final boolean production;

    Rewrite(CompilationUnitTree unit) throws Exception {
      super(unit);
      this.production = isProduction(file);
    }

    void prefix(Tree e, String text) {
      edits.add(new Edit(start(e), start(e), text, PREFIX, -end(e)));
    }

    void suffix(Tree e, String text) {
      edits.add(new Edit(end(e), end(e), text, SUFFIX, -start(e)));
    }

    void replace(int from, int to, String text) {
      edits.add(new Edit(from, to, text, REPLACE, 0));
    }

    static boolean simple(ExpressionTree e) {
      e = strip(e);
      switch (e.getKind()) {
        case STRING_LITERAL, IDENTIFIER:
          return true;
        case MEMBER_SELECT:
          return simple(((MemberSelectTree) e).getExpression());
        default:
          return false;
      }
    }

    /** Converts the expression at the path to what its context wants. */
    void coerce(TreePath path, int want, boolean strict) {
      ExpressionTree outer = (ExpressionTree) path.getLeaf();
      ExpressionTree e = strip(outer);
      claimed.put(e, true);
      if (isNullLiteral(e)) {
        if (want == OPTIONAL) {
          replace(start(e), end(e), OPT + ".empty()");
          count("null -> Optional.empty()");
        } else if (want == EMPTY) {
          replace(start(e), end(e), "\"\"");
          count("null -> \"\"");
        }
        return;
      }
      if (e instanceof ConditionalExpressionTree c) {
        TreePath cp = pathTo(path, e);
        if (want == PLAIN && orElsePattern(c)) {
          return;
        }
        coerce(new TreePath(cp, c.getTrueExpression()), want, strict);
        coerce(new TreePath(cp, c.getFalseExpression()), want, strict);
        return;
      }
      if (e instanceof com.sun.source.tree.SwitchExpressionTree sw) {
        for (ExpressionTree r : switchResults(sw)) {
          coerce(TreePath.getPath(unit, r), want, strict);
        }
        return;
      }
      ExpressionTree receiver = bridgeReceiver(e);
      if (receiver != null) {
        if (want == OPTIONAL) {
          replace(end(receiver), end(e), "");
          count("drop .orElse(null)");
        } else if (want == EMPTY) {
          replace(end(receiver), end(e), ".orElse(\"\")");
          count(".orElse(null) -> .orElse(\"\")");
        } else if (guarded(path)) {
          replace(end(receiver), end(e), ".get()");
          count(".orElse(null) -> .get()");
        }
        return;
      }
      if (optKey(e) != null) {
        if (want == EMPTY) {
          suffix(e, ".orElse(\"\")");
          count(".orElse(\"\")");
        } else if (want == PLAIN) {
          if (strict || guarded(path)) {
            suffix(e, ".get()");
            count(".get()");
          } else if (!production) {
            suffix(e, ".orElse(null)");
            count(".orElse(null) (test)");
          } else {
            plainUse(path, e);
          }
        }
        return;
      }
      if (empKey(e) != null) {
        if (want == OPTIONAL) {
          prefix(outer, OPT + ".of(");
          if (guarded(path)) {
            suffix(outer, ")");
            count("Optional.of(...)");
          } else {
            suffix(outer, ").filter(s -> !s.isEmpty())");
            count("Optional.of(E).filter(s -> !s.isEmpty())");
          }
        } else if (want == PLAIN && !strict && production && !guarded(path)) {
          flow("EFLOW", path, e);
        }
        return;
      }
      if (want == OPTIONAL && !isOptionalOfString(e)) {
        Symbol sym = callee(e);
        // A source method that returns a type variable (a container's `T get()`) is a boundary
        // like a library call.
        boolean ofNullable =
            sym instanceof Symbol.MethodSymbol m
                && !isLibrary(m)
                && m.getReturnType().hasTag(TypeTag.TYPEVAR);
        if (sym != null && isLibrary(sym)) {
          String owner = sym.owner.getQualifiedName().toString();
          ofNullable =
              owner.equals("java.lang.System")
                  || owner.equals("java.lang.Class")
                  || !(owner.startsWith("java.lang.")
                      || owner.equals("com.google.common.base.Joiner")
                      || owner.equals("com.google.common.base.Ascii")
                      || owner.equals("com.google.common.base.CharMatcher")
                      || owner.equals("com.google.common.base.Strings"));
          if (owner.equals("com.google.common.base.Strings") && sym.name.contentEquals("emptyToNull")) {
            ofNullable = true;
          }
        }
        if (!production && e.getKind() != Tree.Kind.STRING_LITERAL) {
          // Test code keeps its null Strings.
          prefix(outer, OPT + ".ofNullable(");
          suffix(outer, ")");
          count("Optional.ofNullable(...) (test)");
          return;
        }
        prefix(outer, OPT + (ofNullable ? ".ofNullable(" : ".of("));
        suffix(outer, ")");
        count(ofNullable ? "Optional.ofNullable(library call)" : "Optional.of(...)");
      }
    }

    private static final Set<String> PRINTS =
        Set.of(
            "java.lang.StringBuilder#append",
            "java.lang.AbstractStringBuilder#append",
            "java.lang.Appendable#append",
            "java.io.PrintStream#print",
            "java.io.PrintStream#println",
            "java.lang.String#valueOf",
            "com.google.common.base.MoreObjects.ToStringHelper#add");
    private static final Set<String> MESSAGES =
        Set.of(
            "java.lang.String#format",
            "com.google.common.base.Preconditions#checkState",
            "com.google.common.base.Preconditions#checkArgument",
            "com.google.common.base.Preconditions#checkNotNull",
            "com.google.common.base.Verify#verify");

    private void flow(String kind, TreePath path, ExpressionTree e) {
      Tree parent = path.getParentPath().getLeaf();
      String context = parent.getKind().toString();
      if (parent instanceof MethodInvocationTree call && callee(call) != null) {
        Symbol sym = callee(call);
        context = (isLibrary(sym) ? "LIBCALL " : "CALL ") + sym.owner.name + "." + sym.name;
      }
      long line = unit.getLineMap().getLineNumber(start(e));
      flows.add(kind + " " + context + "  " + file + ":" + line + "  " + src.substring(start(e), end(e)));
    }

    /**
     * An Optional-mode expression that is read as a plain String outside a presence check, in
     * production code:
     *   argument for a String parameter (source or library, also `Collection<String>.add`) -> get()
     *   checkNotNull(X) / requireNonNull(X)                    -> X.get()
     *   string concatenation, append/print/valueOf, format and check-message arguments, String
     *   varargs of source methods (diagnostic arguments)       -> X.orElse("null")
     *   "literal".equals(X)                                    -> "literal".equals(X.orElse(""))
     *   a.equals(X)                                            -> Optional.of(a).equals(X)
     *   c.contains(X) / m.containsKey(X), X a name             -> X.isPresent() && c.contains(X.get())
     *   m.get(X), X a name                                     -> X.isPresent() ? m.get(X.get()) : null
     *   anything else                                          -> X.orElse(null)
     */
    private void plainUse(TreePath path, ExpressionTree e) {
      TreePath pp = path.getParentPath();
      Tree child = path.getLeaf();
      while (pp.getLeaf() instanceof JCTree.JCParens) {
        child = pp.getLeaf();
        pp = pp.getParentPath();
      }
      Tree parent = pp.getLeaf();
      if (parent.getKind() == Tree.Kind.PLUS && isString(parent)
          || parent.getKind() == Tree.Kind.PLUS_ASSIGNMENT) {
        suffix(e, ".orElse(\"null\")");
        count(".orElse(\"null\") (printed)");
        return;
      }
      Symbol sym = null;
      Type methodType = null;
      Type varargsElement = null;
      List<? extends ExpressionTree> args = null;
      if (parent instanceof JCTree.JCMethodInvocation call) {
        sym = TreeInfo.symbol(call.meth);
        methodType = call.meth.type;
        varargsElement = call.varargsElement;
        args = call.args;
      } else if (parent instanceof JCTree.JCNewClass call) {
        sym = call.constructor;
        methodType = call.constructorType;
        varargsElement = call.varargsElement;
        args = call.args;
      }
      int index = args == null ? -1 : args.indexOf(child);
      if (sym == null || index < 0 || !(methodType instanceof Type.MethodType)) {
        unproven(path, e);
        return;
      }
      String key = sym.owner.getQualifiedName() + "#" + sym.name;
      String name = sym.name.toString();
      List<Type> formals = methodType.getParameterTypes();
      boolean inVarargs = varargsElement != null && index >= formals.size() - 1;
      Type formal = inVarargs ? varargsElement : formals.get(index);
      boolean library = isLibrary(sym);
      Symbol.VarSymbol param = stringParam(sym, index);
      if ((name.equals("checkNotNull") || name.equals("requireNonNull") || name.equals("verifyNotNull"))
          && index == 0) {
        suffix(e, ".orElseThrow()");
        count(".orElseThrow() (checkNotNull in the old code)");
      } else if (param != null && pinned.contains(find(keyOf(param)))) {
        suffix(e, ".orElseThrow()");
        count(".orElseThrow() (parameter in keep.txt: reviewed)");
      } else if (PRINTS.contains(key) || (MESSAGES.contains(key) && index >= 1)) {
        suffix(e, ".orElse(\"null\")");
        count(".orElse(\"null\") (printed)");
      } else if (!library && inVarargs && isString(formal)) {
        suffix(e, ".orElse(\"null\")");
        count(".orElse(\"null\") (String varargs of a source method)");
      } else if (isString(formal)
          || (formal.tsym != null && formal.tsym.getQualifiedName().contentEquals("java.lang.CharSequence"))) {
        boolean dereferenced =
            library ? rejectsNull(sym) : param != null && derefParams.contains(find(keyOf(param)));
        if (reviewed.contains(siteKey(path, e)) || reviewed.contains(siteKey(path, e) + " :: " + sinkName(path))) {
          unproven(path, e);
        } else if (dereferenced) {
          suffix(e, ".get()");
          count(
              library
                  ? ".get(): argument of a library method that rejects null"
                  : ".get(): argument that the callee dereferences");
        } else if (!library && param != null && assertedParams.contains(find(keyOf(param)))) {
          suffix(e, ".orElseThrow()");
          count(".orElseThrow() (the callee had checkNotNull(parameter), deleted by this step)");
        } else {
          unproven(path, e);
        }
      } else if (name.equals("equals")
          && args.size() == 1
          && parent instanceof JCTree.JCMethodInvocation call
          && call.meth instanceof JCTree.JCFieldAccess select
          && isString(select.selected)
          && optKey(strip(select.selected)) == null) {
        ExpressionTree receiver = strip(select.selected);
        if (receiver.getKind() == Tree.Kind.STRING_LITERAL
            && !((com.sun.source.tree.LiteralTree) receiver).getValue().toString().isEmpty()) {
          suffix(e, ".orElse(\"\")");
          count("\"literal\".equals(X) -> \"literal\".equals(X.orElse(\"\"))");
        } else {
          prefix(select.selected, OPT + ".of(");
          suffix(select.selected, ")");
          count("a.equals(X) -> Optional.of(a).equals(X)");
        }
      } else if (library
          && (name.equals("contains") || name.equals("containsKey"))
          && args.size() == 1
          && simple(e)
          && e.getKind() != Tree.Kind.STRING_LITERAL) {
        String x = src.substring(start(e), end(e));
        Tree grand = pp.getParentPath().getLeaf();
        boolean bare =
            grand instanceof JCTree.JCParens
                || grand.getKind() == Tree.Kind.CONDITIONAL_AND
                || grand.getKind() == Tree.Kind.CONDITIONAL_OR
                || grand instanceof ReturnTree
                || grand instanceof VariableTree;
        prefix(parent, (bare ? "" : "(") + x + ".isPresent() && ");
        suffix(e, ".get()");
        if (!bare) {
          suffix(parent, ")");
        }
        count("c.contains(X) -> X.isPresent() && c.contains(X.get())");
      } else if (library && isMapGet(sym) && args.size() == 1 && simple(e) && !isString(parent)) {
        String x = src.substring(start(e), end(e));
        Tree grand = pp.getParentPath().getLeaf();
        boolean bare =
            grand instanceof JCTree.JCParens
                || grand instanceof ReturnTree
                || grand instanceof VariableTree
                || grand instanceof AssignmentTree;
        prefix(parent, (bare ? "" : "(") + x + ".isPresent() ? ");
        suffix(e, ".get()");
        suffix(parent, " : null" + (bare ? "" : ")"));
        count("m.get(X) -> X.isPresent() ? m.get(X.get()) : null");
      } else {
        unproven(path, e);
      }
    }

    /** `enclosing class#method :: text of X`, the key of a site in reviewed.txt. */
    private String siteKey(TreePath path, ExpressionTree e) {
      String where = "?";
      for (TreePath p = path; p != null; p = p.getParentPath()) {
        if (p.getLeaf() instanceof JCTree.JCMethodDecl m && m.sym != null) {
          where = m.sym.owner.getQualifiedName() + "#" + m.sym.name;
          break;
        }
        if (p.getLeaf() instanceof JCTree.JCClassDecl c && c.sym != null) {
          where = c.sym.getQualifiedName() + "#";
          break;
        }
      }
      return where + " :: " + text(e);
    }

    /**
     * A nullable value leaves the Optional world where nothing proves it present: `orElseThrow()`
     * if reviewed.txt lists the site (presence established by reading), else `orElse(null)`, which
     * is exactly what the code did before (Scan.java reports it as a null literal).
     */
    private String sinkName(TreePath path) {
      TreePath pp = path.getParentPath();
      while (pp.getLeaf() instanceof JCTree.JCParens) {
        pp = pp.getParentPath();
      }
      Tree parent = pp.getLeaf();
      Symbol sym =
          parent instanceof JCTree.JCMethodInvocation call
              ? TreeInfo.symbol(call.meth)
              : parent instanceof JCTree.JCNewClass call ? call.constructor : null;
      return sym != null && sym.owner != null
          ? sym.owner.name + "." + sym.name
          : parent.getKind().toString();
    }

    private void unproven(TreePath path, ExpressionTree e) {
      String key = siteKey(path, e);
      String full = key + " :: " + sinkName(path);
      if (reviewed.contains(full)) {
        key = full;
      }
      if (reviewed.contains(key)) {
        usedReviewed.add(key);
        suffix(e, ".orElseThrow()");
        count(".orElseThrow() (reviewed.txt)");
      } else {
        suffix(e, ".orElse(null)");
        count(".orElse(null)");
        flow("NFLOW", path, e);
        flows.add("KEY " + full);
      }
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

    /** `X != null ? X : y` / `X == null ? y : X` with simple X and y -> `X.orElse(y)`. */
    boolean orElsePattern(ConditionalExpressionTree c) {
      ExpressionTree cond = strip(c.getCondition());
      ExpressionTree x;
      ExpressionTree y;
      if (cond.getKind() == Tree.Kind.NOT_EQUAL_TO) {
        x = strip(c.getTrueExpression());
        y = c.getFalseExpression();
      } else if (cond.getKind() == Tree.Kind.EQUAL_TO) {
        x = strip(c.getFalseExpression());
        y = c.getTrueExpression();
      } else {
        return false;
      }
      BinaryTree b = (BinaryTree) cond;
      if (!isNullLiteral(b.getRightOperand())
          || optKey(x) == null
          || !simple(x)
          || !simple(y)
          || optKey(strip(y)) != null
          || !text(strip(b.getLeftOperand())).equals(text(x))) {
        return false;
      }
      String xs = src.substring(start(x), end(x));
      String ys = src.substring(start(strip(y)), end(strip(y)));
      replace(start(c), end(c), xs + ".orElse(" + ys + ")");
      claimed.put(c, false);
      count("X != null ? X : y -> X.orElse(y)");
      return true;
    }

    @Override
    public Void scan(Tree tree, Void unused) {
      if (tree != null && Boolean.FALSE.equals(claimed.get(tree))) {
        return null; // replaced as a whole
      }
      return super.scan(tree, unused);
    }

    private void defaultUse(ExpressionTree node) {
      if (claimed.containsKey(node)) {
        return;
      }
      if (optKey(node) != null || empKey(node) != null || bridgeReceiver(node) != null) {
        coerce(getCurrentPath(), PLAIN, false);
      }
    }

    @Override
    public Void visitIdentifier(IdentifierTree node, Void unused) {
      defaultUse(node);
      return super.visitIdentifier(node, unused);
    }

    @Override
    public Void visitMemberSelect(MemberSelectTree node, Void unused) {
      defaultUse(node);
      ExpressionTree receiver = strip(node.getExpression());
      if (!claimed.containsKey(receiver) && (optKey(receiver) != null || empKey(receiver) != null)) {
        coerce(child(node.getExpression()), PLAIN, true);
      }
      return super.visitMemberSelect(node, unused);
    }

    @Override
    public Void visitConditionalExpression(ConditionalExpressionTree node, Void unused) {
      if (!claimed.containsKey(node) && isString(node)) {
        coerce(getCurrentPath(), PLAIN, false);
        if (Boolean.FALSE.equals(claimed.get(node))) {
          return null;
        }
      }
      return super.visitConditionalExpression(node, unused);
    }

    private void declaredType(Tree type, List<? extends AnnotationTree> annotations, int mode) {
      if (type == null || !typeEdits.add(start(type))) {
        return;
      }
      if (mode == OPTIONAL) {
        replace(start(type), end(type), OPT + "<String>");
        count("declaration -> Optional<String>");
      } else {
        count("declaration stays String, \"\" for absent");
      }
      for (AnnotationTree a : annotations) {
        String n = a.getAnnotationType().toString();
        if ((n.equals("Nullable") || n.endsWith(".Nullable")) && start(a) >= 0 && end(a) > 0) {
          int to = end(a);
          while (to < src.length() && Character.isWhitespace(src.charAt(to))) {
            to++;
          }
          replace(start(a), to, "");
        }
      }
    }

    @Override
    public Void visitVariable(VariableTree node, Void unused) {
      JCTree.JCVariableDecl decl = (JCTree.JCVariableDecl) node;
      Symbol.VarSymbol sym = decl.sym;
      if (sym != null && isString(sym.type)) {
        int mode = modeOf(keyOf(sym));
        boolean declared = node.getType() != null && start(node.getType()) >= 0 && end(node.getType()) > 0;
        if (mode != PLAIN && declared) {
          declaredType(node.getType(), node.getModifiers().getAnnotations(), mode);
          if (node.getInitializer() == null
              && sym.owner instanceof Symbol.ClassSymbol
              && (sym.flags() & (Flags.FINAL | Flags.RECORD)) == 0) {
            int at = src.indexOf(node.getName().toString(), end(node.getType()));
            int stop = at + node.getName().length();
            edits.add(
                new Edit(stop, stop, mode == EMPTY ? " = \"\"" : " = " + OPT + ".empty()", SUFFIX, 0));
            count("field default -> " + (mode == EMPTY ? "\"\"" : "Optional.empty()"));
          }
        }
        if (node.getInitializer() != null) {
          coerce(child(node.getInitializer()), mode, false);
        }
      }
      return super.visitVariable(node, unused);
    }

    @Override
    public Void visitMethod(MethodTree node, Void unused) {
      Symbol.MethodSymbol sym = ((JCTree.JCMethodDecl) node).sym;
      if (sym != null
          && isString(sym.getReturnType())
          && modeOf(sym) != PLAIN
          && node.getReturnType() != null) {
        declaredType(node.getReturnType(), node.getModifiers().getAnnotations(), modeOf(sym));
      }
      return super.visitMethod(node, unused);
    }

    @Override
    public Void visitReturn(ReturnTree node, Void unused) {
      Symbol.MethodSymbol m = enclosingMethod();
      if (m != null && node.getExpression() != null && isString(m.getReturnType())) {
        coerce(child(node.getExpression()), modeOf(m), false);
      }
      return super.visitReturn(node, unused);
    }

    @Override
    public Void visitLambdaExpression(LambdaExpressionTree node, Void unused) {
      Symbol.MethodSymbol m = descriptor((JCTree.JCLambda) node);
      if (m != null
          && node.getBodyKind() == LambdaExpressionTree.BodyKind.EXPRESSION
          && isString(m.getReturnType())) {
        coerce(child(node.getBody()), modeOf(m), false);
      }
      return super.visitLambdaExpression(node, unused);
    }

    @Override
    public Void visitAssignment(AssignmentTree node, Void unused) {
      Symbol sym = TreeInfo.symbol((JCTree) node.getVariable());
      if (sym instanceof Symbol.VarSymbol && isString(sym.type)) {
        claimed.put(strip(node.getVariable()), true);
        coerce(child(node.getExpression()), modeOf(keyOf(sym)), false);
      }
      return super.visitAssignment(node, unused);
    }

    @Override
    public Void visitCompoundAssignment(CompoundAssignmentTree node, Void unused) {
      if (optKey(strip(node.getVariable())) != null) {
        // x += e  ->  x = Optional.of(x.get() + e)
        if (node.getKind() == Tree.Kind.PLUS_ASSIGNMENT) {
          String x = src.substring(start(node.getVariable()), end(node.getVariable()));
          // Unless a presence check covers it, an absent x printed "null" like any concatenation.
          boolean present = guarded(new TreePath(getCurrentPath(), node.getVariable()));
          replace(
              end(node.getVariable()),
              start(node.getExpression()),
              " = " + OPT + ".of(" + x + (present ? ".get() + " : ".orElse(\"null\") + "));
          suffix(node, ")");
          count("x += e -> x = Optional.of(x.get() + e)");
        } else {
          System.out.println("MANUAL " + file + ": compound assignment to a nullable: " + node);
        }
        claimed.put(strip(node.getVariable()), true);
      }
      return super.visitCompoundAssignment(node, unused);
    }

    private void arguments(Symbol sym, List<? extends ExpressionTree> args) {
      for (int i = 0; i < args.size(); i++) {
        Symbol.VarSymbol p = stringParam(sym, i);
        if (p != null) {
          coerce(child(args.get(i)), modeOf(keyOf(p)), false);
        }
      }
    }

    @Override
    public Void visitNewClass(NewClassTree node, Void unused) {
      arguments(((JCTree.JCNewClass) node).constructor, node.getArguments());
      return super.visitNewClass(node, unused);
    }

    /** Deletes a statement, with its line when it is alone on it. */
    private void deleteStatement(Tree statement) {
      int from = start(statement);
      int to = end(statement);
      int lineStart = src.lastIndexOf('\n', from - 1) + 1;
      int lineEnd = src.indexOf('\n', to);
      if (src.substring(lineStart, from).isBlank() && lineEnd >= 0 && src.substring(to, lineEnd).isBlank()) {
        from = lineStart;
        to = lineEnd + 1;
      }
      replace(from, to, "");
    }

    @Override
    public Void visitIf(IfTree node, Void unused) {
      Object key = throwGuards.get(node);
      if (key != null && production && modeOf(key) == PLAIN) {
        deleteStatement(node);
        count("delete `if (param == null) throw ...`");
        return null;
      }
      return super.visitIf(node, unused);
    }

    private static boolean knownNonNull(Symbol sym) {
      String owner = sym.owner.getQualifiedName().toString();
      return !owner.equals("java.lang.System")
          && !owner.equals("java.lang.Class")
          && (owner.startsWith("java.lang.")
              || owner.equals("com.google.common.base.Joiner")
              || owner.equals("com.google.common.base.Ascii")
              || owner.equals("com.google.common.base.CharMatcher")
              || (owner.equals("com.google.common.base.Strings")
                  && !sym.name.contentEquals("emptyToNull")));
    }

    /**
     * checkNotNull(s) / requireNonNull(s) / verifyNotNull(s) on a plain String, production code:
     * the statement is deleted, an expression is replaced by its argument; when the argument is a
     * call of a library method that may return null, by `Optional.ofNullable(call).get()`.
     */
    private boolean nullCheckCall(MethodInvocationTree node) {
      Symbol sym = callee(node);
      if (sym == null || !production || node.getArguments().isEmpty()) {
        return false;
      }
      String key = sym.owner.getQualifiedName() + "#" + sym.name;
      if (!key.equals("com.google.common.base.Preconditions#checkNotNull")
          && !key.equals("java.util.Objects#requireNonNull")
          && !key.equals("com.google.common.base.Verify#verifyNotNull")) {
        return false;
      }
      ExpressionTree arg = node.getArguments().get(0);
      ExpressionTree e = strip(arg);
      if (!isString(e) || optKey(e) != null || bridgeReceiver(e) != null) {
        return false;
      }
      if (empKey(e) != null) {
        System.out.println("MANUAL " + file + ": " + sym.name + " of an empty-string-mode value: " + node);
        return false;
      }
      Symbol inner = callee(e);
      boolean libraryCall = inner != null && isLibrary(inner) && !knownNonNull(inner);
      Tree parent = getCurrentPath().getParentPath().getLeaf();
      if (libraryCall && node.getArguments().size() > 1) {
        return false; // the message matters: left for a manual rewrite
      }
      if (libraryCall) {
        replace(start(node), start(arg), OPT + ".ofNullable(");
        replace(end(arg), end(node), ").orElseThrow()");
        count("checkNotNull(library call) -> Optional.ofNullable(call).get()");
        return true;
      }
      if (parent instanceof ExpressionStatementTree) {
        deleteStatement(parent);
        claimed.put(node, false);
        count("delete checkNotNull(string);");
        return true;
      }
      switch (e.getKind()) {
        case IDENTIFIER, MEMBER_SELECT, METHOD_INVOCATION, STRING_LITERAL:
          replace(start(node), start(arg), "");
          replace(end(arg), end(node), "");
          count("checkNotNull(string) -> string");
          return true;
        default:
          return false;
      }
    }

    @Override
    public Void visitMethodInvocation(MethodInvocationTree node, Void unused) {
      List<? extends ExpressionTree> args = node.getArguments();
      if (nullCheckCall(node)) {
        if (Boolean.FALSE.equals(claimed.get(node))) {
          return null;
        }
        // Only the first argument survives; the message arguments are gone.
        return scan(args.get(0), unused);
      }
      if (args.size() == 1
          && node.getMethodSelect() instanceof MemberSelectTree select
          && select.getIdentifier().contentEquals("equals")
          && optKey(strip(select.getExpression())) != null
          && optKey(strip(args.get(0))) != null) {
        claimed.put(strip(select.getExpression()), true);
        claimed.put(strip(args.get(0)), true);
        count("X1.equals(X2) compares the Optionals");
      }
      if (args.size() == 1 && optKey(strip(args.get(0))) != null) {
        ExpressionTree x = args.get(0);
        if (isCall(node, "com.google.common.base.Strings", "nullToEmpty")) {
          replace(start(node), start(x), "");
          replace(end(x), end(node), ".orElse(\"\")");
          claimed.put(strip(x), true);
          count("Strings.nullToEmpty(X) -> X.orElse(\"\")");
        } else if (isCall(node, "com.google.common.base.Strings", "isNullOrEmpty")) {
          replace(start(node), start(x), "");
          replace(end(x), end(node), ".orElse(\"\").isEmpty()");
          claimed.put(strip(x), true);
          count("Strings.isNullOrEmpty(X) -> X.orElse(\"\").isEmpty()");
        }
      }
      if (args.size() == 1 && empKey(strip(args.get(0))) != null) {
        ExpressionTree x = args.get(0);
        if (isCall(node, "com.google.common.base.Strings", "nullToEmpty")) {
          replace(start(node), start(x), "");
          replace(end(x), end(node), "");
          claimed.put(strip(x), true);
          count("Strings.nullToEmpty(E) -> E");
        } else if (isCall(node, "com.google.common.base.Strings", "isNullOrEmpty")) {
          replace(start(node), start(x), "");
          replace(end(x), end(node), ".isEmpty()");
          claimed.put(strip(x), true);
          count("Strings.isNullOrEmpty(E) -> E.isEmpty()");
        }
      }
      if (args.size() == 2
          && isCall(node, "java.util.Objects", "equals")
          && optKey(strip(args.get(0))) != null
          && optKey(strip(args.get(1))) != null) {
        replace(start(node), start(args.get(0)), "");
        replace(end(args.get(0)), start(args.get(1)), ".equals(");
        claimed.put(strip(args.get(0)), true);
        claimed.put(strip(args.get(1)), true);
        count("Objects.equals(X1, X2) -> X1.equals(X2)");
      }
      arguments(callee(node), args);
      defaultUse(node);
      return super.visitMethodInvocation(node, unused);
    }

    @Override
    public Void visitBinary(BinaryTree node, Void unused) {
      if (node.getKind() == Tree.Kind.EQUAL_TO || node.getKind() == Tree.Kind.NOT_EQUAL_TO) {
        ExpressionTree l = node.getLeftOperand();
        ExpressionTree r = node.getRightOperand();
        ExpressionTree operand = isNullLiteral(r) ? l : isNullLiteral(l) ? r : null;
        if (operand != null) {
          ExpressionTree e = strip(operand);
          ExpressionTree receiver = bridgeReceiver(e);
          if (empKey(e) != null) {
            if (operand == l) {
              replace(end(operand), end(node), ".isEmpty()");
              if (node.getKind() == Tree.Kind.NOT_EQUAL_TO) {
                prefix(node, "!");
              }
            } else {
              replace(start(node), start(operand), node.getKind() == Tree.Kind.NOT_EQUAL_TO ? "!" : "");
              suffix(node, ".isEmpty()");
            }
            claimed.put(e, true);
            count("null comparison -> isEmpty() on a String");
          } else if (optKey(e) != null || receiver != null) {
            String test = node.getKind() == Tree.Kind.EQUAL_TO ? ".isEmpty()" : ".isPresent()";
            Tree keepTree = receiver != null ? receiver : operand;
            if (operand == l) {
              replace(end(keepTree), end(node), test);
            } else {
              replace(start(node), start(operand), "");
              replace(end(keepTree), end(node), test);
            }
            claimed.put(e, true);
            count("null comparison -> isEmpty()/isPresent()");
          }
        }
      }
      return super.visitBinary(node, unused);
    }

    /** Applies the edits; returns the new text or null if nothing changed. */
    String apply() {
      if (edits.isEmpty()) {
        return null;
      }
      edits.sort(
          (a, b) -> {
            if (a.start != b.start) {
              return Integer.compare(a.start, b.start);
            }
            if (a.kind != b.kind) {
              // At one position: closing suffixes, then replacements, then opening prefixes;
              // a zero-length replacement cannot occur.
              int ka = a.kind == SUFFIX ? 0 : a.kind == PREFIX ? 1 : 2;
              int kb = b.kind == SUFFIX ? 0 : b.kind == PREFIX ? 1 : 2;
              return Integer.compare(ka, kb);
            }
            return Integer.compare(a.order, b.order);
          });
      boolean guavaOptional = false;
      boolean hasImport = false;
      for (ImportTree i : unit.getImports()) {
        String n = i.getQualifiedIdentifier().toString();
        guavaOptional |= n.equals("com.google.common.base.Optional");
        hasImport |= n.equals("java.util.Optional");
      }
      String opt = guavaOptional ? "java.util.Optional" : "Optional";
      StringBuilder out = new StringBuilder();
      int pos = 0;
      for (Edit e : edits) {
        if (e.start < 0 || e.end < 0) {
          continue; // compiler-generated tree (record members)
        }
        if (e.start < pos) {
          System.out.println("CONFLICT " + file + " at offset " + e.start + ": " + e.text);
          continue;
        }
        out.append(src, pos, e.start).append(e.text.replace(OPT, opt));
        pos = e.end;
      }
      out.append(src, pos, src.length());
      String result = out.toString();
      if (!guavaOptional && !hasImport && result.contains("Optional")) {
        result = addImport(result, "java.util.Optional");
      }
      if (!result.contains("@Nullable")) {
        result = result.replace("import org.jspecify.annotations.Nullable;\n", "");
      }
      if (!result.contains("checkNotNull(")) {
        result =
            result.replace(
                "import static com.google.common.base.Preconditions.checkNotNull;\n", "");
      }
      if (!result.contains("requireNonNull(")) {
        result = result.replace("import static java.util.Objects.requireNonNull;\n", "");
      }
      if (!result.contains("Objects.") && src.contains("Objects.")) {
        result = result.replace("import java.util.Objects;\n", "");
      }
      if (!result.contains("Strings.") && src.contains("Strings.")) {
        result = result.replace("import com.google.common.base.Strings;\n", "");
      }
      return result;
    }

    static String addImport(String text, String name) {
      String line = "import " + name + ";\n";
      int best = -1;
      int last = -1;
      int at = 0;
      while (true) {
        int i = text.indexOf("\nimport ", at);
        if (i < 0) {
          break;
        }
        int eol = text.indexOf('\n', i + 1);
        String existing = text.substring(i + 1, eol + 1);
        if (!existing.startsWith("import static ")) {
          last = eol + 1;
          if (existing.compareTo(line) > 0 && best < 0) {
            best = i + 1;
          }
        }
        at = eol;
      }
      if (best >= 0) {
        return text.substring(0, best) + line + text.substring(best);
      }
      if (last >= 0) {
        return text.substring(0, last) + line + text.substring(last);
      }
      int pkg = text.indexOf("\npackage ");
      int eol = text.indexOf('\n', pkg + 1);
      return text.substring(0, eol + 1) + "\n" + line + text.substring(eol + 1);
    }
  }

  private static void readList(String fileName, Set<String> into) throws Exception {
    Path p = Paths.get("tools/prep/10_find_null_strings/" + fileName);
    if (Files.exists(p)) {
      for (String line : Files.readAllLines(p, StandardCharsets.UTF_8)) {
        int hash = line.indexOf(" ## ");
        String key = (hash >= 0 ? line.substring(0, hash) : line).trim();
        if (!key.isEmpty() && !key.startsWith("#")) {
          into.add(key);
        }
      }
    }
  }

  public static void main(String[] argv) throws Exception {
    List<String> args = new ArrayList<>(List.of(argv));
    boolean dry = args.remove("--dry");
    cascade = args.remove("--cascade");
    while (args.contains("--scope")) {
      int i = args.indexOf("--scope");
      args.remove(i);
      scopes.add(args.remove(i));
    }
    readList("keep.txt", keep);
    readList("force.txt", force);
    readList("empty.txt", listedEmpty);
    readList("reviewed.txt", reviewed);
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
    for (CompilationUnitTree unit : parsed) {
      new Families(unit).scan(unit, null);
    }
    for (Object raw : pinnedRaw) {
      pinned.add(find(raw));
    }
    for (CompilationUnitTree unit : parsed) {
      new Declarations(unit).scan(unit, null);
    }
    int rounds = 0;
    do {
      changed = false;
      rounds++;
      strictParams.clear();
      derefParams.clear();
      assertedParams.clear();
      passedOn.clear();
      sourceCount.clear();
      onlySource.clear();
      for (CompilationUnitTree unit : parsed) {
        new Infer(unit).scan(unit, null);
      }
      for (Map.Entry<Object, Symbol.MethodSymbol> e : onlySource.entrySet()) {
        if (sourceCount.get(e.getKey()) == 1
            && e.getKey() instanceof Symbol.VarSymbol v
            && !(v.owner instanceof Symbol.ClassSymbol)
            && (v.flags() & Flags.PARAMETER) == 0) {
          mark(e.getValue(), "its result is the only value of a variable that is compared with null");
        }
      }
      // A parameter handed to a strict parameter is strict; one handed to a nullable one is not.
      boolean grew = true;
      while (grew) {
        grew = false;
        for (Map.Entry<Object, Set<Object>> e : passedOn.entrySet()) {
          if (!strictParams.contains(e.getKey())) {
            for (Object q : e.getValue()) {
              if (strictParams.contains(q) && !nullable.contains(q)) {
                grew |= strictParams.add(e.getKey());
              }
              if (derefParams.contains(q) && !nullable.contains(q)) {
                grew |= derefParams.add(e.getKey());
              }
              if (assertedParams.contains(q) && !nullable.contains(q)) {
                grew |= assertedParams.add(e.getKey());
              }
            }
          }
        }
      }
      firstRound = false;
      for (Object root : givenAbsent) {
        if (usedNullable.contains(root)) {
          mark(root, "is passed a nullable and goes into a nullable");
        } else if (cascade || !strictParams.contains(root)) {
          mark(root, "is passed a nullable and never needs it present [" + callSites.get(root) + " call sites]");
        }
      }
    } while (changed);
    // Empty-string mode: start from every nullable local and method return, drop what has a
    // source that could be a legitimate "" or has no empty-string-mode source at all.
    for (Object root : nullable) {
      if (root instanceof Symbol.MethodSymbol
          || (root instanceof Symbol.VarSymbol v
              && !(v.owner instanceof Symbol.ClassSymbol)
              && (v.flags() & Flags.PARAMETER) == 0)) {
        empty.add(root);
      }
    }
    do {
      changed = false;
      hasEmptySource.clear();
      for (CompilationUnitTree unit : parsed) {
        if (isProduction(unit.getSourceFile().getName())) {
          new Modes(unit).scan(unit, null);
        }
      }
      for (Object root : new ArrayList<>(empty)) {
        if (!listedEmptyRoots.contains(root) && !hasEmptySource.contains(root)) {
          empty.remove(root);
          changed = true;
        }
      }
    } while (changed);
    // A parameter that became nullable only because it feeds empty-string-mode declarations is
    // one itself. (usedOptional was collected while those declarations still counted as Optional
    // candidates, so recompute it now that the modes are known.)
    usedNullable.clear();
    usedOptional.clear();
    for (CompilationUnitTree unit : parsed) {
      if (isProduction(unit.getSourceFile().getName())) {
        new Infer(unit).scan(unit, null);
      }
    }
    for (Object root : nullable) {
      if ("is passed a nullable and goes into a nullable".equals(why.get(root))
          && usedNullable.contains(root)
          && !usedOptional.contains(root)) {
        empty.add(root);
      }
    }
    List<String> report = new ArrayList<>();
    for (Object root : nullable) {
      if (!pinned.contains(root)) {
        report.add(
            (empty.contains(root) ? "EMPTYMODE " : "NULLABLE ") + name(root) + "  (" + why.get(root) + ")");
      }
    }
    report.sort(null);
    for (String s : report) {
      System.out.println(s);
    }
    for (String s : pinnedButNullable) {
      System.out.println(s);
    }
    int written = 0;
    for (CompilationUnitTree unit : parsed) {
      Rewrite rewrite = new Rewrite(unit);
      rewrite.scan(unit, null);
      String result = rewrite.apply();
      if (result != null && !result.equals(rewrite.src)) {
        written++;
        if (!dry) {
          Files.writeString(Paths.get(rewrite.file), result, StandardCharsets.UTF_8);
        }
      }
    }
    for (String s : flows) {
      System.out.println(s);
    }
    for (String key : reviewed) {
      if (!usedReviewed.contains(key)) {
        System.out.println("UNUSED reviewed.txt entry: " + key);
      }
    }
    for (Map.Entry<String, int[]> e : stats.entrySet()) {
      System.out.println("  " + e.getValue()[0] + "  " + e.getKey());
    }
    System.out.println(
        "nullable declarations: " + report.size() + " (" + rounds + " rounds), files "
            + (dry ? "that would change: " : "rewritten: ") + written);
  }
}
