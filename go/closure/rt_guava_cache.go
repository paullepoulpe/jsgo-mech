package closure

// Guava: com.google.common.cache.

import (
	"strconv"
	"strings"
)

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

func (this *CacheBuilder) build(loader *CacheLoader) LoadingCache {
	if loader == nil {
		panic(new_NullPointerException_0().self())
	}
	return &localCache_{map_: new_LinkedHashMap(), max_: this.maximumSize_, loader_: loader}
}

// localCache_ is a Cache / LoadingCache. Its map is kept in order of use, the
// least recently used entry first; that one is evicted when the size bound is
// exceeded.
type localCache_ struct {
	map_    *LinkedHashMap
	max_    int64
	loader_ *CacheLoader
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
	this.map_.remove(key)
	this.map_.put(key, value)
	for this.max_ >= 0 && int64(this.map_.size()) > this.max_ {
		this.map_.remove(this.map_.keySet().iterator().next())
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
