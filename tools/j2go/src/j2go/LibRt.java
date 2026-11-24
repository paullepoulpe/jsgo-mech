package j2go;

import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.TypeMirror;

/**
 * Rule "typed library interfaces", piece `rtcls` (J2GO_NOLIBGEN=rtcls is the translation without
 * it; RESIDUE8's "typed runtime classes"): library classes that the runtime has twice, the erased
 * class the translation without the piece names and a generic Go struct of the class's type
 * parameters, written after the erased one method by method (the same fields, the same steps;
 * the values at their types). With the piece the translator treats each class of NAMES as a typed
 * declaration (Ctx.typedClasses, its type parameters kept) of that Go name, so a field
 * `ThreadLocal<ThreadTrace>` is `*ThreadLocalT[*Tracer_ThreadTrace]` and `traces.get()` gives the
 * ThreadTrace, where the erased class gave an Object that was cast.
 */
final class LibRt {
  /** It needs rule "function types": Ordering's compare is Comparator's at its types, CacheLoader.from takes a typed Function. */
  static boolean on() {
    return LibGen.on("rtcls") && FnTypes.ON;
  }

  /** The classes and the Go name of their typed struct (a name list: the runtime must have each). */
  static final Map<String, String> NAMES = Map.of(
      "java.lang.ThreadLocal", "ThreadLocalT",
      "com.google.common.collect.AbstractIterator", "AbstractIteratorT",
      "com.google.common.cache.CacheLoader", "CacheLoaderT",
      "com.google.common.collect.RangeSet", "RangeSetT",
      "com.google.common.collect.TreeRangeSet", "TreeRangeSetT",
      "com.google.common.collect.Range", "RangeT",
      "com.google.common.collect.Ordering", "OrderingT");

  final Ctx cx;

  LibRt(Ctx cx) {
    this.cx = cx;
  }

  void register() {
    if (!on()) return;
    // (the J2GO_LIBGEN report: the program's classes that extend or implement one, by class)
    for (TypeElement te : cx.source) {
      for (TypeMirror st : cx.types.directSupertypes(te.asType())) {
        TypeElement s = Ctx.typeElement(st);
        if (s != null && goName(s) != null) cx.lg.note("rtcls: a program class extending " + s.getSimpleName() + " (typed)", cx.elems.getBinaryName(te).toString());
      }
    }
    for (String qn : NAMES.keySet()) {
      TypeElement te = cx.elems.getTypeElement(qn);
      if (te == null) continue;
      cx.typedClasses.add(te);
      cx.keptVars.addAll(te.getTypeParameters());
      for (Element e : te.getEnclosedElements()) {
        if (e instanceof ExecutableElement m && m.getModifiers().contains(Modifier.STATIC)) cx.keptVars.addAll(m.getTypeParameters());
      }
    }
  }

  /** The Go name of library class te's typed struct, or null. */
  static String goName(TypeElement te) {
    return on() ? NAMES.get(te.getQualifiedName().toString()) : null;
  }
}
