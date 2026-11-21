package closure

import "testing"

// TestTconvT checks the type-parameter conversions against the erased protocol: keys compared
// by == where Java's equals is identity, by hashCode and equals otherwise.
func TestTconvT(t *testing.T) {
	if c := tconvT_[string](); c.hash != nil || c.eq != nil || c.null != nullstr_ {
		t.Fatalf("string: compared by ==, null is nullstr_")
	}
	if c := tconvT_[Node](); c.hash != nil || c.eq != nil {
		t.Fatalf("Node: compared by ==")
	}
	if c := tconvT_[*GuardedCallback_Context](); c.hash != nil || c.eq != nil {
		t.Fatalf("a class without equals: compared by ==")
	}
	if c := tconvT_[*JSType](); c.hash == nil || c.eq == nil {
		t.Fatalf("a class with equals: hashCode and equals")
	}
	if c := tconvT_[any](); c.hash == nil || c.eq == nil {
		t.Fatalf("an interface: hashCode and equals")
	}
	if tconvT_[string]() != tconvT_[string]() {
		t.Fatalf("one conversion per type")
	}
	if c := tconvIdT_[*JSType](); c.hash != nil || c.eq != nil {
		t.Fatalf("identity of a pointer: ==")
	}
	if c := tconvIdT_[any](); c.hash == nil || c.eq == nil {
		t.Fatalf("identity of an interface value: its box")
	}
}
