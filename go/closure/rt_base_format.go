package closure

// java.util.Formatter (String.format) and java.text.MessageFormat.

import (
	"math"
	"math/big"
	"strconv"
	"strings"
)

type formatSpec_ struct {
	text      string // the whole specifier, for messages
	index     int    // explicit argument index (1-based), 0 for ordinary, -1 for '<'
	minus     bool
	sharp     bool
	plus      bool
	space     bool
	zero      bool
	comma     bool
	paren     bool
	width     int // -1 when absent
	precision int // -1 when absent
	conv      byte
}

func formatFail_(msg string) {
	panic(new_IllegalArgumentException_String(msg).self())
}

// parseFormatSpec_ parses the specifier that starts at f[i] == '%'; it returns
// the index after it.
func parseFormatSpec_(f string, i int) (formatSpec_, int) {
	sp := formatSpec_{width: -1, precision: -1}
	j := i + 1
	// argument index
	k := j
	for k < len(f) && f[k] >= '0' && f[k] <= '9' {
		k++
	}
	if k > j && k < len(f) && f[k] == '$' {
		sp.index, _ = strconv.Atoi(f[j:k])
		j = k + 1
	} else if j < len(f) && f[j] == '<' {
		sp.index = -1
		j++
	}
flags:
	for j < len(f) {
		switch f[j] {
		case '-':
			sp.minus = true
		case '#':
			sp.sharp = true
		case '+':
			sp.plus = true
		case ' ':
			sp.space = true
		case '0':
			sp.zero = true
		case ',':
			sp.comma = true
		case '(':
			sp.paren = true
		default:
			break flags
		}
		j++
	}
	k = j
	for k < len(f) && f[k] >= '0' && f[k] <= '9' {
		k++
	}
	if k > j {
		sp.width, _ = strconv.Atoi(f[j:k])
		j = k
	}
	if j < len(f) && f[j] == '.' {
		k = j + 1
		for k < len(f) && f[k] >= '0' && f[k] <= '9' {
			k++
		}
		if k == j+1 {
			formatFail_("Format specifier '" + f[i:k] + "'")
		}
		sp.precision, _ = strconv.Atoi(f[j+1 : k])
		j = k
	}
	if j >= len(f) {
		panic(new_IllegalArgumentException_String("Conversion = '%'").self())
	}
	sp.conv = f[j]
	sp.text = f[i : j+1]
	return sp, j + 1
}

func (sp *formatSpec_) pad(s string) string {
	if sp.width < 0 {
		return s
	}
	n := int(String_length(s))
	if n >= sp.width {
		return s
	}
	if sp.minus {
		return s + strings.Repeat(" ", sp.width-n)
	}
	return strings.Repeat(" ", sp.width-n) + s
}

func groupDigits_(digits string) string {
	if len(digits) <= 3 {
		return digits
	}
	var sb strings.Builder
	first := len(digits) % 3
	if first > 0 {
		sb.WriteString(digits[:first])
	}
	for i := first; i < len(digits); i += 3 {
		if sb.Len() > 0 {
			sb.WriteByte(',')
		}
		sb.WriteString(digits[i : i+3])
	}
	return sb.String()
}

// number lays out a sign and a magnitude (digits with an optional fraction)
// according to the numeric flags and the width.
func (sp *formatSpec_) number(neg bool, mag string) string {
	if sp.comma {
		intPart, rest := mag, ""
		if p := strings.IndexByte(mag, '.'); p >= 0 {
			intPart, rest = mag[:p], mag[p:]
		}
		mag = groupDigits_(intPart) + rest
	}
	prefix, suffix := "", ""
	switch {
	case neg && sp.paren:
		prefix, suffix = "(", ")"
	case neg:
		prefix = "-"
	case sp.plus:
		prefix = "+"
	case sp.space:
		prefix = " "
	}
	if sp.zero && sp.width > 0 {
		if n := sp.width - len(prefix) - len(suffix) - len(mag); n > 0 {
			mag = strings.Repeat("0", n) + mag
		}
	}
	return sp.pad(prefix + mag + suffix)
}

// javaFixedDigits_ renders a finite non-negative value with prec fraction
// digits the way java.util.Formatter does for %f: the shortest decimal that
// identifies the value, rounded half-up.
func javaFixedDigits_(v float64, prec int, bitSize int) string {
	e := strconv.FormatFloat(v, 'e', -1, bitSize)
	mant, expStr, _ := strings.Cut(e, "e")
	exp, _ := strconv.Atoi(expStr)
	digits := strings.Replace(mant, ".", "", 1)
	point := exp + 1 // the value is 0.digits * 10^point
	return roundDecimal_(digits, point, prec)
}

