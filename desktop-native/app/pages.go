package app

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/iferrnil/ai-news-radar/desktop-native/core"
)

var typeZh = map[string]string{"rss": "RSS", "api": "API", "x": "X 账号", "x_list": "X 列表", "html": "HTML"}

// ---------------------------------------------------------------- 数据源管理

func (a *App) sourcesPage(c *ui.Context) {
	t := c.Theme()
	ui.Row(c).Padding(10, 16).Gap(8).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "数据源管理").FontSize(18).Bold().NoWrap()
		ui.Textf(c, "%d 个", len(a.sources)).TextColor(t.TextMuted)
		ui.Spacer(c)
		if ui.PrimaryButton(c, "添加数据源").Clicked() {
			a.dlg.add, a.dlg.addInput, a.dlg.detected, a.dlg.addErr = true, "", nil, ""
			a.dlg.addPriority = "P1"
			if len(a.sections) > 0 {
				a.dlg.addSection = a.sections[0].Name
			}
		}
		if ui.Button(c, "新建分区").Clicked() {
			a.dlg.newSec, a.dlg.secName, a.dlg.secIcon, a.dlg.secInAll = true, "", "", true
		}
		if ui.Button(c, "导入").Tooltip("导入本应用导出的 JSON 或任意阅读器的 OPML").Clicked() {
			a.startImport(c)
		}
		ui.MenuButton(c, "导出", func(m *ui.Menu) {
			if m.Item("导出 JSON（v2）").Chosen() {
				a.export(c, "json")
			}
			if m.Item("导出 OPML").Chosen() {
				a.export(c, "opml")
			}
		})
	})
	ui.Divider(c)
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(12, 16).Gap(18).Children(func() {
			for i, s := range a.sections {
				a.sectionBlock(c, i, s)
			}
		})
	})
}

func (a *App) sectionBlock(c *ui.Context, idx int, s core.Section) {
	t := c.Theme()
	ui.Column(c.Key(s.Key)).Gap(6).Children(func() {
		ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, iconText(s.Icon)+s.Name).FontSize(15).Bold()
			if !s.InAll {
				ui.Badge(c, "不计入全部")
			}
			ui.Spacer(c)
			ui.MenuButton(c, "分区", func(m *ui.Menu) {
				if m.Item("编辑分区…").Chosen() {
					a.dlg.editSec, a.dlg.secTarget = true, s
					a.dlg.secName, a.dlg.secIcon, a.dlg.secInAll = s.Name, s.Icon, s.InAll
				}
				if m.Item("上移").Disabled(idx == 0).Chosen() {
					a.st.MoveSection(s.Key, -1)
					a.Reload()
				}
				if m.Item("下移").Disabled(idx == len(a.sections)-1).Chosen() {
					a.st.MoveSection(s.Key, 1)
					a.Reload()
				}
				m.Separator()
				if m.Item("删除分区…").Disabled(len(a.sections) < 2).Chosen() {
					a.dlg.delSec, a.dlg.secTarget = true, s
					a.dlg.moveTo = ""
					for _, o := range a.sections {
						if o.Key != s.Key {
							a.dlg.moveTo = o.Name
							break
						}
					}
				}
			})
		})
		n := 0
		for _, x := range a.sources {
			if x.Section != s.Key {
				continue
			}
			n++
			a.sourceRow(c, x)
		}
		if n == 0 {
			ui.Text(c, "（空）").TextColor(t.TextMuted).FontSize(12)
		}
	})
}

