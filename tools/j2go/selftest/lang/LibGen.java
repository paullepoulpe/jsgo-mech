package lang;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.Iterables;
import com.google.common.collect.Maps;
import com.google.common.collect.Sets;
import java.util.ArrayList;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.NoSuchElementException;
import java.util.Set;

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

  public static String run() {
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