// roundDecimal_ rounds 0.digits*10^point half-up to prec fraction digits.
func roundDecimal_(digits string, point int, prec int) string {
	keep := point + prec
	var d []byte
	switch {
	case keep < 0:
		d = nil
		point = -prec
	case keep >= len(digits):
		d = []byte(digits + strings.Repeat("0", keep-len(digits)))
	default:
		d = []byte(digits[:keep])
		if digits[keep] >= '5' {
			i := keep - 1
			for ; i >= 0; i-- {
				if d[i] == '9' {
					d[i] = '0'
				} else {
					d[i]++
					break
				}
			}
			if i < 0 {
				d = append([]byte{'1'}, d...)
				point++
			}
		}
	}
	// d holds point integer digits (point may be <= 0) followed by prec fraction digits.
	var intPart, frac string
	if point <= 0 {
		intPart = "0"
		frac = strings.Repeat("0", -point) + string(d)
	} else {
		intPart = string(d[:point])
		frac = string(d[point:])
	}
	if len(frac) > prec {
		frac = frac[:prec]
	}
	if prec == 0 {
		return intPart
	}
	return intPart + "." + frac
}

func formatFloatArg_(sp *formatSpec_, arg any) (float64, int, bool) {
	switch x := arg.(type) {
	case float64:
		return x, 64, true
	case float32:
		return float64(x), 64, true
	}
	return 0, 0, false
}

func badConversion_(sp *formatSpec_, arg any) {
	formatFail_(string(sp.conv) + " != " + Object_getClass(arg).getName())
}

func (sp *formatSpec_) format(arg any) string {
	switch sp.conv {
	case 's', 'S':
		s := objstr_(arg)
		if sp.precision >= 0 && int(String_length(s)) > sp.precision {
			s = String_substring(s, 0, int32(sp.precision))
		}
		if sp.conv == 'S' {
			s = strings.ToUpper(s)
		}
		return sp.pad(s)
	case 'b', 'B':
		s := "true"
		if arg == nil {
			s = "false"
		} else if b, ok := arg.(bool); ok {
			s = boolstr_(b)
		}
		if sp.precision >= 0 && len(s) > sp.precision {
			s = s[:sp.precision]
		}
		if sp.conv == 'B' {
			s = strings.ToUpper(s)
		}
		return sp.pad(s)
	case 'h', 'H':
		s := "null"
		if arg != nil {
			s = Integer_toHexString(Object_hashCode(arg))
		}
		if sp.conv == 'H' {
			s = strings.ToUpper(s)
		}
		return sp.pad(s)
	case 'c', 'C':
		var s string
		switch x := arg.(type) {
		case nil:
			s = "null"
		case uint16:
			s = String_valueOf_char(x)
		case int32:
			s = cpString_(x)
		case int16:
			s = cpString_(int32(x))
		case int8:
			s = cpString_(int32(x))
		default:
			badConversion_(sp, arg)
		}
		if sp.conv == 'C' {
			s = strings.ToUpper(s)
		}
		return sp.pad(s)
	case 'd':
		var neg bool
		var mag string
		switch x := arg.(type) {
		case nil:
			return sp.pad("null")
		case int32:
			neg, mag = x < 0, strconv.FormatUint(absU64_(int64(x)), 10)
		case int64:
			neg, mag = x < 0, strconv.FormatUint(absU64_(x), 10)
		case int16:
			neg, mag = x < 0, strconv.FormatUint(absU64_(int64(x)), 10)
		case int8:
			neg, mag = x < 0, strconv.FormatUint(absU64_(int64(x)), 10)
		case *BigInteger:
			neg, mag = x.v.Sign() < 0, new(big.Int).Abs(x.v).String()
		default:
			badConversion_(sp, arg)
		}
		return sp.number(neg, mag)
	case 'x', 'X', 'o':
		base := 16
		if sp.conv == 'o' {
			base = 8
		}
		var s string
		neg := false
		switch x := arg.(type) {
		case nil:
			return sp.pad("null")
		case int32:
			s = strconv.FormatUint(uint64(uint32(x)), base)
		case int64:
			s = strconv.FormatUint(uint64(x), base)
		case int16:
			s = strconv.FormatUint(uint64(uint16(x)), base)
		case int8:
			s = strconv.FormatUint(uint64(uint8(x)), base)
		case *BigInteger:
			neg = x.v.Sign() < 0
			s = new(big.Int).Abs(x.v).Text(base)
		default:
			badConversion_(sp, arg)
		}
		prefix := ""
		if neg {
			prefix = "-"
		}
		if sp.sharp {
			if base == 16 {
				prefix += "0x"
			} else {
				prefix += "0"
			}
		}
		if sp.zero && sp.width > 0 {
			if n := sp.width - len(prefix) - len(s); n > 0 {
				s = strings.Repeat("0", n) + s
			}
		}
		s = prefix + s
		if sp.conv == 'X' {
			s = strings.ToUpper(s)
		}
		return sp.pad(s)
	case 'f', 'e', 'E':
		if arg == nil {
			return sp.pad("null")
		}
		v, bitSize, ok := formatFloatArg_(sp, arg)
		if !ok {
			badConversion_(sp, arg)
		}
		if v != v {
			return sp.pad("NaN")
		}
		neg := math.Signbit(v)
		if math.IsInf(v, 0) {
			s := "Infinity"
			switch {
			case neg && sp.paren:
				s = "(Infinity)"
			case neg:
				s = "-Infinity"
			case sp.plus:
				s = "+Infinity"
			}
			return sp.pad(s)
		}
		v = math.Abs(v)
		prec := sp.precision
		if prec < 0 {
			prec = 6
		}
		if sp.conv == 'f' {
			return sp.number(neg, javaFixedDigits_(v, prec, bitSize))
		}
		// %e: d.dddddde+xx
		var mag string
		if v == 0 {
			mag = roundDecimal_("0", 1, prec) + "e+00"
		} else {
			e := strconv.FormatFloat(v, 'e', -1, bitSize)
			mant, expStr, _ := strings.Cut(e, "e")
			exp, _ := strconv.Atoi(expStr)
			digits := strings.Replace(mant, ".", "", 1)
			m := roundDecimal_(digits, 1, prec)
			if strings.HasPrefix(m, "10") {
				m = roundDecimal_("1", 1, prec)
				exp++
			}
			sign := "+"
			if exp < 0 {
				sign = "-"
				exp = -exp
			}
			es := strconv.Itoa(exp)
			if len(es) < 2 {
				es = "0" + es
			}
			mag = m + "e" + sign + es
		}
		saveComma := sp.comma
		sp.comma = false
		s := sp.number(neg, mag)
		sp.comma = saveComma
		if sp.conv == 'E' {
			s = strings.ToUpper(s)
		}
		return s
	}
	formatFail_("Conversion = '" + string(sp.conv) + "'")
	return ""
}