func (a *App) sourceRow(c *ui.Context, x core.Source) {
	t := c.Theme()
	ui.Row(c.Key(x.ID)).Gap(10).Padding(8, 10).Radius(8).Border(1, t.Border).AlignItems(ui.Center).Children(func() {
		on := x.Enabled
		if ui.Switch(c, &on).Label("启用 " + x.Name).Disabled(!x.Supported).Changed() {
			a.st.SetEnabled(x.ID, on)
			a.Reload()
		}
		ui.Column(c).Grow(1).MinWidth(0).Gap(2).Children(func() {
			ui.Row(c).Gap(6).AlignItems(ui.Center).Children(func() {
				ui.Text(c, x.Name).Bold().NoWrap().Ellipsis("…").Shrink(1)
				ui.Badge(c, typeZh[x.Type])
				if x.Priority != "" {
					ui.Badge(c, x.Priority)
				}
				if x.Rule.Has() {
					ui.Badge(c, "过滤").Tooltip(ruleSummary(x.Rule))
				}
			})
			status := fmt.Sprintf("%d 条 · 每 %s", x.Count, every(x.PollInterval))
			if x.LastFetched > 0 {
				status += " · 上次 " + ago(x.LastFetched)
			}
			col := t.TextMuted
			switch {
			case !x.Supported:
				status, col = "不支持："+x.UnsupportedReason, t.TextMuted
			case x.LastError != "":
				status, col = "错误："+x.LastError, t.Danger
			}
			ui.Text(c, status).FontSize(12).TextColor(col).NoWrap().Ellipsis("…")
		})
		ui.MenuButton(c, "操作", func(m *ui.Menu) {
			if m.Item("立即抓取").Disabled(!x.Supported || a.sched == nil).Chosen() {
				go a.sched.PollOne(context.Background(), x)
				c.Toast("正在抓取 " + x.Name)
			}
			if m.Item("重命名…").Chosen() {
				a.dlg.rename, a.dlg.target, a.dlg.renameTo = true, x, x.Name
			}
			if m.Item("过滤规则…").Chosen() {
				a.dlg.filter, a.dlg.target, a.dlg.rule, a.dlg.ruleErr = true, x, x.Rule, ""
				a.dlg.ruleAll = x.Rule.Scope == "all"
			}
			m.Submenu("移动到分区", func(sm *ui.Menu) {
				for _, s := range a.sections {
					if sm.Item(s.Name).Checked(s.Key == x.Section).Chosen() {
						a.st.SetSection(x.ID, s.Key)
						a.Reload()
					}
				}
			})
			m.Submenu("优先级", func(sm *ui.Menu) {
				for _, p := range []string{"P0", "P1", "P2"} {
					if sm.Item(p).Checked(p == x.Priority).Chosen() {
						a.st.SetPriority(x.ID, p)
						a.Reload()
					}
				}
			})
			if x.Homepage != "" && m.Item("打开主页").Chosen() {
				a.host.OpenURL(x.Homepage)
			}
			m.Separator()
			if m.Item("删除…").Chosen() {
				a.dlg.delSrc, a.dlg.target = true, x
			}
		})
	})
}

func every(sec int) string {
	if sec%3600 == 0 {
		return fmt.Sprintf("%d 小时", sec/3600)
	}
	return fmt.Sprintf("%d 分钟", sec/60)
}

func ruleSummary(r core.Rule) string {
	var p []string
	if r.Include != "" {
		p = append(p, "包含 /"+r.Include+"/")
	}
	if r.Exclude != "" {
		p = append(p, "排除 /"+r.Exclude+"/")
	}
	return strings.Join(p, "；")
}

func (a *App) export(c *ui.Context, ext string) {
	var data []byte
	var err error
	if ext == "json" {
		data, err = a.st.ExportJSON(time.Now())
	} else {
		data, err = a.st.ExportOPML(time.Now())
	}
	if err == nil {
		err = a.host.SaveFile(core.ExportFileName(ext, time.Now()), data)
	}
	if err != nil {
		c.Toast("导出失败：" + err.Error())
	} else {
		c.Toast("已导出")
	}
}

func (a *App) startImport(c *ui.Context) {
	name, data, err := a.host.OpenFile()
	if err != nil || name == "" {
		if err != nil {
			c.Toast("读取失败：" + err.Error())
		}
		return
	}
	a.dlg.importing, a.dlg.impName, a.dlg.impErr, a.dlg.impPlan, a.dlg.impMode = true, name, "", nil, 0
	_, rows, err := a.st.ParseImport(data, name, "media")
	if err != nil {
		a.dlg.impErr = err.Error()
		return
	}
	a.dlg.impRows = rows
	a.planImport()
}

