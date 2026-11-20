package j2go;

import com.sun.source.tree.*;
import java.math.BigInteger;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/** Expressions. Every method returns Go source text. */
abstract class ExprEmitter extends Base {
  ExprEmitter(Ctx cx, CompilationUnitTree cu) {
    super(cx, cu);
  }

  abstract String blockCode(BlockTree b);

  abstract String switchExpr(SwitchExpressionTree t);

  abstract void queueLocalClass(ClassTree t);

  /** One actual argument: a tree, or already-emitted code with its Java type. */
  final class Arg {
    ExpressionTree tree;
    String code;
    TypeMirror type;

    Arg(ExpressionTree t) {
      tree = t;
      type = typeOf(t);
    }

    Arg(String code, TypeMirror type) {
      this.code = code;
      this.type = type;
    }

    String as(TypeMirror target) {
      return tree != null ? exprAs(tree, target) : cx.coerce(code, type, target);
    }
  }

  List<Arg> args(List<? extends ExpressionTree> l) {
    List<Arg> out = new ArrayList<>();
    for (ExpressionTree t : l) out.add(new Arg(t));
    return out;
  }

  TypeMirror prim(TypeKind k) {
    return cx.types.getPrimitiveType(k);
  }

  TypeMirror objectType() {
    return cx.elems.getTypeElement("java.lang.Object").asType();
  }

  TypeMirror stringType() {
    return cx.elems.getTypeElement("java.lang.String").asType();
  }

  // ------------------------------------------------------------ constants

  /** Value of an integral constant expression (Integer, Long, Character, Short, Byte), else null. */
  Object constOf(ExpressionTree t0) {
    ExpressionTree t = strip(t0);
    switch (t.getKind()) {
      case INT_LITERAL, LONG_LITERAL, CHAR_LITERAL:
        return ((LiteralTree) t).getValue();
      case IDENTIFIER, MEMBER_SELECT: {
        Element e = elemOf(t);
        if (e instanceof VariableElement v) {
          Object c = v.getConstantValue();
          if (c instanceof Integer || c instanceof Long || c instanceof Character || c instanceof Short || c instanceof Byte) return c;
        }
        return null;
      }
      case TYPE_CAST: {
        Object v = constOf(((TypeCastTree) t).getExpression());
        TypeMirror to = typeOf(t);
        if (v == null || to == null || !to.getKind().isPrimitive()) return null;
        long l = asLong(v);
        switch (to.getKind()) {
          case INT: return (int) l;
          case LONG: return l;
          case CHAR: return (char) l;
          case SHORT: return (short) l;
          case BYTE: return (byte) l;
          default: return null;
        }
      }
      case UNARY_MINUS, UNARY_PLUS, BITWISE_COMPLEMENT: {
        Object v = constOf(((UnaryTree) t).getExpression());
        if (v == null) return null;
        boolean isLong = v instanceof Long;
        long l = asLong(v);
        long r = t.getKind() == Tree.Kind.UNARY_MINUS ? -l : t.getKind() == Tree.Kind.UNARY_PLUS ? l : ~l;
        return isLong ? (Object) r : (Object) (int) r;
      }
      default:
        if (t instanceof BinaryTree b) {
          Object x = constOf(b.getLeftOperand()), y = constOf(b.getRightOperand());
          if (x == null || y == null) return null;
          boolean shift = t.getKind() == Tree.Kind.LEFT_SHIFT || t.getKind() == Tree.Kind.RIGHT_SHIFT || t.getKind() == Tree.Kind.UNSIGNED_RIGHT_SHIFT;
          boolean isLong = shift ? x instanceof Long : (x instanceof Long || y instanceof Long);
          long a = asLong(x), c = asLong(y);
          long r;
          switch (t.getKind()) {
            case PLUS: r = a + c; break;
            case MINUS: r = a - c; break;
            case MULTIPLY: r = a * c; break;
            case DIVIDE: if (c == 0) return null; r = isLong ? a / c : (int) a / (int) c; break;
            case REMAINDER: if (c == 0) return null; r = isLong ? a % c : (int) a % (int) c; break;
            case AND: r = a & c; break;
            case OR: r = a | c; break;
            case XOR: r = a ^ c; break;
            case LEFT_SHIFT: r = isLong ? a << c : (int) a << c; break;
            case RIGHT_SHIFT: r = isLong ? a >> c : (int) a >> c; break;
            case UNSIGNED_RIGHT_SHIFT: r = isLong ? a >>> c : (int) a >>> c; break;
            default: return null;
          }
          return isLong ? (Object) r : (Object) (int) r;
        }
        return null;
    }
  }

  static long asLong(Object v) {
    if (v instanceof Character c) return c;
    return ((Number) v).longValue();
  }

  /** True for trees whose Go spelling is an untyped numeric constant. */
  boolean isNumericLiteral(ExpressionTree t0) {
    ExpressionTree t = strip(t0);
    switch (t.getKind()) {
      case INT_LITERAL, LONG_LITERAL, CHAR_LITERAL, FLOAT_LITERAL, DOUBLE_LITERAL:
        return true;
      case UNARY_MINUS:
        return isNumericLiteral(((UnaryTree) t).getExpression());
      default:
        return t instanceof BinaryTree && constOf(t) != null && !mentionsNames(t);
    }
  }

  boolean mentionsNames(ExpressionTree t0) {
    ExpressionTree t = strip(t0);
    if (t instanceof LiteralTree) return false;
    if (t instanceof UnaryTree u) return mentionsNames(u.getExpression());
    if (t instanceof BinaryTree b) return mentionsNames(b.getLeftOperand()) || mentionsNames(b.getRightOperand());
    if (t instanceof TypeCastTree c) return mentionsNames(c.getExpression());
    return true;
  }

  String literal(LiteralTree t) {
    Object v = t.getValue();
    switch (t.getKind()) {
      case INT_LITERAL, LONG_LITERAL: return v.toString();
      case FLOAT_LITERAL: {
        float f = (Float) v;
        return "float32(" + Float.toString(f) + ")";
      }
      case DOUBLE_LITERAL: {
        double d = (Double) v;
        String s = Double.toString(d);
        return s;
      }
      case BOOLEAN_LITERAL: return v.toString();
      case CHAR_LITERAL: return charLit((Character) v);
      case STRING_LITERAL: return quote((String) v);
      case NULL_LITERAL: return "nil";
      default: throw unsupported(t, "literal " + t.getKind());
    }
  }

  static String charLit(char c) {
    if (c == '\'') return "'\\''";
    if (c == '\\') return "'\\\\'";
    if (c >= 0x20 && c < 0x7f) return "'" + c + "'";
    if (c == '\n') return "'\\n'";
    if (c == '\t') return "'\\t'";
    if (c == '\r') return "'\\r'";
    return String.format("0x%04X", (int) c);
  }

  // ------------------------------------------------------------ entry points

  String exprAs(ExpressionTree t, TypeMirror target) {
    TypeMirror from = typeOf(t);
    target = cx.erase(target);
    if (cx.isTypedColl(target)) {
      String made = newTyped(strip(t), target);
      if (made != null) return made;
      if (strip(t) instanceof MethodInvocationTree mi && Ctx.isUnmodifiable(elemOf(mi.getMethodSelect()), mi)) return exprAs(mi.getArguments().get(0), target);
    }
    if (cx.lg.is(target) && strip(t) instanceof MethodInvocationTree mi && cx.lg.passes(elemOf(mi.getMethodSelect()), mi)) {
      // Collections.unmodifiableCollection(x) and its like where a typed interface is wanted: the
      // runtime's is its argument, so the value is x at that type (no detour through the erased form)
      return exprAs(mi.getArguments().get(0), target);
    }
    Ctx.K kt = cx.kind(target);
    if (kt == Ctx.K.PRIM && target.getKind() != TypeKind.BOOLEAN && isNumericLiteral(t)) {
      TypeMirror ft = cx.unboxed(from);
      boolean floatLit = ft != null && (ft.getKind() == TypeKind.FLOAT || ft.getKind() == TypeKind.DOUBLE);
      boolean floatTarget = target.getKind() == TypeKind.FLOAT || target.getKind() == TypeKind.DOUBLE;
      if (!floatLit || floatTarget) {
        Object c = constOf(t);
        if (c != null && !floatLit) {
          // Narrowing of a constant: let Java's value decide, Go rejects out-of-range constants.
          long l = asLong(c);
          switch (target.getKind()) {
            case BYTE: return Long.toString((byte) l);
            case SHORT: return Long.toString((short) l);
            case CHAR: return charLit((char) l);
            case INT: return Long.toString((int) l);
            default: break;
          }
        }
        return expr(t);
      }
    }
    if (strip(t) instanceof LambdaExpressionTree || strip(t) instanceof MemberReferenceTree) {
      // Poly expressions take their type from the context when javac recorded none.
      if (from == null) from = target;
      // rule "function types": the func literal is written at the type of its place
      if (cx.fn.is(target) && cx.fn.is(cx.erase(from)) && Ctx.typeElement(target).equals(Ctx.typeElement(cx.erase(from)))) {
        return strip(t) instanceof LambdaExpressionTree l ? lambda(l, target) : memberRef((MemberReferenceTree) strip(t), target);
      }
    }
    if (from == null) return expr(t);
    if (strip(t) instanceof MethodInvocationTree mi && cx.kind(target) != Ctx.K.PRIM && cx.kind(from) != Ctx.K.PRIM) {
      // Convert the call's declared result straight to the wanted type, without a detour through the
      // (possibly narrower) static type of the call expression.
      CallResult r = invocation(mi);
      if (r.type.getKind() != TypeKind.VOID) {
        // An array keeps its element representation ([]string is not []any), so a T[] result must
        // become its static array type before it is put into an Object.
        TypeMirror ef = cx.erase(from);
        if (ef.getKind() == TypeKind.ARRAY && target.getKind() != TypeKind.ARRAY) {
          return cx.coerce(cx.coerce(r.code, r.type, ef), ef, target);
        }
        return cx.coerce(r.code, r.type, target);
      }
    }
    if (strip(t).getKind() == Tree.Kind.CONDITIONAL_EXPRESSION && cx.kind(from) != Ctx.K.PRIM) {
      // Push the target into the branches: avoids typed-nil and double conversions.
      ConditionalExpressionTree c = (ConditionalExpressionTree) strip(t);
      return conditional(c, target);
    }
    return cx.coerce(expr(t), from, target);
  }

  /**
   * `new ArrayList<>(..)` or `new HashSet<>(..)` where a typed collection is wanted (rule "typed
   * collections"): the runtime's typed collection with the conversion of its element type; null
   * for any other expression.
   */
  String newTyped(ExpressionTree t, TypeMirror target) {
    if (t instanceof MethodInvocationTree mi) return immutableOf(mi, target);
    if (!(t instanceof NewClassTree n) || n.getClassBody() != null) return null;
    // the class of the constructor: the expression's own type is the typed builder's for the new builder of a chain
    TypeElement made = elemOf(n) instanceof ExecutableElement ce ? cx.ownerOf(ce) : Ctx.typeElement(cx.erase(typeOf(t)));
    String fn = made == null ? null : cx.typedColl(target).made.get(made.getQualifiedName().toString());
    if (fn == null) return null;
    String tc = cx.tconvsOf(target);
    List<? extends ExpressionTree> as = n.getArguments();
    if (as.isEmpty()) return "newT" + fn + "_(" + tc + ")";
    ExpressionTree a = as.get(0);
    TypeMirror at = typeOf(a);
    String lower = "t" + Character.toLowerCase(fn.charAt(0)) + fn.substring(1);
    if (at.getKind() == TypeKind.INT) return "newT" + fn + "N_(" + tc + ", " + exprAs(a, at) + ")";
    if (elemOf(n) instanceof ExecutableElement ctor && ctor.getParameters().size() == 1 && cx.erase(ctor.getParameters().get(0).asType()) instanceof DeclaredType pt
        && Ctx.typeElement(pt).getQualifiedName().contentEquals("java.util.Comparator")) {
      // new TreeSet<>(comparator)
      return "newT" + fn + "C_(" + tc + ", " + exprAs(a, pt) + ")";
    }
    if (cx.isTypedColl(at) && cx.goType(at).equals(cx.goType(target))) return lower + "Copy_(" + tc + ", " + expr(a) + ")";
    return lower + "From_(" + tc + ", " + exprAs(a, cx.elems.getTypeElement(cx.typedColl(target).fromType).asType()) + ")";
  }

  /**
   * The new builder of a typed builder chain (rule "typed builder chains", Colls.chains: the
   * expression `ImmutableList.<X>builder()` or `new ImmutableList.Builder<X>()` has the type of a
   * typed builder): the runtime's typed builder; null for any other expression.
   */
  String chainBuilder(ExpressionTree t) {
    TypeMirror rt = cx.refinedTrees.get(t);
    Ctx.CollKind ck = rt == null ? null : cx.typedColl(rt);
    return ck == null || ck.builds == null ? null : newTyped(t, rt);
  }

