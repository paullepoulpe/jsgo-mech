package lang;

import com.google.common.collect.ImmutableList;
import com.google.common.collect.ImmutableMap;
import com.google.common.collect.ImmutableSet;
import java.util.ArrayDeque;
import java.util.Deque;
import java.util.EnumMap;
import java.util.IdentityHashMap;
import java.util.LinkedHashSet;
import java.util.LinkedList;
import java.util.Map;
import java.util.Set;
import java.util.concurrent.ConcurrentHashMap;

/**
 * Piece `mkext` of rule "typed library interfaces" (j2go ExprEmitter.mkextNew, mkextFreshArg,
 * immutableOf; RESIDUE8 T2.3, T2.8, T3.2): more makers of typed collections for a typed place:
 * `ImmutableSet.copyOf(E[])`, `ImmutableMap.of` with 6 to 10 entries, `ImmutableMap.of(k, v)` of a
 * subclass's value for a place of `? extends V`, `new LinkedHashSet<>(ImmutableSet.of(..))` and
 * `new ArrayDeque<>(ImmutableList.of(..))`, `new EnumMap<>(K.class)`, `new IdentityHashMap<>()`,
 * `new ConcurrentHashMap<>()`, `new LinkedList<>()` for a Deque. selftest/run.sh counts the typed
 * makers in g_MkExt.go. Order, identity, duplicates and nulls behave as Java's.
 */
public class MkExt {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  enum Color {
    RED,
    GREEN,
    BLUE
  }

  static class Base {
    final String n;

    Base(String n) {
      this.n = n;
    }

    @Override
    public String toString() {
      return "B:" + n;
    }
  }

  static class Sub extends Base {
    Sub(String n) {
      super(n);
    }
  }

  /** Equal by name: an IdentityHashMap keeps two of them. */
  static class Eq {
    final String n;

    Eq(String n) {
      this.n = n;
    }

    @Override
    public boolean equals(Object o) {
      return o instanceof Eq && ((Eq) o).n.equals(n);
    }

    @Override
    public int hashCode() {
      return n.hashCode();
    }

    @Override
    public String toString() {
      return "E:" + n;
    }
  }

  static final ImmutableMap<Integer, String> YEARS =
      ImmutableMap.of(2012, "a", 2013, "b", 2014, "c", 2015, "d", 2016, "e", 2017, "f", 2018, "g", 2019, "h", 2020, "i", 2021, "j");

  static final ImmutableMap<String, Color> SIX = ImmutableMap.of("r", Color.RED, "g", Color.GREEN, "b", Color.BLUE, "R", Color.RED, "G", Color.GREEN, "B", Color.BLUE);

  final Deque<Base> stack = new LinkedList<>();
  final Set<String> forbidden = new LinkedHashSet<>(ImmutableSet.of("x", "y", "x"));
  final ArrayDeque<Base> work = new ArrayDeque<>(ImmutableList.of(new Base("w1"), new Sub("w2")));
  EnumMap<Color, String> byColor;
  Map<Color, Integer> counts;
  Map<Eq, Integer> seen;
  Map<String, Integer> conc;

  static String names(Map<String, ? extends Base> m) {
    return m.toString() + m.get("then");
  }

  static int distinct(Base[] arr) {
    Set<Base> s = ImmutableSet.copyOf(arr);
    return s.size();
  }

  String typed() {
    p(YEARS + " " + YEARS.get(2015) + " " + YEARS.size() + " " + SIX.get("G"));
    Sub sub = new Sub("s");
    p(names(ImmutableMap.of("then", sub)));
    Base a = new Base("a");
    p(distinct(new Base[] {a, new Base("b"), a}));
    Set<String> fromArr = ImmutableSet.copyOf(new String[] {"q", "p", "q"});
    p(fromArr + " " + fromArr.contains("p"));
    p(forbidden + " " + forbidden.add("z") + forbidden);
    p(work.size() + " " + work.peek() + " " + work.poll() + work.size());
    stack.push(a);
    stack.push(sub);
    stack.push(null);
    p(stack.size() + " " + stack.pop() + " " + stack.peek());
    StringBuilder sb = new StringBuilder();
    for (Base b : stack) {
      sb.append(b);
    }
    p(sb);
    byColor = new EnumMap<>(Color.class);
    byColor.put(Color.BLUE, "b");
    byColor.put(Color.RED, "r");
    byColor.put(Color.GREEN, null);
    p(byColor + " " + byColor.get(Color.RED) + " " + byColor.containsKey(Color.GREEN) + byColor.remove(Color.BLUE) + " " + byColor.keySet());
    counts = new EnumMap<>(Color.class);
    counts.merge(Color.GREEN, 1, Integer::sum);
    counts.merge(Color.RED, 2, Integer::sum);
    counts.merge(Color.GREEN, 3, Integer::sum);
    p(counts);
    seen = new IdentityHashMap<>();
    Eq e1 = new Eq("a"), e2 = new Eq("a");
    seen.put(e1, 1);
    seen.put(e2, 2);
    seen.put(e1, 3);
    p(seen.size() + " " + seen.get(e1) + " " + seen.get(e2) + " " + seen.get(new Eq("a")) + " " + seen.containsKey(e2));
    conc = new ConcurrentHashMap<>();
    conc.put("k", 1);
    conc.computeIfAbsent("j", k -> 2);
    p(conc.get("k") + " " + conc.get("j") + " " + conc.size());
    try {
      Map<String, Integer> dup = ImmutableMap.of("a", 1, "b", 2, "c", 3, "d", 4, "e", 5, "a", 6);
      p(dup);
    } catch (IllegalArgumentException ex) {
      p("IAE " + ex.getMessage());
    }
    try {
      String none = null;
      Set<String> withNull = ImmutableSet.copyOf(new String[] {"a", none});
      p(withNull);
    } catch (NullPointerException ex) {
      p("NPE");
    }
    return "typed done";
  }

  public static String run() {
    p(new MkExt().typed());
    String s = out.toString();
    out.setLength(0);
    return s;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
