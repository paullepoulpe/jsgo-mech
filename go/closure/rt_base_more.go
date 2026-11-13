package closure

// Library members the prepped Java tree started to use.

import "strings"

func Boolean_toString(b bool) string       { return boolstr_(b) }
func String_valueOf_boolean(b bool) string { return boolstr_(b) }

func Objects_requireNonNull_Object(v any) any {
	if v == nil {
		panic(new_NullPointerException_0().self())
	}
	return v
}

// Predicate_base_test is the default method test of Guava's Predicate.
func Predicate_base_test(p Predicate_base, v any) bool { return p.apply(v) }

func Strings_padEnd(s string, minLength int32, padChar uint16) string {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	n := String_length(s)
	if n >= minLength {
		return s
	}
	return s + strings.Repeat(String_valueOf_char(padChar), int(minLength-n))
}

// Ints_tryParse and Longs_tryParse accept an optional '-' and ASCII digits only.
func tryParseDecimal_(s string, min, max int64) (int64, bool) {
	if s == nullstr_ || s == "" {
		return 0, false
	}
	i, neg := 0, false
	if s[0] == '-' {
		neg, i = true, 1
	}
	if i == len(s) {
		return 0, false
	}
	// Accumulate negatively, as Guava does, so that the minimum value parses.
	var acc int64
	for ; i < len(s); i++ {
		d := int64(s[i]) - '0'
		if d < 0 || d > 9 {
			return 0, false
		}
		if acc < (min+d)/10 {
			return 0, false
		}
		acc = acc*10 - d
	}
	if neg {
		return acc, true
	}
	if acc < -max {
		return 0, false
	}
	return -acc, true
}

func Longs_tryParse(s string) any {
	if v, ok := tryParseDecimal_(s, -1<<63, 1<<63-1); ok {
		return v
	}
	return nil
}

// Doubles_tryParse is Double.valueOf without the surrounding whitespace it tolerates.
func Doubles_tryParse(s string) (result any) {
	if s == nullstr_ {
		panic(new_NullPointerException_0().self())
	}
	if s == "" || s[0] <= ' ' || s[len(s)-1] <= ' ' {
		return nil
	}
	defer func() {
		if r := recover(); r != nil {
			if !is_NumberFormatException(r) {
				panic(r)
			}
			result = nil
		}
	}()
	return Double_parseDouble(s)
}

func new_ArrayIndexOutOfBoundsException_int(index int32) *ArrayIndexOutOfBoundsException {
	return new_ArrayIndexOutOfBoundsException_String("Array index out of range: " + istr_(int64(index)))
}

func new_ParseException(msg string, errorOffset int32) *ParseException {
	this := &ParseException{}
	this.self_ = this
	this.init_Exception_String(msg)
	return this
}

func new_ImmutableSortedSet_Builder(cmp Comparator) *ImmutableSortedSet_Builder {
	return ImmutableSortedSet_orderedBy(cmp)
}

func JsonParser_parseReader(r *JsonReader) *JsonElement { return JsonParser_parseReader_JsonReader(r) }

func new_FileReader(f *File) *FileReader {
	this := &FileReader{InputStreamReader: *new_InputStreamReader(new_FileInputStream_File(f).up_InputStream(), StandardCharsets_UTF_8)}
	this.self_ = this
	return this
}

func new_PrintWriter(out *Writer) *PrintWriter {
	this := &PrintWriter{}
	this.self_ = this
	this.sink_ = out.write
	this.flusher_ = out.flush
	this.closer_ = out.close
	return this
}

func (this *PrintWriter) println(s string) { this.write(sstr_(s) + "\n") }

func Ints_tryParse(s string) any {
	if v, ok := tryParseDecimal_(s, -1<<31, 1<<31-1); ok {
		return int32(v)
	}
	return nil
}

func Collections_shuffle(l List, rnd *Random) {
	for i := l.size(); i > 1; i-- {
		j := rnd.nextInt_int(i)
		a, b := l.get(i-1), l.get(j)
		l.set(i-1, b)
		l.set(j, a)
	}
}

func ImmutableList_sortedCopyOf_Iterable(elements Iterable) *ImmutableList {
	return ImmutableList_sortedCopyOf(nil, elements)
}
