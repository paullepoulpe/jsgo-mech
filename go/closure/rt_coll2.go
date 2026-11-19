package closure

// Sorted collections, queues, bit sets and the abstract bases that translated
// classes extend.

import (
	"math/bits"
	"sort"
)

// ---- TreeMap: a sorted slice of entries

type TreeMap struct {
	cmp     Comparator
	entries []*hentry_
}

func new_TreeMap() *TreeMap { return &TreeMap{} }
func (this *TreeMap) compare(a, b any) int32 {
	if this.cmp != nil {
		return this.cmp.compare(a, b)
	}
	return compareNatural_(a, b)
}

// search returns the position of k and whether it is present.
func (this *TreeMap) search(k any) (int, bool) {
	i := sort.Search(len(this.entries), func(i int) bool { return this.compare(this.entries[i].key, k) >= 0 })
	return i, i < len(this.entries) && this.compare(this.entries[i].key, k) == 0
}

// strKeyed reports whether a string key can be looked up without boxing it: the map orders its
// keys naturally and they are strings (naturally ordered keys are all of one class).
func (this *TreeMap) strKeyed() bool {
	if this == nil || this.cmp != nil {
		return false
	}
	if len(this.entries) == 0 {
		return true
	}
	_, ok := this.entries[0].key.(string)
	return ok
}

// search_s is search for a string key of a strKeyed map (String.compareTo order).
func (this *TreeMap) search_s(k string) (int, bool) {
	e := this.entries
	lo, hi := 0, len(e)
	for lo < hi {
		m := int(uint(lo+hi) >> 1)
		if String_compareTo(e[m].key.(string), k) < 0 {
			lo = m + 1
		} else {
			hi = m
		}
	}
	return lo, lo < len(e) && e[lo].key.(string) == k
}

func (this *TreeMap) size() int32   { return int32(len(this.entries)) }
func (this *TreeMap) isEmpty() bool { return len(this.entries) == 0 }
func (this *TreeMap) clear()        { this.entries = nil }

func (this *TreeMap) get(k any) any {
	if i, ok := this.search(k); ok {
		return this.entries[i].val
	}
	return nil
}

func (this *TreeMap) containsKey(k any) bool { _, ok := this.search(k); return ok }

func (this *TreeMap) containsValue(v any) bool {
	for _, e := range this.entries {
		if objequals_(v, e.val) {
			return true
		}
	}
	return false
}

func (this *TreeMap) put(k any, v any) any {
	i, ok := this.search(k)
	if ok {
		old := this.entries[i].val
		this.entries[i].val = v
		return old
	}
	this.entries = append(this.entries, nil)
	copy(this.entries[i+1:], this.entries[i:])
	this.entries[i] = &hentry_{key: k, val: v}
	return nil
}

func (this *TreeMap) putAll(m Map) {
	for it := m.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		this.put(e.getKey(), e.getValue())
	}
}

func (this *TreeMap) remove(k any) any {
	i, ok := this.search(k)
	if !ok {
		return nil
	}
	old := this.entries[i].val
	this.entries = append(this.entries[:i], this.entries[i+1:]...)
	return old
}

func (this *TreeMap) snapshot(pick func(*hentry_) any) []any {
	out := make([]any, len(this.entries))
	for i, e := range this.entries {
		out[i] = pick(e)
	}
	return out
}

func (this *TreeMap) keySet() Set {
	s := new_LinkedHashSet()
	for _, e := range this.entries {
		s.add(e.key)
	}
	return s
}

func (this *TreeMap) values() Collection {
	return listOf_(this.snapshot(func(e *hentry_) any { return e.val }))
}

func (this *TreeMap) entrySet() Set {
	s := new_LinkedHashSet()
	s.identity = true
	for _, e := range this.entries {
		s.add(e)
	}
	return s
}

func (this *TreeMap) equals(o any) bool { return mapEquals_(this, o) }
func (this *TreeMap) hashCode() int32   { return mapHash_(this) }
func (this *TreeMap) toString() string  { return mapToString_(this) }

// ---- TreeSet: a sorted slice

type TreeSet struct {
	cmp   Comparator
	items []any
}

