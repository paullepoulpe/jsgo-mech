package j2go;

import java.io.IOException;
import java.nio.file.*;
import java.util.*;
import java.util.regex.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Conversions that need no box: rules that keep a value at its static type where the erased
 * translation went through Object (obj_ / strobj_ / box_ and a cast back). Each rule has a
 * switch J2GO_NO<RULE>=1 that turns it off, for bisecting.
 */
final class Conv {
  final Ctx cx;

  Conv(Ctx cx) {
    this.cx = cx;
  }

  static boolean off(String rule) {
    return "1".equals(System.getenv("J2GO_NO" + rule));
  }

  // ------------------------------------------------------------------ report: conversions to Object

  /** J2GO_BOXES=1 reports where values are converted to Object (Go `any`): by kind of value, and by callee for arguments. */
  static final boolean BOXES = System.getenv("J2GO_BOXES") != null;

  final TreeMap<String, Integer> boxKinds = new TreeMap<>(), boxSites = new TreeMap<>();

  /** What a value of (erased) type t is when it is converted to Object. */
  String boxKind(TypeMirror t) {
    if (cx.isTypedColl(t)) return "typed collection";
    switch (cx.kind(t)) {
      case PRIM: return "primitive";
      case STR: return "String";
      case IFACE: return "interface";
      case ARR: return "array";
      case TVAR: return "type parameter";
      case CLS: {
        TypeElement te = Ctx.typeElement(t);
        if (cx.isHandle(te)) return "handle";
        if (cx.isValueLib(te)) return "value type";
        if (te != null && te.getKind() == ElementKind.ENUM) return "enum";
        if (!cx.isSource(te)) return "library class";
        return cx.hasSelf(te) && !cx.isLeaf(te) ? "class with subclasses" : "class";
      }
      default: return "other";
    }
  }

  void noteBox(TypeMirror from) {
    if (BOXES) boxKinds.merge(boxKind(from), 1, Integer::sum);
  }

  /** An argument of static type arg passed for an Object parameter of e (or an element of its Object...). */
  void noteBoxArg(ExecutableElement e, int i, TypeMirror arg, TypeMirror target, String code) {
    if (!BOXES || arg == null || cx.kind(cx.erase(target)) != Ctx.K.ANY) return;
    TypeMirror at = cx.erase(arg);
    if (cx.kind(at) == Ctx.K.ANY || cx.kind(at) == Ctx.K.NULL) return;
    boolean va = e.isVarArgs() && i >= e.getParameters().size() - 1;
    String how = code.startsWith("strobj_(") || code.startsWith("obj_(") || code.startsWith("any(") || code.startsWith("boxT_(") || code.endsWith(".box_()") || code.endsWith(".self()") ? "" : " (no helper)";
    boxSites.merge((cx.isSource(cx.ownerOf(e)) ? "program " : "") + cx.ownerOf(e).getSimpleName() + "." + e.getSimpleName() + (va ? " [varargs]" : " #" + i) + " <- " + boxKind(at) + how, 1, Integer::sum);
  }

  void boxReport() {
    if (!BOXES) return;
    for (var en : boxKinds.entrySet()) System.err.println("boxes: kind " + en.getValue() + " " + en.getKey());
    for (var en : boxSites.entrySet()) System.err.println("boxes: arg " + en.getValue() + " " + en.getKey());
  }

  // ------------------------------------------------------------------ static upcast

  static final boolean UPCAST = !off("UPCAST");

  /** The generated functions C_as_I, by name (written to g_0casts.go). */
  final TreeMap<String, String> upcasts = new TreeMap<>();

  /** Whether code is one call of a constructor function (new_C(...)): never nil. */
  static boolean newCall(String code) {
    if (!code.startsWith("new_")) return false;
    int i = code.indexOf('(');
    return i >= 0 && code.substring(0, i).matches("\\w+") && closesAtEnd(code, i);
  }

