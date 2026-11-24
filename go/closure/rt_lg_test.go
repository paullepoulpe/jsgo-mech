package closure

import "testing"

// A typed list is an Iterable of its elements itself; for erased code and as an Object it is
// its erased view.
func TestLgTypedListIsIterable(t *testing.T) {
	l := newTList_(tc_string)
	l.add("a")
	l.add("b")
	it := l.asIterable()
	if (*TList[string])(nil).asIterable() != nil {
		t.Fatal("null list as an Iterable is not nil")
	}
	if lgBox_(it) != any(l.asList()) || Iterable_un_(it) != Iterable(l.asList()) {
		t.Fatal("the typed list goes to erased code as its view")
	}
	got := ""
	for e := jeachOf_(it); e.next(); {
		got += e.v
	}
	for i := it.iterator_t(); i.hasNext(); {
		got += i.next_t()
	}
	for i := it.iterator(); i.hasNext(); {
		got += i.next().(string)
	}
	if got != "ababab" {
		t.Fatal(got)
	}
	// the erased view is native: no wrapper
	if iterableAs_(tc_string, Iterable(l.asList())) != IterableT[string](l.tview_()) {
		t.Fatal("the view of a typed list was wrapped")
	}
	// the loop sees what the body appends, as the erased loop over an ArrayList does
	n := 0
	for e := jeachOf_(it); e.next(); n++ {
		if n == 0 {
			l.add("c")
		}
	}
	if n != 3 {
		t.Fatal("elements appended during the loop are visited", n)
	}
}

// An erased collection becomes a typed Iterable through a wrapper that unboxes; the wrapper
// does not reach erased code.
func TestLgWrappedErased(t *testing.T) {
	al := new_ArrayList_0()
	al.add("x")
	al.add("y")
	w := iterableAs_(tc_string, Iterable(al))
	if _, ok := w.(*iterableAs[string]); !ok {
		t.Fatal("an ArrayList is no native Iterable of strings")
	}
	if lgBox_(w) != any(al) || Iterable_un_(w) != Iterable(al) {
		t.Fatal("the wrapper reached erased code")
	}
	got := ""
	for e := jeachOf_(w); e.next(); {
		got += e.v
	}
	i := w.iterator_t()
	for i.hasNext() {
		got += i.next_t()
	}
	if got != "xyxy" {
		t.Fatal(got)
	}
	if Iterator_un_(i) == Iterator(i) {
		t.Fatal("the iterator wrapper reached erased code")
	}
	if iterableAs_(tc_string, nil) != nil || iteratorAs_(tc_string, nil) != nil {
		t.Fatal("null stays null")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("next_t past the end must throw")
			}
		}()
		i.next_t()
	}()
	// an element of another type fails when it is read, not before
	al.add(int32(1))
	w2 := iterableAs_(tc_string, Iterable(al))
	e := jeachOf_(w2)
	e.next()
	e.next()
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("reading an element of another type must fail")
			}
		}()
		e.next()
	}()
	// a wrapper at another element type is made anew: the instantiation decides
	if _, ok := iterableAs_(tc_Object, Iterable(w)).(*iterableAs[any]); !ok {
		t.Fatal("an Iterable of strings is no native Iterable of Objects")
	}
}

func TestLgTypedSetAndImmutable(t *testing.T) {
	s := newTSet_(tc_string)
	s.Add("k1")
	s.Add("k2")
	got := ""
	for e := jeachOf_(s.asIterable()); e.next(); {
		got += e.v
	}
	if got != "k1k2" || lgBox_(s.asIterable()) != any(s.asSet()) {
		t.Fatal("typed set as an Iterable", got)
	}
	im := immutableListOf_(tc_string, "p", "q")
	// the erased twin of an immutable typed list is the typed list again
	if iterableAs_(tc_string, Iterable(im.asImm())) != IterableT[string](im) {
		t.Fatal("the twin of an immutable typed list was wrapped")
	}
}

