package closure

// java.lang numbers, Math, Objects, System.

import (
	"math"
	"math/bits"
	"os"
	"reflect"
	"strconv"
	"strings"
	"time"
)

func numberFormat_(s string) any {
	return new_NumberFormatException_String("For input string: \"" + s + "\"").self()
}

func Integer_parseInt_String(s string) int32 { return Integer_parseInt(s, 10) }

func Integer_parseInt(s string, radix int32) int32 {
	if s == nullstr_ || s == "" || s == "+" || s == "-" || strings.ContainsAny(s, "_ ") {
		panic(numberFormat_(s))
	}
	v, err := strconv.ParseInt(s, int(radix), 32)
	if err != nil {
		panic(numberFormat_(s))
	}
	return int32(v)
}

func Integer_valueOf(i int32) any                 { return i }
func Integer_intValue(v any) int32                { return unbox_[int32](v) }
func Integer_toString(i int32) string             { return strconv.FormatInt(int64(i), 10) }
func Integer_toString_int_int(i, r int32) string  { return strconv.FormatInt(int64(i), int(r)) }
func Integer_toHexString(i int32) string          { return strconv.FormatUint(uint64(uint32(i)), 16) }
func Integer_toUnsignedLong(i int32) int64        { return int64(uint32(i)) }
func Integer_bitCount(i int32) int32              { return int32(bits.OnesCount32(uint32(i))) }
func Integer_lowestOneBit(i int32) int32          { return i & -i }
func Integer_reverse(i int32) int32               { return int32(bits.Reverse32(uint32(i))) }
func Integer_numberOfLeadingZeros(i int32) int32  { return int32(bits.LeadingZeros32(uint32(i))) }
func Integer_numberOfTrailingZeros(i int32) int32 { return int32(bits.TrailingZeros32(uint32(i))) }

