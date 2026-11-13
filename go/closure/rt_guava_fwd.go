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

type ArrayListMultimapGwtSerializationDependencies struct{ mmap_ }

type BaseImmutableMultimap struct{ mmap_ }

type LinkedHashMultimap struct{ mmap_ }

type LinkedHashMultimapGwtSerializationDependencies struct{ mmap_ }

type LinkedListMultimap struct{ mmap_ }

type TreeMultimap struct{ mmap_ }

type HashMultimap struct{ mmap_ }

type AbstractMapBasedMultiset struct{ mset_ }

type AbstractMultiset struct{ mset_ }

type AbstractSortedMultiset struct{ mset_ }

type LinkedHashMultiset struct{ mset_ }

type TreeMultiset struct{ mset_ }

type HashMultiset struct{ mset_ }

type AbstractRangeSet struct{ TreeRangeSet }
