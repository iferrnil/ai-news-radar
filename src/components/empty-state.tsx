import { Empty, EmptyContent, EmptyDescription, EmptyHeader, EmptyMedia, EmptyTitle } from "@/components/ui/empty";
import { cn } from "@/lib/utils";

/** One pattern for empty / no-results / error states, built on shadcn <Empty>: icon tile + title + hint + optional actions. */
export function EmptyState({ icon: Icon, title, hint, children, tone = "default", className }: {
  icon: React.ElementType; title: React.ReactNode; hint?: React.ReactNode; children?: React.ReactNode; tone?: "default" | "error"; className?: string;
}) {
  return (
    <Empty role={tone === "error" ? "alert" : "status"} className={cn("gap-0 px-6 py-12", className)}>
      <EmptyHeader className="gap-0">
        <EmptyMedia variant="icon" className={cn("mb-0 size-10 border bg-card shadow-none", tone === "error" && "border-red-500/30 bg-red-500/10")}>
          <Icon className={cn("size-5", tone === "error" ? "text-red-500" : "text-muted-foreground")} />
        </EmptyMedia>
        <EmptyTitle className="mt-4 type-ui font-semibold">{title}</EmptyTitle>
        {hint && <EmptyDescription className="mt-1 type-body text-muted-foreground">{hint}</EmptyDescription>}
      </EmptyHeader>
      {children && <EmptyContent className="mt-4 flex-row flex-wrap justify-center gap-2">{children}</EmptyContent>}
    </Empty>
  );
}