  /**
   * `ImmutableList.of(a, b)`, `ImmutableSet.of(..)` or `ImmutableMap.of(k, v, ..)` where a typed
   * collection is wanted: the runtime's typed factory (rt_timm.go), which takes the elements as
   * they are; null for any other call (its result enters through Ctx.coerce).
   */
  String immutableOf(MethodInvocationTree mi, TypeMirror target) {
    Ctx.CollKind ck = cx.typedColl(target);
    if (!(elemOf(mi.getMethodSelect()) instanceof ExecutableElement m)) return null;
    List<? extends ExpressionTree> as = mi.getArguments();
    String tc = cx.tconvsOf(target);
    if (ck.factory(m) != null) {
      // ImmutableList.builder(), builderWithExpectedSize(n): a typed builder
      String fn = "newT" + ck.made.values().iterator().next() + ck.factory(m) + "_(" + tc;
      for (ExpressionTree a : as) fn += ", " + exprAs(a, typeOf(a));
      return fn + ")";
    }
    String[] maker = cx.typedMaker(m, ck, new ArrayList<>(((DeclaredType) target).getTypeArguments()));
    if (maker != null && m.getModifiers().contains(Modifier.STATIC)) {
      // a library function that makes a collection for its caller: the runtime's typed constructor
      StringBuilder sb = new StringBuilder(maker[0] + "(" + tc);
      for (int i = 0; i < as.size(); i++) sb.append(", ").append(exprAs(as.get(i), cx.erase(m.getParameters().get(i).asType())));
      return sb.append(")").toString();
    }
    if (maker != null && mi.getMethodSelect() instanceof MemberSelectTree ms && !m.getModifiers().contains(Modifier.STATIC)) {
      // a library method that makes a list for its caller: the runtime's typed twin
      StringBuilder sb = new StringBuilder(maker[0] + "(" + tc + ", " + exprAs(ms.getExpression(), cx.ownerOf(m).asType()));
      for (int i = 0; i < as.size(); i++) sb.append(", ").append(exprAs(as.get(i), cx.erase(m.getParameters().get(i).asType())));
      return sb.append(")").toString();
    }
    if (ck.name.equals("set") && m.getSimpleName().contentEquals("keySet") && as.isEmpty() && mi.getMethodSelect() instanceof MemberSelectTree ms) {
      // keySet() of an immutable typed map (javac finds the method in ImmutableMap): its typed key set
      TypeMirror rt = typeOf(ms.getExpression());
      Ctx.CollKind rk = rt == null ? null : cx.typedColl(cx.erase(rt));
      if (rk != null && rk.immType(cx.ownerOf(m)) && cx.goType(((DeclaredType) cx.erase(rt)).getTypeArguments().get(0)).equals(cx.goType(((DeclaredType) target).getTypeArguments().get(0)))) {
        return atomOrParen(expr(ms.getExpression())) + ".keySetImm()";
      }
    }
    if (ck.immClass == null || !m.getModifiers().contains(Modifier.STATIC)) return null;
    // a call that passes an array for the variable arguments goes through the erased factory
    if (m.isVarArgs() && as.size() == m.getParameters().size() && cx.erase(typeOf(as.get(as.size() - 1))).getKind() != TypeKind.DECLARED) return null;
    if (!((TypeElement) m.getEnclosingElement()).getQualifiedName().contentEquals(ck.immClass)) return null;
    String cap = Character.toUpperCase(ck.name.charAt(0)) + ck.name.substring(1);
    List<? extends TypeMirror> tas = ((DeclaredType) target).getTypeArguments();
    if (m.getSimpleName().contentEquals("copyOf") && as.size() == 1) {
      // copyOf: of a typed collection of the same type (an immutable one is itself), of an
      // array of the elements, or of a collection as erased code sees it
      TypeMirror at = typeOf(as.get(0)), pt = cx.erase(m.getParameters().get(0).asType());
      if (cx.isTypedColl(at) && cx.goType(at).equals(cx.goType(target))) return "immutable" + cap + "CopyOfT_(" + tc + ", " + expr(as.get(0)) + ")";
      if (pt.getKind() == TypeKind.ARRAY) {
        TypeMirror et = cx.erase(at);
        if (!ck.name.equals("list") || et.getKind() != TypeKind.ARRAY || !cx.goType(((ArrayType) et).getComponentType()).equals(cx.goType(tas.get(0)))) return null;
        return "immutableListCopyOfArr_(" + tc + ", " + exprAs(as.get(0), et) + ")";
      }
      String pn = pt.getKind() == TypeKind.DECLARED ? Ctx.typeElement(pt).getQualifiedName().toString() : "";
      if (!pn.equals("java.util.Collection") && !pn.equals("java.lang.Iterable") && !pn.equals("java.util.Map")) return null;
      return "immutable" + cap + "CopyOf_(" + tc + ", " + exprAs(as.get(0), pt) + ")";
    }
    if (!m.getSimpleName().contentEquals("of")) return null;
    if (as.isEmpty()) return "t" + ck.name + "EmptyImm_(" + tc + ")";
    boolean map = ck.name.equals("map");
    if (map && as.size() > 10) return null;
    StringBuilder sb = new StringBuilder("immutable" + cap + "Of" + (map ? as.size() / 2 : "") + "_[" + cx.goType(tas.get(0)) + (map ? ", " + cx.goType(tas.get(1)) : "") + "](" + tc);
    for (int i = 0; i < as.size(); i++) sb.append(", ").append(exprAs(as.get(i), tas.get(map ? i % 2 : 0)));
    return sb.append(")").toString();
  }

  /** The value that a function with result type ret returns: a result declared `any` needs no conversion any(x) (Conv.implicit). */
  String resultAs(ExpressionTree t, TypeMirror ret) {
    String r = exprAs(t, ret);
    return cx.goType(ret).equals("any") ? Conv.implicit(r) : r;
  }

  String cond(ExpressionTree t) {
    return exprAs(t, prim(TypeKind.BOOLEAN));
  }

  String asAny(ExpressionTree t) {
    return exprAs(t, objectType());
  }

  /** The expression at its own static (erased) Java type. */
  String expr(ExpressionTree t) {
    switch (t.getKind()) {
      case PARENTHESIZED:
        return expr(((ParenthesizedTree) t).getExpression());
      case INT_LITERAL, LONG_LITERAL, FLOAT_LITERAL, DOUBLE_LITERAL, BOOLEAN_LITERAL, CHAR_LITERAL, STRING_LITERAL:
        return literal((LiteralTree) t);
      case NULL_LITERAL:
        return "nil";
      case IDENTIFIER:
        return identifier((IdentifierTree) t);
      case MEMBER_SELECT:
        return memberSelect((MemberSelectTree) t);
      case METHOD_INVOCATION: {
        MethodInvocationTree mi = (MethodInvocationTree) t;
        String chain = chainBuilder(t);
        if (chain != null) return chain;
        CallResult r = invocation(mi);
        return cx.coerce(r.code, r.type, typeOf(t));
      }
      case NEW_CLASS: {
        String chain = chainBuilder(t);
        return chain != null ? chain : newClass((NewClassTree) t);
      }
      case NEW_ARRAY:
        return newArray((NewArrayTree) t, (ArrayType) cx.erase(typeOf(t)));
      case ARRAY_ACCESS: {
        ArrayAccessTree a = (ArrayAccessTree) t;
        return expr(a.getExpression()) + "[" + index(a.getIndex()) + "]";
      }
      case ASSIGNMENT:
        // Prep step 09: an assignment is always a statement of its own.
        throw unsupported(t, "assignment used as a value");
      case CONDITIONAL_EXPRESSION:
        return conditional((ConditionalExpressionTree) t, typeOf(t));
      case INSTANCE_OF:
        return instanceOf((InstanceOfTree) t);
      case TYPE_CAST: {
        TypeCastTree c = (TypeCastTree) t;
        TypeMirror to = typeOf(t);
        String shared = cx.sharedStatics.get(elemOf(strip(c.getExpression())));
        if ("single".equals(shared)) return sharedSingle(strip(c.getExpression()), to);
        if ("array".equals(shared)) return cx.goType(cx.erase(to)) + "{}";
        Object[] slot = strSlotRead(c.getExpression());
        if (slot != null && cx.kind(to) == Ctx.K.STR) return slot[0] + "." + cx.fieldName((VariableElement) slot[1]) + "_str_()";
        Object cv = constOf(t);
        if (cv != null && to.getKind().isPrimitive()) {
          return cv instanceof Character ch ? charLit(ch) : cv.toString();
        }
        return exprAs(c.getExpression(), to);
      }
      case LAMBDA_EXPRESSION:
        return lambda((LambdaExpressionTree) t);
      case MEMBER_REFERENCE:
        return memberRef((MemberReferenceTree) t);
      case SWITCH_EXPRESSION:
        return switchExpr((SwitchExpressionTree) t);
      case POSTFIX_INCREMENT, POSTFIX_DECREMENT, PREFIX_INCREMENT, PREFIX_DECREMENT:
        return incDecExpr((UnaryTree) t);
      case LOGICAL_COMPLEMENT:
        return "!" + atomOrParen(cond(((UnaryTree) t).getExpression()));
      case UNARY_MINUS, UNARY_PLUS, BITWISE_COMPLEMENT: {
        Object cv = constOf(t);
        if (cv != null) return cv.toString();
        UnaryTree u = (UnaryTree) t;
        if (t.getKind() == Tree.Kind.UNARY_MINUS && strip(u.getExpression()) instanceof LiteralTree lit
            && (lit.getValue() instanceof Double dz && dz == 0.0 || lit.getValue() instanceof Float fz && fz == 0.0f)) {
          // Go has no negative-zero constant.
          return lit.getValue() instanceof Float ? "float32(negzero_)" : "negzero_";
        }
        String op = t.getKind() == Tree.Kind.UNARY_MINUS ? "-" : t.getKind() == Tree.Kind.UNARY_PLUS ? "+" : "^";
        return "(" + op + exprAs(u.getExpression(), typeOf(t)) + ")";
      }
      default:
        if (t instanceof BinaryTree b) return binary(b);
        if (t instanceof CompoundAssignmentTree) throw unsupported(t, "compound assignment used as a value");
        throw unsupported(t, "expression " + t.getKind());
    }
  }

  /** Parenthesizes unless the code is a single operand (a name, a selector chain, a call). */
  static String atomOrParen(String s) {
    int depth = 0;
    for (int i = 0; i < s.length(); i++) {
      char c = s.charAt(i);
      if (c == '(' || c == '[' || c == '{') depth++;
      else if (c == ')' || c == ']' || c == '}') depth--;
      else if (depth == 0 && (c == ' ' || c == '"' || c == '\'' || c == '!' || c == '-' || c == '+' || c == '*' || c == '&' || c == '<' || c == '>' || c == '=')) return paren(s);
    }
    return s;
  }

  static String paren(String s) {
    return s.startsWith("(") && s.endsWith(")") && balanced(s) ? s : "(" + s + ")";
  }

  private static boolean balanced(String s) {
    int d = 0;
    for (int i = 0; i < s.length(); i++) {
      char c = s.charAt(i);
      if (c == '(') d++;
      if (c == ')') {
        d--;
        if (d == 0 && i < s.length() - 1) return false;
      }
    }
    return true;
  }

  String index(ExpressionTree t) {
    TypeMirror ty = typeOf(t);
    if (ty != null && !ty.getKind().isPrimitive()) return exprAs(t, prim(TypeKind.INT));
    return expr(t);
  }

  // ------------------------------------------------------------ names

  static final class Recv {
    String code;
    TypeMirror type;

    Recv(String c, TypeMirror t) {
      code = c;
      type = t;
    }
  }

  /** The object that an unqualified member of {@code owner} is accessed on. */
  Recv implicitRecv(TypeElement owner) {
    String code = "this";
    TypeMirror ot = cx.erase(owner.asType());
    for (ClassCtx c : classStack) {
      if (cx.sub(c.te.asType(), ot)) return new Recv(code, c.te.asType());
      code += ".outer_";
    }
    ClassCtx c = cc();
    return new Recv("this", c.te.asType());
  }

  String localRef(Element e) {
    mc.reads.add(e);
    ClassCtx c = cc();
    if (c != null && !c.ownLocals.contains(e) && c.captures.contains(e)) return "this.cap_" + Ctx.safe(e.getSimpleName().toString());
    return localName(e);
  }

  String staticField(VariableElement f) {
    if (!cx.isSource(cx.ownerOf(f)) && f.getConstantValue() != null) {
      // A library constant: its value, typed.
      Object v = f.getConstantValue();
      String gt = cx.goType(f.asType());
      if (v instanceof String str) return quote(str);
      if (v instanceof Character c) return gt + "(" + charLit(c) + ")";
      if (v instanceof Boolean) return v.toString();
      if (v instanceof Double d && (d.isNaN() || d.isInfinite())) return d.isNaN() ? "nan_" : d > 0 ? "posinf_" : "neginf_";
      if (v instanceof Float fl && (fl.isNaN() || fl.isInfinite())) return fl.isNaN() ? "float32(nan_)" : fl > 0 ? "float32(posinf_)" : "float32(neginf_)";
      return gt + "(" + v + ")";
    }
    String n = cx.fieldName(f);
    if (!cx.isSource(cx.ownerOf(f))) cx.extVars.putIfAbsent(n, cx.goType(f.asType()));
    else if (f.getConstantValue() == null) touch(cx.ownerOf(f));
    return n;
  }

  String identifier(IdentifierTree t) {
    String name = t.getName().toString();
    Element e = elemOf(t);
    if (name.equals("this")) return "this";
    if (e == null) throw unsupported(t, "unresolved identifier " + name);
    if (isLocalKind(e)) return cx.coerce(localRef(e), cx.vtype(e), typeOf(t));
    switch (e.getKind()) {
      case FIELD, ENUM_CONSTANT: {
        VariableElement f = (VariableElement) e;
        if (f.getModifiers().contains(Modifier.STATIC)) return cx.coerce(staticField(f), cx.vtype(f), typeOf(t));
        Recv r = implicitRecv(cx.ownerOf(f));
        noteField(f);
        return cx.coerce(cx.fieldGet(r.code, f), memberType(r.type, f), typeOf(t));
      }
      default:
        throw unsupported(t, "identifier of kind " + e.getKind());
    }
  }

  String memberSelect(MemberSelectTree t) {
    String name = t.getIdentifier().toString();
    ExpressionTree q = t.getExpression();
    Element e = elemOf(t);
    if (name.equals("class")) {
      TypeMirror qt = cx.erase(typeOf(q));
      return "classOf_(" + quote(qt == null ? q.toString() : qt.toString()) + ")";
    }
    if (name.equals("this")) {
      TypeElement target = Ctx.typeElement(cx.erase(typeOf(t)));
      String code = "this";
      for (ClassCtx c : classStack) {
        if (c.te.equals(target)) return code;
        code += ".outer_";
      }
      return "this";
    }
    TypeMirror qt = typeOf(q);
    if (name.equals("length") && qt != null && qt.getKind() == TypeKind.ARRAY) {
      return "int32(len(" + expr(q) + "))";
    }
    if (e == null) throw unsupported(t, "unresolved member " + name);
    switch (e.getKind()) {
      case FIELD, ENUM_CONSTANT: {
        VariableElement f = (VariableElement) e;
        if (f.getModifiers().contains(Modifier.STATIC)) return cx.coerce(staticField(f), cx.vtype(f), typeOf(t));
        noteField(f);
        String recv;
        if (q instanceof IdentifierTree id && id.getName().contentEquals("super")) {
          recv = "this";
        } else {
          recv = expr(q);
        }
        return cx.coerce(cx.fieldGet(recv, f), memberType(typeOf(q), f), typeOf(t));
      }
      default:
        throw unsupported(t, "member of kind " + e.getKind());
    }
  }

