package closure

// The parts of the handwritten runtime the cases use.

func obj_[T comparable](x T) any {
	var zero T
	if x == zero {
		return nil
	}
	return x
}

func cast_[T any](v any) T {
	if v == nil {
		var zero T
		return zero
	}
	return v.(T)
}

func String_format(format string, args ...any) string { return format }

type tconv_[T any] struct {
	box   func(T) any
	unbox func(any) T
}

type TList[T any] struct {
	items []T
	c     *tconv_[T]
}

func (l *TList[T]) get(i int32) T { return l.items[i] }

type ImmutableList_Builder[T any] struct {
	items []T
	c     *tconv_[T]
}

func immutableListBuilder_[T any](c *tconv_[T]) *ImmutableList_Builder[T] {
	return &ImmutableList_Builder[T]{c: c}
}

func (b *ImmutableList_Builder[T]) add(x T) *ImmutableList_Builder[T] {
	if b.c.box(x) == nil {
		panic("null")
	}
	b.items = append(b.items, x)
	return b
}

func (b *ImmutableList_Builder[T]) build() *TList[T] {
	return &TList[T]{items: b.items, c: b.c}
}
