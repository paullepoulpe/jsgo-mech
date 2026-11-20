package closure

// The erased views of the typed collections as steppers (stepper_, rt_coll.go): a for-each
// loop over one (foreach_, elems_) reads it by position with the loop's own cursor, where
// iterator() makes an iterator object (and, for a map's views, the views it goes through) for
// every loop. Each step reads what the view's iterator reads at that step: the list's items or
// the map's keys as they are now, a removed entry of a map skipped, the element boxed as the
// view boxes it.

func (v *tlistView_[T]) step_(i int) (any, int) {
	l := v.tl()
	if i >= len(l.items) {
		return nil, -1
	}
	return l.c.box(l.items[i]), i + 1
}

// tmapStep_ is the step of a view of m whose elements are out(m, i).
func tmapStep_[K comparable, V any](m *TMap[K, V], i int, out func(m *TMap[K, V], i int32) any) (any, int) {
	if i == 0 {
		i = m.first()
	}
	for i < len(m.keys) && m.isDead(i) {
		i++
	}
	if i >= len(m.keys) {
		return nil, -1
	}
	return out(m, int32(i)), i + 1
}

func (v *tkeysView_[K, V]) step_(i int) (any, int)    { return tmapStep_(v.m, i, keyOut_[K, V]) }
func (v *tvalsView_[K, V]) step_(i int) (any, int)    { return tmapStep_(v.m, i, valOut_[K, V]) }
func (v *tentriesView_[K, V]) step_(i int) (any, int) { return tmapStep_(v.m, i, entryOut_[K, V]) }
func (v *tsetView_[K]) step_(i int) (any, int) {
	return tmapStep_(v.tm(), i, keyOut_[K, struct{}])
}
