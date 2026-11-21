package closure

// Protocol buffers runtime: the generic half of the protobuf-java API.
//
// Every generated message (rt_proto_gen.go, written by tools/protogen) and its builder are one
// allocation: the header pbData_ followed by a struct x_ with one typed Go field per proto field
// (the same struct type in the message and in its builder, at the same offset). The generated
// accessors read and write those fields directly; encoding, decoding, text format, equality and
// the builder operations that are not worth generating are implemented here once, driven by the
// descriptor tables, which give the offset of every field in the allocation.
//
// Field storage, by field kind:
//
//	int32, sint32, uint32, fixed32, sfixed32, enum   int32 (enum: its number)
//	int64, sint64, uint64, fixed64, sfixed64         int64
//	float, double, bool, string                      float32, float64, bool, string
//	bytes                                            *ByteString (nil reads as the empty one)
//	message                                          *Foo; in a builder the pointer may be to a
//	                                                 *Foo_Builder (see pbData_.isBuilder_)
//	repeated                                         a slice of the above
//
// Presence: a repeated field is set when it has elements, a message field when its pointer is
// not nil, a field with explicit presence (optional, oneof member, proto2) when its bit in the
// struct's pbHas_ words is set, and any other field when it is not zero. A field that is not set
// always holds its zero value.

import (
	"bytes"
	"math"
	"slices"
	"sort"
	"unicode/utf8"
	"unsafe"
)

// ---------------------------------------------------------------- descriptors

type pbKind_ uint8

// The numbers are those of descriptor.proto's FieldDescriptorProto.Type.
const (
	pbDouble_   pbKind_ = 1
	pbFloat_    pbKind_ = 2
	pbInt64_    pbKind_ = 3
	pbUint64_   pbKind_ = 4
	pbInt32_    pbKind_ = 5
	pbFixed64_  pbKind_ = 6
	pbFixed32_  pbKind_ = 7
	pbBool_     pbKind_ = 8
	pbString_   pbKind_ = 9
	pbGroup_    pbKind_ = 10
	pbMessage_  pbKind_ = 11
	pbBytes_    pbKind_ = 12
	pbUint32_   pbKind_ = 13
	pbEnum_     pbKind_ = 14
	pbSfixed32_ pbKind_ = 15
	pbSfixed64_ pbKind_ = 16
	pbSint32_   pbKind_ = 17
	pbSint64_   pbKind_ = 18
)

// pbWire_ is the wire type of each kind.
var pbWire_ = [...]int{0, 1, 5, 0, 0, 0, 1, 5, 0, 2, 3, 2, 2, 0, 0, 5, 1, 0, 0}

// pbCls_ is the Go type a kind is stored as.
type pbCls_ uint8

const (
	pbCI32_   pbCls_ = iota + 1 // int32
	pbCI64_                     // int64
	pbCBool_                    // bool
	pbCF32_                     // float32
	pbCF64_                     // float64
	pbCStr_                     // string
	pbCBytes_                   // *ByteString
	pbCMsg_                     // pointer to a message (or, in a builder, to a builder)
)

var pbClsOf_ = [...]pbCls_{0, pbCF64_, pbCF32_, pbCI64_, pbCI64_, pbCI32_, pbCI64_, pbCI32_, pbCBool_, pbCStr_, 0,
	pbCMsg_, pbCBytes_, pbCI32_, pbCI32_, pbCI32_, pbCI64_, pbCI32_, pbCI64_}

// pbElemSize_ is the size of one stored value of each class.
var pbElemSize_ = [...]uintptr{0, 4, 8, 1, 4, 8, unsafe.Sizeof(""), unsafe.Sizeof(uintptr(0)), unsafe.Sizeof(uintptr(0))}

type pbField_ struct {
	name   string  // proto field name
	num    int32   // field number
	kind   pbKind_ //
	rep    bool    // repeated
	packed bool    // repeated and written packed
	pres   bool    // has explicit presence (a hasFoo accessor)
	utf8   bool    // string that must be valid UTF-8 when parsed
	oneof  int32   // 1-based index of the containing oneof, 0 if none
	def    any     // explicit default value, boxed
	tname  string  // full name of the message or enum type
	off    uintptr // offset of the field in the struct x_ (after init_: in the allocation)
	hb     int32   // 1-based number of the presence bit in x_.pbHas_, 0 if none

	idx     int
	owner   *pbDesc_
	md      *pbDesc_
	ed      *pbEnumDesc_
	zero    any
	fd      *Descriptors_FieldDescriptor
	cls     pbCls_
	esz     uintptr // size of one stored value
	hOff    uintptr // offset in the allocation of the pbHas_ word of the presence bit
	hMask   uint32  // the presence bit in that word, 0 if none
	tagSize int     // size of the tag on the wire
}

type pbDesc_ struct {
	name   string // full proto name
	fields []*pbField_
	oneofs []string
	newM   func() *pbData_               // a new message (the header of the allocation)
	newB   func() *pbData_               // a new builder
	asM    func(p unsafe.Pointer) any    // the allocation p as the message type
	asB    func(p unsafe.Pointer) any    // the allocation p as the builder type
	cp     func(dst, src unsafe.Pointer) // copies the struct x_ between two allocations (messages or builders)
	xoff   uintptr                       // offset of x_ in the allocation
	hoff   uintptr                       // offset of pbHas_ in x_

	ready  bool
	sorted []*pbField_
	dense  []*pbField_
	byNum  map[int32]*pbField_
	byName map[string]*pbField_
	reps   []*pbField_   // the repeated fields
	msgs   []*pbField_   // the singular message fields
	oneofF [][]*pbField_ // the members of each oneof
	def    *pbData_
	gd     *Descriptors_Descriptor
}

type pbEnumDesc_ struct {
	name   string // full proto name
	closed bool   // unknown numbers are not representable (proto2)
	names  []string
	nums   []int32

	byNum  map[int32]int
	byName map[string]int32
	vds    map[int32]*Descriptors_EnumValueDescriptor
}

var pbDescs_ = map[string]*pbDesc_{}
var pbEnumDescs_ = map[string]*pbEnumDesc_{}

func pbReg_(d *pbDesc_) *pbDesc_ {
	pbDescs_[d.name] = d
	return d
}

func pbRegEnum_(e *pbEnumDesc_) *pbEnumDesc_ {
	e.byNum = make(map[int32]int, len(e.nums))
	e.byName = make(map[string]int32, len(e.nums))
	for i := len(e.nums) - 1; i >= 0; i-- {
		e.byNum[e.nums[i]] = i
		e.byName[e.names[i]] = e.nums[i]
	}
	pbEnumDescs_[e.name] = e
	return e
}

// init_ resolves the descriptor's cross references the first time it is used.
func (d *pbDesc_) init_() {
	if d.ready {
		return
	}
	d.ready = true
	d.byName = make(map[string]*pbField_, len(d.fields))
	d.oneofF = make([][]*pbField_, len(d.oneofs))
	max := int32(0)
	for i, f := range d.fields {
		f.idx = i
		f.owner = d
		d.byName[f.name] = f
		if f.num > max {
			max = f.num
		}
		f.cls = pbClsOf_[f.kind]
		if f.cls == 0 {
			panic("protobuf: unsupported field kind")
		}
		f.esz = pbElemSize_[f.cls]
		f.off += d.xoff
		f.tagSize = pbVarintSize_(uint64(f.num) << 3)
		if f.hb != 0 {
			f.hOff = d.xoff + d.hoff + uintptr((f.hb-1)>>5)*4
			f.hMask = 1 << uint((f.hb-1)&31)
		}
		if f.pres != (!f.rep && (f.hb != 0 || f.kind == pbMessage_)) || f.oneof != 0 && !f.pres {
			panic("protobuf: presence bit of " + d.name + "." + f.name)
		}
		if f.oneof != 0 {
			d.oneofF[f.oneof-1] = append(d.oneofF[f.oneof-1], f)
		}
		if f.rep {
			d.reps = append(d.reps, f)
		} else if f.kind == pbMessage_ {
			d.msgs = append(d.msgs, f)
		}
		switch f.kind {
		case pbMessage_:
			f.md = pbDescs_[f.tname]
			if f.md == nil {
				panic("protobuf: unknown message type " + f.tname)
			}
		case pbEnum_:
			f.ed = pbEnumDescs_[f.tname]
			if f.ed == nil {
				panic("protobuf: unknown enum type " + f.tname)
			}
		}
		f.zero = f.def
		if f.zero == nil {
			switch f.kind {
			case pbDouble_:
				f.zero = float64(0)
			case pbFloat_:
				f.zero = float32(0)
			case pbInt64_, pbUint64_, pbFixed64_, pbSfixed64_, pbSint64_:
				f.zero = int64(0)
			case pbInt32_, pbUint32_, pbFixed32_, pbSfixed32_, pbSint32_:
				f.zero = int32(0)
			case pbBool_:
				f.zero = false
			case pbString_:
				f.zero = ""
			case pbBytes_:
				f.zero = ByteString_EMPTY
			case pbEnum_:
				f.zero = f.ed.nums[0]
			}
		}
	}
	d.sorted = append([]*pbField_(nil), d.fields...)
	sort.SliceStable(d.sorted, func(i, j int) bool { return d.sorted[i].num < d.sorted[j].num })
	if max < 256 {
		d.dense = make([]*pbField_, max+1)
		for _, f := range d.fields {
			d.dense[f.num] = f
		}
	} else {
		d.byNum = make(map[int32]*pbField_, len(d.fields))
		for _, f := range d.fields {
			d.byNum[f.num] = f
		}
	}
}

