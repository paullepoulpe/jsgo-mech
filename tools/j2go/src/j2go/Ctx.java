package j2go;

import com.sun.source.tree.*;
import com.sun.source.util.*;
import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;
import javax.lang.model.util.*;

/** Whole-program model: classes, hierarchy, names, method groups, type mapping and coercions. */
final class Ctx {
  final Trees trees;
  final Types types;
  final Elements elems;

  /** Every class declared in the translated sources, including nested, local and anonymous ones. */
  final LinkedHashSet<TypeElement> source = new LinkedHashSet<>();
  final Map<TypeElement, List<TypeElement>> subclasses = new HashMap<>();
  final Map<TypeElement, String> classNames = new HashMap<>();
  final Map<String, TypeElement> classByName = new HashMap<>();
  /** External (library) members and types the translation refers to: Go spelling -> description. */
  final TreeMap<String, String> externals = new TreeMap<>();

  enum K { PRIM, STR, CLS, IFACE, ANY, ARR, NULL, VOID }

  /** Library types that are represented as Go `any`. */
  static final Set<String> ANY_TYPES = Set.of(
      "java.lang.Object", "java.lang.Number", "java.lang.Integer", "java.lang.Long", "java.lang.Short",
      "java.lang.Byte", "java.lang.Character", "java.lang.Boolean", "java.lang.Double", "java.lang.Float",
      "java.lang.Void", "java.lang.CharSequence", "java.lang.Comparable", "java.lang.Cloneable",
      "java.io.Serializable", "java.lang.Record", "java.lang.constant.Constable",
      "java.lang.constant.ConstantDesc");

  /** Library classes written to be extended: they carry self_ and as_X() like translated classes. */
  static final Set<String> EXTENDABLE = new HashSet<>(List.of(
      "java.lang.Throwable", "java.lang.Exception", "java.lang.RuntimeException", "java.lang.Error",
      "java.io.IOException", "java.lang.IllegalStateException", "java.lang.IllegalArgumentException",
      "java.lang.UnsupportedOperationException", "java.util.AbstractSet", "java.util.AbstractMap",
      "java.util.AbstractList", "java.util.AbstractCollection", "java.util.AbstractQueue",
      "com.google.common.collect.AbstractIterator", "com.google.common.collect.Ordering",
      "com.google.common.collect.ForwardingMap", "com.google.common.collect.ForwardingSet",
      "java.io.OutputStream", "java.io.InputStream", "java.io.Writer", "java.io.Reader",
      "java.io.FilterOutputStream", "java.nio.file.SimpleFileVisitor", "java.util.logging.Handler",
      "java.util.logging.Formatter", "java.util.concurrent.ThreadPoolExecutor", "java.lang.Thread",
      "org.kohsuke.args4j.spi.OptionHandler", "org.kohsuke.args4j.spi.StringOptionHandler",
      "org.kohsuke.args4j.spi.BooleanOptionHandler", "org.kohsuke.args4j.spi.IntOptionHandler",
      "org.kohsuke.args4j.spi.StopOptionHandler", "com.google.gson.TypeAdapter",
      "java.lang.Enum", "java.util.ArrayList", "com.google.gson.reflect.TypeToken",
      "com.google.common.cache.CacheLoader", "com.google.common.escape.Escaper",
      "java.util.zip.GZIPOutputStream", "java.io.FilterInputStream", "org.kohsuke.args4j.OptionDef", "com.google.gson.JsonElement"));

  static final Set<String> GO_RESERVED = new HashSet<>(List.of(
      "break", "default", "func", "interface", "select", "case", "defer", "go", "map", "struct", "chan",
      "else", "goto", "package", "switch", "const", "fallthrough", "if", "range", "type", "continue",
      "for", "import", "return", "var", "append", "bool", "byte", "cap", "clear", "close", "complex",
      "copy", "delete", "error", "false", "imag", "int", "iota", "len", "make", "max", "min", "new",
      "nil", "panic", "print", "println", "real", "recover", "rune", "string", "true", "uint", "any",
      "init", "main", "float32", "float64", "int8", "int16", "int32", "int64", "uint8", "uint16",
      "uint32", "uint64", "uintptr", "comparable", "this"));

  Ctx(Trees trees, Types types, Elements elems) {
    this.trees = trees;
    this.types = types;
    this.elems = elems;
  }

  // ---------------------------------------------------------------- classes

  boolean isSource(TypeElement te) {
    return source.contains(te);
  }

  static TypeElement typeElement(TypeMirror t) {
    if (t == null) return null;
    if (t.getKind() == TypeKind.DECLARED) return (TypeElement) ((DeclaredType) t).asElement();
    return null;
  }

  /** The superclass, or null for none / java.lang.Object. Package-private library classes are skipped. */
  TypeElement superclass(TypeElement te) {
    if (te.getKind().isInterface()) return null;
    TypeElement s = typeElement(te.getSuperclass());
    while (s != null && !isSource(s) && !s.getModifiers().contains(Modifier.PUBLIC)) s = typeElement(s.getSuperclass());
    if (s == null || s.getQualifiedName().contentEquals("java.lang.Object")) return null;
    if (ANY_TYPES.contains(s.getQualifiedName().toString())) return null;
    return s;
  }

