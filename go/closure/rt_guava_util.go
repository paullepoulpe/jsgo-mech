package closure

// Guava utility classes: Lists, Sets, Maps, Iterables, Iterators, Ordering,
// ComparisonChain, Range/TreeRangeSet, Tables, HashBiMap, Interners,
// MultimapBuilder.

type Lists struct{}
type Sets struct{}
type Maps struct{}
type Iterables struct{}
type Iterators struct{}
type Multimaps struct{}
type Tables struct{}
type Interners struct{}
type Maps_IteratorBasedAbstractMap struct{}

// ---- iterators

// filterIter_ yields the elements of it that keep accepts (no remove).
func filterIter_(it Iterator, keep func(any) bool) Iterator {
	var next_ any
	ready := false
	advance := func() bool {
		for !ready && it.hasNext() {
			v := it.next()
			if keep(v) {
				next_, ready = v, true
			}
		}
		return ready
	}
	return &funcIter_{
		has: advance,
		nxt: func() any {
			if !advance() {
				panic(new_NoSuchElementException_0().self())
			}
			v := next_
			next_, ready = nil, false
			return v
		},
	}
}

type UnmodifiableIterator struct {
	it Iterator
}

func (this *UnmodifiableIterator) hasNext() bool { return this.it.hasNext() }
func (this *UnmodifiableIterator) next() any     { return this.it.next() }
func (this *UnmodifiableIterator) remove()       { throwUOE_() }

type peekIter_ struct {
	it     Iterator
	has    bool
	peeked any
}

func (p *peekIter_) hasNext() bool { return p.has || p.it.hasNext() }

func (p *peekIter_) next() any {
	if p.has {
		v := p.peeked
		p.has, p.peeked = false, nil
		return v
	}
	return p.it.next()
}

func (p *peekIter_) peek() any {
	if !p.has {
		p.peeked = p.it.next()
		p.has = true
	}
	return p.peeked
}

func (p *peekIter_) remove() {
	if p.has {
		panic(new_IllegalStateException_String("Can't remove after you've peeked at next").self())
	}
	Iterator_remove(p.it)
}

func Iterators_peekingIterator(p0 Iterator) PeekingIterator {
	if p, ok := p0.(*peekIter_); ok {
		return p
	}
	return &peekIter_{it: p0}
}

// ---- Iterables

// iterView_ is a lazy Iterable; like Guava's FluentIterable it prints as a list.
type iterView_ struct {
	mk func() Iterator
}

func (v *iterView_) iterator() Iterator { return v.mk() }
func (v *iterView_) toString() string   { return collToString_(v) }

func Iterables_addAll(p0 Collection, p1 Iterable) bool {
	if c, ok := p1.(Collection); ok {
		return p0.addAll(c)
	}
	changed := false
	for it := p1.iterator(); it.hasNext(); {
		if p0.add(it.next()) {
			changed = true
		}
	}
	return changed
}

func iterOf_(v any) Iterator { return v.(Iterable).iterator() }

func Iterables_concat(p0 Iterable, p1 Iterable) Iterable {
	return &iterView_{func() Iterator { return flatIter_([]any{p0, p1}, iterOf_) }}
}

func Iterables_concat_Iterable_Iterable_Iterable(p0 Iterable, p1 Iterable, p2 Iterable) Iterable {
	return &iterView_{func() Iterator { return flatIter_([]any{p0, p1, p2}, iterOf_) }}
}

func Iterables_concat_Iterable(p0 Iterable) Iterable {
	return &iterView_{func() Iterator {
		outer := p0.iterator()
		var cur, last Iterator
		advance := func() bool {
			for {
				if cur != nil && cur.hasNext() {
					return true
				}
				if !outer.hasNext() {
					return false
				}
				cur = iterOf_(outer.next())
			}
		}
		return &funcIter_{
			has: advance,
			nxt: func() any {
				if !advance() {
					panic(new_NoSuchElementException_0().self())
				}
				last = cur
				return cur.next()
			},
			del: func() {
				if last == nil {
					panic(new_IllegalStateException_0().self())
				}
				Iterator_remove(last)
			},
		}
	}}
}

