package closure

// Typed twins of runtime methods (tools/j2go LibFns; docs/javaisms6/RESIDUE8.md). Each does
// what the erased method does, step by step, with its arguments at their Java types, so the
// translated call passes them unboxed and needs no cast of the result.
//
// Piece `rtm` (J2GO_NOLIBGEN=rtm is the translation without it; LibFns.RT_METHODS, TYPED_RT):
// methods of a runtime class on a receiver of that class, and methods of Guava's immutable
// classes and of TreeSet / Map on a typed collection. Piece `fns2` (LibFns.TYPED_F2): the
// twins here that need no function type (Splitter.split, Io.listFiles). Piece `seqstr`
// (Conv.STR_SEQ2): variants taking a Go string where Java declares a CharSequence.

// ---- piece rtm: runtime classes

// compareTo_t is BigInteger.compareTo(BigInteger) (compareTo).
func (this *BigInteger) compareTo_t(o *BigInteger) int32 { return int32(this.v.Cmp(o.v)) }

// compare_s is ComparisonChain.compare(Comparable, Comparable) of two Strings (compare, whose
// compareNatural_ compares two strings with String_compareTo; a null throws there).
func (this *ComparisonChain) compare_s(p0 string, p1 string) *ComparisonChain {
	if this.r != 0 {
		return this
	}
	if p0 == nullstr_ || p1 == nullstr_ {
		return this.classify_(compareNatural_(strobj_(p0), strobj_(p1)))
	}
	return this.classify_(String_compareTo(p0, p1))
}

// compareTo_t is Enum.compareTo(E) of an enum of the program (compareTo): the ordinals.
func (this *Enum) compareTo_t(o *Enum) int32 { return this.ordinal_ - o.ordinal_ }

// clone_t is BitSet.clone() at its class (clone).
func (this *BitSet) clone_t() *BitSet { return &BitSet{w: append([]uint64(nil), this.w...)} }

// ---- piece rtm: typed collections

// subListImm_ is ImmutableList.subList of an immutable typed list (asImm().subList, then the
// typed list of the result, tlistImm_): the list itself for the whole range, the typed empty
// list for an empty one, else an immutable list of the range (its items are never changed).
func (l *TList[T]) subListImm_(a int32, b int32) *TList[T] {
	l.immCheck_()
	if a < 0 || int(b) > len(l.items) || a > b {
		panic(new_IndexOutOfBoundsException_String("fromIndex: " + istr_(int64(a)) + ", toIndex: " + istr_(int64(b)) + ", size: " + istr_(int64(len(l.items)))).self())
	}
	if a == 0 && int(b) == len(l.items) {
		return l
	}
	if a == b {
		return tlistEmptyImm_(l.c)
	}
	return tlistOwn_(l.c, l.items[a:b:b])
}

// reverseImm_ is ImmutableList.reverse of an immutable typed list (asImm().reverse, then
// tlistImm_): a new immutable list of the elements in reverse order (the typed empty list for
// an empty one).
func (l *TList[T]) reverseImm_() *TList[T] {
	l.immCheck_()
	if len(l.items) == 0 {
		return tlistEmptyImm_(l.c)
	}
	out := make([]T, len(l.items))
	for i, v := range l.items {
		out[len(l.items)-1-i] = v
	}
	return tlistOwn_(l.c, out)
}

// immCheck_ is the ClassCastException of asImm for a list that is not immutable.
func (l *TList[T]) immCheck_() {
	if l.twin == nil || l.twin == unmodMark_ {
		classCastImm_("java.util.ArrayList", "com.google.common.collect.ImmutableList")
	}
}

// ImmutableList_subListT_ is ImmutableList.subList of a typed List that holds an ImmutableList
// (a place of the class, piece cls): subListImm_ of the typed list it is the view of, else the
// erased method and the typed list of its result.
func ImmutableList_subListT_[T any](l ListT[T], a int32, b int32) *TList[T] {
	if v, ok := l.(*tlistView_[T]); ok {
		return v.l.subListImm_(a, b)
	}
	return tlistImm_(tcT_[T](), cast_[*ImmutableList](lgBox_(l)).subList(a, b).(*ImmutableCollection))
}

