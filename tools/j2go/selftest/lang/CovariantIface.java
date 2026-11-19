package lang;

import java.util.ArrayList;
import java.util.Iterator;
import java.util.List;

/**
 * Rule "covariant overrides", the interface part: an interface method that narrows the result of
 * a super-interface's method has a group of its own, and everything under the interface has a
 * method of both groups: a class that declares the method, one that narrows it again, one whose
 * superclass implements the wider interface only, an abstract class that leaves it to its subclasses, an anonymous class, an
 * enum, a lambda and a method reference (the adapter of the interface and of an interface
 * extending it), default methods on either side, two interfaces narrowing one method to one
 * type, a narrowing of a narrowing, overloads, a generic interface, a library interface; and an
 * interface that keeps one group because a class under it implements a method declared outside.
 */
public class CovariantIface {
  static StringBuilder out = new StringBuilder();

  static void p(Object o) {
    out.append(o).append('\n');
  }

  interface Wide {
    Wide next();

    String name();
  }

  interface Narrow extends Wide {
    @Override
    Narrow next();

    int rank();
  }

  /** Narrows the narrowing. */
  interface Tight extends Narrow {
    @Override
    Tight next();
  }

  /** Declares the method with the interface's result. */
  static class Link implements Narrow {
    final int n;

    Link(int n) {
      this.n = n;
    }

    @Override
    public Narrow next() {
      return n >= 3 ? null : new Link(n + 1);
    }

    @Override
    public String name() {
      return "link" + n;
    }

    @Override
    public int rank() {
      return n;
    }
  }

  /** Overrides it again in a subclass, with the same result. */
  static class Loop extends Link {
    Loop(int n) {
      super(n);
    }

    @Override
    public Narrow next() {
      return this;
    }

    @Override
    public String name() {
      return "loop" + n;
    }
  }

  /** Narrows it again in the class. */
  static final class Chain implements Narrow {
    final int n;

    Chain(int n) {
      this.n = n;
    }

    @Override
    public Chain next() {
      return new Chain(n * 2);
    }

    @Override
    public String name() {
      return "chain" + n;
    }

    @Override
    public int rank() {
      return n;
    }
  }

  // Kept on one group: a class under the narrowing interface whose method also implements a
  // method declared outside the two interfaces.
  interface Outer {
    Outer step();

    String tag();
  }

  interface Inner extends Outer {
    @Override
    Inner step();
  }

  interface Mark {
    Inner step();
  }

  static final class Marked implements Inner, Mark {
    final int n;

    Marked(int n) {
      this.n = n;
    }

    @Override
    public Inner step() {
      return n == 0 ? null : new Marked(n - 1);
    }

    @Override
    public String tag() {
      return "marked" + n;
    }
  }

  /** Implements the wider interface only; the subclass narrows and is under the narrower one. */
  static class WideOnly implements Wide {
    @Override
    public Wide next() {
      return this;
    }

    @Override
    public String name() {
      return "wideonly";
    }
  }

  static class Narrowed extends WideOnly implements Narrow {
    @Override
    public Narrow next() {
      return new Link(2);
    }

    @Override
    public String name() {
      return "narrowed";
    }

    @Override
    public int rank() {
      return 8;
    }
  }

  static final class NarrowedMore extends Narrowed {
    @Override
    public NarrowedMore next() {
      return this;
    }

    @Override
    public String name() {
      return "more";
    }
  }

  /** The wider interface first: the method reached first is not the one with its result. */
  static final class BothOrder implements Wide, Narrow {
    @Override
    public Narrow next() {
      return new Link(3);
    }

    @Override
    public String name() {
      return "both";
    }

    @Override
    public int rank() {
      return 9;
    }
  }

  /** Leaves the method to its subclasses. */
  abstract static class Shell implements Narrow {
    @Override
    public String name() {
      return "shell" + rank();
    }

    String walk() {
      Narrow n = next();
      return name() + ">" + (n == null ? "-" : n.name());
    }
  }

  static final class ShellA extends Shell {
    @Override
    public Narrow next() {
      return new Link(3);
    }

    @Override
    public int rank() {
      return 1;
    }
  }

  static final class ShellB extends Shell {
    @Override
    public ShellB next() {
      return this;
    }

    @Override
    public int rank() {
      return 2;
    }
  }

  /** Declares it abstract again, with a narrower result. */
  abstract static class Frame implements Narrow {
    @Override
    public abstract Frame next();

    @Override
    public int rank() {
      return 5;
    }
  }

  static final class FrameA extends Frame {
    @Override
    public FrameA next() {
      return null;
    }

    @Override
    public String name() {
      return "frameA";
    }
  }

