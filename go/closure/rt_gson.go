package closure

// com.google.gson.Gson, GsonBuilder, TypeToken, TypeAdapter and the exception
// classes (Gson 2.9.1 behaviour).
//
// Java Gson converts arbitrary classes by reflection. Here:
//   - writing walks the Go struct of the translated class with reflect, in
//     field order (which is the Java declaration order), subclass fields
//     before superclass fields, as Gson does;
//   - reading needs to know how to create an object from a Java class name
//     and what the type arguments of a generic type are, neither of which the
//     Go code carries, so the tables below list them (gsonPojoCtors_,
//     gsonTypeTokens_, gsonFieldTypes_).

import (
	"math"
	"reflect"
	"regexp"
	"strings"
	"unsafe"
)

// ---- exceptions

type JsonParseException struct {
	RuntimeException
}

func (this *JsonParseException) as_JsonParseException() *JsonParseException { return this }

func to_JsonParseException(v any) *JsonParseException {
	if v == nil {
		return nil
	}
	return v.(interface{ as_JsonParseException() *JsonParseException }).as_JsonParseException()
}

func is_JsonParseException(v any) bool {
	_, ok := v.(interface{ as_JsonParseException() *JsonParseException })
	return ok
}

func (this *JsonParseException) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *JsonParseException) up_RuntimeException() *RuntimeException {
	if this == nil {
		return nil
	}
	return &this.RuntimeException
}

func (this *JsonParseException) init_JsonParseException_String(p0 string) {
	this.RuntimeException.init_RuntimeException_String(p0)
}

func (this *JsonParseException) init_JsonParseException_String_Throwable(p0 string, p1 *Throwable) {
	this.RuntimeException.init_RuntimeException_String_Throwable(p0, p1)
}

func (this *JsonParseException) init_JsonParseException_Throwable(p0 *Throwable) {
	this.RuntimeException.init_RuntimeException_Throwable(p0)
}

type JsonSyntaxException struct {
	JsonParseException
}

func new_JsonSyntaxException_String(p0 string) *JsonSyntaxException {
	this := &JsonSyntaxException{}
	this.self_ = this
	this.init_JsonParseException_String(p0)
	return this
}

func new_JsonSyntaxException_Throwable(p0 *Throwable) *JsonSyntaxException {
	this := &JsonSyntaxException{}
	this.self_ = this
	this.init_JsonParseException_Throwable(p0)
	return this
}

type JsonIOException struct {
	JsonParseException
}

func new_JsonIOException_Throwable(p0 *Throwable) *JsonIOException {
	this := &JsonIOException{}
	this.self_ = this
	this.init_JsonParseException_Throwable(p0)
	return this
}

func init() {
	javaNames_["JsonSyntaxException"] = "com.google.gson.JsonSyntaxException"
	javaNames_["JsonIOException"] = "com.google.gson.JsonIOException"
	javaNames_["JsonParseException"] = "com.google.gson.JsonParseException"
}

// ---- java.lang.reflect.Type, TypeToken, TypeAdapter

// gsonParamType_ is a java.lang.reflect.ParameterizedType: raw is the Java
// name of the generic class, args its type arguments (*Class or
// *gsonParamType_).
type gsonParamType_ struct {
	raw  string
	args []Type
}

func (this *gsonParamType_) getRawType() Type { return classOf_(this.raw) }

func (this *gsonParamType_) getActualTypeArguments() []Type {
	return append([]Type(nil), this.args...)
}

func (this *gsonParamType_) getTypeName() string { return this.toString() }

func (this *gsonParamType_) toString() string {
	var sb strings.Builder
	sb.WriteString(this.raw)
	sb.WriteByte('<')
	for i, a := range this.args {
		if i > 0 {
			sb.WriteString(", ")
		}
		switch x := a.(type) {
		case *Class:
			sb.WriteString(x.getName())
		default:
			sb.WriteString(objstr_(a))
		}
	}
	sb.WriteByte('>')
	return sb.String()
}

func (this *gsonParamType_) equals(o any) bool {
	p, ok := o.(*gsonParamType_)
	return ok && p.toString() == this.toString()
}

func (this *gsonParamType_) hashCode() int32 { return objhash_(this.toString()) }

func gsonParam_(raw string, args ...Type) Type { return &gsonParamType_{raw: raw, args: args} }

