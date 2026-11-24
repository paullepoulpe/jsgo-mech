package lang;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.Comparator;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;

/**
 * Rule "typed library interfaces", piece `cmp` (LibCmp): classes implementing Comparable of their
 * own type have compareTo_t and the bridge compareTo(Object); the runtime's typed natural orders
 * (typed sorts, typed sorted sets and maps, Comparator.naturalOrder, Comparator.comparing) call
 * compareTo_t, erased code the bridge. Ver and Box are typed; the others keep the erased method
 * (raw Comparable, Comparable of a supertype, a class with a subclass and its subclass, Comparable
 * through an interface of the program, compareTo also implementing another interface's method:
 * tools/j2go/selftest/run.sh checks that exactly two classes have compareTo_t). Must print what
 * Java prints, with the piece on and off.
 */
public class Cmp {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  /** Typed. */
  static final class Ver implements Comparable<Ver> {
    final int major;
    final String tag;

    Ver(int major, String tag) {
      this.major = major;
      this.tag = tag;
    }

    @Override
    public int compareTo(Ver o) {
      if (major != o.major) return Integer.compare(major, o.major);
      return tag.compareTo(o.tag);
    }

    @Override
    public String toString() {
      return major + tag;
    }
  }

  /** Typed: a generic class, Comparable of its own instantiation. */
  static final class Box<T> implements Comparable<Box<T>> {
    final int rank;
    final T value;

    Box(int rank, T value) {
      this.rank = rank;
      this.value = value;
    }

    @Override
    public int compareTo(Box<T> o) {
      return rank - o.rank;
    }

    @Override
    public String toString() {
      return rank + "=" + value;
    }
  }

  /** Refused: raw Comparable. */
  @SuppressWarnings("rawtypes")
  static final class Raw implements Comparable {
    final int n;

    Raw(int n) {
      this.n = n;
    }

    @Override
    public int compareTo(Object o) {
      return n - ((Raw) o).n;
    }

    @Override
    public String toString() {
      return "r" + n;
    }
  }

  static class Parent {
    final int n;

    Parent(int n) {
      this.n = n;
    }
  }

  /** Refused: Comparable of a supertype. */
  static final class Kid extends Parent implements Comparable<Parent> {
    Kid(int n) {
      super(n);
    }

    @Override
    public int compareTo(Parent o) {
      return o.n - n;
    }

    @Override
    public String toString() {
      return "k" + n;
    }
  }

  /** Refused: the class has a subclass. */
  static class Base implements Comparable<Base> {
    final int n;

    Base(int n) {
      this.n = n;
    }

    @Override
    public int compareTo(Base o) {
      return n - o.n;
    }

    @Override
    public String toString() {
      return "b" + n;
    }
  }

  /** Refused: Comparable through a superclass (and it overrides compareTo). */
  static final class Derived extends Base {
    Derived(int n) {
      super(n);
    }

    @Override
    public int compareTo(Base o) {
      return o.n - n;
    }

    @Override
    public String toString() {
      return "d" + n;
    }
  }

  interface Named extends Comparable<Named> {
    String name();
  }

  /** Refused: Comparable through an interface of the program. */
  static final class NamedImpl implements Named {
    final String name;

    NamedImpl(String name) {
      this.name = name;
    }

    @Override
    public String name() {
      return name;
    }

    @Override
    public int compareTo(Named o) {
      return name.compareTo(o.name());
    }

    @Override
    public String toString() {
      return name;
    }
  }

  interface Ord<T> {
    int compareTo(T o);
  }

  /** Refused: compareTo also implements Ord.compareTo, whose erased form takes an Object. */
  static final class Both implements Comparable<Both>, Ord<Both> {
    final int n;

    Both(int n) {
      this.n = n;
    }

    @Override
    public int compareTo(Both o) {
      return n - o.n;
    }

    @Override
    public String toString() {
      return "o" + n;
    }
  }

  static int viaOrd(Ord<Both> o, Both b) {
    return o.compareTo(b);
  }

  @SafeVarargs
  static <T> List<T> listOf(T... xs) {
    List<T> l = new ArrayList<>();
    for (T x : xs) l.add(x);
    return l;
  }