  static final class Knot implements Tight {
    @Override
    public Tight next() {
      return this;
    }

    @Override
    public String name() {
      return "knot";
    }

    @Override
    public int rank() {
      return 4;
    }
  }

  enum Mode implements Narrow {
    ON,
    OFF {
      @Override
      public Narrow next() {
        return ON;
      }
    };

    @Override
    public Narrow next() {
      return OFF;
    }

    @Override
    public int rank() {
      return ordinal();
    }
  }

  // Two interfaces narrowing one method to one type.
  interface Left extends Wide {
    @Override
    Narrow next();
  }

  interface Right extends Wide {
    @Override
    Narrow next();
  }

  static final class Fork implements Left, Right {
    @Override
    public Narrow next() {
      return new Link(2);
    }

    @Override
    public String name() {
      return "fork";
    }
  }

  // Functional interfaces: lambdas and method references.
  interface Maker {
    Wide make(int k);
  }

  interface NarrowMaker extends Maker {
    @Override
    Narrow make(int k);
  }

  interface NarrowMaker2 extends NarrowMaker {}

  interface TightMaker extends NarrowMaker {
    @Override
    Tight make(int k);
  }

  static Narrow link(int k) {
    return new Link(k);
  }

  // Default methods.
  interface Picker {
    default Wide pick() {
      return new WideOnly();
    }

    default String label() {
      return "picker:" + pick().name();
    }
  }

  /** Narrows a default with an abstract method (the functional method). */
  interface NarrowPicker extends Picker {
    @Override
    Narrow pick();
  }

  /** Narrows a default with a default. */
  interface LinkPicker extends Picker {
    @Override
    default Narrow pick() {
      return new Link(0);
    }
  }

  interface Source {
    Wide open();
  }

  /** Narrows an abstract method with a default. */
  interface LinkSource extends Source {
    @Override
    default Narrow open() {
      return new Link(1);
    }

    int id();
  }

  static class DefaultPicks implements LinkPicker, LinkSource {
    @Override
    public int id() {
      return 1;
    }
  }

  static final class OwnPicks extends DefaultPicks {
    @Override
    public Narrow pick() {
      return new Chain(5);
    }

    @Override
    public Chain open() {
      return new Chain(6);
    }
  }

  // Overloads.
  interface Table {
    Wide at(int i);

    Wide at(String key);
  }

  interface NarrowTable extends Table {
    @Override
    Narrow at(int i);

    @Override
    Narrow at(String key);
  }

  static final class Grid implements NarrowTable {
    @Override
    public Narrow at(int i) {
      return new Link(i);
    }

    @Override
    public Chain at(String key) {
      return new Chain(key.length());
    }
  }

  // A generic interface.
  interface Cell<T> {
    Cell<T> tail();

    T head();
  }

  interface Stack<T> extends Cell<T> {
    @Override
    Stack<T> tail();

    int depth();
  }

  static final class Cons<T> implements Stack<T> {
    final T head;
    final Cons<T> tail;

    Cons(T head, Cons<T> tail) {
      this.head = head;
      this.tail = tail;
    }

    @Override
    public Cons<T> tail() {
      return tail;
    }

    @Override
    public T head() {
      return head;
    }

    @Override
    public int depth() {
      return tail == null ? 1 : 1 + tail.depth();
    }
  }

  static <T> String all(Cell<T> c) {
    StringBuilder sb = new StringBuilder();
    for (; c != null; c = c.tail()) {
      sb.append(c.head()).append(' ');
    }
    return sb.toString();
  }

  static <T> int depths(Stack<T> s) {
    int d = 0;
    for (; s != null; s = s.tail()) {
      d += s.depth();
    }
    return d;
  }

  // A library interface.
  interface Countdown extends Iterator<Integer> {
    int left();
  }

  interface Counted extends Iterable<Integer> {
    @Override
    Countdown iterator();
  }

  static final class Down implements Countdown {
    int n;

    Down(int n) {
      this.n = n;
    }

    @Override
    public boolean hasNext() {
      return n > 0;
    }

    @Override
    public Integer next() {
      n = n - 1;
      return n + 1;
    }

    @Override
    public int left() {
      return n;
    }
  }

  static final class From implements Counted {
    final int n;

    From(int n) {
      this.n = n;
    }

    @Override
    public Countdown iterator() {
      return new Down(n);
    }
  }

  static String wide(Wide w) {
    Wide n = w.next();
    return w.name() + "->" + (n == null ? "-" : n.name());
  }

  static String narrow(Narrow w) {
    Narrow n = w.next();
    return w.name() + "=>" + (n == null ? "-" : n.name() + "#" + n.rank());
  }

