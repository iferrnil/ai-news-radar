package core

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"strings"
	"time"
)

// LatestRelease finds the newest non-draft, non-prerelease GitHub release whose tag starts with prefix.
// Used only for the fallback "新版本" prompt; the real install goes through MyGo's signed updater.
func LatestRelease(repo, prefix string) (version, htmlURL string, err error) {
	req, _ := http.NewRequest("GET", "https://api.github.com/repos/"+repo+"/releases?per_page=30", nil)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ai-news-radar-desktop")
	c := &http.Client{Timeout: 15 * time.Second}
	r, err := c.Do(req)
	if err != nil {
		return "", "", err
	}
	defer r.Body.Close()
	if r.StatusCode != 200 {
		return "", "", fmt.Errorf("HTTP %d", r.StatusCode)
	}
	var rel []struct {
		Tag        string `json:"tag_name"`
		URL        string `json:"html_url"`
		Draft      bool   `json:"draft"`
		Prerelease bool   `json:"prerelease"`
	}
	if err := json.NewDecoder(r.Body).Decode(&rel); err != nil {
		return "", "", err
	}
	var ok []relInfo
	for _, x := range rel {
		if !x.Draft && !x.Prerelease {
			ok = append(ok, relInfo{x.Tag, x.URL})
		}
	}
	return PickRelease(ok, prefix)
}

type relInfo struct{ Tag, URL string }

// PickRelease returns the highest version among tags with prefix.
func PickRelease(rels []relInfo, prefix string) (string, string, error) {
	best, link := "", ""
	for _, r := range rels {
		if !strings.HasPrefix(r.Tag, prefix) {
			continue
		}
		v := strings.TrimPrefix(r.Tag, prefix)
		if best == "" || NewerVersion(v, best) {
			best, link = v, r.URL
		}
	}
	return best, link, nil
}

// NewerVersion reports a > b for dotted numeric versions (pre-release suffixes ignored).
func NewerVersion(a, b string) bool {
	pa, pb := vparts(a), vparts(b)
	for i := 0; i < 3; i++ {
		if pa[i] != pb[i] {
			return pa[i] > pb[i]
		}
	}
	return false
}

func vparts(v string) [3]int {
	v = strings.TrimPrefix(v, "v")
	v, _, _ = strings.Cut(v, "-")
	var out [3]int
	for i, p := range strings.SplitN(v, ".", 3) {
		out[i], _ = strconv.Atoi(p)
	}
	return out
}
