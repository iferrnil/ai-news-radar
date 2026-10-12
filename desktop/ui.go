package main

import (
	"fmt"
	"runtime"

	"github.com/egoist/mygo"
)

// windowOptions: remembered size/position (window-state.json in the data dir), a minimum size that still fits the
// two-column feed, and on macOS a hidden title bar with the traffic lights inset into the 56px app header.
func windowOptions() mygo.WindowOptions {
	o := mygo.WindowOptions{Title: appName, Width: 1280, Height: 840, MinWidth: 720, MinHeight: 480,
		StateKey: "main", BackgroundColor: "light-dark(#fcfcfd, #0e0e11)",
		Page: mygo.PageOptions{PreloadScript: pagePreloadJS(runtime.GOOS == "darwin")}}
	if runtime.GOOS == "darwin" {
		o.TitleBarStyle = mygo.TitleBarHiddenInset
		o.TrafficLightPosition = &mygo.Point{X: 18, Y: 21} // vertically centred in the h-14 header
	}
	return o
}

// pagePreloadJS runs before the page's scripts: marks the document as the desktop app (no text selection on chrome,
// ⌘F/⌘, handled in-page, drag regions) and makes the theme follow the system until the user picks one.
func pagePreloadJS(titlebar bool) string {
	tb := ""
	if titlebar {
		tb = `window.__radarTitlebar=true;`
	}
	// (the page's inline <head> script turns the flags into <html data-desktop data-titlebar>)
	return fmt.Sprintf(`(function(){try{if(!/^https?:$/.test(location.protocol))return;window.__radarDesktop=true;%sif(!localStorage.getItem("theme"))localStorage.setItem("theme","system")}catch(e){}})();`, tb)
}

func pageCmdJS(c string) string {
	return fmt.Sprintf(`typeof window.__radarCmd === "function" ? (window.__radarCmd(%q), true) : false`, c)
}

// firstContentJS: the feed shows a card, or an empty/error state.
const firstContentJS = `!!document.querySelector('main [data-slot="card"], main [data-slot="empty"]')`

func appMenu(items []*mygo.MenuItem, cmd func(string) func(*mygo.MenuItem, *mygo.Window)) []*mygo.MenuItem {
	app := []*mygo.MenuItem{{Role: mygo.RoleAbout}, mygo.Separator(),
		{Label: "设置…", Accelerator: "CmdOrCtrl+,", Click: cmd("settings")}, mygo.Separator()}
	return []*mygo.MenuItem{
		{Label: appName, Submenu: append(app, items...)},
		{Label: "编辑", Submenu: []*mygo.MenuItem{{Role: mygo.RoleUndo}, {Role: mygo.RoleRedo}, mygo.Separator(), {Role: mygo.RoleCut}, {Role: mygo.RoleCopy}, {Role: mygo.RolePaste}, {Role: mygo.RoleSelectAll},
			mygo.Separator(), {Label: "搜索…", Accelerator: "CmdOrCtrl+F", Click: cmd("search")}}},
		{Label: "视图", Submenu: []*mygo.MenuItem{{Label: "显示/隐藏侧边栏", Accelerator: "CmdOrCtrl+B", Click: cmd("sidebar")}, mygo.Separator(),
			{Role: mygo.RoleResetZoom}, {Role: mygo.RoleZoomIn}, {Role: mygo.RoleZoomOut}, mygo.Separator(), {Role: mygo.RoleToggleFullScreen}}},
		{Role: mygo.RoleWindow},
	}
}
