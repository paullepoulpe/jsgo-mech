package j2go;

import com.sun.source.tree.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/** Declarations: classes, interfaces, enums, records, their members and static initialization. */
final class Emitter extends StmtEmitter {
  Emitter(Ctx cx, CompilationUnitTree cu) {
    super(cx, cu);
  }

  final List<String> errors = new ArrayList<>();

  String emitUnit() {
    for (Tree t : cu.getTypeDecls()) {
      if (t instanceof ClassTree ct) classDecl(ct);
    }
    while (!pendingClasses.isEmpty()) pendingClasses.remove(0).run();
    return "package closure\n\n" + file;
  }

  @Override
  void queueLocalClass(ClassTree t) {
    ArrayDeque<ClassCtx> snapshot = new ArrayDeque<>(classStack);
    pendingClasses.add(() -> {
      ArrayDeque<ClassCtx> saved = new ArrayDeque<>(classStack);
      classStack.clear();
      classStack.addAll(snapshot);
      MethodCtx savedMc = mc;
      try {
        classDecl(t);
      } finally {
        classStack.clear();
        classStack.addAll(saved);
        mc = savedMc;
      }
    });
  }

  boolean enumLike(TypeElement te) {
    for (TypeElement c = te; c != null; c = Ctx.typeElement(c.getSuperclass())) {
      if (c.getKind() == ElementKind.ENUM) return true;
    }
    return false;
  }

  boolean needsZero(TypeElement te) {
    if (te == null || !cx.isSource(te)) return false;
    for (Element e : te.getEnclosedElements()) {
      if (e.getKind() == ElementKind.FIELD && !e.getModifiers().contains(Modifier.STATIC) && cx.kind(e.asType()) == Ctx.K.STR) return true;
    }
    return needsZero(cx.superclass(te));
  }

  void classDecl(ClassTree ct) {
    Element el = elemOf(ct);
    if (!(el instanceof TypeElement te)) return;
    if (te.getKind() == ElementKind.ANNOTATION_TYPE) return;
    ClassCtx c = new ClassCtx();
    c.te = te;
    c.tree = ct;
    c.hasOuter = cx.hasOuter(te);
    c.captures = capturesOf(te);
    c.ownLocals = ownLocals(ct);
    classStack.push(c);
    try {
      if (te.getKind().isInterface()) interfaceDecl(ct, te);
      else structDecl(ct, te);
      for (Tree m : ct.getMembers()) {
        if (m instanceof ClassTree inner) classDecl(inner);
      }
    } finally {
      classStack.pop();
    }
  }

  /** Runs one member's emission; a construct that is not supported yet becomes a panic stub. */
  void guarded(String what, Runnable r) {
    int mark = file.length();
    try {
      r.run();
    } catch (UnsupportedOperationException | ClassCastException | NullPointerException | IndexOutOfBoundsException ex) {
      file.setLength(mark);
      String msg = ex instanceof UnsupportedOperationException ? ex.getMessage() : ex + " at " + (ex.getStackTrace().length > 0 ? ex.getStackTrace()[0] : "?");
      errors.add(cu.getSourceFile().getName().replaceAll(".*/", "") + " " + what + ": " + msg);
      file.append("// J2GO-ERROR ").append(what).append(": ").append(msg.replace('\n', ' ')).append("\n\n");
    }
  }

  // ------------------------------------------------------------ interfaces

  boolean rootIsObject(ExecutableElement m) {
    return cx.ownerOf(cx.root(m)).getQualifiedName().contentEquals("java.lang.Object");
  }

  void interfaceDecl(ClassTree ct, TypeElement te) {
    String n = cx.className(te);
    StringBuilder sb = new StringBuilder("type " + n + " interface {\n");
    for (TypeMirror st : te.getInterfaces()) {
      if (cx.kind(st) == Ctx.K.IFACE) sb.append("\t").append(cx.className(Ctx.typeElement(st))).append("\n");
    }
    for (Element e : te.getEnclosedElements()) {
      if (!Ctx.isInstanceMethod(e) || e.getModifiers().contains(Modifier.PRIVATE)) continue;
      ExecutableElement m = (ExecutableElement) e;
      if (rootIsObject(m)) continue;
      sb.append("\t").append(cx.methodName(m)).append(cx.goSig(cx.groupParams(m), cx.groupReturn(m))).append("\n");
    }
    sb.append("}\n\n");
    file.append(sb);
    ExecutableElement fm = cx.functionalMethod(te);
    if (fm != null) {
      String fn = n + "_fn";
      List<TypeMirror> ps = cx.groupParams(fm);
      TypeMirror ret = cx.groupReturn(fm);
      file.append("type ").append(fn).append(" struct {\n\tf func").append(cx.goSig(ps, ret)).append("\n}\n\n");
      file.append("func (this *").append(fn).append(") ").append(cx.methodName(fm)).append(paramList(ps)).append(retSuffix(ret))
          .append(" {\n\t").append(ret.getKind() == TypeKind.VOID ? "" : "return ").append("this.f(").append(argNames(ps.size())).append(")\n}\n\n");
      // Default methods of the interface and its super-interfaces.
      Set<ExecutableElement> seen = new HashSet<>();
      List<TypeElement> ifaces = new ArrayList<>();
      ifaces.add(te);
      ifaces.addAll(cx.allSupertypes(te));
      for (TypeElement it : ifaces) {
        if (!cx.isSource(it) || !it.getKind().isInterface()) continue;
        for (Element e : it.getEnclosedElements()) {
          if (!Ctx.isInstanceMethod(e) || !((ExecutableElement) e).isDefault()) continue;
          ExecutableElement dm = (ExecutableElement) e;
          if (!seen.add(cx.root(dm)) || cx.root(dm).equals(cx.root(fm))) continue;
          List<TypeMirror> dps = cx.groupParams(dm);
          TypeMirror dret = cx.groupReturn(dm);
          file.append("func (this *").append(fn).append(") ").append(cx.methodName(dm)).append(paramList(dps)).append(retSuffix(dret))
              .append(" {\n\t").append(dret.getKind() == TypeKind.VOID ? "" : "return ").append(cx.className(it)).append("_").append(cx.methodName(dm))
              .append("_default(this").append(dps.isEmpty() ? "" : ", " + argNames(dps.size())).append(")\n}\n\n");
        }
      }
    }
    if (fm != null) {
      for (ExecutableElement dm : cx.extDefaultOverrides(te)) {
        if (cx.root(dm).equals(cx.root(fm))) continue;
        List<TypeMirror> ps = cx.groupParams(dm);
        file.append("func (this *").append(n).append("_fn) ").append(cx.methodName(dm)).append(paramList(ps)).append(retSuffix(cx.groupReturn(dm)))
            .append(" {\n\t").append(cx.defaultBridgeBody(dm, cx.className(cx.ownerOf(dm)) + "(this)")).append("\n}\n\n");
      }
    }
    for (Tree m : ct.getMembers()) {
      if (m instanceof MethodTree mt) guarded("method " + n + "." + mt.getName(), () -> method(mt, te));
    }
    statics(ct, te, List.of());
  }

