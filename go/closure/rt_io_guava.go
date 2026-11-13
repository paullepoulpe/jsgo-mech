package closure

// Guava: com.google.common.io (CharStreams, ByteStreams, CharSource, CharSink,
// Files) and com.google.common.cache.

import (
	"io"
	"os"
	"strconv"
	"strings"
)

// ---- CharStreams, ByteStreams

type CharStreams struct{}

func CharStreams_toString(r Readable) string { return readAll_(readerOf_(r)) }

func CharStreams_readLines(r Readable) List {
	rd := readerOf_(r)
	out := []any{}
	for {
		line := rd.readLine_()
		if line == nullstr_ {
			return listOf_(out)
		}
		out = append(out, line)
	}
}

type ByteStreams struct{}

// ByteStreams_limit is a stream of at most limit bytes of in. Closing it
// closes in.
func ByteStreams_limit(in *InputStream, limit int64) *InputStream {
	if in == nil {
		panic(new_NullPointerException_0().self())
	}
	if limit < 0 {
		panic(new_IllegalArgumentException_String("limit must be non-negative").self())
	}
	lr := &io.LimitedReader{R: inReader_{in}, N: limit}
	out := newInputStream_(lr, in.close)
	out.avail_ = func() int32 {
		n := int64(in.available())
		if n > lr.N {
			n = lr.N
		}
		return int32(n)
	}
	return out
}

// ---- CharSource, CharSink

type CharSource struct {
	load_ func() string
	desc_ string
}

// CharSource_wrap is a source of the characters of a CharSequence.
func CharSource_wrap(cs any) *CharSource {
	if cs == nil {
		panic(new_NullPointerException_0().self())
	}
	s := csString_(cs)
	return &CharSource{load_: func() string { return s }, desc_: "CharSource.wrap(...)"}
}

func (this *CharSource) read() string { return this.load_() }

func (this *CharSource) openStream() *Reader { return new_StringReader(this.load_()).up_Reader() }

func (this *CharSource) isEmpty() bool { return this.load_() == "" }

func (this *CharSource) length() int64 { return int64(String_length(this.load_())) }

func (this *CharSource) toString() string { return this.desc_ }

type CharSink struct {
	file_   *File
	cs_     *Charset
	append_ bool
}

// ---- com.google.common.io.Files

type Files struct{}

type FileWriteMode struct {
	Enum
}

var FileWriteMode_APPEND = func() *FileWriteMode {
	e := &FileWriteMode{}
	e.self_ = e
	e.init_Enum("APPEND", 0)
	return e
}()

func readFile_(f *File) []byte {
	if f == nil {
		panic(new_NullPointerException_0().self())
	}
	b, err := os.ReadFile(f.path_)
	if err != nil {
		panic(fileNotFound_(f.path_, err))
	}
	return b
}

func Files_io_asCharSource(f *File, cs *Charset) *CharSource {
	if f == nil || cs == nil {
		panic(new_NullPointerException_0().self())
	}
	return &CharSource{
		load_: func() string {
			s, _ := cs.decode_(readFile_(f))
			return s
		},
		desc_: "Files.asCharSource(" + f.path_ + ", " + cs.name_ + ")",
	}
}

func Files_io_toByteArray(f *File) []int8 { return i8_(readFile_(f)) }

func Files_io_write(b []int8, f *File) {
	if f == nil {
		panic(new_NullPointerException_0().self())
	}
	out := new_FileOutputStream(f.path_)
	defer out.close()
	out.write_byteArr(b)
}

// ---- com.google.common.cache

type RemovalNotification struct {
	AbstractMap_SimpleImmutableEntry
	evicted_ bool
}

func (this *RemovalNotification) getKey() any   { return this.key }
func (this *RemovalNotification) getValue() any { return this.val }

type CacheLoader struct {
	self_ any
}

func (this *CacheLoader) as_CacheLoader() *CacheLoader { return this }

