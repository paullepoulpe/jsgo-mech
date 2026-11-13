package closure

// Guava ImmutableMap, ImmutableSortedMap, ImmutableBiMap and the immutable
// multimaps. As with ImmutableCollection (see rt_guava.go) each family is one
// struct and the subclass names are aliases.

import "sort"

const (
	imMap_ int8 = iota
	imBiMap_
	imSorted_
)

type ImmutableMap struct {
	kind     int8
	keys     []any
	vals     []any
	idx      *htable_   // key -> position (nil for sorted maps and until first lookup)
	cmp      Comparator // sorted map (nil: natural ordering)
	inv      *ImmutableMap
	keySet_  *ImmutableCollection
	values_  *ImmutableCollection
	entries_ *ImmutableCollection
}

type ImmutableBiMap = ImmutableMap
type ImmutableSortedMap = ImmutableMap

func (this *ImmutableMap) up_ImmutableMap() *ImmutableMap { return this }

func dupKey_(k, v1, v2 any) {
	throwIAE_("Multiple entries with same key: " + objstr_(k) + "=" + objstr_(v1) + " and " + objstr_(k) + "=" + objstr_(v2))
}

// immMap_ builds a map of the given kind from parallel key/value slices, which
// it owns. Duplicate keys throw, or with keepLast keep the last entry.
func immMap_(kind int8, keys, vals []any, cmp Comparator, keepLast bool) *ImmutableMap {
	if len(keys) == 0 && kind == emptyImmMap_.kind && cmp == nil {
		return emptyImmMap_
	}
	for i := range keys {
		if keys[i] == nil {
			throwNPE_("null key in entry: null=" + objstr_(vals[i]))
		}
		if vals[i] == nil {
			throwNPE_("null value in entry: " + objstr_(keys[i]) + "=null")
		}
	}
	this := &ImmutableMap{kind: kind, cmp: cmp}
	if kind == imSorted_ {
		c := cmpOrNatural_(cmp)
		order := make([]int, len(keys))
		for i := range order {
			order[i] = i
		}
		sort.SliceStable(order, func(a, b int) bool { return c.compare(keys[order[a]], keys[order[b]]) < 0 })
		for _, i := range order {
			n := len(this.keys)
			if n > 0 && c.compare(this.keys[n-1], keys[i]) == 0 {
				if !keepLast {
					dupKey_(keys[i], this.vals[n-1], vals[i])
				}
				this.vals[n-1] = vals[i]
				continue
			}
			this.keys = append(this.keys, keys[i])
			this.vals = append(this.vals, vals[i])
		}
		return this
	}
	t := &htable_{}
	if keepLast {
		// The surviving entry of a duplicated key is its last one, at that entry's position.
		drop := make([]bool, len(keys))
		dups := false
		for i := len(keys) - 1; i >= 0; i-- {
			if _, existed := t.put(keys[i], nil); existed {
				drop[i] = true
				dups = true
			}
		}
		if dups {
			var k2, v2 []any
			for i := range keys {
				if !drop[i] {
					k2 = append(k2, keys[i])
					v2 = append(v2, vals[i])
				}
			}
			keys, vals = k2, v2
		}
		t = &htable_{}
	}
	for i, k := range keys {
		if j := t.find(k); j >= 0 {
			dupKey_(k, vals[j], vals[i])
		}
		t.put(k, nil)
	}
	this.keys, this.vals, this.idx = keys, vals, t
	if kind == imBiMap_ {
		vt := &htable_{}
		for i, v := range vals {
			if j := vt.find(v); j >= 0 {
				throwIAE_("Multiple entries with same value: " + objstr_(keys[j]) + "=" + objstr_(v) + " and " + objstr_(keys[i]) + "=" + objstr_(v))
			}
			vt.put(v, nil)
		}
		this.inv = &ImmutableMap{kind: imBiMap_, keys: vals, vals: keys, idx: vt, inv: this}
	}
	return this
}

