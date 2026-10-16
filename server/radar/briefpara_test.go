package radar

import (
	"strings"
	"testing"
)

func TestOnePerParagraph(t *testing.T) {
	in := "# T\n\n## A\n\nOpenAI 发布 X [[1]](https://a/1)。Google 推出 Y [[2]](https://a/2) [[3]](https://a/3)。最后一条 [[4]](https://a/4)\n\n## 原文链接\n\n1. [x](https://a/1) · s\n2. [y](https://a/2) · s\n"
	got := onePerParagraph(in)
	for _, want := range []string{"OpenAI 发布 X [[1]](https://a/1)。\n\nGoogle 推出 Y [[2]](https://a/2) [[3]](https://a/3)。\n\n最后一条 [[4]](https://a/4)\n", "1. [x](https://a/1) · s\n2. [y]"} {
		if !strings.Contains(got, want) {
			t.Fatalf("missing %q in\n%s", want, got)
		}
	}
}

func TestBriefMarkdownOneItemPerParagraph(t *testing.T) {
	b := &briefOut{Title: "T", Count: 2, Sources: 1, Groups: []briefGroup{{Source: "S", Items: []briefItem{{Title: "甲", URL: "https://a/1"}, {Title: "乙", URL: "https://a/2"}}}}}
	md := briefMarkdown(b)
	if !strings.Contains(md, "甲。 [[1]](https://a/1)\n\n乙。 [[2]](https://a/2)\n\n") {
		t.Fatal(md)
	}
}
