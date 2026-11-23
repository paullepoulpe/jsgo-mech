package lang;

import com.google.common.collect.ImmutableSortedSet;
import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;

/** Every marked use here must be refused by rule "typed library interfaces" (8). */
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
    return "" + raw + strs + m + isList + lm + isSet + ok + ss + ssOk;
  }
}
