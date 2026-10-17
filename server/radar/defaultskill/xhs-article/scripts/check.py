#!/usr/bin/env python3
"""Mechanical self-check for the AI News Radar article skills (the judgement-based half is the model's job).

Usage: check.py twitter|wechat|xhs|brief FILE   (FILE = the Markdown output; "-" reads stdin)
Prints one line per problem; exit status 1 when there are any. Mirrors the server-side check in AI News Radar.
"""
import re
import sys

BANNED = [
    "在当今", "随着科技的", "随着人工智能的", "随着AI的", "近年来，", "众所周知", "本文将", "本期简报", "让我们一起", "让我们来看看", "今天给大家",
    "值得一提的是", "值得注意的是", "不难发现", "毋庸置疑", "不可否认", "显而易见",
    "综上所述", "总而言之", "总的来说", "总之，", "让我们拭目以待", "拭目以待", "未来可期", "敬请期待", "写在最后", "未来已来",
    "重磅", "炸裂", "震撼", "颠覆性", "革命性", "里程碑式", "史诗级", "王炸", "遥遥领先", "引爆", "刷屏", "火出圈", "封神", "天花板", "杀疯了",
    "赋能", "助力", "全方位", "多维度", "深度融合", "持续发力", "开启新篇章", "注入新动能", "迈出了重要一步", "具有重要意义", "凸显了", "彰显了",
    "作为一个AI", "作为AI", "以下是改写", "希望对你有帮助", "希望这篇", "如有疑问",
    "全网最", "史上最", "绝绝子", "yyds", "闭眼入", "无脑冲", "必入", "100%有效",
]
NOT_BUT = re.compile(r"不是[^。！？\n]{1,30}而是")
CONNECTIVE = re.compile(r"此外|同时，|然而|另外，|与此同时|因此，|并且")
ADVERB_SAY = re.compile(r"[\u4e00-\u9fff]{2}地(表示|指出|强调|说|宣布|透露)")
BOLD = re.compile(r"\*\*[^*\n]+\*\*")
HASHTAG = re.compile(r"(?:^|[\s\u3000])#[^\s#]{1,30}")
TWEET = re.compile(r"(?m)^\s*(\d{1,2})/\s*")
URL = re.compile(r"https?://\S+")
MDLINK = re.compile(r"\[([^\]]*)\]\((https?://[^)\s]+)\)")
H1 = re.compile(r"(?m)^#\s+(.+)$")
H2 = re.compile(r"(?m)^##\s+(.+)$")


def emoji_count(s):
    return sum(1 for ch in s if 0x1F300 <= ord(ch) <= 0x1FAFF or 0x2600 <= ord(ch) <= 0x27BF or ord(ch) in (0x2B50, 0x2705))


def x_weight(s):
    """X's weighted length: CJK / full-width / emoji count 2, a URL 23."""
    s = MDLINK.sub(r"\1 \2", s)
    n = 23 * len(URL.findall(s))
    for ch in URL.sub("", s):
        o = ord(ch)
        n += 1 if (o <= 0x10FF or 0x2000 <= o <= 0x200D or 0x2010 <= o <= 0x201F or 0x2032 <= o <= 0x2037) else 2
    return n


def check(md, platform):
    body = md.split("\n## 原文链接")[0]
    out = []
    flat = body.replace(" ", "")
    hits = [p for p in BANNED if p.replace(" ", "") in flat]
    if hits:
        out.append("禁用词句：" + "、".join(hits))
    if len(NOT_BUT.findall(body)) > 1:
        out.append("「不是……而是……」超过 1 次")
    if len(CONNECTIVE.findall(body)) > 3:
        out.append("连接词超过 3 个")
    if ADVERB_SAY.search(body):
        out.append("说话动词前的「X地」：" + "、".join(sorted({m.group(0) for m in ADVERB_SAY.finditer(body)})))
    if platform == "wechat":
        m = H1.search(body)
        if not m:
            out.append("缺少 # 标题")
        elif len(m.group(1).strip()) > 30:
            out.append(f"标题 {len(m.group(1).strip())} 字（≤30，最好 15–25）")
        if len(H2.findall(body)) > 7:
            out.append("小标题超过 7 个")
        if len(BOLD.findall(body)) > 5:
            out.append("加粗超过 5 处")
        if emoji_count(body):
            out.append("公众号正文不用 emoji")
    elif platform == "xhs":
        title = body.strip().split("\n")[0].lstrip("# ").strip()
        if len(title) > 20:
            out.append(f"标题 {len(title)} 字（≤20）")
        if len(MDLINK.sub(r"\1", body).strip()) > 1000:
            out.append("正文超过 1000 字")
        tags = len(HASHTAG.findall(body))
        if tags == 0 or tags > 6:
            out.append(f"话题 {tags} 个（3–5）")
        if emoji_count(body) > 10:
            out.append("emoji 超过 8 个")
    elif platform == "twitter":
        locs = [m.start() for m in TWEET.finditer(body)] + [len(body)]
        for i in range(len(locs) - 1):
            w = x_weight(body[locs[i]:locs[i + 1]].strip())
            if w > 280:
                out.append(f"第 {i + 1} 条推文加权长度 {w}（≤280）")
        if len(locs) - 1 > 12:
            out.append("串推超过 12 条")
        if len(HASHTAG.findall(body)) > 2:
            out.append("话题标签超过 2 个")
        if BOLD.search(body):
            out.append("推文不支持加粗")
    return out


if __name__ == "__main__":
    if len(sys.argv) != 3 or sys.argv[1] not in ("twitter", "wechat", "xhs", "brief"):
        sys.exit(__doc__)
    text = sys.stdin.read() if sys.argv[2] == "-" else open(sys.argv[2], encoding="utf-8").read()
    problems = check(text, sys.argv[1])
    for p in problems:
        print(p)
    sys.exit(1 if problems else 0)
