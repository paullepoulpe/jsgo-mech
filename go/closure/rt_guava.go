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
func safeCompare_(c Comparator, a, b any) (r int32, ok bool) {
	defer func() {
		if recover() != nil {
			r, ok = 0, false
		}
	}()
	return c.compare(a, b), true
}

func cmpOrNatural_(c Comparator) Comparator {
	if c == nil {
		return naturalOrder_
	}
	return c
}

// comparatorOf_ is SortedSet.comparator() / SortedMap.comparator() for the
// sorted collections of this runtime (nil: natural ordering).
func comparatorOf_(v any) Comparator {
	switch x := v.(type) {
	case *TreeSet:
		return x.cmp
	case *EnumSet:
		return x.cmp
	case *TreeMap:
		return x.cmp
	case *EnumMap:
		return x.cmp
	case *ImmutableCollection:
		return x.cmp
	case *ImmutableMap:
		return x.cmp
	case interface{ comparator() Comparator }:
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
	items  []any      // list: the elements; otherwise the distinct elements in iteration order
	idx    *htable_   // set, multiset: element -> position in items (built on demand)
	counts []int32    // multiset: occurrences of items[i]
	total  int32      // multiset: size
	cmp    Comparator // sorted set (nil: natural ordering)
	list_  *ImmutableCollection
	all_   []any // multiset: the elements with repetition
	hash_  int32 // plain set: the hash code, once computed (Guava computes it at construction)
	hashed bool
}

type ImmutableList = ImmutableCollection
type ImmutableSet = ImmutableCollection
type ImmutableSet_CachingAsList = ImmutableCollection
type ImmutableSortedSet = ImmutableCollection
type ImmutableMultiset = ImmutableCollection
type ImmutableMultisetGwtSerializationDependencies = ImmutableCollection

func (this *ImmutableCollection) up_ImmutableCollection() *ImmutableCollection { return this }
func (this *ImmutableCollection) up_ImmutableSet() *ImmutableSet               { return this }

// immList_ makes an ImmutableList that owns items.
func immList_(items []any) *ImmutableList {
	if len(items) == 0 {
		return emptyImmList_
	}
	checkNoNulls_(items)
	return &ImmutableCollection{kind: icList_, items: items}
}

// immSet_ makes an ImmutableSet of the distinct elements, in first-seen order.
func immSet_(items []any) *ImmutableSet {
	if len(items) == 0 {
		return emptyImmSet_
	}
	checkNoNulls_(items)
	t := &htable_{}
	for _, v := range items {
		t.put(v, nil)
	}
	if t.live != len(items) {
		items = t.keys()
	}
	return &ImmutableCollection{kind: icSet_, items: items, idx: t}
}

// immSorted_ makes an ImmutableSortedSet; items is not modified.
func immSorted_(items []any, cmp Comparator) *ImmutableSortedSet {
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
	return &ImmutableCollection{kind: icSorted_, items: out, cmp: cmp}
}

func immMultiset_(items []any) *ImmutableMultiset {
	checkNoNulls_(items)
	this := &ImmutableCollection{kind: icMultiset_, idx: &htable_{}}
	for _, v := range items {
		if i := this.idx.find(v); i >= 0 {
			this.counts[i]++
		} else {
			this.idx.put(v, nil)
			this.items = append(this.items, v)
			this.counts = append(this.counts, 1)
		}
	}
	this.total = int32(len(items))
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
		c := cmpOrNatural_(this.cmp)
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
			t.put(x, nil)
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
	if this.all_ == nil && this.total > 0 {
		all := make([]any, 0, this.total)
		for i, v := range this.items {
			for j := int32(0); j < this.counts[i]; j++ {
				all = append(all, v)
			}
		}
		this.all_ = all
	}
	return this.all_
}

func (this *ImmutableCollection) size() int32 {
	if this.kind == icMultiset_ {
		return this.total
	}
	return int32(len(this.items))
}

func (this *ImmutableCollection) isEmpty() bool       { return len(this.items) == 0 }
func (this *ImmutableCollection) contains(v any) bool { return this.find_(v) >= 0 }

func (this *ImmutableCollection) containsAll(c Collection) bool {
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
func (this *ImmutableCollection) retainAll(c Collection) bool { throwUOE_(); return false }
func (this *ImmutableCollection) remove_Object(v any) bool    { throwUOE_(); return false }
func (this *ImmutableCollection) removeIf(p Predicate_base) bool   { throwUOE_(); return false }

func (this *ImmutableCollection) asList() *ImmutableList {
	if this.kind == icList_ {
		return this
	}
	if this.list_ == nil {
		this.list_ = &ImmutableCollection{kind: icList_, items: this.elems_()}
	}
	return this.list_
}

func (this *ImmutableCollection) equals(o any) bool {
	if o == nil {
		return false
	}
	if o == any(this) {
		return true
	}
	oc, isImm := o.(*ImmutableCollection)
	switch this.kind {
	case icList_:
		if isImm && oc.kind != icList_ {
			return false
		}
		return listEquals_(this.items, o)
	case icMultiset_:
		m, ok := o.(Multiset)
		if !ok || (isImm && oc.kind != icMultiset_) || m.size() != this.total {
			return false
		}
		for i, v := range this.items {
			if m.count(v) != this.counts[i] {
				return false
			}
		}
		return true
	}
	if isImm {
		if oc.kind != icSet_ && oc.kind != icSorted_ {
			return false
		}
	} else if isListLike_(o) || isMultiset_(o) {
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

// isListLike_ tells a java.util.List from the other collections (in Go every
// Collection satisfies the Set interface).
func isListLike_(o any) bool {
	if c, ok := o.(*ImmutableCollection); ok {
		return c.kind == icList_
	}
	_, ok := o.(List)
	return ok
}

func isMultiset_(o any) bool {
	if c, ok := o.(*ImmutableCollection); ok {
		return c.kind == icMultiset_
	}
	_, ok := o.(Multiset)
	return ok
}

func (this *ImmutableCollection) hashCode() int32 {
	switch this.kind {
	case icList_:
		return listHash_(this.items)
	case icMultiset_:
		var h int32
		for i, v := range this.items {
			h += objhash_(v) ^ this.counts[i]
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
		if this.kind == icMultiset_ && this.counts[i] != 1 {
			sb.WriteString(" x " + istr_(int64(this.counts[i])))
		}
	}
	sb.WriteByte(']')
	return sb.String()
}

// ---- ImmutableList

func (this *ImmutableList) get(i int32) any {
	s := this.elems_()
	if i < 0 || int(i) >= len(s) {
		panic(new_IndexOutOfBoundsException_String("Index " + istr_(int64(i)) + " out of bounds for length " + istr_(int64(len(s)))).self())
	}
	return s[i]
}

func (this *ImmutableList) indexOf(v any) int32 {
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

func (this *ImmutableList) lastIndexOf(v any) int32 {
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

func (this *ImmutableList) listIterator(i int32) ListIterator {
	s := this.elems_()
	if i < 0 || int(i) > len(s) {
		panic(new_IndexOutOfBoundsException_String("index (" + istr_(int64(i)) + ") must not be greater than size (" + istr_(int64(len(s))) + ")").self())
	}
	return &sliceIter_{items: &s, i: int(i)}
}

func (this *ImmutableList) reverse() *ImmutableList {
	s := this.elems_()
	out := make([]any, len(s))
	for i, v := range s {
		out[len(s)-1-i] = v
	}
	return &ImmutableCollection{kind: icList_, items: out}
}

func (this *ImmutableList) subList(a int32, b int32) List {
	s := this.elems_()
	if a < 0 || int(b) > len(s) || a > b {
		panic(new_IndexOutOfBoundsException_String("fromIndex: " + istr_(int64(a)) + ", toIndex: " + istr_(int64(b)) + ", size: " + istr_(int64(len(s)))).self())
	}
	if a == 0 && int(b) == len(s) && this.kind == icList_ {
		return this
	}
	return &ImmutableCollection{kind: icList_, items: s[a:b:b]}
}

func (this *ImmutableList) add_int_Object(i int32, v any) { throwUOE_() }
func (this *ImmutableList) remove(i int32) any            { throwUOE_(); return nil }
func (this *ImmutableList) set(i int32, v any) any        { throwUOE_(); return nil }
func (this *ImmutableList) addAll_int_Collection(i int32, c Collection) bool {
	throwUOE_()
	return false
}

// The empty immutable collections are singletons, as in Guava: translated code compares them by identity.
var emptyImmList_ = &ImmutableCollection{kind: icList_}
var emptyImmSet_ = &ImmutableCollection{kind: icSet_}
var emptyImmSorted_ = &ImmutableCollection{kind: icSorted_}

func ImmutableList_of() *ImmutableList { return emptyImmList_ }

func ImmutableList_of_Object(p0 any) *ImmutableList { return immList_([]any{p0}) }

func ImmutableList_of_Object_Object(p0 any, p1 any) *ImmutableList {
	return immList_([]any{p0, p1})
}

func ImmutableList_of_Object_Object_Object(p0 any, p1 any, p2 any) *ImmutableList {
	return immList_([]any{p0, p1, p2})
}

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

func ImmutableList_of_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_ObjectArr(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any, p6 any, p7 any, p8 any, p9 any, p10 any, p11 any, p12 []any) *ImmutableList {
	return immList_(append([]any{p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, p10, p11}, p12...))
}

func ImmutableList_copyOf(p0 Collection) *ImmutableList {
	if c, ok := p0.(*ImmutableCollection); ok {
		return c.asList()
	}
	return immList_(copySlice_(p0))
}

func ImmutableList_copyOf_Iterable(p0 Iterable) *ImmutableList {
	if c, ok := p0.(*ImmutableCollection); ok {
		return c.asList()
	}
	return immList_(copySlice_(p0))
}

func ImmutableList_copyOf_ObjectArr(p0 []any) *ImmutableList {
	return immList_(append([]any(nil), p0...))
}

func ImmutableList_sortedCopyOf(p0 Comparator, p1 Iterable) *ImmutableList {
	s := copySlice_(p1)
	checkNoNulls_(s)
	sortSlice_(s, p0)
	return immList_(s)
}

// ---- ImmutableSet

func ImmutableSet_of() *ImmutableSet { return emptyImmSet_ }

func ImmutableSet_of_Object(p0 any) *ImmutableSet { return immSet_([]any{p0}) }

func ImmutableSet_of_Object_Object(p0 any, p1 any) *ImmutableSet {
	return immSet_([]any{p0, p1})
}

func ImmutableSet_of_Object_Object_Object(p0 any, p1 any, p2 any) *ImmutableSet {
	return immSet_([]any{p0, p1, p2})
}

func ImmutableSet_of_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any) *ImmutableSet {
	return immSet_([]any{p0, p1, p2, p3})
}

func ImmutableSet_of_Object_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any, p4 any) *ImmutableSet {
	return immSet_([]any{p0, p1, p2, p3, p4})
}

func ImmutableSet_of_Object_Object_Object_Object_Object_Object_ObjectArr(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any, p6 []any) *ImmutableSet {
	return immSet_(append([]any{p0, p1, p2, p3, p4, p5}, p6...))
}

func ImmutableSet_copyOf(p0 Collection) *ImmutableSet {
	if c, ok := p0.(*ImmutableCollection); ok && c.kind == icSet_ {
		return c
	}
	return immSet_(copySlice_(p0))
}

func ImmutableSet_copyOf_Iterable(p0 Iterable) *ImmutableSet {
	if c, ok := p0.(*ImmutableCollection); ok && c.kind == icSet_ {
		return c
	}
	return immSet_(copySlice_(p0))
}

func ImmutableSet_copyOf_ObjectArr(p0 []any) *ImmutableSet {
	return immSet_(append([]any(nil), p0...))
}

// ---- ImmutableSortedSet

func ImmutableSortedSet_of() *ImmutableSortedSet { return emptyImmSorted_ }

func ImmutableSortedSet_copyOf(p0 Collection) *ImmutableSortedSet {
	if c, ok := p0.(*ImmutableCollection); ok && c.kind == icSorted_ && c.cmp == nil {
		return c
	}
	return immSorted_(toSlice_(p0), nil)
}

func ImmutableSortedSet_copyOf_Comparator_Collection(p0 Comparator, p1 Collection) *ImmutableSortedSet {
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

func (this *ImmutableSortedSet) comparator() Comparator { return cmpOrNatural_(this.cmp) }

func (this *ImmutableSortedSet) first() any {
	if len(this.items) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return this.items[0]
}

func (this *ImmutableSortedSet) last() any {
	if len(this.items) == 0 {
		panic(new_NoSuchElementException_0().self())
	}
	return this.items[len(this.items)-1]
}

func (this *ImmutableSortedSet) descendingSet() NavigableSet {
	out := make([]any, len(this.items))
	for i, v := range this.items {
		out[len(out)-1-i] = v
	}
	return &ImmutableCollection{kind: icSorted_, items: out, cmp: Comparator_reversed(cmpOrNatural_(this.cmp))}
}

// lowerBound_ is the first position whose element is >= v (> v if !inclusive).
func (this *ImmutableSortedSet) lowerBound_(v any, inclusive bool) int {
	if v == nil {
		throwNPE_("null")
	}
	c := cmpOrNatural_(this.cmp)
	return sort.Search(len(this.items), func(i int) bool {
		r := c.compare(this.items[i], v)
		if inclusive {
			return r >= 0
		}
		return r > 0
	})
}

func (this *ImmutableSortedSet) tailSet(from any, inclusive bool) NavigableSet {
	i := this.lowerBound_(from, inclusive)
	return &ImmutableCollection{kind: icSorted_, items: this.items[i:], cmp: this.cmp}
}

func (this *ImmutableSortedSet) headSet(to any, inclusive bool) NavigableSet {
	i := this.lowerBound_(to, !inclusive)
	return &ImmutableCollection{kind: icSorted_, items: this.items[:i:i], cmp: this.cmp}
}

// ---- ImmutableMultiset

type msEntry_ struct {
	elem  any
	count int32
}

func (e *msEntry_) getElement() any { return e.elem }
func (e *msEntry_) getCount() int32 { return e.count }
func (e *msEntry_) equals(o any) bool {
	x, ok := o.(Multiset_Entry)
	return ok && x != nil && x.getCount() == e.count && objequals_(e.elem, x.getElement())
}
func (e *msEntry_) hashCode() int32 { return objhash_(e.elem) ^ e.count }
func (e *msEntry_) toString() string {
	if e.count == 1 {
		return objstr_(e.elem)
	}
	return objstr_(e.elem) + " x " + istr_(int64(e.count))
}

func ImmutableMultiset_of() *ImmutableMultiset { return immMultiset_(nil) }

func ImmutableMultiset_copyOf(p0 Iterable) *ImmutableMultiset {
	if c, ok := p0.(*ImmutableCollection); ok && c.kind == icMultiset_ {
		return c
	}
	return immMultiset_(toSlice_(p0))
}

func (this *ImmutableMultiset) count(v any) int32 {
	switch this.kind {
	case icMultiset_:
		if i := this.find_(v); i >= 0 {
			return this.counts[i]
		}
		return 0
	case icList_:
		var n int32
		for _, x := range this.items {
			if objequals_(v, x) {
				n++
			}
		}
		return n
	}
	if this.find_(v) >= 0 {
		return 1
	}
	return 0
}

func (this *ImmutableMultiset) elementSet() Set {
	if this.kind == icSet_ || this.kind == icSorted_ {
		return this
	}
	if this.kind == icList_ {
		return immSet_(this.items)
	}
	return &ImmutableCollection{kind: icSet_, items: this.items, idx: this.idx}
}

func (this *ImmutableMultiset) entrySet() Set {
	out := make([]any, len(this.items))
	if this.kind == icList_ {
		return immMultiset_(this.items).entrySet()
	}
	for i, v := range this.items {
		var n int32 = 1
		if this.kind == icMultiset_ {
			n = this.counts[i]
		}
		out[i] = &msEntry_{elem: v, count: n}
	}
	return &ImmutableCollection{kind: icSet_, items: out}
}

func (this *ImmutableMultiset) add_Object_int(v any, n int32) int32 { throwUOE_(); return 0 }
func (this *ImmutableMultiset) setCount(v any, old int32, n int32) bool {
	throwUOE_()
	return false
}

// ---- builders

type ImmutableCollection_Builder struct {
	kind  int8
	items []any
	cmp   Comparator
}

type ImmutableList_Builder = ImmutableCollection_Builder
type ImmutableSet_Builder = ImmutableCollection_Builder
type ImmutableSortedSet_Builder = ImmutableCollection_Builder
type ImmutableMultiset_Builder = ImmutableCollection_Builder

func (this *ImmutableCollection_Builder) up_ImmutableCollection_Builder() *ImmutableCollection_Builder {
	return this
}
func (this *ImmutableCollection_Builder) up_ImmutableSet_Builder() *ImmutableSet_Builder {
	return this
}

func new_ImmutableList_Builder() *ImmutableList_Builder {
	return &ImmutableCollection_Builder{kind: icList_}
}
func new_ImmutableSet_Builder() *ImmutableSet_Builder {
	return &ImmutableCollection_Builder{kind: icSet_}
}
func ImmutableList_builder() *ImmutableList_Builder { return new_ImmutableList_Builder() }
func ImmutableList_builderWithExpectedSize(p0 int32) *ImmutableList_Builder {
	return &ImmutableCollection_Builder{kind: icList_, items: make([]any, 0, p0)}
}
func ImmutableSet_builder() *ImmutableSet_Builder { return new_ImmutableSet_Builder() }
func ImmutableSet_builderWithExpectedSize(p0 int32) *ImmutableSet_Builder {
	return &ImmutableCollection_Builder{kind: icSet_, items: make([]any, 0, p0)}
}
func ImmutableSortedSet_naturalOrder() *ImmutableSortedSet_Builder {
	return &ImmutableCollection_Builder{kind: icSorted_}
}
func ImmutableSortedSet_orderedBy(p0 Comparator) *ImmutableSortedSet_Builder {
	return &ImmutableCollection_Builder{kind: icSorted_, cmp: p0}
}
func ImmutableMultiset_builder() *ImmutableMultiset_Builder {
	return &ImmutableCollection_Builder{kind: icMultiset_}
}

func (this *ImmutableCollection_Builder) add(p0 any) *ImmutableCollection_Builder {
	if p0 == nil {
		throwNPE_("at index " + istr_(int64(len(this.items))))
	}
	this.items = append(this.items, p0)
	return this
}

func (this *ImmutableCollection_Builder) add_ObjectArr(p0 []any) *ImmutableCollection_Builder {
	for _, v := range p0 {
		this.add(v)
	}
	return this
}

func (this *ImmutableCollection_Builder) addAll(p0 Iterable) *ImmutableCollection_Builder {
	for _, v := range toSlice_(p0) {
		this.add(v)
	}
	return this
}

func (this *ImmutableCollection_Builder) addAll_Iterator(p0 Iterator) *ImmutableCollection_Builder {
	for p0.hasNext() {
		this.add(p0.next())
	}
	return this
}

func (this *ImmutableCollection_Builder) addCopies(p0 any, n int32) *ImmutableCollection_Builder {
	for i := int32(0); i < n; i++ {
		this.add(p0)
	}
	return this
}

func (this *ImmutableCollection_Builder) build() *ImmutableCollection {
	items := append([]any(nil), this.items...)
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

