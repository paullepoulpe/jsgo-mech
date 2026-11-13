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
	"reflect"
	"regexp"
	"strings"
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

func (this *JsonParseException) init_JsonParseException_String(p0 string) {
	this.RuntimeException.init_RuntimeException_String(p0)
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

func (this *TypeAdapter) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *TypeAdapter) init_TypeAdapter() {}

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
