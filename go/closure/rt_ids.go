package closure

// Ids for the fields of handle classes that handles.txt stores as a uint32 instead of a Go
// string or interface (tools/j2go/README.md, "Handle classes"), so that the records holding them
// have no Go pointer: what has pointers is here, once per distinct string or value.
//
//	string-id     a String field holds an id into the process-wide table of interned strings:
//	              strid_ interns, strid_str_ reads. Id 0 is the null string, id 1 the empty one,
//	              so a zeroed record reads as null, as a Java field does.
//	string-id     an Object field (also a string slot) holds a value code, valcode_:
//	              0 null; bit 0 set: a String, id code>>1; low bits 010: an integral number
//	              inline, int32(code)>>3; low bits 110: a number, index code>>3 into the table of
//	              doubles; low bits 00 (not 0): any other object, index code>>2 into the table of
//	              objects. Numbers and other objects get a new table entry per store (Java's
//	              Double is a value here as elsewhere in the translation; another object keeps
//	              its identity).
//	handle-table  an Object field holds a handle into a table of distinct values, htab_: value
//	              code>>1, bit 0 a flag the generated setter computes (whether the value is an
//	              instance of the type handles.txt names). A value is found again by identity
//	              (its interface words), so storing the same object twice makes one entry.
//	              An entry counts the fields holding its handle and is freed when none does,
//	              so that the table keeps a value alive exactly as long as the fields would.
//
// The tables are chunked like the records of handle classes and never shrink. A string, a
// number and an object of a value code live as long as the process (strings and numbers hold no
// pointer to anything else; other objects are rare: BigIntegers, template literal strings); the
// entries of the handle table are counted and freed. The translation runs one compiler thread,
// so the tables take no lock.

import (
	"fmt"
	"math"
	"unsafe"
)

// idTable_ is an append-only table of values in chunks of handleChunkSize_, addressed like the
// records of handle classes: an entry is two loads and no bounds check.
type idTable_[T any] struct {
	n      uint32
	chunks [handleChunks_]*[handleChunkSize_]T
}

func (t *idTable_[T]) add(v T) uint32 {
	i := t.n
	if uint16(i) == 0 {
		if t.chunks[i>>handleShift_] != nil {
			panic(new_RuntimeException_String("id table: more than 2^32 entries").self())
		}
		t.chunks[i>>handleShift_] = new([handleChunkSize_]T)
	}
	t.chunks[i>>handleShift_][uint16(i)] = v
	t.n = i + 1
	return i
}

func (t *idTable_[T]) at(i uint32) T {
	return t.chunks[i>>handleShift_][uint16(i)]
}

func (t *idTable_[T]) ptr(i uint32) *T {
	return &t.chunks[i>>handleShift_][uint16(i)]
}

// reserve0 takes entry 0, for a table whose index 0 means none.
func (t *idTable_[T]) reserve0() bool {
	var zero T
	t.add(zero)
	return true
}

// ---------------------------------------------------------------- interned strings

var (
	strTab_ idTable_[string]
	strIds_ = strIdsInit_()
	// strIdCache_ remembers the ids of strings stored lately by the address and length of their
	// bytes: most strings a program stores are the same Go strings again (Java's interned ones),
	// which then need no hash of their contents. An entry holds the bytes alive, so an address
	// in it cannot be reused for other contents.
	strIdCache_ [4096]struct {
		p  *byte
		n  int
		id uint32
	}
)

func strIdsInit_() map[string]uint32 {
	strTab_.add(nullstr_)
	strTab_.add("")
	return map[string]uint32{nullstr_: 0, "": 1}
}

// strid_ is the id of string s, interning it on first use.
func strid_(s string) uint32 {
	if len(s) == 0 {
		return 1
	}
	p := unsafe.StringData(s)
	e := &strIdCache_[(uintptr(unsafe.Pointer(p))>>3^uintptr(len(s)))&uintptr(len(strIdCache_)-1)]
	if e.p == p && e.n == len(s) {
		return e.id
	}
	id, ok := strIds_[s]
	if !ok {
		id = strTab_.add(s)
		if id >= 1<<31 {
			panic(new_RuntimeException_String("string table: more than 2^31 strings").self())
		}
		strIds_[s] = id
	}
	e.p, e.n, e.id = p, len(s), id
	return id
}

// strid_str_ is the string of id.
func strid_str_(id uint32) string {
	return strTab_.at(id)
}

// ---------------------------------------------------------------- value codes

