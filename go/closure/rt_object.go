package closure

import (
	"fmt"
	"math"
	"reflect"
	"strings"
)

// ---- java.lang.Object, for values in the `any` world

// goTypeName_ is the Go type name of an object's class ("Node" for *Node).
func goTypeName_(v any) string {
	t := reflect.TypeOf(v)
	for t.Kind() == reflect.Pointer {
		t = t.Elem()
	}
	return t.Name()
}

func Object_getClass(v any) *Class {
	switch v.(type) {
	case string:
		return classOf_("java.lang.String")
	case int32:
		return classOf_("java.lang.Integer")
	case int64:
		return classOf_("java.lang.Long")
	case bool:
		return classOf_("java.lang.Boolean")
	case float64:
		return classOf_("java.lang.Double")
	case uint16:
		return classOf_("java.lang.Character")
	}
	n := goTypeName_(v)
	if j, ok := javaNames_[n]; ok {
		return classOf_(j)
	}
	return classOf_(n)
}

func Object_toString(v any) string {
	switch x := v.(type) {
	case nil:
		panic(new_NullPointerException_0().self())
	case string:
		return x
	case int32:
		return istr_(int64(x))
	case int64:
		return istr_(x)
	case int16:
		return istr_(int64(x))
	case int8:
		return istr_(int64(x))
	case bool:
		return boolstr_(x)
	case uint16:
		return chrstr_(x)
	case float64:
		return dstr_(x)
	case float32:
		return fstr_(x)
	case interface{ toString() string }:
		return x.toString()
	}
	return Object_getClass(v).getName() + "@" + fmt.Sprintf("%x", Object_hashCode(v))
}

func Object_equals(a, b any) bool {
	switch x := a.(type) {
	case nil:
		panic(new_NullPointerException_0().self())
	case interface{ equals(any) bool }:
		return x.equals(b)
	case float64:
		y, ok := b.(float64)
		return ok && (x == y || x != x && y != y)
	}
	if b == nil {
		return false
	}
	if !reflect.TypeOf(a).Comparable() || !reflect.TypeOf(b).Comparable() {
		return false
	}
	return a == b
}

var identityHashes_ = map[any]int32{}
var nextIdentityHash_ int32 = 0x1000

func identityHash_(v any) int32 {
	if !reflect.TypeOf(v).Comparable() {
		return 1
	}
	h, ok := identityHashes_[v]
	if !ok {
		nextIdentityHash_ += 0x9e3779
		h = nextIdentityHash_ & 0x7fffffff
		identityHashes_[v] = h
	}
	return h
}

func Object_hashCode(v any) int32 {
	switch x := v.(type) {
	case nil:
		panic(new_NullPointerException_0().self())
	case string:
		return String_hashCode(x)
	case int32:
		return x
	case int64:
		return int32(x ^ int64(uint64(x)>>32))
	case int16:
		return int32(x)
	case int8:
		return int32(x)
	case uint16:
		return int32(x)
	case bool:
		if x {
			return 1231
		}
		return 1237
	case float64:
		b := math.Float64bits(x)
		return int32(b ^ (b >> 32))
	case interface{ hashCode() int32 }:
		return x.hashCode()
	}
	return identityHash_(v)
}

func Object_clone(v any) any { panic("Object.clone") }

// ---- java.lang.Class

type Class struct {
	name_ string
}

func (this *Class) getName() string { return this.name_ }

func (this *Class) getSimpleName() string {
	n := this.name_
	if i := strings.LastIndexAny(n, ".$"); i >= 0 {
		n = n[i+1:]
	}
	return n
}

func (this *Class) toString() string { return "class " + this.name_ }

// ---- java.lang.Enum

type Enum struct {
	self_    any
	name_    string
	ordinal_ int32
}

func (this *Enum) init_Enum(name string, ordinal int32) {
	this.name_ = name
	this.ordinal_ = ordinal
}

func (this *Enum) name() string     { return this.name_ }
func (this *Enum) ordinal() int32   { return this.ordinal_ }
func (this *Enum) toString() string { return this.name_ }

func (this *Enum) compareTo(o any) int32 {
	return this.ordinal_ - o.(interface{ ordinal() int32 }).ordinal()
}

func (this *Enum) getDeclaringClass() *Class { return Object_getClass(this.self_) }

// ---- java.lang.Throwable

type Throwable struct {
	self_    any
	message_ string
	cause_   *Throwable
}

func (this *Throwable) init_Throwable_0() { this.message_ = nullstr_ }

func (this *Throwable) init_Throwable_String(m string) { this.message_ = m }

func (this *Throwable) init_Throwable_String_Throwable(m string, c *Throwable) {
	this.message_ = m
	this.cause_ = c
}

func (this *Throwable) init_Throwable_Throwable(c *Throwable) {
	this.cause_ = c
	this.message_ = nullstr_
	if c != nil {
		this.message_ = c.toString()
	}
}

func (this *Throwable) getMessage() string          { return this.message_ }
func (this *Throwable) getLocalizedMessage() string { return this.message_ }
func (this *Throwable) getCause() *Throwable        { return this.cause_ }

func (this *Throwable) toString() string {
	n := Object_getClass(this.self_).getName()
	m := this.self_.(interface{ getMessage() string }).getMessage()
	if m == nullstr_ {
		return n
	}
	return n + ": " + m
}

// Error makes a Java exception that escapes as a Go panic print usefully.
func (this *Throwable) Error() string { return this.toString() }
