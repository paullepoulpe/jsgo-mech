package lang;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSortedMap;
import com.google.common.collect.ImmutableSortedSet;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.Comparator;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;
import java.util.Set;
import java.util.TreeSet;

/**
 * Piece `gsorted` of rule "typed library interfaces" (j2go ExprEmitter.gsortedOf,
 * StmtEmitter.freshLoop; RESIDUE8 T2.2, T1.14, T2.19): Guava's sorted makers for a typed place are
 * typed: `ImmutableSortedSet.copyOf(c)`, `copyOfSorted(s)`, `of()`, `ImmutableSortedMap.copyOf(m)`,
 * `ImmutableList.sortedCopyOf(c)` and `sortedCopyOf(cmp, c)`, and a loop over
 * `ImmutableSortedMap.copyOf(m).values()`. Erased code that reads them (an own method of the
 * class, toString, the comparator) sees the ImmutableSortedSet / ImmutableSortedMap made from
 * them. selftest/run.sh counts the typed makers in g_GSorted.go. Order, duplicates and nulls as
 * Java's.
 */
public class GSorted {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  static class Ver implements Comparable<Ver> {
    final int major;
    final String tag;

    Ver(int major, String tag) {
      this.major = major;
      this.tag = tag;
    }

    @Override
    public int compareTo(Ver o) {
      return Integer.compare(major, o.major);
    }

    @Override
    public String toString() {
      return major + tag;
    }
  }

  static final Comparator<String> BY_LENGTH = Comparator.comparing(String::length);

  ImmutableSortedSet<String> keys;

  static ImmutableSortedSet<Ver> sorted(Collection<Ver> vs) {
    ImmutableSortedSet<Ver> s = ImmutableSortedSet.copyOf(vs);
    return s;
  }

  static ImmutableMap<String, Integer> byName(Map<String, Integer> m) {
    return ImmutableSortedMap.copyOf(m);
  }

  String typed() {
    // copyOf: natural ordering, the first of equal elements kept
    ImmutableSortedSet<Ver> vs = sorted(Arrays.asList(new Ver(3, "a"), new Ver(1, "b"), new Ver(3, "c"), new Ver(2, "d")));
    p(vs + " " + vs.contains(new Ver(2, "x")) + " " + vs.size() + " " + vs.asList());
    // copyOfSorted of a TreeSet with a comparator, and of an immutable sorted set (itself)
    TreeSet<String> ts = new TreeSet<>(BY_LENGTH);
    ts.add("ccc");
    ts.add("a");
    ts.add("bb");
    ts.add("d");
    keys = ImmutableSortedSet.copyOfSorted(ts);
    p(keys + " " + keys.contains("zz") + keys.contains("e"));
    ImmutableSortedSet<String> again = ImmutableSortedSet.copyOfSorted(keys);
    p(again + " " + again.size());
    ImmutableSortedSet<Ver> none = ImmutableSortedSet.of();
    p(none + " " + none.isEmpty());
    // ImmutableSortedMap.copyOf of a typed map
    Map<String, Integer> m = new LinkedHashMap<>();
    m.put("zeta", 1);
    m.put("alpha", 2);
    m.put("mid", 3);
    ImmutableMap<String, Integer> sm = byName(m);
    p(sm + " " + sm.get("mid") + " " + sm.keySet() + " " + sm.values());
    StringBuilder sb = new StringBuilder();
    for (Integer v : ImmutableSortedMap.copyOf(m).values()) {
      m.remove("mid");
      sb.append(v);
    }
    p(sb + " " + m);
    // sortedCopyOf, natural ordering and with a typed comparator
    List<String> names = new ArrayList<>(Arrays.asList("pear", "fig", "apple", "kiwi"));
    ImmutableList<String> nat = ImmutableList.sortedCopyOf(names);
    ImmutableList<String> bylen = ImmutableList.sortedCopyOf(BY_LENGTH, names);
    p(nat + " " + bylen);
    Set<String> ks = m.keySet();
    ImmutableList<String> bylen2 = ImmutableList.sortedCopyOf(BY_LENGTH, ks);
    p(bylen2);
    try {
      p(sorted(Arrays.asList(new Ver(1, "a"), null)));
    } catch (NullPointerException ex) {
      p("NPE");
    }
    return "typed done";
  }

  public static String run() {
    p(new GSorted().typed());
    String s = out.toString();
    out.setLength(0);
    return s;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
