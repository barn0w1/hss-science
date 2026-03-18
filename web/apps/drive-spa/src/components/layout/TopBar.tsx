import { useEffect, useRef } from "react"
import { Search, PanelLeft, Moon, Sun, Share2 } from "lucide-react"
import { Button } from "@/components/ui/button"
import { Input } from "@/components/ui/input"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { useUIStore } from "@/store/ui.store"
import { useAuthStore } from "@/store/auth.store"
import { useDriveStore } from "@/store/drive.store"
import { useTheme } from "@/components/theme-provider"
import { DriveBreadcrumb } from "@/components/drive/Breadcrumb"

export function TopBar() {
  const searchQuery = useUIStore((s) => s.searchQuery)
  const setSearchQuery = useUIStore((s) => s.setSearchQuery)
  const isSidebarCollapsed = useUIStore((s) => s.isSidebarCollapsed)
  const toggleSidebar = useUIStore((s) => s.toggleSidebar)
  const openShareDialog = useUIStore((s) => s.openShareDialog)
  const user = useAuthStore((s) => s.user)
  const currentSpaceId = useDriveStore((s) => s.currentSpaceId)
  const { theme, setTheme } = useTheme()
  const searchRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    ;(globalThis as { focusSearch?: () => void }).focusSearch = () => {
      searchRef.current?.focus()
    }
    return () => {
      delete (globalThis as { focusSearch?: () => void }).focusSearch
    }
  }, [])

  return (
    <header className="flex h-14 shrink-0 items-center gap-3 border-b border-border px-4">
      {isSidebarCollapsed && (
        <Button variant="ghost" size="icon-sm" onClick={toggleSidebar}>
          <PanelLeft className="h-4 w-4" />
        </Button>
      )}
      <div className="flex flex-1 items-center gap-3 overflow-hidden">
        <DriveBreadcrumb />
      </div>
      <div className="flex shrink-0 items-center gap-2">
        <div className="relative hidden sm:block">
          <Search className="absolute left-2.5 top-1/2 h-3.5 w-3.5 -translate-y-1/2 text-muted-foreground" />
          <Input
            ref={searchRef}
            placeholder="Search files…"
            className="h-8 w-48 pl-8 text-sm lg:w-64"
            value={searchQuery}
            onChange={(e) => setSearchQuery(e.target.value)}
          />
        </div>
        {currentSpaceId && (
          <Button variant="ghost" size="icon-sm" onClick={openShareDialog}>
            <Share2 className="h-4 w-4" />
          </Button>
        )}
        <Button
          variant="ghost"
          size="icon-sm"
          onClick={() => setTheme(theme === "dark" ? "light" : "dark")}
        >
          {theme === "dark" ? <Sun className="h-4 w-4" /> : <Moon className="h-4 w-4" />}
        </Button>
        <Avatar className="h-7 w-7">
          <AvatarFallback className="text-xs">
            {user?.displayName?.charAt(0) ?? "?"}
          </AvatarFallback>
        </Avatar>
      </div>
    </header>
  )
}
