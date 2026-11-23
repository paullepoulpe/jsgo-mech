package closure

// Gen: a generic root that dispatches through vt_Gen_; GenSub extends Gen[string].

type Gen[T comparable] struct {
	self_   any
	vt_Gen_ Gen_vt_[T]
	val     T
}

type Gen_vt_[T comparable] interface {
	get_impl() T
}

func (this *Gen[T]) isa_Gen_() {}

func (this *Gen[T]) as_Gen() *Gen[T] { return this }

func (this *Gen[T]) as_Gen_() any { return this }

func to_Gen[T comparable](v any) *Gen[T] {
	if v == nil {
		return nil
	}
	return v.(interface{ as_Gen_() any }).as_Gen_().(*Gen[T])
}

func (this *Gen[T]) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func new_Gen[T comparable](val T) *Gen[T] {
	this := &Gen[T]{}
	this.self_ = this
	this.vt_Gen_ = this
	this.val = val
	return this
}

func (this *Gen[T]) get_impl() T { return this.val }

func (this *Gen[T]) get() T {
	return this.vt_Gen_.get_impl()
}

type GenSub struct {
	Gen[string]
	vt_GenSub_ GenSub_vt_
}

type GenSub_vt_ interface {
	more_impl() int
}

func new_GenSub() *GenSub {
	this := &GenSub{}
	this.self_ = this
	this.vt_GenSub_ = this
	this.vt_Gen_ = this
	return this
}

func (this *GenSub) get_impl() string { return "sub" }

func (this *GenSub) more_impl() int { return 1 }

// get: an object that is exactly a GenSub calls its get_impl directly; others dispatch.
func (this *GenSub) get() string {
	if p, _ := this.self_.(*GenSub); p == this {
		return this.get_impl()
	}
	return this.vt_Gen_.get_impl()
}

// Ref: a generic root without a vt field.

type Ref[T comparable] struct {
	self_ any
	sym   T
}

func (this *Ref[T]) isa_Ref_() {}

func (this *Ref[T]) as_Ref() *Ref[T] { return this }

func (this *Ref[T]) as_Ref_() any { return this }

func to_Ref[T comparable](v any) *Ref[T] {
	if v == nil {
		return nil
	}
	return v.(interface{ as_Ref_() any }).as_Ref_().(*Ref[T])
}

func (this *Ref[T]) self() any { return this.self_ }

func new_Ref[T comparable](sym T) *Ref[T] {
	this := &Ref[T]{}
	this.self_ = this
	this.sym = sym
	return this
}

type IntRef struct {
	Ref[int]
}

func new_IntRef() *IntRef {
	this := &IntRef{}
	this.self_ = this
	return this
}

// Tree: a root whose subclasses are reached by this.self().(*C); Tree_as is declared.

type Tree struct {
	self_ any
	kind  int
}

// Tree_as is implemented by Tree and the classes that extend it: a cast to Tree asserts to it
// (post-flip).
type Tree_as interface {
	as_Tree() *Tree
}

func (this *Tree) as_Tree() *Tree { return this }

func to_Tree(v any) *Tree {
	if v == nil {
		return nil
	}
	return v.(Tree_as).as_Tree()
}

func (this *Tree) self() any { return this.self_ }

type Leaf struct {
	Tree
}

func new_Leaf() *Leaf {
	this := &Leaf{}
	this.self_ = this
	return this
}

func (this *Tree) asLeaf() *Leaf { return this.self().(*Leaf) }

// Reporter: one implementation, called by a concrete assertion; no Reporter_as yet.

type Reporter struct {
	self_ any
	n     int
}

func (this *Reporter) as_Reporter() *Reporter { return this }

func (this *Reporter) self() any { return this.self_ }

func (this *Reporter) report(s string) {
	this.self_.(*Printer).report_impl(s)
}

func (this *Reporter) isSelf() bool {
	return this.self_ == any(this)
}

type Printer struct {
	Reporter
}

func new_Printer() *Printer {
	this := &Printer{}
	this.self_ = this
	return this
}

func (this *Printer) report_impl(s string) { this.n++ }

// Copied: self_ is copied into a variable; left alone.

type Copied struct {
	self_ any
}

func (this *Copied) as_Copied() *Copied { return this }

func new_Copied() *Copied {
	this := &Copied{}
	this.self_ = this
	return this
}

func (this *Copied) same(o *Copied) bool {
	s := this.self_
	return s == o.self_
}

// Unpaired: a constructor sets self_ and not vt_Unpaired_; left alone.

type Unpaired[T comparable] struct {
	self_        any
	vt_Unpaired_ Unpaired_vt_[T]
}

type Unpaired_vt_[T comparable] interface {
	m_impl() T
}

func (this *Unpaired[T]) as_Unpaired() *Unpaired[T] { return this }

func (this *Unpaired[T]) m_impl() (z T) { return }

func (this *Unpaired[T]) m() T { return this.vt_Unpaired_.m_impl() }

func new_Unpaired[T comparable]() *Unpaired[T] {
	this := &Unpaired[T]{}
	this.self_ = this
	this.vt_Unpaired_ = this
	return this
}

func new_Unpaired_bare[T comparable]() *Unpaired[T] {
	this := &Unpaired[T]{}
	this.self_ = this
	return this
}

// Foreign: self_ is set to another class's object; left alone.

type Foreign struct {
	self_ any
}

func (this *Foreign) as_Foreign() *Foreign { return this }

func new_Foreign(l *Leaf) *Foreign {
	this := &Foreign{}
	this.self_ = l
	return this
}

// Listed: an interface of the package lists as_Listed_; the cast keeps its two assertions
// (the root is typed all the same).

type Listed[T comparable] struct {
	self_ any
}

func (this *Listed[T]) as_Listed() *Listed[T] { return this }

func (this *Listed[T]) as_Listed_() any { return this }

func to_Listed[T comparable](v any) *Listed[T] {
	if v == nil {
		return nil
	}
	return v.(interface{ as_Listed_() any }).as_Listed_().(*Listed[T])
}

type anyListed interface {
	as_Listed_() any
}

func isListed(v any) bool {
	_, ok := v.(anyListed)
	return ok
}

// Fmt: an enum; the anonymous interface of its cast gets a name.

type Fmt int

func (this Fmt) as_Fmt() Fmt { return this }

func to_Fmt(v any) Fmt {
	if v == nil {
		return 0
	}
	return v.(interface{ as_Fmt() Fmt }).as_Fmt()
}