  String paramList(List<TypeMirror> ps) {
    StringBuilder sb = new StringBuilder("(");
    for (int i = 0; i < ps.size(); i++) {
      if (i > 0) sb.append(", ");
      sb.append("p").append(i).append(" ").append(cx.goType(ps.get(i)));
    }
    return sb.append(")").toString();
  }

  String retSuffix(TypeMirror ret) {
    String r = cx.goType(ret);
    return r.isEmpty() ? "" : " " + r;
  }

  static String argNames(int n) {
    StringBuilder sb = new StringBuilder();
    for (int i = 0; i < n; i++) sb.append(i > 0 ? ", " : "").append("p").append(i);
    return sb.toString();
  }

  // ------------------------------------------------------------ classes

  void structDecl(ClassTree ct, TypeElement te) {
    String n = cx.className(te);
    TypeElement sup = cx.superclass(te);
    boolean handle = cx.isHandle(te);
    StringBuilder sb = new StringBuilder(handle ? "" : "type " + n + " struct {\n");
    if (sup != null) sb.append("\t").append(cx.className(sup)).append("\n");
    if (cx.declaresSelf(te)) sb.append("\tself_ any\n");
    ClassCtx c = cc();
    if (c.hasOuter) sb.append("\touter_ ").append(cx.ref(cx.enclosingClass(te))).append("\n");
    for (VariableElement cap : c.captures) {
      sb.append("\tcap_").append(Ctx.safe(cap.getSimpleName().toString())).append(" ").append(cx.goType(cap.asType())).append("\n");
    }
    List<VariableTree> enumConsts = new ArrayList<>();
    List<Tree> instanceInits = new ArrayList<>();
    for (Tree m : ct.getMembers()) {
      if (m instanceof VariableTree v) {
        Element fe = elemOf(v);
        if (fe == null) continue;
        if (fe.getKind() == ElementKind.ENUM_CONSTANT) {
          enumConsts.add(v);
        } else if (!fe.getModifiers().contains(Modifier.STATIC)) {
          if (!handle) sb.append("\t").append(cx.fieldName((VariableElement) fe)).append(" ").append(cx.goType(fe.asType())).append("\n");
          if (v.getInitializer() != null) instanceInits.add(v);
        }
      } else if (m instanceof BlockTree b && !b.isStatic()) {
        instanceInits.add(b);
      }
    }
    sb.append(handle ? handleDecl(te) : "}\n\n");
    boolean leaf = cx.isLeaf(te);
    if (!leaf) {
      sb.append("func (this *").append(n).append(") as_").append(n).append("() *").append(n).append(" { return this }\n\n");
      sb.append("func to_").append(n).append("(v any) *").append(n).append(" {\n\tif v == nil {\n\t\treturn nil\n\t}\n\treturn v.(interface{ as_")
          .append(n).append("() *").append(n).append(" }).as_").append(n).append("()\n}\n\n");
      sb.append("func is_").append(n).append("(v any) bool {\n\t_, ok := v.(interface{ as_").append(n).append("() *").append(n).append(" })\n\treturn ok\n}\n\n");
      if (cx.hasSelf(te)) {
        sb.append("func (this *").append(n).append(") self() any {\n\tif this == nil {\n\t\treturn nil\n\t}\n\treturn this.self_\n}\n\n");
      }
    }
    boolean pastLibrary = false;
    for (TypeElement a = sup; a != null && !pastLibrary; a = cx.superclass(a)) {
      // Only the nearest library superclass: the runtime's structs need not mirror the JDK's hierarchy.
      if (!cx.isSource(a)) pastLibrary = true;
      String an = cx.className(a);
      sb.append("func (this *").append(n).append(") up_").append(an).append("() *").append(an)
          .append(" {\n\tif this == nil {\n\t\treturn nil\n\t}\n\treturn &this.").append(an).append("\n}\n\n");
    }
    if (needsZero(te)) {
      sb.append(recv(te)).append("zero_").append(n).append("() {\n");
      if (needsZero(sup)) sb.append("\tthis.").append(cx.className(sup)).append(".zero_").append(cx.className(sup)).append("()\n");
      for (Element e : te.getEnclosedElements()) {
        if (e.getKind() == ElementKind.FIELD && !e.getModifiers().contains(Modifier.STATIC) && cx.kind(e.asType()) == Ctx.K.STR) {
          sb.append("\t").append(cx.fieldLoc("this", (VariableElement) e)).append(" = nullstr_\n");
        }
      }
      sb.append("}\n\n");
    }
    file.append(sb);
    boolean hasFinit = !instanceInits.isEmpty();
    if (hasFinit) guarded("field initializers of " + n, () -> finit(te, n, instanceInits));
    boolean anyCtor = false;
    for (Tree m : ct.getMembers()) {
      if (m instanceof MethodTree mt && mt.getName().contentEquals("<init>")) {
        anyCtor = true;
        guarded("constructor " + n, () -> constructor(mt, te, hasFinit));
      }
    }
    if (!anyCtor) defaultConstructor(te, hasFinit);
    for (Tree m : ct.getMembers()) {
      if (m instanceof MethodTree mt && !mt.getName().contentEquals("<init>")) {
        guarded("method " + n + "." + mt.getName(), () -> method(mt, te));
      }
    }
    stubs(te);
    bridges(te);
    extBridges(te);
    guarded("args4j table of " + n, () -> args4jTable(te));
    if (te.getKind() == ElementKind.RECORD) recordMembers(te);
    statics(ct, te, enumConsts);
  }

