// src/features/chat/types.ts

// ------------------------------------------------------------------
// 1. User Entity
// ------------------------------------------------------------------
export interface User {
  id: string;
  name: string;
  avatarUrl?: string;
  status: 'online' | 'idle' | 'dnd' | 'offline';
  bio?: string;
}

// ------------------------------------------------------------------
// 2. Room Entity (Spaces & DMs)
// ------------------------------------------------------------------

// 共通プロパティ
export interface RoomBase {
  id: string;
  unreadCount: number;  
  isPinned: boolean;    // ピン留め
  isMuted: boolean;     // ミュート
  lastActiveAt: string; // ソート用 (ISO String)
}

// コミュニティの広場
export interface Space extends RoomBase {
  type: 'space';
  name: string;
  description?: string;
  iconUrl?: string;
  isPublic: boolean; // 鍵付きスペースなどのため
}

// 個人の会話
export interface DirectMessage extends RoomBase {
  type: 'dm';
  memberIds: string[]; // 相手のIDリスト
  name?: string;       // グループDMの場合のみ名前がある
}

// これをリストで使う
export type Room = Space | DirectMessage;