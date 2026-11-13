package closure

// com.google.gson.JsonElement and its subclasses (Gson 2.9.1 behaviour).
//
// Every element is handled through its embedded *JsonElement (what
// up_JsonElement() returns); self_ leads back to the concrete object.

import (
	"math"
	"math/big"
	"strings"
)

type JsonElement struct {
	self_ any // *JsonObject, *JsonArray, *JsonPrimitive or *JsonNull
}

type JsonObject struct {
	JsonElement
	members_ *LinkedHashMap // String -> *JsonElement
}

type JsonArray struct {
	JsonElement
	elements_ *ArrayList // of *JsonElement
}

type JsonPrimitive struct {
	JsonElement
	value_ any // string, bool, or a number (int32, int64, float64, gsonLazyNumber_)
}

type JsonNull struct {
	JsonElement
}

// gsonLazyNumber_ is com.google.gson.internal.LazilyParsedNumber: a number
// kept as the text it was read from.
type gsonLazyNumber_ string

func (n gsonLazyNumber_) toString() string { return string(n) }

func (n gsonLazyNumber_) longValue_() int64 {
	if v, ok := gsonTryParseLong_(string(n)); ok {
		return v
	}
	return gsonBigDecimalLong_(string(n))
}

func (n gsonLazyNumber_) intValue_() int32 {
	return int32(n.longValue_())
}

func (n gsonLazyNumber_) doubleValue_() float64 { return Double_parseDouble(string(n)) }

// gsonBigDecimalLong_ is new BigDecimal(s).longValue(): the integer part,
// reduced modulo 2^64.
func gsonBigDecimalLong_(s string) int64 {
	bad := func() any { return new_NumberFormatException_String(s).self() }
	mant := s
	exp := 0
	if i := strings.IndexAny(s, "eE"); i >= 0 {
		mant = s[:i]
		e, ok := gsonTryParseLong_(s[i+1:])
		if !ok || e > 100000 || e < -100000 {
			panic(bad())
		}
		exp = int(e)
	}
	neg := false
	if strings.HasPrefix(mant, "-") || strings.HasPrefix(mant, "+") {
		neg = mant[0] == '-'
		mant = mant[1:]
	}
	digits := mant
	if i := strings.IndexByte(mant, '.'); i >= 0 {
		digits = mant[:i] + mant[i+1:]
		exp -= len(mant) - i - 1
	}
	if digits == "" {
		panic(bad())
	}
	for i := 0; i < len(digits); i++ {
		if digits[i] < '0' || digits[i] > '9' {
			panic(bad())
		}
	}
	v, _ := new(big.Int).SetString(digits, 10)
	if exp > 0 {
		v.Mul(v, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(exp)), nil))
	} else if exp < 0 {
		v.Quo(v, new(big.Int).Exp(big.NewInt(10), big.NewInt(int64(-exp)), nil))
	}
	if neg {
		v.Neg(v)
	}
	mod := new(big.Int).Lsh(big.NewInt(1), 64)
	v.Mod(v, mod) // result in [0, 2^64)
	return int64(v.Uint64())
}

func (this *JsonObject) up_JsonElement() *JsonElement {
	if this == nil {
		return nil
	}
	return &this.JsonElement
}

func (this *JsonArray) up_JsonElement() *JsonElement {
	if this == nil {
		return nil
	}
	return &this.JsonElement
}

func (this *JsonPrimitive) up_JsonElement() *JsonElement {
	if this == nil {
		return nil
	}
	return &this.JsonElement
}

func (this *JsonNull) up_JsonElement() *JsonElement {
	if this == nil {
		return nil
	}
	return &this.JsonElement
}

func new_JsonObject() *JsonObject {
	this := &JsonObject{members_: new_LinkedHashMap()}
	this.self_ = this
	return this
}

func new_JsonArray() *JsonArray {
	this := &JsonArray{elements_: new_ArrayList_0()}
	this.self_ = this
	return this
}

