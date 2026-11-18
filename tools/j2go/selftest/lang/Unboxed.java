package lang;

import static com.google.common.base.Preconditions.checkArgument;
import static com.google.common.base.Preconditions.checkNotNull;
import static com.google.common.base.Preconditions.checkState;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableSet;
import java.util.ArrayList;
import java.util.Collection;
import java.util.List;
import java.util.Objects;
import java.util.Set;

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

  public static String run() {
    p(upcasts());
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
