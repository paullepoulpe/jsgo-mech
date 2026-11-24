package closure

// Rule "typed library interfaces", piece `objlist` (tools/j2go LibObj.listTwin; RESIDUE8 T3.1,
// docs/libgen/COLLECTIONS-PLAN.md section 16): contains / remove(Object) and indexOf /
// lastIndexOf of a list (or a collection) whose element type's equals is Go's == (a String, an
// enum, a class of the program without equals or hashCode of its own), called with an argument
// of the element type. The erased call boxed the argument and the view's indexOf boxed every
// element to compare them with objequals_ (Object.equals, which for such a type is ==; for a
// String its contents, as Go's == and the view's own string scan, indexOfStr_); the twins scan
// the typed list with ==. A null argument takes the erased method (nil in both translations).
//
// The TList twins take the typed list itself; the ListT and CollectionT twins scan where the
// object is a typed list's view (a type assertion to *tlistView_[T]) or call a typed set's own
// find (tset_), else the erased method with the argument boxed as erased code boxes it (boxT_).
// The translator calls those only for element types that have a typed list (and set) already
// (tools/j2go/objarg-pairs.txt, lines `@on objlist`), so they make Go compile no new view.

// TList_indexOfE_ is l.indexOf(x) (tlistView_.indexOf).
func TList_indexOfE_[T comparable](l *TList[T], x T) int32 {
	if x == l.c.null {
		return l.view.indexOf(boxT_(x))
	}
	for i, y := range l.items {
		if y == x {
			return int32(i)
		}
	}
	return -1
}

// TList_lastIndexOfE_ is l.lastIndexOf(x) (tlistView_.lastIndexOf).
func TList_lastIndexOfE_[T comparable](l *TList[T], x T) int32 {
	if x == l.c.null {
		return l.view.lastIndexOf(boxT_(x))
	}
	for i := len(l.items) - 1; i >= 0; i-- {
		if l.items[i] == x {
			return int32(i)
		}
	}
	return -1
}

// TList_containsE_ is l.contains(x) (tlistView_.contains).
func TList_containsE_[T comparable](l *TList[T], x T) bool { return TList_indexOfE_(l, x) >= 0 }

// TList_removeE_ is l.remove(x), remove(Object) (tlistView_.remove_Object).
func TList_removeE_[T comparable](l *TList[T], x T) bool {
	if x == l.c.null {
		return l.view.remove_Object(boxT_(x))
	}
	i := TList_indexOfE_(l, x)
	if i < 0 {
		return false
	}
	l.removeAt(i)
	return true
}

// List_indexOfE_ ... List_removeE_: the same on a ListT receiver.
func List_indexOfE_[T comparable](l ListT[T], x T) int32 {
	if v, ok := l.(*tlistView_[T]); ok && x != v.l.c.null {
		return TList_indexOfE_(v.l, x)
	}
	return l.indexOf(boxT_(x))
}

func List_lastIndexOfE_[T comparable](l ListT[T], x T) int32 {
	if v, ok := l.(*tlistView_[T]); ok && x != v.l.c.null {
		return TList_lastIndexOfE_(v.l, x)
	}
	return l.lastIndexOf(boxT_(x))
}

func List_containsE_[T comparable](l ListT[T], x T) bool {
	if v, ok := l.(*tlistView_[T]); ok && x != v.l.c.null {
		return TList_indexOfE_(v.l, x) >= 0
	}
	return l.contains(boxT_(x))
}

func List_removeE_[T comparable](l ListT[T], x T) bool {
	if v, ok := l.(*tlistView_[T]); ok && x != v.l.c.null {
		return TList_removeE_(v.l, x)
	}
	return l.remove_Object(boxT_(x))
}

// Collection_containsE_ / Collection_removeE_: on a CollectionT receiver, whose object may be a
// typed list's view or a typed set's (Set_containsE_).
func Collection_containsE_[T comparable](c CollectionT[T], x T) bool {
	switch v := c.(type) {
	case *tlistView_[T]:
		if x != v.l.c.null {
			return TList_indexOfE_(v.l, x) >= 0
		}
	case tsetOf_:
		if t := (*TSet[T])(v.tset_()); x != t.c.null {
			return t.find(x) >= 0
		}
	}
	return c.contains(boxT_(x))
}

func Collection_removeE_[T comparable](c CollectionT[T], x T) bool {
	switch v := c.(type) {
	case *tlistView_[T]:
		if x != v.l.c.null {
			return TList_removeE_(v.l, x)
		}
	case tsetOf_:
		if t := (*TSet[T])(v.tset_()); x != t.c.null {
			return t.remove(x)
		}
	}
	return c.remove_Object(boxT_(x))
}
