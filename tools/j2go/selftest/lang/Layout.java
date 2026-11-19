package lang;

import java.util.Optional;

/**
 * Rule "field layout": the fields of a Go struct are written in the order of their alignment,
 * not in Java's order. Nothing a program can see changes: field initializers and constructors
 * run in Java's order, a record prints and compares its components in Java's order, a subclass
 * still holds its superclass's fields, a copy made by clone() has every field, and the fields of
 * an inner or local class (enclosing instance, captured variables) and of a generic class (fields
 * of a type parameter, instantiated with a pointer and with a handle) are all found by name.
 * selftest/run.sh also checks the sizes of these structs (SelfTestSizes).
 */
public class Layout {
  static StringBuilder out = new StringBuilder();

  static void p(Object o) {
    out.append(o).append('\n');
  }

  static int seq;

  static int next(String what) {
    out.append(what).append(' ');
    seq++;
    return seq;
  }

  /** Every alignment, in an order that pads after nearly every field. */
  static class Mixed implements Cloneable {
    boolean a = next("a") > 0;
    long b = next("b");
    byte c = (byte) next("c");
    Object d = "d" + next("d");
    short e = (short) next("e");
    String f;
    char g = (char) ('a' + next("g"));
    double h = next("h") / 2.0;
    boolean i;
    int j = next("j");
    Handles.Tag k;
    float l = next("l");
    int[] m = {next("m")};
    Optional<String> n = Optional.of("n" + next("n"));
    boolean o = true;

    Mixed(String f) {
      this.f = f + next("ctor");
      this.i = f.isEmpty();
      this.k = new Handles.Tag(f, j);
    }

    String show() {
      return a + " " + b + " " + c + " " + d + " " + e + " " + f + " " + g + " " + h + " " + i + " " + j + " " + k.text + k.weight + " " + l
          + " " + m[0] + " " + n.get() + " " + o;
    }

    Mixed copy() {
      try {
        return (Mixed) clone();
      } catch (CloneNotSupportedException ex) {
        throw new AssertionError(ex);
      }
    }
  }

  /** A subclass: its own fields are ordered among themselves, after the superclass's. */
  static class Sub extends Mixed {
    boolean p = next("p") > 0;
    String q = "q" + next("q");
    byte r;
    long s = next("s");

    Sub(String f, int r) {
      super(f);
      this.r = (byte) (r + next("subctor"));
    }

    @Override
    String show() {
      return super.show() + " | " + p + " " + q + " " + r + " " + s;
    }
  }

  /** Fields of a type parameter between narrower fields. */
  static final class Slot<K, V> {
    boolean used;
    K key;
    int hash;
    V value;
    short tag;
    Slot<K, V> nextSlot;

    Slot(K key, V value, int hash) {
      this.key = key;
      this.value = value;
      this.hash = hash;
      this.used = true;
      this.tag = (short) (hash * 3);
    }

    String show() {
      return used + " " + key + " " + hash + " " + value + " " + tag + " " + (nextSlot == null ? "-" : nextSlot.show());
    }
  }

  static String tagOf(Slot<Handles.Tag, Handles.Tag> s) {
    return s.used + " " + s.key.text + " " + s.hash + " " + s.value.weight + " " + s.tag;
  }

  record Rec(boolean flag, String name, byte small, long big, char ch, Object any, int count) {}

  /** An inner class and a local class: the enclosing instance and the captured variables are fields too. */
  final class Inner {
    boolean on = true;
    long total;
    byte step = 3;

    long bump() {
      total += step + base;
      return total;
    }
  }

  byte base = 4;

  interface Counter {
    String count();
  }

  Counter local(boolean neg, long start, byte by, String label) {
    class Local implements Counter {
      boolean done;
      int n;

      @Override
      public String count() {
        n++;
        done = n > 1;
        return label + (neg ? -(start + by * n) : start + by * n) + done + base;
      }
    }
    return new Local();
  }

  public static String run() {
    seq = 0;
    Mixed m = new Mixed("m");
    p("");
    p(m.show());
    Mixed s = new Sub("s", 5);
    p("");
    p(s.show());
    // A copy has every field, and is another object.
    Mixed c = s.copy();
    c.b = -1;
    c.f = "copy";
    c.m[0] = 99; // the array is shared
    ((Sub) c).q = "Q";
    p(c.show());
    p(s.show());
    p((c != s) + " " + (c instanceof Sub) + " " + (c.k == s.k));

    Slot<String, Integer> a = new Slot<>("one", 1, 7);
    a.nextSlot = new Slot<>("two", 2, 9);
    p(a.show());
    Slot<Handles.Tag, Handles.Tag> t = new Slot<>(new Handles.Tag("k", 1), new Handles.Tag("v", 2), 11);
    p(tagOf(t));

    Rec r1 = new Rec(true, "r", (byte) 7, 1L << 40, 'x', 2.5, 3);
    Rec r2 = new Rec(true, "r", (byte) 7, 1L << 40, 'x', 2.5, 3);
    Rec r3 = new Rec(true, "r", (byte) 7, 1L << 40, 'y', 2.5, 3);
    p(r1);
    p(r1.equals(r2) + " " + r1.equals(r3) + " " + (r1.hashCode() == r2.hashCode()) + " " + r3.ch() + r3.count() + r3.small() + r3.big());

    Layout outer = new Layout();
    Inner in = outer.new Inner();
    in.bump();
    outer.base = 10;
    p(in.bump() + " " + in.on + " " + in.step);
    Counter k = outer.local(true, 1L << 33, (byte) 2, "L");
    k.count();
    p(k.count());
    p(outer.local(false, 5, (byte) -1, "M").count());
    String r = out.toString();
    out.setLength(0);
    return r;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
