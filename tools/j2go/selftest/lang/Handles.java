package lang;

import java.util.ArrayList;
import java.util.Arrays;
import java.util.Comparator;
import java.util.IdentityHashMap;
import java.util.Iterator;
import java.util.LinkedHashMap;
import java.util.LinkedHashSet;
import java.util.List;
import java.util.Map;
import java.util.Objects;
import java.util.Optional;
import java.util.Set;
import java.util.function.Function;
import java.util.function.Predicate;
import java.util.function.Supplier;

/**
 * Handle classes: Item and Tag are listed in tools/j2go/selftest/handles.txt, so the Go translation
 * represents them as 32-bit handles into a store. On the JVM they are ordinary classes; both must
 * print the same transcript.
 */
public class Handles {
  static final StringBuilder out = new StringBuilder();

  static void p(Object s) {
    out.append(s).append('\n');
  }

  enum Kind {
    LEAF,
    BRANCH,
    ROOT
  }

  interface Named {
    String label();

    default String shout() {
      return label() + label() + "!";
    }
  }

  /** A second handle class, with equals and hashCode of its own and Object's toString. */
  static final class Tag {
    final String text;
    final int weight;
    Item owner;

    Tag(String text, int weight) {
      this.text = text;
      this.weight = weight;
    }

    @Override
    public boolean equals(Object o) {
      return o instanceof Tag t && t.text.equals(text);
    }

    @Override
    public int hashCode() {
      return text.hashCode();
    }
  }

  static final class Item implements Named, Comparable<Item>, Iterable<Item> {
    static int created;
    static Item last;
    static final Item NONE = null;

    // One field of every storage kind.
    int id;
    long big = 1L << 40;
    double weight = 1.5;
    boolean flag;
    char letter = 'q';
    byte small = -3;
    Kind kind = Kind.LEAF;
    Kind other;
    Item next;
    Item parent;
    Item first;
    Tag tag;
    String name = "";
    String note;
    Object payload;
    List<Item> extra;
    int[] counts;
    Function<Item, String> describe;

    Item(int id) {
      this.id = id;
      created++;
      last = this;
    }

    Item(int id, String name, Kind kind) {
      this(id);
      this.name = name;
      this.kind = kind;
    }

    Item getNext() {
      return next;
    }

    Item add(Item child) {
      child.parent = this;
      child.next = null;
      if (first == null) {
        first = child;
      } else {
        Item c = first;
        while (c.next != null) {
          c = c.next;
        }
        c.next = child;
      }
      kind = parent == null ? Kind.ROOT : Kind.BRANCH;
      return this;
    }

    int count() {
      int n = 1;
      for (Item c = first; c != null; c = c.next) {
        n += c.count();
      }
      return n;
    }

    /** A copy without links, like Node.cloneNode. */
    Item copy() {
      Item c = new Item(id);
      c.big = big;
      c.weight = weight;
      c.flag = flag;
      c.letter = letter;
      c.kind = kind;
      c.other = other;
      c.tag = tag;
      c.name = name;
      c.note = note;
      c.payload = payload;
      c.counts = counts;
      return c;
    }

    Item copyTree() {
      Item c = copy();
      for (Item k = first; k != null; k = k.next) {
        c.add(k.copyTree());
      }
      return c;
    }

    boolean sameAs(Item o) {
      return this == o;
    }

    Supplier<String> namer() {
      return () -> name + "#" + id;
    }

    @Override
    public Iterator<Item> iterator() {
      return new Iterator<Item>() {
        Item cur = first;

        @Override
        public boolean hasNext() {
          return cur != null;
        }

        @Override
        public Item next() {
          Item r = cur;
          cur = cur.next;
          return r;
        }
      };
    }

    @Override
    public String label() {
      return name;
    }

    @Override
    public int compareTo(Item o) {
      return Integer.compare(id, o.id);
    }

    @Override
    public String toString() {
      return "Item(" + id + "," + name + "," + kind + ")";
    }
  }

  static final class Holder {
    Item item;
    Item[] items = new Item[3];
    final Map<String, Item> byName = new LinkedHashMap<>();
  }

  static int countAll(Item... items) {
    int n = 0;
    for (Item i : items) {
      if (i != null) {
        n++;
      }
    }
    return n;
  }

  static <T> T firstOf(List<T> l) {
    return l.isEmpty() ? null : l.get(0);
  }

  static String classify(Object o) {
    return switch (o) {
      case null -> "null";
      case Item i -> "item " + i.id;
      case Tag t -> "tag " + t.text;
      case String s -> "string " + s;
      default -> "other";
    };
  }

