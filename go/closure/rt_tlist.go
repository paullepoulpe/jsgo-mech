package closure

import "unsafe"

// Typed lists (j2go rule "typed collections"): an ArrayList<T> that the translator finds typable
// is a *TList[T], a slice of T. Its elements are Java's values of type T themselves (a string, a
// class pointer, a handle), not boxes.
//
// Code that sees the list at an erased type (Object, Collection<?>, a List parameter of a
// library function or of a method that is not typed) gets asList(), a view that boxes on the way
// out and unboxes on the way in. A list has one view, so the Java object keeps its identity
// across the boundary. Behaviour mirrors ArrayList in rt_coll.go, exception messages included.

// tconv_ converts the elements of one element type to and from their boxed form, as the erased
// code boxes and casts a value of that Java type (the translator generates one per element
// type: tc_string, tc_p_JSType...).
type tconv_[T any] struct {
	box   func(T) any
	unbox func(any) T
	null  T // Java's null as a T (the null string for a String, else the zero value)
	// byEquals: a set or a map compares elements of this type by equals and hashCode (through
	// their boxes), not by Go's == (a class with its own equals, an interface, a library class).
	byEquals bool
	// the empty immutable list, set and map of this element type (as a map's value type, or
	// else its key type): one object each, as Guava's (rt_timm.go)
	emptyL, emptyS, emptyM any
}

type TList[T any] struct {
	items []T
	c     *tconv_[T]
	view  tlistView_[T] // the list as erased code sees it (asList): part of the list, so one object
	// twin: nil for a list that can change (an ArrayList). An immutable list (Guava's
	// ImmutableList, rt_timm.go) has here the ImmutableList that erased code sees it as, or
	// immPending_ until erased code first needs it; every change of such a list throws
	// UnsupportedOperationException. unmodMark_: a list that cannot be changed and is no
	// ImmutableList (Collections.unmodifiableList of a private list): erased code sees its view.
	twin *ImmutableCollection
}

func newTList_[T any](c *tconv_[T]) *TList[T] {
	l := &TList[T]{c: c}
	l.view.l = l
	return l
}

// newTListN_ is new ArrayList<>(n).
func newTListN_[T any](c *tconv_[T], n int32) *TList[T] {
	l := &TList[T]{c: c, items: make([]T, 0, n)}
	l.view.l = l
	return l
}

// tlistFrom_ is new ArrayList<>(from).
func tlistFrom_[T any](c *tconv_[T], from Collection) *TList[T] {
	src := toSlice_(from)
	items := make([]T, len(src))
	for i, x := range src {
		items[i] = c.unbox(x)
	}
	l := &TList[T]{c: c, items: items}
	l.view.l = l
	return l
}

// tlistCopy_ is new ArrayList<>(from) of another typed list of the same element type.
func tlistCopy_[T any](c *tconv_[T], from *TList[T]) *TList[T] {
	l := &TList[T]{c: c, items: append([]T(nil), from.items...)}
	l.view.l = l
	return l
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
	if l.twin != nil {
		panic(frozen_{})
	}
	old := l.items[i]
	l.items[i] = v
	return old
}

func (l *TList[T]) add(v T) bool {
	if l.twin != nil {
		panic(frozen_{})
	}
	l.items = append(l.items, v)
	return true
}

