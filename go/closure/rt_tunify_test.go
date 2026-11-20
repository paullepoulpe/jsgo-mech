package closure

// The typed collections after tools/postflip/unify (one family) behave as the translator's
// family did. Each scenario below is written once, against a table of functions (ufam_...);
// tools/postflip/unify/difftest/run.sh builds a package that has both families (the tree
// before the step, with this step's runtime beside it), runs the scenarios on each and
// compares the traces line by line; the trace of the translator's family, recorded by that
// run, is unifyGolden_ (rt_tunify_golden_test.go), which TestUnifiedFamily compares the one
// family with from then on.

import (
	"fmt"
	"math/rand"
	"strings"
	"testing"
)

// utr_ is the trace of a scenario: one line per observation.
type utr_ struct{ log []string }

func ufmt_(v any) string {
	switch x := v.(type) {
	case nil:
		return "<nil>"
	case string:
		if x == nullstr_ {
			return "<null string>"
		}
		return fmt.Sprintf("%q", x)
	case bool, int, int32:
		return fmt.Sprint(x)
	case []string:
		return fmt.Sprintf("%q", x)
	}
	if isNil_(v) {
		return "<nil>"
	}
	return goTypeName_(v) + objstr_(v)
}

// try logs what f returns, or the Java exception it throws (class and message).
func (t *utr_) try(name string, f func() any) {
	defer func() {
		if r := recover(); r != nil {
			e := catch_(r)
			t.log = append(t.log, name+" -> throws "+goTypeName_(e)+": "+objstr_(e))
		}
	}()
	t.log = append(t.log, name+" -> "+ufmt_(f()))
}

func (t *utr_) run(name string, f func()) { t.try(name, func() any { f(); return "ok" }) }

// ---- the functions of a family, for lists of strings

type ulistFns_[L comparable] struct {
	empty     func() L
	of        func(xs ...string) L
	imm       func(ic *ImmutableCollection) L
	copyOf    func(from Iterable) L
	copyOfArr func(a []string) L
	copyOfT   func(l L) L
	newList   func() L
	from      func(c Collection) L
	copyList  func(l L) L
	split     func(sp *Splitter, seq string) L
	builder   func() ulistBuilder_[L]
	builderN  func(n int32) ulistBuilder_[L]
	// the methods (the same names in both families)
	m func(l L) ulistM_
}

type ulistM_ interface {
	size() int32
	isEmpty() bool
	get(i int32) string
	set(i int32, v string) string
	add(v string) bool
	insert(i int32, v string)
	clear()
	removeAt(i int32) string
	asList() List
	asImm() *ImmutableCollection
	iterator() Iterator
}

type ulistBuilder_[L any] struct {
	add    func(x string)
	addAll func(from Iterable)
	build  func() L
}

func uerasedList_(xs ...any) *ArrayList { return listOf_(append([]any(nil), xs...)) }

func uListScenario_[L comparable](f ulistFns_[L]) []string {
	t := &utr_{}
	var zero L
	e := f.empty()
	t.try("empty: one object, twin the runtime's", func() any {
		return e == f.empty() && f.of() == e && f.copyOfArr(nil) == e && f.m(e).asImm() == ImmutableList_of() &&
			f.imm(ImmutableList_of()) == e && f.copyOf(uerasedList_()) == e && f.builder().build() == e
	})
	l := f.of("a", "b", "a")
	t.try("of", func() any { return f.m(l).asList() })
	t.try("of: asList is the twin", func() any { return f.m(l).asList() == List(f.m(l).asImm()) && f.m(l).asImm() == f.m(l).asImm() })
	t.try("of: back from the twin", func() any { return f.imm(f.m(l).asImm()) == l })
	t.try("of: copyOf of the twin", func() any { return f.copyOf(f.m(l).asImm()) == l && f.copyOfT(l) == l })
	t.try("of null", func() any { return f.m(f.of("a", nullstr_, "c")).asList() })
	t.try("imm nil", func() any { return f.imm(nil) == zero })
	er := ImmutableList_of_Object_Object("x", "y")
	t.try("imm of an erased list", func() any {
		tl := f.imm(er)
		return f.m(tl).asImm() == er && f.imm(er) == tl && f.m(tl).get(1) == "y" && f.m(tl).size() == 2
	})
	t.try("copyOf an erased immutable list", func() any { return f.m(f.copyOf(er)).asImm() == er })
	es := ImmutableSet_of_Object_Object("p", "q")
	t.try("copyOf an erased immutable set", func() any { return f.m(f.copyOf(es)).asList() })
	t.try("copyOf an erased list", func() any { return f.m(f.copyOf(uerasedList_("m", "n"))).asList() })
	t.try("copyOf an erased list with null", func() any { return f.m(f.copyOf(uerasedList_("m", nil))).asList() })
	t.try("copyOfArr", func() any {
		a := []string{"u", "v"}
		c := f.copyOfArr(a)
		a[0] = "changed"
		return f.m(c).asList()
	})
	t.try("copyOfArr with null", func() any { return f.m(f.copyOfArr([]string{nullstr_})).asList() })
	// a list that can change
	ml := f.newList()
	f.m(ml).add("1")
	f.m(ml).add("2")
	t.try("mutable: view", func() any { return f.m(ml).asList() })
	t.try("mutable: one view", func() any { return f.m(ml).asList() == f.m(ml).asList() })
	t.try("mutable: copyOf its view", func() any { c := f.copyOf(f.m(ml).asList()); return c != ml && f.m(c).asList() != nil })
	t.try("mutable: copyOfT", func() any {
		c := f.copyOfT(ml)
		f.m(ml).add("3")
		return f.m(c).asList()
	})
	f.m(ml).add(nullstr_)
	t.try("mutable with null: copyOfT", func() any { return f.m(f.copyOfT(ml)).asList() })
	t.try("mutable with null: copyOf its view", func() any { return f.m(f.copyOf(f.m(ml).asList())).asList() })
	t.try("from", func() any { return f.m(f.from(uerasedList_("k", nil, "k"))).asList() })
	t.try("copy", func() any {
		c := f.copyList(ml)
		f.m(c).add("own")
		return ufmt_(f.m(c).asList()) + " / " + ufmt_(f.m(ml).asList())
	})
	t.try("iterator with remove", func() any {
		c := f.copyList(ml)
		it := f.m(c).iterator()
		it.next()
		Iterator_remove(it)
		return ufmt_(it.next()) + ufmt_(f.m(c).asList())
	})
	// every change of an immutable list throws
	for _, imm := range []struct {
		name string
		l    L
	}{{"of", l}, {"empty", e}, {"erased", f.imm(er)}, {"split", f.split(Splitter_on(','), "a,b")}} {
		m := f.m(imm.l)
		t.run(imm.name+": add", func() { m.add("z") })
		t.run(imm.name+": set", func() { m.set(0, "z") })
		t.run(imm.name+": insert", func() { m.insert(0, "z") })
		t.run(imm.name+": clear", func() { m.clear() })
		t.run(imm.name+": removeAt", func() { m.removeAt(0) })
		t.run(imm.name+": erased add", func() { m.asList().add("z") })
		t.run(imm.name+": erased addAll", func() { m.asList().addAll(uerasedList_("z")) })
		t.run(imm.name+": erased removeAll", func() { m.asList().removeAll(uerasedList_("a")) })
		t.run(imm.name+": erased remove", func() { m.asList().remove_Object("a") })
		t.run(imm.name+": erased clear", func() { m.asList().clear() })
		t.try(imm.name+": still", func() any { return m.asList() })
	}
	// Splitter.splitToList: cannot be changed, is no ImmutableList
	sl := f.split(Splitter_on('.'), "x.y.z")
	t.try("split", func() any { return f.m(sl).asList() })
	t.try("split: one view, no twin", func() any {
		_, isImm := f.m(sl).asList().(*ImmutableCollection)
		return !isImm && f.m(sl).asList() == f.m(sl).asList()
	})
	t.try("split: asImm", func() any { return f.m(sl).asImm() })
	t.try("split: copyOfT is a copy", func() any { c := f.copyOfT(sl); return c != sl && f.m(c).asImm() != nil })
	t.try("split: copyOf its view is a copy", func() any { return f.m(f.copyOf(f.m(sl).asList())).asImm() })
	// builders
	b := f.builder()
	b.add("x")
	b.addAll(f.m(l).asList())
	b.addAll(uerasedList_("e1", "e2"))
	t.try("builder: null in a typed view", func() any { b.addAll(f.m(ml).asList()); return "added" })
	b2 := f.builderN(8)
	b2.add("x")
	b2.addAll(f.m(l).asList())
	built := b2.build()
	b2.add("later")
	t.try("builder: build shares, later adds are not seen", func() any {
		return ufmt_(f.m(built).asList()) + " / " + ufmt_(f.m(b2.build()).asList())
	})
	t.try("builder: add null", func() any { b2.add(nullstr_); return "added" })
	t.try("builder: addAll with null", func() any { b2.addAll(uerasedList_("a", nil)); return "added" })
	t.try("builder: after the failures", func() any { return f.m(b2.build()).asList() })
	t.try("builder: built list is immutable", func() any { return f.m(built).add("z") })
	return t.log
}

