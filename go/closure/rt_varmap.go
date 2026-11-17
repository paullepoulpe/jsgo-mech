package closure

import "hash/maphash"

// varMap_ is a LinkedHashMap<Var, V> (post-flip manual step): insertion order, keyed by
// what Var.equals compares (ScopedName: the name and the scope root), so a lookup compares
// a string and a handle instead of calling ScopedName.hashCode and equals through an
// interface. Each entry keeps the key's hash, which a Var computes once (vkHash_). Up to
// varSmall_ entries the entries are scanned by hash; above, an open-addressed slot array
// (hash and 1 + position in one word, as the runtime hash tables) indexes them.
type varMap_[V any] struct {
	ents  []varEnt_[V] // in insertion order
	slots []uint64
}

// varEnt_ is one mapping: the key and its hash, the variable it came from, the value.
type varEnt_[V any] struct {
	k   varKey_
	h   uint32
	v   *Var
	val V
}

const varSmall_ = 32

type varKey_ struct {
	name string
	root Node
}

var varSeed_ = maphash.MakeSeed()

// keyOfVar_ is v's key and its hash; the hash is computed once per Var (name and scope root
// never change).
func keyOfVar_(v *Var) (varKey_, uint32) {
	k := varKey_{v.getName(), v.getScopeRoot()}
	if v.vkHash_ == 0 {
		v.vkHash_ = (uint32(maphash.String(varSeed_, k.name)) ^ hmix_(int32(k.root))) | 1
	}
	return k, v.vkHash_
}

func (m *varMap_[V]) find(k varKey_, h uint32) int32 {
	if m.slots == nil {
		for i := range m.ents {
			if e := &m.ents[i]; e.h == h && e.k == k {
				return int32(i)
			}
		}
		return -1
	}
	for j, mask := h&uint32(len(m.slots)-1), uint32(len(m.slots)-1); m.slots[j] != 0; j = (j + 1) & mask {
		if sl := m.slots[j]; uint32(sl>>32) == h {
			if i := int32(uint32(sl)) - 1; m.ents[i].k == k {
				return i
			}
		}
	}
	return -1
}

// place puts position i, of a key with hash h, in a free slot.
func (m *varMap_[V]) place(h uint32, i int32) {
	mask := uint32(len(m.slots) - 1)
	j := h & mask
	for m.slots[j] != 0 {
		j = (j + 1) & mask
	}
	m.slots[j] = uint64(h)<<32 | uint64(uint32(i+1))
}

func (m *varMap_[V]) rebuild() {
	n := 64
	for n*3 < len(m.ents)*8 {
		n *= 2
	}
	m.slots = make([]uint64, n)
	for i := range m.ents {
		m.place(m.ents[i].h, int32(i))
	}
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
	k, h := keyOfVar_(v)
	if i := m.find(k, h); i >= 0 {
		m.ents[i].val = x
		return
	}
	m.add(k, h, v, x)
}

// slot is the value of v as a variable, adding v with the zero value (null) at the end if it
// has no mapping: Map.computeIfAbsent(v, f) with one lookup is
//
//	p := m.slot(v)
//	if *p == nil {
//		*p = f(v)
//	}
//
// for an f that never returns null (Java's computeIfAbsent puts f's value at the same place,
// and treats a key mapped to null as absent, as the test of *p does). The pointer is good
// until the next change of m.
func (m *varMap_[V]) slot(v *Var) *V {
	k, h := keyOfVar_(v)
	i := m.find(k, h)
	if i < 0 {
		var z V
		m.add(k, h, v, z)
		i = int32(len(m.ents) - 1)
	}
	return &m.ents[i].val
}

// add appends the mapping of v, which has none (key k, hash h).
func (m *varMap_[V]) add(k varKey_, h uint32, v *Var, x V) {
	if m.ents == nil {
		m.ents = make([]varEnt_[V], 0, 4)
	}
	m.ents = append(m.ents, varEnt_[V]{k, h, v, x})
	if m.slots != nil {
		if len(m.ents)*4 > len(m.slots)*3 {
			m.rebuild()
		} else {
			m.place(h, int32(len(m.ents)-1))
		}
	} else if len(m.ents) > varSmall_ {
		m.rebuild()
	}
}

// copyFrom makes m a copy of o (new LinkedHashMap; putAll into an empty map), with room for a
// few more entries, which data-flow steps usually add.
func (m *varMap_[V]) copyFrom(o *varMap_[V]) {
	m.ents = append(make([]varEnt_[V], 0, len(o.ents)+4), o.ents...)
	if o.slots != nil {
		m.slots = append([]uint64(nil), o.slots...)
	}
}

// equalsWith is Map.equals: the same keys, and values equal by eq.
func (m *varMap_[V]) equalsWith(o *varMap_[V], eq func(a, b V) bool) bool {
	if len(m.ents) != len(o.ents) {
		return false
	}
	for i := range m.ents {
		j := o.find(m.ents[i].k, m.ents[i].h)
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
