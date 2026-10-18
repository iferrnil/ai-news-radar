"use client";
// Section icon picker: brand marks + line icons (icon-lib.tsx) and emoji, in categories, with zh/en keyword search and
// recents. Code-split: only the trigger lives in the dialogs; this panel (and the icon library) loads on first open.
import { useState } from "react";
import { LIB, LIB_CATS } from "./icon-lib";
import { SectionIcon } from "./section-icon";
import { Search, X } from "lucide-react";
import { Button } from "@/components/ui/button";
import { Input } from "@/components/ui/input";
import { cn } from "@/lib/utils";

type E = [emoji: string, keywords: string];
const EMOJI: { k: string; zh: string; list: E[] }[] = [
  { k: "ai", zh: "AI / 科技", list: [["🤖", "机器人 robot ai 模型"], ["🧠", "大脑 brain 智能 llm"], ["✨", "闪光 sparkle 生成 gen"], ["⚡", "闪电 fast 快讯"], ["💡", "灵感 idea"], ["🔬", "研究 research 实验"], ["🧪", "实验 lab test"], ["🛰️", "卫星 satellite"], ["🚀", "火箭 launch 发布"], ["💻", "电脑 code dev 开发"], ["🖥️", "桌面 desktop"], ["📱", "手机 mobile app"], ["⌨️", "键盘 keyboard"], ["🔌", "插件 plugin mcp"], ["🧩", "拼图 plugin 组件"], ["🛠️", "工具 tools"], ["⚙️", "设置 gear infra"], ["🔧", "扳手 wrench"], ["🧬", "基因 bio"], ["🔭", "望远镜 观察"]] },
  { k: "media", zh: "视频 / 图像", list: [["🎬", "视频 video 电影 剪辑"], ["🎥", "摄像 camera video"], ["📹", "录像 video"], ["🎞️", "胶片 film"], ["📺", "电视 tv"], ["🎨", "设计 art 绘画 design"], ["🖌️", "画笔 brush 插画"], ["🖼️", "图片 image 图像"], ["📷", "相机 photo"], ["🎭", "表演 drama"], ["🎵", "音乐 music audio"], ["🎙️", "播客 podcast 语音"], ["🎧", "耳机 audio"], ["🕹️", "游戏 game"], ["🎮", "游戏 game"], ["🧑‍🎨", "设计师 designer"], ["✏️", "铅笔 ui 草图"], ["📐", "尺子 ui layout"], ["🌈", "色彩 color"], ["🪄", "魔法 magic"]] },
  { k: "news", zh: "资讯 / 写作", list: [["📰", "新闻 news"], ["🗞️", "报纸 newspaper"], ["📣", "公告 announce"], ["📢", "广播 broadcast"], ["🔥", "热门 hot trending"], ["📈", "增长 trend 数据"], ["📊", "图表 chart"], ["💬", "评论 chat 社区"], ["🗨️", "讨论 discuss"], ["✍️", "写作 writing"], ["📝", "笔记 note"], ["📚", "书 books 论文 paper"], ["📖", "阅读 read"], ["🔖", "书签 bookmark"], ["📌", "置顶 pin"], ["🏷️", "标签 tag"], ["📬", "邮件 newsletter"], ["🌐", "全球 global web"], ["🇨🇳", "中国 china 国内"], ["🌏", "亚洲 asia"]] },
  { k: "biz", zh: "商业 / 安全", list: [["💼", "商业 business"], ["💰", "钱 money 融资"], ["💸", "投资 invest"], ["🏢", "公司 company"], ["🛒", "电商 shop"], ["📦", "产品 product 发布"], ["🎯", "目标 target"], ["🏆", "榜单 rank"], ["🔒", "安全 security"], ["🛡️", "防护 shield"], ["🕵️", "逆向 reverse 侦探"], ["🐛", "漏洞 bug"], ["🔓", "破解 unlock"], ["🧰", "工具箱 toolbox"], ["⚖️", "法律 law 政策"], ["🏛️", "政府 policy"]] },
  { k: "fun", zh: "娱乐 / 其他", list: [["😂", "笑 搞笑 meme 段子"], ["🍉", "吃瓜 八卦 gossip"], ["🤡", "小丑 meme"], ["🐱", "猫 cat"], ["🐶", "狗 dog"], ["🦄", "独角兽 unicorn"], ["🌟", "星 star"], ["❤️", "喜欢 love"], ["⭐", "收藏 favorite"], ["🍿", "爆米花 drama"], ["☕", "咖啡 coffee"], ["🌙", "夜 night"], ["🌱", "成长 grow"], ["🎉", "庆祝 party"], ["👀", "围观 watch"], ["🧭", "导航 compass"]] },
];
type Entry = { v: string; kw: string; label: string };
const ent = (v: string, kw: string): Entry => ({ v, kw, label: kw.split(" ")[0] });
// one tab per category: its line icons (and brand marks) first, then its emoji
const CATS: { k: string; zh: string; list: Entry[] }[] = [
  { k: "brand", zh: "品牌 / 平台", list: LIB_CATS.brand.map(([n, kw]) => ent("lucide:" + n, kw)) },
  ...EMOJI.map((c) => ({ k: c.k, zh: c.zh, list: [...(LIB_CATS[c.k] ?? []).map(([n, kw]) => ent("lucide:" + n, kw)), ...c.list.map(([e, kw]) => ent(e, kw))] })),
];
const ALL = CATS.flatMap((c) => c.list);
const RECENT_KEY = "iconPicker.recent";
const readRecent = (): string[] => { try { return JSON.parse(localStorage.getItem(RECENT_KEY) || "[]"); } catch { return []; } };
const known = (v: string) => !v.startsWith("lucide:") || !!LIB[v.slice(7)];

