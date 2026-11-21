package closure

// Support for collections that are Go slices and maps (post-flip steps 1 and 2).

// sliceEach_ iterates over a slice the way each_ iterates over an ArrayList: it reads the slice
// anew at every step, so elements appended during the loop are visited.
type sliceEach_[T any] struct {
	src *[]T
	i   int
	v   T
}

func eachOf_[T any](s *[]T) sliceEach_[T] { return sliceEach_[T]{src: s} }

func (e *sliceEach_[T]) next() bool {
	if e.i < len(*e.src) {
		e.v = (*e.src)[e.i]
		e.i++
		return true
	}
	return false
}

// has_ reports whether m has key k (Map.containsKey, Set.contains).
func has_[K comparable, V any](m map[K]V, k K) bool {
	_, ok := m[k]
	return ok
}

// setAdd_ adds k to the set and reports whether it was new (the value of Set.add).
func setAdd_[K comparable](m map[K]struct{}, k K) bool {
	if _, ok := m[k]; ok {
		return false
	}
	m[k] = struct{}{}
	return true
}

// mapStr_ is Map.get for String values: an absent key gives Java's null string.
func mapStr_[K comparable](m map[K]string, k K) string {
	if v, ok := m[k]; ok {
		return v
	}
	return nullstr_
}

// Lists that are slices (tools/postflip/native): what TList does on its items, for a slice.

// sliceAt_ is List.get: an index out of range throws IndexOutOfBoundsException with
// ArrayList's message, as TList.get does.
func sliceAt_[T any](s []T, i int32) T {
	if i < 0 || int(i) >= len(s) {
		throwIndex_(i, len(s))
	}
	return s[i]
}

func throwIndex_(i int32, n int) {
	panic(new_IndexOutOfBoundsException_String("Index " + istr_(int64(i)) + " out of bounds for length " + istr_(int64(n))).self())
}

// sliceInsert_ is List.add(i, v).
func sliceInsert_[T any](s *[]T, i int32, v T) {
	if i < 0 || int(i) > len(*s) {
		panic(new_IndexOutOfBoundsException_String("Index: " + istr_(int64(i)) + ", Size: " + istr_(int64(len(*s)))).self())
	}
	var z T
	*s = append(*s, z)
	copy((*s)[i+1:], (*s)[i:])
	(*s)[i] = v
}

// sliceRemoveAt_ is List.remove(int): it returns the element removed.
func sliceRemoveAt_[T any](s *[]T, i int32) T {
	old := sliceAt_(*s, i)
	var z T
	if i == 0 && len(*s) > frontShift_ { // a long list steps over the slot (see ArrayList.remove)
		(*s)[0] = z
		*s = (*s)[1:]
		return old
	}
	copy((*s)[i:], (*s)[i+1:])
	(*s)[len(*s)-1] = z
	*s = (*s)[:len(*s)-1]
	return old
}

// sliceClear_ is List.clear.
func sliceClear_[T any](s *[]T) {
	clear(*s)
	*s = (*s)[:0]
}

// sliceFrom_ is new ArrayList<>(from) as a slice.
func sliceFrom_[T any](c *tconv_[T], from Collection) []T { return unboxAll_(c, readSlice_(from)) }

// sliceImm_ is an ImmutableList (or nil) as a slice, which nothing changes. The typed copy is
// kept on an immutable list from its second conversion to T on (the first one only notes T):
// a list made to be converted once is copied once, a long-lived one is copied twice at most.
// An empty list is an empty slice, not nil (Java's null).
func sliceImm_[T any](c *tconv_[T], from Collection) []T {
	ic, ok := from.(*ImmutableCollection)
	if !ok || ic == nil {
		if l := tlistImm_(c, from); l != nil {
			return l.items
		}
		return nil
	}
	slot := &ic.typed_
	seen := slot.conv == any(c)
	if seen {
		if l, ok := slot.v.(*TList[T]); ok {
			return l.items
		}
	}
	src := ic.elems_()
	if len(src) == 0 {
		return []T{}
	}
	s := unboxAll_(c, src)
	if seen {
		*slot = immTyped_{c, &TList[T]{items: s, c: c, imm: true}}
	} else {
		*slot = immTyped_{conv: c}
	}
	return s
}

// unboxAll_ is a new slice of the unboxed elements of src.
func unboxAll_[T any](c *tconv_[T], src []any) []T {
	s := make([]T, len(src))
	for i, x := range src {
		s[i] = c.unbox(x)
	}
	return s
}

