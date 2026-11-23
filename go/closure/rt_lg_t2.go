package closure

// Rule "typed library interfaces", the kinds of rule candidate T2 (docs/javaisms6/JAVAISMS6.md;
// tools/j2go LibGen.CLASSES_T2, LibGen.MAKERS_T2, LibGen.KINDS `deque`): the typed makers and
// typed interfaces of library types that the earlier kinds did not reach. Each maker makes the
// typed collection of the same kind as the runtime's erased one, for a place that holds the
// typed interface.

// ---- kind `enumset`: EnumSet is the runtime's TreeSet ordered by ordinal (rt_coll2.go); its
// typed twin is the typed set in its sorted mode with the same comparator (rt_tsorted.go).

// tenumSetNoneOf_ is EnumSet.noneOf(c).
func tenumSetNoneOf_[K comparable](c *tconv_[K], cl *Class) *TSet[K] {
	return newTSortedSetC_(c, ordinalCmp_())
}

// tenumSetAllOf_ is EnumSet.allOf(c): the constants in their declaration order.
func tenumSetAllOf_[K comparable](c *tconv_[K], cl *Class) *TSet[K] {
	s := tenumSetNoneOf_(c, cl)
	for _, x := range enumRegistry_[cl.name_] {
		s.addSorted(c.unbox(x))
	}
	return s
}

// tenumSetOf_ is EnumSet.of(e).
func tenumSetOf_[K comparable](c *tconv_[K], e *Enum) *TSet[K] {
	s := tenumSetNoneOf_(c, nil)
	s.addSorted(c.unbox(e.self()))
	return s
}

// tenumSetOfArr_ is EnumSet.of(e, rest...).
func tenumSetOfArr_[K comparable](c *tconv_[K], e *Enum, rest []*Enum) *TSet[K] {
	s := tenumSetOf_(c, e)
	for _, x := range rest {
		s.addSorted(c.unbox(x.self()))
	}
	return s
}

// tenumSetCopyOf_ is EnumSet.copyOf(collection).
func tenumSetCopyOf_[K comparable](c *tconv_[K], from Collection) *TSet[K] {
	s := tenumSetNoneOf_(c, nil)
	for _, x := range toSlice_(from) {
		s.addSorted(c.unbox(x))
	}
	return s
}

// ---- kind `deque`: Queue and Deque as typed interfaces, as the kinds of rt_lg.go. The typed
// deque of rule "typed collections" (TDeque, rt_tdeque.go) is native through its erased view;
// any other object (an erased ArrayDeque, a LinkedList, a PriorityQueue) through a wrapper. The
// wrappers call the erased object through the interface of the one method (the erased
// interfaces of g_zstubs.go have only the methods some call names).

type QueueT[T any] interface {
	Queue
	iterator_t() IteratorT[T]
	add_t(x T) bool
	peek_t() T
	poll_t() T
	remove_0_t() T
}

type DequeT[T any] interface {
	Deque
	iterator_t() IteratorT[T]
	add_t(x T) bool
	peek_t() T
	poll_t() T
	remove_0_t() T
	pop_t() T
	push_t(x T)
}

// Queue_un_ is typed value x as erased code knows it.
func Queue_un_(x Queue) Queue {
	if w, ok := x.(lgWrap_); ok {
		u, _ := w.unwrap_().(Queue)
		return u
	}
	return x
}

// Deque_un_ is typed value x as erased code knows it.
func Deque_un_(x Deque) Deque {
	if w, ok := x.(lgWrap_); ok {
		u, _ := w.unwrap_().(Deque)
		return u
	}
	return x
}

type queueAs[T any] struct {
	Queue
	c *tconv_[T]
}

func (w *queueAs[T]) iterator_t() IteratorT[T] { return iteratorAs_(w.c, w.Queue.iterator()) }
func (w *queueAs[T]) add_t(x T) bool           { return w.Queue.add(w.c.box(x)) }
func (w *queueAs[T]) peek_t() T {
	return w.c.unbox(w.Queue.(interface{ peek() any }).peek())
}
func (w *queueAs[T]) poll_t() T {
	return w.c.unbox(w.Queue.(interface{ poll() any }).poll())
}
func (w *queueAs[T]) remove_0_t() T {
	return w.c.unbox(w.Queue.(interface{ remove_0() any }).remove_0())
}
func (w *queueAs[T]) unwrap_() any { return w.Queue }

// queueAs_ is erased v as a Queue of T: itself if its object is one (the view of a typed
// deque), else a wrapper.
func queueAs_[T any](c *tconv_[T], v Queue) QueueT[T] {
	if v == nil {
		return nil
	}
	if t, ok := v.(QueueT[T]); ok {
		return t
	}
	return &queueAs[T]{v, c}
}

type dequeAs[T any] struct {
	Deque
	c *tconv_[T]
}

func (w *dequeAs[T]) iterator_t() IteratorT[T] { return iteratorAs_(w.c, w.Deque.iterator()) }
func (w *dequeAs[T]) add_t(x T) bool           { return w.Deque.add(w.c.box(x)) }
func (w *dequeAs[T]) peek_t() T {
	return w.c.unbox(w.Deque.(interface{ peek() any }).peek())
}
func (w *dequeAs[T]) poll_t() T {
	return w.c.unbox(w.Deque.(interface{ poll() any }).poll())
}
func (w *dequeAs[T]) remove_0_t() T {
	return w.c.unbox(w.Deque.(interface{ remove_0() any }).remove_0())
}
func (w *dequeAs[T]) pop_t() T     { return w.c.unbox(w.Deque.(interface{ pop() any }).pop()) }
func (w *dequeAs[T]) push_t(x T)   { w.Deque.(interface{ push(any) }).push(w.c.box(x)) }
func (w *dequeAs[T]) unwrap_() any { return w.Deque }

// dequeAs_ is erased v as a Deque of T: itself if its object is one, else a wrapper.
func dequeAs_[T any](c *tconv_[T], v Deque) DequeT[T] {
	if v == nil {
		return nil
	}
	if t, ok := v.(DequeT[T]); ok {
		return t
	}
	return &dequeAs[T]{v, c}
}

// asDequeT / asQueueT are a typed deque as the Deque / Queue of its elements: its erased view,
// which has the typed methods too (nil for null).
func (d *TDeque[T]) asDequeT() DequeT[T] {
	if d == nil {
		return nil
	}
	return &d.dview
}

func (d *TDeque[T]) asQueueT() QueueT[T] {
	if d == nil {
		return nil
	}
	return &d.dview
}

func (v *tdequeView_[T]) peek_t() T     { return v.d.peek() }
func (v *tdequeView_[T]) poll_t() T     { return v.d.poll() }
func (v *tdequeView_[T]) remove_0_t() T { return v.d.pop() }
func (v *tdequeView_[T]) pop_t() T      { return v.d.pop() }
func (v *tdequeView_[T]) push_t(x T)    { v.d.push(x) }
