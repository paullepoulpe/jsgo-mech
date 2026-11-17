package closure

// The property list of a Node (tools/j2go/handles.txt: "Node::propListHead runtime Node_props_"
// and the Node methods marked runtime there).
//
// Java keeps a node's properties as an immutable linked list of PropListItem, the most recently
// put first, which clonePropsFrom and cloneNode share between nodes. Here a node keeps them in 16
// pointer-free bytes of its record, Node_props_:
//
//	bits  bit p set: Prop p holds the int 1, which is what every boolean property holds; the top
//	      bits, which no Prop uses, hold two small-int properties: CONSTANT_VAR_FLAGS (bits 57-58)
//	      and SIDE_EFFECT_FLAGS (bits 59-62), each while its value fits (0 there: not set);
//	src   SOURCE_FILE holding an object (nearly every node has one): its handle in the table of
//	      distinct values (rt_ids.go, htab_); 0 when it holds none;
//	aux   bit 0 clear: the list's order inline, for lists of at most 4 properties that need
//	      nothing but bits and src: bits 1-3 the count, then 6 bits per property, head first;
//	      bit 0 set: the index (aux>>1) of a side entry, nodePropExt_, which only nodes that need
//	      one occupy: the order of a longer list, the values that are neither the int 1 nor
//	      SOURCE_FILE's object (other ints, other objects), or, for a list this form cannot hold
//	      (one naming a property twice, which only a list built by hand can do), the list itself.
//
// So getBooleanProp, getIntProp, getProp, putIntProp, putProp and removeProp (written here) read
// and write bits for the common case and allocate nothing; the order of the list, which a few
// methods observe (getPropListDebugString, validateProperties, the tail checks of
// deserialization), is kept exactly. Every other method of Node that reads the field is
// translated and reads the list Java would have: propListHead_get_ builds it (a list of at most
// 4 properties is built once and shared, as its items are immutable); propListHead_set_ takes
// one apart. What is not kept is the identity of list items: two nodes whose lists Java shares
// and two nodes with equal lists get the same items or different ones alike. Only
// validateMemorySensitivePropertyGuarantees, which tests call, compares items by identity.

// Ordinals of the Props this file treats apart (checked at init against Node.Prop).
const (
	nodePropSideEffects_ = 8  // SIDE_EFFECT_FLAGS
	nodePropSource_      = 14 // SOURCE_FILE
	nodePropConstVar_    = 25 // CONSTANT_VAR_FLAGS

	nodePropConstVarShift_ = 57
	nodePropConstVarMax_   = 3
	nodePropSideShift_     = 59
	nodePropSideMax_       = 15

	// The Props whose int 1 is a bit and that hold nothing else outside a side entry.
	nodePropPlain_ = (1<<57 - 1) &^ (1<<nodePropSideEffects_ | 1<<nodePropSource_ | 1<<nodePropConstVar_)
)

func init() {
	if Node_Prop_SOURCE_FILE == nil {
		return // not initialized yet (a build with lazy class initialization: the self-test)
	}
	if Node_Prop_SIDE_EFFECT_FLAGS.ordinal() != nodePropSideEffects_ || Node_Prop_SOURCE_FILE.ordinal() != nodePropSource_ ||
		Node_Prop_CONSTANT_VAR_FLAGS.ordinal() != nodePropConstVar_ || len(Node_Prop_values()) > 57 {
		panic("rt_node_props.go: the ordinals of Node.Prop changed")
	}
}

// nodePropSmall_ is the shift and largest inline value of small-int property o (0, 0 if none).
func nodePropSmall_(o uint8) (shift uint8, max int32) {
	switch o {
	case nodePropConstVar_:
		return nodePropConstVarShift_, nodePropConstVarMax_
	case nodePropSideEffects_:
		return nodePropSideShift_, nodePropSideMax_
	}
	return 0, 0
}

type Node_props_ struct {
	bits uint64
	src  uint32
	aux  uint32
}

type nodePropVal_ struct {
	obj  any // the object a property holds; nil for an int
	i    int32
	prop uint8
}

type nodePropExt_ struct {
	order []uint8 // the properties of the list, tail (least recently put) first
	vals  []nodePropVal_
	raw   *Node_PropListItem
}