func (this *ImmutableMap) find_(k any) int {
	if k == nil || len(this.keys) == 0 {
		return -1
	}
	if this.kind == imSorted_ {
		c := cmpOrNatural_(this.cmp)
		lo, hi := 0, len(this.keys)
		for lo < hi {
			mid := (lo + hi) / 2
			r, ok := safeCompare_(c, this.keys[mid], k)
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
	return this.index_().find(k)
}

func (this *ImmutableMap) index_() *htable_ {
	if this.idx == nil {
		t := &htable_{}
		for _, x := range this.keys {
			t.put(x, nil)
		}
		this.idx = t
	}
	return this.idx
}

func (this *ImmutableMap) size() int32   { return int32(len(this.keys)) }
func (this *ImmutableMap) isEmpty() bool { return len(this.keys) == 0 }

func (this *ImmutableMap) get(k any) any {
	if i := this.find_(k); i >= 0 {
		return this.vals[i]
	}
	return nil
}

func (this *ImmutableMap) containsKey(k any) bool { return this.find_(k) >= 0 }

func (this *ImmutableMap) containsValue(v any) bool {
	if v == nil {
		return false
	}
	if this.inv != nil {
		return this.inv.find_(v) >= 0
	}
	for _, x := range this.vals {
		if objequals_(v, x) {
			return true
		}
	}
	return false
}

func (this *ImmutableMap) keySet() Set {
	if this.keySet_ == nil {
		if this.kind == imSorted_ {
			this.keySet_ = &ImmutableCollection{kind: icSorted_, items: this.keys, cmp: this.cmp}
		} else {
			this.keySet_ = &ImmutableCollection{kind: icSet_, items: this.keys, idx: this.index_()}
		}
	}
	return this.keySet_
}

func (this *ImmutableMap) values() Collection {
	if this.values_ == nil {
		if this.inv != nil {
			this.values_ = &ImmutableCollection{kind: icSet_, items: this.vals, idx: this.inv.index_()}
		} else {
			this.values_ = &ImmutableCollection{kind: icList_, items: this.vals}
		}
	}
	return this.values_
}

func (this *ImmutableMap) entrySet() Set {
	if this.entries_ == nil {
		es := make([]any, len(this.keys))
		for i, k := range this.keys {
			es[i] = &immEntry_{key: k, val: this.vals[i]}
		}
		this.entries_ = &ImmutableCollection{kind: icSet_, items: es}
	}
	return this.entries_
}

func (this *ImmutableMap) put(k any, v any) any { throwUOE_(); return nil }
func (this *ImmutableMap) putAll(m Map)         { throwUOE_() }
func (this *ImmutableMap) remove(k any) any     { throwUOE_(); return nil }
func (this *ImmutableMap) clear()               { throwUOE_() }

func (this *ImmutableMap) equals(o any) bool { return mapEquals_(this, o) }
func (this *ImmutableMap) hashCode() int32 {
	var h int32
	for i, k := range this.keys {
		h += objhash_(k) ^ objhash_(this.vals[i])
	}
	return h
}
func (this *ImmutableMap) toString() string { return mapToString_(this) }

func (this *ImmutableBiMap) inverse() BiMap {
	if this.inv == nil {
		// A plain ImmutableMap asked for its inverse: build one, checking value uniqueness.
		b := immMap_(imBiMap_, this.keys, this.vals, nil, false)
		this.inv = b.inv
	}
	return this.inv
}

func (this *ImmutableSortedMap) comparator() Comparator { return cmpOrNatural_(this.cmp) }

// mapEntries_ copies a Map's entries to parallel slices.
func mapKV_(m Map) (keys, vals []any) {
	if im, ok := m.(*ImmutableMap); ok {
		return append([]any(nil), im.keys...), append([]any(nil), im.vals...)
	}
	for it := m.entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		keys = append(keys, e.getKey())
		vals = append(vals, e.getValue())
	}
	return keys, vals
}

func immMapOf_(kv ...any) *ImmutableMap {
	keys := make([]any, 0, len(kv)/2)
	vals := make([]any, 0, len(kv)/2)
	for i := 0; i < len(kv); i += 2 {
		keys = append(keys, kv[i])
		vals = append(vals, kv[i+1])
	}
	return immMap_(imMap_, keys, vals, nil, false)
}

var emptyImmMap_ = &ImmutableMap{}
var emptyImmSortedMap_ = &ImmutableMap{kind: imSorted_}

func ImmutableMap_of() *ImmutableMap { return emptyImmMap_ }

func ImmutableMap_of_Object_Object(p0 any, p1 any) *ImmutableMap { return immMapOf_(p0, p1) }

func ImmutableMap_of_Object_Object_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any) *ImmutableMap {
	return immMapOf_(p0, p1, p2, p3, p4, p5)
}

