import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query"
import { toast } from "sonner"

import { api } from "@/api/client"
import type { components } from "@/api/generated"

type Profile = components["schemas"]["Profile"]
type UpdateProfileRequest = components["schemas"]["UpdateProfileRequest"]

const PROFILE_QUERY_KEY = ["profile"] as const

export function useProfile() {
  const queryClient = useQueryClient()

  const { data: profile, isPending, error } = useQuery({
    queryKey: PROFILE_QUERY_KEY,
    queryFn: async () => {
      const { data, error: apiError } = await api.GET("/api/v1/profile")
      if (apiError) throw new Error(apiError.message ?? "Failed to load profile")
      return data as Profile
    },
  })

  const { mutateAsync: updateProfile, isPending: isUpdating } = useMutation({
    mutationFn: async (body: UpdateProfileRequest) => {
      const { data, error: apiError } = await api.PATCH("/api/v1/profile", {
        body,
      })
      if (apiError) throw new Error(apiError.message ?? "Failed to update profile")
      return data as Profile
    },
    onMutate: async (body) => {
      await queryClient.cancelQueries({ queryKey: PROFILE_QUERY_KEY })
      const snapshot = queryClient.getQueryData<Profile>(PROFILE_QUERY_KEY)
      queryClient.setQueryData<Profile>(PROFILE_QUERY_KEY, (prev) =>
        prev ? { ...prev, ...body } : prev
      )
      return { snapshot }
    },
    onError: (err, _body, context) => {
      queryClient.setQueryData<Profile>(PROFILE_QUERY_KEY, context?.snapshot)
      toast.error(err instanceof Error ? err.message : "Failed to update profile")
    },
    onSuccess: () => {
      toast.success("Profile updated")
    },
    onSettled: () => {
      void queryClient.invalidateQueries({ queryKey: PROFILE_QUERY_KEY })
    },
  })

  return { profile, isPending, error, updateProfile, isUpdating }
}
