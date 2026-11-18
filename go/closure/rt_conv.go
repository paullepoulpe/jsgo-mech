package closure

import (
	"math"
	"reflect"
	"strings"
)

// The runtime side of the translator's conversions that need no box (tools/j2go, Conv.java).

// ---- Typed non-null checks: <T> T checkNotNull(T, ...) of Preconditions and <T> T
// requireNonNull(T, ...) of Objects at the argument's own type. _T is for a pointer, an
// interface or a handle (null is the zero value of the type), _Str for a String (null is the
// null string). They throw what the functions taking an object throw.

func Preconditions_checkNotNull_T[T comparable](v T) T {
	var null T
	if v == null {
		panic(new_NullPointerException_0().self())
	}
	return v
}

func Preconditions_checkNotNull_Str(v string) string {
	if v == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return v
}

func Preconditions_checkNotNull_Object_Object_T[T comparable](v T, msg any) T {
	var null T
	if v == null {
		nullFail_(objstr_(canon_(msg)))
	}
	return v
}

func Preconditions_checkNotNull_Object_Object_Str(v string, msg any) string {
	if v == nullstr_ {
		nullFail_(objstr_(canon_(msg)))
	}
	return v
}

func Preconditions_checkNotNull_Object_String_Object_T[T comparable](v T, t string, p1 any) T {
	var null T
	if v == null {
		nullFail_(Strings_lenientFormat(t, []any{p1}))
	}
	return v
}

func Preconditions_checkNotNull_Object_String_Object_Str(v string, t string, p1 any) string {
	if v == nullstr_ {
		nullFail_(Strings_lenientFormat(t, []any{p1}))
	}
	return v
}

