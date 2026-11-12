package closure

import (
	"fmt"
	"strings"
	"testing"
)

func streamStrs_(a ...string) []any {
	r := make([]any, len(a))
	for i, s := range a {
		r[i] = s
	}
	return r
}

func streamInts_(a ...int32) []any {
	r := make([]any, len(a))
	for i, s := range a {
		r[i] = s
	}
	return r
}

func streamShow_(a []any) string {
	p := make([]string, len(a))
	for i, v := range a {
		p[i] = objstr_(v)
	}
	return "[" + strings.Join(p, ", ") + "]"
}

func streamWant_(t *testing.T, s any, want string) {
	t.Helper()
	if got := streamShow_(streamItems_(s)); got != want {
		t.Errorf("got %s, want %s", got, want)
	}
}

// streamSkipIfStub_ runs f and skips the test when it reaches a placeholder owned
// by another runtime area.
func streamSkipIfStub_(t *testing.T, f func()) {
	t.Helper()
	defer func() {
		if r := recover(); r != nil {
			if s, ok := r.(string); ok && strings.HasPrefix(s, "j2go stub") {
				t.Skip(s)
			}
			panic(r)
		}
	}()
	f()
}

// streamThrown_ returns the exception f throws, or nil.
func streamThrown_(f func()) (ex any) {
	defer func() { ex = recover() }()
	f()
	return nil
}

func TestStreamIntermediate(t *testing.T) {
	src := listOf_(streamInts_(5, 3, 8, 3, 1, 5, 9))
	streamWant_(t, Collection_stream(src), "[5, 3, 8, 3, 1, 5, 9]")
	streamWant_(t, Collection_stream(src).filter(&Predicate_function_fn{func(v any) bool { return v.(int32) > 3 }}), "[5, 8, 5, 9]")
	streamWant_(t, Collection_stream(src).map_(&Function_fn{func(v any) any { return v.(int32) * 2 }}), "[10, 6, 16, 6, 2, 10, 18]")
	streamWant_(t, Collection_stream(src).distinct(), "[5, 3, 8, 1, 9]")
	streamWant_(t, Collection_stream(src).sorted_0(), "[1, 3, 3, 5, 5, 8, 9]")
	streamWant_(t, Collection_stream(src).sorted(Comparator_reversed(Comparator_naturalOrder())), "[9, 8, 5, 5, 3, 3, 1]")
	streamWant_(t, Collection_stream(src).limit(3), "[5, 3, 8]")
	streamWant_(t, Collection_stream(src).limit(100), "[5, 3, 8, 3, 1, 5, 9]")
	streamWant_(t, Collection_stream(src).skip(5), "[5, 9]")
	streamWant_(t, Collection_stream(src).skip(7), "[]")
	streamWant_(t, Collection_stream(src).skip(2).limit(2), "[8, 3]")
	streamWant_(t, Collection_stream(src).flatMap(&Function_fn{func(v any) any {
		if v.(int32) == 3 {
			return nil
		}
		return Stream_of_ObjectArr([]any{v, v})
	}}).limit(4), "[5, 5, 8, 8]")
	// the source is untouched
	if got := src.toString(); got != "[5, 3, 8, 3, 1, 5, 9]" {
		t.Errorf("source changed: %s", got)
	}
	var seen []any
	Collection_stream(src).peek(&Consumer_fn{func(v any) { seen = append(seen, v) }}).forEachOrdered(&Consumer_fn{func(v any) { seen = append(seen, v) }})
	if len(seen) != 14 {
		t.Errorf("peek/forEach saw %d", len(seen))
	}
}

func TestStreamSortedStable(t *testing.T) {
	s := Stream_of_ObjectArr(streamStrs_("bb", "a", "cc", "d", "aa")).sorted(
		Comparator_comparingInt(&ToIntFunction_fn{func(v any) int32 { return String_length(v.(string)) }}))
	streamWant_(t, s, "[a, d, bb, cc, aa]")
}

