package lang;

import java.util.ArrayList;
import java.util.Collection;
import java.util.Collections;
import java.util.Iterator;
import java.util.List;
import java.util.function.Supplier;

/**
 * Typed collections (j2go rule "typed collections", Colls): fields and locals of type List<X> /
 * ArrayList<X> that only hold lists made there are typed lists (*TList[X]); every other place
 * sees the same object through its erased view. Elements that are Strings, classes with
 * subclasses, interfaces and handles; null lists and null elements; identity, equals, hashCode
 * and toString across the erased boundary; changes made through the view; the failures.
 */
public class Colls {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  interface Shape {
    String name();
  }

  static class Base implements Shape {
    final int id;

    Base(int id) {
      this.id = id;
    }

    @Override
    public String name() {
      return "base" + id;
    }

    @Override
    public String toString() {
      return name();
    }
  }

  static class Derived extends Base {
    Derived(int id) {
      super(id);
    }

    @Override
    public String name() {
      return "derived" + id;
    }

    @Override
    public boolean equals(Object o) {
      return o instanceof Derived d && d.id == id;
    }

    @Override
    public int hashCode() {
      return id;
    }
  }

  // typed fields
  private final List<String> names = new ArrayList<>();
  private List<Base> bases;
  private static final List<Shape> SHAPES = new ArrayList<>();
  private ArrayList<String> spare = new ArrayList<>(4);
  // stays erased: a list of another origin enters it
  private List<String> mixed = new ArrayList<>();

  static int count(Collection<?> c) {
    int n = 0;
    for (Object o : c) {
      if (o != null) {
        n++;
      }
    }
    return n;
  }

  static Object asObject(Object o) {
    return o;
  }

  static <T> T same(T t) {
    return t;
  }

  static void addTwice(List<String> l, String s) {
    l.add(s);
    l.add(0, s);
  }

  @SuppressWarnings("unchecked")
  static void addErased(Object l, Object x) {
    ((List<Object>) l).add(x);
  }

  List<Base> basesOrNull() {
    return bases;
  }

  static String fail(Runnable r) {
    try {
      r.run();
      return "no failure";
    } catch (RuntimeException e) {
      return e.getClass().getSimpleName() + (e instanceof IndexOutOfBoundsException ? ": " + e.getMessage() : "");
    }
  }

  void fields() {
    names.add("b");
    names.add("a");
    names.add(null);
    names.add(1, "c");
    p(names + " " + names.size() + " " + names.isEmpty() + " " + names.get(1) + " " + names.contains("a") + " " + names.contains(null)
        + " " + names.indexOf("a") + " " + names.indexOf("zz") + " " + names.lastIndexOf(null));
    p(names.set(0, "B") + " " + names.remove(2) + " " + names.remove("zz") + " " + names.remove("c") + " " + names);
    addTwice(names, "x");
    p(names + " " + count(names) + " " + names.hashCode() + " " + names.equals(Collections.singletonList("x")) + " " + (asObject(names) == same(names)));
    Object o = names;
    List<String> viaObject = (List<String>) o;
    viaObject.add("viaObject");
    p(names.get(names.size() - 1) + " " + (viaObject == names) + " " + o.equals(names) + " " + names.equals(o));
    p(bases == null);
    p(basesOrNull() == null);
    p(fail(() -> bases.size()));
    p(fail(() -> {
      for (Base b : bases) {
        p(b);
      }
    }));
    bases = new ArrayList<>();
    Derived d = new Derived(7);
    bases.add(d);
    bases.add(new Base(1));
    bases.add(null);
    Base first = bases.get(0);
    p(first.name() + " " + (first == d) + " " + (asObject(bases.get(0)) == d) + " " + bases.contains(new Derived(7)) + " " + bases.indexOf(new Derived(8))
        + " " + bases.contains(null) + " " + bases);
    addErased(bases, new Derived(9));
    p(bases.get(3).name() + " " + bases.size() + " " + basesOrNull().get(3).id + " " + (basesOrNull() == bases));
    p(fail(() -> bases.get(9)));
    p(fail(() -> bases.get(-1)));
    p(fail(() -> bases.add(9, null)));
    p(fail(() -> bases.remove(5)));
    for (Base b : bases) {
      p(b == null ? "null" : b.name());
    }
    SHAPES.add(d);
    SHAPES.add(() -> "lambda");
    StringBuilder sb = new StringBuilder();
    for (Shape s : SHAPES) {
      sb.append(s.name()).append(' ');
    }
    p(sb);
    spare.add("s");
    spare.trimToSize();
    ArrayList<String> spare2 = spare;
    spare2.add("t");
    p(spare + " " + (spare == spare2));
    mixed.add("m");
    mixed = Collections.unmodifiableList(mixed);
    p(mixed);
  }

