package j2go;

import java.io.IOException;
import java.nio.file.*;
import java.util.*;
import java.util.regex.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Writes placeholder Go declarations for every library type and member the translation uses and the
 * handwritten runtime (the non-generated .go files of the output directory) does not define yet.
 * A placeholder panics when called, so the package always builds and a run names what it lacks.
 */
final class Stubs {
  final Ctx cx;
  final Set<String> types = new HashSet<>(), funcs = new HashSet<>(), methods = new HashSet<>(), vars = new HashSet<>();

  Stubs(Ctx cx, Path dir) throws IOException {
    this.cx = cx;
    Pattern method = Pattern.compile("^func \\(\\w+ \\*?(\\w+)(?:\\[[^\\]]*\\])?\\) (\\w+)\\(");
    Pattern func = Pattern.compile("^func (\\w+)[\\[(]");
    Pattern type = Pattern.compile("^type (\\w+)\\b");
    Pattern var = Pattern.compile("^(?:var|const) (\\w+)\\b");
    Pattern inBlock = Pattern.compile("^\\t(\\w+)\\b.*");
    try (var list = Files.list(dir)) {
      for (Path p : (Iterable<Path>) list::iterator) {
        String fn = p.getFileName().toString();
        if (!fn.endsWith(".go") || fn.startsWith("g_")) continue;
        boolean block = false;
        for (String line : Files.readAllLines(p)) {
          Matcher m;
          if (line.startsWith("var (") || line.startsWith("const (")) {
            block = true;
            continue;
          }
          if (block) {
            if (line.startsWith(")")) block = false;
            else if ((m = inBlock.matcher(line)).matches()) vars.add(m.group(1));
            continue;
          }
          if ((m = method.matcher(line)).find()) methods.add(m.group(1) + "." + m.group(2));
          else if ((m = func.matcher(line)).find()) funcs.add(m.group(1));
          else if ((m = type.matcher(line)).find()) types.add(m.group(1));
          else if ((m = var.matcher(line)).find()) vars.add(m.group(1));
        }
      }
    }
  }

  final Set<String> generatedInits = new HashSet<>();

  boolean isThrowable(TypeElement te) {
    return cx.types.isSubtype(cx.erase(te.asType()), cx.elems.getTypeElement("java.lang.Throwable").asType());
  }

  static String named(String sig) {
    // "(T0, T1) R" -> "(p0 T0, p1 T1) R"
    int close = sig.indexOf(')');
    String ps = sig.substring(1, close);
    if (ps.isEmpty()) return sig;
    String[] parts = ps.split(", ");
    StringBuilder sb = new StringBuilder("(");
    for (int i = 0; i < parts.length; i++) sb.append(i > 0 ? ", " : "").append("p").append(i).append(" ").append(parts[i]);
    return sb.append(sig.substring(close)).toString();
  }

  static String argsOf(String sig) {
    int close = sig.indexOf(')');
    String ps = sig.substring(1, close);
    if (ps.isEmpty()) return "";
    int n = ps.split(", ").length;
    StringBuilder sb = new StringBuilder();
    for (int i = 0; i < n; i++) sb.append(i > 0 ? ", " : "").append("p").append(i);
    return sb.toString();
  }

  /** The functional interface of FnTypes.LIBS that te extends (Guava's Predicate: java.util's), or null. */
  TypeElement fnSuper(TypeElement te) {
    for (TypeMirror st : te.getInterfaces()) if (cx.fn.is(st)) return Ctx.typeElement(st);
    return null;
  }

