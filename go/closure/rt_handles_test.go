package closure

import (
	"reflect"
	"testing"
)

// pointerIn_ names a part of type t that holds a Go pointer (or is one), or returns "".
func pointerIn_(t reflect.Type, path string) string {
	switch t.Kind() {
	case reflect.Bool, reflect.Int, reflect.Int8, reflect.Int16, reflect.Int32, reflect.Int64,
		reflect.Uint, reflect.Uint8, reflect.Uint16, reflect.Uint32, reflect.Uint64, reflect.Uintptr,
		reflect.Float32, reflect.Float64, reflect.Complex64, reflect.Complex128:
		return ""
	case reflect.Array:
		if t.Len() == 0 {
			return ""
		}
		return pointerIn_(t.Elem(), path+"[i]")
	case reflect.Struct:
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if p := pointerIn_(f.Type, path+"."+f.Name); p != "" {
				return p
			}
		}
		return ""
	}
	return path + " (" + t.String() + ")"
}

// Every per-object array of Node (its records and columns, tools/j2go/handles.txt) must be pointer-free,
// so that the collector never scans the tree: a rule that brought a Go pointer back fails here.
func TestNodeColumnsHoldNoPointers(t *testing.T) {
	n := 0
	for _, c := range handleColumns_ {
		if c.class != "Node" {
			continue
		}
		n++
		if p := pointerIn_(reflect.TypeOf(c.zero), c.name); p != "" {
			t.Errorf("a per-node array holds a Go pointer: %s", p)
		}
	}
	if n == 0 {
		t.Fatal("no per-object arrays are registered for Node")
	}
}

func TestPointerIn(t *testing.T) {
	type ok struct {
		a uint32
		b [2]struct{ c int8 }
	}
	type bad struct {
		a uint32
		s string
	}
	if p := pointerIn_(reflect.TypeOf(ok{}), "ok"); p != "" {
		t.Errorf("ok: %s", p)
	}
	if p := pointerIn_(reflect.TypeOf(bad{}), "bad"); p != "bad.s (string)" {
		t.Errorf("bad: %q", p)
	}
	if p := pointerIn_(reflect.TypeOf([]int{}), "x"); p == "" {
		t.Error("a slice holds a pointer")
	}
}
