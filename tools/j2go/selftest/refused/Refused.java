package lang;

import java.util.HashMap;
import java.util.LinkedHashSet;
import java.util.Map;
import java.util.Set;
import java.util.function.Function;
import java.util.function.Predicate;
import java.util.function.Supplier;

/** Every use here must be refused by rule "function types". */
public class Refused {
  static class Keyed<K, V> {
    final Map<K, V> m = new HashMap<>();
  }

  static Object asObject(Object o) {
    return o;
  }

  static String run() {
    Supplier<String> a = () -> "a", b = () -> "b";
    boolean same = a == b;                                  // identity
    Object o = a;                                           // to Object (initializer)
    boolean isSup = o instanceof Supplier;                  // instanceof
    Supplier<String> back = (Supplier<String>) o;           // cast
    Set<Predicate<String>> set = new LinkedHashSet<>();     // element of a set
    Map<Function<String, String>, String> map = new HashMap<>();  // key of a map
    Keyed<Supplier<String>, String> keyed = new Keyed<>();  // key through a class of the program
    asObject(b);                                            // to Object (argument)
    Object[] arr = {a};
    return "" + same + isSup + back + set + map + keyed + a.equals(b) + arr.length;
  }
}