// gsonTypeTokens_ gives, for each anonymous `new TypeToken<T>() {}` subclass in
// the translated code (by Go type name), the type T that Java reads from the
// class's generic signature.
var gsonTypeTokens_ = map[string]Type{
	// new TypeToken<List<JsChunkSpec>>() {}
	"AbstractCommandLineRunner_1": gsonParam_("java.util.List", classOf_("com.google.javascript.jscomp.AbstractCommandLineRunner.JsChunkSpec")),
	// new TypeToken<List<String>>() {}
	"ReportDecoder_1": gsonParam_("java.util.List", classOf_("java.lang.String")),
	// new TypeToken<Map<String, Long>>() {}
	"ProductionInstrumentationReporter_1": gsonParam_("java.util.Map", classOf_("java.lang.String"), classOf_("java.lang.Long")),
}

type TypeToken struct {
	self_ any
	type_ Type
}

func (this *TypeToken) as_TypeToken() *TypeToken { return this }

func (this *TypeToken) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *TypeToken) init_TypeToken_0() {}

func (this *TypeToken) getType() Type {
	if this.type_ != nil {
		return this.type_
	}
	n := goTypeName_(this.self_)
	t, ok := gsonTypeTokens_[n]
	if !ok {
		panic(new_IllegalStateException_String("TypeToken subclass " + n + " has no entry in gsonTypeTokens_ (rt_gson.go)").self())
	}
	this.type_ = t
	return t
}

func (this *TypeToken) toString() string { return objstr_(this.getType()) }

type TypeAdapter struct {
	self_ any
}

func (this *TypeAdapter) as_TypeAdapter() *TypeAdapter { return this }

func (this *TypeAdapter) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *TypeAdapter) init_TypeAdapter() {}

// toJson is TypeAdapter.toJson(T): the JSON text the subclass's write produces.
func (this *TypeAdapter) toJson(value any) string {
	var sb strings.Builder
	w := newJsonWriterSB_(&sb)
	this.self_.(interface{ write(*JsonWriter, any) }).write(w, value)
	return sb.String()
}

// fromJson is TypeAdapter.fromJson(String).
func (this *TypeAdapter) fromJson(json string) any {
	return this.self_.(interface{ read(*JsonReader) any }).read(newJsonReaderFromString_(json))
}

// ---- tables for reflective conversion

// gsonPojoCtors_ creates an instance of a class Gson deserializes by
// reflection, keyed by Java canonical name ('.' for nested classes), in the
// state Java's no-argument constructor leaves it.
var gsonPojoCtors_ = map[string]func() any{}

type gsonRename_ struct {
	name       string
	alternates []string
}

// gsonSerializedNames_ holds the @SerializedName annotations:
// Go type name -> Java field name -> JSON names.
var gsonSerializedNames_ = map[string]map[string]gsonRename_{
	"AbstractCommandLineRunner_JsonFileSpec": {
		"sourceMap": {"source_map", []string{"sourceMap"}},
		"webpackId": {"webpack_id", []string{"webpackId"}},
	},
}

// gsonTransient_ lists Java fields declared `transient` (never converted):
// Go type name -> Java field name.
var gsonTransient_ = map[string]map[string]bool{
	"JSChunk":      {"inputs": true},
	"JSChunkGraph": {"dependencyMap": true},
	"Node": {"token": true, "next": true, "previous": true, "first": true, "parent": true, "linenoCharno": true,
		"length": true, "jstypeOrColor": true, "originalName": true, "propListHead": true},
	"NamedType":                {"validator": true, "propertyContinuations": true},
	"JSTypeRegistry":           {"reporter": true, "closureNamespaces": true, "forwardDeclaredTypes": true, "nonRefTypesIndexedByProperty": true},
	"CompilerOptions":          {"customPasses": true, "outputCharset": true, "errorHandler": true},
	"DefaultPassConfig":        {"preprocessorSymbolTableFactory": true},
	"LazyParsedDependencyInfo": {"compiler": true},
	"TypeValidator":            {"compiler": true},
}

// gsonFieldTypes_ gives the generic type of collection-typed fields of classes
// that are deserialized: Go type name -> Java field name -> type. Fields not
// listed are read as their raw type (List -> ArrayList<Object>, ...).
var gsonFieldTypes_ = map[string]map[string]Type{}

// gsonInstanceOf_ decides `value instanceof X` for the classes and interfaces
// registered with GsonBuilder.registerTypeHierarchyAdapter, by Java name.
var gsonInstanceOf_ = map[string]func(v any) bool{
	"com.google.javascript.jscomp.diagnostic.LogsGson.Able": func(v any) bool {
		_, ok := v.(interface{ toLogsGson() any })
		return ok
	},
	"com.google.common.collect.Multimap": func(v any) bool {
		_, ok := v.(interface {
			asMap() Map
			containsEntry(any, any) bool
		})
		return ok
	},
}

