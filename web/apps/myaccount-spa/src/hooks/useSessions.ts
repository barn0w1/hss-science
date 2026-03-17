import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import { api } from "@/api/client"
import type { components } from "@/api/generated"

type Session = components["schemas"]["Session"]

const SESSIONS_QUERY_KEY = ["sessions"] as const

export function useSessions() {
  const queryClient = useQueryClient()

  const { data: sessions, isPending, error } = useQuery({
    queryKey: SESSIONS_QUERY_KEY,
    queryFn: async () => {
      const { data, error: apiError } = await api.GET("/api/v1/sessions")
      if (apiError) throw new Error(apiError.message ?? "Failed to load sessions")
      return (data ?? []) as Session[]
    },
  })

  const { mutateAsync: revokeSession, isPending: isRevokingSession } =
    useMutation({
      mutationFn: async (sessionId: string) => {
        const { error: apiError } = await api.DELETE(
          "/api/v1/sessions/{sessionId}",
          { params: { path: { sessionId } } }
        )
        if (apiError) throw new Error(apiError.message ?? "Failed to revoke session")
      },
      onSuccess: () => {
        toast.success("Device signed out")
        void queryClient.invalidateQueries({ queryKey: SESSIONS_QUERY_KEY })
      },
      onError: (err) => {
        toast.error(err instanceof Error ? err.message : "Failed to revoke session")
      },
    })

  const { mutateAsync: revokeAllOther, isPending: isRevokingAll } = useMutation(
    {
      mutationFn: async () => {
        const { error: apiError } = await api.DELETE("/api/v1/sessions")
        if (apiError) throw new Error(apiError.message ?? "Failed to revoke sessions")
      },
      onSuccess: () => {
        toast.success("Other devices signed out")
        void queryClient.invalidateQueries({ queryKey: SESSIONS_QUERY_KEY })
      },
      onError: (err) => {
        toast.error(
          err instanceof Error ? err.message : "Failed to revoke sessions"
        )
      },
    }
  )

  return {
    sessions,
    isPending,
    error,
    revokeSession,
    isRevokingSession,
    revokeAllOther,
    isRevokingAll,
  }
}