// ---- deques of strings (TDeque: the head at index 0)

type udequeFns_[D comparable] struct {
	newDeque  func() D
	newDequeN func(n int32) D
	from      func(c Collection) D
	copyDeque func(d D) D
	m         func(d D) udequeM_
	items     func(d D) []string // a for-each loop over the deque
}

type udequeM_ interface {
	push(v string)
	pop() string
	poll() string
	peek() string
	peekLast() string
	add(v string) bool
	size() int32
	isEmpty() bool
	asDeque() Deque
}

func uDequeScenario_[D comparable](f udequeFns_[D]) []string {
	t := &utr_{}
	d := f.newDeque()
	m := f.m(d)
	t.try("empty", func() any {
		return fmt.Sprint(m.isEmpty(), ufmt_(m.peek()), ufmt_(m.peekLast()), ufmt_(m.poll()), m.size())
	})
	t.try("empty: pop", func() any { return m.pop() })
	m.push("a")
	m.push("b")
	m.add("z")
	t.try("push, add", func() any { return fmt.Sprint(m.peek(), m.peekLast(), m.size(), f.items(d)) + ufmt_(m.asDeque()) })
	t.try("one view", func() any { return m.asDeque() == m.asDeque() && goTypeName_(m.asDeque()) == "ArrayDeque" })
	t.try("erased ops", func() any {
		v := m.asDeque()
		v.push("c")
		v.add("y")
		r := ufmt_(v.peek()) + ufmt_(v.pop()) + ufmt_(v.poll()) + fmt.Sprint(v.size(), v.contains("y"), v.remove_Object("y"))
		for it := v.descendingIterator(); it.hasNext(); {
			r += ufmt_(it.next())
		}
		return r + ufmt_(v) + fmt.Sprint(f.items(d))
	})
	t.try("pop, poll", func() any { return fmt.Sprint(m.pop(), m.poll(), ufmt_(m.poll()), m.isEmpty()) })
	t.try("from, copy, N", func() any {
		x := f.from(uerasedList_("h", "m", "t"))
		c := f.copyDeque(x)
		f.m(c).push("own")
		n := f.newDequeN(4)
		f.m(n).push("n")
		return fmt.Sprint(f.m(x).peek(), f.m(x).peekLast(), f.items(x), f.items(c), f.items(n))
	})
	return t.log
}

// ---- sets of K (strings; `any` elements compared by equals or by a comparator)

type usetFns_[K comparable, S comparable] struct {
	empty    func() S
	of       func(xs ...K) S
	imm      func(ic *ImmutableCollection) S
	copyOf   func(from Iterable) S
	copyOfT  func(s S) S
	newSet   func() S
	newSetN  func(n int32) S
	from     func(c Collection) S
	copySet  func(s S) S
	sorted   func() S
	sortedC  func(cmp Comparator) S
	builder  func() usetBuilder_[S]
	add      func(s S, k K) bool
	has      func(s S, k K) bool
	del      func(s S, k K) bool
	size     func(s S) int32
	isEmpty  func(s S) bool
	clear    func(s S)
	addAll   func(s, o S) bool
	keys     func(s S) []K // a for-each loop over the set
	asSet    func(s S) Set
	asImm    func(s S) *ImmutableCollection
	asSorted func(s S) NavigableSet
	iterator func(s S) Iterator
	box      func(k K) any
	listView func(xs ...K) List // the view of a typed list of the same element type
	nullK    K
}

type usetBuilder_[S any] struct {
	add    func(x any) // boxed
	addAll func(from Iterable)
	build  func() S
}

func uboxAllK_[K comparable, S comparable](f usetFns_[K, S], ks []K) []any {
	out := make([]any, len(ks))
	for i, k := range ks {
		out[i] = f.box(k)
	}
	return out
}

func ukeys_[K comparable, S comparable](f usetFns_[K, S], s S) string {
	return ufmt_(uerasedList_(uboxAllK_(f, f.keys(s))...))
}

