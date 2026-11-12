package closure

// Core runtime of the translated code: conversions between static Go types and
// the dynamic `any` world that erased Java generics live in.

import (
	"fmt"
	"math"
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

func isCharSequence_(v any) bool {
	switch v.(type) {
	case string, *StringBuilder:
		return true
	}
	return false
}

func isComparable_(v any) bool {
	switch v.(type) {
	case string, int32, int64, int16, int8, uint16, float32, float64, bool:
		return true
	}
	_, ok := v.(interface{ compareTo(any) int32 })
	return ok
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

func fstr_(f float32) string { return Double_toString(float64(f)) }

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

// arrconv_ copies an array to one with another element type (Java array covariance).
func arrconv_[T any, S any](src []S) []T {
	if src == nil {
		return nil
	}
	out := make([]T, len(src))
	for i, v := range src {
		var a any = v
		if s, ok := a.(string); ok {
			a = strobj_(s)
		}
		if a == nil {
			var z T
			if _, isStr := any(z).(string); isStr {
				a = nullstr_
			} else {
				continue
			}
		}
		out[i] = a.(T)
	}
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
