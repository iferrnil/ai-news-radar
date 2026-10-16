// Package app is the native (MyGo ui, no webview) interface of AI News Radar.
// All state lives in App; views are pure functions of it, rebuilt by MyGo after input or App.post.
package app

import (
	"context"
	"fmt"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/iferrnil/ai-news-radar/desktop-native/core"
)

// Host is what the shell (main.go) provides; tests use a fake.
type Host interface {
	Post(fn func())   // run fn on the UI thread and rebuild (Window.Update)
	OpenURL(u string) // open in the default browser
	SaveFile(name string, data []byte) error
	OpenFile() (name string, data []byte, err error)
	Notify(title, body, url string)
	Updates() Updates // nil when the app is not packaged / updates unavailable
}

// Updates is the updater as the settings page sees it (MyGo ed25519 updater + GitHub fallback check).
type Updates interface {
	Version() string
	Auto() bool
	SetAuto(on bool)
	Check()
	// Fallback: newer release found by the plain GitHub check (when the in-app updater can't install it).
	Fallback() (version, url string)
}

type App struct {
	st    *core.Store
	sched *core.Scheduler
	host  Host

	sel        string // sidebar choice: all | sec:<key> | src:<id> | page:sources | page:settings
	sections   []core.Section
	sources    []core.Source
	items      []core.Item
	unread     map[string]int
	secOpen    map[string]*bool // per-section 数据源 group open
	navOpen    bool
	err        string
	lastReload time.Time

	// feed toolbar
	view       int // 0 卡片 1 列表
	unreadOnly bool
	search     string
	showZh     bool
	showSrcs   bool // 侧栏显示数据源
	list       ui.ListState

	thumbs          *thumbs
	detail          *core.Item // 详情抽屉
	health          []core.Health
	migrated        core.MigrateResult
	dismissedUpdate bool

	// dialogs
	dlg dialogs
	// settings form
	set settingsForm
}

type dialogs struct {
	add, rename, filter, newSec, editSec, delSrc, delSec, importing bool

	addInput, addSection, addPriority string
	addBusy                           bool
	addErr                            string
	detected                          *core.Detected

	target    core.Source
	renameTo  string
	rule      core.Rule
	ruleAll   bool
	ruleErr   string
	secName   string
	secIcon   string
	secInAll  bool
	secTarget core.Section
	moveTo    string
	impRows   []core.ParsedRow
	impMode   int // 0 merge 1 update 2 replace
	impPlan   *core.ImportPlan
	impName   string
	impErr    string
}

type settingsForm struct {
	loaded         bool
	notify         bool
	translate      bool
	key, base, mdl string
	keySet         bool
	msg            string
}

func New(st *core.Store, sched *core.Scheduler, host Host) *App {
	a := &App{st: st, sched: sched, host: host, sel: "all", secOpen: map[string]*bool{}, navOpen: true, showZh: true, showSrcs: true}
	a.thumbs = newThumbs(st.Thumbs)
	if v, ok := st.GetSetting("desktop_view"); ok {
		a.view, _ = strconv.Atoi(v)
	}
	if v, ok := st.GetSetting("desktop_show_sources"); ok {
		a.showSrcs = v == "1"
	}
	a.list.Key = func(i int) any {
		if i < len(a.items) {
			return a.items[i].ID
		}
		return i
	}
	a.Reload()
	return a
}

// OnEvent is wired to Store.OnEvent (any goroutine).
func (a *App) OnEvent(e core.Event) {
	if e.Kind == "notify" && e.Batch != nil {
		b := e.Batch
		title := "AI News Radar · P0 快讯"
		body := b.Items[0].Title
		if b.Count > 1 {
			body = fmt.Sprintf("%d 条新快讯：%s", b.Count, body)
		}
		a.host.Notify(title, body, b.Items[0].URL)
		return
	}
	a.host.Post(a.Reload)
}

// Reload re-reads everything the views show (UI thread).
func (a *App) Reload() {
	a.lastReload = time.Now()
	var err error
	if a.sections, err = a.st.Sections(); err != nil {
		a.err = err.Error()
	}
	if a.sources, err = a.st.Sources(); err != nil {
		a.err = err.Error()
	}
	a.unread = a.st.UnreadCounts()
	q := core.Query{UnreadOnly: a.unreadOnly, Search: a.search, Limit: 1000}
	switch {
	case strings.HasPrefix(a.sel, "sec:"):
		q.Section = a.sel[4:]
	case strings.HasPrefix(a.sel, "src:"):
		q.SourceID, _ = strconv.ParseInt(a.sel[4:], 10, 64)
	}
	if a.items, err = a.st.Items(q); err != nil {
		a.err = err.Error()
	}
	if a.sel == "page:health" {
		a.health, _ = a.st.Health()
	}
	if a.detail != nil { // keep the drawer's item fresh (translation may have arrived)
		for i := range a.items {
			if a.items[i].ID == a.detail.ID {
				a.detail = &a.items[i]
			}
		}
	}
	for _, s := range a.sections {
		if a.secOpen[s.Key] == nil {
			b := false
			a.secOpen[s.Key] = &b
		}
	}
}

