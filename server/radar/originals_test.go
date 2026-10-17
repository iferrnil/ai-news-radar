package radar

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestFeedContentColumn(t *testing.T) {
	long := strings.Repeat("Full post paragraph with the whole story. ", 40)
	rss := `<?xml version="1.0"?><rss xmlns:content="http://purl.org/rss/1.0/modules/content/"><channel><title>c</title>
<item><title>A</title><link>https://e.com/a</link><description>Short teaser.</description><content:encoded><![CDATA[<p>` + long + `</p><p>Second para.</p>]]></content:encoded></item>
<item><title>B</title><link>https://e.com/b</link><description>Only a short description.</description></item></channel></rss>`
	f := ParseFeed([]byte(rss))
	if f == nil || len(f.Items) != 2 {
		t.Fatal("parse")
	}
	a, b := f.Items[0], f.Items[1]
	if !strings.Contains(a.Content, "Second para.") || !strings.Contains(a.Content, "\n") || len([]rune(a.Summary)) > 300 || b.Content != "" {
		t.Fatalf("content: %q / %q / %q", a.Content[:60], a.Summary, b.Content)
	}
	st := testStore(t)
	srcs, _ := all(st.DB, "SELECT * FROM sources WHERE type NOT LIKE 'x%' ORDER BY id LIMIT 1")
	a.UID, a.Published = "c1", int64(now())
	if _, _, _, err := st.StoreItems(srcs[0], []FetchedItem{a}); err != nil {
		t.Fatal(err)
	}
	var c string
	st.DB.QueryRow("SELECT COALESCE(content,'') FROM items WHERE uid='c1'").Scan(&c)
	if !strings.Contains(c, "Second para.") {
		t.Fatalf("stored content: %q", c)
	}
}

// An item whose page fails takes the longer text of a duplicate (same title, other source); the brief still lists
// it as lacking full text, and every original is saved under 原文/.
func TestGatherFactsSiblingAndOriginals(t *testing.T) {
	st, srv, _, outDir := composeStore(t, true)
	var sid2 int64
	st.DB.QueryRow("SELECT id FROM sources WHERE type NOT LIKE 'x%' ORDER BY id DESC LIMIT 1").Scan(&sid2)
	var t1 string
	var id1 int64
	st.DB.QueryRow("SELECT id,title FROM items ORDER BY id LIMIT 1").Scan(&id1, &t1)
	sib := strings.Repeat("SIBLING-TEXT the duplicate carries the long version of the story. ", 12)
	st.DB.Exec(`INSERT INTO items(uid,source_id,title,url,summary,content,published,fetched) VALUES('dup',?,?,?,?,?,?,?)`, sid2, t1, "https://other.example/dup", "s", sib, now(), now())
	var br briefOut
	r, _ := http.Get(srv.URL + "/api/brief?ai=1&section=all&ids=" + fmt.Sprint(id1))
	json.NewDecoder(r.Body).Decode(&br)
	r.Body.Close()
	if r.StatusCode != 200 || len(br.NoFullText) != 1 || !strings.Contains(br.NoFullText[0].Reason, "同一事件") {
		t.Fatalf("%d %+v", r.StatusCode, br.NoFullText)
	}
	files, _ := filepath.Glob(filepath.Join(outDir, "原文", "*", "*.md"))
	if len(files) != 1 {
		t.Fatalf("originals: %v", files)
	}
	b, _ := os.ReadFile(files[0])
	md := string(b)
	for _, want := range []string{"title: \"" + t1 + "\"", "url: https://e.com/1", "method: feed", "SIBLING-TEXT"} {
		if !strings.Contains(md, want) {
			t.Fatalf("original lacks %q:\n%s", want, md)
		}
	}
}

func TestTranslateItemEndpoint(t *testing.T) {
	mock := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var j struct{ Messages []struct{ Content string } }
		json.NewDecoder(r.Body).Decode(&j)
		var in []trIn
		json.Unmarshal([]byte(j.Messages[len(j.Messages)-1].Content), &in)
		var out []map[string]any
		for _, x := range in {
			out = append(out, map[string]any{"i": x.I, "t": "译" + x.T, "s": "译" + x.S})
		}
		c, _ := json.Marshal(map[string]any{"r": out})
		json.NewEncoder(w).Encode(map[string]any{"choices": []any{map[string]any{"message": map[string]any{"content": string(c)}}}})
	}))
	defer mock.Close()
	t.Setenv("DEEPSEEK_API_KEY", "")
	st := testStore(t)
	t.Setenv("UPSTREAM_MOCK", mock.URL)
	srv := httptest.NewServer(st.Server(nil))
	defer srv.Close()
	var sid int64
	st.DB.QueryRow("SELECT id FROM sources ORDER BY id LIMIT 1").Scan(&sid)
	body := strings.Repeat("A long English body sentence. ", 100) // > 1500 chars: the drawer asks for more
	st.DB.Exec(`INSERT INTO items(uid,source_id,title,url,summary,published,fetched) VALUES('t1',?,?,?,?,?,?)`, sid, "English headline here", "https://e.com/t", body, now(), now())
	st.DB.Exec(`INSERT INTO items(uid,source_id,title,url,summary,published,fetched) VALUES('t2',?,?,?,?,?,?)`, sid, "中文标题不需要翻译", "https://e.com/z", "中文正文", now(), now())
	var id1, id2 int64
	st.DB.QueryRow("SELECT id FROM items WHERE uid='t1'").Scan(&id1)
	st.DB.QueryRow("SELECT id FROM items WHERE uid='t2'").Scan(&id2)
	if c := postJSON(t, srv.URL+"/api/translate/item", "POST", map[string]any{"id": id1}, nil); c != 503 {
		t.Fatalf("without key: %d", c)
	}
	t.Setenv("DEEPSEEK_API_KEY", "sk-"+"unit"+"test0000")
	var res struct {
		Item   map[string]any
		Needed bool
	}
	if c := postJSON(t, srv.URL+"/api/translate/item", "POST", map[string]any{"id": id1}, &res); c != 200 || !res.Needed ||
		res.Item["title_zh"] != "译English headline here" || len([]rune(fmt.Sprint(res.Item["summary_zh"]))) < 2000 {
		t.Fatalf("translate: %d %+v", c, res)
	}
	if c := postJSON(t, srv.URL+"/api/translate/item", "POST", map[string]any{"id": id2}, &res); c != 200 || res.Needed {
		t.Fatalf("chinese item: %d %+v", c, res.Needed)
	}
}
