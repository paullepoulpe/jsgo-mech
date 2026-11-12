package lang;

import java.util.ArrayList;
import java.util.LinkedHashMap;
import java.util.List;
import java.util.Map;

/** Exercises the Java constructs the translator lowers; run() returns a transcript. */
public class Lang {
  static final StringBuilder out = new StringBuilder();

  static void p(String s) {
    out.append(s).append('\n');
  }

  // ---- inheritance and dispatch
  interface Shape {
    double area();

    default String describe() {
      return name() + ":" + area();
    }

    String name();
  }

  abstract static class Base implements Shape {
    protected final String label;
    static int created = 0;

    Base(String label) {
      this.label = label;
      created++;
      init();
    }

    void init() {}

    @Override
    public String name() {
      return "base-" + label;
    }

    @Override
    public String toString() {
      return "Base(" + name() + ")";
    }
  }

  static class Square extends Base {
    final double side;
    String note = "sq";

    Square(double side) {
      super("square");
      this.side = side;
    }

    @Override
    void init() {
      // Runs before the field initializer and the constructor body.
      p("init note=" + note + " side=" + side);
    }

    @Override
    public double area() {
      return side * side;
    }
  }

  static class NamedSquare extends Square {
    NamedSquare() {
      this(3);
    }

    NamedSquare(int n) {
      super(n);
    }

    @Override
    public String name() {
      return "named(" + super.name() + ")";
    }
  }

  enum Op {
    ADD("+") {
      @Override
      int apply(int a, int b) {
        return a + b;
      }
    },
    MUL("*") {
      @Override
      int apply(int a, int b) {
        return a * b;
      }
    },
    NEG("-");

    final String sym;

    Op(String sym) {
      this.sym = sym;
    }

    int apply(int a, int b) {
      return -a;
    }
  }

  record Point(int x, String tag) {
    Point {
      if (tag == null) {
        tag = "none";
      }
    }

    int twice() {
      return x * 2;
    }
  }

  class Inner {
    int k = base + 1;

    int get() {
      return k + base;
    }
  }

  int base = 10;

  // ---- control flow
  static int sw(int x) {
    int r = 0;
    switch (x) {
      case 1:
        r += 1;
      case 2:
        r += 2;
        break;
      case 3: {
        r = 30;
        break;
      }
      default:
        r = -1;
    }
    return r;
  }

  static String sws(String s) {
    return switch (s) {
      case "a", "b" -> "ab";
      case "c" -> {
        String t = s + s;
        yield t;
      }
      default -> "?";
    };
  }

  static String opName(Op op) {
    switch (op) {
      case ADD:
        return "add";
      case MUL:
        return "mul";
      default:
        return "other";
    }
  }

  static int tryIt(int mode) {
    int r = 0;
    try {
      if (mode == 1) {
        throw new IllegalStateException("one");
      }
      if (mode == 2) {
        return 20;
      }
      if (mode == 3) {
        Object o = null;
        o.toString();
      }
      r = 5;
    } catch (IllegalStateException e) {
      r = 100 + e.getMessage().length();
    } catch (RuntimeException e) {
      return 300;
    } finally {
      p("finally " + mode);
    }
    return r;
  }

  static int loops() {
    int total = 0;
    outer:
    for (int i = 0; i < 5; i++) {
      for (int j = 0; j < 5; j++) {
        if (j == 3) {
          continue outer;
        }
        if (i == 3) {
          break outer;
        }
        try {
          if (j == 1) {
            continue;
          }
          total += i * 10 + j;
        } finally {
          total++;
        }
      }
    }
    int k = 0;
    do {
      k += 2;
      if (k == 4) {
        continue;
      }
      total += k;
    } while (k < 8);
    for (int a = 0, b = 10; a < b; a++, b--) {
      total += b - a;
    }
    return total;
  }

  static String numbers() {
    int i = Integer.MAX_VALUE;
    i += 1;
    long l = 1L << 40;
    int sh = -8 >>> 28;
    int sh2 = 1 << 33;
    char c = 'a';
    c += 2;
    c++;
    byte b = (byte) 200;
    short s = (short) 70000;
    int d = (int) 3.99e10;
    int e = (int) -2.5;
    long m = 7 / 2 * 3 + 7 % 3 - -7 / 2;
    double x = 1 / 2 + 1 / 2.0;
    int[] arr = {1, 2, 3};
    arr[i == Integer.MIN_VALUE ? 0 : 1] += 5;
    int n = 0;
    int v = arr[n++] + arr[n++] * 10 + n;
    Integer boxed = 5;
    boxed++;
    int cmp = boxed > 5 ? 1 : 0;
    return i + " " + l + " " + sh + " " + sh2 + " " + c + " " + (int) c + " " + b + " " + s + " " + d + " " + e + " " + m + " " + x
        + " " + v + " " + boxed + " " + cmp + " " + (char) (c + 1) + " " + 1.0f + " " + 100.0 + " " + 1e21 + " " + 0.001 + " " + (0.1 + 0.2);
  }

