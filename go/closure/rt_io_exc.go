package closure

// The exception classes of java.io, java.util.concurrent, Guava's
// util.concurrent and SAX: each is its superclass plus the standard
// constructors.

type IOException struct {
	Exception
}

func (this *IOException) as_IOException() *IOException { return this }

func to_IOException(v any) *IOException {
	if v == nil {
		return nil
	}
	return v.(interface{ as_IOException() *IOException }).as_IOException()
}

func is_IOException(v any) bool {
	_, ok := v.(interface{ as_IOException() *IOException })
	return ok
}

func (this *IOException) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *IOException) up_Exception() *Exception {
	if this == nil {
		return nil
	}
	return &this.Exception
}

func (this *IOException) init_IOException_0() {
	this.Exception.init_Exception_0()
}

func (this *IOException) init_IOException_String(p0 string) {
	this.Exception.init_Exception_String(p0)
}

func new_IOException_String(p0 string) *IOException {
	this := &IOException{}
	this.self_ = this
	this.init_IOException_String(p0)
	return this
}

func (this *IOException) init_IOException_String_Throwable(p0 string, p1 *Throwable) {
	this.Exception.init_Exception_String_Throwable(p0, p1)
}

func new_IOException_String_Throwable(p0 string, p1 *Throwable) *IOException {
	this := &IOException{}
	this.self_ = this
	this.init_IOException_String_Throwable(p0, p1)
	return this
}

func (this *IOException) init_IOException_Throwable(p0 *Throwable) {
	this.Exception.init_Exception_Throwable(p0)
}

func new_IOException_Throwable(p0 *Throwable) *IOException {
	this := &IOException{}
	this.self_ = this
	this.init_IOException_Throwable(p0)
	return this
}

type FileNotFoundException struct {
	IOException
}

func (this *FileNotFoundException) as_FileNotFoundException() *FileNotFoundException { return this }

func (this *FileNotFoundException) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *FileNotFoundException) up_IOException() *IOException {
	if this == nil {
		return nil
	}
	return &this.IOException
}

func (this *FileNotFoundException) init_FileNotFoundException_0() {
	this.IOException.init_IOException_0()
}

func (this *FileNotFoundException) init_FileNotFoundException_String(p0 string) {
	this.IOException.init_IOException_String(p0)
}

func new_FileNotFoundException_String(p0 string) *FileNotFoundException {
	this := &FileNotFoundException{}
	this.self_ = this
	this.init_FileNotFoundException_String(p0)
	return this
}

type UncheckedIOException struct {
	RuntimeException
}

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

func (this *ExecutionException) up_Exception() *Exception {
	if this == nil {
		return nil
	}
	return &this.Exception
}

func (this *ExecutionException) init_ExecutionException_0() {
	this.Exception.init_Exception_0()
}

func (this *ExecutionException) init_ExecutionException_String(p0 string) {
	this.Exception.init_Exception_String(p0)
}

func (this *ExecutionException) init_ExecutionException_String_Throwable(p0 string, p1 *Throwable) {
	this.Exception.init_Exception_String_Throwable(p0, p1)
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

func (this *UncheckedExecutionException) up_RuntimeException() *RuntimeException {
	if this == nil {
		return nil
	}
	return &this.RuntimeException
}

func (this *UncheckedExecutionException) init_UncheckedExecutionException_0() {
	this.RuntimeException.init_RuntimeException_0()
}

func (this *UncheckedExecutionException) init_UncheckedExecutionException_String(p0 string) {
	this.RuntimeException.init_RuntimeException_String(p0)
}

func (this *UncheckedExecutionException) init_UncheckedExecutionException_String_Throwable(p0 string, p1 *Throwable) {
	this.RuntimeException.init_RuntimeException_String_Throwable(p0, p1)
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

func (this *SAXException) up_Exception() *Exception {
	if this == nil {
		return nil
	}
	return &this.Exception
}

func (this *SAXException) init_SAXException_0() {
	this.Exception.init_Exception_0()
}

func (this *SAXException) init_SAXException_String(p0 string) {
	this.Exception.init_Exception_String(p0)
}

func new_SAXException_String(p0 string) *SAXException {
	this := &SAXException{}
	this.self_ = this
	this.init_SAXException_String(p0)
	return this
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
