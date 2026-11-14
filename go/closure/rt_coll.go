package closure

// java.util collections over `any`.
//
// Elements and keys are compared the Java way: equals/hashCode when the value
// has them, identity otherwise (objequals_, objhash_). Hash maps and sets keep
// insertion order (the Java tree has been prepared not to depend on hash order).

import (
	"sort"
	"strings"
)

// ---- iteration helpers

type sliceIter_ struct {
	items *[]any
	i     int
	del   func(i int)
	list  *ArrayList // set instead of del for a plain list, to save the closure
}

func (it *sliceIter_) hasNext() bool { return it.i < len(*it.items) }

func (it *sliceIter_) next() any {
	if it.i >= len(*it.items) {
		panic(new_NoSuchElementException_0().self())
	}
	v := (*it.items)[it.i]
	it.i++
	return v
}

func (it *sliceIter_) remove() {
	if it.list != nil {
		it.i--
		it.list.remove(int32(it.i))
		return
	}
	if it.del == nil {
		panic(new_UnsupportedOperationException_0().self())
	}
	it.i--
	it.del(it.i)
}

// each_ is the cursor of a translated for-each loop:
//
//	for it := foreach_(c); it.next(); { x := it.v; ... }
//
// It is a local variable of the loop's function, so the loop allocates nothing when c is one
// of the runtime's own collections, which each_ reads in place exactly as their iterators do
// (position by position against the collection's current length, so that what the body appends
// is visited and the loop ends early if the body shortens the collection). Any other Iterable
// (a translated class, a subclass of ArrayList, a view) is walked through its iterator().
type each_ struct {
	v    any      // the current element
	src  *[]any   // the collection is one of: a slice (the list's own, read anew at each step),
	t    *htable_ // a hash table (keys, values or entries according to mode),
	it   Iterator // or something with an iterator
	i    int
	mode int8
}

func foreach_(c any) each_ {
	switch x := c.(type) {
	case *ArrayList:
		return each_{src: &x.items}
	case *ImmutableCollection:
		if x.kind == icMultiset_ {
			x.elems_()
			return each_{src: &x.all_}
		}
		return each_{src: &x.items}
	case *LinkedHashSet:
		return each_{t: x.t(), mode: hkeys_}
	case *mapKeys_:
		return each_{t: x.t(), mode: hkeys_}
	case *mapValues_:
		return each_{t: x.t(), mode: hvals_}
	case *mapEntries_:
		return each_{t: x.t(), mode: hentries_}
	case nil:
		panic(new_NullPointerException_0().self())
	}
	return each_{it: c.(Iterable).iterator()}
}

// next is small so that the slice case is inlined into the loop.
func (e *each_) next() bool {
	if s := e.src; s != nil && e.i < len(*s) {
		e.v = (*s)[e.i]
		e.i++
		return true
	}
	return e.more()
}

func (e *each_) more() bool {
	if e.src != nil {
		return false
	}
	if t := e.t; t != nil {
		for e.i < len(t.keys) {
			i := e.i
			e.i++
			if k := t.keys[i]; k != dead_ {
				switch e.mode {
				case hkeys_:
					e.v = k
				case hvals_:
					e.v = t.vals[i]
				default:
					e.v = t.entry(i)
				}
				return true
			}
		}
		return false
	}
	if e.it.hasNext() {
		e.v = e.it.next()
		return true
	}
	return false
}

type funcIter_ struct {
	has func() bool
	nxt func() any
	del func()
}

func (it *funcIter_) hasNext() bool { return it.has() }
func (it *funcIter_) next() any     { return it.nxt() }
func (it *funcIter_) remove() {
	if it.del == nil {
		panic(new_UnsupportedOperationException_0().self())
	}
	it.del()
}

func Iterator_remove(it Iterator) {
	r, ok := it.(interface{ remove() })
	if !ok {
		panic(new_UnsupportedOperationException_String("remove").self())
	}
	r.remove()
}

func Iterable_forEach(it Iterable, c Consumer) {
	for i := it.iterator(); i.hasNext(); {
		c.accept(i.next())
	}
}

func toSlice_(c Iterable) []any {
	switch x := c.(type) {
	case *ArrayList:
		return x.items
	case interface{ toArray_0() []any }:
		return x.toArray_0()
	}
	var out []any
	for i := c.iterator(); i.hasNext(); {
		out = append(out, i.next())
	}
	return out
}

func collToString_(c Iterable) string {
	var sb strings.Builder
	sb.WriteByte('[')
	first := true
	for i := c.iterator(); i.hasNext(); {
		if !first {
			sb.WriteString(", ")
		}
		first = false
		sb.WriteString(objstr_(i.next()))
	}
	sb.WriteByte(']')
	return sb.String()
}

func listEquals_(a []any, o any) bool {
	l, ok := o.(List)
	if !ok || l == nil {
		return false
	}
	if int(l.size()) != len(a) {
		return false
	}
	i := 0
	for it := l.iterator(); it.hasNext(); i++ {
		if !objequals_(a[i], it.next()) {
			return false
		}
	}
	return true
}

func listHash_(a []any) int32 {
	var h int32 = 1
	for _, v := range a {
		h = 31*h + objhash_(v)
	}
	return h
}

func setEquals_(s Collection, o any) bool {
	t, ok := o.(Set)
	if !ok || t == nil {
		return false
	}
	if t.size() != s.size() {
		return false
	}
	for it := t.iterator(); it.hasNext(); {
		if !s.contains(it.next()) {
			return false
		}
	}
	return true
}

func setHash_(s Iterable) int32 {
	var h int32
	for it := s.iterator(); it.hasNext(); {
		h += objhash_(it.next())
	}
	return h
}

// fillArray_ is Collection.toArray(T[]): the argument only supplies the type in
// Java; here every element array is []any.
func fillArray_(items []any, a []any) []any {
	if len(a) >= len(items) {
		copy(a, items)
		if len(a) > len(items) {
			a[len(items)] = nil
		}
		return a
	}
	return append([]any(nil), items...)
}

// ---- ArrayList

type ArrayList struct {
	self_ any
	items []any
}

func (this *ArrayList) as_ArrayList() *ArrayList { return this }

func to_ArrayList(v any) *ArrayList {
	if v == nil {
		return nil
	}
	return v.(interface{ as_ArrayList() *ArrayList }).as_ArrayList()
}

