package closure

import "sync/atomic"

// Typed lists (j2go rule "typed collections"): an ArrayList<T> that the translator finds typable
// is a *JTList[T], a slice of T. Its elements are Java's values of type T themselves (a string, a
// class pointer, a handle), not boxes.
//
// Code that sees the list at an erased type (Object, Collection<?>, a List parameter of a
// library function or of a method that is not typed) gets asList(), a view that boxes on the way
// out and unboxes on the way in. A list has one view, so the Java object keeps its identity
// across the boundary. Behaviour mirrors ArrayList in rt_coll.go, exception messages included.

// jtconv_ converts the elements of one element type to and from their boxed form, as the erased
// code boxes and casts a value of that Java type (the translator generates one per element
// type: jtc_string, jtc_p_JSType...).
type jtconv_[T any] struct {
	box   func(T) any
	unbox func(any) T
	null  T // Java's null as a T (the null string for a String, else the zero value)
	// byEquals: a set or a map compares elements of this type by equals and hashCode (through
	// their boxes), not by Go's == (a class with its own equals, an interface, a library class).
	byEquals bool
	// post: this conversion for the post-flip typed collections (t_), made on first use.
	post *tconv_[T]
}

// t_ is the conversion as the post-flip typed list takes it (rt_tlist.go): the same box, unbox
// and null. One per conversion, so that lists made with it share it as they shared c.
// (Post-flip tool jfam: a list the translator typed is a *TList[T].)
func (c *jtconv_[T]) t_() *tconv_[T] {
	if c.post == nil {
		c.post = &tconv_[T]{box: c.box, unbox: c.unbox, null: c.null}
		if c.byEquals {
			// Keys compared by equals and hashCode, through their boxes as teqFind_ compares them.
			c.post.hash = func(k T) int32 { return objhash_(c.box(k)) }
			c.post.eq = func(a, b T) bool { return objequals_(c.box(a), c.box(b)) }
		}
	}
	return c.post
}

type JTList[T any] struct {
	items []T
	c     *jtconv_[T]
	view  atomic.Pointer[jtlistView_[T]]
}

func jnewTList_[T any](c *jtconv_[T]) *JTList[T] { return &JTList[T]{c: c} }

// jnewTListN_ is new ArrayList<>(n).
func jnewTListN_[T any](c *jtconv_[T], n int32) *JTList[T] {
	return &JTList[T]{c: c, items: make([]T, 0, n)}
}

// jtlistFrom_ is new ArrayList<>(from).
func jtlistFrom_[T any](c *jtconv_[T], from Collection) *JTList[T] {
	src := toSlice_(from)
	items := make([]T, len(src))
	for i, x := range src {
		items[i] = c.unbox(x)
	}
	return &JTList[T]{c: c, items: items}
}

// tlistCopy_ is new ArrayList<>(from) of another typed list of the same element type.
func tlistCopy_[T any](c *jtconv_[T], from *JTList[T]) *JTList[T] {
	return &JTList[T]{c: c, items: append([]T(nil), from.items...)}
}

func (l *JTList[T]) size() int32   { return int32(len(l.items)) }
func (l *JTList[T]) isEmpty() bool { return len(l.items) == 0 }

func (l *JTList[T]) get(i int32) T {
	if i < 0 || int(i) >= len(l.items) {
		panic(new_IndexOutOfBoundsException_String("Index " + istr_(int64(i)) + " out of bounds for length " + istr_(int64(len(l.items)))).self())
	}
	return l.items[i]
}

func (l *JTList[T]) set(i int32, v T) T {
	old := l.items[i]
	l.items[i] = v
	return old
}

func (l *JTList[T]) add(v T) bool {
	l.items = append(l.items, v)
	return true
}

// insert is add(int, T).
func (l *JTList[T]) insert(i int32, v T) {
	if i < 0 || int(i) > len(l.items) {
		panic(new_IndexOutOfBoundsException_String("Index: " + istr_(int64(i)) + ", Size: " + istr_(int64(len(l.items)))).self())
	}
	var z T
	l.items = append(l.items, z)
	copy(l.items[i+1:], l.items[i:])
	l.items[i] = v
}

