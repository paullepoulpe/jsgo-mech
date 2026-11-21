package lang;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.Iterables;
import com.google.common.collect.Maps;
import com.google.common.collect.Sets;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import java.util.AbstractCollection;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Collections;
import java.util.Deque;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.NoSuchElementException;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;

/**
 * Rule "typed library interfaces" (j2go.LibGen): Iterator and Iterable with their typed methods.
 * The values must behave as the erased ones did.
 */
public class LibGen {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  static class Animal {
    final String name;

    Animal(String name) {
      this.name = name;
    }

    @Override
    public String toString() {
      return name;
    }
  }

  static final class Dog extends Animal {
    Dog(String name) {
      super(name);
    }
  }

  /** A class that implements Iterable itself and has no subclass: its objects are typed values. */
  static final class Range implements Iterable<Integer> {
    final int from, to;

    Range(int from, int to) {
      this.from = from;
      this.to = to;
    }

    @Override
    public Iterator<Integer> iterator() {
      return new Iterator<Integer>() {
        int i = from;

        @Override
        public boolean hasNext() {
          return i < to;
        }

        @Override
        public Integer next() {
          if (i >= to) {
            throw new NoSuchElementException();
          }
          int v = i;
          i++;
          return v;
        }
      };
    }
  }

  static final class Words implements Iterable<String> {
    final List<String> ws = new ArrayList<>();

    Words(String... xs) {
      for (String x : xs) {
        ws.add(x);
      }
    }

    @Override
    public Iterator<String> iterator() {
      return ws.iterator();
    }
  }

  /** A class with a subclass: its iterator() is dispatched, the values stay erased. */
  static class Pets implements Iterable<Animal> {
    final List<Animal> list = new ArrayList<>();

    @Override
    public Iterator<Animal> iterator() {
      return list.iterator();
    }
  }

  static final class Dogs extends Pets {
    @Override
    public Iterator<Animal> iterator() {
      List<Animal> rev = new ArrayList<>();
      for (int i = list.size() - 1; i >= 0; i--) {
        rev.add(list.get(i));
      }
      return rev.iterator();
    }
  }

  static final class Box<T> {
    final Iterable<T> items;

    Box(Iterable<T> items) {
      this.items = items;
    }

    T first() {
      Iterator<T> it = items.iterator();
      return it.hasNext() ? it.next() : null;
    }
  }

  static String join(Iterable<String> xs) {
    StringBuilder sb = new StringBuilder();
    for (String x : xs) {
      sb.append(x).append(',');
    }
    return sb.toString();
  }

  static String names(Iterable<? extends Animal> xs) {
    StringBuilder sb = new StringBuilder();
    for (Animal a : xs) {
      sb.append(a.name).append(';');
    }
    return sb.toString();
  }

  static int count(Iterable<?> xs) {
    int n = 0;
    for (Object x : xs) {
      if (x != null) {
        n++;
      }
    }
    return n;
  }

  static <T> T last(Iterable<T> xs) {
    T r = null;
    Iterator<T> it = xs.iterator();
    while (it.hasNext()) {
      r = it.next();
    }
    return r;
  }

  static int sum(Iterator<Integer> it) {
    int n = 0;
    while (it.hasNext()) {
      n += it.next();
    }
    return n;
  }

  static List<String> words() {
    List<String> w = new ArrayList<>();
    w.add("a");
    w.add("bb");
    w.add("ccc");
    return w;
  }

  static String loops() {
    List<String> list = words();
    Set<String> set = new LinkedHashSet<>(list);
    Map<String, Integer> map = new LinkedHashMap<>();
    map.put("k1", 1);
    map.put("k2", 2);
    ImmutableList<String> imm = ImmutableList.of("x", "y");
    Iterable<String> fromList = list, fromSet = set, fromImm = imm, fromKeys = map.keySet();
    Iterable<String> filtered = Iterables.filter(list, s -> s.length() > 1);
    Iterable<String> lambda = () -> list.iterator();
    Words words = new Words("p", "q");
    Iterable<String> asIter = words;
    return join(list) + " " + join(set) + " " + join(imm) + " " + join(fromList) + join(fromSet) + join(fromImm) + join(fromKeys) + " " + join(filtered) + " " + join(lambda) + " "
        + join(words) + join(asIter) + " " + join(map.keySet()) + " " + sum(new Range(1, 5).iterator()) + " " + sum(map.values().iterator()) + " " + count(new Range(0, 3)) + count(list) + count(map.values());
  }

