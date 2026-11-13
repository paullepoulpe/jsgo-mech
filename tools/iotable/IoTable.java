import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Outcome;
import java.io.File;
import java.io.PrintStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.attribute.FileTime;
import java.util.ArrayList;
import java.util.HexFormat;
import java.util.List;
import java.util.Optional;
import java.util.Random;
import java.util.concurrent.TimeUnit;
import java.util.function.Supplier;
import java.util.zip.CRC32;
import java.util.zip.Deflater;

/**
 * Writes what the Java Io and Json facades return for a few thousand inputs, as tab-separated tables that
 * go/closure/rt_io_test.go checks the Go implementation against. See run.sh.
 *
 * <p>A line is: operation, one argument, result. Strings are escaped (see esc); byte arrays are
 * hex; a thrown exception or an error Outcome is "!" + class name + ": " + message.
 */
public final class IoTable {
  static PrintStream out;

  static String esc(String s) {
    if (s == null) {
      return "\\N";
    }
    StringBuilder b = new StringBuilder();
    for (char c : s.toCharArray()) {
      if (c < 0x20 || c > 0x7e || c == '\\') {
        b.append(String.format("\\u%04x", (int) c));
      } else {
        b.append(c);
      }
    }
    return b.toString();
  }

  static String hex(byte[] b) {
    return b == null ? "\\N" : "x" + HexFormat.of().formatHex(b);
  }

  static byte[] unhex(String s) {
    return HexFormat.of().parseHex(s);
  }

  static String exc(Throwable t) {
    return "!" + t.getClass().getName() + (t.getMessage() == null ? "" : ": " + esc(t.getMessage()));
  }

  /** A result that is in the table's notation already. */
  record Raw(String text) {}

  static String show(Object v) {
    if (v instanceof Raw r) {
      return r.text();
    } else if (v instanceof Outcome<?> o) {
      return o.isErr() ? exc(o.error()) : show(o.get());
    } else if (v instanceof byte[] b) {
      return hex(b);
    } else if (v instanceof List<?> l) {
      StringBuilder b = new StringBuilder("[" + l.size() + "]");
      for (Object x : l) {
        b.append('\t').append(show(x));
      }
      return b.toString();
    } else if (v instanceof String s) {
      return esc(s);
    } else if (v instanceof java.util.Optional<?> o) {
      return o.isPresent() ? show(o.get()) : "null";
    } else if (v instanceof com.google.javascript.jscomp.base.SimpleJsonValue j) {
      return esc(j.toString());
    }
    return String.valueOf(v);
  }

  static void row(String op, Supplier<Object> f, String... args) {
    StringBuilder b = new StringBuilder(op);
    for (String a : args) {
      b.append('\t').append(a.startsWith("x") && op.endsWith("#") ? a : esc(a));
    }
    String r;
    try {
      r = show(f.get());
    } catch (RuntimeException e) {
      r = exc(e);
    }
    out.println(b + "\t" + r);
  }

  // ---- UTF-8

  static final String[] BYTES = {
    "", "41", "c3a9", "efbbbf41", "c3", "c341", "80", "bf41", "c0af", "c080", "c1bf", "e0", "e080",
    "e08080", "e09f80", "e0a080", "e0a0", "e0a041", "e141", "e18041", "e180", "eda080", "edbfbf",
    "ed9fbf", "eda0", "ee8080", "efbfbd", "f0", "f08080", "f0808080", "f0908080", "f09080",
    "f0908041", "f09f9880", "f1", "f18080", "f180", "f48fbfbf", "f4908080", "f490", "f5808080",
    "f8888080", "fc80808080", "fe", "ff", "feff", "fffe", "8182839091a0e9ff", "7f8090", "41c2",
    "41e282", "41f09f98", "61f0808080ff62", "eda080edb080", "c3a9c3", "e282ac", "c2a0",
    "f09f98f09f9880", "e282e282ac", "f0e282ac", "c2c2a0", "efbfbdff", "f48fbfbff4908080",
  };

  // The bytes at which the rules of well-formed UTF-8 change.
  static final int[] EDGES = {
    0x00, 0x41, 0x7f, 0x80, 0x8f, 0x90, 0x9f, 0xa0, 0xbf, 0xc0, 0xc1, 0xc2, 0xdf, 0xe0, 0xe1, 0xec,
    0xed, 0xee, 0xef, 0xf0, 0xf1, 0xf3, 0xf4, 0xf5, 0xf7, 0xf8, 0xff,
  };

