// radar-native is AI News Radar's desktop app drawn with MyGo's native UI (no webview, no Node).
package main

import (
	_ "embed"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"sync"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
	updnative "github.com/egoist/mygo/plugins/updater/native"
	"github.com/egoist/mygo/ui"
	"github.com/iferrnil/ai-news-radar/desktop-native/app"
	"github.com/iferrnil/ai-news-radar/desktop-native/core"
)

//go:embed sources.json
var seedJSON []byte

//go:embed tray.png
var trayIcon []byte

const (
	appName    = "AI News Radar"
	repo       = "iferrnil/ai-news-radar"
	tagPrefix  = "desktop-native-v" // keep in sync with mygo.json updates.tagPrefix
	thumbLimit = 200 << 20
)

type host struct {
	win   *mygo.Window
	mu    sync.Mutex
	urls  map[string]string // notification id -> url
	upd   *updates
	ready bool
}

func (h *host) Post(fn func()) {
	if h.win != nil {
		h.win.Update(fn)
	}
}
func (h *host) OpenURL(u string) { go mygo.Shell.OpenExternal(u) }
func (h *host) SaveFile(name string, data []byte) error {
	p, err := mygo.Dialog.Save(mygo.SaveDialogOptions{Parent: h.win, Title: "导出数据源", DefaultPath: name})
	if err != nil || p == "" {
		return err
	}
	return os.WriteFile(p, data, 0o644)
}
func (h *host) OpenFile() (string, []byte, error) {
	ps, err := mygo.Dialog.Open(mygo.OpenDialogOptions{Parent: h.win, Title: "导入数据源",
		Filters: []mygo.FileFilter{{Name: "JSON / OPML", Extensions: []string{"json", "opml", "xml"}}}})
	if err != nil || len(ps) == 0 {
		return "", nil, err
	}
	b, err := os.ReadFile(ps[0])
	return filepath.Base(ps[0]), b, err
}
func (h *host) Notify(title, body, url string) {
	if !mygo.NotificationsSupported() {
		return
	}
	n := mygo.NewNotification(mygo.NotificationOptions{Title: title, Body: body})
	h.mu.Lock()
	h.urls[n.ID()] = url
	h.mu.Unlock()
	if err := n.Show(); err != nil {
		log.Println("notify:", err)
	}
}
func (h *host) Updates() app.Updates {
	if h.upd == nil {
		return nil
	}
	return h.upd
}

// updates wraps MyGo's ed25519 updater (packaged builds) plus a plain GitHub check used as the fallback prompt.
type updates struct {
	mu           sync.Mutex
	latest, link string
	packaged     bool
}

func (u *updates) Version() string { return mygo.App.Version() }
func (u *updates) Auto() bool      { return updater.AutomaticChecks() }
func (u *updates) SetAuto(on bool) { updater.SetAutomaticChecks(on) }
func (u *updates) Check() {
	updater.CheckForUpdates()
	go u.fallbackCheck()
}
func (u *updates) Fallback() (string, string) {
	u.mu.Lock()
	defer u.mu.Unlock()
	return u.latest, u.link
}
func (u *updates) fallbackCheck() {
	v, link, err := core.LatestRelease(repo, tagPrefix)
	if err != nil || v == "" || !core.NewerVersion(v, mygo.App.Version()) {
		return
	}
	u.mu.Lock()
	u.latest, u.link = v, link
	u.mu.Unlock()
}

func userData() string {
	d, err := mygo.App.Path(mygo.PathUserData)
	if err != nil {
		d = filepath.Join(os.TempDir(), "ai-news-radar")
	}
	return d
}

