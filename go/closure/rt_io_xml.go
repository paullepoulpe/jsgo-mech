package closure

// The SAX API (javax.xml.parsers, org.xml.sax) on Go's encoding/xml: a parser
// that is not namespace aware and does not validate, which is how the JDK's
// default SAXParserFactory starts out.

import (
	"bytes"
	"encoding/xml"
	"io"
	"regexp"
	"strconv"
	"strings"
)

var xmlEncodingDecl_ = regexp.MustCompile(`^<\?xml[^>]*?encoding\s*=\s*["']([A-Za-z0-9._-]+)["']`)

func init() {
	javaNames_["SAXParseException"] = "org.xml.sax.SAXParseException"
}

// SAXParseException is org.xml.sax.SAXParseException.
type SAXParseException struct {
	SAXException
	line_ int32
}

func new_SAXParseException_(msg string, line int32) *SAXParseException {
	this := &SAXParseException{line_: line}
	this.self_ = this
	this.init_SAXException_String(msg)
	return this
}

// ---- InputSource

// InputSource is the document to parse: bytes, or text that is decoded already.
type InputSource struct {
	data_  []byte
	chars_ bool
}

func new_InputSource(in *InputStream) *InputSource { return &InputSource{data_: in.bytes_} }

func new_InputSource_Reader(r *Reader) *InputSource {
	return &InputSource{data_: []byte(r.self_.(*StringReader).rest_()), chars_: true}
}

// ---- SAXParserFactory, SAXParser

type SAXParserFactory struct {
	features_ map[string]bool
}

func SAXParserFactory_newInstance() *SAXParserFactory {
	return &SAXParserFactory{features_: map[string]bool{}}
}

func (this *SAXParserFactory) setFeature(name string, value bool) {
	if name == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	this.features_[name] = value
}

func (this *SAXParserFactory) setValidating(on bool)    {}
func (this *SAXParserFactory) setXIncludeAware(on bool) {}
func (this *SAXParserFactory) newSAXParser() *SAXParser {
	return &SAXParser{reader_: &xmlReader_{}}
}

type SAXParser struct {
	reader_ *xmlReader_
}

// getXMLReader returns the parser's one reader.
func (this *SAXParser) getXMLReader() XMLReader { return this.reader_ }

// ---- Attributes

type saxAttributes_ struct {
	names_  []string
	values_ []string
}

func (this *saxAttributes_) getIndex(qName string) int32 {
	for i, n := range this.names_ {
		if n == qName {
			return int32(i)
		}
	}
	return -1
}

// getValue is the value of the attribute with the qualified name, or null.
func (this *saxAttributes_) getValue(qName string) string {
	if i := this.getIndex(qName); i >= 0 {
		return this.values_[i]
	}
	return nullstr_
}

func (this *saxAttributes_) getType(i int32) string {
	if i < 0 || int(i) >= len(this.names_) {
		return nullstr_
	}
	return "CDATA"
}

// ---- XMLReader

type xmlReader_ struct {
	handler_  any
	resolver_ any
}

func (this *xmlReader_) setContentHandler(h ContentHandler) { this.handler_ = h }
func (this *xmlReader_) setEntityResolver(r EntityResolver) { this.resolver_ = r }
func qname_(n xml.Name) string {
	if n.Space != "" {
		return n.Space + ":" + n.Local
	}
	return n.Local
}

