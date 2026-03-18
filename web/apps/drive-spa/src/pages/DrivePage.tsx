import { useEffect } from "react"
import { useParams } from "react-router-dom"
import { useDriveStore } from "@/store/drive.store"
import type { SpaceId, NodeId } from "@/types/domain"
import { DriveView } from "@/components/drive/DriveView"
import { TopBar } from "@/components/layout/TopBar"

export function DrivePage() {
  const params = useParams<{ spaceId?: string; nodeId?: string }>()
  const setCurrentSpace = useDriveStore((s) => s.setCurrentSpace)
  const navigateTo = useDriveStore((s) => s.navigateTo)
  const spaces = useDriveStore((s) => s.spaces)

  useEffect(() => {
    if (params.spaceId) {
      setCurrentSpace(params.spaceId as SpaceId)
    } else if (spaces.length > 0 && !params.spaceId) {
      setCurrentSpace(spaces[0].id)
    }
  }, [params.spaceId, spaces, setCurrentSpace])

  useEffect(() => {
    navigateTo(params.nodeId ? (params.nodeId as NodeId) : null)
  }, [params.nodeId, navigateTo])

  return (
    <div className="flex flex-1 flex-col overflow-hidden">
      <TopBar />
      <DriveView />
    </div>
  )
}
