package closure

// com.google.common.hash: Hashing.farmHashFingerprint64, Hashing.murmur3_128,
// Hasher and HashCode.

import (
	"encoding/binary"
	"math/bits"
)

type HashCode struct {
	bytes []byte // little-endian, as HashCode.asBytes()
}

func (this *HashCode) asInt() int32 { return int32(binary.LittleEndian.Uint32(this.bytes)) }

func (this *HashCode) asLong() int64 {
	if len(this.bytes) < 8 {
		stateFail_("HashCode#asLong() requires >= 8 bytes (it only has " + istr_(int64(len(this.bytes))) + " bytes).")
	}
	return int64(binary.LittleEndian.Uint64(this.bytes))
}

func (this *HashCode) equals(o any) bool {
	x, ok := o.(*HashCode)
	return ok && x != nil && string(x.bytes) == string(this.bytes)
}

func (this *HashCode) hashCode() int32 {
	if len(this.bytes) >= 4 {
		return this.asInt()
	}
	var v int32
	for i, b := range this.bytes {
		v |= int32(b) << (8 * uint(i))
	}
	return v
}

func (this *HashCode) toString() string {
	const hex = "0123456789abcdef"
	out := make([]byte, 0, 2*len(this.bytes))
	for _, b := range this.bytes {
		out = append(out, hex[b>>4], hex[b&15])
	}
	return string(out)
}

// bufferHasher_ collects the bytes put into it; both hash functions here hash
// the complete input at the end, which gives the same result as streaming.
type bufferHasher_ struct {
	buf    []byte
	finish func([]byte) *HashCode
}

func (this *bufferHasher_) putInt(i int32) PrimitiveSink {
	this.buf = binary.LittleEndian.AppendUint32(this.buf, uint32(i))
	return this
}

func (this *bufferHasher_) putLong(l int64) PrimitiveSink {
	this.buf = binary.LittleEndian.AppendUint64(this.buf, uint64(l))
	return this
}

func (this *bufferHasher_) putBytes(b []int8) PrimitiveSink {
	this.buf = append(this.buf, u8_(b)...)
	return this
}

func (this *bufferHasher_) putUnencodedChars(seq any) PrimitiveSink {
	s := csString_(seq)
	if allASCII_(s) {
		for i := 0; i < len(s); i++ {
			this.buf = append(this.buf, s[i], 0)
		}
		return this
	}
	for _, c := range String_toCharArray(s) {
		this.buf = append(this.buf, byte(c), byte(c>>8))
	}
	return this
}

func (this *bufferHasher_) hash() *HashCode { return this.finish(this.buf) }

type hashFunction_ struct {
	name   string
	finish func([]byte) *HashCode
}

func (this *hashFunction_) newHasher() Hasher {
	return &bufferHasher_{finish: this.finish}
}

func (this *hashFunction_) toString() string { return this.name }

var farmHashFingerprint64_ = &hashFunction_{"Hashing.farmHashFingerprint64()", func(b []byte) *HashCode {
	return &HashCode{binary.LittleEndian.AppendUint64(nil, farmFingerprint64_(b))}
}}

var murmur3_128_ = &hashFunction_{"Hashing.murmur3_128(0)", func(b []byte) *HashCode {
	h1, h2 := murmur3x64_128_(b, 0)
	out := binary.LittleEndian.AppendUint64(nil, h1)
	return &HashCode{binary.LittleEndian.AppendUint64(out, h2)}
}}

func Hashing_farmHashFingerprint64() HashFunction { return farmHashFingerprint64_ }

// ---- FarmHash Fingerprint64 (farmhashna::Hash64)

const (
	farmK0_ uint64 = 0xc3a5c85c97cb3127
	farmK1_ uint64 = 0xb492b66fbe98f273
	farmK2_ uint64 = 0x9ae16a3b2f90404f
)

func farmShiftMix_(v uint64) uint64 { return v ^ (v >> 47) }

func farmHashLen16_(u, v, mul uint64) uint64 {
	a := (u ^ v) * mul
	a ^= a >> 47
	b := (v ^ a) * mul
	b ^= b >> 47
	return b * mul
}

func farmWeak_(s []byte, a, b uint64) (uint64, uint64) {
	w := binary.LittleEndian.Uint64(s)
	x := binary.LittleEndian.Uint64(s[8:])
	y := binary.LittleEndian.Uint64(s[16:])
	z := binary.LittleEndian.Uint64(s[24:])
	a += w
	b = bits.RotateLeft64(b+a+z, -21)
	c := a
	a += x
	a += y
	b += bits.RotateLeft64(a, -44)
	return a + z, b + c
}

