package main

import (
	"context"
	"errors"
	"fmt"
	"log"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/iferrnil/ai-news-radar/server/radar"
)

// notifier shows macOS banners and remembers what happened, so 设置 › 通知 can show it.
//
// Root cause of "banners never appear" (≤0.5.1): the release builds are signed ad hoc (no Developer ID / Team ID).
// MyGo v0.4.3 posts through UNUserNotificationCenter: the first Show calls requestAuthorization and then
// addNotificationRequest. For an ad-hoc-signed bundle macOS commonly answers requestAuthorization with
// granted=false *without showing the prompt* (and every auto-update changes the ad-hoc code signature, so a
// grant does not carry over), after which addNotificationRequest fails with UNErrorDomain 1. MyGo ignores
// the authorization callback's result and only returns ErrNotificationsDenied from Show; 0.5.x turned that
// into a status word and logged the error to server.log, so nothing on screen explained it and no banner
// was ever shown. (0.4.x additionally reused one request identifier, which made later batches silent updates.)
//
// Fix: method "auto" (default) tries the native center and falls back to AppleScript's `display notification`
// (posted by the system-signed osascript, which works for unsigned apps); "applescript" always uses it,
// "native" never falls back. Every attempt records the path taken and the exact error.
type notifier struct {
	mu        sync.Mutex
	status    string // native permission: granted | denied | unknown | unsupported
	method    string // auto | native | applescript
	lastPath  string
	lastError string
	lastAt    time.Time
	prev      *mygo.Notification
	seq       int
	dir       string
}

const notifySettingsURL = "x-apple.systempreferences:com.apple.Notifications-Settings.extension"

func newNotifier(dataDir string) *notifier {
	n := &notifier{status: "unknown", method: "auto", dir: dataDir}
	if b, err := os.ReadFile(filepath.Join(dataDir, "notify-permission")); err == nil {
		if s := strings.TrimSpace(string(b)); s == "granted" || s == "denied" {
			n.status = s
		}
	}
	if b, err := os.ReadFile(filepath.Join(dataDir, "notify-method")); err == nil {
		if m := strings.TrimSpace(string(b)); validMethod(m) {
			n.method = m
		}
	}
	return n
}

func validMethod(m string) bool { return m == "auto" || m == "native" || m == "applescript" }

func nativeSupported() bool { return runtime.GOOS == "darwin" && mygo.NotificationsSupported() }

func (n *notifier) State() radar.NotifyState {
	n.mu.Lock()
	defer n.mu.Unlock()
	st := n.status
	if !nativeSupported() {
		st = "unsupported"
	}
	s := radar.NotifyState{Status: st, Method: n.method, LastPath: n.lastPath, LastError: n.lastError, AppleScript: appleScriptAvailable()}
	if !n.lastAt.IsZero() {
		s.LastAt = n.lastAt.UnixMilli()
	}
	return s
}

func (n *notifier) SetMethod(m string) error {
	if !validMethod(m) {
		return errors.New("未知的通知方式")
	}
	n.mu.Lock()
	n.method = m
	n.mu.Unlock()
	return os.WriteFile(filepath.Join(n.dir, "notify-method"), []byte(m), 0o644)
}

func (n *notifier) setStatus(s string) {
	n.mu.Lock()
	changed := n.status != s
	n.status = s
	n.mu.Unlock()
	if changed {
		_ = os.WriteFile(filepath.Join(n.dir, "notify-permission"), []byte(s), 0o644)
	}
}

func (n *notifier) record(path string, err error) {
	n.mu.Lock()
	n.lastPath, n.lastAt = path, time.Now()
	n.lastError = ""
	if err != nil {
		n.lastError = err.Error()
	}
	n.mu.Unlock()
}

// showNative posts through UNUserNotificationCenter with a fresh identifier (the previous banner is removed).
func (n *notifier) showNative(title, body string, onClick func()) error {
	if !nativeSupported() {
		return errors.New("系统通知中心不可用（需要从打包后的 .app 运行）")
	}
	n.mu.Lock()
	n.seq++
	id := fmt.Sprintf("ai-radar-%d-%d", time.Now().UnixMilli(), n.seq)
	prev := n.prev
	n.mu.Unlock()
	nt := mygo.NewNotification(mygo.NotificationOptions{ID: id, Title: title, Body: body, Group: "ai-radar"})
	if onClick != nil {
		nt.OnClick(onClick)
	}
	err := nt.Show()
	if errors.Is(err, mygo.ErrNotificationsDenied) {
		n.setStatus("denied")
		return errors.New("系统未允许本 App 发送通知（UNErrorDomain 1：未授权；未签名的 App 常被系统直接拒绝）")
	}
	if err != nil {
		return fmt.Errorf("系统通知失败：%w", err)
	}
	n.setStatus("granted")
	if prev != nil {
		prev.Close()
	}
	n.mu.Lock()
	n.prev = nt
	n.mu.Unlock()
	return nil
}

func appleScriptAvailable() bool {
	if runtime.GOOS != "darwin" {
		return false
	}
	_, err := exec.LookPath("osascript")
	return err == nil
}

// appleScriptArgs passes title/body as argv (no string interpolation into the script).
func appleScriptArgs(title, body string) []string {
	return []string{"-e", "on run argv", "-e", `display notification (item 2 of argv) with title (item 1 of argv) sound name "default"`, "-e", "end run", title, body}
}

var runOsascript = func(ctx context.Context, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, "osascript", args...).CombinedOutput()
}

func (n *notifier) showAppleScript(title, body string) error {
	if !appleScriptAvailable() {
		return errors.New("osascript 不可用")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	out, err := runOsascript(ctx, appleScriptArgs(title, body)...)
	if err != nil {
		return fmt.Errorf("AppleScript 通知失败：%v %s", err, strings.TrimSpace(string(out)))
	}
	return nil
}

// show delivers one banner by the configured method; returns the path used.
func (n *notifier) show(title, body string, onClick func()) (string, error) {
	n.mu.Lock()
	m := n.method
	n.mu.Unlock()
	var path string
	var err error
	switch m {
	case "applescript":
		path, err = "applescript", n.showAppleScript(title, body)
	case "native":
		path, err = "native", n.showNative(title, body, onClick)
	default:
		path, err = "native", n.showNative(title, body, onClick)
		if err != nil {
			log.Println("notification (native):", err, "→ AppleScript")
			if e2 := n.showAppleScript(title, body); e2 == nil {
				path, err = "applescript", nil
				n.record(path, nil)
				n.mu.Lock()
				n.lastError = "系统通知中心不可用，已改用 AppleScript：" + err.Error()
				n.mu.Unlock()
				return path, nil
			} else {
				err = fmt.Errorf("%v；%v", err, e2)
			}
		}
	}
	if err != nil {
		log.Println("notification:", err)
	}
	n.record(path, err)
	return path, err
}

// install wires the radar server's desktop hooks (设置 › 通知 / 数据).
func (n *notifier) install(show func(), dataDir string) {
	radar.DesktopHooks = radar.DesktopHooksT{
		NotifyState:     n.State,
		SetNotifyMethod: n.SetMethod,
		NotifyTest: func() (radar.NotifyState, error) {
			_, err := n.show("AI News Radar 测试通知", "看到这条横幅，说明系统通知工作正常。", show)
			return n.State(), err
		},
		OpenNotifySettings: func() error { return mygo.Shell.OpenExternal(notifySettingsURL) },
		OpenDataDir:        func() error { return mygo.Shell.OpenPath(dataDir) },
	}
}