  /** Whether the parenthesis opened at index i of code (Go source) is closed by its last character. */
  static boolean closesAtEnd(String code, int i) {
    int d = 0;
    boolean str = false;
    char q = 0;
    for (int k = i; k < code.length(); k++) {
      char ch = code.charAt(k);
      if (str) {
        if (ch == '\\' && q != '`') k++;
        else if (ch == q) str = false;
        continue;
      }
      if (ch == '"' || ch == '`' || ch == '\'') {
        str = true;
        q = ch;
        continue;
      }
      if (ch == '(') d++;
      if (ch == ')' && --d == 0) return k == code.length() - 1;
    }
    return false;
  }

  /**
   * Rule "static upcast": a class without subclasses becomes an interface that it implements (in
   * Java a conversion without a check) as Go converts a pointer to an interface: no box and no
   * type assertion. The value of such a class is the object itself (there is no most derived
   * object to find), so the interface value is the one that the assertion icast_I(obj_(v))
   * made. null stays the nil interface: `this` and a new object are never null and are converted
   * in place, I(this); any other value goes through a generated function C_as_I that tests it.
   * A class of the program must have every method of the Go interface (Ctx.implementsInGo); for
   * a class of the runtime the Go compiler checks the conversion.
   *
   * @return the conversion, or null when the rule does not apply
   */
  String upcast(String code, TypeMirror from, TypeMirror to, String gf, String gt) {
    if (!UPCAST) return null;
    TypeElement fe = Ctx.typeElement(from), te = Ctx.typeElement(to);
    if (fe == null || te == null || !gf.startsWith("*") || gf.contains("[") || cx.typedDecl(to)) return null;
    if (!cx.isLeaf(fe) || cx.isHandle(fe) || cx.isValueLib(fe)) return null;
    // Not a subtype in Java: still a conversion in Go when the class is the one Go type of several
    // library classes (the result of Builder.build is declared ImmutableCollection in the runtime
    // and is a List or a Set in Java): the assertion of that Go type to the interface cannot fail.
    if (!cx.types.isSubtype(from, to) && !(ALIAS && !cx.isSource(fe) && aliases.containsValue(gf.substring(1)))) return null;
    if (cx.isSource(fe) ? !cx.implementsInGo(fe, te) : cx.isSource(te)) return null;
    if (code.equals("this") || newCall(code)) return gt + "(" + code + ")";
    String fn = gf.substring(1) + "_as_" + gt;
    upcasts.put(fn, "func " + fn + "(v " + gf + ") " + gt + " {\n\tif v == nil {\n\t\treturn nil\n\t}\n\treturn v\n}\n");
    return fn + "(" + code + ")";
  }

  // ------------------------------------------------------------------ non-null checks

  static final boolean NONNULL = !off("NONNULL");

  /**
   * Rule "typed non-null checks": {@code <T> T checkNotNull(T, ...)} of Guava's Preconditions and
   * {@code <T> T requireNonNull(T, ...)} of java.util.Objects return their first argument. The
   * call is a Go generic function of the argument's static type, which tests the value against
   * the null of that type (nil, or the null string) and returns it: no box and no cast back.
   *
   * @return the static type of the argument when the rule applies to a call of e (the type of
   *     the parameter and of the result of the Go function), else null
   */
  /** Whether e is `<T> T checkNotNull(T, ...)` of Guava's Preconditions or `<T> T requireNonNull(T, ...)` of java.util.Objects. */
  boolean isNonNullCheck(ExecutableElement e) {
    String o = cx.ownerOf(e).getQualifiedName().toString(), n = e.getSimpleName().toString();
    if (!(o.equals("com.google.common.base.Preconditions") && n.equals("checkNotNull") || o.equals("java.util.Objects") && n.equals("requireNonNull"))) return false;
    return e.getReturnType().getKind() == TypeKind.TYPEVAR && e.getParameters().get(0).asType().getKind() == TypeKind.TYPEVAR;
  }

  TypeMirror nonNull(ExecutableElement e, TypeMirror arg) {
    if (!NONNULL || arg == null) return null;
    if (!isNonNullCheck(e)) return null;
    TypeMirror at = cx.erase(arg);
    Ctx.K k = cx.kind(at);
    if (k != Ctx.K.CLS && k != Ctx.K.IFACE && k != Ctx.K.STR || cx.isTypedColl(at) || cx.isValueLibType(at) || cx.fn.is(cx.erase(at))) return null;
    return at;
  }

