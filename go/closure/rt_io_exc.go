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

func new_IOException_0() *IOException {
	this := &IOException{}
	this.self_ = this
	this.init_IOException_0()
	return this
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

func to_FileNotFoundException(v any) *FileNotFoundException {
	if v == nil {
		return nil
	}
	return v.(interface{ as_FileNotFoundException() *FileNotFoundException }).as_FileNotFoundException()
}

func is_FileNotFoundException(v any) bool {
	_, ok := v.(interface{ as_FileNotFoundException() *FileNotFoundException })
	return ok
}

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

func new_FileNotFoundException_0() *FileNotFoundException {
	this := &FileNotFoundException{}
	this.self_ = this
	this.init_FileNotFoundException_0()
	return this
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

func (this *UncheckedIOException) as_UncheckedIOException() *UncheckedIOException { return this }

func to_UncheckedIOException(v any) *UncheckedIOException {
	if v == nil {
		return nil
	}
	return v.(interface{ as_UncheckedIOException() *UncheckedIOException }).as_UncheckedIOException()
}

func is_UncheckedIOException(v any) bool {
	_, ok := v.(interface{ as_UncheckedIOException() *UncheckedIOException })
	return ok
}

func (this *UncheckedIOException) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *UncheckedIOException) up_RuntimeException() *RuntimeException {
	if this == nil {
		return nil
	}
	return &this.RuntimeException
}

type ExecutionException struct {
	Exception
}

func (this *ExecutionException) as_ExecutionException() *ExecutionException { return this }

func to_ExecutionException(v any) *ExecutionException {
	if v == nil {
		return nil
	}
	return v.(interface{ as_ExecutionException() *ExecutionException }).as_ExecutionException()
}

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

func new_ExecutionException_0() *ExecutionException {
	this := &ExecutionException{}
	this.self_ = this
	this.init_ExecutionException_0()
	return this
}

func (this *ExecutionException) init_ExecutionException_String(p0 string) {
	this.Exception.init_Exception_String(p0)
}

func new_ExecutionException_String(p0 string) *ExecutionException {
	this := &ExecutionException{}
	this.self_ = this
	this.init_ExecutionException_String(p0)
	return this
}

func (this *ExecutionException) init_ExecutionException_String_Throwable(p0 string, p1 *Throwable) {
	this.Exception.init_Exception_String_Throwable(p0, p1)
}

func new_ExecutionException_String_Throwable(p0 string, p1 *Throwable) *ExecutionException {
	this := &ExecutionException{}
	this.self_ = this
	this.init_ExecutionException_String_Throwable(p0, p1)
	return this
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

func (this *TimeoutException) as_TimeoutException() *TimeoutException { return this }

func to_TimeoutException(v any) *TimeoutException {
	if v == nil {
		return nil
	}
	return v.(interface{ as_TimeoutException() *TimeoutException }).as_TimeoutException()
}

func is_TimeoutException(v any) bool {
	_, ok := v.(interface{ as_TimeoutException() *TimeoutException })
	return ok
}

func (this *TimeoutException) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *TimeoutException) up_Exception() *Exception {
	if this == nil {
		return nil
	}
	return &this.Exception
}

func (this *TimeoutException) init_TimeoutException_0() {
	this.Exception.init_Exception_0()
}

func new_TimeoutException_0() *TimeoutException {
	this := &TimeoutException{}
	this.self_ = this
	this.init_TimeoutException_0()
	return this
}

func (this *TimeoutException) init_TimeoutException_String(p0 string) {
	this.Exception.init_Exception_String(p0)
}

func new_TimeoutException_String(p0 string) *TimeoutException {
	this := &TimeoutException{}
	this.self_ = this
	this.init_TimeoutException_String(p0)
	return this
}

type UncheckedExecutionException struct {
	RuntimeException
}

func (this *UncheckedExecutionException) as_UncheckedExecutionException() *UncheckedExecutionException {
	return this
}

func to_UncheckedExecutionException(v any) *UncheckedExecutionException {
	if v == nil {
		return nil
	}
	return v.(interface {
		as_UncheckedExecutionException() *UncheckedExecutionException
	}).as_UncheckedExecutionException()
}

func is_UncheckedExecutionException(v any) bool {
	_, ok := v.(interface {
		as_UncheckedExecutionException() *UncheckedExecutionException
	})
	return ok
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

func new_UncheckedExecutionException_0() *UncheckedExecutionException {
	this := &UncheckedExecutionException{}
	this.self_ = this
	this.init_UncheckedExecutionException_0()
	return this
}

func (this *UncheckedExecutionException) init_UncheckedExecutionException_String(p0 string) {
	this.RuntimeException.init_RuntimeException_String(p0)
}

func new_UncheckedExecutionException_String(p0 string) *UncheckedExecutionException {
	this := &UncheckedExecutionException{}
	this.self_ = this
	this.init_UncheckedExecutionException_String(p0)
	return this
}

func (this *UncheckedExecutionException) init_UncheckedExecutionException_String_Throwable(p0 string, p1 *Throwable) {
	this.RuntimeException.init_RuntimeException_String_Throwable(p0, p1)
}

func new_UncheckedExecutionException_String_Throwable(p0 string, p1 *Throwable) *UncheckedExecutionException {
	this := &UncheckedExecutionException{}
	this.self_ = this
	this.init_UncheckedExecutionException_String_Throwable(p0, p1)
	return this
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

func to_SAXException(v any) *SAXException {
	if v == nil {
		return nil
	}
	return v.(interface{ as_SAXException() *SAXException }).as_SAXException()
}

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

func new_SAXException_0() *SAXException {
	this := &SAXException{}
	this.self_ = this
	this.init_SAXException_0()
	return this
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

func (this *ParserConfigurationException) as_ParserConfigurationException() *ParserConfigurationException {
	return this
}

func to_ParserConfigurationException(v any) *ParserConfigurationException {
	if v == nil {
		return nil
	}
	return v.(interface {
		as_ParserConfigurationException() *ParserConfigurationException
	}).as_ParserConfigurationException()
}

func is_ParserConfigurationException(v any) bool {
	_, ok := v.(interface {
		as_ParserConfigurationException() *ParserConfigurationException
	})
	return ok
}

func (this *ParserConfigurationException) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *ParserConfigurationException) up_Exception() *Exception {
	if this == nil {
		return nil
	}
	return &this.Exception
}

func (this *ParserConfigurationException) init_ParserConfigurationException_0() {
	this.Exception.init_Exception_0()
}

func new_ParserConfigurationException_0() *ParserConfigurationException {
	this := &ParserConfigurationException{}
	this.self_ = this
	this.init_ParserConfigurationException_0()
	return this
}

func (this *ParserConfigurationException) init_ParserConfigurationException_String(p0 string) {
	this.Exception.init_Exception_String(p0)
}

func new_ParserConfigurationException_String(p0 string) *ParserConfigurationException {
	this := &ParserConfigurationException{}
	this.self_ = this
	this.init_ParserConfigurationException_String(p0)
	return this
}
