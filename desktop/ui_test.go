package main

import (
	"testing"

	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

func find(items []*mygo.MenuItem, label string) *mygo.MenuItem {
	for _, it := range items {
		if it.Label == label {
			return it
		}
		if f := find(it.Submenu, label); f != nil {
			return f
		}
	}
	return nil
}

func TestAppMenuNative(t *testing.T) {
	m := appMenuNative(nil)
	if find(m, appName) == nil {
		t.Fatal("missing app menu")
	}
	if find(m, "编辑") == nil {
		t.Fatal("missing edit menu")
	}
}

func TestWindowOptionsNative(t *testing.T) {
	o := windowOptions(ui.View(func(c *ui.Context) {}))
	if o.Content == nil {
		t.Fatal("expected Content, not webview Page")
	}
	if o.StateKey != "native-main" {
		t.Fatalf("state key: %q", o.StateKey)
	}
	if o.MinWidth < 800 || o.Vibrancy != mygo.VibrancySidebar {
		t.Fatalf("%+v", o)
	}
	if o.Page.PreloadScript != "" {
		t.Fatal("native shell must not set Page preload")
	}
}

func TestRefilter(t *testing.T) {
	a := newRadarApp(nil, "0.2.0")
	a.items = []feedItem{
		{ID: 1, Title: "Hello Alpha", Section: "lab", Source: "A", Summary: "x"},
		{ID: 2, Title: "Beta News", Section: "cn", Source: "B", Summary: "y"},
		{ID: 3, TitleZh: "中文标题", Title: "EN", Section: "lab", Source: "C", SummaryZh: "摘要"},
	}
	a.nav = "lab"
	a.refilter()
	if len(a.shown) != 2 {
		t.Fatalf("lab filter: %d", len(a.shown))
	}
	a.query = "中文"
	a.refilter()
	if len(a.shown) != 1 || a.shown[0].ID != 3 {
		t.Fatalf("query filter: %+v", a.shown)
	}
	if a.shown[0].DisplayTitle() != "中文标题" {
		t.Fatal(a.shown[0].DisplayTitle())
	}
}

func TestNativeViewSmoke(t *testing.T) {
	a := newRadarApp(nil, "0.2.0-test")
	a.healthy = true
	a.status = "ok"
	a.sections = []sectionInfo{{Key: "lab", Name: "实验室"}}
	a.items = []feedItem{
		{ID: 1, Title: "Smoke item", Section: "lab", Source: "Test", Summary: "hello", Published: 1.7e9, Priority: "P0"},
		{ID: 2, TitleZh: "中文可读标题", Title: "EN", Section: "lab", Source: "实验室", SummaryZh: "摘要内容用于排版", Published: 1.7e9},
	}
	a.refilter()
	tr := ui.NewTester(a.view, 1180, 760)
	tr.SetDark(true)
	tr.SetVibrancy(true)
	tr.Frame()
	if !tr.HasText("AI News Radar") {
		t.Fatalf("missing title; texts=%v", tr.Texts())
	}
	if !tr.HasText("Native") && !containsAnyExperiment(tr.Texts()) {
		t.Fatalf("missing experiment badge; texts=%v", tr.Texts())
	}
	if !tr.HasText("Smoke item") {
		t.Fatalf("missing feed item; texts=%v", tr.Texts())
	}
	if !tr.HasText("中文可读标题") {
		t.Fatalf("missing Chinese title; texts=%v", tr.Texts())
	}
	if !tr.HasText("全部资讯") {
		t.Fatalf("missing section title; texts=%v", tr.Texts())
	}
	// select first item → detail
	a.selected = 0
	a.openSelected()
	tr.Frame()
	if !tr.HasText("在浏览器打开") && !tr.HasText("复制链接") {
		t.Fatalf("missing detail actions; texts=%v", tr.Texts())
	}
	// about page
	a.nav = navAbout
	tr.Frame()
	if !tr.HasText("关于") {
		t.Fatalf("missing about; texts=%v", tr.Texts())
	}
	// stub / empty polish
	a.nav = navStubBrief
	tr.Frame()
	if !tr.HasText("简报") {
		t.Fatalf("missing stub; texts=%v", tr.Texts())
	}
	// settings
	a.nav = navSettings
	tr.Frame()
	if !tr.HasText("设置") {
		t.Fatalf("missing settings; texts=%v", tr.Texts())
	}
}

func TestApplyThemeRestrainedAccent(t *testing.T) {
	tr := ui.NewTester(func(c *ui.Context) {
		applyTheme(c)
		th := c.Theme()
		if th.Spacing < 5 {
			panic("expected roomier spacing")
		}
		if th.Radius < 8 {
			panic("expected softer radius")
		}
		ui.Text(c, "theme-ok")
	}, 200, 80)
	tr.SetDark(true)
	tr.Frame()
	if !tr.HasText("theme-ok") {
		t.Fatal(tr.Texts())
	}
}

func containsAnyExperiment(texts []string) bool {
	for _, s := range texts {
		if containsSub(s, "实验") || containsSub(s, "Native") || containsSub(s, "预览") {
			return true
		}
	}
	return false
}

func containsSub(s, sub string) bool {
	return len(s) >= len(sub) && (s == sub || len(sub) == 0 || stringIndex(s, sub) >= 0)
}

func stringIndex(s, sub string) int {
	for i := 0; i+len(sub) <= len(s); i++ {
		if s[i:i+len(sub)] == sub {
			return i
		}
	}
	return -1
}