func main() {
	mygo.Use(updnative.New(updater.Options{Interval: 12 * time.Hour, Language: "zh-Hans"}))
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	h := &host{urls: map[string]string{}, upd: &updates{packaged: mygo.App.IsPackaged()}}
	var quitting bool
	show := func() {
		if h.win != nil {
			h.win.Show()
			h.win.Focus()
		}
	}
	mygo.App.OnSecondInstance(func([]string, string) { show() })
	mygo.App.OnActivate(func(bool) { show() })

	mygo.App.WhenReady(func() {
		base := userData()
		dir := filepath.Join(base, "native")
		if d := os.Getenv("AI_RADAR_DATA"); d != "" {
			dir = d
		}
		// one-time, non-destructive import of the webview build's data (<UserData>/news.db + secrets.json)
		mig, err := core.MigrateFromWebview(base, dir)
		if err != nil {
			log.Println("migrate:", err)
		}
		st, err := core.Open(dir, seedJSON)
		if err != nil {
			mygo.Dialog.Message(mygo.MessageOptions{Type: mygo.MessageError, Message: "无法打开数据库", Detail: err.Error()})
			mygo.App.Quit()
			return
		}
		cache, _ := mygo.App.Path(mygo.PathCache)
		st.Thumbs = core.NewThumbCache(filepath.Join(cache, "thumbs"), thumbLimit)
		sched := st.NewScheduler()
		a := app.New(st, sched, h)
		a.SetMigrated(mig)
		st.OnEvent = a.OnEvent
		mygo.App.OnNotificationClick(func(id string) {
			h.mu.Lock()
			u := h.urls[id]
			h.mu.Unlock()
			if u != "" {
				h.OpenURL(u)
			}
			show()
		})
		refresh := func() { h.Post(a.Refresh) }
		mygo.App.SetMenu(mygo.NewMenu([]*mygo.MenuItem{
			{Role: mygo.RoleAppMenu},
			{Role: mygo.RoleEditMenu},
			{Label: "视图", Submenu: []*mygo.MenuItem{
				{Label: "立即抓取全部", Accelerator: "CmdOrCtrl+R", Click: func(*mygo.MenuItem, *mygo.Window) { refresh() }},
				{Label: "打开数据目录", Click: func(*mygo.MenuItem, *mygo.Window) { go mygo.Shell.OpenPath(st.Dir) }},
			}},
			{Label: "帮助", Submenu: []*mygo.MenuItem{
				{Label: "检查更新…", Click: func(*mygo.MenuItem, *mygo.Window) { h.upd.Check() }},
			}},
			{Role: mygo.RoleWindowMenu},
		}))
		h.win = mygo.NewWindow(mygo.WindowOptions{Title: appName, Width: 1180, Height: 780, Content: ui.View(a.View)})
		h.win.OnClose(func(e *mygo.CloseEvent) { // closing hides to the tray; 退出 quits
			if !quitting {
				e.PreventDefault()
				h.win.Hide()
			}
		})
		quit := func() { quitting = true; mygo.App.Quit() }

		tray, err := mygo.NewTray(mygo.TrayOptions{Icon: trayIcon, IconIsTemplate: true, ToolTip: appName,
			Menu: mygo.NewMenu([]*mygo.MenuItem{
				{Label: "显示窗口", Click: func(*mygo.MenuItem, *mygo.Window) { show() }},
				{Label: "立即抓取", Click: func(*mygo.MenuItem, *mygo.Window) { refresh() }},
				{Label: "检查更新…", Click: func(*mygo.MenuItem, *mygo.Window) { h.upd.Check() }},
				mygo.Separator(),
				{Label: "退出 " + appName, Click: func(*mygo.MenuItem, *mygo.Window) { quit() }},
			})})
		if err != nil {
			log.Println("tray:", err) // e.g. Linux without AppIndicator; the app menu still works
		}
		// unread count next to the tray icon (macOS shows the title; elsewhere the tooltip)
		go func() {
			last := -1
			for {
				h.Post(func() {
					n := a.UnreadTotal()
					if tray == nil || n == last {
						return
					}
					last = n
					title, tip := "", appName
					if n > 0 {
						title, tip = fmt.Sprint(n), fmt.Sprintf("%s · %d 条未读", appName, n)
					}
					tray.SetTitle(title)
					tray.SetToolTip(tip)
				})
				time.Sleep(15 * time.Second)
			}
		}()
		if err := mygo.GlobalShortcut.Register("CmdOrCtrl+Shift+R", func() {
			if h.win.IsVisible() && h.win.IsFocused() {
				h.win.Hide()
			} else {
				show()
			}
		}); err != nil {
			log.Println("shortcut:", err)
		}
		sched.Start()
		go h.upd.fallbackCheck()
		mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting = true; sched.Stop(); st.Close() })
	})
	if err := mygo.App.Run(); err != nil {
		log.Fatal(err)
	}
}
