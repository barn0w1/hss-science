import { useEffect, useRef, useState } from "react"
import type { DriveNode } from "@/types/domain"
import { useDriveStore } from "@/store/drive.store"
import { Input } from "@/components/ui/input"

interface NodeRenameInputProps {
  node: DriveNode
}

export function NodeRenameInput({ node }: NodeRenameInputProps) {
  const renameNode = useDriveStore((s) => s.renameNode)
  const cancelRename = useDriveStore((s) => s.cancelRename)
  const currentName = useDriveStore(
    (s) => s.mutations.renamed.get(node.id) ?? node.name
  )

  const [value, setValue] = useState(currentName)
  const inputRef = useRef<HTMLInputElement>(null)

  useEffect(() => {
    const el = inputRef.current
    if (!el) return
    el.focus()
    const dotIdx = currentName.lastIndexOf(".")
    if (dotIdx > 0) {
      el.setSelectionRange(0, dotIdx)
    } else {
      el.select()
    }
  }, [currentName])

  function commit() {
    const trimmed = value.trim()
    if (trimmed && trimmed !== node.name) renameNode(node.id, trimmed)
    else cancelRename()
  }

  return (
    <Input
      ref={inputRef}
      value={value}
      onChange={(e) => setValue(e.target.value)}
      onBlur={commit}
      onKeyDown={(e) => {
        e.stopPropagation()
        if (e.key === "Enter") commit()
        if (e.key === "Escape") cancelRename()
      }}
      onClick={(e) => e.stopPropagation()}
      className="h-6 w-full px-1 py-0 text-xs"
    />
  )
}
