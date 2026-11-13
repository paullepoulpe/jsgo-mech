package closure

// java.lang.Runtime / Thread / ThreadLocal / Throwable printing,
// java.util.concurrent and Guava's concurrency helpers, java.lang.management
// and java.util.logging.
//
// Everything runs on one goroutine: starting a thread runs it to completion,
// and an executor runs a task when it is submitted.

import (
	"runtime"
	"strconv"
	"strings"
)

func init() {
	registerEnum_("java.util.concurrent.TimeUnit", []any{TimeUnit_NANOSECONDS, TimeUnit_MICROSECONDS, TimeUnit_MILLISECONDS,
		TimeUnit_SECONDS, TimeUnit_MINUTES, TimeUnit_HOURS, TimeUnit_DAYS})
}

// caught_ is the Java exception of a recovered panic value.
func caught_(r any) any {
	if _, ok := r.(interface{ as_Throwable() *Throwable }); ok {
		return r
	}
	return catch_(r)
}

// ---- java.lang.Runtime

type Runtime struct{}

var runtime_ = &Runtime{}

func Runtime_getRuntime() *Runtime { return runtime_ }

func memStats_() *runtime.MemStats {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)
	return &m
}

// totalMemory is the memory obtained from the operating system for the heap,
// freeMemory the part of it that holds no live objects.
func (this *Runtime) totalMemory() int64 { return int64(memStats_().HeapSys) }

func (this *Runtime) freeMemory() int64 {
	m := memStats_()
	return int64(m.HeapSys - m.HeapAlloc)
}

// ---- java.lang.Thread

type ThreadGroup struct{}

type Thread struct {
	self_        any
	target_      Runnable
	name_        string
	daemon_      bool
	started_     bool
	done_        bool
	interrupted_ bool
	handler_     Thread_UncaughtExceptionHandler
}

func (this *Thread) as_Thread() *Thread { return this }

var threadCount_ int32

// mainThread_ is the thread of the process; threads_ are the threads whose
// run() is executing, innermost last.
var mainThread_ = func() *Thread {
	t := &Thread{name_: "main", started_: true}
	t.self_ = t
	return t
}()
var threads_ = []*Thread{mainThread_}

func (this *Thread) init_Thread_ThreadGroup_Runnable_String_long(g *ThreadGroup, r Runnable, name string, stackSize int64) {
	this.target_ = r
	this.name_ = name
}

func new_Thread_ThreadGroup_Runnable_String_long(g *ThreadGroup, r Runnable, name string, stackSize int64) *Thread {
	if name == nullstr_ {
		panic(new_NullPointerException_String("name cannot be null").self())
	}
	this := &Thread{}
	this.self_ = this
	this.init_Thread_ThreadGroup_Runnable_String_long(g, r, name, stackSize)
	return this
}

func nextThreadName_() string {
	n := threadCount_
	threadCount_++
	return "Thread-" + istr_(int64(n))
}

func new_Thread_Runnable(r Runnable) *Thread {
	return new_Thread_ThreadGroup_Runnable_String_long(nil, r, nextThreadName_(), 0)
}

func Thread_currentThread() *Thread { return threads_[len(threads_)-1] }

// Thread_interrupted reports and clears the interrupt status of the current thread.
func Thread_interrupted() bool {
	t := Thread_currentThread()
	was := t.interrupted_
	t.interrupted_ = false
	return was
}

// run runs the thread's Runnable, if it has one.
func (this *Thread) run() {
	if this.target_ != nil {
		this.target_.run()
	}
}

// start runs the thread to completion. An exception that escapes run() ends
// the thread, not the caller: it goes to the uncaught exception handler, or
// is printed.
func (this *Thread) start() {
	if this.started_ {
		panic(new_RuntimeException_String("java.lang.IllegalThreadStateException").self())
	}
	this.started_ = true
	threads_ = append(threads_, this)
	defer func() {
		threads_ = threads_[:len(threads_)-1]
		this.done_ = true
		if r := recover(); r != nil {
			ex := caught_(r)
			if h, ok := this.handler_.(interface{ uncaughtException(*Thread, *Throwable) }); ok {
				h.uncaughtException(this, to_Throwable(ex))
				return
			}
			printErr_("Exception in thread \"" + this.name_ + "\" " + Io_stackTraceText(to_Throwable(ex)))
		}
	}()
	if s, ok := this.self_.(interface{ run() }); ok {
		s.run()
	} else {
		this.run()
	}
}

