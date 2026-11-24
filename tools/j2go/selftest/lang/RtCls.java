package lang;

import com.google.common.cache.CacheBuilder;
import com.google.common.cache.CacheLoader;
import com.google.common.cache.LoadingCache;
import com.google.common.collect.AbstractIterator;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.Ordering;
import com.google.common.collect.Range;
import com.google.common.collect.RangeSet;
import com.google.common.collect.TreeRangeSet;
import java.lang.annotation.ElementType;
import java.lang.annotation.Target;
import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.Iterator;
import java.util.List;
import java.util.Objects;
import java.util.TreeSet;
import java.util.function.Function;
import java.util.function.Supplier;

/**
 * Pieces `rtcls`, `annqual`, `ownwild2` (RESIDUE8 rules "typed runtime classes" and "Source.Lazy<T>
 * typed"): typed runtime classes (ThreadLocal, AbstractIterator, CacheLoader and the cache it
 * makes, Range / TreeRangeSet / RangeSet, Ordering), a program generic class written as
 * `Outer.@N Lazy<String>` (a type annotation on a qualified type) whose equals is the
 * expression form of a listed equals, and two classes of that form that stay erased.
 */
public class RtCls {
  @Target(ElementType.TYPE_USE)
  @interface N {}

  static final StringBuilder out = new StringBuilder();

  static void p(Object o) {
    out.append(o).append('\n');
  }

  // ---- ThreadLocal
  static final class Trace {
    final String name;

    Trace(String name) {
      this.name = name;
    }
  }

  private static final ThreadLocal<Trace> traces = new ThreadLocal<>();

  static String threadLocal() {
    StringBuilder sb = new StringBuilder();
    sb.append(traces.get() == null);
    traces.set(new Trace("t1"));
    sb.append(' ').append(traces.get().name);
    traces.remove();
    sb.append(' ').append(traces.get() == null);
    return sb.toString();
  }

  // ---- AbstractIterator
  static final class Countdown extends AbstractIterator<String> {
    int n;

    Countdown(int n) {
      this.n = n;
    }

    @Override
    protected String computeNext() {
      if (n == 0) {
        return endOfData();
      }
      n--;
      return "c" + n;
    }
  }

  static String iterators() {
    StringBuilder sb = new StringBuilder();
    Countdown c = new Countdown(3);
    sb.append(c.hasNext()).append(' ').append(c.peek());
    while (c.hasNext()) {
      sb.append(' ').append(c.next());
    }
    // erased code reads one (an Iterator<?> of an Object)
    Object o = new Countdown(2);
    Iterator<?> w = (Iterator<?>) o;
    while (w.hasNext()) {
      sb.append(' ').append(w.next());
    }
    Iterator<Integer> nums =
        new AbstractIterator<Integer>() {
          int i;

          @Override
          protected Integer computeNext() {
            if (i >= 3) {
              return endOfData();
            }
            i += 1;
            return Integer.valueOf((i - 1) * 10);
          }
        };
    while (nums.hasNext()) {
      sb.append(' ').append(nums.next() + 1);
    }
    try {
      c.next();
    } catch (java.util.NoSuchElementException e) {
      sb.append(" NoSuchElementException");
    }
    return sb.toString();
  }

  // ---- CacheLoader / LoadingCache
  static final class Key {
    final String path;

    Key(String path) {
      this.path = path;
    }

    @Override
    public boolean equals(Object o) {
      return o instanceof Key k && k.path.equals(path);
    }

    @Override
    public int hashCode() {
      return path.hashCode();
    }
  }

  static int loads;

  static String caches() {
    StringBuilder sb = new StringBuilder();
    LoadingCache<Key, String> cache =
        CacheBuilder.newBuilder()
            .maximumSize(2)
            .build(
                new CacheLoader<Key, String>() {
                  @Override
                  public String load(Key key) {
                    loads++;
                    if (key.path.equals("bad")) {
                      throw new IllegalStateException("bad key");
                    }
                    return key.path.toUpperCase();
                  }
                });
    sb.append(cache.getUnchecked(new Key("a"))).append(' ').append(cache.getUnchecked(new Key("a")));
    sb.append(' ').append(cache.getUnchecked(new Key("b"))).append(' ').append(cache.getUnchecked(new Key("c")));
    sb.append(' ').append(cache.getUnchecked(new Key("a"))).append(' ').append(loads);
    try {
      cache.getUnchecked(new Key("bad"));
    } catch (RuntimeException e) {
      sb.append(' ').append(e.getCause().getMessage());
    }
    LoadingCache<String, Integer> lens = CacheBuilder.from("maximumSize=10").build(CacheLoader.from(String::length));
    sb.append(' ').append(lens.getUnchecked("abc")).append(' ').append(lens.getUnchecked("abc") + lens.getUnchecked("de"));
    return sb.toString();
  }

