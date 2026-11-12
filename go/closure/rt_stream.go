package closure

// java.util.stream (Stream, IntStream, Collectors), Guava's Streams and the
// Immutable* collectors, plus static helpers of functional interfaces.
//
// Streams are evaluated eagerly: every stream holds its elements in a slice
// and each intermediate operation produces a new stream. Encounter order is
// always preserved.

type stream_ struct {
	items []any
}

func newStream_(items []any) *stream_ { return &stream_{items: items} }

// streamItems_ returns the elements of any Stream (nil stream: no elements).
func streamItems_(s any) []any {
	switch x := s.(type) {
	case nil:
		return nil
	case *stream_:
		if x == nil {
			return nil
		}
		return x.items
	case *streamBuilder_:
		return x.items
	case *intStream_:
		return x.boxed().(*stream_).items
	case Iterable:
		return toSlice_(x)
	}
	panic(new_IllegalArgumentException_String("not a stream: " + goTypeName_(s)).self())
}

func streamCopy_(a []any) []any {
	r := make([]any, len(a))
	copy(r, a)
	return r
}

func streamNpe_(msg string) any {
	if msg == "" {
		return new_NullPointerException_0().self()
	}
	return new_NullPointerException_String(msg).self()
}

// ---- BaseStream

func (this *stream_) close()                  {}
func (this *stream_) iterator() Iterator      { return &sliceIter_{items: &this.items} }
func (this *stream_) sequential() Stream      { return this }
func (this *stream_) parallel() Stream        { return this }
func (this *stream_) unordered() Stream       { return this }
func (this *stream_) isParallel() bool        { return false }
func (this *stream_) onClose(Runnable) Stream { return this }

// ---- intermediate operations

func (this *stream_) filter(p Predicate_function) Stream {
	r := make([]any, 0, len(this.items))
	for _, v := range this.items {
		if p.test(v) {
			r = append(r, v)
		}
	}
	return newStream_(r)
}

func (this *stream_) map_(f Function) Stream {
	r := make([]any, len(this.items))
	for i, v := range this.items {
		r[i] = f.apply(v)
	}
	return newStream_(r)
}

func (this *stream_) flatMap(f Function) Stream {
	var r []any
	for _, v := range this.items {
		r = append(r, streamItems_(f.apply(v))...)
	}
	return newStream_(r)
}

func (this *stream_) distinct() Stream {
	seen := new_LinkedHashSet()
	r := make([]any, 0, len(this.items))
	for _, v := range this.items {
		if seen.add(v) {
			r = append(r, v)
		}
	}
	return newStream_(r)
}

func (this *stream_) sorted(c Comparator) Stream {
	r := streamCopy_(this.items)
	sortSlice_(r, c)
	return newStream_(r)
}

func (this *stream_) sorted_0() Stream { return this.sorted(nil) }

func (this *stream_) peek(c Consumer) Stream {
	for _, v := range this.items {
		c.accept(v)
	}
	return this
}

func (this *stream_) limit(n int64) Stream {
	if n < 0 {
		panic(new_IllegalArgumentException_String(istr_(n)).self())
	}
	if n >= int64(len(this.items)) {
		return this
	}
	return newStream_(this.items[:n:n])
}

func (this *stream_) skip(n int64) Stream {
	if n < 0 {
		panic(new_IllegalArgumentException_String(istr_(n)).self())
	}
	if n >= int64(len(this.items)) {
		return newStream_(nil)
	}
	return newStream_(this.items[n:])
}

func (this *stream_) mapToInt(f ToIntFunction) IntStream {
	r := make([]int32, len(this.items))
	for i, v := range this.items {
		r[i] = f.applyAsInt(v)
	}
	return &intStream_{items: r}
}

// ---- terminal operations

func (this *stream_) forEach(c Consumer) {
	for _, v := range this.items {
		c.accept(v)
	}
}

func (this *stream_) forEachOrdered(c Consumer) { this.forEach(c) }

func (this *stream_) allMatch(p Predicate_function) bool {
	for _, v := range this.items {
		if !p.test(v) {
			return false
		}
	}
	return true
}

func (this *stream_) anyMatch(p Predicate_function) bool {
	for _, v := range this.items {
		if p.test(v) {
			return true
		}
	}
	return false
}

func (this *stream_) noneMatch(p Predicate_function) bool { return !this.anyMatch(p) }

func (this *stream_) count() int64 { return int64(len(this.items)) }

func (this *stream_) findFirst() *Optional_util {
	if len(this.items) == 0 {
		return Optional_util_empty()
	}
	if this.items[0] == nil {
		panic(streamNpe_(""))
	}
	return Optional_util_of(this.items[0])
}

func (this *stream_) findAny() *Optional_util { return this.findFirst() }

func (this *stream_) min(c Comparator) *Optional_util {
	if len(this.items) == 0 {
		return Optional_util_empty()
	}
	best := this.items[0]
	for _, v := range this.items[1:] {
		if c.compare(v, best) < 0 {
			best = v
		}
	}
	if best == nil {
		panic(streamNpe_(""))
	}
	return Optional_util_of(best)
}

