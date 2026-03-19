import { Link } from "react-router-dom"
import { Button } from "@/components/ui/button"

export function NotFoundPage() {
  return (
    <div className="flex flex-1 flex-col items-center justify-center gap-4">
      <p className="text-4xl font-bold text-muted-foreground/30">404</p>
      <p className="text-sm text-muted-foreground">Page not found</p>
      <Button asChild size="sm" variant="outline">
        <Link to="/drive">Go home</Link>
      </Button>
    </div>
  )
}