  public static String run() {
    List<Narrow> ns = new ArrayList<>();
    ns.add(new Link(1));
    ns.add(new Link(3));
    ns.add(new Loop(2));
    ns.add(new Chain(3));
    ns.add(new Narrowed());
    ns.add(new NarrowedMore());
    ns.add(new BothOrder());
    ns.add(new ShellA());
    ns.add(new ShellB());
    ns.add(new FrameA());
    ns.add(new Knot());
    ns.add(Mode.ON);
    ns.add(Mode.OFF);
    ns.add(
        new Narrow() {
          @Override
          public Narrow next() {
            return this;
          }

          @Override
          public String name() {
            return "anon";
          }

          @Override
          public int rank() {
            return 11;
          }
        });
    for (Narrow n : ns) {
      p(narrow(n) + " " + wide(n));
    }
    // Through the classes.
    Chain c = new Chain(1);
    p(c.next().next().n + " " + new Loop(5).next().rank());
    p(new NarrowedMore().next().next().name() + " " + new Narrowed().next().rank());
    WideOnly wo = new NarrowedMore();
    p(wo.next().name() + " " + new WideOnly().next().name());
    Shell sh = new ShellB();
    p(sh.walk() + " " + sh.next().rank() + " " + new ShellA().walk());
    Frame fr = new FrameA();
    p((fr.next() == null) + " " + fr.rank());
    Tight t = new Knot();
    p(t.next().next().name() + " " + narrow(t) + " " + wide(t));
    // An interface that keeps one group.
    Inner in = new Marked(2);
    Outer ou = in;
    p(in.step().tag() + " " + ou.step().tag() + " " + in.step().step().tag() + " " + (in.step().step().step() == null) + " " + new Marked(1).step().tag());
    // Two narrowings of one method.
    Fork f = new Fork();
    Left l = f;
    Right r = f;
    p(l.next().rank() + " " + r.next().rank() + " " + f.next().name() + " " + wide(l) + " " + wide(r));
    // Lambdas and method references.
    NarrowMaker nm = k -> new Link(k);
    NarrowMaker2 nm2 = k -> new Chain(k);
    NarrowMaker ref = CovariantIface::link;
    TightMaker tm = k -> new Knot();
    Maker m = nm;
    p(nm.make(2).rank() + " " + m.make(2).name() + " " + nm2.make(4).rank() + " " + ((Maker) nm2).make(4).name());
    p(ref.make(3).rank() + " " + ((Maker) ref).make(3).name());
    NarrowMaker viaTight = tm;
    p(tm.make(0).next().name() + " " + viaTight.make(0).rank() + " " + ((Maker) tm).make(0).name());
    // Default methods.
    NarrowPicker np = () -> new Link(2);
    Picker pk = np;
    p(np.pick().rank() + " " + pk.pick().name() + " " + pk.label() + " " + np.label());
    Picker plain = new Picker() {};
    p(plain.pick().name() + " " + plain.label());
    DefaultPicks dp = new DefaultPicks();
    DefaultPicks op = new OwnPicks();
    for (DefaultPicks d : new DefaultPicks[] {dp, op}) {
      Picker asPicker = d;
      Source asSource = d;
      LinkPicker lp = d;
      LinkSource ls = d;
      p(d.pick().rank() + " " + asPicker.pick().name() + " " + lp.pick().rank() + " " + d.label() + " "
          + d.open().rank() + " " + asSource.open().name() + " " + ls.open().rank() + " " + ls.id());
    }
    p(new OwnPicks().open().next().n);
    // Overloads.
    Grid g = new Grid();
    NarrowTable nt = g;
    Table tb = g;
    p(g.at(2).rank() + " " + g.at("abc").next().n + " " + nt.at(1).rank() + " " + nt.at("ab").rank() + " "
        + tb.at(3).name() + " " + tb.at("a").name());
    // A generic interface.
    Cons<String> list = new Cons<>("a", new Cons<>("b", new Cons<>("c", null)));
    Stack<String> st = list;
    Cell<String> ce = list;
    p(all(list) + all(st.tail()) + ce.tail().head() + " " + depths(st) + " " + list.tail().tail().head + " " + st.tail().depth());
    // A library interface.
    From from = new From(3);
    Counted counted = from;
    Iterable<Integer> it = from;
    int sum = 0;
    for (int v : it) {
      sum += v;
    }
    for (Integer v : counted) {
      sum += v * 10;
    }
    p(sum + " " + counted.iterator().left() + " " + from.iterator().left() + " " + it.iterator().next());
    String res = out.toString();
    out.setLength(0);
    return res;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