func (this *ArrayList) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *ArrayList) init_ArrayList_0()          {}
func (this *ArrayList) init_ArrayList_int(n int32) { this.items = make([]any, 0, n) }
func (this *ArrayList) init_ArrayList_Collection(c Collection) {
	this.items = append([]any(nil), toSlice_(c)...)
}

func new_ArrayList_0() *ArrayList {
	this := &ArrayList{}
	this.self_ = this
	return this
}

func new_ArrayList_int(n int32) *ArrayList {
	this := &ArrayList{items: make([]any, 0, n)}
	this.self_ = this
	return this
}

func new_ArrayList_Collection(c Collection) *ArrayList {
	this := new_ArrayList_0()
	this.init_ArrayList_Collection(c)
	return this
}

func listOf_(items []any) *ArrayList {
	this := &ArrayList{items: items}
	this.self_ = this
	return this
}

func (this *ArrayList) size() int32   { return int32(len(this.items)) }
func (this *ArrayList) isEmpty() bool { return len(this.items) == 0 }
func (this *ArrayList) get(i int32) any {
	if i < 0 || int(i) >= len(this.items) {
		panic(new_IndexOutOfBoundsException_String("Index " + istr_(int64(i)) + " out of bounds for length " + istr_(int64(len(this.items)))).self())
	}
	return this.items[i]
}

func (this *ArrayList) set(i int32, v any) any {
	old := this.items[i]
	this.items[i] = v
	return old
}

func (this *ArrayList) add(v any) bool {
	this.items = append(this.items, v)
	return true
}

func (this *ArrayList) add_int_Object(i int32, v any) {
	if i < 0 || int(i) > len(this.items) {
		panic(new_IndexOutOfBoundsException_String("Index: " + istr_(int64(i)) + ", Size: " + istr_(int64(len(this.items)))).self())
	}
	this.items = append(this.items, nil)
	copy(this.items[i+1:], this.items[i:])
	this.items[i] = v
}

func (this *ArrayList) addAll(c Collection) bool {
	src := toSlice_(c)
	this.items = append(this.items, src...)
	return len(src) > 0
}

func (this *ArrayList) clear() { this.items = this.items[:0] }

func (this *ArrayList) indexOf(v any) int32 {
	for i, x := range this.items {
		if objequals_(v, x) {
			return int32(i)
		}
	}
	return -1
}

func (this *ArrayList) lastIndexOf(v any) int32 {
	for i := len(this.items) - 1; i >= 0; i-- {
		if objequals_(v, this.items[i]) {
			return int32(i)
		}
	}
	return -1
}

func (this *ArrayList) contains(v any) bool { return this.indexOf(v) >= 0 }

func (this *ArrayList) contains_s(v string) bool {
	for _, x := range this.items {
		if s, ok := x.(string); ok && s == v {
			return true
		}
	}
	return false
}

func (this *ArrayList) containsAll(c Collection) bool {
	for it := c.iterator(); it.hasNext(); {
		if !this.contains(it.next()) {
			return false
		}
	}
	return true
}

func (this *ArrayList) remove(i int32) any {
	old := this.get(i)
	if i == 0 {
		// Front removal is what queues do all the time: step over the slot instead of shifting.
		this.items[0] = nil
		this.items = this.items[1:]
		return old
	}
	copy(this.items[i:], this.items[i+1:])
	this.items[len(this.items)-1] = nil
	this.items = this.items[:len(this.items)-1]
	return old
}

func (this *ArrayList) remove_Object(v any) bool {
	i := this.indexOf(v)
	if i < 0 {
		return false
	}
	this.remove(i)
	return true
}

func (this *ArrayList) filter(keep func(any) bool) bool {
	out := this.items[:0]
	for _, x := range this.items {
		if keep(x) {
			out = append(out, x)
		}
	}
	changed := len(out) != len(this.items)
	for i := len(out); i < len(this.items); i++ {
		this.items[i] = nil
	}
	this.items = out
	return changed
}

func (this *ArrayList) removeAll(c Collection) bool {
	return this.filter(func(x any) bool { return !c.contains(x) })
}

func (this *ArrayList) iterator() Iterator {
	return &sliceIter_{items: &this.items, list: this}
}

func (this *ArrayList) listIterator(i int32) ListIterator {
	return &sliceIter_{items: &this.items, i: int(i), list: this}
}

func (this *ArrayList) subList(a int32, b int32) List {
	if a < 0 || b > this.size() || a > b {
		panic(new_IndexOutOfBoundsException_String("fromIndex: " + istr_(int64(a)) + ", toIndex: " + istr_(int64(b)) + ", size: " + istr_(int64(len(this.items)))).self())
	}
	return listOf_(this.items[a:b:b])
}

func (this *ArrayList) toArray_0() []any      { return append([]any(nil), this.items...) }
func (this *ArrayList) toArray(a []any) []any { return fillArray_(this.items, a) }
func (this *ArrayList) equals(o any) bool     { return listEquals_(this.items, o) }
func (this *ArrayList) hashCode() int32       { return listHash_(this.items) }
func (this *ArrayList) toString() string      { return collToString_(this) }
func (this *ArrayList) trimToSize()           {}

// LinkedList and CopyOnWriteArrayList behave as lists backed by a slice.
type LinkedList struct{ ArrayList }

func new_LinkedList() *LinkedList {
	this := &LinkedList{}
	this.self_ = this
	return this
}

func (this *LinkedList) peek() any {
	if len(this.items) == 0 {
		return nil
	}
	return this.items[0]
}
func (this *LinkedList) peekFirst() any { return this.peek() }
func (this *LinkedList) peekLast() any {
	if len(this.items) == 0 {
		return nil
	}
	return this.items[len(this.items)-1]
}
func (this *LinkedList) poll() any {
	if len(this.items) == 0 {
		return nil
	}
	return this.remove(0)
}
func (this *LinkedList) pollFirst() any { return this.poll() }
func (this *LinkedList) pop() any {
	if len(this.items) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return this.remove(0)
}
func (this *LinkedList) remove_0() any { return this.pop() }
func (this *LinkedList) push(v any)    { this.add_int_Object(0, v) }
func (this *LinkedList) descendingIterator() Iterator {
	rev := make([]any, len(this.items))
	for i, v := range this.items {
		rev[len(rev)-1-i] = v
	}
	return &sliceIter_{items: &rev}
}

type CopyOnWriteArrayList struct{ ArrayList }

func new_CopyOnWriteArrayList() *CopyOnWriteArrayList {
	this := &CopyOnWriteArrayList{}
	this.self_ = this
	return this
}

