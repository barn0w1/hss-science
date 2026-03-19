import { NavLink } from "react-router-dom"
import { Trash2 } from "lucide-react"
import { cn } from "@/lib/utils"

export function SidebarNav() {
  return (
    <nav className="px-2">
      <NavLink
        to="/trash"
        className={({ isActive }) =>
          cn(
            "flex items-center gap-2.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors duration-100",
            isActive
              ? "bg-accent text-accent-foreground"
              : "text-muted-foreground hover:bg-accent/60 hover:text-accent-foreground",
          )
        }
      >
        <Trash2 className="h-4 w-4 shrink-0" />
        Trash
      </NavLink>
    </nav>
  )
}
