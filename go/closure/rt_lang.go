package closure

import (
	"fmt"
	"runtime"
	"strings"
)

// ---- enums and classes

var enumRegistry_ = map[string][]any{}

func registerEnum_(class string, values []any) { enumRegistry_[class] = values }

func enumValues_[T comparable](vs []T) []any {
	out := make([]any, len(vs))
	for i, v := range vs {
		out[i] = obj_(v)
	}
	return out
}

func enumValueOfFail_(enum string, name string) any {
	return new_IllegalArgumentException_String("No enum constant " + enum + "." + name).self()
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
	switch e := r.(type) {
	case runtime.Error:
		msg := e.Error()
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