// ---- GsonBuilder and Gson

type gsonHierarchyAdapter_ struct {
	class   string // canonical Java name
	adapter any
}

type GsonBuilder struct {
	htmlSafe_       bool
	pretty_         bool
	serializeNulls_ bool
	lenient_        bool
	specialFloats_  bool
	longAsString_   bool
	hierarchy_      []gsonHierarchyAdapter_
}

type Gson struct {
	htmlSafe_       bool
	pretty_         bool
	serializeNulls_ bool
	lenient_        bool
	specialFloats_  bool
	longAsString_   bool
	hierarchy_      []gsonHierarchyAdapter_
}

var gsonAnonymousRE_ = regexp.MustCompile(`\$[0-9]+$`)

// ---- reflection over translated classes

type gsonFieldInfo_ struct {
	index      []int // reflect field index path
	name       string
	alternates []string
	javaName   string
}

var gsonFieldCache_ = map[reflect.Type][]gsonFieldInfo_{}

// ---- reading

var gsonObjectType_ Type = classOf_("java.lang.Object")

func (this *JsonSyntaxException) up_JsonParseException() *JsonParseException {
	if this == nil {
		return nil
	}
	return &this.JsonParseException
}

func (this *JsonIOException) up_JsonParseException() *JsonParseException {
	if this == nil {
		return nil
	}
	return &this.JsonParseException
}

func (this *GsonBuilder) disableHtmlEscaping() *GsonBuilder {
	this.htmlSafe_ = false
	return this
}

func (this *GsonBuilder) setPrettyPrinting() *GsonBuilder {
	this.pretty_ = true
	return this
}

func (this *GsonBuilder) serializeNulls() *GsonBuilder {
	this.serializeNulls_ = true
	return this
}

func (this *GsonBuilder) setLenient() *GsonBuilder {
	this.lenient_ = true
	return this
}

func (this *GsonBuilder) serializeSpecialFloatingPointValues() *GsonBuilder {
	this.specialFloats_ = true
	return this
}

func (this *GsonBuilder) registerTypeHierarchyAdapter(p0 *Class, p1 any) *GsonBuilder {
	this.hierarchy_ = append(this.hierarchy_, gsonHierarchyAdapter_{gsonCanon_(p0.getName()), p1})
	return this
}

func (this *GsonBuilder) create() *Gson {
	g := &Gson{
		htmlSafe_:       this.htmlSafe_,
		pretty_:         this.pretty_,
		serializeNulls_: this.serializeNulls_,
		lenient_:        this.lenient_,
		specialFloats_:  this.specialFloats_,
		longAsString_:   this.longAsString_,
	}
	// Gson consults the adapters registered last first.
	for i := len(this.hierarchy_) - 1; i >= 0; i-- {
		g.hierarchy_ = append(g.hierarchy_, this.hierarchy_[i])
	}
	return g
}

func (this *Gson) toString() string {
	return "{serializeNulls:" + boolstr_(this.serializeNulls_) + "}"
}

// newJsonWriter_ is Gson.newJsonWriter: a writer with this Gson's settings.
func (this *Gson) newJsonWriter_(sb *strings.Builder) *JsonWriter {
	w := newJsonWriterSB_(sb)
	if this.pretty_ {
		w.setIndent("  ")
	}
	w.setHtmlSafe(this.htmlSafe_)
	w.setLenient(this.lenient_)
	w.setSerializeNulls(this.serializeNulls_)
	return w
}

// withWriterSettings_ runs f with the writer temporarily lenient and set to
// this Gson's htmlSafe / serializeNulls, as Gson.toJson(…, JsonWriter) does.
func (this *Gson) withWriterSettings_(w *JsonWriter, f func()) {
	oldLenient, oldHtml, oldNulls := w.lenient_, w.htmlSafe_, w.serializeNulls_
	w.lenient_ = true
	w.htmlSafe_ = this.htmlSafe_
	w.serializeNulls_ = this.serializeNulls_
	defer func() {
		w.lenient_, w.htmlSafe_, w.serializeNulls_ = oldLenient, oldHtml, oldNulls
	}()
	f()
}

func (this *Gson) toJson(p0 any) string {
	var sb strings.Builder
	w := this.newJsonWriter_(&sb)
	this.withWriterSettings_(w, func() { this.write_(w, p0) })
	return sb.String()
}

