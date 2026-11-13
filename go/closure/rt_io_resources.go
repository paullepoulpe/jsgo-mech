package closure

// The resources of the Closure Compiler jar (externs.zip, the runtime JS
// libraries, *.properties ...), embedded from the resources directory, which
// tools/j2go/extract-resources.sh fills from the jar. They are served by
// Class.getResource / Class.getResourceAsStream and by ResourceBundle.

import (
	"embed"
	"strings"
	"unicode/utf8"
)

//go:embed all:resources
var resourcesFS_ embed.FS

// resourceBytes_ is the content of the resource with the absolute name (no
// leading slash). The bytes must not be modified.
func resourceBytes_(name string) ([]byte, bool) {
	if name == "" || strings.HasSuffix(name, "/") {
		return nil, false
	}
	b, err := resourcesFS_.ReadFile("resources/" + name)
	if err != nil {
		return nil, false
	}
	return b, true
}

// resourceName_ resolves a resource name the way Class.getResource does: a
// name starting with "/" is absolute, any other is relative to the package of
// the class.
func (this *Class) resourceName_(name string) string {
	if name == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	if strings.HasPrefix(name, "/") {
		return name[1:]
	}
	if i := strings.LastIndexByte(this.name_, '.'); i >= 0 {
		return strings.ReplaceAll(this.name_[:i], ".", "/") + "/" + name
	}
	return name
}

// ---- java.util.ResourceBundle, backed by the embedded .properties files

type ResourceBundle struct {
	baseName_ string
	keys_     []string
	values_   map[string]string
}

var bundles_ = map[string]*ResourceBundle{}

func missingResource_(msg string) any {
	// java.util.MissingResourceException is a RuntimeException.
	return new_RuntimeException_String(msg).self()
}

func ResourceBundle_getBundle(baseName string) *ResourceBundle {
	if baseName == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	if b := bundles_[baseName]; b != nil {
		return b
	}
	data, ok := resourceBytes_(strings.ReplaceAll(baseName, ".", "/") + ".properties")
	if !ok {
		panic(missingResource_("Can't find bundle for base name " + baseName + ", locale en_US"))
	}
	b := &ResourceBundle{baseName_: baseName, values_: map[string]string{}}
	// A properties bundle is UTF-8, or ISO-8859-1 if it is not valid UTF-8.
	text := string(data)
	if !utf8.Valid(data) {
		text, _ = charsets_[csLatin1_].decode(data)
	}
	parseProperties_(text, func(k, v string) {
		if _, dup := b.values_[k]; !dup {
			b.keys_ = append(b.keys_, k)
		}
		b.values_[k] = v
	})
	bundles_[baseName] = b
	return b
}

func (this *ResourceBundle) containsKey(key string) bool {
	if key == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	_, ok := this.values_[key]
	return ok
}

func (this *ResourceBundle) getString(key string) string {
	if key == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	v, ok := this.values_[key]
	if !ok {
		panic(missingResource_("Can't find resource for bundle java.util.PropertyResourceBundle, key " + key))
	}
	return v
}

func (this *ResourceBundle) getObject(key string) any { return this.getString(key) }

func (this *ResourceBundle) keySet() Set {
	s := new_LinkedHashSet()
	for _, k := range this.keys_ {
		s.add(k)
	}
	return s
}

// parseProperties_ reads the format of java.util.Properties.load.
func parseProperties_(text string, put func(k, v string)) {
	isSpace := func(c byte) bool { return c == ' ' || c == '\t' || c == '\f' }
	// Split into natural lines.
	var lines []string
	for len(text) > 0 {
		i := strings.IndexAny(text, "\r\n")
		if i < 0 {
			lines = append(lines, text)
			break
		}
		lines = append(lines, text[:i])
		if text[i] == '\r' && i+1 < len(text) && text[i+1] == '\n' {
			i++
		}
		text = text[i+1:]
	}
	for n := 0; n < len(lines); n++ {
		line := strings.TrimLeftFunc(lines[n], func(r rune) bool { return r < 0x80 && isSpace(byte(r)) })
		if line == "" || line[0] == '#' || line[0] == '!' {
			continue
		}
		// Join continuation lines: a line ending in an odd number of backslashes.
		for {
			k := 0
			for k < len(line) && line[len(line)-1-k] == '\\' {
				k++
			}
			if k%2 == 0 {
				break
			}
			line = line[:len(line)-1]
			if n+1 >= len(lines) {
				break
			}
			n++
			line += strings.TrimLeftFunc(lines[n], func(r rune) bool { return r < 0x80 && isSpace(byte(r)) })
		}
		// The key ends at the first unescaped '=', ':' or white space.
		i := 0
		for i < len(line) {
			c := line[i]
			if c == '\\' {
				i += 2
				continue
			}
			if c == '=' || c == ':' || isSpace(c) {
				break
			}
			i++
		}
		if i > len(line) {
			i = len(line)
		}
		key := line[:i]
		for i < len(line) && isSpace(line[i]) {
			i++
		}
		if i < len(line) && (line[i] == '=' || line[i] == ':') {
			i++
			for i < len(line) && isSpace(line[i]) {
				i++
			}
		}
		put(unescapeProperty_(key), unescapeProperty_(line[i:]))
	}
}

func unescapeProperty_(s string) string {
	if strings.IndexByte(s, '\\') < 0 {
		return s
	}
	var units []uint16
	var b []byte
	flush := func() {
		if len(units) > 0 {
			b = append(b, fromUnits_(units)...)
			units = units[:0]
		}
	}
	for i := 0; i < len(s); i++ {
		c := s[i]
		if c != '\\' {
			flush()
			b = append(b, c)
			continue
		}
		i++
		if i >= len(s) {
			break
		}
		switch c = s[i]; c {
		case 't':
			flush()
			b = append(b, '\t')
		case 'n':
			flush()
			b = append(b, '\n')
		case 'r':
			flush()
			b = append(b, '\r')
		case 'f':
			flush()
			b = append(b, '\f')
		case 'u':
			if i+4 >= len(s) {
				panic(new_IllegalArgumentException_String("Malformed \\uxxxx encoding.").self())
			}
			var u uint16
			for _, h := range []byte(s[i+1 : i+5]) {
				switch {
				case h >= '0' && h <= '9':
					u = u<<4 | uint16(h-'0')
				case h >= 'a' && h <= 'f':
					u = u<<4 | uint16(h-'a'+10)
				case h >= 'A' && h <= 'F':
					u = u<<4 | uint16(h-'A'+10)
				default:
					panic(new_IllegalArgumentException_String("Malformed \\uxxxx encoding.").self())
				}
			}
			i += 4
			units = append(units, u) // surrogate pairs are joined by fromUnits_
		default:
			flush()
			b = append(b, c)
		}
	}
	flush()
	return string(b)
}
