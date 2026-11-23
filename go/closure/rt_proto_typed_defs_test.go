package closure

// A message with a field of every kind the runtime implements (the compiler's own messages use
// nine of them), declared by hand as tools/protogen declares the generated ones. Written by a
// script; see TestProtoTypedAllKinds.

import "unsafe"

type pbTestAll_x_ struct {
	f_double_     float64
	f_float_      float32
	f_int64_      int64
	f_uint64_     int64
	f_int32_      int32
	f_fixed64_    int64
	f_fixed32_    int32
	f_bool_       bool
	f_string_     string
	f_bytes_      *ByteString
	f_uint32_     int32
	f_enum_       int32
	f_sfixed32_   int32
	f_sfixed64_   int64
	f_sint32_     int32
	f_sint64_     int64
	f_msg_        *pbTestAll_
	o_double_     float64
	o_float_      float32
	o_int64_      int64
	o_int32_      int32
	o_bool_       bool
	o_string_     string
	o_bytes_      *ByteString
	o_uint32_     int32
	o_enum_       int32
	o_def_int32_  int32
	o_def_string_ string
	o_def_bool_   bool
	o_def_double_ float64
	r_double_     []float64
	r_float_      []float32
	r_int64_      []int64
	r_uint64_     []int64
	r_int32_      []int32
	r_fixed64_    []int64
	r_fixed32_    []int32
	r_bool_       []bool
	r_uint32_     []int32
	r_enum_       []int32
	r_sfixed32_   []int32
	r_sfixed64_   []int64
	r_sint32_     []int32
	r_sint64_     []int64
	u_int32_      []int32
	r_string_     []string
	r_bytes_      []*ByteString
	r_msg_        []*pbTestAll_
	u_enum_       []int32
	c_int32_      int32
	c_string_     string
	c_bytes_      *ByteString
	c_double_     float64
	c_bool_       bool
	c_msg_        *pbTestAll_
	f_big_        int32
	pbHas_        [1]uint32
}

type pbTestAll_ struct {
	GeneratedMessage
	x_ pbTestAll_x_
}

type pbTestAll_Builder struct {
	GeneratedMessage_Builder
	x_ pbTestAll_x_
}

var pbTestOpen_desc_ = pbRegEnum_(&pbEnumDesc_{name: "pbtest.Open", closed: false, names: []string{"ZERO", "ONE", "BIG", "NEG"}, nums: []int32{0, 1, 100000, -2}})

var pbTestColor_desc_ = pbRegEnum_(&pbEnumDesc_{name: "pbtest.Color", closed: true, names: []string{"RED", "GREEN", "ALIAS"}, nums: []int32{1, 2, 2}})

