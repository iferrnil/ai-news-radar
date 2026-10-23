package main

// Pure MyGo native UI shell (no webview chrome). Demo-quality preview wired to radar-server APIs.
// See https://mygo.egoist.dev/docs/ui — themes, glass plugin, vibrancy, Sidebar, List, Split.

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/plugins/glass"
	"github.com/egoist/mygo/ui"
)

// nav keys for the sidebar (settings/about are pages; the rest are feed sections).
const (
	navAll       = "all"
	navSettings  = "settings"
	navAbout     = "about"
	navStubBrief = "stub-brief"
)

type radarApp struct {
	win *mygo.Window
	srv *Server

	nav          string // sidebar selection
	sectionsOpen bool
	toolsOpen    bool
	query        string
	status       string
	errMsg       string
	loading      bool
	healthy      bool
	version      string
	banner       string // 实验预览 note

	sections []sectionInfo
	items    []feedItem // raw from API
	shown    []feedItem // filtered for current nav + query
	selected int        // index into shown; -1 = none
	detail   *feedItem

	list        ui.ListState
	split       float32 // list pane width fraction for Split
	read        map[int64]bool
	lastRefresh time.Time
}

func newRadarApp(srv *Server, version string) *radarApp {
	a := &radarApp{
		srv:          srv,
		nav:          navAll,
		sectionsOpen: true,
		toolsOpen:    true,
		version:      version,
		selected:     -1,
		split:        0.42,
		read:         map[int64]bool{},
		banner:       "实验预览 · 纯原生 MyGo UI（非完整功能）",
		status:       "启动中…",
	}
	a.list.Selected = &a.selected
	a.list.Key = func(row int) any {
		if row >= 0 && row < len(a.shown) {
			return a.shown[row].ID
		}
		return row
	}
	a.list.Label = func(row int) string {
		if row >= 0 && row < len(a.shown) {
			return a.shown[row].DisplayTitle()
		}
		return ""
	}
	return a
}

func (a *radarApp) base() string {
	if a.srv == nil {
		return ""
	}
	return a.srv.URL
}

func (a *radarApp) setStatus(s string) {
	a.status = s
}

func (a *radarApp) refreshAsync() {
	base := a.base()
	if base == "" {
		return
	}
	a.loading = true
	a.errMsg = ""
	a.setStatus("刷新中…")
	if a.win != nil {
		a.win.Invalidate()
	}
	go func() {
		started := time.Now()
		ok, herr := fetchHealth(base)
		secs, serr := fetchSections(base)
		items, ierr := fetchItems(base, 400, "")
		errMsg := ""
		switch {
		case herr != nil:
			errMsg = herr.Error()
		case serr != nil:
			errMsg = serr.Error()
		case ierr != nil:
			errMsg = ierr.Error()
		}
		ver := a.version
		if v := fetchSettingsVersion(base); v != "" {
			ver = v
		}
		elapsed := time.Since(started).Round(time.Millisecond)
		if a.win == nil {
			return
		}
		a.win.Update(func() {
			a.healthy = ok && errMsg == ""
			a.sections = secs
			a.items = items
			a.version = ver
			a.errMsg = errMsg
			a.loading = false
			a.lastRefresh = time.Now()
			a.refilter()
			if errMsg != "" {
				a.setStatus("错误 · " + truncate(errMsg, 80))
			} else {
				a.setStatus(fmt.Sprintf("已同步 · %d 条 · %v", len(items), elapsed))
			}
		})
	}()
}