func (this *CopyOnWriteArrayList) iterator() Iterator {
	snapshot := append([]any(nil), this.items...)
	return &sliceIter_{items: &snapshot}
}

// ---- ArrayDeque

type ArrayDeque struct{ ArrayList }

func new_ArrayDeque() *ArrayDeque {
	this := &ArrayDeque{}
	this.self_ = this
	return this
}

func new_ArrayDeque_int(n int32) *ArrayDeque { return new_ArrayDeque() }

func new_ArrayDeque_Collection(c Collection) *ArrayDeque {
	this := new_ArrayDeque()
	this.addAll(c)
	return this
}

func (this *ArrayDeque) peek() any {
	if len(this.items) == 0 {
		return nil
	}
	return this.items[0]
}
func (this *ArrayDeque) peekFirst() any { return this.peek() }
func (this *ArrayDeque) peekLast() any {
	if len(this.items) == 0 {
		return nil
	}
	return this.items[len(this.items)-1]
}
func (this *ArrayDeque) poll() any {
	if len(this.items) == 0 {
		return nil
	}
	v := this.items[0]
	this.items[0] = nil
	this.items = this.items[1:]
	return v
}
func (this *ArrayDeque) pollFirst() any { return this.poll() }
func (this *ArrayDeque) pop() any {
	if len(this.items) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return this.poll()
}
func (this *ArrayDeque) remove_0() any { return this.pop() }
func (this *ArrayDeque) push(v any)    { this.add_int_Object(0, v) }
func (this *ArrayDeque) descendingIterator() Iterator {
	rev := make([]any, len(this.items))
	for i, v := range this.items {
		rev[len(rev)-1-i] = v
	}
	return &sliceIter_{items: &rev}
}

func SequencedCollection_addFirst(c SequencedCollection, v any) {
	c.(interface{ add_int_Object(int32, any) }).add_int_Object(0, v)
}
func SequencedCollection_addLast(c SequencedCollection, v any) { c.add(v) }
func SequencedCollection_getFirst(c SequencedCollection) any {
	it := c.iterator()
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	return it.next()
}
func SequencedCollection_getLast(c SequencedCollection) any {
	if l, ok := c.(List); ok {
		n := l.size()
		if n == 0 {
			panic(new_NoSuchElementException_0().self())
		}
		return l.get(n - 1)
	}
	s := toSlice_(c)
	if len(s) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return s[len(s)-1]
}
func SequencedCollection_removeFirst(c SequencedCollection) any {
	v := SequencedCollection_getFirst(c)
	if l, ok := c.(interface{ remove(int32) any }); ok {
		l.remove(0)
	} else {
		c.remove_Object(v)
	}
	return v
}
func SequencedCollection_removeLast(c SequencedCollection) any {
	v := SequencedCollection_getLast(c)
	if l, ok := c.(interface{ remove(int32) any }); ok {
		l.remove(c.size() - 1)
	} else {
		c.remove_Object(v)
	}
	return v
}

// ---- hash table core shared by the map and set types
//
// A table is its keys in insertion order (keys) with the values beside them (vals; a table
// used as a set has none), plus an index from key to position. Nothing is allocated per
// entry: a removed entry leaves the marker dead_ in keys until the table is compacted.
// A small table has no index at all and is searched linearly (see hsmall_).
// Map.Entry objects exist only for the entries that an entrySet iteration has handed out;
// the table remembers them (ents) so that a mapping keeps one entry object and that object
// sees later writes, as in java.util.HashMap.

// hentry_ is a Map.Entry that owns its key and value (TreeMap's entries, SimpleImmutableEntry).
type hentry_ struct {
	key, val any
}

func (e *hentry_) getKey() any   { return e.key }
func (e *hentry_) getValue() any { return e.val }
func (e *hentry_) setValue(v any) any {
	old := e.val
	e.val = v
	return old
}
func (e *hentry_) equals(o any) bool {
	x, ok := o.(Map_Entry)
	return ok && x != nil && objequals_(e.key, x.getKey()) && objequals_(e.val, x.getValue())
}
func (e *hentry_) hashCode() int32  { return objhash_(e.key) ^ objhash_(e.val) }
func (e *hentry_) toString() string { return objstr_(e.key) + "=" + objstr_(e.val) }

// hmapEntry_ is the entry object of one mapping of a hash map. val mirrors the table's value
// while the mapping exists (t != nil); after its removal the entry keeps the last value.
type hmapEntry_ struct {
	hentry_
	t *htable_
}

func (e *hmapEntry_) setValue(v any) any {
	old := e.val
	e.val = v
	if e.t != nil {
		e.t.vals[e.t.find(e.key)] = v
	}
	return old
}

type hashed_ interface {
	hashCode() int32
	equals(any) bool
}

type hdead_ struct{ _ int8 }

// dead_ stands in keys for a removed entry.
var dead_ any = &hdead_{}

type htable_ struct {
	keys     []any
	vals     []any    // beside keys; nil while the table is used as a set
	x        *hindex_ // nil until needed
	live     int32
	identity bool
	indexed  bool // x holds the index of all keys
}

// hsmall_ is the number of entries (removed ones included) up to which a table is searched
// linearly instead of through an index. Such a table holds only strings and keys compared by
// identity or Go equality: a key with its own equals/hashCode gets the table indexed at once,
// so that it is found exactly when its hash code and equals say so, as in Java. Most tables
// of the compiler stay this small, and for them a scan is faster than hashing as well as free
// of allocations (measured on the bundle benchmark with thresholds 0, 4, 8 and 16).
const hsmall_ = 8

type hindex_ struct {
	strs   map[string]int32  // string keys
	prim   map[any]int32     // keys compared by identity or by Go equality
	custom map[int32][]int32 // keys with their own hashCode/equals: hash -> positions
	ents   []*hmapEntry_     // beside keys once entrySet has handed out an entry; else nil
}

// customKey_ returns k as a key with its own hashCode/equals, if the table should use them.
func (t *htable_) customKey_(k any) (hashed_, bool) {
	if t.identity {
		return nil, false
	}
	switch k.(type) {
	case int32, int64, uint16, bool, nil:
		return nil, false
	}
	h, ok := k.(hashed_)
	return h, ok
}

