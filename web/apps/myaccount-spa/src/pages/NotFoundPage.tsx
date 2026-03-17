import { Link } from "react-router-dom"

import { Button } from "@/components/ui/button"
import { ROUTES } from "@/routes"

export function NotFoundPage() {
  return (
    <div className="flex flex-col items-center justify-center gap-4 py-24 text-center">
      <h1 className="text-4xl font-bold tracking-tight">404</h1>
      <h2 className="text-xl font-semibold text-muted-foreground">
        Page not found
      </h2>
      <p className="text-sm text-muted-foreground">
        The page you&apos;re looking for doesn&apos;t exist.
      </p>
      <Button asChild>
        <Link to={ROUTES.PROFILE}>Go to profile</Link>
      </Button>
    </div>
  )
}