  final Map<TypeElement, List<VariableElement>> captureCache = new HashMap<>();
  /** Local and anonymous classes created where an enclosing instance exists. */
  final Set<TypeElement> localWithOuter = new HashSet<>();
  /** The lexically enclosing class of local and anonymous classes. */
  final Map<TypeElement, TypeElement> lexicalOuter = new HashMap<>();
  /** Static-initializer functions, in emission order. */
  final List<String> clinits = new ArrayList<>();
  final Map<String, String> clinitOwner = new LinkedHashMap<>();

  /** Instrumented output: every function first makes sure the classes whose statics it touches are initialized. */
  boolean initGuards = System.getenv("J2GO_INIT_GUARDS") != null;
  private final Map<TypeElement, Boolean> clinitCache = new HashMap<>();

  /** True if the class has a static initializer function. */
  boolean hasClinit(TypeElement te) {
    if (!isSource(te)) return false;
    return clinitCache.computeIfAbsent(te, k -> {
      if (te.getKind() == ElementKind.ENUM) return true;
      for (Element e : te.getEnclosedElements()) {
        if (e.getKind() == ElementKind.ENUM_CONSTANT) return true;
        if (e.getKind() == ElementKind.FIELD && e.getModifiers().contains(Modifier.STATIC)) {
          VariableElement f = (VariableElement) e;
          Object cv = f.getConstantValue();
          if (cv == null) return true;
          if (cv instanceof Double d && (d.isNaN() || d.isInfinite())) return true;
          if (cv instanceof Float fl && (fl.isNaN() || fl.isInfinite())) return true;
        }
      }
      ClassTree ct = trees.getTree(te);
      if (ct != null) {
        for (Tree m : ct.getMembers()) if (m instanceof BlockTree b && b.isStatic()) return true;
      }
      return false;
    });
  }

  boolean isExtendable(TypeElement te) {
    if (EXTENDABLE.contains(te.getQualifiedName().toString())) return true;
    return types.isSubtype(erase(te.asType()), elems.getTypeElement("java.lang.Throwable").asType());
  }

  boolean isLeaf(TypeElement te) {
    if (isSource(te)) return !subclasses.containsKey(te);
    return !isExtendable(te);
  }

  boolean hasOuter(TypeElement te) {
    switch (te.getNestingKind()) {
      case MEMBER:
        return te.getKind() == ElementKind.CLASS && !te.getModifiers().contains(Modifier.STATIC)
            && !te.getEnclosingElement().getKind().isInterface();
      case LOCAL, ANONYMOUS:
        return localWithOuter.contains(te);
      default:
        return false;
    }
  }

  TypeElement enclosingClass(TypeElement te) {
    TypeElement o = lexicalOuter.get(te);
    if (o != null) return o;
    Element e = te.getEnclosingElement();
    while (e != null && !(e instanceof TypeElement)) e = e.getEnclosingElement();
    return (TypeElement) e;
  }

  /** True if objects of this class carry self_ (the class is in a hierarchy with subclassing). */
  boolean hasSelf(TypeElement te) {
    for (TypeElement c = te; c != null; c = superclass(c)) {
      if (!isLeaf(c)) return true;
    }
    return false;
  }

  /** True if this source class is the one that declares the self_ field. */
  boolean declaresSelf(TypeElement te) {
    if (!hasSelf(te)) return false;
    TypeElement s = superclass(te);
    if (s == null) return true;
    if (isSource(s)) return false;
    return !isExtendable(s);
  }

  boolean isSubclass(TypeElement sub, TypeElement sup) {
    for (TypeElement c = sub; c != null; c = superclass(c)) if (c.equals(sup)) return true;
    return false;
  }

  void registerClass(TypeElement te) {
    source.add(te);
  }

  /** Builds the subclass map; only now is it known which superclasses are translated classes. */
  void registerSourcesDone() {
    for (TypeElement te : source) {
      TypeElement s = superclass(te);
      if (s != null) subclasses.computeIfAbsent(s, k -> new ArrayList<>()).add(te);
    }
  }

  /** Assigns Go names to all source classes; call after every class is registered. */
  void assignClassNames() {
    Map<String, List<TypeElement>> byName = new TreeMap<>();
    for (TypeElement te : source) byName.computeIfAbsent(rawName(te), k -> new ArrayList<>()).add(te);
    for (var e : byName.entrySet()) {
      List<TypeElement> l = e.getValue();
      for (TypeElement te : l) {
        String n = e.getKey();
        if (l.size() > 1) {
          String pkg = elems.getPackageOf(te).getQualifiedName().toString();
          n = n + "_" + pkg.substring(pkg.lastIndexOf('.') + 1);
        }
        while (classByName.containsKey(n)) n += "_";
        classNames.put(te, n);
        classByName.put(n, te);
      }
    }
  }

  private String rawName(TypeElement te) {
    String bin = elems.getBinaryName(te).toString();
    String pkg = elems.getPackageOf(te).getQualifiedName().toString();
    if (!pkg.isEmpty() && bin.startsWith(pkg + ".")) bin = bin.substring(pkg.length() + 1);
    return bin.replace('$', '_').replace('.', '_');
  }

  private static final String[] JDK_PACKAGES = {
    "java.lang", "java.util", "java.util.function", "java.io", "java.util.concurrent", "java.nio.file"
  };

