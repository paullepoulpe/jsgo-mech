package j2go;

import com.sun.source.tree.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/** Statements. */
abstract class StmtEmitter extends ExprEmitter {
  StmtEmitter(Ctx cx, CompilationUnitTree cu) {
    super(cx, cu);
  }

  abstract String ctorCall(MethodInvocationTree t);

  final Map<String, Element> useMarkers = new HashMap<>();
  int markerCount;

  String useMarker(Element e) {
    String m = "//USE#" + (++markerCount) + "#";
    useMarkers.put(m, e);
    return m;
  }

  /** Resolves the unused-variable and unused-label markers of a finished function body. */
  String finish(String code) {
    StringBuilder sb = new StringBuilder();
    for (String line : code.split("\n", -1)) {
      String tr = line.trim();
      if (tr.startsWith("//USE#")) {
        Element e = useMarkers.get(tr);
        if (e != null && !mc.reads.contains(e)) {
          sb.append(line, 0, line.indexOf("//USE#")).append("_ = ").append(mc.names.get(e)).append('\n');
        }
        continue;
      }
      if (tr.startsWith("//LABEL#")) {
        String l = tr.substring(8);
        if (mc.usedLabels.contains(l)) sb.append(line, 0, line.indexOf("//LABEL#")).append(l).append(":\n");
        continue;
      }
      sb.append(line).append('\n');
    }
    if (sb.length() > 0) sb.setLength(sb.length() - 1);
    return sb.toString();
  }

  @Override
  String blockCode(BlockTree b) {
    return stmts(b.getStatements());
  }

  String stmts(List<? extends StatementTree> list) {
    StringBuilder sb = new StringBuilder();
    Set<String> savedHoisted = mc.hoisted;
    mc.hoisted = new HashSet<>(savedHoisted);
    for (StatementTree s : list) {
      String c = stmt(s);
      if (!c.isEmpty()) sb.append(c).append('\n');
    }
    mc.hoisted = savedHoisted;
    return sb.toString();
  }

  String stmt(StatementTree s) {
    String code = stmt0(s);
    return withPre(code);
  }

  /** Takes the declarations that the header expressions of a statement asked for. */
  List<String> holdPre() {
    List<String> held = new ArrayList<>(mc.pre);
    mc.pre.clear();
    return held;
  }

  void restorePre(List<String> held) {
    mc.pre.addAll(0, held);
  }

  String withPre(String code) {
    if (mc.pre.isEmpty()) return code;
    String p = String.join("\n", mc.pre);
    mc.pre.clear();
    return p + "\n" + code;
  }

  String braced(StatementTree s) {
    String body = s instanceof BlockTree b ? blockCode(b) : stmt(s) + "\n";
    return "{\n" + indent(body, "\t") + "}";
  }

  // ---- scopes

  int pushScope(String kind, String label, Object data) {
    mc.scopeKinds.add(kind);
    mc.scopeLabels.add(label);
    mc.scopes.add(data);
    return mc.scopes.size() - 1;
  }

  void popScope() {
    int i = mc.scopes.size() - 1;
    mc.scopes.remove(i);
    mc.scopeKinds.remove(i);
    mc.scopeLabels.remove(i);
  }

  String pendingLabel;

  /** Pushes a loop or switch scope; returns the label-marker line to put before the statement. */
  String enter(String kind) {
    String label = pendingLabel != null ? pendingLabel : fresh("L");
    pendingLabel = null;
    pushScope(kind, label, null);
    return "//LABEL#" + label + "\n";
  }