func (this *CacheLoader) init_CacheLoader() {}

// functionCacheLoader_ is CacheLoader.from(function).
type functionCacheLoader_ struct {
	CacheLoader
	f_ Function_base
}

type CacheBuilder struct {
	maximumSize_ int64
	listener_    RemovalListener
}

func CacheBuilder_newBuilder() *CacheBuilder { return &CacheBuilder{maximumSize_: -1} }

func (this *CacheBuilder) maximumSize(n int64) *CacheBuilder {
	if this.maximumSize_ != -1 {
		panic(new_IllegalStateException_String("maximum size was already set to " + istr_(this.maximumSize_)).self())
	}
	if n < 0 {
		panic(new_IllegalArgumentException_String("maximum size must not be negative").self())
	}
	this.maximumSize_ = n
	return this
}

func (this *CacheBuilder) removalListener(l RemovalListener) *CacheBuilder {
	if this.listener_ != nil {
		panic(new_IllegalStateException_0().self())
	}
	if l == nil {
		panic(new_NullPointerException_0().self())
	}
	this.listener_ = l
	return this
}

func (this *CacheBuilder) build(loader *CacheLoader) LoadingCache {
	if loader == nil {
		panic(new_NullPointerException_0().self())
	}
	return &localCache_{map_: new_LinkedHashMap(), max_: this.maximumSize_, listener_: this.listener_, loader_: loader}
}

// localCache_ is a Cache / LoadingCache. Its map is kept in order of use, the
// least recently used entry first; that one is evicted when the size bound is
// exceeded.
type localCache_ struct {
	map_      *LinkedHashMap
	max_      int64
	listener_ RemovalListener
	loader_   *CacheLoader
}

func (this *localCache_) notify_(key any, value any, evicted bool) {
	if this.listener_ != nil {
		n := &RemovalNotification{evicted_: evicted}
		n.key, n.val = key, value
		this.listener_.onRemoval(n)
	}
}

func (this *localCache_) getIfPresent(key any) any {
	if key == nil {
		panic(new_NullPointerException_0().self())
	}
	v := this.map_.get(key)
	if v != nil {
		// Move the entry to the most recently used end.
		this.map_.remove(key)
		this.map_.put(key, v)
	}
	return v
}

func (this *localCache_) put(key any, value any) {
	if key == nil || value == nil {
		panic(new_NullPointerException_0().self())
	}
	old := this.map_.remove(key)
	this.map_.put(key, value)
	if old != nil {
		this.notify_(key, old, false)
	}
	for this.max_ >= 0 && int64(this.map_.size()) > this.max_ {
		eldest := this.map_.keySet().iterator().next()
		v := this.map_.remove(eldest)
		this.notify_(eldest, v, true)
	}
}

// load_ runs the loader; what it throws is wrapped the way Guava does it
// for getUnchecked (unchecked=true) or get.
func (this *localCache_) load_(key any, loader func() any, unchecked bool) any {
	var v any
	func() {
		defer func() {
			if r := recover(); r != nil {
				ex := caught_(r)
				t := to_Throwable(ex)
				if _, isError := ex.(interface{ as_Error() *Error }); isError {
					panic(ex) // Guava's ExecutionError; the Error itself is rethrown here
				}
				_, isRuntime := ex.(interface{ as_RuntimeException() *RuntimeException })
				if unchecked || isRuntime {
					panic(new_UncheckedExecutionException_Throwable(t).self())
				}
				panic(new_ExecutionException_Throwable(t).self())
			}
		}()
		v = loader()
	}()
	if v == nil {
		// com.google.common.cache.CacheLoader.InvalidCacheLoadException
		panic(new_RuntimeException_String("CacheLoader returned null for key " + objstr_(key) + ".").self())
	}
	this.put(key, v)
	return v
}

func (this *localCache_) loaderFunc_(key any) func() any {
	if this.loader_ == nil {
		panic(new_UnsupportedOperationException_0().self())
	}
	l := this.loader_.self_.(interface{ load(any) any })
	return func() any { return l.load(key) }
}

