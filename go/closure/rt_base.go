package closure

// com.google.common.base: Preconditions, Strings, Ascii, MoreObjects, Optional_base,
// Predicates, Functions, Suppliers, Throwables, CaseFormat; java.util.Optional_base.

import (
	"strings"
)

type Ascii struct{}
type Chars struct{}
type Functions struct{}
type Hashing struct{}
type Math struct{}
type MoreObjects struct{}
type Objects struct{}
type Preconditions struct{}
type Predicates struct{}
type Strings struct{}
type Suppliers struct{}
type Throwables struct{}

// ---- Strings

// lenientToString_ is Guava's Strings.lenientToString: a toString that throws
// is reported inside the text instead of propagating.
func lenientToString_(v any) (s string) {
	v = canon_(v) // a message argument may come at its own type (rt_conv.go)
	if v == nil {
		return "null"
	}
	defer func() {
		if r := recover(); r != nil {
			ex := baseCatch_(r)
			name := "java.lang.RuntimeException"
			if ex != nil {
				name = Object_getClass(ex).getName()
			}
			s = "<" + Object_getClass(v).getName() + "@" + Integer_toHexString(System_identityHashCode(v)) + " threw " + name + ">"
		}
	}()
	return Object_toString(v)
}

// baseCatch_ is catch_, except that a Java exception is recognised before
// anything else (exception objects also implement Go's error interface).
func baseCatch_(r any) any {
	if _, ok := r.(interface{ as_Throwable() *Throwable }); ok {
		return r
	}
	return catch_(r)
}

func Strings_lenientFormat(template string, args []any) string {
	template = sstr_(template)
	strs := make([]string, 0, len(args))
	if args == nil {
		strs = append(strs, "(Object[])null")
	} else {
		for _, a := range args {
			strs = append(strs, lenientToString_(a))
		}
	}
	var sb strings.Builder
	sb.Grow(len(template) + 16*len(strs))
	start, i := 0, 0
	for i < len(strs) {
		p := strings.Index(template[start:], "%s")
		if p < 0 {
			break
		}
		sb.WriteString(template[start : start+p])
		sb.WriteString(strs[i])
		i++
		start += p + 2
	}
	sb.WriteString(template[start:])
	if i < len(strs) {
		sb.WriteString(" [")
		sb.WriteString(strs[i])
		for i++; i < len(strs); i++ {
			sb.WriteString(", ")
			sb.WriteString(strs[i])
		}
		sb.WriteByte(']')
	}
	return sb.String()
}

func Strings_isNullOrEmpty(s string) bool { return s == nullstr_ || s == "" }

func Strings_padStart(s string, minLength int32, padChar uint16) string {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	n := String_length(s)
	if n >= minLength {
		return s
	}
	return strings.Repeat(String_valueOf_char(padChar), int(minLength-n)) + s
}

// ---- Preconditions

func argFail_(msg string) {
	panic(new_IllegalArgumentException_String(msg).self())
}

func stateFail_(msg string) {
	panic(new_IllegalStateException_String(msg).self())
}

func nullFail_(msg string) {
	panic(new_NullPointerException_String(msg).self())
}

func Preconditions_checkArgument(b bool) {
	if !b {
		panic(new_IllegalArgumentException_0().self())
	}
}

func Preconditions_checkArgument_boolean_Object(b bool, msg any) {
	if !b {
		argFail_(objstr_(canon_(msg)))
	}
}

func Preconditions_checkArgument_boolean_String_Object(b bool, t string, p1 any) {
	if !b {
		argFail_(Strings_lenientFormat(t, []any{p1}))
	}
}

