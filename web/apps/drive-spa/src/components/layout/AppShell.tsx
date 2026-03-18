import { useEffect } from "react"
import { Outlet } from "react-router-dom"
import { Toaster } from "@/components/ui/sonner"
import { Sidebar } from "./Sidebar"
import { useAuthStore } from "@/store/auth.store"
import { useDriveStore } from "@/store/drive.store"
import { useUIStore } from "@/store/ui.store"
import { useKeyboardShortcuts } from "@/hooks/useKeyboardShortcuts"
import { CommandPalette } from "@/components/drive/CommandPalette"
import { cn } from "@/lib/utils"
import type { User, Space } from "@/types/domain"

export function AppShell() {
  const setUser = useAuthStore((s) => s.setUser)
  const setLoading = useAuthStore((s) => s.setLoading)
  const setSpaces = useDriveStore((s) => s.setSpaces)
  const isSidebarCollapsed = useUIStore((s) => s.isSidebarCollapsed)

  useKeyboardShortcuts()

  useEffect(() => {
    Promise.all([
      fetch("/api/me").then((r) => r.json()),
      fetch("/api/spaces").then((r) => r.json()),
    ])
      .then(([user, spacesRes]: [User, { items: Space[] }]) => {
        setUser(user)
        setSpaces(spacesRes.items)
      })
      .catch(() => setLoading(false))
  }, [setUser, setLoading, setSpaces])

  return (
    <div className="flex h-svh overflow-hidden bg-background text-foreground">
      <Sidebar />
      <main
        className={cn(
          "flex flex-1 flex-col overflow-hidden transition-all duration-200",
          isSidebarCollapsed ? "ml-0" : "",
        )}
      >
        <Outlet />
      </main>
      <CommandPalette />
      <Toaster position="bottom-right" />
    </div>
  )
}
