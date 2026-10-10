import Link from "next/link";
import { ArrowUpRight, BookOpen, FileText } from "lucide-react";
import { DOCS, docHref } from "@/lib/docs-registry";

export const metadata = { title: "文档 · AI Radar" };

export default function DocsIndex() {
  return (
    <div className="mx-auto w-full max-w-[920px] px-4 pb-24 pt-8 sm:px-6">
      <div className="flex items-start gap-3">
        <div className="grid size-9 shrink-0 place-items-center rounded-lg bg-brand-soft text-brand"><BookOpen className="size-4" strokeWidth={1.75} /></div>
        <div>
          <h1 className="type-h1 font-semibold leading-8 tracking-[-0.015em]">文档</h1>
          <p className="max-w-[60ch] type-body leading-5 text-muted-foreground">项目运行与数据源相关的说明。新增页面只需在 docs-registry 中登记。</p>
        </div>
      </div>
      <div className="mt-8 grid gap-3 sm:grid-cols-2">
        {DOCS.map((d) => (
          <Link key={d.slug} href={docHref(d)} className="group rounded-xl bg-card p-4 shadow-card transition-[background-color,box-shadow] duration-150 ease-out hover:bg-hover">
            <div className="flex items-center gap-2">
              <FileText className="size-4 text-faint" strokeWidth={1.75} />
              <span className="type-ui font-medium tracking-[-0.01em]">{d.title}</span>
              <ArrowUpRight className="ml-auto size-4 text-faint transition-transform duration-150 ease-out group-hover:-translate-y-0.5 group-hover:translate-x-0.5 group-hover:text-foreground" strokeWidth={1.75} />
            </div>
            <p className="mt-2 type-body leading-5 text-muted-foreground">{d.desc}</p>
            <div className="mt-3 font-mono type-meta tabular-nums text-faint">更新于 {d.updated}</div>
          </Link>
        ))}
      </div>
    </div>
  );
}