  /** Receiver clause of an instance method of class te. */
  String recv(TypeElement te) {
    return "func (this " + cx.ref(te) + ") ";
  }

  /**
   * The storage of a handle class: the handle type, one record type for the fields the collector need
   * not look at and one for the others, the chunk tables, the accessors and the allocator.
   */
  String handleDecl(TypeElement te) {
    String n = cx.className(te);
    List<VariableElement> ns = new ArrayList<>(), ps = new ArrayList<>();
    for (VariableElement f : Ctx.instanceFields(te)) (cx.pointerFree(f) ? ns : ps).add(f);
    // Widest first, so that the pointer-free record has no padding between fields.
    ns.sort(Comparator.comparingInt((VariableElement f) -> -storedSize(f)));
    StringBuilder sb = new StringBuilder("type " + n + " uint32\n\n");
    StringBuilder alloc = new StringBuilder();
    for (int k = 0; k < 2; k++) {
      String rec = n + (k == 0 ? "_ns_" : "_ps_"), acc = k == 0 ? "ns_" : "ps_", tab = n + "_" + acc + "chunks_";
      sb.append("type ").append(rec).append(" struct {\n");
      for (VariableElement f : k == 0 ? ns : ps) sb.append("\t").append(cx.fieldName(f)).append(" ").append(cx.storedType(f)).append("\n");
      sb.append("}\n\n");
      sb.append("var ").append(tab).append(" [handleChunks_]*[handleChunkSize_]").append(rec).append("\n\n");
      sb.append(recv(te)).append(acc).append("() *").append(rec).append(" {\n\treturn &").append(tab).append("[this>>handleShift_][uint16(this)]\n}\n\n");
      alloc.append("\t\t").append(tab).append("[h>>handleShift_] = new([handleChunkSize_]").append(rec).append(")\n");
    }
    // Chunk 0 is never allocated: handle 0 is null, and reaching for its record is a nil dereference.
    sb.append("var ").append(n).append("_count_ uint32 = handleChunkSize_\n\n");
    sb.append("func ").append(n).append("_alloc_() ").append(n).append(" {\n\th := ").append(n).append("_count_\n\tif uint16(h) == 0 {\n\t\thandleGrow_(h)\n")
        .append(alloc).append("\t}\n\t").append(n).append("_count_ = h + 1\n\treturn ").append(n).append("(h)\n}\n\n");
    sb.append(recv(te)).append("handle_() uint32 {\n\treturn uint32(this)\n}\n\n");
    sb.append(recv(te)).append("box_() any {\n\tif this == 0 {\n\t\treturn nil\n\t}\n\treturn this\n}\n\n");
    sb.append("func ").append(n).append("_of_(v any) ").append(n).append(" {\n\tif v == nil {\n\t\treturn 0\n\t}\n\treturn v.(").append(n).append(")\n}\n\n");
    // For JSGO_HANDLE_STATS: how many objects exist and how often each field is set.
    sb.append("func init() {\n\thandleStats_ = append(handleStats_, func() string {\n\t\tvar c [").append(ns.size() + ps.size()).append("]int\n")
        .append("\t\tfor h := ").append(n).append("(handleChunkSize_); uint32(h) < ").append(n).append("_count_; h++ {\n");
    StringBuilder names = new StringBuilder();
    int i = 0;
    for (List<VariableElement> l : List.of(ns, ps)) {
      for (VariableElement f : l) {
        String loc = cx.fieldLoc("h", f), gt = cx.storedType(f);
        String unset = cx.kind(f.asType()) == Ctx.K.STR ? loc + " == nullstr_ || " + loc + " == \"\"" : loc + " == " + (gt.equals("bool") ? "false" : cx.pointerFree(f) ? "0" : "nil");
        sb.append("\t\t\tif !(").append(unset).append(") {\n\t\t\t\tc[").append(i++).append("]++\n\t\t\t}\n");
        names.append(quote(f.getSimpleName() + " (" + gt + (l == ns ? ", pointer-free)" : ")"))).append(", ");
      }
    }
    sb.append("\t\t}\n\t\treturn handleStatsText_(").append(quote(n)).append(", ").append(n).append("_count_, []string{").append(names).append("}, c[:])\n\t})\n}\n\n");
    return sb.toString();
  }

  int storedSize(VariableElement f) {
    switch (cx.storedType(f)) {
      case "int64", "float64": return 8;
      case "int16", "uint16": return 2;
      case "int8", "uint8", "bool": return 1;
      default: return 4;
    }
  }

  void finit(TypeElement te, String n, List<Tree> inits) {
    mc = new MethodCtx();
    StringBuilder b = new StringBuilder();
    for (Tree t : inits) {
      if (t instanceof VariableTree v) {
        VariableElement f = (VariableElement) elemOf(v);
        TypeMirror ft = cx.erase(f.asType());
        ExpressionTree init = v.getInitializer();
        String val = init instanceof NewArrayTree na && na.getType() == null ? newArray(na, (ArrayType) ft) : exprAs(init, ft);
        b.append(withPre(cx.fieldSet("this", f, val))).append("\n");
      } else {
        b.append("{\n").append(indent(blockCode((BlockTree) t), "\t")).append("}\n");
      }
    }
    file.append(recv(te)).append("finit_").append(n).append("() {\n").append(indent(guards() + finish(b.toString()), "\t")).append("\n}\n\n");
    mc = null;
  }

  static final class Params {
    String decl; // Go parameter list
    String entry; // conversions at function entry
    String names; // argument list forwarding the parameters
  }

