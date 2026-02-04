// src/features/chat/types.ts

// ------------------------------------------------------------------
// Core Entities
// ------------------------------------------------------------------

export type UserStatus = 'online' | 'idle' | 'dnd' | 'offline';

export interface User {
  id: string;
  name: string;
  email?: string;
  avatarUrl?: string;
  status: UserStatus;
  bio?: string;
}

// ------------------------------------------------------------------
// Sidebar UI State (Common)
// ------------------------------------------------------------------
interface RoomUIState {
  unreadCount: number;  // 未読件数
  isPinned: boolean;    // ピン留めされているか
  isMuted: boolean;     // 通知をミュートしているか
  lastActiveAt: string; // 最終更新日時 (並び順用)
}

// ------------------------------------------------------------------
// Rooms (Spaces & DMs)
// ------------------------------------------------------------------

// Space: コミュニティの「広場」
export interface Space extends RoomUIState {
  type: 'space';
  id: string;
  name: string;
  description?: string;
  iconUrl?: string;
  isPublic: boolean;
}

// DM: 個人の「会話」
export interface DirectMessage extends RoomUIState {
  type: 'dm';
  id: string;
  name?: string; // グループDM用。1対1ならundefinedで相手の名前を表示
  memberIds: string[];
}

// Room: リスト表示用Union型
export type Room = Space | DirectMessage;