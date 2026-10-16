package app

import (
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/iferrnil/ai-news-radar/desktop-native/core"
)

type fakeHost struct {
	opened []string
	upd    *fakeUpd
}

func (f *fakeHost) Post(fn func())                    { fn() }
func (f *fakeHost) OpenURL(u string)                  { f.opened = append(f.opened, u) }
func (f *fakeHost) SaveFile(string, []byte) error     { return nil }
func (f *fakeHost) OpenFile() (string, []byte, error) { return "", nil, nil }
func (f *fakeHost) Notify(title, body, url string)    {}

func setup(t *testing.T) (*App, *fakeHost, *ui.Tester) {
	t.Helper()
	seed := `{"sources":[{"name":"Lab Feed","type":"rss","url":"https://lab.test/feed","group":"官方实验室","priority":"P0"},
	{"name":"Fun Feed","type":"rss","url":"https://fun.test/feed","group":"娱乐"}]}`
	st, err := core.Open(t.TempDir(), []byte(seed))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { st.Close() })
	list, _ := st.Sources()
	now := time.Now().Unix()
	for _, s := range list {
		s.LastFetched = 1
		host := strings.ToLower(strings.Fields(s.Name)[0])
		st.StoreItems(s, []core.FetchedItem{
			{UID: host + "/1", Title: s.Name + " 头条新闻", URL: "https://" + host + ".test/1", Summary: "摘要内容", Published: now - 120},
		})
	}
	h := &fakeHost{}
	a := New(st, nil, h)
	tt := ui.NewTester(a.View, 1100, 760)
	return a, h, tt
}

func TestFeedAndSidebar(t *testing.T) {
	a, h, tt := setup(t)
	for _, s := range []string{"全部", "实验室", "娱乐", "数据源管理", "设置", "Lab Feed 头条新闻"} {
		if !tt.HasText(s) {
			t.Fatalf("missing %q in %q", s, tt.Texts())
		}
	}
	// 娱乐 has in_all=0: its item is not in 全部
	if tt.HasText("Fun Feed 头条新闻") {
		t.Error("娱乐 item should not be in 全部")
	}
	if err := tt.Click("娱乐"); err != nil {
		t.Fatal(err)
	}
	if a.sel != "sec:ent" || !tt.HasText("Fun Feed 头条新闻") {
		t.Fatalf("section view: sel=%s %q", a.sel, tt.Texts())
	}
	if err := tt.Click("Fun Feed 头条新闻"); err != nil {
		t.Fatal(err)
	}
	if a.detail == nil || !tt.HasText("详情") {
		t.Fatalf("drawer not open: %q", tt.Texts())
	}
	if err := tt.Click("在浏览器中打开"); err != nil {
		t.Fatal(err)
	}
	if len(h.opened) != 1 || h.opened[0] != "https://fun.test/1" {
		t.Errorf("opened %v", h.opened)
	}
	if a.unread["ent"] != 0 {
		t.Errorf("should be read: %v", a.unread)
	}
	// list mode
	tt.Click("列表")
	tt.Frame()
	if a.view != 1 || !tt.HasText("Fun Feed 头条新闻") {
		t.Error("list mode")
	}
}

func TestSourcesAndSettingsPages(t *testing.T) {
	a, _, tt := setup(t)
	if err := tt.Click("数据源管理"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"添加数据源", "新建分区", "导入", "导出", "Lab Feed", "Fun Feed"} {
		if !tt.HasText(s) {
			t.Fatalf("sources page missing %q: %q", s, tt.Texts())
		}
	}
	// create a section through the dialog
	if err := tt.Click("新建分区"); err != nil {
		t.Fatal(err)
	}
	tt.Type("工具")
	if err := tt.Click("创建"); err != nil {
		t.Fatal(err)
	}
	tt.Frame()
	found := false
	for _, s := range a.sections {
		found = found || s.Name == "工具"
	}
	if !found {
		t.Fatalf("section not created: %+v (dialog err %q)", a.sections, a.dlg.addErr)
	}
	if err := tt.Click("设置"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"P0 快讯通知", "启用翻译", "API key", "侧栏显示数据源"} {
		if !tt.HasText(s) {
			t.Fatalf("settings missing %q", s)
		}
	}
	if a.st.TranslateEnabled() {
		t.Error("translation must be off by default")
	}
}

func (f *fakeHost) Updates() Updates {
	if f.upd == nil {
		return nil
	}
	return f.upd
}

type fakeUpd struct {
	auto     bool
	fallback string
}

func (u *fakeUpd) Version() string { return "0.4.0" }
func (u *fakeUpd) Auto() bool      { return u.auto }
func (u *fakeUpd) SetAuto(on bool) { u.auto = on }
func (u *fakeUpd) Check()          {}
func (u *fakeUpd) Fallback() (string, string) {
	if u.fallback == "" {
		return "", ""
	}
	return u.fallback, "https://github.com/iferrnil/ai-news-radar/releases"
}