  /**
   * Rule "function types": a library functional interface is a named func type with the
   * interface's method (and box_, the function as an Object: nil for null). One that extends
   * another is an alias of it, and its method one more method of that type.
   */
  void fnTypes(StringBuilder sb) {
    if (!FnTypes.ON) {
      fnErased(sb);
      return;
    }
    List<TypeElement> order = new ArrayList<>(cx.fn.libs);
    order.sort(Comparator.comparing(cx::className));
    Map<TypeElement, Set<String>> have = new HashMap<>();
    for (TypeElement te : order) {
      String n = cx.className(te);
      if (types.contains(n)) continue;
      TypeElement sup = fnSuper(te);
      // (any Go type is a type argument: a func type compares nothing)
      StringBuilder tb = new StringBuilder("[");
      for (TypeParameterElement tp : te.getTypeParameters()) tb.append(tb.length() > 1 ? ", " : "").append(cx.tvName(tp)).append(" any");
      String tpd = tb.append("]").toString();
      if (sup != null) {
        sb.append("type ").append(n).append(tpd).append(" = ").append(cx.goType(cx.superView(te.asType(), sup))).append("\n\n");
        continue;
      }
      ExecutableElement m = cx.functionalMethod(te);
      sb.append("type ").append(n).append(tpd).append(" func").append(cx.goSig(cx.groupParams(m, te.asType()), cx.groupReturn(m, te.asType()))).append("\n\n");
      String recv = "func (f " + n + cx.typeParamsUse(te.getTypeParameters()) + ") ";
      sb.append(recv).append("box_() any {\n\treturn fnBox_(f)\n}\n\n");
    }
    for (TypeElement te : order) {
      // the interface's method, on the type that te is (itself, or the one it is an alias of)
      TypeElement top = te;
      while (fnSuper(top) != null) top = fnSuper(top);
      ExecutableElement m = cx.functionalMethod(te), tm = cx.functionalMethod(top);
      String name = cx.methodName(m), tn = cx.className(top);
      if (!have.computeIfAbsent(top, k -> new HashSet<>()).add(name) || methods.contains(tn + "." + name)) continue;
      String sig = cx.goSig(cx.groupParams(tm, top.asType()), cx.groupReturn(tm, top.asType()));
      boolean isVoid = cx.groupReturn(tm).getKind() == TypeKind.VOID;
      sb.append("func (f ").append(tn).append(cx.typeParamsUse(top.getTypeParameters())).append(") ").append(name).append(named(sig)).append(" {\n\t")
          .append(isVoid ? "" : "return ").append("f(").append(argsOf(sig)).append(")\n}\n\n");
    }
    // For the runtime, which is written for both translations: X_e, the interface at `any`
    // (the type erased runtime code takes and gives), and X_of_, a Go func as an X_e.
    for (TypeElement te : order) {
      String n = cx.className(te);
      TypeMirror raw = cx.fn.raw(te.asType());
      ExecutableElement m = cx.functionalMethod(te);
      sb.append("type ").append(n).append("_e = ").append(cx.goType(raw)).append("\n\n");
      sb.append("func ").append(n).append("_of_(f func").append(cx.goSig(cx.groupParams(m, raw), cx.groupReturn(m, raw))).append(") ").append(n).append("_e {\n\treturn f\n}\n\n");
    }
  }

  /** The functional interfaces of FnTypes.LIBS without the rule: every one is declared, with its adapter. */
  void fnErasedNote() {
    if (FnTypes.ON) return;
    for (String qn : FnTypes.LIBS) {
      TypeElement te = cx.elems.getTypeElement(qn);
      if (te == null) continue;
      cx.noteType(te);
      ExecutableElement m = cx.functionalMethod(te);
      if (cx.extAdapters.add(te)) cx.noteMethodOn(cx.ownerOf(cx.root(m)), cx.methodName(m), cx.groupParams(m), cx.groupReturn(m));
    }
  }

  /** X_e and X_of_ (fnTypes) without the rule: the erased interface, and its adapter object. */
  void fnErased(StringBuilder sb) {
    for (String qn : FnTypes.LIBS) {
      TypeElement te = cx.elems.getTypeElement(qn);
      if (te == null) continue;
      String n = cx.className(te);
      ExecutableElement m = cx.functionalMethod(te);
      sb.append("type ").append(n).append("_e = ").append(n).append("\n\n");
      sb.append("func ").append(n).append("_of_(f func").append(cx.goSig(cx.groupParams(m), cx.groupReturn(m))).append(") ").append(n).append("_e {\n\treturn &").append(n).append("_fn{f}\n}\n\n");
    }
  }