func (a *App) sectionName(key string) string {
	for _, s := range a.sections {
		if s.Key == key {
			return s.Name
		}
	}
	return "未分类"
}

func (a *App) sectionKeyByName(name string) string {
	for _, s := range a.sections {
		if s.Name == name {
			return s.Key
		}
	}
	return ""
}

func (a *App) sectionNames() []string {
	out := make([]string, len(a.sections))
	for i, s := range a.sections {
		out[i] = s.Name
	}
	return out
}

func (a *App) title() string {
	switch {
	case a.sel == "all":
		return "全部"
	case strings.HasPrefix(a.sel, "sec:"):
		return a.sectionName(a.sel[4:])
	case strings.HasPrefix(a.sel, "src:"):
		id, _ := strconv.ParseInt(a.sel[4:], 10, 64)
		for _, s := range a.sources {
			if s.ID == id {
				return s.Name
			}
		}
	}
	return ""
}

// View is the window content.
func (a *App) View(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Fill().Children(func() {
		a.sidebar(c)
		ui.Box(c).Width(1).FillHeight().Background(t.Border)
		ui.Column(c).Grow(1).FillHeight().MinWidth(0).Children(func() {
			a.banner(c)
			switch a.sel {
			case "page:sources":
				a.sourcesPage(c)
			case "page:settings":
				a.settingsPage(c)
			case "page:health":
				a.healthPage(c)
			default:
				a.feed(c)
			}
		})
	})
	a.dialogs(c)
}

// UnreadTotal is the 全部 unread count (tray title).
func (a *App) UnreadTotal() int { return a.unread[""] }

// Refresh polls every enabled source now (tray / menu).
func (a *App) Refresh() { prev := a.sel; a.sel = "all"; a.refreshNow(); a.sel = prev }

// SetMigrated shows a one-time banner after importing the webview build's data.
func (a *App) SetMigrated(r core.MigrateResult) { a.migrated = r }

func (a *App) banner(c *ui.Context) {
	t := c.Theme()
	bar := func(text, action string, fn func(), close func()) {
		ui.Row(c).Padding(8, 16).Gap(10).AlignItems(ui.Center).Background(t.Surface).Children(func() {
			ui.Text(c, text).FontSize(13).Grow(1).Shrink(1)
			if action != "" && ui.PrimaryButton(c, action).Clicked() {
				fn()
			}
			if ui.Button(c, "关闭").Clicked() {
				close()
			}
		})
	}
	if a.migrated.Done {
		bar(fmt.Sprintf("已从旧版（网页版窗口）导入数据：%d 个数据源、%d 条新闻、%d 条已读记录%s。旧数据未改动。",
			a.migrated.Sources, a.migrated.Items, a.migrated.Read, map[bool]string{true: "、DeepSeek 设置", false: ""}[a.migrated.Secrets]),
			"", nil, func() { a.migrated.Done = false })
	}
	if u := a.host.Updates(); u != nil {
		if v, link := u.Fallback(); v != "" && !a.dismissedUpdate {
			bar("发现新版本 "+v+"（当前 "+u.Version()+"）", "前往下载", func() { a.host.OpenURL(link) }, func() { a.dismissedUpdate = true })
		}
	}
}

func (a *App) sidebar(c *ui.Context) {
	prev := a.sel
	sb := ui.Sidebar(c, &a.sel, func() {
		ui.SidebarSection(c, "分区", nil, func() {
			ui.SidebarItem(c, "all", nil, "全部").Children(func() { a.badge(c, a.unread[""]) })
			for _, s := range a.sections {
				ui.SidebarItem(c.Key("sec:"+s.Key), "sec:"+s.Key, nil, iconText(s.Icon)+s.Name).Children(func() { a.badge(c, a.unread[s.Key]) })
			}
		})
		if a.showSrcs {
			for _, s := range a.sections {
				var srcs []core.Source
				for _, x := range a.sources {
					if x.Section == s.Key && x.Supported {
						srcs = append(srcs, x)
					}
				}
				if len(srcs) == 0 {
					continue
				}
				ui.SidebarSection(c.Key("grp:"+s.Key), s.Name+" · 数据源", a.secOpen[s.Key], func() {
					for _, x := range srcs {
						label := x.Name
						if !x.Enabled {
							label += "（已停用）"
						}
						ui.SidebarItem(c.Key(x.ID), "src:"+strconv.FormatInt(x.ID, 10), nil, label)
					}
				})
			}
		}
		ui.SidebarSection(c, "管理", nil, func() {
			ui.SidebarItem(c, "page:sources", nil, "数据源管理")
			ui.SidebarItem(c, "page:health", nil, "运行状态")
			ui.SidebarItem(c, "page:settings", nil, "设置")
		})
	}).Width(240).FillHeight()
	if sb.Changed() || prev != a.sel {
		a.Reload()
		a.list.Handle = ui.Handle{}
	}
}