// Entry 0 of the table of objects is never used: code 0 is null.
var (
	valDoubles_  idTable_[float64]
	valObjects_  idTable_[any]
	valObjects0_ = valObjects_.reserve0()
)

func valcode_s_(s string) uint32 {
	return strid_(s)<<1 | 1
}

func valcode_d_(d float64) uint32 {
	if i := int32(d); float64(i) == d && i >= -1<<28 && i < 1<<28 && (i != 0 || !math.Signbit(d)) {
		return uint32(i)<<3 | 2
	}
	return valDoubles_.add(d)<<3 | 6
}

func valcode_(v any) uint32 {
	switch x := v.(type) {
	case nil:
		return 0
	case string:
		return valcode_s_(x)
	case float64:
		return valcode_d_(x)
	}
	return valObjects_.add(v) << 2
}

func valcode_isd_(c uint32) bool {
	return c&3 == 2
}

func valcode_dbl_(c uint32) float64 {
	if c&4 == 0 {
		return float64(int32(c) >> 3)
	}
	return valDoubles_.at(c >> 3)
}

// valcode_str_ is the String a code holds, or the null string.
func valcode_str_(c uint32) string {
	if c&1 == 0 {
		return nullstr_
	}
	return strid_str_(c >> 1)
}

// valcode_obj_ is the value a code holds unless it is a String (nil then, as for null).
func valcode_obj_(c uint32) any {
	switch {
	case c&1 != 0 || c == 0:
		return nil
	case c&3 == 2:
		return valcode_dbl_(c)
	}
	return valObjects_.at(c >> 2)
}

func valcode_get_(c uint32) any {
	if c&1 != 0 {
		return strid_str_(c >> 1)
	}
	return valcode_obj_(c)
}

// valcode_caststr_ is (String) of the value: its String, the null string for null, and the
// ClassCastException str_ throws for anything else.
func valcode_caststr_(c uint32) string {
	if c&1 != 0 {
		return strid_str_(c >> 1)
	}
	if c == 0 {
		return nullstr_
	}
	return str_(valcode_obj_(c))
}

// ---------------------------------------------------------------- handle tables

// htabEntry_ is a value of the table and the number of fields holding its handle (the largest
// number: for ever); inSlot: the entry is found by the value's own slot, not by the map.
type htabEntry_ struct {
	v      any
	refs   uint32
	inSlot bool
}

// htabHolder_ is an object that keeps its handle itself (post-flip htabself writes the field
// and this method into the classes whose objects the generated setters store): htabSlot_ is
// where. The slot is the index of the object's entry when that entry was made while the slot
// was 0; every other entry is in the map. A slot is believed only when the entry it names holds
// the same interface words as the value, so a value finds the entry it would find in the map
// alone, whatever its slot holds; handles, counts and the reuse of freed entries are as without
// slots.
type htabHolder_ interface{ htabSlot_() *uint32 }

const htabPinned_ = 1<<32 - 1

// Entry 0 is never used: code 0 is null. htabCache_ remembers the handles of values stored
// lately by their interface words, so that storing one of them again needs no map lookup.
var (
	htabTab_   idTable_[htabEntry_]
	htabTab0_  = htabTab_.reserve0()
	htabIds_   = map[eface_]uint32{}
	htabCache_ [1024]struct {
		k  eface_
		id uint32
	}
	htabFree_ []uint32 // entries no field holds any more, to use again
	// entries found by their value's slot, and entries in the map whose value has a slot (a
	// value other than the one its slot names: none in the generated code, which stores each
	// object by its one class)
	htabInSlots_, htabHoldersInMap_ int
)

func htabCacheAt_(k eface_) *struct {
	k  eface_
	id uint32
} {
	return &htabCache_[(uintptr(k.data)>>4^uintptr(k.typ)>>3)&uintptr(len(htabCache_)-1)]
}

// htabRef_ is the entry of v (not nil), new if v has none, counted once more.
func htabRef_(v any) uint32 {
	k := *(*eface_)(unsafe.Pointer(&v))
	if h, ok := v.(htabHolder_); ok {
		s := h.htabSlot_()
		if id := *s; id != 0 && id < htabTab_.n {
			if p := htabTab_.ptr(id); *(*eface_)(unsafe.Pointer(&p.v)) == k {
				if p.refs != htabPinned_ {
					p.refs++
				}
				return id
			}
		}
		return htabRefHolder_(v, k, s)
	}
	e := htabCacheAt_(k)
	id := e.id
	if e.k != k {
		var ok bool
		if id, ok = htabIds_[k]; !ok {
			id = htabNew_(v, false)
			htabIds_[k] = id
		}
		e.k, e.id = k, id
	}
	if p := htabTab_.ptr(id); p.refs != htabPinned_ {
		p.refs++
	}
	return id
}

