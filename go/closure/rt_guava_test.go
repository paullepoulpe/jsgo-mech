package closure

import (
	"strings"
	"testing"
)

// Static checks: the types are usable as the generated interfaces.
var (
	_ List         = (*ImmutableList)(nil)
	_ Set          = (*ImmutableSet)(nil)
	_ NavigableSet = (*ImmutableSortedSet)(nil)
	_ Multiset     = (*ImmutableMultiset)(nil)
	_ Map          = (*ImmutableMap)(nil)
	_ BiMap        = (*ImmutableBiMap)(nil)
	_ BiMap        = (*HashBiMap)(nil)
	_ Multimap     = (*ImmutableMultimap)(nil)
	_ Multimap     = (*ArrayListMultimap)(nil)
	_ Multimap     = (*LinkedHashMultimap)(nil)
	_ Multimap     = (*LinkedListMultimap)(nil)
	_ Multimap     = (*TreeMultimap)(nil)
	_ Multiset     = (*LinkedHashMultiset)(nil)
	_ Multiset     = (*TreeMultiset)(nil)
	_ RangeSet     = (*TreeRangeSet)(nil)
	_ Comparator   = (*Ordering)(nil)
	_ List         = (*viewList_)(nil)
	_ List         = (*mmList_)(nil)
	_ Set          = (*Sets_SetView)(nil)
	_ Table        = (*customTable_)(nil)
)

// guavaThrows_ runs f and returns the simple class name of the Java exception it throws ("" if none).
func guavaThrows_(f func()) (name string) {
	defer func() {
		if r := recover(); r != nil {
			name = goTypeName_(r)
		}
	}()
	f()
	return ""
}

func guavaEq_(t *testing.T, got any, want any) {
	t.Helper()
	if !objequals_(got, want) {
		t.Errorf("got %s, want %s", objstr_(got), objstr_(want))
	}
}

func guavaStr_(t *testing.T, got any, want string) {
	t.Helper()
	if s := objstr_(got); s != want {
		t.Errorf("got %s, want %s", s, want)
	}
}

func guavaThrew_(t *testing.T, want string, f func()) {
	t.Helper()
	if got := guavaThrows_(f); got != want {
		t.Errorf("threw %q, want %q", got, want)
	}
}

func TestGuavaImmutableList(t *testing.T) {
	l := ImmutableList_of_Object_Object_Object("a", "b", "a")
	guavaStr_(t, l, "[a, b, a]")
	guavaEq_(t, l.size(), int32(3))
	guavaEq_(t, l.get(1), "b")
	guavaEq_(t, l.indexOf("a"), int32(0))
	guavaEq_(t, l.lastIndexOf("a"), int32(2))
	guavaEq_(t, l.indexOf(nil), int32(-1))
	guavaEq_(t, l.contains("b"), true)
	guavaEq_(t, l.contains("z"), false)
	guavaStr_(t, l.reverse(), "[a, b, a]")
	guavaStr_(t, ImmutableList_of_Object_Object("x", "y").reverse(), "[y, x]")
	guavaStr_(t, l.subList(1, 3), "[b, a]")

	// List contract: equal to an ArrayList with the same elements, both ways, same hash.
	al := listOf_([]any{"a", "b", "a"})
	if !l.equals(al) || !al.equals(l) {
		t.Error("ImmutableList should equal ArrayList")
	}
	guavaEq_(t, l.hashCode(), al.hashCode())
	if l.equals(ImmutableSet_of_Object_Object("a", "b")) || l.equals(listOf_([]any{"a", "b"})) {
		t.Error("unexpected equality")
	}

	guavaThrew_(t, "UnsupportedOperationException", func() { l.add("c") })
	guavaThrew_(t, "UnsupportedOperationException", func() { l.remove(0) })
	guavaThrew_(t, "UnsupportedOperationException", func() { l.set(0, "c") })
	guavaThrew_(t, "UnsupportedOperationException", func() { l.clear() })
	guavaThrew_(t, "UnsupportedOperationException", func() { it := l.iterator(); it.next(); Iterator_remove(it) })
	guavaThrew_(t, "IndexOutOfBoundsException", func() { l.get(3) })
	guavaThrew_(t, "NullPointerException", func() { ImmutableList_of_Object_Object("a", nil) })
	guavaThrew_(t, "NullPointerException", func() { ImmutableList_copyOf(listOf_([]any{"a", nil})) })
	guavaThrew_(t, "NullPointerException", func() { ImmutableList_builder().add(nil) })

	// copyOf is a snapshot.
	src := listOf_([]any{int32(1), int32(2)})
	c := ImmutableList_copyOf(src)
	src.add(int32(3))
	guavaStr_(t, c, "[1, 2]")
	if ImmutableList_copyOf(c) != c {
		t.Error("copyOf(ImmutableList) should return the same instance")
	}
	guavaStr_(t, ImmutableList_sortedCopyOf(naturalOrder_, listOf_([]any{"b", "c", "a"})), "[a, b, c]")
	guavaStr_(t, ImmutableList_of_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_ObjectArr(
		int32(1), int32(2), int32(3), int32(4), int32(5), int32(6), int32(7), int32(8), int32(9), int32(10), int32(11), int32(12), []any{int32(13)}),
		"[1, 2, 3, 4, 5, 6, 7, 8, 9, 10, 11, 12, 13]")
	guavaStr_(t, Lists_charactersOf("ab"), "[a, b]")

	// The call shape of translated code: the builder methods are declared on the base builder.
	b := ImmutableList_builder()
	built := cast_[*ImmutableList](obj_(cast_[*ImmutableList_Builder](obj_(b.add("x"))).addAll(Iterable(al)).add_ObjectArr([]any{"y"}).build()))
	guavaStr_(t, built, "[x, a, b, a, y]")
	var asList List = cast_[List](obj_(built))
	guavaEq_(t, asList.get(4), "y")
	// The builder can go on after build().
	b.add("z")
	guavaStr_(t, b.build(), "[x, a, b, a, y, z]")
	guavaEq_(t, built.size(), int32(5))
}

