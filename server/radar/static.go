package radar

import (
	"bytes"
	"compress/gzip"
	"fmt"
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
	"sync"

	"github.com/andybalholm/brotli"
)

func init() {
	mime.AddExtensionType(".woff2", "font/woff2")
	mime.AddExtensionType(".webmanifest", "application/manifest+json")
}

// StaticHandler serves the Next.js static export (`out/`): exact file, then <path>.html, then <path>/index.html,
// otherwise 404.html with status 404. Hashed assets under /_next/static are cached forever.
func StaticHandler(root fs.FS) http.Handler {
	fsrv := http.FileServerFS(root)
	notFound, _ := fs.ReadFile(root, "404.html")
	exists := func(p string) bool {
		st, err := fs.Stat(root, p)
		return err == nil && !st.IsDir()
	}
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodHead {
			w.Header().Set("Allow", "GET, HEAD")
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		p := strings.TrimPrefix(path.Clean("/"+r.URL.Path), "/")
		if strings.HasPrefix(p, "_next/static/") {
			w.Header().Set("Cache-Control", "public, max-age=31536000, immutable")
		}
		var file string
		switch {
		case p == "" || p == ".":
			file = "index.html"
		case exists(p):
			file = p
		case exists(p + ".html"):
			file = p + ".html"
		case exists(path.Join(p, "index.html")):
			file = path.Join(p, "index.html")
		}
		if file == "" {
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.WriteHeader(http.StatusNotFound)
			w.Write(notFound)
			return
		}
		if strings.HasSuffix(file, ".html") {
			w.Header().Set("Cache-Control", "no-cache")
		}
		if serveCompressed(w, r, root, file) {
			return
		}
		if strings.HasSuffix(file, ".html") {
			b, err := fs.ReadFile(root, file)
			if err != nil {
				http.Error(w, "not found", 404)
				return
			}
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			w.Write(b)
			return
		}
		r2 := r.Clone(r.Context())
		r2.URL.Path = "/" + file
		fsrv.ServeHTTP(w, r2)
	})
}

// Server combines the API (/api/*) and the static UI.
func (s *Store) Server(ui fs.FS) http.Handler {
	var st http.Handler = http.NotFoundHandler()
	if ui != nil {
		st = StaticHandler(ui)
		s.notFound = st
	}
	api := s.Handler()
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		securityHeaders(w)
		if !hostAllowed(r) {
			http.Error(w, "unknown host", http.StatusMisdirectedRequest)
			return
		}
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			if crossSite(r) {
				detail(w, 403, "cross-site request refused")
				return
			}
			api.ServeHTTP(w, r)
			return
		}
		st.ServeHTTP(w, r)
	})
}

// ---- compressed static files: JS / CSS / HTML / text of the export are sent brotli (or gzip) encoded. Each file
// is compressed once on first request and kept in memory (keyed by name + size + mtime, so a rebuilt out/ is
// picked up); the whole export is ~1 MB compressed.

// .txt (Next Flight / RSC payloads for output:export) stay uncompressed: WKWebView has been
// observed to fall through to MPA `location.assign` when soft-nav fetch of br-compressed .txt fails decode.
var staticTypes = map[string]string{".js": "text/javascript; charset=utf-8", ".css": "text/css; charset=utf-8",
	".html": "text/html; charset=utf-8", ".json": "application/json",
	".svg": "image/svg+xml", ".webmanifest": "application/manifest+json"}
// text/plain for .txt is set in StaticHandler when serving; listed here only if we ever compress again.

type staticZ struct {
	once sync.Once
	br   []byte
	gz   []byte
}

var staticZCache sync.Map // "file\x00size\x00mtime" -> *staticZ

// staticZMu: one file is compressed at a time; a brotli encoder holds several MB, and a cold page load asks for
// dozens of chunks at once (unbounded at quality 9 that peaked at ~650 MB RSS; serialized at quality 6 / 256 KB
// window it stays ~35 MB for 0.5 % more bytes than quality 7).
var staticZMu sync.Mutex

func acceptEnc(r *http.Request) string {
	ae := r.Header.Get("Accept-Encoding")
	switch {
	case strings.Contains(ae, "br"):
		return "br"
	case gzRe.MatchString(ae):
		return "gzip"
	}
	return ""
}

func serveCompressed(w http.ResponseWriter, r *http.Request, root fs.FS, file string) bool {
	ct, ok := staticTypes[path.Ext(file)]
	enc := acceptEnc(r)
	if !ok || enc == "" || r.Header.Get("Range") != "" {
		return false
	}
	st, err := fs.Stat(root, file)
	if err != nil || st.Size() < 1024 {
		return false
	}
	key := fmt.Sprintf("%s\x00%d\x00%d", file, st.Size(), st.ModTime().UnixNano())
	v, _ := staticZCache.LoadOrStore(key, &staticZ{})
	z := v.(*staticZ)
	z.once.Do(func() {
		raw, err := fs.ReadFile(root, file)
		if err != nil {
			return
		}
		staticZMu.Lock()
		defer staticZMu.Unlock()
		var b bytes.Buffer
		bw := brotli.NewWriterOptions(&b, brotli.WriterOptions{Quality: 6, LGWin: 18})
		bw.Write(raw)
		bw.Close()
		z.br = bytes.Clone(b.Bytes())
		b.Reset()
		gw, _ := gzip.NewWriterLevel(&b, 9)
		gw.Write(raw)
		gw.Close()
		z.gz = bytes.Clone(b.Bytes())
	})
	body := z.br
	if enc == "gzip" {
		body = z.gz
	}
	if body == nil {
		return false
	}
	h := w.Header()
	etag := fmt.Sprintf(`"%x-%x-%s"`, st.ModTime().UnixNano(), st.Size(), enc)
	h.Set("Content-Type", ct)
	h.Set("Vary", "Accept-Encoding")
	h.Set("ETag", etag)
	if !st.ModTime().IsZero() {
		h.Set("Last-Modified", st.ModTime().UTC().Format(http.TimeFormat))
	}
	if inm := r.Header.Get("If-None-Match"); inm != "" && strings.Contains(inm, etag) {
		w.WriteHeader(http.StatusNotModified)
		return true
	}
	h.Set("Content-Encoding", enc)
	h.Set("Content-Length", fmt.Sprint(len(body)))
	if r.Method == http.MethodHead {
		return true
	}
	w.Write(body)
	return true
}
