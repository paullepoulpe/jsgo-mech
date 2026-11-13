package closure

// Collection views computed from closures, and the cores of Guava's mutable
// multimaps (mmap_) and multisets (mset_). The public types (ArrayListMultimap,
// LinkedHashMultiset, ...) embed a core; see rt_guava_fwd.go.

// ---- viewColl_: a Collection (or Set) defined by an iterator factory

type viewColl_ struct {
	it  func() Iterator
	sz  func() int32   // optional: default counts the iteration
	has func(any) bool // optional: default scans the iteration
	rm  func(any) bool // optional: default removes through the iterator
	clr func()         // optional: default removes through the iterator
	set bool           // equals/hashCode as a java.util.Set (otherwise identity)
}

// ---- idxIter_: iterator / list iterator over an indexed list

type idxList_ interface {
	size() int32
	get(int32) any
	remove(int32) any
}

type idxIter_ struct {
	l    idxList_
	i    int32
	last int32
}

func (it *idxIter_) hasNext() bool { return it.i < it.l.size() }

func (it *idxIter_) next() any {
	if it.i >= it.l.size() {
		panic(new_NoSuchElementException_0().self())
	}
	v := it.l.get(it.i)
	it.last = it.i
	it.i++
	return v
}

func (it *idxIter_) remove() {
	if it.last < 0 {
		panic(new_IllegalStateException_0().self())
	}
	it.l.remove(it.last)
	if it.last < it.i {
		it.i--
	}
	it.last = -1
}

// ---- viewList_: a List defined by size and element functions

type viewList_ struct {
	sz    func() int32
	at    func(int32) any
	setf  func(int32, any) any // optional
	rmAt  func(int32) any      // optional
	addAt func(int32, any)     // optional
}

func (v *viewList_) size() int32   { return v.sz() }
func (v *viewList_) isEmpty() bool { return v.sz() == 0 }

func (v *viewList_) get(i int32) any {
	if n := v.sz(); i < 0 || i >= n {
		panic(new_IndexOutOfBoundsException_String("Index " + istr_(int64(i)) + " out of bounds for length " + istr_(int64(n))).self())
	}
	return v.at(i)
}

func (v *viewList_) set(i int32, x any) any {
	if v.setf == nil {
		throwUOE_()
	}
	return v.setf(i, x)
}

func (v *viewList_) add_int_Object(i int32, x any) {
	if v.addAt == nil {
		throwUOE_()
	}
	v.addAt(i, x)
}

func (v *viewList_) add(x any) bool {
	v.add_int_Object(v.sz(), x)
	return true
}

func (v *viewList_) addAll(c Collection) bool {
	s := toSlice_(c)
	for _, x := range s {
		v.add(x)
	}
	return len(s) > 0
}

func (v *viewList_) remove(i int32) any {
	if v.rmAt == nil {
		throwUOE_()
	}
	return v.rmAt(i)
}

func (v *viewList_) clear() {
	for i := v.sz() - 1; i >= 0; i-- {
		v.remove(i)
	}
}

func (v *viewList_) indexOf(x any) int32 {
	n := v.sz()
	for i := int32(0); i < n; i++ {
		if objequals_(x, v.at(i)) {
			return i
		}
	}
	return -1
}

func (v *viewList_) lastIndexOf(x any) int32 {
	for i := v.sz() - 1; i >= 0; i-- {
		if objequals_(x, v.at(i)) {
			return i
		}
	}
	return -1
}

func (v *viewList_) contains(x any) bool { return v.indexOf(x) >= 0 }

func (v *viewList_) remove_Object(x any) bool {
	i := v.indexOf(x)
	if i < 0 {
		return false
	}
	v.remove(i)
	return true
}

func (v *viewList_) iterator() Iterator                { return &idxIter_{l: v, last: -1} }
func (v *viewList_) listIterator(i int32) ListIterator { return &idxIter_{l: v, i: i, last: -1} }
func (v *viewList_) containsAll(c Collection) bool     { return containsAll_(v, c) }
func (v *viewList_) removeAll(c Collection) bool       { return removeAll_(v, c) }
func (v *viewList_) subList(a int32, b int32) List {
	if n := v.sz(); a < 0 || b > n || a > b {
		panic(new_IndexOutOfBoundsException_String("fromIndex: " + istr_(int64(a)) + ", toIndex: " + istr_(int64(b)) + ", size: " + istr_(int64(n))).self())
	}
	out := &viewList_{
		sz: func() int32 { return b - a },
		at: func(i int32) any { return v.at(a + i) },
	}
	if v.setf != nil {
		out.setf = func(i int32, x any) any { return v.setf(a+i, x) }
	}
	return out
}

func (v *viewList_) toArray_0() []any {
	n := v.sz()
	out := make([]any, n)
	for i := int32(0); i < n; i++ {
		out[i] = v.at(i)
	}
	return out
}

func (v *viewList_) toArray(a []any) []any { return fillArray_(v.toArray_0(), a) }
func (v *viewList_) equals(o any) bool     { return isListLike_(o) && listEquals_(v.toArray_0(), o) }
func (v *viewList_) hashCode() int32       { return listHash_(v.toArray_0()) }
func (v *viewList_) toString() string      { return collToString_(v) }

// flatIter_ chains the iterators mk yields for each element of outer; remove
// goes to the iterator that produced the last element.
func flatIter_(outer []any, mk func(k any) Iterator) Iterator {
	i := 0
	var cur, last Iterator
	advance := func() bool {
		for {
			if cur != nil && cur.hasNext() {
				return true
			}
			if i >= len(outer) {
				return false
			}
			cur = mk(outer[i])
			i++
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
}

func mapIter_(it Iterator, f func(any) any) Iterator {
	return &funcIter_{
		has: it.hasNext,
		nxt: func() any { return f(it.next()) },
		del: func() { Iterator_remove(it) },
	}
}

// ---- mmap_: the mutable multimap core

const (
	mmArrayList_ int8 = iota
	mmHashSet_
	mmTreeSet_
)

type mmap_ struct {
	m      Map  // key -> the key's values: *ArrayList, *LinkedHashSet or *TreeSet; never empty
	kind   int8 // kind of value collection
	n      int32
	linked bool         // LinkedListMultimap: values() and entries() follow global insertion order
	seq    []*immEntry_ // linked: every entry, in insertion order
}

// ---- the live value-collection views of a key

type mmSet_ struct {
	mm  *mmap_
	key any
}

type mmList_ struct{ mmSet_ }

// ---- Multimap.asMap()

type mmAsMap_ struct{ mm *mmap_ }

// ---- mset_: the mutable multiset core

type mset_ struct {
	m Map // element -> int32 count (> 0)
	n int32
}

func (mm *mmap_) size() int32 { return mm.n }

func (mm *mmap_) isEmpty() bool { return mm.n == 0 }

func (mm *mmap_) containsKey(k any) bool { return mm.m.containsKey(k) }

func (mm *mmap_) clear() {
	mm.m.clear()
	mm.n = 0
	mm.seq = nil
}

func (ms *mset_) size() int32 { return ms.n }

func (ms *mset_) isEmpty() bool { return ms.n == 0 }

func (ms *mset_) clear() {
	ms.m.clear()
	ms.n = 0
}
