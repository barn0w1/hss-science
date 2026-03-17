import { useState } from "react"
import { Link2Off, Loader2 } from "lucide-react"

import { useProviders } from "@/hooks/useProviders"
import { ProviderIcon } from "@/components/shared/ProviderIcon"
import { PageHeader } from "@/components/shared/PageHeader"
import {
  Alert,
  AlertDescription,
  AlertTitle,
} from "@/components/ui/alert"
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
import { Button } from "@/components/ui/button"
import {
  Card,
  CardContent,
  CardHeader,
  CardTitle,
} from "@/components/ui/card"
import {
  Empty,
  EmptyDescription,
  EmptyHeader,
  EmptyMedia,
  EmptyTitle,
} from "@/components/ui/empty"
import { Skeleton } from "@/components/ui/skeleton"
import {
  Tooltip,
  TooltipContent,
  TooltipTrigger,
} from "@/components/ui/tooltip"

function formatProviderName(provider: string): string {
  const names: Record<string, string> = {
    google: "Google",
    github: "GitHub",
  }
  return names[provider] ?? provider.charAt(0).toUpperCase() + provider.slice(1)
}

function formatRelativeDate(dateStr: string | undefined): string {
  if (!dateStr) return "Never"
  const date = new Date(dateStr)
  const now = new Date()
  const diffMs = now.getTime() - date.getTime()
  const diffDays = Math.floor(diffMs / (1000 * 60 * 60 * 24))
  const diffHours = Math.floor(diffMs / (1000 * 60 * 60))
  const diffMinutes = Math.floor(diffMs / (1000 * 60))

  const rtf = new Intl.RelativeTimeFormat(undefined, { numeric: "auto" })
  if (diffMinutes < 60) return rtf.format(-diffMinutes, "minutes")
  if (diffHours < 24) return rtf.format(-diffHours, "hours")
  return rtf.format(-diffDays, "days")
}

function formatFullDate(dateStr: string | undefined): string {
  if (!dateStr) return "—"
  return new Intl.DateTimeFormat(undefined, {
    dateStyle: "full",
    timeStyle: "short",
  }).format(new Date(dateStr))
}

export function LinkedAccountsPage() {
  const { providers, isPending, conflictError, unlinkProvider, isUnlinking, clearConflict } =
    useProviders()

  const [pendingUnlinkId, setPendingUnlinkId] = useState<string | null>(null)

  async function handleUnlink(identityId: string) {
    clearConflict()
    await unlinkProvider(identityId)
    setPendingUnlinkId(null)
  }

  if (isPending) {
    return (
      <div className="flex flex-col gap-6">
        <PageHeader
          title="Linked accounts"
          description="Sign in with your connected accounts"
        />
        <div className="flex flex-col gap-4">
          {[1, 2].map((i) => (
            <Card key={i}>
              <CardContent className="py-4">
                <div className="flex items-center gap-3">
                  <Skeleton className="size-8 rounded-full" />
                  <div className="flex flex-col gap-1.5 flex-1">
                    <Skeleton className="h-4 w-24" />
                    <Skeleton className="h-3.5 w-40" />
                  </div>
                  <Skeleton className="h-8 w-16" />
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
        title="Linked accounts"
        description="Sign in with your connected accounts"
      />

      {conflictError && (
        <Alert variant="destructive">
          <Link2Off className="size-4" />
          <AlertTitle>Cannot unlink account</AlertTitle>
          <AlertDescription>{conflictError}</AlertDescription>
        </Alert>
      )}

      {providers?.length === 0 ? (
        <Empty>
          <EmptyMedia>
            <Link2Off className="size-8 text-muted-foreground" />
          </EmptyMedia>
          <EmptyHeader>
            <EmptyTitle>No linked accounts</EmptyTitle>
            <EmptyDescription>
              You have no federated accounts linked.
            </EmptyDescription>
          </EmptyHeader>
        </Empty>
      ) : (
        <div className="flex flex-col gap-4">
          {providers?.map((provider) => (
            <Card key={provider.identity_id}>
              <CardHeader>
                <CardTitle className="flex items-center gap-3">
                  <ProviderIcon provider={provider.provider ?? ""} />
                  {formatProviderName(provider.provider ?? "")}
                </CardTitle>
              </CardHeader>
              <CardContent className="flex items-center justify-between">
                <div className="flex flex-col gap-0.5">
                  <span className="text-sm">{provider.provider_email ?? "—"}</span>
                  <Tooltip>
                    <TooltipTrigger asChild>
                      <span className="text-xs text-muted-foreground cursor-default">
                        Last used{" "}
                        {formatRelativeDate(provider.last_login_at)}
                      </span>
                    </TooltipTrigger>
                    <TooltipContent>
                      <time dateTime={provider.last_login_at}>
                        {formatFullDate(provider.last_login_at)}
                      </time>
                    </TooltipContent>
                  </Tooltip>
                </div>
                <AlertDialog
                  open={pendingUnlinkId === provider.identity_id}
                  onOpenChange={(open) => {
                    if (!open) setPendingUnlinkId(null)
                  }}
                >
                  <AlertDialogTrigger asChild>
                    <Button
                      variant="destructive"
                      size="sm"
                      onClick={() =>
                        setPendingUnlinkId(provider.identity_id ?? null)
                      }
                    >
                      Unlink
                    </Button>
                  </AlertDialogTrigger>
                  <AlertDialogContent>
                    <AlertDialogHeader>
                      <AlertDialogTitle>
                        Remove{" "}
                        {formatProviderName(provider.provider ?? "")}?
                      </AlertDialogTitle>
                      <AlertDialogDescription>
                        You won&apos;t be able to sign in with{" "}
                        {formatProviderName(provider.provider ?? "")} after
                        unlinking it.
                      </AlertDialogDescription>
                    </AlertDialogHeader>
                    <AlertDialogFooter>
                      <AlertDialogCancel>Cancel</AlertDialogCancel>
                      <AlertDialogAction
                        onClick={() =>
                          void handleUnlink(provider.identity_id ?? "")
                        }
                        disabled={isUnlinking}
                        className="bg-destructive text-destructive-foreground hover:bg-destructive/90"
                      >
                        {isUnlinking && (
                          <Loader2
                            data-icon="inline-start"
                            className="animate-spin"
                          />
                        )}
                        Unlink
                      </AlertDialogAction>
                    </AlertDialogFooter>
                  </AlertDialogContent>
                </AlertDialog>
              </CardContent>
            </Card>
          ))}
        </div>
      )}
    </div>
  )
}