func (this *localCache_) get(key any) any {
	if v := this.getIfPresent(key); v != nil {
		return v
	}
	return this.load_(key, this.loaderFunc_(key), false)
}

func (this *localCache_) getUnchecked(key any) any {
	if v := this.getIfPresent(key); v != nil {
		return v
	}
	return this.load_(key, this.loaderFunc_(key), true)
}

func (this *localCache_) apply(key any) any { return this.getUnchecked(key) }

func (this *localCache_) size() int64 { return int64(this.map_.size()) }

func (this *localCache_) cleanUp() {}

func (this *localCache_) asMap() ConcurrentMap { return this.map_ }

// write replaces (or, in append mode, extends) the file with the characters.
func (this *CharSink) write(cs any) {
	if cs == nil {
		panic(new_NullPointerException_0().self())
	}
	out := new_FileOutputStream_String_boolean(this.file_.path_, this.append_)
	defer out.close()
	out.write_byteArr(String_getBytes(csString_(cs), this.cs_))
}

func Files_io_asCharSink(f *File, cs *Charset, modes []*FileWriteMode) *CharSink {
	if f == nil || cs == nil {
		panic(new_NullPointerException_0().self())
	}
	sink := &CharSink{file_: f, cs_: cs}
	for _, m := range modes {
		if m == FileWriteMode_APPEND {
			sink.append_ = true
		}
	}
	return sink
}

func Files_io_newReader(f *File, cs *Charset) *BufferedReader {
	if f == nil || cs == nil {
		panic(new_NullPointerException_0().self())
	}
	return new_BufferedReader(new_InputStreamReader(new_FileInputStream(f.path_).up_InputStream(), cs).up_Reader())
}

func CacheLoader_from(f Function_base) *CacheLoader {
	if f == nil {
		panic(new_NullPointerException_0().self())
	}
	l := &functionCacheLoader_{f_: f}
	l.self_ = l
	return &l.CacheLoader
}

// CacheBuilder_from reads a cache specification ("maximumSize=100,..."). Only
// the size bound has an effect here; time-based expiry, weak references,
// statistics and the concurrency level are accepted and ignored.
func CacheBuilder_from(spec string) *CacheBuilder {
	b := CacheBuilder_newBuilder()
	for _, kv := range strings.Split(spec, ",") {
		kv = strings.TrimSpace(kv)
		if kv == "" {
			continue
		}
		key, value, hasValue := strings.Cut(kv, "=")
		key, value = strings.TrimSpace(key), strings.TrimSpace(value)
		switch key {
		case "maximumSize", "maximumWeight":
			n, err := strconv.ParseInt(value, 10, 64)
			if !hasValue || err != nil {
				panic(new_IllegalArgumentException_String("key " + key + " value set to " + value + ", must be integer").self())
			}
			if key == "maximumSize" {
				b.maximumSize(n)
			}
		case "initialCapacity", "concurrencyLevel":
			if _, err := strconv.ParseInt(value, 10, 32); !hasValue || err != nil {
				panic(new_IllegalArgumentException_String("key " + key + " value set to " + value + ", must be integer").self())
			}
		case "weakKeys", "softValues", "weakValues", "recordStats":
			if hasValue {
				panic(new_IllegalArgumentException_String("key " + key + " does not take values").self())
			}
		case "expireAfterAccess", "expireAfterWrite", "refreshAfterWrite":
			if !hasValue || value == "" {
				panic(new_IllegalArgumentException_String("value of key " + key + " omitted").self())
			}
		default:
			panic(new_IllegalArgumentException_String("unknown key " + key).self())
		}
	}
	return b
}

func (this *functionCacheLoader_) load(key any) any {
	if key == nil {
		panic(new_NullPointerException_0().self())
	}
	return this.f_.apply(key)
}
