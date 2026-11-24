package lang;

import java.util.ArrayList;
import java.util.List;

/**
 * Piece `ownwild` of rule "typed generics" (GenScan.ownWild): a C<?> inside C's own code that
 * only holds a C of the receiver's instantiation is C<T>. Pos and Entry are typed (Go generic
 * types); the eight other classes keep their wildcard (each for one reason) and stay erased.
 */
public class OwnWild {
  enum Color { RED, GREEN }

  enum Size { S, M }

  /** Typed: a parameter Pos<?> of a method that nothing overrides, every call of the receiver's instantiation. */
  abstract static class Pos<T> {
    T item;
    int line;

    Pos(T item, int line) {
      this.item = item;
      this.line = line;
    }

    boolean samePos(Pos<?> that) {
      return this.line == that.line;
    }

    T getItem() {
      return item;
    }
  }

  static final class SPos extends Pos<String> {
    SPos(String s, int l) {
      super(s, l);
    }

    boolean eq(Pos<String> o) {
      return samePos(o) && getItem().equals(o.getItem());
    }
  }

  static final class NPos extends Pos<Integer> {
    NPos(Integer s, int l) {
      super(s, l);
    }

    boolean eq(NPos o) {
      return o.samePos(this) && getItem().equals(o.getItem());
    }
  }

  /** Typed: an equals listed in selftest/ownwild-equals.txt; every instantiation an enum. */
  static class Entry<T> {
    final T flag;
    final String value;

    Entry(T flag, String value) {
      this.flag = flag;
      this.value = value;
    }

    @Override
    public boolean equals(Object o) {
      if (o instanceof Entry) {
        Entry<?> that = (Entry<?>) o;
        return that.flag.equals(this.flag) && that.value.equals(this.value);
      }
      return false;
    }

    @Override
    public int hashCode() {
      return value.hashCode();
    }

    T getFlag() {
      return flag;
    }
  }

  /** Left: the method is overridden. */
  static class Over<T> {
    T v;

    boolean same(Over<?> o) {
      return o.v == null;
    }
  }

  static class Over2 extends Over<String> {
    @Override
    boolean same(Over<?> o) {
      return true;
    }
  }

  /** Left: a call whose argument is another instantiation. */
  static class Mixed<T> {
    T v;

    Mixed(T v) {
      this.v = v;
    }

    boolean same(Mixed<?> o) {
      return o.v != null && v != null;
    }
  }

  /** Left: a parameter of a static method. */
  static class Stat<T> {
    T v;

    static boolean empty(Stat<?> a) {
      return a.v == null;
    }
  }

  /** Left: the parameter is assigned. */
  static class Asg<T> {
    T v;

    boolean same(Asg<?> o) {
      if (o == null) {
        o = this;
      }
      return o.v == v;
    }
  }

  /** Left: the method is referenced with ::. */
  static class Ref<T> {
    T v;

    boolean same(Ref<?> o) {
      return o.v == v;
    }
  }

  /** Left: an equals that the list does not name. */
  static class Unl<T> {
    final T flag;

    Unl(T flag) {
      this.flag = flag;
    }

    @Override
    public boolean equals(Object o) {
      if (o instanceof Unl) {
        Unl<?> that = (Unl<?>) o;
        return that.flag.equals(this.flag);
      }
      return false;
    }

    @Override
    public int hashCode() {
      return 1;
    }
  }

  /** Left: listed, but instantiated with a type that is not an enum (String). */
  static class Str<T> {
    final T flag;

    Str(T flag) {
      this.flag = flag;
    }

    @Override
    public boolean equals(Object o) {
      if (o instanceof Str) {
        Str<?> that = (Str<?>) o;
        return that.flag.equals(this.flag);
      }
      return false;
    }

    @Override
    public int hashCode() {
      return 2;
    }
  }

  /** Left: listed, but the cast is not guarded by instanceof. */
  static class NoGuard<T> {
    final T flag;

    NoGuard(T flag) {
      this.flag = flag;
    }

    @Override
    public boolean equals(Object o) {
      NoGuard<?> that = (NoGuard<?>) o;
      return that != null && that.flag.equals(this.flag);
    }

    @Override
    public int hashCode() {
      return 3;
    }
  }

  public static void main(String[] args) {
    System.out.print(run());
  }

  public static String run() {
    StringBuilder sb = new StringBuilder("OwnWild\n");
    SPos a = new SPos("x", 1), b = new SPos("x", 1), c = new SPos("y", 2);
    NPos n = new NPos(7, 3), m = new NPos(7, 3);
    sb.append(a.eq(b)).append(' ').append(a.eq(c)).append(' ').append(a.samePos(c)).append(' ').append(n.eq(m)).append(' ').append(n.getItem() + 1).append('\n');
    Entry<Color> e1 = new Entry<>(Color.RED, "a"), e2 = new Entry<>(Color.RED, "a"), e3 = new Entry<>(Color.GREEN, "a");
    Entry<Size> s1 = new Entry<>(Size.S, "a");
    List<Entry<Color>> es = new ArrayList<>();
    es.add(e1);
    es.add(e3);
    es.remove(e2);
    sb.append(e1.equals(e2)).append(' ').append(e1.equals(e3)).append(' ').append(e1.equals(s1)).append(' ').append(s1.equals(e1))
        .append(' ').append(e1.equals("a")).append(' ').append(es.size()).append(' ').append(es.get(0).getFlag()).append('\n');
    Over<String> o1 = new Over<>();
    Over2 o2 = new Over2();
    Mixed<String> x1 = new Mixed<>("s");
    Mixed<Integer> x2 = new Mixed<>(1);
    Stat<String> st = new Stat<>();
    Asg<String> as = new Asg<>();
    Ref<String> r = new Ref<>();
    java.util.function.Predicate<Ref<String>> rp = r::same;
    sb.append(o1.same(o2)).append(' ').append(o2.same(o1)).append(' ').append(x1.same(x2)).append(' ').append(Stat.empty(st)).append(' ')
        .append(as.same(null)).append(' ').append(rp.test(r)).append('\n');
    sb.append(new Unl<>(Color.RED).equals(new Unl<>(Color.RED))).append(' ').append(new Str<>("q").equals(new Str<>("q"))).append(' ')
        .append(new NoGuard<>(Size.M).equals(new NoGuard<>(Size.M))).append('\n');
    return sb.toString();
  }
}
