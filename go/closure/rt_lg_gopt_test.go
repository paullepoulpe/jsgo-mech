//go:build !j2go_nofntypes

package closure

import "testing"

// Kind `goptional` (rt_lg_gopt.go): GOptional does what Optional_base does.
func TestLgGOptional(t *testing.T) {
	a, none := GOptional_of[string]("a"), GOptional_absent[string]()
	if !a.isPresent() || none.isPresent() || a.get() != "a" || a.or("z") != "a" || none.or("z") != "z" {
		t.Fatal("of / absent / get / or")
	}
	if none.orNull() != nullstr_ || GOptional_fromNullable[string](nullstr_).isPresent() || !GOptional_fromNullable[string]("x").isPresent() {
		t.Fatal("orNull / fromNullable")
	}
	var null GOptional[string]
	if !null.jnull_() || a.jnull_() || null.box_() != nil {
		t.Fatal("null")
	}
	if a.toString() != Optional_base_of("a").toString() || none.toString() != Optional_base_absent().toString() || a.hashCode() != Optional_base_of("a").hashCode() || none.hashCode() != Optional_base_absent().hashCode() {
		t.Fatal("toString / hashCode")
	}
	b := optGOptionalAs_[any](GOptional_of[string]("b"))
	if none.or_Optional(b).get() != "b" || a.or_Optional(b).get() != "a" {
		t.Fatal("or_Optional")
	}
	if !a.equals(GOptional_of[any]("a").box_()) || a.equals(none.box_()) || !none.equals(GOptional_absent[int32]().box_()) || a.equals(Optional_of[string]("a").box_()) || a.equals(nil) {
		t.Fatal("equals")
	}
	n := a.transform(func(s string) any { return int32(len(s)) })
	if n.get() != int32(1) || none.transform(func(s string) any { return s }).isPresent() {
		t.Fatal("transform")
	}
	if o := optGOptionalOf_[string](a.box_()); o.get() != "a" || !optGOptionalOf_[string](nil).jnull_() || !isGOptional_(a.box_()) || isGOptional_(Optional_of[string]("a").box_()) {
		t.Fatal("optGOptionalOf_ / isGOptional_")
	}
	panics := func(f func()) (p bool) {
		defer func() { p = recover() != nil }()
		f()
		return
	}
	if !panics(func() { none.get() }) || !panics(func() { GOptional_of[string](nullstr_) }) || !panics(func() { null.isPresent() }) || !panics(func() { none.or(nullstr_) }) {
		t.Fatal("throws")
	}
}
