package closure

// The public mutable multimap and multiset types. Each embeds its core (mmap_
// or mset_, rt_guava_multi.go). The methods the translated code calls are
// spelled out per type, mechanically, because the placeholder generator looks
// for a declaration with the type's own name as receiver and would otherwise
// shadow the embedded method with a placeholder. The Abstract* and *Gwt* types
// are Guava's non-instantiable bases, which the translation only names.

type AbstractListMultimap struct{ mmap_ }

func (this *AbstractListMultimap) asMap() Map { return this.mmap_.asMap() }
func (this *AbstractListMultimap) clear()     { this.mmap_.clear() }
func (this *AbstractListMultimap) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *AbstractListMultimap) containsKey(p0 any) bool         { return this.mmap_.containsKey(p0) }
func (this *AbstractListMultimap) get(p0 any) Collection           { return this.mmap_.get(p0) }
func (this *AbstractListMultimap) isEmpty() bool                   { return this.mmap_.isEmpty() }
func (this *AbstractListMultimap) keySet() Set                     { return this.mmap_.keySet() }
func (this *AbstractListMultimap) put(p0 any, p1 any) bool         { return this.mmap_.put(p0, p1) }
func (this *AbstractListMultimap) putAll(p0 any, p1 Iterable) bool { return this.mmap_.putAll(p0, p1) }
func (this *AbstractListMultimap) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *AbstractListMultimap) remove(p0 any, p1 any) bool  { return this.mmap_.remove(p0, p1) }
func (this *AbstractListMultimap) removeAll(p0 any) Collection { return this.mmap_.removeAll(p0) }
func (this *AbstractListMultimap) size() int32                 { return this.mmap_.size() }
func (this *AbstractListMultimap) values() Collection          { return this.mmap_.values() }

type AbstractMapBasedMultimap struct{ mmap_ }

func (this *AbstractMapBasedMultimap) asMap() Map { return this.mmap_.asMap() }
func (this *AbstractMapBasedMultimap) clear()     { this.mmap_.clear() }
func (this *AbstractMapBasedMultimap) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *AbstractMapBasedMultimap) containsKey(p0 any) bool { return this.mmap_.containsKey(p0) }
func (this *AbstractMapBasedMultimap) get(p0 any) Collection   { return this.mmap_.get(p0) }
func (this *AbstractMapBasedMultimap) isEmpty() bool           { return this.mmap_.isEmpty() }
func (this *AbstractMapBasedMultimap) keySet() Set             { return this.mmap_.keySet() }
func (this *AbstractMapBasedMultimap) put(p0 any, p1 any) bool { return this.mmap_.put(p0, p1) }
func (this *AbstractMapBasedMultimap) putAll(p0 any, p1 Iterable) bool {
	return this.mmap_.putAll(p0, p1)
}
func (this *AbstractMapBasedMultimap) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *AbstractMapBasedMultimap) remove(p0 any, p1 any) bool  { return this.mmap_.remove(p0, p1) }
func (this *AbstractMapBasedMultimap) removeAll(p0 any) Collection { return this.mmap_.removeAll(p0) }
func (this *AbstractMapBasedMultimap) size() int32                 { return this.mmap_.size() }
func (this *AbstractMapBasedMultimap) values() Collection          { return this.mmap_.values() }

type AbstractMultimap struct{ mmap_ }

func (this *AbstractMultimap) asMap() Map { return this.mmap_.asMap() }
func (this *AbstractMultimap) clear()     { this.mmap_.clear() }
func (this *AbstractMultimap) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *AbstractMultimap) containsKey(p0 any) bool         { return this.mmap_.containsKey(p0) }
func (this *AbstractMultimap) get(p0 any) Collection           { return this.mmap_.get(p0) }
func (this *AbstractMultimap) isEmpty() bool                   { return this.mmap_.isEmpty() }
func (this *AbstractMultimap) keySet() Set                     { return this.mmap_.keySet() }
func (this *AbstractMultimap) put(p0 any, p1 any) bool         { return this.mmap_.put(p0, p1) }
func (this *AbstractMultimap) putAll(p0 any, p1 Iterable) bool { return this.mmap_.putAll(p0, p1) }
func (this *AbstractMultimap) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *AbstractMultimap) remove(p0 any, p1 any) bool  { return this.mmap_.remove(p0, p1) }
func (this *AbstractMultimap) removeAll(p0 any) Collection { return this.mmap_.removeAll(p0) }
func (this *AbstractMultimap) size() int32                 { return this.mmap_.size() }
func (this *AbstractMultimap) values() Collection          { return this.mmap_.values() }