func (this *Gson) toJson_JsonElement(p0 *JsonElement) string {
	var sb strings.Builder
	w := this.newJsonWriter_(&sb)
	this.withWriterSettings_(w, func() { gsonWriteElement_(w, p0) })
	return sb.String()
}

func (this *Gson) toJson_Object_Type_JsonWriter(p0 any, p1 Type, p2 *JsonWriter) {
	// The declared type only matters to Java Gson where the runtime class of a
	// value is not more specific; every value here knows its class.
	this.withWriterSettings_(p2, func() { this.write_(p2, p0) })
}

func (this *Gson) checkDouble_(d float64) {
	if !this.specialFloats_ && (math.IsNaN(d) || math.IsInf(d, 0)) {
		panic(new_IllegalArgumentException_String(Double_toString(d) + " is not a valid double value as per JSON specification. To override this behavior, use GsonBuilder.serializeSpecialFloatingPointValues() method.").self())
	}
}

// write_ writes any value with the adapter Gson picks for its runtime class.
func (this *Gson) write_(w *JsonWriter, v any) {
	if gsonIsNilValue_(v) {
		w.nullValue()
		return
	}
	switch x := v.(type) {
	case *JsonElement:
		gsonWriteElement_(w, x)
		return
	case interface{ up_JsonElement() *JsonElement }:
		gsonWriteElement_(w, x.up_JsonElement())
		return
	}
	// Excluder: instances of anonymous and local classes are written as null.
	rt := reflect.TypeOf(v)
	if rt.Kind() == reflect.Pointer && rt.Elem().Kind() == reflect.Struct {
		if j, ok := javaNames_[rt.Elem().Name()]; ok && gsonAnonymousRE_.MatchString(j) {
			if _, isEnum := v.(interface{ up_Enum() *Enum }); !isEnum {
				w.nullValue()
				return
			}
		}
	}
	for _, h := range this.hierarchy_ {
		if gsonIsInstance_(v, h.class) {
			ad := h.adapter
			if s, ok := ad.(interface{ self() any }); ok && s.self() != nil {
				ad = s.self()
			}
			ad.(interface{ write(*JsonWriter, any) }).write(w, v)
			return
		}
	}
	switch x := v.(type) {
	case string:
		w.value(x)
	case bool:
		w.value_boolean(x)
	case int32:
		w.value_long(int64(x))
	case int64:
		if this.longAsString_ {
			w.value(Long_toString(x))
		} else {
			w.value_long(x)
		}
	case int16:
		w.value_long(int64(x))
	case int8:
		w.value_long(int64(x))
	case uint16:
		w.value(chrstr_(x))
	case float64:
		this.checkDouble_(x)
		w.value_Number(x)
	case float32:
		this.checkDouble_(float64(x))
		w.value_Number(x)
	case gsonLazyNumber_:
		w.value_Number(x)
	case *StringBuilder:
		w.value(x.toString())
	case *Class:
		panic(new_UnsupportedOperationException_String("Attempted to serialize java.lang.Class: " + x.getName() + ". Forgot to register a type adapter?").self())
	case interface{ up_Enum() *Enum }:
		w.value(x.up_Enum().name())
	case interface {
		entrySet() Set
		get(any) any
		containsKey(any) bool
	}:
		w.beginObject()
		for it := x.entrySet().iterator(); it.hasNext(); {
			en := it.next().(Map_Entry)
			w.name(objstr_(en.getKey()))
			this.write_(w, en.getValue())
		}
		w.endObject()
	case interface {
		iterator() Iterator
		size() int32
		contains(any) bool
	}:
		w.beginArray()
		for it := x.iterator(); it.hasNext(); {
			this.write_(w, it.next())
		}
		w.endArray()
	case []any:
		w.beginArray()
		for _, e := range x {
			this.write_(w, e)
		}
		w.endArray()
	default:
		this.writeReflect_(w, v)
	}
}

func (this *Gson) writeReflect_(w *JsonWriter, v any) {
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Slice, reflect.Array:
		w.beginArray()
		for i := 0; i < rv.Len(); i++ {
			this.write_(w, gsonBox_(rv.Index(i)))
		}
		w.endArray()
		return
	case reflect.Pointer:
		if rv.Elem().Kind() == reflect.Struct {
			obj := rv.Elem()
			w.beginObject()
			for _, f := range gsonFields_(obj.Type()) {
				w.name(f.name)
				this.write_(w, gsonBox_(gsonFieldValue_(obj, f.index)))
			}
			w.endObject()
			return
		}
	}
	panic(new_UnsupportedOperationException_String("Gson: cannot serialize Go value of type " + rv.Type().String()).self())
}

