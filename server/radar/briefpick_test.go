package radar

import (
	"fmt"
	"testing"
)

func row(id int, title, url string, src int, pub float64) *Row {
	return obj("id", id, "title", title, "url", url, "source_id", src, "source", fmt.Sprintf("S%d", src), "priority", "P1", "published", pub)
}

func TestBriefDedupe(t *testing.T) {
	rs := []*Row{
		row(1, "OpenAI releases GPT-6 with 1M context", "https://openai.com/blog/gpt-6?utm_source=x", 1, 100),
		row(2, "OpenAI releases GPT-6 with 1M context - The Verge", "https://theverge.com/a", 2, 99),   // same title + publisher tail
		row(3, "Something else", "https://openai.com/blog/gpt-6", 3, 98),                             // same URL (utm dropped)
		row(4, "OpenAI releases GPT‑6, with a 1M-token context", "https://techcrunch.com/b", 4, 97), // near-identical
		row(5, "谷歌发布 Gemini 3 多模态模型", "https://a.cn/1", 5, 96),
		row(6, "谷歌发布Gemini 3多模态模型！", "https://b.cn/2", 6, 95),
		row(7, "Anthropic raises Series F", "https://x.com/7", 7, 94),
		row(8, "Meta open-sources Llama 5", "https://x.com/8", 8, 93),
	}
	rs[6].Set("title_zh", "")
	tr := row(9, "Llama 5 开源", "https://y.com/9", 9, 92)
	tr.Set("title_zh", "Meta open-sources Llama 5") // translated duplicate of 8
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

func TestBriefSpreadPick(t *testing.T) {
	words := []string{"quantum", "robotics", "pricing", "lawsuit", "benchmark", "chips", "funding", "privacy", "agents", "vision",
		"speech", "datacenter", "energy", "safety", "education", "medical", "coding", "search", "music", "video",
		"translation", "weather", "finance", "gaming", "drones", "satellite", "battery", "genome", "climate", "retail"}
	var rs []*Row
	for i := 1; i <= 30; i++ { // published = i: 1 oldest … 30 newest
		rs = append(rs, row(i, fmt.Sprintf("%s %s %d", words[i-1], words[(i*7)%30], i*37), fmt.Sprintf("https://e.com/%d", i), i, float64(i)))
	}
	if n := len(dedupeBrief(rs)); n != 30 {
		t.Fatalf("distinct titles deduped to %d", n)
	}
	order, groups, _ := selectBrief(rs)
	var pubs []int
	for _, sid := range order {
		for _, r := range groups[sid] {
			pubs = append(pubs, int(r.Num("published")))
		}
	}
	// newest first: 3 newest (30,29,28), 3 middle of 5..27 (15,16,17), 4 oldest (4,3,2,1)
	if fmt.Sprint(pubs) != "[30 29 28 17 16 15 4 3 2 1]" {
		t.Fatalf("picked %v", pubs)
	}
	if got := spreadPick(rs[:7], 10); len(got) != 7 || got[0].Num("published") != 7 {
		t.Fatalf("≤10 keeps all, newest first: %d", len(got))
	}
}
