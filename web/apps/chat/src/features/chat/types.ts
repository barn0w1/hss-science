// ------------------------------------------------------------------
// Core Entities
// ------------------------------------------------------------------

export type UserStatus = 'online' | 'idle' | 'dnd' | 'offline';

export interface User {
  id: string;
  name: string;
  email?: string;
  avatarUrl?: string; // アイコン画像
  status: UserStatus; // オンライン状態
  bio?: string;       // プロフィール一言（コミュニティなのであると良い）
}

// ------------------------------------------------------------------
// Rooms (Spaces & DMs)
// ------------------------------------------------------------------

// Space: 永続的なコミュニティの場所（トピック、プロジェクト、雑談広場）
export interface Space {
  type: 'space';
  id: string;
  name: string;
  description?: string; // 「〇〇について語る場所です」など
  iconUrl?: string;     // スペースごとのアイコン（重要）
  
  // 権限や公開範囲（将来的な拡張用）
  isPublic: boolean;    // 誰でも参加できるか、招待制か
  
  // UI用状態
  unreadCount: number;
  lastActiveAt: string; // 並び替え用 (ISO String)
}

// DM: 個人または複数人での会話
export interface DirectMessage {
  type: 'dm';
  id: string;
  
  // DMの場合、ルーム名は自動生成（相手の名前など）する場合が多いが、
  // グループDMで名前をつけることもあるのでnameを持つ
  name?: string; 
  
  memberIds: string[]; // 参加者（相手が誰かを知るため）
  
  // UI用状態
  unreadCount: number;
  lastActiveAt: string;
}

// Room: リスト表示でSpaceとDMを統一して扱うためのUnion型
export type Room = Space | DirectMessage;