// ---- ImmutableList.Builder as the slice it builds (step immbuild)

// immAdd_ is ImmutableList.Builder.add(x) on a builder kept as the slice it builds: a null
// element throws NullPointerException as the builder does. The first element makes room for
// four, as Guava's builder starts.
func immAdd_[T any](c *tconv_[T], s []T, x T) []T {
	if c.box(x) == nil {
		throwNPE_("at index " + istr_(int64(len(s))))
	}
	if cap(s) == 0 {
		s = make([]T, 0, 4)
	}
	return append(s, x)
}

// immAddObj_ is immAdd_ of a boxed element.
func immAddObj_[T any](c *tconv_[T], s []T, x any) []T {
	if x == nil {
		throwNPE_("at index " + istr_(int64(len(s))))
	}
	if cap(s) == 0 {
		s = make([]T, 0, 4)
	}
	return append(s, c.unbox(x))
}

// immAddAll_ is ImmutableList.Builder.addAll(from): each element is added as by add.
func immAddAll_[T any](c *tconv_[T], s []T, from Iterable) []T {
	for _, x := range readSlice_(from) {
		s = immAddObj_(c, s, x)
	}
	return s
}

// immBuilt_ is ImmutableList.Builder.build() of a builder kept as the slice s: the elements so
// far, with the capacity cut to them so that later adds to the builder never reach the list
// built. An empty list is an empty slice, not nil (Java's null).
func immBuilt_[T any](s []T) []T {
	if s == nil {
		return []T{}
	}
	return s[:len(s):len(s)]
}

// immCopyOf_ is ImmutableList.copyOf(from) as a slice: an immutable collection is converted as
// sliceImm_ converts its asList(); any other is copied, and a null element throws
// NullPointerException as copyOf does.
func immCopyOf_[T any](c *tconv_[T], from Iterable) []T {
	if ic, ok := from.(*ImmutableCollection); ok {
		return sliceImm_(c, Collection(ic.asList()))
	}
	src := toSlice_(from)
	if len(src) == 0 {
		return []T{}
	}
	checkNoNulls_(src)
	return unboxAll_(c, src)
}

// immCopySlice_ is ImmutableList.copyOf(sliceView_(c, s)) of a slice s seen as a list: a null
// list throws NullPointerException, as its view did; the elements are checked and copied.
func immCopySlice_[T any](c *tconv_[T], s []T) []T {
	if s == nil {
		panic(new_NullPointerException_0().self())
	}
	return immCopyArr_(c, s)
}

// immCopyArr_ is ImmutableList.copyOf(a) of an array a as a slice: the elements, checked for
// null as copyOf does, are copied (the array can change later). An empty or null array is an
// empty list, as ImmutableList_copyOf_ObjectArr makes it.
func immCopyArr_[T any](c *tconv_[T], a []T) []T {
	if len(a) == 0 {
		return []T{}
	}
	for i, x := range a {
		if c.box(x) == nil {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	return append([]T(nil), a...)
}

// sliceView_ is s as an erased List for one call that reads it, or sorts it in place: a typed
// list over the same items (it does not see later changes of the slice's length).
func sliceView_[T any](c *tconv_[T], s []T) List {
	if s == nil {
		return nil
	}
	return (&TList[T]{items: s, c: c}).asList()
}

// sliceAddAll_ is List.addAll(from).
func sliceAddAll_[T any](c *tconv_[T], s *[]T, from Collection) bool {
	src := toSlice_(from)
	for _, x := range src {
		*s = append(*s, c.unbox(x))
	}
	return len(src) > 0
}

// sliceRemoveObject_ is List.remove(Object): the first element equal to x goes.
func sliceRemoveObject_[T any](c *tconv_[T], s *[]T, x any) bool {
	for i, y := range *s {
		if objequals_(x, c.box(y)) {
			sliceRemoveAt_(s, int32(i))
			return true
		}
	}
	return false
}

// sliceRemoveAll_ is List.removeAll(from).
func sliceRemoveAll_[T any](c *tconv_[T], s *[]T, from Collection) bool {
	out := (*s)[:0]
	for _, x := range *s {
		if !from.contains(c.box(x)) {
			out = append(out, x)
		}
	}
	changed := len(out) != len(*s)
	clear((*s)[len(out):])
	*s = out
	return changed
}

// Typed maps and sets that are Go maps (tools/postflip/native -kind map|set): what TMap and
// TSet do, for a Go map that nothing iterates. c is the value conversion; its null is Java's
// null for the value type.

// mapPut_ is Map.put used as a value: the previous value, or null.
func mapPut_[K comparable, V any](c *tconv_[V], m map[K]V, k K, v V) V {
	old, ok := m[k]
	m[k] = v
	if !ok {
		return c.null
	}
	return old
}

// mapRemove_ is Map.remove used as a value: the value removed, or null.
func mapRemove_[K comparable, V any](c *tconv_[V], m map[K]V, k K) V {
	old, ok := m[k]
	if !ok {
		return c.null
	}
	delete(m, k)
	return old
}

// mapGetOr_ is Map.getOrDefault.
func mapGetOr_[K comparable, V any](m map[K]V, k K, def V) V {
	if v, ok := m[k]; ok {
		return v
	}
	return def
}

// mapPutIfAbsent_ is Map.putIfAbsent: a key mapped to null counts as absent.
func mapPutIfAbsent_[K comparable, V any](c *tconv_[V], m map[K]V, k K, v V) V {
	old, ok := m[k]
	if ok && !c.isNull(old) {
		return old
	}
	m[k] = v
	if !ok {
		return c.null
	}
	return old
}

// mapComputeIfAbsent_ is Map.computeIfAbsent with an erased function.
func mapComputeIfAbsent_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], m map[K]V, k K, f Function_e) V {
	if v, ok := m[k]; ok && !vc.isNull(v) {
		return v
	}
	r := f.apply(kc.box(k))
	if r == nil {
		return vc.null
	}
	v := vc.unbox(r)
	m[k] = v
	return v
}

