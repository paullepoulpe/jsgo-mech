package closure

// org.kohsuke.args4j: the command-line parser behind CommandLineRunner's flags.
//
// Java args4j finds a bean's @Option/@Argument fields by reflection. Here the
// translator emits, for each such class, args4jFields_(): one row per annotated
// field with the annotation's values, a getter and a setter.

import (
	"sort"
	"strings"
)

type args4jField_ struct {
	field      string
	option     bool
	name       string
	aliases    []string
	usage      string
	metaVar    string
	required   bool
	hidden     bool
	multi      bool
	typ        string // Java type of the value (the element type for a collection field)
	handler    string
	newHandler func(p *CmdLineParser, o *OptionDef, s Setter) any
	get        func() any
	set        func(v any)
}

type OptionDef struct {
	self_       any
	usage_      string
	metaVar_    string
	required_   bool
	hidden_     bool
	multiValued bool
}

func (this *OptionDef) as_OptionDef() *OptionDef { return this }

func to_OptionDef(v any) *OptionDef {
	if v == nil {
		return nil
	}
	return v.(interface{ as_OptionDef() *OptionDef }).as_OptionDef()
}

func is_OptionDef(v any) bool {
	_, ok := v.(interface{ as_OptionDef() *OptionDef })
	return ok
}

func (this *OptionDef) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *OptionDef) usage() string       { return this.usage_ }
func (this *OptionDef) metaVar() string     { return this.metaVar_ }
func (this *OptionDef) required() bool      { return this.required_ }
func (this *OptionDef) hidden() bool        { return this.hidden_ }
func (this *OptionDef) isMultiValued() bool { return this.multiValued }
func (this *OptionDef) isArgument() bool    { _, named := this.self_.(*NamedOptionDef); return !named }
func (this *OptionDef) toString() string {
	if n, ok := this.self_.(*NamedOptionDef); ok {
		return n.name_
	}
	return "argument"
}

type NamedOptionDef struct {
	OptionDef
	name_    string
	aliases_ []string
}

func (this *NamedOptionDef) up_OptionDef() *OptionDef {
	if this == nil {
		return nil
	}
	return &this.OptionDef
}

func (this *NamedOptionDef) name() string      { return this.name_ }
func (this *NamedOptionDef) aliases() []string { return this.aliases_ }

// FieldSetter stores parsed values into one field of the bean.
type FieldSetter struct {
	f *args4jField_
}

func (this *FieldSetter) addValue(v any) {
	if !this.f.multi {
		this.f.set(v)
		return
	}
	l := this.f.get()
	if l == nil {
		l = new_ArrayList_0()
		this.f.set(l)
	}
	l.(interface{ add(any) bool }).add(v)
}

func (this *FieldSetter) asAnnotatedElement() AnnotatedElement { return nil }
func (this *FieldSetter) asFieldSetter() *FieldSetter          { return this }
func (this *FieldSetter) getType() *Class                      { return classOf_(this.f.typ) }
func (this *FieldSetter) isMultiValued() bool                  { return this.f.multi }
func (this *FieldSetter) getValue() any                        { return this.f.get() }

// OptionHandler is the base of every handler, including the ones translated
// from CommandLineRunner.
type OptionHandler struct {
	self_  any
	option *OptionDef
	setter Setter
	owner  *CmdLineParser
}

func (this *OptionHandler) as_OptionHandler() *OptionHandler { return this }

func to_OptionHandler(v any) *OptionHandler {
	if v == nil {
		return nil
	}
	return v.(interface{ as_OptionHandler() *OptionHandler }).as_OptionHandler()
}

func is_OptionHandler(v any) bool {
	_, ok := v.(interface{ as_OptionHandler() *OptionHandler })
	return ok
}

func (this *OptionHandler) self() any {
	if this == nil {
		return nil
	}
	return this.self_
}

func (this *OptionHandler) init_OptionHandler(p *CmdLineParser, o *OptionDef, s Setter) {
	this.owner, this.option, this.setter = p, o, s
}

func (this *OptionHandler) getDefaultMetaVariable() string { return nullstr_ }