var (
	nodePropExts_  idTable_[nodePropExt_]
	nodePropExts0_ = nodePropExts_.reserve0()
	nodePropFree_  []uint32
	nodePropLists_ = map[uint64]*Node_PropListItem{}
)

func nodePropUOE_() {
	panic(new_UnsupportedOperationException_0().self())
}

func (r *Node_props_) ext() *nodePropExt_ {
	return nodePropExts_.ptr(r.aux >> 1)
}

func (r *Node_props_) inlineCount() uint32 {
	return r.aux >> 1 & 7
}

func (r *Node_props_) inlineAt(i uint32) uint8 {
	return uint8(r.aux >> (4 + 6*i) & 63)
}

// toExt moves an inline order to a new side entry.
func (r *Node_props_) toExt() *nodePropExt_ {
	var idx uint32
	if n := len(nodePropFree_); n > 0 {
		idx = nodePropFree_[n-1]
		nodePropFree_ = nodePropFree_[:n-1]
	} else {
		idx = nodePropExts_.add(nodePropExt_{})
		if idx >= 1<<31 {
			panic(new_RuntimeException_String("node properties: more than 2^31 side entries").self())
		}
	}
	e := nodePropExts_.ptr(idx)
	for i := int(r.inlineCount()) - 1; i >= 0; i-- {
		e.order = append(e.order, r.inlineAt(uint32(i)))
	}
	r.aux = idx<<1 | 1
	return e
}

// shrink returns a side entry that is no longer needed and keeps the order inline again.
func (r *Node_props_) shrink() {
	e := r.ext()
	if e.raw != nil || len(e.vals) != 0 || len(e.order) > 4 {
		return
	}
	var aux uint32
	for i, o := range e.order {
		// e.order is tail first: the head is the last one.
		aux |= uint32(o) << (4 + 6*uint32(len(e.order)-1-i))
	}
	aux |= uint32(len(e.order)) << 1
	nodePropFree_ = append(nodePropFree_, r.aux>>1)
	*e = nodePropExt_{order: e.order[:0], vals: e.vals[:0]}
	r.aux = aux
}

// remove takes property o out of the list, wherever it is (no-op if it is not there).
func (r *Node_props_) remove(o uint8) {
	sh, max := nodePropSmall_(o)
	switch {
	case sh == 0 && r.bits>>o&1 != 0:
		r.bits &^= 1 << o
	case sh != 0 && r.bits>>sh&uint64(max) != 0:
		r.bits &^= uint64(max) << sh
	case r.src != 0 && o == nodePropSource_:
		r.src = 0
	case r.aux&1 != 0:
		e := r.ext()
		found := false
		for i, v := range e.vals {
			if v.prop == o {
				e.vals = append(e.vals[:i], e.vals[i+1:]...)
				found = true
				break
			}
		}
		if !found {
			return
		}
	default:
		return
	}
	if r.aux&1 == 0 {
		n := r.inlineCount()
		var aux uint32
		k := uint32(0)
		for i := uint32(0); i < n; i++ {
			if x := r.inlineAt(i); x != o {
				aux |= uint32(x) << (4 + 6*k)
				k++
			}
		}
		r.aux = aux | k<<1
		return
	}
	e := r.ext()
	for i, x := range e.order {
		if x == o {
			e.order = append(e.order[:i], e.order[i+1:]...)
			break
		}
	}
	r.shrink()
}

// pushFront puts property o at the head of the list; ext says it needs a side entry.
func (r *Node_props_) pushFront(o uint8, ext bool) *nodePropExt_ {
	if r.aux&1 == 0 {
		n := r.inlineCount()
		if !ext && n < 4 {
			r.aux = (r.aux>>4)<<10 | uint32(o)<<4 | (n+1)<<1
			return nil
		}
		r.toExt()
	}
	e := r.ext()
	e.order = append(e.order, o)
	return e
}

