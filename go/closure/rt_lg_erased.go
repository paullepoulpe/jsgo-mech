//go:build j2go_nolibgen

package closure

// The runtime's functions that only a translation with a piece of rule "typed library
// interfaces" off names (J2GO_NOLIBGEN=...; build with -tags j2go_nolibgen): the erased
// factories and constructors that the typed forms have replaced in the default translation.
// Found as the top-level functions that the default translation, the runtime and the self-test
// do not name while the translation with every switch off does (docs/libgen/COLLECTIONS-PLAN.md
// 5.6). They do not count as the default build's runtime.

func to_ArrayList(v any) *ArrayList {
	if v == nil {
		return nil
	}
	return v.(interface{ as_ArrayList() *ArrayList }).as_ArrayList()
}

func new_ArrayList_int(n int32) *ArrayList {
	this := &ArrayList{items: make([]any, 0, n)}
	this.self_ = this
	return this
}

// The unmodifiable views are the collections themselves: nothing checks that
// they are not written through.
func Collections_unmodifiableCollection(c Collection) Collection { return c }

func Collections_unmodifiableSet(s Set) Set { return s }

func ImmutableList_of_Object_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any, p4 any) *ImmutableList {
	return immList_([]any{p0, p1, p2, p3, p4})
}

func ImmutableList_of_Object_Object_Object_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any, p6 any) *ImmutableList {
	return immList_([]any{p0, p1, p2, p3, p4, p5, p6})
}

func ImmutableList_of_Object_Object_Object_Object_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any, p6 any, p7 any) *ImmutableList {
	return immList_([]any{p0, p1, p2, p3, p4, p5, p6, p7})
}

func ImmutableList_of_Object_Object_Object_Object_Object_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any, p6 any, p7 any, p8 any) *ImmutableList {
	return immList_([]any{p0, p1, p2, p3, p4, p5, p6, p7, p8})
}

func ImmutableList_of_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any, p6 any, p7 any, p8 any, p9 any) *ImmutableList {
	return immList_([]any{p0, p1, p2, p3, p4, p5, p6, p7, p8, p9})
}

func ImmutableList_of_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any, p6 any, p7 any, p8 any, p9 any, p10 any) *ImmutableList {
	return immList_([]any{p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, p10})
}

func ImmutableList_copyOf_ObjectArr(p0 []any) *ImmutableList {
	return immList_(append([]any(nil), p0...))
}

func ImmutableSet_of_Object_Object_Object_Object_Object_Object_ObjectArr(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any, p6 []any) *ImmutableSet {
	return immSet_(append([]any{p0, p1, p2, p3, p4, p5}, p6...))
}

func ImmutableList_of_Object_Object_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any) *ImmutableList {
	return immList_([]any{p0, p1, p2, p3, p4, p5})
}

func Sets_newLinkedHashSetWithExpectedSize(p0 int32) *LinkedHashSet { return new_LinkedHashSet() }

func Maps_newLinkedHashMapWithExpectedSize(p0 int32) *LinkedHashMap { return new_LinkedHashMap() }
