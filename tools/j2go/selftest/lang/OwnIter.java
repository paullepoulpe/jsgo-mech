package lang;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.EnumSet;
import java.util.Iterator;
import java.util.List;

/**
 * Piece `owniter` of rule "typed library interfaces" (rule candidate T6 of
 * docs/javaisms6/JAVAISMS6.md): a for-each loop over a class of the program that has the typed
 * Iterable methods itself (it names Iterable<X> among its interfaces and has no subclass: Words,
 * Countdown, Bag, as Node.AncestorIterable, ReferenceCollection, SimpleMultiset) runs on the
 * typed cursor (eachOf_), and one over the set of an EnumSet maker of kind `enumset` (allOf,
 * noneOf, of(e), of(e, rest...), copyOf) on the typed set's cursor. Left erased, counted by
 * reason: a class whose Iterable comes through a library superclass (Row extends ArrayList), a
 * class with a subclass (Base), a class that is Iterable through an interface of the program
 * (SeqImpl), a loop over that interface (Seq), EnumSet.of(e, rest...) called with its rest as
 * separate arguments (a variable-arity call). (EnumSet's other overloads, of(a, b) and range,
 * are not in the runtime.) selftest/run.sh counts the cursors in g_OwnIter.go.
 */
public class OwnIter {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  enum Color {
    RED,
    GREEN,
    BLUE,
    BLACK
  }

  /** Typed: names Iterable<String>, no subclass; its iterator is the list's. */
  static final class Words implements Iterable<String> {
    final List<String> items = new ArrayList<>();

    Words add(String s) {
      items.add(s);
      return this;
    }

    @Override
    public Iterator<String> iterator() {
      return items.iterator();
    }
  }

  /** Typed: an iterator class of its own that changes the object (as Node.AncestorIterable). */
  static final class Countdown implements Iterable<String> {
    int n;

    Countdown(int n) {
      this.n = n;
    }

    @Override
    public Iterator<String> iterator() {
      return new Iterator<String>() {
        @Override
        public boolean hasNext() {
          return n > 0;
        }

        @Override
        public String next() {
          String s = "c" + n;
          n--;
          return s;
        }
      };
    }
  }

  /** Typed: generic (as SimpleMultiset<E>). */
  static final class Bag<E> implements Iterable<E> {
    final List<E> items = new ArrayList<>();

    void add(E e, int times) {
      for (int i = 0; i < times; i++) items.add(e);
    }

    @Override
    public Iterator<E> iterator() {
      return items.iterator();
    }
  }

  /** Left: Iterable through a library superclass. */
  static final class Row extends ArrayList<String> {}

  /** Left: the class has a subclass. */
  static class Base implements Iterable<String> {
    @Override
    public Iterator<String> iterator() {
      return Arrays.asList("b1", "b2").iterator();
    }
  }

  static final class Derived extends Base {
    @Override
    public Iterator<String> iterator() {
      return Arrays.asList("d1").iterator();
    }
  }

  /** Left: Iterable through an interface of the program (a loop over the interface is left too). */
  interface Seq extends Iterable<String> {}

  static final class SeqImpl implements Seq {
    @Override
    public Iterator<String> iterator() {
      return Arrays.asList("s1", "s2", "s3").iterator();
    }
  }

  static Countdown countdown(int n) {
    return new Countdown(n);
  }

  static void typedLoops() {
    Words w = new Words().add("a").add("b").add("c");
    StringBuilder sb = new StringBuilder();
    for (String s : w) sb.append(s).append(s.length());
    p("words " + sb);
    StringBuilder cs = new StringBuilder();
    for (String i : countdown(4)) cs.append(i);
    p("countdown " + cs);
    Countdown c = new Countdown(3);
    for (String i : c) {
      if (i.equals("c2")) break;
      p("cd " + i);
    }
    // the cursor took the object's iterator once: the rest of the countdown is left in it
    p("left " + c.n);
    Bag<String> bag = new Bag<>();
    bag.add("x", 2);
    bag.add("y", 1);
    for (String s : bag) p("bag " + s);
    Bag<Color> colors = new Bag<>();
    colors.add(Color.GREEN, 2);
    for (Color k : colors) p("bag " + k + " " + k.ordinal());
    Words none = null;
    try {
      for (String s : none) p("never " + s);
    } catch (NullPointerException e) {
      p("null words: NullPointerException");
    }
    for (Color k : EnumSet.allOf(Color.class)) p("allOf " + k);
    for (Color k : EnumSet.noneOf(Color.class)) p("noneOf " + k);
    for (Color k : EnumSet.of(Color.BLUE)) p("of " + k);
    for (Color k : EnumSet.of(Color.BLACK, new Color[] {Color.RED, Color.BLUE})) p("ofArr " + k);
    for (Color k : EnumSet.copyOf(Arrays.asList(Color.BLUE, Color.RED))) p("copyOf " + k);
  }

  static void leftLoops() {
    Row r = new Row();
    r.add("r1");
    r.add("r2");
    for (String s : r) p("row " + s);
    Base b = new Derived();
    for (String s : b) p("base " + s);
    SeqImpl si = new SeqImpl();
    for (String s : si) p("seqImpl " + s);
    Seq sq = si;
    for (String s : sq) p("seq " + s);
    for (Color k : EnumSet.of(Color.BLACK, Color.RED, Color.BLUE, Color.RED, Color.GREEN, Color.BLACK)) p("ofVar " + k);
  }

  public static String run() {
    typedLoops();
    leftLoops();
    return out.toString();
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
