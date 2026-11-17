package lang;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Typed generics (j2go GenScan): generic classes and interfaces of the program whose type
 * parameters are Go type parameters, values of a type parameter meeting Object, null of a
 * String type argument, and type parameters erased by a wildcard use.
 */
public class Generics {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  // ---- a generic interface, implemented by a generic class and by a plain class
  interface Store<K, V> {
    V get(K k);

    void put(K k, V v);

    int size();

    default V getOr(K k, V dflt) {
      V v = get(k);
      return v == null ? dflt : v;
    }
  }

  static final class MapStore<K, V> implements Store<K, V> {
    private final Map<K, V> m = new LinkedHashMap<>();

    @Override
    public V get(K k) {
      return m.get(k);
    }

    @Override
    public void put(K k, V v) {
      m.put(k, v);
    }

    @Override
    public int size() {
      return m.size();
    }

    static <K, V> MapStore<K, V> create() {
      return new MapStore<>();
    }

    @Override
    public String toString() {
      return "MapStore" + m;
    }
  }

  static final class Names implements Store<String, String> {
    String lastK;
    String last;

    @Override
    public String get(String k) {
      return k.equals(lastK) ? last : null;
    }

    @Override
    public void put(String k, String v) {
      lastK = k;
      last = v;
    }

    @Override
    public int size() {
      return last == null ? 0 : 1;
    }
  }

  static <V> String describe(Store<String, V> s, String k) {
    V v = s.get(k);
    return k + "=" + v + (v == null ? " (null)" : " hash=" + v.hashCode()) + " size=" + s.size();
  }

  // ---- generic functional interfaces: lambdas, method and constructor references
  interface Maker<T> {
    T make(String s);
  }

  static final class Box<T> {
    final T v;

    Box(T v) {
      this.v = v;
    }

    T get() {
      return v;
    }

    boolean same(Box<T> o) {
      return v == o.v;
    }

    boolean eq(Box<T> o) {
      return v == null ? o.v == null : v.equals(o.v);
    }
  }

  interface BoxMaker<T> {
    Box<T> box(T v);
  }

  // ---- a type parameter that a wildcard use erases (only that one)
  interface Pair<A, B> {
    A first();

    B second();
  }

  static final class P<A, B> implements Pair<A, B> {
    final A a;
    final B b;

    P(A a, B b) {
      this.a = a;
      this.b = b;
    }

    @Override
    public A first() {
      return a;
    }

    @Override
    public B second() {
      return b;
    }
  }

  static String firsts(List<Pair<String, ?>> ps) {
    StringBuilder sb = new StringBuilder();
    for (Pair<String, ?> p : ps) sb.append(p.first()).append(':').append(p.second()).append(' ');
    return sb.toString();
  }

  public static String run() {
    MapStore<String, Integer> counts = MapStore.create();
    counts.put("a", 1);
    counts.put("b", null);
    p(describe(counts, "a") + "; " + describe(counts, "b") + "; " + describe(counts, "c") + "; " + counts);
    Store<String, String> names = new Names();
    p(describe(names, "x") + " " + names.getOr("x", "dflt"));
    names.put("x", "y");
    p(describe(names, "x") + " " + names.getOr("x", "dflt") + " " + names.getOr("z", null));
    Store<String, String> ss = MapStore.create();
    ss.put(null, "n");
    ss.put("k", null);
    p(ss.get(null) + " " + ss.get("k") + " " + (ss.get("k") == null) + " " + ss.getOr("k", "d") + " " + ss);

    Maker<String> up = s -> s + s.length();
    Maker<Box<String>> boxer = Box::new;
    BoxMaker<Integer> bm = Box::new;
    Box<String> b1 = boxer.make(up.make("ABC"));
    Box<String> b2 = new Box<>("ABC" + "ABC".length());
    Object o1 = new ArrayList<String>(), o2 = new ArrayList<String>();
    Box<Object> bo1 = new Box<>(o1);
    p(b1.get() + " " + b1.eq(b2) + " " + bm.box(7).get() + " " + new Box<String>(null).eq(new Box<>(null))
        + " " + bo1.same(new Box<>(o1)) + " " + bo1.same(new Box<>(o2)) + " " + bo1.eq(new Box<>(o2)));

    List<Pair<String, ?>> ps = new ArrayList<>();
    ps.add(new P<>("one", 1));
    ps.add(new P<>("two", "zwei"));
    ps.add(new P<String, Object>(null, null));
    p(firsts(ps));
    return out.toString();
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
