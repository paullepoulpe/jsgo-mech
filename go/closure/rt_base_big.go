package closure

// java.math.BigInteger on math/big.

import (
	"math"
	"math/big"
)

type BigInteger struct {
	v *big.Int
}

var BigInteger_ZERO = &BigInteger{big.NewInt(0)}
var BigInteger_ONE = &BigInteger{big.NewInt(1)}
var BigInteger_TWO = &BigInteger{big.NewInt(2)}
var BigInteger_TEN = &BigInteger{big.NewInt(10)}

func BigInteger_valueOf(v int64) *BigInteger { return &BigInteger{big.NewInt(v)} }

func new_BigInteger(s string, radix int32) *BigInteger {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	if radix < 2 || radix > 36 {
		panic(new_NumberFormatException_String("Radix out of range").self())
	}
	digits := s
	if len(digits) > 0 && (digits[0] == '-' || digits[0] == '+') {
		digits = digits[1:]
	}
	if digits == "" {
		panic(new_NumberFormatException_String("Zero length BigInteger").self())
	}
	for i := 0; i < len(digits); i++ {
		c := digits[i]
		d := 99
		switch {
		case c >= '0' && c <= '9':
			d = int(c - '0')
		case c >= 'a' && c <= 'z':
			d = int(c-'a') + 10
		case c >= 'A' && c <= 'Z':
			d = int(c-'A') + 10
		}
		if d >= int(radix) {
			if c == '-' || c == '+' {
				panic(new_NumberFormatException_String("Illegal embedded sign character").self())
			}
			panic(numberFormat_(s))
		}
	}
	v, ok := new(big.Int).SetString(s, int(radix))
	if !ok {
		panic(numberFormat_(s))
	}
	return &BigInteger{v}
}

func new_BigInteger_String(s string) *BigInteger { return new_BigInteger(s, 10) }

// new_BigInteger_byteArr reads a big-endian two's-complement number.
func new_BigInteger_byteArr(b []int8) *BigInteger {
	if len(b) == 0 {
		panic(new_NumberFormatException_String("Zero length BigInteger").self())
	}
	raw := make([]byte, len(b))
	for i, x := range b {
		raw[i] = byte(x)
	}
	v := new(big.Int).SetBytes(raw)
	if b[0] < 0 {
		v.Sub(v, new(big.Int).Lsh(big.NewInt(1), uint(8*len(b))))
	}
	return &BigInteger{v}
}

func (this *BigInteger) add(o *BigInteger) *BigInteger {
	return &BigInteger{new(big.Int).Add(this.v, o.v)}
}

func (this *BigInteger) subtract(o *BigInteger) *BigInteger {
	return &BigInteger{new(big.Int).Sub(this.v, o.v)}
}

func (this *BigInteger) multiply(o *BigInteger) *BigInteger {
	return &BigInteger{new(big.Int).Mul(this.v, o.v)}
}

func (this *BigInteger) and(o *BigInteger) *BigInteger {
	return &BigInteger{new(big.Int).And(this.v, o.v)}
}

func (this *BigInteger) or(o *BigInteger) *BigInteger {
	return &BigInteger{new(big.Int).Or(this.v, o.v)}
}

func (this *BigInteger) xor(o *BigInteger) *BigInteger {
	return &BigInteger{new(big.Int).Xor(this.v, o.v)}
}

func (this *BigInteger) not() *BigInteger { return &BigInteger{new(big.Int).Not(this.v)} }

func (this *BigInteger) negate() *BigInteger { return &BigInteger{new(big.Int).Neg(this.v)} }

func (this *BigInteger) abs() *BigInteger { return &BigInteger{new(big.Int).Abs(this.v)} }

func (this *BigInteger) divide(o *BigInteger) *BigInteger {
	if o.v.Sign() == 0 {
		panic(new_ArithmeticException_String("BigInteger divide by zero").self())
	}
	return &BigInteger{new(big.Int).Quo(this.v, o.v)}
}

func (this *BigInteger) remainder(o *BigInteger) *BigInteger {
	if o.v.Sign() == 0 {
		panic(new_ArithmeticException_String("BigInteger divide by zero").self())
	}
	return &BigInteger{new(big.Int).Rem(this.v, o.v)}
}