  static String variance() {
    List<Dog> dogs = new ArrayList<>();
    dogs.add(new Dog("rex"));
    dogs.add(new Dog("fido"));
    Dogs pets = new Dogs();
    pets.list.add(new Animal("cat"));
    pets.list.add(new Dog("pug"));
    Pets asPets = pets;
    Iterable<Animal> it = asPets;
    Box<String> box = new Box<>(words());
    Box<Integer> ints = new Box<>(new Range(7, 9));
    Box<Animal> animals = new Box<>(pets);
    return names(dogs) + " " + names(pets) + " " + names(it) + " " + names(new ArrayList<Animal>()) + "|" + box.first() + ints.first() + animals.first() + " " + last(dogs) + last(words()) + last(new Range(1, 4)) + last(it);
  }

  static String identity() {
    List<String> list = words();
    Iterable<String> a = list, b = list;
    Iterable<String> c = words();
    Object o = a;
    Iterable<String> back = (Iterable<String>) o;
    Words w = new Words("z");
    Iterable<String> wi = w;
    Object wo = wi;
    Iterable<String> copy = ImmutableList.copyOf(a);
    ImmutableList<String> imm = ImmutableList.of("i");
    Iterable<String> immIt = imm;
    return (a == b) + " " + (a == c) + " " + (o == list) + " " + (back == a) + " " + (o instanceof List) + " " + (wo == w) + (wo instanceof Words) + " " + a.equals(list) + " " + join(copy)
        + " " + (ImmutableList.copyOf(immIt) == imm) + " " + a.toString() + " " + Iterables.size(a) + Iterables.getFirst(wi, "none") + " " + String.join("+", a);
  }

  @SuppressWarnings("unchecked")
  static String failures() {
    List<String> list = words();
    Iterator<String> it = list.iterator();
    it.next();
    it.next();
    it.next();
    String r;
    try {
      it.next();
      r = "no";
    } catch (NoSuchElementException e) {
      r = "nse";
    }
    Iterable<String> none = null;
    try {
      join(none);
      r += "no";
    } catch (NullPointerException e) {
      r += "npe";
    }
    List<Object> mixed = new ArrayList<>();
    mixed.add("s");
    mixed.add(1);
    Object o = mixed;
    Iterable<String> wrong = (Iterable<String>) o;
    int n = 0;
    try {
      for (String s : wrong) {
        n += s.length();
      }
      r += "no";
    } catch (ClassCastException e) {
      r += "cce" + n;
    }
    return r + count(wrong);
  }

  // ---- pieces `makers`, `views`, `cov`

  /** A kennel: its pets by name, in a map that a runtime factory makes; the views of the map as Iterables. */
  abstract static class Shelter {
    abstract Iterable<? extends Animal> all();

    abstract Iterable<? extends Animal> some(String prefix);

    String list() {
      String r = "";
      for (Animal a : all()) {
        r += a + ";";
      }
      return r;
    }
  }

  static final class Kennel extends Shelter {
    final Map<String, Dog> dogs = Maps.newLinkedHashMapWithExpectedSize(4);
    final Set<String> gone = Sets.newLinkedHashSetWithExpectedSize(2);

    void add(String n) {
      dogs.put(n, new Dog(n));
    }

    // narrows the result of Shelter.all: a caller that has a Kennel reads Dogs
    @Override
    Iterable<Dog> all() {
      return dogs.values();
    }

    @Override
    Iterable<Dog> some(String prefix) {
      List<Dog> r = new ArrayList<>();
      for (Dog d : dogs.values()) {
        if (d.name.startsWith(prefix)) {
          r.add(d);
        }
      }
      return r;
    }

    Iterable<String> names() {
      return dogs.keySet();
    }
  }