// find returns the position of key k, or -1.
func (t *htable_) find(k any) int {
	if s, ok := k.(string); ok {
		return t.findStr(s)
	}
	if !t.indexed {
		// No key here has its own equals, so one that does is not found either.
		for i, x := range t.keys {
			if x == k {
				return i
			}
		}
		return -1
	}
	x := t.x
	if h, ok := t.customKey_(k); ok {
		for _, i := range x.custom[h.hashCode()] {
			if h.equals(t.keys[i]) {
				return int(i)
			}
		}
		return -1
	}
	if i, ok := x.prim[k]; ok {
		return int(i)
	}
	return -1
}

// findStr is find for a string key that is not boxed.
func (t *htable_) findStr(s string) int {
	if !t.indexed {
		for i, x := range t.keys {
			if xs, ok := x.(string); ok && xs == s {
				return i
			}
		}
		return -1
	}
	if i, ok := t.x.strs[s]; ok {
		return int(i)
	}
	return -1
}

// setAt replaces the value at position i.
func (t *htable_) setAt(i int, v any) (old any) {
	old = t.vals[i]
	t.vals[i] = v
	if x := t.x; x != nil && x.ents != nil && x.ents[i] != nil {
		x.ents[i].val = v
	}
	return old
}

// put is Map.put; add is Set.add. A table is used through one of the two only.
func (t *htable_) put(k, v any) (old any, existed bool) {
	if i := t.find(k); i >= 0 {
		return t.setAt(i, v), true
	}
	t.vals = append(t.vals, v)
	t.insert(k)
	return nil, false
}

// putStr is put for a string key; the key is boxed only if it is new.
func (t *htable_) putStr(s string, v any) (old any, existed bool) {
	if i := t.findStr(s); i >= 0 {
		return t.setAt(i, v), true
	}
	t.vals = append(t.vals, v)
	t.insertPlain(s)
	return nil, false
}

func (t *htable_) add(k any) bool {
	if t.find(k) >= 0 {
		return false
	}
	t.insert(k)
	return true
}

func (t *htable_) addStr(s string) bool {
	if t.findStr(s) >= 0 {
		return false
	}
	t.insertPlain(s)
	return true
}

// insert appends a key that find did not find.
func (t *htable_) insert(k any) {
	if !t.indexed {
		if _, isStr := k.(string); !isStr {
			if _, custom := t.customKey_(k); custom {
				t.buildIndex()
			}
		}
	}
	t.insertPlain(k)
}

// insertPlain is insert for a key without its own equals/hashCode.
func (t *htable_) insertPlain(k any) {
	i := len(t.keys)
	t.keys = append(t.keys, k)
	t.live++
	if x := t.x; x != nil && x.ents != nil {
		x.ents = append(x.ents, nil)
	}
	if t.indexed {
		t.index(k, int32(i))
	} else if i >= hsmall_ {
		t.buildIndex()
	}
}

func (t *htable_) buildIndex() {
	if t.x == nil {
		t.x = &hindex_{}
	}
	t.indexed = true
	for i, k := range t.keys {
		if k != dead_ {
			t.index(k, int32(i))
		}
	}
}

// index records that key k is at position i.
func (t *htable_) index(k any, i int32) {
	x := t.x
	if s, ok := k.(string); ok {
		if x.strs == nil {
			x.strs = map[string]int32{}
		}
		x.strs[s] = i
		return
	}
	if h, ok := t.customKey_(k); ok {
		if x.custom == nil {
			x.custom = map[int32][]int32{}
		}
		hc := h.hashCode()
		x.custom[hc] = append(x.custom[hc], i)
		return
	}
	if x.prim == nil {
		x.prim = map[any]int32{}
	}
	x.prim[k] = i
}

// unindex forgets that key k is at position i.
func (t *htable_) unindex(k any, i int) {
	x := t.x
	if s, ok := k.(string); ok {
		delete(x.strs, s)
	} else if h, ok := t.customKey_(k); ok {
		hc := h.hashCode()
		b := x.custom[hc]
		for j, p := range b {
			if int(p) == i {
				b = append(b[:j], b[j+1:]...)
				break
			}
		}
		if len(b) == 0 {
			delete(x.custom, hc)
		} else {
			x.custom[hc] = b
		}
	} else {
		delete(x.prim, k)
	}
}

// removeAt removes the entry at position i. Positions of the other entries change only if
// it reports true (the table was emptied or compacted).
func (t *htable_) removeAt(i int) (moved bool) {
	k := t.keys[i]
	if k == dead_ {
		return false
	}
	if t.indexed {
		t.unindex(k, i)
	}
	t.keys[i] = dead_
	if t.vals != nil {
		t.vals[i] = nil
	}
	if x := t.x; x != nil && x.ents != nil && x.ents[i] != nil {
		x.ents[i].t = nil
		x.ents[i] = nil
	}
	t.live--
	if t.live == 0 {
		// Empty: start again at position 0 (an index stays, empty, for the next entries).
		t.keys = t.keys[:0]
		if t.vals != nil {
			t.vals = t.vals[:0]
		}
		if t.x != nil {
			t.x.ents = nil
		}
		return true
	}
	if len(t.keys) > 32 && int(t.live)*2 < len(t.keys) {
		t.compact()
		return true
	}
	return false
}

// compact drops the removed entries; the order of the others is unchanged.
func (t *htable_) compact() {
	keys, vals := t.keys, t.vals
	var ents []*hmapEntry_
	if t.x != nil {
		ents = t.x.ents
	}
	t.keys = make([]any, 0, t.live)
	if vals != nil {
		t.vals = make([]any, 0, t.live)
	}
	t.x, t.indexed = nil, false
	if ents != nil {
		t.x = &hindex_{ents: make([]*hmapEntry_, 0, t.live)}
	}
	custom := false
	for i, k := range keys {
		if k == dead_ {
			continue
		}
		t.keys = append(t.keys, k)
		if vals != nil {
			t.vals = append(t.vals, vals[i])
		}
		if ents != nil {
			t.x.ents = append(t.x.ents, ents[i])
		}
		if _, c := t.customKey_(k); c {
			custom = true
		}
	}
	if custom || len(t.keys) > hsmall_ {
		t.buildIndex()
	}
}

func (t *htable_) remove(k any) (old any, existed bool) {
	return t.removeFound(t.find(k))
}

func (t *htable_) removeStr(s string) (old any, existed bool) {
	return t.removeFound(t.findStr(s))
}

func (t *htable_) removeFound(i int) (old any, existed bool) {
	if i < 0 {
		return nil, false
	}
	if t.vals != nil {
		old = t.vals[i]
	}
	t.removeAt(i)
	return old, true
}

