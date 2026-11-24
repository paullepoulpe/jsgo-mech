package closure

// Rule "typed library interfaces", piece `mkext` (tools/j2go ExprEmitter.mkextNew,
// mkextFreshArg, immutableOf; RESIDUE8 T2.3, T2.8, T3.2; docs/libgen/COLLECTIONS-PLAN.md section
// 16): more makers of typed collections for a typed place, each the typed collection of the same
// behaviour as the erased object the runtime makes.

// tenumMapNew_ is new EnumMap<>(K.class): the typed map in its sorted mode with the ordinal
// comparator, as the runtime's EnumMap is a TreeMap with ordinalCmp_ (rt_coll2.go).
func tenumMapNew_[K comparable, V any](kc *tconv_[K], vc *tconv_[V]) *TMap[K, V] {
	return newTSortedMapC_(kc, vc, ordinalCmp_())
}

// tdequeCopyL_ is new ArrayDeque<>(from) of a typed list of the same element type: from's first
// element is the head (tdequeFrom_).
func tdequeCopyL_[T any](c *tconv_[T], from *TList[T]) *TDeque[T] {
	d := newTDeque_(c)
	d.items = append([]T(nil), from.items...)
	return d
}

// immutableMapOf6_ ... immutableMapOf10_ are ImmutableMap.of(k1, v1, ...) with 6 to 10 entries
// (immutableMapOf1_ ... 5_ in rt_timm.go): the arguments in Java's order.

func immutableMapOf6_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V, k5 K, v5 V, k6 K, v6 V) *TMap[K, V] {
	return tmapMake_(kc, vc, []K{k1, k2, k3, k4, k5, k6}, []V{v1, v2, v3, v4, v5, v6})
}

func immutableMapOf7_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V, k5 K, v5 V, k6 K, v6 V, k7 K, v7 V) *TMap[K, V] {
	return tmapMake_(kc, vc, []K{k1, k2, k3, k4, k5, k6, k7}, []V{v1, v2, v3, v4, v5, v6, v7})
}

func immutableMapOf8_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V, k5 K, v5 V, k6 K, v6 V, k7 K, v7 V, k8 K, v8 V) *TMap[K, V] {
	return tmapMake_(kc, vc, []K{k1, k2, k3, k4, k5, k6, k7, k8}, []V{v1, v2, v3, v4, v5, v6, v7, v8})
}

func immutableMapOf9_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V, k5 K, v5 V, k6 K, v6 V, k7 K, v7 V, k8 K, v8 V, k9 K, v9 V) *TMap[K, V] {
	return tmapMake_(kc, vc, []K{k1, k2, k3, k4, k5, k6, k7, k8, k9}, []V{v1, v2, v3, v4, v5, v6, v7, v8, v9})
}

func immutableMapOf10_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], k1 K, v1 V, k2 K, v2 V, k3 K, v3 V, k4 K, v4 V, k5 K, v5 V, k6 K, v6 V, k7 K, v7 V, k8 K, v8 V, k9 K, v9 V, k10 K, v10 V) *TMap[K, V] {
	return tmapMake_(kc, vc, []K{k1, k2, k3, k4, k5, k6, k7, k8, k9, k10}, []V{v1, v2, v3, v4, v5, v6, v7, v8, v9, v10})
}