  /** The type of field f read through a value of type recv: a field of a typed class at the receiver's type arguments. */
  TypeMirror memberType(TypeMirror recv, VariableElement f) {
    if (recv != null && !f.getModifiers().contains(Modifier.STATIC) && cx.typedClasses.contains(cx.ownerOf(f))) {
      return cx.atSite(cx.vtype(f), cx.ownerOf(f), recv);
    }
    return cx.vtype(f);
  }

  /** The instantiated type of the method invocation being translated (javac's, after inference), or null. */
  ExecutableType curInst;
  /** The type a call of a static generic function is to return, when not javac's (GenScan.shared). */
  TypeMirror retOverride;

  /** Binds the type variables in generic type g to the parts of a that stand where they stand. */
  static void unify(TypeMirror g, TypeMirror a, Map<Element, TypeMirror> m) {
    if (g == null || a == null) return;
    if (g.getKind() == TypeKind.TYPEVAR) {
      m.putIfAbsent(((TypeVariable) g).asElement(), a);
    } else if (g.getKind() == TypeKind.DECLARED && a.getKind() == TypeKind.DECLARED) {
      List<? extends TypeMirror> ga = ((DeclaredType) g).getTypeArguments(), aa = ((DeclaredType) a).getTypeArguments();
      if (ga.size() == aa.size()) for (int i = 0; i < ga.size(); i++) unify(ga.get(i), aa.get(i), m);
    } else if (g.getKind() == TypeKind.ARRAY && a.getKind() == TypeKind.ARRAY) {
      unify(((ArrayType) g).getComponentType(), ((ArrayType) a).getComponentType(), m);
    } else if (FnTypes.ON && g.getKind() == TypeKind.WILDCARD) {
      // rule "function types": `? super T` / `? extends U` of a functional parameter binds T and U
      WildcardType gw = (WildcardType) g;
      TypeMirror gb = gw.getExtendsBound() != null ? gw.getExtendsBound() : gw.getSuperBound(), ab = a;
      if (a.getKind() == TypeKind.WILDCARD) {
        WildcardType aw = (WildcardType) a;
        ab = aw.getExtendsBound() != null ? aw.getExtendsBound() : aw.getSuperBound();
      }
      if (gb != null && ab != null && !FnTypes.captured(ab)) unify(gb, ab, m);
    }
  }

  void noteField(VariableElement f) {
    if (cx.isSource(cx.ownerOf(f)) || f.getModifiers().contains(Modifier.STATIC)) return;
    cx.className(cx.ownerOf(f));
    cx.extFields.computeIfAbsent(cx.ownerOf(f), k -> new TreeMap<>()).put(cx.fieldName(f), cx.goType(f.asType()));
  }

  static final class LValue {
    String code;
    TypeMirror type;
    /** The field assigned, when its stored form may differ from its value (Ctx.stored). */
    VariableElement field;
  }

  LValue lvalue(ExpressionTree t0) {
    ExpressionTree t = strip(t0);
    LValue lv = new LValue();
    Element e = elemOf(t);
    if (t instanceof IdentifierTree) {
      if (isLocalKind(e)) {
        lv.code = localName(e);
        ClassCtx c = cc();
        if (c != null && !c.ownLocals.contains(e) && c.captures.contains(e)) lv.code = "this.cap_" + Ctx.safe(e.getSimpleName().toString());
        lv.type = cx.erase(cx.vtype(e));
        return lv;
      }
      VariableElement f = (VariableElement) e;
      lv.type = cx.erase(cx.vtype(f));
      if (f.getModifiers().contains(Modifier.STATIC)) lv.code = staticField(f);
      else {
        Recv r = implicitRecv(cx.ownerOf(f));
        lv.code = cx.fieldLoc(r.code, f);
        lv.type = cx.erase(memberType(r.type, f));
      }
      lv.field = f;
      noteField(f);
      return lv;
    }
    if (t instanceof MemberSelectTree ms) {
      VariableElement f = (VariableElement) e;
      lv.type = cx.erase(memberType(typeOf(ms.getExpression()), f));
      if (f.getModifiers().contains(Modifier.STATIC)) {
        lv.code = staticField(f);
      } else {
        ExpressionTree q = ms.getExpression();
        String recv = q instanceof IdentifierTree id && id.getName().contentEquals("super") ? "this" : expr(q);
        lv.code = cx.fieldLoc(recv, f);
      }
      lv.field = f;
      return lv;
    }
    if (t instanceof ArrayAccessTree a) {
      lv.code = expr(a.getExpression()) + "[" + index(a.getIndex()) + "]";
      lv.type = cx.erase(((ArrayType) cx.erase(typeOf(a.getExpression()))).getComponentType());
      return lv;
    }
    throw unsupported(t, "assignment target " + t.getKind());
  }

  // ------------------------------------------------------------ operators

  TypeMirror promote(TypeMirror a, TypeMirror b) {
    TypeMirror ua = cx.unboxed(a), ub = cx.unboxed(b);
    if (ua == null || ub == null) return null;
    TypeKind x = ua.getKind(), y = ub.getKind();
    if (x == TypeKind.BOOLEAN || y == TypeKind.BOOLEAN) return prim(TypeKind.BOOLEAN);
    if (x == TypeKind.DOUBLE || y == TypeKind.DOUBLE) return prim(TypeKind.DOUBLE);
    if (x == TypeKind.FLOAT || y == TypeKind.FLOAT) return prim(TypeKind.FLOAT);
    if (x == TypeKind.LONG || y == TypeKind.LONG) return prim(TypeKind.LONG);
    return prim(TypeKind.INT);
  }

  TypeMirror promote1(TypeMirror a) {
    TypeMirror ua = cx.unboxed(a);
    if (ua == null) return null;
    switch (ua.getKind()) {
      case BYTE, SHORT, CHAR, INT: return prim(TypeKind.INT);
      default: return ua;
    }
  }

  static String goOp(Tree.Kind k) {
    switch (k) {
      case PLUS, PLUS_ASSIGNMENT: return "+";
      case MINUS, MINUS_ASSIGNMENT: return "-";
      case MULTIPLY, MULTIPLY_ASSIGNMENT: return "*";
      case DIVIDE, DIVIDE_ASSIGNMENT: return "/";
      case REMAINDER, REMAINDER_ASSIGNMENT: return "%";
      case AND, AND_ASSIGNMENT: return "&";
      case OR, OR_ASSIGNMENT: return "|";
      case XOR, XOR_ASSIGNMENT: return "^";
      case LEFT_SHIFT, LEFT_SHIFT_ASSIGNMENT: return "<<";
      case RIGHT_SHIFT, RIGHT_SHIFT_ASSIGNMENT: return ">>";
      case UNSIGNED_RIGHT_SHIFT, UNSIGNED_RIGHT_SHIFT_ASSIGNMENT: return ">>>";
      case LESS_THAN: return "<";
      case GREATER_THAN: return ">";
      case LESS_THAN_EQUAL: return "<=";
      case GREATER_THAN_EQUAL: return ">=";
      case EQUAL_TO: return "==";
      case NOT_EQUAL_TO: return "!=";
      case CONDITIONAL_AND: return "&&";
      case CONDITIONAL_OR: return "||";
      default: throw new IllegalArgumentException(k.toString());
    }
  }

  String toStr(ExpressionTree t0) {
    ExpressionTree t = strip(t0);
    TypeMirror ty = typeOf(t);
    if (t.getKind() == Tree.Kind.NULL_LITERAL) return "\"null\"";
    if (t.getKind() == Tree.Kind.STRING_LITERAL) return expr(t);
    if (t.getKind() == Tree.Kind.CHAR_LITERAL) return quote(String.valueOf(((LiteralTree) t).getValue()));
    if (t.getKind() == Tree.Kind.INT_LITERAL || t.getKind() == Tree.Kind.LONG_LITERAL) return quote(String.valueOf(((LiteralTree) t).getValue()));
    switch (cx.kind(ty)) {
      case STR: return "sstr_(" + expr(t) + ")";
      case PRIM:
        switch (ty.getKind()) {
          case CHAR: return "chrstr_(" + expr(t) + ")";
          case BOOLEAN: return "boolstr_(" + expr(t) + ")";
          case FLOAT: return "fstr_(" + expr(t) + ")";
          case DOUBLE: return "dstr_(" + expr(t) + ")";
          default: return "istr_(int64(" + expr(t) + "))";
        }
      default: {
        // an object at its static type (Conv.typedStr), else through Object
        String typed = cx.conv.typedStr("", ty) == null || !declaredAs(t, ty) ? null : cx.conv.typedStr(expr(t), ty);
        return typed != null ? typed : "objstr_(" + Conv.implicit(asAny(t)) + ")";
      }
    }
  }

  /**
   * Whether the value of t has type ty by its declaration: a variable, a field or the result of
   * a method declared with that (erased) type, a new object or `this`. Such an expression at
   * its own type is the code that the conversion to Object boxes; any other one (a result or a
   * field of a type variable's type, a conditional) is cast first when it is used at its type.
   */
  boolean declaredAs(ExpressionTree t, TypeMirror ty) {
    switch (t.getKind()) {
      case NEW_CLASS:
        return true;
      case IDENTIFIER, MEMBER_SELECT, METHOD_INVOCATION: {
        Element e = elemOf(t.getKind() == Tree.Kind.METHOD_INVOCATION ? ((MethodInvocationTree) t).getMethodSelect() : t);
        if (e instanceof ExecutableElement m) return cx.types.isSameType(cx.types.erasure(m.getReturnType()), cx.types.erasure(ty));
        if (e instanceof VariableElement v) return cx.types.isSameType(cx.types.erasure(v.asType()), cx.types.erasure(ty));
        return false;
      }
      default:
        return false;
    }
  }

  void concatParts(ExpressionTree t0, List<String> out) {
    ExpressionTree t = strip(t0);
    if (t.getKind() == Tree.Kind.PLUS && cx.kind(typeOf(t)) == Ctx.K.STR) {
      BinaryTree b = (BinaryTree) t;
      if (cx.kind(typeOf(b.getLeftOperand())) == Ctx.K.STR) concatParts(b.getLeftOperand(), out);
      else out.add(toStr(b.getLeftOperand()));
      out.add(toStr(b.getRightOperand()));
      return;
    }
    out.add(toStr(t));
  }

  String shiftCount(ExpressionTree t, boolean isLong) {
    Object c = constOf(t);
    int mask = isLong ? 63 : 31;
    if (c != null) return Long.toString(asLong(c) & mask);
    return "(uint32(" + exprAs(t, prim(TypeKind.INT)) + ") & " + mask + ")";
  }

  String binary(BinaryTree t) {
    Tree.Kind k = t.getKind();
    ExpressionTree l = t.getLeftOperand(), r = t.getRightOperand();
    TypeMirror tl = typeOf(l), tr = typeOf(r), ty = typeOf(t);
    if (k == Tree.Kind.CONDITIONAL_AND || k == Tree.Kind.CONDITIONAL_OR) {
      return "(" + cond(l) + " " + goOp(k) + " " + cond(r) + ")";
    }
    if (k == Tree.Kind.PLUS && cx.kind(ty) == Ctx.K.STR) {
      List<String> parts = new ArrayList<>();
      concatParts(t, parts);
      return "(" + String.join(" + ", parts) + ")";
    }
    if (k == Tree.Kind.EQUAL_TO || k == Tree.Kind.NOT_EQUAL_TO) return equality(t, t.getLeftOperand(), t.getRightOperand(), k == Tree.Kind.EQUAL_TO ? "==" : "!=");
    Object cv = constOf(t);
    if (cv != null) return cv instanceof Character c ? Integer.toString(c) : cv.toString();
    if (k == Tree.Kind.LEFT_SHIFT || k == Tree.Kind.RIGHT_SHIFT || k == Tree.Kind.UNSIGNED_RIGHT_SHIFT) {
      TypeMirror p = promote1(tl);
      boolean isLong = p.getKind() == TypeKind.LONG;
      String a = exprAs(l, p);
      if (isNumericLiteral(l)) a = cx.goType(p) + "(" + a + ")";
      String n = shiftCount(r, isLong);
      if (k == Tree.Kind.UNSIGNED_RIGHT_SHIFT) {
        return isLong ? "int64(uint64(" + a + ") >> " + n + ")" : "int32(uint32(" + a + ") >> " + n + ")";
      }
      return "(" + a + " " + goOp(k) + " " + n + ")";
    }
    TypeMirror p = promote(tl, tr);
    if (p == null) throw unsupported(t, "operands of " + k);
    boolean cmp = k == Tree.Kind.LESS_THAN || k == Tree.Kind.GREATER_THAN || k == Tree.Kind.LESS_THAN_EQUAL || k == Tree.Kind.GREATER_THAN_EQUAL;
    if (p.getKind() == TypeKind.BOOLEAN) {
      String a = cond(l), b = cond(r);
      switch (k) {
        case AND: return "band_(" + a + ", " + b + ")";
        case OR: return "bor_(" + a + ", " + b + ")";
        case XOR: return "(" + a + " != " + b + ")";
        default: throw unsupported(t, "boolean operator " + k);
      }
    }
    String a = operand(l, p, r), b = operand(r, p, l);
    if (k == Tree.Kind.REMAINDER && (p.getKind() == TypeKind.DOUBLE || p.getKind() == TypeKind.FLOAT)) {
      return cx.goType(p) + "(fmod_(float64(" + a + "), float64(" + b + ")))";
    }
    return "(" + a + " " + goOp(k) + " " + b + ")";
  }

  /** An operand of a binary numeric operation promoted to p; literals stay untyped unless both are. */
  String operand(ExpressionTree t, TypeMirror p, ExpressionTree other) {
    if (isNumericLiteral(t) && !isNumericLiteral(other)) {
      TypeMirror lt = cx.unboxed(typeOf(t));
      boolean fl = lt.getKind() == TypeKind.FLOAT || lt.getKind() == TypeKind.DOUBLE;
      boolean fp = p.getKind() == TypeKind.FLOAT || p.getKind() == TypeKind.DOUBLE;
      if (!fl || fp) return exprAs(t, p);
    }
    if (isNumericLiteral(t)) return cx.goType(p) + "(" + expr(t) + ")";
    return exprAs(t, p);
  }

  /** The object of a shared static (rule "shared statics") for instantiation ty: C_F[K, V](). */
  String sharedSingle(ExpressionTree x, TypeMirror ty) {
    VariableElement f = (VariableElement) elemOf(x);
    ty = cx.erase(ty);
    if (!cx.typedDecl(ty)) return staticFieldFn(f) + "()";
    return staticFieldFn(f) + cx.typeArgsOf(ty) + "()";
  }