func Iterables_contains(p0 Iterable, p1 any) bool {
	if c, ok := p0.(Collection); ok {
		return c.contains(p1)
	}
	for it := p0.iterator(); it.hasNext(); {
		if objequals_(p1, it.next()) {
			return true
		}
	}
	return false
}

func Iterables_filter(p0 Iterable, p1 Predicate_base) Iterable {
	return &iterView_{func() Iterator { return filterIter_(p0.iterator(), p1.apply) }}
}

func Iterables_transform(p0 Iterable, p1 Function_base) Iterable {
	return &iterView_{func() Iterator { return mapIter_(p0.iterator(), p1.apply) }}
}

func Iterables_skip(p0 Iterable, p1 int32) Iterable {
	if p1 < 0 {
		throwIAE_("number to skip cannot be negative")
	}
	return &iterView_{func() Iterator {
		it := p0.iterator()
		for i := int32(0); i < p1 && it.hasNext(); i++ {
			it.next()
		}
		return it
	}}
}

func Iterables_size(p0 Iterable) int32 {
	if c, ok := p0.(Collection); ok {
		return c.size()
	}
	var n int32
	for it := p0.iterator(); it.hasNext(); it.next() {
		n++
	}
	return n
}

func Iterables_isEmpty(p0 Iterable) bool {
	if c, ok := p0.(Collection); ok {
		return c.isEmpty()
	}
	return !p0.iterator().hasNext()
}

func Iterables_get(p0 Iterable, p1 int32) any {
	if l, ok := p0.(List); ok {
		return l.get(p1)
	}
	if p1 < 0 {
		panic(new_IndexOutOfBoundsException_String("position (" + istr_(int64(p1)) + ") must not be negative").self())
	}
	var n int32
	for it := p0.iterator(); it.hasNext(); n++ {
		v := it.next()
		if n == p1 {
			return v
		}
	}
	panic(new_IndexOutOfBoundsException_String("position (" + istr_(int64(p1)) + ") must be less than the number of elements that remained (" + istr_(int64(n)) + ")").self())
}

func Iterables_getFirst(p0 Iterable, p1 any) any {
	if it := p0.iterator(); it.hasNext() {
		return it.next()
	}
	return p1
}

func Iterables_getLast(p0 Iterable) any {
	if l, ok := p0.(List); ok {
		if l.isEmpty() {
			panic(new_NoSuchElementException_0().self())
		}
		return l.get(l.size() - 1)
	}
	it := p0.iterator()
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	var v any
	for it.hasNext() {
		v = it.next()
	}
	return v
}

func Iterables_getOnlyElement(p0 Iterable) any {
	it := p0.iterator()
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	first := it.next()
	if !it.hasNext() {
		return first
	}
	msg := "expected one element but was: <" + objstr_(first)
	for i := 0; i < 4 && it.hasNext(); i++ {
		msg += ", " + objstr_(it.next())
	}
	if it.hasNext() {
		msg += ", ..."
	}
	throwIAE_(msg + ">")
	return nil
}

// ---- Lists

func Lists_charactersOf(p0 string) *ImmutableList {
	cs := String_toCharArray(p0)
	out := make([]any, len(cs))
	for i, c := range cs {
		out[i] = c
	}
	return &ImmutableCollection{kind: icList_, items: out}
}

// Lists.reverse: a live reversed view (an ImmutableList reverses to a copy).
func Lists_reverse(p0 List) List {
	if c, ok := p0.(*ImmutableCollection); ok {
		return c.reverse()
	}
	return &viewList_{
		sz:    p0.size,
		at:    func(i int32) any { return p0.get(p0.size() - 1 - i) },
		setf:  func(i int32, v any) any { return p0.set(p0.size()-1-i, v) },
		rmAt:  func(i int32) any { return p0.remove(p0.size() - 1 - i) },
		addAt: func(i int32, v any) { p0.add_int_Object(p0.size()-i, v) },
	}
}

