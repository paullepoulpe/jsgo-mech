package closure

// Values of type parameters. j2go translates the generic classes that GenScan finds typable as Go
// generic types (C<X> is *C[X]); a value of a type parameter T is Java's value of type X itself
// (a string, a class pointer, a handle, an interface value) and meets Object, an erased slot, the
// way the erased code boxed it.

import "reflect"

// null_ is Java null as a value of type parameter T: the null string for a String, the zero value
// otherwise (nil for a class or an interface, the zero handle).
func null_[T any]() T {
	var z T
	if p, ok := any(&z).(*string); ok {
		*p = nullstr_
	}
	return z
}

// isnull_ is `v == null` for a value of a type parameter: the null string, a nil pointer, interface
// or slice, the zero handle.
func isnull_[T any](v T) bool {
	switch p := any(&v).(type) {
	case *string:
		return *p == nullstr_
	case *Node:
		return *p == 0
	}
	x := any(v)
	if x == nil {
		return true
	}
	switch r := reflect.ValueOf(x); r.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Func:
		return r.IsNil()
	}
	return false
}

// boxT_ is a value of a type parameter boxed as the erased code boxes a value of its Java type:
// strobj_ for a String, box_() for a handle, the most derived object (self()) for a class, and
// null as nil.
func boxT_[T any](v T) any {
	switch p := any(&v).(type) {
	case *string:
		return strobj_(*p)
	case *Node:
		return p.box_()
	case *any:
		return *p
	}
	x := any(v)
	if x == nil {
		return nil
	}
	if s, ok := x.(interface{ self() any }); ok {
		return s.self()
	}
	switch r := reflect.ValueOf(x); r.Kind() {
	case reflect.Pointer, reflect.Map, reflect.Func:
		if r.IsNil() {
			return nil
		}
	case reflect.Slice:
		if r.IsNil() {
			return nil
		}
	}
	return x
}

// unboxT_ is an Object as a value of type parameter T, as the erased code's cast of it to T's Java
// type: nil is Java null; a most derived object gives its view of the class T points to (to_X); a
// value of another type is the ClassCastException of the cast.
func unboxT_[T any](v any) T {
	if t, ok := v.(T); ok {
		return t
	}
	if v == nil {
		return null_[T]()
	}
	want := reflect.TypeFor[T]()
	if want == reflect.TypeFor[Node]() {
		return any(Node_of_(v)).(T)
	}
	if want.Kind() == reflect.Pointer {
		if p := embeddedPtr_(reflect.ValueOf(v), want.Elem()); p.IsValid() {
			return p.Interface().(T)
		}
	}
	return v.(T) // the ClassCastException of the erased cast
}

// embeddedPtr_ is the pointer to the struct of type want that the object v points to embeds (the
// view of a superclass), or an invalid value.
func embeddedPtr_(v reflect.Value, want reflect.Type) reflect.Value {
	if v.Kind() != reflect.Pointer || v.IsNil() || v.Elem().Kind() != reflect.Struct {
		return reflect.Value{}
	}
	s := v.Elem()
	for i := 0; i < s.NumField(); i++ {
		f := s.Type().Field(i)
		if !f.Anonymous || f.Type.Kind() != reflect.Struct {
			continue
		}
		if f.Type == want {
			return s.Field(i).Addr()
		}
		if p := embeddedPtr_(s.Field(i).Addr(), want); p.IsValid() {
			return p
		}
	}
	return reflect.Value{}
}
