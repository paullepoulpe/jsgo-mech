package closure

// The public mutable multimap and multiset types. Each embeds its core (mmap_
// or mset_, rt_guava_multi.go). The methods the translated code calls are
// spelled out per type, mechanically, because the placeholder generator looks
// for a declaration with the type's own name as receiver and would otherwise
// shadow the embedded method with a placeholder. The Abstract* and *Gwt* types
// are Guava's non-instantiable bases, which the translation only names.

type AbstractListMultimap struct{ mmap_ }

type AbstractMapBasedMultimap struct{ mmap_ }

type AbstractMultimap struct{ mmap_ }

type AbstractSetMultimap struct{ mmap_ }

type AbstractSortedKeySortedSetMultimap struct{ mmap_ }

type AbstractSortedSetMultimap struct{ mmap_ }

type ArrayListMultimap struct{ mmap_ }

func (this *ArrayListMultimap) asMap() Map { return this.mmap_.asMap() }
func (this *ArrayListMultimap) clear()     { this.mmap_.clear() }
func (this *ArrayListMultimap) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *ArrayListMultimap) containsKey(p0 any) bool         { return this.mmap_.containsKey(p0) }
func (this *ArrayListMultimap) get(p0 any) Collection           { return this.mmap_.get(p0) }
func (this *ArrayListMultimap) isEmpty() bool                   { return this.mmap_.isEmpty() }
func (this *ArrayListMultimap) keySet() Set                     { return this.mmap_.keySet() }
func (this *ArrayListMultimap) put(p0 any, p1 any) bool         { return this.mmap_.put(p0, p1) }
func (this *ArrayListMultimap) putAll(p0 any, p1 Iterable) bool { return this.mmap_.putAll(p0, p1) }
func (this *ArrayListMultimap) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *ArrayListMultimap) remove(p0 any, p1 any) bool  { return this.mmap_.remove(p0, p1) }
func (this *ArrayListMultimap) removeAll(p0 any) Collection { return this.mmap_.removeAll(p0) }
func (this *ArrayListMultimap) size() int32                 { return this.mmap_.size() }
func (this *ArrayListMultimap) values() Collection          { return this.mmap_.values() }

type ArrayListMultimapGwtSerializationDependencies struct{ mmap_ }

type BaseImmutableMultimap struct{ mmap_ }

type LinkedHashMultimap struct{ mmap_ }

func (this *LinkedHashMultimap) asMap() Map { return this.mmap_.asMap() }
func (this *LinkedHashMultimap) clear()     { this.mmap_.clear() }
func (this *LinkedHashMultimap) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *LinkedHashMultimap) containsKey(p0 any) bool         { return this.mmap_.containsKey(p0) }
func (this *LinkedHashMultimap) get(p0 any) Collection           { return this.mmap_.get(p0) }
func (this *LinkedHashMultimap) isEmpty() bool                   { return this.mmap_.isEmpty() }
func (this *LinkedHashMultimap) keySet() Set                     { return this.mmap_.keySet() }
func (this *LinkedHashMultimap) put(p0 any, p1 any) bool         { return this.mmap_.put(p0, p1) }
func (this *LinkedHashMultimap) putAll(p0 any, p1 Iterable) bool { return this.mmap_.putAll(p0, p1) }
func (this *LinkedHashMultimap) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *LinkedHashMultimap) remove(p0 any, p1 any) bool  { return this.mmap_.remove(p0, p1) }
func (this *LinkedHashMultimap) removeAll(p0 any) Collection { return this.mmap_.removeAll(p0) }
func (this *LinkedHashMultimap) size() int32                 { return this.mmap_.size() }
func (this *LinkedHashMultimap) values() Collection          { return this.mmap_.values() }

type LinkedHashMultimapGwtSerializationDependencies struct{ mmap_ }

type LinkedListMultimap struct{ mmap_ }

func (this *LinkedListMultimap) asMap() Map { return this.mmap_.asMap() }
func (this *LinkedListMultimap) clear()     { this.mmap_.clear() }
func (this *LinkedListMultimap) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *LinkedListMultimap) containsKey(p0 any) bool         { return this.mmap_.containsKey(p0) }
func (this *LinkedListMultimap) get(p0 any) Collection           { return this.mmap_.get(p0) }
func (this *LinkedListMultimap) isEmpty() bool                   { return this.mmap_.isEmpty() }
func (this *LinkedListMultimap) keySet() Set                     { return this.mmap_.keySet() }
func (this *LinkedListMultimap) put(p0 any, p1 any) bool         { return this.mmap_.put(p0, p1) }
func (this *LinkedListMultimap) putAll(p0 any, p1 Iterable) bool { return this.mmap_.putAll(p0, p1) }
func (this *LinkedListMultimap) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *LinkedListMultimap) remove(p0 any, p1 any) bool  { return this.mmap_.remove(p0, p1) }
func (this *LinkedListMultimap) removeAll(p0 any) Collection { return this.mmap_.removeAll(p0) }
func (this *LinkedListMultimap) size() int32                 { return this.mmap_.size() }
func (this *LinkedListMultimap) values() Collection          { return this.mmap_.values() }

type TreeMultimap struct{ mmap_ }

type HashMultimap struct{ mmap_ }

type AbstractMapBasedMultiset struct{ mset_ }

type AbstractMultiset struct{ mset_ }

type AbstractSortedMultiset struct{ mset_ }

type LinkedHashMultiset struct{ mset_ }

func (this *LinkedHashMultiset) add_Object_int(p0 any, p1 int32) int32 {
	return this.mset_.add_Object_int(p0, p1)
}
func (this *LinkedHashMultiset) count(p0 any) int32 { return this.mset_.count(p0) }
func (this *LinkedHashMultiset) elementSet() Set    { return this.mset_.elementSet() }
func (this *LinkedHashMultiset) entrySet() Set      { return this.mset_.entrySet() }
func (this *LinkedHashMultiset) setCount(p0 any, p1 int32, p2 int32) bool {
	return this.mset_.setCount(p0, p1, p2)
}

type TreeMultiset struct{ mset_ }

type HashMultiset struct{ mset_ }

type AbstractRangeSet struct{ TreeRangeSet }

func ArrayListMultimap_create() *ArrayListMultimap {
	this := &ArrayListMultimap{}
	this.init_(mmArrayList_, false, false)
	return this
}

func LinkedListMultimap_create() *LinkedListMultimap {
	this := &LinkedListMultimap{}
	this.init_(mmArrayList_, false, true)
	return this
}

func LinkedHashMultimap_create() *LinkedHashMultimap {
	this := &LinkedHashMultimap{}
	this.init_(mmHashSet_, false, false)
	return this
}

func LinkedHashMultiset_create() *LinkedHashMultiset {
	this := &LinkedHashMultiset{}
	this.init_(false)
	return this
}