  static String views() {
    Kennel k = new Kennel();
    k.add("rex");
    k.add("fido");
    k.add("rolf");
    String r = "";
    for (Dog d : k.all()) {
      r += d.name + ",";
    }
    for (Dog d : k.some("r")) {
      r += d.name.length();
    }
    Shelter s = k;
    r += "|" + s.list() + names(s.some("f")) + "|" + join(k.names()) + count(k.all());
    // the views are live, and an iterator of a view removes from the map
    Iterable<Dog> all = k.all();
    Iterable<String> names = k.names();
    k.add("ace");
    Iterator<String> it = names.iterator();
    while (it.hasNext()) {
      if (it.next().equals("fido")) {
        it.remove();
      }
    }
    r += "|" + names(all) + join(names) + k.dogs.size() + k.dogs.containsKey("fido");
    Iterator<Dog> di = all.iterator();
    di.next();
    di.remove();
    r += k.dogs.keySet() + "" + (k.all() == (Object) k.dogs.values()) + (s.all() == (Object) k.all());
    k.gone.add("x");
    r += k.gone;
    try {
      Iterator<Dog> e = new Kennel().all().iterator();
      e.next();
      r += "no";
    } catch (NoSuchElementException e) {
      r += "nse";
    }
    return r;
  }

  // ---- kind `map` (Map, Map.Entry) and collections as elements

  static String dump(Map<String, Integer> m) {
    String r = m.size() + ":";
    for (Map.Entry<String, Integer> e : m.entrySet()) {
      r += e.getKey() + "=" + e.getValue() + ",";
    }
    for (String k : m.keySet()) {
      r += k;
    }
    int sum = 0;
    for (Integer v : m.values()) {
      sum += v == null ? 1000 : v;
    }
    Iterator<Map.Entry<String, Integer>> it = m.entrySet().iterator();
    while (it.hasNext()) {
      Map.Entry<String, Integer> e = it.next();
      r += e.getKey().length() + (e.getValue() == null ? "n" : "");
    }
    return r + sum + m.get("b") + m.get("zz") + m.containsKey("b") + m.containsValue(2) + m.isEmpty() + m;
  }

  static String wide(Map<? extends Object, ? extends Animal> m) {
    String r = "";
    for (Map.Entry<? extends Object, ? extends Animal> e : m.entrySet()) {
      r += e.getKey() + e.getValue().name;
    }
    for (Animal a : m.values()) {
      r += a.name;
    }
    return r + m.size();
  }

  static int anyMap(Map<?, ?> m) {
    int n = 0;
    for (Object k : m.keySet()) {
      n += k == null ? 100 : 1;
    }
    for (Map.Entry<?, ?> e : m.entrySet()) {
      n += e.getValue() == null ? 10 : 0;
    }
    return n;
  }

  static Map<String, Integer> pickMap(int how) {
    switch (how) {
      case 0:
        return new LinkedHashMap<>();
      case 1:
        return ImmutableMap.<String, Integer>builder().put("b", 2).put("a", 1).buildOrThrow();
      case 2: {
        Map<String, Integer> t = new TreeMap<>();
        t.put("z", 26);
        t.put("b", 2);
        t.put("m", 13);
        return t;
      }
      case 3:
        return ImmutableMap.of();
      case 4:
        return Maps.newLinkedHashMapWithExpectedSize(4);
      default: {
        Map<String, Integer> src = new LinkedHashMap<>();
        src.put("u", 21);
        src.put("b", 2);
        return ImmutableMap.copyOf(src);
      }
    }
  }

  static Map.Entry<String, Integer> firstEntry(Map<String, Integer> m) {
    return m.entrySet().iterator().next();
  }

  static String kind(Object o) {
    if (o instanceof Map<?, ?> m) {
      return "map" + m.size();
    } else if (o instanceof Collection<?> c) {
      return "coll" + c.size();
    } else if (o instanceof Map.Entry<?, ?> e) {
      return "entry" + e.getKey();
    }
    return "other";
  }

  static final class Index {
    final Map<String, List<String>> byKey = new LinkedHashMap<>();
    final Map<String, Map<String, Set<String>>> deep = new LinkedHashMap<>();
    Map<Key, Dog> dogs = ImmutableMap.of();

