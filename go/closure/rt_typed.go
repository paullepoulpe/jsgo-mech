package closure

// Typed collections for experiment T: what a non-erasing translator would emit for
// List<T>, LinkedHashMap<K,V>, LinkedHashSet<K>. Same algorithms as rt_coll.go's htable_
// (keys in insertion order in a slice, values beside them, linear search up to tsmall_
// entries, a Go map from key to position above that, tombstones for removals), but the keys
// and values have their static types: no boxing, no type assertion on the way out, a
// typed Go map (map[string]int32, map[*T]int32, map[uint32]int32) as the index.

const tsmall_ = 8

// ---- TList: ArrayList<T>

type TList[T any] struct{ items []T }

func newTList_[T any]() *TList[T]           { return &TList[T]{} }
func newTListN_[T any](n int32) *TList[T]   { return &TList[T]{items: make([]T, 0, n)} }
func (l *TList[T]) add(v T)                 { l.items = append(l.items, v) }
func (l *TList[T]) size() int32             { return int32(len(l.items)) }
func (l *TList[T]) isEmpty() bool           { return len(l.items) == 0 }
func (l *TList[T]) get(i int32) T {
	if i < 0 || int(i) >= len(l.items) {
		panic(new_IndexOutOfBoundsException_String("Index: " + istr_(int64(i)) + ", Size: " + istr_(int64(len(l.items)))).self())
	}
	return l.items[i]
}

// ---- OMap: LinkedHashMap<K,V> for keys whose Java equality is Go's == (String, handle
// classes, enums, classes that do not override equals, boxed primitives).

type OMap[K comparable, V any] struct {
	keys  []K
	vals  []V
	dead  []bool // beside keys once something was removed; else nil
	idx   map[K]int32
	live  int32
	ndead int32
	head  int32 // every position below head is dead (removeFirst)
}

func newOMap_[K comparable, V any]() *OMap[K, V] { return &OMap[K, V]{} }

func (m *OMap[K, V]) size() int32   { return m.live }
func (m *OMap[K, V]) isEmpty() bool { return m.live == 0 }

// n is the number of positions; position i holds an entry if alive(i).
func (m *OMap[K, V]) n() int           { return len(m.keys) }
func (m *OMap[K, V]) alive(i int) bool { return m.dead == nil || !m.dead[i] }

func (m *OMap[K, V]) find(k K) int {
	if m.idx == nil {
		if m.dead == nil {
			for i, x := range m.keys {
				if x == k {
					return i
				}
			}
			return -1
		}
		for i, x := range m.keys {
			if x == k && !m.dead[i] {
				return i
			}
		}
		return -1
	}
	if i, ok := m.idx[k]; ok {
		return int(i)
	}
	return -1
}

func (m *OMap[K, V]) has(k K) bool { return m.find(k) >= 0 }

func (m *OMap[K, V]) get(k K) (v V) {
	if i := m.find(k); i >= 0 {
		return m.vals[i]
	}
	return
}

func (m *OMap[K, V]) get2(k K) (v V, ok bool) {
	if i := m.find(k); i >= 0 {
		return m.vals[i], true
	}
	return
}

func (m *OMap[K, V]) insert(k K, v V) {
	i := len(m.keys)
	if m.keys == nil {
		m.keys = make([]K, 0, 4)
		m.vals = make([]V, 0, 4)
	}
	m.keys = append(m.keys, k)
	m.vals = append(m.vals, v)
	if m.dead != nil {
		m.dead = append(m.dead, false)
	}
	m.live++
	if m.idx != nil {
		m.idx[k] = int32(i)
	} else if i >= tsmall_ {
		m.idx = make(map[K]int32, 2*len(m.keys))
		for j, x := range m.keys {
			if m.alive(j) {
				m.idx[x] = int32(j)
			}
		}
	}
}

// put returns the previous value, if any.
func (m *OMap[K, V]) put(k K, v V) (old V, existed bool) {
	if i := m.find(k); i >= 0 {
		old = m.vals[i]
		m.vals[i] = v
		return old, true
	}
	m.insert(k, v)
	return
}

// add is Set.add: true if k was not there.
func (m *OMap[K, V]) add(k K) bool {
	if m.find(k) >= 0 {
		return false
	}
	var z V
	m.insert(k, z)
	return true
}