func absU64_(x int64) uint64 {
	if x < 0 {
		return uint64(-x)
	}
	return uint64(x)
}

func String_format(format string, args []any) string {
	if format == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	p := strings.IndexByte(format, '%')
	if p < 0 {
		return format
	}
	var sb strings.Builder
	next, last := 0, -1
	i := 0
	for {
		sb.WriteString(format[i : i+p])
		i += p
		sp, end := parseFormatSpec_(format, i)
		i = end
		switch sp.conv {
		case 'n':
			sb.WriteByte('\n')
		case '%':
			sb.WriteString(sp.pad("%"))
		default:
			var idx int
			switch {
			case sp.index > 0:
				idx = sp.index - 1
			case sp.index < 0:
				idx = last
			default:
				idx = next
				next++
			}
			if idx < 0 || idx >= len(args) {
				formatFail_("Format specifier '" + sp.text + "'")
			}
			last = idx
			sb.WriteString(sp.format(args[idx]))
		}
		p = strings.IndexByte(format[i:], '%')
		if p < 0 {
			sb.WriteString(format[i:])
			return sb.String()
		}
	}
}

// ---- java.text.MessageFormat

type MessageFormat struct{}

// messageNumber_ is NumberFormat.getInstance().format: grouping, at most three
// fraction digits, half-even rounding.
func messageNumber_(arg any, integerOnly bool, grouping bool) (string, bool) {
	group := func(neg bool, intPart string, frac string) string {
		if grouping {
			intPart = groupDigits_(intPart)
		}
		s := intPart
		if frac != "" {
			s += "." + frac
		}
		if neg {
			s = "-" + s
		}
		return s
	}
	switch x := arg.(type) {
	case int32:
		return group(x < 0, strconv.FormatUint(absU64_(int64(x)), 10), ""), true
	case int64:
		return group(x < 0, strconv.FormatUint(absU64_(x), 10), ""), true
	case int16:
		return group(x < 0, strconv.FormatUint(absU64_(int64(x)), 10), ""), true
	case int8:
		return group(x < 0, strconv.FormatUint(absU64_(int64(x)), 10), ""), true
	case *BigInteger:
		return group(x.v.Sign() < 0, new(big.Int).Abs(x.v).String(), ""), true
	case float32:
		return messageNumber_(float64(x), integerOnly, grouping)
	case float64:
		if x != x {
			return "NaN", true
		}
		if math.IsInf(x, 0) {
			if x < 0 {
				return "-∞", true
			}
			return "∞", true
		}
		prec := 3
		if integerOnly {
			prec = 0
		}
		s := strconv.FormatFloat(math.Abs(x), 'f', prec, 64)
		if !integerOnly && strconv.FormatFloat(math.Abs(x), 'e', -1, 64) == "5e-04" {
			// DecimalFormat sees the shortest decimal form "0.0005" as an exact tie here.
			s = "0.000"
		}
		intPart, frac, _ := strings.Cut(s, ".")
		frac = strings.TrimRight(frac, "0")
		return group(math.Signbit(x), intPart, frac), true
	}
	return "", false
}

