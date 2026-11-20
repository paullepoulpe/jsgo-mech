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
	for e := eachOf_(it); e.next(); {
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
	if iterableAs_(tc_string, Iterable(l.asList())) != IterableT[string](&l.view) {
		t.Fatal("the view of a typed list was wrapped")
	}
	// the loop sees what the body appends, as the erased loop over an ArrayList does
	n := 0
	for e := eachOf_(it); e.next(); n++ {
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
	for e := eachOf_(w); e.next(); {
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
	e := eachOf_(w2)
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
	s.add("k1")
	s.add("k2")
	got := ""
	for e := eachOf_(s.asIterable()); e.next(); {
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
		m.put(k, k+k)
	}
	m.remove("b")
	ks, vs := Iterable(m.asMap().keySet()), Iterable(m.asMap().values())
	kt, vt := iterableAs_(tc_string, ks), iterableAs_(tc_string, vs)
	if any(kt) != any(ks) || any(vt) != any(vs) {
		t.Fatal("a view of a typed map was wrapped")
	}
	got := ""
	for e := eachOf_(kt); e.next(); {
		got += e.v
	}
	for e := eachOf_(vt); e.next(); {
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
	if got != "acd"+"aaccdd"+"acd"+"aadd" || m.size() != 2 || m.containsKey("c") {
		t.Fatal(got, m.size())
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
	for e := eachOf_[string](ic); e.next(); {
		got += e.v
	}
	// an erased collection: a wrapper that unboxes; erased code gets the collection back
	al := new_ArrayList_0()
	al.add(strobj_("m"))
	w := collectionAs_(tc_string, Collection(al))
	for e := eachOf_[string](w); e.next(); {
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
	m.put("a", "x")
	m.put("b", "y")
	v := m.valuesT()
	if Collection_un_(v) != m.asMap().values() || lgBox_(v) != any(m.asMap().values()) {
		t.Fatal("valuesT of a map that can change is not the erased view's values()")
	}
	m.put("c", "z")
	got := ""
	for e := eachOf_[string](v); e.next(); {
		got += e.v
	}
	im := immutableMapOf1_(tc_string, tc_string, "k", "w")
	iv := im.valuesT()
	for e := eachOf_[string](iv); e.next(); {
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
	for e := eachOf_[string](l.asListT()); e.next(); {
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
	if l.get(1) != "B" || tostr_(s) != "[B, c]" {
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