func (this *Thread) setDaemon(on bool)   { this.daemon_ = on }
func (this *Thread) getName() string     { return this.name_ }
func (this *Thread) setPriority(p int32) {}
func (this *Thread) getPriority() int32  { return 5 }
func (this *Thread) getId() int64        { return int64(identityHash_(this)) }
func (this *Thread) toString() string    { return "Thread[" + this.name_ + ",5,main]" }

// ---- java.lang.ThreadLocal

type ThreadLocal struct {
	value_   any
	set_     bool
	initial_ Supplier
}

func new_ThreadLocal() *ThreadLocal { return &ThreadLocal{} }

func (this *ThreadLocal) get() any {
	if !this.set_ {
		this.set_ = true
		this.value_ = nil
		if this.initial_ != nil {
			this.value_ = this.initial_.get()
		}
	}
	return this.value_
}

func (this *ThreadLocal) set(v any) {
	this.value_ = v
	this.set_ = true
}

func (this *ThreadLocal) remove() {
	this.value_ = nil
	this.set_ = false
}

// ---- java.lang.Throwable (the struct is in rt_object.go)

func new_Throwable_0() *Throwable {
	this := &Throwable{}
	this.self_ = this
	this.init_Throwable_0()
	return this
}

func new_Throwable_String(m string) *Throwable {
	this := &Throwable{}
	this.self_ = this
	this.init_Throwable_String(m)
	return this
}

func throwableString_(t *Throwable) string {
	if s, ok := t.self_.(interface{ toString() string }); ok {
		return s.toString()
	}
	return t.toString()
}

// ---- java.util.concurrent.TimeUnit

type TimeUnit struct {
	Enum
	nanos_ int64
}

func newTimeUnit_(name string, ordinal int32, nanos int64) *TimeUnit {
	e := &TimeUnit{nanos_: nanos}
	e.self_ = e
	e.init_Enum(name, ordinal)
	return e
}

var TimeUnit_NANOSECONDS = newTimeUnit_("NANOSECONDS", 0, 1)
var TimeUnit_MICROSECONDS = newTimeUnit_("MICROSECONDS", 1, 1e3)
var TimeUnit_MILLISECONDS = newTimeUnit_("MILLISECONDS", 2, 1e6)
var TimeUnit_SECONDS = newTimeUnit_("SECONDS", 3, 1e9)
var TimeUnit_MINUTES = newTimeUnit_("MINUTES", 4, 60e9)
var TimeUnit_HOURS = newTimeUnit_("HOURS", 5, 3600e9)
var TimeUnit_DAYS = newTimeUnit_("DAYS", 6, 86400e9)

// ---- futures

// future_ is a completed Future (also a Guava ListenableFuture).
type future_ struct {
	value_  any
	failed_ *Throwable // what the task threw
}

func (this *future_) get() any {
	if this.failed_ != nil {
		panic(new_ExecutionException_Throwable(this.failed_).self())
	}
	return this.value_
}

func (this *future_) get_long_TimeUnit(timeout int64, unit *TimeUnit) any { return this.get() }

// runTask_ runs a task and returns its completed future.
func runTask_(task func() any) *future_ {
	f := &future_{}
	func() {
		defer func() {
			if r := recover(); r != nil {
				f.failed_ = to_Throwable(caught_(r))
			}
		}()
		f.value_ = task()
	}()
	return f
}

