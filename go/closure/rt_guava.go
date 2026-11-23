package closure

// Guava immutable collections (ImmutableList, ImmutableSet, ImmutableSortedSet,
// ImmutableMultiset) and the helpers shared by the other rt_guava*.go files.
//
// The translated code converts between ImmutableCollection and its subclasses
// with plain Go type assertions (a builder's build() is declared on
// ImmutableCollection.Builder and its result is asserted to *ImmutableSet), so
// the whole family is ONE struct with a kind tag, and the subclass names are
// aliases of it. The same holds for the builders.

import (
	"sort"
	"strings"
)

func throwUOE_() {
	panic(new_UnsupportedOperationException_0().self())
}

func throwNPE_(msg string) {
	panic(new_NullPointerException_String(msg).self())
}

func throwIAE_(msg string) {
	panic(new_IllegalArgumentException_String(msg).self())
}

func checkNoNulls_(items []any) {
	for i, v := range items {
		if v == nil {
			throwNPE_("at index " + istr_(int64(i)))
		}
	}
}

// copySlice_ is a private copy of an Iterable's elements.
func copySlice_(c Iterable) []any {
	return append([]any(nil), toSlice_(c)...)
}

// safeCompare_ compares with c, reporting ok=false where Java would throw
// ClassCastException or NullPointerException from the comparator.
func safeCompare_(c Comparator_e, a, b any) (r int32, ok bool) {
	defer func() {
		if recover() != nil {
			r, ok = 0, false
		}
	}()
	return c.compare(a, b), true
}

func cmpOrNatural_(c Comparator_e) Comparator_e {
	if c == nil {
		return naturalOrder_
	}
	return c
}

// comparatorOf_ is SortedSet.comparator() / SortedMap.comparator() for the
// sorted collections of this runtime (nil: natural ordering).
func comparatorOf_(v any) Comparator_e {
	switch x := v.(type) {
	case *TreeSet:
		return x.cmp
	case *TreeMap:
		return x.cmp
	case *ImmutableCollection:
		return x.cmp_()
	case *ErasedImmutableMap:
		return x.cmp
	case interface{ comparator() Comparator_e }:
		return x.comparator()
	}
	return nil
}

// ---- immutable entry

type immEntry_ struct {
	key, val any
}

func (e *immEntry_) getKey() any        { return e.key }
func (e *immEntry_) getValue() any      { return e.val }
func (e *immEntry_) setValue(v any) any { throwUOE_(); return nil }
func (e *immEntry_) equals(o any) bool {
	x, ok := o.(Map_Entry)
	return ok && x != nil && objequals_(e.key, x.getKey()) && objequals_(e.val, x.getValue())
}
func (e *immEntry_) hashCode() int32  { return objhash_(e.key) ^ objhash_(e.val) }
func (e *immEntry_) toString() string { return objstr_(e.key) + "=" + objstr_(e.val) }

// ---- ImmutableCollection and its aliases

const (
	icList_ int8 = iota
	icSet_
	icSorted_
	icMultiset_
)

type ImmutableCollection struct {
	kind   int8
	hashed bool
	hash_  int32     // plain set: the hash code, once computed (Guava computes it at construction)
	items  []any     // list: the elements; otherwise the distinct elements in iteration order
	idx    *htable_  // set, multiset: element -> position in items (built on demand)
	typed_ immTyped_ // its typed copy (post-flip typed collections), made on first use
	// What only some kinds have, nil otherwise: most immutable collections are short lists
	// and plain sets, made in great numbers, and stay 80 bytes instead of 192.
	rare *immRare_
}

// immRare_ holds the fields of an ImmutableCollection that a list or a plain set does not use
// (a set only once its asList() is asked for).
type immRare_ struct {
	counts []int32      // multiset: occurrences of items[i]
	total  int32        // multiset: size
	cmp    Comparator_e // sorted set (nil: natural ordering)
	list_  *ImmutableCollection
	all_   []any // multiset: the elements with repetition
	byOrd  []any // immutable enum set: the element with ordinal i at i (nil where absent)
}