var impModes = []string{"merge", "update", "replace"}

func (a *App) planImport() {
	p, err := a.st.PlanImport(a.dlg.impRows, impModes[a.dlg.impMode])
	if err != nil {
		a.dlg.impErr = err.Error()
		return
	}
	a.dlg.impPlan = p
}

// ---------------------------------------------------------------- 设置

func (a *App) loadSettings() {
	f := &a.set
	f.loaded = true
	p := a.st.NotifyPrefs()
	f.notify = p.Enabled
	v, _ := a.st.GetSetting("translate")
	f.translate = v == "1"
	sec := a.st.Secrets()
	f.keySet, f.key, f.base, f.mdl = sec.DeepseekKey != "", "", sec.DeepseekBaseURL, sec.DeepseekModel
}

func (a *App) settingsPage(c *ui.Context) {
	if !a.set.loaded {
		a.loadSettings()
	}
	t := c.Theme()
	f := &a.set
	ui.Row(c).Padding(10, 16).Children(func() { ui.Text(c, "设置").FontSize(18).Bold() })
	ui.Divider(c)
	ui.Scroll(c).Grow(1).Children(func() {
		ui.Column(c).Padding(16).Gap(20).MaxWidth(640).Children(func() {
			ui.Text(c, "通知").Bold()
			ui.Form(c, func() {
				ui.Field(c, "P0 快讯通知", func() {
					if ui.Switch(c, &f.notify).Changed() {
						a.st.SetNotifyPrefs(core.NotifyPrefs{Enabled: f.notify, Sections: a.st.NotifyPrefs().Sections})
					}
				})
			})
			ui.Text(c, "P0 来源的新条目会合并成系统通知（启动时的积压不通知，至少间隔 1 分钟）。").FontSize(12).TextColor(t.TextMuted)
			if f.notify {
				ui.Text(c, "通知哪些分区（都不选 = 全部分区）").FontSize(13)
				prefs := a.st.NotifyPrefs()
				ui.Row(c).Gap(14).Wrap().Children(func() {
					for _, s := range a.sections {
						on := false
						for _, k := range prefs.Sections {
							on = on || k == s.Key
						}
						if ui.Checkbox(c.Key("n:"+s.Key), &on, s.Name).Changed() {
							a.toggleNotifySection(s.Key, on)
						}
					}
				})
			}

			ui.Text(c, "翻译（DeepSeek）").Bold()
			ui.Form(c, func() {
				ui.Field(c, "启用翻译", func() {
					if ui.Switch(c, &f.translate).Disabled(!f.keySet).Changed() {
						v := "0"
						if f.translate {
							v = "1"
						}
						a.st.SetSetting("translate", v)
						if f.translate {
							go a.st.BackfillTranslations()
						}
						a.Reload()
					}
				})
				ph := "未配置"
				if f.keySet {
					ph = "已配置（留空则保持不变）"
				}
				ui.Field(c, "API key", func() { ui.TextInput(c, &f.key).Password().Placeholder(ph).Width(320) })
				ui.Field(c, "Base URL", func() { ui.TextInput(c, &f.base).Placeholder("https://api.deepseek.com").Width(320) })
				ui.Field(c, "模型", func() { ui.TextInput(c, &f.mdl).Placeholder("deepseek-chat").Width(320) })
			})
			ui.Row(c).Gap(8).Children(func() {
				if ui.PrimaryButton(c, "保存").Clicked() {
					if err := a.st.SaveSecrets(f.key, false, f.base, f.mdl); err != nil {
						f.msg = err.Error()
					} else {
						f.msg = "已保存"
						a.loadSettings()
					}
				}
				if ui.Button(c, "测试连接").Disabled(!f.keySet).Clicked() {
					f.msg = "正在测试…"
					go func() {
						ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
						defer cancel()
						err := a.st.TestDeepseek(ctx)
						a.host.Post(func() {
							if err != nil {
								f.msg = err.Error()
							} else {
								f.msg = "连接正常"
							}
						})
					}()
				}
				if ui.Button(c, "清除 key").Disabled(!f.keySet).Clicked() {
					a.st.SaveSecrets("", true, f.base, f.mdl)
					a.st.SetSetting("translate", "0")
					a.loadSettings()
					f.msg = "已清除"
				}
			})
			if f.msg != "" {
				ui.Text(c, f.msg).FontSize(12).TextColor(t.TextMuted)
			}
			done, queued, lastErr := a.st.TranslateStatus()
			st := fmt.Sprintf("默认关闭。key 只保存在本机数据目录的 secrets.json（权限 0600）。已翻译 %d 条，排队 %d 条。", done, queued)
			if lastErr != "" {
				st += " 最近错误：" + lastErr
			}
			ui.Text(c, st).FontSize(12).TextColor(t.TextMuted)

			ui.Text(c, "显示").Bold()
			ui.Form(c, func() {
				ui.Field(c, "侧栏显示数据源", func() {
					if ui.Switch(c, &a.showSrcs).Changed() {
						v := "0"
						if a.showSrcs {
							v = "1"
						}
						a.st.SetSetting("desktop_show_sources", v)
					}
				})
			})

			ui.Text(c, "更新").Bold()
			if u := a.host.Updates(); u != nil {
				auto := u.Auto()
				ui.Form(c, func() {
					ui.Field(c, "自动检查更新", func() {
						if ui.Switch(c, &auto).Changed() {
							u.SetAuto(auto)
						}
					})
				})
				ui.Row(c).Gap(8).AlignItems(ui.Center).Children(func() {
					ui.Text(c, "当前版本 "+u.Version()).FontSize(12).TextColor(t.TextMuted)
					if ui.Button(c, "检查更新…").Clicked() {
						u.Check()
					}
					if v, link := u.Fallback(); v != "" {
						if ui.PrimaryButton(c, "下载新版本 "+v).Clicked() {
							a.host.OpenURL(link)
						}
					}
				})
				ui.Text(c, "更新包经 ed25519 签名校验后安装；若应用内更新不可用，会提示前往 GitHub Releases 下载。").FontSize(12).TextColor(t.TextMuted)
			} else {
				ui.Text(c, "开发版本，未启用自动更新。").FontSize(12).TextColor(t.TextMuted)
			}

			ui.Text(c, "数据").Bold()
			ui.Text(c, "数据目录："+a.st.Dir).FontSize(12).TextColor(t.TextMuted).Selectable()
			ui.Textf(c, "只保留最近 %d 小时的新闻；数据库与网页版格式相同。", core.RetentionHours).FontSize(12).TextColor(t.TextMuted)
			if a.st.Thumbs != nil {
				ui.Textf(c, "缩略图缓存：%.1f MB / 上限 %d MB（随 48 小时保留期清理）", float64(a.st.Thumbs.Size())/1e6, a.st.Thumbs.MaxBytes>>20).FontSize(12).TextColor(t.TextMuted)
			}
			if v, ok := a.st.GetSetting("migrated_from"); ok {
				ui.Text(c, "已从旧版导入："+v).FontSize(12).TextColor(t.TextMuted)
			}
		})
	})
}

// toggleNotifySection adds/removes a section from the P0 notification filter (empty = all sections).
func (a *App) toggleNotifySection(key string, on bool) {
	prefs := a.st.NotifyPrefs()
	var next []string
	for _, k := range prefs.Sections {
		if k != key {
			next = append(next, k)
		}
	}
	if on {
		next = append(next, key)
	}
	a.st.SetNotifyPrefs(core.NotifyPrefs{Enabled: prefs.Enabled, Sections: next})
}
