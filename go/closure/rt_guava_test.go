package closure

import (
	"testing"
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
	even := &Predicate_base_fn{func(v any) bool { return v.(int32)%2 == 0 }}
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
	guavaEq_(t, Iterables_isEmpty(Iterables_filter(b, &Predicate_base_fn{func(v any) bool { return false }})), true)
	guavaEq_(t, Iterables_contains(tr, int32(20)), true)
	guavaEq_(t, Iterables_get(tr, 1), int32(20))
	guavaEq_(t, Iterables_getFirst(a, nil), int32(1))
	guavaEq_(t, Iterables_getFirst(listOf_(nil), "dflt"), "dflt")
	guavaEq_(t, Iterables_getLast(tr), int32(40))
	guavaEq_(t, Iterables_getOnlyElement(b), int32(4))
	guavaEq_(t, Iterables_indexOf(a, even), int32(1))
	guavaEq_(t, Iterables_indexOf(b, &Predicate_base_fn{func(v any) bool { return false }}), int32(-1))
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