  static String fields() {
    StringBuilder sb = new StringBuilder();
    Item a = new Item(1, "a", Kind.BRANCH);
    sb.append(a.id).append(' ').append(a.big).append(' ').append(a.weight).append(' ').append(a.flag).append(' ').append(a.letter)
        .append(' ').append(a.small).append(' ').append(a.kind).append(' ').append(a.other).append(' ').append(a.next).append(' ')
        .append(a.tag).append(' ').append(a.name).append(' ').append(a.note).append(' ').append(a.payload).append(' ').append(a.extra)
        .append(' ').append(a.counts == null).append(' ').append(a.describe == null).append('\n');
    a.id += 41;
    a.id++;
    a.big >>= 3;
    a.weight *= 4;
    a.flag |= true;
    a.letter++;
    a.small -= 100;
    a.kind = null;
    a.other = Kind.ROOT;
    a.name += "bc";
    a.note = "n" + a.id;
    a.payload = 3.5;
    a.extra = new ArrayList<>();
    a.extra.add(a);
    a.counts = new int[] {4, 5};
    a.counts[1] += a.id;
    a.describe = x -> x.name + "/" + x.kind;
    a.tag = new Tag("t", 2);
    a.tag.owner = a;
    sb.append(a.id).append(' ').append(a.big).append(' ').append(a.weight).append(' ').append(a.flag).append(' ').append(a.letter)
        .append(' ').append(a.small).append(' ').append(a.kind).append(' ').append(a.other).append(' ').append(a.name).append(' ')
        .append(a.note).append(' ').append(a.payload).append(' ').append(a.extra.size()).append(' ').append(a.extra.get(0) == a)
        .append(' ').append(a.counts[1]).append(' ').append(a.describe.apply(a)).append(' ').append(a.tag.text).append(' ')
        .append(a.tag.owner == a).append(' ').append(a.tag.owner.tag.weight).append('\n');
    a.payload = a;
    sb.append(a.payload == a).append(' ').append(((Item) a.payload).id).append(' ').append(Item.created > 0).append(' ')
        .append(Item.last == a).append(' ').append(Item.NONE == null).append('\n');
    Item b = a.copy();
    sb.append(b != a).append(' ').append(b.id).append(' ').append(b.name).append(' ').append(b.other).append(' ').append(b.kind)
        .append(' ').append(b.tag == a.tag).append(' ').append(b.payload == a).append(' ').append(b.counts == a.counts).append('\n');
    return sb.toString();
  }

  static String nulls() {
    StringBuilder sb = new StringBuilder();
    Item n = null;
    try {
      sb.append(n.getNext());
    } catch (NullPointerException e) {
      sb.append("npe-method ");
    }
    try {
      sb.append(n.id);
    } catch (NullPointerException e) {
      sb.append("npe-int ");
    }
    try {
      sb.append(n.name);
    } catch (NullPointerException e) {
      sb.append("npe-string ");
    }
    try {
      sb.append(n.kind);
    } catch (NullPointerException e) {
      sb.append("npe-enum ");
    }
    try {
      n.id = 3;
    } catch (NullPointerException e) {
      sb.append("npe-store ");
    }
    try {
      n.payload = "x";
    } catch (NullPointerException e) {
      sb.append("npe-store-object ");
    }
    Item a = new Item(7);
    try {
      sb.append(a.next.next);
    } catch (NullPointerException e) {
      sb.append("npe-link ");
    }
    try {
      sb.append(a.tag.text);
    } catch (RuntimeException e) {
      sb.append("npe-other-class:").append(e instanceof NullPointerException).append(' ');
    }
    try {
      sb.append(n.toString());
    } catch (NullPointerException e) {
      sb.append("npe-toString ");
    }
    try {
      sb.append(n.equals(a));
    } catch (NullPointerException e) {
      sb.append("npe-equals ");
    }
    Object o = n;
    Item back = (Item) o;
    sb.append('\n').append(o == null).append(' ').append(back == null).append(' ').append(o instanceof Item).append(' ')
        .append(n == Item.NONE).append(' ').append("s" + n).append(' ').append(String.valueOf((Object) n)).append(' ')
        .append(Objects.equals(n, null)).append(' ').append(Objects.equals(n, a)).append(' ').append(a.equals(n)).append(' ')
        .append(classify(n)).append(' ').append(Optional.ofNullable(n).isPresent()).append(' ').append(countAll(n, a, null));
    Item t = a.id > 100 ? a : null;
    Item u = a.id > 1 ? a : null;
    Item[] arr = new Item[2];
    List<Item> l = new ArrayList<>();
    l.add(null);
    l.add(a);
    Map<String, Item> m = new LinkedHashMap<>();
    m.put("k", null);
    sb.append(' ').append(t == null).append(' ').append(u == a).append(' ').append(arr[0] == null).append(' ').append(l.get(0) == null)
        .append(' ').append(l.indexOf(null)).append(' ').append(l.contains(a)).append(' ').append(m.get("k") == null).append(' ')
        .append(m.get("zz") == null).append(' ').append(m.containsKey("k")).append(' ').append(l).append('\n');
    return sb.toString();
  }

