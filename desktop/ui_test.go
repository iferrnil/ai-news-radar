package main

import (
	"strings"
	"testing"

	"github.com/egoist/mygo"
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

func TestAppMenuShortcuts(t *testing.T) {
	var got []string
	m := appMenu(nil, func(c string) func(*mygo.MenuItem, *mygo.Window) {
		return func(*mygo.MenuItem, *mygo.Window) { got = append(got, c) }
	})
	for label, acc := range map[string]string{"显示/隐藏侧边栏": "CmdOrCtrl+B", "设置…": "CmdOrCtrl+,", "搜索…": "CmdOrCtrl+F"} {
		it := find(m, label)
		if it == nil || it.Accelerator != acc || it.Click == nil {
			t.Fatalf("%s: %+v", label, it)
		}
		it.Click(it, nil)
	}
	if v := find(m, "视图"); v == nil || v.Submenu[0].Label != "显示/隐藏侧边栏" {
		t.Fatal("sidebar item not first in 视图")
	}
	if len(got) != 3 {
		t.Fatal(got)
	}
}

func TestWindowOptions(t *testing.T) {
	o := windowOptions()
	if o.StateKey == "" || o.MinWidth < 700 || o.MinHeight < 400 {
		t.Fatalf("%+v", o)
	}
	js := o.Page.PreloadScript
	for _, s := range []string{"__radarDesktop=true", `"theme","system"`} {
		if !strings.Contains(js, s) {
			t.Fatal("preload missing", s)
		}
	}
	if pageCmdJS("sidebar") != `typeof window.__radarCmd === "function" ? (window.__radarCmd("sidebar"), true) : false` {
		t.Fatal(pageCmdJS("sidebar"))
	}
}