func (d *pbDesc_) field_(num int32) *pbField_ {
	if d.dense != nil {
		if num > 0 && int(num) < len(d.dense) {
			return d.dense[num]
		}
		return nil
	}
	return d.byNum[num]
}

// pbM_ and pbB_ are implemented by every message and every builder.
type pbM_ interface {
	pbm_() *AbstractMessageLite
}

type pbB_ interface {
	pbb_() *AbstractMessageLite_Builder
}

func (d *pbDesc_) newMsg_() *pbData_ {
	d.init_()
	m := d.newM()
	m.d_ = d
	return m
}

func (d *pbDesc_) newBuilder_() *pbData_ {
	d.init_()
	b := d.newB()
	b.d_ = d
	b.size_ = pbBuilder_
	return b
}

func (d *pbDesc_) default_() *pbData_ {
	if d.def == nil {
		d.def = d.newMsg_()
	}
	return d.def
}

func (e *pbEnumDesc_) nameOf_(n int32) (string, bool) {
	i, ok := e.byNum[n]
	if !ok {
		return "", false
	}
	return e.names[i], true
}

// ---------------------------------------------------------------- field storage

// pbData_ is the header of every message and builder; the struct of the fields follows it in
// the same allocation, so a *pbData_ is also the address of the message or builder.
type pbData_ struct {
	d_   *pbDesc_
	unk_ *[]byte // unknown fields, as they appear on the wire; the slice is never changed, only replaced
	// size_ of a message: its serialized size + 1, 0 while unknown. Of a builder: pbBuilder_, or
	// pbShared_ when its repeated fields' slices are also those of a message (build() and
	// toBuilder() copy the struct of the fields, not the slices in it); the builder then copies
	// them before it changes a repeated field (own_).
	size_ int32
	hash_ int32
}

const (
	pbBuilder_ int32 = -1
	pbShared_  int32 = -2
)

// The message and builder base types are the header alone, so x_ is at one offset in both.
const _ = -uint(unsafe.Sizeof(GeneratedMessage_ExtendableMessage{}) - unsafe.Sizeof(pbData_{}))
const _ = -uint(unsafe.Sizeof(GeneratedMessage_ExtendableBuilder{}) - unsafe.Sizeof(pbData_{}))

// pbSlice_ is the layout of a slice.
type pbSlice_ struct {
	p    unsafe.Pointer
	n, c int
}

func (this *pbData_) isBuilder_() bool { return this.size_ < 0 }

// me_ is the concrete message or builder.
func (this *pbData_) me_() any { return pbAny_(unsafe.Pointer(this)) }

// pbAny_ is the message or builder at p as its concrete type.
func pbAny_(p unsafe.Pointer) any {
	d := (*pbData_)(p)
	if d.isBuilder_() {
		return d.d_.asB(p)
	}
	return d.d_.asM(p)
}

// fp_ is the address of the storage of field f.
func (this *pbData_) fp_(f *pbField_) unsafe.Pointer { return unsafe.Add(unsafe.Pointer(this), f.off) }

func (this *pbData_) hasWord_(f *pbField_) *uint32 {
	return (*uint32)(unsafe.Add(unsafe.Pointer(this), f.hOff))
}

func (this *pbData_) unkBytes_() []byte {
	if this.unk_ == nil {
		return nil
	}
	return *this.unk_
}

func (this *pbData_) setUnk_(b []byte) {
	if len(b) == 0 {
		this.unk_ = nil
		return
	}
	this.unk_ = &b
}

func (this *pbData_) addUnk_(b []byte) {
	u := this.unkBytes_()
	this.setUnk_(append(u[:len(u):len(u)], b...))
}

// own_ gives a builder its own repeated-field slices again after build() or toBuilder() shared them.
func (this *pbData_) own_() {
	if this.size_ == pbShared_ {
		this.ownSlow_()
	}
}

func (this *pbData_) ownSlow_() {
	for _, f := range this.d_.reps {
		this.repOp_(f, pbOpClone_, 0, nil)
	}
	this.size_ = pbBuilder_
}

// pbDataOf_ returns the storage of a message or builder.
func pbDataOf_(v any) *pbData_ {
	switch x := v.(type) {
	case pbM_:
		return &x.pbm_().pbData_
	case pbB_:
		return &x.pbb_().pbData_
	}
	if v == nil {
		panic(new_NullPointerException_0().self())
	}
	panic(new_IllegalArgumentException_String("not a protocol message: " + goTypeName_(v)).self())
}

// ptrOf_ is the message or builder v as a value of message field f.
func (f *pbField_) ptrOf_(v any) unsafe.Pointer {
	d := pbDataOf_(v)
	if d.d_ != f.md {
		panic(new_ClassCastException_String(goTypeName_(v) + " is not a " + f.md.name).self())
	}
	return unsafe.Pointer(d)
}

func pbNPE_() {
	panic(new_NullPointerException_0().self())
}

// pbBuilt_ is the message in a value that may be a builder.
func pbBuilt_(v any) any {
	if b, ok := v.(pbB_); ok {
		return b.pbb_().pbBuild_()
	}
	return v
}

// pbBuiltP_ is the message at p, which may be a builder.
func pbBuiltP_(p unsafe.Pointer) unsafe.Pointer {
	if d := (*pbData_)(p); d.isBuilder_() {
		return unsafe.Pointer(d.build_())
	}
	return p
}

// hasF_ says whether field f is set (a repeated field: has elements).
func (this *pbData_) hasF_(f *pbField_) bool {
	p := this.fp_(f)
	if f.rep {
		return (*pbSlice_)(p).n != 0
	}
	if f.hMask != 0 {
		return *this.hasWord_(f)&f.hMask != 0
	}
	switch f.cls {
	case pbCI32_, pbCF32_:
		return *(*uint32)(p) != 0
	case pbCI64_, pbCF64_:
		return *(*uint64)(p) != 0
	case pbCBool_:
		return *(*bool)(p)
	case pbCStr_:
		return *(*string)(p) != ""
	case pbCBytes_:
		b := *(**ByteString)(p)
		return b != nil && len(b.b_) != 0
	}
	return *(*unsafe.Pointer)(p) != nil
}

// pbBox_ is the stored value at p (a singular field or an element of a repeated one) boxed.
func pbBox_(f *pbField_, p unsafe.Pointer) any {
	switch f.cls {
	case pbCI32_:
		return *(*int32)(p)
	case pbCI64_:
		return *(*int64)(p)
	case pbCBool_:
		return *(*bool)(p)
	case pbCF32_:
		return *(*float32)(p)
	case pbCF64_:
		return *(*float64)(p)
	case pbCStr_:
		return *(*string)(p)
	case pbCBytes_:
		if b := *(**ByteString)(p); b != nil {
			return b
		}
		return ByteString_EMPTY
	}
	return pbAny_(*(*unsafe.Pointer)(p))
}

// slot_ is the boxed value of singular field f (a message field: the message or the builder
// stored), nil when the field is not set.
func (this *pbData_) slot_(f *pbField_) any {
	if !this.hasF_(f) {
		return nil
	}
	return pbBox_(f, this.fp_(f))
}

// elems_ is the boxed elements of repeated field f.
func (this *pbData_) elems_(f *pbField_) []any {
	s := (*pbSlice_)(this.fp_(f))
	if s.n == 0 {
		return nil
	}
	a := make([]any, s.n)
	for i := range a {
		a[i] = pbBox_(f, unsafe.Add(s.p, uintptr(i)*f.esz))
	}
	return a
}

// get_ is the boxed value of singular non-message field f, its default when not set.
func (this *pbData_) get_(i int) any {
	f := this.d_.fields[i]
	if v := this.slot_(f); v != nil {
		return v
	}
	return f.zero
}

// gMsgP_ is the value of a singular message field: never nil.
func (this *pbData_) gMsgP_(i int) unsafe.Pointer {
	f := this.d_.fields[i]
	if p := *(*unsafe.Pointer)(this.fp_(f)); p != nil {
		return pbBuiltP_(p)
	}
	return unsafe.Pointer(f.md.default_())
}

func (this *pbData_) gMsg_(i int) any { return pbAny_(this.gMsgP_(i)) }

func (this *pbData_) has_(i int) bool { return this.hasF_(this.d_.fields[i]) }

func (this *pbData_) cnt_(i int) int32 {
	return int32((*pbSlice_)(this.fp_(this.d_.fields[i])).n)
}

func pbIndexError_(idx int32, n int) {
	panic(new_IndexOutOfBoundsException_String("Index: " + istr_(int64(idx)) + ", Size: " + istr_(int64(n))).self())
}

// pbAt_ is getFoo(int) of a repeated field.
func pbAt_[T any](a []T, idx int32) T {
	if idx < 0 || int(idx) >= len(a) {
		pbIndexError_(idx, len(a))
	}
	return a[idx]
}

// pbListOf_ is getFooList(): the elements in their boxed Java representation.
func pbListOf_[T any](a []T) List {
	c := make([]any, len(a))
	for i, v := range a {
		c[i] = v
	}
	return listOf_(c)
}

