"use client";
import { memo } from "react";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { cn } from "@/lib/utils";
import { avatarUrl, brandOf, initial } from "./common";
import { GithubLogo } from "./x-logo";

const HUES = [285, 200, 160, 30, 340, 250];
export const SourceAvatar = memo(function SourceAvatar({ name, username, homepage, url, className, x }: { name: string; username?: string | null; homepage?: string | null; url?: string; className?: string; x?: boolean }) {
  const hue = HUES[[...name].reduce((a, c) => a + c.charCodeAt(0), 0) % HUES.length];
  if (!username && brandOf({ homepage, url }) === "github") return (
    <span aria-hidden className={cn("grid size-8 shrink-0 place-items-center rounded-md bg-neutral-900 text-white ring-1 ring-white/10 dark:bg-neutral-100 dark:text-neutral-900", className)}>
      <GithubLogo className="size-[62%]" />
    </span>
  );
  return (
    <Avatar className={cn("size-8 shrink-0", !x && "rounded-md after:rounded-md", className)}>
      <AvatarImage src={avatarUrl({ username, homepage, url })} alt="" loading="lazy" decoding="async" className={cn(!x && "rounded-md bg-white p-[3px]")} />
      <AvatarFallback className={cn("type-meta font-semibold text-white", !x && "rounded-md")} style={{ background: `oklch(0.6 0.14 ${hue})` }}>{initial(name)}</AvatarFallback>
    </Avatar>
  );
});
