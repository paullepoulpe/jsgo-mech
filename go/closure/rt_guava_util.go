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

func (this *Sets_SetView) equals(o any) bool { return o == any(this) || setLikeEquals_(this, o) }

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
	return immSet_(items)
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

func (this *MultimapBuilder) build() Multimap {
	if this.set {
		return LinkedHashMultimap_create()
	}
	return ArrayListMultimap_create()
}

type MultimapBuilder_MultimapBuilderWithKeys struct{}

type MultimapBuilder_ListMultimapBuilder struct{ MultimapBuilder }

type MultimapBuilder_SetMultimapBuilder struct{ MultimapBuilder }

func MultimapBuilder_linkedHashKeys() *MultimapBuilder_MultimapBuilderWithKeys {
	return &MultimapBuilder_MultimapBuilderWithKeys{}
}

func (this *MultimapBuilder_MultimapBuilderWithKeys) arrayListValues() *MultimapBuilder_ListMultimapBuilder {
	return &MultimapBuilder_ListMultimapBuilder{}
}

func (this *MultimapBuilder_MultimapBuilderWithKeys) linkedHashSetValues() *MultimapBuilder_SetMultimapBuilder {
	return &MultimapBuilder_SetMultimapBuilder{MultimapBuilder{set: true}}
}

// ---- Tables.newCustomTable: rows in the backing map, each row a map from the factory

type customTable_ struct {
	m Map // row key -> Map (column key -> value)
	f Supplier
}

func Tables_newCustomTable(p0 Map, p1 Supplier_base) Table {
	if !p0.isEmpty() {
		throwIAE_("backing map must be empty")
	}
	return &customTable_{m: p0, f: p1}
}

func (t *customTable_) rowMap_(r any) Map {
	if r == nil {
		return nil
	}
	x := t.m.get(r)
	if x == nil {
		return nil
	}
	return x.(Map)
}

func (t *customTable_) contains(r any, c any) bool {
	m := t.rowMap_(r)
	return m != nil && c != nil && m.containsKey(c)
}

func (t *customTable_) containsRow(r any) bool {
	m := t.rowMap_(r)
	return m != nil && !m.isEmpty()
}

func (t *customTable_) get(r any, c any) any {
	if m := t.rowMap_(r); m != nil && c != nil {
		return m.get(c)
	}
	return nil
}

func (t *customTable_) rows_() []any {
	var out []any
	for _, r := range toSlice_(t.m.keySet()) {
		if !t.rowMap_(r).isEmpty() {
			out = append(out, r)
		}
	}
	return out
}

func (t *customTable_) isEmpty() bool { return len(t.rows_()) == 0 }

func (t *customTable_) size() int32 {
	var n int32
	for _, r := range t.rows_() {
		n += t.rowMap_(r).size()
	}
	return n
}

func (t *customTable_) clear() { t.m.clear() }

func (t *customTable_) put(r any, c any, v any) any {
	if r == nil || c == nil || v == nil {
		throwNPE_("null")
	}
	m := t.rowMap_(r)
	if m == nil {
		m = t.f.get().(Map)
		t.m.put(r, m)
	}
	return m.put(c, v)
}

func (t *customTable_) remove(r any, c any) any {
	m := t.rowMap_(r)
	if m == nil || c == nil {
		return nil
	}
	old := m.remove(c)
	if m.isEmpty() {
		t.m.remove(r)
	}
	return old
}

func (t *customTable_) row(r any) Map {
	if r == nil {
		throwNPE_("null")
	}
	return &tableRow_{t: t, r: r}
}

func (t *customTable_) rowKeySet() Set {
	return &viewColl_{
		set: true,
		it: func() Iterator {
			rows := t.rows_()
			si := &sliceIter_{items: &rows}
			return &funcIter_{has: si.hasNext, nxt: si.next, del: func() { t.m.remove(rows[si.i-1]) }}
		},
		has: t.containsRow,
		rm: func(r any) bool {
			if !t.containsRow(r) {
				return false
			}
			t.m.remove(r)
			return true
		},
		clr: t.clear,
	}
}

func (t *customTable_) rowMap() Map {
	out := new_LinkedHashMap()
	for _, r := range t.rows_() {
		out.put(r, t.row(r))
	}
	return out
}

func (t *customTable_) values() Collection {
	return &viewColl_{
		it: func() Iterator {
			return flatIter_(t.rows_(), func(r any) Iterator { return t.row(r).values().iterator() })
		},
		sz:  t.size,
		clr: t.clear,
	}
}

func (t *customTable_) equals(o any) bool {
	x, ok := o.(interface{ rowMap() Map })
	return ok && objequals_(t.rowMap(), x.rowMap())
}
func (t *customTable_) hashCode() int32  { return objhash_(t.rowMap()) }
func (t *customTable_) toString() string { return objstr_(t.rowMap()) }

// tableRow_ is the live view Table.row(r).
type tableRow_ struct {
	t *customTable_
	r any
}

func (v *tableRow_) m_() Map { return v.t.rowMap_(v.r) }

func (v *tableRow_) size() int32 {
	if m := v.m_(); m != nil {
		return m.size()
	}
	return 0
}

func (v *tableRow_) isEmpty() bool { return v.size() == 0 }

func (v *tableRow_) get(c any) any {
	if m := v.m_(); m != nil && c != nil {
		return m.get(c)
	}
	return nil
}

func (v *tableRow_) containsKey(c any) bool {
	m := v.m_()
	return m != nil && c != nil && m.containsKey(c)
}

func (v *tableRow_) containsValue(x any) bool {
	m := v.m_()
	return m != nil && x != nil && m.containsValue(x)
}

