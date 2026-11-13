package closure

// com.google.common.base: Preconditions, Strings, Ascii, MoreObjects, Optional,
// Predicates, Functions, Suppliers, Throwables, CaseFormat; java.util.Optional.

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

func Strings_emptyToNull(s string) string {
	if s == "" {
		return nullstr_
	}
	return s
}

func Strings_isNullOrEmpty(s string) bool { return s == nullstr_ || s == "" }

func Strings_nullToEmpty(s string) string {
	if s == nullstr_ {
		return ""
	}
	return s
}

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
		argFail_(objstr_(msg))
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
		nullFail_(objstr_(msg))
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

func Preconditions_checkState(b bool) {
	if !b {
		panic(new_IllegalStateException_0().self())
	}
}

func Preconditions_checkState_boolean_Object(b bool, msg any) {
	if !b {
		stateFail_(objstr_(msg))
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

// ---- com.google.common.base.Optional

type Optional struct {
	present bool
	value   any
}

var optionalAbsent_ = &Optional{}

func Optional_absent() *Optional { return optionalAbsent_ }

func Optional_fromNullable(v any) *Optional {
	if v == nil {
		return optionalAbsent_
	}
	return &Optional{true, v}
}

func Optional_of(v any) *Optional {
	if v == nil {
		panic(new_NullPointerException_0().self())
	}
	return &Optional{true, v}
}

func (this *Optional) get() any {
	if !this.present {
		stateFail_("Optional.get() cannot be called on an absent value")
	}
	return this.value
}

func (this *Optional) isPresent() bool { return this.present }

func (this *Optional) or(def any) any {
	if def == nil {
		nullFail_("use Optional.orNull() instead of Optional.or(null)")
	}
	if this.present {
		return this.value
	}
	return def
}

func (this *Optional) orNull() any { return this.value }

func (this *Optional) or_Optional(o *Optional) *Optional {
	if o == nil {
		panic(new_NullPointerException_0().self())
	}
	if this.present {
		return this
	}
	return o
}

func (this *Optional) transform(f Function_base) *Optional {
	if f == nil {
		panic(new_NullPointerException_0().self())
	}
	if !this.present {
		return this
	}
	r := f.apply(this.value)
	if r == nil {
		nullFail_("the Function passed to Optional.transform() must not return null.")
	}
	return &Optional{true, r}
}

func (this *Optional) equals(o any) bool {
	x, ok := o.(*Optional)
	if !ok || x == nil {
		return false
	}
	if !this.present || !x.present {
		return this.present == x.present
	}
	return objequals_(this.value, x.value)
}

func (this *Optional) hashCode() int32 {
	if !this.present {
		return 0x79a31aac
	}
	return 0x598df91c + objhash_(this.value)
}

func (this *Optional) toString() string {
	if !this.present {
		return "Optional.absent()"
	}
	return "Optional.of(" + objstr_(this.value) + ")"
}

// ---- java.util.Optional

type Optional_util struct {
	value any
}

var optionalUtilEmpty_ = &Optional_util{}

func Optional_util_empty() *Optional_util { return optionalUtilEmpty_ }

func Optional_util_of(v any) *Optional_util {
	if v == nil {
		panic(new_NullPointerException_0().self())
	}
	return &Optional_util{v}
}

func (this *Optional_util) get() any {
	if this.value == nil {
		panic(new_NoSuchElementException_String("No value present").self())
	}
	return this.value
}

func (this *Optional_util) ifPresent(c Consumer) {
	if this.value != nil {
		c.accept(this.value)
	}
}

func (this *Optional_util) isEmpty() bool   { return this.value == nil }
func (this *Optional_util) isPresent() bool { return this.value != nil }

func (this *Optional_util) orElse(def any) any {
	if this.value != nil {
		return this.value
	}
	return def
}

func (this *Optional_util) orElseThrow(s Supplier) any {
	if this.value != nil {
		return this.value
	}
	ex := s.get()
	if e, ok := ex.(interface{ self() any }); ok {
		ex = e.self()
	}
	panic(ex)
}

func (this *Optional_util) equals(o any) bool {
	x, ok := o.(*Optional_util)
	return ok && x != nil && objequals_(this.value, x.value)
}

func (this *Optional_util) hashCode() int32 { return objhash_(this.value) }

func (this *Optional_util) toString() string {
	if this.value == nil {
		return "Optional.empty"
	}
	return "Optional[" + objstr_(this.value) + "]"
}

// ---- Predicates, Functions, Suppliers

type alwaysTruePredicate_ struct{}

func (alwaysTruePredicate_) apply(any) bool   { return true }
func (alwaysTruePredicate_) test(any) bool    { return true }
func (alwaysTruePredicate_) toString() string { return "Predicates.alwaysTrue()" }

var alwaysTrue_ Predicate = alwaysTruePredicate_{}

func Predicates_alwaysTrue() Predicate { return alwaysTrue_ }

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

func Predicates_equalTo(target any) Predicate {
	if target == nil {
		return isNullPredicate_{}
	}
	return &equalToPredicate_{target}
}

type identityFunction_ struct{}

func (identityFunction_) apply(v any) any  { return v }
func (identityFunction_) toString() string { return "Functions.identity()" }

func Functions_identity() Function_base { return identityFunction_{} }

type memoizingSupplier_ struct {
	delegate Supplier_base
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

func Suppliers_memoize(s Supplier_base) Supplier_base {
	if m, ok := s.(*memoizingSupplier_); ok {
		return m
	}
	if s == nil {
		panic(new_NullPointerException_0().self())
	}
	return &memoizingSupplier_{delegate: s}
}

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

type CaseFormat struct {
	Enum
	sep_ string
}

func (this *CaseFormat) up_Enum() *Enum {
	if this == nil {
		return nil
	}
	return &this.Enum
}

func newCaseFormat_(name string, ordinal int32, sep string) *CaseFormat {
	this := &CaseFormat{sep_: sep}
	this.self_ = this
	this.Enum.init_Enum(name, ordinal)
	return this
}

var CaseFormat_LOWER_HYPHEN = newCaseFormat_("LOWER_HYPHEN", 0, "-")
var CaseFormat_LOWER_UNDERSCORE = newCaseFormat_("LOWER_UNDERSCORE", 1, "_")
var CaseFormat_LOWER_CAMEL = newCaseFormat_("LOWER_CAMEL", 2, "")
var CaseFormat_UPPER_CAMEL = newCaseFormat_("UPPER_CAMEL", 3, "")
var CaseFormat_UPPER_UNDERSCORE = newCaseFormat_("UPPER_UNDERSCORE", 4, "_")

func init() {
	registerEnum_("com.google.common.base.CaseFormat", []any{CaseFormat_LOWER_HYPHEN, CaseFormat_LOWER_UNDERSCORE, CaseFormat_LOWER_CAMEL, CaseFormat_UPPER_CAMEL, CaseFormat_UPPER_UNDERSCORE})
}

func firstCharOnlyToUpper_(w string) string {
	if w == "" {
		return w
	}
	return asciiUpper_(w[:1]) + asciiLower_(w[1:])
}

func (this *CaseFormat) normalizeWord_(w string) string {
	switch this {
	case CaseFormat_LOWER_HYPHEN, CaseFormat_LOWER_UNDERSCORE:
		return asciiLower_(w)
	case CaseFormat_UPPER_UNDERSCORE:
		return asciiUpper_(w)
	}
	return firstCharOnlyToUpper_(w)
}

func (this *CaseFormat) normalizeFirstWord_(w string) string {
	if this == CaseFormat_LOWER_CAMEL {
		return asciiLower_(w)
	}
	return this.normalizeWord_(w)
}

// boundary_ finds the next word boundary of this format at or after byte from.
func (this *CaseFormat) boundary_(s string, from int) int {
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

func (this *CaseFormat) to(format *CaseFormat, s string) string {
	if format == nil || s == nullstr_ {
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
		out.WriteString(format.sep_)
		i = j + len(this.sep_)
	}
	if i == 0 {
		return format.normalizeFirstWord_(s)
	}
	out.WriteString(format.normalizeWord_(s[i:]))
	return out.String()
}

// ---- StandardSystemProperty

type StandardSystemProperty struct {
	Enum
	key_ string
}

func (this *StandardSystemProperty) up_Enum() *Enum {
	if this == nil {
		return nil
	}
	return &this.Enum
}

func (this *StandardSystemProperty) key() string { return this.key_ }

func (this *StandardSystemProperty) value() string {
	return System_getProperty(this.key_, nullstr_)
}

func (this *StandardSystemProperty) toString() string {
	return this.key_ + "=" + sstr_(this.value())
}

var StandardSystemProperty_USER_DIR = func() *StandardSystemProperty {
	this := &StandardSystemProperty{key_: "user.dir"}
	this.self_ = this
	this.Enum.init_Enum("USER_DIR", 27)
	return this
}()
