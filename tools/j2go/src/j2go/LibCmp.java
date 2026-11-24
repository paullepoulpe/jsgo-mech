package j2go;

import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Rule "typed library interfaces", piece `cmp` (J2GO_NOLIBGEN=cmp is the translation without it;
 * rule candidate T4 of docs/javaisms6/JAVAISMS6.md): a class of the program that implements
 * java.lang.Comparable&lt;C&gt; of its own type C has the method at its Java types,
 * `compareTo_t(other *C) int32`, and the bridge with the erased signature that casts its argument
 * and calls it, as javac's class file has compareTo(C) and the bridge compareTo(Object):
 *
 * <pre>
 *   func (this *DiagnosticType) compareTo_t(diagnosticType *DiagnosticType) int32 { ..the body.. }
 *   func (this *DiagnosticType) compareTo(p0 any) int32 {
 *   	return this.compareTo_t(cast_[*DiagnosticType](p0))
 *   }
 * </pre>
 *
 * So its objects are values of the runtime's typed interface ComparableT[*C] (rt_lgcmp.go), which
 * the runtime's typed natural orderings (naturalT_ of the typed sorts, tsortCmp_ of the typed
 * sorted collections, Comparator_naturalOrder[T], Comparator_comparing[T, U]) call before they
 * box the two values for compareNatural_, which calls the bridge; erased code calls the bridge as
 * before. A call x.compareTo(y) of the program on a receiver of the class is the call of
 * compareTo_t, its argument at its type.
 *
 * The class keeps the erased method alone (refused, counted by reason in J2GO_LIBGEN's report)
 * where the typed method could be reached by another dispatch than the bridge's, or the bridge's
 * cast would not be javac's: Comparable named raw; Comparable of another type than the class
 * (a supertype, another instantiation); compareTo inherited through a superclass or an interface
 * of the program; the class has a subclass (an override would not be reached through
 * compareTo_t, which Go promotes); a handle class; compareTo also implementing a method of
 * another type (which needs the erased form itself).
 */
final class LibCmp {
  /** The Go name of the typed method. */
  static final String TYPED = "compareTo_t";

  final Ctx cx;
  /** Per class: null if typed, else why not (computed once). */
  private final Map<TypeElement, String> why = new HashMap<>();

  LibCmp(Ctx cx) {
    this.cx = cx;
  }

  static boolean on() {
    return LibGen.on("cmp");
  }

  /** Whether m is an instance compareTo of one parameter (the shape of Comparable.compareTo). */
  static boolean shape(ExecutableElement m) {
    return m.getKind() == ElementKind.METHOD && m.getSimpleName().contentEquals("compareTo") && m.getParameters().size() == 1
        && !m.getModifiers().contains(Modifier.STATIC) && !m.getModifiers().contains(Modifier.PRIVATE);
  }

  /** Whether method e of class te is the compareTo that piece `cmp` writes as compareTo_t and its bridge. */
  boolean typed(ExecutableElement e, TypeElement te) {
    if (!on() || e == null || !shape(e) || !te.equals(e.getEnclosingElement())) return false;
    return decide(te) == null && cmpMethod(te) == e;
  }

  /** Whether a call of e on a receiver of static type site is the call of compareTo_t. */
  boolean typedCall(ExecutableElement e, TypeMirror site) {
    if (!on() || e == null || !shape(e) || site == null || site.getKind() != TypeKind.DECLARED) return false;
    TypeElement te = Ctx.typeElement(cx.erase(site));
    return te != null && te.equals(e.getEnclosingElement()) && typed(e, te);
  }

  /** The class's own compareTo that implements Comparable.compareTo, or null. */
  ExecutableElement cmpMethod(TypeElement te) {
    TypeElement cmp = cx.elems.getTypeElement("java.lang.Comparable");
    ExecutableElement cm = null;
    for (Element x : cmp.getEnclosedElements()) if (x instanceof ExecutableElement xe && shape(xe)) cm = xe;
    for (Element x : te.getEnclosedElements()) {
      if (x instanceof ExecutableElement xe && shape(xe) && cx.elems.overrides(xe, cm, te)) return xe;
    }
    return null;
  }

  /** Null if class te of the program gets the typed method, else the reason it keeps the erased one (reported once). */
  String decide(TypeElement te) {
    if (why.containsKey(te)) return why.get(te);
    String w = reason(te);
    why.put(te, w);
    if (w != null && !w.isEmpty()) cx.lg.note("cmp: classes implementing Comparable left erased: " + w, te.getQualifiedName().toString());
    else if (w == null) cx.lg.note("cmp: classes implementing Comparable<C> typed (compareTo_t and the bridge)", te.getQualifiedName().toString());
    return w;
  }

  /** "" : not a class implementing Comparable at all (not counted). */
  private String reason(TypeElement te) {
    if (!cx.isSource(te) || te.getKind().isInterface()) return "";
    ExecutableElement m = cmpMethod(te);
    if (m == null) return "";
    if (m.getModifiers().contains(Modifier.ABSTRACT)) return "compareTo is abstract";
    DeclaredType named = null;
    for (TypeMirror st : te.getInterfaces()) {
      if (Ctx.typeElement(st) != null && Ctx.typeElement(st).getQualifiedName().contentEquals("java.lang.Comparable")) named = (DeclaredType) st;
    }
    if (named == null) return "Comparable through a superclass or an interface of the program";
    if (named.getTypeArguments().isEmpty()) return "raw Comparable";
    if (!cx.types.isSameType(named.getTypeArguments().get(0), te.asType())) return "Comparable of another type than the class (a supertype or another instantiation)";
    if (!cx.isLeaf(te)) return "the class has a subclass";
    if (cx.isHandle(te)) return "a handle class";
    // compareTo also implementing a method of another type than Comparable: that method's callers need the erased form
    Deque<TypeMirror> todo = new ArrayDeque<>(cx.types.directSupertypes(te.asType()));
    Set<TypeElement> seen = new HashSet<>();
    while (!todo.isEmpty()) {
      TypeElement s = Ctx.typeElement(todo.pop());
      if (s == null || !seen.add(s)) continue;
      todo.addAll(cx.types.directSupertypes(s.asType()));
      if (s.getQualifiedName().contentEquals("java.lang.Comparable")) continue;
      for (Element x : s.getEnclosedElements()) {
        if (x instanceof ExecutableElement xe && shape(xe) && cx.elems.overrides(m, xe, te)) return "compareTo also implements " + s.getSimpleName() + ".compareTo";
      }
    }
    return null;
  }
}