  String staticFieldFn(VariableElement f) {
    touch(cx.ownerOf(f));
    return cx.fieldName(f);
  }

  /**
   * Rule "identity functions" (Conv.IDENT): a static method of the program that is Java's `==`
   * on two Objects, `static boolean f(Object a, Object b) { return a == b; }`.
   */
  boolean identityFn(ExecutableElement e) {
    return identityFns.computeIfAbsent(e, k -> {
      if (!e.getModifiers().contains(Modifier.STATIC) || !e.getTypeParameters().isEmpty() || e.isVarArgs() || e.getParameters().size() != 2
          || e.getReturnType().getKind() != TypeKind.BOOLEAN || !cx.isSource(cx.ownerOf(e))) return false;
      for (VariableElement pe : e.getParameters()) if (!cx.types.isSameType(pe.asType(), objectType())) return false;
      if (!(cx.trees.getTree(e) instanceof MethodTree mt) || mt.getBody() == null || mt.getBody().getStatements().size() != 1
          || !(mt.getBody().getStatements().get(0) instanceof ReturnTree rt) || rt.getExpression() == null) return false;
      if (!(strip(rt.getExpression()) instanceof BinaryTree b) || b.getKind() != Tree.Kind.EQUAL_TO) return false;
      if (!(strip(b.getLeftOperand()) instanceof IdentifierTree x) || !(strip(b.getRightOperand()) instanceof IdentifierTree y)) return false;
      List<String> names = List.of(x.getName().toString(), y.getName().toString());
      String p0 = e.getParameters().get(0).getSimpleName().toString(), p1 = e.getParameters().get(1).getSimpleName().toString();
      // the two parameters (a method has no other name in scope that hides one of them without a declaration in its body)
      return !p0.equals(p1) && names.contains(p0) && names.contains(p1);
    });
  }

  private final Map<ExecutableElement, Boolean> identityFns = new HashMap<>();

  /**
   * A call f(l, r) of an identity function as `l == r` is translated (equality): the operands
   * stay at their static types where the comparison of two values of those types is the
   * comparison of their boxes, and are boxed as the call boxed them where it is not (or where
   * an operand does not have its static type by declaration). Null for
   * operands that `==` would compare otherwise than the function does: a primitive (the
   * function compares its box) and a value type (which has no identity; equality refuses it).
   */
  String identityCall(Tree at, ExpressionTree l, ExpressionTree r) {
    TypeMirror tl = typeOf(l), tr = typeOf(r);
    if (tl == null || tr == null) return null;
    for (TypeMirror t : List.of(tl, tr)) {
      Ctx.K k = cx.kind(t);
      if (k == Ctx.K.PRIM || k != Ctx.K.NULL && cx.isValueLibType(cx.erase(t))) return null;
    }
    if (cx.sharedStatics.containsKey(elemOf(strip(l))) || cx.sharedStatics.containsKey(elemOf(strip(r)))) return null;
    // An operand that does not have its type by declaration (the result of an erased generic)
    // would be cast to it by `==`; the function takes Objects and never casts: the boxes are compared.
    for (ExpressionTree x : List.of(l, r)) {
      TypeMirror t = typeOf(x);
      if (cx.kind(t) != Ctx.K.NULL && !declaredAs(strip(x), t)) return "(" + asAny(l) + " == " + asAny(r) + ")";
    }
    return equality(at, l, r, "==");
  }

  String equality(Tree t, ExpressionTree l, ExpressionTree r, String op) {
    TypeMirror tl = typeOf(l), tr = typeOf(r);
    if ("single".equals(cx.sharedStatics.get(elemOf(strip(l))))) return "(" + sharedSingle(strip(l), tr) + " " + op + " " + expr(r) + ")";
    if ("single".equals(cx.sharedStatics.get(elemOf(strip(r))))) return "(" + expr(l) + " " + op + " " + sharedSingle(strip(r), tl) + ")";
    Ctx.K kl = cx.kind(tl), kr = cx.kind(tr);
    if (kl == Ctx.K.NULL || kr == Ctx.K.NULL) {
      ExpressionTree x = kl == Ctx.K.NULL ? r : l;
      Ctx.K kx = kl == Ctx.K.NULL ? kr : kl;
      if (kx == Ctx.K.NULL) return op.equals("==") ? "true" : "false";
      Object[] rf = fieldRead(x, f -> cx.runtimeField(f) != null);
      if (rf != null) return (op.equals("==") ? "" : "!") + rf[0] + "." + cx.fieldName((VariableElement) rf[1]) + "_isnull_()";
      Object[] idf = fieldRead(x, f -> (cx.strId(f) && !cx.valCode(f)) || cx.valCode(f) || cx.handleTable(f) != null);
      // Null is code 0 of a string id, value code or table handle.
      if (idf != null) return "(" + cx.fieldLoc((String) idf[0], (VariableElement) idf[1]) + " " + op + " 0)";
      if (kx == Ctx.K.TVAR) return (op.equals("==") ? "" : "!") + "isnull_(" + expr(x) + ")";
      if (cx.isValueLibType(cx.erase(kl == Ctx.K.NULL ? tr : tl))) return (op.equals("==") ? "" : "!") + atomOrParen(expr(x)) + ".null_()";
      return "(" + expr(x) + " " + op + " " + cx.nil(kl == Ctx.K.NULL ? tr : tl) + ")";
    }
    if (kl == Ctx.K.TVAR || kr == Ctx.K.TVAR) {
      // a type parameter is not comparable in Go: the boxes are, as in the erased code
      if (kl == kr && cx.goType(tl).equals(cx.goType(tr))) return (op.equals("==") ? "" : "!") + "eqT_(" + expr(l) + ", " + expr(r) + ")";
      return "(" + asAny(l) + " " + op + " " + asAny(r) + ")";
    }
    if (kl == Ctx.K.PRIM || kr == Ctx.K.PRIM) {
      TypeMirror p = promote(tl, tr);
      if (p == null) throw unsupported(t, "comparison operands");
      if (p.getKind() == TypeKind.BOOLEAN) return "(" + cond(l) + " " + op + " " + cond(r) + ")";
      if (kl == Ctx.K.PRIM && kr == Ctx.K.PRIM && cx.goType(tl).equals(cx.goType(tr))) {
        return "(" + expr(l) + " " + op + " " + expr(r) + ")";
      }
      return "(" + operand(l, p, r) + " " + op + " " + operand(r, p, l) + ")";
    }
    if (kl == Ctx.K.ARR && kr == Ctx.K.ARR) {
      return (op.equals("==") ? "" : "!") + "samearr_(" + expr(l) + ", " + expr(r) + ")";
    }
    if (cx.isValueLibType(cx.erase(tl)) || cx.isValueLibType(cx.erase(tr))) throw unsupported(t, "identity of a value library class (" + tl + " " + op + " " + tr + ")");
    if (cx.fn.is(cx.erase(tl)) || cx.fn.is(cx.erase(tr))) throw unsupported(t, "identity of a functional value (" + tl + " " + op + " " + tr + ")");
    // rule "typed library interfaces": the objects that erased code knows are compared
    if (cx.lg.is(cx.erase(tl)) || cx.lg.is(cx.erase(tr))) return "(" + asAny(l) + " " + op + " " + asAny(r) + ")";
    if (kl == kr && cx.goType(tl).equals(cx.goType(tr))) {
      return "(" + expr(l) + " " + op + " " + expr(r) + ")";
    }
    if (kl == Ctx.K.CLS && kr == Ctx.K.CLS) {
      TypeElement a = Ctx.typeElement(cx.erase(tl)), b = Ctx.typeElement(cx.erase(tr));
      if (cx.isSubclass(a, b)) return "(" + exprAs(l, tr) + " " + op + " " + expr(r) + ")";
      if (cx.isSubclass(b, a)) return "(" + expr(l) + " " + op + " " + exprAs(r, tl) + ")";
    }
    return "(" + asAny(l) + " " + op + " " + asAny(r) + ")";
  }

  String conditional(ConditionalExpressionTree t, TypeMirror target) {
    String gt = cx.goType(target);
    if (gt.isEmpty()) gt = "any";
    return "func() " + gt + " { if " + cond(t.getCondition()) + " { return " + exprAs(t.getTrueExpression(), target)
        + " }; return " + exprAs(t.getFalseExpression(), target) + " }()";
  }

  String isCheck(String anyCode, TypeMirror target0) {
    TypeMirror target = cx.erase(target0);
    switch (cx.kind(target)) {
      case STR: return "isstr_(" + anyCode + ")";
      case CLS: {
        TypeElement te = Ctx.typeElement(target);
        if (cx.isValueLib(te)) return "is" + cx.className(te) + "_(" + anyCode + ")";
        // a generic Go type: any instantiation, through the marker method isa_C_ (Emitter.structDecl)
        if (cx.isSource(te) && !cx.goTparams(te).isEmpty()) return "is_" + cx.className(te) + "(" + anyCode + ")";
        if (cx.isLeaf(te)) return "is_[" + cx.goType(target) + "](" + anyCode + ")";
        return "is_" + cx.className(te) + "(" + anyCode + ")";
      }
      case IFACE: case ARR: return "is_[" + cx.goType(target) + "](" + anyCode + ")";
      case ANY: {
        String qn = Ctx.typeElement(target).getQualifiedName().toString();
        TypeMirror u = cx.unboxed(target);
        if (u != null) return "is_[" + cx.goType(u) + "](" + anyCode + ")";
        if (qn.equals("java.lang.Object")) return "(" + anyCode + " != nil)";
        return "is" + Ctx.typeElement(target).getSimpleName() + "_(" + anyCode + ")";
      }
      default: throw new UnsupportedOperationException("instanceof " + target);
    }
  }

  /**
   * If t reads a string-slot field (Ctx.strSlots): the receiver's Go code and the field; else null.
   */
  Object[] strSlotRead(ExpressionTree t0) {
    return fieldRead(t0, cx::strSlot);
  }

  /** If t reads an instance field f for which p holds: the receiver's Go code and f; else null. */
  Object[] fieldRead(ExpressionTree t0, java.util.function.Predicate<VariableElement> p) {
    ExpressionTree t = strip(t0);
    Element e = (t instanceof IdentifierTree || t instanceof MemberSelectTree) ? elemOf(t) : null;
    if (!(e instanceof VariableElement f) || e.getKind() != ElementKind.FIELD || f.getModifiers().contains(Modifier.STATIC) || !p.test(f)) return null;
    if (t instanceof IdentifierTree) return new Object[] {implicitRecv(cx.ownerOf(f)).code, f};
    return new Object[] {expr(((MemberSelectTree) t).getExpression()), f};
  }

  String instanceOf(InstanceOfTree t) {
    Object[] ht = fieldRead(t.getExpression(), f -> cx.handleTable(f) != null);
    if (ht != null) {
      VariableElement f = (VariableElement) ht[1];
      TypeMirror target = t.getPattern() instanceof BindingPatternTree b0 ? cx.erase(elemOf(b0.getVariable()).asType())
          : typeOf(t.getType()) != null ? typeOf(t.getType()) : cx.trees.getTypeMirror(cx.trees.getPath(cu, t.getType()));
      if (cx.types.isSameType(cx.erase(target), cx.handleTable(f))) {
        // The handle's bit 0 says whether the value is an instance of the flag type.
        String test = "(" + cx.fieldLoc((String) ht[0], f) + "&1 != 0)";
        if (t.getPattern() == null) return test;
        Element ve = elemOf(((BindingPatternTree) t.getPattern()).getVariable());
        String name = declareHoisted(ve);
        return "func() bool { if !" + test + " { return false }; " + name + " = " + cx.coerce(ht[0] + "." + cx.fieldName(f) + "_get_()", objectType(), cx.erase(ve.asType()))
            + "; return true }()";
      }
    }
    Object[] slot = strSlotRead(t.getExpression());
    if (slot != null) {
      // The String of a string-slot field is in the string slot, everything else in the other.
      TypeMirror target = t.getPattern() instanceof BindingPatternTree b0 ? cx.erase(elemOf(b0.getVariable()).asType())
          : typeOf(t.getType()) != null ? typeOf(t.getType()) : cx.trees.getTypeMirror(cx.trees.getPath(cu, t.getType()));
      VariableElement f = (VariableElement) slot[1];
      String fn = slot[0] + "." + cx.fieldName(f);
      if (cx.kind(target) == Ctx.K.STR) {
        String loc = cx.strSlotLoc((String) slot[0], f);
        if (cx.valCode(f)) {
          // A value code says whether it is a String without reading the string table.
          if (t.getPattern() == null) return fn + "_iss_()";
          String name = declareHoisted(elemOf(((BindingPatternTree) t.getPattern()).getVariable()));
          return "func() bool { if !" + fn + "_iss_() { return false }; " + name + " = " + loc + "; return true }()";
        }
        if (t.getPattern() == null) return "(" + loc + " != nullstr_)";
        String name = declareHoisted(elemOf(((BindingPatternTree) t.getPattern()).getVariable()));
        return "func() bool { v_ := " + loc + "; if v_ == nullstr_ { return false }; " + name + " = v_; return true }()";
      }
      if (cx.valCode(f) && cx.unboxed(target) != null && cx.unboxed(target).getKind() == TypeKind.DOUBLE) {
        // A Double in a value code is a float64 inline or in the table of doubles: no box is made.
        if (t.getPattern() == null) return fn + "_isd_()";
        Element ve = elemOf(((BindingPatternTree) t.getPattern()).getVariable());
        String name = declareHoisted(ve);
        return "func() bool { if !" + fn + "_isd_() { return false }; " + name + " = " + cx.coerce(fn + "_d_()", cx.unboxed(target), cx.erase(ve.asType()))
            + "; return true }()";
      }
      String raw = cx.valCode(f) ? fn + "_obj_()" : cx.fieldLoc((String) slot[0], f);
      if (t.getPattern() == null) return isCheck(raw, target);
      String name = declareHoisted(elemOf(((BindingPatternTree) t.getPattern()).getVariable()));
      return "func() bool { v_ := " + raw + "; if !" + isCheck("v_", target) + " { return false }; " + name + " = "
          + cx.coerce("v_", objectType(), target) + "; return true }()";
    }
    String v = asAny(t.getExpression());
    if (t.getPattern() == null) return isCheck(v, typeOf(t.getType()) != null ? typeOf(t.getType()) : cx.trees.getTypeMirror(cx.trees.getPath(cu, t.getType())));
    if (!(t.getPattern() instanceof BindingPatternTree bp)) throw unsupported(t, "pattern " + t.getPattern().getKind());
    VariableTree var = bp.getVariable();
    Element ve = elemOf(var);
    TypeMirror vt = cx.erase(cx.vtype(ve));
    String name = declareHoisted(ve);
    return "func() bool { v_ := " + v + "; if !" + isCheck("v_", vt) + " { return false }; " + name + " = "
        + cx.coerce("v_", objectType(), vt) + "; return true }()";
  }