type handler_ interface {
	parseArguments(Parameters) int32
	getDefaultMetaVariable() string
	as_OptionHandler() *OptionHandler
}

type StringOptionHandler struct{ OptionHandler }

func (this *StringOptionHandler) as_StringOptionHandler() *StringOptionHandler { return this }

func to_StringOptionHandler(v any) *StringOptionHandler {
	if v == nil {
		return nil
	}
	return v.(interface{ as_StringOptionHandler() *StringOptionHandler }).as_StringOptionHandler()
}

func is_StringOptionHandler(v any) bool {
	_, ok := v.(interface{ as_StringOptionHandler() *StringOptionHandler })
	return ok
}

func (this *StringOptionHandler) up_OptionHandler() *OptionHandler {
	if this == nil {
		return nil
	}
	return &this.OptionHandler
}

func (this *StringOptionHandler) init_StringOptionHandler(p *CmdLineParser, o *OptionDef, s Setter) {
	this.init_OptionHandler(p, o, s)
}

func (this *StringOptionHandler) parseArguments(params Parameters) int32 {
	this.setter.addValue(strobj_(params.getParameter(0)))
	return 1
}

func (this *StringOptionHandler) getDefaultMetaVariable() string { return "VAL" }

// builtinHandler_ covers args4j's own handlers for int, boolean and enum fields.
type builtinHandler_ struct {
	OptionHandler
	typ string
}

func (this *builtinHandler_) getDefaultMetaVariable() string {
	switch this.typ {
	case "int", "java.lang.Integer", "long", "java.lang.Long":
		return "N"
	case "boolean", "java.lang.Boolean":
		return nullstr_
	}
	if vs, ok := enumRegistry_[this.typ]; ok {
		names := make([]string, len(vs))
		for i, v := range vs {
			names[i] = objstr_(v)
		}
		return "[" + strings.Join(names, " | ") + "]"
	}
	return "VAL"
}

func (this *builtinHandler_) parseArguments(params Parameters) int32 {
	name := this.option.toString()
	switch this.typ {
	case "boolean", "java.lang.Boolean":
		var p string
		func() {
			defer func() { recover() }()
			p = params.getParameter(0)
		}()
		switch strings.ToLower(p) {
		case "true", "on", "yes", "1":
			this.setter.addValue(true)
			return 1
		case "false", "off", "no", "0":
			this.setter.addValue(false)
			return 1
		}
		this.setter.addValue(true)
		return 0
	case "int", "java.lang.Integer":
		p := params.getParameter(0)
		var v int32
		ok := func() (ok bool) {
			defer func() {
				if recover() != nil {
					ok = false
				}
			}()
			v = Integer_parseInt(p)
			return true
		}()
		if !ok {
			panic(new_CmdLineException_CmdLineParser_String(this.owner, "\""+p+"\" is not a valid value for \""+name+"\"").self())
		}
		this.setter.addValue(v)
		return 1
	}
	if vs, ok := enumRegistry_[this.typ]; ok {
		p := params.getParameter(0)
		for _, v := range vs {
			if strings.EqualFold(v.(interface{ name() string }).name(), p) {
				this.setter.addValue(v)
				return 1
			}
		}
		panic(new_CmdLineException_CmdLineParser_String(this.owner, "\""+p+"\" is not a valid value for \""+name+"\"").self())
	}
	this.setter.addValue(strobj_(params.getParameter(0)))
	return 1
}

type CmdLineException struct {
	Exception
	parser *CmdLineParser
}

func (this *CmdLineException) up_Exception() *Exception {
	if this == nil {
		return nil
	}
	return &this.Exception
}

func new_CmdLineException_CmdLineParser_String(p *CmdLineParser, msg string) *CmdLineException {
	this := &CmdLineException{parser: p}
	this.self_ = this
	this.init_Exception_String(msg)
	return this
}

func (this *CmdLineException) getParser() *CmdLineParser { return this.parser }

type CmdLineParser struct {
	options    []handler_
	arguments  []handler_
	usageWidth int32
}

