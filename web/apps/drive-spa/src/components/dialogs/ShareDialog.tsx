import {
  Dialog,
  DialogContent,
  DialogHeader,
  DialogTitle,
  DialogDescription,
} from "@/components/ui/dialog"
import { Avatar, AvatarFallback } from "@/components/ui/avatar"
import { Badge } from "@/components/ui/badge"
import { useUIStore } from "@/store/ui.store"
import { useDriveStore } from "@/store/drive.store"

const USER_MAP: Record<string, { displayName: string; email: string }> = {
  "user-alice": { displayName: "Alice Lambert", email: "alice@example.com" },
  "user-bob":   { displayName: "Bob Chen",      email: "bob@example.com"   },
  "user-carol": { displayName: "Carol Diaz",    email: "carol@example.com" },
}

export function ShareDialog() {
  const isOpen = useUIStore((s) => s.isShareDialogOpen)
  const closeShareDialog = useUIStore((s) => s.closeShareDialog)
  const currentSpaceId = useDriveStore((s) => s.currentSpaceId)
  const spaces = useDriveStore((s) => s.spaces)

  const space = spaces.find((s) => s.id === currentSpaceId)

  return (
    <Dialog open={isOpen} onOpenChange={(open) => !open && closeShareDialog()}>
      <DialogContent className="sm:max-w-md">
        <DialogHeader>
          <DialogTitle>Share "{space?.name}"</DialogTitle>
          <DialogDescription>
            People with access to this space
          </DialogDescription>
        </DialogHeader>
        <div className="flex flex-col gap-2 py-2">
          {space?.members.map((member) => {
            const u = USER_MAP[member.userId] ?? { displayName: member.userId, email: "" }
            return (
              <div key={member.userId} className="flex items-center gap-3">
                <Avatar className="h-8 w-8">
                  <AvatarFallback className="text-xs">
                    {u.displayName.charAt(0)}
                  </AvatarFallback>
                </Avatar>
                <div className="flex min-w-0 flex-1 flex-col">
                  <span className="truncate text-sm font-medium">{u.displayName}</span>
                  <span className="truncate text-xs text-muted-foreground">{u.email}</span>
                </div>
                <Badge variant="outline" className="shrink-0 capitalize">
                  {member.role}
                </Badge>
              </div>
            )
          })}
        </div>
      </DialogContent>
    </Dialog>
  )
}
