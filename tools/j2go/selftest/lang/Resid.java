package lang;

import com.google.common.base.Ascii;
import com.google.common.base.CharMatcher;
import com.google.common.base.Splitter;
import com.google.common.collect.ComparisonChain;
import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.Lists;
import java.math.BigInteger;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.BitSet;
import java.util.Collections;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;
import java.util.concurrent.ConcurrentHashMap;

/**
 * The rules of docs/javaisms6/RESIDUE8.md that pieces `rtm`, `fns2` and `seqstr` build (j2go.LibFns,
 * j2go.Conv.STR_SEQ2): typed() and the methods it calls have calls of the typed twins and the
 * String variants; left() has calls that keep the erased method (the reasons in LibFns).
 * tools/j2go/selftest/run.sh counts both in the translation. Both must print what Java prints.
 */
public class Resid {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  enum Level {
    LOW,
    MID,
    HIGH
  }

  static final class Item {
    final String name;
    final int n;

    Item(String name, int n) {
      this.name = name;
      this.n = n;
    }

    String name() {
      return name;
    }

    int n() {
      return n;
    }

    @Override
    public String toString() {
      return name + n;
    }
  }

  static final class Pair implements Comparable<Pair> {
    final String a;
    final String b;

    Pair(String a, String b) {
      this.a = a;
      this.b = b;
    }

    @Override
    public int compareTo(Pair o) {
      return ComparisonChain.start().compare(a, o.a).compare(b, o.b).result();
    }
  }

  static final Splitter DOT = Splitter.on('.');
  static final CharMatcher SLASH = CharMatcher.is('/');
  static final Comparator<Item> BY_NAME = Comparator.comparing(Item::name);
  static final Comparator<Item> BY_N_NAME = Comparator.comparingInt(Item::n).thenComparing(Item::name);

  final List<String> names = new ArrayList<>();
  final Map<String, Long> counts = new ConcurrentHashMap<>();
  final Map<String, Item> items = new ConcurrentHashMap<>();
  final ImmutableList.Builder<Item> built = ImmutableList.builder();
  final TreeSet<String> sorted = new TreeSet<>();

  static ImmutableList<String> letters() {
    return ImmutableList.of("a", "b", "c", "d");
  }

  void insert(String s) {
    names.add(s);
    names.add(0, s + "0");
  }

  static String rtm() {
    BigInteger x = BigInteger.valueOf(5);
    BigInteger y = BigInteger.TEN;
    String r = x.compareTo(y) + " " + y.compareTo(x) + " " + x.compareTo(BigInteger.valueOf(5));
    r += " " + new Pair("a", "b").compareTo(new Pair("a", "c")) + new Pair("b", "a").compareTo(new Pair("a", "z")) + new Pair("q", "q").compareTo(new Pair("q", "q"));
    Level lo = Level.LOW;
    Level hi = Level.HIGH;
    r += " " + lo.compareTo(hi) + " " + hi.compareTo(lo) + " " + lo.compareTo(Level.LOW);
    BitSet bs = new BitSet();
    bs.set(3);
    BitSet c = (BitSet) bs.clone();
    c.set(5);
    r += " " + bs.get(3) + bs.get(5) + c.get(3) + c.get(5) + c.cardinality();
    ImmutableList<String> il = letters();
    ImmutableList<String> sub = il.subList(1, 3);
    r += " " + sub + il.reverse() + (il.subList(0, 4) == il) + il.subList(2, 2).isEmpty() + letters().subList(3, 4);
    ImmutableList.Builder<String> b = ImmutableList.builder();
    b.add("p").add("q");
    r += b.build().reverse();
    Resid o = new Resid();
    o.sorted.addAll(Arrays.asList("c", "a", "b", "d"));
    Set<String> tail = o.sorted.tailSet("b", false);
    Set<String> tail2 = o.sorted.tailSet("b", true);
    r += " " + tail + tail2;
    ImmutableMap<String, String> im = ImmutableMap.of("k", "v");
    r += " " + im.containsValue("v") + im.containsValue("k");
    o.insert("i");
    o.insert("j");
    r += " " + o.names;
    return r;
  }

  static String fns2() {
    Item[] arr = new Item[3];
    Arrays.setAll(arr, i -> new Item("i", i));
    String r = "";
    for (Item it : arr) {
      r += it + ",";
    }
    List<Item> its = new ArrayList<>(Arrays.asList(new Item("b", 2), new Item("a", 2), new Item("c", 1)));
    its.sort(BY_NAME);
    r += " " + its;
    its.sort(BY_N_NAME);
    r += " " + its;
    Set<Item> byName = new TreeSet<>(Comparator.comparing(Item::name));
    byName.addAll(its);
    r += " " + byName;
    Resid o = new Resid();
    o.built.addAll(Lists.transform(ImmutableList.copyOf(Arrays.asList("x", "y")), s -> new Item(s, 7)));
    r += " " + o.built.build();
    for (String part : DOT.split("a.b..c")) {
      r += "<" + part + ">";
    }
    o.counts.putIfAbsent("k", 1L);
    o.counts.putIfAbsent("k", 2L);
    Long old = o.counts.get("k");
    boolean rep = o.counts.replace("k", old, old + 5);
    boolean rep2 = o.counts.replace("k", old, old + 9);
    r += " " + o.counts + rep + rep2;
    Item prev = o.items.putIfAbsent("a", new Item("a", 1));
    Item prev2 = o.items.putIfAbsent("a", new Item("a", 2));
    r += " " + prev + prev2;
    List<String> one = Collections.singletonList("only");
    r += " " + one + one.size();
    return r;
  }

  static String seqstr() {
    String r = Ascii.equalsIgnoreCase("0X", "0x") + " " + Ascii.equalsIgnoreCase("ab", "ac");
    r += " " + Integer.parseInt("x1Fz", 1, 3, 16);
    r += " " + SLASH.lastIndexIn("a/b/c") + SLASH.collapseFrom("a//b///c", '/');
    return r;
  }

  static String left() {
    // a key that is no String: the erased comparing
    List<Item> its = new ArrayList<>(Arrays.asList(new Item("b", 2), new Item("a", 1)));
    its.sort(Comparator.comparing(Item::n));
    // Lists.transform that is not read once: the live view
    List<Integer> lens = Lists.transform(Arrays.asList("aa", "b"), String::length);
    // containsValue of another Go type than the values' (an int for Integer values)
    Map<String, Integer> m = new LinkedHashMap<>();
    m.put("a", 1);
    return its + " " + lens + " " + m.containsValue(1);
  }

  public static String run() {
    p(rtm());
    p(fns2());
    p(seqstr());
    p(left());
    String s = out.toString();
    out.setLength(0);
    return s;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
