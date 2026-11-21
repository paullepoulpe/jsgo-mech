package closure

// LinkedIdentityHashMap's methods on a typed map (post-flip step colls -kind idmap).
//
// com.google.javascript.jscomp.base.LinkedIdentityHashMap is an identity hash map in insertion
// order without removal. colls types one whose keys are pointers as a *TMap[K, V]: Go's == on
// pointers is identity, and a TMap keeps insertion order. get and put are the TMap's (a
// missing key reads as null, put returns null for a new key, as LinkedIdentityHashMap's do);
// the methods below are LinkedIdentityHashMap's own, which differ from Map's default methods
// (computeIfAbsent stores a null result, merge stores the value when the key reads as null).

// lihmComputeIfAbsent_ is LinkedIdentityHashMap.computeIfAbsent: the value if non-null, else
// fn(key), stored even when null.
func lihmComputeIfAbsent_[K comparable, V any](m *TMap[K, V], k K, fn func(any) any) V {
	if old := m.get(k); !m.vc.isNull(old) {
		return old
	}
	v := m.vc.unbox(fn(m.kc.box(k)))
	m.put(k, v)
	return v
}

// lihmMerge_ is LinkedIdentityHashMap.merge: value when the key reads as null, else
// fn(old, value); stored either way.
func lihmMerge_[K comparable, V any](m *TMap[K, V], k K, value V, fn func(any, any) any) V {
	old := m.get(k)
	if m.vc.isNull(old) {
		m.put(k, value)
		return value
	}
	v := m.vc.unbox(fn(m.vc.box(old), m.vc.box(value)))
	m.put(k, v)
	return v
}

// lihmForEach_ is LinkedIdentityHashMap.forEach: fn(key, value) in insertion order, reading the
// entries anew at every step (it walked its entry list the same way).
func lihmForEach_[K comparable, V any](m *TMap[K, V], fn func(any, any)) {
	for e := m.each(); e.next(); {
		fn(m.kc.box(e.k), m.vc.box(e.v))
	}
}
