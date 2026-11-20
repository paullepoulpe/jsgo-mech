package closure

import "maps"

// varMap_ is a LinkedHashMap<Var, V> (post-flip manual step): insertion order, keyed by
// what Var.equals compares (ScopedName: the name and the scope root), so a lookup compares
// a string and a handle instead of calling ScopedName.hashCode and equals through an
// interface. Up to varSmall_ entries it is searched linearly (a key compares a string and a
// handle); above, a Go map indexes it. The threshold is higher than the runtime tables' because
// data-flow analysis copies these maps at every step, and a copy must clone the index.
type varMap_[V any] struct {
	ents []varEnt_[V] // in insertion order
	idx  map[varKey_]int32
}

// varEnt_ is one mapping: the key, the variable it came from, the value, side by side.
type varEnt_[V any] struct {
	k   varKey_
	v   *Var
	val V
}

const varSmall_ = 32

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
	for i := range m.ents {
		if m.ents[i].k == k {
			return int32(i)
		}
	}
	return -1
}

func (m *varMap_[V]) size() int32 { return int32(len(m.ents)) }

// get returns the value of v, and whether v has one (containsKey).
func (m *varMap_[V]) get(v *Var) (V, bool) {
	if v == nil { // get(null): no Var key is null here
		var z V
		return z, false
	}
	if i := m.find(keyOfVar_(v)); i >= 0 {
		return m.ents[i].val, true
	}
	var z V
	return z, false
}

func (m *varMap_[V]) put(v *Var, x V) {
	k := keyOfVar_(v)
	if i := m.find(k); i >= 0 {
		m.ents[i].val = x
		return
	}
	if m.ents == nil {
		m.ents = make([]varEnt_[V], 0, 4)
	}
	m.ents = append(m.ents, varEnt_[V]{k, v, x})
	if m.idx != nil {
		m.idx[k] = int32(len(m.ents) - 1)
	} else if len(m.ents) > varSmall_ {
		m.idx = make(map[varKey_]int32, len(m.ents))
		for i := range m.ents {
			m.idx[m.ents[i].k] = int32(i)
		}
	}
}

// copyFrom makes m a copy of o (new LinkedHashMap; putAll into an empty map), with room for a
// few more entries, which data-flow steps usually add.
func (m *varMap_[V]) copyFrom(o *varMap_[V]) {
	m.ents = append(make([]varEnt_[V], 0, len(o.ents)+4), o.ents...)
	if o.idx != nil {
		m.idx = maps.Clone(o.idx)
	}
}

// equalsWith is Map.equals: the same keys, and values equal by eq.
func (m *varMap_[V]) equalsWith(o *varMap_[V], eq func(a, b V) bool) bool {
	if len(m.ents) != len(o.ents) {
		return false
	}
	for i := range m.ents {
		j := o.find(m.ents[i].k)
		if j < 0 || !eq(m.ents[i].val, o.ents[j].val) {
			return false
		}
	}
	return true
}

// hashWith is Map.hashCode: the sum over entries of key hash ^ value hash.
func (m *varMap_[V]) hashWith(h func(V) int32) int32 {
	var s int32
	for i := range m.ents {
		s += m.ents[i].v.hashCode() ^ h(m.ents[i].val)
	}
	return s
}

// varList is keySet() as an erased list of the variables, in insertion order.
func (m *varMap_[V]) varList() List {
	items := make([]any, len(m.ents))
	for i := range m.ents {
		items[i] = m.ents[i].v.self()
	}
	return listOf_(items)
}

// vkey_ is v's key in a Go map keyed by what Var.equals compares (tools/postflip/native): the
// name and the scope root. A null Var has a key of its own: no node handle reaches the
// largest uint32.
func vkey_(v *Var) varKey_ {
	if v == nil {
		return varKey_{root: Node(^uint32(0))}
	}
	return varKey_{v.getName(), v.getScopeRoot()}
}

// exportTraceKey_ is what ExportTrace.equals compares (an AutoValue of the module, compared by
// identity, and the export name), for a Go map keyed by it; null has a key of its own.
type exportTraceKey_ struct {
	module *UnresolvedModule
	name   string
	null   bool
}

func etkey_(t *ExportTrace) exportTraceKey_ {
	if t == nil {
		return exportTraceKey_{null: true}
	}
	return exportTraceKey_{t.module_, t.exportName_, false}
}