// atMsgP_ is getFoo(int) of a repeated message field of a builder, where an element may be a builder.
func (this *pbData_) atMsgP_(i int, idx int32) unsafe.Pointer {
	a := *(*[]unsafe.Pointer)(this.fp_(this.d_.fields[i]))
	return pbBuiltP_(pbAt_(a, idx))
}

// at_ is the boxed element idx of a repeated field.
func (this *pbData_) at_(i int, idx int32) any {
	f := this.d_.fields[i]
	s := (*pbSlice_)(this.fp_(f))
	if idx < 0 || int(idx) >= s.n {
		pbIndexError_(idx, s.n)
	}
	return pbBuilt_(pbBox_(f, unsafe.Add(s.p, uintptr(idx)*f.esz)))
}

// list_ is getFooList() of a builder's repeated message field.
func (this *pbData_) list_(i int) List {
	a := this.elems_(this.d_.fields[i])
	for j, v := range a {
		a[j] = pbBuilt_(v)
	}
	return listOf_(a[:len(a):len(a)])
}

// enumList_ is getFooList() of a repeated enum field.
func (this *pbData_) enumList_(i int, conv func(int32) any) List {
	a := *(*[]int32)(this.fp_(this.d_.fields[i]))
	c := make([]any, len(a))
	for j, v := range a {
		c[j] = conv(v)
	}
	return listOf_(c)
}

// case_ is the field number of the member of a oneof that is set, or 0.
func (this *pbData_) case_(oneof int32) int32 {
	for _, f := range this.d_.oneofF[oneof-1] {
		if this.hasF_(f) {
			return f.num
		}
	}
	return 0
}

// clrF_ makes field f not set.
func (this *pbData_) clrF_(f *pbField_) {
	p := this.fp_(f)
	if f.rep {
		*(*pbSlice_)(p) = pbSlice_{}
		return
	}
	if f.hMask != 0 {
		*this.hasWord_(f) &^= f.hMask
	}
	switch f.cls {
	case pbCI32_, pbCF32_:
		*(*uint32)(p) = 0
	case pbCI64_, pbCF64_:
		*(*uint64)(p) = 0
	case pbCBool_:
		*(*bool)(p) = false
	case pbCStr_:
		*(*string)(p) = ""
	case pbCBytes_:
		*(**ByteString)(p) = nil
	default:
		*(*unsafe.Pointer)(p) = nil
	}
}

func (this *pbData_) clearOneof_(oneof int32, except *pbField_) {
	for _, f := range this.d_.oneofF[oneof-1] {
		if f != except {
			this.clrF_(f)
		}
	}
}

// mark_ records that singular field f is being set: the other members of its oneof are cleared
// and its presence bit is set.
func (this *pbData_) mark_(f *pbField_) {
	if f.oneof != 0 {
		this.clearOneof_(f.oneof, f)
	}
	if f.hMask != 0 {
		*this.hasWord_(f) |= f.hMask
	}
}

// The st*_ functions store one value of field f: the value of a singular field (with the
// presence rules of the field), one more element of a repeated one.

func (this *pbData_) stI32_(f *pbField_, v int32) {
	p := (*int32)(this.fp_(f))
	if f.rep {
		this.own_()
		s := (*[]int32)(unsafe.Pointer(p))
		*s = append(*s, v)
		return
	}
	this.mark_(f)
	*p = v
}

func (this *pbData_) stI64_(f *pbField_, v int64) {
	p := (*int64)(this.fp_(f))
	if f.rep {
		this.own_()
		s := (*[]int64)(unsafe.Pointer(p))
		*s = append(*s, v)
		return
	}
	this.mark_(f)
	*p = v
}

func (this *pbData_) stBool_(f *pbField_, v bool) {
	p := (*bool)(this.fp_(f))
	if f.rep {
		this.own_()
		s := (*[]bool)(unsafe.Pointer(p))
		*s = append(*s, v)
		return
	}
	this.mark_(f)
	*p = v
}

func (this *pbData_) stF32_(f *pbField_, v float32) {
	p := (*float32)(this.fp_(f))
	if f.rep {
		this.own_()
		s := (*[]float32)(unsafe.Pointer(p))
		*s = append(*s, v)
		return
	}
	this.mark_(f)
	*p = v
}

func (this *pbData_) stF64_(f *pbField_, v float64) {
	p := (*float64)(this.fp_(f))
	if f.rep {
		this.own_()
		s := (*[]float64)(unsafe.Pointer(p))
		*s = append(*s, v)
		return
	}
	this.mark_(f)
	*p = v
}

func (this *pbData_) stStr_(f *pbField_, v string) {
	p := (*string)(this.fp_(f))
	if f.rep {
		this.own_()
		s := (*[]string)(unsafe.Pointer(p))
		*s = append(*s, v)
		return
	}
	this.mark_(f)
	*p = v
}

func (this *pbData_) stBytes_(f *pbField_, v *ByteString) {
	p := (**ByteString)(this.fp_(f))
	if f.rep {
		this.own_()
		s := (*[]*ByteString)(unsafe.Pointer(p))
		*s = append(*s, v)
		return
	}
	this.mark_(f)
	if f.hMask == 0 && len(v.b_) == 0 {
		v = nil
	}
	*p = v
}

// stMsg_ stores a message (or, in a builder, a builder) of the field's type.
func (this *pbData_) stMsg_(f *pbField_, v unsafe.Pointer) {
	p := (*unsafe.Pointer)(this.fp_(f))
	if f.rep {
		this.own_()
		s := (*[]unsafe.Pointer)(unsafe.Pointer(p))
		*s = append(*s, v)
		return
	}
	this.mark_(f)
	*p = v
}

// put_ stores a boxed value as st*_ do.
func (this *pbData_) put_(f *pbField_, v any) {
	switch f.cls {
	case pbCI32_:
		this.stI32_(f, v.(int32))
	case pbCI64_:
		this.stI64_(f, v.(int64))
	case pbCBool_:
		this.stBool_(f, v.(bool))
	case pbCF32_:
		this.stF32_(f, v.(float32))
	case pbCF64_:
		this.stF64_(f, v.(float64))
	case pbCStr_:
		this.stStr_(f, v.(string))
	case pbCBytes_:
		this.stBytes_(f, v.(*ByteString))
	default:
		this.stMsg_(f, f.ptrOf_(v))
	}
}

// Operations on a repeated field's slice.
const (
	pbOpSet_    = iota // element idx = v
	pbOpInsert_        // v inserted at idx
	pbOpDelete_        // element idx removed
	pbOpClone_         // a slice of its own
)

func pbRepOp_[T any](s *[]T, op int, idx int, v T) {
	switch op {
	case pbOpSet_:
		(*s)[idx] = v
	case pbOpInsert_:
		*s = slices.Insert(*s, idx, v)
	case pbOpDelete_:
		*s = slices.Delete(*s, idx, idx+1)
	case pbOpClone_:
		if len(*s) == 0 {
			*s = nil
		} else {
			c := make([]T, len(*s))
			copy(c, *s)
			*s = c
		}
	}
}

func pbUnbox_[T any](v any) T {
	if v == nil {
		var zero T
		return zero
	}
	return v.(T)
}

// repOp_ applies an operation to the slice of repeated field f; v is the boxed element, if the
// operation takes one.
func (this *pbData_) repOp_(f *pbField_, op int, idx int, v any) {
	p := this.fp_(f)
	switch f.cls {
	case pbCI32_:
		pbRepOp_((*[]int32)(p), op, idx, pbUnbox_[int32](v))
	case pbCI64_:
		pbRepOp_((*[]int64)(p), op, idx, pbUnbox_[int64](v))
	case pbCBool_:
		pbRepOp_((*[]bool)(p), op, idx, pbUnbox_[bool](v))
	case pbCF32_:
		pbRepOp_((*[]float32)(p), op, idx, pbUnbox_[float32](v))
	case pbCF64_:
		pbRepOp_((*[]float64)(p), op, idx, pbUnbox_[float64](v))
	case pbCStr_:
		pbRepOp_((*[]string)(p), op, idx, pbUnbox_[string](v))
	case pbCBytes_:
		pbRepOp_((*[]*ByteString)(p), op, idx, pbUnbox_[*ByteString](v))
	default:
		var e unsafe.Pointer
		if v != nil {
			e = f.ptrOf_(v)
		}
		pbRepOp_((*[]unsafe.Pointer)(p), op, idx, e)
	}
}

// build_ is the message of a builder. It copies the struct of the fields; the repeated fields'
// slices stay shared with the builder (which copies them before its next change of a repeated
// field, own_), except where an element is a builder, which is built for the message.
func (this *pbData_) build_() *pbData_ {
	d := this.d_
	m := d.newM()
	m.d_ = d
	d.cp(unsafe.Pointer(m), unsafe.Pointer(this))
	m.unk_ = this.unk_
	if m.seal_() {
		this.size_ = pbShared_
	}
	return m
}

// buildLast_ is build_ for a builder that nothing uses afterwards (post-flip step pblast
// shows that of the calls it rewrites): the builder's allocation becomes the message, which a
// message and its builder allow by having one layout (pbDesc_.asM). The message is the one
// build_ would have copied out of it.
func (this *pbData_) buildLast_() *pbData_ {
	this.size_ = 0
	this.seal_()
	return this
}

