import com.google.javascript.jscomp.base.Io;
import com.google.javascript.jscomp.base.Outcome;
import java.io.File;
import java.io.PrintStream;
import java.nio.file.Files;
import java.nio.file.Path;
import java.util.ArrayList;
import java.util.Collections;
import java.util.HexFormat;
import java.util.List;
import java.util.function.Supplier;

/**
 * Writes what the Java Io facade returns for a few thousand inputs, as tab-separated tables that
 * go/closure/rt_io_test.go checks the Go implementation against. See run.sh.
 *
 * <p>A line is: operation, arguments, result. Strings are escaped (see esc); byte arrays are hex;
 * a thrown exception or an error Outcome is "!" + class name + ": " + message.
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

  static String show(Object v) {
    if (v instanceof Outcome<?> o) {
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

  static final String[] PATHS = {
    "", "/", "a", "a/b", "/a/b", "a/", "a//b", "//a", "/a/", ".", "..", "./a", "../a", "a/..",
    "a/../b", "a/./b", "/..", "/../a", "a/b/../../..", "../..", "a/b/c", "/a/b/c", "a b",
    "\u00e9/\u00fc", ".a", "a.", "...", "a/.../b", "./", "../", "/.", "a/b/..", "/a/../..",
    "a\\b", "a/b/c/d.js", "../../x/y", "./.", "a/../../b", "///", "/a//b///c/", "x/\u0000y",
    "b/a", "a/b/../c", "/x", "./../a",
  };

  static final String[] URIS = {
    "", " ", "a", "a/b.js", "./a/../b", "/abs/x", "//host/p", "//", "http://a/b", "http://a/b c",
    "http:", "http:/", "http:/a", "http:a", "http://", "http://a", "file:///x/y", "file:/x%20y",
    "mailto:a@b", "urn:isbn:1", "data:text/plain;base64,AAA=", "a b", "%zz", "a%20b", "a%2",
    "a%", "/a%", "a%e4%b8%ad", "%C3%A9", "%c3", "%ff%41", "a%25b", ":a", "1a:b", "a:", "a:b:c",
    "+a:b", "a+b:c", "a.b-c+d:e", "a_b:c", "a?b#c", "a#b#c", "a#", "a?", "?q", "#f", "a[b",
    "a]b", "http://[::1]/x", "http://[::1/x", "http://[zz]/x", "a\\b", "c:\\x", "C:/x", "\u00e9",
    "a\u00a0b", "a\u0080b", "a\u3000b", "a\u2028b", "http://a:b@c:80/d", "http://a_b/c",
    "x://y z", "a^b", "a|b", "{a}", "a\"b", "<a>", "a`b", "a\tb", "a\u0000b", "a\u007fb",
    "\ud83d\ude00", "\ud83d", "http://\u4f8b\u3048.jp/\u30d1\u30b9", "http://host:port/",
    "http://h:99999999999/", "http://h:80x/", "a/b?c d", "a#b c", "a//b", "http://@/x",
    "http://a@/x", "http://:80/x", "http://a:/x", "http://a.b.c/x", "http://1.2.3.4/x",
    "http://1.2.3/x", "http://a..b/x", "http://-a/x", "http://a-/x", "http://a/%7Efoo?x=%zz",
    "a;b=c", "a,b", "a$b&c=d+e", "a!b~c*d'e(f)", "a@b", "a:b@c", "./a:b", "a/b:c", "http://a?b",
    "http://a#b", "http://?b", "http://#b", "x:?", "x:#", "x:/?", "x:a?b#c", "x:%41", "x:%4",
    "x: ", "../../a.js", "goog:foo.bar", "node_modules/@scope/pkg/index.js", "@scope/pkg",
    "./f\u00f6\u00f6.js", "http://a/b#c%", "http://a/b?c%1", "x:y#%zz", "//a/b%2fc", "a%2Fb",
    "a%00b", "%e9", "%ED%A0%80", "%F0%9F%98%80", "%f0%9f", "a%41%zz",
  };

  static final String[] LINES = {
    "", "a", "a\n", "a\nb", "a\r\nb", "a\rb", "a\n\nb", "\n", "\r", "\r\n", "\n\r", "a\r\n",
    "a\r\r\nb", "\n\n", "a\u2028b", "a\u0085b", "a\u000bb\fc", "x\ny\rz\r\n", " \n ", "\r\n\r\n",
  };

  static final String[] BYTES = {
    "", "41", "c3a9", "efbbbf41", "c3", "c341", "80", "bf41", "c0af", "c080", "c1bf", "e0", "e080",
    "e08080", "e09f80", "e0a080", "e0a0", "e0a041", "e141", "e18041", "e180", "eda080", "edbfbf",
    "ed9fbf", "eda0", "ee8080", "efbfbd", "f0", "f08080", "f0808080", "f0908080", "f09080",
    "f0908041", "f09f9880", "f0", "f1", "f18080", "f180", "f48fbfbf", "f4908080", "f490", "f5808080",
    "f8888080", "fc80808080", "fe", "ff", "feff", "fffe", "feff0041", "fffe4100", "0041", "4100",
    "00", "0041d8", "d83dde00", "3dd800de", "d800", "00d8", "dc000041", "d8000041", "41d83d",
    "8182839091a0e9ff", "7f8090", "fffe", "41c2", "41e282", "41f09f98", "61f0808080ff62",
    "eda080edb080", "fffe0000", "fefffeff", "c3a9c3", "e282ac", "c2a0",
  };

  static final String[] TEXTS = {
    "", "a", "\u00e9", "\u20ac", "\u0152\u017d\u2122", "\ud83d\ude00", "\ud83d", "\ude00",
    "a\ud800b", "\ude00\ud83d", "\ud83d\ud83d\ude00", "\ufeffa", "\ufffd", "\u0080\u009f\u00a0",
    "\u0081\u008d\u008f\u0090\u009d", "\uffff", "ab\u00e9\u4e2d\ud83d\ude00",
  };

  static final String[] CHARSETS = {
    "UTF-8", "US-ASCII", "ISO-8859-1", "UTF-16", "UTF-16BE", "UTF-16LE", "windows-1252",
  };

  static final String[] CHARSET_NAMES = {
    "UTF-8", "utf-8", "utf8", "UTF8", "unicode-1-1-utf-8", "US-ASCII", "ascii", "ASCII", "us",
    "iso-ir-6", "ISO646-US", "646", "ISO-8859-1", "latin1", "l1", "ISO_8859-1", "iso8859_1",
    "ISO8859-1", "8859_1", "cp819", "IBM819", "UTF-16", "utf16", "UTF_16", "unicode",
    "UnicodeBig", "UTF-16BE", "UTF_16BE", "X-UTF-16BE", "UnicodeBigUnmarked", "UTF-16LE",
    "x-utf-16le", "UnicodeLittleUnmarked", "windows-1252", "cp1252", "CP1252", "ibm-1252",
    "NOPE-8", "nope", "latin-1", "", " ", "bad name", "a!", "-utf8", "utf-8 ", "x", "UTF-9",
    "a:b", "a+b", "_a", ".a", "\u00e9",
  };

  static final String[] BASE64 = {
    "", "AA==", "AA", "AAA=", "AAA", "AAAA", "A", "AAAAA", "aGVsbG8=", "aGVsbG8", "aGVs bG8=",
    "aGVs\nbG8=", "a-b_", "a+b/", "====", "AA=", "AA===", "A=", "=", "AA==AA==", "\u00e9AAA",
    "AAA*", "AB==", "AAB=", "/+/+",
  };

  // The tree for the glob table: files, then links (name -> target).
  static final String[] TREE = {
    "a.js", "b.JS", "c.txt", ".hidden.js", "sub/c.js", "sub/d_test.js", "sub/deep/e.js",
    "sub/deep/deeper/f.js", "sub/x.y.js", "other/g.js", "other/[odd].js", "other/{b}.js",
    "other/a,b.js", "other/q?.js", "other/star*.js", "other/back\\slash.js", "sp ace/h.js",
    "\u00fcml/i.js", "empty/.keep", "sub/deep/a.js",
  };
  static final String[][] LINKS = {
    {"lnk", "sub/deep"}, {"dangling.js", "nowhere"}, {"sub/loop", ".."}, {"filelnk.js", "a.js"},
  };

  static final String[] GLOBS = {
    "**", "*", "*.js", "**.js", "**/*.js", "T/*.js", "T/**.js", "T/**/*.js", "T/sub/*.js",
    "T/sub/**", "T/sub/**/*.js", "T/**/deep/*", "T/*/c.js", "T/?.js", "T/?.??", "T/[ab].js",
    "T/[!a].js", "T/[a-c].*", "T/[^a].js", "T/{a,b}.js", "T/{a,c}.{js,txt}", "T/*.{js,JS}",
    "T/**_test.js", "T/**/[a-f].js", "T/sub/../a.js", "T/./a.js", "./T/a.js", "T/a.js",
    "T//a.js", "T/a.js/", "T/sub/deep/**", "T/lnk/*", "T/**/loop/**", "T/*.JS", "T/.*",
    "T/**/.*", "T/other/\\[odd\\].js", "T/other/[[]odd].js", "T/other/\\{b\\}.js",
    "T/other/a\\,b.js", "T/other/a,b.js", "T/other/q\\?.js", "T/other/q?.js", "T/other/star\\*.js",
    "T/other/back\\\\slash.js", "T/sp ace/*", "T/\u00fcml/*", "T/**/", "T/**/*", "T/*/*",
    "T/*/*/*", "T/sub/**/f.js", "T/**f.js", "T/**/**/e.js", "T/sub/d?test.js", "T/[.]hidden.js",
    "T/*hidden*", "T/dangling.js", "T/filelnk.js", "T/{sub,other}/*.js", "T/{sub/c,a}.js",
    "T/{**/e,a}.js", "T/[a-", "T/[a", "T/{a", "T/{a,{b,c}}", "T/a}", "T/\\", "T/[b-a].js",
    "T/[a/b].js", "T/[]a].js", "T/[!]a].js", "T/[-a].js", "T/[a-].js", "T/[a\\]].js",
    "T/[!-a].js", "T/a.j[s&&x]", "T/+(a).js", "T/a.js|b.js", "T/(a|b).js", "T/a$.js", "T/^a.js",
    "T/[[:alpha:]].js", "T/**/x.y.js", "T/**/x?y.js", "T/sub/[c-d]*.js", "T/empty/*", "T/empty/**",
    "T/[a-c-e].js", "T/[--a].js", "T/[a\\-c].js", "T/{}", "T/{,a}.js", "T/a{,.js}", "T/[!",
    "T/[^", "T/[", "T/[]", "T/[a]]", "T/,a", "T/{a,b", "T/a.js\\",
  };

  static byte[] zip(String charset, boolean stored, String... namesAndTexts) throws Exception {
    java.io.ByteArrayOutputStream bytes = new java.io.ByteArrayOutputStream();
    try (java.util.zip.ZipOutputStream z =
        new java.util.zip.ZipOutputStream(bytes, java.nio.charset.Charset.forName(charset))) {
      for (int i = 0; i < namesAndTexts.length; i += 2) {
        byte[] data = namesAndTexts[i + 1].getBytes("UTF-8");
        java.util.zip.ZipEntry e = new java.util.zip.ZipEntry(namesAndTexts[i]);
        e.setTime(0);
        if (stored) {
          java.util.zip.CRC32 crc = new java.util.zip.CRC32();
          crc.update(data);
          e.setMethod(java.util.zip.ZipEntry.STORED);
          e.setSize(data.length);
          e.setCrc(crc.getValue());
        }
        z.putNextEntry(e);
        z.write(data);
      }
    }
    return bytes.toByteArray();
  }

  static void zipRow(String charset, byte[] archive) {
    row(
        "readZipEntries#",
        () -> {
          Outcome<List<Io.ZipEntryData>> r = Io.readZipEntries(archive, charset);
          if (r.isErr()) {
            return r;
          }
          List<String> entries = new ArrayList<>();
          for (Io.ZipEntryData e : r.get()) {
            entries.add(e.name() + "=" + hex(e.data()));
          }
          return entries;
        },
        charset,
        hex(archive));
  }

  /** Whole archives, every truncation of two of them, and damage to single header fields. */
  static void zipRows() throws Exception {
    String text = "var a = 1; var a = 1; var a = 1;\n";
    byte[] deflated = zip("UTF-8", false, "a.js", text, "dir/", "", "dir/b.js", "x", "\u00e9.js", "");
    byte[] stored = zip("UTF-8", true, "a.js", text, "dir/", "", "dir/b.js", "x");
    zipRow("UTF-8", deflated);
    zipRow("UTF-8", stored);
    zipRow("ISO-8859-1", deflated);
    zipRow("UTF-8", zip("ISO-8859-1", false, "\u00e9.js", "y"));
    zipRow("ISO-8859-1", zip("ISO-8859-1", false, "\u00e9.js", "y"));
    zipRow("windows-1252", zip("windows-1252", true, "\u20ac.js", "y"));
    zipRow("UTF-8", zip("UTF-8", false));
    zipRow("UTF-8", new byte[0]);
    zipRow("UTF-8", "PK not a zip at all, but long enough for a header".getBytes("UTF-8"));
    for (byte[] archive : new byte[][] {deflated, stored}) {
      for (int n = 0; n < archive.length; n++) {
        zipRow("UTF-8", java.util.Arrays.copyOf(archive, n));
      }
      // Version, flags, method, checksum, sizes, name length, extra length of the first entry;
      // the first byte of its name and of its data, and the bytes that follow the data.
      for (int at : new int[] {0, 4, 6, 7, 8, 9, 14, 18, 22, 26, 28, 30, 34, 40, 50, 60, 64, 68, 72, 76}) {
        for (int flip : new int[] {1, 8, 0x80}) {
          byte[] damaged = archive.clone();
          damaged[at] ^= (byte) flip;
          zipRow("UTF-8", damaged);
        }
      }
    }
  }

  public static void main(String[] args) throws Exception {
    File dir = new File(args[0]);
    try (PrintStream p = new PrintStream(new File(dir, "paths.tsv"), "UTF-8")) {
      out = p;
      for (String a : PATHS) {
        row("pathString", () -> Io.pathString(a), a);
        row("pathFileName", () -> Io.pathFileName(a), a);
        row("pathParent", () -> Io.pathParent(a), a);
        row("pathNormalize", () -> Io.pathNormalize(a), a);
        for (String b : PATHS) {
          row("pathString", () -> Io.pathString(a, b), a, b);
          row("pathResolve", () -> Io.pathResolve(a, b), a, b);
          row("pathRelativize", () -> Io.pathRelativize(a, b), a, b);
        }
      }
      row("pathString", () -> Io.pathString("a", "b", "", "c/", "/d"), "a", "b", "", "c/", "/d");
      row("pathString", () -> Io.pathString("", "", "x"), "", "", "x");
    }
    try (PrintStream p = new PrintStream(new File(dir, "uris.tsv"), "UTF-8")) {
      out = p;
      for (String u : URIS) {
        row("uriParse", () -> Io.uriParse(u), u);
        row("uriScheme", () -> Io.uriScheme(u), u);
        row("uriPath", () -> Io.uriPath(u), u);
      }
    }
    Path tmp = Files.createTempFile("iotable", ".bin");
    try (PrintStream p = new PrintStream(new File(dir, "text.tsv"), "UTF-8")) {
      out = p;
      for (String s : LINES) {
        row("splitLines", () -> Io.splitLines(s), s);
      }
      for (String s : BASE64) {
        row("base64Decode", () -> Io.base64Decode(s), s);
      }
      for (String h : BYTES) {
        byte[] b = unhex(h);
        row("base64Encode#", () -> Io.base64Encode(b), "x" + h);
        row("utf8String#", () -> Io.utf8String(b), "x" + h);
        Io.writeBytes(tmp.toString(), b);
        for (String cs : CHARSETS) {
          row("readText#", () -> Io.readText(tmp.toString(), cs), cs, "x" + h);
          row("readTextStrict#", () -> Io.readTextStrict(tmp.toString(), cs), cs, "x" + h);
        }
      }
      for (String s : TEXTS) {
        row("utf8Bytes", () -> Io.utf8Bytes(s), s);
        for (String cs : CHARSETS) {
          row(
              "writeText",
              () -> {
                Io.writeText(tmp.toString(), cs, s);
                return Io.readBytes(tmp.toString());
              },
              cs,
              s);
        }
      }
      for (String cs : CHARSETS) {
        Io.CharEncoder e = Io.newEncoder(cs);
        StringBuilder yes = new StringBuilder();
        for (int c = 0; c < 0x10000; c++) {
          boolean can = e.canEncode((char) c);
          if (can != e.canEncode((char) (c - 1)) || c == 0) {
            yes.append(String.format("%s%04x", can ? "+" : "-", c));
          }
        }
        // The chars at which the answer changes, each with the answer from there on.
        out.println("canEncode\t" + cs + "\t" + yes);
      }
      for (String n : CHARSET_NAMES) {
        row("isCharsetSupported", () -> Io.isCharsetSupported(n), n);
        row("charsetName", () -> Io.charsetName(n), n);
      }
    }
    Files.delete(tmp);
    try (PrintStream p = new PrintStream(new File(dir, "zip.tsv"), "UTF-8")) {
      out = p;
      zipRows();
    }
    // The glob table: the tree T is made in the current directory (run.sh starts the JVM in an
    // empty one), so that the paths, and the error messages that quote them, are relative.
    try (PrintStream p = new PrintStream(new File(dir, "glob.tsv"), "UTF-8")) {
      out = p;
      Path t = Path.of("T");
      for (String f : TREE) {
        Files.createDirectories(t.resolve(f).getParent());
        Files.write(t.resolve(f), new byte[0]);
        out.println("file\t" + esc(f));
      }
      for (String[] l : LINKS) {
        Files.createSymbolicLink(t.resolve(l[0]), Path.of(l[1]));
        out.println("link\t" + esc(l[0]) + "\t" + esc(l[1]));
      }
      for (String g : GLOBS) {
        for (String d : new String[] {"T", "T/sub", "T/a.js", "T/nope", "./T/sub/../other", ""}) {
          row(
              "glob",
              () -> {
                Outcome<List<String>> r = Io.listFilesMatchingGlob(d, g);
                if (r.isErr()) {
                  return r;
                }
                // The order is the directory's own, which differs from one copy of the tree to
                // the next.
                List<String> names = new ArrayList<>(r.get());
                Collections.sort(names);
                return names;
              },
              d,
              g);
        }
      }
    }
  }
}
