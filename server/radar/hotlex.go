package radar

import (
	"fmt"
	"hash/fnv"
	"regexp"
	"sort"
	"strings"
	"sync"
	"unicode"
	"unicode/utf8"
)

// 热点 name lexicon: Chinese AI companies / products and their English aliases, so 「字节跳动」 and "ByteDance",
// 「通义千问3.5」 and "Qwen 3.5" become the same name token. Chinese text is segmented only by longest match against
// this lexicon (no dictionary-based word segmentation: gse's dictionary costs ~1 s and ~170 MB of heap to load, far
// beyond the app's idle footprint, and the hot rules only need names). Unknown Chinese words are ignored; Latin
// names inside Chinese text ("Meta杀回开源", "Kling 4.0杀疯了") are tokenized as usual.
//
// One entry per line: `id[*] alias alias …` — id is the canonical token (lowercase Latin), * marks a company / big
// model family (orgs support a match but never make one on their own; see hotIdx.distinctive). Aliases are Chinese
// (matched as substrings, longest first) or Latin words (matched as whole tokens).
const hotLexBuiltin = `
bytedance* 字节跳动 字节 bytedance 火山引擎 volcengine
doubao 豆包 doubao
seedance 即梦 seedance jimeng
alibaba* 阿里巴巴 阿里 阿里云 alibaba aliyun
qwen* 通义千问 千问 通义 qwen tongyi
moonshot* 月之暗面 moonshot kimi
zhipu* 智谱 智谱清言 zhipu glm chatglm
baidu* 百度 baidu
ernie 文心 文心一言 文心大模型 ernie
tencent* 腾讯 tencent
hunyuan 混元 腾讯混元 hunyuan
yuanbao 元宝 腾讯元宝 yuanbao
huawei* 华为 huawei
pangu 盘古 盘古大模型 pangu
ascend 昇腾 ascend
sensetime* 商汤 商汤科技 sensetime
sensenova 日日新 sensenova
minimax* 稀宇科技 稀宇 minimax
hailuo 海螺 海螺ai hailuo
stepfun* 阶跃星辰 阶跃 stepfun
01ai* 零一万物 01ai
kuaishou* 快手 kuaishou
kling 可灵 kling
deepseek* 深度求索 deepseek
xiaomi* 小米 xiaomi
mimo mimo
iflytek* 科大讯飞 讯飞 iflytek
xinghuo 星火 讯飞星火 xinghuo
antgroup* 蚂蚁集团 蚂蚁 antgroup
ling 百灵 ling
kunlun* 昆仑万维 kunlun
skywork 天工 skywork
jd* 京东 jd
meituan* 美团 meituan
unitree* 宇树 宇树科技 unitree
ubtech* 优必选 ubtech
openai* openai
chatgpt* chatgpt
anthropic* anthropic
claude* 克劳德 claude
google* 谷歌 google
deepmind* deepmind
gemini* gemini
microsoft* 微软 microsoft
nvidia* 英伟达 nvidia
apple* 苹果 apple
meta* meta
amazon* 亚马逊 amazon aws
tesla* 特斯拉 tesla
spacex* spacex
samsung* 三星 samsung
intel* 英特尔 intel
`

// version families: "GPT 6", "GPT-6" and "gpt6" are the same token gpt6 (a bare "X 6" is joined only for these;
// "word-N" and "word N.N" are joined for every word, see hotDashNum)
const hotFamilies = `gpt claude gemini llama qwen glm kimi ernie grok sora veo kling doubao hunyuan minimax mimo opus sonnet haiku
	mistral phi gemma seed seedance hailuo pangu xinghuo yuanbao skywork ling`

type hotLex struct {
	han     map[string]string // Chinese alias → id
	hanMax  int               // longest Chinese alias (runes)
	latin   map[string]string // Latin alias → id
	org     map[string]bool
	user    map[string]bool // ids from the user's lexicon (always distinctive when rare)
	family  *regexp.Regexp
	lexText string // the user lexicon this was built from
	memoMu  sync.Mutex
	memo    map[string]hotScan // scan() results
}

// parseUserLexicon: one name per line, aliases joined by "=" (also ，, / 、 / tab): 「元宝=Yuanbao=腾讯元宝」.
// Returns the normalized lines (for storage) — at most 200 lines of ≤ 8 names of ≤ 30 characters.
func parseUserLexicon(text string) [][]string {
	var out [][]string
	seen := map[string]bool{} // a name belongs to its first line
	for _, line := range strings.Split(strings.ReplaceAll(text, "\r", ""), "\n") {
		var names []string
		for _, n := range strings.FieldsFunc(line, func(r rune) bool { return r == '=' || r == '＝' || r == ',' || r == '，' || r == '、' || r == '\t' }) {
			n = strings.Join(strings.Fields(n), " ")
			if n == "" || utf8.RuneCountInString(n) > 30 || seen[strings.ToLower(n)] || len(names) >= 8 {
				continue
			}
			seen[strings.ToLower(n)] = true
			names = append(names, n)
		}
		if len(names) > 0 && len(out) < 200 {
			out = append(out, names)
		}
	}
	return out
}

