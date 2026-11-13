import com.google.gson.*;
import com.google.gson.reflect.TypeToken;
import com.google.gson.stream.*;
import com.google.javascript.jscomp.AbstractCommandLineRunner;
import java.io.*;
import java.lang.reflect.Type;
import java.util.*;
import java.nio.charset.StandardCharsets;

public class G {
  interface Body { String run() throws Exception; }
  static void c(String name, Body b) {
    String out;
    try { out = b.run(); } catch (Throwable e) { out = "EXC " + e.toString(); }
    System.out.println(name + "\t" + Base64.getEncoder().encodeToString(out.getBytes(StandardCharsets.UTF_8)));
  }
  interface W { void run(JsonWriter w) throws Exception; }
  static String w(W f) throws Exception { StringWriter sw = new StringWriter(); JsonWriter w = new JsonWriter(sw); f.run(w); return sw.toString(); }
  static String allChars() { StringBuilder sb = new StringBuilder(); for (int i = 0; i < 0x80; i++) sb.append((char) i); sb.append("\u2028\u2029\u00e9\u20ac\uD83D\uDE00\u007f\u0080\u2027\u202a"); return sb.toString(); }
  static class P { String name = "n<&>"; int count = 3; long big = 1L << 40; double d = 1.0E10; double z = -0.0; boolean flag = true; String missing = null; List<Object> list = new ArrayList<>(Arrays.asList("a", 1, 2.5, null, 7L)); Map<String, Object> map = new LinkedHashMap<>(); char ch = 'x'; float f = 1.1f; int[] arr = {1, 2}; String[] sarr = {"q", null}; Inner inner = new Inner(); Inner noInner = null; transient int skipped = 9; static int st = 4; }
  static class Inner { String a = "='"; Double boxed = 0.1; Integer nul = null; }
  static Map<String, Object> sample() {
    Map<String, Object> m = new LinkedHashMap<>();
    m.put("s", "<a href='x'>&=</a>\u2028\n\t\"\\");
    m.put("i", 42); m.put("l", 9007199254740993L); m.put("d", 1.0E10); m.put("d2", 0.1); m.put("d3", 100.0); m.put("d4", 1.0E-5); m.put("d5", -0.0); m.put("d6", 1.23456789012E8); m.put("d8", Double.MAX_VALUE); m.put("d9", 1234567.0); m.put("d10", 1.0E7); m.put("d11", 0.001); m.put("d12", 9.999E-4);
    m.put("b", true); m.put("n", null);
    m.put("list", new ArrayList<Object>(Arrays.asList("x", 1, null, new ArrayList<Object>(), new LinkedHashMap<String, Object>())));
    Map<String, Object> inner = new LinkedHashMap<>(); inner.put("k", "v"); inner.put("nul", null); inner.put("e", new ArrayList<Object>());
    m.put("obj", inner);
    m.put("emptyObj", new LinkedHashMap<String, Object>());
    return m;
  }
  static final String DOC = "{\"version\":3,\"file\":\"a.js\",\"n\":[1,1.0E10,1e3,0.10,-0,12345678901234567890,1.5,true,false,null,\"s<>\"],\"o\":{\"a\":null,\"b\":{},\"c\":[]},\"u\":\"\\u00e9\\n\\/\\ud83d\\ude00\"}";
  public static void main(String[] a) throws Exception {
    c("w_chars", () -> w(w -> { w.beginArray(); w.value(allChars()); w.endArray(); }));
    c("w_chars_html", () -> w(w -> { w.setHtmlSafe(true); w.beginObject(); w.name(allChars()).value(allChars()); w.endObject(); }));
    c("w_numbers", () -> w(w -> { w.beginArray(); w.value(0L); w.value(Long.MIN_VALUE); w.value(Long.MAX_VALUE); w.value(1.0E10); w.value(-0.0); w.value(1.0E-5); w.value(100.0); w.value(1.23456789012E8); w.value(Double.MAX_VALUE); w.value((Number) 1.1f); w.value((Number) 3); w.value(true); w.value(false); w.value((String) null); w.nullValue(); w.endArray(); }));
    c("w_indent", () -> w(w -> { w.setIndent("  "); w.beginObject(); w.name("a").beginArray().endArray(); w.name("b").beginObject().endObject(); w.name("c").beginArray().value(1).beginArray().value("x").endArray().beginObject().name("k").nullValue().endObject().endArray(); w.name("d").value("e"); w.endObject(); }));
    c("w_indent_tab", () -> w(w -> { w.setIndent("\t"); w.beginArray(); w.beginObject(); w.name("a").value(1); w.name("b").jsonValue("{\"raw\": 1}"); w.endObject(); w.endArray(); }));
    c("w_nonulls", () -> w(w -> { w.setSerializeNulls(false); w.beginObject(); w.name("a").nullValue(); w.name("b").value((String) null); w.name("c").beginArray().nullValue().endArray(); w.name("d").jsonValue(null); w.endObject(); }));
    c("w_lenient", () -> w(w -> { w.setLenient(true); w.value(Double.NaN); w.value(Double.NEGATIVE_INFINITY); w.value("x"); w.beginArray().endArray(); }));
    c("w_err_nan", () -> w(w -> { w.value(Double.NaN); }));
    c("w_err_two", () -> w(w -> { w.value(1); w.value(2); }));
    c("w_err_name", () -> w(w -> { w.beginArray(); w.name("a"); w.value(1); }));
    c("w_err_dangling", () -> w(w -> { w.beginObject(); w.name("a"); w.endObject(); }));
    c("w_err_close", () -> w(w -> { w.beginObject(); w.close(); }));
    c("w_err_nest", () -> w(w -> { w.beginObject(); w.endArray(); }));
    c("w_err_noname", () -> w(w -> { w.beginObject(); w.value(1); }));

    c("g_map", () -> new Gson().toJson(sample()));
    c("g_map_nohtml", () -> new GsonBuilder().disableHtmlEscaping().create().toJson(sample()));
    c("g_map_pretty", () -> new GsonBuilder().setPrettyPrinting().create().toJson(sample()));
    c("g_map_nulls", () -> new GsonBuilder().serializeNulls().setPrettyPrinting().disableHtmlEscaping().create().toJson(sample()));
    c("g_null", () -> new Gson().toJson((Object) null));
    c("g_str", () -> new Gson().toJson("a<b"));
    c("g_set", () -> new GsonBuilder().disableHtmlEscaping().create().toJson(new LinkedHashSet<>(Arrays.asList("a.js", "b<c>.js", "it's"))));
    c("g_nan", () -> new Gson().toJson(Double.NaN));
    c("g_pojo", () -> new Gson().toJson(new P()));
    c("g_pojo_pretty_nulls", () -> new GsonBuilder().setPrettyPrinting().serializeNulls().create().toJson(new P()));
    c("g_filespec", () -> {
      List<Object> files = new ArrayList<>();
      files.add(new AbstractCommandLineRunner.JsonFileSpec("var a='<x>';\n", "out/a.js", "{\"version\":3}", "17"));
      files.add(new AbstractCommandLineRunner.JsonFileSpec("b", "b.js"));
      StringWriter sw = new StringWriter(); JsonWriter jw = new JsonWriter(sw);
      Gson g = new GsonBuilder().disableHtmlEscaping().setPrettyPrinting().create();
      g.toJson(files, new TypeToken<List<AbstractCommandLineRunner.JsChunkSpec>>() {}.getType(), jw); jw.close(); return sw.toString(); });
    c("g_writer_settings", () -> { StringWriter sw = new StringWriter(); JsonWriter jw = new JsonWriter(sw); jw.beginArray(); new Gson().toJson(sample(), Map.class, jw); jw.value("<"); jw.endArray(); return sw.toString(); });
    c("r_filespec", () -> { JsonReader r = new JsonReader(new StringReader("[{\"src\":\"x\",\"path\":\"p\",\"source_map\":\"m\",\"webpack_id\":\"3\",\"other\":[1,{\"a\":2}]},{\"sourceMap\":\"m2\",\"webpackId\":5,\"src\":null,\"path\":true},{}]"));
      StringBuilder sb = new StringBuilder(); Gson g = new Gson(); r.beginArray();
      while (r.hasNext()) { AbstractCommandLineRunner.JsonFileSpec f = g.fromJson(r, AbstractCommandLineRunner.JsonFileSpec.class); sb.append(f.getSrc() + "|" + f.getPath() + "|" + f.getSourceMap() + "|" + f.getWebpackId() + ";"); }
      r.endArray(); return sb.toString(); });
    c("t_roundtrip", () -> new Gson().fromJson(DOC, JsonObject.class).toString());
    c("t_gson", () -> new Gson().toJson(new Gson().fromJson(DOC, JsonElement.class)));
    c("t_pretty", () -> new GsonBuilder().setPrettyPrinting().create().toJson(new Gson().fromJson(DOC, JsonObject.class)));
    c("t_access", () -> { JsonObject o = new Gson().fromJson(DOC, JsonObject.class); JsonArray n = o.get("n").getAsJsonArray(); StringBuilder sb = new StringBuilder();
      sb.append(o.get("version").getAsInt()).append(',').append(o.has("file")).append(o.has("nope")).append(o.get("nope") == null).append(',');
      for (int i = 0; i < 7; i++) { String s; try { s = "" + n.get(i).getAsInt(); } catch (Exception e) { s = e.toString(); } sb.append(s).append(','); }
      for (JsonElement e : n) { sb.append(e.isJsonPrimitive() ? (e.getAsJsonPrimitive().isString() ? "S" : "P") : e.isJsonNull() ? "N" : "?").append(e.isJsonNull() ? "" : e.getAsString()).append(';'); }
      for (Map.Entry<String, JsonElement> e : o.entrySet()) sb.append(e.getKey()).append('=').append(e.getValue()).append(';');
      sb.append(o.get("u").getAsString());
      return sb.toString(); });
    for (String[] t : new String[][] {{"obj", "[1]"}, {"arr", "{}"}, {"null", "null"}, {"empty", ""}, {"ws", "  "}}) {
      c("t_cls_obj_" + t[0], () -> String.valueOf(new Gson().fromJson(t[1], JsonObject.class)));
      c("t_cls_arr_" + t[0], () -> String.valueOf(new Gson().fromJson(t[1], JsonArray.class)));
      c("t_cls_el_" + t[0], () -> String.valueOf(new Gson().fromJson(t[1], JsonElement.class)));
    }
    c("t_err_getarr", () -> "" + new Gson().fromJson("{\"a\":1}", JsonObject.class).get("a").getAsJsonArray());
    c("t_err_getobj", () -> "" + new Gson().fromJson("[null]", JsonArray.class).get(0).getAsJsonObject());
    c("t_err_getstr", () -> "" + new Gson().fromJson("{\"a\":{}}", JsonObject.class).get("a").getAsString());
    c("t_err_getint", () -> "" + new Gson().fromJson("{\"a\":\"x\"}", JsonObject.class).get("a").getAsInt());
    c("t_build", () -> { JsonArray chunks = new JsonArray(); JsonObject node = new JsonObject(); node.add("name", new JsonPrimitive("m<1>")); JsonArray deps = new JsonArray(); node.add("dependencies", deps); deps.add(new JsonPrimitive("d")); node.add("inputs", new JsonArray()); node.add("x", null); chunks.add(node);
      return chunks.toString() + "\n" + new GsonBuilder().setPrettyPrinting().create().toJson(chunks) + "\n" + new Gson().toJson(chunks); });
    Type ls = new TypeToken<List<String>>() {}.getType();
    Type ml = new TypeToken<Map<String, Long>>() {}.getType();
    String[] lsIn = {"[\"a\",1,true,null,1.50,\"\\u0041\"]", "null", "", "[", "[\"a\" \"b\"]", "{}", "[[1]]", "['a',b,]", "[\"a\"] x", "[\"a\"]]", "[\"unterminated]", "[\"bad\\q\"]", "// c\n[\"a\"/*x*/,#y\n\"b\"]", ")]}'\n[\"p\"]", "[\"a\";\"b\"]", "[1 2]", "\"str\"", "[\n \"a\",\n  x y]", "[\"a\\\nb\",\n\"c\" 1]"};
    for (int i = 0; i < lsIn.length; i++) { final String in = lsIn[i]; c("r_list_" + i, () -> String.valueOf((Object) new GsonBuilder().disableHtmlEscaping().create().fromJson(in, ls))); }
    String[] mlIn = {"{\"a\":1,\"b\":\"2\",\"c\":3.0,\"d\":-9223372036854775808,\"e\":1e2}", "{\"a\":1.5}", "{\"a\":1,\"a\":2}", "{\"a\":null}", "{a:1,'b'=2;c=>3}", "{\"a\":\"x\"}", "{\"a\":9223372036854775808}", "{\"a\":1", "{\"a\" 1}", "{\"a\":}", "{,}", "{\"a\":[1]}", "[[\"k\",1]]", "{\"a\":true}", "{\"a\":01}", "{\"a\":1}{", "{\"a\":NaN}"};
    for (int i = 0; i < mlIn.length; i++) { final String in = mlIn[i]; c("r_map_" + i, () -> String.valueOf((Object) new Gson().fromJson(in, ml))); }
    String[] objIn = {"{\"a\":[1,2.5,\"x\",true,null,{\"b\":-0}],\"c\":1E400}", "tRuE", "NULL", "truex", "-", "1.", "\"a\\u00zz\"", "[1e5,1E+5,1e-5,-0.0,0123]"};
    for (int i = 0; i < objIn.length; i++) { final String in = objIn[i]; c("r_obj_" + i, () -> { Object o = new Gson().fromJson(in, Object.class); return String.valueOf(o) + "|" + new Gson().toJson(o); }); }
    c("r_stream", () -> { JsonReader r = new JsonReader(new StringReader("{\"a\":[1,2,{\"skip\":[1,{\"x\":null}]},\"s\"],\"b\":12.5,\"c\":\"7\",\"d\":null,\"e\":false,\"f\":123456789012}")); StringBuilder sb = new StringBuilder();
      r.beginObject(); sb.append(r.peek()).append(r.nextName()); r.beginArray(); sb.append(r.nextInt()).append(r.nextString()); sb.append(r.getPath()); r.skipValue(); sb.append(r.getPath()).append(r.peek()).append(r.nextString()).append(r.hasNext()); r.endArray();
      sb.append(r.nextName()).append(r.peek()).append(r.nextDouble()); sb.append(r.nextName()).append(r.nextInt()); sb.append(r.nextName()); r.nextNull(); sb.append(r.nextName()).append(r.nextBoolean()); sb.append(r.nextName()).append(r.nextLong()).append(r.peek()); r.endObject(); sb.append(r.peek()).append(r.getPath()); return sb.toString(); });
    String[][] strict = {{"s0", "{'a':1}"}, {"s1", "[1,]"}, {"s2", "[1] 2"}, {"s3", "{\"a\":1}"}, {"s4", "[NaN]"}, {"s5", "[\"NaN\"]"}, {"s6", "x"}, {"s7", "[1;2]"}, {"s8", "[//\n1]"}, {"s9", "[1.5]"}, {"s10", "[\"1.0\"]"}, {"s11", "[true]"}, {"s12", "[99999999999]"}};
    for (String[] t : strict) c("r_strict_" + t[0], () -> { JsonReader r = new JsonReader(new StringReader(t[1])); StringBuilder sb = new StringBuilder(); try { while (true) { JsonToken k = r.peek(); sb.append(k).append(' '); if (k == JsonToken.END_DOCUMENT) break; switch (k) { case BEGIN_ARRAY: r.beginArray(); break; case END_ARRAY: r.endArray(); break; case BEGIN_OBJECT: r.beginObject(); break; case END_OBJECT: r.endObject(); break; case NAME: sb.append(r.nextName()).append(' '); break; case NUMBER: sb.append(r.nextInt()).append(' '); break; case STRING: sb.append(r.nextDouble()).append(' '); break; case BOOLEAN: sb.append(r.nextString()); break; default: r.skipValue(); } } } catch (Exception e) { sb.append("EXC ").append(e); } return sb.toString(); });
  }
}
