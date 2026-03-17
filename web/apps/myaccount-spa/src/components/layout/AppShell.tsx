import * as React from "react"

import { NavSidebar } from "./NavSidebar"
import { TopBar } from "./TopBar"

type AppShellProps = {
  children: React.ReactNode
}

export function AppShell({ children }: AppShellProps) {
  return (
    <div className="flex min-h-svh">
      <div className="hidden md:flex">
        <NavSidebar />
      </div>
      <div className="flex flex-1 flex-col min-w-0">
        <TopBar />
        <main className="flex-1 p-6 md:p-8">
          <div className="mx-auto max-w-3xl">{children}</div>
        </main>
      </div>
    </div>
  )
}
