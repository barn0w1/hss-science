import { ChevronDown, Link2, LogOut, ShieldCheck, UserRound } from "lucide-react"
import { NavLink } from "react-router-dom"

import { api } from "@/api/client"
import {
  DropdownMenu,
  DropdownMenuContent,
  DropdownMenuGroup,
  DropdownMenuItem,
  DropdownMenuTrigger,
} from "@/components/ui/dropdown-menu"
import { Separator } from "@/components/ui/separator"
import { UserAvatar } from "@/components/shared/UserAvatar"
import { useProfile } from "@/hooks/useProfile"
import { ROUTES } from "@/routes"
import { cn } from "@/lib/utils"

const NAV_ITEMS = [
  { icon: UserRound, label: "Personal info", to: ROUTES.PROFILE },
  { icon: ShieldCheck, label: "Security", to: ROUTES.SECURITY },
  { icon: Link2, label: "Linked accounts", to: ROUTES.LINKED_ACCOUNTS },
]

async function handleLogout() {
  const { data } = await api.POST("/api/v1/auth/logout")
  if (data?.redirect_to) {
    window.location.href = data.redirect_to
  }
}

export function NavSidebar() {
  const { profile } = useProfile()

  return (
    <aside className="flex w-64 flex-col border-r border-border bg-sidebar">
      <div className="p-4">
        <DropdownMenu>
          <DropdownMenuTrigger asChild>
            <button
              className="flex w-full items-center gap-3 rounded-lg p-2 text-left outline-none transition-colors hover:bg-accent focus-visible:ring-2 focus-visible:ring-ring"
              aria-label="User menu"
            >
              <UserAvatar
                name={profile?.name}
                email={profile?.email}
                picture={profile?.picture}
              />
              <div className="min-w-0 flex-1">
                <p className="truncate text-sm font-medium text-foreground">
                  {profile?.name ?? profile?.email ?? "Loading..."}
                </p>
                {profile?.name && (
                  <p className="truncate text-xs text-muted-foreground">
                    {profile.email}
                  </p>
                )}
              </div>
              <ChevronDown className="size-4 shrink-0 text-muted-foreground" />
            </button>
          </DropdownMenuTrigger>
          <DropdownMenuContent align="start" className="w-56">
            <DropdownMenuGroup>
              <DropdownMenuItem
                onSelect={() => void handleLogout()}
                className="text-destructive focus:text-destructive"
              >
                <LogOut />
                Sign out
              </DropdownMenuItem>
            </DropdownMenuGroup>
          </DropdownMenuContent>
        </DropdownMenu>
      </div>

      <Separator />

      <nav
        className="flex flex-col gap-1 p-4"
        aria-label="Main navigation"
      >
        {NAV_ITEMS.map(({ icon: Icon, label, to }) => (
          <NavLink key={to} to={to}>
            {({ isActive }) => (
              <span
                className={cn(
                  "flex items-center gap-3 rounded-lg px-3 py-2 text-sm transition-colors",
                  isActive
                    ? "bg-[var(--primary-container)] font-medium text-foreground"
                    : "text-muted-foreground hover:bg-accent hover:text-foreground"
                )}
              >
                <Icon className="size-4 shrink-0" />
                {label}
              </span>
            )}
          </NavLink>
        ))}
      </nav>
    </aside>
  )
}