func (a *radarApp) refilter() {
	q := strings.ToLower(strings.TrimSpace(a.query))
	nav := a.nav
	out := a.shown[:0]
	if out == nil {
		out = []feedItem{}
	}
	for _, it := range a.items {
		if nav != navAll && nav != navSettings && nav != navAbout && nav != navStubBrief {
			if it.Section != nav {
				continue
			}
		}
		if q != "" {
			hay := strings.ToLower(it.DisplayTitle() + " " + it.DisplaySummary() + " " + it.Source)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		out = append(out, it)
	}
	a.shown = out
	if a.selected >= len(a.shown) {
		a.selected = len(a.shown) - 1
	}
	if a.selected >= 0 && a.selected < len(a.shown) {
		it := a.shown[a.selected]
		a.detail = &it
	} else if nav == navAll || (nav != navSettings && nav != navAbout && nav != navStubBrief) {
		// keep detail if still in shown; else clear
		if a.detail != nil {
			found := false
			for i, it := range a.shown {
				if it.ID == a.detail.ID {
					a.selected = i
					found = true
					break
				}
			}
			if !found {
				a.detail = nil
				a.selected = -1
			}
		}
	}
}

func (a *radarApp) openSelected() {
	if a.selected < 0 || a.selected >= len(a.shown) {
		return
	}
	it := a.shown[a.selected]
	a.detail = &it
	a.read[it.ID] = true
	go markRead(a.base(), it.ID)
}

func (a *radarApp) openURL(u string) {
	if externalURL(u) {
		go mygo.Shell.OpenExternal(u)
	}
}

func (a *radarApp) selectItemID(id int64) {
	for i, it := range a.shown {
		if it.ID == id {
			a.selected = i
			a.openSelected()
			return
		}
	}
	// not in current filter — load and show
	go func() {
		it, err := fetchItem(a.base(), id)
		if err != nil || a.win == nil {
			return
		}
		a.win.Update(func() {
			a.nav = navAll
			a.detail = it
			a.read[id] = true
			// try to select in shown after refilter
			a.refilter()
			for i, x := range a.shown {
				if x.ID == id {
					a.selected = i
					break
				}
			}
		})
	}()
}

// applyTheme forces a premium dark palette with violet accent (preview look).
func applyTheme(c *ui.Context) {
	t := *ui.DarkTheme()
	t.Background = ui.Hex("#0b0b10")
	t.Surface = ui.Hex("#14141c")
	t.SurfaceHover = ui.Hex("#1c1c28")
	t.SurfacePressed = ui.Hex("#242432")
	t.Border = ui.Hex("#2a2a38")
	t.Text = ui.Hex("#f4f4f8")
	t.TextMuted = ui.Hex("#9a9aaf")
	t.Accent = ui.Oklch(0.62, 0.22, 295) // violet
	t.AccentHover = ui.Oklch(0.68, 0.22, 295)
	t.AccentPressed = ui.Oklch(0.55, 0.2, 295)
	t.AccentText = ui.Hex("#ffffff")
	t.Focus = ui.RGBA(167, 139, 250, 0.55)
	t.Selection = ui.RGBA(167, 139, 250, 0.35)
	t.Radius = 10
	t.Spacing = 4
	t.Inverse = ui.Hex("#2a2a38")
	t.InverseText = ui.Hex("#f4f4f8")
	c.SetTheme(&t)
}

func (a *radarApp) view(c *ui.Context) {
	applyTheme(c)
	t := c.Theme()

	if c.Vibrancy() {
		c.Root().Background(ui.Transparent)
	} else {
		c.Root().Background(t.Background)
	}

	ui.Column(c).Fill().Children(func() {
		a.titleBar(c)
		ui.Row(c).Grow(1).AlignItems(ui.Stretch).Children(func() {
			a.sidebar(c)
			ui.Column(c).Grow(1).Background(t.Background).Children(func() {
				switch a.nav {
				case navSettings:
					a.settingsPage(c)
				case navAbout:
					a.aboutPage(c)
				case navStubBrief:
					a.stubPage(c, "简报 / 文章生成", "此实验预览未接入简报与文章生成。请使用正式桌面版（WebView）或网页端。")
				default:
					a.feedPage(c)
				}
			})
		})
	})
}

func (a *radarApp) titleBar(c *ui.Context) {
	t := c.Theme()
	bar := c.TitleBar()
	h := bar.Height
	if h < 44 {
		h = 44
	}
	row := ui.Row(c).Height(h).Padding(0, max32(bar.Right, 12)+8, 0, max32(bar.Left, 12)+8).
		AlignItems(ui.Center).Gap(10).DragWindow()
	if !c.Vibrancy() {
		row.Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(t.Border)
	} else {
		row.Material(glass.Glass{Style: glass.Regular})
	}
	row.Children(func() {
		ui.Text(c, "AI News Radar").Bold().FontSize(13)
		badge := ui.Row(c).Padding(3, 8).Radius(999).Background(ui.RGBA(167, 139, 250, 0.18)).Children(func() {
			ui.Text(c, "实验 · Native").FontSize(11).TextColor(t.Accent)
		})
		_ = badge
		ui.Box(c).Grow(1)
		if a.loading {
			ui.Spinner(c)
		}
		dot := t.Success
		if !a.healthy {
			dot = t.Danger
		}
		ui.Box(c).Size(8, 8).Radius(4).Background(dot)
		ui.Text(c, a.status).FontSize(11).TextColor(t.TextMuted)
	})
}

func (a *radarApp) sidebar(c *ui.Context) {
	t := c.Theme()
	side := ui.Column(c).Width(228).Padding(8, 8, 12, 8)
	if !c.Vibrancy() {
		side.Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(t.Border)
	}
	side.Children(func() {
		ui.Text(c, a.banner).FontSize(10).TextColor(t.TextMuted).Padding(4, 8, 8, 8)

		nav := a.nav
		sb := ui.Sidebar(c, &nav, func() {
			ui.SidebarSection(c, "资讯", &a.sectionsOpen, func() {
				ui.SidebarItem(c, navAll, nil, "全部")
				for _, s := range a.sections {
					ui.SidebarItem(c, s.Key, nil, s.Name)
				}
			})
			ui.SidebarSection(c, "更多", &a.toolsOpen, func() {
				ui.SidebarItem(c, navStubBrief, nil, "简报（未接入）")
				ui.SidebarItem(c, navSettings, nil, "设置")
				ui.SidebarItem(c, navAbout, nil, "关于")
			})
		}).Width(212).Grow(1)
		if sb.Changed() {
			a.nav = nav
			a.selected = -1
			a.detail = nil
			a.refilter()
		}

		ui.Row(c).Gap(6).Padding(8, 4, 0, 4).Children(func() {
			if ui.Button(c, "刷新").Grow(1).Clicked() {
				a.refreshAsync()
			}
		})
	})
}

func (a *radarApp) feedPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(12, 14, 14, 14).Gap(10).Children(func() {
		// glass toolbar
		tb := ui.Row(c).Padding(8, 10).Gap(8).Radius(16).AlignItems(ui.Center)
		tb.Material(glass.Glass{Style: glass.Regular})
		tb.Children(func() {
			sf := ui.SearchField(c, &a.query).Grow(1).Label("搜索标题 / 摘要 / 来源")
			if sf.Changed() {
				a.refilter()
			}
			n := len(a.shown)
			ui.Badge(c, fmt.Sprintf("%d", n))
			if ui.PrimaryButton(c, "打开链接").Clicked() {
				if a.detail != nil {
					a.openURL(a.detail.URL)
				}
			}
		})

		if a.errMsg != "" {
			ui.Row(c).Padding(10, 12).Radius(12).Background(ui.RGBA(239, 68, 68, 0.12)).Children(func() {
				ui.Text(c, "无法连接内置服务："+a.errMsg).TextColor(t.Danger).FontSize(12)
			})
		}

		ui.Split(c, &a.split, func() {
			a.itemList(c)
		}, func() {
			a.detailPane(c)
		}).Grow(1)
	})
}

