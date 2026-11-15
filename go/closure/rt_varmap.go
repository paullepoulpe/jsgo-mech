package closure

import "maps"

// varMap_ is a LinkedHashMap<Var, V> (post-flip manual step): insertion order, keyed by
// what Var.equals compares (ScopedName: the name and the scope root), so a lookup compares
// a string and a handle instead of calling ScopedName.hashCode and equals through an
// interface. Up to tsmall_ entries it is searched linearly; above, a Go map indexes it.
type varMap_[V any] struct {
	keys []varKey_
	vars []*Var
	vals []V
	idx  map[varKey_]int32
}

type varKey_ struct {
	name string
	root Node
}

func keyOfVar_(v *Var) varKey_ { return varKey_{v.getName(), v.getScopeRoot()} }

func (m *varMap_[V]) find(k varKey_) int32 {
	if m.idx != nil {
		if i, ok := m.idx[k]; ok {
			return i
		}
		return -1
	}
	for i, x := range m.keys {
		if x == k {
			return int32(i)
		}
	}
	return -1
}

func (m *varMap_[V]) size() int32 { return int32(len(m.keys)) }

// get returns the value of v, and whether v has one (containsKey).
func (m *varMap_[V]) get(v *Var) (V, bool) {
	if v == nil { // get(null): no Var key is null here
		var z V
		return z, false
	}
	if i := m.find(keyOfVar_(v)); i >= 0 {
		return m.vals[i], true
	}
	var z V
	return z, false
}

func (m *varMap_[V]) put(v *Var, x V) {
	k := keyOfVar_(v)
	if i := m.find(k); i >= 0 {
		m.vals[i] = x
		return
	}
	if m.keys == nil {
		m.keys, m.vars, m.vals = make([]varKey_, 0, 4), make([]*Var, 0, 4), make([]V, 0, 4)
	}
	m.keys = append(m.keys, k)
	m.vars = append(m.vars, v)
	m.vals = append(m.vals, x)
	if m.idx != nil {
		m.idx[k] = int32(len(m.keys) - 1)
	} else if len(m.keys) > tsmall_ {
		m.idx = make(map[varKey_]int32, len(m.keys))
		for i, x := range m.keys {
			m.idx[x] = int32(i)
		}
	}
}

// copyFrom makes m a copy of o (new LinkedHashMap; putAll into an empty map).
func (m *varMap_[V]) copyFrom(o *varMap_[V]) {
	m.keys = append([]varKey_(nil), o.keys...)
	m.vars = append([]*Var(nil), o.vars...)
	m.vals = append([]V(nil), o.vals...)
	if o.idx != nil {
		m.idx = maps.Clone(o.idx)
	}
}

// equalsWith is Map.equals: the same keys, and values equal by eq.
func (m *varMap_[V]) equalsWith(o *varMap_[V], eq func(a, b V) bool) bool {
	if len(m.keys) != len(o.keys) {
		return false
	}
	for i, k := range m.keys {
		j := o.find(k)
		if j < 0 || !eq(m.vals[i], o.vals[j]) {
			return false
		}
	}
	return true
}

// hashWith is Map.hashCode: the sum over entries of key hash ^ value hash.
func (m *varMap_[V]) hashWith(h func(V) int32) int32 {
	var s int32
	for i, v := range m.vars {
		s += v.hashCode() ^ h(m.vals[i])
	}
	return s
}

// varList is keySet() as an erased list of the variables, in insertion order.
func (m *varMap_[V]) varList() List {
	items := make([]any, len(m.vars))
	for i, v := range m.vars {
		items[i] = v.self()
	}
	return listOf_(items)
}
