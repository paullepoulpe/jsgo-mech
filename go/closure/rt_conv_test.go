package closure

import "testing"

func msgOf_(f func()) (s string) {
	defer func() {
		r := recover()
		s = to_Throwable(r).getMessage()
	}()
	f()
	return
}

func TestConvCanon(t *testing.T) {
	var jt *JSType
	var il *ImmutableList
	var n Node
	for i, v := range []any{jt, il, nullstr_, n.box_(), nil} {
		if canon_(v) != nil {
			t.Errorf("case %d: canon_ is not nil", i)
		}
	}
	if canon_("x") != any("x") {
		t.Errorf("string")
	}
	l := ImmutableList_of()
	if canon_(l) != any(l) {
		t.Errorf("pointer")
	}
}

func TestConvMessages(t *testing.T) {
	var jt *JSType
	var n Node
	l := ImmutableList_of()
	cases := []struct{ got, want string }{
		{msgOf_(func() { Preconditions_checkState_boolean_Object(false, jt) }), msgOf_(func() { Preconditions_checkState_boolean_Object(false, obj_(jt)) })},
		{msgOf_(func() { Preconditions_checkState_boolean_Object(false, nullstr_) }), msgOf_(func() { Preconditions_checkState_boolean_Object(false, strobj_(nullstr_)) })},
		{msgOf_(func() { Preconditions_checkState_boolean_Object(false, "m") }), "m"},
		{msgOf_(func() { Preconditions_checkState_boolean_String_Object(false, "a %s b", jt) }), "a null b"},
		{msgOf_(func() { Preconditions_checkArgument_boolean_String_Object_Object(false, "%s %s", nullstr_, l) }), msgOf_(func() { Preconditions_checkArgument_boolean_String_Object_Object(false, "%s %s", nil, obj_(l)) })},
		{msgOf_(func() { Preconditions_checkState_boolean_Object_H(false, n) }), msgOf_(func() { Preconditions_checkState_boolean_Object(false, n.box_()) })},
		{msgOf_(func() { Preconditions_checkArgument_boolean_String_Object_H(false, "n=%s", n) }), "n=null"},
		{msgOf_(func() { Preconditions_checkNotNull_T(jt) }), msgOf_(func() { Preconditions_checkNotNull(obj_(jt)) })},
		{msgOf_(func() { Preconditions_checkNotNull_Str(nullstr_) }), msgOf_(func() { Preconditions_checkNotNull(strobj_(nullstr_)) })},
		{msgOf_(func() { Preconditions_checkNotNull_Object_Object_T(n, "gone") }), "gone"},
	}
	for i, c := range cases {
		if c.got != c.want {
			t.Errorf("case %d: %q, want %q", i, c.got, c.want)
		}
	}
	if Preconditions_checkNotNull_T(l) != l || Preconditions_checkNotNull_Str("") != "" {
		t.Errorf("identity")
	}
}

// npe_ reports whether f throws NullPointerException.
func npe_(f func()) (npe bool) {
	defer func() {
		if r := recover(); r != nil {
			_, npe = r.(*NullPointerException)
		}
	}()
	f()
	return
}

// Rule "typed Object calls": the typed functions do what Object's methods do with the object
// that the erased code passed, a null receiver included.
func TestConvObjectCalls(t *testing.T) {
	l, m := ImmutableList_of_Object("a"), ImmutableList_of_Object("a")
	nl := l // a null of the lists' type, without naming it: a post-flip step renames the type
	nl = nil
	var ni List
	var nn Node
	if !equalsRR_(l, m) || !equalsR_(l, obj_(m)) || equalsR_(l, nil) || equalsRR_(l, nl) || equalsR_(l, "a") {
		t.Errorf("equalsR_ of two equal lists")
	}
	if equalsRR_(List(l), List(m)) != Object_equals(any(List(l)), any(List(m))) || equalsRR_(List(l), ni) {
		t.Errorf("equalsRR_ of interfaces")
	}
	if hashR_(l) != Object_hashCode(obj_(l)) || hashR_(List(l)) != hashR_(m) || tostrR_(l) != Object_toString(obj_(l)) || tostrR_(List(l)) != "[a]" {
		t.Errorf("hashR_ / tostrR_")
	}
	if !eqR_(l, l) || eqR_(l, m) || eqR_(l, nl) {
		t.Errorf("eqR_ is identity")
	}
	for i, f := range []func(){
		func() { equalsR_(nl, obj_(l)) }, func() { equalsRR_(nl, nl) }, func() { equalsRR_(ni, List(l)) }, func() { hashR_(nl) }, func() { tostrR_(ni) }, func() { eqR_(nl, nl) },
		func() { equalsRH_(nn, nil) }, func() { equalsRRH_(nn, nn) }, func() { hashRH_(nn) }, func() { tostrRH_(nn) },
	} {
		if !npe_(f) {
			t.Errorf("case %d: a null receiver does not throw NullPointerException", i)
		}
	}
}