func (v *tableRow_) put(c any, x any) any { return v.t.put(v.r, c, x) }

func (v *tableRow_) putAll(m Map) {
	for it := m.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		v.put(e.getKey(), e.getValue())
	}
}

func (v *tableRow_) remove(c any) any { return v.t.remove(v.r, c) }
func (v *tableRow_) clear()           { v.t.m.remove(v.r) }

func (v *tableRow_) view_(pick func(m Map) Collection, set bool) *viewColl_ {
	return &viewColl_{
		set: set,
		it: func() Iterator {
			m := v.m_()
			if m == nil {
				return Collections_emptyIterator()
			}
			inner := pick(m).iterator()
			return &funcIter_{has: inner.hasNext, nxt: inner.next, del: func() {
				Iterator_remove(inner)
				if m.isEmpty() {
					v.t.m.remove(v.r)
				}
			}}
		},
		sz:  v.size,
		clr: v.clear,
	}
}

func (v *tableRow_) keySet() Set {
	s := v.view_(func(m Map) Collection { return m.keySet() }, true)
	s.has = v.containsKey
	s.rm = func(c any) bool {
		if !v.containsKey(c) {
			return false
		}
		v.remove(c)
		return true
	}
	return s
}

func (v *tableRow_) values() Collection {
	return v.view_(func(m Map) Collection { return m.values() }, false)
}

func (v *tableRow_) entrySet() Set {
	return v.view_(func(m Map) Collection { return m.entrySet() }, true)
}

func (v *tableRow_) equals(o any) bool { return o == any(v) || mapEquals_(v, o) }
func (v *tableRow_) hashCode() int32   { return mapHash_(v) }
func (v *tableRow_) toString() string  { return mapToString_(v) }

// ---- HashBiMap

type HashBiMap struct {
	fwd, bwd *LinkedHashMap // key -> value and value -> key, as seen from this side
	inv_     *HashBiMap
}

func HashBiMap_create() *HashBiMap {
	this := &HashBiMap{fwd: new_LinkedHashMap(), bwd: new_LinkedHashMap()}
	this.inv_ = &HashBiMap{fwd: this.bwd, bwd: this.fwd, inv_: this}
	return this
}

func (this *HashBiMap) inverse() BiMap            { return this.inv_ }
func (this *HashBiMap) size() int32               { return this.fwd.size() }
func (this *HashBiMap) isEmpty() bool             { return this.fwd.isEmpty() }
func (this *HashBiMap) get(p0 any) any            { return this.fwd.get(p0) }
func (this *HashBiMap) containsKey(p0 any) bool   { return this.fwd.containsKey(p0) }
func (this *HashBiMap) containsValue(p0 any) bool { return this.bwd.containsKey(p0) }
func (this *HashBiMap) put(p0 any, p1 any) any    { return this.put_(p0, p1, false) }
func (this *HashBiMap) put_(k any, v any, force bool) any {
	had := this.fwd.containsKey(k)
	old := this.fwd.get(k)
	if had && objequals_(old, v) {
		return v
	}
	if this.bwd.containsKey(v) {
		if !force {
			throwIAE_("value already present: " + objstr_(v))
		}
		this.fwd.remove(this.bwd.get(v))
		this.bwd.remove(v)
	}
	if had {
		this.bwd.remove(old)
	}
	this.fwd.put(k, v)
	this.bwd.put(v, k)
	return old
}

func (this *HashBiMap) putAll(p0 Map) {
	for it := p0.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		this.put(e.getKey(), e.getValue())
	}
}

func (this *HashBiMap) remove(p0 any) any {
	if !this.fwd.containsKey(p0) {
		return nil
	}
	old := this.fwd.remove(p0)
	this.bwd.remove(old)
	return old
}

func (this *HashBiMap) clear() {
	this.fwd.clear()
	this.bwd.clear()
}

func (this *HashBiMap) iter_(pick func(k, v any) any) Iterator {
	keys := toSlice_(this.fwd.keySet())
	si := &sliceIter_{items: &keys}
	return &funcIter_{
		has: si.hasNext,
		nxt: func() any { k := si.next(); return pick(k, this.fwd.get(k)) },
		del: func() { this.remove(keys[si.i-1]) },
	}
}

func (this *HashBiMap) keySet() Set {
	return &viewColl_{
		set: true,
		it:  func() Iterator { return this.iter_(func(k, v any) any { return k }) },
		sz:  this.size,
		has: this.containsKey,
		rm: func(k any) bool {
			if !this.containsKey(k) {
				return false
			}
			this.remove(k)
			return true
		},
		clr: this.clear,
	}
}

func (this *HashBiMap) values() Collection {
	return &viewColl_{
		set: true,
		it:  func() Iterator { return this.iter_(func(k, v any) any { return v }) },
		sz:  this.size,
		has: this.containsValue,
		clr: this.clear,
	}
}

func (this *HashBiMap) entrySet() Set {
	return &viewColl_{
		set: true,
		it:  func() Iterator { return this.iter_(func(k, v any) any { return &immEntry_{key: k, val: v} }) },
		sz:  this.size,
		has: func(x any) bool {
			e, ok := x.(Map_Entry)
			return ok && e != nil && this.containsKey(e.getKey()) && objequals_(this.get(e.getKey()), e.getValue())
		},
		clr: this.clear,
	}
}

func (this *HashBiMap) equals(o any) bool { return o == any(this) || mapEquals_(this, o) }
func (this *HashBiMap) hashCode() int32   { return mapHash_(this) }
func (this *HashBiMap) toString() string  { return mapToString_(this) }

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
