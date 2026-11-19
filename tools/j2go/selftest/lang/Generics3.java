package lang;

import java.util.ArrayList;
import java.util.List;
import java.util.Objects;
import java.util.Optional;

/**
 * Typed generics, wildcards that do not erase (j2go GenScan.refine and GenScan.shared): a local
 * with wildcard type arguments typed from its initializer, a shared static of type C<?, ?> that
 * is one object per instantiation, the shared empty array, a new array of C<?, ?> under a cast,
 * and a final class converted to a typed interface it implements (null stays null); and
 * java.util.Optional as a value (rule "Optional is a value"): null, empty, content, equals between
 * instantiations, an Optional as an Object and back.
 */
public class Generics3 {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  // ---- a persistent list with a shared empty object and a shared empty array
  interface Seq<K, V> {
    Seq<K, V> plus(K k, V v);

    V get(K k);

    int size();
  }

  static final class Trie<K, V> implements Seq<K, V> {
    private final K key;
    private final V value;
    private final Trie<K, V>[] kids;

    private static final Trie<?, ?>[] NO_KIDS = new Trie<?, ?>[0];
    private static final Trie<?, ?> EMPTY = new Trie<>(null, null, noKids());

    private Trie(K key, V value, Trie<K, V>[] kids) {
      this.key = key;
      this.value = value;
      this.kids = kids;
      for (Trie<?, ?> kid : this.kids) {
        Objects.requireNonNull(kid);
      }
    }

    @SuppressWarnings("unchecked")
    static <K, V> Trie<K, V> empty() {
      return (Trie<K, V>) EMPTY;
    }

    @SuppressWarnings("unchecked")
    private static <K, V> Trie<K, V>[] noKids() {
      return (Trie<K, V>[]) NO_KIDS;
    }

    boolean isEmpty() {
      return this == EMPTY;
    }

    @Override
    public Seq<K, V> plus(K k, V v) {
      if (this.isEmpty()) {
        return new Trie<>(k, v, noKids());
      }
      @SuppressWarnings("unchecked")
      Trie<K, V>[] more = (Trie<K, V>[]) new Trie<?, ?>[kids.length + 1];
      System.arraycopy(kids, 0, more, 0, kids.length);
      more[kids.length] = new Trie<>(k, v, noKids());
      return new Trie<>(key, value, more);
    }

    @Override
    public V get(K k) {
      if (this.isEmpty()) {
        return null;
      }
      if (key.equals(k)) {
        return value;
      }
      for (Trie<?, ?> kid : kids) {
        if (kid.key.equals(k)) {
          @SuppressWarnings("unchecked")
          V v = (V) kid.value;
          return v;
        }
      }
      return null;
    }

    @Override
    public int size() {
      return this.isEmpty() ? 0 : 1 + kids.length;
    }

    Seq<K, V> orNull(boolean none) {
      Trie<K, V> t = none ? null : this;
      return t;
    }
  }

  // ---- a wildcard static that is used as a value: its class stays erased
  static final class Box<T> {
    final T v;
    static final Box<?> NONE = new Box<>(null);

    Box(T v) {
      this.v = v;
    }

    static String show(Box<?> b) {
      return b == NONE ? "none" : "box(" + b.v + ")";
    }
  }

  // ---- locals with wildcards, typed from what they hold
  static final class Pair<A, B> {
    final A a;
    final B b;

    Pair(A a, B b) {
      this.a = a;
      this.b = b;
    }

    A first() {
      return a;
    }

    B second() {
      return b;
    }
  }

  static <A> List<Pair<A, String>> pairs(List<A> as) {
    List<Pair<A, String>> out = new ArrayList<>();
    for (A a : as) {
      out.add(new Pair<>(a, "<" + a + ">"));
    }
    return out;
  }

  static <A> String firsts(List<A> as) {
    StringBuilder sb = new StringBuilder();
    for (Pair<A, ?> pr : pairs(as)) {
      A a = pr.first();
      sb.append(a).append(Objects.requireNonNull(pr).second()).append(' ');
    }
    Pair<?, ?> one = new Pair<>("k", 7);
    sb.append(one.first()).append('=').append(one.second());
    return sb.toString();
  }