func (this *Gson) fromJson(p0 string, p1 Type) any {
	if p0 == nullstr_ {
		return nil
	}
	r := newJsonReaderFromString_(p0)
	r.setLenient(this.lenient_)
	object := this.fromJson_JsonReader_Type(r, p1)
	if object != nil {
		func() {
			defer func() {
				if x := recover(); x != nil {
					if m, ok := x.(*MalformedJsonException); ok {
						panic(new_JsonSyntaxException_Throwable(to_Throwable(m)).self())
					}
					if is_IOException(x) {
						panic(new_JsonIOException_Throwable(to_Throwable(x)).self())
					}
					panic(x)
				}
			}()
			if r.peek() != JsonToken_END_DOCUMENT {
				panic(new_JsonSyntaxException_String("JSON document was not fully consumed.").self())
			}
		}()
	}
	return object
}

func (this *Gson) fromJson_String_Class(p0 string, p1 *Class) any {
	return this.fromJson(p0, p1)
}

func (this *Gson) fromJson_JsonReader_Type(p0 *JsonReader, p1 Type) (result any) {
	isEmpty := true
	oldLenient := p0.isLenient()
	p0.setLenient(true)
	defer func() {
		p0.setLenient(oldLenient)
		if x := recover(); x != nil {
			if e, ok := x.(*gsonEOFException_); ok {
				if isEmpty {
					result = nil
					return
				}
				panic(new_JsonSyntaxException_Throwable(to_Throwable(e)).self())
			}
			if _, ok := x.(interface {
				as_IllegalStateException() *IllegalStateException
			}); ok {
				panic(new_JsonSyntaxException_Throwable(to_Throwable(x)).self())
			}
			if is_IOException(x) {
				panic(new_JsonSyntaxException_Throwable(to_Throwable(x)).self())
			}
			panic(x)
		}
	}()
	p0.peek()
	isEmpty = false
	return this.read_(p0, p1)
}

// read_ reads a value of the given Java type.
func (this *Gson) read_(r *JsonReader, t Type) any {
	switch tt := t.(type) {
	case *gsonParamType_:
		arg := func(i int) Type {
			if i < len(tt.args) {
				return tt.args[i]
			}
			return gsonObjectType_
		}
		switch tt.raw {
		case "java.util.List", "java.util.Collection", "java.lang.Iterable", "java.util.ArrayList":
			return this.readCollection_(r, arg(0), false)
		case "java.util.Set", "java.util.LinkedHashSet", "java.util.HashSet":
			return this.readCollection_(r, arg(0), true)
		case "java.util.Map", "java.util.LinkedHashMap", "java.util.HashMap":
			return this.readMap_(r, arg(0), arg(1))
		}
		return this.read_(r, classOf_(tt.raw))
	case *Class:
		name := gsonCanon_(tt.getName())
		for _, h := range this.hierarchy_ {
			if h.class == name {
				ad := h.adapter
				if s, ok := ad.(interface{ self() any }); ok && s.self() != nil {
					ad = s.self()
				}
				return ad.(interface{ read(*JsonReader) any }).read(r)
			}
		}
		switch name {
		case "com.google.gson.JsonElement", "com.google.gson.JsonObject", "com.google.gson.JsonArray",
			"com.google.gson.JsonPrimitive", "com.google.gson.JsonNull":
			e := gsonReadElement_(r)
			return gsonElementAs_(r, e, name)
		case "java.lang.String":
			return gsonReadString_(r)
		case "java.lang.Integer", "int":
			return gsonReadInt_(r)
		case "java.lang.Long", "long":
			return gsonReadLong_(r)
		case "java.lang.Double", "double", "java.lang.Number":
			return gsonReadDouble_(r)
		case "java.lang.Boolean", "boolean":
			return gsonReadBoolean_(r)
		case "java.lang.Object":
			return gsonReadObject_(r)
		case "java.util.List", "java.util.Collection", "java.lang.Iterable", "java.util.ArrayList":
			return this.readCollection_(r, gsonObjectType_, false)
		case "java.util.Set", "java.util.LinkedHashSet", "java.util.HashSet":
			return this.readCollection_(r, gsonObjectType_, true)
		case "java.util.Map", "java.util.LinkedHashMap", "java.util.HashMap":
			return this.readMap_(r, gsonObjectType_, gsonObjectType_)
		}
		if ctor, ok := gsonPojoCtors_[name]; ok {
			if gsonReadNull_(r) {
				return nil
			}
			obj := ctor()
			this.readStruct_(r, reflect.ValueOf(obj).Elem())
			return obj
		}
		panic(new_UnsupportedOperationException_String("Gson: no way to create " + name + "; add it to gsonPojoCtors_ (rt_gson.go)").self())
	}
	panic(new_UnsupportedOperationException_String("Gson: unsupported type " + objstr_(t)).self())
}