func newJsonPrimitive_(v any) *JsonPrimitive {
	this := &JsonPrimitive{value_: v}
	this.self_ = this
	return this
}

func new_JsonPrimitive(p0 string) *JsonPrimitive {
	if p0 == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	return newJsonPrimitive_(p0)
}

func newJsonNull_() *JsonNull {
	this := &JsonNull{}
	this.self_ = this
	return this
}

var JsonNull_INSTANCE = newJsonNull_()

// ---- JsonElement

func (this *JsonElement) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *JsonElement) isJsonArray() bool {
	_, ok := this.self_.(*JsonArray)
	return ok
}

func (this *JsonElement) isJsonObject() bool {
	_, ok := this.self_.(*JsonObject)
	return ok
}

func (this *JsonElement) isJsonPrimitive() bool {
	_, ok := this.self_.(*JsonPrimitive)
	return ok
}

func (this *JsonElement) isJsonNull() bool {
	_, ok := this.self_.(*JsonNull)
	return ok
}

func (this *JsonElement) getAsJsonObject() *JsonObject {
	if o, ok := this.self_.(*JsonObject); ok {
		return o
	}
	panic(new_IllegalStateException_String("Not a JSON Object: " + this.toString()).self())
}

func (this *JsonElement) getAsJsonArray() *JsonArray {
	if o, ok := this.self_.(*JsonArray); ok {
		return o
	}
	panic(new_IllegalStateException_String("Not a JSON Array: " + this.toString()).self())
}

func (this *JsonElement) getAsJsonPrimitive() *JsonPrimitive {
	if o, ok := this.self_.(*JsonPrimitive); ok {
		return o
	}
	panic(new_IllegalStateException_String("Not a JSON Primitive: " + this.toString()).self())
}

func (this *JsonElement) getAsJsonNull() *JsonNull {
	if o, ok := this.self_.(*JsonNull); ok {
		return o
	}
	panic(new_IllegalStateException_String("Not a JSON Null: " + this.toString()).self())
}

// single_ is the element a getAsXxx call on this element delegates to.
func (this *JsonElement) single_() *JsonPrimitive {
	switch x := this.self_.(type) {
	case *JsonPrimitive:
		return x
	case *JsonArray:
		if x.elements_.size() == 1 {
			return cast_[*JsonElement](x.elements_.get(0)).single_()
		}
		panic(new_IllegalStateException_0().self())
	case *JsonObject:
		panic(new_UnsupportedOperationException_String("JsonObject").self())
	case *JsonNull:
		panic(new_UnsupportedOperationException_String("JsonNull").self())
	}
	panic(new_NullPointerException_0().self())
}

func (this *JsonElement) getAsString() string {
	p := this.single_()
	switch v := p.value_.(type) {
	case string:
		return v
	case bool:
		return boolstr_(v)
	}
	return gsonNumberString_(p.value_)
}

func (this *JsonElement) getAsBoolean() bool {
	p := this.single_()
	if b, ok := p.value_.(bool); ok {
		return b
	}
	return strings.EqualFold(this.getAsString(), "true")
}

func (this *JsonElement) getAsInt() int32 {
	p := this.single_()
	switch v := p.value_.(type) {
	case string:
		return Integer_parseInt_String(v)
	case bool:
		return Integer_parseInt_String(boolstr_(v))
	case int32:
		return v
	case int64:
		return int32(v)
	case int16:
		return int32(v)
	case int8:
		return int32(v)
	case float64:
		return f2i_[int32](v)
	case float32:
		return f2i_[int32](float64(v))
	case gsonLazyNumber_:
		return v.intValue_()
	}
	panic(new_NumberFormatException_String(objstr_(p.value_)).self())
}

