import { useNavigate, useParams } from "react-router-dom"
import { useDriveStore } from "@/store/drive.store"
import { cn } from "@/lib/utils"
import { Users, User } from "lucide-react"

export function SidebarSpaceSwitcher() {
  const spaces = useDriveStore((s) => s.spaces)
  const currentSpaceId = useDriveStore((s) => s.currentSpaceId)
  const setCurrentSpace = useDriveStore((s) => s.setCurrentSpace)
  const navigate = useNavigate()
  const params = useParams()

  function handleSpaceClick(spaceId: string) {
    setCurrentSpace(spaceId as Parameters<typeof setCurrentSpace>[0])
    navigate(`/drive/${spaceId}`)
  }

  return (
    <div className="px-2">
      <p className="mb-1 px-3 text-xs font-medium text-muted-foreground/70 uppercase tracking-wider">
        Spaces
      </p>
      <div className="flex flex-col gap-0.5">
        {spaces.map((space) => {
          const isActive = space.id === (params.spaceId ?? currentSpaceId)
          return (
            <button
              key={space.id}
              onClick={() => handleSpaceClick(space.id)}
              className={cn(
                "flex w-full items-center gap-2.5 rounded-md px-3 py-1.5 text-sm font-medium transition-colors duration-100",
                isActive
                  ? "bg-accent text-accent-foreground"
                  : "text-muted-foreground hover:bg-accent/60 hover:text-accent-foreground",
              )}
            >
              {space.personal ? (
                <User className="h-4 w-4 shrink-0" />
              ) : (
                <Users className="h-4 w-4 shrink-0" />
              )}
              <span className="truncate">{space.name}</span>
            </button>
          )
        })}
      </div>
    </div>
  )
}
