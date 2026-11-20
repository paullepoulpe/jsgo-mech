package closure

// Typed deques (j2go rule "typed collections"): an ArrayDeque<T> that the translator finds
// typable is a *TDeque[T]: a typed list whose first item is the deque's head, as the erased
// ArrayDeque in rt_coll.go is an ArrayList whose first item is its head. Erased code sees it
// through asDeque(), its one view (see TList). (The deques that post-flip step colls -kind
// deque types are TLists whose last item is the head: rt_tlist.go push_, pop_.)

type TDeque[T any] struct {
	TList[T]
	dview *tdequeFace_[T]
}

// newTDeque_ is new ArrayDeque<>().
func newTDeque_[T any](c *tconv_[T]) *TDeque[T] {
	d := &TDeque[T]{}
	d.c = c
	return d
}

// newTDequeN_ is new ArrayDeque<>(n): the capacity is not used.
func newTDequeN_[T any](c *tconv_[T], n int32) *TDeque[T] { return newTDeque_(c) }

// tdequeFrom_ is new ArrayDeque<>(from): from's first element is the head.
func tdequeFrom_[T any](c *tconv_[T], from Collection) *TDeque[T] {
	d := newTDeque_(c)
	src := toSlice_(from)
	d.items = make([]T, len(src))
	for i, x := range src {
		d.items[i] = c.unbox(x)
	}
	return d
}

// tdequeCopy_ is new ArrayDeque<>(from) of another typed deque of the same element type.
func tdequeCopy_[T any](c *tconv_[T], from *TDeque[T]) *TDeque[T] {
	d := newTDeque_(c)
	d.items = append([]T(nil), from.items...)
	return d
}

// push is push and addFirst: v is the new head.
func (d *TDeque[T]) push(v T) { d.insert(0, v) }

// poll is poll and pollFirst: the head, removed; null when the deque is empty.
func (d *TDeque[T]) poll() T {
	if len(d.items) == 0 {
		return d.c.null
	}
	v := d.items[0]
	var z T
	d.items[0] = z
	d.items = d.items[1:]
	return v
}

// pop is pop, remove() and removeFirst: NoSuchElementException when the deque is empty.
func (d *TDeque[T]) pop() T {
	if len(d.items) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return d.poll()
}

// peek is peek and peekFirst: the head, or null when the deque is empty.
func (d *TDeque[T]) peek() T {
	if len(d.items) == 0 {
		return d.c.null
	}
	return d.items[0]
}

// peekLast is the tail, or null when the deque is empty.
func (d *TDeque[T]) peekLast() T {
	if len(d.items) == 0 {
		return d.c.null
	}
	return d.items[len(d.items)-1]
}

// asDeque is the deque as an erased Deque (nil for null): one view per deque, its identity for
// erased code.
func (d *TDeque[T]) asDeque() Deque {
	if d == nil {
		return nil
	}
	if d.dview == nil {
		v := &tdequeFace_[T]{d: d}
		v.l = &d.TList
		d.dview = v
	}
	return d.dview
}

// tdequeFace_ is a TDeque seen as an erased Deque: the list view of its items, and the
// operations at its ends as ArrayDeque in rt_coll.go has them.
type tdequeFace_[T any] struct {
	tlistView_[T]
	d *TDeque[T]
}

func (v *tdequeFace_[T]) self() any       { return v }
func (v *tdequeFace_[T]) goName_() string { return "ArrayDeque" }
func (v *tdequeFace_[T]) peek() any       { return v.d.c.box(v.d.peek()) }
func (v *tdequeFace_[T]) peekFirst() any  { return v.d.c.box(v.d.peek()) }
func (v *tdequeFace_[T]) peekLast() any   { return v.d.c.box(v.d.peekLast()) }
func (v *tdequeFace_[T]) poll() any       { return v.d.c.box(v.d.poll()) }
func (v *tdequeFace_[T]) pollFirst() any  { return v.d.c.box(v.d.poll()) }
func (v *tdequeFace_[T]) pop() any        { return v.d.c.box(v.d.pop()) }
func (v *tdequeFace_[T]) remove_0() any   { return v.d.c.box(v.d.pop()) }
func (v *tdequeFace_[T]) push(x any)      { v.d.push(v.d.c.unbox(x)) }

func (v *tdequeFace_[T]) descendingIterator() Iterator {
	rev := make([]any, len(v.d.items))
	for i, x := range v.d.items {
		rev[len(rev)-1-i] = v.d.c.box(x)
	}
	return &sliceIter_{items: &rev}
}
