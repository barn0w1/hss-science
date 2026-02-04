// src/features/chat/api/index.ts
import type { Room } from '../types';
import { MockChatRepository } from './mock';
import { RestChatRepository } from './rest';

// Repository Interface
// UIが必要とする「操作」の定義
export interface ChatRepository {
  getRooms(): Promise<Room[]>;
}

// Dependency Injection
// 環境変数 (VITE_USE_MOCK) で実装を切り替え
const USE_MOCK = true; // 開発中は強制的にtrueにしておいてもOK

export const chatRepository: ChatRepository = USE_MOCK
  ? new MockChatRepository()
  : new RestChatRepository();