// keySet() and values() of the erased view of a typed map are native Iterables of the keys and
// of the values: no wrapper, the elements as they are, removed mappings skipped.
func TestLgTypedMapViews(t *testing.T) {
	m := newTMap_(tc_string, tc_string)
	for _, k := range []string{"a", "b", "c", "d"} {
		m.Put(k, k+k)
	}
	m.Delete("b")
	ks, vs := Iterable(m.asMap().keySet()), Iterable(m.asMap().values())
	kt, vt := iterableAs_(tc_string, ks), iterableAs_(tc_string, vs)
	if any(kt) != any(ks) || any(vt) != any(vs) {
		t.Fatal("a view of a typed map was wrapped")
	}
	got := ""
	for e := jeachOf_(kt); e.next(); {
		got += e.v
	}
	for e := jeachOf_(vt); e.next(); {
		got += e.v
	}
	for i := kt.iterator_t(); i.hasNext(); {
		got += i.next_t()
	}
	i := vt.iterator_t()
	for i.hasNext() {
		if v := i.next_t(); v == "cc" {
			i.(interface{ remove() }).remove()
		} else {
			got += v
		}
	}
	if got != "acd"+"aaccdd"+"acd"+"aadd" || m.Len() != 2 || m.Has("c") {
		t.Fatal(got, m.Len())
	}
	defer func() {
		if recover() == nil {
			t.Fatal("next_t of an exhausted iterator did not throw")
		}
	}()
	i.next_t()
}

// Kind coll: a typed list, set or deque as a Collection is its view (native), an erased
// collection is wrapped, an erased immutable list gives the view of its typed list; erased code
// gets the object it knows.
func TestLgCollection(t *testing.T) {
	l := newTList_(tc_string)
	l.add("a")
	c := l.asCollT()
	if (*TList[string])(nil).asCollT() != nil || (*TSet[string])(nil).asCollT() != nil || (*TDeque[string])(nil).asCollT() != nil {
		t.Fatal("null as a Collection is not nil")
	}
	if Collection_un_(c) != Collection(l.asList()) || lgBox_(c) != any(l.asList()) || collectionAs_(tc_string, Collection(l.asList())) != c {
		t.Fatal("the typed list as a Collection is not its view")
	}
	c.add("b") // the erased method, through the view
	if l.size() != 2 || c.iterator_t().next_t() != "a" {
		t.Fatal(l.size())
	}
	d := newTDeque_(tc_string)
	d.push("x")
	if Collection_un_(d.asCollT()) != Collection(d.asDeque()) {
		t.Fatal("the typed deque as a Collection is not its deque view")
	}
	// an immutable typed list: the view for typed code, the twin for erased code
	il := immutableListOf_(tc_string, "p", "q")
	ic := il.asCollT()
	if Collection_un_(ic) != Collection(il.asImm()) || collectionAs_(tc_string, Collection(il.asImm())) != ic {
		t.Fatal("the immutable typed list as a Collection")
	}
	got := ""
	for e := jeachOf_[string](ic); e.next(); {
		got += e.v
	}
	// an erased collection: a wrapper that unboxes; erased code gets the collection back
	al := new_ArrayList_0()
	al.add(strobj_("m"))
	w := collectionAs_(tc_string, Collection(al))
	for e := jeachOf_[string](w); e.next(); {
		got += e.v
	}
	if got != "pqm" || Collection_un_(w) != Collection(al) || w.size() != 1 {
		t.Fatal(got)
	}
	if collectionAs_(tc_string, nil) != nil || Collection_un_(nil) != nil {
		t.Fatal("null")
	}
}

// valuesT of a typed map: the map's values at their type; of an immutable map too, where every
// change throws; erased code gets the erased map's values().
func TestLgValuesT(t *testing.T) {
	m := newTMap_(tc_string, tc_string)
	m.Put("a", "x")
	m.Put("b", "y")
	v := m.valuesT()
	if Collection_un_(v) != m.asMap().values() || lgBox_(v) != any(m.asMap().values()) {
		t.Fatal("valuesT of a map that can change is not the erased view's values()")
	}
	m.Put("c", "z")
	got := ""
	for e := jeachOf_[string](v); e.next(); {
		got += e.v
	}
	im := immutableMapOf1_(tc_string, tc_string, "k", "w")
	iv := im.valuesT()
	for e := jeachOf_[string](iv); e.next(); {
		got += e.v
	}
	if got != "xyzw" || iv.size() != 1 || Collection_un_(iv) != im.asImm().values() {
		t.Fatal(got)
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("removing through the values of an immutable map did not throw")
			}
		}()
		it := iv.iterator_t()
		it.next_t()
		it.(interface{ remove() }).remove()
	}()
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("valuesT of null did not throw")
			}
		}()
		(*TMap[string, string])(nil).valuesT()
	}()
}