func (m *OMap[K, V]) removeAt(i int) {
	var zk K
	var zv V
	if m.idx != nil {
		delete(m.idx, m.keys[i])
	}
	if m.dead == nil {
		m.dead = make([]bool, len(m.keys), cap(m.keys))
	}
	m.dead[i] = true
	m.keys[i] = zk
	m.vals[i] = zv
	m.live--
	m.ndead++
	if m.live == 0 {
		m.clear()
	} else if m.ndead > 32 && m.ndead > 3*m.live {
		m.compact()
	}
}

func (m *OMap[K, V]) remove(k K) (old V, existed bool) {
	i := m.find(k)
	if i < 0 {
		return
	}
	old = m.vals[i]
	m.removeAt(i)
	return old, true
}

// first returns the first live position, or -1.
func (m *OMap[K, V]) first() int {
	if m.live == 0 {
		return -1
	}
	i := int(m.head)
	for m.dead != nil && m.dead[i] {
		i++
	}
	m.head = int32(i)
	return i
}

func (m *OMap[K, V]) compact() {
	j := 0
	for i := range m.keys {
		if !m.dead[i] {
			m.keys[j] = m.keys[i]
			m.vals[j] = m.vals[i]
			if m.idx != nil {
				m.idx[m.keys[j]] = int32(j)
			}
			j++
		}
	}
	clear(m.keys[j:])
	clear(m.vals[j:])
	m.keys = m.keys[:j]
	m.vals = m.vals[:j]
	m.dead = nil
	m.ndead = 0
	m.head = 0
}

func (m *OMap[K, V]) clear() {
	clear(m.keys)
	clear(m.vals)
	m.keys = m.keys[:0]
	m.vals = m.vals[:0]
	m.dead = nil
	m.idx = nil
	m.live, m.ndead, m.head = 0, 0, 0
}

// OSet is LinkedHashSet<K>.
type OSet[K comparable] = OMap[K, struct{}]

func newOSet_[K comparable]() *OSet[K] { return &OSet[K]{} }

// ---- HMap: LinkedHashMap<K,V> for a key class that overrides equals/hashCode. Like
// java.util.HashMap it keeps each key's hash code and tests identity before equals.

type hkey_ interface {
	comparable
	hashCode() int32
	equals(any) bool
}

type HMap[K hkey_, V any] struct {
	keys   []K
	hashes []int32
	vals   []V
	idx    map[int32]int32 // hash code -> first position
	next   []int32         // beside keys when indexed: 1 + next position with the same hash code
}

func newHMap_[K hkey_, V any]() *HMap[K, V] { return &HMap[K, V]{} }

func (m *HMap[K, V]) size() int32 { return int32(len(m.keys)) }

func (m *HMap[K, V]) find(k K) (int, int32) {
	var zk K
	if k == zk {
		return -1, 0 // no null key is ever stored
	}
	if m.idx == nil {
		for i, x := range m.keys {
			if x == k {
				return i, 0
			}
		}
		if len(m.keys) == 0 {
			return -1, k.hashCode()
		}
		h := k.hashCode()
		for i, xh := range m.hashes {
			if xh == h && k.equals(any(m.keys[i])) {
				return i, h
			}
		}
		return -1, h
	}
	h := k.hashCode()
	i, ok := m.idx[h]
	for ok {
		if x := m.keys[i]; x == k || k.equals(any(x)) {
			return int(i), h
		}
		if m.next[i] == 0 {
			break
		}
		i = m.next[i] - 1
	}
	return -1, h
}

func (m *HMap[K, V]) get(k K) (v V) {
	if i, _ := m.find(k); i >= 0 {
		return m.vals[i]
	}
	return
}

func (m *HMap[K, V]) link(h int32, i int32) {
	j, shared := m.idx[h]
	if !shared {
		m.idx[h] = i
		return
	}
	for m.next[j] != 0 {
		j = m.next[j] - 1
	}
	m.next[j] = i + 1
}

// insert appends a key that find did not find; h is its hash code as find returned it.
func (m *HMap[K, V]) insert(k K, h int32, v V) {
	i := int32(len(m.keys))
	if m.keys == nil {
		m.keys = make([]K, 0, 4)
		m.hashes = make([]int32, 0, 4)
		m.vals = make([]V, 0, 4)
	}
	m.keys = append(m.keys, k)
	m.hashes = append(m.hashes, h)
	m.vals = append(m.vals, v)
	if m.idx != nil {
		m.next = append(m.next, 0)
		m.link(h, i)
	} else if i >= tsmall_ {
		m.idx = make(map[int32]int32, 2*len(m.keys))
		m.next = make([]int32, len(m.keys), cap(m.keys))
		for j, xh := range m.hashes {
			m.link(xh, int32(j))
		}
	}
}

