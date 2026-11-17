package lang;

import java.util.ArrayList;
import java.util.Comparator;
import java.util.Iterator;
import java.util.List;
import java.util.function.Function;

/**
 * Typed generics, part 2: class hierarchies of generic classes (a struct embedding its
 * superclass at its view of it, dispatch through self_), local and anonymous classes of generic
 * scopes, generic instance methods (their type variables erased to their bounds), arrays of a
 * type parameter, and instanceof of a generic class.
 */
public class Generics2 {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  // ---- a generic hierarchy: an abstract base, a concrete generic subclass, a plain subclass
  abstract static class Base<T> {
    T v;
    int n;

    Base(T v) {
      this.v = v;
    }

    abstract T make(String s);

    T get() {
      return v;
    }

    void set(T t) {
      v = t;
      n++;
    }

    String show() {
      return "Base(" + get() + "," + n + ")";
    }

    <R> R apply(Function<T, R> f) {
      return f.apply(v);
    }

    <X extends Base<T>> X same(X x) {
      return x.get() == v ? x : null;
    }
  }

  static class Strs extends Base<String> {
    Strs(String s) {
      super(s);
    }

    @Override
    String make(String s) {
      return s + "!";
    }

    @Override
    String show() {
      return "Strs:" + super.show() + ":" + v.length();
    }
  }

  static class Lists<U> extends Base<List<U>> {
    final U first;

    Lists(U first) {
      super(new ArrayList<>());
      this.first = first;
      v.add(first);
    }

    @Override
    List<U> make(String s) {
      List<U> l = new ArrayList<>(v);
      l.add(first);
      return l;
    }

    @Override
    void set(List<U> t) {
      super.set(t);
      n += 10;
    }
  }

  static final class Named<U> extends Lists<U> {
    final String name;

    Named(String name, U first) {
      super(first);
      this.name = name;
    }

    @Override
    String show() {
      return name + "=" + super.show();
    }
  }

  static <T> String describe(Base<T> b, String s) {
    T made = b.make(s);
    b.set(made);
    return b.show() + " " + made + " " + (b.get() == made);
  }

  // ---- anonymous and local classes of a generic scope
  static final class Bag<T> implements Iterable<T> {
    final List<T> items = new ArrayList<>();

    Bag<T> add(T t) {
      items.add(t);
      return this;
    }

    @Override
    public Iterator<T> iterator() {
      return new Iterator<T>() {
        int i = items.size();

        @Override
        public boolean hasNext() {
          return i > 0;
        }

        @Override
        public T next() {
          i--;
          return items.get(i);
        }
      };
    }

    Comparator<T> byString() {
      return new Comparator<T>() {
        @Override
        public int compare(T a, T b) {
          return String.valueOf(a).compareTo(String.valueOf(b));
        }
      };
    }

    static <E> Bag<E> of(E a, E b) {
      class Pairing {
        final E x;
        final E y;

        Pairing(E x, E y) {
          this.x = x;
          this.y = y;
        }

        Bag<E> bag() {
          return new Bag<E>().add(x).add(y);
        }
      }
      return new Pairing(a, b).bag();
    }

    @SuppressWarnings("unchecked")
    String arrayInfo(int extra) {
      T[] arr = (T[]) new Object[items.size() + extra];
      for (int i = 0; i < items.size(); i++) arr[i] = items.get(i);
      T last = arr[arr.length - 1];
      return arr.length + " " + arr[1] + " " + (last == null) + " " + (arr[1] == null) + " " + arr[0];
    }
  }

  // ---- a type parameter bounded by an interface of the program: a Go constraint
  interface Lat {
    Lat join(Lat o);

    String show();
  }

  static final class Max implements Lat {
    final int v;

    Max(int v) {
      this.v = v;
    }

    @Override
    public Lat join(Lat o) {
      return o == null || ((Max) o).v < v ? this : o;
    }

    @Override
    public String show() {
      return "max" + v;
    }
  }

  static final class Flow<N, L extends Lat> {
    L state;
    final List<N> seen = new ArrayList<>();

    @SuppressWarnings("unchecked")
    L step(N n, L x) {
      seen.add(n);
      state = state == null ? x : (L) x.join(state);
      return state;
    }

    String show() {
      return seen + ":" + (state == null ? "none" : state.show());
    }
  }

  // ---- a result type used with the type variable of a generic method (a Go generic function)
  //      and with a class type parameter that every use instantiates with its bound
  static final class Result<T> {
    final T value;
    final String error;

    private Result(T value, String error) {
      this.value = value;
      this.error = error;
    }

    static <T> Result<T> ok(T v) {
      return new Result<>(v, null);
    }

    static <T> Result<T> err(String e) {
      return new Result<>(null, e);
    }

    @Override
    public String toString() {
      return error != null ? "err(" + error + ")" : "ok(" + value + ")";
    }
  }

  enum Color {
    RED,
    GREEN
  }

  static class Opts {
    String name = "opts";

    @Override
    public String toString() {
      return "Opts(" + name + ")";
    }
  }

  abstract static class Runner<O extends Opts> {
    abstract Result<O> make();

    <E extends Enum<E>> Result<E> pick(E[] values, String name) {
      for (E e : values) if (e.name().equalsIgnoreCase(name)) return Result.ok(e);
      return Result.err("no " + name + " among " + values.length);
    }

    String run(String c) {
      Result<O> o = make();
      Result<Color> col = pick(Color.values(), c);
      return o + " " + o.value.name + " " + col + " " + (col.value == null ? -1 : col.value.ordinal());
    }
  }

  static final class MyRunner extends Runner<Opts> {
    @Override
    Result<Opts> make() {
      Opts o = new Opts();
      o.name = "mine";
      return Result.ok(o);
    }
  }

  public static String run() {
    Strs s = new Strs("a");
    Lists<Integer> li = new Lists<>(7);
    Named<String> nm = new Named<>("nm", "z");
    p(describe(s, "b") + " | " + describe(li, "x") + " | " + describe(nm, "y"));
    Base<String> bs = s;
    Object o = nm;
    p(bs.show() + " " + (o instanceof Base) + " " + (o instanceof Lists) + " " + (o instanceof Strs) + " " + ((Lists<String>) o).first
        + " " + ((Base<List<String>>) o).get() + " " + bs.apply(x -> x + "?") + " " + li.apply(List::size));
    p((s.same(s) == s) + " " + (s.same(new Strs("q")) == null) + " " + new Strs(null).get());
    Bag<String> bag = Bag.of("q", null).add("p");
    StringBuilder sb = new StringBuilder();
    for (String x : bag) sb.append(x).append(',');
    List<String> sorted = new ArrayList<>();
    sorted.add("c");
    sorted.add("a");
    sorted.add("b");
    sorted.sort(bag.byString());
    p(sb + " " + sorted + " " + bag.arrayInfo(1));
    Flow<String, Max> f = new Flow<>();
    f.step("a", new Max(3));
    f.step("b", new Max(1));
    Flow<Integer, Lat> g = new Flow<>();
    p(f.show() + " " + f.step("c", new Max(5)).v + " " + g.show() + " " + g.step(1, new Max(2)).show());
    MyRunner r = new MyRunner();
    p(r.run("green") + " | " + r.run("blue"));
    return out.toString();
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