  /** Declares the parameters of a function; group types may differ from the declared ones. */
  Params params(List<? extends VariableTree> trees, List<TypeMirror> groupTypes) {
    Params p = new Params();
    StringBuilder d = new StringBuilder(), en = new StringBuilder(), nm = new StringBuilder();
    for (int i = 0; i < trees.size(); i++) {
      VariableTree v = trees.get(i);
      Element pe = elemOf(v);
      String name = Ctx.safe(v.getName().toString());
      mc.names.put(pe, name);
      mc.reads.add(pe);
      TypeMirror declared = cx.erase(pe.asType());
      TypeMirror group = groupTypes == null ? declared : groupTypes.get(i);
      if (i > 0) {
        d.append(", ");
        nm.append(", ");
      }
      if (cx.goType(declared).equals(cx.goType(group))) {
        d.append(name).append(" ").append(cx.goType(group));
        nm.append(name);
      } else {
        d.append(name).append("_ ").append(cx.goType(group));
        nm.append(name).append("_");
        en.append("var ").append(name).append(" ").append(cx.goType(declared)).append(" = ").append(cx.coerce(name + "_", group, declared))
            .append("\n_ = ").append(name).append("\n");
      }
    }
    p.decl = d.toString();
    p.entry = en.toString();
    p.names = nm.toString();
    return p;
  }

  String join(String... parts) {
    StringBuilder sb = new StringBuilder();
    for (String s : parts) {
      if (s == null || s.isEmpty()) continue;
      if (sb.length() > 0) sb.append(", ");
      sb.append(s);
    }
    return sb.toString();
  }

  /** Extra leading parameters of new_X: enclosing instance and captured variables. */
  String[] creationExtras(TypeElement te) {
    ClassCtx c = cc();
    StringBuilder d = new StringBuilder(), set = new StringBuilder();
    if (c.hasOuter) {
      d.append("outer_ ").append(cx.ref(cx.enclosingClass(te)));
      set.append("this.outer_ = outer_\n");
    }
    for (VariableElement cap : c.captures) {
      String cn = "cap_" + Ctx.safe(cap.getSimpleName().toString());
      if (d.length() > 0) d.append(", ");
      d.append(cn).append(" ").append(cx.goType(cap.asType()));
      set.append("this.").append(cn).append(" = ").append(cn).append("\n");
    }
    return new String[] {d.toString(), set.toString()};
  }

  String newPrologue(TypeElement te, String n) {
    StringBuilder b = new StringBuilder();
    if (cx.initGuards) {
      for (TypeElement c = te; c != null && cx.isSource(c); c = cx.superclass(c)) {
        if (cx.hasClinit(c) && c.getKind() != ElementKind.ENUM && !enumLike(c)) b.append(cx.className(c)).append("_clinit()\n");
      }
    }
    b.append(cx.isHandle(te) ? "this := " + n + "_alloc_()\n" : "this := &" + n + "{}\n");
    if (cx.hasSelf(te)) b.append("this.self_ = this\n");
    if (needsZero(te)) b.append("this.zero_").append(n).append("()\n");
    return b.toString();
  }

  void defaultConstructor(TypeElement te, boolean hasFinit) {
    String n = cx.className(te);
    boolean en = enumLike(te);
    String[] extras = creationExtras(te);
    String enumDecl = en ? "name_ string, ordinal_ int32" : "";
    String enumArgs = en ? "name_, ordinal_" : "";
    StringBuilder b = new StringBuilder();
    TypeElement sup = cx.superclass(te);
    if (sup != null) {
      if (sup.getQualifiedName().contentEquals("java.lang.Enum")) b.append("this.Enum.init_Enum(name_, ordinal_)\n");
      else b.append("this.").append(cx.className(sup)).append(".init_").append(cx.className(sup)).append("(").append(enumArgs).append(")\n");
    }
    if (hasFinit) b.append("this.finit_").append(n).append("()\n");
    file.append(recv(te)).append("init_").append(n).append("(").append(enumDecl).append(") {\n").append(indent(b.toString(), "\t")).append("}\n\n");
    if (!te.getModifiers().contains(Modifier.ABSTRACT)) {
      file.append("func new_").append(n).append("(").append(join(extras[0], enumDecl)).append(") ").append(cx.ref(te)).append(" {\n")
          .append(indent(newPrologue(te, n) + extras[1] + "this.init_" + n + "(" + enumArgs + ")\nreturn this\n", "\t")).append("}\n\n");
    }
  }

  boolean currentEnumLike;

