package radar

import (
	"encoding/json"
	"math"
	"regexp"
	"sort"
	"strings"
	"unicode/utf8"

	"golang.org/x/net/html"
	"golang.org/x/net/html/atom"
)

// Main-text extraction for the self-written fetcher. Candidates, best first:
//   json-ld   schema.org articleBody (NewsArticle / BlogPosting / …)
//   dom       readability-style scoring: paragraphs score their parent / grandparent (length, commas incl. CJK
//             punctuation, class / id hints), scores are damped by link density, the best node plus its
//             qualifying siblings is the article
//   embedded  __NEXT_DATA__ / application/json / window.__INITIAL_STATE__-style JSON: the longest text field
//             under a content-like key (also rich-text trees such as Portable Text / Contentful)
//   meta      og:description / description (partial only)

type extracted struct {
	Text   string
	Method string // json-ld | dom | embedded | meta
	Title  string
	Full   bool // Text has ≥ articleFullMin runes from a real body (not just the meta description)
}

var (
	reUnlikely  = regexp.MustCompile(`(?i)-ad-|\bads?\b|advert|banner|breadcrumb|combx|comment|community|cover-wrap|disqus|extra|footer|gdpr|header|legends|menu|modal|nav|newsletter|pager|pagination|popup|promo|related|remark|replies|rss|share|shoutbox|sidebar|skyscraper|social|sponsor|subscribe|supplemental|tags?\b|toolbar|tooltip|cookie|recommend|hot-?list|copyright|download-app|qrcode|login|signup`)
	reMaybe     = regexp.MustCompile(`(?i)and|article|body|column|content|main|shadow|post|entry|text|story|detail|rich|prose|markdown`)
	rePositive  = regexp.MustCompile(`(?i)article|body|content|entry|hentry|h-entry|main|page|pagination|post|text|blog|story|detail|rich_media|prose|markdown|news`)
	reNegative  = regexp.MustCompile(`(?i)-ad-|hidden|^hid$| hid$| hid |^hid |banner|combx|comment|com-|contact|foot|footer|footnote|gdpr|masthead|media|meta|outbrain|promo|related|scroll|share|shoutbox|sidebar|skyscraper|sponsor|shopping|tags|tool|widget|recommend|author-info|byline`)
	reCommas    = regexp.MustCompile(`[,，、。；;！？]`)
	reStateJSON = regexp.MustCompile(`(?s)(?:window\.)?(__INITIAL_STATE__|__INITIAL_DATA__|__PRELOADED_STATE__|__APOLLO_STATE__|__NUXT_DATA__|__DATA__|__STATE__)\s*=\s*`)
	reByline    = regexp.MustCompile(`(?i)^(?:(?:written )?by\s+\S|(?:作者|文|编辑|撰文|编译|来源|责编|责任编辑|记者|图)\s*[:：|丨｜/])`)
	reHiddenCSS = regexp.MustCompile(`(?i)display\s*:\s*none|visibility\s*:\s*hidden`)
)

var dropTags = map[atom.Atom]bool{atom.Script: true, atom.Style: true, atom.Noscript: true, atom.Svg: true, atom.Iframe: true, atom.Form: true,
	atom.Button: true, atom.Nav: true, atom.Footer: true, atom.Aside: true, atom.Template: true, atom.Select: true, atom.Input: true,
	atom.Textarea: true, atom.Canvas: true, atom.Object: true, atom.Embed: true, atom.Figure: true, atom.Picture: true, atom.Video: true,
	atom.Audio: true, atom.Math: true, atom.Dialog: true, atom.Menu: true, atom.Link: true, atom.Meta: true}

var blockTags = map[atom.Atom]bool{atom.P: true, atom.Div: true, atom.Section: true, atom.Article: true, atom.Main: true, atom.Li: true,
	atom.Ul: true, atom.Ol: true, atom.Blockquote: true, atom.Pre: true, atom.Table: true, atom.Tr: true, atom.Td: true, atom.Th: true,
	atom.H1: true, atom.H2: true, atom.H3: true, atom.H4: true, atom.H5: true, atom.H6: true, atom.Br: true, atom.Hr: true, atom.Dl: true,
	atom.Dd: true, atom.Dt: true, atom.Header: true, atom.Figcaption: true, atom.Address: true}