  /** The suffix of the Go function of a typed non-null check: _Str tests for the null string, _T for nil. */
  String nonNullSuffix(TypeMirror at) {
    return cx.kind(at) == Ctx.K.STR ? "_Str" : "_T";
  }

  // ------------------------------------------------------------------ arguments that are never null

  static final boolean BARE = !off("BARE");

  /**
   * Rule "bare arguments": an argument for an Object parameter that cannot be null, a string
   * literal or a new object of a class without subclasses, is passed as it is. Go converts it
   * to the interface value that strobj_ / obj_ return for a value that is not null.
   *
   * @param r the argument converted to Object (the code Ctx.coerce gave)
   */
  String bare(String r, TypeMirror target) {
    if (!BARE || cx.kind(cx.erase(target)) != Ctx.K.ANY) return r;
    if (r.startsWith("strobj_(\"") && r.endsWith("\")") && r.substring(8, r.length() - 1).matches("\"([^\"\\\\]|\\\\.)*\"")) return r.substring(8, r.length() - 1);
    if (r.startsWith("obj_(new_") && r.endsWith(")") && newCall(r.substring(5, r.length() - 1))) return r.substring(5, r.length() - 1);
    return r;
  }

  // ------------------------------------------------------------------ message arguments of checks

  static final boolean MSG = !off("MSG");

  /** The checks with one message argument that have a variant taking a handle (rt_conv.go). */
  static final Set<String> HANDLE_MSG = Set.of("Preconditions_checkState_boolean_Object", "Preconditions_checkArgument_boolean_Object",
      "Preconditions_checkState_boolean_String_Object", "Preconditions_checkArgument_boolean_String_Object");

  boolean isPreconditions(ExecutableElement e) {
    return cx.ownerOf(e).getQualifiedName().contentEquals("com.google.common.base.Preconditions");
  }

  /**
   * Rule "message arguments": an Object parameter after the first of Preconditions.checkState /
   * checkArgument / checkNotNull is only formatted, and only when the check fails. A String or
   * an object of a class is passed at its own type (Go converts it to the interface without a
   * test or a call); the runtime makes it the object that the erased translation passed (null
   * for the null string and for a nil pointer, the most derived object for a class with
   * subclasses: canon_ in rt_conv.go) before it formats the message.
   *
   * @return the type to pass the argument at, or null when it is converted to Object as usual
   */
  TypeMirror msgArg(ExecutableElement e, int i, TypeMirror arg, TypeMirror target) {
    if (!MSG || i < 1 || arg == null || cx.kind(cx.erase(target)) != Ctx.K.ANY || !isPreconditions(e)) return null;
    TypeMirror at = cx.erase(arg);
    Ctx.K k = cx.kind(at);
    if (k == Ctx.K.STR) return at;
    TypeElement te = k == Ctx.K.CLS ? Ctx.typeElement(at) : null;
    if (te == null || cx.isHandle(te) || cx.isValueLib(te) || cx.isTypedColl(at) || !cx.goType(at).startsWith("*")) return null;
    return at;
  }

  /**
   * Rule "message arguments", handles: a handle (a Node) as the one message argument of a check
   * stays a handle; the Go function (suffix _H, generic in the handle type) boxes it when the
   * check fails.
   *
   * @return the handle type of the last argument when the function `name` has such a variant
   */
  TypeMirror handleMsg(ExecutableElement e, String name, int nparams, int nargs, TypeMirror last) {
    if (!MSG || e.isVarArgs() || nparams < 2 || nargs != nparams || !HANDLE_MSG.contains(name) || last == null) return null;
    TypeMirror lt = cx.erase(last);
    return cx.kind(lt) == Ctx.K.CLS && cx.isHandle(Ctx.typeElement(lt)) ? lt : null;
  }

  // ------------------------------------------------------------------ library classes of one Go type

  static final boolean ALIAS = !off("ALIAS");

