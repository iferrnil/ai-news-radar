package radar

import (
	"bytes"
	"strings"
	"testing"
)

// writeStr is the hand-rolled fast path of the JSON row encoder (0.14.2 perf); it must stay byte-identical to the
// encoder it replaced, including the rare cases it hands back to it.
func TestWriteStrMatchesEncoder(t *testing.T) {
	cases := []string{"", "abc", `a"b\c`, "line\nbreak\r\ttab", "中文 · emoji 😀", "\u2028x\u2029", "bad \xff utf8", "ctl \x01 \x08 \x0c", "<a href='x'>&amp;</a>", "\xe4\xb8", "end\xc3", "a\u00e9b", "\U0001F600\"q\"", strings.Repeat("长文本", 200)}
	for _, c := range cases {
		var b bytes.Buffer
		writeStr(&b, c)
		want, _ := marshalNoEscape(c)
		if b.String() != string(want) {
			t.Errorf("%q: got %s want %s", c, b.String(), want)
		}
	}
}

func TestRowsEncodeTypes(t *testing.T) {
	r := obj("s", "x\"y", "i", int64(-3), "f", 1.5, "b", true, "n", nil, "raw", []byte("z"), "sub", obj("k", false))
	b, _ := Rows{List: []*Row{r}, DropNulls: true}.MarshalJSON()
	if want := `[{"s":"x\"y","i":-3,"f":1.5,"b":true,"raw":"z","sub":{"k":false}}]`; string(b) != want {
		t.Fatalf("got %s want %s", b, want)
	}
	r.Del("i")
	r.Del("s")
	r.Set("t", "new")
	if b, _ := r.MarshalJSON(); string(b) != `{"f":1.5,"b":true,"n":null,"raw":"z","sub":{"k":false},"t":"new"}` {
		t.Fatalf("after Del/Set: %s", b)
	}
}
