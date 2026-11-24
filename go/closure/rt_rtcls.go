//go:build !j2go_nofntypes

package closure

// Piece `rtcls` of rule "typed library interfaces" (tools/j2go/src/j2go/LibRt.java): library
// classes the runtime has twice, the erased class (the translation without the piece names it)
// and the generic struct below, written after the erased one method by method: the same fields,
// the same steps, the values at their types. Erased code that meets one of these objects calls
// the erased methods, which box what the typed ones give (boxT_) as the erased class held it.

// ---- java.lang.ThreadLocal (rt_io_sys.go)

type ThreadLocalT[T any] struct {
	value_ T
	set_   bool
}

func new_ThreadLocalT[T any]() *ThreadLocalT[T] { return &ThreadLocalT[T]{} }

func (this *ThreadLocalT[T]) get() T {
	if !this.set_ {
		this.set_ = true
		this.value_ = null_[T]()
	}
	return this.value_
}

func (this *ThreadLocalT[T]) set(v T) {
	this.value_ = v
	this.set_ = true
}

func (this *ThreadLocalT[T]) remove() {
	this.value_ = null_[T]()
	this.set_ = false
}

// ---- Guava's AbstractIterator (rt_coll2.go): subclasses implement computeNext() and call
// endOfData() when done.

type AbstractIteratorT[T any] struct {
	self_ any
	state int8 // 0 not ready, 1 ready, 2 done
	next_ T
}

func (this *AbstractIteratorT[T]) init_AbstractIteratorT() {}

func (this *AbstractIteratorT[T]) as_AbstractIteratorT() *AbstractIteratorT[T] { return this }
func (this *AbstractIteratorT[T]) isa_AbstractIteratorT_()                     {}

func to_AbstractIteratorT[T any](v any) *AbstractIteratorT[T] {
	if v == nil {
		return nil
	}
	return v.(interface{ as_AbstractIteratorT() *AbstractIteratorT[T] }).as_AbstractIteratorT()
}

func is_AbstractIteratorT(v any) bool {
	_, ok := v.(interface{ isa_AbstractIteratorT_() })
	return ok
}

func (this *AbstractIteratorT[T]) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *AbstractIteratorT[T]) endOfData() T {
	this.state = 2
	return null_[T]()
}

func (this *AbstractIteratorT[T]) hasNext() bool {
	if this.state == 0 {
		v := this.self_.(interface{ computeNext() T }).computeNext()
		if this.state != 2 {
			this.next_ = v
			this.state = 1
		}
	}
	return this.state == 1
}

func (this *AbstractIteratorT[T]) next_t() T {
	if !this.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	v := this.next_
	this.next_ = null_[T]()
	this.state = 0
	return v
}

func (this *AbstractIteratorT[T]) peek() T {
	if !this.hasNext() {
		panic(new_NoSuchElementException_0().self())
	}
	return this.next_
}

// the erased Iterator method
func (this *AbstractIteratorT[T]) next() any { return boxT_(this.next_t()) }

// ---- Guava's CacheLoader and the cache CacheBuilder.build makes of it (rt_guava_cache.go).
// CacheBuilder.build takes either loader: an erased one makes the erased cache as before, a typed
// one (newCache_) the typed cache, whose map, size bound and eviction are the erased cache's
// (localCache_: the same object), with the loader called at its types.

type CacheLoaderT[K any, V any] struct {
	self_ any
}

func (this *CacheLoaderT[K, V]) init_CacheLoaderT() {}

func (this *CacheLoaderT[K, V]) as_CacheLoaderT() *CacheLoaderT[K, V] { return this }
func (this *CacheLoaderT[K, V]) isa_CacheLoaderT_()                   {}

func to_CacheLoaderT[K any, V any](v any) *CacheLoaderT[K, V] {
	if v == nil {
		return nil
	}
	return v.(interface{ as_CacheLoaderT() *CacheLoaderT[K, V] }).as_CacheLoaderT()
}

func is_CacheLoaderT(v any) bool {
	_, ok := v.(interface{ isa_CacheLoaderT_() })
	return ok
}

func (this *CacheLoaderT[K, V]) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *CacheLoaderT[K, V]) load_(key K) V {
	return this.self_.(interface{ load(K) V }).load(key)
}