  static void locals(boolean flag) {
    List<String> l = new ArrayList<>();
    l.add("one");
    l.add("two");
    List<String> copy = new ArrayList<>(l);
    List<String> alias = flag ? l : copy;
    alias.add("three");
    copy.add("four");
    p(l + " " + copy + " " + (alias == l) + " " + (alias == copy) + " " + l.equals(copy));
    List<String> fromErased = new ArrayList<>(Collections.nCopies(2, "x"));
    fromErased.addAll(l);
    fromErased.addAll(Collections.singletonList("z"));
    p(fromErased + " " + fromErased.containsAll(l) + " " + String.join(",", fromErased));
    l.add("one!");
    l.add("two!");
    p(l);
    // removal through the iterator of the view
    for (Iterator<String> it = l.iterator(); it.hasNext(); ) {
      if (it.next().endsWith("!")) {
        it.remove();
      }
    }
    p(l);
    Collections.sort(l);
    p(l);
    l.sort((a, b) -> b.compareTo(a));
    p(l + " " + l.toArray().length + " " + l.toArray(new String[0])[0]);
    l.clear();
    p(l + " " + l.isEmpty());
    List<Base> none = null;
    p(none == null);
    p(fail(() -> none.add(null)));
    // captured by a lambda and by an anonymous class
    List<Base> captured = new ArrayList<>();
    Supplier<Integer> size = () -> captured.size();
    Runnable adder = new Runnable() {
      @Override
      public void run() {
        captured.add(new Base(captured.size()));
      }
    };
    adder.run();
    adder.run();
    p(size.get() + " " + captured);
    // a list of lists stays erased; its elements are views
    List<List<Base>> nested = new ArrayList<>();
    nested.add(captured);
    nested.get(0).add(new Derived(3));
    p(captured.size() + " " + (nested.get(0) == captured) + " " + nested);
    List<Integer> ints = new ArrayList<>();
    ints.add(3);
    p(ints);
  }

  // ---- parameters and results: constructors, static and private methods, methods nothing overrides
  static class Bag {
    private final List<Base> items;
    private final ArrayList<String> tags;

    Bag(List<Base> items, ArrayList<String> tags) {
      this.items = items;
      this.tags = tags;
    }

    List<Base> items() {
      return items;
    }

    ArrayList<String> tags() {
      return tags;
    }

    // overridden below: its signature stays erased
    List<String> labels() {
      List<String> l = new ArrayList<>();
      l.add("bag");
      return l;
    }

    private Base last(List<Base> from) {
      return from.isEmpty() ? null : from.get(from.size() - 1);
    }

    Base lastItem() {
      return last(items);
    }
  }

  static class LabelledBag extends Bag {
    LabelledBag() {
      super(new ArrayList<>(), null);
    }

    @Override
    List<String> labels() {
      return Collections.singletonList("labelled");
    }
  }

  static List<Base> make(int n) {
    List<Base> l = new ArrayList<>(n);
    for (int i = 0; i < n; i++) {
      l.add(i % 2 == 0 ? new Base(i) : new Derived(i));
    }
    return l;
  }

  static List<Base> makeOrNull(boolean flag) {
    if (flag) {
      return null;
    }
    return make(1);
  }

  static int total(List<Base> l) {
    int t = 0;
    for (Base b : l) {
      t += b.id;
    }
    return t;
  }

  static List<String> upper(List<String> in) {
    List<String> out = new ArrayList<>();
    for (String s : in) {
      out.add(s + s);
    }
    return out;
  }

  static void methods() {
    List<Base> made = make(4);
    ArrayList<String> tags = new ArrayList<>();
    tags.add("t1");
    Bag bag = new Bag(made, tags);
    bag.items().add(new Base(10));
    bag.tags().add("t2");
    p(made + " " + tags + " " + (bag.items() == made) + " " + (bag.tags() == tags) + " " + total(bag.items()) + " " + bag.lastItem() + " " + bag.labels());
    Bag labelled = new LabelledBag();
    p(labelled.labels() + " " + labelled.items() + " " + labelled.lastItem() + " " + (labelled.tags() == null) + " " + count(labelled.items()));
    p((makeOrNull(true) == null) + " " + makeOrNull(false) + " " + fail(() -> total(makeOrNull(true))));
    // named by a method reference: called at the erased signature
    java.util.function.Function<List<String>, List<String>> f = Colls::upper;
    List<String> words = new ArrayList<>();
    words.add("a");
    words.add("b");
    p(f.apply(words) + " " + upper(words) + " " + upper(Collections.singletonList("z")));
    Object o = make(2);
    p(o + " " + (o instanceof List) + " " + ((List<?>) o).size() + " " + o.getClass().getSimpleName().isEmpty());
  }

  public static String run() {
    new Colls().fields();
    locals(true);
    locals(false);
    methods();
    return out.toString();
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
