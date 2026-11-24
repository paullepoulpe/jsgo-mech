package j2go;

import com.sun.source.tree.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Rule "typed library interfaces", piece `objarg` (rule candidate T3 of docs/javaisms6/JAVAISMS6.md,
 * recorded in docs/libgen/COLLECTIONS-PLAN.md section 10; J2GO_NOLIBGEN=objarg is the translation
 * without it): the collection methods that Java declares with an Object parameter,
 * `Map.get / containsKey / remove(Object)` and `Collection.contains / remove(Object)` called on a
 * Map or Set receiver (a typed interface, or a typed collection whose own methods do not serve
 * the call), with an argument whose static Java type is the key or element type or a subtype of
 * it, are the call of a typed twin (go/closure/rt_lgobj.go) that takes the argument at the key or
 * element type instead of its box, where the translation has a typed map or set of those types
 * (PAIRS, a checked-in name list). The twin does what the erased call does: the typed map's or
 * set's own lookup where the receiver's object is its view (the same `find` that the erased method
 * reaches after it has taken the key out of its box), else the erased method with the argument
 * boxed as erased code boxes a value of that type (boxT_); a null argument always takes the erased
 * method. Java looks the key up by its equals and hashCode: the object is the same in both calls,
 * so the answer is.
 *
 * Any other call keeps the erased method, as before; J2GO_LIBGEN=2 reports each by reason
 * ("Object-parameter methods: ..."): the argument's static type is Object, a supertype of the
 * element type, of another class or interface, or the element's class at other type arguments;
 * the element type is Object in Go or a wildcard; the argument is a field or result of a type
 * variable's type (Go has it as `any`: the twin would cast it); the receiver is erased; a String
 * argument the String-key helpers of rt_coll.go serve; a result that enters an Object (Java
 * casts nothing there, as LibFns); a key type that Go cannot compare; a List or Collection
 * receiver (`List.indexOf / lastIndexOf`, `contains` on a List: no typed lookup, so the call
 * keeps its box where it is); a pair that is not on PAIRS.
 */
final class LibObj {
  /** The methods (key: LibFns.key of the root method) and the twin of a Map receiver's call. */
  static final Map<String, String> TYPED = Map.of(
      "java.util.Map.get(java.lang.Object)", "Map_getK_",
      "java.util.Map.containsKey(java.lang.Object)", "Map_containsKeyK_",
      "java.util.Map.remove(java.lang.Object)", "Map_removeK_",
      "java.util.Collection.contains(java.lang.Object)", "",
      "java.util.Collection.remove(java.lang.Object)", "",
      "java.util.List.indexOf(java.lang.Object)", "",
      "java.util.List.lastIndexOf(java.lang.Object)", "");

  /** The twins of the Collection methods for a receiver whose static Java type is a Set. */
  static final Map<String, String> TYPED_SET = Map.of(
      "java.util.Collection.contains(java.lang.Object)", "Set_containsE_",
      "java.util.Collection.remove(java.lang.Object)", "Set_removeE_");

  /** The Go type of the receiver a twin takes, by the owner of the root method ($0, $1: the view's type arguments). */
  static final Map<String, String> RECV = Map.of(
      "java.util.Map", "MapT[$0, $1]",
      "java.util.Collection", "CollectionT[$0]",
      "java.util.Set", "SetT[$0]",
      "java.util.List", "ListT[$0]");

  /**
   * The key / element types for which the translation has a typed map or set ("map K, V" /
   * "set T", Go types): a checked-in name list (tools/j2go/objarg-pairs.txt; J2GO_OBJARG_PAIRS,
   * paths separated by ':', replaces it: the self-test adds its own). A twin is called only for a
   * pair on it: for any other pair its typed lookup could never be taken (no typed map of those
   * types exists), and Go would compile a typed map for it. finish() fails the translation when
   * the list is not exactly the pairs that the calls ask for and that have a typed map or set.
   */
  static final Set<String> PAIRS = new TreeSet<>();

  static {
    String v = System.getenv("J2GO_OBJARG_PAIRS");
    for (String f : (v != null ? v : "tools/j2go/objarg-pairs.txt").split(":")) {
      try {
        for (String l : java.nio.file.Files.readAllLines(java.nio.file.Path.of(f))) {
          l = l.replaceAll("#.*", "").trim();
          if (!l.isEmpty()) PAIRS.add(l);
        }
      } catch (java.io.IOException ex) {
        // (no list: no twin is called; finish() reports what the list should be)
      }
    }
  }