// htabUnref_ counts the entry of handle c (not 0) once less and frees it at zero: the table
// keeps a value alive as long as a field holds it, as the field itself would.
func htabUnref_(c uint32) {
	p := htabTab_.ptr(c >> 1)
	if p.refs == htabPinned_ {
		return
	}
	if p.refs--; p.refs == 0 {
		if p.inSlot {
			if s := p.v.(htabHolder_).htabSlot_(); *s == c>>1 {
				*s = 0
			}
			htabInSlots_--
		} else {
			k := *(*eface_)(unsafe.Pointer(&p.v))
			if _, ok := p.v.(htabHolder_); ok {
				htabHoldersInMap_--
			} else if e := htabCacheAt_(k); e.k == k {
				e.k, e.id = eface_{}, 0
			}
			delete(htabIds_, k)
		}
		*p = htabEntry_{}
		htabFree_ = append(htabFree_, c>>1)
	}
}

// htabNew_ is a new entry of v, counted 0 times: a freed one again (the last freed first), or
// the next.
func htabNew_(v any, inSlot bool) uint32 {
	var id uint32
	if n := len(htabFree_); n > 0 {
		id = htabFree_[n-1]
		htabFree_ = htabFree_[:n-1]
		*htabTab_.ptr(id) = htabEntry_{v: v, inSlot: inSlot}
	} else {
		id = htabTab_.add(htabEntry_{v: v, inSlot: inSlot})
		if id >= 1<<31 {
			panic(new_RuntimeException_String("handle table: more than 2^31 values").self())
		}
	}
	return id
}

// htabRefHolder_ is htabRef_ of a value with slot s that its slot does not name: its entry in
// the map, or a new one, kept in s when s is free (in the map when it names another entry).
func htabRefHolder_(v any, k eface_, s *uint32) uint32 {
	if htabHoldersInMap_ > 0 {
		if id, ok := htabIds_[k]; ok {
			if p := htabTab_.ptr(id); p.refs != htabPinned_ {
				p.refs++
			}
			return id
		}
	}
	if *s == 0 {
		id := htabNew_(v, true)
		*s = id
		htabInSlots_++
		htabTab_.ptr(id).refs = 1
		return id
	}
	id := htabNew_(v, false)
	htabIds_[k] = id
	htabHoldersInMap_++
	htabTab_.ptr(id).refs = 1
	return id
}

// htab_set_ stores v in the field at loc: its handle (0 for nil) shifted left by one, with
// flag in bit 0.
func htab_set_(loc *uint32, v any, flag bool) {
	old := *loc
	if v == nil {
		*loc = 0
	} else {
		c := htabRef_(v) << 1
		if flag {
			c |= 1
		}
		*loc = c
	}
	if old != 0 {
		htabUnref_(old)
	}
}

// htab_copy_ stores in the field at loc what another field of the same kind holds, c.
func htab_copy_(loc *uint32, c uint32) {
	old := *loc
	if c != 0 {
		if p := htabTab_.ptr(c >> 1); p.refs != htabPinned_ {
			p.refs++
		}
	}
	*loc = c
	if old != 0 {
		htabUnref_(old)
	}
}

// htab_pin_ is the handle of v (0 for nil) shifted left by one, for ever: for the runtime's own
// fields, which copy and drop handles without counting (rt_node_props.go).
func htab_pin_(v any) uint32 {
	if v == nil {
		return 0
	}
	id := htabRef_(v)
	htabTab_.ptr(id).refs = htabPinned_
	return id << 1
}

func htab_get_(c uint32) any {
	if c == 0 {
		return nil
	}
	return htabTab_.ptr(c >> 1).v
}

// For JSGO_HANDLE_STATS.
func init() {
	handleStats_ = append(handleStats_, func() string {
		pinned := 0
		for i := uint32(1); i < htabTab_.n; i++ {
			if htabTab_.ptr(i).refs == htabPinned_ {
				pinned++
			}
		}
		return fmt.Sprintf("handle table: %d entries, %d in use (%d for ever)\n", htabTab_.n-1, len(htabIds_)+htabInSlots_, pinned)
	})
}