// seal_ finishes the fields of a new message that were copied from a builder: the repeated
// fields' slices stay those of the builder, except where an element is a builder, which is
// built for the message; it says whether a slice is still the builder's.
func (m *pbData_) seal_() bool {
	d := m.d_
	shared := false
	for _, f := range d.reps {
		s := (*pbSlice_)(m.fp_(f))
		if s.n == 0 {
			if s.p != nil {
				*s = pbSlice_{}
			}
			continue
		}
		if f.cls == pbCMsg_ {
			ap := (*[]unsafe.Pointer)(unsafe.Pointer(s))
			a := *ap
			nested := false
			for _, e := range a {
				if (*pbData_)(e).isBuilder_() {
					nested = true
					break
				}
			}
			if nested {
				c := make([]unsafe.Pointer, len(a))
				for i, e := range a {
					c[i] = pbBuiltP_(e)
				}
				*ap = c
				continue
			}
		}
		shared = true
	}
	for _, f := range d.msgs {
		pp := (*unsafe.Pointer)(m.fp_(f))
		if e := *pp; e != nil && (*pbData_)(e).isBuilder_() {
			*pp = unsafe.Pointer((*pbData_)(e).build_())
		}
	}
	return shared
}

// toBuilder_ is a builder with the fields of a message.
func (this *pbData_) toBuilder_() *pbData_ {
	d := this.d_
	b := d.newBuilder_()
	d.cp(unsafe.Pointer(b), unsafe.Pointer(this))
	b.unk_ = this.unk_
	if len(d.reps) > 0 {
		b.size_ = pbShared_
	}
	return b
}

// clone_ is a message with the fields of this message in slices of its own, for merging into.
func (this *pbData_) clone_() *pbData_ {
	d := this.d_
	m := d.newMsg_()
	d.cp(unsafe.Pointer(m), unsafe.Pointer(this))
	m.unk_ = this.unk_
	for _, f := range d.reps {
		m.repOp_(f, pbOpClone_, 0, nil)
	}
	return m
}

// ---------------------------------------------------------------- messages

type AbstractMessageLite struct {
	pbData_
}

type AbstractMessage struct {
	AbstractMessageLite
}

type GeneratedMessage struct {
	AbstractMessage
}

type GeneratedMessage_ExtendableMessage struct {
	GeneratedMessage
}

func (this *AbstractMessageLite) pbm_() *AbstractMessageLite { return this }

func (this *AbstractMessageLite) toBuilder() MessageLite_Builder {
	return this.toBuilder_().me_().(MessageLite_Builder)
}

func (this *AbstractMessageLite) bytes_() []byte {
	return pbAppendMsg_(make([]byte, 0, pbSizeOf_(&this.pbData_)), &this.pbData_)
}

func (this *AbstractMessageLite) toByteString() *ByteString { return &ByteString{b_: this.bytes_()} }

func (this *AbstractMessageLite) toByteArray() []int8 { return pbI8_(this.bytes_()) }

func (this *AbstractMessageLite) getSerializedSize() int32 { return int32(pbSizeOf_(&this.pbData_)) }

func (this *AbstractMessageLite) writeTo(p0 *CodedOutputStream) { p0.put_(this.bytes_()) }

func (this *AbstractMessageLite) equals(o any) bool {
	m, ok := o.(pbM_)
	if !ok {
		return false
	}
	return pbEqual_(&this.pbData_, &m.pbm_().pbData_)
}

func (this *AbstractMessageLite) hashCode() int32 {
	if this.hash_ == 0 {
		h := pbHash_(&this.pbData_)
		if h == 0 {
			h = 1
		}
		this.hash_ = h
	}
	return this.hash_
}

func (this *AbstractMessageLite) toString() string { return pbText_(&this.pbData_, false, true) }

func (this *AbstractMessage) getAllFields() Map { return pbAllFields_(&this.pbData_) }

// ---------------------------------------------------------------- builders

type AbstractMessageLite_Builder struct {
	pbData_
}

type AbstractMessage_Builder struct {
	AbstractMessageLite_Builder
}

type GeneratedMessage_Builder struct {
	AbstractMessage_Builder
}

type GeneratedMessage_ExtendableBuilder struct {
	GeneratedMessage_Builder
}

func (this *AbstractMessageLite_Builder) pbb_() *AbstractMessageLite_Builder { return this }

func (this *AbstractMessageLite_Builder) pbBuild_() any {
	m := this.build_()
	return m.d_.asM(unsafe.Pointer(m))
}

func (this *AbstractMessageLite_Builder) build() MessageLite { return this.pbBuild_().(MessageLite) }

func (this *AbstractMessageLite_Builder) clear() MessageLite_Builder {
	d := this.d_
	d.cp(unsafe.Pointer(&this.pbData_), unsafe.Pointer(d.default_()))
	this.unk_ = nil
	this.size_ = pbBuilder_
	return this.me_().(MessageLite_Builder)
}

func (this *AbstractMessageLite_Builder) toString() string {
	return pbText_(&this.pbData_, false, true)
}

// ---- helpers the generated builder accessors call

func (this *AbstractMessageLite_Builder) checkValue_(v any) {
	if v == nil {
		pbNPE_()
	}
	if s, ok := v.(string); ok && s == nullstr_ {
		pbNPE_()
	}
}

func (this *AbstractMessageLite_Builder) set_(i int, v any) {
	this.checkValue_(v)
	this.put_(this.d_.fields[i], v)
}

// setMsg_ is setFoo(Foo) of a singular message field.
func (this *AbstractMessageLite_Builder) setMsg_(i int, m unsafe.Pointer) {
	if m == nil {
		pbNPE_()
	}
	this.stMsg_(this.d_.fields[i], m)
}

func (this *AbstractMessageLite_Builder) clr_(i int) { this.clrF_(this.d_.fields[i]) }

func (this *AbstractMessageLite_Builder) add_(i int, v any) {
	this.checkValue_(v)
	this.put_(this.d_.fields[i], v)
}

func (this *AbstractMessageLite_Builder) addAll_(i int, it Iterable, conv func(any) any) {
	if it == nil {
		pbNPE_()
	}
	f := this.d_.fields[i]
	for _, v := range toSlice_(it) {
		if conv != nil && v != nil {
			v = conv(v)
		}
		this.checkValue_(v)
		this.put_(f, v)
	}
}

// bld_ is getFooBuilder() for a singular message field.
func (this *AbstractMessageLite_Builder) bld_(i int) any {
	f := this.d_.fields[i]
	pp := (*unsafe.Pointer)(this.fp_(f))
	e := (*pbData_)(*pp)
	switch {
	case e == nil:
		e = f.md.newBuilder_()
		this.stMsg_(f, unsafe.Pointer(e))
	case !e.isBuilder_():
		e = e.toBuilder_()
		*pp = unsafe.Pointer(e)
	}
	return e.me_()
}

// addBld_ is addFooBuilder().
func (this *AbstractMessageLite_Builder) addBld_(i int) any {
	f := this.d_.fields[i]
	b := f.md.newBuilder_()
	this.stMsg_(f, unsafe.Pointer(b))
	return b.me_()
}

// ---------------------------------------------------------------- static entry points used by generated code

func pbParseCoded_(d *pbDesc_, in *CodedInputStream) any {
	m := d.newMsg_()
	in.mergeInto_(m)
	return m.me_()
}

// ---------------------------------------------------------------- wire format: sizes and writing

func pbVarintSize_(v uint64) int {
	n := 1
	for v >= 0x80 {
		v >>= 7
		n++
	}
	return n
}

func pbAppendVarint_(b []byte, v uint64) []byte {
	for v >= 0x80 {
		b = append(b, byte(v)|0x80)
		v >>= 7
	}
	return append(b, byte(v))
}

func pbAppendFixed32_(b []byte, v uint32) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24))
}

func pbAppendFixed64_(b []byte, v uint64) []byte {
	return append(b, byte(v), byte(v>>8), byte(v>>16), byte(v>>24), byte(v>>32), byte(v>>40), byte(v>>48), byte(v>>56))
}

func pbSizeOf_(m *pbData_) int {
	if m.size_ > 0 {
		return int(m.size_ - 1)
	}
	n := 0
	for _, f := range m.d_.sorted {
		p := m.fp_(f)
		if !f.rep {
			if m.hasF_(f) {
				n += f.tagSize + pbValueSize_(f, p)
			}
			continue
		}
		s := (*pbSlice_)(p)
		if s.n == 0 {
			continue
		}
		e := pbElemsSize_(f, s)
		if f.packed {
			n += f.tagSize + pbVarintSize_(uint64(e)) + e
		} else {
			n += e + f.tagSize*s.n
		}
	}
	n += len(m.unkBytes_())
	if m.size_ == 0 {
		m.size_ = int32(n + 1)
	}
	return n
}

// pbElemsSize_ is the size of the elements of a repeated field, without tags.
func pbElemsSize_(f *pbField_, s *pbSlice_) int {
	n := 0
	for i := 0; i < s.n; i++ {
		n += pbValueSize_(f, unsafe.Add(s.p, uintptr(i)*f.esz))
	}
	return n
}