// ---- Sets

type Sets_SetView struct {
	AbstractSet
	it  func() Iterator
	has func(any) bool
}

func newSetView_(it func() Iterator, has func(any) bool) *Sets_SetView {
	this := &Sets_SetView{it: it, has: has}
	this.self_ = this
	return this
}

func (this *Sets_SetView) iterator() Iterator  { return this.it() }
func (this *Sets_SetView) contains(v any) bool { return this.has(v) }
func (this *Sets_SetView) isEmpty() bool       { return !this.it().hasNext() }

func (this *Sets_SetView) size() int32 {
	var n int32
	for it := this.it(); it.hasNext(); it.next() {
		n++
	}
	return n
}

func (this *Sets_SetView) immutableCopy() *ImmutableSet { return immSet_(this.toArray_0()) }

func Sets_union(p0 Set, p1 Set) *Sets_SetView {
	return newSetView_(
		func() Iterator {
			return flatIter_([]any{0, 1}, func(k any) Iterator {
				if k == 0 {
					return &UnmodifiableIterator{it: p0.iterator()}
				}
				return filterIter_(p1.iterator(), func(v any) bool { return !p0.contains(v) })
			})
		},
		func(v any) bool { return p0.contains(v) || p1.contains(v) })
}

func Sets_intersection(p0 Set, p1 Set) *Sets_SetView {
	return newSetView_(
		func() Iterator { return filterIter_(p0.iterator(), p1.contains) },
		func(v any) bool { return p0.contains(v) && p1.contains(v) })
}

func Sets_difference(p0 Set, p1 Set) *Sets_SetView {
	return newSetView_(
		func() Iterator { return filterIter_(p0.iterator(), func(v any) bool { return !p1.contains(v) }) },
		func(v any) bool { return p0.contains(v) && !p1.contains(v) })
}

// Sets.immutableEnumSet: iterates in the constants' declaration order.
func Sets_immutableEnumSet(p0 *Enum, p1 []*Enum) *ImmutableSet {
	items := make([]any, 0, len(p1)+1)
	items = append(items, p0.self())
	for _, e := range p1 {
		items = append(items, e.self())
	}
	checkNoNulls_(items)
	sortSlice_(items, ordinalCmp_())
	set := immSet_(items)
	last := set.items[len(set.items)-1].(interface{ ordinal() int32 }).ordinal()
	set.byOrd = make([]any, last+1)
	for _, x := range set.items {
		set.byOrd[x.(interface{ ordinal() int32 }).ordinal()] = x
	}
	return set
}

func Sets_newIdentityHashSet() Set { return identitySet_() }

func Sets_newLinkedHashSetWithExpectedSize(p0 int32) *LinkedHashSet { return new_LinkedHashSet() }

func Maps_newLinkedHashMapWithExpectedSize(p0 int32) *LinkedHashMap { return new_LinkedHashMap() }

// ---- Ordering (translated classes extend it and override compare)

type Ordering struct {
	self_ any
	cmp_  Comparator // set for orderings made by this library
}

func (this *Ordering) as_Ordering() *Ordering { return this }

func (this *Ordering) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *Ordering) init_Ordering() {}

func (this *Ordering) compare(p0 any, p1 any) int32 {
	if this.cmp_ != nil {
		return this.cmp_.compare(p0, p1)
	}
	return this.self_.(Comparator).compare(p0, p1)
}

func Ordering_from(p0 Comparator) *Ordering {
	if o, ok := p0.(interface{ as_Ordering() *Ordering }); ok {
		return o.as_Ordering()
	}
	this := &Ordering{cmp_: p0}
	this.self_ = this
	return this
}

var naturalOrdering_ = Ordering_from(&cmpFunc_{func(a, b any) int32 {
	if a == nil || b == nil {
		throwNPE_("null")
	}
	return compareNatural_(a, b)
}})

func Ordering_natural() *Ordering { return naturalOrdering_ }

