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

func new_TreeMap() *TreeMap                        { return &TreeMap{} }
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
	s.table.identity = true
	for _, e := range this.entries {
		s.add(e)
	}
	return s
}

func (this *TreeMap) firstKey() any {
	if len(this.entries) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return this.entries[0].key
}

func (this *TreeMap) lastKey() any {
	if len(this.entries) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return this.entries[len(this.entries)-1].key
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
func (this *TreeSet) retainAll(c Collection) bool   { return retainAll_(this, c) }
func (this *TreeSet) toArray_0() []any              { return append([]any(nil), this.items...) }
func (this *TreeSet) toArray(a []any) []any         { return fillArray_(this.items, a) }
func (this *TreeSet) equals(o any) bool             { return setEquals_(this, o) }
func (this *TreeSet) hashCode() int32               { return setHash_(this) }
func (this *TreeSet) toString() string              { return collToString_(this) }

func (this *TreeSet) first() any {
	if len(this.items) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return this.items[0]
}

func (this *TreeSet) last() any {
	if len(this.items) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return this.items[len(this.items)-1]
}

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

func (this *PriorityQueue) siftUp(k int, x any) {
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

func (this *PriorityQueue) siftDown(k int, x any) {
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

func (this *PriorityQueue) add(v any) bool {
	if v == nil {
		panic(new_NullPointerException_0().self())
	}
	this.items = append(this.items, nil)
	this.siftUp(len(this.items)-1, v)
	return true
}

func (this *PriorityQueue) offer(v any) bool { return this.add(v) }

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

// ---- EnumMap, EnumSet: ordered by ordinal

func ordinalCmp_() Comparator {
	return &cmpFunc_{func(a, b any) int32 {
		return a.(interface{ ordinal() int32 }).ordinal() - b.(interface{ ordinal() int32 }).ordinal()
	}}
}

type EnumMap struct{ TreeMap }

func new_EnumMap(c *Class) *EnumMap { return &EnumMap{TreeMap{cmp: ordinalCmp_()}} }

type EnumSet struct{ TreeSet }

func EnumSet_noneOf(c *Class) *EnumSet { return &EnumSet{TreeSet{cmp: ordinalCmp_()}} }

func EnumSet_allOf(c *Class) *EnumSet {
	s := EnumSet_noneOf(c)
	s.items = append(s.items, enumRegistry_[c.name_]...)
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

func (this *BitSet) nextClearBit(from int32) int32 {
	for i := from; ; i++ {
		if !this.get(i) {
			return i
		}
	}
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

func (this *AbstractSet) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

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

// AbstractIterator is Guava's: subclasses implement computeNext() and call
// endOfData() when done.
type AbstractIterator struct {
	self_ any
	state int8 // 0 not ready, 1 ready, 2 done
	next_ any
}

func (this *AbstractIterator) as_AbstractIterator() *AbstractIterator { return this }

func (this *AbstractIterator) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

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