// rare_ is the collection's immRare_, made if it has none yet.
func (this *ImmutableCollection) rare_() *immRare_ {
	if this.rare == nil {
		this.rare = &immRare_{}
	}
	return this.rare
}

// cmp_ is the comparator of a sorted set (nil: natural ordering).
func (this *ImmutableCollection) cmp_() Comparator_e {
	if this.rare == nil {
		return nil
	}
	return this.rare.cmp
}

// byOrd_ is the by-ordinal table of an immutable enum set, nil for any other collection.
func (this *ImmutableCollection) byOrd_() []any {
	if this.rare == nil {
		return nil
	}
	return this.rare.byOrd
}

// immSortedOf_ is a sorted set of items (in order) under cmp.
func immSortedOf_(items []any, cmp Comparator_e) *ImmutableCollection {
	this := &ImmutableCollection{kind: icSorted_, items: items}
	if cmp != nil {
		this.rare = &immRare_{cmp: cmp}
	}
	return this
}

type ErasedImmutableList = ImmutableCollection
type ErasedImmutableSet = ImmutableCollection
type ImmutableSet_CachingAsList = ImmutableCollection
type ImmutableSortedSet = ImmutableCollection
type ImmutableMultiset = ImmutableCollection
type ImmutableMultisetGwtSerializationDependencies = ImmutableCollection

func (this *ImmutableCollection) up_ImmutableCollection() *ImmutableCollection { return this }

// immList_ makes an ImmutableList that owns items.
func immList_(items []any) *ErasedImmutableList {
	if len(items) == 0 {
		return emptyImmList_
	}
	checkNoNulls_(items)
	return &ImmutableCollection{kind: icList_, items: items}
}

// immSet_ makes an ImmutableSet of the distinct elements, in first-seen order.
func immSet_(items []any) *ErasedImmutableSet {
	if len(items) == 0 {
		return emptyImmSet_
	}
	// Guava's ImmutableSet checks its elements with checkNotNull: no message
	for _, v := range items {
		if v == nil {
			panic(new_NullPointerException_0().self())
		}
	}
	// The index's own key slice is the element slice (an immutable set never has removed entries).
	t := &htable_{keys: make([]any, 0, len(items))}
	for _, v := range items {
		t.add(v)
	}
	return &ImmutableCollection{kind: icSet_, items: t.keys, idx: t}
}

// immSorted_ makes an ImmutableSortedSet; items is not modified.
func immSorted_(items []any, cmp Comparator_e) *ImmutableSortedSet {
	checkNoNulls_(items)
	c := cmpOrNatural_(cmp)
	s := append([]any(nil), items...)
	sortSlice_(s, c)
	out := s[:0]
	for _, v := range s {
		if len(out) == 0 || c.compare(out[len(out)-1], v) != 0 {
			out = append(out, v)
		}
	}
	return immSortedOf_(out, cmp)
}

func immMultiset_(items []any) *ImmutableMultiset {
	checkNoNulls_(items)
	this := &ImmutableCollection{kind: icMultiset_, idx: &htable_{}, rare: &immRare_{}}
	for _, v := range items {
		if i := this.idx.find(v); i >= 0 {
			this.rare.counts[i]++
		} else {
			this.idx.add(v)
			this.items = append(this.items, v)
			this.rare.counts = append(this.rare.counts, 1)
		}
	}
	this.rare.total = int32(len(items))
	return this
}

// find_ is the position of v in items, or -1.
func (this *ImmutableCollection) find_(v any) int {
	if v == nil || len(this.items) == 0 {
		return -1
	}
	switch this.kind {
	case icList_:
		for i, x := range this.items {
			if objequals_(v, x) {
				return i
			}
		}
		return -1
	case icSorted_:
		c := cmpOrNatural_(this.cmp_())
		lo, hi := 0, len(this.items)
		for lo < hi {
			mid := (lo + hi) / 2
			r, ok := safeCompare_(c, this.items[mid], v)
			if !ok {
				return -1
			}
			if r == 0 {
				return mid
			}
			if r < 0 {
				lo = mid + 1
			} else {
				hi = mid
			}
		}
		return -1
	}
	if this.idx == nil {
		t := &htable_{}
		for _, x := range this.items {
			t.add(x)
		}
		this.idx = t
	}
	return this.idx.find(v)
}