func TestGuavaImmutableSet(t *testing.T) {
	s := ImmutableSet_of_Object_Object_Object_Object("b", "a", "b", "c")
	guavaStr_(t, s, "[b, a, c]")
	guavaEq_(t, s.size(), int32(3))
	guavaEq_(t, s.contains("a"), true)
	guavaEq_(t, s.contains("z"), false)
	guavaEq_(t, s.contains(nil), false)
	guavaStr_(t, s.asList(), "[b, a, c]")
	guavaEq_(t, s.asList().get(2), "c")

	hs := new_LinkedHashSet()
	hs.add("c")
	hs.add("b")
	hs.add("a")
	if !s.equals(hs) || !hs.equals(s) {
		t.Error("ImmutableSet should equal a HashSet with the same elements")
	}
	guavaEq_(t, s.hashCode(), hs.hashCode())
	if s.equals(ImmutableList_of_Object_Object_Object("b", "a", "c")) {
		t.Error("a set is not equal to a list")
	}

	guavaThrew_(t, "UnsupportedOperationException", func() { s.add("q") })
	guavaThrew_(t, "UnsupportedOperationException", func() { s.remove_Object("a") })
	guavaThrew_(t, "NullPointerException", func() { ImmutableSet_of_Object_Object("a", nil) })

	built := cast_[*ImmutableSet](obj_(cast_[*ImmutableSet_Builder](obj_(ImmutableSet_builder().add("p"))).addAll(Iterable(hs)).add("p").build()))
	guavaStr_(t, built, "[p, c, b, a]")
	guavaStr_(t, ImmutableSet_copyOf(listOf_([]any{int32(3), int32(1), int32(3)})), "[3, 1]")
	guavaStr_(t, ImmutableSet_of_Object_Object_Object_Object_Object_Object_ObjectArr("a", "b", "c", "d", "e", "f", []any{"a", "g"}), "[a, b, c, d, e, f, g]")
	guavaEq_(t, ImmutableSet_of().isEmpty(), true)

	// Elements with equals/hashCode.
	k := ImmutableSet_of_Object_Object(ImmutableList_of_Object("x"), ImmutableList_of_Object("x"))
	guavaEq_(t, k.size(), int32(1))
	guavaEq_(t, k.contains(listOf_([]any{"x"})), true)
}

func TestGuavaImmutableSortedSet(t *testing.T) {
	s := cast_[*ImmutableSortedSet](obj_(ImmutableSortedSet_naturalOrder().add("c").add("a").add("b").add("a").build()))
	guavaStr_(t, s, "[a, b, c]")
	guavaEq_(t, s.contains("b"), true)
	guavaEq_(t, s.contains("bb"), false)
	guavaEq_(t, s.contains(int32(1)), false) // ClassCastException inside contains is "false"
	guavaEq_(t, s.first(), "a")
	guavaEq_(t, s.last(), "c")
	guavaStr_(t, s.descendingSet(), "[c, b, a]")
	guavaStr_(t, s.tailSet("b", true), "[b, c]")
	guavaStr_(t, s.tailSet("b", false), "[c]")
	guavaStr_(t, s.headSet("b", true), "[a, b]")
	guavaStr_(t, ImmutableSortedSet_copyOf(listOf_([]any{int32(3), int32(1), int32(2), int32(1)})), "[1, 2, 3]")
	guavaStr_(t, ImmutableSortedSet_copyOf_Comparator_Collection(Comparator_reversed(naturalOrder_), listOf_([]any{int32(3), int32(1), int32(2)})), "[3, 2, 1]")
	ts := new_TreeSet_Comparator(Comparator_reversed(naturalOrder_))
	ts.add("a")
	ts.add("b")
	cs := ImmutableSortedSet_copyOfSorted(ts)
	guavaStr_(t, cs, "[b, a]")
	guavaEq_(t, cs.contains("a"), true)
	guavaEq_(t, ImmutableSortedSet_of().isEmpty(), true)
	if !s.equals(ImmutableSet_of_Object_Object_Object("c", "b", "a")) {
		t.Error("sorted set should equal a set with the same elements")
	}
}

