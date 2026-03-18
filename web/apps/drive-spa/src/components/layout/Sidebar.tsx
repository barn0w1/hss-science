import { cn } from "@/lib/utils"
import { useUIStore } from "@/store/ui.store"
import { SidebarNav } from "./SidebarNav"
import { SidebarSpaceSwitcher } from "./SidebarSpaceSwitcher"
import { PanelLeft } from "lucide-react"
import { Button } from "@/components/ui/button"

export function Sidebar() {
  const isSidebarCollapsed = useUIStore((s) => s.isSidebarCollapsed)
  const toggleSidebar = useUIStore((s) => s.toggleSidebar)

  return (
    <>
      {!isSidebarCollapsed && (
        <div
          className="fixed inset-0 z-20 bg-black/50 md:hidden"
          onClick={toggleSidebar}
        />
      )}
      <aside
        className={cn(
          "flex h-full flex-col border-r border-border bg-sidebar transition-all duration-200",
          isSidebarCollapsed
            ? "fixed -translate-x-full md:relative md:w-0 md:translate-x-0 md:overflow-hidden md:border-0"
            : "fixed z-30 w-60 md:relative md:z-auto",
        )}
      >
        <div className="flex h-14 items-center gap-2 border-b border-border px-4">
          <div className="flex items-center gap-2">
            <div className="flex h-7 w-7 items-center justify-center rounded-lg bg-primary">
              <span className="text-xs font-bold text-primary-foreground">D</span>
            </div>
            <span className="font-semibold text-sm tracking-tight">Drive</span>
          </div>
          <Button
            variant="ghost"
            size="icon-sm"
            className="ml-auto"
            onClick={toggleSidebar}
          >
            <PanelLeft className="h-4 w-4" />
          </Button>
        </div>
        <div className="flex flex-1 flex-col overflow-y-auto py-2">
          <SidebarNav />
          <div className="my-2 h-px bg-border" />
          <SidebarSpaceSwitcher />
        </div>
      </aside>
    </>
  )
}
