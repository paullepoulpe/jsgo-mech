package lang;

import java.util.ArrayList;
import java.util.List;

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

  public static String run() {
    p(upcasts());
    return out.toString();
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