func (t *htable_) clear() {
	if x := t.x; x != nil {
		for _, e := range x.ents {
			if e != nil {
				e.t = nil
			}
		}
	}
	t.keys, t.vals, t.x, t.live, t.indexed = nil, nil, nil, 0, false
}

// entry returns the entry object of the mapping at position i.
func (t *htable_) entry(i int) *hmapEntry_ {
	if t.x == nil {
		t.x = &hindex_{}
	}
	x := t.x
	if x.ents == nil {
		x.ents = make([]*hmapEntry_, len(t.keys), cap(t.keys))
	}
	e := x.ents[i]
	if e == nil {
		e = &hmapEntry_{hentry_{t.keys[i], t.vals[i]}, t}
		x.ents[i] = e
	}
	return e
}

// liveKeys returns a copy of the keys.
func (t *htable_) liveKeys() []any {
	if int(t.live) == len(t.keys) {
		return append(make([]any, 0, t.live), t.keys...)
	}
	out := make([]any, 0, t.live)
	for _, k := range t.keys {
		if k != dead_ {
			out = append(out, k)
		}
	}
	return out
}

func (t *htable_) liveVals() []any {
	out := make([]any, 0, t.live)
	for i, k := range t.keys {
		if k != dead_ {
			out = append(out, t.vals[i])
		}
	}
	return out
}

func (t *htable_) containsValue(v any) bool {
	for i, k := range t.keys {
		if k != dead_ && objequals_(v, t.vals[i]) {
			return true
		}
	}
	return false
}

const (
	hkeys_ int8 = iota
	hvals_
	hentries_
)

// hiter_ walks the live entries and yields keys, values or entries. Removing through the
// iterator may compact the table, after which the iterator finds its place again.
type hiter_ struct {
	t    *htable_
	i    int // next position to look at
	last int // position of the entry next returned, -1 if none or removed
	mode int8
}

func (t *htable_) iter(mode int8) Iterator { return &hiter_{t: t, mode: mode, last: -1} }

func (it *hiter_) hasNext() bool {
	keys := it.t.keys
	for it.i < len(keys) && keys[it.i] == dead_ {
		it.i++
	}
	return it.i < len(keys)
}

func (it *hiter_) next() any {
	if !it.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	i := it.i
	it.last = i
	it.i++
	switch it.mode {
	case hkeys_:
		return it.t.keys[i]
	case hvals_:
		return it.t.vals[i]
	}
	return it.t.entry(i)
}

func (it *hiter_) remove() {
	t, last := it.t, it.last
	if last < 0 || last >= len(t.keys) || t.keys[last] == dead_ {
		return
	}
	it.last = -1
	// The live entries before the cursor, which is where the cursor is after a compaction.
	before := 0
	for j := 0; j < it.i && j < len(t.keys); j++ {
		if j != last && t.keys[j] != dead_ {
			before++
		}
	}
	if t.removeAt(last) {
		it.i = before
	}
}

// ---- LinkedHashMap (and HashMap, ConcurrentHashMap, IdentityHashMap)

type LinkedHashMap struct {
	t htable_
}

func new_LinkedHashMap() *LinkedHashMap { return &LinkedHashMap{} }

func new_LinkedHashMap_Map(m Map) *LinkedHashMap {
	this := &LinkedHashMap{}
	this.putAll(m)
	return this
}

func (this *LinkedHashMap) size() int32   { return this.t.live }
func (this *LinkedHashMap) isEmpty() bool { return this.t.live == 0 }
func (this *LinkedHashMap) clear()        { this.t.clear() }

func (this *LinkedHashMap) get(k any) any {
	if i := this.t.find(k); i >= 0 {
		return this.t.vals[i]
	}
	return nil
}

func (this *LinkedHashMap) get_s(k string) any {
	if i := this.t.findStr(k); i >= 0 {
		return this.t.vals[i]
	}
	return nil
}

func (this *LinkedHashMap) containsKey(k any) bool   { return this.t.find(k) >= 0 }
func (this *LinkedHashMap) containsValue(v any) bool { return this.t.containsValue(v) }

func (this *LinkedHashMap) put(k any, v any) any {
	old, _ := this.t.put(k, v)
	return old
}

func (this *LinkedHashMap) putAll(m Map) {
	if h := hashMapOf_(m); h != nil {
		for i, k := range h.t.keys {
			if k != dead_ {
				this.t.put(k, h.t.vals[i])
			}
		}
		return
	}
	for it := m.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		this.t.put(e.getKey(), e.getValue())
	}
}

func (this *LinkedHashMap) remove(k any) any {
	old, _ := this.t.remove(k)
	return old
}

// The three views are the table itself under another type: making one allocates nothing.
func (this *LinkedHashMap) keySet() Set        { return (*mapKeys_)(&this.t) }
func (this *LinkedHashMap) values() Collection { return (*mapValues_)(&this.t) }
func (this *LinkedHashMap) entrySet() Set      { return (*mapEntries_)(&this.t) }
func (this *LinkedHashMap) equals(o any) bool  { return mapEquals_(this, o) }
func (this *LinkedHashMap) hashCode() int32    { return mapHash_(this) }
func (this *LinkedHashMap) toString() string   { return mapToString_(this) }

func mapEquals_(m Map, o any) bool {
	x, ok := o.(Map)
	if !ok || x == nil || x.size() != m.size() {
		return false
	}
	for it := m.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		if !x.containsKey(e.getKey()) || !objequals_(e.getValue(), x.get(e.getKey())) {
			return false
		}
	}
	return true
}

func mapHash_(m Map) int32 {
	var h int32
	for it := m.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		h += objhash_(e.getKey()) ^ objhash_(e.getValue())
	}
	return h
}

func mapToString_(m Map) string {
	var sb strings.Builder
	sb.WriteByte('{')
	first := true
	for it := m.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		if !first {
			sb.WriteString(", ")
		}
		first = false
		sb.WriteString(objstr_(e.getKey()) + "=" + objstr_(e.getValue()))
	}
	sb.WriteByte('}')
	return sb.String()
}

type HashMap struct{ LinkedHashMap }

type ConcurrentHashMap struct{ LinkedHashMap }

func new_ConcurrentHashMap() *ConcurrentHashMap            { return &ConcurrentHashMap{} }
func new_ConcurrentHashMap_int(n int32) *ConcurrentHashMap { return &ConcurrentHashMap{} }

type IdentityHashMap struct{ LinkedHashMap }

func new_IdentityHashMap() *IdentityHashMap {
	this := &IdentityHashMap{}
	this.t.identity = true
	return this
}