  String jump(boolean isBreak, int target) {
    // What lies between here and the target decides the spelling.
    boolean crossesBlock = false;
    for (int i = mc.scopes.size() - 1; i > target; i--) {
      String k = mc.scopeKinds.get(i);
      if (k.equals("try")) {
        TryCtx tc = (TryCtx) mc.scopes.get(i);
        String key = (isBreak ? "b" : "c") + target;
        int idx = tc.exits.indexOf(key);
        if (idx < 0) {
          tc.exits.add(key);
          idx = tc.exits.size() - 1;
        }
        return exitTry(tc, 2 + idx);
      }
      if (k.equals("block") || k.equals("loop") || k.equals("switch") && !isBreak) crossesBlock = true;
      if (k.equals("switch") && isBreak) crossesBlock = true;
    }
    String word = isBreak ? "break" : "continue";
    if (!crossesBlock) return word;
    String l = mc.scopeLabels.get(target);
    mc.usedLabels.add(l);
    return word + " " + l;
  }

  String exitTry(TryCtx tc, int code) {
    if (tc.inHandler) return "code_ = " + code + "\nreturn";
    return "return " + code;
  }

  String breakOrContinue(boolean isBreak, String label) {
    int target = -1;
    for (int i = mc.scopes.size() - 1; i >= 0; i--) {
      String k = mc.scopeKinds.get(i);
      if (label != null) {
        if (label.equals(mc.scopeLabels.get(i)) && !k.equals("try")) {
          target = i;
          break;
        }
      } else if (k.equals("loop") || isBreak && k.equals("switch")) {
        target = i;
        break;
      }
    }
    if (target < 0) throw new UnsupportedOperationException("jump target " + label);
    if (label != null) {
      // A labelled jump always names its label, unless it leaves a try closure first.
      for (int i = mc.scopes.size() - 1; i > target; i--) {
        if (mc.scopeKinds.get(i).equals("try")) return jump(isBreak, target);
      }
      mc.usedLabels.add(label);
      return (isBreak ? "break " : "continue ") + label;
    }
    return jump(isBreak, target);
  }

  String returnCode(String value) {
    for (int i = mc.scopes.size() - 1; i >= 0; i--) {
      if (mc.scopeKinds.get(i).equals("try")) {
        TryCtx tc = (TryCtx) mc.scopes.get(i);
        tc.usesReturn = true;
        String set = value == null ? "" : "ret_" + tc.id + " = " + value + "\n";
        return set + exitTry(tc, 1);
      }
    }
    return value == null ? "return" : "return " + value;
  }

  // ---- statements

  static boolean isTrue(ExpressionTree t) {
    ExpressionTree s = strip(t);
    return s instanceof LiteralTree l && Boolean.TRUE.equals(l.getValue());
  }

