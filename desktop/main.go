// AI News Radar desktop shell (MyGo): runs the Go backend in-process on 127.0.0.1 (static Next.js UI embedded
// in the binary) and shows it in a native window with a tray/menu.
package main

import (
	"context"
	_ "embed"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/updater"
)

//go:embed resources/tray.png
var trayIcon []byte

const appName = "AI News Radar"

func main() {
	// Sparkle-style updates from GitHub Releases (desktop-v* tags, ed25519-signed archives, see mygo.json "updates"):
	// a background check twice a day (first ~10 s after launch when due) + 检查更新… in the app and tray menus.
	mygo.Use(updater.New(updater.Options{Interval: 12 * time.Hour, Language: "zh-Hans"}))
	if !mygo.App.RequestSingleInstanceLock() {
		return // the running instance gets OnSecondInstance and comes to the front
	}
	var (
		srv  *Server
		win  *mygo.Window
		data string
	)
	show := func() {
		if win != nil {
			win.Show()
			win.Focus()
		}
	}
	mygo.App.OnSecondInstance(func([]string, string) { show() })
	mygo.App.OnActivate(func(bool) { show() })

	started := time.Now()
	mygo.App.WhenReady(func() {
		var err error
		data, err = mygo.App.Path(mygo.PathUserData)
		if err != nil {
			fatal("data dir", err)
			return
		}
		_ = os.MkdirAll(data, 0o755)
		logf, _ := os.OpenFile(filepath.Join(data, "server.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		var lw io.Writer = os.Stderr
		if logf != nil {
			lw = io.MultiWriter(os.Stderr, logf)
		}

		// debug: `touch <data>/debug-webview-transparent` restores MyGo's upstream see-through WKWebView
		// (drawsBackground=false) to compare scrolling; see third_party/mygo/PATCHES.md
		if _, err := os.Stat(filepath.Join(data, "debug-webview-transparent")); err == nil {
			os.Setenv("MYGO_WEBVIEW_TRANSPARENT", "1")
		}
		win = mygo.NewWindow(windowOptions())
		win.Page().LoadHTML(splashHTML, "")
		// closing the window hides it (the app keeps running in the tray); 退出 quits
		win.OnClose(func(e *mygo.CloseEvent) {
			if !quitting.Load() {
				e.PreventDefault()
				win.Hide()
			}
		})

		nt := newNotifier()
		nt.install(show, data)
		nt.start() // asks for notification permission once (system prompt), reads the real status
		srv, err = StartServer(ServerConfig{DataDir: data, Log: lw, Port: PreferredPort, Version: mygo.App.Version()})
		if err != nil {
			fatal("启动内置服务失败", err)
			return
		}
		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if err := srv.WaitHealthy(ctx); err != nil {
				fatal("内置服务未就绪（详见数据目录 server.log）", err)
				return
			}
			healthy := time.Since(started)
			var offDOM func()
			offDOM = win.Page().OnDOMReady(func() {
				offDOM()
				dom := time.Since(started)
				go func() { // first feed card (or an empty/error state) on screen = "first content"
					for i := 0; i < 400; i++ {
						if ok, err := win.Page().Eval(firstContentJS); err == nil && ok == true {
							log.Printf("startup: healthy=%v dom=%v content=%v", healthy.Round(time.Millisecond), dom.Round(time.Millisecond), time.Since(started).Round(time.Millisecond))
							return
						}
						time.Sleep(50 * time.Millisecond)
					}
				}()
			})
			win.Page().LoadURL(srv.StartURL())
			go watchNotifications(appCtx, srv.URL, func(b notifyBatch) { notifyP0(nt, win, srv.URL, b, show) })
		}()
		go func() { // the server died on its own: tell the user instead of a blank window
			<-srv.Done()
			if !quitting.Load() {
				win.Page().LoadHTML(errorHTML("内置服务已停止，请重启应用。日志：server.log"), "")
			}
		}()

		// links: our own origin stays in the window, everything else goes to the default browser
		// (only http / https / mailto ever reach the OS: a file://, smb:// or app-scheme link from a feed would
		// otherwise open a local file or app; see externalURL)
		own := func(u string) bool { return srv != nil && sameOrigin(u, srv.URL) }
		win.Page().SetWindowOpenHandler(func(req mygo.WindowOpenRequest) *mygo.WindowOptions {
			if externalURL(req.URL) {
				go mygo.Shell.OpenExternal(req.URL)
			}
			return nil
		})
		win.Page().OnWillNavigate(func(e *mygo.NavigateEvent) {
			if own(e.URL) || strings.HasPrefix(e.URL, "about:") || strings.HasPrefix(e.URL, "blob:"+srv.URL+"/") {
				return // our pages, blank frames, our own downloads (blob:)
			}
			e.PreventDefault() // nothing else loads inside the app window
			if externalURL(e.URL) {
				go mygo.Shell.OpenExternal(e.URL)
			}
		})

		// Soft History API nav does not fire these; a line here during a sidebar click means a hard document load
		// (out/*.html) — the empty Suspense body until hydrate looks like a full-webview black flash.
		win.Page().OnDidNavigate(func(url string) { log.Printf("nav: did-navigate %s", url) })
		win.Page().OnPageTitleUpdated(func(e *mygo.TitleEvent) {
			// Keep NSWindow title = appName. Page <title> changes on every route/unread bump; AppKit relayouts the
			// hidden inset title bar on setTitle, which can blank the WKWebView content behind the traffic lights.
			e.PreventDefault()
		})

		items := func() []*mygo.MenuItem {
			return []*mygo.MenuItem{
				{Label: "显示 / 隐藏", Accelerator: "CmdOrCtrl+Shift+R", Click: func(*mygo.MenuItem, *mygo.Window) {
					if win.IsVisible() {
						win.Hide()
					} else {
						show()
					}
				}},
				{Label: "刷新", Accelerator: "CmdOrCtrl+R", Click: func(*mygo.MenuItem, *mygo.Window) { show(); win.Page().Reload() }},
				{Label: "打开数据目录", Click: func(*mygo.MenuItem, *mygo.Window) { go mygo.Shell.OpenPath(data) }},
				{Label: "检查更新…", Click: func(*mygo.MenuItem, *mygo.Window) { updater.CheckForUpdates() }},
				mygo.Separator(),
				{Label: "退出 " + appName, Accelerator: "CmdOrCtrl+Q", Click: func(*mygo.MenuItem, *mygo.Window) { quit() }},
			}
		}
		// page commands (window.__radarCmd in app-shell.tsx); the page also handles the same keys itself and ignores a repeat
		cmd := func(c string) func(*mygo.MenuItem, *mygo.Window) {
			return func(*mygo.MenuItem, *mygo.Window) {
				show()
				go func() { _, _ = win.Page().Eval(pageCmdJS(c)) }()
			}
		}
		mygo.App.SetMenu(mygo.NewMenu(appMenu(items(), cmd)))
		if _, err := mygo.NewTray(mygo.TrayOptions{Icon: trayIcon, IconIsTemplate: true, ToolTip: appName, Menu: mygo.NewMenu(items())}); err != nil {
			log.Println("tray:", err) // e.g. Linux without AppIndicator; the app menu still works
		}
	})

	// graceful shutdown: stop the poller and close SQLite before the process exits
	// any quit (menu, Cmd+Q, Dock, SIGTERM) must let the window really close instead of hiding
	mygo.App.OnBeforeQuit(func(*mygo.QuitEvent) { quitting.Store(true); stopApp() })
	mygo.App.OnWillQuit(func(*mygo.QuitEvent) { quitting.Store(true); stopServer(srv) })
	mygo.App.OnQuit(func() { stopServer(srv) })
	if err := mygo.App.Run(); err != nil {
		stopServer(srv)
		log.Fatal(err)
	}
	stopServer(srv)
}

var quitting atomic.Bool

// appCtx ends when the app quits (stops the notification stream before the server goes away).
var appCtx, stopApp = context.WithCancel(context.Background())

// notifyP0 shows one native notification per batch; a click brings the window up and opens the (first) item.
func notifyP0(nt *notifier, win *mygo.Window, base string, b notifyBatch, show func()) {
	title, body := batchText(b)
	id := b.Items[0].ID
	_ = nt.show(title, body, func() {
		show()
		go func() {
			ok, err := win.Page().Eval(fmt.Sprintf("typeof window.__radarOpenItem === 'function' ? (window.__radarOpenItem(%d), true) : false", id))
			if err != nil || ok != true {
				_ = win.Page().LoadURL(fmt.Sprintf("%s/?item=%d", base, id))
			}
		}()
	})
}

func quit() { quitting.Store(true); mygo.App.Quit() }

func stopServer(s *Server) {
	if s != nil {
		t := time.Now()
		err := s.Stop()
		log.Printf("server stopped in %v (err=%v)", time.Since(t).Round(time.Millisecond), err)
	}
}

func fatal(what string, err error) {
	log.Printf("%s: %v", what, err)
	for _, w := range mygo.Windows() {
		w.Page().LoadHTML(errorHTML(fmt.Sprintf("%s：%v", what, err)), "")
	}
}

const page = `<!doctype html><meta charset="utf-8"><style>html{color-scheme:light dark}body{margin:0;height:100vh;display:grid;place-items:center;font:15px -apple-system,"PingFang SC",sans-serif;background:light-dark(#fff,#0b0b0f);color:light-dark(#333,#ccc)}.b{display:flex;flex-direction:column;align-items:center;gap:14px}.l{width:56px;height:56px;border-radius:14px;background:linear-gradient(#8b5cf6,#4f46e5)}</style><div class="b"><div class="l"></div><div>%s</div></div>`

var splashHTML = fmt.Sprintf(page, "AI News Radar 正在启动…")

func errorHTML(msg string) string {
	return fmt.Sprintf(page, strings.NewReplacer("&", "&amp;", "<", "&lt;", ">", "&gt;").Replace(msg))
}