func new_TreeSet() *TreeSet                        { return &TreeSet{} }
func new_TreeSet_Comparator(c Comparator) *TreeSet { return &TreeSet{cmp: c} }
func (this *TreeSet) compare(a, b any) int32 {
	if this.cmp != nil {
		return this.cmp.compare(a, b)
	}
	return compareNatural_(a, b)
}

func (this *TreeSet) search(k any) (int, bool) {
	i := sort.Search(len(this.items), func(i int) bool { return this.compare(this.items[i], k) >= 0 })
	return i, i < len(this.items) && this.compare(this.items[i], k) == 0
}

func (this *TreeSet) size() int32         { return int32(len(this.items)) }
func (this *TreeSet) isEmpty() bool       { return len(this.items) == 0 }
func (this *TreeSet) clear()              { this.items = nil }
func (this *TreeSet) contains(k any) bool { _, ok := this.search(k); return ok }

func (this *TreeSet) add(k any) bool {
	i, ok := this.search(k)
	if ok {
		return false
	}
	this.items = append(this.items, nil)
	copy(this.items[i+1:], this.items[i:])
	this.items[i] = k
	return true
}

func (this *TreeSet) addAll(c Collection) bool {
	changed := false
	for it := c.iterator(); it.hasNext(); {
		if this.add(it.next()) {
			changed = true
		}
	}
	return changed
}

func (this *TreeSet) remove_Object(k any) bool {
	i, ok := this.search(k)
	if !ok {
		return false
	}
	this.items = append(this.items[:i], this.items[i+1:]...)
	return true
}

func (this *TreeSet) iterator() Iterator {
	return &sliceIter_{items: &this.items, del: func(i int) { this.items = append(this.items[:i], this.items[i+1:]...) }}
}

func (this *TreeSet) containsAll(c Collection) bool { return containsAll_(this, c) }
func (this *TreeSet) removeAll(c Collection) bool   { return removeAll_(this, c) }
func (this *TreeSet) toArray_0() []any              { return append([]any(nil), this.items...) }
func (this *TreeSet) toArray(a []any) []any         { return fillArray_(this.items, a) }
func (this *TreeSet) equals(o any) bool             { return setEquals_(this, o) }
func (this *TreeSet) hashCode() int32               { return setHash_(this) }
func (this *TreeSet) toString() string              { return collToString_(this) }

func (this *TreeSet) descendingSet() NavigableSet {
	out := &TreeSet{cmp: Comparator_reversed(&cmpFunc_{this.compare})}
	for i := len(this.items) - 1; i >= 0; i-- {
		out.items = append(out.items, this.items[i])
	}
	return out
}

func (this *TreeSet) tailSet(from any, inclusive bool) NavigableSet {
	i, ok := this.search(from)
	if ok && !inclusive {
		i++
	}
	return &TreeSet{cmp: this.cmp, items: append([]any(nil), this.items[i:]...)}
}

// ---- PriorityQueue: the JDK's binary heap, step for step, so that elements of equal priority
// come out in the same order as in Java.

type PriorityQueue struct {
	ArrayList
	cmp Comparator
}

func new_PriorityQueue(c Comparator) *PriorityQueue {
	this := &PriorityQueue{cmp: c}
	this.self_ = this
	return this
}

func new_PriorityQueue_int_Comparator(n int32, c Comparator) *PriorityQueue {
	return new_PriorityQueue(c)
}

func (this *PriorityQueue) order() Comparator {
	if this.cmp == nil {
		return naturalOrder_
	}
	return this.cmp
}

// siftUp and siftDown compare as the JDK's do, the same pairs in the same order. With an
// int-key comparator (Comparator.comparingInt, reversed or not) they call its compare on the
// concrete type, so it is inlined into the loop.
func (this *PriorityQueue) siftUp(k int, x any) {
	if c, ok := this.cmp.(*intKeyCmp_); ok {
		siftUpInt_(c, this.items, k, x)
		return
	}
	c, es := this.order(), this.items
	for k > 0 {
		parent := (k - 1) >> 1
		e := es[parent]
		if c.compare(x, e) >= 0 {
			break
		}
		es[k] = e
		k = parent
	}
	es[k] = x
}