    void add(String k, String v) {
      byKey.computeIfAbsent(k, x -> new ArrayList<>()).add(v);
      deep.computeIfAbsent(k, x -> new LinkedHashMap<>()).computeIfAbsent(v, x -> new LinkedHashSet<>()).add(k + v);
    }

    List<String> of(String k) {
      return byKey.get(k);
    }

    Map<String, List<String>> all() {
      return byKey;
    }

    Map<Key, Dog> dogs() {
      return dogs;
    }
  }

  static String maps() {
    String r = "";
    for (int i = 0; i < 6; i++) {
      Map<String, Integer> m = pickMap(i);
      r += dump(m) + anyMap(m);
      try {
        r += m.put("b", 7);
        r += m.put("n", null) + "" + m.remove("n") + m.remove("nope") + m.getOrDefault("q", 5) + m.putIfAbsent("b", 9);
      } catch (UnsupportedOperationException e) {
        r += "uoe";
      }
      r += m + ";";
    }
    Map<String, Integer> a = pickMap(0);
    a.put("b", 2);
    a.put("a", 1);
    Map<String, Integer> b = pickMap(1);
    Object o = a;
    r += "|" + a.equals(b) + b.equals(a) + (a.hashCode() == b.hashCode()) + (a == (Object) b) + (o == a) + (o instanceof Map) + ((Map<?, ?>) o).size();
    r += kind(o) + kind(b) + kind(a.keySet()) + kind(a.values()) + kind(firstEntry(a)) + kind(Arrays.asList(1, 2)) + kind(new ArrayList<String>()) + kind("s");
    // entries: live, setValue writes through, equal to another map's entry with the same key and value
    Map.Entry<String, Integer> e = firstEntry(a);
    Map.Entry<String, Integer> f = firstEntry(b);
    r += e.getKey() + e.getValue() + e.equals(f) + (e.hashCode() == f.hashCode()) + e;
    r += e.setValue(20) + "" + a.get("b") + e.getValue() + e.equals(f);
    try {
      f.setValue(3);
      r += "no";
    } catch (UnsupportedOperationException x) {
      r += "uoe";
    }
    List<Map.Entry<String, Integer>> es = new ArrayList<>(a.entrySet());
    es.add(f);
    for (Map.Entry<String, Integer> x : es) {
      r += x.getKey() + x.getValue();
    }
    // views: live, removing through them removes from the map
    Set<String> ks = a.keySet();
    Collection<Integer> vs = a.values();
    Set<Map.Entry<String, Integer>> en = a.entrySet();
    a.put("c", 3);
    r += "|" + ks + vs + en + ks.size() + vs.size() + en.size() + ks.contains("c") + vs.contains(3) + (ks == a.keySet());
    ks.remove("c");
    vs.remove(20);
    r += a;
    Iterator<Map.Entry<String, Integer>> ei = en.iterator();
    ei.next();
    ei.remove();
    r += a + "" + a.isEmpty() + ks.isEmpty();
    try {
      ei.next();
      r += "no";
    } catch (NoSuchElementException x) {
      r += "nse";
    }
    Map<String, Integer> nul = null;
    r += (nul == null);
    try {
      dump(nul);
      r += "no";
    } catch (NullPointerException x) {
      r += "npe";
    }
    // collections as elements: one object per key, typed by its place
    Index ix = new Index();
    ix.add("k", "v1");
    ix.add("k", "v2");
    ix.add("j", "w");
    r += "|" + ix.of("k") + ix.of("j").get(0) + ix.of("none") + (ix.of("k") == ix.byKey.get("k")) + ix.all() + ix.deep;
    for (Map.Entry<String, List<String>> x : ix.all().entrySet()) {
      r += x.getKey() + x.getValue().size() + x.getValue().get(0);
    }
    for (List<String> l : ix.all().values()) {
      r += show(l);
    }
    Map<String, Set<String>> inner = ix.deep.get("k");
    r += inner.keySet() + "" + inner.get("v2") + each(inner.get("v1"));
    Object lo = ix.of("k");
    r += (lo instanceof List) + "" + (lo == ix.all().get("k")) + ((List<?>) lo).size();
    // keys with equals; values of a subclass through a wildcard
    r += wide(ix.dogs()) + ix.dogs().get(new Key("x"));
    ix.dogs = new LinkedHashMap<>();
    Key k1 = new Key("x");
    ix.dogs().put(k1, new Dog("d1"));
    ix.dogs().put(new Key("y"), new Dog("d2"));
    ix.dogs().put(new Key("x"), new Dog("d3"));
    r += wide(ix.dogs()) + ix.dogs().get(new Key("x")).name + (ix.dogs().keySet().iterator().next() == k1) + ix.dogs().containsKey(new Key("y")) + anyMap(ix.dogs());
    // an unchecked cast: nothing is checked until a value is read at its type
    Map<String, Object> raw = new LinkedHashMap<>();
    raw.put("k", "str");
    @SuppressWarnings("unchecked")
    Map<String, Dog> wrong = (Map<String, Dog>) (Map<?, ?>) raw;
    r += wrong.size() + "" + anyMap(wrong) + (wrong == (Object) raw) + wrong.containsKey("k") + String.valueOf(wrong.get("k"));
    try {
      r += wrong.get("k").name;
      r += "no";
    } catch (ClassCastException x) {
      r += "cce";
    }
    try {
      for (Map.Entry<String, Dog> x : wrong.entrySet()) {
        r += x.getKey();
        r += x.getValue().name;
      }
      r += "no";
    } catch (ClassCastException x) {
      r += "cce";
    }
    return r;
  }

