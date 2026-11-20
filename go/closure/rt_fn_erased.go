//go:build j2go_nofntypes

package closure

// The runtime's functions that take or give a functional interface with its Java types, for the
// translation without rule "function types" (J2GO_NOFNTYPES=1; build with -tags j2go_nofntypes):
// the erased forms. rt_fn.go has the forms of the rule.

func (this Optional[T]) filter(p Predicate_e) Optional[T] {
	this.nn_()
	if p == nil {
		panic(new_NullPointerException_0().self())
	}
	if !this.present || p.test(this.content_()) {
		return this
	}
	return Optional[T]{set: true}
}

func (this Optional[T]) ifPresent(c Consumer_e) {
	this.nn_()
	if this.present {
		c.accept(this.content_())
	}
}

// ---- Predicates, Functions, Suppliers

type alwaysTruePredicate_ struct{}

func (alwaysTruePredicate_) apply(any) bool   { return true }
func (alwaysTruePredicate_) test(any) bool    { return true }
func (alwaysTruePredicate_) toString() string { return "Predicates.alwaysTrue()" }

var alwaysTrue_ Predicate_base_e = alwaysTruePredicate_{}

func Predicates_alwaysTrue() Predicate_base_e { return alwaysTrue_ }

type isNullPredicate_ struct{}

func (isNullPredicate_) apply(v any) bool { return v == nil }
func (isNullPredicate_) test(v any) bool  { return v == nil }
func (isNullPredicate_) toString() string { return "Predicates.isNull()" }

type equalToPredicate_ struct {
	target any
}

func (this *equalToPredicate_) apply(v any) bool { return Object_equals(this.target, v) }
func (this *equalToPredicate_) test(v any) bool  { return Object_equals(this.target, v) }
func (this *equalToPredicate_) toString() string {
	return "Predicates.equalTo(" + objstr_(this.target) + ")"
}
func (this *equalToPredicate_) hashCode() int32 { return objhash_(this.target) }
func (this *equalToPredicate_) equals(o any) bool {
	x, ok := o.(*equalToPredicate_)
	return ok && objequals_(this.target, x.target)
}

func Predicates_equalTo(target any) Predicate_base_e {
	if target == nil {
		return isNullPredicate_{}
	}
	return &equalToPredicate_{target}
}

type identityFunction_ struct{}

type memoizingSupplier_ struct {
	delegate Supplier_base_e
	done     bool
	value    any
}

func (this *memoizingSupplier_) get() any {
	if !this.done {
		v := this.delegate.get()
		this.value = v
		this.done = true
		this.delegate = nil
	}
	return this.value
}

func (this *memoizingSupplier_) toString() string {
	if this.done {
		return "Suppliers.memoize(<supplier that returned " + objstr_(this.value) + ">)"
	}
	return "Suppliers.memoize(" + objstr_(this.delegate) + ")"
}

func Suppliers_memoize(s Supplier_base_e) Supplier_base_e {
	if m, ok := s.(*memoizingSupplier_); ok {
		return m
	}
	if s == nil {
		panic(new_NullPointerException_0().self())
	}
	return &memoizingSupplier_{delegate: s}
}

func Functions_identity() Function_base_e { return identityFunction_{} }

func (identityFunction_) apply(v any) any { return v }

func (identityFunction_) toString() string { return "Functions.identity()" }

// computeIfAbsent returns the value of key k if it has one that is not null; else the value f
// makes for the key, which becomes the key's value unless it is null.
func (m *JTMap[K, V]) computeIfAbsent(k K, f Function_e) V {
	if m.imm != nil && m.imm != sortedMapMark_ {
		panic(frozen_{})
	}
	if i := m.find(k); i >= 0 && !m.isNull(m.vals[i]) {
		return m.vals[i]
	}
	v := m.vc.unbox(f.apply(m.kc.box(k)))
	if !m.isNull(v) {
		m.put(k, v)
	}
	return v
}

func (this *Escaper) asFunction() Function_base {
	return &Function_base_fn{func(v any) any { return strobj_(this.escape(str_(v))) }}
}

// ---- Comparator

func Comparator_naturalOrder() Comparator_e { return naturalOrder_ }

func Comparator_reversed(c Comparator_e) Comparator_e { return cmpReversed_(c) }

func Comparator_comparing(f Function_e) Comparator_e {
	return Comparator_of_(func(a, b any) int32 { return compareNatural_(f.apply(a), f.apply(b)) })
}

func Comparator_comparing_Function_Comparator(f Function_e, c Comparator_e) Comparator_e {
	return Comparator_of_(func(a, b any) int32 { return c.compare(f.apply(a), f.apply(b)) })
}

func Comparator_comparingInt(f ToIntFunction_e) Comparator_e {
	return Comparator_of_(func(a, b any) int32 { return Integer_compare(f.applyAsInt(a), f.applyAsInt(b)) })
}

func Comparator_comparingLong(f ToLongFunction_e) Comparator_e {
	return Comparator_of_(func(a, b any) int32 { return Long_compare(f.applyAsLong(a), f.applyAsLong(b)) })
}

func Comparator_thenComparing_Comparator(c Comparator_e, d Comparator_e) Comparator_e {
	return Comparator_of_(func(a, b any) int32 {
		if r := c.compare(a, b); r != 0 {
			return r
		}
		return d.compare(a, b)
	})
}

func Comparator_thenComparing(c Comparator_e, f Function_e) Comparator_e {
	return Comparator_thenComparing_Comparator(c, Comparator_comparing(f))
}

func Comparator_thenComparingInt(c Comparator_e, f ToIntFunction_e) Comparator_e {
	return Comparator_thenComparing_Comparator(c, Comparator_comparingInt(f))
}

func Map_Entry_comparingByKey() Comparator_e {
	return Comparator_of_(func(a, b any) int32 {
		return compareNatural_(a.(Map_Entry).getKey(), b.(Map_Entry).getKey())
	})
}

func Comparator_thenComparingLong(c Comparator_e, f ToLongFunction_e) Comparator_e {
	return Comparator_thenComparing_Comparator(c, Comparator_comparingLong(f))
}
