package closure

// Small members next to the com.google.common.base / java.util area that the
// translated code needs: the default method of Guava's Predicate and two
// java.util.function statics.

// Predicate_test is the default method Predicate.test of Guava's Predicate.

type identityFn_ struct{}

func (identityFn_) apply(v any) any { return v }

// AbstractStringBuilder is the superclass of StringBuilder; nothing creates
// one directly.
type AbstractStringBuilder struct {
	sb StringBuilder
}

func (this *AbstractStringBuilder) append_CharSequence(v any) Appendable {
	this.sb.append_CharSequence(v)
	return this
}

func (this *AbstractStringBuilder) append_CharSequence_int_int(v any, a int32, b int32) Appendable {
	this.sb.append_CharSequence_int_int(v, a, b)
	return this
}

func (this *AbstractStringBuilder) append_char(c uint16) Appendable {
	this.sb.append_char(c)
	return this
}

func (this *AbstractStringBuilder) charAt(i int32) uint16       { return this.sb.charAt(i) }
func (this *AbstractStringBuilder) length() int32               { return this.sb.length() }
func (this *AbstractStringBuilder) setCharAt(i int32, c uint16) { this.sb.setCharAt(i, c) }
func (this *AbstractStringBuilder) setLength(n int32)           { this.sb.setLength(n) }
func (this *AbstractStringBuilder) substring(a int32) string    { return this.sb.substring(a) }
func (this *AbstractStringBuilder) toString() string            { return this.sb.toString() }
func (this *AbstractStringBuilder) substring_int_int(a int32, b int32) string {
	return this.sb.substring_int_int(a, b)
}

// Format is java.text.Format, the superclass of MessageFormat; only the
// static MessageFormat.format is used.
type Format struct{}