  String className(TypeElement te) {
    String n = classNames.get(te);
    if (n != null) return n;
    n = rawName(te);
    String pkg = elems.getPackageOf(te).getQualifiedName().toString();
    // A library class keeps its simple name unless a translated class or a core JDK class has it;
    // the rule must not depend on which class is met first.
    boolean taken = classByName.containsKey(n);
    if (!taken && !pkg.startsWith("java.") && te.getNestingKind() == NestingKind.TOP_LEVEL) {
      for (String jdk : JDK_PACKAGES) {
        if (elems.getTypeElement(jdk + "." + n) != null) taken = true;
      }
    }
    if (taken) {
      n = n + "_" + pkg.substring(pkg.lastIndexOf('.') + 1);
      while (classByName.containsKey(n)) n += "_";
    }
    classNames.put(te, n);
    classByName.put(n, te);
    noteType(te);
    return n;
  }

  // ---------------------------------------------------------------- types

  TypeMirror erase(TypeMirror t) {
    if (t == null) return null;
    switch (t.getKind()) {
      case NULL, VOID, NONE, ERROR, PACKAGE, EXECUTABLE, OTHER, MODULE:
        return t;
      default:
        return types.erasure(t);
    }
  }

  K kind(TypeMirror t0) {
    TypeMirror t = erase(t0);
    if (t == null) return K.ANY;
    switch (t.getKind()) {
      case BOOLEAN, BYTE, SHORT, INT, LONG, CHAR, FLOAT, DOUBLE:
        return K.PRIM;
      case ARRAY:
        return K.ARR;
      case NULL:
        return K.NULL;
      case VOID, NONE:
        return K.VOID;
      case DECLARED: {
        TypeElement te = typeElement(t);
        String qn = te.getQualifiedName().toString();
        if (qn.equals("java.lang.String")) return K.STR;
        if (ANY_TYPES.contains(qn)) return K.ANY;
        if (te.getKind().isInterface()) return K.IFACE;
        return K.CLS;
      }
      default:
        return K.ANY;
    }
  }

  static String primName(TypeKind k) {
    switch (k) {
      case BOOLEAN: return "bool";
      case BYTE: return "int8";
      case SHORT: return "int16";
      case INT: return "int32";
      case LONG: return "int64";
      case CHAR: return "uint16";
      case FLOAT: return "float32";
      case DOUBLE: return "float64";
      default: throw new IllegalArgumentException(k.toString());
    }
  }

  String goType(TypeMirror t0) {
    TypeMirror t = erase(t0);
    switch (kind(t)) {
      case PRIM: return primName(t.getKind());
      case STR: return "string";
      case ARR: return "[]" + goType(((ArrayType) t).getComponentType());
      case CLS: return "*" + className(typeElement(t));
      case IFACE: return className(typeElement(t));
      case VOID: return "";
      default: return "any";
    }
  }

  /** The type a boxed type unboxes to, or the type itself if primitive, else null. */
  TypeMirror unboxed(TypeMirror t) {
    if (t == null) return null;
    if (t.getKind().isPrimitive()) return t;
    try {
      return types.unboxedType(erase(t));
    } catch (IllegalArgumentException e) {
      return null;
    }
  }

  /** Expression (taking a canonical `any`) that yields a class-typed value as its most derived object. */
  String canon(String code, TypeMirror from) {
    TypeElement te = typeElement(erase(from));
    if (code.equals("this") && !hasSelf(te)) return "any(this)";
    if (hasSelf(te) && !isLeaf(te)) return code + ".self()";
    return "obj_(" + code + ")";
  }

  /** Go code converting {@code code}, an expression of Java type {@code from}, to Java type {@code to}. */
  String coerce(String code, TypeMirror from, TypeMirror to) {
    from = erase(from);
    to = erase(to);
    K kf = kind(from), kt = kind(to);
    if (kt == K.VOID) return code;
    if (kf == K.NULL) return kt == K.STR ? "nullstr_" : "nil";
    if (kf == K.VOID) return code;
    String gf = goType(from), gt = goType(to);
    if (gf.equals(gt)) return code;
    switch (kt) {
      case ANY:
        switch (kf) {
          case PRIM: return "any(" + gf + "(" + code + "))";
          case STR: return "strobj_(" + code + ")";
          case CLS: return canon(code, from);
          case ARR: return "any(" + code + ")";
          default: return "any(" + code + ")";
        }
      case IFACE:
        switch (kf) {
          case CLS: return "cast_[" + gt + "](" + canon(code, from) + ")";
          case IFACE:
            if (types.isSubtype(from, to)) return gt + "(" + code + ")";
            return "cast_[" + gt + "](" + code + ")";
          case STR: return "cast_[" + gt + "](strobj_(" + code + "))";
          case PRIM: return "cast_[" + gt + "](any(" + gf + "(" + code + ")))";
          default: return "cast_[" + gt + "](" + code + ")";
        }
      case CLS: {
        TypeElement tt = typeElement(to);
        String src;
        switch (kf) {
          case CLS: {
            TypeElement tf = typeElement(from);
            if (isSubclass(tf, tt)) return code + ".up_" + className(tt) + "()";
            src = canon(code, from);
            break;
          }
          case STR: src = "strobj_(" + code + ")"; break;
          case PRIM: src = "any(" + code + ")"; break;
          default: src = code;
        }
        if (isLeaf(tt)) return "cast_[" + gt + "](" + src + ")";
        return "to_" + className(tt) + "(" + src + ")";
      }
      case STR:
        switch (kf) {
          case CLS: return "str_(" + canon(code, from) + ")";
          case PRIM: return "str_(any(" + code + "))";
          default: return "str_(" + code + ")";
        }
      case PRIM:
        switch (kf) {
          case PRIM: {
            if (to.getKind() == TypeKind.BOOLEAN || from.getKind() == TypeKind.BOOLEAN) return code;
            boolean ff = from.getKind() == TypeKind.FLOAT || from.getKind() == TypeKind.DOUBLE;
            boolean tf = to.getKind() == TypeKind.FLOAT || to.getKind() == TypeKind.DOUBLE;
            if (ff && !tf) return "f2i_[" + gt + "](float64(" + code + "))";
            return gt + "(" + code + ")";
          }
          case ANY: case IFACE: {
            // Unboxing; a boxed value of another primitive type first unboxes to its own type.
            return "unbox_[" + gt + "](" + code + ")";
          }
          default: return "unbox_[" + gt + "](any(" + code + "))";
        }
      case ARR:
        switch (kf) {
          case ARR: {
            // Another element type: a converted copy.
            TypeMirror cf = ((ArrayType) from).getComponentType(), ct = ((ArrayType) to).getComponentType();
            return "arrmap_(" + code + ", func(v_ " + goType(cf) + ") " + goType(ct) + " { return " + coerce("v_", cf, ct) + " })";
          }
          default: return "cast_[" + gt + "](" + code + ")";
        }
      default:
        return code;
    }
  }