func (l *JTList[T]) clear() { l.items = l.items[:0] }

// removeAt is remove(int).
func (l *JTList[T]) removeAt(i int32) T {
	old := l.get(i)
	var z T
	if i == 0 {
		// as ArrayList.remove: step over the slot instead of shifting
		l.items[0] = z
		l.items = l.items[1:]
		return old
	}
	copy(l.items[i:], l.items[i+1:])
	l.items[len(l.items)-1] = z
	l.items = l.items[:len(l.items)-1]
	return old
}

// addAllT is addAll of another typed list of the same element type.
func (l *JTList[T]) addAllT(o *JTList[T]) bool {
	l.items = append(l.items, o.items...)
	return len(o.items) > 0
}

// jsliceEach_ is the cursor of a for-each loop over a typed list. As each_ over an ArrayList, it
// reads the slice anew at every step, so elements appended during the loop are visited.
type jsliceEach_[T any] struct {
	src *[]T
	i   int
	v   T
}

func (e *jsliceEach_[T]) next() bool {
	if e.i < len(*e.src) {
		e.v = (*e.src)[e.i]
		e.i++
		return true
	}
	return false
}

// each is the cursor of a for-each loop over the list; a null list throws NullPointerException
// here, where the loop asked for its iterator.
func (l *JTList[T]) each() jsliceEach_[T] { return jsliceEach_[T]{src: &l.items} }

// iterator is the list's Iterator (of boxed elements, with remove), as its view's.
func (l *JTList[T]) iterator() Iterator {
	if l == nil {
		panic(new_NullPointerException_0().self())
	}
	return &jtlistIter_[T]{l: l}
}

// asList is the list as an erased List (nil for null): one view per list, its identity for
// erased code.
func (l *JTList[T]) asList() List {
	if l == nil {
		return nil
	}
	v := l.view.Load()
	if v == nil {
		// one view, also when two threads ask for the first time
		l.view.CompareAndSwap(nil, &jtlistView_[T]{l: l})
		v = l.view.Load()
	}
	return v
}

// asList_ is a list of the translated program as an erased List, whether the translator made
// its declaration a typed list or not (for handwritten runtime code that calls generated code).
func asList_(l any) List {
	switch x := l.(type) {
	case List:
		return x
	case interface{ asList() List }:
		return x.asList()
	}
	return nil
}

// jtlistView_ is a JTList seen as an erased List.
type jtlistView_[T any] struct {
	l *JTList[T]
}

func (v *jtlistView_[T]) boxed() []any {
	out := make([]any, len(v.l.items))
	for i, x := range v.l.items {
		out[i] = v.l.c.box(x)
	}
	return out
}

// step_ is for the cursor of a for-each loop in erased code (foreach_).
func (v *jtlistView_[T]) step_(i int) (any, int) {
	if i < len(v.l.items) {
		return v.l.c.box(v.l.items[i]), i + 1
	}
	return nil, -1
}

// goName_ makes getClass() of the view java.util.ArrayList, the class of the Java object.
func (v *jtlistView_[T]) goName_() string { return "ArrayList" }

func (v *jtlistView_[T]) self() any                     { return v }
func (v *jtlistView_[T]) size() int32                   { return v.l.size() }
func (v *jtlistView_[T]) isEmpty() bool                 { return v.l.isEmpty() }
func (v *jtlistView_[T]) get(i int32) any               { return v.l.c.box(v.l.get(i)) }
func (v *jtlistView_[T]) set(i int32, x any) any        { return v.l.c.box(v.l.set(i, v.l.c.unbox(x))) }
func (v *jtlistView_[T]) add(x any) bool                { return v.l.add(v.l.c.unbox(x)) }
func (v *jtlistView_[T]) add_int_Object(i int32, x any) { v.l.insert(i, v.l.c.unbox(x)) }
func (v *jtlistView_[T]) remove(i int32) any            { return v.l.c.box(v.l.removeAt(i)) }
func (v *jtlistView_[T]) clear()                        { v.l.clear() }