func MessageFormat_format(pattern string, args []any) string {
	if pattern == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	var sb strings.Builder
	inQuote := false
	n := len(pattern)
	for i := 0; i < n; i++ {
		ch := pattern[i]
		if ch == '\'' {
			if i+1 < n && pattern[i+1] == '\'' {
				sb.WriteByte('\'')
				i++
			} else {
				inQuote = !inQuote
			}
			continue
		}
		if ch != '{' || inQuote {
			sb.WriteByte(ch)
			continue
		}
		// An argument: {index[,type[,style]]}
		var seg [3]strings.Builder
		part, depth, q, closed := 0, 0, false, false
		for i++; i < n; i++ {
			c := pattern[i]
			if q {
				seg[part].WriteByte(c)
				if c == '\'' {
					q = false
				}
				continue
			}
			switch c {
			case ',':
				if part < 2 {
					part++
				} else {
					seg[part].WriteByte(c)
				}
			case '{':
				depth++
				seg[part].WriteByte(c)
			case '}':
				if depth == 0 {
					closed = true
				} else {
					depth--
					seg[part].WriteByte(c)
				}
			case ' ':
				// Leading spaces of the type are skipped.
				if part != 1 || seg[part].Len() > 0 {
					seg[part].WriteByte(c)
				}
			case '\'':
				q = true
				seg[part].WriteByte(c)
			default:
				seg[part].WriteByte(c)
			}
			if closed {
				break
			}
		}
		if !closed {
			argFail_("Unmatched braces in the pattern.")
		}
		numStr := seg[0].String()
		idx, err := strconv.Atoi(numStr)
		if err != nil {
			argFail_("can't parse argument number: " + numStr)
		}
		if idx < 0 {
			argFail_("negative argument number: " + numStr)
		}
		typ := strings.ToLower(strings.TrimSpace(seg[1].String()))
		style := seg[2].String()
		if idx >= len(args) {
			sb.WriteString("{" + strconv.Itoa(idx) + "}")
			continue
		}
		arg := args[idx]
		switch typ {
		case "":
			if arg == nil {
				sb.WriteString("null")
			} else if s, ok := messageNumber_(arg, false, true); ok {
				sb.WriteString(s)
			} else {
				sb.WriteString(objstr_(arg))
			}
		case "number":
			if arg == nil {
				sb.WriteString("null")
				break
			}
			st := strings.ToLower(strings.TrimSpace(style))
			var s string
			var ok bool
			switch {
			case st == "":
				s, ok = messageNumber_(arg, false, true)
			case st == "integer":
				s, ok = messageNumber_(arg, true, true)
			case strings.Trim(style, "#0") == "":
				s, ok = messageNumber_(arg, true, false)
				if ok {
					neg := strings.HasPrefix(s, "-")
					s = strings.TrimPrefix(s, "-")
					if z := strings.Count(style, "0"); len(s) < z {
						s = strings.Repeat("0", z-len(s)) + s
					}
					if neg {
						s = "-" + s
					}
				}
			default:
				argFail_("unsupported MessageFormat number style: " + style)
			}
			if !ok {
				argFail_("Cannot format given Object as a Number")
			}
			sb.WriteString(s)
		default:
			argFail_("unknown format type: " + seg[1].String())
		}
	}
	return sb.String()
}