var pbTestAll_desc_ = pbReg_(&pbDesc_{name: "pbtest.All", oneofs: []string{"choice"},
	newM: func() *pbData_ { return &new(pbTestAll_).pbData_ }, newB: func() *pbData_ { return &new(pbTestAll_Builder).pbData_ },
	asM: func(p unsafe.Pointer) any { return (*pbTestAll_)(p) }, asB: func(p unsafe.Pointer) any { return (*pbTestAll_Builder)(p) },
	cp:   func(dst, src unsafe.Pointer) { (*pbTestAll_)(dst).x_ = (*pbTestAll_)(src).x_ },
	xoff: unsafe.Offsetof(pbTestAll_{}.x_), hoff: unsafe.Offsetof(pbTestAll_{}.x_.pbHas_),
	fields: []*pbField_{
		{name: "f_double", num: 1, kind: pbDouble_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_double_)},
		{name: "f_float", num: 2, kind: pbFloat_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_float_)},
		{name: "f_int64", num: 3, kind: pbInt64_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_int64_)},
		{name: "f_uint64", num: 4, kind: pbUint64_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_uint64_)},
		{name: "f_int32", num: 5, kind: pbInt32_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_int32_)},
		{name: "f_fixed64", num: 6, kind: pbFixed64_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_fixed64_)},
		{name: "f_fixed32", num: 7, kind: pbFixed32_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_fixed32_)},
		{name: "f_bool", num: 8, kind: pbBool_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_bool_)},
		{name: "f_string", num: 9, kind: pbString_, utf8: true, off: unsafe.Offsetof(pbTestAll_{}.x_.f_string_)},
		{name: "f_bytes", num: 11, kind: pbBytes_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_bytes_)},
		{name: "f_uint32", num: 12, kind: pbUint32_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_uint32_)},
		{name: "f_enum", num: 13, kind: pbEnum_, tname: "pbtest.Open", off: unsafe.Offsetof(pbTestAll_{}.x_.f_enum_)},
		{name: "f_sfixed32", num: 14, kind: pbSfixed32_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_sfixed32_)},
		{name: "f_sfixed64", num: 15, kind: pbSfixed64_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_sfixed64_)},
		{name: "f_sint32", num: 16, kind: pbSint32_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_sint32_)},
		{name: "f_sint64", num: 17, kind: pbSint64_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_sint64_)},
		{name: "f_msg", num: 10, kind: pbMessage_, pres: true, tname: "pbtest.All", off: unsafe.Offsetof(pbTestAll_{}.x_.f_msg_)},
		{name: "o_double", num: 21, kind: pbDouble_, pres: true, off: unsafe.Offsetof(pbTestAll_{}.x_.o_double_), hb: 1},
		{name: "o_float", num: 22, kind: pbFloat_, pres: true, off: unsafe.Offsetof(pbTestAll_{}.x_.o_float_), hb: 2},
		{name: "o_int64", num: 23, kind: pbInt64_, pres: true, off: unsafe.Offsetof(pbTestAll_{}.x_.o_int64_), hb: 3},
		{name: "o_int32", num: 25, kind: pbInt32_, pres: true, off: unsafe.Offsetof(pbTestAll_{}.x_.o_int32_), hb: 4},
		{name: "o_bool", num: 28, kind: pbBool_, pres: true, off: unsafe.Offsetof(pbTestAll_{}.x_.o_bool_), hb: 5},
		{name: "o_string", num: 29, kind: pbString_, pres: true, off: unsafe.Offsetof(pbTestAll_{}.x_.o_string_), hb: 6},
		{name: "o_bytes", num: 31, kind: pbBytes_, pres: true, off: unsafe.Offsetof(pbTestAll_{}.x_.o_bytes_), hb: 7},
		{name: "o_uint32", num: 32, kind: pbUint32_, pres: true, off: unsafe.Offsetof(pbTestAll_{}.x_.o_uint32_), hb: 8},
		{name: "o_enum", num: 33, kind: pbEnum_, pres: true, tname: "pbtest.Color", off: unsafe.Offsetof(pbTestAll_{}.x_.o_enum_), hb: 9},
		{name: "o_def_int32", num: 34, kind: pbInt32_, pres: true, def: int32(7), off: unsafe.Offsetof(pbTestAll_{}.x_.o_def_int32_), hb: 10},
		{name: "o_def_string", num: 35, kind: pbString_, pres: true, def: "dflt", off: unsafe.Offsetof(pbTestAll_{}.x_.o_def_string_), hb: 11},
		{name: "o_def_bool", num: 36, kind: pbBool_, pres: true, def: true, off: unsafe.Offsetof(pbTestAll_{}.x_.o_def_bool_), hb: 12},
		{name: "o_def_double", num: 37, kind: pbDouble_, pres: true, def: float64(2.5), off: unsafe.Offsetof(pbTestAll_{}.x_.o_def_double_), hb: 13},
		{name: "r_double", num: 41, kind: pbDouble_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_double_)},
		{name: "r_float", num: 42, kind: pbFloat_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_float_)},
		{name: "r_int64", num: 43, kind: pbInt64_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_int64_)},
		{name: "r_uint64", num: 44, kind: pbUint64_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_uint64_)},
		{name: "r_int32", num: 45, kind: pbInt32_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_int32_)},
		{name: "r_fixed64", num: 46, kind: pbFixed64_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_fixed64_)},
		{name: "r_fixed32", num: 47, kind: pbFixed32_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_fixed32_)},
		{name: "r_bool", num: 48, kind: pbBool_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_bool_)},
		{name: "r_uint32", num: 52, kind: pbUint32_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_uint32_)},
		{name: "r_enum", num: 53, kind: pbEnum_, rep: true, packed: true, tname: "pbtest.Open", off: unsafe.Offsetof(pbTestAll_{}.x_.r_enum_)},
		{name: "r_sfixed32", num: 54, kind: pbSfixed32_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_sfixed32_)},
		{name: "r_sfixed64", num: 55, kind: pbSfixed64_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_sfixed64_)},
		{name: "r_sint32", num: 56, kind: pbSint32_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_sint32_)},
		{name: "r_sint64", num: 57, kind: pbSint64_, rep: true, packed: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_sint64_)},
		{name: "u_int32", num: 60, kind: pbInt32_, rep: true, off: unsafe.Offsetof(pbTestAll_{}.x_.u_int32_)},
		{name: "r_string", num: 61, kind: pbString_, rep: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_string_)},
		{name: "r_bytes", num: 62, kind: pbBytes_, rep: true, off: unsafe.Offsetof(pbTestAll_{}.x_.r_bytes_)},
		{name: "r_msg", num: 63, kind: pbMessage_, rep: true, tname: "pbtest.All", off: unsafe.Offsetof(pbTestAll_{}.x_.r_msg_)},
		{name: "u_enum", num: 64, kind: pbEnum_, rep: true, tname: "pbtest.Color", off: unsafe.Offsetof(pbTestAll_{}.x_.u_enum_)},
		{name: "c_int32", num: 71, kind: pbInt32_, pres: true, oneof: 1, off: unsafe.Offsetof(pbTestAll_{}.x_.c_int32_), hb: 14},
		{name: "c_string", num: 72, kind: pbString_, pres: true, oneof: 1, off: unsafe.Offsetof(pbTestAll_{}.x_.c_string_), hb: 15},
		{name: "c_bytes", num: 73, kind: pbBytes_, pres: true, oneof: 1, off: unsafe.Offsetof(pbTestAll_{}.x_.c_bytes_), hb: 16},
		{name: "c_double", num: 75, kind: pbDouble_, pres: true, oneof: 1, off: unsafe.Offsetof(pbTestAll_{}.x_.c_double_), hb: 17},
		{name: "c_bool", num: 76, kind: pbBool_, pres: true, oneof: 1, off: unsafe.Offsetof(pbTestAll_{}.x_.c_bool_), hb: 18},
		{name: "c_msg", num: 74, kind: pbMessage_, pres: true, oneof: 1, tname: "pbtest.All", off: unsafe.Offsetof(pbTestAll_{}.x_.c_msg_)},
		{name: "f_big", num: 1000, kind: pbInt32_, off: unsafe.Offsetof(pbTestAll_{}.x_.f_big_)},
	}})

func pbTestAllBuilder_() Message_Builder {
	return pbTestAll_desc_.newBuilder_().me_().(Message_Builder)
}