func TestGuavaImmutableMultiset(t *testing.T) {
	b := ImmutableMultiset_builder()
	b.add("a")
	b.add("b")
	b.add("a")
	ms := cast_[*ImmutableMultiset](obj_(b.build()))
	guavaStr_(t, ms, "[a x 2, b]")
	guavaEq_(t, ms.size(), int32(3))
	guavaEq_(t, ms.count("a"), int32(2))
	guavaEq_(t, ms.count("z"), int32(0))
	guavaStr_(t, ms.elementSet(), "[a, b]")
	guavaStr_(t, ms.entrySet(), "[a x 2, b]")
	guavaStr_(t, listOf_(ms.toArray_0()), "[a, a, b]")
	e := ms.entrySet().iterator().next().(Multiset_Entry)
	guavaEq_(t, e.getElement(), "a")
	guavaEq_(t, e.getCount(), int32(2))
	lm := LinkedHashMultiset_create()
	lm.add("b")
	lm.add_Object_int("a", 2)
	if !ms.equals(lm) || !lm.equals(ms) {
		t.Error("multisets with the same counts should be equal")
	}
	guavaEq_(t, ms.hashCode(), lm.hashCode())
	guavaThrew_(t, "UnsupportedOperationException", func() { ms.add_Object_int("a", 1) })
	guavaEq_(t, ImmutableMultiset_of().isEmpty(), true)
}

func TestGuavaImmutableMap(t *testing.T) {
	m := ImmutableMap_of_Object_Object_Object_Object_Object_Object("b", int32(1), "a", int32(2), "c", int32(3))
	guavaStr_(t, m, "{b=1, a=2, c=3}")
	guavaEq_(t, m.get("a"), int32(2))
	guavaEq_(t, m.get("z"), nil)
	guavaEq_(t, m.get(nil), nil)
	guavaEq_(t, m.containsKey("c"), true)
	guavaEq_(t, m.containsValue(int32(3)), true)
	guavaEq_(t, m.containsValue(int32(4)), false)
	guavaStr_(t, m.keySet(), "[b, a, c]")
	guavaStr_(t, m.values(), "[1, 2, 3]")
	guavaStr_(t, m.entrySet(), "[b=1, a=2, c=3]")
	if _, ok := any(m.keySet()).(*ImmutableSet); !ok {
		t.Error("keySet should be an ImmutableSet")
	}
	guavaEq_(t, m.entrySet().contains(&immEntry_{"a", int32(2)}), true)
	guavaEq_(t, m.entrySet().contains(&immEntry_{"a", int32(3)}), false)

	lm := new_LinkedHashMap()
	lm.put("a", int32(2))
	lm.put("b", int32(1))
	lm.put("c", int32(3))
	if !m.equals(lm) || !lm.equals(m) {
		t.Error("ImmutableMap should equal a HashMap with the same entries")
	}
	guavaEq_(t, m.hashCode(), lm.hashCode())
	guavaStr_(t, ImmutableMap_copyOf(lm), "{a=2, b=1, c=3}")
	if ImmutableMap_copyOf(m) != m {
		t.Error("copyOf(ImmutableMap) should return the same instance")
	}

	guavaThrew_(t, "UnsupportedOperationException", func() { m.put("x", int32(1)) })
	guavaThrew_(t, "UnsupportedOperationException", func() { m.remove("a") })
	guavaThrew_(t, "UnsupportedOperationException", func() { m.clear() })
	guavaThrew_(t, "UnsupportedOperationException", func() { m.entrySet().iterator().next().(Map_Entry).setValue(int32(9)) })
	guavaThrew_(t, "NullPointerException", func() { ImmutableMap_of_Object_Object("a", nil) })
	guavaThrew_(t, "NullPointerException", func() { ImmutableMap_builder().put(nil, "a") })
	guavaThrew_(t, "IllegalArgumentException", func() { ImmutableMap_of_Object_Object_Object_Object_Object_Object("a", "1", "b", "2", "a", "3") })

	b := ImmutableMap_builder().put("a", int32(1)).put("b", int32(2)).put("a", int32(3)).putAll(ImmutableMap_of_Object_Object("c", int32(4)))
	func() {
		defer func() {
			r := recover()
			if r == nil {
				t.Fatal("buildOrThrow should throw")
			}
			msg := r.(interface{ getMessage() string }).getMessage()
			if msg != "Multiple entries with same key: a=1 and a=3" {
				t.Errorf("message %q", msg)
			}
		}()
		b.buildOrThrow()
	}()
	guavaStr_(t, b.buildKeepingLast(), "{b=2, a=3, c=4}")
	guavaEq_(t, ImmutableMap_of().isEmpty(), true)
	guavaEq_(t, ImmutableMap_of().get("a"), nil)
}

