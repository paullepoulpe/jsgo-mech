package closure

import (
	"hash/maphash"
	"unsafe"
)

// scopeVars_ is AbstractScope.vars, Java's LinkedHashMap<String, V> of the variables a scope
// declares (post-flip runtime): the entries in declaration order in one slice, the first four
// in the same allocation as the table, and once there are more than scopeVarsSmall_ an
// open-addressed slot array (the name's hash and 1 + position in one word, as varMap_).
//
// A typed map with string keys allocated a header and two slices, and above eight entries
// rehashed the name in a Go map at every scope of a lookup chain. Java hashes a name once
// (String caches its hashCode); here a lookup that walks the chain passes the hash along
// (lookup), computing it only when it first meets an indexed scope.
//
// An entry can also hold the id of its name in the table of interned strings (rt_ids.go), and
// an indexed table then has further slots keyed by that id: a lookup by the name of a node
// (Node.getStringId_: the id is what the node's record holds) compares ids and hashes no
// bytes (findId, lookupId). Equal strings have one id, so the two lookups find the same entry.
// The id comes with the declaration (putId: the declaring node holds it); interning a name
// that no node holds would hash it and keep it for good, so a table that was given a name
// without its id (put: the qualified names of a typed scope) keeps no ids, and a lookup by
// id there is a lookup by the id's string.
//
// Removal (undeclare, rare) closes the gap, so that the remaining entries keep their order,
// as in LinkedHashMap.
type scopeVars_ struct {
	ents  []scopeVarEnt_
	slots []uint64 // nil, or n slots by the name's hash and, unless noIds, n 32-bit slots by its id (n/2 words)
	view  *scopeVarsView_
	noIds bool // an entry was put without the id of its name
	first [4]scopeVarEnt_
}

// scopeVarEnt_ is an entry: the name as its bytes and length (a Go string would be two words;
// with a 32-bit length the id fits beside it and the entry is as large as name and variable
// alone), the name's string id, and the variable.
type scopeVarEnt_ struct {
	p  *byte
	n  uint32
	id uint32
	v  *AbstractVar
}

func (e *scopeVarEnt_) name() string { return unsafe.String(e.p, int(e.n)) }

const scopeVarsSmall_ = 8

var scopeVarsSeed_ = maphash.MakeSeed()

// scopeVarHash_ is the hash of a declared name; never 0, so that 0 means "not computed yet".
func scopeVarHash_(name string) uint32 { return uint32(maphash.String(scopeVarsSeed_, name)) | 1 }

func newScopeVars_() *scopeVars_ {
	t := &scopeVars_{}
	t.ents = t.first[:0]
	return t
}

// find is the position of name, or -1. h is the name's hash if *h != 0; find computes it
// there when it needs it, so that the caller can pass it on to the next table.
func (t *scopeVars_) find(name string, h *uint32) int {
	if t.slots == nil {
		for i := range t.ents {
			if t.ents[i].name() == name {
				return i
			}
		}
		return -1
	}
	if *h == 0 {
		*h = scopeVarHash_(name)
	}
	slots := t.slots[:t.nslots()]
	mask := uint32(len(slots) - 1)
	for j := *h & mask; slots[j] != 0; j = (j + 1) & mask {
		if sl := slots[j]; uint32(sl>>32) == *h {
			if i := int(uint32(sl)) - 1; t.ents[i].name() == name {
				return i
			}
		}
	}
	return -1
}

// scopeVarIdHash_ spreads the ids of interned strings (consecutive numbers) over the slots.
func scopeVarIdHash_(id uint32) uint32 {
	h := id * 0x9E3779B1
	return h ^ h>>15
}

// findId is the position of the name with string id `id`, or -1.
func (t *scopeVars_) findId(id uint32) int {
	if t.noIds {
		var h uint32
		return t.find(strid_str_(id), &h)
	}
	if t.slots == nil {
		for i := range t.ents {
			if t.ents[i].id == id {
				return i
			}
		}
		return -1
	}
	slots := t.idSlots()
	mask := uint32(len(slots) - 1)
	for j := scopeVarIdHash_(id) & mask; slots[j] != 0; j = (j + 1) & mask {
		if i := int(slots[j]) - 1; t.ents[i].id == id {
			return i
		}
	}
	return -1
}

// nslots is the number of slots keyed by the name's hash.
func (t *scopeVars_) nslots() int {
	if t.noIds {
		return len(t.slots)
	}
	return len(t.slots) / 3 * 2
}

// idSlots is the part of slots keyed by id: 1 + position, 0 for a free slot.
func (t *scopeVars_) idSlots() []uint32 {
	n := len(t.slots) / 3 * 2
	return unsafe.Slice((*uint32)(unsafe.Pointer(unsafe.SliceData(t.slots[n:]))), n)
}

// lookupId is get(name) for the name with string id `id`. A nil table has no variables.
func (t *scopeVars_) lookupId(id uint32) *AbstractVar {
	if t == nil {
		return nil
	}
	if i := t.findId(id); i >= 0 {
		return t.ents[i].v
	}
	return nil
}

// lookup is get(name) for a lookup chain: h carries the name's hash from table to table.
// A nil table (a scope that declares nothing) has no variables.
func (t *scopeVars_) lookup(name string, h *uint32) *AbstractVar {
	if t == nil {
		return nil
	}
	if i := t.find(name, h); i >= 0 {
		return t.ents[i].v
	}
	return nil
}

// get is vars.get(name): nil if the scope declares no such variable.
func (t *scopeVars_) get(name string) *AbstractVar {
	var h uint32
	return t.lookup(name, &h)
}

func (t *scopeVars_) containsKey(name string) bool {
	if t == nil {
		return false
	}
	var h uint32
	return t.find(name, &h) >= 0
}

