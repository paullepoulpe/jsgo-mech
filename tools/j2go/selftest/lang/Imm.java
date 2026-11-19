package lang;

import com.google.common.base.Splitter;
import com.google.common.collect.ImmutableCollection;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import com.google.common.collect.ImmutableSortedSet;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Typed immutable collections (j2go rule "typed collections"): declarations of type
 * ImmutableList<X> / ImmutableSet<X> / ImmutableMap<K, V>, and List / Set / Map declarations
 * that hold Guava's immutable collections, are typed collections that never change; erased
 * code sees them as the runtime's immutable collections (the twin), and an immutable collection
 * that erased code made enters a typed declaration as the typed collection of that object.
 * Order, null checks and their messages, duplicate keys, the empty collections, identity on
 * both sides, copyOf of an immutable collection, the UnsupportedOperationException of changes.
 */
public class Imm {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  static class Base {
    final int id;

    Base(int id) {
      this.id = id;
    }

    @Override
    public String toString() {
      return "b" + id;
    }
  }

  /** Compared by equals: equal ids are one element. */
  static class Key {
    final int id;
    final String tag;

    Key(int id, String tag) {
      this.id = id;
      this.tag = tag;
    }

    @Override
    public boolean equals(Object o) {
      return o instanceof Key k && k.id == id;
    }

    @Override
    public int hashCode() {
      return id % 3;
    }

    @Override
    public String toString() {
      return "k" + id + tag;
    }
  }

  static String fails(Runnable r) {
    try {
      r.run();
      return "no exception";
    } catch (RuntimeException e) {
      return e.getClass().getSimpleName();
    }
  }

  static String failsMsg(Runnable r) {
    try {
      r.run();
      return "no exception";
    } catch (RuntimeException e) {
      return e.getClass().getSimpleName() + ": " + e.getMessage();
    }
  }

  // ---- lists

  List<String> names = ImmutableList.of();
  ImmutableList<Base> bases = ImmutableList.of(new Base(1), new Base(2), new Base(3));
  final ImmutableList<String> empty = ImmutableList.of();

  static Object erased(Object o) {
    return o;
  }

  static int count(Collection<?> c) {
    int n = 0;
    for (Object o : c) {
      if (o != null) n++;
    }
    return n;
  }

  static ImmutableList<String> pick(boolean b, ImmutableList<String> x, ImmutableList<String> y) {
    return b ? x : y;
  }

  static List<String> either(boolean imm) {
    if (imm) return ImmutableList.of("i1", "i2");
    List<String> l = new ArrayList<>();
    l.add("m1");
    return l;
  }

