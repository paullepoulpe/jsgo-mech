//go:build !j2go_nofntypes

package closure

import (
	"testing"
	"unsafe"
)

// A function as an Object: nil for null, one box per closure, unboxed only at its own type.
func TestFnBox(t *testing.T) {
	n := 0
	var f Supplier[any] = func() any { n++; return n }
	g := f
	var h Supplier[any] = func() any { return n }
	var null Supplier[any]
	if f.box_() == nil || null.box_() != nil {
		t.Fatal("box of a function is nil exactly for null")
	}
	if f.box_() != g.box_() || f.box_() == h.box_() {
		t.Fatal("two boxes are equal exactly when they box the same closure")
	}
	if !objequals_(f.box_(), g.box_()) || objequals_(f.box_(), h.box_()) {
		t.Fatal("objequals_ of boxes")
	}
	u := fnUnbox_[Supplier[any]](f.box_())
	if u() != 1 || u() != 2 || n != 2 {
		t.Fatal("the unboxed function is the function")
	}
	if fnUnbox_[Supplier[any]](nil) != nil {
		t.Fatal("unboxing null")
	}
	// a map of the erased runtime holds boxes as values and as keys
	m := new_LinkedHashMap()
	m.put(f.box_(), "f")
	m.put(h.box_(), "h")
	if m.get(g.box_()) != "f" || m.size() != 2 {
		t.Fatal("boxes as keys")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("unboxing at another type must fail")
			}
		}()
		fnUnbox_[Function[any, any]](f.box_())
	}()
}

func TestFnIdentity(t *testing.T) {
	a := Predicate[string](func(s string) bool { return s == "" })
	k := "k"
	b := Predicate[string](func(s string) bool { return s == k })
	c := b
	var null Predicate[string]
	if !fnSame_(a, a) || fnSame_(a, b) || !fnSame_(b, c) || !fnSame_(null, null) || fnSame_(a, null) {
		t.Fatal("fnSame_")
	}
	if fnHash_(b) != fnHash_(c) || fnHash_(null) != 0 {
		t.Fatal("fnHash_")
	}
	if fnStr_(null) != "null" || fnStr_(b) != fnStr_(c) || fnStr_(b) == "null" {
		t.Fatal("fnStr_")
	}
}

func TestFnPredicatesFunctionsSuppliers(t *testing.T) {
	if !Predicates_alwaysTrue[string]()("x") || !Predicates_alwaysTrue[Node]().test(0) {
		t.Fatal("alwaysTrue")
	}
	eq := Predicates_equalTo[string]("a")
	if !eq("a") || eq("b") || eq(nullstr_) {
		t.Fatal("equalTo")
	}
	isNull := Predicates_equalTo[string](nullstr_)
	if !isNull(nullstr_) || isNull("a") {
		t.Fatal("equalTo(null)")
	}
	if Functions_identity[int32]()(7) != 7 {
		t.Fatal("identity")
	}
	n := 0
	memo := Suppliers_memoize[string](func() string { n++; return "v" })
	if memo() != "v" || memo.get() != "v" || n != 1 {
		t.Fatal("memoize calls its supplier once")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("memoize(null) must throw")
			}
		}()
		Suppliers_memoize[string](nil)
	}()
}

func TestFnOptional(t *testing.T) {
	o := Optional_of[string]("abc")
	if !o.filter(func(s string) bool { return len(s) == 3 }).isPresent() || o.filter(func(s string) bool { return false }).isPresent() {
		t.Fatal("filter")
	}
	got := ""
	o.ifPresent(func(s string) { got = s })
	Optional_empty[string]().ifPresent(func(s string) { got = "no" })
	if got != "abc" {
		t.Fatal("ifPresent")
	}
	func() {
		defer func() {
			if recover() == nil {
				t.Fatal("filter(null) must throw")
			}
		}()
		o.filter(nil)
	}()
}

// A typed map whose values are functions: null values are told apart without comparing funcs.
func TestFnTypedMapOfFunctions(t *testing.T) {
	c := &tconv_[Supplier[string]]{
		box:    func(v Supplier[string]) any { return v.box_() },
		unbox:  func(v any) Supplier[string] { return fnUnbox_[Supplier[string]](v) },
		isnull: func(v Supplier[string]) bool { return v == nil },
	}
	m := newTMap_(tc_string, c)
	calls := 0
	mk := func(k string) Supplier[string] { calls++; return func() string { return k + "!" } }
	if m.ComputeIfAbsent("a", mk)() != "a!" || m.ComputeIfAbsent("a", mk)() != "a!" || calls != 1 {
		t.Fatal("computeIfAbsent makes a value once")
	}
	if m.ComputeIfAbsent("n", func(string) Supplier[string] { return nil }) != nil || m.Has("n") {
		t.Fatal("a null result is not stored")
	}
	if m.Get("zz") != nil || m.PutIfAbsent("b", mk("b")) != nil || m.PutIfAbsent("b", mk("c"))() != "b!" {
		t.Fatal("get, putIfAbsent")
	}
	if e := SourceCodeEscapers_javascriptEscaper().asFunction(); e("a'b") == "a'b" || e.apply("ab") != "ab" {
		t.Fatal("Escaper.asFunction")
	}
}

// fnBox_, fnSame_ and fnHash_ rest on a func value being one pointer, to its closure. If a
// toolchain represents func values otherwise, this fails before anything else does.
func TestFnValueIsOnePointer(t *testing.T) {
	var f Supplier[any]
	if unsafe.Sizeof(f) != unsafe.Sizeof(uintptr(0)) {
		t.Fatalf("a func value is %d bytes, not one pointer", unsafe.Sizeof(f))
	}
	if *(*unsafe.Pointer)(unsafe.Pointer(&f)) != nil {
		t.Fatal("a nil func value is not the nil pointer")
	}
	// a func value in an interface is stored as the pointer itself (pointer-shaped): the
	// interface's data word is the func value
	n := 0
	f = func() any { n++; return n }
	var x any = f
	type eface struct{ typ, data unsafe.Pointer }
	if (*eface)(unsafe.Pointer(&x)).data != *(*unsafe.Pointer)(unsafe.Pointer(&f)) {
		t.Fatal("a func value in an interface is not its pointer")
	}
	// the pointer is the closure: a func value rebuilt from it calls the same closure, and two
	// closures of one func literal have different pointers
	var g Supplier[any]
	*(*unsafe.Pointer)(unsafe.Pointer(&g)) = *(*unsafe.Pointer)(unsafe.Pointer(&f))
	if g() != 1 || f() != 2 || n != 2 {
		t.Fatal("a func value rebuilt from its pointer is not the same closure")
	}
	mk := func(k int) Supplier[any] { return func() any { return k } }
	if fnSame_(mk(1), mk(2)) {
		t.Fatal("two closures have one pointer")
	}
}