  String stmt0(StatementTree s) {
    switch (s.getKind()) {
      case BLOCK:
        return "{\n" + indent(blockCode((BlockTree) s), "\t") + "}";
      case EMPTY_STATEMENT:
        return "";
      case VARIABLE:
        return localVar((VariableTree) s);
      case CLASS, INTERFACE, ENUM, RECORD:
        queueLocalClass((ClassTree) s);
        return "";
      case EXPRESSION_STATEMENT: {
        ExpressionTree e = strip(((ExpressionStatementTree) s).getExpression());
        if (e instanceof MethodInvocationTree mi && mi.getMethodSelect() instanceof IdentifierTree id
            && (id.getName().contentEquals("this") || id.getName().contentEquals("super"))) {
          return ctorCall(mi);
        }
        if (e instanceof MethodInvocationTree mi && mi.getMethodSelect() instanceof MemberSelectTree ms
            && ms.getIdentifier().contentEquals("super") && elemOf(mi) != null && elemOf(mi).getKind() == ElementKind.CONSTRUCTOR) {
          return ctorCall(mi);
        }
        return stmtExpr(e);
      }
      case IF: {
        IfTree t = (IfTree) s;
        String c0 = cond(t.getCondition());
        List<String> held = holdPre();
        String code = "if " + c0 + " " + braced(t.getThenStatement());
        StatementTree el = t.getElseStatement();
        if (el != null) {
          if (el instanceof IfTree) {
            List<String> savedPre = mc.pre;
            mc.pre = new ArrayList<>();
            String ec = stmt0(el);
            List<String> innerPre = mc.pre;
            mc.pre = savedPre;
            if (innerPre.isEmpty()) code += " else " + ec;
            else code += " else {\n" + indent(String.join("\n", innerPre) + "\n" + ec + "\n", "\t") + "}";
          } else {
            code += " else " + braced(el);
          }
        }
        restorePre(held);
        return code;
      }
      case WHILE_LOOP: {
        WhileLoopTree t = (WhileLoopTree) s;
        String lab = enter("loop");
        String head = isTrue(t.getCondition()) ? "for " : "for " + cond(t.getCondition()) + " ";
        List<String> held = holdPre();
        String code = lab + head + braced(t.getStatement());
        popScope();
        restorePre(held);
        return code;
      }
      case DO_WHILE_LOOP: {
        DoWhileLoopTree t = (DoWhileLoopTree) s;
        String lab = enter("loop");
        String v = fresh("do");
        String body = braced(t.getStatement());
        String code = lab + "for " + v + " := true; " + v + "; " + v + " = " + cond(t.getCondition()) + " " + body;
        popScope();
        return code;
      }
      case FOR_LOOP:
        return forLoop((ForLoopTree) s);
      case ENHANCED_FOR_LOOP:
        return forEach((EnhancedForLoopTree) s);
      case LABELED_STATEMENT: {
        LabeledStatementTree t = (LabeledStatementTree) s;
        String label = Ctx.safe(t.getLabel().toString()) + "_";
        StatementTree body = t.getStatement();
        switch (body.getKind()) {
          case WHILE_LOOP, DO_WHILE_LOOP, FOR_LOOP, ENHANCED_FOR_LOOP, SWITCH:
            pendingLabel = label;
            return stmt0(body);
          default: {
            pushScope("block", label, null);
            String code = "//LABEL#" + label + "\nfor range 1 " + braced(body);
            // A labelled block is always left through its label.
            popScope();
            return code;
          }
        }
      }
      case SWITCH:
        return switchStmt((SwitchTree) s);
      case SYNCHRONIZED:
        return "{\n" + indent(blockCode(((SynchronizedTree) s).getBlock()), "\t") + "}";
      case RETURN: {
        ReturnTree t = (ReturnTree) s;
        if (t.getExpression() == null) return returnCode(null);
        if (mc.isVoid) return stmtExpr(t.getExpression()) + "\n" + returnCode(null);
        return returnCode(exprAs(t.getExpression(), mc.ret));
      }
      case YIELD:
        return returnCode(exprAs(((YieldTree) s).getValue(), mc.ret));
      case BREAK: {
        BreakTree t = (BreakTree) s;
        return breakOrContinue(true, t.getLabel() == null ? null : Ctx.safe(t.getLabel().toString()) + "_");
      }
      case CONTINUE: {
        ContinueTree t = (ContinueTree) s;
        return breakOrContinue(false, t.getLabel() == null ? null : Ctx.safe(t.getLabel().toString()) + "_");
      }
      case THROW:
        return "panic(" + asAny(((ThrowTree) s).getExpression()) + ")";
      case TRY:
        return tryStmt((TryTree) s);
      case ASSERT:
        return "";
      default:
        throw unsupported(s, "statement " + s.getKind());
    }
  }

  final Set<Element> predeclared = new HashSet<>();

  /** Variables declared directly in one case of an old-style switch and used in another. */
  String casePredeclarations(List<? extends CaseTree> cases) {
    StringBuilder sb = new StringBuilder();
    for (CaseTree c : cases) {
      if (c.getCaseKind() == CaseTree.CaseKind.RULE || c.getStatements() == null) continue;
      for (StatementTree st : c.getStatements()) {
        if (!(st instanceof VariableTree v)) continue;
        Element ve = elemOf(v);
        boolean[] usedElsewhere = {false};
        for (CaseTree o : cases) {
          if (o == c || o.getStatements() == null) continue;
          new com.sun.source.util.TreeScanner<Void, Void>() {
            @Override
            public Void visitIdentifier(IdentifierTree t, Void x) {
              if (elemOf(t) == ve) usedElsewhere[0] = true;
              return null;
            }
          }.scan(o.getStatements(), null);
        }
        if (usedElsewhere[0]) {
          String name = Ctx.safe(v.getName().toString());
          mc.names.put(ve, name);
          predeclared.add(ve);
          sb.append("var ").append(name).append(" ").append(cx.goType(ve.asType())).append("\n_ = ").append(name).append("\n");
        }
      }
    }
    return sb.toString();
  }

