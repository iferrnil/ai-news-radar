"use client";
import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar";
import { cn } from "@/lib/utils";
import { avatarUrl, initial } from "./common";

const HUES = [285, 200, 160, 30, 340, 250];
export function SourceAvatar({ name, username, homepage, url, className, x }: { name: string; username?: string | null; homepage?: string | null; url?: string; className?: string; x?: boolean }) {
  const hue = HUES[[...name].reduce((a, c) => a + c.charCodeAt(0), 0) % HUES.length];
  return (
    <Avatar className={cn("size-8 shrink-0", !x && "rounded-md after:rounded-md", className)}>
      <AvatarImage src={avatarUrl({ username, homepage, url })} alt="" className={cn(!x && "rounded-md bg-white p-[3px]")} />
      <AvatarFallback className={cn("type-meta font-semibold text-white", !x && "rounded-md")} style={{ background: `oklch(0.6 0.14 ${hue})` }}>{initial(name)}</AvatarFallback>
    </Avatar>
  );
}