// newCache_ is CacheBuilder.build of this loader with the builder's size bound.
func (this *CacheLoaderT[K, V]) newCache_(max int64) LoadingCache {
	c := &localCacheT_[K, V]{localCache_: &localCache_{map_: new_LinkedHashMap(), max_: max}, l: this}
	// (erased code calling the erased get / getUnchecked: the loader at Object, as the erased cache's)
	el := &functionCacheLoader_{f_: Function_base_of_(func(key any) any { return boxT_(this.load_(unboxT_[K](key))) })}
	el.self_ = el
	c.loader_ = &el.CacheLoader
	return c
}

// functionCacheLoaderT_ is CacheLoader.from(function).
type functionCacheLoaderT_[K any, V any] struct {
	CacheLoaderT[K, V]
	f_ Function_base[K, V]
}

func CacheLoaderT_from[K any, V any](f Function_base[K, V]) *CacheLoaderT[K, V] {
	if f == nil {
		panic(new_NullPointerException_0().self())
	}
	l := &functionCacheLoaderT_[K, V]{f_: f}
	l.self_ = l
	return &l.CacheLoaderT
}

func (this *functionCacheLoaderT_[K, V]) load(key K) V {
	if isnull_(key) {
		panic(new_NullPointerException_0().self())
	}
	return this.f_.apply(key)
}

// localCacheT_ is the typed view of a cache made from a typed loader (LoadingCacheT).
type localCacheT_[K any, V any] struct {
	*localCache_
	l *CacheLoaderT[K, V]
}

func (this *localCacheT_[K, V]) getUnchecked_t(k K) V {
	key := boxT_(k)
	if v := this.getIfPresent(key); v != nil {
		return unboxT_[V](v)
	}
	return unboxT_[V](this.load_(key, func() any { return boxT_(this.l.load_(k)) }, true))
}

func (this *localCacheT_[K, V]) get_t(k K) V {
	key := boxT_(k)
	if v := this.getIfPresent(key); v != nil {
		return unboxT_[V](v)
	}
	return unboxT_[V](this.load_(key, func() any { return boxT_(this.l.load_(k)) }, false))
}

// ---- Guava's Range (closed ranges) and TreeRangeSet, RangeSet (rt_guava_util.go): the
// endpoints at their type, compared as the erased ones compareNatural_ compares (a class with a
// typed compareTo_t, piece cmp: that method; else the boxes).

func cmpNaturalT_[C any](a, b C) int32 {
	if r, ok := cmpTypedT_(a, b); ok {
		return r
	}
	return compareNatural_(boxT_(a), boxT_(b))
}

type RangeT[C any] struct {
	lo, hi C
}

func (this *RangeT[C]) toString() string {
	return "[" + objstr_(boxT_(this.lo)) + "‥" + objstr_(boxT_(this.hi)) + "]"
}

func RangeT_closed[C any](p0 C, p1 C) *RangeT[C] {
	if isnull_(p0) || isnull_(p1) {
		throwNPE_("null")
	}
	r := &RangeT[C]{lo: p0, hi: p1}
	if cmpNaturalT_(p0, p1) > 0 {
		throwIAE_("Invalid range: " + r.toString())
	}
	return r
}

func (this *RangeT[C]) contains(p0 C) bool {
	if isnull_(p0) {
		throwNPE_("null")
	}
	return cmpNaturalT_(this.lo, p0) <= 0 && cmpNaturalT_(p0, this.hi) <= 0
}

type RangeSetT[C any] interface {
	add(*RangeT[C])
	contains(C) bool
	isEmpty() bool
}

type TreeRangeSetT[C any] struct {
	ranges []*RangeT[C] // disjoint and not connected, ascending
}

func TreeRangeSetT_create[C any]() *TreeRangeSetT[C] { return &TreeRangeSetT[C]{} }

func (this *TreeRangeSetT[C]) isEmpty() bool { return len(this.ranges) == 0 }

func (this *TreeRangeSetT[C]) contains(p0 C) bool {
	if isnull_(p0) {
		throwNPE_("null")
	}
	lo, hi := 0, len(this.ranges)
	for lo < hi {
		mid := (lo + hi) / 2
		r := this.ranges[mid]
		if cmpNaturalT_(p0, r.lo) < 0 {
			hi = mid
		} else if cmpNaturalT_(p0, r.hi) > 0 {
			lo = mid + 1
		} else {
			return true
		}
	}
	return false
}

