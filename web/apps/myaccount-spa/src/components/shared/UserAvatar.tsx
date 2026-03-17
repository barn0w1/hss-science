import { Avatar, AvatarFallback, AvatarImage } from "@/components/ui/avatar"
import { cn } from "@/lib/utils"

type UserAvatarProps = {
  name?: string | null
  email?: string | null
  picture?: string | null
  size?: "default" | "sm" | "lg"
  className?: string
}

function getInitials(name?: string | null, email?: string | null): string {
  if (name) {
    const parts = name.trim().split(/\s+/)
    if (parts.length >= 2) {
      return (parts[0][0] + parts[parts.length - 1][0]).toUpperCase()
    }
    return name.slice(0, 2).toUpperCase()
  }
  if (email) {
    return email.slice(0, 2).toUpperCase()
  }
  return "?"
}

export function UserAvatar({
  name,
  email,
  picture,
  size = "default",
  className,
}: UserAvatarProps) {
  return (
    <Avatar size={size} className={cn(className)}>
      {picture && (
        <AvatarImage src={picture} alt={name ?? email ?? "User avatar"} />
      )}
      <AvatarFallback>{getInitials(name, email)}</AvatarFallback>
    </Avatar>
  )
}