func ImmutableMap_of_Object_Object_Object_Object_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any, p6 any, p7 any) *ImmutableMap {
	return immMapOf_(p0, p1, p2, p3, p4, p5, p6, p7)
}

func ImmutableMap_of_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object_Object(p0 any, p1 any, p2 any, p3 any, p4 any, p5 any, p6 any, p7 any, p8 any, p9 any, p10 any, p11 any, p12 any, p13 any, p14 any, p15 any, p16 any, p17 any, p18 any, p19 any) *ImmutableMap {
	return immMapOf_(p0, p1, p2, p3, p4, p5, p6, p7, p8, p9, p10, p11, p12, p13, p14, p15, p16, p17, p18, p19)
}

func ImmutableMap_copyOf(p0 Map) *ImmutableMap {
	if im, ok := p0.(*ImmutableMap); ok && im.kind != imSorted_ {
		return im
	}
	keys, vals := mapKV_(p0)
	return immMap_(imMap_, keys, vals, nil, false)
}

func ImmutableBiMap_of() *ImmutableBiMap { return immMap_(imBiMap_, nil, nil, nil, false) }

func ImmutableBiMap_copyOf(p0 Map) *ImmutableBiMap {
	if im, ok := p0.(*ImmutableMap); ok && im.kind == imBiMap_ {
		return im
	}
	keys, vals := mapKV_(p0)
	return immMap_(imBiMap_, keys, vals, nil, false)
}

func ImmutableSortedMap_copyOf(p0 Map) *ImmutableSortedMap {
	if im, ok := p0.(*ImmutableMap); ok && im.kind == imSorted_ && im.cmp == nil {
		return im
	}
	keys, vals := mapKV_(p0)
	return immMap_(imSorted_, keys, vals, nil, false)
}

// Maps.immutableEnumMap: iteration follows the enum constants' declaration order.
func Maps_immutableEnumMap(p0 Map) *ImmutableMap {
	keys, vals := mapKV_(p0)
	order := make([]any, len(keys))
	for i := range order {
		order[i] = i
	}
	c := ordinalCmp_()
	sortSlice_(order, &cmpFunc_{func(a, b any) int32 { return c.compare(keys[a.(int)], keys[b.(int)]) }})
	k2, v2 := make([]any, len(keys)), make([]any, len(keys))
	for i, j := range order {
		k2[i], v2[i] = keys[j.(int)], vals[j.(int)]
	}
	return immMap_(imMap_, k2, v2, nil, false)
}

// ---- ImmutableMap.Builder

type ImmutableMap_Builder struct {
	kind int8
	keys []any
	vals []any
	cmp  Comparator
}

type ImmutableSortedMap_Builder = ImmutableMap_Builder
type ImmutableBiMap_Builder = ImmutableMap_Builder

func new_ImmutableMap_Builder() *ImmutableMap_Builder { return &ImmutableMap_Builder{} }
func ImmutableMap_builder() *ImmutableMap_Builder     { return &ImmutableMap_Builder{} }
func ImmutableSortedMap_naturalOrder() *ImmutableSortedMap_Builder {
	return &ImmutableMap_Builder{kind: imSorted_}
}
func (this *ImmutableMap_Builder) put(p0 any, p1 any) *ImmutableMap_Builder {
	if p0 == nil {
		throwNPE_("null key in entry: null=" + objstr_(p1))
	}
	if p1 == nil {
		throwNPE_("null value in entry: " + objstr_(p0) + "=null")
	}
	this.keys = append(this.keys, p0)
	this.vals = append(this.vals, p1)
	return this
}

func (this *ImmutableMap_Builder) putAll(p0 Map) *ImmutableMap_Builder {
	keys, vals := mapKV_(p0)
	for i := range keys {
		this.put(keys[i], vals[i])
	}
	return this
}