  // ---------------------------------------------------------------- method groups

  private final Map<ExecutableElement, ExecutableElement> rootCache = new HashMap<>();

  /** The root-most method that {@code m} overrides or implements (itself if none). */
  ExecutableElement root(ExecutableElement m) {
    ExecutableElement r = rootCache.get(m);
    if (r != null) return r;
    r = m;
    if (m.getKind() == ElementKind.METHOD && !m.getModifiers().contains(Modifier.STATIC)
        && !m.getModifiers().contains(Modifier.PRIVATE)) {
      TypeElement owner = (TypeElement) m.getEnclosingElement();
      ExecutableElement found = findOverridden(m, owner, owner, new HashSet<>());
      if (found != null) r = root(found);
    }
    rootCache.put(m, r);
    return r;
  }

  private ExecutableElement findOverridden(ExecutableElement m, TypeElement owner, TypeElement in, Set<TypeElement> seen) {
    // Superclass chain first, then interfaces, so that the choice is deterministic.
    List<TypeMirror> supers = new ArrayList<>();
    if (in.getSuperclass().getKind() == TypeKind.DECLARED) supers.add(in.getSuperclass());
    supers.addAll(in.getInterfaces());
    if (in.getKind().isInterface() && in == owner) {
      // Interfaces implicitly declare Object's public methods.
      supers.add(elems.getTypeElement("java.lang.Object").asType());
    }
    for (TypeMirror st : supers) {
      TypeElement s = typeElement(st);
      if (s == null || !seen.add(s)) continue;
      // A package-private library class (AbstractStringBuilder) is an implementation detail: look through it.
      boolean hidden = !isSource(s) && !s.getModifiers().contains(Modifier.PUBLIC);
      if (!hidden) for (Element e : s.getEnclosedElements()) {
        if (e.getKind() != ElementKind.METHOD || !e.getSimpleName().equals(m.getSimpleName())) continue;
        ExecutableElement m2 = (ExecutableElement) e;
        if (m2.getModifiers().contains(Modifier.STATIC) || m2.getModifiers().contains(Modifier.PRIVATE)) continue;
        if (elems.overrides(m, m2, owner)) return m2;
      }
      ExecutableElement deeper = findOverridden(m, owner, s, seen);
      if (deeper != null) return deeper;
    }
    return null;
  }

  TypeElement ownerOf(Element e) {
    return (TypeElement) e.getEnclosingElement();
  }

  /** Erased parameter types of the group {@code m} belongs to. */
  List<TypeMirror> groupParams(ExecutableElement m) {
    ExecutableElement r = root(m);
    List<TypeMirror> out = new ArrayList<>();
    for (VariableElement p : r.getParameters()) out.add(erase(p.asType()));
    return out;
  }

  TypeMirror groupReturn(ExecutableElement m) {
    return erase(root(m).getReturnType());
  }

  // ---- naming of methods

  /** Go names of method groups (keyed by root). */
  final Map<ExecutableElement, String> methodNames = new HashMap<>();
  /** Roots whose group has two or more concrete declarations along one class chain: dispatched through self_. */
  final Map<TypeElement, Set<ExecutableElement>> virtualAt = new HashMap<>();
  /** For each source class, the roots it declares (instance methods), including synthesized default bridges. */
  final Map<TypeElement, Map<ExecutableElement, ExecutableElement>> declared = new HashMap<>();
  final Set<String> instanceMethodNames = new HashSet<>();
  final Map<Element, String> staticNames = new HashMap<>();
  final Map<VariableElement, String> fieldNames = new HashMap<>();
  /** Default methods that a class must bridge: class -> list of interface default methods. */
  final Map<TypeElement, List<ExecutableElement>> defaultBridges = new HashMap<>();

  static String typeToken(TypeMirror t) {
    switch (t.getKind()) {
      case ARRAY: return typeToken(((ArrayType) t).getComponentType()) + "Arr";
      case DECLARED: return ((DeclaredType) t).asElement().getSimpleName().toString();
      default: return t.toString();
    }
  }

  String sigToken(ExecutableElement m) {
    StringBuilder sb = new StringBuilder();
    for (VariableElement p : m.getParameters()) sb.append('_').append(typeToken(erase(p.asType())));
    return sb.toString();
  }