func (m *HMap[K, V]) put(k K, v V) (old V, existed bool) {
	i, h := m.find(k)
	if i >= 0 {
		old = m.vals[i]
		m.vals[i] = v
		return old, true
	}
	m.insert(k, h, v)
	return
}

// ---- erased views, for the places where a typed collection is handed to erased code

// tview_ is a read-only Iterable/Collection over n positions of a typed collection.
type tview_ struct {
	n   func() int
	at  func(i int) (any, bool) // the boxed element at position i, false if the position is dead
	szf func() int32
}

type tviewIter_ struct {
	v   *tview_
	i   int
	cur any
	has bool
}

func (it *tviewIter_) advance() {
	it.has = false
	for it.i < it.v.n() {
		x, ok := it.v.at(it.i)
		it.i++
		if ok {
			it.cur, it.has = x, true
			return
		}
	}
}
func (it *tviewIter_) hasNext() bool { return it.has }
func (it *tviewIter_) next() any {
	if !it.has {
		panic(new_NoSuchElementException_0().self())
	}
	x := it.cur
	it.advance()
	return x
}
func (it *tviewIter_) remove() { panic(new_UnsupportedOperationException_0().self()) }

func (v *tview_) iterator() Iterator {
	it := &tviewIter_{v: v}
	it.advance()
	return it
}
func (v *tview_) size() int32   { return v.szf() }
func (v *tview_) isEmpty() bool { return v.szf() == 0 }
func (v *tview_) toArray_0() []any {
	var out []any
	for i := 0; i < v.n(); i++ {
		if x, ok := v.at(i); ok {
			out = append(out, x)
		}
	}
	return out
}
func (v *tview_) toArray(a []any) []any { return fillArray_(v.toArray_0(), a) }
func (v *tview_) contains(x any) bool {
	for i := 0; i < v.n(); i++ {
		if y, ok := v.at(i); ok && objequals_(x, y) {
			return true
		}
	}
	return false
}
func (v *tview_) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *tview_) add(any) bool                  { panic(new_UnsupportedOperationException_0().self()) }
func (v *tview_) addAll(Collection) bool        { panic(new_UnsupportedOperationException_0().self()) }
func (v *tview_) clear()                        { panic(new_UnsupportedOperationException_0().self()) }
func (v *tview_) remove_Object(any) bool        { panic(new_UnsupportedOperationException_0().self()) }
func (v *tview_) removeAll(Collection) bool     { panic(new_UnsupportedOperationException_0().self()) }
func (v *tview_) toString() string              { return collToString_(v) }

// ---- typed for-each cursors (the typed counterpart of each_)

type teach_[T any] struct {
	src *[]T
	i   int
	v   T
}

func (l *TList[T]) each() teach_[T] { return teach_[T]{src: &l.items} }
func (e *teach_[T]) next() bool {
	if s := *e.src; e.i < len(s) {
		e.v = s[e.i]
		e.i++
		return true
	}
	return false
}

type omeach_[K comparable, V any] struct {
	m *OMap[K, V]
	i int
	k K
	v V
}

func (m *OMap[K, V]) each() omeach_[K, V] { return omeach_[K, V]{m: m} }
func (e *omeach_[K, V]) next() bool {
	m := e.m
	if m == nil {
		return false
	}
	for e.i < len(m.keys) {
		i := e.i
		e.i++
		if m.dead == nil || !m.dead[i] {
			e.k, e.v = m.keys[i], m.vals[i]
			return true
		}
	}
	return false
}

// stack operations of ArrayDeque<T> (push/peek/pop at the same end)
func (l *TList[T]) push(v T) { l.items = append(l.items, v) }
func (l *TList[T]) peek() (v T) {
	if n := len(l.items); n > 0 {
		return l.items[n-1]
	}
	return
}
func (l *TList[T]) pop() T {
	n := len(l.items)
	if n == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	v := l.items[n-1]
	var z T
	l.items[n-1] = z
	l.items = l.items[:n-1]
	return v
}

