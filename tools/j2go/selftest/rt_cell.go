package closure

// The runtime side of lang.Handles.Cell's runtime field (selftest/handles.txt: "Cell::extra runtime
// Handles_Cell_extra_" and "Cell::getExtra runtime"): the record holds a value code (rt_ids.go).

type Handles_Cell_extra_ struct{ c uint32 }

func (this Handles_Cell) extra_get_() any           { return valcode_get_(this.extra_p_().c) }
func (this Handles_Cell) extra_set_(v any)          { this.extra_p_().c = valcode_(v) }
func (this Handles_Cell) extra_isnull_() bool       { return this.extra_p_().c == 0 }
func (this Handles_Cell) extra_copy_(o Handles_Cell) { *this.extra_p_() = *o.extra_p_() }

func (this Handles_Cell) getExtra() any { return this.extra_get_() }
