package lang;

import com.google.common.base.MoreObjects;
import com.google.common.base.Splitter;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.Iterables;
import com.google.common.collect.Lists;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collections;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.TreeSet;

/**
 * Rule "typed library interfaces", piece `fns` (j2go.LibFns): the typed twins of the runtime's
 * library functions. typed() has calls of the twins; erased() has calls that must stay erased
 * (variance, an erased argument, a map whose keys are not Strings, a function without a twin):
 * tools/j2go/selftest/run.sh counts both in the translation. Both must print what Java prints.
 */
public class LibFns {
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
      return "A:" + name;
    }
  }

  static class Dog extends Animal {
    Dog(String name) {
      super(name);
    }
  }

  static final Splitter COMMA = Splitter.on(',');

  static String typed() {
    List<String> names = new ArrayList<>(Arrays.asList("b", "a", "c"));
    Set<String> set = new LinkedHashSet<>(names);
    List<String> none = new ArrayList<>();
    String r = Iterables.getFirst(names, null) + Iterables.getFirst(none, "d") + (Iterables.getFirst(none, null) == null);
    r += Iterables.getLast(names) + Iterables.getLast(set) + Iterables.get(set, 1) + Iterables.get(names, 2);
    r += Iterables.getOnlyElement(Arrays.asList("x"));
    try {
      Iterables.getOnlyElement(names);
    } catch (IllegalArgumentException e) {
      r += " " + e.getMessage();
    }
    try {
      Iterables.getLast(none);
    } catch (java.util.NoSuchElementException e) {
      r += " nse";
    }
    try {
      Iterables.get(set, 7);
    } catch (IndexOutOfBoundsException e) {
      r += " " + e.getMessage();
    }
    r += Iterables.indexOf(names, s -> s.equals("a")) + "" + Iterables.indexOf(names, s -> s.isEmpty());
    for (String s : Iterables.filter(names, s -> !s.equals("a"))) r += s;
    Iterable<Integer> lens = Iterables.transform(names, s -> s.length() + 10);
    r += lens + "" + Iterables.getLast(Iterables.transform(set, s -> s + "!"));
    String nul = null;
    r += MoreObjects.firstNonNull(nul, "fb") + MoreObjects.firstNonNull("f1", "f2");
    try {
      MoreObjects.firstNonNull(nul, nul);
    } catch (NullPointerException e) {
      r += " " + e.getMessage();
    }
    p(r);
    StringBuilder sb = new StringBuilder();
    names.forEach(s -> sb.append(s).append(';'));
    set.forEach(sb::append);
    names.sort((a, b) -> b.compareTo(a));
    sb.append(names);
    Collections.sort(names);
    sb.append(names);
    Collections.sort(names, (a, b) -> b.compareTo(a));
    sb.append(names);
    List<Animal> zoo = new ArrayList<>(Arrays.asList(new Animal("z"), new Dog("m"), new Animal("a")));
    zoo.sort(Comparator.comparing(a -> a.name));
    sb.append(zoo);
    zoo.sort(null == zoo ? null : (x, y) -> y.name.compareTo(x.name));
    sb.append(zoo);
    p(sb);
    Map<String, Integer> m = new LinkedHashMap<>();
    m.put("one", 1);
    m.put("two", 2);
    StringBuilder mb = new StringBuilder();
    m.forEach((k, v) -> mb.append(k).append('=').append(v).append(' '));
    m.replaceAll((k, v) -> v * 10 + k.length());
    mb.append(m);
    mb.append(m.computeIfAbsent("three", k -> k.length())).append(m.computeIfAbsent("one", k -> 99));
    mb.append(m.computeIfAbsent("none", k -> null)).append(m.containsKey("none"));
    mb.append(m.getOrDefault("two", -1)).append(m.getOrDefault("zz", -1)).append(m.get("three")).append(m.get("nope"));
    String key = "one";
    mb.append(m.get(key)).append(m.getOrDefault(key + "x", 7));
    mb.append(m.merge("one", 5, Integer::sum)).append(m.merge("new", 5, Integer::sum)).append(m.merge("two", 0, (a, b) -> null)).append(m);
    Map<Animal, String> byAnimal = new LinkedHashMap<>();
    Animal cat = new Animal("cat");
    byAnimal.put(cat, "meow");
    mb.append(byAnimal.getOrDefault(cat, "?")).append(byAnimal.getOrDefault(new Animal("x"), "?"));
    p(mb);
    Comparator<String> byLen = Comparator.comparing(String::length);
    TreeSet<String> ts = new TreeSet<>(byLen.thenComparing((a, b) -> a.compareTo(b)));
    ts.addAll(Arrays.asList("ccc", "a", "bb", "aa", "a"));
    Comparator<String> nullCmp = null;
    TreeSet<String> natural = new TreeSet<>(nullCmp);
    natural.addAll(Arrays.asList("q", "b"));
    TreeMap<String, Integer> tm = new TreeMap<>((a, b) -> b.compareTo(a));
    tm.put("a", 1);
    tm.put("c", 3);
    tm.put("b", 2);
    p(ts + " " + ts.contains("bb") + ts.contains("b") + " " + natural + " " + tm + " " + tm.get("c") + ts.remove("aa") + ts);
    String csv = "x,y,,z";
    List<String> parts = COMMA.splitToList(csv);
    p(parts + " " + parts.size() + " " + COMMA.splitToList("") + COMMA.splitToList(csv.substring(2)));
    return "typed done";
  }

  static String erased() {
    List<Dog> dogs = new ArrayList<>(Arrays.asList(new Dog("rex")));
    Animal fallback = new Animal("none");
    // variance: T is Animal, the list is one of Dogs
    Animal first = Iterables.getFirst(dogs, fallback);
    Iterable<?> wild = dogs;
    Object w = Iterables.getFirst(wild, null);
    Map<Object, String> byObj = new LinkedHashMap<>();
    byObj.put("k", "v");
    String got = byObj.get("k");
    List<String> rev = Lists.reverse(new ArrayList<>(Arrays.asList("1", "2", "3")));
    return first + " " + w + " " + got + " " + rev;
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
