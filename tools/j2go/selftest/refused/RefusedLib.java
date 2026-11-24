package lang;

import com.google.common.base.Optional;
import com.google.common.base.Suppliers;
import com.google.common.collect.ImmutableSortedSet;
import java.util.ArrayList;
import java.util.EnumSet;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;

/** Every marked use here must be refused by rule "typed library interfaces" (15). */
public class RefusedLib {
  static String run(Object o, List<?> objs, ImmutableSortedSet<?> sorted) {
    List raw = new ArrayList<String>();                         // 1: a raw use of a typed interface
    List<String> strs = (List<String>) objs;                    // 2: an unchecked cast between two instantiations
    Map<String, Integer> m = (Map<String, Integer>) o;          // 3: an unchecked cast of an Object
    boolean isList = o instanceof ArrayList;                    // 4: instanceof a collection class
    LinkedHashMap<String, String> lm = (LinkedHashMap<String, String>) o;  // 5: a cast to a collection class
    boolean isSet = o instanceof LinkedHashSet<?> s && s.isEmpty();        // 6: instanceof with a pattern
    ArrayList<String> al = new ArrayList<>();
    al.ensureCapacity(3);                                       // 7: a method of the collection class itself that the typed view lacks
    al.trimToSize();                                            // not refused: the view has it (LibGen.OWN)
    List<?> ok = (List<?>) o;                                   // not refused: nothing is claimed about the elements
    ImmutableSortedSet<String> ss = (ImmutableSortedSet<String>) sorted;  // 8: an unchecked cast to a class of LibGen.CLASSES_T2 (kind sorted)
    ImmutableSortedSet<?> ssOk = (ImmutableSortedSet<?>) o;               // not refused
    boolean isEnumSet = o instanceof EnumSet;                   // 9: instanceof a class whose typed places LibGen.MAKERS_T2 serve (kind enumset)
    EnumSet<java.util.concurrent.TimeUnit> units = EnumSet.copyOf(EnumSet.noneOf(java.util.concurrent.TimeUnit.class));  // 10: a library parameter of that class
    Optional<String> opt = (Optional<String>) o;                // 11: an unchecked cast to a Guava Optional (kind goptional)
    Optional<?> optOk = (Optional<?>) o;                        // not refused
    boolean same = opt == Optional.of("a");                     // 12: identity of a Guava Optional (a value)
    boolean isNull = opt == null;                               // not refused
    String orS = opt.or(Suppliers.ofInstance("s"));             // 13: a method that the runtime's GOptional does not have (the third overload of or)
    java.util.Set<String> asSet = opt.asSet();                  // 14: another
    com.google.common.base.Function<String, Optional<String>> wrap = Optional::fromNullable;  // 15: a reference to a generic static of Guava's Optional
    return "" + raw + strs + m + isList + lm + isSet + ok + ss + ssOk + isEnumSet + units + opt + optOk + same + isNull + orS + asSet + wrap;
  }
}