func (this *JsonElement) getAsLong() int64 {
	p := this.single_()
	switch v := p.value_.(type) {
	case string:
		return Long_parseLong_String(v)
	case bool:
		return Long_parseLong_String(boolstr_(v))
	case int32:
		return int64(v)
	case int64:
		return v
	case int16:
		return int64(v)
	case int8:
		return int64(v)
	case float64:
		return f2i_[int64](v)
	case float32:
		return f2i_[int64](float64(v))
	case gsonLazyNumber_:
		return v.longValue_()
	}
	panic(new_NumberFormatException_String(objstr_(p.value_)).self())
}

func (this *JsonElement) getAsDouble() float64 {
	p := this.single_()
	switch v := p.value_.(type) {
	case string:
		return Double_parseDouble(v)
	case bool:
		return Double_parseDouble(boolstr_(v))
	}
	return gsonNumDouble_(p.value_)
}

func gsonNumDouble_(n any) float64 {
	switch v := n.(type) {
	case int32:
		return float64(v)
	case int64:
		return float64(v)
	case int16:
		return float64(v)
	case int8:
		return float64(v)
	case float64:
		return v
	case float32:
		return float64(v)
	case gsonLazyNumber_:
		return v.doubleValue_()
	}
	panic(new_NumberFormatException_String(objstr_(n)).self())
}

func gsonIsIntegral_(v any) bool {
	switch v.(type) {
	case int32, int64, int16, int8:
		return true
	}
	return false
}

func gsonIsNumber_(v any) bool {
	switch v.(type) {
	case int32, int64, int16, int8, float64, float32, gsonLazyNumber_:
		return true
	}
	return false
}

func gsonIntegralLong_(v any) int64 {
	switch x := v.(type) {
	case int32:
		return int64(x)
	case int64:
		return x
	case int16:
		return int64(x)
	case int8:
		return int64(x)
	}
	return 0
}

// toString is the compact JSON text (nulls included, no HTML escaping).
func (this *JsonElement) toString() string {
	var sb strings.Builder
	w := newJsonWriterSB_(&sb)
	w.setLenient(true)
	gsonWriteElement_(w, this)
	return sb.String()
}

func (this *JsonElement) equals(o any) bool {
	var other *JsonElement
	switch x := o.(type) {
	case *JsonElement:
		other = x
	case interface{ up_JsonElement() *JsonElement }:
		other = x.up_JsonElement()
	}
	if other == nil {
		return false
	}
	if this == other || this.self_ == other.self_ {
		return true
	}
	switch a := this.self_.(type) {
	case *JsonNull:
		return other.isJsonNull()
	case *JsonObject:
		b, ok := other.self_.(*JsonObject)
		return ok && a.members_.equals(b.members_)
	case *JsonArray:
		b, ok := other.self_.(*JsonArray)
		return ok && a.elements_.equals(b.elements_)
	case *JsonPrimitive:
		b, ok := other.self_.(*JsonPrimitive)
		if !ok {
			return false
		}
		if gsonIsIntegral_(a.value_) && gsonIsIntegral_(b.value_) {
			return gsonIntegralLong_(a.value_) == gsonIntegralLong_(b.value_)
		}
		if gsonIsNumber_(a.value_) && gsonIsNumber_(b.value_) {
			x, y := gsonNumDouble_(a.value_), gsonNumDouble_(b.value_)
			return x == y || (math.IsNaN(x) && math.IsNaN(y))
		}
		return objequals_(a.value_, b.value_)
	}
	return false
}

func (this *JsonElement) hashCode() int32 {
	switch a := this.self_.(type) {
	case *JsonNull:
		return objhash_("com.google.gson.JsonNull")
	case *JsonObject:
		return a.members_.hashCode()
	case *JsonArray:
		return a.elements_.hashCode()
	case *JsonPrimitive:
		if gsonIsIntegral_(a.value_) {
			v := uint64(gsonIntegralLong_(a.value_))
			return int32(uint32(v ^ (v >> 32)))
		}
		if gsonIsNumber_(a.value_) {
			v := math.Float64bits(gsonNumDouble_(a.value_))
			return int32(uint32(v ^ (v >> 32)))
		}
		return objhash_(a.value_)
	}
	return 0
}

