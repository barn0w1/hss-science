import { useState } from "react"
import { Loader2 } from "lucide-react"

import { useProfile } from "@/hooks/useProfile"
import { UserAvatar } from "@/components/shared/UserAvatar"
import { PageHeader } from "@/components/shared/PageHeader"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Dialog,
  DialogContent,
  DialogFooter,
  DialogHeader,
  DialogTitle,
} from "@/components/ui/dialog"
import { Input } from "@/components/ui/input"
import { Label } from "@/components/ui/label"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"

function formatMemberSince(dateStr: string | undefined): string {
  if (!dateStr) return "—"
  return new Intl.DateTimeFormat(undefined, {
    year: "numeric",
    month: "long",
  }).format(new Date(dateStr))
}

export function ProfilePage() {
  const { profile, isPending, updateProfile, isUpdating } = useProfile()

  const [nameDialogOpen, setNameDialogOpen] = useState(false)
  const [nameInput, setNameInput] = useState("")

  const [pictureDialogOpen, setPictureDialogOpen] = useState(false)
  const [pictureInput, setPictureInput] = useState("")

  async function handleNameSave() {
    await updateProfile({ name: nameInput.trim() })
    setNameDialogOpen(false)
  }

  async function handlePictureSave() {
    await updateProfile({ picture: pictureInput.trim() || undefined })
    setPictureDialogOpen(false)
  }

  if (isPending) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader title="Personal info" />
        <div className="flex items-center gap-4">
          <Skeleton className="size-20 rounded-full" />
          <div className="flex flex-col gap-2">
            <Skeleton className="h-6 w-48" />
            <Skeleton className="h-4 w-36" />
          </div>
        </div>
        <Card>
          <CardHeader>
            <Skeleton className="h-5 w-24" />
          </CardHeader>
          <CardContent className="flex flex-col gap-4">
            {[1, 2, 3].map((i) => (
              <div key={i} className="flex items-center justify-between">
                <div className="flex flex-col gap-1.5">
                  <Skeleton className="h-3.5 w-16" />
                  <Skeleton className="h-4 w-40" />
                </div>
              </div>
            ))}
          </CardContent>
        </Card>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader title="Personal info" />

      <div className="flex items-center gap-4">
        <UserAvatar
          name={profile?.name}
          email={profile?.email}
          picture={profile?.picture}
          size="lg"
          className="size-20"
        />
        <div className="flex flex-col gap-1">
          <h2 className="text-xl font-semibold">
            {profile?.name ?? profile?.email ?? "—"}
          </h2>
          <div className="flex items-center gap-2">
            <span className="text-sm text-muted-foreground">
              {profile?.email}
            </span>
            {profile?.email_verified && (
              <Badge variant="outline" className="text-xs">
                Verified
              </Badge>
            )}
          </div>
        </div>
      </div>

      <Card>
        <CardHeader>
          <CardTitle>Basic info</CardTitle>
        </CardHeader>
        <CardContent className="flex flex-col">
          <div className="flex items-center justify-between py-3">
            <div className="flex flex-col gap-0.5">
              <span className="text-xs text-muted-foreground">Name</span>
              <span className="text-sm">{profile?.name ?? "—"}</span>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setNameInput(profile?.name ?? "")
                setNameDialogOpen(true)
              }}
            >
              Edit
            </Button>
          </div>
          <Separator />
          <div className="flex items-center justify-between py-3">
            <div className="flex flex-col gap-0.5">
              <span className="text-xs text-muted-foreground">Email</span>
              <span className="text-sm">{profile?.email ?? "—"}</span>
            </div>
          </div>
          <Separator />
          <div className="py-3">
            <div className="flex flex-col gap-0.5">
              <span className="text-xs text-muted-foreground">Member since</span>
              <span className="text-sm">
                {formatMemberSince(profile?.created_at)}
              </span>
            </div>
          </div>
        </CardContent>
      </Card>

      <Card>
        <CardHeader>
          <CardTitle>Profile picture</CardTitle>
        </CardHeader>
        <CardContent>
          <div className="flex items-center justify-between">
            <div className="flex items-center gap-3">
              <UserAvatar
                name={profile?.name}
                email={profile?.email}
                picture={profile?.picture}
              />
              <div className="flex flex-col gap-0.5">
                <span className="text-xs text-muted-foreground">
                  Picture URL
                </span>
                <span className="max-w-xs truncate text-sm">
                  {profile?.picture ?? "Not set"}
                </span>
                {profile?.picture_is_local && (
                  <Badge variant="outline" className="w-fit text-xs">
                    Custom
                  </Badge>
                )}
              </div>
            </div>
            <Button
              variant="ghost"
              size="sm"
              onClick={() => {
                setPictureInput(profile?.picture ?? "")
                setPictureDialogOpen(true)
              }}
            >
              Edit
            </Button>
          </div>
        </CardContent>
      </Card>

      <Dialog open={nameDialogOpen} onOpenChange={setNameDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit name</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-2">
            <Label htmlFor="name-input">Name</Label>
            <Input
              id="name-input"
              value={nameInput}
              onChange={(e) => setNameInput(e.target.value)}
              maxLength={100}
              autoFocus
              onKeyDown={(e) => {
                if (e.key === "Enter" && nameInput.trim()) {
                  void handleNameSave()
                }
              }}
            />
          </div>
          <DialogFooter>
            <Button
              variant="ghost"
              onClick={() => setNameDialogOpen(false)}
              disabled={isUpdating}
            >
              Cancel
            </Button>
            <Button
              onClick={() => void handleNameSave()}
              disabled={isUpdating || !nameInput.trim()}
            >
              {isUpdating && (
                <Loader2 data-icon="inline-start" className="animate-spin" />
              )}
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>

      <Dialog open={pictureDialogOpen} onOpenChange={setPictureDialogOpen}>
        <DialogContent>
          <DialogHeader>
            <DialogTitle>Edit profile picture</DialogTitle>
          </DialogHeader>
          <div className="flex flex-col gap-4">
            <div className="flex flex-col gap-2">
              <Label htmlFor="picture-input">Picture URL</Label>
              <Input
                id="picture-input"
                type="url"
                value={pictureInput}
                onChange={(e) => setPictureInput(e.target.value)}
                placeholder="https://example.com/photo.jpg"
                autoFocus
              />
            </div>
            {pictureInput && (
              <div className="flex items-center gap-3">
                <UserAvatar
                  name={profile?.name}
                  email={profile?.email}
                  picture={pictureInput}
                />
                <span className="text-sm text-muted-foreground">Preview</span>
              </div>
            )}
          </div>
          <DialogFooter>
            <Button
              variant="ghost"
              onClick={() => setPictureDialogOpen(false)}
              disabled={isUpdating}
            >
              Cancel
            </Button>
            <Button
              onClick={() => void handlePictureSave()}
              disabled={isUpdating}
            >
              {isUpdating && (
                <Loader2 data-icon="inline-start" className="animate-spin" />
              )}
              Save
            </Button>
          </DialogFooter>
        </DialogContent>
      </Dialog>
    </div>
  )
}