// Kind list: a typed list as a List is its view, with the elements at their type; an erased
// list is wrapped; an erased immutable list gives the view of its typed list.
func TestLgList(t *testing.T) {
	l := newTList_(tc_string)
	l.add("a")
	l.add("b")
	lt := l.asListT()
	if (*TList[string])(nil).asListT() != nil || listAs_(tc_string, nil) != nil || List_un_(nil) != nil {
		t.Fatal("null")
	}
	if List_un_(lt) != l.asList() || lgBox_(lt) != any(l.asList()) || listAs_(tc_string, l.asList()) != lt || CollectionT[string](lt) != l.asCollT() {
		t.Fatal("the typed list as a List is not its view")
	}
	if lt.get_t(1) != "b" || lt.size() != 2 {
		t.Fatal(lt.get_t(1))
	}
	il := immutableListOf_(tc_string, "p", "q")
	it := il.asListT()
	if List_un_(it) != List(il.asImm()) || listAs_(tc_string, List(il.asImm())) != it || it.get_t(0) != "p" {
		t.Fatal("the immutable typed list as a List")
	}
	al := new_ArrayList_0()
	al.add(strobj_("m"))
	al.add(obj_(int32(1)))
	w := listAs_(tc_string, List(al))
	got := w.get_t(0)
	for e := jeachOf_[string](l.asListT()); e.next(); {
		got += e.v
	}
	if got != "mab" || List_un_(w) != List(al) || lgBox_(w) != any(al) {
		t.Fatal(got)
	}
	for _, f := range []func(){func() { lt.get_t(2) }, func() { w.get_t(1) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("get_t out of range, or of an element of another class, did not throw")
				}
			}()
			f()
		}()
	}
}

// subList of the view of a typed list is a typed list on the same items, as the erased
// ArrayList's is a list on the same boxes: set writes through, and nothing is boxed.
func TestLgSubList(t *testing.T) {
	l := newTList_(tc_string)
	for _, x := range []string{"a", "b", "c", "d"} {
		l.add(x)
	}
	s := listAs_(tc_string, l.asListT().subList(1, 3))
	if _, wrapped := s.(*listAs[string]); wrapped || s.size() != 2 || s.get_t(0) != "b" || s.get_t(1) != "c" {
		t.Fatal("subList of a typed list")
	}
	s.set(0, strobj_("B"))
	if l.get(1) != "B" || jtostr_(s) != "[B, c]" {
		t.Fatal("set through the sublist", l.get(1))
	}
	im := immutableListOf_(tc_string, "p", "q", "r").asListT().subList(0, 2)
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("the sublist of an immutable list was changed")
			}
		}()
		im.add(strobj_("x"))
	}()
	defer func() {
		if recover() == nil {
			t.Fatal("subList out of range did not throw")
		}
	}()
	l.asListT().subList(2, 9)
}