  static String objects() {
    StringBuilder sb = new StringBuilder();
    Item a = new Item(1, "a", Kind.LEAF), b = new Item(2, "b", Kind.LEAF);
    Object o = a;
    sb.append(o == a).append(' ').append(o != b).append(' ').append(o instanceof Item).append(' ').append(o instanceof Tag).append(' ')
        .append(o instanceof Named).append(' ').append(o instanceof Iterable).append(' ').append(((Item) o).id).append(' ').append(o)
        .append(' ').append(o.toString()).append(' ').append(o.equals(a)).append(' ').append(o.equals(b)).append(' ')
        .append(a.equals(a)).append(' ').append(a.equals(b)).append(' ').append(a.equals("a")).append(' ')
        .append(a.hashCode() == a.hashCode()).append(' ').append(o.hashCode() == a.hashCode()).append(' ')
        .append(System.identityHashCode(a) == System.identityHashCode(o)).append(' ').append(a.hashCode() == System.identityHashCode(a))
        .append(' ').append(a.sameAs(a)).append(' ').append(a.sameAs(b)).append('\n');
    if (o instanceof Item it && it.id == 1) {
      sb.append("pattern ").append(it.name).append(' ');
    }
    try {
      Object s = "text";
      sb.append(((Item) s).id);
    } catch (ClassCastException e) {
      sb.append("cce ");
    }
    try {
      Object s = new Tag("x", 1);
      sb.append(((Item) s).id);
    } catch (ClassCastException e) {
      sb.append("cce2 ");
    }
    Named named = a;
    Comparable<Item> cmp = a;
    Item fromNamed = (Item) named;
    sb.append(named.label()).append(' ').append(named.shout()).append(' ').append(a.shout()).append(' ').append(cmp.compareTo(b)).append(' ')
        .append(fromNamed == a).append(' ').append(named == o).append(' ').append(classify(a)).append(' ').append(classify(new Tag("g", 0)))
        .append(' ').append(classify("s")).append(' ').append(classify(4)).append(' ').append(a.getClass() == b.getClass()).append(' ').append(a.getClass() == o.getClass()).append(' ')
        .append(a.getClass().getSimpleName()).append(' ').append(o.getClass().getName()).append('\n');
    Tag t1 = new Tag("same", 1), t2 = new Tag("same", 2), t3 = new Tag("else", 3);
    Object ot = t1;
    sb.append(t1.equals(t2)).append(' ').append(t1 == t2).append(' ').append(t1.equals(t3)).append(' ').append(t1.hashCode() == t2.hashCode())
        .append(' ').append(ot.equals(t2)).append(' ').append(Objects.equals(t2, ot)).append(' ').append(Objects.hash(t1) == Objects.hash(t2))
        .append(' ').append(t1.toString().startsWith("lang.Handles$Tag@")).append(' ').append(("" + t1).equals(t1.toString())).append(' ')
        .append(System.identityHashCode(t1) == System.identityHashCode(t1)).append(' ').append("a" + a + 'b' + b + 1 + null + a.next).append('\n');
    return sb.toString();
  }