  static boolean isInstanceMethod(Element e) {
    return e.getKind() == ElementKind.METHOD && !e.getModifiers().contains(Modifier.STATIC);
  }

  List<TypeElement> allSupertypes(TypeElement te) {
    LinkedHashSet<TypeElement> out = new LinkedHashSet<>();
    Deque<TypeElement> work = new ArrayDeque<>();
    work.add(te);
    while (!work.isEmpty()) {
      TypeElement c = work.poll();
      for (TypeMirror st : types.directSupertypes(c.asType())) {
        TypeElement s = typeElement(st);
        if (s != null && out.add(s)) work.add(s);
      }
    }
    return new ArrayList<>(out);
  }

  void analyzeMethods() {
    // 1. Declared roots per class; names in use per class (declared + inherited from source supertypes).
    for (TypeElement te : source) {
      Map<ExecutableElement, ExecutableElement> d = new LinkedHashMap<>();
      for (Element e : te.getEnclosedElements()) {
        if (isInstanceMethod(e)) {
          d.put(root((ExecutableElement) e), (ExecutableElement) e);
          noteExtRoot((ExecutableElement) e);
        }
      }
      declared.put(te, d);
    }
    // 2. Default-method bridges: the top-most class under each interface default method gets a bridge
    //    unless it or a superclass declares the method.
    for (TypeElement te : source) {
      if (te.getKind().isInterface()) continue;
      TypeElement sup = superclass(te);
      for (TypeElement st : allSupertypes(te)) {
        if (!st.getKind().isInterface() || !isSource(st)) continue;
        if (sup != null && types.isSubtype(erase(sup.asType()), erase(st.asType()))) continue;
        for (Element e : st.getEnclosedElements()) {
          if (!isInstanceMethod(e) || !((ExecutableElement) e).isDefault()) continue;
          ExecutableElement dm = (ExecutableElement) e;
          ExecutableElement r = root(dm);
          boolean has = false;
          for (TypeElement c = te; c != null && !has; c = superclass(c)) {
            if (isSource(c) && declared.get(c).containsKey(r)) has = true;
          }
          // A more specific default in a sub-interface wins: skip if another supertype redeclares it.
          if (!has) {
            for (TypeElement other : allSupertypes(te)) {
              if (other.equals(st) || !isSource(other) || !other.getKind().isInterface()) continue;
              if (!types.isSubtype(erase(other.asType()), erase(st.asType()))) continue;
              ExecutableElement od = declared.get(other).get(r);
              if (od != null && od.isDefault()) has = true;
            }
          }
          if (!has) {
            defaultBridges.computeIfAbsent(te, k -> new ArrayList<>()).add(dm);
            declared.get(te).put(r, dm);
          }
        }
      }
    }
    for (TypeElement te : source) {
      if (te.getKind().isInterface()) continue;
      TypeElement sup = superclass(te);
      for (ExecutableElement dm : extDefaultOverrides(te)) {
        ExecutableElement r = root(dm);
        if (sup != null && types.isSubtype(erase(sup.asType()), erase(ownerOf(dm).asType()))) continue;
        boolean has = false;
        for (TypeElement c = te; c != null && !has; c = superclass(c)) {
          if (isSource(c) && declared.get(c).containsKey(r)) has = true;
        }
        if (!has) {
          extBridges.computeIfAbsent(te, k -> new ArrayList<>()).add(dm);
          declared.get(te).put(r, dm);
          noteExtRoot(dm);
        }
      }
    }
    // 3. Name conflicts: within the members visible in one class, two groups with one Java name.
    Map<ExecutableElement, Set<ExecutableElement>> conflicts = new HashMap<>();
    for (TypeElement te : source) {
      Map<String, Set<ExecutableElement>> byName = new HashMap<>();
      List<TypeElement> chain = new ArrayList<>();
      chain.add(te);
      chain.addAll(allSupertypes(te));
      for (TypeElement c : chain) {
        if (isSource(c)) {
          for (ExecutableElement r : declared.get(c).keySet()) {
            byName.computeIfAbsent(r.getSimpleName().toString(), k -> new LinkedHashSet<>()).add(r);
          }
        }
      }
      for (Set<ExecutableElement> set : byName.values()) {
        if (set.size() < 2) continue;
        for (ExecutableElement r : set) conflicts.computeIfAbsent(r, k -> new LinkedHashSet<>()).addAll(set);
      }
    }
    for (TypeElement te : source) {
      for (ExecutableElement r : declared.get(te).keySet()) {
        if (methodNames.containsKey(r)) continue;
        methodNames.put(r, nameRoot(r, conflicts.get(r)));
      }
    }
    instanceMethodNames.addAll(methodNames.values());
    // 4. Virtual dispatch: for each class chain, a group declared concretely at two or more levels.
    for (TypeElement te : source) {
      if (te.getKind().isInterface()) continue;
      for (ExecutableElement r : declared.get(te).keySet()) {
        TypeElement top = te;
        for (TypeElement c = superclass(te); c != null && isSource(c); c = superclass(c)) {
          if (memberOf(r, c)) top = c;
        }
        if (top != te) virtualAt.computeIfAbsent(top, k -> new LinkedHashSet<>()).add(r);
      }
    }
    // Abstract declarations at the top are virtual too: there must be something to dispatch to.
    for (TypeElement te : source) {
      if (te.getKind().isInterface()) continue;
      for (var en : declared.get(te).entrySet()) {
        if (en.getValue().getModifiers().contains(Modifier.ABSTRACT) && !en.getValue().isDefault()) {
          TypeElement top = te;
          for (TypeElement c = superclass(te); c != null && isSource(c); c = superclass(c)) {
            if (memberOf(en.getKey(), c)) top = c;
          }
          virtualAt.computeIfAbsent(top, k -> new LinkedHashSet<>()).add(en.getKey());
        }
      }
    }
    // An abstract class that takes a method from an interface without declaring it, while subclasses
    // implement it: the stub lives on that class.
    for (TypeElement te : source) {
      if (te.getKind().isInterface()) continue;
      for (ExecutableElement r : declared.get(te).keySet()) {
        TypeElement top = null;
        for (TypeElement c = superclass(te); c != null && isSource(c); c = superclass(c)) {
          if (memberOf(r, c)) top = c;
        }
        if (top != null) virtualAt.computeIfAbsent(top, k -> new LinkedHashSet<>()).add(r);
      }
    }
  }