// viewBase_ supplies the Collection methods of a map view from three functions.
type viewBase_ struct{}

type mapKeys_ htable_

func (v *mapKeys_) t() *htable_              { return (*htable_)(v) }
func (v *mapKeys_) iterator() Iterator       { return v.t().iter(hkeys_) }
func (v *mapKeys_) size() int32              { return v.live }
func (v *mapKeys_) isEmpty() bool            { return v.live == 0 }
func (v *mapKeys_) contains(k any) bool      { return v.t().find(k) >= 0 }
func (v *mapKeys_) add(k any) bool           { panic(new_UnsupportedOperationException_0().self()) }
func (v *mapKeys_) addAll(c Collection) bool { panic(new_UnsupportedOperationException_0().self()) }
func (v *mapKeys_) clear()                   { v.t().clear() }
func (v *mapKeys_) remove_Object(k any) bool {
	_, ok := v.t().remove(k)
	return ok
}
func (v *mapKeys_) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *mapKeys_) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *mapKeys_) toArray_0() []any              { return v.t().liveKeys() }
func (v *mapKeys_) toArray(a []any) []any         { return fillArray_(v.t().liveKeys(), a) }
func (v *mapKeys_) equals(o any) bool             { return setEquals_(v, o) }
func (v *mapKeys_) hashCode() int32               { return setHash_(v) }
func (v *mapKeys_) toString() string              { return collToString_(v) }

type mapValues_ htable_

func (v *mapValues_) t() *htable_              { return (*htable_)(v) }
func (v *mapValues_) iterator() Iterator       { return v.t().iter(hvals_) }
func (v *mapValues_) size() int32              { return v.live }
func (v *mapValues_) isEmpty() bool            { return v.live == 0 }
func (v *mapValues_) contains(x any) bool      { return v.t().containsValue(x) }
func (v *mapValues_) add(k any) bool           { panic(new_UnsupportedOperationException_0().self()) }
func (v *mapValues_) addAll(c Collection) bool { panic(new_UnsupportedOperationException_0().self()) }
func (v *mapValues_) clear()                   { v.t().clear() }
func (v *mapValues_) remove_Object(x any) bool {
	for i, k := range v.keys {
		if k != dead_ && objequals_(x, v.vals[i]) {
			v.t().removeAt(i)
			return true
		}
	}
	return false
}
func (v *mapValues_) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *mapValues_) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *mapValues_) toArray_0() []any              { return v.t().liveVals() }
func (v *mapValues_) toArray(a []any) []any         { return fillArray_(v.t().liveVals(), a) }
func (v *mapValues_) toString() string              { return collToString_(v) }

type mapEntries_ htable_

func (v *mapEntries_) t() *htable_        { return (*htable_)(v) }
func (v *mapEntries_) iterator() Iterator { return v.t().iter(hentries_) }
func (v *mapEntries_) size() int32        { return v.live }
func (v *mapEntries_) isEmpty() bool      { return v.live == 0 }
func (v *mapEntries_) contains(x any) bool {
	e, ok := x.(Map_Entry)
	if !ok || e == nil {
		return false
	}
	i := v.t().find(e.getKey())
	return i >= 0 && objequals_(v.vals[i], e.getValue())
}
func (v *mapEntries_) add(k any) bool           { panic(new_UnsupportedOperationException_0().self()) }
func (v *mapEntries_) addAll(c Collection) bool { panic(new_UnsupportedOperationException_0().self()) }
func (v *mapEntries_) clear()                   { v.t().clear() }
func (v *mapEntries_) remove_Object(x any) bool {
	if !v.contains(x) {
		return false
	}
	v.t().remove(x.(Map_Entry).getKey())
	return true
}
func (v *mapEntries_) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *mapEntries_) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *mapEntries_) toArray_0() []any {
	out := make([]any, 0, v.live)
	for i, k := range v.keys {
		if k != dead_ {
			out = append(out, v.t().entry(i))
		}
	}
	return out
}
func (v *mapEntries_) toArray(a []any) []any { return fillArray_(v.toArray_0(), a) }
func (v *mapEntries_) equals(o any) bool     { return setEquals_(v, o) }
func (v *mapEntries_) hashCode() int32       { return setHash_(v) }
func (v *mapEntries_) toString() string      { return collToString_(v) }

func containsAll_(s Collection, c Collection) bool {
	for it := c.iterator(); it.hasNext(); {
		if !s.contains(it.next()) {
			return false
		}
	}
	return true
}

func removeAll_(s Collection, c Collection) bool {
	changed := false
	for it := s.iterator(); it.hasNext(); {
		if c.contains(it.next()) {
			Iterator_remove(it)
			changed = true
		}
	}
	return changed
}

// ---- LinkedHashSet (and HashSet): a table without values, seen through its key view

type LinkedHashSet struct {
	mapKeys_
}

func new_LinkedHashSet() *LinkedHashSet { return &LinkedHashSet{} }

func new_LinkedHashSet_Collection(c Collection) *LinkedHashSet {
	this := new_LinkedHashSet()
	this.addAll(c)
	return this
}

func (this *LinkedHashSet) add(k any) bool { return this.t().add(k) }

func (this *LinkedHashSet) addAll(c Collection) bool {
	changed := false
	for it := c.iterator(); it.hasNext(); {
		if this.add(it.next()) {
			changed = true
		}
	}
	return changed
}

func (this *LinkedHashSet) equals(o any) bool { return setEquals_(this, o) }
func (this *LinkedHashSet) hashCode() int32   { return setHash_(this) }
func (this *LinkedHashSet) toString() string  { return collToString_(this) }

type HashSet struct{ LinkedHashSet }

func identitySet_() *LinkedHashSet {
	this := new_LinkedHashSet()
	this.identity = true
	return this
}

// ---- ordering

func compareNatural_(a, b any) int32 {
	switch x := a.(type) {
	case string:
		return String_compareTo(x, b.(string))
	case int32:
		return Integer_compare(x, b.(int32))
	case int64:
		return Long_compare(x, b.(int64))
	case uint16:
		return int32(x) - int32(b.(uint16))
	case float64:
		return Double_compare(x, b.(float64))
	case bool:
		y := b.(bool)
		if x == y {
			return 0
		}
		if x {
			return 1
		}
		return -1
	case interface{ compareTo(any) int32 }:
		return x.compareTo(b)
	}
	panic(new_ClassCastException_String(goTypeName_(a) + " is not Comparable").self())
}

