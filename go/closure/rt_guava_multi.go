package closure

// Collection views computed from closures, and the cores of Guava's mutable
// multimaps (mmap_) and multisets (mset_). The public types (ArrayListMultimap,
// LinkedHashMultiset, ...) embed a core; see rt_guava_fwd.go.

import "strings"

// ---- viewColl_: a Collection (or Set) defined by an iterator factory

type viewColl_ struct {
	it  func() Iterator
	sz  func() int32   // optional: default counts the iteration
	has func(any) bool // optional: default scans the iteration
	rm  func(any) bool // optional: default removes through the iterator
	clr func()         // optional: default removes through the iterator
	set bool           // equals/hashCode as a java.util.Set (otherwise identity)
}

func (v *viewColl_) iterator() Iterator { return v.it() }

func (v *viewColl_) size() int32 {
	if v.sz != nil {
		return v.sz()
	}
	var n int32
	for it := v.it(); it.hasNext(); it.next() {
		n++
	}
	return n
}

func (v *viewColl_) isEmpty() bool {
	if v.sz != nil {
		return v.sz() == 0
	}
	return !v.it().hasNext()
}

func (v *viewColl_) contains(x any) bool {
	if v.has != nil {
		return v.has(x)
	}
	for it := v.it(); it.hasNext(); {
		if objequals_(x, it.next()) {
			return true
		}
	}
	return false
}

func (v *viewColl_) add(x any) bool           { throwUOE_(); return false }
func (v *viewColl_) addAll(c Collection) bool { throwUOE_(); return false }

func (v *viewColl_) clear() {
	if v.clr != nil {
		v.clr()
		return
	}
	for it := v.it(); it.hasNext(); {
		it.next()
		Iterator_remove(it)
	}
}

func (v *viewColl_) remove_Object(x any) bool {
	if v.rm != nil {
		return v.rm(x)
	}
	for it := v.it(); it.hasNext(); {
		if objequals_(x, it.next()) {
			Iterator_remove(it)
			return true
		}
	}
	return false
}

func (v *viewColl_) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *viewColl_) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *viewColl_) retainAll(c Collection) bool   { return retainAll_(v, c) }

func (v *viewColl_) toArray_0() []any {
	out := []any{}
	for it := v.it(); it.hasNext(); {
		out = append(out, it.next())
	}
	return out
}

func (v *viewColl_) toArray(a []any) []any { return fillArray_(v.toArray_0(), a) }

func (v *viewColl_) equals(o any) bool {
	if !v.set {
		return o == any(v)
	}
	return setLikeEquals_(v, o)
}

func (v *viewColl_) hashCode() int32 {
	if !v.set {
		return identityHash_(v)
	}
	return setHash_(v)
}

func (v *viewColl_) toString() string { return collToString_(v) }