func attr(n *html.Node, k string) string {
	for _, a := range n.Attr {
		if a.Key == k {
			return a.Val
		}
	}
	return ""
}

// extractPage pulls the article out of an HTML page.
func extractPage(page string) extracted {
	doc, err := html.Parse(strings.NewReader(page))
	if err != nil {
		return extracted{}
	}
	meta := map[string]string{}
	var title string
	var ld, embedded []string
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode {
			switch n.DataAtom {
			case atom.Title:
				if title == "" && n.FirstChild != nil {
					title = strings.TrimSpace(n.FirstChild.Data)
				}
			case atom.Meta:
				k := strings.ToLower(firstNonEmpty(attr(n, "property"), attr(n, "name"), attr(n, "itemprop")))
				if c := attr(n, "content"); k != "" && c != "" && meta[k] == "" {
					meta[k] = c
				}
			case atom.Script:
				if n.FirstChild != nil {
					body := n.FirstChild.Data
					typ := strings.ToLower(attr(n, "type"))
					switch {
					case strings.Contains(typ, "ld+json"):
						ld = append(ld, body)
					case attr(n, "id") == "__NEXT_DATA__" || strings.Contains(typ, "json"):
						embedded = append(embedded, body)
					default:
						if loc := reStateJSON.FindStringIndex(body); loc != nil {
							embedded = append(embedded, body[loc[1]:])
						}
					}
				}
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(doc)
	title = firstNonEmpty(meta["og:title"], title)
	desc := Clean(firstNonEmpty(meta["og:description"], meta["description"], meta["twitter:description"]), 1200)

	ldText := jsonLDBody(ld)
	embText := embeddedBody(embedded)
	domText := readability(doc)

	n := func(s string) int { return utf8.RuneCountInString(s) }
	pick := extracted{Title: Clean(title, 300)}
	switch {
	case n(ldText) >= articleFullMin && n(domText) <= n(ldText)*3/2:
		pick.Text, pick.Method = ldText, "json-ld"
	case n(domText) >= articleFullMin && n(embText) <= n(domText)*3/2:
		pick.Text, pick.Method = domText, "dom"
	case n(embText) >= articleFullMin:
		pick.Text, pick.Method = embText, "embedded"
	}
	if pick.Text != "" {
		pick.Full = true
	} else { // partial: the longest thing we have
		for _, c := range []struct{ t, m string }{{domText, "dom"}, {ldText, "json-ld"}, {embText, "embedded"}, {desc, "meta"}} {
			if n(c.t) > n(pick.Text) {
				pick.Text, pick.Method = c.t, c.m
			}
		}
	}
	if pick.Text != "" && reLoginWall.MatchString(pick.Text) && n(pick.Text) < 400 {
		return extracted{Title: pick.Title}
	}
	pick.Text = capArticle(pick.Text)
	return pick
}

func capArticle(t string) string {
	t = strings.TrimSpace(t)
	if r := []rune(t); len(r) > articleMaxRunes {
		t = string(r[:articleMaxRunes]) + "…"
	}
	return t
}

// ---- JSON-LD

func jsonLDBody(scripts []string) string {
	best := ""
	var visit func(v any)
	visit = func(v any) {
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				visit(e)
			}
		case map[string]any:
			for _, k := range []string{"articleBody", "text"} {
				if s, ok := x[k].(string); ok {
					if k == "text" && !ldArticleType(x["@type"]) {
						continue
					}
					if t := cleanBlock(s, articleMaxRunes*2); utf8.RuneCountInString(t) > utf8.RuneCountInString(best) {
						best = t
					}
				}
			}
			for _, k := range []string{"@graph", "mainEntity", "mainEntityOfPage", "hasPart", "itemListElement"} {
				if c, ok := x[k]; ok {
					visit(c)
				}
			}
		}
	}
	for _, s := range scripts {
		var v any
		if json.Unmarshal([]byte(strings.TrimSpace(s)), &v) == nil {
			visit(v)
		}
	}
	return best
}

