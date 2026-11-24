package lang;

import com.google.common.base.Function;
import com.google.common.base.Optional;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/**
 * Rule "typed library interfaces", kind `goptional`: Guava's Optional as a value library class
 * (the runtime's GOptional[T]). Every method the runtime has (LibGen.GOPT_METHODS), the three
 * statics, the two overloads of `or` it has, null, Object and back, instantiations that meet,
 * an interface method (a bridge), elements of collections. Must print what Java prints, with
 * the kind on and off.
 */
public class GOpt {
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

    @Override
    public boolean equals(Object o) {
      return o instanceof Animal a && a.name.equals(name);
    }

    @Override
    public int hashCode() {
      return name.hashCode();
    }
  }

  static class Dog extends Animal {
    Dog(String name) {
      super(name);
    }
  }

  interface Source<T> {
    Optional<T> find(String key);
  }

  static class MapSource implements Source<String> {
    final Map<String, String> m = new LinkedHashMap<>();

    @Override
    public Optional<String> find(String key) {
      return Optional.fromNullable(m.get(key));
    }
  }

  static Optional<String> never; // null: never assigned
  static Optional<Boolean> flag = Optional.absent();
  static Optional<Integer> count = Optional.of(3);

  static String name(Optional<? extends Animal> a) {
    return a.isPresent() ? a.get().name : "-";
  }

  static String caught(Runnable r) {
    try {
      r.run();
      return "ok";
    } catch (RuntimeException e) {
      return e.getClass().getSimpleName() + ":" + e.getMessage();
    }
  }

  /** The class of what r throws (a call on null: the JVM's message is its own). */
  static String thrown(Runnable r) {
    try {
      r.run();
      return "ok";
    } catch (RuntimeException e) {
      return e.getClass().getSimpleName();
    }
  }

  static String run1() {
    Optional<String> a = Optional.of("a");
    Optional<String> none = Optional.absent();
    Optional<String> fromNull = Optional.fromNullable(null);
    Optional<String> fromX = Optional.fromNullable("x");
    p(a + " " + none + " " + fromNull + " " + fromX);
    p(a.isPresent() + " " + none.isPresent() + " " + a.get() + " " + a.or("z") + " " + none.or("z") + " " + a.orNull() + " " + none.orNull());
    p(none.or(fromX) + " " + a.or(fromX) + " " + none.or(none));
    p(a.equals(Optional.of("a")) + " " + a.equals(fromX) + " " + none.equals(fromNull) + " " + a.equals(none) + " " + a.equals("a") + " " + a.equals(java.util.Optional.of("a")));
    p((a.hashCode() == Optional.of("a").hashCode()) + " " + none.hashCode() + " " + (Optional.of(1).hashCode() - Optional.of(0).hashCode()));
    Optional<Integer> len = a.transform(s -> s.length() + 10);
    Optional<Integer> noLen = none.transform(String::length);
    p(len + " " + noLen + " " + (len.get() + 1));
    Function<String, Optional<String>> wrap = v -> Optional.fromNullable(v);
    p(wrap.apply("w") + " " + wrap.apply(null));
    // null, and the methods of null
    p((never == null) + " " + (a != null) + " " + thrown(() -> never.isPresent()));
    p(caught(() -> none.get()) + " " + caught(() -> Optional.of(null)) + " " + caught(() -> none.or((String) null)));
    p(caught(() -> a.transform(s -> null)));
    // Boolean and Integer contents (boxes)
    flag = Optional.of(true);
    p(flag.get() + " " + flag.or(false) + " " + count.get() * 2 + " " + Optional.<Boolean>absent().or(flag).get());
    // Object and back
    Object o = a;
    Object n = never;
    p((o instanceof Optional) + " " + (n instanceof Optional) + " " + o.equals(Optional.of("a")) + " " + o);
    Optional<?> w = (Optional<?>) o;
    p(w.isPresent() + " " + w.get() + " " + w.orNull());
    List<Object> objs = new ArrayList<>();
    objs.add(a);
    objs.add(none);
    p(objs + " " + objs.contains(Optional.of("a")) + " " + objs.indexOf(Optional.absent()));
    // instantiations that meet: Optional<Dog> where Optional<? extends Animal> is declared
    Optional<Dog> dog = Optional.of(new Dog("rex"));
    Optional<Animal> any = Optional.of(new Animal("cat"));
    p(name(dog) + " " + name(any) + " " + name(Optional.<Dog>absent()));
    Optional<Animal> either = Optional.<Animal>absent().or(dog);
    p(either.get() + " " + either.equals(dog) + " " + any.or(dog).get());
    // an interface method, through the bridge
    MapSource ms = new MapSource();
    ms.m.put("k", "v");
    Source<String> src = ms;
    p(src.find("k") + " " + src.find("q") + " " + ms.find("k").get());
    // elements of collections
    List<Optional<String>> list = new ArrayList<>();
    list.add(a);
    list.add(none);
    list.add(Optional.of("b"));
    StringBuilder sb = new StringBuilder();
    for (Optional<String> e : list) sb.append(e.or("?"));
    Map<String, Optional<Animal>> byName = new LinkedHashMap<>();
    byName.put("d", Optional.of(new Animal("d")));
    byName.put("none", Optional.absent());
    p(sb + " " + list.get(2).get() + " " + byName + " " + byName.get("d").get().name);
    return "goptional done";
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
