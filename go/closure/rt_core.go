package closure

// Core runtime of the translated code: conversions between static Go types and
// the dynamic `any` world that erased Java generics live in.

import (
	"fmt"
	"math"
	"reflect"
	"strconv"
)

// nullstr_ is the Go spelling of a null Java String. It is a string no program
// produces otherwise.
const nullstr_ = "\x00\xff<null>\xff\x00"

// obj_ turns a typed pointer (or other comparable value) into an `any` that is
// nil when the pointer is nil.
func obj_[T comparable](x T) any {
	var zero T
	if x == zero {
		return nil
	}
	return x
}

// cast_ is a Java reference cast: nil stays nil (the zero value of T).
func cast_[T any](v any) T {
	if v == nil {
		var zero T
		return zero
	}
	return v.(T)
}

// is_ is `v instanceof T` for types that have no subclasses.
func is_[T any](v any) bool {
	_, ok := v.(T)
	return ok
}

func str_(v any) string {
	if v == nil {
		return nullstr_
	}
	return v.(string)
}

func strobj_(s string) any {
	if s == nullstr_ {
		return nil
	}
	return s
}

// isnull_ is `v == null` for a value of a type argument, as the erased code would see it once
// boxed: a String is null when it is nullstr_, a class pointer, handle or interface when it is
// nil (obj_ and box_ make those nil), an enum constant when it is 0; arrays and numbers box to
// non-null objects.
func isnull_[T any](v T) bool {
	if p, ok := any(&v).(*string); ok {
		return *p == nullstr_
	}
	x := any(v)
	if x == nil {
		return true
	}
	if h, ok := x.(interface{ box_() any }); ok {
		return h.box_() == nil
	}
	r := reflect.ValueOf(x)
	if r.Kind() == reflect.Pointer {
		return r.IsNil()
	}
	// an enum that is an integer type: 0 is null
	if e, ok := x.(enumConst_); ok {
		return e.enum_() == nil
	}
	return false
}

// boxValue_ is a value of a type argument boxed as the erased code boxes it: the most derived
// object of a class, the handle's box, a String as itself (never null here).
func boxValue_[T any](v T) any {
	x := any(v)
	if s, ok := x.(interface{ self() any }); ok {
		return s.self()
	}
	if h, ok := x.(interface{ box_() any }); ok {
		return h.box_()
	}
	return x
}

// null_ is Java null as a value of a type argument T: nullstr_ for a String, the zero value
// otherwise (nil for a class or an interface, the zero handle).
func null_[T any]() T {
	var z T
	if p, ok := any(&z).(*string); ok {
		*p = nullstr_
	}
	return z
}

// boxNull_ is a value of a type argument boxed as the erased code boxes it (strobj_, obj_,
// self(), box_()): null is nil.
func boxNull_[T comparable](v T) any {
	switch x := any(v).(type) {
	case string:
		return strobj_(x)
	case Node:
		return x.box_()
	}
	var z T
	if v == z {
		return nil
	}
	return boxValue_(v)
}

// equalsT_ is a.equals(b) for values of a type argument: Object_equals of their boxes, without
// boxing a String (a null a throws NullPointerException, as Object_equals does).
func equalsT_[T comparable](a, b T) bool {
	if s, ok := any(a).(string); ok {
		if s == nullstr_ {
			panic(new_NullPointerException_0().self())
		}
		return a == b
	}
	return Object_equals(boxNull_(a), boxNull_(b))
}

// objectsEqualsT_ is Objects.equals(a, b) for values of a type argument.
func objectsEqualsT_[T comparable](a, b T) bool {
	if _, ok := any(a).(string); ok {
		return a == b
	}
	return objequals_(boxNull_(a), boxNull_(b))
}

// hashT_ is v.hashCode() for a value of a type argument: Object_hashCode of its box, without
// boxing a String.
func hashT_[T comparable](v T) int32 {
	if s, ok := any(v).(string); ok {
		if s == nullstr_ {
			panic(new_NullPointerException_0().self())
		}
		return String_hashCode(s)
	}
	return Object_hashCode(boxNull_(v))
}

func isstr_(v any) bool {
	_, ok := v.(string)
	return ok
}

func isNumber_(v any) bool {
	switch v.(type) {
	case int32, int64, int16, int8, float32, float64:
		return true
	}
	return false
}

// sstr_ is a String operand of string concatenation: null prints as "null".
func sstr_(s string) string {
	if s == nullstr_ {
		return "null"
	}
	return s
}

