package closure

import "testing"

// TestJsonParse checks Json_parse against what the Java Json.parse returns for the documents of
// testdata/io/json.tsv (written by tools/iotable; the notation is described in rt_io_test.go): the
// parsed value printed with its toString, or the failure.
func TestJsonParse(t *testing.T) {
	ioTable_(t, "json.tsv", map[string]func(a string) any{
		"jsonParse": func(a string) any { return Json_parse(a) },
	})
}