type AbstractSetMultimap struct{ mmap_ }

func (this *AbstractSetMultimap) asMap() Map { return this.mmap_.asMap() }
func (this *AbstractSetMultimap) clear()     { this.mmap_.clear() }
func (this *AbstractSetMultimap) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *AbstractSetMultimap) containsKey(p0 any) bool         { return this.mmap_.containsKey(p0) }
func (this *AbstractSetMultimap) get(p0 any) Collection           { return this.mmap_.get(p0) }
func (this *AbstractSetMultimap) isEmpty() bool                   { return this.mmap_.isEmpty() }
func (this *AbstractSetMultimap) keySet() Set                     { return this.mmap_.keySet() }
func (this *AbstractSetMultimap) put(p0 any, p1 any) bool         { return this.mmap_.put(p0, p1) }
func (this *AbstractSetMultimap) putAll(p0 any, p1 Iterable) bool { return this.mmap_.putAll(p0, p1) }
func (this *AbstractSetMultimap) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *AbstractSetMultimap) remove(p0 any, p1 any) bool  { return this.mmap_.remove(p0, p1) }
func (this *AbstractSetMultimap) removeAll(p0 any) Collection { return this.mmap_.removeAll(p0) }
func (this *AbstractSetMultimap) size() int32                 { return this.mmap_.size() }
func (this *AbstractSetMultimap) values() Collection          { return this.mmap_.values() }

type AbstractSortedKeySortedSetMultimap struct{ mmap_ }

func (this *AbstractSortedKeySortedSetMultimap) asMap() Map { return this.mmap_.asMap() }
func (this *AbstractSortedKeySortedSetMultimap) clear()     { this.mmap_.clear() }
func (this *AbstractSortedKeySortedSetMultimap) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *AbstractSortedKeySortedSetMultimap) containsKey(p0 any) bool {
	return this.mmap_.containsKey(p0)
}
func (this *AbstractSortedKeySortedSetMultimap) get(p0 any) Collection { return this.mmap_.get(p0) }
func (this *AbstractSortedKeySortedSetMultimap) isEmpty() bool         { return this.mmap_.isEmpty() }
func (this *AbstractSortedKeySortedSetMultimap) keySet() Set           { return this.mmap_.keySet() }
func (this *AbstractSortedKeySortedSetMultimap) put(p0 any, p1 any) bool {
	return this.mmap_.put(p0, p1)
}
func (this *AbstractSortedKeySortedSetMultimap) putAll(p0 any, p1 Iterable) bool {
	return this.mmap_.putAll(p0, p1)
}
func (this *AbstractSortedKeySortedSetMultimap) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *AbstractSortedKeySortedSetMultimap) remove(p0 any, p1 any) bool {
	return this.mmap_.remove(p0, p1)
}
func (this *AbstractSortedKeySortedSetMultimap) removeAll(p0 any) Collection {
	return this.mmap_.removeAll(p0)
}
func (this *AbstractSortedKeySortedSetMultimap) size() int32        { return this.mmap_.size() }
func (this *AbstractSortedKeySortedSetMultimap) values() Collection { return this.mmap_.values() }

type AbstractSortedSetMultimap struct{ mmap_ }

func (this *AbstractSortedSetMultimap) asMap() Map { return this.mmap_.asMap() }
func (this *AbstractSortedSetMultimap) clear()     { this.mmap_.clear() }
func (this *AbstractSortedSetMultimap) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *AbstractSortedSetMultimap) containsKey(p0 any) bool { return this.mmap_.containsKey(p0) }
func (this *AbstractSortedSetMultimap) get(p0 any) Collection   { return this.mmap_.get(p0) }
func (this *AbstractSortedSetMultimap) isEmpty() bool           { return this.mmap_.isEmpty() }
func (this *AbstractSortedSetMultimap) keySet() Set             { return this.mmap_.keySet() }
func (this *AbstractSortedSetMultimap) put(p0 any, p1 any) bool { return this.mmap_.put(p0, p1) }
func (this *AbstractSortedSetMultimap) putAll(p0 any, p1 Iterable) bool {
	return this.mmap_.putAll(p0, p1)
}
func (this *AbstractSortedSetMultimap) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *AbstractSortedSetMultimap) remove(p0 any, p1 any) bool  { return this.mmap_.remove(p0, p1) }
func (this *AbstractSortedSetMultimap) removeAll(p0 any) Collection { return this.mmap_.removeAll(p0) }
func (this *AbstractSortedSetMultimap) size() int32                 { return this.mmap_.size() }
func (this *AbstractSortedSetMultimap) values() Collection          { return this.mmap_.values() }

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

