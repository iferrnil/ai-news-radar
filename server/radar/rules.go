package radar

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/dlclark/regexp2"
)

// Rule is a per-source include/exclude filter (filter-rule.ts). Patterns are JavaScript regexes, run with
// regexp2 in ECMAScript mode (lookaround etc. like V8) under a hard timeout instead of node:vm.
type Rule struct {
	Include, Exclude any    // string or nil
	Scope            string // "title" | "all"
	CS               bool
}

const RuleMax = 200

func (r Rule) inc() string { s, _ := r.Include.(string); return s }
func (r Rule) exc() string { s, _ := r.Exclude.(string); return s }
func (r Rule) Has() bool   { return r.inc() != "" || r.exc() != "" }

// RuleText is the string a rule is tested against.
func RuleText(scope, title, summary string) string {
	t := title
	if scope == "all" {
		t = title + "\n" + jsSlice(summary, 2000)
	}
	return jsSlice(t, 2500)
}

var nestedQ = regexp.MustCompile(`\((?:[^()\\]|\\.)*[+*}](?:[^()\\]|\\.)*\)\s*[+*{]`)
var backref = regexp.MustCompile(`\\[1-9]`)

func compileJS(p string, cs bool) (*regexp2.Regexp, error) {
	opt := regexp2.RegexOptions(regexp2.ECMAScript)
	if !cs {
		opt |= regexp2.IgnoreCase
	}
	return regexp2.Compile(p, opt)
}

// StaticCheck mirrors filter-rule.ts staticCheck.
func StaticCheck(p string) string {
	if jsLen(p) > RuleMax {
		return fmt.Sprintf("正则过长（最多 %d 字符）", RuleMax)
	}
	if nestedQ.MatchString(p) {
		return "正则含嵌套量词（如 (a+)+），可能导致回溯爆炸"
	}
	if backref.MatchString(p) {
		return "不支持反向引用"
	}
	if _, err := compileJS(p, true); err != nil {
		msg := err.Error()
		if i := strings.Index(msg, " in `"); i > 0 {
			msg = msg[:i]
		}
		msg = strings.TrimPrefix(msg, "error parsing regexp: ")
		for k, v := range v8Msg {
			if strings.HasPrefix(msg, k) {
				msg = v
				break
			}
		}
		return "正则无效：/" + p + "/: " + msg
	}
	return ""
}

// v8Msg maps regexp2 parser errors to the V8 wording users saw in the Node build.
var v8Msg = map[string]string{
	"missing closing )":                       "Unterminated group",
	"unexpected )":                            "Unmatched ')'",
	"unterminated [] set":                     "Unterminated character class",
	"missing argument to repetition operator": "Nothing to repeat",
	"invalid nested repetition operator":      "Nothing to repeat",
	"illegal \\ at end of pattern":            "\\ at end of pattern",
	"invalid character class range":           "Range out of order in character class",
	"unrecognized grouping construct":         "Invalid group",
	"invalid group name":                      "Invalid capture group name",
	"invalid repeat count":                    "numbers out of order in {} quantifier",
}

// dupAlt finds quantified groups whose alternatives can match the same text — (a|a)*, (\w|\d)+ — which backtrack
// exponentially in V8 (the browser runs the same pattern for the live preview); regexp2 optimises some of them away.
var quantGroup = regexp.MustCompile(`\(((?:[^()\\]|\\.)*\|(?:[^()\\]|\\.)*)\)\s*[+*{]`)
var wideAlt = map[string]bool{".": true, "\\w": true, "\\S": true, "\\D": true, "\\W": true, "[^]": true}

func dupAlt(p string) bool {
	for _, m := range quantGroup.FindAllStringSubmatch(p, -1) {
		inner := strings.TrimPrefix(strings.TrimPrefix(m[1], "?:"), "?")
		var alts []string
		depth, cur := 0, ""
		for i := 0; i < len(inner); i++ {
			c := inner[i]
			if c == '\\' && i+1 < len(inner) {
				cur += inner[i : i+2]
				i++
				continue
			}
			if c == '[' {
				depth++
			} else if c == ']' && depth > 0 {
				depth--
			}
			if c == '|' && depth == 0 {
				alts = append(alts, cur)
				cur = ""
				continue
			}
			cur += string(c)
		}
		alts = append(alts, cur)
		seen := map[string]bool{}
		wide := false
		for _, a := range alts {
			k := strings.ToLower(a)
			if seen[k] {
				return true
			}
			seen[k] = true
			if wideAlt[a] {
				wide = true
			}
		}
		if wide && len(alts) > 1 {
			return true
		}
	}
	return false
}