  static String collections() {
    StringBuilder sb = new StringBuilder();
    List<Item> items = new ArrayList<>();
    for (int i = 0; i < 6; i++) {
      items.add(new Item(i, "n" + i, i % 2 == 0 ? Kind.LEAF : Kind.BRANCH));
    }
    Item a = items.get(0), c = items.get(2), e = items.get(4);
    Map<Item, String> names = new LinkedHashMap<>();
    Set<Item> set = new LinkedHashSet<>();
    Map<Item, Integer> ident = new IdentityHashMap<>();
    for (Item i : items) {
      names.put(i, i.name);
      set.add(i);
      ident.put(i, i.id * 10);
    }
    names.put(c, "again");
    set.add(c);
    set.remove(e);
    sb.append(names.size()).append(' ').append(names.get(c)).append(' ').append(names.get(new Item(2))).append(' ').append(names.containsKey(a))
        .append(' ').append(set.size()).append(' ').append(set.contains(c)).append(' ').append(set.contains(e)).append(' ').append(set)
        .append(' ').append(ident.get(a)).append(' ').append(ident.get(e)).append(' ').append(ident.containsKey(new Item(0))).append(' ')
        .append(items.indexOf(e)).append(' ').append(items.contains(c)).append(' ').append(firstOf(items)).append(' ')
        .append(firstOf(new ArrayList<Item>()) == null).append('\n');
    items.sort(Comparator.comparing((Item i) -> i.name).reversed());
    sb.append(items).append(' ');
    items.sort(null);
    sb.append(items.get(0).id).append(' ');
    Item[] arr = items.toArray(new Item[0]);
    Arrays.sort(arr, (x, y) -> y.id - x.id);
    Item[][] grid = new Item[2][3];
    grid[1][2] = c;
    Object[] objs = arr;
    sb.append(arr.length).append(' ').append(arr[0]).append(' ').append(objs[5] == a).append(' ').append(grid[0][0] == null).append(' ')
        .append(grid[1][2].name).append(' ').append(countAll()).append(' ').append(countAll(a)).append(' ').append(countAll(arr)).append(' ')
        .append(Arrays.asList(arr).indexOf(c)).append('\n');
    Map<Tag, Item> byTag = new LinkedHashMap<>();
    byTag.put(new Tag("x", 1), a);
    byTag.put(new Tag("y", 1), c);
    byTag.put(new Tag("x", 9), e);
    Set<Tag> tags = new LinkedHashSet<>(byTag.keySet());
    sb.append(byTag.size()).append(' ').append(byTag.get(new Tag("x", 0))).append(' ').append(byTag.get(new Tag("z", 0))).append(' ')
        .append(tags.contains(new Tag("y", 5))).append(' ').append(byTag.values()).append('\n');
    Holder h = new Holder();
    h.item = a;
    h.items[1] = c;
    h.byName.put("e", e);
    Supplier<Item> sup = () -> h.item;
    Function<Item, Item> nextOf = Item::getNext;
    Function<Item, String> label = Item::label;
    Predicate<Item> leaf = i -> i.kind == Kind.LEAF;
    Supplier<Item> maker = () -> new Item(99, "made", Kind.ROOT);
    Function<Integer, Item> ctor = Item::new;
    a.next = c;
    Optional<Item> opt = Optional.of(c);
    sb.append(sup.get() == a).append(' ').append(nextOf.apply(a) == c).append(' ').append(nextOf.apply(c)).append(' ').append(label.apply(e))
        .append(' ').append(leaf.test(a)).append(' ').append(maker.get()).append(' ').append(ctor.apply(5).id).append(' ')
        .append(opt.get() == c).append(' ').append(opt.isPresent() ? opt.get().name : "?").append(' ').append(h.items[0] == null).append(' ')
        .append(h.items[1]).append(' ').append(h.byName.get("e").id).append(' ').append(a.namer().get()).append('\n');
    return sb.toString();
  }

  static String trees() {
    StringBuilder sb = new StringBuilder();
    Item root = new Item(0, "root", Kind.LEAF);
    root.add(new Item(1, "x", Kind.LEAF)).add(new Item(2, "y", Kind.LEAF).add(new Item(3, "z", Kind.LEAF)));
    Item cp = root.copyTree();
    for (Item k : root) {
      sb.append(k).append(k.parent == root).append(' ');
    }
    sb.append(root.count()).append(' ').append(cp.count()).append(' ').append(cp != root).append(' ').append(cp.first.next.first)
        .append(' ').append(cp.first.next.first.parent.parent == cp).append(' ').append(root.kind).append(' ').append(root.first.next.kind)
        .append('\n');
    // Far more objects than one chunk of the store holds.
    int n = 200_000;
    Item head = null;
    long before = Item.created;
    for (int i = 0; i < n; i++) {
      Item it = new Item(i);
      it.next = head;
      it.kind = Kind.values()[i % 3];
      if (i % 1000 == 0) {
        it.name = "k" + i;
        it.payload = it;
      }
      head = it;
    }
    long sum = 0;
    int kinds = 0, named = 0, self = 0, len = 0;
    for (Item it = head; it != null; it = it.next) {
      sum += it.id;
      kinds += it.kind.ordinal();
      if (!it.name.isEmpty()) {
        named++;
      }
      if (it.payload == it) {
        self++;
      }
      if (it.big != 1L << 40 || it.weight != 1.5 || it.letter != 'q' || it.note != null || it.parent != null) {
        sb.append("bad defaults at ").append(it.id).append('\n');
        break;
      }
      len++;
    }
    Set<Item> all = new LinkedHashSet<>();
    for (Item it = head; it != null; it = it.next) {
      all.add(it);
    }
    sb.append(len).append(' ').append(sum).append(' ').append(kinds).append(' ').append(named).append(' ').append(self).append(' ')
        .append(Item.created - before).append(' ').append(all.size()).append(' ').append(all.contains(head)).append(' ')
        .append(head.id).append(' ').append(Item.last == head).append('\n');
    return sb.toString();
  }

  public static String run() {
    p(fields());
    p(nulls());
    p(objects());
    p(collections());
    p(trees());
    return out.toString();
  }

  public static void main(String[] args) {
    System.out.print(run());
  }
}