  /** Declares a pattern binding before the current statement and returns its Go name. */
  String declareHoisted(Element ve) {
    String name = Ctx.safe(ve.getSimpleName().toString());
    if (!mc.hoisted.add(name) || mc.names.containsValue(name)) {
      name = fresh(name);
      mc.hoisted.add(name);
    }
    mc.names.put(ve, name);
    mc.pre.add("var " + name + " " + cx.goType(cx.vtype(ve)) + "\n_ = " + name);
    return name;
  }

  String incDecExpr(UnaryTree t) {
    // Prep step 09: ++ and -- are always statements of their own.
    throw unsupported(t, "++/-- used as a value");
  }

  /** A string-id or handle-table field is assigned only as a whole (stmtExpr). */
  void storedWhole(Tree t, LValue lv) {
    if (lv.field != null && (cx.strId(lv.field) || cx.handleTable(lv.field) != null || cx.runtimeField(lv.field) != null)) throw unsupported(t, "compound assignment to a string-id or handle-table field");
  }

  String incDecStmt(UnaryTree t) {
    LValue lv = lvalue(t.getExpression());
    storedWhole(t, lv);
    boolean inc = t.getKind() == Tree.Kind.POSTFIX_INCREMENT || t.getKind() == Tree.Kind.PREFIX_INCREMENT;
    Element e = elemOf(strip(t.getExpression()));
    if (e != null) mc.reads.add(e);
    if (lv.type.getKind().isPrimitive()) return lv.code + (inc ? "++" : "--");
    TypeMirror u = cx.unboxed(lv.type);
    return lv.code + " = any(" + cx.coerce(lv.code, lv.type, u) + (inc ? " + 1" : " - 1") + ")";
  }

  String compoundStmt(CompoundAssignmentTree t) {
    LValue lv = lvalue(t.getVariable());
    storedWhole(t, lv);
    Element le = elemOf(strip(t.getVariable()));
    if (le != null) mc.reads.add(le);
    Tree.Kind k = t.getKind();
    ExpressionTree r = t.getExpression();
    TypeMirror tl = lv.type, tr = typeOf(r);
    if (cx.kind(tl) == Ctx.K.STR) return lv.code + " += " + toStr(r);
    String op = goOp(k);
    boolean shift = k == Tree.Kind.LEFT_SHIFT_ASSIGNMENT || k == Tree.Kind.RIGHT_SHIFT_ASSIGNMENT || k == Tree.Kind.UNSIGNED_RIGHT_SHIFT_ASSIGNMENT;
    if (shift) {
      TypeMirror p = promote1(tl);
      boolean isLong = p.getKind() == TypeKind.LONG;
      String a = cx.coerce(lv.code, tl, p), n = shiftCount(r, isLong), res;
      if (k == Tree.Kind.UNSIGNED_RIGHT_SHIFT_ASSIGNMENT) {
        res = isLong ? "int64(uint64(" + a + ") >> " + n + ")" : "int32(uint32(" + a + ") >> " + n + ")";
      } else {
        res = "(" + a + " " + op + " " + n + ")";
      }
      return lv.code + " = " + cx.coerce(res, p, tl);
    }
    TypeMirror p = promote(tl, tr);
    if (p == null) throw unsupported(t, "compound assignment operands");
    if (p.getKind() == TypeKind.BOOLEAN) {
      String a = cx.coerce(lv.code, tl, p), b = cond(r), res;
      switch (k) {
        case AND_ASSIGNMENT: res = "band_(" + a + ", " + b + ")"; break;
        case OR_ASSIGNMENT: res = "bor_(" + a + ", " + b + ")"; break;
        default: res = "(" + a + " != " + b + ")";
      }
      return lv.code + " = " + cx.coerce(res, p, tl);
    }
    if (tl.getKind().isPrimitive() && cx.types.isSameType(p, tl)) {
      return lv.code + " " + op + "= " + exprAs(r, p);
    }
    String res = "(" + cx.coerce(lv.code, tl, p) + " " + op + " " + operandCode(r, p) + ")";
    return lv.code + " = " + cx.coerce(res, p, tl);
  }

  String operandCode(ExpressionTree t, TypeMirror p) {
    if (isNumericLiteral(t)) return cx.goType(p) + "(" + expr(t) + ")";
    return exprAs(t, p);
  }

  // ------------------------------------------------------------ calls

  static final class CallResult {
    String code;
    TypeMirror type; // Java type the code's Go type corresponds to

    CallResult(String c, TypeMirror t) {
      code = c;
      type = t;
    }
  }

  String argList(ExecutableElement e, List<TypeMirror> params, List<Arg> args) {
    List<String> out = new ArrayList<>();
    int n = params.size();
    boolean pack = false;
    if (e.isVarArgs()) {
      pack = true;
      if (args.size() == n) {
        Arg last = args.get(n - 1);
        TypeMirror lt = cx.erase(last.type);
        if (lt == null || lt.getKind() == TypeKind.NULL || lt.getKind() == TypeKind.ARRAY && cx.types.isAssignable(lt, params.get(n - 1))) {
          pack = false;
        }
      }
    }
    int fixed = pack ? n - 1 : n;
    // rule "implicit conversions": no any(x) where the Go parameter is `any` (Conv.implicit)
    boolean anyParams = cx.isSource(cx.ownerOf(e)) || cx.conv.anyParams(e.getKind() == ElementKind.CONSTRUCTOR ? cx.ctorName(e, "new_") : cx.staticMethodName(e));
    for (int i = 0; i < fixed && i < args.size(); i++) {
      String a = argAs(e, i, args.get(i), params.get(i));
      out.add(anyParams && cx.goType(params.get(i)).equals("any") ? Conv.implicit(a) : a);
    }
    if (pack) {
      TypeMirror arr = params.get(n - 1);
      TypeMirror comp = ((ArrayType) arr).getComponentType();
      StringBuilder sb = new StringBuilder(cx.goType(arr) + "{");
      for (int i = fixed; i < args.size(); i++) {
        if (i > fixed) sb.append(", ");
        String a = argAs(e, i, args.get(i), comp);
        sb.append(cx.goType(comp).equals("any") ? Conv.implicit(a) : a);
      }
      out.add(sb.append("}").toString());
    }
    return String.join(", ", out);
  }

  /** Argument i of a call of e, for a parameter (or an element of the variable arguments) of type target. */
  String argAs(ExecutableElement e, int i, Arg a, TypeMirror target) {
    TypeMirror own = cx.conv.msgArg(e, i, a.type, target);
    if (own != null) return a.as(own);
    String r = cx.conv.bare(a.as(target), target);
    cx.conv.noteBoxArg(e, i, a.type, target, r);
    return r;
  }

  List<TypeMirror> argTypes(List<Arg> args) {
    List<TypeMirror> out = new ArrayList<>();
    for (Arg a : args) out.add(a.type == null ? null : cx.erase(a.type));
    return out;
  }

  List<TypeMirror> ownParams(ExecutableElement e) {
    List<TypeMirror> out = new ArrayList<>();
    boolean lib = !cx.isSource(cx.ownerOf(e)) && (!cx.typedClasses.contains(cx.ownerOf(e)) || cx.lg.isLib(cx.ownerOf(e)));
    // (a library method: its signature as the runtime declares it, LibGen.rt)
    for (VariableElement p : e.getParameters()) out.add(lib ? cx.lg.rt(cx.vtype(p)) : cx.erase(cx.vtype(p)));
    return out;
  }

  void noteMethod(String go, ExecutableElement e, List<TypeMirror> params, TypeMirror ret, String style) {
    cx.noteExternal(go, style + " " + cx.ownerOf(e).getQualifiedName() + "." + e.getSimpleName() + cx.goSig(params, ret));
    if (go.startsWith("func ")) {
      cx.noteFunc(go.substring(5), params, ret);
    } else {
      String name = go.substring(go.indexOf('.') + 1);
      cx.noteMethodOn(cx.ownerOf(e), name, params, ret == null ? cx.types.getNoType(TypeKind.VOID) : ret);
    }
  }