  /** Go type names that the runtime declares as aliases (`type ImmutableList = ImmutableCollection`): name -> the type it is. */
  final Map<String, String> aliases = new HashMap<>();

  /** Reads the alias declarations of the runtime (rt_*.go in the output directory). */
  void loadAliases(Path out) throws IOException {
    if (!ALIAS || !Files.isDirectory(out)) return;
    Pattern decl = Pattern.compile("^type (\\w+) = (\\w+)$");
    try (var fs = Files.list(out)) {
      for (Path f : (Iterable<Path>) fs.sorted()::iterator) {
        String fn = f.getFileName().toString();
        if (!fn.startsWith("rt_") || !fn.endsWith(".go") || fn.endsWith("_test.go")) continue;
        for (String l : Files.readAllLines(f)) {
          Matcher m = decl.matcher(l);
          if (m.matches()) aliases.put(m.group(1), m.group(2));
        }
      }
    }
  }

  /** The Go type that the type named n is, through the runtime's aliases. */
  String aliased(String n) {
    for (int i = 0; i < 8 && aliases.containsKey(n); i++) n = aliases.get(n);
    return n;
  }

  /**
   * Rule "one Go type": two library classes that the runtime declares as one Go type (an alias:
   * ImmutableList and ImmutableSet are ImmutableCollection) convert as the value itself. The
   * erased translation asserted the type to itself, cast_[*ImmutableList](obj_(v)), which
   * cannot fail and keeps nil.
   */
  boolean sameGoType(String gf, String gt) {
    return ALIAS && gf.startsWith("*") && gt.startsWith("*") && !gf.contains("[") && !gt.contains("[") && aliased(gf.substring(1)).equals(aliased(gt.substring(1)));
  }

  // ------------------------------------------------------------------ typed Object methods

  static final boolean TYPEDOBJ = !off("TYPEDOBJ");

  /**
   * The runtime helpers for equals / hashCode / toString of a value held at its static type:
   * "" (equals_, hash_, tostr_: generic in a pointer to a class without subclasses or an
   * interface, null is the zero value), "H" (the same for a handle), or null when the value
   * has no such helper: a class with subclasses is its most derived object (v.self()), a typed
   * collection its view, a value type its box, and an Object is one already.
   */
  String objHelper(TypeMirror t0) {
    if (!TYPEDOBJ || t0 == null) return null;
    TypeMirror t = cx.erase(t0);
    if (cx.isTypedColl(t) || cx.isValueLibType(t) || cx.fn.is(t) || cx.lg.is(t)) return null;
    switch (cx.kind(t)) {
      case IFACE:
        return "";
      case CLS: {
        TypeElement te = Ctx.typeElement(t);
        if (cx.isValueLib(te)) return null;
        if (cx.isHandle(te)) return "H";
        if (cx.hasSelf(te) && !cx.isLeaf(te)) return null;
        return cx.goType(t).startsWith("*") ? "" : null;
      }
      default:
        return null;
    }
  }

  /**
   * Whether equals of every object of static type te is identity: an enum, or a class of the
   * program without subclasses that neither declares nor inherits an equals(Object) other than
   * Object's. Objects.equals(a, b) of two such values is a == b.
   */
  boolean identityEquals(TypeElement te) {
    if (te == null || !cx.isSource(te)) return false;
    if (te.getKind() == ElementKind.ENUM) return true;
    if (te.getKind() != ElementKind.CLASS || !cx.isLeaf(te)) return false;
    for (Element e : cx.elems.getAllMembers(te)) {
      if (e instanceof ExecutableElement m && m.getSimpleName().contentEquals("equals") && m.getParameters().size() == 1
          && !m.getModifiers().contains(Modifier.STATIC) && !cx.ownerOf(m).getQualifiedName().contentEquals("java.lang.Object")) return false;
    }
    return true;
  }