  void constructor(MethodTree mt, TypeElement te, boolean hasFinit) {
    String n = cx.className(te);
    ExecutableElement e = (ExecutableElement) elemOf(mt);
    boolean en = enumLike(te);
    currentEnumLike = en;
    mc = new MethodCtx();
    Params p = params(mt.getParameters(), null);
    String enumDecl = en ? "name_ string, ordinal_ int32" : "";
    String enumArgs = en ? "name_, ordinal_" : "";
    StringBuilder b = new StringBuilder(p.entry);
    List<? extends StatementTree> body = mt.getBody() == null ? List.of() : mt.getBody().getStatements();
    int start = 0;
    boolean delegates = false;
    if (!body.isEmpty() && body.get(0) instanceof ExpressionStatementTree es && strip(es.getExpression()) instanceof MethodInvocationTree mi) {
      ExpressionTree sel = mi.getMethodSelect();
      String sn = sel instanceof IdentifierTree id ? id.getName().toString() : sel instanceof MemberSelectTree ms ? ms.getIdentifier().toString() : "";
      if (sn.equals("this") || sn.equals("super")) {
        delegates = sn.equals("this");
        String cc = withPre(ctorCall(mi));
        if (!cc.isEmpty()) b.append(cc).append("\n");
        start = 1;
      }
    }
    if (start == 0) {
      TypeElement sup = cx.superclass(te);
      if (sup != null) {
        if (sup.getQualifiedName().contentEquals("java.lang.Enum")) b.append("this.Enum.init_Enum(name_, ordinal_)\n");
        else b.append("this.").append(cx.className(sup)).append(".init_").append(cx.className(sup)).append("(").append(enumArgs).append(")\n");
      }
    }
    if (!delegates && hasFinit) b.append("this.finit_").append(n).append("()\n");
    if (te.getKind() == ElementKind.RECORD && (cx.elems.isCompactConstructor(e) || cx.elems.getOrigin(e) != javax.lang.model.util.Elements.Origin.EXPLICIT)) {
      // The canonical constructor's field assignments, which javac adds later.
      StringBuilder assigns = new StringBuilder();
      for (RecordComponentElement rc : te.getRecordComponents()) {
        for (Element f : te.getEnclosedElements()) {
          if (f.getKind() == ElementKind.FIELD && f.getSimpleName().equals(rc.getSimpleName())) {
            for (VariableTree pv : mt.getParameters()) {
              if (pv.getName().equals(rc.getSimpleName())) assigns.append("this.").append(cx.fieldName((VariableElement) f)).append(" = ").append(mc.names.get(elemOf(pv))).append("\n");
            }
          }
        }
      }
      b.append(stmts(body.subList(start, body.size())));
      b.append(assigns);
    } else {
      b.append(stmts(body.subList(start, body.size())));
    }
    String init = cx.ctorName(e, "init_");
    file.append(recv(te)).append(init).append("(").append(join(enumDecl, p.decl)).append(") {\n")
        .append(indent(guards() + finish(b.toString()), "\t")).append("\n}\n\n");
    if (!te.getModifiers().contains(Modifier.ABSTRACT)) {
      String[] extras = creationExtras(te);
      file.append("func ").append(cx.ctorName(e, "new_")).append("(").append(join(extras[0], enumDecl, p.decl)).append(") ").append(cx.ref(te)).append(" {\n")
          .append(indent(newPrologue(te, n) + extras[1] + "this." + init + "(" + join(enumArgs, p.names) + ")\nreturn this\n", "\t")).append("}\n\n");
    }
    mc = null;
  }

  @Override
  String ctorCall(MethodInvocationTree t) {
    ExecutableElement e = (ExecutableElement) elemOf(t);
    ExpressionTree sel = t.getMethodSelect();
    boolean isThis = sel instanceof IdentifierTree id && id.getName().contentEquals("this");
    TypeElement te = cc().te;
    String enumArgs = currentEnumLike ? "name_, ordinal_" : "";
    if (isThis) {
      return "this." + cx.ctorName(e, "init_") + "(" + join(enumArgs, argList(e, ownParams(e), args(t.getArguments()))) + ")";
    }
    TypeElement sup = cx.superclass(te);
    if (sup == null) return "";
    String sn = cx.className(sup);
    if (sup.getQualifiedName().contentEquals("java.lang.Enum")) return "this.Enum.init_Enum(name_, ordinal_)";
    StringBuilder sb = new StringBuilder();
    if (cx.isSource(sup) && cx.hasOuter(sup)) {
      String outer = sel instanceof MemberSelectTree ms ? expr(ms.getExpression()) : outerFor(sup);
      sb.append("this.").append(sn).append(".outer_ = ").append(outer).append("\n");
    }
    if (cx.isSource(sup)) {
      int k = 0;
      for (VariableElement cap : capturesOf(sup)) {
        sb.append("this.").append(sn).append(".cap_").append(Ctx.safe(cap.getSimpleName().toString())).append(" = ").append(localRef(cap)).append("\n");
        k++;
      }
    }
    List<TypeMirror> ps = ownParams(e);
    String init = cx.ctorName(e, "init_");
    if (!cx.isSource(sup)) cx.noteMethodOn(sup, init, ps, cx.types.getNoType(TypeKind.VOID));
    sb.append("this.").append(sn).append(".").append(init).append("(").append(join(cx.isSource(sup) ? enumArgs : "", argList(e, ps, args(t.getArguments())))).append(")");
    return sb.toString();
  }

  void method(MethodTree mt, TypeElement te) {
    ExecutableElement e = (ExecutableElement) elemOf(mt);
    if (e == null || mt.getBody() == null) return;
    String n = cx.className(te);
    boolean isStatic = e.getModifiers().contains(Modifier.STATIC);
    boolean iface = te.getKind().isInterface();
    mc = new MethodCtx();
    mc.staticCtx = isStatic;
    String head;
    TypeMirror ret;
    Params p;
    if (isStatic) {
      ret = cx.erase(e.getReturnType());
      p = params(mt.getParameters(), null);
      head = "func " + cx.staticMethodName(e) + "(" + p.decl + ")";
    } else if (iface) {
      boolean priv = e.getModifiers().contains(Modifier.PRIVATE);
      ret = priv ? cx.erase(e.getReturnType()) : cx.groupReturn(e);
      p = params(mt.getParameters(), priv ? null : cx.groupParams(e));
      String fn = priv ? cx.staticMethodName(e) : n + "_" + cx.methodName(e) + "_default";
      head = "func " + fn + "(" + join("this " + n, p.decl) + ")";
    } else {
      ret = cx.groupReturn(e);
      p = params(mt.getParameters(), cx.groupParams(e));
      head = recv(te) + cx.implName(e, te) + "(" + p.decl + ")";
    }
    mc.isVoid = ret.getKind() == TypeKind.VOID;
    mc.ret = mc.isVoid ? null : ret;
    StringBuilder b = new StringBuilder(p.entry);
    b.append(blockCode(mt.getBody()));
    List<? extends StatementTree> sts = mt.getBody().getStatements();
    if (!mc.isVoid && (sts.isEmpty() || !(sts.get(sts.size() - 1) instanceof ReturnTree || sts.get(sts.size() - 1) instanceof ThrowTree))) {
      b.append("panic(\"unreachable\")\n");
    }
    String own = cx.initGuards && isStatic && cx.hasClinit(te) ? n + "_clinit()\n" : "";
    file.append(head).append(retSuffix(ret)).append(" {\n").append(indent(own + guards() + finish(b.toString()), "\t")).append("\n}\n\n");
    mc = null;
  }

