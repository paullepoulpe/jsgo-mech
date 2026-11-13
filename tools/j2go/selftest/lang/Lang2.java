package lang;

import java.util.ArrayList;
import java.util.Collections;
import java.util.Comparator;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeMap;
import java.util.function.Function;
import java.util.function.Supplier;

/** More constructs: generics, nesting, initialization order, exceptions, references. */
public class Lang2 {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  // ---- static initialization order
  static class A {
    static final List<String> LOG = new ArrayList<>();
    static final int X = compute("A.X", 1);
    static int Y;

    static {
      Y = X + B.Z;
      LOG.add("A.static Y=" + Y);
    }

    static int compute(String name, int v) {
      LOG.add(name);
      return v;
    }
  }

  static class B {
    static final int Z = A.compute("B.Z", 10);
  }

  // ---- generics
  static class Box<T extends Comparable<T>> implements Comparable<Box<T>> {
    final T value;

    Box(T value) {
      this.value = value;
    }

    T get() {
      return value;
    }

    <R> R map(Function<? super T, ? extends R> f) {
      return f.apply(value);
    }

    @Override
    public int compareTo(Box<T> o) {
      return value.compareTo(o.value);
    }

    @Override
    public boolean equals(Object o) {
      return o instanceof Box<?> b && value.equals(b.value);
    }

    @Override
    public int hashCode() {
      return value.hashCode() * 31;
    }

    @Override
    public String toString() {
      return "Box(" + value + ")";
    }
  }

  static <T extends Comparable<T>> T maxOf(List<? extends T> xs) {
    T best = null;
    for (T x : xs) {
      if (best == null || x.compareTo(best) > 0) {
        best = x;
      }
    }
    return best;
  }

  static class Pair<K, V> {
    K k;
    V v;

    Pair(K k, V v) {
      this.k = k;
      this.v = v;
    }

    static <K, V> Pair<K, V> of(K k, V v) {
      return new Pair<>(k, v);
    }
  }

  // ---- interfaces
  interface Animal {
    String sound();

    default String speak() {
      return prefix() + sound();
    }

    private String prefix() {
      return "[" + kind() + "] ";
    }

    default String kind() {
      return "animal";
    }

    static Animal quiet() {
      return () -> "...";
    }
  }

  interface Pet extends Animal {
    @Override
    default String kind() {
      return "pet";
    }
  }

  static class Dog implements Pet {
    @Override
    public String sound() {
      return "woof";
    }
  }

  abstract static class Node implements Iterable<Node> {
    final List<Node> kids = new ArrayList<>();
    int id;
    static int next = 1;

    {
      id = next++;
    }

    Node add(Node n) {
      kids.add(n);
      return this;
    }

    abstract Node copy();

    @Override
    public Iterator<Node> iterator() {
      return kids.iterator();
    }

    int count() {
      int c = 1;
      for (Node k : this) {
        c += k.count();
      }
      return c;
    }
  }

  static class Leaf extends Node {
    String text;

    Leaf(String t) {
      text = t;
    }

    @Override
    Leaf copy() {
      return new Leaf(text + "'");
    }
  }

  static class Branch extends Node {
    @Override
    Branch copy() {
      Branch b = new Branch();
      for (Node k : kids) {
        b.add(k.copy());
      }
      return b;
    }
  }

  // ---- exceptions
  static class AppException extends Exception {
    final int code;

    AppException(String m, int code) {
      super(m);
      this.code = code;
    }
  }

  static class DeepException extends AppException {
    DeepException() {
      super("deep", 7);
    }

    @Override
    public String getMessage() {
      return "very " + super.getMessage();
    }
  }

  static class Res implements AutoCloseable {
    final String name;

    Res(String name) {
      this.name = name;
      p("open " + name);
    }

    @Override
    public void close() {
      p("close " + name);
    }
  }

  static int thrower(int k) throws AppException {
    if (k == 1) {
      throw new AppException("app", 3);
    }
    if (k == 2) {
      throw new DeepException();
    }
    if (k == 3) {
      throw new UnsupportedOperationException("unsup");
    }
    return k;
  }

  static String exceptions() {
    StringBuilder sb = new StringBuilder();
    for (int k = 0; k < 5; k++) {
      try (Res r = new Res("r" + k);
          Res r2 = new Res("s" + k)) {
        sb.append(thrower(k));
        if (k == 4) {
          int[] a = new int[1];
          a[k] = 1;
        }
      } catch (DeepException e) {
        sb.append("D").append(e.getMessage()).append(e.code);
      } catch (AppException e) {
        sb.append("A").append(e.getMessage()).append(e.code);
      } catch (UnsupportedOperationException | ArrayIndexOutOfBoundsException e) {
        sb.append(e instanceof UnsupportedOperationException ? "U" : "I");
      } finally {
        sb.append(";");
      }
    }
    try {
      try {
        throw new IllegalArgumentException("inner");
      } finally {
        sb.append("f1");
      }
    } catch (RuntimeException e) {
      sb.append(e.getMessage());
      try {
        throw new RuntimeException("wrapped", e);
      } catch (RuntimeException e2) {
        sb.append(e2.getCause().getMessage()).append(e2.getCause() == e);
      }
    }
    return sb.toString();
  }