func (this *ImmutableMap_Builder) make_(keepLast bool) *ImmutableMap {
	return immMap_(this.kind, append([]any(nil), this.keys...), append([]any(nil), this.vals...), this.cmp, keepLast)
}

func (this *ImmutableMap_Builder) build() *ImmutableMap        { return this.make_(false) }
func (this *ImmutableMap_Builder) buildOrThrow() *ImmutableMap { return this.make_(false) }
func (this *ImmutableMap_Builder) buildKeepingLast() *ImmutableMap {
	if this.kind == imSorted_ {
		// ImmutableSortedMap.Builder does not support buildKeepingLast.
		throwUOE_()
	}
	return this.make_(true)
}

// ---- ImmutableMultimap (list- or set-valued)

type ImmutableMultimap struct {
	set  bool
	m    *ImmutableMap // key -> *ImmutableCollection
	n    int32
	inv_ *ImmutableMultimap
	vals *ImmutableCollection
	ents *ImmutableCollection
}

type ImmutableListMultimap = ImmutableMultimap
type ImmutableSetMultimap = ImmutableMultimap

func (this *ImmutableMultimap) size() int32   { return this.n }
func (this *ImmutableMultimap) isEmpty() bool { return this.n == 0 }

func (this *ImmutableMultimap) get(p0 any) Collection {
	if c := this.m.get(p0); c != nil {
		return c.(*ImmutableCollection)
	}
	if this.set {
		return ImmutableSet_of()
	}
	return ImmutableList_of()
}

func (this *ImmutableMultimap) containsKey(p0 any) bool { return this.m.containsKey(p0) }

func (this *ImmutableMultimap) containsEntry(p0 any, p1 any) bool {
	c := this.m.get(p0)
	return c != nil && c.(*ImmutableCollection).contains(p1)
}

func (this *ImmutableMultimap) containsValue(p0 any) bool { return this.values().contains(p0) }

func (this *ImmutableMultimap) asMap() Map  { return this.m }
func (this *ImmutableMultimap) keySet() Set { return this.m.keySet() }
func (this *ImmutableMultimap) keys() Multiset {
	b := ImmutableMultiset_builder()
	for i, k := range this.m.keys {
		b.addCopies(k, this.m.vals[i].(*ImmutableCollection).size())
	}
	return b.build()
}

func (this *ImmutableMultimap) values() Collection {
	if this.vals == nil {
		all := make([]any, 0, this.n)
		for _, c := range this.m.vals {
			all = append(all, c.(*ImmutableCollection).items...)
		}
		this.vals = &ImmutableCollection{kind: icList_, items: all}
	}
	return this.vals
}

func (this *ImmutableMultimap) inverse() *ImmutableMultimap {
	if this.inv_ == nil {
		b := &ImmutableMultimap_Builder{set: this.set}
		for i, k := range this.m.keys {
			for _, v := range this.m.vals[i].(*ImmutableCollection).items {
				b.put(v, k)
			}
		}
		this.inv_ = b.build()
		this.inv_.inv_ = this
	}
	return this.inv_
}

func (this *ImmutableMultimap) put(p0 any, p1 any) bool          { throwUOE_(); return false }
func (this *ImmutableMultimap) putAll(p0 any, p1 Iterable) bool  { throwUOE_(); return false }
func (this *ImmutableMultimap) putAll_Multimap(p0 Multimap) bool { throwUOE_(); return false }
func (this *ImmutableMultimap) remove(p0 any, p1 any) bool       { throwUOE_(); return false }
func (this *ImmutableMultimap) removeAll(p0 any) Collection      { throwUOE_(); return nil }
func (this *ImmutableMultimap) clear()                           { throwUOE_() }

func (this *ImmutableMultimap) equals(o any) bool { return multimapEquals_(this, o) }
func (this *ImmutableMultimap) hashCode() int32   { return this.m.hashCode() }
func (this *ImmutableMultimap) toString() string  { return this.m.toString() }

func multimapEquals_(m Multimap, o any) bool {
	if o == any(m) {
		return true
	}
	x, ok := o.(Multimap)
	if !ok || x == nil {
		return false
	}
	return objequals_(m.asMap(), x.asMap())
}