func (this *xmlReader_) parse(source *InputSource) {
	if source == nil {
		panic(new_NullPointerException_0().self())
	}
	data, chars := source.data_, source.chars_
	// A UTF-16 document is recognised by its byte order mark.
	if !chars && len(data) >= 2 && (data[0] == 0xFE && data[1] == 0xFF || data[0] == 0xFF && data[1] == 0xFE) {
		s, _ := charsets_[csUTF16_].decode(data)
		data = []byte(s)
		chars = true
	}
	data = bytes.TrimPrefix(data, []byte{0xEF, 0xBB, 0xBF})
	// A document in another declared encoding is decoded up front.
	if m := xmlEncodingDecl_.FindSubmatch(data); !chars && m != nil {
		cs := charsetByName_[strings.ToLower(string(m[1]))]
		if cs == nil {
			panic(new_SAXParseException_("Invalid encoding name \""+string(m[1])+"\".", 1).self())
		}
		if cs.kind != csUTF8_ {
			s, _ := cs.decode(data)
			data = []byte(s)
		}
	}

	h := this.handler_
	d := xml.NewDecoder(bytes.NewReader(data))
	// The text is decoded already, whatever its declaration says.
	d.CharsetReader = func(label string, input io.Reader) (io.Reader, error) { return input, nil }
	// A fatal error is printed, as the JDK parser does when it has no error
	// handler, and thrown.
	fail := func(msg string) {
		line, col := d.InputPos()
		Io_stderr().println("[Fatal Error] :" + istr_(int64(line)) + ":" + istr_(int64(col)) + ": " + msg)
		panic(new_SAXParseException_(msg, int32(line)).self())
	}
	if hh, ok := h.(interface{ startDocument() }); ok {
		hh.startDocument()
	}
	var stack []string
	sawRoot := false
	for {
		start := d.InputOffset()
		tok, err := d.RawToken()
		if err == io.EOF {
			break
		}
		if err != nil {
			fail(err.Error())
		}
		raw := data[start:d.InputOffset()]
		switch t := tok.(type) {
		case xml.StartElement:
			if len(stack) == 0 && sawRoot {
				fail("The markup in the document following the root element must be well-formed.")
			}
			sawRoot = true
			name := qname_(t.Name)
			atts := &saxAttributes_{}
			for _, a := range t.Attr {
				an := qname_(a.Name)
				if atts.getIndex(an) >= 0 {
					fail("Attribute \"" + an + "\" was already specified for element \"" + name + "\".")
				}
				atts.names_ = append(atts.names_, an)
				// Attribute values are normalized: white space becomes a space.
				atts.values_ = append(atts.values_, strings.NewReplacer("\n", " ", "\t", " ", "\r", " ").Replace(a.Value))
			}
			stack = append(stack, name)
			if hh, ok := h.(interface {
				startElement(string, string, string, Attributes)
			}); ok {
				hh.startElement("", "", name, atts)
			}
		case xml.EndElement:
			name := qname_(t.Name)
			if len(stack) == 0 || stack[len(stack)-1] != name {
				open := ""
				if len(stack) > 0 {
					open = stack[len(stack)-1]
				}
				fail("The element type \"" + open + "\" must be terminated by the matching end-tag \"</" + open + ">\".")
			}
			stack = stack[:len(stack)-1]
			if hh, ok := h.(interface{ endElement(string, string, string) }); ok {
				hh.endElement("", "", name)
			}
		case xml.CharData:
			if len(stack) == 0 {
				if len(bytes.TrimSpace(t)) > 0 {
					if sawRoot {
						fail("Content is not allowed in trailing section.")
					}
					fail("Content is not allowed in prolog.")
				}
				continue
			}
			hh, ok := h.(interface{ characters([]uint16, int32, int32) })
			if !ok {
				continue
			}
			for _, chunk := range saxChunks_(raw, []byte(t)) {
				u := String_toCharArray(chunk)
				hh.characters(u, 0, int32(len(u)))
			}
		case xml.ProcInst:
			if t.Target == "xml" {
				continue
			}
			if hh, ok := h.(interface{ processingInstruction(string, string) }); ok {
				hh.processingInstruction(t.Target, string(t.Inst))
			}
		case xml.Directive:
			this.doctype_(string(t))
		case xml.Comment:
		}
	}
	if len(stack) > 0 {
		fail("XML document structures must start and end within the same entity.")
	}
	if !sawRoot {
		fail("Premature end of file.")
	}
	if hh, ok := h.(interface{ endDocument() }); ok {
		hh.endDocument()
	}
}

