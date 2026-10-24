package main

// Pure MyGo native UI shell (no webview chrome). Premium dark news-reader chrome
// wired to radar-server APIs. See https://mygo.egoist.dev/docs/ui

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

// Accent tokens — restrained violet (not neon).
var (
	accentSoft   = ui.Oklch(0.72, 0.12, 295) // hover / soft fills
	accentMid    = ui.Oklch(0.64, 0.13, 295) // primary accent
	accentDeep   = ui.Oklch(0.55, 0.12, 295) // pressed
	accentWash   = ui.RGBA(139, 120, 210, 0.14)
	accentWashHi = ui.RGBA(139, 120, 210, 0.22)
	accentLine   = ui.RGBA(167, 139, 250, 0.45)
	hairline     = ui.RGBA(255, 255, 255, 0.06)
	hairlineSoft = ui.RGBA(255, 255, 255, 0.04)
	shadowDeep   = ui.RGBA(0, 0, 0, 0.35)
	unreadDot    = ui.Oklch(0.68, 0.14, 295)
	p0Tint       = ui.RGBA(245, 158, 11, 0.16)
	p0Text       = ui.Oklch(0.82, 0.12, 75)
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
		split:        0.38,
		read:         map[int64]bool{},
		banner:       "实验预览 · 纯原生 UI",
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
	go func() {
		it, err := fetchItem(a.base(), id)
		if err != nil || a.win == nil {
			return
		}
		a.win.Update(func() {
			a.nav = navAll
			a.detail = it
			a.read[id] = true
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

// applyTheme — premium dark palette, restrained violet, roomy type metrics.
func applyTheme(c *ui.Context) {
	t := *ui.DarkTheme()
	t.Background = ui.Hex("#0a0a0f")
	t.Surface = ui.Hex("#12121a")
	t.SurfaceHover = ui.Hex("#1a1a24")
	t.SurfacePressed = ui.Hex("#22222e")
	t.Border = ui.Hex("#262633")
	t.Text = ui.Hex("#f2f2f7")      // high contrast for Chinese titles
	t.TextMuted = ui.Hex("#9b9baf") // readable secondary
	t.Accent = accentMid
	t.AccentHover = accentSoft
	t.AccentPressed = accentDeep
	t.AccentText = ui.Hex("#ffffff")
	t.Focus = ui.RGBA(167, 139, 250, 0.42)
	t.Selection = ui.RGBA(139, 120, 210, 0.32)
	t.Inverse = ui.Hex("#2a2a36")
	t.InverseText = ui.Hex("#f2f2f7")
	t.Scrollbar = ui.RGBA(255, 255, 255, 0.22)
	t.Radius = 8
	t.Spacing = 5 // roomier chrome
	t.FontSize = 13
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
			// Main content surface — slightly elevated vs sidebar vibrancy
			main := ui.Column(c).Grow(1)
			if !c.Vibrancy() {
				main.Background(t.Background)
			} else {
				// Soft opaque wash so feed/detail read as distinct panes over vibrancy
				main.Background(ui.RGBA(10, 10, 15, 0.72))
			}
			main.Children(func() {
				switch a.nav {
				case navSettings:
					a.settingsPage(c)
				case navAbout:
					a.aboutPage(c)
				case navStubBrief:
					a.stubPage(c, "简报", "文章生成与 Telegram 简报尚未接入此实验壳。\n请使用正式桌面版（WebView）或网页端。")
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
	if h < 48 {
		h = 48
	}
	padL := max32(bar.Left, 12) + 10
	padR := max32(bar.Right, 12) + 10
	row := ui.Row(c).Height(h).Padding(0, padR, 0, padL).
		AlignItems(ui.Center).Gap(t.Space(2)).DragWindow()
	if c.Vibrancy() {
		row.Material(glass.Glass{Style: glass.Regular})
	} else {
		row.Background(t.Surface).BorderWidth(0, 0, 1, 0).BorderColor(hairline)
	}
	row.Children(func() {
		ui.Text(c, "AI News Radar").Bold().FontSize(13).FontWeight(600)
		pill(c, "Native", accentWash, accentMid)
		ui.Box(c).Grow(1)
		if a.loading {
			ui.Spinner(c).Label("刷新中")
		}
		// Live status chip
		chip := ui.Row(c).Padding(4, 10).Gap(6).Radius(999).
			Background(ui.RGBA(255, 255, 255, 0.05)).
			AlignItems(ui.Center).
			InsetShadow(0, 0, 0, 1, hairlineSoft)
		chip.Children(func() {
			dot := t.Success
			if !a.healthy {
				dot = t.Danger
			}
			ui.Box(c).Size(7, 7).Radius(4).Background(dot).
				Shadow(0, 0, 6, 0, dot.Alpha(0.45))
			ui.Text(c, a.status).FontSize(11).TextColor(t.TextMuted)
		})
	})
}

func (a *radarApp) sidebar(c *ui.Context) {
	t := c.Theme()
	side := ui.Column(c).Width(236).Padding(10, 10, 14, 10).Gap(t.Space(1))
	if !c.Vibrancy() {
		side.Background(t.Surface).BorderWidth(0, 1, 0, 0).BorderColor(hairline)
	}
	side.Children(func() {
		// Compact experiment note — soft pill, not a gray banner strip
		note := ui.Row(c).Padding(6, 10).Radius(10).
			Background(accentWash).AlignItems(ui.Center)
		note.Children(func() {
			ui.Text(c, a.banner).FontSize(11).TextColor(accentSoft)
		})

		nav := a.nav
		sb := ui.Sidebar(c, &nav, func() {
			ui.SidebarSection(c, "资讯", &a.sectionsOpen, func() {
				ui.SidebarItem(c, navAll, nil, "全部")
				for _, s := range a.sections {
					ui.SidebarItem(c, s.Key, nil, s.Name)
				}
			})
			ui.SidebarSection(c, "更多", &a.toolsOpen, func() {
				ui.SidebarItem(c, navStubBrief, nil, "简报")
				ui.SidebarItem(c, navSettings, nil, "设置")
				ui.SidebarItem(c, navAbout, nil, "关于")
			})
		}).Width(216).Grow(1)
		if sb.Changed() {
			a.nav = nav
			a.selected = -1
			a.detail = nil
			a.refilter()
		}

		ui.Row(c).Gap(t.Space(1)).Padding(4, 2, 0, 2).Children(func() {
			if ui.Button(c, "刷新").Grow(1).Clicked() {
				a.refreshAsync()
			}
		})
	})
}

func (a *radarApp) feedPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Children(func() {
		a.appToolbar(c)

		body := ui.Column(c).Grow(1).Padding(0, 14, 14, 14).Gap(t.Space(2))
		body.Children(func() {
			if a.errMsg != "" {
				errCard := ui.Row(c).Padding(12, 14).Gap(10).Radius(12).
					Background(ui.RGBA(239, 68, 68, 0.10)).
					InsetShadow(0, 0, 0, 1, ui.RGBA(239, 68, 68, 0.25)).
					AlignItems(ui.Center)
				errCard.Children(func() {
					ui.Box(c).Size(8, 8).Radius(4).Background(t.Danger)
					ui.Text(c, "无法连接内置服务："+a.errMsg).TextColor(t.Danger).FontSize(12)
				})
			}

			ui.Split(c, &a.split, func() {
				a.itemList(c)
			}, func() {
				a.detailPane(c)
			}).Grow(1)
		})
	})
}

// appToolbar — real chrome: search, count, primary actions.
func (a *radarApp) appToolbar(c *ui.Context) {
	t := c.Theme()
	bar := ui.Row(c).Padding(10, 14).Gap(t.Space(2)).AlignItems(ui.Center).
		BorderWidth(0, 0, 1, 0).BorderColor(hairlineSoft)
	if c.Vibrancy() {
		bar.Material(glass.Blur{Radius: 18})
	} else {
		bar.Background(ui.RGBA(18, 18, 26, 0.92))
	}
	bar.Children(func() {
		ui.Toolbar(c, func() {
			sf := ui.SearchField(c, &a.query).MinWidth(220).Grow(1).Label("搜索标题 / 摘要 / 来源")
			if sf.Changed() {
				a.refilter()
			}
			ui.Spacer(c)
			pill(c, fmt.Sprintf("%d", len(a.shown)), ui.RGBA(255, 255, 255, 0.06), t.TextMuted)
			if ui.Button(c, "刷新").Clicked() {
				a.refreshAsync()
			}
			if ui.PrimaryButton(c, "打开原文").Clicked() {
				if a.detail != nil {
					a.openURL(a.detail.URL)
				}
			}
		}).Label("资讯工具栏").Grow(1)
	})
}

func (a *radarApp) itemList(c *ui.Context) {
	t := c.Theme()
	pane := ui.Column(c).Fill().Gap(t.Space(1)).
		Padding(4, 4, 4, 0)
	pane.Children(func() {
		head := ui.Row(c).Padding(4, 8, 8, 8).AlignItems(ui.Center).Gap(8)
		head.Children(func() {
			ui.Text(c, sectionTitle(a.nav, a.sections)).Bold().FontSize(16).FontWeight(600)
			ui.Box(c).Grow(1)
			ui.Text(c, fmt.Sprintf("%d 条", len(a.shown))).FontSize(11).TextColor(t.TextMuted)
		})

		if len(a.shown) == 0 {
			a.emptyState(c, "暂无资讯", "点击侧栏或工具栏「刷新」，\n或稍候订阅源拉取完成。")
			return
		}

		listSurface := ui.Column(c).Grow(1).Radius(14).
			Background(ui.RGBA(255, 255, 255, 0.025)).
			InsetShadow(0, 0, 0, 1, hairlineSoft).
			Clip()
		listSurface.Children(func() {
			lst := ui.List(c, &a.list, len(a.shown), func(row int) {
				if row < 0 || row >= len(a.shown) {
					return
				}
				it := a.shown[row]
				a.itemRow(c, it, row == a.selected, row)
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
	})
}

func (a *radarApp) itemRow(c *ui.Context, it feedItem, selected bool, row int) {
	t := c.Theme()
	unread := !a.read[it.ID]

	rowEl := ui.Row(c).Padding(10, 14).Gap(10).AlignItems(ui.Start).
		BorderWidth(0, 0, 1, 0).BorderColor(hairlineSoft)
	if selected {
		rowEl.Background(accentWashHi)
	} else if rowEl.Hovered() {
		rowEl.Background(t.SurfaceHover)
	}

	rowEl.Children(func() {
		// Unread / selected indicator column
		ind := ui.Column(c).Width(10).Padding(4, 0, 0, 0).AlignItems(ui.Center)
		ind.Children(func() {
			if selected {
				ui.Box(c).Size(3, 28).Radius(2).Background(accentMid)
			} else if unread {
				ui.Box(c).Size(7, 7).Radius(4).Background(unreadDot).
					Shadow(0, 0, 8, 0, unreadDot.Alpha(0.35))
			} else {
				ui.Box(c).Size(7, 7) // spacer keeps alignment
			}
		})

		ui.Column(c).Grow(1).Gap(4).MinWidth(0).Children(func() {
			titleColor := t.Text
			weight := 600
			if !unread {
				titleColor = t.TextMuted
				weight = 500
			}
			if selected {
				titleColor = t.Text
				weight = 600
			}
			// 14pt + weight for readable Chinese titles
			ui.Text(c, it.DisplayTitle()).FontSize(14).FontWeight(weight).
				TextColor(titleColor).LineHeight(1.35)

			meta := ui.Row(c).Gap(6).AlignItems(ui.Center)
			meta.Children(func() {
				if it.Priority == "P0" {
					pill(c, "P0", p0Tint, p0Text)
				}
				ui.Text(c, it.Source).FontSize(11).TextColor(t.TextMuted)
				ui.Text(c, "·").FontSize(11).TextColor(ui.RGBA(255, 255, 255, 0.2))
				ui.Text(c, fmtAgo(it.Published)).FontSize(11).TextColor(t.TextMuted)
			})

			sum := it.DisplaySummary()
			if sum != "" {
				ui.Text(c, clampRunes(sum, 72)).FontSize(12).
					TextColor(ui.RGBA(155, 155, 175, 0.9)).LineHeight(1.4)
			}
		})
	})
}

func (a *radarApp) detailPane(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(8, 4, 4, 12).Children(func() {
		if a.detail == nil {
			a.emptyState(c, "选择一条资讯", "右侧显示摘要与操作。\n双击或 Enter 可在浏览器打开原文。")
			return
		}
		it := a.detail

		// Comfortable reading column centered in pane
		wrap := ui.Column(c).Fill().AlignItems(ui.Center)
		wrap.Children(func() {
			card := ui.Column(c).MaxWidth(640).Grow(1).
				Padding(22, 24).Gap(14).Radius(18).
				Material(glass.Glass{Style: glass.Regular}).
				Shadow(0, 12, 40, -4, shadowDeep)
			card.Children(func() {
				ui.Text(c, it.DisplayTitle()).Bold().FontSize(20).FontWeight(650).
					LineHeight(1.35)

				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					pill(c, it.Source, ui.RGBA(255, 255, 255, 0.07), t.TextMuted)
					if it.Priority != "" {
						if it.Priority == "P0" {
							pill(c, "P0", p0Tint, p0Text)
						} else {
							pill(c, it.Priority, accentWash, accentSoft)
						}
					}
					ui.Box(c).Grow(1)
					ui.Text(c, fmtTime(it.Published)+" · "+fmtAgo(it.Published)).
						FontSize(11).TextColor(t.TextMuted)
				})

				ui.Divider(c)

				sum := it.DisplaySummary()
				if sum == "" {
					sum = "（无摘要）"
				}
				ui.Scroll(c).Grow(1).Children(func() {
					ui.Column(c).Gap(12).Padding(2, 0).MaxWidth(600).Children(func() {
						ui.Text(c, sum).FontSize(15).LineHeight(1.55).TextColor(t.Text)
					})
				})

				ui.Divider(c)

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

				ui.Text(c, "演示说明：Telegram / 简报 / 全文翻译等未接入此原生壳。").
					FontSize(11).TextColor(t.TextMuted)
			})
		})
	})
}

func (a *radarApp) settingsPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(28, 32).Gap(16).AlignItems(ui.Start).Children(func() {
		ui.Column(c).MaxWidth(560).Gap(16).Children(func() {
			ui.Text(c, "设置").Bold().FontSize(24).FontWeight(650)
			ui.Text(c, "实验预览仅提供只读状态；完整设置请用正式桌面版。").
				FontSize(13).TextColor(t.TextMuted)

			card := ui.Column(c).Padding(18).Gap(12).Radius(16).
				Material(glass.Glass{Style: glass.Regular}).
				Shadow(0, 8, 28, -2, shadowDeep)
			card.Children(func() {
				ui.Text(c, "服务").Bold().FontSize(14)
				kv(c, "内置 API", a.base())
				kv(c, "健康", fmt.Sprintf("%v", a.healthy))
				last := "—"
				if !a.lastRefresh.IsZero() {
					last = a.lastRefresh.In(time.FixedZone("CST", 8*3600)).Format("15:04:05")
				}
				kv(c, "上次刷新", last)
				ui.Divider(c)
				if ui.Button(c, "打开数据目录").Clicked() {
					go func() {
						if p, err := mygo.App.Path(mygo.PathUserData); err == nil {
							_ = mygo.Shell.OpenPath(p)
						}
					}()
				}
			})

			stub := ui.Column(c).Padding(18).Gap(10).Radius(16).
				Background(ui.RGBA(255, 255, 255, 0.03)).
				InsetShadow(0, 0, 0, 1, hairline)
			stub.Children(func() {
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "未接入").Bold().FontSize(14)
					pill(c, "占位", accentWash, accentSoft)
				})
				for _, line := range []string{
					"DeepSeek 翻译密钥",
					"系统通知分区偏好",
					"Telegram 推送",
					"自动更新（本包 desktop-native-v*，不影响正式 desktop-v*）",
				} {
					ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
						ui.Box(c).Size(5, 5).Radius(3).Background(ui.RGBA(255, 255, 255, 0.2))
						ui.Text(c, line).FontSize(12).TextColor(t.TextMuted)
					})
				}
			})
		})
	})
}

func (a *radarApp) aboutPage(c *ui.Context) {
	t := c.Theme()
	ui.Column(c).Fill().Padding(28, 32).Gap(16).Children(func() {
		ui.Column(c).MaxWidth(520).Gap(14).Children(func() {
			ui.Text(c, "关于").Bold().FontSize(24).FontWeight(650)
			card := ui.Column(c).Padding(22).Gap(12).Radius(18).
				Material(glass.Glass{
					Style: glass.Regular,
					Tint:  ui.RGBA(139, 120, 210, 0.18),
				}).
				Shadow(0, 10, 32, -2, shadowDeep)
			card.Children(func() {
				ui.Text(c, "AI News Radar · Native").Bold().FontSize(17).FontWeight(600)
				ui.Text(c, "Version "+a.version+"（desktop-native）").
					FontSize(13).TextColor(t.TextMuted)
				ui.Text(c, "© 2026 Lang").FontSize(13)
				ui.Divider(c)
				ui.Text(c, "纯 MyGo 原生 UI 实验分支，用于预览观感与性能。正式功能仍以 WebView 桌面版为准。").
					FontSize(13).LineHeight(1.5)
				ui.Row(c).Gap(16).Children(func() {
					if ui.Link(c, "MyGo UI 文档", "https://mygo.egoist.dev/docs/ui").Clicked() {
						a.openURL("https://mygo.egoist.dev/docs/ui")
					}
					if ui.Link(c, "GitHub", "https://github.com/iferrnil/ai-news-radar").Clicked() {
						a.openURL("https://github.com/iferrnil/ai-news-radar")
					}
				})
			})
		})
	})
}

func (a *radarApp) stubPage(c *ui.Context, title, body string) {
	a.emptyState(c, title, body)
}

func (a *radarApp) emptyState(c *ui.Context, title, body string) {
	t := c.Theme()
	ui.Column(c).Grow(1).Center().Gap(14).Padding(32).Children(func() {
		glyph := ui.Column(c).Size(56, 56).Radius(16).
			Background(accentWash).
			InsetShadow(0, 0, 0, 1, accentLine).
			Center()
		glyph.Children(func() {
			ui.Text(c, "◎").FontSize(22).TextColor(accentSoft)
		})
		ui.Text(c, title).Bold().FontSize(17).FontWeight(600)
		ui.Text(c, body).FontSize(13).TextColor(t.TextMuted).LineHeight(1.5)
		pill(c, "实验预览", accentWash, accentSoft)
	})
}

func pill(c *ui.Context, label string, bg, fg ui.Color) {
	ui.Row(c).Padding(3, 9).Radius(999).Background(bg).
		InsetShadow(0, 0, 0, 1, hairlineSoft).Children(func() {
		ui.Text(c, label).FontSize(11).FontWeight(550).TextColor(fg)
	})
}

func kv(c *ui.Context, k, v string) {
	t := c.Theme()
	ui.Row(c).Gap(12).AlignItems(ui.Center).Children(func() {
		ui.Text(c, k).FontSize(12).TextColor(t.TextMuted).Width(72)
		ui.Text(c, v).FontSize(12).TextColor(t.Text)
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
