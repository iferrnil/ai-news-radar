package radar

import (
	"fmt"
	"math"
	"testing"
	"time"
)

func row(id int, title, url string, src int, pub float64) *Row {
	return obj("id", id, "title", title, "url", url, "source_id", src, "source", fmt.Sprintf("S%d", src), "priority", "P1", "published", pub)
}

func TestBriefDedupe(t *testing.T) {
	rs := []*Row{
		row(1, "OpenAI releases GPT-6 with 1M context", "https://openai.com/blog/gpt-6?utm_source=x", 1, 100),
		row(2, "OpenAI releases GPT-6 with 1M context - The Verge", "https://theverge.com/a", 2, 99),
		row(3, "Something else", "https://openai.com/blog/gpt-6", 3, 98),
		row(4, "OpenAI releases GPT‑6, with a 1M-token context", "https://techcrunch.com/b", 4, 97),
		row(5, "谷歌发布 Gemini 3 多模态模型", "https://a.cn/1", 5, 96),
		row(6, "谷歌发布Gemini 3多模态模型！", "https://b.cn/2", 6, 95),
		row(7, "Anthropic raises Series F", "https://x.com/7", 7, 94),
		row(8, "Meta open-sources Llama 5", "https://x.com/8", 8, 93),
	}
	rs[6].Set("title_zh", "")
	tr := row(9, "Llama 5 开源", "https://y.com/9", 9, 92)
	tr.Set("title_zh", "Meta open-sources Llama 5")
	rs = append(rs, tr)
	got := dedupeBrief(rs)
	var ids []int
	for _, r := range got {
		ids = append(ids, int(r.Num("id")))
	}
	if fmt.Sprint(ids) != "[1 5 7 8]" {
		t.Fatalf("dedupe kept %v, want [1 5 7 8]", ids)
	}
	if dice(bigrams(normTitle("Anthropic raises Series F")), bigrams(normTitle("Meta open-sources Llama 5"))) >= briefSimilar {
		t.Fatal("different stories look similar")
	}
}

func TestBriefAutoPickCapAndMorning(t *testing.T) {
	// fixed "now": 2026-10-23 14:00 Asia/Shanghai
	at := time.Date(2026, 10, 23, 14, 0, 0, 0, cst)
	day0 := time.Date(2026, 10, 23, 0, 0, 0, 0, cst)
	words := []string{"quantum", "robotics", "pricing", "lawsuit", "benchmark", "chips", "funding", "privacy", "agents", "vision",
		"speech", "datacenter", "energy", "safety", "education", "medical", "coding", "search", "music", "video",
		"translation", "weather", "finance", "gaming", "drones", "satellite", "battery", "genome", "climate", "retail"}
	var rs []*Row
	// 25 distinct afternoon items (10:00–14:00) + 5 morning (01:00–07:00)
	for i := 1; i <= 25; i++ {
		pub := float64(day0.Add(time.Duration(10+i%4)*time.Hour + time.Duration(i)*time.Minute).Unix())
		rs = append(rs, row(i, fmt.Sprintf("%s %s %d", words[i-1], words[(i*7)%30], i*37), fmt.Sprintf("https://e.com/%d", i), i, pub))
	}
	mornWords := []string{"xylophone zebra quarry", "bamboo kettle nectar", "cactus orchid pebble", "dolphin quartz raven", "emerald sapphire tulip"}
	for i := 1; i <= 5; i++ {
		pub := float64(day0.Add(time.Duration(i) * time.Hour).Unix())
		id := 100 + i
		rs = append(rs, row(id, mornWords[i-1]+fmt.Sprintf(" dawn brief %d", id), fmt.Sprintf("https://m.com/%d", id), id, pub))
	}
	got := autoPickAt(rs, 20, at, nil)
	if len(got) != 20 {
		t.Fatalf("cap 20 got %d", len(got))
	}
	// all 5 morning items should be preferred in the fill (no multi-coverage here)
	morn := 0
	for _, r := range got {
		if isMorningSH(r.Num("published"), at) {
			morn++
		}
	}
	if morn < 5 {
		t.Fatalf("expected all 5 morning in pick, got %d", morn)
	}
	// ≤20 → all (one per story), never pad
	if g := autoPickAt(rs[:7], 20, at, nil); len(g) != 7 {
		t.Fatalf("≤20 keeps all: %d", len(g))
	}
	// default briefPick is 20
	if briefPick != 20 {
		t.Fatalf("briefPick=%d", briefPick)
	}
}