export default function IconPickerPanel({ value, onPick }: { value: string; onPick: (v: string) => void }) {
  const [q, setQ] = useState(""); const [cat, setCat] = useState(CATS[0].k);
  const [recent] = useState(() => readRecent().filter(known));
  const ql = q.trim().toLowerCase();
  const list: Entry[] = ql ? ALL.filter((x) => x.kw.toLowerCase().includes(ql) || x.v === ql) : CATS.find((c) => c.k === cat)!.list;
  const pick = (v: string) => {
    try { localStorage.setItem(RECENT_KEY, JSON.stringify([v, ...readRecent().filter((x) => x !== v)].slice(0, 9))); } catch {}
    onPick(v);
  };
  const cell = (v: string, label: string, key = v) => {
    const icon = v.startsWith("lucide:");
    return (
      <button key={key} type="button" title={label} aria-label={`图标 ${icon ? v.slice(7) : v} ${label}`} aria-pressed={value === v} onClick={() => pick(v)}
        className={cn("grid size-8 place-items-center rounded-md text-[18px] leading-none text-foreground/85 outline-none transition-colors duration-150 hover:bg-hover hover:text-foreground focus-visible:ring-3 focus-visible:ring-ring/50", value === v && "bg-brand/15 text-foreground ring-1 ring-brand/50")}>
        {icon ? (() => { const C = LIB[v.slice(7)] ?? LIB.x; return <C aria-hidden className="size-[18px]" />; })() : v}
      </button>
    );
  };
  return (
    <div className="grid w-[320px] max-w-[calc(100vw-2rem)] gap-2" data-testid="icon-picker">
      <div className="relative">
        <Search className="pointer-events-none absolute left-2.5 top-1/2 size-3.5 -translate-y-1/2 text-faint" />
        <Input autoFocus aria-label="搜索图标" placeholder="搜索：github、视频、ai…" className="h-8 pl-8" value={q} onChange={(e) => setQ(e.target.value)} />
      </div>
      {!ql && (
        <div className="flex flex-wrap gap-1" role="tablist" aria-label="图标分类">
          {CATS.map((c) => (
            <button key={c.k} type="button" role="tab" aria-selected={cat === c.k} onClick={() => setCat(c.k)}
              className={cn("h-6 rounded-md px-2 type-meta transition-colors duration-150", cat === c.k ? "bg-muted text-foreground" : "text-muted-foreground hover:text-foreground")}>{c.zh}</button>
          ))}
        </div>
      )}
      {!ql && recent.length > 0 && (
        <div><div className="mb-1 type-meta text-faint">最近使用</div><div className="flex flex-wrap gap-0.5">{recent.map((v) => cell(v, "最近", "r:" + v))}</div></div>
      )}
      <div className="grid max-h-[216px] grid-cols-9 gap-0.5 overflow-y-auto" data-testid="icon-grid">
        {list.map((x) => cell(x.v, x.label))}
        {!list.length && <div className="col-span-9 py-4 text-center type-caption text-faint">没有匹配的图标</div>}
      </div>
      <div className="flex items-center justify-between border-t pt-2">
        <span className="flex items-center gap-2 type-caption text-muted-foreground">预览 <span className="grid size-7 place-items-center rounded-md border text-[16px]">{value ? <SectionIcon icon={value} /> : "—"}</span></span>
        <Button type="button" variant="ghost" size="sm" disabled={!value} onClick={() => onPick("")}><X />清除图标</Button>
      </div>
    </div>
  );
}
