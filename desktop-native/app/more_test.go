package app

import (
	"bytes"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/iferrnil/ai-news-radar/desktop-native/core"
)

func TestHealthNotifyUpdates(t *testing.T) {
	a, h, tt := setup(t)
	h.upd = &fakeUpd{auto: true, fallback: "0.5.0"}
	tt.Frame()
	if !tt.HasText("发现新版本 0.5.0（当前 0.4.0）") {
		t.Errorf("fallback banner missing: %q", tt.Texts())
	}
	tt.Click("前往下载")
	if len(h.opened) != 1 {
		t.Error("download link not opened")
	}
	if err := tt.Click("运行状态"); err != nil {
		t.Fatal(err)
	}
	for _, s := range []string{"正常", "出错", "冷却中（熔断）", "Lab Feed", "等待首次抓取"} {
		if !tt.HasText(s) {
			t.Fatalf("health missing %q: %q", s, tt.Texts())
		}
	}
	tt.Click("设置")
	if !tt.HasText("通知哪些分区（都不选 = 全部分区）") || !tt.HasText("自动检查更新") {
		t.Fatalf("settings: %q", tt.Texts())
	}
	_ = a
}

func TestNotifySectionToggle(t *testing.T) {
	a, _, tt := setup(t)
	a.toggleNotifySection("lab", true)
	a.toggleNotifySection("ent", true)
	a.toggleNotifySection("lab", false)
	if p := a.st.NotifyPrefs(); !p.Enabled || len(p.Sections) != 1 || p.Sections[0] != "ent" {
		t.Fatalf("%+v", p)
	}
	a.sel = "page:settings"
	tt.Frame()
	if !tt.HasText("实验室") {
		t.Error("section checkboxes missing")
	}
}

func TestThumbnailsLazy(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, 800, 450))
	for x := 0; x < 800; x++ {
		img.Set(x, 10, color.RGBA{255, 0, 0, 255})
	}
	var buf bytes.Buffer
	png.Encode(&buf, img)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/ok.png" {
			w.Header().Set("Content-Type", "image/png")
			w.Write(buf.Bytes())
			return
		}
		w.Write([]byte("garbage"))
	}))
	defer srv.Close()
	st, _ := core.Open(t.TempDir(), []byte(`{"sources":[{"name":"Pics","type":"api","url":"https://api.bilibili.com/x/web-interface/popular","group":"娱乐"}]}`))
	defer st.Close()
	st.Thumbs = core.NewThumbCache(t.TempDir(), 50<<20)
	src, _ := st.Sources()
	src[0].LastFetched = 1
	now := time.Now().Unix()
	st.StoreItems(src[0], []core.FetchedItem{
		{UID: "p/1", Title: "有图", URL: "https://p.test/1", Published: now, Image: srv.URL + "/ok.png"},
		{UID: "p/2", Title: "坏图", URL: "https://p.test/2", Published: now - 1, Image: srv.URL + "/bad.png"},
	})
	posted := make(chan func(), 8)
	h := &asyncHost{posted: posted}
	a := New(st, nil, h)
	a.sel = "sec:ent"
	a.Reload()
	tt := ui.NewTester(a.View, 1000, 700)
	deadline := time.After(5 * time.Second)
	for done := 0; done < 2; {
		select {
		case fn := <-posted:
			fn()
			done++
		case <-deadline:
			t.Fatal("thumbnails never resolved")
		}
	}
	tt.Frame()
	ok, bad := a.thumbs.m[srv.URL+"/ok.png"], a.thumbs.m[srv.URL+"/bad.png"]
	if ok == nil || ok.bmp == nil || bad == nil || !bad.failed {
		t.Fatalf("ok=%+v bad=%+v", ok, bad)
	}
	if w, _ := ok.bmp.Size(); w != thumbW {
		t.Errorf("not downscaled: %v", w)
	}
	if !tt.HasText("坏图") {
		t.Error("item with broken image must still render")
	}
}

type asyncHost struct {
	fakeHost
	posted chan func()
}

func (h *asyncHost) Post(fn func()) { h.posted <- fn }
