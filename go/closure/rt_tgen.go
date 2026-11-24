package closure

// Typed forms of Java generic methods the translation erased (post-flip step tgeneric): a
// <T> T method of the library or the program called on typed values, whose erased result the
// caller cast back. Each returns what the erased call returned, unboxed, and fails where it
// failed (the erased call is made for every path but the common one, so the exceptions and
// their messages are the same).

// getOnlyElementT_ is Iterables.getOnlyElement(l): its item when it has exactly one, else the
// erased call on the list's view, which throws NoSuchElementException or
// IllegalArgumentException with Guava's message.
func (l *TList[T]) getOnlyElementT_() T {
	if l != nil && len(l.items) == 1 {
		return l.items[0]
	}
	r := Iterables_getOnlyElement(Iterable(l.asList()))
	return l.c.unbox(r)
}

// getFirstT_ is Iterables.getFirst(l, def): its first item, or def (unboxed as the caller
// unboxed the result).
func (l *TList[T]) getFirstT_(def any) T {
	if len(l.items) > 0 {
		return l.items[0]
	}
	return l.c.unbox(def)
}

// getLastT_ is Iterables.getLast(l): its last item; an empty list throws
// NoSuchElementException as before.
func (l *TList[T]) getLastT_() T {
	if l != nil && len(l.items) > 0 {
		return l.items[len(l.items)-1]
	}
	r := Iterables_getLast(Iterable(l.asList()))
	return l.c.unbox(r)
}

// getOnlyElementT_ is Iterables.getOnlyElement(s) of a typed set: its element when it has
// exactly one (the first live entry), else the erased call on its view.
func (s *TSet[K]) getOnlyElementT_() K {
	if s != nil && s.live == 1 {
		return s.keys[s.first()]
	}
	r := Iterables_getOnlyElement(Iterable(s.asSet()))
	return s.kc.unbox(r)
}

// sliceGetOnlyElementT_, sliceGetFirstT_ and sliceGetLastT_ are the list forms for a native
// slice seen through sliceView_(c, s) (a nil slice is the null list: the erased call throws).
func sliceGetOnlyElementT_[T any](c *tconv_[T], s []T) T {
	if len(s) == 1 {
		return s[0]
	}
	return c.unbox(Iterables_getOnlyElement(sliceView_(c, s)))
}

func sliceGetFirstT_[T any](c *tconv_[T], s []T, def any) T {
	if len(s) > 0 {
		return s[0]
	}
	return c.unbox(Iterables_getFirst(sliceView_(c, s), def))
}

func sliceGetLastT_[T any](c *tconv_[T], s []T) T {
	if len(s) > 0 {
		return s[len(s)-1]
	}
	return c.unbox(Iterables_getLast(sliceView_(c, s)))
}

// Compiler_runInCompilerThreadT_ is Compiler.runInCompilerThread(Callable<T>) of a typed callable
// (Java's <T> T method): the executor runs the same call in the compiler thread; its result
// is taken from the call itself instead of from the box the erased callable returned.
func Compiler_runInCompilerThreadT_[T any](this *Compiler, callable func() T) T {
	var r T
	this.runInCompilerThread(func() any {
		r = callable()
		return nil
	})
	return r
}
