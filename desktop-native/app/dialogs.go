package app

import (
	"context"
	"fmt"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/iferrnil/ai-news-radar/desktop-native/core"
)

func (a *App) dialogs(c *ui.Context) {
	t := c.Theme()
	d := &a.dlg
	buttons := func(cancel *bool, ok string, disabled bool, onOK func()) {
		ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
			if ui.Button(c, "取消").Clicked() {
				*cancel = false
			}
			if ui.PrimaryButton(c, ok).Disabled(disabled).Clicked() {
				onOK()
			}
		})
	}
	errText := func(s string) {
		if s != "" {
			ui.Text(c, s).FontSize(12).TextColor(t.Danger)
		}
	}

	// 添加数据源
	ui.Modal(c, &d.add, func() {
		ui.Column(c).Gap(10).Width(460).Children(func() {
			ui.Text(c, "添加数据源").Bold().FontSize(16)
			ui.Text(c, "粘贴 RSS/Atom 地址、网站首页、X 账号（@handle）或 X 列表链接。").FontSize(12).TextColor(t.TextMuted)
			ui.Row(c).Gap(8).Children(func() {
				in := ui.TextInput(c, &d.addInput).Placeholder("https://… 或 @handle").AutoFocus().Grow(1)
				if (ui.Button(c, "检测").Disabled(d.addBusy || d.addInput == "").Clicked() || in.Submitted()) && !d.addBusy {
					a.detect()
				}
			})
			if d.addBusy {
				ui.Text(c, "正在检测…").FontSize(12).TextColor(t.TextMuted)
			}
			errText(d.addErr)
			if x := d.detected; x != nil {
				ui.Textf(c, "%s · %s · %d 条", x.Name, typeZh[x.Type], x.Count).Bold()
				for i, s := range x.Sample {
					ui.Text(c.Key(i), "• "+s.Title).FontSize(12).TextColor(t.TextMuted).MaxLines(1).Ellipsis("…")
				}
				ui.Form(c, func() {
					ui.Field(c, "名称", func() { ui.TextInput(c, &x.Name).Width(280) })
					ui.Field(c, "分区", func() { ui.Select(c, &d.addSection, a.sectionNames()) })
					ui.Field(c, "优先级", func() { ui.Select(c, &d.addPriority, []string{"P0", "P1", "P2"}) })
				})
			}
			buttons(&d.add, "添加", d.detected == nil, func() {
				id, err := a.st.AddSource(*d.detected, a.sectionKeyByName(d.addSection), d.addPriority)
				if err != nil {
					d.addErr = err.Error()
					return
				}
				d.add = false
				a.Reload()
				if a.sched != nil {
					if s, err := a.st.Source(id); err == nil {
						go a.sched.PollOne(context.Background(), s)
					}
				}
				c.Toast("已添加")
			})
		})
	})

	// 重命名
	ui.Modal(c, &d.rename, func() {
		ui.Column(c).Gap(10).Width(380).Children(func() {
			ui.Text(c, "重命名数据源").Bold().FontSize(16)
			ok := ui.TextInput(c, &d.renameTo).AutoFocus().Submitted()
			errText(d.addErr)
			do := func() {
				if err := a.st.RenameSource(d.target.ID, d.renameTo); err != nil {
					d.addErr = err.Error()
					return
				}
				d.rename, d.addErr = false, ""
				a.Reload()
			}
			if ok {
				do()
			}
			buttons(&d.rename, "保存", d.renameTo == "", do)
		})
	})

	// 过滤规则
	ui.Modal(c, &d.filter, func() {
		ui.Column(c).Gap(10).Width(480).Children(func() {
			ui.Text(c, "过滤规则 · "+d.target.Name).Bold().FontSize(16)
			ui.Text(c, "正则表达式；保留匹配「包含」且不匹配「排除」的条目。修改后会重新应用到已保存的条目。").FontSize(12).TextColor(t.TextMuted)
			ui.Form(c, func() {
				ui.Field(c, "包含", func() { ui.TextInput(c, &d.rule.Include).Placeholder("例如 GPT|Claude|Gemini").Width(300) })
				ui.Field(c, "排除", func() { ui.TextInput(c, &d.rule.Exclude).Placeholder("例如 广告|招聘").Width(300) })
				ui.Field(c, "同时匹配摘要", func() { ui.Switch(c, &d.ruleAll) })
				ui.Field(c, "区分大小写", func() { ui.Switch(c, &d.rule.CS) })
			})
			if p := a.rulePreview(); p != "" {
				ui.Text(c, p).FontSize(12).TextColor(t.TextMuted)
			}
			errText(d.ruleErr)
			buttons(&d.filter, "保存", false, func() {
				d.rule.Scope = "title"
				if d.ruleAll {
					d.rule.Scope = "all"
				}
				n, err := a.st.SetRule(d.target.ID, d.rule)
				if err != nil {
					d.ruleErr = err.Error()
					return
				}
				d.filter = false
				a.Reload()
				c.Toast(fmt.Sprintf("已保存，隐藏 %d 条", n))
			})
		})
	})

	// 新建 / 编辑分区
	secForm := func() {
		ui.Form(c, func() {
			ui.Field(c, "名称", func() { ui.TextInput(c, &d.secName).AutoFocus().Width(240) })
			ui.Field(c, "图标（emoji，可空）", func() { ui.TextInput(c, &d.secIcon).Width(80) })
			ui.Field(c, "计入「全部」", func() { ui.Switch(c, &d.secInAll) })
		})
		errText(d.addErr)
	}
	ui.Modal(c, &d.newSec, func() {
		ui.Column(c).Gap(10).Width(400).Children(func() {
			ui.Text(c, "新建分区").Bold().FontSize(16)
			secForm()
			buttons(&d.newSec, "创建", d.secName == "", func() {
				if _, err := a.st.CreateSection(d.secName, d.secIcon, d.secInAll); err != nil {
					d.addErr = err.Error()
					return
				}
				d.newSec, d.addErr = false, ""
				a.Reload()
			})
		})
	})
	ui.Modal(c, &d.editSec, func() {
		ui.Column(c).Gap(10).Width(400).Children(func() {
			ui.Text(c, "编辑分区").Bold().FontSize(16)
			secForm()
			buttons(&d.editSec, "保存", d.secName == "", func() {
				if err := a.st.UpdateSection(d.secTarget.Key, d.secName, d.secIcon, d.secInAll); err != nil {
					d.addErr = err.Error()
					return
				}
				d.editSec, d.addErr = false, ""
				a.Reload()
			})
		})
	})

	// 删除分区
	ui.Modal(c, &d.delSec, func() {
		ui.Column(c).Gap(10).Width(400).Children(func() {
			ui.Text(c, "删除分区「"+d.secTarget.Name+"」").Bold().FontSize(16)
			var others []string
			for _, s := range a.sections {
				if s.Key != d.secTarget.Key {
					others = append(others, s.Name)
				}
			}
			ui.Form(c, func() {
				ui.Field(c, "数据源移入", func() { ui.Select(c, &d.moveTo, others) })
			})
			ui.Row(c).Gap(8).Justify(ui.End).Children(func() {
				if ui.Button(c, "取消").Clicked() {
					d.delSec = false
				}
				if ui.PrimaryButton(c, "删除").Background(c.Theme().Danger).Clicked() {
					if err := a.st.DeleteSection(d.secTarget.Key, a.sectionKeyByName(d.moveTo)); err != nil {
						c.Toast(err.Error())
					}
					if a.sel == "sec:"+d.secTarget.Key {
						a.sel = "all"
					}
					d.delSec = false
					a.Reload()
				}
			})
		})
	})

	// 删除数据源
	switch ui.AlertDialog(c, &d.delSrc, "删除「"+d.target.Name+"」？", "该来源及其已保存的新闻都会被删除；内置源删除后不会被自动加回。", "取消", "删除") {
	case 1:
		a.st.DeleteSources([]int64{d.target.ID})
		if a.sel == fmt.Sprintf("src:%d", d.target.ID) {
			a.sel = "all"
		}
		a.Reload()
	}

	// 导入
	ui.Modal(c, &d.importing, func() {
		ui.Column(c).Gap(10).Width(520).Children(func() {
			ui.Text(c, "导入数据源 · "+d.impName).Bold().FontSize(16)
			errText(d.impErr)
			if d.impPlan == nil {
				buttons(&d.importing, "导入", true, func() {})
				return
			}
			if ui.Segmented(c, &d.impMode, "合并（跳过已存在）", "更新已存在", "覆盖（删除文件中没有的）").Label("导入方式").Changed() {
				a.planImport()
			}
			p := d.impPlan
			ui.Textf(c, "新增 %d · 恢复 %d · 已存在 %d · 更新 %d · 无效 %d · 删除 %d", p.New, p.Restore, p.Dup, p.Update, p.Invalid, len(p.Remove)).FontSize(12)
			ui.List(c, nil, len(p.Rows), func(i int) {
				r := p.Rows[i]
				line := map[string]string{"new": "新增", "restore": "恢复", "dup": "已存在", "update": "更新", "invalid": "无效"}[r.Status] + " · " + r.Name
				if r.Reason != "" {
					line += "（" + r.Reason + "）"
				}
				ui.Text(c, line).FontSize(12).NoWrap().Ellipsis("…").Padding(2, 0)
			}).Height(220)
			change := p.New + p.Restore + len(p.Remove)
			if d.impMode > 0 {
				change += p.Update
			}
			label := "导入"
			if d.impMode == 2 {
				label = "覆盖导入"
			}
			buttons(&d.importing, fmt.Sprintf("%s · %d", label, change), change == 0, func() {
				added, err := a.st.ApplyImport(p, impModes[d.impMode])
				if err != nil {
					d.impErr = err.Error()
					return
				}
				d.importing = false
				a.Reload()
				c.Toast(fmt.Sprintf("导入完成，新增 %d", len(added)))
			})
		})
	})
}