// setRemove_ is Set.remove used as a value: whether k was there.
func setRemove_[K comparable](m map[K]struct{}, k K) bool {
	if _, ok := m[k]; !ok {
		return false
	}
	delete(m, k)
	return true
}

// setAddAll_ is Set.addAll of another set: the result does not depend on either's order.
func setAddAll_[K comparable](m, from map[K]struct{}) bool {
	n := len(m)
	for k := range from { // order-independent: only membership changes
		m[k] = struct{}{}
	}
	return len(m) != n
}

// mapFrom_ is new LinkedHashMap<>(from) as a Go map.
func mapFrom_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], from Map) map[K]V {
	m := map[K]V{}
	for it := from.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		m[kc.unbox(e.getKey())] = vc.unbox(e.getValue())
	}
	return m
}

// mapImm_ is an ImmutableMap (or nil) as a Go map that nothing changes, made once per map.
func mapImm_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], from Map) map[K]V {
	if isNil_(from) {
		return nil
	}
	slot := immSlot_(from)
	if slot != nil && slot.conv == any(vc) {
		if m, ok := slot.v.(map[K]V); ok {
			return m
		}
	}
	m := mapFrom_(kc, vc, from)
	if slot != nil {
		*slot = immTyped_{vc, m}
	}
	return m
}

// setFrom_ is new LinkedHashSet<>(from) as a Go map.
func setFrom_[K comparable](kc *tconv_[K], from Collection) map[K]struct{} {
	src := toSlice_(from)
	m := make(map[K]struct{}, len(src))
	for _, x := range src {
		m[kc.unbox(x)] = struct{}{}
	}
	return m
}

// setImm_ is an ImmutableSet (or nil) as a Go map that nothing changes, made once per set.
func setImm_[K comparable](kc *tconv_[K], from Collection) map[K]struct{} {
	if isNil_(from) {
		return nil
	}
	slot := immSlot_(from)
	if slot != nil && slot.conv == any(kc) {
		if m, ok := slot.v.(map[K]struct{}); ok {
			return m
		}
	}
	m := setFrom_(kc, from)
	if slot != nil {
		*slot = immTyped_{kc, m}
	}
	return m
}

// mapComputeIfAbsentObj_ is Map_computeIfAbsent(m.asMap(), k, f) on a Go map: boxed key and
// result, as through the typed map's view.
func mapComputeIfAbsentObj_[K comparable, V any](kc *tconv_[K], vc *tconv_[V], m map[K]V, k any, f Function_e) any {
	if kk, ok := kc.try(k); ok {
		if old, ok := m[kk]; ok {
			if b := vc.box(old); b != nil {
				return b
			}
		}
	}
	v := f.apply(k)
	if v != nil {
		m[kc.unbox(k)] = vc.unbox(v)
	}
	return v
}