// The concrete classes answer equals/hashCode/toString through the element.
func (this *JsonObject) equals(o any) bool    { return this.JsonElement.equals(o) }
func (this *JsonArray) equals(o any) bool     { return this.JsonElement.equals(o) }
func (this *JsonPrimitive) equals(o any) bool { return this.JsonElement.equals(o) }
func (this *JsonNull) equals(o any) bool      { return this.JsonElement.equals(o) }

// ---- JsonPrimitive

func (this *JsonPrimitive) isString() bool {
	_, ok := this.value_.(string)
	return ok
}

func (this *JsonPrimitive) isBoolean() bool {
	_, ok := this.value_.(bool)
	return ok
}

func (this *JsonPrimitive) isNumber() bool { return gsonIsNumber_(this.value_) }

// ---- JsonObject

func gsonOrNull_(e *JsonElement) *JsonElement {
	if e == nil {
		return &JsonNull_INSTANCE.JsonElement
	}
	return e
}

func (this *JsonObject) add(p0 string, p1 *JsonElement) {
	this.members_.put(strobj_(p0), gsonOrNull_(p1))
}

func (this *JsonObject) addProperty(p0 string, p1 string) {
	if p1 == nullstr_ {
		this.add(p0, nil)
	} else {
		this.add(p0, new_JsonPrimitive(p1).up_JsonElement())
	}
}

func (this *JsonObject) remove(p0 string) *JsonElement {
	return cast_[*JsonElement](this.members_.remove(strobj_(p0)))
}

func (this *JsonObject) entrySet() Set { return this.members_.entrySet() }

func (this *JsonObject) keySet() Set { return this.members_.keySet() }

func (this *JsonObject) size() int32 { return this.members_.size() }

func (this *JsonObject) has(p0 string) bool { return this.members_.containsKey(strobj_(p0)) }

func (this *JsonObject) get(p0 string) *JsonElement {
	return cast_[*JsonElement](this.members_.get(strobj_(p0)))
}

func (this *JsonObject) getAsJsonPrimitive_String(p0 string) *JsonPrimitive {
	e := this.get(p0)
	if e == nil {
		return nil
	}
	return e.self_.(*JsonPrimitive)
}

func (this *JsonObject) getAsJsonArray_String(p0 string) *JsonArray {
	e := this.get(p0)
	if e == nil {
		return nil
	}
	return e.self_.(*JsonArray)
}

func (this *JsonObject) getAsJsonObject_String(p0 string) *JsonObject {
	e := this.get(p0)
	if e == nil {
		return nil
	}
	return e.self_.(*JsonObject)
}

// ---- JsonArray

func (this *JsonArray) add(p0 *JsonElement) {
	this.elements_.add(gsonOrNull_(p0))
}

func (this *JsonArray) add_String(p0 string) {
	if p0 == nullstr_ {
		this.add(nil)
	} else {
		this.add(new_JsonPrimitive(p0).up_JsonElement())
	}
}

func (this *JsonArray) get(p0 int32) *JsonElement {
	return cast_[*JsonElement](this.elements_.get(p0))
}

func (this *JsonArray) iterator() Iterator { return this.elements_.iterator() }

func (this *JsonArray) size() int32 { return this.elements_.size() }

func (this *JsonArray) isEmpty() bool { return this.elements_.size() == 0 }

// ---- reading and writing trees (TypeAdapters.JSON_ELEMENT)