func pbBytesAt_(p unsafe.Pointer) []byte {
	if b := *(**ByteString)(p); b != nil {
		return b.b_
	}
	return nil
}

// pbValueSize_ is the size on the wire of the stored value at p.
func pbValueSize_(f *pbField_, p unsafe.Pointer) int {
	switch f.kind {
	case pbInt32_, pbEnum_:
		return pbVarintSize_(uint64(int64(*(*int32)(p))))
	case pbUint32_:
		return pbVarintSize_(uint64(*(*uint32)(p)))
	case pbSint32_:
		x := *(*int32)(p)
		return pbVarintSize_(uint64(uint32(x<<1) ^ uint32(x>>31)))
	case pbInt64_, pbUint64_:
		return pbVarintSize_(*(*uint64)(p))
	case pbSint64_:
		x := *(*int64)(p)
		return pbVarintSize_(uint64(x<<1) ^ uint64(x>>63))
	case pbBool_:
		return 1
	case pbFixed32_, pbSfixed32_, pbFloat_:
		return 4
	case pbFixed64_, pbSfixed64_, pbDouble_:
		return 8
	case pbString_:
		n := len(*(*string)(p))
		return pbVarintSize_(uint64(n)) + n
	case pbBytes_:
		n := len(pbBytesAt_(p))
		return pbVarintSize_(uint64(n)) + n
	case pbMessage_:
		n := pbSizeOf_((*pbData_)(*(*unsafe.Pointer)(p)))
		return pbVarintSize_(uint64(n)) + n
	}
	panic("protobuf: unsupported field kind")
}

// pbGoNaN_ is the bit pattern of Go's math.NaN(), which differs from Java's.
var pbGoNaN_ = math.Float64bits(math.NaN())

func pbAppendMsg_(b []byte, m *pbData_) []byte {
	for _, f := range m.d_.sorted {
		p := m.fp_(f)
		tag := uint64(f.num)<<3 | uint64(pbWire_[f.kind])
		if !f.rep {
			if m.hasF_(f) {
				b = pbAppendVarint_(b, tag)
				b = pbAppendValue_(b, f, p)
			}
			continue
		}
		s := (*pbSlice_)(p)
		if s.n == 0 {
			continue
		}
		if f.packed {
			b = pbAppendVarint_(b, uint64(f.num)<<3|2)
			b = pbAppendVarint_(b, uint64(pbElemsSize_(f, s)))
			for i := 0; i < s.n; i++ {
				b = pbAppendValue_(b, f, unsafe.Add(s.p, uintptr(i)*f.esz))
			}
			continue
		}
		for i := 0; i < s.n; i++ {
			b = pbAppendVarint_(b, tag)
			b = pbAppendValue_(b, f, unsafe.Add(s.p, uintptr(i)*f.esz))
		}
	}
	return append(b, m.unkBytes_()...)
}

// pbAppendValue_ writes the stored value at p.
func pbAppendValue_(b []byte, f *pbField_, p unsafe.Pointer) []byte {
	switch f.kind {
	case pbInt32_, pbEnum_:
		return pbAppendVarint_(b, uint64(int64(*(*int32)(p))))
	case pbUint32_:
		return pbAppendVarint_(b, uint64(*(*uint32)(p)))
	case pbSint32_:
		x := *(*int32)(p)
		return pbAppendVarint_(b, uint64(uint32(x<<1)^uint32(x>>31)))
	case pbInt64_, pbUint64_:
		return pbAppendVarint_(b, *(*uint64)(p))
	case pbSint64_:
		x := *(*int64)(p)
		return pbAppendVarint_(b, uint64(x<<1)^uint64(x>>63))
	case pbBool_:
		if *(*bool)(p) {
			return append(b, 1)
		}
		return append(b, 0)
	case pbFixed32_, pbSfixed32_, pbFloat_:
		return pbAppendFixed32_(b, *(*uint32)(p))
	case pbFixed64_, pbSfixed64_:
		return pbAppendFixed64_(b, *(*uint64)(p))
	case pbDouble_:
		bits := *(*uint64)(p)
		if bits == pbGoNaN_ {
			bits = 0x7ff8000000000000 // Double.NaN
		}
		return pbAppendFixed64_(b, bits)
	case pbString_:
		s := *(*string)(p)
		b = pbAppendVarint_(b, uint64(len(s)))
		return append(b, s...)
	case pbBytes_:
		s := pbBytesAt_(p)
		b = pbAppendVarint_(b, uint64(len(s)))
		return append(b, s...)
	case pbMessage_:
		m := (*pbData_)(*(*unsafe.Pointer)(p))
		b = pbAppendVarint_(b, uint64(pbSizeOf_(m)))
		return pbAppendMsg_(b, m)
	}
	panic("protobuf: unsupported field kind")
}

// ---------------------------------------------------------------- wire format: parsing

const (
	pbErrTruncated_ = "While parsing a protocol message, the input ended unexpectedly in the middle of a field.  This could mean either that the input has been truncated or that an embedded message misreported its own length."
	pbErrVarint_    = "CodedInputStream encountered a malformed varint."
	pbErrTag_       = "Protocol message contained an invalid tag (zero)."
	pbErrEndGroup_  = "Protocol message end-group tag did not match expected tag."
	pbErrWireType_  = "Protocol message tag had invalid wire type."
	pbErrRecursion_ = "Protocol message had too many levels of nesting.  May be malicious.  Use setRecursionLimit() to increase the recursion depth limit."
	pbErrUtf8_      = "Protocol message had invalid UTF-8."
	pbErrNegative_  = "CodedInputStream encountered an embedded string or message which claimed to have negative size."
)

func pbFail_(msg string) {
	panic(new_InvalidProtocolBufferException_String(msg).self())
}

func pbVarint_(data []byte, pos int) (uint64, int) {
	if pos < len(data) && data[pos] < 0x80 {
		return uint64(data[pos]), pos + 1
	}
	var v uint64
	for shift := uint(0); shift < 64; shift += 7 {
		if pos >= len(data) {
			pbFail_(pbErrTruncated_)
		}
		c := data[pos]
		pos++
		v |= uint64(c&0x7f) << shift
		if c < 0x80 {
			return v, pos
		}
	}
	pbFail_(pbErrVarint_)
	return 0, 0
}

// pbLen_ reads a length prefix and checks that the bytes are there.
func pbLen_(data []byte, pos int) (int, int) {
	v, pos := pbVarint_(data, pos)
	n := int(int32(v))
	if n < 0 {
		pbFail_(pbErrNegative_)
	}
	if n > len(data)-pos {
		pbFail_(pbErrTruncated_)
	}
	return n, pos
}

// pbSkip_ returns the position after the value of a field with wire type wt.
func pbSkip_(data []byte, pos int, wt int, num int32) int {
	switch wt {
	case 0:
		_, pos = pbVarint_(data, pos)
	case 1:
		pos += 8
	case 5:
		pos += 4
	case 2:
		var n int
		n, pos = pbLen_(data, pos)
		pos += n
	case 3:
		for {
			var tag uint64
			tag, pos = pbVarint_(data, pos)
			if tag>>3 == 0 {
				pbFail_(pbErrTag_)
			}
			if tag&7 == 4 {
				if int32(tag>>3) != num {
					pbFail_(pbErrEndGroup_)
				}
				break
			}
			pos = pbSkip_(data, pos, int(tag&7), int32(tag>>3))
		}
	default:
		pbFail_(pbErrWireType_)
	}
	if pos > len(data) {
		pbFail_(pbErrTruncated_)
	}
	return pos
}

type pbParser_ struct {
	limit int32 // recursion limit
	alias bool  // bytes fields may share the input buffer
}

func (p *pbParser_) merge_(d *pbData_, data []byte, depth int32) {
	desc := d.d_
	unknown := false
	for pos := 0; pos < len(data); {
		start := pos
		var tag uint64
		tag, pos = pbVarint_(data, pos)
		num := int32(uint32(tag) >> 3)
		wt := int(tag & 7)
		if num == 0 {
			pbFail_(pbErrTag_)
		}
		if f := desc.field_(num); f != nil {
			ewt := pbWire_[f.kind]
			if wt == ewt {
				pos = p.readField_(d, f, data, pos, depth)
				continue
			}
			if wt == 2 && f.rep && ewt != 2 {
				var n int
				n, pos = pbLen_(data, pos)
				end := pos + n
				sub := data[:end]
				for pos < end {
					pos = p.readField_(d, f, sub, pos, depth)
				}
				continue
			}
		}
		if wt == 4 {
			break
		}
		pos = pbSkip_(data, pos, wt, num)
		d.addUnk_(data[start:pos])
		unknown = true
	}
	if unknown {
		d.setUnk_(pbSortUnknown_(d.unkBytes_()))
	}
}