  // ---- kind `set`

  static String each(Set<String> c) {
    String r = c.size() + ":";
    for (String s : c) {
      r += s;
    }
    Iterator<String> it = c.iterator();
    while (it.hasNext()) {
      r += it.next().length();
    }
    return r + c.contains("b") + c.isEmpty() + c;
  }

  static String petSet(Set<? extends Animal> c) {
    String r = "";
    for (Animal a : c) {
      r += a.name;
    }
    return r + c.size();
  }

  static int anySet(Set<?> c) {
    int n = 0;
    for (Object o : c) {
      n += o == null ? 100 : 1;
    }
    return n + c.size();
  }

  static Set<String> pickSet(int how) {
    switch (how) {
      case 0:
        return new LinkedHashSet<>();
      case 1:
        return ImmutableSet.of("b", "a");
      case 2:
        return ImmutableSet.copyOf(Arrays.asList("c", "b", "c"));
      case 3:
        return ImmutableSet.of();
      case 4: {
        Set<String> t = new TreeSet<>();
        t.add("z");
        t.add("b");
        t.add("m");
        return t;
      }
      case 5:
        return Sets.newLinkedHashSetWithExpectedSize(4);
      default:
        return new LinkedHashSet<>(Arrays.asList("u", "b"));
    }
  }

  /** A key with equals of its own: looked up by equals and hashCode, not by identity. */
  static final class Key {
    final String k;

    Key(String k) {
      this.k = k;
    }

    @Override
    public boolean equals(Object o) {
      return o instanceof Key && ((Key) o).k.equals(k);
    }

    @Override
    public int hashCode() {
      return k.hashCode();
    }

    @Override
    public String toString() {
      return "K" + k;
    }
  }

  static final class Registry {
    Map<String, Dog> byName = ImmutableMap.of();
    final Map<Key, Integer> byKey = new LinkedHashMap<>();
    Set<Key> seen = new LinkedHashSet<>();

    Set<String> names() {
      return byName.keySet();
    }

    Set<Key> keys() {
      return byKey.keySet();
    }

    Set<Key> seen() {
      return seen;
    }
  }