  // ---- Range / TreeRangeSet / RangeSet
  record Pos(int line, int col) implements Comparable<Pos> {
    @Override
    public int compareTo(Pos o) {
      return line != o.line ? Integer.compare(line, o.line) : Integer.compare(col, o.col);
    }
  }

  private final RangeSet<Pos> ranges = TreeRangeSet.create();

  static String rangeSets() {
    RtCls r = new RtCls();
    StringBuilder sb = new StringBuilder();
    sb.append(r.ranges.isEmpty());
    r.ranges.add(Range.closed(new Pos(1, 0), new Pos(2, 5)));
    r.ranges.add(Range.closed(new Pos(4, 0), new Pos(4, 9)));
    r.ranges.add(Range.closed(new Pos(2, 3), new Pos(3, 0)));
    sb.append(' ').append(r.ranges.isEmpty());
    sb.append(' ').append(r.ranges.contains(new Pos(2, 9))).append(' ').append(r.ranges.contains(new Pos(3, 1)));
    sb.append(' ').append(r.ranges.contains(new Pos(4, 9))).append(' ').append(r.ranges.contains(new Pos(0, 0)));
    RangeSet<String> words = TreeRangeSet.create();
    words.add(Range.closed("b", "d"));
    sb.append(' ').append(words.contains("c")).append(' ').append(words.contains("e"));
    try {
      Range.closed(new Pos(5, 0), new Pos(1, 0));
    } catch (IllegalArgumentException e) {
      sb.append(" IllegalArgumentException");
    }
    return sb.toString();
  }

  // ---- Ordering
  private final Ordering<String> names = Ordering.natural().nullsFirst();
  private final Ordering<Pos> byLine =
      new Ordering<Pos>() {
        @Override
        public int compare(Pos a, Pos b) {
          int r = names.compare(a == null ? null : "" + a.line, b == null ? null : "" + b.line);
          return r != 0 ? r : a.col - b.col;
        }
      };

  static final class ByLength extends Ordering<String> {
    @Override
    public int compare(String a, String b) {
      return a.length() - b.length();
    }
  }

  static String orderings() {
    RtCls r = new RtCls();
    StringBuilder sb = new StringBuilder();
    sb.append(r.names.compare(null, "a")).append(' ').append(r.names.compare("b", "a")).append(' ').append(r.names.compare(null, null));
    sb.append(' ').append(r.byLine.compare(new Pos(1, 2), new Pos(1, 1)));
    sb.append(' ').append(r.byLine.nullsFirst().compare(null, new Pos(1, 1)));
    List<String> ws = new ArrayList<>(ImmutableList.of("ccc", "a", "bb", "dd"));
    Collections.sort(ws, new ByLength().reverse());
    sb.append(' ').append(ws);
    sb.append(' ').append(new ByLength().sortedCopy(ImmutableList.of("xyz", "", "pq")));
    sb.append(' ').append(Ordering.natural().sortedCopy(ImmutableList.of("q", "c", "m")));
    TreeSet<Pos> sorted = new TreeSet<>(r.byLine);
    sorted.add(new Pos(2, 1));
    sorted.add(new Pos(1, 7));
    sorted.add(new Pos(1, 3));
    sb.append(' ').append(sorted.iterator().next().col);
    return sb.toString();
  }

  // ---- annqual / ownwild2: a program generic class at one instantiation, written with a type annotation on its qualified name
  abstract static class Lazy<T> implements Supplier<T> {
    @Override
    public boolean equals(Object other) {
      return other instanceof Lazy<?> && Objects.equals(get(), ((Lazy<?>) other).get());
    }

    @Override
    public int hashCode() {
      return Objects.hashCode(get());
    }