// insert is add(int, T).
func (l *TList[T]) insert(i int32, v T) {
	if l.twin != nil {
		panic(frozen_{})
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
	if l.twin != nil {
		panic(frozen_{})
	}
	l.items = l.items[:0]
}

// removeAt is remove(int).
func (l *TList[T]) removeAt(i int32) T {
	if l.twin != nil {
		panic(frozen_{})
	}
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
func (l *TList[T]) addAllT(o *TList[T]) bool {
	if l.twin != nil {
		panic(frozen_{})
	}
	l.items = append(l.items, o.items...)
	return len(o.items) > 0
}

// sliceEach_ is the cursor of a for-each loop over a typed list. As each_ over an ArrayList, it
// reads the slice anew at every step, so elements appended during the loop are visited.
type sliceEach_[T any] struct {
	src *[]T
	i   int
	v   T
}

func (e *sliceEach_[T]) next() bool {
	if e.i < len(*e.src) {
		e.v = (*e.src)[e.i]
		e.i++
		return true
	}
	return false
}

// each is the cursor of a for-each loop over the list; a null list throws NullPointerException
// here, where the loop asked for its iterator.
func (l *TList[T]) each() sliceEach_[T] { return sliceEach_[T]{src: &l.items} }

// iterator is the list's Iterator (of boxed elements, with remove), as its view's.
func (l *TList[T]) iterator() Iterator {
	if l == nil {
		panic(new_NullPointerException_0().self())
	}
	return &tlistIter_[T]{l: l}
}

// asList is the list as an erased List (nil for null): the list's one view, its identity for
// erased code. The view is a field of the list (set when the list is made), so that a list
// which erased code sees is still one object. An immutable list is seen as its ImmutableList.
func (l *TList[T]) asList() List {
	if l == nil || l.twin != nil {
		return l.asOtherList()
	}
	return &l.view
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

// tlistView_ is a TList seen as an erased List.
type tlistView_[T any] struct {
	l *TList[T]
}

func (v *tlistView_[T]) boxed() []any {
	out := make([]any, len(v.l.items))
	for i, x := range v.l.items {
		out[i] = v.l.c.box(x)
	}
	return out
}

// step_ is for the cursor of a for-each loop in erased code (foreach_).
func (v *tlistView_[T]) step_(i int) (any, int) {
	if i < len(v.l.items) {
		return v.l.c.box(v.l.items[i]), i + 1
	}
	return nil, -1
}

// goName_ makes getClass() of the view java.util.ArrayList, the class of the Java object.
func (v *tlistView_[T]) goName_() string { return "ArrayList" }

func (v *tlistView_[T]) self() any                     { return v }
func (v *tlistView_[T]) size() int32                   { return v.l.size() }
func (v *tlistView_[T]) isEmpty() bool                 { return v.l.isEmpty() }
func (v *tlistView_[T]) get(i int32) any               { return v.l.c.box(v.l.get(i)) }
func (v *tlistView_[T]) set(i int32, x any) any        { return v.l.c.box(v.l.set(i, v.l.c.unbox(x))) }
func (v *tlistView_[T]) add(x any) bool                { return v.l.add(v.l.c.unbox(x)) }
func (v *tlistView_[T]) add_int_Object(i int32, x any) { v.l.insert(i, v.l.c.unbox(x)) }
func (v *tlistView_[T]) remove(i int32) any            { return v.l.c.box(v.l.removeAt(i)) }
func (v *tlistView_[T]) clear()                        { v.l.clear() }

func (v *tlistView_[T]) addAll(c Collection) bool {
	if v.l.twin != nil {
		panic(frozen_{})
	}
	if o, ok := c.(*tlistView_[T]); ok && o != nil {
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
func (v *tlistView_[T]) strs() ([]string, bool) {
	p, ok := any(&v.l.items).(*[]string)
	if !ok {
		return nil, false
	}
	return *p, true
}

func (v *tlistView_[T]) indexOf(x any) int32 {
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
func (v *tlistView_[T]) contains_s(s string) bool {
	if items, ok := v.strs(); ok {
		return indexOfStr_(items, s) >= 0
	}
	return v.indexOf(strobj_(s)) >= 0
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
	if l.twin != nil {
		panic(frozen_{})
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

// subList is, as ArrayList's in rt_coll.go, a list of the elements in the range as they are now.
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

func (it *tlistIter_[T]) remove() {
	it.i--
	it.l.removeAt(int32(it.i))
}

// tcT_ is the conversion of an element type that is a Go type parameter (a List<T> in a generic
// class): a value of that type meets Object as a value of a type parameter does (boxT_,
// unboxT_). One object per instantiation, made on first use. (An instantiation of a generic
// class that names type parameters, a List<DiGraphNode<N, E>>, has a generated generic function
// with the conversions of the class: g_0casts.go.)
func tcT_[T any]() *tconv_[T] {
	return single_(&tcT_single_, func() *tconv_[T] {
		return &tconv_[T]{box: boxT_[T], unbox: unboxT_[T], null: null_[T]()}
	})
}

// tceT_ is tcT_ for the elements of a set or the keys of a map (a Set<T>, a Map<T, V> in a
// generic class): the collection compares them with == where that is Java's equals for the
// instantiation (a String, a class that the translator registered with regGoEquals_), else by
// equals and hashCode (an interface, Object, a class with its own equals, a library class).
func tceT_[T any]() *tconv_[T] {
	return single_(&tceT_single_, func() *tconv_[T] {
		var z T
		return &tconv_[T]{box: boxT_[T], unbox: unboxT_[T], null: null_[T](), byEquals: !goEquals_(any(z))}
	})
}

var tcT_single_, tceT_single_ singles_

// goEqualsTypes_ holds the Go types (of the program's classes) whose == is Java's equals.
var goEqualsTypes_ = map[unsafe.Pointer]struct{}{}

// regGoEquals_ registers the types of xs (generated: g_0casts.go).
func regGoEquals_(xs ...any) {
	for _, x := range xs {
		goEqualsTypes_[(*eface_)(unsafe.Pointer(&x)).typ] = struct{}{}
	}
}

// goEquals_ reports whether == on values of the type of z (the zero value of a type argument)
// is Java's equals: a string, or a registered class. The zero value of an interface type has
// no type: its values are compared by equals.
func goEquals_(z any) bool {
	if _, ok := z.(string); ok {
		return true
	}
	_, ok := goEqualsTypes_[(*eface_)(unsafe.Pointer(&z)).typ]
	return z != nil && ok
}