func ldArticleType(t any) bool {
	var ts []string
	switch x := t.(type) {
	case string:
		ts = []string{x}
	case []any:
		for _, e := range x {
			if s, ok := e.(string); ok {
				ts = append(ts, s)
			}
		}
	}
	for _, s := range ts {
		if strings.Contains(s, "Article") || strings.Contains(s, "Posting") || s == "Report" {
			return true
		}
	}
	return false
}

// ---- embedded JSON (__NEXT_DATA__ etc.)

var contentKeys = map[string]bool{"content": true, "body": true, "articlebody": true, "article_body": true, "text": true, "html": true,
	"contenthtml": true, "content_html": true, "articlecontent": true, "article_content": true, "bodyhtml": true, "body_html": true,
	"richtext": true, "rich_text": true, "markdown": true, "fulltext": true, "full_text": true, "desc": true, "description": true, "post_content": true}

func embeddedBody(scripts []string) string {
	best := ""
	consider := func(t string) {
		if utf8.RuneCountInString(t) > utf8.RuneCountInString(best) {
			best = t
		}
	}
	var visit func(key string, v any, depth int)
	visit = func(key string, v any, depth int) {
		if depth > 40 {
			return
		}
		k := strings.ToLower(key)
		switch x := v.(type) {
		case string:
			if contentKeys[k] && len(x) > 200 {
				t := cleanBlock(x, articleMaxRunes*2)
				if proseLike(t) {
					consider(t)
				}
			}
		case []any:
			if contentKeys[k] { // rich-text block list (Portable Text, Contentful, Draft.js …)
				if t := richText(x); proseLike(t) {
					consider(t)
				}
			}
			for _, e := range x {
				visit(key, e, depth+1)
			}
		case map[string]any:
			if contentKeys[k] {
				if t := richText(x); proseLike(t) {
					consider(t)
				}
			}
			for kk, e := range x {
				visit(kk, e, depth+1)
			}
		}
	}
	for _, s := range scripts {
		dec := json.NewDecoder(strings.NewReader(strings.TrimSpace(s)))
		var v any
		if dec.Decode(&v) == nil { // stops after the first value (window.X = {...};)
			visit("", v, 0)
		}
	}
	return best
}

// richText joins the "text" / "value" leaves of a rich-text tree, one paragraph per block.
func richText(v any) string {
	var paras []string
	var cur strings.Builder
	var walk func(v any, depth int)
	walk = func(v any, depth int) {
		if depth > 30 {
			return
		}
		switch x := v.(type) {
		case []any:
			for _, e := range x {
				walk(e, depth+1)
			}
		case map[string]any:
			if s, ok := x["text"].(string); ok {
				cur.WriteString(s)
			} else if s, ok := x["value"].(string); ok {
				cur.WriteString(s)
			}
			for _, k := range []string{"children", "content", "nodes", "blocks", "spans"} {
				if c, ok := x[k]; ok {
					if _, isStr := c.(string); !isStr {
						walk(c, depth+1)
					}
				}
			}
			if _, block := x["_type"]; block || x["nodeType"] == "paragraph" || x["type"] == "paragraph" {
				if t := strings.TrimSpace(cur.String()); t != "" {
					paras = append(paras, t)
				}
				cur.Reset()
			}
		}
	}
	walk(v, 0)
	if t := strings.TrimSpace(cur.String()); t != "" {
		paras = append(paras, t)
	}
	return strings.Join(paras, "\n\n")
}

// proseLike: sentences rather than a list of names / ids / URLs.
func proseLike(t string) bool {
	n := utf8.RuneCountInString(t)
	if n < 200 {
		return false
	}
	cjk := len(cjkRe.FindAllStringIndex(t, -1))
	if cjk > n/4 {
		return len(reCommas.FindAllStringIndex(t, -1)) >= 3
	}
	return strings.Count(t, " ") > n/12 && strings.Count(t, "http") < n/200+3
}

// ---- readability

type scored struct {
	n     *html.Node
	score float64
}

// classID: class + id with utility-CSS noise removed (Tailwind arbitrary values / variants like
// "[--header-h:…]" or "md:hidden" would otherwise look like "header" / "hidden").
func classID(n *html.Node) string {
	return cleanClass(attr(n, "class")) + " " + cleanClass(attr(n, "id"))
}

