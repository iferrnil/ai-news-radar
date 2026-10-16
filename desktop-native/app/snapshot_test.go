package app

import (
	"image/png"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
	"github.com/iferrnil/ai-news-radar/desktop-native/core"
)

// RADAR_SNAPSHOT=<data dir> RADAR_SNAPSHOT_OUT=<dir> go test ./app -run Snapshot  renders the main pages to PNG
// (thumbnails are downloaded into <out>/thumbs).
func TestSnapshot(t *testing.T) {
	dir, out := os.Getenv("RADAR_SNAPSHOT"), os.Getenv("RADAR_SNAPSHOT_OUT")
	if dir == "" || out == "" {
		t.Skip("set RADAR_SNAPSHOT and RADAR_SNAPSHOT_OUT")
	}
	st, err := core.Open(dir, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer st.Close()
	st.Thumbs = core.NewThumbCache(filepath.Join(out, "thumbs"), 200<<20)
	posted := make(chan func(), 256)
	h := &asyncHost{posted: posted}
	h.upd = &fakeUpd{auto: true, fallback: "0.4.1"}
	a := New(st, nil, h)
	a.SetMigrated(core.MigrateResult{Done: true, Sources: 22, Items: 744, Read: 31})
	tt := ui.NewTester(a.View, 1180, 780)
	drain := func(d time.Duration) {
		end := time.After(d)
		for {
			select {
			case fn := <-posted:
				fn()
			case <-end:
				return
			}
		}
	}
	save := func(name string) {
		tt.Frame()
		drain(3 * time.Second)
		tt.Frame()
		f, _ := os.Create(filepath.Join(out, name+".png"))
		png.Encode(f, tt.Image())
		f.Close()
	}
	save("01-feed-banners")
	a.migrated.Done, a.dismissedUpdate = false, true
	tt.Click("娱乐")
	save("02-thumbnails")
	for i := range a.items {
		if a.items[i].Image == "" && strings.TrimSpace(a.items[i].Summary) != "" {
			continue
		}
		a.show(&a.items[i])
		break
	}
	save("03-drawer")
	tt.Click("全部")
	if len(a.items) > 0 {
		a.show(&a.items[0])
	}
	tt.Click("列表")
	save("04-list-drawer")
	a.detail = nil
	tt.Click("运行状态")
	save("05-health")
	tt.Click("设置")
	save("06-settings")
	tt.Click("数据源管理")
	save("07-sources")
	tt.SetDark(true)
	tt.Click("娱乐")
	tt.Click("卡片")
	save("08-dark-thumbnails")
}