func TestStreamTerminal(t *testing.T) {
	mk := func() Stream { return Stream_of_ObjectArr(streamInts_(4, 7, 10)) }
	even := &Predicate_function_fn{func(v any) bool { return v.(int32)%2 == 0 }}
	if mk().allMatch(even) || !mk().anyMatch(even) {
		t.Error("allMatch/anyMatch")
	}
	empty := Stream_of_ObjectArr(nil)
	if !empty.allMatch(even) || empty.anyMatch(even) {
		t.Error("empty allMatch/anyMatch")
	}
	sum := mk().reduce(any(int32(0)), &BinaryOperator_fn{func(a, b any) any { return a.(int32) + b.(int32) }})
	if sum != any(int32(21)) {
		t.Errorf("reduce = %v", sum)
	}
	if got := streamShow_(mk().toArray(&IntFunction_fn{func(n int32) any { return make([]any, n) }})); got != "[4, 7, 10]" {
		t.Errorf("toArray = %s", got)
	}
	if !mk().allMatch(Predicate_function_isEqual(any(int32(4)))) == false {
		t.Error("isEqual")
	}
	if !Predicate_function_isEqual(nil).test(nil) || Predicate_function_isEqual(nil).test("x") ||
		!Predicate_function_isEqual("x").test("x") {
		t.Error("isEqual null handling")
	}
	if Function_identity().apply("q") != any("q") {
		t.Error("identity")
	}
	if !Predicate_test(&Predicate_fn{func(v any) bool { return v == any("y") }}, "y") {
		t.Error("Predicate_test")
	}
}

func TestStreamFindFirst(t *testing.T) {
	streamSkipIfStub_(t, func() {
		if got := Stream_of_ObjectArr(streamStrs_("a", "b")).findFirst().get(); got != any("a") {
			t.Errorf("findFirst = %v", got)
		}
		if Stream_of_ObjectArr(nil).findFirst().isPresent() {
			t.Error("findFirst on empty")
		}
	})
}

func TestStreamSources(t *testing.T) {
	streamWant_(t, Stream_of("x"), "[x]")
	streamWant_(t, Stream_concat(Stream_of("x"), Stream_of_ObjectArr(streamStrs_("y", "z"))), "[x, y, z]")
	b := Stream_builder()
	Stream_Builder_add(Stream_Builder_add(b, "p"), "q")
	b.accept("r")
	streamWant_(t, b.build(), "[p, q, r]")
	if ex := streamThrown_(func() { b.accept("s") }); !is_IllegalStateException(ex) {
		t.Errorf("accept after build: %v", ex)
	}
	set := new_LinkedHashSet()
	set.add("m")
	set.add("k")
	streamWant_(t, Streams_stream(set), "[m, k]")
	streamWant_(t, Streams_mapWithIndex(Streams_stream(set), &Streams_FunctionWithIndex_fn{func(v any, i int64) any {
		return fmt.Sprintf("%s%d", v, i)
	}}), "[m0, k1]")
	streamWant_(t, IntStream_range(2, 5).mapToObj(&IntFunction_fn{func(i int32) any { return i * i }}), "[4, 9, 16]")
	streamWant_(t, IntStream_range(5, 2).mapToObj(&IntFunction_fn{func(i int32) any { return i }}), "[]")
}