// doctype_ offers the external subset of a document type declaration to the
// entity resolver, as a parser does before it would load it. Nothing is
// loaded: the declaration has no further effect here.
func (this *xmlReader_) doctype_(decl string) {
	if !strings.HasPrefix(decl, "DOCTYPE") {
		return
	}
	if i := strings.IndexByte(decl, '['); i >= 0 {
		decl = decl[:i]
	}
	fields := strings.Fields(decl)
	if len(fields) < 3 {
		return
	}
	// The quoted literals after SYSTEM or PUBLIC.
	var lits []string
	rest := decl
	for {
		i := strings.IndexAny(rest, "\"'")
		if i < 0 {
			break
		}
		j := strings.IndexByte(rest[i+1:], rest[i])
		if j < 0 {
			break
		}
		lits = append(lits, rest[i+1:i+1+j])
		rest = rest[i+j+2:]
	}
	publicId, systemId := nullstr_, nullstr_
	switch {
	case fields[2] == "SYSTEM" && len(lits) >= 1:
		systemId = lits[0]
	case fields[2] == "PUBLIC" && len(lits) >= 2:
		publicId, systemId = lits[0], lits[1]
	default:
		return
	}
	// The parser passes the system identifier as an absolute URI.
	if !strings.Contains(systemId, ":") {
		if strings.HasPrefix(systemId, "/") {
			systemId = "file://" + systemId
		} else {
			systemId = "file://" + Io_pathToAbsolute(systemId)
		}
	}
	if r, ok := this.resolver_.(interface {
		resolveEntity(string, string) *InputSource
	}); ok {
		r.resolveEntity(publicId, systemId)
	}
}

// saxChunks_ splits character data into the pieces the JDK's parser reports
// with separate characters() calls. raw is the source text of the data and
// decoded its value. Each character or entity reference is a piece of its
// own, and the text between is cut as that parser scans it (see textEvents_).
// (The JDK also cuts where its 8K input buffer ends; that is not reproduced.)
func saxChunks_(raw []byte, decoded []byte) []string {
	if bytes.HasPrefix(raw, []byte("<![CDATA[")) {
		if len(decoded) == 0 {
			return nil
		}
		return []string{normalizeEOL_(string(decoded))}
	}
	var out []string
	s := string(raw)
	for len(s) > 0 {
		i := strings.IndexByte(s, '&')
		if i < 0 {
			out = textEvents_(out, normalizeEOL_(s))
			break
		}
		if i > 0 {
			out = textEvents_(out, normalizeEOL_(s[:i]))
		}
		j := strings.IndexByte(s[i:], ';')
		if j < 0 {
			out = append(out, s[i:])
			break
		}
		out = append(out, xmlReference_(s[i+1:i+j]))
		s = s[i+j+1:]
	}
	return out
}

func normalizeEOL_(s string) string {
	if strings.IndexByte(s, '\r') < 0 {
		return s
	}
	return strings.ReplaceAll(strings.ReplaceAll(s, "\r\n", "\n"), "\r", "\n")
}

// textEvents_ appends the pieces of a run of plain text. The JDK's scanner
// reads a run in scans, each taking the line ends in front of it and then
// text up to the next line end or ']'; a ']' (and those that follow it) is
// added to the scan that met it. One event is a scan, plus a second one if
// the first did not end the run.
func textEvents_(out []string, s string) []string {
	pos := 0
	scan := func() (more bool) {
		for pos < len(s) && s[pos] == '\n' {
			pos++
		}
		for pos < len(s) && s[pos] != '\n' && s[pos] != ']' {
			pos++
		}
		if pos < len(s) && s[pos] == ']' {
			for pos < len(s) && s[pos] == ']' {
				pos++
			}
		}
		return pos < len(s) || s[len(s)-1] == ']'
	}
	for pos < len(s) {
		start := pos
		if scan() && pos < len(s) {
			scan()
		}
		out = append(out, s[start:pos])
	}
	return out
}

func xmlReference_(name string) string {
	switch name {
	case "lt":
		return "<"
	case "gt":
		return ">"
	case "amp":
		return "&"
	case "apos":
		return "'"
	case "quot":
		return "\""
	}
	if strings.HasPrefix(name, "#") {
		digits, base := name[1:], 10
		if strings.HasPrefix(digits, "x") {
			digits, base = digits[1:], 16
		}
		if cp, err := strconv.ParseUint(digits, base, 21); err == nil {
			return string(appendCP_(nil, int32(cp)))
		}
	}
	return ""
}

func (this *SAXParseException) getLineNumber() int32 { return this.line_ }
