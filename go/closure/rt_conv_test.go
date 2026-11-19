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
