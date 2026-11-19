package main

// A reader of Java class declarations, enough to tell which members of a generic class are
// declared with its type variables: the type parameters of classes and methods, and the declared
// types of fields, record components, method parameters and results. Method bodies and field
// initializers are skipped.

import (
	"os"
	"strings"
)

type jType struct {
	name string   // "K", "Map", "java.util.Map", "?"
	args []*jType // type arguments
	dims int      // array dimensions (varargs count as one)
}

func (t *jType) String() string {
	if t == nil {
		return "<nil>"
	}
	s := t.name
	if len(t.args) > 0 {
		var as []string
		for _, a := range t.args {
			as = append(as, a.String())
		}
		s += "<" + strings.Join(as, ", ") + ">"
	}
	return s + strings.Repeat("[]", t.dims)
}

type jParam struct {
	name string
	typ  *jType
}

type jMember struct {
	kind    string // "field", "method", "ctor"
	name    string
	typ     *jType // field type or method result (nil for a constructor)
	params  []jParam
	tparams []string
	static  bool
}

type jClass struct {
	bounded   bool // a type parameter has a bound (extends)
	name      string
	kind      string // class, interface, enum, record
	tparams   []string
	static    bool
	members   []*jMember
	nested    map[string]*jClass
	order     []string // nested names in source order
	superName string   // the class this one extends, if any
	ifaces    []string
}

// ---- tokens

type jtok struct {
	s     string
	ident bool
}

func jTokens(src string) []jtok {
	var out []jtok
	i := 0
	n := len(src)
	isID := func(c byte) bool {
		return c == '_' || c == '$' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9' || c >= 0x80
	}
	for i < n {
		c := src[i]
		switch {
		case c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\f':
			i++
		case strings.HasPrefix(src[i:], "//"):
			for i < n && src[i] != '\n' {
				i++
			}
		case strings.HasPrefix(src[i:], "/*"):
			j := strings.Index(src[i+2:], "*/")
			if j < 0 {
				i = n
			} else {
				i += j + 4
			}
		case strings.HasPrefix(src[i:], `"""`):
			j := strings.Index(src[i+3:], `"""`)
			if j < 0 {
				i = n
			} else {
				i += j + 6
			}
			out = append(out, jtok{s: `"..."`})
		case c == '"' || c == '\'':
			j := i + 1
			for j < n && src[j] != c {
				if src[j] == '\\' {
					j++
				}
				j++
			}
			i = j + 1
			out = append(out, jtok{s: `"..."`})
		case isID(c):
			j := i
			for j < n && isID(src[j]) {
				j++
			}
			out = append(out, jtok{s: src[i:j], ident: !(c >= '0' && c <= '9')})
			i = j
		case strings.HasPrefix(src[i:], "..."):
			out = append(out, jtok{s: "..."})
			i += 3
		case strings.HasPrefix(src[i:], "::") || strings.HasPrefix(src[i:], "->"):
			out = append(out, jtok{s: src[i : i+2]})
			i += 2
		default:
			out = append(out, jtok{s: string(c)})
			i++
		}
	}
	return out
}

type jparser struct {
	t []jtok
	i int
}

func (p *jparser) peek(k int) string {
	if p.i+k < len(p.t) {
		return p.t[p.i+k].s
	}
	return ""
}

func (p *jparser) isIdent(k int) bool {
	return p.i+k < len(p.t) && p.t[p.i+k].ident
}

func (p *jparser) next() string {
	s := p.peek(0)
	p.i++
	return s
}

// skipBalanced skips from an opening token to just after its matching close.
func (p *jparser) skipBalanced(open, close string) {
	depth := 0
	for p.i < len(p.t) {
		s := p.next()
		if s == open {
			depth++
		} else if s == close {
			depth--
			if depth == 0 {
				return
			}
		}
	}
}

func (p *jparser) skipAnnotation() {
	p.next() // @
	p.next() // name
	for p.peek(0) == "." && p.isIdent(1) {
		p.i += 2
	}
	if p.peek(0) == "(" {
		p.skipBalanced("(", ")")
	}
}

var jModifiers = map[string]bool{"public": true, "private": true, "protected": true, "static": true, "final": true,
	"abstract": true, "native": true, "synchronized": true, "transient": true, "volatile": true, "strictfp": true,
	"default": true, "sealed": true, "non-sealed": true}