  String localVar(VariableTree v) {
    Element e = elemOf(v);
    if (predeclared.contains(e)) {
      if (v.getInitializer() == null) return "";
      return mc.names.get(e) + " = " + exprAs(v.getInitializer(), cx.erase(e.asType()));
    }
    String name = Ctx.safe(v.getName().toString());
    if (mc.hoisted.contains(name)) name = fresh(name);
    mc.names.put(e, name);
    TypeMirror ty = cx.erase(e.asType());
    String gt = cx.goType(ty);
    String code = "var " + name + " " + gt;
    if (v.getInitializer() != null) {
      ExpressionTree init = v.getInitializer();
      boolean[] self = {false};
      new com.sun.source.util.TreeScanner<Void, Void>() {
        @Override
        public Void visitIdentifier(IdentifierTree t, Void x) {
          if (elemOf(t) == e) self[0] = true;
          return null;
        }
      }.scan(init, null);
      // An initializer that mentions its own variable needs the variable declared first.
      String eq = self[0] ? "\n" + name + " = " : " = ";
      if (init instanceof NewArrayTree na && na.getType() == null) code += eq + newArray(na, (ArrayType) ty);
      else code += eq + exprAs(init, ty);
    }
    return code + "\n" + useMarker(e);
  }

  String forLoop(ForLoopTree t) {
    StringBuilder pre = new StringBuilder();
    String init = "";
    List<? extends StatementTree> inits = t.getInitializer();
    if (inits.size() == 1 && inits.get(0) instanceof VariableTree v && v.getInitializer() != null) {
      Element e = elemOf(v);
      String name = Ctx.safe(v.getName().toString());
      if (mc.hoisted.contains(name)) name = fresh(name);
      mc.names.put(e, name);
      mc.reads.add(e);
      TypeMirror ty = cx.erase(e.asType());
      String val = exprAs(v.getInitializer(), ty);
      init = name + " := " + cx.goType(ty) + "(" + val + ")";
      if (cx.kind(ty) != Ctx.K.PRIM || ty.getKind() == TypeKind.BOOLEAN) {
        pre.append("var ").append(name).append(" ").append(cx.goType(ty)).append(" = ").append(val).append("\n");
        init = "";
      }
    } else {
      for (StatementTree i : inits) {
        pre.append(stmt(i)).append("\n");
        if (i instanceof VariableTree v) mc.reads.add(elemOf(v));
      }
    }
    String lab = enter("loop");
    String c = t.getCondition() == null ? "" : cond(t.getCondition());
    List<String> held = holdPre();
    List<? extends ExpressionStatementTree> ups = t.getUpdate();
    String code;
    if (ups.size() <= 1) {
      String post = ups.isEmpty() ? "" : stmtExpr(ups.get(0).getExpression());
      if (post.contains("\n") || post.startsWith("_ =")) {
        ups = new ArrayList<>(ups);
        code = generalFor(init, c, ups, t.getStatement());
      } else {
        String head = init.isEmpty() && post.isEmpty() ? (c.isEmpty() ? "for " : "for " + c + " ") : "for " + init + "; " + c + "; " + post + " ";
        code = head + braced(t.getStatement());
      }
    } else {
      code = generalFor(init, c, ups, t.getStatement());
    }
    popScope();
    restorePre(held);
    code = lab + code;
    code = withPreInline(code);
    if (pre.length() > 0) return "{\n" + indent(pre + code + "\n", "\t") + "}";
    return code;
  }

  String withPreInline(String code) {
    return code;
  }

