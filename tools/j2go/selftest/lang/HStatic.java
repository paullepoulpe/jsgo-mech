package lang;

import com.google.common.collect.ImmutableMap;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.EnumSet;
import java.util.List;
import java.util.Objects;
import java.util.function.Function;
import java.util.function.Supplier;

/**
 * Piece `hstatic` of rule "typed library interfaces" (rule candidate T7 of
 * docs/javaisms6/JAVAISMS6.md): a static field of a handle class (H is listed in
 * tools/j2go/selftest/handles.txt) is a Go variable as any other static field, so rule "typed
 * collections" decides it as it decides those (its instance fields live in the handle's chunks
 * and stay erased). GETTERS is Node.PROP_MAP_FOR_EQUALITY's form: a builder chain of functions
 * whose element type is a function type, typed with the function type's own conversion and no
 * converting closure per function; MAKERS a list of suppliers. NAMES is left erased (a value that
 * is no new collection enters it). selftest/run.sh counts the declarations in g_HStatic.go.
 */
public class HStatic {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  enum P {
    FLAG,
    KEY,
    TWICE
  }

  /** A handle class (tools/j2go/selftest/handles.txt). */
  static final class H {
    final int k;
    final boolean f;

    H(int k, boolean f) {
      this.k = k;
      this.f = f;
    }

    boolean isF() {
      return f;
    }

    int getK() {
      return k;
    }

    static final ImmutableMap<P, Function<H, Object>> GETTERS =
        new ImmutableMap.Builder<P, Function<H, Object>>()
            .put(P.FLAG, H::isF)
            .put(P.KEY, H::getK)
            .put(P.TWICE, (h) -> h.k * 2)
            .buildOrThrow();

    static final EnumSet<P> KEYS = EnumSet.copyOf(GETTERS.keySet());

    static final List<Supplier<H>> MAKERS = new ArrayList<>();

    static List<String> names = Arrays.asList("n1", "n2");

    boolean same(H o, P only) {
      for (P p : KEYS) {
        if (only != null && p != only) continue;
        Function<H, Object> getter = GETTERS.get(p);
        if (!Objects.equals(getter.apply(this), getter.apply(o))) return false;
      }
      return true;
    }
  }

  public static String run() {
    H a = new H(3, true), b = new H(3, false), c = new H(4, true);
    p("same a b " + a.same(b, null) + " " + a.same(b, P.KEY) + " " + a.same(b, P.FLAG));
    p("same a c " + a.same(c, null) + " " + a.same(c, P.FLAG) + " " + a.same(c, P.TWICE));
    p("twice " + H.GETTERS.get(P.TWICE).apply(c) + " keys " + H.KEYS + " size " + H.GETTERS.size());
    p("missing " + (H.GETTERS.get(null) == null));
    H.MAKERS.add(() -> new H(7, false));
    H.MAKERS.add(() -> a);
    int sum = 0;
    for (Supplier<H> s : H.MAKERS) sum += s.get().getK();
    p("makers " + H.MAKERS.size() + " " + sum + " " + (H.MAKERS.get(1).get() == a));
    p("names " + H.names);
    return out.toString();
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