type ImmutableMultimap_Builder struct {
	set    bool
	keys   []any
	idx    htable_ // key -> *[]any
	keyCmp Comparator
	valCmp Comparator
}

type ImmutableListMultimap_Builder = ImmutableMultimap_Builder
type ImmutableSetMultimap_Builder = ImmutableMultimap_Builder

func new_ImmutableMultimap_Builder() *ImmutableMultimap_Builder { return &ImmutableMultimap_Builder{} }
func ImmutableMultimap_builder() *ImmutableMultimap_Builder     { return &ImmutableMultimap_Builder{} }
func ImmutableListMultimap_builder() *ImmutableListMultimap_Builder {
	return &ImmutableMultimap_Builder{}
}
func ImmutableSetMultimap_builder() *ImmutableSetMultimap_Builder {
	return &ImmutableMultimap_Builder{set: true}
}

func (this *ImmutableMultimap_Builder) put(p0 any, p1 any) *ImmutableMultimap_Builder {
	if p0 == nil {
		throwNPE_("null key in entry: null=" + objstr_(p1))
	}
	if p1 == nil {
		throwNPE_("null value in entry: " + objstr_(p0) + "=null")
	}
	var vs *[]any
	if i := this.idx.find(p0); i >= 0 {
		vs = this.idx.entries[i].val.(*[]any)
	} else {
		vs = new([]any)
		this.idx.put(p0, vs)
		this.keys = append(this.keys, p0)
	}
	*vs = append(*vs, p1)
	return this
}

func (this *ImmutableMultimap_Builder) putAll(p0 any, p1 Iterable) *ImmutableMultimap_Builder {
	if p0 == nil {
		throwNPE_("null key in entry: null=" + collToString_(p1))
	}
	for _, v := range toSlice_(p1) {
		this.put(p0, v)
	}
	return this
}

func (this *ImmutableMultimap_Builder) putAll_Multimap(p0 Multimap) *ImmutableMultimap_Builder {
	for it := p0.asMap().entrySet().iterator(); it.hasNext(); {
		e := it.next().(Map_Entry)
		this.putAll(e.getKey(), e.getValue().(Iterable))
	}
	return this
}

func (this *ImmutableMultimap_Builder) orderKeysBy(p0 Comparator) *ImmutableMultimap_Builder {
	this.keyCmp = p0
	return this
}

func (this *ImmutableMultimap_Builder) build() *ImmutableMultimap {
	keys := append([]any(nil), this.keys...)
	if this.keyCmp != nil {
		sortSlice_(keys, this.keyCmp)
	}
	out := &ImmutableMultimap{set: this.set}
	vals := make([]any, len(keys))
	for i, k := range keys {
		vs := append([]any(nil), *this.idx.entries[this.idx.find(k)].val.(*[]any)...)
		if this.valCmp != nil {
			sortSlice_(vs, this.valCmp)
		}
		var c *ImmutableCollection
		if this.set {
			c = immSet_(vs)
		} else {
			c = immList_(vs)
		}
		vals[i] = c
		out.n += c.size()
	}
	out.m = immMap_(imMap_, keys, vals, nil, false)
	return out
}

func immMultimapCopy_(set bool, p0 Multimap) *ImmutableMultimap {
	if im, ok := p0.(*ImmutableMultimap); ok && im.set == set {
		return im
	}
	b := &ImmutableMultimap_Builder{set: set}
	b.putAll_Multimap(p0)
	return b.build()
}

func ImmutableSetMultimap_copyOf(p0 Multimap) *ImmutableSetMultimap {
	return immMultimapCopy_(true, p0)
}
func Multimaps_asMap(p0 ListMultimap) Map { return p0.asMap() }

func Multimaps_index(p0 Iterable, p1 Function_base) *ImmutableListMultimap {
	b := ImmutableListMultimap_builder()
	for it := p0.iterator(); it.hasNext(); {
		v := it.next()
		if v == nil {
			throwNPE_("null")
		}
		b.put(p1.apply(v), v)
	}
	return b.build()
}

func Multimaps_asMap_SetMultimap(p0 SetMultimap) Map { return p0.asMap() }
