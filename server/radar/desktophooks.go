package radar

import "net/http"

// NotifyState is what 设置 › 通知 shows about native banners (desktop only).
type NotifyState struct {
	Status      string `json:"status"`    // native permission: granted | denied | unknown | unsupported
	Method      string `json:"method"`    // auto | native | applescript
	LastPath    string `json:"lastPath"`  // path the last banner took: native | applescript | ""
	LastError   string `json:"lastError"` // exact error / fallback note of the last attempt
	LastAt      int64  `json:"lastAt"`    // ms
	AppleScript bool   `json:"appleScript"`
}

// DesktopHooks are set by the desktop shell (zero on the web server).
type DesktopHooksT struct {
	NotifyState        func() NotifyState
	SetNotifyMethod    func(string) error
	NotifyTest         func() (NotifyState, error) // shows a banner by the configured method
	OpenNotifySettings func() error                // System Settings › Notifications
	OpenDataDir        func() error                // reveal the data folder
}

var DesktopHooks DesktopHooksT

func (s *Store) hDesktopNotifyStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if DesktopHooks.NotifyState == nil {
		writeJSON(w, 200, NotifyState{Status: "unsupported", Method: "auto"})
		return
	}
	writeJSON(w, 200, DesktopHooks.NotifyState())
}

func (s *Store) hDesktopNotifyPatch(w http.ResponseWriter, r *http.Request) {
	if DesktopHooks.SetNotifyMethod == nil {
		detail(w, 404, "仅桌面版可用")
		return
	}
	if err := DesktopHooks.SetNotifyMethod(str(readBody(r)["method"])); err != nil {
		detail(w, 400, err.Error())
		return
	}
	writeJSON(w, 200, DesktopHooks.NotifyState())
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