  void lists() {
    p("names " + names + " " + names.size() + " " + names.isEmpty());
    p("add to names: " + fails(() -> names.add("x")));
    names = new ArrayList<>();
    names.add("a");
    names.add("b");
    p("names " + names);
    names = ImmutableList.of("x", "y", "z");
    p("names " + names + " " + names.get(1) + " " + names.contains("z") + " " + names.indexOf("y") + " " + names.indexOf("q"));
    p("set: " + fails(() -> names.set(0, "w")));
    p("remove: " + fails(() -> names.remove(0)));
    p("removeObject: " + fails(() -> names.remove("x")));
    p("clear: " + fails(() -> names.clear()));
    p("addAll: " + fails(() -> names.addAll(Arrays.asList("q"))));
    Iterator<String> it = names.iterator();
    it.next();
    p("iterator remove: " + fails(it::remove));
    StringBuilder sb = new StringBuilder();
    for (String s : names) sb.append(s).append('/');
    for (Base b : bases) sb.append(b.id);
    p(sb);
    p("bases " + bases + " " + bases.size() + " " + bases.get(2) + " " + bases.reverse() + " " + bases.subList(1, 3) + " " + count(bases));
    p("hash " + names.hashCode() + " " + names.equals(Arrays.asList("x", "y", "z")) + " " + Arrays.asList("x", "y", "z").equals(names) + " " + names.equals(ImmutableList.of("x", "y")));

    // identity on both sides
    Object o = erased(bases);
    @SuppressWarnings("unchecked")
    ImmutableList<Base> back = (ImmutableList<Base>) o;
    p("same " + (back == bases) + " " + (o == bases) + " " + (ImmutableList.copyOf(bases) == bases) + " " + (erased(bases) == o) + " " + (o instanceof ImmutableList));
    Object made = erased(ImmutableList.of("p", "q"));
    @SuppressWarnings("unchecked")
    ImmutableList<String> t1 = (ImmutableList<String>) made;
    @SuppressWarnings("unchecked")
    ImmutableList<String> t2 = (ImmutableList<String>) made;
    p("same " + (t1 == t2) + " " + (erased(t1) == made) + " " + t1 + " " + t2.get(0) + " " + pick(true, t1, empty) + pick(false, t1, empty));
    p("empty " + (empty == ImmutableList.<String>of()) + " " + (erased(empty) == erased(ImmutableList.<Base>of())) + " " + empty.size() + empty);

    // null elements
    String nul = names.size() > 100 ? "never" : null;
    p("of null: " + failsMsg(() -> p(ImmutableList.of("a", nul))));
    ImmutableList<String> one = ImmutableList.of("only");
    p(one + " " + one.get(0).length());

    // a List that is an ArrayList or an ImmutableList
    List<String> e1 = either(true);
    List<String> e2 = either(false);
    p(e1 + " " + e2 + " " + fails(() -> e1.add("more")) + " " + fails(() -> e2.add("more")) + " " + e2);
    p("cast: " + fails(() -> p((ImmutableList<String>) e1)) + " / " + fails(() -> p((ImmutableList<String>) e2)));

    // made by a builder, by copyOf
    ImmutableList.Builder<Base> b = ImmutableList.builder();
    b.add(new Base(7)).add(new Base(8));
    ImmutableList<Base> built = b.build();
    List<Base> src = new ArrayList<>(built);
    src.add(new Base(9));
    ImmutableList<Base> copy = ImmutableList.copyOf(src);
    src.clear();
    ImmutableCollection<Base> coll = copy;
    p(built + " " + copy + " " + src + " " + (ImmutableList.copyOf(copy) == copy) + " " + coll.asList().size() + " " + (coll.asList() == copy));
  }

  // ---- sets

  Set<String> seen = new LinkedHashSet<>();
  ImmutableSet<String> words = ImmutableSet.of("b", "a", "b", "c", "a");
  ImmutableSet<Key> keys = ImmutableSet.of(new Key(1, "x"), new Key(4, "y"), new Key(1, "z"), new Key(7, "w"));

  void sets() {
    seen.add("s");
    p("seen " + seen);
    seen = ImmutableSet.of("t", "u");
    p("seen " + seen + " " + seen.contains("u") + " " + seen.contains("s") + " " + fails(() -> seen.add("t")) + " " + fails(() -> seen.remove("zz")) + " " + fails(() -> seen.clear()));
    p("words " + words + " " + words.size() + " " + words.contains("c") + " " + words.asList() + " " + words.asList().get(0) + " " + words.hashCode());
    p("keys " + keys + " " + keys.contains(new Key(4, "")) + " " + keys.contains(new Key(10, "")) + " " + keys.contains(null) + " " + keys.size());
    StringBuilder sb = new StringBuilder();
    for (String w : words) sb.append(w);
    for (Key k : keys) sb.append(k.id);
    p(sb);
    ImmutableSet<String> none = ImmutableSet.of();
    p("empty " + (none == ImmutableSet.<String>of()) + " " + none + " " + none.isEmpty() + " " + (erased(none) == erased(ImmutableSet.<Key>of())));
    Object o = erased(words);
    @SuppressWarnings("unchecked")
    ImmutableSet<String> back = (ImmutableSet<String>) o;
    p("same " + (back == words) + " " + (ImmutableSet.copyOf(words) == words) + " " + words.equals(new LinkedHashSet<>(Arrays.asList("a", "b", "c"))) + " " + new LinkedHashSet<>(Arrays.asList("a", "b", "c")).equals(words));
    String nul = words.size() > 100 ? "never" : null;
    p("of null: " + fails(() -> p(ImmutableSet.of("a", "b", nul))));

    // a sorted set that erased code made, as an ImmutableSet
    Object sorted = erased(ImmutableSortedSet.copyOf(Arrays.asList("pear", "apple", "fig")));
    @SuppressWarnings("unchecked")
    ImmutableSet<String> fruit = (ImmutableSet<String>) sorted;
    p("fruit " + fruit + " " + fruit.contains("fig") + " " + fruit.contains("kiwi") + " " + (erased(fruit) == sorted) + " " + (fruit instanceof ImmutableSortedSet));

    ImmutableSet.Builder<Key> kb = ImmutableSet.builder();
    kb.add(new Key(2, "a"));
    kb.add(new Key(5, "b"));
    kb.add(new Key(2, "c"));
    ImmutableSet<Key> built = kb.build();
    Set<Key> mutable = new LinkedHashSet<>(built);
    mutable.add(new Key(8, "d"));
    p(built + " " + mutable + " " + ImmutableSet.copyOf(mutable));
  }

