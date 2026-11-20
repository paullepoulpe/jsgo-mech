package closure

// Values of type parameters. j2go translates the generic classes that GenScan finds typable as Go
// generic types (C<X> is *C[X]); a value of a type parameter T is Java's value of type X itself
// (a string, a class pointer, a handle, an interface value) and meets Object, an erased slot, the
// way the erased code boxed it.

import (
	"reflect"
	"sync"
	"sync/atomic"
	"unsafe"
)

// jnull_ is Java null as a value of type parameter T: the null string for a String, the zero value
// otherwise (nil for a class or an interface, the zero handle).
func jnull_[T any]() T {
	var z T
	if p, ok := any(&z).(*string); ok {
		*p = nullstr_
	}
	return z
}

// jisnull_ is `v == null` for a value of a type parameter: the null string, a nil pointer, interface
// or slice, the zero handle.
func jisnull_[T any](v T) bool {
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
	if h, ok := x.(interface{ box_() any }); ok {
		// another handle class: its zero handle boxes as nil
		return h.box_() == nil
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
	if x == nil || (*eface_)(unsafe.Pointer(&x)).data == nil {
		// null: a nil interface, or a nil pointer (whose self() a superclass would promote and
		// dereference); a pointer-shaped value is the interface's data word itself
		return nil
	}
	if s, ok := x.(interface{ self() any }); ok {
		return s.self()
	}
	if h, ok := x.(interface{ box_() any }); ok {
		// another handle class: its own box (nil for the zero handle)
		return h.box_()
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

// junboxT_ is an Object as a value of type parameter T, as the erased code's cast of it to T's Java
// type: nil is Java null; a most derived object gives its view of the class T points to (to_X); a
// value of another type is the ClassCastException of the cast.
func junboxT_[T any](v any) T {
	if t, ok := v.(T); ok {
		return t
	}
	if v == nil {
		return jnull_[T]()
	}
	want := reflect.TypeFor[T]()
	if want == reflect.TypeFor[Node]() {
		return any(Node_of_(v)).(T)
	}
	if want.Kind() == reflect.Pointer {
		if p := jembeddedPtr_(reflect.ValueOf(v), want.Elem()); p.IsValid() {
			return p.Interface().(T)
		}
	}
	return v.(T) // the ClassCastException of the erased cast
}

// jembeddedPtr_ is the pointer to the struct of type want that the object v points to embeds (the
// view of a superclass), or an invalid value.
func jembeddedPtr_(v reflect.Value, want reflect.Type) reflect.Value {
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
		if p := jembeddedPtr_(s.Field(i).Addr(), want); p.IsValid() {
			return p
		}
	}
	return reflect.Value{}
}

// Object's methods called on a value of a type parameter, and == between two values of one type
// parameter: as the erased code computes them on the box, without making a box for a String or a
// Node (strobj_ allocates; the erased code had the box at hand).

// hashCodeT_ is v.hashCode() for a value of a type parameter.
func hashCodeT_[T any](v T) int32 {
	switch p := any(&v).(type) {
	case *string:
		if *p == nullstr_ {
			panic(new_NullPointerException_0().self())
		}
		return String_hashCode(*p)
	case *Node:
		return Object_hashCode(p.box_())
	}
	return Object_hashCode(boxT_(v))
}

// jequalsT_ is a.equals(b) for a value a of a type parameter.
func jequalsT_[T any](a T, b any) bool {
	if p, ok := any(&a).(*string); ok {
		if *p == nullstr_ {
			panic(new_NullPointerException_0().self())
		}
		s, ok := b.(string)
		return ok && s == *p
	}
	return Object_equals(boxT_(a), b)
}

// equalsTT_ is a.equals(b) for two values of one type parameter.
func equalsTT_[T any](a, b T) bool {
	if p, ok := any(&a).(*string); ok {
		if *p == nullstr_ {
			panic(new_NullPointerException_0().self())
		}
		return *p == *any(&b).(*string)
	}
	return Object_equals(boxT_(a), boxT_(b))
}

// toStringT_ is v.toString() for a value of a type parameter.
func toStringT_[T any](v T) string {
	if p, ok := any(&v).(*string); ok {
		if *p == nullstr_ {
			panic(new_NullPointerException_0().self())
		}
		return *p
	}
	return Object_toString(boxT_(v))
}

// eqT_ is a == b for two values of one type parameter: the boxes compared, as in the erased code
// (a String compares by contents there too, null only equal to null).
func eqT_[T any](a, b T) bool {
	switch p := any(&a).(type) {
	case *string:
		return *p == *any(&b).(*string)
	case *Node:
		return *p == *any(&b).(*Node)
	}
	return boxT_(a) == boxT_(b)
}

// singles_ holds the objects of one shared static of a generic class (j2go rule "shared
// statics"): Java's one object of type C<?, ?>, of which the Go program has one per instantiation.
type singles_ struct {
	list atomic.Pointer[[]singleEntry_]
	mu   sync.Mutex
}

type singleEntry_ struct {
	typ unsafe.Pointer
	v   any
}

// single_ is the object of s for instantiation T (a pointer type), made by mk on first use.
func single_[T any](s *singles_, mk func() T) T {
	var z T
	x := any(z)
	typ := (*eface_)(unsafe.Pointer(&x)).typ
	if l := s.list.Load(); l != nil {
		for i := range *l {
			if (*l)[i].typ == typ {
				return (*l)[i].v.(T)
			}
		}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var old []singleEntry_
	if l := s.list.Load(); l != nil {
		old = *l
		for i := range old {
			if old[i].typ == typ {
				return old[i].v.(T)
			}
		}
	}
	v := mk()
	l := append(append([]singleEntry_(nil), old...), singleEntry_{typ, v})
	s.list.Store(&l)
	return v
}

// Typed containers inside generic code (post-flip step gmaps): a typed map or set keyed by a
// type parameter needs the key's conversion (tconv_), which generated code names per type
// (tc_X, tch_X); generic code gets it from the instantiation instead, once per type.

var tconvCache_, tconvIdCache_ sync.Map // reflect.Type -> *tconv_[T]

// tconvT_ is the conversion of type parameter T's values for a LinkedHashMap / LinkedHashSet
// keyed by them: boxed as the erased code boxes them (boxT_, junboxT_, jnull_); compared by
// Go's == where that is Java's equals (strings, handles and enums, pointers to classes without
// equals of their own: their box is their most derived object, one per object), else by
// hashCode and equals as the erased table compared the boxes (hashCodeT_, equalsTT_; an
// interface type: its values may be of either kind).
func tconvT_[T any]() *tconv_[T] {
	t := reflect.TypeFor[T]()
	if c, ok := tconvCache_.Load(t); ok {
		return c.(*tconv_[T])
	}
	c := &tconv_[T]{box: boxT_[T], unbox: junboxT_[T], null: jnull_[T]()}
	if !eqIsIdentityT_[T]() {
		c.hash = hashCodeT_[T]
		c.eq = equalsTT_[T]
	}
	v, _ := tconvCache_.LoadOrStore(t, c)
	return v.(*tconv_[T])
}

// tconvIdT_ is the conversion of T's values for an IdentityHashMap keyed by them: compared by
// identity, as the erased table compared the boxes (==): by Go's == where the value is its own
// identity (a pointer, a string, a handle, an enum), else by its box (an interface value, whose
// dynamic type may be any class of the object's hierarchy).
func tconvIdT_[T any]() *tconv_[T] {
	t := reflect.TypeFor[T]()
	if c, ok := tconvIdCache_.Load(t); ok {
		return c.(*tconv_[T])
	}
	c := &tconv_[T]{box: boxT_[T], unbox: junboxT_[T], null: jnull_[T]()}
	if t.Kind() == reflect.Interface {
		c.hash = func(v T) int32 { return identityHash_(boxT_(v)) }
		c.eq = func(a, b T) bool { return boxT_(a) == boxT_(b) }
	}
	v, _ := tconvIdCache_.LoadOrStore(t, c)
	return v.(*tconv_[T])
}

// eqIsIdentityT_ reports whether Java's equals of T's values is Go's == on them.
func eqIsIdentityT_[T any]() bool {
	t := reflect.TypeFor[T]()
	switch t.Kind() {
	case reflect.String, reflect.Bool, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64:
		return true
	case reflect.Pointer:
		if t.Elem().Kind() != reflect.Struct {
			return false
		}
		var z T
		_, has := any(z).(interface{ equals(any) bool })
		return !has
	}
	return false
}