// uSetScenario_: ks are distinct keys (at least 6), dup a key equal to ks[0] by the set's
// comparison (the same key for strings).
// natural: the keys have a natural ordering (the scenarios of new TreeSet<>() run).
func uSetScenario_[K comparable, S comparable](f usetFns_[K, S], ks []K, dup K, other any, cmp Comparator, natural bool) []string {
	t := &utr_{}
	var zero S
	e := f.empty()
	t.try("empty: one object, twin the runtime's", func() any {
		return e == f.empty() && f.of() == e && f.asImm(e) == ImmutableSet_of() && f.imm(ImmutableSet_of()) == e &&
			f.copyOf(uerasedList_()) == e && f.builder().build() == e && f.asSet(e) == Set(ImmutableSet_of())
	})
	s := f.of(ks[1], ks[0], ks[1], dup, ks[2])
	t.try("of: first-seen order", func() any { return f.asSet(s) })
	t.try("of: contents", func() any {
		return fmt.Sprint(f.size(s), f.has(s, ks[0]), f.has(s, dup), f.has(s, ks[3]), f.isEmpty(s), ukeys_(f, s))
	})
	t.try("of: twin", func() any {
		return f.asImm(s) == f.asImm(s) && f.asSet(s) == Set(f.asImm(s)) && f.imm(f.asImm(s)) == s &&
			f.copyOf(f.asImm(s)) == s && f.copyOfT(s) == s
	})
	t.try("of: the twin is an ImmutableSet", func() any {
		return ImmutableSet_copyOf(Collection(f.asImm(s))) == f.asImm(s) && f.asImm(s).contains(f.box(ks[0])) && !f.asImm(s).contains(other)
	})
	t.try("of null", func() any { return f.asSet(f.of(ks[0], f.nullK)) })
	t.try("imm nil", func() any { return f.imm(nil) == zero })
	es := immSet_(uboxAllK_(f, []K{ks[3], ks[4]}))
	t.try("imm of an erased set", func() any {
		ts := f.imm(es)
		return f.asImm(ts) == es && f.imm(es) == ts && f.has(ts, ks[3]) && !f.has(ts, ks[0]) && f.size(ts) == 2 && f.copyOf(es) == ts
	})
	t.try("imm of an erased list", func() any { return f.asSet(f.imm(ImmutableList_of_Object_Object("x", "y"))) })
	t.try("copyOf an erased list", func() any { return f.asSet(f.copyOf(uerasedList_(uboxAllK_(f, []K{ks[2], ks[1], ks[2]})...))) })
	t.try("copyOf an erased list with null", func() any { return f.asSet(f.copyOf(uerasedList_(f.box(ks[2]), nil))) })
	t.try("copyOf an erased immutable list", func() any {
		return f.asSet(f.copyOf(immList_(uboxAllK_(f, []K{ks[5], ks[4], ks[5]}))))
	})
	t.try("copyOf a typed list's view", func() any { return f.asSet(f.copyOf(f.listView(ks[4], ks[3], ks[4]))) })
	t.try("copyOf a typed list's view with null", func() any { return f.asSet(f.copyOf(f.listView(ks[4], f.nullK))) })
	// a set that can change
	ms := f.newSet()
	for _, k := range ks {
		f.add(ms, k)
	}
	t.try("mutable: add again", func() any { return fmt.Sprint(f.add(ms, dup), f.size(ms)) })
	f.del(ms, ks[1])
	t.try("mutable: view", func() any { return f.asSet(ms) })
	t.try("mutable: one view", func() any { return f.asSet(ms) == f.asSet(ms) })
	t.try("mutable: copyOfT", func() any {
		c := f.copyOfT(ms)
		f.add(ms, ks[1])
		return ufmt_(f.asSet(c)) + fmt.Sprint(c != ms, f.asImm(c) != nil)
	})
	t.try("mutable: copyOf its view", func() any { return f.asSet(f.copyOf(f.asSet(ms))) })
	t.try("mutable: copy", func() any {
		c := f.copySet(ms)
		f.del(c, ks[0])
		return ufmt_(f.asSet(c)) + " / " + ufmt_(f.asSet(ms))
	})
	t.try("mutable: N, from, addAll", func() any {
		n := f.newSetN(16)
		r := fmt.Sprint(f.addAll(n, ms), f.addAll(n, ms))
		return r + ufmt_(f.asSet(n)) + ufmt_(f.asSet(f.from(f.asSet(n))))
	})
	t.try("mutable: iterator with remove", func() any {
		c := f.copySet(ms)
		it := f.iterator(c)
		it.next()
		it.next()
		Iterator_remove(it)
		return ufmt_(it.next()) + ufmt_(f.asSet(c))
	})
	ns := f.newSet()
	f.add(ns, f.nullK)
	t.try("mutable with null: copyOfT", func() any { return f.asSet(f.copyOfT(ns)) })
	t.try("mutable with null: copyOf its view", func() any { return f.asSet(f.copyOf(f.asSet(ns))) })
	// every change of an immutable set throws
	for _, imm := range []struct {
		name string
		s    S
	}{{"of", s}, {"empty", e}, {"erased", f.imm(es)}} {
		x := imm.s
		t.run(imm.name+": add", func() { f.add(x, ks[5]) })
		t.run(imm.name+": add of an element", func() { f.add(x, ks[0]) })
		t.run(imm.name+": del", func() { f.del(x, ks[0]) })
		t.run(imm.name+": clear", func() { f.clear(x) })
		t.run(imm.name+": addAll", func() { f.addAll(x, ms) })
		t.run(imm.name+": erased add", func() { f.asSet(x).add(f.box(ks[5])) })
		t.run(imm.name+": erased remove", func() { f.asSet(x).remove_Object(f.box(ks[0])) })
		t.run(imm.name+": erased clear", func() { f.asSet(x).clear() })
		t.try(imm.name+": still", func() any { return f.asSet(x) })
		t.try(imm.name+": asSorted", func() any { return f.asSorted(x) })
	}
	t.try("mutable: asSorted", func() any { return f.asSorted(ms) })
	// builders
	b := f.builder()
	b.add(f.box(ks[2]))
	b.add(f.box(ks[2]))
	b.addAll(f.asSet(ms))
	b.addAll(f.asSet(s))
	b.addAll(uerasedList_(f.box(ks[5])))
	b.addAll(f.listView(ks[0], ks[4]))
	t.try("builder", func() any { return f.asSet(b.build()) })
	t.try("builder: add null", func() any { b.add(nil); return "added" })
	t.try("builder: addAll with null", func() any { b.addAll(uerasedList_(nil)); return "added" })
	t.try("builder: null in a typed view", func() any { b.addAll(f.asSet(ns)); return "added" })
	t.try("builder: after the failures", func() any { return f.asSet(b.build()) })
	// sorted sets: natural ordering, then the comparator
	for _, sc := range []struct {
		name string
		mk   func() S
		cmp  Comparator
	}{{"sorted", f.sorted, nil}, {"sortedC", func() S { return f.sortedC(cmp) }, cmp}} {
		if sc.cmp == nil && !natural {
			continue
		}
		ss := sc.mk()
		for _, i := range []int{3, 1, 4, 0, 5, 2, 3} {
			f.add(ss, ks[i])
		}
		n := sc.name
		t.try(n+": order", func() any { return f.asSet(ss) })
		t.try(n+": contents", func() any {
			return fmt.Sprint(f.size(ss), f.has(ss, ks[2]), f.has(ss, dup), f.add(ss, dup), f.del(ss, ks[4]), f.del(ss, ks[4]), f.has(ss, ks[4]), ukeys_(f, ss))
		})
		t.try(n+": view", func() any {
			v := f.asSet(ss)
			return v == f.asSet(ss) && v == Set(f.asSorted(ss)) && goTypeName_(v) == "TreeSet"
		})
		t.try(n+": erased contains of another class", func() any { return f.asSet(ss).contains(other) })
		t.try(n+": erased remove of another class", func() any { return f.asSet(ss).remove_Object(other) })
		t.try(n+": erased ops", func() any {
			v := f.asSet(ss)
			r := fmt.Sprint(v.contains(f.box(ks[0])), v.add(f.box(ks[4])), v.add(f.box(ks[4])), v.remove_Object(f.box(ks[0])), v.remove_Object(f.box(ks[0])),
				v.size(), v.isEmpty(), v.containsAll(uerasedList_(f.box(ks[1]), f.box(ks[2]))), v.addAll(uerasedList_(f.box(ks[0]), f.box(ks[1]))))
			return r + ufmt_(v) + ufmt_(uerasedList_(v.toArray_0()...)) + fmt.Sprint(objhash_(v), objequals_(v, new_LinkedHashSet_Collection(v)))
		})
		t.try(n+": descendingSet, tailSet", func() any {
			v := f.asSorted(ss)
			d := v.descendingSet()
			tl := v.tailSet(f.box(ks[2]), true)
			tx := v.tailSet(f.box(ks[2]), false)
			f.del(ss, ks[5])
			return ufmt_(d) + ufmt_(tl) + ufmt_(tx) + ufmt_(v)
		})
		t.try(n+": iterator with remove", func() any {
			it := f.asSet(ss).iterator()
			a := it.next()
			Iterator_remove(it)
			b := it.next()
			it2 := f.iterator(ss)
			it2.next()
			it2.next()
			Iterator_remove(it2)
			return ufmt_(a) + ufmt_(b) + ufmt_(it2.hasNext()) + ufmt_(f.asSet(ss))
		})
		t.try(n+": removeAll, clear", func() any {
			f.add(ss, ks[0])
			f.add(ss, ks[5])
			r := fmt.Sprint(f.asSet(ss).removeAll(uerasedList_(f.box(ks[5]), other)))
			r += ufmt_(f.asSet(ss))
			f.clear(ss)
			f.add(ss, ks[3])
			f.add(ss, ks[1])
			return r + ufmt_(f.asSet(ss)) + fmt.Sprint(f.size(ss))
		})
		t.try(n+": copyOfT is an ImmutableSet in the set's order", func() any {
			c := f.copyOfT(ss)
			return ufmt_(f.asSet(c)) + fmt.Sprint(c != ss, f.asImm(c).kind == icSet_)
		})
		t.try(n+": copy is a hash set in the set's order", func() any {
			c := f.copySet(ss)
			f.add(c, ks[0])
			return ufmt_(f.asSet(c)) + goTypeName_(f.asSet(c))
		})
		if sc.cmp != nil {
			t.try(n+": null", func() any { return fmt.Sprint(f.has(ss, f.nullK), f.add(ss, f.nullK)) + ufmt_(f.asSet(ss)) })
		}
		// an ImmutableSortedSet entering a typed declaration
		is := immSorted_(uboxAllK_(f, []K{ks[4], ks[0], ks[2]}), sc.cmp)
		t.try(n+": imm of an ImmutableSortedSet", func() any {
			ts := f.imm(is)
			return fmt.Sprint(f.imm(is) == ts, f.asImm(ts) == is, f.asSet(ts) == Set(is), f.asSorted(ts) == NavigableSet(is), f.has(ts, ks[0]), f.has(ts, dup),
				f.has(ts, ks[1]), f.size(ts)) + ukeys_(f, ts)
		})
		t.try(n+": the typed ImmutableSortedSet cannot change", func() any { return f.add(f.imm(is), ks[1]) })
		t.try(n+": copyOf / copyOfT of an ImmutableSortedSet", func() any {
			c, c2 := f.copyOf(is), f.copyOfT(f.imm(is))
			return ufmt_(f.asSet(c)) + ufmt_(f.asSet(c2)) + fmt.Sprint(f.asImm(c) != is, f.asImm(c2) != is, f.asImm(c).kind == icSet_, f.asImm(c2).kind == icSet_)
		})
	}
	// random operations on a hash set and on the two sorted ones, typed and through the view
	for _, sc := range []struct {
		name string
		mk   func() S
	}{{"hash", f.newSet}, {"sorted", f.sorted}, {"sortedC", func() S { return f.sortedC(cmp) }}} {
		if sc.name == "sorted" && !natural {
			continue
		}
		r := rand.New(rand.NewSource(7))
		rs := sc.mk()
		var sum []string
		for i := 0; i < 400; i++ {
			k := ks[r.Intn(len(ks))]
			switch r.Intn(9) {
			case 0, 1, 2:
				sum = append(sum, fmt.Sprint(f.add(rs, k)))
			case 3, 4:
				sum = append(sum, fmt.Sprint(f.del(rs, k)))
			case 5:
				sum = append(sum, fmt.Sprint(f.has(rs, k), f.size(rs)))
			case 6:
				sum = append(sum, fmt.Sprint(f.asSet(rs).add(f.box(k)), f.asSet(rs).contains(f.box(k))))
			case 7:
				sum = append(sum, fmt.Sprint(f.asSet(rs).remove_Object(f.box(k))))
			case 8:
				it := f.iterator(rs)
				if it.hasNext() {
					it.next()
					if it.hasNext() && r.Intn(2) == 0 {
						it.next()
					}
					Iterator_remove(it)
				}
				sum = append(sum, ukeys_(f, rs))
			}
			if i%40 == 39 {
				t.try(fmt.Sprintf("random %s %d", sc.name, i), func() any { return strings.Join(sum, " ") + ufmt_(f.asSet(rs)) })
				sum = nil
			}
		}
	}
	return t.log
}