func TestGuavaImmutableSortedMapAndBiMap(t *testing.T) {
	sm := cast_[*ImmutableSortedMap](obj_(ImmutableSortedMap_naturalOrder().put("b", int32(1)).put("a", int32(2)).buildOrThrow()))
	guavaStr_(t, sm, "{a=2, b=1}")
	guavaEq_(t, sm.get("b"), int32(1))
	guavaEq_(t, sm.get("zz"), nil)
	guavaEq_(t, sm.firstKey(), "a")
	guavaStr_(t, sm.keySet(), "[a, b]")
	guavaThrew_(t, "IllegalArgumentException", func() { ImmutableSortedMap_naturalOrder().put("a", "1").put("a", "2").buildOrThrow() })
	lm := new_LinkedHashMap()
	lm.put(int32(3), "c")
	lm.put(int32(1), "a")
	guavaStr_(t, ImmutableSortedMap_copyOf(lm), "{1=a, 3=c}")

	bm := ImmutableBiMap_copyOf(lm)
	guavaStr_(t, bm, "{3=c, 1=a}")
	guavaStr_(t, bm.inverse(), "{c=3, a=1}")
	guavaEq_(t, bm.inverse().get("a"), int32(1))
	if bm.inverse().inverse() != BiMap(bm) {
		t.Error("inverse of inverse should be the original")
	}
	lm.put(int32(4), "a")
	guavaThrew_(t, "IllegalArgumentException", func() { ImmutableBiMap_copyOf(lm) })
	guavaEq_(t, ImmutableBiMap_of().inverse().isEmpty(), true)
}

func TestGuavaImmutableMultimap(t *testing.T) {
	b := ImmutableListMultimap_builder()
	b.put("b", int32(1)).put("a", int32(2)).put("b", int32(1)).putAll("c", listOf_([]any{int32(5), int32(6)}))
	lm := cast_[*ImmutableListMultimap](obj_(b.build()))
	guavaStr_(t, lm, "{b=[1, 1], a=[2], c=[5, 6]}")
	guavaEq_(t, lm.size(), int32(5))
	guavaStr_(t, lm.get("b"), "[1, 1]")
	guavaStr_(t, lm.get("zz"), "[]")
	if _, ok := lm.get("b").(*ImmutableList); !ok {
		t.Error("get should return an ImmutableList")
	}
	guavaStr_(t, lm.keySet(), "[b, a, c]")
	guavaStr_(t, lm.values(), "[1, 1, 2, 5, 6]")
	guavaStr_(t, lm.entries(), "[b=1, b=1, a=2, c=5, c=6]")
	guavaStr_(t, lm.asMap(), "{b=[1, 1], a=[2], c=[5, 6]}")
	guavaStr_(t, lm.inverse(), "{1=[b, b], 2=[a], 5=[c], 6=[c]}")
	guavaEq_(t, lm.containsEntry("a", int32(2)), true)
	guavaEq_(t, lm.containsEntry("a", int32(3)), false)
	guavaThrew_(t, "UnsupportedOperationException", func() { lm.put("a", int32(1)) })
	guavaThrew_(t, "UnsupportedOperationException", func() { lm.get("a").add(int32(1)) })
	guavaThrew_(t, "NullPointerException", func() { ImmutableMultimap_builder().put("a", nil) })

	sb := ImmutableSetMultimap_builder()
	sb.put("b", int32(1)).put("a", int32(2)).put("b", int32(1)).put("b", int32(0))
	sm := cast_[*ImmutableSetMultimap](obj_(sb.orderKeysBy(naturalOrder_).build()))
	guavaStr_(t, sm, "{a=[2], b=[1, 0]}")
	guavaEq_(t, sm.size(), int32(3))

	al := ArrayListMultimap_create()
	al.put("x", "1")
	al.put("x", "1")
	al.put("y", "2")
	guavaStr_(t, ImmutableSetMultimap_copyOf(al), "{x=[1], y=[2]}")
	if !ImmutableListMultimap_copyOf(al).equals(al) || !al.equals(ImmutableListMultimap_copyOf(al)) {
		t.Error("list multimaps with the same content should be equal")
	}

	idx := Multimaps_index(listOf_([]any{"apple", "avocado", "banana"}), &Function_base_fn{func(v any) any { return v.(string)[:1] }})
	guavaStr_(t, idx, "{a=[apple, avocado], b=[banana]}")
}

