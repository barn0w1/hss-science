import { QueryClient, QueryClientProvider } from "@tanstack/react-query"
import { BrowserRouter, Navigate, Route, Routes } from "react-router-dom"

import { Toaster } from "@/components/ui/sonner"
import { TooltipProvider } from "@/components/ui/tooltip"
import { AuthProvider } from "@/context/AuthContext"
import { AppShell } from "@/components/layout/AppShell"
import { ProfilePage } from "@/pages/ProfilePage"
import { LinkedAccountsPage } from "@/pages/LinkedAccountsPage"
import { SecurityPage } from "@/pages/SecurityPage"
import { NotFoundPage } from "@/pages/NotFoundPage"
import { ROUTES } from "@/routes"

const queryClient = new QueryClient({
  defaultOptions: {
    queries: {
      staleTime: 30_000,
      retry: 1,
    },
  },
})

export default function App() {
  return (
    <QueryClientProvider client={queryClient}>
      <TooltipProvider>
        <BrowserRouter>
          <AuthProvider>
            <AppShell>
              <Routes>
                <Route
                  path={ROUTES.ROOT}
                  element={<Navigate to={ROUTES.PROFILE} replace />}
                />
                <Route path={ROUTES.PROFILE} element={<ProfilePage />} />
                <Route
                  path={ROUTES.LINKED_ACCOUNTS}
                  element={<LinkedAccountsPage />}
                />
                <Route path={ROUTES.SECURITY} element={<SecurityPage />} />
                <Route path="*" element={<NotFoundPage />} />
              </Routes>
            </AppShell>
            <Toaster />
          </AuthProvider>
        </BrowserRouter>
      </TooltipProvider>
    </QueryClientProvider>
  )
}