// elems_ is the elements as iterated (shared; do not modify).
func (this *ImmutableCollection) elems_() []any {
	if this.kind != icMultiset_ {
		return this.items
	}
	if r := this.rare; r.all_ == nil && r.total > 0 {
		all := make([]any, 0, r.total)
		for i, v := range this.items {
			for j := int32(0); j < r.counts[i]; j++ {
				all = append(all, v)
			}
		}
		r.all_ = all
	}
	return this.rare.all_
}

func (this *ImmutableCollection) size() int32 {
	if this.kind == icMultiset_ {
		return this.rare.total
	}
	return int32(len(this.items))
}

func (this *ImmutableCollection) isEmpty() bool { return len(this.items) == 0 }
func (this *ImmutableCollection) contains(v any) bool {
	if byOrd := this.byOrd_(); byOrd != nil {
		// Enum constants are equal only to themselves.
		e, ok := v.(interface{ ordinal() int32 })
		if !ok {
			return false
		}
		o := int(e.ordinal())
		return o < len(byOrd) && byOrd[o] == v
	}
	return this.find_(v) >= 0
}

func (this *ImmutableCollection) contains_s(v string) bool {
	if len(this.items) == 0 {
		return false // find_ is -1, and v need not be boxed for it
	}
	if this.kind == icList_ {
		for _, x := range this.items {
			if s, ok := x.(string); ok && s == v {
				return true
			}
		}
		return false
	}
	if this.kind == icSorted_ || len(this.items) == 0 {
		return this.find_(v) >= 0
	}
	if this.idx == nil {
		this.find_(v) // builds the index
	}
	return this.idx.findStr(v) >= 0
}

func (this *ImmutableCollection) containsAll(c Collection) bool {
	if e, ok := c.(*EnumSet); ok && this.byOrd_() != nil {
		// An immutable enum set and an EnumSet: each constant of e is the one at its ordinal here.
		for it := e.iter(); it.hasNext(); {
			if !this.contains(it.next()) {
				return false
			}
		}
		return true
	}
	for it := c.iterator(); it.hasNext(); {
		if this.find_(it.next()) < 0 {
			return false
		}
	}
	return true
}

func (this *ImmutableCollection) iterator() Iterator {
	s := this.elems_()
	return &sliceIter_{items: &s}
}

func (this *ImmutableCollection) toArray_0() []any {
	return append([]any(nil), this.elems_()...)
}

func (this *ImmutableCollection) toArray(a []any) []any { return fillArray_(this.elems_(), a) }

func (this *ImmutableCollection) add(v any) bool              { throwUOE_(); return false }
func (this *ImmutableCollection) addAll(c Collection) bool    { throwUOE_(); return false }
func (this *ImmutableCollection) clear()                      { throwUOE_() }
func (this *ImmutableCollection) removeAll(c Collection) bool { throwUOE_(); return false }
func (this *ImmutableCollection) remove_Object(v any) bool    { throwUOE_(); return false }
func (this *ImmutableCollection) asList() *ErasedImmutableList {
	if this.kind == icList_ {
		return this
	}
	r := this.rare_()
	if r.list_ == nil {
		r.list_ = &ImmutableCollection{kind: icList_, items: this.elems_()}
	}
	return r.list_
}

// isListLike_ tells a java.util.List from the other collections (in Go every
// Collection satisfies the Set interface).
func isListLike_(o any) bool {
	if c, ok := o.(*ImmutableCollection); ok {
		return c.kind == icList_
	}
	_, ok := o.(List)
	return ok
}

func (this *ImmutableCollection) hashCode() int32 {
	switch this.kind {
	case icList_:
		return listHash_(this.items)
	case icMultiset_:
		var h int32
		for i, v := range this.items {
			h += objhash_(v) ^ this.rare.counts[i]
		}
		return h
	}
	if this.kind == icSet_ && this.hashed {
		return this.hash_
	}
	var h int32
	for _, v := range this.items {
		h += objhash_(v)
	}
	if this.kind == icSet_ {
		this.hash_, this.hashed = h, true
	}
	return h
}