  /** Objects.equals(a, b) of two values of static type t without a box, or null (as before). */
  String typedEquals(String a, String b, TypeMirror t) {
    // rule "function types": a function has the identity of its closure (rt_fn.go)
    if (cx.fn.is(cx.erase(t))) return "fnSame_(" + a + ", " + b + ")";
    if (!TYPEDOBJ) return null;
    TypeMirror et = cx.erase(t);
    if (cx.kind(et) == Ctx.K.CLS && !cx.isTypedColl(et) && identityEquals(Ctx.typeElement(et))) return a + " == " + b;
    String h = objHelper(t);
    return h == null ? null : "equals" + h + "_(" + a + ", " + b + ")";
  }

  /** Objects.hashCode(v) (0 for null) of a value of static type t without a box, or null (as before). */
  String typedHash(String v, TypeMirror t) {
    if (cx.fn.is(cx.erase(t))) return "fnHash_(" + v + ")";
    if (!TYPEDOBJ) return null;
    TypeMirror et = cx.erase(t);
    switch (cx.kind(et)) {
      case STR:
        return "strhash_(" + v + ")";
      case PRIM:
        switch (et.getKind()) {
          case BOOLEAN: return "boolhash_(" + v + ")";
          case BYTE, SHORT, CHAR: return "int32(" + v + ")";
          case INT: return v;
          case LONG: return "longhash_(" + v + ")";
          case DOUBLE: return "dblhash_(" + v + ")";
          default: return null;
        }
      default: {
        String h = objHelper(t);
        return h == null ? null : "hash" + h + "_(" + v + ")";
      }
    }
  }

  /** String.valueOf(v) ("null" for null) of an object of static type t without a box, or null (as before). */
  String typedStr(String v, TypeMirror t) {
    if (cx.fn.is(cx.erase(t))) return "fnStr_(" + v + ")";
    String h = objHelper(t);
    return h == null ? null : "tostr" + h + "_(" + v + ")";
  }

  // ------------------------------------------------------------------ typed Object calls

  /**
   * Rule "typed Object calls" (ExprEmitter.typedObjCall, typedObjStatic): a call that the program
   * writes of Object's equals / hashCode / toString on a receiver whose class does not declare
   * the method, and of Objects.equals / Objects.hashCode / String.valueOf(Object), passes the
   * values at their static types to runtime functions generic in them (rt_conv.go), where the
   * erased translation converted each to Object first.
   */
  static final boolean OBJCALL = !off("OBJCALL");

  /**
   * Rule "identity functions" (ExprEmitter.identityFn, identityCall): a call of a static method
   * of the program whose body is `return a == b` on its two Object parameters is translated as
   * the comparison `a == b` of its arguments, which keeps them at their static types where the
   * call converted both to Object.
   */
  static final boolean IDENT = !off("IDENT");

  // ------------------------------------------------------------------ implicit conversions

  static final boolean IMPLICIT = !off("IMPLICIT");

  /** The generic functions of the runtime (`func name[...]` in rt_*.go), by name. */
  final Set<String> rtGenerics = new HashSet<>();

  /** Reads the names of the generic functions of the runtime (rt_*.go in the output directory). */
  void loadGenerics(Path out) throws IOException {
    if (!IMPLICIT || !Files.isDirectory(out)) return;
    Pattern decl = Pattern.compile("^func (\\w+)\\[.*");
    try (var fs = Files.list(out)) {
      for (Path f : (Iterable<Path>) fs.sorted()::iterator) {
        String fn = f.getFileName().toString();
        if (!fn.startsWith("rt_") || !fn.endsWith(".go") || fn.endsWith("_test.go")) continue;
        for (String l : Files.readAllLines(f)) {
          Matcher m = decl.matcher(l);
          if (m.matches()) rtGenerics.add(m.group(1));
        }
      }
    }
  }

  /**
   * Rule "implicit conversions": a value passed where Go declares `any` (a parameter of that
   * type, an element of a []any) is written as it is, where the erased translation wrote the
   * conversion any(x) of a primitive, an interface value or an array. Go converts the value to
   * the parameter's type as the explicit conversion did; nothing changes at run time. (obj_,
   * strobj_ and box_() are not such conversions: they make null of a nil pointer, the null
   * string and handle 0.)
   *
   * @param r code for a place whose Go type is `any`
   */
  static String implicit(String r) {
    return IMPLICIT && r.startsWith("any(") && closesAtEnd(r, 3) ? r.substring(4, r.length() - 1) : r;
  }