// lookup finds property o: 0 absent, 1 an int (v), 2 an object (obj). A list kept raw is not read here.
func (r *Node_props_) lookup(o uint8) (kind int, v int32, obj any) {
	if sh, max := nodePropSmall_(o); sh != 0 {
		if v := int32(r.bits>>sh) & max; v != 0 {
			return 1, v, nil
		}
	} else if r.bits>>o&1 != 0 {
		return 1, 1, nil
	}
	if r.src != 0 && o == nodePropSource_ {
		return 2, 0, htab_get_(r.src)
	}
	if r.aux&1 != 0 {
		for _, x := range r.ext().vals {
			if x.prop == o {
				if x.obj != nil {
					return 2, 0, x.obj
				}
				return 1, x.i, nil
			}
		}
	}
	return 0, 0, nil
}

func (r *Node_props_) rawList() *Node_PropListItem {
	if r.aux&1 == 0 {
		return nil
	}
	return r.ext().raw
}

func nodePropRawLookup_(x *Node_PropListItem, o uint8) *Node_PropListItem {
	for x != nil && uint8(x.propType) != o {
		x = x.next_
	}
	return x
}

// setRaw keeps l as the list (it names a property twice); nil empties the list.
func (r *Node_props_) setRaw(l *Node_PropListItem) {
	if l == nil {
		r.free()
		return
	}
	r.ext().raw = l
}

// ---------------------------------------------------------------- Node's methods

func (this Node) getIntProp(propType *Node_Prop) int32 {
	r := this.propListHead_p_()
	o := uint8(propType.ordinal_)
	if r.bits>>o&1 != 0 {
		return 1
	}
	if r.aux&1 == 0 && nodePropPlain_>>o&1 != 0 {
		return 0
	}
	return r.intSlow(o)
}

func (this Node) getBooleanProp(propType *Node_Prop) bool {
	r := this.propListHead_p_()
	o := uint8(propType.ordinal_)
	if r.bits>>o&1 != 0 {
		return true
	}
	if r.aux&1 == 0 && nodePropPlain_>>o&1 != 0 {
		return false
	}
	return r.intSlow(o) != 0
}

func (r *Node_props_) intSlow(o uint8) int32 {
	if raw := r.rawList(); raw != nil {
		if item := nodePropRawLookup_(raw, o); item != nil {
			return item.getIntValue()
		}
		return 0
	}
	kind, v, _ := r.lookup(o)
	if kind == 2 {
		nodePropUOE_() // ObjectPropListItem.getIntValue
	}
	return v
}

func (this Node) getProp(propType *Node_Prop) any {
	r := this.propListHead_p_()
	if r.src != 0 && propType == Node_Prop_SOURCE_FILE {
		return htab_get_(r.src)
	}
	o := uint8(propType.ordinal_)
	if r.bits>>o&1 != 0 && o < 57 {
		nodePropUOE_() // IntPropListItem.getObjectValue
	}
	if r.aux&1 == 0 && nodePropPlain_>>o&1 != 0 {
		return nil
	}
	if raw := r.rawList(); raw != nil {
		if item := nodePropRawLookup_(raw, o); item != nil {
			return item.getObjectValue()
		}
		return nil
	}
	kind, _, obj := r.lookup(o)
	if kind == 1 {
		nodePropUOE_()
	}
	return obj
}

func (this Node) putIntProp(prop *Node_Prop, value int32) {
	r := this.propListHead_p_()
	if raw := r.rawList(); raw != nil {
		l := Node_rebuildListWithoutProp(raw, prop)
		if value != 0 {
			l = new_Node_IntPropListItem(int8(prop.ordinal()), value, l).up_Node_PropListItem()
		}
		r.setRaw(l)
		return
	}
	o := uint8(prop.ordinal_)
	r.remove(o)
	if value == 0 {
		return
	}
	r.putInt(o, value)
}

// putInt puts int property o (absent) at the head of the list.
func (r *Node_props_) putInt(o uint8, value int32) {
	if sh, max := nodePropSmall_(o); sh != 0 {
		if value >= 1 && value <= max {
			r.bits |= uint64(value) << sh
			r.pushFront(o, false)
			return
		}
	} else if value == 1 {
		r.bits |= 1 << o
		r.pushFront(o, false)
		return
	}
	e := r.pushFront(o, true)
	e.vals = append(e.vals, nodePropVal_{i: value, prop: o})
}