// ImmutableList_reverseT_ is ImmutableList.reverse of a typed List that holds an ImmutableList.
func ImmutableList_reverseT_[T any](l ListT[T]) *TList[T] {
	if v, ok := l.(*tlistView_[T]); ok {
		return v.l.reverseImm_()
	}
	return tlistImm_(tcT_[T](), cast_[*ImmutableList](lgBox_(l)).reverse())
}

// tailSetT_ is tailSet(from, inclusive) of a TreeSet (a sorted typed set that can change) for
// typed code: as the erased view's tailSet, a new sorted set with the set's comparator of the
// elements as they are now, here at their type.
func (s *TSet[K]) tailSetT_(from K, inclusive bool) *TSet[K] {
	s.asSorted()
	i, ok := tsortSearch_(s.eq, s.c, s.keys, from)
	if ok && !inclusive {
		i++
	}
	out := newTSortedSetC_(s.c, s.eq.cmp)
	out.eq.cmpT = s.eq.cmpT
	for ; i < len(s.keys); i++ {
		if !s.dead(i) {
			out.keys = append(out.keys, s.keys[i])
		}
	}
	out.live = int32(len(out.keys))
	return out
}

// containsValueT_ is Map.containsValue of a value of the map's value type (the view's
// containsValue, the value boxed once).
func (m *TMap[K, V]) containsValueT_(x V) bool {
	bx := m.vc.box(x)
	for i := range m.keys {
		if !m.dead(i) && objequals_(bx, m.vc.box(m.vals[i])) {
			return true
		}
	}
	return false
}

// List_add_intT_ is List.add(int, E) of a typed List (add_int_Object): the typed list's insert
// where the List is its view.
func List_add_intT_[T any](l ListT[T], i int32, x T) {
	if v, ok := l.(*tlistView_[T]); ok {
		v.l.insert(i, x)
		return
	}
	l.add_int_Object(i, boxT_(x))
}

// ---- piece fns2 (no function types)

// split_sT is Splitter.split of a String as an Iterable of Strings (split_s): a typed list of the
// parts (split_s gives an ArrayList of them).
func (this *Splitter) split_sT(seq string) IterableT[string] {
	l := &TList[string]{c: tcT_[string](), items: this.splitStrs_(csStr_(seq))}
	l.view.l = l
	return l
}

// Io_listFilesT_ is Io.listFiles as a typed list of Strings (Io_listFiles).
func Io_listFilesT_(directory string) ListT[string] {
	if directory == "" {
		directory = "."
	}
	l := newTList_(tcT_[string]())
	listFilesInto_(directory, "", nil, func(s string) { l.items = append(l.items, s) })
	return l.asListT()
}

// ---- piece seqstr: String for CharSequence (rt_conv.go)

// TextFormat_merge_s is TextFormat.merge of a String (TextFormat_merge).
func TextFormat_merge_s(p0 string, p1 Message_Builder) {
	if p0 == nullstr_ {
		pbNPE_()
	}
	if p1 == nil {
		pbNPE_()
	}
	pbDefaultParser_.mergeText_(p0, p1)
}

// Ascii_equalsIgnoreCase_s is Ascii.equalsIgnoreCase of two Strings (Ascii_equalsIgnoreCase).
func Ascii_equalsIgnoreCase_s(a string, b string) bool {
	return asciiEqualsIgnoreCase_(csStr_(a), csStr_(b))
}

// Integer_parseInt_s_int_int_int is Integer.parseInt(CharSequence, int, int, int) of a String.
func Integer_parseInt_s_int_int_int(cs string, a int32, b int32, radix int32) int32 {
	return Integer_parseInt(String_substring(csStr_(cs), a, b), radix)
}

// lastIndexIn_s is CharMatcher.lastIndexIn of a String.
func (this *CharMatcher) lastIndexIn_s(seq string) int32 { return this.lastIndexStr_(csStr_(seq)) }

// collapseFrom_s is CharMatcher.collapseFrom of a String.
func (this *CharMatcher) collapseFrom_s(seq string, repl uint16) string {
	return this.collapseStr_(csStr_(seq), repl)
}