func siftUpInt_(c *intKeyCmp_, es []any, k int, x any) {
	for k > 0 {
		parent := (k - 1) >> 1
		e := es[parent]
		if c.compare(x, e) >= 0 {
			break
		}
		es[k] = e
		k = parent
	}
	es[k] = x
}

func (this *PriorityQueue) siftDown(k int, x any) {
	if c, ok := this.cmp.(*intKeyCmp_); ok {
		siftDownInt_(c, this.items, k, x)
		return
	}
	c, es := this.order(), this.items
	n := len(es)
	half := n >> 1
	for k < half {
		child := 2*k + 1
		v := es[child]
		if right := child + 1; right < n && c.compare(v, es[right]) > 0 {
			child = right
			v = es[child]
		}
		if c.compare(x, v) <= 0 {
			break
		}
		es[k] = v
		k = child
	}
	es[k] = x
}

func siftDownInt_(c *intKeyCmp_, es []any, k int, x any) {
	n := len(es)
	half := n >> 1
	for k < half {
		child := 2*k + 1
		v := es[child]
		if right := child + 1; right < n && c.compare(v, es[right]) > 0 {
			child = right
			v = es[child]
		}
		if c.compare(x, v) <= 0 {
			break
		}
		es[k] = v
		k = child
	}
	es[k] = x
}

func (this *PriorityQueue) add(v any) bool {
	if v == nil {
		panic(new_NullPointerException_0().self())
	}
	this.items = append(this.items, nil)
	this.siftUp(len(this.items)-1, v)
	return true
}

func (this *PriorityQueue) addAll(c Collection) bool {
	for it := c.iterator(); it.hasNext(); {
		this.add(it.next())
	}
	return c.size() > 0
}

func (this *PriorityQueue) peek() any {
	if len(this.items) == 0 {
		return nil
	}
	return this.items[0]
}

func (this *PriorityQueue) poll() any {
	n := len(this.items)
	if n == 0 {
		return nil
	}
	result := this.items[0]
	x := this.items[n-1]
	this.items[n-1] = nil
	this.items = this.items[:n-1]
	if n > 1 {
		this.siftDown(0, x)
	}
	return result
}

func (this *PriorityQueue) remove_0() any {
	if len(this.items) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return this.poll()
}

func (this *PriorityQueue) remove(i int32) any {
	old := this.items[i]
	this.removeAt(int(i))
	return old
}

func (this *PriorityQueue) removeAt(i int) {
	s := len(this.items) - 1
	moved := this.items[s]
	this.items[s] = nil
	this.items = this.items[:s]
	if s == i {
		return
	}
	this.siftDown(i, moved)
	if this.items[i] == moved {
		this.siftUp(i, moved)
	}
}

func (this *PriorityQueue) remove_Object(v any) bool {
	i := this.indexOf(v)
	if i < 0 {
		return false
	}
	this.removeAt(int(i))
	return true
}

type LinkedBlockingQueue struct{ ArrayDeque }

func new_LinkedBlockingQueue() *LinkedBlockingQueue {
	this := &LinkedBlockingQueue{}
	this.self_ = this
	return this
}

// ---- EnumMap, EnumSet

// ordinalCmp_ orders enum constants by ordinal. It holds no state, so there is one.
func ordinalCmp_() Comparator { return ordinalCmpOne_ }

var ordinalCmpOne_ Comparator = &cmpFunc_{func(a, b any) int32 {
	return a.(interface{ ordinal() int32 }).ordinal() - b.(interface{ ordinal() int32 }).ordinal()
}}

// EnumMap is a TreeMap ordered by ordinal.
type EnumMap struct{ TreeMap }

func new_EnumMap(c *Class) *EnumMap { return &EnumMap{TreeMap{cmp: ordinalCmp_()}} }

// EnumSet is java.util.EnumSet: a bit set over the constants of one enum, which iterates in
// ordinal order. univ is that enum's constants by ordinal (Enum.values_), taken from the first
// constant the set sees; an element is in the set when its bit is set and it is a constant of
// univ (Java's EnumSet holds the constants of one enum type and contains nothing else).
type EnumSet struct {
	univ []any
	w    uint64   // ordinals 0-63
	more []uint64 // ordinals from 64 on
}

var _ Set = (*EnumSet)(nil)