func (this *ImmutableCollection) toString() string {
	var sb strings.Builder
	sb.WriteByte('[')
	for i, v := range this.items {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString(objstr_(v))
		if this.kind == icMultiset_ && this.rare.counts[i] != 1 {
			sb.WriteString(" x " + istr_(int64(this.rare.counts[i])))
		}
	}
	sb.WriteByte(']')
	return sb.String()
}

// ---- ImmutableList

func (this *ErasedImmutableList) get(i int32) any {
	s := this.elems_()
	if i < 0 || int(i) >= len(s) {
		panic(new_IndexOutOfBoundsException_String("Index " + istr_(int64(i)) + " out of bounds for length " + istr_(int64(len(s)))).self())
	}
	return s[i]
}

func (this *ErasedImmutableList) indexOf(v any) int32 {
	if v == nil {
		return -1
	}
	for i, x := range this.elems_() {
		if objequals_(v, x) {
			return int32(i)
		}
	}
	return -1
}

func (this *ErasedImmutableList) lastIndexOf(v any) int32 {
	if v == nil {
		return -1
	}
	s := this.elems_()
	for i := len(s) - 1; i >= 0; i-- {
		if objequals_(v, s[i]) {
			return int32(i)
		}
	}
	return -1
}

func (this *ErasedImmutableList) listIterator(i int32) ListIterator {
	s := this.elems_()
	if i < 0 || int(i) > len(s) {
		panic(new_IndexOutOfBoundsException_String("index (" + istr_(int64(i)) + ") must not be greater than size (" + istr_(int64(len(s))) + ")").self())
	}
	return &sliceIter_{items: &s, i: int(i)}
}