func (this *ArrayListMultimapGwtSerializationDependencies) asMap() Map { return this.mmap_.asMap() }
func (this *ArrayListMultimapGwtSerializationDependencies) clear()     { this.mmap_.clear() }
func (this *ArrayListMultimapGwtSerializationDependencies) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *ArrayListMultimapGwtSerializationDependencies) containsKey(p0 any) bool {
	return this.mmap_.containsKey(p0)
}
func (this *ArrayListMultimapGwtSerializationDependencies) get(p0 any) Collection {
	return this.mmap_.get(p0)
}
func (this *ArrayListMultimapGwtSerializationDependencies) isEmpty() bool {
	return this.mmap_.isEmpty()
}
func (this *ArrayListMultimapGwtSerializationDependencies) keySet() Set { return this.mmap_.keySet() }
func (this *ArrayListMultimapGwtSerializationDependencies) put(p0 any, p1 any) bool {
	return this.mmap_.put(p0, p1)
}
func (this *ArrayListMultimapGwtSerializationDependencies) putAll(p0 any, p1 Iterable) bool {
	return this.mmap_.putAll(p0, p1)
}
func (this *ArrayListMultimapGwtSerializationDependencies) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *ArrayListMultimapGwtSerializationDependencies) remove(p0 any, p1 any) bool {
	return this.mmap_.remove(p0, p1)
}
func (this *ArrayListMultimapGwtSerializationDependencies) removeAll(p0 any) Collection {
	return this.mmap_.removeAll(p0)
}
func (this *ArrayListMultimapGwtSerializationDependencies) size() int32 { return this.mmap_.size() }
func (this *ArrayListMultimapGwtSerializationDependencies) values() Collection {
	return this.mmap_.values()
}

type BaseImmutableMultimap struct{ mmap_ }

func (this *BaseImmutableMultimap) asMap() Map { return this.mmap_.asMap() }
func (this *BaseImmutableMultimap) clear()     { this.mmap_.clear() }
func (this *BaseImmutableMultimap) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *BaseImmutableMultimap) containsKey(p0 any) bool         { return this.mmap_.containsKey(p0) }
func (this *BaseImmutableMultimap) get(p0 any) Collection           { return this.mmap_.get(p0) }
func (this *BaseImmutableMultimap) isEmpty() bool                   { return this.mmap_.isEmpty() }
func (this *BaseImmutableMultimap) keySet() Set                     { return this.mmap_.keySet() }
func (this *BaseImmutableMultimap) put(p0 any, p1 any) bool         { return this.mmap_.put(p0, p1) }
func (this *BaseImmutableMultimap) putAll(p0 any, p1 Iterable) bool { return this.mmap_.putAll(p0, p1) }
func (this *BaseImmutableMultimap) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *BaseImmutableMultimap) remove(p0 any, p1 any) bool  { return this.mmap_.remove(p0, p1) }
func (this *BaseImmutableMultimap) removeAll(p0 any) Collection { return this.mmap_.removeAll(p0) }
func (this *BaseImmutableMultimap) size() int32                 { return this.mmap_.size() }
func (this *BaseImmutableMultimap) values() Collection          { return this.mmap_.values() }

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

