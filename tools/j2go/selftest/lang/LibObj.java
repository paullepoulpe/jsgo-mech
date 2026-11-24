package lang;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import java.util.ArrayDeque;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Deque;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;

/**
 * Rule "typed library interfaces", piece `objarg` (j2go.LibObj): Map.get / containsKey / remove,
 * Collection.contains / remove and List.indexOf / lastIndexOf take an Object in Java; with an
 * argument of the key or element type (or a subtype) they are the typed twins. The receivers are
 * typed interfaces (parameters of methods with a type parameter, which rule "typed collections"
 * does not make typed collections) and typed collections whose own methods do
 * not serve the call. typed() has calls of the twins; erased() calls that must stay erased (an
 * Object, a supertype, another interface, a map of Object keys, a wildcard, a result that enters
 * an Object, a field of a type variable's type): tools/j2go/selftest/run.sh counts both. Both
 * print what Java prints, null keys and keys equal by equals included.
 */
public class LibObj {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  interface Named {
    String name();
  }

  static class Animal implements Named {
    final String name;

    Animal(String name) {
      this.name = name;
    }

    @Override
    public String name() {
      return name;
    }

    @Override
    public String toString() {
      return "A:" + name;
    }
  }

  static class Dog extends Animal {
    Dog(String name) {
      super(name);
    }

    @Override
    public String toString() {
      return "D:" + name;
    }
  }

