package closure

import (
	"strings"
	"testing"
)

// The expected strings in gsonGolden_ were produced by running the same
// operations against the real Gson 2.9.1 (see testdata/gson/G.java in the
// porting notes); each case below mirrors one case there.

func gsonTry_(f func() string) (out string) {
	defer func() {
		if r := recover(); r != nil {
			out = "EXC " + r.(interface{ toString() string }).toString()
		}
	}()
	return f()
}

func gsonW_(f func(w *JsonWriter)) func() string {
	return func() string {
		var sb strings.Builder
		w := newJsonWriterSB_(&sb)
		f(w)
		return sb.String()
	}
}

func gsonAllChars_() string {
	var u []uint16
	for i := 0; i < 0x80; i++ {
		u = append(u, uint16(i))
	}
	u = append(u, 0x2028, 0x2029, 0xe9, 0x20ac, 0xD83D, 0xDE00, 0x7f, 0x80, 0x2027, 0x202a)
	return fromUnits_(u)
}

type gsonTestInner_ struct {
	a     string
	boxed any
	nul   any
}

type gsonTestP_ struct {
	name_   string
	count   int32
	big     int64
	d       float64
	z       float64
	flag    bool
	missing string
	list    List
	map_    Map
	ch      uint16
	f       float32
	arr     []int32
	sarr    []string
	inner   *gsonTestInner_
	noInner *gsonTestInner_
	skipped int32
}

func gsonNewP_() *gsonTestP_ {
	gsonTransient_["gsonTestP_"] = map[string]bool{"skipped": true}
	negZero := 0.0
	negZero = -negZero
	return &gsonTestP_{name_: "n<&>", count: 3, big: 1 << 40, d: 1.0e10, z: negZero, flag: true, missing: nullstr_,
		list: listOf_([]any{"a", int32(1), 2.5, nil, int64(7)}), map_: new_LinkedHashMap(), ch: 'x', f: 1.1,
		arr: []int32{1, 2}, sarr: []string{"q", nullstr_}, inner: &gsonTestInner_{a: "='", boxed: 0.1}, skipped: 9}
}

func gsonSample_() *LinkedHashMap {
	negZero := 0.0
	negZero = -negZero
	m := new_LinkedHashMap()
	m.put("s", "<a href='x'>&=</a>\u2028\n\t\"\\")
	m.put("i", int32(42))
	m.put("l", int64(9007199254740993))
	m.put("d", 1.0e10)
	m.put("d2", 0.1)
	m.put("d3", 100.0)
	m.put("d4", 1.0e-5)
	m.put("d5", negZero)
	m.put("d6", 1.23456789012e8)
	m.put("d8", 1.7976931348623157e308)
	m.put("d9", 1234567.0)
	m.put("d10", 1.0e7)
	m.put("d11", 0.001)
	m.put("d12", 9.999e-4)
	m.put("b", true)
	m.put("n", nil)
	m.put("list", listOf_([]any{"x", int32(1), nil, new_ArrayList_0(), new_LinkedHashMap()}))
	inner := new_LinkedHashMap()
	inner.put("k", "v")
	inner.put("nul", nil)
	inner.put("e", new_ArrayList_0())
	m.put("obj", inner)
	m.put("emptyObj", new_LinkedHashMap())
	return m
}

const gsonDoc_ = "{\"version\":3,\"file\":\"a.js\",\"n\":[1,1.0E10,1e3,0.10,-0,12345678901234567890,1.5,true,false,null,\"s<>\"],\"o\":{\"a\":null,\"b\":{},\"c\":[]},\"u\":\"\\u00e9\\n\\/\\ud83d\\ude00\"}"

var gsonJsonObjectClass_ = classOf_("com.google.gson.JsonObject")
var gsonJsonArrayClass_ = classOf_("com.google.gson.JsonArray")
var gsonJsonElementClass_ = classOf_("com.google.gson.JsonElement")

func gsonElemStr_(v any) string {
	if v == nil {
		return "null"
	}
	return objstr_(v)
}

type gsonTestAble_ struct{ v any }

func (this *gsonTestAble_) toLogsGson() any { return this.v }

// TestGsonWriterSink checks writing through an Appendable/Writer-like object.
func TestGsonWriterSink(t *testing.T) {
	sb := new_StringBuilder()
	w := newJsonWriterFunc_(gsonWriterSink_(sb))
	w.setIndent(" ")
	w.beginObject().name("a").value("b").endObject()
	w.flush()
	if got := sb.toString(); got != "{\n \"a\": \"b\"\n}" {
		t.Errorf("got %q", got)
	}
}

// TestGsonReaderPull checks reading through a character source.
func TestGsonReaderPull(t *testing.T) {
	src := String_toCharArray("[\"\u00e9\U0001F600\", 2]  ")
	i := 0
	closed := false
	r := newJsonReaderFunc_(func() int32 {
		if i >= len(src) {
			return -1
		}
		i++
		return int32(src[i-1])
	}, func() { closed = true })
	r.beginArray()
	if s := r.nextString(); s != "\u00e9\U0001F600" {
		t.Errorf("got %q", s)
	}
	if n := r.nextLong(); n != 2 {
		t.Errorf("got %d", n)
	}
	r.endArray()
	if r.peek() != JsonToken_END_DOCUMENT {
		t.Errorf("not at end")
	}
	r.close()
	if !closed {
		t.Errorf("not closed")
	}
}
