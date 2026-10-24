package main

import (
	"fmt"
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestPerfFrames(t *testing.T) {
	a := newRadarApp(nil, "0.1.0")
	a.healthy = true
	a.status = "perf"
	a.sections = []sectionInfo{{Key: "lab", Name: "实验室"}, {Key: "cn", Name: "国内媒体"}}
	items := make([]feedItem, 200)
	for i := range items {
		items[i] = feedItem{ID: int64(i + 1), Title: fmt.Sprintf("Item %d title about AI news radar", i), Section: "lab", Source: "Src", Summary: "summary text for performance", Published: 1.7e9}
	}
	a.items = items
	a.refilter()
	tr := ui.NewTester(a.view, 1280, 840)
	tr.SetDark(true)
	tr.SetVibrancy(true)
	for i := 0; i < 5; i++ {
		tr.Frame()
	}
	n := 30
	t0 := time.Now()
	for i := 0; i < n; i++ {
		tr.Frame()
	}
	elapsed := time.Since(t0)
	avg := elapsed / time.Duration(n)
	t.Logf("frames=%d total=%v avg=%v", n, elapsed.Round(time.Millisecond), avg.Round(time.Microsecond))
	if avg > 50*time.Millisecond {
		t.Fatalf("avg frame too slow: %v", avg)
	}
}