  static String strings() {
    String s = "héllo 😀!";
    StringBuilder sb = new StringBuilder();
    for (int i = 0; i < s.length(); i++) {
      sb.append((int) s.charAt(i)).append(',');
    }
    String n = null;
    String t = "x" + n + 'c' + 1 + 2L + true + 1.5 + null;
    return sb + " " + s.length() + " " + s.substring(1, 3) + " " + s.indexOf("llo") + " " + s.indexOf('!') + " " + t + " "
        + "abc".compareTo("abd") + " " + "b".equals(n) + " " + (n == null) + " " + s.hashCode() + " " + "".isEmpty();
  }

  interface Fn {
    int call(int x);
  }

  static int twice(Fn f, int x) {
    return f.call(f.call(x));
  }

  String closures() {
    int local = 7;
    List<Fn> fns = new ArrayList<>();
    for (int i = 0; i < 3; i++) {
      final int j = i;
      fns.add(x -> x + j + local + base);
    }
    Fn anon =
        new Fn() {
          int calls = 0;

          @Override
          public int call(int x) {
            calls++;
            return x * local + calls + base;
          }
        };
    int sum = 0;
    for (Fn f : fns) {
      sum += twice(f, 1);
    }
    Inner in = new Inner();
    Map<String, Integer> counts = new LinkedHashMap<>();
    for (String w : new String[] {"a", "b", "a"}) {
      counts.merge(w, 1, Integer::sum);
    }
    return sum + " " + twice(anon, 2) + " " + in.get() + " " + counts + " " + twice(Math::abs, -3);
  }

  static String varargs(String first, Object... rest) {
    return first + rest.length + (rest.length > 0 ? String.valueOf(rest[0]) : "");
  }

  static String patterns(Object o) {
    if (o instanceof String str && !str.isEmpty()) {
      return "str " + str.length();
    } else if (o instanceof Integer i) {
      return "int " + (i + 1);
    } else if (!(o instanceof Shape sh)) {
      return "other";
    } else {
      return sh.describe();
    }
  }

  static int assignExpr() {
    int a, b;
    a = b = 4;
    int c = (a += 3) * 2;
    int[] xs = new int[3];
    int i = 0;
    xs[i++] = i;
    xs[i] = xs[i - 1] = 9;
    boolean t = true, f = false;
    boolean bits = t & f | t ^ f;
    t &= f;
    return a + b + c + xs[0] + xs[1] + i + (bits ? 1 : 0) + (t ? 1 : 0);
  }

  public static String run() {
    Shape sq = new Square(2);
    Square ns = new NamedSquare();
    p(sq.describe() + " " + ns.describe() + " " + ns + " " + Base.created);
    p(Op.ADD.apply(2, 3) + " " + Op.MUL.apply(2, 3) + " " + Op.NEG.apply(2, 3) + " " + Op.valueOf("MUL").sym + " " + Op.values().length
        + " " + Op.NEG.ordinal() + " " + Op.ADD + " " + opName(Op.MUL) + opName(Op.NEG) + " " + (Op.ADD.compareTo(Op.NEG)));
    Point pt = new Point(4, null);
    p(pt + " " + pt.twice() + " " + pt.equals(new Point(4, "none")) + " " + pt.tag() + " " + (pt.hashCode() == new Point(4, "none").hashCode()));
    p(sw(1) + " " + sw(2) + " " + sw(3) + " " + sw(9) + " " + sws("a") + sws("c") + sws("z"));
    p(tryIt(0) + " " + tryIt(1) + " " + tryIt(2) + " " + tryIt(3));
    p("loops " + loops());
    p(numbers());
    p(strings());
    p(new Lang().closures());
    p(varargs("v") + varargs("w", 1, "x") + varargs("n", (Object[]) new String[] {"q"}));
    p(patterns("hey") + " " + patterns(41) + " " + patterns(2.0) + " " + patterns(sq) + " " + patterns(""));
    p("assign " + assignExpr());
    return out.toString();
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
