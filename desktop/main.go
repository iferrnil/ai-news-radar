// AI News Radar — experimental pure MyGo native UI shell (no webview chrome).
// Runs the Go radar-server in-process and draws the UI with package ui + glass.
package main

import (
	"context"
	_ "embed"
	"io"
	"log"
	"os"
	"path/filepath"
	"sync/atomic"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

//go:embed resources/tray.png
var trayIcon []byte

const appName = "AI News Radar Native"

func main() {
	// Experimental channel: no Sparkle updater — avoids confusing the web (desktop-v*) channel.
	if !mygo.App.RequestSingleInstanceLock() {
		return
	}
	var (
		srv  *Server
		win  *mygo.Window
		app  *radarApp
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
			log.Printf("data dir: %v", err)
			return
		}
		_ = os.MkdirAll(data, 0o755)
		logf, _ := os.OpenFile(filepath.Join(data, "server.log"), os.O_CREATE|os.O_WRONLY|os.O_TRUNC, 0o644)
		var lw io.Writer = os.Stderr
		if logf != nil {
			lw = io.MultiWriter(os.Stderr, logf)
		}

		app = newRadarApp(nil, mygo.App.Version())
		win = mygo.NewWindow(windowOptions(ui.View(app.view)))
		app.win = win
		win.OnClose(func(e *mygo.CloseEvent) {
			if !quitting.Load() {
				e.PreventDefault()
				win.Hide()
			}
		})

		nt := newNotifier()
		nt.install(show, data)
		nt.start()

		srv, err = StartServer(ServerConfig{DataDir: data, Log: lw, Port: PreferredPort, Version: mygo.App.Version()})
		if err != nil {
			app.errMsg = err.Error()
			app.status = "启动失败"
			win.Invalidate()
			log.Printf("start server: %v", err)
			return
		}
		app.srv = srv

		go func() {
			ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
			defer cancel()
			if err := srv.WaitHealthy(ctx); err != nil {
				win.Update(func() {
					app.errMsg = err.Error()
					app.status = "服务未就绪"
					app.healthy = false
				})
				return
			}
			log.Printf("startup: healthy=%v", time.Since(started).Round(time.Millisecond))
			app.refreshAsync()
			go watchNotifications(appCtx, srv.URL, func(b notifyBatch) {
				notifyP0Native(nt, app, b, show)
			})
		}()
		go func() {
			<-srv.Done()
			if !quitting.Load() {
				win.Update(func() {
					app.errMsg = "内置服务已停止，请重启应用。日志：server.log"
					app.healthy = false
					app.status = "已停止"
				})
			}
		}()

		items := func() []*mygo.MenuItem {
			return []*mygo.MenuItem{
				{Label: "显示 / 隐藏", Accelerator: "CmdOrCtrl+Shift+R", Click: func(*mygo.MenuItem, *mygo.Window) {
					if win.IsVisible() {
						win.Hide()
					} else {
						show()
					}
				}},
				{Label: "刷新", Accelerator: "CmdOrCtrl+R", Click: func(*mygo.MenuItem, *mygo.Window) {
					show()
					app.refreshAsync()
				}},
				{Label: "打开数据目录", Click: func(*mygo.MenuItem, *mygo.Window) { go mygo.Shell.OpenPath(data) }},
				mygo.Separator(),
				{Label: "退出 " + appName, Accelerator: "CmdOrCtrl+Q", Click: func(*mygo.MenuItem, *mygo.Window) { quit() }},
			}
		}
		mygo.App.SetMenu(mygo.NewMenu(appMenuNative(items())))
		if _, err := mygo.NewTray(mygo.TrayOptions{Icon: trayIcon, IconIsTemplate: true, ToolTip: appName, Menu: mygo.NewMenu(items())}); err != nil {
			log.Println("tray:", err)
		}
	})

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

var appCtx, stopApp = context.WithCancel(context.Background())

func notifyP0Native(nt *notifier, app *radarApp, b notifyBatch, show func()) {
	title, body := batchText(b)
	id := b.Items[0].ID
	_ = nt.show(title, body, func() {
		show()
		if app != nil && app.win != nil {
			app.win.Update(func() { app.selectItemID(id) })
		}
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

func appMenuNative(items []*mygo.MenuItem) []*mygo.MenuItem {
	app := []*mygo.MenuItem{{Role: mygo.RoleAbout}, mygo.Separator()}
	return []*mygo.MenuItem{
		{Label: appName, Submenu: append(app, items...)},
		{Label: "编辑", Submenu: []*mygo.MenuItem{{Role: mygo.RoleUndo}, {Role: mygo.RoleRedo}, mygo.Separator(), {Role: mygo.RoleCut}, {Role: mygo.RoleCopy}, {Role: mygo.RolePaste}, {Role: mygo.RoleSelectAll}}},
		{Label: "视图", Submenu: []*mygo.MenuItem{{Role: mygo.RoleToggleFullScreen}}},
		{Role: mygo.RoleWindow},
	}
}