func (this *LinkedHashMultimapGwtSerializationDependencies) asMap() Map { return this.mmap_.asMap() }
func (this *LinkedHashMultimapGwtSerializationDependencies) clear()     { this.mmap_.clear() }
func (this *LinkedHashMultimapGwtSerializationDependencies) containsEntry(p0 any, p1 any) bool {
	return this.mmap_.containsEntry(p0, p1)
}
func (this *LinkedHashMultimapGwtSerializationDependencies) containsKey(p0 any) bool {
	return this.mmap_.containsKey(p0)
}
func (this *LinkedHashMultimapGwtSerializationDependencies) get(p0 any) Collection {
	return this.mmap_.get(p0)
}
func (this *LinkedHashMultimapGwtSerializationDependencies) isEmpty() bool {
	return this.mmap_.isEmpty()
}
func (this *LinkedHashMultimapGwtSerializationDependencies) keySet() Set { return this.mmap_.keySet() }
func (this *LinkedHashMultimapGwtSerializationDependencies) put(p0 any, p1 any) bool {
	return this.mmap_.put(p0, p1)
}
func (this *LinkedHashMultimapGwtSerializationDependencies) putAll(p0 any, p1 Iterable) bool {
	return this.mmap_.putAll(p0, p1)
}
func (this *LinkedHashMultimapGwtSerializationDependencies) putAll_Multimap(p0 Multimap) bool {
	return this.mmap_.putAll_Multimap(p0)
}
func (this *LinkedHashMultimapGwtSerializationDependencies) remove(p0 any, p1 any) bool {
	return this.mmap_.remove(p0, p1)
}
func (this *LinkedHashMultimapGwtSerializationDependencies) removeAll(p0 any) Collection {
	return this.mmap_.removeAll(p0)
}
func (this *LinkedHashMultimapGwtSerializationDependencies) size() int32 { return this.mmap_.size() }
func (this *LinkedHashMultimapGwtSerializationDependencies) values() Collection {
	return this.mmap_.values()
}

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

func (this *TreeMultimap) asMap() Map                        { return this.mmap_.asMap() }
func (this *TreeMultimap) clear()                            { this.mmap_.clear() }
func (this *TreeMultimap) containsEntry(p0 any, p1 any) bool { return this.mmap_.containsEntry(p0, p1) }
func (this *TreeMultimap) containsKey(p0 any) bool           { return this.mmap_.containsKey(p0) }
func (this *TreeMultimap) get(p0 any) Collection             { return this.mmap_.get(p0) }
func (this *TreeMultimap) isEmpty() bool                     { return this.mmap_.isEmpty() }
func (this *TreeMultimap) keySet() Set                       { return this.mmap_.keySet() }
func (this *TreeMultimap) put(p0 any, p1 any) bool           { return this.mmap_.put(p0, p1) }
func (this *TreeMultimap) putAll(p0 any, p1 Iterable) bool   { return this.mmap_.putAll(p0, p1) }
func (this *TreeMultimap) putAll_Multimap(p0 Multimap) bool  { return this.mmap_.putAll_Multimap(p0) }
func (this *TreeMultimap) remove(p0 any, p1 any) bool        { return this.mmap_.remove(p0, p1) }
func (this *TreeMultimap) removeAll(p0 any) Collection       { return this.mmap_.removeAll(p0) }
func (this *TreeMultimap) size() int32                       { return this.mmap_.size() }
func (this *TreeMultimap) values() Collection                { return this.mmap_.values() }

type HashMultimap struct{ mmap_ }

func (this *HashMultimap) asMap() Map                        { return this.mmap_.asMap() }
func (this *HashMultimap) clear()                            { this.mmap_.clear() }
func (this *HashMultimap) containsEntry(p0 any, p1 any) bool { return this.mmap_.containsEntry(p0, p1) }
func (this *HashMultimap) containsKey(p0 any) bool           { return this.mmap_.containsKey(p0) }
func (this *HashMultimap) get(p0 any) Collection             { return this.mmap_.get(p0) }
func (this *HashMultimap) isEmpty() bool                     { return this.mmap_.isEmpty() }
func (this *HashMultimap) keySet() Set                       { return this.mmap_.keySet() }
func (this *HashMultimap) put(p0 any, p1 any) bool           { return this.mmap_.put(p0, p1) }
func (this *HashMultimap) putAll(p0 any, p1 Iterable) bool   { return this.mmap_.putAll(p0, p1) }
func (this *HashMultimap) putAll_Multimap(p0 Multimap) bool  { return this.mmap_.putAll_Multimap(p0) }
func (this *HashMultimap) remove(p0 any, p1 any) bool        { return this.mmap_.remove(p0, p1) }
func (this *HashMultimap) removeAll(p0 any) Collection       { return this.mmap_.removeAll(p0) }
func (this *HashMultimap) size() int32                       { return this.mmap_.size() }
func (this *HashMultimap) values() Collection                { return this.mmap_.values() }