// readField_ reads one value of field f (whose wire type has been checked).
func (p *pbParser_) readField_(d *pbData_, f *pbField_, data []byte, pos int, depth int32) int {
	switch f.kind {
	case pbInt32_, pbUint32_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		d.stI32_(f, int32(x))
	case pbEnum_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		if f.ed.closed {
			if _, ok := f.ed.byNum[int32(x)]; !ok {
				d.addUnk_(pbAppendVarint_(pbAppendVarint_(nil, uint64(f.num)<<3), uint64(int64(int32(x)))))
				d.setUnk_(pbSortUnknown_(d.unkBytes_()))
				return pos
			}
		}
		d.stI32_(f, int32(x))
	case pbSint32_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		u := uint32(x)
		d.stI32_(f, int32(u>>1)^-int32(u&1))
	case pbInt64_, pbUint64_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		d.stI64_(f, int64(x))
	case pbSint64_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		d.stI64_(f, int64(x>>1)^-int64(x&1))
	case pbBool_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		d.stBool_(f, x != 0)
	case pbFixed32_, pbSfixed32_, pbFloat_:
		if pos+4 > len(data) {
			pbFail_(pbErrTruncated_)
		}
		x := uint32(data[pos]) | uint32(data[pos+1])<<8 | uint32(data[pos+2])<<16 | uint32(data[pos+3])<<24
		pos += 4
		if f.kind == pbFloat_ {
			d.stF32_(f, math.Float32frombits(x))
		} else {
			d.stI32_(f, int32(x))
		}
	case pbFixed64_, pbSfixed64_, pbDouble_:
		if pos+8 > len(data) {
			pbFail_(pbErrTruncated_)
		}
		x := uint64(data[pos]) | uint64(data[pos+1])<<8 | uint64(data[pos+2])<<16 | uint64(data[pos+3])<<24 |
			uint64(data[pos+4])<<32 | uint64(data[pos+5])<<40 | uint64(data[pos+6])<<48 | uint64(data[pos+7])<<56
		pos += 8
		if f.kind == pbDouble_ {
			d.stF64_(f, math.Float64frombits(x))
		} else {
			d.stI64_(f, int64(x))
		}
	case pbString_:
		var n int
		n, pos = pbLen_(data, pos)
		s := string(data[pos : pos+n])
		pos += n
		if f.utf8 && !utf8.ValidString(s) {
			pbFail_(pbErrUtf8_)
		}
		d.stStr_(f, s)
	case pbBytes_:
		var n int
		n, pos = pbLen_(data, pos)
		b := data[pos : pos+n : pos+n]
		pos += n
		if n == 0 {
			d.stBytes_(f, ByteString_EMPTY)
		} else if p.alias {
			d.stBytes_(f, &ByteString{b_: b})
		} else {
			d.stBytes_(f, &ByteString{b_: append([]byte(nil), b...)})
		}
	case pbMessage_:
		var n int
		n, pos = pbLen_(data, pos)
		sub := data[pos : pos+n : pos+n]
		pos += n
		if depth >= p.limit {
			pbFail_(pbErrRecursion_)
		}
		if !f.rep {
			pp := (*unsafe.Pointer)(d.fp_(f))
			if x := (*pbData_)(*pp); x != nil {
				if !x.isBuilder_() {
					x = x.clone_()
					*pp = unsafe.Pointer(x)
				}
				p.merge_(x, sub, depth+1)
				return pos
			}
		}
		m := f.md.newMsg_()
		p.merge_(m, sub, depth+1)
		d.stMsg_(f, unsafe.Pointer(m))
	default:
		panic("protobuf: unsupported field kind")
	}
	return pos
}

// pbSortUnknown_ puts unknown fields in field-number order, as
// UnknownFieldSet writes them.
func pbSortUnknown_(raw []byte) []byte {
	type ent struct {
		key        int64 // field number, then the order UnknownFieldSet writes wire types in
		start, end int
	}
	rank := [...]int64{0, 2, 3, 4, 0, 1, 0, 0}
	var ents []ent
	ordered := true
	last := int64(0)
	for pos := 0; pos < len(raw); {
		start := pos
		var tag uint64
		tag, pos = pbVarint_(raw, pos)
		num := int32(tag >> 3)
		pos = pbSkip_(raw, pos, int(tag&7), num)
		key := int64(num)<<3 | rank[tag&7]
		if key < last {
			ordered = false
		}
		last = key
		ents = append(ents, ent{key, start, pos})
	}
	if ordered {
		return raw
	}
	sort.SliceStable(ents, func(i, j int) bool { return ents[i].key < ents[j].key })
	out := make([]byte, 0, len(raw))
	for _, e := range ents {
		out = append(out, raw[e.start:e.end]...)
	}
	return out
}

// ---------------------------------------------------------------- equality

func pbEqual_(a, b *pbData_) bool {
	if a == b {
		return true
	}
	if a.d_ != b.d_ || !bytes.Equal(a.unkBytes_(), b.unkBytes_()) {
		return false
	}
	for _, f := range a.d_.fields {
		p, q := a.fp_(f), b.fp_(f)
		if !f.rep {
			if a.hasF_(f) != b.hasF_(f) || !pbValueEqual_(f, p, q) {
				return false
			}
			continue
		}
		s, t := (*pbSlice_)(p), (*pbSlice_)(q)
		if s.n != t.n {
			return false
		}
		for i := 0; i < s.n; i++ {
			if !pbValueEqual_(f, unsafe.Add(s.p, uintptr(i)*f.esz), unsafe.Add(t.p, uintptr(i)*f.esz)) {
				return false
			}
		}
	}
	return true
}

// pbValueEqual_ compares the stored values at p and q.
func pbValueEqual_(f *pbField_, p, q unsafe.Pointer) bool {
	switch f.cls {
	case pbCI32_:
		return *(*int32)(p) == *(*int32)(q)
	case pbCI64_:
		return *(*int64)(p) == *(*int64)(q)
	case pbCBool_:
		return *(*bool)(p) == *(*bool)(q)
	case pbCF32_:
		x, y := *(*float32)(p), *(*float32)(q)
		return math.Float32bits(x) == math.Float32bits(y) || x != x && y != y
	case pbCF64_:
		x, y := *(*float64)(p), *(*float64)(q)
		return math.Float64bits(x) == math.Float64bits(y) || x != x && y != y
	case pbCStr_:
		return *(*string)(p) == *(*string)(q)
	case pbCBytes_:
		return bytes.Equal(pbBytesAt_(p), pbBytesAt_(q))
	}
	x, y := (*pbData_)(*(*unsafe.Pointer)(p)), (*pbData_)(*(*unsafe.Pointer)(q))
	if x == nil || y == nil {
		return x == y
	}
	return pbEqual_(x, y)
}

func pbHash_(d *pbData_) int32 {
	h := int32(41)
	h = 19*h + String_hashCode(d.d_.name)
	for _, f := range d.d_.sorted {
		if !d.hasF_(f) {
			continue
		}
		h = 37*h + f.num
		if f.rep {
			e := int32(1)
			for _, v := range d.elems_(f) {
				e = 31*e + pbValueHash_(v)
			}
			h = 53*h + e
		} else {
			h = 53*h + pbValueHash_(d.slot_(f))
		}
	}
	for _, c := range d.unkBytes_() {
		h = 29*h + int32(c)
	}
	return h
}

func pbValueHash_(v any) int32 {
	switch x := v.(type) {
	case float32:
		if x != x {
			return 0x7fc00000
		}
		return int32(math.Float32bits(x))
	case float64:
		if x != x {
			return 0x7ff80000
		}
	case pbM_, pbB_:
		return pbHash_(pbDataOf_(v))
	}
	return Object_hashCode(v)
}

// ---------------------------------------------------------------- reflection (the little that is used)

type Descriptors_GenericDescriptor struct {
	name_     string
	fullName_ string
}

func (this *Descriptors_GenericDescriptor) getName() string  { return this.name_ }
func (this *Descriptors_GenericDescriptor) toString() string { return this.fullName_ }

type Descriptors_FieldDescriptor struct {
	Descriptors_GenericDescriptor
	f_ *pbField_
}

func (this *Descriptors_FieldDescriptor) getNumber() int32 { return this.f_.num }
func (this *Descriptors_FieldDescriptor) compareTo(p0 any) int32 {
	o := p0.(*Descriptors_FieldDescriptor)
	if o.f_.owner != this.f_.owner {
		panic(new_IllegalArgumentException_String("FieldDescriptors can only be compared to other FieldDescriptors for fields of the same message type.").self())
	}
	return this.f_.num - o.f_.num
}

type Descriptors_Descriptor struct {
	Descriptors_GenericDescriptor
	d_ *pbDesc_
}

type Descriptors_EnumValueDescriptor struct {
	Descriptors_GenericDescriptor
	num_   int32
	index_ int32
}

func (this *Descriptors_EnumValueDescriptor) getNumber() int32 { return this.num_ }
func (this *Descriptors_EnumValueDescriptor) toString() string { return this.name_ }

func pbSimpleName_(full string) string {
	for i := len(full) - 1; i >= 0; i-- {
		if full[i] == '.' {
			return full[i+1:]
		}
	}
	return full
}

func (f *pbField_) descriptor_() *Descriptors_FieldDescriptor {
	if f.fd == nil {
		f.fd = &Descriptors_FieldDescriptor{f_: f}
		f.fd.name_ = f.name
		f.fd.fullName_ = f.owner.name + "." + f.name
	}
	return f.fd
}