func TestStreamCollectors(t *testing.T) {
	mk := func() Stream { return Stream_of_ObjectArr(streamStrs_("pear", "fig", "apple", "fig", "kiwi")) }
	l := mk().collect(Collectors_toList()).(*ArrayList)
	if l.toString() != "[pear, fig, apple, fig, kiwi]" {
		t.Errorf("toList = %s", l.toString())
	}
	l.add("more") // mutable
	s := mk().collect(Collectors_toSet()).(*LinkedHashSet)
	if s.toString() != "[pear, fig, apple, kiwi]" {
		t.Errorf("toSet = %s", s.toString())
	}
	ts := mk().collect(Collectors_toCollection(&Supplier_fn{func() any { return new_TreeSet() }})).(*TreeSet)
	if ts.toString() != "[apple, fig, kiwi, pear]" {
		t.Errorf("toCollection = %s", ts.toString())
	}
	for sep, want := range map[string]string{", ": "pear, fig, apple, fig, kiwi", "": "pearfigapplefigkiwi"} {
		if got := mk().collect(Collectors_joining(sep)); got != any(want) {
			t.Errorf("joining(%q) = %v", sep, got)
		}
	}
	if got := Stream_of_ObjectArr(nil).collect(Collectors_joining(",")); got != any("") {
		t.Errorf("joining empty = %v", got)
	}
	if got := Stream_of_ObjectArr([]any{"a", nil}).collect(Collectors_joining("-")); got != any("a-null") {
		t.Errorf("joining null = %v", got)
	}
	length := &Function_fn{func(v any) any { return String_length(v.(string)) }}
	g := mk().collect(Collectors_groupingBy(length)).(*LinkedHashMap)
	if g.toString() != "{4=[pear, kiwi], 3=[fig, fig], 5=[apple]}" {
		t.Errorf("groupingBy = %s", g.toString())
	}
	m := mk().distinct().collect(Collectors_toMap(Function_identity(), length)).(*LinkedHashMap)
	if m.toString() != "{pear=4, fig=3, apple=5, kiwi=4}" {
		t.Errorf("toMap = %s", m.toString())
	}
	ex := streamThrown_(func() { mk().collect(Collectors_toMap(Function_identity(), length)) })
	if !is_IllegalStateException(ex) {
		t.Fatalf("toMap duplicate: %v", ex)
	}
	if msg := to_Throwable(ex).getMessage(); msg != "Duplicate key fig (attempted merging values 3 and 3)" {
		t.Errorf("toMap duplicate message = %q", msg)
	}
	if ex := streamThrown_(func() { mk().collect(Collectors_groupingBy(&Function_fn{func(any) any { return nil }})) }); !is_NullPointerException(ex) {
		t.Errorf("groupingBy null key: %v", ex)
	}
}

func TestStreamImmutableCollectors(t *testing.T) {
	// Accumulation logic that does not depend on the Immutable* classes.
	length := &Function_fn{func(v any) any { return String_length(v.(string)) }}
	keys, values := streamEntries_(streamStrs_("bb", "a"), Function_identity(), length)
	if streamShow_(keys) != "[bb, a]" || streamShow_(values) != "[2, 1]" {
		t.Errorf("streamEntries_ = %s %s", streamShow_(keys), streamShow_(values))
	}
	if tm := streamSortedMap_(Comparator_naturalOrder(), keys, values); tm.toString() != "{a=1, bb=2}" {
		t.Errorf("streamSortedMap_ = %s", tm.toString())
	}
	ex := streamThrown_(func() { streamSortedMap_(Comparator_naturalOrder(), streamStrs_("a", "a"), streamInts_(1, 2)) })
	if !is_IllegalArgumentException(ex) {
		t.Errorf("duplicate sorted key: %v", ex)
	}
	if ex := streamThrown_(func() {
		streamEntries_(streamStrs_("a"), Function_identity(), &Function_fn{func(any) any { return nil }})
	}); !is_NullPointerException(ex) {
		t.Errorf("null value: %v", ex)
	}
	if ex := streamThrown_(func() { Stream_of_ObjectArr([]any{nil}).collect(ImmutableList_toImmutableList()) }); !is_NullPointerException(ex) {
		t.Errorf("null element: %v", ex)
	}
	for name, c := range map[string]Collector{
		"list":      ImmutableList_toImmutableList(),
		"set":       ImmutableSet_toImmutableSet(),
		"sortedSet": ImmutableSortedSet_toImmutableSortedSet(Comparator_naturalOrder()),
		"multiset":  ImmutableMultiset_toImmutableMultiset(),
		"map":       ImmutableMap_toImmutableMap(Function_identity(), length),
		"sortedMap": ImmutableSortedMap_toImmutableSortedMap(Comparator_naturalOrder(), Function_identity(), length),
	} {
		t.Run(name, func(t *testing.T) {
			streamSkipIfStub_(t, func() {
				r := Stream_of_ObjectArr(streamStrs_("bb", "a", "ccc")).collect(c)
				want := map[string]string{
					"list": "[bb, a, ccc]", "set": "[bb, a, ccc]", "sortedSet": "[a, bb, ccc]", "multiset": "[bb, a, ccc]",
					"map": "{bb=2, a=1, ccc=3}", "sortedMap": "{a=1, bb=2, ccc=3}",
				}[name]
				if got := objstr_(r); got != want {
					t.Errorf("got %s, want %s", got, want)
				}
			})
		})
	}
}