  String generalFor(String init, String c, List<? extends ExpressionStatementTree> ups, StatementTree body) {
    String first = fresh("first");
    StringBuilder b = new StringBuilder();
    b.append("if !").append(first).append(" {\n");
    for (ExpressionStatementTree u : ups) b.append("\t").append(stmtExpr(u.getExpression()).replace("\n", "\n\t")).append("\n");
    b.append("}\n").append(first).append(" = false\n");
    if (!c.isEmpty()) b.append("if !").append(paren(c)).append(" {\n\tbreak\n}\n");
    b.append(body instanceof BlockTree bt ? blockCode(bt) : stmt(body) + "\n");
    String head = init.isEmpty() ? first + " := true" : init.replace(" := ", ", " + first + " := ") + ", true";
    return "for " + head + "; ; {\n" + indent(b.toString(), "\t") + "}";
  }

  String forEach(EnhancedForLoopTree t) {
    VariableTree v = t.getVariable();
    Element e = elemOf(v);
    String name = Ctx.safe(v.getName().toString());
    if (mc.hoisted.contains(name)) name = fresh(name);
    mc.names.put(e, name);
    TypeMirror vt = cx.erase(e.asType());
    ExpressionTree ex = t.getExpression();
    TypeMirror et = cx.erase(typeOf(ex));
    String lab;
    String head, bind = "";
    if (et.getKind() == TypeKind.ARRAY) {
      TypeMirror comp = cx.erase(((ArrayType) et).getComponentType());
      String src = expr(ex);
      lab = enter("loop");
      if (cx.goType(comp).equals(cx.goType(vt))) {
        head = "for _, " + name + " := range " + src + " ";
      } else {
        String tmp = fresh("e");
        head = "for _, " + tmp + " := range " + src + " ";
        bind = "var " + name + " " + cx.goType(vt) + " = " + cx.coerce(tmp, comp, vt) + "\n";
      }
    } else {
      TypeMirror iterable = cx.elems.getTypeElement("java.lang.Iterable").asType();
      String it = fresh("it");
      String src = cx.coerce(expr(ex), et, iterable);
      TypeElement itE = Ctx.typeElement(iterable), iterE = cx.elems.getTypeElement("java.util.Iterator");
      cx.noteMethodOn(itE, "iterator", List.of(), iterE.asType());
      cx.noteMethodOn(iterE, "hasNext", List.of(), prim(TypeKind.BOOLEAN));
      cx.noteMethodOn(iterE, "next", List.of(), objectType());
      lab = enter("loop");
      head = "for " + it + " := " + src + ".iterator(); " + it + ".hasNext(); ";
      bind = "var " + name + " " + cx.goType(vt) + " = " + cx.coerce(it + ".next()", objectType(), vt) + "\n";
    }
    List<String> held = holdPre();
    String body = bind + useMarker(e) + "\n" + (t.getStatement() instanceof BlockTree b ? blockCode(b) : stmt(t.getStatement()) + "\n");
    popScope();
    restorePre(held);
    return lab + head + "{\n" + indent(body, "\t") + "}";
  }

  // ---- switch

  boolean canComplete(StatementTree s) {
    if (s == null) return true;
    switch (s.getKind()) {
      case BREAK, RETURN, THROW, CONTINUE, YIELD:
        return false;
      case BLOCK: {
        List<? extends StatementTree> l = ((BlockTree) s).getStatements();
        return l.isEmpty() || canComplete(l.get(l.size() - 1));
      }
      case IF: {
        IfTree t = (IfTree) s;
        return t.getElseStatement() == null || canComplete(t.getThenStatement()) || canComplete(t.getElseStatement());
      }
      case LABELED_STATEMENT:
        return canComplete(((LabeledStatementTree) s).getStatement());
      case SYNCHRONIZED:
        return canComplete(((SynchronizedTree) s).getBlock());
      case WHILE_LOOP:
        return !isTrue(((WhileLoopTree) s).getCondition());
      case TRY: {
        TryTree t = (TryTree) s;
        if (t.getFinallyBlock() != null && !canComplete(t.getFinallyBlock())) return false;
        if (canComplete(t.getBlock())) return true;
        for (CatchTree c : t.getCatches()) if (canComplete(c.getBlock())) return true;
        return false;
      }
      default:
        return true;
    }
  }

