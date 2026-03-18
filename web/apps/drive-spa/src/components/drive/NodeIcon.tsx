import {
  Folder,
  File,
  FileText,
  Image,
  Film,
  Music,
  Archive,
  Code,
  FileSpreadsheet,
  Presentation,
  Link,
} from "lucide-react"
import { getMimeCategory } from "@/lib/format"
import { cn } from "@/lib/utils"
import type { DriveNode } from "@/types/domain"

const categoryConfig = {
  image: { Icon: Image, color: "text-purple-500" },
  pdf: { Icon: FileText, color: "text-blue-500" },
  video: { Icon: Film, color: "text-red-500" },
  audio: { Icon: Music, color: "text-green-500" },
  archive: { Icon: Archive, color: "text-yellow-500" },
  code: { Icon: Code, color: "text-cyan-500" },
  text: { Icon: FileText, color: "text-foreground" },
  spreadsheet: { Icon: FileSpreadsheet, color: "text-emerald-500" },
  presentation: { Icon: Presentation, color: "text-orange-500" },
  default: { Icon: File, color: "text-muted-foreground" },
} as const

interface NodeIconProps {
  node: DriveNode
  className?: string
  size?: "sm" | "md" | "lg"
}

export function NodeIcon({ node, className, size = "md" }: NodeIconProps) {
  const sizeClass =
    size === "sm" ? "h-4 w-4" : size === "lg" ? "h-10 w-10" : "h-5 w-5"

  if (node.kind === "folder") {
    return (
      <Folder
        className={cn(sizeClass, "fill-sky-400/20 text-sky-400", className)}
      />
    )
  }

  if (node.kind === "symlink") {
    return (
      <Link className={cn(sizeClass, "text-muted-foreground", className)} />
    )
  }

  const { Icon, color } = categoryConfig[getMimeCategory(node.mimeType)]
  return <Icon className={cn(sizeClass, color, className)} />
}
