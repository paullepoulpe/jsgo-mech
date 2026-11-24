//go:build !j2go_nofntypes

package closure

// Rule "typed library interfaces", kind `goptional` (tools/j2go Ctx.registerValueLibs;
// J2GO_NOLIBGEN=goptional is the translation without it, which names the erased class
// Optional_base of rt_base.go): Guava's com.google.common.base.Optional<T> is a value library
// class, as java.util.Optional is (rule "Optional is a value", rt_base.go): a value whose zero
// value is Java's null, with its content held as a T (an Optional<String> holds its string
// unboxed), so that Optional.of and Optional.absent allocate nothing. Every method of a null
// Optional throws NullPointerException, as a call on null does; == on two of them is refused
// by the translator (LibGen.check). A wildcard, raw or erased type argument is GOptional[any];
// optGOptionalAs_ converts between instantiations, box_ / optGOptionalOf_ to and from Object.
// Each method does what Optional_base's does (rt_base.go), with the same messages. The methods
// are the ones of LibGen.GOPT_METHODS (the overloads of `or` by the translator's names of
// library overloads: or(T) is `or`, or(Optional) `or_Optional`); the translator refuses a call
// of any other. Built with rule "function types" only (transform takes a Go func).
type GOptional[T any] struct {
	value   T
	present bool
	set     bool // not null
}

func (this GOptional[T]) nn_() {
	if !this.set {
		panic(new_NullPointerException_0().self())
	}
}

// jnull_ is `this == null`.
func (this GOptional[T]) jnull_() bool { return !this.set }

func GOptional_absent[T any]() GOptional[T] { return GOptional[T]{set: true} }

func GOptional_of[T any](v T) GOptional[T] {
	if jisnull_(v) {
		panic(new_NullPointerException_0().self())
	}
	return GOptional[T]{v, true, true}
}

func GOptional_fromNullable[T any](v T) GOptional[T] {
	if jisnull_(v) {
		return GOptional[T]{set: true}
	}
	return GOptional[T]{v, true, true}
}

// goptional_ is any instantiation of GOptional (not java.util.Optional's joptional_: the two
// classes' objects are never equal).
type goptional_ interface {
	gcontent_() any
	gisSet_() bool
	gpresent_() bool
}

// gcontent_ is the content as an object, nil when absent.
func (this GOptional[T]) gcontent_() any {
	if !this.present {
		return nil
	}
	return boxT_(this.value)
}

func (this GOptional[T]) gisSet_() bool   { return this.set }
func (this GOptional[T]) gpresent_() bool { return this.present }

// box_ is the Optional as an Object: nil for null.
func (this GOptional[T]) box_() any {
	if !this.set {
		return nil
	}
	return this
}

// optGOptionalOf_ is an Object as an Optional<T> (a cast): nil is null, an Optional of another
// instantiation is converted, anything else is the cast's ClassCastException.
func optGOptionalOf_[T any](v any) GOptional[T] {
	if v == nil {
		return GOptional[T]{}
	}
	if o, ok := v.(GOptional[T]); ok {
		return o
	}
	o := v.(goptional_)
	if c := o.gcontent_(); c != nil {
		return GOptional[T]{junboxT_[T](c), true, true}
	}
	return GOptional[T]{set: o.gisSet_()}
}

// optGOptionalAs_ is an Optional<S> seen as an Optional<T> (in Java the same object).
func optGOptionalAs_[T, S any](o GOptional[S]) GOptional[T] {
	if !o.present {
		return GOptional[T]{set: o.set}
	}
	return GOptional[T]{junboxT_[T](boxT_(o.value)), true, true}
}

// isGOptional_ is `v instanceof com.google.common.base.Optional`.
func isGOptional_(v any) bool {
	_, ok := v.(goptional_)
	return ok
}

// get is Optional.get (Optional_base.get).
func (this GOptional[T]) get() T {
	this.nn_()
	if !this.present {
		stateFail_("Optional.get() cannot be called on an absent value")
	}
	return this.value
}

func (this GOptional[T]) isPresent() bool { this.nn_(); return this.present }

// or is Optional.or(T) (Optional_base.or).
func (this GOptional[T]) or(def T) T {
	this.nn_()
	if jisnull_(def) {
		nullFail_("use Optional.orNull() instead of Optional.or(null)")
	}
	if this.present {
		return this.value
	}
	return def
}

// orNull is Optional.orNull (Optional_base.orNull): Java's null of T when absent.
func (this GOptional[T]) orNull() T {
	this.nn_()
	if !this.present {
		return jnull_[T]()
	}
	return this.value
}

// or_Optional is Optional.or(Optional<? extends T>) (Optional_base.or_Optional): the
// parameter's wildcard is `any` in Go (the translator converts the argument), the result the
// receiver's instantiation.
func (this GOptional[T]) or_Optional(o GOptional[any]) GOptional[T] {
	this.nn_()
	if !o.set {
		panic(new_NullPointerException_0().self())
	}
	if this.present {
		return this
	}
	return optGOptionalAs_[T](o)
}

// transform is Optional.transform (Optional_base.transform); the result's type argument is
// the method's own, which Go methods do not have: Object, converted by the caller.
func (this GOptional[T]) transform(f Function_base[T, any]) GOptional[any] {
	this.nn_()
	if f == nil {
		panic(new_NullPointerException_0().self())
	}
	if !this.present {
		return GOptional[any]{set: true}
	}
	r := f(this.value)
	if r == nil {
		nullFail_("the Function passed to Optional.transform() must not return null.")
	}
	return GOptional[any]{r, true, true}
}

func (this GOptional[T]) equals(o any) bool {
	this.nn_()
	x, ok := o.(goptional_)
	if !ok || !x.gisSet_() {
		return false
	}
	if !this.present || !x.gpresent_() {
		return this.present == x.gpresent_()
	}
	return objequals_(this.gcontent_(), x.gcontent_())
}

func (this GOptional[T]) hashCode() int32 {
	this.nn_()
	if !this.present {
		return 0x79a31aac
	}
	return 0x598df91c + objhash_(this.gcontent_())
}

func (this GOptional[T]) toString() string {
	this.nn_()
	if !this.present {
		return "Optional.absent()"
	}
	return "Optional.of(" + objstr_(this.gcontent_()) + ")"
}