// enumConst_ is an enum constant: every enum class embeds Enum.
type enumConst_ interface{ enum_() *Enum }

// of_ takes the enum of constant e as the set's when it has none yet, and otherwise checks that
// e belongs to it (Java: a ClassCastException for a constant of another enum).
func (s *EnumSet) of_(e *Enum) {
	if len(e.values_) == 0 {
		panic("EnumSet: enum not registered: " + e.name_)
	}
	if s.univ == nil {
		s.univ = e.values_
	} else if &s.univ[0] != &e.values_[0] {
		panic(new_ClassCastException_String(goTypeName_(e.self_) + " is not an enum of this EnumSet").self())
	}
}

// member returns the constant e of k, if k is a constant of this set's enum.
func (s *EnumSet) member(k any) (*Enum, bool) {
	c, ok := k.(enumConst_)
	if !ok || s.univ == nil {
		return nil, false
	}
	e := c.enum_()
	return e, len(e.values_) > 0 && &e.values_[0] == &s.univ[0]
}

func (s *EnumSet) word(i int) *uint64 {
	if i == 0 {
		return &s.w
	}
	for len(s.more) < i {
		s.more = append(s.more, 0)
	}
	return &s.more[i-1]
}

func (s *EnumSet) has(o int32) bool {
	if o < 64 {
		return s.w&(1<<uint(o)) != 0
	}
	i := int(o>>6) - 1
	return i < len(s.more) && s.more[i]&(1<<uint(o&63)) != 0
}

func (s *EnumSet) size() int32 {
	n := bits.OnesCount64(s.w)
	for _, x := range s.more {
		n += bits.OnesCount64(x)
	}
	return int32(n)
}

func (s *EnumSet) isEmpty() bool { return s.size() == 0 }

func (s *EnumSet) clear() { s.w, s.more = 0, nil }

func (s *EnumSet) contains(k any) bool {
	e, ok := s.member(k)
	return ok && s.has(e.ordinal_)
}

func (s *EnumSet) add(k any) bool {
	if k == nil {
		panic(new_NullPointerException_0().self())
	}
	e := k.(enumConst_).enum_()
	s.of_(e)
	w, b := s.word(int(e.ordinal_>>6)), uint64(1)<<uint(e.ordinal_&63)
	if *w&b != 0 {
		return false
	}
	*w |= b
	return true
}

func (s *EnumSet) remove_Object(k any) bool {
	e, ok := s.member(k)
	if !ok || !s.has(e.ordinal_) {
		return false
	}
	*s.word(int(e.ordinal_ >> 6)) &^= 1 << uint(e.ordinal_&63)
	return true
}

// same reports whether c is an EnumSet over the same enum as s (both have seen a constant).
func (s *EnumSet) same(c Collection) (*EnumSet, bool) {
	t, ok := c.(*EnumSet)
	return t, ok && s.univ != nil && t.univ != nil && &s.univ[0] == &t.univ[0]
}

func (s *EnumSet) addAll(c Collection) bool {
	if t, ok := c.(*EnumSet); ok && (t.univ == nil || s.univ == nil || &s.univ[0] == &t.univ[0]) {
		if t.univ == nil {
			return false
		}
		s.univ = t.univ
		changed := s.w|t.w != s.w
		s.w |= t.w
		for i, x := range t.more {
			w := s.word(i + 1)
			changed = changed || *w|x != *w
			*w |= x
		}
		return changed
	}
	changed := false
	for it := c.iterator(); it.hasNext(); {
		if s.add(it.next()) {
			changed = true
		}
	}
	return changed
}

func (s *EnumSet) containsAll(c Collection) bool {
	if t, ok := s.same(c); ok {
		if t.w&^s.w != 0 {
			return false
		}
		for i, x := range t.more {
			if i >= len(s.more) {
				if x != 0 {
					return false
				}
			} else if x&^s.more[i] != 0 {
				return false
			}
		}
		return true
	}
	return containsAll_(s, c)
}

func (s *EnumSet) removeAll(c Collection) bool { return removeAll_(s, c) }

func (s *EnumSet) items() []any {
	out := make([]any, 0, s.size())
	for it := s.iter(); it.hasNext(); {
		out = append(out, it.next())
	}
	return out
}