// modifiers skips annotations and modifiers and reports whether static was among them.
func (p *jparser) modifiers() bool {
	static := false
	for {
		switch {
		case p.peek(0) == "@" && p.peek(1) != "interface":
			p.skipAnnotation()
		case p.peek(0) == "non" && p.peek(1) == "-" && p.peek(2) == "sealed":
			p.i += 3
		case jModifiers[p.peek(0)]:
			if p.next() == "static" {
				static = true
			}
		default:
			return static
		}
	}
}

// typeParams parses <A extends B, C> into the names.
func (p *jparser) typeParams() []string {
	var names []string
	if p.peek(0) != "<" {
		return nil
	}
	p.next()
	depth := 1
	expectName := true
	for p.i < len(p.t) && depth > 0 {
		s := p.next()
		switch s {
		case "<":
			depth++
		case ">":
			depth--
		case ",":
			if depth == 1 {
				expectName = true
			}
		case "@":
			p.i--
			p.skipAnnotation()
		default:
			if expectName && depth == 1 {
				names = append(names, s)
				expectName = false
			}
		}
	}
	return names
}

func (p *jparser) typ() *jType {
	for p.peek(0) == "@" {
		p.skipAnnotation()
	}
	if p.peek(0) == "?" {
		p.next()
		t := &jType{name: "?"}
		if p.peek(0) == "extends" || p.peek(0) == "super" {
			kw := p.next()
			b := p.typ()
			if kw == "extends" {
				t.args = []*jType{b}
			}
		}
		return t
	}
	if !p.isIdent(0) {
		return nil
	}
	t := &jType{name: p.next()}
	for {
		if p.peek(0) == "<" {
			p.next()
			for p.peek(0) != ">" && p.i < len(p.t) {
				a := p.typ()
				if a == nil {
					return nil
				}
				t.args = append(t.args, a)
				if p.peek(0) == "," {
					p.next()
				}
			}
			p.next() // >
		}
		if p.peek(0) == "." && (p.isIdent(1) || p.peek(1) == "@") {
			p.next()
			for p.peek(0) == "@" {
				p.skipAnnotation()
			}
			t.name += "." + p.next()
			t.args = nil
			continue
		}
		break
	}
	for {
		for p.peek(0) == "@" {
			p.skipAnnotation()
		}
		if p.peek(0) == "[" && p.peek(1) == "]" {
			p.i += 2
			t.dims++
			continue
		}
		if p.peek(0) == "..." {
			p.next()
			t.dims++
		}
		break
	}
	return t
}

func (p *jparser) params() []jParam {
	var out []jParam
	if p.next() != "(" {
		return nil
	}
	for p.peek(0) != ")" && p.i < len(p.t) {
		p.modifiers()
		t := p.typ()
		if t == nil {
			p.skipTo(")")
			return out
		}
		if p.peek(0) == "this" { // receiver parameter
			p.next()
		} else {
			out = append(out, jParam{name: p.next(), typ: t})
		}
		for p.peek(0) == "[" {
			p.i += 2
			t.dims++
		}
		if p.peek(0) == "," {
			p.next()
		}
	}
	p.next() // )
	return out
}

func (p *jparser) skipTo(s string) {
	for p.i < len(p.t) && p.peek(0) != s {
		switch p.peek(0) {
		case "(":
			p.skipBalanced("(", ")")
		case "{":
			p.skipBalanced("{", "}")
		case "[":
			p.skipBalanced("[", "]")
		default:
			p.next()
		}
	}
}

// typeDecl parses a class, interface, enum or record declaration from its keyword.
func (p *jparser) typeDecl(static bool) *jClass {
	kw := p.next()
	if kw == "@" {
		kw = "@" + p.next()
	}
	c := &jClass{kind: kw, name: p.next(), static: static, nested: map[string]*jClass{}}
	start := p.i
	c.tparams = p.typeParams()
	for k := start; k < p.i; k++ {
		if p.t[k].s == "extends" {
			c.bounded = true
		}
	}
	if kw == "record" && p.peek(0) == "(" {
		comps := p.params()
		for _, pr := range comps {
			c.members = append(c.members, &jMember{kind: "field", name: pr.name, typ: pr.typ},
				&jMember{kind: "method", name: pr.name, typ: pr.typ})
		}
		c.members = append(c.members, &jMember{kind: "ctor", name: c.name, params: comps})
	}
	for p.peek(0) != "{" && p.i < len(p.t) {
		switch p.next() {
		case "extends":
			if t := p.typ(); t != nil {
				if kw == "class" {
					c.superName = t.name
				} else {
					c.ifaces = append(c.ifaces, t.name)
				}
			}
		case "implements":
			if t := p.typ(); t != nil {
				c.ifaces = append(c.ifaces, t.name)
			}
		case ",":
			if t := p.typ(); t != nil {
				c.ifaces = append(c.ifaces, t.name)
			}
		}
	}
	p.body(c)
	return c
}