func farmFingerprint64_(s []byte) uint64 {
	f64 := func(i int) uint64 { return binary.LittleEndian.Uint64(s[i:]) }
	f32 := func(i int) uint64 { return uint64(binary.LittleEndian.Uint32(s[i:])) }
	rot := func(v uint64, n int) uint64 { return bits.RotateLeft64(v, -n) }
	n := len(s)
	ln := uint64(n)
	switch {
	case n <= 16:
		switch {
		case n >= 8:
			mul := farmK2_ + ln*2
			a := f64(0) + farmK2_
			b := f64(n - 8)
			c := rot(b, 37)*mul + a
			d := (rot(a, 25) + b) * mul
			return farmHashLen16_(c, d, mul)
		case n >= 4:
			mul := farmK2_ + ln*2
			a := f32(0)
			return farmHashLen16_(ln+(a<<3), f32(n-4), mul)
		case n > 0:
			a, b, c := uint32(s[0]), uint32(s[n>>1]), uint32(s[n-1])
			y := a + (b << 8)
			z := uint32(n) + (c << 2)
			return farmShiftMix_(uint64(y)*farmK2_^uint64(z)*farmK0_) * farmK2_
		}
		return farmK2_
	case n <= 32:
		mul := farmK2_ + ln*2
		a := f64(0) * farmK1_
		b := f64(8)
		c := f64(n-8) * mul
		d := f64(n-16) * farmK2_
		return farmHashLen16_(rot(a+b, 43)+rot(c, 30)+d, a+rot(b+farmK2_, 18)+c, mul)
	case n <= 64:
		mul := farmK2_ + ln*2
		a := f64(0) * farmK2_
		b := f64(8)
		c := f64(n-8) * mul
		d := f64(n-16) * farmK2_
		y := rot(a+b, 43) + rot(c, 30) + d
		z := farmHashLen16_(y, a+rot(b+farmK2_, 18)+c, mul)
		e := f64(16) * mul
		f := f64(24)
		g := (y + f64(n-32)) * mul
		h := (z + f64(n-24)) * mul
		return farmHashLen16_(rot(e+f, 43)+rot(g, 30)+h, e+rot(f+a, 18)+g, mul)
	}
	var seed uint64 = 81
	x := seed
	y := seed*farmK1_ + 113
	z := farmShiftMix_(y*farmK2_+113) * farmK2_
	var v1, v2, w1, w2 uint64
	x = x*farmK2_ + f64(0)
	end := ((n - 1) / 64) * 64
	last64 := end + ((n - 1) & 63) - 63
	p := 0
	for {
		x = rot(x+y+v1+f64(p+8), 37) * farmK1_
		y = rot(y+v2+f64(p+48), 42) * farmK1_
		x ^= w2
		y += v1 + f64(p+40)
		z = rot(z+w1, 33) * farmK1_
		v1, v2 = farmWeak_(s[p:], v2*farmK1_, x+w1)
		w1, w2 = farmWeak_(s[p+32:], z+w2, y+f64(p+16))
		z, x = x, z
		p += 64
		if p == end {
			break
		}
	}
	mul := farmK1_ + ((z & 0xff) << 1)
	p = last64
	w1 += uint64((n - 1) & 63)
	v1 += w1
	w1 += v1
	x = rot(x+y+v1+f64(p+8), 37) * mul
	y = rot(y+v2+f64(p+48), 42) * mul
	x ^= w2 * 9
	y += v1*9 + f64(p+40)
	z = rot(z+w1, 33) * mul
	v1, v2 = farmWeak_(s[p:], v2*mul, x+w1)
	w1, w2 = farmWeak_(s[p+32:], z+w2, y+f64(p+16))
	z, x = x, z
	return farmHashLen16_(farmHashLen16_(v1, w1, mul)+farmShiftMix_(y)*farmK0_+z, farmHashLen16_(v2, w2, mul)+x, mul)
}

// ---- MurmurHash3 x64 128

func murmurFmix64_(k uint64) uint64 {
	k ^= k >> 33
	k *= 0xff51afd7ed558ccd
	k ^= k >> 33
	k *= 0xc4ceb9fe1a85ec53
	k ^= k >> 33
	return k
}

func murmur3x64_128_(data []byte, seed uint32) (uint64, uint64) {
	const c1 uint64 = 0x87c37b91114253d5
	const c2 uint64 = 0x4cf5ad432745937f
	h1, h2 := uint64(seed), uint64(seed)
	n := len(data)
	i := 0
	for ; i+16 <= n; i += 16 {
		k1 := binary.LittleEndian.Uint64(data[i:])
		k2 := binary.LittleEndian.Uint64(data[i+8:])
		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
		h1 = bits.RotateLeft64(h1, 27)
		h1 += h2
		h1 = h1*5 + 0x52dce729
		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
		h2 = bits.RotateLeft64(h2, 31)
		h2 += h1
		h2 = h2*5 + 0x38495ab5
	}
	tail := data[i:]
	var k1, k2 uint64
	for j := len(tail) - 1; j >= 8; j-- {
		k2 ^= uint64(tail[j]) << (8 * uint(j-8))
	}
	if len(tail) > 8 {
		k2 *= c2
		k2 = bits.RotateLeft64(k2, 33)
		k2 *= c1
		h2 ^= k2
	}
	for j := min(len(tail), 8) - 1; j >= 0; j-- {
		k1 ^= uint64(tail[j]) << (8 * uint(j))
	}
	if len(tail) > 0 {
		k1 *= c1
		k1 = bits.RotateLeft64(k1, 31)
		k1 *= c2
		h1 ^= k1
	}
	h1 ^= uint64(n)
	h2 ^= uint64(n)
	h1 += h2
	h2 += h1
	h1 = murmurFmix64_(h1)
	h2 = murmurFmix64_(h2)
	h1 += h2
	h2 += h1
	return h1, h2
}

func Hashing_murmur3_128() HashFunction { return murmur3_128_ }
