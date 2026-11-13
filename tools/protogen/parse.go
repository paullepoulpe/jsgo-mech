package main

// A small parser for the subset of the protobuf language the Closure Compiler
// .proto files use: proto2, proto3 and edition 2023 files with messages,
// enums, oneofs, nested types, options, reserved and extension ranges.

import (
	"fmt"
	"os"
	"strconv"
	"strings"
)

type File struct {
	Path     string
	Syntax   string // "proto2", "proto3" or "editions"
	Package  string
	Options  map[string]string
	Messages []*Message
	Enums    []*Enum
}

type Message struct {
	Name       string
	FullName   string // proto full name
	File       *File
	Parent     *Message
	Fields     []*Field
	Oneofs     []string
	Messages   []*Message
	Enums      []*Enum
	Extendable bool

	Java string // Java binary name
	Go   string // Go name of the message class
	GoB  string // Go name of its Builder
	GoOB string // Go name of its OrBuilder interface
}

type Field struct {
	Name     string
	Number   int
	Label    string // "", "optional", "required", "repeated"
	TypeName string // as written
	Options  map[string]string
	Oneof    int // 1-based, 0 = none

	Kind   string // "int32", ..., "message", "enum"
	Msg    *Message
	Enum   *Enum
	Rep    bool
	Packed bool
	Pres   bool
	Utf8   bool
}

type Enum struct {
	Name     string
	FullName string
	File     *File
	Parent   *Message
	Values   []EnumValue
	Closed   bool

	Java string
	Go   string
}

type EnumValue struct {
	Name   string
	Number int
}

type lexer struct {
	path string
	src  string
	pos  int
	line int
	tok  string
	str  bool // tok is a string literal (already unquoted)
}

func (l *lexer) fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "%s:%d: %s\n", l.path, l.line, fmt.Sprintf(format, args...))
	os.Exit(1)
}

func isIdent(c byte) bool {
	return c == '_' || c >= 'a' && c <= 'z' || c >= 'A' && c <= 'Z' || c >= '0' && c <= '9'
}

func (l *lexer) next() {
	s := l.src
	for l.pos < len(s) {
		c := s[l.pos]
		switch {
		case c == '\n':
			l.line++
			l.pos++
		case c == ' ' || c == '\t' || c == '\r':
			l.pos++
		case strings.HasPrefix(s[l.pos:], "//"):
			for l.pos < len(s) && s[l.pos] != '\n' {
				l.pos++
			}
		case strings.HasPrefix(s[l.pos:], "/*"):
			end := strings.Index(s[l.pos:], "*/")
			if end < 0 {
				l.fail("unterminated comment")
			}
			l.line += strings.Count(s[l.pos:l.pos+end], "\n")
			l.pos += end + 2
		default:
			goto token
		}
	}
	l.tok, l.str = "", false
	return
token:
	c := s[l.pos]
	start := l.pos
	l.str = false
	switch {
	case c == '"' || c == '\'':
		l.pos++
		var sb strings.Builder
		for l.pos < len(s) && s[l.pos] != c {
			if s[l.pos] == '\\' {
				l.pos++
				switch e := s[l.pos]; e {
				case 'n':
					sb.WriteByte('\n')
				case 't':
					sb.WriteByte('\t')
				case 'r':
					sb.WriteByte('\r')
				case '0':
					sb.WriteByte(0)
				default:
					sb.WriteByte(e)
				}
			} else {
				sb.WriteByte(s[l.pos])
			}
			l.pos++
		}
		l.pos++
		l.tok, l.str = sb.String(), true
	case isIdent(c) || c == '.' && l.pos+1 < len(s) && isIdent(s[l.pos+1]):
		l.pos++
		for l.pos < len(s) && (isIdent(s[l.pos]) || s[l.pos] == '.') {
			l.pos++
		}
		l.tok = s[start:l.pos]
	default:
		l.pos++
		l.tok = s[start:l.pos]
	}
}

func (l *lexer) accept(tok string) bool {
	if !l.str && l.tok == tok {
		l.next()
		return true
	}
	return false
}

func (l *lexer) expect(tok string) {
	if !l.accept(tok) {
		l.fail("expected %q, found %q", tok, l.tok)
	}
}

func (l *lexer) ident() string {
	if l.str || l.tok == "" || !(isIdent(l.tok[0]) || l.tok[0] == '.') {
		l.fail("expected identifier, found %q", l.tok)
	}
	t := l.tok
	l.next()
	return t
}

func (l *lexer) number() int {
	neg := l.accept("-")
	t := l.tok
	l.next()
	n, err := strconv.ParseInt(t, 0, 64)
	if err != nil {
		l.fail("expected number, found %q", t)
	}
	if neg {
		n = -n
	}
	return int(n)
}

// optionName reads `foo`, `foo.bar` or `(custom.name).sub`.
func (l *lexer) optionName() string {
	name := ""
	for {
		if l.accept("(") {
			name += "(" + l.ident() + ")"
			l.expect(")")
		} else {
			name += l.ident()
		}
		if l.tok == "=" || l.tok == "" {
			return name
		}
	}
}

