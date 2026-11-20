package closure

// RtRoot is a root of the handwritten runtime: -rest does not look at it.
type RtRoot struct {
	self_ any
}

func (this *RtRoot) self() any { return this.self_ }
