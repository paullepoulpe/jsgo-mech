package lang;

import static com.google.common.base.Preconditions.checkArgument;
import static com.google.common.base.Preconditions.checkNotNull;
import static com.google.common.base.Preconditions.checkState;

import com.google.common.base.CharMatcher;
import com.google.common.base.Splitter;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableSet;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Objects;
import java.util.Set;
import java.util.function.Supplier;
import com.google.re2j.Matcher;
import com.google.re2j.Pattern;

/** The conversions that need no box (j2go.Conv): the values must behave as the boxed ones did. */
public class Unboxed {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  // ---- static upcast: a class without subclasses as an interface it implements
  interface Shape {
    String name();

    default String twice() {
      return name() + name();
    }
  }

  interface Sized {
    int size();
  }

  static final class Dot implements Shape, Sized {
    final List<Shape> seen = new ArrayList<>();

    @Override
    public String name() {
      return "dot";
    }

    @Override
    public int size() {
      return 1;
    }

    Shape me() {
      return this;
    }

    void register() {
      seen.add(this);
      take(this);
    }

    @Override
    public boolean equals(Object o) {
      return o instanceof Dot;
    }

    @Override
    public int hashCode() {
      return 7;
    }
  }

  static class Box implements Shape {
    @Override
    public String name() {
      return "box";
    }
  }

  static class Crate extends Box {
    @Override
    public String name() {
      return "crate";
    }
  }

  static Shape last;

  static String take(Shape s) {
    last = s;
    return s == null ? "null" : s.twice();
  }

  static Shape pick(Dot d, boolean first) {
    Shape s = d;
    Sized z = d;
    return first ? s : (z == null ? null : d);
  }

  static String upcasts() {
    Dot d = new Dot(), none = null;
    Shape s = d, n = none;
    Object o = s;
    Box b = new Crate();
    Shape viaBox = b;
    d.register();
    Shape made = new Dot();
    Sized sized = d;
    return take(d) + " " + take(none) + " " + (n == null) + " " + (s == d) + " " + (o == d) + " " + (d.me() == s) + " " + (last == null)
        + " " + take(new Dot()) + " " + viaBox.name() + " " + (d.seen.get(0) == d) + " " + (pick(none, true) == null) + " " + (pick(none, false) == null)
        + " " + (pick(d, false) == d) + " " + made.equals(d) + " " + (made == d) + " " + sized.size() + " " + (s instanceof Sized) + " " + take(b);
  }

  // ---- typed non-null checks
  static int evals;

  static <T> T counted(T v) {
    evals++;
    return v;
  }

  static String caught(Runnable r) {
    try {
      r.run();
      return "ok";
    } catch (RuntimeException e) {
      return e.getClass().getSimpleName() + ":" + e.getMessage();
    }
  }

  static String nonNull() {
    Dot d = new Dot(), none = null;
    Box b = new Crate(), noBox = null;
    Shape s = d, noShape = null;
    String str = "s", noStr = null;
    Shape viaCrate = checkNotNull(b);
    Box sameBox = checkNotNull(b, "box");
    Object o = Objects.requireNonNull(d);
    checkNotNull(counted(str));
    return (checkNotNull(d) == d) + " " + (sameBox == b) + " " + viaCrate.name() + " " + checkNotNull(s, "m %s", str).name() + " " + checkNotNull(str) + checkNotNull("lit", "x")
        + " " + (o == d) + " " + Objects.requireNonNull(str, "msg").length() + " " + Objects.requireNonNull(b, "msg").name() + " " + evals
        + " " + caught(() -> checkNotNull(none)) + " " + caught(() -> checkNotNull(noBox, "no box"))
        + " " + caught(() -> checkNotNull(noShape, "no %s of %s", "shape", noStr)) + " " + caught(() -> checkNotNull(noStr))
        + " " + caught(() -> checkNotNull(noStr, noBox)) + " " + caught(() -> checkNotNull(noStr, "%s|%s", d.name(), str))
        + " " + caught(() -> Objects.requireNonNull(noStr)) + " " + caught(() -> Objects.requireNonNull(noStr, "why"))
        + " " + caught(() -> Objects.requireNonNull(none)) + " " + caught(() -> Objects.requireNonNull(noShape, "shape"))
        + " " + caught(() -> checkNotNull(counted(noBox))) + " " + evals;
  }