func TestLgSet(t *testing.T) {
	s := newTSet_(tc_string)
	s.Add("a")
	st := s.asSetT()
	if (*TSet[string])(nil).asSetT() != nil {
		t.Fatal("null as a Set is not nil")
	}
	if Set_un_(st) != s.asSet() || lgBox_(st) != any(s.asSet()) || setAs_(tc_string, s.asSet()) != st || CollectionT[string](st) != s.asCollT() {
		t.Fatal("the typed set as a Set is not its view")
	}
	st.add("b") // the erased method, through the view
	st.add("a")
	if s.Len() != 2 || st.iterator_t().next_t() != "a" || !st.contains("b") || st.contains(nil) {
		t.Fatal(s.Len())
	}
	// an immutable typed set: the view for typed code, the twin for erased code, found from the twin
	is := immutableSetOf_(tc_string, "p", "q", "p")
	it := is.asSetT()
	if Set_un_(it) != Set(is.asImm()) || setAs_(tc_string, Set(is.asImm())) != it || collectionAs_(tc_string, Collection(is.asImm())) != CollectionT[string](it) {
		t.Fatal("the immutable typed set as a Set")
	}
	if iterableAs_(tc_string, Iterable(is.asImm())) != IterableT[string](is) {
		t.Fatal("the immutable typed set as an Iterable")
	}
	got := ""
	for e := jeachOf_[string](it); e.next(); {
		got += e.v
	}
	// an erased set: a wrapper that unboxes; erased code gets the set back
	hs := new_LinkedHashSet()
	hs.add("x")
	hs.add("y")
	w := setAs_(tc_string, Set(hs))
	if Set_un_(w) != Set(hs) || lgBox_(w) != any(hs) || w.size() != 2 {
		t.Fatal("a wrapped erased set")
	}
	for e := jeachOf_[string](w); e.next(); {
		got += e.v
	}
	for i := w.iterator_t(); i.hasNext(); {
		got += i.next_t()
	}
	if got != "pqxyxy" {
		t.Fatal(got)
	}
	if setAs_(tc_string, nil) != nil {
		t.Fatal("null")
	}
}

func TestLgKeySetT(t *testing.T) {
	m := newTMap_(tc_string, tc_string)
	m.Put("a", "1")
	m.Put("b", "2")
	ks := m.keySetT()
	if Set_un_(ks) != m.asMap().keySet() || lgBox_(ks) != any(m.asMap().keySet()) || setAs_(tc_string, m.asMap().keySet()) != ks {
		t.Fatal("keySetT of a map that can change is not the erased view's key set")
	}
	m.Put("c", "3")
	got := ""
	for e := jeachOf_[string](ks); e.next(); {
		got += e.v
	}
	it := ks.iterator_t()
	it.next_t()
	Iterator_remove(it)
	if got != "abc" || m.Len() != 2 || ks.contains("a") || !ks.contains("b") || !ks.remove_Object("c") || m.Len() != 1 {
		t.Fatal(got, m.Len())
	}
	// an immutable map: the typed view reads the map; erased code gets the twin's key set
	im := immutableMapOf1_(tc_string, tc_string, "k", "w")
	iks := im.keySetT()
	if iks.size() != 1 || iks.iterator_t().next_t() != "k" || !Set_un_(iks).contains("k") {
		t.Fatal("keySetT of an immutable map")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("clear of the key set of an immutable map did not throw")
			}
		}()
		iks.clear()
	}()
}