// ---- maps of K to strings

type umapFns_[K comparable, M comparable, S comparable] struct {
	empty     func() M
	of1       func(k1 K, v1 string) M
	of3       func(k1 K, v1 string, k2 K, v2 string, k3 K, v3 string) M
	of4       func(k1 K, v1 string, k2 K, v2 string, k3 K, v3 string, k4 K, v4 string) M
	imm       func(im *ErasedImmutableMap) M
	copyOf    func(from Map) M
	copyOfT   func(m M) M
	newMap    func() M
	newMapN   func(n int32) M
	from      func(m Map) M
	copyMap   func(m M) M
	sorted    func() M
	builder   func() umapBuilder_[M]
	put       func(m M, k K, v string) string
	get       func(m M, k K) string
	getOr     func(m M, k K, def string) string
	has       func(m M, k K) bool
	del       func(m M, k K) string
	putIfAbs  func(m M, k K, v string) string
	compute   func(m M, k K, f Function) string
	putAll    func(m, o M)
	size      func(m M) int32
	isEmpty   func(m M) bool
	clear     func(m M)
	keys      func(m M) []K      // for-each over keySet()
	vals      func(m M) []string // for-each over values()
	entries   func(m M, each func(getKey func() K, getValue func() string, setValue func(string) string, asEntry func() Map_Entry))
	asMap     func(m M) Map
	asImm     func(m M) *ErasedImmutableMap
	keySetImm func(m M) S
	setAsImm  func(s S) *ImmutableCollection
	box       func(k K) any
	nullK     K
}

type umapBuilder_[M any] struct {
	put              func(k any, v string) // the key boxed
	putAll           func(from Map)
	build            func() M
	buildKeepingLast func() M
}

type ufn_ struct{ f func(any) any }

func (f *ufn_) apply(x any) any { return f.f(x) }

func uerasedMap_(kv ...any) *LinkedHashMap {
	m := new_LinkedHashMap()
	for i := 0; i < len(kv); i += 2 {
		m.put(kv[i], kv[i+1])
	}
	return m
}

func umapKV_[K comparable, M comparable, S comparable](f umapFns_[K, M, S], m M) string {
	ks := f.keys(m)
	out := make([]any, len(ks))
	for i, k := range ks {
		out[i] = f.box(k)
	}
	return ufmt_(uerasedList_(out...)) + fmt.Sprintf("%q", f.vals(m))
}