  /** The pairs the calls asked for, those a twin was called for, and the typed maps and sets the translation makes (Ctx.tconvsOf). */
  final Set<String> requested = new TreeSet<>(), emitted = new TreeSet<>(), typedColls = new TreeSet<>();

  /** Ctx.tconvsOf: the translation makes a typed collection of Go type g. */
  void made(String g) {
    if (g.startsWith("*TMap[") && g.endsWith("]")) typedColls.add("map " + g.substring(6, g.length() - 1));
    if (g.startsWith("*TSet[") && g.endsWith("]")) typedColls.add("set " + g.substring(6, g.length() - 1));
  }

  /**
   * After the translation: the pairs that should be on the list (asked for by a call and made as
   * a typed map or set), written to J2GO_OBJARG_PAIRS_OUT if it is set. With the piece on and the
   * rest of the rules as by default (J2GO_NOLIBGEN and J2GO_NOFNTYPES unset) a list that differs
   * from them is stale and the translation fails; else the differences are reported.
   *
   * @return whether the list is stale where that fails
   */
  boolean finish() throws java.io.IOException {
    if (!on()) return false;
    Set<String> want = new TreeSet<>(requested);
    want.retainAll(typedColls);
    String out = System.getenv("J2GO_OBJARG_PAIRS_OUT");
    if (out != null) {
      StringBuilder sb = new StringBuilder("# piece objarg (LibObj.PAIRS): the key / element types of Map.get / containsKey / remove and Set.contains /\n"
          + "# remove calls for which the translation has a typed map or set. Generated: J2GO_OBJARG_PAIRS_OUT=<file> tools/j2go/run.sh\n");
      for (String p : want) sb.append(p).append("\n");
      java.nio.file.Files.writeString(java.nio.file.Path.of(out), sb.toString());
    }
    Set<String> extra = new TreeSet<>(PAIRS), missing = new TreeSet<>(want), stray = new TreeSet<>(emitted);
    extra.removeAll(want);
    missing.removeAll(PAIRS);
    stray.removeAll(PAIRS);
    if (extra.isEmpty() && missing.isEmpty() && stray.isEmpty()) return false;
    boolean strict = System.getenv("J2GO_NOLIBGEN") == null && System.getenv("J2GO_NOFNTYPES") == null;
    System.err.println("objarg: the pair list is stale" + (strict ? "" : " (not checked: a rule is off)") + ": on the list with no typed map or set or no call " + extra
        + "; asked for, with a typed map or set, not on the list " + missing + "; twins called for pairs not on the list " + stray);
    return strict;
  }

  final Ctx cx;

  LibObj(Ctx cx) {
    this.cx = cx;
  }

  static boolean on() {
    return LibGen.on("objarg");
  }

  /** Reports that the call of key stays erased, and why; returns null (the erased call). */
  <T> T left(String key, String why) {
    cx.lg.note("Object-parameter methods: calls left erased (" + why + ")", key);
    return null;
  }

  /** Whether an argument of static Java type a is a value of key or element type elem (Java's static typing). */
  boolean fits(TypeMirror a, TypeMirror elem) {
    if (a.getKind() == TypeKind.NULL) return true;
    if (a.getKind().isPrimitive()) return cx.types.isSameType(cx.types.boxedClass((PrimitiveType) a).asType(), elem);
    return cx.types.isSubtype(a, elem);
  }

  /** Why an argument of static Java type a that does not fit elem stays boxed. */
  String wider(TypeMirror a, TypeMirror elem) {
    if (a.getKind() == TypeKind.DECLARED && Ctx.typeElement(a).getQualifiedName().contentEquals("java.lang.Object")) return "the argument's static type is Object";
    TypeMirror ea = cx.types.erasure(a), ee = cx.types.erasure(elem);
    if (cx.types.isSubtype(ea, ee)) return "the argument's static type is the element's class at other type arguments";
    if (cx.types.isSubtype(ee, ea)) return "the argument's static type is a supertype of the element type";
    return "the argument's static type is another class or interface";
  }