    static <T> Lazy<T> of(T v) {
      return new Lazy<T>() {
        @Override
        public T get() {
          return v;
        }
      };
    }
  }

  private RtCls.@N Lazy<String> lazy = Lazy.of("x");

  /** Left: the expression form, listed, but written at two instantiations (String and Integer). */
  abstract static class Two<T> implements Supplier<T> {
    @Override
    public boolean equals(Object other) {
      return other instanceof Two<?> && Objects.equals(get(), ((Two<?>) other).get());
    }

    @Override
    public int hashCode() {
      return 1;
    }

    static <T> Two<T> of(T v) {
      return new Two<T>() {
        @Override
        public T get() {
          return v;
        }
      };
    }
  }

  /** Left: the expression form, not listed. */
  abstract static class Unl<T> implements Supplier<T> {
    @Override
    public boolean equals(Object other) {
      return other instanceof Unl<?> && Objects.equals(get(), ((Unl<?>) other).get());
    }

    @Override
    public int hashCode() {
      return 2;
    }

    static <T> Unl<T> of(T v) {
      return new Unl<T>() {
        @Override
        public T get() {
          return v;
        }
      };
    }
  }

  static String lazies() {
    RtCls r = new RtCls();
    Two<String> ts = Two.of("1");
    Two<Integer> ti = Two.of(1);
    Unl<String> u = Unl.of("u");
    return r.lazy.get() + " " + r.lazy.equals(Lazy.of("x")) + " " + r.lazy.equals(Lazy.of("y")) + " " + r.lazy.equals("x")
        + " " + (r.lazy.hashCode() == Objects.hashCode("x")) + " " + ts.equals(ti) + " " + ts.equals(Two.of("1")) + " " + u.equals(Unl.of("u"));
  }

  // ---- nboxarg: boxed type arguments of function types at Nbox
  static int dots(String s) {
    return s.length() - s.replace(".", "").length();
  }

  static Integer nullable(String s) {
    return s.isEmpty() ? null : s.length();
  }

  static double weight(String s) {
    return s.length() / 2.0;
  }

  /** Typed: an Integer key (Comparator_comparingN_). */
  static final Comparator<String> SHORT_TO_LONG = Comparator.comparing(RtCls::dots);
  /** Left: a Double key (not integral). */
  static final Comparator<String> BY_WEIGHT = Comparator.comparing(RtCls::weight);

  /** Typed: a constructor's function parameter kept in a field, given a method reference and a lambda. */
  static final class Pool {
    private final Function<String, Integer> indexFn;

    Pool(Function<String, Integer> indexFn) {
      this.indexFn = indexFn;
    }

    int index(String s) {
      return indexFn.apply(s);
    }

    String look(String s) {
      Integer i = indexFn.apply(s);
      return i == null ? "null" : "i" + i;
    }
  }

  /** Left: the function is given a variable that is no candidate. */
  static final class Other {
    private final Function<String, Integer> fn;

    Other(Function<String, Integer> fn) {
      this.fn = fn;
    }

    int get(String s) {
      return fn.apply(s);
    }
  }

  static String nboxArgs() {
    List<String> names = new ArrayList<>(ImmutableList.of("a.b.c", "x", "p.q", "m.n"));
    names.sort(SHORT_TO_LONG);
    List<String> ws = new ArrayList<>(ImmutableList.of("ccc", "a", "bb"));
    ws.sort(BY_WEIGHT);
    Pool p = new Pool(RtCls::dots);
    Pool q = new Pool(s -> nullable(s));
    Function<String, Integer> f = String::length;
    Other o = new Other(f);
    String npe;
    try {
      Comparator.comparing(RtCls::nullable).compare("", "a");
      npe = "no";
    } catch (NullPointerException e) {
      npe = "NullPointerException";
    }
    return names + " " + ws + " " + p.index("a.b") + " " + q.look("") + " " + q.look("abc") + " " + o.get("four") + " "
        + Comparator.comparing(RtCls::nullable).compare("ab", "a") + " " + npe;
  }

  public static String run() {
    p(threadLocal());
    p(iterators());
    p(caches());
    p(rangeSets());
    p(orderings());
    p(lazies());
    p(nboxArgs());
    String s = out.toString();
    out.setLength(0);
    return s;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
