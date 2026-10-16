package radar

import (
	"net/url"
	"regexp"
	"strings"
)

// Import validation (portable.ts EntrySchema, zod v4 semantics incl. .catch() defaults).
const (
	MaxBytes   = 1024 * 1024
	MaxEntries = 2000
)

type Entry struct {
	Name, Type, URL, Username, Homepage string
	Section, SectionName, SectionIcon   string
	SectionInAll, SectionTranslate      *bool
	Translate                           string // "inherit" | "on" | "off" | "" (absent)
	Filter                              *EntryFilter
	Priority                            string
	Interval                            int // 0 = unset
	Enabled                             bool
}

type EntryFilter struct {
	Include, Exclude string
	Scope            string
	CaseSensitive    bool
}

var xUserImport = regexp.MustCompile(`^@?[A-Za-z0-9_]{1,15}$`)

func cleanName(s string) string {
	return jsSlice(strings.TrimSpace(wsRe.ReplaceAllString(ctrlRe.ReplaceAllString(s, ""), " ")), 120)
}

func typeName(v any) string {
	switch v.(type) {
	case nil:
		return "null"
	case string:
		return "string"
	case float64:
		return "number"
	case bool:
		return "boolean"
	case []any:
		return "array"
	}
	return "object"
}

func httpURLCheck(v any) (string, string) {
	s, ok := v.(string)
	if !ok {
		return "", "Invalid input: expected string, received " + typeName(v)
	}
	s = strings.TrimSpace(s)
	if jsLen(s) > 2000 {
		return "", "Too big: expected string to have <=2000 characters"
	}
	u, err := url.Parse(s)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
		return "", "只允许 http(s) URL"
	}
	return s, ""
}

// ValidateEntry returns the clean entry or the zod-style issue messages joined with "；".
func ValidateEntry(raw any) (*Entry, string) {
	m, ok := raw.(map[string]any)
	if !ok {
		return nil, "Invalid input: expected object, received " + typeName(raw)
	}
	var issues []string
	e := &Entry{Priority: "P1", Enabled: true, Section: "media"}
	if _, has := m["name"]; !has {
		issues = append(issues, "Invalid input: expected string, received undefined")
	} else if n, ok := m["name"].(string); ok {
		e.Name = cleanName(n)
		if e.Name == "" {
			issues = append(issues, "名称为空")
		}
	} else {
		issues = append(issues, "Invalid input: expected string, received "+typeName(m["name"]))
	}
	switch t, _ := m["type"].(string); t {
	case "rss", "api", "x", "x_list":
		e.Type = t
	default:
		issues = append(issues, `Invalid option: expected one of "rss"|"api"|"x"|"x_list"`)
	}
	if v, has := m["url"]; has {
		if u, msg := httpURLCheck(v); msg != "" {
			issues = append(issues, msg)
		} else {
			e.URL = u
		}
	}
	if v, has := m["username"]; has {
		if s, ok := v.(string); !ok {
			issues = append(issues, "Invalid input: expected string, received "+typeName(v))
		} else if s = strings.TrimSpace(s); !xUserImport.MatchString(s) {
			issues = append(issues, "X 用户名不合法")
		} else {
			e.Username = strings.TrimPrefix(s, "@")
		}
	}
	if v, has := m["homepage"]; has {
		if u, msg := httpURLCheck(v); msg == "" {
			e.Homepage = u
		}
	}
	if s, ok := m["section"].(string); ok && jsLen(strings.TrimSpace(s)) <= 40 {
		e.Section = strings.TrimSpace(s)
	}
	if s, ok := m["sectionName"].(string); ok {
		if t := strings.TrimSpace(s); t != "" && jsLen(t) <= 24 {
			e.SectionName = t
		}
	}
	if s, ok := m["sectionIcon"].(string); ok && jsLen(strings.TrimSpace(s)) <= 8 {
		e.SectionIcon = strings.TrimSpace(s)
	}
	if b, ok := m["sectionInAll"].(bool); ok {
		e.SectionInAll = &b
	}
	if b, ok := m["sectionTranslate"].(bool); ok {
		e.SectionTranslate = &b
	}
	if t, _ := m["translate"].(string); t == "inherit" || t == "on" || t == "off" { // zod .catch(undefined)
		e.Translate = t
	}
	if v, has := m["filter"]; has {
		if f, ok := v.(map[string]any); !ok {
			issues = append(issues, "Invalid input: expected object, received "+typeName(v))
		} else {
			ef := &EntryFilter{Scope: "title"}
			for _, k := range []string{"include", "exclude"} {
				fv, has := f[k]
				if !has {
					continue
				}
				s, ok := fv.(string)
				if !ok {
					issues = append(issues, "Invalid input: expected string, received "+typeName(fv))
					continue
				}
				if jsLen(s) > 200 {
					issues = append(issues, "Too big: expected string to have <=200 characters")
					continue
				}
				if k == "include" {
					ef.Include = s
				} else {
					ef.Exclude = s
				}
			}
			if f["scope"] == "all" {
				ef.Scope = "all"
			}
			ef.CaseSensitive, _ = f["caseSensitive"].(bool)
			e.Filter = ef
		}
	}
	switch p, _ := m["priority"].(string); p {
	case "P0", "P1", "P2":
		e.Priority = p
	}
	if iv, ok := m["interval"].(float64); ok && iv == float64(int64(iv)) && iv >= 60 && iv <= 86400 {
		e.Interval = int(iv)
	}
	if b, ok := m["enabled"].(bool); ok {
		e.Enabled = b
	}
	if len(issues) == 0 {
		if e.Type == "x" && e.Username == "" {
			issues = append(issues, "X 账号缺少 username")
		}
		if e.Type != "x" && e.URL == "" {
			issues = append(issues, "缺少 URL")
		}
	}
	if len(issues) > 0 {
		return nil, strings.Join(issues, "；")
	}
	return e, ""
}

func xProfileURL(u string) string { return "https://api.fxtwitter.com/2/profile/" + u + "/statuses" }
