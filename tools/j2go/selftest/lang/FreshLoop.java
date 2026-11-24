package lang;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.Lists;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Set;

/**
 * Piece `freshloop` of rule "typed library interfaces" (j2go StmtEmitter.freshLoop; RESIDUE8
 * T2.1, T1.8): a for-each loop over `ImmutableList.of(..)`, `copyOf(..)` or `sortedCopyOf(..)`
 * makes the typed list at the element type and runs on its cursor; a loop over
 * `Lists.reverse(l)` of a typed ImmutableList runs backwards on a typed cursor. typed() has 10
 * such loops; erased() 2 that stay erased (Lists.reverse of a list that can change; sortedCopyOf
 * with a comparator). selftest/run.sh counts both. The copies are made before the loops change
 * their sources; null elements throw NullPointerException as Java's do.
 */
public class FreshLoop {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  static class Item implements Comparable<Item> {
    final String n;

    Item(String n) {
      this.n = n;
    }

    @Override
    public int compareTo(Item o) {
      return n.compareTo(o.n);
    }

    @Override
    public String toString() {
      return "I:" + n;
    }
  }

  static <X> Collection<Item> coll(Collection<Item> c) {
    return c;
  }

  static ImmutableList<String> prefixes(String path) {
    ImmutableList.Builder<String> b = ImmutableList.builder();
    int i = path.indexOf('.');
    while (i >= 0) {
      b.add(path.substring(0, i));
      i = path.indexOf('.', i + 1);
    }
    b.add(path);
    return b.build();
  }

  static String typed() {
    Item a = new Item("a"), b = new Item("b"), c = new Item("c");
    StringBuilder sb = new StringBuilder();
    // of(..): the elements as they are, no box each
    for (Item i : ImmutableList.of(c, a)) {
      sb.append(i).append(' ');
    }
    for (String s : ImmutableList.of("x", "y", "z")) {
      sb.append(s);
    }
    p(sb);
    // copyOf of a typed list that the loop changes: the copy is read
    List<Item> items = new ArrayList<>(Arrays.asList(a, b));
    for (Item i : ImmutableList.copyOf(items)) {
      items.add(new Item(i.n + "2"));
    }
    p(items);
    // copyOf of a typed map's values, of a typed set, of a typed interface, of an erased list, of an array
    Map<String, Item> byName = new LinkedHashMap<>();
    byName.put("b", b);
    byName.put("c", c);
    sb.setLength(0);
    for (Item i : ImmutableList.copyOf(byName.values())) {
      byName.remove(i.n);
      sb.append(i);
    }
    p(sb + " " + byName);
    Set<String> names = new LinkedHashSet<>(Arrays.asList("q", "p"));
    sb.setLength(0);
    for (String s : ImmutableList.copyOf(names)) {
      names.add(s + s);
      sb.append(s);
    }
    p(sb + " " + names);
    sb.setLength(0);
    for (Item i : ImmutableList.copyOf(coll(new ArrayList<>(Arrays.asList(c, b))))) {
      sb.append(i);
    }
    for (Item i : ImmutableList.copyOf(Arrays.asList(a, c))) {
      sb.append(i);
    }
    for (Item i : ImmutableList.copyOf(new Item[] {b, a})) {
      sb.append(i);
    }
    p(sb);
    // sortedCopyOf (natural ordering) of a typed set and of a typed list of a Comparable class
    sb.setLength(0);
    for (String s : ImmutableList.sortedCopyOf(names)) {
      sb.append(s).append(',');
    }
    for (Item i : ImmutableList.sortedCopyOf(Arrays.asList(c, a, b, a))) {
      sb.append(i);
    }
    p(sb);
    // Lists.reverse of an ImmutableList
    sb.setLength(0);
    for (String s : Lists.reverse(prefixes("a.b.c"))) {
      sb.append(s).append(';');
    }
    p(sb);
    // a null element: NullPointerException from the copy
    List<Item> withNull = new ArrayList<>(Arrays.asList(a, null));
    try {
      for (Item i : ImmutableList.copyOf(withNull)) {
        p("read " + i);
      }
    } catch (NullPointerException e) {
      p("NPE");
    }
    return "typed done";
  }

  static String erased() {
    List<String> l = new ArrayList<>(Arrays.asList("1", "2", "3"));
    StringBuilder sb = new StringBuilder();
    for (String s : Lists.reverse(l)) {
      sb.append(s);
    }
    for (String s : ImmutableList.sortedCopyOf((x, y) -> y.compareTo(x), l)) {
      sb.append(s);
    }
    return sb.toString();
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
