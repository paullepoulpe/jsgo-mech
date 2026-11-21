package closure

import (
	"reflect"
	"slices"
)

// Typed lists (post-flip step 3): ArrayList<T> as a slice of T.
//
// A TList keeps its elements unboxed. Code that still expects an erased List gets asList(),
// a view that boxes on the way out and unboxes on the way in; a list has one view, so its
// identity is preserved across the boundary. Behaviour mirrors ArrayList in rt_coll.go,
// exception messages included.

// tconv_ converts the elements of one element type to and from their boxed form.
type tconv_[T any] struct {
	box   func(T) any
	unbox func(any) T
	null  T // Java's null as a T (the null string for strings, else the zero value)
	// For a key of a typed map or set compared by equals (a class with its own equals and
	// hashCode, or an interface such classes implement): Java's hashCode of a non-null key, and
	// a.equals(b) for a non-null a. Nil for keys compared by ==.
	hash func(T) int32
	eq   func(a, b T) bool
	// The empty immutable list, set and map of this element type (of a map: of this value
	// type), made on first use (rt_timm.go): ImmutableList.of() is one object in Guava.
	emptyL *TList[T]
	emptyS any
	emptyM any
}

type TList[T any] struct {
	items []T
	c     *tconv_[T]
	view  *tlistView_[T]
	imm   bool // an ImmutableList: every change throws UnsupportedOperationException
}

func newTList_[T any](c *tconv_[T]) *TList[T] { return &TList[T]{c: c} }

func newTListN_[T any](c *tconv_[T], n int32) *TList[T] {
	return &TList[T]{c: c, items: make([]T, 0, n)}
}

// tlistImm_ is an immutable list (ImmutableList.of, copyOf, build) as a typed list.
func tlistImm_[T any](c *tconv_[T], from Collection) *TList[T] {
	if isNil_(from) {
		return nil
	}
	if from == Collection(emptyImmList_) {
		return tlistEmptyImm_(c) // ImmutableList.of(), one object as in Guava
	}
	slot := immSlot_(from)
	if slot != nil && slot.conv == any(c) {
		if l, ok := slot.v.(*TList[T]); ok {
			return l
		}
	}
	l := tlistFrom_(c, from)
	l.imm = true
	if slot != nil {
		*slot = immTyped_{c, l}
	}
	if ic, ok := from.(*ImmutableCollection); ok && ic.kind == icList_ {
		// The list is the immutable list itself for erased code (asList): its identity.
		l.view = &tlistView_[T]{l: l, twin: ic}
	}
	return l
}

// immTyped_ is the typed copy of an immutable collection, kept on the collection itself: an
// ImmutableList/Set/Map never changes, so a long-lived one (a constant passed on every call)
// is copied once, and the same object comes back each time, as the erased code shared it.
type immTyped_ struct {
	conv any // the element conversion of the copy (its element type)
	v    any
}

// immSlot_ is where from keeps its typed copy, or nil if from is not a runtime immutable collection.
func immSlot_(from any) *immTyped_ {
	switch x := from.(type) {
	case *ImmutableCollection:
		return &x.typed_
	case *ErasedImmutableMap:
		return &x.typed_
	}
	return nil
}

// tlistFrom_ is new ArrayList<>(from) as a typed list.
func tlistFrom_[T any](c *tconv_[T], from Collection) *TList[T] {
	return &TList[T]{c: c, items: unboxAll_(c, readSlice_(from))}
}

// tlistFromRev_ is new ArrayDeque(from) as a deque whose head is the end of the list (colls
// -kind deque, a deque used at its head): from's elements in reverse order.
func tlistFromRev_[T any](c *tconv_[T], from Collection) *TList[T] {
	l := tlistFrom_(c, from)
	slices.Reverse(l.items)
	return l
}

// readSlice_ is the elements of c for a caller that only reads them: those of an immutable
// collection are not copied.
func readSlice_(c Iterable) []any {
	if ic, ok := c.(*ImmutableCollection); ok && ic != nil {
		return ic.elems_()
	}
	return toSlice_(c)
}

func (l *TList[T]) size() int32   { return int32(len(l.items)) }
func (l *TList[T]) isEmpty() bool { return len(l.items) == 0 }

func (l *TList[T]) get(i int32) T {
	if i < 0 || int(i) >= len(l.items) {
		panic(new_IndexOutOfBoundsException_String("Index " + istr_(int64(i)) + " out of bounds for length " + istr_(int64(len(l.items)))).self())
	}
	return l.items[i]
}

func (l *TList[T]) set(i int32, v T) T {
	if l.imm {
		throwUOE_()
	}
	old := l.items[i]
	l.items[i] = v
	return old
}

func (l *TList[T]) add(v T) bool {
	if l.imm {
		throwUOE_()
	}
	l.items = append(l.items, v)
	return true
}

func (l *TList[T]) insert(i int32, v T) {
	if l.imm {
		throwUOE_()
	}
	if i < 0 || int(i) > len(l.items) {
		panic(new_IndexOutOfBoundsException_String("Index: " + istr_(int64(i)) + ", Size: " + istr_(int64(len(l.items)))).self())
	}
	var z T
	l.items = append(l.items, z)
	copy(l.items[i+1:], l.items[i:])
	l.items[i] = v
}

