package lang;

import java.util.function.Consumer;
import java.util.function.Supplier;

/**
 * Piece `nbox` (LibNbox): fields and parameters declared as a boxed primitive, and function
 * parameters with a boxed type argument, hold an Nbox[T] (a primitive and a flag; the zero value
 * is null). The fields tri, year, flag and seen, the parameters of setYear and setTri, and opt's
 * and num's function parameters with the lambdas given for them are typed; the others are left,
 * each for one reason.
 */
public class TriBox {
  static Boolean tri;
  Integer year = 0;
  Boolean flag = true;
  Integer seen;
  boolean plain;
  int count;

  // left: a method called on it
  Boolean called = true;
  // left: ++
  Integer counter = 0;
  // left: a string concatenation
  Integer shown = 5;
  // left: == with another box
  Integer a1 = 1000;
  Integer a2;

  TriBox(Integer init) { // left: a constructor parameter
    seen = init;
  }

  void setYear(Integer y) {
    year = y;
  }

  static void setTri(Boolean b) {
    tri = b;
  }

  static String opt(Supplier<Boolean> get, Consumer<Boolean> set) {
    String d = String.valueOf(get.get());
    set.accept(true);
    set.accept(false);
    set.accept(true);
    return d;
  }

  static String num(Supplier<Integer> get, Consumer<Integer> set) {
    Integer v = Integer.valueOf(7);
    String d = String.valueOf(get.get());
    set.accept(v);
    set.accept(3);
    return d;
  }

  // left: given a method reference
  static void each(Consumer<Integer> c) {
    c.accept(4);
  }

  // left: passed on as a value
  static void pass(Consumer<Boolean> c) {
    opt2(c);
  }

  static void opt2(Consumer<Boolean> c) { // left: given something else than a lambda (pass's c)
    c.accept(true);
  }

  static class Base {
    void set(Integer x) { // left: overridden
      sink++;
    }
  }

  static class Sub extends Base {
    @Override
    void set(Integer x) { // left: overrides
      sink++;
    }
  }

  public static void main(String[] args) {
    System.out.print(run());
  }

  static int sink;

  public static String run() {
    StringBuilder sb = new StringBuilder("Tri\n");
    sb.append(tri == null).append(' ').append(String.valueOf(tri)).append(' ');
    try {
      boolean x = tri;
      sb.append(x);
    } catch (NullPointerException e) {
      sb.append("npe");
    }
    sb.append('\n');
    setTri(true);
    if (tri) sb.append("on ");
    setTri(null);
    sb.append(tri != null).append(' ');
    setTri(false);
    sb.append(!tri).append('\n');
    TriBox t = new TriBox(null);
    sb.append(t.seen == null).append(' ').append(t.year != 0).append(' ').append(t.flag).append('\n');
    t.setYear(2020);
    sb.append(t.year > 2000).append(' ').append(t.year + 1).append(' ');
    t.setYear(null);
    sb.append(t.year == null).append('\n');
    sb.append(opt(() -> t.plain, v -> {
      t.plain = v;
    })).append(' ').append(t.plain).append(' ');
    sb.append(opt(() -> t.flag, v -> {
      t.flag = v;
    })).append(' ').append(t.flag).append(' ');
    sb.append(opt(() -> tri, v -> tri = v)).append(' ').append(tri).append('\n');
    t.year = null;
    sb.append(num(() -> t.year, v -> {
      t.year = v;
    })).append(' ').append(t.year).append(' ');
    sb.append(num(() -> t.count, v -> {
      t.count += v;
    })).append(' ').append(t.count).append('\n');
    each(TriBox::record);
    pass(v -> sb.append(v).append(' '));
    sb.append(sink).append(' ').append(t.called.booleanValue()).append(' ');
    t.counter++;
    sb.append(t.counter).append(' ').append("shown " + t.shown).append(' ');
    t.a2 = t.a1;
    sb.append(t.a1 == t.a2).append(' ');
    sb.append('\n');
    new Sub().set(1);
    new Base().set(null);
    return sb.toString();
  }

  static void record(Integer v) {
    sink += v;
  }
}