func TestGuavaMultimapLiveViews(t *testing.T) {
	mm := ArrayListMultimap_create()
	var m Multimap = cast_[Multimap](obj_(mm))
	v := m.get("k")
	guavaEq_(t, v.isEmpty(), true)
	guavaEq_(t, m.containsKey("k"), false)
	v.add(int32(1)) // adding to the view creates the key
	v.add(int32(1))
	guavaEq_(t, m.containsKey("k"), true)
	guavaEq_(t, m.size(), int32(2))
	m.put("k", int32(2))
	guavaStr_(t, v, "[1, 1, 2]")
	l := cast_[List](v)
	guavaEq_(t, l.get(2), int32(2))
	guavaEq_(t, l.remove(0), int32(1))
	guavaEq_(t, m.size(), int32(2))
	m.put("j", int32(7))
	guavaStr_(t, m, "{k=[1, 2], j=[7]}")
	guavaStr_(t, m.keySet(), "[k, j]")
	guavaStr_(t, m.values(), "[1, 2, 7]")
	guavaStr_(t, mm.entries(), "[k=1, k=2, j=7]")
	guavaStr_(t, mm.keys(), "[k x 2, j]")
	guavaStr_(t, m.asMap(), "{k=[1, 2], j=[7]}")
	guavaEq_(t, m.asMap().get("nope"), nil)
	guavaEq_(t, m.containsEntry("k", int32(2)), true)
	guavaEq_(t, m.remove("k", int32(9)), false)

	// Removing the last value through an iterator removes the key.
	it := m.get("j").iterator()
	it.next()
	Iterator_remove(it)
	guavaEq_(t, m.containsKey("j"), false)
	guavaEq_(t, m.size(), int32(2))

	// asMap views are live as well.
	m.asMap().get("k").(Collection).add(int32(3))
	guavaStr_(t, m.get("k"), "[1, 2, 3]")
	guavaStr_(t, m.removeAll("k"), "[1, 2, 3]")
	guavaEq_(t, m.isEmpty(), true)
	guavaStr_(t, m.removeAll("k"), "[]")
	guavaStr_(t, v, "[]")

	guavaEq_(t, m.putAll("a", listOf_([]any{"x", "y"})), true)
	other := LinkedHashMultimap_create()
	other.put("a", "x")
	other.put("b", "z")
	guavaEq_(t, m.putAll_Multimap(other), true)
	guavaStr_(t, m, "{a=[x, y, x], b=[z]}")
	m.keySet().remove_Object("a")
	guavaStr_(t, m, "{b=[z]}")
	m.clear()
	guavaEq_(t, m.size(), int32(0))
}

func TestGuavaSetAndSortedMultimaps(t *testing.T) {
	lh := LinkedHashMultimap_create()
	guavaEq_(t, lh.put("b", "y"), true)
	guavaEq_(t, lh.put("b", "x"), true)
	guavaEq_(t, lh.put("b", "y"), false)
	lh.put("a", "z")
	guavaStr_(t, lh, "{b=[y, x], a=[z]}")
	guavaEq_(t, lh.size(), int32(3))
	guavaEq_(t, lh.get("b").add("x"), false)
	guavaEq_(t, lh.get("c").add("w"), true)
	guavaStr_(t, lh.keySet(), "[b, a, c]")
	guavaEq_(t, lh.remove("b", "y"), true)
	guavaStr_(t, lh.values(), "[x, z, w]")
	hs := new_LinkedHashSet()
	hs.add("x")
	guavaEq_(t, objequals_(lh.get("b"), hs), true)
	guavaEq_(t, objequals_(lh.get("b"), listOf_([]any{"x"})), false)

	tm := TreeMultimap_create()
	tm.put("b", int32(2))
	tm.put("a", int32(9))
	tm.put("b", int32(1))
	tm.put("b", int32(2))
	guavaStr_(t, tm, "{a=[9], b=[1, 2]}")
	guavaStr_(t, tm.values(), "[9, 1, 2]")

	// LinkedListMultimap: values() and entries() follow global insertion order.
	ll := LinkedListMultimap_create()
	ll.put("a", int32(1))
	ll.put("b", int32(2))
	ll.put("a", int32(3))
	guavaStr_(t, ll.values(), "[1, 2, 3]")
	guavaStr_(t, ll.entries(), "[a=1, b=2, a=3]")
	guavaStr_(t, ll, "{a=[1, 3], b=[2]}")
	guavaStr_(t, ll.get("a"), "[1, 3]")
	ll.remove("a", int32(1))
	guavaStr_(t, ll.values(), "[2, 3]")
	ll.get("b").clear()
	guavaStr_(t, ll.values(), "[3]")
	guavaStr_(t, ll.keySet(), "[a]")
	ab := ArrayListMultimap_create()
	ab.put("a", int32(1))
	ab.put("b", int32(2))
	ab.put("a", int32(3))
	guavaStr_(t, ab.values(), "[1, 3, 2]")

	lb := MultimapBuilder_linkedHashKeys().arrayListValues().build()
	lb.put("k", "v")
	lb.put("k", "v")
	guavaStr_(t, lb, "{k=[v, v]}")
	sb := MultimapBuilder_linkedHashKeys().linkedHashSetValues().build()
	sb.put("k", "v")
	sb.put("k", "v")
	guavaStr_(t, sb, "{k=[v]}")
	guavaStr_(t, Multimaps_asMap(cast_[ListMultimap](lb)), "{k=[v, v]}")
}