func (this *Ordering) reverse() *Ordering {
	return Ordering_from(&cmpFunc_{func(a, b any) int32 { return this.compare(b, a) }})
}

func (this *Ordering) nullsFirst() *Ordering {
	return Ordering_from(&cmpFunc_{func(a, b any) int32 {
		switch {
		case a == nil && b == nil:
			return 0
		case a == nil:
			return -1
		case b == nil:
			return 1
		}
		return this.compare(a, b)
	}})
}

// sortedCopy is a stable sort into a new mutable list.
func (this *Ordering) sortedCopy(p0 Iterable) List {
	s := copySlice_(p0)
	sortSlice_(s, &cmpFunc_{this.compare})
	return listOf_(s)
}

// ---- ComparisonChain

type ComparisonChain struct {
	r int32
}

func ComparisonChain_start() *ComparisonChain { return &ComparisonChain{} }

func (this *ComparisonChain) classify_(r int32) *ComparisonChain {
	if this.r == 0 {
		if r < 0 {
			this.r = -1
		} else if r > 0 {
			this.r = 1
		}
	}
	return this
}

func (this *ComparisonChain) compare(p0 any, p1 any) *ComparisonChain {
	if this.r != 0 {
		return this
	}
	return this.classify_(compareNatural_(p0, p1))
}

func (this *ComparisonChain) compare_int_int(p0 int32, p1 int32) *ComparisonChain {
	if this.r != 0 {
		return this
	}
	return this.classify_(Integer_compare(p0, p1))
}

func (this *ComparisonChain) result() int32 { return this.r }

// ---- Range (closed ranges only: the one factory the translation uses) and TreeRangeSet

type Range struct {
	lo, hi any
}

func (this *Range) toString() string { return "[" + objstr_(this.lo) + "‥" + objstr_(this.hi) + "]" }

func Range_closed(p0 any, p1 any) *Range {
	if p0 == nil || p1 == nil {
		throwNPE_("null")
	}
	r := &Range{lo: p0, hi: p1}
	if compareNatural_(p0, p1) > 0 {
		throwIAE_("Invalid range: " + r.toString())
	}
	return r
}

func (this *Range) contains(p0 any) bool {
	if p0 == nil {
		throwNPE_("null")
	}
	return compareNatural_(this.lo, p0) <= 0 && compareNatural_(p0, this.hi) <= 0
}

func (this *Range) apply(p0 any) bool { return this.contains(p0) }
func (this *Range) test(p0 any) bool  { return this.contains(p0) }
func (this *Range) equals(o any) bool {
	x, ok := o.(*Range)
	return ok && x != nil && objequals_(this.lo, x.lo) && objequals_(this.hi, x.hi)
}

func (this *Range) hashCode() int32 { return objhash_(this.lo)*31 + objhash_(this.hi) }

type TreeRangeSet struct {
	ranges []*Range // disjoint and not connected, ascending
}

func TreeRangeSet_create() *TreeRangeSet { return &TreeRangeSet{} }

func (this *TreeRangeSet) isEmpty() bool { return len(this.ranges) == 0 }

func (this *TreeRangeSet) contains(p0 any) bool {
	if p0 == nil {
		throwNPE_("null")
	}
	lo, hi := 0, len(this.ranges)
	for lo < hi {
		mid := (lo + hi) / 2
		r := this.ranges[mid]
		if compareNatural_(p0, r.lo) < 0 {
			hi = mid
		} else if compareNatural_(p0, r.hi) > 0 {
			lo = mid + 1
		} else {
			return true
		}
	}
	return false
}

