/* eslint-disable react-refresh/only-export-components */
import * as React from "react"

import { api } from "@/api/client"
import { Skeleton } from "@/components/ui/skeleton"

type AuthContextValue = {
  userId: string | null
}

const AuthContext = React.createContext<AuthContextValue>({ userId: null })

export function AuthProvider({ children }: { children: React.ReactNode }) {
  const [userId, setUserId] = React.useState<string | null>(null)
  const [loading, setLoading] = React.useState(true)

  React.useEffect(() => {
    api.GET("/api/v1/auth/me").then(({ data }) => {
      if (data?.logged_in) {
        setUserId(data.user_id ?? null)
      } else {
        window.location.href = "/api/v1/auth/login"
      }
      setLoading(false)
    })
  }, [])

  if (loading) {
    return (
      <div className="flex min-h-svh items-center justify-center">
        <div className="flex flex-col items-center gap-4">
          <Skeleton className="size-12 rounded-full" />
          <div className="flex flex-col gap-2">
            <Skeleton className="h-4 w-48" />
            <Skeleton className="h-4 w-32" />
          </div>
        </div>
      </div>
    )
  }

  return (
    <AuthContext.Provider value={{ userId }}>{children}</AuthContext.Provider>
  )
}

export function useAuth() {
  return React.useContext(AuthContext)
}
