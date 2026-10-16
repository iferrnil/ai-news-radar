package app

import (
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/iferrnil/ai-news-radar/desktop-native/core"
)

// show opens the detail drawer for an item and marks it read.
func (a *App) show(it *core.Item) {
	a.detail = it
	if !it.Read {
		a.st.SetRead([]int64{it.ID}, true)
		it.Read = true
		a.unread = a.st.UnreadCounts()
	}
}

// drawer shows the full item as plain text: original and translation, source, time, links.
func (a *App) drawer(c *ui.Context) {
	it := a.detail
	t := c.Theme()
	if c.Shortcut(0, ui.KeyEscape) {
		a.detail = nil
		return
	}
	ui.Column(c).Width(400).FillHeight().Children(func() {
		ui.Row(c).Padding(10, 14).Gap(8).AlignItems(ui.Center).Children(func() {
			ui.Text(c, "详情").Bold().Grow(1)
			if ui.Button(c, "×").Label("关闭详情").Tooltip("关闭（Esc）").Clicked() {
				a.detail = nil
			}
		})
		ui.Divider(c)
		if a.detail == nil {
			return
		}
		ui.Scroll(c).Grow(1).Children(func() {
			ui.Column(c).Padding(14).Gap(10).Children(func() {
				ui.Row(c).Gap(6).Wrap().AlignItems(ui.Center).Children(func() {
					if it.Priority == "P0" {
						ui.Badge(c, "P0")
					}
					ui.Text(c, it.Source).FontSize(12).TextColor(t.TextMuted)
					ui.Text(c, "· "+a.sectionName(it.Section)).FontSize(12).TextColor(t.TextMuted)
				})
				ui.Text(c, time.Unix(int64(it.Published), 0).Format("2006-01-02 15:04")+"（"+ago(it.Published)+"）").FontSize(12).TextColor(t.TextMuted)
				if it.Author != "" {
					ui.Text(c, "作者："+it.Author).FontSize(12).TextColor(t.TextMuted)
				}
				if it.Image != "" {
					if bmp := a.thumb(it.Image, refererFor(it)); bmp != nil {
						ui.Image(c, bmp).FillWidth().AspectRatio(16.0 / 9).Fit(ui.Cover).Radius(8)
					}
				}
				if it.TitleZh != "" {
					ui.Text(c, it.TitleZh).FontSize(17).Bold().Selectable()
					if it.SummaryZh != "" {
						ui.Text(c, it.SummaryZh).FontSize(14).LineHeight(1.5).Selectable()
					}
					ui.Divider(c)
					ui.Text(c, "原文").FontSize(12).TextColor(t.TextMuted)
				}
				ui.Text(c, it.Title).FontSize(16).Bold().Selectable()
				if it.Summary != "" && it.Summary != it.Title {
					ui.Text(c, it.Summary).FontSize(14).LineHeight(1.5).TextColor(t.Text).Selectable()
				}
				ui.Row(c).Gap(8).Wrap().Children(func() {
					if ui.PrimaryButton(c, "在浏览器中打开").Clicked() {
						a.host.OpenURL(it.URL)
					}
					if ui.Button(c, "标记为未读").Clicked() {
						a.st.SetRead([]int64{it.ID}, false)
						it.Read = false
						a.unread = a.st.UnreadCounts()
					}
					if ui.Button(c, "复制链接").Clicked() {
						c.Services().WriteClipboard(it.URL)
						c.Toast("已复制链接")
					}
				})
				if len(it.AlsoOnX) > 0 {
					ui.Text(c, "X 上也在讨论").FontSize(12).Bold()
					for i, x := range it.AlsoOnX {
						label := x.URL
						if x.Author != nil {
							label = "@" + *x.Author
						}
						if ui.Button(c.Key(i), label).Clicked() {
							a.host.OpenURL(x.URL)
						}
					}
				}
			})
		})
	})
}
