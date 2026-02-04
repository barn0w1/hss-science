// メッセージの型定義
export interface Message {
  id: string;
  content: string;
  senderId: string; // 誰が送ったか
  createdAt: string; // 送信日時 (ISO 8601形式)
  updatedAt?: string;
  // 将来的にはここに reactions: [] や attachments: [] が増えます
}

// チャットルーム（チャンネルやDM）の型定義
export interface Room {
  id: string;
  name: string; // 部屋名、またはDM相手の名前
  type: 'dm' | 'group' | 'channel'; // 部屋の種類
  avatarUrl?: string; // 部屋のアイコン
  unreadCount: number; // 未読数バッジ用
  lastMessage?: Message; // サイドバーで「最後のメッセージ」を表示する用
  memberIds: string[]; // 参加者ID
}