  void stubs(TypeElement te) {
    Set<ExecutableElement> roots = cx.virtualAt.get(te);
    if (roots == null) return;
    String n = cx.className(te);
    for (ExecutableElement r : roots) {
      String name = cx.methodName(r);
      List<TypeMirror> ps = cx.groupParams(r);
      TypeMirror ret = cx.groupReturn(r);
      String body;
      String ret0 = ret.getKind() == TypeKind.VOID ? "" : "return ";
      String iface = "interface{ " + name + "_impl" + cx.goSig(ps, ret) + " }";
      TypeElement ro = cx.ownerOf(r);
      TypeElement sup = cx.superclass(te);
      boolean libraryImpl = !cx.isSource(ro) && !ro.getKind().isInterface() && !r.getModifiers().contains(Modifier.ABSTRACT)
          && !ro.getQualifiedName().contentEquals("java.lang.Object") && sup != null;
      if (libraryImpl && cx.hasSelf(te)) {
        // The library superclass implements the method: classes that do not override it use that.
        body = "if o, ok := this.self_.(" + iface + "); ok {\n\t\t" + ret0 + "o." + name + "_impl(" + argNames(ps.size()) + ")\n"
            + (ret0.isEmpty() ? "\t\treturn\n" : "") + "\t}\n\t" + ret0 + "this." + cx.className(sup) + "." + name + "(" + argNames(ps.size()) + ")";
      } else if (cx.hasSelf(te)) {
        body = ret0 + "this.self_.(" + iface + ")." + name + "_impl(" + argNames(ps.size()) + ")";
      } else {
        body = "panic(\"abstract method " + n + "." + name + "\")";
      }
      file.append(recv(te)).append(name).append(paramList(ps)).append(retSuffix(ret)).append(" {\n\t").append(body).append("\n}\n\n");
    }
  }

  void bridges(TypeElement te) {
    List<ExecutableElement> dms = cx.defaultBridges.get(te);
    if (dms == null) return;
    String n = cx.className(te);
    for (ExecutableElement dm : dms) {
      TypeElement it = cx.ownerOf(dm);
      List<TypeMirror> ps = cx.groupParams(dm);
      TypeMirror ret = cx.groupReturn(dm);
      String self = cx.coerce("this", te.asType(), it.asType());
      file.append(recv(te)).append(cx.implName(dm, te)).append(paramList(ps)).append(retSuffix(ret)).append(" {\n\t")
          .append(ret.getKind() == TypeKind.VOID ? "" : "return ").append(cx.className(it)).append("_").append(cx.methodName(dm)).append("_default(")
          .append(join(self, argNames(ps.size()))).append(")\n}\n\n");
    }
  }

  static String annotationString(Object v) {
    return v == null ? "" : v.toString();
  }

  /**
   * For a class whose fields carry args4j @Option / @Argument annotations: a table that gives the
   * command-line parser what Java gets by reflection (annotation values, a getter and a setter).
   */
  void args4jTable(TypeElement te) {
    String n = cx.className(te);
    StringBuilder rows = new StringBuilder();
    mc = new MethodCtx();
    for (Element e : te.getEnclosedElements()) {
      if (e.getKind() != ElementKind.FIELD) continue;
      VariableElement f = (VariableElement) e;
      for (AnnotationMirror am : f.getAnnotationMirrors()) {
        String an = ((TypeElement) am.getAnnotationType().asElement()).getQualifiedName().toString();
        boolean isOption = an.equals("org.kohsuke.args4j.Option");
        if (!isOption && !an.equals("org.kohsuke.args4j.Argument")) continue;
        Map<String, Object> vals = new HashMap<>();
        cx.elems.getElementValuesWithDefaults(am).forEach((k, v) -> vals.put(k.getSimpleName().toString(), v.getValue()));
        TypeMirror ft = f.asType();
        TypeMirror elem = ft;
        boolean multi = false;
        TypeElement fte = Ctx.typeElement(cx.erase(ft));
        if (fte != null && cx.types.isSubtype(cx.erase(ft), cx.erase(cx.elems.getTypeElement("java.util.Collection").asType()))) {
          multi = true;
          List<? extends TypeMirror> targs = ((DeclaredType) ft).getTypeArguments();
          elem = targs.isEmpty() ? objectType() : targs.get(0);
        }
        String fn = "this." + cx.fieldName(f);
        StringBuilder r = new StringBuilder("{");
        r.append("field: ").append(quote(f.getSimpleName().toString())).append(", option: ").append(isOption);
        if (isOption) {
          r.append(", name: ").append(quote(annotationString(vals.get("name"))));
          StringBuilder al = new StringBuilder();
          Object aliases = vals.get("aliases");
          if (aliases instanceof List<?> l) for (Object a : l) al.append(al.length() > 0 ? ", " : "").append(quote(((AnnotationValue) a).getValue().toString()));
          r.append(", aliases: []string{").append(al).append("}");
        }
        r.append(", usage: ").append(quote(annotationString(vals.get("usage"))));
        r.append(", metaVar: ").append(quote(annotationString(vals.get("metaVar"))));
        r.append(", required: ").append(Boolean.TRUE.equals(vals.get("required")));
        r.append(", hidden: ").append(Boolean.TRUE.equals(vals.get("hidden")));
        r.append(", multi: ").append(multi);
        r.append(", typ: ").append(quote(cx.erase(elem).toString()));
        Object h = vals.get("handler");
        if (h instanceof TypeMirror ht) {
          TypeElement hte = Ctx.typeElement(ht);
          r.append(", handler: ").append(quote(hte.getQualifiedName().toString()));
          if (cx.isSource(hte)) {
            for (Element c : hte.getEnclosedElements()) {
              if (c.getKind() == ElementKind.CONSTRUCTOR && ((ExecutableElement) c).getParameters().size() == 3) {
                r.append(", newHandler: func(p *CmdLineParser, o *OptionDef, s Setter) any { return ")
                    .append(cx.coerce(cx.ctorName((ExecutableElement) c, "new_") + "(p, o, s)", hte.asType(), objectType())).append(" }");
              }
            }
          }
        }
        r.append(", get: func() any { return ").append(cx.coerce(fn, ft, objectType())).append(" }");
        r.append(", set: func(v any) { ").append(fn).append(" = ").append(cx.coerce("v", objectType(), ft)).append(" }");
        rows.append("\t\t").append(r).append("},\n");
      }
    }
    mc = null;
    if (rows.length() == 0) return;
    file.append("func (this *").append(n).append(") args4jFields_() []*args4jField_ {\n\treturn []*args4jField_{\n").append(rows).append("\t}\n}\n\n");
  }