// EvalRule returns keep-flags per text, or nil when the rule timed out / failed (callers then keep everything).
func EvalRule(r Rule, texts []string, timeout time.Duration) []bool {
	out := make([]bool, len(texts))
	if !r.Has() {
		for i := range out {
			out[i] = true
		}
		return out
	}
	deadline := time.Now().Add(timeout)
	var inc, exc *regexp2.Regexp
	var err error
	if r.inc() != "" {
		if inc, err = compileJS(r.inc(), r.CS); err != nil {
			return nil
		}
	}
	if r.exc() != "" {
		if exc, err = compileJS(r.exc(), r.CS); err != nil {
			return nil
		}
	}
	test := func(re *regexp2.Regexp, t string) (bool, bool) {
		left := time.Until(deadline)
		if left <= 0 {
			return false, false
		}
		re.MatchTimeout = left
		ok, err := re.MatchString(t)
		return ok, err == nil
	}
	for i, t := range texts {
		keep := true
		if inc != nil {
			m, ok := test(inc, t)
			if !ok {
				return nil
			}
			keep = m
		}
		if keep && exc != nil {
			m, ok := test(exc, t)
			if !ok {
				return nil
			}
			keep = !m
		}
		out[i] = keep
	}
	return out
}

// ValidateRule normalizes + validates a rule from user input / import (rule-guard.ts validateRule).
func ValidateRule(raw map[string]any, samples []string) (Rule, string) {
	trim := func(v any) any {
		if s, ok := v.(string); ok && strings.TrimSpace(s) != "" {
			return strings.TrimSpace(s)
		}
		return nil
	}
	r := Rule{Include: trim(raw["include"]), Exclude: trim(raw["exclude"]), Scope: "title", CS: truthy(raw["cs"])}
	if raw["scope"] == "all" {
		r.Scope = "all"
	}
	for _, x := range [][2]string{{"包含", r.inc()}, {"排除", r.exc()}} {
		if x[1] == "" {
			continue
		}
		if e := StaticCheck(x[1]); e != "" {
			return r, x[0] + "规则：" + e
		}
	}
	if !r.Has() {
		return r, ""
	}
	// adversarial probes built from the pattern's own characters + real samples, all under one short timeout
	seen := map[rune]bool{}
	var chars []rune
	for _, c := range probeStrip.ReplaceAllString(r.inc()+r.exc(), "") {
		if !seen[c] && len(chars) < 12 {
			seen[c] = true
			chars = append(chars, c)
		}
	}
	probes := append([]string{}, samples[:min(200, len(samples))]...)
	for _, c := range chars {
		probes = append(probes, strings.Repeat(string(c), 2000)+"!")
	}
	probes = append(probes, strings.Repeat("a", 3000)+"!", strings.Repeat(" ", 3000)+"!")
	if dupAlt(r.inc()) || dupAlt(r.exc()) || EvalRule(r, probes, 150*time.Millisecond) == nil {
		return r, "正则执行超时（过于复杂，可能导致回溯爆炸），请简化"
	}
	return r, ""
}

var probeStrip = regexp.MustCompile(`[^\p{L}\p{N} _-]`)

// truthy is JavaScript `!!v` for JSON values.
func truthy(v any) bool {
	switch x := v.(type) {
	case nil:
		return false
	case bool:
		return x
	case float64:
		return x != 0
	case int64:
		return x != 0
	case string:
		return x != ""
	}
	return true
}

// ruleOf reads a source row's stored rule.
func ruleOf(r *Row) Rule {
	return Rule{Include: nilIfEmpty(r.Get("f_include")), Exclude: nilIfEmpty(r.Get("f_exclude")), Scope: r.Str("f_scope"), CS: truthy(r.Get("f_cs"))}
}

func nilIfEmpty(v any) any {
	if s, ok := v.(string); ok && s != "" {
		return s
	}
	return nil
}