  /**
   * Whether the Go type of key or element type t is comparable: a kept type parameter whose
   * constraint is `comparable` (Ctx.constraint), a pointer, an interface, a string, a handle;
   * not a Go func type (rule "function types"), a slice, a map or a value struct (Optional).
   */
  boolean comparable(TypeMirror t) {
    TypeMirror e = cx.erase(t);
    if (e.getKind() == TypeKind.TYPEVAR) return cx.typedVar(e) && cx.constraint((TypeParameterElement) ((TypeVariable) e).asElement()).contains("comparable");
    if (e.getKind() == TypeKind.ARRAY || e.getKind() != TypeKind.DECLARED) return false;
    if (cx.fn.is(e) || cx.isValueLib(Ctx.typeElement(e))) return false;
    String g = cx.goType(e);
    return !g.startsWith("func(") && !g.startsWith("[]") && !g.startsWith("map[");
  }

  /** Whether Java type t names a type variable or a wildcard. */
  static boolean hasVar(TypeMirror t) {
    if (t == null) return false;
    switch (t.getKind()) {
      case TYPEVAR, WILDCARD, INTERSECTION, UNION: return true;
      case ARRAY: return hasVar(((ArrayType) t).getComponentType());
      case DECLARED: {
        for (TypeMirror x : ((DeclaredType) t).getTypeArguments()) if (hasVar(x)) return true;
        return false;
      }
      default: return false;
    }
  }

  /**
   * Null where argument a is, in Go, a value of its static type as it is (the code that the erased
   * call boxed): null, a new object, `this`, a cast, a local variable or parameter (also the
   * unconverted parameter of a method reference), a field or the
   * result of a method whose declared type names no type variable. Else why not: a field or result
   * of a type variable's type is erased in Go (`any`) and would be cast to the key type, where Java
   * casts nothing (the parameter is Object).
   */
  String argForm(ExprEmitter x, ExprEmitter.Arg a) {
    // (a parameter of a method reference, `memoized::get`: as it is where the code is the parameter itself, unconverted)
    if (a.tree == null) return a.code != null && a.code.matches("[A-Za-z_][A-Za-z0-9_]*") ? null : "the argument is a converted parameter of a method reference";
    if (a.type.getKind() == TypeKind.NULL) return null;
    ExpressionTree t = Base.strip(a.tree);
    switch (t.getKind()) {
      case NEW_CLASS, TYPE_CAST:
        return null;
      case IDENTIFIER, MEMBER_SELECT, METHOD_INVOCATION: {
        if (t instanceof IdentifierTree id && id.getName().contentEquals("this")) return null;
        Element e = x.elemOf(t instanceof MethodInvocationTree mi ? mi.getMethodSelect() : t);
        if (e instanceof ExecutableElement m) return hasVar(m.getReturnType()) ? "the argument is the result of a method of a type variable's type (erased in Go: Java casts nothing)" : null;
        if (e instanceof VariableElement v) {
          if (Base.isLocalKind(v)) return null;
          return hasVar(v.asType()) ? "the argument is a field of a type variable's type (erased in Go: Java casts nothing)" : null;
        }
        return "the argument is no variable, field or call";
      }
      default:
        return "the argument is another expression (" + t.getKind() + ")";
    }
  }