func (this *Gson) readCollection_(r *JsonReader, elem Type, set bool) any {
	if gsonReadNull_(r) {
		return nil
	}
	var add func(any) bool
	var result any
	if set {
		s := new_LinkedHashSet()
		add, result = s.add, s
	} else {
		l := new_ArrayList_0()
		add, result = l.add, l
	}
	r.beginArray()
	for r.hasNext() {
		add(this.read_(r, elem))
	}
	r.endArray()
	return result
}

func (this *Gson) readMap_(r *JsonReader, keyType Type, valueType Type) any {
	tok := r.peek()
	if tok == JsonToken_NULL {
		r.nextNull()
		return nil
	}
	m := new_LinkedHashMap()
	put := func(k any, v any) {
		if m.containsKey(k) {
			panic(new_JsonSyntaxException_String("duplicate key: " + objstr_(k)).self())
		}
		m.put(k, v)
	}
	if tok == JsonToken_BEGIN_ARRAY {
		r.beginArray()
		for r.hasNext() {
			r.beginArray()
			k := this.read_(r, keyType)
			v := this.read_(r, valueType)
			put(k, v)
			r.endArray()
		}
		r.endArray()
	} else {
		r.beginObject()
		for r.hasNext() {
			r.promoteNameToValue_()
			k := this.read_(r, keyType)
			v := this.read_(r, valueType)
			put(k, v)
		}
		r.endObject()
	}
	return m
}

// readStruct_ is ReflectiveTypeAdapterFactory.Adapter.read for a translated
// class; obj is the addressable struct.
func (this *Gson) readStruct_(r *JsonReader, obj reflect.Value) {
	fields := gsonFields_(obj.Type())
	byName := map[string]*gsonFieldInfo_{}
	for i := range fields {
		f := &fields[i]
		byName[f.name] = f
		for _, a := range f.alternates {
			byName[a] = f
		}
	}
	func() {
		defer func() {
			if x := recover(); x != nil {
				if _, ok := x.(interface {
					as_IllegalStateException() *IllegalStateException
				}); ok {
					panic(new_JsonSyntaxException_Throwable(to_Throwable(x)).self())
				}
				panic(x)
			}
		}()
		r.beginObject()
		for r.hasNext() {
			name := r.nextName()
			f := byName[name]
			if f == nil {
				r.skipValue()
				continue
			}
			this.readField_(r, obj, f)
		}
	}()
	r.endObject()
}

func (this *Gson) readField_(r *JsonReader, obj reflect.Value, f *gsonFieldInfo_) {
	fv := gsonFieldValue_(obj, f.index)
	ft := fv.Type()
	declared := gsonFieldTypes_[obj.Type().Name()][f.javaName]
	switch ft.Kind() {
	case reflect.String:
		if v := gsonReadString_(r); v == nil {
			fv.SetString(nullstr_)
		} else {
			fv.SetString(v.(string))
		}
	case reflect.Bool:
		if v := gsonReadBoolean_(r); v != nil {
			fv.SetBool(v.(bool))
		}
	case reflect.Int32, reflect.Int16, reflect.Int8:
		if v := gsonReadInt_(r); v != nil {
			fv.SetInt(int64(v.(int32)))
		}
	case reflect.Int64:
		if v := gsonReadLong_(r); v != nil {
			fv.SetInt(v.(int64))
		}
	case reflect.Float64, reflect.Float32:
		if v := gsonReadDouble_(r); v != nil {
			fv.SetFloat(v.(float64))
		}
	case reflect.Uint16:
		if v := gsonReadString_(r); v != nil {
			u := String_toCharArray(v.(string))
			if len(u) != 1 {
				panic(new_JsonSyntaxException_String("Expecting character, got: " + v.(string)).self())
			}
			fv.SetUint(uint64(u[0]))
		}
	case reflect.Pointer:
		var v any
		if declared != nil {
			v = this.read_(r, declared)
		} else if j, ok := javaNames_[ft.Elem().Name()]; ok {
			if _, known := gsonPojoCtors_[gsonCanon_(j)]; known || strings.HasPrefix(j, "com.google.gson.") {
				v = this.read_(r, classOf_(gsonCanon_(j)))
			} else if gsonReadNull_(r) {
				v = nil
			} else if ft.Elem().Kind() == reflect.Struct {
				// A class with no registered constructor: a zero struct whose
				// String fields are null.
				p := reflect.New(ft.Elem())
				gsonNullStrings_(p.Elem())
				this.readStruct_(r, p.Elem())
				v = p.Interface()
			}
		} else {
			panic(new_UnsupportedOperationException_String("Gson: cannot deserialize field " + f.javaName + " of Go type " + ft.String()).self())
		}
		gsonSet_(fv, v, f)
	case reflect.Interface:
		var v any
		if declared != nil {
			v = this.read_(r, declared)
		} else {
			switch ft.Name() {
			case "List", "Collection", "Iterable":
				v = this.readCollection_(r, gsonObjectType_, false)
			case "Set":
				v = this.readCollection_(r, gsonObjectType_, true)
			case "Map":
				v = this.readMap_(r, gsonObjectType_, gsonObjectType_)
			default:
				v = gsonReadObject_(r)
			}
		}
		gsonSet_(fv, v, f)
	default:
		panic(new_UnsupportedOperationException_String("Gson: cannot deserialize field " + f.javaName + " of Go type " + ft.String()).self())
	}
}

