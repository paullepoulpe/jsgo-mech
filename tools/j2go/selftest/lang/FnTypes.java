package lang;

import static com.google.common.base.Preconditions.checkNotNull;

import com.google.common.base.Predicates;
import com.google.common.base.Suppliers;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.Iterables;
import com.google.common.collect.Ordering;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
import java.util.TreeSet;
import java.util.concurrent.Callable;
import java.util.function.BiConsumer;
import java.util.function.BiFunction;
import java.util.function.Consumer;
import java.util.function.Function;
import java.util.function.Predicate;
import java.util.function.Supplier;
import java.util.function.ToIntFunction;

/**
 * Rule "function types" (j2go.FnTypes): library functional interfaces as Go func types. The
 * values must behave as the adapter objects of the erased translation did.
 */
public class FnTypes {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  static class Animal {
    final String name;

    Animal(String name) {
      this.name = name;
    }

    String name() {
      return name;
    }

    @Override
    public String toString() {
      return "Animal(" + name + ")";
    }
  }

  static class Dog extends Animal {
    Dog(String name) {
      super(name);
    }

    @Override
    public String toString() {
      return "Dog(" + name + ")";
    }
  }

  // ---- classes that implement a functional interface
  static final class LongerThan implements Predicate<String> {
    final int n;

    LongerThan(int n) {
      this.n = n;
    }

    @Override
    public boolean test(String s) {
      return s.length() > n;
    }
  }

  static final class GuavaStartsWith implements com.google.common.base.Predicate<String> {
    final String prefix;

    GuavaStartsWith(String prefix) {
      this.prefix = prefix;
    }

    @Override
    public boolean apply(String s) {
      return s.startsWith(prefix);
    }
  }

  static class Counter implements Supplier<Integer> {
    int n;

    @Override
    public Integer get() {
      n++;
      return n;
    }
  }

  static class Doubling extends Counter {
    @Override
    public Integer get() {
      n += 2;
      return n;
    }
  }

  static <T> int count(Iterable<T> xs, Predicate<? super T> p) {
    int n = 0;
    for (T x : xs) {
      if (p.test(x)) {
        n++;
      }
    }
    return n;
  }

  static String names(List<? extends Animal> xs, Function<? super Animal, ? extends CharSequence> f) {
    StringBuilder sb = new StringBuilder();
    for (Animal a : xs) {
      sb.append(f.apply(a)).append(';');
    }
    return sb.toString();
  }

  static int sum(List<String> xs, ToIntFunction<String> f) {
    int n = 0;
    for (String x : xs) {
      n += f.applyAsInt(x);
    }
    return n;
  }

  static <T> T callIt(Callable<T> c) {
    try {
      return c.call();
    } catch (Exception e) {
      throw new RuntimeException(e);
    }
  }

  static List<String> words() {
    List<String> w = new ArrayList<>();
    w.add("a");
    w.add("bb");
    w.add("ccc");
    w.add("dddd");
    return w;
  }

  static String lambdas() {
    List<String> words = words();
    int limit = 1;
    Predicate<String> longer = s -> s.length() > limit;
    Predicate<Object> notNull = o -> o != null;
    Supplier<String> hello = () -> "hello";
    Function<String, Integer> len = String::length;
    BiFunction<String, Integer, String> rep = (s, n) -> s.repeat(n);
    StringBuilder sb = new StringBuilder();
    Consumer<String> add = sb::append;
    BiConsumer<String, Integer> add2 = (s, n) -> sb.append(s).append(n);
    add.accept("x");
    add2.accept("y", 2);
    return count(words, longer) + " " + count(words, notNull) + " " + hello.get() + " " + len.apply("four") + " " + rep.apply("ab", 2) + " " + sb
        + " " + sum(words, String::length) + " " + callIt(() -> "called") + " " + callIt(() -> 7);
  }

  static String objects() {
    List<String> words = words();
    Predicate<String> p = new LongerThan(2);
    com.google.common.base.Predicate<String> g = new GuavaStartsWith("b");
    Predicate<String> asJava = g;
    Counter c = new Doubling();
    Supplier<Integer> s = c;
    Supplier<Integer> plain = new Counter();
    LongerThan none = null;
    Predicate<String> nullPred = none;
    return count(words, p) + " " + count(words, g) + " " + count(words, asJava) + " " + g.apply("bx") + g.test("xb") + " " + s.get() + s.get() + " " + plain.get() + plain.get()
        + " " + (nullPred == null) + " " + count(words, new LongerThan(0)) + " " + Iterables.size(Iterables.filter(words, g));
  }

  static String variance() {
    List<Dog> dogs = ImmutableList.of(new Dog("rex"), new Dog("fido"));
    Function<Animal, String> name = Animal::name;
    Function<Object, String> str = String::valueOf;
    Predicate<Object> any = Predicates.alwaysTrue();
    Predicate<Animal> isRex = a -> a.name.equals("rex");
    return names(dogs, name) + " " + names(dogs, str) + " " + count(dogs, any) + " " + count(dogs, isRex) + " " + count(dogs, x -> x != null);
  }