  void extBridges(TypeElement te) {
    List<ExecutableElement> dms = cx.extBridges.get(te);
    if (dms == null) return;
    String n = cx.className(te);
    for (ExecutableElement dm : dms) {
      List<TypeMirror> ps = cx.groupParams(dm);
      TypeMirror ret = cx.groupReturn(dm);
      String self = cx.coerce("this", te.asType(), cx.ownerOf(dm).asType());
      file.append(recv(te)).append(cx.implName(dm, te)).append(paramList(ps)).append(retSuffix(ret)).append(" {\n\t")
          .append(cx.defaultBridgeBody(dm, self)).append("\n}\n\n");
    }
  }

  void recordMembers(TypeElement te) {
    String n = cx.className(te);
    List<VariableElement> fields = new ArrayList<>();
    for (RecordComponentElement rc : te.getRecordComponents()) {
      for (Element f : te.getEnclosedElements()) {
        if (f.getKind() == ElementKind.FIELD && f.getSimpleName().equals(rc.getSimpleName())) fields.add((VariableElement) f);
      }
      ExecutableElement acc = rc.getAccessor();
      if (acc != null && cx.trees.getTree(acc) == null) {
        VariableElement f = fields.get(fields.size() - 1);
        file.append("func (this *").append(n).append(") ").append(cx.implName(acc, te)).append("() ").append(cx.goType(cx.groupReturn(acc)))
            .append(" {\n\treturn ").append(cx.coerce("this." + cx.fieldName(f), f.asType(), cx.groupReturn(acc))).append("\n}\n\n");
      }
    }
    boolean hasEquals = false, hasHash = false, hasToString = false;
    for (Element e : te.getEnclosedElements()) {
      if (e.getKind() != ElementKind.METHOD || cx.trees.getTree(e) == null) continue;
      ExecutableElement m = (ExecutableElement) e;
      String s = m.getSimpleName().toString();
      if (s.equals("equals") && m.getParameters().size() == 1) hasEquals = true;
      if (s.equals("hashCode") && m.getParameters().isEmpty()) hasHash = true;
      if (s.equals("toString") && m.getParameters().isEmpty()) hasToString = true;
    }
    if (!hasEquals) {
      StringBuilder b = new StringBuilder("\to, ok := p0.(*" + n + ")\n\tif !ok || o == nil {\n\t\treturn false\n\t}\n\treturn true");
      for (VariableElement f : fields) {
        String fn = cx.fieldName(f);
        Ctx.K k = cx.kind(f.asType());
        if (k == Ctx.K.PRIM || k == Ctx.K.STR) b.append(" && this.").append(fn).append(" == o.").append(fn);
        else b.append(" && Objects_equals(").append(cx.coerce("this." + fn, f.asType(), objectType())).append(", ").append(cx.coerce("o." + fn, f.asType(), objectType())).append(")");
      }
      file.append("func (this *").append(n).append(") equals(p0 any) bool {\n").append(b).append("\n}\n\n");
    }
    if (!hasHash) {
      StringBuilder b = new StringBuilder("\tvar h int32\n");
      for (VariableElement f : fields) {
        b.append("\th = h*31 + Objects_hashCode(").append(cx.coerce("this." + cx.fieldName(f), f.asType(), objectType())).append(")\n");
      }
      file.append("func (this *").append(n).append(") hashCode() int32 {\n").append(b).append("\treturn h\n}\n\n");
    }
    if (!hasToString) {
      StringBuilder b = new StringBuilder("\treturn " + quote(te.getSimpleName() + "["));
      boolean first = true;
      for (VariableElement f : fields) {
        b.append(" + ").append(quote((first ? "" : ", ") + f.getSimpleName() + "=")).append(" + objstr_(").append(cx.coerce("this." + cx.fieldName(f), f.asType(), objectType())).append(")");
        first = false;
      }
      file.append("func (this *").append(n).append(") toString() string {\n").append(b).append(" + \"]\"\n}\n\n");
    }
  }

  // ------------------------------------------------------------ statics

  String constLiteral(Object v, TypeMirror t) {
    if (v instanceof String s) return quote(s);
    if (v instanceof Character c) return charLit(c);
    if (v instanceof Boolean || v instanceof Integer || v instanceof Long || v instanceof Short || v instanceof Byte) return v.toString();
    if (v instanceof Double d) {
      if (d.isNaN() || d.isInfinite()) return null;
      return d.toString();
    }
    if (v instanceof Float f) {
      if (f.isNaN() || f.isInfinite()) return null;
      return f.toString();
    }
    return null;
  }

