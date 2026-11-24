package closure

// varKey_ is what Var.equals compares (ScopedName: the name and the scope root), the key of a
// Go map keyed by a Var.
type varKey_ struct {
	name string
	root Node
}

// vkey_ is v's key in a Go map keyed by what Var.equals compares (tools/postflip/native): the
// name and the scope root. A null Var has a key of its own: no node handle reaches the
// largest uint32.
func vkey_(v *Var) varKey_ {
	if v == nil {
		return varKey_{root: Node(^uint32(0))}
	}
	return varKey_{v.getName(), v.getScopeRoot()}
}

// exportTraceKey_ is what ExportTrace.equals compares (an AutoValue of the module, compared by
// identity, and the export name), for a Go map keyed by it; null has a key of its own.
type exportTraceKey_ struct {
	module *UnresolvedModule
	name   string
	null   bool
}

func etkey_(t *ExportTrace) exportTraceKey_ {
	if t == nil {
		return exportTraceKey_{null: true}
	}
	return exportTraceKey_{t.module_, t.exportName_, false}
}
