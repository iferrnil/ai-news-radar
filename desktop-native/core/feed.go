package core

import (
	"bytes"
	"encoding/xml"
	"io"
	"net/url"
	"regexp"
	"strings"
)

// node is a tiny generic XML tree (local names only, like fast-xml-parser with prefixes collapsed).
type node struct {
	Name     string // local name; prefixed names keep "prefix:local" for dc:/content: (see key)
	Attr     map[string]string
	Text     string
	Children []*node
}

func (n *node) all(name string) []*node {
	var out []*node
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Name == name {
			out = append(out, c)
		}
	}
	return out
}
func (n *node) first(name string) *node {
	if n == nil {
		return nil
	}
	for _, c := range n.Children {
		if c.Name == name {
			return c
		}
	}
	return nil
}
func (n *node) text(name string) string {
	c := n.first(name)
	if c == nil {
		return ""
	}
	return strings.TrimSpace(c.Text)
}

var nsPrefix = map[string]string{
	"http://purl.org/dc/elements/1.1/":            "dc",
	"http://purl.org/rss/1.0/modules/content/":    "content",
	"http://www.w3.org/1999/02/22-rdf-syntax-ns#": "rdf",
}

func parseXML(b []byte) (*node, error) {
	d := xml.NewDecoder(bytes.NewReader(b))
	d.Strict = false
	d.Entity = xml.HTMLEntity
	d.CharsetReader = func(_ string, r io.Reader) (io.Reader, error) { return r, nil }
	root := &node{}
	stack := []*node{root}
	for {
		tok, err := d.Token()
		if err == io.EOF {
			break
		}
		if err != nil {
			if len(root.Children) > 0 {
				break // keep what parsed
			}
			return nil, err
		}
		switch t := tok.(type) {
		case xml.StartElement:
			name := t.Name.Local
			if p, ok := nsPrefix[t.Name.Space]; ok {
				name = p + ":" + name
			} else if t.Name.Space != "" && !strings.Contains(t.Name.Space, "/") {
				name = t.Name.Space + ":" + name // undeclared prefix
			}
			n := &node{Name: name, Attr: map[string]string{}}
			for _, a := range t.Attr {
				n.Attr[a.Name.Local] = a.Value
			}
			top := stack[len(stack)-1]
			top.Children = append(top.Children, n)
			stack = append(stack, n)
		case xml.EndElement:
			if len(stack) > 1 {
				stack = stack[:len(stack)-1]
			}
		case xml.CharData:
			stack[len(stack)-1].Text += string(t)
		}
	}
	return root, nil
}

// FetchedItem mirrors types.ts FetchedItem. Published == 0 means unknown.
type FetchedItem struct {
	UID, Title, URL, Summary string
	Published                int64
	IsX                      bool
	Author, Image            string
	Links                    []string
}

type ParsedFeed struct {
	Title, Link string
	Items       []FetchedItem
}

var feedSniff = regexp.MustCompile(`(?i)<(rss|feed|rdf:RDF)[\s>]`)

func atomLink(links []*node) string {
	for _, l := range links {
		if r := l.Attr["rel"]; r == "" || r == "alternate" {
			if h := l.Attr["href"]; h != "" {
				return h
			}
			return strings.TrimSpace(l.Text)
		}
	}
	if len(links) > 0 {
		if h := links[0].Attr["href"]; h != "" {
			return h
		}
		return strings.TrimSpace(links[0].Text)
	}
	return ""
}

// ParseFeed parses RSS 2.0 / Atom / RSS 1.0. Returns nil if it isn't a feed.
func ParseFeed(body []byte) *ParsedFeed {
	head := body
	if len(head) > 5000 {
		head = head[:5000]
	}
	if !feedSniff.Match(head) {
		return nil
	}
	root, err := parseXML(body)
	if err != nil {
		return nil
	}
	var title, link string
	var raw []*node
	atom := false
	if rss := root.first("rss"); rss != nil {
		ch := rss.first("channel")
		title, link, raw = ch.text("title"), ch.text("link"), ch.all("item")
	} else if f := root.first("feed"); f != nil {
		atom = true
		title, link, raw = f.text("title"), atomLink(f.all("link")), f.all("entry")
	} else if r := root.first("rdf:RDF"); r != nil {
		ch := r.first("channel")
		title, link, raw = ch.text("title"), ch.text("link"), r.all("item")
	} else {
		return nil
	}
	if len(raw) > 60 {
		raw = raw[:60]
	}
	out := &ParsedFeed{Title: Clean(title, 100), Link: link}
	for _, e := range raw {
		var l string
		if atom {
			l = atomLink(e.all("link"))
		} else {
			l = e.text("link")
			if g := e.text("guid"); l == "" && (strings.HasPrefix(g, "http:") || strings.HasPrefix(g, "https:")) {
				l = g
			}
		}
		if l == "" {
			continue
		}
		t := Clean(e.text("title"), 300)
		if t == "" {
			t = l
		}
		body := firstNonEmpty(e.text("description"), e.text("summary"), e.text("content:encoded"), e.text("content"))
		author := ""
		if a := e.first("author"); a != nil {
			author = firstNonEmpty(a.text("name"), strings.TrimSpace(a.Text))
		}
		if author == "" {
			author = e.text("dc:creator")
		}
		out.Items = append(out.Items, FetchedItem{
			UID: NormURL(l), Title: t, URL: l, Summary: Clean(body, 280),
			Published: ParseDate(firstNonEmpty(e.text("pubDate"), e.text("published"), e.text("updated"), e.text("dc:date"))),
			Author:    author,
		})
	}
	return out
}

func firstNonEmpty(xs ...string) string {
	for _, x := range xs {
		if x != "" {
			return x
		}
	}
	return ""
}

var (
	linkTagRe  = regexp.MustCompile(`(?i)<link\b[^>]*>`)
	altRe      = regexp.MustCompile(`(?i)rel=["']?[^"'>]*alternate`)
	feedTypeRe = regexp.MustCompile(`(?i)type=["']?application/(rss|atom)\+xml|type=["']?application/feed\+json|type=["']?(text|application)/xml`)
	hrefRe     = regexp.MustCompile(`(?i)href=["']([^"']+)["']`)
)

// DiscoverFeeds finds <link rel="alternate" type="application/rss+xml"> URLs.
func DiscoverFeeds(htmlBody, base string) []string {
	var out []string
	b, _ := url.Parse(base)
	for _, tag := range linkTagRe.FindAllString(htmlBody, -1) {
		if !altRe.MatchString(tag) || !feedTypeRe.MatchString(tag) {
			continue
		}
		m := hrefRe.FindStringSubmatch(tag)
		if m == nil {
			continue
		}
		u, err := url.Parse(strings.ReplaceAll(m[1], "&amp;", "&"))
		if err != nil {
			continue
		}
		if b != nil {
			u = b.ResolveReference(u)
		}
		out = append(out, u.String())
	}
	return out
}
