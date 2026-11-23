package closure

import "testing"

// threwNPE reports whether f throws Java's NullPointerException (directly or as the Go
// runtime's nil dereference, which catch_ maps to it).
func threwNPE(f func()) (npe bool) {
	defer func() {
		if r := recover(); r != nil {
			_, npe = catch_(r).(*NullPointerException)
		}
	}()
	f()
	return false
}

// TestNullListForEach: a for-each over a null list throws NullPointerException in Java and
// for a typed list; a list that is a pointer to a slice keeps that (its nil pointer), but a
// nil []T reads as an empty list, so tools/postflip/native makes a list a plain slice only
// when it can never be null.
func TestNullListForEach(t *testing.T) {
	var l *TList[string]
	if !threwNPE(func() {
		for range l.All() {
		}
	}) {
		t.Error("for-each over a null typed list should throw NullPointerException")
	}
	var p *[]string
	if !threwNPE(func() {
		for range *p {
		}
	}) {
		t.Error("range over a null pointer to a slice should throw NullPointerException")
	}
	if !threwNPE(func() {
		for range sliceAll_(p) {
		}
	}) {
		t.Error("sliceAll_ over a null pointer to a slice should throw NullPointerException")
	}
	if !threwNPE(func() { sliceCursor_(p) }) {
		t.Error("an iterator over a null pointer to a slice should throw NullPointerException")
	}
	if sliceViewP_(tc_string, p) != nil || slicePtrView_(tc_string, p) != nil {
		t.Error("the view of a null list should be null")
	}
	// The difference a plain slice cannot represent: nil runs no step and throws nothing.
	var s []string
	steps := 0
	if threwNPE(func() {
		for range s {
			steps++
		}
	}) || steps != 0 {
		t.Error("a nil slice should read as empty")
	}
}

// TestNullMapReads: reading a null Map or Set throws NullPointerException in Java; a nil Go
// map reads as an empty one, so the helpers of rt_native.go check for nil and
// tools/postflip/nilmaps wraps the inline reads of a map that may be nil in mapNN_.
func TestNullMapReads(t *testing.T) {
	var m map[string]string
	var s map[string]struct{}
	for name, f := range map[string]func(){
		"mapNN_":         func() { _ = mapNN_(m)["a"] },
		"mapAt_":         func() { mapAt_(m, "a") },
		"mapAt2_":        func() { mapAt2_(m, "a") },
		"mapDelete_":     func() { mapDelete_(m, "a") },
		"has_":           func() { has_(m, "a") },
		"mapStr_":        func() { mapStr_(m, "a") },
		"mapGetOr_":      func() { mapGetOr_(m, "a", "b") },
		"mapRemove_":     func() { mapRemove_(tc_string, m, "a") },
		"mapPut_":        func() { mapPut_(tc_string, m, "a", "b") },
		"setAdd_":        func() { setAdd_(s, "a") },
		"setRemove_":     func() { setRemove_(s, "a") },
		"setAddAll_":     func() { setAddAll_(s, map[string]struct{}{}) },
		"setAddAll_/2":   func() { setAddAll_(map[string]struct{}{}, s) },
		"setAddAllFrom_": func() { setAddAllFrom_(tc_string, s, Collection(new_ArrayList_0())) },
		"mapComputeIfAbsent_f": func() {
			mapComputeIfAbsent_f(tc_string, tc_string, m, "a", func(any) any { t.Error("the function ran"); return nil })
		},
	} {
		if !threwNPE(f) {
			t.Errorf("%s of a null map should throw NullPointerException", name)
		}
	}
	// The key is evaluated before the map is found null.
	evaluated := false
	if !threwNPE(func() { mapAt_(m, func() string { evaluated = true; return "a" }()) }) || !evaluated {
		t.Error("mapAt_ should evaluate the key, then throw")
	}
	if mapNN_(map[string]string{"a": "b"})["a"] != "b" {
		t.Error("mapNN_ should return the map")
	}
}