func TestBriefAutoPickHot(t *testing.T) {
	at := time.Date(2026, 10, 23, 14, 0, 0, 0, cst)
	day0 := time.Date(2026, 10, 23, 0, 0, 0, 0, cst)
	words := []string{"quantum", "robotics", "pricing", "lawsuit", "benchmark", "chips", "funding", "privacy", "agents", "vision",
		"speech", "datacenter", "energy", "safety", "education", "medical", "coding", "search", "music", "video"}
	words2 := append(append([]string(nil), words...), "translation", "weather", "finance", "gaming", "drones", "satellite", "battery", "genome", "climate", "retail")
	var rs []*Row
	for i := 1; i <= 20; i++ {
		pub := float64(day0.Add(10*time.Hour + time.Duration(i)*time.Minute).Unix())
		rs = append(rs, row(i, fmt.Sprintf("%s %s %d", words[i-1], words2[(i*7)%30], i*37), fmt.Sprintf("https://e.com/%d", i), i, pub))
	}
	// hot: 3 copies of story 10, 2 of 12
	rs = append(rs,
		row(101, rs[9].Str("title")+" - The Verge", "https://v.com/a", 50, rs[9].Num("published")+0.5),
		row(102, rs[9].Str("title"), "https://w.com/a", 51, rs[9].Num("published")+0.4),
		row(103, rs[11].Str("title"), "https://v.com/b", 50, rs[11].Num("published")+0.5))
	got := autoPickAt(rs, 10, at, nil)
	var ids []int
	for _, r := range got {
		ids = append(ids, int(r.Num("id")))
	}
	has := func(id int) bool {
		for _, x := range ids {
			if x == id {
				return true
			}
		}
		return false
	}
	n10 := 0
	for _, id := range []int{10, 101, 102} {
		if has(id) {
			n10++
		}
	}
	if len(got) != 10 || n10 != 1 || !(has(12) || has(103)) || (has(12) && has(103)) {
		t.Fatalf("hot stories once each: %v", ids)
	}
	// multi-coverage first: both hot reps present
	if n10 != 1 || !(has(12) || has(103)) {
		t.Fatalf("missing hot: %v", ids)
	}
}

func TestBriefRepPrefersP0(t *testing.T) {
	a := row(1, "Same Story Title Alpha Beta", "https://a.com/1", 1, 100)
	a.Set("priority", "P1")
	b := row(2, "Same Story Title Alpha Beta - Verge", "https://b.com/2", 2, 99)
	b.Set("priority", "P0")
	g := storyGroups([]*Row{a, b})
	if len(g) != 1 || int(g[0][0].Num("id")) != 2 {
		t.Fatalf("P0 should be representative: %+v", g[0][0])
	}
}

func TestBriefFinalDedupe(t *testing.T) {
	at := time.Now().In(cst)
	rs := []*Row{
		row(1, "OpenAI ships GPT-6 Astra model", "https://a.com/1", 1, 100),
		row(2, "Anthropic announces Claude Opus 5", "https://b.com/2", 2, 99),
		row(3, "OpenAI ships GPT-6 Astra model - The Verge", "https://c.com/3", 3, 98), // same story as 1
	}
	got := autoPickAt(rs, 20, at, nil)
	if len(got) != 2 {
		t.Fatalf("final dedupe: %d", len(got))
	}
}

func TestDiceVecMatchesDice(t *testing.T) {
	ts := []string{"OpenAI 发布 GPT-6 Astra", "openai 发布 gpt-6 astra 模型", "aaaa", "abab abab", "Meta open-sources Llama 5", "a", ""}
	for _, a := range ts {
		for _, b := range ts {
			va, na := gramVec(normTitle(a))
			vb, nb := gramVec(normTitle(b))
			if d1, d2 := dice(bigrams(normTitle(a)), bigrams(normTitle(b))), diceVec(va, vb, na, nb); math.Abs(d1-d2) > 1e-12 {
				t.Fatalf("%q %q: %v vs %v", a, b, d1, d2)
			}
		}
	}
}