  static final String[] TEXTS = {
    "", "a", "é", "€", "ŒŽ™", "😀", "\ud83d", "\ude00",
    "a\ud800b", "\ude00\ud83d", "\ud83d😀", "﻿a", "�", "\u0080\u009f ",
    "￿", "abé中😀", "퟿", "􏿿", "\udfff\udbff", "\u0000",
    "😀\ude00", "\ud800𐀀\udc00",
  };

  static void utf8Row(byte[] b) {
    // Lenient and strict in one line: the text with U+FFFD, then the text or null.
    row("utf8#", () -> new Raw(show(Io.utf8String(b)) + "\t" + show(Io.utf8StringStrict(b))), hex(b));
  }

  static void utf8Rows() {
    Random random = new Random(20261113);
    for (String b : BYTES) {
      utf8Row(unhex(b));
    }
    for (int x : EDGES) {
      utf8Row(new byte[] {(byte) x});
      for (int y : EDGES) {
        utf8Row(new byte[] {(byte) x, (byte) y});
      }
    }
    for (int i = 0; i < 1500; i++) { // edge bytes, strung together
      byte[] b = new byte[1 + random.nextInt(10)];
      for (int k = 0; k < b.length; k++) {
        b[k] = (byte) EDGES[random.nextInt(EDGES.length)];
      }
      utf8Row(b);
    }
    for (int i = 0; i < 500; i++) { // any bytes
      byte[] b = new byte[1 + random.nextInt(12)];
      random.nextBytes(b);
      utf8Row(b);
    }
    for (int i = 0; i < 1000; i++) { // well-formed text with one or two bytes changed, cut or added
      StringBuilder text = new StringBuilder();
      for (int k = 1 + random.nextInt(6); k > 0; k--) {
        int[] limits = {0x80, 0x800, 0xd800, 0x10000, 0x110000};
        int cp = random.nextInt(limits[random.nextInt(limits.length)]);
        text.appendCodePoint(cp >= 0xd800 && cp < 0xe000 ? cp + 0x800 : cp);
      }
      byte[] b = Io.utf8Bytes(text.toString());
      for (int k = random.nextInt(3); k > 0 && b.length > 0; k--) {
        int at = random.nextInt(b.length);
        switch (random.nextInt(3)) {
          case 0 -> b[at] = (byte) EDGES[random.nextInt(EDGES.length)];
          case 1 -> b = java.util.Arrays.copyOf(b, at);
          default -> b[at] ^= (byte) (1 << random.nextInt(8));
        }
      }
      utf8Row(b);
    }
    for (String t : TEXTS) {
      row("utf8Bytes", () -> Io.utf8Bytes(t), t);
    }
    for (int i = 0; i < 500; i++) { // chars with many surrogates among them
      char[] c = new char[1 + random.nextInt(6)];
      for (int k = 0; k < c.length; k++) {
        c[k] =
            switch (random.nextInt(4)) {
              case 0 -> (char) (0xd800 + random.nextInt(0x400));
              case 1 -> (char) (0xdc00 + random.nextInt(0x400));
              case 2 -> (char) random.nextInt(0x80);
              default -> (char) random.nextInt(0x10000);
            };
      }
      String t = new String(c);
      row("utf8Bytes", () -> Io.utf8Bytes(t), t);
    }
  }

  // ---- Codecs

  static final String[] BASE64 = {
    "", "AA==", "AA", "AAA=", "AAA", "AAAA", "A", "AAAAA", "aGVsbG8=", "aGVsbG8", "aGVs bG8=",
    "aGVs\nbG8=", "aGVs\rbG8=", "a-b_", "a+b/", "====", "AA=", "AA===", "A=", "=", "AA==AA==",
    "éAAA", "AAA*", "AB==", "AAB=", "/+/+", "AA\n", "\nAA", "AA==\n", "A===", "AAAAA===",
    "AAAA====", " AAAA", "AAAA ", "AA\u0000A", "AB", "AAB", "//8=", "//9=", "/x==", "/w==",
  };

  /** The length and CRC-32 of data, or the failure: what the inflate and gunzip lines hold. */
  static Object summary(Outcome<byte[]> data) {
    if (data.isErr()) {
      return data;
    }
    CRC32 crc = new CRC32();
    crc.update(data.get());
    return data.get().length + " " + Long.toHexString(crc.getValue());
  }

