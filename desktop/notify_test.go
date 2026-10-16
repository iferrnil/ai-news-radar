package main

import (
	"strings"
	"testing"
)

func sp(s string) *string { return &s }

func TestFilterBatch(t *testing.T) {
	b := notifyBatch{Count: 7, Items: []notifyItem{{ID: 1, Section: sp("lab")}, {ID: 2, Section: sp("x")}, {ID: 3}}}
	if got := filterBatch(b, nil); got == nil || got.Count != 7 {
		t.Fatalf("all sections: %+v", got)
	}
	got := filterBatch(b, []string{"x"})
	if got == nil || got.Count != 1 || got.Items[0].ID != 2 {
		t.Fatalf("x only: %+v", got)
	}
	if filterBatch(b, []string{"cn"}) != nil {
		t.Fatal("no match should be nil")
	}
}

func TestBatchText(t *testing.T) {
	title, body := batchText(notifyBatch{Count: 1, Items: []notifyItem{{Source: "OpenAI", Title: "GPT-6"}}})
	if title != "P0 · OpenAI" || body != "GPT-6" {
		t.Fatal(title, body)
	}
	items := []notifyItem{{Source: "a", Title: "1"}, {Source: "b", Title: "2"}, {Source: "c", Title: "3"}, {Source: "d", Title: "4"}}
	title, body = batchText(notifyBatch{Count: 9, Items: items})
	if title != "9 条新 P0 动态" || !strings.Contains(body, "…还有 6 条") || strings.Contains(body, "d：4") {
		t.Fatal(title, body)
	}
}

func TestReadSSE(t *testing.T) {
	in := "retry: 3000\n\n: ping\n\nevent: item\ndata: {\"a\":1}\n\nevent: notify\ndata: {\"batch\":\ndata: 1}\n\n"
	var got []string
	if err := readSSE(strings.NewReader(in), func(e, d string) { got = append(got, e+"="+d) }); err != nil {
		t.Fatal(err)
	}
	if len(got) != 2 || got[0] != `item={"a":1}` || got[1] != "notify={\"batch\":\n1}" {
		t.Fatalf("%q", got)
	}
}

func TestPrefsWants(t *testing.T) {
	off := false
	cases := []struct {
		p     notifyPrefs
		event string
		want  bool
	}{
		{notifyPrefs{Enabled: true}, "notify", true},
		{notifyPrefs{Enabled: true}, "digest", false},
		{notifyPrefs{Enabled: true, Scope: "all"}, "digest", true},
		{notifyPrefs{Enabled: true, Scope: "all"}, "notify", false},
		{notifyPrefs{Enabled: true, System: &off}, "notify", false},
		{notifyPrefs{Enabled: false, Scope: "all"}, "digest", false},
	}
	for i, c := range cases {
		if got := c.p.wants(c.event); got != c.want {
			t.Errorf("case %d: %v", i, got)
		}
	}
}

func TestDigestText(t *testing.T) {
	items := []notifyItem{{Source: "a", Title: "1"}, {Source: "b", Title: "2"}, {Source: "c", Title: "3"}, {Source: "d", Title: "4"}}
	title, body := batchText(notifyBatch{Count: 5, Items: items, Digest: true})
	if title != "新增 5 条" || !strings.Contains(body, "· a：1") || !strings.Contains(body, "…还有 2 条") {
		t.Fatal(title, body)
	}
	title, _ = batchText(notifyBatch{Count: 1, Items: items[:1], Digest: true})
	if title != "a" {
		t.Fatal(title)
	}
}