func TestLgMap(t *testing.T) {
	m := newTMap_(tc_string, tc_string)
	m.Put("a", "1")
	mt := m.asMapT()
	if (*TMap[string, string])(nil).asMapT() != nil || mapAs_(tc_string, tc_string, nil) != nil {
		t.Fatal("null as a Map is not nil")
	}
	if Map_un_(mt) != m.asMap() || lgBox_(mt) != any(m.asMap()) || mapAs_(tc_string, tc_string, m.asMap()) != mt {
		t.Fatal("the typed map as a Map is not its view")
	}
	mt.put("b", "2") // the erased method, through the view
	if m.Len() != 2 || mt.get_t("b") != "2" || mt.get_t("zz") != nullstr_ || mt.get_t(nil) != nullstr_ || mt.get_t(7) != nullstr_ {
		t.Fatal(m.Len())
	}
	if mt.keySet_t() != m.keySetT() || mt.values_t() != m.valuesT() || mt.entrySet_t() != m.entrySetT() || Set_un_(mt.entrySet_t()) != m.asMap().entrySet() {
		t.Fatal("the views of the typed Map are not the map's typed views")
	}
	got := ""
	for e := jeachOf_[Map_EntryT[string, string]](mt.entrySet_t()); e.next(); {
		got += e.v.getKey_t() + e.v.getValue_t()
	}
	it := mt.entrySet_t().iterator_t()
	en := it.next_t()
	if en.setValue("9") != any("1") || m.Get("a") != "9" || en.getValue_t() != "9" || Map_Entry_un_(en) != Map_Entry(en) {
		t.Fatal("an entry of a typed map does not write through")
	}
	Iterator_remove(it)
	if m.Len() != 1 || en.getValue_t() != "9" {
		t.Fatal("removing through the entry set")
	}
	// an immutable typed map: the view for typed code, the twin for erased code, found from the twin
	im := immutableMapOf1_(tc_string, tc_string, "k", "w")
	imt := im.asMapT()
	if Map_un_(imt) != Map(im.asImm()) || mapAs_(tc_string, tc_string, Map(im.asImm())) != imt || imt.get_t("k") != "w" {
		t.Fatal("the immutable typed map as a Map")
	}
	// an erased map: a wrapper that unboxes; erased code gets the map back
	hm := new_LinkedHashMap()
	hm.put("x", "1")
	hm.put("y", "2")
	w := mapAs_(tc_string, tc_string, Map(hm))
	if Map_un_(w) != Map(hm) || lgBox_(w) != any(hm) || w.size() != 2 || w.get_t("y") != "2" || w.get_t("q") != nullstr_ {
		t.Fatal("a wrapped erased map")
	}
	for e := jeachOf_[string](w.keySet_t()); e.next(); {
		got += e.v
	}
	for e := jeachOf_[string](w.values_t()); e.next(); {
		got += e.v
	}
	for e := jeachOf_[Map_EntryT[string, string]](w.entrySet_t()); e.next(); {
		got += e.v.getKey_t() + e.v.getValue_t()
		if lgBox_(e.v) == any(e.v) {
			t.Fatal("a wrapped entry reaches erased code")
		}
	}
	if got != "a1b2xy12x1y2" {
		t.Fatal(got)
	}
	// an entry of a loop over a typed map as a typed entry
	for c := m.eachEntry(); c.next(); {
		e := c.entry().asEntryT()
		if e.getKey_t() != "b" || e.getValue_t() != "2" || !objequals_(e, m.asMap().entrySet().iterator().next()) {
			t.Fatal("asEntryT")
		}
	}
	if (*TEntry[string, string])(nil).asEntryT() != nil || map_EntryAs_(tc_string, tc_string, nil) != nil {
		t.Fatal("null as an entry is not nil")
	}
}

// Piece `cls`: the value of a place of an immutable class entering a typed declaration is the
// typed collection whose view it is; a collection that is not immutable fails as the cast does.
func TestLgCls(t *testing.T) {
	il := immutableListOf_(tc_string, "p", "q")
	if tlistOfT_(tc_string, il.asListT()) != il || tlistOfT_(tc_string, il.asCollT()) != il || tlistOfT_(tc_string, nil) != nil {
		t.Fatal("the typed list of its view")
	}
	if il.view.twin != nil {
		t.Fatal("the twin of an immutable list was made although only typed code read it")
	}
	// an erased immutable list in a wrapper: the typed list of that object, made once
	el := ImmutableList_of_Object_Object(strobj_("a"), strobj_("b"))
	w := listAs_(tc_string, List(el))
	tl := tlistOfT_(tc_string, w)
	if tl.get(1) != "b" || tlistOfT_(tc_string, w) != tl || tl.asImm() != el {
		t.Fatal("the typed list of an erased immutable list")
	}
	is := immutableSetOf_(tc_string, "x")
	if tsetOfT_(tc_string, is.asSetT()) != is || tsetOfT_(tc_string, nil) != nil {
		t.Fatal("the typed set of its view")
	}
	im := immutableMapOf1_(tc_string, tc_string, "k", "v")
	if tmapOfT_(tc_string, tc_string, im.asMapT()) != im || tmapOfT_(tc_string, tc_string, nil) != nil {
		t.Fatal("the typed map of its view")
	}
	ml := newTList_(tc_string)
	ms := newTSet_(tc_string)
	mm := newTMap_(tc_string, tc_string)
	for i, f := range []func(){func() { tlistOfT_(tc_string, ml.asListT()) }, func() { tsetOfT_(tc_string, ms.asSetT()) }, func() { tmapOfT_(tc_string, tc_string, mm.asMapT()) }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("a collection that can change entered a place of an immutable class", i)
				}
			}()
			f()
		}()
	}
	// ArrayList's own method on the view of a list made for a typed place
	cast_[interface{ trimToSize() }](lgBox_(ml.asListT())).trimToSize()
}