  static byte[] deflate(byte[] data, int level, int strategy) {
    Deflater deflater = new Deflater(level, true);
    deflater.setStrategy(strategy);
    deflater.setInput(data);
    deflater.finish();
    java.io.ByteArrayOutputStream out = new java.io.ByteArrayOutputStream();
    byte[] buffer = new byte[4096];
    while (!deflater.finished()) {
      out.write(buffer, 0, deflater.deflate(buffer));
    }
    deflater.end();
    return out.toByteArray();
  }

  static byte[] concat(byte[] a, byte[] b) {
    byte[] r = java.util.Arrays.copyOf(a, a.length + b.length);
    System.arraycopy(b, 0, r, a.length, b.length);
    return r;
  }

  static List<byte[]> payloads() {
    Random random = new Random(42);
    List<byte[]> payloads = new ArrayList<>();
    payloads.add(new byte[0]);
    payloads.add(new byte[] {0});
    payloads.add(Io.utf8Bytes("hello, hello, hello, hello world\n"));
    payloads.add(new byte[70000]);
    byte[] noise = new byte[3000];
    random.nextBytes(noise);
    payloads.add(noise);
    StringBuilder text = new StringBuilder();
    while (text.length() < 40000) {
      text.append("var x").append(random.nextInt(500)).append(" = function() { return ");
      text.append(random.nextInt(100000)).append("; };\n");
    }
    payloads.add(Io.utf8Bytes(text.toString()));
    return payloads;
  }

  static void codecRows() {
    Random random = new Random(7);
    for (String b : BASE64) {
      // The contract names the class of the failure only.
      row("base64Decode", () -> {
        try {
          return Io.base64Decode(b);
        } catch (IllegalArgumentException e) {
          return new Raw("!" + e.getClass().getName());
        }
      }, b);
    }
    for (int n = 0; n < 40; n++) {
      byte[] b = new byte[n < 20 ? n : random.nextInt(200)];
      random.nextBytes(b);
      row("base64Encode#", () -> Io.base64Encode(b), hex(b));
      row("base64Decode", () -> Io.base64Decode(Io.base64Encode(b)), Io.base64Encode(b));
    }
    List<byte[]> streams = new ArrayList<>();
    for (byte[] data : payloads()) {
      for (int level : new int[] {0, 1, 6, 9}) {
        if (level > 0 || data.length < 5000) { // a stored stream is as long as its data
          streams.add(deflate(data, level, Deflater.DEFAULT_STRATEGY));
        }
      }
      streams.add(deflate(data, 6, Deflater.HUFFMAN_ONLY));
      streams.add(deflate(data, 6, Deflater.FILTERED));
    }
    for (byte[] s : streams) {
      row("inflate#", () -> summary(Io.inflate(s)), hex(s));
    }
    byte[] small = deflate(Io.utf8Bytes("hello, hello, hello, hello world\n"), 6, 0);
    byte[] stored = deflate(Io.utf8Bytes("stored"), 0, 0);
    byte[] dynamic = java.util.Arrays.copyOf(streams.get(streams.size() - 5), 400);
    row("inflate#", () -> summary(Io.inflate(concat(small, new byte[] {1, 2, 3}))), hex(concat(small, new byte[] {1, 2, 3})));
    row("inflate#", () -> summary(Io.inflate(concat(small, small))), hex(concat(small, small)));
    for (byte[] s : new byte[][] {small, stored, dynamic}) { // cut short
      for (int n = 0; n < s.length; n += s.length > 100 ? 7 : 1) {
        byte[] cut = java.util.Arrays.copyOf(s, n);
        row("inflate#", () -> summary(Io.inflate(cut)), hex(cut));
      }
    }
    for (String bad : new String[] {"07", "06", "05", "ff", "00", "0000", "010100feff41", "010100ffff41", "0300", "0b", "fdff"}) {
      row("inflate#", () -> summary(Io.inflate(unhex(bad))), bad); // block types, stored lengths
    }
    for (int i = 0; i < 200; i++) { // random bytes are almost never a stream
      byte[] b = new byte[1 + random.nextInt(30)];
      random.nextBytes(b);
      row("inflate#", () -> summary(Io.inflate(b)), hex(b));
    }

    List<byte[]> files = new ArrayList<>();
    for (byte[] data : payloads()) {
      files.add(Io.gzip(data));
    }
    for (byte[] f : files) {
      row("gunzip#", () -> summary(Io.gunzip(f)), hex(f));
    }
    byte[] gz = files.get(2);
    for (int n = 0; n < gz.length; n++) { // cut short
      byte[] cut = java.util.Arrays.copyOf(gz, n);
      row("gunzip#", () -> summary(Io.gunzip(cut)), hex(cut));
    }
    for (int at : new int[] {0, 1, 2, gz.length - 8, gz.length - 5, gz.length - 4, gz.length - 1}) {
      byte[] changed = gz.clone(); // magic, method, checksum, length
      changed[at] ^= 0x10;
      row("gunzip#", () -> summary(Io.gunzip(changed)), hex(changed));
    }
    // The optional header fields: extra (4), name (8), comment (16), all three.
    String tail = hex(java.util.Arrays.copyOfRange(gz, 10, gz.length)).substring(1);
    for (String header : new String[] {
      "1f8b0804000000000000" + "03004142" + "43", "1f8b0808000000000003" + "612e747874" + "00",
      "1f8b0810000000000003" + "6869" + "00", "1f8b081c000000000003" + "0100ff" + "6100" + "6200",
    }) {
      row("gunzip#", () -> summary(Io.gunzip(unhex(header + tail))), header + tail);
    }
    row("gunzip#", () -> summary(Io.gunzip(small)), hex(small));
    row("gunzip#", () -> summary(Io.gunzip(Io.utf8Bytes("not gzip at all"))), hex(Io.utf8Bytes("not gzip at all")));
  }