  static String sets() {
    String r = "";
    for (int i = 0; i < 7; i++) {
      Set<String> s = pickSet(i);
      r += each(s) + anySet(s);
      try {
        r += s.add("b");
        r += s.add("n") + "" + s.remove("n") + s.remove("nope");
      } catch (UnsupportedOperationException e) {
        r += "uoe";
      }
      r += s + ";";
    }
    Set<String> a = pickSet(0);
    a.add("b");
    a.add("a");
    a.add(null);
    Set<String> b = pickSet(1);
    r += "|" + a.equals(b) + a.contains(null) + anySet(a) + a.remove(null) + a.equals(b) + b.equals(a) + (a.hashCode() == b.hashCode()) + (a == (Object) b);
    Object o = a;
    Collection<String> c = a;
    Iterable<String> it = a;
    r += (o == c) + "" + (c == (Object) it) + (o instanceof Set) + ((Set<?>) o).size() + all(c);
    Set<String> nul = null;
    r += (nul == null);
    try {
      each(nul);
      r += "no";
    } catch (NullPointerException e) {
      r += "npe";
    }
    // keySet() of a map that is immutable at first, then one that can change
    Registry g = new Registry();
    r += "|" + each(g.names());
    try {
      g.names().clear();
      r += "cleared";
    } catch (UnsupportedOperationException e) {
      r += "uoe";
    }
    try {
      g.names().iterator().next();
      r += "no";
    } catch (NoSuchElementException e) {
      r += "nse";
    }
    g.byName = new LinkedHashMap<>();
    g.byName.put("rex", new Dog("rex"));
    g.byName.put("b", new Dog("b"));
    Set<String> ns = g.names();
    g.byName.put("ace", new Dog("ace"));
    r += each(ns) + (ns == g.names()) + (ns.equals(g.byName.keySet()));
    try {
      ns.add("x");
      r += "no";
    } catch (UnsupportedOperationException e) {
      r += "uoe";
    }
    Iterator<String> ni = ns.iterator();
    ni.next();
    ni.remove();
    ns.remove("ace");
    r += g.byName.size() + "" + ns + g.byName.containsKey("b");
    // keys with equals: found by equals, kept in insertion order, the first key object stays
    Key k1 = new Key("x");
    Key k2 = new Key("x");
    g.byKey.put(k1, 1);
    g.byKey.put(new Key("y"), 2);
    g.byKey.put(k2, 3);
    Set<Key> ks = g.keys();
    r += "|" + ks + ks.contains(new Key("y")) + ks.contains(new Key("q")) + (ks.iterator().next() == k1) + ks.size();
    Set<Key> seen = g.seen();
    r += seen.add(k1) + "" + seen.add(k2) + seen.contains(k2) + seen + seen.equals(ImmutableSet.of(new Key("x")));
    r += petSet(ImmutableSet.of(new Dog("d1"), new Dog("d2"))) + petSet(new LinkedHashSet<Animal>());
    // sets of sets and an unchecked cast: nothing is checked until an element is read
    List<Set<String>> nested = new ArrayList<>();
    nested.add(a);
    nested.add(b);
    Map<String, Set<String>> bySet = new LinkedHashMap<>();
    bySet.computeIfAbsent("k", k -> new LinkedHashSet<>()).add("v1");
    bySet.computeIfAbsent("k", k -> new LinkedHashSet<>()).add("v2");
    for (Set<String> x : nested) {
      r += each(x);
    }
    r += bySet + each(bySet.get("k")) + (bySet.get("k") == bySet.get("k"));
    Set<Object> raw = new LinkedHashSet<>();
    raw.add(7);
    @SuppressWarnings("unchecked")
    Set<String> wrong = (Set<String>) (Set<?>) raw;
    r += wrong.size() + "" + anySet(wrong) + (wrong == (Object) raw);
    try {
      r += each(wrong);
      r += "no";
    } catch (ClassCastException e) {
      r += "cce";
    }
    return r;
  }

  // ---- kind `coll`

  static String all(Collection<String> c) {
    String r = c.size() + ":";
    for (String s : c) {
      r += s;
    }
    Iterator<String> it = c.iterator();
    while (it.hasNext()) {
      r += it.next().length();
    }
    return r + c.contains("b") + c.isEmpty() + c;
  }

  static String pets(Collection<? extends Animal> c) {
    String r = "";
    for (Animal a : c) {
      r += a.name;
    }
    return r + c.size();
  }

  static int any(Collection<?> c) {
    int n = 0;
    for (Object o : c) {
      n += o == null ? 100 : 1;
    }
    return n + c.size();
  }