// An erased LinkedHashMap keeps the wrapper typed code reads it through; the wrapper keeps its
// typed key set and values (live views).
func TestLgMapWrapperKept(t *testing.T) {
	m := new_LinkedHashMap()
	m.put(strobj_("a"), strobj_("x"))
	w := mapAs_(tc_string, tc_string, Map(m))
	if mapAs_(tc_string, tc_string, Map(m)) != w || Map_un_(w) != Map(m) || lgBox_(w) != any(m) {
		t.Fatal("the wrapper of an erased map is not kept, or is not the map for erased code")
	}
	ks, vs := w.keySet_t(), w.values_t()
	if w.keySet_t() != ks || w.values_t() != vs {
		t.Fatal("the typed views of a wrapper are not kept")
	}
	m.put(strobj_("b"), strobj_("y"))
	got := ""
	for e := jeachOf_[string](ks); e.next(); {
		got += e.v
	}
	for e := jeachOf_[string](vs); e.next(); {
		got += e.v
	}
	if got != "abxy" || w.get_t(strobj_("b")) != "y" {
		t.Fatal(got)
	}
	// another instantiation, and another map, get wrappers of their own
	w2 := mapAs_(tc_string, tc_Object, Map(m))
	if any(w2) == any(w) || w2.get_t(strobj_("a")) != any("x") {
		t.Fatal("a wrapper of another instantiation")
	}
	m2 := new_LinkedHashMap_Map(Map(m))
	if any(mapAs_(tc_string, tc_Object, Map(m2))) == any(w2) || Map_un_(mapAs_(tc_string, tc_Object, Map(m2))) != Map(m2) {
		t.Fatal("the wrapper of a copy is the original's")
	}
}

// Piece `meth`: the methods that take an element at its type, on views and on wrappers.
func TestLgMeth(t *testing.T) {
	l := newTList_(tc_string)
	lt := l.asListT()
	al := new_ArrayList_0()
	wl := listAs_(tc_string, List(al))
	for _, x := range []ListT[string]{lt, wl} {
		if !x.add_t("a") || !CollectionT[string](x).add_t("b") || x.set_t(0, "c") != "a" || x.remove_t(1) != "b" || x.size() != 1 || x.get_t(0) != "c" {
			t.Fatal("add_t / set_t / remove_t of a list")
		}
	}
	if l.get(0) != "c" || str_(al.get(0)) != "c" {
		t.Fatal("the typed methods did not write the list itself")
	}
	s := newTSet_(tc_string)
	hs := new_LinkedHashSet()
	for _, x := range []SetT[string]{s.asSetT(), setAs_(tc_string, Set(hs))} {
		if !x.add_t("a") || x.add_t("a") || x.size() != 1 {
			t.Fatal("add_t of a set")
		}
	}
	m := newTMap_(tc_string, tc_string)
	hm := new_LinkedHashMap()
	for _, x := range []MapT[string, string]{m.asMapT(), mapAs_(tc_string, tc_string, Map(hm))} {
		if x.put_t("k", "v") != nullstr_ || x.put_t("k", "w") != "v" || x.get_t(strobj_("k")) != "w" {
			t.Fatal("put_t")
		}
		for e := jeachOf_[Map_EntryT[string, string]](x.entrySet_t()); e.next(); {
			if e.v.setValue_t("z") != "w" || e.v.getValue_t() != "z" {
				t.Fatal("setValue_t")
			}
		}
		if x.remove_t(obj_(int32(1))) != nullstr_ || x.remove_t(strobj_("k")) != "z" || x.remove_t(strobj_("k")) != nullstr_ || x.size() != 0 {
			t.Fatal("remove_t of a map")
		}
	}
	il := immutableListOf_(tc_string, "p")
	im := immutableMapOf1_(tc_string, tc_string, "k", "v")
	for i, f := range []func(){func() { il.asListT().add_t("x") }, func() { il.asListT().set_t(0, "x") }, func() { il.asListT().remove_t(0) },
		func() { im.asMapT().put_t("a", "b") }, func() { im.asMapT().remove_t(strobj_("k")) }, func() { m.keySetT().add_t("x") }, func() { m.valuesT().add_t("x") }} {
		func() {
			defer func() {
				if recover() == nil {
					t.Fatal("a typed method changed an immutable collection or a view of a map", i)
				}
			}()
			f()
		}()
	}
}