func chrstr_(c uint16) string { return String_valueOf_char(c) }

func boolstr_(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func istr_(i int64) string { return strconv.FormatInt(i, 10) }

func dstr_(d float64) string { return Double_toString(d) }

func fstr_(f float32) string { return Float_toString(f) }

// objstr_ is String.valueOf(Object).
func objstr_(v any) string {
	if v == nil {
		return "null"
	}
	return Object_toString(v)
}

func band_(a, b bool) bool { return a && b }

func bor_(a, b bool) bool { return a || b }

type integer_ interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~uint16
}

// f2i_ is Java's float-to-integer conversion: NaN is 0, out-of-range saturates.
func f2i_[T integer_](f float64) T {
	if f != f {
		return 0
	}
	var z T
	if _, isLong := any(z).(int64); isLong {
		var l int64
		if f >= 9.223372036854775807e18 {
			l = math.MaxInt64
		} else if f <= -9.223372036854775808e18 {
			l = math.MinInt64
		} else {
			l = int64(f)
		}
		return any(l).(T)
	}
	// Narrower targets convert through int.
	var i int32
	if f >= math.MaxInt32 {
		i = math.MaxInt32
	} else if f <= math.MinInt32 {
		i = math.MinInt32
	} else {
		i = int32(f)
	}
	return T(i)
}

func fmod_(a, b float64) float64 { return math.Mod(a, b) }

type number_ interface {
	~int8 | ~int16 | ~int32 | ~int64 | ~uint16 | ~float32 | ~float64
}

// unbox_ unboxes a boxed primitive to T, widening if the box holds a narrower type.
func unbox_[T any](v any) T {
	if t, ok := v.(T); ok {
		return t
	}
	if v == nil {
		panic(new_NullPointerException_0().self())
	}
	var z T
	var out any
	switch any(z).(type) {
	case int32:
		switch x := v.(type) {
		case int16:
			out = int32(x)
		case int8:
			out = int32(x)
		case uint16:
			out = int32(x)
		}
	case int64:
		switch x := v.(type) {
		case int32:
			out = int64(x)
		case int16:
			out = int64(x)
		case int8:
			out = int64(x)
		case uint16:
			out = int64(x)
		}
	case float64:
		switch x := v.(type) {
		case int32:
			out = float64(x)
		case int64:
			out = float64(x)
		case float32:
			out = float64(x)
		case int16:
			out = float64(x)
		case int8:
			out = float64(x)
		case uint16:
			out = float64(x)
		}
	case float32:
		switch x := v.(type) {
		case int32:
			out = float32(x)
		case int64:
			out = float32(x)
		}
	}
	if out == nil {
		panic(fmt.Sprintf("unbox_: %T is not %T", v, z))
	}
	return out.(T)
}

// arrmap_ is the copy made when an array is used at another element type
// (Java array covariance, or a generic T[] seen as Object[]).
func arrmap_[S any, T any](src []S, f func(S) T) []T {
	if src == nil {
		return nil
	}
	out := make([]T, len(src))
	for i, v := range src {
		out[i] = f(v)
	}
	return out
}

// arrcopyOf_ is Arrays.copyOf(a, n) of a typed array (post-flip step arraycopy): a new array of
// length n holding a's first elements, the rest zero.
func arrcopyOf_[T any](a []T, n int32) []T {
	out := make([]T, n)
	copy(out, a)
	return out
}

func samearr_[T any](a, b []T) bool {
	if len(a) != len(b) {
		return false
	}
	if len(a) == 0 {
		return (a == nil) == (b == nil)
	}
	return &a[0] == &b[0]
}

func newstrarr_(n int32) []string {
	a := make([]string, n)
	for i := range a {
		a[i] = nullstr_
	}
	return a
}

// objequals_ is Objects.equals for values in the `any` world.
func objequals_(a, b any) bool {
	if a == nil || b == nil {
		return a == nil && b == nil
	}
	return Object_equals(a, b)
}

func objhash_(a any) int32 {
	if a == nil {
		return 0
	}
	return Object_hashCode(a)
}

var nan_ = math.NaN()
var posinf_ = math.Inf(1)
var neginf_ = math.Inf(-1)

var negzero_ = math.Copysign(0, -1)

func newarr2_[T any](n, m int32) [][]T {
	a := make([][]T, n)
	for i := range a {
		a[i] = make([]T, m)
		if s, ok := any(a[i]).([]string); ok {
			for j := range s {
				s[j] = nullstr_
			}
		}
	}
	return a
}