func (e *pbEnumDesc_) value_(n int32) *Descriptors_EnumValueDescriptor {
	if vd := e.vds[n]; vd != nil {
		return vd
	}
	vd := &Descriptors_EnumValueDescriptor{num_: n, index_: -1}
	if i, ok := e.byNum[n]; ok {
		vd.name_ = e.names[i]
		vd.index_ = int32(i)
	} else {
		vd.name_ = "UNKNOWN_ENUM_VALUE_" + pbSimpleName_(e.name) + "_" + istr_(int64(n))
	}
	vd.fullName_ = e.name[:len(e.name)-len(pbSimpleName_(e.name))] + vd.name_
	if e.vds == nil {
		e.vds = map[int32]*Descriptors_EnumValueDescriptor{}
	}
	e.vds[n] = vd
	return vd
}

// pbReflect_ is a slot value as the reflection API shows it.
func pbReflect_(f *pbField_, v any) any {
	if f.kind == pbEnum_ {
		return f.ed.value_(v.(int32))
	}
	return pbBuilt_(v)
}

func pbGetField_(d *pbData_, fd *Descriptors_FieldDescriptor) any {
	f := fd.f_
	if f.owner != d.d_ {
		panic(new_IllegalArgumentException_String("FieldDescriptor does not match message type.").self())
	}
	if f.rep {
		out := d.elems_(f)
		for i, e := range out {
			out[i] = pbReflect_(f, e)
		}
		return listOf_(out[:len(out):len(out)])
	}
	if f.kind == pbMessage_ {
		return d.gMsg_(f.idx)
	}
	return pbReflect_(f, d.get_(f.idx))
}

// pbAllFields_ is getAllFields(): the set fields in field-number order.
func pbAllFields_(d *pbData_) Map {
	m := new_TreeMap()
	for _, f := range d.d_.sorted {
		if !d.hasF_(f) {
			continue
		}
		m.put(f.descriptor_(), pbGetField_(d, f.descriptor_()))
	}
	return m
}

// ---------------------------------------------------------------- enums

// pbEnumNumber_ is getNumber() of an enum constant; UNRECOGNIZED has none.
func pbEnumNumber_(num int32, unrecognized bool) int32 {
	if unrecognized {
		panic(new_IllegalArgumentException_String("Can't get the number of an unknown enum value.").self())
	}
	return num
}

// ---------------------------------------------------------------- byte slices and streams

func pbI8_(b []byte) []int8 {
	if len(b) == 0 {
		return []int8{}
	}
	return unsafe.Slice((*int8)(unsafe.Pointer(&b[0])), len(b))
}

func pbU8_(b []int8) []byte {
	if len(b) == 0 {
		return nil
	}
	return unsafe.Slice((*byte)(unsafe.Pointer(&b[0])), len(b))
}

// ---------------------------------------------------------------- ByteString

type ByteString struct {
	b_    []byte
	hash_ int32
}

var ByteString_EMPTY = &ByteString{}

func ByteString_copyFrom(p0 []int8) *ByteString {
	if len(p0) == 0 {
		if p0 == nil {
			pbNPE_()
		}
		return ByteString_EMPTY
	}
	return &ByteString{b_: append([]byte(nil), pbU8_(p0)...)}
}

func ByteString_newOutput(p0 int32) *ByteString_Output {
	o := &ByteString_Output{}
	o.self_ = o
	if p0 > 0 {
		o.buf_ = make([]byte, 0, p0)
	}
	return o
}

func (this *ByteString) size() int32   { return int32(len(this.b_)) }
func (this *ByteString) isEmpty() bool { return len(this.b_) == 0 }

func (this *ByteString) byteAt(p0 int32) int8 {
	if p0 < 0 || int(p0) >= len(this.b_) {
		panic(new_ArrayIndexOutOfBoundsException_String("Index: " + istr_(int64(p0)) + ", Length: " + istr_(int64(len(this.b_)))).self())
	}
	return int8(this.b_[p0])
}

func (this *ByteString) iterator() Iterator {
	a := make([]any, len(this.b_))
	for i, c := range this.b_ {
		a[i] = int8(c)
	}
	return listOf_(a).iterator()
}

func (this *ByteString) newCodedInput() *CodedInputStream {
	return &CodedInputStream{buf_: this.b_, limit_: math.MaxInt, recLimit_: 100, alias_: true}
}

func (this *ByteString) equals(o any) bool {
	b, ok := o.(*ByteString)
	return ok && b != nil && bytes.Equal(this.b_, b.b_)
}

func (this *ByteString) hashCode() int32 {
	if this.hash_ == 0 {
		h := int32(len(this.b_))
		for _, c := range this.b_ {
			h = h*31 + int32(int8(c))
		}
		if h == 0 {
			h = 1
		}
		this.hash_ = h
	}
	return this.hash_
}

func (this *ByteString) toString() string {
	contents := this.b_
	suffix := ""
	if len(contents) > 50 {
		contents = contents[:47]
		suffix = "..."
	}
	return "<ByteString@" + Integer_toHexString_(identityHash_(this)) + " size=" + istr_(int64(len(this.b_))) +
		" contents=\"" + pbEscapeBytes_(contents) + suffix + "\">"
}

func Integer_toHexString_(v int32) string {
	const digits = "0123456789abcdef"
	u := uint32(v)
	if u == 0 {
		return "0"
	}
	var buf [8]byte
	i := len(buf)
	for u != 0 {
		i--
		buf[i] = digits[u&15]
		u >>= 4
	}
	return string(buf[i:])
}

type ByteString_Output struct {
	OutputStream
	buf_ []byte
}

func (this *ByteString_Output) write(p0 int32) { this.buf_ = append(this.buf_, byte(p0)) }

func (this *ByteString_Output) flush() {}
func (this *ByteString_Output) close() {}

func (this *ByteString_Output) size() int32 { return int32(len(this.buf_)) }

func (this *ByteString_Output) reset() { this.buf_ = nil }

func (this *ByteString_Output) toByteString() *ByteString {
	if len(this.buf_) == 0 {
		return ByteString_EMPTY
	}
	return &ByteString{b_: append([]byte(nil), this.buf_...)}
}

// ---------------------------------------------------------------- WireFormat, ExtensionRegistry

type WireFormat struct {
}

const (
	WireFormat_WIRETYPE_VARINT           int32 = 0
	WireFormat_WIRETYPE_FIXED64          int32 = 1
	WireFormat_WIRETYPE_LENGTH_DELIMITED int32 = 2
	WireFormat_WIRETYPE_START_GROUP      int32 = 3
	WireFormat_WIRETYPE_END_GROUP        int32 = 4
	WireFormat_WIRETYPE_FIXED32          int32 = 5
)

func WireFormat_getTagWireType(p0 int32) int32    { return p0 & 7 }
func WireFormat_getTagFieldNumber(p0 int32) int32 { return int32(uint32(p0) >> 3) }

type ExtensionRegistryLite struct {
}

type ExtensionRegistry struct {
	ExtensionRegistryLite
}

func (this *ExtensionRegistry) up_ExtensionRegistryLite() *ExtensionRegistryLite {
	if this == nil {
		return nil
	}
	return &this.ExtensionRegistryLite
}

var pbEmptyRegistry_ = &ExtensionRegistry{}

func ExtensionRegistry_getEmptyRegistry() *ExtensionRegistry { return pbEmptyRegistry_ }

// ---------------------------------------------------------------- CodedInputStream

// CodedInputStream reads from memory.
type CodedInputStream struct {
	buf_      []byte
	pos_      int
	base_     int // bytes consumed before buf_[0], since the last resetSizeCounter
	limit_    int // absolute position (base_+pos_) the reader may not pass
	lastTag_  int32
	recLimit_ int32
	alias_    bool
}

func CodedInputStream_newInstance(p0 []int8) *CodedInputStream {
	return &CodedInputStream{buf_: pbU8_(p0), limit_: math.MaxInt, recLimit_: 100, alias_: true}
}

// fill_ says whether n bytes are available at pos_.
func (this *CodedInputStream) fill_(n int) bool { return len(this.buf_)-this.pos_ >= n }

func (this *CodedInputStream) getTotalBytesRead() int32 { return int32(this.total_()) }

// pushLimit makes the reader stop p0 bytes from here and returns the limit it replaces.
func (this *CodedInputStream) pushLimit(p0 int32) int32 {
	if p0 < 0 {
		pbFail_(pbErrNegative_)
	}
	old := this.limit_
	if this.total_()+int(p0) > old {
		pbFail_(pbErrTruncated_)
	}
	this.limit_ = this.total_() + int(p0)
	return int32(min(old, math.MaxInt32))
}

func (this *CodedInputStream) popLimit(p0 int32) {
	this.limit_ = int(p0)
	if p0 == math.MaxInt32 {
		this.limit_ = math.MaxInt
	}
}

func (this *CodedInputStream) total_() int { return this.base_ + this.pos_ }

func (this *CodedInputStream) isAtEnd() bool {
	return this.total_() >= this.limit_ || !this.fill_(1)
}

func (this *CodedInputStream) resetSizeCounter() {
	if this.limit_ != math.MaxInt {
		this.limit_ -= this.total_()
	}
	this.base_ = -this.pos_
}

func (this *CodedInputStream) setRecursionLimit(p0 int32) int32 {
	if p0 < 0 {
		panic(new_IllegalArgumentException_String("Recursion limit cannot be negative: " + istr_(int64(p0))).self())
	}
	old := this.recLimit_
	this.recLimit_ = p0
	return old
}

func (this *CodedInputStream) readByte_() byte {
	if this.total_() >= this.limit_ || !this.fill_(1) {
		pbFail_(pbErrTruncated_)
	}
	c := this.buf_[this.pos_]
	this.pos_++
	return c
}