  String caseLabel(ExpressionTree l, TypeMirror selType) {
    Element e = elemOf(l);
    if (e != null && e.getKind() == ElementKind.ENUM_CONSTANT) {
      return cx.coerce(staticField((VariableElement) e), e.asType(), selType);
    }
    if (l instanceof IdentifierTree id && cx.kind(selType) == Ctx.K.CLS) {
      // An unqualified enum constant that javac did not attribute as an expression.
      TypeElement en = Ctx.typeElement(cx.erase(selType));
      for (Element m : en.getEnclosedElements()) {
        if (m.getKind() == ElementKind.ENUM_CONSTANT && m.getSimpleName().equals(id.getName())) return staticField((VariableElement) m);
      }
    }
    return exprAs(l, selType);
  }

  String switchBody(TypeMirror selType, String sel, List<? extends CaseTree> cases, boolean isExpr) {
    boolean patterns = false;
    for (CaseTree c : cases) {
      for (CaseLabelTree l : c.getLabels()) if (l instanceof PatternCaseLabelTree) patterns = true;
    }
    StringBuilder sb = new StringBuilder();
    String subject = sel;
    if (patterns) {
      sb.append("{\nsel_ := ").append(sel).append("\n_ = sel_\n");
      subject = "";
    }
    sb.append("switch ").append(subject).append(subject.isEmpty() ? "{\n" : " {\n");
    List<String> pendingLabels = new ArrayList<>();
    boolean pendingDefault = false;
    for (int ci = 0; ci < cases.size(); ci++) {
      CaseTree c = cases.get(ci);
      String bindCode = "";
      for (CaseLabelTree l : c.getLabels()) {
        if (l instanceof DefaultCaseLabelTree) {
          pendingDefault = true;
        } else if (l instanceof ConstantCaseLabelTree cl) {
          ExpressionTree ce = cl.getConstantExpression();
          if (ce.getKind() == Tree.Kind.NULL_LITERAL) pendingLabels.add(patterns ? "sel_ == nil" : "nil");
          else pendingLabels.add(patterns ? "sel_ == " + caseLabel(ce, selType) : caseLabel(ce, selType));
        } else if (l instanceof PatternCaseLabelTree pl) {
          if (!(pl.getPattern() instanceof BindingPatternTree bp)) throw unsupported(c, "case pattern");
          Element ve = elemOf(bp.getVariable());
          TypeMirror vt = cx.erase(ve.asType());
          String any = cx.coerce("sel_", selType, objectType());
          String check = isCheck(any, vt);
          if (c.getGuard() != null) {
            String name = declareHoisted(ve);
            check = "func() bool { if !" + check + " { return false }; " + name + " = " + cx.coerce(any, objectType(), vt) + "; return " + cond(c.getGuard()) + " }()";
          } else {
            String name = Ctx.safe(ve.getSimpleName().toString());
            mc.names.put(ve, name);
            bindCode = "var " + name + " " + cx.goType(vt) + " = " + cx.coerce(any, objectType(), vt) + "\n" + useMarker(ve) + "\n";
          }
          pendingLabels.add(check);
        }
      }
      boolean arrow = c.getCaseKind() == CaseTree.CaseKind.RULE;
      List<? extends StatementTree> body = arrow ? null : c.getStatements();
      if (!arrow && body.isEmpty() && ci < cases.size() - 1) continue; // shares the next case's body
      if (pendingDefault && !pendingLabels.isEmpty()) {
        // "case X: default:" — X is covered by default.
        pendingLabels.clear();
      }
      sb.append(pendingDefault ? "default:\n" : "case " + String.join(", ", pendingLabels) + ":\n");
      pendingLabels.clear();
      pendingDefault = false;
      StringBuilder b = new StringBuilder(bindCode);
      if (arrow) {
        Tree rb = c.getBody();
        if (rb instanceof ExpressionTree re) {
          if (isExpr) b.append(withPre(returnCode(exprAs(re, mc.ret)))).append("\n");
          else b.append(withPre(stmtExpr(re))).append("\n");
        } else if (rb instanceof BlockTree bt) {
          b.append(blockCode(bt));
        } else {
          b.append(stmt((StatementTree) rb)).append("\n");
        }
      } else {
        List<StatementTree> l = new ArrayList<>(body);
        boolean falls = l.isEmpty() || canComplete(l.get(l.size() - 1));
        if (!l.isEmpty() && l.get(l.size() - 1) instanceof BreakTree br && br.getLabel() == null) {
          l.remove(l.size() - 1);
          falls = false;
        }
        b.append(stmts(l));
        if (falls && ci < cases.size() - 1) b.append("fallthrough\n");
      }
      sb.append(indent(b.toString(), "\t"));
    }
    sb.append("}");
    if (patterns) sb.append("\n}");
    return sb.toString();
  }