  /**
   * Default methods of library interfaces above {@code te} that implement an abstract method of a
   * super-interface (Guava's Predicate.test implementing java.util.function.Predicate.test): the Go
   * interface asks for the method, so implementers need a bridge to the default.
   */
  List<ExecutableElement> extDefaultOverrides(TypeElement te) {
    List<ExecutableElement> out = new ArrayList<>();
    Set<ExecutableElement> seen = new HashSet<>();
    List<TypeElement> all = new ArrayList<>();
    if (te.getKind().isInterface()) all.add(te);
    all.addAll(allSupertypes(te));
    for (TypeElement st : all) {
      if (isSource(st) || !st.getKind().isInterface() || kind(st.asType()) != K.IFACE) continue;
      for (Element e : st.getEnclosedElements()) {
        if (!isInstanceMethod(e) || !((ExecutableElement) e).isDefault()) continue;
        ExecutableElement dm = (ExecutableElement) e, r = root(dm);
        if (r == dm || r.isDefault() || kind(ownerOf(r).asType()) != K.IFACE) continue;
        if (seen.add(r)) out.add(dm);
      }
    }
    return out;
  }

  final Map<TypeElement, List<ExecutableElement>> extBridges = new HashMap<>();

  /** True if group {@code r} is a member of class {@code c}: c declares it or is a subtype of r's owner. */
  boolean memberOf(ExecutableElement r, TypeElement c) {
    if (isSource(c) && declared.get(c).containsKey(r)) return true;
    TypeElement owner = ownerOf(r);
    if (owner.getQualifiedName().contentEquals("java.lang.Object")) return false;
    return types.isSubtype(erase(c.asType()), erase(owner.asType()));
  }

  /** The top-most source class above-or-at {@code te} of which group r is a member. */
  TypeElement topOf(ExecutableElement r, TypeElement te) {
    TypeElement top = te;
    for (TypeElement c = superclass(te); c != null && isSource(c); c = superclass(c)) {
      if (memberOf(r, c)) top = c;
    }
    return top;
  }

  boolean isVirtual(ExecutableElement r, TypeElement te) {
    Set<ExecutableElement> s = virtualAt.get(topOf(r, te));
    return s != null && s.contains(r);
  }

  private String nameRoot(ExecutableElement r, Set<ExecutableElement> conflict) {
    String base = safeMember(r.getSimpleName().toString());
    TypeElement owner = ownerOf(r);
    if (!isSource(owner)) return externalMethodName(r);
    if (conflict == null) return base;
    boolean arityUnique = true;
    for (ExecutableElement o : conflict) {
      if (o != r && o.getParameters().size() == r.getParameters().size()) arityUnique = false;
    }
    // The external group in a conflict keeps the plain name.
    boolean anyExternal = false;
    for (ExecutableElement o : conflict) if (!isSource(ownerOf(o))) anyExternal = true;
    if (!anyExternal) {
      // The first-declared, lowest-arity group keeps the plain name when arities differ.
      ExecutableElement best = null;
      for (ExecutableElement o : conflict) {
        if (best == null || o.getParameters().size() < best.getParameters().size()) best = o;
      }
      if (best == r && arityUnique) return base;
    }
    if (arityUnique) return base + "_" + r.getParameters().size();
    return base + sigToken(r);
  }

  static String safe(String n) {
    if (n.indexOf('$') >= 0) n = n.replace("$", "_d_");
    return GO_RESERVED.contains(n) ? n + "_" : n;
  }

  static final Set<String> GO_KEYWORDS = Set.of(
      "break", "default", "func", "interface", "select", "case", "defer", "go", "map", "struct", "chan",
      "else", "goto", "package", "switch", "const", "fallthrough", "if", "range", "type", "continue",
      "for", "import", "return", "var");

  /** Names of methods and fields: only Go keywords are a problem there. */
  static String safeMember(String n) {
    if (n.indexOf('$') >= 0) n = n.replace("$", "_d_");
    return GO_KEYWORDS.contains(n) ? n + "_" : n;
  }

  /** How often each library method or constructor is used (instance methods: by group root). */
  final Map<ExecutableElement, Integer> useCount = new HashMap<>();
  /** Every type that appears as the type of an expression, plus the source classes. */
  final Set<TypeElement> seenTypes = new LinkedHashSet<>();
  private final Map<TypeElement, Set<TypeElement>> subsCache = new HashMap<>();
  private Map<ExecutableElement, String> extNames;

