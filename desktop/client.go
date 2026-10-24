package main

// Thin HTTP client for the in-process radar-server APIs (token via localClient).

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

type sectionInfo struct {
	Key   string `json:"key"`
	Name  string `json:"name"`
	Icon  string `json:"icon"`
	InAll *bool  `json:"inAll"`
}

type feedItem struct {
	ID        int64   `json:"id"`
	Title     string  `json:"title"`
	TitleZh   string  `json:"title_zh"`
	URL       string  `json:"url"`
	Summary   string  `json:"summary"`
	SummaryZh string  `json:"summary_zh"`
	Published float64 `json:"published"`
	Source    string  `json:"source"`
	SourceID  float64 `json:"source_id"`
	Section   string  `json:"section"`
	Priority  string  `json:"priority"`
	IsX       any     `json:"is_x"`
	Author    string  `json:"author"`
	Homepage  string  `json:"homepage"`
	Fav       bool    `json:"fav"`
	Truncated bool    `json:"truncated"`
}

func (it feedItem) DisplayTitle() string {
	if it.TitleZh != "" {
		return it.TitleZh
	}
	return it.Title
}

func (it feedItem) DisplaySummary() string {
	if it.SummaryZh != "" {
		return it.SummaryZh
	}
	return it.Summary
}

type healthOut struct {
	OK bool `json:"ok"`
}

func apiGET(base, path string, dest any) error {
	req, err := http.NewRequest(http.MethodGet, base+path, nil)
	if err != nil {
		return err
	}
	res, err := localClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return err
	}
	if res.StatusCode >= 400 {
		return fmt.Errorf("%s: HTTP %d: %s", path, res.StatusCode, truncate(string(body), 200))
	}
	if dest == nil {
		return nil
	}
	return json.Unmarshal(body, dest)
}

func apiPOST(base, path string, body any) error {
	var r io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return err
		}
		r = bytes.NewReader(b)
	}
	req, err := http.NewRequest(http.MethodPost, base+path, r)
	if err != nil {
		return err
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	res, err := localClient.Do(req)
	if err != nil {
		return err
	}
	defer res.Body.Close()
	io.Copy(io.Discard, res.Body)
	if res.StatusCode >= 400 {
		return fmt.Errorf("%s: HTTP %d", path, res.StatusCode)
	}
	return nil
}

func fetchHealth(base string) (bool, error) {
	var h healthOut
	if err := apiGET(base, "/api/health", &h); err != nil {
		return false, err
	}
	return h.OK, nil
}

func fetchSections(base string) ([]sectionInfo, error) {
	var list []sectionInfo
	if err := apiGET(base, "/api/sections", &list); err != nil {
		return nil, err
	}
	return list, nil
}

func fetchItems(base string, limit int, q string) ([]feedItem, error) {
	if limit <= 0 {
		limit = 200
	}
	u := "/api/items?zh=1&limit=" + strconv.Itoa(limit)
	if q = trimSpace(q); q != "" {
		u += "&q=" + url.QueryEscape(q)
	}
	var list []feedItem
	if err := apiGET(base, u, &list); err != nil {
		return nil, err
	}
	return list, nil
}

func fetchItem(base string, id int64) (*feedItem, error) {
	var list []feedItem
	if err := apiGET(base, "/api/items?zh=1&id="+strconv.FormatInt(id, 10), &list); err != nil {
		return nil, err
	}
	if len(list) == 0 {
		return nil, fmt.Errorf("item %d not found", id)
	}
	return &list[0], nil
}

func markRead(base string, id int64) {
	_ = apiPOST(base, "/api/read", map[string]any{"add": []int64{id}})
}

func fetchSettingsVersion(base string) string {
	var s map[string]any
	if err := apiGET(base, "/api/settings", &s); err != nil {
		return ""
	}
	if v, ok := s["version"].(string); ok {
		return v
	}
	return ""
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}

func trimSpace(s string) string {
	for len(s) > 0 && (s[0] == ' ' || s[0] == '\t' || s[0] == '\n') {
		s = s[1:]
	}
	for len(s) > 0 {
		c := s[len(s)-1]
		if c != ' ' && c != '\t' && c != '\n' {
			break
		}
		s = s[:len(s)-1]
	}
	return s
}

func fmtTime(epoch float64) string {
	if epoch <= 0 {
		return ""
	}
	t := time.Unix(int64(epoch), 0).In(time.FixedZone("CST", 8*3600))
	return t.Format("01-02 15:04")
}

func fmtAgo(epoch float64) string {
	if epoch <= 0 {
		return ""
	}
	d := time.Since(time.Unix(int64(epoch), 0))
	switch {
	case d < time.Minute:
		return "刚刚"
	case d < time.Hour:
		return fmt.Sprintf("%d 分钟前", int(d.Minutes()))
	case d < 24*time.Hour:
		return fmt.Sprintf("%d 小时前", int(d.Hours()))
	default:
		return fmt.Sprintf("%d 天前", int(d.Hours()/24))
	}
}