  TypeMirror selectorType(ExpressionTree sel) {
    TypeMirror t = cx.erase(typeOf(sel));
    TypeMirror u = cx.unboxed(t);
    return u != null ? u : t;
  }

  String switchStmt(SwitchTree t) {
    TypeMirror st = selectorType(t.getExpression());
    String sel = exprAs(t.getExpression(), st);
    List<String> held = holdPre();
    String pd = casePredeclarations(t.getCases());
    String lab = enter("switch");
    String code = lab + switchBody(st, sel, t.getCases(), false);
    popScope();
    restorePre(held);
    if (!pd.isEmpty()) return "{\n" + indent(pd + code + "\n", "\t") + "}";
    return code;
  }

  @Override
  String switchExpr(SwitchExpressionTree t) {
    TypeMirror ty = cx.erase(typeOf(t));
    TypeMirror st = selectorType(t.getExpression());
    String sel = exprAs(t.getExpression(), st);
    // The switch becomes a function of its own: returns and jumps start afresh.
    MethodCtx saved = mc;
    MethodCtx inner = new MethodCtx();
    inner.names = saved.names;
    inner.reads = saved.reads;
    inner.counter = saved.counter;
    inner.hoisted = saved.hoisted;
    inner.staticCtx = saved.staticCtx;
    inner.usedLabels = saved.usedLabels;
    inner.isVoid = false;
    inner.ret = ty;
    mc = inner;
    pushScope("switch", fresh("L"), null);
    String body = switchBody(st, sel, t.getCases(), true);
    popScope();
    saved.counter = inner.counter;
    mc = saved;
    return "func() " + cx.goType(ty) + " {\n" + indent(body + "\npanic(\"unreachable\")\n", "\t") + "}()";
  }

  // ---- try