  void countUse(ExecutableElement e) {
    if (e.getKind() == ElementKind.METHOD && !e.getModifiers().contains(Modifier.STATIC)) {
      useCount.merge(root(e), 1, Integer::sum);
    }
    useCount.merge(e, 1, Integer::sum);
  }

  private Set<TypeElement> subs(TypeElement o) {
    return subsCache.computeIfAbsent(o, k -> {
      Set<TypeElement> out = new HashSet<>();
      TypeMirror ot = erase(o.asType());
      for (TypeElement t : seenTypes) if (types.isSubtype(erase(t.asType()), ot)) out.add(t);
      out.add(o);
      return out;
    });
  }

  private String stableKey(ExecutableElement e) {
    return ownerOf(e).getQualifiedName() + "." + e.getSimpleName() + sigToken(e);
  }

  /** Orders overloads: most used first. */
  private List<ExecutableElement> byUse(Collection<ExecutableElement> c) {
    List<ExecutableElement> l = new ArrayList<>(c);
    l.sort(Comparator.<ExecutableElement>comparingInt(e -> -useCount.getOrDefault(e, 0)).thenComparing(this::stableKey));
    return l;
  }

  static String overloadSuffix(ExecutableElement e, String sig) {
    return e.getParameters().isEmpty() ? "_0" : sig;
  }

  private void assignExternalNames() {
    extNames = new HashMap<>();
    seenTypes.addAll(source);
    for (Set<ExecutableElement> group : extRootsByName.values()) {
      List<ExecutableElement> plain = new ArrayList<>();
      for (ExecutableElement r : byUse(group)) {
        String name = safeMember(r.getSimpleName().toString());
        boolean clash = false;
        for (ExecutableElement p : plain) {
          if (!Collections.disjoint(subs(ownerOf(r)), subs(ownerOf(p)))) clash = true;
        }
        if (clash) {
          extNames.put(r, name + overloadSuffix(r, sigToken(r)));
        } else {
          extNames.put(r, name);
          plain.add(r);
        }
      }
    }
  }

  // ---- external member names

  /** How often each external overload is used; filled by a pre-pass, used to pick plain names. */
  final Map<String, Map<String, Integer>> externalUse = new HashMap<>();

  /** Library instance-method groups the program uses, by simple name (filled before naming). */
  final Map<String, Set<ExecutableElement>> extRootsByName = new HashMap<>();

  void noteExtRoot(ExecutableElement m) {
    if (m.getKind() != ElementKind.METHOD || m.getModifiers().contains(Modifier.STATIC)) return;
    ExecutableElement r = root(m);
    if (isSource(ownerOf(r))) return;
    extRootsByName.computeIfAbsent(r.getSimpleName().toString(), k -> new LinkedHashSet<>()).add(r);
  }

  String externalMethodName(ExecutableElement m) {
    ExecutableElement r = root(m);
    if (extNames == null) assignExternalNames();
    String n = extNames.get(r);
    return n != null ? n : safeMember(r.getSimpleName().toString());
  }

  boolean rootIsObject(ExecutableElement m) {
    return ownerOf(root(m)).getQualifiedName().contentEquals("java.lang.Object");
  }

  String methodName(ExecutableElement m) {
    ExecutableElement r = root(m);
    String n = methodNames.get(r);
    if (n == null) {
      n = isSource(ownerOf(r)) ? safeMember(r.getSimpleName().toString()) : externalMethodName(r);
      methodNames.put(r, n);
    }
    return n;
  }

  /** Name of the implementation of group-of-m declared in class te: m_ when dispatched through self_. */
  String implName(ExecutableElement m, TypeElement te) {
    String n = methodName(m);
    return isVirtual(root(m), te) ? n + "_impl" : n;
  }

  String staticMethodName(ExecutableElement m) {
    String n = staticNames.get(m);
    if (n != null) return n;
    TypeElement owner = ownerOf(m);
    String base = className(owner) + "_" + m.getSimpleName().toString().replace("$", "_d_");
    int same = 0;
    for (Element e : owner.getEnclosedElements()) {
      if (e.getKind() == ElementKind.METHOD && e.getSimpleName().equals(m.getSimpleName())
          && (e.getModifiers().contains(Modifier.STATIC) || !isSource(owner) || ((ExecutableElement) e).isDefault()
              || owner.getKind().isInterface() && e.getModifiers().contains(Modifier.PRIVATE))) {
        same++;
      }
    }
    if (!isSource(owner)) {
      // Helpers for library types: among the overloads the program uses, the most used has the plain name.
      List<ExecutableElement> used = new ArrayList<>();
      for (Element e : owner.getEnclosedElements()) {
        if (e.getKind() == ElementKind.METHOD && e.getSimpleName().equals(m.getSimpleName())
            && (useCount.containsKey(e) || e == m)) used.add((ExecutableElement) e);
      }
      List<ExecutableElement> sorted = byUse(used);
      n = sorted.get(0) == m ? base : base + overloadSuffix(m, sigToken(m));
      staticNames.put(m, n);
      return n;
    }
    n = same > 1 && !m.getParameters().isEmpty() ? base + sigToken(m) : base;
    if (same > 1 && m.getParameters().isEmpty() && isSource(owner)) n = base + "_0";
    staticNames.put(m, n);
    return n;
  }