type cmpFunc_ struct{ f func(a, b any) int32 }

func (c *cmpFunc_) compare(a, b any) int32 { return c.f(a, b) }

var naturalOrder_ Comparator = &cmpFunc_{compareNatural_}

func Comparator_naturalOrder() Comparator { return naturalOrder_ }

func Comparator_reversed(c Comparator) Comparator {
	return &cmpFunc_{func(a, b any) int32 { return c.compare(b, a) }}
}

func Comparator_comparing(f Function) Comparator {
	return &cmpFunc_{func(a, b any) int32 { return compareNatural_(f.apply(a), f.apply(b)) }}
}

func Comparator_comparing_Function_Comparator(f Function, c Comparator) Comparator {
	return &cmpFunc_{func(a, b any) int32 { return c.compare(f.apply(a), f.apply(b)) }}
}

func Comparator_comparingInt(f ToIntFunction) Comparator {
	return &cmpFunc_{func(a, b any) int32 { return Integer_compare(f.applyAsInt(a), f.applyAsInt(b)) }}
}

func Comparator_comparingLong(f ToLongFunction) Comparator {
	return &cmpFunc_{func(a, b any) int32 { return Long_compare(f.applyAsLong(a), f.applyAsLong(b)) }}
}

func Comparator_thenComparing_Comparator(c Comparator, d Comparator) Comparator {
	return &cmpFunc_{func(a, b any) int32 {
		if r := c.compare(a, b); r != 0 {
			return r
		}
		return d.compare(a, b)
	}}
}

func Comparator_thenComparing(c Comparator, f Function) Comparator {
	return Comparator_thenComparing_Comparator(c, Comparator_comparing(f))
}

func Comparator_thenComparingInt(c Comparator, f ToIntFunction) Comparator {
	return Comparator_thenComparing_Comparator(c, Comparator_comparingInt(f))
}

func Map_Entry_comparingByKey() Comparator {
	return &cmpFunc_{func(a, b any) int32 {
		return compareNatural_(a.(Map_Entry).getKey(), b.(Map_Entry).getKey())
	}}
}

func sortSlice_(s []any, c Comparator) {
	if c == nil {
		c = naturalOrder_
	}
	sort.SliceStable(s, func(i, j int) bool { return c.compare(s[i], s[j]) < 0 })
}

func List_sort(l List, c Comparator) {
	if a, ok := l.(interface{ as_ArrayList() *ArrayList }); ok {
		sortSlice_(a.as_ArrayList().items, c)
		return
	}
	s := toSlice_(l)
	s = append([]any(nil), s...)
	sortSlice_(s, c)
	for i, v := range s {
		l.set(int32(i), v)
	}
}

func Collections_sort(l List)                               { List_sort(l, nil) }
func Collections_sort_List_Comparator(l List, c Comparator) { List_sort(l, c) }

func Collections_reverse(l List) {
	n := l.size()
	for i, j := int32(0), n-1; i < j; i, j = i+1, j-1 {
		a, b := l.get(i), l.get(j)
		l.set(i, b)
		l.set(j, a)
	}
}

func Collections_fill(l List, v any) {
	for i := int32(0); i < l.size(); i++ {
		l.set(i, v)
	}
}

func Collections_emptyList() List          { return listOf_(nil) }
func Collections_emptyIterator() Iterator  { var e []any; return &sliceIter_{items: &e} }
func Collections_singletonList(v any) List { return listOf_([]any{v}) }
func Collections_nCopies(n int32, v any) List {
	items := make([]any, n)
	for i := range items {
		items[i] = v
	}
	return listOf_(items)
}

// The unmodifiable views are the collections themselves: nothing checks that
// they are not written through.
func Collections_unmodifiableCollection(c Collection) Collection { return c }
func Collections_unmodifiableList(l List) List                   { return l }
func Collections_unmodifiableMap(m Map) Map                      { return m }
func Collections_unmodifiableSet(s Set) Set                      { return s }

// ---- String keys without a box
//
// The translator calls these instead of the method when the key or element argument is a Java
// String (m.get(s) becomes Map_get_s(m, s)), so that a lookup does not first allocate an `any`
// for the string. The fast path is taken only when the receiver is exactly one of the runtime's
// own collections; for anything else (a translated class that implements Map or extends
// ArrayList, say, whose own methods must run) the key is boxed as before. The receiver arrives
// as `any` because converting to it is free for every static type the translator has in hand.
// A null String is the key nil, as with strobj_.

// hashMapOf_ is m as the runtime's hash map, or nil.
func hashMapOf_(m any) *LinkedHashMap {
	switch x := m.(type) {
	case *LinkedHashMap:
		return x
	case *ConcurrentHashMap:
		return &x.LinkedHashMap
	case *IdentityHashMap:
		return &x.LinkedHashMap
	}
	return nil
}

// hashSetOf_ is the table of c if c is the runtime's hash set or a key view of its hash map, or nil.
func hashSetOf_(c any) *htable_ {
	switch x := c.(type) {
	case *LinkedHashSet:
		if x != nil {
			return x.t()
		}
	case *mapKeys_:
		return x.t()
	}
	return nil
}

// asMap_ is the receiver of the boxed call; a null receiver is a NullPointerException, as it is
// for the method call that the helper replaces.
func asMap_(m any) Map {
	if m == nil {
		panic(new_NullPointerException_0().self())
	}
	return m.(Map)
}

func asColl_(c any) Collection {
	if c == nil {
		panic(new_NullPointerException_0().self())
	}
	return c.(Collection)
}

func Map_get_s(m any, k string) any {
	if k != nullstr_ {
		if h := hashMapOf_(m); h != nil {
			return h.get_s(k)
		}
		if im, ok := m.(*ImmutableMap); ok && im != nil {
			return im.get_s(k)
		}
	}
	return asMap_(m).get(strobj_(k))
}

func Map_containsKey_s(m any, k string) bool {
	if k != nullstr_ {
		if h := hashMapOf_(m); h != nil {
			return h.t.findStr(k) >= 0
		}
		if im, ok := m.(*ImmutableMap); ok && im != nil {
			return im.containsKey_s(k)
		}
	}
	return asMap_(m).containsKey(strobj_(k))
}

func Map_put_s(m any, k string, v any) any {
	if h := hashMapOf_(m); h != nil && k != nullstr_ {
		old, _ := h.t.putStr(k, v)
		return old
	}
	return asMap_(m).put(strobj_(k), v)
}

