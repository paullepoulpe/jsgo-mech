package closure

import (
	"fmt"
	"math/rand"
	"testing"
)

// propsModel_ is Node.java's property list as Java keeps it: the translated list items, put and
// removed the way Node.putProp, putIntProp and removeProp do it in Java.
type propsModel_ struct{ head *Node_PropListItem }

func (m *propsModel_) putInt(p *Node_Prop, v int32) {
	m.head = Node_rebuildListWithoutProp(m.head, p)
	if v != 0 {
		m.head = new_Node_IntPropListItem(int8(p.ordinal()), v, m.head).up_Node_PropListItem()
	}
}

func (m *propsModel_) putObj(p *Node_Prop, v any) {
	m.head = Node_rebuildListWithoutProp(m.head, p)
	if v != nil {
		m.head = new_Node_ObjectPropListItem(int8(p.ordinal()), v, m.head).up_Node_PropListItem()
	}
}

func propsListText_(l *Node_PropListItem) string {
	s := ""
	for x := l; x != nil; x = x.next_ {
		switch it := x.self_.(type) {
		case *Node_IntPropListItem:
			s += fmt.Sprintf("[%d int %d]", x.propType, it.intValue)
		case *Node_ObjectPropListItem:
			s += fmt.Sprintf("[%d obj %p]", x.propType, it.objectValue)
		}
	}
	return s
}

// propsCall_ is f's result as text, or "panic" if it throws.
func propsCall_(f func() any) (s string) {
	defer func() {
		if recover() != nil {
			s = "panic"
		}
	}()
	return fmt.Sprintf("%v", f())
}

