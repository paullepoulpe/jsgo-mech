//go:build !j2go_nofntypes

package closure

// Piece `fns2` (tools/j2go LibFns.TYPED_F2; J2GO_NOLIBGEN=fns2 is the translation without it;
// docs/javaisms6/RESIDUE8.md, rule "more LibFns twins"): more twins of the kind of rt_lgfn.go,
// at the Java generic signatures, each mirroring the erased function step by step.

// Arrays_setAllT_ is Arrays.setAll(T[], IntFunction) of an array of a class type (Arrays_setAll,
// which the erased translation gave a converted copy of the array).
func Arrays_setAllT_[T any](a []T, f IntFunction[T]) {
	for i := range a {
		a[i] = f(int32(i))
	}
}

// Comparator_comparingS_ is Comparator.comparing with a String key (Comparator_comparing at
// U = String, whose compareNatural_ compares two strings with String_compareTo).
func Comparator_comparingS_[T any](f Function[T, string]) Comparator[T] {
	return func(a, b T) int32 { return cmpStrKeys_(f(a), f(b)) }
}

// Comparator_thenComparingS_ is Comparator.thenComparing(Function) with a String key
// (thenComparing, which compares the boxes of the keys).
func Comparator_thenComparingS_[T any](c Comparator[T], f Function[T, string]) Comparator[T] {
	return c.thenComparing_Comparator(Comparator_comparingS_(f))
}

// cmpStrKeys_ is compareNatural_ of two String keys (a null one throws there).
func cmpStrKeys_(a, b string) int32 {
	if a == nullstr_ || b == nullstr_ {
		return compareNatural_(strobj_(a), strobj_(b))
	}
	return String_compareTo(a, b)
}

// Map_putIfAbsent_sT_ is Map.putIfAbsent with a String key on a map whose keys are Strings
// (Map_putIfAbsent_s).
func Map_putIfAbsent_sT_[V any](m MapT[string, V], k string, v V) V {
	return unboxT_[V](Map_putIfAbsent_s(Map_un_(m), k, boxT_(v)))
}

// Map_replace_sT_ is Map.replace(key, oldValue, newValue) with a String key on a map whose keys
// are Strings (Map_replace).
func Map_replace_sT_[V any](m MapT[string, V], k string, oldV V, newV V) bool {
	u := Map_un_(m)
	if Map_containsKey_s(u, k) && objequals_(Map_get_s(u, k), boxT_(oldV)) {
		Map_put_s(u, k, boxT_(newV))
		return true
	}
	return false
}

// Collections_singletonListT_ is Collections.singletonList for a typed place
// (Collections_singletonList, an ArrayList of the one element).
func Collections_singletonListT_[T any](x T) ListT[T] {
	l := newTList_(tcT_[T]())
	l.items = []T{x}
	return l.asListT()
}