  // ---- maps

  Map<String, Integer> counts = ImmutableMap.of();
  ImmutableMap<String, Base> byName = ImmutableMap.of("one", new Base(1), "two", new Base(2));
  ImmutableMap<Key, String> byKey = ImmutableMap.of(new Key(1, "x"), "first", new Key(4, "y"), "second");

  void maps() {
    p("counts " + counts + " " + counts.size() + " " + fails(() -> counts.put("a", 1)));
    counts = new LinkedHashMap<>();
    counts.put("a", 1);
    counts = ImmutableMap.of("x", 10, "y", 20, "z", 30);
    p("counts " + counts + " " + counts.get("y") + " " + counts.get("q") + " " + counts.containsKey("z") + " " + counts.getOrDefault("q", -1) + " " + (counts.get("x") + counts.get("z")));
    p("put: " + fails(() -> counts.put("x", 1)) + " remove: " + fails(() -> counts.remove("x")) + " clear: " + fails(() -> counts.clear())
        + " putIfAbsent: " + fails(() -> counts.putIfAbsent("n", 1)) + " computeIfAbsent: " + fails(() -> counts.computeIfAbsent("x", k -> 1)));
    StringBuilder sb = new StringBuilder();
    for (Map.Entry<String, Integer> e : counts.entrySet()) sb.append(e.getKey()).append('=').append(e.getValue() + 1).append(' ');
    for (String k : byName.keySet()) sb.append(k).append(' ');
    for (Base b : byName.values()) sb.append(b.id);
    p(sb);
    for (Map.Entry<String, Integer> e : counts.entrySet()) {
      p("setValue: " + fails(() -> e.setValue(0)));
      break;
    }
    p("byName " + byName + " " + byName.get("two") + " " + byName.keySet() + " " + byName.values() + " " + byName.entrySet() + " " + byName.containsValue(byName.get("one")));
    p("byKey " + byKey + " " + byKey.get(new Key(4, "")) + " " + byKey.get(new Key(9, "")) + " " + byKey.containsKey(new Key(1, "q")));
    Object o = erased(byName);
    @SuppressWarnings("unchecked")
    ImmutableMap<String, Base> back = (ImmutableMap<String, Base>) o;
    p("same " + (back == byName) + " " + (ImmutableMap.copyOf(byName) == byName) + " " + byName.equals(new LinkedHashMap<>(byName)) + " " + (byName.hashCode() == new LinkedHashMap<>(byName).hashCode()));
    ImmutableMap<String, Integer> none = ImmutableMap.of();
    p("empty " + (none == ImmutableMap.<String, Integer>of()) + " " + none + " " + (erased(none) == erased(ImmutableMap.<Key, String>of())));
    String nul = counts.size() > 100 ? "never" : null;
    p("dup: " + fails(() -> {
      ImmutableMap<String, Integer> d = ImmutableMap.of("k", 1, "k", 2);
      p(d);
    }));
    p("null key: " + failsMsg(() -> {
      ImmutableMap<String, Integer> d = ImmutableMap.of("k", 1, nul, 2);
      p(d);
    }));
    p("null value: " + failsMsg(() -> {
      ImmutableMap<String, String> d = ImmutableMap.of("k", "v", "l", nul);
      p(d);
    }));
    ImmutableMap.Builder<String, Base> mb = ImmutableMap.builder();
    mb.put("n", new Base(5));
    mb.put("m", new Base(6));
    ImmutableMap<String, Base> built = mb.buildOrThrow();
    Map<String, Base> mutable = new LinkedHashMap<>(built);
    mutable.put("o", new Base(7));
    p(built + " " + mutable + " " + ImmutableMap.copyOf(mutable) + " " + built.get("m").id);
  }

