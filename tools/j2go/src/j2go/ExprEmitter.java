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
        CallResult r = invocation(mi);
        return cx.coerce(r.code, r.type, typeOf(t));
      }
      case NEW_CLASS:
        return newClass((NewClassTree) t);
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
      if (cx.types.isSubtype(cx.erase(c.te.asType()), ot)) return new Recv(code, c.te.asType());
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
    if (isLocalKind(e)) return cx.coerce(localRef(e), e.asType(), typeOf(t));
    switch (e.getKind()) {
      case FIELD, ENUM_CONSTANT: {
        VariableElement f = (VariableElement) e;
        if (f.getModifiers().contains(Modifier.STATIC)) return cx.coerce(staticField(f), f.asType(), typeOf(t));
        Recv r = implicitRecv(cx.ownerOf(f));
        noteField(f);
        return cx.coerce(r.code + "." + cx.fieldName(f), f.asType(), typeOf(t));
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
        if (f.getModifiers().contains(Modifier.STATIC)) return cx.coerce(staticField(f), f.asType(), typeOf(t));
        String fn = cx.fieldName(f);
        noteField(f);
        String recv;
        if (q instanceof IdentifierTree id && id.getName().contentEquals("super")) {
          recv = "this";
        } else {
          recv = expr(q);
        }
        return cx.coerce(recv + "." + fn, f.asType(), typeOf(t));
      }
      default:
        throw unsupported(t, "member of kind " + e.getKind());
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
        lv.type = cx.erase(e.asType());
        return lv;
      }
      VariableElement f = (VariableElement) e;
      lv.type = cx.erase(f.asType());
      if (f.getModifiers().contains(Modifier.STATIC)) lv.code = staticField(f);
      else lv.code = implicitRecv(cx.ownerOf(f)).code + "." + cx.fieldName(f);
      noteField(f);
      return lv;
    }
    if (t instanceof MemberSelectTree ms) {
      VariableElement f = (VariableElement) e;
      lv.type = cx.erase(f.asType());
      if (f.getModifiers().contains(Modifier.STATIC)) {
        lv.code = staticField(f);
      } else {
        ExpressionTree q = ms.getExpression();
        String recv = q instanceof IdentifierTree id && id.getName().contentEquals("super") ? "this" : expr(q);
        lv.code = recv + "." + cx.fieldName(f);
      }
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
      default:
        return "objstr_(" + asAny(t) + ")";
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
    if (k == Tree.Kind.EQUAL_TO || k == Tree.Kind.NOT_EQUAL_TO) return equality(t);
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

  String equality(BinaryTree t) {
    ExpressionTree l = t.getLeftOperand(), r = t.getRightOperand();
    TypeMirror tl = typeOf(l), tr = typeOf(r);
    String op = t.getKind() == Tree.Kind.EQUAL_TO ? "==" : "!=";
    Ctx.K kl = cx.kind(tl), kr = cx.kind(tr);
    if (kl == Ctx.K.NULL || kr == Ctx.K.NULL) {
      ExpressionTree x = kl == Ctx.K.NULL ? r : l;
      Ctx.K kx = kl == Ctx.K.NULL ? kr : kl;
      if (kx == Ctx.K.NULL) return op.equals("==") ? "true" : "false";
      if (kx == Ctx.K.STR) return "(" + expr(x) + " " + op + " nullstr_)";
      return "(" + expr(x) + " " + op + " nil)";
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

  String instanceOf(InstanceOfTree t) {
    String v = asAny(t.getExpression());
    if (t.getPattern() == null) return isCheck(v, typeOf(t.getType()) != null ? typeOf(t.getType()) : cx.trees.getTypeMirror(cx.trees.getPath(cu, t.getType())));
    if (!(t.getPattern() instanceof BindingPatternTree bp)) throw unsupported(t, "pattern " + t.getPattern().getKind());
    VariableTree var = bp.getVariable();
    Element ve = elemOf(var);
    TypeMirror vt = cx.erase(ve.asType());
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
    mc.pre.add("var " + name + " " + cx.goType(ve.asType()) + "\n_ = " + name);
    return name;
  }

  String incDecExpr(UnaryTree t) {
    // Prep step 09: ++ and -- are always statements of their own.
    throw unsupported(t, "++/-- used as a value");
  }

  String incDecStmt(UnaryTree t) {
    LValue lv = lvalue(t.getExpression());
    boolean inc = t.getKind() == Tree.Kind.POSTFIX_INCREMENT || t.getKind() == Tree.Kind.PREFIX_INCREMENT;
    Element e = elemOf(strip(t.getExpression()));
    if (e != null) mc.reads.add(e);
    if (lv.type.getKind().isPrimitive()) return lv.code + (inc ? "++" : "--");
    TypeMirror u = cx.unboxed(lv.type);
    return lv.code + " = any(" + cx.coerce(lv.code, lv.type, u) + (inc ? " + 1" : " - 1") + ")";
  }

  String compoundStmt(CompoundAssignmentTree t) {
    LValue lv = lvalue(t.getVariable());
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
    for (int i = 0; i < fixed && i < args.size(); i++) out.add(args.get(i).as(params.get(i)));
    if (pack) {
      TypeMirror arr = params.get(n - 1);
      TypeMirror comp = ((ArrayType) arr).getComponentType();
      StringBuilder sb = new StringBuilder(cx.goType(arr) + "{");
      for (int i = fixed; i < args.size(); i++) {
        if (i > fixed) sb.append(", ");
        sb.append(args.get(i).as(comp));
      }
      out.add(sb.append("}").toString());
    }
    return String.join(", ", out);
  }

  List<TypeMirror> ownParams(ExecutableElement e) {
    List<TypeMirror> out = new ArrayList<>();
    for (VariableElement p : e.getParameters()) out.add(cx.erase(p.asType()));
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
    if (isStatic) {
      touch(owner);
      List<TypeMirror> params = ownParams(e);
      TypeMirror ret = cx.erase(e.getReturnType());
      String name = cx.staticMethodName(e);
      if (!src) noteMethod("func " + name, e, params, ret, "static");
      return new CallResult(name + "(" + argList(e, params, args) + ")", ret);
    }
    if (recv == null) recv = implicitRecv(owner);
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
      return new CallResult(cx.staticMethodName(e) + "(" + cx.coerce(recv.code, recv.type, owner.asType()) + (al.isEmpty() ? "" : ", " + al) + ")", cx.erase(e.getReturnType()));
    }
    Ctx.K ok = cx.kind(owner.asType());
    ExecutableElement re = cx.root(e);
    TypeElement ro = cx.ownerOf(re);
    if (!src && ok != Ctx.K.STR && re != e && !superCall
        && (cx.kind(ro.asType()) == Ctx.K.ANY && ro.getQualifiedName().contentEquals("java.lang.Object")
            || !cx.isSource(ro) && ro.getKind().isInterface() && re.isDefault())) {
      // Declared again by a library type, but really Object's method or an interface default:
      // dispatch through the helper of the root.
      e = re;
      owner = ro;
      ok = cx.kind(owner.asType());
    }
    if (!src && (ok == Ctx.K.STR || ok == Ctx.K.ANY || owner.getKind().isInterface() && e.isDefault())) {
      // Static-style helper taking the receiver first.
      List<TypeMirror> params = ownParams(e);
      TypeMirror ret = cx.erase(e.getReturnType());
      String name = cx.staticMethodName(e);
      String r = cx.coerce(recv.code, recv.type, owner.asType());
      String al = argList(e, params, args);
      List<TypeMirror> full = new ArrayList<>();
      full.add(cx.erase(owner.asType()));
      full.addAll(params);
      noteMethod("func " + name, e, full, ret, "receiver-first");
      return new CallResult(name + "(" + r + (al.isEmpty() ? "" : ", " + al) + ")", ret);
    }
    List<TypeMirror> params = cx.groupParams(e);
    TypeMirror ret = cx.groupReturn(e);
    String name = cx.methodName(e);
    if (!src) noteMethod("method " + cx.className(cx.ownerOf(cx.root(e))) + "." + name, cx.root(e), params, ret, "method");
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
      if (rk == Ctx.K.ANY || rk == Ctx.K.STR || rk == Ctx.K.ARR || rk == Ctx.K.PRIM) {
        rc = cx.coerce(rc, recv.type, owner.asType());
      } else if (rk == Ctx.K.CLS && owner.getKind().isInterface() && !cx.types.isSubtype(cx.erase(recv.type), cx.erase(owner.asType()))) {
        rc = cx.coerce(rc, recv.type, owner.asType());
      }
      target = rc + "." + name;
    }
    return new CallResult(target + "(" + argList(e, params, args) + ")", ret);
  }

  CallResult invocation(MethodInvocationTree t) {
    ExpressionTree sel = t.getMethodSelect();
    Element el = elemOf(t);
    if (!(el instanceof ExecutableElement)) el = elemOf(sel);
    if (!(el instanceof ExecutableElement e)) throw unsupported(t, "unresolved call " + sel);
    List<Arg> as = args(t.getArguments());
    if (sel instanceof MemberSelectTree ms) {
      ExpressionTree q = ms.getExpression();
      if (e.getModifiers().contains(Modifier.STATIC)) return special(e, null, as);
      if (q instanceof IdentifierTree id && id.getName().contentEquals("super")) return call(e, new Recv("this", cc().te.asType()), true, as);
      if (q instanceof MemberSelectTree qm && qm.getIdentifier().contentEquals("super")) {
        // I.super.m(): the default method of interface I.
        TypeElement owner = cx.ownerOf(e);
        List<TypeMirror> params = cx.groupParams(e);
        String self = cx.coerce("this", cc().te.asType(), owner.asType());
        String al = argList(e, params, as);
        return new CallResult(cx.className(owner) + "_" + cx.methodName(e) + "_default(" + self + (al.isEmpty() ? "" : ", " + al) + ")", cx.groupReturn(e));
      }
      TypeMirror qt = typeOf(q);
      if (qt != null && qt.getKind() == TypeKind.ARRAY && e.getSimpleName().contentEquals("clone")) {
        return new CallResult("append(" + cx.goType(qt) + "(nil), " + expr(q) + "...)", qt);
      }
      return special(e, new Recv(expr(q), qt), as);
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
      if (cx.types.isSubtype(cx.erase(c.te.asType()), cx.erase(outer.asType()))) {
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

  String adapterName(TypeElement fi) {
    String n = cx.className(fi) + "_fn";
    if (!cx.isSource(fi) && cx.extAdapters.add(fi)) {
      ExecutableElement m = cx.functionalMethod(fi);
      cx.noteMethodOn(cx.ownerOf(cx.root(m)), cx.methodName(m), cx.groupParams(m), cx.groupReturn(m));
    }
    return n;
  }

  String lambda(LambdaExpressionTree t) {
    TypeMirror ft = cx.erase(typeOf(t));
    TypeElement fi = Ctx.typeElement(ft);
    ExecutableElement m = fi == null ? null : cx.functionalMethod(fi);
    if (m == null) throw unsupported(t, "lambda target " + ft);
    List<TypeMirror> params = cx.groupParams(m);
    TypeMirror ret = cx.groupReturn(m);
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
    StringBuilder sb = new StringBuilder("&" + adapterName(fi) + "{func(");
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
      String code = inner.isVoid ? stmtExpr(be) : "return " + exprAs(be, ret);
      for (String p : mc.pre) body.append(p).append("\n");
      mc.pre.clear();
      body.append(code).append("\n");
    } else {
      body.append(blockCode((BlockTree) t.getBody()));
      if (!inner.isVoid) body.append("panic(\"unreachable\")\n");
    }
    sb.append(indent(body.toString(), "\t"));
    sb.append("}}");
    saved.counter = inner.counter;
    mc = saved;
    return sb.toString();
  }

  String memberRef(MemberReferenceTree t) {
    TypeMirror ft = typeOf(t);
    TypeElement fi = Ctx.typeElement(cx.erase(ft));
    ExecutableElement m = fi == null ? null : cx.functionalMethod(fi);
    if (m == null) throw unsupported(t, "method reference target " + ft);
    List<TypeMirror> params = cx.groupParams(m);
    TypeMirror ret = cx.groupReturn(m);
    // Instantiated parameter types, to coerce from the erased ones.
    List<TypeMirror> inst = new ArrayList<>();
    try {
      ExecutableType mt = (ExecutableType) cx.types.asMemberOf((DeclaredType) ft, m);
      for (TypeMirror p : mt.getParameterTypes()) inst.add(cx.erase(p));
    } catch (RuntimeException ex) {
      inst.addAll(params);
    }
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
        List<String> pre = new ArrayList<>();
        if (cx.isSource(te) && cx.hasOuter(te)) pre.add(outerFor(te));
        String al = argList(ctor, cp, as);
        if (!al.isEmpty()) pre.add(al);
        r = new CallResult(name + "(" + String.join(", ", pre) + ")", te.asType());
      }
    } else {
      ExecutableElement e = (ExecutableElement) el;
      if (e.getModifiers().contains(Modifier.STATIC)) {
        r = call(e, null, false, as);
      } else if (typeQualifier) {
        Arg first = as.remove(0);
        TypeMirror rt = cx.erase(typeOf(q));
        if (rt == null) rt = cx.ownerOf(e).asType();
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
    String fn = "&" + adapterName(fi) + "{func(" + sig + ")";
    String gr = cx.goType(ret);
    String body = ret.getKind() == TypeKind.VOID ? r.code : "return " + cx.coerce(r.code, r.type, ret);
    fn += (gr.isEmpty() ? "" : " " + gr) + " { " + body + " }}";
    if (prefix.isEmpty()) return fn;
    return "func() *" + adapterName(fi) + " { " + prefix + "return " + fn + " }()";
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
        LValue lv = lvalue(a.getVariable());
        return lv.code + " = " + exprAs(a.getExpression(), lv.type);
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
