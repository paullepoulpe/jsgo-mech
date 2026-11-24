package closure

// A stub of the runtime's typed collections (rt_tlist.go, rt_tmap.go) and the helpers the
// cases use.

type tconv_[T any] struct{ null T }

var tc_string = &tconv_[string]{}

type TList[T any] struct {
	items []T
	c     *tconv_[T]
}

func newTList_[T any](c *tconv_[T]) *TList[T] { return &TList[T]{c: c} }
func newTListN_[T any](c *tconv_[T], n int32) *TList[T] {
	return &TList[T]{c: c, items: make([]T, 0, n)}
}
func initTList_[T any](l *TList[T], c *tconv_[T]) { *l = TList[T]{c: c} }
func initTListN_[T any](l *TList[T], c *tconv_[T], n int32) {
	*l = TList[T]{c: c, items: make([]T, 0, n)}
}
func tlistCopy_[T any](c *tconv_[T], o *TList[T]) *TList[T] {
	return &TList[T]{c: c, items: append([]T(nil), o.items...)}
}
func (l *TList[T]) add(v T) bool { l.items = append(l.items, v); return true }
func (l *TList[T]) size() int32  { return int32(len(l.items)) }

type TMap[K comparable, V any] struct {
	keys []K
	vals []V
	kc   *tconv_[K]
	vc   *tconv_[V]
}

func newTMap_[K comparable, V any](kc *tconv_[K], vc *tconv_[V]) *TMap[K, V] {
	return &TMap[K, V]{kc: kc, vc: vc}
}
func newTMapN_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], n int32) *TMap[K, V] {
	return &TMap[K, V]{kc: kc, vc: vc}
}
func initTMap_[K comparable, V any](m *TMap[K, V], kc *tconv_[K], vc *tconv_[V]) {
	*m = TMap[K, V]{kc: kc, vc: vc}
}
func initTMapN_[K comparable, V any](m *TMap[K, V], kc *tconv_[K], vc *tconv_[V], n int32) {
	*m = TMap[K, V]{kc: kc, vc: vc}
}
func (m *TMap[K, V]) Put(k K, v V) { m.keys = append(m.keys, k); m.vals = append(m.vals, v) }

type TSet[K comparable] struct {
	TMap[K, struct{}]
}

func newTSet_[K comparable](kc *tconv_[K]) *TSet[K]           { return &TSet[K]{TMap[K, struct{}]{kc: kc}} }
func newTSetN_[K comparable](kc *tconv_[K], n int32) *TSet[K] { return newTSet_(kc) }
func initTSet_[K comparable](s *TSet[K], kc *tconv_[K])       { *s = TSet[K]{TMap[K, struct{}]{kc: kc}} }
func initTSetN_[K comparable](s *TSet[K], kc *tconv_[K], n int32) {
	initTSet_(s, kc)
}
func (s *TSet[K]) Add(k K) { s.Put(k, struct{}{}) }

func obj_(v any) any { return v }

func Object_clone(v any) any { return v }

func useList(l *TList[string]) int32 { return l.size() }

var registry []any

func register(o any) { registry = append(registry, o) }

// RtReader is the runtime reading a field of a generated class.
func RtReader(r *RtRead) int32 { return r.f.size() }