// Futures_allAsList is a future of the list of all results, which fails if
// any of the futures failed.
func Futures_allAsList(futures Iterable) ListenableFuture {
	out := &future_{}
	results := []any{}
	for it := futures.iterator(); it.hasNext(); {
		f := it.next().(Future)
		one := runTask_(f.get)
		if one.failed_ != nil {
			cause := one.failed_
			if is_ExecutionException(cause.self_) && cause.cause_ != nil {
				cause = cause.cause_
			}
			if out.failed_ == nil {
				out.failed_ = cause
			}
			continue
		}
		results = append(results, one.value_)
	}
	if out.failed_ == nil {
		out.value_ = listOf_(results)
	}
	return out
}

// ---- executors

// AbstractExecutorService runs every task at once, on a thread from its
// factory if it has one.
type AbstractExecutorService struct {
	factory_  ThreadFactory
	shutdown_ bool
}

func rejected_() any {
	// java.util.concurrent.RejectedExecutionException
	return new_RuntimeException_String("Task rejected from executor that has been shut down").self()
}

func (this *AbstractExecutorService) execute(task Runnable) {
	if task == nil {
		panic(new_NullPointerException_0().self())
	}
	if this.shutdown_ {
		panic(rejected_())
	}
	if this.factory_ != nil {
		if t := this.factory_.newThread(task); t != nil {
			t.start()
			return
		}
	}
	new_Thread_Runnable(task).start()
}

func (this *AbstractExecutorService) submit(task Callable) Future {
	if task == nil {
		panic(new_NullPointerException_0().self())
	}
	var f *future_
	this.execute(&Runnable_fn{func() { f = runTask_(task.call) }})
	return f
}

func (this *AbstractExecutorService) shutdown() { this.shutdown_ = true }

func (this *AbstractExecutorService) close() { this.shutdown_ = true }

type ThreadPoolExecutor struct {
	AbstractExecutorService
	self_ any
}

func (this *ThreadPoolExecutor) as_ThreadPoolExecutor() *ThreadPoolExecutor { return this }

func (this *ThreadPoolExecutor) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *ThreadPoolExecutor) init_ThreadPoolExecutor_int_int_long_TimeUnit_BlockingQueue_ThreadFactory(core int32, max int32, keepAlive int64, unit *TimeUnit, queue BlockingQueue, factory ThreadFactory) {
	if core < 0 || max <= 0 || max < core || keepAlive < 0 {
		panic(new_IllegalArgumentException_0().self())
	}
	if queue == nil || factory == nil {
		panic(new_NullPointerException_0().self())
	}
	this.factory_ = factory
}

func new_ThreadPoolExecutor_int_int_long_TimeUnit_BlockingQueue_ThreadFactory(core int32, max int32, keepAlive int64, unit *TimeUnit, queue BlockingQueue, factory ThreadFactory) *ThreadPoolExecutor {
	this := &ThreadPoolExecutor{}
	this.self_ = this
	this.init_ThreadPoolExecutor_int_int_long_TimeUnit_BlockingQueue_ThreadFactory(core, max, keepAlive, unit, queue, factory)
	return this
}

type Executors struct{}

func newExecutor_(factory ThreadFactory) *ThreadPoolExecutor {
	this := &ThreadPoolExecutor{}
	this.self_ = this
	this.factory_ = factory
	return this
}

func Executors_newSingleThreadExecutor(factory ThreadFactory) ExecutorService {
	if factory == nil {
		panic(new_NullPointerException_0().self())
	}
	return newExecutor_(factory)
}

// ---- Guava: MoreExecutors

type MoreExecutors struct{}

type GwtFuturesCatchingSpecialization struct{}

type Futures struct{}

// listeningExecutor_ is MoreExecutors.listeningDecorator: the futures of the
// executors here are ListenableFutures already.
type listeningExecutor_ struct {
	delegate_ ExecutorService
}

func (this *listeningExecutor_) submit(task Callable) Future { return this.delegate_.submit(task) }
func (this *listeningExecutor_) shutdown()                   { this.delegate_.shutdown() }
func (this *listeningExecutor_) execute(task Runnable) {
	if e, ok := this.delegate_.(interface{ execute(Runnable) }); ok {
		e.execute(task)
	} else {
		task.run()
	}
}