func TestGuavaMultiset(t *testing.T) {
	ms := LinkedHashMultiset_create()
	guavaEq_(t, ms.add("b"), true)
	guavaEq_(t, ms.add_Object_int("a", 3), int32(0))
	guavaEq_(t, ms.add_Object_int("a", 1), int32(3))
	guavaStr_(t, ms, "[b, a x 4]")
	guavaEq_(t, ms.size(), int32(5))
	guavaEq_(t, ms.count("a"), int32(4))
	guavaEq_(t, ms.count("q"), int32(0))
	guavaEq_(t, ms.contains("b"), true)
	guavaEq_(t, ms.remove_Object("a"), true)
	guavaEq_(t, ms.count("a"), int32(3))
	guavaEq_(t, ms.setCount("a", 5, 1), false)
	guavaEq_(t, ms.setCount("a", 3, 1), true)
	guavaStr_(t, ms.elementSet(), "[b, a]")
	guavaStr_(t, ms.entrySet(), "[b, a]")
	guavaEq_(t, ms.remove_Object("b"), true)
	guavaEq_(t, ms.remove_Object("b"), false)
	guavaStr_(t, ms, "[a]")
	guavaThrew_(t, "IllegalArgumentException", func() { ms.add_Object_int("a", -1) })

	tm := TreeMultiset_create()
	tm.add("c")
	tm.add("a")
	tm.add("c")
	guavaStr_(t, tm, "[a, c x 2]")
	guavaStr_(t, listOf_(tm.toArray_0()), "[a, c, c]")
	var n int
	for it := tm.iterator(); it.hasNext(); it.next() {
		n++
	}
	guavaEq_(t, n, 3)
}

func TestGuavaTable(t *testing.T) {
	tb := Tables_newCustomTable(new_LinkedHashMap(), &Supplier_base_fn{func() any { return new_LinkedHashMap() }})
	guavaEq_(t, tb.isEmpty(), true)
	guavaEq_(t, tb.put("r1", "c1", int32(1)), nil)
	guavaEq_(t, tb.put("r1", "c1", int32(2)), int32(1))
	tb.put("r2", "c1", int32(3))
	guavaEq_(t, tb.get("r1", "c1"), int32(2))
	guavaEq_(t, tb.get("r1", "zz"), nil)
	guavaEq_(t, tb.get("zz", "c1"), nil)
	guavaEq_(t, tb.contains("r2", "c1"), true)
	guavaEq_(t, tb.containsRow("r3"), false)
	guavaStr_(t, tb.rowKeySet(), "[r1, r2]")
	guavaStr_(t, tb.values(), "[2, 3]")
	guavaStr_(t, tb, "{r1={c1=2}, r2={c1=3}}")

	// row() is a live view, in both directions.
	row := tb.row("r3")
	guavaEq_(t, row.isEmpty(), true)
	row.put("c9", int32(9))
	guavaEq_(t, tb.get("r3", "c9"), int32(9))
	guavaEq_(t, tb.containsRow("r3"), true)
	tb.put("r3", "c8", int32(8))
	guavaStr_(t, row, "{c9=9, c8=8}")
	guavaStr_(t, row.keySet(), "[c9, c8]")
	guavaEq_(t, tb.remove("r3", "c9"), int32(9))
	guavaEq_(t, row.remove("c8"), int32(8))
	guavaEq_(t, tb.containsRow("r3"), false)
	guavaStr_(t, tb.rowKeySet(), "[r1, r2]")
	guavaEq_(t, tb.remove("r1", "nope"), nil)
	guavaThrew_(t, "NullPointerException", func() { tb.put("r", "c", nil) })
}

func TestGuavaHashBiMap(t *testing.T) {
	bm := HashBiMap_create()
	guavaEq_(t, bm.put("a", int32(1)), nil)
	bm.put("b", int32(2))
	guavaEq_(t, bm.get("a"), int32(1))
	inv := bm.inverse()
	guavaEq_(t, inv.get(int32(2)), "b")
	guavaStr_(t, inv, "{1=a, 2=b}")
	guavaThrew_(t, "IllegalArgumentException", func() { bm.put("c", int32(1)) })
	guavaEq_(t, bm.put("a", int32(5)), int32(1))
	guavaEq_(t, inv.containsKey(int32(1)), false)
	guavaEq_(t, inv.get(int32(5)), "a")
	inv.put(int32(7), "z") // the inverse is a live view
	guavaEq_(t, bm.get("z"), int32(7))
	guavaEq_(t, bm.remove("b"), int32(2))
	guavaEq_(t, inv.containsKey(int32(2)), false)
	guavaStr_(t, bm, "{a=5, z=7}")
	guavaStr_(t, bm.keySet(), "[a, z]")
	guavaStr_(t, bm.values(), "[5, 7]")
	if inv.inverse() != BiMap(bm) {
		t.Error("inverse of inverse")
	}
	lm := new_LinkedHashMap()
	lm.put("z", int32(7))
	lm.put("a", int32(5))
	guavaEq_(t, bm.equals(lm), true)
}

type guavaTestOrdering_ struct {
	Ordering
}

// compare orders strings by length (a translated subclass overrides compare like this).
func (this *guavaTestOrdering_) compare(a any, b any) int32 {
	return String_length(a.(string)) - String_length(b.(string))
}