// add coalesces the range with every range it is connected to.
func (this *TreeRangeSet) add(p0 *Range) {
	if p0 == nil {
		throwNPE_("null")
	}
	lo, hi := p0.lo, p0.hi
	out := make([]*Range, 0, len(this.ranges)+1)
	placed := false
	for _, r := range this.ranges {
		switch {
		case compareNatural_(r.hi, lo) < 0:
			out = append(out, r)
		case compareNatural_(hi, r.lo) < 0:
			if !placed {
				out = append(out, &Range{lo: lo, hi: hi})
				placed = true
			}
			out = append(out, r)
		default:
			if compareNatural_(r.lo, lo) < 0 {
				lo = r.lo
			}
			if compareNatural_(r.hi, hi) > 0 {
				hi = r.hi
			}
		}
	}
	if !placed {
		out = append(out, &Range{lo: lo, hi: hi})
	}
	this.ranges = out
}

func (this *TreeRangeSet) asRanges() Set {
	items := make([]any, len(this.ranges))
	for i, r := range this.ranges {
		items[i] = r
	}
	return &ImmutableCollection{kind: icSet_, items: items}
}

func (this *TreeRangeSet) equals(o any) bool {
	x, ok := o.(interface{ asRanges() Set })
	return ok && objequals_(this.asRanges(), x.asRanges())
}
func (this *TreeRangeSet) hashCode() int32  { return objhash_(this.asRanges()) }
func (this *TreeRangeSet) toString() string { return objstr_(this.asRanges()) }

// ---- Interners

type interner_ struct {
	m *LinkedHashMap
}

func (this *interner_) intern(v any) any {
	if v == nil {
		throwNPE_("null")
	}
	if c := this.m.get(v); c != nil {
		return c
	}
	this.m.put(v, v)
	return v
}

// The interner holds its samples strongly (nothing observes the difference).
func Interners_newWeakInterner() Interner { return &interner_{m: new_LinkedHashMap()} }

// ---- MultimapBuilder

type MultimapBuilder struct {
	set bool
}

type MultimapBuilder_MultimapBuilderWithKeys struct{}

type MultimapBuilder_ListMultimapBuilder struct{ MultimapBuilder }

type MultimapBuilder_SetMultimapBuilder struct{ MultimapBuilder }

// ---- Tables.newCustomTable: rows in the backing map, each row a map from the factory

type customTable_ struct {
	m Map // row key -> Map (column key -> value)
	f Supplier
}

// tableRow_ is the live view Table.row(r).
type tableRow_ struct {
	t *customTable_
	r any
}

// ---- HashBiMap

type HashBiMap struct {
	fwd, bwd *LinkedHashMap // key -> value and value -> key, as seen from this side
	inv_     *HashBiMap
}

func Iterators_filter(p0 Iterator, p1 Predicate_base) *UnmodifiableIterator {
	return &UnmodifiableIterator{it: filterIter_(p0, p1.apply)}
}

func Iterables_unmodifiableIterable(p0 Iterable) Iterable {
	if v, ok := p0.(*ImmutableCollection); ok {
		return v
	}
	return &iterView_{func() Iterator { return &UnmodifiableIterator{it: p0.iterator()} }}
}

func Iterables_indexOf(p0 Iterable, p1 Predicate_base) int32 {
	var i int32
	for it := p0.iterator(); it.hasNext(); i++ {
		if p1.apply(it.next()) {
			return i
		}
	}
	return -1
}

func Lists_newArrayList(p0 Iterator) *ArrayList {
	l := new_ArrayList_0()
	for p0.hasNext() {
		l.add(p0.next())
	}
	return l
}

// Lists.transform: a lazy view; the function runs on every access.
func Lists_transform(p0 List, p1 Function_base) List {
	return &viewList_{
		sz:   p0.size,
		at:   func(i int32) any { return p1.apply(p0.get(i)) },
		rmAt: func(i int32) any { return p1.apply(p0.remove(i)) },
	}
}

func (this *Sets_SetView) equals(o any) bool { return o == any(this) || setLikeEquals_(this, o) }

func setLikeEquals_(s Collection, o any) bool {
	if o == nil || isListLike_(o) {
		return false
	}
	if c, ok := o.(*ImmutableCollection); ok {
		return c.equals(s) || (int(s.size()) == len(c.items) && s.containsAll(c))
	}
	return setEquals_(s, o)
}

func (t *customTable_) clear() { t.m.clear() }