func (a *radarApp) itemList(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Gap(6).Children(func() {
		ui.Text(c, sectionTitle(a.nav, a.sections)).Bold().FontSize(15)
		if len(a.shown) == 0 {
			ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
				ui.Text(c, "暂无资讯").TextColor(t.TextMuted)
				ui.Text(c, "点击侧栏「刷新」，或稍候订阅源拉取完成。").FontSize(12).TextColor(t.TextMuted)
			})
			return
		}
		lst := ui.List(c, &a.list, len(a.shown), func(row int) {
			if row < 0 || row >= len(a.shown) {
				return
			}
			it := a.shown[row]
			a.itemRow(c, it, row == a.selected)
		}).Grow(1)
		if lst.Changed() {
			a.openSelected()
		}
		if lst.Submitted() {
			a.openSelected()
			if a.detail != nil {
				a.openURL(a.detail.URL)
			}
		}
	})
}

func (a *radarApp) itemRow(c *ui.Context, it feedItem, selected bool) {
	t := c.Theme()
	row := ui.Column(c).Padding(10, 12).Gap(4).Radius(12)
	if selected {
		row.Background(ui.RGBA(167, 139, 250, 0.16)).Border(1, ui.RGBA(167, 139, 250, 0.35))
	} else {
		row.Background(t.Surface)
		if row.Hovered() {
			row.Background(t.SurfaceHover)
		}
	}
	muted := a.read[it.ID]
	titleColor := t.Text
	if muted {
		titleColor = t.TextMuted
	}
	row.Children(func() {
		ui.Text(c, it.DisplayTitle()).Bold().FontSize(13).TextColor(titleColor)
		meta := it.Source
		if it.Priority == "P0" {
			meta = "P0 · " + meta
		}
		meta += " · " + fmtAgo(it.Published)
		ui.Text(c, meta).FontSize(11).TextColor(t.TextMuted)
		sum := it.DisplaySummary()
		if sum != "" {
			ui.Text(c, clampRunes(sum, 90)).FontSize(12).TextColor(t.TextMuted)
		}
	})
}

