package radar

import "net/http"

// DesktopHooks are set by the desktop shell (nil on the web server): the native notification bridge.
type DesktopHooksT struct {
	NotifyStatus       func() string          // "granted" | "denied" | "unknown" | "unsupported"
	NotifyTest         func() (string, error) // shows a native test banner; returns the resulting status
	OpenNotifySettings func() error           // opens System Settings > Notifications
	OpenDataDir        func() error           // reveals the user data folder in Finder
}

var DesktopHooks DesktopHooksT

func (s *Store) hDesktopNotifyStatus(w http.ResponseWriter, r *http.Request) {
	st := "unsupported"
	if DesktopHooks.NotifyStatus != nil {
		st = DesktopHooks.NotifyStatus()
	}
	w.Header().Set("Cache-Control", "no-store")
	writeJSON(w, 200, obj("status", st, "native", DesktopHooks.NotifyStatus != nil))
}

func (s *Store) hDesktopNotifyOpen(w http.ResponseWriter, r *http.Request) {
	if DesktopHooks.OpenNotifySettings == nil {
		detail(w, 404, "仅桌面版可用")
		return
	}
	if err := DesktopHooks.OpenNotifySettings(); err != nil {
		writeJSON(w, 200, obj("ok", false, "error", err.Error()))
		return
	}
	writeJSON(w, 200, obj("ok", true))
}

func (s *Store) hDesktopOpenData(w http.ResponseWriter, r *http.Request) {
	if DesktopHooks.OpenDataDir == nil {
		detail(w, 404, "仅桌面版可用")
		return
	}
	if err := DesktopHooks.OpenDataDir(); err != nil {
		writeJSON(w, 200, obj("ok", false, "error", err.Error()))
		return
	}
	writeJSON(w, 200, obj("ok", true))
}