// add coalesces the range with every range it is connected to.
func (this *TreeRangeSetT[C]) add(p0 *RangeT[C]) {
	if p0 == nil {
		throwNPE_("null")
	}
	lo, hi := p0.lo, p0.hi
	out := make([]*RangeT[C], 0, len(this.ranges)+1)
	placed := false
	for _, r := range this.ranges {
		switch {
		case cmpNaturalT_(r.hi, lo) < 0:
			out = append(out, r)
		case cmpNaturalT_(hi, r.lo) < 0:
			if !placed {
				out = append(out, &RangeT[C]{lo: lo, hi: hi})
				placed = true
			}
			out = append(out, r)
		default:
			if cmpNaturalT_(r.lo, lo) < 0 {
				lo = r.lo
			}
			if cmpNaturalT_(r.hi, hi) > 0 {
				hi = r.hi
			}
		}
	}
	if !placed {
		out = append(out, &RangeT[C]{lo: lo, hi: hi})
	}
	this.ranges = out
}

// ---- Guava's Ordering (rt_guava_util.go): translated classes extend it and override compare,
// at their type argument (rule "function types": a Comparator's method at its types).

type OrderingT[T any] struct {
	self_ any
	cmp_  func(a, b T) int32 // set for orderings made by this library
}

func (this *OrderingT[T]) init_OrderingT() {}

func (this *OrderingT[T]) as_OrderingT() *OrderingT[T] { return this }
func (this *OrderingT[T]) isa_OrderingT_()             {}

func to_OrderingT[T any](v any) *OrderingT[T] {
	if v == nil {
		return nil
	}
	return v.(interface{ as_OrderingT() *OrderingT[T] }).as_OrderingT()
}

func is_OrderingT(v any) bool {
	_, ok := v.(interface{ isa_OrderingT_() })
	return ok
}

func (this *OrderingT[T]) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *OrderingT[T]) compare(p0 T, p1 T) int32 {
	if this.cmp_ != nil {
		return this.cmp_(p0, p1)
	}
	return this.self_.(interface{ compare(T, T) int32 }).compare(p0, p1)
}

func orderingFromT_[T any](cmp func(a, b T) int32) *OrderingT[T] {
	this := &OrderingT[T]{cmp_: cmp}
	this.self_ = this
	return this
}

func OrderingT_natural[C any]() *OrderingT[C] {
	return orderingFromT_(func(a, b C) int32 {
		if isnull_(a) || isnull_(b) {
			throwNPE_("null")
		}
		return cmpNaturalT_(a, b)
	})
}

func (this *OrderingT[T]) reverse() *OrderingT[T] {
	return orderingFromT_(func(a, b T) int32 { return this.compare(b, a) })
}

func (this *OrderingT[T]) nullsFirst() *OrderingT[T] {
	return orderingFromT_(func(a, b T) int32 {
		switch {
		case isnull_(a) && isnull_(b):
			return 0
		case isnull_(a):
			return -1
		case isnull_(b):
			return 1
		}
		return this.compare(a, b)
	})
}

// sortedCopy is a stable sort into a new mutable list.
func (this *OrderingT[T]) sortedCopy(p0 IterableT[T]) ListT[T] {
	s := copySlice_(Iterable_un_(p0))
	sortSlice_(s, Comparator_of_(func(a, b any) int32 { return this.compare(unboxT_[T](a), unboxT_[T](b)) }))
	return listAs_(tcT_[T](), List(listOf_(s)))
}

// OrderingT_as_ is an Ordering of T as one of S, where Java's generic method gives the same object
// another type argument (Ordering.natural().nullsFirst(): Ordering<Comparable> -> Ordering<String>):
// itself if it is one, else the ordering of the boxes, as the erased Ordering compares them.
func OrderingT_as_[S any, T any](o *OrderingT[T]) *OrderingT[S] {
	if o == nil {
		return nil
	}
	if s, ok := any(o).(*OrderingT[S]); ok {
		return s
	}
	return orderingFromT_(func(a, b S) int32 { return o.compare(unboxT_[T](boxT_(a)), unboxT_[T](boxT_(b))) })
}
