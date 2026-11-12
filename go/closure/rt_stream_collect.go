package closure

// Collectors. A collector receives the stream's elements, in encounter order,
// in a slice it owns.

type collector_ struct {
	finish func(items []any) any
}

func Collectors_toList() Collector {
	return &collector_{func(items []any) any { return listOf_(items) }}
}

func Collectors_toSet() Collector {
	return &collector_{func(items []any) any {
		s := new_LinkedHashSet()
		for _, v := range items {
			s.add(v)
		}
		return s
	}}
}

func Collectors_toCollection(factory Supplier) Collector {
	return &collector_{func(items []any) any {
		r := factory.get()
		c := r.(Collection)
		for _, v := range items {
			c.add(v)
		}
		return r
	}}
}

func Collectors_toMap(keyFn Function, valueFn Function) Collector {
	return &collector_{func(items []any) any {
		m := new_LinkedHashMap()
		for _, e := range items {
			k := keyFn.apply(e)
			v := valueFn.apply(e)
			if v == nil {
				panic(streamNpe_(""))
			}
			if m.containsKey(k) {
				panic(new_IllegalStateException_String("Duplicate key " + objstr_(k) +
					" (attempted merging values " + objstr_(m.get(k)) + " and " + objstr_(v) + ")").self())
			}
			m.put(k, v)
		}
		return m
	}}
}

func Collectors_joining(sep any) Collector {
	return &collector_{func(items []any) any {
		d := objstr_(sep)
		n := 0
		for _, v := range items {
			n += len(objstr_(v)) + len(d)
		}
		b := make([]byte, 0, n)
		for i, v := range items {
			if i > 0 {
				b = append(b, d...)
			}
			b = append(b, objstr_(v)...)
		}
		return string(b)
	}}
}

func Collectors_groupingBy(classifier Function) Collector {
	return &collector_{func(items []any) any {
		m := new_LinkedHashMap()
		for _, e := range items {
			k := classifier.apply(e)
			if k == nil {
				panic(streamNpe_("element cannot be mapped to a null key"))
			}
			l, _ := m.get(k).(*ArrayList)
			if l == nil {
				l = new_ArrayList_0()
				m.put(k, l)
			}
			l.add(e)
		}
		return m
	}}
}

// ---- Guava collectors: accumulate here, finish through the collection
// classes' own factories.

func streamRequire_(items []any) {
	for _, v := range items {
		if v == nil {
			panic(streamNpe_(""))
		}
	}
}

func ImmutableList_toImmutableList() Collector {
	return &collector_{func(items []any) any {
		streamRequire_(items)
		return ImmutableList_copyOf_ObjectArr(items)
	}}
}

func ImmutableSet_toImmutableSet() Collector {
	return &collector_{func(items []any) any {
		streamRequire_(items)
		return ImmutableSet_copyOf_ObjectArr(items)
	}}
}

func ImmutableSortedSet_toImmutableSortedSet(c Comparator) Collector {
	if c == nil {
		panic(streamNpe_(""))
	}
	return &collector_{func(items []any) any {
		streamRequire_(items)
		return ImmutableSortedSet_copyOf_Comparator_Collection(c, listOf_(items))
	}}
}

func ImmutableMultiset_toImmutableMultiset() Collector {
	return &collector_{func(items []any) any {
		streamRequire_(items)
		b := ImmutableMultiset_builder()
		for _, v := range items {
			b.add(v)
		}
		return b.build()
	}}
}

// streamEntries_ applies the key and value functions to every element.
func streamEntries_(items []any, keyFn Function, valueFn Function) (keys []any, values []any) {
	keys = make([]any, len(items))
	values = make([]any, len(items))
	for i, e := range items {
		k := keyFn.apply(e)
		v := valueFn.apply(e)
		if k == nil || v == nil {
			panic(streamNpe_("null key or value in entry: " + objstr_(k) + "=" + objstr_(v)))
		}
		keys[i], values[i] = k, v
	}
	return
}

func ImmutableMap_toImmutableMap(keyFn Function, valueFn Function) Collector {
	return &collector_{func(items []any) any {
		keys, values := streamEntries_(items, keyFn, valueFn)
		b := ImmutableMap_builder()
		for i := range keys {
			b.put(keys[i], values[i])
		}
		return b.buildOrThrow()
	}}
}

// streamSortedMap_ collects the entries into a TreeMap ordered by c; equal keys
// are an IllegalArgumentException, as in Guava.
func streamSortedMap_(c Comparator, keys []any, values []any) *TreeMap {
	m := new_TreeMap_Comparator(c)
	for i, k := range keys {
		if m.containsKey(k) {
			panic(new_IllegalArgumentException_String("Multiple entries with same key: " +
				objstr_(k) + "=" + objstr_(m.get(k)) + " and " + objstr_(k) + "=" + objstr_(values[i])).self())
		}
		m.put(k, values[i])
	}
	return m
}

func ImmutableSortedMap_toImmutableSortedMap(c Comparator, keyFn Function, valueFn Function) Collector {
	if c == nil {
		panic(streamNpe_(""))
	}
	return &collector_{func(items []any) any {
		keys, values := streamEntries_(items, keyFn, valueFn)
		return ImmutableSortedMap_copyOf(streamSortedMap_(c, keys, values))
	}}
}