  // ---- bare arguments: a literal or a new object for an Object parameter
  static String describe(Object a, Object... more) {
    StringBuilder sb = new StringBuilder(a instanceof String ? "S" : a instanceof Dot ? "D" : a instanceof Box ? "B" : "?").append(a instanceof Dot ? "dot" : a);
    for (Object m : more) sb.append(m instanceof String ? "+S" : m == null ? "+null" : "+O").append(m instanceof Dot ? "dot" : m);
    return sb.toString();
  }

  static String bare() {
    List<Object> l = new ArrayList<>();
    l.add("lit");
    l.add(new Dot());
    l.add("quote\"d \\ (paren");
    Object sameLit = "lit";
    return describe("a") + " " + describe("b", "c", new Dot(), null, "d)") + " " + describe(new Dot(), new Crate().name()) + " " + describe(new Crate().name())
        + " " + l.contains("lit") + l.contains(new Dot()) + l.indexOf("quote\"d \\ (paren") + " " + l.get(0).equals("lit") + (l.get(0) == sameLit) + " " + new Dot().equals(new Dot())
        + " " + Objects.equals("x", "x") + Objects.equals(new Dot(), "x") + " " + Objects.hash("a", new Dot(), 1);
  }

  // ---- message arguments of checks, at their own type
  static final class Tagged {
    final String tag;

    Tagged(String tag) {
      this.tag = tag;
    }

    @Override
    public String toString() {
      evals++;
      return "Tagged(" + tag + ")";
    }
  }

  static class Named extends Box {
    @Override
    public String toString() {
      return "named:" + name();
    }

    Named me() {
      return this;
    }
  }

  static final class Thrower {
    @Override
    public String toString() {
      throw new IllegalStateException("no text");
    }
  }

  static String messages() {
    Tagged t = new Tagged("t"), noTag = null;
    Box b = new Named(), noBox = null;
    String str = "s", noStr = null;
    Shape shape = new Named(), noShape = null;
    evals = 0;
    checkState(true, t);
    checkArgument(true, "%s %s", t, b);
    checkNotNull(str, "%s", t);
    String ok = "" + evals;
    Thrower thrower = new Thrower();
    String thrown = caught(() -> checkState(false, "%s!", thrower));
    return ok + " " + caught(() -> checkState(false, t)) + " " + caught(() -> checkState(false, noTag)) + " " + caught(() -> checkState(false, str))
        + " " + caught(() -> checkState(false, noStr)) + " " + caught(() -> checkArgument(false, b)) + " " + caught(() -> checkArgument(false, noBox))
        + " " + caught(() -> checkState(false, "%s|%s|%s", t, noBox, noStr)) + " " + caught(() -> checkArgument(false, "%s|%s", b, str))
        + " " + caught(() -> checkArgument(false, "%s|%s|%s", noTag, b, "lit")) + " " + caught(() -> checkState(false, "%s and more", str, b, noTag, 4))
        + " " + caught(() -> checkNotNull(noBox, "%s|%s", t, noStr)) + " " + caught(() -> checkNotNull(noStr, b))
        + " " + caught(() -> checkState(false, shape)) + " " + caught(() -> checkState(false, "%s", noShape))
        + " " + thrown.startsWith("IllegalStateException:<lang.Unboxed$Thrower@") + thrown.endsWith(" threw java.lang.IllegalStateException>!") + " " + evals;
  }

  // ---- library classes that are one Go type
  static ImmutableList<String> built(String... parts) {
    ImmutableList.Builder<String> b = ImmutableList.builder();
    for (String p : parts) b.add(p);
    return parts.length == 0 ? null : b.build();
  }