func Preconditions_checkArgument_boolean_String_Object_Object(b bool, t string, p1 any, p2 any) {
	if !b {
		argFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
}

func Preconditions_checkArgument_boolean_String_Object_Object_Object(b bool, t string, p1 any, p2 any, p3 any) {
	if !b {
		argFail_(Strings_lenientFormat(t, []any{p1, p2, p3}))
	}
}

func Preconditions_checkArgument_boolean_String_int(b bool, t string, p1 int32) {
	if !b {
		argFail_(Strings_lenientFormat(t, []any{p1}))
	}
}

func Preconditions_checkArgument_boolean_String_int_int(b bool, t string, p1 int32, p2 int32) {
	if !b {
		argFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
}

func Preconditions_checkNotNull(v any) any {
	if v == nil {
		panic(new_NullPointerException_0().self())
	}
	return v
}

func Preconditions_checkNotNull_Object_Object(v any, msg any) any {
	if v == nil {
		nullFail_(objstr_(canon_(msg)))
	}
	return v
}

func Preconditions_checkNotNull_Object_String_Object(v any, t string, p1 any) any {
	if v == nil {
		nullFail_(Strings_lenientFormat(t, []any{p1}))
	}
	return v
}

func Preconditions_checkNotNull_Object_String_Object_Object(v any, t string, p1 any, p2 any) any {
	if v == nil {
		nullFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
	return v
}

// Preconditions_checkNotNullT is Preconditions.checkNotNull for a value v of a pointer or
// interface type, which is Java null when it is nil: the same NullPointerException when v is
// nil, else v, with no boxing (post-flip: a cast of the erased checkNotNull's result back to the
// type of its argument is this call). Not for strings (null is nullstr_, not "").
func Preconditions_checkNotNullT[T comparable](v T) T {
	var zero T
	if v == zero {
		panic(new_NullPointerException_0().self())
	}
	return v
}

// Preconditions_checkNotNullT_Object is Preconditions_checkNotNull_Object_Object, typed.
func Preconditions_checkNotNullT_Object[T comparable](v T, msg any) T {
	var zero T
	if v == zero {
		nullFail_(objstr_(msg))
	}
	return v
}

// Preconditions_checkNotNullT_String_Object is Preconditions_checkNotNull_Object_String_Object,
// typed.
func Preconditions_checkNotNullT_String_Object[T comparable](v T, t string, p1 any) T {
	var zero T
	if v == zero {
		nullFail_(Strings_lenientFormat(t, []any{p1}))
	}
	return v
}

// Preconditions_checkNotNullT_String_Object_Object is
// Preconditions_checkNotNull_Object_String_Object_Object, typed.
func Preconditions_checkNotNullT_String_Object_Object[T comparable](v T, t string, p1 any, p2 any) T {
	var zero T
	if v == zero {
		nullFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
	return v
}

func Preconditions_checkState(b bool) {
	if !b {
		panic(new_IllegalStateException_0().self())
	}
}

func Preconditions_checkState_boolean_Object(b bool, msg any) {
	if !b {
		stateFail_(objstr_(canon_(msg)))
	}
}

func Preconditions_checkState_boolean_String_Object(b bool, t string, p1 any) {
	if !b {
		stateFail_(Strings_lenientFormat(t, []any{p1}))
	}
}

func Preconditions_checkState_boolean_String_ObjectArr(b bool, t string, args []any) {
	if !b {
		stateFail_(Strings_lenientFormat(t, args))
	}
}

func Preconditions_checkState_boolean_String_Object_Object(b bool, t string, p1 any, p2 any) {
	if !b {
		stateFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
}

func Preconditions_checkState_boolean_String_Object_Object_Object(b bool, t string, p1 any, p2 any, p3 any) {
	if !b {
		stateFail_(Strings_lenientFormat(t, []any{p1, p2, p3}))
	}
}

func Preconditions_checkState_boolean_String_Object_Object_Object_Object(b bool, t string, p1 any, p2 any, p3 any, p4 any) {
	if !b {
		stateFail_(Strings_lenientFormat(t, []any{p1, p2, p3, p4}))
	}
}

func Preconditions_checkState_boolean_String_Object_int(b bool, t string, p1 any, p2 int32) {
	if !b {
		stateFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
}

func Preconditions_checkState_boolean_String_int(b bool, t string, p1 int32) {
	if !b {
		stateFail_(Strings_lenientFormat(t, []any{p1}))
	}
}

func Preconditions_checkState_boolean_String_int_Object(b bool, t string, p1 int32, p2 any) {
	if !b {
		stateFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
}

func Preconditions_checkState_boolean_String_int_int(b bool, t string, p1 int32, p2 int32) {
	if !b {
		stateFail_(Strings_lenientFormat(t, []any{p1, p2}))
	}
}

// ---- Ascii

func asciiLower_(s string) string {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'A' && c <= 'Z' {
			b := []byte(s)
			for ; i < len(b); i++ {
				if c := b[i]; c >= 'A' && c <= 'Z' {
					b[i] = c + 32
				}
			}
			return string(b)
		}
	}
	return s
}

func asciiUpper_(s string) string {
	for i := 0; i < len(s); i++ {
		if c := s[i]; c >= 'a' && c <= 'z' {
			b := []byte(s)
			for ; i < len(b); i++ {
				if c := b[i]; c >= 'a' && c <= 'z' {
					b[i] = c - 32
				}
			}
			return string(b)
		}
	}
	return s
}

func Ascii_equalsIgnoreCase(a any, b any) bool {
	x, y := csString_(a), csString_(b)
	if len(x) != len(y) {
		return false
	}
	for i := 0; i < len(x); i++ {
		c, d := x[i], y[i]
		if c == d {
			continue
		}
		if c >= 'A' && c <= 'Z' {
			c += 32
		}
		if d >= 'A' && d <= 'Z' {
			d += 32
		}
		if c != d || c < 'a' || c > 'z' {
			return false
		}
	}
	return true
}

func Ascii_isUpperCase(c uint16) bool { return c >= 'A' && c <= 'Z' }

func Ascii_toLowerCase(s string) string {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return asciiLower_(s)
}

func Ascii_toLowerCase_char(c uint16) uint16 {
	if c >= 'A' && c <= 'Z' {
		return c + 32
	}
	return c
}

func Ascii_toUpperCase(s string) string {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return asciiUpper_(s)
}

func Ascii_toUpperCase_char(c uint16) uint16 {
	if c >= 'a' && c <= 'z' {
		return c - 32
	}
	return c
}

// ---- MoreObjects

func MoreObjects_firstNonNull(a any, b any) any {
	if a != nil {
		return a
	}
	if b != nil {
		return b
	}
	panic(new_NullPointerException_String("Both parameters are null").self())
}

type toStringHolder_ struct {
	name  string
	value any
}

type MoreObjects_ToStringHelper struct {
	className string
	values    []toStringHolder_
	omitNull  bool
}

func MoreObjects_toStringHelper(self any) *MoreObjects_ToStringHelper {
	return &MoreObjects_ToStringHelper{className: Object_getClass(self).getSimpleName()}
}

func MoreObjects_toStringHelper_Class(c *Class) *MoreObjects_ToStringHelper {
	return &MoreObjects_ToStringHelper{className: c.getSimpleName()}
}

func (this *MoreObjects_ToStringHelper) add(name string, v any) *MoreObjects_ToStringHelper {
	if name == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	this.values = append(this.values, toStringHolder_{name, v})
	return this
}

func (this *MoreObjects_ToStringHelper) add_String_boolean(name string, v bool) *MoreObjects_ToStringHelper {
	return this.add(name, v)
}

func (this *MoreObjects_ToStringHelper) add_String_int(name string, v int32) *MoreObjects_ToStringHelper {
	return this.add(name, v)
}

func (this *MoreObjects_ToStringHelper) omitNullValues() *MoreObjects_ToStringHelper {
	this.omitNull = true
	return this
}

// deepStr_ prints a helper value; arrays print like Arrays.deepToString.
func deepStr_(v any) string {
	switch x := v.(type) {
	case nil:
		return "null"
	case []any:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = deepStr_(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []string:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = sstr_(e)
		}
		return "[" + strings.Join(parts, ", ") + "]"
	case []int32:
		parts := make([]string, len(x))
		for i, e := range x {
			parts[i] = istr_(int64(e))
		}
		return "[" + strings.Join(parts, ", ") + "]"
	}
	return objstr_(v)
}

func (this *MoreObjects_ToStringHelper) toString() string {
	var sb strings.Builder
	sb.WriteString(this.className)
	sb.WriteByte('{')
	sep := ""
	for _, h := range this.values {
		if h.value == nil && this.omitNull {
			continue
		}
		sb.WriteString(sep)
		sep = ", "
		sb.WriteString(h.name)
		sb.WriteByte('=')
		sb.WriteString(deepStr_(h.value))
	}
	sb.WriteByte('}')
	return sb.String()
}

// ---- com.google.common.base.Optional_base

// Optional_base is Guava's com.google.common.base.Optional as a value (post-flip, as
// java.util.Optional in step 7): its zero value is Java's null (a field never assigned), so
// absent() and of(v) allocate nothing. Guava's failures are kept: of(null) throws
// NullPointerException, get() of an absent value IllegalStateException, or(null) and the
// null results of transform NullPointerException with Guava's messages, and every method of a
// null Optional throws NullPointerException, as a call on null does (nn_). Two absent values
// are equal by ==, where Guava's absent() is one object: the generated code compares them only
// with equals.
type Optional_base struct {
	value   any
	present bool
	set     bool // not null
}

// optBaseNil_ is a null Guava Optional; optBaseNull_ reports whether o is null (post-flip tool
// optional -guava wrote them for nil and == nil).
var optBaseNil_ Optional_base

func optBaseNull_(o Optional_base) bool { return !o.set }

func (this Optional_base) nn_() {
	if !this.set {
		panic(new_NullPointerException_0())
	}
}

func Optional_base_absent() Optional_base { return Optional_base{set: true} }

func Optional_base_fromNullable(v any) Optional_base {
	if v == nil {
		return Optional_base{set: true}
	}
	return Optional_base{v, true, true}
}

func Optional_base_of(v any) Optional_base {
	if v == nil {
		panic(new_NullPointerException_0())
	}
	return Optional_base{v, true, true}
}

func (this Optional_base) get() any {
	this.nn_()
	if !this.present {
		stateFail_("Optional.get() cannot be called on an absent value")
	}
	return this.value
}

func (this Optional_base) isPresent() bool { this.nn_(); return this.present }

func (this Optional_base) or(def any) any {
	this.nn_()
	if def == nil {
		nullFail_("use Optional.orNull() instead of Optional.or(null)")
	}
	if this.present {
		return this.value
	}
	return def
}

func (this Optional_base) orNull() any { this.nn_(); return this.value }

func (this Optional_base) or_Optional(o Optional_base) Optional_base {
	this.nn_()
	if !o.set {
		panic(new_NullPointerException_0())
	}
	if this.present {
		return this
	}
	return o
}

func (this Optional_base) transform(f Function_base_e) Optional_base {
	this.nn_()
	if f == nil {
		panic(new_NullPointerException_0())
	}
	if !this.present {
		return this
	}
	r := f.apply(this.value)
	if r == nil {
		nullFail_("the Function passed to Optional.transform() must not return null.")
	}
	return Optional_base{r, true, true}
}

func (this Optional_base) equals(o any) bool {
	this.nn_()
	x, ok := o.(Optional_base)
	if !ok || !x.set {
		return false
	}
	if !this.present || !x.present {
		return this.present == x.present
	}
	return objequals_(this.value, x.value)
}

func (this Optional_base) hashCode() int32 {
	this.nn_()
	if !this.present {
		return 0x79a31aac
	}
	return 0x598df91c + objhash_(this.value)
}

func (this Optional_base) toString() string {
	this.nn_()
	if !this.present {
		return "Optional.absent()"
	}
	return "Optional.of(" + objstr_(this.value) + ")"
}

// ---- java.util.Optional

// Optional is java.util.Optional<T> with its type argument (j2go rule "Optional is a value"):
// a value, not a heap object, whose zero value is Java's null (a field never assigned), so that
// Optional.of and Optional.empty allocate nothing, and whose content is held as a T (an
// Optional<String> holds its string unboxed). Every method of a null Optional throws
// NullPointerException, as a call on null does. Java's Optional has no identity the program may
// use (a value-based class); == on two Optionals is refused by the translator. A wildcard, raw or
// erased type argument is Optional[any]; optAs_ converts between instantiations, which in Java
// are the same object, and box_/optOf_ convert to and from Object.
type Optional[T any] struct {
	value   T
	present bool
	set     bool // not null
}

func (this Optional[T]) nn_() {
	if !this.set {
		panic(new_NullPointerException_0().self())
	}
}

// jnull_ is `this == null`.
func (this Optional[T]) jnull_() bool { return !this.set }

func Optional_empty[T any]() Optional[T] { return Optional[T]{set: true} }

func Optional_of[T any](v T) Optional[T] {
	if jisnull_(v) {
		panic(new_NullPointerException_0().self())
	}
	return Optional[T]{v, true, true}
}

func Optional_ofNullable[T any](v T) Optional[T] {
	if jisnull_(v) {
		return Optional[T]{set: true}
	}
	return Optional[T]{v, true, true}
}

// ---- post-flip step 65 (generics), kept beside j2go's Optional[T]: the helpers its tool writes

// optConv_ is an Optional<S> seen as an Optional<T> (a cast or a widening in Java, where both
// are the same object): f converts the content.
func optConv_[S, T any](o Optional[S], f func(S) T) Optional[T] {
	if !o.present {
		return Optional[T]{set: o.set}
	}
	return Optional[T]{f(o.value), true, true}
}

// optErase_ is an Optional<T> seen as a raw Optional, its content boxed as erased code boxes it.
func optErase_[T any](o Optional[T]) Optional[any] { return optConv_(o, boxValue_[T]) }

// filterFn_ is filter with a predicate that takes the content unboxed.
func (this Optional[T]) filterFn_(p func(T) bool) Optional[T] {
	this.nn_()
	if !this.present || p(this.value) {
		return this
	}
	return Optional[T]{set: true}
}

// optional_ is the post-flip name of joptional_.
type optional_ = joptional_

// joptional_ is any instantiation of Optional.
type joptional_ interface {
	content_() any
	isSet_() bool
}

// content_ is the content as an object, nil when empty.
func (this Optional[T]) content_() any {
	if !this.present {
		return nil
	}
	return boxT_(this.value)
}

func (this Optional[T]) isSet_() bool { return this.set }

// box_ is the Optional as an Object: nil for null.
func (this Optional[T]) box_() any {
	if !this.set {
		return nil
	}
	return this
}

// optOf_ is an Object as an Optional<T> (a cast): nil is null, an Optional of another
// instantiation is converted, anything else is the cast's ClassCastException.
func optOf_[T any](v any) Optional[T] {
	if v == nil {
		return Optional[T]{}
	}
	if o, ok := v.(Optional[T]); ok {
		return o
	}
	o := v.(joptional_)
	if c := o.content_(); c != nil {
		return Optional[T]{junboxT_[T](c), true, true}
	}
	return Optional[T]{set: o.isSet_()}
}

// optAs_ is an Optional<S> seen as an Optional<T> (in Java the same object).
func optAs_[T, S any](o Optional[S]) Optional[T] {
	if !o.present {
		return Optional[T]{set: o.set}
	}
	return Optional[T]{junboxT_[T](boxT_(o.value)), true, true}
}

// isOptional_ is `v instanceof Optional`.
func isOptional_(v any) bool {
	_, ok := v.(joptional_)
	return ok
}

func (this Optional[T]) get() T {
	this.nn_()
	if !this.present {
		panic(new_NoSuchElementException_String("No value present").self())
	}
	return this.value
}

func (this Optional[T]) orElse(other T) T {
	this.nn_()
	if !this.present {
		return other
	}
	return this.value
}

func (this Optional[T]) orElseThrow() T { return this.get() }

func (this Optional[T]) isEmpty() bool   { this.nn_(); return !this.present }
func (this Optional[T]) isPresent() bool { this.nn_(); return this.present }

func (this Optional[T]) equals(o any) bool {
	this.nn_()
	x, ok := o.(joptional_)
	return ok && x.isSet_() && objequals_(this.content_(), x.content_())
}

func (this Optional[T]) hashCode() int32 { this.nn_(); return objhash_(this.content_()) }

func (this Optional[T]) toString() string {
	this.nn_()
	if !this.present {
		return "Optional.empty"
	}
	return "Optional[" + objstr_(this.content_()) + "]"
}

// ---- Predicates, Functions, Suppliers: rt_fn.go (rule "function types") and rt_fn_erased.go

// ---- Throwables

func Throwables_throwIfUnchecked(t *Throwable) {
	if t == nil {
		panic(new_NullPointerException_0().self())
	}
	s := t.self_
	if is_RuntimeException(s) || is_Error(s) {
		panic(s)
	}
}

// ---- CaseFormat

// CaseFormat is Guava's enum as a uint8 whose zero value is null; a constant is its ordinal + 1
// (as the post-flip step tools/postflip/enums writes the translated enums).
type CaseFormat uint8

const (
	CaseFormat_LOWER_HYPHEN CaseFormat = 1 + iota
	CaseFormat_LOWER_UNDERSCORE
	CaseFormat_LOWER_CAMEL
	CaseFormat_UPPER_CAMEL
	CaseFormat_UPPER_UNDERSCORE
)

// CaseFormat_enum_ is the java.lang.Enum record of each constant, by ordinal.
var CaseFormat_enum_ = [5]Enum{
	{self_: CaseFormat_LOWER_HYPHEN, name_: "LOWER_HYPHEN", ordinal_: 0},
	{self_: CaseFormat_LOWER_UNDERSCORE, name_: "LOWER_UNDERSCORE", ordinal_: 1},
	{self_: CaseFormat_LOWER_CAMEL, name_: "LOWER_CAMEL", ordinal_: 2},
	{self_: CaseFormat_UPPER_CAMEL, name_: "UPPER_CAMEL", ordinal_: 3},
	{self_: CaseFormat_UPPER_UNDERSCORE, name_: "UPPER_UNDERSCORE", ordinal_: 4},
}

// caseFormatSep_ is each format's word separator, by ordinal.
var caseFormatSep_ = [5]string{"-", "_", "", "", "_"}

func (this CaseFormat) enum_() *Enum {
	if this == 0 {
		return nil
	}
	return &CaseFormat_enum_[this-1]
}

func (this CaseFormat) name() string          { return this.enum_().name_ }
func (this CaseFormat) ordinal() int32        { return this.enum_().ordinal_ }
func (this CaseFormat) toString() string      { return this.enum_().name_ }
func (this CaseFormat) compareTo(o any) int32 { return this.enum_().compareTo(o) }
func (this CaseFormat) sep_() string          { return caseFormatSep_[this.ordinal()] }

func (this CaseFormat) self() any {
	if this == 0 {
		return nil
	}
	return this
}

func init() {
	registerEnum_("com.google.common.base.CaseFormat", []any{CaseFormat_LOWER_HYPHEN, CaseFormat_LOWER_UNDERSCORE, CaseFormat_LOWER_CAMEL, CaseFormat_UPPER_CAMEL, CaseFormat_UPPER_UNDERSCORE})
}

func firstCharOnlyToUpper_(w string) string {
	if w == "" {
		return w
	}
	return asciiUpper_(w[:1]) + asciiLower_(w[1:])
}

func (this CaseFormat) normalizeWord_(w string) string {
	switch this {
	case CaseFormat_LOWER_HYPHEN, CaseFormat_LOWER_UNDERSCORE:
		return asciiLower_(w)
	case CaseFormat_UPPER_UNDERSCORE:
		return asciiUpper_(w)
	}
	return firstCharOnlyToUpper_(w)
}

func (this CaseFormat) normalizeFirstWord_(w string) string {
	if this == CaseFormat_LOWER_CAMEL {
		return asciiLower_(w)
	}
	return this.normalizeWord_(w)
}

// boundary_ finds the next word boundary of this format at or after byte from.
func (this CaseFormat) boundary_(s string, from int) int {
	for i := from; i < len(s); i++ {
		c := s[i]
		switch this {
		case CaseFormat_LOWER_HYPHEN:
			if c == '-' {
				return i
			}
		case CaseFormat_LOWER_UNDERSCORE, CaseFormat_UPPER_UNDERSCORE:
			if c == '_' {
				return i
			}
		default:
			if c >= 'A' && c <= 'Z' {
				return i
			}
		}
	}
	return -1
}

func (this CaseFormat) to(format CaseFormat, s string) string {
	if format == 0 || s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	if format == this {
		return s
	}
	// The conversions between the two hyphen/underscore lower-case formats and
	// to upper underscore only replace the separator and change case.
	switch {
	case this == CaseFormat_LOWER_HYPHEN && format == CaseFormat_LOWER_UNDERSCORE:
		return strings.ReplaceAll(s, "-", "_")
	case this == CaseFormat_LOWER_HYPHEN && format == CaseFormat_UPPER_UNDERSCORE:
		return asciiUpper_(strings.ReplaceAll(s, "-", "_"))
	case this == CaseFormat_LOWER_UNDERSCORE && format == CaseFormat_LOWER_HYPHEN:
		return strings.ReplaceAll(s, "_", "-")
	case this == CaseFormat_LOWER_UNDERSCORE && format == CaseFormat_UPPER_UNDERSCORE:
		return asciiUpper_(s)
	case this == CaseFormat_UPPER_UNDERSCORE && format == CaseFormat_LOWER_HYPHEN:
		return asciiLower_(strings.ReplaceAll(s, "_", "-"))
	case this == CaseFormat_UPPER_UNDERSCORE && format == CaseFormat_LOWER_UNDERSCORE:
		return asciiLower_(s)
	}
	var out strings.Builder
	i, j := 0, -1
	for {
		j = this.boundary_(s, j+1)
		if j < 0 {
			break
		}
		if i == 0 {
			// A camel-case boundary at index 0 ends an empty first word, which is dropped.
			out.Reset()
			out.WriteString(format.normalizeFirstWord_(s[i:j]))
		} else {
			out.WriteString(format.normalizeWord_(s[i:j]))
		}
		out.WriteString(format.sep_())
		i = j + len(this.sep_())
	}
	if i == 0 {
		return format.normalizeFirstWord_(s)
	}
	out.WriteString(format.normalizeWord_(s[i:]))
	return out.String()
}

// ---- StandardSystemProperty

// StandardSystemProperty is Guava's enum as a uint8 (0 is null, a constant is its ordinal + 1);
// the compiler uses only USER_DIR (ordinal 27), so only it is declared.
type StandardSystemProperty uint8

const StandardSystemProperty_USER_DIR StandardSystemProperty = 27 + 1

var standardSystemPropertyUserDir_ = Enum{self_: StandardSystemProperty_USER_DIR, name_: "USER_DIR", ordinal_: 27}

func (this StandardSystemProperty) enum_() *Enum {
	if this == 0 {
		return nil
	}
	return &standardSystemPropertyUserDir_
}

func (this StandardSystemProperty) name() string          { return this.enum_().name_ }
func (this StandardSystemProperty) ordinal() int32        { return this.enum_().ordinal_ }
func (this StandardSystemProperty) compareTo(o any) int32 { return this.enum_().compareTo(o) }

func (this StandardSystemProperty) self() any {
	if this == 0 {
		return nil
	}
	return this
}

func (this StandardSystemProperty) key() string {
	_ = this.enum_().name_ // null: NullPointerException
	return "user.dir"
}

func (this StandardSystemProperty) value() string {
	return System_getProperty(this.key(), nullstr_)
}

func (this StandardSystemProperty) toString() string {
	return this.key() + "=" + sstr_(this.value())
}