func userLexiconText(groups [][]string) string {
	var b strings.Builder
	for _, g := range groups {
		b.WriteString(strings.Join(g, "="))
		b.WriteByte('\n')
	}
	return strings.TrimRight(b.String(), "\n")
}

var hotLatinWord = regexp.MustCompile(`^[a-z0-9][a-z0-9.+]*$`)

func newHotLex(user string) *hotLex {
	x := &hotLex{han: map[string]string{}, latin: map[string]string{}, org: map[string]bool{}, user: map[string]bool{}, lexText: user}
	addHan := func(a, id string) {
		x.han[a] = id
		x.hanMax = max(x.hanMax, utf8.RuneCountInString(a))
	}
	fam := strings.Fields(hotFamilies)
	for _, line := range strings.Split(hotLexBuiltin, "\n") {
		f := strings.Fields(line)
		if len(f) == 0 {
			continue
		}
		id := f[0]
		if strings.HasSuffix(id, "*") {
			id = strings.TrimSuffix(id, "*")
			x.org[id] = true
		}
		for _, a := range f[1:] {
			if hasCJK(a) {
				addHan(strings.ToLower(a), id)
			} else if a != id {
				x.latin[a] = id
			}
		}
	}
	for _, g := range parseUserLexicon(user) {
		id := ""
		for _, n := range g { // a single Latin word names the entry; otherwise a stable synthetic id
			if l := strings.ToLower(n); hotLatinWord.MatchString(l) && !hotStop[l] {
				id = l
				break
			}
		}
		if id == "" {
			h := fnv.New32a()
			h.Write([]byte(g[0]))
			id = fmt.Sprintf("u%08x", h.Sum32())
		}
		if x.latin[id] != "" { // the user's word is a known alias ("Kimi"): extend that entry
			id = x.latin[id]
		}
		x.user[id] = !x.org[id]
		for _, n := range g {
			l := strings.ToLower(n)
			switch {
			case hasCJK(l):
				addHan(l, id)
			case hotLatinWord.MatchString(l) && l != id:
				x.latin[l] = id
			}
		}
	}
	for id := range x.user {
		if hotLatinWord.MatchString(id) && !strings.ContainsAny(id, "0123456789") {
			fam = append(fam, id)
		}
	}
	sort.Slice(fam, func(i, j int) bool { return len(fam[i]) > len(fam[j]) })
	x.family = regexp.MustCompile(`(?i)\b(` + strings.Join(fam, "|") + `)[ -]?(\d+(?:\.\d+)*[a-z]?)\b`)
	return x
}

// hanReplace: longest-match lexicon names in Chinese runs become their Latin id ("可灵4.0杀疯了" → " kling4.0杀疯了");
// returns the ids found.
func (x *hotLex) hanReplace(s string) (string, []string) {
	if x.hanMax == 0 || !hasCJK(s) {
		return s, nil
	}
	rs := []rune(s)
	var b strings.Builder
	var hits []string
	for i := 0; i < len(rs); {
		if unicode.Is(unicode.Han, rs[i]) {
			matched := false
			for n := min(x.hanMax, len(rs)-i); n >= 1; n-- {
				if id, ok := x.han[strings.ToLower(string(rs[i:i+n]))]; ok {
					b.WriteByte(' ') // own token: 「阿里通义千问」 → " alibaba qwen"; a version right after stays attached
					b.WriteString(id)
					if j := i + n; j < len(rs) && rs[j] < utf8.RuneSelf && unicode.IsLetter(rs[j]) {
						b.WriteByte(' ')
					}
					hits = append(hits, id)
					i += n
					matched = true
					break
				}
			}
			if matched {
				continue
			}
		}
		b.WriteRune(rs[i])
		i++
	}
	return b.String(), hits
}

// the lexicon in use: built-in + the user's (setting hot_lexicon), rebuilt when the setting changes
var hotLexCur struct {
	sync.Mutex
	lex *hotLex
}

func (s *Store) hotLexicon() *hotLex {
	user, _ := s.GetSetting("hot_lexicon")
	hotLexCur.Lock()
	defer hotLexCur.Unlock()
	if hotLexCur.lex == nil || hotLexCur.lex.lexText != user {
		hotLexCur.lex = newHotLex(user)
	}
	return hotLexCur.lex
}

var hotLexDefault = sync.OnceValue(func() *hotLex { return newHotLex("") })