func (this Node) putProp(prop *Node_Prop, value any) {
	r := this.propListHead_p_()
	if raw := r.rawList(); raw != nil {
		l := Node_rebuildListWithoutProp(raw, prop)
		if value != nil {
			l = new_Node_ObjectPropListItem(int8(prop.ordinal()), value, l).up_Node_PropListItem()
		}
		r.setRaw(l)
		return
	}
	o := uint8(prop.ordinal_)
	r.remove(o)
	if value == nil {
		return
	}
	r.putObj(o, value)
}

// putObj puts object property o (absent) at the head of the list.
func (r *Node_props_) putObj(o uint8, value any) {
	if o == nodePropSource_ {
		r.src = htab_(value, false)
		r.pushFront(o, false)
		return
	}
	e := r.pushFront(o, true)
	e.vals = append(e.vals, nodePropVal_{obj: value, prop: o})
}

func (this Node) removeProp(prop *Node_Prop) {
	r := this.propListHead_p_()
	if raw := r.rawList(); raw != nil {
		r.setRaw(Node_rebuildListWithoutProp(raw, prop))
		return
	}
	r.remove(uint8(prop.ordinal_))
}

// ---------------------------------------------------------------- the field as Java sees it

func (this Node) propListHead_isnull_() bool {
	return *this.propListHead_p_() == Node_props_{}
}

// propListHead_copy_ gives this node the list of o (Java shares the immutable list).
func (this Node) propListHead_copy_(o Node) {
	if this == o {
		return
	}
	r, from := this.propListHead_p_(), *o.propListHead_p_()
	if r.aux&1 != 0 {
		r.free()
	}
	*r = from
	if from.aux&1 != 0 {
		fe := *from.ext()
		r.aux = 0
		e := r.toExt()
		e.order = append(e.order[:0], fe.order...)
		e.vals = append(e.vals[:0], fe.vals...)
		e.raw = fe.raw
	}
}

func (r *Node_props_) free() {
	e := r.ext()
	*e = nodePropExt_{order: e.order[:0], vals: e.vals[:0]}
	nodePropFree_ = append(nodePropFree_, r.aux>>1)
	*r = Node_props_{}
}

// propListHead_get_ is the list Java would hold.
func (this Node) propListHead_get_() *Node_PropListItem {
	r := this.propListHead_p_()
	if r.aux&1 == 0 {
		if r.aux == 0 {
			return nil
		}
		key := uint64(r.aux)<<32 | uint64(r.src)
		if l, ok := nodePropLists_[key]; ok {
			return l
		}
		var l *Node_PropListItem
		for i := int(r.inlineCount()) - 1; i >= 0; i-- {
			l = r.item(r.inlineAt(uint32(i)), l)
		}
		nodePropLists_[key] = l
		return l
	}
	e := r.ext()
	if e.raw != nil {
		return e.raw
	}
	var l *Node_PropListItem
	for _, o := range e.order {
		l = r.item(o, l)
	}
	return l
}

func (r *Node_props_) item(o uint8, next *Node_PropListItem) *Node_PropListItem {
	kind, v, obj := r.lookup(o)
	if kind == 2 {
		return new_Node_ObjectPropListItem(int8(o), obj, next).up_Node_PropListItem()
	}
	return new_Node_IntPropListItem(int8(o), v, next).up_Node_PropListItem()
}

// propListHead_set_ makes l the list.
func (this Node) propListHead_set_(l *Node_PropListItem) {
	r := this.propListHead_p_()
	if r.aux&1 != 0 {
		r.free()
	}
	*r = Node_props_{}
	var items []*Node_PropListItem
	var seen uint64
	dup := false
	for x := l; x != nil; x = x.next_ {
		o := uint8(x.propType)
		if o >= 57 || seen>>o&1 != 0 {
			dup = true
		}
		seen |= 1 << (o & 63)
		items = append(items, x)
	}
	if dup {
		r.toExt()
		r.setRaw(l)
		return
	}
	for i := len(items) - 1; i >= 0; i-- {
		x := items[i]
		o := uint8(x.propType)
		switch it := x.self_.(type) {
		case *Node_IntPropListItem:
			r.putInt(o, it.intValue)
		case *Node_ObjectPropListItem:
			r.putObj(o, it.objectValue)
		}
	}
}
