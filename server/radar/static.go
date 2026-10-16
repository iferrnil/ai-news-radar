package radar

import (
	"io/fs"
	"mime"
	"net/http"
	"path"
	"strings"
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
		if r.URL.Path == "/api" || strings.HasPrefix(r.URL.Path, "/api/") {
			api.ServeHTTP(w, r)
			return
		}
		st.ServeHTTP(w, r)
	})
}
