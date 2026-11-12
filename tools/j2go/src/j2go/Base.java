package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/** State shared by the expression, statement and declaration emitters of one compilation unit. */
abstract class Base {
  final Ctx cx;
  final CompilationUnitTree cu;
  final IdentityHashMap<Tree, TypeMirror> typeMap = new IdentityHashMap<>();
  final IdentityHashMap<Tree, Element> elemMap = new IdentityHashMap<>();
  /** Top-level declarations of the Go file being written. */
  final StringBuilder file = new StringBuilder();

  static final class ClassCtx {
    TypeElement te;
    ClassTree tree;
    boolean hasOuter;
    List<VariableElement> captures = List.of();
    /** Local variables declared inside this class body. */
    Set<Element> ownLocals = new HashSet<>();
  }

  static final class TryCtx {
    int id;
    boolean usesReturn;
    boolean inHandler;
    List<String> exits = new ArrayList<>(); // "break L" / "continue L" / "break" / "continue"
  }

  static final class MethodCtx {
    TypeMirror ret; // erased return type the Go function declares; null for void
    boolean isVoid = true;
    Map<Element, String> names = new HashMap<>();
    Set<Element> reads = new HashSet<>();
    /** Enclosing constructs from outermost to innermost: TryCtx, or String label / "" for loops and switches. */
    ArrayList<Object> scopes = new ArrayList<>();
    ArrayList<String> scopeKinds = new ArrayList<>(); // "try", "loop", "switch", "block"
    ArrayList<String> scopeLabels = new ArrayList<>();
    int counter;
    Set<String> usedLabels = new HashSet<>();
    /** Declarations to put before the statement being emitted (pattern bindings). */
    List<String> pre = new ArrayList<>();
    Set<String> hoisted = new HashSet<>();
    boolean staticCtx;
    /** Go expression for the receiver object: "this", or null in a static context. */
  }

  final ArrayDeque<ClassCtx> classStack = new ArrayDeque<>();
  MethodCtx mc;
  /** Classes found inside method bodies, emitted after the class that contains them. */
  final ArrayList<Runnable> pendingClasses = new ArrayList<>();

  Base(Ctx cx, CompilationUnitTree cu) {
    this.cx = cx;
    this.cu = cu;
    new TreePathScanner<Void, Void>() {
      @Override
      public Void scan(Tree t, Void v) {
        if (t == null) return null;
        TreePath p = new TreePath(getCurrentPath(), t);
        try {
          if (t instanceof ExpressionTree || t instanceof VariableTree || t instanceof PatternTree) {
            TypeMirror tm = cx.trees.getTypeMirror(p);
            if (tm != null) typeMap.put(t, tm);
          }
          Element e = cx.trees.getElement(p);
          if (e != null) elemMap.put(t, e);
        } catch (RuntimeException ex) {
          // Some trees (e.g. inside annotations) have no attribution.
        }
        return super.scan(t, v);
      }
    }.scan(new TreePath(cu), null);
  }

  /** Local variables of enclosing methods that a local or anonymous class uses. */
  List<VariableElement> capturesOf(TypeElement te) {
    List<VariableElement> c = cx.captureCache.get(te);
    if (c != null) return c;
    if (te.getNestingKind() != NestingKind.LOCAL && te.getNestingKind() != NestingKind.ANONYMOUS) return List.of();
    ClassTree ct = cx.trees.getTree(te);
    if (ct == null) return List.of();
    Set<Element> own = ownLocals(ct);
    LinkedHashSet<VariableElement> out = new LinkedHashSet<>();
    new TreeScanner<Void, Void>() {
      @Override
      public Void visitIdentifier(IdentifierTree t, Void v) {
        Element e = elemMap.get(t);
        if (isLocalKind(e) && !own.contains(e)) out.add((VariableElement) e);
        return null;
      }
    }.scan(ct, null);
    c = new ArrayList<>(out);
    cx.captureCache.put(te, c);
    return c;
  }

  Set<Element> ownLocals(Tree ct) {
    Set<Element> own = new HashSet<>();
    new TreeScanner<Void, Void>() {
      @Override
      public Void visitVariable(VariableTree t, Void v) {
        Element e = elemMap.get(t);
        if (e != null) own.add(e);
        return super.visitVariable(t, v);
      }
    }.scan(ct, null);
    return own;
  }

  TypeMirror typeOf(Tree t) {
    TypeMirror tm = typeMap.get(t);
    if (tm == null && t instanceof ParenthesizedTree p) return typeOf(p.getExpression());
    return tm;
  }

  Element elemOf(Tree t) {
    return elemMap.get(t);
  }

  ClassCtx cc() {
    return classStack.peek();
  }

  String fresh(String base) {
    return base + "_" + (++mc.counter);
  }

  static String indent(String code, String ind) {
    if (code.isEmpty()) return code;
    StringBuilder sb = new StringBuilder();
    for (String line : code.split("\n", -1)) {
      if (!line.isEmpty()) sb.append(ind).append(line);
      sb.append('\n');
    }
    sb.setLength(sb.length() - 1);
    return sb.toString();
  }

  static ExpressionTree strip(ExpressionTree t) {
    while (t instanceof ParenthesizedTree p) t = p.getExpression();
    return t;
  }

  static boolean isLocalKind(Element e) {
    if (e == null) return false;
    switch (e.getKind()) {
      case LOCAL_VARIABLE, PARAMETER, EXCEPTION_PARAMETER, RESOURCE_VARIABLE, BINDING_VARIABLE:
        return true;
      default:
        return false;
    }
  }

  String localName(Element e) {
    String n = mc.names.get(e);
    if (n == null) {
      n = Ctx.safe(e.getSimpleName().toString());
      mc.names.put(e, n);
    }
    return n;
  }

  RuntimeException unsupported(Tree t, String what) {
    long pos = cx.trees.getSourcePositions().getStartPosition(cu, t);
    long line = pos < 0 ? -1 : cu.getLineMap().getLineNumber(pos);
    return new UnsupportedOperationException(what + " at " + cu.getSourceFile().getName() + ":" + line);
  }

  static String quote(String s) {
    StringBuilder sb = new StringBuilder("\"");
    for (int i = 0; i < s.length(); i++) {
      char c = s.charAt(i);
      switch (c) {
        case '"': sb.append("\\\""); break;
        case '\\': sb.append("\\\\"); break;
        case '\n': sb.append("\\n"); break;
        case '\r': sb.append("\\r"); break;
        case '\t': sb.append("\\t"); break;
        default:
          if (c >= 0x20 && c < 0x7f) {
            sb.append(c);
          } else if (Character.isHighSurrogate(c) && i + 1 < s.length() && Character.isLowSurrogate(s.charAt(i + 1))) {
            sb.append(String.format("\\U%08x", Character.toCodePoint(c, s.charAt(i + 1))));
            i++;
          } else if (Character.isSurrogate(c)) {
            // A lone surrogate: its three-byte (WTF-8) encoding.
            sb.append(String.format("\\x%02x\\x%02x\\x%02x", 0xE0 | (c >> 12), 0x80 | ((c >> 6) & 0x3F), 0x80 | (c & 0x3F)));
          } else if (c < 0x20 || c == 0x7f) {
            sb.append(String.format("\\x%02x", (int) c));
          } else {
            sb.append(String.format("\\u%04x", (int) c));
          }
      }
    }
    return sb.append('"').toString();
  }
}