func (a *radarApp) detailPane(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(4, 8, 4, 16).Gap(10).Children(func() {
		if a.detail == nil {
			ui.Column(c).Grow(1).Center().Gap(8).Children(func() {
				ui.Text(c, "选择一条资讯").TextColor(t.TextMuted)
				ui.Text(c, "右侧将显示摘要；双击或 Enter 可在浏览器打开原文。").FontSize(12).TextColor(t.TextMuted)
			})
			return
		}
		it := a.detail
		card := ui.Column(c).Fill().Padding(16).Gap(12).Radius(18)
		card.Material(glass.Glass{Style: glass.Regular})
		card.Children(func() {
			ui.Text(c, it.DisplayTitle()).Bold().FontSize(18)
			ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
				ui.Badge(c, it.Source)
				if it.Priority != "" {
					ui.Badge(c, it.Priority)
				}
				ui.Text(c, fmtTime(it.Published)+" · "+fmtAgo(it.Published)).FontSize(11).TextColor(t.TextMuted)
			})
			sum := it.DisplaySummary()
			if sum == "" {
				sum = "（无摘要）"
			}
			ui.Scroll(c).Grow(1).Children(func() {
				ui.Text(c, sum).FontSize(14)
			})
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, "在浏览器打开").Clicked() {
					a.openURL(it.URL)
				}
				if ui.Button(c, "复制链接").Clicked() {
					c.Services().WriteClipboard(it.URL)
				}
				if it.Homepage != "" {
					if ui.Button(c, "来源主页").Clicked() {
						a.openURL(it.Homepage)
					}
				}
			})
			ui.Text(c, "演示说明：Telegram / 简报 / 全文翻译等高级能力未接入此原生壳。").FontSize(11).TextColor(t.TextMuted)
		})
	})
}

func (a *radarApp) settingsPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(24).Gap(14).Children(func() {
		ui.Text(c, "设置").Bold().FontSize(22)
		ui.Text(c, "实验预览仅提供只读状态；完整设置请用正式桌面版。").TextColor(t.TextMuted)

		card := ui.Column(c).Padding(16).Gap(10).Radius(16).Material(glass.Glass{})
		card.Children(func() {
			ui.Text(c, "服务").Bold()
			ui.Text(c, fmt.Sprintf("内置 API：%s", a.base())).FontSize(12).TextColor(t.TextMuted)
			ui.Text(c, fmt.Sprintf("健康：%v", a.healthy)).FontSize(12)
			ui.Text(c, fmt.Sprintf("上次刷新：%s", a.lastRefresh.In(time.FixedZone("CST", 8*3600)).Format("15:04:05"))).FontSize(12)
			if ui.Button(c, "打开数据目录").Clicked() {
				if a.srv != nil {
					// data dir is parent of news.db — use App path again via env already set
					go func() {
						if p, err := mygo.App.Path(mygo.PathUserData); err == nil {
							_ = mygo.Shell.OpenPath(p)
						}
					}()
				}
			}
		})

		ui.Column(c).Padding(16).Gap(8).Radius(16).Background(t.Surface).Children(func() {
			ui.Text(c, "未接入（占位）").Bold()
			ui.Text(c, "· DeepSeek 翻译密钥\n· 系统通知分区偏好\n· Telegram 推送\n· 自动更新通道（本实验包使用 desktop-native-v*，不影响正式 desktop-v*）").FontSize(12).TextColor(t.TextMuted)
		})
	})
}

func (a *radarApp) aboutPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(24).Gap(12).Children(func() {
		ui.Text(c, "关于").Bold().FontSize(22)
		card := ui.Column(c).Padding(20).Gap(10).Radius(18).Material(glass.Glass{Tint: ui.RGBA(167, 139, 250, 0.25)})
		card.Children(func() {
			ui.Text(c, "AI News Radar · Native Preview").Bold().FontSize(16)
			ui.Text(c, "Version "+a.version+"（desktop-native）").FontSize(13).TextColor(t.TextMuted)
			ui.Text(c, "© 2026 Lang").FontSize(13)
			ui.Text(c, "纯 MyGo 原生 UI 实验分支，用于预览观感与性能。正式功能仍以 WebView 桌面版为准。").FontSize(13)
			if ui.Link(c, "MyGo UI 文档", "https://mygo.egoist.dev/docs/ui").Clicked() {
				a.openURL("https://mygo.egoist.dev/docs/ui")
			}
			if ui.Link(c, "GitHub: iferrnil/ai-news-radar", "https://github.com/iferrnil/ai-news-radar").Clicked() {
				a.openURL("https://github.com/iferrnil/ai-news-radar")
			}
		})
	})
}

func (a *radarApp) stubPage(c *ui.Context, title, body string) {
	t := c.Theme()
	ui.Column(c).Fill().Center().Gap(12).Padding(24).Children(func() {
		ui.Text(c, title).Bold().FontSize(20)
		ui.Text(c, body).FontSize(13).TextColor(t.TextMuted)
		ui.Text(c, "实验预览").FontSize(12).TextColor(t.Accent)
	})
}

func sectionTitle(nav string, sections []sectionInfo) string {
	if nav == navAll {
		return "全部资讯"
	}
	for _, s := range sections {
		if s.Key == nav {
			return s.Name
		}
	}
	return nav
}

func clampRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n]) + "…"
}

func max32(a, b float32) float32 {
	if a > b {
		return a
	}
	return b
}
