package core

import (
	"fmt"
	"regexp"
	"strings"
)

// Rule is a per-source include/exclude filter (see src/lib/filter-rule.ts).
// Go's regexp is RE2: linear time, so no catastrophic backtracking and no sandbox/timeout is needed.
// JS-only syntax (lookaround, backreferences) is rejected with a Chinese error.
type Rule struct {
	Include, Exclude string
	Scope            string // "title" | "all"
	CS               bool
}

const RuleMax = 200

func (r Rule) Has() bool { return r.Include != "" || r.Exclude != "" }

// RuleText is the string a rule is tested against.
func RuleText(scope, title, summary string) string {
	t := title
	if scope == "all" {
		t = title + "\n" + Trunc(summary, 2000, "")
	}
	return Trunc(t, 2500, "")
}

var nestedQ = regexp.MustCompile(`\((?:[^()\\]|\\.)*[+*}](?:[^()\\]|\\.)*\)\s*[+*{]`)
var backref = regexp.MustCompile(`\\[1-9]`)

// StaticCheck mirrors filter-rule.ts staticCheck (kept identical so a rule valid here is valid in the web app too).
func StaticCheck(p string) string {
	if len([]rune(p)) > RuleMax {
		return fmt.Sprintf("正则过长（最多 %d 字符）", RuleMax)
	}
	if nestedQ.MatchString(p) {
		return "正则含嵌套量词（如 (a+)+），可能导致回溯爆炸"
	}
	if backref.MatchString(p) {
		return "不支持反向引用"
	}
	if strings.Contains(p, "(?=") || strings.Contains(p, "(?!") || strings.Contains(p, "(?<=") || strings.Contains(p, "(?<!") {
		return "正则无效：桌面版不支持前瞻/后顾断言"
	}
	if _, err := regexp.Compile(jsToRE2(p)); err != nil {
		return "正则无效：" + err.Error()
	}
	return ""
}

// jsToRE2 rewrites the few JS spellings RE2 lacks: named groups (?<n>…) -> (?P<n>…), \uXXXX -> \x{XXXX}.
var jsU = regexp.MustCompile(`\\u([0-9a-fA-F]{4})`)

func jsToRE2(p string) string {
	p = strings.ReplaceAll(p, "(?<", "(?P<")
	return jsU.ReplaceAllString(p, `\x{$1}`)
}

// Validate normalizes + checks a rule from user input/import.
func (r Rule) Validate() (Rule, error) {
	r.Include, r.Exclude = strings.TrimSpace(r.Include), strings.TrimSpace(r.Exclude)
	if r.Scope != "all" {
		r.Scope = "title"
	}
	for _, x := range [][2]string{{"包含", r.Include}, {"排除", r.Exclude}} {
		if x[1] == "" {
			continue
		}
		if e := StaticCheck(x[1]); e != "" {
			return r, fmt.Errorf("%s规则：%s", x[0], e)
		}
	}
	return r, nil
}

type compiled struct{ inc, exc *regexp.Regexp }

func (r Rule) compile() compiled {
	fl := "(?i)"
	if r.CS {
		fl = ""
	}
	var c compiled
	if r.Include != "" {
		c.inc, _ = regexp.Compile(fl + jsToRE2(r.Include))
	}
	if r.Exclude != "" {
		c.exc, _ = regexp.Compile(fl + jsToRE2(r.Exclude))
	}
	return c
}

func (c compiled) keeps(text string) bool {
	return (c.inc == nil || c.inc.MatchString(text)) && !(c.exc != nil && c.exc.MatchString(text))
}

// Keeps reports, for each text, whether the rule keeps it.
func (r Rule) Keeps(texts []string) []bool {
	c := r.compile()
	out := make([]bool, len(texts))
	for i, t := range texts {
		out[i] = c.keeps(t)
	}
	return out
}
