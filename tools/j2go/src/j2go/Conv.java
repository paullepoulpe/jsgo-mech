package j2go;

import java.util.*;
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

  // ------------------------------------------------------------------ static upcast

  static final boolean UPCAST = !off("UPCAST");

  /** The generated functions C_as_I, by name (written to g_0casts.go). */
  final TreeMap<String, String> upcasts = new TreeMap<>();

  /** Whether code is one call of a constructor function (new_C(...)): never nil. */
  static boolean newCall(String code) {
    if (!code.startsWith("new_")) return false;
    int i = code.indexOf('('), d = 0;
    if (i < 0 || !code.substring(0, i).matches("\\w+")) return false;
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
    if (!cx.isLeaf(fe) || cx.isHandle(fe) || cx.isValueLib(fe) || !cx.types.isSubtype(from, to)) return null;
    if (cx.isSource(fe) ? !cx.implementsInGo(fe, te) : cx.isSource(te)) return null;
    if (code.equals("this") || newCall(code)) return gt + "(" + code + ")";
    String fn = gf.substring(1) + "_as_" + gt;
    upcasts.put(fn, "func " + fn + "(v " + gf + ") " + gt + " {\n\tif v == nil {\n\t\treturn nil\n\t}\n\treturn v\n}\n");
    return fn + "(" + code + ")";
  }
}