  String ctorName(ExecutableElement c, String prefix) {
    TypeElement owner = ownerOf(c);
    int n = 0;
    for (Element e : owner.getEnclosedElements()) if (e.getKind() == ElementKind.CONSTRUCTOR) n++;
    String base = prefix + className(owner);
    if (n <= 1) return base;
    if (!isSource(owner) && !isExtendable(owner)) {
      List<ExecutableElement> used = new ArrayList<>();
      for (Element e : owner.getEnclosedElements()) {
        if (e.getKind() == ElementKind.CONSTRUCTOR && (useCount.containsKey(e) || e == c)) used.add((ExecutableElement) e);
      }
      if (byUse(used).get(0) == c) return base;
    }
    return base + (c.getParameters().isEmpty() ? "_0" : sigToken(c));
  }

  String fieldName(VariableElement f) {
    String n = fieldNames.get(f);
    if (n != null) return n;
    TypeElement owner = ownerOf(f);
    if (f.getModifiers().contains(Modifier.STATIC)) {
      n = className(owner) + "_" + f.getSimpleName().toString().replace("$", "_d_");
    } else {
      n = safeMember(f.getSimpleName().toString());
      if (instanceMethodNames.contains(n) || n.equals("self") || n.equals("toString") || n.equals("hashCode")
          || n.equals("equals") || n.equals("name") || n.equals("ordinal")) {
        n = n + "_";
      }
    }
    fieldNames.put(f, n);
    return n;
  }

  /** The single abstract method of a functional interface type, or null. */
  ExecutableElement functionalMethod(TypeElement te) {
    ExecutableElement found = null;
    Set<ExecutableElement> seenRoots = new HashSet<>();
    for (Element e : elems.getAllMembers(te)) {
      if (e.getKind() != ElementKind.METHOD || !e.getModifiers().contains(Modifier.ABSTRACT)) continue;
      ExecutableElement m = (ExecutableElement) e;
      if (isObjectMethod(m)) continue;
      if (!seenRoots.add(root(m))) continue;
      if (found != null) return null;
      found = m;
    }
    return found;
  }

  boolean isObjectMethod(ExecutableElement m) {
    TypeElement obj = elems.getTypeElement("java.lang.Object");
    for (Element e : obj.getEnclosedElements()) {
      if (e.getKind() == ElementKind.METHOD && e.getSimpleName().equals(m.getSimpleName())
          && e.getModifiers().contains(Modifier.PUBLIC)
          && ((ExecutableElement) e).getParameters().size() == m.getParameters().size()) {
        if (m.getParameters().isEmpty() || elems.overrides(m, (ExecutableElement) e, ownerOf(m))
            || types.isSameType(erase(m.getParameters().get(0).asType()), obj.asType())) {
          return true;
        }
      }
    }
    return false;
  }

  String goSig(List<TypeMirror> params, TypeMirror ret) {
    StringBuilder sb = new StringBuilder("(");
    for (int i = 0; i < params.size(); i++) {
      if (i > 0) sb.append(", ");
      sb.append(goType(params.get(i)));
    }
    sb.append(")");
    String r = goType(ret);
    if (!r.isEmpty()) sb.append(" ").append(r);
    return sb.toString();
  }

  void noteExternal(String goName, String desc) {
    externals.putIfAbsent(goName, desc);
  }

  /** Library types the translation names, with everything the stub generator needs. */
  final LinkedHashSet<TypeElement> extTypes = new LinkedHashSet<>();
  /** Go function name -> signature text "(params) ret". */
  final TreeMap<String, String> extFuncs = new TreeMap<>();
  /** Library instance methods: "name(sig) ret" -> root owner types. */
  final TreeMap<String, Set<TypeElement>> extMethods = new TreeMap<>();
  final TreeMap<String, String> extVars = new TreeMap<>();
  final Map<TypeElement, TreeMap<String, String>> extFields = new HashMap<>();
  final LinkedHashSet<TypeElement> extAdapters = new LinkedHashSet<>();

  void noteType(TypeElement te) {
    if (isSource(te) || !extTypes.add(te)) return;
    for (TypeMirror st : types.directSupertypes(te.asType())) {
      TypeElement s = typeElement(st);
      if (s != null && !s.getQualifiedName().contentEquals("java.lang.Object")) {
        className(s);
      }
    }
  }

  void noteFunc(String name, List<TypeMirror> params, TypeMirror ret) {
    extFuncs.putIfAbsent(name, goSig(params, ret));
  }

  /** Code calling the receiver-first helper of library default method dm from a method of group root(dm). */
  String defaultBridgeBody(ExecutableElement dm, String self) {
    ExecutableElement r = root(dm);
    List<TypeMirror> gp = groupParams(r), own = new ArrayList<>();
    for (VariableElement p : dm.getParameters()) own.add(erase(p.asType()));
    StringBuilder args = new StringBuilder(self);
    for (int i = 0; i < gp.size(); i++) args.append(", ").append(coerce("p" + i, gp.get(i), own.get(i)));
    List<TypeMirror> full = new ArrayList<>();
    full.add(erase(ownerOf(dm).asType()));
    full.addAll(own);
    String fn = staticMethodName(dm);
    noteFunc(fn, full, erase(dm.getReturnType()));
    String call = fn + "(" + args + ")";
    if (groupReturn(r).getKind() == TypeKind.VOID) return call;
    return "return " + coerce(call, erase(dm.getReturnType()), groupReturn(r));
  }

  void noteMethodOn(TypeElement owner, String name, List<TypeMirror> params, TypeMirror ret) {
    className(owner);
    extMethods.computeIfAbsent(name + goSig(params, ret), k -> new LinkedHashSet<>()).add(owner);
  }
}