func (v *jtlistView_[T]) addAll(c Collection) bool {
	if o, ok := c.(*jtlistView_[T]); ok && o != nil {
		return v.l.addAllT(o.l)
	}
	src := toSlice_(c)
	for _, x := range src {
		v.l.items = append(v.l.items, v.l.c.unbox(x))
	}
	return len(src) > 0
}

// strs is the list's items when its elements are strings (a search for a string compares
// them without boxing each).
func (v *jtlistView_[T]) strs() ([]string, bool) {
	p, ok := any(&v.l.items).(*[]string)
	if !ok {
		return nil, false
	}
	return *p, true
}

func (v *jtlistView_[T]) indexOf(x any) int32 {
	if s, ok := x.(string); ok {
		if items, ok := v.strs(); ok {
			return indexOfStr_(items, s)
		}
	}
	for i, y := range v.l.items {
		if objequals_(x, v.l.c.box(y)) {
			return int32(i)
		}
	}
	return -1
}

func indexOfStr_(items []string, s string) int32 {
	for i, y := range items {
		if y == s {
			return int32(i)
		}
	}
	return -1
}

// contains_s is contains of a String that is not null, without its box (Collection_contains_s).
func (v *jtlistView_[T]) contains_s(s string) bool {
	if items, ok := v.strs(); ok {
		return indexOfStr_(items, s) >= 0
	}
	return v.indexOf(strobj_(s)) >= 0
}

func (v *jtlistView_[T]) lastIndexOf(x any) int32 {
	for i := len(v.l.items) - 1; i >= 0; i-- {
		if objequals_(x, v.l.c.box(v.l.items[i])) {
			return int32(i)
		}
	}
	return -1
}

func (v *jtlistView_[T]) contains(x any) bool { return v.indexOf(x) >= 0 }

func (v *jtlistView_[T]) containsAll(c Collection) bool {
	for it := c.iterator(); it.hasNext(); {
		if !v.contains(it.next()) {
			return false
		}
	}
	return true
}

func (v *jtlistView_[T]) remove_Object(x any) bool {
	i := v.indexOf(x)
	if i < 0 {
		return false
	}
	v.l.removeAt(i)
	return true
}

func (v *jtlistView_[T]) removeAll(c Collection) bool {
	l := v.l
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

func (v *jtlistView_[T]) iterator() Iterator { return &jtlistIter_[T]{l: v.l} }
func (v *jtlistView_[T]) listIterator(i int32) ListIterator {
	return &jtlistIter_[T]{l: v.l, i: int(i)}
}

// subList is, as ArrayList's in rt_coll.go, a list of the elements in the range as they are now.
func (v *jtlistView_[T]) subList(a int32, b int32) List {
	if a < 0 || b > v.l.size() || a > b {
		panic(new_IndexOutOfBoundsException_String("fromIndex: " + istr_(int64(a)) + ", toIndex: " + istr_(int64(b)) + ", size: " + istr_(int64(len(v.l.items)))).self())
	}
	return listOf_(v.boxed()[a:b:b])
}

func (v *jtlistView_[T]) toArray_0() []any      { return v.boxed() }
func (v *jtlistView_[T]) toArray(a []any) []any { return fillArray_(v.boxed(), a) }
func (v *jtlistView_[T]) equals(o any) bool     { return listEquals_(v.boxed(), o) }
func (v *jtlistView_[T]) hashCode() int32       { return listHash_(v.boxed()) }
func (v *jtlistView_[T]) toString() string      { return collToString_(v) }

// jtlistIter_ iterates over a JTList for erased code (Iterator, with remove).
type jtlistIter_[T any] struct {
	l *JTList[T]
	i int
}

func (it *jtlistIter_[T]) hasNext() bool { return it.i < len(it.l.items) }

func (it *jtlistIter_[T]) next() any {
	if it.i >= len(it.l.items) {
		panic(new_NoSuchElementException_0().self())
	}
	v := it.l.items[it.i]
	it.i++
	return it.l.c.box(v)
}

func (it *jtlistIter_[T]) remove() {
	it.i--
	it.l.removeAt(int32(it.i))
}