func new_CmdLineParser(bean any) *CmdLineParser {
	this := &CmdLineParser{usageWidth: 80}
	src, ok := bean.(interface{ args4jFields_() []*args4jField_ })
	if !ok {
		return this
	}
	for _, f := range src.args4jFields_() {
		var def *OptionDef
		if f.option {
			nd := &NamedOptionDef{name_: f.name, aliases_: f.aliases}
			nd.self_ = nd
			def = &nd.OptionDef
		} else {
			def = &OptionDef{}
			def.self_ = def
		}
		def.usage_, def.metaVar_, def.required_, def.hidden_, def.multiValued = f.usage, f.metaVar, f.required, f.hidden, f.multi
		setter := &FieldSetter{f: f}
		var h handler_
		switch {
		case f.newHandler != nil:
			h = f.newHandler(this, def, setter).(handler_)
		case f.typ == "java.lang.String" || f.handler == "org.kohsuke.args4j.spi.StringOptionHandler":
			sh := &StringOptionHandler{}
			sh.self_ = sh
			sh.init_StringOptionHandler(this, def, setter)
			h = sh
		default:
			bh := &builtinHandler_{typ: f.typ}
			bh.self_ = bh
			bh.init_OptionHandler(this, def, setter)
			h = bh
		}
		if f.option {
			this.options = append(this.options, h)
		} else {
			this.arguments = append(this.arguments, h)
		}
	}
	return this
}

func (this *CmdLineParser) setUsageWidth(w int32) { this.usageWidth = w }

func (this *CmdLineParser) findOption(name string) handler_ {
	for _, h := range this.options {
		nd := h.as_OptionHandler().option.self_.(*NamedOptionDef)
		if nd.name_ == name {
			return h
		}
		for _, a := range nd.aliases_ {
			if a == name {
				return h
			}
		}
	}
	return nil
}

// cmdLine_ is the Parameters a handler reads its operands from.
type cmdLine_ struct {
	owner  *CmdLineParser
	args   []string
	pos    int
	option string
}

func (c *cmdLine_) getParameter(i int32) string {
	if c.pos+int(i) >= len(c.args) || c.pos+int(i) < 0 {
		panic(new_CmdLineException_CmdLineParser_String(c.owner, "Option \""+c.option+"\" takes an operand").self())
	}
	return c.args[c.pos+int(i)]
}

func (c *cmdLine_) size() int32 { return int32(len(c.args) - c.pos) }

func (this *CmdLineParser) parseArgument_Collection(c Collection) {
	var args []string
	for it := c.iterator(); it.hasNext(); {
		args = append(args, str_(it.next()))
	}
	this.parseArgument(args)
}

func (this *CmdLineParser) parseArgument(args []string) {
	cl := &cmdLine_{owner: this, args: append([]string(nil), args...)}
	present := map[handler_]bool{}
	argIndex := 0
	parseOptions := true
	for cl.pos < len(cl.args) {
		arg := cl.args[cl.pos]
		var h handler_
		if parseOptions && strings.HasPrefix(arg, "-") {
			if arg == "--" {
				parseOptions = false
				cl.pos++
				continue
			}
			name := arg
			if i := strings.IndexByte(arg, '='); i > 0 && this.findOption(arg) == nil {
				name = arg[:i]
				// Split the token into the name and its operand.
				rest := append([]string{arg[i+1:]}, cl.args[cl.pos+1:]...)
				cl.args = append(append(cl.args[:cl.pos:cl.pos], name), rest...)
			}
			h = this.findOption(name)
			if h == nil {
				panic(new_CmdLineException_CmdLineParser_String(this, "\""+arg+"\" is not a valid option").self())
			}
			cl.option = name
			cl.pos++
		} else {
			if argIndex >= len(this.arguments) {
				msg := "Too many arguments: \"" + arg + "\""
				if len(this.arguments) == 0 {
					msg = "No argument is allowed: " + arg
				}
				panic(new_CmdLineException_CmdLineParser_String(this, msg).self())
			}
			h = this.arguments[argIndex]
			if !h.as_OptionHandler().option.multiValued {
				argIndex++
			}
			cl.option = arg
		}
		cl.pos += int(h.parseArguments(cl))
		present[h] = true
	}
	for _, h := range this.options {
		o := h.as_OptionHandler().option
		if o.required_ && !present[h] {
			panic(new_CmdLineException_CmdLineParser_String(this, "Option \""+o.toString()+"\" is required").self())
		}
	}
	for _, h := range this.arguments {
		o := h.as_OptionHandler().option
		if o.required_ && !present[h] {
			panic(new_CmdLineException_CmdLineParser_String(this, "Argument \""+o.toString()+"\" is required").self())
		}
	}
}

