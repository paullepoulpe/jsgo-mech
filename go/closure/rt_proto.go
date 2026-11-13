package closure

// Protocol buffers runtime: the generic half of the protobuf-java API.
//
// Every generated message (rt_proto_gen.go, written by tools/protogen) is a
// thin typed wrapper around pbData_: a descriptor plus one `any` slot per
// field. Encoding, decoding, text format, equality and builders are all
// implemented here once, driven by the descriptor tables.
//
// Slot contents, by field kind (nil always means "not set"):
//
//	int32, sint32, uint32, fixed32, sfixed32, enum   int32 (enum: its number)
//	int64, sint64, uint64, fixed64, sfixed64         int64
//	float, double, bool, string                      float32, float64, bool, string
//	bytes                                            *ByteString
//	message                                          the message (*Foo); in a builder possibly *Foo_Builder
//	repeated                                         *pbRep_ holding the above
//
// A field without explicit presence (proto3 scalars) is stored as nil when it
// has its default value, so "slot is nil" is the single presence test.

import (
	"bytes"
	"math"
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

type pbField_ struct {
	name   string  // proto field name
	num    int32   // field number
	kind   pbKind_ //
	rep    bool    // repeated
	packed bool    // repeated and written packed
	pres   bool    // has explicit presence (a hasFoo accessor)
	utf8   bool    // string that must be valid UTF-8 when parsed
	oneof  int32   // 1-based index of the containing oneof, 0 if none
	def    any     // explicit default value, in slot representation
	tname  string  // full name of the message or enum type

	idx   int
	owner *pbDesc_
	md    *pbDesc_
	ed    *pbEnumDesc_
	zero  any
	fd    *Descriptors_FieldDescriptor
}

type pbDesc_ struct {
	name   string // full proto name
	fields []*pbField_
	oneofs []string
	newM   func() any
	newB   func() any

	ready  bool
	sorted []*pbField_
	dense  []*pbField_
	byNum  map[int32]*pbField_
	byName map[string]*pbField_
	def    any
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
	max := int32(0)
	for i, f := range d.fields {
		f.idx = i
		f.owner = d
		d.byName[f.name] = f
		if f.num > max {
			max = f.num
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

func (d *pbDesc_) newMsg_() *AbstractMessageLite {
	d.init_()
	me := d.newM()
	m := me.(pbM_).pbm_()
	m.d_ = d
	m.me_ = me
	m.v_ = make([]any, len(d.fields))
	return m
}

func (d *pbDesc_) newBuilder_() *AbstractMessageLite_Builder {
	d.init_()
	me := d.newB()
	b := me.(pbB_).pbb_()
	b.d_ = d
	b.me_ = me
	b.v_ = make([]any, len(d.fields))
	return b
}

func (d *pbDesc_) default_() any {
	if d.def == nil {
		d.def = d.newMsg_().me_
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

type pbRep_ struct {
	a []any
}

type pbData_ struct {
	d_   *pbDesc_
	v_   []any
	unk_ []byte // unknown fields, as they appear on the wire
	me_  any    // the concrete message or builder
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

func pbNPE_() {
	panic(new_NullPointerException_0().self())
}

func pbIsZero_(v any) bool {
	switch x := v.(type) {
	case int32:
		return x == 0
	case int64:
		return x == 0
	case bool:
		return !x
	case string:
		return x == ""
	case float64:
		return math.Float64bits(x) == 0
	case float32:
		return math.Float32bits(x) == 0
	case *ByteString:
		return len(x.b_) == 0
	}
	return false
}

// built_ is the message in a slot that may hold a builder.
func pbBuilt_(v any) any {
	if b, ok := v.(pbB_); ok {
		return b.pbb_().pbBuild_()
	}
	return v
}

func (this *pbData_) get_(i int) any {
	v := this.v_[i]
	if v == nil {
		return this.d_.fields[i].zero
	}
	return v
}

func (this *pbData_) gI32_(i int) int32 {
	if v := this.v_[i]; v != nil {
		return v.(int32)
	}
	return this.d_.fields[i].zero.(int32)
}

func (this *pbData_) gI64_(i int) int64 {
	if v := this.v_[i]; v != nil {
		return v.(int64)
	}
	return this.d_.fields[i].zero.(int64)
}

func (this *pbData_) gF32_(i int) float32 { return this.get_(i).(float32) }
func (this *pbData_) gF64_(i int) float64 { return this.get_(i).(float64) }
func (this *pbData_) gBool_(i int) bool   { return this.get_(i).(bool) }
func (this *pbData_) gStr_(i int) string  { return this.get_(i).(string) }

func (this *pbData_) gBytes_(i int) *ByteString { return this.get_(i).(*ByteString) }

func (this *pbData_) gStrBytes_(i int) *ByteString {
	return ByteString_copyFromUtf8(this.get_(i).(string))
}

// gMsg_ is the value of a singular message field: never nil.
func (this *pbData_) gMsg_(i int) any {
	v := this.v_[i]
	if v == nil {
		return this.d_.fields[i].md.default_()
	}
	return pbBuilt_(v)
}

func (this *pbData_) has_(i int) bool { return this.v_[i] != nil }

func (this *pbData_) rep_(i int) []any {
	if v := this.v_[i]; v != nil {
		return v.(*pbRep_).a
	}
	return nil
}

func (this *pbData_) cnt_(i int) int32 { return int32(len(this.rep_(i))) }

func (this *pbData_) at_(i int, idx int32) any {
	a := this.rep_(i)
	if idx < 0 || int(idx) >= len(a) {
		panic(new_IndexOutOfBoundsException_String("Index: " + istr_(int64(idx)) + ", Size: " + istr_(int64(len(a)))).self())
	}
	return pbBuilt_(a[idx])
}

func (this *pbData_) isBuilder_() bool {
	_, ok := this.me_.(pbB_)
	return ok
}

// list_ is getFooList(): the elements in their boxed Java representation.
func (this *pbData_) list_(i int) List {
	a := this.rep_(i)
	if this.isBuilder_() {
		c := make([]any, len(a))
		for j, v := range a {
			c[j] = pbBuilt_(v)
		}
		a = c
	}
	return listOf_(a[:len(a):len(a)])
}

// enumList_ is getFooList() of a repeated enum field.
func (this *pbData_) enumList_(i int, conv func(int32) any) List {
	a := this.rep_(i)
	c := make([]any, len(a))
	for j, v := range a {
		c[j] = conv(v.(int32))
	}
	return listOf_(c)
}

func (this *pbData_) bytesList_(i int) List {
	a := this.rep_(i)
	c := make([]any, len(a))
	for j, v := range a {
		c[j] = ByteString_copyFromUtf8(v.(string))
	}
	return listOf_(c)
}

// case_ is the field number of the member of a oneof that is set, or 0.
func (this *pbData_) case_(oneof int32) int32 {
	for _, f := range this.d_.fields {
		if f.oneof == oneof && this.v_[f.idx] != nil {
			return f.num
		}
	}
	return 0
}

func (this *pbData_) clearOneof_(oneof int32, except int) {
	for _, f := range this.d_.fields {
		if f.oneof == oneof && f.idx != except {
			this.v_[f.idx] = nil
		}
	}
}

// put_ stores a singular value with the presence rules of its field.
func (this *pbData_) put_(f *pbField_, v any) {
	if f.oneof != 0 {
		this.clearOneof_(f.oneof, f.idx)
	} else if !f.pres && pbIsZero_(v) {
		v = nil
	}
	this.v_[f.idx] = v
}

func (this *pbData_) append_(f *pbField_, v any) {
	r, _ := this.v_[f.idx].(*pbRep_)
	if r == nil {
		r = &pbRep_{}
		this.v_[f.idx] = r
	}
	r.a = append(r.a, v)
}

// copyInto_ copies the fields into fresh storage; nested builders are built
// when the target is a message.
func (this *pbData_) copyInto_(to *pbData_, build bool) {
	for i, v := range this.v_ {
		switch x := v.(type) {
		case nil:
		case *pbRep_:
			if len(x.a) == 0 {
				continue
			}
			c := make([]any, len(x.a))
			copy(c, x.a)
			if build {
				for j, e := range c {
					c[j] = pbBuilt_(e)
				}
			}
			to.v_[i] = &pbRep_{c}
		case pbB_:
			if build {
				to.v_[i] = x.pbb_().pbBuild_()
			} else {
				to.v_[i] = v
			}
		default:
			to.v_[i] = v
		}
	}
	to.unk_ = this.unk_[:len(this.unk_):len(this.unk_)]
}

// mergeData_ is Message.Builder.mergeFrom(Message) on storage.
func (this *pbData_) mergeData_(o *pbData_) {
	if this.d_ != o.d_ {
		panic(new_IllegalArgumentException_String("mergeFrom(Message) can only merge messages of the same type.").self())
	}
	for i, v := range o.v_ {
		if v == nil {
			continue
		}
		f := this.d_.fields[i]
		switch {
		case f.rep:
			for _, e := range v.(*pbRep_).a {
				this.append_(f, pbBuilt_(e))
			}
		case f.kind == pbMessage_:
			this.mergeMsgField_(f, pbBuilt_(v))
		default:
			this.put_(f, v)
		}
	}
	if len(o.unk_) > 0 {
		this.unk_ = pbSortUnknown_(append(this.unk_[:len(this.unk_):len(this.unk_)], o.unk_...))
	}
}

func (this *pbData_) mergeMsgField_(f *pbField_, m any) {
	switch x := this.v_[f.idx].(type) {
	case nil:
		this.put_(f, m)
	case pbB_:
		x.pbb_().mergeData_(pbDataOf_(m))
	default:
		n := f.md.newMsg_()
		pbDataOf_(x).copyInto_(&n.pbData_, true)
		n.mergeData_(pbDataOf_(m))
		this.put_(f, n.me_)
	}
}

// ---------------------------------------------------------------- messages

type AbstractMessageLite struct {
	pbData_
	size_ int32 // serialized size + 1, 0 while unknown
	hash_ int32
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

func (this *AbstractMessage) up_AbstractMessageLite() *AbstractMessageLite {
	if this == nil {
		return nil
	}
	return &this.AbstractMessageLite
}

func (this *AbstractMessageLite) toBuilder() MessageLite_Builder {
	b := this.d_.newBuilder_()
	this.copyInto_(&b.pbData_, false)
	return b.me_.(MessageLite_Builder)
}

func (this *AbstractMessageLite) newBuilderForType() MessageLite_Builder {
	return this.d_.newBuilder_().me_.(MessageLite_Builder)
}

func (this *AbstractMessageLite) getDefaultInstanceForType() MessageLite {
	return this.d_.default_().(MessageLite)
}

func (this *AbstractMessageLite) isInitialized() bool { return true }

func (this *AbstractMessageLite) getSerializedSize() int32 { return int32(pbSizeOf_(this)) }

func (this *AbstractMessageLite) bytes_() []byte {
	return pbAppendMsg_(make([]byte, 0, pbSizeOf_(this)), this)
}

func (this *AbstractMessageLite) toByteArray() []int8 { return pbI8_(this.bytes_()) }

func (this *AbstractMessageLite) toByteString() *ByteString { return &ByteString{b_: this.bytes_()} }

func (this *AbstractMessageLite) writeTo(p0 *OutputStream) { pbWriteStream_(p0, this.bytes_()) }

func (this *AbstractMessageLite) writeTo_CodedOutputStream(p0 *CodedOutputStream) {
	p0.buf_ = pbAppendMsg_(p0.buf_, this)
}

func (this *AbstractMessageLite) writeDelimitedTo(p0 *OutputStream) {
	n := pbSizeOf_(this)
	buf := pbAppendVarint_(make([]byte, 0, n+5), uint64(n))
	pbWriteStream_(p0, pbAppendMsg_(buf, this))
}

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

func (this *AbstractMessage) getDescriptorForType() *Descriptors_Descriptor {
	return this.d_.descriptor_()
}

func (this *AbstractMessage) hasField(p0 *Descriptors_FieldDescriptor) bool {
	return pbHasField_(&this.pbData_, p0)
}

func (this *AbstractMessage) getField(p0 *Descriptors_FieldDescriptor) any {
	return pbGetField_(&this.pbData_, p0)
}

func (this *AbstractMessage) getRepeatedFieldCount(p0 *Descriptors_FieldDescriptor) int32 {
	return this.cnt_(p0.f_.idx)
}

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
	m := this.d_.newMsg_()
	this.copyInto_(&m.pbData_, true)
	return m.me_
}

func (this *AbstractMessageLite_Builder) build() MessageLite { return this.pbBuild_().(MessageLite) }

func (this *AbstractMessageLite_Builder) buildPartial() MessageLite {
	return this.pbBuild_().(MessageLite)
}

func (this *AbstractMessageLite_Builder) clear() MessageLite_Builder {
	for i := range this.v_ {
		this.v_[i] = nil
	}
	this.unk_ = nil
	return this.me_.(MessageLite_Builder)
}

func (this *AbstractMessageLite_Builder) isInitialized() bool { return true }

func (this *AbstractMessageLite_Builder) getDefaultInstanceForType() MessageLite {
	return this.d_.default_().(MessageLite)
}

func (this *AbstractMessageLite_Builder) toString() string {
	return pbText_(&this.pbData_, false, true)
}

func (this *AbstractMessageLite_Builder) mergeBytes_(data []byte, alias bool) {
	p := pbParser_{limit: 100, alias: alias}
	p.merge_(&this.pbData_, data, 0)
}

func (this *AbstractMessageLite_Builder) mergeFrom_byteArr(p0 []int8) MessageLite_Builder {
	this.mergeBytes_(pbU8_(p0), false)
	return this.me_.(MessageLite_Builder)
}

func (this *AbstractMessageLite_Builder) mergeFrom_ByteString(p0 *ByteString) MessageLite_Builder {
	this.mergeBytes_(p0.b_, true)
	return this.me_.(MessageLite_Builder)
}

func (this *AbstractMessageLite_Builder) mergeFrom_InputStream(p0 *InputStream) MessageLite_Builder {
	this.mergeBytes_(pbReadAll_(p0), true)
	return this.me_.(MessageLite_Builder)
}

func (this *AbstractMessageLite_Builder) mergeFrom_CodedInputStream(p0 *CodedInputStream) MessageLite_Builder {
	p0.mergeInto_(&this.pbData_)
	return this.me_.(MessageLite_Builder)
}

func (this *AbstractMessageLite_Builder) mergeFrom_CodedInputStream_ExtensionRegistryLite(p0 *CodedInputStream, p1 *ExtensionRegistryLite) MessageLite_Builder {
	p0.mergeInto_(&this.pbData_)
	return this.me_.(MessageLite_Builder)
}

func (this *AbstractMessageLite_Builder) mergeFrom_MessageLite(p0 MessageLite) MessageLite_Builder {
	this.mergeData_(pbDataOf_(p0))
	return this.me_.(MessageLite_Builder)
}

func (this *AbstractMessageLite_Builder) mergeDelimitedFrom(p0 *InputStream) bool {
	data, ok := pbReadDelimited_(p0)
	if !ok {
		return false
	}
	this.mergeBytes_(data, true)
	return true
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

func (this *AbstractMessageLite_Builder) clr_(i int) { this.v_[i] = nil }

func (this *AbstractMessageLite_Builder) clrOneof_(oneof int32) { this.clearOneof_(oneof, -1) }

func (this *AbstractMessageLite_Builder) add_(i int, v any) {
	this.checkValue_(v)
	this.append_(this.d_.fields[i], v)
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
		this.append_(f, v)
	}
}

func (this *AbstractMessageLite_Builder) checkIndex_(i int, idx int32, extra int) []any {
	a := this.rep_(i)
	if idx < 0 || int(idx) >= len(a)+extra {
		panic(new_IndexOutOfBoundsException_String("Index: " + istr_(int64(idx)) + ", Size: " + istr_(int64(len(a)))).self())
	}
	return a
}

func (this *AbstractMessageLite_Builder) setAt_(i int, idx int32, v any) {
	this.checkValue_(v)
	this.checkIndex_(i, idx, 0)[idx] = v
}

func (this *AbstractMessageLite_Builder) addAt_(i int, idx int32, v any) {
	this.checkValue_(v)
	this.checkIndex_(i, idx, 1)
	this.append_(this.d_.fields[i], nil)
	a := this.rep_(i)
	copy(a[idx+1:], a[idx:])
	a[idx] = v
}

func (this *AbstractMessageLite_Builder) rm_(i int, idx int32) {
	a := this.checkIndex_(i, idx, 0)
	r := this.v_[i].(*pbRep_)
	r.a = append(a[:idx], a[idx+1:]...)
}

// mergeMsg_ is mergeFoo(Foo) for a singular message field.
func (this *AbstractMessageLite_Builder) mergeMsg_(i int, m any) {
	if m == nil {
		pbNPE_()
	}
	f := this.d_.fields[i]
	if this.v_[i] == nil && f.oneof != 0 {
		this.clearOneof_(f.oneof, i)
	}
	this.mergeMsgField_(f, m)
}

// bld_ is getFooBuilder() for a singular message field.
func (this *AbstractMessageLite_Builder) bld_(i int) any {
	f := this.d_.fields[i]
	switch x := this.v_[i].(type) {
	case nil:
		b := f.md.newBuilder_()
		this.put_(f, b.me_)
		return b.me_
	case pbB_:
		return x
	default:
		b := x.(pbM_).pbm_().toBuilder()
		this.v_[i] = b
		return b
	}
}

// bldAt_ is getFooBuilder(int) for a repeated message field.
func (this *AbstractMessageLite_Builder) bldAt_(i int, idx int32) any {
	a := this.checkIndex_(i, idx, 0)
	if _, ok := a[idx].(pbB_); !ok {
		a[idx] = a[idx].(pbM_).pbm_().toBuilder()
	}
	return a[idx]
}

// addBld_ is addFooBuilder().
func (this *AbstractMessageLite_Builder) addBld_(i int) any {
	f := this.d_.fields[i]
	b := f.md.newBuilder_().me_
	this.append_(f, b)
	return b
}

// addBldAt_ is addFooBuilder(int).
func (this *AbstractMessageLite_Builder) addBldAt_(i int, idx int32) any {
	b := this.d_.fields[i].md.newBuilder_().me_
	this.addAt_(i, idx, b)
	return b
}

// bldList_ is getFooBuilderList().
func (this *AbstractMessageLite_Builder) bldList_(i int) List {
	a := this.rep_(i)
	for j := range a {
		this.bldAt_(i, int32(j))
	}
	return listOf_(append([]any(nil), a...))
}

// orBuilderList_ is getFooOrBuilderList().
func (this *pbData_) orBuilderList_(i int) List {
	return listOf_(append([]any(nil), this.rep_(i)...))
}

// ---------------------------------------------------------------- static entry points used by generated code

func pbParse_(d *pbDesc_, data []byte, alias bool) any {
	m := d.newMsg_()
	p := pbParser_{limit: 100, alias: alias}
	p.merge_(&m.pbData_, data, 0)
	return m.me_
}

func pbParseCoded_(d *pbDesc_, in *CodedInputStream) any {
	m := d.newMsg_()
	in.mergeInto_(&m.pbData_)
	return m.me_
}

// pbParseDelimited_ returns nil at end of stream, as parseDelimitedFrom does.
func pbParseDelimited_(d *pbDesc_, in *InputStream) any {
	data, ok := pbReadDelimited_(in)
	if !ok {
		return nil
	}
	return pbParse_(d, data, true)
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

func pbSizeOf_(m *AbstractMessageLite) int {
	if m.size_ != 0 {
		return int(m.size_ - 1)
	}
	n := 0
	for _, f := range m.d_.sorted {
		if v := m.v_[f.idx]; v != nil {
			n += pbFieldSize_(f, v)
		}
	}
	n += len(m.unk_)
	m.size_ = int32(n + 1)
	return n
}

func pbFieldSize_(f *pbField_, v any) int {
	tag := pbVarintSize_(uint64(f.num) << 3)
	if !f.rep {
		return tag + pbValueSize_(f.kind, v)
	}
	a := v.(*pbRep_).a
	if len(a) == 0 {
		return 0
	}
	n := 0
	for _, e := range a {
		n += pbValueSize_(f.kind, e)
	}
	if f.packed {
		return tag + pbVarintSize_(uint64(n)) + n
	}
	return n + tag*len(a)
}

func pbValueSize_(k pbKind_, v any) int {
	switch k {
	case pbInt32_, pbEnum_:
		return pbVarintSize_(uint64(int64(v.(int32))))
	case pbUint32_:
		return pbVarintSize_(uint64(uint32(v.(int32))))
	case pbSint32_:
		x := v.(int32)
		return pbVarintSize_(uint64(uint32(x<<1) ^ uint32(x>>31)))
	case pbInt64_, pbUint64_:
		return pbVarintSize_(uint64(v.(int64)))
	case pbSint64_:
		x := v.(int64)
		return pbVarintSize_(uint64(x<<1) ^ uint64(x>>63))
	case pbBool_:
		return 1
	case pbFixed32_, pbSfixed32_, pbFloat_:
		return 4
	case pbFixed64_, pbSfixed64_, pbDouble_:
		return 8
	case pbString_:
		n := len(v.(string))
		return pbVarintSize_(uint64(n)) + n
	case pbBytes_:
		n := len(v.(*ByteString).b_)
		return pbVarintSize_(uint64(n)) + n
	case pbMessage_:
		n := pbSizeOf_(v.(pbM_).pbm_())
		return pbVarintSize_(uint64(n)) + n
	}
	panic("protobuf: unsupported field kind")
}

// pbGoNaN_ is the bit pattern of Go's math.NaN(), which differs from Java's.
var pbGoNaN_ = math.Float64bits(math.NaN())

func pbAppendMsg_(b []byte, m *AbstractMessageLite) []byte {
	for _, f := range m.d_.sorted {
		v := m.v_[f.idx]
		if v == nil {
			continue
		}
		tag := uint64(f.num)<<3 | uint64(pbWire_[f.kind])
		if !f.rep {
			b = pbAppendVarint_(b, tag)
			b = pbAppendValue_(b, f.kind, v)
			continue
		}
		a := v.(*pbRep_).a
		if len(a) == 0 {
			continue
		}
		if f.packed {
			n := 0
			for _, e := range a {
				n += pbValueSize_(f.kind, e)
			}
			b = pbAppendVarint_(b, uint64(f.num)<<3|2)
			b = pbAppendVarint_(b, uint64(n))
			for _, e := range a {
				b = pbAppendValue_(b, f.kind, e)
			}
			continue
		}
		for _, e := range a {
			b = pbAppendVarint_(b, tag)
			b = pbAppendValue_(b, f.kind, e)
		}
	}
	return append(b, m.unk_...)
}

func pbAppendValue_(b []byte, k pbKind_, v any) []byte {
	switch k {
	case pbInt32_, pbEnum_:
		return pbAppendVarint_(b, uint64(int64(v.(int32))))
	case pbUint32_:
		return pbAppendVarint_(b, uint64(uint32(v.(int32))))
	case pbSint32_:
		x := v.(int32)
		return pbAppendVarint_(b, uint64(uint32(x<<1)^uint32(x>>31)))
	case pbInt64_, pbUint64_:
		return pbAppendVarint_(b, uint64(v.(int64)))
	case pbSint64_:
		x := v.(int64)
		return pbAppendVarint_(b, uint64(x<<1)^uint64(x>>63))
	case pbBool_:
		if v.(bool) {
			return append(b, 1)
		}
		return append(b, 0)
	case pbFixed32_, pbSfixed32_:
		return pbAppendFixed32_(b, uint32(v.(int32)))
	case pbFloat_:
		return pbAppendFixed32_(b, math.Float32bits(v.(float32)))
	case pbFixed64_, pbSfixed64_:
		return pbAppendFixed64_(b, uint64(v.(int64)))
	case pbDouble_:
		bits := math.Float64bits(v.(float64))
		if bits == pbGoNaN_ {
			bits = 0x7ff8000000000000 // Double.NaN
		}
		return pbAppendFixed64_(b, bits)
	case pbString_:
		s := v.(string)
		b = pbAppendVarint_(b, uint64(len(s)))
		return append(b, s...)
	case pbBytes_:
		s := v.(*ByteString).b_
		b = pbAppendVarint_(b, uint64(len(s)))
		return append(b, s...)
	case pbMessage_:
		m := v.(pbM_).pbm_()
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
		d.unk_ = append(d.unk_, data[start:pos]...)
		unknown = true
	}
	if unknown {
		d.unk_ = pbSortUnknown_(d.unk_)
	}
}

// readField_ reads one value of field f (whose wire type has been checked).
func (p *pbParser_) readField_(d *pbData_, f *pbField_, data []byte, pos int, depth int32) int {
	var v any
	switch f.kind {
	case pbInt32_, pbUint32_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		v = int32(x)
	case pbEnum_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		if f.ed.closed {
			if _, ok := f.ed.byNum[int32(x)]; !ok {
				d.unk_ = pbAppendVarint_(d.unk_, uint64(f.num)<<3)
				d.unk_ = pbSortUnknown_(pbAppendVarint_(d.unk_, uint64(int64(int32(x)))))
				return pos
			}
		}
		v = int32(x)
	case pbSint32_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		u := uint32(x)
		v = int32(u>>1) ^ -int32(u&1)
	case pbInt64_, pbUint64_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		v = int64(x)
	case pbSint64_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		v = int64(x>>1) ^ -int64(x&1)
	case pbBool_:
		var x uint64
		x, pos = pbVarint_(data, pos)
		v = x != 0
	case pbFixed32_, pbSfixed32_, pbFloat_:
		if pos+4 > len(data) {
			pbFail_(pbErrTruncated_)
		}
		x := uint32(data[pos]) | uint32(data[pos+1])<<8 | uint32(data[pos+2])<<16 | uint32(data[pos+3])<<24
		pos += 4
		if f.kind == pbFloat_ {
			v = math.Float32frombits(x)
		} else {
			v = int32(x)
		}
	case pbFixed64_, pbSfixed64_, pbDouble_:
		if pos+8 > len(data) {
			pbFail_(pbErrTruncated_)
		}
		x := uint64(data[pos]) | uint64(data[pos+1])<<8 | uint64(data[pos+2])<<16 | uint64(data[pos+3])<<24 |
			uint64(data[pos+4])<<32 | uint64(data[pos+5])<<40 | uint64(data[pos+6])<<48 | uint64(data[pos+7])<<56
		pos += 8
		if f.kind == pbDouble_ {
			v = math.Float64frombits(x)
		} else {
			v = int64(x)
		}
	case pbString_:
		var n int
		n, pos = pbLen_(data, pos)
		s := string(data[pos : pos+n])
		pos += n
		if f.utf8 && !utf8.ValidString(s) {
			pbFail_(pbErrUtf8_)
		}
		v = s
	case pbBytes_:
		var n int
		n, pos = pbLen_(data, pos)
		b := data[pos : pos+n : pos+n]
		pos += n
		if n == 0 {
			v = ByteString_EMPTY
		} else if p.alias {
			v = &ByteString{b_: b}
		} else {
			v = &ByteString{b_: append([]byte(nil), b...)}
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
			switch x := d.v_[f.idx].(type) {
			case nil:
			case pbB_:
				p.merge_(&x.pbb_().pbData_, sub, depth+1)
				return pos
			default:
				m := f.md.newMsg_()
				pbDataOf_(x).copyInto_(&m.pbData_, true)
				p.merge_(&m.pbData_, sub, depth+1)
				d.v_[f.idx] = m.me_
				return pos
			}
		}
		m := f.md.newMsg_()
		p.merge_(&m.pbData_, sub, depth+1)
		v = m.me_
	default:
		panic("protobuf: unsupported field kind")
	}
	if f.rep {
		d.append_(f, v)
	} else {
		d.put_(f, v)
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
	if a.d_ != b.d_ || !bytes.Equal(a.unk_, b.unk_) {
		return false
	}
	for i, x := range a.v_ {
		if !pbValueEqual_(x, b.v_[i]) {
			return false
		}
	}
	return true
}

func pbValueEqual_(x, y any) bool {
	if r, ok := x.(*pbRep_); ok && len(r.a) == 0 {
		x = nil
	}
	if r, ok := y.(*pbRep_); ok && len(r.a) == 0 {
		y = nil
	}
	if x == nil || y == nil {
		return x == nil && y == nil
	}
	switch p := x.(type) {
	case *pbRep_:
		q := y.(*pbRep_)
		if len(p.a) != len(q.a) {
			return false
		}
		for i := range p.a {
			if !pbValueEqual_(p.a[i], q.a[i]) {
				return false
			}
		}
		return true
	case float64:
		q := y.(float64)
		return math.Float64bits(p) == math.Float64bits(q) || p != p && q != q
	case float32:
		q := y.(float32)
		return math.Float32bits(p) == math.Float32bits(q) || p != p && q != q
	case *ByteString:
		return bytes.Equal(p.b_, y.(*ByteString).b_)
	case pbM_, pbB_:
		return pbEqual_(pbDataOf_(x), pbDataOf_(y))
	}
	return x == y
}

func pbHash_(d *pbData_) int32 {
	h := int32(41)
	h = 19*h + String_hashCode(d.d_.name)
	for _, f := range d.d_.sorted {
		v := d.v_[f.idx]
		if r, ok := v.(*pbRep_); ok && len(r.a) == 0 {
			v = nil
		}
		if v == nil {
			continue
		}
		h = 37*h + f.num
		h = 53*h + pbValueHash_(v)
	}
	for _, c := range d.unk_ {
		h = 29*h + int32(c)
	}
	return h
}

func pbValueHash_(v any) int32 {
	switch x := v.(type) {
	case *pbRep_:
		h := int32(1)
		for _, e := range x.a {
			h = 31*h + pbValueHash_(e)
		}
		return h
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

func (this *Descriptors_GenericDescriptor) getName() string     { return this.name_ }
func (this *Descriptors_GenericDescriptor) getFullName() string { return this.fullName_ }
func (this *Descriptors_GenericDescriptor) toString() string    { return this.fullName_ }

type Descriptors_FieldDescriptor struct {
	Descriptors_GenericDescriptor
	f_ *pbField_
}

func (this *Descriptors_FieldDescriptor) up_Descriptors_GenericDescriptor() *Descriptors_GenericDescriptor {
	if this == nil {
		return nil
	}
	return &this.Descriptors_GenericDescriptor
}

func (this *Descriptors_FieldDescriptor) getNumber() int32 { return this.f_.num }
func (this *Descriptors_FieldDescriptor) getIndex() int32  { return int32(this.f_.idx) }
func (this *Descriptors_FieldDescriptor) isRepeated() bool { return this.f_.rep }
func (this *Descriptors_FieldDescriptor) isPacked() bool   { return this.f_.packed }
func (this *Descriptors_FieldDescriptor) hasPresence() bool {
	return !this.f_.rep && (this.f_.pres || this.f_.oneof != 0)
}

func (this *Descriptors_FieldDescriptor) getContainingType() *Descriptors_Descriptor {
	return this.f_.owner.descriptor_()
}

func (this *Descriptors_FieldDescriptor) getMessageType() *Descriptors_Descriptor {
	if this.f_.md == nil {
		panic(new_UnsupportedOperationException_String("This field is not of message type. (" + this.fullName_ + ")").self())
	}
	return this.f_.md.descriptor_()
}

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

func (this *Descriptors_EnumValueDescriptor) up_Descriptors_GenericDescriptor() *Descriptors_GenericDescriptor {
	if this == nil {
		return nil
	}
	return &this.Descriptors_GenericDescriptor
}

func (this *Descriptors_EnumValueDescriptor) getNumber() int32 { return this.num_ }
func (this *Descriptors_EnumValueDescriptor) getIndex() int32  { return this.index_ }
func (this *Descriptors_EnumValueDescriptor) toString() string { return this.name_ }

func pbSimpleName_(full string) string {
	for i := len(full) - 1; i >= 0; i-- {
		if full[i] == '.' {
			return full[i+1:]
		}
	}
	return full
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
		a := d.rep_(f.idx)
		out := make([]any, len(a))
		for i, e := range a {
			out[i] = pbReflect_(f, e)
		}
		return listOf_(out)
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
		v := d.v_[f.idx]
		if v == nil {
			continue
		}
		if f.rep && len(v.(*pbRep_).a) == 0 {
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

func pbStreamSelf_(s any, fallback any) any {
	if s == nil {
		return fallback
	}
	return s
}

// pbWriteStream_ writes bytes to a java.io.OutputStream, through whichever of
// its write methods the stream's class has.
func pbWriteStream_(out *OutputStream, b []byte) {
	if out == nil {
		pbNPE_()
	}
	if len(b) == 0 {
		return
	}
	switch w := pbStreamSelf_(out.self(), out).(type) {
	case interface{ write_byteArr_int_int([]int8, int32, int32) }:
		w.write_byteArr_int_int(pbI8_(b), 0, int32(len(b)))
	case interface{ write([]int8, int32, int32) }:
		w.write(pbI8_(b), 0, int32(len(b)))
	case interface{ write_byteArr([]int8) }:
		w.write_byteArr(pbI8_(b))
	case interface{ write(int32) }:
		for _, c := range b {
			w.write(int32(c))
		}
	default:
		panic("protobuf: cannot write to " + goTypeName_(w))
	}
}

// pbReadStream_ reads up to len(b) bytes from a java.io.InputStream; -1 at end.
func pbReadStream_(in *InputStream, b []byte) int {
	if in == nil {
		pbNPE_()
	}
	if len(b) == 0 {
		return 0
	}
	one := func(read func() int32) int {
		c := read()
		if c < 0 {
			return -1
		}
		b[0] = byte(c)
		return 1
	}
	switch r := pbStreamSelf_(in.self(), in).(type) {
	case interface {
		read_byteArr_int_int([]int8, int32, int32) int32
	}:
		return int(r.read_byteArr_int_int(pbI8_(b), 0, int32(len(b))))
	case interface {
		read([]int8, int32, int32) int32
	}:
		return int(r.read(pbI8_(b), 0, int32(len(b))))
	case interface{ read_byteArr([]int8) int32 }:
		return int(r.read_byteArr(pbI8_(b)))
	case interface{ read([]int8) int32 }:
		return int(r.read(pbI8_(b)))
	case interface{ read_0() int32 }:
		return one(r.read_0)
	case interface{ read() int32 }:
		return one(r.read)
	default:
		panic("protobuf: cannot read from " + goTypeName_(r))
	}
}

func pbReadAll_(in *InputStream) []byte {
	buf := make([]byte, 0, 4096)
	for {
		if len(buf) == cap(buf) {
			buf = append(buf, 0)[:len(buf)]
		}
		n := pbReadStream_(in, buf[len(buf):cap(buf)])
		if n < 0 {
			return buf
		}
		buf = buf[:len(buf)+n]
	}
}

func pbReadFull_(in *InputStream, b []byte) {
	for len(b) > 0 {
		n := pbReadStream_(in, b)
		if n < 0 {
			pbFail_(pbErrTruncated_)
		}
		b = b[n:]
	}
}

// pbReadDelimited_ reads a varint length and that many bytes, and no more.
func pbReadDelimited_(in *InputStream) ([]byte, bool) {
	var one [1]byte
	var size uint64
	for shift := uint(0); ; shift += 7 {
		n := pbReadStream_(in, one[:])
		if n < 0 {
			if shift == 0 {
				return nil, false
			}
			pbFail_(pbErrTruncated_)
		}
		if shift >= 64 {
			pbFail_(pbErrVarint_)
		}
		size |= uint64(one[0]&0x7f) << shift
		if one[0] < 0x80 {
			break
		}
	}
	n := int(int32(size))
	if n < 0 {
		pbFail_(pbErrNegative_)
	}
	data := make([]byte, n)
	pbReadFull_(in, data)
	return data, true
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

func (this *ByteString) toByteArray() []int8 {
	return pbI8_(append([]byte(nil), this.b_...))
}

func (this *ByteString) toStringUtf8() string { return string(this.b_) }

func (this *ByteString) isValidUtf8() bool { return utf8.Valid(this.b_) }

func (this *ByteString) newCodedInput() *CodedInputStream {
	return &CodedInputStream{buf_: this.b_, limit_: math.MaxInt, recLimit_: 100, alias_: true}
}

func (this *ByteString) concat(p0 *ByteString) *ByteString {
	if len(p0.b_) == 0 {
		return this
	}
	if len(this.b_) == 0 {
		return p0
	}
	return &ByteString{b_: append(append(make([]byte, 0, len(this.b_)+len(p0.b_)), this.b_...), p0.b_...)}
}

func (this *ByteString) substring(p0 int32) *ByteString {
	return this.substring_int_int(p0, int32(len(this.b_)))
}

func (this *ByteString) substring_int_int(p0 int32, p1 int32) *ByteString {
	if p0 < 0 || p1 > int32(len(this.b_)) || p0 > p1 {
		panic(new_IndexOutOfBoundsException_String("Beginning index: " + istr_(int64(p0)) + ", ending index: " + istr_(int64(p1))).self())
	}
	if p0 == p1 {
		return ByteString_EMPTY
	}
	return &ByteString{b_: this.b_[p0:p1:p1]}
}

func (this *ByteString) startsWith(p0 *ByteString) bool { return bytes.HasPrefix(this.b_, p0.b_) }
func (this *ByteString) endsWith(p0 *ByteString) bool   { return bytes.HasSuffix(this.b_, p0.b_) }

func (this *ByteString) writeTo(p0 *OutputStream) { pbWriteStream_(p0, this.b_) }

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

func (this *ByteString_Output) up_OutputStream() *OutputStream {
	if this == nil {
		return nil
	}
	return &this.OutputStream
}

func (this *ByteString_Output) write(p0 int32) { this.buf_ = append(this.buf_, byte(p0)) }

func (this *ByteString_Output) write_byteArr(p0 []int8) {
	this.buf_ = append(this.buf_, pbU8_(p0)...)
}

func (this *ByteString_Output) write_byteArr_int_int(p0 []int8, p1 int32, p2 int32) {
	this.buf_ = append(this.buf_, pbU8_(p0[p1:p1+p2])...)
}

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

func (this *ByteString_Output) writeTo(p0 *OutputStream) { pbWriteStream_(p0, this.buf_) }

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

type CodedInputStream struct {
	buf_      []byte
	pos_      int
	in_       *InputStream // nil when reading from memory
	eof_      bool
	base_     int // bytes consumed before buf_[0], since the last resetSizeCounter
	limit_    int // absolute position (base_+pos_) the reader may not pass
	lastTag_  int32
	recLimit_ int32
	alias_    bool
}

func CodedInputStream_newInstance(p0 *InputStream) *CodedInputStream {
	if p0 == nil {
		pbNPE_()
	}
	return &CodedInputStream{in_: p0, limit_: math.MaxInt, recLimit_: 100, alias_: true}
}

// fill_ makes n bytes available at pos_ if the input has them.
func (this *CodedInputStream) fill_(n int) bool {
	if len(this.buf_)-this.pos_ >= n {
		return true
	}
	if this.in_ == nil || this.eof_ {
		return false
	}
	rest := len(this.buf_) - this.pos_
	size := 4096
	if n > size {
		size = n
	}
	nb := make([]byte, rest, rest+size)
	copy(nb, this.buf_[this.pos_:])
	this.base_ += this.pos_
	this.pos_ = 0
	this.buf_ = nb
	for len(this.buf_) < n {
		got := pbReadStream_(this.in_, this.buf_[len(this.buf_):cap(this.buf_)])
		if got < 0 {
			this.eof_ = true
			return false
		}
		this.buf_ = this.buf_[:len(this.buf_)+got]
	}
	return true
}

func (this *CodedInputStream) total_() int { return this.base_ + this.pos_ }

func (this *CodedInputStream) isAtEnd() bool {
	return this.total_() >= this.limit_ || !this.fill_(1)
}

func (this *CodedInputStream) getTotalBytesRead() int32 { return int32(this.total_()) }

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

func (this *CodedInputStream) setSizeLimit(p0 int32) int32 { return math.MaxInt32 }

func (this *CodedInputStream) pushLimit(p0 int32) int32 {
	if p0 < 0 {
		pbFail_(pbErrNegative_)
	}
	old := this.limit_
	nl := this.total_() + int(p0)
	if nl > old {
		pbFail_(pbErrTruncated_)
	}
	this.limit_ = nl
	if old == math.MaxInt {
		return math.MaxInt32
	}
	return int32(old)
}

func (this *CodedInputStream) popLimit(p0 int32) {
	if p0 == math.MaxInt32 {
		this.limit_ = math.MaxInt
	} else {
		this.limit_ = int(p0)
	}
}

func (this *CodedInputStream) getBytesUntilLimit() int32 {
	if this.limit_ == math.MaxInt {
		return -1
	}
	return int32(this.limit_ - this.total_())
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
	if this.in_ != nil && !this.eof_ {
		rest := append(this.buf_[this.pos_:len(this.buf_):len(this.buf_)], pbReadAll_(this.in_)...)
		this.base_ += this.pos_
		this.pos_ = 0
		this.buf_ = rest
		this.eof_ = true
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

func (this *CodedInputStream) getLastTag() int32 { return this.lastTag_ }

func (this *CodedInputStream) checkLastTagWas(p0 int32) {
	if this.lastTag_ != p0 {
		pbFail_(pbErrEndGroup_)
	}
}

func (this *CodedInputStream) readInt32() int32  { return this.readRawVarint32() }
func (this *CodedInputStream) readUInt32() int32 { return this.readRawVarint32() }
func (this *CodedInputStream) readEnum() int32   { return this.readRawVarint32() }
func (this *CodedInputStream) readInt64() int64  { return this.readRawVarint64() }
func (this *CodedInputStream) readUInt64() int64 { return this.readRawVarint64() }
func (this *CodedInputStream) readBool() bool    { return this.readRawVarint64() != 0 }

func (this *CodedInputStream) readSInt32() int32 {
	u := uint32(this.readRawVarint32())
	return int32(u>>1) ^ -int32(u&1)
}

func (this *CodedInputStream) readSInt64() int64 {
	u := uint64(this.readRawVarint64())
	return int64(u>>1) ^ -int64(u&1)
}

func (this *CodedInputStream) readRawLittleEndian32() int32 {
	b := this.raw_(4)
	return int32(uint32(b[0]) | uint32(b[1])<<8 | uint32(b[2])<<16 | uint32(b[3])<<24)
}

func (this *CodedInputStream) readRawLittleEndian64() int64 {
	b := this.raw_(8)
	return int64(uint64(b[0]) | uint64(b[1])<<8 | uint64(b[2])<<16 | uint64(b[3])<<24 |
		uint64(b[4])<<32 | uint64(b[5])<<40 | uint64(b[6])<<48 | uint64(b[7])<<56)
}

func (this *CodedInputStream) readFixed32() int32  { return this.readRawLittleEndian32() }
func (this *CodedInputStream) readSFixed32() int32 { return this.readRawLittleEndian32() }
func (this *CodedInputStream) readFixed64() int64  { return this.readRawLittleEndian64() }
func (this *CodedInputStream) readSFixed64() int64 { return this.readRawLittleEndian64() }

func (this *CodedInputStream) readFloat() float32 {
	return math.Float32frombits(uint32(this.readRawLittleEndian32()))
}

func (this *CodedInputStream) readDouble() float64 {
	return math.Float64frombits(uint64(this.readRawLittleEndian64()))
}

func (this *CodedInputStream) readBytes() *ByteString {
	b := this.raw_(int(this.readRawVarint32()))
	if len(b) == 0 {
		return ByteString_EMPTY
	}
	if !this.alias_ {
		b = append([]byte(nil), b...)
	}
	return &ByteString{b_: b}
}

func (this *CodedInputStream) readByteArray() []int8 {
	return pbI8_(append([]byte(nil), this.raw_(int(this.readRawVarint32()))...))
}

func (this *CodedInputStream) readString() string {
	return string(this.raw_(int(this.readRawVarint32())))
}

func (this *CodedInputStream) readStringRequireUtf8() string {
	s := this.readString()
	if !utf8.ValidString(s) {
		pbFail_(pbErrUtf8_)
	}
	return s
}

func (this *CodedInputStream) skipField(p0 int32) bool {
	switch p0 & 7 {
	case 0:
		this.readRawVarint64()
	case 1:
		this.raw_(8)
	case 2:
		this.raw_(int(this.readRawVarint32()))
	case 3:
		for {
			tag := this.readTag()
			if tag == 0 || !this.skipField(tag) {
				break
			}
		}
		this.checkLastTagWas(int32(uint32(p0)>>3<<3 | 4))
	case 4:
		return false
	case 5:
		this.raw_(4)
	default:
		pbFail_(pbErrWireType_)
	}
	return true
}

func (this *CodedInputStream) skipMessage() {
	for {
		tag := this.readTag()
		if tag == 0 || !this.skipField(tag) {
			return
		}
	}
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

type CodedOutputStream struct {
	buf_ []byte
	out_ *OutputStream
}

func (this *CodedOutputStream) flush() {
	if this.out_ != nil && len(this.buf_) > 0 {
		pbWriteStream_(this.out_, this.buf_)
		this.buf_ = this.buf_[:0]
	}
}

func (this *CodedOutputStream) writeTag(p0 int32, p1 int32) {
	this.buf_ = pbAppendVarint_(this.buf_, uint64(uint32(p0)<<3|uint32(p1)))
}

func (this *CodedOutputStream) writeUInt32NoTag(p0 int32) {
	this.buf_ = pbAppendVarint_(this.buf_, uint64(uint32(p0)))
}

func (this *CodedOutputStream) writeInt32NoTag(p0 int32) {
	this.buf_ = pbAppendVarint_(this.buf_, uint64(int64(p0)))
}

func (this *CodedOutputStream) writeUInt64NoTag(p0 int64) {
	this.buf_ = pbAppendVarint_(this.buf_, uint64(p0))
}

func (this *CodedOutputStream) writeInt64NoTag(p0 int64) {
	this.buf_ = pbAppendVarint_(this.buf_, uint64(p0))
}

func (this *CodedOutputStream) writeBoolNoTag(p0 bool) {
	if p0 {
		this.buf_ = append(this.buf_, 1)
	} else {
		this.buf_ = append(this.buf_, 0)
	}
}

func (this *CodedOutputStream) writeFixed32NoTag(p0 int32) {
	this.buf_ = pbAppendFixed32_(this.buf_, uint32(p0))
}

func (this *CodedOutputStream) writeFixed64NoTag(p0 int64) {
	this.buf_ = pbAppendFixed64_(this.buf_, uint64(p0))
}

func (this *CodedOutputStream) writeStringNoTag(p0 string) {
	this.buf_ = append(pbAppendVarint_(this.buf_, uint64(len(p0))), p0...)
}

func (this *CodedOutputStream) writeBytesNoTag(p0 *ByteString) {
	this.buf_ = append(pbAppendVarint_(this.buf_, uint64(len(p0.b_))), p0.b_...)
}

func (this *CodedOutputStream) writeMessageNoTag(p0 MessageLite) {
	m := p0.(pbM_).pbm_()
	this.buf_ = pbAppendMsg_(pbAppendVarint_(this.buf_, uint64(pbSizeOf_(m))), m)
}

func (this *CodedOutputStream) writeInt32(p0 int32, p1 int32) {
	this.writeTag(p0, 0)
	this.writeInt32NoTag(p1)
}

func (this *CodedOutputStream) writeUInt32(p0 int32, p1 int32) {
	this.writeTag(p0, 0)
	this.writeUInt32NoTag(p1)
}

func (this *CodedOutputStream) writeInt64(p0 int32, p1 int64) {
	this.writeTag(p0, 0)
	this.writeInt64NoTag(p1)
}

func (this *CodedOutputStream) writeBool(p0 int32, p1 bool) {
	this.writeTag(p0, 0)
	this.writeBoolNoTag(p1)
}

func (this *CodedOutputStream) writeString(p0 int32, p1 string) {
	this.writeTag(p0, 2)
	this.writeStringNoTag(p1)
}

func (this *CodedOutputStream) writeBytes(p0 int32, p1 *ByteString) {
	this.writeTag(p0, 2)
	this.writeBytesNoTag(p1)
}

func (this *CodedOutputStream) writeMessage(p0 int32, p1 MessageLite) {
	this.writeTag(p0, 2)
	this.writeMessageNoTag(p1)
}