func (l *TList[T]) clear() {
	if l.imm {
		throwUOE_()
	}
	clear(l.items)
	l.items = l.items[:0]
}

func (l *TList[T]) removeAt(i int32) T {
	if l.imm {
		throwUOE_()
	}
	old := l.get(i)
	var z T
	if i == 0 && len(l.items) > frontShift_ {
		l.items[0] = z
		l.items = l.items[1:]
		return old
	}
	copy(l.items[i:], l.items[i+1:])
	l.items[len(l.items)-1] = z
	l.items = l.items[:len(l.items)-1]
	return old
}

func (l *TList[T]) trimToSize()            {}
func (l *TList[T]) ensureCapacity(n int32) {}

// each is the cursor of a for-each loop over the list (as each_ over an ArrayList: the
// slice is read anew at every step).
func (l *TList[T]) each() sliceEach_[T] { return sliceEach_[T]{src: &l.items} }

// asList is the list as an erased List (nil for a nil list); an immutable list is its
// ImmutableList (asImm).
func (l *TList[T]) asList() List {
	if l == nil {
		return nil
	}
	if l.imm {
		return l.asImm()
	}
	if l.view == nil {
		l.view = &tlistView_[T]{l: l}
	}
	return l.view
}

// tlistView_ is a TList seen as an erased List. Of an immutable list it holds the
// ImmutableList erased code sees instead (twin, rt_timm.go).
type tlistView_[T any] struct {
	l    *TList[T]
	twin *ImmutableCollection
}

func (v *tlistView_[T]) boxed() []any {
	out := make([]any, len(v.l.items))
	for i, x := range v.l.items {
		out[i] = v.l.c.box(x)
	}
	return out
}

func (v *tlistView_[T]) self() any                     { return v }
func (v *tlistView_[T]) size() int32                   { return v.l.size() }
func (v *tlistView_[T]) isEmpty() bool                 { return v.l.isEmpty() }
func (v *tlistView_[T]) get(i int32) any               { return v.l.c.box(v.l.get(i)) }
func (v *tlistView_[T]) set(i int32, x any) any        { return v.l.c.box(v.l.set(i, v.l.c.unbox(x))) }
func (v *tlistView_[T]) add(x any) bool                { return v.l.add(v.l.c.unbox(x)) }
func (v *tlistView_[T]) add_int_Object(i int32, x any) { v.l.insert(i, v.l.c.unbox(x)) }
func (v *tlistView_[T]) remove(i int32) any            { return v.l.c.box(v.l.removeAt(i)) }
func (v *tlistView_[T]) clear()                        { v.l.clear() }
func (v *tlistView_[T]) trimToSize()                   {}

func (v *tlistView_[T]) addAll(c Collection) bool {
	if v.l.imm {
		throwUOE_()
	}
	src := toSlice_(c)
	for _, x := range src {
		v.l.items = append(v.l.items, v.l.c.unbox(x))
	}
	return len(src) > 0
}

func (v *tlistView_[T]) indexOf(x any) int32 {
	for i, y := range v.l.items {
		if objequals_(x, v.l.c.box(y)) {
			return int32(i)
		}
	}
	return -1
}

func (v *tlistView_[T]) lastIndexOf(x any) int32 {
	for i := len(v.l.items) - 1; i >= 0; i-- {
		if objequals_(x, v.l.c.box(v.l.items[i])) {
			return int32(i)
		}
	}
	return -1
}

func (v *tlistView_[T]) contains(x any) bool { return v.indexOf(x) >= 0 }

func (v *tlistView_[T]) containsAll(c Collection) bool {
	for it := c.iterator(); it.hasNext(); {
		if !v.contains(it.next()) {
			return false
		}
	}
	return true
}

func (v *tlistView_[T]) remove_Object(x any) bool {
	i := v.indexOf(x)
	if i < 0 {
		return false
	}
	v.l.removeAt(i)
	return true
}

func (v *tlistView_[T]) removeAll(c Collection) bool {
	l := v.l
	if l.imm {
		throwUOE_()
	}
	out := l.items[:0]
	for _, x := range l.items {
		if !c.contains(l.c.box(x)) {
			out = append(out, x)
		}
	}
	changed := len(out) != len(l.items)
	clear(l.items[len(out):])
	l.items = out
	return changed
}

func (v *tlistView_[T]) iterator() Iterator                { return &tlistIter_[T]{l: v.l} }
func (v *tlistView_[T]) listIterator(i int32) ListIterator { return &tlistIter_[T]{l: v.l, i: int(i)} }

func (v *tlistView_[T]) subList(a int32, b int32) List {
	if a < 0 || b > v.l.size() || a > b {
		panic(new_IndexOutOfBoundsException_String("fromIndex: " + istr_(int64(a)) + ", toIndex: " + istr_(int64(b)) + ", size: " + istr_(int64(len(v.l.items)))).self())
	}
	return listOf_(v.boxed()[a:b:b])
}