  static String oneGoType() {
    ImmutableList.Builder<String> lb = ImmutableList.builder();
    lb.add("x").add("y");
    ImmutableList<String> l = lb.build(), none = built();
    List<String> asList = lb.build(), noList = none;
    Collection<String> coll = l;
    ImmutableSet.Builder<String> sb = ImmutableSet.builder();
    Set<String> set = sb.add("q").add("q").build();
    ImmutableSet<String> iset = sb.build();
    Iterable<String> it = built("i");
    Object o = lb.build();
    return l + " " + asList + " " + (noList == null) + (none == null) + " " + coll.size() + set + iset.contains("q") + " " + it.iterator().next()
        + " " + l.equals(asList) + (o instanceof List) + asList.get(1) + built("a", "b").size();
  }

  // ---- typed Object methods: the generated members of a record, string concatenation
  enum Tone {
    LOW,
    HIGH {
      @Override
      public String toString() {
        return "high!";
      }
    }
  }

  static final class Plain {
    final int n;

    Plain(int n) {
      this.n = n;
    }

    @Override
    public String toString() {
      return "plain" + n;
    }
  }

  static final class Valued {
    final String v;

    Valued(String v) {
      this.v = v;
    }

    @Override
    public boolean equals(Object o) {
      return o instanceof Valued other && other.v.equals(v);
    }

    @Override
    public int hashCode() {
      return v.length();
    }

    @Override
    public String toString() {
      return "valued:" + v;
    }
  }

  record Mixed(Plain plain, Valued valued, Tone tone, Shape shape, Box box, List<String> list, Object any, String str, int i, long l, boolean b, double d, char c, short s) {}

  record Pair(Valued left, Valued right) {}

  static Mixed mixed(Plain p, String v, Tone t, Shape sh, Box bx, String str, int i) {
    List<String> l = new ArrayList<>();
    l.add("e" + i);
    return new Mixed(p, v == null ? null : new Valued(v), t, sh, bx, i < 0 ? null : l, i == 0 ? null : (Object) Integer.valueOf(i), str, i, 1L << 40 | i, i > 1, i / 4.0, (char) ('a' + i), (short) -i);
  }

  static String typedObjects() {
    Plain p = new Plain(1), noPlain = null;
    Named named = new Named();
    Shape shape = named, noShape = null;
    Box box = named, noBox = null;
    Tone tone = Tone.HIGH, noTone = null;
    Mixed a = mixed(p, "v", Tone.LOW, shape, box, "s", 2), same = mixed(p, "v", Tone.LOW, shape, box, "s", 2);
    Mixed nulls = mixed(null, null, null, null, null, null, -1), nulls2 = mixed(null, null, null, null, null, null, -1);
    StringBuilder eq = new StringBuilder();
    Mixed[] others = {
      mixed(new Plain(1), "v", Tone.LOW, shape, box, "s", 2), mixed(p, "w", Tone.LOW, shape, box, "s", 2), mixed(p, "v", Tone.HIGH, shape, box, "s", 2),
      mixed(p, "v", Tone.LOW, new Named(), box, "s", 2), mixed(p, "v", Tone.LOW, shape, new Named(), "s", 2), mixed(p, "v", Tone.LOW, shape, box, "t", 2),
      mixed(p, "v", Tone.LOW, shape, box, "s", 3), mixed(p, null, Tone.LOW, shape, box, "s", 2), mixed(p, "v", Tone.LOW, shape, box, null, 2), nulls
    };
    for (Mixed o : others) eq.append(a.equals(o) ? 'T' : 'F').append(o.equals(a) ? 'T' : 'F');
    Pair pair = new Pair(new Valued("ab"), null);
    List<String> typedList = new ArrayList<>(), noTypedList = null;
    typedList.add("t");
    Set<Plain> typedSet = new java.util.LinkedHashSet<>();
    typedSet.add(p);
    return a.equals(same) + " " + (a.hashCode() == same.hashCode()) + " " + nulls.equals(nulls2) + " " + (nulls.hashCode() == nulls2.hashCode()) + " " + eq
        + " " + a.equals(null) + a.equals("x") + " " + pair + " " + pair.hashCode() + " " + pair.equals(new Pair(new Valued("ab"), null)) + pair.equals(new Pair(null, null))
        + " " + new Pair(null, new Valued("abc")).hashCode() + " " + nulls + " " + a
        + " | " + p + noPlain + shape + noShape + box + noBox + tone + noTone + Tone.LOW + pair.left() + pair.right() + new Plain(7) + named.me() + a.list() + nulls.list() + a.any() + a.str() + nulls.str() + typedList + noTypedList + typedSet;
  }

