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
//
// The tables are chunked like the records of handle classes and never shrink: an entry lives as
// long as the process. The translation runs one compiler thread, so they take no lock.

import (
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