func cleanClass(s string) string {
	var keep []string
	for _, tok := range strings.Fields(s) {
		if strings.ContainsAny(tok, "[]():@/!=") || strings.HasPrefix(tok, "--") {
			continue
		}
		keep = append(keep, tok)
	}
	return strings.Join(keep, " ")
}

func classWeight(n *html.Node) float64 {
	w := 0.0
	for _, s := range []string{cleanClass(attr(n, "class")), cleanClass(attr(n, "id"))} {
		if s == "" {
			continue
		}
		if reNegative.MatchString(s) {
			w -= 25
		}
		if rePositive.MatchString(s) {
			w += 25
		}
	}
	return w
}

func tagWeight(a atom.Atom) float64 {
	switch a {
	case atom.Article:
		return 10
	case atom.Div, atom.Main, atom.Section:
		return 5
	case atom.Pre, atom.Td, atom.Blockquote:
		return 3
	case atom.Address, atom.Ol, atom.Ul, atom.Dl, atom.Dd, atom.Dt, atom.Li, atom.Form:
		return -3
	case atom.H1, atom.H2, atom.H3, atom.H4, atom.H5, atom.H6, atom.Th:
		return -5
	}
	return 0
}

// innerText: whitespace-normalised text of n.
func innerText(n *html.Node) string {
	var b strings.Builder
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.TextNode {
			b.WriteString(n.Data)
			return
		}
		if n.Type == html.ElementNode && dropTags[n.DataAtom] {
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if n.Type == html.ElementNode && blockTags[n.DataAtom] {
			b.WriteByte(' ')
		}
	}
	walk(n)
	return strings.Join(strings.Fields(b.String()), " ")
}

func linkDensity(n *html.Node, textLen int) float64 {
	if textLen == 0 {
		return 0
	}
	l := 0
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		if n.Type == html.ElementNode && n.DataAtom == atom.A {
			href := attr(n, "href")
			w := 1.0
			if strings.HasPrefix(href, "#") {
				w = 0.3
			}
			l += int(float64(utf8.RuneCountInString(innerText(n))) * w)
			return
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
	}
	walk(n)
	return math.Min(1, float64(l)/float64(textLen))
}

func hasBlockChild(n *html.Node) bool {
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if c.Type == html.ElementNode && blockTags[c.DataAtom] && c.DataAtom != atom.Br {
			return true
		}
	}
	return false
}

func remove(n *html.Node) {
	if n.Parent != nil {
		n.Parent.RemoveChild(n)
	}
}

// prune drops non-content elements, hidden ones and "unlikely candidates" (by class / id).
func prune(n *html.Node) {
	for c := n.FirstChild; c != nil; {
		next := c.NextSibling
		switch c.Type {
		case html.CommentNode:
			remove(c)
		case html.ElementNode:
			ci := classID(c)
			hidden := attr(c, "hidden") != "" || hasAttr(c, "hidden") || strings.EqualFold(attr(c, "aria-hidden"), "true") || reHiddenCSS.MatchString(attr(c, "style"))
			unlikely := c.DataAtom != atom.Body && c.DataAtom != atom.Article && c.DataAtom != atom.Main && c.DataAtom != atom.A &&
				strings.TrimSpace(ci) != "" && reUnlikely.MatchString(ci) && !reMaybe.MatchString(ci)
			if unlikely && (findFirst(c, atom.Article) != nil || findFirst(c, atom.Main) != nil) {
				unlikely = false // a wrapper around the article itself
			}
			if dropTags[c.DataAtom] || c.DataAtom == atom.Header && !reMaybe.MatchString(ci) || hidden || unlikely ||
				strings.EqualFold(attr(c, "role"), "navigation") || strings.EqualFold(attr(c, "role"), "complementary") {
				remove(c)
			} else {
				prune(c)
			}
		}
		c = next
	}
}

func hasAttr(n *html.Node, k string) bool {
	for _, a := range n.Attr {
		if a.Key == k {
			return true
		}
	}
	return false
}