// setAddAllFrom_ is Set.addAll of an erased collection.
func setAddAllFrom_[K comparable](kc *tconv_[K], m map[K]struct{}, from Collection) bool {
	changed := false
	for _, x := range toSlice_(from) {
		if setAdd_(m, kc.unbox(x)) {
			changed = true
		}
	}
	return changed
}

// sliceOf_ is ImmutableList.of(items...) as a slice: a null element throws as Guava does.
func sliceOf_[T any](c *tconv_[T], items ...T) []T {
	for i, v := range items {
		if c.box(v) == nil {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
	return items
}

// sliceContains_ is ImmutableList.contains(o).
func sliceContains_[T any](c *tconv_[T], s []T, o any) bool {
	if s == nil {
		panic(new_NullPointerException_0().self())
	}
	if o == nil {
		return false // an ImmutableList has no null
	}
	for _, x := range s {
		if objequals_(o, c.box(x)) {
			return true
		}
	}
	return false
}

// sliceHasStr_ is Collection_contains_s on an ImmutableList of strings that is a slice.
func sliceHasStr_(s []string, k string) bool {
	if s == nil {
		panic(new_NullPointerException_0().self())
	}
	if k == nullstr_ {
		return false // an ImmutableList has no null
	}
	for _, x := range s {
		if x == k {
			return true
		}
	}
	return false
}

// sliceObj_ stands for slice s in a null check: nil if s is, else a value that is not nil.
func sliceObj_[T any](s []T) any {
	if s == nil {
		return nil
	}
	return true
}

// sliceNext_ is next() of ArrayList's iterator over a list that became slice s, with *i its
// cursor: the element at the cursor, which then moves on; NoSuchElementException at the end.
func sliceNext_[T any](s []T, i *int) T {
	if *i >= len(s) {
		panic(new_NoSuchElementException_0().self())
	}
	v := s[*i]
	*i++
	return v
}

// Deques (colls -kind deque) whose typed list became a slice, at the end of the slice (the
// head of a deque used only at its head, else the tail; see rt_tlist.go).

// slicePop_ is pop / removeFirst (removeLast at the tail): NoSuchElementException when empty.
func slicePop_[T any](c *tconv_[T], s *[]T) T {
	n := len(*s)
	if n == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	v := (*s)[n-1]
	var z T
	(*s)[n-1] = z
	*s = (*s)[:n-1]
	return v
}

// slicePollTop_ is poll / pollFirst: null when empty.
func slicePollTop_[T any](c *tconv_[T], s *[]T) T {
	if len(*s) == 0 {
		return c.null
	}
	return slicePop_(c, s)
}

// slicePeekTop_ is peek / peekFirst: null when empty.
func slicePeekTop_[T any](c *tconv_[T], s []T) T {
	if len(s) == 0 {
		return c.null
	}
	return s[len(s)-1]
}

// sliceTop_ is getFirst / element: NoSuchElementException when empty.
func sliceTop_[T any](c *tconv_[T], s []T) T {
	if len(s) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return s[len(s)-1]
}

// eachRevOf_ is TList.eachRev over a slice: the k-th step reads the k-th item from the end.
func eachRevOf_[T any](s *[]T) sliceEachRev_[T] { return sliceEachRev_[T]{src: s} }

// The head of a deque used at its tail as well (a queue) is index 0; the first item is dropped
// by moving the start of the slice, as ArrayDeque does.

// slicePopFirst_ is pop / removeFirst / remove(): NoSuchElementException when empty.
func slicePopFirst_[T any](c *tconv_[T], s *[]T) T {
	if len(*s) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	v := (*s)[0]
	var z T
	(*s)[0] = z
	*s = (*s)[1:]
	return v
}

// slicePollFirst_ is poll / pollFirst: null when empty.
func slicePollFirst_[T any](c *tconv_[T], s *[]T) T {
	if len(*s) == 0 {
		return c.null
	}
	return slicePopFirst_(c, s)
}

// slicePeekFirst_ is peek / peekFirst: null when empty.
func slicePeekFirst_[T any](c *tconv_[T], s []T) T {
	if len(s) == 0 {
		return c.null
	}
	return s[0]
}

// sliceFirst_ is getFirst / element: NoSuchElementException when empty.
func sliceFirst_[T any](c *tconv_[T], s []T) T {
	if len(s) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return s[0]
}