  /** A call of method e. recv is null for static methods and for implicit receivers. */
  CallResult call(ExecutableElement e, Recv recv, boolean superCall, List<Arg> args) {
    TypeElement owner = cx.ownerOf(e);
    boolean src = cx.isSource(owner);
    boolean isStatic = e.getModifiers().contains(Modifier.STATIC);
    TypeMirror retOver = retOverride;
    retOverride = null;
    if (isStatic) {
      touch(owner);
      List<TypeMirror> params = ownParams(e);
      TypeMirror ret = src ? cx.erase(cx.rtype(e)) : cx.lg.rt(cx.rtype(e));
      String name = cx.staticMethodName(e);
      if (!src) noteMethod("func " + name, e, params, ret, "static");
      if (!src) {
        // a String for each CharSequence parameter: the variant taking strings (Conv.strSeq)
        List<TypeMirror> sp = new ArrayList<>(params);
        String sv = cx.conv.strSeq(name, e, sp, argTypes(args));
        if (sv != null) return new CallResult(sv + "(" + argList(e, sp, args) + ")", ret);
      }
      if (!src) {
        CallResult tr = typedObjStatic(e, args);
        if (tr != null) return tr;
      }
      TypeMirror hm = src || args.isEmpty() ? null : cx.conv.handleMsg(e, name, params.size(), args.size(), args.get(args.size() - 1).type);
      if (hm != null) {
        // a handle as the message argument of a check (Conv.handleMsg)
        params = new ArrayList<>(params);
        params.set(params.size() - 1, hm);
        name += "_H";
      }
      TypeMirror nn = src || args.isEmpty() ? null : cx.conv.nonNull(e, args.get(0).type);
      if (nn != null) {
        // a typed non-null check (Conv.nonNull): a generic function of the argument's type
        List<TypeMirror> p2 = new ArrayList<>(params);
        p2.set(0, nn);
        return new CallResult(name + cx.conv.nonNullSuffix(nn) + "(" + argList(e, p2, args) + ")", nn);
      }
      if (!src && !args.isEmpty() && cx.fn.is(cx.erase(args.get(0).type)) && cx.conv.isNonNullCheck(e)) {
        // rule "function types": the check of a function tests its box (no allocation) and gives the function
        TypeMirror at = cx.erase(args.get(0).type);
        List<Arg> rest = new ArrayList<>(args);
        rest.set(0, new Arg("f_.box_()", objectType()));
        cx.fn.note("null checks of a function", null);
        String gt = cx.goType(at);
        return new CallResult("func() " + gt + " { f_ := " + args.get(0).as(at) + "; " + name + "(" + argList(e, params, rest) + "); return f_ }()", at);
      }
      if ((src || cx.isValueLib(owner) || cx.fn.genericStatics(owner)) && !cx.keptOf(e.getTypeParameters()).isEmpty()) {
        // A generic function: instantiated with javac's inferred type arguments (kept ones).
        ExecutableType inst = curInst;
        curInst = null;
        if (inst == null || inst.getParameterTypes().size() != params.size()) throw new IllegalStateException("no instantiated type for a call of " + e);
        Map<Element, TypeMirror> m = new HashMap<>();
        unify(e.getReturnType(), inst.getReturnType(), m);
        for (int i = 0; i < params.size(); i++) unify(e.getParameters().get(i).asType(), inst.getParameterTypes().get(i), m);
        if (retOver != null) {
          // the call's type is given (an argument of a shared static's constructor)
          m.clear();
          unify(e.getReturnType(), retOver, m);
        }
        if (!src && cx.fn.genericStatics(owner)) {
          // a type parameter that javac left to the enclosing inference: its erasure, as the erased code has
          for (TypeParameterElement tp : e.getTypeParameters()) {
            TypeMirror b = m.get(tp);
            if (b != null && b.getKind() == TypeKind.TYPEVAR && e.getTypeParameters().contains(((TypeVariable) b).asElement())) m.put(tp, cx.types.erasure(tp.asType()));
          }
        }
        Map<Element, TypeMirror> mk = new HashMap<>();
        StringBuilder ta = new StringBuilder("[");
        for (TypeParameterElement tp : cx.keptOf(e.getTypeParameters())) {
          ta.append(ta.length() > 1 ? ", " : "").append(cx.goTypeArg(m.get(tp)));
          if (m.get(tp) != null) mk.put(tp, m.get(tp));
        }
        params = new ArrayList<>();
        for (VariableElement pe : e.getParameters()) params.add(cx.erase(cx.subst(pe.asType(), mk, 0)));
        ret = cx.erase(cx.subst(e.getReturnType(), mk, 0));
        return new CallResult(name + ta + "]" + "(" + argList(e, params, args) + ")", ret);
      }
      return new CallResult(name + "(" + argList(e, params, args) + ")", ret);
    }
    if (recv == null) recv = implicitRecv(owner);
    if (cx.isTypedColl(recv.type) && !superCall) {
      // A typed collection: its typed method, or the method of List or Set on its erased view.
      List<TypeMirror> ats = new ArrayList<>();
      for (Arg a : args) ats.add(a.type);
      ExecutableElement tm = cx.typedMethod(recv.type, e, ats);
      if (tm != null) {
        e = tm;
        owner = cx.ownerOf(tm);
        src = false;
      } else {
        Ctx.CollKind rk = cx.typedColl(recv.type);
        TypeMirror vt = cx.lg.mapValues(rk, recv.type, e, args.size());
        if (vt != null) {
          // values() of a typed map where Collection is a typed interface: the map's typed view (piece `views`)
          return new CallResult(atomOrParen(recv.code) + ".valuesT()", vt);
        }
        if (owner.getQualifiedName().contentEquals(String.valueOf(rk.sortedClass))) {
          // a method that javac finds in TreeSet / TreeMap: the method of the interface it implements
          e = cx.root(e);
          owner = cx.ownerOf(e);
        }
        // a method of Guava's immutable class (reverse(), asList()) is called on the twin, one
        // of SortedSet / NavigableSet (tailSet) on the sorted view
        TypeMirror view = rk.immType(owner) || rk.sortedType(owner) ? cx.types.erasure(owner.asType()) : cx.typedView(recv.type);
        recv = new Recv(cx.coerce(recv.code, recv.type, view), view);
      }
    }
    if (superCall && owner.getQualifiedName().contentEquals("java.lang.Object")) {
      // super.hashCode() / equals / toString reaching Object: the identity-based defaults.
      String self = cx.coerce(recv.code, recv.type, objectType());
      switch (e.getSimpleName().toString()) {
        case "hashCode": return new CallResult("identityHash_(" + self + ")", prim(TypeKind.INT));
        case "equals": return new CallResult("(" + self + " == " + args.get(0).as(objectType()) + ")", prim(TypeKind.BOOLEAN));
        case "toString": return new CallResult("defaultToString_(" + self + ")", stringType());
        default: break;
      }
    }
    if (src && owner.getKind().isInterface() && e.getModifiers().contains(Modifier.PRIVATE)) {
      List<TypeMirror> params = ownParams(e);
      String al = argList(e, params, args);
      return new CallResult(cx.staticMethodName(e) + "(" + cx.coerce(recv.code, recv.type, owner.asType()) + (al.isEmpty() ? "" : ", " + al) + ")", cx.erase(cx.rtype(e)));
    }
    Ctx.K ok = cx.kind(owner.asType());
    ExecutableElement re = cx.root(e);
    TypeElement ro = cx.ownerOf(re);
    if (!src && !superCall) {
      String sk = stringKeyHelper(re, recv, args);
      if (sk != null) {
        // Performance rule: a String key or element reaches the collection unboxed (rt_coll.go,
        // "String keys without a box").
        Ctx.K rk = cx.kind(recv.type);
        StringBuilder sb = new StringBuilder(sk).append("(")
            .append(rk == Ctx.K.CLS || rk == Ctx.K.IFACE ? recv.code : cx.coerce(recv.code, recv.type, ro.asType()))
            .append(", ").append(args.get(0).as(stringType()));
        for (int i = 1; i < args.size(); i++) {
          TypeMirror pt = cx.erase(re.getParameters().get(i).asType());
          String a = args.get(i).as(pt);
          sb.append(", ").append(cx.goType(pt).equals("any") ? Conv.implicit(a) : a);
        }
        return new CallResult(sb.append(")").toString(), cx.lg.rt(re.getReturnType()));
      }
    }
    if (!src && ok != Ctx.K.STR && re != e && !superCall
        && (cx.kind(ro.asType()) == Ctx.K.ANY && ro.getQualifiedName().contentEquals("java.lang.Object")
            || !cx.isSource(ro) && ro.getKind().isInterface() && re.isDefault() && !cx.fn.isLib(ro))) {
      // Declared again by a library type, but really Object's method or an interface default:
      // dispatch through the helper of the root.
      e = re;
      owner = ro;
      ok = cx.kind(owner.asType());
    }
    if (!src && !superCall && recv != null && cx.kind(recv.type) == Ctx.K.TVAR && owner.getQualifiedName().contentEquals("java.lang.Object")) {
      // Object's methods on a value of a type parameter: without boxing a String (rt_generic.go)
      String mn = e.getSimpleName().toString();
      if (mn.equals("hashCode") && args.isEmpty()) return new CallResult("hashCodeT_(" + recv.code + ")", prim(TypeKind.INT));
      if (mn.equals("toString") && args.isEmpty()) return new CallResult("toStringT_(" + recv.code + ")", stringType());
      if (mn.equals("equals") && args.size() == 1) {
        Arg a = args.get(0);
        if (cx.kind(a.type) == Ctx.K.TVAR && cx.goType(a.type).equals(cx.goType(recv.type))) {
          return new CallResult("equalsTT_(" + recv.code + ", " + a.as(a.type) + ")", prim(TypeKind.BOOLEAN));
        }
        return new CallResult("equalsT_(" + recv.code + ", " + a.as(objectType()) + ")", prim(TypeKind.BOOLEAN));
      }
    }
    if (!src && !superCall && owner.getQualifiedName().contentEquals("java.lang.Object")) {
      CallResult tr = typedObjCall(e, recv, args);
      if (tr != null) return tr;
    }
    if (!src && (ok == Ctx.K.STR || ok == Ctx.K.ANY || owner.getKind().isInterface() && e.isDefault() && !cx.fn.isLib(owner))) {
      // Static-style helper taking the receiver first.
      List<TypeMirror> params = ownParams(e);
      TypeMirror ret = cx.lg.rt(e.getReturnType());
      String name = cx.staticMethodName(e);
      String r = cx.coerce(recv.code, recv.type, owner.asType());
      if (ok == Ctx.K.ANY && cx.conv.anyParams(name)) r = Conv.implicit(r);
      List<TypeMirror> sp = new ArrayList<>(params);
      String sv = cx.conv.strSeq(name, e, sp, argTypes(args));
      if (sv != null) return new CallResult(sv + "(" + r + ", " + argList(e, sp, args) + ")", ret);
      String al = argList(e, params, args);
      List<TypeMirror> full = new ArrayList<>();
      full.add(cx.erase(owner.asType()));
      full.addAll(params);
      noteMethod("func " + name, e, full, ret, "receiver-first");
      return new CallResult(name + "(" + r + (al.isEmpty() ? "" : ", " + al) + ")", ret);
    }
    if (src && !superCall && cx.genFns.contains(e)) return genFnCall(e, recv, args);
    // A method of a typed class or interface: its parameters and result at the receiver's view of it.
    TypeMirror site = superCall ? cc().te.asType() : recv != null ? recv.type : null;
    List<TypeMirror> params = cx.groupParams(e, site);
    TypeMirror ret = cx.groupReturn(e, site);
    if (!src && !cx.groupAt(e, site)) {
      // a method of an erased library type: its signature as the runtime declares it
      params = new ArrayList<>(params);
      params.replaceAll(cx.lg::rt);
      ret = cx.lg.rt(ret);
    }
    String name = cx.methodName(e);
    boolean lgTyped = cx.lg.typedAt(e, site);
    if (lgTyped) {
      // rule "typed library interfaces": the typed method (next_t), declared by the runtime
      name = cx.lg.methodName(e, site);
    } else if (recv != null && cx.isTypedColl(recv.type) && cx.lg.is(ret) && name.equals("iterator")) {
      // the iterator of a typed collection, at its element type
      name = "iterator_t";
    } else if (!src) noteMethod("method " + cx.className(cx.ownerOf(cx.root(e))) + "." + name, cx.root(e), params, ret, "method");
    String strVariant = null;
    if (!src && !superCall && recv != null) {
      List<TypeMirror> sp = new ArrayList<>(params);
      strVariant = cx.conv.strSeq(cx.className(cx.ownerOf(cx.root(e))) + "." + name, cx.root(e), sp, argTypes(args));
      if (strVariant != null) params = sp;
    }
    String target;
    if (superCall) {
      TypeElement sup = cx.superclass(cc().te);
      String impl = src ? cx.implName(e, owner) : name;
      if (sup == null) {
        // super.m() on an interface default or on Object.
        target = "this." + impl;
      } else if (!src) {
        // The library superclass's own method, past any dispatching stub of the classes in between.
        TypeElement lib = sup;
        while (lib != null && cx.isSource(lib)) lib = cx.superclass(lib);
        target = "this." + cx.className(lib == null ? sup : lib) + "." + impl;
      } else {
        target = "this." + cx.className(sup) + "." + impl;
      }
    } else {
      Ctx.K rk = cx.kind(recv.type);
      String rc = recv.code;
      boolean boxedTvar = rk == Ctx.K.TVAR && cx.bound((TypeParameterElement) ((TypeVariable) cx.erase(recv.type)).asElement()) == null;
      if (rk == Ctx.K.ANY || rk == Ctx.K.STR || rk == Ctx.K.ARR || rk == Ctx.K.PRIM || boxedTvar) {
        rc = cx.coerce(rc, recv.type, owner.asType());
      } else if (rk == Ctx.K.CLS && owner.getKind().isInterface() && !cx.sub(recv.type, owner.asType())) {
        rc = cx.coerce(rc, recv.type, owner.asType());
      } else if (cx.fn.isLib(owner) && e.isDefault() && !cx.fn.is(cx.erase(recv.type)) && cx.superView(cx.erase(recv.type), owner) != null) {
        // a default method of a functional interface, called on an object: on its function
        rc = cx.coerce(rc, recv.type, cx.superView(cx.erase(recv.type), owner));
      }
      target = rc + "." + name;
      if (strVariant != null) {
        // the variant is a method of the class, or a function taking the receiver first
        if (strVariant.contains("_") && Character.isUpperCase(strVariant.charAt(0))) return new CallResult(strVariant + "(" + rc + ", " + argList(e, params, args) + ")", ret);
        target = rc + "." + strVariant;
      }
    }
    return new CallResult(target + "(" + argList(e, params, args) + ")", ret);
  }

  /** Whether argument a is, by its declaration, a value of the Go type of t (declaredAs): its code at type t is the code that was boxed. */
  boolean argDeclaredAs(Arg a, TypeMirror t) {
    if (a.tree == null || a.type == null) return false;
    TypeMirror at = cx.erase(a.type);
    return cx.kind(at) == cx.kind(t) && !cx.isTypedColl(at) && cx.goType(at).equals(cx.goType(t)) && declaredAs(a.tree, a.type);
  }

  /**
   * Rule "typed Object calls" (Conv.OBJCALL), instance methods: x.equals(y), x.hashCode() and
   * x.toString() where the method javac finds is Object's (the receiver's class, a library class
   * or a class of the program, has no Go method for it), on a receiver that is a pointer to a
   * class without subclasses, an interface or a handle. The receiver stays at its static type:
   * the runtime function (equalsR_, hashR_, tostrR_; suffix H for a handle) makes it the object
   * that the erased translation passed and calls Object's method, so a null receiver throws as
   * before. An argument of equals that has the receiver's type by its declaration stays typed
   * too (equalsRR_); where equals of that type is identity (an enum, a class of the program
   * without subclasses that has only Object's equals) the call is eqR_: a test of the receiver
   * for null and `==`.
   *
   * @return the call, or null when the rule does not apply
   */
  CallResult typedObjCall(ExecutableElement e, Recv recv, List<Arg> args) {
    if (!Conv.OBJCALL || recv == null || recv.type == null) return null;
    String h = cx.conv.objHelper(recv.type);
    if (h == null) return null;
    String n = e.getSimpleName().toString();
    if (n.equals("toString") && args.isEmpty()) return new CallResult("tostrR" + h + "_(" + recv.code + ")", stringType());
    if (n.equals("hashCode") && args.isEmpty()) return new CallResult("hashR" + h + "_(" + recv.code + ")", prim(TypeKind.INT));
    if (!n.equals("equals") || args.size() != 1) return null;
    TypeMirror rt = cx.erase(recv.type);
    Arg a = args.get(0);
    if (argDeclaredAs(a, rt)) {
      String fn = h.isEmpty() && cx.kind(rt) == Ctx.K.CLS && cx.conv.identityEquals(Ctx.typeElement(rt)) ? "eqR_" : "equalsRR" + h + "_";
      return new CallResult(fn + "(" + recv.code + ", " + a.as(rt) + ")", prim(TypeKind.BOOLEAN));
    }
    return new CallResult("equalsR" + h + "_(" + recv.code + ", " + Conv.implicit(argAs(e, 0, a, objectType())) + ")", prim(TypeKind.BOOLEAN));
  }

  /**
   * Rule "typed Object calls", static methods: Objects.equals(a, b) of two values that have one
   * type by their declarations (two Strings are compared with ==, as String.equals is; other
   * types as Conv.typedEquals compares the components of a record), Objects.hashCode(v)
   * (Conv.typedHash) and String.valueOf(Object) (Conv.typedStr) of a value that has its type by
   * its declaration.
   *
   * @return the call, or null when the rule does not apply
   */
  CallResult typedObjStatic(ExecutableElement e, List<Arg> args) {
    if (!Conv.OBJCALL || e.isVarArgs() || args.size() != e.getParameters().size()) return null;
    for (VariableElement p : e.getParameters()) if (!cx.types.isSameType(p.asType(), objectType())) return null;
    String qn = cx.ownerOf(e).getQualifiedName() + "#" + e.getSimpleName();
    switch (qn) {
      case "java.util.Objects#equals", "com.google.common.base.Objects#equal": {
        if (args.size() != 2 || args.get(0).type == null) return null;
        TypeMirror t = cx.erase(args.get(0).type);
        if (!argDeclaredAs(args.get(0), t) || !argDeclaredAs(args.get(1), t)) return null;
        if (cx.kind(t) != Ctx.K.STR && cx.conv.typedEquals("", "", t) == null) return null;
        String a = args.get(0).as(t), b = args.get(1).as(t);
        return new CallResult(cx.kind(t) == Ctx.K.STR ? "(" + a + " == " + b + ")" : "(" + cx.conv.typedEquals(a, b, t) + ")", prim(TypeKind.BOOLEAN));
      }
      case "java.util.Objects#hashCode", "java.lang.String#valueOf": {
        if (args.size() != 1 || args.get(0).type == null) return null;
        boolean hash = qn.endsWith("hashCode");
        TypeMirror t = cx.erase(args.get(0).type);
        Ctx.K k = cx.kind(t);
        if (!argDeclaredAs(args.get(0), t) || (hash ? k != Ctx.K.STR && k != Ctx.K.CLS && k != Ctx.K.IFACE || cx.conv.typedHash("", t) == null : cx.conv.typedStr("", t) == null)) return null;
        String v = args.get(0).as(t);
        return hash ? new CallResult(cx.conv.typedHash(v, t), prim(TypeKind.INT)) : new CallResult(cx.conv.typedStr(v, t), stringType());
      }
      default:
        return null;
    }
  }