  // ---- typed Object calls: equals / hashCode / toString that are Object's, Objects.equals / hashCode, String.valueOf
  enum Flat {
    ONE,
    TWO
  }

  static final class Ident {
    final int n;

    Ident(int n) {
      this.n = n;
    }
  }

  static <T> T erasedAs(Object o) {
    @SuppressWarnings("unchecked")
    T t = (T) o;
    return t;
  }

  static int order;

  static <T> T step(int expect, T v) {
    order = order * 10 + expect;
    return v;
  }

  // ---- identity functions: a static method that is `a == b` on two Objects is the comparison
  @SuppressWarnings("ReferenceEquality")
  static boolean identical(Object a, Object b) {
    return a == b;
  }

  /** Not an identity function: the call stays a call. */
  static boolean identicalOrNull(Object a, Object b) {
    return a == b || a == null;
  }

  static boolean different(Object a, Object b) {
    return a != b;
  }

  static String identity() {
    Flat one = Flat.ONE, two = Flat.TWO, noFlat = null;
    Ident i1 = new Ident(1), i2 = new Ident(1), noIdent = null;
    Dot d1 = new Dot(), d2 = new Dot();
    Shape s1 = d1, s2 = new Named(), noShape = null;
    Box b1 = new Named(), b2 = new Crate(), noBox = null;
    Named n1 = (Named) s2;
    Object o = i1, od = d1;
    String str = "str", other = "other", noStr = null;
    int[] arr = {1, 2}, arr2 = {1, 2};
    List<String> l1 = new ArrayList<>(), l2 = new ArrayList<>();
    List<Object> erased = new java.util.LinkedList<>();
    erased.add(one);
    erased.add(i1);
    List<Ident> idents = erasedAs(erased);
    String same = identical(one, one) + "" + identical(one, two) + identical(noFlat, one) + identical(noFlat, noFlat) + identical(one, null) + identical(null, noFlat) + identical(null, null)
        + " " + identical(i1, i1) + identical(i1, i2) + identical(i1, noIdent) + identical(noIdent, noIdent) + identical(i1, o) + identical(o, i1) + identical(o, o) + identical(o, od)
        // equal by equals, not identical
        + " " + identical(d1, d1) + identical(d1, d2) + identical(s1, d1) + identical(d1, s1) + identical(s1, s2) + identical(s1, noShape) + identical(noShape, noShape) + identical(od, s1)
        // a class with subclasses, seen at two static types
        + " " + identical(b1, b1) + identical(b1, b2) + identical(s2, n1) + identical(n1, s2) + identical((Box) n1, n1) + identical(n1, (Box) n1) + identical(b1, noBox) + identical(noBox, noShape)
        + " " + identical(str, str) + identical(str, other) + identical(str, noStr) + identical(noStr, noStr) + identical(noStr, null) + identical(str, o)
        + " " + identical(arr, arr) + identical(arr, arr2) + identical(l1, l1) + identical(l1, l2) + identical(l1, (Object) l1) + identical(l1, null)
        // not what its static type says: never cast
        + " " + identical(idents.get(0), i1) + identical(i1, idents.get(1)) + identical(idents.get(0), one)
        + " " + identical(1, 1) + identical('c', 'c') + identical(true, false) + identical(str.length(), 3);
    String others = identicalOrNull(i1, i1) + "" + identicalOrNull(i1, i2) + identicalOrNull(noIdent, i2) + different(i1, i2) + different(i1, i1) + different(str, noStr);
    order = 0;
    String evals = identical(step(1, i1), step(2, i2)) + "" + identical(step(3, s1), step(4, d1)) + identical(step(5, str), step(6, noStr)) + order;
    return same + " | " + others + " | " + evals;
  }

