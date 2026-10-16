package app

import (
	"fmt"
	"strings"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/iferrnil/ai-news-radar/desktop-native/core"
)

// healthPage mirrors the web 运行状态 page: per-source last fetch, error, cool-down, success rate, item counts.
func (a *App) healthPage(c *ui.Context) {
	t := c.Theme()
	if a.health == nil {
		a.health, _ = a.st.Health()
	}
	var ok, failing, cooling, disabled, items int
	for _, h := range a.health {
		items += h.Count
		switch {
		case !h.Enabled || !h.Supported:
			disabled++
		case h.OpenUntil > nowF():
			cooling++
		case h.LastError != "":
			failing++
		default:
			ok++
		}
	}
	ui.Row(c).Padding(10, 16).Gap(10).AlignItems(ui.Center).Children(func() {
		ui.Text(c, "运行状态").FontSize(18).Bold()
		ui.Spacer(c)
		if ui.Button(c, "刷新").Clicked() {
			a.health, _ = a.st.Health()
		}
	})
	ui.Divider(c)
	ui.Row(c).Padding(10, 16).Gap(10).Wrap().Children(func() {
		stat := func(label string, n int, col ui.Color) {
			ui.Column(c).Padding(10, 14).Radius(8).Border(1, t.Border).Gap(2).MinWidth(110).Children(func() {
				ui.Textf(c, "%d", n).FontSize(20).Bold().TextColor(col)
				ui.Text(c, label).FontSize(12).TextColor(t.TextMuted)
			})
		}
		stat("正常", ok, t.Text)
		stat("出错", failing, t.Danger)
		stat("冷却中（熔断）", cooling, t.Danger)
		stat("停用 / 不支持", disabled, t.TextMuted)
		stat("保留条目（48 小时）", items, t.Text)
	})
	ui.Row(c).Padding(4, 16).Gap(8).Children(func() {
		for _, h := range []struct {
			s string
			w float32
		}{{"数据源", 0}, {"上次抓取", 110}, {"成功率", 70}, {"耗时", 70}, {"条目", 70}, {"状态", 260}} {
			e := ui.Text(c, h.s).FontSize(12).Bold().TextColor(t.TextMuted)
			if h.w == 0 {
				e.Grow(1).MinWidth(0)
			} else {
				e.Width(h.w)
			}
		}
	})
	ui.List(c, nil, len(a.health), func(i int) {
		h := a.health[i]
		ui.Row(c.Key(h.ID)).Padding(6, 16).Gap(8).AlignItems(ui.Center).Children(func() {
			name := ui.Text(c, h.Name).NoWrap().Ellipsis("…").Grow(1).MinWidth(0)
			if !h.Enabled {
				name.TextColor(t.TextMuted)
			}
			last := "—"
			if h.LastFetched > 0 {
				last = ago(h.LastFetched)
			}
			ui.Text(c, last).FontSize(12).Width(110).NoWrap()
			rate := "—"
			if r := h.SuccessRate(); r >= 0 {
				rate = fmt.Sprintf("%.0f%%", r*100)
			}
			ui.Text(c, rate).FontSize(12).Width(70).Tooltip("最近 " + fmt.Sprint(len(h.Runs)) + " 次：" + runsBar(h.Runs))
			lat := "—"
			if h.LatMs > 0 {
				lat = fmt.Sprintf("%d ms", h.LatMs)
			}
			ui.Text(c, lat).FontSize(12).Width(70)
			cnt := fmt.Sprint(h.Count)
			if h.Hidden > 0 {
				cnt += fmt.Sprintf("（滤 %d）", h.Hidden)
			}
			ui.Text(c, cnt).FontSize(12).Width(70).NoWrap()
			st, col := healthStatus(h)
			if col {
				ui.Text(c, st).FontSize(12).Width(260).NoWrap().Ellipsis("…").TextColor(t.Danger).Tooltip(h.LastError)
			} else {
				ui.Text(c, st).FontSize(12).Width(260).NoWrap().Ellipsis("…").TextColor(t.TextMuted)
			}
		})
	}).Grow(1)
}

func nowF() float64 { return float64(time.Now().UnixMilli()) / 1000 }

func runsBar(r string) string {
	return strings.NewReplacer("1", "●", "0", "○").Replace(r)
}

func healthStatus(h core.Health) (string, bool) {
	switch {
	case !h.Supported:
		return "不支持：" + h.UnsupportedReason, false
	case !h.Enabled:
		return "已停用", false
	case h.OpenUntil > nowF():
		return fmt.Sprintf("冷却中，%s 后重试（连续失败 %d 次）：%s", time.Until(time.Unix(int64(h.OpenUntil), 0)).Round(time.Minute), h.Consec, h.LastError), true
	case h.LastError != "":
		return fmt.Sprintf("出错（连续 %d 次）：%s", h.Consec, h.LastError), true
	case h.LastFetched == 0:
		return "等待首次抓取", false
	}
	return "正常", false
}
