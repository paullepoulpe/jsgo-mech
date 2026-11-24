package j2go;

import java.util.*;
import javax.lang.model.element.*;
import javax.lang.model.type.*;

/**
 * Rule "typed library interfaces", piece `proto` (J2GO_NOLIBGEN=proto is the translation without
 * it; rule candidate T5 of docs/javaisms6/JAVAISMS6.md): the accessors of a protobuf message that
 * javac types and the runtime's protobuf API (tools/protogen) gives erased are called in a typed
 * form that protogen writes next to the erased one (its rule T5, PROTOGEN_NOT5=1 is protogen
 * without it), as jsgo's generated messages have `GetValueList() []string` and `Clone() *Requirement`:
 *
 * <pre>
 *   getFooList_t() ListT[*Foo]      // List&lt;Foo&gt; getFooList(), a repeated message field
 *   getBarList_t() ListT[string]    // ProtocolStringList getBarList(), a repeated string field
 *   toBuilder_t() *Msg_Builder      // Msg.Builder Msg.toBuilder()
 * </pre>
 *
 * on the message and its builder (getFooList_t; the program's calls through FooOrBuilder put the
 * method in that interface) and on the message (toBuilder_t). A for-each over the list runs on
 * the typed cursor, a read of an element needs no cast, and a typed place takes the list as it is.
 * A local declared {@code var} whose initializer is a call of a string list accessor, never
 * assigned again, is a List&lt;String&gt; (javac's type is ProtocolStringList, which is List&lt;String&gt;).
 *
 * Left erased (counted in J2GO_LIBGEN's report): a list of another element type (an enum, a
 * number, bytes) and the OrBuilder and Builder lists (`List&lt;? extends FooOrBuilder&gt;`).
 */
final class LibProto {
  final Ctx cx;

  LibProto(Ctx cx) {
    this.cx = cx;
  }

  static boolean on() {
    return LibGen.on("proto") && LibGen.on("list") && LibGen.on("iter");
  }

  /** A typed accessor: its Go name, its result, and the type whose method it is (for externals.tsv and the stubs). */
  record Typed(String name, TypeMirror ret, ExecutableElement noteAs) {}

  /** Whether te is a type that protoc generated: a message, its builder or its OrBuilder interface (not protobuf's own). */
  boolean generated(TypeElement te) {
    if (te == null || cx.isSource(te) || te.getQualifiedName().toString().startsWith("com.google.protobuf.")) return false;
    TypeElement mob = cx.elems.getTypeElement("com.google.protobuf.MessageLiteOrBuilder");
    return mob != null && cx.types.isSubtype(cx.types.erasure(te.asType()), cx.types.erasure(mob.asType()));
  }

  /** Whether te is a message class that protoc generated (not a builder, not an interface). */
  boolean message(TypeElement te) {
    if (!generated(te) || te.getKind() != ElementKind.CLASS) return false;
    TypeElement ml = cx.elems.getTypeElement("com.google.protobuf.MessageLite");
    return cx.types.isSubtype(cx.types.erasure(te.asType()), cx.types.erasure(ml.asType()));
  }

  /** The element type of list accessor e (String or a message class), or null where e is not one that protogen types. */
  TypeMirror listElem(ExecutableElement e) {
    String n = e.getSimpleName().toString();
    if (!e.getParameters().isEmpty() || e.getModifiers().contains(Modifier.STATIC) || !n.startsWith("get") || !n.endsWith("List")) return null;
    if (!generated(cx.ownerOf(e))) return null;
    TypeMirror r = e.getReturnType();
    if (r.getKind() != TypeKind.DECLARED) return null;
    String rq = Ctx.typeElement(r).getQualifiedName().toString();
    if (rq.equals("com.google.protobuf.ProtocolStringList")) return cx.elems.getTypeElement("java.lang.String").asType();
    if (!rq.equals("java.util.List") || n.endsWith("OrBuilderList") || n.endsWith("BuilderList")) return null;
    TypeMirror a = ((DeclaredType) r).getTypeArguments().get(0);
    return a.getKind() == TypeKind.DECLARED && message(Ctx.typeElement(a)) ? a : null;
  }

  /** Why list accessor e of a generated type stays erased (for the report), or null where e is no list accessor. */
  String leftWhy(ExecutableElement e) {
    String n = e.getSimpleName().toString();
    if (!e.getParameters().isEmpty() || !n.startsWith("get") || !n.endsWith("List") || !generated(cx.ownerOf(e))) return null;
    if (n.endsWith("OrBuilderList") || n.endsWith("BuilderList")) return "an OrBuilder or Builder list (List<? extends ..>)";
    return "the element is no message and no String";
  }

  /** The typed form of a call of library method e, or null. */
  Typed typed(ExecutableElement e) {
    if (!on() || e == null) return null;
    String n = e.getSimpleName().toString();
    if (n.equals("toBuilder") && e.getParameters().isEmpty() && message(cx.ownerOf(e)) && e.getReturnType().getKind() == TypeKind.DECLARED
        && generated(Ctx.typeElement(e.getReturnType()))) {
      if (LibGen.REPORT) cx.lg.note("proto: typed toBuilder", cx.ownerOf(e).getQualifiedName().toString());
      return new Typed("toBuilder_t", cx.erase(e.getReturnType()), e);
    }
    TypeMirror el = listElem(e);
    if (el == null) {
      String why = LibGen.REPORT ? leftWhy(e) : null;
      if (why != null) cx.lg.note("proto: list accessors left erased: " + why, cx.ownerOf(e).getSimpleName() + "." + n);
      return null;
    }
    if (LibGen.REPORT) cx.lg.note("proto: typed list accessors", cx.ownerOf(e).getSimpleName() + "." + n);
    return new Typed(cx.methodName(e) + "_t", listOf(el), cx.root(e));
  }

  /** List&lt;el&gt; as a typed place has it. */
  TypeMirror listOf(TypeMirror el) {
    return cx.erase(cx.types.getDeclaredType(cx.elems.getTypeElement("java.util.List"), el));
  }

  /** The type of a {@code var} local whose initializer is a call of e: List&lt;String&gt; for a string list accessor, else null. */
  TypeMirror varLocal(ExecutableElement e) {
    if (!on() || e == null || cx.isSource(cx.ownerOf(e))) return null;
    TypeMirror el = listElem(e);
    if (el == null || !Ctx.typeElement(e.getReturnType()).getQualifiedName().contentEquals("com.google.protobuf.ProtocolStringList")) return null;
    return cx.types.getDeclaredType(cx.elems.getTypeElement("java.util.List"), el);
  }
}
