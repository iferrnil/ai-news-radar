package main

import (
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/iferrnil/ai-news-radar/server/radar"
)

// notifier wraps MyGo's native notifications and tracks the permission the system last reported.
//
// Root cause of the missing banners (≤0.4.0): every P0 batch reused one request identifier ("ai-radar-p0").
// UNUserNotificationCenter treats a request with the identifier of a delivered notification as an *update*:
// it silently replaces the entry in Notification Center and shows no banner. So only the very first P0 batch
// of a session ever produced a banner. Each notification now gets a unique identifier (the previous one is
// removed so Notification Center still keeps just the latest batch), and the permission answer is surfaced in
// 设置 › 通知 together with a test button and a link to System Settings.
type notifier struct {
	mu     sync.Mutex
	status string // granted | denied | unknown | unsupported
	prev   *mygo.Notification
	seq    int
	file   string
}

const notifySettingsURL = "x-apple.systempreferences:com.apple.Notifications-Settings.extension"

func newNotifier(dataDir string) *notifier {
	n := &notifier{status: "unknown", file: filepath.Join(dataDir, "notify-permission")}
	if b, err := os.ReadFile(n.file); err == nil {
		if s := strings.TrimSpace(string(b)); s == "granted" || s == "denied" {
			n.status = s
		}
	}
	return n
}

func (n *notifier) Status() string {
	if !mygo.NotificationsSupported() {
		return "unsupported"
	}
	n.mu.Lock()
	defer n.mu.Unlock()
	return n.status
}

func (n *notifier) setStatus(s string) {
	n.mu.Lock()
	changed := n.status != s
	n.status = s
	n.mu.Unlock()
	if changed {
		_ = os.WriteFile(n.file, []byte(s), 0o644)
	}
}

// show displays a banner with a fresh identifier and drops the previous one from Notification Center.
func (n *notifier) show(title, body string, onClick func()) error {
	if !mygo.NotificationsSupported() {
		n.setStatus("unsupported")
		return errors.New("当前运行方式不支持系统通知（需要打包后的 .app）")
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
	switch {
	case errors.Is(err, mygo.ErrNotificationsDenied):
		n.setStatus("denied")
		return errors.New("系统设置中未允许 AI News Radar 发送通知")
	case err != nil:
		log.Println("notification:", err)
		return err
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

// install wires the radar server's desktop hooks (设置 › 通知: status / 发送测试通知 / 打开系统设置).
func (n *notifier) install(show func(), dataDir string) {
	radar.DesktopHooks = radar.DesktopHooksT{
		NotifyStatus: n.Status,
		NotifyTest: func() (string, error) {
			err := n.show("AI News Radar 测试通知", "如果你看到这条横幅，系统通知工作正常。", show)
			return n.Status(), err
		},
		OpenNotifySettings: func() error { return mygo.Shell.OpenExternal(notifySettingsURL) },
		OpenDataDir:        func() error { return mygo.Shell.OpenPath(dataDir) },
	}
}