func Integer_compare(a, b int32) int32 {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func Long_parseLong(s string, radix int32) int64 {
	if s == nullstr_ || s == "" || strings.ContainsAny(s, "_ ") {
		panic(numberFormat_(s))
	}
	v, err := strconv.ParseInt(s, int(radix), 64)
	if err != nil {
		panic(numberFormat_(s))
	}
	return v
}

func Long_toString(i int64) string                   { return strconv.FormatInt(i, 10) }
func Long_toString_long_int(i int64, r int32) string { return strconv.FormatInt(i, int(r)) }
func Long_toHexString(i int64) string                { return strconv.FormatUint(uint64(i), 16) }
func Long_bitCount(i int64) int32                    { return int32(bits.OnesCount64(uint64(i))) }
func Long_numberOfTrailingZeros(i int64) int32       { return int32(bits.TrailingZeros64(uint64(i))) }

func Long_compare(a, b int64) int32 {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	return 0
}

func Byte_toUnsignedInt(b int8) int32 { return int32(uint8(b)) }

func Boolean_valueOf(b bool) any      { return b }
func Boolean_booleanValue(v any) bool { return v.(bool) }

func Number_doubleValue(v any) float64 { return unbox_[float64](v) }

// ---- Double

// javaFloat_ formats in the style of Double.toString from the shortest digits
// that identify the value.
func javaFloat_(d float64, bitSize int) string {
	switch {
	case d != d:
		return "NaN"
	case math.IsInf(d, 1):
		return "Infinity"
	case math.IsInf(d, -1):
		return "-Infinity"
	case d == 0:
		if math.Signbit(d) {
			return "-0.0"
		}
		return "0.0"
	}
	if bitSize == 64 && math.Abs(d) == 4.9e-324 {
		// Double.MIN_VALUE: Java keeps two digits here.
		if d < 0 {
			return "-4.9E-324"
		}
		return "4.9E-324"
	}
	e := strconv.FormatFloat(d, 'e', -1, bitSize) // d.ddddde±xx
	neg := e[0] == '-'
	if neg {
		e = e[1:]
	}
	mant, expS, _ := strings.Cut(e, "e")
	exp, _ := strconv.Atoi(expS)
	digits := strings.Replace(mant, ".", "", 1)
	var out string
	if exp >= -3 && exp < 7 {
		if exp >= 0 {
			for len(digits) < exp+2 {
				digits += "0"
			}
			out = digits[:exp+1] + "." + digits[exp+1:]
		} else {
			out = "0." + strings.Repeat("0", -exp-1) + digits
		}
	} else {
		frac := digits[1:]
		if frac == "" {
			frac = "0"
		}
		out = digits[:1] + "." + frac + "E" + strconv.Itoa(exp)
	}
	if neg {
		return "-" + out
	}
	return out
}

func Double_toString(d float64) string { return javaFloat_(d, 64) }

func Float_toString(f float32) string { return javaFloat_(float64(f), 32) }

func Double_valueOf(d float64) any               { return d }
func Double_doubleValue(v any) float64           { return unbox_[float64](v) }
func Double_intValue(v any) int32                { return f2i_[int32](v.(float64)) }
func Double_longValue(v any) int64               { return f2i_[int64](v.(float64)) }
func Double_isNaN(d float64) bool                { return d != d }
func Double_isNaN_0(v any) bool                  { d := v.(float64); return d != d }
func Double_isInfinite(d float64) bool           { return math.IsInf(d, 0) }
func Double_isInfinite_0(v any) bool             { return math.IsInf(v.(float64), 0) }
func Double_isFinite(d float64) bool             { return !math.IsInf(d, 0) && d == d }
func Double_doubleToRawLongBits(d float64) int64 { return int64(math.Float64bits(d)) }
func Double_longBitsToDouble(l int64) float64    { return math.Float64frombits(uint64(l)) }

func Double_doubleToLongBits(d float64) int64 {
	if d != d {
		return 0x7ff8000000000000
	}
	return int64(math.Float64bits(d))
}

func Double_compare(a, b float64) int32 {
	if a < b {
		return -1
	}
	if a > b {
		return 1
	}
	x, y := Double_doubleToLongBits(a), Double_doubleToLongBits(b)
	if x == y {
		return 0
	}
	if x < y {
		return -1
	}
	return 1
}

func Double_parseDouble(s string) float64 {
	t := String_trim(s)
	if t == "" || s == nullstr_ {
		panic(numberFormat_(s))
	}
	switch t {
	case "NaN", "+NaN", "-NaN":
		return math.NaN()
	case "Infinity", "+Infinity":
		return math.Inf(1)
	case "-Infinity":
		return math.Inf(-1)
	}
	u := t
	if n := len(u); n > 0 && strings.ContainsRune("dDfF", rune(u[n-1])) && !strings.HasPrefix(strings.TrimLeft(u, "+-"), "0x") && !strings.HasPrefix(strings.TrimLeft(u, "+-"), "0X") {
		u = u[:n-1]
	}
	low := strings.ToLower(u)
	if strings.Contains(low, "inf") || strings.Contains(low, "nan") || strings.Contains(u, "_") {
		panic(numberFormat_(s))
	}
	v, err := strconv.ParseFloat(u, 64)
	if err != nil {
		if ne, ok := err.(*strconv.NumError); ok && ne.Err == strconv.ErrRange {
			return v
		}
		panic(numberFormat_(s))
	}
	return v
}

// ---- Math

func Math_abs(d float64) float64    { return math.Abs(d) }
func Math_cbrt(d float64) float64   { return math.Cbrt(d) }
func Math_ceil(d float64) float64   { return math.Ceil(d) }
func Math_floor(d float64) float64  { return math.Floor(d) }
func Math_pow(a, b float64) float64 { return math.Pow(a, b) }

func Math_abs_int(i int32) int32 {
	if i < 0 {
		return -i
	}
	return i
}

func Math_max(a, b int32) int32 {
	if a > b {
		return a
	}
	return b
}

func Math_min(a, b int32) int32 {
	if a < b {
		return a
	}
	return b
}

func Math_max_long_long(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}

func Math_min_long_long(a, b int64) int64 {
	if a < b {
		return a
	}
	return b
}

func Math_max_double_double(a, b float64) float64 {
	if a != a || b != b {
		return math.NaN()
	}
	return math.Max(a, b)
}

func Math_min_double_double(a, b float64) float64 {
	if a != a || b != b {
		return math.NaN()
	}
	return math.Min(a, b)
}

func Math_round(d float64) int64 { return f2i_[int64](math.Floor(d + 0.5)) }

func Math_signum(d float64) float64 {
	if d > 0 {
		return 1
	}
	if d < 0 {
		return -1
	}
	return d
}

func Math_toIntExact(l int64) int32 {
	if int64(int32(l)) != l {
		panic(new_ArithmeticException_String("integer overflow").self())
	}
	return int32(l)
}

// ---- Objects

func Objects_equals(a, b any) bool { return objequals_(a, b) }
func Objects_hashCode(a any) int32 { return objhash_(a) }

func Objects_hash(vs []any) int32 {
	if vs == nil {
		return 0
	}
	var h int32 = 1
	for _, v := range vs {
		h = 31*h + objhash_(v)
	}
	return h
}

func Objects_requireNonNull(v any, msg string) any {
	if v == nil {
		panic(new_NullPointerException_String(msg).self())
	}
	return v
}

// ---- System

var startTime_ = time.Now()

func System_currentTimeMillis() int64 { return time.Now().UnixMilli() }
func System_exit(code int32)          { flushStd_(); os.Exit(int(code)) }
func System_identityHashCode(v any) int32 {
	if v == nil {
		return 0
	}
	return identityHash_(v)
}

var systemProperties_ = map[string]string{
	"line.separator": "\n",
	"file.separator": "/",
	"path.separator": ":",
	"user.dir":       func() string { d, _ := os.Getwd(); return d }(),
	"java.version":   "21",
	"os.name":        "Linux",
}

func System_getProperty(k string, def string) string {
	if v, ok := systemProperties_[k]; ok {
		return v
	}
	return def
}

func System_arraycopy(src any, sp int32, dst any, dp int32, n int32) {
	switch s := src.(type) {
	case []int32:
		copy(dst.([]int32)[dp:dp+n], s[sp:sp+n])
	case []uint16:
		copy(dst.([]uint16)[dp:dp+n], s[sp:sp+n])
	case []int8:
		copy(dst.([]int8)[dp:dp+n], s[sp:sp+n])
	case []any:
		copy(dst.([]any)[dp:dp+n], s[sp:sp+n])
	case []string:
		copy(dst.([]string)[dp:dp+n], s[sp:sp+n])
	case []int64:
		copy(dst.([]int64)[dp:dp+n], s[sp:sp+n])
	case []float64:
		copy(dst.([]float64)[dp:dp+n], s[sp:sp+n])
	case []bool:
		copy(dst.([]bool)[dp:dp+n], s[sp:sp+n])
	default:
		reflect.Copy(reflect.ValueOf(dst).Slice(int(dp), int(dp+n)), reflect.ValueOf(src).Slice(int(sp), int(sp+n)))
	}
}

func Integer_parseInt_CharSequence_int_int_int(cs any, a int32, b int32, radix int32) int32 {
	return Integer_parseInt(String_substring(csString_(cs), a, b), radix)
}

func Long_parseLong_String(s string) int64 { return Long_parseLong(s, 10) }
