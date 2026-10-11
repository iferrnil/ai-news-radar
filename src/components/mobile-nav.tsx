"use client";
import { Sheet, SheetContent, SheetTitle } from "@/components/ui/sheet";
import { SidebarBody } from "./app-shell";

// Mobile navigation sheet, code-split: mounted after the menu button is first tapped.
export default function MobileNav({ open, onOpenChange }: { open: boolean; onOpenChange: (b: boolean) => void }) {
  return (
    <Sheet open={open} onOpenChange={onOpenChange}>
      <SheetContent side="left" className="w-[270px] bg-sidebar p-0" showCloseButton={false}>
        <SheetTitle className="sr-only">导航</SheetTitle><SidebarBody onNavigate={() => onOpenChange(false)} />
      </SheetContent>
    </Sheet>
  );
}
