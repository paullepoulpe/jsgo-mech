package closure

// The exception classes of java.util.concurrent, Guava's util.concurrent and
// SAX: each is its superclass plus the standard constructors. (The exceptions
// of I/O are in rt_io.go.)

type ExecutionException struct {
	Exception
}

func (this *ExecutionException) as_ExecutionException() *ExecutionException { return this }

func is_ExecutionException(v any) bool {
	_, ok := v.(interface{ as_ExecutionException() *ExecutionException })
	return ok
}

func (this *ExecutionException) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *ExecutionException) init_ExecutionException_Throwable(p0 *Throwable) {
	this.Exception.init_Exception_Throwable(p0)
}

func new_ExecutionException_Throwable(p0 *Throwable) *ExecutionException {
	this := &ExecutionException{}
	this.self_ = this
	this.init_ExecutionException_Throwable(p0)
	return this
}

type TimeoutException struct {
	Exception
}

func is_TimeoutException(v any) bool {
	_, ok := v.(interface{ as_TimeoutException() *TimeoutException })
	return ok
}

type UncheckedExecutionException struct {
	RuntimeException
}

func (this *UncheckedExecutionException) as_UncheckedExecutionException() *UncheckedExecutionException {
	return this
}

func (this *UncheckedExecutionException) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *UncheckedExecutionException) init_UncheckedExecutionException_Throwable(p0 *Throwable) {
	this.RuntimeException.init_RuntimeException_Throwable(p0)
}

func new_UncheckedExecutionException_Throwable(p0 *Throwable) *UncheckedExecutionException {
	this := &UncheckedExecutionException{}
	this.self_ = this
	this.init_UncheckedExecutionException_Throwable(p0)
	return this
}

type SAXException struct {
	Exception
}

func (this *SAXException) as_SAXException() *SAXException { return this }

func is_SAXException(v any) bool {
	_, ok := v.(interface{ as_SAXException() *SAXException })
	return ok
}

func (this *SAXException) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *SAXException) init_SAXException_String(p0 string) {
	this.Exception.init_Exception_String(p0)
}

type ParserConfigurationException struct {
	Exception
}

func is_ParserConfigurationException(v any) bool {
	_, ok := v.(interface {
		as_ParserConfigurationException() *ParserConfigurationException
	})
	return ok
}