func TestGuavaOrdering(t *testing.T) {
	nat := Ordering_natural()
	guavaStr_(t, nat.sortedCopy(listOf_([]any{"b", "c", "a"})), "[a, b, c]")
	guavaEq_(t, nat.compare(int32(1), int32(2)), int32(-1))
	guavaEq_(t, nat.reverse().compare(int32(1), int32(2)) > 0, true)
	nf := nat.nullsFirst()
	guavaEq_(t, nf.compare(nil, "a") < 0, true)
	guavaEq_(t, nf.compare("a", nil) > 0, true)
	guavaEq_(t, nf.compare(nil, nil), int32(0))
	guavaEq_(t, nf.compare("a", "b") < 0, true)
	guavaThrew_(t, "NullPointerException", func() { nat.compare(nil, "a") })

	sub := &guavaTestOrdering_{}
	sub.self_ = sub
	sub.init_Ordering()
	up := &sub.Ordering // what up_Ordering() hands to library code
	guavaEq_(t, up.compare("aaa", "b") > 0, true)
	// sortedCopy is stable.
	guavaStr_(t, up.sortedCopy(listOf_([]any{"ccc", "bb", "a", "aa"})), "[a, bb, aa, ccc]")
	guavaStr_(t, up.reverse().sortedCopy(listOf_([]any{"ccc", "bb", "a", "aa"})), "[ccc, bb, aa, a]")
	cmp := cast_[Comparator](up.reverse().self())
	guavaEq_(t, cmp.compare("a", "bb") > 0, true)
	guavaEq_(t, to_Ordering(up.self()) == up, true)
	guavaEq_(t, is_Ordering(any(sub)), true)

	guavaEq_(t, ComparisonChain_start().compare("a", "a").compare_int_int(2, 1).compare("a", "b").result(), int32(1))
	guavaEq_(t, ComparisonChain_start().compare("a", "b").compare_int_int(2, 1).result(), int32(-1))
	guavaEq_(t, ComparisonChain_start().compare("a", "a").compare_int_int(1, 1).result(), int32(0))
}

func TestGuavaRangeSet(t *testing.T) {
	r := Range_closed(int32(3), int32(5))
	guavaEq_(t, r.contains(int32(3)), true)
	guavaEq_(t, r.apply(int32(5)), true)
	guavaEq_(t, r.test(int32(6)), false)
	guavaStr_(t, r, "[3‥5]")
	guavaThrew_(t, "IllegalArgumentException", func() { Range_closed(int32(5), int32(3)) })
	guavaEq_(t, r.equals(Range_closed(int32(3), int32(5))), true)

	var rs RangeSet = cast_[RangeSet](obj_(TreeRangeSet_create()))
	guavaEq_(t, rs.isEmpty(), true)
	rs.add(Range_closed(int32(10), int32(20)))
	rs.add(Range_closed(int32(1), int32(3)))
	rs.add(Range_closed(int32(30), int32(40)))
	guavaStr_(t, rs, "[[1‥3], [10‥20], [30‥40]]")
	guavaEq_(t, rs.contains(int32(2)), true)
	guavaEq_(t, rs.contains(int32(4)), false)
	guavaEq_(t, rs.contains(int32(40)), true)
	guavaEq_(t, rs.contains(int32(41)), false)
	rs.add(Range_closed(int32(3), int32(12))) // connected ranges coalesce
	guavaStr_(t, rs, "[[1‥20], [30‥40]]")
	rs.add(Range_closed(int32(0), int32(50)))
	guavaStr_(t, rs, "[[0‥50]]")
	guavaEq_(t, rs.isEmpty(), false)
}