  String generate() {
    fnErasedNote();
    StringBuilder sb = new StringBuilder("// Code generated by j2go: placeholders for library members not yet in the runtime. DO NOT EDIT.\n\npackage closure\n\n");
    List<TypeElement> ts = new ArrayList<>(cx.extTypes);
    ts.sort(Comparator.comparing(cx::className));
    int nTypes = 0, nMethods = 0, nFuncs = 0;
    for (TypeElement te : ts) {
      Ctx.K k = cx.kind(te.asType());
      if (k != Ctx.K.IFACE && k != Ctx.K.CLS) continue;
      String n = cx.className(te);
      TypeMirror tt = cx.erase(te.asType());
      if (k == Ctx.K.IFACE) {
        if (types.contains(n) || cx.fn.isLib(te)) continue;
        nTypes++;
        sb.append("type ").append(n).append(" interface {\n");
        for (TypeMirror st : te.getInterfaces()) {
          if (cx.kind(st) == Ctx.K.IFACE && !cx.fn.is(st)) sb.append("\t").append(cx.className(Ctx.typeElement(st))).append("\n");
        }
        for (var e : cx.extMethods.entrySet()) {
          if (e.getValue().contains(te)) sb.append("\t").append(e.getKey()).append("\n");
        }
        sb.append("}\n\n");
        continue;
      }
      TypeElement sup = cx.superclass(te);
      boolean ext = cx.isExtendable(te);
      if (!types.contains(n)) {
        nTypes++;
        sb.append("type ").append(n).append(" struct {\n");
        if (sup != null) sb.append("\t").append(cx.className(sup)).append("\n");
        if (ext && (sup == null || !cx.isExtendable(sup))) sb.append("\tself_ any\n");
        TreeMap<String, String> fs = cx.extFields.get(te);
        if (fs != null) fs.forEach((fn, ft) -> sb.append("\t").append(fn).append(" ").append(ft).append("\n"));
        sb.append("}\n\n");
      }
      if (ext) {
        if (!methods.contains(n + ".as_" + n)) sb.append("func (this *").append(n).append(") as_").append(n).append("() *").append(n).append(" { return this }\n\n");
        if (!funcs.contains("to_" + n)) {
          sb.append("func to_").append(n).append("(v any) *").append(n).append(" {\n\tif v == nil {\n\t\treturn nil\n\t}\n\treturn v.(interface{ as_")
              .append(n).append("() *").append(n).append(" }).as_").append(n).append("()\n}\n\n");
        }
        if (!funcs.contains("is_" + n)) {
          sb.append("func is_").append(n).append("(v any) bool {\n\t_, ok := v.(interface{ as_").append(n).append("() *").append(n).append(" })\n\treturn ok\n}\n\n");
        }
        if (!methods.contains(n + ".self")) {
          sb.append("func (this *").append(n).append(") self() any {\n\tif this == nil {\n\t\treturn nil\n\t}\n\treturn this.self_\n}\n\n");
        }
      }
      for (TypeElement a = sup; a != null && !types.contains(n) && a == sup; a = cx.superclass(a)) {
        String an = cx.className(a);
        if (methods.contains(n + ".up_" + an)) continue;
        sb.append("func (this *").append(n).append(") up_").append(an).append("() *").append(an)
            .append(" {\n\tif this == nil {\n\t\treturn nil\n\t}\n\treturn &this.").append(an).append("\n}\n\n");
      }
      if (isThrowable(te) && !n.equals("Throwable")) {
        for (Element ce : te.getEnclosedElements()) {
          if (ce.getKind() != ElementKind.CONSTRUCTOR || ce.getModifiers().contains(Modifier.PRIVATE)) continue;
          ExecutableElement c = (ExecutableElement) ce;
          String shape = cx.sigToken(c);
          if (!(shape.isEmpty() || shape.equals("_String") || shape.equals("_String_Throwable") || shape.equals("_Throwable") || shape.equals("_Object"))) continue;
          if (sup == null) continue;
          String init = cx.ctorName(c, "init_"), mk = cx.ctorName(c, "new_");
          String decl = shape.isEmpty() ? "" : shape.equals("_String") ? "p0 string" : shape.equals("_Throwable") ? "p0 *Throwable" : shape.equals("_Object") ? "p0 any" : "p0 string, p1 *Throwable";
          String fwd = shape.isEmpty() ? "" : shape.equals("_String_Throwable") ? "p0, p1" : "p0";
          // The superclass may lack the same shape (AssertionError(Object)): fall back to the message form.
          String supShape = shape.equals("_Object") ? "_String" : shape;
          String supArgs = shape.equals("_Object") ? "objstr_(p0)" : fwd;
          String sn = cx.className(sup);
          String supInit = "init_" + sn + (supShape.isEmpty() ? "_0" : supShape);
          if (!methods.contains(n + "." + init)) {
            generatedInits.add(n + "." + init);
            sb.append("func (this *").append(n).append(") ").append(init).append("(").append(decl).append(") {\n\tthis.").append(sn).append(".")
                .append(supInit).append("(").append(supArgs).append(")\n}\n\n");
          }
          if (!funcs.contains(mk)) {
            funcs.add(mk);
            sb.append("func ").append(mk).append("(").append(decl).append(") *").append(n).append(" {\n\tthis := &").append(n)
                .append("{}\n\tthis.self_ = this\n\tthis.").append(init).append("(").append(fwd).append(")\n\treturn this\n}\n\n");
          }
        }
      }
      for (var e : cx.extMethods.entrySet()) {
        String key = e.getKey();
        String name = key.substring(0, key.indexOf('('));
        if (methods.contains(n + "." + name) || generatedInits.contains(n + "." + name)) continue;
        // A handwritten type decides its own methods (it may get them from an embedded struct).
        if (types.contains(n)) continue;
        boolean mine = false, inherited = false;
        for (TypeElement o : e.getValue()) {
          TypeMirror ot = cx.erase(o.asType());
          if (cx.types.isSubtype(tt, ot)) mine = true;
          if (sup != null && cx.types.isSubtype(cx.erase(sup.asType()), ot)) inherited = true;
        }
        if (!mine || inherited) continue;
        nMethods++;
        sb.append("func (this *").append(n).append(") ").append(name).append(named(key.substring(name.length())))
            .append(" {\n\tpanic(\"j2go stub: ").append(n).append(".").append(name).append("\")\n}\n\n");
      }
    }
    fnTypes(sb);
    for (TypeElement fi : cx.extAdapters) {
      String n = cx.className(fi) + "_fn";
      if (types.contains(n)) continue;
      ExecutableElement m = cx.functionalMethod(fi);
      String sig = cx.goSig(cx.groupParams(m), cx.groupReturn(m));
      boolean isVoid = cx.groupReturn(m).getKind() == TypeKind.VOID;
      sb.append("type ").append(n).append(" struct {\n\tf func").append(sig).append("\n}\n\n");
      sb.append("func (this *").append(n).append(") ").append(cx.methodName(m)).append(named(sig)).append(" {\n\t")
          .append(isVoid ? "" : "return ").append("this.f(").append(argsOf(sig)).append(")\n}\n\n");
      for (ExecutableElement dm : cx.extDefaultOverrides(fi)) {
        if (cx.root(dm).equals(cx.root(m))) continue;
        String dsig = cx.goSig(cx.groupParams(dm), cx.groupReturn(dm));
        sb.append("func (this *").append(n).append(") ").append(cx.methodName(dm)).append(named(dsig)).append(" {\n\t")
            .append(cx.defaultBridgeBody(dm, cx.className(cx.ownerOf(dm)) + "(this)")).append("\n}\n\n");
      }
    }
    TypeMirror object = cx.elems.getTypeElement("java.lang.Object").asType();
    for (var ce : cx.extConcreteAdapters.entrySet()) {
      // A typed adapter of one instantiation: the lambda is a func of its Java types; the erased
      // slots of the functional method meet it with the conversions of those types (performance
      // rule: a generic adapter converting through unboxT_/boxT_ costs a dictionary lookup and an
      // uncached interface assertion per call).
      String n = ce.getKey();
      TypeElement fi = (TypeElement) ce.getValue()[0];
      @SuppressWarnings("unchecked")
      List<TypeMirror> fts = (List<TypeMirror>) ce.getValue()[1];
      TypeMirror fr = (TypeMirror) ce.getValue()[2];
      ExecutableElement m = cx.functionalMethod(fi);
      List<TypeMirror> ps = cx.groupParams(m);
      TypeMirror ret = cx.groupReturn(m);
      boolean isVoid = ret.getKind() == TypeKind.VOID;
      List<String> fps = new ArrayList<>(), mps = new ArrayList<>(), call = new ArrayList<>();
      for (int i = 0; i < ps.size(); i++) {
        mps.add("p" + i + " " + cx.goType(ps.get(i)));
        fps.add(cx.goType(fts.get(i)));
        call.add(cx.goType(ps.get(i)).equals("any") ? cx.coerce("p" + i, object, fts.get(i)) : "p" + i);
      }
      sb.append("type ").append(n).append(" struct {\n\tf func(").append(String.join(", ", fps)).append(")").append(isVoid ? "" : " " + cx.goType(fr)).append("\n}\n\n");
      String inner = "this.f(" + String.join(", ", call) + ")";
      if (!isVoid && cx.goType(ret).equals("any")) inner = cx.coerce(inner, fr, object);
      sb.append("func (this *").append(n).append(") ").append(cx.methodName(m)).append("(").append(String.join(", ", mps)).append(")")
          .append(isVoid ? "" : " " + cx.goType(ret)).append(" {\n\t").append(isVoid ? inner : "return " + inner).append("\n}\n\n");
      for (ExecutableElement dm : cx.extDefaultOverrides(fi)) {
        if (cx.root(dm).equals(cx.root(m))) continue;
        String dsig = cx.goSig(cx.groupParams(dm), cx.groupReturn(dm));
        sb.append("func (this *").append(n).append(") ").append(cx.methodName(dm)).append(named(dsig)).append(" {\n\t")
            .append(cx.defaultBridgeBody(dm, cx.className(cx.ownerOf(dm)) + "(this)")).append("\n}\n\n");
      }
    }
    for (TypeElement fi : cx.extTypedAdapters) {
      // A typed adapter: the lambda is a func of its Java types; the erased slots of the
      // functional method meet it through unboxT_ and boxT_ (rt_generic.go).
      String n = cx.className(fi) + "_tfn";
      if (types.contains(n)) continue;
      ExecutableElement m = cx.functionalMethod(fi);
      List<TypeMirror> ps = cx.groupParams(m);
      TypeMirror ret = cx.groupReturn(m);
      boolean isVoid = ret.getKind() == TypeKind.VOID, retAny = !isVoid && cx.goType(ret).equals("any");
      List<String> tps = new ArrayList<>(), fps = new ArrayList<>(), mps = new ArrayList<>(), call = new ArrayList<>();
      for (int i = 0; i < ps.size(); i++) {
        String g = cx.goType(ps.get(i));
        mps.add("p" + i + " " + g);
        if (g.equals("any")) {
          tps.add("P" + i);
          fps.add("P" + i);
          call.add("unboxT_[P" + i + "](p" + i + ")");
        } else {
          fps.add(g);
          call.add("p" + i);
        }
      }
      if (retAny) tps.add("R");
      String fret = isVoid ? "" : " " + (retAny ? "R" : cx.goType(ret));
      String tpd = tps.isEmpty() ? "" : "[" + String.join(" any, ", tps) + " any]", tpu = tps.isEmpty() ? "" : "[" + String.join(", ", tps) + "]";
      sb.append("type ").append(n).append(tpd).append(" struct {\n\tf func(").append(String.join(", ", fps)).append(")").append(fret).append("\n}\n\n");
      String inner = "this.f(" + String.join(", ", call) + ")";
      sb.append("func (this *").append(n).append(tpu).append(") ").append(cx.methodName(m)).append("(").append(String.join(", ", mps)).append(")")
          .append(isVoid ? "" : " " + cx.goType(ret)).append(" {\n\t").append(isVoid ? inner : "return " + (retAny ? "boxT_(" + inner + ")" : inner)).append("\n}\n\n");
      for (ExecutableElement dm : cx.extDefaultOverrides(fi)) {
        if (cx.root(dm).equals(cx.root(m))) continue;
        String dsig = cx.goSig(cx.groupParams(dm), cx.groupReturn(dm));
        sb.append("func (this *").append(n).append(tpu).append(") ").append(cx.methodName(dm)).append(named(dsig)).append(" {\n\t")
            .append(cx.defaultBridgeBody(dm, cx.className(cx.ownerOf(dm)) + "(this)")).append("\n}\n\n");
      }
    }
    for (var e : cx.extFuncs.entrySet()) {
      if (funcs.contains(e.getKey())) continue;
      nFuncs++;
      sb.append("func ").append(e.getKey()).append(named(e.getValue())).append(" {\n\tpanic(\"j2go stub: ").append(e.getKey()).append("\")\n}\n\n");
    }
    for (var e : cx.extVars.entrySet()) {
      if (vars.contains(e.getKey())) continue;
      sb.append("var ").append(e.getKey()).append(" ").append(e.getValue()).append("\n");
    }
    System.err.println("stubs: types=" + nTypes + " methods=" + nMethods + " funcs=" + nFuncs);
    return sb.toString();
  }
}