func (this *ErasedImmutableList) reverse() *ErasedImmutableList {
	s := this.elems_()
	out := make([]any, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return &ImmutableCollection{kind: icList_, items: out}
}

func (this *ErasedImmutableList) subList(a int32, b int32) List {
	s := this.elems_()
	if a < 0 || int(b) > len(s) || a > b {
		panic(new_IndexOutOfBoundsException_String("fromIndex: " + istr_(int64(a)) + ", toIndex: " + istr_(int64(b)) + ", size: " + istr_(int64(len(s)))).self())
	}
	if a == 0 && int(b) == len(s) && this.kind == icList_ {
		return this
	}
	return &ImmutableCollection{kind: icList_, items: s[a:b:b]}
}

func (this *ErasedImmutableList) add_int_Object(i int32, v any) { throwUOE_() }
func (this *ErasedImmutableList) remove(i int32) any            { throwUOE_(); return nil }
func (this *ErasedImmutableList) set(i int32, v any) any        { throwUOE_(); return nil }

// The empty immutable collections are singletons, as in Guava: translated code compares them by identity.
var emptyImmList_ = &ImmutableCollection{kind: icList_}
var emptyImmSet_ = &ImmutableCollection{kind: icSet_}
var emptyImmSorted_ = &ImmutableCollection{kind: icSorted_}

func ImmutableList_of() *ErasedImmutableList { return emptyImmList_ }

func ImmutableList_of_Object(p0 any) *ErasedImmutableList { return immList_([]any{p0}) }

func ImmutableList_of_Object_Object(p0 any, p1 any) *ErasedImmutableList {
	return immList_([]any{p0, p1})
}

func ImmutableList_copyOf(p0 Collection) *ErasedImmutableList {
	if c, ok := p0.(*ImmutableCollection); ok {
		return c.asList()
	}
	return immList_(copySlice_(p0))
}

func ImmutableList_copyOf_Iterable(p0 Iterable) *ErasedImmutableList {
	if c, ok := p0.(*ImmutableCollection); ok {
		return c.asList()
	}
	return immList_(copySlice_(p0))
}

func ImmutableList_sortedCopyOf(p0 Comparator_e, p1 Iterable) *ErasedImmutableList {
	s := copySlice_(p1)
	checkNoNulls_(s)
	sortSlice_(s, p0)
	return immList_(s)
}

// ---- ImmutableSet

func ImmutableSet_of() *ErasedImmutableSet { return emptyImmSet_ }

func ImmutableSet_of_Object(p0 any) *ErasedImmutableSet { return immSet_([]any{p0}) }

func ImmutableSet_of_Object_Object(p0 any, p1 any) *ErasedImmutableSet {
	return immSet_([]any{p0, p1})
}

func ImmutableSet_of_Object_Object_Object(p0 any, p1 any, p2 any) *ErasedImmutableSet {
	return immSet_([]any{p0, p1, p2})
}

func ImmutableSet_copyOf(p0 Collection) *ErasedImmutableSet {
	if c, ok := p0.(*ImmutableCollection); ok && c.kind == icSet_ {
		return c
	}
	if e, ok := p0.(*EnumSet); ok {
		return immEnumSet_(e)
	}
	return immSet_(copySlice_(p0))
}

// immEnumSet_ is Guava's copy of an EnumSet, an ImmutableEnumSet: the constants in ordinal order
// (the EnumSet's iteration order), looked up by ordinal.
func immEnumSet_(e *EnumSet) *ErasedImmutableSet {
	return withOrdinals_(immSet_(e.items()))
}

// withOrdinals_ indexes a non-empty immutable set of enum constants of one enum by ordinal, for
// contains and containsAll; the set is otherwise unchanged.
func withOrdinals_(set *ErasedImmutableSet) *ErasedImmutableSet {
	if len(set.items) == 0 {
		return set // the shared empty set
	}
	last := int32(0)
	for _, x := range set.items {
		last = max(last, x.(interface{ ordinal() int32 }).ordinal())
	}
	byOrd := make([]any, last+1)
	set.rare_().byOrd = byOrd
	for _, x := range set.items {
		byOrd[x.(interface{ ordinal() int32 }).ordinal()] = x
	}
	return set
}

func ImmutableSet_copyOf_ObjectArr(p0 []any) *ErasedImmutableSet {
	return immSet_(append([]any(nil), p0...))
}

// ---- ImmutableSortedSet

func ImmutableSortedSet_of() *ImmutableSortedSet { return emptyImmSorted_ }

func ImmutableSortedSet_copyOf(p0 Collection) *ImmutableSortedSet {
	if c, ok := p0.(*ImmutableCollection); ok && c.kind == icSorted_ && c.cmp_() == nil {
		return c
	}
	return immSorted_(toSlice_(p0), nil)
}

func ImmutableSortedSet_copyOf_Comparator_Collection(p0 Comparator_e, p1 Collection) *ImmutableSortedSet {
	if p0 == nil {
		throwNPE_("comparator")
	}
	return immSorted_(toSlice_(p1), p0)
}

func ImmutableSortedSet_copyOfSorted(p0 SortedSet) *ImmutableSortedSet {
	if c, ok := p0.(*ImmutableCollection); ok && c.kind == icSorted_ {
		return c
	}
	return immSorted_(toSlice_(p0), comparatorOf_(p0))
}

func (this *ImmutableSortedSet) comparator() Comparator_e { return cmpOrNatural_(this.cmp_()) }

func (this *ImmutableSortedSet) descendingSet() NavigableSet {
	out := make([]any, len(this.items))
	for i, v := range this.items {
		out[len(out)-1-i] = v
	}
	return immSortedOf_(out, cmpReversed_(cmpOrNatural_(this.cmp_())))
}

func (this *ImmutableSortedSet) tailSet(from any, inclusive bool) NavigableSet {
	i := this.lowerBound_(from, inclusive)
	return immSortedOf_(this.items[i:], this.cmp_())
}

// ---- ImmutableMultiset

type msEntry_ struct {
	elem  any
	count int32
}

// ---- builders

type ImmutableCollection_Builder struct {
	kind  int8
	items []any
	cmp   Comparator_e
}

type ErasedImmutableList_Builder = ImmutableCollection_Builder
type ErasedImmutableSet_Builder = ImmutableCollection_Builder
type ImmutableSortedSet_Builder = ImmutableCollection_Builder
type ImmutableMultiset_Builder = ImmutableCollection_Builder

func new_ImmutableList_Builder() *ErasedImmutableList_Builder {
	return &ImmutableCollection_Builder{kind: icList_}
}
func new_ImmutableSet_Builder() *ErasedImmutableSet_Builder {
	return &ImmutableCollection_Builder{kind: icSet_}
}
func ImmutableList_builder() *ErasedImmutableList_Builder { return new_ImmutableList_Builder() }
func ImmutableSet_builder() *ErasedImmutableSet_Builder   { return new_ImmutableSet_Builder() }
func ImmutableSortedSet_naturalOrder() *ImmutableSortedSet_Builder {
	return &ImmutableCollection_Builder{kind: icSorted_}
}
func ImmutableSortedSet_orderedBy(p0 Comparator_e) *ImmutableSortedSet_Builder {
	return &ImmutableCollection_Builder{kind: icSorted_, cmp: p0}
}

// add throws the NullPointerException of Guava's checkNotNull for null: no message.
func (this *ImmutableCollection_Builder) add(p0 any) *ImmutableCollection_Builder {
	if p0 == nil {
		panic(new_NullPointerException_0().self())
	}
	if cap(this.items) == 0 {
		this.items = make([]any, 0, 4) // as Guava's builder starts
	}
	this.items = append(this.items, p0)
	return this
}

// add_ObjectArr is add(E...): the builder of a list checks the elements first, naming the
// position of a null in them.
func (this *ImmutableCollection_Builder) add_ObjectArr(p0 []any) *ImmutableCollection_Builder {
	if this.kind == icList_ {
		checkNoNulls_(p0)
	}
	for _, v := range p0 {
		this.add(v)
	}
	return this
}

func (this *ImmutableCollection_Builder) addAll(p0 Iterable) *ImmutableCollection_Builder {
	for _, v := range readSlice_(p0) {
		this.add(v)
	}
	return this
}

// build shares the elements so far with the collection built, as Guava's builder does: the
// builder only appends, and the capacity is cut so that later adds never reach it.
func (this *ImmutableCollection_Builder) build() *ImmutableCollection {
	items := this.items[:len(this.items):len(this.items)]
	switch this.kind {
	case icSet_:
		return immSet_(items)
	case icSorted_:
		return immSorted_(items, this.cmp)
	case icMultiset_:
		return immMultiset_(items)
	}
	return immList_(items)
}

// ---- collectors
//
// java.util.stream.Collector has no methods the translated code calls, so the
// Go interface is empty; these collectors expose the Java accessor methods.

func (this *ImmutableCollection) equals(o any) bool {
	if o == nil {
		return false
	}
	if o == any(this) {
		return true
	}
	oc, isImm := o.(*ImmutableCollection)
	if this.kind == icList_ {
		if isImm && oc.kind != icList_ {
			return false
		}
		return listEquals_(this.items, o)
	}
	if isImm {
		if oc.kind != icSet_ && oc.kind != icSorted_ {
			return false
		}
	} else if isListLike_(o) {
		return false
	}
	s, ok := o.(Set)
	if !ok || int(s.size()) != len(this.items) {
		return false
	}
	for it := s.iterator(); it.hasNext(); {
		if this.find_(it.next()) < 0 {
			return false
		}
	}
	return true
}

func (this *ImmutableMultiset) entrySet() Set {
	out := make([]any, len(this.items))
	if this.kind == icList_ {
		return immMultiset_(this.items).entrySet()
	}
	for i, v := range this.items {
		var n int32 = 1
		if this.kind == icMultiset_ {
			n = this.rare.counts[i]
		}
		out[i] = &msEntry_{elem: v, count: n}
	}
	return &ImmutableCollection{kind: icSet_, items: out}
}

// lowerBound_ is the first position whose element is >= v (> v if !inclusive).
func (this *ImmutableSortedSet) lowerBound_(v any, inclusive bool) int {
	if v == nil {
		throwNPE_("null")
	}
	c := cmpOrNatural_(this.cmp_())
	return sort.Search(len(this.items), func(i int) bool {
		r := c.compare(this.items[i], v)
		if inclusive {
			return r >= 0
		}
		return r > 0
	})
}