func TestGuavaIterables(t *testing.T) {
	a := listOf_([]any{int32(1), int32(2), int32(3)})
	b := listOf_([]any{int32(4)})
	even := &Predicate_fn{func(v any) bool { return v.(int32)%2 == 0 }}
	guavaStr_(t, Iterables_concat(a, b), "[1, 2, 3, 4]")
	guavaStr_(t, Iterables_concat_Iterable_Iterable_Iterable(b, a, b), "[4, 1, 2, 3, 4]")
	guavaStr_(t, Iterables_concat_Iterable(listOf_([]any{a, listOf_(nil), b})), "[1, 2, 3, 4]")
	guavaStr_(t, Iterables_filter(a, even), "[2]")
	tr := Iterables_transform(a, &Function_base_fn{func(v any) any { return v.(int32) * 10 }})
	guavaStr_(t, tr, "[10, 20, 30]")
	a.add(int32(4)) // views are lazy
	guavaStr_(t, tr, "[10, 20, 30, 40]")
	guavaStr_(t, Iterables_skip(a, 3), "[4]")
	guavaStr_(t, Iterables_skip(a, 9), "[]")
	guavaEq_(t, Iterables_size(tr), int32(4))
	guavaEq_(t, Iterables_isEmpty(Iterables_filter(b, &Predicate_fn{func(v any) bool { return false }})), true)
	guavaEq_(t, Iterables_contains(tr, int32(20)), true)
	guavaEq_(t, Iterables_get(tr, 1), int32(20))
	guavaEq_(t, Iterables_getFirst(a, nil), int32(1))
	guavaEq_(t, Iterables_getFirst(listOf_(nil), "dflt"), "dflt")
	guavaEq_(t, Iterables_getLast(tr), int32(40))
	guavaEq_(t, Iterables_getOnlyElement(b), int32(4))
	guavaEq_(t, Iterables_indexOf(a, even), int32(1))
	guavaEq_(t, Iterables_indexOf(b, &Predicate_fn{func(v any) bool { return false }}), int32(-1))
	guavaThrew_(t, "NoSuchElementException", func() { Iterables_getOnlyElement(listOf_(nil)) })
	guavaThrew_(t, "NoSuchElementException", func() { Iterables_getLast(Iterables_skip(a, 9)) })
	guavaThrew_(t, "IndexOutOfBoundsException", func() { Iterables_get(tr, 9) })
	func() {
		defer func() {
			msg := recover().(interface{ getMessage() string }).getMessage()
			if msg != "expected one element but was: <1, 2, 3, 4>" {
				t.Errorf("message %q", msg)
			}
		}()
		Iterables_getOnlyElement(a)
	}()
	dst := new_ArrayList_0()
	guavaEq_(t, Iterables_addAll(dst, tr), true)
	guavaStr_(t, dst, "[10, 20, 30, 40]")
	um := Iterables_unmodifiableIterable(dst)
	guavaThrew_(t, "UnsupportedOperationException", func() { it := um.iterator(); it.next(); Iterator_remove(it) })

	fi := Iterators_filter(a.iterator(), even)
	guavaStr_(t, Lists_newArrayList(fi), "[2, 4]")
	pk := Iterators_peekingIterator(a.iterator())
	guavaEq_(t, pk.peek(), int32(1))
	guavaEq_(t, pk.peek(), int32(1))
	guavaEq_(t, pk.next(), int32(1))
	guavaEq_(t, pk.next(), int32(2))
	guavaEq_(t, pk.hasNext(), true)

	in := Interners_newWeakInterner()
	x := ImmutableList_of_Object("q")
	guavaEq_(t, in.intern(x) == any(x), true)
	guavaEq_(t, in.intern(ImmutableList_of_Object("q")) == any(x), true)
}

func TestGuavaListsAndSets(t *testing.T) {
	a := listOf_([]any{"a", "b", "c"})
	rv := Lists_reverse(a)
	guavaStr_(t, rv, "[c, b, a]")
	a.add("d") // live view
	guavaStr_(t, rv, "[d, c, b, a]")
	guavaEq_(t, rv.get(0), "d")
	guavaEq_(t, objequals_(rv, listOf_([]any{"d", "c", "b", "a"})), true)
	guavaStr_(t, Lists_reverse(ImmutableList_of_Object_Object("x", "y")), "[y, x]")
	tr := Lists_transform(a, &Function_base_fn{func(v any) any { return strings.ToUpper(v.(string)) }})
	guavaStr_(t, tr, "[A, B, C, D]")
	guavaEq_(t, tr.size(), int32(4))
	guavaEq_(t, tr.get(1), "B")
	guavaEq_(t, tr.indexOf("C"), int32(2))
	guavaStr_(t, tr.subList(1, 3), "[B, C]")

	s1 := new_LinkedHashSet()
	s2 := new_LinkedHashSet()
	for _, v := range []any{"a", "b", "c"} {
		s1.add(v)
	}
	for _, v := range []any{"c", "d", "b"} {
		s2.add(v)
	}
	u := Sets_union(s1, s2)
	guavaStr_(t, u, "[a, b, c, d]")
	guavaEq_(t, u.size(), int32(4))
	guavaEq_(t, u.contains("d"), true)
	guavaStr_(t, Sets_intersection(s1, s2), "[b, c]")
	d := Sets_difference(s1, s2)
	guavaStr_(t, d, "[a]")
	s2.remove_Object("b") // views are live
	guavaStr_(t, d, "[a, b]")
	guavaEq_(t, d.isEmpty(), false)
	guavaStr_(t, d.immutableCopy(), "[a, b]")
	guavaEq_(t, d.equals(ImmutableSet_of_Object_Object("b", "a")), true)
	guavaThrew_(t, "UnsupportedOperationException", func() { d.add("x") })
	var asSet Set = cast_[Set](obj_(u))
	guavaEq_(t, asSet.contains("a"), true)
	guavaEq_(t, u.up_AbstractSet().toString(), "[a, b, c, d]")

	ids := Sets_newIdentityHashSet()
	x, y := ImmutableList_of_Object("q"), ImmutableList_of_Object("q")
	ids.add(x)
	ids.add(y)
	guavaEq_(t, ids.size(), int32(2))
	guavaEq_(t, Sets_newLinkedHashSetWithExpectedSize(4).isEmpty(), true)
	guavaEq_(t, Maps_newLinkedHashMapWithExpectedSize(4).isEmpty(), true)
}