func (v *tlistView_[T]) toArray_0() []any      { return v.boxed() }
func (v *tlistView_[T]) toArray(a []any) []any { return fillArray_(v.boxed(), a) }
func (v *tlistView_[T]) equals(o any) bool     { return listEquals_(v.boxed(), o) }
func (v *tlistView_[T]) hashCode() int32       { return listHash_(v.boxed()) }
func (v *tlistView_[T]) toString() string      { return collToString_(v) }

// tlistIter_ iterates over a TList for erased code (Iterator, with remove).
type tlistIter_[T any] struct {
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
	return it.l.c.box(v)
}

// cursor is l.asList().iterator() as a typed pointer, without the view (post-flip step iters):
// a function that only advances it keeps it on the stack. A null list throws
// NullPointerException here, as asList().iterator() did.
func (l *TList[T]) cursor() *tlistIter_[T] {
	if l == nil {
		panic(new_NullPointerException_0().self())
	}
	return &tlistIter_[T]{l: l}
}

// nextT is next without boxing the element (the code unboxed it).
func (it *tlistIter_[T]) nextT() T {
	if it.i >= len(it.l.items) {
		panic(new_NoSuchElementException_0().self())
	}
	v := it.l.items[it.i]
	it.i++
	return v
}

func (it *tlistIter_[T]) remove() {
	it.i--
	it.l.removeAt(int32(it.i))
}

// isNil_ reports whether c is null: a nil interface or an interface holding a nil pointer (an
// ImmutableList field that is null reaches the copy as the latter).
func isNil_(c any) bool {
	if c == nil {
		return true
	}
	v := reflect.ValueOf(c)
	return v.Kind() == reflect.Pointer && v.IsNil()
}

// addAllT is addAll from another typed list.
func (l *TList[T]) addAllT(o *TList[T]) bool {
	if l.imm {
		throwUOE_()
	}
	l.items = append(l.items, o.items...)
	return len(o.items) > 0
}

// ---- An ArrayDeque as a typed list (post-flip tool colls -kind deque). A deque used only at
// its head keeps its head at the end of items, so that push and pop are an append and a
// truncation (a for-each loop walks it with eachRev); any other keeps the deque's order, its
// tail at the end of items, and works at its head at index 0 (popFirst_...). The methods
// below work at the end of items: the head of the first kind, the tail of the second. A deque
// pushed at its head and used at its tail, or seen as an erased collection, stays erased.

// push_ is push / addFirst at the head, add / addLast / offer at the tail.
func (l *TList[T]) push_(v T) { l.items = append(l.items, v) }

// pop_ is pop / removeFirst (removeLast at the tail): NoSuchElementException when empty.
func (l *TList[T]) pop_() T {
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

// peekTop_ is peek / peekFirst (peekLast at the tail): null when empty.
func (l *TList[T]) peekTop_() T {
	if len(l.items) == 0 {
		return l.c.null
	}
	return l.items[len(l.items)-1]
}

// pollTop_ is poll / pollFirst (pollLast at the tail): null when empty.
func (l *TList[T]) pollTop_() T {
	if len(l.items) == 0 {
		return l.c.null
	}
	return l.pop_()
}

// getTop_ is getFirst / element (getLast at the tail): NoSuchElementException when empty.
func (l *TList[T]) getTop_() T {
	if len(l.items) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return l.items[len(l.items)-1]
}

// eachRev is the cursor of a for-each loop over a deque used at its head: from the head (the
// end of items) to the tail, the k-th step reading the k-th item from the end anew, as the
// deque's loop reads the k-th item from its head.
func (l *TList[T]) eachRev() sliceEachRev_[T] { return sliceEachRev_[T]{src: &l.items} }

type sliceEachRev_[T any] struct {
	src *[]T
	k   int
	v   T
}

func (e *sliceEachRev_[T]) next() bool {
	if s := *e.src; e.k < len(s) {
		e.v = s[len(s)-1-e.k]
		e.k++
		return true
	}
	return false
}

// The head of a deque used at its tail as well (a queue), at index 0. pollFirst_ and popFirst_
// drop the first item as ArrayDeque does, by moving the start of items.

// popFirst_ is pop / removeFirst / remove(): NoSuchElementException when empty.
func (l *TList[T]) popFirst_() T {
	if len(l.items) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	v := l.items[0]
	var z T
	l.items[0] = z
	l.items = l.items[1:]
	return v
}

// pollFirst_ is poll / pollFirst: null when empty.
func (l *TList[T]) pollFirst_() T {
	if len(l.items) == 0 {
		return l.c.null
	}
	return l.popFirst_()
}

// peekFirst_ is peek / peekFirst: null when empty.
func (l *TList[T]) peekFirst_() T {
	if len(l.items) == 0 {
		return l.c.null
	}
	return l.items[0]
}

// getFirst_ is getFirst / element: NoSuchElementException when empty.
func (l *TList[T]) getFirst_() T {
	if len(l.items) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return l.items[0]
}