// setLikeEquals_ is Set.equals for s, refusing lists and multisets (which
// satisfy the Go Set interface).
func setLikeEquals_(s Collection, o any) bool {
	if o == nil || isListLike_(o) || isMultiset_(o) {
		return false
	}
	if c, ok := o.(*ImmutableCollection); ok {
		return c.equals(s) || (int(s.size()) == len(c.items) && s.containsAll(c))
	}
	return setEquals_(s, o)
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

func (it *idxIter_) hasPrevious() bool { return it.i > 0 }
func (it *idxIter_) nextIndex() int32  { return it.i }
func (it *idxIter_) previous() any {
	if it.i <= 0 {
		panic(new_NoSuchElementException_0().self())
	}
	it.i--
	it.last = it.i
	return it.l.get(it.i)
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
func (v *viewList_) retainAll(c Collection) bool       { return retainAll_(v, c) }

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

func (mm *mmap_) init_(kind int8, sortedKeys bool, linked bool) {
	mm.kind, mm.linked = kind, linked
	if sortedKeys {
		mm.m = new_TreeMap()
	} else {
		mm.m = new_LinkedHashMap()
	}
}

func (mm *mmap_) coll_(k any) Collection {
	c := mm.m.get(k)
	if c == nil {
		return nil
	}
	return c.(Collection)
}

func (mm *mmap_) newColl_() Collection {
	switch mm.kind {
	case mmHashSet_:
		return new_LinkedHashSet()
	case mmTreeSet_:
		return new_TreeSet()
	}
	return new_ArrayList_0()
}

func (mm *mmap_) addVal_(k, v any) bool {
	c := mm.coll_(k)
	if c == nil {
		c = mm.newColl_()
		c.add(v)
		mm.m.put(k, c)
	} else if !c.add(v) {
		return false
	}
	mm.n++
	if mm.linked {
		mm.seq = append(mm.seq, &immEntry_{key: k, val: v})
	}
	return true
}

// removeAt_ removes the i-th value of a list-valued key.
func (mm *mmap_) removeAt_(k any, l *ArrayList, i int32) any {
	old := l.remove(i)
	mm.n--
	if l.isEmpty() {
		mm.m.remove(k)
	}
	if mm.linked {
		for j, e := range mm.seq {
			if objequals_(e.key, k) {
				if i == 0 {
					mm.seq = append(mm.seq[:j], mm.seq[j+1:]...)
					break
				}
				i--
			}
		}
	}
	return old
}

func (mm *mmap_) removeVal_(k, v any) bool {
	c := mm.coll_(k)
	if c == nil {
		return false
	}
	if l, ok := c.(*ArrayList); ok {
		i := l.indexOf(v)
		if i < 0 {
			return false
		}
		mm.removeAt_(k, l, i)
		return true
	}
	if !c.remove_Object(v) {
		return false
	}
	mm.n--
	if c.isEmpty() {
		mm.m.remove(k)
	}
	return true
}

// removeKey_ detaches and returns the values of k (nil if none).
func (mm *mmap_) removeKey_(k any) Collection {
	c := mm.coll_(k)
	if c == nil {
		return nil
	}
	mm.m.remove(k)
	mm.n -= c.size()
	if mm.linked {
		out := mm.seq[:0:0]
		for _, e := range mm.seq {
			if !objequals_(e.key, k) {
				out = append(out, e)
			}
		}
		mm.seq = out
	}
	return c
}

func (mm *mmap_) keys_() []any { return toSlice_(mm.m.keySet()) }

func (mm *mmap_) size() int32              { return mm.n }
func (mm *mmap_) isEmpty() bool            { return mm.n == 0 }
func (mm *mmap_) containsKey(k any) bool   { return mm.m.containsKey(k) }
func (mm *mmap_) put(k any, v any) bool    { return mm.addVal_(k, v) }
func (mm *mmap_) remove(k any, v any) bool { return mm.removeVal_(k, v) }

func (mm *mmap_) clear() {
	mm.m.clear()
	mm.n = 0
	mm.seq = nil
}

func (mm *mmap_) containsEntry(k any, v any) bool {
	c := mm.coll_(k)
	return c != nil && c.contains(v)
}

func (mm *mmap_) containsValue(v any) bool {
	for _, k := range mm.keys_() {
		if mm.coll_(k).contains(v) {
			return true
		}
	}
	return false
}

func (mm *mmap_) get(k any) Collection {
	if mm.kind == mmArrayList_ {
		return &mmList_{mmSet_{mm: mm, key: k}}
	}
	return &mmSet_{mm: mm, key: k}
}

func (mm *mmap_) putAll(k any, vs Iterable) bool {
	changed := false
	for _, v := range copySlice_(vs) {
		if mm.addVal_(k, v) {
			changed = true
		}
	}
	return changed
}

func (mm *mmap_) putAll_Multimap(o Multimap) bool {
	changed := false
	for _, x := range toSlice_(o.asMap().entrySet()) {
		e := x.(Map_Entry)
		if mm.putAll(e.getKey(), e.getValue().(Iterable)) {
			changed = true
		}
	}
	return changed
}

func (mm *mmap_) removeAll(k any) Collection {
	if c := mm.removeKey_(k); c != nil {
		return c
	}
	return mm.newColl_()
}

func (mm *mmap_) replaceValues(k any, vs Iterable) Collection {
	old := mm.removeAll(k)
	mm.putAll(k, vs)
	return old
}

func (mm *mmap_) keySet() Set {
	return &viewColl_{
		set: true,
		it: func() Iterator {
			keys := mm.keys_()
			si := &sliceIter_{items: &keys}
			return &funcIter_{has: si.hasNext, nxt: si.next, del: func() { mm.removeKey_(keys[si.i-1]) }}
		},
		sz:  func() int32 { return mm.m.size() },
		has: mm.containsKey,
		rm:  func(k any) bool { return mm.removeKey_(k) != nil },
		clr: mm.clear,
	}
}

func (mm *mmap_) seqIter_(pick func(*immEntry_) any) Iterator {
	snap := append([]*immEntry_(nil), mm.seq...)
	i := 0
	return &funcIter_{
		has: func() bool { return i < len(snap) },
		nxt: func() any {
			if i >= len(snap) {
				panic(new_NoSuchElementException_0().self())
			}
			i++
			return pick(snap[i-1])
		},
		del: func() {
			e := snap[i-1]
			var idx int32
			for _, x := range mm.seq {
				if x == e {
					mm.removeAt_(e.key, mm.coll_(e.key).(*ArrayList), idx)
					return
				}
				if objequals_(x.key, e.key) {
					idx++
				}
			}
		},
	}
}

func (mm *mmap_) values() Collection {
	return &viewColl_{
		it: func() Iterator {
			if mm.linked {
				return mm.seqIter_(func(e *immEntry_) any { return e.val })
			}
			return flatIter_(mm.keys_(), func(k any) Iterator { return mm.get(k).iterator() })
		},
		sz:  mm.size,
		clr: mm.clear,
	}
}

func (mm *mmap_) entries() Collection {
	return &viewColl_{
		set: mm.kind != mmArrayList_,
		it: func() Iterator {
			if mm.linked {
				return mm.seqIter_(func(e *immEntry_) any { return &immEntry_{key: e.key, val: e.val} })
			}
			return flatIter_(mm.keys_(), func(k any) Iterator {
				return mapIter_(mm.get(k).iterator(), func(v any) any { return &immEntry_{key: k, val: v} })
			})
		},
		sz: mm.size,
		has: func(x any) bool {
			e, ok := x.(Map_Entry)
			return ok && e != nil && mm.containsEntry(e.getKey(), e.getValue())
		},
		rm: func(x any) bool {
			e, ok := x.(Map_Entry)
			return ok && e != nil && mm.removeVal_(e.getKey(), e.getValue())
		},
		clr: mm.clear,
	}
}

// keys is a snapshot (Guava's is a live view).
func (mm *mmap_) keys() Multiset {
	ms := &mset_{}
	ms.init_(false)
	for _, k := range mm.keys_() {
		ms.add_Object_int(k, mm.coll_(k).size())
	}
	return ms
}

func (mm *mmap_) asMap() Map { return &mmAsMap_{mm: mm} }

func (mm *mmap_) equals(o any) bool {
	x, ok := o.(Multimap)
	if !ok || x == nil {
		return false
	}
	return objequals_(mm.asMap(), x.asMap())
}

func (mm *mmap_) hashCode() int32  { return mm.asMap().(*mmAsMap_).hashCode() }
func (mm *mmap_) toString() string { return mm.asMap().(*mmAsMap_).toString() }

// ---- the live value-collection views of a key

type mmSet_ struct {
	mm  *mmap_
	key any
}

func (v *mmSet_) size() int32 {
	if c := v.mm.coll_(v.key); c != nil {
		return c.size()
	}
	return 0
}

func (v *mmSet_) isEmpty() bool { return v.mm.coll_(v.key) == nil }

func (v *mmSet_) contains(x any) bool {
	c := v.mm.coll_(v.key)
	return c != nil && c.contains(x)
}

func (v *mmSet_) add(x any) bool { return v.mm.addVal_(v.key, x) }

func (v *mmSet_) addAll(c Collection) bool { return v.mm.putAll(v.key, c) }

func (v *mmSet_) clear() { v.mm.removeKey_(v.key) }

func (v *mmSet_) remove_Object(x any) bool { return v.mm.removeVal_(v.key, x) }

func (v *mmSet_) iterator() Iterator {
	c := v.mm.coll_(v.key)
	if c == nil {
		return Collections_emptyIterator()
	}
	if v.mm.linked {
		return &idxIter_{l: &mmList_{*v}, last: -1}
	}
	inner := c.iterator()
	return &funcIter_{
		has: inner.hasNext,
		nxt: inner.next,
		del: func() {
			Iterator_remove(inner)
			v.mm.n--
			if c.isEmpty() {
				v.mm.m.remove(v.key)
			}
		},
	}
}

func (v *mmSet_) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *mmSet_) removeAll(c Collection) bool   { return removeAll_(v, c) }
func (v *mmSet_) retainAll(c Collection) bool   { return retainAll_(v, c) }

func (v *mmSet_) toArray_0() []any {
	if c := v.mm.coll_(v.key); c != nil {
		return c.toArray_0()
	}
	return []any{}
}

func (v *mmSet_) toArray(a []any) []any { return fillArray_(v.toArray_0(), a) }
func (v *mmSet_) equals(o any) bool     { return o == any(v) || setLikeEquals_(v, o) }
func (v *mmSet_) hashCode() int32       { return setHash_(v) }
func (v *mmSet_) toString() string      { return collToString_(v) }

type mmList_ struct{ mmSet_ }

func (v *mmList_) list_() *ArrayList {
	if c := v.mm.coll_(v.key); c != nil {
		return c.(*ArrayList)
	}
	return nil
}

func (v *mmList_) outOfBounds_(i int32) {
	panic(new_IndexOutOfBoundsException_String("Index " + istr_(int64(i)) + " out of bounds for length " + istr_(int64(v.size()))).self())
}

func (v *mmList_) get(i int32) any {
	l := v.list_()
	if l == nil {
		v.outOfBounds_(i)
	}
	return l.get(i)
}

func (v *mmList_) set(i int32, x any) any {
	l := v.list_()
	if l == nil || i < 0 || i >= l.size() {
		v.outOfBounds_(i)
	}
	old := l.set(i, x)
	if v.mm.linked {
		n := i
		for _, e := range v.mm.seq {
			if objequals_(e.key, v.key) {
				if n == 0 {
					e.val = x
					break
				}
				n--
			}
		}
	}
	return old
}

func (v *mmList_) add_int_Object(i int32, x any) {
	l := v.list_()
	if l == nil || i == l.size() {
		if i != 0 && l == nil {
			v.outOfBounds_(i)
		}
		v.mm.addVal_(v.key, x)
		return
	}
	l.add_int_Object(i, x)
	v.mm.n++
	if v.mm.linked {
		// Insert before the entry that now follows it for this key.
		n := i
		for j, e := range v.mm.seq {
			if objequals_(e.key, v.key) {
				if n == 0 {
					v.mm.seq = append(v.mm.seq, nil)
					copy(v.mm.seq[j+1:], v.mm.seq[j:])
					v.mm.seq[j] = &immEntry_{key: v.key, val: x}
					break
				}
				n--
			}
		}
	}
}

func (v *mmList_) remove(i int32) any {
	l := v.list_()
	if l == nil || i < 0 || i >= l.size() {
		v.outOfBounds_(i)
	}
	return v.mm.removeAt_(v.key, l, i)
}

func (v *mmList_) indexOf(x any) int32 {
	if l := v.list_(); l != nil {
		return l.indexOf(x)
	}
	return -1
}

func (v *mmList_) lastIndexOf(x any) int32 {
	if l := v.list_(); l != nil {
		return l.lastIndexOf(x)
	}
	return -1
}

func (v *mmList_) listIterator(i int32) ListIterator { return &idxIter_{l: v, i: i, last: -1} }

func (v *mmList_) subList(a int32, b int32) List {
	l := v.list_()
	if l == nil {
		l = new_ArrayList_0()
	}
	return l.subList(a, b)
}

func (v *mmList_) equals(o any) bool {
	return o == any(v) || (isListLike_(o) && listEquals_(v.toArray_0(), o))
}
func (v *mmList_) hashCode() int32 { return listHash_(v.toArray_0()) }

// ---- Multimap.asMap()

type mmAsMap_ struct{ mm *mmap_ }

func (a *mmAsMap_) size() int32            { return a.mm.m.size() }
func (a *mmAsMap_) isEmpty() bool          { return a.mm.n == 0 }
func (a *mmAsMap_) containsKey(k any) bool { return a.mm.m.containsKey(k) }

func (a *mmAsMap_) containsValue(v any) bool {
	for _, k := range a.mm.keys_() {
		if objequals_(a.mm.get(k), v) {
			return true
		}
	}
	return false
}

func (a *mmAsMap_) get(k any) any {
	if !a.mm.m.containsKey(k) {
		return nil
	}
	return a.mm.get(k)
}

func (a *mmAsMap_) put(k any, v any) any { throwUOE_(); return nil }
func (a *mmAsMap_) putAll(m Map)         { throwUOE_() }
func (a *mmAsMap_) clear()               { a.mm.clear() }

func (a *mmAsMap_) remove(k any) any {
	if c := a.mm.removeKey_(k); c != nil {
		return c
	}
	return nil
}

func (a *mmAsMap_) keySet() Set { return a.mm.keySet() }

func (a *mmAsMap_) iter_(pick func(k any) any) Iterator {
	keys := a.mm.keys_()
	si := &sliceIter_{items: &keys}
	return &funcIter_{
		has: si.hasNext,
		nxt: func() any { return pick(si.next()) },
		del: func() { a.mm.removeKey_(keys[si.i-1]) },
	}
}

func (a *mmAsMap_) values() Collection {
	return &viewColl_{
		it:  func() Iterator { return a.iter_(func(k any) any { return a.mm.get(k) }) },
		sz:  a.size,
		clr: a.mm.clear,
	}
}

func (a *mmAsMap_) entrySet() Set {
	return &viewColl_{
		set: true,
		it: func() Iterator {
			return a.iter_(func(k any) any { return &immEntry_{key: k, val: a.mm.get(k)} })
		},
		sz: a.size,
		has: func(x any) bool {
			e, ok := x.(Map_Entry)
			return ok && e != nil && a.containsKey(e.getKey()) && objequals_(a.mm.get(e.getKey()), e.getValue())
		},
		clr: a.mm.clear,
	}
}

func (a *mmAsMap_) equals(o any) bool { return o == any(a) || mapEquals_(a, o) }
func (a *mmAsMap_) hashCode() int32   { return mapHash_(a) }
func (a *mmAsMap_) toString() string  { return mapToString_(a) }

// ---- mset_: the mutable multiset core

type mset_ struct {
	m Map // element -> int32 count (> 0)
	n int32
}

func (ms *mset_) init_(sorted bool) {
	if sorted {
		ms.m = new_TreeMap()
	} else {
		ms.m = new_LinkedHashMap()
	}
}

func (ms *mset_) size() int32   { return ms.n }
func (ms *mset_) isEmpty() bool { return ms.n == 0 }

func (ms *mset_) count(v any) int32 {
	if ms.n == 0 {
		return 0
	}
	if c := ms.m.get(v); c != nil {
		return c.(int32)
	}
	return 0
}

func (ms *mset_) contains(v any) bool { return ms.count(v) > 0 }

func (ms *mset_) setRaw_(v any, old, n int32) {
	if n == 0 {
		ms.m.remove(v)
	} else {
		ms.m.put(v, n)
	}
	ms.n += n - old
}

func (ms *mset_) add_Object_int(v any, k int32) int32 {
	if k < 0 {
		throwIAE_("occurrences cannot be negative: " + istr_(int64(k)))
	}
	old := ms.count(v)
	if k > 0 {
		ms.setRaw_(v, old, old+k)
	}
	return old
}

func (ms *mset_) add(v any) bool {
	ms.add_Object_int(v, 1)
	return true
}

func (ms *mset_) addAll(c Collection) bool {
	s := copySlice_(c)
	for _, v := range s {
		ms.add_Object_int(v, 1)
	}
	return len(s) > 0
}

func (ms *mset_) remove_Object_int(v any, k int32) int32 {
	if k < 0 {
		throwIAE_("occurrences cannot be negative: " + istr_(int64(k)))
	}
	old := ms.count(v)
	if old > 0 && k > 0 {
		n := old - k
		if n < 0 {
			n = 0
		}
		ms.setRaw_(v, old, n)
	}
	return old
}

func (ms *mset_) remove_Object(v any) bool { return ms.remove_Object_int(v, 1) > 0 }

func (ms *mset_) setCount_Object_int(v any, n int32) int32 {
	if n < 0 {
		throwIAE_("count cannot be negative: " + istr_(int64(n)))
	}
	old := ms.count(v)
	if old != n {
		ms.setRaw_(v, old, n)
	}
	return old
}

func (ms *mset_) setCount(v any, old int32, n int32) bool {
	if old < 0 || n < 0 {
		throwIAE_("count cannot be negative")
	}
	if ms.count(v) != old {
		return false
	}
	if old != n {
		ms.setRaw_(v, old, n)
	}
	return true
}

func (ms *mset_) clear() {
	ms.m.clear()
	ms.n = 0
}

func (ms *mset_) elems_() []any { return toSlice_(ms.m.keySet()) }

func (ms *mset_) toArray_0() []any {
	out := make([]any, 0, ms.n)
	for _, v := range ms.elems_() {
		for i := ms.count(v); i > 0; i-- {
			out = append(out, v)
		}
	}
	return out
}

func (ms *mset_) toArray(a []any) []any { return fillArray_(ms.toArray_0(), a) }

func (ms *mset_) iterator() Iterator {
	all := ms.toArray_0()
	si := &sliceIter_{items: &all}
	return &funcIter_{has: si.hasNext, nxt: si.next, del: func() { ms.remove_Object_int(all[si.i-1], 1) }}
}

func (ms *mset_) containsAll(c Collection) bool { return containsAll_(ms, c) }

func (ms *mset_) removeAll(c Collection) bool {
	changed := false
	for _, v := range ms.elems_() {
		if c.contains(v) {
			ms.setCount_Object_int(v, 0)
			changed = true
		}
	}
	return changed
}

func (ms *mset_) retainAll(c Collection) bool {
	changed := false
	for _, v := range ms.elems_() {
		if !c.contains(v) {
			ms.setCount_Object_int(v, 0)
			changed = true
		}
	}
	return changed
}

func (ms *mset_) elementSet() Set {
	return &viewColl_{
		set: true,
		it: func() Iterator {
			es := ms.elems_()
			si := &sliceIter_{items: &es}
			return &funcIter_{has: si.hasNext, nxt: si.next, del: func() { ms.setCount_Object_int(es[si.i-1], 0) }}
		},
		sz:  func() int32 { return ms.m.size() },
		has: ms.contains,
		rm:  func(v any) bool { return ms.setCount_Object_int(v, 0) > 0 },
		clr: ms.clear,
	}
}

func (ms *mset_) entrySet() Set {
	return &viewColl_{
		set: true,
		it: func() Iterator {
			es := ms.elems_()
			si := &sliceIter_{items: &es}
			return &funcIter_{
				has: si.hasNext,
				nxt: func() any { v := si.next(); return &msEntry_{elem: v, count: ms.count(v)} },
				del: func() { ms.setCount_Object_int(es[si.i-1], 0) },
			}
		},
		sz: func() int32 { return ms.m.size() },
		has: func(x any) bool {
			e, ok := x.(Multiset_Entry)
			return ok && e != nil && e.getCount() > 0 && ms.count(e.getElement()) == e.getCount()
		},
		clr: ms.clear,
	}
}

func (ms *mset_) equals(o any) bool {
	if o == any(ms) {
		return true
	}
	x, ok := o.(Multiset)
	if !ok || x == nil || !isMultiset_(o) || x.size() != ms.n {
		return false
	}
	for _, v := range ms.elems_() {
		if x.count(v) != ms.count(v) {
			return false
		}
	}
	return true
}

func (ms *mset_) hashCode() int32 {
	var h int32
	for _, v := range ms.elems_() {
		h += objhash_(v) ^ ms.count(v)
	}
	return h
}

func (ms *mset_) toString() string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range ms.elems_() {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(objstr_(v))
		if n := ms.count(v); n != 1 {
			sb.WriteString(" x " + istr_(int64(n)))
		}
	}
	sb.WriteByte(']')
	return sb.String()
}