  // ---- Files

  // The tree: directories, files (each holds its own path), then links "name -> target".
  static final String[] DIRS = {"T/empty", "T/sub/deep/deeper", "T/other", "T/x", "T/y", "T/sp ace"};
  static final String[] FILES = {
    "T/a.js", "T/B.js", "T/b.JS", "T/_.js", "T/.hidden", "T/sub/c.js", "T/sub/deep/e.js",
    "T/sub/deep/deeper/f.js", "T/sub.js", "T/sub-1", "T/sub0", "T/other/g.js", "T/other/[odd].js",
    "T/sp ace/h.js", "T/üml.js", "T/～.js", "T/😀.js", "T/x/in-x", "T/y/in-y",
    "T/Z", "T/sub/deep/a.js", "plain",
  };
  static final String[] LINKS = {
    "T/lnk -> sub/deep", "T/dangling -> nowhere", "T/sub/up -> ..", "T/sub/self -> .",
    "T/sub/deep/top -> ../..", "T/filelnk -> a.js", "T/x/toY -> ../y", "T/y/toX -> ../x",
    "T/other/abs -> /nonexistent-jsgo", "outside -> T/sub", "T/loop -> loop",
    "plainlnk -> plain",
  };

  // Not here: a file's path with "/" after it, which the contract does not speak of (java.io.File
  // drops the "/" and finds the file, the operating system says it is not a directory).
  static final String[] PATHS = {
    "T", "T/", "T/a.js", "T/empty", "T/sub", "T/sub/", "T/sub/deep/deeper", "T/lnk", "T/lnk/e.js",
    "T/dangling", "T/filelnk", "T/loop", "T/sub/up", "T/sub/up/sub/c.js", "T/x", "T/x/toY",
    "outside", "plain", "plain/x", "nope", "nope/x", "", ".", "..", "./T/../T/a.js",
    "T//a.js", "T/üml.js", "T/😀.js", "/", "/nonexistent-jsgo/x", "T/sp ace/h.js",
    "x\u0000y",
  };

  static String sinkScript(String path, boolean append) {
    Outcome<Io.Sink> opened = Io.openFile(path, append);
    if (opened.isErr()) {
      return show(opened);
    }
    Io.Sink sink = opened.get();
    sink.write(Io.utf8Bytes("one "));
    sink.write(new byte[0]);
    sink.write(Io.utf8Bytes("two"));
    String first = show(sink.close());
    sink.write(Io.utf8Bytes(" never"));
    return first + " " + show(sink.close()) + " " + show(Io.readBytes(path));
  }

