import { useState } from "react"
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import { api } from "@/api/client"
import type { components } from "@/api/generated"

type FederatedProvider = components["schemas"]["FederatedProvider"]

const PROVIDERS_QUERY_KEY = ["providers"] as const

export function useProviders() {
  const queryClient = useQueryClient()
  const [conflictError, setConflictError] = useState<string | null>(null)

  const { data: providers, isPending, error } = useQuery({
    queryKey: PROVIDERS_QUERY_KEY,
    queryFn: async () => {
      const { data, error: apiError } = await api.GET("/api/v1/providers")
      if (apiError) throw new Error(apiError.message ?? "Failed to load providers")
      return (data ?? []) as FederatedProvider[]
    },
  })

  const { mutateAsync: unlinkProvider, isPending: isUnlinking } = useMutation({
    mutationFn: async (identityId: string) => {
      const { error: apiError, response } = await api.DELETE(
        "/api/v1/providers/{identityId}",
        { params: { path: { identityId } } }
      )
      if (response.status === 409) {
        setConflictError(
          apiError?.message ?? "You must keep at least one sign-in method"
        )
        throw new Error("conflict")
      }
      if (apiError) throw new Error(apiError.message ?? "Failed to unlink provider")
    },
    onSuccess: () => {
      toast.success("Account unlinked")
      void queryClient.invalidateQueries({ queryKey: PROVIDERS_QUERY_KEY })
    },
    onError: (err) => {
      if (err instanceof Error && err.message !== "conflict") {
        toast.error(err.message)
      }
    },
  })

  const clearConflict = () => setConflictError(null)

  return {
    providers,
    isPending,
    error,
    conflictError,
    unlinkProvider,
    isUnlinking,
    clearConflict,
  }
}