  String tryStmt(TryTree t) {
    TryCtx tc = new TryCtx();
    tc.id = ++mc.counter;
    int scope = pushScope("try", "", tc);
    StringBuilder b = new StringBuilder();
    if (t.getFinallyBlock() != null) {
      tc.inHandler = true;
      b.append("defer func() {\n").append(indent(blockCode(t.getFinallyBlock()), "\t")).append("}()\n");
      tc.inHandler = false;
    }
    if (!t.getCatches().isEmpty()) {
      tc.inHandler = true;
      StringBuilder h = new StringBuilder();
      h.append("if r_ := recover(); r_ != nil {\n");
      StringBuilder hb = new StringBuilder("ex_ := catch_(r_)\n");
      for (CatchTree c : t.getCatches()) {
        VariableTree p = c.getParameter();
        Element pe = elemOf(p);
        String name = Ctx.safe(p.getName().toString());
        if (mc.hoisted.contains(name)) name = fresh(name);
        mc.names.put(pe, name);
        List<TypeMirror> alts = new ArrayList<>();
        if (p.getType() instanceof UnionTypeTree u) {
          for (Tree alt : u.getTypeAlternatives()) alts.add(cx.erase(cx.trees.getTypeMirror(cx.trees.getPath(cu, alt))));
        } else {
          alts.add(cx.erase(pe.asType()));
        }
        TypeMirror vt = cx.erase(pe.asType());
        if (vt.getKind() == TypeKind.UNION) vt = cx.erase(((UnionType) vt).getAlternatives().get(0));
        if (alts.size() > 1) vt = lub(alts);
        List<String> checks = new ArrayList<>();
        for (TypeMirror a : alts) checks.add(isCheck("ex_", a));
        hb.append("if ").append(String.join(" || ", checks)).append(" {\n");
        String cb = "var " + name + " " + cx.goType(vt) + " = " + cx.coerce("ex_", objectType(), vt) + "\n" + useMarker(pe) + "\n" + blockCode(c.getBlock()) + "return\n";
        hb.append(indent(cb, "\t")).append("}\n");
      }
      hb.append("panic(r_)\n");
      h.append(indent(hb.toString(), "\t")).append("}\n");
      b.append("defer func() {\n").append(indent(h.toString(), "\t")).append("}()\n");
      tc.inHandler = false;
    }
    for (Tree r : t.getResources()) {
      if (r instanceof VariableTree rv) {
        b.append(localVar(rv)).append("\n");
        Element re = elemOf(rv);
        mc.reads.add(re);
        b.append("defer closeResource_(").append(cx.coerce(mc.names.get(re), re.asType(), objectType())).append(")\n");
      } else {
        b.append("defer closeResource_(").append(asAny((ExpressionTree) r)).append(")\n");
      }
    }
    b.append(blockCode(t.getBlock()));
    b.append("return 0\n");
    popScope();
    StringBuilder out = new StringBuilder();
    if (tc.usesReturn && mc.ret != null) out.append("var ret_").append(tc.id).append(" ").append(cx.goType(mc.ret)).append("\n");
    boolean needCode = tc.usesReturn || !tc.exits.isEmpty();
    out.append(needCode ? "code_" + tc.id + " := " : "_ = ");
    out.append("func() (code_ int) {\n").append(indent(b.toString(), "\t")).append("}()");
    if (tc.usesReturn) {
      String v = mc.ret != null ? "ret_" + tc.id : null;
      out.append("\nif code_").append(tc.id).append(" == 1 {\n").append(indent(returnCode(v), "\t")).append("\n}");
    }
    for (int i = 0; i < tc.exits.size(); i++) {
      String key = tc.exits.get(i);
      int target = Integer.parseInt(key.substring(1));
      out.append("\nif code_").append(tc.id).append(" == ").append(2 + i).append(" {\n").append(indent(jumpFromOutside(key.charAt(0) == 'b', target), "\t")).append("\n}");
    }
    return out.toString();
  }

  /** A jump emitted right after a try closure: an unlabelled spelling is only right if nothing lies between. */
  String jumpFromOutside(boolean isBreak, int target) {
    for (int i = mc.scopes.size() - 1; i > target; i--) {
      if (mc.scopeKinds.get(i).equals("try")) return jump(isBreak, target);
    }
    if (target == mc.scopes.size() - 1) return isBreak ? "break" : "continue";
    String l = mc.scopeLabels.get(target);
    mc.usedLabels.add(l);
    return (isBreak ? "break " : "continue ") + l;
  }

  TypeMirror lub(List<TypeMirror> alts) {
    // The nearest common superclass of the alternatives.
    TypeElement a = Ctx.typeElement(alts.get(0));
    for (TypeElement c = a; c != null; c = Ctx.typeElement(c.getSuperclass())) {
      boolean all = true;
      for (TypeMirror t : alts) if (!cx.types.isSubtype(t, cx.erase(c.asType()))) all = false;
      if (all) return cx.erase(c.asType());
    }
    return cx.elems.getTypeElement("java.lang.Throwable").asType();
  }
}
