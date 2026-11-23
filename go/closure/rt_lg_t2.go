package closure

// Rule "typed library interfaces", the kinds of rule candidate T2 (docs/javaisms6/JAVAISMS6.md;
// tools/j2go LibGen.CLASSES_T2, LibGen.MAKERS_T2, LibGen.KINDS `deque`): the typed makers and
// typed interfaces of library types that the earlier kinds did not reach. Each maker makes the
// typed collection of the same kind as the runtime's erased one, for a place that holds the
// typed interface.

// ---- kind `enumset`: EnumSet is the runtime's TreeSet ordered by ordinal (rt_coll2.go); its
// typed twin is the typed set in its sorted mode with the same comparator (rt_tsorted.go).

// tenumSetNoneOf_ is EnumSet.noneOf(c).
func tenumSetNoneOf_[K comparable](c *jtconv_[K], cl *Class) *JTSet[K] {
	return newTSortedSetC_(c, ordinalCmp_())
}

// tenumSetAllOf_ is EnumSet.allOf(c): the constants in their declaration order.
func tenumSetAllOf_[K comparable](c *jtconv_[K], cl *Class) *JTSet[K] {
	s := tenumSetNoneOf_(c, cl)
	for _, x := range enumRegistry_[cl.name_] {
		s.addSorted(c.unbox(x))
	}
	return s
}

// tenumSetOf_ is EnumSet.of(e).
func tenumSetOf_[K comparable](c *jtconv_[K], e *Enum) *JTSet[K] {
	s := tenumSetNoneOf_(c, nil)
	s.addSorted(c.unbox(e.self()))
	return s
}

// tenumSetOfArr_ is EnumSet.of(e, rest...).
func tenumSetOfArr_[K comparable](c *jtconv_[K], e *Enum, rest []*Enum) *JTSet[K] {
	s := tenumSetOf_(c, e)
	for _, x := range rest {
		s.addSorted(c.unbox(x.self()))
	}
	return s
}

// tenumSetCopyOf_ is EnumSet.copyOf(collection).
func tenumSetCopyOf_[K comparable](c *jtconv_[K], from Collection) *JTSet[K] {
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
	c *jtconv_[T]
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
func queueAs_[T any](c *jtconv_[T], v Queue) QueueT[T] {
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
	c *jtconv_[T]
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
func dequeAs_[T any](c *jtconv_[T], v Deque) DequeT[T] {
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

func (v *jtdequeView_[T]) peek_t() T     { return v.d.peek() }
func (v *jtdequeView_[T]) poll_t() T     { return v.d.poll() }
func (v *jtdequeView_[T]) remove_0_t() T { return v.d.pop() }
func (v *jtdequeView_[T]) pop_t() T      { return v.d.pop() }
func (v *jtdequeView_[T]) push_t(x T)    { v.d.push(x) }

// ---- kind `guava`: PeekingIterator and LoadingCache as typed interfaces. The runtime makes
// both erased (Iterators.peekingIterator, CacheBuilder.build), so every value is a wrapper.

type PeekingIteratorT[T any] interface {
	PeekingIterator
	next_t() T
	peek_t() T
}

// PeekingIterator_un_ is typed value x as erased code knows it.
func PeekingIterator_un_(x PeekingIterator) PeekingIterator {
	if w, ok := x.(lgWrap_); ok {
		u, _ := w.unwrap_().(PeekingIterator)
		return u
	}
	return x
}

type peekingIteratorAs[T any] struct {
	PeekingIterator
	c *jtconv_[T]
}

func (w *peekingIteratorAs[T]) next_t() T { return w.c.unbox(w.PeekingIterator.next()) }
func (w *peekingIteratorAs[T]) peek_t() T {
	return w.c.unbox(w.PeekingIterator.(interface{ peek() any }).peek())
}
func (w *peekingIteratorAs[T]) unwrap_() any { return w.PeekingIterator }

// peekingIteratorAs_ is erased v as a PeekingIterator of T: itself if it is one, else a wrapper.
func peekingIteratorAs_[T any](c *jtconv_[T], v PeekingIterator) PeekingIteratorT[T] {
	if v == nil {
		return nil
	}
	if t, ok := v.(PeekingIteratorT[T]); ok {
		return t
	}
	return &peekingIteratorAs[T]{v, c}
}

type LoadingCacheT[K any, V any] interface {
	LoadingCache
	getUnchecked_t(k K) V
	get_t(k K) V
}

// LoadingCache_un_ is typed value x as erased code knows it.
func LoadingCache_un_(x LoadingCache) LoadingCache {
	if w, ok := x.(lgWrap_); ok {
		u, _ := w.unwrap_().(LoadingCache)
		return u
	}
	return x
}

type loadingCacheAs[K any, V any] struct {
	LoadingCache
	kc *jtconv_[K]
	vc *jtconv_[V]
}

func (w *loadingCacheAs[K, V]) getUnchecked_t(k K) V {
	return w.vc.unbox(w.LoadingCache.(interface{ getUnchecked(any) any }).getUnchecked(w.kc.box(k)))
}
func (w *loadingCacheAs[K, V]) get_t(k K) V {
	return w.vc.unbox(w.LoadingCache.(interface{ get(any) any }).get(w.kc.box(k)))
}
func (w *loadingCacheAs[K, V]) unwrap_() any { return w.LoadingCache }

// loadingCacheAs_ is erased v as a LoadingCache of K and V: itself if it is one, else a wrapper.
func loadingCacheAs_[K any, V any](kc *jtconv_[K], vc *jtconv_[V], v LoadingCache) LoadingCacheT[K, V] {
	if v == nil {
		return nil
	}
	if t, ok := v.(LoadingCacheT[K, V]); ok {
		return t
	}
	return &loadingCacheAs[K, V]{v, kc, vc}
}
