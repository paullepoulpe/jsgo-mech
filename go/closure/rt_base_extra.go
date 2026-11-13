package closure

// Small members next to the com.google.common.base / java.util area that the
// translated code needs: the default method of Guava's Predicate_base and two
// java.util.function statics.

// Predicate_test is the default method Predicate_base.test of Guava's Predicate_base.

type identityFn_ struct{}

// AbstractStringBuilder is the superclass of StringBuilder; nothing creates
// one directly.
type AbstractStringBuilder struct {
	sb StringBuilder
}

// Format is java.text.Format, the superclass of MessageFormat; only the
// static MessageFormat.format is used.
type Format struct{}