  static void fileRows() throws Exception {
    for (String d : DIRS) {
      Files.createDirectories(Path.of(d));
      out.println("dir\t" + esc(d) + "\tnull");
    }
    for (String f : FILES) {
      Files.write(Path.of(f), Io.utf8Bytes(f));
      out.println("file\t" + esc(f) + "\tnull");
    }
    for (String l : LINKS) {
      String[] parts = l.split(" -> ");
      Files.createSymbolicLink(Path.of(parts[0]), Path.of(parts[1]));
      out.println("link\t" + esc(l) + "\tnull");
    }
    // A time with nine digits after the second, to see that none are lost.
    Files.setLastModifiedTime(Path.of("plain"), FileTime.from(1000000000123456789L, TimeUnit.NANOSECONDS));
    out.println("mtime\tplain\tnull");
    for (String p : PATHS) {
      row("exists", () -> Io.exists(p), p);
      row("isDirectory", () -> Io.isDirectory(p), p);
      row("readBytes", () -> Io.readBytes(p), p);
      if (!p.equals("/") && !p.equals("..")) {
        row("listFiles", () -> Io.listFiles(p), p);
      }
    }
    for (String p : new String[] {"plain", "plainlnk", "nope", "T/dangling", "plain/x"}) {
      row("lastModified", () -> Io.lastModified(p), p);
    }
    // Writing: each line leaves what the next ones see.
    for (String p : new String[] {
      "w1", "T/a.js", "T/empty", "nope/w", "plain/w", "T/filelnk", "T/dangling", "T/lnk/w", "",
      "T/loop", "wé😀",
    }) {
      row("writeBytes", () -> new Raw(show(Io.writeBytes(p, Io.utf8Bytes("written"))) + " " + show(Io.readBytes(p))), p);
      row("truncate", () -> new Raw(sinkScript(p, false)), p);
      row("append", () -> new Raw(sinkScript(p, true)), p);
      row("append", () -> new Raw(sinkScript(p, true)), p);
    }
    for (String p : new String[] {"m1", "m2/a/b/c", "m2/a", "T/sub", "plain/m", "T/lnk/made", "m3/", "m4//x/./y", ""}) {
      row("mkdirs", () -> {
        Io.mkdirs(p);
        return Io.isDirectory(p);
      }, p);
    }
    row("readBytes", () -> Io.readBytes("T/a.js"), "T/a.js");
    row("listFiles", () -> Io.listFiles("."), ".");
  }

  /** Documents around every rule of Json.parse: standard JSON, its limits, and what it refuses. */
  static List<String> jsonDocuments() {
    List<String> docs = new ArrayList<>(List.of(
        "null", "true", "false", "0", "-0", "1.5e+10", "1E5", "-1.0e-2", "\"\"", "\"a\"", "[]", "{}",
        " \t\r\n[1, 2 ,3]\n", "{\"a\":1,\"b\":[true,false,null],\"c\":{\"d\":\"e\"}}",
        "{\"a\":1,\"b\":2,\"a\":3}", "{\"\":0}", "{\"a\":{\"x\":1,\"x\":[2]},\"a\":4,\"b\":5}",
        "\"\\\" \\\\ \\/ \\b \\f \\n \\r \\t \\u00e9 \\u4E2d\"", "\"\u00e9\u4e2d\ud83d\ude00\"",
        "\"\\ud83d\\ude00\"", "\"\\ud83d\"", "\"\\ude00\\ud83d\"", "\"a\\ud83dx\"", "\"\ud83d\"",
        "\"\ude00\"", "\"" + (char) 0xd83d + "\\ude00\"", "{\"\\ud800\":1,\"\ufffd\":2}", "\"\\u0000\"", "\"\u007f\u0080\"",
        "\ufeff1", "\ufeff\ufeff1", "\ufeff", "1\ufeff", " \ufeff1", "\ufeff [\"\ufeff\"]",
        "", " ", "nul", "NULL", "True", "tru", "truee", "nulll", "[nul]", "NaN", "Infinity", "-Infinity",
        "01", "-01", "1.", ".5", "+1", "1e", "1e+", "0x10", "-", "--1", "1.2.3", "1e5e5", "1 2", "1,",
        "'a'", "\"a", "a", "\"\\x\"", "\"\\'\"", "\"\\u12\"", "\"\\u12G4\"", "\"\\\"", "\"a\nb\"", "\"a\tb\"",
        "\"\u001f\"", "[", "]", "[1", "[1,", "[1,]", "[,1]", "[1 2]", "[1,,2]", "[1]]", "{", "}", "{\"a\"",
        "{\"a\":", "{\"a\":1", "{\"a\":1,}", "{,}", "{a:1}", "{'a':1}", "{\"a\" 1}", "{\"a\":1 \"b\":2}",
        "{1:2}", "{\"a\":1}}", "[1] x", "[1] //c", "/*c*/[1]", "#c\n1", "[1;2]", "{\"a\"=1}", "{\"a\"=>1}",
        "[1] \n\t ", "[1]\u0000", "\u00a01", "[\u00a0]", "[1]\u000c", ")]}'\n[1]", "[1e400]", "[-1E-400]",
        "123456789012345678901234567890", "[0.1234567890123456789012345678901234567890]", "tRue", "[E]",
        "{\"a\":tru}", "\"\\u00E9\\u00e9\"", "[[[[[[[[[[1]]]]]]]]]]", "{\"a\":{\"a\":{\"a\":{}}}}"));
    for (int n : new int[] {999, 1000, 1001}) {
      docs.add("[".repeat(n) + "]".repeat(n));
      docs.add("{\"a\":".repeat(n) + "1" + "}".repeat(n));
      docs.add("[".repeat(n) + "1" + "]".repeat(n));
    }
    for (int n : new int[] {19, 20, 21, 22}) {
      docs.add("1".repeat(n));
      docs.add("-" + "1".repeat(n));
      docs.add("[" + "9".repeat(n) + ".5e3]");
      docs.add("0." + "1".repeat(n));
      docs.add("1e" + "1".repeat(n));
      docs.add("\"" + "1".repeat(n) + "\"");
    }
    for (int n : new int[] {1022, 1023, 1024, 1025}) {
      docs.add("0." + "1".repeat(n - 2));
      docs.add("-1." + "0".repeat(n - 3));
      docs.add("[1e" + "0".repeat(n - 3) + "1]");
    }
    return docs;
  }

