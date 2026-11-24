package closure

import "testing"

// TestGenericNullOfIntegerEnum: jisnull_ of an integer enum (post-flip step enums) is its zero, so the
// generic library twins (rt_lgfn.go) see a null enum as the erased code saw a null enum object.
func TestGenericNullOfIntegerEnum(t *testing.T) {
	var null AccessorSummary_PropertyAccessKind
	k := AccessorSummary_PropertyAccessKind(1)
	if !jisnull_(null) || jisnull_(k) {
		t.Fatalf("jisnull_ of an integer enum: zero %v, constant %v", jisnull_(null), jisnull_(k))
	}
	if MoreObjects_firstNonNullT_(null, k) != k || MoreObjects_firstNonNullT_(k, null) != k {
		t.Fatalf("firstNonNull of an integer enum")
	}
	if jisnull_(int32(0)) || jisnull_(uint8(0)) {
		t.Fatalf("jisnull_ of a plain integer")
	}
}