func (t *scopeVars_) size() int32 {
	if t == nil {
		return 0
	}
	return int32(len(t.ents))
}

// put maps name to v; a name already there keeps its position (LinkedHashMap). The table
// keeps no ids from here on.
func (t *scopeVars_) put(name string, v *AbstractVar) {
	if !t.noIds {
		t.noIds = true
		if t.slots != nil {
			t.rebuild()
		}
	}
	t.putId(name, 0, v)
}

// putId is put for a name whose string id is `id`.
func (t *scopeVars_) putId(name string, id uint32, v *AbstractVar) {
	var h uint32
	if i := t.find(name, &h); i >= 0 {
		t.ents[i].v = v
		return
	}
	t.ents = append(t.ents, scopeVarEnt_{unsafe.StringData(name), uint32(len(name)), id, v})
	if t.slots != nil {
		if len(t.ents)*8 > t.nslots()*3 {
			t.rebuild()
		} else {
			if h == 0 {
				h = scopeVarHash_(name)
			}
			t.place(h, id, len(t.ents)-1)
		}
	} else if len(t.ents) > scopeVarsSmall_ {
		t.rebuild()
	}
}

// place puts position i, of a name with hash h and string id `id`, in a free slot of each half.
func (t *scopeVars_) place(h uint32, id uint32, i int) {
	n := t.nslots()
	mask := uint32(n - 1)
	slots := t.slots[:n]
	j := h & mask
	for slots[j] != 0 {
		j = (j + 1) & mask
	}
	slots[j] = uint64(h)<<32 | uint64(uint32(i+1))
	if t.noIds {
		return
	}
	ids := t.idSlots()
	j = scopeVarIdHash_(id) & mask
	for ids[j] != 0 {
		j = (j + 1) & mask
	}
	ids[j] = uint32(i + 1)
}

func (t *scopeVars_) rebuild() {
	if len(t.ents) <= scopeVarsSmall_ {
		t.slots = nil
		return
	}
	n := 32
	for n*3 < len(t.ents)*8 {
		n *= 2
	}
	if t.noIds {
		t.slots = make([]uint64, n)
	} else {
		t.slots = make([]uint64, n+n/2)
	}
	for i := range t.ents {
		t.place(scopeVarHash_(t.ents[i].name()), t.ents[i].id, i)
	}
}

// remove deletes the mapping of name, if any; the entries after it move up one place.
func (t *scopeVars_) remove(name string) {
	if t == nil {
		return
	}
	var h uint32
	if i := t.find(name, &h); i >= 0 {
		t.removeAt(i)
	}
}

func (t *scopeVars_) removeAt(i int) {
	copy(t.ents[i:], t.ents[i+1:])
	t.ents[len(t.ents)-1] = scopeVarEnt_{}
	t.ents = t.ents[:len(t.ents)-1]
	if t.slots != nil {
		t.rebuild()
	}
}

func (t *scopeVars_) clear() {
	clear(t.ents)
	t.ents = t.ents[:0]
	t.slots = nil
	t.noIds = false
}

// values is vars.values(): a live view of the variables in declaration order, made once per
// table, as LinkedHashMap's.
func (t *scopeVars_) values() Collection {
	if t.view == nil {
		t.view = &scopeVarsView_{t}
	}
	return t.view
}

// scopeVarsView_ is the values view of a scope's variables; its elements are the variables'
// self() (a *Var or *TypedVar), as an erased Map's values.
type scopeVarsView_ struct{ t *scopeVars_ }

func (v *scopeVarsView_) self() any        { return v }
func (v *scopeVarsView_) size() int32      { return int32(len(v.t.ents)) }
func (v *scopeVarsView_) isEmpty() bool    { return len(v.t.ents) == 0 }
func (v *scopeVarsView_) clear()           { v.t.clear() }
func (v *scopeVarsView_) add(x any) bool   { panic(new_UnsupportedOperationException_0().self()) }
func (v *scopeVarsView_) toString() string { return collToString_(v) }
func (v *scopeVarsView_) addAll(c Collection) bool {
	panic(new_UnsupportedOperationException_0().self())
}
func (v *scopeVarsView_) iterator() Iterator { return &scopeVarsIter_{t: v.t, last: -1} }
func (v *scopeVarsView_) toArray_0() []any {
	out := make([]any, len(v.t.ents))
	for i := range v.t.ents {
		out[i] = v.t.ents[i].v.self()
	}
	return out
}
func (v *scopeVarsView_) toArray(a []any) []any { return fillArray_(v.toArray_0(), a) }
func (v *scopeVarsView_) contains(x any) bool {
	for i := range v.t.ents {
		if objequals_(x, v.t.ents[i].v.self()) {
			return true
		}
	}
	return false
}
func (v *scopeVarsView_) remove_Object(x any) bool {
	for i := range v.t.ents {
		if objequals_(x, v.t.ents[i].v.self()) {
			v.t.removeAt(i)
			return true
		}
	}
	return false
}
func (v *scopeVarsView_) containsAll(c Collection) bool { return containsAll_(v, c) }
func (v *scopeVarsView_) removeAll(c Collection) bool   { return removeAll_(v, c) }

type scopeVarsIter_ struct {
	t    *scopeVars_
	i    int
	last int
}

func (it *scopeVarsIter_) hasNext() bool { return it.i < len(it.t.ents) }

func (it *scopeVarsIter_) next() any {
	if it.i >= len(it.t.ents) {
		panic(new_NoSuchElementException_0().self())
	}
	it.last = it.i
	it.i++
	return it.t.ents[it.last].v.self()
}

func (it *scopeVarsIter_) remove() {
	if it.last < 0 {
		panic(new_IllegalStateException_0().self())
	}
	it.t.removeAt(it.last)
	it.i = it.last
	it.last = -1
}