// iconText shows emoji icons; lucide:* icons (web-only SVG names) are dropped.
func iconText(icon string) string {
	if icon == "" || strings.HasPrefix(icon, "lucide:") {
		return ""
	}
	return icon + " "
}

func (a *App) badge(c *ui.Context, n int) {
	if n > 0 {
		ui.Badge(c, strconv.Itoa(n))
	}
}

// ---------------------------------------------------------------- feed

func ago(ts float64) string {
	d := time.Since(time.Unix(int64(ts), 0))
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	}
	return time.Unix(int64(ts), 0).Format("01-02 15:04")
}

func (a *App) feed(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(10, 16).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, a.title()).FontSize(18).Bold().NoWrap()
		ui.Textf(c, "%d 条", len(a.items)).TextColor(t.TextMuted).NoWrap()
		ui.Spacer(c)
		if ui.TextInput(c, &a.search).Placeholder("搜索标题 / 摘要").Label("搜索").Width(220).Changed() {
			a.Reload()
		}
		if ui.Segmented(c, &a.view, "卡片", "列表").Label("视图").Changed() {
			a.st.SetSetting("desktop_view", strconv.Itoa(a.view))
		}
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "仅未读").NoWrap()
			if ui.Switch(c, &a.unreadOnly).Label("仅未读").Changed() {
				a.Reload()
			}
		})
		if a.st.TranslateEnabled() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, "译文").NoWrap()
				ui.Switch(c, &a.showZh).Label("显示译文")
			})
		}
		if ui.Button(c, "全部已读").Disabled(len(a.items) == 0).Clicked() {
			ids := make([]int64, len(a.items))
			for i, it := range a.items {
				ids[i] = it.ID
			}
			a.st.SetRead(ids, true)
			a.Reload()
		}
		if ui.Button(c, "刷新").Tooltip("立即抓取当前视图的数据源").Clicked() {
			a.refreshNow()
			c.Toast("正在抓取…")
		}
	})
	ui.Divider(c)
	if a.err != "" {
		ui.Text(c, a.err).TextColor(t.Danger).Padding(8, 16)
	}
	gap := float32(10)
	if a.view == 1 {
		gap = 0
	}
	ui.Row(c).Grow(1).MinHeight(0).Children(func() {
		ui.List(c, &a.list, len(a.items), func(i int) { a.row(c, i) }).Grow(1).FillHeight().MinWidth(0).Gap(gap).Padding(12, 16).Children(func() {
			if len(a.items) == 0 {
				msg := "暂无新闻（保留最近 48 小时）"
				if a.unreadOnly {
					msg = "没有未读新闻"
				}
				ui.Text(c, msg).TextColor(t.TextMuted).Padding(24)
			}
		})
		if a.detail != nil {
			ui.Box(c).Width(1).FillHeight().Background(t.Border)
			a.drawer(c)
		}
	})
}

func (a *App) refreshNow() {
	targets := a.sources
	switch {
	case strings.HasPrefix(a.sel, "sec:"):
		targets = slices.DeleteFunc(slices.Clone(a.sources), func(s core.Source) bool { return s.Section != a.sel[4:] })
	case strings.HasPrefix(a.sel, "src:"):
		id, _ := strconv.ParseInt(a.sel[4:], 10, 64)
		targets = slices.DeleteFunc(slices.Clone(a.sources), func(s core.Source) bool { return s.ID != id })
	}
	if a.sched == nil {
		return
	}
	for _, s := range targets {
		if s.Enabled && s.Supported {
			go a.sched.PollOne(context.Background(), s)
		}
	}
}

func (a *App) open(it *core.Item) {
	a.host.OpenURL(it.URL)
	if !it.Read {
		a.st.SetRead([]int64{it.ID}, true)
		it.Read = true
		a.unread = a.st.UnreadCounts()
	}
}

