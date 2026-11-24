//go:build !j2go_nofntypes

package closure

import (
	"strconv"
	"unsafe"
)

// Rule "function types" (tools/j2go, FnTypes): a library functional interface is a Go func
// type of its Java type arguments (Predicate[T], Function[T, R]; declared in g_zstubs.go with
// the interface's method). This file has the runtime's functions that take or give such a
// function at its Java types. Everything else in the runtime takes functions at `any`
// (Function_e, Predicate_e, ...) and is the same code for both translations; rt_fn_erased.go
// has this file's functions for the translation without the rule.

// filter is the Optional if its value satisfies p, else the empty one.
func (this Optional[T]) filter(p Predicate[T]) Optional[T] {
	this.nn_()
	if p == nil {
		panic(new_NullPointerException_0().self())
	}
	if !this.present || p(this.value) {
		return this
	}
	return Optional[T]{set: true}
}

func (this Optional[T]) ifPresent(c Consumer[T]) {
	this.nn_()
	if this.present {
		c(this.value)
	}
}

// ---- Predicates, Functions, Suppliers

func Predicates_alwaysTrue[T any]() Predicate_base[T] { return alwaysTrueT_[T] }

func alwaysTrueT_[T any](T) bool { return true }

// Predicates_equalTo is Guava's: null only equals null, any other target decides by its equals.
func Predicates_equalTo[T any](target T) Predicate_base[T] {
	t := boxT_(target)
	if t == nil {
		return func(v T) bool { return boxT_(v) == nil }
	}
	return func(v T) bool { return Object_equals(t, boxT_(v)) }
}

func Functions_identity[E any]() Function_base[E, E] { return identityT_[E] }

func identityT_[E any](v E) E { return v }

// Suppliers_memoize calls s once, the first time the result is asked for, and then lets s go.
func Suppliers_memoize[T any](s Supplier_base[T]) Supplier_base[T] {
	if s == nil {
		panic(new_NullPointerException_0().self())
	}
	var done bool
	var value T
	return func() T {
		if !done {
			v := s()
			value = v
			done = true
			s = nil
		}
		return value
	}
}

// asFunction is the escaper as a function of strings.
func (this *Escaper) asFunction() Function_base[string, string] {
	return this.escape
}

// ---- a function as an Object

// fnref_ is what a function of type F is as an Object: the pointer that the Go func value is
// (a func value is one pointer, to its closure), at a type of its own. So the box costs no
// allocation, is comparable like every other object (two boxes are == exactly when they box
// the same closure, which is the identity a Java lambda object has), and unboxes only as an F.
type fnref_[F any] struct{ _ uintptr }

// fnBox_ is function f as an Object: nil for null.
func fnBox_[F any](f F) any {
	p := *(*unsafe.Pointer)(unsafe.Pointer(&f))
	if p == nil {
		return nil
	}
	return (*fnref_[F])(p)
}

// fnUnbox_ is the function of type F that Object v is (null for nil; any other object fails as
// a Java cast does).
func fnUnbox_[F any](v any) F {
	var f F
	if v == nil {
		return f
	}
	p := v.(*fnref_[F])
	*(*unsafe.Pointer)(unsafe.Pointer(&f)) = unsafe.Pointer(p)
	return f
}

// fnSame_ is a == b of two functions as Java objects: the same closure, or both null.
func fnSame_[F any](a, b F) bool {
	return *(*unsafe.Pointer)(unsafe.Pointer(&a)) == *(*unsafe.Pointer)(unsafe.Pointer(&b))
}

// fnHash_ is Objects.hashCode of a function: 0 for null, else a number of its closure (closures
// do not move).
func fnHash_[F any](f F) int32 {
	p := uintptr(*(*unsafe.Pointer)(unsafe.Pointer(&f)))
	return int32(uint32(p>>3) ^ uint32(p>>35))
}

// fnStr_ is String.valueOf of a function (Java prints a lambda's synthetic class and identity
// hash, which no two runs share).
func fnStr_[F any](f F) string {
	if *(*unsafe.Pointer)(unsafe.Pointer(&f)) == nil {
		return "null"
	}
	return "lambda@" + strconv.FormatInt(int64(uint32(fnHash_(f))), 16)
}

// ---- Comparator

func Comparator_naturalOrder[T any]() Comparator[T] {
	return func(a, b T) int32 {
		if r, ok := cmpTypedT_(a, b); ok {
			return r
		}
		return compareNatural_(boxT_(a), boxT_(b))
	}
}

func Comparator_comparing[T, U any](f Function[T, U]) Comparator[T] {
	return func(a, b T) int32 {
		ka, kb := f(a), f(b)
		if r, ok := cmpTypedT_(ka, kb); ok {
			return r
		}
		return compareNatural_(boxT_(ka), boxT_(kb))
	}
}

func Comparator_comparing_Function_Comparator[T, U any](f Function[T, U], c Comparator[U]) Comparator[T] {
	return func(a, b T) int32 { return c(f(a), f(b)) }
}

func Comparator_comparingInt[T any](f ToIntFunction[T]) Comparator[T] {
	return func(a, b T) int32 { return Integer_compare(f(a), f(b)) }
}

func Comparator_comparingLong[T any](f ToLongFunction[T]) Comparator[T] {
	return func(a, b T) int32 { return Long_compare(f(a), f(b)) }
}

// The default methods of Comparator that the program calls: methods of the func type.

func (c Comparator[T]) reversed() Comparator[T] {
	return func(a, b T) int32 { return c(b, a) }
}

func (c Comparator[T]) thenComparing_Comparator(d Comparator[T]) Comparator[T] {
	return func(a, b T) int32 {
		if r := c(a, b); r != 0 {
			return r
		}
		return d(a, b)
	}
}

// thenComparing compares the keys by their natural order (the key type is a type variable of
// the Java method, which a Go method cannot have: the keys are Objects).
func (c Comparator[T]) thenComparing(f Function[T, any]) Comparator[T] {
	return c.thenComparing_Comparator(func(a, b T) int32 { return compareNatural_(f(a), f(b)) })
}

func (c Comparator[T]) thenComparingInt(f ToIntFunction[T]) Comparator[T] {
	return c.thenComparing_Comparator(Comparator_comparingInt(f))
}

func (c Comparator[T]) thenComparingLong(f ToLongFunction[T]) Comparator[T] {
	return c.thenComparing_Comparator(Comparator_comparingLong(f))
}

func Map_Entry_comparingByKey() Comparator[Map_Entry] {
	return func(a, b Map_Entry) int32 { return compareNatural_(a.getKey(), b.getKey()) }
}