// Node's property methods (rt_node_props.go) against Java's list: after every operation the
// list the node would hold in Java, item by item and in order, and what getProp, getIntProp and
// getBooleanProp answer or throw for every Prop.
func TestNodePropsAgainstJavaList(t *testing.T) {
	props := Node_Prop_values()
	objs := []any{&struct{ a int }{1}, &struct{ a int }{2}, &struct{ b string }{"x"}}
	ints := []int32{0, 1, 1, 1, 1, 2, 3, 4, 15, 16, -1, 1 << 20}
	rnd := rand.New(rand.NewSource(1))
	// Mostly a few properties, as real nodes have, and sometimes any of them.
	pick := func() *Node_Prop {
		if rnd.Intn(4) != 0 {
			return props[[]int{nodePropSource_, nodePropSideEffects_, nodePropConstVar_, 0, 1, 2, 3, 30}[rnd.Intn(8)]]
		}
		return props[rnd.Intn(len(props))]
	}
	type pair struct {
		n Node
		m *propsModel_
	}
	var nodes []pair
	fresh := func() pair {
		p := pair{new_Node_Token(Token_rhino_NAME), &propsModel_{}}
		nodes = append(nodes, p)
		return p
	}
	check := func(step int, what string, p pair) {
		t.Helper()
		got, want := propsListText_(p.n.propListHead_get_()), propsListText_(p.m.head)
		if got != want {
			t.Fatalf("step %d (%s): list %s, Java's %s", step, what, got, want)
		}
		if p.n.propListHead_isnull_() != (p.m.head == nil) {
			t.Fatalf("step %d (%s): isnull %v", step, what, p.n.propListHead_isnull_())
		}
		for _, prop := range props {
			item := func() *Node_PropListItem {
				x := p.m.head
				for x != nil && int32(x.propType) != prop.ordinal() {
					x = x.next_
				}
				return x
			}()
			wi := propsCall_(func() any {
				if item == nil {
					return int32(0)
				}
				return item.getIntValue()
			})
			wo := propsCall_(func() any {
				if item == nil {
					return nil
				}
				return item.getObjectValue()
			})
			wb := wi
			if wi != "panic" {
				wb = fmt.Sprint(wi != "0")
			}
			if g := propsCall_(func() any { return p.n.getIntProp(prop) }); g != wi {
				t.Fatalf("step %d (%s): getIntProp(%s) %s, Java's %s; list %s", step, what, prop.name(), g, wi, want)
			}
			if g := propsCall_(func() any { return p.n.getBooleanProp(prop) }); g != wb {
				t.Fatalf("step %d (%s): getBooleanProp(%s) %s, Java's %s; list %s", step, what, prop.name(), g, wb, want)
			}
			if g := propsCall_(func() any { return p.n.getProp(prop) }); g != wo {
				t.Fatalf("step %d (%s): getProp(%s) %s, Java's %s; list %s", step, what, prop.name(), g, wo, want)
			}
		}
	}
	cur := fresh()
	for step := 0; step < 30000; step++ {
		what := ""
		switch k := rnd.Intn(100); {
		case k < 40:
			p, v := pick(), ints[rnd.Intn(len(ints))]
			what = fmt.Sprintf("putIntProp %s %d", p.name(), v)
			cur.n.putIntProp(p, v)
			cur.m.putInt(p, v)
		case k < 60:
			p := pick()
			var v any
			if rnd.Intn(8) != 0 {
				v = objs[rnd.Intn(len(objs))]
			}
			what = fmt.Sprintf("putProp %s %p", p.name(), v)
			cur.n.putProp(p, v)
			cur.m.putObj(p, v)
		case k < 75:
			p := pick()
			what = "removeProp " + p.name()
			cur.n.removeProp(p)
			cur.m.putInt(p, 0)
		case k < 80:
			// clonePropsFrom and cloneNode: the same list in another node.
			what = "copy"
			from := cur
			cur = fresh()
			cur.n.propListHead_copy_(from.n)
			cur.m.head = from.m.head
			check(step, "copy source", from)
		case k < 84:
			// setStaticSourceFileFrom: the tail of another node's list.
			what = "tail"
			from := nodes[rnd.Intn(len(nodes))]
			tail := from.m.head
			for tail != nil && tail.next_ != nil {
				tail = tail.next_
			}
			cur.n.propListHead_set_(tail)
			cur.m.head = tail
		case k < 87:
			// deserialization: an item put in front of the list without removing its property.
			p := pick()
			what = "push " + p.name()
			l := new_Node_IntPropListItem(int8(p.ordinal()), ints[1+rnd.Intn(len(ints)-1)], cur.m.head).up_Node_PropListItem()
			cur.n.propListHead_set_(l)
			cur.m.head = l
		case k < 90:
			what = "empty"
			cur.n.propListHead_set_(nil)
			cur.m.head = nil
		case k < 95:
			what = "switch"
			cur = nodes[rnd.Intn(len(nodes))]
		default:
			what = "fresh"
			cur = fresh()
		}
		check(step, what, cur)
		if step%1000 == 0 {
			for _, p := range nodes {
				check(step, "all nodes after "+what, p)
			}
		}
	}
}

// A node's JSType is kept in the table of distinct values as the *JSType itself (post-flip
// manual step), so getJSType is one type comparison; whatever reads the field as an Object
// still sees the type's own object, a nil type clears the field, and colors are unaffected.
func TestNodeJSTypeField(t *testing.T) {
	nt := &NumberType{}
	nt.self_ = nt
	base := nt.as_JSType()
	n := new_Node_Token(Token_rhino_NAME)
	if n.getJSType() != nil || n.jstypeOrColor_get_() != nil {
		t.Fatal("a new node has a type")
	}
	n.setJSType(base)
	if n.getJSType() != base {
		t.Fatal("getJSType is not the type that was set")
	}
	if n.jstypeOrColor_get_() != any(nt) {
		t.Fatalf("the field as an Object is %T, want the type's own object", n.jstypeOrColor_get_())
	}
	if n.getColor() != nil {
		t.Fatal("a JSType reads as a color")
	}
	m := new_Node_Token(Token_rhino_NAME)
	m.copyTypeFrom(n)
	if m.getJSType() != base || m.jstypeOrColor_get_() != any(nt) {
		t.Fatal("copyTypeFrom lost the type")
	}
	n.setJSType(nil)
	if n.getJSType() != nil || n.jstypeOrColor_get_() != nil || n.cs_().jstypeOrColor != 0 {
		t.Fatal("setJSType(nil) left a type")
	}
	if m.getJSType() != base {
		t.Fatal("clearing one node's type cleared another's")
	}
	m.setJSType(nil)
}
