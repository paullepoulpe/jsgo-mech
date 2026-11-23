package closure

// Pos is a record held in fields: it becomes a value. Holder.last is compared with nil, so it
// stays a pointer and takes a copy.
type Pos struct {
	line int32
	col  int32
}

func (this *Pos) init_Pos(line int32, col int32) {
	this.line = line
	this.col = col
}

func new_Pos(line int32, col int32) *Pos {
	this := &Pos{}
	this.init_Pos(line, col)
	return this
}

func (this *Pos) getLine() int32 {
	return this.line
}

func (this *Pos) toString() string {
	return "pos"
}

// Span holds two Pos: it is measured with them as values (2 words of two int32 each).
type Span struct {
	start *Pos
	end   *Pos
}

func (this *Span) init_Span(start *Pos, end *Pos) {
	this.start = start
	this.end = end
}

func new_Span(start *Pos, end *Pos) *Span {
	this := &Span{}
	this.init_Span(start, end)
	return this
}

func (this *Span) toString() string {
	return String_format("<%s - %s>", obj_(this.start), obj_(this.end))
}

type Holder struct {
	at    *Pos
	span  *Span
	last  *Pos
	marks *TList[*Pos]
}

func (this *Holder) init_Holder(at *Pos, marks *TList[*Pos]) {
	this.at = at
	this.span = new_Span(at, new_Pos((at.line+1), 0))
	this.marks = marks
}

func new_Holder(at *Pos, marks *TList[*Pos]) *Holder {
	this := &Holder{}
	this.init_Holder(at, marks)
	return this
}

func (this *Holder) seen(p *Pos) bool {
	if this.last == nil {
		this.last = p
		return false
	}
	return (this.last.line == p.getLine())
}

func (this *Holder) mark(i int32) *Pos {
	var m *Pos = this.marks.get(i)
	return m
}

func (this *Holder) width() int32 {
	var s *Span = this.span
	var from *Pos = s.start
	var to *Pos = s.end
	return (to.col - from.col)
}

func describe(h *Holder) string {
	return String_format("%s", obj_(h.at))
}

func build(line int32) *Holder {
	var marks *ImmutableList_Builder[*Pos] = immutableListBuilder_(tc_p_Pos)
	var p *Pos = new_Pos(line, 0)
	marks.add(p)
	return new_Holder(p, marks.build())
}

// Ident is compared by identity: left.
type Ident struct {
	n int32
}

func (this *Ident) init_Ident(n int32) {
	this.n = n
}

func new_Ident(n int32) *Ident {
	this := &Ident{}
	this.init_Ident(n)
	return this
}

type IdentHolder struct {
	id *Ident
}

func (this *IdentHolder) init_IdentHolder(id *Ident) {
	this.id = id
}

func new_IdentHolder(id *Ident) *IdentHolder {
	this := &IdentHolder{}
	this.init_IdentHolder(id)
	return this
}

func (this *IdentHolder) is(other *Ident) bool {
	return (this.id == other)
}

// Boxed is stored as an Object: left.
type Boxed struct {
	n int32
}

func (this *Boxed) init_Boxed(n int32) {
	this.n = n
}

func new_Boxed(n int32) *Boxed {
	this := &Boxed{}
	this.init_Boxed(n)
	return this
}

type BoxedHolder struct {
	b   *Boxed
	obj any
}

func (this *BoxedHolder) init_BoxedHolder(b *Boxed) {
	this.b = b
	this.obj = obj_(b)
}

func new_BoxedHolder(b *Boxed) *BoxedHolder {
	this := &BoxedHolder{}
	this.init_BoxedHolder(b)
	return this
}

// Keyed is a key of a set: left.
type Keyed struct {
	n int32
}

func (this *Keyed) init_Keyed(n int32) {
	this.n = n
}

func new_Keyed(n int32) *Keyed {
	this := &Keyed{}
	this.init_Keyed(n)
	return this
}

type KeyedHolder struct {
	k    *Keyed
	seen map[*Keyed]bool
}

func (this *KeyedHolder) init_KeyedHolder(k *Keyed) {
	this.k = k
	this.seen = map[*Keyed]bool{}
	this.seen[k] = true
}

func new_KeyedHolder(k *Keyed) *KeyedHolder {
	this := &KeyedHolder{}
	this.init_KeyedHolder(k)
	return this
}

// Maybe is null in its only field (nil is passed to the constructor): no field would hold it by
// value, left.
type Maybe struct {
	n int32
}

func (this *Maybe) init_Maybe(n int32) {
	this.n = n
}

func new_Maybe(n int32) *Maybe {
	this := &Maybe{}
	this.init_Maybe(n)
	return this
}

type MaybeHolder struct {
	m *Maybe
}

func (this *MaybeHolder) init_MaybeHolder(m *Maybe) {
	this.m = m
}

func new_MaybeHolder(m *Maybe) *MaybeHolder {
	this := &MaybeHolder{}
	this.init_MaybeHolder(m)
	return this
}

func (this *MaybeHolder) value() int32 {
	if this.m == nil {
		return 0
	}
	return this.m.n
}

func noMaybe() *MaybeHolder {
	return new_MaybeHolder(nil)
}

func someMaybe() *MaybeHolder {
	return new_MaybeHolder(new_Maybe(1))
}

// Late is assigned by a setter after construction: the field stays a pointer (its zero value
// can be read), so nothing holds it by value, left.
type Late struct {
	n int32
}

func (this *Late) init_Late(n int32) {
	this.n = n
}

func new_Late(n int32) *Late {
	this := &Late{}
	this.init_Late(n)
	return this
}

type LateHolder struct {
	l *Late
}

func (this *LateHolder) init_LateHolder() {
}

func new_LateHolder() *LateHolder {
	this := &LateHolder{}
	this.init_LateHolder()
	return this
}

func (this *LateHolder) set(l *Late) {
	this.l = l
}

func (this *LateHolder) get() int32 {
	return this.l.n
}

// Counter is written after construction: not a candidate.
type Counter struct {
	n int32
}

func (this *Counter) init_Counter() {
	this.n = 0
}

func new_Counter() *Counter {
	this := &Counter{}
	this.init_Counter()
	return this
}

func (this *Counter) next() int32 {
	this.n++
	return this.n
}