  static String objectCalls() {
    Flat one = Flat.ONE, two = Flat.TWO, noFlat = null;
    Tone low = Tone.LOW, high = Tone.HIGH, noTone = null;
    Ident i1 = new Ident(1), i2 = new Ident(1), noIdent = null;
    Dot d1 = new Dot(), d2 = new Dot(), noDot = null;
    Shape s1 = d1, s2 = new Named(), noShape = null;
    Object o = one, oi = i1;
    StringBuilder sb = new StringBuilder("sb"), noSb = null;
    java.math.BigInteger b1 = java.math.BigInteger.valueOf(12), b2 = new java.math.BigInteger("12"), noBig = null;
    List<Object> erased = new java.util.LinkedList<>();
    erased.add(one);
    erased.add("str");
    erased.add(i1);
    List<Flat> flats = erasedAs(erased);
    List<Ident> idents = erasedAs(erased);
    List<String> strs = erasedAs(erased);
    String str = "str", same = new StringBuilder("st").append('r').toString(), noStr = null;
    String eq = one.equals(one) + "" + one.equals(two) + one.equals(noFlat) + one.equals(o) + one.equals("ONE") + Flat.TWO.equals(two) + two.equals(Flat.ONE)
        + " " + low.equals(low) + low.equals(high) + high.equals(Tone.HIGH) + high.equals(noTone)
        + " " + i1.equals(i1) + i1.equals(i2) + i1.equals(noIdent) + i1.equals(oi) + i1.equals(str)
        + " " + s1.equals(s1) + s1.equals(d2) + s2.equals(s2) + s2.equals(s1) + s1.equals(noShape) + s2.equals(noShape)
        + " " + b1.equals(b2) + b1.equals(noBig) + b1.equals(str) + b1.equals(b1.add(b2))
        // an argument that is not what its static type says is never cast: equals takes an Object
        + " " + one.equals(flats.get(0)) + one.equals(flats.get(1)) + i1.equals(idents.get(1)) + i1.equals(idents.get(2)) + s1.equals(erased.get(1));
    String hashes = (i1.hashCode() == i1.hashCode()) + "" + (one.hashCode() == one.hashCode()) + (b1.hashCode() == b2.hashCode()) + b1.hashCode() + (s1.hashCode() == d2.hashCode()) + s1.hashCode()
        + (sb.hashCode() == sb.hashCode());
    String strings = one.toString() + two + low.toString() + high.toString() + sb.toString() + b1.toString() + i1.toString().startsWith("lang.Unboxed$Ident@") + s2.toString() + s1.toString().startsWith("lang.Unboxed$Dot@");
    String statics = Objects.equals(str, same) + "" + Objects.equals(str, noStr) + Objects.equals(noStr, noStr) + Objects.equals(noStr, str) + Objects.equals(str, "st")
        + " " + Objects.equals(one, one) + Objects.equals(one, two) + Objects.equals(noFlat, one) + Objects.equals(noFlat, noFlat) + Objects.equals(one, o) + Objects.equals(o, str)
        + " " + Objects.equals(i1, i1) + Objects.equals(i1, i2) + Objects.equals(noIdent, noIdent) + Objects.equals(d1, d2) + Objects.equals(d1, noDot) + Objects.equals(noDot, d1)
        + " " + Objects.equals(s1, s2) + Objects.equals(s1, d2) + Objects.equals(noShape, noShape) + Objects.equals(b1, b2) + Objects.equals(b1, noBig) + Objects.equals(low, high)
        + " " + Objects.equals(strs.get(1), str) + Objects.equals(str, strs.get(0)) + Objects.equals(flats.get(1), one) + Objects.equals(flats.get(0), one) + Objects.equals(idents.get(1), idents.get(1))
        + " " + Objects.hashCode(str) + Objects.hashCode(noStr) + Objects.hashCode(d1) + Objects.hashCode(noDot) + Objects.hashCode(b1) + Objects.hashCode(noBig) + Objects.hashCode(noShape) + Objects.hashCode(s1)
        + (Objects.hashCode(i1) == i1.hashCode()) + Objects.hashCode(noFlat) + Objects.hashCode(strs.get(1))
        + " " + String.valueOf(one) + String.valueOf(noFlat) + String.valueOf(high) + String.valueOf(d1 == null ? null : sb) + String.valueOf(noSb) + String.valueOf(b1) + String.valueOf(noBig)
        + String.valueOf(noShape) + String.valueOf(noIdent) + String.valueOf((Object) str) + String.valueOf(flats.get(1));
    String nulls = thrown(() -> noFlat.equals(one)) + " " + thrown(() -> noFlat.equals(noFlat)) + " " + thrown(() -> noTone.equals(low)) + " " + thrown(() -> noIdent.equals(noIdent))
        + " " + thrown(() -> noShape.equals(s1)) + " " + thrown(() -> noBig.equals(b1)) + " " + thrown(() -> noBig.equals(str)) + " " + thrown(() -> noFlat.hashCode())
        + " " + thrown(() -> noIdent.hashCode()) + " " + thrown(() -> noShape.hashCode()) + " " + thrown(() -> noBig.hashCode()) + " " + thrown(() -> noFlat.toString())
        + " " + thrown(() -> noSb.toString()) + " " + thrown(() -> noIdent.toString()) + " " + thrown(() -> noShape.toString()) + " " + thrown(() -> noBig.toString());
    // the receiver and the argument are evaluated once, in order, before a null receiver throws
    order = 0;
    String evals = step(1, one).equals(step(2, two)) + "" + step(3, i1).equals(step(4, oi)) + Objects.equals(step(5, str), step(6, same)) + thrown(() -> step(7, noFlat).equals(step(8, one))) + step(9, sb).toString() + order;
    return eq + " | " + hashes + " | " + strings + " | " + statics + " | " + nulls + " | " + evals;
  }