  /**
   * The call of the typed twin of method e on receiver recv with arguments as, or null where the
   * erased call stays (ExprEmitter.special). toObject: the call's result enters an Object.
   */
  ExprEmitter.CallResult call(ExprEmitter x, ExecutableElement e, ExprEmitter.Recv recv, List<ExprEmitter.Arg> as, boolean toObject) {
    if (!on() || recv == null || recv.type == null || e.getModifiers().contains(Modifier.STATIC) || cx.isSource(cx.ownerOf(e))) return null;
    ExecutableElement r = cx.root(e);
    String key = LibFns.key(cx, r);
    String go = TYPED.get(key);
    if (go == null || as.size() != 1) return null;
    TypeElement lib = cx.ownerOf(r);
    TypeMirror a = as.get(0).type;
    if (a == null) return left(key, "the argument has no type");
    if (!cx.lg.isLib(lib)) return left(key, "the owner's kind is off");
    // the receiver at the typed interface of the method's owner (vlib; a Set receiver of a
    // Collection method: at the typed Set, TYPED_SET)
    TypeMirror at = cx.erase(recv.type);
    Ctx.CollKind k = cx.typedColl(at);
    TypeElement vlib = lib, set = cx.elems.getTypeElement("java.util.Set");
    if (TYPED_SET.containsKey(key) && cx.lg.isLib(set)
        && (k != null ? k.name.equals("set") : cx.types.isSubtype(cx.types.erasure(recv.type), cx.types.erasure(set.asType())))) {
      vlib = set;
      go = TYPED_SET.get(key);
    }
    DeclaredType view;
    String rc;
    if (k != null) {
      // (a method the typed collection has itself, TMap.get of a key: rule "typed collections")
      if (cx.typedMethod(recv.type, e, List.of(a)) != null) {
        cx.lg.note("Object-parameter methods: not this piece (the typed collection's own method)", key);
        return null;
      }
      List<? extends TypeMirror> sa = ((DeclaredType) at).getTypeArguments();
      if (k.builds != null || sa.size() != vlib.getTypeParameters().size()) return left(key, "the receiver is no typed collection of the owner");
      TypeMirror to = cx.erase(cx.types.getDeclaredType(vlib, sa.toArray(new TypeMirror[0])));
      rc = cx.lg.is(to) ? cx.lg.fromTypedColl(recv.code, at, to) : null;
      if (rc == null) return left(key, "the receiver is no typed collection of the owner");
      view = (DeclaredType) to;
    } else {
      view = cx.lg.typedView(recv.type, vlib);
      if (view == null) return left(key, "the receiver is erased");
      rc = null;
    }
    // the key or element type: the receiver's Java view of the owner (javac's static types), and its Go type
    DeclaredType jv = cx.superView(k != null ? cx.typedView(at) : recv.type, vlib);
    if (jv == null || jv.getTypeArguments().size() != view.getTypeArguments().size()) return left(key, "no Java view of the owner");
    TypeMirror elem = jv.getTypeArguments().get(0);
    if (elem.getKind() == TypeKind.WILDCARD) return left(key, "the element type is a wildcard");
    List<String> ta = cx.lf.goArgs(view);
    if (ta.get(0).equals("any")) return left(key, "the element type is Object in Go");
    if (!cx.goType(view).equals(LibFns.fill(RECV.get(vlib.getQualifiedName().toString()), ta))) return left(key, "the receiver's type arguments");
    if (!cx.goTypeArg(elem).equals(ta.get(0))) return left(key, "the element type's Go type is not the view's");
    // (the twins of Map and Set call the typed map's or set's lookup: a key type Go can compare)
    if ((vlib.getQualifiedName().contentEquals("java.util.Map") || vlib.equals(set)) && !comparable(elem)) return left(key, "the element type is not comparable in Go");
    if (!fits(a, elem)) return left(key, wider(a, elem));
    // a String: the String-key helpers serve it unboxed (ExprEmitter.stringKeyHelper; Map.get: LibFns' twin of Map_get_s)
    if (cx.kind(a) == Ctx.K.STR && x.stringKeyHelper(r, recv, as) != null) return left(key, "a String: the String-key helper serves it");
    String form = argForm(x, as.get(0));
    if (form != null) return left(key, form);
    // (an element given as Object: Java casts nothing there; the twin would give it at its type)
    ExecutableType mt = (ExecutableType) cx.types.asMemberOf(view, r);
    TypeMirror ret = cx.erase(mt.getReturnType());
    if (toObject && r.getReturnType().getKind() == TypeKind.TYPEVAR) return left(key, "the result enters an Object");
    // only a Map or Set receiver has a typed lookup, and only where the translation has a typed
    // map or set of these types (PAIRS); elsewhere the erased call keeps its box
    boolean mapOrSet = vlib.getQualifiedName().contentEquals("java.util.Map") || vlib.equals(set);
    if (!mapOrSet) return left(key, "no typed lookup: a List or Collection receiver (the erased call keeps its box)");
    String pair = (vlib.equals(set) ? "set " : "map ") + String.join(", ", ta);
    requested.add(pair);
    if (!PAIRS.contains(pair)) return left(key, "no typed map or set of these types in the translation (objarg-pairs.txt)");
    emitted.add(pair);
    if (!cx.conv.rtGenerics.contains(go)) return left(key, "the runtime has no " + go);
    if (rc == null) rc = cx.coerce(recv.code, recv.type, view);
    // the erased interface keeps the erased method, which the twin calls (the runtime's own code calls it)
    List<TypeMirror> ep = new ArrayList<>(cx.groupParams(e));
    ep.replaceAll(cx.lg::rt);
    x.noteMethod("method " + cx.className(lib) + "." + cx.methodName(e), r, ep, cx.lg.rt(cx.groupReturn(e)), "method");
    cx.lg.note("Object-parameter methods: calls of the typed twin", key);
    String arg = as.get(0).as(cx.erase(elem));
    return new ExprEmitter.CallResult(go + "[" + String.join(", ", ta) + "](" + rc + ", " + arg + ")", ret);
  }
}