func gsonWriteElement_(w *JsonWriter, e *JsonElement) {
	if e == nil {
		w.nullValue()
		return
	}
	switch x := e.self_.(type) {
	case *JsonNull:
		w.nullValue()
	case *JsonPrimitive:
		switch v := x.value_.(type) {
		case string:
			w.value(v)
		case bool:
			w.value_boolean(v)
		default:
			w.value_Number(v)
		}
	case *JsonArray:
		w.beginArray()
		for _, c := range x.elements_.items {
			gsonWriteElement_(w, c.(*JsonElement))
		}
		w.endArray()
	case *JsonObject:
		w.beginObject()
		for it := x.members_.entrySet().iterator(); it.hasNext(); {
			en := it.next().(Map_Entry)
			w.name(str_(en.getKey()))
			gsonWriteElement_(w, en.getValue().(*JsonElement))
		}
		w.endObject()
	default:
		panic(new_IllegalArgumentException_String("Couldn't write " + goTypeName_(e.self_)).self())
	}
}

func gsonReadElement_(r *JsonReader) *JsonElement {
	switch r.peek() {
	case JsonToken_STRING:
		return newJsonPrimitive_(r.nextString()).up_JsonElement()
	case JsonToken_NUMBER:
		return newJsonPrimitive_(gsonLazyNumber_(r.nextString())).up_JsonElement()
	case JsonToken_BOOLEAN:
		return newJsonPrimitive_(r.nextBoolean()).up_JsonElement()
	case JsonToken_NULL:
		r.nextNull()
		return &JsonNull_INSTANCE.JsonElement
	case JsonToken_BEGIN_ARRAY:
		a := new_JsonArray()
		r.beginArray()
		for r.hasNext() {
			a.add(gsonReadElement_(r))
		}
		r.endArray()
		return a.up_JsonElement()
	case JsonToken_BEGIN_OBJECT:
		o := new_JsonObject()
		r.beginObject()
		for r.hasNext() {
			name := r.nextName()
			o.add(name, gsonReadElement_(r))
		}
		r.endObject()
		return o.up_JsonElement()
	}
	panic(new_IllegalArgumentException_0().self())
}

// JsonParser

type JsonParser struct{}

func JsonParser_parseReader_JsonReader(r *JsonReader) *JsonElement {
	return gsonParseReader_(r, false)
}

func gsonParseReader_(r *JsonReader, full bool) *JsonElement {
	lenient := r.isLenient()
	r.setLenient(true)
	defer r.setLenient(lenient)
	var e *JsonElement
	func() {
		isEmpty := true
		defer func() {
			if x := recover(); x != nil {
				switch ex := x.(type) {
				case *gsonEOFException_:
					if isEmpty {
						e = &JsonNull_INSTANCE.JsonElement
						return
					}
					panic(new_JsonSyntaxException_Throwable(to_Throwable(ex)).self())
				case *MalformedJsonException:
					panic(new_JsonSyntaxException_Throwable(to_Throwable(ex)).self())
				case *NumberFormatException:
					panic(new_JsonSyntaxException_Throwable(to_Throwable(ex)).self())
				}
				if is_IOException(x) {
					panic(new_JsonIOException_Throwable(to_Throwable(x)).self())
				}
				panic(x)
			}
		}()
		r.peek()
		isEmpty = false
		e = gsonReadElement_(r)
	}()
	if full && !e.isJsonNull() {
		tok := func() (t *JsonToken) {
			defer func() {
				if x := recover(); x != nil {
					if m, ok := x.(*MalformedJsonException); ok {
						panic(new_JsonSyntaxException_Throwable(to_Throwable(m)).self())
					}
					panic(x)
				}
			}()
			return r.peek()
		}()
		if tok != JsonToken_END_DOCUMENT {
			panic(new_JsonSyntaxException_String("Did not consume the entire document.").self())
		}
	}
	return e
}

func (this *JsonElement) getAsNumber() any {
	p := this.single_()
	if s, ok := p.value_.(string); ok {
		return gsonLazyNumber_(s)
	}
	if gsonIsNumber_(p.value_) {
		return p.value_
	}
	panic(new_UnsupportedOperationException_String("Primitive is neither a number nor a string").self())
}
