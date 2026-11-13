// Prep step 15 checker: the JSON library (Gson) may be used by one production class only,
// com.google.javascript.jscomp.base.Json. Prints "file:line: text" for every line of a .java file
// under the given source roots (files under a /testing/ directory are skipped) that mentions
// com.google.gson, except in base/Json.java; exit status 1 if there is one.
// Run from the repo root (JDK 21):
//   java tools/prep/15_find_gson_uses/Scan.java java/src

import java.nio.charset.StandardCharsets;
import java.nio.file.Files;
import java.nio.file.Path;
import java.nio.file.Paths;
import java.util.List;
import java.util.TreeSet;
import java.util.stream.Stream;

public final class Scan {
  public static void main(String[] args) throws Exception {
    int uses = 0;
    int files = 0;
    for (String root : args) {
      TreeSet<Path> paths;
      try (Stream<Path> s = Files.walk(Paths.get(root))) {
        paths = new TreeSet<>(s.toList());
      }
      for (Path p : paths) {
        String n = p.toString().replace('\\', '/');
        if (!n.endsWith(".java")
            || n.contains("/testing/")
            || n.endsWith("com/google/javascript/jscomp/base/Json.java")) {
          continue;
        }
        files++;
        List<String> lines = Files.readAllLines(p, StandardCharsets.UTF_8);
        for (int i = 0; i < lines.size(); i++) {
          if (lines.get(i).contains("com.google.gson")) {
            System.out.println(n + ":" + (i + 1) + ": " + lines.get(i).trim());
            uses++;
          }
        }
      }
    }
    System.out.println("Gson uses outside base/Json.java: " + uses + " (" + files + " files scanned)");
    System.exit(uses == 0 ? 0 : 1);
  }
}