func gsonCanon_(javaName string) string { return strings.ReplaceAll(javaName, "$", ".") }

func gsonIsNilValue_(v any) bool {
	if v == nil {
		return true
	}
	rv := reflect.ValueOf(v)
	switch rv.Kind() {
	case reflect.Pointer, reflect.Slice, reflect.Map, reflect.Interface, reflect.Func:
		return rv.IsNil()
	}
	return false
}

func gsonIsInstance_(v any, class string) bool {
	if f, ok := gsonInstanceOf_[class]; ok {
		return f(v)
	}
	return gsonCanon_(gsonJavaNameOf_(v)) == class
}

// gsonFields_ lists the fields Gson binds for a struct type: the class's own
// fields in declaration order, then those of its superclasses (embedded
// structs), without synthetic and transient fields.
func gsonFields_(t reflect.Type) []gsonFieldInfo_ {
	if fs, ok := gsonFieldCache_[t]; ok {
		return fs
	}
	var out []gsonFieldInfo_
	seen := map[string]bool{}
	var walk func(t reflect.Type, prefix []int)
	walk = func(t reflect.Type, prefix []int) {
		renames := gsonSerializedNames_[t.Name()]
		transient := gsonTransient_[t.Name()]
		var supers []int
		for i := 0; i < t.NumField(); i++ {
			f := t.Field(i)
			if f.Anonymous {
				if f.Type.Kind() == reflect.Struct {
					supers = append(supers, i)
				}
				continue
			}
			if f.Name == "self_" || f.Name == "outer_" || strings.HasPrefix(f.Name, "cap_") {
				continue
			}
			if f.Type.Kind() == reflect.Func || f.Type.Kind() == reflect.Chan {
				continue
			}
			javaName := strings.TrimSuffix(f.Name, "_")
			if transient[javaName] {
				continue
			}
			info := gsonFieldInfo_{index: append(append([]int(nil), prefix...), i), name: javaName, javaName: javaName}
			if r, ok := renames[javaName]; ok {
				info.name = r.name
				info.alternates = r.alternates
			}
			if seen[info.name] {
				continue
			}
			seen[info.name] = true
			out = append(out, info)
		}
		for _, i := range supers {
			walk(t.Field(i).Type, append(append([]int(nil), prefix...), i))
		}
	}
	walk(t, nil)
	gsonFieldCache_[t] = out
	return out
}

// gsonFieldValue_ is a settable view of a (possibly unexported) struct field.
// obj must be addressable. This is the one place that goes around Go's
// export rules.
func gsonFieldValue_(obj reflect.Value, index []int) reflect.Value {
	f := obj.FieldByIndex(index)
	return reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem()
}

// gsonBox_ turns a field or array slot into the `any` form of the data model
// (null String -> nil, nil pointer -> nil).
func gsonBox_(f reflect.Value) any {
	switch f.Kind() {
	case reflect.String:
		s := f.String()
		if s == nullstr_ {
			return nil
		}
		return s
	case reflect.Pointer, reflect.Interface, reflect.Slice, reflect.Map:
		if f.IsNil() {
			return nil
		}
	}
	return f.Interface()
}

func gsonReadNull_(r *JsonReader) bool {
	if r.peek() == JsonToken_NULL {
		r.nextNull()
		return true
	}
	return false
}