func (s *EnumSet) toArray_0() []any      { return s.items() }
func (s *EnumSet) toArray(a []any) []any { return fillArray_(s.items(), a) }

func (s *EnumSet) equals(o any) bool {
	if t, ok := o.(*EnumSet); ok {
		if t, ok := s.same(t); ok {
			if s.w != t.w {
				return false
			}
			for i := 0; i < max(len(s.more), len(t.more)); i++ {
				var x, y uint64
				if i < len(s.more) {
					x = s.more[i]
				}
				if i < len(t.more) {
					y = t.more[i]
				}
				if x != y {
					return false
				}
			}
			return true
		}
	}
	return setEquals_(s, o)
}

func (s *EnumSet) hashCode() int32  { return setHash_(s) }
func (s *EnumSet) toString() string { return collToString_(s) }

func (s *EnumSet) iterator() Iterator { return s.iter() }

func (s *EnumSet) iter() *enumSetIter_ { return &enumSetIter_{s: s, unseen: s.w, last: -1} }

// enumSetIter_ walks the set in ordinal order as Java's does: it takes a copy of one word of
// bits at a time (all of them for an enum of up to 64 constants), so additions to a word it is
// in are not seen, and remove() clears the constant it returned last.
type enumSetIter_ struct {
	s      *EnumSet
	unseen uint64
	wi     int // index of the word unseen was taken from
	last   int32
}

func (it *enumSetIter_) hasNext() bool {
	for it.unseen == 0 {
		if it.wi >= len(it.s.more) {
			return false
		}
		it.unseen = it.s.more[it.wi]
		it.wi++
	}
	return true
}

func (it *enumSetIter_) next() any {
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	b := bits.TrailingZeros64(it.unseen)
	it.unseen &= it.unseen - 1
	it.last = int32(it.wi*64 + b)
	return it.s.univ[it.last]
}

func (it *enumSetIter_) remove() {
	if it.last < 0 {
		panic(new_IllegalStateException_0().self())
	}
	*it.s.word(int(it.last >> 6)) &^= 1 << uint(it.last&63)
	it.last = -1
}

func EnumSet_noneOf(c *Class) *EnumSet { return &EnumSet{} }

func EnumSet_allOf(c *Class) *EnumSet {
	s := EnumSet_noneOf(c)
	for _, v := range enumRegistry_[c.name_] {
		s.add(v)
	}
	return s
}

func EnumSet_copyOf(c Collection) *EnumSet {
	s := EnumSet_noneOf(nil)
	s.addAll(c)
	return s
}

func EnumSet_of(e *Enum) *EnumSet {
	s := EnumSet_noneOf(nil)
	s.add(e.self())
	return s
}

func EnumSet_of_Enum_EnumArr(e *Enum, rest []*Enum) *EnumSet {
	s := EnumSet_of(e)
	for _, x := range rest {
		s.add(x.self())
	}
	return s
}

// ---- BitSet

type BitSet struct {
	w []uint64
}

func new_BitSet() *BitSet            { return &BitSet{} }
func new_BitSet_int(n int32) *BitSet { return &BitSet{w: make([]uint64, 0, (n+63)/64)} }

func (this *BitSet) grow(i int32) {
	for int(i>>6) >= len(this.w) {
		this.w = append(this.w, 0)
	}
}

func (this *BitSet) get(i int32) bool {
	return int(i>>6) < len(this.w) && this.w[i>>6]&(1<<(uint(i)&63)) != 0
}

func (this *BitSet) set(i int32) {
	this.grow(i)
	this.w[i>>6] |= 1 << (uint(i) & 63)
}

func (this *BitSet) clear_int(i int32) {
	if int(i>>6) < len(this.w) {
		this.w[i>>6] &^= 1 << (uint(i) & 63)
	}
}

func (this *BitSet) clear() { this.w = this.w[:0] }

func (this *BitSet) set_int_boolean(i int32, v bool) {
	if v {
		this.set(i)
	} else {
		this.clear_int(i)
	}
}

func (this *BitSet) set_int_int(a int32, b int32) {
	for i := a; i < b; i++ {
		this.set(i)
	}
}