func Preconditions_checkNotNull_Object_String_Object_Object_T[T comparable](v T, t string, p1 any, p2 any) T {
	var null T
	if v == null {
		nullFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
	return v
}

func Preconditions_checkNotNull_Object_String_Object_Object_Str(v string, t string, p1 any, p2 any) string {
	if v == nullstr_ {
		nullFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
	return v
}

func Objects_requireNonNull_T[T comparable](v T, msg string) T {
	var null T
	if v == null {
		panic(new_NullPointerException_String(msg).self())
	}
	return v
}

func Objects_requireNonNull_Str(v string, msg string) string {
	if v == nullstr_ {
		panic(new_NullPointerException_String(msg).self())
	}
	return v
}

func Objects_requireNonNull_Object_T[T comparable](v T) T {
	var null T
	if v == null {
		panic(new_NullPointerException_0().self())
	}
	return v
}

func Objects_requireNonNull_Object_Str(v string) string {
	if v == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return v
}

// ---- Message arguments of checks. The translated code passes an Object argument that
// Preconditions only formats (and only when the check fails) at its own type: a string or a
// pointer to a class. canon_ makes it the object that Java sees: null for the null string and
// for a nil pointer, the most derived object for a pointer to a class with subclasses. It is
// the identity on a value that already is such an object.
func canon_(v any) any {
	switch x := v.(type) {
	case nil:
		return nil
	case string:
		return strobj_(x)
	case interface{ self() any }:
		if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer && rv.IsNil() {
			return nil
		}
		return x.self()
	}
	if rv := reflect.ValueOf(v); rv.Kind() == reflect.Pointer && rv.IsNil() {
		return nil
	}
	return v
}

// handle_ is a handle class (a Node): its zero value is null, box_ gives the object.
type handle_ interface {
	comparable
	box_() any
}

// A handle as the one message argument of a check is boxed only when the check fails.

func Preconditions_checkState_boolean_Object_H[M handle_](b bool, msg M) {
	if !b {
		stateFail_(objstr_(msg.box_()))
	}
}

func Preconditions_checkArgument_boolean_Object_H[M handle_](b bool, msg M) {
	if !b {
		argFail_(objstr_(msg.box_()))
	}
}

func Preconditions_checkState_boolean_String_Object_H[M handle_](b bool, t string, p1 M) {
	if !b {
		stateFail_(Strings_lenientFormat(t, []any{p1.box_()}))
	}
}

func Preconditions_checkArgument_boolean_String_Object_H[M handle_](b bool, t string, p1 M) {
	if !b {
		argFail_(Strings_lenientFormat(t, []any{p1.box_()}))
	}
}

// ---- Typed Object methods: Objects.equals, Objects.hashCode and String.valueOf of values
// held at their static type (the members that the translator generates for a record, and string
// concatenation). equals_, hash_ and tostr_ are for a pointer to a class without subclasses or
// an interface, the H variants for a handle; null is the zero value.

func equals_[T comparable](a, b T) bool { return objequals_(obj_(a), obj_(b)) }

func hash_[T comparable](v T) int32 { return objhash_(obj_(v)) }

func tostr_[T comparable](v T) string { return objstr_(obj_(v)) }

func equalsH_[T handle_](a, b T) bool { return objequals_(a.box_(), b.box_()) }

func hashH_[T handle_](v T) int32 { return objhash_(v.box_()) }

func tostrH_[T handle_](v T) string { return objstr_(v.box_()) }

// strhash_ is Objects.hashCode of a String: 0 for null.
func strhash_(s string) int32 {
	if s == nullstr_ {
		return 0
	}
	return String_hashCode(s)
}

// boolhash_, longhash_ and dblhash_ are Boolean.hashCode, Long.hashCode and Double.hashCode
// of a value (what Object_hashCode gives for the boxed value).

func boolhash_(b bool) int32 {
	if b {
		return 1231
	}
	return 1237
}

func longhash_(v int64) int32 { return int32(v ^ int64(uint64(v)>>32)) }

func dblhash_(v float64) int32 {
	b := math.Float64bits(v)
	return int32(b ^ (b >> 32))
}

// ---- String for CharSequence: the variants (suffix _s) of library functions and methods that
// declare a CharSequence, for a call that passes a String. They take the Go string, so the
// translated code does not box it (a string in an interface is an allocation). Each does what
// the function taking the sequence does with a string: csStr_ is csString_ for a String (null
// is the NullPointerException that calling a method of the null sequence throws). The variants of
// Pattern.matcher, Matcher.reset, CharMatcher.matchesAllOf and CharMatcher.anyOf are next to
// the functions they vary.

func csStr_(s string) string {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return s
}

func String_contains_s(s string, sub string) bool { return strings.Contains(s, csStr_(sub)) }

func String_replace_s(s string, a string, b string) string {
	return strings.ReplaceAll(s, csStr_(a), csStr_(b))
}

func String_join_s(sep string, it Iterable) string {
	parts := joinParts_(it)
	return strings.Join(parts, csStr_(sep))
}

func Pattern_matches_s(regex string, input string) bool {
	return Pattern_compile(regex).matcher_s(input).matches()
}

func (this *Splitter) split_s(seq string) Iterable { return listOf_(this.splitAll_(csStr_(seq))) }

func (this *Splitter) splitToList_s(seq string) List { return listOf_(this.splitAll_(csStr_(seq))) }

// Appendable_append_s is Appendable.append(CharSequence) for a String: a StringBuilder appends
// the string (null appends "null"); any other Appendable (a class of the program) gets the
// sequence as an object.
func Appendable_append_s(a Appendable, s string) Appendable {
	if sb, ok := a.(*StringBuilder); ok {
		return sb.append(s)
	}
	return a.append_CharSequence(strobj_(s))
}

// Appendable_append_s_int_int is Appendable.append(CharSequence, int, int) for a String.
func Appendable_append_s_int_int(a Appendable, s string, start int32, end int32) Appendable {
	if sb, ok := a.(*StringBuilder); ok {
		return sb.append(String_substring(csStr_(s), start, end))
	}
	return a.append_CharSequence_int_int(strobj_(s), start, end)
}
