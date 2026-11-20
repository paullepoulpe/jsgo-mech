package closure

// Comparisons by the id of a node's string (post-flip runtime, tools/postflip/strids).
//
// A Node record holds its string as an id into the table of interned strings (rt_ids.go);
// getString() reads the table, and the caller then compares the bytes. Equal strings have one
// id, so where only equality matters the id is enough:
//
//	n.getString() == "prototype"          n.getStringIs_(sid_prototype)
//	a.getString() == b.getString()        a.getStringId_() == b.getStringId_()
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