func uMapScenario_[K comparable, M comparable, S comparable](f umapFns_[K, M, S], ks []K, dup K, other any, natural bool) []string {
	t := &utr_{}
	var zero M
	e := f.empty()
	t.try("empty: one object, twin the runtime's", func() any {
		return e == f.empty() && f.asImm(e) == ImmutableMap_of() && f.imm(ImmutableMap_of()) == e && f.copyOf(uerasedMap_()) == e &&
			f.builder().build() == e && f.builder().buildKeepingLast() == e && f.asMap(e) == Map(ImmutableMap_of())
	})
	m := f.of3(ks[1], "one", ks[0], "zero", ks[2], "two")
	t.try("of3", func() any { return f.asMap(m) })
	t.try("of: contents", func() any {
		return fmt.Sprint(f.size(m), f.isEmpty(m), f.has(m, ks[0]), f.has(m, dup), f.has(m, ks[3]), f.get(m, ks[2]), ufmt_(f.get(m, ks[3])), f.getOr(m, ks[3], "def"),
			f.getOr(m, ks[0], "def")) + umapKV_(f, m)
	})
	t.try("of: twin", func() any {
		return f.asImm(m) == f.asImm(m) && f.asMap(m) == Map(f.asImm(m)) && f.imm(f.asImm(m)) == m && f.copyOf(f.asImm(m)) == m && f.copyOfT(m) == m &&
			ImmutableMap_copyOf(f.asMap(m)) == f.asImm(m)
	})
	t.try("of1, of4", func() any {
		return ufmt_(f.asMap(f.of1(ks[5], "five"))) + ufmt_(f.asMap(f.of4(ks[3], "3", ks[2], "2", ks[1], "1", ks[0], "0")))
	})
	t.try("of: duplicate key", func() any { return f.asMap(f.of3(ks[0], "a", ks[1], "b", dup, "c")) })
	t.try("of: duplicate key twice", func() any { return f.asMap(f.of4(ks[0], "a", ks[1], "b", dup, "c", ks[1], "d")) })
	t.try("of: null key", func() any { return f.asMap(f.of1(f.nullK, "v")) })
	t.try("of: null value", func() any { return f.asMap(f.of1(ks[0], nullstr_)) })
	t.try("imm nil", func() any { return f.imm(nil) == zero })
	em := ImmutableMap_of_Object_Object(f.box(ks[3]), "three")
	t.try("imm of an erased map", func() any {
		tm := f.imm(em)
		return f.asImm(tm) == em && f.imm(em) == tm && f.get(tm, ks[3]) == "three" && !f.has(tm, ks[0]) && f.copyOf(em) == tm
	})
	t.try("copyOf an erased map", func() any { return f.asMap(f.copyOf(uerasedMap_(f.box(ks[2]), "b", f.box(ks[1]), "a"))) })
	t.try("copyOf an erased map with a null key", func() any { return f.asMap(f.copyOf(uerasedMap_(nil, "b"))) })
	t.try("copyOf an erased map with a null value", func() any { return f.asMap(f.copyOf(uerasedMap_(f.box(ks[2]), nil))) })
	// keySet() of an immutable map
	t.try("keySetImm: one set, the twin's key set", func() any {
		k1 := f.of3(ks[1], "one", ks[0], "zero", ks[2], "two")
		s := f.keySetImm(k1) // before the map has its twin
		k2 := f.of1(ks[4], "four")
		tw := f.asImm(k2)
		s2 := f.keySetImm(k2) // after
		return fmt.Sprint(f.keySetImm(k1) == s, f.setAsImm(s) == f.asImm(k1).keySet().(*ImmutableCollection), f.keySetImm(k2) == s2,
			f.setAsImm(s2) == tw.keySet().(*ImmutableCollection)) + ufmt_(f.setAsImm(s)) + ufmt_(f.setAsImm(s2)) + ufmt_(f.setAsImm(f.keySetImm(f.imm(em)))) +
			fmt.Sprint(f.setAsImm(f.keySetImm(e)) == ImmutableSet_of(), f.setAsImm(f.keySetImm(f.imm(em))) == em.keySet().(*ImmutableCollection))
	})
	// a map that can change
	mm := f.newMap()
	for i, k := range ks {
		f.put(mm, k, fmt.Sprint("v", i))
	}
	t.try("mutable: put again, del", func() any {
		return fmt.Sprint(f.put(mm, dup, "again"), f.del(mm, ks[1]), ufmt_(f.del(mm, ks[1])), f.size(mm)) + umapKV_(f, mm)
	})
	t.try("mutable: view", func() any { return f.asMap(mm) == f.asMap(mm) && goTypeName_(f.asMap(mm)) == "LinkedHashMap" })
	t.try("mutable: putIfAbsent, computeIfAbsent", func() any {
		calls := 0
		fn := &ufn_{func(x any) any { calls++; return "made" }}
		none := &ufn_{func(x any) any { calls++; return nil }}
		f.put(mm, ks[2], nullstr_)
		r := fmt.Sprint(ufmt_(f.putIfAbs(mm, ks[1], "pia")), f.putIfAbs(mm, ks[1], "pia2"), ufmt_(f.putIfAbs(mm, ks[2], "wasnull")),
			f.compute(mm, ks[0], fn), ufmt_(f.compute(mm, ks[1], fn)))
		f.del(mm, ks[3])
		return r + fmt.Sprint(f.compute(mm, ks[3], fn), ufmt_(f.compute(mm, f.nullK, none)), f.has(mm, f.nullK), calls) + umapKV_(f, mm)
	})
	t.try("mutable: copyOfT, copyOf its view", func() any {
		c, c2 := f.copyOfT(mm), f.copyOf(f.asMap(mm))
		f.put(mm, ks[4], "changed")
		return ufmt_(f.asMap(c)) + ufmt_(f.asMap(c2)) + fmt.Sprint(c != mm, f.asImm(c) != nil)
	})
	t.try("mutable: copy, N, putAll, from", func() any {
		c := f.copyMap(mm)
		f.del(c, ks[0])
		n := f.newMapN(4)
		f.put(n, ks[5], "mine")
		f.putAll(n, c)
		return ufmt_(f.asMap(c)) + ufmt_(f.asMap(n)) + ufmt_(f.asMap(f.from(f.asMap(n)))) + ufmt_(f.asMap(mm))
	})
	t.try("mutable: entries", func() any {
		var out []string
		c := f.copyMap(mm)
		f.entries(c, func(getKey func() K, getValue func() string, setValue func(string) string, asEntry func() Map_Entry) {
			k := getKey()
			if k == ks[0] {
				out = append(out, "set:"+setValue("new0")+">"+getValue())
			}
			if k == ks[4] {
				f.put(c, ks[4], "under")
				out = append(out, "sees:"+getValue())
				f.del(c, ks[4])
				out = append(out, "keeps:"+getValue()+">"+ufmt_(setValue("gone"))+">"+getValue())
			}
			en := asEntry()
			out = append(out, ufmt_(en.getKey())+"="+ufmt_(en.getValue())+objstr_(en)+fmt.Sprint(objhash_(en) == (objhash_(en.getKey())^objhash_(en.getValue()))))
		})
		return strings.Join(out, " ") + ufmt_(f.asMap(c))
	})
	nm := f.newMap()
	f.put(nm, ks[0], nullstr_)
	t.try("mutable with a null value: copyOfT", func() any { return f.asMap(f.copyOfT(nm)) })
	t.try("mutable with a null value: copyOf its view", func() any { return f.asMap(f.copyOf(f.asMap(nm))) })
	// every change of an immutable map throws
	for _, imm := range []struct {
		name string
		m    M
	}{{"of", m}, {"empty", e}, {"erased", f.imm(em)}} {
		x := imm.m
		t.run(imm.name+": put", func() { f.put(x, ks[5], "z") })
		t.run(imm.name+": put of a key", func() { f.put(x, ks[0], "z") })
		t.run(imm.name+": del", func() { f.del(x, ks[0]) })
		t.run(imm.name+": clear", func() { f.clear(x) })
		t.run(imm.name+": putIfAbsent", func() { f.putIfAbs(x, ks[0], "z") })
		t.run(imm.name+": computeIfAbsent", func() { f.compute(x, ks[0], &ufn_{func(any) any { return "z" }}) })
		t.run(imm.name+": putAll", func() { f.putAll(x, mm) })
		t.run(imm.name+": putAll of nothing", func() { f.putAll(x, f.newMap()) })
		t.run(imm.name+": entry setValue", func() {
			f.entries(x, func(getKey func() K, getValue func() string, setValue func(string) string, asEntry func() Map_Entry) {
				setValue("z")
			})
		})
		t.run(imm.name+": erased put", func() { f.asMap(x).put(f.box(ks[5]), "z") })
		t.run(imm.name+": erased remove", func() { f.asMap(x).remove(f.box(ks[0])) })
		t.run(imm.name+": erased clear", func() { f.asMap(x).clear() })
		t.try(imm.name+": still", func() any { return f.asMap(x) })
	}
	t.try("mutable: keySetImm", func() any { return f.keySetImm(mm) == f.keySetImm(mm) })
	// builders
	b := f.builder()
	b.put(f.box(ks[2]), "b2")
	b.putAll(uerasedMap_(f.box(ks[1]), "b1"))
	b.putAll(f.asMap(f.of1(ks[0], "b0")))
	t.try("builder", func() any { return f.asMap(b.build()) })
	c := f.copyMap(mm)
	f.del(c, ks[0])
	f.del(c, ks[1])
	f.del(c, ks[2])
	b.putAll(f.asMap(c))
	t.try("builder: putAll of a typed map's view", func() any { return f.asMap(b.build()) })
	t.try("builder: put a null key", func() any { b.put(nil, "v"); return "put" })
	t.try("builder: put a null value", func() any { b.put(f.box(ks[0]), nullstr_); return "put" })
	t.try("builder: putAll with a null value", func() any { b.putAll(f.asMap(nm)); return "put" })
	t.try("builder: putAll with a null key", func() any { b.putAll(uerasedMap_(nil, "v")); return "put" })
	b.put(f.box(ks[1]), "second")
	b.put(f.box(dup), "third")
	b.put(f.box(ks[1]), "fourth")
	t.try("builder: duplicate keys", func() any { return f.asMap(b.build()) })
	t.try("builder: buildKeepingLast", func() any { return f.asMap(b.buildKeepingLast()) })
	if natural {
		// a sorted map
		sm := f.sorted()
		for _, i := range []int{3, 1, 4, 0, 5, 2} {
			f.put(sm, ks[i], fmt.Sprint("s", i))
		}
		t.try("sorted: order", func() any { return ufmt_(f.asMap(sm)) + umapKV_(f, sm) })
		t.try("sorted: contents", func() any {
			return fmt.Sprint(f.size(sm), f.get(sm, ks[2]), f.put(sm, ks[2], "S2"), f.del(sm, ks[4]), ufmt_(f.del(sm, ks[4])), f.has(sm, ks[4]), ufmt_(f.get(sm, ks[4])),
				f.getOr(sm, ks[4], "def"), ufmt_(f.putIfAbs(sm, ks[4], "back")), f.compute(sm, ks[0], &ufn_{func(any) any { return "no" }})) + umapKV_(f, sm)
		})
		t.try("sorted: view", func() any { v := f.asMap(sm); return v == f.asMap(sm) && goTypeName_(v) == "TreeMap" })
		t.try("sorted: erased lookups of another class", func() any { return f.asMap(sm).containsKey(other) })
		t.try("sorted: erased get of another class", func() any { return f.asMap(sm).get(other) })
		t.try("sorted: erased remove of another class", func() any { return f.asMap(sm).remove(other) })
		t.try("sorted: erased ops", func() any {
			v := f.asMap(sm)
			r := fmt.Sprint(ufmt_(v.get(f.box(ks[0]))), v.containsKey(f.box(ks[1])), ufmt_(v.put(f.box(ks[1]), "E1")), ufmt_(v.remove(f.box(ks[3]))), ufmt_(v.remove(f.box(ks[3]))),
				v.size(), v.containsValue("E1"))
			v.putAll(uerasedMap_(f.box(ks[3]), "E3", f.box(ks[0]), "E0"))
			return r + ufmt_(v) + objstr_(v.keySet()) + objstr_(v.values()) + objstr_(v.entrySet()) + fmt.Sprint(objhash_(v), objequals_(v, new_LinkedHashMap_Map(v)))
		})
		t.try("sorted: iterators with remove, entries", func() any {
			it := f.asMap(sm).keySet().iterator()
			it.next()
			it.next()
			Iterator_remove(it)
			a := it.next()
			var out []string
			f.entries(sm, func(getKey func() K, getValue func() string, setValue func(string) string, asEntry func() Map_Entry) {
				out = append(out, ufmt_(f.box(getKey()))+"="+setValue(getValue()+"!"))
			})
			return ufmt_(a) + strings.Join(out, " ") + ufmt_(f.asMap(sm))
		})
		t.try("sorted: copyOfT is an ImmutableMap in the map's order", func() any {
			c := f.copyOfT(sm)
			return ufmt_(f.asMap(c)) + fmt.Sprint(c != sm, f.asImm(c).kind == imMap_)
		})
		t.try("sorted: clear, copy", func() any {
			c := f.copyMap(sm)
			f.clear(sm)
			f.put(sm, ks[5], "x")
			f.put(sm, ks[0], "y")
			return ufmt_(f.asMap(sm)) + ufmt_(f.asMap(c)) + goTypeName_(f.asMap(c))
		})
		// an ImmutableSortedMap entering a typed declaration
		ism := ImmutableSortedMap_copyOf(uerasedMap_(f.box(ks[4]), "i4", f.box(ks[0]), "i0", f.box(ks[2]), "i2"))
		t.try("imm of an ImmutableSortedMap", func() any {
			tm := f.imm(ism)
			return fmt.Sprint(f.imm(ism) == tm, f.asImm(tm) == ism, f.asMap(tm) == Map(ism), f.get(tm, ks[0]), f.has(tm, ks[1]), f.size(tm)) + umapKV_(f, tm) +
				ufmt_(f.setAsImm(f.keySetImm(tm))) + fmt.Sprint(f.setAsImm(f.keySetImm(tm)) == ism.keySet().(*ImmutableCollection))
		})
		t.try("the typed ImmutableSortedMap cannot change", func() any { return f.put(f.imm(ism), ks[1], "z") })
		t.try("copyOf / copyOfT of an ImmutableSortedMap", func() any {
			c, c2 := f.copyOf(ism), f.copyOfT(f.imm(ism))
			return ufmt_(f.asMap(c)) + ufmt_(f.asMap(c2)) + fmt.Sprint(f.asImm(c) != ism, f.asImm(c2) != ism, f.asImm(c).kind == imMap_, f.asImm(c2).kind == imMap_)
		})
	}
	// random operations on a hash map and a sorted one
	for _, sc := range []struct {
		name string
		mk   func() M
	}{{"hash", f.newMap}, {"sorted", f.sorted}} {
		if sc.name == "sorted" && !natural {
			continue
		}
		r := rand.New(rand.NewSource(11))
		rm := sc.mk()
		var sum []string
		for i := 0; i < 400; i++ {
			k := ks[r.Intn(len(ks))]
			v := fmt.Sprint("r", i)
			switch r.Intn(9) {
			case 0, 1, 2:
				sum = append(sum, ufmt_(f.put(rm, k, v)))
			case 3, 4:
				sum = append(sum, ufmt_(f.del(rm, k)))
			case 5:
				sum = append(sum, fmt.Sprint(f.has(rm, k), ufmt_(f.get(rm, k)), f.size(rm)))
			case 6:
				sum = append(sum, ufmt_(f.asMap(rm).put(f.box(k), v))+ufmt_(f.asMap(rm).get(f.box(k))))
			case 7:
				sum = append(sum, ufmt_(f.asMap(rm).remove(f.box(k))))
			case 8:
				it := f.asMap(rm).entrySet().iterator()
				if it.hasNext() {
					it.next()
					if it.hasNext() && r.Intn(2) == 0 {
						it.next().(Map_Entry).setValue("set")
					}
					Iterator_remove(it)
				}
				sum = append(sum, umapKV_(f, rm))
			}
			if i%40 == 39 {
				t.try(fmt.Sprintf("random %s %d", sc.name, i), func() any { return strings.Join(sum, " ") + ufmt_(f.asMap(rm)) })
				sum = nil
			}
		}
	}
	return t.log
}

