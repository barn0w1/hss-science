export type UserStatus = 'online' | 'idle' | 'dnd' | 'offline';

export interface User {
  id: string;
  name: string;
  email: string;
  avatarUrl?: string; // プロフィール画像のURL
  status: UserStatus; // オンライン状態（アイコンの右下に色を付ける用）
}