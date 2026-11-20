package closure

import (
	"fmt"
	"os"
	"runtime"
	"runtime/debug"
	"strings"
)

// ---- enums and classes

var enumRegistry_ = map[string][]any{}

// registerEnum_ records the constants of an enum, in ordinal order, by class name (for
// EnumSet.allOf and Class.getEnumConstants) and in each constant (for EnumSet). The constants
// a constant holds are the most derived objects, as self() gives them.
func registerEnum_(class string, values []any) {
	enumRegistry_[class] = values
	univ := make([]any, len(values))
	for i, v := range values {
		e := v.(enumConst_).enum_()
		if int(e.ordinal_) != i {
			panic("registerEnum_: " + class + " constants not in ordinal order")
		}
		univ[i] = e.self()
	}
	for _, v := range univ {
		v.(enumConst_).enum_().values_ = univ
	}
}

func enumValues_[T comparable](vs []T) []any {
	out := make([]any, len(vs))
	for i, v := range vs {
		out[i] = obj_(v)
	}
	return out
}

// enumValueOfFail_ is the exception Enum.valueOf throws for a name that is not a constant of
// the enum: a Go error value, which the generated valueOf panics with.
func enumValueOfFail_(enum string, name string) *IllegalArgumentException {
	return new_IllegalArgumentException_String("No enum constant " + enum + "." + name)
}

var classes_ = map[string]*Class{}

// classOf_ is the Class object of a class literal, by qualified Java name.
func classOf_(name string) *Class {
	c := classes_[name]
	if c == nil {
		c = &Class{name_: name}
		classes_[name] = c
	}
	return c
}

// ---- exceptions

// catch_ turns a recovered panic value into a Java exception object.
func catch_(r any) any {
	if _, ok := r.(interface{ as_Throwable() *Throwable }); ok {
		return r
	}
	switch e := r.(type) {
	case runtime.Error:
		msg := e.Error()
		if os.Getenv("JSGO_TRACE_THROW") != "" {
			fmt.Fprintf(os.Stderr, "--- go runtime error: %s\n%s\n", msg, debug.Stack())
		}
		switch {
		case strings.Contains(msg, "nil pointer dereference") || strings.Contains(msg, "nil map"):
			return new_NullPointerException_0().self()
		case strings.Contains(msg, "index out of range") || strings.Contains(msg, "slice bounds out of range"):
			return new_ArrayIndexOutOfBoundsException_String(msg).self()
		case strings.Contains(msg, "interface conversion"):
			return new_ClassCastException_String(msg).self()
		case strings.Contains(msg, "divide by zero"):
			return new_ArithmeticException_String("/ by zero").self()
		}
		return new_RuntimeException_String("go runtime: " + msg).self()
	case frozen_:
		// a change to an immutable typed collection (rt_timm.go)
		return new_UnsupportedOperationException_0().self()
	case error:
		return new_RuntimeException_String(e.Error()).self()
	case string:
		return new_RuntimeException_String(e).self()
	}
	if _, ok := r.(interface{ as_Throwable() *Throwable }); ok {
		return r
	}
	return new_RuntimeException_String(fmt.Sprint(r)).self()
}

func closeResource_(r any) {
	if r == nil {
		return
	}
	if c, ok := r.(interface{ close() }); ok {
		c.close()
	}
}

// flushStd_ flushes the standard streams before the process exits.
var flushStd_ = func() {}

// ---- static initialization

// initOrder_ records, in an instrumented build, the classes in the order their
// static initialization completed.
var initOrder_ []string

// initFailures_ are the static initializers that panicked at startup.
var initFailures_ []string

// runClinit_ runs one class's static initializer. A failure is remembered and
// does not stop the others: Java would only fail when the class is first used.
func runClinit_(f func()) {
	defer func() {
		if r := recover(); r != nil {
			msg := fmt.Sprint(r)
			if t, ok := r.(interface{ toString() string }); ok {
				msg = t.toString()
			}
			if os.Getenv("JSGO_DEBUG_INIT") == "2" {
				msg += "\n" + string(debug.Stack())
			}
			initFailures_ = append(initFailures_, msg)
		}
	}()
	f()
}