// ---- the keys of the scenarios

// ukey_ is a class with its own equals and hashCode (compared by equals in a set or map).
type ukey_ struct {
	a, b int32
}

func (k *ukey_) self() any         { return k }
func (k *ukey_) equals(o any) bool { x, ok := o.(*ukey_); return ok && x != nil && x.a == k.a }
func (k *ukey_) hashCode() int32   { return k.a }
func (k *ukey_) toString() string  { return fmt.Sprint("k", k.a, "#", k.b) }

var (
	ustrKeys_ = []string{"d", "b", "f", "a", "e", "c"}
	// six keys; uanyDup_ equals uanyKeys_[0] and is another object
	uanyKeys_ = []any{&ukey_{4, 0}, &ukey_{2, 0}, &ukey_{6, 0}, &ukey_{1, 0}, &ukey_{5, 0}, &ukey_{3, 0}}
	uanyDup_  = any(&ukey_{4, 1})
	// the comparators: strings without regard to case, descending; keys by their number
	ustrCmp_ Comparator = &cmpFunc_{func(a, b any) int32 {
		return String_compareTo(strings.ToLower(str_(b)), strings.ToLower(str_(a)))
	}}
	uanyCmp_ Comparator = &cmpFunc_{func(a, b any) int32 {
		if a == nil || b == nil {
			if a == nil && b == nil {
				return 0
			}
			if a == nil {
				return -1
			}
			return 1
		}
		return Integer_compare(a.(*ukey_).a, b.(*ukey_).a)
	}}
)