// nameAndMeta_ is the left column of a usage line: "--name (-alias) METAVAR".
func nameAndMeta_(h handler_) string {
	o := h.as_OptionHandler().option
	var s string
	if nd, ok := o.self_.(*NamedOptionDef); ok {
		s = nd.name_
		if len(nd.aliases_) > 0 {
			s += " (" + strings.Join(nd.aliases_, ", ") + ")"
		}
	}
	meta := o.metaVar_
	if meta == "" {
		meta = h.getDefaultMetaVariable()
	}
	if meta != nullstr_ && meta != "" {
		if s != "" {
			s += " "
		}
		s += meta
	}
	return s
}

func wrapLines_(s string, width int) []string {
	var out []string
	for _, line := range strings.Split(s, "\n") {
		for len(line) > width {
			cut := strings.LastIndexByte(line[:width+1], ' ')
			if cut <= 0 {
				cut = width
			}
			out = append(out, strings.TrimRight(line[:cut], " "))
			line = strings.TrimLeft(line[cut:], " ")
		}
		out = append(out, line)
	}
	return out
}

// usageText_ lays the selected options out like args4j's printUsage.
func (this *CmdLineParser) usageText_(filter OptionHandlerFilter) string {
	var hs []handler_
	for _, h := range append(append([]handler_(nil), this.arguments...), this.options...) {
		oh := h.as_OptionHandler()
		if oh.option.usage_ == "" {
			continue
		}
		if filter != nil && !filter.select_(oh) {
			continue
		}
		hs = append(hs, h)
	}
	named := hs[:0:0]
	var args []handler_
	for _, h := range hs {
		if _, ok := h.as_OptionHandler().option.self_.(*NamedOptionDef); ok {
			named = append(named, h)
		} else {
			args = append(args, h)
		}
	}
	sort.SliceStable(named, func(i, j int) bool {
		return named[i].as_OptionHandler().option.toString() < named[j].as_OptionHandler().option.toString()
	})
	hs = append(args, named...)
	width := 0
	for _, h := range hs {
		if n := len(nameAndMeta_(h)); n > width {
			width = n
		}
	}
	total := int(this.usageWidth)
	if width > (total-4)/2 {
		width = (total - 4) / 2
	}
	var sb strings.Builder
	for _, h := range hs {
		left := wrapLines_(nameAndMeta_(h), width)
		right := wrapLines_(h.as_OptionHandler().option.usage_, total-4-width)
		for i := 0; i < len(left) || i < len(right); i++ {
			l, r := "", ""
			if i < len(left) {
				l = left[i]
			}
			if i < len(right) {
				r = right[i]
			}
			sb.WriteString(" " + l + strings.Repeat(" ", width-len(l)) + " : " + r + "\n")
		}
	}
	return sb.String()
}

func (this *CmdLineParser) printUsage(w *Writer, rb *ResourceBundle, filter OptionHandlerFilter) {
	writeString_(w.self(), this.usageText_(filter))
}

// writeString_ writes text to a java.io.Writer-like object.
func writeString_(w any, s string) {
	switch x := w.(type) {
	case interface{ write_String(string) }:
		x.write_String(s)
	case interface{ write(string) }:
		x.write(s)
	case interface{ append_CharSequence(any) Appendable }:
		x.append_CharSequence(s)
	default:
		panic("writeString_: " + goTypeName_(w))
	}
}