  /**
   * Whether the Object parameters of library function or constructor `name` are declared `any`
   * in Go, so that an argument needs no conversion. A generic function of the runtime could
   * declare a type parameter there, which Go would infer from the argument: its arguments keep
   * their conversions. (The variants _T and _H of the checks are generic in a parameter that the
   * translator passes typed; their other parameters are `any`.)
   */
  boolean anyParams(String name) {
    return !rtGenerics.contains(name);
  }

  // ------------------------------------------------------------------ String for CharSequence

  static final boolean STRSEQ = !off("STRSEQ");
  static final boolean STRSEQ_REPORT = System.getenv("J2GO_STRSEQ") != null;

  /**
   * The library functions and methods (by their Go names) that have a variant taking a Go string
   * where Java declares a CharSequence (rt_conv.go): name -> the variant. A method of a class is
   * "Class.method"; a variant without a dot for a method is a function taking the receiver first
   * (the method of an interface that classes of the program implement too).
   */
  static final Map<String, String> STR_SEQ = Map.ofEntries(
      Map.entry("String_contains", "String_contains_s"),
      Map.entry("String_replace", "String_replace_s"),
      Map.entry("Pattern_matches", "Pattern_matches_s"),
      Map.entry("Pattern.matcher", "matcher_s"),
      Map.entry("Matcher_re2j.reset", "reset_s"),
      Map.entry("String_join", "String_join_s"),
      Map.entry("CharMatcher_anyOf", "CharMatcher_anyOf_s"),
      Map.entry("Appendable.append_CharSequence_int_int", "Appendable_append_s_int_int"),
      Map.entry("CharMatcher.matchesAllOf", "matchesAllOf_s"),
      Map.entry("Splitter.split", "split_s"),
      Map.entry("Splitter.splitToList", "splitToList_s"),
      Map.entry("Appendable.append_CharSequence", "Appendable_append_s"));

  /** What J2GO_STRSEQ=1 reports: calls passing a String for a CharSequence that have no variant, by name. */
  final TreeMap<String, Integer> strSeqLeft = new TreeMap<>();

  boolean isCharSequence(TypeMirror t) {
    TypeElement te = Ctx.typeElement(cx.erase(t));
    return te != null && te.getQualifiedName().contentEquals("java.lang.CharSequence");
  }

  /**
   * Rule "String for CharSequence": a call of a library method that declares CharSequence
   * parameters, with a String for each of them, calls the variant of the method that takes Go
   * strings (STR_SEQ), so that the string is not boxed (which allocates). The variant does what
   * the method does with the string of the sequence: null (the null string) is the
   * NullPointerException of the method, or the text "null" where Java appends that.
   *
   * @param name the Go name of the method ("Class.method") or function
   * @param params the erased parameter types; those that are CharSequence become String
   * @return the name of the variant (params is changed), or null when the call stays as it is
   */
  String strSeq(String name, ExecutableElement e, List<TypeMirror> params, List<TypeMirror> args) {
    if (!STRSEQ || e.isVarArgs() || params.size() != args.size()) return null;
    boolean any = false;
    for (int i = 0; i < params.size(); i++) {
      if (!isCharSequence(e.getParameters().get(i).asType())) continue;
      if (args.get(i) == null || cx.kind(args.get(i)) != Ctx.K.STR) return null;
      any = true;
    }
    if (!any) return null;
    String v = STR_SEQ.get(name);
    if (v == null) {
      strSeqLeft.merge(name, 1, Integer::sum);
      return null;
    }
    TypeMirror string = cx.elems.getTypeElement("java.lang.String").asType();
    for (int i = 0; i < params.size(); i++) if (isCharSequence(e.getParameters().get(i).asType())) params.set(i, string);
    return v;
  }

  void strSeqReport() {
    if (!STRSEQ_REPORT) return;
    for (var en : strSeqLeft.entrySet()) System.err.println("strseq: left " + en.getValue() + " " + en.getKey() + ": no variant taking a string");
  }
}
