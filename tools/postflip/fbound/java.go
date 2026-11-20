package main

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// javaClass is what the step reads from a class's Java source: its type parameters with the
// class of each bound, the type arguments it gives the class it extends, and the declared
// result type of each method of the class body (not of nested classes).
type javaClass struct {
	name    string
	params  []string          // type parameter names, in order
	bounds  map[string]string // parameter -> the class of its bound (`S extends AbstractScope<S, V>` -> AbstractScope)
	fbound  map[string]bool   // the bound mentions a parameter of the class (F-bounded)
	extends string            // the class it extends
	extArgs []string          // the type arguments it gives that class
	results map[string]string // method name -> declared result type (annotations dropped)
}

var annotRx = regexp.MustCompile(`@\w+(\([^)]*\))?\s*`)
var methodRx = regexp.MustCompile(`^  (?:(?:public|protected|private|static|final|abstract|synchronized|default)\s+)*([\w.<>?,\[\] ]+?)\s+(\w+)\s*\(`)

// readJava parses the top-level class of the Java file at path (nil if it cannot be read).
func readJava(path string) *javaClass {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil
	}
	src := string(b)
	name := strings.TrimSuffix(filepath.Base(path), ".java")
	jc := &javaClass{name: name, bounds: map[string]string{}, fbound: map[string]bool{}, results: map[string]string{}}
	i := regexp.MustCompile(`\bclass\s+` + name + `\b`).FindStringIndex(src)
	if i == nil {
		return jc
	}
	rest := src[i[1]:]
	rest = strings.TrimLeft(rest, " \t\n")
	if strings.HasPrefix(rest, "<") {
		inner, after := balanced(rest)
		rest = after
		for _, p := range splitTop(inner) {
			f := strings.Fields(p)
			if len(f) == 0 {
				continue
			}
			jc.params = append(jc.params, f[0])
			if k := strings.Index(p, " extends "); k >= 0 {
				bound := strings.TrimSpace(p[k+len(" extends "):])
				cls := bound
				if j := strings.Index(bound, "<"); j >= 0 {
					cls = bound[:j]
				}
				jc.bounds[f[0]] = strings.TrimSpace(cls)
			}
		}
		for p := range jc.bounds {
			for _, q := range splitTop(inner) {
				if strings.Fields(q)[0] != p {
					continue
				}
				k := strings.Index(q, "<")
				if k < 0 {
					continue
				}
				for _, a := range splitTop(q[k+1 : strings.LastIndex(q, ">")]) {
					for _, pp := range jc.params {
						if strings.TrimSpace(a) == pp {
							jc.fbound[p] = true
						}
					}
				}
			}
		}
	}
	rest = strings.TrimLeft(rest, " \t\n")
	if strings.HasPrefix(rest, "extends ") {
		rest = strings.TrimLeft(rest[len("extends "):], " \t\n")
		j := 0
		for j < len(rest) && (isIdent(rest[j]) || rest[j] == '.') {
			j++
		}
		jc.extends = rest[:j]
		rest = rest[j:]
		if strings.HasPrefix(rest, "<") {
			inner, _ := balanced(rest)
			for _, a := range splitTop(inner) {
				jc.extArgs = append(jc.extArgs, strings.TrimSpace(a))
			}
		}
	}
	// method result types at the class body's indentation (two spaces, google-java-format)
	for _, line := range strings.Split(src, "\n") {
		l := annotRx.ReplaceAllString(line, "")
		if m := methodRx.FindStringSubmatch(l); m != nil {
			if _, ok := jc.results[m[2]]; !ok {
				jc.results[m[2]] = strings.TrimSpace(m[1])
			}
		}
	}
	return jc
}

func isIdent(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

// balanced returns the text inside the <...> that s starts with, and what follows it.
func balanced(s string) (string, string) {
	d := 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			d++
		case '>':
			d--
			if d == 0 {
				return s[1:i], s[i+1:]
			}
		}
	}
	return s, ""
}

// splitTop splits s at the commas outside <...>.
func splitTop(s string) []string {
	var out []string
	d, last := 0, 0
	for i := 0; i < len(s); i++ {
		switch s[i] {
		case '<':
			d++
		case '>':
			d--
		case ',':
			if d == 0 {
				out = append(out, strings.TrimSpace(s[last:i]))
				last = i + 1
			}
		}
	}
	if strings.TrimSpace(s[last:]) != "" {
		out = append(out, strings.TrimSpace(s[last:]))
	}
	return out
}
