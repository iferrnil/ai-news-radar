import { cn } from "@/lib/utils";

/** One pattern for empty / no-results / error states: icon tile + title + hint + optional actions. */
export function EmptyState({ icon: Icon, title, hint, children, tone = "default", className }: {
  icon: React.ElementType; title: React.ReactNode; hint?: React.ReactNode; children?: React.ReactNode; tone?: "default" | "error"; className?: string;
}) {
  return (
    <div role={tone === "error" ? "alert" : "status"} className={cn("flex flex-col items-center px-6 py-12 text-center", className)}>
      <div className={cn("grid size-10 place-items-center rounded-lg border bg-card", tone === "error" && "border-red-500/30 bg-red-500/10")}>
        <Icon className={cn("size-5", tone === "error" ? "text-red-500" : "text-muted-foreground")} />
      </div>
      <div className="mt-4 type-ui font-semibold">{title}</div>
      {hint && <p className="mt-1 max-w-sm type-body text-muted-foreground">{hint}</p>}
      {children && <div className="mt-4 flex flex-wrap justify-center gap-2">{children}</div>}
    </div>
  );
}