  static String erased() {
    // functions as values of erased collections and as arguments of the runtime
    Map<String, Function<String, Object>> table = new LinkedHashMap<>();
    table.put("len", s -> s.length());
    table.put("up", s -> s + s);
    ImmutableMap<String, Supplier<String>> im = ImmutableMap.of("a", () -> "A", "b", () -> "B");
    List<Supplier<String>> list = new ArrayList<>();
    list.add(() -> "one");
    list.add(im.get("b"));
    StringBuilder sb = new StringBuilder();
    for (Map.Entry<String, Function<String, Object>> e : table.entrySet()) {
      sb.append(e.getKey()).append('=').append(e.getValue().apply("abc")).append(' ');
    }
    for (Supplier<String> s : list) {
      sb.append(s.get()).append(' ');
    }
    Map<String, List<String>> groups = new LinkedHashMap<>();
    groups.computeIfAbsent("k", k -> new ArrayList<>()).add("v1");
    groups.computeIfAbsent("k", k -> new ArrayList<>()).add("v2");
    Map<String, Integer> counts = new LinkedHashMap<>();
    counts.merge("x", 1, Integer::sum);
    counts.merge("x", 1, Integer::sum);
    groups.forEach((k, v) -> sb.append(k).append(v));
    ImmutableList.of("p", "q").forEach(sb::append);
    Optional<String> o = Optional.of("opt").filter(x -> x.length() == 3);
    Optional<String> e = Optional.of("opt").filter(x -> x.isEmpty());
    o.ifPresent(sb::append);
    return sb + " " + counts + " " + o.isPresent() + e.isPresent() + " " + table.get("len").apply("12345") + " " + (table.get("nope") == null) + " " + im.get("a").get();
  }

  record Named(String name, Supplier<String> value) {}

  static int made;

  static String identityAndNull() {
    Supplier<String> s = () -> "s";
    Supplier<String> t = () -> "t";
    Named a = new Named("n", s), b = new Named("n", s), c = new Named("n", t);
    Supplier<String> memo = Suppliers.memoize(() -> {
          made++;
          return "made" + made;
        });
    Supplier<String> none = null;
    String npe;
    try {
      npe = none.get();
    } catch (NullPointerException ex) {
      npe = "npe";
    }
    String npe2;
    try {
      checkNotNull(none);
      npe2 = "no";
    } catch (NullPointerException ex) {
      npe2 = "npe";
    }
    Supplier<String> checked = checkNotNull(s);
    Supplier<String> req = Objects.requireNonNull(t, "t");
    return a.equals(b) + " " + a.equals(c) + " " + (a.hashCode() == b.hashCode()) + " " + a.equals(a) + " " + memo.get() + memo.get() + made + " " + npe + npe2 + " " + checked.get() + req.get()
        + " " + a.value().get() + (none == null) + (s != null);
  }

  static <T, R> List<R> map(List<T> xs, Function<? super T, ? extends R> f) {
    List<R> out = new ArrayList<>();
    for (T x : xs) {
      out.add(f.apply(x));
    }
    return out;
  }

  static Function<Integer, Function<Integer, Integer>> adder() {
    return a -> b -> a + b;
  }

  static String generic() {
    List<Integer> lens = map(ImmutableList.of("a", "bbb"), String::length);
    List<Animal> animals = map(ImmutableList.of("x", "y"), Dog::new);
    Function<Integer, Integer> add3 = adder().apply(3);
    Supplier<Supplier<String>> nested = () -> () -> "nested";
    return lens + " " + animals + " " + add3.apply(4) + " " + nested.get().get();
  }

  // ---- comparators
  static final class ByLength implements Comparator<String> {
    @Override
    public int compare(String a, String b) {
      return Integer.compare(a.length(), b.length());
    }
  }

  static final class ByName extends Ordering<Animal> {
    @Override
    public int compare(Animal a, Animal b) {
      return a.name.compareTo(b.name);
    }
  }

  static <T> T max(List<T> xs, Comparator<? super T> c) {
    T best = xs.get(0);
    for (T x : xs) {
      if (c.compare(x, best) > 0) {
        best = x;
      }
    }
    return best;
  }

  static String comparators() {
    List<String> words = words();
    words.add("zz");
    words.add("b");
    Comparator<String> byLen = new ByLength();
    Comparator<String> lambda = (a, b) -> b.compareTo(a);
    Comparator<String> lenThenText = Comparator.comparingInt(String::length).thenComparing(Comparator.naturalOrder());
    Comparator<String> keyed = Comparator.comparing(s -> s.charAt(0));
    Comparator<Object> byString = Comparator.comparing(String::valueOf);
    List<String> a = new ArrayList<>(words), b = new ArrayList<>(words), c = new ArrayList<>(words), d = new ArrayList<>(words);
    a.sort(byLen);
    b.sort(lambda);
    c.sort(lenThenText.reversed());
    Collections.sort(d, lenThenText);
    Set<String> tree = new TreeSet<>(byLen.thenComparing(lambda));
    tree.addAll(words);
    List<Dog> dogs = new ArrayList<>();
    dogs.add(new Dog("rex"));
    dogs.add(new Dog("ace"));
    Ordering<Animal> byName = new ByName();
    Comparator<Animal> asCmp = byName;
    dogs.sort(byName);
    Comparator<String> none = null;
    return a + " " + b + " " + c + " " + d + " " + tree + " " + dogs + " " + max(words, byLen) + " " + max(words, keyed) + " " + max(dogs, asCmp) + " " + max(words, byString)
        + " " + byLen.compare("a", "bb") + " " + (none == null) + " " + byName.compare(dogs.get(0), dogs.get(1)) + " " + max(words, byLen.thenComparingInt(s -> -s.charAt(0)));
  }

  public static String run() {
    p(lambdas());
    p(comparators());
    p(objects());
    p(variance());
    p(erased());
    p(identityAndNull());
    p(generic());
    String s = out.toString();
    out.setLength(0);
    return s;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