func MoreExecutors_listeningDecorator(delegate ExecutorService) ListeningExecutorService {
	if delegate == nil {
		panic(new_NullPointerException_0().self())
	}
	if l, ok := delegate.(*listeningExecutor_); ok {
		return l
	}
	return &listeningExecutor_{delegate}
}

// directExecutor_ runs each task in the calling thread.
type directExecutor_ struct{}

// ---- java.lang.management

type ManagementFactory struct{}

type MemoryUsage struct {
	init_, used_, committed_, max_ int64
}

func new_MemoryUsage(init int64, used int64, committed int64, max int64) *MemoryUsage {
	if init < -1 || max < -1 || used < 0 || committed < 0 || used > committed || max >= 0 && committed > max {
		panic(new_IllegalArgumentException_String("invalid memory usage").self())
	}
	return &MemoryUsage{init, used, committed, max}
}

func (this *MemoryUsage) getUsed() int64 { return this.used_ }
func (this *MemoryUsage) getMax() int64  { return this.max_ }

func (this *MemoryUsage) toString() string {
	k := func(n int64) string { return istr_(n) + "(" + istr_(n>>10) + "K)" }
	return "init = " + k(this.init_) + " used = " + k(this.used_) + " committed = " + k(this.committed_) + " max = " + k(this.max_)
}

func usage_(used uint64, committed uint64) *MemoryUsage {
	if committed < used {
		committed = used
	}
	return &MemoryUsage{init_: 0, used_: int64(used), committed_: int64(committed), max_: -1}
}

type memoryBean_ struct{}

func (memoryBean_) getHeapMemoryUsage() *MemoryUsage {
	m := memStats_()
	return usage_(m.HeapAlloc, m.HeapSys)
}

func (memoryBean_) getNonHeapMemoryUsage() *MemoryUsage {
	m := memStats_()
	n := m.StackSys + m.MSpanSys + m.MCacheSys + m.BuckHashSys + m.GCSys + m.OtherSys
	return usage_(m.StackInuse+m.MSpanInuse+m.MCacheInuse+m.BuckHashSys+m.GCSys+m.OtherSys, n)
}

func ManagementFactory_getMemoryMXBean() MemoryMXBean { return memoryBean_{} }

// peakHeap_ is the largest heap seen by the memory pool bean.
var peakHeap_ uint64

type heapPoolBean_ struct{}

func (heapPoolBean_) getName() string { return "Go heap" }

func (heapPoolBean_) getUsage() *MemoryUsage {
	m := memStats_()
	if m.HeapAlloc > peakHeap_ {
		peakHeap_ = m.HeapAlloc
	}
	return usage_(m.HeapAlloc, m.HeapSys)
}

func (heapPoolBean_) getPeakUsage() *MemoryUsage {
	m := memStats_()
	if m.HeapAlloc > peakHeap_ {
		peakHeap_ = m.HeapAlloc
	}
	return usage_(peakHeap_, m.HeapSys)
}

func ManagementFactory_getMemoryPoolMXBeans() List { return listOf_([]any{heapPoolBean_{}}) }

type gcBean_ struct{}

func (gcBean_) getName() string { return "Go GC" }

func (gcBean_) getCollectionCount() int64 { return int64(memStats_().NumGC) }

func (gcBean_) getCollectionTime() int64 { return int64(memStats_().PauseTotalNs / 1e6) }

func ManagementFactory_getGarbageCollectorMXBeans() List { return listOf_([]any{gcBean_{}}) }

// compilationBean_ stands for the JIT compiler: there is none.
type compilationBean_ struct{}

func (compilationBean_) getName() string { return "Go ahead-of-time compiler" }

func (compilationBean_) getTotalCompilationTime() int64 { return 0 }

func ManagementFactory_getCompilationMXBean() CompilationMXBean { return compilationBean_{} }

// ---- java.util.logging

type Level struct {
	name_  string
	value_ int32
}

