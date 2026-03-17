import { Monitor, Smartphone, Tablet } from "lucide-react"
import { Loader2 } from "lucide-react"

import { useSessions } from "@/hooks/useSessions"
import { PageHeader } from "@/components/shared/PageHeader"
import {
  AlertDialog,
  AlertDialogAction,
  AlertDialogCancel,
  AlertDialogContent,
  AlertDialogDescription,
  AlertDialogFooter,
  AlertDialogHeader,
  AlertDialogTitle,
  AlertDialogTrigger,
} from "@/components/ui/alert-dialog"
import { Badge } from "@/components/ui/badge"
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardDescription,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import { Separator } from "@/components/ui/separator"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

type DeviceIconComponent = typeof Monitor

function getDeviceIcon(deviceName: string | undefined): DeviceIconComponent {
  const name = deviceName?.toLowerCase() ?? ""
  if (
    name.includes("mobile") ||
    name.includes("iphone") ||
    name.includes("android") ||
    name.includes("phone")
  ) {
    return Smartphone
  }
  if (name.includes("ipad") || name.includes("tablet")) {
    return Tablet
  }
  return Monitor
}

function formatRelativeTime(dateStr: string | undefined): string {
  if (!dateStr) return "Unknown"
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
  const diffMinutes = Math.floor(diffMs / (1000 * 60))

  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" })
  if (diffMinutes < 1) return "Just now"
  if (diffMinutes < 60) return rtf.format(-diffMinutes, "minutes")
  if (diffHours < 24) return rtf.format(-diffHours, "hours")
  return rtf.format(-diffDays, "days")
}

function formatFullDateTime(dateStr: string | undefined): string {
  if (!dateStr) return "—"
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "full",
    timeStyle: "short",
  }).format(new Date(dateStr))
}

export function SecurityPage() {
  const { sessions, isPending, revokeSession, isRevokingSession, revokeAllOther, isRevokingAll } =
    useSessions()

  const otherSessions = sessions?.filter((s) => !s.is_current) ?? []

  if (isPending) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader
          title="Your devices"
          description="These devices have access to your account"
        />
        <div className="flex flex-col gap-4">
          {[1, 2, 3].map((i) => (
            <Card key={i}>
              <CardContent className="py-4">
                <div className="flex items-center gap-3">
                  <Skeleton className="size-8 rounded-md" />
                  <div className="flex flex-col gap-1.5 flex-1">
                    <Skeleton className="h-4 w-40" />
                    <Skeleton className="h-3.5 w-28" />
                  </div>
                  <Skeleton className="h-8 w-20" />
                </div>
              </CardContent>
            </Card>
          ))}
        </div>
      </div>
    )
  }

  return (
    <div className="flex flex-col gap-6">
      <PageHeader
        title="Your devices"
        description="These devices have access to your account"
      />

      <div className="flex flex-col gap-4">
        {sessions?.map((session) => {
          const DeviceIcon = getDeviceIcon(session.device_name)
          return (
            <Card key={session.session_id}>
              <CardContent className="flex items-center gap-4 py-4">
                <DeviceIcon className="size-8 shrink-0 text-muted-foreground" />
                <div className="flex-1 min-w-0">
                  <div className="flex items-center gap-2">
                    <span className="text-sm font-medium truncate">
                      {session.device_name ?? "Unknown device"}
                    </span>
                    {session.is_current && (
                      <Badge variant="outline" className="text-xs shrink-0">
                        This device
                      </Badge>
                    )}
                  </div>
                  <div className="flex items-center gap-2 text-xs text-muted-foreground mt-0.5">
                    {session.ip_address && (
                      <span>{session.ip_address}</span>
                    )}
                    {session.ip_address && session.last_used_at && (
                      <span aria-hidden="true">·</span>
                    )}
                    <Tooltip>
                      <TooltipTrigger asChild>
                        <span className="cursor-default">
                          Active {formatRelativeTime(session.last_used_at)}
                        </span>
                      </TooltipTrigger>
                      <TooltipContent>
                        <time dateTime={session.last_used_at}>
                          {formatFullDateTime(session.last_used_at)}
                        </time>
                      </TooltipContent>
                    </Tooltip>
                  </div>
                </div>
                {!session.is_current && (
                  <AlertDialog>
                    <AlertDialogTrigger asChild>
                      <Button variant="outline" size="sm">
                        Sign out
                      </Button>
                    </AlertDialogTrigger>
                    <AlertDialogContent>
                      <AlertDialogHeader>
                        <AlertDialogTitle>Sign out this device?</AlertDialogTitle>
                        <AlertDialogDescription>
                          {session.device_name ?? "This device"} will be signed
                          out immediately.
                        </AlertDialogDescription>
                      </AlertDialogHeader>
                      <AlertDialogFooter>
                        <AlertDialogCancel>Cancel</AlertDialogCancel>
                        <AlertDialogAction
                          onClick={() =>
                            void revokeSession(session.session_id ?? "")
                          }
                          disabled={isRevokingSession}
                          className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                        >
                          {isRevokingSession && (
                            <Loader2
                              data-icon="inline-start"
                              className="animate-spin"
                            />
                          )}
                          Sign out
                        </AlertDialogAction>
                      </AlertDialogFooter>
                    </AlertDialogContent>
                  </AlertDialog>
                )}
              </CardContent>
            </Card>
          )
        })}
      </div>

      {otherSessions.length > 0 && (
        <>
          <Separator />
          <Card>
            <CardHeader>
              <CardTitle>Sign out other devices</CardTitle>
              <CardDescription>
                Remove access from all devices except this one.
              </CardDescription>
            </CardHeader>
            <CardContent>
              <AlertDialog>
                <AlertDialogTrigger asChild>
                  <Button variant="destructive">
                    Sign out all other devices
                  </Button>
                </AlertDialogTrigger>
                <AlertDialogContent>
                  <AlertDialogHeader>
                    <AlertDialogTitle>
                      Sign out all other devices?
                    </AlertDialogTitle>
                    <AlertDialogDescription>
                      {otherSessions.length}{" "}
                      {otherSessions.length === 1 ? "device" : "devices"} will
                      be signed out. You will remain signed in on this device.
                    </AlertDialogDescription>
                  </AlertDialogHeader>
                  <AlertDialogFooter>
                    <AlertDialogCancel>Cancel</AlertDialogCancel>
                    <AlertDialogAction
                      onClick={() => void revokeAllOther()}
                      disabled={isRevokingAll}
                      className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                    >
                      {isRevokingAll && (
                        <Loader2
                          data-icon="inline-start"
                          className="animate-spin"
                        />
                      )}
                      Sign out all
                    </AlertDialogAction>
                  </AlertDialogFooter>
                </AlertDialogContent>
              </AlertDialog>
            </CardContent>
          </Card>
        </>
      )}
    </div>
  )
}