// readability returns the main text of the document ("" when nothing scores).
func readability(doc *html.Node) string {
	body := findFirst(doc, atom.Body)
	if body == nil {
		body = doc
	}
	prune(body)
	scores := map[*html.Node]float64{}
	var order []*html.Node
	add := func(n *html.Node, s float64) {
		if n == nil || n.Type != html.ElementNode {
			return
		}
		if _, ok := scores[n]; !ok {
			scores[n] = tagWeight(n.DataAtom) + classWeight(n)
			order = append(order, n)
		}
		scores[n] += s
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type != html.ElementNode {
				continue
			}
			para := c.DataAtom == atom.P || c.DataAtom == atom.Pre || c.DataAtom == atom.Td || c.DataAtom == atom.Blockquote ||
				((c.DataAtom == atom.Div || c.DataAtom == atom.Section || c.DataAtom == atom.Span) && !hasBlockChild(c))
			if para {
				t := innerText(c)
				rc := utf8.RuneCountInString(t)
				cjk := len(cjkRe.FindAllStringIndex(t, -1))
				if rc >= 25 || cjk >= 12 {
					s := 1 + float64(len(reCommas.FindAllStringIndex(t, -1)))
					if cjk > rc/3 {
						s += math.Min(float64(rc)/50, 3)
					} else {
						s += math.Min(float64(rc)/100, 3)
					}
					add(c.Parent, s)
					if c.Parent != nil {
						add(c.Parent.Parent, s/2)
						if c.Parent.Parent != nil {
							add(c.Parent.Parent.Parent, s/3)
						}
					}
				}
				if c.DataAtom == atom.P || c.DataAtom == atom.Pre {
					continue
				}
			}
			walk(c)
		}
	}
	walk(body)
	var cands []scored
	for _, n := range order {
		t := innerText(n)
		final := scores[n] * (1 - linkDensity(n, utf8.RuneCountInString(t)))
		cands = append(cands, scored{n, final})
	}
	if len(cands) == 0 {
		return ""
	}
	sort.SliceStable(cands, func(i, j int) bool { return cands[i].score > cands[j].score })
	top := cands[0]
	// a parent that holds several strong candidates (article split into blocks) is the real article
	for p := top.n.Parent; p != nil && p != body && p.Type == html.ElementNode; p = p.Parent {
		strong := 0
		for _, c := range cands[1:min(len(cands), 6)] {
			if c.score >= top.score*0.75 && isAncestor(p, c.n) && !isAncestor(top.n, c.n) {
				strong++
			}
		}
		if strong >= 2 {
			top = scored{p, top.score}
		}
		break
	}
	threshold := math.Max(10, top.score*0.2)
	var parts []string
	emit := func(n *html.Node) {
		if t := renderText(n); t != "" {
			parts = append(parts, t)
		}
	}
	if top.n.Parent == nil {
		emit(top.n)
	} else {
		for s := top.n.Parent.FirstChild; s != nil; s = s.NextSibling {
			if s == top.n {
				emit(s)
				continue
			}
			if s.Type != html.ElementNode {
				continue
			}
			bonus := 0.0
			if strings.TrimSpace(attr(s, "class")) != "" && attr(s, "class") == attr(top.n, "class") {
				bonus = top.score * 0.2
			}
			if sc, ok := scores[s]; ok && sc*(1-linkDensity(s, utf8.RuneCountInString(innerText(s))))+bonus >= threshold {
				emit(s)
				continue
			}
			if s.DataAtom == atom.P {
				t := innerText(s)
				rc := utf8.RuneCountInString(t)
				ld := linkDensity(s, rc)
				if (rc > 80 && ld < 0.25) || (rc > 0 && rc <= 80 && ld == 0 && endsSentence(t)) {
					emit(s)
				}
			}
		}
	}
	textA := strings.TrimSpace(strings.Join(parts, "\n\n"))
	// fragmented layouts (every paragraph in its own wrapper, scores spread thin) can make a teaser card win:
	// also take the deepest node that holds most of the page's paragraph text and keep it when it says much more
	if b := massNode(body); b != nil && b != top.n {
		if textB := renderText(b); utf8.RuneCountInString(textB) > utf8.RuneCountInString(textA)*3/2 && linkDensity(b, utf8.RuneCountInString(innerText(b))) < 0.35 {
			return textB
		}
	}
	return textA
}

