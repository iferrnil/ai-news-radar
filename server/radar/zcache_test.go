package radar

import (
	"bytes"
	"compress/gzip"
	"io"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/andybalholm/brotli"
)

// compressed JSON bodies are cached by ETag (0.14.2 perf): every encoding must still decode to the identity body,
// and a changed body must not be served from the cache.
func TestJSONCompressCache(t *testing.T) {
	get := func(body, ae string) []byte {
		w := httptest.NewRecorder()
		r := httptest.NewRequest("GET", "/api/items", nil)
		r.Header.Set("Accept-Encoding", ae)
		jsonResponse(w, r, []byte(body))
		var rd io.Reader = w.Body
		switch w.Header().Get("Content-Encoding") {
		case "br":
			rd = brotli.NewReader(w.Body)
		case "gzip":
			rd, _ = gzip.NewReader(w.Body)
		}
		b, _ := io.ReadAll(rd)
		return b
	}
	a := `[{"t":"` + strings.Repeat("新闻 news ", 400) + `"}]`
	b := strings.Replace(a, "news", "item", 1)
	for i := 0; i < 2; i++ { // second round is served from the cache
		for _, ae := range []string{"br", "gzip", "identity"} {
			if got := get(a, ae); !bytes.Equal(got, []byte(a)) {
				t.Fatalf("%s round %d: body differs", ae, i)
			}
			if got := get(b, ae); !bytes.Equal(got, []byte(b)) {
				t.Fatalf("%s round %d: changed body served stale", ae, i)
			}
		}
	}
	for i := 0; i < zCacheMax*2; i++ {
		get(a+strings.Repeat(" ", i), "br")
	}
	if len(zCache.m) > zCacheMax || len(zCache.order) > zCacheMax {
		t.Fatalf("cache grew to %d", len(zCache.m))
	}
}