  static boolean grow(Collection<String> c, String s) {
    return c.add(s);
  }

  /** A collection of the program under a library class: it has no typed methods of its own. */
  static final class Pair extends AbstractCollection<String> {
    final String a;
    final String b;

    Pair(String a, String b) {
      this.a = a;
      this.b = b;
    }

    @Override
    public Iterator<String> iterator() {
      List<String> l = new ArrayList<>();
      l.add(a);
      l.add(b);
      return l.iterator();
    }

    @Override
    public int size() {
      return 2;
    }
  }

  static final class Zoo {
    Map<String, Dog> byName = ImmutableMap.of();
    final Collection<String> log = new ArrayList<>();

    void add(String n) {
      if (byName.isEmpty()) {
        byName = new LinkedHashMap<>();
      }
      byName.put(n, new Dog(n));
      log.add("+" + n);
    }

    Collection<Dog> dogs() {
      return byName.values();
    }

    Collection<Dog> frozen() {
      return Collections.unmodifiableCollection(byName.values());
    }
  }

  static String colls() {
    List<String> l = new ArrayList<>();
    l.add("a");
    l.add("b");
    Set<String> s = new LinkedHashSet<>(l);
    Deque<String> d = new ArrayDeque<>(l);
    d.push("z");
    ImmutableList<String> il = ImmutableList.of("i", "b");
    ImmutableSet<String> is = ImmutableSet.of("j");
    String r = all(l) + all(s) + all(d) + all(il) + all(is) + all(new Pair("p", "b")) + all(Collections.emptyList());
    Collection<String> c = l;
    grow(c, "c");
    grow(s, "c");
    r += "|" + l + s + (c == (Object) l) + c.equals(l) + (c.hashCode() == l.hashCode()) + (c instanceof List);
    try {
      grow(il, "x");
      r += "no";
    } catch (UnsupportedOperationException e) {
      r += "uoe";
    }
    Collection<String> nul = null;
    r += (nul == null);
    try {
      all(nul);
      r += "no";
    } catch (NullPointerException e) {
      r += "npe";
    }
    // values() of a map that is immutable at first, then one that can change
    Zoo z = new Zoo();
    r += "|" + pets(z.dogs()) + any(z.dogs()) + z.dogs().isEmpty();
    try {
      z.dogs().clear();
      r += "cleared";
    } catch (UnsupportedOperationException e) {
      r += "uoe";
    }
    try {
      Iterator<Dog> it = z.dogs().iterator();
      it.next();
      r += "no";
    } catch (NoSuchElementException e) {
      r += "nse";
    }
    z.add("rex");
    z.add("ace");
    Collection<Dog> ds = z.dogs();
    z.add("bo");
    r += pets(ds) + any(ds) + (ds == z.dogs()) + (z.frozen().size()) + pets(z.frozen()) + all(z.log);
    Iterator<Dog> it = ds.iterator();
    it.next();
    it.remove();
    ds.remove(z.byName.get("bo"));
    r += z.byName.keySet() + "" + ds + ds.contains(z.byName.get("ace"));
    Object o = ds;
    r += (o == z.byName.values()) + "" + (o instanceof Collection) + ((Collection<?>) o).size();
    List<Collection<String>> nested = new ArrayList<>();
    nested.add(l);
    nested.add(is);
    for (Collection<String> x : nested) {
      r += all(x);
    }
    return r;
  }

  // ---- kind `list`, piece `place`

  /** A list that one path makes with a runtime function, so that its declaration is no typed list of rule "typed collections". */
  static List<String> pick(int how) {
    List<String> r;
    switch (how) {
      case 0:
        r = new ArrayList<>();
        r.add("n");
        r.add("m");
        break;
      case 1:
        r = Arrays.asList("x", "y", "z");
        break;
      case 2:
        r = ImmutableList.of("i", "j");
        break;
      case 3:
        r = Collections.emptyList();
        break;
      case 4:
        r = new ArrayList<>(Arrays.asList("s", "t", "u", "v")).subList(1, 3);
        break;
      default:
        r = null;
    }
    return r;
  }