  // ---- implicit conversions: a primitive, an interface value or an array where Go declares `any`
  static String kindOf(Object o) {
    return o == null ? "null" : o instanceof Integer ? "int" : o instanceof Long ? "long" : o instanceof Character ? "char" : o instanceof Double ? "double" : o instanceof Boolean ? "bool"
        : o instanceof Short ? "short" : o instanceof Byte ? "byte" : o instanceof Float ? "float" : o instanceof Shape ? "shape" : o instanceof int[] ? "ints" : o instanceof List ? "list" : o instanceof String ? "str" : "?";
  }

  static String kindsOf(Object first, Object... rest) {
    StringBuilder sb = new StringBuilder(kindOf(first));
    for (Object o : rest) sb.append(',').append(kindOf(o));
    return sb.toString();
  }

  static <T> String generic(T a, Object b) {
    return kindOf(a) + "/" + kindOf(b);
  }

  static Object asObject(int kind, Shape s, int[] arr) {
    switch (kind) {
      case 0:
        return 7;
      case 1:
        return s;
      case 2:
        return arr;
      case 3:
        return 'c';
      default:
        return kind > 4 ? 2.5 : (Object) 5L;
    }
  }

  static String implicitAny() {
    int i = 3;
    long l = 4L;
    char c = 'x';
    short sh = 2;
    byte by = 1;
    float f = 1.5f;
    double d = 0.25;
    boolean b = true;
    int[] arr = {1, 2};
    Shape shape = new Dot(), noShape = null;
    Iterable<String> words = built("u", "v");
    List<Object> objs = new java.util.LinkedList<>();
    objs.add(i);
    objs.add(l);
    objs.add(c);
    objs.add(shape);
    objs.add(noShape);
    objs.add(arr);
    objs.add(d);
    java.util.Map<String, Object> m = new java.util.TreeMap<>();
    m.put("i", i + 1);
    m.put("s", shape);
    m.put("b", !b);
    StringBuilder each = new StringBuilder();
    for (Object o : objs) each.append(kindOf(o)).append(';');
    for (String w : words) each.append(w);
    Supplier<Object> sup = () -> i * 2;
    java.util.function.Function<Shape, Object> fn = x -> x;
    StringBuilder ret = new StringBuilder();
    for (int k = 0; k < 6; k++) ret.append(kindOf(asObject(k, shape, arr))).append(k == 1 ? asObject(k, noShape, arr) == null : "").append(' ');
    return kindOf(i) + kindOf(l) + kindOf(c) + kindOf(sh) + kindOf(by) + kindOf(f) + kindOf(d) + kindOf(b) + kindOf(arr) + kindOf(shape) + kindOf(noShape) + kindOf(words) + kindOf(i + l) + kindOf(c + 1) + kindOf(1 << 20) + kindOf('q')
        + " " + kindsOf(i, l, shape, noShape, arr, 5, 'z', 1.0, b, "s") + " " + kindsOf(shape) + " " + generic(i, shape) + generic(shape, c) + Unboxed.<Object>generic(l, arr)
        + " " + each + " " + kindOf(m.get("i")) + kindOf(m.get("s")) + kindOf(m.get("b")) + m.get("i") + " " + String.format("%d %s %c %s", i, l, c, b) + " " + Objects.hash(i, c, b)
        + " " + sup.get() + kindOf(sup.get()) + kindOf(fn.apply(shape)) + (fn.apply(noShape) == null) + " " + ret + (i + "" + shape.equals(shape) + objs.contains(l) + objs.contains(4) + objs.indexOf(c));
  }