  /** A key compared by equals and hashCode: an equal key that is another object finds the entry. */
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
      return "K:" + k;
    }
  }

  static final Comparator<Animal> BY_NAME = (x, y) -> x == null ? (y == null ? 0 : -1) : y == null ? 1 : x.name.compareTo(y.name);
  static final Comparator<Key> BY_KEY = (x, y) -> x == null ? (y == null ? 0 : -1) : y == null ? 1 : x.k.compareTo(y.k);

  static final Animal CAT = new Animal("cat");
  static final Dog REX = new Dog("rex");

  // the receivers: typed interfaces (each is given collections of several classes)

  static <X> String maps(Map<Animal, String> m, Dog d, Animal a) {
    Animal none = null;
    String r = m.get(a) + " " + m.get(d) + " " + m.get(none) + " " + m.containsKey(d) + m.containsKey(a) + m.containsKey(none);
    r += " " + m.get(new Dog("other")) + " " + m.get(CAT) + m.containsKey(REX);
    // a method reference: its parameter is the key
    java.util.function.Function<Animal, String> f = m::get;
    return r + " " + f.apply(a) + f.apply(none);
  }

  static <X> String change(Map<Animal, String> m, Dog d, Animal a) {
    Animal none = null;
    String r = m.remove(d) + " " + m.remove(d) + " " + m.remove(none) + " " + m.containsKey(none) + " " + m.remove(a);
    return r + " " + m;
  }

  static <X> String keys(Map<Key, Integer> m) {
    Key k = new Key("a");
    Integer v = m.get(k);
    return v + " " + m.containsKey(new Key("b")) + m.containsKey(new Key("z")) + " " + m.remove(new Key("b")) + " " + m;
  }

  static <X> String colls(Collection<Animal> c, Dog d, Animal a) {
    Animal none = null;
    String r = c.contains(d) + " " + c.contains(a) + " " + c.contains(none) + " " + c.contains(new Dog("x"));
    r += " " + c.remove(d) + c.remove(d) + c.remove(none) + " " + c;
    return r;
  }

  static <X> String lists(List<Animal> l, Dog d, Animal a) {
    Animal none = null;
    return l.indexOf(d) + " " + l.indexOf(a) + " " + l.lastIndexOf(d) + " " + l.indexOf(none) + " " + l.lastIndexOf(none) + " " + l.contains(d) + l.remove(a) + " " + l;
  }

  static <X> String sets(Set<Key> s) {
    Key none = null;
    return s.contains(new Key("a")) + " " + s.contains(none) + " " + s.remove(new Key("a")) + s.remove(new Key("a")) + s.remove(none) + " " + s;
  }

  static <X> String strs(List<String> l, String s) {
    // a String in a list: List.indexOf has no String-key helper
    String none = null;
    return l.indexOf(s) + " " + l.lastIndexOf(s) + " " + l.indexOf(none);
  }

  /** Keys of a type parameter of a generic class (a Go type parameter). */
  static class Reg<T> {
    <X> String look(Map<T, Integer> m, Set<T> s, T k) {
      return m.get(k) + " " + m.containsKey(k) + " " + s.contains(k) + " " + m.remove(k) + m.containsKey(k) + s.remove(k) + s.contains(k);
    }
  }

  static String typed() {
    Dog d = new Dog("d");
    Animal a = new Animal("a");
    Map<Animal, String> lm = new LinkedHashMap<>();
    lm.put(d, "dog");
    lm.put(a, "animal");
    lm.put(null, "null key");
    lm.put(REX, "rex");
    Map<Animal, String> hm = new TreeMap<>(BY_NAME);
    hm.putAll(lm);
    p(maps(lm, d, a));
    p(maps(hm, d, a));
    Map<Animal, String> im = new LinkedHashMap<>();
    im.put(d, "imm");
    im.put(REX, "rex");
    p(maps(ImmutableMap.copyOf(im), d, a));
    p(change(lm, d, a));
    p(change(hm, d, a));
    // a map's key set and a typed map's view of Collection
    Map<Animal, String> km = new LinkedHashMap<>();
    km.put(d, "x");
    km.put(a, "y");
    p(colls(km.keySet(), d, a));
    Map<Key, Integer> eq = new LinkedHashMap<>();
    eq.put(new Key("a"), 1);
    eq.put(new Key("b"), 2);
    p(keys(eq));
    Map<Key, Integer> tk = new TreeMap<>(BY_KEY);
    tk.put(new Key("a"), 3);
    tk.put(new Key("b"), 4);
    p(keys(tk));
    List<Animal> la = new ArrayList<>(Arrays.asList(a, d, null, d, REX));
    p(colls(la, d, a));
    p(colls(new LinkedHashSet<>(Arrays.asList(d, a, CAT)), d, a));
    p(colls(new ArrayDeque<>(Arrays.asList(d, a)), d, a));
    p(lists(new ArrayList<>(Arrays.asList(a, d, null, d, REX, null)), d, a));
    p(lists(new ArrayList<>(ImmutableList.copyOf(Arrays.asList(d, a, d))), d, a));
    Set<Key> ks = new LinkedHashSet<>(Arrays.asList(new Key("a"), new Key("b")));
    ks.add(null);
    p(sets(ks));
    Set<Key> tks = new TreeSet<>(BY_KEY);
    tks.add(new Key("a"));
    tks.add(null);
    p(sets(tks));
    p(strs(new ArrayList<>(Arrays.asList("x", "y", "x", null)), "x"));
    p(strs(ImmutableList.of("y"), "y"));
    Map<Named, Integer> gm = new LinkedHashMap<>();
    gm.put(d, 1);
    Set<Named> gs = new LinkedHashSet<>(Arrays.asList(d, a));
    p(new Reg<Named>().look(gm, gs, d));
    Map<String, Integer> sm = new TreeMap<>();
    sm.put("k", 7);
    Set<String> ss = new TreeSet<>();
    ss.add("k");
    p(new Reg<String>().look(sm, ss, "k"));
    // a typed list whose own methods do not serve indexOf / contains: the view
    List<Animal> own = new ArrayList<>();
    own.add(d);
    own.add(a);
    p(own.indexOf(d) + " " + own.lastIndexOf(a) + " " + own.contains(d) + own.remove(d) + " " + own);
    Deque<Animal> dq = new ArrayDeque<>();
    dq.add(d);
    p(dq.contains(d) + " " + dq.remove(d) + dq.contains(d));
    return "typed done";
  }

  static class Holder<T> {
    T item;

    Holder(T item) {
      this.item = item;
    }
  }

  static <X> String wide(Map<Dog, String> m, Collection<Dog> c, List<Dog> l, Object o, Animal a, Named n) {
    // an Object, a supertype, another interface: the erased method
    return m.get(o) + " " + m.get(a) + " " + m.containsKey(n) + " " + c.contains(a) + c.remove(o) + " " + l.indexOf(a) + l.lastIndexOf(n);
  }

  static String erased() {
    Dog d = new Dog("d");
    Map<Dog, String> m = new LinkedHashMap<>();
    m.put(d, "dog");
    Map<Dog, String> h = new TreeMap<>(BY_NAME);
    h.putAll(m);
    List<Dog> l = new ArrayList<>(Arrays.asList(d));
    String r = wide(m, l, l, d, d, d) + " " + wide(new LinkedHashMap<>(m), new LinkedHashSet<>(l), new ArrayList<>(l), CAT, CAT, CAT);
    // keys that are Objects in Go, a wildcard
    Map<Object, String> byObj = new LinkedHashMap<>();
    byObj.put(d, "o");
    Map<Object, String> byObj2 = ImmutableMap.copyOf(byObj);
    r += " " + obj(byObj, d) + obj(byObj2, d);
    r += " " + wild(m, d);
    // the result enters an Object
    Object got = ((Map<Dog, String>) h).get(d);
    // a field of a type variable's type
    Holder<Dog> hd = new Holder<>(d);
    r += " " + got + " " + h.containsKey(hd.item);
    return r;
  }

  static String wild(Map<? extends Animal, String> m, Dog d) {
    return m.get(d) + m.containsKey(d);
  }

  static String obj(Map<Object, String> m, Dog d) {
    return m.get(d) + m.containsKey(d);
  }

  public static String run() {
    p(typed());
    p(erased());
    String s = out.toString();
    out.setLength(0);
    return s;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