  static String show(List<String> l) {
    String r = l.size() + "";
    for (int i = 0; i < l.size(); i++) {
      r += l.get(i);
    }
    for (String s : l) {
      r += s + s.length();
    }
    return r + l + l.indexOf("y") + l.contains("j") + l.isEmpty();
  }

  static String firstName(List<? extends Animal> l) {
    return l.isEmpty() ? "-" : l.get(0).name + l.size();
  }

  interface Source {
    List<Animal> animals();
  }

  static final class DogSource implements Source {
    final List<Animal> list = new ArrayList<>();

    @Override
    public List<Animal> animals() {
      return list;
    }
  }

  static final class FixedSource implements Source {
    @Override
    public List<Animal> animals() {
      return ImmutableList.of(new Animal("cat"), new Dog("dog"));
    }
  }

  static String lists() {
    String r = "";
    for (int i = 0; i < 5; i++) {
      r += show(pick(i)) + ";";
    }
    List<String> a = pick(0);
    a.add("o");
    a.set(0, "N");
    a.remove(1);
    a.add(0, "first");
    a.remove("o");
    a.addAll(pick(1));
    Collections.sort(a);
    r += a + "" + a.hashCode() + a.equals(Arrays.asList("N", "first", "x", "y", "z")) + (a == pick(0)) + (pick(5) == null);
    List<String> fixed = pick(1);
    fixed.set(1, "Y");
    r += fixed + show(fixed.subList(1, 3));
    // (only the immutable list: the runtime's Arrays.asList and Collections.emptyList are lists that can grow)
    for (int how = 2; how <= 2; how++) {
      try {
        pick(how).add("more");
        r += "added";
      } catch (UnsupportedOperationException e) {
        r += "uoe";
      }
    }
    try {
      r += pick(2).get(2);
    } catch (IndexOutOfBoundsException e) {
      r += "ioobe";
    }
    try {
      r += pick(5).get(0);
    } catch (NullPointerException e) {
      r += "npe";
    }
    DogSource ds = new DogSource();
    ds.list.add(new Dog("d1"));
    List<Source> srcs = new ArrayList<>();
    srcs.add(ds);
    srcs.add(new FixedSource());
    for (Source src : srcs) {
      List<Animal> as = src.animals();
      r += "|" + firstName(as) + as.get(as.size() - 1) + names(as) + pets(as) + (as == src.animals()) + (as.equals(src.animals()));
    }
    List<Dog> dogs = new ArrayList<>();
    dogs.add(new Dog("rex"));
    r += firstName(dogs) + firstName(ImmutableList.of(new Dog("im"))) + firstName(Collections.unmodifiableList(dogs));
    // collections of lists
    Map<String, List<String>> byKey = new LinkedHashMap<>();
    byKey.computeIfAbsent("k", k -> new ArrayList<>()).add("v1");
    byKey.computeIfAbsent("k", k -> new ArrayList<>()).add("v2");
    byKey.put("i", ImmutableList.of("w"));
    List<String> got = byKey.get("k");
    r += "|" + show(got) + (got == byKey.get("k")) + show(byKey.get("i")) + byKey;
    for (Map.Entry<String, List<String>> e : byKey.entrySet()) {
      List<String> v = e.getValue();
      r += e.getKey() + v.get(0);
    }
    List<List<String>> ll = new ArrayList<>();
    ll.add(got);
    ll.add(pick(1));
    for (List<String> x : ll) {
      r += x.get(x.size() - 1);
    }
    // an unchecked cast: nothing fails until an element is read at its type
    List<Object> mixed = new ArrayList<>();
    mixed.add("s");
    mixed.add(1);
    Object o = mixed;
    @SuppressWarnings("unchecked")
    List<String> wrong = (List<String>) o;
    r += wrong.size() + wrong.get(0);
    try {
      String s = wrong.get(1);
      r += s;
    } catch (ClassCastException e) {
      r += "cce";
    }
    return r + (wrong == o) + (o instanceof List);
  }

  public static String run() {
    p(maps());
    p(sets());
    p(lists());
    p(colls());
    p(views());
    p(loops());
    p(variance());
    p(identity());
    p(failures());
    String s = out.toString();
    out.setLength(0);
    return s;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