func (a *App) detect() {
	d := &a.dlg
	d.addBusy, d.addErr, d.detected = true, "", nil
	input := d.addInput
	go func() {
		ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
		defer cancel()
		det, err := core.Detect(ctx, input)
		a.host.Post(func() {
			d.addBusy = false
			if err != nil {
				d.addErr = err.Error()
				return
			}
			d.detected = &det
		})
	}()
}

// rulePreview applies the edited rule to the source's stored items (validation errors shown instead).
func (a *App) rulePreview() string {
	r := a.dlg.rule
	r.Scope = "title"
	if a.dlg.ruleAll {
		r.Scope = "all"
	}
	if _, err := r.Validate(); err != nil {
		return err.Error()
	}
	if !r.Has() {
		return ""
	}
	items, err := a.st.Items(core.Query{SourceID: a.dlg.target.ID, Limit: 500})
	if err != nil || len(items) == 0 {
		return ""
	}
	texts := make([]string, len(items))
	for i, it := range items {
		texts[i] = core.RuleText(r.Scope, it.Title, it.Summary)
	}
	keep := 0
	for _, k := range r.Keeps(texts) {
		if k {
			keep++
		}
	}
	return fmt.Sprintf("预览：当前可见的 %d 条中保留 %d 条", len(items), keep)
}