// erased iterator over a typed list of class pointers (Iterable<T> handed to erased code)
type tlistIter_[T comparable] struct {
	l *TList[T]
	i int
}

func (it *tlistIter_[T]) hasNext() bool { return it.i < len(it.l.items) }
func (it *tlistIter_[T]) next() any {
	if it.i >= len(it.l.items) {
		panic(new_NoSuchElementException_0().self())
	}
	v := it.l.items[it.i]
	it.i++
	return obj_(v)
}
func (it *tlistIter_[T]) remove() { panic(new_UnsupportedOperationException_0().self()) }

// ---- the erased faces of the typed collections of experiment T (Iterable handed to erased code)

type scopeVarsView_ struct{ m *OMap[string, *AbstractVar] }

func (v scopeVarsView_) iterator() Iterator {
	n := 0
	if v.m != nil {
		n = v.m.n()
	}
	i := 0
	adv := func() {
		for i < n && !v.m.alive(i) {
			i++
		}
	}
	adv()
	return &funcIter_{has: func() bool { return i < n }, nxt: func() any {
		x := v.m.vals[i].self()
		i++
		adv()
		return x
	}}
}

type refMapKeysView_ struct{ m *HMap[*Var, *ReferenceCollection] }

func (v refMapKeysView_) iterator() Iterator {
	i := 0
	return &funcIter_{has: func() bool { return i < len(v.m.keys) }, nxt: func() any {
		x := v.m.keys[i]
		i++
		return obj_(x)
	}}
}

func (m *HMap[K, V]) has(k K) bool {
	i, _ := m.find(k)
	return i >= 0
}

// putAll is Map.putAll(other); the keys' stored hash codes are reused.
func (m *HMap[K, V]) putAll(o *HMap[K, V]) {
	if len(m.keys) == 0 && o.idx == nil {
		m.keys = append(m.keys[:0], o.keys...)
		m.hashes = append(m.hashes[:0], o.hashes...)
		m.vals = append(m.vals[:0], o.vals...)
		return
	}
	for i, k := range o.keys {
		if j := m.findH(k, o.hashes[i]); j >= 0 {
			m.vals[j] = o.vals[i]
		} else {
			m.insert(k, o.hashes[i], o.vals[i])
		}
	}
}

// findH is find for a key whose hash code is known.
func (m *HMap[K, V]) findH(k K, h int32) int {
	if m.idx == nil {
		for i, x := range m.keys {
			if x == k {
				return i
			}
		}
		for i, xh := range m.hashes {
			if xh == h && k.equals(any(m.keys[i])) {
				return i
			}
		}
		return -1
	}
	i, ok := m.idx[h]
	for ok {
		if x := m.keys[i]; x == k || k.equals(any(x)) {
			return int(i)
		}
		if m.next[i] == 0 {
			break
		}
		i = m.next[i] - 1
	}
	return -1
}

type hmeach_[K hkey_, V any] struct {
	m *HMap[K, V]
	i int
	k K
	v V
}

func (m *HMap[K, V]) each() hmeach_[K, V] { return hmeach_[K, V]{m: m} }
func (e *hmeach_[K, V]) next() bool {
	if m := e.m; e.i < len(m.keys) {
		e.k, e.v = m.keys[e.i], m.vals[e.i]
		e.i++
		return true
	}
	return false
}
func (e *hmeach_[K, V]) setValue(v V) { e.m.vals[e.i-1] = v }

// hmapEquals_ and hmapHash_ are AbstractMap.equals / hashCode.
func hmapEquals_[K hkey_, V comparable](a, b *HMap[K, V]) bool {
	if a == b {
		return true
	}
	if a == nil || b == nil || len(a.keys) != len(b.keys) {
		return false
	}
	for i, k := range a.keys {
		j := b.findH(k, a.hashes[i])
		if j < 0 || !objequals_(obj_(a.vals[i]), obj_(b.vals[j])) {
			return false
		}
	}
	return true
}

func hmapHash_[K hkey_, V comparable](a *HMap[K, V]) int32 {
	var h int32
	for i := range a.keys {
		h += a.hashes[i] ^ objhash_(obj_(a.vals[i]))
	}
	return h
}

// strSetAddAll_ is typedSet.addAll(erased Collection<String>).
func strSetAddAll_(s *OSet[string], c any) {
	it := foreach_(c)
	for it.next() {
		s.add(str_(it.v))
	}
}