type AbstractMapBasedMultiset struct{ mset_ }

func (this *AbstractMapBasedMultiset) add_Object_int(p0 any, p1 int32) int32 {
	return this.mset_.add_Object_int(p0, p1)
}
func (this *AbstractMapBasedMultiset) count(p0 any) int32 { return this.mset_.count(p0) }
func (this *AbstractMapBasedMultiset) elementSet() Set    { return this.mset_.elementSet() }
func (this *AbstractMapBasedMultiset) entrySet() Set      { return this.mset_.entrySet() }
func (this *AbstractMapBasedMultiset) setCount(p0 any, p1 int32, p2 int32) bool {
	return this.mset_.setCount(p0, p1, p2)
}

type AbstractMultiset struct{ mset_ }

func (this *AbstractMultiset) add_Object_int(p0 any, p1 int32) int32 {
	return this.mset_.add_Object_int(p0, p1)
}
func (this *AbstractMultiset) count(p0 any) int32 { return this.mset_.count(p0) }
func (this *AbstractMultiset) elementSet() Set    { return this.mset_.elementSet() }
func (this *AbstractMultiset) entrySet() Set      { return this.mset_.entrySet() }
func (this *AbstractMultiset) setCount(p0 any, p1 int32, p2 int32) bool {
	return this.mset_.setCount(p0, p1, p2)
}

type AbstractSortedMultiset struct{ mset_ }

func (this *AbstractSortedMultiset) add_Object_int(p0 any, p1 int32) int32 {
	return this.mset_.add_Object_int(p0, p1)
}
func (this *AbstractSortedMultiset) count(p0 any) int32 { return this.mset_.count(p0) }
func (this *AbstractSortedMultiset) elementSet() Set    { return this.mset_.elementSet() }
func (this *AbstractSortedMultiset) entrySet() Set      { return this.mset_.entrySet() }
func (this *AbstractSortedMultiset) setCount(p0 any, p1 int32, p2 int32) bool {
	return this.mset_.setCount(p0, p1, p2)
}

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

func (this *TreeMultiset) add_Object_int(p0 any, p1 int32) int32 {
	return this.mset_.add_Object_int(p0, p1)
}
func (this *TreeMultiset) count(p0 any) int32 { return this.mset_.count(p0) }
func (this *TreeMultiset) elementSet() Set    { return this.mset_.elementSet() }
func (this *TreeMultiset) entrySet() Set      { return this.mset_.entrySet() }
func (this *TreeMultiset) setCount(p0 any, p1 int32, p2 int32) bool {
	return this.mset_.setCount(p0, p1, p2)
}

type HashMultiset struct{ mset_ }

func (this *HashMultiset) add_Object_int(p0 any, p1 int32) int32 {
	return this.mset_.add_Object_int(p0, p1)
}
func (this *HashMultiset) count(p0 any) int32 { return this.mset_.count(p0) }
func (this *HashMultiset) elementSet() Set    { return this.mset_.elementSet() }
func (this *HashMultiset) entrySet() Set      { return this.mset_.entrySet() }
func (this *HashMultiset) setCount(p0 any, p1 int32, p2 int32) bool {
	return this.mset_.setCount(p0, p1, p2)
}

type AbstractRangeSet struct{ TreeRangeSet }

func (this *AbstractRangeSet) add(p0 *Range)        { this.TreeRangeSet.add(p0) }
func (this *AbstractRangeSet) contains(p0 any) bool { return this.TreeRangeSet.contains(p0) }
func (this *AbstractRangeSet) isEmpty() bool        { return this.TreeRangeSet.isEmpty() }

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

func HashMultimap_create() *HashMultimap {
	this := &HashMultimap{}
	this.init_(mmHashSet_, false, false)
	return this
}

func TreeMultimap_create() *TreeMultimap {
	this := &TreeMultimap{}
	this.init_(mmTreeSet_, true, false)
	return this
}

func LinkedHashMultiset_create() *LinkedHashMultiset {
	this := &LinkedHashMultiset{}
	this.init_(false)
	return this
}

func HashMultiset_create() *HashMultiset {
	this := &HashMultiset{}
	this.init_(false)
	return this
}

func TreeMultiset_create() *TreeMultiset {
	this := &TreeMultiset{}
	this.init_(true)
	return this
}