var Level_OFF = &Level{"OFF", 0x7fffffff}
var Level_SEVERE = &Level{"SEVERE", 1000}
var Level_WARNING = &Level{"WARNING", 900}
var Level_INFO = &Level{"INFO", 800}
var Level_CONFIG = &Level{"CONFIG", 700}
var Level_FINE = &Level{"FINE", 500}
var Level_FINER = &Level{"FINER", 400}
var Level_FINEST = &Level{"FINEST", 300}
var Level_ALL = &Level{"ALL", -0x80000000}

var levels_ = []*Level{Level_OFF, Level_SEVERE, Level_WARNING, Level_INFO, Level_CONFIG, Level_FINE, Level_FINER, Level_FINEST, Level_ALL}

// Level_parse accepts the name of a level or an integer value.
func Level_parse(name string) *Level {
	if name == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	for _, l := range levels_ {
		if l.name_ == name {
			return l
		}
	}
	if n, err := strconv.ParseInt(name, 10, 32); err == nil {
		for _, l := range levels_ {
			if int64(l.value_) == n {
				return l
			}
		}
		return &Level{name, int32(n)}
	}
	panic(new_IllegalArgumentException_String("Bad level \"" + name + "\"").self())
}

func (this *Level) getName() string  { return this.name_ }
func (this *Level) toString() string { return this.name_ }
func (this *Level) hashCode() int32  { return this.value_ }

func (this *Level) equals(o any) bool {
	l, ok := o.(*Level)
	return ok && l != nil && l.value_ == this.value_
}

// Logger writes the records at or above its level to System.err as
// "LEVEL: message". A logger without a level of its own uses its parent's
// (the logger named by the prefix before the last dot); the root logger's
// level is WARNING.
type Logger struct {
	name_   string
	level_  *Level
	parent_ *Logger
}

var rootLogger_ = &Logger{name_: "", level_: Level_WARNING}
var loggers_ = map[string]*Logger{"": rootLogger_}

func Logger_getLogger(name string) *Logger {
	if name == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	if l := loggers_[name]; l != nil {
		return l
	}
	l := &Logger{name_: name}
	loggers_[name] = l
	if i := strings.LastIndexByte(name, '.'); i >= 0 {
		l.parent_ = Logger_getLogger(name[:i])
	} else {
		l.parent_ = rootLogger_
	}
	return l
}

func (this *Logger) getName() string   { return this.name_ }
func (this *Logger) setLevel(l *Level) { this.level_ = l }

func (this *Logger) effectiveLevel_() int32 {
	for l := this; l != nil; l = l.parent_ {
		if l.level_ != nil {
			return l.level_.value_
		}
	}
	return Level_WARNING.value_
}

func (this *Logger) isLoggable(l *Level) bool {
	limit := this.effectiveLevel_()
	return l.value_ >= limit && limit != Level_OFF.value_
}

func (this *Logger) emit_(l *Level, msg string, thrown *Throwable) {
	if !this.isLoggable(l) {
		return
	}
	printErr_(l.name_ + ": " + sstr_(msg) + "\n")
	if thrown != nil {
		printErr_(Io_stackTraceText(thrown))
	}
}

func (this *Logger) log(l *Level, msg string, thrown *Throwable) { this.emit_(l, msg, thrown) }

func (this *Logger) log_Level_String(l *Level, msg string) { this.emit_(l, msg, nil) }

// log_Level_String_Object formats the message with one parameter, {0}.
func (this *Logger) log_Level_String_Object(l *Level, msg string, param any) {
	if !this.isLoggable(l) {
		return
	}
	if msg != nullstr_ && strings.Contains(msg, "{0") {
		msg = strings.ReplaceAll(msg, "{0}", objstr_(param))
	}
	this.emit_(l, msg, nil)
}

func (this *Logger) severe(msg string)  { this.emit_(Level_SEVERE, msg, nil) }
func (this *Logger) warning(msg string) { this.emit_(Level_WARNING, msg, nil) }
func (this *Logger) info(msg string)    { this.emit_(Level_INFO, msg, nil) }
func (this *Logger) fine(msg string)    { this.emit_(Level_FINE, msg, nil) }
func (this *Logger) finest(msg string)  { this.emit_(Level_FINEST, msg, nil) }