  /** A call of generic instance method e (Ctx.genFns): Owner_m[class args, method args](recv, args). */
  CallResult genFnCall(ExecutableElement e, Recv recv, List<Arg> args) {
    TypeElement owner = cx.ownerOf(e);
    if (recv == null) recv = implicitRecv(owner);
    ExecutableType inst = curInst;
    curInst = null;
    if (inst == null || inst.getParameterTypes().size() != e.getParameters().size()) throw new IllegalStateException("no instantiated type for a call of " + e);
    Map<Element, TypeMirror> m = new HashMap<>();
    unify(e.getReturnType(), inst.getReturnType(), m);
    for (int i = 0; i < e.getParameters().size(); i++) unify(e.getParameters().get(i).asType(), inst.getParameterTypes().get(i), m);
    Map<Element, TypeMirror> mk = new HashMap<>();
    StringBuilder ta = new StringBuilder();
    String ca = cx.typedClasses.contains(owner) ? cx.viewArgs(recv.type, owner) : cx.goTparamsUse(owner);
    if (!ca.isEmpty()) ta.append(ca, 1, ca.length() - 1);
    for (TypeParameterElement tp : cx.keptOf(e.getTypeParameters())) {
      ta.append(ta.length() > 0 ? ", " : "").append(cx.goTypeArg(m.get(tp)));
      if (m.get(tp) != null) mk.put(tp, m.get(tp));
    }
    List<TypeMirror> params = new ArrayList<>();
    for (VariableElement pe : e.getParameters()) params.add(cx.erase(cx.subst(cx.atSite(pe.asType(), owner, recv.type), mk, 0)));
    TypeMirror ret = cx.erase(cx.subst(cx.atSite(e.getReturnType(), owner, recv.type), mk, 0));
    DeclaredType view = cx.superView(recv.type, owner);
    String rc = cx.coerce(recv.code, recv.type, view != null && cx.typedDecl(view) ? view : owner.asType());
    String al = argList(e, params, args);
    return new CallResult(cx.genFnName(e) + "[" + ta + "](" + rc + (al.isEmpty() ? "" : ", " + al) + ")", ret);
  }

  /**
   * The runtime function to call instead of root method {@code re} of java.util.Map or
   * java.util.Collection when its key/element argument is statically a String, or null.
   */
  String stringKeyHelper(ExecutableElement re, Recv recv, List<Arg> args) {
    if (args.isEmpty() || args.size() != re.getParameters().size() || cx.kind(args.get(0).type) != Ctx.K.STR) return null;
    TypeMirror p0 = cx.erase(re.getParameters().get(0).asType());
    if (cx.kind(p0) != Ctx.K.ANY || p0.getKind() != TypeKind.DECLARED) return null;
    String owner = cx.ownerOf(re).getQualifiedName().toString(), name = re.getSimpleName().toString();
    if (owner.equals("java.util.Map")) {
      switch (name) {
        case "get", "containsKey", "remove": return args.size() == 1 ? "Map_" + name + "_s" : null;
        case "put", "getOrDefault", "putIfAbsent", "computeIfAbsent": return "Map_" + name + "_s";
        default: return null;
      }
    }
    if (owner.equals("java.util.Collection")) {
      switch (name) {
        case "contains", "remove": return "Collection_" + name + "_s";  // remove(int) is not a root of Collection's
        case "add": {
          // A list stores the box in any case; only a set can find that it has the element already.
          TypeMirror set = cx.elems.getTypeElement("java.util.Set").asType();
          return cx.types.isSubtype(cx.erase(recv.type), cx.erase(set)) ? "Collection_add_s" : null;
        }
        default: return null;
      }
    }
    return null;
  }

  CallResult invocation(MethodInvocationTree t) {
    ExpressionTree sel = t.getMethodSelect();
    Element el = elemOf(t);
    if (!(el instanceof ExecutableElement)) el = elemOf(sel);
    if (!(el instanceof ExecutableElement e)) throw unsupported(t, "unresolved call " + sel);
    if (Conv.IDENT && t.getArguments().size() == 2 && identityFn(e)) {
      // rule "identity functions": the call is the comparison
      String eq = identityCall(t, t.getArguments().get(0), t.getArguments().get(1));
      if (eq != null) return new CallResult(eq, prim(TypeKind.BOOLEAN));
    }
    List<Arg> as = args(t.getArguments());
    curInst = typeOf(sel) instanceof ExecutableType et ? et : null;
    retOverride = e.getModifiers().contains(Modifier.STATIC) && !cx.isTypedColl(cx.refinedTrees.get(t)) ? cx.refinedTrees.get(t) : null;
    if (sel instanceof MemberSelectTree ms) {
      ExpressionTree q = ms.getExpression();
      if (e.getModifiers().contains(Modifier.STATIC)) return special(e, null, as);
      if (q instanceof IdentifierTree id && id.getName().contentEquals("super")) return call(e, new Recv("this", cc().te.asType()), true, as);
      if (q instanceof MemberSelectTree qm && qm.getIdentifier().contentEquals("super")) {
        // I.super.m(): the default method of interface I.
        TypeElement owner = cx.ownerOf(e);
        TypeMirror site = cc().te.asType();
        List<TypeMirror> params = cx.groupParams(e, site);
        DeclaredType view = cx.superView(site, owner);
        String self = cx.coerce("this", site, view != null && cx.typedDecl(view) ? view : owner.asType());
        String al = argList(e, params, as);
        return new CallResult(cx.className(owner) + "_" + cx.methodName(e) + "_default" + cx.viewArgs(site, owner) + "(" + self + (al.isEmpty() ? "" : ", " + al) + ")", cx.groupReturn(e, site));
      }
      TypeMirror qt = typeOf(q);
      if (qt != null && qt.getKind() == TypeKind.ARRAY && e.getSimpleName().contentEquals("clone")) {
        return new CallResult("append(" + cx.goType(qt) + "(nil), " + expr(q) + "...)", qt);
      }
      // the receiver's code first: a call in it sets curInst for itself
      String rcode = expr(q);
      curInst = typeOf(sel) instanceof ExecutableType et ? et : null;
      retOverride = e.getModifiers().contains(Modifier.STATIC) && !cx.isTypedColl(cx.refinedTrees.get(t)) ? cx.refinedTrees.get(t) : null;
      return special(e, new Recv(rcode, qt), as);
    }
    return special(e, null, as);
  }

  /** Calls with a direct Go spelling. */
  CallResult special(ExecutableElement e, Recv recv, List<Arg> as) {
    TypeElement owner = cx.ownerOf(e);
    String qn = owner.getQualifiedName() + "#" + e.getSimpleName();
    if (recv != null && as.size() == 1 && cx.kind(as.get(0).type) == Ctx.K.STR && cx.kind(recv.type) == Ctx.K.STR) {
      if (qn.equals("java.lang.String#equals")) {
        return new CallResult("(" + recv.code + " == " + as.get(0).as(stringType()) + ")", prim(TypeKind.BOOLEAN));
      }
    }
    if (recv == null && !as.isEmpty() && owner.getQualifiedName().contentEquals("java.util.Arrays")
        && (e.getSimpleName().contentEquals("sort") || e.getSimpleName().contentEquals("setAll") || e.getSimpleName().contentEquals("fill"))) {
      // These write into the array they are given. When the Go slice has another element type than
      // the library function's parameter, it works on a converted copy that is then copied back.
      TypeMirror at = cx.erase(as.get(0).type), pt = cx.erase(e.getParameters().get(0).asType());
      if (at != null && at.getKind() == TypeKind.ARRAY && pt.getKind() == TypeKind.ARRAY && !cx.goType(at).equals(cx.goType(pt))) {
        TypeMirror ca = ((ArrayType) at).getComponentType(), cp = ((ArrayType) pt).getComponentType();
        String arr = fresh("arr"), tmp = fresh("tmp");
        List<Arg> rest = new ArrayList<>(as);
        rest.set(0, new Arg(tmp, pt));
        CallResult inner = call(e, null, false, rest);
        String code = "func() { " + arr + " := " + as.get(0).as(at) + "; " + tmp + " := " + cx.coerce(arr, at, pt) + "; " + inner.code
            + "; for i_ := range " + tmp + " { " + arr + "[i_] = " + cx.coerce(tmp + "[i_]", cp, ca) + " } }()";
        return new CallResult(code, cx.types.getNoType(TypeKind.VOID));
      }
    }
    return call(e, recv, false, as);
  }

  String newClass(NewClassTree t) {
    Element el = elemOf(t);
    TypeMirror ty = cx.erase(typeOf(t));
    TypeElement te = Ctx.typeElement(ty);
    ExecutableElement ctor = (ExecutableElement) el;
    if (t.getClassBody() != null) {
      te = (TypeElement) elemOf(t.getClassBody());
      queueLocalClass(t.getClassBody());
      for (Tree m : t.getClassBody().getMembers()) {
        if (m instanceof MethodTree mt && mt.getName().contentEquals("<init>")) ctor = (ExecutableElement) elemOf(mt);
      }
    }
    if (ctor == null) throw unsupported(t, "unresolved constructor");
    touch(te);
    List<String> pre = new ArrayList<>();
    if (cx.isSource(te)) {
      if (cx.hasOuter(te)) {
        if (t.getEnclosingExpression() != null) pre.add(expr(t.getEnclosingExpression()));
        else pre.add(outerFor(te));
      }
      for (VariableElement cap : capturesOf(te)) pre.add(localRefForCapture(cap));
    }
    List<TypeMirror> params = ownParams(ctor);
    String name = cx.ctorName(ctor, "new_");
    if (!cx.isSource(te)) noteMethod("func " + name, ctor, params, ty, "constructor");
    if (cx.isSource(te) && (te.getNestingKind() == NestingKind.LOCAL || te.getNestingKind() == NestingKind.ANONYMOUS) && !cx.goTparams(te).isEmpty()) {
      // a local or anonymous class of a generic scope: at that scope's type parameters
      name += cx.goTparamsUse(te);
    } else if (cx.typedDecl(ty)) {
      // new C<X>(...): new_C[X], its parameters at those type arguments
      params = new ArrayList<>();
      for (VariableElement pe : ctor.getParameters()) params.add(cx.erase(cx.atSite(cx.vtype(pe), te, ty)));
      name += cx.typeArgsOf(ty);
    }
    String al = argList(ctor, params, args(t.getArguments()));
    if (!al.isEmpty()) pre.add(al);
    return name + "(" + String.join(", ", pre) + ")";
  }

  String localRefForCapture(VariableElement cap) {
    return localRef(cap);
  }

  /** The enclosing instance to pass when creating an inner class te from the current context. */
  String outerFor(TypeElement te) {
    TypeElement outer = cx.enclosingClass(te);
    String code = "this";
    for (ClassCtx c : classStack) {
      if (cx.sub(c.te.asType(), outer.asType())) {
        return c.te.equals(outer) ? code : cx.coerce(code, c.te.asType(), outer.asType());
      }
      code += ".outer_";
    }
    return "this";
  }

  String newArray(NewArrayTree t, ArrayType ty) {
    TypeMirror comp = ty.getComponentType();
    String gt = cx.goType(ty);
    if (t.getInitializers() != null) {
      StringBuilder sb = new StringBuilder(gt + "{");
      boolean first = true;
      for (ExpressionTree i : t.getInitializers()) {
        if (!first) sb.append(", ");
        first = false;
        if (i instanceof NewArrayTree na && na.getType() == null) sb.append(newArray(na, (ArrayType) cx.erase(comp)));
        else sb.append(exprAs(i, comp));
      }
      return sb.append("}").toString();
    }
    List<? extends ExpressionTree> dims = t.getDimensions();
    String n0 = exprAs(dims.get(0), prim(TypeKind.INT));
    if (dims.size() == 1) {
      if (cx.kind(comp) == Ctx.K.STR) return "newstrarr_(" + n0 + ")";
      return "make(" + gt + ", " + n0 + ")";
    }
    if (dims.size() == 2) {
      TypeMirror inner = ((ArrayType) comp).getComponentType();
      return "newarr2_[" + cx.goType(inner) + "](" + n0 + ", " + exprAs(dims.get(1), prim(TypeKind.INT)) + ")";
    }
    throw unsupported(t, "array of " + dims.size() + " dimensions");
  }

  // ------------------------------------------------------------ lambdas

  /** The adapter struct of functional interface fi for a lambda of type ft: I_fn, or I_fn[X] for a typed interface. */
  String adapterName(TypeElement fi, TypeMirror ft) {
    if (cx.typedDecl(ft)) return cx.className(fi) + "_fn" + cx.typeArgsOf(ft);
    return adapterName(fi);
  }

  String adapterName(TypeElement fi) {
    String n = cx.className(fi) + "_fn";
    if (!cx.isSource(fi) && cx.extAdapters.add(fi)) {
      ExecutableElement m = cx.functionalMethod(fi);
      cx.noteMethodOn(cx.ownerOf(cx.root(m)), cx.methodName(m), cx.groupParams(m), cx.groupReturn(m));
    }
    return n;
  }

  /**
   * For a lambda or method reference whose target is a library functional interface: the typed
   * adapter X_tfn[...] (Stubs) when a parameter or the result that the erased signature has as
   * `any` has a Java type the adapter reaches with one assertion (Ctx.assertable). Returns the
   * adapter's name with its type arguments, and fills params/ret with the func's Go types; null
   * when the plain adapter X_fn is used.
   */
  String typedAdapter(TypeElement fi, ExecutableElement m, TypeMirror ft0, List<TypeMirror> inst, List<TypeMirror> params, TypeMirror[] ret) {
    if (System.getenv("J2GO_ERASED") != null || cx.isSource(fi) || !(ft0 instanceof DeclaredType ft)) return null;
    List<TypeMirror> gp = cx.groupParams(m);
    TypeMirror gr = cx.groupReturn(m);
    TypeMirror ir;
    try {
      ir = ((ExecutableType) cx.types.asMemberOf(ft, m)).getReturnType();
    } catch (RuntimeException ex) {
      return null;
    }
    StringBuilder ta = new StringBuilder();
    boolean any = false;
    List<TypeMirror> ps = new ArrayList<>();
    for (int i = 0; i < gp.size(); i++) {
      TypeMirror g = gp.get(i), p = i < inst.size() ? inst.get(i) : null;
      if (!cx.goType(g).equals("any")) {
        ps.add(g);
        continue;
      }
      boolean typed = p != null && cx.assertable(p) && p.getKind() != TypeKind.WILDCARD;
      ps.add(typed ? cx.erase(p) : g);
      ta.append(ta.length() > 0 ? ", " : "").append(typed ? cx.goType(p) : "any");
      any |= typed;
    }
    TypeMirror r = gr;
    if (gr.getKind() != TypeKind.VOID && cx.goType(gr).equals("any")) {
      boolean typed = ir != null && ir.getKind() != TypeKind.WILDCARD && ir.getKind() != TypeKind.TYPEVAR && cx.assertable(ir);
      if (typed) r = cx.erase(ir);
      ta.append(ta.length() > 0 ? ", " : "").append(typed ? cx.goType(ir) : "any");
      any |= typed;
    }
    if (!any) return null;
    params.clear();
    params.addAll(ps);
    ret[0] = r;
    cx.className(fi);
    boolean generic = cx.mentionsTypedVar(r);
    for (TypeMirror p : ps) generic |= cx.mentionsTypedVar(p);
    if (!generic) {
      // Every type is known here: a concrete adapter per instantiation (Stubs), whose conversions
      // are the ones the erased code makes for these types.
      String n = cx.className(fi) + "_tfn_" + ta.toString().replace("*", "").replace("[]", "s_").replace(", ", "_").replaceAll("[^A-Za-z0-9_]", "_");
      Object[] was = cx.extConcreteAdapters.get(n);
      if (was == null) {
        adapterName(fi);
        cx.extConcreteAdapters.put(n, new Object[] {fi, new ArrayList<>(ps), r, ta.toString()});
      } else if (!was[3].equals(ta.toString())) {
        throw new IllegalStateException("typed adapter name clash: " + n + " for " + was[3] + " and " + ta);
      }
      return n;
    }
    if (cx.extTypedAdapters.add(fi)) adapterName(fi);
    return cx.className(fi) + "_tfn[" + ta + "]";
  }

