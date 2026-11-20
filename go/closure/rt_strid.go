package closure

import "unsafe"

// Lookups and comparisons by the id of a node's string (post-flip runtime, tools/postflip/strids).
//
// A Node record holds its string as an id into the table of interned strings (rt_ids.go);
// getString() reads the table, and the caller then compares or hashes the bytes. Equal
// strings have one id, so where only equality matters the id is enough:
//
//	n.getString() == "prototype"          n.getStringIs_(sid_prototype)
//	a.getString() == b.getString()        a.getStringId_() == b.getStringId_()
//	scope.getVar(n.getString())           scope.getVarById_(n.getStringId_())
//
// getStringId_ throws what getString() throws for a node that holds no string.

// getStringId_ is the string id of getString().
func (this Node) getStringId_() uint32 {
	c := Node_payload_c_chunks_[this>>handleShift_][uint16(this)]
	if c&1 == 0 {
		noString_(this)
	}
	return c >> 1
}

// noString_ throws what getString() throws for a node that holds no string.
//
//go:noinline
func noString_(n Node) {
	n.getString()
}

// getStringIs_ is getString().equals(s) for the string s with id `id` (a sid_ variable).
func (this Node) getStringIs_(id uint32) bool {
	return this.getStringId_() == id
}

var (
	implicitVarArguments_ = strid_("arguments")
	implicitVarSuper_     = strid_("super")
	implicitVarThis_      = strid_("this")
	implicitVarExports_   = strid_("exports")
)

// AbstractScope_ImplicitVar_ofId_ is ImplicitVar.of(name) for the name with string id `id`.
func AbstractScope_ImplicitVar_ofId_(id uint32) AbstractScope_ImplicitVar {
	switch id {
	case implicitVarArguments_:
		return AbstractScope_ImplicitVar_ARGUMENTS
	case implicitVarSuper_:
		return AbstractScope_ImplicitVar_SUPER
	case implicitVarThis_:
		return AbstractScope_ImplicitVar_THIS
	case implicitVarExports_:
		return AbstractScope_ImplicitVar_EXPORTS
	}
	return 0
}

// nameId_ is the string id of name, the name a declaration at node n declares: the id n holds
// when n's string is name (a NAME node declares its string), else name's id from the table,
// interning it.
func nameId_(name string, n Node) uint32 {
	if n != 0 {
		if c := *n.payload_c_(); c&1 != 0 {
			if s := strid_str_(c >> 1); len(s) == len(name) && (unsafe.StringData(s) == unsafe.StringData(name) || s == name) {
				return c >> 1
			}
		}
	}
	return strid_(name)
}
