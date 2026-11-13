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

func gsonCases_() map[string]func() string {
	cases := map[string]func() string{}
	nan := nan_
	cases["w_chars"] = gsonW_(func(w *JsonWriter) { w.beginArray(); w.value(gsonAllChars_()); w.endArray() })
	cases["w_chars_html"] = gsonW_(func(w *JsonWriter) {
		w.setHtmlSafe(true)
		w.beginObject()
		w.name(gsonAllChars_()).value(gsonAllChars_())
		w.endObject()
	})
	cases["w_numbers"] = gsonW_(func(w *JsonWriter) {
		negZero := 0.0
		negZero = -negZero
		w.beginArray()
		w.value_long(0)
		w.value_long(-9223372036854775808)
		w.value_long(9223372036854775807)
		w.value_double(1.0e10)
		w.value_double(negZero)
		w.value_double(1.0e-5)
		w.value_double(100.0)
		w.value_double(1.23456789012e8)
		w.value_double(1.7976931348623157e308)
		w.value_Number(float32(1.1))
		w.value_Number(int32(3))
		w.value_boolean(true)
		w.value_boolean(false)
		w.value(nullstr_)
		w.nullValue()
		w.endArray()
	})
	cases["w_indent"] = gsonW_(func(w *JsonWriter) {
		w.setIndent("  ")
		w.beginObject()
		w.name("a").beginArray().endArray()
		w.name("b").beginObject().endObject()
		w.name("c").beginArray().value_long(1).beginArray().value("x").endArray().beginObject().name("k").nullValue().endObject().endArray()
		w.name("d").value("e")
		w.endObject()
	})
	cases["w_indent_tab"] = gsonW_(func(w *JsonWriter) {
		w.setIndent("\t")
		w.beginArray()
		w.beginObject()
		w.name("a").value_long(1)
		w.name("b").jsonValue("{\"raw\": 1}")
		w.endObject()
		w.endArray()
	})
	cases["w_nonulls"] = gsonW_(func(w *JsonWriter) {
		w.setSerializeNulls(false)
		w.beginObject()
		w.name("a").nullValue()
		w.name("b").value(nullstr_)
		w.name("c").beginArray().nullValue().endArray()
		w.name("d").jsonValue(nullstr_)
		w.endObject()
	})
	cases["w_lenient"] = gsonW_(func(w *JsonWriter) {
		w.setLenient(true)
		w.value_double(nan)
		w.value_double(neginf_)
		w.value("x")
		w.beginArray().endArray()
	})
	cases["w_err_nan"] = gsonW_(func(w *JsonWriter) { w.value_double(nan) })
	cases["w_err_two"] = gsonW_(func(w *JsonWriter) { w.value_long(1); w.value_long(2) })
	cases["w_err_name"] = gsonW_(func(w *JsonWriter) { w.beginArray(); w.name("a"); w.value_long(1) })
	cases["w_err_dangling"] = gsonW_(func(w *JsonWriter) { w.beginObject(); w.name("a"); w.endObject() })
	cases["w_err_close"] = gsonW_(func(w *JsonWriter) { w.beginObject(); w.close() })
	cases["w_err_nest"] = gsonW_(func(w *JsonWriter) { w.beginObject(); w.endArray() })
	cases["w_err_noname"] = gsonW_(func(w *JsonWriter) { w.beginObject(); w.value_long(1) })

	cases["g_map"] = func() string { return new_Gson().toJson(gsonSample_()) }
	cases["g_map_nohtml"] = func() string { return new_GsonBuilder().disableHtmlEscaping().create().toJson(gsonSample_()) }
	cases["g_map_pretty"] = func() string { return new_GsonBuilder().setPrettyPrinting().create().toJson(gsonSample_()) }
	cases["g_map_nulls"] = func() string {
		return new_GsonBuilder().serializeNulls().setPrettyPrinting().disableHtmlEscaping().create().toJson(gsonSample_())
	}
	cases["g_null"] = func() string { return new_Gson().toJson(nil) }
	cases["g_str"] = func() string { return new_Gson().toJson("a<b") }
	cases["g_set"] = func() string {
		s := new_LinkedHashSet()
		s.add("a.js")
		s.add("b<c>.js")
		s.add("it's")
		return new_GsonBuilder().disableHtmlEscaping().create().toJson(s)
	}
	cases["g_nan"] = func() string { return new_Gson().toJson(nan) }
	cases["g_pojo"] = func() string { return new_Gson().toJson(gsonNewP_()) }
	cases["g_pojo_pretty_nulls"] = func() string {
		return new_GsonBuilder().setPrettyPrinting().serializeNulls().create().toJson(gsonNewP_())
	}
	cases["g_filespec"] = func() string {
		files := new_ArrayList_0()
		files.add(new_AbstractCommandLineRunner_JsonFileSpec_String_String_String_String("var a='<x>';\n", "out/a.js", "{\"version\":3}", "17"))
		files.add(new_AbstractCommandLineRunner_JsonFileSpec_String_String("b", "b.js"))
		var sb strings.Builder
		jw := newJsonWriterSB_(&sb)
		g := new_GsonBuilder().disableHtmlEscaping().setPrettyPrinting().create()
		g.toJson_Object_Type_JsonWriter(files, gsonTypeTokens_["AbstractCommandLineRunner_1"], jw)
		jw.close()
		return sb.String()
	}
	cases["g_writer_settings"] = func() string {
		var sb strings.Builder
		jw := newJsonWriterSB_(&sb)
		jw.beginArray()
		new_Gson().toJson_Object_Type_JsonWriter(gsonSample_(), classOf_("java.util.Map"), jw)
		jw.value("<")
		jw.endArray()
		return sb.String()
	}
	cases["r_filespec"] = func() string {
		r := newJsonReaderFromString_("[{\"src\":\"x\",\"path\":\"p\",\"source_map\":\"m\",\"webpack_id\":\"3\",\"other\":[1,{\"a\":2}]},{\"sourceMap\":\"m2\",\"webpackId\":5,\"src\":null,\"path\":true},{}]")
		var sb strings.Builder
		g := new_Gson()
		r.beginArray()
		for r.hasNext() {
			f := cast_[*AbstractCommandLineRunner_JsonFileSpec](g.fromJson_JsonReader_Type(r, classOf_("com.google.javascript.jscomp.AbstractCommandLineRunner.JsonFileSpec")))
			sb.WriteString(sstr_(f.getSrc()) + "|" + sstr_(f.getPath()) + "|" + sstr_(f.getSourceMap()) + "|" + sstr_(f.getWebpackId()) + ";")
		}
		r.endArray()
		return sb.String()
	}
	cases["t_roundtrip"] = func() string {
		return cast_[*JsonObject](new_Gson().fromJson_String_Class(gsonDoc_, gsonJsonObjectClass_)).toString()
	}
	cases["t_gson"] = func() string {
		return new_Gson().toJson(new_Gson().fromJson_String_Class(gsonDoc_, gsonJsonElementClass_))
	}
	cases["t_pretty"] = func() string {
		o := cast_[*JsonObject](new_Gson().fromJson_String_Class(gsonDoc_, gsonJsonObjectClass_))
		return new_GsonBuilder().setPrettyPrinting().create().toJson_JsonElement(o.up_JsonElement())
	}
	cases["t_access"] = func() string {
		o := cast_[*JsonObject](new_Gson().fromJson_String_Class(gsonDoc_, gsonJsonObjectClass_))
		n := o.get("n").getAsJsonArray()
		var sb strings.Builder
		sb.WriteString(istr_(int64(o.get("version").getAsInt())) + "," + boolstr_(o.has("file")) + boolstr_(o.has("nope")) + boolstr_(o.get("nope") == nil) + ",")
		for i := int32(0); i < 7; i++ {
			sb.WriteString(strings.TrimPrefix(gsonTry_(func() string { return istr_(int64(n.get(i).getAsInt())) }), "EXC ") + ",")
		}
		for it := cast_[Iterable](obj_(n)).iterator(); it.hasNext(); {
			e := cast_[*JsonElement](it.next())
			if e.isJsonPrimitive() {
				if e.getAsJsonPrimitive().isString() {
					sb.WriteString("S")
				} else {
					sb.WriteString("P")
				}
			} else if e.isJsonNull() {
				sb.WriteString("N")
			} else {
				sb.WriteString("?")
			}
			if !e.isJsonNull() {
				sb.WriteString(e.getAsString())
			}
			sb.WriteString(";")
		}
		for it := Iterable(o.entrySet()).iterator(); it.hasNext(); {
			en := cast_[Map_Entry](it.next())
			sb.WriteString(str_(en.getKey()) + "=" + objstr_(obj_(cast_[*JsonElement](en.getValue()))) + ";")
		}
		sb.WriteString(o.get("u").getAsString())
		return sb.String()
	}
	for _, t := range [][2]string{{"obj", "[1]"}, {"arr", "{}"}, {"null", "null"}, {"empty", ""}, {"ws", "  "}} {
		in := t[1]
		cases["t_cls_obj_"+t[0]] = func() string { return gsonElemStr_(new_Gson().fromJson_String_Class(in, gsonJsonObjectClass_)) }
		cases["t_cls_arr_"+t[0]] = func() string { return gsonElemStr_(new_Gson().fromJson_String_Class(in, gsonJsonArrayClass_)) }
		cases["t_cls_el_"+t[0]] = func() string { return gsonElemStr_(new_Gson().fromJson_String_Class(in, gsonJsonElementClass_)) }
	}
	cases["t_err_getarr"] = func() string {
		return cast_[*JsonObject](new_Gson().fromJson_String_Class("{\"a\":1}", gsonJsonObjectClass_)).get("a").getAsJsonArray().toString()
	}
	cases["t_err_getobj"] = func() string {
		return cast_[*JsonArray](new_Gson().fromJson_String_Class("[null]", gsonJsonArrayClass_)).get(0).getAsJsonObject().toString()
	}
	cases["t_err_getstr"] = func() string {
		return cast_[*JsonObject](new_Gson().fromJson_String_Class("{\"a\":{}}", gsonJsonObjectClass_)).get("a").getAsString()
	}
	cases["t_err_getint"] = func() string {
		return istr_(int64(cast_[*JsonObject](new_Gson().fromJson_String_Class("{\"a\":\"x\"}", gsonJsonObjectClass_)).get("a").getAsInt()))
	}
	cases["t_build"] = func() string {
		chunks := new_JsonArray()
		node := new_JsonObject()
		node.add("name", new_JsonPrimitive("m<1>").up_JsonElement())
		deps := new_JsonArray()
		node.add("dependencies", deps.up_JsonElement())
		deps.add(new_JsonPrimitive("d").up_JsonElement())
		node.add("inputs", new_JsonArray().up_JsonElement())
		node.add("x", nil)
		chunks.add(node.up_JsonElement())
		return Object_toString(obj_(chunks)) + "\n" + new_GsonBuilder().setPrettyPrinting().create().toJson_JsonElement(chunks.up_JsonElement()) + "\n" + new_Gson().toJson(chunks)
	}
	ls := gsonTypeTokens_["ReportDecoder_1"]
	ml := gsonTypeTokens_["ProductionInstrumentationReporter_1"]
	lsIn := []string{"[\"a\",1,true,null,1.50,\"\\u0041\"]", "null", "", "[", "[\"a\" \"b\"]", "{}", "[[1]]", "['a',b,]", "[\"a\"] x", "[\"a\"]]", "[\"unterminated]", "[\"bad\\q\"]", "// c\n[\"a\"/*x*/,#y\n\"b\"]", ")]}'\n[\"p\"]", "[\"a\";\"b\"]", "[1 2]", "\"str\"", "[\n \"a\",\n  x y]", "[\"a\\\nb\",\n\"c\" 1]"}
	for i, in := range lsIn {
		in := in
		cases["r_list_"+istr_(int64(i))] = func() string {
			return gsonElemStr_(new_GsonBuilder().disableHtmlEscaping().create().fromJson(in, ls))
		}
	}
	mlIn := []string{"{\"a\":1,\"b\":\"2\",\"c\":3.0,\"d\":-9223372036854775808,\"e\":1e2}", "{\"a\":1.5}", "{\"a\":1,\"a\":2}", "{\"a\":null}", "{a:1,'b'=2;c=>3}", "{\"a\":\"x\"}", "{\"a\":9223372036854775808}", "{\"a\":1", "{\"a\" 1}", "{\"a\":}", "{,}", "{\"a\":[1]}", "[[\"k\",1]]", "{\"a\":true}", "{\"a\":01}", "{\"a\":1}{", "{\"a\":NaN}"}
	for i, in := range mlIn {
		in := in
		cases["r_map_"+istr_(int64(i))] = func() string { return gsonElemStr_(new_Gson().fromJson(in, ml)) }
	}
	objIn := []string{"{\"a\":[1,2.5,\"x\",true,null,{\"b\":-0}],\"c\":1E400}", "tRuE", "NULL", "truex", "-", "1.", "\"a\\u00zz\"", "[1e5,1E+5,1e-5,-0.0,0123]"}
	for i, in := range objIn {
		in := in
		cases["r_obj_"+istr_(int64(i))] = func() string {
			o := new_Gson().fromJson_String_Class(in, classOf_("java.lang.Object"))
			return gsonElemStr_(o) + "|" + new_Gson().toJson(o)
		}
	}
	cases["r_stream"] = func() string {
		r := newJsonReaderFromString_("{\"a\":[1,2,{\"skip\":[1,{\"x\":null}]},\"s\"],\"b\":12.5,\"c\":\"7\",\"d\":null,\"e\":false,\"f\":123456789012}")
		var sb strings.Builder
		a := func(s string) { sb.WriteString(s) }
		r.beginObject()
		a(r.peek().name())
		a(r.nextName())
		r.beginArray()
		a(istr_(int64(r.nextInt())))
		a(r.nextString())
		a(r.getPath())
		r.skipValue()
		a(r.getPath())
		a(r.peek().name())
		a(r.nextString())
		a(boolstr_(r.hasNext()))
		r.endArray()
		a(r.nextName())
		a(r.peek().name())
		a(dstr_(r.nextDouble()))
		a(r.nextName())
		a(istr_(int64(r.nextInt())))
		a(r.nextName())
		r.nextNull()
		a(r.nextName())
		a(boolstr_(r.nextBoolean()))
		a(r.nextName())
		a(istr_(r.nextLong()))
		a(r.peek().name())
		r.endObject()
		a(r.peek().name())
		a(r.getPath())
		return sb.String()
	}
	strict := [][2]string{{"s0", "{'a':1}"}, {"s1", "[1,]"}, {"s2", "[1] 2"}, {"s3", "{\"a\":1}"}, {"s4", "[NaN]"}, {"s5", "[\"NaN\"]"}, {"s6", "x"}, {"s7", "[1;2]"}, {"s8", "[//\n1]"}, {"s9", "[1.5]"}, {"s10", "[\"1.0\"]"}, {"s11", "[true]"}, {"s12", "[99999999999]"}}
	for _, t := range strict {
		in := t[1]
		cases["r_strict_"+t[0]] = func() string {
			r := newJsonReaderFromString_(in)
			var sb strings.Builder
			func() {
				defer func() {
					if x := recover(); x != nil {
						sb.WriteString("EXC " + x.(interface{ toString() string }).toString())
					}
				}()
				for {
					k := r.peek()
					sb.WriteString(k.name() + " ")
					if k == JsonToken_END_DOCUMENT {
						break
					}
					switch k {
					case JsonToken_BEGIN_ARRAY:
						r.beginArray()
					case JsonToken_END_ARRAY:
						r.endArray()
					case JsonToken_BEGIN_OBJECT:
						r.beginObject()
					case JsonToken_END_OBJECT:
						r.endObject()
					case JsonToken_NAME:
						sb.WriteString(r.nextName() + " ")
					case JsonToken_NUMBER:
						sb.WriteString(istr_(int64(r.nextInt())) + " ")
					case JsonToken_STRING:
						sb.WriteString(dstr_(r.nextDouble()) + " ")
					case JsonToken_BOOLEAN:
						sb.WriteString(r.nextString())
					default:
						r.skipValue()
					}
				}
			}()
			return sb.String()
		}
	}
	return cases
}

func TestGsonGolden(t *testing.T) {
	cases := gsonCases_()
	for name, want := range gsonGolden_ {
		f, ok := cases[name]
		if !ok {
			t.Errorf("%s: no Go case", name)
			continue
		}
		if got := gsonTry_(f); got != want {
			t.Errorf("%s:\n got: %q\nwant: %q", name, got, want)
		}
	}
	for name := range cases {
		if _, ok := gsonGolden_[name]; !ok {
			t.Errorf("%s: no golden value", name)
		}
	}
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