// uTraces_ runs every scenario on the functions of one family and returns the trace: lines
// "scenario: observation".
func uTraces_[L, D, S, M, SA, MA comparable](lf ulistFns_[L], df udequeFns_[D], sf usetFns_[string, S], mf umapFns_[string, M, S], saf usetFns_[any, SA], maf umapFns_[any, MA, SA], tparam func() []string) []string {
	var out []string
	add := func(name string, lines []string) {
		for _, l := range lines {
			out = append(out, name+": "+l)
		}
	}
	add("lists", uListScenario_(lf))
	add("deques", uDequeScenario_(df))
	add("sets of strings", uSetScenario_(sf, ustrKeys_, "d", any(int32(7)), ustrCmp_, true))
	add("maps of strings", uMapScenario_(mf, ustrKeys_, "d", any(int32(7)), true))
	add("sets by equals", uSetScenario_(saf, uanyKeys_, uanyDup_, any("str"), uanyCmp_, false))
	add("maps by equals", uMapScenario_(maf, uanyKeys_, uanyDup_, any("str"), false))
	add("type parameters", tparam())
	return out
}

// ---- the one family

var (
	utS_ = &tconv_[string]{box: strobj_, unbox: str_, null: nullstr_}
	// `any` elements compared by equals and hashCode, as tools/postflip/unify writes the
	// translator's byEquals
	utA_ = &tconv_[any]{box: func(v any) any { return v }, unbox: func(v any) any { return v },
		hash: func(v any) int32 { return objhash_(v) },
		eq:   func(a, b any) bool { return objequals_(a, b) }}
)

func uListFns_() ulistFns_[*TList[string]] {
	c := utS_
	type L = *TList[string]
	mkb := func(b *ImmutableList_Builder[string]) ulistBuilder_[L] {
		return ulistBuilder_[L]{add: func(x string) { b.add(x) }, addAll: func(from Iterable) { b.addAll(from) }, build: b.build}
	}
	return ulistFns_[L]{
		empty:     func() L { return tlistEmptyImm_(c) },
		of:        func(xs ...string) L { return immutableListOf_(c, xs...) },
		imm:       func(ic *ImmutableCollection) L { return tlistImm_(c, ic) },
		copyOf:    func(from Iterable) L { return immutableListCopyOf_(c, from) },
		copyOfArr: func(a []string) L { return immutableListCopyOfArr_(c, a) },
		copyOfT:   func(l L) L { return immutableListCopyOfT_(c, l) },
		newList:   func() L { return newTList_(c) },
		from:      func(x Collection) L { return tlistFrom_(c, x) },
		copyList:  func(l L) L { return tlistCopy_(c, l) },
		split:     func(sp *Splitter, seq string) L { return splitToListT_(c, sp, strobj_(seq)) },
		builder:   func() ulistBuilder_[L] { return mkb(immutableListBuilder_(c)) },
		builderN:  func(n int32) ulistBuilder_[L] { return mkb(immutableListBuilderN_(c, n)) },
		m:         func(l L) ulistM_ { return l },
	}
}

func uDequeFns_() udequeFns_[*TDeque[string]] {
	c := utS_
	type D = *TDeque[string]
	return udequeFns_[D]{
		newDeque:  func() D { return newTDeque_(c) },
		newDequeN: func(n int32) D { return newTDequeN_(c, n) },
		from:      func(x Collection) D { return tdequeFrom_(c, x) },
		copyDeque: func(d D) D { return tdequeCopy_(c, d) },
		m:         func(d D) udequeM_ { return d },
		items: func(d D) []string {
			var out []string
			for it := d.each(); it.next(); {
				out = append(out, it.v)
			}
			return out
		},
	}
}

func uSetFns_[K comparable](c *tconv_[K]) usetFns_[K, *TSet[K]] {
	type S = *TSet[K]
	return usetFns_[K, S]{
		empty:   func() S { return tsetEmptyImm_(c) },
		of:      func(xs ...K) S { return immutableSetOf_(c, xs...) },
		imm:     func(ic *ImmutableCollection) S { return tsetImm_(c, ic) },
		copyOf:  func(from Iterable) S { return immutableSetCopyOf_(c, from) },
		copyOfT: func(s S) S { return immutableSetCopyOfT_(c, s) },
		newSet:  func() S { return newTSet_(c) },
		newSetN: func(n int32) S { return newTSetN_(c, n) },
		from:    func(x Collection) S { return tsetFrom_(c, x, false) },
		copySet: func(s S) S { return tsetCopy_(c, s) },
		sorted:  func() S { return newTSortedSet_(c) },
		sortedC: func(cmp Comparator) S { return newTSortedSetC_(c, cmp) },
		builder: func() usetBuilder_[S] {
			b := immutableSetBuilder_(c)
			return usetBuilder_[S]{add: func(x any) { b.add(c.unbox(x)) }, addAll: func(from Iterable) { b.addAll(from) }, build: b.build}
		},
		add:     func(s S, k K) bool { return s.Add(k) },
		has:     func(s S, k K) bool { return s.Has(k) },
		del:     func(s S, k K) bool { return s.Delete(k) },
		size:    func(s S) int32 { return s.Len() },
		isEmpty: func(s S) bool { return s.IsEmpty() },
		clear:   func(s S) { s.Clear() },
		addAll:  func(s, o S) bool { return s.AddAll(o) },
		keys: func(s S) []K {
			var out []K
			for it := s.each(); it.next(); {
				out = append(out, it.v)
			}
			return out
		},
		asSet:    func(s S) Set { return s.asSet() },
		asImm:    func(s S) *ImmutableCollection { return s.asImm() },
		asSorted: func(s S) NavigableSet { return s.asSorted() },
		iterator: func(s S) Iterator { return s.iterator() },
		box:      c.box,
		listView: func(xs ...K) List {
			l := newTList_(c)
			for _, x := range xs {
				l.add(x)
			}
			return l.asList()
		},
		nullK: c.null,
	}
}