  String lambda(LambdaExpressionTree t) {
    return lambda(t, null);
  }

  /** The lambda as a value of its own type, or (rule "function types") of function type target. */
  String lambda(LambdaExpressionTree t, TypeMirror target) {
    TypeMirror ft = target != null ? target : cx.erase(typeOf(t));
    // rule "typed library interfaces": the erased adapter object, then as the typed interface
    if (cx.lg.is(ft)) return cx.coerce(lambda(t, cx.types.erasure(ft)), cx.types.erasure(ft), ft);
    TypeElement fi = Ctx.typeElement(ft);
    ExecutableElement m = fi == null ? null : cx.functionalMethod(fi);
    if (m == null) throw unsupported(t, "lambda target " + ft);
    List<TypeMirror> params = cx.groupParams(m, ft);
    TypeMirror ret = cx.groupReturn(m, ft);
    List<TypeMirror> declared = new ArrayList<>();
    for (VariableTree p : t.getParameters()) declared.add(elemOf(p).asType());
    TypeMirror[] tret = {ret};
    boolean fnt = cx.fn.isLib(fi);
    if (fnt && target != null && typeOf(t) != null && !cx.goType(typeOf(t)).equals(cx.goType(target))) cx.fn.note("lambdas and method references written at a place of another instantiation", cx.goType(typeOf(t)) + " at " + cx.goType(target));
    String tad = fnt ? null : typedAdapter(fi, m, typeOf(t), declared, params, tret);
    ret = tret[0];
    MethodCtx saved = mc;
    MethodCtx inner = new MethodCtx();
    inner.names = saved.names;
    inner.reads = saved.reads;
    inner.counter = saved.counter;
    inner.hoisted = saved.hoisted;
    inner.staticCtx = saved.staticCtx;
    inner.touched = saved.touched;
    inner.isVoid = ret.getKind() == TypeKind.VOID;
    inner.ret = inner.isVoid ? null : ret;
    mc = inner;
    StringBuilder sb = new StringBuilder(fnt ? cx.goType(ft) + "(func(" : "&" + (tad != null ? tad : adapterName(fi, ft)) + "{func(");
    StringBuilder entry = new StringBuilder();
    for (int i = 0; i < t.getParameters().size(); i++) {
      VariableTree p = t.getParameters().get(i);
      Element pe = elemOf(p);
      String name = localName(pe);
      if (name.equals("_")) name = fresh("p");
      mc.names.put(pe, name);
      TypeMirror pt = cx.erase(pe.asType());
      if (i > 0) sb.append(", ");
      String gp = cx.goType(params.get(i));
      if (cx.goType(pt).equals(gp)) {
        sb.append(name).append(" ").append(gp);
      } else {
        sb.append(name).append("_ ").append(gp);
        entry.append("var ").append(name).append(" ").append(cx.goType(pt)).append(" = ")
            .append(cx.coerce(name + "_", params.get(i), pt)).append("\n_ = ").append(name).append("\n");
      }
    }
    sb.append(")");
    String gr = cx.goType(ret);
    if (!gr.isEmpty()) sb.append(" ").append(gr);
    sb.append(" {\n");
    StringBuilder body = new StringBuilder(entry);
    if (t.getBodyKind() == LambdaExpressionTree.BodyKind.EXPRESSION) {
      ExpressionTree be = (ExpressionTree) t.getBody();
      String code = inner.isVoid ? stmtExpr(be) : "return " + resultAs(be, ret);
      for (String p : mc.pre) body.append(p).append("\n");
      mc.pre.clear();
      body.append(code).append("\n");
    } else {
      body.append(blockCode((BlockTree) t.getBody()));
      if (!inner.isVoid) body.append("panic(\"unreachable\")\n");
    }
    sb.append(indent(body.toString(), "\t"));
    sb.append(fnt ? "})" : "}}");
    saved.counter = inner.counter;
    mc = saved;
    return sb.toString();
  }

  String memberRef(MemberReferenceTree t) {
    return memberRef(t, null);
  }

  /** The method reference as a value of its own type, or (rule "function types") of function type target. */
  String memberRef(MemberReferenceTree t, TypeMirror target) {
    TypeMirror ft = typeOf(t);
    TypeMirror eft = target != null ? target : cx.erase(ft);
    if (cx.lg.is(eft)) return cx.coerce(memberRef(t, cx.types.erasure(eft)), cx.types.erasure(eft), eft);
    TypeElement fi = Ctx.typeElement(eft);
    ExecutableElement m = fi == null ? null : cx.functionalMethod(fi);
    if (m == null) throw unsupported(t, "method reference target " + ft);
    boolean fnt = cx.fn.isLib(fi);
    if (fnt && target != null && ft != null && !cx.goType(ft).equals(cx.goType(target))) cx.fn.note("lambdas and method references written at a place of another instantiation", cx.goType(ft) + " at " + cx.goType(target));
    List<TypeMirror> params = cx.groupParams(m, eft);
    TypeMirror ret = cx.groupReturn(m, eft);
    // Instantiated parameter types, to coerce from the erased ones.
    List<TypeMirror> inst = new ArrayList<>();
    try {
      ExecutableType mt = (ExecutableType) cx.types.asMemberOf((DeclaredType) ft, m);
      for (TypeMirror p : mt.getParameterTypes()) inst.add(cx.erase(p));
    } catch (RuntimeException ex) {
      inst.addAll(params);
    }
    TypeMirror[] tret = {ret};
    String tad = fnt ? null : typedAdapter(fi, m, ft, inst, params, tret);
    ret = tret[0];
    Element el = elemOf(t);
    ExpressionTree q = t.getQualifierExpression();
    Element qe = elemOf(q);
    boolean typeQualifier = qe instanceof TypeElement || q instanceof PrimitiveTypeTree || q instanceof ArrayTypeTree
        || q instanceof ParameterizedTypeTree || (qe == null && typeOf(q) != null && !(q instanceof IdentifierTree id && id.getName().contentEquals("this")) && !(q instanceof ExpressionTree && typeMap.containsKey(q) && elemMap.get(q) instanceof VariableElement) && isTypeTree(q));
    StringBuilder sb = new StringBuilder();
    List<Arg> as = new ArrayList<>();
    StringBuilder sig = new StringBuilder();
    for (int i = 0; i < params.size(); i++) {
      if (i > 0) sig.append(", ");
      sig.append("p").append(i).append(" ").append(cx.goType(params.get(i)));
      // Each parameter is used at its instantiated type.
      as.add(new Arg(cx.coerce("p" + i, params.get(i), inst.get(i)), inst.get(i)));
    }
    String prefix = "";
    CallResult r;
    if (t.getMode() == MemberReferenceTree.ReferenceMode.NEW) {
      TypeMirror qt = cx.erase(typeOf(q));
      if (qt != null && qt.getKind() == TypeKind.ARRAY) {
        r = new CallResult("make(" + cx.goType(qt) + ", " + as.get(0).as(prim(TypeKind.INT)) + ")", qt);
      } else {
        ExecutableElement ctor = (ExecutableElement) el;
        TypeElement te = cx.ownerOf(ctor);
        List<TypeMirror> cp = ownParams(ctor);
        String name = cx.ctorName(ctor, "new_");
        if (!cx.isSource(te)) noteMethod("func " + name, ctor, cp, te.asType(), "constructor");
        TypeMirror made = te.asType();
        if (cx.typedClasses.contains(te)) {
          // C::new for a typed class: C at the type arguments the functional method returns
          TypeMirror want = ((ExecutableType) cx.types.asMemberOf((DeclaredType) ft, m)).getReturnType();
          if (want.getKind() != TypeKind.DECLARED || !Ctx.typeElement(want).equals(te)) throw unsupported(t, "constructor reference to a typed class at " + want);
          made = want;
          cp = new ArrayList<>();
          for (VariableElement pe : ctor.getParameters()) cp.add(cx.erase(cx.atSite(pe.asType(), te, want)));
          name += cx.typeArgsOf(want);
        }
        List<String> pre = new ArrayList<>();
        if (cx.isSource(te) && cx.hasOuter(te)) pre.add(outerFor(te));
        String al = argList(ctor, cp, as);
        if (!al.isEmpty()) pre.add(al);
        r = new CallResult(name + "(" + String.join(", ", pre) + ")", made);
      }
    } else {
      ExecutableElement e = (ExecutableElement) el;
      if (e.getModifiers().contains(Modifier.STATIC)) {
        r = call(e, null, false, as);
      } else if (typeQualifier) {
        Arg first = as.remove(0);
        TypeMirror rt = cx.erase(typeOf(q));
        if (rt == null) rt = cx.ownerOf(e).asType();
        // A typed declaration named raw (DiGraphNode::getPriority): the receiver at its own type arguments.
        if (rt.getKind() == TypeKind.DECLARED && cx.typedClasses.contains(Ctx.typeElement(rt)) && ((DeclaredType) rt).getTypeArguments().isEmpty()
            && first.type != null && cx.sub(first.type, rt)) rt = first.type;
        r = special(e, new Recv(first.as(rt), rt), as);
      } else if (q instanceof IdentifierTree id && id.getName().contentEquals("super")) {
        r = call(e, new Recv("this", cc().te.asType()), true, as);
      } else {
        String rc = expr(q);
        if (!(strip(q) instanceof IdentifierTree)) {
          String tmp = fresh("recv");
          prefix = tmp + " := " + rc + "; ";
          rc = tmp;
        }
        r = special(e, new Recv(rc, typeOf(q)), as);
      }
    }
    String an = fnt ? cx.goType(eft) : tad != null ? tad : adapterName(fi, cx.erase(ft));
    String fn = (fnt ? an + "(func(" : "&" + an + "{func(") + sig + ")";
    String gr = cx.goType(ret);
    String body = ret.getKind() == TypeKind.VOID ? r.code : "return " + cx.coerce(r.code, r.type, ret);
    fn += (gr.isEmpty() ? "" : " " + gr) + " { " + body + (fnt ? " })" : " }}");
    if (prefix.isEmpty()) return fn;
    return "func() " + (fnt ? "" : "*") + an + " { " + prefix + "return " + fn + " }()";
  }

  boolean isTypeTree(Tree q) {
    if (q instanceof MemberSelectTree ms) {
      Element e = elemOf(ms);
      return e instanceof TypeElement;
    }
    if (q instanceof IdentifierTree) return elemOf(q) instanceof TypeElement;
    return false;
  }

  // ------------------------------------------------------------ expression statements

  String stmtExpr(ExpressionTree t0) {
    ExpressionTree t = strip(t0);
    switch (t.getKind()) {
      case METHOD_INVOCATION:
        return invocation((MethodInvocationTree) t).code;
      case ASSIGNMENT: {
        AssignmentTree a = (AssignmentTree) t;
        Object[] slot = strSlotRead(a.getVariable());
        if (slot != null) {
          String fn = slot[0] + "." + cx.fieldName((VariableElement) slot[1]);
          Object[] from = strSlotRead(a.getExpression());
          if (from != null && from[1] == slot[1]) return fn + "_copy_(" + from[0] + ")";
          TypeMirror rt = typeOf(a.getExpression());
          if (rt != null && cx.kind(rt) == Ctx.K.STR) return fn + "_sets_(" + expr(a.getExpression()) + ")";
          if (cx.valCode((VariableElement) slot[1]) && rt != null && rt.getKind() == TypeKind.DOUBLE) return fn + "_setd_(" + expr(a.getExpression()) + ")";
          return fn + "_set_(" + asAny(a.getExpression()) + ")";
        }
        Object[] rtf = fieldRead(a.getVariable(), f -> cx.runtimeField(f) != null);
        if (rtf != null) {
          VariableElement f = (VariableElement) rtf[1];
          Object[] from = fieldRead(a.getExpression(), g -> g == f);
          String fn = rtf[0] + "." + cx.fieldName(f);
          if (from != null) return fn + "_copy_(" + from[0] + ")";
          return fn + "_set_(" + exprAs(a.getExpression(), cx.erase(f.asType())) + ")";
        }
        Object[] idf = fieldRead(a.getVariable(), f -> cx.strId(f) || cx.handleTable(f) != null);
        if (idf != null) {
          // A string id or table handle: a copy from the same field of another object copies the id.
          VariableElement f = (VariableElement) idf[1];
          Object[] from = fieldRead(a.getExpression(), g -> g == f);
          // (the table counts the fields holding a handle).
          if (from != null && cx.handleTable(f) != null) return "htab_copy_(&" + cx.fieldLoc((String) idf[0], f) + ", " + cx.fieldLoc((String) from[0], f) + ")";
          if (from != null) return cx.fieldLoc((String) idf[0], f) + " = " + cx.fieldLoc((String) from[0], f);
          if (cx.handleTable(f) != null) return idf[0] + "." + cx.fieldName(f) + "_set_(" + asAny(a.getExpression()) + ")";
        }
        LValue lv = lvalue(a.getVariable());
        String val = exprAs(a.getExpression(), lv.type);
        return lv.code + " = " + (lv.field == null ? val : cx.stored(lv.field, val));
      }
      case POSTFIX_INCREMENT, POSTFIX_DECREMENT, PREFIX_INCREMENT, PREFIX_DECREMENT:
        return incDecStmt((UnaryTree) t);
      case NEW_CLASS:
        return "_ = " + newClass((NewClassTree) t);
      default:
        if (t instanceof CompoundAssignmentTree c) return compoundStmt(c);
        return "_ = " + expr(t);
    }
  }
}