  // ---- builders and copies

  static void fill(ImmutableMap.Builder<String, Base> b, int n) {
    for (int i = 0; i < n; i++) b.put("f" + i, new Base(i));
  }

  static ImmutableList.Builder<String> start(String first) {
    ImmutableList.Builder<String> b = new ImmutableList.Builder<>();
    b.add(first);
    return b;
  }

  void builders() {
    List<String> some = new ArrayList<>();
    some.add("s1");
    some.add("s2");
    ImmutableList.Builder<String> lb = start("first");
    lb.add("x").addAll(some).add("y");
    ImmutableList<String> l1 = lb.build();
    lb.addAll(names).addAll(Arrays.asList("e1", "e2")).addAll(l1);
    ImmutableList<String> l2 = lb.build();
    p(l1 + " " + l2 + " " + l1.size() + " " + l2.get(4));
    String nul = some.size() > 100 ? "never" : null;
    p("add null: " + failsMsg(() -> lb.add(nul)));
    some.add(null);
    p("addAll null: " + fails(() -> lb.addAll(some)) + " copyOf null: " + failsMsg(() -> {
      ImmutableList<String> c = ImmutableList.copyOf(some);
      p(c);
    }));
    ImmutableList.Builder<Base> sized = ImmutableList.builderWithExpectedSize(2);
    ImmutableList<Base> none = sized.build();
    sized.add(bases.get(0));
    String[] arr = {"a1", "a2"};
    ImmutableList<String> fromArr = ImmutableList.copyOf(arr);
    arr[0] = "changed";
    p(none + " " + (none == ImmutableList.<Base>of()) + " " + sized.build() + " " + fromArr + " " + (ImmutableList.copyOf(fromArr) == fromArr));

    ImmutableSet.Builder<String> sb = ImmutableSet.builder();
    sb.add("q").add("r");
    sb.addAll(some.subList(0, 2)).addAll(words).add("q");
    ImmutableSet<String> s1 = sb.build();
    Set<String> ms = new LinkedHashSet<>(s1);
    ms.remove("r");
    ms.add("zz");
    ImmutableSet<String> s2 = ImmutableSet.copyOf(ms);
    ImmutableSet<String> s3 = ImmutableSet.copyOf(l2);
    ImmutableSet.Builder<Key> kb = ImmutableSet.builderWithExpectedSize(4);
    kb.addAll(keys).add(new Key(1, "late")).add(new Key(2, "new"));
    p(s1 + " " + s2 + " " + s3 + " " + (ImmutableSet.copyOf(s2) == s2) + " " + kb.build() + " " + s3.contains("e2"));

    ImmutableMap.Builder<String, Base> mb = ImmutableMap.builder();
    mb.put("a", new Base(1)).put("b", new Base(2));
    fill(mb, 2);
    Map<String, Base> more = new LinkedHashMap<>();
    more.put("m", new Base(9));
    mb.putAll(more).putAll(byName);
    ImmutableMap<String, Base> m1 = mb.buildOrThrow();
    mb.put("a", new Base(100));
    p(m1 + " " + m1.get("f1") + " " + fails(() -> mb.build()) + " " + mb.buildKeepingLast() + " " + (ImmutableMap.copyOf(m1) == m1) + " " + ImmutableMap.copyOf(more));
    p("put null: " + failsMsg(() -> mb.put("k", null)) + " / " + failsMsg(() -> mb.put(nul, new Base(0))));
  }

  // ---- records, and the key set of an immutable map

  interface Named {
    ImmutableList<String> names();
  }

  record Pair(ImmutableSet<String> first, ImmutableSet<String> second) {}

  record Checked(ImmutableList<String> parts, List<Base> more, int n) {
    Checked {
      if (n < 0) throw new IllegalArgumentException("n");
      more = ImmutableList.copyOf(more);
    }

    Checked(String only) {
      this(ImmutableList.of(only), new ArrayList<>(), 1);
    }

    public List<Base> more() {
      return more;
    }

    int total() {
      return parts.size() + more.size() + n;
    }
  }