func uMapFns_[K comparable](kc *tconv_[K]) umapFns_[K, *TMap[K, string], *TSet[K]] {
	vc := utS_
	type M = *TMap[K, string]
	return umapFns_[K, M, *TSet[K]]{
		empty: func() M { return tmapEmptyImm_(kc, vc) },
		of1:   func(k1 K, v1 string) M { return immutableMapOf1_(kc, vc, k1, v1) },
		of3: func(k1 K, v1 string, k2 K, v2 string, k3 K, v3 string) M {
			return immutableMapOf3_(kc, vc, k1, v1, k2, v2, k3, v3)
		},
		of4: func(k1 K, v1 string, k2 K, v2 string, k3 K, v3 string, k4 K, v4 string) M {
			return immutableMapOf4_(kc, vc, k1, v1, k2, v2, k3, v3, k4, v4)
		},
		imm:     func(im *ErasedImmutableMap) M { return tmapImm_(kc, vc, im) },
		copyOf:  func(from Map) M { return immutableMapCopyOf_(kc, vc, from) },
		copyOfT: func(m M) M { return immutableMapCopyOfT_(kc, vc, m) },
		newMap:  func() M { return newTMap_(kc, vc) },
		newMapN: func(n int32) M { return newTMapN_(kc, vc, n) },
		from:    func(m Map) M { return tmapFrom_(kc, vc, m, false) },
		copyMap: func(m M) M { return tmapCopy_(kc, vc, m) },
		sorted:  func() M { return newTSortedMap_(kc, vc) },
		builder: func() umapBuilder_[M] {
			b := immutableMapBuilder_(kc, vc)
			return umapBuilder_[M]{put: func(k any, v string) { b.put(kc.unbox(k), v) }, putAll: func(from Map) { b.putAll(from) },
				build: b.buildOrThrow, buildKeepingLast: b.buildKeepingLast}
		},
		put:      func(m M, k K, v string) string { return m.Put(k, v) },
		get:      func(m M, k K) string { return m.Get(k) },
		getOr:    func(m M, k K, def string) string { return m.GetOr(k, def) },
		has:      func(m M, k K) bool { return m.Has(k) },
		del:      func(m M, k K) string { return m.Delete(k) },
		putIfAbs: func(m M, k K, v string) string { return m.PutIfAbsent(k, v) },
		compute:  func(m M, k K, f Function) string { return m.computeIfAbsent(k, f) },
		putAll:   func(m, o M) { m.putAllT(o) },
		size:     func(m M) int32 { return m.Len() },
		isEmpty:  func(m M) bool { return m.IsEmpty() },
		clear:    func(m M) { m.Clear() },
		keys: func(m M) []K {
			var out []K
			for it := m.each(); it.next(); {
				out = append(out, it.k)
			}
			return out
		},
		vals: func(m M) []string {
			var out []string
			for it := m.each(); it.next(); {
				out = append(out, it.v)
			}
			return out
		},
		entries: func(m M, each func(getKey func() K, getValue func() string, setValue func(string) string, asEntry func() Map_Entry)) {
			for it := m.eachEntry(); it.next(); {
				e := it.entry()
				each(e.getKey, e.getValue, e.setValue, e.asEntry)
			}
		},
		asMap:     func(m M) Map { return m.asMap() },
		asImm:     func(m M) *ErasedImmutableMap { return m.asImm() },
		keySetImm: func(m M) *TSet[K] { return m.keySetImm() },
		setAsImm:  func(s *TSet[K]) *ImmutableCollection { return s.asImm() },
		box:       kc.box,
		nullK:     kc.null,
	}
}

// uTparam_ is the conversions of a type parameter: which instantiations compare by equals.
func uTparam_() []string {
	t := &utr_{}
	regGoEquals_((*utparamClass_)(nil))
	t.try("tcT_: one object per instantiation", func() any {
		return tcT_[string]() == tcT_[string]() && tchT_[string]() == tchT_[string]() && any(tcT_[string]()) != any(tchT_[string]()) && tcT_[string]().null == nullstr_
	})
	t.try("by equals: string, registered class, class with equals, interface", func() any {
		return fmt.Sprint(tchT_[string]().hash != nil, tchT_[*utparamClass_]().hash != nil, tchT_[*ukey_]().hash != nil, tchT_[any]().hash != nil, tcT_[any]().hash != nil)
	})
	t.try("a set of a type parameter by equals", func() any {
		s := newTSet_(tchT_[*ukey_]())
		return fmt.Sprint(s.Add(&ukey_{1, 0}), s.Add(&ukey_{1, 1}), s.Add(&ukey_{2, 0}), s.Has(&ukey_{2, 5}), s.Add(nil), s.Add(nil), s.Len()) + ufmt_(s.asSet())
	})
	t.try("a set of a type parameter by identity", func() any {
		s := newTSet_(tchT_[*utparamClass_]())
		a, b := &utparamClass_{1}, &utparamClass_{1}
		return fmt.Sprint(s.Add(a), s.Add(b), s.Add(a), s.Has(b), s.Len())
	})
	return t.log
}

type utparamClass_ struct{ n int32 }

func (k *utparamClass_) self() any { return k }

func uOneFamily_() []string {
	return uTraces_(uListFns_(), uDequeFns_(), uSetFns_(utS_), uMapFns_(utS_), uSetFns_(utA_), uMapFns_(utA_), uTparam_)
}

func TestUnifiedFamily(t *testing.T) {
	got := uOneFamily_()
	for i := 0; i < len(got) || i < len(unifyGolden_); i++ {
		var g, w string
		if i < len(got) {
			g = got[i]
		}
		if i < len(unifyGolden_) {
			w = unifyGolden_[i]
		}
		if g != w {
			t.Errorf("line %d:\n   the one family: %s\n  the translator's: %s", i, g, w)
		}
	}
}

// ImmutableMap.of() is one object per key and value type, also where several key types share
// the conversion of the value type (the translator's family kept it so).
func TestEmptyImmutableMapIdentity(t *testing.T) {
	k2 := &tconv_[string]{box: strobj_, unbox: str_, null: nullstr_}
	a, b, c := tmapEmptyImm_(utS_, utS_), tmapEmptyImm_(utA_, utS_), tmapEmptyImm_(k2, utS_)
	if tmapEmptyImm_(utS_, utS_) != a || tmapEmptyImm_(utA_, utS_) != b || tmapEmptyImm_(k2, utS_) != c || tmapEmptyImm_(utS_, utS_) != a || a == c {
		t.Error("the empty immutable map of a key and value conversion is not one object")
	}
	if a.asImm() != ImmutableMap_of() || b.asImm() != ImmutableMap_of() || c.asImm() != ImmutableMap_of() {
		t.Error("the twin of an empty immutable map is not the runtime's")
	}
}