func (p *jparser) body(c *jClass) {
	p.next() // {
	if c.kind == "enum" {
		// constants up to the first ; at depth 0 (or the end of the body)
		for p.i < len(p.t) && p.peek(0) != ";" && p.peek(0) != "}" {
			switch p.peek(0) {
			case "(":
				p.skipBalanced("(", ")")
			case "{":
				p.skipBalanced("{", "}")
			default:
				p.next()
			}
		}
	}
	for p.i < len(p.t) {
		switch p.peek(0) {
		case "}":
			p.next()
			return
		case ";":
			p.next()
			continue
		}
		static := p.modifiers()
		switch {
		case p.peek(0) == "{":
			p.skipBalanced("{", "}")
			continue
		case p.peek(0) == "class" || p.peek(0) == "interface" || p.peek(0) == "enum" || p.peek(0) == "@" ||
			p.peek(0) == "record" && p.isIdent(1) && (p.peek(2) == "(" || p.peek(2) == "<"):
			n := p.typeDecl(static || c.kind == "interface")
			if n.kind != "class" {
				n.static = true
			}
			c.nested[n.name] = n
			c.order = append(c.order, n.name)
			continue
		}
		m := &jMember{static: static || c.kind == "interface" && false}
		m.tparams = p.typeParams()
		if p.peek(0) == c.name && p.peek(1) == "(" {
			p.next()
			m.kind, m.name = "ctor", c.name
			m.params = p.params()
		} else if c.kind == "record" && p.peek(0) == c.name && p.peek(1) == "{" {
			// compact canonical constructor
			p.next()
			p.skipBalanced("{", "}")
			continue
		} else {
			t := p.typ()
			if t == nil || !p.isIdent(0) {
				p.skipTo(";")
				continue
			}
			name := p.next()
			if p.peek(0) == "(" {
				m.kind, m.name, m.typ = "method", name, t
				m.params = p.params()
				for p.peek(0) == "[" {
					p.i += 2
				}
			} else {
				m.kind, m.name, m.typ = "field", name, t
				c.members = append(c.members, m)
				// further declarators without initializers: , name
				for {
					for p.peek(0) == "[" {
						p.i += 2
					}
					if p.peek(0) == "=" {
						p.skipTo(";")
						break
					}
					if p.peek(0) == "," && p.isIdent(1) {
						p.next()
						c.members = append(c.members, &jMember{kind: "field", name: p.next(), typ: t, static: static})
						continue
					}
					break
				}
				p.skipTo(";")
				p.next()
				continue
			}
		}
		c.members = append(c.members, m)
		// throws clause, then a body or ;
		for p.i < len(p.t) && p.peek(0) != "{" && p.peek(0) != ";" {
			p.next()
		}
		if p.peek(0) == "{" {
			p.skipBalanced("{", "}")
		} else {
			p.next()
		}
	}
}

// javaFile is a parsed Java file: its imports and top-level type declarations.
type javaFile struct {
	pkg     string
	imports []string
	types   map[string]*jClass
}

var javaFiles = map[string]*javaFile{}

func parseJava(path string) *javaFile {
	if f, ok := javaFiles[path]; ok {
		return f
	}
	b, err := os.ReadFile(path)
	if err != nil {
		javaFiles[path] = nil
		return nil
	}
	p := &jparser{t: jTokens(string(b))}
	f := &javaFile{types: map[string]*jClass{}}
	for p.i < len(p.t) {
		switch p.peek(0) {
		case "package":
			p.next()
			var s []string
			for p.peek(0) != ";" {
				s = append(s, p.next())
			}
			f.pkg = strings.Join(s, "")
			p.next()
			continue
		case "import":
			p.next()
			var s []string
			for p.peek(0) != ";" {
				s = append(s, p.next())
			}
			f.imports = append(f.imports, strings.Join(s, ""))
			p.next()
			continue
		case ";":
			p.next()
			continue
		}
		static := p.modifiers()
		switch p.peek(0) {
		case "class", "interface", "enum", "record", "@":
			c := p.typeDecl(static)
			f.types[c.name] = c
		default:
			p.next()
		}
	}
	javaFiles[path] = f
	return f
}