  record Tagged(ImmutableList<String> names, String tag) implements Named {}

  static Pair split(ImmutableSet<String> all) {
    Set<String> a = new LinkedHashSet<>();
    Set<String> b = new LinkedHashSet<>();
    for (String s : all) (s.length() > 1 ? a : b).add(s);
    return new Pair(ImmutableSet.copyOf(a), ImmutableSet.copyOf(b));
  }

  static int sizeOf(ImmutableSet<String> s) {
    return s.size();
  }

  void records() {
    Pair pr = split(ImmutableSet.of("aa", "b", "cc", "d"));
    ImmutableSet<String> f = pr.first();
    p(pr + " " + f + " " + pr.second().contains("b") + " " + (pr.first() == f) + " " + pr.equals(new Pair(f, pr.second())) + " " + (pr.hashCode() == new Pair(f, pr.second()).hashCode()));
    List<Base> src = new ArrayList<>();
    src.add(new Base(4));
    Checked c = new Checked(ImmutableList.of("p", "q"), src, 2);
    src.clear();
    Checked d = new Checked("solo");
    p(c + " " + c.total() + " " + c.more() + " " + c.parts().get(1) + " " + d + " " + d.total() + " " + fails(() -> new Checked(ImmutableList.of(), src, -1)) + " " + fails(() -> c.more().add(null)));
    Named n = new Tagged(ImmutableList.of("n1", "n2"), "t");
    Tagged t = new Tagged(n.names(), "u");
    p(n + " " + n.names().size() + " " + t.names().get(0) + " " + (t.names() == n.names()) + " " + t.equals(n));

    ImmutableSet<String> ks = byName.keySet();
    ImmutableSet<String> ks2 = byName.keySet();
    Object ek = erased(byName).equals(null) ? null : ((Map<?, ?>) erased(byName)).keySet();
    p(ks + " " + (ks == ks2) + " " + (erased(ks) == ek) + " " + sizeOf(byName.keySet()) + " " + ks.contains("two") + " " + sizeOf(byKey.size() > 0 ? ImmutableSet.of("z") : ImmutableSet.of()));
    ImmutableMap<String, Integer> fresh = ImmutableMap.of("k1", 1, "k2", 2);
    Object ek2 = ((Map<?, ?>) erased(fresh)).keySet();
    ImmutableSet<String> fk = fresh.keySet();
    ImmutableMap<String, Integer> none = ImmutableMap.of();
    p(fk + " " + (erased(fk) == ek2) + " " + (fresh.keySet() == fk) + " " + none.keySet() + " " + sizeOf(none.keySet()));
  }

  // ---- a list that a library method makes for its caller (Splitter.splitToList)

  static final Splitter DOTS = Splitter.on('.');

  static int longest(List<String> parts) {
    int n = 0;
    for (String p : parts) n = Math.max(n, p.length());
    return n;
  }

  void split() {
    List<String> parts = DOTS.splitToList("goog.string.Const.from");
    List<String> csv = Splitter.on(',').trimResults().omitEmptyStrings().splitToList(" a, b ,,c ");
    Object o = parts;
    p(parts + " " + parts.size() + " " + parts.get(2) + " " + longest(parts) + " " + csv + " " + longest(csv) + " " + parts.contains("string") + " " + parts.indexOf("from") + " " + o.equals(Arrays.asList("goog", "string", "Const", "from")) + " " + (o == parts));
    p(fails(() -> parts.add("x")) + " " + fails(() -> parts.set(0, "x")) + " " + fails(() -> parts.remove(0)) + " " + fails(() -> parts.clear()) + " " + fails(() -> ((List<Object>) o).add("y")) + " " + fails(() -> p((ImmutableList<String>) parts)) + " " + parts);
    List<String> copy = new ArrayList<>(parts);
    copy.add("more");
    ImmutableList<String> imm = ImmutableList.copyOf(parts);
    p(copy + " " + imm + " " + DOTS.splitToList("") + " " + DOTS.splitToList("x").size());
  }

  public static String run() {
    out.setLength(0);
    new Imm().lists();
    new Imm().sets();
    new Imm().maps();
    new Imm().builders();
    new Imm().records();
    new Imm().split();
    return out.toString();
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