  // ---- The report of a failure, without the lines that say where ("at ...", "... 3 more")

  static Throwable failure(String kind) {
    return switch (kind) {
      case "plain" -> new RuntimeException("something\nin two lines");
      case "bare" -> new IllegalStateException();
      case "io" -> new java.io.IOException("x: No such file");
      case "caused" -> new RuntimeException("outer", new IllegalArgumentException("inner"));
      case "wrapped" -> new RuntimeException(new java.io.IOException("inner"));
      case "chain" ->
          new IllegalStateException("a", new RuntimeException("b", new NullPointerException()));
      default -> throw new AssertionError(kind);
    };
  }

  static void traceRows() {
    for (String kind : new String[] {"plain", "bare", "io", "caused", "wrapped", "chain"}) {
      row("stackTraceText", () -> {
        StringBuilder kept = new StringBuilder();
        for (String line : Io.stackTraceText(failure(kind)).split("\n", -1)) {
          if (!line.matches("\\s*(at |\\.\\.\\. \\d+ more).*")) {
            kept.append(line).append('\n');
          }
        }
        return kept.toString();
      }, kind);
    }
    for (String name : new String[] {
      "externs.zip", "com/google/javascript/jscomp/js/polyfills.txt", "com/google/javascript/jscomp/js/base.js",
      "com/google/javascript/jscomp/parsing/ParserConfig.properties", "nope.txt", "/externs.zip",
      "com/google/javascript/jscomp/js/nope.js",
    }) {
      row("readResource", () -> {
        Optional<byte[]> data = Io.readResource(name);
        return data.isPresent() ? summary(Outcome.ok(data.get())) : "null";
      }, name);
    }
  }

  public static void main(String[] args) throws Exception {
    File dir = new File(args[0]);
    try (PrintStream p = new PrintStream(new File(dir, "utf8.tsv"), "UTF-8")) {
      out = p;
      utf8Rows();
    }
    try (PrintStream p = new PrintStream(new File(dir, "codecs.tsv"), "UTF-8")) {
      out = p;
      codecRows();
    }
    // The tree is made in the current directory (run.sh starts the JVM in an empty one), so that
    // the paths, and the error messages that quote them, are relative.
    try (PrintStream p = new PrintStream(new File(dir, "files.tsv"), "UTF-8")) {
      out = p;
      fileRows();
    }
    try (PrintStream p = new PrintStream(new File(dir, "misc.tsv"), "UTF-8")) {
      out = p;
      traceRows();
    }
    try (PrintStream p = new PrintStream(new File(dir, "json.tsv"), "UTF-8")) {
      out = p;
      for (String j : jsonDocuments()) {
        row("jsonParse", () -> com.google.javascript.jscomp.base.Json.parse(j), j);
      }
    }
  }
}