// constant reads an option value; strings are marked with a leading quote.
func (l *lexer) constant() string {
	if l.str {
		t := l.tok
		l.next()
		for l.str {
			t += l.tok
			l.next()
		}
		return "\"" + t
	}
	if l.accept("{") {
		depth := 1
		for depth > 0 {
			if l.tok == "" {
				l.fail("unterminated option value")
			}
			if !l.str && l.tok == "{" {
				depth++
			} else if !l.str && l.tok == "}" {
				depth--
			}
			l.next()
		}
		return "{}"
	}
	sign := ""
	if l.accept("-") {
		sign = "-"
	} else {
		l.accept("+")
	}
	t := l.tok
	l.next()
	return sign + t
}

func (l *lexer) option(into map[string]string) {
	name := l.optionName()
	l.expect("=")
	into[name] = l.constant()
}

func (l *lexer) fieldOptions() map[string]string {
	opts := map[string]string{}
	if l.accept("[") {
		for {
			l.option(opts)
			if !l.accept(",") {
				break
			}
		}
		l.expect("]")
	}
	return opts
}

func (l *lexer) skipStatement() {
	for l.tok != ";" || l.str {
		if l.tok == "" {
			l.fail("unexpected end of file")
		}
		l.next()
	}
	l.next()
}

func (l *lexer) skipBlock() {
	for !l.accept("{") {
		l.next()
	}
	for depth := 1; depth > 0; l.next() {
		if l.tok == "" {
			l.fail("unexpected end of file")
		}
		if !l.str && l.tok == "{" {
			depth++
		} else if !l.str && l.tok == "}" {
			depth--
		}
	}
}

func parseFile(path string) *File {
	data, err := os.ReadFile(path)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	l := &lexer{path: path, src: string(data), line: 1}
	l.next()
	f := &File{Path: path, Syntax: "proto2", Options: map[string]string{}}
	for l.tok != "" {
		switch {
		case l.accept("syntax"):
			l.expect("=")
			f.Syntax = l.tok
			l.next()
			l.expect(";")
		case l.accept("edition"):
			l.expect("=")
			f.Syntax = "editions"
			l.next()
			l.expect(";")
		case l.accept("package"):
			f.Package = l.ident()
			l.expect(";")
		case l.accept("import"):
			l.skipStatement()
		case l.accept("option"):
			l.option(f.Options)
			l.expect(";")
		case l.accept("message"):
			f.Messages = append(f.Messages, l.message(f, nil))
		case l.accept("enum"):
			f.Enums = append(f.Enums, l.enum(f, nil))
		case l.accept("extend"), l.accept("service"):
			l.skipBlock()
		case l.accept(";"):
		default:
			l.fail("unexpected %q", l.tok)
		}
	}
	return f
}

func (l *lexer) message(f *File, parent *Message) *Message {
	m := &Message{Name: l.ident(), File: f, Parent: parent}
	l.expect("{")
	for !l.accept("}") {
		switch {
		case l.accept("message"):
			m.Messages = append(m.Messages, l.message(f, m))
		case l.accept("enum"):
			m.Enums = append(m.Enums, l.enum(f, m))
		case l.accept("oneof"):
			m.Oneofs = append(m.Oneofs, l.ident())
			l.expect("{")
			for !l.accept("}") {
				if l.accept("option") {
					l.skipStatement()
					continue
				}
				fd := l.field("")
				fd.Oneof = len(m.Oneofs)
				m.Fields = append(m.Fields, fd)
			}
		case l.accept("option"):
			l.skipStatement()
		case l.accept("reserved"):
			l.skipStatement()
		case l.accept("extensions"):
			m.Extendable = true
			l.skipStatement()
		case l.accept("extend"):
			l.skipBlock()
		case l.accept(";"):
		case l.tok == "map" || strings.HasPrefix(l.tok, "map<"):
			l.fail("map fields are not supported by protogen")
		case l.tok == "group":
			l.fail("groups are not supported by protogen")
		case l.tok == "optional" || l.tok == "required" || l.tok == "repeated":
			label := l.tok
			l.next()
			if l.tok == "group" {
				l.fail("groups are not supported by protogen")
			}
			m.Fields = append(m.Fields, l.field(label))
		default:
			m.Fields = append(m.Fields, l.field(""))
		}
	}
	return m
}

func (l *lexer) field(label string) *Field {
	fd := &Field{Label: label}
	fd.TypeName = l.ident()
	fd.Name = l.ident()
	l.expect("=")
	fd.Number = l.number()
	fd.Options = l.fieldOptions()
	l.expect(";")
	return fd
}

func (l *lexer) enum(f *File, parent *Message) *Enum {
	e := &Enum{Name: l.ident(), File: f, Parent: parent}
	l.expect("{")
	for !l.accept("}") {
		switch {
		case l.accept("option"), l.accept("reserved"):
			l.skipStatement()
		case l.accept(";"):
		default:
			name := l.ident()
			l.expect("=")
			num := l.number()
			l.fieldOptions()
			l.expect(";")
			e.Values = append(e.Values, EnumValue{name, num})
		}
	}
	return e
}