func (this *stream_) max(c Comparator) *Optional_util {
	if len(this.items) == 0 {
		return Optional_util_empty()
	}
	best := this.items[0]
	for _, v := range this.items[1:] {
		// On ties the later element wins, as BinaryOperator.maxBy does.
		if c.compare(v, best) >= 0 {
			best = v
		}
	}
	if best == nil {
		panic(streamNpe_(""))
	}
	return Optional_util_of(best)
}

func (this *stream_) reduce(identity any, op BinaryOperator) any {
	r := identity
	for _, v := range this.items {
		r = op.apply(r, v)
	}
	return r
}

func (this *stream_) reduce_BinaryOperator(op BinaryOperator) *Optional_util {
	if len(this.items) == 0 {
		return Optional_util_empty()
	}
	r := this.items[0]
	for _, v := range this.items[1:] {
		r = op.apply(r, v)
	}
	if r == nil {
		panic(streamNpe_(""))
	}
	return Optional_util_of(r)
}

// toArray returns the elements as an Object[]; the generator is only a type
// witness in Java and erased arrays are all []any here.
func (this *stream_) toArray(IntFunction) []any { return streamCopy_(this.items) }

func (this *stream_) toArray_0() []any { return streamCopy_(this.items) }

func (this *stream_) toList() List { return listOf_(streamCopy_(this.items)) }

func (this *stream_) collect(c Collector) any {
	if k, ok := c.(*collector_); ok && k != nil {
		return k.finish(streamCopy_(this.items))
	}
	panic(new_IllegalArgumentException_String("unsupported Collector: " + goTypeName_(c)).self())
}

// ---- Stream statics

func Collection_stream(c Collection) Stream {
	if c == nil {
		panic(streamNpe_(""))
	}
	return newStream_(streamCopy_(toSlice_(c)))
}

func Stream_of(v any) Stream { return newStream_([]any{v}) }

func Stream_of_ObjectArr(a []any) Stream { return newStream_(streamCopy_(a)) }

func Stream_concat(a Stream, b Stream) Stream {
	x, y := streamItems_(a), streamItems_(b)
	r := make([]any, 0, len(x)+len(y))
	r = append(r, x...)
	r = append(r, y...)
	return newStream_(r)
}

type streamBuilder_ struct {
	items []any
	built bool
}

func (this *streamBuilder_) accept(v any) {
	if this.built {
		panic(new_IllegalStateException_0().self())
	}
	this.items = append(this.items, v)
}

func (this *streamBuilder_) add(v any) Stream_Builder {
	this.accept(v)
	return this
}

func (this *streamBuilder_) build() Stream {
	if this.built {
		panic(new_IllegalStateException_0().self())
	}
	this.built = true
	return newStream_(this.items)
}

func Stream_builder() Stream_Builder { return &streamBuilder_{} }

func Stream_Builder_add(b Stream_Builder, v any) Stream_Builder {
	b.accept(v)
	return b
}

// ---- Guava Streams

func Streams_stream(it Iterable) Stream {
	if it == nil {
		panic(streamNpe_(""))
	}
	return newStream_(streamCopy_(toSlice_(it)))
}

func Streams_mapWithIndex(s Stream, f Streams_FunctionWithIndex) Stream {
	items := streamItems_(s)
	r := make([]any, len(items))
	for i, v := range items {
		r[i] = f.apply(v, int64(i))
	}
	return newStream_(r)
}

// ---- IntStream

type intStream_ struct {
	items []int32
}

func IntStream_range(from int32, to int32) IntStream {
	s := &intStream_{}
	if from < to {
		s.items = make([]int32, 0, int(int64(to)-int64(from)))
		for i := int64(from); i < int64(to); i++ {
			s.items = append(s.items, int32(i))
		}
	}
	return s
}

func (this *intStream_) close() {}

func (this *intStream_) mapToObj(f IntFunction) Stream {
	r := make([]any, len(this.items))
	for i, v := range this.items {
		r[i] = f.apply(v)
	}
	return newStream_(r)
}

func (this *intStream_) boxed() Stream {
	r := make([]any, len(this.items))
	for i, v := range this.items {
		r[i] = v
	}
	return newStream_(r)
}

func (this *intStream_) toArray() []int32 {
	r := make([]int32, len(this.items))
	copy(r, this.items)
	return r
}

func (this *intStream_) sum() int32 {
	var s int32
	for _, v := range this.items {
		s += v
	}
	return s
}

func (this *intStream_) count() int64 { return int64(len(this.items)) }

// ---- functional interface helpers

var streamIdentity_ Function = &Function_fn{func(x any) any { return x }}

func Function_identity() Function { return streamIdentity_ }

func Predicate_function_isEqual(target any) Predicate_function {
	return &Predicate_function_fn{func(x any) bool { return objequals_(target, x) }}
}

// Predicate_test is the default method of Guava's Predicate: test == apply.
func Predicate_test(p Predicate, v any) bool { return p.apply(v) }