// raw_ returns the next n bytes; the slice is not modified afterwards.
func (this *CodedInputStream) raw_(n int) []byte {
	if n < 0 {
		pbFail_(pbErrNegative_)
	}
	if n > this.limit_-this.total_() || !this.fill_(n) {
		pbFail_(pbErrTruncated_)
	}
	b := this.buf_[this.pos_ : this.pos_+n : this.pos_+n]
	this.pos_ += n
	return b
}

// rest_ returns everything up to the limit or the end of the input.
func (this *CodedInputStream) rest_() []byte {
	if this.limit_ != math.MaxInt {
		return this.raw_(this.limit_ - this.total_())
	}
	b := this.buf_[this.pos_:len(this.buf_):len(this.buf_)]
	this.pos_ = len(this.buf_)
	return b
}

func (this *CodedInputStream) readRawVarint64() int64 {
	var v uint64
	for shift := uint(0); shift < 64; shift += 7 {
		c := this.readByte_()
		v |= uint64(c&0x7f) << shift
		if c < 0x80 {
			return int64(v)
		}
	}
	pbFail_(pbErrVarint_)
	return 0
}

func (this *CodedInputStream) readRawVarint32() int32 { return int32(this.readRawVarint64()) }

func (this *CodedInputStream) readTag() int32 {
	if this.isAtEnd() {
		this.lastTag_ = 0
		return 0
	}
	this.lastTag_ = this.readRawVarint32()
	if uint32(this.lastTag_)>>3 == 0 {
		pbFail_(pbErrTag_)
	}
	return this.lastTag_
}

// readMessage reads a length-prefixed message and merges it into the builder.
func (this *CodedInputStream) readMessage(p0 MessageLite_Builder, p1 *ExtensionRegistryLite) {
	data := this.raw_(int(this.readRawVarint32()))
	if this.recLimit_ <= 0 {
		pbFail_(pbErrRecursion_)
	}
	p := pbParser_{limit: this.recLimit_, alias: this.alias_}
	p.merge_(pbDataOf_(p0), data, 1)
}

// mergeInto_ parses the rest of the input as one message.
func (this *CodedInputStream) mergeInto_(d *pbData_) {
	p := pbParser_{limit: this.recLimit_, alias: this.alias_}
	p.merge_(d, this.rest_(), 0)
	this.lastTag_ = 0
}

// ---------------------------------------------------------------- CodedOutputStream

// CodedOutputStream writes into a byte array of fixed size.
type CodedOutputStream struct {
	buf_ []byte
	pos_ int
}

func CodedOutputStream_newInstance(p0 []int8) *CodedOutputStream {
	return &CodedOutputStream{buf_: pbU8_(p0)}
}

func CodedOutputStream_computeUInt32SizeNoTag(p0 int32) int32 {
	return int32(pbVarintSize_(uint64(uint32(p0))))
}

func (this *CodedOutputStream) put_(b []byte) {
	if len(b) > len(this.buf_)-this.pos_ {
		// com.google.protobuf.CodedOutputStream.OutOfSpaceException is an IOException.
		panic(new_IOException_String("CodedOutputStream was writing to a flat byte array and ran out of space.").self())
	}
	this.pos_ += copy(this.buf_[this.pos_:], b)
}

func (this *CodedOutputStream) writeUInt32NoTag(p0 int32) {
	this.put_(pbAppendVarint_(nil, uint64(uint32(p0))))
}

func (this *CodedOutputStream) writeRawBytes(p0 []int8) { this.put_(pbU8_(p0)) }

func (this *CodedOutputStream) flush() {}

func pbEnumValueOf_(javaName string, name string) {
	if name == nullstr_ {
		panic(new_NullPointerException_String("Name is null").self())
	}
	panic(new_IllegalArgumentException_String("No enum constant " + javaName + "." + name).self())
}

func (this *AbstractMessage_Builder) getAllFields() Map { return pbAllFields_(&this.pbData_) }

func pbParse_(d *pbDesc_, data []byte, alias bool) any {
	m := d.newMsg_()
	p := pbParser_{limit: 100, alias: alias}
	p.merge_(m, data, 0)
	return m.me_()
}

func (this *pbData_) gStrBytes_(i int) *ByteString {
	return ByteString_copyFromUtf8(this.get_(i).(string))
}

// mergeData_ is Message.Builder.mergeFrom(Message) on storage.
func (this *pbData_) mergeData_(o *pbData_) {
	if this.d_ != o.d_ {
		panic(new_IllegalArgumentException_String("mergeFrom(Message) can only merge messages of the same type.").self())
	}
	for _, f := range o.d_.fields {
		if !o.hasF_(f) {
			continue
		}
		switch {
		case f.rep:
			for _, e := range o.elems_(f) {
				this.put_(f, pbBuilt_(e))
			}
		case f.kind == pbMessage_:
			this.mergeMsgField_(f, pbBuiltP_(*(*unsafe.Pointer)(o.fp_(f))))
		default:
			this.put_(f, o.slot_(f))
		}
	}
	if u := o.unkBytes_(); len(u) > 0 {
		t := this.unkBytes_()
		this.setUnk_(pbSortUnknown_(append(t[:len(t):len(t)], u...)))
	}
}

func (this *AbstractMessageLite_Builder) clrOneof_(oneof int32) { this.clearOneof_(oneof, nil) }

// checkIndex_ checks an index into repeated field f, which may be the length too if extra is 1.
func (this *AbstractMessageLite_Builder) checkIndex_(f *pbField_, idx int32, extra int) {
	if n := (*pbSlice_)(this.fp_(f)).n; idx < 0 || int(idx) >= n+extra {
		pbIndexError_(idx, n)
	}
}

func (this *AbstractMessageLite_Builder) setAt_(i int, idx int32, v any) {
	f := this.d_.fields[i]
	this.own_()
	this.checkValue_(v)
	this.checkIndex_(f, idx, 0)
	this.repOp_(f, pbOpSet_, int(idx), v)
}

func (this *AbstractMessageLite_Builder) addAt_(i int, idx int32, v any) {
	f := this.d_.fields[i]
	this.own_()
	this.checkValue_(v)
	this.checkIndex_(f, idx, 1)
	this.repOp_(f, pbOpInsert_, int(idx), v)
}

func (this *AbstractMessageLite_Builder) rm_(i int, idx int32) {
	f := this.d_.fields[i]
	this.own_()
	this.checkIndex_(f, idx, 0)
	this.repOp_(f, pbOpDelete_, int(idx), nil)
}

// mergeMsg_ is mergeFoo(Foo) for a singular message field.
func (this *AbstractMessageLite_Builder) mergeMsg_(i int, m any) {
	if m == nil {
		pbNPE_()
	}
	f := this.d_.fields[i]
	this.mergeMsgField_(f, f.ptrOf_(m))
}

// bldAt_ is getFooBuilder(int) for a repeated message field.
func (this *AbstractMessageLite_Builder) bldAt_(i int, idx int32) any {
	f := this.d_.fields[i]
	this.own_()
	this.checkIndex_(f, idx, 0)
	a := *(*[]unsafe.Pointer)(this.fp_(f))
	e := (*pbData_)(a[idx])
	if !e.isBuilder_() {
		e = e.toBuilder_()
		a[idx] = unsafe.Pointer(e)
	}
	return e.me_()
}

// addBldAt_ is addFooBuilder(int).
func (this *AbstractMessageLite_Builder) addBldAt_(i int, idx int32) any {
	b := this.d_.fields[i].md.newBuilder_().me_()
	this.addAt_(i, idx, b)
	return b
}

// bldList_ is getFooBuilderList().
func (this *AbstractMessageLite_Builder) bldList_(i int) List {
	n := int(this.cnt_(i))
	a := make([]any, n)
	for j := range a {
		a[j] = this.bldAt_(i, int32(j))
	}
	return listOf_(a)
}

func pbBuilderFrom_(d *pbDesc_, proto any) any {
	if proto == nil {
		pbNPE_()
	}
	return proto.(pbM_).pbm_().toBuilder_().me_()
}

func (d *pbDesc_) descriptor_() *Descriptors_Descriptor {
	d.init_()
	if d.gd == nil {
		d.gd = &Descriptors_Descriptor{d_: d}
		d.gd.name_ = pbSimpleName_(d.name)
		d.gd.fullName_ = d.name
	}
	return d.gd
}

func ByteString_copyFromUtf8(p0 string) *ByteString {
	if p0 == nullstr_ {
		pbNPE_()
	}
	if p0 == "" {
		return ByteString_EMPTY
	}
	return &ByteString{b_: []byte(p0)}
}

func (this *ByteString) toStringUtf8() string { return string(this.b_) }

// mergeMsgField_ merges message m into singular message field f.
func (this *pbData_) mergeMsgField_(f *pbField_, m unsafe.Pointer) {
	pp := (*unsafe.Pointer)(this.fp_(f))
	x := (*pbData_)(*pp)
	switch {
	case x == nil:
		this.stMsg_(f, m)
	case x.isBuilder_():
		x.mergeData_((*pbData_)(m))
	default:
		n := x.clone_()
		n.mergeData_((*pbData_)(m))
		*pp = unsafe.Pointer(n)
	}
}