func (a *App) itemMenu(it *core.Item) func(m *ui.Menu) {
	return func(m *ui.Menu) {
		if m.Item("查看详情").Chosen() {
			a.show(it)
		}
		if m.Item("在浏览器中打开").Chosen() {
			a.open(it)
		}
		label := "标记为已读"
		if it.Read {
			label = "标记为未读"
		}
		if m.Item(label).Chosen() {
			a.st.SetRead([]int64{it.ID}, !it.Read)
			it.Read = !it.Read
			a.unread = a.st.UnreadCounts()
		}
		for _, x := range it.AlsoOnX {
			if m.Item("打开 X 帖子 " + x.URL).Chosen() {
				a.host.OpenURL(x.URL)
			}
		}
		if it.Homepage != "" && m.Item("打开来源主页").Chosen() {
			a.host.OpenURL(it.Homepage)
		}
	}
}

func (a *App) texts(it *core.Item) (title, summary string) {
	title, summary = it.Title, it.Summary
	if a.showZh && it.TitleZh != "" {
		title = it.TitleZh
		if it.SummaryZh != "" {
			summary = it.SummaryZh
		}
	}
	return
}

func (a *App) row(c *ui.Context, i int) {
	if i >= len(a.items) {
		return
	}
	it := &a.items[i]
	t := c.Theme()
	title, summary := a.texts(it)
	titleColor := t.Text
	if it.Read {
		titleColor = t.TextMuted
	}
	meta := func() {
		ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
			if it.Priority == "P0" {
				ui.Badge(c, "P0")
			}
			ui.Text(c, it.Source).FontSize(12).TextColor(t.TextMuted).NoWrap().Ellipsis("…").Shrink(1)
			ui.Text(c, "· "+ago(it.Published)).FontSize(12).TextColor(t.TextMuted).NoWrap()
			if len(it.AlsoOnX) > 0 {
				ui.Textf(c, "· X 上也在讨论（%d）", len(it.AlsoOnX)).FontSize(12).TextColor(t.Accent).NoWrap()
			}
		})
	}
	if a.view == 1 { // 列表
		r := ui.Row(c.Key(it.ID)).Gap(10).Padding(7, 6).AlignItems(ui.Center).Radius(6).Cursor(ui.CursorPointer)
		if r.Hovered() {
			r.Background(t.SurfaceHover)
		}
		r.Children(func() {
			dot := ui.Box(c).Size(6, 6).Radius(3)
			if !it.Read {
				dot.Background(t.Accent)
			}
			ui.Text(c, title).TextColor(titleColor).NoWrap().Ellipsis("…").Grow(1).Shrink(1).MinWidth(0)
			ui.Text(c, it.Source).FontSize(12).TextColor(t.TextMuted).NoWrap().MaxWidth(180).Ellipsis("…")
			ui.Text(c, ago(it.Published)).FontSize(12).TextColor(t.TextMuted).NoWrap().Width(84).TextAlign(ui.End)
		}).Tooltip(summary).ContextMenu(a.itemMenu(it))
		if r.DoubleClicked() {
			a.open(it)
		} else if r.Clicked() {
			a.show(it)
		}
		return
	}
	card := ui.Column(c.Key(it.ID)).Gap(6).Padding(12, 14).Radius(10).Border(1, t.Border).Cursor(ui.CursorPointer)
	if card.Hovered() {
		card.Background(t.SurfaceHover)
	}
	if a.detail != nil && a.detail.ID == it.ID {
		card.Border(1, t.Accent)
	}
	card.Children(func() {
		ui.Row(c).Gap(12).AlignItems(ui.Start).Children(func() {
			ui.Column(c).Gap(6).Grow(1).MinWidth(0).Children(func() {
				meta()
				ui.Text(c, title).FontSize(15).Bold().TextColor(titleColor).MaxLines(2).Ellipsis("…")
				if summary != "" && summary != title {
					ui.Text(c, summary).FontSize(13).TextColor(t.TextMuted).MaxLines(3).Ellipsis("…")
				}
			})
			if it.Image != "" {
				if bmp := a.thumb(it.Image, refererFor(it)); bmp != nil {
					ui.Image(c, bmp).Size(144, 81).Fit(ui.Cover).Radius(6)
				} else if s := a.thumbs.m[it.Image]; s != nil && s.pending {
					ui.Box(c).Size(144, 81).Radius(6).Background(t.Surface) // placeholder while loading
				}
			}
		})
	}).ContextMenu(a.itemMenu(it))
	if card.DoubleClicked() {
		a.open(it)
	} else if card.Clicked() {
		a.show(it)
	}
}
