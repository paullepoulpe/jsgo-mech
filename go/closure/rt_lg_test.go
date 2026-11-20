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
