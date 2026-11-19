package lang;

import java.util.ArrayList;
import java.util.List;

/**
 * Rule "covariant overrides": a method that overrides one with a wider result type has a group
 * of its own with the narrower result, and its class implements the overridden group through a
 * bridge. Calls through the wider type reach the override; calls through the narrower type need
 * no cast; a second narrowing, an override of the narrowed method, super calls, an interface
 * method narrowed by a class and an abstract method narrowed by its implementations all work as
 * in Java.
 */
public class Covariant {
  static StringBuilder out = new StringBuilder();

  static void p(Object o) {
    out.append(o).append('\n');
  }

  interface Shape {
    Shape scaled(int k);

    String name();
  }

  abstract static class Base implements Shape {
    final int size;

    Base(int size) {
      this.size = size;
    }

    abstract Base parent();

    Base me() {
      return this;
    }

    Object tag() {
      return "base" + size;
    }

    @Override
    public String name() {
      return "base";
    }
  }

  static class Box extends Base {
    final Box up;

    Box(int size, Box up) {
      super(size);
      this.up = up;
    }

    @Override
    Box parent() {
      return up;
    }

    @Override
    Box me() {
      return this;
    }

    @Override
    String tag() {
      return "box" + size;
    }

    @Override
    public Box scaled(int k) {
      return new Box(size * k, up);
    }

    @Override
    public String name() {
      return "box";
    }

    int depth() {
      int d = 0;
      for (Box b = parent(); b != null; b = b.parent()) {
        d++;
      }
      return d;
    }
  }

  static final class Crate extends Box {
    Crate(int size, Box up) {
      super(size, up);
    }

    @Override
    Crate me() {
      return this;
    }

    @Override
    String tag() {
      return "crate/" + super.tag();
    }

    @Override
    public Crate scaled(int k) {
      return new Crate(size * k + 1, up);
    }

    @Override
    public String name() {
      return "crate";
    }

    int twice() {
      return me().scaled(2).size;
    }
  }

  static final class Dot extends Base {
    Dot() {
      super(0);
    }

    @Override
    Dot parent() {
      return null;
    }

    @Override
    public Shape scaled(int k) {
      return this;
    }
  }

  public static String run() {
    Box root = new Box(1, null);
    Box mid = new Box(2, root);
    Crate leaf = new Crate(3, mid);
    // Through the narrower types.
    p(leaf.depth() + " " + mid.depth() + " " + root.depth());
    p(leaf.parent().parent().size + " " + (root.parent() == null));
    p(leaf.me().twice() + " " + leaf.scaled(3).size + " " + mid.scaled(3).size);
    p(leaf.tag() + " " + mid.tag().length());
    // Through the wider types: the bridges.
    List<Base> bases = new ArrayList<>();
    bases.add(root);
    bases.add(mid);
    bases.add(leaf);
    bases.add(new Dot());
    for (Base b : bases) {
      Base parent = b.parent();
      Shape s = b.scaled(2);
      p(b.name() + " " + b.tag() + " " + (parent == null ? "-" : parent.name() + parent.size) + " "
          + s.name() + " " + (b.me() == b) + " " + (s == b));
    }
    Box asBox = leaf;
    p(asBox.me().name() + " " + asBox.scaled(2).size + " " + asBox.tag());
    Shape sh = leaf;
    p(sh.scaled(5).name() + " " + ((Base) sh.scaled(5)).size);
    String r = out.toString();
    out.setLength(0);
    return r;
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
