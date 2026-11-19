package closure

// java.util.Optional<String> as a string (post-flip step optstr, tools/postflip/optstr): the
// empty Optional is nullstr_ (the null String, which Optional.ofNullable makes empty and
// Optional.of rejects), a present one is its string. The step proves that no Optional<String>
// is ever null (the zero value of j2go's Optional[T]), so these are the methods of a non-null
// Optional; each throws what the method of Optional[string] throws.

// optstrOf_ is Optional.of(s): a NullPointerException for null.
func optstrOf_(s string) string {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return s
}

// optstrGet_ is o.get() and o.orElseThrow(): NoSuchElementException on the empty Optional.
func optstrGet_(o string) string {
	if o == nullstr_ {
		optstrNoValue_()
	}
	return o
}

// optstrNoValue_ throws get()'s exception (a call of its own, so that optstrGet_ inlines).
//
//go:noinline
func optstrNoValue_() {
	panic(new_NoSuchElementException_String("No value present").self())
}

// optstrOr_ is o.orElse(other).
func optstrOr_(o, other string) string {
	if o == nullstr_ {
		return other
	}
	return o
}

// optstrFilter_ is o.filter(p) with p taking the content unboxed (Optional.filterFn_).
func optstrFilter_(o string, p func(string) bool) string {
	if o == nullstr_ || p(o) {
		return o
	}
	return nullstr_
}

// optstrHash_ is Objects.hashCode(o) of the Optional: its content's String.hashCode, 0 when
// empty (Optional.hashCode is Objects.hashCode of the content).
func optstrHash_(o string) int32 {
	if o == nullstr_ {
		return 0
	}
	return String_hashCode(o)
}

// optstrBox_ is the Optional as erased code sees it: the Optional object (Optional.box_), so
// that toString, equals and hashCode of the Object are Optional's. The empty one is one object,
// as Java's Optional.empty() is (j2go's Optional_empty[string]().box_() of a constant was one
// static object too).
func optstrBox_(o string) any {
	if o == nullstr_ {
		return optstrEmpty_
	}
	return Optional_of[string](o).box_()
}

var optstrEmpty_ any = Optional_empty[string]().box_()

// optstr_ is a non-null Optional<String> of the runtime (Optional[string]) as the string.
func optstr_(o Optional[string]) string {
	o.nn_()
	if !o.present {
		return nullstr_
	}
	return o.value
}

// optstrUnbox_ is the cast of an Object holding an Optional<String> (optOf_[string]): the
// step proves the Object is never null.
func optstrUnbox_(v any) string { return optstr_(optOf_[string](v)) }