  // ---- nesting
  int field = 5;

  class Outer1 {
    int f1 = field * 2;

    class Outer2 {
      int f2 = f1 + field;

      int sum() {
        return f2 + f1 + field + Lang2.this.field;
      }
    }
  }

  String nesting() {
    int local = 3;
    class Counter {
      int n = local;

      int inc() {
        return n += field;
      }
    }
    Counter c = new Counter();
    c.inc();
    Outer1 o1 = new Outer1();
    Outer1.Outer2 o2 = o1.new Outer2();
    Supplier<Supplier<Integer>> nested = () -> () -> local + field + c.inc();
    field = 6;
    return c.n + " " + o2.sum() + " " + nested.get().get() + " " + new Object() {
      @Override
      public String toString() {
        return "anon" + local + field;
      }
    };
  }

  static String collections() {
    Map<String, List<Integer>> m = new TreeMap<>((x, y) -> y.compareTo(x));
    for (int i = 0; i < 6; i++) {
      m.computeIfAbsent(i % 2 == 0 ? "even" : "odd", k -> new ArrayList<>()).add(i);
    }
    Set<Box<String>> boxes = new LinkedHashSet<>();
    boxes.add(new Box<>("b"));
    boxes.add(new Box<>("a"));
    boxes.add(new Box<>("b"));
    List<Box<String>> sorted = new ArrayList<>(boxes);
    Collections.sort(sorted);
    Map<Box<String>, Integer> byBox = new LinkedHashMap<>();
    byBox.put(new Box<>("k"), 1);
    Integer hit = byBox.get(new Box<>("k"));
    Integer miss = byBox.get(new Box<>("z"));
    Pair<String, Integer> pr = Pair.of("p", 9);
    int unboxed = pr.v + 1;
    long widened = pr.v;
    List<Integer> ints = java.util.Arrays.asList(3, 1, 2);
    StringBuilder sb = new StringBuilder();
    for (Map.Entry<String, List<Integer>> e : m.entrySet()) {
      sb.append(e.getKey()).append(e.getValue());
    }
    Iterator<Integer> it = m.get("odd").iterator();
    while (it.hasNext()) {
      if (it.next() == 3) {
        it.remove();
      }
    }
    return sb + " " + boxes + " " + sorted + " " + hit + " " + miss + " " + unboxed + " " + widened + " " + maxOf(ints) + " "
        + maxOf(sorted) + " " + new Box<>(41).map(x -> x + 1) + " " + m.get("odd") + " " + (hit == 1) + " " + sorted.contains(new Box<>("a"));
  }

  static String misc() {
    char c = 'x';
    String viaSwitch;
    switch (c) {
      case 'x':
      case 'y':
        viaSwitch = "xy";
        break;
      default:
        viaSwitch = "no";
    }
    int[][] grid = new int[2][3];
    grid[1][2] = 7;
    int[][] jag = {{1}, {2, 3}};
    Object[] objs = new String[] {"s"};
    long big = Integer.MAX_VALUE;
    big *= 2;
    int tern = big > 0 ? (int) (big % 1000) : -1;
    Integer i1 = 127, i2 = 127;
    Object cond = tern > 5 ? "str" : Integer.valueOf(1);
    String lbl = "";
    block:
    {
      if (tern > 0) {
        lbl = "broke";
        break block;
      }
      lbl = "fell";
    }
    double dd = 10 / 4;
    float ff = 10 / 4f;
    int idx = 0;
    int[] order = new int[3];
    order[idx++] = idx++;
    order[idx] = idx = 0;
    StringBuilder sb = new StringBuilder();
    sb.append('a' + 1).append((char) ('a' + 1)).append("" + 'a' + 1).append(1 + 2 + "s" + 1 + 2);
    return viaSwitch + grid[1][2] + grid.length + grid[0].length + jag[1][1] + objs[0] + big + " " + tern + i1.equals(i2) + cond + lbl + dd + " "
        + ff + " " + order[0] + order[1] + order[2] + idx + " " + sb + " " + (5 & 3 | 8 ^ 2) + " " + (-7 >> 1) + " " + (-7 >>> 29) + " " + (7 % -3)
        + " " + (long) 1e19 + " " + (int) Double.NaN + " " + (0.0 == -0.0) + " " + Double.compare(0.0, -0.0) + " " + Long.MAX_VALUE / -1;
  }

  public static String run() {
    p(A.Y + " " + A.LOG);
    Dog d = new Dog();
    p(d.speak() + " " + Animal.quiet().speak());
    Branch tree = new Branch();
    tree.add(new Leaf("a")).add(new Branch().add(new Leaf("b")));
    Node cp = tree.copy();
    p(tree.count() + " " + cp.count() + " " + cp.id + " " + ((Leaf) cp.kids.get(0)).text + " " + (cp instanceof Branch) + " " + cp.kids.get(1).kids.size());
    p(exceptions());
    p(new Lang2().nesting());
    p(collections());
    p(misc());
    return out.toString();
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