  void statics(ClassTree ct, TypeElement te, List<VariableTree> enumConsts) {
    String n = cx.className(te);
    StringBuilder decls = new StringBuilder();
    StringBuilder init = new StringBuilder();
    mc = new MethodCtx();
    mc.staticCtx = true;
    int ordinal = 0;
    List<String> constNames = new ArrayList<>();
    for (VariableTree v : enumConsts) {
      VariableElement f = (VariableElement) elemOf(v);
      String fn = cx.fieldName(f);
      decls.append("var ").append(fn).append(" *").append(n).append("\n");
      constNames.add(fn);
      final int ord = ordinal++;
      int mark = init.length();
      try {
        NewClassTree nc = (NewClassTree) v.getInitializer();
        ExecutableElement ctor = (ExecutableElement) elemOf(nc);
        TypeElement made = te;
        if (nc.getClassBody() != null) {
          made = (TypeElement) elemOf(nc.getClassBody());
          queueLocalClass(nc.getClassBody());
          for (Tree m : nc.getClassBody().getMembers()) {
            if (m instanceof MethodTree mt && mt.getName().contentEquals("<init>")) ctor = (ExecutableElement) elemOf(mt);
          }
        }
        mc.touched.clear();
        String call = cx.ctorName(ctor, "new_") + "(" + join(quote(v.getName().toString()), Integer.toString(ord), argList(ctor, ownParams(ctor), args(nc.getArguments()))) + ")";
        String line = withPre(fn + " = " + cx.coerce(call, made.asType(), te.asType()));
        init.append(guards()).append(line).append("\n");
      } catch (RuntimeException ex) {
        init.setLength(mark);
        errors.add(n + " enum constant " + v.getName() + ": " + ex);
        init.append("// J2GO-ERROR enum constant ").append(v.getName()).append("\n");
      }
    }
    if (te.getKind() == ElementKind.ENUM) {
      decls.append("var ").append(n).append("_values_ []*").append(n).append("\n");
      init.append(n).append("_values_ = []*").append(n).append("{").append(String.join(", ", constNames)).append("}\n");
      init.append("registerEnum_(").append(quote(cx.erase(te.asType()).toString())).append(", enumValues_(").append(n).append("_values_))\n");
      if (cx.handleEnums.contains(te)) {
        // Fields of handle classes hold this enum as ordinal+1 (0 is null); these convert both ways.
        boolean small = Ctx.enumConstants(te) < 256;
        String ot = small ? "uint8" : "uint16";
        decls.append("var ").append(n).append("_byOrd1_ [").append(small ? 256 : 65536).append("]*").append(n).append("\n");
        init.append("for i_, v_ := range ").append(n).append("_values_ {\n\t").append(n).append("_byOrd1_[i_+1] = v_\n}\n");
        file.append("func ").append(n).append("_ord1_(v *").append(n).append(") ").append(ot).append(" {\n\tif v == nil {\n\t\treturn 0\n\t}\n\treturn ")
            .append(ot).append("(v.Enum.ordinal_ + 1)\n}\n\n");
      }
      file.append("func ").append(n).append("_values() []*").append(n).append(" {\n\treturn append([]*").append(n).append("(nil), ").append(n).append("_values_...)\n}\n\n");
      file.append("func ").append(n).append("_valueOf(name string) *").append(n).append(" {\n\tfor _, v := range ").append(n)
          .append("_values_ {\n\t\tif v.Enum.name() == name {\n\t\t\treturn v\n\t\t}\n\t}\n\tpanic(enumValueOfFail_(").append(quote(te.getSimpleName().toString())).append(", name))\n}\n\n");
    }
    for (Tree m : ct.getMembers()) {
      if (m instanceof VariableTree v) {
        Element fe = elemOf(v);
        if (fe == null || fe.getKind() == ElementKind.ENUM_CONSTANT || !fe.getModifiers().contains(Modifier.STATIC)) continue;
        VariableElement f = (VariableElement) fe;
        String fn = cx.fieldName(f);
        TypeMirror ft = cx.erase(f.asType());
        Object cv = f.getConstantValue();
        String lit = cv == null ? null : constLiteral(cv, ft);
        if (lit != null) {
          decls.append("const ").append(fn).append(" ").append(cx.goType(ft)).append(" = ").append(lit).append("\n");
          continue;
        }
        decls.append("var ").append(fn).append(" ").append(cx.goType(ft)).append("\n");
        if (v.getInitializer() != null) {
          int mark = init.length();
          try {
            ExpressionTree ie = v.getInitializer();
            mc.touched.clear();
            String val = ie instanceof NewArrayTree na && na.getType() == null ? newArray(na, (ArrayType) ft) : exprAs(ie, ft);
            String line = withPre(fn + " = " + val);
            init.append(guards()).append(line).append("\n");
          } catch (UnsupportedOperationException | ClassCastException | NullPointerException | IndexOutOfBoundsException ex) {
            init.setLength(mark);
            mc.pre.clear();
            errors.add(n + " static " + fn + ": " + ex);
            init.append("// J2GO-ERROR static ").append(fn).append("\n");
          }
        } else if (cx.kind(ft) == Ctx.K.STR) {
          init.append(fn).append(" = nullstr_\n");
        }
      } else if (m instanceof BlockTree b && b.isStatic()) {
        int mark = init.length();
        try {
          // In instrumented output each statement initializes the classes it touches just before it runs,
          // which is when Java would.
          StringBuilder bb = new StringBuilder();
          for (StatementTree st : b.getStatements()) {
            mc.touched.clear();
            String code = stmt(st);
            bb.append(guards());
            if (!code.isEmpty()) bb.append(code).append("\n");
          }
          mc.touched.clear();
          init.append("{\n").append(indent(bb.toString(), "\t")).append("}\n");
        } catch (UnsupportedOperationException | ClassCastException | NullPointerException | IndexOutOfBoundsException ex) {
          init.setLength(mark);
          errors.add(n + " static block: " + ex);
          init.append("// J2GO-ERROR static block\n");
        }
      }
    }
    if (decls.length() > 0) file.append(decls).append("\n");
    if (init.length() > 0 || cx.hasClinit(te)) {
      String fn = n + "_clinit";
      StringBuilder pro = new StringBuilder("if " + fn + "_done {\n\treturn\n}\n" + fn + "_done = true\n");
      TypeElement sup = cx.superclass(te);
      if (cx.initGuards && sup != null && cx.hasClinit(sup)) pro.append(cx.className(sup)).append("_clinit()\n");
      String post = "";
      if (cx.initGuards) pro.append("defer func() { initOrder_ = append(initOrder_, " + quote(cx.elems.getBinaryName(te).toString()) + ") }()\n");
      file.append("var ").append(fn).append("_done bool\n\n");
      mc.touched.clear();
      file.append("func ").append(fn).append("() {\n").append(indent(pro + finish(init.toString()) + "\n" + post, "\t")).append("\n}\n\n");
      cx.clinits.add(fn);
      cx.clinitOwner.put(fn, cx.elems.getBinaryName(te).toString());
    }
    mc = null;
  }
}