  // ---- java.util.Optional as a value: null, empty, content, other instantiations, Object
  static final class Holder {
    Optional<String> name;
    Optional<Integer> count = Optional.empty();
    Optional<Pair<String, String>> pair = Optional.empty();
  }

  static String show(Optional<?> o) {
    return o == null ? "null!" : o.isPresent() ? "has " + o.get() : "none";
  }

  static <T> Optional<T> first(List<T> l) {
    return l.isEmpty() ? Optional.empty() : Optional.ofNullable(l.get(0));
  }

  static String optionals() {
    StringBuilder sb = new StringBuilder();
    Holder h = new Holder();
    sb.append(h.name == null).append(' ').append(show(h.name)).append(' ').append(show(h.count)).append(' ');
    try {
      sb.append(h.name.isPresent());
    } catch (NullPointerException e) {
      sb.append("NPE");
    }
    h.name = Optional.of("n");
    h.count = Optional.of(3);
    h.pair = Optional.of(new Pair<>("a", "b"));
    sb.append(' ').append(h.name != null).append(' ').append(show(h.name)).append(' ').append(show(h.count)).append(' ').append(h.pair.get().first());
    sb.append('\n');
    Optional<String> e = Optional.empty();
    Optional<Integer> e2 = Optional.empty();
    sb.append(e.equals(e2)).append(' ').append(h.name.equals(Optional.of("n"))).append(' ').append(h.name.equals("n")).append(' ')
        .append(h.name.hashCode() == "n".hashCode()).append(' ').append(e.hashCode()).append(' ').append(h.name).append(' ').append(e).append(' ').append(h.count.orElse(9)).append(' ')
        .append(e.orElse("dflt")).append(' ').append(e.orElse(null) == null).append(' ').append(e.isEmpty());
    sb.append('\n');
    List<Optional<String>> l = new ArrayList<>();
    l.add(h.name);
    l.add(e);
    l.add(null);
    Object o = l.get(0);
    sb.append(o instanceof Optional).append(' ').append(l.get(2) == null).append(' ').append(l.get(1).isPresent()).append(' ').append(l.indexOf(Optional.empty())).append(' ');
    @SuppressWarnings("unchecked")
    Optional<String> back = (Optional<String>) o;
    sb.append(back.get()).append(' ').append(first(l).get().get()).append(' ').append(first(new ArrayList<String>()).isPresent()).append(' ');
    sb.append(h.name.filter(x -> x.startsWith("n")).isPresent()).append(' ').append(h.name.filter(x -> x.isEmpty()).isPresent()).append(' ');
    h.count.ifPresent(c -> sb.append("count=").append(c + 1));
    e.ifPresent(x -> sb.append("never"));
    try {
      Optional.of((String) null);
    } catch (NullPointerException ex) {
      sb.append(" NPE");
    }
    try {
      e.get();
    } catch (java.util.NoSuchElementException ex) {
      sb.append(" NSE");
    }
    try {
      e.orElseThrow();
    } catch (java.util.NoSuchElementException ex) {
      sb.append(" NSE");
    }
    return sb.toString();
  }

  public static String run() {
    Seq<String, Integer> s = Trie.empty();
    p(s.size() + " " + s.get("a"));
    s = s.plus("a", 1).plus("b", 2).plus("c", null);
    p(s.size() + " " + s.get("a") + " " + s.get("b") + " " + s.get("c") + " " + s.get("d"));
    Trie<String, Integer> e1 = Trie.empty();
    Trie<String, Integer> e2 = Trie.empty();
    Trie<Integer, String> e3 = Trie.empty();
    p((e1 == e2) + " " + e1.isEmpty() + " " + e3.isEmpty() + " " + e3.plus(1, "one").get(1));
    p(e1.orNull(true) == null);
    p(e1.orNull(false) == e2);
    p(Box.show(Box.NONE) + " " + Box.show(new Box<>("x")));
    List<String> names = new ArrayList<>();
    names.add("x");
    names.add("y");
    p(firsts(names));
    p(optionals());
    String r = out.toString();
    out.setLength(0);
    return r;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