  // ---- a String for a CharSequence parameter
  static final class Shout implements Appendable {
    final StringBuilder sb = new StringBuilder();

    @Override
    public Appendable append(CharSequence s) {
      sb.append(s == null ? "<null>" : "<" + s + ">");
      return this;
    }

    @Override
    public Appendable append(CharSequence s, int a, int b) {
      sb.append(s, a, b);
      return this;
    }

    @Override
    public Appendable append(char c) {
      sb.append(c);
      return this;
    }
  }

  static String appendTo(Appendable out, String a, String b) {
    try {
      out.append(a).append(b).append("lit").append(a, 1, 2);
    } catch (java.io.IOException e) {
      throw new RuntimeException(e);
    }
    return out instanceof Shout sh ? sh.sb.toString() : out.toString();
  }

  static String thrown(Runnable r) {
    try {
      r.run();
      return "none";
    } catch (RuntimeException e) {
      return e.getClass().getSimpleName();
    }
  }

  static String strSeqs() {
    String s = "a-b-c", dash = "-", none = null;
    CharSequence seq = dash;
    Pattern p = Pattern.compile("b+");
    Matcher m = p.matcher(s);
    boolean found = m.find();
    m.reset("xbbx");
    List<String> parts = new ArrayList<>();
    parts.add("p");
    parts.add(null);
    parts.add("q");
    return s.contains(dash) + "" + s.contains(seq) + s.contains("z") + " " + s.replace(dash, "+") + s.replace("a", dash) + s.replace(seq, dash)
        + " " + found + m.find() + m.group(0) + p.matcher(dash).find() + Pattern.matches("a.*", s)
        + " " + Splitter.on('-').splitToList(s) + Splitter.on('-').split(s).iterator().next() + " " + CharMatcher.anyOf(dash).matchesAllOf("--") + CharMatcher.is('a').matchesAllOf(s)
        + " " + String.join(dash, parts) + " " + appendTo(new StringBuilder(), s, none) + " " + appendTo(new Shout(), s, none) + " " + appendTo(new Shout(), "xyz", dash)
        + " " + thrown(() -> s.contains(none)) + " " + thrown(() -> s.replace(none, dash)) + " " + thrown(() -> s.replace(dash, none)) + " " + thrown(() -> p.matcher(none))
        + " " + thrown(() -> m.reset(none)) + " " + thrown(() -> Splitter.on('-').splitToList(none)) + " " + thrown(() -> String.join(none, parts))
        + " " + thrown(() -> CharMatcher.anyOf(none)) + " " + thrown(() -> Pattern.matches("a", none));
  }

  public static String run() {
    p(upcasts());
    p(strSeqs());
    p(typedObjects());
    p(objectCalls());
    p(identity());
    p(implicitAny());
    p(nonNull());
    p(bare());
    p(messages());
    p(oneGoType());
    return out.toString();
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
