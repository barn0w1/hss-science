import * as React from "react"
import { Link2, Menu, ShieldCheck, UserRound } from "lucide-react"
import { NavLink } from "react-router-dom"

import { Button } from "@/components/ui/button"
import { Separator } from "@/components/ui/separator"
import {
  Sheet,
  SheetContent,
  SheetTitle,
  SheetTrigger,
} from "@/components/ui/sheet"
import { UserAvatar } from "@/components/shared/UserAvatar"
import { useProfile } from "@/hooks/useProfile"
import { ROUTES } from "@/routes"
import { cn } from "@/lib/utils"

const NAV_ITEMS = [
  { icon: UserRound, label: "Personal info", to: ROUTES.PROFILE },
  { icon: ShieldCheck, label: "Security", to: ROUTES.SECURITY },
  { icon: Link2, label: "Linked accounts", to: ROUTES.LINKED_ACCOUNTS },
]

export function TopBar() {
  const { profile } = useProfile()
  const [open, setOpen] = React.useState(false)

  return (
    <header className="flex items-center gap-4 border-b border-border px-4 py-3 md:hidden">
      <Sheet open={open} onOpenChange={setOpen}>
        <SheetTrigger asChild>
          <Button variant="ghost" size="icon" aria-label="Open navigation">
            <Menu />
          </Button>
        </SheetTrigger>
        <SheetContent side="left" className="w-64 p-0">
          <SheetTitle className="sr-only">Navigation</SheetTitle>
          <div className="flex items-center gap-3 p-4">
            <UserAvatar
              name={profile?.name}
              email={profile?.email}
              picture={profile?.picture}
            />
            <div className="min-w-0 flex-1">
              <p className="truncate text-sm font-medium">
                {profile?.name ?? profile?.email ?? ""}
              </p>
              {profile?.name && (
                <p className="truncate text-xs text-muted-foreground">
                  {profile.email}
                </p>
              )}
            </div>
          </div>
          <Separator />
          <nav className="flex flex-col gap-1 p-4" aria-label="Main navigation">
            {NAV_ITEMS.map(({ icon: Icon, label, to }) => (
              <NavLink key={to} to={to} onClick={() => setOpen(false)}>
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
        </SheetContent>
      </Sheet>
      <span className="flex-1 text-sm font-semibold">Account</span>
      <UserAvatar
        name={profile?.name}
        email={profile?.email}
        picture={profile?.picture}
        size="sm"
      />
    </header>
  )
}