func (this *BitSet) set_int_int_boolean(a int32, b int32, v bool) {
	for i := a; i < b; i++ {
		this.set_int_boolean(i, v)
	}
}

func (this *BitSet) flip(a int32, b int32) {
	for i := a; i < b; i++ {
		this.set_int_boolean(i, !this.get(i))
	}
}

func (this *BitSet) and(o *BitSet) {
	for i := range this.w {
		if i < len(o.w) {
			this.w[i] &= o.w[i]
		} else {
			this.w[i] = 0
		}
	}
}

func (this *BitSet) andNot(o *BitSet) {
	for i := range this.w {
		if i < len(o.w) {
			this.w[i] &^= o.w[i]
		}
	}
}

func (this *BitSet) or(o *BitSet) {
	for i, x := range o.w {
		if x != 0 {
			this.grow(int32(i * 64))
			this.w[i] |= x
		}
	}
}

func (this *BitSet) intersects(o *BitSet) bool {
	for i := range this.w {
		if i < len(o.w) && this.w[i]&o.w[i] != 0 {
			return true
		}
	}
	return false
}

func (this *BitSet) cardinality() int32 {
	var n int
	for _, x := range this.w {
		n += bits.OnesCount64(x)
	}
	return int32(n)
}

func (this *BitSet) isEmpty() bool {
	for _, x := range this.w {
		if x != 0 {
			return false
		}
	}
	return true
}

func (this *BitSet) length() int32 {
	for i := len(this.w) - 1; i >= 0; i-- {
		if this.w[i] != 0 {
			return int32(i*64 + 64 - bits.LeadingZeros64(this.w[i]))
		}
	}
	return 0
}

func (this *BitSet) size() int32 { return int32(len(this.w) * 64) }

func (this *BitSet) nextSetBit(from int32) int32 {
	for i := from; int(i>>6) < len(this.w); {
		x := this.w[i>>6] >> (uint(i) & 63)
		if x != 0 {
			return i + int32(bits.TrailingZeros64(x))
		}
		i = (i>>6 + 1) << 6
	}
	return -1
}

func (this *BitSet) previousSetBit(from int32) int32 {
	for i := from; i >= 0; i-- {
		if this.get(i) {
			return i
		}
	}
	return -1
}

func (this *BitSet) toByteArray() []int8 {
	n := (this.length() + 7) / 8
	out := make([]int8, n)
	for i := range out {
		out[i] = int8(this.w[i/8] >> (uint(i%8) * 8))
	}
	return out
}

func (this *BitSet) equals(o any) bool {
	x, ok := o.(*BitSet)
	if !ok || x == nil {
		return false
	}
	a, b := this.w, x.w
	if len(a) < len(b) {
		a, b = b, a
	}
	for i := range a {
		var y uint64
		if i < len(b) {
			y = b[i]
		}
		if a[i] != y {
			return false
		}
	}
	return true
}

func (this *BitSet) hashCode() int32 {
	var h uint64 = 1234
	for i := len(this.w) - 1; i >= 0; i-- {
		if this.w[i] != 0 || h != 1234 {
			h ^= this.w[i] * uint64(i+1)
		}
	}
	return int32(h>>32) ^ int32(h)
}

func (this *BitSet) clone() any { return &BitSet{w: append([]uint64(nil), this.w...)} }

// ---- bases that translated classes extend

// AbstractSet supplies the Set operations from the subclass's iterator() and
// size(), reached through self_.
type AbstractSet struct {
	self_ any
}

type absColl_ interface {
	iterator() Iterator
	size() int32
}

func (this *AbstractSet) as_AbstractSet() *AbstractSet { return this }

func (this *AbstractSet) init_AbstractSet() {}
func (this *AbstractSet) me() absColl_      { return this.self_.(absColl_) }
func (this *AbstractSet) isEmpty() bool     { return this.me().size() == 0 }

func (this *AbstractSet) contains(v any) bool {
	for it := this.me().iterator(); it.hasNext(); {
		if objequals_(v, it.next()) {
			return true
		}
	}
	return false
}