func Map_remove_s(m any, k string) any {
	if h := hashMapOf_(m); h != nil && k != nullstr_ {
		old, _ := h.t.removeStr(k)
		return old
	}
	return asMap_(m).remove(strobj_(k))
}

func Map_getOrDefault_s(m any, k string, def any) any {
	if v := Map_get_s(m, k); v != nil || Map_containsKey_s(m, k) {
		return v
	}
	return def
}

func Map_putIfAbsent_s(m any, k string, v any) any {
	if old := Map_get_s(m, k); old != nil {
		return old
	}
	Map_put_s(m, k, v)
	return nil
}

func Map_computeIfAbsent_s(m any, k string, f Function) any {
	if old := Map_get_s(m, k); old != nil {
		return old
	}
	v := f.apply(strobj_(k))
	if v != nil {
		Map_put_s(m, k, v)
	}
	return v
}

func Collection_contains_s(c any, k string) bool {
	if k != nullstr_ {
		if t := hashSetOf_(c); t != nil {
			return t.findStr(k) >= 0
		}
		switch x := c.(type) {
		case *ImmutableCollection:
			if x != nil {
				return x.contains_s(k)
			}
		case *ArrayList:
			if x != nil {
				return x.contains_s(k)
			}
		}
	}
	return asColl_(c).contains(strobj_(k))
}

// Collection_add_s is called for sets only (a list stores the box in any case).
func Collection_add_s(c any, k string) bool {
	if x, ok := c.(*LinkedHashSet); ok && x != nil && k != nullstr_ {
		return x.t().addStr(k)
	}
	return asColl_(c).add(strobj_(k))
}

func Collection_remove_s(c any, k string) bool {
	if t := hashSetOf_(c); t != nil && k != nullstr_ {
		_, existed := t.removeStr(k)
		return existed
	}
	return asColl_(c).remove_Object(strobj_(k))
}

// ---- Map defaults

func Map_getOrDefault(m Map, k any, def any) any {
	if v := m.get(k); v != nil || m.containsKey(k) {
		return v
	}
	return def
}

func Map_putIfAbsent(m Map, k any, v any) any {
	if old := m.get(k); old != nil {
		return old
	}
	m.put(k, v)
	return nil
}

func Map_computeIfAbsent(m Map, k any, f Function) any {
	if old := m.get(k); old != nil {
		return old
	}
	v := f.apply(k)
	if v != nil {
		m.put(k, v)
	}
	return v
}

func Map_merge(m Map, k any, v any, f BiFunction) any {
	old := m.get(k)
	nv := v
	if old != nil {
		nv = f.apply(old, v)
	}
	if nv == nil {
		m.remove(k)
	} else {
		m.put(k, nv)
	}
	return nv
}

func Map_forEach(m Map, f BiConsumer) {
	for it := m.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		f.accept(e.getKey(), e.getValue())
	}
}

func Map_replaceAll(m Map, f BiFunction) {
	for it := m.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		e.setValue(f.apply(e.getKey(), e.getValue()))
	}
}

func Map_replace(m Map, k any, oldV any, newV any) bool {
	if m.containsKey(k) && objequals_(m.get(k), oldV) {
		m.put(k, newV)
		return true
	}
	return false
}

type AbstractMap_SimpleImmutableEntry struct{ hentry_ }

// ---- Arrays

func Arrays_asList(a []any) List { return listOf_(a) }

func Arrays_binarySearch(a []int32, k int32) int32 {
	lo, hi := 0, len(a)-1
	for lo <= hi {
		mid := (lo + hi) >> 1
		if a[mid] < k {
			lo = mid + 1
		} else if a[mid] > k {
			hi = mid - 1
		} else {
			return int32(mid)
		}
	}
	return int32(-(lo + 1))
}

func Arrays_copyOf(a []int32, n int32) []int32 {
	out := make([]int32, n)
	copy(out, a)
	return out
}

// Arrays_copyOfRange copies a[from:to], padding with zeros past the end of a.
func Arrays_copyOfRange(a []int8, from int32, to int32) []int8 {
	if from > to {
		panic(new_IllegalArgumentException_String(istr_(int64(from)) + " > " + istr_(int64(to))).self())
	}
	if from < 0 || int(from) > len(a) {
		panic(new_ArrayIndexOutOfBoundsException_String("Array index out of range: " + istr_(int64(from))).self())
	}
	out := make([]int8, to-from)
	copy(out, a[from:])
	return out
}

func Arrays_copyOf_ObjectArr_int(a []any, n int32) []any {
	out := make([]any, n)
	copy(out, a)
	return out
}

func Arrays_equals(a []int32, b []int32) bool {
	if (a == nil) != (b == nil) || len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

func Arrays_fill(a []int32, v int32) {
	for i := range a {
		a[i] = v
	}
}

func Arrays_fill_intArr_int_int_int(a []int32, from int32, to int32, v int32) {
	for i := from; i < to; i++ {
		a[i] = v
	}
}

func Arrays_setAll(a []any, f IntFunction) {
	for i := range a {
		a[i] = f.apply(int32(i))
	}
}

func Arrays_sort(a []any)                                    { sortSlice_(a, nil) }
func Arrays_sort_ObjectArr_Comparator(a []any, c Comparator) { sortSlice_(a, c) }
func Arrays_sort_byteArr(a []int8)                           { sort.Slice(a, func(i, j int) bool { return a[i] < a[j] }) }
func Arrays_sort_intArr(a []int32)                           { sort.Slice(a, func(i, j int) bool { return a[i] < a[j] }) }
func Arrays_sort_longArr(a []int64)                          { sort.Slice(a, func(i, j int) bool { return a[i] < a[j] }) }

func Arrays_toString(a []uint16) string {
	parts := make([]string, len(a))
	for i, c := range a {
		parts[i] = String_valueOf_char(c)
	}
	return "[" + strings.Join(parts, ", ") + "]"
}

func Comparator_thenComparingLong(c Comparator, f ToLongFunction) Comparator {
	return Comparator_thenComparing_Comparator(c, Comparator_comparingLong(f))
}

func Collections_unmodifiableSortedSet(s SortedSet) SortedSet { return s }

func Map_compute(m Map, k any, f BiFunction) any {
	old := m.get(k)
	v := f.apply(k, old)
	if v == nil {
		if old != nil || m.containsKey(k) {
			m.remove(k)
		}
		return nil
	}
	m.put(k, v)
	return v
}

func Comparable_compareTo(a any, b any) int32 { return compareNatural_(a, b) }