// massNode: descend from root into the child carrying ≥ 75 % of the paragraph text until no child does.
func massNode(root *html.Node) *html.Node {
	mass := map[*html.Node]float64{}
	var calc func(n *html.Node) float64
	calc = func(n *html.Node) float64 {
		if n.Type != html.ElementNode {
			return 0
		}
		m := 0.0
		if n.DataAtom == atom.P || n.DataAtom == atom.Pre || n.DataAtom == atom.Blockquote || n.DataAtom == atom.Li ||
			((n.DataAtom == atom.Div || n.DataAtom == atom.Section) && !hasBlockChild(n)) {
			t := innerText(n)
			rc := utf8.RuneCountInString(t)
			if rc >= 25 || len(cjkRe.FindAllStringIndex(t, -1)) >= 12 {
				m = float64(rc) * (1 - linkDensity(n, rc))
			}
		} else {
			for c := n.FirstChild; c != nil; c = c.NextSibling {
				m += calc(c)
			}
		}
		mass[n] = m
		return m
	}
	total := calc(root)
	if total < float64(articleFullMin) {
		return nil
	}
	n := root
	for {
		var best *html.Node
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			if c.Type == html.ElementNode && mass[c] >= total*0.75 && (best == nil || mass[c] > mass[best]) {
				best = c
			}
		}
		if best == nil {
			return n
		}
		n = best
	}
}

func endsSentence(t string) bool {
	r := []rune(strings.TrimSpace(t))
	if len(r) == 0 {
		return false
	}
	return strings.ContainsRune(".。!！?？”\"", r[len(r)-1])
}

func isAncestor(a, n *html.Node) bool {
	for p := n.Parent; p != nil; p = p.Parent {
		if p == a {
			return true
		}
	}
	return false
}

func findFirst(n *html.Node, a atom.Atom) *html.Node {
	if n.Type == html.ElementNode && n.DataAtom == a {
		return n
	}
	for c := n.FirstChild; c != nil; c = c.NextSibling {
		if f := findFirst(c, a); f != nil {
			return f
		}
	}
	return nil
}

// renderText turns the chosen subtree into paragraphs: one per block, list items as "- …", link-heavy lists
// (related / tag lists inside the article) and tiny fragments dropped.
func renderText(n *html.Node) string {
	var paras []string
	var cur strings.Builder
	flush := func() {
		t := strings.Join(strings.Fields(cur.String()), " ")
		cur.Reset()
		if utf8.RuneCountInString(t) >= 2 && !(len(paras) > 0 && paras[len(paras)-1] == t) {
			paras = append(paras, t)
		}
	}
	var walk func(*html.Node)
	walk = func(n *html.Node) {
		switch n.Type {
		case html.TextNode:
			cur.WriteString(n.Data)
			return
		case html.ElementNode:
			if dropTags[n.DataAtom] {
				return
			}
			if n.DataAtom == atom.Ul || n.DataAtom == atom.Ol {
				t := innerText(n)
				if linkDensity(n, utf8.RuneCountInString(t)) > 0.5 {
					return
				}
			}
			if n.DataAtom == atom.Br {
				flush()
				return
			}
		}
		block := n.Type == html.ElementNode && blockTags[n.DataAtom]
		if block {
			flush()
			if n.DataAtom == atom.Li {
				cur.WriteString("- ")
			}
		}
		for c := n.FirstChild; c != nil; c = c.NextSibling {
			walk(c)
		}
		if block {
			flush()
		}
	}
	walk(n)
	flush()
	var out []string
	for _, p := range paras {
		rc := utf8.RuneCountInString(p)
		if p == "-" || p == "- " || (rc < 60 && reByline.MatchString(p)) || (!endsSentence(p) && rc < 12 && len(cjkRe.FindAllStringIndex(p, -1)) < 6) {
			continue // bylines, captions, buttons, stray labels
		}
		out = append(out, p)
	}
	return strings.Join(out, "\n\n")
}
