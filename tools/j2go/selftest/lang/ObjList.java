package lang;

import com.google.common.collect.ImmutableList;
import java.util.ArrayList;
import java.util.Arrays;
import java.util.Collection;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Objects;

/**
 * Piece `objlist` of rule "typed library interfaces" (j2go.LibObj.listTwin; RESIDUE8 T3.1):
 * contains / remove(Object) and indexOf / lastIndexOf on a list (or a collection), with an
 * argument of the element type whose equals is Go's == (a String, an enum, a class of the program
 * without equals of its own), scan the typed list with == instead of boxing the argument and each
 * element. typed() has the twins' calls (a typed list itself, a List and a Collection of a method
 * with a type parameter, whose objects are typed lists, a typed set, an erased list); erased() the
 * calls that stay erased (a class with its own equals, an interface element type, an Object
 * argument). selftest/run.sh counts both. Null arguments and null elements included.
 */
public class ObjList {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  enum Color {
    RED,
    GREEN,
    BLUE
  }

  static class Thing {
    final String n;

    Thing(String n) {
      this.n = n;
    }

    @Override
    public String toString() {
      return "T:" + n;
    }
  }

  static class SubThing extends Thing {
    SubThing(String n) {
      super(n);
    }
  }

  /** A class with its own equals: Java's x.equals(e) is not ==. */
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

  interface Shape {}

  static class Sq implements Shape {
    @Override
    public String toString() {
      return "Sq";
    }
  }

  static final Thing A = new Thing("a");
  static final SubThing B = new SubThing("b");

  // receivers that are typed interfaces (a type parameter keeps rule "typed collections" off them)

  static <X> String lists(List<Thing> l, Thing t, SubThing s) {
    Thing none = null;
    return l.indexOf(t) + " " + l.indexOf(s) + " " + l.lastIndexOf(t) + " " + l.indexOf(none) + " " + l.lastIndexOf(none) + " " + l.contains(s) + l.contains(new Thing("x")) + l.remove(t) + " " + l;
  }

  static <X> String colls(Collection<Thing> c, Thing t) {
    Thing none = null;
    return c.contains(t) + " " + c.contains(none) + " " + c.contains(B) + " " + c.remove(t) + c.remove(t) + c.remove(none) + " " + c;
  }

  static <X> String strs(List<String> l, String s) {
    String none = null;
    return l.indexOf(s) + " " + l.lastIndexOf(s) + " " + l.indexOf(none) + " " + l.indexOf("zz");
  }

  static String typed() {
    // typed lists of rule "typed collections": the twin takes the list itself
    List<String> names = new ArrayList<>(Arrays.asList("x", "y", "x", null, "z"));
    String none = null;
    p(names.indexOf("x") + " " + names.lastIndexOf("x") + " " + names.indexOf(none) + " " + names.lastIndexOf(none) + " " + names.contains("z") + names.contains("q"));
    p(names.remove("y") + " " + names.remove("q") + " " + names.remove(none) + " " + names);
    List<Color> colors = new ArrayList<>();
    colors.add(Color.BLUE);
    colors.add(Color.RED);
    colors.add(Color.BLUE);
    Color c0 = null;
    p(colors.indexOf(Color.BLUE) + " " + colors.lastIndexOf(Color.BLUE) + " " + colors.indexOf(Color.GREEN) + " " + colors.contains(Color.RED) + colors.contains(c0) + colors.remove(Color.BLUE) + " " + colors);
    List<Thing> things = new ArrayList<>();
    things.add(A);
    things.add(B);
    things.add(null);
    Thing t0 = null;
    p(things.indexOf(B) + " " + things.indexOf(A) + " " + things.indexOf(t0) + " " + things.contains(new Thing("a")) + things.remove(A) + things.remove(A) + " " + things);
    // an immutable list's twin finds what the view finds
    List<Color> imm = ImmutableList.of(Color.GREEN, Color.RED);
    p(imm.indexOf(Color.RED) + " " + imm.contains(Color.BLUE) + " " + imm.lastIndexOf(Color.GREEN));
    // a typed interface whose object is a typed list, a typed set, or an erased list
    p(lists(new ArrayList<>(Arrays.asList(A, B, null, A)), A, B));
    p(lists(new ArrayList<>(ImmutableList.of(B, A)), A, B));
    List<Thing> erasedList = Arrays.asList(B, A, A);
    p(erasedList.indexOf(A) + " " + erasedList.lastIndexOf(A));
    p(colls(new ArrayList<>(Arrays.asList(A, null, B)), A));
    p(colls(new LinkedHashSet<>(Arrays.asList(B, A)), A));
    p(strs(new ArrayList<>(Arrays.asList("p", "q", "p", null)), "p"));
    p(strs(ImmutableList.of("q"), "q"));
    return "typed done";
  }

  static <X> String eqs(List<Eq> l, Eq e) {
    return l.indexOf(e) + " " + l.contains(e) + " " + l.lastIndexOf(e);
  }

  static String erased() {
    // a class with its own equals, an interface, an Object argument: the erased call
    List<Eq> es = new ArrayList<>(Arrays.asList(new Eq("a"), new Eq("b")));
    String r = es.indexOf(new Eq("b")) + " " + es.contains(new Eq("a")) + es.remove(new Eq("a")) + " " + es;
    r += " " + eqs(new ArrayList<>(Arrays.asList(new Eq("c"))), new Eq("c"));
    Shape sq = new Sq();
    List<Shape> shapes = new ArrayList<>();
    shapes.add(sq);
    r += " " + shapes.indexOf(sq) + shapes.contains(sq);
    Object o = A;
    List<Thing> ts = new ArrayList<>(Arrays.asList(A));
    r += " " + ts.indexOf(o) + ts.contains(o) + " " + Objects.hash(es.size());
    return r;
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
