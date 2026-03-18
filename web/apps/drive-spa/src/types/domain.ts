export type ContentHash = string & { readonly __brand: "ContentHash" }
export type NodeId = string & { readonly __brand: "NodeId" }
export type SpaceId = string & { readonly __brand: "SpaceId" }
export type UserId = string & { readonly __brand: "UserId" }

interface NodeBase {
  id: NodeId
  spaceId: SpaceId
  parentId: NodeId | null
  name: string
  createdAt: string
  updatedAt: string
  createdBy: UserId
}

export interface FileNode extends NodeBase {
  kind: "file"
  contentHash: ContentHash
  size: number
  mimeType: string
}

export interface FolderNode extends NodeBase {
  kind: "folder"
  childCount: number
}

export interface SymlinkNode extends NodeBase {
  kind: "symlink"
  targetId: NodeId
}

export type DriveNode = FileNode | FolderNode | SymlinkNode

export type SpaceRole = "owner" | "editor" | "viewer"

export interface SpaceMember {
  userId: UserId
  role: SpaceRole
}

export interface Space {
  id: SpaceId
  name: string
  ownerId: UserId
  personal: boolean
  members: SpaceMember[]
  createdAt: string
  rootNodeId: NodeId
}

export interface User {
  id: UserId
  displayName: string
  email: string
  avatarUrl: string | null
}