func (this *AbstractSet) add(v any) bool { panic(new_UnsupportedOperationException_0().self()) }
func (this *AbstractSet) remove_Object(v any) bool {
	panic(new_UnsupportedOperationException_0().self())
}
func (this *AbstractSet) clear() { panic(new_UnsupportedOperationException_0().self()) }
func (this *AbstractSet) removeAll(c Collection) bool {
	panic(new_UnsupportedOperationException_0().self())
}

func (this *AbstractSet) addAll(c Collection) bool {
	changed := false
	for it := c.iterator(); it.hasNext(); {
		if this.self_.(interface{ add(any) bool }).add(it.next()) {
			changed = true
		}
	}
	return changed
}

func (this *AbstractSet) containsAll(c Collection) bool {
	return containsAll_(this.self_.(Collection), c)
}

func (this *AbstractSet) toArray_0() []any {
	var out []any
	for it := this.me().iterator(); it.hasNext(); {
		out = append(out, it.next())
	}
	return out
}

func (this *AbstractSet) toArray(a []any) []any { return fillArray_(this.toArray_0(), a) }
func (this *AbstractSet) equals(o any) bool     { return setEquals_(this.self_.(Collection), o) }
func (this *AbstractSet) hashCode() int32       { return setHash_(this.self_.(Iterable)) }
func (this *AbstractSet) toString() string      { return collToString_(this.self_.(Iterable)) }

// AbstractCollection supplies the Collection operations from the subclass's iterator() and
// size(), reached through self_.
type AbstractCollection struct {
	self_ any
}

func (this *AbstractCollection) as_AbstractCollection() *AbstractCollection { return this }

func (this *AbstractCollection) init_AbstractCollection() {}
func (this *AbstractCollection) me() absColl_             { return this.self_.(absColl_) }
func (this *AbstractCollection) isEmpty() bool            { return this.me().size() == 0 }

func (this *AbstractCollection) contains(v any) bool {
	for it := this.me().iterator(); it.hasNext(); {
		if objequals_(v, it.next()) {
			return true
		}
	}
	return false
}

func (this *AbstractCollection) add(v any) bool { panic(new_UnsupportedOperationException_0().self()) }
func (this *AbstractCollection) remove_Object(v any) bool {
	panic(new_UnsupportedOperationException_0().self())
}
func (this *AbstractCollection) clear() { panic(new_UnsupportedOperationException_0().self()) }
func (this *AbstractCollection) removeAll(c Collection) bool {
	panic(new_UnsupportedOperationException_0().self())
}

func (this *AbstractCollection) addAll(c Collection) bool {
	changed := false
	for it := c.iterator(); it.hasNext(); {
		if this.self_.(interface{ add(any) bool }).add(it.next()) {
			changed = true
		}
	}
	return changed
}

func (this *AbstractCollection) containsAll(c Collection) bool {
	return containsAll_(this.self_.(Collection), c)
}

func (this *AbstractCollection) toArray_0() []any {
	var out []any
	for it := this.me().iterator(); it.hasNext(); {
		out = append(out, it.next())
	}
	return out
}

func (this *AbstractCollection) toArray(a []any) []any { return fillArray_(this.toArray_0(), a) }
func (this *AbstractCollection) toString() string      { return collToString_(this.self_.(Iterable)) }

// AbstractIterator is Guava's: subclasses implement computeNext() and call
// endOfData() when done.
type AbstractIterator struct {
	self_ any
	state int8 // 0 not ready, 1 ready, 2 done
	next_ any
}

func (this *AbstractIterator) as_AbstractIterator() *AbstractIterator { return this }

func (this *AbstractIterator) init_AbstractIterator() {}

func (this *AbstractIterator) endOfData() any {
	this.state = 2
	return nil
}

func (this *AbstractIterator) hasNext() bool {
	if this.state == 0 {
		v := this.self_.(interface{ computeNext() any }).computeNext()
		if this.state != 2 {
			this.next_ = v
			this.state = 1
		}
	}
	return this.state == 1
}

func (this *AbstractIterator) next() any {
	if !this.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	v := this.next_
	this.next_ = nil
	this.state = 0
	return v
}

func (this *AbstractIterator) peek() any {
	if !this.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	return this.next_
}

func new_TreeMap_Comparator(c Comparator) *TreeMap { return &TreeMap{cmp: c} }