func (this *BigInteger) mod(o *BigInteger) *BigInteger {
	if o.v.Sign() <= 0 {
		panic(new_ArithmeticException_String("BigInteger: modulus not positive").self())
	}
	return &BigInteger{new(big.Int).Mod(this.v, o.v)}
}

func (this *BigInteger) divideAndRemainder(o *BigInteger) []*BigInteger {
	if o.v.Sign() == 0 {
		panic(new_ArithmeticException_String("BigInteger divide by zero").self())
	}
	q, r := new(big.Int).QuoRem(this.v, o.v, new(big.Int))
	return []*BigInteger{{q}, {r}}
}

func (this *BigInteger) pow(e int32) *BigInteger {
	if e < 0 {
		panic(new_ArithmeticException_String("Negative exponent").self())
	}
	return &BigInteger{new(big.Int).Exp(this.v, big.NewInt(int64(e)), nil)}
}

func (this *BigInteger) shiftLeft(n int32) *BigInteger {
	if n < 0 {
		return &BigInteger{new(big.Int).Rsh(this.v, uint(-int64(n)))}
	}
	return &BigInteger{new(big.Int).Lsh(this.v, uint(n))}
}

func (this *BigInteger) shiftRight(n int32) *BigInteger {
	if n < 0 {
		return &BigInteger{new(big.Int).Lsh(this.v, uint(-int64(n)))}
	}
	return &BigInteger{new(big.Int).Rsh(this.v, uint(n))}
}

func (this *BigInteger) signum() int32 { return int32(this.v.Sign()) }

// bitLength is the number of bits of the minimal two's-complement form, without a sign bit.
func (this *BigInteger) bitLength() int32 {
	if this.v.Sign() < 0 {
		return int32(new(big.Int).Not(this.v).BitLen())
	}
	return int32(this.v.BitLen())
}

func (this *BigInteger) compareTo(o any) int32 { return int32(this.v.Cmp(o.(*BigInteger).v)) }

func (this *BigInteger) equals(o any) bool {
	x, ok := o.(*BigInteger)
	return ok && x != nil && this.v.Cmp(x.v) == 0
}

func (this *BigInteger) hashCode() int32 {
	var h int32
	mag := new(big.Int).Abs(this.v).Bytes()
	// 32-bit words of the magnitude, most significant first.
	for len(mag)%4 != 0 {
		mag = append([]byte{0}, mag...)
	}
	for i := 0; i < len(mag); i += 4 {
		w := int32(uint32(mag[i])<<24 | uint32(mag[i+1])<<16 | uint32(mag[i+2])<<8 | uint32(mag[i+3]))
		h = 31*h + w
	}
	return h * int32(this.v.Sign())
}

func (this *BigInteger) toString() string { return this.v.String() }

func (this *BigInteger) toString_int(radix int32) string {
	if radix < 2 || radix > 36 {
		radix = 10
	}
	return this.v.Text(int(radix))
}

func (this *BigInteger) longValue() int64 {
	// The low 64 bits of the two's-complement form.
	m := new(big.Int).And(this.v, new(big.Int).SetUint64(math.MaxUint64))
	return int64(m.Uint64())
}

func (this *BigInteger) intValue() int32 { return int32(this.longValue()) }

func (this *BigInteger) intValueExact() int32 {
	if !this.v.IsInt64() || this.v.Int64() > math.MaxInt32 || this.v.Int64() < math.MinInt32 {
		panic(new_ArithmeticException_String("BigInteger out of int range").self())
	}
	return int32(this.v.Int64())
}

func (this *BigInteger) longValueExact() int64 {
	if !this.v.IsInt64() {
		panic(new_ArithmeticException_String("BigInteger out of long range").self())
	}
	return this.v.Int64()
}

func (this *BigInteger) doubleValue() float64 {
	f, _ := new(big.Float).SetPrec(uint(this.v.BitLen()) + 64).SetInt(this.v).Float64()
	return f
}

// toByteArray is the minimal big-endian two's-complement form.
func (this *BigInteger) toByteArray() []int8 {
	n := int(this.bitLength())/8 + 1
	v := this.v
	if v.Sign() < 0 {
		v = new(big.Int).Add(v, new(big.Int).Lsh(big.NewInt(1), uint(8*n)))
	}
	raw := v.FillBytes(make([]byte, n))
	out := make([]int8, n)
	for i, b := range raw {
		out[i] = int8(b)
	}
	return out
}