  static List<Ver> vers() {
    List<Ver> l = new ArrayList<>();
    l.add(new Ver(2, "b"));
    l.add(new Ver(1, "z"));
    l.add(new Ver(2, "a"));
    l.add(new Ver(1, "a"));
    return l;
  }

  /** A direct call on the class: compareTo_t (run.sh counts one). */
  static int direct(Ver a, Ver b) {
    return a.compareTo(b);
  }

  /** Through Comparable and Object: the bridge. */
  @SuppressWarnings({"unchecked", "rawtypes"})
  static int erased(Comparable<Ver> a, Object b) {
    return a.compareTo((Ver) b) + ((Comparable) a).compareTo(b);
  }

  static String run1() {
    List<Ver> l = vers();
    Collections.sort(l);
    p("Collections.sort " + l);
    l = vers();
    l.sort(null);
    p("List.sort(null) " + l);
    l = vers();
    l.sort(Comparator.naturalOrder());
    p("naturalOrder " + l);
    l = vers();
    l.sort(Comparator.<Ver>naturalOrder().reversed());
    p("reversed " + l);
    List<Box<Ver>> bl = new ArrayList<>();
    bl.add(new Box<>(3, new Ver(1, "x")));
    bl.add(new Box<>(1, new Ver(2, "y")));
    bl.add(new Box<>(2, new Ver(3, "z")));
    bl.sort(Comparator.comparing(b -> b.value));
    p("comparing " + bl);
    Collections.sort(bl);
    p("Box sort " + bl);
    Set<Ver> ts = new TreeSet<>(vers());
    ts.add(new Ver(0, "q"));
    p("TreeSet " + ts + " " + ts.contains(new Ver(2, "a")) + " " + ts.contains(new Ver(2, "c")));
    Map<Ver, String> tm = new TreeMap<>();
    for (Ver v : vers()) tm.put(v, v.tag);
    tm.put(new Ver(1, "a"), "again");
    p("TreeMap " + tm);
    Object[] arr = vers().toArray();
    Arrays.sort(arr);
    StringBuilder sb = new StringBuilder();
    for (Object o : arr) sb.append(o).append(' ');
    p("Arrays.sort " + sb);
    p("direct " + direct(new Ver(1, "a"), new Ver(1, "b")) + " " + direct(new Ver(3, "a"), new Ver(1, "b")));
    p("erased " + erased(new Ver(1, "a"), new Ver(1, "b")));
    try {
      erased(new Ver(1, "a"), "not a Ver");
      p("no exception");
    } catch (ClassCastException e) {
      p("ClassCastException");
    }
    List<Raw> rl = listOf(new Raw(3), new Raw(1), new Raw(2));
    Collections.sort(rl);
    p("Raw " + rl);
    List<Kid> kl = listOf(new Kid(1), new Kid(3), new Kid(2));
    Collections.sort(kl);
    Set<Kid> ks = new TreeSet<>();
    for (Kid k : kl) ks.add(k);
    p("Kid " + kl + " " + ks);
    List<Base> basel = listOf(new Base(2), new Base(3), new Base(1));
    Collections.sort(basel);
    List<Derived> dl = listOf(new Derived(2), new Derived(5), new Derived(3));
    Collections.sort(dl);
    Base d = new Derived(5);
    p("Base " + basel + " " + dl + " " + d.compareTo(new Base(1)) + " " + new Base(1).compareTo(d));
    List<NamedImpl> nl = listOf(new NamedImpl("c"), new NamedImpl("a"), new NamedImpl("b"));
    Collections.sort(nl);
    p("Named " + nl);
    List<Both> ol = new ArrayList<>();
    ol.add(new Both(2));
    ol.add(new Both(1));
    Collections.sort(ol);
    // (not called through Ord: an object of Both given for Ord<Both> fails its cast in Go, with the
    // piece on or off: compareTo implementing Ord.compareTo has no Go method of Ord's name; not this piece's subject)
    p("Both " + ol);
    return "cmp done";
  }

  public static String run() {
    p(run1());
    String s = out.toString();
    out.setLength(0);
    return s;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
