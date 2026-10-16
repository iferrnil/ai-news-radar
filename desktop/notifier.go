package main

import (
	"errors"
	"fmt"
	"log"
	"runtime"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/iferrnil/ai-news-radar/server/radar"
)

// notifier shows macOS banners through UNUserNotificationCenter only, and remembers what happened so
// 设置 › 通知 can show the permission and the last error.
//
// Why banners did not appear (≤0.6.0):
//   - Upstream MyGo v0.4.3 only asks for permission lazily inside the first Show and ignores the answer, so the
//     status was unknown until the first P0 batch and a refusal surfaced only as ErrNotificationsDenied.
//     The vendored MyGo (third_party/mygo, notify_auth.go) adds RequestNotificationAuthorization /
//     NotificationAuthorizationStatus: we ask once at launch (the system prompt appears then) and read the
//     real status (getNotificationSettings) whenever the settings page asks.
//   - The releases were signed ad hoc: the code signature — and so the identity macOS keys the notification
//     permission to — changed with every update, so a permission given to one version did not carry over.
//     From 0.7.0 the release workflow signs with a stable self-signed certificate (MACOS_CERTIFICATE secret),
//     so the designated requirement stays the same across updates.
//   - The delegate (installed by MyGo at launch, on the main thread) answers willPresent with
//     banner | list | sound, so banners also show while the app is in the foreground; each banner has a
//     unique identifier and replaces the previous one.
type notifier struct {
	mu        sync.Mutex
	status    string // granted | denied | unknown | unsupported
	checked   time.Time
	lastError string
	lastAt    time.Time
	prev      *mygo.Notification
	seq       int
}

const notifySettingsURL = "x-apple.systempreferences:com.apple.Notifications-Settings.extension"

func newNotifier() *notifier { return &notifier{status: "unknown"} }

func nativeSupported() bool { return runtime.GOOS == "darwin" && mygo.NotificationsSupported() }

// queryStatus / requestAuth are vars so tests can stub the system.
var (
	queryStatus = mygo.NotificationAuthorizationStatus
	requestAuth = mygo.RequestNotificationAuthorization
)

// start asks for permission once at launch when the user has not answered yet (shows the system prompt).
func (n *notifier) start() {
	if !nativeSupported() {
		n.set("unsupported", nil)
		return
	}
	go func() {
		st := queryStatus()
		if st == "unknown" {
			ok, err := requestAuth()
			if err != nil {
				log.Println("notification authorization:", err)
			}
			st = queryStatus()
			if st == "unknown" && ok {
				st = "granted"
			}
		}
		n.set(st, nil)
	}()
}

func (n *notifier) set(st string, err error) {
	n.mu.Lock()
	defer n.mu.Unlock()
	if st != "" {
		n.status, n.checked = st, time.Now()
	}
	if err != nil {
		n.lastError = err.Error()
	}
}

// refresh re-reads the system's status (the user may have changed it in System Settings), at most every 2 s.
func (n *notifier) refresh() {
	if !nativeSupported() {
		n.set("unsupported", nil)
		return
	}
	n.mu.Lock()
	fresh := time.Since(n.checked) < 2*time.Second
	n.mu.Unlock()
	if !fresh {
		n.set(queryStatus(), nil)
	}
}

func (n *notifier) State() radar.NotifyState {
	n.refresh()
	n.mu.Lock()
	defer n.mu.Unlock()
	s := radar.NotifyState{Status: n.status, LastError: n.lastError}
	if !n.lastAt.IsZero() {
		s.LastAt = n.lastAt.UnixMilli()
	}
	return s
}

// show posts one banner with a fresh identifier and removes the previous one.
func (n *notifier) show(title, body string, onClick func()) error {
	err := n.post(title, body, onClick)
	n.mu.Lock()
	n.lastAt, n.lastError = time.Now(), ""
	if err != nil {
		n.lastError = err.Error()
	}
	n.mu.Unlock()
	if err != nil {
		log.Println("notification:", err)
	}
	return err
}

func (n *notifier) post(title, body string, onClick func()) error {
	if !nativeSupported() {
		n.set("unsupported", nil)
		return errors.New("系统通知不可用（需要从打包后的 .app 运行）")
	}
	n.mu.Lock()
	st := n.status
	n.seq++
	id := fmt.Sprintf("ai-radar-%d-%d", time.Now().UnixMilli(), n.seq)
	prev := n.prev
	n.mu.Unlock()
	if st == "unknown" {
		if _, err := requestAuth(); err != nil {
			log.Println("notification authorization:", err)
		}
		n.set(queryStatus(), nil)
	}
	nt := mygo.NewNotification(mygo.NotificationOptions{ID: id, Title: title, Body: body, Group: "ai-radar"})
	if onClick != nil {
		nt.OnClick(onClick)
	}
	err := nt.Show()
	if errors.Is(err, mygo.ErrNotificationsDenied) {
		n.set("denied", nil)
		return errors.New("系统设置中未允许本 App 的通知")
	}
	if err != nil {
		return fmt.Errorf("系统通知失败：%w", err)
	}
	n.set("granted", nil)
	if prev != nil {
		prev.Close()
	}
	n.mu.Lock()
	n.prev = nt
	n.mu.Unlock()
	return nil
}

// install wires the radar server's desktop hooks (设置 › 通知 / 数据).
func (n *notifier) install(show func(), dataDir string) {
	radar.DesktopHooks = radar.DesktopHooksT{
		NotifyState: n.State,
		NotifyTest: func() (radar.NotifyState, error) {
			err := n.show("AI News Radar 测试通知", "看到这条横幅，说明系统通知工作正常。", show)
			return n.State(), err
		},
		OpenNotifySettings: func() error { return mygo.Shell.OpenExternal(notifySettingsURL) },
		OpenDataDir:        func() error { return mygo.Shell.OpenPath(dataDir) },
		OpenPath:           func(p string) error { return mygo.Shell.OpenPath(p) },
	}
}
