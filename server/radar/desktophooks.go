package radar

import "net/http"

// NotifyState is what 设置 › 通知 shows about native banners (desktop only).
type NotifyState struct {
	Status    string `json:"status"`    // system permission: granted | denied | unknown | unsupported
	LastError string `json:"lastError"` // exact error of the last banner, "" when it was shown
	LastAt    int64  `json:"lastAt"`    // ms of the last banner attempt
}

// DesktopHooks are set by the desktop shell (zero on the web server).
type DesktopHooksT struct {
	NotifyState        func() NotifyState
	NotifyTest         func() (NotifyState, error) // shows a banner through UNUserNotificationCenter
	OpenNotifySettings func() error                // System Settings › Notifications
	OpenDataDir        func() error                // reveal the data folder
}

var DesktopHooks DesktopHooksT

func (s *Store) hDesktopNotifyStatus(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if DesktopHooks.NotifyState == nil {
		writeJSON(w, 200, NotifyState{Status: "unsupported"})
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