// gsonReadString_ is TypeAdapters.STRING.read.
func gsonReadString_(r *JsonReader) any {
	switch r.peek() {
	case JsonToken_NULL:
		r.nextNull()
		return nil
	case JsonToken_BOOLEAN:
		return boolstr_(r.nextBoolean())
	}
	return r.nextString()
}

// gsonReadObject_ is ObjectTypeAdapter.read: the natural Java value of any JSON.
func gsonReadObject_(r *JsonReader) any {
	switch r.peek() {
	case JsonToken_BEGIN_ARRAY:
		list := new_ArrayList_0()
		r.beginArray()
		for r.hasNext() {
			list.add(gsonReadObject_(r))
		}
		r.endArray()
		return list
	case JsonToken_BEGIN_OBJECT:
		m := new_LinkedHashMap()
		r.beginObject()
		for r.hasNext() {
			name := r.nextName()
			m.put(strobj_(name), gsonReadObject_(r))
		}
		r.endObject()
		return m
	case JsonToken_STRING:
		return r.nextString()
	case JsonToken_NUMBER:
		return r.nextDouble()
	case JsonToken_BOOLEAN:
		return r.nextBoolean()
	case JsonToken_NULL:
		r.nextNull()
		return nil
	}
	panic(new_IllegalStateException_0().self())
}

func gsonReadBoolean_(r *JsonReader) any {
	switch r.peek() {
	case JsonToken_NULL:
		r.nextNull()
		return nil
	case JsonToken_STRING:
		return strings.EqualFold(r.nextString(), "true")
	}
	return r.nextBoolean()
}

func gsonReadInt_(r *JsonReader) any {
	if gsonReadNull_(r) {
		return nil
	}
	var v int32
	gsonWrapNFE_(func() { v = r.nextInt() })
	return v
}

func gsonReadLong_(r *JsonReader) any {
	if gsonReadNull_(r) {
		return nil
	}
	var v int64
	gsonWrapNFE_(func() { v = r.nextLong() })
	return v
}

func gsonReadDouble_(r *JsonReader) any {
	if gsonReadNull_(r) {
		return nil
	}
	return r.nextDouble()
}

// gsonElementAs_ returns the element as the requested JsonElement class, in
// the form the translated code casts to.
func gsonElementAs_(r *JsonReader, e *JsonElement, class string) any {
	switch class {
	case "com.google.gson.JsonElement":
		return e
	case "com.google.gson.JsonObject":
		if o, ok := e.self_.(*JsonObject); ok {
			return o
		}
	case "com.google.gson.JsonArray":
		if o, ok := e.self_.(*JsonArray); ok {
			return o
		}
	case "com.google.gson.JsonPrimitive":
		if o, ok := e.self_.(*JsonPrimitive); ok {
			return o
		}
	case "com.google.gson.JsonNull":
		if o, ok := e.self_.(*JsonNull); ok {
			return o
		}
	}
	panic(new_JsonSyntaxException_String("Expected a " + class + " but was " + gsonJavaNameOf_(e.self_) + "; at path " + r.getPreviousPath()).self())
}

func gsonSet_(fv reflect.Value, v any, f *gsonFieldInfo_) {
	if v == nil {
		fv.Set(reflect.Zero(fv.Type()))
		return
	}
	rv := reflect.ValueOf(v)
	if !rv.Type().AssignableTo(fv.Type()) {
		panic(new_IllegalArgumentException_String("Gson: cannot set field " + f.javaName + " (" + fv.Type().String() + ") to a " + rv.Type().String()).self())
	}
	fv.Set(rv)
}

// gsonNullStrings_ makes every String field of a fresh struct null.
func gsonNullStrings_(obj reflect.Value) {
	for i := 0; i < obj.NumField(); i++ {
		f := obj.Field(i)
		switch f.Kind() {
		case reflect.String:
			reflect.NewAt(f.Type(), unsafe.Pointer(f.UnsafeAddr())).Elem().SetString(nullstr_)
		case reflect.Struct:
			gsonNullStrings_(f)
		}
	}
}

func gsonJavaNameOf_(v any) string {
	n := goTypeName_(v)
	if j, ok := javaNames_[n]; ok {
		return j
	}
	return n
}

// gsonWrapNFE_ runs f, turning a NumberFormatException into a
// JsonSyntaxException as Gson's number adapters do.
func gsonWrapNFE_(f func()) {
	defer func() {
		if x := recover(); x != nil {
			if e, ok := x.(*NumberFormatException); ok {
				panic(new_JsonSyntaxException_Throwable(to_Throwable(e)).self())
			}
			panic(x)
		}
	}()
	f()
}
