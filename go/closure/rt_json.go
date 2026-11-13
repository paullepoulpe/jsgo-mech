package closure

// com.google.javascript.jscomp.base.Json, written by hand on encoding/json (the Java class is
// listed in tools/j2go/handwritten.txt and is not translated; there it sits on Gson).
//
// The language is the one java/src/com/google/javascript/jscomp/base/Json.java documents:
// RFC 8259, one byte order mark allowed in front, limits on nesting and on the length of
// numbers, an unpaired surrogate read as U+FFFD. go/closure/testdata/io/json.tsv holds what the
// Java class returns for a list of documents (tools/iotable); rt_json_test.go checks against it.

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
)

const (
	jsonMaxDepth_         = 1000 // Json.MAX_DEPTH
	jsonMaxNumberLength_  = 1024 // Json.MAX_NUMBER_LENGTH
	jsonMaxIntegerDigits_ = 20   // Json.MAX_INTEGER_DIGITS
)

// Json_parse parses a text that is one JSON value. A number keeps its source text; a member
// whose name is repeated replaces the earlier one, at the earlier one's position.
func Json_parse(text string) *Outcome_base {
	malformed := Outcome_base_err(new_SimpleJsonException("Malformed JSON").as_Throwable())
	doc := []byte(text)
	if bytes.IndexByte(doc, 0xED) >= 0 {
		// A surrogate that the string holds on its own (three bytes, see rt_string.go) is written
		// as its escape: the decoder pairs escapes, and reads an unpaired one as U+FFFD.
		doc = doc[:0]
		for i := 0; i < len(text); i++ {
			if text[i] == 0xED && i+2 < len(text) && text[i+1] >= 0xA0 {
				cp, _ := decode_(text, i)
				doc = append(doc, fmt.Sprintf(`\u%04x`, cp)...)
				i += 2
			} else {
				doc = append(doc, text[i])
			}
		}
	}
	doc = bytes.TrimPrefix(doc, []byte("\xEF\xBB\xBF"))
	if !json.Valid(doc) { // also: a second byte order mark, anything after the value
		return malformed
	}
	decoder := json.NewDecoder(bytes.NewReader(doc))
	decoder.UseNumber()
	value := readJson_(decoder, 0)
	if value == nil {
		return malformed
	}
	return Outcome_base_ok_Object(any(value))
}

// readJson_ reads the next value of a valid document, which is inside depth arrays or objects;
// nil if the document goes over one of the limits.
func readJson_(decoder *json.Decoder, depth int) *SimpleJsonValue {
	token, _ := decoder.Token()
	switch t := token.(type) {
	case json.Delim:
		if depth == jsonMaxDepth_ {
			return nil
		}
		value := SimpleJsonValue_newArray()
		if t == '{' {
			value = SimpleJsonValue_newObject()
		}
		for decoder.More() {
			name := ""
			if t == '{' {
				key, _ := decoder.Token()
				name = key.(string)
			}
			item := readJson_(decoder, depth+1)
			if item == nil {
				return nil
			}
			if t == '{' {
				value.members().put(any(name), any(item))
			} else {
				value.elements().add(any(item))
			}
		}
		decoder.Token() // the closing bracket
		return value
	case string:
		return SimpleJsonValue_ofString(t)
	case json.Number:
		digits := strings.TrimPrefix(string(t), "-")
		if len(t) >= jsonMaxNumberLength_ || len(digits)-len(strings.TrimLeft(digits, "0123456789")) > jsonMaxIntegerDigits_ {
			return nil
		}
		return SimpleJsonValue_ofNumber(string(t))
	case bool:
		return SimpleJsonValue_ofBoolean(t)
	}
	return SimpleJsonValue_ofNull()
}
